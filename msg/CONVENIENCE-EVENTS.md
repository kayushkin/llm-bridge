# Convenience Events — Design Spec

Status: **partially superseded (2026-05-06)** — the `agent_state` projection
described below has been folded into `SessionState`; the rest of the spec
(usage_total, turn_complete) still applies.

## Migration note (2026-05-06)

The original spec proposed `AgentState` as a coarser 4-value UI projection
of `SessionState` (which had 6 values mirroring the harness lifecycle).
With the agent-manager work in bridge-ui requiring finer granularity than
either enum carried, `SessionState` has been extended to 13 values that
encompass the full UI vocabulary. `AgentState` is no longer a coarsening —
it would be a strict subset of the new `SessionState` — so it has been
deprecated.

**Single SessionState (13 values), grouped by operator action:**

- Pre-flight: `starting`
- Active: `model_generating`, `tool_running`, `compacting`
- Blocked on user: `awaiting_permission`, `awaiting_user`
- Self-healing wait: `rate_limited`, `paused`
- Quiet: `idle`
- Terminal: `completed`, `error`, `aborted`, `disconnected`

**Authoritative source:** llm-bridge-server derives `SessionState` from
the raw event stream plus server-only context (pause/abort calls,
permission-store hook state, subprocess lifecycle, provider rate-limit
signals). Harness packages no longer emit `EventSessionState` — that's a
follow-up cleanup. Until they do, the server drops harness-emitted
`EventSessionState` on intake (defensive); only the central derivation
emits the authoritative state.

**Deprecation surface (kept valid during migration):**

- `msg.AgentState` type + 4 constants
- `msg.AgentStateEvent` struct
- `msg.EventAgentState` event type
- `Event.AgentState` field
- `SessionRunning`, `SessionWaitingApproval` constants (replaced by the
  split `model_generating`/`tool_running` and renamed `awaiting_permission`)

These will be removed in a follow-up PR once consumers stop reading them.

**Presentation note:** pill colors / icons / row groupings are derived in
a separate presentation map at the rendering edge. The enum stays granular
regardless of how many distinct colors a UI ends up using. Two states with
the same color are still distinct states.

---

## Original spec (2026-04-27, AgentState section is historic)

Status: **draft (2026-04-27)** — pre-implementation. Open questions tagged `[OPEN]`.

## Why

Every consumer of `msg.Event` over SSE re-derives the same three pieces of high-level state from the raw event stream:

1. **Where in its lifecycle is this session right now?** Currently each consumer has to watch `EventSessionState` (with 6 distinct values), `EventToolCall`/`EventToolResult` to know if a tool is in flight, and `EventApproval` to know if a permission prompt is pending. Every UI gets this slightly differently.
2. **How much has this session cost so far?** `ResultEvent.Usage` is per-turn. To get a session running total, every consumer keeps its own running sum.
3. **Did the agent just finish a turn?** There's no single signal that says "the user-visible turn the user just sent is done". Consumers infer it from `EventResult` plus the previous `user_message`'s `turn_id`.

User intent (from todo `834ca382`):

> Improvement #4 from the modularity roadmap. Add three derived event types to msg.Event so consumers do not have to re-derive them.

The three new event types — `state`, `usage_total`, `turn_complete` — are pre-aggregated convenience signals derived once, by `llm-bridge-server`, from the raw event stream. Consumers that want them get them; consumers that don't can ignore them (they land in `Overflow` for old clients).

This is the modularity move: today an integrator wanting "show a status pill + cumulative cost + per-turn summary" must consume the full event stream and re-derive everything. After this, those three pieces are first-class events. Smaller integration surface; broader set of projects we can plausibly fit into.

## Non-goals (this spec)

