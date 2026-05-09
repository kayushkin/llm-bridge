package msg

import (
	"encoding/json"
	"time"
)

// Event is the unified harness event type.
// All harness-specific events are normalized into this structure.
type Event struct {
	Type    EventType `json:"type"`
	Harness Harness   `json:"harness"`

	// BridgeSessionID is bridge-server's stable session id — the routing and
	// storage key. Bridges stamp every event with the same id bridge-server
	// passed in at session start. Required.
	BridgeSessionID string `json:"bridge_session_id"`

	// HarnessSessionID is the harness-internal id at the time of emission
	// (Codex thread_id, Claude session_id, ...). Rotates when the harness mints
	// a new internal id (resume, fork). Informational — bridge-server stores
	// the latest reported value on the session row but never routes on it.
	HarnessSessionID string `json:"harness_session_id,omitempty"`

	CompletionID string `json:"completion_id,omitempty"`

	// ClientRequestID is the caller-minted per-turn identifier forwarded from
	// SendMessageRequest. Stamped on every event the bridge emits for that
	// turn so producers can correlate server-side state with their outbound
	// request. Empty for events outside a turn (session_state, system, etc.)
	// and for turns where the caller didn't supply one.
	ClientRequestID string `json:"client_request_id,omitempty"`

	// TurnID is the bridge-server-minted identifier for an entire turn —
	// from a user_message through every event it produces (init,
	// task_progress, one or more assistant message bubbles, tool calls,
	// etc.) until the terminating result/error. Coarser than MessageID,
	// which scopes to a single message bubble. Empty between turns.
	TurnID string `json:"turn_id,omitempty"`

	// MessageID is the canonical bridge-server-assigned identifier for the
	// chat message this event belongs to (ULID, "msg_…"). Stable across SSE
	// reconnects and harness restarts. Empty for bookkeeping events
	// (system, session_state, session_info, harness_id_set) that do not
	// belong to a user-visible message bubble.
	MessageID string `json:"message_id,omitempty"`

	// HarnessMessageID is the harness-native identifier for the same logical
	// message (e.g. Claude Code's msg_…). Used for reconciliation when a
	// harness resumes and re-emits prior messages. Empty when the harness
	// does not surface one.
	HarnessMessageID string `json:"harness_message_id,omitempty"`

	Timestamp time.Time `json:"timestamp"`

	// Content — at most one of these will be populated based on Type.
	Result       *ResultEvent       `json:"result,omitempty"`
	Stream       *HarnessStream     `json:"stream,omitempty"`
	Block        *BlockEvent        `json:"block,omitempty"`
	ToolCall     *ToolCallEvent     `json:"tool_call,omitempty"`
	ToolResult   *ToolResultEvent   `json:"tool_result,omitempty"`
	Thinking     *ThinkingEvent     `json:"thinking,omitempty"`
	Plan         *PlanEvent         `json:"plan,omitempty"`
	System       *SystemEvent       `json:"system,omitempty"`
	Approval     *ApprovalEvent     `json:"approval,omitempty"`
	Error        *ErrorEvent        `json:"error,omitempty"`
	State        *StateEvent        `json:"state,omitempty"`
	Info         *SessionInfo       `json:"info,omitempty"`
	Hook         *HookEvent         `json:"hook,omitempty"`
	// Deprecated: AgentState was a coarser projection of SessionState. With
	// SessionState extended to 13 values, the projection is no longer needed.
	// Consumers should switch on State (StateEvent) directly. Field kept
	// during the migration window so existing emitters compile.
	AgentState   *AgentStateEvent   `json:"agent_state,omitempty"`
	UsageTotal   *UsageTotalEvent   `json:"usage_total,omitempty"`
	TurnComplete *TurnCompleteEvent `json:"turn_complete,omitempty"`

	// DerivedFrom lists the upstream event ids this event was synthesized
	// from, when llm-bridge-server (or a harness) emits a convenience event
	// derived from one or more raw events. Empty when the event is a
	// first-class observation rather than a derivation. Used by consumers
	// that want to retract or dedupe derived events.
	DerivedFrom []string `json:"derived_from,omitempty"`

	// Raw preserves the original event JSON from the harness for pass-through.
	Raw json.RawMessage `json:"raw,omitempty"`

	// Extensions holds bridge-specific typed data not yet in the canonical schema.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// HarnessMessageIDOf extracts the harness-native message id from an event,
// looking in the variant fields where harnesses typically place it.
// Returns "" when no harness id is present (true for thinking, for
// system/state/info events, and for harnesses that simply don't surface one).
func HarnessMessageIDOf(ev *Event) string {
	if ev == nil {
		return ""
	}
	if ev.Stream != nil && ev.Stream.MessageID != "" {
		return ev.Stream.MessageID
	}
	if ev.Block != nil && ev.Block.MessageID != "" {
		return ev.Block.MessageID
	}
	if ev.ToolCall != nil && ev.ToolCall.MessageID != "" {
		return ev.ToolCall.MessageID
	}
	if ev.ToolResult != nil && ev.ToolResult.MessageID != "" {
		return ev.ToolResult.MessageID
	}
	if ev.Result != nil && ev.Result.Message != nil && ev.Result.Message.ID != "" {
		return ev.Result.Message.ID
	}
	return ""
}

// ResultEvent is a completed harness run. Also reused as the payload for
// EventUserMessage events to record what the user sent: Text holds plain-text
// input; Blocks holds multimodal input (text/image/document/audio/video).
// The two are mutually exclusive at the request boundary.
type ResultEvent struct {
	Text             string          `json:"text"`
	Blocks           []ContentBlock  `json:"blocks,omitempty"`
	Message          *Message        `json:"message,omitempty"`
	IsError          bool            `json:"is_error,omitempty"`
	StructuredOutput json.RawMessage `json:"structured_output,omitempty"`

	Usage         TokenUsage   `json:"usage"`
	Cost          *Cost        `json:"cost,omitempty"`
	DurationMS    int64        `json:"duration_ms,omitempty"`
	DurationAPIMS int64        `json:"duration_api_ms,omitempty"`
	NumTurns      int          `json:"num_turns,omitempty"`
	APICalls      int          `json:"api_calls,omitempty"`
	Model         string       `json:"model,omitempty"`
	APICallUsages []TokenUsage `json:"api_call_usages,omitempty"`
	ToolEvents    []ToolSummary `json:"tool_events,omitempty"`
}

// ToolSummary records a tool invocation within a completed run.
type ToolSummary struct {
	Tool   string `json:"tool"`
	Input  string `json:"input,omitempty"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// HarnessStream is a streaming text/content event from the harness.
// Named HarnessStream to avoid collision with StreamEvent (API-level streaming).
//
// EventStream carries true incremental deltas (token chunks, partial JSON,
// thinking summary chunks). Whole finished content blocks ride EventBlock
// instead — see BlockEvent. A harness emits EventStream only when it has
// granular streaming enabled (e.g. Claude Code with --include-partial-messages).
type HarnessStream struct {
	Delta     *BlockDelta `json:"delta,omitempty"`
	MessageID string      `json:"message_id,omitempty"`
	Hidden    bool        `json:"hidden,omitempty"`
}

// BlockEvent carries one finished content block from a harness assistant
// message. Distinct from EventStream: EventBlock is one-shot per completed
// block (the block has stopped accumulating); EventStream is incremental.
//
// Block is the discriminated-union content carrier from msg/content.go,
// covering text, thinking, tool_use, tool_result, images, etc. — every
// content variant a harness might surface.
//
// MessageID is the harness-native id of the assistant message containing
// this block (e.g. Claude Code's msg_…), used for grouping multiple blocks
// from the same model turn into one chat bubble. Index matches the block's
// position in the original message's content array.
type BlockEvent struct {
	Index     int           `json:"index"`
	MessageID string        `json:"message_id,omitempty"`
	Block     *ContentBlock `json:"block"`
}

// ToolCallEvent is a tool being invoked.
//
// MessageID is the harness-native id of the assistant bubble that contained
// this tool_use block (e.g. Claude Code's msg_…). It's the same id the
// surrounding text/thinking events carry on HarnessStream.MessageID — so the
// bridge server can group tool_call events into their bubble without parsing
// the raw harness payload.
type ToolCallEvent struct {
	ToolID    string          `json:"tool_id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	MessageID string          `json:"message_id,omitempty"`
}

// ToolResultEvent is the output of a tool invocation.
//
// MessageID mirrors ToolCallEvent.MessageID — the harness-native id of the
// bubble that contained the tool_result block.
type ToolResultEvent struct {
	ToolID    string `json:"tool_id"`
	Name      string `json:"name"`
	Output    string `json:"output"`
	IsError   bool   `json:"is_error,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

// ThinkingEvent is a thinking/reasoning event.
type ThinkingEvent struct {
	Text    string `json:"text"`
	Subtype string `json:"subtype,omitempty"` // "", "summary"
}

// PlanEvent is a structured task-planning event (distinct from thinking).
type PlanEvent struct {
	Text string `json:"text"`
}

// SystemEvent is a system-level notification.
//
// ToolUseID / TaskID / Description / LastToolName are populated for the
// "task_progress" subtype (Claude Code's narration of an in-flight tool
// call). ToolUseID is the harness-native tool id (e.g. Anthropic's
// toolu_…) — the bridge server uses it to resolve the event back to the
// containing message bubble and stamp MessageID/HarnessMessageID.
type SystemEvent struct {
	Subtype      string `json:"subtype"`
	Message      string `json:"message,omitempty"`
	Attempt      int    `json:"attempt,omitempty"`
	MaxRetries   int    `json:"max_retries,omitempty"`
	RetryDelayMS int    `json:"retry_delay_ms,omitempty"`
	ErrorStatus  int    `json:"error_status,omitempty"`
	ErrorType    string `json:"error_type,omitempty"`
	ToolUseID    string `json:"tool_use_id,omitempty"`
	TaskID       string `json:"task_id,omitempty"`
	Description  string `json:"description,omitempty"`
	LastToolName string `json:"last_tool_name,omitempty"`
}

// ApprovalEvent represents a permission request or response.
//
// Deprecated: emit HookEvent with Source="permission_prompt" and the
// awaiting_resolution / completed phase pair instead. Kept this cycle so
// existing harnesses can transition; consumers should treat both as the same
// signal. Will be removed in a future release.
type ApprovalEvent struct {
	Action      string   `json:"action"`
	Status      string   `json:"status"`
	ToolName    string   `json:"tool_name,omitempty"`
	Detail      string   `json:"detail,omitempty"`
	Command     string   `json:"command,omitempty"`
	Path        string   `json:"path,omitempty"`
	Patch       string   `json:"patch,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
}

// ErrorEvent represents a harness-level error.
type ErrorEvent struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
}

