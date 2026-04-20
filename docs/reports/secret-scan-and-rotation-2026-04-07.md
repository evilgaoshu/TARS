# Secret Scan And Rotation Report

Date: 2026-04-07, refreshed 2026-04-08 after publishable-tree cleanup.

Scope: practical local scan of the current publishable working tree under `/Users/yue/TARS`. I excluded obvious non-publishable or generated/local state paths: `.git`, local agent/browser state, `.alma-snapshots`, `.codex-tmp`, `.claude`, `.gemini`, `.playwright-cli`, `.superpowers`, `data`, dependency folders, build outputs, `web/dist`, binaries, caches, screenshots/images, and other generated artifacts.

Important git context: `git ls-files` returned `0` tracked files, so the scan treated the visible working tree as the publishable tree rather than relying on tracked files.

## Gate

DO NOT PUSH to GitHub until:

- the current-tree credential findings below are remediated or explicitly accepted by the repo owner as non-secret demo defaults, and
- every credential class in the rotation-required checklist has been rotated externally or explicitly accepted by its owner as already invalid/non-live.

This worker did not rotate external secrets, did not commit, and did not push.

## Scanner Availability

Dedicated local scanners were not available:

```bash
for tool in gitleaks trufflehog detect-secrets git-secrets; do
  if command -v "$tool" >/dev/null 2>&1; then
    printf '%s %s\n' "$tool" "$(command -v "$tool")"
  else
    printf '%s NOT_FOUND\n' "$tool"
  fi
done
```

Result:

- `gitleaks NOT_FOUND`
- `trufflehog NOT_FOUND`
- `detect-secrets NOT_FOUND`
- `git-secrets NOT_FOUND`

Because no dedicated scanner was installed, I used targeted `rg`/regex scans. Limitation: this is not equivalent to entropy/history scanning. It does not inspect Git history beyond the current working tree, and it can miss novel provider formats.

## Commands Run And Results

Publishable file count after exclusions:

```bash
rg --files --hidden --no-ignore-vcs \
  --glob '!.git/**' --glob '!**/.git/**' --glob '!**/node_modules/**' \
  --glob '!**/vendor/**' --glob '!**/dist/**' --glob '!**/bin/**' \
  --glob '!**/build/**' --glob '!**/target/**' --glob '!**/coverage/**' \
  --glob '!**/.cache/**' --glob '!**/.next/**' --glob '!**/.venv/**' \
  --glob '!**/venv/**' --glob '!**/.alma-snapshots/**' \
  --glob '!**/.codex-tmp/**' --glob '!**/.claude/**' \
  --glob '!**/.gemini/**' --glob '!**/.playwright-cli/**' \
  --glob '!**/.superpowers/**' --glob '!**/data/**' \
  --glob '!**/.DS_Store' --glob '!**/*.png' --glob '!**/*.jpg' \
  --glob '!**/*.jpeg' --glob '!**/*.gif' --glob '!**/*.ico' | wc -l
```

Result: `564` files scanned.

Dedicated provider/token-format scan:

```bash
rg -n --hidden --no-ignore-vcs [same exclusions] \
  '(AIza[0-9A-Za-z_-]{35}|[0-9]{8,10}:[A-Za-z0-9_-]{20,}|sk-[A-Za-z0-9_-]{20,}|sk-ant-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}|AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|ya29\.[A-Za-z0-9_-]{20,})' .
```

Result: no real provider tokens found. One benign false positive matched a test filename containing `sk-` text.

Private-key scan:

```bash
rg -n --hidden --no-ignore-vcs [same exclusions] \
  '(-----BEGIN (RSA |DSA |EC |OPENSSH |PGP )?PRIVATE KEY-----|-----BEGIN OPENSSH PRIVATE KEY-----|BEGIN PRIVATE KEY)' .
```

Result: no private key blocks found.

Literal credential assignment scan:

```bash
rg -n -i --pcre2 --hidden --no-ignore-vcs [same exclusions] \
  '\b(api[_-]?key|bot[_-]?token|webhook[_-]?secret|bearer[_-]?token|client[_-]?secret|password(?:_hash)?|totp[_-]?secret|access[_-]?key|secret[_-]?key|token|secret)\b\s*[:=]\s*["'\''"]?(?!\$\{|<|REPLACE|replace|changeme|example|dummy|redacted|__|\s*$)[A-Za-z0-9_./:+@-]{6,}' .
```

