package msg

import "encoding/json"

// Conversation represents a complete request to an LLM API.
type Conversation struct {
	ID       string   `json:"id,omitempty"`
	Model    string   `json:"model"`
	Provider Provider `json:"provider"`

	// System prompt — always a slice of content blocks, never a bare string.
	System []ContentBlock `json:"system,omitempty"`

	Messages []Message `json:"messages"`

	Tools      []ToolDef   `json:"tools,omitempty"`
	ToolChoice *ToolChoice `json:"tool_choice,omitempty"`

	Config         GenerationConfig `json:"config"`
	ProviderConfig json.RawMessage  `json:"provider_config,omitempty"` // AnthropicConfig | OpenAIConfig | GeminiConfig | OpenRouterConfig

	// Extensions holds bridge-specific typed data not yet in the canonical schema.
	// Keys are namespaced by bridge (e.g. "anthropic:beta_feature").
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// Message is a single turn in a conversation.
type Message struct {
	ID   string `json:"id,omitempty"`
	Role Role   `json:"role"`

	// Content is always a slice of typed blocks.
	Content []ContentBlock `json:"content"`

	// Name is an optional participant identifier (OpenAI supports this).
	Name string `json:"name,omitempty"`

	// NativeRole preserves the original provider role for lossless round-trip.
	NativeRole *NativeRole `json:"native_role,omitempty"`

	// SourcePattern records how this message was originally structured for round-trip.
	SourcePattern string `json:"source_pattern,omitempty"`

	// Extensions holds bridge-specific typed data not yet in the canonical schema.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}
