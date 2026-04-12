package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sync"

	"github.com/kayushkin/llm-bridge/internal/store"
	"github.com/kayushkin/llm-bridge/msg"
)

// Manager handles harness subprocess lifecycle.
type Manager struct {
	mu          sync.RWMutex
	processes   map[string]*Process       // sessionID → process
	subscribers map[string][]chan msg.Event // sessionID → SSE subscriber channels
	store       *store.Store
}

// NewManager creates a harness manager.
func NewManager(st *store.Store) *Manager {
	return &Manager{
		processes:   make(map[string]*Process),
		subscribers: make(map[string][]chan msg.Event),
		store:       st,
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
	case msg.HarnessInber:
		return "llm-bridge-inber"
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

// Subscribe creates a new event channel for SSE consumers.
// The returned channel receives all events for the session.
// Call Unsubscribe when done.
func (m *Manager) Subscribe(sessionID string) chan msg.Event {
	ch := make(chan msg.Event, 100)
	m.mu.Lock()
	m.subscribers[sessionID] = append(m.subscribers[sessionID], ch)
	m.mu.Unlock()
	return ch
}

// Unsubscribe removes an SSE subscriber channel.
func (m *Manager) Unsubscribe(sessionID string, ch chan msg.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	subs := m.subscribers[sessionID]
	for i, s := range subs {
		if s == ch {
			m.subscribers[sessionID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}

// HasProcess returns true if a harness process is running for the session.
func (m *Manager) HasProcess(sessionID string) bool {
	return m.Get(sessionID) != nil
}

// readEvents reads events from process, persists them, updates state,
// and fans out to all SSE subscribers.
func (m *Manager) readEvents(proc *Process) {
	sid := proc.SessionID()

	for event := range proc.Events() {
		// Persist event
		if data, err := json.Marshal(event); err == nil {
			if err := m.store.StoreEvent(sid, string(event.Type), data); err != nil {
				log.Printf("[harness] failed to store event: %v", err)
			}
		}

		// Update session state based on event type
		switch event.Type {
		case msg.EventSessionState:
			if event.State != nil {
				m.store.UpdateSessionState(sid, string(event.State.State))
			}
		case msg.EventResult:
			m.store.UpdateSessionState(sid, string(msg.SessionCompleted))
		case msg.EventError:
			m.store.UpdateSessionState(sid, string(msg.SessionError))
		}

		// Fan out to SSE subscribers
		m.mu.RLock()
		for _, ch := range m.subscribers[sid] {
			select {
			case ch <- event:
			default:
				// Subscriber too slow, drop event
			}
		}
		m.mu.RUnlock()
	}

	// Process exited — close all subscriber channels
	m.mu.Lock()
	for _, ch := range m.subscribers[sid] {
		close(ch)
	}
	delete(m.subscribers, sid)
	delete(m.processes, sid)
	m.mu.Unlock()

	m.store.UpdateSessionPID(sid, 0)
}

// ActiveCount returns the number of running processes.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.processes)
}
