package msg

import (
	"encoding/json"
	"time"
)

// Session represents an agent session across any harness.
type Session struct {
	ID      string       `json:"id"`
	Harness Harness      `json:"harness"`
	State   SessionState `json:"state"`

	Agent        string `json:"agent,omitempty"`
	Orchestrator string `json:"orchestrator,omitempty"`
	Model        string `json:"model,omitempty"`

	CreatedAt  time.Time  `json:"created_at"`
	LastActive time.Time  `json:"last_active"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`

	ContextTokens int `json:"context_tokens,omitempty"`
	ContextLimit  int `json:"context_limit,omitempty"`

	TotalUsage TokenUsage `json:"total_usage"`
	TotalCost  *Cost      `json:"total_cost,omitempty"`
	TurnCount  int        `json:"turn_count"`

	// Config is harness-specific session configuration (json.RawMessage).
	Config json.RawMessage `json:"config,omitempty"`

	Tasks         []SessionTask `json:"tasks,omitempty"`
	ActiveRequest string        `json:"active_request,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// SessionTask tracks a task within a session.
type SessionTask struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"active_form,omitempty"`
}
