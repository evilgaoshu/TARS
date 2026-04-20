#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STARTED_AT=${SECONDS}

cd "${ROOT_DIR}/web"

echo "== TARS static-demo-build =="
echo "scope=frontend static demo artifact"
echo

npm ci
npm run build

echo
echo "static-demo-build=passed total=$((SECONDS - STARTED_AT))s"
