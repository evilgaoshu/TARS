#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() {
  printf 'secret-scan-regression: %s\n' "$*" >&2
  exit 1
}

run_in_temp_repo() {
  local temp_repo="$1"
  local output_file="$2"

  mkdir -p "${temp_repo}/scripts/ci"
  cp "${ROOT_DIR}/scripts/ci/secret-scan.sh" "${temp_repo}/scripts/ci/secret-scan.sh"
  chmod +x "${temp_repo}/scripts/ci/secret-scan.sh"

  (
    cd "${temp_repo}"
    bash ./scripts/ci/secret-scan.sh
  ) >"${output_file}" 2>&1
}

test_publishable_docs_and_scripts_are_scanned() {
  local temp_repo output
  temp_repo="$(mktemp -d)"
  output="$(mktemp)"
  trap 'rm -rf "${temp_repo}"; rm -f "${output}"' RETURN

  mkdir -p \
    "${temp_repo}/docs/operations" \
    "${temp_repo}/docs/reports" \
    "${temp_repo}/docs/operations/records" \
    "${temp_repo}/scripts/helpers" \
    "${temp_repo}/web/src" \
    "${temp_repo}/internal/modules/demo"

  cat >"${temp_repo}/README.md" <<'EOF'
# Demo repo
EOF

  cat >"${temp_repo}/docs/operations/push-readiness.md" <<'EOF'
review token: ghp_0123456789abcdefghijklmnopqrstuvwxyzABCD
EOF

  cat >"${temp_repo}/scripts/helpers/aws-example.sh" <<'EOF'
#!/usr/bin/env bash
echo "AKIA0123456789ABCDEF"
EOF

  cat >"${temp_repo}/web/src/connector-form.tsx" <<'EOF'
export const placeholder = <input placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" />;
EOF

  cat >"${temp_repo}/internal/modules/demo/demo_test.go" <<'EOF'
package demo

const fixturePassword = "password-123"
EOF

  cat >"${temp_repo}/docs/reports/archive.md" <<'EOF'
historical regex: ghp_0123456789abcdefghijklmnopqrstuvwxyzABCD
EOF

  cat >"${temp_repo}/docs/operations/records/archive.md" <<'EOF'
historical regex: AKIA0123456789ABCDEF
EOF

  if run_in_temp_repo "${temp_repo}" "${output}"; then
    fail "expected expanded publishable-tree secret scan to fail on docs/scripts secrets"
  fi

  grep -q 'docs/operations/push-readiness.md' "${output}" || \
    fail "expected docs/operations files to be included in secret scan"
  grep -q 'scripts/helpers/aws-example.sh' "${output}" || \
    fail "expected scripts files to be included in secret scan"

  if grep -q 'docs/reports/archive.md' "${output}"; then
    fail "expected docs/reports historical archive to stay excluded from machine secret scan"
  fi
  if grep -q 'docs/operations/records/archive.md' "${output}"; then
    fail "expected docs/operations/records historical archive to stay excluded from machine secret scan"
  fi
  if grep -q 'demo_test.go' "${output}"; then
    fail "expected Go test fixtures to stay excluded from machine secret scan"
  fi
  if grep -q 'connector-form.tsx' "${output}"; then
    fail "expected placeholder private key hints to stay exempt"
  fi
}

test_publishable_tree_passes_when_only_archives_and_fixtures_match() {
  local temp_repo output
  temp_repo="$(mktemp -d)"
  output="$(mktemp)"
  trap 'rm -rf "${temp_repo}"; rm -f "${output}"' RETURN

  mkdir -p \
    "${temp_repo}/docs/reports" \
    "${temp_repo}/docs/operations/records" \
    "${temp_repo}/web/src" \
    "${temp_repo}/internal/modules/demo"

  cat >"${temp_repo}/README.md" <<'EOF'
# Demo repo
EOF

  cat >"${temp_repo}/docs/reports/archive.md" <<'EOF'
historical regex: ghp_0123456789abcdefghijklmnopqrstuvwxyzABCD
EOF

  cat >"${temp_repo}/docs/operations/records/archive.md" <<'EOF'
historical regex: AKIA0123456789ABCDEF
EOF

  cat >"${temp_repo}/web/src/connector-form.tsx" <<'EOF'
export const placeholder = <input placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" />;
EOF

  cat >"${temp_repo}/internal/modules/demo/demo_test.go" <<'EOF'
package demo

const fixturePassword = "password-123"
EOF

  if ! run_in_temp_repo "${temp_repo}" "${output}"; then
    cat "${output}" >&2
    fail "expected publishable-tree secret scan to ignore archives and known fixtures"
  fi

  grep -q 'secret-scan=passed' "${output}" || fail "expected secret scan success output"
}

test_publishable_docs_and_scripts_are_scanned
test_publishable_tree_passes_when_only_archives_and_fixtures_match

printf 'secret-scan-regression=passed\n'
