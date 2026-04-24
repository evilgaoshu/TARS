# EVI-15 3.100 Validation Record

## Intended Checks

- Verify SSH credential `rotation_required` hard-blocks `ssh_native` execution.
- Verify replacing credential material clears `rotation_required` and refreshes `LastRotatedAt`.
- Verify Secrets Inventory / Ops surface shows custody status, `key_id`, and missing/rotation-required states without raw `secret_ref`.
- Verify break-glass approval path remains explicit and auditable.

## Local Checks

```bash
make check-mvp
make smoke-remote
```

Observed locally:

- `make check-mvp` passed before shared-env validation.
- `make smoke-remote` passed against `192.168.3.100` after deploy.

## Shared Environment Commands

```bash
# Deploy current branch to 192.168.3.100 using the canonical shared token.
token="$(ssh root@192.168.3.100 'sed -n "s/^TARS_OPS_API_TOKEN=//p" /data/tars-setup-lab/team-shared/shared-test.env | head -n 1')"
TARS_REMOTE_USER=root TARS_OPS_API_TOKEN="$token" make deploy-sync

# Verify Secrets Inventory no longer exposes raw secret_ref and now shows custody key_id.
curl -fsS -H "Authorization: Bearer $token" http://192.168.3.100:8081/api/v1/config/secrets

# Verify shared env currently has SSH custody configured.
curl -fsS -H "Authorization: Bearer $token" http://192.168.3.100:8081/api/v1/ssh-credentials

# Verify ops-token cannot use ssh_native runtime health or execution paths.
curl -sS -o /tmp/evi15_ssh_health_body.json -w '%{http_code}' \
  -H "Authorization: Bearer $token" \
  -H 'Content-Type: application/json' \
  -X POST http://192.168.3.100:8081/api/v1/connectors/ssh-main/health \
  -d '{}'
cat /tmp/evi15_ssh_health_body.json

curl -sS -o /tmp/evi15_ssh_exec_body.json -w '%{http_code}' \
  -H "Authorization: Bearer $token" \
  -H 'Content-Type: application/json' \
  -X POST http://192.168.3.100:8081/api/v1/connectors/ssh-main/execution/execute \
  -d '{"target_host":"192.168.3.100","command":"whoami","operator_reason":"evi-15 break-glass validation"}'
cat /tmp/evi15_ssh_exec_body.json

# Create a fresh approval-required execution and approve it explicitly.
chat_resp="$(curl -fsS -H "Authorization: Bearer $token" -H 'Content-Type: application/json' \
  -X POST http://192.168.3.100:8081/api/v1/chat/messages \
  -d '{"message":"host=192.168.3.100 看一下你的出口IP是多少 #evi15","host":"192.168.3.100","severity":"info"}')"
session_id="$(printf '%s' "$chat_resp" | python3 -c 'import sys,json; print(json.load(sys.stdin)["session_id"])')"
curl -fsS -H "Authorization: Bearer $token" http://192.168.3.100:8081/api/v1/sessions/$session_id
curl -fsS -H "Authorization: Bearer $token" -X POST \
  http://192.168.3.100:8081/api/v1/executions/a27ce070-e05d-4c0c-9de9-95e265128cbd/approve

# Confirm approval endpoint audit metadata contains break-glass markers.
curl -fsS -H "Authorization: Bearer $token" \
  'http://192.168.3.100:8081/api/v1/audit?limit=500&page=1&sort_by=created_at&sort_order=desc'
```

## Result Summary

- Deploy succeeded after injecting the canonical shared token from `/data/tars-setup-lab/team-shared/shared-test.env` into `make deploy-sync`.
- `/api/v1/config/secrets` now returns `custody_configured=true`, `custody_key_id="local-shared"`, and `status` fields on items without exposing raw `ref` values.
- `/api/v1/ssh-credentials` returned `configured=true` and an empty item list in the current shared fixture state.
- `POST /api/v1/connectors/ssh-main/health` returned `403` with `break_glass_denied`.
- `POST /api/v1/connectors/ssh-main/execution/execute` returned `403` with `break_glass_denied`.
- A fresh approval-required execution was created from chat flow and explicitly approved through `/api/v1/executions/{id}/approve`.
- Audit evidence confirms `approval_endpoint_invoked` was written with:
  - `action=approve`
  - `actor_source=ops-token`
  - `actor_user_id=ops-admin`
  - `break_glass=true`
  - `execution_id=a27ce070-e05d-4c0c-9de9-95e265128cbd`

## Remaining Gap

- Shared env currently has no seeded SSH credential entry, so `rotation_required -> update clears status -> LastRotatedAt refresh` was validated by automated tests and local code paths, but not replayed end-to-end on `192.168.3.100` in this session.