Result: mostly test fixtures, placeholders, and code field names. Real current-tree review items from the non-test publishable tree are listed below.

Known legacy/default value scan:

```bash
rg -n -i --hidden --no-ignore-vcs [same exclusions plus test/spec/project exclusions] \
  '(REPLACE_WITH_TELEGRAM_BOT_TOKEN|REPLACE_WITH_GEMINI_API_KEY|REPLACE_WITH_DASHSCOPE_API_KEY|REPLACE_WITH_TOTP_SECRET|REPLACE_WITH_BREAKGLASS_TOKEN|REPLACE_WITH_DEX_CLIENT_SECRET|REPLACE_WITH_OPS_API_TOKEN|tars-shared-secret|JBSWY3DPEHPK3PXP|ops-token)' \
  deploy docs scripts configs .github web/src internal migrations Makefile README.md CONTRIBUTING.md CHANGELOG.md CLAUDE.md
```

Result:

- `deploy/team-shared` now uses `REPLACE_WITH_*` placeholders for ops token, Telegram token, Dex client secret, password hash, TOTP seed, Gemini key, and DashScope key.
- `deploy/docker/dex.config.yaml` now uses placeholders for the Dex client secret and bcrypt password hash.
- `deploy/docker/docker-compose.yml` now requires `TARS_OPS_API_TOKEN` injection instead of falling back to a legacy default.
- Shared-environment smoke/live scripts now require `TARS_OPS_API_TOKEN`; `scripts/validate_auth_enhancements_live.sh` also requires password and TOTP secret injection.
- Residual `ops-token` matches are code/test fixtures, historical docs, or the break-glass concept label, not publishable runtime defaults in the audited Docker/shared scripts.

Bcrypt hash scan:

```bash
rg -n --hidden --no-ignore-vcs [same exclusions] '\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53}' .
```

Result after cleanup: no bcrypt hash was found outside this report.

Root SSH / host-key bypass scan:

```bash
rg -n -i --hidden --no-ignore-vcs [same exclusions] \
  '(^|[^A-Za-z0-9_])(root@|StrictHostKeyChecking\s*=?\s*no|UserKnownHostsFile\s*=?\s*/dev/null|PermitRootLogin|sshpass|password_auth|passwordauth|authorized_keys)' .
```

Result after cleanup: the current team-shared and team-dev docs no longer normalize `root@192.168.3.100`; residual root SSH or host-key-bypass matches are older reports, troubleshooting/security guide examples, tests, or implementation code for the explicit SSH override path. They are not credentials, but the security owner should still review the broader docs before public launch.

## Current-Tree Findings

| Finding | Location | Risk | Required action before GitHub push |
| --- | --- | --- | --- |
| External rotation still required | Telegram / model provider / Dex / Ops API / TOTP / SSH owners | Current tree cleanup does not invalidate credentials that may have existed before sanitization. | Rotate or owner-accept every credential class in the checklist below before repo visibility expands. |
| Dedicated entropy/history scanner still missing | local workstation | `rg` scans are targeted and current-tree only; they do not replace `gitleaks`/`trufflehog` history or entropy scans. | Run a real scanner before first push if available; otherwise keep the documented targeted scan as a minimum gate. |
| Broader legacy doc cleanup still useful | older reports/guides/project notes | Some historical docs still mention old `192.168.3.106`, `ops-token`, root SSH, or host-key-bypass examples. These are mostly history or conceptual references, but can confuse future readers. | Keep them out of quick-start/free-path docs, or explicitly mark them as historical examples. |

No current-tree private key blocks or exact-format Telegram/Gemini/DashScope/GitHub/AWS/Slack/OpenAI/Anthropic token patterns were found in the publishable scan scope.

## Final Targeted Sanity Scan

After writing this report, I ran one final targeted `rg` pass and excluded this report file itself to avoid self-matches:

