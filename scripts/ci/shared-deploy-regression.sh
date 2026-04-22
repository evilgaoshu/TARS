#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() {
  printf 'shared-deploy-regression: %s\n' "$*" >&2
  exit 1
}

test_local_placeholder_token_is_not_accepted_as_override() {
  (
    source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"
    export TARS_OPS_API_TOKEN="REPLACE_WITH_OPS_API_TOKEN"

    if resolved="$(shared_ops_token_local_override 2>/dev/null)"; then
      fail "expected local placeholder token to be rejected as local override, got ${resolved}"
    fi
  )
}

test_local_placeholder_token_falls_back_to_remote() {
  (
    source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"
    export TARS_OPS_API_TOKEN="REPLACE_WITH_OPS_API_TOKEN"

    shared_ops_token_fetch_remote() {
      printf 'remote-secret\n'
    }

    resolved="$(shared_ops_token_resolve)" || fail "expected placeholder local token to fall back to remote token"
    [[ "${resolved}" == "remote-secret" ]] || fail "expected remote token fallback when local token is placeholder"
  )
}

test_remote_placeholder_token_is_rejected() {
  (
    source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"
    unset TARS_OPS_API_TOKEN

    shared_ops_token_fetch_remote() {
      printf 'REPLACE_WITH_OPS_API_TOKEN\n'
    }

    if resolved="$(shared_ops_token_resolve 2>/dev/null)"; then
      fail "expected remote placeholder token to be rejected, got ${resolved}"
    fi
  )
}

test_local_real_token_is_accepted() {
  (
    source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"
    export TARS_OPS_API_TOKEN="local-secret"

    resolved="$(shared_ops_token_resolve)" || fail "expected local token to resolve"
    [[ "${resolved}" == "local-secret" ]] || fail "expected local token to round-trip"
  )
}

test_shared_host_token_fallback_defaults_remote_user() {
  (
    source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"
    unset TARS_OPS_API_TOKEN
    unset TARS_REMOTE_USER
    export TARS_REMOTE_HOST="192.168.3.100"

    resolved_user="$(shared_ops_token_remote_user)" || fail "expected shared host remote user helper to resolve"
    [[ "${resolved_user}" == "root" ]] || fail "expected shared host token fallback to default remote user to root"

    ssh() {
      printf 'remote-secret\n'
    }
    resolved="$(shared_ops_token_resolve)" || fail "expected shared host token fallback to resolve without explicit TARS_REMOTE_USER"
    [[ "${resolved}" == "remote-secret" ]] || fail "expected shared host token fallback to return remote token"
  )
}

test_sync_only_uses_local_token_override() {
  (
    source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"

    unset TARS_OPS_API_TOKEN
    shared_ops_token_fetch_remote() {
      printf 'remote-secret\n'
    }

    if resolved="$(shared_ops_token_local_override 2>/dev/null)"; then
      fail "expected missing local override to stay empty, got ${resolved}"
    fi

    export TARS_OPS_API_TOKEN="local-secret"
    resolved="$(shared_ops_token_local_override)" || fail "expected explicit local override to resolve"
    [[ "${resolved}" == "local-secret" ]] || fail "expected explicit local override to round-trip"
  )
}

test_check_shared_lab_script_exists_and_is_executable() {
  local script_path="${ROOT_DIR}/scripts/check-shared-lab.sh"

  [[ -f "${script_path}" ]] || fail "expected shared lab verification script at scripts/check-shared-lab.sh"
  [[ -x "${script_path}" ]] || fail "expected scripts/check-shared-lab.sh to be executable"
}

test_check_shared_lab_script_enforces_canonical_runtime_and_endpoints() {
  local script_path="${ROOT_DIR}/scripts/check-shared-lab.sh"

  grep -q 'CANONICAL_BASE_DIR="${TARS_SHARED_LAB_CANONICAL_BASE_DIR:-/data/tars-setup-lab}"' "${script_path}" || \
    fail "check-shared-lab should default the canonical shared lab root to /data/tars-setup-lab"
  grep -q '/api/v1/auth/login' "${script_path}" || \
    fail "check-shared-lab should verify the local_token login endpoint"
  grep -q '/api/v1/setup/status' "${script_path}" || \
    fail "check-shared-lab should verify the setup/status endpoint"
  grep -q 'session_url' "${script_path}" || \
    fail "check-shared-lab should support an explicit session URL input"
  grep -q 'workdir/config points outside canonical shared lab root' "${script_path}" || \
    fail "check-shared-lab should emit a blocker when workdir or config escapes the canonical root"
}

