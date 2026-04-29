# sse-tail

Smallest possible consumer of llm-bridge-server's per-session SSE feed. Tails
`GET /sessions/{id}/events`, decodes each frame as a canonical
[`msg.Event`](../../msg/event.go), and prints a one-line summary keyed by
event type.

This is the canonical reference for the **additive integration** pattern
described in [`for-integrators.md`](../../for-integrators.md): your project
adds an env var or config flag, opens an SSE connection, and renders
canonical events. No need to embed any harness bridge, parse JSONL, or
re-derive per-agent state machines — the server normalizes everything for
you.

## Run

```bash
# Tail an existing session by id.
go run ./examples/sse-tail -session br_1234567890

# Different server.
go run ./examples/sse-tail -server http://other-host:8160 -session br_1234567890

# Resume after disconnect (SSE replay from a known event id).
go run ./examples/sse-tail -session br_1234567890 -last-event-id 41576
```

## Sample output

```
[41576] session_state  running (was idle)
[41577] system         harness_id_set: 6dac0750-8389-4f2a-adfc-c8a4d46f961f
[41579] system         init: model=claude-opus-4-7 cwd=/home/me/repos/foo
[41580] session_info
[41581] thinking
[41583] tool_call      Bash({"command":"go test ./..."})
[41584] tool_result    Bash -> ok      github.com/me/foo  0.234s
[41599] result         All tests pass on main.
[41600] agent_state    idle (was tool_running) approval_required=false
[41601] usage_total    turns=1 in=2456 out=812 total=3268
[41602] turn_complete  turn=t_01 tools=3 duration=4823ms err=false
```

The first column is the SSE row id (use it with `-last-event-id` to resume).
The second column is the canonical `msg.Event.Type`.

## What to copy

The two pieces worth lifting into your project are:

1. The **frame parser** in `tail()` — reads SSE blank-line-separated
   frames and pulls out `data:` and `id:` lines. ~20 lines, no deps.
2. The **type switch** in `render()` — exhaustive over the canonical
   event types, including the convenience events (`agent_state`,
   `usage_total`, `turn_complete`). Replace each `fmt.Printf` with the
   call into your own UI: status pill update, chat bubble append,
   token meter, etc.

Forward-compat: unknown event types added upstream land in the default
branch and `Overflow`. You can ignore them safely until you choose to
handle them.

## Why a single Go file?

The whole sample is intentionally one file (~150 lines) with no
third-party dependencies beyond `github.com/kayushkin/llm-bridge/msg`.
The point is "here is the smallest working consumer; copy and adapt."
A larger sample with framework polish would be less reusable as a
reference.

For TypeScript/Python equivalents see the snippets in
[`../../for-integrators.md`](../../for-integrators.md). A standalone
TypeScript example may follow as a sibling directory once the
`@kayushkin/llm-bridge-types` npm package is published.
