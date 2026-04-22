#!/usr/bin/env bash

shared_remote_service_restart() {
  local remote="$1"
  local shared_dir="$2"
  local binary_path="$3"
  local log_path="$4"
  local env_file="${5:-${shared_dir}/shared-test.env}"
  local pid_file="${6:-${shared_dir}/tars-dev.pid}"

  ssh "${remote}" bash -s -- "${shared_dir}" "${binary_path}" "${log_path}" "${env_file}" "${pid_file}" <<'EOF'
set -euo pipefail

shared_dir="$1"
binary_path="$2"
log_path="$3"
env_file="$4"
pid_file="$5"

mkdir -p "${shared_dir}"

existing_pid=""
if [[ -f "${pid_file}" ]]; then
  existing_pid="$(tr -d '[:space:]' < "${pid_file}" 2>/dev/null || true)"
fi

stop_pid() {
  local pid="$1"

  [[ -n "${pid}" ]] || return 0
  if ! kill -0 "${pid}" 2>/dev/null; then
    return 0
  fi

  kill "${pid}" 2>/dev/null || true
  for _ in $(seq 1 20); do
    if ! kill -0 "${pid}" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done

  kill -9 "${pid}" 2>/dev/null || true
}

stop_pid "${existing_pid}"

while IFS= read -r pid; do
  [[ -n "${pid}" ]] || continue
  if [[ "${pid}" != "${existing_pid}" ]]; then
    stop_pid "${pid}"
  fi
done < <(pgrep -f -x -- "${binary_path}" || true)

while IFS= read -r pid; do
  [[ -n "${pid}" ]] || continue
  if [[ "${pid}" == "${existing_pid}" ]]; then
    continue
  fi
  if ! kill -0 "${pid}" 2>/dev/null; then
    continue
  fi
  cmd="$(ps -p "${pid}" -o args= 2>/dev/null | sed 's/^[[:space:]]*//' || true)"
  case "${cmd}" in
    */tars-linux-amd64-dev|*/tars-linux-arm64-dev) ;;
    *)
      continue
      ;;
  esac
  if [[ "${cmd}" == "${binary_path}" ]]; then
    continue
  fi
  stop_pid "${pid}"
done < <(pgrep -f -- "tars-linux-.*-dev" || true)

rm -f "${pid_file}"

set -a
source "${env_file}"
set +a

cd "${shared_dir}"

nohup "${binary_path}" >"${log_path}" 2>&1 </dev/null &
pid=$!
printf '%s\n' "${pid}" >"${pid_file}"

sleep 1
if ! kill -0 "${pid}" 2>/dev/null; then
  echo "process exited immediately; see ${log_path}" >&2
  exit 1
fi
EOF
}
