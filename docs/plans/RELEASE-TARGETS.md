# llm-bridge Release Targets

Working doc — strategy for getting llm-bridge in front of early adopters once the OSS audit (see `open-source-prep` noteboard tag) lands.

## Goal

Land 2–3 sample integrations in third-party OSS projects to:
1. Prove the abstraction holds against code we don't control.
2. Generate user-report-driven bug fixes / API revisions before we lock anything down.
3. Seed visibility — every merged PR is a backlink and a referral channel.

We are **not** chasing the largest projects first. They have entrenched architectures and slow review queues. We want active, smaller maintainers who feel the pain llm-bridge solves.

## What llm-bridge actually offers an integrator

The pitch, four bullets:

- **One event shape across agents** — `msg.Event` is identical whether the user is running Claude Code, Codex, Aider, or Goose. No per-agent parsing.
- **Session lifecycle for free** — start / stop / resume / fork / compact / interrupt over an HTTP+SSE API. No subprocess plumbing.
- **No lock-in** — every component is optional. Use just `msg` types, or just one harness, or the full server.
- **Forward-compatible by design** — `Overflow` map preserves unknown fields; new agent capabilities don't break old consumers.

The integration sell is *not* "rip out your Claude Code code." It's "add llm-bridge-server as a backend, get multi-agent for free later, simplify your event parsing now."

## Targets by tier

### Tier A — single-agent wrappers (easiest sells)

These projects today shell out to `claude` directly, parse `~/.claude/projects/*.jsonl`, or scrape stdout. llm-bridge replaces fragile parsing with a structured SSE stream and gives them a path to multi-agent without rewriting their UI.

Candidate categories (verify current maintainer + activity per project before reaching out):

- **Claude Code usage trackers** (e.g. ccusage, ccseva, claude-monitor). Today they tail JSONL. With llm-bridge they subscribe to canonical `usage` events, become "agent usage trackers" overnight.
- **Claude Code GUIs** (e.g. opcode/claudia, conductor, happy). Today they spawn `claude` and parse stream-json. With llm-bridge they hit one HTTP endpoint and can offer a backend dropdown.
- **Claude Code launchers / session managers** (e.g. claunch, ccmanager-like tools). Smallest surface area; quickest to integrate.

### Tier B — multi-agent dashboards (highest value, medium friction)

These already feel the pain — they maintain N agent-specific drivers. llm-bridge collapses those drivers into one.

- **claude-squad** style tools that already wrap Aider / Codex / Cursor / Gemini side-by-side.
- TUI multiplexers / orchestrators that today shell out to whichever CLI the user picks.
- IDE extensions targeting "any coding agent."

The PR here is bigger but the value-prop is unmistakable. Wait until at least one Tier A integration is shipping cleanly before going here.

### Tier C — agents themselves (defer)

continue.dev, plandex, aider, etc. **They are the agents**, not consumers of agents. Any integration here is them implementing a `HarnessBridge` for themselves — that's a partnership, not a drive-by PR. Defer until we have referrals or a maintainer reaches out.

### Tier D — orchestration frameworks (defer)

crewAI / autogen / langchain-style frameworks could spawn coding agents via llm-bridge, but the abstraction mismatch is large and the audience is mostly Python. Revisit once the Python types package has been validated by a real consumer.

## Live candidates (data as of 2026-04-26)

Source: GitHub API. Stale projects (no push in 5+ months) and PR-overwhelmed projects (>800 open issues with low merge velocity) excluded.

