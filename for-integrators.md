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

## What you don't have to do

- You don't have to embed llm-bridge as a library — the SSE feed is enough.
- You don't have to import any harness bridge.
- You don't have to rewrite your UI.
- You don't have to remove your existing direct-CLI code path. They coexist.

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