// StateEvent represents a session state change.
type StateEvent struct {
	State    SessionState `json:"state"`
	Previous SessionState `json:"previous,omitempty"`
	Reason   string       `json:"reason,omitempty"`
}

// AgentState was a coarser UI-projection of session lifecycle (4 values).
//
// Deprecated: SessionState now carries the full UI vocabulary (13 values)
// that AgentState was projecting toward. Consumers should switch on
// SessionState directly via StateEvent. Type kept during the migration
// window so existing references compile; will be removed once
// llm-bridge-server stops emitting EventAgentState and consumers migrate.
type AgentState string

const (
	AgentStateIdle          AgentState = "idle"           // Deprecated: use SessionIdle
	AgentStateAwaitingInput AgentState = "awaiting_input" // Deprecated: use SessionAwaitingUser or SessionAwaitingPermission
	AgentStateToolRunning   AgentState = "tool_running"   // Deprecated: use SessionToolRunning or SessionModelGenerating
	AgentStateError         AgentState = "error"          // Deprecated: use SessionError
)

// AgentStateEvent is the body for EventAgentState.
//
// Deprecated: see AgentState above.
type AgentStateEvent struct {
	State    AgentState `json:"state"`
	Previous AgentState `json:"previous,omitempty"`
	Reason   string     `json:"reason,omitempty"` // free-form, e.g. "tool=Bash", "approval_required"
}

