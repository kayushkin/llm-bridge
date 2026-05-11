# Migration — Identity & Layer Consolidation

Status: **proposed**, not started. This doc describes a multi-phase breaking change to the session, turn, message, and event identity model, plus the storage layout that backs it. It is the single source of truth for the planned shape; consumer repos can read it to anticipate work.

The migration ships in three phases. Each phase is independently shippable after Phase I lands. Within a phase, sub-steps are ordered.

## Why

The current model accumulated parallel ID namespaces and field overloads that hide the real architecture:

1. **`bridge_id` and `bus_session_id` are 1:1 aliases.** `bus_session_id` is minted by the caller (autoworker / kanban-dispatcher / scheduler) and stored alongside `bridge_id` in `~/.llm-bridge-adapter/sessions.json` as a 1:1 map. It is not aggregating multiple bridge sessions; it is a synonym. The mapping table exists to give callers a synchronous handle before `POST /sessions` returns — solvable by letting the caller mint the canonical ID directly.

2. **`client_id` is overloaded.** `msg/server.go:37-39` documents it as "the frontend's correlation key (fe_*)". In practice, `scheduler/cmd/autoworker/main.go:377` passes the literal string `"autoworker"`, `scheduler/cmd/kanban-dispatcher/main.go:540` passes `"kanban-dispatcher"`, and `scheduler/internal/executor/agent.go:112` passes `"scheduler"`. The same field carries a per-session handle in one caller and a service tag in another. Both jobs already have better homes (`source` for service identity, `session_id` for the local handle).

3. **`source` already does the "who originated this" job.** `msg/server.go:55` documents it as "origin tag carried from the creator (e.g. 'scheduler', 'autoworker'); empty = interactive". `sessions` table has an index on `source`. A `source_folders` table maps source values to UI folders. The field is first-class today; the workers happen to double-stamp the same value into both `client_id` and `source`.

4. **Events are stored twice.** `llm-bridge-server/internal/harness/manager.go:503` pushes every event to log-store with the comment "Push to log-store (durable source of truth)" — but bridge-server also writes its own `events` table. log-store is the documented SSOT; bridge-server's table is the redundant copy.

