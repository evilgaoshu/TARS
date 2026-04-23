# EVI-12 Runtime Validated Commit vs Final PR Head Commit

## Scope

EVI-12 is a docs-only/template-only update for shared-lab verification records.

Files in scope:

1. `docs/operations/templates/verification-evidence.md`
2. `docs/operations/shared-lab-verification.md`
3. This versioned spec document

Out of scope:

1. Product functionality
2. Runtime behavior on `192.168.3.100`
3. Deployment scripts
4. GitHub CI logic

This task does not require fresh `192.168.3.100` validation because it only clarifies documentation and evidence templates.

## Problem Statement

Shared-lab verification can happen on one commit, while the PR head later moves forward because the branch adds screenshots, records, or other evidence files.

Without explicit fields, a record can look inconsistent even when the runtime evidence is correct. That creates review churn and can accidentally start an unnecessary redeploy loop.

## Required Record Fields

Every shared-lab verification record must distinguish these commit references:

1. `Runtime validated commit SHA`
2. `Evidence commit SHA` or `docs-only evidence commit SHA`
3. `Final PR head commit SHA`

Definitions:

1. `Runtime validated commit SHA` is the commit that was actually deployed and verified in the shared lab runtime.
2. `Evidence commit SHA` is the follow-up commit that adds screenshots, records, or other evidence after runtime validation.
3. `Final PR head commit SHA` is the latest PR head before final acceptance and must be the commit whose GitHub checks are green.

## Runbook Rules

The shared-lab runbook must state all of the following:

1. Evidence commits moving the PR head is expected behavior.
2. If an evidence commit only changes docs or evidence files and does not change runtime code, deployment configuration, or scripts, `192.168.3.100` does not need to be redeployed.
3. If no redeploy is required, runtime validation does not need to be repeated.
4. Even without redeploying, the final PR head still needs green GitHub CI/checks before the PR can be described as finally complete.
5. When `Runtime validated commit SHA` and `Final PR head commit SHA` differ, the record must explain the reason in `Notes` or an equivalent field.
6. The process must avoid infinite evidence loops caused by evidence-only commits triggering more evidence-only commits.

## Suggested Docs-Only Evidence Scope

Suggested low-risk examples for docs-only evidence commits:

1. `docs/**/*.md`
2. `docs/**/*.png`
3. `docs/**/*.jpg`

These examples are guidance, not a mechanical allowlist. The real rule is that a docs-only evidence commit must not change runtime code, deployment configuration, or scripts.

If the repository later defines a stricter allowlist, that repository rule should take precedence.

## Example Scenario

The intended example is the pattern seen in [EVI-11](../records/EVI-11-PR6-2026-04-22.md):

1. Commit A is deployed and passes shared-lab runtime validation.
2. Commit B only adds screenshots or records.
3. The PR head moves from commit A to commit B.
4. The record keeps commit A as `Runtime validated commit SHA`.
5. The record stores commit B as `Evidence commit SHA` and `Final PR head commit SHA` if B is still the latest head.
6. The notes explain that commit B is evidence-only, so no redeploy was required.

## Acceptance Intent

This spec is satisfied when the template and runbook make the above distinctions explicit and reviewers can tell, from the record alone, whether:

1. the runtime was validated on the correct commit,
2. a later docs-only evidence commit moved the PR head, and
3. the final PR head still needs fresh green GitHub checks before final acceptance.
