# llm-bridge

A unified interface for AI coding agents. Written in Go, with type definitions available for TypeScript and Python.

AI coding agents — Claude Code, Codex, Aider, Goose, Cline, and others — each have their own protocol, CLI interface, and event format. llm-bridge treats every agent as a **black box** and provides a single canonical event stream to your application. You don't need to know which agent is running. You just consume `msg.Event`.

Every component is a separate repo and completely optional. Use only what you need.

## How it works

```
  ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
    Your Application  (dashboard, CLI, bot, anything)
  └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┬ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
                            │ msg.Event via HTTP/SSE
  ╔═════════════════════════╪═════════════════════════════╗
  ║              llm-bridge ecosystem                     ║
  ║                         │                             ║
  ║   ┌─────────────────────▼───────────────────────┐     ║
  ║   │            llm-bridge-server                │     ║
  ║   │     HTTP gateway + SSE event streaming      │     ║
  ║   │                                             │     ║
  ║   │  Sessions: start, send, stop, resume, fork, │     ║
  ║   │    interrupt, compact, config, discover     │     ║
  ║   │                                             │     ║
  ║   │  Optional stores:                           │     ║
  ║   │    agent-store · model-store                │     ║
  ║   │    harness-store · memory-store · log-store │     ║
  ║   └─────────────────────┬───────────────────────┘     ║
  ║                         │ stdin/stdout NDJSON          ║
  ║   ┌─────────────────────▼───────────────────────┐     ║
  ║   │            Harness Bridges                  │     ║
  ║   │     One per agent, translates native        │     ║
  ║   │     protocol → canonical msg.Event          │     ║
  ║   │                                             │     ║
  ║   │  claudecode · jig · codex · hermes · aider  │     ║
  ║   │  goose · openclaw · nanoclaw · cline        │     ║
  ║   │  roocode · kilocode · commander             │     ║
  ║   │  autohand · dexto · inber                   │     ║
  ║   └─────────────────────┬───────────────────────┘     ║
  ╚═════════════════════════╪═════════════════════════════╝
                            │ native protocol (varies)
  ┌─────────────────────────▼───────────────────────────┐
  │                    Agent CLIs                       │
  │   Claude Code, Codex, Aider, Goose, Cline, ...     │
  │          (completely opaque — black boxes)          │
  └─────────────────────────────────────────────────────┘
```

Your application sits on one side, the agents sit on the other, and the llm-bridge ecosystem handles everything in between. The harness bridge is the only component that knows an agent's native protocol. Everything above it — the server, the stores, your code — just sees canonical events.

## What you get from every agent

Regardless of which agent is behind the harness, your application receives a uniform set of capabilities through the server API:

| Capability | Description |
|------------|-------------|
| **Event streaming** | Real-time `msg.Event` stream over SSE — results, tool calls, tool results, thinking, errors, state changes |
| **Action approval** | Approval events surface tool/command permission requests; your app can confirm or deny |
| **Session lifecycle** | Start, stop, resume, and discover sessions across any harness |
| **Forking** | Fork a session to branch a conversation from a specific point |
| **Compaction** | Compact a session's context to stay within token limits |
| **Interruption** | Interrupt a running session mid-turn, then resume or send a new message |
| **Configuration** | Update session config (model, tools, permissions) on the fly |
| **Usage tracking** | Token counts, cost, duration, API call breakdowns per session |
| **Task tracking** | Structured task/todo state from agents that support it |
| **Thinking/planning** | Extended thinking and plan events surfaced from agents that emit them |
| **Message history** | Materialized conversation history via log-store (optional) |

## Packages

### `msg` — Canonical message types

The lingua franca of the ecosystem. All bridges, stores, and consumers work with these types.

```go
import "github.com/kayushkin/llm-bridge/msg"
```

```typescript
import { Message, Event, Conversation } from '@kayushkin/llm-bridge-types'
```

```python
from llm_bridge_types import Message, Event, Conversation
```

**Core types:**
- `Conversation` — Full conversation state: messages, tools, generation config, provider-specific options
- `Message` — Single message with role, content blocks, and metadata
- `ContentBlock` — Polymorphic content: `TextBlock`, `ImageBlock`, `AudioBlock`, `VideoBlock`, `DocumentBlock`, `ToolUseBlock`, `ToolResultBlock`, `ThinkingBlock`, `CodeExecBlock`, and more
- `CompletionResponse` — Parsed LLM response with choices, usage, and raw provider JSON
- `Event` — Canonical harness event (`result`, `stream`, `tool_call`, `tool_result`, `thinking`, `error`, `session_state`, `approval`, `plan`)
- `StreamEvent` — Granular streaming deltas (block start/delta/stop, message delta)
- `ToolDef` / `ToolChoice` — Tool definitions and selection modes
- `GenerationConfig` — Temperature, top-p, max tokens, stop sequences
- `TokenUsage` / `Cost` — Token counts and cost tracking
- `Session` / `Instance` — Session and harness instance metadata

All types include `Extensions map[string]any` and `Overflow map[string]json.RawMessage` fields for forward compatibility — unknown fields are preserved, not dropped.

