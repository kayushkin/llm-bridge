# llm-bridge

Canonical message types and bridge interfaces for building LLM integrations in Go. This is the core library — all provider bridges, harness bridges, stores, and UI components in the ecosystem import from here.

Every component is a separate repo and **completely optional**. Use only what you need.

## Architecture

```
                          ┌─────────────────────────────────┐
                          │        llm-bridge-server        │
                          │  HTTP gateway + SSE streaming   │
                          │  Session lifecycle management   │
                          └──────┬──────────┬───────────────┘
                                 │          │
              ┌──────────────────┤          ├──────────────────┐
              │                  │          │                  │
     ┌────────▼────────┐  ┌─────▼────┐  ┌──▼───────────┐  ┌──▼──────────┐
     │  Harness Bridges │  │  Stores  │  │   Provider   │  │     UI      │
     │  (subprocesses)  │  │          │  │   Bridges    │  │             │
     │                  │  │ agent    │  │  (stateless  │  │ bridge-ui   │
     │ claudecode       │  │ harness  │  │  converters) │  │ llmux       │
     │ jig, codex       │  │ memory   │  │              │  │             │
     │ hermes, ...      │  │ log      │  │ anthropic    │  └─────────────┘
     │                  │  │ model    │  │ openai       │
     └────────┬─────────┘  └──────────┘  │ gemini       │
              │                          │ openrouter   │
              ▼                          └──────────────┘
     ┌──────────────────┐
     │   Agent CLIs     │
     │  Claude Code,    │
     │  Codex, Aider,   │
     │  Goose, ...      │
     └──────────────────┘
```

**Data flow:** Harness bridges spawn agent subprocesses and translate their native output into canonical `msg.Event` streams. Provider bridges convert canonical `msg.Conversation` objects to/from provider wire formats. The server orchestrates everything and exposes it over HTTP + SSE. The UI consumes the SSE stream.

## Packages

### `msg` — Canonical message types

The lingua franca of the ecosystem. All bridges, stores, and UI components work with these types.

```go
import "github.com/kayushkin/llm-bridge/msg"
```

**Core types:**
- `Conversation` — Full conversation state: messages, tools, generation config, provider-specific options
- `Message` — Single message with role, content blocks, and metadata
- `ContentBlock` — Polymorphic content: `TextBlock`, `ImageBlock`, `AudioBlock`, `VideoBlock`, `DocumentBlock`, `ToolUseBlock`, `ToolResultBlock`, `ThinkingBlock`, `CodeExecBlock`, and more
- `CompletionResponse` — Parsed LLM response with choices, usage, and raw provider JSON
- `Event` — Canonical harness event (result, stream, tool_call, tool_result, thinking, error, etc.)
- `StreamEvent` — Granular streaming events (block start/delta/stop, message delta)
- `ToolDef` / `ToolChoice` — Tool definitions and selection modes
- `GenerationConfig` — Temperature, top-p, max tokens, stop sequences
- `TokenUsage` / `Cost` — Token counts and cost tracking
- `Session` / `Instance` — Session and harness instance metadata

**Provider-specific config** is carried in typed fields (`AnthropicConfig`, `OpenAIConfig`, `GeminiConfig`, `OpenRouterConfig`) so provider bridges can access native options without losing type safety.

All types include `Extensions map[string]any` and `Overflow map[string]json.RawMessage` fields for forward compatibility — unknown provider fields are preserved, not dropped.

### `bridge` — Bridge interfaces

```go
import "github.com/kayushkin/llm-bridge/bridge"
```

Two interfaces define the contract:

**`APIBridge`** — Stateless provider format conversion. No HTTP calls, no connections. The caller handles transport.

```go
type APIBridge interface {
    Provider() msg.Provider
    BuildRequest(conv *msg.Conversation) ([]byte, error)
    ParseResponse(data []byte) (*msg.CompletionResponse, error)
    ParseStreamEvent(data []byte) (*msg.StreamEvent, error)
}
```

**`HarnessBridge`** — Session lifecycle management. Owns transport (subprocess, WebSocket, file tailing).

