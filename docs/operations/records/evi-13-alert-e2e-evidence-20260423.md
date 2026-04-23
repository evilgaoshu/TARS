# EVI-13 Alert / Telegram Evidence 2026-04-23

- Date: `2026-04-23`
- Branch: `agent/dev-opencode-gpt5-4/57e31772`
- Shared lab host: `192.168.3.100`
- Canonical runtime root: `/data/tars-setup-lab`
- Verifier: `DEV-opencode-gpt5.4`
- Commit: `b5e4765`

## Summary

This record captures fresh `192.168.3.100` evidence for the EVI-13 go/no-go gate.

Two things are true at the same time:

1. The shared lab is healthy enough to run smoke, connector live validation, and Telegram/webhook ingestion.
2. The full approval/execution/verifier path is currently blocked by live shared-env credential drift:
   - Telegram bot token is still a placeholder, so delivery is `stub`
   - `lmstudio-local` was previously unreachable from `192.168.3.100`
   - `gemini-backup` previously reported `API key not valid`

This run also found and fixed one repo-controlled baseline defect:

- `deploy/team-shared/shared-test.env.example` used `TARS_SSH_ALLOWED_HOSTS=REPLACE_WITH_SSH_ALLOWED_HOSTS`
- On `192.168.3.100`, that placeholder leaked into the live process and caused the Telegram missing-host path to create a bogus session instead of returning guidance
- The template now ships `TARS_SSH_ALLOWED_HOSTS=192.168.3.100,127.0.0.1`
- After redeploy, the live process env reported `TARS_SSH_ALLOWED_HOSTS=192.168.3.100,127.0.0.1`
- After redeploy, the missing-host Telegram request returned guidance and created no new session

## Repo-Side Fix Applied

- File: `deploy/team-shared/shared-test.env.example`
- Change: replace placeholder SSH allowlist with `192.168.3.100,127.0.0.1`
- Why: the shared deploy flow preserves non-secret env values from the repo template, so leaving a placeholder here breaks real runtime behavior on `192.168.3.100`

## 3.100 Baseline Checks

### Command

```sh
source scripts/lib/shared_ops_token.sh
export TARS_REMOTE_HOST=192.168.3.100
export TARS_REMOTE_USER=root
export TARS_REMOTE_BASE_DIR=/data/tars-setup-lab
TOKEN=$(shared_ops_token_export)

curl -fsS http://192.168.3.100:8081/healthz
curl -fsS http://192.168.3.100:8081/readyz
curl -fsS -H "Authorization: Bearer $TOKEN" http://192.168.3.100:8081/api/v1/setup/status
TARS_OPS_API_TOKEN="$TOKEN" TARS_OPS_BASE_URL=http://192.168.3.100:8081 TARS_SERVER_BASE_URL=http://192.168.3.100:8081 bash scripts/pilot_hygiene_check.sh
```

### Result

- `/healthz`: `{"status":"ok"}`
- `/readyz`: `{"status":"ready","degraded":false}`
- `pilot_hygiene_check.sh`: `result=clean`
- `rollout_mode`: `knowledge_on`
- `telegram.last_result`: `stub`
- `model.provider_id`: `lmstudio-local`
- `assist_model.provider_id`: `gemini-backup`
- `smoke_defaults.hosts`: `192.168.3.100,127.0.0.1` after redeploy

## Shared Config Snapshot

### Effective process env

Observed from `/proc/<pid>/environ` after redeploy:

```text
TARS_SERVER_LISTEN=0.0.0.0:8081
TARS_WEB_DIST_DIR=/data/tars-setup-lab/web-dist
TARS_PROVIDERS_CONFIG_PATH=/data/tars-setup-lab/team-shared/providers.shared.yaml
TARS_CONNECTORS_CONFIG_PATH=/data/tars-setup-lab/team-shared/connectors.shared.yaml
TARS_ACCESS_CONFIG_PATH=/data/tars-setup-lab/team-shared/access.shared.yaml
TARS_AUTHORIZATION_CONFIG_PATH=/data/tars-setup-lab/team-shared/authorization.shared.yaml
TARS_SECRETS_CONFIG_PATH=/data/tars-setup-lab/team-shared/secrets.shared.yaml
TARS_AUTOMATIONS_CONFIG_PATH=/data/tars-setup-lab/team-shared/automations.shared.yaml
TARS_SKILLS_CONFIG_PATH=/data/tars-setup-lab/team-shared/skills.shared.yaml
TARS_ROLLOUT_MODE=knowledge_on
TARS_SSH_ALLOWED_HOSTS=192.168.3.100,127.0.0.1
```

