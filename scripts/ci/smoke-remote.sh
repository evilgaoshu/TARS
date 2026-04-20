#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"
STARTED_AT=${SECONDS}
REMOTE_HOST="${TARS_REMOTE_HOST:-192.168.3.100}"
REMOTE_USER="$(shared_ops_token_remote_user 2>/dev/null || true)"
OPS_BASE_URL="${TARS_OPS_BASE_URL:-http://${REMOTE_HOST}:8081}"
OPS_TOKEN="$(shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}" 2>/dev/null || true)"
RUN_WEB_SMOKE="${TARS_SMOKE_REMOTE_RUN_WEB:-0}"
SSH_TARGET="${REMOTE_USER}@${REMOTE_HOST}"

if [[ -z "${REMOTE_USER}" ]]; then
  echo "TARS_REMOTE_USER is required for smoke-remote" >&2
  exit 1
fi

if [[ -z "${OPS_TOKEN}" ]] && OPS_TOKEN="$(shared_ops_token_resolve 2>/dev/null || true)"; then
  :
fi

if [[ -z "${OPS_TOKEN}" ]]; then
  echo "failed to resolve TARS_OPS_API_TOKEN for smoke-remote" >&2
  echo "tip=脚本已自动尝试远端 canonical shared-test.env 回退；192.168.3.100 默认使用 root。若不是默认 SSH 用户，再显式设置 TARS_REMOTE_USER。" >&2
  exit 1
fi

run_step() {
  local title="$1"
  shift
  local step_started=${SECONDS}

  echo "== ${title} =="
  "$@"
  echo "-- ok ($((SECONDS - step_started))s)"
  echo
}

probe_remote_path() {
  local path="$1"
  local output

  if ! output="$(ssh -o BatchMode=yes -o ConnectTimeout=5 "${SSH_TARGET}" "curl -fsS http://127.0.0.1:8081${path}" 2>&1)"; then
    echo "probe_failed=${path}" >&2
    echo "target=${SSH_TARGET}" >&2
    echo "tip=检查共享环境服务是否已启动，或确认当前 SSH key 对 ${SSH_TARGET} 可用。" >&2
    printf '%s\n' "${output}" >&2
    return 1
  fi

  printf '%s\n' "${output}"
}

run_hygiene() {
  if ! env \
    TARS_OPS_BASE_URL="${OPS_BASE_URL}" \
    TARS_OPS_API_TOKEN="${OPS_TOKEN}" \
    bash "${ROOT_DIR}/scripts/pilot_hygiene_check.sh"; then
    echo "pilot_hygiene_check=failed" >&2
    echo "tip=优先检查 TARS_OPS_BASE_URL/TARS_OPS_API_TOKEN，或在远端查看共享目录内的 tars-dev.log。" >&2
	return 1
	fi
}

run_performance_spot_check() {
	if ! env \
	  TARS_REMOTE_HOST="${REMOTE_HOST}" \
	  TARS_REMOTE_USER="${REMOTE_USER}" \
	  TARS_REMOTE_BASE_DIR="${TARS_REMOTE_BASE_DIR:-/data/tars-setup-lab}" \
	  TARS_OPS_BASE_URL="${OPS_BASE_URL}" \
	  TARS_OPS_API_TOKEN="${OPS_TOKEN}" \
	  bash "${ROOT_DIR}/scripts/ci/performance-spot-check.sh"; then
		echo "performance_spot_check=failed" >&2
		echo "tip=优先检查远端进程是否已正常启动、/api/v1/dashboard/health 是否可用、以及 /metrics 是否暴露 store 指标。" >&2
		return 1
	fi
}

echo "== TARS remote smoke =="
echo "scope=L3 shared environment readiness + hygiene"
echo "remote=${SSH_TARGET}"
echo "ops_base_url=${OPS_BASE_URL}"
echo

run_step "Remote healthz" probe_remote_path "/healthz"
run_step "Remote readyz" probe_remote_path "/readyz"
run_step "Remote discovery" probe_remote_path "/api/v1/platform/discovery"
run_step "Ops hygiene" run_hygiene
run_step "Performance spot-check" run_performance_spot_check

if [[ "${RUN_WEB_SMOKE}" == "1" ]]; then
  run_step \
    "Web control-plane smoke" \
    env \
      TARS_PLAYWRIGHT_BASE_URL="${OPS_BASE_URL}" \
      TARS_PLAYWRIGHT_TOKEN="${OPS_TOKEN}" \
      bash "${ROOT_DIR}/scripts/ci/web-smoke.sh"
else
  echo "== Web control-plane smoke =="
  echo "-- skipped (set TARS_SMOKE_REMOTE_RUN_WEB=1 to include Playwright smoke)"
  echo
fi

echo "smoke-remote=passed total=$((SECONDS - STARTED_AT))s"