// UsageTotalEvent is the body for EventUsageTotal. Carries the running
// session totals, not a delta — emitted once after every ResultEvent
// fan-out. ContextTokens / ContextLimit on Usage are last-value-wins
// (state, not consumption); all other TokenUsage fields are summed.
type UsageTotalEvent struct {
	Usage TokenUsage `json:"usage"`          // cumulative across every result event in this session
	Cost  *Cost      `json:"cost,omitempty"` // sum where reported; nil until any priced turn lands
	Turns int        `json:"turns"`          // how many completed turns contributed
}

// TurnCompleteEvent is the body for EventTurnComplete. Emitted once per
// turn, immediately after the terminating ResultEvent (or ErrorEvent for
// error-terminated turns) is fanned out. TurnID matches the originating
// user_message's turn_id.
type TurnCompleteEvent struct {
	TurnID       string        `json:"turn_id"`                 // matches the originating user_message's turn_id
	FinalMessage string        `json:"final_message,omitempty"` // last assistant text in the turn (from ResultEvent.Text)
	ToolCalls    []ToolSummary `json:"tool_calls,omitempty"`    // every tool_call/tool_result pair seen in this turn
	UsageDelta   TokenUsage    `json:"usage_delta"`             // this turn's contribution
	Cost         *Cost         `json:"cost,omitempty"`
	DurationMS   int64         `json:"duration_ms"` // wall-clock from user_message → result
	IsError      bool          `json:"is_error,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// SessionInfo describes what the harness knows about its session at start:
// the system prompt it was configured with, the working directory, and the
// tools / slash commands / sub-agents / MCP servers the underlying agent
// reports as available. Emitted by the harness as an EventSessionInfo after
// the agent's initial handshake, and persisted on ManagedSession so it can
// be retrieved via GET /sessions/{id} without replaying events.
type SessionInfo struct {
	SystemPrompt       string           `json:"system_prompt,omitempty"`
	AppendSystemPrompt string           `json:"append_system_prompt,omitempty"`
	WorkingDir         string           `json:"working_dir,omitempty"`
	Model              string           `json:"model,omitempty"`
	PermissionMode     string           `json:"permission_mode,omitempty"`
	Tools              []ToolInfo       `json:"tools,omitempty"`
	SlashCommands      []string         `json:"slash_commands,omitempty"`
	Agents             []string         `json:"agents,omitempty"`
	Skills             []string         `json:"skills,omitempty"`
	MCPServers         []MCPServerInfo  `json:"mcp_servers,omitempty"`
}

// ToolInfo names a tool the agent has available. Description is optional —
// the harness only reports what the underlying agent exposes; the UI may
// enrich with its own descriptions.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// MCPServerInfo describes an MCP server connection reported by the agent.
type MCPServerInfo struct {
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

// HookEvent records a harness hook lifecycle event.
//
// Three sources produce HookEvents, discriminated by Source / HookID:
//
//  1. Harness-native observation (Source empty or "hook", HookID empty) — the
//     underlying harness emits its own hook lifecycle notifications (e.g.
//     Claude Code with --include-hook-events) and the bridge translates them.
//     The bridge has no control over these hooks.
//  2. Bridge-registered invocation (Source empty or "hook", HookID set) — a
//     hook registered with the hook-store fires via the harness's native
//     mechanism, which shells out to the bridge server's /hooks/exec/{id}
//     endpoint.
//  3. Permission-prompt invocation (Source="permission_prompt") — the harness
//     wired a permission-prompt callback (e.g. Claude Code's
//     --permission-prompt-tool MCP server) and is consulting the bridge for a
//     tool-use decision. Functionally a synthetic PreToolUse hook whose
//     resolver is human (or shell or auto) per the bridge's settings.
//
// Input and Output are raw JSON, passed through in the underlying harness's
// native dialect (e.g. Claude Code's stdin/stdout hook protocol). Layers are
// transparent: no translation into a neutral schema.
type HookEvent struct {
	// Event names the hook lifecycle point (e.g. "PreToolUse", "PostToolUse",
	// "UserPromptSubmit"). Values match the underlying harness's native event
	// names — no rewriting.
	Event string `json:"event"`

	// Matcher is the harness-native matcher that selected this hook (for
	// Claude Code: the tool-name pattern like "Bash" or "Edit|Write"). Empty
	// when the hook applies to all events of its type or when the harness
	// does not expose a matcher concept.
	Matcher string `json:"matcher,omitempty"`

	// ToolName is the tool the hook fired against, when applicable
	// (PreToolUse, PostToolUse). Empty for non-tool-scoped events.
	ToolName string `json:"tool_name,omitempty"`

	// HookID is the bridge-registry id of the hook, when the event came from
	// a hook registered with the bridge's hook-store. Empty for harness-native
	// (observation-only) hooks that the bridge did not register.
	HookID string `json:"hook_id,omitempty"`

	// Source discriminates the emission origin: "hook" (harness-native or
	// bridge-registered hook; default when empty) or "permission_prompt"
	// (harness's permission-prompt callback). Determines which decision
	// protocol applies on resolution.
	Source string `json:"source,omitempty"`

	// Phase is the lifecycle point: "started", "progress", "completed", or
	// "awaiting_resolution". Most hook invocations produce a "started" and a
	// "completed" event; long-running hooks may emit "progress" between.
	// Hooks that defer to a human resolver (or any out-of-band resolver)
	// emit "awaiting_resolution" between "started" and "completed" — the
	// matching "completed" event carries Resolution and shares RequestID.
	Phase string `json:"phase"`

	// RequestID correlates an awaiting_resolution event with its eventual
	// completed event. Set on phase="awaiting_resolution" and on the matching
	// phase="completed" emitted once the resolver decides. Empty for hooks
	// that complete synchronously without an external resolver.
	RequestID string `json:"request_id,omitempty"`

	// Input is the raw JSON the harness passed to the hook (stdin payload for
	// Claude Code; tool input JSON for permission-prompt). Populated on
	// Phase="started" and Phase="awaiting_resolution". Pass-through.
	Input json.RawMessage `json:"input,omitempty"`

	// Output is the raw JSON the hook returned to the harness (stdout payload
	// for Claude Code). Populated on Phase="completed". Pass-through.
	Output json.RawMessage `json:"output,omitempty"`

	// Decision summarizes the hook's effect on the tool call when the harness
	// surfaces one (e.g. Claude Code's "allow", "deny", "modify", "continue").
	// Empty when the harness does not report a decision.
	Decision string `json:"decision,omitempty"`

	// Resolution captures how an awaiting_resolution hook was resolved.
	// Populated on Phase="completed" when a prior awaiting_resolution event
	// existed for the same RequestID. Nil for hooks that completed without
	// going through awaiting_resolution.
	Resolution *HookResolution `json:"resolution,omitempty"`

	// ExitCode is the exit status of the hook command when the harness
	// reports one (Claude Code does, on Phase="completed").
	ExitCode int `json:"exit_code,omitempty"`

	// DurationMS is the wall-clock time the hook took to run. Populated on
	// Phase="completed" when the harness reports timing.
	DurationMS int64 `json:"duration_ms,omitempty"`

	// Error is a non-empty message when the hook failed to execute or
	// returned malformed output. An errored hook may still carry Output if
	// the harness produced something before failing.
	Error string `json:"error,omitempty"`
}

// HookResolution captures the decision that closed an awaiting_resolution hook.
type HookResolution struct {
	// Behavior is "allow" or "deny". Required.
	Behavior string `json:"behavior"`

	// UpdatedInput, if non-nil, is the input the harness should run instead
	// of the original Input. Only meaningful for permission-prompt-style
	// hooks where the underlying harness supports input substitution
	// (e.g. Claude Code's permission-prompt-tool returning updatedInput).
	UpdatedInput json.RawMessage `json:"updated_input,omitempty"`

	// Message is an optional explanation forwarded to the harness (and shown
	// alongside denied calls in the UI).
	Message string `json:"message,omitempty"`

	// ResolvedBy identifies the resolver that decided: "user", "shell:<cmd>",
	// "auto", "allow_always". Informational; consumers should not branch on
	// it for correctness.
	ResolvedBy string `json:"resolved_by,omitempty"`
}
