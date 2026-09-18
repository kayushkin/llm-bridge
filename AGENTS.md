# About llm-bridge

## What it owns

Canonical message types (`msg/`) and bridge interfaces (`bridge/`). The lingua franca for the entire ecosystem — all harnesses and providers import this. It has no dependencies, so any service here can import it. **`servicesettings/` (since 2026-09-18) is how a service declares, reads, stores and describes its own configuration**: one `Definition` per setting (key, environment variable, kind — `wiring`, `path`, `secret`, `behaviour` — type, default, what it changes); `New` refuses a value that does not parse and a set variable under the service's own prefix that nothing declares; a behaviour setting declared `Editable` is stored by the service, seeded from the environment once and from then on read from the record; `Handler` serves `GET /settings` (`msg.ServiceSettings`, a secret as set-or-not only) and `PUT /settings/{key}` the same way everywhere — 404 unknown key, 409 not editable, 400 invalid, 502 when a validator could not ask the owner. bridge-ui's `/service-settings` page draws any service that serves it; llm-bridge-server is the first. **Give a service its settings this way, not with scattered `os.Getenv`.**

## Where this prompt lives

These sections are stored in agent-store as a project prompt collection and rendered, with identical text, to `AGENTS.md` and `CLAUDE.md` at the root of this repo, so that whichever file a harness reads it gets the same thing. Edit them on dash `/files`, or edit either rendered file: the 15-minute scan carries the edit back into the sections and out to the other file. The host prompt keeps one row for this repo with only what an agent elsewhere needs.
