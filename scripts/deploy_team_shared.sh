#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"
source "${ROOT_DIR}/scripts/lib/shared_remote_service.sh"
REMOTE_HOST="${TARS_REMOTE_HOST:-192.168.3.100}"
REMOTE_USER="${TARS_REMOTE_USER:-}"
REMOTE=""
REMOTE_HOME=""
REMOTE_BASE_DIR="${TARS_REMOTE_BASE_DIR:-}"
REMOTE_SHARED_DIR="${TARS_REMOTE_SHARED_DIR:-}"
REMOTE_WEB_DIST="${TARS_REMOTE_WEB_DIST:-}"
REMOTE_DATA_DIR="${TARS_REMOTE_DATA_DIR:-}"
REMOTE_EXECUTION_OUTPUT_DIR="${TARS_REMOTE_EXECUTION_OUTPUT_DIR:-}"
REMOTE_DEX_CONFIG="${TARS_REMOTE_DEX_CONFIG:-}"
DEX_IMAGE="${TARS_DEX_IMAGE:-ghcr.io/dexidp/dex:v2.44.0}"
DEX_CONTAINER_NAME="${TARS_DEX_CONTAINER_NAME:-tars-shared-dex}"
SKIP_BUILD="${TARS_DEPLOY_SKIP_BUILD:-0}"
SKIP_WEB="${TARS_DEPLOY_SKIP_WEB:-0}"
SKIP_VALIDATE="${TARS_DEPLOY_SKIP_VALIDATE:-0}"
SKIP_RESTART="${TARS_DEPLOY_SKIP_RESTART:-0}"
TARGET_ARCH="${TARS_TARGET_ARCH:-}"
REMOTE_BINARY=""
REMOTE_BINARY_BACKUP=""
LOCAL_BINARY=""
OPS_API_TOKEN=""

log() {
  printf '[deploy_team_shared] %s\n' "$*"
}

require_remote_user() {
  if [[ -z "${REMOTE_USER}" ]]; then
    log "set TARS_REMOTE_USER explicitly; this script no longer defaults to root"
    return 1
  fi
  REMOTE="${REMOTE_USER}@${REMOTE_HOST}"
}

require_ops_token() {
  OPS_API_TOKEN="$(shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}" 2>/dev/null || true)"
  if [[ -z "${OPS_API_TOKEN}" ]]; then
    export TARS_REMOTE_HOST="${REMOTE_HOST}"
    export TARS_REMOTE_USER="${REMOTE_USER}"
    export TARS_REMOTE_BASE_DIR="${REMOTE_BASE_DIR}"
    export TARS_REMOTE_SHARED_DIR="${REMOTE_SHARED_DIR}"
    OPS_API_TOKEN="$(shared_ops_token_resolve 2>/dev/null || true)"
  fi
  if [[ -z "${OPS_API_TOKEN}" ]]; then
    log "set TARS_OPS_API_TOKEN explicitly before config sync or live validation, or keep a shared token in ${REMOTE_SHARED_DIR}/shared-test.env"
    return 1
  fi
}

resolve_sync_ops_token() {
  shared_ops_token_local_override 2>/dev/null || true
}

detect_remote_home() {
  ssh "${REMOTE}" 'printf %s "$HOME"' 2>/dev/null | tr -d '\r\n'
}

resolve_remote_paths() {
  if [[ -z "${REMOTE_BASE_DIR}" ]]; then
    REMOTE_HOME="$(detect_remote_home)" || {
      log "failed to detect remote home from ${REMOTE}; set TARS_REMOTE_BASE_DIR explicitly"
      return 1
    }
    if [[ -z "${REMOTE_HOME}" ]]; then
      log "remote home is empty for ${REMOTE}; set TARS_REMOTE_BASE_DIR explicitly"
      return 1
    fi
    REMOTE_BASE_DIR="${REMOTE_HOME}/tars-dev"
  fi

  REMOTE_SHARED_DIR="${REMOTE_SHARED_DIR:-${REMOTE_BASE_DIR}/team-shared}"
  REMOTE_WEB_DIST="${REMOTE_WEB_DIST:-${REMOTE_BASE_DIR}/web-dist}"
  REMOTE_DATA_DIR="${REMOTE_DATA_DIR:-${REMOTE_BASE_DIR}/data}"
  REMOTE_EXECUTION_OUTPUT_DIR="${REMOTE_EXECUTION_OUTPUT_DIR:-${REMOTE_BASE_DIR}/execution-output}"
  REMOTE_DEX_CONFIG="${REMOTE_DEX_CONFIG:-${REMOTE_SHARED_DIR}/dex.config.yaml}"
}

