# Shared Lab Verification on 192.168.3.100

> Scope: runtime identity verification and evidence capture for the canonical shared lab on `192.168.3.100`.
> Canonical instance: `/data/tars-setup-lab`

## Unified Spec

This runbook is the shared-lab verification spec and template companion for evidence records such as [EVI-11](./records/EVI-11-PR6-2026-04-22.md) and [EVI-12](../../specs/evi-12-runtime-vs-pr-head-commit.md).

The goal is to make PR review and QA handoff answer two questions with one repeatable command:

1. Is port `8081` currently serving the canonical `/data/tars-setup-lab` instance?
2. Does the verification evidence attached to the PR match the runtime that was actually checked on `192.168.3.100`?

The verification flow adds four required artifacts:

1. `scripts/check-shared-lab.sh` for runtime identity and endpoint checks.
2. This runbook for timing, PASS/FAIL interpretation, and failure handling.
3. `docs/operations/templates/verification-evidence.md` as the PR evidence template.
4. `docs/operations/records/` as the best-effort home for filled records committed into the branch when requested.

## When To Run It

Run the script on `192.168.3.100` after the branch has been deployed to the shared lab and before any of the following:

1. Posting a PR comment that says the branch is ready for review.
2. Asking PM or TEST to start acceptance.
3. Capturing the 1440px and 390px browser screenshots that will be cited as evidence.

Do not treat local checks or GitHub CI as a substitute for this step. This script is the runtime identity gate for the shared lab.

## Command

On the shared host itself:

```sh
cd /path/to/TARS
export TARS_SHARED_LAB_SESSION_URL="http://192.168.3.100:8081/sessions/<session-id>"
export TARS_SHARED_LAB_EXPECTED_GIT_HEAD="$(git rev-parse HEAD)"
bash scripts/check-shared-lab.sh
```

From a local checkout, if you need to execute the current branch script on the shared host without copying files into the canonical tree:

```sh
export TARS_SHARED_LAB_SESSION_URL="http://192.168.3.100:8081/sessions/<session-id>"
export TARS_SHARED_LAB_EXPECTED_GIT_HEAD="$(git rev-parse HEAD)"
ssh root@192.168.3.100 \
  "TARS_SHARED_LAB_SESSION_URL=${TARS_SHARED_LAB_SESSION_URL} TARS_SHARED_LAB_EXPECTED_GIT_HEAD=${TARS_SHARED_LAB_EXPECTED_GIT_HEAD} bash -s" \
  < scripts/check-shared-lab.sh
```

Optional overrides:

```sh
export TARS_SHARED_LAB_BASE_URL="http://127.0.0.1:8081"
export TARS_SHARED_LAB_CANONICAL_BASE_DIR="/data/tars-setup-lab"
export TARS_OPS_API_TOKEN="<temporary override only when the env file is wrong>"
```

## PASS Criteria

The script only returns `overall: PASS` when all of the following are true:

1. Port `8081` has a LISTEN pid.
2. The pid resolves to a binary under `/data/tars-setup-lab`.
3. The process workdir is `/data/tars-setup-lab` or a child path.
4. The process `TARS_*` path variables point to `/data/tars-setup-lab`, not `/root/tars-dev` or another tree.
5. `shared-test.env` resolves under `/data/tars-setup-lab`.
6. `POST /api/v1/auth/login` succeeds with the shared `local_token`.
7. `GET /api/v1/setup/status` succeeds.
8. The specified session URL is reachable.

## FAIL Criteria

Treat any `overall: FAIL` result as a blocker for PR acceptance on the shared lab.

Common blockers:

1. `workdir/config points outside canonical shared lab root`: the live process is probably running from `/root/tars-dev` or another stale tree.
2. `shared env file outside canonical shared lab root`: the process may have been started with the wrong env file.
3. `local_token login failed`: the break-glass auth path is not healthy, or the env file token is stale.
4. `setup/status failed`: the authenticated runtime checks path is not healthy.
5. `missing session URL input`: no session-specific evidence can be tied to the branch yet.

## What To Do On Failure

1. Stop the acceptance flow. Do not mark the branch ready.
2. Paste the script output into the PR or issue comment verbatim.
3. State whether the blocker is runtime identity, auth, setup status, or session reachability.
4. If the process is on `/root/tars-dev` or another non-canonical tree, restart or redeploy the canonical `/data/tars-setup-lab` instance before taking screenshots.
5. Re-run the script and replace the failed evidence with the passing output before asking for review again.

## Evidence Capture Flow

After the script passes:

1. Open the shared lab in a browser.
2. Capture one desktop screenshot at `1440px` width.
3. Capture one mobile screenshot at `390px` width.
4. Fill `docs/operations/templates/verification-evidence.md`.
5. If PM asks for committed evidence, save the filled record under `docs/operations/records/EVI-11-PR{number}-{date}.md` and include it in the PR.

### Evidence Commits And PR Head Movement

Evidence commits can move the PR head after runtime validation. That is expected and does not automatically invalidate the shared-lab result.

Use these definitions consistently in the template and PR comments:

1. `Runtime validated commit SHA`: the commit that was actually deployed and verified on `192.168.3.100`.
2. `Evidence commit SHA` or `docs-only evidence commit SHA`: the follow-up commit that only adds records, screenshots, or other evidence files.
3. `Final PR head commit SHA`: the latest PR head before final acceptance.

If the evidence commit only changes documentation or evidence files and does not change runtime code, deployment configuration, or scripts, do not redeploy `192.168.3.100` and do not repeat runtime validation just because the PR head moved.

Examples of likely docs-only evidence files:

1. `docs/**/*.md`
2. `docs/**/*.png`
3. `docs/**/*.jpg`

Even when no redeploy is required, the `Final PR head commit SHA` must still have green GitHub CI/checks before anyone describes the PR as finally complete.

Treat the file examples above as guidance, not as the only rule. The real decision point is whether the follow-up commit changed runtime code, deployment configuration, or scripts.

If `Runtime validated commit SHA` and `Final PR head commit SHA` differ, explain the reason in `Notes / Blockers` or an equivalent field, for example `evidence-only commit` or `docs-only screenshots and records`.

Do not create an infinite evidence loop. Evidence-only commits do not trigger another shared-lab redeploy, which means they also do not require another round of runtime screenshots solely because the PR head advanced.

This EVI-12 update is a docs-only/template-only task. It does not require fresh `192.168.3.100` runtime validation by itself.

## Output Format

The script intentionally prints comment-friendly lines:

```text
hostname: mff
timestamp_utc: 2026-04-22T10:00:00Z
check.listener_8081: PASS pid=1234 port=8081
check.workdir_path: PASS /data/tars-setup-lab
check.session_url: PASS status=200 url=http://192.168.3.100:8081/sessions/...
overall: PASS
```

Paste this block directly into a PR comment or issue reply so reviewers can compare the runtime identity output with the screenshot evidence, the runtime validated commit, and the final PR head commit.
