package msg

import "time"

// HookScope identifies the resolution layer a hook lives on.
//
// Hooks compose: a session inherits every enabled hook whose scope
// matches, across all three layers. When two scopes register a hook with
// the same (harness, event, matcher), the most specific scope wins:
// session > instance > global.
type HookScope string

const (
	HookScopeGlobal   HookScope = "global"   // applies to every session
	HookScopeInstance HookScope = "instance" // applies to every session on an instance
	HookScopeSession  HookScope = "session"  // applies to one specific session
)

// Hook is a registered hook handler. Each Hook describes one (event, matcher)
// pair on one harness, bound to a local shell command that runs when the
// underlying harness fires the event. The command receives the harness's
// native hook stdin and returns the harness's native hook stdout — the bridge
// does not translate either direction.
//
// For Claude Code: the command is wired into a synthesized settings JSON
// (passed to `claude` via --settings) when a session spawns. When CC invokes
// the hook, it shells out to the bridge server's /hooks/exec/{id} endpoint,
// which executes Command with stdin/stdout pass-through.
type Hook struct {
	ID      string  `json:"id"`      // ULID
	Harness Harness `json:"harness"` // claudecode, codex, ...
	Event   string  `json:"event"`   // harness-native event name (PreToolUse, etc.)

	// Matcher is the harness-native matcher that scopes the hook to a subset
	// of events (for Claude Code: a tool-name pattern like "Bash" or
	// "Edit|Write"). Empty matches everything the harness delivers for Event.
	Matcher string `json:"matcher,omitempty"`

	// Command is the local shell command that runs when the hook fires.
	// Executed via `sh -c <Command>`. Stdin is the harness-native hook input
	// payload; stdout is the harness-native response. No wrapping, no
	// translation — the handler speaks the harness's dialect directly.
	Command string `json:"command"`

	ScopeKind HookScope `json:"scope_kind"`
	// ScopeID is the instance id or session id the hook is bound to. Empty
	// for HookScopeGlobal.
	ScopeID string `json:"scope_id,omitempty"`

	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
