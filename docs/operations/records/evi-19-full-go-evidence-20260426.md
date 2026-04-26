# EVI-19 Full-Go Execution / Verifier Evidence

- Issue: EVI-19
- Capture window: `2026-04-27 00:08 +08` (`2026-04-26 16:08 UTC`)
- Shared environment: `192.168.3.100`
- Target host: `192.168.3.9`
- PR: `https://github.com/evilgaoshu/TARS/pull/14`
- PR head commit at initial PR creation: `43fb6db7b7c31837fdb07eef8e71da1162bfcbf8`
- Deployed runtime commit: `61eab80f3e2628bd6cc3f27ef51462a67e0c85c7`
- Runtime head file: `/data/tars-setup-lab/team-shared/runtime_git_head`
- Runtime log: `/data/tars-setup-lab/team-shared/tars-dev.log`
- Config paths:
  - `/data/tars-setup-lab/team-shared/connectors.shared.yaml`
  - `/data/tars-setup-lab/team-shared/secrets.shared.yaml`
  - `/data/tars-setup-lab/team-shared/shared-test.env`

## Scope

This record closes the EVI-13 owner-approved skip for full execution / verifier / final resolved by collecting a new shared-lab evidence pair:

1. `jumpserver-main` API health is configured, but command execution is denied by JumpServer policy.
2. `ssh-main` is used as the controlled fallback and completes `approval -> execution -> verifier -> resolved`.

No plaintext JumpServer or SSH secrets are stored in this record.

## Runtime Configuration

- `jumpserver-main`
  - protocol: `jumpserver_api`
  - base URL: `http://192.168.3.100`
  - secrets: `connector/jumpserver-main/access_key`, `connector/jumpserver-main/secret_key`
- `ssh-main`
  - protocol: `ssh_native`
  - host: `192.168.3.9`
  - username: `root`
  - credential custody ID: `evi19-ssh-root-3-9`
- SSH allowlist: `192.168.3.100,127.0.0.1,192.168.3.9`
- rollout mode: `knowledge_on`

## Commands

```sh
make check-mvp

TARS_REMOTE_USER=root \
TARS_DEPLOY_SKIP_WEB=1 \
TARS_DEPLOY_SKIP_VALIDATE=1 \
bash scripts/deploy_team_shared.sh

TARS_REMOTE_USER=root make smoke-remote

# EVI-19 replay used Ops API smoke alerts, then approved the generated execution:
# POST /api/v1/smoke/alerts
# GET  /api/v1/sessions/{session_id}
# POST /api/v1/executions/{execution_id}/approve
# GET  /api/v1/executions/{execution_id}/output
```

## Local / Shared Validation Summary

| Check | Result | Notes |
| --- | --- | --- |
| `make check-mvp` | PASS | Includes `go test ./...`, core coverage, Go build, OpenAPI validation, web lint/test/build. |
| Targeted regression tests | PASS | `go test ./internal/modules/workflow -run TestSelectExecutionRuntimeFallsBackToHealthySSHNativeConnector -count=1`; `go test ./internal/repo/postgres -run TestSelectExecutionRuntimeFallsBackToHealthySSHNativeConnector -count=1`. |
| `TARS_REMOTE_USER=root make smoke-remote` | PASS | healthz/readyz/discovery/hygiene/performance spot-check passed on `192.168.3.100`; failed/blocked/pending outbox totals were `0`. |
| GitHub PR checks | PASS | PR #14 checks passed: L0 Pre-check, L1 MVP Checks, L2 Security Regression, Secret Scan, Static Demo Build. |
| Full `deploy_team_shared.sh` validation | PARTIAL | Deploy/restart/readiness passed, but `live-validate` failed in the model-planner smoke scenario: `scenario=logs expected tools ['metrics.query_range', 'logs.query', 'observability.query'], got ['logs.query']`. This is recorded as residual shared-live validation risk, separate from the EVI-19 execution/verifier closure. |

## JumpServer Probe

Purpose: prove the primary JumpServer runtime was configured and why fallback was used.

