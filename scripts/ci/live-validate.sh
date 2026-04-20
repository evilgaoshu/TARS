#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"
STARTED_AT=${SECONDS}
REMOTE_HOST="${TARS_REMOTE_HOST:-192.168.3.100}"
REMOTE_USER="$(shared_ops_token_remote_user 2>/dev/null || true)"
OPS_BASE_URL="${TARS_OPS_BASE_URL:-http://${REMOTE_HOST}:8081}"
OPS_TOKEN="$(shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}" 2>/dev/null || true)"
PROFILE="${TARS_LIVE_VALIDATE_PROFILE:-core}"

cd "${ROOT_DIR}"

require_ops_token() {
  if [[ -z "${OPS_TOKEN}" ]] && OPS_TOKEN="$(shared_ops_token_resolve 2>/dev/null || true)"; then
    :
  fi
  if [[ -z "${OPS_TOKEN}" ]]; then
    echo "failed to resolve TARS_OPS_API_TOKEN for live validation" >&2
    echo "tip=脚本已自动尝试远端 canonical shared-test.env 回退；192.168.3.100 默认使用 root。若不是默认 SSH 用户，再显式设置 TARS_REMOTE_USER。" >&2
    return 1
  fi
}

require_remote_user() {
  if [[ -z "${REMOTE_USER}" ]]; then
    echo "TARS_REMOTE_USER is required for remote hygiene validation" >&2
    echo "tip=192.168.3.100 会默认使用 root；其他主机请显式设置 TARS_REMOTE_USER。" >&2
    return 1
  fi
}

resolve_default() {
  local name="$1"

  case "${PROFILE}" in
    core)
      case "${name}" in
        hygiene|auth|extensions|web) echo 0 ;;
        observability) echo 1 ;;
        *) return 1 ;;
      esac
      ;;
    full)
      case "${name}" in
        hygiene|auth|observability) echo 1 ;;
        extensions|web) echo 0 ;;
        *) return 1 ;;
      esac
      ;;
    exhaustive)
      case "${name}" in
        hygiene|auth|extensions|web|observability) echo 1 ;;
        *) return 1 ;;
      esac
      ;;
    *)
      echo "unsupported profile: ${PROFILE}" >&2
      echo "supported profiles: core, full, exhaustive" >&2
      return 1
      ;;
  esac
}

INCLUDE_HYGIENE="${TARS_LIVE_VALIDATE_INCLUDE_HYGIENE:-$(resolve_default hygiene)}"
INCLUDE_AUTH="${TARS_LIVE_VALIDATE_INCLUDE_AUTH:-$(resolve_default auth)}"
INCLUDE_EXTENSIONS="${TARS_LIVE_VALIDATE_INCLUDE_EXTENSIONS:-$(resolve_default extensions)}"
INCLUDE_WEB="${TARS_LIVE_VALIDATE_INCLUDE_WEB:-$(resolve_default web)}"
INCLUDE_OBSERVABILITY="${TARS_LIVE_VALIDATE_INCLUDE_OBSERVABILITY:-$(resolve_default observability)}"

run_step() {
  local title="$1"
  shift
  local step_started=${SECONDS}

  echo "== ${title} =="
  "$@"
  echo "-- ok ($((SECONDS - step_started))s)"
  echo
}

run_tool_plan_validation() {
  require_ops_token
  local tool_plan_profile="${TARS_VALIDATE_PROFILE:-all}"
  if [[ "${tool_plan_profile}" == "all" && -n "${TARS_LIVE_VALIDATE_TOOL_PLAN_PROFILE:-}" ]]; then
    tool_plan_profile="${TARS_LIVE_VALIDATE_TOOL_PLAN_PROFILE}"
  fi
  if ! env \
    TARS_OPS_BASE_URL="${OPS_BASE_URL}" \
    TARS_OPS_API_TOKEN="${OPS_TOKEN}" \
    TARS_VALIDATE_PROFILE="${tool_plan_profile}" \
    TARS_VALIDATE_RUN_SMOKE="${TARS_VALIDATE_RUN_SMOKE:-1}" \
    TARS_VALIDATE_SMOKE_SCENARIOS="${TARS_VALIDATE_SMOKE_SCENARIOS:-logs,observability,delivery}" \
    bash "${ROOT_DIR}/scripts/validate_tool_plan_live.sh"; then
    echo "tool_plan_live_validation=failed" >&2
    echo "tip=优先检查共享环境 readiness、connector 基线和 ops token。" >&2
    return 1
  fi
}

