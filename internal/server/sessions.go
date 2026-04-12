package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kayushkin/llm-bridge/internal/harness"
	"github.com/kayushkin/llm-bridge/internal/store"
	"github.com/kayushkin/llm-bridge/msg"
)

type CreateSessionRequest struct {
	Harness     string `json:"harness"`
	DisplayName string `json:"display_name,omitempty"`
	AgentID     string `json:"agent_id,omitempty"`
	SpawnerID   string `json:"spawner_id,omitempty"`
	AutoStart   bool   `json:"auto_start,omitempty"` // start harness immediately
}

type SendMessageRequest struct {
	Message string `json:"message"`
}

type ForkSessionRequest struct {
	DisplayName string `json:"display_name,omitempty"`
}

type CompactSessionRequest struct {
	Summary string `json:"summary,omitempty"`
}

type ConfigSessionRequest struct {
	Model         string   `json:"model,omitempty"`
	Effort        string   `json:"effort,omitempty"`
	DisabledTools []string `json:"disabled_tools,omitempty"`
	MaxBudget     *float64 `json:"max_budget,omitempty"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.store.ListSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if sessions == nil {
		sessions = []store.Session{}
	}
	writeJSON(w, sessions)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.store.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	writeJSON(w, sess)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	h := msg.Harness(req.Harness)
	if h != msg.HarnessClaudeCode && h != msg.HarnessCodex && h != msg.HarnessOpenClaw {
		http.Error(w, "invalid harness", http.StatusBadRequest)
		return
	}

	// Check harness is available if auto_start requested
	if req.AutoStart {
		if _, ok := harness.Available(h); !ok {
			http.Error(w, fmt.Sprintf("harness not available: %s", harness.BinaryName(h)), http.StatusServiceUnavailable)
			return
		}
	}

	sess := &store.Session{
		ID:          generateID(),
		DisplayName: req.DisplayName,
		Harness:     req.Harness,
		State:       string(msg.SessionIdle),
		AgentID:     req.AgentID,
		SpawnerID:   req.SpawnerID,
	}

	if err := s.store.CreateSession(sess); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Start harness subprocess if requested
	if req.AutoStart {
		if _, err := s.harness.Start(r.Context(), sess); err != nil {
			// Session created but harness failed to start
			s.store.UpdateSessionState(sess.ID, string(msg.SessionError))
			sess.State = string(msg.SessionError)
		} else {
			sess.State = string(msg.SessionRunning)
		}
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, sess)
}

func (s *Server) handleStopSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.store.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Kill the harness process
	if err := s.harness.Kill(id); err != nil {
		// Process might not be running, just update state
	}

	if err := s.store.UpdateSessionState(id, string(msg.SessionAborted)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sess.State = string(msg.SessionAborted)
	writeJSON(w, sess)
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.store.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Start harness if not running
	if s.harness.Get(id) == nil {
		if _, err := s.harness.Start(r.Context(), sess); err != nil {
			http.Error(w, fmt.Sprintf("failed to start harness: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Persist user message as an event for history
	userEvent := msg.Event{
		Type:      "user_message",
		SessionID: id,
		Timestamp: time.Now(),
		Result:    &msg.ResultEvent{Text: req.Message},
	}
	if data, err := json.Marshal(userEvent); err == nil {
		s.store.StoreEvent(id, "user_message", data)
	}

	if err := s.harness.Send(id, req.Message); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "sent"})
}

func (s *Server) handleSessionEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetSession(id); err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Subscribe to live events fan-out
	events := s.harness.Subscribe(id)

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.harness.Unsubscribe(id, events)
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Replay current turn's events from DB — covers the race where events
	// were persisted before this subscriber was registered.
	replayedTimestamps := make(map[string]bool)
	if stored, err := s.store.ListCurrentTurnEvents(id); err == nil && len(stored) > 0 {
		for _, raw := range stored {
			var ev msg.Event
			if json.Unmarshal(raw, &ev) == nil {
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, raw)
				// Track by timestamp+type for dedup against live events
				replayedTimestamps[ev.Timestamp.String()+string(ev.Type)] = true
			}
		}
		flusher.Flush()
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			s.harness.Unsubscribe(id, events)
			return
		case event, ok := <-events:
			if !ok {
				w.Write([]byte("event: close\ndata: {}\n\n"))
				flusher.Flush()
				return
			}
			// Skip events already sent via replay
			key := event.Timestamp.String() + string(event.Type)
			if replayedTimestamps[key] {
				delete(replayedTimestamps, key)
				continue
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}

func (s *Server) handleSessionHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetSession(id); err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	events, err := s.store.ListEvents(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if events == nil {
		events = []json.RawMessage{}
	}
	writeJSON(w, events)
}

func (s *Server) handleInterruptSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.store.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if sess.State != string(msg.SessionRunning) {
		http.Error(w, "session not running", http.StatusConflict)
		return
	}

	if err := s.harness.Stop(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.store.UpdateSessionState(id, string(msg.SessionIdle)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sess.State = string(msg.SessionIdle)
	writeJSON(w, sess)
}

func (s *Server) handleResumeSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.store.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if sess.State != string(msg.SessionIdle) {
		http.Error(w, "session not idle", http.StatusConflict)
		return
	}

	// Restart harness with resume flag
	if _, err := s.harness.Start(r.Context(), sess); err != nil {
		http.Error(w, fmt.Sprintf("failed to resume: %v", err), http.StatusInternalServerError)
		return
	}

	sess.State = string(msg.SessionRunning)
	writeJSON(w, sess)
}

func (s *Server) handleCompactSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.store.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req CompactSessionRequest
	json.NewDecoder(r.Body).Decode(&req)

	// Send compact command
	cmd := "compact"
	if req.Summary != "" {
		cmd = "compact:" + req.Summary
	}
	if err := s.harness.SendCommand(id, cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, sess)
}

func (s *Server) handleForkSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	parent, err := s.store.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req ForkSessionRequest
	json.NewDecoder(r.Body).Decode(&req)

	displayName := req.DisplayName
	if displayName == "" {
		displayName = parent.DisplayName + " (fork)"
	}

	forked := &store.Session{
		ID:          generateID(),
		DisplayName: displayName,
		Harness:     parent.Harness,
		State:       string(msg.SessionIdle),
		AgentID:     parent.AgentID,
		SpawnerID:   parent.SpawnerID,
		ParentID:    parent.ID,
	}

	if err := s.store.CreateSession(forked); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Start forked session (harness will use parent_id to fork state)
	if _, err := s.harness.Start(context.Background(), forked); err != nil {
		s.store.UpdateSessionState(forked.ID, string(msg.SessionError))
		forked.State = string(msg.SessionError)
	} else {
		forked.State = string(msg.SessionRunning)
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, forked)
}

func (s *Server) handleConfigSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.store.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req ConfigSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Send config command to harness (the harness binary interprets the JSON params)
	params, _ := json.Marshal(req)
	if err := s.harness.SendCommand(id, "config:"+string(params)); err != nil {
		// Process might not be running — store config for next start
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, sess)
}

func generateID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}