- **Replacing the existing event types.** `EventSessionState` (the 6-state lifecycle) stays — `state` is a coarser, UI-shaped projection of it, not a substitute. Same for `EventResult` vs `turn_complete`.
- **Per-API-call usage breakdown.** `usage_total` is session-cumulative. Per-call breakdown is already in `ResultEvent.APICallUsages` and stays there.
- **Cost projections / budget enforcement.** `usage_total` reports what was spent. Anything that needs "what would the next turn cost" or "stop if over $X" is consumer logic.
- **Deriving from harness internals the harness doesn't surface.** We derive from `msg.Event` only — no peeking at raw NDJSON, no harness-specific shortcuts.

## Surface

### `state` — agent status projection

Coarse, UI-friendly status. Three or four discrete values (see `[OPEN]` below):

```go
// AgentState is the UI-friendly projection of session lifecycle.
type AgentState string

const (
    AgentStateIdle           AgentState = "idle"            // no active turn; ready for next user message
    AgentStateAwaitingInput  AgentState = "awaiting_input"  // turn paused; needs user (approval, mid-turn question)
    AgentStateToolRunning    AgentState = "tool_running"    // turn active; tool currently executing
    AgentStateError          AgentState = "error"           // last activity ended in error
)

// AgentStateEvent is the body for EventAgentState.
type AgentStateEvent struct {
    State    AgentState `json:"state"`
    Previous AgentState `json:"previous,omitempty"`
    Reason   string     `json:"reason,omitempty"` // free-form, e.g. "tool=Bash", "approval_required"
}
```

Carried on `Event` as a new pointer field:

```go
AgentState *AgentStateEvent `json:"agent_state,omitempty"`
```

**Emission rule:** one event on every transition. Server-side derivation maintains a per-bridgeID state machine (see "Implementation" below). No event when state doesn't change.

### `usage_total` — cumulative session usage

```go
// UsageTotalEvent is the body for EventUsageTotal.
type UsageTotalEvent struct {
    Usage TokenUsage `json:"usage"`           // sum across every result event in this session
    Cost  *Cost      `json:"cost,omitempty"`  // sum where reported
    Turns int        `json:"turns"`           // how many completed turns contributed
}
```

Carried on `Event` as:

```go
UsageTotal *UsageTotalEvent `json:"usage_total,omitempty"`
```

**Emission rule:** one event after every `EventResult` fan-out. The server's Manager already sees every `EventResult`; on each, it adds the result's `Usage` to a per-bridgeID accumulator and emits `usage_total` with the running totals.

`TokenUsage` field-level addition: input + output + cache_read + cache_write + reasoning + total. `ContextTokens` and `ContextLimit` are **last-value-wins** (not summed) — they describe current-context state, not consumption.

### `turn_complete` — coalesced turn summary

```go
// TurnCompleteEvent is the body for EventTurnComplete.
type TurnCompleteEvent struct {
    TurnID         string        `json:"turn_id"`                 // matches the originating user_message's turn_id
    FinalMessage   string        `json:"final_message,omitempty"` // last assistant text in the turn (from ResultEvent.Text)
    ToolCalls      []ToolSummary `json:"tool_calls,omitempty"`    // every tool_call/tool_result pair seen in this turn
    UsageDelta     TokenUsage    `json:"usage_delta"`             // this turn's contribution
    Cost           *Cost         `json:"cost,omitempty"`
    DurationMS     int64         `json:"duration_ms"`             // wall-clock from user_message → result
    IsError        bool          `json:"is_error,omitempty"`
    ErrorMessage   string        `json:"error_message,omitempty"`
}
```

Carried on `Event` as:

```go
TurnComplete *TurnCompleteEvent `json:"turn_complete,omitempty"`
```

**Emission rule:** one event per turn, fired immediately after the terminating `EventResult` (or `EventError` for error-terminated turns) is fanned out. Server keeps a per-turn accumulator keyed on `Event.TurnID`; closes it on terminator.

`ToolSummary` is the existing type from `event.go`. Reusing it keeps the schema consistent with
`ResultEvent.ToolEvents` at the type level — but note that `ResultEvent.ToolEvents` is filled by
nothing on the live event path (measured 2026-08-22: 2 of 28 constructions set it, both in
`import_history.go`). `TurnCompleteEvent.ToolCalls` is the only per-turn tool record any harness
actually produces. See the note on `ToolEvents` in `event.go`.

