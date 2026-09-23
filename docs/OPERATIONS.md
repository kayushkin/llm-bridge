# Operations and receipts

An operation is asynchronous work a caller asks the bridge to do. The caller
sends an `OperationIntent` and gets an `OperationReceipt` back at once; the
receipt then moves to a terminal state, and the caller follows it by reading it
or by streaming its events. The Go types in `msg/operation.go` are the source
of truth, and `ts/msg.ts` is generated from them. llm-bridge-server runs
operations; this document is the contract a caller and an executor can rely on.

## Routes (llm-bridge-server)

| Route | What it does |
| --- | --- |
| `POST /operations` | Accept an intent. `202` and a new receipt, or `200` and the first receipt for a repeated intent. |
| `GET /operations/{id}` | The receipt. |
| `GET /operations/{id}/events` | Server-sent events: every stored event, then new ones as they happen. The stream closes once the operation and all its children have finished. An event's SSE id is its sequence, which equals the receipt revision; `Last-Event-ID` resumes after it. |
| `POST /operations/{id}/cancel` | Ask the operation to stop. |
| `GET /operations/{id}/children` | The receipts of its children. |
| `GET /operations` | Receipts, filtered by `organization_id`, `principal_id`, `type`, `state`, `created_after`, `created_before`. |
| `GET /operation-types` | The types this server can run. Needs no credential. |
| `GET /operation-budgets`, `GET`/`PUT`/`DELETE /operation-budgets/{organization_id}` | Organization monthly limits and this month's spend. Operators only. |

A principal sees only operations it started; an administrator and the internal
service see all of them. Someone else's operation is `404`, as a missing one is.
`organization_id` must be an active principal-store group, and a principal who
is not an administrator must be a member of it (`403
not_a_member_of_organization`).

The first event is `accepted`, with the receipt `queued`. `started` marks each
attempt, `progress` each change an executor makes, `queued` an attempt that
will be retried, `cancel_requested` a cancel that reached a running operation,
and `child_updated` a change to a child. The last is the event named for the
terminal state.

## States

```text
queued ──► running ──► succeeded | failed | conflicted | unknown | cancelled
   │          │
   │          └─► queued   (lease expired and the executor says a rerun is safe)
   └─► cancelled
```

The five states on the right are terminal: once there, a receipt's state,
result and error never change. `Revision` rises by one with every change, so a
reader keeps whichever copy has the higher revision.

- **failed** means the bridge knows what happened outside it: nothing, or the
  effects listed in `Effects` with outcome `applied`.
- **conflicted** means the store that owns the target refused the change
  because the record had moved on. Nothing was applied.
- **unknown** means at least one effect has outcome `unknown`: the request left
  and no answer said whether it landed. The bridge never retries an unknown
  operation on its own. A person, or a later reconciliation, decides.
- **cancelled** means the caller asked it to stop. A queued operation is
  cancelled at once. A running one is told to stop and reaches `cancelled` when
  its executor returns; if its executor finishes first, it keeps that state.

## Who may start what

A principal needs a grant-store `can_run_operation` grant on the
`operation_type` it starts (the id is the type name, such as
`classification.run`); a grant to a group covers its members. The bridge's
`operations.grant_enforcement` setting decides what holding no such grant
means:

- `lenient` (the default): a principal holding no `can_run_operation` grant may
  start any type; one holding any may start only the types they name.
- `strict`: a principal may start only the types its grants name.

Administrators and the internal service are not checked. A refusal is `403
not_granted`; a grant-store that cannot answer is `502`.
`requested_capabilities` must be empty until capabilities are defined (`400
unknown_capability`).

## Budgets

Money is counted in US dollars at **list price**: each model call's tokens
times model-store's per-model input and output price, with cache reads and
writes charged at the input price. A call on a subscription login costs
nothing extra, so for those the figure is an upper bound. `usage.cost_basis`
says `list_price`.

Two limits apply, and the lower one wins:

- **Per organization per month.** An operator sets it with `PUT
  /operation-budgets/{organization_id} {"monthly_limit_usd": 50}`. Months are
  UTC calendar months. Once the month's spend reaches the limit, a new intent is
  `402 organization_budget_exhausted` (a repeat of an accepted one still gets
  its receipt).
- **Per operation.** `maximum_cost_usd` on the intent caps the operation and
  every child it starts.

An executor asks for the allowance before each model call and does not make one
when it is spent (`budget_exhausted`). A call may overshoot by its own cost;
the next one is refused. Under a budget, a call whose model has no price in
model-store is refused before it is made (`model_price_unknown`), and so is one
whose model is not known in advance (`budget_unenforceable`).

## llm.completion

One stateless model call through a bridge harness instance
(`operations.completion_instance`), the same one-shot path the bridge's own
classifier uses. The harness holds the login; nothing calls a provider API
directly. Input is `LLMCompletionInput`; a `schema` forces a JSON answer into
`parsed`. With no `model`, `operations.completion_model` is used. A failed call
is retried once.

## Idempotency

`idempotency_key` is required. The key is unique within one principal, one
organization and one operation type (the internal service acting on its own
counts as one principal).

- The same key with the same intent returns the first receipt, `200`, whatever
  state it is in now. Nothing runs twice.
- The same key with a different intent is `409 idempotency_key_reused`.
- "The same intent" compares `type`, `organization_id`, `scope`,
  `input_references`, `input`, `requested_capabilities` and `policy_revision`.
  `input` is compared after parsing, so key order and white space do not
  matter; `correlation_id` is not compared.
- Keys do not expire. To run the same work again, send a new key.

## Correlation

`correlation_id` is the caller's trace id. When empty the bridge sets it to the
operation id. Every call an executor makes to another store carries three
headers, so that store can log it and refuse a duplicate:

| Header | Value |
| --- | --- |
| `X-Correlation-Id` | the operation's correlation id |
| `X-Operation-Id` | the operation id |
| `Idempotency-Key` | `<operation id>:<effect number>` — stable across retries of one effect |

A child operation inherits its parent's correlation id and organization.

## Effects and unknown outcomes

Before an executor asks another system to change something, it writes an
`OperationEffect` with outcome `unknown`. When the answer comes it sets the
outcome to `applied` or `not_applied` and records the other system's id for the
request in `external_operation_id`. An effect still `unknown` when the attempt
ends is why an operation lands in `unknown` rather than `failed`. On restart the
bridge asks the executor to reconcile every operation whose lease expired; an
executor that can ask the other system about its effects may settle them.

## Redaction and retention

- An intent must not carry credentials. The bridge resolves credentials itself,
  from auth-store, when an executor needs one. The server refuses an `input`
  that has a key named like a credential (`password`, `api_key`, `secret`,
  `token`, `authorization`, and the like).
- Carry content by reference when another store owns it: a file id from
  file-store rather than the file's bytes, a mail message id rather than the
  body. `input` holds only what the operation needs and no store owns.
- `error.message` is written for a person and never quotes a credential, a
  request header or a whole provider response.
- `executor` and model details are diagnostics. A product surface may show them
  in an expandable panel, and must not branch on them.
- Operations, their events and their effects are kept until deleted by hand.
  There is no automatic expiry yet.