test_shared_lab_verification_docs_and_template_exist() {
  local doc_path="${ROOT_DIR}/docs/operations/shared-lab-verification.md"
  local template_path="${ROOT_DIR}/docs/operations/templates/verification-evidence.md"
  local records_dir="${ROOT_DIR}/docs/operations/records"

  [[ -f "${doc_path}" ]] || fail "expected shared lab verification runbook doc"
  [[ -f "${template_path}" ]] || fail "expected verification evidence template doc"
  [[ -d "${records_dir}" ]] || fail "expected docs/operations/records directory to exist"

  grep -q 'PR review' "${doc_path}" || \
    fail "shared lab verification doc should explain when to run checks before PR review"
  grep -q 'PASS' "${doc_path}" || \
    fail "shared lab verification doc should explain PASS/FAIL interpretation"
  grep -q 'PR URL' "${template_path}" || \
    fail "verification evidence template should capture PR URL"
  grep -q 'Head commit SHA' "${template_path}" || \
    fail "verification evidence template should capture head commit SHA"
  grep -q '1440px' "${template_path}" || \
    fail "verification evidence template should capture desktop screenshot evidence"
  grep -q '390px' "${template_path}" || \
    fail "verification evidence template should capture mobile screenshot evidence"
}

test_shared_remote_service_restart_sets_canonical_workdir() {
  local helper_path="${ROOT_DIR}/scripts/lib/shared_remote_service.sh"

  grep -q 'cd "${shared_dir}"' "${helper_path}" || \
    fail "shared_remote_service_restart should cd into the canonical shared dir before launching the binary"
}

test_deploy_normalizes_local_placeholder_before_remote_fallback() {
  local deploy_path="${ROOT_DIR}/scripts/deploy_team_shared.sh"

  grep -q 'shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}"' "${deploy_path}" || \
    fail "deploy script should normalize placeholder local token before deciding whether to fall back to remote canonical token"
}

test_smoke_scripts_normalize_placeholder_before_remote_fallback() {
  local smoke_path="${ROOT_DIR}/scripts/ci/smoke-remote.sh"
  local live_path="${ROOT_DIR}/scripts/ci/live-validate.sh"
  local web_path="${ROOT_DIR}/scripts/ci/web-smoke.sh"
  local observability_path="${ROOT_DIR}/scripts/validate_observability_connectors_live.sh"

  grep -q 'shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}"' "${smoke_path}" || \
    fail "smoke-remote should normalize placeholder TARS_OPS_API_TOKEN before remote fallback"
  grep -q 'shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}"' "${live_path}" || \
    fail "live-validate should normalize placeholder TARS_OPS_API_TOKEN before remote fallback"
  grep -q 'shared_ops_token_normalize "${TARS_PLAYWRIGHT_TOKEN:-}"' "${web_path}" || \
    fail "web-smoke should normalize placeholder TARS_PLAYWRIGHT_TOKEN before remote fallback"
  grep -q 'shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}"' "${web_path}" || \
    fail "web-smoke should fall back through normalized TARS_OPS_API_TOKEN before remote canonical token"
  grep -q 'shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}"' "${observability_path}" || \
    fail "validate_observability_connectors_live should normalize placeholder TARS_OPS_API_TOKEN before remote fallback"
  grep -q 'shared_ops_token_resolve' "${observability_path}" || \
    fail "validate_observability_connectors_live should resolve the shared ops token when local token is missing or placeholder"
}

