# JumpServer Primary Execution Runtime Closure

## Goal

Close EVI-22 by turning the current `jumpserver-main` 403 into an auditable runtime decision:

- path A: `jumpserver-main` becomes the real primary execution runtime and completes `approval -> execution -> verifier -> resolved` on `192.168.3.100`
- path B: TARS records a verified JumpServer blocker, keeps `ssh-main` as the controlled executable runtime, and waits for owner/admin acceptance before claiming primary-path closure

## Root Cause Summary

Fresh shared-lab investigation on `192.168.3.100` shows:

- `GET /api/v1/assets/hosts/?limit=1` with the configured JumpServer AK/SK succeeds, so API reachability and signing are valid.
- `POST /api/v1/ops/jobs/` with the same AK/SK returns `403 {"detail":"命令执行已禁用","code":"permission_denied"}`.
- `POST /api/v1/ops/command/` returns `404 {"error":"Not found"}` on the target JumpServer instance.

Conclusion: this is not just a TARS-side endpoint typo. The current JumpServer instance exposes the jobs API path, but the configured identity cannot execute commands on the target asset under current policy/permission settings.

## Runtime Health Semantics

TARS must distinguish two different truths:

1. API health: JumpServer credentials can authenticate and read host metadata.
2. Execution health: JumpServer can actually submit and complete approved commands for the target asset.

For `jumpserver-main`, API health alone is not enough to make it the primary execution runtime.

Rules:

- `jumpserver-main` API probe success records `degraded`, not `healthy`, with summary `jumpserver API probe succeeded; execution not yet verified`.
- `jumpserver-main` only becomes execution-primary after a real execution succeeds and records `execution runtime completed successfully`.
- `ssh-main` remains the auto-selected execution and verification runtime while JumpServer is still API-only or policy-blocked.

## Fallback Selection Logic

Execution runtime selection order:

1. Prefer enabled, compatible, healthy execution connectors.
2. Treat `jumpserver_api` with API-only probe summary as non-primary, even if the last connector health endpoint returned `healthy` before this fix.
3. If `ssh-main` is healthy, select `ssh-main` as the primary runtime.
4. If no healthy execution connector is available, fall back to legacy `ssh` runtime metadata with `fallback_reason=no_healthy_connector_selected`.

Verification runtime follows the same selection contract as execution runtime.

## Decision Tree

1. Run JumpServer connector health.
2. If config/secrets/signing fail, TARS-side issue, fix in repo or shared config.
3. If API probe succeeds, run direct execution probe on `jumpserver-main`.
4. If execution succeeds, path A: redeploy, capture approval/execution/verifier/resolved evidence, promote JumpServer to primary.
5. If execution fails with `permission_denied`, inspect JumpServer policy/authorization scope:
   - command filter
   - asset permission for `192.168.3.9 dev`
   - AK/SK user ops permission
6. If the fix requires JumpServer admin/owner action, stop at path B and produce blocker evidence plus fallback wording.

## A/B Acceptance Matrix

| Path | Required evidence | Acceptance bar |
| --- | --- | --- |
| A. JumpServer primary restored | `192.168.3.100` run proves `jumpserver-main` completes approval, execution, verifier, final `resolved`; TARS `setup/status` shows `jumpserver-main` as primary because of real execution success; spec + record + CI status committed | DEV can hand to QA/PR as runtime-closed |
| B. JumpServer blocker documented | Fresh probe shows API-read success plus command-execution denial; TARS keeps `ssh-main` primary after health probe; spec + blocker record + fallback recommendation committed; owner/admin still must ACK policy acceptance | Valid DEV closure candidate, but not final release signoff until owner ACK |

## Shared-Lab Evidence Requirements

Every EVI-22 record in `docs/operations/records` must include:

- branch / PR / head commit
- deployed runtime commit on `192.168.3.100`
- runtime identity or config paths
- JumpServer connector health result
- direct execution probe command and result
- `session_id`, `execution_id`, approval result, execution output or spool path, verifier result, final session state when applicable
- explicit separation of GitHub CI status vs `192.168.3.100` shared-lab status

## Current Recommendation

Until JumpServer admin/owner enables command execution for the configured identity and target asset, treat `ssh-main` as the only executable primary runtime in the shared lab.

Keep `jumpserver-main` configured for auditability and future revalidation, but do not let API-only health re-promote it into the primary execution slot.
