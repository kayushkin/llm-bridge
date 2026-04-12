package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/kayushkin/llm-bridge/internal/harness"
	"github.com/kayushkin/llm-bridge/internal/store"
	"github.com/kayushkin/llm-bridge/msg"
)

func testServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	srv := New(st)
	return srv, st
}

// --- Health ---

func TestHealth(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Should report all 3 harnesses
	if len(resp.Harnesses) != 3 {
		t.Fatalf("harness count = %d, want 3", len(resp.Harnesses))
	}

	// Log availability for visibility
	for _, h := range resp.Harnesses {
		if h.Available {
			t.Logf("HARNESS AVAILABLE: %s at %s (capabilities: %v)", h.Name, h.Binary, h.Capabilities)
		} else {
			t.Logf("HARNESS UNAVAILABLE: %s", h.Name)
		}
	}
}

func TestHealth_SessionCounts(t *testing.T) {
	srv, st := testServer(t)

	st.CreateSession(&store.Session{ID: "s1", Harness: "claude_code", State: "running"})
	st.CreateSession(&store.Session{ID: "s2", Harness: "codex", State: "idle"})
	st.CreateSession(&store.Session{ID: "s3", Harness: "openclaw", State: "completed"})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp HealthResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Sessions.Running != 1 {
		t.Errorf("Running = %d, want 1", resp.Sessions.Running)
	}
	if resp.Sessions.Idle != 1 {
		t.Errorf("Idle = %d, want 1", resp.Sessions.Idle)
	}
	if resp.Sessions.Completed != 1 {
		t.Errorf("Completed = %d, want 1", resp.Sessions.Completed)
	}
}

// --- Harnesses endpoint ---