```go
type HarnessBridge interface {
    Harness() msg.Harness
    Start(ctx context.Context, prompt string, config json.RawMessage) (HarnessSession, error)
    Resume(ctx context.Context, sessionID string, prompt string, config json.RawMessage) (HarnessSession, error)
}
```

**`HarnessSession`** — A running session that emits `msg.Event` on a channel.

**`StreamReader`** — Optional interface for API bridges that support SSE/NDJSON streaming.

### `bridgeutil` — Schema drift detection

Utilities for detecting when provider APIs add new fields that aren't yet mapped to canonical types.

## Ecosystem

Everything below is a separate repository. Install only what your project needs.

### Server

| Repo | Description |
|------|-------------|
| [llm-bridge-server](https://github.com/kayushkin/llm-bridge-server) | Central HTTP gateway. Manages harness lifecycle, SSE event streaming, credential bindings. Orchestrates agent-store, memory-store, harness-store, and model-store into a single API surface. |
| [llm-bridge-adapter](https://github.com/kayushkin/llm-bridge-adapter) | NATS bus adapter. Bridges llm-bridge-server with the inber messaging ecosystem, translating between NATS pub/sub and HTTP/SSE. |

### Provider Bridges

Stateless Go libraries that convert `msg.Conversation` to/from provider wire formats. You call `BuildRequest`, send the bytes yourself, then call `ParseResponse` on what comes back.

| Repo | Provider | Status | What it does |
|------|----------|--------|--------------|
| [llm-bridge-anthropic](https://github.com/kayushkin/llm-bridge-anthropic) | Anthropic Claude | Implemented | Builds `MessageNewParams`, parses responses. Handles alternation fixing, tool choice, thinking blocks, images, documents. |
| [llm-bridge-openai](https://github.com/kayushkin/llm-bridge-openai) | OpenAI | Implemented | Builds `ChatCompletionRequest`, parses responses. Tool handling, response formats, streaming. |
| [llm-bridge-gemini](https://github.com/kayushkin/llm-bridge-gemini) | Google Gemini | Scaffold | Structure in place, conversion logic pending. |
| [llm-bridge-openrouter](https://github.com/kayushkin/llm-bridge-openrouter) | OpenRouter | Scaffold | Structure in place, conversion logic pending. |

**Usage example:**

```go
import (
    "github.com/kayushkin/llm-bridge/msg"
    anthropic "github.com/kayushkin/llm-bridge-anthropic"
)

conv := &msg.Conversation{
    Messages: []msg.Message{
        {Role: msg.RoleUser, Content: []msg.ContentBlock{msg.TextBlock{Text: "Hello"}}},
    },
    Config: msg.GenerationConfig{MaxTokens: 1024},
}

body, _ := anthropic.BuildRequest(conv)
// POST body to https://api.anthropic.com/v1/messages
// then parse the response:
resp, _ := anthropic.ParseResponse(responseBody)
```

### Harness Bridges

Go binaries that wrap agent CLIs as managed subprocesses. They communicate with llm-bridge-server via stdin/stdout NDJSON (JSON-RPC requests in, `msg.Event` stream out). Each implements the `HarnessBridge` interface.

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
| [llm-bridge-aider](https://github.com/kayushkin/llm-bridge-aider) | Aider | Scaffold | CLI wrapper. |
| [llm-bridge-goose](https://github.com/kayushkin/llm-bridge-goose) | Goose | Scaffold | Agent framework bridge. |
| [llm-bridge-roocode](https://github.com/kayushkin/llm-bridge-roocode) | Roo Code | Scaffold | CLI wrapper. |
| [llm-bridge-kilocode](https://github.com/kayushkin/llm-bridge-kilocode) | Kilo Code | Scaffold | CLI wrapper. |
| [llm-bridge-autohand](https://github.com/kayushkin/llm-bridge-autohand) | Autohand | Scaffold | ACP-over-stdio bridge. |
| [llm-bridge-dexto](https://github.com/kayushkin/llm-bridge-dexto) | Dexto | Scaffold | REST+SSE client. |

### Stores

Go libraries with SQLite backends. Each is independently usable — import the one you need. llm-bridge-server composes all of them into a unified API.

| Repo | Description | Key types |
|------|-------------|-----------|
| [harness-store](https://github.com/kayushkin/harness-store) | Registry of harness instances deployed across machines (local or SSH). Tracks credential bindings with priority and concurrency limits. | `Store`, `Instance`, `HarnessType` |
| [memory-store](https://github.com/kayushkin/memory-store) | Persistent vector memory with semantic search, importance decay, compaction, and context building. Pluggable backend via `MemoryStore` interface. | `Store`, `Memory` (content, embeddings, tags, importance, expiry) |
| [log-store](https://github.com/kayushkin/log-store) | Durable event log service. Stores events as JSONL by date/source, materializes message history on read. Includes an HTTP client library for pushing events. | `POST /api/v1/events`, `GET /api/v1/sessions/{id}/messages` |
| [agent-store](https://github.com/kayushkin/agent-store) | Single source of truth for agent identity and config. Stores identity, runtime configs, tools, limits, and memories. | `Store`, agent CRUD |
| [model-store](https://github.com/kayushkin/model-store) | Centralized model registry, auth, and usage tracking. Manages API keys, OAuth tokens, and model credentials across providers. | `Store`, model/credential CRUD |

### UI

| Repo | Description |
|------|-------------|
| [bridge-ui](https://github.com/kayushkin/bridge-ui) | React component library (`@kayushkin/bridge-ui`). Provides `useBridgeSession`, `useBridgeInstances`, and `useBridgePrefs` hooks plus SSE connection helpers. Peer dependency on React 18+. |
| [llmux](https://github.com/kayushkin/llmux) | Dashboard for deploying and managing harness instances. Built on bridge-ui. Vite + React + TypeScript. |

### TypeScript Types

The `ts/` directory in this repo contains auto-generated TypeScript type definitions (via [tygo](https://github.com/gzuidhof/tygo)) published as `@kayushkin/llm-bridge-types`. This keeps Go and TypeScript types in sync automatically.

## Using in your own project

The simplest useful integration is a provider bridge for making LLM calls:

```go
// go get github.com/kayushkin/llm-bridge
// go get github.com/kayushkin/llm-bridge-anthropic

import (
    "github.com/kayushkin/llm-bridge/msg"
    bridge "github.com/kayushkin/llm-bridge-anthropic"
)

// Build a canonical conversation
conv := &msg.Conversation{
    System:   "You are helpful.",
    Messages: []msg.Message{{Role: msg.RoleUser, Content: []msg.ContentBlock{msg.TextBlock{Text: "Hi"}}}},
    Config:   msg.GenerationConfig{MaxTokens: 256},
}

// Convert to Anthropic wire format
body, err := bridge.BuildRequest(conv)
// Send body to Anthropic API, get response bytes back
resp, err := bridge.ParseResponse(responseBytes)
// resp.Choices[0].Message has the canonical response
```

For agent session management, use a harness bridge directly or go through llm-bridge-server:

```go
// Direct harness usage
harness := claudecode.New(claudecode.Config{})
session, _ := harness.Start(ctx, "Fix the tests", nil)
for event := range session.Events() {
    // Handle msg.Event (result, tool_call, thinking, error, etc.)
}
```

For dashboards, use bridge-ui's React hooks against a running llm-bridge-server:

```tsx
import { useBridgeSession } from '@kayushkin/bridge-ui'

function Chat() {
    const { events, send, stop } = useBridgeSession(sessionId)
    // events is a live stream of canonical msg.Event objects
}
```

## Design principles

- **Canonical types are the contract.** All bridges convert to/from `msg.*` types. If you speak the canonical format, you can swap providers or harnesses without changing application code.
- **Bridges are transparent.** Provider and harness bridges pass data through without formatting, truncation, or lossy transforms. Presentation belongs at the edge.
- **Everything is optional.** Need just the types? Import `llm-bridge`. Need Anthropic conversion? Add `llm-bridge-anthropic`. Need session management? Add `llm-bridge-server`. No component forces you to adopt another.
- **Overflow is preserved.** Unknown provider fields land in `Overflow` maps, not the garbage collector. Round-tripping through canonical types doesn't lose data.
