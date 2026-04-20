#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STARTED_AT=${SECONDS}
GOCACHE_DIR="${GOCACHE:-/tmp/tars-go-build}"
INCLUDE_WEB_LINT="${TARS_PRECHECK_INCLUDE_WEB:-0}"

cd "${ROOT_DIR}"

run_step() {
  local title="$1"
  shift
  local step_started=${SECONDS}

  echo "== ${title} =="
  "$@"
  echo "-- ok ($((SECONDS - step_started))s)"
  echo
}

run_web_lint() {
  cd "${ROOT_DIR}/web"
  npm run lint
}

echo "== TARS pre-check =="
echo "scope=L0 local quick gate"
echo

run_step "Go compile baseline" env GOCACHE="${GOCACHE_DIR}" go build ./...
run_step "OpenAPI contract validation" ruby "${ROOT_DIR}/scripts/validate_openapi.rb"

if [[ "${INCLUDE_WEB_LINT}" == "1" ]]; then
  run_step "Web lint (opt-in)" run_web_lint
else
  echo "== Web lint =="
  echo "-- skipped (set TARS_PRECHECK_INCLUDE_WEB=1 to include frontend lint)"
  echo
fi

echo "pre-check=passed total=$((SECONDS - STARTED_AT))s"