5. **memory-store has dead session tables.** `memory-store/sessions.go` defines `sessions` and `session_tags` tables and a `SaveSession` function with no live callers. Inber-era artifacts; everything in there is either operational state (belongs in bridge-server's sessions table) or derivable aggregate (computable from log-store events).

6. **Harness-internal IDs leak into bridge-server logic.** `harness_session_id` and `harness_message_id` are stamped on bridge-server's session row and events table, indexed, and used for resume reconciliation, bubble-split detection, and tool-use rebinding (`manager.go:251-278`). All of this is harness-specific behavior in what should be a harness-agnostic core.

7. **Three identifiers per turn (turn_id, message_id, client_request_id) overlap in scope.** `client_request_id` covers the user→response cycle; `turn_id` covers the same span; `message_id` covers each bubble within. With `turn_id` redefined as per-speaker-contribution, idempotency-via-message_id and turn-ordering for cross-turn linkage make `client_request_id` redundant.

8. **`actor` is derived from `kind` is derived from `ev.type`.** Three layers of representation for the same fact, and the derivation has bugs — convenience events emitted by the bridge surface as `bc-row-assistant` despite being metadata.

## Data preservation rule (applies to all phases)

Every phase of this migration preserves the canonical `msg.Event` content end-to-end. `Event.Overflow` continues to carry unknown fields verbatim. Adapter → bridge → log-store remains a transparent pipeline. We are restructuring *where things are stored* and *which layer owns which logic*, not changing what data flows on the wire.

The only data that moves *off* the wire is the harness-internal IDs (`harness_session_id`, `harness_message_id`), which become adapter-private. They remain recoverable via lazy-loaded diagnostics endpoints (Phase II.C, III.B). Pre-canonical raw harness frames before adapter normalization are out of scope (those are stripped today and continue to be stripped — separate concern if needed later).

## Phase I — Session Identity Consolidation

Renames and consolidates the session-level identifier model. No structural changes to events, messages, or storage layout.

### Final session identifier shape

| Field | Purpose | Origin | Replaces |
|---|---|---|---|
| `session_id` | Stable session PK | Caller mints when it originates the session, or server mints on harness-discovery | `bridge_id` (server side) + `bus_session_id` (caller side) |
| `harness_session_id` | Harness-internal thread; rotates on resume/fork | Harness | (unchanged in Phase I; moves to adapter in Phase II.C) |
| `source` | Specific originating service | Caller (declared) | (unchanged — already a first-class field) |
| `session_type` | Category of session (interactive / autonomous / system) | Caller (declared) | (new; absorbs the categorical half of `client_id`) |
| `instance_id` | Which harness instance hosts this session | Resolved at create from harness-store | (unchanged) |

`client_id` is removed. Its two jobs are now `session_id` (local handle) plus `source` + `session_type` (service identity and category).

### `session_type` values

```
interactive    Human-in-the-loop. Frontend chat, CLI, mobile.
autonomous     Fire-and-forget agent runs, no human watching live.
               Autoworker, scheduler-fired tasks, kanban dispatcher.
system         Bridge-internal subsystems and meta-agents.
               Renamer, subagents, kanban classifier, permission_prompt.
```

Bridge-side behavior that depends on category (e.g. completion notifications, SSE keepalive policy) reads `session_type`. Behavior that depends on specific service identity reads `source`. The bridge does not maintain an internal source→type table; callers declare both.

`session_type` is **required** on Create — no default, no inference. A session without a declared type is rejected at the boundary. Defaults look friendly but mask misconfiguration (a worker that forgets to declare `autonomous` silently shows up as `interactive`); requiring it forces every caller to make the choice once and stamp it explicitly.

### `session_id` minting rules

- If the caller originates the session (frontend, autoworker, dispatcher, scheduler), the caller mints `session_id`. Recommended format: ULID. Caller passes it on `POST /sessions`.
- If the bridge discovers a session that no caller created (harness-watch finds an existing harness-side session and registers it), the bridge mints `session_id` at discovery time.
- Bridge validates uniqueness on Create. Collision returns 409.
- The frontend's existing `fe_${ts}_${rand}` pattern becomes a valid `session_id` directly.
- The autoworker's `autoworker-{provider}-{nanos}` pattern becomes a valid `session_id` directly. (No more separate `bridge_id` returned by the bridge.)
- The field name is `session_id` everywhere — wire format, SQL column, struct field. The previous concern about ambiguity with the rich `Session` agent-state entity is resolved by deleting that entity (Phase II.B).

### Sub-steps

1. **Add `session_type` to canonical types** (`llm-bridge/msg/server.go`, plus generated `ts/` and `py/` mirrors). Required on Create — no default. Sub-step 1 ships the field but does not yet enforce; sub-step 2 turns enforcement on once all callers populate it.
2. **All callers start declaring `session_type`** (scheduler workers, frontend, inber). Once every caller is updated, the bridge starts rejecting CreateSession requests without a `session_type`.
3. **Add `session_id` column to `sessions` table aliased to `bridge_id`.** Both columns populated, both queryable. Dual-write window so the adapter and consumers can switch without coordination.
4. **Update llm-bridge-adapter to mint `session_id` directly when it has a `bus_session_id` from the caller**, eliminating the mapping. `~/.llm-bridge-adapter/sessions.json` becomes a discovery cache (or is removed entirely).
5. **Switch consumers (bridge-ui, scheduler, inber) to use `session_id`** field name on read.
6. **Drop `client_id` column.** All callers should be passing `source` + `session_type` by this point. Pre-drop, audit the DB to confirm no row has `client_id` set to a value not derivable from `source`.
7. **Drop `bridge_id` column** (or rename to `session_id` if no readers remain on the old name). HTTP path parameters change from `/sessions/{bridge_id}/...` to `/sessions/{session_id}/...`. Routes can keep both during the transition by accepting either name.

Each sub-step is backward-compatible. The breaking moment is sub-step 7; everything before is additive or dual-named.

### Cleanups bundled with Phase I

Two types live in the wrong place today; fix them as part of Phase I:

- **`internal/permclient/client.go:53`** has a request struct with `BridgeID` defined locally inside `llm-bridge-server`. That's a wire-format type (the bridge calls out to the permission prehook over HTTP); it belongs in `llm-bridge/msg/`. Move it.
- **`bridgeSession` in `llm-bridge-adapter/main.go:149`** is a thin local wrapper around the `POST /sessions` response. Should just be `msg.ManagedSession`. Remove the wrapper.

Also: today's pre-existing inconsistency where `Event.BridgeSessionID` (json `bridge_session_id`) and `ManagedSession.BridgeID` (json `bridge_id`) name the same concept differently — both collapse to `session_id` in this rename.

## Phase II — Layer Separation

Restructures storage to match the layer-responsibility model:

| Layer | Owns | Universal handle | Layer-private metadata |
|---|---|---|---|
| Harness subprocess | Its own runtime state | — | Harness's native ids, internal threads |
| Harness adapter | session_id ↔ harness-native-id mapping; resume/fork dispatch | session_id | `harness_session_id`, `harness_message_id`, harness-specific config |
| Bridge-server | Session lifecycle, routing, instance binding | session_id | `pid`, `instance_id`, `harness_config`, in-flight turn state |
| Log-store | Event log, materialized history, aggregates | session_id | event row IDs, raw event JSON |
| Memory-store | Memories | session_id (FK only) | memory bodies, embeddings, tags |
| Agent-store | Agent identity, model preferences | session_id (FK only) | `agent_id`, `agent_name`, `model` |

Each layer exposes its own diagnostics endpoint. The frontend's session detail panel fans out to these endpoints lazily; day-to-day rendering uses only core fields.

### II.A — Events SSOT in log-store

Bridge-server already pushes every event to log-store at `manager.go:503` with the comment "durable source of truth." Bridge-server's local `events` table is the redundant copy. Eliminate it.

**Sequencing note**: do **III.B before II.A**. The read-path audit (see `MIGRATION-session-identity-readpath-audit.md` if extracted, or the discussion thread) found that three of the most painful queries against bridge-server's events table — `RecoverInFlightTurn` line 987, `HarnessToBridgeMap` line 852, `ToolUseToMessageMap` line 886 — depend on `message_id` and `harness_message_id` as separate indexed columns. These three queries **disappear entirely** after III.B (the harness-side reconciliation moves to the adapter's SQLite). Doing III.B first means log-store doesn't need to grow JSON-extract logic or extra columns to serve these. The remaining gaps after III.B are small.

