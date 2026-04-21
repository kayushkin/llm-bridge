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

	SessionID    string `json:"session_id"`
	BridgeID     string `json:"bridge_id,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
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
	Result     *ResultEvent     `json:"result,omitempty"`
	Stream     *HarnessStream   `json:"stream,omitempty"`
	ToolCall   *ToolCallEvent   `json:"tool_call,omitempty"`
	ToolResult *ToolResultEvent `json:"tool_result,omitempty"`
	Thinking   *ThinkingEvent   `json:"thinking,omitempty"`
	Plan       *PlanEvent       `json:"plan,omitempty"`
	System     *SystemEvent     `json:"system,omitempty"`
	Approval   *ApprovalEvent   `json:"approval,omitempty"`
	Error      *ErrorEvent      `json:"error,omitempty"`
	State      *StateEvent      `json:"state,omitempty"`
	Info       *SessionInfo     `json:"info,omitempty"`

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

// ResultEvent is a completed harness run.
type ResultEvent struct {
	Text             string          `json:"text"`
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
type HarnessStream struct {
	Delta     *BlockDelta `json:"delta,omitempty"`
	MessageID string      `json:"message_id,omitempty"`
	Hidden    bool        `json:"hidden,omitempty"`
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
