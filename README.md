# llm-bridge

llm-bridge runs coding agents (Claude Code, codex, hermes, aider, …) behind one API. Each agent CLI speaks its own protocol; a wrapper per agent turns that into one event type, `msg.Event`, and a session server hands those events to every client the same way.

This repo holds the shared contract: the types every other part reads and writes, and the interfaces a wrapper implements. This README is also the map of the whole system: which repo owns what, and where a given change belongs.

## The system

```
  clients:  dash + bridge-ui · scheduler · bridge-agent · llm-bridge-tui · llm-bridge-adapter (NATS)
                │ HTTP, SSE, WebSocket
                ▼
  ┌───────────────────────────┐      services it asks: principal-store, grant-store,
  │     llm-bridge-server     │ ───► kanban-store, log-store, auth-store, tool-store,
  │  sessions, access, routes │      bundle-store, permission-store, healthcheck
  │  + six embedded stores    │
  └───────────────────────────┘
                │ JSON-RPC in, msg.Event out, as NDJSON over stdin/stdout
                │ (locally, over SSH, or through llm-bridge-runner on another machine)
                ▼
  harness wrapper, one repo per agent:  llm-bridge-claudecode, -codex, -hermes, …
                │ the agent's own protocol
                ▼
  the agent:  claude, codex, a hermes server, …
```

Everything above the wrappers sees only `msg.Event`. Everything below them is the agent's own business.

## Who owns what

### The contract (this repo)

| Package | What it holds |
|---|---|
| `msg/` | Every shared type: `Event`, `Message`, `Conversation`, sessions, instances, signals, `EffectiveConfig`, the harness list (`msg.AllHarnesses`), session purposes |
| `bridge/` | The interfaces a wrapper or a provider converter implements (`HarnessBridge`, `HarnessSession`, `APIBridge`, `AgentReconciler`) |
| `ndjson/` | Reads the NDJSON stream between the server and a wrapper |
| `identity/` | Mints the `message_id` a wrapper puts on each event |
| `render/` | Turns an agent definition into one harness's flags, MCP config and prompt text |
| `servicesettings/` | How any service here declares, reads and serves its settings (`GET /settings`) |
| `bridgeutil/` | Helpers wrappers share, and detection of fields a provider added that `msg/` does not map yet |
| `ts/`, `py/` | `msg/` as TypeScript and Python types, generated; never edit by hand |

This repo imports nothing else here, so everything can import it.

### The session server