```bash
rg -n --pcre2 --hidden --no-ignore-vcs \
  --glob '!.git/**' --glob '!**/.git/**' --glob '!**/node_modules/**' \
  --glob '!**/vendor/**' --glob '!**/dist/**' --glob '!**/bin/**' \
  --glob '!**/build/**' --glob '!**/target/**' --glob '!**/coverage/**' \
  --glob '!**/.cache/**' --glob '!**/.next/**' --glob '!**/.venv/**' \
  --glob '!**/venv/**' --glob '!**/.alma-snapshots/**' \
  --glob '!**/.codex-tmp/**' --glob '!**/.claude/**' \
  --glob '!**/.gemini/**' --glob '!**/.playwright-cli/**' \
  --glob '!**/.superpowers/**' --glob '!**/data/**' \
  --glob '!**/.DS_Store' --glob '!**/*.png' --glob '!**/*.jpg' \
  --glob '!**/*.jpeg' --glob '!**/*.gif' --glob '!**/*.ico' \
  --glob '!docs/reports/secret-scan-and-rotation-2026-04-07.md' \
  '(AIza[0-9A-Za-z_-]{35}|[0-9]{8,10}:[A-Za-z0-9_-]{20,}|sk-[A-Za-z0-9_-]{20,}|sk-ant-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}|AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|ya29\.[A-Za-z0-9_-]{20,}|\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53}|tars-shared-secret|JBSWY3DPEHPK3PXP|ops-token)' .
```

Result summary after the 2026-04-08 cleanup:

- The prior push-blocking current-tree items were remediated: Docker Dex config uses placeholders, Docker Compose requires `TARS_OPS_API_TOKEN`, shared smoke/live scripts require injected tokens, Playwright smoke no longer falls back to `ops-token`, and the live auth script requires password/TOTP injection.
- Remaining matches are expected code/test fixtures, historical docs/project notes, or break-glass concept labels.
- No private keys, no bcrypt hashes outside this report, and no exact-format Telegram/Gemini/DashScope/GitHub/AWS/Slack/OpenAI/Anthropic provider tokens were found by the targeted current-tree pass.

## Rotation-Required Checklist

Treat the following as exposed because the repo previously contained credential values before sanitization. Do not rely on current placeholders alone.

| Credential class | Owner | Known repo/current locations to review | Rotation / acceptance action | Post-rotation verification |
| --- | --- | --- | --- | --- |
| Ops API / break-glass token | Platform/Ops owner | `deploy/team-shared/shared-test.env`, `deploy/docker/docker-compose.yml`, `scripts/ci/*.sh`, `scripts/validate_auth_enhancements_live.sh`, ops/login docs | Generate a new high-entropy token and inject it via machine-local env or an external secret manager. Remove hardcoded defaults or mark them invalid. | New token can call `/api/v1/setup/status`; old token returns unauthorized/forbidden; smoke scripts work only when env token is provided. |
| Telegram bot token | Integration/Channel owner | historical `deploy/team-shared/shared-test.env`, Telegram runtime env, pilot/deploy docs | Rotate via Telegram BotFather or create a replacement bot, then update only runtime secret injection. | `getMe`/polling/send succeeds with the new token; old token fails; no bot token in repo scan. |
| Telegram webhook secret | Integration/Channel owner | `TARS_TELEGRAM_WEBHOOK_SECRET`, webhook docs/scripts | Generate a new webhook secret if webhook mode was ever used. | Webhook callback with old secret is rejected; callback with new secret is accepted. |
| VMAlert webhook secret | Observability owner | `TARS_VMALERT_WEBHOOK_SECRET`, deploy/team-shared placeholder, demo acceptance script | Generate a new webhook secret and inject it outside repo. | Old webhook secret is rejected; new smoke alert is accepted and creates a session. |
| Gemini API key | Model/AI owner | historical `deploy/team-shared/secrets.shared.yaml`, `providers.shared.yaml` secret ref | Revoke/regenerate in Google AI Studio or the owning Google Cloud project. Store only by secret ref. | Provider probe succeeds with new key; old key is revoked/disabled; no Gemini key pattern in repo scan. |
| DashScope API key | Model/AI owner | historical `deploy/team-shared/secrets.shared.yaml`, `providers.shared.yaml` secret ref | Revoke/regenerate in Alibaba Cloud/DashScope. Store only by secret ref. | Provider probe succeeds with new key; old key is revoked/disabled; no DashScope-like key in repo scan. |
| Dex / OIDC client secret | IAM owner | historical `deploy/team-shared/dex.config.yaml`, `deploy/team-shared/access.shared.yaml`, template placeholder in `deploy/docker/dex.config.yaml` | Generate a new Dex client secret and inject it outside the repo. | OIDC login succeeds with new secret; old client secret fails; repo scan has no literal Dex client secret. |
| Dex/local-password hash | IAM owner | historical `deploy/team-shared/dex.config.yaml`, `deploy/team-shared/access.shared.yaml`, template placeholder in `deploy/docker/dex.config.yaml` | Change the demo/local password and regenerate the bcrypt hash outside the repo. | Old password fails; new password login works; repo scan has no bcrypt hash in publishable config. |
| Local token auth provider client secret | IAM owner | historical `deploy/team-shared/access.shared.yaml` | Generate a new client secret and inject it outside repo. | Login with old local token fails; new local token issues a session; config export redacts the secret. |
| TOTP seed | IAM owner | historical `deploy/team-shared/access.shared.yaml`, `scripts/validate_auth_enhancements_live.sh` now requires env injection | Re-enroll MFA for the shared admin/test user and inject the active seed only from local/private secret storage. | Old TOTP codes fail; new TOTP codes pass; repo scan has no active seed. |
| JumpServer connector access/secret keys | Connector/Execution owner | `deploy/team-shared/connectors.shared.yaml` secret refs, external secret store | If real keys ever appeared in the repo or shared secret file, revoke/regenerate them in JumpServer and update secret storage only. | Connector health passes with new keys; old keys fail; no inline `access_key`/`secret_key` values in publishable configs. |
| SSH private key / root access material | Ops/Infra owner | `deploy/team-shared/shared-test.env`, deploy docs/scripts | No private key block was found. If any key path corresponded to a shared key that was ever committed or copied broadly, re-key it and avoid root SSH defaults. | SSH works with the new key and known_hosts; old key is removed; host-key checking remains enabled by default. |
| GitHub/deploy tokens | Repo/CI owner | `.github/`, local helpers | None found in exact-format scan. If any were used locally or appear in history later, revoke and recreate with minimum scope. | GitHub secret inventory has only required values; scan has no `gh*` tokens. |

