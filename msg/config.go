package msg

import "encoding/json"

// GenerationConfig holds provider-agnostic generation parameters.
type GenerationConfig struct {
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	TopP           *float64        `json:"top_p,omitempty"`
	TopK           *int            `json:"top_k,omitempty"`
	StopSequences  []string        `json:"stop_sequences,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// ResponseFormat specifies structured output requirements.
type ResponseFormat struct {
	Type       string          `json:"type"`
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
	Strict     bool            `json:"strict,omitempty"`
}

// --- Provider-specific configs ---

// AnthropicConfig holds Anthropic-specific request parameters.
type AnthropicConfig struct {
	ThinkingBudget   int              `json:"thinking_budget,omitempty"`
	ThinkingDisplay  string           `json:"thinking_display,omitempty"`
	Effort           string           `json:"effort,omitempty"`
	ContainerID      string           `json:"container_id,omitempty"`
	ServiceTier      string           `json:"service_tier,omitempty"`
	InferenceGeo     string           `json:"inference_geo,omitempty"`
	AssistantPrefill []ContentBlock   `json:"assistant_prefill,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// OpenAIConfig holds OpenAI-specific request parameters.
type OpenAIConfig struct {
	Prediction         string `json:"prediction,omitempty"`
	LogProbs           bool   `json:"logprobs,omitempty"`
	TopLogProbs        int    `json:"top_logprobs,omitempty"`
	N                  int    `json:"n,omitempty"`
	FrequencyPenalty   float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty    float64 `json:"presence_penalty,omitempty"`
	Seed               *int   `json:"seed,omitempty"`
	StreamIncludeUsage bool   `json:"stream_include_usage,omitempty"`
	Store              bool   `json:"store,omitempty"`
	PreviousResponseID string `json:"previous_response_id,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// GeminiConfig holds Gemini-specific request parameters.
type GeminiConfig struct {
	SafetySettings []SafetySetting `json:"safety_settings,omitempty"`
	CachedContent  string          `json:"cached_content,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// OpenRouterConfig holds OpenRouter-specific request parameters.
type OpenRouterConfig struct {
	Models            []string                 `json:"models,omitempty"`
	Route             string                   `json:"route,omitempty"`
	ProviderPrefs     *OpenRouterProviderPrefs `json:"provider_prefs,omitempty"`
	Plugins           []OpenRouterPlugin       `json:"plugins,omitempty"`
	Debug             bool                     `json:"debug,omitempty"`
	TopA              *float64                 `json:"top_a,omitempty"`
	MinP              *float64                 `json:"min_p,omitempty"`
	RepetitionPenalty *float64                 `json:"repetition_penalty,omitempty"`
	Prediction        string                   `json:"prediction,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// OpenRouterProviderPrefs controls model routing preferences.
type OpenRouterProviderPrefs struct {
	Order          []string `json:"order,omitempty"`
	AllowFallbacks bool     `json:"allow_fallbacks,omitempty"`
	RequireParams  bool     `json:"require_parameters,omitempty"`
}

// OpenRouterPlugin is an OpenRouter plugin configuration.
type OpenRouterPlugin struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}
