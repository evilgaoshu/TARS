#!/usr/bin/env bash
set -euo pipefail

PORT="${TARS_SHARED_LAB_PORT:-8081}"
HOST="${TARS_SHARED_LAB_HOST:-127.0.0.1}"
BASE_URL="${TARS_SHARED_LAB_BASE_URL:-http://${HOST}:${PORT}}"
CANONICAL_BASE_DIR="${TARS_SHARED_LAB_CANONICAL_BASE_DIR:-/data/tars-setup-lab}"
SESSION_URL_INPUT="${1:-${TARS_SHARED_LAB_SESSION_URL:-}}"
EXPECTED_GIT_HEAD="${TARS_SHARED_LAB_EXPECTED_GIT_HEAD:-${GITHUB_SHA:-}}"

declare -a BLOCKERS=()
FAIL_COUNT=0

timestamp_utc() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

trim() {
  local value="${1:-}"
  value="${value//$'\r'/}"
  value="${value//$'\n'/}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

normalize_token() {
  local token
  token="$(trim "${1:-}")"
  case "${token}" in
    ""|placeholder|PLACEHOLDER|REPLACE_WITH_*)
      return 1
      ;;
  esac
  printf '%s\n' "${token}"
}

emit_meta() {
  printf '%s: %s\n' "$1" "$2"
}

emit_check() {
  printf 'check.%s: %s %s\n' "$1" "$2" "$3"
}