## 2026-04-08 Publishable-Tree Recheck

After the publishable baseline review, I added `/.claude/` to the repo ignore rules and re-ran the targeted grep pass. The result did not change the secret findings:

- No private keys or live provider tokens were found in the publishable scan scope.
- Remaining matches are placeholders, historical docs, example data, or explicit break-glass concept labels.
- `deploy/team-shared/*.local.env` and `deploy/team-shared/*.private.env` remain the intended home for local-only or machine-private values.

## Follow-Up Scan Before First Commit/Push

Run a real scanner if one is installed by then:

```bash
gitleaks detect --source . --no-git --redact --verbose
```

If no scanner is installed, at minimum run this targeted sanity scan again:

```bash
rg -n --pcre2 --hidden --no-ignore-vcs \
  --glob '!.git/**' --glob '!**/.git/**' --glob '!**/node_modules/**' \
  --glob '!**/vendor/**' --glob '!**/dist/**' --glob '!**/bin/**' \
  --glob '!**/build/**' --glob '!**/target/**' --glob '!**/coverage/**' \
  --glob '!**/.cache/**' --glob '!**/.next/**' --glob '!**/.venv/**' \
  --glob '!**/venv/**' --glob '!**/.alma-snapshots/**' \
  --glob '!**/.codex-tmp/**' --glob '!**/.claude/**' \
  --glob '!**/.gemini/**' --glob '!**/.playwright-cli/**' \
  --glob '!**/.superpowers/**' --glob '!**/data/**' \
  --glob '!**/.DS_Store' --glob '!**/*.png' --glob '!**/*.jpg' \
  --glob '!**/*.jpeg' --glob '!**/*.gif' --glob '!**/*.ico' \
  '(AIza[0-9A-Za-z_-]{35}|[0-9]{8,10}:[A-Za-z0-9_-]{20,}|sk-[A-Za-z0-9_-]{20,}|sk-ant-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}|AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|ya29\.[A-Za-z0-9_-]{20,}|\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53}|tars-shared-secret|JBSWY3DPEHPK3PXP|ops-token)' .
```

Expected result before push: no live provider tokens, no private keys, no bcrypt hashes in publishable config, no active Dex/client/local/TOTP defaults, and no owner-unaccepted legacy ops-token fallback.
