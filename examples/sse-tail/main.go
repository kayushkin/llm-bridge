// Package main is the canonical "additive integration" sample for
// llm-bridge. It connects to llm-bridge-server's per-session SSE feed,
// parses each frame as a canonical msg.Event, and prints a one-line
// summary per event type.
//
// This is the smallest useful consumer of the SSE wire — about 150 lines
// with no third-party dependencies. Other projects can copy the parse
// loop and the type switch verbatim and adapt the render() function to
// their own UI.
//
// Run:
//
//	go run ./examples/sse-tail -server http://localhost:8160 -session <id>
//
// Resume after disconnect (SSE replay from a known event id):
//
//	go run ./examples/sse-tail -session <id> -last-event-id 42
//
// See for-integrators.md for the broader integration story and
// msg/CONVENIENCE-EVENTS.md for the agent_state / usage_total /
// turn_complete derivation rules.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/kayushkin/llm-bridge/msg"
)

func main() {
	server := flag.String("server", "http://localhost:8160", "llm-bridge-server base URL")
	session := flag.String("session", "", "session id to tail (required)")
	lastEventID := flag.String("last-event-id", "", "resume from this SSE id (optional)")
	flag.Parse()

	if *session == "" {
		fmt.Fprintln(os.Stderr, "error: -session is required")
		flag.Usage()
		os.Exit(2)
	}

	url := strings.TrimRight(*server, "/") + "/sessions/" + *session + "/events"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fail("build request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if *lastEventID != "" {
		req.Header.Set("Last-Event-ID", *lastEventID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("dial %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fail("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := tail(resp.Body); err != nil {
		fail("stream: %v", err)
	}
}

// tail reads SSE frames (blank-line-separated) and renders the canonical
// msg.Event from each frame's data line. The id and event lines are
// decorative — msg.Event already carries Type, and the row id is just
// stamped in for reference.
func tail(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var data, lastID string
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if data != "" {
				render(data, lastID)
			}
			data, lastID = "", ""
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case strings.HasPrefix(line, "id: "):
			lastID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: close"):
			fmt.Println("# server closed stream")
		}
	}
	return scanner.Err()
}

// render decodes one frame and prints a concise summary. It is the
// canonical place to wire incoming events into your own UI: switch on
// ev.Type, pull the few fields you care about per type, ignore the
// rest. Unknown event types added upstream land in Overflow so they
// don't break this consumer.
func render(data, id string) {
	var ev msg.Event
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		fmt.Printf("[%s] parse-error: %v\n", id, err)
		return
	}
	prefix := fmt.Sprintf("[%s] %-14s", id, ev.Type)
	switch ev.Type {
	case msg.EventResult:
		if ev.Result != nil {
			fmt.Printf("%s %s\n", prefix, truncate(ev.Result.Text, 200))
		}
	case msg.EventStream:
		fmt.Println(prefix)
	case msg.EventToolCall:
		if ev.ToolCall != nil {
			fmt.Printf("%s %s(%s)\n", prefix, ev.ToolCall.Name, truncate(string(ev.ToolCall.Input), 80))
		}
	case msg.EventToolResult:
		if ev.ToolResult != nil {
			fmt.Printf("%s %s -> %s\n", prefix, ev.ToolResult.Name, truncate(ev.ToolResult.Output, 120))
		}
	case msg.EventThinking:
		if ev.Thinking != nil {
			fmt.Printf("%s %s\n", prefix, truncate(ev.Thinking.Text, 120))
		}
	case msg.EventSystem:
		if ev.System != nil {
			fmt.Printf("%s %s: %s\n", prefix, ev.System.Subtype, truncate(ev.System.Message, 120))
		}
	case msg.EventApproval:
		if ev.Approval != nil {
			fmt.Printf("%s %s/%s tool=%s\n", prefix, ev.Approval.Action, ev.Approval.Status, ev.Approval.ToolName)
		}
	case msg.EventError:
		if ev.Error != nil {
			fmt.Printf("%s %s\n", prefix, ev.Error.Message)
		}
	case msg.EventSessionState:
		if ev.State != nil {
			fmt.Printf("%s %s (was %s) %s\n", prefix, ev.State.State, ev.State.Previous, ev.State.Reason)
		}
	case msg.EventAgentState:
		if ev.AgentState != nil {
			fmt.Printf("%s %s (was %s) %s\n", prefix, ev.AgentState.State, ev.AgentState.Previous, ev.AgentState.Reason)
		}
	case msg.EventUsageTotal:
		if ev.UsageTotal != nil {
			u := ev.UsageTotal.Usage
			fmt.Printf("%s turns=%d in=%d out=%d total=%d\n", prefix, ev.UsageTotal.Turns, u.InputTokens, u.OutputTokens, u.TotalTokens)
		}
	case msg.EventTurnComplete:
		if ev.TurnComplete != nil {
			tc := ev.TurnComplete
			fmt.Printf("%s turn=%s tools=%d duration=%dms err=%t\n", prefix, tc.TurnID, len(tc.ToolCalls), tc.DurationMS, tc.IsError)
		}
	default:
		fmt.Println(prefix)
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return truncateAtRuneBoundary(s, n) + "…"
	}
	return s
}

// truncateAtRuneBoundary returns the longest prefix of s that is at most
// maxBytes long and does not split a rune.
//
// Cutting a string at a fixed byte offset splits whatever rune straddles that
// offset, and the result is not valid UTF-8. Nothing reports it: encoding/json
// substitutes U+FFFD rather than returning an error, so a reader sees a
// replacement character and no error is raised anywhere along the path.
//
// The ellipsis is the caller's, and it sits outside maxBytes — this function
// changes where the cut lands, not what the budget covers.
func truncateAtRuneBoundary(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	// s[cut] is the first byte past the prefix. While it is a continuation
	// byte, a rune straddles the cut, so move the cut earlier.
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "sse-tail: "+format+"\n", args...)
	os.Exit(1)
}
