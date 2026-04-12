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
	if h != msg.HarnessClaudeCode && h != msg.HarnessCodex && h != msg.HarnessOpenClaw && h != msg.HarnessInber {
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

	// Check for Last-Event-ID for reconnection support
	lastEventID := r.Header.Get("Last-Event-ID")
	var lastRowID int
	if lastEventID != "" {
		fmt.Sscanf(lastEventID, "%d", &lastRowID)
	}

	// Replay events from DB — either since last event ID (reconnection)
	// or current turn events (initial connection).
	replayedIDs := make(map[int]bool)
	if lastRowID > 0 {
		// Reconnection: replay everything since last seen event
		if stored, err := s.store.ListEventsSinceID(id, lastRowID); err == nil {
			for _, ev := range stored {
				var parsed msg.Event
				if json.Unmarshal(ev.Data, &parsed) == nil {
					fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.RowID, parsed.Type, ev.Data)
					replayedIDs[ev.RowID] = true
				}
			}
			flusher.Flush()
		}
	} else {
		// Initial connection: replay current turn events
		if stored, err := s.store.ListCurrentTurnEventsWithIDs(id); err == nil && len(stored) > 0 {
			for _, ev := range stored {
				var parsed msg.Event
				if json.Unmarshal(ev.Data, &parsed) == nil {
					fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.RowID, parsed.Type, ev.Data)
					replayedIDs[ev.RowID] = true
				}
			}
			flusher.Flush()
		}
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
			data, _ := json.Marshal(event)
			// Get the row ID for this event (it was just persisted by the manager)
			rowID, _ := s.store.MaxEventID(id)
			if replayedIDs[rowID] {
				delete(replayedIDs, rowID)
				continue
			}
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", rowID, event.Type, data)
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

// MaterializedMessage is an assembled message for the /messages endpoint.
type MaterializedMessage struct {
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	Thinking  string            `json:"thinking,omitempty"`
	Tools     []MaterializedTool `json:"tools,omitempty"`
	Meta      *MaterializedMeta  `json:"meta,omitempty"`
	Timestamp string            `json:"timestamp"`
	Done      bool              `json:"done"`
}

type MaterializedTool struct {
	Tool   string `json:"tool"`
	Input  string `json:"input,omitempty"`
	Output string `json:"output,omitempty"`
	Error  bool   `json:"error,omitempty"`
}

