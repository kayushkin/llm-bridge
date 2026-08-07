# ACP as a surface for llm-bridge — the two measurements

Measured 2026-08-07 (nightly worker). Decision todo: `48504ba6-67ea-4ab7-90b4-b0b77e3522ed`.
Source of the question: `RELEASE-TARGETS.md` §"The finding that may outrank this whole
document: ACP", from the Tier B re-validation sweep (llm-bridge `16274fa`).

**Nothing here decides anything.** That todo reserves four judgements for the owner. It also
names two things as *not verified and needed before any build*, and both are measurement
rather than judgement:

> ⚠️ Not verified by this pass and needed before any build: the ACP spec version each of the
> four projects implements, and whether they agree.

> 3. What is the mapping, and what is lossy? ACP's `session/update` notification stream
>    against `msg.Event`. The sweep did NOT measure this.

This document is those two measurements and nothing else. Both came back with results that
change how the decision should be priced, and neither answers it.

Everything below was read off the ACP schema
(`agentclientprotocol/agent-client-protocol`, `schema/v1/schema.json`, 242 KB, and
`schema.unstable.json`) and off `msg/` in this repo. The upstream org has moved — the old
`zed-industries/agent-client-protocol` URL redirects.

---

## Measurement 1 — do the four consumers agree on a wire format?

**On the version integer, yes. On what an agent must actually satisfy, no.**

All four send `protocolVersion: 1`, and that is the weakest possible agreement: `1` is a
constant that has never been bumped in a shipped release, so no implementation could get it
wrong. `ProtocolVersion::LATEST` is still `V1` in the newest schema crate.

What they were compiled against is a different question, and there the spread is ~8 months:

| | omnigent | happy | AionCore (`acp_conn`) | vibe-kanban-indie |
|---|---|---|---|---|
| client library | hand-rolled | npm `@agentclientprotocol/sdk` **^0.14.1** (2026-02) | hand-rolled, deliberately SDK-free | crate `agent-client-protocol` **0.8.0** / schema 0.9.1 (2025-12) |
| unknown `sessionUpdate` | silently dropped | zod-rejected, logged, dropped | **preserved** as `AdapterSpecific` | serde-rejected, logged, dropped |
| `plan` | ✗ | ✗ (probes `update.plan`; the spec frame carries `entries`) | ✓ | ✓ |
| `usage_update` | ✓ | ✗ | ✓ | **✗ rejected — absent from schema 0.9.1** |
| `config_option_update` | ✗ | ✓ | ✓ | **✗ rejected — absent from schema 0.9.1** |
| `available_commands_update` | ✗ | ✓ | ✓ | ✗ |
| `current_mode_update` | ✗ | ✓ | ✓ | ✗ |
| `fs/read_text_file`, `fs/write_text_file` | ✓ conditional | ✗ | ✗ (`-32601`) | ✗ (`method_not_found`) |
| `terminal/*` | ✗ | ✗ | ✗ | ✗ |

Points worth carrying into the decision:

- **The guaranteed-handled set is four variants**: `agent_message_chunk`,
  `agent_thought_chunk`, `tool_call`, `tool_call_update`. Plus `session/request_permission`,
  the one reverse method all four honour. The union across the four is 13 variants plus three
  non-spec legacy shapes only happy understands.
- **Two consumers hard-fail on frames that are standard today.** vibe-kanban's pinned schema
  knows 8 discriminators against v1's 11; `usage_update` and `config_option_update` fail
  deserialization in its transport before application code sees them. Happy's zod union is
  closed at 11 and will reject the unstable `plan_update` / `plan_removed`.
- **`terminal/*` is implemented by none of the four**, so an agent needing terminal
  delegation has zero consumers. `fs/*` has one, conditionally.
- **AionCore contains a second, divergent ACP client** (`aionui-ai-agent/src/protocol/acp.rs`,
  on crate 2.0.0) which *does* advertise `terminal: true`. One repo, two client surfaces,
  same protocol version.

So "one surface, many consumers" survives, but only for a deliberately minimal agent:
`initialize` + `session/new` + `session/prompt` + `session/cancel`, emitting the four-variant
core, calling back only `session/request_permission`, requiring neither `fs/*` nor
`terminal/*`. Emitting `usage_update` — standard, useful, and the obvious home for what this
project already measures — is on its own enough to break vibe-kanban today.