### New EventType constants

In `msg/provider.go`:

```go
const (
    // ... existing values ...
    EventAgentState   EventType = "agent_state"
    EventUsageTotal   EventType = "usage_total"
    EventTurnComplete EventType = "turn_complete"
)
```

Note: chose `agent_state` rather than `state` to disambiguate from the existing `session_state` event type. They're related but distinct: `session_state` is the harness-reported lifecycle (6 values), `agent_state` is the UI-shaped projection (4 values). Confusing them is the worst-case bug; distinct names prevent it.

### TS / Python regen

Run `~/repos/llm-bridge/generate-ts.sh` and `~/repos/llm-bridge/cmd/genpy` after the Go types land. The new types flow through tygo + the python generator without manual intervention — already exercised by every prior msg-package change.

## Implementation

### Where derivation lives

Centrally, in `llm-bridge-server`. Specifically: `Manager.readEvents` at `~/repos/llm-bridge-server/internal/harness/manager.go:373`. Every harness event already flows through this function — it persists, updates session state, and fans out to SSE subscribers. Adding a derivation step here means:

- **One source of truth.** No per-harness duplication. Adding a new harness gets convenience events for free.
- **Same fan-out path.** Derived events go through the existing `BroadcastEvent` mechanism (manager.go:515), so they reach SSE subscribers, log-store persistence, and `Last-Event-ID` replay automatically.
- **No new persistence layer.** Derived events are stored alongside raw events, replayed alongside them on reconnect.

Rejected alternatives:

- **(a) Each harness emits these alongside raw events.** Forces every harness bridge to implement the same state machine. Drift across harnesses is inevitable. No.
- **(c) Separate aggregator package consumers run.** Pushes the same complexity onto every consumer that the centralized version eliminates. The whole point of this work is to NOT make consumers re-derive.

### Per-bridgeID derivation state

A new struct lives on the `Manager` keyed by `bridgeID`:

```go
type derivationState struct {
    agentState     msg.AgentState
    usage          msg.TokenUsage
    cost           msg.Cost
    turns          int
    activeTools    map[string]string // tool_use_id → tool name (for tool_running)
    turnAccum      map[string]*turnAccumulator // turn_id → in-progress turn data
    pendingHookRequests map[string]struct{} // RequestIDs of awaiting_resolution hooks
    awaitingApproval bool                    // derived: len(pendingHookRequests) > 0
}
```

Lifecycle:

- Created on first event for a bridgeID.
- Mutated inside `readEvents` after persistence, before fan-out (so derived events are visible to subscribers in the same fan-out cycle).
- Torn down on session stop (Manager already has the cleanup hook at `manager.go:498-509`).

### State machine

Transitions, in order of precedence within a single inbound event:

| Inbound event | New `agent_state` | Notes |
|---|---|---|
| `EventUserMessage` | `tool_running` (provisional) | Turn started; idle → tool_running. (Or `awaiting_input` if no model call yet — see `[OPEN]`.) |
| `EventToolCall` | `tool_running` | Add tool to `activeTools`. |
| `EventToolResult` | `tool_running` if any active remain, else previous | Remove from `activeTools`. |
| `EventHook` (phase=awaiting_resolution) | `awaiting_input` | Mark `awaitingApproval=true`, key by RequestID. |
| `EventHook` (phase=completed, has Resolution) | previous (typically `tool_running`) | Clear pending RequestID; `awaitingApproval` clears when none remain. |
| `EventApproval` (status=requested) | `awaiting_input` | **Deprecated** — emitted by harnesses on the legacy approval path. Same effect as `EventHook` awaiting_resolution. |
| `EventApproval` (status=resolved) | previous (typically `tool_running`) | **Deprecated** — paired with the legacy requested event. |
| `EventResult` | `idle` | Turn complete (success). |
| `EventError` | `error` | |
| `EventSessionState{State: SessionAborted}` | `idle` | User killed the turn; back to ready. |

Edge cases:

