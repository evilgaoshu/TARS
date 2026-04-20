#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STARTED_AT=${SECONDS}

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

mkdir -p "${ROOT_DIR}/bin"

echo "== TARS full-check =="
echo "scope=L1 standard local regression + multi-arch deployment checks"
echo

run_step "MVP baseline" bash "${ROOT_DIR}/scripts/check_mvp.sh"
run_step "Multi-arch deployment regression" bash "${ROOT_DIR}/scripts/ci/multiarch-regression.sh"
run_step \
  "Linux amd64 cross build" \
  env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "${ROOT_DIR}/bin/tars-linux-amd64" "${ROOT_DIR}/cmd/tars"
run_step \
  "Linux arm64 cross build" \
  env GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o "${ROOT_DIR}/bin/tars-linux-arm64" "${ROOT_DIR}/cmd/tars"

echo "artifacts=${ROOT_DIR}/bin/tars-linux-amd64,${ROOT_DIR}/bin/tars-linux-arm64"
echo "full-check=passed total=$((SECONDS - STARTED_AT))s"