---

## Measurement 2 — `msg.Event` against ACP `session/update`

Direction matters and the todo states it correctly: **llm-bridge would be the ACP *agent***,
the process being driven. So the question is whether `msg.Event` can be rendered as ACP
`session/update`, not the reverse.

ACP v1 has **11** `session/update` variants. This repo has **19** `msg.EventType` values
(`msg/provider.go:170-205`, including the deprecated `agent_state`). The mapping:

| `msg.Event.Type` | ACP v1 carrier | fidelity |
|---|---|---|
| `user_message` | `user_message_chunk` | clean for text |
| `stream` | `agent_message_chunk` | clean for text deltas |
| `block` | `agent_message_chunk` | **lossy** — 13 block types into 5 |
| `thinking` | `agent_thought_chunk` | near-clean; `Subtype:"summary"` has no home |
| `tool_call` | `tool_call` | near-clean; `kind` has no source field (below) |
| `tool_result` | `tool_call_update` | lossy; `IsError` survives only as `status:"failed"` |
| `plan` | `plan` | **cannot be emitted conformantly** (below) |
| `session_info` | 4 different variants | 4 of 10 fields find a carrier |
| `usage_total` | `usage_update` | 2 of 8 usage fields survive (below) |
| `result` / `turn_complete` | the `session/prompt` response | `stopReason` is not derivable (below) |
| `session_state` | — | **no carrier. 13 values dropped.** |
| `error` | — | no carrier in the update stream |
| `system` | — | no carrier |
| `approval` | — | direction inverted (below) |
| `hook` | — | direction inverted (below) |
| `api_call` | — | no carrier |
| `api_spend_total` | — | no carrier |
| `agent_state` (deprecated) | — | no carrier |

**Nine of nineteen event types have a carrier at all. Ten have none.**

Cross the mapping with measurement 1 and the reach is narrower again. Restricting to the four
variants every consumer handles, the events that actually arrive at all four are `stream`,
`block`, `thinking`, `tool_call`, `tool_result` — **5 of 19**.

### The six findings behind that table

**1. `msg.PlanEvent` cannot produce a conformant ACP plan.** `PlanEvent` is
`{ Text string }` (`msg/event.go`) — flat prose. ACP's `PlanEntry` requires **all three** of
`content`, `priority` (`high|medium|low`) and `status` (`pending|in_progress|completed`);
verified against `$defs.PlanEntry.required`. Nothing in this repo holds a priority or a
per-entry status, so emitting a plan means inventing both. Note this is moot either way:
`plan` is handled by only two of the four consumers.

**2. `stopReason` has nothing to derive it from.** ACP's `StopReason` is a closed set of five
— `end_turn`, `max_tokens`, `max_turn_requests`, `refusal`, `cancelled` — and
`session/prompt` must return one. This repo's own `msg.StopReason` is a *different* closed set
of six (`end`, `max_tokens`, `tool_use`, `stop_sequence`, `safety`, `content_filter`), only
`max_tokens` is common to both, and — the sharper problem — **it does not live on the harness
event path at all.** `grep StopReason msg/event.go` is empty; it sits on `msg.StreamEvent`
(`msg/stream.go:84`), which is the provider-bridge path. An ACP agent built over
`HarnessSession` has `ResultEvent.IsError` and nothing else.

**3. ACP v1 cannot carry this project's usage numbers.** `UsageUpdate` is
`{ used, size, cost? }` — a context-window gauge plus cumulative money. `msg.TokenUsage` has
eight fields. Only `ContextTokens → used` and `ContextLimit → size` map; `InputTokens`,
`OutputTokens`, `TotalTokens`, `CacheReadTokens`, `CacheWriteTokens` and `ReasoningTokens`
have nowhere to go, and `msg.Cost`'s five fields collapse into one `{amount, currency}`.
Per-turn input/output tokens exist only as `PromptResponse.usage` in
`schema.unstable.json`, whose own description reads *"UNSTABLE — This capability is not part
of the spec yet, and may be removed or changed at any point."*

That is worth weighing against the open cost-accounting work: the cache-read and cache-write
counters that `0d052752` and `8754300f` exist to get right are exactly the fields ACP v1
drops.

