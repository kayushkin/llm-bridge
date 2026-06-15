package msg

// Provider identifies which LLM API produced or will consume a message.
type Provider string

const (
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenAI     Provider = "openai"
	ProviderGemini     Provider = "gemini"
	ProviderOpenRouter Provider = "openrouter"
)

// Role is the normalized message role across all providers.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// NativeRole maps to provider-specific role strings.
type NativeRole struct {
	Provider Provider `json:"provider"`
	Value    string   `json:"value"` // e.g. "model" for Gemini, "developer" for OpenAI o-series
}

// StopReason is the normalized reason a completion ended.
type StopReason string

const (
	StopEnd           StopReason = "end"
	StopMaxTokens     StopReason = "max_tokens"
	StopToolUse       StopReason = "tool_use"
	StopStopSequence  StopReason = "stop_sequence"
	StopSafety        StopReason = "safety"
	StopContentFilter StopReason = "content_filter"
	StopRecitation    StopReason = "recitation"
	StopError         StopReason = "error"
)

// Harness identifies which agent harness produced an event.
type Harness string

const (
	HarnessClaudeCode Harness = "claude_code"
	HarnessCodex      Harness = "codex"
	HarnessOpenClaw   Harness = "openclaw"
	HarnessInber      Harness = "inber"
	HarnessHermes     Harness = "hermes"
	HarnessAider      Harness = "aider"
	HarnessGoose      Harness = "goose"
	HarnessAutohand   Harness = "autohand"
	HarnessJig        Harness = "jig"
	HarnessDexto      Harness = "dexto"
	HarnessCommander  Harness = "commander"
	HarnessNanoClaw   Harness = "nanoclaw"
	HarnessCline      Harness = "cline"
	HarnessRooCode    Harness = "roo_code"
	HarnessKiloCode   Harness = "kilo_code"
	HarnessOpenCode   Harness = "opencode"
	HarnessForgecode  Harness = "forgecode"
	HarnessGemini     Harness = "gemini"
	HarnessCopilotCLI Harness = "copilot_cli"
	// HarnessMock is a test/deploy-verification harness. Its wrapper binary
	// (`llm-bridge-mock`, ships from llm-bridge-server/cmd/mock-harness)
	// speaks the full NDJSON protocol but makes no LLM calls, so it can drive
	// end-to-end deploy smoke tests without any provider credentials.
	HarnessMock Harness = "mock"
)

// AllHarnesses is the canonical list of known harness identifiers, in the
// order callers should iterate when surfacing them in UIs or capability lists.
var AllHarnesses = []Harness{
	HarnessClaudeCode,
	HarnessCodex,
	HarnessOpenClaw,
	HarnessInber,
	HarnessHermes,
	HarnessAider,
	HarnessGoose,
	HarnessAutohand,
	HarnessJig,
	HarnessDexto,
	HarnessCommander,
	HarnessNanoClaw,
	HarnessCline,
	HarnessRooCode,
	HarnessKiloCode,
	HarnessOpenCode,
	HarnessForgecode,
	HarnessGemini,
	HarnessCopilotCLI,
	HarnessMock,
}

// HarnessShortName returns the suffix used in wrapper-binary filenames
// (e.g. "claudecode" for HarnessClaudeCode). Distinct from string(h)
// because wrapper binaries omit underscores. Used as the `name` query
// param when the runner fetches missing wrappers from
// <bridge>/api/runner/binary?name=claudecode&os=…&arch=…. Returns "" for
// unknown harness types.
func HarnessShortName(h Harness) string {
	bin := HarnessBinaryName(h)
	if bin == "" {
		return ""
	}
	const prefix = "llm-bridge-"
	if len(bin) <= len(prefix) || bin[:len(prefix)] != prefix {
		return ""
	}
	return bin[len(prefix):]
}

// HarnessBinaryName returns the expected wrapper-binary filename for a
// harness type. Callers (server, runner, install scripts) use this to look
// up the binary on PATH. Returns "" for unknown harness types.
func HarnessBinaryName(h Harness) string {
	switch h {
	case HarnessClaudeCode:
		return "llm-bridge-claudecode"
	case HarnessCodex:
		return "llm-bridge-codex"
	case HarnessOpenClaw:
		return "llm-bridge-openclaw"
	case HarnessInber:
		return "llm-bridge-inber"
	case HarnessHermes:
		return "llm-bridge-hermes"
	case HarnessAider:
		return "llm-bridge-aider"
	case HarnessGoose:
		return "llm-bridge-goose"
	case HarnessAutohand:
		return "llm-bridge-autohand"
	case HarnessJig:
		return "llm-bridge-jig"
	case HarnessDexto:
		return "llm-bridge-dexto"
	case HarnessCommander:
		return "llm-bridge-commander"
	case HarnessNanoClaw:
		return "llm-bridge-nanoclaw"
	case HarnessCline:
		return "llm-bridge-cline"
	case HarnessRooCode:
		return "llm-bridge-roocode"
	case HarnessKiloCode:
		return "llm-bridge-kilocode"
	case HarnessOpenCode:
		return "llm-bridge-opencode"
	case HarnessForgecode:
		return "llm-bridge-forgecode"
	case HarnessGemini:
		return "llm-bridge-gemini"
	case HarnessCopilotCLI:
		return "llm-bridge-copilotcli"
	case HarnessMock:
		return "llm-bridge-mock"
	default:
		return ""
	}
}

// EventType classifies a harness event.
type EventType string