test_tool_plan_live_validate_requires_monitoring_first_tools() {
  local tool_plan_path="${ROOT_DIR}/scripts/validate_tool_plan_live.sh"
  local live_path="${ROOT_DIR}/scripts/ci/live-validate.sh"
  local validator_path="${ROOT_DIR}/scripts/lib/validate_tool_plan_smoke.py"

  grep -Fq 'if "execution.run_command" in tools:' "${validator_path}" || \
    fail "tool-plan smoke validator should only accept monitoring-first evidence tools in the smoke assertion"
  ! grep -Fq 'connector.invoke_capability' "${tool_plan_path}" || \
    fail "validate_tool_plan_live should not treat generic connector.invoke_capability as a monitoring-first smoke pass condition"
  grep -q 'PROFILE="${TARS_VALIDATE_PROFILE:-all}"' "${tool_plan_path}" || \
    fail "validate_tool_plan_live should support TARS_VALIDATE_PROFILE for metrics/logs/observability/delivery targeted runs"
  grep -q 'source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"' "${tool_plan_path}" || \
    fail "validate_tool_plan_live should source the shared ops token helper instead of requiring a local token"
  grep -q 'shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}"' "${tool_plan_path}" || \
    fail "validate_tool_plan_live should normalize placeholder local tokens before remote fallback"
  grep -q 'shared_ops_token_resolve' "${tool_plan_path}" || \
    fail "validate_tool_plan_live should resolve the shared ops token when local token is missing or placeholder"
  grep -Eq 'case "\$PROFILE" in|metrics\)|logs\)|observability\)|delivery\)|all\)' "${tool_plan_path}" || \
    fail "validate_tool_plan_live should define targeted profiles for metrics/logs/observability/delivery"
  grep -q -- '-- logs.query --' "${tool_plan_path}" || \
    fail "validate_tool_plan_live should include a direct shared-host logs.query validation before smoke scenarios"
  grep -q 'validator.validate_scenario(name, detail)' "${tool_plan_path}" || \
    fail "validate_tool_plan_live should delegate smoke session checks to the shared validator"
  grep -q 'TARS_VALIDATE_PROFILE="${tool_plan_profile}"' "${live_path}" || \
    fail "live-validate should pass through targeted tool-plan profile selection"
  grep -q 'TARS_VALIDATE_SMOKE_SCENARIOS="${TARS_VALIDATE_SMOKE_SCENARIOS:-logs,observability,delivery}"' "${live_path}" || \
    fail "live-validate should default tool-plan smoke to logs, observability, and delivery scenarios"
  grep -q 'scenario=%s session_id=%s status=%s attachments=%s executions=%s tools=%s summary=%s' "${tool_plan_path}" || \
    fail "validate_tool_plan_live should print per-scenario smoke evidence for logs, observability, and delivery"
}

test_tool_plan_smoke_validator_accepts_connector_artifacts_without_fixture_specific_names() {
  local validator_path="${ROOT_DIR}/scripts/lib/validate_tool_plan_smoke.py"
  local fixture_path

  fixture_path="$(mktemp)"
  python3 - <<'PY' "${fixture_path}"
import json
import sys

detail = {
    "status": "resolved",
    "diagnosis_summary": "logs.query: matched shared marker; observability.query: alert evidence available",
    "executions": [],
    "attachments": [
        {"name": "metrics-range.json"},
        {"name": "metrics-range.png"},
        {"name": "shared-host-logs.json"},
        {"name": "alerts-evidence.json"},
    ],
    "tool_plan": [
        {
            "tool": "metrics.query_range",
            "status": "completed",
            "output": {"series_count": 1, "points": 12},
        },
        {
            "tool": "logs.query",
            "status": "completed",
            "output": {
                "artifact_count": 1,
                "result": {"summary": "matched shared marker"},
            },
        },
        {
            "tool": "observability.query",
            "status": "completed",
            "output": {
                "artifact_count": 1,
                "result": {"summary": "alert evidence available", "result_count": 2},
            },
        },
    ],
}

with open(sys.argv[1], "w", encoding="utf-8") as fh:
    json.dump(detail, fh)
PY

  python3 "${validator_path}" --scenario logs --detail-file "${fixture_path}" >/dev/null || \
    fail "tool-plan smoke validator should accept connector artifacts without fixture-specific filenames or summary wording"
}

test_tool_plan_smoke_validator_accepts_logs_scenario_without_metrics_attachments() {
  local validator_path="${ROOT_DIR}/scripts/lib/validate_tool_plan_smoke.py"
  local fixture_path

  fixture_path="$(mktemp)"
  python3 - <<'PY' "${fixture_path}"
import json
import sys

detail = {
    "status": "resolved",
    "diagnosis_summary": "metrics.query_range: series_count=2, points=13; logs.query: victorialogs returned 5 log entry/entries; observability.query: returned 2 alert(s), 2 firing",
    "executions": [],
    "attachments": [
        {"name": "victorialogs-result.json"},
        {"name": "observability-result.json"},
    ],
    "tool_plan": [
        {
            "tool": "metrics.query_range",
            "status": "completed",
            "output": {"series_count": 2, "points": 13},
        },
        {
            "tool": "logs.query",
            "status": "completed",
            "output": {
                "artifact_count": 1,
                "result": {"summary": "victorialogs returned 5 log entry/entries", "result_count": 5},
            },
        },
        {
            "tool": "observability.query",
            "status": "completed",
            "output": {
                "artifact_count": 1,
                "result": {"summary": "returned 2 alert(s), 2 firing", "result_count": 2},
            },
        },
    ],
}

with open(sys.argv[1], "w", encoding="utf-8") as fh:
    json.dump(detail, fh)
PY

  python3 "${validator_path}" --scenario logs --detail-file "${fixture_path}" >/dev/null || \
    fail "tool-plan smoke validator should accept logs scenario evidence even when session attachments omit metrics-range artifacts"
}