| Repo | License | ★ | Contribs | Commits/90d | PRs merged/90d | Last push | Open issues | What they do | Tier |
|---|---|---:|---:|---:|---:|---|---:|---|---|
| ryoppippi/ccusage | MIT | 13,354 | 56 | 35 | 11 | 2026-04-26 | 181 | CLI: parses local Claude Code/Codex JSONL for usage | **A** |
| smtg-ai/claude-squad | AGPL-3.0 | 7,171 | 14 | 12 | 9 | 2026-03-28 | 51 | Manage multiple Claude Code/Codex/OpenCode terminal agents | **B** |
| slopus/happy | (verify) | 19,195 | — | — | — | 2026-04-26 | 634 | Mobile/web client for Codex + Claude Code, voice, encrypted | **B** |
| BloopAI/vibe-kanban | Apache-2.0 | 25,561 | 61 | 583 | 538 | 2026-04-24 | 518 | Kanban for "any coding agent" | **B** |
| davila7/claude-code-templates | (verify) | 25,539 | 66 | 344 | 104 | 2026-04-26 | 144 | CLI: configure + monitor Claude Code | **A/B** |
| iOfficeAI/AionUi | (verify) | 22,613 | 81 | 2,904 | 971 | 2026-04-26 | 389 | Multi-agent GUI for Gemini CLI / Claude Code / Codex | **B** |
| farion1231/cc-switch | (verify) | 51,866 | 99 | 579 | 128 | 2026-04-26 | 619 | Desktop All-in-One for Claude Code / Codex / OpenCode | **B** |
| charmbracelet/crush | (verify) | 23,516 | 111 | 616 | 285 | 2026-04-26 | 406 | Charm's "agentic coding for all" TUI | **C** (opinionated; defer) |

Excluded (logged here for reference):

- **winfunc/opcode** — 21k★ but last push 2025-10-16 (>6 months stale).
- **Maciek-roboblog/Claude-Code-Usage-Monitor** — 7.8k★ but last push 2025-09-14.
- **musistudio/claude-code-router** — 33k★ but 902 open issues; review backlog likely too deep.
- **anthropics/claude-code, openai/codex, google-gemini/gemini-cli** — Tier C; they *are* the agents llm-bridge wraps.

## Locked recommendations

**First PR — `ryoppippi/ccusage`.**
Tier A. They parse local JSONL today; swapping that to an SSE subscription on llm-bridge-server is the textbook demo. Modest PR throughput (~11/90d) signals careful review (good for the first integration). Risk: 56 contributors and 181 open issues means maintainer attention is divided; keep the PR small.

> ⚠️ **Every factual claim in the paragraph above is out of date. Read "Re-validation 2026-08-07" below before acting on it.** The repo moved org, was rewritten in Rust, built the adapter abstraction this pitch was going to propose, and now auto-closes issues from new contributors.

**Second PR — `BloopAI/vibe-kanban`.**
Tier B. Tagline is literally "any coding agent" — value-prop fits without explanation. 538 PRs merged in 90 days = high throughput, fast reviews, low ceremony. Risk: that velocity also means our PR could land underbaked; we should ship it polished.

> ⛔ **This target is DEAD. Do not act on the paragraph above.** vibe-kanban merged its own shutdown on 2026-04-24 and has merged **0 PRs in the 90 days since**. See "Re-validation 2026-08-07 — Tier B" below for the measurement and for the replacement shortlist.

**Backup / third — `smtg-ai/claude-squad`.**
Tier B, small team (14 contributors), careful review (9 PRs merged/90d). Exact use-case match — they already maintain Claude Code + Codex + OpenCode drivers. If either of the above drags, swap this in. Strong candidate for the highest-quality conversation but slowest cycle.

> ⛔ **Ruled out on architecture, twice (2026-06-04, re-confirmed 2026-08-07).** They do not maintain drivers — an agent is a shell string, and agent state is `strings.Contains` over a `tmux capture-pane` screen-scrape. There is nothing for `msg.Event` to map onto. Also carries a CLA granting irrevocable relicensing rights. See below.

## Re-validation 2026-08-07 — ccusage (Tier A #1)

Measured against the live GitHub API on 2026-08-07 by the nightly worker, because the
April snapshot above was 3.5 months old and the sibling Tier B target (vibe-kanban) had
already died under re-validation. **ccusage is the opposite case: very much alive, and
every particular of our plan is stale.**

| claim (April) | measured 2026-08-07 |
|---|---|
| repo is `ryoppippi/ccusage` | moved to its own org, **`ccusage/ccusage`** (old path redirects) |
| 13,354 ★ | **17,782 ★** |
| last push 2026-04-26 | pushed **today**; releases still flowing (v20.0.19) |
| "parses local Claude Code/Codex JSONL", TypeScript | **rewritten in Rust.** `apps/ccusage/src` is now `cli.js` alone — a launcher for a prebuilt native binary shipped as six per-platform npm packages |
| companion packages `apps/{codex,opencode,amp,pi}` | **removed.** CONTRIBUTING.md: *"Standalone wrapper packages such as `ccusage-codex`, `ccusage-opencode`, `ccusage-amp`, and `ccusage-pi` have been removed and should not be reintroduced."* |
| no CONTRIBUTING.md | **exists**, and it is the whole story — see below |
| 4 per-tool data-loaders | **17 adapter crates** under `rust/adapters/`, plus a shared `common/` |

