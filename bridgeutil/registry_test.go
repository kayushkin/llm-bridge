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
		"opencode:event",
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

func TestCheckEnum_OpenCode_EventTypes(t *testing.T) {
	reg := NewRegistry()
	validTypes := []string{
		"message.updated", "message.removed",
		"message.part.updated", "message.part.removed",
		"session.created", "session.updated", "session.deleted",
		"session.status", "session.idle", "session.error",
		"session.diff", "session.compacted",
		"permission.updated", "permission.replied",
	}
	for _, et := range validTypes {
		t.Run(et, func(t *testing.T) {
			drift := reg.CheckEnum(ProviderOpenCode, "event", "type", et)
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

// --- The schemas themselves, pinned against literals this file owns ---

// registryFieldNamesEveryProviderMustCarry is written out by hand rather than read
// out of NewRegistry(), and that is the whole point of it. A fixture drawn from the
// table under test loops one fewer time when a row is deleted and stays green, so it
// pins nothing against deletion -- the defect the derived-fixture sweep exists to find.
//
// Measured 2026-08-29 by deleting every row of all eight schemas in turn, 52
// mutations: 46 survived with the package suite green. Every one of the 6 catches
// came from TestRegistry_AnthropicRequiredFields, which was the only test here
// owning a literal, and it covers one provider of eight.
var registryFieldNamesEveryProviderMustCarry = map[string][]string{
	"anthropic:message_response": {
		"id", "type", "role", "content", "model", "stop_reason", "stop_sequence", "usage",
	},
	"openai:chat_completion": {
		"id", "object", "created", "model", "choices", "usage", "system_fingerprint", "service_tier",
	},
	"gemini:generate_content": {
		"candidates", "usageMetadata", "modelVersion",
	},
	"openrouter:chat_completion": {
		"id", "object", "created", "model", "choices", "usage", "system_fingerprint",
	},
	"claude_code:stream_event": {
		"type", "subtype", "session_id", "message", "result", "is_error",
		"total_cost_usd", "duration_ms", "duration_api_ms", "num_turns", "usage", "modelUsage",
	},
	"codex:event": {
		"type", "item",
	},
	"openclaw:frame": {
		"type", "id", "method", "params", "ok", "payload", "event", "seq", "stateVersion", "error",
	},
	"opencode:event": {
		"type", "properties",
	},
}

// TestEverySchemaCarriesExactlyTheFieldsThisTestNames fails in both directions: a
// field dropped from a shipped schema, and a field added to one without anybody
// recording it here. The registry's job is to notice that a provider's wire format
// moved, so a row silently leaving it is the registry losing the ability to report
// exactly the drift it exists to report.
func TestEverySchemaCarriesExactlyTheFieldsThisTestNames(t *testing.T) {
	reg := NewRegistry()

	if len(reg.Types) != len(registryFieldNamesEveryProviderMustCarry) {
		t.Errorf("registry has %d schemas, this test names %d", len(reg.Types), len(registryFieldNamesEveryProviderMustCarry))
	}

	for key, wantFields := range registryFieldNamesEveryProviderMustCarry {
		t.Run(key, func(t *testing.T) {
			schema, ok := reg.Types[key]
			if !ok {
				t.Fatalf("missing schema for %q", key)
			}
			for _, field := range wantFields {
				if _, ok := schema.Fields[field]; !ok {
					t.Errorf("schema %q no longer carries field %q", key, field)
				}
			}
			if len(schema.Fields) != len(wantFields) {
				for field := range schema.Fields {
					if !containsString(wantFields, field) {
						t.Errorf("schema %q carries field %q that this test does not name", key, field)
					}
				}
			}
		})
	}
}

// enumFieldsEveryProviderDeclares names every (schema, field) pair that ships a
// non-empty EnumValues list.
//
// The five TestCheckEnum_*_*Types tests above assert only that a KNOWN value
// produces no drift, and that is a one-sided assertion CheckEnum satisfies for four
// unrelated reasons: unknown provider, unknown type, unknown field, and field with
// no enum values all return nil too. So deleting claude_code stream_event's whole
// "type" row left TestCheckEnum_ClaudeCode_EventTypes green -- measured, not
// reasoned. Asserting the other direction is what makes the field's presence
// load-bearing.
var enumFieldsEveryProviderDeclares = []struct {
	provider Provider
	typeName string
	field    string
}{
	{ProviderAnthropic, "message_response", "type"},
	{ProviderAnthropic, "message_response", "role"},
	{ProviderAnthropic, "message_response", "stop_reason"},
	{ProviderOpenAI, "chat_completion", "object"},
	{ProviderClaudeCode, "stream_event", "type"},
	{ProviderClaudeCode, "stream_event", "subtype"},
	{ProviderCodex, "event", "type"},
	{ProviderOpenCode, "event", "type"},
	{ProviderOpenClaw, "frame", "type"},
}

func TestAnUnknownValueDriftsOnEveryFieldThatDeclaresAnEnum(t *testing.T) {
	reg := NewRegistry()
	const notAnyDeclaredValue = "definitely-not-a-value-any-provider-declares"

	for _, enumField := range enumFieldsEveryProviderDeclares {
		t.Run(string(enumField.provider)+":"+enumField.typeName+"."+enumField.field, func(t *testing.T) {
			drift := reg.CheckEnum(enumField.provider, enumField.typeName, enumField.field, notAnyDeclaredValue)
			if drift == nil {
				t.Fatalf("CheckEnum accepted %q on %s:%s.%s -- the field's enum list is gone, or the field is",
					notAnyDeclaredValue, enumField.provider, enumField.typeName, enumField.field)
			}
			if drift.Kind != DriftNewEnumValue {
				t.Errorf("Kind = %q, want %q", drift.Kind, DriftNewEnumValue)
			}
		})
	}
}

func containsString(haystack []string, needle string) bool {
	for _, candidate := range haystack {
		if candidate == needle {
			return true
		}
	}
	return false
}