**Log-store API extensions needed (after III.B):**

1. `GET /api/v1/sessions/{id}/turn-state` — returns `{last_user_message_event_id, last_terminator_event_id, in_flight: bool}`. Covers `RecoverInFlightTurn` (lines 945, 959), `PendingTurnMessage` (lines 439, 450), and the "find last user_message" half of `ListCurrentTurnEventsWithIDs` (line 758). **Shipped 2026-05-10 (log-store 43e7b7a).**
2. `?types=user_message,result,...` filter on `/events` and `/history`. Covers `RecentTurnTexts` (line 673, renamer transcript), `ListToolCallInputs` (line 824, git path discovery), and the events-after-last-user-message half of `ListCurrentTurnEventsWithIDs` (line 763). **Shipped 2026-05-10 (log-store 43e7b7a).**
3. Turn count + last activity — already in II.B's sessions projection table (`turn_count`, `last_active`). No separate endpoint needed; bridge-server reads from the projection. Covers `CountUserMessages` (line 587) and `LastActivityAt` (line 530).

That's one new endpoint, one filter add to two existing endpoints, and zero new endpoints if II.B ships alongside or before II.A. The client (`log-store/client/client.go`) gained `GetTurnState` and `ListEvents(after, types)` so bridge-server can call them once the cutover begins.

**Cutover plan (safety-first; no event loss permitted):**