### The pitch's premise is refuted, not merely stale

Our value proposition was "replace N per-tool drivers with one llm-bridge backend, and
future agents land for free." In the 3.5 months since we wrote that, ccusage went from 4
per-tool loaders to **17**, wrote `rust/adapters/README.md` documenting the shape of an
adapter crate, and factored the shared work into `ccusage-adapter-common`. They built the
abstraction themselves and they scale it fine. Arguing they need ours is arguing against
measured evidence.

Worse, their adapter list already overlaps ours heavily — `claude`, `codex`, `gemini`,
`goose`, `hermes`, `kilo`, `openclaw`, `opencode`, `copilot`, `qwen`, `kimi`, `droid`,
`amp`, `pi`, `codebuff`. Several are agents llm-bridge bridges. We are not offering
coverage they lack.

### What we would actually be offering, if anything

One thing, and it is narrow but real: **ccusage reads logs after the fact; llm-bridge
already holds a normalised, cross-agent record.** `~/.config/log-store/events.db` on this
host is a live 3.4 GB SQLite database of canonical `msg.Event` rows carrying model, token
counts and cost per session, across every harness. That is a *source*, in exactly the
sense their adapters mean the word.

⚠️ **And it must be pitched as a local file, not as SSE.** Every adapter they have resolves
a local directory with an env-var override (`OPENCLAW_DIR`, `paths.rs`), and PR #1487 sells
"decoded fully offline from the local SQLite databases" as a feature. An adapter that
subscribes to `LLM_BRIDGE_SERVER_URL` over HTTP+SSE — which is what our todo and draft both
specify — runs against the grain of all 17. An adapter that reads log-store's SQLite with an
`LLM_BRIDGE_DIR` override lands in their pattern exactly.

### The gate that actually decides this

CONTRIBUTING.md installs a formal two-stage approval, borrowed from `earendil-works/pi`:

