package msg

import (
	"bytes"
	"encoding/json"
	"strings"
)

// maxToolCallSummaryChars caps a summary. The full input stays on the
// tool_call event for any view that wants it.
const maxToolCallSummaryChars = 120

// summaryFieldByTool names the input field that best says what a call is
// doing, per tool. A tool not listed falls through to the first string field
// of its input, in the order the harness wrote them.
var summaryFieldByTool = map[string][]string{
	"Bash":         {"command"},
	"Read":         {"file_path"},
	"Write":        {"file_path"},
	"Edit":         {"file_path"},
	"NotebookEdit": {"notebook_path"},
	"Grep":         {"pattern"},
	"Glob":         {"pattern"},
	"Task":         {"description", "prompt"},
	"Agent":        {"description", "prompt"},
	"WebFetch":     {"url"},
	"WebSearch":    {"query"},
	"Skill":        {"skill"},
}

// ToolCallSummary is one human line for a tool call's input: `cat thing.txt`
// for Bash, the path for Read/Write/Edit, the description for Task — and the
// raw JSON only when the input has no string field at all. Whitespace is
// collapsed and the line is capped at 120 characters.
//
// It lives here so every reader of a SessionStatus sees a call described in the
// same words. It was chat-core's `toolCallSummary` until the status moved to
// the server; the two must not both exist.
func ToolCallSummary(name string, input json.RawMessage) string {
	trimmed := bytes.TrimSpace(input)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] == '"' {
		var s string
		if json.Unmarshal(trimmed, &s) == nil {
			return flattenAndCap(s)
		}
	}
	if trimmed[0] != '{' {
		return flattenAndCap(string(trimmed))
	}
	keys, values := orderedStringFields(trimmed)
	for _, field := range summaryFieldByTool[name] {
		for i, key := range keys {
			if key == field && values[i] != "" {
				return flattenAndCap(values[i])
			}
		}
	}
	for _, value := range values {
		if value != "" {
			return flattenAndCap(value)
		}
	}
	return flattenAndCap(string(trimmed))
}

// orderedStringFields returns an object's top-level string-valued fields in
// document order. A map would lose that order, and "the first string field" is
// the fallback rule.
func orderedStringFields(object []byte) (keys, values []string) {
	dec := json.NewDecoder(bytes.NewReader(object))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, nil
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return keys, values
		}
		key, _ := keyTok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return keys, values
		}
		var s string
		if len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, &s) == nil {
			keys = append(keys, key)
			values = append(values, s)
		}
	}
	return keys, values
}

func flattenAndCap(s string) string {
	flat := strings.Join(strings.Fields(s), " ")
	runes := []rune(flat)
	if len(runes) <= maxToolCallSummaryChars {
		return flat
	}
	return string(runes[:maxToolCallSummaryChars-1]) + "…"
}