[llm-bridge-server](https://github.com/kayushkin/llm-bridge-server) owns sessions, signals, folders and who may call what. It embeds model-store, agent-store, harness-store, hook-store, snapshot-store and memory-store as libraries. Its README lists every route.

### Harness wrappers

One binary per agent. The server starts it; it starts or connects to the agent and translates. It is the only code that knows the agent's protocol.

| Repo | Agent | How it drives the agent |
|---|---|---|
| [llm-bridge-claudecode](https://github.com/kayushkin/llm-bridge-claudecode) | Claude Code | `claude` with stream-json in and out; also pty mode |
| [llm-bridge-jig](https://github.com/kayushkin/llm-bridge-jig) | Claude Code with jig profiles | Loads a YAML profile, then runs `claude` |
| [llm-bridge-codex](https://github.com/kayushkin/llm-bridge-codex) | codex | codex app-server's JSON-RPC |
| [llm-bridge-hermes](https://github.com/kayushkin/llm-bridge-hermes) | hermes | HTTP and SSE to hermes `/v1/responses` |
| [llm-bridge-inber](https://github.com/kayushkin/llm-bridge-inber) | inber | inber's HTTP API |
| [llm-bridge-openclaw](https://github.com/kayushkin/llm-bridge-openclaw) | OpenClaw | HTTP and SSE, plus its session log |
| [llm-bridge-nanoclaw](https://github.com/kayushkin/llm-bridge-nanoclaw) | NanoClaw | A Docker container per session |
| [llm-bridge-cline](https://github.com/kayushkin/llm-bridge-cline) | Cline | `cline -y --json`, one process per turn |
| [llm-bridge-aider](https://github.com/kayushkin/llm-bridge-aider) | aider | `aider --message`, one process per turn |
| [llm-bridge-kilocode](https://github.com/kayushkin/llm-bridge-kilocode) | Kilo Code | `kilo serve` and its HTTP API |
| [llm-bridge-forgecode](https://github.com/kayushkin/llm-bridge-forgecode) | ForgeCode | `forge -p`, one process per turn |
| [llm-bridge-copilotcli](https://github.com/kayushkin/llm-bridge-copilotcli) | GitHub Copilot CLI | Planned only: its code is still a copy of the claudecode wrapper |

`cmd/mock-harness` in llm-bridge-server is a fake agent for tests.

⚠️ **Retired; do not build on them:** `llm-bridge-gemini`, `-commander`, `-goose`, `-roocode`, `-autohand` and `-dexto`. They only print "not yet implemented".

### Reaching the server from elsewhere

| Repo | What it does |
|---|---|
| [llm-bridge-runner](https://github.com/kayushkin/llm-bridge-runner) | A daemon on another machine. Holds one WebSocket open to the server and starts wrappers there, so no inbound SSH is needed |
| [llm-bridge-adapter](https://github.com/kayushkin/llm-bridge-adapter) | Drives sessions from NATS: takes requests on `bridge.inbound`, publishes events on `bridge.event` |
| [llm-bridge-tui](https://github.com/kayushkin/llm-bridge-tui) | Terminal client |
| [bridge-ui](https://github.com/kayushkin/bridge-ui), [chat-core](https://github.com/kayushkin/chat-core), [dash](https://github.com/kayushkin/dash) | The web surface: bridge-ui draws it, chat-core holds the chat's data, dash serves both and signs users in |

### Provider format converters

Libraries that turn a `msg.Conversation` into one provider's request bytes and parse its response. They make no HTTP calls.

| Repo | Provider |
|---|---|
| [llm-bridge-anthropic](https://github.com/kayushkin/llm-bridge-anthropic) | Anthropic |
| [llm-bridge-openai](https://github.com/kayushkin/llm-bridge-openai) | OpenAI |
| [llm-bridge-google](https://github.com/kayushkin/llm-bridge-google) | Google Gemini |
| [llm-bridge-openrouter](https://github.com/kayushkin/llm-bridge-openrouter) | OpenRouter; empty so far |

### Stores

Each store owns one kind of record, in its own repo and database. Take a record's id from the store that owns it.

| Store | Owns | How the server reaches it |
|---|---|---|
| model-store | Models, roles, prices | embedded |
| agent-store | Agents, context files, every prompt | embedded |
| harness-store | Machines, instances, credential bindings | embedded |
| hook-store | Hooks wired into harnesses | embedded |
| snapshot-store | File contents around each Edit or Write | embedded |
| memory-store | Agent memories | embedded |
| log-store | Every session's event history | HTTP |
| principal-store | People and groups | HTTP |
| grant-store | Who may use which instance, agent or tool | HTTP |
| kanban-store | Boards and cards | HTTP |
| auth-store | Credentials | HTTP |
| tool-store, bundle-store | Tools, and the sets a session is given | HTTP |
| permission-store | Rules for tool calls | HTTP |

## Where a change belongs

| To… | Change | Then |
|---|---|---|
| Add a field to an event or session | `msg/` here | Run `./generate-ts.sh` and `./generate-py.sh`; update the wrapper that fills it and the client that reads it |
| Support a new agent CLI | A new `llm-bridge-<name>` wrapper repo | Add it to `msg.AllHarnesses` here; the server gives it a prompt-delivery row on next start |
| Change how one agent is driven | That agent's wrapper only | Nothing else should need to change |
| Add or change a route | llm-bridge-server | Give it an access rule and a description there; its README's route table is generated from them |
| Change who may do what | grant-store or principal-store records | Not code in the server |
| Change what a stored record holds | The store that owns it | Then whoever reads it |
| Support a new provider's wire format | A new `llm-bridge-<provider>` converter | Implement `APIBridge` |
| Add a setting to a Go service | That service, through `servicesettings/` | Never a bare `os.Getenv` |

## The event contract

A session's events arrive as `msg.Event`, one per SSE `data:` line from `GET /sessions/{id}/events`. `Type` says which field is set: `result`, `stream`, `tool_call`, `tool_result`, `thinking`, `system`, `approval`, `error`, `session_state`, `plan`, `session_info`, `user_message`, `hook`.

The server adds three events of its own so clients need not work them out: `agent_state` (idle, awaiting input, tool running, error), `usage_total` (running totals after each result) and `turn_complete` (one summary per turn). [`msg/CONVENIENCE-EVENTS.md`](msg/CONVENIENCE-EVENTS.md) has the rules.

A field a type does not map yet lands in its `Overflow` map rather than being dropped, so passing a record through changes nothing.

## Rules every part keeps

- **Only a wrapper knows its agent's protocol.** Nothing above it may depend on which agent runs.
- **Layers pass data through unchanged.** No formatting, cutting or lossy change between the wrapper and the client; presentation happens in the client.
- **Each record has one owner.** Join on the owner's id, never on a name, and never keep a second copy.
- **Fail loudly.** A missing setting, an unknown field or a store that will not answer is an error, not a quiet default.

## Types for other languages

`ts/` (`@kayushkin/llm-bridge-types`) and `py/` (`llm-bridge-types`) are generated from `msg/` by `./generate-ts.sh` and `./generate-py.sh`, and each file records the commit it came from. They are not published; use them from this repo (`file:../llm-bridge/ts`, `pip install -e py/`).

## Docs

- `docs/ARCHITECTURE.md`: how the layers fit, at more length; older than this README, and partly out of date
- `msg/CONVENIENCE-EVENTS.md`: the three server-made events
- `examples/sse-tail`: a minimal client of the event stream
- `docs/plans/`: the session-identity migration (partly done), and the write-ups from the plan to publish llm-bridge (`RELEASE-TARGETS`, `ACP-SURFACE`, `for-integrators`)