fail_check() {
  local name="$1"
  local detail="$2"
  emit_check "${name}" "FAIL" "${detail}"
  BLOCKERS+=("${detail}")
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

pass_check() {
  local name="$1"
  local detail="$2"
  emit_check "${name}" "PASS" "${detail}"
}

listener_pid() {
  if command -v lsof >/dev/null 2>&1; then
    lsof -tiTCP:"${PORT}" -sTCP:LISTEN 2>/dev/null | head -n 1
    return 0
  fi

  if command -v ss >/dev/null 2>&1; then
    ss -ltnp 2>/dev/null | awk -v port=":${PORT}" '
      index($4, port) && match($0, /pid=[0-9]+/) {
        value = substr($0, RSTART + 4, RLENGTH - 4)
        print value
        exit
      }
    '
    return 0
  fi

  return 1
}

resolve_env_file() {
  local workdir="$1"
  local candidate

  for candidate in \
    "${TARS_SHARED_LAB_ENV_FILE:-}" \
    "${workdir}/team-shared/shared-test.env" \
    "${CANONICAL_BASE_DIR}/team-shared/shared-test.env"
  do
    [[ -n "${candidate}" ]] || continue
    if [[ -f "${candidate}" ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done

  return 1
}

read_env_value() {
  local file_path="$1"
  local key="$2"

  sed -n "s/^${key}=//p" "${file_path}" | head -n 1
}

resolve_token() {
  local env_file="${1:-}"
  local token=""

  token="$(normalize_token "${TARS_OPS_API_TOKEN:-}" 2>/dev/null || true)"
  if [[ -n "${token}" ]]; then
    printf '%s\n' "${token}"
    return 0
  fi

  if [[ -n "${env_file}" && -f "${env_file}" ]]; then
    token="$(read_env_value "${env_file}" "TARS_OPS_API_TOKEN")"
    token="$(normalize_token "${token}" 2>/dev/null || true)"
    if [[ -n "${token}" ]]; then
      printf '%s\n' "${token}"
      return 0
    fi
  fi

  return 1
}

resolve_session_url() {
  if [[ -z "${SESSION_URL_INPUT}" ]]; then
    return 1
  fi

  case "${SESSION_URL_INPUT}" in
    http://*|https://*)
      printf '%s\n' "${SESSION_URL_INPUT}"
      ;;
    /*)
      printf '%s%s\n' "${BASE_URL}" "${SESSION_URL_INPUT}"
      ;;
    *)
      printf '%s/%s\n' "${BASE_URL}" "${SESSION_URL_INPUT}"
      ;;
  esac
}

http_status() {
  local method="$1"
  local url="$2"
  local payload="${3:-}"
  local auth_header="${4:-}"

  if [[ -n "${payload}" ]]; then
    if [[ -n "${auth_header}" ]]; then
      curl -sS -o /tmp/tars-shared-lab-http-body.$$ -w '%{http_code}' \
        -H 'Content-Type: application/json' \
        -H "${auth_header}" \
        -X "${method}" "${url}" \
        --data "${payload}"
    else
      curl -sS -o /tmp/tars-shared-lab-http-body.$$ -w '%{http_code}' \
        -H 'Content-Type: application/json' \
        -X "${method}" "${url}" \
        --data "${payload}"
    fi
  else
    if [[ -n "${auth_header}" ]]; then
      curl -sS -o /tmp/tars-shared-lab-http-body.$$ -w '%{http_code}' \
        -H "${auth_header}" \
        -X "${method}" "${url}"
    else
      curl -sS -o /tmp/tars-shared-lab-http-body.$$ -w '%{http_code}' \
        -X "${method}" "${url}"
    fi
  fi
}

cleanup_body_file() {
  rm -f /tmp/tars-shared-lab-http-body.$$ 2>/dev/null || true
}

main() {
  local now hostname pid exe_path workdir env_file repo_head expected_head
  local env_dump path_summary path_failures token login_payload login_status session_token auth_header
  local setup_status setup_summary session_url session_status

  trap cleanup_body_file EXIT

  now="$(timestamp_utc)"
  hostname="$(hostname)"
  expected_head="${EXPECTED_GIT_HEAD:-n/a}"
  repo_head="$(git rev-parse HEAD 2>/dev/null || true)"
  [[ -n "${repo_head}" ]] || repo_head="n/a"

  emit_meta "hostname" "${hostname}"
  emit_meta "timestamp_utc" "${now}"
  emit_meta "base_url" "${BASE_URL}"
  emit_meta "canonical_base_dir" "${CANONICAL_BASE_DIR}"
  emit_meta "expected_git_head" "${expected_head}"
  emit_meta "repo_git_head" "${repo_head}"

  pid="$(listener_pid 2>/dev/null || true)"
  if [[ -z "${pid}" ]]; then
    fail_check "listener_8081" "no LISTEN pid found on port ${PORT}"
    goto_summary=1
  else
    pass_check "listener_8081" "pid=${pid} port=${PORT}"
    goto_summary=0
  fi

  if [[ "${goto_summary}" -eq 0 ]]; then
    exe_path="$(readlink -f "/proc/${pid}/exe" 2>/dev/null || true)"
    workdir="$(readlink -f "/proc/${pid}/cwd" 2>/dev/null || true)"
    env_dump="$(tr '\0' '\n' < "/proc/${pid}/environ" 2>/dev/null || true)"

    if [[ -n "${exe_path}" && "${exe_path}" == ${CANONICAL_BASE_DIR}/* ]]; then
      pass_check "binary_path" "${exe_path}"
    else
      fail_check "binary_path" "binary path outside canonical shared lab root: ${exe_path:-unavailable}"
    fi

    if [[ -n "${workdir}" && ("${workdir}" == "${CANONICAL_BASE_DIR}" || "${workdir}" == ${CANONICAL_BASE_DIR}/*) ]]; then
      pass_check "workdir_path" "${workdir}"
    else
      fail_check "workdir_path" "workdir/config points outside canonical shared lab root: ${workdir:-unavailable}"
    fi

    path_summary=""
    path_failures=0
    while IFS= read -r line; do
      local_name="${line%%=*}"
      local_value="${line#*=}"
      case "${local_name}" in
        TARS_*CONFIG_PATH|TARS_*STATE_PATH|TARS_*SQLITE_PATH|TARS_*DIST_DIR|TARS_*DATA_DIR|TARS_*SPOOL_DIR|TARS_SERVER_LISTEN)
          ;;
        *)
          continue
          ;;
      esac

      if [[ -n "${path_summary}" ]]; then
        path_summary="${path_summary}; "
      fi
      path_summary="${path_summary}${local_name}=${local_value}"

      case "${local_name}" in
        TARS_SERVER_LISTEN)
          ;;
        *)
          if [[ "${local_value}" != "${CANONICAL_BASE_DIR}" && "${local_value}" != ${CANONICAL_BASE_DIR}/* ]]; then
            path_failures=1
          fi
          ;;
      esac
    done <<< "${env_dump}"

    if [[ -z "${path_summary}" ]]; then
      fail_check "config_paths" "no TARS_* path variables found in process environment"
    elif [[ "${path_failures}" -ne 0 ]]; then
      fail_check "config_paths" "workdir/config points outside canonical shared lab root: ${path_summary}"
    else
      pass_check "config_paths" "${path_summary}"
    fi

    env_file="$(resolve_env_file "${workdir}" 2>/dev/null || true)"
    if [[ -n "${env_file}" && "${env_file}" == ${CANONICAL_BASE_DIR}/* ]]; then
      pass_check "shared_env_file" "${env_file}"
    elif [[ -n "${env_file}" ]]; then
      fail_check "shared_env_file" "shared env file outside canonical shared lab root: ${env_file}"
    else
      fail_check "shared_env_file" "unable to locate shared-test.env under canonical shared lab root"
    fi

    token="$(resolve_token "${env_file}" 2>/dev/null || true)"
    if [[ -z "${token}" ]]; then
      fail_check "auth_token" "unable to resolve TARS_OPS_API_TOKEN from environment or shared-test.env"
    else
      pass_check "auth_token" "token_resolved=yes source=${env_file:-environment}"

      login_payload="$(TOKEN="${token}" python3 - <<'PY'
import json, os
print(json.dumps({"provider_id": "local_token", "token": os.environ["TOKEN"]}))
PY
)"
      login_status="$(http_status POST "${BASE_URL}/api/v1/auth/login" "${login_payload}")"
      if [[ "${login_status}" == "200" ]]; then
        session_token="$(python3 - <<'PY' /tmp/tars-shared-lab-http-body.$$
import json, sys
with open(sys.argv[1], encoding="utf-8") as fh:
    data = json.load(fh)
print(data.get("session_token", ""))
PY
)"
        pass_check "auth_login_local_token" "status=${login_status} session_token_present=$( [[ -n "${session_token}" ]] && printf yes || printf no )"
      else
        fail_check "auth_login_local_token" "local_token login failed with status=${login_status}"
      fi

      auth_header="Authorization: Bearer ${token}"
      setup_status="$(http_status GET "${BASE_URL}/api/v1/setup/status" "" "${auth_header}")"
      if [[ "${setup_status}" == "200" ]]; then
        setup_summary="$(python3 - <<'PY' /tmp/tars-shared-lab-http-body.$$
import json, sys
with open(sys.argv[1], encoding="utf-8") as fh:
    data = json.load(fh)
print("initialization=%s rollout_mode=%s" % (data.get("initialization", {}).get("initialized", "unknown"), data.get("rollout_mode", "unknown")))
PY
)"
        pass_check "setup_status_endpoint" "status=${setup_status} ${setup_summary}"
      else
        fail_check "setup_status_endpoint" "setup/status failed with status=${setup_status}"
      fi

      session_url="$(resolve_session_url 2>/dev/null || true)"
      if [[ -z "${session_url}" ]]; then
        fail_check "session_url" "missing session URL input; pass it as the first argument or TARS_SHARED_LAB_SESSION_URL"
      else
        session_status="$(http_status GET "${session_url}" "" "${auth_header}")"
        case "${session_status}" in
          200|301|302|307|308)
            pass_check "session_url" "status=${session_status} url=${session_url}"
            ;;
          *)
            fail_check "session_url" "session URL failed with status=${session_status} url=${session_url}"
            ;;
        esac
      fi
    fi
  fi

  if [[ "${FAIL_COUNT}" -eq 0 ]]; then
    emit_meta "overall" "PASS"
    exit 0
  fi

  emit_meta "overall" "FAIL blockers=${FAIL_COUNT}"
  local index=1
  for blocker in "${BLOCKERS[@]}"; do
    emit_meta "blocker.${index}" "${blocker}"
    index=$((index + 1))
  done
  exit 1
}

main "$@"
