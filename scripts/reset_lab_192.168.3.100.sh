#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/scripts/lib/shared_remote_service.sh"

REMOTE_HOST="${TARS_REMOTE_HOST:-192.168.3.100}"
REMOTE_USER="${TARS_REMOTE_USER:-root}"
REMOTE_BASE_DIR="${TARS_REMOTE_BASE_DIR:-/data/tars-setup-lab}"
REMOTE_SHARED_DIR="${TARS_REMOTE_SHARED_DIR:-${REMOTE_BASE_DIR}/team-shared}"
REMOTE_BINARY="${TARS_REMOTE_BINARY:-${REMOTE_BASE_DIR}/bin/tars-linux-amd64-dev}"
REMOTE_LOG_PATH="${REMOTE_SHARED_DIR}/tars-dev.log"
REMOTE_RESET_BACKUP_DIR="${TARS_REMOTE_RESET_BACKUP_DIR:-${REMOTE_BASE_DIR}/reset-backups}"
REMOTE="${REMOTE_USER}@${REMOTE_HOST}"

log() {
  printf '[reset_lab] %s\n' "$*"
}

backup_remote_state() {
  log "backing up remote setup/config state"
  ssh "${REMOTE}" bash -s -- "${REMOTE_SHARED_DIR}" "${REMOTE_RESET_BACKUP_DIR}" <<'EOF'
set -euo pipefail

shared_dir="$1"
backup_root="$2"
timestamp="$(date +%Y%m%d-%H%M%S)"
backup_dir="${backup_root}/${timestamp}"

mkdir -p "${backup_dir}"

for path in \
  shared-test.env \
  access.shared.yaml \
  providers.shared.yaml \
  connectors.shared.yaml \
  skills.shared.yaml \
  automations.shared.yaml \
  approvals.shared.yaml \
  authorization.shared.yaml \
  reasoning-prompts.shared.yaml \
  desensitization.shared.yaml \
  extensions.state.yaml
do
  if [[ -f "${shared_dir}/${path}" ]]; then
    cp "${shared_dir}/${path}" "${backup_dir}/${path}"
  fi
done

printf '%s\n' "${backup_dir}"
EOF
}

reset_remote_database() {
  log "clearing remote setup/runtime config tables"
  ssh "${REMOTE}" "export PGPASSWORD=tars; psql -h 127.0.0.1 -U tars -d tars -c 'TRUNCATE TABLE setup_state;' && psql -h 127.0.0.1 -U tars -d tars -c 'TRUNCATE TABLE runtime_config_documents;'"
}

reset_remote_configs() {
  log "resetting shared registry files to empty baselines"
  ssh "${REMOTE}" bash -s -- "${REMOTE_SHARED_DIR}" <<'EOF'
set -euo pipefail

shared_dir="$1"

cat >"${shared_dir}/access.shared.yaml" <<'YAML'
access:
  users: []
  groups: []
  auth_providers: []
  roles: []
  people: []
  channels: []
YAML

cat >"${shared_dir}/providers.shared.yaml" <<'YAML'
providers:
  entries: []
YAML

cat >"${shared_dir}/connectors.shared.yaml" <<'YAML'
connectors:
  entries: []
YAML

cat >"${shared_dir}/skills.shared.yaml" <<'YAML'
skills:
  entries: []
YAML

cat >"${shared_dir}/automations.shared.yaml" <<'YAML'
automations:
  entries: []
YAML

rm -f "${shared_dir}/extensions.state.yaml"
EOF
}

main() {
  log "resetting TARS shared lab at ${REMOTE_HOST}"
  backup_remote_state
  reset_remote_database
  reset_remote_configs
  log "restarting remote service via shared helper"
  shared_remote_service_restart "${REMOTE}" "${REMOTE_SHARED_DIR}" "${REMOTE_BINARY}" "${REMOTE_LOG_PATH}"
  log "done; platform should be back in first-run setup mode"
}

main "$@"
