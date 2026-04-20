#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() {
  printf 'multiarch-regression: %s\n' "$*" >&2
  exit 1
}

grep -q '^build-linux-amd64:' "${ROOT_DIR}/Makefile" || fail "missing build-linux-amd64 target"
grep -q '^build-linux-arm64:' "${ROOT_DIR}/Makefile" || fail "missing build-linux-arm64 target"
grep -q 'build:' "${ROOT_DIR}/deploy/docker/docker-compose.yml" || fail "docker compose should build a multi-arch image"
! grep -q 'tars-linux-amd64' "${ROOT_DIR}/deploy/docker/docker-compose.yml" || fail "docker compose should not hardcode amd64 binary paths"
grep -q 'TARGETARCH' "${ROOT_DIR}/deploy/docker/Dockerfile" || fail "docker image should consume TARGETARCH"
grep -q 'TARS_TARGET_ARCH' "${ROOT_DIR}/scripts/deploy_team_shared.sh" || fail "shared deploy script should support explicit target arch"
grep -q 'detect_remote_arch' "${ROOT_DIR}/scripts/deploy_team_shared.sh" || fail "shared deploy script should auto-detect remote arch"

printf 'multiarch-regression=passed\n'
