# Event / Outbox Flow

This note captures the current outbox-backed async path and its operational semantics. It complements [project/tars_technical_design.md](/Users/yue/TARS/project/tars_technical_design.md) and focuses on what the runtime actually does today.

## Contract Shape

The event bus contract lives in [internal/contracts/event_bus.go](/Users/yue/TARS/internal/contracts/event_bus.go:15). The key types are:

- `EventPublishRequest` for producers
- `EventEnvelope` for claimed work items
- `DeliveryPolicy` and `DeliveryResult` for retry/ack/dead-letter decisions

Default topic policies are currently:

- `session.analyze_requested`: 1 attempt
- `session.closed`: 1 attempt
- `telegram.send`: 3 attempts with backoff of `1s` then `5s`

## Producer Side

Outbox rows are inserted in the same transaction as the session/workflow change that created them. The Postgres store writes to `outbox_events` through `PublishEvent` and helper paths like `EnqueueNotifications` and `publishEventTx` in [internal/repo/postgres/workflow.go](/Users/yue/TARS/internal/repo/postgres/workflow.go:1031) and [internal/repo/postgres/workflow.go](/Users/yue/TARS/internal/repo/postgres/workflow.go:1872).

The table shape is defined in [internal/repo/postgres/schema.go](/Users/yue/TARS/internal/repo/postgres/schema.go:183). The important columns are `topic`, `aggregate_id`, `payload`, `status`, `available_at`, `retry_count`, `last_error`, and `blocked_reason`.

## Claim / Dispatch Loop

The dispatcher is started from [internal/app/workers.go](/Users/yue/TARS/internal/app/workers.go:14). It creates one outbox dispatcher, one approval-timeout worker, and the other background loops.

The dispatcher behavior is:

1. On start, call `RecoverPendingEvents` to move stuck `processing` rows back into a recoverable state.
2. Repeatedly run a polling loop every `200ms`.
3. Claim up to 10 pending rows with `ClaimEvents`.
4. Dispatch each envelope by topic.
5. Resolve each row with `ack`, `retry`, or `dead_letter`.

Claiming uses `FOR UPDATE SKIP LOCKED` on rows whose `status = 'pending'` and `available_at <= NOW()`, so multiple workers can coexist without double-claiming the same row. See [internal/repo/postgres/workflow.go](/Users/yue/TARS/internal/repo/postgres/workflow.go:992) and [internal/events/dispatcher.go](/Users/yue/TARS/internal/events/dispatcher.go:89).

## Resolution Semantics

Resolution is centralized in `ResolveEvent`:

- `ack` sets `status = 'done'`
- `retry` sets `status = 'pending'`, increments `retry_count`, and moves `available_at` forward
- `dead_letter` sets `status = 'failed'`

The compatibility helpers `CompleteOutbox` and `MarkOutboxFailed` both route through that same resolver in [internal/repo/postgres/workflow.go](/Users/yue/TARS/internal/repo/postgres/workflow.go:1405).

The dispatcher currently handles three topic families in [internal/events/dispatcher.go](/Users/yue/TARS/internal/events/dispatcher.go:168):

- `session.analyze_requested`
- `session.closed`
- `telegram.send`

Unknown topics are treated as dead letters.

## Ops Surface

Failed or blocked rows are inspectable through the ops routes registered in [internal/api/http/routes.go](/Users/yue/TARS/internal/api/http/routes.go:97). The guard and token behavior for that surface live in [internal/api/http/ops_support.go](/Users/yue/TARS/internal/api/http/ops_support.go:16).

The practical rule for operators is:

- `blocked` means the row is intentionally held back, usually by feature-flag or business gating.
- `failed` means delivery exhausted the topic policy and now needs manual replay or cleanup.
- replaying or deleting should happen through the Ops API, not by editing the table directly.

## Files To Read First

- [internal/contracts/event_bus.go](/Users/yue/TARS/internal/contracts/event_bus.go:15)
- [internal/repo/postgres/schema.go](/Users/yue/TARS/internal/repo/postgres/schema.go:183)
- [internal/repo/postgres/workflow.go](/Users/yue/TARS/internal/repo/postgres/workflow.go:992)
- [internal/repo/postgres/workflow.go](/Users/yue/TARS/internal/repo/postgres/workflow.go:1405)
- [internal/repo/postgres/workflow.go](/Users/yue/TARS/internal/repo/postgres/workflow.go:1872)
- [internal/events/dispatcher.go](/Users/yue/TARS/internal/events/dispatcher.go:89)
- [internal/app/workers.go](/Users/yue/TARS/internal/app/workers.go:14)
- [internal/api/http/routes.go](/Users/yue/TARS/internal/api/http/routes.go:97)
