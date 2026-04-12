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
   | anthropic      | | openai         | | gemini         |
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
| **usage-store** | Go library | Token usage tracking per agent/orchestrator/model/day. Cost calculation from model-store pricing. Aggregation queries. |
| **aiauth** | Go library | OAuth2 flow runner (browser redirect, token exchange, refresh). Used by auth-store for token lifecycle. |

### Provider Bridges

Convert between canonical `msg` types and provider-specific wire formats. Stateless libraries.

| Repo | Type | Status |
|------|------|--------|
| **llm-bridge-anthropic** | Go library | Has implementation (from msgbridge-anthropic) |
| **llm-bridge-openai** | Go library | Has implementation (from msgbridge-openai) |
| **llm-bridge-gemini** | Go library | Scaffold |
| **llm-bridge-openrouter** | Go library | Scaffold |

### Harness Bridges

Manage agent harness subprocesses. Spawned by llm-bridge-server, communicate via stdin/stdout JSON protocol, emit canonical `msg.Event`.

| Repo | Type | Status |
|------|------|--------|
| **llm-bridge-claudecode** | Go binary | Scaffold -- wraps Claude Code CLI |
| **llm-bridge-codex** | Go binary | Scaffold -- wraps Codex CLI |
| **llm-bridge-openclaw** | Go binary | Scaffold -- WebSocket client |
| **llm-bridge-inber** | Go binary | Scaffold |
| **llm-bridge-hermes** | Go binary | Scaffold |
| **llm-bridge-aider** | Go binary | Scaffold -- wraps Aider CLI |
| **llm-bridge-goose** | Go binary | Scaffold -- wraps Goose CLI/API |
| **llm-bridge-autohand** | Go binary | Scaffold -- wraps Autohand Code CLI (ACP stdio) |
| **llm-bridge-jig** | Go binary | Scaffold -- wraps Jig CLI |
| **llm-bridge-dexto** | Go binary | Scaffold -- REST+SSE client |
| **llm-bridge-commander** | Go binary | Scaffold -- subprocess orchestrator |
| **llm-bridge-nanoclaw** | Go binary | Scaffold -- container subprocess |
| **llm-bridge-cline** | Go binary | Scaffold -- wraps Cline CLI |
| **llm-bridge-roocode** | Go binary | Scaffold -- wraps Roo Code CLI |

### Service

| Repo | Type | Description |
|------|------|-------------|
| **llm-bridge-server** | Go service | The gateway. HTTP API for sessions, event streaming (SSE), harness lifecycle management. Imports all store libraries. Exposes agent, model, credential, and usage data via API. Single process to run. |
| **llm-bridge-adapter** | Go service | NATS bus adapter for integrating llm-bridge with the inber messaging ecosystem. |

### Retired / To Delete

| Repo | Reason |
|------|--------|
| **llm-msg-spec** | Empty shell, never populated. Types live in `llm-bridge/msg/`. Delete. |
| **msgbridge-anthropic** | Rename to `llm-bridge-anthropic`, migrate implementation. Delete original. |
| **msgbridge-openai** | Rename to `llm-bridge-openai`, migrate implementation. Delete original. |
| **msgbridge-gemini** | Scaffold only. Recreate as `llm-bridge-gemini`. Delete original. |
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
  |--- llm-bridge-gemini      (imports llm-bridge/msg)
  |--- llm-bridge-openrouter  (imports llm-bridge/msg)
  |
  |--- llm-bridge-claudecode  (imports llm-bridge/msg, llm-bridge/bridge)
  |--- llm-bridge-codex       (imports llm-bridge/msg, llm-bridge/bridge)
  |--- llm-bridge-openclaw    (imports llm-bridge/msg, llm-bridge/bridge)
  |--- llm-bridge-inber       (imports llm-bridge/msg, llm-bridge/bridge)
  |--- llm-bridge-hermes      (imports llm-bridge/msg, llm-bridge/bridge)
  |
  +--- llm-bridge-server      (imports all stores + harness bridges)
       |-- runtime store: sessions, events, credential_slots
       |-- imports: agent-store, memory-store, harness-store (model-store, auth-store planned)
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
- `llm-bridge-gemini` -- new repo, scaffold from msgbridge-gemini TODOs
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
- [x] Created scaffold `llm-bridge-gemini`
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

### Remaining Phases

```
Phase 5 (wire provider bridges into harnesses)
     Ongoing as harnesses mature.
Phase 6 (fold model-store, auth-store, usage-store into llm-bridge-server)
     Additional stores to mount when ready.
```

## Ports (post-migration)

| Port | Service | Notes |
|------|---------|-------|
| 8160 | llm-bridge-server | The only server. Replaces model-store :8150 and agent-store :8300 |

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

## Design Principles

1. **Libraries over servers.** Stores are Go packages you import. One server composes them.
2. **One namespace.** Everything is `llm-bridge-*`. Provider bridges, harness bridges, the server.
3. **Pick what you need.** Want just the types? Import `llm-bridge`. Want credentials? Import `auth-store`. Want the full gateway? Run `llm-bridge-server`.
4. **No fallback data.** Stores are sources of truth. If a field is empty, it's empty -- don't fabricate defaults that downstream services will treat as real.
5. **Provider bridges are stateless.** They convert formats. They don't hold connections or state.
6. **Harness bridges are processes.** They manage subprocesses and emit events. llm-bridge-server orchestrates them.
7. **Instances, not singletons.** A harness type is a template. Instances are deployments with their own credentials and limits.
