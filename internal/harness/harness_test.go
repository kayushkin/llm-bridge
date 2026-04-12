package harness

import (
	"path/filepath"
	"testing"

	"github.com/kayushkin/llm-bridge/internal/store"
	"github.com/kayushkin/llm-bridge/msg"
)

func tempManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	st, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewManager(st)
}

// --- BinaryName ---

func TestBinaryName(t *testing.T) {
	tests := []struct {
		harness msg.Harness
		want    string
	}{
		{msg.HarnessClaudeCode, "llm-bridge-claudecode"},
		{msg.HarnessCodex, "llm-bridge-codex"},
		{msg.HarnessOpenClaw, "llm-bridge-openclaw"},
		{msg.Harness("unknown"), ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.harness), func(t *testing.T) {
			got := BinaryName(tt.harness)
			if got != tt.want {
				t.Errorf("BinaryName(%q) = %q, want %q", tt.harness, got, tt.want)
			}
		})
	}
}

// --- Availability ---
// These tests make harness availability clearly visible in test output.

func TestAvailable_ClaudeCode(t *testing.T) {
	path, ok := Available(msg.HarnessClaudeCode)
	if !ok {
		t.Skipf("UNAVAILABLE: %s binary not in PATH (expected: %s)", msg.HarnessClaudeCode, BinaryName(msg.HarnessClaudeCode))
	}
	t.Logf("AVAILABLE: %s at %s", msg.HarnessClaudeCode, path)
}

func TestAvailable_Codex(t *testing.T) {
	path, ok := Available(msg.HarnessCodex)
	if !ok {
		t.Skipf("UNAVAILABLE: %s binary not in PATH (expected: %s)", msg.HarnessCodex, BinaryName(msg.HarnessCodex))
	}
	t.Logf("AVAILABLE: %s at %s", msg.HarnessCodex, path)
}

func TestAvailable_OpenClaw(t *testing.T) {
	path, ok := Available(msg.HarnessOpenClaw)
	if !ok {
		t.Skipf("UNAVAILABLE: %s binary not in PATH (expected: %s)", msg.HarnessOpenClaw, BinaryName(msg.HarnessOpenClaw))
	}
	t.Logf("AVAILABLE: %s at %s", msg.HarnessOpenClaw, path)
}

func TestAvailable_Unknown(t *testing.T) {
	_, ok := Available(msg.Harness("bogus"))
	if ok {
		t.Error("unknown harness should not be available")
	}
}

// --- Manager basics (no subprocess needed) ---

func TestManager_GetNonexistent(t *testing.T) {
	m := tempManager(t)
	if p := m.Get("nonexistent"); p != nil {
		t.Error("expected nil for nonexistent session")
	}
}

func TestManager_HasProcessNonexistent(t *testing.T) {
	m := tempManager(t)
	if m.HasProcess("nonexistent") {
		t.Error("expected false for nonexistent session")
	}
}

func TestManager_ActiveCountEmpty(t *testing.T) {
	m := tempManager(t)
	if c := m.ActiveCount(); c != 0 {
		t.Errorf("ActiveCount = %d, want 0", c)
	}
}

func TestManager_StopNonexistent(t *testing.T) {
	m := tempManager(t)
	err := m.Stop("nonexistent")
	if err == nil {
		t.Error("expected error stopping nonexistent session")
	}
}

func TestManager_KillNonexistent(t *testing.T) {
	m := tempManager(t)
	err := m.Kill("nonexistent")
	if err == nil {
		t.Error("expected error killing nonexistent session")
	}
}

func TestManager_SendNonexistent(t *testing.T) {
	m := tempManager(t)
	err := m.Send("nonexistent", "hello")
	if err == nil {
		t.Error("expected error sending to nonexistent session")
	}
}

func TestManager_SendCommandNonexistent(t *testing.T) {
	m := tempManager(t)
	err := m.SendCommand("nonexistent", "compact")
	if err == nil {
		t.Error("expected error sending command to nonexistent session")
	}
}

// --- Subscribe/Unsubscribe ---

func TestManager_SubscribeUnsubscribe(t *testing.T) {
	m := tempManager(t)

	ch := m.Subscribe("sess_1")
	if ch == nil {
		t.Fatal("Subscribe returned nil channel")
	}

	// Should be able to unsubscribe without panic
	m.Unsubscribe("sess_1", ch)

	// Channel should be closed after unsubscribe
	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after unsubscribe")
	}
}

func TestManager_MultipleSubscribers(t *testing.T) {
	m := tempManager(t)

	ch1 := m.Subscribe("sess_1")
	ch2 := m.Subscribe("sess_1")

	// Unsubscribe first, second should still be open
	m.Unsubscribe("sess_1", ch1)

	// ch2 should still be open
	select {
	case <-ch2:
		t.Error("ch2 should still be open")
	default:
		// Good — channel is open
	}

	m.Unsubscribe("sess_1", ch2)
}

// --- Start with unavailable binary ---

func TestManager_StartUnavailableHarness(t *testing.T) {
	m := tempManager(t)
	sess := &store.Session{
		ID:      "sess_1",
		Harness: "bogus_harness",
		State:   "idle",
	}
	_, err := m.Start(t.Context(), sess)
	if err == nil {
		t.Error("expected error starting unavailable harness")
	}
}
