package msg

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Server-managed session
// ──────────────────────────────────────────────────────────────────────────────

// SessionMode selects the I/O contract for a session at spawn time.
//
// SessionModeEvents (default) runs the harness in its current structured-
// events posture: stdin/stdout NDJSON between server and harness, normalized
// msg.Event stream over GET /sessions/{id}/events.
//
// SessionModePTY runs the harness's upstream CLI inside a pseudoterminal
// the caller attaches to over a WebSocket at GET /sessions/{id}/attach.
// Bytes pass through verbatim — no msg.Event derivation. Per-harness
// opt-in via bridge.PTYCapableHarness.
type SessionMode string

const (
	SessionModeEvents SessionMode = "events"
	SessionModePTY    SessionMode = "pty"
)

// SessionType categorizes a session by how it runs (interactive vs
// autonomous vs system infrastructure). Drives bridge-side behavior:
// completion notifications, SSE keepalive policy, default visibility in
// session lists. Required on Create.
//
// SessionType is the COARSE classification. The fine-grained "what is this
// session for" lives on Purpose; the "who created it" identity lives on
// Origin. The three are orthogonal:
//
//   - Type: interactive | autonomous | system | herald | external  (how it runs)
//   - Purpose: chat, autoworker, conformance, subagent, ...  (what it does)
//   - Origin: frontend-dash, autoworker, claudecode-adapter, ...  (who spawned it)
//
// Most autonomous services have Purpose == Origin (autoworker spawns its
// own kind of work); the values diverge for indirect spawns like subagents
// (Purpose=subagent, Origin=claudecode-adapter) and shared frontends
// (Purpose=chat, Origin=frontend-dash vs frontend-llmux).
//
// See MIGRATION-session-identity.md.
type SessionType string

const (
	// SessionTypeInteractive: human-in-the-loop. Frontend chat, CLI, mobile.
	SessionTypeInteractive SessionType = "interactive"

	// SessionTypeAutonomous: fire-and-forget agent runs, no human watching
	// live. Autoworker, scheduler-fired tasks, kanban dispatcher.
	SessionTypeAutonomous SessionType = "autonomous"

	// SessionTypeSystem: bridge-internal subsystems and meta-agents.
	// Renamer, subagents, kanban classifier, permission_prompt.
	SessionTypeSystem SessionType = "system"

	// SessionTypeHerald: an agent-initiated question/alert relayed to the user
	// in a session they didn't open (see the `ask` CLI). It runs unattended for
	// the relay turn — no human is watching to resolve a tool prehook — so it is
	// treated like autonomous for permission gating (isUnattendedSession), yet it
	// deliberately solicits the human, ending its turn in awaiting_user. Kept
	// distinct from autonomous so frontends can surface these as a "needs you"
	// inbox rather than mixing them with fire-and-forget worker runs.
	SessionTypeHerald SessionType = "herald"

	// SessionTypeExternal: ran outside the bridge entirely, and the bridge
	// imported it afterwards by scanning the harness's on-disk history (see
	// the -discover path). Nobody POSTed /sessions for it, so there is no
	// caller to have declared anything: no agent, no pid, no parent, no
	// spend, and an origin of "discovery" rather than a service name.
	//
	// It exists because the alternative was guessing. Discovery cannot tell
	// a `claude -p` one-shot from a human's terminal REPL, and for years it
	// resolved that by filing both as interactive — a shell smoke test
	// recorded as a human chat, and shown as one. External says the honest
	// thing, which is that the bridge did not run this and does not know.
	SessionTypeExternal SessionType = "external"
)

// sessionTypes is every valid SessionType. A type outside this set cannot be
// stored honestly — nothing downstream knows how to gate, file, or render it
// — so the server rejects it at create rather than persisting a value it will
// have to guess about later.
var sessionTypes = map[SessionType]bool{
	SessionTypeInteractive: true,
	SessionTypeAutonomous:  true,
	SessionTypeSystem:      true,
	SessionTypeHerald:      true,
	SessionTypeExternal:    true,
}

// ValidSessionType reports whether t is a known session type. Empty is not
// valid: "no type" and "some type we don't recognize" are both a caller that
// failed to classify its own session, and neither should reach the table.
func ValidSessionType(t SessionType) bool { return sessionTypes[t] }