type MaterializedMeta struct {
	InputTokens         int     `json:"input_tokens,omitempty"`
	OutputTokens        int     `json:"output_tokens,omitempty"`
	CacheReadTokens     int     `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens int     `json:"cache_creation_tokens,omitempty"`
	Cost                float64 `json:"cost,omitempty"`
	DurationMs          int64   `json:"duration_ms,omitempty"`
	NumTurns            int     `json:"num_turns,omitempty"`
	Model               string  `json:"model,omitempty"`
	ToolCalls           int     `json:"tool_calls,omitempty"`
}

func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetSession(id); err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	rawEvents, err := s.store.ListEvents(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	msgs := materializeMessages(rawEvents)
	writeJSON(w, msgs)
}

func materializeMessages(rawEvents []json.RawMessage) []MaterializedMessage {
	var msgs []MaterializedMessage
	var current *MaterializedMessage

	flushAssistant := func() {
		if current != nil {
			current.Done = true
			if current.Tools != nil {
				if current.Meta == nil {
					current.Meta = &MaterializedMeta{}
				}
				current.Meta.ToolCalls = len(current.Tools)
			}
			msgs = append(msgs, *current)
			current = nil
		}
	}

	for _, raw := range rawEvents {
		var ev msg.Event
		if json.Unmarshal(raw, &ev) != nil {
			continue
		}

		switch ev.Type {
		case "user_message":
			flushAssistant()
			text := ""
			if ev.Result != nil {
				text = ev.Result.Text
			}
			msgs = append(msgs, MaterializedMessage{
				Role:      "user",
				Content:   text,
				Timestamp: ev.Timestamp.Format(time.RFC3339),
				Done:      true,
			})

		case "result":
			if current == nil {
				current = newAssistantMsg(ev.Timestamp)
			}
			if ev.Result != nil {
				// Use final text from result (skips delta replay)
				if ev.Result.Text != "" {
					current.Content = ev.Result.Text
				}
				current.Meta = &MaterializedMeta{
					InputTokens:         ev.Result.Usage.InputTokens,
					OutputTokens:        ev.Result.Usage.OutputTokens,
					CacheReadTokens:     ev.Result.Usage.CacheReadTokens,
					CacheCreationTokens: ev.Result.Usage.CacheWriteTokens,
					DurationMs:          ev.Result.DurationMS,
					NumTurns:            ev.Result.NumTurns,
					Model:               ev.Result.Model,
				}
				if ev.Result.Cost != nil {
					current.Meta.Cost = ev.Result.Cost.TotalUSD
				}
			}
			flushAssistant()

		case "stream":
			// Only accumulate deltas if we don't have a result yet
			if current == nil {
				current = newAssistantMsg(ev.Timestamp)
			}

		case "thinking":
			if current == nil {
				current = newAssistantMsg(ev.Timestamp)
			}
			if ev.Thinking != nil {
				current.Thinking += ev.Thinking.Text
			}

		case "tool_call":
			if current == nil {
				current = newAssistantMsg(ev.Timestamp)
			}
			if ev.ToolCall != nil {
				inputStr := ""
				if ev.ToolCall.Input != nil {
					inputStr = string(ev.ToolCall.Input)
				}
				current.Tools = append(current.Tools, MaterializedTool{
					Tool:  ev.ToolCall.Name,
					Input: inputStr,
				})
			}

		case "tool_result":
			if current == nil {
				current = newAssistantMsg(ev.Timestamp)
			}
			if ev.ToolResult != nil {
				// Attach to matching tool call
				for i := len(current.Tools) - 1; i >= 0; i-- {
					if current.Tools[i].Tool == ev.ToolResult.Name && current.Tools[i].Output == "" {
						current.Tools[i].Output = ev.ToolResult.Output
						current.Tools[i].Error = ev.ToolResult.IsError
						break
					}
				}
			}
		}
	}

	// Flush any in-progress assistant message (still streaming)
	if current != nil {
		// For in-progress messages, we need to accumulate text from stream deltas
		// since there's no result event yet. Re-walk events for this turn.
		if current.Content == "" {
			current.Content = accumulateStreamText(rawEvents)
		}
		current.Done = false
		if current.Tools != nil {
			if current.Meta == nil {
				current.Meta = &MaterializedMeta{}
			}
			current.Meta.ToolCalls = len(current.Tools)
		}
		msgs = append(msgs, *current)
	}

	if msgs == nil {
		msgs = []MaterializedMessage{}
	}
	return msgs
}

func newAssistantMsg(ts time.Time) *MaterializedMessage {
	return &MaterializedMessage{
		Role:      "assistant",
		Timestamp: ts.Format(time.RFC3339),
	}
}

// accumulateStreamText walks events backward from the end to find
// text deltas for the current (unfinished) assistant turn.
func accumulateStreamText(rawEvents []json.RawMessage) string {
	// Find last user_message index
	lastUser := -1
	for i := len(rawEvents) - 1; i >= 0; i-- {
		var ev msg.Event
		if json.Unmarshal(rawEvents[i], &ev) == nil && ev.Type == "user_message" {
			lastUser = i
			break
		}
	}

	var text string
	start := lastUser + 1
	if start < 0 {
		start = 0
	}
	for i := start; i < len(rawEvents); i++ {
		var ev msg.Event
		if json.Unmarshal(rawEvents[i], &ev) != nil {
			continue
		}
		if ev.Type == "stream" && ev.Stream != nil && ev.Stream.Delta != nil {
			if ev.Stream.Delta.Type == "text_delta" {
				text += ev.Stream.Delta.Text
			}
		}
	}
	return text
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