### Shared env and config paths

- Shared env: `/data/tars-setup-lab/team-shared/shared-test.env`
- Providers: `/data/tars-setup-lab/team-shared/providers.shared.yaml`
  - primary: `lmstudio-local / qwen/qwen3-4b-2507`
  - assist: `gemini-backup / gemini-flash-lite-latest`
  - extra configured entry: `dashscope-kimi`
- Connectors: `/data/tars-setup-lab/team-shared/connectors.shared.yaml`
  - metrics: `victoriametrics-main -> http://127.0.0.1:8428`
  - logs: `victorialogs-main -> http://127.0.0.1:9428`
  - execution primary: `jumpserver-main`
  - execution fallback: `ssh-main`
- Access / Telegram channel: `/data/tars-setup-lab/team-shared/access.shared.yaml`
  - channel id: `telegram-main`
  - target: `445308292`
- Approval routing: `/data/tars-setup-lab/team-shared/approvals.shared.yaml`
  - timeout: `15m`
  - `service_owner.sshd -> 445308292`
  - `oncall_group.default -> 445308292`
- Authorization: `/data/tars-setup-lab/team-shared/authorization.shared.yaml`
  - whitelist default: `direct_execute`
  - unmatched default: `require_approval`
  - override: `systemctl restart sshd*` on `192.168.3.100` requires approval

## Alert-Driven Sample

### Command

```sh
curl -fsS -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -X POST http://192.168.3.100:8081/api/v1/smoke/alerts \
  -d '{"alertname":"Evi13Manual","service":"sshd","host":"192.168.3.100","severity":"critical","summary":"manual smoke from EVI-13"}'
```

### Result

- session_id: `05294966-2bd3-4815-87d7-12cd153b7fc8`
- source: `vmalert`
- initial status: `analyzing`
- final status: `resolved`
- execution_id: none
- approval result: none
- verification status: none
- outbox state: no failed/blocked outbox at capture time
- spool evidence: none, because no execution was drafted
- notification evidence: diagnosis notification prepared for Telegram target `445308292`

### Timeline Summary

- `alert_received`
- `diagnosis_requested`
- `diagnosis_message_prepared`
- `chat_answer_completed`
- `diagnosis_completed`

### Why it did not reach approval/execution

- The runtime produced a diagnosis-only closeout and no execution draft.
- This does not satisfy the hard requirement `diagnosis -> approval -> execution -> verifier -> resolved`.
- Live runtime blockers are listed in the blocker section below.

## Telegram Conversational Samples

### A. Missing host guidance after allowlist fix

#### Command

```sh
curl -sS -H 'Content-Type: application/json' \
  -X POST http://192.168.3.100:8081/api/v1/channels/telegram/webhook \
  --data '{"update_id":920101,"message":{"message_id":101,"text":"看系统负载","from":{"id":42,"username":"alice","is_bot":false},"chat":{"id":"445308292","type":"private"}}}'
```

#### Result

- HTTP status: `200 accepted`
- Session creation: none
- Log evidence:

```text
resource_type=telegram_chat action=guidance resource_id=telegram_update:920101
body_preview=还不知道要查哪台主机。请直接发主机名/IP，或在消息里写 `host=192.168.3.106 看系统负载`。
telegram send stub reason=bot token is a placeholder
```

- This satisfies the missing-host guidance requirement on the fixed shared-lab baseline.
- The example host in the guidance text still mentions `192.168.3.106` because that string is current product copy; EVI-13 only fixes the live allowlist baseline and validates the no-session guidance behavior.

### B. Explicit host Telegram request

#### Command

