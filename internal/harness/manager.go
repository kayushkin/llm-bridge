package harness

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/kayushkin/llm-bridge/internal/store"
	"github.com/kayushkin/llm-bridge/msg"
)

// Manager handles harness subprocess lifecycle.
type Manager struct {
	mu        sync.RWMutex
	processes map[string]*Process // sessionID → process
	store     *store.Store
}

// NewManager creates a harness manager.
func NewManager(st *store.Store) *Manager {
	return &Manager{
		processes: make(map[string]*Process),
		store:     st,
	}
}

// BinaryName returns the expected binary name for a harness.
func BinaryName(h msg.Harness) string {
	switch h {
	case msg.HarnessClaudeCode:
		return "llm-bridge-claudecode"
	case msg.HarnessCodex:
		return "llm-bridge-codex"
	case msg.HarnessOpenClaw:
		return "llm-bridge-openclaw"
	default:
		return ""
	}
}

// Available checks if a harness binary is in PATH.
func Available(h msg.Harness) (string, bool) {
	bin := BinaryName(h)
	if bin == "" {
		return "", false
	}
	path, err := exec.LookPath(bin)
	return path, err == nil
}

// Start spawns a new harness session.
func (m *Manager) Start(ctx context.Context, sess *store.Session) (*Process, error) {
	h := msg.Harness(sess.Harness)
	binPath, ok := Available(h)
	if !ok {
		return nil, fmt.Errorf("harness binary not found: %s", BinaryName(h))
	}

	proc, err := StartProcess(ctx, binPath, sess)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.processes[sess.ID] = proc
	m.mu.Unlock()

	// Update session with PID
	m.store.UpdateSessionPID(sess.ID, proc.PID())
	m.store.UpdateSessionState(sess.ID, string(msg.SessionRunning))

	// Start event reader goroutine
	go m.readEvents(proc)

	return proc, nil
}

// Get returns a running process by session ID.
func (m *Manager) Get(sessionID string) *Process {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.processes[sessionID]
}

// Stop sends interrupt signal to pause session.
func (m *Manager) Stop(sessionID string) error {
	proc := m.Get(sessionID)
	if proc == nil {
		return fmt.Errorf("session not running: %s", sessionID)
	}
	return proc.Interrupt()
}

// Kill terminates the session.
func (m *Manager) Kill(sessionID string) error {
	proc := m.Get(sessionID)
	if proc == nil {
		return fmt.Errorf("session not running: %s", sessionID)
	}

	m.mu.Lock()
	delete(m.processes, sessionID)
	m.mu.Unlock()

	return proc.Kill()
}

// Send writes a message to the harness stdin.
func (m *Manager) Send(sessionID string, message string) error {
	proc := m.Get(sessionID)
	if proc == nil {
		return fmt.Errorf("session not running: %s", sessionID)
	}
	return proc.Send(message)
}

// SendCommand sends a command (compact, resume, etc.) to the harness.
func (m *Manager) SendCommand(sessionID string, cmd string) error {
	proc := m.Get(sessionID)
	if proc == nil {
		return fmt.Errorf("session not running: %s", sessionID)
	}
	return proc.SendCommand(cmd)
}

// Events returns the event channel for a session.
func (m *Manager) Events(sessionID string) <-chan msg.Event {
	proc := m.Get(sessionID)
	if proc == nil {
		return nil
	}
	return proc.Events()
}

// readEvents reads events from process and updates session state.
func (m *Manager) readEvents(proc *Process) {
	for event := range proc.Events() {
		// Update session state based on event type
		switch event.Type {
		case msg.EventSessionState:
			if event.State != nil {
				m.store.UpdateSessionState(proc.SessionID(), string(event.State.State))
			}
		case msg.EventResult:
			m.store.UpdateSessionState(proc.SessionID(), string(msg.SessionCompleted))
		case msg.EventError:
			m.store.UpdateSessionState(proc.SessionID(), string(msg.SessionError))
		}
	}

	// Process exited
	m.mu.Lock()
	delete(m.processes, proc.SessionID())
	m.mu.Unlock()

	m.store.UpdateSessionPID(proc.SessionID(), 0)
}

// ActiveCount returns the number of running processes.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.processes)
}
