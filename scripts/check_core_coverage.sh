#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/tars-core-coverage.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

package_patterns="
./internal/modules/alertintake
./internal/modules/workflow
./internal/modules/reasoning
./internal/modules/action/...
./internal/modules/channel/...
"

packages=$(
  for pattern in $package_patterns; do
    (cd "$ROOT_DIR" && go list "$pattern")
  done | awk 'NF && !seen[$0]++'
)

fail=0

for pkg in $packages; do
  safe_name=$(printf '%s' "$pkg" | tr '/.' '__')
  profile="$TMP_DIR/$safe_name.out"

  if ! (cd "$ROOT_DIR" && GOCACHE="${GOCACHE:-/tmp/tars-go-build}" go test -coverprofile="$profile" "$pkg"); then
    echo "$pkg: coverage check failed"
    fail=1
    continue
  fi

  percent=$(cd "$ROOT_DIR" && go tool cover -func="$profile" | awk '/^total:/{printf "%.1f", $3 + 0}')
  printf '%s: %s%%\n' "$pkg" "$percent"

  if awk "BEGIN { exit !($percent < 90.0) }"; then
    echo "$pkg: below 90.0% minimum"
    fail=1
  fi
done

exit "$fail"
