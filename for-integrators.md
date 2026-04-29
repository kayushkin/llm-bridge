# Integrating llm-bridge into your project

You maintain a project that wraps Claude Code, Codex, Aider, or another coding agent. This doc tells you what's involved in adding llm-bridge as a backend option and what you get out of it.

## What llm-bridge is, in 60 seconds

llm-bridge defines a canonical event type — `msg.Event` — for any coding-agent session, and ships a server (`llm-bridge-server`) that fronts agent harnesses with HTTP+SSE. Per-agent harness bridges translate native protocols (Claude Code stream-json, Codex JSON-RPC, Aider stdout, ...) into that one canonical event stream.

You don't need to adopt the whole stack. The minimum integration is "subscribe to SSE on a server URL your user provides."

## What you get

- **Stop parsing JSONL.** If you tail `~/.claude/projects/*.jsonl` today, you get a structured event stream instead of fragile file-tail parsing.
- **One client, N agents.** The same code path serves Claude Code, Codex, Aider, Goose, and more, as their bridges land. No per-agent driver code.
- **Session ops for free.** start, stop, resume, fork, compact, interrupt — over HTTP, no subprocess plumbing on your side.
- **Forward-compatible.** Unknown fields land in an `Overflow` map; new event types added upstream don't break old consumers.
- **No lock-in.** Each piece is independently usable. Your project can use just the types, just the server SSE feed, or the full stack — your call.

## The recommended integration pattern: additive

Add a config option or env var, e.g. `LLM_BRIDGE_SERVER_URL`. When unset, your existing code runs unchanged. When set, your app subscribes to SSE events from llm-bridge-server instead of (or alongside) your current source.

Why additive first:

- Small diff, easy to review.
- Opt-in only — current users see zero behavior change.
- Reversible — flip the env var off and you're back where you started.
- Easy to promote to default later once the experience is good.

For most projects, this is one new file (the SSE client + event mapping) plus one or two call-site changes wrapped behind a feature check.

## Choosing a session mode (events vs pty)

`POST /sessions` accepts a `mode` field that picks the I/O contract for the lifetime of that session:

- **`events` (default)** — harness emits structured `msg.Event` records over SSE at `GET /sessions/{id}/events`. Use this when you're building your own UI: chat panes, log viewers, dashboards. Normalized across agents, forward-compatible, what most integrations want.
- **`pty`** — the upstream CLI runs inside a pseudoterminal you attach to over WebSocket at `GET /sessions/{id}/attach`. Bytes pass through verbatim, no event derivation. Use this when you want the user to see exactly what running `claude` (or `codex`, etc.) by hand looks like — terminal multiplexers, ssh-like attach UIs, anything that already speaks xterm.

The two modes are independent: a session is either-or, picked at spawn time. If you're not sure, default to events — it's the strictly more abstracted surface and easier to fall back from.

### Per-harness support

Pty mode is opt-in per harness because not every harness has a subprocess to wire into a pty. CLI-based harnesses (claudecode today; codex on the roadmap) advertise `pty: true`. HTTP-backed harnesses (hermes, dexto, inber) advertise `pty: false`. Discover at runtime:

```bash
curl -s "${serverURL}/harnesses/claude_code/capabilities" | jq .pty
# true
```

`POST /sessions` with `mode: "pty"` against a harness that does not support it returns `400 Bad Request` with `error.code = "pty_unsupported"`.

### Tiny end-to-end example

```bash
# 1. Spawn a pty-mode session.
curl -s -X POST "${serverURL}/sessions" \
  -H 'Content-Type: application/json' \
  -d '{
    "harness": "claude_code",
    "instance": "<your-instance-id>",
    "mode": "pty"
  }'
# {"id":"<sess>", ... "mode":"pty"}

# 2. Attach via WebSocket. Binary frames carry pty bytes both directions;
#    text frames carry JSON control messages.
wscat -c "${serverURL/http/ws}/sessions/<sess>/attach"
```

For the full wire format (binary vs text frames, control message schema, single-writer / multi-reader policy, auth posture) and the spec's open questions, see [`PTY-MODE.md`](https://github.com/kayushkin/llm-bridge-server/blob/main/PTY-MODE.md) in `llm-bridge-server`.

## Minimum integration (Go)

```go
import "github.com/kayushkin/llm-bridge/msg"

// GET /sessions/{id}/events on llm-bridge-server returns SSE; each event is one msg.Event.
for ev := range events {
    switch ev.Type {
    case msg.EventResult:
        // ev.Result.Message holds the assistant's reply
    case msg.EventToolCall:
        // ev.ToolCall.Name + Input
    case msg.EventToolResult:
        // ev.ToolResult.Content
    case msg.EventError:
        // ev.Error.Message
    }
}
```

## Minimum integration (TypeScript)

