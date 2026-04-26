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
)

// EventType classifies a harness event.
type EventType string

const (
	EventResult       EventType = "result"
	EventStream       EventType = "stream"
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
