# EVI-18 Runtime Canonical Verification

Date: 2026-04-26

## Scope

Verify and harden the shared-lab runtime identity for `192.168.3.100`.

## Expected Canonical Path

- Canonical runtime root: `/data/tars-setup-lab`
- Shared config path: `/data/tars-setup-lab/team-shared`
- Shared env path: `/data/tars-setup-lab/team-shared/shared-test.env`
- Managed service: `tars-shared-lab.service` (systemd)

## Runtime Identity

- PID: `845342`
- Binary: `/data/tars-setup-lab/bin/tars-linux-amd64-dev`
- CWD / workdir: `/data/tars-setup-lab/team-shared`
- `TARS_DIR`: `/data/tars-setup-lab`
- `shared-test.env`: `/data/tars-setup-lab/team-shared/shared-test.env`
- Runtime git head: `5de6078656a93521c8b133c2a2aa02ec045de12c`
- PR/head commit at runtime validation: `5de6078656a93521c8b133c2a2aa02ec045de12c`
- systemd unit evidence: `tars-shared-lab.service` with canonical `WorkingDirectory`, `ExecStart`, and `EnvironmentFile`

## Commands And Results

### Initial Drift Capture

```sh
TARS_SHARED_LAB_EXPECTED_GIT_HEAD="$(git rev-parse HEAD)" \
ssh root@192.168.3.100 \
  "TARS_SHARED_LAB_EXPECTED_GIT_HEAD=${TARS_SHARED_LAB_EXPECTED_GIT_HEAD} bash -s" \
  < scripts/check-shared-lab.sh
```

Result before deployment: `overall: FAIL blockers=7 warnings=0`.

Blockers:

- missing `runtime_git_head`
- binary path `/root/tars-dev/bin/tars-linux-amd64-dev`
- workdir `/root/tars-dev/team-shared`
- process `TARS_*` paths under `/root/tars-dev`
- missing `TARS_DIR` in canonical `shared-test.env`
- missing `tars-shared-lab.service`
- missing session URL input

### Deploy And Runtime Gate

```sh
TARS_REMOTE_HOST=192.168.3.100 TARS_REMOTE_USER=root bash scripts/deploy_team_shared.sh
```

The deploy built and synced commit `5de6078656a93521c8b133c2a2aa02ec045de12c`, restarted via `tars-shared-lab.service`, and passed remote smoke/performance. The final AI smoke portion of `live-validate` failed separately; see "Remaining Blocker".

```sh
TARS_SHARED_LAB_EXPECTED_GIT_HEAD="$(git rev-parse HEAD)" \
TARS_SHARED_LAB_SESSION_URL="http://192.168.3.100:8081/runtime-checks" \
ssh root@192.168.3.100 \
  "TARS_SHARED_LAB_EXPECTED_GIT_HEAD=${TARS_SHARED_LAB_EXPECTED_GIT_HEAD} TARS_SHARED_LAB_SESSION_URL=${TARS_SHARED_LAB_SESSION_URL} bash -s" \
  < scripts/check-shared-lab.sh
```

Runtime identity result:

