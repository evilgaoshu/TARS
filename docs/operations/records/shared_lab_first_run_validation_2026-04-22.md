# Shared Lab First-Run Validation 2026-04-22

Date: 2026-04-22
Target: `root@192.168.3.100`
Branch: `agent/dev-opencode-gpt5-4/5e4a52d6`

## Scope

- Verify the documented shared-lab deploy path on `192.168.3.100` using `shared-test.env` and `team-shared` config paths.
- Capture real smoke/live/profile evidence for `smoke-remote`, `live-validate`, and the TARS connector main path.
- Preserve a concise, replayable record for later QA/PM verification.

## Local Preconditions

- Repository root: `TARS`
- Required local prep discovered during execution:
  - `web/node_modules` was absent, so `cd web && npm ci` was required before `npm run build` could succeed.
- Shared-lab SSH access was available:

```sh
ssh -o BatchMode=yes -o ConnectTimeout=5 root@192.168.3.100 \
  "uname -m && test -f /data/tars-setup-lab/team-shared/shared-test.env && printf 'shared-env-present\n' && test -x /data/tars-setup-lab/bin/tars-linux-amd64-dev && printf 'binary-present\n'"
```

Observed:

- `x86_64`
- `shared-env-present`
- `binary-present`

## Minimal Fixes Made Before Validation

### 1. Deploy script template source mismatch

Observed failure:

```text
cp: .../deploy/team-shared/shared-test.env: No such file or directory
```

Root cause:

- The repo commits `deploy/team-shared/shared-test.env.example`.
- `scripts/deploy_team_shared.sh` expected `deploy/team-shared/shared-test.env` in the local tree.
- `.gitignore` excludes `deploy/team-shared/shared-test.env`, so the documented path was not reproducible from a clean checkout.

Fix:

- `scripts/deploy_team_shared.sh` now uses `shared-test.env` when present, otherwise falls back to `shared-test.env.example`, copying it into the sync temp directory as `shared-test.env` before remote merge/sync.
- Docs were updated to state that the committed template source is `deploy/team-shared/shared-test.env.example`.

### 2. Local web build dependency precondition

Observed failure on the first deploy attempt:

```text
sh: tsc: command not found
```

Root cause:

- `web/package.json` expects local TypeScript from installed dependencies.
- This checkout did not yet have `web/node_modules`.

Resolution:

```sh
cd web && npm ci
```

No repo code change was needed for this step.

## Standard Deploy Command Run

Executed from repo root:

```sh
TARS_REMOTE_HOST=192.168.3.100 \
TARS_REMOTE_USER=root \
TARS_REMOTE_BASE_DIR=/data/tars-setup-lab \
bash scripts/deploy_team_shared.sh
```

## Deploy Outcome

The standard path completed successfully after the script/docs fix.

Observed high-level phases:

- local `linux/amd64` build: passed
- local web build: passed
- remote `team-shared` sync: passed
- remote binary/web sync: passed
- remote restart via `shared_remote_service_restart`: passed
- remote readiness / smoke: passed
- `live-validate` profile `core`: passed
- golden scenario 2 (`victoriametrics-main/query.instant` automation -> inbox): passed
- golden scenario 1 (web chat -> session resolved -> inbox): passed

## Shared-Lab Runtime Snapshot After Deploy

Executed:

```sh
ssh -o BatchMode=yes root@192.168.3.100 '
  pid=$(pgrep -f -x /data/tars-setup-lab/bin/tars-linux-amd64-dev | head -n 1)
  printf "pid=%s\n" "$pid"
  tr "\0" "\n" < /proc/$pid/environ | egrep "^(TARS_WEB_DIST_DIR|TARS_POSTGRES_DSN|TARS_CONNECTORS_CONFIG_PATH|TARS_ACCESS_CONFIG_PATH)="
  printf "bootstrap="; curl -fsS http://127.0.0.1:8081/api/v1/bootstrap/status; printf "\n"
  printf "health="; curl -fsS http://127.0.0.1:8081/healthz; printf "\n"
  printf "ready="; curl -fsS http://127.0.0.1:8081/readyz; printf "\n"
'
```

Observed:

```text
pid=3618077
TARS_ACCESS_CONFIG_PATH=/data/tars-setup-lab/team-shared/access.shared.yaml
TARS_POSTGRES_DSN=postgres://tars:tars@127.0.0.1:5432/tars?sslmode=disable
TARS_WEB_DIST_DIR=/data/tars-setup-lab/web-dist
TARS_CONNECTORS_CONFIG_PATH=/data/tars-setup-lab/team-shared/connectors.shared.yaml
bootstrap={"initialized":true,"mode":"runtime"}
health={"status":"ok"}
ready={"status":"ready","degraded":false}
```

This confirms the process is running with the shared config paths instead of a bare local-default startup.

