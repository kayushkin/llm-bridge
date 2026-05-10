// Package msg.
//
// The legacy Session and SessionTask types lived here as inber-era rich
// agent-state representations (usage, cost, tasks). They were retired in
// Phase II.B of MIGRATION-session-identity.md — confirmed zero callers
// across the ecosystem; per-session aggregates now live in log-store's
// sessions projection table, queried via /api/v1/sessions/aggregates.
//
// ManagedSession (msg/server.go) is the canonical session entity going
// forward.
package msg

import (
	"time"
)

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
