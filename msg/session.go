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

// StoredSession represents a session discovered on disk from a harness's
// native storage (e.g. ~/.claude/projects/ or ~/.codex/sessions/).
// These are sessions that exist outside the llm-bridge-server's own store.
type StoredSession struct {
	// HarnessSessionID is the harness-native session id (Claude session UUID,
	// Codex thread_id, Hermes dashboard id, ...). Required. Bridge-server
	// dedupes discovery results on this field, so it MUST be the harness's
	// own id — never a bridge-server-minted bridge_id.
	HarnessSessionID string `json:"harness_session_id"`

	// BridgeSessionID is bridge-server's stable session id, set when the
	// harness has already been adopted by bridge-server (i.e. state.db
	// recorded a `bridge_session_id` for this harness session). Empty for
	// cold-imported sessions that have never been bound to bridge-server.
	// When non-empty, bridge-server treats the session as already known and
	// skips it during discovery.
	BridgeSessionID string `json:"bridge_session_id,omitempty"`

	Harness   Harness   `json:"harness"`
	Project   string    `json:"project,omitempty"`    // project directory or context
	Prompt    string    `json:"prompt,omitempty"`     // initial prompt snippet
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`           // file modification time
	Path      string    `json:"path"`                 // on-disk path to session file
	TurnCount int       `json:"turn_count,omitempty"` // approximate number of turns

	// Source is a structural origin tag the harness adapter sets when the
	// on-disk layout classifies a session (e.g. CC stores Task()-spawned
	// subagents under <session>/subagents/agent-*.jsonl, so the claudecode
	// adapter tags those Source="subagent"). Empty when the adapter has no
	// structural signal — bridge-server then falls back to prompt-prefix
	// inference. Mirrors ManagedSession.Source so it flows straight into
	// LLMBRIDGE_SOURCE_FOLDERS bucketing.
	Source string `json:"source,omitempty"`
}
