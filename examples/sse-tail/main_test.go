package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kayushkin/llm-bridge/msg"
)

// gClef is four bytes in UTF-8 (f0 9d 84 9e). A two-byte rune is not enough:
// only one of its two interior offsets is wrong, so a test that happens to
// pick the other one passes against a plain byte cut.
const gClef = "\U0001D11E"

// reachGuard marks a failure meaning the TEST could not reach the code it
// claims to exercise — a broken fixture, not a defect in the code under test.
//
// The sabotage scorer keys on this prefix to tell a reach-guard firing from an
// assertion firing. Without the distinction a mutation that merely breaks the
// fixture scores as "caught", which inflates the score: the suite would be
// credited with detecting a defect when all it detected was itself.
const reachGuard = "REACH-GUARD: "

// straddleInput holds ASCII on both sides of a run of four-byte runes, so
// sliding a budget across it produces both straddling and aligned cuts.
func straddleInput() string {
	return strings.Repeat("a", 40) + strings.Repeat(gClef, 20) + strings.Repeat("b", 40)
}

// cutStraddlesARune reports whether byte offset n falls inside a multi-byte
// rune of s — i.e. whether a plain s[:n] would split one.
func cutStraddlesARune(s string, n int) bool {
	return n < len(s) && !utf8.RuneStart(s[n])
}

// TestTruncateNeverSplitsARune slides the BUDGET across a fixed input.
//
// Sliding the input instead is the trap: trimming it from the front moves its
// start and its cut by the same amount, so the absolute cut position never
// moves and the loop runs one case N times. Trimming from the back moves the
// cut but splits a rune at the far end, so the test goes red on damage it
// caused itself.
func TestTruncateNeverSplitsARune(t *testing.T) {
	input := straddleInput()
	straddled := 0

	for budget := 1; budget <= len(input); budget++ {
		if cutStraddlesARune(input, budget) {
			straddled++
		}
		got := truncate(input, budget)

		if !utf8.ValidString(got) {
			t.Errorf("budget=%d: result is not valid UTF-8: %q", budget, got)
			continue
		}
		if len(input) <= budget {
			continue
		}
		// Truncation happened, so the result is a prefix plus the ellipsis.
		prefix := strings.TrimSuffix(got, "…")
		if prefix == got {
			t.Errorf("budget=%d: truncated result lost its ellipsis: %q", budget, got)
			continue
		}
		// Validity alone is not falsifiable — a helper that returns "" for
		// everything is valid UTF-8 within budget. Pin the length too: never
		// over budget, and never losing more than the width of the rune that
		// straddles the cut.
		if len(prefix) > budget {
			t.Errorf("budget=%d: kept %d bytes, over budget", budget, len(prefix))
		}
		if len(prefix) <= budget-utf8.UTFMax {
			t.Errorf("budget=%d: kept only %d bytes, lost more than one rune's width", budget, len(prefix))
		}
	}

	// A loop is a claim that a range was covered, and nothing checks that
	// claim unless it is written down.
	if straddled == 0 {
		t.Fatalf(reachGuard+"no budget in 1..%d straddled a rune — the input or the loop is wrong, "+
			"so this test proved nothing", len(input))
	}
	t.Logf("%d of %d budgets straddled a rune", straddled, len(input))
}

// TestTruncateAtAnAlignedBudgetIsUnchanged is the known-negative control. It
// must pass against the unfixed byte cut too. Without it there is no way to
// tell "detects a straddle" from "detects non-ASCII input" — a test that is
// red at every offset is measuring the latter.
func TestTruncateAtAnAlignedBudgetIsUnchanged(t *testing.T) {
	input := straddleInput()
	checked := 0

	for budget := 1; budget < len(input); budget++ {
		if cutStraddlesARune(input, budget) {
			continue
		}
		checked++
		got := truncate(input, budget)
		want := input[:budget] + "…"
		if got != want {
			t.Errorf("budget=%d: aligned cut changed: got %q want %q", budget, got, want)
		}
	}

	if checked == 0 {
		t.Fatal(reachGuard + "no aligned budget was exercised — the control proved nothing")
	}
	t.Logf("%d aligned budgets held", checked)
}

// TestTruncateHandlesBudgetsNoInputEverReaches covers the two inputs the byte
// cut was never given. A negative budget panics on s[:n], and a panic in a
// table-driven case takes the whole test binary with it, so every test after
// it measures nothing.
func TestTruncateHandlesBudgetsNoInputEverReaches(t *testing.T) {
	cases := []struct {
		name   string
		s      string
		n      int
		want   string
		reason string
	}{
		{"empty string", "", 10, "", "nothing to truncate"},
		{"zero budget", "abc", 0, "…", "no bytes fit, so only the ellipsis"},
		{"negative budget", "abc", -1, "…", "clamped, not panicked"},
		{"negative budget, multibyte", gClef + "abc", -3, "…", "clamped, not panicked"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncate(c.s, c.n)
			if got != c.want {
				t.Errorf("truncate(%q, %d) = %q, want %q (%s)", c.s, c.n, got, c.want, c.reason)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncate(%q, %d) = %q, not valid UTF-8", c.s, c.n, got)
			}
		})
	}
}

// TestRenderedToolResultStaysValidUTF8 drives the real render path rather than
// the helper, so the defect is shown reaching the output a reader of this
// example would copy. render writes to stdout, so stdout is captured.
func TestRenderedToolResultStaysValidUTF8(t *testing.T) {
	// ToolResult.Output is cut at 120 bytes. Put a four-byte rune across that
	// offset: 118 bytes of ASCII, then the rune spanning bytes 118..121.
	output := strings.Repeat("x", 118) + gClef + strings.Repeat("y", 40)
	if !cutStraddlesARune(output, 120) {
		t.Fatalf(reachGuard + "fixture does not straddle offset 120 — the test would prove nothing")
	}

	ev := msg.Event{
		Type:       msg.EventToolResult,
		ToolResult: &msg.ToolResultEvent{Name: "read_file", Output: output},
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf(reachGuard+"marshal fixture: %v", err)
	}

	got := captureStdout(t, func() { render(string(data), "sess-1") })

	if !utf8.ValidString(got) {
		t.Errorf("rendered line is not valid UTF-8: %q", got)
	}
	if !strings.Contains(got, "read_file") {
		t.Errorf("rendered line lost the tool name, so it is not the line under test: %q", got)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf(reachGuard+"os.Pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = saved }()

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()
	if err := w.Close(); err != nil {
		t.Fatalf(reachGuard+"close pipe writer: %v", err)
	}
	out := <-done
	if err := r.Close(); err != nil {
		t.Fatalf(reachGuard+"close pipe reader: %v", err)
	}
	return out
}