- **Streaming text (`EventStream`) is not a state transition.** State stays whatever it was. The model-generation phase between user message and tool call/result lives under `tool_running` per the table above (see `[OPEN]`).
- **Multiple concurrent tool calls.** `tool_running` until ALL active tools finish.
- **`awaiting_approval` overrides `tool_running`.** While any hook is in `awaiting_resolution` (permission-prompt or human-resolved hook), agent_state is `awaiting_input` even though a tool is technically mid-flight.
- **Multiple concurrent pending hooks.** Track pending RequestIDs as a set; `awaitingApproval` is true while the set is non-empty, false when it drains.

### Per-turn accumulator (for `turn_complete`)

```go
type turnAccumulator struct {
    turnID       string
    startedAt    time.Time
    toolCalls    []msg.ToolSummary
    activeCalls  map[string]*msg.ToolSummary // tool_use_id → in-progress (filled by tool_call, completed by tool_result)
}
```

Created on first event seen for a `turn_id` (typically `EventUserMessage`). Closed on `EventResult` or `EventError` carrying that `turn_id` — at which point a `turn_complete` event is constructed and broadcast.

Memory bound: keep at most N=8 in-progress turns per session; oldest is evicted (with a logged warning) if a turn never terminates. Pathological case only; normal sessions have ≤1 in-flight turn.

### Ordering with raw events

For the same inbound raw event, the derived event is emitted **after** the raw event reaches subscribers. Two reasons:

1. UIs that consume both want to see the raw event first (e.g., update message bubble) then the derived signal (e.g., update status pill).
2. `Last-Event-ID` replay must preserve causal order. The raw event is the cause; the derived event is the effect. Persisting in that order is unambiguous.

Implementation: `readEvents` does its existing fan-out at `manager.go:447-494`, then immediately calls `m.deriveAndBroadcast(bridgeID, &event)` which may produce 0, 1, or multiple derived events (a single result event can produce both a `usage_total` and a `turn_complete`).

### Forward-compatibility

Old SSE consumers don't know `agent_state`/`usage_total`/`turn_complete`. They get the raw type discriminator on `Event.Type`, fail to find a matching switch case, and the entire event lands in `Overflow`. This is the existing forward-compat mechanism; no special handling needed.

`for-integrators.md` should advertise the new events as **opt-in to consume but always-on to emit** — i.e. the server always emits them, but consumers choose whether to switch on them.

## Testing

### Conformance tests

Add to `~/repos/llm-bridge-server/conformance/`:

- A harness recording (existing pattern: replay a recorded NDJSON session through the manager) where each step's expected derived events are asserted.
- One recording per "interesting" path: simple turn, multi-tool turn, approval mid-turn, error mid-turn, aborted turn.

### Unit tests on the state machine

The derivation state struct and transition table are pure logic. Test in isolation:

```go
func TestDerivation_SimpleHappyPath(t *testing.T) { ... }
func TestDerivation_ApprovalOverridesToolRunning(t *testing.T) { ... }
func TestDerivation_MultipleConcurrentTools(t *testing.T) { ... }
func TestDerivation_TurnCompleteOnError(t *testing.T) { ... }
```

No subprocess spawn, no I/O. Should be fast and high-coverage.

### Integration check

Existing `TestSessionEvents_*` tests in `internal/server/server_test.go` should continue to pass; the new derived events show up in their event streams but don't break their assertions (they switch on specific raw event types).

Add one new integration test that asserts a recorded session ends with `agent_state == idle`, `usage_total.turns == N`, and a `turn_complete` per turn.

## Phasing

Children scoped so each can be finished by an unattended session. 1 is a prereq for 2-4. 5 + 6 are docs/test polish.

