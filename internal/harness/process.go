package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/kayushkin/llm-bridge/internal/store"
	"github.com/kayushkin/llm-bridge/msg"
)

// Request is sent to harness subprocess via stdin.
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// StartParams for the "start" method.
type StartParams struct {
	SessionID   string `json:"session_id"`
	DisplayName string `json:"display_name,omitempty"`
	AgentID     string `json:"agent_id,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	Resume      bool   `json:"resume,omitempty"`
	Fork        string `json:"fork,omitempty"` // parent session ID
}

// MessageParams for the "message" method.
type MessageParams struct {
	Content string `json:"content"`
}

// Process represents a running harness subprocess.
type Process struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	sessionID string
	events    chan msg.Event
	done      chan struct{}
}

// StartProcess spawns a harness bridge subprocess.
func StartProcess(ctx context.Context, binPath string, sess *store.Session) (*Process, error) {
	cmd := exec.Command(binPath)
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("start: %w", err)
	}

	p := &Process{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		sessionID: sess.ID,
		events:    make(chan msg.Event, 100),
		done:      make(chan struct{}),
	}

	// Send start request
	params := StartParams{
		SessionID:   sess.ID,
		DisplayName: sess.DisplayName,
		AgentID:     sess.AgentID,
	}
	if sess.ParentID != "" {
		params.Fork = sess.ParentID
	}
	if err := p.sendRequest("start", params); err != nil {
		p.Kill()
		return nil, fmt.Errorf("send start: %w", err)
	}

	// Start reading stdout
	go p.readLoop()

	return p, nil
}

// PID returns the process ID.
func (p *Process) PID() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// SessionID returns the session ID.
func (p *Process) SessionID() string {
	return p.sessionID
}

// Events returns the event channel.
func (p *Process) Events() <-chan msg.Event {
	return p.events
}

// Send writes a user message to the harness.
func (p *Process) Send(message string) error {
	return p.sendRequest("message", MessageParams{Content: message})
}

// SendCommand sends a command (compact, resume, etc.).
func (p *Process) SendCommand(cmd string) error {
	return p.sendRequest(cmd, nil)
}

// Interrupt sends SIGINT to pause the harness.
func (p *Process) Interrupt() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd.Process == nil {
		return fmt.Errorf("process not running")
	}
	return p.cmd.Process.Signal(syscall.SIGINT)
}

// Kill terminates the process.
func (p *Process) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	close(p.done)

	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
	return p.cmd.Wait()
}

// sendRequest writes a JSON request to stdin.
func (p *Process) sendRequest(method string, params any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	req := Request{Method: method}
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return err
		}
		req.Params = data
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = p.stdin.Write(data)
	return err
}

// readLoop reads NDJSON events from stdout.
func (p *Process) readLoop() {
	defer close(p.events)

	scanner := bufio.NewScanner(p.stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		select {
		case <-p.done:
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event msg.Event
		if err := json.Unmarshal(line, &event); err != nil {
			// Log error but continue
			continue
		}

		select {
		case p.events <- event:
		case <-p.done:
			return
		}
	}
}
