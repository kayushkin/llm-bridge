package msg

import "encoding/json"

// TokenUsage tracks token consumption across all providers.
type TokenUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	TotalTokens     int `json:"total_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
	ContextTokens    int `json:"context_tokens,omitempty"`
	ContextLimit     int `json:"context_limit,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// Cost tracks monetary cost of a request.
type Cost struct {
	TotalUSD     float64 `json:"total_usd"`
	InputUSD     float64 `json:"input_usd,omitempty"`
	OutputUSD    float64 `json:"output_usd,omitempty"`
	IsByok       bool    `json:"is_byok,omitempty"`
	UpstreamCost float64 `json:"upstream_cost,omitempty"`
}
