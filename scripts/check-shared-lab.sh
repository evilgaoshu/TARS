#!/usr/bin/env bash
set -euo pipefail

PORT="${TARS_SHARED_LAB_PORT:-8081}"
HOST="${TARS_SHARED_LAB_HOST:-127.0.0.1}"
BASE_URL="${TARS_SHARED_LAB_BASE_URL:-http://${HOST}:${PORT}}"
CANONICAL_BASE_DIR="${TARS_SHARED_LAB_CANONICAL_BASE_DIR:-/data/tars-setup-lab}"
CANONICAL_OVERRIDE_FILE="${TARS_SHARED_LAB_CANONICAL_OVERRIDE_FILE:-${CANONICAL_BASE_DIR}/.canonical-override}"
SERVICE_NAME="${TARS_SHARED_LAB_SERVICE_NAME:-tars-shared-lab.service}"
SESSION_URL_INPUT="${1:-${TARS_SHARED_LAB_SESSION_URL:-}}"
EXPECTED_GIT_HEAD="${TARS_SHARED_LAB_EXPECTED_GIT_HEAD:-${GITHUB_SHA:-}}"

declare -a BLOCKERS=()
FAIL_COUNT=0
WARN_COUNT=0
OVERRIDE_ACCEPTED_PATH=""
OVERRIDE_OWNER_ID=""
OVERRIDE_DATE=""
OVERRIDE_REASON=""
OVERRIDE_EXPIRES_AT=""

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

