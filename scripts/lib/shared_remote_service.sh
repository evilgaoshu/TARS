#!/usr/bin/env bash

shared_remote_service_restart() {
  local remote="$1"
  local shared_dir="$2"
  local binary_path="$3"
  local log_path="$4"
  local env_file="${5:-${shared_dir}/shared-test.env}"
  local pid_file="${6:-${shared_dir}/tars-dev.pid}"
  local service_name="${7:-tars-shared-lab.service}"

  ssh "${remote}" bash -s -- "${shared_dir}" "${binary_path}" "${log_path}" "${env_file}" "${pid_file}" "${service_name}" <<'EOF'
set -euo pipefail

shared_dir="$1"
binary_path="$2"
log_path="$3"
env_file="$4"
pid_file="$5"
service_name="$6"
unit_path="/etc/systemd/system/${service_name}"

mkdir -p "${shared_dir}"
mkdir -p "$(dirname "${unit_path}")"

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

systemctl stop "${service_name}" >/dev/null 2>&1 || true
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

cat >"${unit_path}" <<UNIT
[Unit]
Description=TARS shared lab runtime
After=network.target

[Service]
Type=simple
WorkingDirectory=${shared_dir}
EnvironmentFile=${env_file}
ExecStart=${binary_path}
Restart=always
RestartSec=2
StandardOutput=append:${log_path}
StandardError=append:${log_path}

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now "${service_name}"
systemctl restart "${service_name}"

pid="$(systemctl show -p MainPID --value "${service_name}")"
if [[ -z "${pid}" || "${pid}" == "0" ]]; then
  echo "managed service has no active pid; see ${log_path}" >&2
  systemctl status "${service_name}" --no-pager >&2 || true
  exit 1
fi

printf '%s\n' "${pid}" >"${pid_file}"

sleep 1
if ! kill -0 "${pid}" 2>/dev/null; then
  echo "process exited immediately; see ${log_path}" >&2
  systemctl status "${service_name}" --no-pager >&2 || true
  exit 1
fi
EOF
}