- **Issues and PRs from new contributors are auto-closed by a bot**, immediately. Both
  sampled precedents (#1325, #1486) were auto-closed within a day.
- A maintainer replying **`lgtmi`** stops your future *issues* being auto-closed.
- A maintainer replying **`lgtm`** stops your future *PRs* being auto-closed.
- *"Do not open a PR unless you have already been approved with `lgtm`."*

So "maintainer ack" is no longer a vague hope — it is a specific token we must be granted,
and the only route to it is one issue that survives review after being auto-closed.

**Selection criterion 2 in this document — "accepts external PRs in practice" — now fails
on measurement.** Every non-bot PR merged in the sampled window (2026-07-29 → 2026-08-06)
was authored by `ryoppippi` himself; the rest are renovate/github-actions. When an outside
contributor asked for a new agent source (#1486), it was auto-closed and the maintainer
implemented it himself (#1544) — then reverted it (#1569). That is the observed path for a
new-source request, and it does not run through an external fork.

### Verdict

Not a "close this" like vibe-kanban — the project is healthy and the integration is still
*technically* sensible. But the plan of record (TypeScript companion package under `apps/`,
SSE subscription, discovery issue addressed to `@ryoppippi` on `ryoppippi/ccusage`) would
propose a retired pattern, in the wrong language, at a dead URL, to a bot that closes it.
The draft has been re-aimed in `.local/outreach/ccusage-discovery-issue.md`; posting is
still the owner's call, and it is now a single cheap action with a clear success signal
(`lgtmi`).

## Re-validation 2026-08-07 — Tier B (the #2 slot)

Measured against the live GitHub API and against each candidate's source on 2026-08-07 by
the nightly worker. Companion to the ccusage section above, run the same night. Full
evidence, per-repo, in noteboard note `fdd9ebd4-1417-45cd-99bf-0effc5fe587c`.

**ccusage landed "target alive, plan dead". This one lands the other way: the Tier B target
is genuinely dead, and the replacement had to be found.**

### The #2 slot was empty, and nobody had said so in this document

- `BloopAI/vibe-kanban` — **dead.** Last push 2026-04-24; **0 PRs merged in the 90 days
  since**; 151 open PRs stranded; last release 2026-04-24. Its final four commits were its
  own shutdown (`Sunset project routes to export-only page` #3387, `Add README sunsetting
  banner` #3388). Not archived, so a naive liveness check still passes. No successor — the
  whole `BloopAI` org's next-most-recent repo was pushed 2026-03-13.
- `smtg-ai/claude-squad` — the designated backup, **ruled out on architecture in June and
  re-confirmed unchanged today** against HEAD `2dd388e`. `net/http` appears zero times; an
  agent is a shell string; agent state is `(updated bool, hasPrompt bool)` from
  `strings.Contains` over `tmux capture-pane` output. Its `cs serve` RFC (PR #283) has been
  an untouched draft for 15 months — `updatedAt` is 11 seconds after `createdAt`.

⚠️ **vibe-kanban was the right target for the right reason, and that reason still holds.**
Its `crates/executors/` was a real per-agent executor abstraction — the exact seam an
additive backend slots into. The *fit* was never wrong; the company stopped. So the shape
to hunt for is unchanged, and the shortlist below is ranked on it.

### Candidates measured (merged-PR window 2026-05-09 → 2026-08-07)

| repo | ★ | merged/90d | authors | licence | verdict |
|---|---:|---:|---:|---|---|
| slopus/happy | 23,205 | 39 | 6 | MIT | **GOOD** |
| dexloom/vibe-kanban-indie | 51 | 12 | 2 | Apache-2.0 | **GOOD** |
| awslabs/cli-agent-orchestrator | 1,006 | 192 | 31 | Apache-2.0 | MARGINAL |
| iOfficeAI/AionUi (+ AionCore) | 31,653 | 504 | many | Apache-2.0 | POOR — competitor |
| omnigent-ai/omnigent | 8,250 | 2,450 | 18 | Apache-2.0 | ⛔ COMPETITOR |
| farion1231/cc-switch | 125,396 | 150 | many | MIT | POOR |
| smtg-ai/claude-squad | 8,247 | 8 | 6 | AGPL + CLA | POOR |
| BloopAI/vibe-kanban | 27,693 | **0** | 0 | Apache-2.0 | DEAD |

Excluded on licence or velocity: `golutra/golutra` (0 merges/90d, no detectable licence),
`dcouple/Pane` (no detectable licence), `Enderfga/claw-orchestrator` (3 merges/90d),
`mco-org/mco` (23 merges/90d, 21 by one author), `TechDufus/openkanban` (0 merges/90d),
`kagan-sh/kagan-legacy` (3 merges/90d, "legacy" in the name).

### ⛔ Two candidates are competitors, not targets — this is new information

**`omnigent-ai/omnigent` is Databricks** (`pyproject.toml:12-14`; `.github/MAINTAINER`
lists Matei Zaharia). In eight weeks it has built 1.06M lines of Python containing an
`Executor` interface with **25 concrete drivers**, a canonical `ExecutorEvent` hierarchy,
the full lifecycle including cross-harness fork and live `switch-agent`, a declared
per-harness capability matrix, and a conformance probe suite. Its ACP adapter docstring
(`omnigent/inner/acp_executor.py:36-38`) states our thesis as their implementation:
*"This executor translates the ACP event stream into Omnigent `ExecutorEvent`s."*

**`iOfficeAI/AionCore`** (the Rust backend behind the AionUi frontend) is the same story at
385k lines: `SessionBackend` / `BackendConnection` traits, a `SessionEvent` enum, capability
negotiation, ~40 agents. Header comment: *"the only thing that crosses the seam is `Command`
down and `SessionEnvelope` up."*

Neither is a place to add a backend; both would nest a second normalization layer under an
existing one.

### 🔑 The finding that may outrank this whole document: ACP

Four independent audits converged on Agent Client Protocol without being asked:

- **omnigent** will drive any ACP-over-stdio process from a user's config file — **zero code
  in their repo** (`docs/AGENT_YAML_SPEC.md:172-190`).
- **happy** ships `happy acp -- <any-command>` — **zero upstream change**
  (`packages/happy-cli/src/agent/acp/acpAgentConfig.ts:17-53`).
- **AionCore** serves ~35 of ~40 agents through one generic ACP driver; adding one is a
  **43-line SQL migration**.
- **vibe-kanban-indie** routes Gemini/Copilot/Qwen through an `AcpAgentHarness` rather than
  scraping stdout.

**If llm-bridge exposed an ACP-over-stdio surface, three of those would be able to drive it
with no PR at all.** Compare the bespoke route's measured cost: ~1,700–2,900 lines across
24–34 files for an awslabs provider; 1.5k–4k Rust lines for an AionCore backend. This is a
product decision about what llm-bridge *is*, so it is filed as its own todo and is
deliberately not folded into the Tier B pick. The two are not exclusive.

⚠️ **Both things this section left unmeasured are now measured — see `ACP-SURFACE.md`
(2026-08-07), and read it before pricing this.** Two results narrow the claim above. The four
consumers agree on the protocol version integer and on a **four-variant core**
(`agent_message_chunk`, `agent_thought_chunk`, `tool_call`, `tool_call_update`) and disagree
past it — two of them reject frames that are standard v1 today. And of this repo's 19
`msg.EventType` values, **nine have an ACP v1 carrier and five reach all four consumers**;
`session_state`, the cache-token counters, `stopReason` and `msg.PlanEvent` have none. The
surface is real, but what crosses it is a text/thought/tool stream, not the normalization
this project is built on. Nothing about the decision itself was answered.

### Recommendation — the pick is the owner's

**Option 1 — `slopus/happy`.** Best governance measured anywhere here: no CLA, no DCO,
**86.2% of all 130 merged PRs externally authored** across 56 authors, 2-day median merge,
and entire agent backends contributed by outsiders (#1430, #376). A maintainer (`ex3ndr`,
top contributor, 847 commits) has already invited exactly this on **issue #217**: *"We are
pretty much happy to accept if someone will contribute this!"* There is a template to copy —
`OpenClawBackend` (`src/openclaw/OpenClawBackend.ts:57`) is already a remote-gateway,
non-subprocess driver behind `--gateway-url` flags, which is precisely our shape. Its E2E
encryption turns out to be orthogonal: it applies on the client↔server axis only, so an
agent-side driver is crypto-neutral. Serves goals #2 and #3.
*Risks:* the envelope we would map onto is marked *"UNDER REVIEW… frozen. Do not add new
consumers"*; `AgentRegistry` is dead code so registration is a ~10-file shotgun edit; 366
open PRs behind one maintainer who merges 83% of everything.

**Option 2 — `dexloom/vibe-kanban-indie`.** A deliberate post-sunset continuation of
vibe-kanban (forked 2026-05-13 from upstream's final commit; 315 commits since; new TUI,
macOS app and Telegram bridge; 26 npm releases, latest 2026-08-05). It carries the
`StandardCodingAgentExecutor` seam **and has extended it twice** — `ClaudeCodeHeaded`
(`dffe1165`) and `OpencodeHeaded` (`4e387576`) are new executors added by the fork. There is
a protocol-client precedent to copy (`AcpAgentHarness`) and a normalized `NormalizedEntry`
type to map onto. **200–350 LOC**, the smallest PR of any candidate. Apache-2.0, same as
ours. Serves goal #1 at the lowest cost.
*Risks:* 51 stars, so goal #3 is barely served; **issues are disabled**, so step 1 of the
outreach playbook below (open a discovery issue first) is impossible here — contact must be
a direct PR or email; four of five recent PRs merged with no review.

They are not exclusive: Option 2 is cheap enough to be the proof-of-concept that de-risks
Option 1.

**`awslabs/cli-agent-orchestrator` is a genuine maybe-later.** Green governance (no CLA, no
samples-only notice, three outsiders have merged whole providers) and a real `BaseProvider`
ABC with nine implementations — but its data contract is a 6-value `TerminalStatus` enum
plus a string, scraped by regex from a tmux pane. `grep` for
`stream-json|jsonl|json.loads|--output-format` across `providers/` returns zero hits; it
drives even Hermes, an HTTP/SSE agent, by scraping its terminal. We would have to render
`msg.Event` into a fake TUI for them to regex back out. If ever pursued, the honest framing
is "add a structured-output path to `BaseProvider`", arguing from the existing
`_resolve_native_status()` hook — a design conversation, not a drive-by PR.

## License notes

llm-bridge is **Apache-2.0**. Compatibility with the three locked targets:

- **ccusage (MIT)** — fully compatible. Note: GitHub's license classifier shows "Other" but the LICENSE file is standard MIT.
- **vibe-kanban (Apache-2.0)** — same as ours. Trivial.
- **claude-squad (AGPL-3.0)** — fine for what we're doing. Our PR contribution becomes AGPL (theirs to keep). llm-bridge stays Apache-2.0 because they're consuming it as a network service / library, not the other way around. Two cautions: (a) never paste claude-squad code into our Apache-2.0 repos; (b) check their CONTRIBUTING.md for a CLA before opening a PR.

  > ⚠️ **Caution (b) resolved on 2026-08-07, and the answer is worse than "there is a CLA".** `CLA.md` grants the maintainers a **perpetual, irrevocable** licence to relicense contributions "under any license terms", with an explicit Relicensing Rights clause, enforced automatically by `contributor-assistant` on `pull_request_target`. Contributing there means signing away any say in how the contribution is later licensed. Moot while the target is ruled out on architecture, but it is the kind of term that should be read before any future AGPL target too.

## Selection criteria (for posterity / future waves)

1. Active in last 60 days (recent commits + recent merged PRs).
2. Maintainer reachable (responds on issues; has CONTRIBUTING.md or accepts external PRs in practice).
3. Single-agent today with multi-agent ambition, **or** already multi-agent and maintaining N drivers.
4. Small enough that one focused PR is reviewable in one sitting (<500 LOC of integration glue).

## Sample integration recipe (additive, not replacement)

This is the pattern every starter PR should follow. **Open question for the user:** confirm additive is the right framing, or do we want some integrations to be full replacements?

**Additive integration (recommended):**

1. Add a config option / env var: `LLM_BRIDGE_SERVER_URL` (or a CLI flag).
2. When unset → existing code path runs unchanged. Zero behavior change for current users.
3. When set → app subscribes to `GET /sessions/{id}/events` SSE on the bridge server, maps `msg.Event` into the project's existing event/UI types.
4. Document the new option in the project's README under "Experimental / advanced."
5. Keep the diff small. If the project has no abstraction layer, introduce the *thinnest possible* one — one interface, two implementations.

The goal of the first PR is **not** to make llm-bridge the default. It's to land a working code path that downstream users can opt into and report bugs against.

**What ships in the PR body:**

- One-paragraph "what is llm-bridge" with a link to the main repo.
- One-paragraph "why opt in" framed in terms of *their users'* benefit, not ours.
- Screencast or terminal recording of the new path running.
- Explicit "this is opt-in, no default behavior changes."
- Link to a discussion issue (which we open *first* — see playbook).

## Outreach playbook

For each target, sequence:

1. **Discovery issue (we open).** Title: "Pluggable agent backend via llm-bridge?" Body: 3 sentences on what llm-bridge is, what additive integration would look like in their codebase, ask for maintainer's posture before we invest. Goal: avoid the rejected-PR scenario where we waste a week.
2. **Wait for ack** (1 week, then nudge once). If no response or hard "no" → drop and move to next target.
3. **Branch + draft PR on a fork.** Build the sample integration. Run their test suite. Don't open the PR yet.
4. **Open the PR with the recording.** Reference the discovery issue. Tag the maintainer who acked.
5. **Be present for review.** Same-day responses for the first 72h after opening. Iterate fast.

## Contacts

Per-project maintainer info changes; this section is intentionally empty. Each starter-target todo includes a "verify current maintainer + preferred contact channel" step to be done at the moment of outreach, not earlier.

## Decisions locked (2026-04-26)

1. **Additive integration pattern** is the default for the first wave. Replacement PRs only on maintainer request, not from us.
2. **First two targets:** `ryoppippi/ccusage` (#1), `BloopAI/vibe-kanban` (#2). Backup: `smtg-ai/claude-squad`.
3. **Issues + PRs signed by user (kayushkin GitHub identity).** No bot account.
4. **Integrators guide** lives at [`for-integrators.md`](for-integrators.md) in this repo. Linked from every discovery issue + PR.

## Status

- Doc created: 2026-04-26
- Bridge ecosystem README audits: in progress (`open-source-prep` tag in noteboard)
- Integrators guide: drafted at `for-integrators.md`
- First outreach: blocked on README audits + reusable integration sample
