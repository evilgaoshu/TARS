# GitHub Migration Execution Prep Runbook

> Pre-migration only. No external secret rotation, GitHub push, or deployment cutover happens in this step.

## Goal

Turn the current research into a short, executable prep sequence for the GitHub move and the environment-hardening workstream.
Use `docs/operations/github_publishable_baseline.md` as the local keep-track vs keep-local review gate before the first commit.

## Non-goals

- Do not rotate external secrets in this pass.
- Do not deploy or cut over runtime traffic in this pass.
- Do not widen shared-environment exposure before the publishable tree is clean.

## 1. Migration Prep Checklist

- [ ] Confirm the tracked baseline exists and is the version we want to migrate from.
- [x] Inventory the publishable tree and mark anything that must stay shared-only or local-only.
- [x] Review `docs/operations/github_publishable_baseline.md` and resolve every keep-track vs keep-local item before any first commit.
- [x] Separate CI-safe checks from shared-machine checks.
- [x] Template or sanitize `deploy/team-shared` before any broader GitHub exposure.
- [x] Pin GitHub Actions to commit SHAs and add explicit workflow permissions.
- [ ] Decide the branch policy for the GitHub move: protected `main`, PR-only changes, and required checks (`MVP Checks`, `Security Regression`, `Secret Scan`, `Static Demo Build`).
- [x] Make sure current quick-start/free-path docs do not imply root SSH, host-key bypassing, or persistent-machine assumptions.

## 2. Secrets Cleanup And Rotation Checklist

Use this checklist to prepare the rotation window. The actual rotation still happens later.

### Secret classes to inventory

| Secret class | Typical places to check | Prep action |
| --- | --- | --- |
| Telegram bot token | `deploy/team-shared/shared-test.env`, scripts, docs | mark for rotation and move to secret storage |
| Ops API token | `deploy/team-shared/shared-test.env`, runbooks, smoke scripts | replace checked-in values with injection-only references |
| OIDC / Dex credentials | `deploy/team-shared/dex.config.yaml`, access docs | prepare replacement credentials and owner list |
| Model API keys | provider configs, env files, deployment scripts | separate demo-only keys from shared-environment keys |
| Provider credentials | `deploy/team-shared/*.yaml`, example configs | remove inline values from publishable examples |
| SSH material | `deploy/team-shared/shared-test.env`, deploy scripts | confirm no free-path dependency on root SSH |
| GitHub tokens / deploy tokens | CI config, local helpers | scope to the smallest possible GitHub secret or environment secret |

### Rotation prep steps

- [ ] List every file and script that currently reads the secret.
- [ ] Decide the target storage for each secret: GitHub secret, GitHub environment secret, external secret manager, or machine-local injection.
- [ ] Mark which secrets must be rotated before repo visibility expands.
- [x] Remove current-tree secret-bearing examples from docs, templates, and sample configs.
- [x] Verify that demo/smoke-only paths require explicit credential injection instead of checked-in defaults.
- [ ] Record the owner for each secret class so the actual rotation can be executed without guesswork.
- [ ] Decide whether a free-tier preview path (GitHub Pages or Supabase Free) is needed, and keep it out of required checks.

## 3. Exit Criteria

- [x] No checked-in runtime secret remains in the audited publishable Docker/shared-script path.
- [x] `deploy/team-shared` is template-safe for GitHub exposure.
- [x] Current-tree findings from `docs/reports/secret-scan-and-rotation-2026-04-07.md` are remediated or explicitly accepted by the responsible owner.
- [x] GitHub Actions is limited to the CI baseline, not deploy or shared-host access.
- [x] Free-environment boundaries are documented and acknowledged.
- [x] The secret worker has acknowledged scan and rotation status.
- [ ] A separate rotation window is approved for the actual key changes.

## 4. Next Execution Step

Next, run the external rotation window for credential classes in `docs/reports/secret-scan-and-rotation-2026-04-07.md`, then create the first local GitHub tracked baseline. Do not push until the rotation owners confirm either “rotated” or “already invalid / fixture-only.”

## 5. Push-Readiness Assessment (2026-04-11)

### Local verification results

| Check | Result |
| --- | --- |
| `make secret-scan` | passed -- 499 publishable non-test files scanned, 0 matches |
| `make security-regression` | passed -- 19 test groups, all PASS |
| `make pre-check` | passed -- Go compile + OpenAPI validation |
| `make check-mvp` | passed -- Go tests, coverage, build, OpenAPI, web lint, web build |
| `make static-demo-build` | passed -- frontend static artifact builds cleanly, no runtime secrets |

### What was fixed in this pass

- `.gitignore`: added `.alma/` to the "Local agent / browser state" block.
- `scripts/ci/secret-scan.sh`: expanded from 3 test-fixture patterns to include PEM private key headers, GitHub token format (`gh[pousr]_...`), and AWS access key ID format (`AKIA...`). JSX `placeholder=` attributes correctly exempted.
- `scripts/ci/secret-scan.sh`: expanded scan scope from runtime-only roots to the publishable non-test tree (`docs/`, `scripts/`, `project/`, `specs/`, `web/`, repo metadata); historical archives under `docs/reports/` and `docs/operations/records/` plus test fixtures stay on the human-review path to avoid noisy false positives.
- `scripts/ci/secret-scan-regression.sh`: added a functional regression that proves publishable docs/scripts are scanned while test fixtures and historical archives remain excluded.
- `web/tests/first-run-setup-e2e.test.tsx`: removed unused `_params` parameter that caused lint error.
- `web/tests/ops-action-view-react.test.tsx`: replaced `as any` with typed `SSHCredential & { secret_ref?: string }`.
- `docs/operations/github_actions_baseline_scope.md`: marked Minimum Workflow Rules and Review Gate as verified.
- `docs/operations/github_publishable_baseline.md`: added `.alma/` to "Must Stay Local Or Ignored" list.

### What still blocks pushing to GitHub

| Blocker | Type | Owner |
| --- | --- | --- |
| Branch protection policy not yet decided | Human decision | Team |
| Separate rotation window not yet approved | Human coordination | Secret worker / owners |
| Confirm tracked baseline is the intended migration version | Human sign-off | Team |

### What does NOT block pushing (technical gate is clear)

- No runtime secrets in the machine-scannable publishable non-test tree; historical reports/records and test fixtures are explicitly left on the human-review path.
- All GitHub Actions are scoped to L0/L1/L2/static-demo; no SSH, no deploy, no shared-machine access.
- All Actions are SHA-pinned with `permissions: contents: read`.
- All 5 CI checks pass locally and are reproducible on GitHub-hosted runners.