const (
	EventResult       EventType = "result"
	EventStream       EventType = "stream"
	EventBlock        EventType = "block"
	EventToolCall     EventType = "tool_call"
	EventToolResult   EventType = "tool_result"
	EventThinking     EventType = "thinking"
	EventSystem       EventType = "system"
	EventApproval     EventType = "approval"
	EventError        EventType = "error"
	EventSessionState EventType = "session_state"
	EventPlan         EventType = "plan"
	EventSessionInfo  EventType = "session_info"
	EventUserMessage  EventType = "user_message"
	EventHook         EventType = "hook"
	// EventAPICall is one upstream model API call observed by the harness —
	// per call, not per turn. Surfaces auxiliary calls (e.g. Claude Code's
	// session-title generation, prompt-suggestion calls in TUI mode) that
	// don't appear in turn-result aggregations. Harnesses that have access
	// to per-call telemetry (currently Claude Code via OpenTelemetry)
	// populate this; others may not.
	EventAPICall      EventType = "api_call"
	// Convenience events — derived centrally by llm-bridge-server from
	// the raw event stream. Distinct from EventSessionState (the harness
	// lifecycle) because agent_state is the coarser UI projection. See
	// msg/CONVENIENCE-EVENTS.md.
	EventAgentState     EventType = "agent_state"
	EventUsageTotal     EventType = "usage_total"
	EventTurnComplete   EventType = "turn_complete"
	// EventAPISpendTotal is the running per-API-call spend total, derived
	// by llm-bridge-server from the EventAPICall stream. Distinct from
	// EventUsageTotal: that one sums per-turn EventResult.Usage (the
	// harness's "this is what the user spent" figure), while this one
	// sums every upstream API call including auxiliary calls
	// (session-title generation, prompt-suggestion). The delta between
	// the two is the ambient overhead.
	EventAPISpendTotal EventType = "api_spend_total"
)

// SessionState represents the current state of an llm-bridge-managed
// session. Authoritative values are derived centrally by llm-bridge-server
// from the raw event stream plus server-only context (pause/abort calls,
// permission-store hook state, subprocess lifecycle, provider rate-limit
// signals). Harnesses no longer emit SessionState directly — they emit
// raw actions (tool_call, tool_result, block, hook, error) and the server
// computes the state.
type SessionState string

const (
	// Pre-flight.
	SessionStarting SessionState = "starting" // subprocess spawned, no first event yet

	// Active (agent is working).
	SessionModelGenerating SessionState = "model_generating" // assistant streaming tokens
	SessionToolRunning     SessionState = "tool_running"     // tool call in flight
	SessionCompacting      SessionState = "compacting"       // context compaction in progress

	// Blocked on user (action required).
	SessionAwaitingPermission SessionState = "awaiting_permission" // hook prompt open, blocking on approve/deny
	SessionAwaitingUser       SessionState = "awaiting_user"       // turn ended, expects user reply (best-effort heuristic)

	// Self-healing wait (no action; will resume).
	SessionRateLimited SessionState = "rate_limited" // provider backoff
	SessionPaused      SessionState = "paused"       // user-interrupted, can be resumed

	// Quiet but alive.
	SessionIdle SessionState = "idle" // between turns, no terminal signal

	// Terminal.
	SessionCompleted    SessionState = "completed"    // agent signaled work done
	SessionError        SessionState = "error"        // last activity ended in error
	SessionAborted      SessionState = "aborted"      // killed by user/system (intentional)
	SessionDisconnected SessionState = "disconnected" // process gone unexpectedly (no clean exit)

	// Deprecated: replaced by SessionModelGenerating / SessionToolRunning.
	// Kept valid during the migration window so harness packages that still
	// emit EventSessionState continue to compile. Will be removed once all
	// llm-bridge-* harnesses stop emitting SessionState.
	SessionRunning SessionState = "running"

	// Deprecated: renamed to SessionAwaitingPermission. Kept valid during
	// the migration window for the same reason as SessionRunning above.
	SessionWaitingApproval SessionState = "waiting_on_approval"
)

// IsActive reports whether the state implies a live (or expected-to-be-live)
// harness subprocess. Used by llm-bridge-server's startup reconcile and
// watchdog to identify sessions whose in-memory process is missing and that
// should be reset to idle / auto-resumed.
//
// Excludes states that are intentionally waiting (awaiting_user,
// awaiting_permission, paused) — those don't represent abandoned work,
// just blocked-on-human work — and terminal/quiet states.
func (s SessionState) IsActive() bool {
	switch s {
	case SessionStarting,
		SessionModelGenerating,
		SessionToolRunning,
		SessionCompacting,
		SessionRateLimited,
		SessionRunning:
		return true
	}
	return false
}

// IsBlockedOnUser reports whether the state is one where the harness is
// legitimately parked waiting on a human action rather than doing (or about
// to do) work: an open permission/question prompt (awaiting_permission, plus
// its deprecated alias waiting_on_approval) or a turn that ended soliciting a
// reply (awaiting_user).
//
// These are NOT abandoned work, so the idle reaper must not kill them even
// after a long quiet stretch — a human deliberating over a prompt emits no
// events. Reaping awaiting_permission would cancel the parked hook and
// auto-deny a live question/permission the user never got to answer; reaping
// awaiting_user would discard the warm process the expected reply lands in.
func (s SessionState) IsBlockedOnUser() bool {
	switch s {
	case SessionAwaitingPermission,
		SessionAwaitingUser,
		SessionWaitingApproval:
		return true
	}
	return false
}

// ActiveSessionStates returns every SessionState for which IsActive is true.
// Convenient for SQL `IN (...)` filters in stores that key on state strings.
func ActiveSessionStates() []SessionState {
	return []SessionState{
		SessionStarting,
		SessionModelGenerating,
		SessionToolRunning,
		SessionCompacting,
		SessionRateLimited,
		SessionRunning,
	}
}
