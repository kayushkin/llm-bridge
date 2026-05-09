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

// SessionType categorizes a session by how it is being used. Bridge-side
// behavior that depends on category (e.g. completion notifications, SSE
// keepalive policy) reads SessionType. Behavior that depends on which
// specific service originated the session reads Source.
//
// Will be required on Create — the field is currently optional in the wire
// format for backward compatibility while callers are updated. Enforcement
// flips on once all callers populate it. See MIGRATION-session-identity.md.
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
)

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
type ManagedSession struct {
	SessionID        string `json:"session_id"`
	HarnessSessionID string `json:"harness_session_id,omitempty"`
	DisplayName     string    `json:"display_name"`
	Harness         Harness   `json:"harness"`
	InstanceID      string    `json:"instance_id,omitempty"`
	State           string    `json:"state"`
	PID             int       `json:"pid,omitempty"`
	AgentID         string    `json:"agent_id,omitempty"`
	SpawnerID       string    `json:"spawner_id,omitempty"`
	ParentID        string          `json:"parent_id,omitempty"`
	HarnessConfig   json.RawMessage `json:"harness_config,omitempty"` // opaque harness-specific config
	Info            *SessionInfo    `json:"info,omitempty"`           // latest session info reported by the harness
	FolderName      string          `json:"folder_name,omitempty"`    // user-assigned sidebar folder; empty = unfiled
	Source          string          `json:"source,omitempty"`         // origin tag carried from the creator (e.g. "scheduler", "autoworker"); empty = interactive
	SessionType     SessionType     `json:"session_type,omitempty"`   // category of session; required on Create once enforcement flips on
	Mode            SessionMode     `json:"mode,omitempty"`           // I/O mode picked at creation; empty = events (legacy default)
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
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
	// PTY reports whether this harness can run inside a pseudoterminal
	// (pty session mode). CLI harnesses with a real subprocess set this
	// true; HTTP-backed harnesses set it false. See bridge.PTYCapableHarness.
	PTY bool `json:"pty"`
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
	LastHarness  string                     `json:"last_harness,omitempty"`
	LastInstanceID string                   `json:"last_instance_id,omitempty"`
	LastSession  map[string]string          `json:"last_session,omitempty"`
	LastInstance  map[string]string          `json:"last_instance,omitempty"`
	Defaults     map[string]HarnessDefaults `json:"defaults,omitempty"`

	// BypassPermissions, when true, makes new sessions launch with
	// --permission-mode bypassPermissions (CC never calls our MCP) AND
	// flips every live MCP into always-allow via /bridge/bypass-permissions.
	// The toggle is on dash /settings; written exclusively through that
	// endpoint (not through PUT /bridge-prefs partial updates) so the
	// bool merge ambiguity doesn't apply.
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
type ConformanceTestResult struct {
	Feature  ConformanceFeature `json:"feature"`
	Passed   bool               `json:"passed"`
	Skipped  bool               `json:"skipped,omitempty"`
	Error    string             `json:"error,omitempty"`
	Duration string             `json:"duration,omitempty"`
}

// ConformanceHarnessResult records all test results for a single harness.
type ConformanceHarnessResult struct {
	Harness   string                  `json:"harness"`
	Binary    string                  `json:"binary"`
	TestedAt  time.Time               `json:"tested_at"`
	Results   []ConformanceTestResult `json:"results"`
	Summary   ConformanceSummary      `json:"summary"`
}

// ConformanceSummary counts test outcomes.
type ConformanceSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
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
// Source is an origin tag set by the caller identifying what created this
// session (e.g. "scheduler", "autoworker"). Interactive UI callers leave it
// empty. The server persists it on the session and may auto-assign a folder
// based on the value (see LLMBRIDGE_SOURCE_FOLDERS).
//
// SessionType categorizes the session (interactive / autonomous / system).
// Currently optional for backward compatibility; will be required once all
// callers are updated. See MIGRATION-session-identity.md.
type CreateSessionRequest struct {
	Harness       Harness         `json:"harness"`
	SessionID     string          `json:"session_id,omitempty"`
	InstanceID    string          `json:"instance_id,omitempty"`
	DisplayName   string          `json:"display_name,omitempty"`
	AgentID       string          `json:"agent_id,omitempty"`
	SpawnerID     string          `json:"spawner_id,omitempty"`
	AutoStart     bool            `json:"auto_start,omitempty"`
	Source        string          `json:"source,omitempty"`
	SessionType   SessionType     `json:"session_type,omitempty"`
	HarnessConfig json.RawMessage `json:"harness_config,omitempty"` // opaque harness-specific config, merged into start params
	// Mode selects events (default) or pty I/O. Pty requires the harness
	// to advertise SupportsPTY()==true; otherwise the server returns
	// 400 with error code "pty_unsupported".
	Mode SessionMode `json:"mode,omitempty"`
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
type ForkSessionRequest struct {
	DisplayName string      `json:"display_name,omitempty"`
	SessionType SessionType `json:"session_type,omitempty"`
}

// CompactSessionRequest is the request body for POST /sessions/{id}/compact.
type CompactSessionRequest struct {
	Summary string `json:"summary,omitempty"`
}

// ConfigSessionRequest is the request body for POST /sessions/{id}/config.
type ConfigSessionRequest struct {
	Model         string   `json:"model,omitempty"`
	Effort        string   `json:"effort,omitempty"`
	DisabledTools []string `json:"disabled_tools,omitempty"`
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

// SourceFolderMapping is one row of the runtime source→folder map.
// Default=true means the value comes from the env-var defaults
// (LLMBRIDGE_SOURCE_FOLDERS) with no runtime override; Default=false means
// the row originates from the source_folders table. UpdatedAt is zero for
// env defaults.
type SourceFolderMapping struct {
	Source     string    `json:"source"`
	FolderName string    `json:"folder_name"`
	Default    bool      `json:"default"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

// PutSourceFolderRequest is the request body for PUT /source-folders/{source}.
// FolderName must reference an existing folder (validated server-side).
// ApplyToExisting=true rebuckets prior sessions whose folder is empty or
// matches the previous effective folder for this source; manual moves are
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