```typescript
import type { Event } from '@kayushkin/llm-bridge-types'

const es = new EventSource(`${serverURL}/sessions/${sessionId}/events`)
es.onmessage = (e) => {
    const ev: Event = JSON.parse(e.data)
    // Canonical shape regardless of which agent is running.
}
```

## Minimum integration (Python)

```python
from llm_bridge_types import Event
import json, requests, sseclient

resp = requests.get(f"{server_url}/sessions/{session_id}/events", stream=True)
for sse in sseclient.SSEClient(resp).events():
    ev: Event = json.loads(sse.data)
    # Same canonical shape regardless of agent.
```

## Convenience events: less wiring, same data

In `events` mode, llm-bridge-server pre-derives three high-level events alongside the raw stream so you don't have to re-implement the same state machine in every consumer:

- **`agent_state`** — `idle` / `awaiting_input` / `tool_running` / `error`. Emitted on every transition, body carries the new state, the previous state, and a free-form `reason` (e.g. `tool=Bash`, `approval_required`). Drop-in for a status pill.
- **`usage_total`** — running session totals (tokens, cost, turn count) after every `result`. `Usage` field-by-field cumulative; `Cost` summed where reported; `ContextTokens` / `ContextLimit` are last-value-wins (current-context state, not consumption).
- **`turn_complete`** — coalesced "this user turn finished" with `final_message`, `tool_calls` (every tool_call/tool_result pair seen this turn), `usage_delta`, per-turn `cost`, `duration_ms`, and an `is_error` flag. Emitted once per turn, immediately after the terminating `result` (or `error` for error-terminated turns).

Two properties make these safe to wire directly into your UI:

1. **Always-on emission, opt-in consumption.** The server always emits them, but consumers that don't switch on the new types just see them land in `Overflow` via the existing forward-compat mechanism. You enable them by adding cases; you can't break by ignoring them.
2. **Ordering is deterministic.** The raw event always reaches subscribers first, then the derived events for that same input land in the order `agent_state` → `usage_total` → `turn_complete`. UIs that show both a chat bubble and a status pill can update them in causal order on a single replay.

The four-line consumer pattern that replaces the old re-derivation glue:

```go
switch ev.Type {
case msg.EventAgentState:   updateStatusPill(ev.AgentState.State)         // idle / tool_running / awaiting_input / error
case msg.EventUsageTotal:   updateMeter(ev.UsageTotal.Usage, ev.UsageTotal.Cost)
case msg.EventTurnComplete: appendTurnSummary(ev.TurnComplete)            // tool_calls + usage_delta + duration
}
```

For the full state machine, emission ordering rules, and design rationale, see [`msg/CONVENIENCE-EVENTS.md`](https://github.com/kayushkin/llm-bridge/blob/main/msg/CONVENIENCE-EVENTS.md).

## What you don't have to do

- You don't have to embed llm-bridge as a library — the SSE feed is enough.
- You don't have to import any harness bridge.
- You don't have to rewrite your UI.
- You don't have to remove your existing direct-CLI code path. They coexist.
- You don't have to re-derive `agent_state` / `usage_total` / `turn_complete` from raw events — the server emits them for you (see above).

## Compatibility promise

- `msg.Event` is versioned. New fields land in `Overflow`/`Extensions` so old clients don't break.
- Server SSE wire format is stable across minor versions; breaking changes get a deprecation cycle.
- Type packages (`@kayushkin/llm-bridge-types`, `llm-bridge-types` on PyPI) are auto-generated from the Go canonical types — they stay in sync.

## What an integration PR looks like

A typical first PR:

1. Adds a config flag / env var (e.g. `LLM_BRIDGE_SERVER_URL`).
2. Adds one new file: an SSE client that yields events in your project's existing event/UI shape.
3. Adds a feature check at the existing event source: if flag set → use new path, else → existing path.
4. Documents the new option as **experimental / opt-in** in the README.
5. Adds a small test that mocks the SSE feed.

Goal of this PR is **not** to make llm-bridge default. It's to land a working code path that downstream users can opt into and report bugs against.

## Where to ask questions

- **Canonical types or interface questions:** [github.com/kayushkin/llm-bridge](https://github.com/kayushkin/llm-bridge)
- **Server / SSE / session API:** [github.com/kayushkin/llm-bridge-server](https://github.com/kayushkin/llm-bridge-server)
- **Specific agent behavior:** the relevant harness bridge repo (e.g. `llm-bridge-claudecode`, `llm-bridge-codex`)

For "would you accept a PR for X?" or "is approach Y or Z better?" — open a discussion on `llm-bridge`. We'd rather hash out the shape before you spend a week on implementation.

## Footer

- See [README.md](README.md) for the full ecosystem map.
- See [ARCHITECTURE.md](ARCHITECTURE.md) for design rationale.
- License: Apache-2.0. See [LICENSE](LICENSE).