```text
hostname: mff
timestamp_utc: 2026-04-26T02:12:33Z
base_url: http://127.0.0.1:8081
canonical_base_dir: /data/tars-setup-lab
canonical_override_file: /data/tars-setup-lab/.canonical-override
managed_service_name: tars-shared-lab.service
expected_git_head: 5de6078656a93521c8b133c2a2aa02ec045de12c
repo_git_head: n/a
runtime_git_head: 5de6078656a93521c8b133c2a2aa02ec045de12c
runtime_git_head_source: /data/tars-setup-lab/team-shared/runtime_git_head
check.runtime_git_head: PASS expected=5de6078656a93521c8b133c2a2aa02ec045de12c actual=5de6078656a93521c8b133c2a2aa02ec045de12c
check.listener_8081: PASS pid=845342 port=8081
check.binary_path: PASS /data/tars-setup-lab/bin/tars-linux-amd64-dev
check.workdir_path: PASS /data/tars-setup-lab/team-shared
check.config_paths: PASS TARS_DIR=/data/tars-setup-lab; TARS_SERVER_LISTEN=0.0.0.0:8081; TARS_WEB_DIST_DIR=/data/tars-setup-lab/web-dist; TARS_VECTOR_SQLITE_PATH=/data/tars-setup-lab/data/tars-vec.db; TARS_OBSERVABILITY_DATA_DIR=/data/tars-setup-lab/data/observability; TARS_PROVIDERS_CONFIG_PATH=/data/tars-setup-lab/team-shared/providers.shared.yaml; TARS_CONNECTORS_CONFIG_PATH=/data/tars-setup-lab/team-shared/connectors.shared.yaml; TARS_SKILLS_CONFIG_PATH=/data/tars-setup-lab/team-shared/skills.shared.yaml; TARS_AUTOMATIONS_CONFIG_PATH=/data/tars-setup-lab/team-shared/automations.shared.yaml; TARS_EXTENSIONS_STATE_PATH=/data/tars-setup-lab/team-shared/extensions.state.yaml; TARS_REASONING_PROMPTS_CONFIG_PATH=/data/tars-setup-lab/team-shared/reasoning-prompts.shared.yaml; TARS_DESENSITIZATION_CONFIG_PATH=/data/tars-setup-lab/team-shared/desensitization.shared.yaml; TARS_SECRETS_CONFIG_PATH=/data/tars-setup-lab/team-shared/secrets.shared.yaml; TARS_EXECUTION_OUTPUT_SPOOL_DIR=/data/tars-setup-lab/execution-output; TARS_APPROVALS_CONFIG_PATH=/data/tars-setup-lab/team-shared/approvals.shared.yaml; TARS_AUTHORIZATION_CONFIG_PATH=/data/tars-setup-lab/team-shared/authorization.shared.yaml; TARS_ACCESS_CONFIG_PATH=/data/tars-setup-lab/team-shared/access.shared.yaml
check.shared_env_file: PASS /data/tars-setup-lab/team-shared/shared-test.env
check.shared_env_tars_dir: PASS /data/tars-setup-lab
check.managed_service_unit: PASS tars-shared-lab.service
check.managed_service_workdir: PASS /data/tars-setup-lab/team-shared
check.managed_service_execstart: PASS /data/tars-setup-lab/bin/tars-linux-amd64-dev
check.managed_service_environment_file: PASS /data/tars-setup-lab/team-shared/shared-test.env
check.auth_token: PASS token_resolved=yes source=/data/tars-setup-lab/team-shared/shared-test.env
check.auth_login_local_token: PASS status=200 session_token_present=yes
check.setup_status_endpoint: PASS status=200 initialization=True rollout_mode=knowledge_on
check.session_url: PASS status=200 url=http://192.168.3.100:8081/runtime-checks
overall: PASS
```

### Additional Shared Validation

```sh
TARS_REMOTE_HOST=192.168.3.100 \
TARS_REMOTE_USER=root \
TARS_OPS_BASE_URL=http://192.168.3.100:8081 \
TARS_VALIDATE_RUN_SMOKE=0 \
bash scripts/ci/live-validate.sh
```

Result: `live-validate=passed total=5s`.

Included evidence:

- `metrics.query_range`: 2 series, 13 points
- `logs.query`: completed, 5 entries
- `observability.query`: completed, 2 results
- `delivery.query`: completed, 5 results
- observability connector live validation: passed

## Remaining Blocker

The default deploy command runs `scripts/ci/live-validate.sh` with AI smoke scenarios enabled. That part failed after the runtime drift was fixed:

1. First deploy run: `scenario=logs expected tools ['metrics.query_range', 'logs.query', 'observability.query'], got ['logs.query']`.
2. Rerun: logs smoke session `e3625169-0856-4d7e-8351-f3cc64636082` generated invalid VictoriaLogs syntax, then opened execution `15408ef4-d5ae-452a-a895-85e165f1e1c2` for approval.
3. Cleanup: execution `15408ef4-d5ae-452a-a895-85e165f1e1c2` was rejected with reason `cleanup failed EVI-18 live-validation smoke pending approval`.
4. Hygiene after cleanup: `pending_approvals=0`, `failed_outbox=0`, `blocked_outbox=0`.

This blocker is outside the canonical runtime identity drift fix. The runtime gate for EVI-18 passed, but default shared live validation with AI smoke scenarios is not green.

## Current Status

Canonical runtime drift is fixed on `192.168.3.100` for commit `5de6078656a93521c8b133c2a2aa02ec045de12c`. The remaining blocker is the default AI smoke scenario inside live validation, documented above.
