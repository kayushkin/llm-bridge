package msg

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolCallSummary(t *testing.T) {
	cases := []struct {
		name, tool, input, want string
	}{
		{"bash command", "Bash", `{"description":"list","command":"cat  thing.txt\n| head"}`, "cat thing.txt | head"},
		{"read path", "Read", `{"file_path":"/a/b.go","limit":5}`, "/a/b.go"},
		{"task prefers description", "Task", `{"prompt":"long prompt","description":"find the parser"}`, "find the parser"},
		{"task falls to prompt", "Task", `{"prompt":"long prompt"}`, "long prompt"},
		{"unlisted tool takes first string field in document order", "mcp__x__y", `{"n":3,"zeta":"first","alpha":"second"}`, "first"},
		{"no string field falls to the json", "X", `{"n":3}`, `{"n":3}`},
		{"string input", "X", `"hello  there"`, "hello there"},
		{"null", "X", `null`, ""},
		{"empty", "X", ``, ""},
	}
	for _, c := range cases {
		if got := ToolCallSummary(c.tool, json.RawMessage(c.input)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestToolCallSummaryCapsAt120Characters(t *testing.T) {
	input, _ := json.Marshal(map[string]string{"command": strings.Repeat("é", 300)})
	got := []rune(ToolCallSummary("Bash", input))
	if len(got) != 120 || got[119] != '…' {
		t.Fatalf("got %d runes ending %q, want 120 ending in an ellipsis", len(got), string(got[len(got)-1]))
	}
}

func TestSessionStatusEqualIgnoresAsOf(t *testing.T) {
	a := SessionStatus{State: SessionToolRunning, Tools: []StatusTool{{ToolID: "t1", Name: "Bash"}}, AsOf: 1}
	b := a
	b.AsOf = 2
	if !a.Equal(b) {
		t.Fatal("statuses differing only in AsOf must be equal")
	}
	b.Tools = []StatusTool{{ToolID: "t1", Name: "Bash"}, {ToolID: "t2", Name: "Read"}}
	if a.Equal(b) {
		t.Fatal("a second tool in flight is a different status")
	}
	c := a
	c.RateLimit = &StatusRateLimit{Status: RateLimitRejected}
	if a.Equal(c) {
		t.Fatal("a rate-limit report is a different status")
	}
}