1. **Add the three event types + new EventType constants to `msg/`.** Pure type addition. Run `generate-ts.sh` + `cmd/genpy`. Update `validate.go` if any new validation codes apply. No derivation yet — the types ship empty. Conformance suite still passes.
2. **Implement `state` derivation in `llm-bridge-server`.** New `derivationState` struct, transition table from this spec, hook into `Manager.readEvents`. Unit-test the state machine in isolation. Wire `agent_state` events into fan-out + persistence. One recorded conformance fixture asserts state transitions.
3. **Implement `usage_total` derivation.** Cumulative accumulator hooked to `EventResult`. Unit + conformance tests.
4. **Implement `turn_complete` derivation.** Per-turn accumulator + emission on terminator. Unit + conformance tests.
5. **Update `for-integrators.md` and `~/repos/llm-bridge/README.md`.** New section: "Convenience events: less wiring, same data". Show the four-line consumer pattern that wasn't possible before.
6. **End-to-end check against a real session.** Spawn claudecode, send a tool-using prompt, assert the convenience event sequence matches expectations. Live test (real `claude` binary), behind a build tag, similar to the pty integration test pattern.

## Open questions

- `[OPEN]` **Three states or four?** The user originally asked for `idle / awaiting_input / tool_running / error` — which is what's specified above. But this elides "model is generating text" between user_message and the first tool call (or final result). Two options:
  - **A. Stay at four states.** "Model generating" is implicit when state == `tool_running` with `activeTools` empty. UI shows "Working…" generically. Simpler API. The downside: `tool_running` is a slight misnomer for the text-generation case. Could rename the constant to `working` to reduce that mismatch.
  - **B. Add a fifth state `generating`.** Distinct UI affordance ("✨ thinking" vs "🔧 running Bash"). More expressive. The downside: "is the model still streaming text or done?" requires watching `EventStream` close, which means another piece of bookkeeping in the derivation layer.
  - **Lean: A with rename to `working` (or keep `tool_running` if four-state-only is preferred for backwards-compat with the user's original wording).** Pick before child 2.
- `[OPEN]` **Field name for the carrier on `Event`.** This spec uses `AgentState *AgentStateEvent`. Could also be `Status` or `AgentStatus`. The reason `agent_state` was picked over `state`: there's already an `EventSessionState` whose body is `*StateEvent`, and reusing `state` as a field name would silently shadow nothing but read confusingly. Safe to revisit.
- `[OPEN]` **Should `turn_complete` carry the full conversation slice for the turn, or just the summary above?** Full slice (every assistant message bubble + tool call) would let a consumer render a turn from `turn_complete` alone with no scrollback over the event log. Bigger event payload, but simpler consumers. Lean: summary-only in v1; revisit if users ask.
- `[OPEN]` **Backfill on reconnect.** A consumer reconnecting with `Last-Event-ID` will replay the raw event tail and any derived events that were persisted alongside them. But what if the server restarted mid-turn and the in-memory `derivationState` was lost? Two options: (a) rebuild from the persisted event log on first event after restart (full replay through derivation, costly for long sessions), (b) persist derivation snapshots (more code but bounded cost). Defer to a follow-up; v1 ships without restart-survival and accepts the rare anomaly that the first turn after a server restart may emit a slightly-off `usage_total` until the next `result` event re-anchors it.
- `[OPEN]` **Cost summation when one turn reports cost and another doesn't.** `ResultEvent.Cost` is `*Cost` (nullable). For `usage_total`, do we (a) sum present-only and emit a `cost` field whose `total_usd` reflects only the priced turns, or (b) leave `cost` null whenever any turn was un-priced? Lean: (a) — partial cost is more useful than no cost; document the behavior.

## Out of scope, but worth noting

- **Streaming text token estimates.** `usage_total` updates only on `EventResult`. A consumer wanting a live "tokens used so far in the in-flight turn" needs to estimate from `EventStream` deltas. Not in scope.
- **Cross-session aggregates.** Convenience events are per-session. Aggregating across sessions ("total cost this week") is what `log-store` is for.
- **`turn_started` event.** Symmetric to `turn_complete` would be `turn_started` (fired when a turn-bearing `user_message` is seen). Useful but not requested. If a consumer needs it today they can switch on `EventUserMessage` themselves; no derivation required. Add later if multiple consumers ask.
