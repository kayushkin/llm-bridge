package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestNew_CreatesDBAndMigrates(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "test.db")

	st, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer st.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file not created: %v", err)
	}

	// Tables should exist
	var name string
	err = st.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'`).Scan(&name)
	if err != nil {
		t.Fatal("sessions table not created")
	}
	err = st.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='events'`).Scan(&name)
	if err != nil {
		t.Fatal("events table not created")
	}
}

// --- Session CRUD ---

func TestCreateAndGetSession(t *testing.T) {
	st := tempStore(t)

	sess := &Session{
		ID:          "sess_1",
		DisplayName: "Test Session",
		Harness:     "claude_code",
		State:       "idle",
		AgentID:     "agent_1",
		SpawnerID:   "spawner_1",
		ParentID:    "",
	}
	if err := st.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := st.GetSession("sess_1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.DisplayName != "Test Session" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Test Session")
	}
	if got.Harness != "claude_code" {
		t.Errorf("Harness = %q, want %q", got.Harness, "claude_code")
	}
	if got.State != "idle" {
		t.Errorf("State = %q, want %q", got.State, "idle")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestGetSession_NotFound(t *testing.T) {
	st := tempStore(t)
	_, err := st.GetSession("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestCreateSession_DuplicateID(t *testing.T) {
	st := tempStore(t)

	sess := &Session{ID: "dup", Harness: "codex", State: "idle"}
	if err := st.CreateSession(sess); err != nil {
		t.Fatal(err)
	}
	err := st.CreateSession(sess)
	if err == nil {
		t.Fatal("expected error on duplicate ID")
	}
}

func TestListSessions(t *testing.T) {
	st := tempStore(t)

	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "idle"})
	st.CreateSession(&Session{ID: "s2", Harness: "codex", State: "running"})
	st.CreateSession(&Session{ID: "s3", Harness: "openclaw", State: "idle"})

	all, err := st.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("ListSessions: got %d, want 3", len(all))
	}
}

func TestListSessionsByState(t *testing.T) {
	st := tempStore(t)

	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "idle"})
	st.CreateSession(&Session{ID: "s2", Harness: "codex", State: "running"})
	st.CreateSession(&Session{ID: "s3", Harness: "openclaw", State: "idle"})

	idle, err := st.ListSessionsByState("idle")
	if err != nil {
		t.Fatal(err)
	}
	if len(idle) != 2 {
		t.Fatalf("idle sessions: got %d, want 2", len(idle))
	}

	running, _ := st.ListSessionsByState("running")
	if len(running) != 1 {
		t.Fatalf("running sessions: got %d, want 1", len(running))
	}

	completed, _ := st.ListSessionsByState("completed")
	if len(completed) != 0 {
		t.Fatalf("completed sessions: got %d, want 0", len(completed))
	}
}

// --- Session Updates ---

func TestUpdateSessionState(t *testing.T) {
	st := tempStore(t)
	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "idle"})

	if err := st.UpdateSessionState("s1", "running"); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetSession("s1")
	if got.State != "running" {
		t.Errorf("State = %q, want %q", got.State, "running")
	}
}

func TestUpdateSessionState_NotFound(t *testing.T) {
	st := tempStore(t)
	err := st.UpdateSessionState("nope", "running")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestUpdateSessionPID(t *testing.T) {
	st := tempStore(t)
	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "running"})

	if err := st.UpdateSessionPID("s1", 12345); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetSession("s1")
	if got.PID != 12345 {
		t.Errorf("PID = %d, want 12345", got.PID)
	}
}

func TestUpdateSessionPID_NotFound(t *testing.T) {
	st := tempStore(t)
	err := st.UpdateSessionPID("nope", 1)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// --- Delete ---

func TestDeleteSession(t *testing.T) {
	st := tempStore(t)
	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "idle"})

	if err := st.DeleteSession("s1"); err != nil {
		t.Fatal(err)
	}
	_, err := st.GetSession("s1")
	if err != sql.ErrNoRows {
		t.Error("session still exists after delete")
	}
}

func TestDeleteSession_NotFound(t *testing.T) {
	st := tempStore(t)
	err := st.DeleteSession("nope")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// --- Events ---

func TestStoreAndListEvents(t *testing.T) {
	st := tempStore(t)
	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "running"})

	data1 := json.RawMessage(`{"type":"stream","text":"hello"}`)
	data2 := json.RawMessage(`{"type":"result","text":"done"}`)

	if err := st.StoreEvent("s1", "stream", data1); err != nil {
		t.Fatal(err)
	}
	if err := st.StoreEvent("s1", "result", data2); err != nil {
		t.Fatal(err)
	}

	events, err := st.ListEvents("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events: got %d, want 2", len(events))
	}
}

func TestEventCount(t *testing.T) {
	st := tempStore(t)
	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "running"})

	count, _ := st.EventCount("s1")
	if count != 0 {
		t.Fatalf("initial count = %d, want 0", count)
	}

	st.StoreEvent("s1", "stream", []byte(`{}`))
	st.StoreEvent("s1", "stream", []byte(`{}`))

	count, _ = st.EventCount("s1")
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestMaxEventID(t *testing.T) {
	st := tempStore(t)
	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "running"})

	maxID, _ := st.MaxEventID("s1")
	if maxID != 0 {
		t.Fatalf("initial maxID = %d, want 0", maxID)
	}

	st.StoreEvent("s1", "stream", []byte(`{}`))
	st.StoreEvent("s1", "result", []byte(`{}`))

	maxID, _ = st.MaxEventID("s1")
	if maxID < 2 {
		t.Fatalf("maxID = %d, want >= 2", maxID)
	}
}

func TestListEventsSince(t *testing.T) {
	st := tempStore(t)
	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "running"})

	st.StoreEvent("s1", "stream", []byte(`{"n":1}`))
	st.StoreEvent("s1", "stream", []byte(`{"n":2}`))
	st.StoreEvent("s1", "result", []byte(`{"n":3}`))

	// Get events after id 1 (should get 2 events)
	events, err := st.ListEventsSince("s1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events since 1: got %d, want 2", len(events))
	}
}

func TestListCurrentTurnEvents(t *testing.T) {
	st := tempStore(t)
	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "running"})

	st.StoreEvent("s1", "stream", []byte(`{"n":1}`))
	st.StoreEvent("s1", "user_message", []byte(`{"n":2}`))
	st.StoreEvent("s1", "stream", []byte(`{"n":3}`))
	st.StoreEvent("s1", "result", []byte(`{"n":4}`))

	events, err := st.ListCurrentTurnEvents("s1")
	if err != nil {
		t.Fatal(err)
	}
	// Should only get events after the last user_message, excluding user_message itself
	if len(events) != 2 {
		t.Fatalf("current turn events: got %d, want 2", len(events))
	}
}

func TestListEvents_EmptySession(t *testing.T) {
	st := tempStore(t)
	st.CreateSession(&Session{ID: "s1", Harness: "claude_code", State: "idle"})

	events, err := st.ListEvents("s1")
	if err != nil {
		t.Fatal(err)
	}
	if events != nil {
		t.Fatalf("expected nil events for empty session, got %d", len(events))
	}
}

// --- Session with ParentID ---

func TestSessionParentID(t *testing.T) {
	st := tempStore(t)

	st.CreateSession(&Session{ID: "parent", Harness: "claude_code", State: "idle"})
	st.CreateSession(&Session{ID: "child", Harness: "claude_code", State: "idle", ParentID: "parent"})

	child, _ := st.GetSession("child")
	if child.ParentID != "parent" {
		t.Errorf("ParentID = %q, want %q", child.ParentID, "parent")
	}
}