test_tool_plan_smoke_validator_rejects_execution_fallbacks() {
  local validator_path="${ROOT_DIR}/scripts/lib/validate_tool_plan_smoke.py"
  local fixture_path

  fixture_path="$(mktemp)"
  python3 - <<'PY' "${fixture_path}"
import json
import sys

detail = {
    "status": "resolved",
    "diagnosis_summary": "已分析请求：artifact_count=1",
    "executions": [],
    "attachments": [{"name": "metrics-range.json"}, {"name": "metrics-range.png"}],
    "tool_plan": [
        {
            "tool": "metrics.query_range",
            "status": "completed",
            "output": {"series_count": 1, "points": 12},
        },
        {
            "tool": "execution.run_command",
            "status": "completed",
            "output": {"status": "completed"},
        },
    ],
}

with open(sys.argv[1], "w", encoding="utf-8") as fh:
    json.dump(detail, fh)
PY

  if python3 "${validator_path}" --scenario metrics --detail-file "${fixture_path}" >/dev/null 2>&1; then
    fail "tool-plan smoke validator should reject execution.run_command and generic artifact_count summaries"
  fi
}

test_deploy_merges_shared_test_env_instead_of_skipping_it() {
  local deploy_path="${ROOT_DIR}/scripts/deploy_team_shared.sh"

  grep -q 'rm -f "${tmp_sync_dir}/shared-test.env"' "${deploy_path}" && \
    fail "deploy script should merge shared-test.env with remote canonical values instead of skipping the file entirely"
  grep -q 'scp "${tmp_sync_dir}/shared-test.env"' "${deploy_path}" || \
    fail "deploy script should continue syncing shared-test.env after merge"
}

test_deploy_syncs_tool_plan_live_dependencies() {
  local deploy_path="${ROOT_DIR}/scripts/deploy_team_shared.sh"

  grep -q "'\${REMOTE_SHARED_DIR}/lib'" "${deploy_path}" || \
    fail "deploy script should create the remote validate_tool_plan_live dependency directory"
  grep -q "'\${REMOTE_BASE_DIR}/scripts/lib'" "${deploy_path}" || \
    fail "deploy script should create the remote shared token helper directory"
  grep -q 'validate_tool_plan_smoke.py' "${deploy_path}" || \
    fail "deploy script should sync the tool-plan smoke validator with validate_tool_plan_live.sh"
  grep -q 'shared_ops_token.sh' "${deploy_path}" || \
    fail "deploy script should sync the shared ops token helper with validate_tool_plan_live.sh"
}

