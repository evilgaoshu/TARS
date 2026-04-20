#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"

REMOTE_HOST="${TARS_REMOTE_HOST:-192.168.3.100}"
REMOTE_USER="${TARS_REMOTE_USER:-}"
REMOTE_BASE_DIR="${TARS_REMOTE_BASE_DIR:-/data/tars-setup-lab}"
REMOTE_SHARED_DIR="${TARS_REMOTE_SHARED_DIR:-${REMOTE_BASE_DIR}/team-shared}"
OPS_BASE_URL="${TARS_OPS_BASE_URL:-http://${REMOTE_HOST}:8081}"
OPS_TOKEN="$(shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}" 2>/dev/null || true)"
SSH_TARGET="${REMOTE_USER}@${REMOTE_HOST}"
CPU_THRESHOLD="${TARS_PERF_CPU_THRESHOLD:-30}"
CPU_GRACE_UPTIME_SECONDS="${TARS_PERF_CPU_GRACE_UPTIME_SECONDS:-20}"

run_remote() {
  ssh -o BatchMode=yes -o ConnectTimeout=5 "${SSH_TARGET}" "$@"
}

check_cpu_threshold() {
  local observed="$1"
  local threshold="$2"

  python3 - "$observed" "$threshold" <<'PY'
import sys

observed = sys.argv[1].strip()
threshold = sys.argv[2].strip()
try:
    observed_value = float(observed)
    threshold_value = float(threshold)
except ValueError:
    print(f"cpu_threshold_invalid observed={observed!r} threshold={threshold!r}", file=sys.stderr)
    raise SystemExit(1)

if observed_value > threshold_value:
    print(f"cpu_threshold_exceeded observed={observed_value:.1f} threshold={threshold_value:.1f}", file=sys.stderr)
    raise SystemExit(1)
PY
}

collect_process_sample() {
  run_remote "pid=\$(pgrep -f 'tars-linux-.*-dev' | head -n 1); if [[ -z \"\${pid}\" ]]; then echo 'tars pid not found' >&2; exit 1; fi; ps -p \"\${pid}\" -o pid=,%cpu=,etime=,time=,rss=,command="
}

fetch_dashboard_health() {
  curl -fsS -H "Authorization: Bearer ${OPS_TOKEN}" "${OPS_BASE_URL}/api/v1/dashboard/health"
}

wait_for_uptime_grace() {
  if [[ -z "${CPU_GRACE_UPTIME_SECONDS}" || "${CPU_GRACE_UPTIME_SECONDS}" == "0" ]]; then
    return 0
  fi

  local dashboard_json uptime_seconds wait_seconds
  dashboard_json="$(fetch_dashboard_health)"
  uptime_seconds="$(printf '%s\n' "${dashboard_json}" | python3 -c 'import json,sys; data=json.load(sys.stdin); value=((data.get("resources") or {}).get("uptime_seconds")); print(int(value or 0))')"
  if [[ -n "${uptime_seconds}" && "${uptime_seconds}" =~ ^[0-9]+$ ]] && (( uptime_seconds < CPU_GRACE_UPTIME_SECONDS )); then
    wait_seconds=$((CPU_GRACE_UPTIME_SECONDS - uptime_seconds))
    echo "warmup_uptime_seconds=${uptime_seconds} wait_seconds=${wait_seconds}"
    sleep "${wait_seconds}"
  fi
}

main() {
  if [[ -z "${REMOTE_USER}" ]]; then
    echo "TARS_REMOTE_USER is required for performance spot-check" >&2
    exit 1
  fi

  if [[ -z "${OPS_TOKEN}" ]] && OPS_TOKEN="$(shared_ops_token_resolve 2>/dev/null || true)"; then
    :
  fi

  if [[ -z "${OPS_TOKEN}" ]]; then
    echo "TARS_OPS_API_TOKEN is required for performance spot-check" >&2
    echo "tip=可显式 export TARS_OPS_API_TOKEN，或设置 TARS_REMOTE_USER 让脚本从共享机 shared-test.env 自动解析。" >&2
    exit 1
  fi

  echo "== TARS performance spot-check =="
  echo "remote=${SSH_TARGET}"
  echo "ops_base_url=${OPS_BASE_URL}"
  echo "cpu_threshold=${CPU_THRESHOLD}"
  echo

  echo "-- process cpu --"
  wait_for_uptime_grace
  local process_sample
  process_sample="$(collect_process_sample)"
  printf '%s\n' "${process_sample}"
  local observed_cpu
  observed_cpu="$(printf '%s\n' "${process_sample}" | awk 'NR==1 {print $2}')"
  check_cpu_threshold "${observed_cpu}" "${CPU_THRESHOLD}"
  echo

  echo "-- dashboard health resources --"
  local dashboard_json
  dashboard_json="$(fetch_dashboard_health)"
  printf '%s\n' "${dashboard_json}" | python3 -c 'import json,sys; data=json.load(sys.stdin); resources=data.get("resources") or {}; summary=data.get("summary") or {}; print("goroutines=%s heap_alloc_bytes=%s uptime_seconds=%s active_alerts=%s healthy_connectors=%s" % (resources.get("goroutines"), resources.get("heap_alloc_bytes"), resources.get("uptime_seconds"), summary.get("active_alerts"), summary.get("healthy_connectors")))'
  echo

  echo "-- metrics evidence --"
  local metrics_payload
  metrics_payload="$(curl -fsS "${OPS_BASE_URL}/metrics")"
  printf '%s\n' "${metrics_payload}" | python3 -c 'import sys; required=("tars_observability_store_append_duration_seconds","tars_observability_store_file_bytes","tars_observability_store_records_total"); optional=("tars_observability_store_governance_duration_seconds",); lines=[line for line in sys.stdin.read().splitlines() if any(target in line for target in required + optional) and not line.startswith("#")]; missing=[target for target in required if not any(target in line for line in lines)]; missing_optional=[target for target in optional if not any(target in line for line in lines)]; [print(line) for line in lines[:20]]; [print("optional_missing=" + target) for target in missing_optional]; sys.exit(0 if not missing else (print("missing=" + ",".join(missing), file=sys.stderr) or 1))'
  echo

  echo "-- observability files --"
  run_remote "set -a; if [[ -f '${REMOTE_SHARED_DIR}/shared-test.env' ]]; then . '${REMOTE_SHARED_DIR}/shared-test.env'; fi; set +a; pid=\$(pgrep -f '/data/tars-setup-lab/bin/tars-linux-amd64-dev' | head -n 1); if [[ -z \"\${pid}\" ]]; then pid=\$(pgrep -f 'tars-linux-.*-dev' | head -n 1); fi; if [[ -z \"\${pid}\" ]]; then echo 'tars pid not found' >&2; exit 1; fi; process_obs_dir=\$(tr '\0' '\n' < /proc/\${pid}/environ | sed -n 's/^TARS_OBSERVABILITY_DATA_DIR=//p' | head -n 1); cwd=\$(readlink -f /proc/\${pid}/cwd); obs_dir=\${process_obs_dir:-\${TARS_OBSERVABILITY_DATA_DIR:-\${cwd}/data/observability}}; printf 'observability_dir=%s\n' \"\${obs_dir}\"; for path in \"\${obs_dir}\"/logs/*.jsonl \"\${obs_dir}\"/traces/*.jsonl \"\${obs_dir}\"/metrics/*.jsonl; do [[ -e \"\${path}\" ]] || continue; stat -c '%n %s' \"\${path}\"; done"
  echo

  echo "performance-spot-check=passed"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
