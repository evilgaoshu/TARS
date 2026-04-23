# GitHub Baseline Sign-off Summary

Date: 2026-04-23

## Summary

- Scope: document the GitHub baseline publish gate for EVI-14 without executing a real tag, push, cutover, or GitHub settings mutation.
- Tracked baseline candidate: `origin/main` at `2ab7be8f9a28eeb81d613d8da57544a758e72108`.
- Planned first baseline tag: `v0.1.0-baseline`.
- Rotation decision: closed as `no rotation required` for this engineering gate because the owner confirmed all 12 credential classes were `invalid/non-live` on 2026-04-08.

## Required checks mapping

| GitHub required check | Workflow file | Job id |
| --- | --- | --- |
| `L0 Pre-check` | `.github/workflows/mvp-checks.yml` | `l0-pre-check` |
| `L1 MVP Checks` | `.github/workflows/mvp-checks.yml` | `l1-mvp-checks` |
| `L2 Security Regression` | `.github/workflows/mvp-checks.yml` | `l2-security-regression` |
| `Secret Scan` | `.github/workflows/mvp-checks.yml` | `secret-scan` |
| `Static Demo Build` | `.github/workflows/mvp-checks.yml` | `static-demo-build` |

## Recommended branch protection

- Protect `main`.
- Require pull requests before merge.
- Require 1 approving review.
- Require status checks to pass.
- Recommend `include administrators`.
- Recommend `require branches to be up to date before merging`.
- Do not allow force pushes.
- Do not allow branch deletion.

## Explicitly out of scope for GitHub required checks

- `192.168.3.100` shared-lab validation
- deploy, cutover, or live validation
- `smoke-remote`
- `deploy-sync`
- Telegram-dependent checks
- VictoriaMetrics or VictoriaLogs runtime checks
- playground or other third-party environment dependencies
- `.github/workflows/ci-layered.yml`, which remains `workflow_dispatch` only

## Not executed in this pass

- No real `git tag` creation.
- No `git push` to GitHub.
- No branch protection mutation.
- No GitHub settings changes.
- No `192.168.3.100` shared-environment verification, because this task is docs-only.