**4. `session_state` is the largest single loss and it is this project's own subject.** ACP has
no session-state channel — none of the 11 variants carries one. The 13 `SessionState` values
(`msg/provider.go:219-241`: `starting`, `model_generating`, `tool_running`, `compacting`,
`awaiting_permission`, `awaiting_user`, `rate_limited`, `paused`, `idle`, `completed`,
`error`, `aborted`, `disconnected`) have no representation. Every consumer of an ACP surface
would infer liveness from the token stream, which is the thing bridge-ui stopped doing on
purpose.

**5. The permission direction is inverted, and this is a fork the todo does not currently
list.** ACP's `session/request_permission` is the **agent asking the client**. In this
project the decision already belongs to permission-store and the hook resolver inside
bridge-server, and `msg.ApprovalEvent` / `msg.HookEvent` are observations of a gate that has
already run. Being an ACP agent therefore means choosing one of:

  - keep deciding locally and never call `session/request_permission` — the one reverse
    method all four consumers implement goes unused, and their approval UI is dead; or
  - delegate to whichever ACP client is attached — which moves the decision out of
    permission-store, i.e. out of the single source of truth for it.

  Neither is obviously right, both are the owner's, and it is a third decision beside the four
  the todo already lists.

**6. `ContentBlock` is 13 into 5.** This repo has `text`, `image`, `audio`, `video`,
`document`, `tool_use`, `tool_result`, `thinking`, `redacted_thinking`, `code_exec`,
`code_exec_result`, `server_tool_result`, `refusal` (`msg/content.go:12-24`). ACP has `text`,
`image`, `audio`, `resource_link`, `resource`. Six have no analogue; and on the inbound side
`image`/`audio`/`resource` each need an explicit `promptCapabilities` opt-in, since the
baseline is `text` + `resource_link` only.

### What maps *well*, which is worth saying

The `tool_call` / `tool_call_update` pair is a genuinely good fit: `ToolCall` requires only
`toolCallId` and `title`, patch semantics on the update are field-level replace, and
`rawInput`/`rawOutput` accept arbitrary JSON — so `ToolCallEvent.Input` and
`ToolResultEvent.Output` cross intact. The only invention needed is `kind`, a closed set of
ten (`read`, `edit`, `delete`, `move`, `search`, `execute`, `think`, `fetch`, `switch_mode`,
`other`) that no field in `msg.ToolCallEvent` supplies; `other` is a legitimate answer for all
of them.

`msg.SessionInfo` also lines up better than expected: `SlashCommands` →
`available_commands_update`, `PermissionMode` → `current_mode_update`, `Model` →
`config_option_update` with `category: "model"`. But per measurement 1 those three variants
reach two of four consumers each, and none of the three reaches vibe-kanban.

---

## What this does and does not settle

**Settled, and it needs no further measurement:**

- The four consumers agree on the version integer and on a four-variant core, and disagree on
  everything past it. ⛔ Do not re-derive the per-repo table — it is above with paths.
- Nine of nineteen `msg.Event` types have an ACP v1 carrier; five reach all four consumers.
- `plan` is unemittable from `msg.PlanEvent`; `stopReason` is underivable from the harness
  path; `session_state` and the cache-token counters have no carrier at all.

**Not settled, and deliberately untouched** — the four judgements in `48504ba6` (does
llm-bridge expose ACP at all; where it lives; is it worth it when two consumers are
competitors), plus the permission-direction fork this measurement added as a fifth.

One framing correction the numbers suggest, offered as framing and not as an answer: the
strategic pitch was *"one surface, many consumers"*, priced against per-target integration
PRs of 200–4,000 lines. What crosses an ACP surface to all four is a text, thought and tool
stream. What does not cross is normalized session state, per-turn and cache token accounting,
spend, hooks, subagent demux and session lineage — which is most of what this project adds
over driving a harness directly. Whether that is the right trade is exactly the judgement
being reserved.

---

## Correction shipped alongside this document

`msg/event.go` named an ACP method that does not exist. Two comments (the `HookSourceUserInput`
constant and the `HookEvent` doc block) cited *"ACP request_user_input"* as the analogue of
Claude Code's `AskUserQuestion`. Verified first-hand: `request_user_input` appears in neither
`schema/v1/schema.json` nor `schema.unstable.json`. The nearest real method is
`elicitation/create`, which is capability-gated behind `elicitation.form` / `elicitation.url`
— and which **none of the four consumers implements**. The comments now name the real method
and say so.