| Field | Value |
| --- | --- |
| session_id | `b82cb213-d09b-4e88-8b90-313e6fa9ace9` |
| execution_id | `41043106-2b4f-408b-82dd-e3c6d876d867` |
| connector_id | `jumpserver-main` |
| protocol / mode | `jumpserver_api` / `jumpserver_job` |
| approval result | accepted through `POST /api/v1/executions/41043106-2b4f-408b-82dd-e3c6d876d867/approve` (`200`) |
| execution result | failed |
| exit_code | `1` |
| verifier | not run because execution failed |
| final state | `failed` |

Connector health after the probe:

```text
status=degraded
summary=execution runtime failed: jumpserver job submission failed: status=403 body={"detail":"命令执行已禁用","code":"permission_denied"}
checked_at=2026-04-26T16:08:37.707135868Z
```

Log anchors:

- `/data/tars-setup-lab/team-shared/tars-dev.log:2836` approval route prepared for `41043106-2b4f-408b-82dd-e3c6d876d867`.
- `/data/tars-setup-lab/team-shared/tars-dev.log:2839` approval endpoint invoked by `ops-token`.

## ssh_native Fallback Closure

Purpose: complete the full-go path after the recorded JumpServer command-execution blocker.

| Field | Value |
| --- | --- |
| session_id | `811a30f4-8e3e-4ccc-8989-8af485a25c38` |
| execution_id | `0f46f347-29c5-49b4-b01d-23d41ebb1253` |
| connector_id | `ssh-main` |
| protocol / mode | `ssh_native` / `ssh_native` |
| authorization decision | `require_approval` |
| approval route | `service_owner:sshd`, target `445308292` |
| approval result | accepted through `POST /api/v1/executions/0f46f347-29c5-49b4-b01d-23d41ebb1253/approve` (`200`) |
| command | `whoami` |
| execution result | `completed` |
| output_ref | `/data/tars-setup-lab/execution-output/0f46f347-29c5-49b4-b01d-23d41ebb1253-20260426T160848Z.log` |
| execution output | `root` |
| verification result | `success` |
| verifier basis | `systemctl is-active sshd`, exit code `0`, output `active` |
| final session state | `resolved` |
| failed/blocked/pending outbox | `0 / 0 / 0` |

Timeline evidence:

```text
alert_received
diagnosis_requested
diagnosis_message_prepared
authorization_decided action=require_approval command=whoami
approval_route_selected approval route=service_owner:sshd targets=445308292
execution_draft_ready execution=0f46f347-29c5-49b4-b01d-23d41ebb1253 connector=ssh-main protocol=ssh_native mode=ssh_native fallback_used=false
approval_message_prepared
approval_accepted execution=0f46f347-29c5-49b4-b01d-23d41ebb1253 approved by ops-admin connector=ssh-main protocol=ssh_native mode=ssh_native
execution_completed execution=0f46f347-29c5-49b4-b01d-23d41ebb1253 completed successfully connector=ssh-main protocol=ssh_native mode=ssh_native
verify_success verification passed: sshd is active
```

Log anchors:

- `/data/tars-setup-lab/team-shared/tars-dev.log:2852` approval route prepared for `0f46f347-29c5-49b4-b01d-23d41ebb1253`.
- `/data/tars-setup-lab/team-shared/tars-dev.log:2858` approval endpoint invoked by `ops-token`.
- `/data/tars-setup-lab/team-shared/tars-dev.log:2859` session read shows final status `resolved`.

## Residual Risks

- JumpServer remains unsuitable for full execution until JumpServer command execution is enabled for the supplied AK/SK and target asset. The API probe succeeds, but job submission returns `403 permission_denied`.
- The final ssh_native sample proves the TARS execution/verifier/resolved closure, but it is a controlled fallback rather than JumpServer primary-path success.
- A planner auxiliary `connector.invoke_capability` step against `ssh-main` reported no generic capability runtime for `host.verify`; it did not block the approved `execution.run_command` path or verifier success.
- Browser screenshots at `1440px` and `390px` were not captured in this run.

## Runtime Choice Recommendation

Use `ssh-main` as the controlled shared-lab execution fallback for full-go evidence until JumpServer command execution is enabled. Keep `jumpserver-main` configured and probed, but do not treat API health alone as proof of executable runtime health.
