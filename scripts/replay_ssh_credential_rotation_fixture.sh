#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)

REMOTE_HOST="${TARS_REMOTE_HOST:-192.168.3.100}"
REMOTE_USER="${TARS_REMOTE_USER:-root}"
REMOTE_BASE_DIR="${TARS_REMOTE_BASE_DIR:-/root/tars-dev}"
BASE_URL="${TARS_OPS_BASE_URL:-http://${REMOTE_HOST}:8081}"

FIXTURE_PROVIDER_ID="${TARS_EVI17_PROVIDER_ID:-evi17-local-token}"
FIXTURE_PROVIDER_SECRET="${TARS_EVI17_PROVIDER_SECRET:-}"
FIXTURE_CREDENTIAL_ID="${TARS_EVI17_CREDENTIAL_ID:-evi17-ssh-rotation}"
FIXTURE_CONNECTOR_ID="${TARS_EVI17_CONNECTOR_ID:-ssh-main}"
TARGET_HOST="${TARS_EVI17_TARGET_HOST:-192.168.3.9}"
TARGET_USER="${TARS_EVI17_TARGET_USER:-root}"
FIXTURE_HOST_SCOPE="${TARS_EVI17_HOST_SCOPE:-192.168.3.9}"
FIXTURE_AUTH_TYPE="${TARS_EVI17_AUTH_TYPE:-password}"
FIXTURE_PASSWORD="${TARS_EVI17_PASSWORD:-}"
FIXTURE_PRIVATE_KEY_PATH="${TARS_EVI17_PRIVATE_KEY_PATH:-}"
FIXTURE_PASSPHRASE="${TARS_EVI17_PASSPHRASE:-}"
FIXTURE_COMMAND="${TARS_EVI17_COMMAND:-whoami}"
PRESERVE_FIXTURE="${TARS_EVI17_PRESERVE_FIXTURE:-0}"

TMP_DIR=$(mktemp -d)
AUTH_CONFIG_JSON="${TMP_DIR}/auth-config.json"
CONNECTORS_JSON="${TMP_DIR}/connectors.json"
SUMMARY_JSON="${TARS_EVI17_SUMMARY_PATH:-${TMP_DIR}/summary.json}"
AUTH_MUTATED=0
CONNECTORS_MUTATED=0
CREDENTIAL_MUTATED=0

cleanup_remote() {
  [[ "${PRESERVE_FIXTURE}" == "1" ]] && return 0

  if [[ "${CREDENTIAL_MUTATED}" == "1" ]]; then
    api DELETE "/api/v1/ssh-credentials/${FIXTURE_CREDENTIAL_ID}" '{"operator_reason":"evi-17 fixture cleanup"}' "${OPS_TOKEN}" "${TMP_DIR}/credential-delete.json" >/dev/null 2>&1 || true
  fi
  if [[ "${CONNECTORS_MUTATED}" == "1" && -f "${CONNECTORS_JSON}" ]]; then
    api PUT /api/v1/config/connectors "$(python3 - <<'PY' "${CONNECTORS_JSON}"
import json
import sys
with open(sys.argv[1]) as fh:
    data = json.load(fh)
print(json.dumps({
    'content': data['content'],
    'operator_reason': 'evi-17 restore connectors config after fixture cleanup',
}))
PY
)" "${OPS_TOKEN}" "${TMP_DIR}/connectors-restore.json" >/dev/null 2>&1 || true
  fi
  if [[ "${AUTH_MUTATED}" == "1" && -f "${AUTH_CONFIG_JSON}" ]]; then
    api PUT /api/v1/config/auth "$(python3 - <<'PY' "${AUTH_CONFIG_JSON}"
import json
import sys
with open(sys.argv[1]) as fh:
    data = json.load(fh)
print(json.dumps({
    'content': data['content'],
    'operator_reason': 'evi-17 restore auth config after fixture cleanup',
}))
PY
)" "${OPS_TOKEN}" "${TMP_DIR}/auth-restore.json" >/dev/null 2>&1 || true
  fi
}

