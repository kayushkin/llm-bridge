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

**Second PR — `BloopAI/vibe-kanban`.**
Tier B. Tagline is literally "any coding agent" — value-prop fits without explanation. 538 PRs merged in 90 days = high throughput, fast reviews, low ceremony. Risk: that velocity also means our PR could land underbaked; we should ship it polished.

**Backup / third — `smtg-ai/claude-squad`.**
Tier B, small team (14 contributors), careful review (9 PRs merged/90d). Exact use-case match — they already maintain Claude Code + Codex + OpenCode drivers. If either of the above drags, swap this in. Strong candidate for the highest-quality conversation but slowest cycle.

## License notes

llm-bridge is **Apache-2.0**. Compatibility with the three locked targets:

- **ccusage (MIT)** — fully compatible. Note: GitHub's license classifier shows "Other" but the LICENSE file is standard MIT.
- **vibe-kanban (Apache-2.0)** — same as ours. Trivial.
- **claude-squad (AGPL-3.0)** — fine for what we're doing. Our PR contribution becomes AGPL (theirs to keep). llm-bridge stays Apache-2.0 because they're consuming it as a network service / library, not the other way around. Two cautions: (a) never paste claude-squad code into our Apache-2.0 repos; (b) check their CONTRIBUTING.md for a CLA before opening a PR.

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