1. **Audit phase**: confirm the existing push isn't lossy. Bridge-server logs failures via `log.Printf` but doesn't retry. Add reconciliation: for every event written to bridge-server's `events` table, verify it landed in log-store with the same shape. Run a one-shot diff script over a recent session window. **Done 2026-05-11.** Found `BroadcastEvent` (used by bridge-originated hook / session_state / display_name / user_message events) wrote only to bridge-server's local table, never to log-store. ~10–100 events / session missing in log-store for sessions with hook activity. Tool stream and convenience events (which flow through `readEvents`) were already in parity.
2. **Backfill any gaps**: replay missing events from bridge-server's table into log-store. **Decision 2026-05-11: skip historical backfill.** Backfilling re-inserts rows under new log-store ids, which inverts chronological order against the existing id-asc reads. The missing types (hook, partial system) don't feed any read path we cut over to (turn-state queries `user_message` + `result`/`error`; SSE replay attaches by `message_id`; git discovery reads `tool_call`). New events (post-2026-05-11 deploy) are mirrored perfectly. Historical gap is cosmetic in materialized history for finished hook bubbles; tolerable.
3. **Read-path audit**: enumerate every place bridge-server reads from its `events` table (SSE replay endpoint, `MaterializedMessage` rendering, `RecoverInFlightTurn`, `HarnessToBridgeMap`, `InFlightTurnState` recovery). Confirm log-store can serve each query — extending log-store's API where needed before the read switch. **Done 2026-05-10:** 8 read methods inventoried; 3 disappear after III.B (`HarnessToBridgeMap`, `ToolUseToMessageMap`, parts of `RecoverInFlightTurn`); rest are served by `turn-state` + `?types=` filter (shipped 2026-05-10 in log-store 43e7b7a).
4. **Read switch**: route bridge-server's read paths to log-store's HTTP API. Bridge-server still writes to both tables. **Partial 2026-05-11 (bridge-server 82ccb3e).** Cut over four non-SSE read paths: `RecoverInFlightTurn` (harness boot), `PendingTurnMessage` (auto-resume), `RecentTurnTexts` (renamer), `ListToolCallInputs` (git endpoint). All four use bridge-server's row ids only internally and never expose them on the wire, so the id-space switch is invisible to clients. SSE replay paths (`ListEventsSinceID`, `ListCurrentTurnEventsWithIDs`) intentionally deferred: switching them mid-flight would change the row-id space against which connected SSE clients send `Last-Event-ID`, causing wrong replay until they reconnect again. The verification window starts here.
5. **Verification window**: leave dual-write running for ~1 week. Monitor for divergence. Confirm SSE replay and turn recovery work correctly under reads-from-log-store.
6. **SSE replay cutover**: switch `ListEventsSinceID` and `ListCurrentTurnEventsWithIDs` to log-store. Requires either (a) a brief window of new-row-id-only replay (clients accept a one-time replay glitch) or (b) a translation table mapping legacy bridge-server row ids to log-store row ids. Acceptable to do at a quiet hour.
7. **Stop dual-write**: bridge-server stops inserting into its own `events` table.
8. **Drop the table**: only after the verification window, drop bridge-server's `events` table and the related internal types (`EventWithID`, `InFlightTurnState`, `ToolUseBinding`, `TurnText`).

Bridge-server reads through log-store via HTTP. No local cache; cold-start cost on turn recovery is acceptable since restarts are rare.

### II.B — Memory-store untangle + log-store sessions projection

memory-store's `sessions` and `session_tags` tables are dead infrastructure (the only entry point is `PrepareSession`, which loads memories *for* a session; `SaveSession` has no live callers). Drop them.

Two related cleanups:

- **`msg.Session`** (`msg/session.go:9`) and **`msg.SessionTask`** (`msg/session.go:39`) are inber-era rich types with no remaining home in the bridge stack. Delete from canonical types.
- **memory-store keeps `memories`, `memory_tags`, `memory_usage`**. `memory_usage` retains `session_id` as an opaque FK with no constraint (references the canonical session_id from bridge-server).

Aggregate fields (`agent_name`, `model`, `cost`, `summary`, `started_at`, `ended_at`, `input_tokens`, `output_tokens`) that lived in memory-store's sessions table are now derived from log-store events. To avoid scanning events on every "show me all sessions sorted by cost" query, **log-store grows a `sessions` projection table**:

| Column | Source |
|---|---|
| `session_id` | PK; from any event in this session |
| `input_tokens`, `output_tokens`, `cost_usd` | SUM of result events |
| `turn_count` | COUNT of user_message events |
| `model` | most recent model from result events |
| `agent_id`, `agent_name` | from session_info events / first event |
| `started_at`, `last_active`, `ended_at` | first event / last event / terminal event |
| `summary` | from auto-summary events (if/when those exist) |

Updated synchronously on event ingest. Always rebuildable from the underlying events — drop the table, replay events, get back the same data. This is a derived cache, not a separate source of truth.

Bridge-server's `sessions` table stays as the **lifecycle** owner: state, pid, instance_id, source, session_type, display_name, harness, mode, folder. Two tables, two concerns:
- `bridge-server.sessions`: "what is this session doing right now?"
- `log-store.sessions`: "what has this session done in total?"

The frontend joins on session_id when it needs both. No third service.

### II.C — Harness mapping moves to adapter

