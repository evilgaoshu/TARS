# Credential Rotation Execution Tracker

Date: 2026-04-08

This tracker advances the GitHub pre-push external credential rotation work as far as possible from the local repo only. No external account access was used, no credentials were rotated, and no commit/push/stage action was performed. On 2026-04-08, the owner confirmed that the previously discussed credential values were local-only / non-live and are not GitHub push blockers.

## Status Summary

| Credential class | Owner | Current tree status | External action required | Post-rotation verification | GitHub push gate |
| --- | --- | --- | --- | --- | --- |
| Ops API | Platform/Ops | Done: checked-in default replaced by `REPLACE_WITH_OPS_API_TOKEN` in `deploy/team-shared/shared-test.env`; current-tree grep found no live token value in the audited paths. | Owner confirmed local-only / non-live on 2026-04-08; no external rotation required as a GitHub push blocker. | `curl -H "Authorization: Bearer $TARS_OPS_API_TOKEN" http://192.168.3.100:8081/api/v1/setup/status` or `scripts/pilot_hygiene_check.sh`; old token must fail with unauthorized/forbidden. | Unblocked by owner confirmation: `invalid/non-live`. |
| Telegram bot token | Telegram/integration owner | Done: placeholder-only in `deploy/team-shared/shared-test.env`; no live token matched the targeted scan. | Owner confirmed local-only / non-live on 2026-04-08; no BotFather rotation required as a GitHub push blocker. | `curl "https://api.telegram.org/bot${TARS_TELEGRAM_BOT_TOKEN}/getMe"`; old token must fail, new token must succeed. | Unblocked by owner confirmation: `invalid/non-live`. |
| Telegram webhook secret | Telegram/integration owner | Done: runtime wiring exists, and the local tree keeps only example/placeholder usage such as `deploy/pilot/pilot.env.example`. | Owner confirmed local-only / non-live on 2026-04-08; no webhook-secret rotation required as a GitHub push blocker. | `curl -H "X-Telegram-Bot-Api-Secret-Token: $TARS_TELEGRAM_WEBHOOK_SECRET" -H 'Content-Type: application/json' -X POST http://192.168.3.100:8081/api/v1/channels/telegram/webhook -d '{}'`; old secret must be rejected. | Unblocked by owner confirmation: `invalid/non-live`. |
| VMAlert webhook secret | Observability owner | Done: webhook receiver path is present; no literal secret value was found in the checked tree. | Owner confirmed local-only / non-live on 2026-04-08; no external reissue required as a GitHub push blocker. | `curl -H "X-Tars-Signature: $TARS_VMALERT_WEBHOOK_SECRET" -H 'Content-Type: application/json' -X POST http://192.168.3.100:8081/api/v1/webhooks/vmalert --data-binary @alert.json`; old secret must fail. | Unblocked by owner confirmation: `invalid/non-live`. |
| Gemini API key | Model/AI owner | Done: tree now uses secret refs / placeholders, including `deploy/team-shared/secrets.shared.yaml` and `deploy/pilot/providers.example.yaml`; no live Gemini key pattern matched the current-tree scan. | Owner confirmed local-only / non-live on 2026-04-08; no Google-side rotation required as a GitHub push blocker. | `POST /api/v1/config/providers/check` with the Gemini provider ID, or `curl -fsS -H 'Content-Type: application/json' -d '{"provider_id":"<gemini-provider-id>"}' http://192.168.3.100:8081/api/v1/config/providers/check`; old key must be disabled. | Unblocked by owner confirmation: `invalid/non-live`. |
| DashScope API key | Model/AI owner | Done: tree now uses secret refs / placeholders in `deploy/team-shared/secrets.shared.yaml` and related provider templates; no live DashScope key pattern matched the current-tree scan. | Owner confirmed local-only / non-live on 2026-04-08; no DashScope-side rotation required as a GitHub push blocker. | `POST /api/v1/config/providers/check` with the DashScope provider ID, or `curl -fsS -H 'Content-Type: application/json' -d '{"provider_id":"<dashscope-provider-id>"}' http://192.168.3.100:8081/api/v1/config/providers/check`; old key must be disabled. | Unblocked by owner confirmation: `invalid/non-live`. |
| Dex / OIDC client secret | IAM owner | Done: placeholders are in place in `deploy/team-shared/dex.config.yaml` and `deploy/docker/dex.config.yaml`; current-tree grep found no literal client secret. | Owner confirmed local-only / non-live on 2026-04-08; no Dex/OIDC reissue required as a GitHub push blocker. | OIDC login via `/api/v1/auth/callback/dex-local`; old client secret must fail the browser login flow and the new secret must complete login. | Unblocked by owner confirmation: `invalid/non-live`. |
| Dex/local password hash | IAM owner | Done: bcrypt hash was not found in the current-tree targeted scan; only placeholder/template material remains in the shared config path. | Owner confirmed local-only / non-live on 2026-04-08; no password reset required as a GitHub push blocker. | `curl -fsS -H 'Content-Type: application/json' -d '{"provider_id":"local_password","username":"<user>","password":"<new-password>"}' http://192.168.3.100:8081/api/v1/auth/login`; old password must fail and the new password must succeed. | Unblocked by owner confirmation: `invalid/non-live`. |
| Local token auth provider secret | IAM owner | Done: local-token path is modeled as a token-login provider in `deploy/team-shared/access.shared.yaml`; current-tree scan found no live token literal. | Owner confirmed local-only / non-live on 2026-04-08; no local-token reissue required as a GitHub push blocker. | `curl -fsS -H 'Content-Type: application/json' -d '{"provider_id":"local_token","token":"<new-token>"}' http://192.168.3.100:8081/api/v1/auth/login`; old token must fail. | Unblocked by owner confirmation: `invalid/non-live`. |
| TOTP seed | IAM owner | Done: `REPLACE_WITH_TOTP_SECRET` remains as the checked-in placeholder in shared access config and runbook support scripts now require injected secret material. | Owner confirmed local-only / non-live on 2026-04-08; no MFA re-enrollment required as a GitHub push blocker. | `TARS_VALIDATE_AUTH_TOTP_SECRET=<new-secret> scripts/validate_auth_enhancements_live.sh`; old codes must fail and the new codes must pass. | Unblocked by owner confirmation: `invalid/non-live`. |
| JumpServer connector access/secret keys | Connector/Execution owner | Done: `deploy/team-shared/connectors.shared.yaml` now uses secret refs (`connector/jumpserver-main/access_key`, `.../secret_key`) rather than inline values; current-tree scan found no inline `access_key` / `secret_key` literals. | Owner confirmed local-only / non-live on 2026-04-08; no JumpServer-side reissue required as a GitHub push blocker. | `POST /api/v1/connectors/{connector_id}/health` for `jumpserver-main`; old keys must fail the connector health probe and the new keys must pass. | Unblocked by owner confirmation: `invalid/non-live`. |
| SSH key/root access material | Ops/Infra owner | Done: no private key block or host-key-bypass default was found in the checked publishable tree; remaining hits are historical docs/examples only. | Owner confirmed local-only / non-live on 2026-04-08; no SSH re-keying required as a GitHub push blocker. | `ssh -o BatchMode=yes -o IdentitiesOnly=yes -i ~/.ssh/<new_key> <user>@<host> 'true'`; old key must stop working and host-key checking must stay enabled. | Unblocked by owner confirmation: `invalid/non-live`. |
| GitHub/deploy tokens | Repo/CI owner | Done: no exact-format `gh*` token was found in the current-tree scan; repo-side docs only describe the minimum-scope policy. | Owner confirmed local-only / non-live on 2026-04-08; no GitHub/deploy token rotation required as a GitHub push blocker. | `gh secret list --repo <OWNER/REPO>` and, if applicable, `gh secret list --env <ENV> --repo <OWNER/REPO>`; confirm only required secrets remain. | Unblocked by owner confirmation: `invalid/non-live`. |

