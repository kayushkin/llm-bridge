package msg

import "encoding/json"

// OneShotRequest is a stateless, single-turn LLM call.
//
// It is not a session: there is no resumption, no event stream, no memory.
// The caller sends a prompt (with an optional JSON schema) and gets a single
// structured response back. Schema, when set, forces the model to emit JSON
// matching that schema — the bridge implementation hard-forces this via
// tool_choice if the underlying provider supports it.
type OneShotRequest struct {
	// Prompt is the user message. Required.
	Prompt string `json:"prompt"`

	// SystemPrompt is an optional system message.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Model overrides the harness instance's default model.
	Model string `json:"model,omitempty"`

	// Schema, when non-empty, is a JSON Schema (raw bytes) the response must
	// conform to. The bridge is expected to hard-force a tool call so that
	// the model returns valid JSON matching the schema.
	Schema json.RawMessage `json:"schema,omitempty"`

	// MaxTokens caps the model's response. 0 = harness default.
	MaxTokens int `json:"max_tokens,omitempty"`
}

// OneShotResponse is the result of a OneShotRequest.
type OneShotResponse struct {
	// Text is the free-text response. Populated when no schema was requested,
	// or when the model emitted text alongside its tool call.
	Text string `json:"text,omitempty"`

	// Parsed is the schema-conformant JSON output. Populated only when the
	// request had a Schema and the model called the synthetic output tool.
	Parsed json.RawMessage `json:"parsed,omitempty"`

	// Usage tracks token consumption for cost accounting.
	Usage TokenUsage `json:"usage"`

	// DurationMs is the wall-clock time of the call.
	DurationMs int64 `json:"duration_ms"`

	// StopReason is the underlying provider's stop reason (e.g. "end_turn",
	// "tool_use", "max_tokens"). Passed through unchanged.
	StopReason string `json:"stop_reason"`

	// Model is the resolved model identifier the provider used.
	Model string `json:"model,omitempty"`
}
