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
	// Convenience events — derived centrally by llm-bridge-server from
	// the raw event stream. Distinct from EventSessionState (the harness
	// lifecycle) because agent_state is the coarser UI projection. See
	// msg/CONVENIENCE-EVENTS.md.
	EventAgentState   EventType = "agent_state"
	EventUsageTotal   EventType = "usage_total"
	EventTurnComplete EventType = "turn_complete"
)

// SessionState represents the current state of a harness session.
type SessionState string

const (
	SessionIdle            SessionState = "idle"
	SessionRunning         SessionState = "running"
	SessionCompleted       SessionState = "completed"
	SessionError           SessionState = "error"
	SessionAborted         SessionState = "aborted"
	SessionWaitingApproval SessionState = "waiting_on_approval"
)
