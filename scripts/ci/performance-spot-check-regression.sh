#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() {
  printf 'performance-spot-check-regression: %s\n' "$*" >&2
  exit 1
}

test_cpu_threshold_gate() {
  (
    source "${ROOT_DIR}/scripts/ci/performance-spot-check.sh"

    check_cpu_threshold "12.5" "30" >/dev/null || fail "expected CPU below threshold to pass"

    if check_cpu_threshold "45" "30" >/dev/null 2>&1; then
      fail "expected CPU above threshold to fail"
    fi
  )
}

test_script_declares_cpu_threshold_default() {
  local script_path="${ROOT_DIR}/scripts/ci/performance-spot-check.sh"
  grep -q 'CPU_THRESHOLD=' "${script_path}" || fail "expected performance spot-check to define a CPU threshold"
}

test_cpu_threshold_gate
test_script_declares_cpu_threshold_default

echo "performance-spot-check-regression=passed"