// SessionTypes returns every valid session type, for callers that need to
// render or validate the set without importing the map.
func SessionTypes() []SessionType {
	return []SessionType{
		SessionTypeInteractive,
		SessionTypeAutonomous,
		SessionTypeSystem,
		SessionTypeHerald,
		SessionTypeExternal,
	}
}

// ManagedSession is a session as tracked by llm-bridge-server.
// This is the server's lightweight session management entity, distinct from
// [Session] which models rich agent-level state (usage, tasks, context).
//
// Identifier fields:
//   - SessionID: canonical session id. Caller-minted on Create or
//     server-minted (br_<nanos>) when omitted. Stable from creation.
//   - HarnessSessionID: the canonical harness session ID (e.g. CC UUID).
//     Rotates on resume/fork. Empty until the harness reports it on first
//     event. Will move to adapter-private storage in Phase II.C.
//
// Classification fields (orthogonal — see SessionType doc for the model):
//   - Type: interactive | autonomous | system | herald | external  (how it runs)
//   - Purpose: chat, autoworker, conformance, subagent, ...  (what it does)
//   - Origin: frontend-dash, autoworker, claudecode-adapter, ...  (who spawned it)
type ManagedSession struct {
	SessionID        string  `json:"session_id"`
	HarnessSessionID string  `json:"harness_session_id,omitempty"`
	DisplayName      string  `json:"display_name"`
	Harness          Harness `json:"harness"`
	InstanceID       string  `json:"instance_id,omitempty"`
	State            string  `json:"state"`
	PID              int     `json:"pid,omitempty"`
	AgentID          string  `json:"agent_id,omitempty"`
	// Deprecated: ParentID holds the FORK parent's HarnessSessionID (a harness
	// UUID, fed to --fork) — not a session id and not a general "parent". It is
	// superseded by ForkedFromSessionID and will be removed once the fork
	// plumbing resolves the harness id from the parent row (§21).
	ParentID string `json:"parent_id,omitempty"`
	// Orchestration lineage (TEAM-ORCHESTRATION.md §21; additive — set by the
	// team-orchestration layer, empty for ordinary sessions).
	ForkedFromSessionID    string          `json:"forked_from_session_id,omitempty"`    // the session this one was forked from (bridge_session_id) — the honest replacement for ParentID
	ManagerSessionID       string          `json:"manager_session_id,omitempty"`        // managing/parent session in the team tree (bridge_session_id); empty = top-level
	RootSessionID          string          `json:"root_session_id,omitempty"`           // top of this session's tree (bridge_session_id)
	Depth                  int             `json:"depth,omitempty"`                     // depth in the team tree (0 = root)
	ControlledBy           string          `json:"controlled_by,omitempty"`             // who controls this session's execution (message / steer / kill): "bridge" (bridge-server can act on it directly) | "harness" (coupled to a parent harness process — frontends disable direct actions)
	RefreshedFromSessionID string          `json:"refreshed_from_session_id,omitempty"` // the session this one continues after a context refresh (bridge_session_id; fresh reseed, no history)
	HarnessConfig          json.RawMessage `json:"harness_config,omitempty"`            // opaque harness-specific config
	Info                   *SessionInfo    `json:"info,omitempty"`                      // latest session info reported by the harness
	FolderName             string          `json:"folder_name"`                         // user-assigned sidebar folder; empty = unfiled. NOT omitempty: the session-list SSE upsert frame is a full canonical session that clients shallow-merge, so a cleared folder must serialize as "" explicitly — otherwise omitempty drops the field and the merge keeps a stale folder (e.g. header stays "Reopen" after un-archive until reload).
	Type                   SessionType     `json:"type"`                                // how this session runs; required, and rejected at create unless it is one of the values on SessionType
	Purpose                string          `json:"purpose"`                             // what this session is for (chat, autoworker, conformance, subagent, ...); required
	Origin                 string          `json:"origin"`                              // which service/script created this session (frontend-dash, autoworker, claudecode-adapter, ...); required
	Mode                   SessionMode     `json:"mode,omitempty"`                      // I/O mode picked at creation; empty = events (legacy default)

	// WorkingDir is the directory this session's harness runs in, and it is
	// the top of a four-level cascade: session > Instance.WorkingDir >
	// Machine.DefaultWorkingDir > whatever the spawning process already had.
	// Empty inherits the level below, exactly as the two instance-level
	// fields already do — there is no *string anywhere in this cascade and a
	// session must not introduce one.
	//
	// It is harness-neutral on purpose. Every harness runs somewhere, and
	// before this field the only way to say where was Instance.WorkingDir,
	// which is shared by every session on that instance.
	WorkingDir string `json:"working_dir,omitempty"`

	// Spend ceiling. MaxBudgetUSD is the server-side cap on this session's
	// derived API spend, set from CreateSessionRequest.MaxBudget at creation
	// or POST /sessions/{id}/config later. SpendUSD is the session's spend
	// against it.
	//
	// ⚠️ ZERO MEANS NO CEILING, not "stop immediately". That is the same
	// convention Claude Code's own --max-budget-usd flag uses, and every
	// caller on this box already relies on it (bridge-ui strips max_budget
	// <= 0 before sending). Read it as "unset", and never treat a zero
	// arriving from a client as a request to halt.
	MaxBudgetUSD float64 `json:"max_budget_usd,omitempty"`
	// SpendUSD is what this session has spent in total, in US dollars —
	// every API call it has made, across every harness process it has run
	// in, matching the latest APISpendTotalEvent.TotalUSD.
	//
	// ⚠️ Read "across every process" literally, because the earlier wording
	// here did not and that is what let a real hole sit open. This said the
	// derivation producing the running total "restarts at zero when the
	// process does", meaning bridge-server's — so a monotonic high-water
	// mark looked like enough. But the derivation is discarded whenever the
	// HARNESS process exits, which is every stop, crash, idle reap and
	// resume, and the totals under a previous run's high-water mark were
	// then never counted. Measured on one box 2026-08-01: 64 of 3,691
	// sessions had spent $1,174 more than this field recorded, one of them
	// $201.76 against a recorded $83.77. bridge-server now seeds a fresh
	// derivation from this field, so the running total continues the
	// session's history rather than starting a second one.
	SpendUSD float64 `json:"spend_usd,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Harness registry
// ──────────────────────────────────────────────────────────────────────────────

// HarnessAgent is one of the agents registered against a harness type.
// Surfaced by /harnesses/{name}/capabilities so frontends can offer a
// per-harness agent picker without re-querying agent-store.
type HarnessAgent struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"is_default,omitempty"`
}

