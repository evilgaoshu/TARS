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

log_and_check "[1/6] go test" "GOCACHE=\"\${GOCACHE:-/tmp/tars-go-build}\" go test ./..."
log_and_check "[2/6] core coverage" "\"$ROOT_DIR/scripts/check_core_coverage.sh\""
log_and_check "[3/6] go build" "GOCACHE=\"\${GOCACHE:-/tmp/tars-go-build}\" go build ./..."
log_and_check "[4/6] openapi validation" "\"$ROOT_DIR/scripts/validate_openapi.rb\""

if [ -d "$ROOT_DIR/web" ]; then
  log_and_check "[5/6] web lint" "(cd \"$ROOT_DIR/web\" && npm run lint)"
  log_and_check "[6/6] web build" "(cd \"$ROOT_DIR/web\" && npm run build)"
else
  echo "[5/6] web lint skipped"
  echo "[6/6] web build skipped"
fi

echo "\n✅ ALL MVP checks passed (Strict Mode)"
rm -f "$LOG_FILE"