### `bridge` — Bridge interfaces

```go
import "github.com/kayushkin/llm-bridge/bridge"
```

**`HarnessBridge`** — Session lifecycle for an agent harness. Owns transport (subprocess, WebSocket, etc.).

```go
type HarnessBridge interface {
    Harness() msg.Harness
    Start(ctx context.Context, prompt string, config json.RawMessage) (HarnessSession, error)
    Resume(ctx context.Context, sessionID string, prompt string, config json.RawMessage) (HarnessSession, error)
}
```

**`HarnessSession`** — A running session that emits `msg.Event` on a channel.

```go
type HarnessSession interface {
    ID() string
    Events() <-chan msg.Event
    Stop() error
}
```

**`APIBridge`** — Stateless format conversion between canonical types and provider wire formats (Anthropic, OpenAI, etc.). No HTTP calls — the caller handles transport.

**`StreamReader`** — Optional interface for reading provider SSE/NDJSON streams.

### `bridgeutil` — Schema drift detection

Utilities for detecting when provider APIs add new fields that aren't yet mapped to canonical types.

### Generated type packages

Type definitions are auto-generated from the canonical Go types to keep all languages in sync.

| Language | Package | Generator | Source |
|----------|---------|-----------|--------|
| TypeScript | `@kayushkin/llm-bridge-types` | [tygo](https://github.com/gzuidhof/tygo) | `ts/` directory in this repo |
| Python | `llm-bridge-types` | `cmd/genpy` (Go AST → Python dataclasses) | `py/` directory in this repo |

## Ecosystem

Everything below is a separate repository. Install only what your project needs.

### [llm-bridge-server](https://github.com/kayushkin/llm-bridge-server)

Central HTTP gateway and session server. Spawns harness bridges as subprocesses, manages their lifecycle, and streams their `msg.Event` output to clients over SSE. Handles credential bindings and session operations (start, stop, resume, fork, compact, interrupt).

Optionally composes store libraries (see below) for agent identity, model registry, harness tracking, memory, and event logging.

### [llm-bridge-adapter](https://github.com/kayushkin/llm-bridge-adapter)

NATS bus adapter. Bridges llm-bridge-server with the [inber](https://github.com/kayushkin/inber) messaging ecosystem, translating between NATS pub/sub and HTTP/SSE.

### Harness Bridges

Each harness bridge wraps a single agent CLI or API as a black box. It knows how to spawn the agent, speak its native protocol, and translate everything into canonical `msg.Event` streams. The agent's internals are completely opaque — the harness is the translation layer, and the server (or any consumer) only sees uniform events.

Harness bridges communicate with llm-bridge-server via stdin/stdout NDJSON (JSON-RPC requests in, `msg.Event` stream out). Each implements the `HarnessBridge` interface.

| Repo | Agent | Status | Notes |
|------|-------|--------|-------|
| [llm-bridge-claudecode](https://github.com/kayushkin/llm-bridge-claudecode) | Claude Code | Implemented | Wraps `claude` CLI via `--input-format stream-json`. Session resume/fork, message injection, usage aggregation. |
| [llm-bridge-jig](https://github.com/kayushkin/llm-bridge-jig) | Claude Code (profiles) | Implemented | Profile manager. Loads YAML profiles from `.jig/profiles/` with inheritance and env var substitution. |
| [llm-bridge-codex](https://github.com/kayushkin/llm-bridge-codex) | Codex | Scaffold | WebSocket client to Codex app-server JSON-RPC API. |
| [llm-bridge-hermes](https://github.com/kayushkin/llm-bridge-hermes) | Hermes | Scaffold | HTTP+SSE client for OpenAI-compatible Hermes API. |
| [llm-bridge-inber](https://github.com/kayushkin/llm-bridge-inber) | Inber | Scaffold | HTTP client for inber agent framework. |
| [llm-bridge-openclaw](https://github.com/kayushkin/llm-bridge-openclaw) | OpenClaw | Scaffold | WebSocket client. |
| [llm-bridge-nanoclaw](https://github.com/kayushkin/llm-bridge-nanoclaw) | NanoClaw | Scaffold | Docker container subprocess harness. |
| [llm-bridge-commander](https://github.com/kayushkin/llm-bridge-commander) | Commander | Scaffold | Rust/Tauri desktop app bridge. |
| [llm-bridge-cline](https://github.com/kayushkin/llm-bridge-cline) | Cline | Scaffold | CLI wrapper. |
| [llm-bridge-gemini](https://github.com/kayushkin/llm-bridge-gemini) | Gemini CLI | Scaffold | CLI wrapper for Google's Gemini CLI agent. |
| [llm-bridge-aider](https://github.com/kayushkin/llm-bridge-aider) | Aider | Scaffold | CLI wrapper. |
| [llm-bridge-goose](https://github.com/kayushkin/llm-bridge-goose) | Goose | Scaffold | Agent framework bridge. |
| [llm-bridge-roocode](https://github.com/kayushkin/llm-bridge-roocode) | Roo Code | Scaffold | CLI wrapper. |
| [llm-bridge-kilocode](https://github.com/kayushkin/llm-bridge-kilocode) | Kilo Code | Scaffold | CLI wrapper. |
| [llm-bridge-autohand](https://github.com/kayushkin/llm-bridge-autohand) | Autohand | Scaffold | ACP-over-stdio bridge. |
| [llm-bridge-dexto](https://github.com/kayushkin/llm-bridge-dexto) | Dexto | Scaffold | REST+SSE client. |

To add support for a new agent, implement `HarnessBridge` in a new repo. The server and all consumers pick it up automatically.

### Provider Bridges

Stateless Go libraries that convert `msg.Conversation` to/from provider wire formats. These are **not** LLM API clients — you call `BuildRequest` to get bytes, make the HTTP call yourself, then call `ParseResponse` on what comes back.

| Repo | Provider | Status |
|------|----------|--------|
| [llm-bridge-anthropic](https://github.com/kayushkin/llm-bridge-anthropic) | Anthropic Claude | Implemented |
| [llm-bridge-openai](https://github.com/kayushkin/llm-bridge-openai) | OpenAI | Implemented |
| [llm-bridge-google](https://github.com/kayushkin/llm-bridge-google) | Google Gemini | Implemented |
| [llm-bridge-openrouter](https://github.com/kayushkin/llm-bridge-openrouter) | OpenRouter | Scaffold |

### Stores (optional)

Go libraries with SQLite backends. Each is independently usable — import one without importing any others. llm-bridge-server can optionally compose them for a richer API, but none are required.

| Repo | Description |
|------|-------------|
| [agent-store](https://github.com/kayushkin/agent-store) | Agent identity and config. Stores identity, runtime configs, tools, limits, and memories. |
| [model-store](https://github.com/kayushkin/model-store) | Model registry, auth, and usage tracking. Manages API keys, OAuth tokens, and model credentials across providers. |
| [harness-store](https://github.com/kayushkin/harness-store) | Registry of harness instances deployed across machines (local or SSH). Credential bindings with priority and concurrency limits. |
| [memory-store](https://github.com/kayushkin/memory-store) | Persistent vector memory with semantic search, importance decay, compaction, and context building. Pluggable backend via `MemoryStore` interface. |
| [log-store](https://github.com/kayushkin/log-store) | Durable event log. Stores events as JSONL by date/source, materializes message history on read. Includes an HTTP client library. |

### Example consumers

These projects consume the llm-bridge ecosystem and serve as reference implementations:

| Project | Description |
|---------|-------------|
| [bridge-ui](https://github.com/kayushkin/bridge-ui) | React component library (`@kayushkin/bridge-ui`). Session hooks, instance management, SSE helpers. |
| [llmux](https://github.com/kayushkin/llmux) | Dashboard for deploying and managing harness instances. Built on bridge-ui. |

## Quick start

### Consume events from a harness (Go)

```go
import "github.com/kayushkin/llm-bridge/msg"

// Connect to llm-bridge-server SSE endpoint
// GET /sessions/{id}/events
// Each event is a msg.Event — same type regardless of which agent is running
for event := range events {
    switch event.Type {
    case msg.EventResult:
        fmt.Println(event.Result.Message.Content)
    case msg.EventToolCall:
        fmt.Println("Tool:", event.ToolCall.Name)
    case msg.EventError:
        fmt.Println("Error:", event.Error.Message)
    }
}
```

### Use a harness bridge directly (Go)

```go
import claudecode "github.com/kayushkin/llm-bridge-claudecode"

harness := claudecode.New(claudecode.Config{})
session, _ := harness.Start(ctx, "Fix the tests", nil)
for event := range session.Events() {
    // Canonical msg.Event — same shape for every agent
}
```

### Consume events from a harness (TypeScript)

```typescript
import type { Event } from '@kayushkin/llm-bridge-types'

const events = new EventSource(`${serverURL}/sessions/${id}/events`)
events.onmessage = (e) => {
    const event: Event = JSON.parse(e.data)
    // Same canonical shape regardless of agent
}
```

### Consume events from a harness (Python)

```python
from llm_bridge_types import Event
import json, sseclient

response = requests.get(f"{server_url}/sessions/{id}/events", stream=True)
for event in sseclient.SSEClient(response).events():
    e: Event = json.loads(event.data)
    # Same canonical shape regardless of agent
```

## Design principles

- **Agents are black boxes.** A harness bridge is the only thing that knows an agent's native protocol. Everything above the harness sees one uniform event stream. Swap agents without changing your application.
- **Bridges are transparent.** No formatting, truncation, or lossy transforms in bridge layers. Data passes through unchanged. Presentation belongs at the edge.
- **Everything is optional.** Need just the types? Import `llm-bridge`. Need session management? Add the server. Need memory? Add memory-store. No component forces you to adopt another.
- **Overflow is preserved.** Unknown fields land in `Overflow` maps, not the garbage collector. Round-tripping through canonical types doesn't lose data.