ensure_non_root_paths() {
  if [[ "${REMOTE_USER}" == "root" ]]; then
    return 0
  fi

  case "${REMOTE_SHARED_DIR}" in
    /root/*)
      log "set TARS_REMOTE_SHARED_DIR explicitly for non-root TARS_REMOTE_USER=${REMOTE_USER}"
      return 1
      ;;
  esac

  case "${REMOTE_WEB_DIST}" in
    /root/*)
      log "set TARS_REMOTE_WEB_DIST explicitly for non-root TARS_REMOTE_USER=${REMOTE_USER}"
      return 1
      ;;
  esac

  case "${REMOTE_BINARY}" in
    /root/*)
      log "set TARS_REMOTE_BINARY explicitly for non-root TARS_REMOTE_USER=${REMOTE_USER}"
      return 1
      ;;
  esac
}

normalize_arch() {
  case "$1" in
    x86_64|amd64)
      printf 'amd64\n'
      ;;
    aarch64|arm64)
      printf 'arm64\n'
      ;;
    *)
      return 1
      ;;
  esac
}

detect_remote_arch() {
  local raw_arch
  raw_arch="$(ssh "${REMOTE}" "uname -m" 2>/dev/null | tr -d '\r\n')" || return 1
  normalize_arch "${raw_arch}"
}

resolve_target_arch() {
  local resolved_arch
  if [[ -n "${TARGET_ARCH}" ]]; then
    resolved_arch="$(normalize_arch "${TARGET_ARCH}")" || {
      log "unsupported TARS_TARGET_ARCH=${TARGET_ARCH}; use amd64 or arm64"
      return 1
    }
    TARGET_ARCH="${resolved_arch}"
    return 0
  fi

  resolved_arch="$(detect_remote_arch)" || {
    log "failed to auto-detect remote arch from ${REMOTE}; set TARS_TARGET_ARCH explicitly"
    return 1
  }
  TARGET_ARCH="${resolved_arch}"
}

configure_arch_paths() {
  require_remote_user
  resolve_target_arch
  resolve_remote_paths
  REMOTE_BINARY="${TARS_REMOTE_BINARY:-${REMOTE_BASE_DIR}/bin/tars-linux-${TARGET_ARCH}-dev}"
  REMOTE_BINARY_BACKUP="${TARS_REMOTE_BINARY_BACKUP:-${REMOTE_BINARY}.prev}"
  LOCAL_BINARY="${ROOT_DIR}/bin/tars-linux-${TARGET_ARCH}"
  ensure_non_root_paths
  log "using remote base dir ${REMOTE_BASE_DIR}"
  log "using target arch ${TARGET_ARCH}"
}

build_binary() {
  log "building linux/${TARGET_ARCH} binary"
  mkdir -p "${ROOT_DIR}/bin"
  GOOS=linux GOARCH="${TARGET_ARCH}" CGO_ENABLED=0 go build -o "${LOCAL_BINARY}" "${ROOT_DIR}/cmd/tars"
}

build_web() {
  log "building web dist"
  (cd "${ROOT_DIR}/web" && npm run build)
}

prepare_remote_dirs() {
  log "preparing remote directories"
  local remote_binary_dir
  remote_binary_dir="$(dirname "${REMOTE_BINARY}")"
  ssh "${REMOTE}" "mkdir -p '${REMOTE_SHARED_DIR}' '${REMOTE_SHARED_DIR}/marketplace' '${REMOTE_SHARED_DIR}/fixtures' '${REMOTE_SHARED_DIR}/lib' '${REMOTE_BASE_DIR}/scripts/lib' '${REMOTE_WEB_DIST}' '${REMOTE_DATA_DIR}' '${REMOTE_EXECUTION_OUTPUT_DIR}' '${remote_binary_dir}'"
}

sync_shared_files() {
  log "syncing and patching shared config and marketplace files"
  # Use a temporary directory for patching
  local tmp_sync_dir
  local sync_ops_token=""
  local shared_env_path="${ROOT_DIR}/deploy/team-shared/shared-test.env"
  local shared_env_source="${shared_env_path}"
  local remote_env_snapshot=""
  tmp_sync_dir=$(mktemp -d)
  if [[ ! -f "${shared_env_source}" ]]; then
    shared_env_source="${shared_env_path}.example"
  fi
  if [[ ! -f "${shared_env_source}" ]]; then
    log "missing shared-test env template; expected ${shared_env_path} or ${shared_env_path}.example"
    return 1
  fi
  cp "${shared_env_source}" "${tmp_sync_dir}/shared-test.env"
  cp "${ROOT_DIR}/deploy/team-shared/"*.yaml "${ROOT_DIR}/deploy/team-shared/README.md" "${tmp_sync_dir}/"
  remote_env_snapshot="${tmp_sync_dir}/.remote-shared-test.env"
  ssh "${REMOTE}" "cat '${REMOTE_SHARED_DIR}/shared-test.env' 2>/dev/null || true" > "${remote_env_snapshot}" || true

  sync_ops_token="$(resolve_sync_ops_token)"
  log "patching shared host/path values -> ${REMOTE_HOST}"
  if [[ -n "${sync_ops_token}" ]]; then
    log "injecting shared ops token from local env override"
  else
    log "shared ops token not explicitly overridden; merging template shared-test.env with remote canonical values"
  fi
  python3 - <<'PY' "${tmp_sync_dir}" "${remote_env_snapshot}" "${REMOTE_HOST}" "${REMOTE_SHARED_DIR}" "${REMOTE_WEB_DIST}" "${REMOTE_DATA_DIR}" "${REMOTE_EXECUTION_OUTPUT_DIR}" "${REMOTE_BINARY}" "${sync_ops_token}"
from pathlib import Path
import sys

root = Path(sys.argv[1])
remote_env_snapshot = Path(sys.argv[2])
remote_host = sys.argv[3]
remote_shared_dir = sys.argv[4]
remote_web_dist = sys.argv[5]
remote_data_dir = sys.argv[6]
remote_execution_output_dir = sys.argv[7]
remote_binary = sys.argv[8]
sync_ops_token = sys.argv[9]

replacements = [
    ("192.168.3.106", remote_host),
    ("192.168.3.100", remote_host),
    ("REPLACE_WITH_REMOTE_SHARED_DIR", remote_shared_dir),
    ("REPLACE_WITH_REMOTE_WEB_DIST", remote_web_dist),
    ("REPLACE_WITH_REMOTE_DATA_DIR", remote_data_dir),
    ("REPLACE_WITH_REMOTE_EXECUTION_OUTPUT_DIR", remote_execution_output_dir),
    ("REPLACE_WITH_REMOTE_BINARY", remote_binary),
]

if sync_ops_token:
    replacements.extend([
        ("REPLACE_WITH_OPS_API_TOKEN", sync_ops_token),
        ("REPLACE_WITH_BREAKGLASS_TOKEN", sync_ops_token),
    ])

def is_placeholder(value: str) -> bool:
    trimmed = value.strip()
    return trimmed == "" or trimmed in {"placeholder", "PLACEHOLDER"} or trimmed.startswith("REPLACE_WITH_")

def parse_env(path: Path):
    values = {}
    order = []
    if not path.exists():
        return values, order
    for raw_line in path.read_text().splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in raw_line:
            continue
        key, value = raw_line.split("=", 1)
        key = key.strip()
        if not key:
            continue
        if key not in values:
            order.append(key)
        values[key] = value
    return values, order

for path in root.rglob("*"):
    if not path.is_file() or path == remote_env_snapshot:
        continue
    content = path.read_text()
    patched = content
    for old, new in replacements:
        patched = patched.replace(old, new)
    if patched != content:
        path.write_text(patched)

env_path = root / "shared-test.env"
remote_env, remote_order = parse_env(remote_env_snapshot)

rendered_lines = []
seen_keys = set()
for raw_line in env_path.read_text().splitlines():
    stripped = raw_line.strip()
    if not stripped or stripped.startswith("#") or "=" not in raw_line:
        rendered_lines.append(raw_line)
        continue
    key, value = raw_line.split("=", 1)
    key = key.strip()
    chosen = value
    remote_value = remote_env.get(key, "")
    if is_placeholder(value) and remote_value and not is_placeholder(remote_value):
        chosen = remote_value
    rendered_lines.append(f"{key}={chosen}")
    seen_keys.add(key)

extra_remote_keys = [key for key in remote_order if key not in seen_keys and remote_env.get(key, "").strip()]
if extra_remote_keys:
    if rendered_lines and rendered_lines[-1] != "":
        rendered_lines.append("")
    rendered_lines.append("# Preserved remote-only overrides")
    for key in extra_remote_keys:
        rendered_lines.append(f"{key}={remote_env[key]}")

env_path.write_text("\n".join(rendered_lines) + "\n")
PY

  scp "${tmp_sync_dir}/shared-test.env" "${tmp_sync_dir}/"*.yaml "${tmp_sync_dir}/README.md" "${REMOTE}:${REMOTE_SHARED_DIR}/"
  scp -r "${ROOT_DIR}/deploy/team-shared/marketplace/." "${REMOTE}:${REMOTE_SHARED_DIR}/marketplace/"
  scp "${ROOT_DIR}/scripts/lib/validate_tool_plan_smoke.py" "${REMOTE}:${REMOTE_SHARED_DIR}/lib/"
  scp "${ROOT_DIR}/scripts/lib/shared_ops_token.sh" "${REMOTE}:${REMOTE_BASE_DIR}/scripts/lib/"
  scp "${ROOT_DIR}/scripts/seed_team_shared_fixtures.sh" "${ROOT_DIR}/scripts/validate_tool_plan_live.sh" \
    "${ROOT_DIR}/scripts/run_golden_scenario_1.sh" "${ROOT_DIR}/scripts/run_golden_scenario_2.sh" \
    "${REMOTE}:${REMOTE_SHARED_DIR}/"
  
  rm -rf "${tmp_sync_dir}"
}

sync_binary() {
  log "syncing binary"
  scp "${LOCAL_BINARY}" "${REMOTE}:${REMOTE_BINARY}.new"
  ssh "${REMOTE}" "if [ -f '${REMOTE_BINARY}' ]; then cp '${REMOTE_BINARY}' '${REMOTE_BINARY_BACKUP}'; fi && mv '${REMOTE_BINARY}.new' '${REMOTE_BINARY}' && chmod +x '${REMOTE_BINARY}'"
}

sync_web_dist() {
  log "syncing web dist"
  scp -r "${ROOT_DIR}/web/dist/." "${REMOTE}:${REMOTE_WEB_DIST}/"
}

seed_fixtures() {
  log "seeding shared fixtures"
  ssh "${REMOTE}" "chmod +x '${REMOTE_SHARED_DIR}/seed_team_shared_fixtures.sh' && '${REMOTE_SHARED_DIR}/seed_team_shared_fixtures.sh' '${REMOTE_SHARED_DIR}/fixtures'"
  ssh "${REMOTE}" "pkill -f '[/]usr/bin/python3 -m http.server 8880' || true"
  ssh "${REMOTE}" "bash -lc 'nohup python3 -m http.server 8880 --directory "${REMOTE_SHARED_DIR}/fixtures/observability-http" >\"${REMOTE_SHARED_DIR}/tars-observability-fixture.log\" 2>&1 </dev/null &'"
}

restart_remote() {
  log "restarting remote service"
  shared_remote_service_restart \
    "${REMOTE}" \
    "${REMOTE_SHARED_DIR}" \
    "${REMOTE_BINARY}" \
    "${REMOTE_SHARED_DIR}/tars-dev.log"
}

wait_remote_ready() {
  log "waiting for remote readiness"
  local attempt
  for attempt in $(seq 1 30); do
    if ssh "${REMOTE}" "curl -fsS http://127.0.0.1:8081/healthz >/dev/null"; then
      return 0
    fi
    sleep 1
  done
  log "remote did not become ready in time"
  return 1
}

sync_runtime_connectors() {
  log "syncing runtime connectors config"
  require_ops_token
  local patched_content payload
  patched_content="$(python3 - <<'PY' "${ROOT_DIR}/deploy/team-shared/connectors.shared.yaml" "${REMOTE_HOST}"
from pathlib import Path
import sys

content = Path(sys.argv[1]).read_text()
print(content.replace('192.168.3.106', sys.argv[2]).replace('192.168.3.100', sys.argv[2]), end='')
PY
)"
  payload="$(printf '%s' "${patched_content}" | python3 -c 'import json,sys; print(json.dumps({"content": sys.stdin.read(), "operator_reason": "sync team-shared connectors baseline after deploy"}))')"
  curl -fsS -H "Authorization: Bearer ${OPS_API_TOKEN}" -H 'Content-Type: application/json' \
    -X PUT "http://${REMOTE_HOST}:8081/api/v1/config/connectors" \
    -d "${payload}" >/dev/null
}

validate_remote() {
  log "running readiness and live validation"
  require_ops_token
  TARS_REMOTE_HOST="${REMOTE_HOST}" \
  TARS_REMOTE_USER="${REMOTE_USER}" \
  TARS_OPS_API_TOKEN="${OPS_API_TOKEN}" \
  TARS_OPS_BASE_URL="http://${REMOTE_HOST}:8081" \
  bash "${ROOT_DIR}/scripts/ci/smoke-remote.sh"

  TARS_OPS_API_TOKEN="${OPS_API_TOKEN}" \
  TARS_OPS_BASE_URL="http://${REMOTE_HOST}:8081" \
  bash "${ROOT_DIR}/scripts/ci/live-validate.sh"
}

run_golden_scenarios() {
  if [[ "${TARS_SKIP_GOLDEN_SCENARIOS:-0}" == "1" ]]; then
    log "golden scenarios skipped (TARS_SKIP_GOLDEN_SCENARIOS=1)"
    return
  fi
  require_ops_token
  log "running golden scenario 2: scheduled inspection → inbox"
  TARS_OPS_API_TOKEN="${OPS_API_TOKEN}" \
  TARS_OPS_BASE_URL="http://${REMOTE_HOST}:8081" \
  bash "${ROOT_DIR}/scripts/run_golden_scenario_2.sh"

  log "running golden scenario 1: web chat + SSH → inbox"
  TARS_OPS_API_TOKEN="${OPS_API_TOKEN}" \
  TARS_OPS_BASE_URL="http://${REMOTE_HOST}:8081" \
  TARS_CHAT_HOST="${REMOTE_HOST}" \
  bash "${ROOT_DIR}/scripts/run_golden_scenario_1.sh"
}

main() {
  configure_arch_paths
  if [[ "${SKIP_BUILD}" != "1" ]]; then
    build_binary
  fi
  if [[ "${SKIP_WEB}" != "1" ]]; then
    build_web
  fi
  prepare_remote_dirs
  sync_shared_files
  sync_binary
  if [[ "${SKIP_WEB}" != "1" ]]; then
    sync_web_dist
  fi
  seed_fixtures
  if [[ "${SKIP_RESTART}" != "1" ]]; then
    restart_remote
    wait_remote_ready
  fi
  sync_runtime_connectors
  if [[ "${SKIP_VALIDATE}" != "1" ]]; then
    validate_remote
    run_golden_scenarios
  fi
  log "done"
}

main "$@"
