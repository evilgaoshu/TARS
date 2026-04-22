#!/usr/bin/env sh
set -eu

# Enterprise-grade MVP Check Script
# Enhanced to catch hidden warnings and enforce zero-tolerance policy for circular dependencies.

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
LOG_FILE="/tmp/tars-check-$(date +%s).log"

log_and_check() {
  step=$1
  cmd=$2
  echo "$step"
  if ! eval "$cmd" > "$LOG_FILE" 2>&1; then
    cat "$LOG_FILE"
    echo "\n❌ Step failed: $step"
    exit 1
  fi
  
  # Special scan for hidden Rollup/Vite errors that don't trigger exit code
  if echo "$step" | grep -q "web build"; then
    if grep -Ei "Circular chunk|Circular dependency" "$LOG_FILE"; then
      cat "$LOG_FILE"
      echo "\n❌ BLOCKER: Circular dependency detected in build output!"
      echo "Enterprise-grade projects require zero circular dependencies."
      exit 1
    fi
  fi
}

log_and_check "[1/7] go test" "GOCACHE=\"\${GOCACHE:-/tmp/tars-go-build}\" go test ./..."
log_and_check "[2/7] core coverage" "\"$ROOT_DIR/scripts/check_core_coverage.sh\""
log_and_check "[3/7] go build" "GOCACHE=\"\${GOCACHE:-/tmp/tars-go-build}\" go build ./..."
log_and_check "[4/7] openapi validation" "\"$ROOT_DIR/scripts/validate_openapi.rb\""

if [ -d "$ROOT_DIR/web" ]; then
  log_and_check "[5/7] web lint" "(cd \"$ROOT_DIR/web\" && npm run lint)"
  log_and_check "[6/7] web test" "(cd \"$ROOT_DIR/web\" && npm run test)"
  log_and_check "[7/7] web build" "(cd \"$ROOT_DIR/web\" && npm run build)"
else
  echo "[5/7] web lint skipped"
  echo "[6/7] web test skipped"
  echo "[7/7] web build skipped"
fi

echo "\n✅ ALL MVP checks passed (Strict Mode)"
rm -f "$LOG_FILE"