`harness_session_id` is a harness-specific concept that bridge-server tracks today only because that's where the code grew. Move it to the harness adapter.

Each harness adapter that has a real native session id (claudecode, codex, jig, hermes — not pure HTTP harnesses where session_id IS the harness id) keeps a small SQLite:

```sql
CREATE TABLE harness_sessions (
  session_id          TEXT PRIMARY KEY,
  harness_session_id  TEXT NOT NULL,
  harness_path        TEXT,             -- on-disk session file, if applicable
  created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_harness_session ON harness_sessions(harness_session_id);
```

The adapter:
- Writes a row when bridge-server tells it to start a session.
- Updates `harness_session_id` when the harness reports its native id.
- Looks up by `session_id` when bridge-server says "resume" or "fork".
- Uses the table as the dedup index when scanning disk for `StoredSession` discovery.
- Exposes `GET /sessions/{session_id}` returning `{harness_session_id, harness_path, ...}` for frontend diagnostics.

Bridge-server side:
- Drop `harness_session_id` column from `sessions` table.
- Drop the partial UNIQUE index on `harness_session_id`.
- Drop the phantom-row self-heal logic at `store.go:207-219`.
- The frontend's lazy diagnostics call goes through bridge-server, which proxies to the adapter.

`Event` still carries `harness_session_id` if an adapter chooses to stamp it (transparent pass-through), but bridge-server doesn't read or persist it. (Phase III.B removes the field from `Event` entirely; II.C just removes it from bridge-server's storage layer.)

## Phase III — Message and Turn Identity

Applies the Phase I consolidation pattern to messages and turns, and removes derived/redundant fields.

### Final identifier inventory after Phase III

Per event:

| Field | Required | Minted by | Scope |
|---|---|---|---|
| `session_id` | always | caller or bridge | one chat/agent session |
| `turn_id` | always | caller or bridge | one speaker contribution (user message OR assistant response OR lifecycle boundary) |
| `message_id` | always | caller or bridge | one message bubble (multiple events may share when coalescing stream/tool pairs) |

Per session (already covered in Phase I):

| Field | Required | Minted by |
|---|---|---|
| `session_id` | yes | caller or bridge |
| `instance_id` | yes (resolved) | resolved at create |
| `source` | yes | caller |
| `session_type` | yes | caller |

Layer-private metadata (not on wire post-Phase III):

| Field | Owner |
|---|---|
| `harness_session_id` | adapter (II.C) |
| `harness_message_id` | adapter (III.B) |

Removed:

- `client_id` (Phase I)
- `client_request_id` (III.D, replaced by message_id idempotency + turn ordering)
- `actor` (III.E, derived from `kind`)
- `bridge_id` (Phase I, renamed to `session_id`)
- `bus_session_id` (Phase I, collapsed into `session_id`)
- `bridge_session_id` JSON tag inconsistency (Phase I, all `session_id`)

### III.A — Single canonical message_id

Apply the session_id pattern: caller may mint `message_id` when sending; bridge mints `msg_{ULID}` if absent. Same idempotency story — caller retries with same `message_id`, bridge dedupes.

Frontend benefit: the optimistic React key (today's misnamed `LogRow.clientId`, value `tmp_{ts}_{rand}`) goes away. The frontend mints the canonical `message_id` at submission time and uses it as its React key directly. No swap when `/send` returns; optimistic and canonical are the same value.

### III.B — Harness mapping moves to adapter (strong form, CC first)

`harness_message_id` is removed from `msg.Event` entirely. Adapters mint `message_id` themselves and own all reconciliation. Specifically:

| Use today | New owner | Mechanism |
|---|---|---|
| Resume reconciliation (`assignAssistantID:257-262`) | Adapter | Adapter holds `harness_id ↔ message_id` mapping in its SQLite |
| Split detection (`assignAssistantID:266-268`) | Adapter | Adapter mints a new `message_id` when the harness's native id changes mid-turn |
| Tool use rebinding (`manager.go:236-237`) | Adapter | Adapter resolves binding from its own state |
| Persistent mapping (`store.go:846 HarnessToBridgeMap`) | Adapter | Adapter's SQLite |
| `events.harness_message_id` column | (removed) | Not stored anywhere on the bridge side |
| Frontend `hid:` chip rendering | Lazy fetch | `GET /api/sessions/{id}/messages/{message_id}/harness-info` |

Bridge-server drops `assignAssistantID`, `HarnessToBridgeMap`, the `harness_message_id` column, and the `idx_events_harness_msg_id` index. The events table column also drops in log-store (II.A migration).

**Focused on claudecode first.** Other harness adapters (codex, jig, etc.) get a stub that just passes through `message_id` from the bridge until they're updated — works for any harness without native multi-message-per-turn behavior. The shared logic lives in a small `bridge/identity` package in `llm-bridge` itself, importable by adapters that need it (so we don't reimplement split-detection in every adapter).