// HarnessInfo describes a registered harness type and its capabilities.
//
// HookEvents lists the hook lifecycle events the bridge can register handlers
// against for this harness (e.g. Claude Code's "PreToolUse", "PostToolUse"…).
// Empty for harnesses with no registerable hook mechanism — harnesses that
// only emit observation-style lifecycle notifications or run agents remotely
// without local hook points.
type HarnessInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Emoji string `json:"emoji"`
	Image string `json:"image,omitempty"`
	// Tint is a sRGB hex (e.g. "#d97757") used by UIs to key chrome to a
	// specific harness — header washes, chips, etc. Empty when the harness
	// has no canonical color and the UI should fall through to its own
	// theme accent.
	Tint               string   `json:"tint,omitempty"`
	Available          bool     `json:"available"`
	Binary             string   `json:"binary,omitempty"`
	Capabilities       []string `json:"capabilities"`
	HookEvents         []string `json:"hook_events,omitempty"`
	SupportedProviders []string `json:"supported_providers,omitempty"`

	// SupportedPermissionModes lists the PermissionMode* values valid for
	// this harness. Always includes at least "ask" and "bypass" (every
	// harness can fall back to the prehook's universal gate). "auto" is
	// included only for harnesses whose start-param translation knows how
	// to express the auto-mode tool subset to the underlying agent.
	SupportedPermissionModes []string `json:"supported_permission_modes,omitempty"`
	// PTY reports whether this harness can run inside a pseudoterminal
	// (pty session mode). CLI harnesses with a real subprocess set this
	// true; HTTP-backed harnesses set it false. See bridge.PTYCapableHarness.
	PTY bool `json:"pty"`

	// SupportsDisableNetwork reports whether the harness can enforce a
	// "no outbound network" gate at the sandbox layer. Surfaced as a
	// checkbox alongside the permission-mode dropdown; the bridge writes
	// the boolean to HarnessConfig.disable_network and the harness reads
	// it at spawn time (codex maps to sandbox_workspace_write.network_access).
	// Harnesses without sandbox-level network gating set this false; the
	// bridge can still implement an equivalent via hook-store rules later.
	SupportsDisableNetwork bool `json:"supports_disable_network,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Credentials
// ──────────────────────────────────────────────────────────────────────────────

// Credential represents an API credential as exposed by the server.
// Sensitive fields are masked (api_key_masked, token_masked).
type Credential struct {
	ID           string `json:"id"`
	Provider     string `json:"provider"`
	Label        string `json:"label"`
	AuthType     string `json:"auth_type"`
	APIKeyMasked string `json:"api_key_masked,omitempty"`
	TokenMasked  string `json:"token_masked,omitempty"`
	Priority     int    `json:"priority"`
	Enabled      bool   `json:"enabled"`
	ExpiresAt    int64  `json:"expires_at"`
	ErrorCount   int    `json:"error_count,omitempty"`
	LastError    string `json:"last_error,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Bridge preferences
