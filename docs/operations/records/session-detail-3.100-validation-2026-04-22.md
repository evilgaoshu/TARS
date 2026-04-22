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
- Screenshots:
  - `docs/operations/records/session-detail-3.100-2026-04-22/desktop-1440.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/mobile-390.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/desktop-1440-after-deploy.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/mobile-390-after-deploy.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/desktop-1440-after-relogin.png`
  - `docs/operations/records/session-detail-3.100-2026-04-22/mobile-390-after-relogin.png`
  - Playwright failure screenshot: `web/test-results/sessions-executions.smoke--dad6d-h-current-diagnosis-section/test-failed-1.png`
  - Playwright trace: `web/test-results/sessions-executions.smoke--dad6d-h-current-diagnosis-section/trace.zip`
- Result summary:
  - Local implementation checks passed.
  - Shared env service on `192.168.3.100:8081` was restarted with the new binary and web dist, and healthz returned OK.
  - Shared env browser verification is blocked by auth: the token that still works for `GET /api/v1/setup/status` no longer completes `/login?provider_id=local_token` and Playwright stays on `/login`.
  - Mobile overflow probe after deploy remained `0`, but full authenticated page acceptance could not be completed because the shared env rejected browser login.

## Notes

- Browser auth blocker details:
  - `POST /api/v1/auth/login` with `provider_id=local_token` returned `401` when replayed manually.
  - The official shared-lab smoke test reproduces the same blocker, so this is not a one-off manual browser issue.
  - Console evidence after the failed smoke login is captured in `web/test-results/.../error-context.md` and `test-failed-1.png`.
