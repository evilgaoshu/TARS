#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STARTED_AT=${SECONDS}

cd "${ROOT_DIR}"

echo "== TARS secret-scan =="
echo "scope=publishable non-test tree (.github, api, cmd, configs, deploy, docs, internal, migrations, project, scripts, specs, web, repo metadata)"
echo

scan_roots=()
for path in \
  .github \
  api \
  cmd \
  configs \
  deploy \
  docs \
  internal \
  migrations \
  project \
  scripts \
  specs \
  web \
  README.md \
  CHANGELOG.md \
  CLAUDE.md \
  CONTRIBUTING.md \
  Makefile \
  go.mod \
  go.sum \
  .gitignore \
  .dockerignore; do
  if [ -e "${path}" ]; then
    scan_roots+=("${path}")
  fi
done

if [ "${#scan_roots[@]}" -eq 0 ]; then
  echo "secret-scan=failed reason=no-scan-roots"
  exit 1
fi

files_file="$(mktemp)"
trap 'rm -f "${files_file}"' EXIT

for path in "${scan_roots[@]}"; do
  if [ -d "${path}" ]; then
    find "${path}" -type f \
      ! -name '*_test.go' \
      ! -name '*.test.ts' \
      ! -name '*.test.tsx' \
      ! -name '*.spec.ts' \
      ! -name '*.spec.tsx' \
      ! -path '*/node_modules/*' \
      ! -path '*/dist/*' \
      ! -path '*/playwright-report/*' \
      ! -path '*/test-results/*' \
      ! -path 'docs/reports/*' \
      ! -path 'docs/operations/records/*' \
      ! -path 'scripts/ci/secret-scan.sh' \
      ! -path 'scripts/ci/secret-scan-regression.sh' \
      -print >>"${files_file}"
    continue
  fi

  printf '%s\n' "${path}" >>"${files_file}"
done

sort -u "${files_file}" -o "${files_file}"

file_count="$(wc -l <"${files_file}" | tr -d '[:space:]')"
if [ "${file_count}" = "0" ]; then
  echo "secret-scan=failed reason=no-files-selected"
  exit 1
fi

echo "selected_files=${file_count}"

matches=""
# Case-insensitive: known test-fixture strings that must not reach publishable sources
ci_matches=""
ci_matches="$(xargs rg -n -i \
  -e 'tars-shared-secret' \
  -e 'JBSWY3DPEHPK3PXP' \
  -e 'password-123' \
  <"${files_file}" || true)"

# Case-sensitive: structural secret patterns (PEM keys, GitHub tokens, AWS key IDs)
# Filter out HTML/JSX placeholder= attributes which legitimately show key format hints.
struct_matches=""
struct_raw=""
struct_raw="$(xargs rg -n \
  -e '-----BEGIN (RSA |EC |OPENSSH |PGP |DSA )?PRIVATE KEY' \
  -e 'gh[pousr]_[A-Za-z0-9]{36,}' \
  -e 'AKIA[A-Z0-9]{16}' \
  <"${files_file}" || true)"
struct_matches="$(printf '%s' "${struct_raw}" | grep -v 'placeholder=' || true)"

matches="${ci_matches}${struct_matches}"

if [ -n "${matches}" ]; then
  printf '%s\n' "${matches}"
  echo
  echo "secret-scan=failed"
  exit 1
fi

echo "secret-scan=passed total=$((SECONDS - STARTED_AT))s"