test_restart_helper_uses_single_ssh_with_env_and_pid_checks() {
  (
    local helper_path="${ROOT_DIR}/scripts/lib/shared_remote_service.sh"
    [[ -f "${helper_path}" ]] || fail "missing ${helper_path}"
    source "${helper_path}"

    SSH_CALL_COUNT=0
    SSH_CAPTURED_STDIN=""
    SSH_CAPTURED_ARGS=()

    ssh() {
      SSH_CALL_COUNT=$((SSH_CALL_COUNT + 1))
      SSH_CAPTURED_ARGS=("$@")
      SSH_CAPTURED_STDIN="$(cat)"
    }

    shared_remote_service_restart \
      "root@192.168.3.100" \
      "/data/tars-setup-lab/team-shared" \
      "/data/tars-setup-lab/bin/tars-linux-amd64-dev" \
      "/data/tars-setup-lab/team-shared/tars-dev.log"

    [[ "${SSH_CALL_COUNT}" -eq 1 ]] || fail "expected exactly one ssh restart call"
    [[ "${SSH_CAPTURED_ARGS[0]}" == "root@192.168.3.100" ]] || fail "expected restart helper to target the remote host"
    [[ "${SSH_CAPTURED_ARGS[1]}" == "bash" ]] || fail "expected restart helper to invoke remote bash"
    [[ "${SSH_CAPTURED_ARGS[2]}" == "-s" ]] || fail "expected restart helper to stream a single remote bash script"
    [[ "${SSH_CAPTURED_ARGS[3]}" == "--" ]] || fail "expected restart helper to pass remote args via --"
    [[ "${SSH_CAPTURED_ARGS[4]}" == "/data/tars-setup-lab/team-shared" ]] || fail "expected shared dir arg"
    [[ "${SSH_CAPTURED_ARGS[5]}" == "/data/tars-setup-lab/bin/tars-linux-amd64-dev" ]] || fail "expected binary path arg"
    [[ "${SSH_CAPTURED_ARGS[6]}" == "/data/tars-setup-lab/team-shared/tars-dev.log" ]] || fail "expected log path arg"
    [[ "${SSH_CAPTURED_ARGS[7]}" == "/data/tars-setup-lab/team-shared/shared-test.env" ]] || fail "expected env file arg"
    [[ "${SSH_CAPTURED_ARGS[8]}" == "/data/tars-setup-lab/team-shared/tars-dev.pid" ]] || fail "expected pid file arg"

    case "${SSH_CAPTURED_STDIN}" in
      *'source "${env_file}"'* ) ;;
      *) fail "expected restart helper to source shared-test.env before starting the binary" ;;
    esac

    case "${SSH_CAPTURED_STDIN}" in
      *'printf '\''%s\n'\'' "${pid}" >"${pid_file}"'* ) ;;
      *) fail "expected restart helper to persist the new pid" ;;
    esac

    case "${SSH_CAPTURED_STDIN}" in
      *'kill -0 "${pid}" 2>/dev/null'* ) ;;
      *) fail "expected restart helper to detect immediate process exit" ;;
    esac

    case "${SSH_CAPTURED_STDIN}" in
      *'pgrep -f -x -- "${binary_path}"'* ) ;;
      *) fail "expected restart helper to match existing processes by exact command line" ;;
    esac

    case "${SSH_CAPTURED_STDIN}" in
      *'pgrep -f -- "tars-linux-.*-dev"'* ) ;;
      *) fail "expected restart helper to sweep legacy tars dev processes on alternate paths" ;;
    esac

    case "${SSH_CAPTURED_STDIN}" in
      *'ps -p "${pid}" -o args='* ) ;;
      *) fail "expected restart helper to verify matched processes are actual tars binaries before stopping them" ;;
    esac

    case "${SSH_CAPTURED_STDIN}" in
      *'pkill -f'* ) fail "restart helper should not use broad pkill -f matching" ;;
      * ) ;;
    esac
  )
}

test_reset_access_baseline_uses_real_schema() {
  local reset_path="${ROOT_DIR}/scripts/reset_lab_192.168.3.100.sh"

  python3 - <<'PY' "${reset_path}" || fail "reset script should write access.shared.yaml with the real access schema"
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text()
marker = 'cat >"${shared_dir}/access.shared.yaml" <<\'YAML\'\n'
if marker not in text:
    raise SystemExit(1)
snippet = text.split(marker, 1)[1].split('\nYAML', 1)[0]
required = ["users: []", "groups: []", "auth_providers: []", "roles: []", "people: []", "channels: []"]
if any(item not in snippet for item in required):
    raise SystemExit(1)
if "entries: []" in snippet:
    raise SystemExit(1)
PY
}

test_reset_script_uses_backup_guardrails() {
  local reset_path="${ROOT_DIR}/scripts/reset_lab_192.168.3.100.sh"

  grep -q 'reset-backups' "${reset_path}" || fail "reset script should backup into reset-backups"
  grep -q 'shared_remote_service_restart' "${reset_path}" || fail "reset script should reuse shared remote restart helper"
  ! grep -q 'pkill -f' "${reset_path}" || fail "reset script should not use broad pkill -f"
}

