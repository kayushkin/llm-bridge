package drift

// Registry tracks known fields and enum values per provider per type.
// This allows detecting not just unknown fields, but also new enum values
// and removed fields across provider API versions.
type Registry struct {
	// Types maps "provider:type_name" to known field definitions.
	Types map[string]TypeSchema `json:"types"`
}

// TypeSchema defines the expected shape of a provider's JSON type.
type TypeSchema struct {
	Provider Provider          `json:"provider"`
	TypeName string            `json:"type_name"`
	Fields   map[string]FieldSchema `json:"fields"`
	Version  string            `json:"version,omitempty"` // API version this was documented from.
}

// FieldSchema defines what we expect for a single JSON field.
type FieldSchema struct {
	// JSONType is the expected JSON type: "string", "number", "boolean", "object", "array", "null".
	JSONType string `json:"json_type"`

	// Required indicates this field should always be present.
	Required bool `json:"required,omitempty"`

	// EnumValues lists known string values for enum-like fields.
	// Empty means any string is accepted.
	EnumValues []string `json:"enum_values,omitempty"`

	// Description documents what this field is for.
	Description string `json:"description,omitempty"`
}

// NewRegistry creates a registry pre-populated with known schemas.
func NewRegistry() *Registry {
	return &Registry{
		Types: map[string]TypeSchema{
			"anthropic:message_response": anthropicMessageResponse(),
			"openai:chat_completion":     openaiChatCompletion(),
			"gemini:generate_content":    geminiGenerateContent(),
			"openrouter:chat_completion": openrouterChatCompletion(),
			"claude_code:stream_event":   claudeCodeStreamEvent(),
			"codex:event":               codexEvent(),
			"openclaw:frame":            openclawFrame(),
		},
	}
}

// CheckEnum validates a string value against known enum values.
// Returns a DriftField if the value is not in the known set, nil otherwise.
func (r *Registry) CheckEnum(provider Provider, typeName, fieldPath, value string) *DriftField {
	key := string(provider) + ":" + typeName
	schema, ok := r.Types[key]
	if !ok {
		return nil
	}
	field, ok := schema.Fields[fieldPath]
	if !ok || len(field.EnumValues) == 0 {
		return nil
	}
	for _, v := range field.EnumValues {
		if v == value {
			return nil
		}
	}
	return &DriftField{
		Kind:     DriftNewEnumValue,
		Path:     fieldPath,
		Field:    fieldPath,
		Expected: joinStrings(field.EnumValues),
		Got:      value,
	}
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// --- Known schemas per provider ---

func anthropicMessageResponse() TypeSchema {
	return TypeSchema{
		Provider: ProviderAnthropic,
		TypeName: "message_response",
		Fields: map[string]FieldSchema{
			"id":            {JSONType: "string", Required: true},
			"type":          {JSONType: "string", Required: true, EnumValues: []string{"message"}},
			"role":          {JSONType: "string", Required: true, EnumValues: []string{"assistant"}},
			"content":       {JSONType: "array", Required: true},
			"model":         {JSONType: "string", Required: true},
			"stop_reason":   {JSONType: "string", Required: false, EnumValues: []string{"end_turn", "max_tokens", "stop_sequence", "tool_use"}},
			"stop_sequence": {JSONType: "string"},
			"usage":         {JSONType: "object", Required: true},
		},
	}
}

func openaiChatCompletion() TypeSchema {
	return TypeSchema{
		Provider: ProviderOpenAI,
		TypeName: "chat_completion",
		Fields: map[string]FieldSchema{
			"id":                 {JSONType: "string", Required: true},
			"object":            {JSONType: "string", Required: true, EnumValues: []string{"chat.completion"}},
			"created":           {JSONType: "number", Required: true},
			"model":             {JSONType: "string", Required: true},
			"choices":           {JSONType: "array", Required: true},
			"usage":             {JSONType: "object"},
			"system_fingerprint": {JSONType: "string"},
			"service_tier":      {JSONType: "string"},
		},
	}
}

func geminiGenerateContent() TypeSchema {
	return TypeSchema{
		Provider: ProviderGemini,
		TypeName: "generate_content",
		Fields: map[string]FieldSchema{
			"candidates":    {JSONType: "array", Required: true},
			"usageMetadata": {JSONType: "object"},
			"modelVersion":  {JSONType: "string"},
		},
	}
}

func openrouterChatCompletion() TypeSchema {
	return TypeSchema{
		Provider: ProviderOpenRouter,
		TypeName: "chat_completion",
		Fields: map[string]FieldSchema{
			"id":                 {JSONType: "string", Required: true},
			"object":            {JSONType: "string"},
			"created":           {JSONType: "number"},
			"model":             {JSONType: "string", Required: true},
			"choices":           {JSONType: "array", Required: true},
			"usage":             {JSONType: "object"},
			"system_fingerprint": {JSONType: "string"},
		},
	}
}

func claudeCodeStreamEvent() TypeSchema {
	return TypeSchema{
		Provider: ProviderClaudeCode,
		TypeName: "stream_event",
		Fields: map[string]FieldSchema{
			"type":           {JSONType: "string", Required: true, EnumValues: []string{"assistant", "system", "result"}},
			"subtype":        {JSONType: "string", EnumValues: []string{"api_retry", "init", "compact_boundary"}},
			"session_id":     {JSONType: "string"},
			"message":        {JSONType: "object"},
			"result":         {JSONType: "string"},
			"is_error":       {JSONType: "boolean"},
			"total_cost_usd": {JSONType: "number"},
			"duration_ms":    {JSONType: "number"},
			"duration_api_ms": {JSONType: "number"},
			"num_turns":      {JSONType: "number"},
			"usage":          {JSONType: "object"},
			"modelUsage":     {JSONType: "object"},
		},
	}
}

func codexEvent() TypeSchema {
	return TypeSchema{
		Provider: ProviderCodex,
		TypeName: "event",
		Fields: map[string]FieldSchema{
			"type": {JSONType: "string", Required: true, EnumValues: []string{
				"thread.started", "turn.started", "turn.completed", "turn.failed",
				"item.started", "item.completed", "error",
			}},
			"item": {JSONType: "object"},
		},
	}
}

func openclawFrame() TypeSchema {
	return TypeSchema{
		Provider: ProviderOpenClaw,
		TypeName: "frame",
		Fields: map[string]FieldSchema{
			"type":         {JSONType: "string", Required: true, EnumValues: []string{"req", "res", "event"}},
			"id":           {JSONType: "string"},
			"method":       {JSONType: "string"},
			"params":       {JSONType: "object"},
			"ok":           {JSONType: "boolean"},
			"payload":      {JSONType: "object"},
			"event":        {JSONType: "string"},
			"seq":          {JSONType: "number"},
			"stateVersion": {JSONType: "number"},
			"error":        {JSONType: "object"},
		},
	}
}