// ──────────────────────────────────────────────────────────────────────────────

// PermissionMode is the canonical enum for how the prehook gates tool
// calls. Set globally on BridgePrefs and overridable per session via
// HarnessConfig.permission_mode. Empty string means "use the global value
// for this session" (which itself defaults to PermissionModeAsk).
//
// Modes split into three families:
//   - Restrictive (Block All / Plan / Read): deny most tools without
//     consulting permission-store rules. Useful for review, planning,
//     or pausing.
//   - Rule-driven (Ask All / Ask / Auto): consult permission-store
//     (or always park) per the mode's policy. Ask is the default.
//   - Permissive (Bypass / Custom): allow everything (bypass), or
//     let the user define raw harness-specific knobs (custom).
//
// Each harness declares which modes it supports in HarnessInfo.
// SupportedPermissionModes. The prehook short-circuits on mode before
// any rule evaluation, so adding a new mode requires extending both
// the prehook table and any harness-side translation.
const (
	// PermissionModeBlockAll denies every tool call with a "blocked by
	// user" reason. The agent sees the deny in its tool result and can
	// keep reasoning — useful for pausing autonomy without ending the
	// session. Bypasses permission-store entirely.
	PermissionModeBlockAll = "block_all"

	// PermissionModePlan allows only planning tools (Read/Glob/Grep/
	// TodoWrite/LS/NotebookRead/ExitPlanMode); everything else is denied.
	// Maps to CC's native "plan" mode. Bypasses permission-store.
	PermissionModePlan = "plan"

	// PermissionModeRead allows read-only inspection (Read/Glob/Grep/LS/
	// NotebookRead); denies all writes and shell. Bypasses permission-store.
	PermissionModeRead = "read"

	// PermissionModeAskAll parks every tool call for human approval,
	// regardless of permission-store rules. Paranoia mode for high-stakes
	// sessions and demos. Skips the rule engine deliberately.
	PermissionModeAskAll = "ask_all"

	// PermissionModeAsk routes every novel tool call through permission-store
	// (and the human resolver on "ask" outcomes). The default.
	PermissionModeAsk = "ask"

	// PermissionModeAuto auto-allows a hardcoded set of low-impact tools
	// (reads + edits + planning) and gates everything else through the
	// permission-store ask flow. Bridge-defined — independent of any
	// harness's native "auto" semantics. Harnesses opt in by listing it in
	// HarnessInfo.SupportedPermissionModes.
	PermissionModeAuto = "auto"

	// PermissionModeBypass auto-allows every tool call. Tools tagged
	// HookSourceUserInput (AskUserQuestion etc.) still park for the human —
	// bypass is a permission grant, not an answer.
	PermissionModeBypass = "bypass"

	// PermissionModeCustom is the power-user escape hatch: the user
	// sets raw harness-specific knobs (e.g. codex approval_policy +
	// sandbox_mode) via HarnessConfig.permission_mode_custom. The prehook
	// still applies permission-store rules.
	PermissionModeCustom = "custom"
)

// HarnessDefaults stores per-harness configuration defaults.
type HarnessDefaults struct {
	Model          string   `json:"model,omitempty"`
	Effort         string   `json:"effort,omitempty"`
	MaxBudget      *float64 `json:"max_budget,omitempty"`
	DisabledTools  []string `json:"disabled_tools,omitempty"`
	PermissionMode string   `json:"permission_mode,omitempty"`
}

