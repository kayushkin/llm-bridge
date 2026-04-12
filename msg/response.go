package msg

import "encoding/json"

// CompletionResponse is the unified response from any LLM API.
type CompletionResponse struct {
	ID       string   `json:"id"`
	Model    string   `json:"model"`
	Provider Provider `json:"provider"`

	Message          Message    `json:"message"`
	StopReason       StopReason `json:"stop_reason"`
	NativeStopReason string     `json:"native_stop_reason"`
	StopSequence     string     `json:"stop_sequence,omitempty"`

	Usage TokenUsage `json:"usage"`
	Cost  *Cost      `json:"cost,omitempty"`

	// Gemini: per-response safety ratings.
	SafetyRatings []SafetyRating `json:"safety_ratings,omitempty"`

	// OpenAI: server-side configuration fingerprint.
	SystemFingerprint string `json:"system_fingerprint,omitempty"`

	// OpenAI: logprobs for the response tokens.
	LogProbs *LogProbs `json:"logprobs,omitempty"`

	// OpenAI: multiple choices when n > 1.
	AdditionalChoices []CompletionChoice `json:"additional_choices,omitempty"`

	// Raw is the complete original provider JSON response.
	// Preserved for debugging and detecting data missed by the bridge.
	Raw json.RawMessage `json:"raw,omitempty"`

	// Extensions holds bridge-specific typed data not yet in the canonical schema.
	// Keys are namespaced by bridge (e.g. "anthropic:beta_feature", "openai:service_tier").
	// This allows bridges to pass through new features before the spec adds official support.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// CompletionChoice represents one of potentially multiple completions (OpenAI n > 1).
type CompletionChoice struct {
	Index            int        `json:"index"`
	Message          Message    `json:"message"`
	StopReason       StopReason `json:"stop_reason"`
	NativeStopReason string     `json:"native_stop_reason"`
}

// SafetyRating is a Gemini per-category safety assessment.
type SafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
	Blocked     bool   `json:"blocked,omitempty"`
}

// SafetySetting configures Gemini safety thresholds.
type SafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// LogProbs contains OpenAI token log probability data.
type LogProbs struct {
	Content []TokenLogProb `json:"content,omitempty"`
}

// TokenLogProb is the log probability for a single token.
type TokenLogProb struct {
	Token       string       `json:"token"`
	LogProb     float64      `json:"logprob"`
	Bytes       []int        `json:"bytes,omitempty"`
	TopLogProbs []TopLogProb `json:"top_logprobs,omitempty"`
}

// TopLogProb is one of the top-k alternative tokens.
type TopLogProb struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}