func TestHarnesses(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("GET", "/harnesses", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var harnesses []HarnessStatus
	json.NewDecoder(w.Body).Decode(&harnesses)

	if len(harnesses) != 3 {
		t.Fatalf("count = %d, want 3", len(harnesses))
	}

	// Check capabilities are populated
	for _, h := range harnesses {
		if h.Capabilities == nil {
			t.Errorf("capabilities nil for %s", h.Name)
		}
	}
}

// --- Harness capabilities ---

func TestHarnessCapabilities_ClaudeCode(t *testing.T) {
	caps := harnessCapabilities[msg.HarnessClaudeCode]
	expected := []string{"compact", "fork", "model", "effort", "tools", "budget", "system_prompt"}
	for _, e := range expected {
		found := false
		for _, c := range caps {
			if c == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("claude_code missing capability %q", e)
		}
	}
}

func TestHarnessCapabilities_Codex(t *testing.T) {
	caps := harnessCapabilities[msg.HarnessCodex]
	if len(caps) != 1 || caps[0] != "model" {
		t.Errorf("codex capabilities = %v, want [model]", caps)
	}
}

func TestHarnessCapabilities_OpenClaw(t *testing.T) {
	caps := harnessCapabilities[msg.HarnessOpenClaw]
	expected := []string{"compact", "model", "effort"}
	for _, e := range expected {
		found := false
		for _, c := range caps {
			if c == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("openclaw missing capability %q", e)
		}
	}
}

// --- Sessions ---

func TestListSessions_Empty(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("GET", "/sessions", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var sessions []store.Session
	json.NewDecoder(w.Body).Decode(&sessions)
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessions_WithData(t *testing.T) {
	srv, st := testServer(t)
	st.CreateSession(&store.Session{ID: "s1", Harness: "claude_code", State: "idle"})
	st.CreateSession(&store.Session{ID: "s2", Harness: "codex", State: "running"})

	req := httptest.NewRequest("GET", "/sessions", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var sessions []store.Session
	json.NewDecoder(w.Body).Decode(&sessions)
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestCreateSession_Valid(t *testing.T) {
	srv, _ := testServer(t)
	body := `{"harness":"claude_code","display_name":"Test"}`
	req := httptest.NewRequest("POST", "/sessions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}

	var sess store.Session
	json.NewDecoder(w.Body).Decode(&sess)
	if sess.Harness != "claude_code" {
		t.Errorf("Harness = %q", sess.Harness)
	}
	if sess.State != string(msg.SessionIdle) {
		t.Errorf("State = %q, want %q", sess.State, msg.SessionIdle)
	}
	if sess.ID == "" {
		t.Error("ID is empty")
	}
}

func TestCreateSession_InvalidHarness(t *testing.T) {
	srv, _ := testServer(t)
	body := `{"harness":"invalid_thing"}`
	req := httptest.NewRequest("POST", "/sessions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestCreateSession_InvalidBody(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("POST", "/sessions", bytes.NewBufferString("{broken"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestCreateSession_AutoStart_Unavailable(t *testing.T) {
	// If the harness binary isn't installed, auto_start should return 503
	_, ok := harness.Available(msg.HarnessClaudeCode)
	if ok {
		t.Skip("claude_code binary is available, skipping unavailability test")
	}

	srv, _ := testServer(t)
	body := `{"harness":"claude_code","auto_start":true}`
	req := httptest.NewRequest("POST", "/sessions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestCreateSession_AllHarnessTypes(t *testing.T) {
	harnesses := []string{"claude_code", "codex", "openclaw"}
	for _, h := range harnesses {
		t.Run(h, func(t *testing.T) {
			srv, _ := testServer(t)
			body, _ := json.Marshal(CreateSessionRequest{Harness: h})
			req := httptest.NewRequest("POST", "/sessions", bytes.NewBuffer(body))
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("status = %d, want 201 for harness %s", w.Code, h)
			}
		})
	}
}

// --- Get Session ---

func TestGetSession(t *testing.T) {
	srv, st := testServer(t)
	st.CreateSession(&store.Session{ID: "s1", Harness: "claude_code", State: "idle", DisplayName: "My Session"})

	req := httptest.NewRequest("GET", "/sessions/s1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var sess store.Session
	json.NewDecoder(w.Body).Decode(&sess)
	if sess.DisplayName != "My Session" {
		t.Errorf("DisplayName = %q", sess.DisplayName)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("GET", "/sessions/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- Session History ---

func TestSessionHistory_Empty(t *testing.T) {
	srv, st := testServer(t)
	st.CreateSession(&store.Session{ID: "s1", Harness: "claude_code", State: "idle"})

	req := httptest.NewRequest("GET", "/sessions/s1/history", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var events []json.RawMessage
	json.NewDecoder(w.Body).Decode(&events)
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestSessionHistory_WithEvents(t *testing.T) {
	srv, st := testServer(t)
	st.CreateSession(&store.Session{ID: "s1", Harness: "claude_code", State: "running"})
	st.StoreEvent("s1", "stream", []byte(`{"type":"stream"}`))
	st.StoreEvent("s1", "result", []byte(`{"type":"result"}`))

	req := httptest.NewRequest("GET", "/sessions/s1/history", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var events []json.RawMessage
	json.NewDecoder(w.Body).Decode(&events)
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestSessionHistory_NotFound(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("GET", "/sessions/nope/history", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- Stop Session ---

func TestStopSession(t *testing.T) {
	srv, st := testServer(t)
	st.CreateSession(&store.Session{ID: "s1", Harness: "claude_code", State: "running"})

	req := httptest.NewRequest("POST", "/sessions/s1/stop", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var sess store.Session
	json.NewDecoder(w.Body).Decode(&sess)
	if sess.State != string(msg.SessionAborted) {
		t.Errorf("State = %q, want %q", sess.State, msg.SessionAborted)
	}
}

func TestStopSession_NotFound(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("POST", "/sessions/nope/stop", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- Interrupt Session ---

func TestInterruptSession_NotRunning(t *testing.T) {
	srv, st := testServer(t)
	st.CreateSession(&store.Session{ID: "s1", Harness: "claude_code", State: "idle"})

	req := httptest.NewRequest("POST", "/sessions/s1/interrupt", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestInterruptSession_NotFound(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("POST", "/sessions/nope/interrupt", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- Resume Session ---

func TestResumeSession_NotIdle(t *testing.T) {
	srv, st := testServer(t)
	st.CreateSession(&store.Session{ID: "s1", Harness: "claude_code", State: "running"})

	req := httptest.NewRequest("POST", "/sessions/s1/resume", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestResumeSession_NotFound(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("POST", "/sessions/nope/resume", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- Send Message ---

func TestSendMessage_NotFound(t *testing.T) {
	srv, _ := testServer(t)
	body := `{"message":"hello"}`
	req := httptest.NewRequest("POST", "/sessions/nope/send", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSendMessage_InvalidBody(t *testing.T) {
	srv, st := testServer(t)
	st.CreateSession(&store.Session{ID: "s1", Harness: "claude_code", State: "idle"})

	req := httptest.NewRequest("POST", "/sessions/s1/send", bytes.NewBufferString("{broken"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// --- Fork Session ---

func TestForkSession_NotFound(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("POST", "/sessions/nope/fork", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- Compact Session ---

func TestCompactSession_NotFound(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("POST", "/sessions/nope/compact", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- SSE Events endpoint ---

func TestSessionEvents_NotFound(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("GET", "/sessions/nope/events", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- generateID ---

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" || id2 == "" {
		t.Error("generated empty ID")
	}
	if id1 == id2 {
		t.Error("generated duplicate IDs")
	}
	if len(id1) < 5 {
		t.Errorf("ID too short: %q", id1)
	}
}