strip_env_value() {
  local value
  value="$(trim "${1:-}")"
  case "${value}" in
    \"*\")
      value="${value#\"}"
      value="${value%\"}"
      ;;
    \'*\')
      value="${value#\'}"
      value="${value%\'}"
      ;;
  esac
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

warn_check() {
  local name="$1"
  local detail="$2"
  emit_check "${name}" "WARN" "${detail}"
  WARN_COUNT=$((WARN_COUNT + 1))
}

pass_check() {
  local name="$1"
  local detail="$2"
  emit_check "${name}" "PASS" "${detail}"
}

path_under_base() {
  local path_value="$1"
  local base_dir="$2"

  [[ -n "${path_value}" ]] || return 1
  [[ "${path_value}" == "${base_dir}" || "${path_value}" == "${base_dir}/"* ]]
}

check_path_value() {
  local name="$1"
  local path_value="$2"
  local fail_prefix="$3"

  if path_under_base "${path_value}" "${CANONICAL_BASE_DIR}"; then
    pass_check "${name}" "${path_value}"
    return 0
  fi

  if [[ -n "${OVERRIDE_ACCEPTED_PATH}" ]] && path_under_base "${path_value}" "${OVERRIDE_ACCEPTED_PATH}"; then
    warn_check "${name}" "owner-accepted non-canonical path: expected=${CANONICAL_BASE_DIR} accepted=${OVERRIDE_ACCEPTED_PATH} actual=${path_value}"
    return 0
  fi

  fail_check "${name}" "${fail_prefix}: expected=${CANONICAL_BASE_DIR} actual=${path_value:-unavailable}"
  return 1
}

path_is_allowed() {
  local path_value="$1"

  path_under_base "${path_value}" "${CANONICAL_BASE_DIR}" && return 0
  [[ -n "${OVERRIDE_ACCEPTED_PATH}" ]] && path_under_base "${path_value}" "${OVERRIDE_ACCEPTED_PATH}" && return 0
  return 1
}

path_uses_override() {
  local path_value="$1"

  [[ -n "${OVERRIDE_ACCEPTED_PATH}" ]] && path_under_base "${path_value}" "${OVERRIDE_ACCEPTED_PATH}"
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

  strip_env_value "$(sed -n "s/^${key}=//p" "${file_path}" | head -n 1)"
}

read_override_value() {
  local file_path="$1"
  local key="$2"

  strip_env_value "$(sed -n -E "s/^[[:space:]]*${key}[[:space:]]*=[[:space:]]*//p" "${file_path}" | head -n 1)"
}

override_expiry_is_valid() {
  local expires_at="$1"

  python3 - "${expires_at}" <<'PY'
from datetime import date, datetime
import sys

try:
    expires_at = datetime.strptime(sys.argv[1], "%Y-%m-%d").date()
except ValueError:
    raise SystemExit(1)

raise SystemExit(0 if expires_at >= date.today() else 1)
PY
}

load_canonical_override() {
  local missing=()

  if [[ ! -f "${CANONICAL_OVERRIDE_FILE}" ]]; then
    return 0
  fi

  OVERRIDE_ACCEPTED_PATH="$(read_override_value "${CANONICAL_OVERRIDE_FILE}" "accepted_path")"
  OVERRIDE_OWNER_ID="$(read_override_value "${CANONICAL_OVERRIDE_FILE}" "owner_id")"
  OVERRIDE_DATE="$(read_override_value "${CANONICAL_OVERRIDE_FILE}" "date")"
  OVERRIDE_REASON="$(read_override_value "${CANONICAL_OVERRIDE_FILE}" "reason")"
  OVERRIDE_EXPIRES_AT="$(read_override_value "${CANONICAL_OVERRIDE_FILE}" "expires_at")"

  [[ -n "${OVERRIDE_ACCEPTED_PATH}" ]] || missing+=("accepted_path")
  [[ -n "${OVERRIDE_OWNER_ID}" ]] || missing+=("owner_id")
  [[ -n "${OVERRIDE_DATE}" ]] || missing+=("date")
  [[ -n "${OVERRIDE_REASON}" ]] || missing+=("reason")
  [[ -n "${OVERRIDE_EXPIRES_AT}" ]] || missing+=("expires_at")

  if [[ "${#missing[@]}" -ne 0 ]]; then
    OVERRIDE_ACCEPTED_PATH=""
    fail_check "canonical_override" "invalid .canonical-override missing fields: ${missing[*]}"
    return 0
  fi

  if [[ "${OVERRIDE_ACCEPTED_PATH}" != /* ]]; then
    OVERRIDE_ACCEPTED_PATH=""
    fail_check "canonical_override" "invalid .canonical-override accepted_path must be absolute"
    return 0
  fi

  if ! override_expiry_is_valid "${OVERRIDE_EXPIRES_AT}"; then
    OVERRIDE_ACCEPTED_PATH=""
    fail_check "canonical_override" "invalid .canonical-override expires_at=${OVERRIDE_EXPIRES_AT} is expired or not YYYY-MM-DD"
    return 0
  fi

  warn_check "canonical_override" "non-canonical path accepted by owner until ${OVERRIDE_EXPIRES_AT}: accepted_path=${OVERRIDE_ACCEPTED_PATH} owner_id=${OVERRIDE_OWNER_ID} date=${OVERRIDE_DATE} reason=${OVERRIDE_REASON}"
}

runtime_head_candidates() {
  if [[ -n "${OVERRIDE_ACCEPTED_PATH}" ]]; then
    printf '%s\n' \
      "${OVERRIDE_ACCEPTED_PATH}/team-shared/runtime_git_head" \
      "${OVERRIDE_ACCEPTED_PATH}/runtime_git_head"
  fi
  printf '%s\n' \
    "${CANONICAL_BASE_DIR}/team-shared/runtime_git_head" \
    "${CANONICAL_BASE_DIR}/runtime_git_head"
}

resolve_runtime_git_head() {
  local candidate root

  while IFS= read -r candidate; do
    [[ -n "${candidate}" ]] || continue
    if [[ -f "${candidate}" ]]; then
      printf '%s\t%s\n' "$(trim "$(head -n 1 "${candidate}")")" "${candidate}"
      return 0
    fi
  done < <(runtime_head_candidates)

  for root in "${CANONICAL_BASE_DIR}" "${OVERRIDE_ACCEPTED_PATH}"; do
    [[ -n "${root}" ]] || continue
    if [[ -d "${root}/.git" ]]; then
      printf '%s\tgit:%s\n' "$(git -C "${root}" rev-parse HEAD 2>/dev/null || true)" "${root}"
      return 0
    fi
  done

  return 1
}

last_systemd_field() {
  local unit_text="$1"
  local key="$2"

  awk -F= -v key="${key}" '
    $0 !~ /^#/ && $1 == key {
      value = $0
      sub(/^[^=]*=/, "", value)
    }
    END { print value }
  ' <<< "${unit_text}"
}

check_managed_service_config() {
  local unit_text unit_workdir unit_exec unit_env_file unit_env

  if ! command -v systemctl >/dev/null 2>&1; then
    fail_check "managed_service_config" "systemctl unavailable; no systemd/supervisor managed runtime config could be inspected"
    return 0
  fi

  if ! unit_text="$(systemctl cat "${SERVICE_NAME}" 2>/dev/null)"; then
    fail_check "managed_service_config" "systemd unit ${SERVICE_NAME} not found; no supervisor fallback accepted for shared lab"
    return 0
  fi

  pass_check "managed_service_unit" "${SERVICE_NAME}"

  unit_workdir="$(last_systemd_field "${unit_text}" "WorkingDirectory")"
  check_path_value "managed_service_workdir" "${unit_workdir}" "systemd WorkingDirectory outside canonical shared lab root" || true

  unit_exec="$(last_systemd_field "${unit_text}" "ExecStart")"
  unit_exec="${unit_exec#-}"
  unit_exec="${unit_exec%% *}"
  check_path_value "managed_service_execstart" "${unit_exec}" "systemd ExecStart outside canonical shared lab root" || true

  unit_env_file="$(last_systemd_field "${unit_text}" "EnvironmentFile")"
  unit_env_file="${unit_env_file#-}"
  unit_env_file="${unit_env_file%% *}"
  if [[ -n "${unit_env_file}" ]]; then
    check_path_value "managed_service_environment_file" "${unit_env_file}" "systemd EnvironmentFile outside canonical shared lab root" || true
    return 0
  fi

  unit_env="$(last_systemd_field "${unit_text}" "Environment")"
  if [[ -n "${unit_env}" ]]; then
    case "${unit_env}" in
      *TARS_DIR=*)
        check_path_value "managed_service_environment" "${unit_env#*TARS_DIR=}" "systemd Environment TARS_DIR outside canonical shared lab root" || true
        ;;
      *)
        warn_check "managed_service_environment" "systemd Environment present but does not include TARS_DIR"
        ;;
    esac
    return 0
  fi

  fail_check "managed_service_environment" "systemd unit ${SERVICE_NAME} lacks EnvironmentFile or Environment entries"
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
  local env_dump path_summary path_failures path_warnings token login_payload login_status session_token auth_header
  local setup_status setup_summary session_url session_status
  local runtime_head runtime_head_source runtime_head_pair shared_env_tars_dir

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
  emit_meta "canonical_override_file" "${CANONICAL_OVERRIDE_FILE}"
  emit_meta "managed_service_name" "${SERVICE_NAME}"
  emit_meta "expected_git_head" "${expected_head}"
  emit_meta "repo_git_head" "${repo_head}"

  load_canonical_override

  runtime_head=""
  runtime_head_source=""
  runtime_head_pair=""
  runtime_head_pair="$(resolve_runtime_git_head 2>/dev/null || true)"
  if [[ -n "${runtime_head_pair}" ]]; then
    runtime_head="${runtime_head_pair%%$'\t'*}"
    runtime_head_source="${runtime_head_pair#*$'\t'}"
    emit_meta "runtime_git_head" "${runtime_head}"
    emit_meta "runtime_git_head_source" "${runtime_head_source}"
  fi

  if [[ -z "${expected_head}" || "${expected_head}" == "n/a" ]]; then
    warn_check "runtime_git_head" "expected git head not provided; set TARS_SHARED_LAB_EXPECTED_GIT_HEAD to bind evidence to a PR head"
  elif [[ -z "${runtime_head}" ]]; then
    fail_check "runtime_git_head" "unable to resolve deployed runtime git head; expected=${expected_head}"
  elif [[ "${runtime_head}" == "${expected_head}" ]]; then
    pass_check "runtime_git_head" "expected=${expected_head} actual=${runtime_head}"
  else
    fail_check "runtime_git_head" "runtime git head mismatch: expected=${expected_head} actual=${runtime_head} source=${runtime_head_source}"
  fi

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

    check_path_value "binary_path" "${exe_path}" "binary path outside canonical shared lab root" || true
    check_path_value "workdir_path" "${workdir}" "workdir/config points outside canonical shared lab root" || true

    path_summary=""
    path_failures=0
    path_warnings=0
    while IFS= read -r line; do
      local_name="${line%%=*}"
      local_value="${line#*=}"
      case "${local_name}" in
        TARS_DIR|TARS_HOME|TARS_*CONFIG_PATH|TARS_*STATE_PATH|TARS_*SQLITE_PATH|TARS_*DIST_DIR|TARS_*DATA_DIR|TARS_*SPOOL_DIR|TARS_SERVER_LISTEN)
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
          if ! path_is_allowed "${local_value}"; then
            path_failures=1
          elif path_uses_override "${local_value}"; then
            path_warnings=1
          fi
          ;;
      esac
    done <<< "${env_dump}"

    if [[ -z "${path_summary}" ]]; then
      fail_check "config_paths" "no TARS_* path variables found in process environment"
    elif [[ "${path_failures}" -ne 0 ]]; then
      fail_check "config_paths" "workdir/config points outside canonical shared lab root: expected=${CANONICAL_BASE_DIR} actual=${path_summary}"
    elif [[ "${path_warnings}" -ne 0 ]]; then
      warn_check "config_paths" "owner-accepted non-canonical config paths: expected=${CANONICAL_BASE_DIR} accepted=${OVERRIDE_ACCEPTED_PATH} actual=${path_summary}"
    else
      pass_check "config_paths" "${path_summary}"
    fi

    env_file="$(resolve_env_file "${workdir}" 2>/dev/null || true)"
    if [[ -n "${env_file}" ]]; then
      check_path_value "shared_env_file" "${env_file}" "shared env file outside canonical shared lab root" || true
      shared_env_tars_dir="$(read_env_value "${env_file}" "TARS_DIR")"
      if [[ -z "${shared_env_tars_dir}" ]]; then
        fail_check "shared_env_tars_dir" "TARS_DIR missing from shared-test.env: ${env_file}"
      else
        check_path_value "shared_env_tars_dir" "${shared_env_tars_dir}" "TARS_DIR from shared-test.env points outside canonical shared lab root" || true
      fi
    else
      fail_check "shared_env_file" "unable to locate shared-test.env under canonical shared lab root"
    fi

    check_managed_service_config

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
    if [[ "${WARN_COUNT}" -eq 0 ]]; then
      emit_meta "overall" "PASS"
    else
      emit_meta "overall" "PASS warnings=${WARN_COUNT}"
    fi
    exit 0
  fi

  emit_meta "overall" "FAIL blockers=${FAIL_COUNT} warnings=${WARN_COUNT}"
  local index=1
  for blocker in "${BLOCKERS[@]}"; do
    emit_meta "blocker.${index}" "${blocker}"
    index=$((index + 1))
  done
  exit 1
}

main "$@"
