# Pilot Core Flow

This note captures the current request/data path for the pilot core. It is intentionally short and complements the longer design doc in [project/tars_technical_design.md](/Users/yue/TARS/project/tars_technical_design.md).

## What Actually Boots

The launched process is a single Go HTTP server started from [cmd/tars/main.go](/Users/yue/TARS/cmd/tars/main.go:16). It builds one `App`, starts background workers, and binds one listener at `a.Config.Server.Listen` (default `:8081` from [internal/foundation/config/config.go](/Users/yue/TARS/internal/foundation/config/config.go:192)). The handler wired into that server is [App.Handler()](/Users/yue/TARS/internal/app/router.go:11), which registers both public routes and ops routes onto the same mux.

Bootstrap is split into three layers:

- [buildSharedBootstrap](/Users/yue/TARS/internal/app/bootstrap_shared.go:85) loads logger, metrics, observability, Postgres, vector store, and runtime config stores.
- [buildPilotCoreServices](/Users/yue/TARS/internal/app/bootstrap_core.go:27) wires the pilot loop: alert ingest, workflow, reasoning, action, and Telegram channel.
- [buildOptionalServices](/Users/yue/TARS/internal/app/bootstrap_optional.go:35) attaches knowledge, inbox, trigger, automation, templates, and other non-core managers.

## Core Request Flow

The pilot loop is centered on `Workflow`. A typical alert path is:

1. External request enters the HTTP mux through webhook or API routes registered in [internal/api/http/routes.go](/Users/yue/TARS/internal/api/http/routes.go:7).
2. `AlertIngest` normalizes the payload.
3. `Workflow` owns the session state transition.
4. `Reasoning` produces diagnosis or next-step suggestions.
5. `Action` performs metric queries, SSH execution, or capability invocation.
6. `Channel` sends user-facing messages, usually via Telegram.

The important boundary is that `Workflow` is the state owner. Other modules may read state or emit commands, but they should not directly mutate the session/execution tables.

## Data Ownership

- `Workflow` owns session, execution, approval, and outbox-related state transitions.
- `Reasoning` is a pure-ish analysis layer: it consumes context and returns diagnosis/plans.
- `Action` is the execution layer: it talks to VictoriaMetrics, SSH, JumpServer, and connector runtimes.
- `Channel` is the delivery layer: it sends messages and receives Telegram updates.
- Optional services such as knowledge, inbox, and automations are attached after the pilot core is already assembled.

## Operational Notes

- If `TARS_POSTGRES_DSN` is empty, the app still boots, but workflow state is in-memory and will not survive restarts. The bootstrap warning is emitted in [bootstrap_shared.go](/Users/yue/TARS/internal/app/bootstrap_shared.go:108).
- Shared/pilot deployments should set `TARS_RUNTIME_CONFIG_REQUIRE_POSTGRES=true`. With that flag enabled, missing `TARS_POSTGRES_DSN` is a startup error instead of a silent in-memory runtime config fallback.
- Runtime config documents now cover `access`, `providers`, `connectors`, `authorization`, `approval_routing`, `org`, `reasoning_prompts`, `desensitization`, `agent_roles`, and `setup_state`. Secrets remain outside normal runtime config documents; optional lifecycle-heavy modules such as automations, skills, and extensions keep their current file/state backing until a dedicated migration is justified.
- The pilot core remains usable even if optional modules are absent; the bootstrap tests explicitly check that the core and optional assemblies can be separated in [internal/app/bootstrap_test.go](/Users/yue/TARS/internal/app/bootstrap_test.go:1).

## Files To Read First

- [cmd/tars/main.go](/Users/yue/TARS/cmd/tars/main.go:16)
- [internal/app/bootstrap.go](/Users/yue/TARS/internal/app/bootstrap.go:69)
- [internal/app/bootstrap_shared.go](/Users/yue/TARS/internal/app/bootstrap_shared.go:85)
- [internal/app/bootstrap_core.go](/Users/yue/TARS/internal/app/bootstrap_core.go:27)
- [internal/app/bootstrap_optional.go](/Users/yue/TARS/internal/app/bootstrap_optional.go:35)
- [internal/app/router.go](/Users/yue/TARS/internal/app/router.go:11)
- [internal/api/http/routes.go](/Users/yue/TARS/internal/api/http/routes.go:7)
