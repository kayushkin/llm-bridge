package msg

import "encoding/json"

// ToolDef defines a tool the model can call.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`

	Strict       bool          `json:"strict,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// ToolChoiceMode determines how the model selects tools.
type ToolChoiceMode string

const (
	ToolChoiceAuto     ToolChoiceMode = "auto"
	ToolChoiceAny      ToolChoiceMode = "any"
	ToolChoiceNone     ToolChoiceMode = "none"
	ToolChoiceRequired ToolChoiceMode = "required"
	ToolChoiceSpecific ToolChoiceMode = "specific"
)

// ToolChoice controls model tool selection behavior.
type ToolChoice struct {
	Mode            ToolChoiceMode `json:"mode"`
	Name            string         `json:"name,omitempty"`
	DisableParallel bool           `json:"disable_parallel,omitempty"`
}
