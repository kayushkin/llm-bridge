package bridgeutil

import (
	"encoding/json"
	"testing"
)

// --- Unmarshal ---

type sampleTarget struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestUnmarshal_NoDrift(t *testing.T) {
	data := []byte(`{"id":"1","name":"test","value":42}`)
	var target sampleTarget

	report, err := Unmarshal(data, &target, ProviderAnthropic)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if report.HasDrift() {
		t.Errorf("expected no drift, got: %v", report.Fields)
	}
	if target.ID != "1" || target.Name != "test" || target.Value != 42 {
		t.Errorf("target not populated correctly: %+v", target)
	}
	if report.Provider != ProviderAnthropic {
		t.Errorf("Provider = %q, want %q", report.Provider, ProviderAnthropic)
	}
	if report.TypeName != "sampleTarget" {
		t.Errorf("TypeName = %q, want %q", report.TypeName, "sampleTarget")
	}
}

func TestUnmarshal_UnknownField(t *testing.T) {
	data := []byte(`{"id":"1","name":"test","value":42,"new_field":"surprise"}`)
	var target sampleTarget

	report, err := Unmarshal(data, &target, ProviderOpenAI)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !report.HasDrift() {
		t.Fatal("expected drift for unknown field")
	}
	if len(report.Fields) != 1 {
		t.Fatalf("expected 1 drift field, got %d", len(report.Fields))
	}
	f := report.Fields[0]
	if f.Kind != DriftUnknownField {
		t.Errorf("Kind = %q, want %q", f.Kind, DriftUnknownField)
	}
	if f.Field != "new_field" {
		t.Errorf("Field = %q, want %q", f.Field, "new_field")
	}
}

func TestUnmarshal_MultipleUnknownFields(t *testing.T) {
	data := []byte(`{"id":"1","name":"test","value":42,"extra1":"a","extra2":123}`)
	var target sampleTarget

	report, err := Unmarshal(data, &target, ProviderGemini)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(report.Fields) != 2 {
		t.Fatalf("expected 2 drift fields, got %d", len(report.Fields))
	}
}

func TestUnmarshal_InvalidJSON(t *testing.T) {
	data := []byte(`{broken`)
	var target sampleTarget

	_, err := Unmarshal(data, &target, ProviderAnthropic)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestUnmarshal_OverflowFieldIgnored(t *testing.T) {
	type withOverflow struct {
		ID       string                     `json:"id"`
		Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
	}
	data := []byte(`{"id":"1","_overflow":{"x":"y"}}`)
	var target withOverflow

	report, err := Unmarshal(data, &target, ProviderAnthropic)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// _overflow should not be reported as unknown
	for _, f := range report.Fields {
		if f.Field == "_overflow" {
			t.Error("_overflow should be ignored in drift detection")
		}
	}
}

// --- UnmarshalStrict ---

func TestUnmarshalStrict_MissingExpected(t *testing.T) {
	data := []byte(`{"id":"1"}`)
	var target sampleTarget

	report, err := UnmarshalStrict(data, &target, ProviderAnthropic, []string{"id", "name", "value"})
	if err != nil {
		t.Fatalf("UnmarshalStrict: %v", err)
	}
	// "name" and "value" are missing from JSON
	missing := 0
	for _, f := range report.Fields {
		if f.Kind == DriftMissingField {
			missing++
		}
	}
	if missing != 2 {
		t.Errorf("expected 2 missing fields, got %d", missing)
	}
}

func TestUnmarshalStrict_AllPresent(t *testing.T) {
	data := []byte(`{"id":"1","name":"test","value":42}`)
	var target sampleTarget

	report, err := UnmarshalStrict(data, &target, ProviderAnthropic, []string{"id", "name", "value"})
	if err != nil {
		t.Fatalf("UnmarshalStrict: %v", err)
	}
	for _, f := range report.Fields {
		if f.Kind == DriftMissingField {
			t.Errorf("unexpected missing field: %s", f.Field)
		}
	}
}

// --- Report methods ---

func TestReport_Summary_NoDrift(t *testing.T) {
	r := &Report{Provider: ProviderAnthropic, TypeName: "Test"}
	if s := r.Summary(); s != "no drift detected" {
		t.Errorf("Summary = %q", s)
	}
}

func TestReport_Summary_WithDrift(t *testing.T) {
	r := &Report{
		Provider: ProviderOpenAI,
		TypeName: "ChatCompletion",
		Fields: []DriftField{
			{Kind: DriftUnknownField, Path: "new_field", Field: "new_field"},
			{Kind: DriftUnknownField, Path: "other", Field: "other"},
		},
	}
	s := r.Summary()
	if s == "no drift detected" {
		t.Error("expected non-empty summary")
	}
}

func TestReport_UnknownFields(t *testing.T) {
	r := &Report{
		Fields: []DriftField{
			{Kind: DriftUnknownField, Field: "a"},
			{Kind: DriftMissingField, Field: "b"},
			{Kind: DriftUnknownField, Field: "c"},
		},
	}
	uf := r.UnknownFields()
	if len(uf) != 2 {
		t.Errorf("expected 2 unknown fields, got %d", len(uf))
	}
}