test_seeded_observability_fixture_contains_victorialogs_marker() {
  local tmp_dir fixture_log host_file_log
  tmp_dir="$(mktemp -d)"
  fixture_log="${tmp_dir}/observability-main.log"
  host_file_log="${tmp_dir}/tars-observability-test.log"

  TARS_HOST_FILE_LOG_PATH="${host_file_log}" bash "${ROOT_DIR}/scripts/seed_team_shared_fixtures.sh" "${tmp_dir}" >/dev/null

  grep -q 'tars-observability-host-file-test' "${fixture_log}" || \
    fail "seed_team_shared_fixtures.sh should include the victorialogs validation marker in observability-main.log"
  grep -q 'tars-observability-host-file-test' "${host_file_log}" || \
    fail "seed_team_shared_fixtures.sh should also stamp the live host-file log consumed by promtail"

  python3 - <<'PY' "${fixture_log}" || fail "seed_team_shared_fixtures.sh should stamp the victorialogs marker with a fresh timestamp"
from datetime import datetime, timezone
import json
import pathlib
import sys

lines = pathlib.Path(sys.argv[1]).read_text().splitlines()
marker = None
for line in lines:
    record = json.loads(line)
    if "tars-observability-host-file-test" in str(record.get("message", "")):
        marker = record
        break

if marker is None:
    raise SystemExit(1)

timestamp = marker.get("time")
if not timestamp:
    raise SystemExit(1)

dt = datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
age = abs((datetime.now(timezone.utc) - dt.astimezone(timezone.utc)).total_seconds())
if age > 600:
    raise SystemExit(1)
PY

  python3 - <<'PY' "${host_file_log}" || fail "seed_team_shared_fixtures.sh should stamp the host-file marker with a fresh timestamp"
from datetime import datetime, timezone
import pathlib
import re
import sys

text = pathlib.Path(sys.argv[1]).read_text().strip().splitlines()
marker = next((line for line in text if "tars-observability-host-file-test" in line), "")
if not marker:
    raise SystemExit(1)

match = re.search(r"ts=([0-9T:\-]+Z)", marker)
if not match:
    raise SystemExit(1)

dt = datetime.fromisoformat(match.group(1).replace("Z", "+00:00"))
age = abs((datetime.now(timezone.utc) - dt.astimezone(timezone.utc)).total_seconds())
if age > 600:
    raise SystemExit(1)
PY
}

test_observability_validation_default_window_covers_shared_marker_age() {
  local observability_path="${ROOT_DIR}/scripts/validate_observability_connectors_live.sh"

  grep -q 'VL_TIME_RANGE="${TARS_VALIDATE_VL_TIME_RANGE:-168h}"' "${observability_path}" || \
    fail "validate_observability_connectors_live should use a freshness window that still proves recent shared fixture ingestion"
}

test_live_validate_refreshes_shared_victorialogs_marker() {
  local live_validate_path="${ROOT_DIR}/scripts/ci/live-validate.sh"
  local observability_path="${ROOT_DIR}/scripts/validate_observability_connectors_live.sh"

  grep -q 'marker_remote_host="192.168.3.9"' "${live_validate_path}" || \
    fail "live-validate should default the shared victorialogs host-file refresh target to 192.168.3.9"
  grep -q 'refresh_vl_marker_if_configured' "${observability_path}" || \
    fail "validate_observability_connectors_live should refresh the shared victorialogs host-file marker when configured"
}

test_local_placeholder_token_is_not_accepted_as_override
test_local_placeholder_token_falls_back_to_remote
test_remote_placeholder_token_is_rejected
test_local_real_token_is_accepted
test_shared_host_token_fallback_defaults_remote_user
test_sync_only_uses_local_token_override
test_check_shared_lab_script_exists_and_is_executable
test_check_shared_lab_script_enforces_canonical_runtime_and_endpoints
test_shared_lab_verification_docs_and_template_exist
test_shared_remote_service_restart_sets_canonical_workdir
test_deploy_normalizes_local_placeholder_before_remote_fallback
test_smoke_scripts_normalize_placeholder_before_remote_fallback
test_tool_plan_live_validate_requires_monitoring_first_tools
test_tool_plan_smoke_validator_accepts_connector_artifacts_without_fixture_specific_names
test_tool_plan_smoke_validator_accepts_logs_scenario_without_metrics_attachments
test_tool_plan_smoke_validator_rejects_execution_fallbacks
test_deploy_merges_shared_test_env_instead_of_skipping_it
test_deploy_syncs_tool_plan_live_dependencies
test_restart_helper_uses_single_ssh_with_env_and_pid_checks
test_reset_access_baseline_uses_real_schema
test_reset_script_uses_backup_guardrails
test_seeded_observability_fixture_contains_victorialogs_marker
test_observability_validation_default_window_covers_shared_marker_age
test_live_validate_refreshes_shared_victorialogs_marker

printf 'shared-deploy-regression=passed\n'