cleanup() {
  local exit_code=$?
  cleanup_remote || true
  if [[ "${SUMMARY_JSON}" != ${TMP_DIR}/* ]]; then
    mkdir -p "$(dirname "${SUMMARY_JSON}")"
  fi
  rm -rf "${TMP_DIR}"
  exit "${exit_code}"
}
trap cleanup EXIT

log() {
  printf '%s\n' "$*" >&2
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    log "missing required command: $1"
    exit 1
  }
}

require_cmd curl
require_cmd python3
require_cmd ssh

source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"

export TARS_REMOTE_HOST="${REMOTE_HOST}"
export TARS_REMOTE_USER="${REMOTE_USER}"
export TARS_REMOTE_BASE_DIR="${REMOTE_BASE_DIR}"

OPS_TOKEN="${TARS_OPS_API_TOKEN:-}"
if [[ -z "${OPS_TOKEN}" ]]; then
  OPS_TOKEN="$(shared_ops_token_resolve)"
fi

if [[ -z "${OPS_TOKEN}" ]]; then
  log "failed to resolve ops token"
  exit 1
fi

if [[ -z "${FIXTURE_PROVIDER_SECRET}" ]]; then
  FIXTURE_PROVIDER_SECRET="$(python3 - <<'PY'
import secrets
print('evi17-' + secrets.token_urlsafe(24))
PY
)"
fi

if [[ "${FIXTURE_AUTH_TYPE}" == "password" && -z "${FIXTURE_PASSWORD}" ]]; then
  log "TARS_EVI17_PASSWORD is required for password auth fixtures"
  exit 1
fi

if [[ "${FIXTURE_AUTH_TYPE}" == "private_key" && -z "${FIXTURE_PRIVATE_KEY_PATH}" ]]; then
  log "TARS_EVI17_PRIVATE_KEY_PATH is required for private_key auth fixtures"
  exit 1
fi

api() {
  local method=$1
  local path=$2
  local data=${3:-}
  local auth=${4:-$OPS_TOKEN}
  local body_file=${5:-"${TMP_DIR}/body.json"}
  local status

  if [[ -n "${data}" ]]; then
    status=$(curl -sS -o "${body_file}" -w '%{http_code}' \
      -H "Authorization: Bearer ${auth}" \
      -H 'Content-Type: application/json' \
      -X "${method}" "${BASE_URL}${path}" \
      -d "${data}")
  else
    status=$(curl -sS -o "${body_file}" -w '%{http_code}' \
      -H "Authorization: Bearer ${auth}" \
      -X "${method}" "${BASE_URL}${path}")
  fi

  printf '%s\n' "${status}"
}

snapshot_runtime() {
  local pid
  pid=$(ssh -o BatchMode=yes "${REMOTE_USER}@${REMOTE_HOST}" "pgrep -o -f tars-linux-amd64-dev")
  ssh -o BatchMode=yes "${REMOTE_USER}@${REMOTE_HOST}" "python3 - <<'PY' '${pid}'
import json
import os
import sys

pid = sys.argv[1]
env = {}
with open(f'/proc/{pid}/environ', 'rb') as fh:
    for chunk in fh.read().split(b'\0'):
        if not chunk or b'=' not in chunk:
            continue
        key, value = chunk.split(b'=', 1)
        key = key.decode()
        if key.startswith('TARS_') or key == 'PWD':
            env[key] = value.decode(errors='replace')
print(json.dumps({
    'pid': pid,
    'cwd': os.readlink(f'/proc/{pid}/cwd'),
    'env': env,
}, ensure_ascii=True))
PY"
}

status_name() {
  case "$1" in
    200|201) printf 'PASS' ;;
    *) printf 'FAIL' ;;
  esac
}

log "capturing live auth config and connector config"
api GET /api/v1/config/auth '' "${OPS_TOKEN}" "${AUTH_CONFIG_JSON}" >/dev/null
api GET /api/v1/config/connectors '' "${OPS_TOKEN}" "${CONNECTORS_JSON}" >/dev/null

log "creating temporary non-breakglass auth provider"
provider_payload=$(python3 - <<'PY' "${FIXTURE_PROVIDER_ID}" "${FIXTURE_PROVIDER_SECRET}"
import json
import sys

provider_id = sys.argv[1]
secret = sys.argv[2]
print(json.dumps({
    'provider': {
        'id': provider_id,
        'type': 'local_token',
        'name': 'EVI17 Local Token',
        'enabled': True,
        'client_secret': secret,
        'default_roles': ['platform_admin'],
    },
    'operator_reason': 'evi-17 temporary session auth provider',
}))
PY
)
provider_status=$(api POST /api/v1/auth/providers "${provider_payload}" "${OPS_TOKEN}" "${TMP_DIR}/provider-create.json")
if [[ "${provider_status}" != "200" && "${provider_status}" != "201" ]]; then
  provider_status=$(api PUT "/api/v1/auth/providers/${FIXTURE_PROVIDER_ID}" "${provider_payload}" "${OPS_TOKEN}" "${TMP_DIR}/provider-create.json")
fi
if [[ "${provider_status}" == "200" || "${provider_status}" == "201" ]]; then
  AUTH_MUTATED=1
fi

log "logging in through temporary auth provider"
login_status=$(curl -sS -o "${TMP_DIR}/provider-login.json" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -X POST "${BASE_URL}/api/v1/auth/login" \
  -d "$(python3 - <<'PY' "${FIXTURE_PROVIDER_ID}" "${FIXTURE_PROVIDER_SECRET}"
import json
import sys
print(json.dumps({'provider_id': sys.argv[1], 'token': sys.argv[2]}))
PY
)")

if [[ "${login_status}" != "200" ]]; then
  log "temporary provider login failed"
  sed -n '1,120p' "${TMP_DIR}/provider-login.json" >&2
  exit 1
fi

SESSION_TOKEN=$(python3 - <<'PY' "${TMP_DIR}/provider-login.json"
import json
import sys
with open(sys.argv[1]) as fh:
    print(json.load(fh)['session_token'])
PY
)

log "capturing live runtime identity"
runtime_identity=$(snapshot_runtime)

log "updating ssh-main connector target values"
connectors_payload=$(python3 - <<'PY' "${CONNECTORS_JSON}" "${FIXTURE_CONNECTOR_ID}" "${TARGET_HOST}" "${TARGET_USER}" "${FIXTURE_CREDENTIAL_ID}"
import json
import sys

with open(sys.argv[1]) as fh:
    data = json.load(fh)
lines = data['content'].splitlines()
start = -1
for idx, line in enumerate(lines):
    if line.strip() == f'id: {sys.argv[2]}':
        start = idx
        break
if start == -1:
    raise SystemExit('connector not found in live config content')
while start > 0 and not lines[start].lstrip().startswith('- api_version:'):
    start -= 1
end = len(lines)
for idx in range(start + 1, len(lines)):
    if lines[idx].lstrip().startswith('- api_version:'):
        end = idx
        break
block = lines[start:end]
replaced = {'host': False, 'username': False, 'credential_id': False}
for idx, line in enumerate(block):
    stripped = line.lstrip()
    indent = line[:len(line) - len(stripped)]
    if stripped.startswith('host:'):
        block[idx] = f'{indent}host: {sys.argv[3]}'
        replaced['host'] = True
    elif stripped.startswith('username:'):
        block[idx] = f'{indent}username: {sys.argv[4]}'
        replaced['username'] = True
    elif stripped.startswith('credential_id:'):
        block[idx] = f'{indent}credential_id: {sys.argv[5]}'
        replaced['credential_id'] = True
for key, ok in replaced.items():
    if not ok:
        raise SystemExit(f'failed to patch {key} in connector block')
lines[start:end] = block
content = '\n'.join(lines) + '\n'
print(json.dumps({
    'content': content,
    'operator_reason': 'evi-17 ssh credential rotation fixture patch',
}))
PY
)
connectors_update_status=$(api PUT /api/v1/config/connectors "${connectors_payload}" "${OPS_TOKEN}" "${TMP_DIR}/connectors-update.json")
if [[ "${connectors_update_status}" == "200" ]]; then
  CONNECTORS_MUTATED=1
fi

log "creating or updating ssh credential fixture metadata"
if [[ "${FIXTURE_AUTH_TYPE}" == "password" ]]; then
  credential_payload=$(python3 - <<'PY' "${FIXTURE_CREDENTIAL_ID}" "${FIXTURE_CONNECTOR_ID}" "${TARGET_USER}" "${FIXTURE_HOST_SCOPE}" "${FIXTURE_PASSWORD}"
import json
import sys
print(json.dumps({
    'credential_id': sys.argv[1],
    'display_name': 'EVI17 SSH rotation fixture',
    'owner_type': 'connector',
    'owner_id': sys.argv[2],
    'connector_id': sys.argv[2],
    'username': sys.argv[3],
    'auth_type': 'password',
    'password': sys.argv[5],
    'host_scope': sys.argv[4],
    'operator_reason': 'evi-17 create fixture credential',
}))
PY
)
else
  private_key_json=$(python3 - <<'PY' "${FIXTURE_PRIVATE_KEY_PATH}"
import json
import pathlib
import sys
print(json.dumps(pathlib.Path(sys.argv[1]).read_text()))
PY
)
  passphrase_json=$(python3 - <<'PY' "${FIXTURE_PASSPHRASE}"
import json
import sys
print(json.dumps(sys.argv[1]))
PY
)
  credential_payload=$(python3 - <<'PY' "${FIXTURE_CREDENTIAL_ID}" "${FIXTURE_CONNECTOR_ID}" "${TARGET_USER}" "${FIXTURE_HOST_SCOPE}" "${private_key_json}" "${passphrase_json}"
import json
import sys
print(json.dumps({
    'credential_id': sys.argv[1],
    'display_name': 'EVI17 SSH rotation fixture',
    'owner_type': 'connector',
    'owner_id': sys.argv[2],
    'connector_id': sys.argv[2],
    'username': sys.argv[3],
    'auth_type': 'private_key',
    'private_key': json.loads(sys.argv[5]),
    'passphrase': json.loads(sys.argv[6]),
    'host_scope': sys.argv[4],
    'operator_reason': 'evi-17 create fixture credential',
}))
PY
)
fi

create_status=$(api POST /api/v1/ssh-credentials "${credential_payload}" "${OPS_TOKEN}" "${TMP_DIR}/credential-create.json")
if [[ "${create_status}" == "400" ]] && grep -q 'already exists' "${TMP_DIR}/credential-create.json"; then
  create_status=$(api PUT "/api/v1/ssh-credentials/${FIXTURE_CREDENTIAL_ID}" "${credential_payload}" "${OPS_TOKEN}" "${TMP_DIR}/credential-create.json")
fi
if [[ "${create_status}" == "200" || "${create_status}" == "201" ]]; then
  CREDENTIAL_MUTATED=1
fi

log "running pre-rotation execution"
pre_exec_status=$(api POST "/api/v1/connectors/${FIXTURE_CONNECTOR_ID}/execution/execute" "$(python3 - <<'PY' "${TARGET_HOST}" "${FIXTURE_COMMAND}"
import json
import sys
print(json.dumps({
    'target_host': sys.argv[1],
    'command': sys.argv[2],
    'operator_reason': 'evi-17 pre-rotation execution',
}))
PY
)" "${SESSION_TOKEN}" "${TMP_DIR}/pre-exec.json")

log "marking rotation required"
rotation_required_status=$(api POST "/api/v1/ssh-credentials/${FIXTURE_CREDENTIAL_ID}/rotation-required" '{"operator_reason":"evi-17 mark rotation required"}' "${OPS_TOKEN}" "${TMP_DIR}/rotation-required.json")

log "running blocked execution after rotation_required"
blocked_exec_status=$(api POST "/api/v1/connectors/${FIXTURE_CONNECTOR_ID}/execution/execute" "$(python3 - <<'PY' "${TARGET_HOST}" "${FIXTURE_COMMAND}"
import json
import sys
print(json.dumps({
    'target_host': sys.argv[1],
    'command': sys.argv[2],
    'operator_reason': 'evi-17 blocked execution after rotation',
}))
PY
)" "${SESSION_TOKEN}" "${TMP_DIR}/blocked-exec.json")

log "rotating credential material to clear rotation_required"
if [[ "${FIXTURE_AUTH_TYPE}" == "password" ]]; then
  update_payload=$(python3 - <<'PY' "${FIXTURE_PASSWORD}" "${FIXTURE_HOST_SCOPE}"
import json
import sys
print(json.dumps({
    'password': sys.argv[1],
    'host_scope': sys.argv[2],
    'operator_reason': 'evi-17 rotate fixture credential',
}))
PY
)
else
  update_payload=$(python3 - <<'PY' "${FIXTURE_PRIVATE_KEY_PATH}" "${FIXTURE_PASSPHRASE}" "${FIXTURE_HOST_SCOPE}"
import json
import pathlib
import sys
print(json.dumps({
    'private_key': pathlib.Path(sys.argv[1]).read_text(),
    'passphrase': sys.argv[2],
    'host_scope': sys.argv[3],
    'operator_reason': 'evi-17 rotate fixture credential',
}))
PY
)
fi
update_status=$(api PUT "/api/v1/ssh-credentials/${FIXTURE_CREDENTIAL_ID}" "${update_payload}" "${OPS_TOKEN}" "${TMP_DIR}/credential-update.json")

log "running post-rotation execution"
post_exec_status=$(api POST "/api/v1/connectors/${FIXTURE_CONNECTOR_ID}/execution/execute" "$(python3 - <<'PY' "${TARGET_HOST}" "${FIXTURE_COMMAND}"
import json
import sys
print(json.dumps({
    'target_host': sys.argv[1],
    'command': sys.argv[2],
    'operator_reason': 'evi-17 post-rotation execution',
}))
PY
)" "${SESSION_TOKEN}" "${TMP_DIR}/post-exec.json")

log "capturing final ssh credential state"
detail_status=$(api GET "/api/v1/ssh-credentials/${FIXTURE_CREDENTIAL_ID}" '' "${OPS_TOKEN}" "${TMP_DIR}/credential-detail.json")

python3 - <<'PY' \
  "${SUMMARY_JSON}" \
  "${runtime_identity}" \
  "${provider_status}" \
  "${login_status}" \
  "${connectors_update_status}" \
  "${create_status}" \
  "${pre_exec_status}" \
  "${rotation_required_status}" \
  "${blocked_exec_status}" \
  "${update_status}" \
  "${post_exec_status}" \
  "${detail_status}" \
  "${TMP_DIR}"
import json
import pathlib
import sys

summary_path = pathlib.Path(sys.argv[1])
runtime_identity = json.loads(sys.argv[2])
tmp_dir = pathlib.Path(sys.argv[13])

def load_json(name):
    path = tmp_dir / name
    if not path.exists():
        return None
    try:
        return json.loads(path.read_text())
    except Exception:
        return {'raw': path.read_text()}

summary = {
    'runtime_identity': runtime_identity,
    'statuses': {
        'provider_create': sys.argv[3],
        'provider_login': sys.argv[4],
        'connectors_update': sys.argv[5],
        'credential_create_or_update': sys.argv[6],
        'pre_execution': sys.argv[7],
        'rotation_required': sys.argv[8],
        'blocked_execution': sys.argv[9],
        'credential_rotate': sys.argv[10],
        'post_execution': sys.argv[11],
        'credential_detail': sys.argv[12],
    },
    'responses': {
        'credential_create': load_json('credential-create.json'),
        'pre_exec': load_json('pre-exec.json'),
        'rotation_required': load_json('rotation-required.json'),
        'blocked_exec': load_json('blocked-exec.json'),
        'credential_update': load_json('credential-update.json'),
        'post_exec': load_json('post-exec.json'),
        'credential_detail': load_json('credential-detail.json'),
    }
}
summary_path.write_text(json.dumps(summary, indent=2) + '\n')
print(summary_path)
PY

printf 'summary_json=%s\n' "${SUMMARY_JSON}"
printf 'provider_create=%s\n' "$(status_name "${provider_status}") ${provider_status}"
printf 'provider_login=%s\n' "$(status_name "${login_status}") ${login_status}"
printf 'connector_patch=%s\n' "$(status_name "${connectors_update_status}") ${connectors_update_status}"
printf 'credential_create=%s\n' "$(status_name "${create_status}") ${create_status}"
printf 'pre_execution=%s\n' "${pre_exec_status}"
printf 'rotation_required=%s\n' "$(status_name "${rotation_required_status}") ${rotation_required_status}"
printf 'blocked_execution=%s\n' "${blocked_exec_status}"
printf 'credential_rotate=%s\n' "$(status_name "${update_status}") ${update_status}"
printf 'post_execution=%s\n' "${post_exec_status}"
printf 'credential_detail=%s\n' "$(status_name "${detail_status}") ${detail_status}"
