// Package drift provides schema drift detection for LLM API payloads.
//
// When providers update their APIs (new fields, changed types, removed fields),
// this package detects the changes at unmarshal time and reports them without
// breaking normal operation.
//
// Usage:
//
//	var resp api.CompletionResponse
//	report, err := drift.Unmarshal(data, &resp, drift.ProviderAnthropic)
//	if err != nil {
//	    // Hard parse failure — data is malformed
//	}
//	if report.HasDrift() {
//	    log.Warn("schema drift detected", "report", report)
//	    // Continue operating — data was parsed, drift is informational
//	}
package drift

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Provider mirrors api.Provider to avoid circular imports.
type Provider string

const (
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenAI     Provider = "openai"
	ProviderGemini     Provider = "gemini"
	ProviderOpenRouter Provider = "openrouter"
	ProviderClaudeCode Provider = "claude_code"
	ProviderCodex      Provider = "codex"
	ProviderOpenClaw   Provider = "openclaw"
)

// DriftKind classifies the type of schema change detected.
type DriftKind string

const (
	// DriftUnknownField is a JSON key not mapped to any struct field.
	DriftUnknownField DriftKind = "unknown_field"

	// DriftTypeChange is a JSON value whose type doesn't match the struct field.
	DriftTypeChange DriftKind = "type_change"

	// DriftNewEnumValue is a string value not in the known set of enum values.
	DriftNewEnumValue DriftKind = "new_enum_value"

	// DriftMissingField is an expected field absent from the JSON.
	DriftMissingField DriftKind = "missing_field"

	// DriftNullField is a field that was non-null before but is now null.
	DriftNullField DriftKind = "null_field"
)

// DriftField records a single detected drift.
type DriftField struct {
	Kind     DriftKind `json:"kind"`
	Path     string    `json:"path"`              // JSON path, e.g. "choices[0].message.role"
	Field    string    `json:"field"`             // Field name at the leaf.
	Expected string    `json:"expected,omitempty"` // Expected type or enum values.
	Got      string    `json:"got,omitempty"`      // Actual type or value.
	RawValue json.RawMessage `json:"raw_value,omitempty"` // The raw JSON for unknown fields.
}

// Report is the result of drift-aware unmarshaling.
type Report struct {
	Provider  Provider     `json:"provider"`
	TypeName  string       `json:"type_name"` // Go type that was unmarshaled into.
	Timestamp time.Time    `json:"timestamp"`
	Fields    []DriftField `json:"fields,omitempty"`
}

// HasDrift returns true if any drift was detected.
func (r *Report) HasDrift() bool {
	return len(r.Fields) > 0
}

// UnknownFields returns only the unknown field drifts.
func (r *Report) UnknownFields() []DriftField {
	var out []DriftField
	for _, f := range r.Fields {
		if f.Kind == DriftUnknownField {
			out = append(out, f)
		}
	}
	return out
}

// Summary returns a human-readable summary of the drift.
func (r *Report) Summary() string {
	if !r.HasDrift() {
		return "no drift detected"
	}
	counts := map[DriftKind]int{}
	for _, f := range r.Fields {
		counts[f.Kind]++
	}
	var parts []string
	for kind, count := range counts {
		parts = append(parts, fmt.Sprintf("%d %s", count, kind))
	}
	return fmt.Sprintf("drift in %s from %s: %s", r.TypeName, r.Provider, strings.Join(parts, ", "))
}

// Unmarshal decodes JSON data into target while detecting schema drift.
// It first does a standard unmarshal, then a second pass to find unknown keys.
// The target must have Overflow fields (map[string]json.RawMessage) to capture unknowns.
func Unmarshal(data []byte, target any, provider Provider) (*Report, error) {
	report := &Report{
		Provider:  provider,
		TypeName:  reflect.TypeOf(target).Elem().Name(),
		Timestamp: time.Now(),
	}

	// Standard unmarshal first — this populates the struct.
	if err := json.Unmarshal(data, target); err != nil {
		return report, fmt.Errorf("unmarshal %s payload: %w", provider, err)
	}

	// Second pass: unmarshal into a generic map to find all keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return report, nil // First unmarshal succeeded, so data is valid. Map parse failure is non-fatal.
	}

	// Compare raw keys against struct fields.
	knownFields := structJSONFields(reflect.TypeOf(target).Elem())
	for key, value := range raw {
		if key == "_overflow" {
			continue
		}
		if _, known := knownFields[key]; !known {
			report.Fields = append(report.Fields, DriftField{
				Kind:     DriftUnknownField,
				Path:     key,
				Field:    key,
				RawValue: value,
			})
		}
	}

	return report, nil
}

// UnmarshalStrict is like Unmarshal but also checks for expected fields that are missing.
func UnmarshalStrict(data []byte, target any, provider Provider, expectedFields []string) (*Report, error) {
	report, err := Unmarshal(data, target, provider)
	if err != nil {
		return report, err
	}

	var raw map[string]json.RawMessage
	if jsonErr := json.Unmarshal(data, &raw); jsonErr != nil {
		return report, nil
	}

	for _, field := range expectedFields {
		if _, present := raw[field]; !present {
			report.Fields = append(report.Fields, DriftField{
				Kind:  DriftMissingField,
				Path:  field,
				Field: field,
			})
		}
	}

	return report, nil
}

// structJSONFields returns a set of JSON field names for a struct type.
// Cached for performance.
func structJSONFields(t reflect.Type) map[string]struct{} {
	fieldCacheMu.RLock()
	if cached, ok := fieldCache[t]; ok {
		fieldCacheMu.RUnlock()
		return cached
	}
	fieldCacheMu.RUnlock()

	fields := map[string]struct{}{}
	for i := range t.NumField() {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name != "" {
			fields[name] = struct{}{}
		}
	}

	fieldCacheMu.Lock()
	fieldCache[t] = fields
	fieldCacheMu.Unlock()

	return fields
}

var (
	fieldCache   = map[reflect.Type]map[string]struct{}{}
	fieldCacheMu sync.RWMutex
)