// BridgePrefs stores user preferences for the bridge dashboard.
type BridgePrefs struct {
	LastHarness    string                     `json:"last_harness,omitempty"`
	LastInstanceID string                     `json:"last_instance_id,omitempty"`
	LastSession    map[string]string          `json:"last_session,omitempty"`
	LastInstance   map[string]string          `json:"last_instance,omitempty"`
	Defaults       map[string]HarnessDefaults `json:"defaults,omitempty"`

	// PermissionMode is the global default mode applied to new sessions and
	// to legacy sessions that haven't been migrated to a per-session value.
	// One of PermissionModeAsk / PermissionModeAuto / PermissionModeBypass;
	// empty string is treated as PermissionModeAsk. Written exclusively via
	// /bridge/permission-mode so the partial-update merge in PUT
	// /bridge-prefs doesn't accidentally clobber it.
	PermissionMode string `json:"permission_mode,omitempty"`

	// BypassPermissions is the legacy boolean form of PermissionMode.
	//
	// Deprecated: read for back-compat (true → bypass, false/absent → ask),
	// never written. New code should read/write PermissionMode. Will be
	// removed once all live bridge-prefs.json files have a non-empty
	// PermissionMode value.
	BypassPermissions bool `json:"bypass_permissions,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Materialized messages (from log-store)
// ──────────────────────────────────────────────────────────────────────────────

// MaterializedMessage is a flattened message returned by the messages API.
// Assembled from raw events by log-store: stream deltas are concatenated,
// tool calls are collected, and the final ResultEvent becomes Meta.
type MaterializedMessage struct {
	ID               string             `json:"id,omitempty"`                 // canonical bridge MessageID
	HarnessMessageID string             `json:"harness_message_id,omitempty"` // harness-native id, if known
	Role             string             `json:"role"`
	Content          string             `json:"content"`
	Thinking         string             `json:"thinking,omitempty"`
	Tools            []MaterializedTool `json:"tools,omitempty"`
	Meta             *ResultEvent       `json:"meta,omitempty"`
	Events           []json.RawMessage  `json:"events,omitempty"`
	Timestamp        string             `json:"timestamp"`
	Done             bool               `json:"done"`
}

// MaterializedTool records a tool invocation within a materialized message.
type MaterializedTool struct {
	ToolID string          `json:"tool_id"`
	Tool   string          `json:"tool"`
	Input  json.RawMessage `json:"input,omitempty"`
	Output string          `json:"output,omitempty"`
	Error  bool            `json:"error,omitempty"`
}

// SessionAggregate is the per-session token/cost summary returned by
// GET /api/v1/sessions/aggregates. Computed by SUMming the result events
// stored for each session — no separate aggregate table. Model is the
// most recent value reported across the session's result events.
type SessionAggregate struct {
	SessionID    string  `json:"session_id"`
	Turns        int     `json:"turns"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	DurationMS   int64   `json:"duration_ms"`
	Model        string  `json:"model,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Health
// ──────────────────────────────────────────────────────────────────────────────

// HealthResponse is the server's health check response.
type HealthResponse struct {
	Status    string        `json:"status"`
	Harnesses []HarnessInfo `json:"harnesses"`
	Sessions  SessionCounts `json:"sessions"`
}

// SessionCounts is a breakdown of active sessions by state.
type SessionCounts struct {
	Running   int `json:"running"`
	Idle      int `json:"idle"`
	Completed int `json:"completed"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Conformance
// ──────────────────────────────────────────────────────────────────────────────

// ConformanceFeature is a capability that a harness may or may not support.
type ConformanceFeature string

// ConformanceTestResult records the outcome of a single feature test.
//
// Passed, Skipped and Unsupported are three different answers and a reader must
// not collapse them. Skipped means the test never ran. Unsupported means it ran,
// the harness answered, and it does not do this — a fact about the harness, not
// a defect in it. Failed (Passed false with neither flag) means it ran and the
// harness got it wrong. Grading Unsupported as Failed is what made a scaffold
// and a broken harness read alike on the matrix.
type ConformanceTestResult struct {
	Feature     ConformanceFeature `json:"feature"`
	Passed      bool               `json:"passed"`
	Skipped     bool               `json:"skipped,omitempty"`
	Unsupported bool               `json:"unsupported,omitempty"`
	Error       string             `json:"error,omitempty"`
	Duration    string             `json:"duration,omitempty"`
}

// ConformanceHarnessResult records all test results for a single harness.
type ConformanceHarnessResult struct {
	Harness  string                  `json:"harness"`
	Binary   string                  `json:"binary"`
	TestedAt time.Time               `json:"tested_at"`
	Results  []ConformanceTestResult `json:"results"`
	Summary  ConformanceSummary      `json:"summary"`
}

// ConformanceSummary counts test outcomes. The four counts are disjoint and sum
// to Total, so Unsupported has to be its own column rather than folded into
// Failed — see ConformanceTestResult.
type ConformanceSummary struct {
	Total       int `json:"total"`
	Passed      int `json:"passed"`
	Failed      int `json:"failed"`
	Skipped     int `json:"skipped"`
	Unsupported int `json:"unsupported"`
}

// ConformanceMatrix holds conformance results for all tested harnesses.
type ConformanceMatrix struct {
	GeneratedAt time.Time                  `json:"generated_at"`
	Harnesses   []ConformanceHarnessResult `json:"harnesses"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Request types
// ──────────────────────────────────────────────────────────────────────────────

// CreateSessionRequest is the request body for POST /sessions.
//
// SessionID is the caller-minted session identifier. When set, the bridge
// uses it as the session's primary key and rejects collisions with 409.
// When empty, the bridge mints one (br_<nanos>). Caller-mints lets workers
// (autoworker, scheduler, etc.) hold a synchronous handle for kanban links
// before the create round-trip returns; interactive callers may leave it
// empty and consume the server-minted value. Recommended caller format:
// ULID. See MIGRATION-session-identity.md.
//
// Type, Purpose, Origin classify the session along three orthogonal axes.
// All three are required:
//
//   - Type:    interactive | autonomous | system | herald | external  (how it runs)
//   - Purpose: chat, autoworker, conformance, subagent, ...  (what it does)
//   - Origin:  frontend-dash, autoworker, claudecode-adapter, ...  (who spawned it)
//
// Type and Origin are enforced: the server answers 400 invalid_session_type
// for a type outside the list above, and 400 missing_origin for an absent
// origin. Neither can be reconstructed once the caller is gone, and for years
// the absence of this check let unclassified sessions accumulate until a
// startup migration guessed at them and turned shell one-shots into human
// chats.
//
// Purpose is checked against the registry in purpose.go but never rejected —
// a new caller inventing a slug is how that list grows, so an unregistered
// purpose is stored, logged, and reported by session-taxonomy-guard. Keep it
// a category: what a particular run is about belongs in DisplayName.
//
// The server persists Purpose on the session and may auto-assign a folder
// based on it. The folder comes from the registry, overridable per purpose by
// the source_folders table and by LLMBRIDGE_PURPOSE_FOLDERS.
type CreateSessionRequest struct {
	Harness       Harness         `json:"harness"`
	SessionID     string          `json:"session_id,omitempty"`
	InstanceID    string          `json:"instance_id,omitempty"`
	DisplayName   string          `json:"display_name,omitempty"`
	AgentID       string          `json:"agent_id,omitempty"`
	AutoStart     bool            `json:"auto_start,omitempty"`
	Type          SessionType     `json:"type"`
	Purpose       string          `json:"purpose"`
	Origin        string          `json:"origin"`
	HarnessConfig json.RawMessage `json:"harness_config,omitempty"` // opaque harness-specific config, merged into start params
	// Mode selects events (default) or pty I/O. Pty requires the harness
	// to advertise SupportsPTY()==true; otherwise the server returns
	// 400 with error code "pty_unsupported".
	Mode SessionMode `json:"mode,omitempty"`
	// MaxBudget is the session's spend ceiling in US dollars, stored as
	// ManagedSession.MaxBudgetUSD and enforced server-side against the
	// session's derived API spend. Nil and 0 both mean no ceiling;
	// negative is rejected with 400 invalid_max_budget.
	//
	// This is the only way to give a ceiling to a session at the moment
	// it is created, which is the moment that matters for anything
	// nobody is watching. POST /sessions/{id}/config can change it later.
	MaxBudget *float64 `json:"max_budget,omitempty"`
	// WorkingDir is the directory this session's harness runs in, stored as
	// ManagedSession.WorkingDir and resolved at spawn ahead of the
	// instance's own WorkingDir and the machine's DefaultWorkingDir. Empty
	// inherits the instance, which is how every session behaved before this
	// field existed.
	//
	// Creation is the only place it can be set: the directory is read once,
	// when the harness process is spawned, so changing it afterwards would
	// silently do nothing until the session was restarted.
	WorkingDir string `json:"working_dir,omitempty"`
}

// SendMessageRequest is the request body for POST /sessions/{id}/send.
//
// Either Message (plain text) or Blocks (canonical content-block array for
// multimodal input — text, images, documents) carries the user's turn.
// The two are mutually exclusive: setting both is rejected at the server
// boundary so harnesses never have to disambiguate downstream.
//
// ClientRequestID is a caller-minted identifier for this turn. The bridge
// stamps it on every event it emits for the turn so producers can correlate
// server-side state with their own outbound request. Optional — callers that
// don't need correlation can leave it empty.
type SendMessageRequest struct {
	Message         string         `json:"message"`
	Blocks          []ContentBlock `json:"blocks,omitempty"`
	ClientRequestID string         `json:"client_request_id,omitempty"`
}

// ForkSessionRequest is the request body for POST /sessions/{id}/fork.
//
// Type / Purpose / Origin default to the parent session's values when omitted;
// callers may override (e.g. forking an interactive chat as an autonomous
// background investigation).
type ForkSessionRequest struct {
	DisplayName string      `json:"display_name,omitempty"`
	Type        SessionType `json:"type,omitempty"`
	Purpose     string      `json:"purpose,omitempty"`
	Origin      string      `json:"origin,omitempty"`
}

// CompactSessionRequest is the request body for POST /sessions/{id}/compact.
type CompactSessionRequest struct {
	Summary string `json:"summary,omitempty"`
}

// ConfigSessionRequest is the request body for POST /sessions/{id}/config.
//
// DisabledTools carries no omitempty, and that is load-bearing rather than an
// oversight. It is the whole set of names to exclude, not an addition to one,
// so an empty list is how a caller re-enables every tool — a real request with
// a real effect, and the opposite of saying nothing. omitempty cannot tell an
// empty slice from a nil one and drops both, which erased exactly that request:
// bridge-server's handleConfigSession decodes into this struct and re-marshals
// it as "config:<json>", so `{"disabled_tools":[]}` reached the harness as `{}`.
// Both receivers test the field for nil rather than for length — llm-bridge-jig's
// handleConfig and inber's handleBridgeConfig, each with a comment explaining
// why the distinction matters — and neither could ever see the value it was
// written to distinguish. jig answered "config: payload sets nothing"; inber
// answered "updated" and changed nothing.
//
// Without the tag a nil slice marshals to null, which decodes back to nil, so a
// request that names no tool set still reads as absent at both ends.
type ConfigSessionRequest struct {
	Model         string   `json:"model,omitempty"`
	Effort        string   `json:"effort,omitempty"`
	DisabledTools []string `json:"disabled_tools"`
	MaxBudget     *float64 `json:"max_budget,omitempty"`
}

// CreateInstanceRequest is the request body for POST /instances.
// MachineID is required — instances always belong to a machine. Use
// CreateMachineRequest first (or have a runner enroll) to obtain one.
type CreateInstanceRequest struct {
	HarnessType           string `json:"harness_type"`
	Name                  string `json:"name"`
	MachineID             string `json:"machine_id"`
	WorkingDir            string `json:"working_dir,omitempty"`
	MaxConcurrentSessions int    `json:"max_concurrent_sessions,omitempty"`
}

// CreateMachineRequest is the request body for POST /machines.
type CreateMachineRequest struct {
	Name              string    `json:"name"`
	Emoji             string    `json:"emoji,omitempty"`
	Hostname          string    `json:"hostname,omitempty"`
	OS                string    `json:"os,omitempty"`
	Arch              string    `json:"arch,omitempty"`
	Transport         Transport `json:"transport"`
	SSHUser           string    `json:"ssh_user,omitempty"`
	SSHKeyPath        string    `json:"ssh_key_path,omitempty"`
	SSHPort           int       `json:"ssh_port,omitempty"`
	DefaultWorkingDir string    `json:"default_working_dir,omitempty"`
	Notes             string    `json:"notes,omitempty"`
}

// UpdateMachineRequest patches a machine. Empty fields are left unchanged.
type UpdateMachineRequest struct {
	Name              string    `json:"name,omitempty"`
	Emoji             string    `json:"emoji,omitempty"`
	Hostname          string    `json:"hostname,omitempty"`
	OS                string    `json:"os,omitempty"`
	Arch              string    `json:"arch,omitempty"`
	Transport         Transport `json:"transport,omitempty"`
	SSHUser           string    `json:"ssh_user,omitempty"`
	SSHKeyPath        string    `json:"ssh_key_path,omitempty"`
	SSHPort           int       `json:"ssh_port,omitempty"`
	DefaultWorkingDir string    `json:"default_working_dir,omitempty"`
	Notes             string    `json:"notes,omitempty"`
}

// MintEnrollmentRequest creates a one-time enrollment passphrase for a runner.
// TTLSeconds defaults to 900 (15 min) on the server when zero.
type MintEnrollmentRequest struct {
	TTLSeconds int `json:"ttl_seconds,omitempty"`
}

// MintEnrollmentResponse carries the freshly-minted passphrase. The server
// stores only its hash; this is the only chance to capture the plaintext.
type MintEnrollmentResponse struct {
	Passphrase string    `json:"passphrase"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// EnrollRunnerRequest is the request body for POST /api/runner/enroll.
// The passphrase is a single-use credential; the rest is the runner's
// self-report of its environment, used to populate the auto-created
// Machine row.
type EnrollRunnerRequest struct {
	Passphrase         string             `json:"passphrase"`
	MachineName        string             `json:"machine_name,omitempty"` // defaults to hostname
	Hostname           string             `json:"hostname"`
	OS                 string             `json:"os"`
	Arch               string             `json:"arch"`
	User               string             `json:"user,omitempty"`
	WorkingDir         string             `json:"working_dir,omitempty"`
	AvailableHarnesses []HarnessAvailable `json:"available_harnesses,omitempty"`
	RunnerVersion      string             `json:"runner_version,omitempty"`
}

// EnrollRunnerResponse returns the durable per-machine token the runner
// will present on subsequent WS connections. Save it locally; the server
// stores only its hash.
type EnrollRunnerResponse struct {
	MachineID   string   `json:"machine_id"`
	MachineName string   `json:"machine_name"`
	RunnerToken string   `json:"runner_token"`
	InstanceIDs []string `json:"instance_ids"` // default instances auto-created for this machine
}

// BindCredentialRequest is the request body for POST /instances/{id}/credentials.
type BindCredentialRequest struct {
	CredentialID string `json:"credential_id"`
	Priority     int    `json:"priority"`
}

// RenameSessionRequest is the request body for POST /sessions/{id}/rename.
type RenameSessionRequest struct {
	DisplayName string `json:"display_name"`
}

// SourceFolderMapping is one row of the runtime purpose→folder map. Purpose
// is the session.purpose tag (autoworker, scheduler, conformance, ...); the
// mapping decides which sidebar folder a new session with that purpose lands
// in. Default=true means the value comes from the env-var defaults
// (LLMBRIDGE_SOURCE_FOLDERS) with no runtime override; Default=false means
// the row originates from the source_folders table. UpdatedAt is zero for
// env defaults. The env var and override table keep the legacy "source"
// naming for storage internals — see llm-bridge-server commit f03b058.
type SourceFolderMapping struct {
	Purpose    string    `json:"purpose"`
	FolderName string    `json:"folder_name"`
	Default    bool      `json:"default"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

// PutSourceFolderRequest is the request body for PUT /source-folders/{purpose}.
// FolderName must reference an existing folder (validated server-side).
// ApplyToExisting=true rebuckets prior sessions whose folder is empty or
// matches the previous effective folder for this purpose; manual moves are
// preserved.
type PutSourceFolderRequest struct {
	FolderName      string `json:"folder_name"`
	ApplyToExisting bool   `json:"apply_to_existing,omitempty"`
}

// SourceFolderApplyResult is the response payload for PUT/DELETE when
// ApplyToExisting was requested. Updated is the row count from the rebucket
// UPDATE.
type SourceFolderApplyResult struct {
	Mapping SourceFolderMapping `json:"mapping"`
	Updated int64               `json:"updated"`
}

// Who controls a session's execution — the values of ManagedSession.ControlledBy.
//
// ControlledByBridge means bridge-server owns the process and may message,
// resume or kill it. ControlledByHarness means the session is coupled to a
// parent harness process and has no process of its own: a harness subagent
// lives inside its parent, so "running with no process" is its normal state
// between the parent's frames, not a fault to repair.
//
// Every resume / message / kill path must check this. Resuming a
// harness-controlled session is actively destructive rather than merely
// useless: bridge-server cannot resume what it did not spawn, so the harness
// starts a FRESH agent, which replays the turn and runs tools unsupervised, and
// the new session id overwrites the row's harness_session_id — destroying the
// dedupe key discovery uses to converge on that row.
const (
	ControlledByBridge  = "bridge"
	ControlledByHarness = "harness"
)
