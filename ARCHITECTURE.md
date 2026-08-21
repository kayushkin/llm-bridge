# llm-bridge Ecosystem Architecture

## Overview

llm-bridge is a modular, composable system for unifying access to LLM providers and agent harnesses. It is designed as an OSS project where users can pick the layer they need and ignore the rest.

## Architecture Diagram

```
                          Consumers
                   (dashboards, CLIs, agents)
                             |
                             v
                  +---------------------+
                  |  llm-bridge-server   |  :8160
                  |  (HTTP gateway)      |
                  |                      |
                  |  - sessions          |
                  |  - event streaming   |
                  |  - harness lifecycle |
                  |  - agent/model API   |
                  +----+-----+-----+----+
                       |     |     |
          +------------+     |     +------------+
          |                  |                  |
          v                  v                  v
   +-----------+     +-----------+     +-----------+     +-------------+
   |auth-store |     |model-store|     |agent-store|     |harness-store|
   | (library) |     | (library) |     | (library) |     | (library)   |
   |           |     |           |     |           |     |             |
   | creds     |     | providers |     | identity  |     | instances   |
   | oauth     |     | models    |     | nature    |     | SSH/local   |
   | tokens    |     | aliases   |     | configs   |     | cred binds  |
   | refresh   |     | pricing   |     | tools     |     | concurrency |
   +-----------+     | failover  |     | status    |     +-------------+
        |            +-----------+     +-----------+
        v                                                +--------------+
   +-----------+     +-------------+                     | usage-store  |
   |  aiauth   |     |memory-store |                     | (library)    |
   | (library) |     | (library)   |                     |              |
   | OAuth flows|    |             |                     | tokens       |
   +-----------+     | memories    |                     | cost         |
                     | embeddings  |                     | per-agent    |
                     | context     |                     +--------------+
                     | compaction  |
                     +-------------+
                                                         +--------------+
                                                         |  log-store   |
                                                         |  (service)   |
                                                         |              |
                                                         | event log    |
                                                         | msg history  |
                                                         | JSONL storage|
                                                         +--------------+

   llm-bridge-server spawns harness subprocesses:

                  +---------------------+
                  |  llm-bridge-server   |
                  +----+----+----+------+
                       |    |    |
            +----------+    |    +----------+
            |               |               |
            v               v               v
   +----------------+ +----------------+ +----------------+
   | llm-bridge-    | | llm-bridge-    | | llm-bridge-    |  ...
   | claudecode     | | codex          | | openclaw       |
   | (harness bin)  | | (harness bin)  | | (harness bin)  |
   +-------+--------+ +-------+--------+ +-------+--------+
           |                   |                   |
           v                   v                   v
   +----------------+ +----------------+ +----------------+
   | llm-bridge-    | | llm-bridge-    | | llm-bridge-    |  ...
   | anthropic      | | openai         | | google         |
   | (provider lib) | | (provider lib) | | (provider lib) |
   +----------------+ +----------------+ +----------------+
           |                   |                   |
           v                   v                   v
     Anthropic API       OpenAI API          Gemini API
```

## Repository Map

### Core Library

| Repo | Type | Description |
|------|------|-------------|
| **llm-bridge** | Go library | Canonical message types (`msg/`), bridge interfaces (`bridge/`), schema drift detection (`bridgeutil/`). The lingua franca -- everything imports this. |

### Store Libraries

All stores are **Go libraries** that open their own SQLite file. No server process needed. llm-bridge-server composes them all into one HTTP surface.

| Repo | Type | Description |
|------|------|-------------|
| **auth-store** | Go library | Credential vault. API keys, OAuth tokens, refresh logic, per-model credential bindings, credential health tracking. Imports `aiauth` for OAuth flows. |
| **model-store** | Go library | Provider/model registry. Model aliases, pricing (input/output cost), context windows, failover chains, enable/disable. Pure metadata, no auth. |
| **agent-store** | Go library | Agent identity, nature (principles/values), per-orchestrator configs (model, tools, limits), tool registry, agent status. |
| **memory-store** | Go library | Context management. Persistent memories with embeddings, semantic search, context building, compaction, session tracking, recency decay. Standalone — not tied to agents. |
| **harness-store** | Go library | Harness instance registry. Deployments of harness types on specific machines (local or SSH), credential bindings per instance with priority and concurrency limits. Static config — runtime state (active slots) lives in llm-bridge-server. |
| **log-store** | Go service | Durable event log for llm-bridge sessions. Stores events as JSONL by date/source, materializes message history. llm-bridge-server pushes events to it and proxies `/sessions/{id}/messages` and `/sessions/{id}/history` through it. |
| **usage-store** | Go library | Token usage tracking per agent/orchestrator/model/day. Cost calculation from model-store pricing. Aggregation queries. |
| **aiauth** | Go library | OAuth2 flow runner (browser redirect, token exchange, refresh). Used by auth-store for token lifecycle. |

### Provider Bridges

Convert between canonical `msg` types and provider-specific wire formats. Stateless libraries.