## Smoke / Live Validation Evidence

### Remote smoke

Observed from `scripts/ci/smoke-remote.sh`:

- `/healthz`: passed
- `/readyz`: passed
- `/api/v1/platform/discovery`: passed
- `registered_connector_ids` included:
  - `victoriametrics-main`
  - `victorialogs-main`
  - `observability-main`
  - `delivery-main`
  - `jumpserver-main`
  - `ssh-main`
- pilot hygiene: `result=clean`
- performance spot check: `performance-spot-check=passed`

### Tool-plan live validation

Observed from `scripts/ci/live-validate.sh` -> `scripts/validate_tool_plan_live.sh`:

- `metrics.query_range`: passed
  - `runtime=victoriametrics-main`
  - `series_count=2`
  - `points=13`
- capability invoke approval path: passed
  - HTTP `202`
  - `capability_status=pending_approval`
- capability deny path: passed
  - HTTP `403`
  - `deny_status=denied`
- `logs.query`: passed
  - `logs_status=completed`
  - `logs_count=5`
- `observability.query`: passed
  - `observability_status=completed`
  - `observability_count=2`
- `delivery.query`: passed
  - `delivery_status=completed`
  - `delivery_count=5`

Smoke scenarios created resolved sessions with attachment evidence and no execution escalation:

- `logs` scenario: `executions=0`
- `observability` scenario: `executions=0`
- `delivery` scenario: `executions=0`

### Observability connector main-path validation

Observed from `scripts/validate_observability_connectors_live.sh`:

Baseline connector health/query:

- `victoriametrics-main`
  - health: `status=healthy`
  - metrics query:
    - `series_count=1`
    - `points=13`
    - `first_job=node_3_100`
    - `first_host=192.168.3.100`
- `victorialogs-main`
  - health: `status=healthy`
  - logs query:
    - `status=completed`
    - `result_count=2`
    - `request_url=http://127.0.0.1:9428/select/logsql/query?...`
    - `first_msg=tars-observability-host-file-test host=192.168.3.9 ts=2026-04-22T03:33:50Z`

Temporary connector CRUD/probe path:

- `victoriametrics-live-1776828859`
  - draft probe: healthy + compatible
  - create: passed
  - update: passed
  - health: passed
  - metrics query: passed
- `victorialogs-live-1776828859`
  - draft probe: healthy + compatible
  - create: passed
  - update: passed
  - health: passed
  - logs query: passed
- cleanup: `cleanup=ok`

This provides real `192.168.3.100` evidence for both the baseline connectors and the create/update/probe/health/query main path.

## Golden Scenario Evidence

### Golden scenario 2

- automation job: `golden-inspection-victoriametrics`
- manual run id: `b4e70ac7-565e-41b2-a4ab-d784f02db164`
- run status: `completed`
- inbox evidence: matched subject `执行结果：完成 - victoriametrics-main/query.instant`

### Golden scenario 1

- session id: `da0f95ce-0638-4f22-aa88-e59841d81f9a`
- session status: `resolved`
- inbox evidence: matched subject `会话已关闭`

## Non-Blocking Runtime Signal

The deploy/validation path passed, but the shared-lab runtime log still shows reasoning model fallback warnings:

```text
reasoning primary model failed, trying assist model
... dial tcp 192.168.1.132:1234: connect: no route to host
diagnosis finalizer failed, falling back
... assist model failed: status=400 message=API key not valid
```

Impact seen during this run:

- golden scenarios still completed successfully
- connector validation and inbox evidence still passed
- this is therefore recorded as a shared-lab quality risk, not a blocker for the deploy/live-validation loop completed here

Recommended next step:

- separately verify the shared provider baseline for `lmstudio-local` reachability and `gemini-backup` credential validity on `192.168.3.100`

## Replay Commands

### Standard deploy + validation

```sh
cd /path/to/TARS
cd web && npm ci
cd ..
TARS_REMOTE_HOST=192.168.3.100 \
TARS_REMOTE_USER=root \
TARS_REMOTE_BASE_DIR=/data/tars-setup-lab \
bash scripts/deploy_team_shared.sh
```

### Standalone live validation

```sh
TARS_REMOTE_HOST=192.168.3.100 \
TARS_REMOTE_USER=root \
TARS_OPS_BASE_URL=http://192.168.3.100:8081 \
bash scripts/ci/live-validate.sh
```

### Shared-lab reset

```sh
TARS_REMOTE_HOST=192.168.3.100 \
TARS_REMOTE_USER=root \
TARS_REMOTE_BASE_DIR=/data/tars-setup-lab \
bash scripts/reset_lab_192.168.3.100.sh
```

## Verdict

`192.168.3.100` now has a replayable, documented shared-lab deploy + live-validation loop using the required `shared-test.env` / `team-shared` path, with fresh VM/VL connector evidence captured on the target machine.