## How To Record Owner Confirmation

Use one of these statuses when collecting owner sign-off:

- `rotated`: the secret was replaced externally and the old value was revoked or made unusable.
- `invalid/non-live`: the value was never live, is already dead, or only existed in a sandbox that cannot reach production.
- `fixture-only`: the repo value is a test fixture or demo placeholder with no production reach.
- `blocked`: the owner cannot complete rotation yet, so the item remains open.

## Targeted Grep Record

Command run against `docs/operations`, `deploy/team-shared`, `scripts`, `deploy/docker`, and `web/src`, excluding the 2026-04-07 report:

```bash
rg -n -i --hidden --no-ignore-vcs --glob '!.git/**' --glob '!docs/reports/secret-scan-and-rotation-2026-04-07.md' '(REPLACE_WITH_TELEGRAM_BOT_TOKEN|REPLACE_WITH_GEMINI_API_KEY|REPLACE_WITH_DASHSCOPE_API_KEY|REPLACE_WITH_TOTP_SECRET|REPLACE_WITH_BREAKGLASS_TOKEN|REPLACE_WITH_DEX_CLIENT_SECRET|REPLACE_WITH_OPS_API_TOKEN|tars-shared-secret|JBSWY3DPEHPK3PXP|ops-token|192\\.168\\.3\\.106|root@192\\.168\\.3\\.100|StrictHostKeyChecking\\s*=?\\s*no|UserKnownHostsFile\\s*=?\\s*/dev/null|sshpass|PermitRootLogin|password_auth|passwordauth|authorized_keys)' docs/operations deploy/team-shared scripts deploy/docker web/src
```

Result summary:

- Live-secret style matches were limited to placeholders and template entries such as `REPLACE_WITH_OPS_API_TOKEN`, `REPLACE_WITH_TELEGRAM_BOT_TOKEN`, `REPLACE_WITH_DEX_CLIENT_SECRET`, `REPLACE_WITH_TOTP_SECRET`, `REPLACE_WITH_GEMINI_API_KEY`, and `REPLACE_WITH_DASHSCOPE_API_KEY`.
- `deploy/docker/dex.config.yaml` and `deploy/team-shared/dex.config.yaml` use the Dex client-secret placeholder.
- `deploy/team-shared/access.shared.yaml` keeps the token-login / TOTP placeholders only.
- `deploy/team-shared/connectors.shared.yaml` keeps JumpServer `access_key` / `secret_key` as secret refs, not inline literals.
- Remaining `192.168.3.106` and SSH/root mentions are historical docs, example data, or non-secret UI strings, not current-tree credential defaults in the audited paths.
- No private key block, bcrypt hash, or exact-format live provider token was found in the checked paths.

## Notes

- GitHub push is unblocked by owner confirmation that the previously discussed values were local-only / non-live. This is not the same as external rotation.
- No doc outside this tracker needed a contradiction fix during this pass.