| Repo | Type | Status |
|------|------|--------|
| **llm-bridge-anthropic** | Go library | Implemented (~988 LOC) |
| **llm-bridge-openai** | Go library | Implemented (~807 LOC) |
| **llm-bridge-google** | Go library | Implemented (~901 LOC) |
| **llm-bridge-openrouter** | Go library | Scaffold (~39 LOC) |

### Harness Bridges

Manage agent harness subprocesses. Spawned by llm-bridge-server, communicate via stdin/stdout JSON protocol, emit canonical `msg.Event`.

| Repo | Type | Status |
|------|------|--------|
| **llm-bridge-claudecode** | Go binary | Implemented -- wraps Claude Code CLI (`--input-format stream-json`). Session discovery, history import, auto-approve, work dir support. Session-chain ported. |
| **llm-bridge-jig** | Go binary | Implemented -- profile manager harness for Claude Code. Loads YAML profiles with inheritance and env var substitution. Session-chain ported. |
| **llm-bridge-codex** | Go binary | Implemented -- wraps Codex CLI. Reference impl for session-chain contract. |
| **llm-bridge-openclaw** | Go binary | Implemented -- HTTP+SSE+JSONL-tail (CLAUDE.md's "WebSocket" label is stale). Session-chain ported. |
| **llm-bridge-inber** | Go binary | Implemented -- HTTP API to inber server. Session-chain ported. |
| **llm-bridge-hermes** | Go binary | Implemented -- OpenAI-compatible HTTP+SSE. Session-chain ported (no state.db; server-side state). |
| **llm-bridge-aider** | Go binary | Implemented -- pty subprocess wrapping Aider CLI. Session-chain ported. |
| **llm-bridge-nanoclaw** | Go binary | Implemented -- container subprocess (Docker). Session-chain ported. |
| **llm-bridge-cline** | Go binary | Implemented -- one-shot subprocess wrapping Cline CLI. Session-chain ported (with native taskID rotation). |
| **llm-bridge-kilocode** | Go binary | Implemented -- `kilo serve` subprocess + REST/SSE. Session-chain ported. |
| **llm-bridge-forgecode** | Go binary | Implemented -- wraps `forge -p` one-shot subprocess. Session-chain ported. |
| **llm-bridge-gemini** | Go binary | Scaffold (~24 LOC) -- wraps Gemini CLI |
| **llm-bridge-goose** | Go binary | Scaffold (~24 LOC) -- wraps Goose CLI/API |
| **llm-bridge-autohand** | Go binary | Scaffold (~24 LOC) -- wraps Autohand Code CLI (ACP stdio) |
| **llm-bridge-dexto** | Go binary | Scaffold (~24 LOC) -- REST+SSE client |
| **llm-bridge-commander** | Go binary | Scaffold (~24 LOC) -- subprocess orchestrator |
| **llm-bridge-roocode** | Go binary | Scaffold (~24 LOC) -- wraps Roo Code CLI |
| **llm-bridge-opencode** | Go binary | Not present in this tree; scaffold targeted but never created. Onboarding research in noteboard note `f0e71652`. |

### Service

| Repo | Type | Description |
|------|------|-------------|
| **llm-bridge-server** | Go service | The gateway. HTTP API for sessions, event streaming (SSE), harness lifecycle management. Imports store libraries (agent-store, memory-store, harness-store, model-store, log-store). Auto-discovers harness native sessions on startup and imports history to log-store. Single process to run. |
| **llm-bridge-adapter** | Go service | NATS bus adapter for integrating llm-bridge with the inber messaging ecosystem. Translates bus ChatInbound/ChatOutbound to bridge sessions and SSE events. |
| **log-store** | Go service | Durable event log. Receives events from llm-bridge-server, stores as JSONL, materializes message history. Queried via REST. |

### Retired / To Delete

| Repo | Reason |
|------|--------|
| **llm-msg-spec** | Empty shell, never populated. Types live in `llm-bridge/msg/`. Delete. |
| **msgbridge-anthropic** | Rename to `llm-bridge-anthropic`, migrate implementation. Delete original. |
| **msgbridge-openai** | Rename to `llm-bridge-openai`, migrate implementation. Delete original. |
| **msgbridge-gemini** | Scaffold only. Recreate as `llm-bridge-google`. Delete original. |
| **msgbridge-openrouter** | Scaffold only. Recreate as `llm-bridge-openrouter`. Delete original. |
| **msgbridge-claudecode** | Scaffold only. Functionality covered by `llm-bridge-claudecode`. Delete. |
| **msgbridge-codex** | Scaffold only. Functionality covered by `llm-bridge-codex`. Delete. |
| **msgbridge-openclaw** | Scaffold only. Functionality covered by `llm-bridge-openclaw`. Delete. |

## Import Graph

```
llm-bridge              (no deps -- canonical types + interfaces)
  ^
  |--- auth-store       (imports llm-bridge for provider types, imports aiauth)
  |--- model-store      (imports llm-bridge for provider types)
  |--- agent-store      (imports llm-bridge for harness/event types)
  |--- memory-store     (standalone, no llm-bridge dependency)
  |--- harness-store    (imports llm-bridge for instance types)
  |--- usage-store      (imports llm-bridge for provider types, imports model-store for pricing)
  |
  |--- llm-bridge-anthropic   (imports llm-bridge/msg)
  |--- llm-bridge-openai      (imports llm-bridge/msg)
  |--- llm-bridge-google      (imports llm-bridge/msg)
  |--- llm-bridge-openrouter  (imports llm-bridge/msg)
  |
  |--- llm-bridge-claudecode  (imports llm-bridge/msg, llm-bridge/bridge)
  |--- llm-bridge-jig         (imports llm-bridge/msg, llm-bridge/bridge)
  |--- llm-bridge-codex       (imports llm-bridge/msg, llm-bridge/bridge)
  |--- llm-bridge-openclaw    (imports llm-bridge/msg, llm-bridge/bridge)
  |--- llm-bridge-inber       (imports llm-bridge/msg, llm-bridge/bridge)
  |--- llm-bridge-gemini      (imports llm-bridge/msg, llm-bridge/bridge)
  |--- llm-bridge-hermes      (imports llm-bridge/msg, llm-bridge/bridge)
  |
  +--- llm-bridge-server      (imports stores + orchestrates harness binaries)
       |-- runtime store: sessions, events, credential_slots (internal SQLite)
       |-- imports: agent-store, memory-store, harness-store, model-store, aiauth
       |-- proxies: log-store (HTTP, event push + message/history queries)
       |-- not yet wired: auth-store, usage-store

  +--- llm-bridge-adapter     (imports bus, talks to llm-bridge-server via HTTP/SSE)
```

## Migration Plan

### Phase 1: Split model-store (auth-store + usage-store extraction)

This is the highest-value change -- model-store currently mixes four domains.

**1a. Create auth-store**
- New repo: `auth-store`
- Move from model-store:
  - `credentials.go` -> auth-store root
  - `oauth.go` -> auth-store root
  - Credential-related tables from `store.go`: `credentials`, `model_credentials`
  - `sync.go` OpenClaw sync -> do NOT move. Drop it or move to openclaw-adapter.
- auth-store imports `aiauth` for OAuth flows
- auth-store has its own SQLite file: `~/.config/auth-store/auth.db`

**1b. Create usage-store**
- New repo: `usage-store`
- Move from model-store:
  - `usage.go` -> usage-store root
  - `usage` table from `store.go`
- usage-store imports model-store for pricing lookups (cost calculation)
- Own SQLite file: `~/.config/usage-store/usage.db`

**1c. Slim down model-store**
- What remains in model-store:
  - `providers.go` (provider/model registry)
  - `seed.go` (default provider/model data)
  - `health.go` (model health -- keep here or move to llm-bridge-server later)
  - Tables: `providers`, `models`, `model_aliases`, `model_health`
- Remove: credentials, oauth, usage, sync code
- Remove: aiauth dependency (that moves to auth-store)
- Own SQLite file stays: `~/.config/model-store/store.db`

**1d. Update model-store CLI (`ms`)**
- `ms keys add` -> moves to an auth-store CLI or removed
- `ms usage` -> moves to usage-store CLI or removed
- `ms providers`, `ms models`, `ms resolve`, `ms seed` -> stay

### Phase 2: Fold agent-store server into llm-bridge-server

agent-store becomes a library only. Its HTTP+NATS server surface moves to llm-bridge-server.

**2a. Strip server from agent-store**
- Remove `cmd/server/` from agent-store
- Remove `server.go` HTTP handlers from agent-store root (or keep as handler funcs that llm-bridge-server can mount)
- agent-store becomes: `store.go` + `memory/` + schema + seed tooling
- Keep `cmd/seed/` and `cmd/test-cycle/` as utilities

**2b. Add agent-store routes to llm-bridge-server**
- llm-bridge-server imports agent-store
- Mount agent-store's HTTP routes under `/agents/*` and `/memories/*`
- Migrate NATS handlers (memory.save, memory.search, etc.) to llm-bridge-server
- Mount model-store routes under `/models/*`
- Mount auth-store routes under `/credentials/*`
- Mount usage-store routes under `/usage/*`

**2c. Update model-store server**
- model-store's `cmd/model-store/` HTTP server becomes redundant
- Remove it -- llm-bridge-server is the single server now
- Keep the `cmd/ms/` CLI tool (it uses the library directly)

### Phase 3: Rename msgbridge-* to llm-bridge-*

**3a. Migrate msgbridge-anthropic**
- Create `llm-bridge-anthropic` repo
- Copy `build.go` (416 lines) from msgbridge-anthropic
- Update imports: `llm-msg-spec/api` -> `llm-bridge/msg`
- Fix up type references to match current msg package
- Verify it compiles against llm-bridge/msg

**3b. Migrate msgbridge-openai**
- Same process as anthropic
- Copy `build.go` (235 lines)
- Update imports

**3c. Create scaffold provider bridges**
- `llm-bridge-google` -- new repo, from msgbridge-gemini (renamed: provider bridges use company name)
- `llm-bridge-openrouter` -- new repo, scaffold from msgbridge-openrouter TODOs

**3d. Delete old repos**
- Delete: llm-msg-spec, msgbridge-anthropic, msgbridge-openai, msgbridge-gemini, msgbridge-openrouter, msgbridge-claudecode, msgbridge-codex, msgbridge-openclaw

### Phase 4: Extract llm-bridge-server from llm-bridge

**4a. Create llm-bridge-server repo**
- Move from llm-bridge:
  - `cmd/llm-bridge/` -> `cmd/llm-bridge-server/`
  - `internal/server/` -> server package
  - `internal/store/` -> session store
  - `internal/harness/` -> harness manager
  - `internal/config/` -> config
- llm-bridge-server imports: llm-bridge, auth-store, model-store, agent-store, usage-store

**4b. Slim down llm-bridge to library only**
- What remains:
  - `msg/` -- canonical types
  - `bridge/` -- bridge interfaces
  - `bridgeutil/` -- schema drift detection
- Remove: `cmd/`, `internal/`, `deploy.sh`
- No binary, no server, no SQLite dep. Pure library.

### Phase 5: Wire harness bridges to use provider bridges

**5a. Update llm-bridge-claudecode**
- Import `llm-bridge-anthropic` for Anthropic API format conversion
- Harness manages Claude Code subprocess
- Provider bridge handles message format translation

**5b. Repeat for other harnesses as they mature**

## Progress

### Phase 1: COMPLETE (2026-04-12)

- [x] Created `auth-store` repo with credentials, OAuth, refresh logic
- [x] Created `usage-store` repo with token tracking
- [x] Slimmed `model-store` to pure registry (providers, models, aliases, health)
- [x] Removed `sync.go` (OpenClaw-specific, doesn't belong in generic stores)
- [x] Removed model-store HTTP server (will be replaced by llm-bridge-server)
- [x] Updated `ms` CLI to registry-only commands (providers, models, resolve, seed, enable, disable)
- [x] All three stores compile and pass basic tests

### Phase 3: COMPLETE (2026-04-12)

- [x] Created `llm-bridge-anthropic` with full implementation (build, parse, stream)
- [x] Created `llm-bridge-openai` with full implementation (build, parse, stream)
- [x] Created `llm-bridge-google` (renamed from llm-bridge-gemini, provider bridges use company name)
- [x] Created scaffold `llm-bridge-openrouter`
- [x] Deleted old repos: llm-msg-spec, msgbridge-{anthropic,openai,gemini,openrouter,claudecode,codex,openclaw}

### Phase 4: COMPLETE (2026-04-12)

- [x] Created `llm-bridge-server` repo with HTTP gateway
- [x] Moved `cmd/llm-bridge/` → `llm-bridge-server/cmd/llm-bridge-server/`
- [x] Moved `internal/` (config, store, server, harness) to llm-bridge-server
- [x] Slimmed `llm-bridge` to pure library: `msg/`, `bridge/`, `bridgeutil/` only
- [x] Removed SQLite dependency from llm-bridge
- [x] Both repos compile successfully

### Phase 2: COMPLETE (2026-04-12)

- [x] llm-bridge-server imports agent-store as library
- [x] Mounts agent-store handlers via `RegisterHandlers()`
- [x] Routes: `/agents/*`, `/memories/*`, `/configs`, `/reconcile`
- [x] Removed standalone `cmd/server/` from agent-store

### Post-Phase 4: Session Discovery & Log-Store Integration (2026-04-13)

- [x] Created `log-store` service for durable event logging (JSONL storage, REST API)
- [x] llm-bridge-server pushes events to log-store, proxies `/messages` and `/history`
- [x] Removed local history store fallback — log-store is the single source for message history
- [x] Auto-discover harness native sessions on startup (Claude Code `~/.claude/*`, Codex `~/.codex/*`)
- [x] Import discovered session conversation history to log-store
- [x] Assign discovered sessions to the local instance that ran discovery
- [x] Use prompt as display_name for discovered sessions

### Post-Phase 4: Harness Implementations (2026-04-13)

- [x] `llm-bridge-claudecode` — full implementation: subprocess via `--input-format stream-json`, session discovery (`-discover`), history import (`-import-history`), auto-approve toggle, work dir support
- [x] `llm-bridge-jig` — profile manager bridge with subprocess management, YAML profile inheritance

### Remaining Phases

```
Phase 5 (wire provider bridges into harnesses)
     Not yet started — verified 2026-05-08: no harness imports any
     llm-bridge-{anthropic,openai,google,openrouter} package yet
     (claudecode, jig, codex, hermes go.mod checked).
Phase 6 (fold auth-store, usage-store into llm-bridge-server)
     Status reversed by host ecosystem decisions: auth-store became a
     standalone service on :8303 (canonical credential vault, replaces
     apiauth) and usage-store-server is running on :8185. The "fold in"
     plan no longer matches the deployed shape — see "Service status
     reality check" below for editorial decisions still pending.
```

### Service status reality check (2026-05-08, doc audit)

The following claims in this document are stale relative to the running host:

1. **agent-store cmd/server still exists and runs.** `~/repos/agent-store/cmd/server/` is present and the binary listens on `:8300`. Phase 2 mounted agent-store's handlers in llm-bridge-server (`agentstore.RegisterHandlersWithHooks` + `memorystore.RegisterHandlers`) but did not retire the standalone server, contrary to the "Removed standalone `cmd/server/` from agent-store" line above.
2. **auth-store is a service, not a library.** ARCHITECTURE.md describes auth-store as a Go library to be folded into llm-bridge-server. Reality: auth-store is the canonical credential vault on `:8303` with audit log, OAuth refresh, and lease enforcement. llm-bridge-server's `go.mod` has no `auth-store` import — credential routes (`/credentials`, `/instances/{id}/credentials`) are handled directly by llm-bridge-server against its internal `credential_slots` store (not auth-store).
3. **usage-store is a service.** Standalone `usage-store-server` runs on `:8185`. llm-bridge-server's go.mod has no `usage-store` import. Token usage tracking is not currently wired through llm-bridge-server.
4. **Harness "Scaffold" labels above were stale (2026-04-12-era).** Re-measured 2026-08-21: **12 of the 18 harness bridges are Implemented** (codex, claudecode, jig, openclaw, inber, hermes, aider, nanoclaw, cline, kilocode, forgecode, copilotcli); **6 remain scaffolds** (gemini, goose, autohand, dexto, commander, roocode), each a 34-line `main.go` scoring 1/22 on the stored conformance matrix. See the parent session-chain port todo `3722e9d6` in the noteboard for the per-harness ship log.

   ⚠️ **This entry is the reason dating a claim is not the same as keeping it true.** It was stamped *"Updated 2026-05-08"* and both of its numbers were correct that day. `llm-bridge-copilotcli` was created 2026-05-11 — three days later — and appeared in neither list, so the total silently became 18 while the entry still summed to 17. The scaffolds' `main.go` grew from 24 lines to 34 on 2026-05-10, two days later. A date protects the author, not the reader: a reader meets this stamp attached to *a correction of stale claims* and reads it as current. If a number here has to stay, re-measure it; if it does not, drop it.
5. **`llm-bridge-runner` is missing from the diagram.** A separate remote-machine daemon (`~/repos/llm-bridge-runner`, ~2108 LOC) provides outbound-WS-based harness spawning for machines behind NAT, complementary to the SSH transport described under "Harness Instance Model". Not currently in the Repository Map or import graph.

These items need an editorial call before doc-rewrite: are auth-store/agent-store/usage-store still on a fold-in path (in which case the migration is in progress, not phase-2/phase-1-COMPLETE) or has the host architecture deliberately diverged toward standalone services (in which case the Phase 1 / Phase 2 sections need rewriting, not just status-flipping).



## Ports (post-migration)

The "single port 8160" target was the original migration goal but the host ecosystem grew separate canonical services for credentials and usage. Current reality:

| Port | Service | Notes |
|------|---------|-------|
| 8160 | llm-bridge-server | HTTP gateway — sessions, harness lifecycle, mounts model-store/agent-store/memory-store handlers as libraries |
| 8300 | agent-store | Standalone service still running (`cmd/server/`); the "Phase 2 fold-in" mounted handlers in llm-bridge-server but did not retire the standalone server |
| 8303 | auth-store | Standalone service — credential vault, OAuth refresh, lease enforcement (replaced `apiauth` + the auth-as-store half of `aiauth`). **Not** library-folded into llm-bridge-server |
| 8185 | usage-store | Standalone service (`usage-store-server` cmd). **Not** library-folded into llm-bridge-server |
| log-store | Service | Durable event log; pushed to by llm-bridge-server, proxied for `/messages` + `/history` |

## Harness Instance Model

A **harness type** (e.g., `claudecode`, `codex`, `aider`) is a template. A **harness instance** is a running deployment of that type on a specific machine, with its own credential bindings and concurrency limits.

### Why Instances Matter

- You might run 3 Claude Code instances: one on your laptop, one on a dev server, one on a CI machine
- Each instance has different credentials available (laptop has your personal API key, CI has team key)
- Each instance has different concurrency limits (laptop: 1 session, dev server: 5 sessions)
- SSH transport lets llm-bridge-server orchestrate remote instances without running the server there

### Instance Registry

```
harness_instances
├── id (uuid)
├── harness_type (claudecode, codex, aider, ...)
├── name (human label: "laptop-cc", "ci-runner-1")
├── host (localhost, dev.internal, ci-01.example.com)
├── transport (local, ssh)
├── ssh_user (optional)
├── ssh_key_path (optional)
├── working_dir (where sessions run)
├── max_concurrent_sessions (1, 5, 10, ...)
├── enabled (bool)
├── created_at
└── updated_at
```

### Credential Binding

Each harness instance declares which credentials it can use, in priority order (primary + fallbacks).

```
instance_credentials
├── instance_id (FK harness_instances)
├── credential_id (FK auth-store credentials)
├── priority (0 = primary, 1+ = fallbacks)
├── max_concurrent (how many sessions can use this cred simultaneously)
└── enabled (bool)
```

**Example**: A Claude Code instance on `dev.internal` might have:
- Priority 0: Team Anthropic API key (max 3 concurrent)
- Priority 1: Personal API key (max 1 concurrent, fallback if team key exhausted)
- Priority 2: OpenRouter key (max 5 concurrent, fallback for rate limits)

### Concurrent Credential Awareness

When starting a session, llm-bridge-server:
1. Picks the harness instance (based on session request or round-robin)
2. Checks credential bindings for that instance
3. Finds the highest-priority credential with available concurrency slots
4. Reserves a slot, injects the credential into the harness process
5. Releases the slot when the session ends

This enables:
- Load balancing across API keys
- Graceful degradation when a key hits rate limits
- Different credential pools for different machines

### SSH Transport

For remote instances, llm-bridge-server uses SSH to:
1. Spawn the harness binary on the remote machine
2. Pipe stdin/stdout over the SSH connection
3. Handle the same JSON protocol as local processes

```go
// Local: exec.Command("llm-bridge-claudecode")
// SSH:   exec.Command("ssh", "-i", keyPath, "user@host", "llm-bridge-claudecode")
```

The harness binary must be installed on the remote machine. llm-bridge-server just orchestrates it.

### API Additions

```
GET  /instances                     # list all harness instances
POST /instances                     # register a new instance
GET  /instances/{id}                # get instance details
PUT  /instances/{id}                # update instance config
DELETE /instances/{id}              # remove instance

GET  /instances/{id}/credentials    # list bound credentials
POST /instances/{id}/credentials    # bind a credential
DELETE /instances/{id}/credentials/{cred_id}  # unbind

GET  /instances/{id}/sessions       # list active sessions on instance
POST /sessions                      # now accepts instance_id param
```

## Session Identity & Resumption

A logical conversation in llm-bridge has **one** stable identifier from end to end: `bridge_session_id`. The harness underneath may use entirely different identifiers that rotate over the conversation's lifetime (Codex mints a new `thread_id` on every `thread/resume`; Claude Code keeps a session_id but starts a new file on `--fork`). Bridge-server never sees those rotations — the harness bridge absorbs them.

### Identifiers

| Field | Meaning | Lifecycle |
|---|---|---|
| `harness` | Harness type — `"codex"`, `"claudecode"`, etc. | Static |
| `instance_id` | A specific running harness instance (host + creds) | Per-deployment |
| `bridge_session_id` | Bridge-server's stable session id | Created once, never changes |
| `harness_session_id` | Harness-internal id (Codex thread_id, Claude session_id, ...) | Mutable; bridge owns rotation |

**Every event a bridge emits to bridge-server carries both `bridge_session_id` and `harness_session_id`.** `bridge_session_id` is the routing/storage key. `harness_session_id` is informational — diagnostic, used to look at the harness's native rollout/log file. Bridge-server stores the latest reported `harness_session_id` on the session row but never makes routing decisions on it.

### Bridge responsibilities

Each harness bridge owns:

1. **A local persistent store** (default `~/.local/share/llm-bridge-<harness>/state.db`) tracking the chain of `harness_session_id`s for every `bridge_session_id` it has touched.
2. **A WAL** for atomic chain mutations. Before calling any harness operation that mints a new `harness_session_id` (start, resume, fork), write a `pending` WAL row. After the harness returns, write the `committed` row with the new id. Crash between the two = orphan recoverable on next discover.
3. **Translation discipline.** The bridge demuxes harness-side events by `harness_session_id`, looks up `bridge_session_id`, and forwards tagged with the latter. The bridge's translation layer is the *only* place that knows about both ids.

### State schema (per-bridge)

```
sessions:
  bridge_session_id    PK
  current_harness_id   TEXT       -- latest harness_session_id; rotates
  created_at, updated_at

rollouts:
  harness_session_id   PK         -- UNIQUE; demux key for live events
  bridge_session_id    FK
  rollout_path         TEXT       -- harness-native log/rollout file
  sequence             INT        -- 0 = original, 1..N = resumes/forks
  parent_harness_id    TEXT NULL  -- predecessor in the chain
  kind                 TEXT       -- 'start' | 'resume' | 'fork'
  created_at

wal:
  id                   PK
  bridge_session_id    TEXT
  intent               TEXT       -- 'start' | 'resume' | 'fork'
  parent_harness_id    TEXT NULL
  new_harness_id       TEXT NULL  -- filled on commit
  rollout_path         TEXT NULL
  status               TEXT       -- 'pending' | 'committed' | 'orphaned'
  created_at, committed_at
```

### Resume flow

```
bridge-server:  "attach to bridge_session_id X"
bridge:         load X from local store → parent = sessions.current_harness_id
                WAL: insert {intent:'resume', parent, status:'pending'}    fsync
                harness call: ThreadResume(parent) → returns new id Z
                WAL: update {new_harness_id:Z, status:'committed'}         fsync
                rollouts: insert (Z, X, sequence+1, parent, 'resume', path)
                sessions: update current_harness_id = Z
                trans: SetSessionID(X)   ← always X, never Z
                proceed with turn — events arrive tagged Z, forwarded as X
```

If the bridge crashes between `pending` and `committed`, on restart it scans the WAL: any `pending` row is reconciled against rollouts written since `wal.created_at` matching `(originator, cwd)`. One match → claim it. Zero or many → mark `orphaned` and surface to the operator.

### Discover

`<harness-bridge> -discover` reads the bridge's `state.db` and emits one `StoredSession` per `bridge_session_id`, with the rollout chain attached. Cold rollouts on disk that aren't in `state.db` (created before this contract was adopted, or by direct CLI use outside the bridge) are imported as single-rollout sessions on first run. Going forward, every rollout the bridge produces is in the chain.

### Process model

**One bridge process per `(harness, instance_id)`, sessions multiplexed.** Bridge-server warm-pools one bridge subprocess per harness instance and routes every session for that instance through it. The bridge demuxes by `bridge_session_id` on requests, by `harness_session_id` (looked up via `rollouts` table) on harness-side events.

- `manager.processes` is keyed by `instance_id`, not `bridge_session_id`.
- `manager.routing` (`bridge_session_id → instance_id`) tells the server which process to write to for a given session.
- Lazy spawn on first session, kept warm indefinitely. Process termination is a separate concern from session termination.
- A misrouted message (unknown `bridge_session_id` arriving at a bridge) must be rejected loudly, not silently dispatched. A misrouted event (unknown `bridge_session_id` arriving at the server) must be logged and dropped, not auto-create a session.

### Wire protocol

The bridge ↔ bridge-server pipe carries two ids and only two ids. Conflating them silently is the bug class this section exists to prevent.

**Requests (server → bridge stdin)** carry `bridge_session_id` and nothing else id-shaped. The bridge looks up its own `harness_session_id` chain locally; the server never tells the bridge which harness id to use:

```go
type StartParams struct {
    BridgeSessionID string  // routing key — server-side identifier, stable
    Resume          bool    // bridge consults state.db chain for the latest harness id
    Fork            string  // parent BRIDGE_SESSION_ID (not a harness id)
    DisplayName, AgentID, CredentialID, Prompt, WorkDir string
}

type MessageParams struct {
    BridgeSessionID string  // required — multiplexing demands explicit routing
    Content         string
}

type InterruptParams struct{ BridgeSessionID string }
// ... and so on for compact, set_model, set_permission_mode, etc.
```

**Events (bridge → server stdout)** carry both ids. The first is the routing/storage key; the second is informational and stored on the session row for diagnostics:

```go
type Event struct {
    BridgeSessionID  string  // required, the routing/storage key
    HarnessSessionID string  // informational, the harness-native id at emission time
    // ...
}
```

**Id generation responsibilities:**

| Id | Generated by | First appears | Stable across resumes |
|---|---|---|---|
| `bridge_session_id` | bridge-server, in `handleCreateSession` | `sess.BridgeID` | **Yes** |
| `instance_id` | bridge-server, picked from `harness_instances` | `manager.Start()` | Yes |
| `harness_session_id` | the harness itself (e.g. Codex `ThreadStart`/`Resume` returns it) | bridge subprocess | **No** — rotates |

**Invariants the bridge must hold:**

1. The bridge never substitutes `bridge_session_id` on its way out. What bridge-server passed in on a request is what comes back on every event.
2. The bridge never receives a `harness_session_id` from bridge-server. Server-to-bridge requests are pure `bridge_session_id` plus action params.
3. The bridge writes `harness_session_id` on every event, sourced from its own state.db. Bridge-server reads it as an opaque last-known value.
4. A request without a `bridge_session_id` (or with one the bridge doesn't recognize) is an error. No silent defaulting.

These rules collapse the historical `SessionID` ambiguity (where the same field meant `bridge_id` on `start` and `harness_id` on resume, depending on context) into two distinct, named fields that never trade places.

### What this guarantees

- Resumes don't fragment a conversation across multiple bridge-server sessions.
- Forks produce new `bridge_session_id`s with `parent_id` pointing at the parent's `bridge_session_id` (not a harness-internal id).
- Concurrent threads in one bridge process disambiguate by `rollouts.harness_session_id` lookup.
- After a bridge crash, the chain is recoverable from `state.db` + WAL — no in-memory-only state.
- Bridge-server has no awareness of harness id rotations; it stores `harness_session_id` as an opaque last-known value.

### Contract for new harness bridges

Any bridge added to the ecosystem must:

1. Accept `bridge_session_id` from bridge-server at spawn and treat it as the single routing key.
2. Maintain a local `state.db` + WAL per the schema above.
3. Tag every outgoing event with both `bridge_session_id` and `harness_session_id`.
4. Implement `-discover` against `state.db`, not against the harness's native filesystem.
5. Never mutate `bridge_session_id` after creation. If the harness mints a new internal id, append to the chain — don't tell bridge-server it's a new session.

## Canonical Types (`msg/`)

The `msg/` package is the **single source of truth** for all shared types across the llm-bridge ecosystem. Every type that crosses a service boundary — API requests, API responses, events, sessions, instances, credentials, preferences — is defined here.

### What lives in `msg/`

| File | Types | Description |
|------|-------|-------------|
| `message.go` | `Conversation`, `Message` | LLM API wire types |
| `content.go` | `ContentBlock`, `TextBlock`, `ImageBlock`, ... | Content block discriminated union |
| `event.go` | `Event`, `ResultEvent`, `ToolSummary`, ... | Harness event types |
| `session.go` | `Session`, `SessionTask`, `StoredSession` | Agent-level session state |
| `instance.go` | `Instance`, `InstanceCredential`, `CredentialSlot`, `InstanceStatus`, `CredentialStatus` | Harness instance types |
| `provider.go` | `Provider`, `Role`, `Harness`, `EventType`, `SessionState`, ... | Enums and constants |
| `config.go` | `GenerationConfig`, `AnthropicConfig`, `OpenAIConfig`, ... | Provider config types |
| `response.go` | `CompletionResponse`, `SafetyRating`, ... | LLM API response types |
| `stream.go` | `StreamEvent`, `BlockDelta`, `MessageDelta`, ... | Streaming event types |
| `tool.go` | `ToolDef`, `ToolChoice` | Tool definition types |
| `usage.go` | `TokenUsage`, `Cost` | Usage tracking types |
| `server.go` | `ManagedSession`, `HarnessInfo`, `Credential`, `BridgePrefs`, `MaterializedMessage`, request types, health types | Server API surface types |
| `validate.go` | `ValidationError`, `ValidationFailure` | Validation types |

### Adding new types

**If a type is used in API responses, API requests, events, or SSE streams — it belongs in `msg/`.** Do not define API contract types locally in `llm-bridge-server`, `log-store`, or any other service. Instead:

1. Define the Go struct in the appropriate `msg/*.go` file
2. Run `./generate-ts.sh` and `./generate-py.sh` to regenerate TypeScript and Python
3. Commit Go, TypeScript, and Python changes together
4. Import from `msg` in Go, `@kayushkin/llm-bridge-types` in TypeScript, `llm_bridge_types` in Python

**Types that do NOT belong in `msg/`:** UI-only state (React hooks, component props), internal store implementation details, transport-layer types that never leave a process.

### Consumers

| Consumer | How it imports |
|----------|---------------|
| **llm-bridge-server** | Go: `import "github.com/kayushkin/llm-bridge/msg"` — uses type aliases for backward compat (e.g. `type Session = msg.ManagedSession`) |
| **log-store** | Go: same as above |
| **bridge-ui** | TS: `import type { ... } from '@kayushkin/llm-bridge-types'` — re-exports with aliases (e.g. `BridgeSession` = `ManagedSession`) |
| **llmux** | TS: imports canonical types directly from `@kayushkin/llm-bridge-types`, or indirectly via `@kayushkin/bridge-ui` |

## TypeScript Types (`ts/`)

The `ts/` directory contains **auto-generated TypeScript types** derived from the Go types in `msg/`. Published as `@kayushkin/llm-bridge-types`. These files must never be edited by hand — any manual changes will be overwritten.

**Source of truth:** Go structs and constants in `msg/*.go`.

**Generation:** Run `./generate-ts.sh` to regenerate. This uses [tygo](https://github.com/gzuidhof/tygo) to parse the Go source and emit TypeScript interfaces and constants. The output is stamped with the source commit SHA.

**Drift check:** `deploy.sh` regenerates the types and diffs against the committed version. If they don't match, the deploy fails. This ensures the committed TypeScript always reflects the current Go types.

**Workflow:**
1. Edit Go types in `msg/`
2. Run `./generate-ts.sh`
3. Commit both Go and TypeScript changes together
4. Downstream consumers (`bridge-ui`, `llmux`) import from `@kayushkin/llm-bridge-types`

**Do not:**
- Edit files in `ts/` directly
- Add hand-written types to `ts/` — frontend-only types belong in the consuming package (e.g. `bridge-ui`)
- Define API contract types locally in consuming services — add them to `msg/` instead

## Python Types (`py/`)

The `py/` directory contains **auto-generated Python dataclasses** derived from the Go types in `msg/`. Packaged as `llm-bridge-types` (importable as `llm_bridge_types`). These files must never be edited by hand — any manual changes will be overwritten.

**Source of truth:** Go structs and constants in `msg/*.go`.

**Generation:** Run `./generate-py.sh` to regenerate. This runs `cmd/genpy`, a Go program that uses `go/ast` to parse the Go source and emit Python dataclasses and constants. The output is stamped with the source commit SHA.

**Drift check:** `deploy.sh` regenerates the types and diffs against the committed version. If they don't match, the deploy fails. This ensures the committed Python always reflects the current Go types.

**Workflow:**
1. Edit Go types in `msg/`
2. Run `./generate-py.sh`
3. Commit both Go and Python changes together
4. Downstream consumers import from `llm_bridge_types`

**Do not:**
- Edit files in `py/llm_bridge_types/msg.py` directly
- Add hand-written types to `py/` — consumer-specific types belong in the consuming package
- Define API contract types locally in consuming services — add them to `msg/` instead

## Design Principles

1. **Libraries over servers.** Stores are Go packages you import. One server composes them.
2. **One namespace.** Everything is `llm-bridge-*`. Provider bridges, harness bridges, the server.
3. **Pick what you need.** Want just the types? Import `llm-bridge`. Want credentials? Import `auth-store`. Want the full gateway? Run `llm-bridge-server`.
4. **No fallback data.** Stores are sources of truth. If a field is empty, it's empty -- don't fabricate defaults that downstream services will treat as real.
5. **Provider bridges are stateless.** They convert formats. They don't hold connections or state.
6. **Harness bridges are processes.** They manage subprocesses and emit events. llm-bridge-server orchestrates them.
7. **Instances, not singletons.** A harness type is a template. Instances are deployments with their own credentials and limits.