run_auth_validation() {
  require_ops_token
  if ! env \
    TARS_OPS_BASE_URL="${OPS_BASE_URL}" \
    TARS_OPS_API_TOKEN="${OPS_TOKEN}" \
    bash "${ROOT_DIR}/scripts/validate_auth_enhancements_live.sh"; then
    echo "auth_live_validation=failed" >&2
    echo "tip=检查 local_password provider、共享测试账号和 MFA 基线是否仍在。" >&2
    return 1
  fi
}

run_extensions_validation() {
  require_ops_token
  if ! env \
    TARS_OPS_BASE_URL="${OPS_BASE_URL}" \
    TARS_OPS_API_TOKEN="${OPS_TOKEN}" \
    bash "${ROOT_DIR}/scripts/validate_extensions_live.sh"; then
    echo "extensions_live_validation=failed" >&2
    echo "tip=该验证会写入一个新的 skill 样本，失败时请检查 /extensions 与 /skills 控制面状态。" >&2
    return 1
  fi
}

run_hygiene_validation() {
  require_ops_token
  require_remote_user
  env \
    TARS_REMOTE_HOST="${REMOTE_HOST}" \
    TARS_REMOTE_USER="${REMOTE_USER}" \
    TARS_OPS_BASE_URL="${OPS_BASE_URL}" \
    TARS_OPS_API_TOKEN="${OPS_TOKEN}" \
    TARS_SMOKE_REMOTE_RUN_WEB=0 \
    bash "${ROOT_DIR}/scripts/ci/smoke-remote.sh"
}

run_web_validation() {
  require_ops_token
  env \
    TARS_PLAYWRIGHT_BASE_URL="${OPS_BASE_URL}" \
    TARS_PLAYWRIGHT_TOKEN="${OPS_TOKEN}" \
    bash "${ROOT_DIR}/scripts/ci/web-smoke.sh"
}

run_observability_validation() {
  require_ops_token
  local marker_remote_host="${TARS_VALIDATE_VL_MARKER_REMOTE_HOST:-}"
  if [[ -z "${marker_remote_host}" && "${REMOTE_HOST}" == "192.168.3.100" ]]; then
    marker_remote_host="192.168.3.9"
  fi
  env \
    TARS_OPS_BASE_URL="${OPS_BASE_URL}" \
    TARS_OPS_API_TOKEN="${OPS_TOKEN}" \
    TARS_VALIDATE_VL_MARKER_REMOTE_HOST="${marker_remote_host}" \
    TARS_VALIDATE_VL_MARKER_REMOTE_USER="${TARS_VALIDATE_VL_MARKER_REMOTE_USER:-root}" \
    bash "${ROOT_DIR}/scripts/validate_observability_connectors_live.sh"
}

echo "== TARS live validation =="
echo "scope=L3 shared environment live validation"
echo "profile=${PROFILE} hygiene=${INCLUDE_HYGIENE} observability=${INCLUDE_OBSERVABILITY} auth=${INCLUDE_AUTH} extensions=${INCLUDE_EXTENSIONS} web=${INCLUDE_WEB}"
echo "ops_base_url=${OPS_BASE_URL}"
echo

if [[ "${INCLUDE_HYGIENE}" == "1" ]]; then
  run_step "Shared readiness and hygiene" run_hygiene_validation
fi

run_step "Tool-plan live validation" run_tool_plan_validation

if [[ "${INCLUDE_OBSERVABILITY}" == "1" ]]; then
  run_step "Observability connector live validation" run_observability_validation
fi

if [[ "${INCLUDE_AUTH}" == "1" ]]; then
  run_step "Auth live validation" run_auth_validation
fi

if [[ "${INCLUDE_EXTENSIONS}" == "1" ]]; then
  run_step "Extensions live validation" run_extensions_validation
fi

if [[ "${INCLUDE_WEB}" == "1" ]]; then
  run_step "Web control-plane smoke" run_web_validation
fi

echo "live-validate=passed total=$((SECONDS - STARTED_AT))s"
