# EVI-13 Pilot Go/No-Go Evidence Pack

## Goal

Turn the current pilot-ready MVP into a reviewable go/no-go evidence pack backed by fresh `192.168.3.100` runtime validation.

## In Scope

- Capture fresh `192.168.3.100` evidence for health, readiness, rollout mode, provider/channel status, and pilot hygiene.
- Validate one alert-driven sample and one Telegram conversational sample against the shared-lab runtime.
- Record the effective shared-lab config snapshot for providers, SSH allowlist, Telegram, VictoriaMetrics, VictoriaLogs, authorization, approval routing, and rollout mode.
- Prefill the pilot decision gate with the evidence that is actually collectible now.
- Update the MVP checklist go/no-go section with pass/fail/gap notes.

## Hard Deliverables

1. `specs/evi-13-pilot-go-no-go-evidence-pack.md`
2. `docs/operations/records/evi-13-alert-e2e-evidence-20260423.md`
3. `docs/reports/pilot-decision-gate-evi13-prefill.md`
4. Updated `docs/operations/mvp_completion_checklist.md` section 5 status

## Shared-Lab Execution Plan

1. Run baseline checks on `192.168.3.100`
   - `GET /healthz`
   - `GET /readyz`
   - `GET /api/v1/setup/status`
   - `bash scripts/pilot_hygiene_check.sh`
2. Exercise the alert-driven path with `POST /api/v1/smoke/alerts` and record the resulting session.
3. Exercise the Telegram conversational path through `/api/v1/channels/telegram/webhook`
   - missing-host message should return guidance and create no session when multiple hosts are configured
   - explicit-host message should create a `telegram_chat` session
4. Capture the live process config snapshot from `/proc/<pid>/environ` and `/data/tars-setup-lab/team-shared/shared-test.env`.
5. Run CI-aligned verification
   - `make check-mvp`
   - `make smoke-remote`
   - `make live-validate`

## Acceptance Rules

### Required for a full go

- Shared-lab runtime must stay healthy and ready.
- Alert-driven path must produce `diagnosis -> approval -> execution -> verifier -> resolved` with real `session_id`, `execution_id`, spool path, and verification success.
- Telegram conversational path must show both:
  - missing-host guidance with no session creation
  - explicit-host session creation and an execution-capable flow when the request calls for one
- Shared-lab config must use a real SSH allowlist instead of placeholder values.

### Allowed to ship as evidence-only closeout

- If live provider or Telegram credentials are invalid, record the exact blocker with commands and observed runtime state.
- If a repo-side baseline issue is found, fix it with the smallest change and redeploy before writing the final record.

## Observed Risk Areas

- `lmstudio-local` may be unreachable from `192.168.3.100`.
- `gemini-backup` may have an invalid API key.
- `telegram-main` may still be running with a placeholder bot token, which allows stub delivery but blocks real approval interaction.
- Shared env placeholders in `shared-test.env` can silently break acceptance behavior even when the repo code is correct.

## Non-Goals

- No production rollout.
- No new post-MVP platform capability.
- No decision here on long-term `knowledge / vector / outbox` investment depth.
