# EVI-13 Alert / Telegram Evidence 2026-04-23

- Date: `2026-04-23`
- Branch: `agent/dev-opencode-gpt5-4/57e31772`
- Shared lab host: `192.168.3.100`
- Canonical runtime root: `/data/tars-setup-lab`
- Verifier: `DEV-opencode-gpt5.4`
- Commit: `41d60fbcfd793939cdce92d64f14eeeb0ad518af`

## Scope Note

- Owner approved an EVI-13 scope exception on `2026-04-23`: PR #8 closeout may stop at `diagnosis -> execution_draft_ready -> approval_accepted`.
- SSH / JumpServer execution and verifier evidence are explicitly skipped for this closeout because the live execution runtime is being bypassed by owner instruction.
- This record therefore distinguishes between:
  - the owner-approved PR #8 closeout scope, and
  - the still-unmet full-go requirement for a future `execution -> verifier -> resolved` sample.

## Summary

This refreshed record captures the post-fix shared-lab state after commit `41d60fbcfd793939cdce92d64f14eeeb0ad518af` was deployed to `192.168.3.100`.

Current facts:

1. Shared-lab health/readiness is green.
2. The repo-side workflow bug that previously resolved `telegram_chat` sessions without creating an execution draft is fixed and verified live.
3. Telegram/provider runtime is now good enough to reach `execution_draft_ready` and `approval_accepted` with real `session_id` / `execution_id` evidence.
4. Execution still fails when routed into `jumpserver-main`, but owner explicitly asked QA/DEV to skip SSH and JumpServer for this PR closeout.

## 3.100 Baseline Checks

### Commands

```sh
source scripts/lib/shared_ops_token.sh
export TARS_REMOTE_HOST=192.168.3.100
export TARS_REMOTE_USER=root
export TARS_REMOTE_BASE_DIR=/data/tars-setup-lab
TOKEN=$(shared_ops_token_export)

curl -fsS http://192.168.3.100:8081/healthz
curl -fsS http://192.168.3.100:8081/readyz
curl -fsS -H "Authorization: Bearer $TOKEN" http://192.168.3.100:8081/api/v1/setup/status
```

### Result

- `/healthz`: `{"status":"ok"}`
- `/readyz`: `{"status":"ready","degraded":false}`
- shared runtime root: `/data/tars-setup-lab`
- shared log: `/data/tars-setup-lab/team-shared/tars-dev.log`
- `telegram.configured=true`
- `telegram.mode=polling`
- `telegram.last_result=error`
- `telegram.last_detail=telegram getUpdates failed: status=409 description=Conflict: terminated by other getUpdates request; make sure that only one bot instance is running`
- primary provider: `dashscope-kimi / kimi-k2.5`
- assist provider: `dashscope-kimi / kimi-k2.5`
- provider readiness summary: `model.last_result=success`

## Shared Config Snapshot

- Shared env: `/data/tars-setup-lab/team-shared/shared-test.env`
- Providers: `/data/tars-setup-lab/team-shared/providers.shared.yaml`
  - primary: `dashscope-kimi / kimi-k2.5`
  - assist: `dashscope-kimi / kimi-k2.5`
- Connectors: `/data/tars-setup-lab/team-shared/connectors.shared.yaml`
  - execution primary: `jumpserver-main`
  - execution fallback: `ssh-main`
- Access / Telegram channel: `/data/tars-setup-lab/team-shared/access.shared.yaml`
- Approval routing: `/data/tars-setup-lab/team-shared/approvals.shared.yaml`
- Authorization: `/data/tars-setup-lab/team-shared/authorization.shared.yaml`
- Effective non-secret runtime fields observed in process env / config:
  - `TARS_ROLLOUT_MODE=knowledge_on`
  - `TARS_SSH_ALLOWED_HOSTS=192.168.3.100,127.0.0.1`

## Telegram Conversational Evidence

### A. Missing-host guidance

Previously verified on the fixed shared baseline:

- missing-host Telegram message returns guidance
- no session is created when multiple hosts are configured and no host is supplied

