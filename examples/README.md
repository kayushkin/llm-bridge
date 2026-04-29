# Examples

Tiny reference consumers of the llm-bridge ecosystem. Each example is a
standalone, copy-pasteable starting point for the **additive integration**
pattern documented in [`../for-integrators.md`](../for-integrators.md):
your project subscribes to llm-bridge-server's SSE feed and renders
canonical `msg.Event` records, without embedding any harness bridge.

Downstream PRs adding an llm-bridge backend to existing tools (kanban
boards, terminal multiplexers, dashboards, CLIs, etc.) can link to one
of these as the canonical reference, so the same SSE-glue isn't
re-derived from scratch in every PR.

## Available samples

| Directory | Language | What it shows |
|-----------|----------|---------------|
| [`sse-tail/`](sse-tail) | Go | Smallest possible SSE consumer: parse frames, switch on `msg.Event.Type`, render. ~150 lines, no third-party deps. |

## Design rules

These examples are deliberately tiny:

- **One file each, where possible.** No frameworks, no cleverness.
- **Stdlib only.** Past `github.com/kayushkin/llm-bridge/msg`, no extra
  Go dependencies. Past `@kayushkin/llm-bridge-types`, no extra npm
  dependencies (when a TS sample lands).
- **Exhaustive type switch over the canonical events.** Including the
  server-derived convenience events (`agent_state`, `usage_total`,
  `turn_complete`). A consumer that handles all of them today is
  forward-compatible with new types via `msg.Event.Overflow`.
- **No session lifecycle ops.** These samples consume an existing
  session's events. Creating a session, sending messages, interrupting,
  resuming, forking — all of that is documented in the parent README's
  Quick start; the canonical "additive" pattern is purely consumer-side.

## Adding a new sample

If you want a TypeScript / Python / Rust / etc. equivalent, mirror the
`sse-tail/` shape: one source file, one README explaining run + sample
output + what to copy. Keep total LOC under ~200. Anything bigger
belongs in its own repo (e.g. `bridge-ui` for React) rather than here.