### III.C — Turn redefined; turn_id always required

A turn is **a discrete event sequence from a single initiator**:

- Session creation mints a turn_id for the init/lifecycle phase.
- Each user message mints a new turn_id (the user's turn).
- The LLM response mints its own turn_id (the assistant's turn — covers streaming text, tool calls, tool results, and the result event, regardless of how many message bubbles).
- Subsequent lifecycle events (state transitions, etc.) mint their own turn_ids at boundaries.
- Autoworker / scheduler / dispatcher creating a session can supply a turn_id at create time, same caller-or-bridge mints pattern.

`turn_id` becomes a peer to `session_id` and `message_id`: caller-or-bridge mints, always present, NOT NULL in the events table.

This is a **semantic redefinition**. Today turn_id covers user_message → result/error (round-trip). After: turn_id covers a single speaker's contribution. A conversational exchange becomes two turns (user turn + assistant turn) instead of one. This is closer to how "turn" is used in conversational AI literature.

Implications:
- `TurnCompleteEvent` and other convenience events fire per-speaker-turn — usage/cost stats arrive per assistant response, not per round-trip. More granular and more useful.
- Queries that say "all events in this turn" return less now. Audit any caller that depends on the old scope.
- Cross-turn linkage (user message ↔ corresponding response) goes via session ordering — response turn is the next turn after the user turn in the same session. No `parent_turn_id` field for now; can be added later if non-linear sessions become a thing.

### III.D — Drop request_id (was client_request_id)

With turn_id per-speaker, `request_id` has no remaining scope. Its three jobs collapse:

- **Idempotency for retries**: handled by `message_id` (user retries with same `message_id`; bridge dedupes).
- **Stamping the round-trip**: handled by turn ordering (response turn is the next turn in the session after the user turn).
- **Audit trace correlation**: handled by message_id chain plus turn ordering.

The field is removed from `Event`, from `SendMessageRequest`, and from any storage. Adapters that needed it for harness retry semantics handle that internally.

### III.E — Drop actor field; render by kind

`actor` is computed from `ev.type` in `bridge-ui`'s `actorFor()`, never stored, only consumed by CSS class names. It's a 1:1 derivation from `kind` (which is itself a derivation from `ev.type`). Three layers of representation for the same fact, and the derivation has bugs — convenience events emitted by the bridge surface as `bc-row-assistant` despite being metadata.

Fix:
- Delete `actor` from `LogRow`.
- Render rows with `bc-row-{kind}` directly. CSS handles whatever visual grouping is needed.
- Extend `rowKindOf()` to recognize convenience event types (`agent_state`, `usage_total`, `turn_complete`) instead of falling through to `'other'`. Give them a `kind` like `'turn_meta'` or fold into `'session_info'`.
- Decide rendering for the convenience events: hidden in turns/timeline view, visible in thread view (which is the raw event log).

After this fix, the user's reported bug (assistant-class row with turn_id but no message_id) goes away — the convenience event renders as `bc-row-turn_meta` (or whatever kind), not `bc-row-assistant`.

## Affected repos (consolidated across all phases)

Direct (require code changes):

- **`llm-bridge`** — canonical types in `msg/server.go`, `msg/event.go`, `msg/session.go` (delete `Session`, `SessionTask`); generated `ts/msg.ts` and `py/llm_bridge_types/msg.py`. New `session_type` field; rename of `bridge_id`/`bridge_session_id` → `session_id`; removal of `client_id`, `client_request_id`, `harness_message_id` from `Event`, `actor` references. New `bridge/identity` package for shared adapter mapping logic.
- **`llm-bridge-server`** — `internal/store/store.go` schema (rename `bridge_id`, drop `client_id`, drop `harness_session_id`, drop `harness_message_id`, drop entire `events` table after II.A). All `WHERE bridge_id=?` queries. HTTP route path parameters. `assignAssistantID`, `HarnessToBridgeMap`, `RecoverInFlightTurn` logic redirected through log-store. Move `permclient` request struct into `llm-bridge/msg/`. Add proxy endpoints for harness diagnostics.
- **`llm-bridge-adapter`** — drop `bus_session_id ↔ bridge_id` mapping logic; replace `bridgeSession` wrapper with `msg.ManagedSession`. After Phase I, may remove `~/.llm-bridge-adapter/sessions.json` entirely.
- **`llm-bridge-claudecode`** (and other harness adapters that need it) — gain SQLite for `harness_sessions` mapping, the `bridge/identity` reconciliation logic, and the diagnostics HTTP endpoint. Mint `message_id` on bubble boundaries.
- **`log-store`** — extend API for queries bridge-server needs (turn recovery, tool-use binding, materialized message variants). Add `sessions` projection table updated synchronously on ingest. Drop `harness_message_id` column when ingesting (no longer in `Event`).
- **`memory-store`** — drop `sessions`, `session_tags` tables; drop `SaveSession`. Keep `memories`, `memory_tags`, `memory_usage` (with session_id as opaque FK).
- **`bridge-ui`** — `src/types.ts`: rename `bridge_id` → `session_id`, drop `client_id`, drop `client_request_id` references on event-shaped types, drop `actor` from `LogRow`, drop `LogRow.clientId` (was the optimistic React key). Components that read `session.bridge_id` (Workspace, SessionList, SessionHeader, SessionBypassToggle, BridgeUsage, bridgeSSE). Frontend mints `message_id` at submission time. Lazy-fetch `harness_message_id` for the diagnostic chip. Extend `rowKindOf()` for convenience events.
- **`scheduler`** — `cmd/autoworker`, `cmd/kanban-dispatcher`, `cmd/kanban-curator`, `cmd/kanban-classifier`, `internal/executor/agent.go`. Stop double-stamping `client_id`+`source`; pass `session_id`, `source`, `session_type`. Remove `bus_session_id` minting and the adapter sessions.json reads in kanban-curator.
- **`inber`** — `server/api_bridge.go`, `scripts/harness-watch.sh`. Pass-through references to old field names. Audit any direct memory-store sessions calls.

Indirect (no code change but semantics shift):

- **`kanban-store`** — opaque to the change. Card-link `entity_type="bus_session"` literal lives in scheduler; kanban-store just stores strings. After Phase I, the entity_type label can stay or be renamed to `"session"`; entity_ref values shift.
- **`harness-store`** — no references; unaffected.
- **`dash`, `llmux`** — host bridge-ui. Inherit changes when bridge-ui ships.
- **Other harness adapters** (`-codex`, `-jig`, `-anthropic`, `-openai`, etc.) — pass-through wrappers. Need to import the new `bridge/identity` package only if they have multi-message-per-turn behavior; otherwise unchanged.

## Open questions

- **Frontend message_id minting on session resume**: when a user reconnects to an existing session, the frontend doesn't know what message_ids the bridge expects. Either the frontend asks the bridge for the next `message_id` before composing (extra round-trip), or the bridge accepts caller-minted `message_id` only on Send and mints itself on resume. Probably the latter — caller mints when it can, bridge mints otherwise.
- **`bridge/identity` package shape**: what's the minimal surface adapters need? At a minimum: a `MessageIDMinter` that takes a harness_id and returns a stable bridge message_id, with persistence callbacks the adapter implements. Worth designing the API before implementing in claudecode.
- **Cross-turn linkage**: implicit ordering vs. explicit `parent_turn_id`. Doc commits to implicit for now. Revisit if non-linear conversation patterns arise.
- **Auto-summary events**: the log-store sessions projection has a `summary` column sourced from "auto-summary events (if/when those exist)." None exist today. Either drop the column from the projection until they do, or leave it and let it stay NULL.
- **Card body markdown in autoworker**: `scheduler/cmd/autoworker/main.go:217` writes `bus_session_id: \`%s\`` into kanban card body — user-visible markdown. Replace with `session_id` once Phase I ships, but historic cards keep the old text. Acceptable.

## Out of scope

- The `Session` rich-state entity in `msg/Session` — being deleted (II.B), not preserved.
- Pre-canonical raw harness frames before adapter normalization — separate concern if needed later.
- `parent_turn_id` for cross-turn linkage — deferred until non-linear sessions become real.
