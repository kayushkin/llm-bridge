# @kayushkin/llm-bridge-types

TypeScript type definitions for [llm-bridge](https://github.com/kayushkin/llm-bridge), a unified interface for AI coding agents.

These types are auto-generated from the canonical Go types in `github.com/kayushkin/llm-bridge/msg` via [tygo](https://github.com/gzuidhof/tygo). The Go package is the source of truth; do not edit `msg.ts` by hand.

## Install

```bash
npm install @kayushkin/llm-bridge-types
```

## Usage

```ts
import type { Event, Conversation, Credential, Harness } from '@kayushkin/llm-bridge-types'

function handleEvent(ev: Event) {
  if (ev.user_message) {
    console.log('user said:', ev.user_message.text)
  } else if (ev.result) {
    console.log('agent finished:', ev.result.text)
  }
}
```

The package ships raw `.ts` (not pre-compiled `.d.ts`), so it works directly with TypeScript projects. If you need pre-compiled declarations, run your own `tsc` against `node_modules/@kayushkin/llm-bridge-types/msg.ts`.

## What's in here

- `Event` — the canonical SSE event type emitted by `llm-bridge-server`
- `Conversation`, `Message`, `Block` — message-history shapes
- `Credential`, `Harness`, `Provider` — registry/identity types
- `CreateSessionRequest`, `SendMessageRequest`, `ManagedSession` — server API shapes
- `HarnessInfo`, `Capabilities` — runtime discovery shapes

See [`for-integrators.md`](https://github.com/kayushkin/llm-bridge/blob/main/for-integrators.md) in the parent repo for end-to-end integration guidance.

## Versioning

Types track the Go module version of `github.com/kayushkin/llm-bridge`. Breaking changes to the Go types follow semver and are reflected in this package's major version.

## License

Apache 2.0 — see [LICENSE](./LICENSE).
