package msg

import "time"

// SessionStatus is everything a status line needs to say what a session is
// doing right now, decided in one place: llm-bridge-server's derivation.
//
// It exists because the one-word SessionState was not enough to draw a status
// line from, so every client rebuilt the rest — which tool, which subagents,
// since when — from its own copy of the transcript, and then had to guess
// which of two unordered facts was newer when the transcript and the state
// disagreed. A status carries AsOf, the event row id it is current as of, so
// that question has one answer: the larger AsOf wins, whichever stream it
// arrived on and in whatever order.
//
// The same value travels two ways: as the body of an EventSessionStatus on the
// session's own event stream, and as ManagedSession.Status on the session row,
// which the session-list stream carries for every session whether or not
// anyone has it open.
type SessionStatus struct {
	State SessionState `json:"state"`

	// Generating says what the model is emitting while State is
	// model_generating: GeneratingThinking or GeneratingText. Empty in every
	// other state, and empty under model_generating until the first delta of
	// the turn says which it is.
	Generating string `json:"generating,omitempty"`

	// Tools are the tool calls issued and not yet answered, oldest first. A
	// call that spawned a subagent listed in Subagents is not repeated here.
	Tools []StatusTool `json:"tools,omitempty"`

	// Subagents are the harness tasks (subagents and backgrounded shells)
	// started and not yet at a terminal status, oldest first.
	Subagents []StatusSubagent `json:"subagents,omitempty"`

	// RateLimit is the provider's last word on this session's rate limit when
	// that word was anything other than "allowed". Advisory: a rejected limit
	// ends the turn with an error, so State says error and this says why and
	// until when.
	RateLimit *StatusRateLimit `json:"rate_limit,omitempty"`

	// TurnStartedAt is when the turn in flight opened. Zero between turns.
	TurnStartedAt time.Time `json:"turn_started_at,omitzero"`

	// ChangedAt is when State last changed.
	ChangedAt time.Time `json:"changed_at,omitzero"`

	// AsOf is the llm-bridge-server event row id this status is current as of
	// — the id space of the session stream's `id:` line. A consumer holding
	// two statuses for one session keeps the one with the larger AsOf.
	AsOf int64 `json:"as_of"`
}

// Values of SessionStatus.Generating.
const (
	GeneratingThinking = "thinking"
	GeneratingText     = "text"
)

// StatusTool is one tool call in flight.
type StatusTool struct {
	ToolID string `json:"tool_id"`
	Name   string `json:"name"`
	// Summary is one human line for the call's input — see ToolCallSummary.
	Summary   string    `json:"summary,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

// StatusSubagent is one harness task still running.
type StatusSubagent struct {
	TaskID       string `json:"task_id"`
	TaskType     string `json:"task_type,omitempty"`
	SubagentType string `json:"subagent_type,omitempty"`
	Description  string `json:"description,omitempty"`
	// LastToolName is the tool the task last reported running.
	LastToolName string    `json:"last_tool_name,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	// SessionID is the task's own bridge session, when one was promoted for
	// it. Empty for a backgrounded shell, which never gets one.
	SessionID string `json:"session_id,omitempty"`
}

// StatusRateLimit is a provider rate-limit report.
type StatusRateLimit struct {
	// Status is the provider's verdict: RateLimitAllowedWarning or
	// RateLimitRejected. RateLimitAllowed is never stored — it clears the field.
	Status string `json:"status"`
	// LimitType names the window that was hit (five_hour, seven_day, ...).
	LimitType string `json:"limit_type,omitempty"`
	// ResetsAt is when that window resets, unix seconds. Zero when unreported.
	ResetsAt int64 `json:"resets_at,omitempty"`
}

// Rate-limit verdicts carried on SystemEvent.RateLimitStatus.
const (
	RateLimitAllowed        = "allowed"
	RateLimitAllowedWarning = "allowed_warning"
	RateLimitRejected       = "rejected"
)

// SystemStatusCompacting is the SystemEvent.Status value a harness reports
// while it compacts context on its own initiative. An empty Status on a
// `status` subtype means whatever it was reporting has ended.
const SystemStatusCompacting = "compacting"

// Equal reports whether two statuses say the same thing, AsOf aside. The
// derivation emits a new status only when this is false, so a consumer is not
// told twice what it already knows.
func (s SessionStatus) Equal(o SessionStatus) bool {
	if s.State != o.State || s.Generating != o.Generating ||
		!s.TurnStartedAt.Equal(o.TurnStartedAt) || !s.ChangedAt.Equal(o.ChangedAt) ||
		len(s.Tools) != len(o.Tools) || len(s.Subagents) != len(o.Subagents) {
		return false
	}
	if (s.RateLimit == nil) != (o.RateLimit == nil) {
		return false
	}
	if s.RateLimit != nil && *s.RateLimit != *o.RateLimit {
		return false
	}
	for i := range s.Tools {
		a, b := s.Tools[i], o.Tools[i]
		if a.ToolID != b.ToolID || a.Name != b.Name || a.Summary != b.Summary || !a.StartedAt.Equal(b.StartedAt) {
			return false
		}
	}
	for i := range s.Subagents {
		a, b := s.Subagents[i], o.Subagents[i]
		if a.TaskID != b.TaskID || a.TaskType != b.TaskType || a.SubagentType != b.SubagentType ||
			a.Description != b.Description || a.LastToolName != b.LastToolName ||
			a.SessionID != b.SessionID || !a.StartedAt.Equal(b.StartedAt) {
			return false
		}
	}
	return true
}
