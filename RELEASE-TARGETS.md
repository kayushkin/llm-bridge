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

## Recommended starter set

Pick **two Tier A projects** for the first wave. Selection criteria:

1. Active in the last 60 days (recent commits + open PR review activity).
2. Maintainer is reachable (visible on issues, has a contact channel, accepts external PRs — check CONTRIBUTING.md).
3. Single-agent today, multi-agent ambition implied or stated in their README/issues.
4. Small enough that one focused PR is reviewable in one sitting (<500 LOC of integration glue).

Concrete shortlist to validate (research todos below):

- **A Claude Code usage tracker** — likely best first target. Smallest blast radius; clear demo of "parse JSONL → subscribe to SSE." Concrete candidate to vet: ccusage. Fallback: any active Claude Code menubar/CLI tracker.
- **A Claude Code session/GUI tool** — second target, slightly bigger surface. Concrete candidate to vet: opcode (getAsterisk). Fallback: a smaller Tauri/Electron Claude Code wrapper.
- **A multi-agent terminal multiplexer** as a stretch third — only if review bandwidth allows. Concrete candidate to vet: claude-squad.

> **Validation required for every name above.** Maintainer activity and PR-acceptance posture change quickly; do not assume current state. See todos.

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

## Open questions for review

1. **Additive vs. replacement integration** — confirm the additive pattern as default, or mark some targets for full replacement.
2. **Which two starters** — the shortlist above is plausible but the user's market awareness is better than mine. Pick 2 by name.
3. **Who signs the PRs / opens the issues** — personal GitHub identity, a project bot, or both?
4. **Do we want a public "integrators guide"** in this repo before the first PR, or is the README + this doc enough?

## Status

- Doc created: 2026-04-26
- Bridge ecosystem README audits: in progress (`open-source-prep` tag in noteboard)
- First outreach: blocked on README audits + user's pick of two starters
