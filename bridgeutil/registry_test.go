package bridgeutil

import (
	"testing"
)

func TestNewRegistry_AllProviders(t *testing.T) {
	reg := NewRegistry()

	expectedKeys := []string{
		"anthropic:message_response",
		"openai:chat_completion",
		"gemini:generate_content",
		"openrouter:chat_completion",
		"claude_code:stream_event",
		"codex:event",
		"openclaw:frame",
	}
	for _, key := range expectedKeys {
		t.Run(key, func(t *testing.T) {
			schema, ok := reg.Types[key]
			if !ok {
				t.Fatalf("missing schema for %q", key)
			}
			if len(schema.Fields) == 0 {
				t.Errorf("schema %q has no fields", key)
			}
			if schema.Provider == "" {
				t.Errorf("schema %q has empty provider", key)
			}
			if schema.TypeName == "" {
				t.Errorf("schema %q has empty type name", key)
			}
		})
	}
}

// --- CheckEnum ---

func TestCheckEnum_KnownValue(t *testing.T) {
	reg := NewRegistry()
	drift := reg.CheckEnum(ProviderAnthropic, "message_response", "type", "message")
	if drift != nil {
		t.Errorf("expected nil for known enum value, got: %+v", drift)
	}
}

func TestCheckEnum_UnknownValue(t *testing.T) {
	reg := NewRegistry()
	drift := reg.CheckEnum(ProviderAnthropic, "message_response", "type", "conversation")
	if drift == nil {
		t.Fatal("expected drift for unknown enum value")
	}
	if drift.Kind != DriftNewEnumValue {
		t.Errorf("Kind = %q, want %q", drift.Kind, DriftNewEnumValue)
	}
	if drift.Got != "conversation" {
		t.Errorf("Got = %q, want %q", drift.Got, "conversation")
	}
}

func TestCheckEnum_NonEnumField(t *testing.T) {
	reg := NewRegistry()
	// "id" has no enum values, so any value should be fine
	drift := reg.CheckEnum(ProviderAnthropic, "message_response", "id", "anything")
	if drift != nil {
		t.Error("expected nil for non-enum field")
	}
}

func TestCheckEnum_UnknownProvider(t *testing.T) {
	reg := NewRegistry()
	drift := reg.CheckEnum(Provider("martian"), "message_response", "type", "message")
	if drift != nil {
		t.Error("expected nil for unknown provider")
	}
}

func TestCheckEnum_UnknownType(t *testing.T) {
	reg := NewRegistry()
	drift := reg.CheckEnum(ProviderAnthropic, "nonexistent_type", "type", "message")
	if drift != nil {
		t.Error("expected nil for unknown type")
	}
}

// --- Provider-specific enum checks ---

func TestCheckEnum_Anthropic_StopReasons(t *testing.T) {
	reg := NewRegistry()
	validReasons := []string{"end_turn", "max_tokens", "stop_sequence", "tool_use"}
	for _, reason := range validReasons {
		t.Run(reason, func(t *testing.T) {
			drift := reg.CheckEnum(ProviderAnthropic, "message_response", "stop_reason", reason)
			if drift != nil {
				t.Errorf("expected nil for valid stop reason %q", reason)
			}
		})
	}
}

func TestCheckEnum_ClaudeCode_EventTypes(t *testing.T) {
	reg := NewRegistry()
	validTypes := []string{"assistant", "system", "result"}
	for _, et := range validTypes {
		t.Run(et, func(t *testing.T) {
			drift := reg.CheckEnum(ProviderClaudeCode, "stream_event", "type", et)
			if drift != nil {
				t.Errorf("expected nil for valid type %q", et)
			}
		})
	}
}

func TestCheckEnum_Codex_EventTypes(t *testing.T) {
	reg := NewRegistry()
	validTypes := []string{"thread.started", "turn.started", "turn.completed", "turn.failed", "item.started", "item.completed", "error"}
	for _, et := range validTypes {
		t.Run(et, func(t *testing.T) {
			drift := reg.CheckEnum(ProviderCodex, "event", "type", et)
			if drift != nil {
				t.Errorf("expected nil for valid type %q", et)
			}
		})
	}
}

func TestCheckEnum_OpenClaw_FrameTypes(t *testing.T) {
	reg := NewRegistry()
	validTypes := []string{"req", "res", "event"}
	for _, ft := range validTypes {
		t.Run(ft, func(t *testing.T) {
			drift := reg.CheckEnum(ProviderOpenClaw, "frame", "type", ft)
			if drift != nil {
				t.Errorf("expected nil for valid type %q", ft)
			}
		})
	}
}

// --- Schema field completeness ---

func TestRegistry_AnthropicRequiredFields(t *testing.T) {
	reg := NewRegistry()
	schema := reg.Types["anthropic:message_response"]

	required := []string{"id", "type", "role", "content", "model", "usage"}
	for _, field := range required {
		t.Run(field, func(t *testing.T) {
			fs, ok := schema.Fields[field]
			if !ok {
				t.Fatalf("missing field %q in anthropic schema", field)
			}
			if !fs.Required {
				t.Errorf("field %q should be required", field)
			}
		})
	}
}
