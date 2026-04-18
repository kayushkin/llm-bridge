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
type ToolCallEvent struct {
	ToolID string          `json:"tool_id"`
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
}

// ToolResultEvent is the output of a tool invocation.
type ToolResultEvent struct {
	ToolID  string `json:"tool_id"`
	Name    string `json:"name"`
	Output  string `json:"output"`
	IsError bool   `json:"is_error,omitempty"`
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
type SystemEvent struct {
	Subtype      string `json:"subtype"`
	Message      string `json:"message,omitempty"`
	Attempt      int    `json:"attempt,omitempty"`
	MaxRetries   int    `json:"max_retries,omitempty"`
	RetryDelayMS int    `json:"retry_delay_ms,omitempty"`
	ErrorStatus  int    `json:"error_status,omitempty"`
	ErrorType    string `json:"error_type,omitempty"`
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