```sh
curl -sS -H 'Content-Type: application/json' \
  -X POST http://192.168.3.100:8081/api/v1/channels/telegram/webhook \
  --data '{"update_id":920102,"message":{"message_id":102,"text":"host=192.168.3.100 看一下你的出口IP是多少","from":{"id":52,"username":"bigluandou","is_bot":false},"chat":{"id":"445308292","type":"private"}}}'
```

#### Result

- session_id: `51aa17e4-377b-418a-93c1-a666a4b4bfe4`
- source: `telegram_chat`
- final status: `resolved`
- tool_plan: planned `execution.run_command` with command `curl -fsS https://api.ipify.org && echo`
- execution_id: none
- approval result: none
- verification status: none
- outbox state: no failed/blocked outbox at capture time
- spool evidence: none
- notification evidence: diagnosis message prepared for target `445308292`

### Gap vs hard requirement

- The explicit-host request created a real `telegram_chat` session, but it still resolved without creating an execution draft or pending approval item.
- This does not satisfy the required conversational closure `specified host -> approval -> execution -> verifier -> resolved`.

## Live Blockers On 3.100

### Telegram channel blocker

- `GET /api/v1/setup/status` reports:
  - `telegram.last_result=stub`
  - `telegram.last_detail=bot token is a placeholder`
- Runtime log evidence shows `telegram send stub` instead of a real send.

### Provider baseline blocker

Fresh and recent shared-lab evidence shows both reasoning providers are not trustworthy for a real approval-driven sample:

- earlier fresh setup/status before redeploy reported:
  - `lmstudio-local`: `connect: no route to host`
  - `gemini-backup`: `API key not valid`
- runtime log before redeploy showed:

```text
diagnosis finalizer failed, falling back
primary model failed: dial tcp 192.168.1.132:1234: connect: no route to host
assist model failed: model completion failed: status=400 message=API key not valid
```

After redeploy, setup/status still reports Telegram `stub` and no fresh provider success timestamps, so this issue cannot claim the full approval/execution chain is restored.

## CI-Aligned Verification

### Local main check

- Command: `make check-mvp`
- Result: passed

### Shared env smoke

- Command: `make smoke-remote`
- Result: passed
- Notes:
  - readiness passed
  - discovery passed
  - hygiene passed
  - performance spot check passed

### Shared env live validation

- Command: `make live-validate`
- Result: passed
- Notes:
  - tool-plan live validation passed
  - capability approval path still returns `202 pending_approval` for capability invoke
  - observability connector live validation passed
  - these checks prove shared runtime health and connector capability paths, not the broken Telegram approval/execution conversation path for EVI-13

## MVP Checklist Section 5 Status Snapshot

- `make check-mvp`: pass
- validation entrypoints (`make smoke-remote`, `make live-validate`): pass
- providers / SSH / Telegram / VM full readiness: partial
  - SSH allowlist baseline: fixed and verified
  - VM/VL runtime: healthy
  - Telegram: blocked by placeholder bot token
  - reasoning providers: blocked/untrusted for full acceptance
- `/runtime-checks -> Telegram approval -> resolved`: not satisfied in this run
- Telegram conversational acceptance:
  - missing-host guidance with no session: pass after fix
  - explicit-host full approval/execution closure: not satisfied

## GitHub CI vs Shared-Env Status

- Local CI-aligned checks in this branch: `make check-mvp` passed
- Shared env checks on `192.168.3.100`: `make smoke-remote` and `make live-validate` passed
- Shared env pilot go/no-go acceptance: blocked
  - reason: Telegram bot token placeholder and reasoning-provider baseline not healthy enough to produce the required approval/execution/verifier evidence

## Conclusion

EVI-13 is not a full go/no-go pass yet.

What is complete in this branch:

- repo-side shared baseline defect fixed
- `192.168.3.100` redeployed and verified healthy
- missing-host Telegram guidance behavior now works correctly with no session creation
- explicit-host Telegram request still creates a real `telegram_chat` session
- decision-gate and checklist evidence are now documented from fresh runtime data

What still blocks final acceptance:

- real Telegram send/approval flow
- real execution draft / approval / verifier / spool evidence from the required end-to-end path
- trustworthy provider baseline for reasoning-driven execution planning
