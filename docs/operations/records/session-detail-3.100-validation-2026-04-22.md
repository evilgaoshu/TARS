# Session Detail 3.100 Validation 2026-04-22

Scope: `/sessions/95f49aeb-4312-4c31-a86a-07848e18ea66`

- Shared env base URL: `http://192.168.3.100`
- Desktop viewport: `1440x900`
- Mobile viewport: `390x844`
- Expected focus: current diagnosis, recommended next step, left timeline, expandable evidence summary, bottom-collapsed audit trace, no horizontal overflow.

## Evidence

- Command log:
  - `cd web && npm run test:unit -- tests/session-detail-react.test.tsx` -> PASS
  - `cd web && npm run build` -> PASS
  - `make check-mvp` -> PASS
  - `TARS_REMOTE_HOST=192.168.3.100 TARS_REMOTE_USER=root TARS_DEPLOY_SKIP_VALIDATE=1 bash scripts/deploy_team_shared.sh` -> binary + web dist synced, remote service restarted, `ssh root@192.168.3.100 'curl -fsS http://127.0.0.1:8081/healthz'` -> `{"status":"ok"}`
  - `cd web && TARS_PLAYWRIGHT_BASE_URL=http://192.168.3.100:8081 TARS_PLAYWRIGHT_TOKEN=$TARS_OPS_API_TOKEN npx playwright test tests/sessions-executions.smoke.spec.ts --grep '/sessions/:id shows session detail with current diagnosis section'` -> FAIL at login, app stayed on `/login`
  - `source scripts/lib/shared_ops_token.sh && export TARS_REMOTE_BASE_DIR=/data/tars-setup-lab && shared_ops_token_export >/tmp/tars_shared_token && TOKEN=$(cat /tmp/tars_shared_token) && curl -sS -D - -o /tmp/tars_setup_status_after.json -H "Authorization: Bearer ${TOKEN}" "http://192.168.3.100:8081/api/v1/setup/status"` -> `200 OK`
  - `source scripts/lib/shared_ops_token.sh && export TARS_REMOTE_BASE_DIR=/data/tars-setup-lab && shared_ops_token_export >/tmp/tars_shared_token && TOKEN=$(cat /tmp/tars_shared_token) && curl -sS -D - -o /tmp/tars_login_after.json -X POST "http://192.168.3.100:8081/api/v1/auth/login" -H "Content-Type: application/json" --data "{\"provider_id\":\"local_token\",\"token\":\"${TOKEN}\"}"` -> `200 OK`
  - `source scripts/lib/shared_remote_service.sh && shared_remote_service_restart "root@192.168.3.100" "/data/tars-setup-lab/team-shared" "/data/tars-setup-lab/bin/tars-linux-amd64-dev" "/data/tars-setup-lab/team-shared/tars-dev.log"` -> canonical shared-lab service restarted onto `/data/tars-setup-lab`
  - `browse goto http://192.168.3.100:8081/login?provider_id=local_token` + fill token + open `/sessions/95f49aeb-4312-4c31-a86a-07848e18ea66` + capture screenshots -> PASS
- Screenshots:
  - `docs/operations/records/session-detail-3.100-2026-04-22/desktop-1440.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/mobile-390.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/desktop-1440-after-deploy.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/mobile-390-after-deploy.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/desktop-1440-after-relogin.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/mobile-390-after-relogin.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/desktop-1440-restored.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/mobile-390-restored.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/desktop-1440-redeployed.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/mobile-390-redeployed.png`
  - Playwright failure screenshot: `web/test-results/sessions-executions.smoke--dad6d-h-current-diagnosis-section/test-failed-1.png`
  - Playwright trace: `web/test-results/sessions-executions.smoke--dad6d-h-current-diagnosis-section/trace.zip`
- Result summary:
  - Local implementation checks passed.
  - Shared env service on `192.168.3.100:8081` was initially serving the wrong runtime tree: `/root/tars-dev/bin/tars-linux-amd64-dev` with `/root/tars-dev/team-shared/shared-test.env`, whose `TARS_OPS_API_TOKEN` was still a placeholder.
  - Canonical shared-lab env remained healthy under `/data/tars-setup-lab/team-shared/shared-test.env`, which still held the real shared token.
  - Restarting the service onto the documented canonical tree `/data/tars-setup-lab` restored both bearer auth and `local_token` login.
  - Shared env browser verification then passed on the real session page for both `1440x900` and `390x844`, and the restored run reported no console errors.
  - The mobile viewport loaded without horizontal overflow in the authenticated session detail page.

## Fresh Runtime Check

- Checked at: `2026-04-22` after PR `#5` was already open.
- Remote process:
  - `ssh root@192.168.3.100 'ps -ef | grep tars-linux-amd64-dev | grep -v grep'`
  - active pid: `3636837`
- Binary path:
  - `ssh root@192.168.3.100 'readlink -f /proc/3636837/exe'`
  - result: `/data/tars-setup-lab/bin/tars-linux-amd64-dev`