This remains part of the acceptance evidence for PR #8 closeout.

### B. Explicit-host approval-path sample

#### Request

- Channel: Telegram webhook / conversational request
- Message text: `host=192.168.3.100 看一下你的出口IP是多少`

#### Result

- session_id: `e24d6484-a27c-4b3b-a73b-c165e7f76807`
- source: `telegram_chat`
- execution_id: `249ba0b4-1c1f-4bde-9fb5-341b07aecbc0`

#### Timeline evidence

The live session trace includes all of the following nodes:

- `authorization_decided`
- `approval_route_selected`
- `execution_draft_ready`
- `approval_message_prepared`
- `approval_accepted`

#### Delivery evidence

Shared runtime log `/data/tars-setup-lab/team-shared/tars-dev.log` records both diagnosis and approval Telegram dispatch with `delivery=sent` for this sample.

#### What this proves

- The repo-side workflow bug is fixed in live runtime.
- The conversational path no longer stops at the old `chat_answer_completed` closeout when a planned `execution.run_command` exists.
- `192.168.3.100` now reliably reaches `execution_draft_ready` and `approval_accepted`, which is the owner-approved stop point for PR #8 closeout.

## Owner-Skipped Execution / Verifier State

The same execution still fails if allowed to continue into the live execution runtime:

- execution_id: `249ba0b4-1c1f-4bde-9fb5-341b07aecbc0`
- status: `failed`
- connector_id: `jumpserver-main`
- protocol: `jumpserver_api`
- execution_mode: `jumpserver_job`
- exit_code: `1`
- `output_ref=null`
- execution output API returns empty chunks

Connector health evidence from `GET /api/v1/config/connectors`:

- `jumpserver-main` is degraded
- summary: `execution runtime failed: missing required fields: access_key, secret_key`

This is recorded for completeness, but it is not a PR #8 closeout blocker after the owner instruction to skip SSH / JumpServer.

## Approval Evidence Required By QA

QA asked whether `approval_accepted` needs to be reflected in the decision gate. The answer is yes.

This record intentionally includes the concrete evidence QA asked for:

- `session_id=e24d6484-a27c-4b3b-a73b-c165e7f76807`
- `execution_id=249ba0b4-1c1f-4bde-9fb5-341b07aecbc0`
- timeline entries `execution_draft_ready` and `approval_accepted`
- log evidence that diagnosis and approval messages were actually sent

For this closeout, log/API evidence is sufficient; screenshot evidence is best-effort rather than mandatory.

## Verification Snapshot

- Local targeted workflow tests: pass
- `go test ./internal/modules/workflow ./internal/repo/postgres -count=1`: pass
- `make check-mvp`: pass
- PR #8 GitHub checks: green at the time QA reviewed the workflow fix

## GitHub CI vs Shared-Env Status

- GitHub CI / PR checks: pass for the branch at commit `41d60fbcfd793939cdce92d64f14eeeb0ad518af`
- Shared env health/readiness: pass
- Shared env owner-approved closeout scope: pass
  - missing-host guidance with no session creation
  - explicit-host session creation
  - `execution_draft_ready`
  - `approval_accepted`
- Shared env full-go execution/verifier sample: still not collected
  - reason: execution runtime currently routes into degraded `jumpserver-main`
  - disposition for PR #8: owner-skipped

## Conclusion

Within the owner-approved PR #8 closeout scope, EVI-13 evidence is now sufficient.

What is now proven on `192.168.3.100`:

- shared runtime is healthy and ready
- provider baseline is good enough to support the approval path
- Telegram conversational missing-host guidance works without creating a bogus session
- explicit-host Telegram requests reach `execution_draft_ready` and `approval_accepted`
- the workflow fix from commit `41d60fbcfd793939cdce92d64f14eeeb0ad518af` is verified live

What is intentionally deferred beyond this closeout:

- real SSH / JumpServer execution success
- verifier success
- final `resolved` evidence for the execution runtime path

Those items remain future full-go work, not PR #8 closeout requirements after the owner-approved scope change.
