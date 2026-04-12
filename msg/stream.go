package msg

import "encoding/json"

// StreamEventType identifies the kind of streaming event.
type StreamEventType string

const (
	StreamEventStart             StreamEventType = "stream_start"
	StreamEventContentBlockStart StreamEventType = "content_block_start"
	StreamEventContentBlockDelta StreamEventType = "content_block_delta"
	StreamEventContentBlockStop  StreamEventType = "content_block_stop"
	StreamEventMessageDelta      StreamEventType = "message_delta"
	StreamEventMessageStop       StreamEventType = "message_stop"
	StreamEventPing              StreamEventType = "ping"
	StreamEventError             StreamEventType = "error"
)

// StreamEvent is the unified streaming event type.
// Follows Anthropic's granular model (most expressive) and synthesizes
// block-level events for providers that don't have them (OpenAI, Gemini).
type StreamEvent struct {
	Type StreamEventType `json:"type"`

	Start      *StreamStart  `json:"start,omitempty"`
	BlockStart *BlockStart   `json:"block_start,omitempty"`
	BlockDelta *BlockDelta   `json:"block_delta,omitempty"`
	BlockStop  *BlockStop    `json:"block_stop,omitempty"`
	MsgDelta   *MessageDelta `json:"msg_delta,omitempty"`
	Error      *StreamError  `json:"error,omitempty"`

	// Raw is the complete original provider JSON for this stream event.
	Raw json.RawMessage `json:"raw,omitempty"`

	// Extensions holds bridge-specific typed data not yet in the canonical schema.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// StreamStart carries initial message metadata.
type StreamStart struct {
	ID       string     `json:"id"`
	Model    string     `json:"model"`
	Provider Provider   `json:"provider"`
	Usage    TokenUsage `json:"usage"`
}

// BlockStart signals a new content block beginning.
type BlockStart struct {
	Index int          `json:"index"`
	Block ContentBlock `json:"block"`
}

// DeltaType identifies the kind of streaming delta.
type DeltaType string

const (
	DeltaText      DeltaType = "text_delta"
	DeltaInputJSON DeltaType = "input_json_delta"
	DeltaThinking  DeltaType = "thinking_delta"
	DeltaSignature DeltaType = "signature_delta"
	DeltaAudio     DeltaType = "audio_delta"
)

// BlockDelta carries incremental content for a block.
type BlockDelta struct {
	Index       int       `json:"index"`
	Type        DeltaType `json:"type"`
	Text        string    `json:"text,omitempty"`
	PartialJSON string    `json:"partial_json,omitempty"`
	Thinking    string    `json:"thinking,omitempty"`
	Signature   string    `json:"signature,omitempty"`
	AudioData   string    `json:"audio_data,omitempty"`
}

// BlockStop signals a content block has finished.
type BlockStop struct {
	Index int `json:"index"`
}

// MessageDelta carries end-of-message metadata.
type MessageDelta struct {
	StopReason       StopReason `json:"stop_reason"`
	NativeStopReason string     `json:"native_stop_reason,omitempty"`
	StopSequence     string     `json:"stop_sequence,omitempty"`
	Usage            TokenUsage `json:"usage"`
}

// StreamError represents an error during streaming.
type StreamError struct {
	Type         string `json:"type"`
	Message      string `json:"message"`
	Code         string `json:"code,omitempty"`
	Retryable    bool   `json:"retryable,omitempty"`
	RetryAfterMs int    `json:"retry_after_ms,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
}