- Working directory:
  - `ssh root@192.168.3.100 'pwdx 3636837'`
  - result: `/root`
- Listener:
  - `ssh root@192.168.3.100 'lsof -iTCP:8081 -sTCP:LISTEN -n -P'`
  - result: pid `3636837` listening on `*:8081`
- Effective env/config paths from `/proc/3636837/environ`:
  - `TARS_SERVER_LISTEN=0.0.0.0:8081`
  - `TARS_WEB_DIST_DIR=/data/tars-setup-lab/web-dist`
  - `TARS_PROVIDERS_CONFIG_PATH=/data/tars-setup-lab/team-shared/providers.shared.yaml`
  - `TARS_CONNECTORS_CONFIG_PATH=/data/tars-setup-lab/team-shared/connectors.shared.yaml`
  - `TARS_SKILLS_CONFIG_PATH=/data/tars-setup-lab/team-shared/skills.shared.yaml`
  - `TARS_AUTOMATIONS_CONFIG_PATH=/data/tars-setup-lab/team-shared/automations.shared.yaml`
  - `TARS_SECRETS_CONFIG_PATH=/data/tars-setup-lab/team-shared/secrets.shared.yaml`
  - `TARS_ACCESS_CONFIG_PATH=/data/tars-setup-lab/team-shared/access.shared.yaml`
- Fresh endpoint checks:
  - `GET /api/v1/setup/status` with canonical shared token -> `200 OK`
  - `POST /api/v1/auth/login` with `{"provider_id":"local_token","token":"<shared token>"}` -> `200 OK`
  - `GET /sessions/95f49aeb-4312-4c31-a86a-07848e18ea66` -> `200 OK`
- Conclusion:
  - `192.168.3.100:8081` is currently served by the canonical shared-lab instance under `/data/tars-setup-lab`, not `/root/tars-dev`.
  - The deployed session detail page still corresponds to the feature implementation commit `3f3252596dceba94a3932ac22adbb28a776d7075`, which was the PR `#5` head when the restored runtime/browser verification was captured. The follow-up evidence commit is docs-only and does not change the deployed page behavior.

## Redeploy Verification

- Root cause update:
  - Source still had two real IA gaps: `web/src/pages/sessions/SessionDetail.tsx` still rendered `Service / Host / Last update / Executions` inside Current diagnosis, and `web/src/locales/en-US.json` still carried the old description copy.
  - Live 3.100 also served stale web assets: `/data/tars-setup-lab/web-dist/index.html` still pointed at `index-DI6ml4bX.js`, and the page loaded `SessionDetail-CwCLjup3.js`.
- Fixes applied:
  - Removed the repeated kicker row from Current diagnosis.
  - Updated English copy to match the reduced operator summary and tool-plan evidence table.
  - Rebuilt `web/dist`, cleared stale files under `/data/tars-setup-lab/web-dist`, and copied the fresh dist back to the canonical shared-lab path.
- Fresh verification commands:
  - `cd web && npm run test:unit -- tests/session-detail-react.test.tsx` -> PASS
  - `cd web && npm run build` -> PASS
  - `ssh root@192.168.3.100 'python3 - <<'"'"'PY'"'"' ... unlink stale /data/tars-setup-lab/web-dist files ... PY' && scp -r web/dist/. root@192.168.3.100:/data/tars-setup-lab/web-dist/` -> PASS
  - `browse goto http://192.168.3.100:8081/login?provider_id=local_token&next=%2Fsessions%2F95f49aeb-4312-4c31-a86a-07848e18ea66` + token login + capture screenshots -> PASS
- Fresh deployed asset hashes:
  - `index.html` now points to `index-CLQv2fnS.js`
  - Session detail chunk loaded in browser network log: `SessionDetail-U2JxxYNQ.js`
- Fresh browser results:
  - Desktop screenshot: `docs/operations/records/session-detail-3.100-2026-04-22/desktop-1440-redeployed.png`
  - Mobile screenshot: `docs/operations/records/session-detail-3.100-2026-04-22/mobile-390-redeployed.png`
  - Current diagnosis no longer repeats `Service / Host / Last update / Executions`.
  - Empty `Knowledge context` and `Linked executions` sections are hidden on the live page.
  - Timeline remains left of the evidence summary on desktop, and mobile view shows no horizontal overflow in the captured viewport.

## Notes

- Root cause details:
  - The live listener on `:8081` had drifted to `/root/tars-dev/bin/tars-linux-amd64-dev`, while the documented canonical shared lab for `192.168.3.100` is `/data/tars-setup-lab`.
  - `/root/tars-dev/team-shared/shared-test.env` still held `TARS_OPS_API_TOKEN=REPLACE_WITH_OPS_API_TOKEN`, so the running process rejected both bearer auth and `local_token` fallback.
  - The canonical `/data/tars-setup-lab/team-shared/shared-test.env` still held the real shared token, which is why the token helper and the live process had diverged.
  - No product-code change was required for this fix; the issue was shared-env process/path drift.
