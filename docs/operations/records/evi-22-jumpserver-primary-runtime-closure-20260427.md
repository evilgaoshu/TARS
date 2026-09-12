# EVI-22 JumpServer Primary Runtime Closure Record

- Issue: EVI-22
- Capture time (UTC): `2026-04-27 05:12:56 UTC`
- Shared environment: `192.168.3.100`
- Target asset: `192.168.3.9 dev`
- Branch: `agent/dev-opencode-gpt5-4/57d65e06`
- Local head commit before commit: `1020d575380e6919549e37e77542103f4d4a3765`
- Runtime config paths:
  - `/data/tars-setup-lab/team-shared/connectors.shared.yaml`
  - `/data/tars-setup-lab/team-shared/secrets.shared.yaml`
  - `/data/tars-setup-lab/team-shared/shared-test.env`

## Summary

This run closes the investigation side of EVI-22:

1. Fresh shared-lab repro proves `jumpserver-main` still cannot execute commands because JumpServer returns `403 permission_denied` / `命令执行已禁用`.
2. Fresh endpoint replay proves the target JumpServer instance does not expose `/api/v1/ops/command/`; it returns `404`.
3. TARS had a local product bug: an API-only JumpServer health probe could make `setup/status` re-advertise `jumpserver-main` as the primary execution runtime, even though direct execution still failed.
4. This change fixes that TARS-side bug and keeps `ssh-main` primary until JumpServer has real execution-success evidence.

## Root Cause Evidence

Direct replay on `192.168.3.100` using the same AK/SK configured in `secrets.shared.yaml`:

| Method | Path | Result |
| --- | --- | --- |
| `GET` | `/api/v1/assets/hosts/?limit=1` | `200`, host metadata returned for `192.168.3.9 dev` |
| `POST` | `/api/v1/ops/jobs/` | `403`, `{"detail":"命令执行已禁用","code":"permission_denied"}` |
| `POST` | `/api/v1/ops/command/` | `404`, `{"error":"Not found"}` |

Interpretation:

- request signing and JumpServer connectivity are valid
- the jobs endpoint exists on this JumpServer instance
- the configured JumpServer identity is blocked from command execution by JumpServer-side policy/permission settings
- this remains a path B blocker until owner/admin decides to change JumpServer policy or grants wider permissions

## Product-Level Reproduction

Fresh TARS repro before the fix:

- `POST /api/v1/connectors/jumpserver-main/health` returned connector health based on API probe success.
- `POST /api/v1/connectors/jumpserver-main/execution/execute` with `target_host=192.168.3.9`, `command=whoami`, `operator_reason=EVI-22 live probe` returned:

```json
{"error":{"code":"internal_error","message":"jumpserver job submission failed: status=403 body={\"detail\":\"命令执行已禁用\",\"code\":\"permission_denied\"}"}}
```

- Immediately after a manual JumpServer health probe, `GET /api/v1/setup/status` incorrectly reselected `jumpserver-main` as both `execution_runtime.primary` and `verification_runtime.primary`.

That was the TARS-side defect fixed in this change.

## TARS Fix

Changed behavior:

- `jumpserver-main` API probe success now records degraded execution health: `jumpserver API probe succeeded; execution not yet verified`.
- execution runtime auto-selection ignores JumpServer API-only health and keeps `ssh-main` primary while JumpServer remains unverified for execution.
- successful JumpServer execution can still promote the connector later via the normal `execution runtime completed successfully` health record.

Code paths touched:

- `internal/modules/action/service.go`
- `internal/modules/connectors/runtime_selection.go`
- regression tests in workflow / postgres / action / http layers

## Validation

### Local CI-Aligned Checks

| Command | Result |
| --- | --- |
| `make check-mvp` | PASS after restoring web deps with `npm ci` |

Notes:

- first `make check-mvp` run exposed one expected HTTP-layer assertion mismatch after the health-semantics change
- second run passed fully

### Shared-Lab Validation Status

Runtime deploy completed to `192.168.3.100` with runtime head:

- `/data/tars-setup-lab/team-shared/runtime_git_head` -> `1020d575380e6919549e37e77542103f4d4a3765`

Post-deploy validation:

1. `POST /api/v1/connectors/jumpserver-main/health`
   - result: `degraded`
   - summary: `jumpserver API probe succeeded; execution not yet verified`
2. `GET /api/v1/setup/status`
   - `execution_runtime.primary.connector_id=ssh-main`
   - `verification_runtime.primary.connector_id=ssh-main`
   - `jumpserver-main` no longer re-promotes itself into primary execution after an API-only health probe
3. `POST /api/v1/connectors/jumpserver-main/execution/execute`
   - still returns `403 permission_denied`
   - message: `jumpserver job submission failed: status=403 body={"detail":"命令执行已禁用","code":"permission_denied"}`

Result: TARS-side runtime semantics are fixed, but the JumpServer-side execution blocker remains.

### GitHub CI vs Shared-Lab

- Local CI-aligned gate: `make check-mvp` PASS
- Shared-lab smoke: PASS during deploy (`smoke-remote=passed`)
- Shared-lab live validation: PARTIAL, still blocked by pre-existing tool-plan logs scenario failure
  - `scenario=logs expected tools ['metrics.query_range', 'logs.query', 'observability.query'], got ['logs.query']`
  - this failure is outside the JumpServer runtime-selection fix and existed as a separate live-validation problem

## Fallback Recommendation

- keep `ssh-main` as the shared-lab executable primary runtime
- keep `jumpserver-main` configured for auditability and future revalidation
- do not treat JumpServer API health as proof of executable runtime health

## Owner/Admin Action Needed For Path A

One of these must change on the JumpServer side before path A is possible:

- command filter or command-execution policy for the target asset / node
- asset authorization for the TARS JumpServer identity on `192.168.3.9 dev`
- ops-module permission for the AK/SK user

Until owner/admin ACKs the blocker or updates JumpServer policy, EVI-22 remains on path B.
