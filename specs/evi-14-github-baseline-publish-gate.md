# EVI-14 GitHub Baseline Publish Gate

## Background and goal

This spec turns the GitHub baseline publish gate for EVI-14 into versioned project documentation.
The goal of this pass is to record the exact tracked baseline candidate, required GitHub branch protection recommendations, required checks boundaries, and the first-baseline runbook without performing any real tag creation, push, cutover, or GitHub settings changes.

## Current evidence

### CI baseline evidence

- `.github/workflows/mvp-checks.yml` defines the GitHub-safe baseline workflow with these exact job names:
  - `L0 Pre-check`
  - `L1 MVP Checks`
  - `L2 Security Regression`
  - `Secret Scan`
  - `Static Demo Build`
- `docs/operations/github_actions_baseline_scope.md` already states that GitHub-hosted runners are validation-only and that shared-host deploy or live validation remains out of scope.
- `docs/operations/github_migration_prep_runbook.md` records local verification from 2026-04-11 for:
  - `make pre-check`
  - `make check-mvp`
  - `make security-regression`
  - `make secret-scan`
  - `make static-demo-build`

### Rotation and credential evidence

- `docs/operations/records/credential_rotation_execution_tracker_2026-04-08.md` records owner confirmation on 2026-04-08 that all 12 credential classes were `invalid/non-live`.
- Based on that owner confirmation, there is no engineering blocker for a separate external credential rotation window in this baseline publish-gate pass.

### Tracked baseline evidence

- Local repository `main` currently resolves to exact commit `3557aa1dabe95bf0f0afc2e3b09110e754cba8ee`.
- Local repository `origin/main` currently resolves to exact commit `2ab7be8f9a28eeb81d613d8da57544a758e72108`.
- Because the documented goal is the GitHub baseline publish gate, this spec treats `origin/main` commit `2ab7be8f9a28eeb81d613d8da57544a758e72108` as the tracked baseline candidate to sign off for the first GitHub baseline.
- The local `main` divergence is not treated as an engineering blocker for this doc-only pass, but owner sign-off must confirm that the remote-tracked candidate stays the intended first baseline before any real tag or push action.

## Final decisions

1. This task remains docs-only. No real `git tag`, `git push`, branch protection change, or GitHub settings mutation happens in this PR.
2. The tracked baseline candidate for the first GitHub baseline is `origin/main` at `2ab7be8f9a28eeb81d613d8da57544a758e72108`.
3. The recommended first baseline tag name is `v0.1.0-baseline`.
4. `rotation window closed / no rotation required` is recorded as the current decision for this baseline gate because the owner already confirmed all 12 credential classes are `invalid/non-live` on 2026-04-08.
5. GitHub required checks stay limited to CI-safe baseline validation only. Shared-lab, deploy, remote smoke, live validation, and third-party dependency checks are explicitly excluded.

## Branch protection recommendations

Recommended branch protection target: `main`

- Protect `main`.
- Require pull requests before merge.
- Require 1 approving review.
- Require status checks to pass before merging.
- Recommended required status checks:
  - `L0 Pre-check`
  - `L1 MVP Checks`
  - `L2 Security Regression`
  - `Secret Scan`
  - `Static Demo Build`
- Recommend `require branches to be up to date before merging`.
- Recommend `include administrators`.
- Do not allow force pushes.
- Do not allow branch deletion.

### Required checks mapping

| GitHub required check name | Workflow file | Job id |
| --- | --- | --- |
| `L0 Pre-check` | `.github/workflows/mvp-checks.yml` | `l0-pre-check` |
| `L1 MVP Checks` | `.github/workflows/mvp-checks.yml` | `l1-mvp-checks` |
| `L2 Security Regression` | `.github/workflows/mvp-checks.yml` | `l2-security-regression` |
| `Secret Scan` | `.github/workflows/mvp-checks.yml` | `secret-scan` |
| `Static Demo Build` | `.github/workflows/mvp-checks.yml` | `static-demo-build` |

## Required checks boundary

Only these CI-safe checks belong in the GitHub required-checks baseline:

- `L0 Pre-check`
- `L1 MVP Checks`
- `L2 Security Regression`
- `Secret Scan`
- `Static Demo Build`

The following are explicitly outside GitHub required checks for this baseline:

- `192.168.3.100` shared-environment validation
- deploy or cutover jobs
- `deploy/live-validate`
- `smoke-remote`
- `deploy-sync`
- Telegram-integrated checks
- VictoriaMetrics or VictoriaLogs runtime checks
- playground or other third-party environment checks
- `.github/workflows/ci-layered.yml`, which remains `workflow_dispatch` only

## First baseline operation checklist

This checklist records the intended first-baseline procedure only. This PR does not execute any step below.

1. Confirm owner sign-off that tracked baseline candidate `2ab7be8f9a28eeb81d613d8da57544a758e72108` is the intended GitHub baseline commit.
2. Re-run the CI-safe baseline commands in a clean tree:
   - `make pre-check`
   - `make check-mvp`
   - `make security-regression`
   - `make secret-scan`
   - `make static-demo-build`
3. Configure GitHub branch protection for `main` using the recommendations in this spec.
4. Create planned tag `v0.1.0-baseline` on commit `2ab7be8f9a28eeb81d613d8da57544a758e72108`.
5. Push `main` and the baseline tag after owner approval.
6. Verify that GitHub required checks map exactly to the five job names above.
7. Record the final sign-off and any owner-approved deviations from this spec.

If `v0.1.0-baseline` conflicts with an existing repository tag or owner naming convention at execution time, owner may choose an alternate tag name such as `baseline-2026-04-23`; that rename is an owner decision, not an engineering blocker for this spec.

## Non-goals

- Do not create or push a real Git tag in this task.
- Do not push commits to GitHub in this task.
- Do not modify branch protection settings in this task.
- Do not add shared-lab or external runtime validation into GitHub required checks.
- Do not add new deploy scripts, runtime automation, or 192.168.3.100 validation logic in this task.

## Acceptance criteria

- This spec is committed with the same PR as the runbook and workstream updates.
- The spec records an exact tracked baseline SHA, not a generic placeholder.
- The documented required checks include only the five CI-safe GitHub checks from `mvp-checks.yml`.
- The documented baseline gate does not treat `192.168.3.100` or any remote deploy/live validation as a GitHub required check.
- The runbook no longer leaves the three push blockers in an ambiguous state.

## Risks and owner decisions

### Risks

- Local `main` and `origin/main` differ today, so owner sign-off is still required before any irreversible baseline tag or push action.
- GitHub required-check names are case-sensitive in settings; configuration must use the exact names listed in this spec.
- If the repository already contains a conflicting baseline tag naming convention, execution must choose an alternate owner-approved tag name before tagging.

### Owner decisions

- Whether to enforce `include administrators` on `main`.
- Whether to require branches to be up to date before merge.
- Whether `v0.1.0-baseline` stays the final first-baseline tag name.
- Final owner sign-off that tracked baseline candidate `2ab7be8f9a28eeb81d613d8da57544a758e72108` is the intended first GitHub baseline commit.
