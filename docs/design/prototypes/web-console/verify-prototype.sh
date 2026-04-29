#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOC="$ROOT/../../web-console-product-prototype-2026-04-28.md"
HTML="$ROOT/index.html"

test -f "$DOC"
test -f "$HTML"

required_routes=(
  "route-login"
  "route-setup"
  "route-runtime-checks"
  "route-runtime"
  "route-sessions"
  "route-session-detail"
  "route-executions"
  "route-execution-detail"
  "route-chat"
  "route-inbox"
  "route-providers"
  "route-channels"
  "route-notification-templates"
  "route-connectors"
  "route-connector-detail"
  "route-skills"
  "route-skill-detail"
  "route-automations"
  "route-extensions"
  "route-knowledge"
  "route-observability"
  "route-audit"
  "route-logs"
  "route-outbox"
  "route-ops"
  "route-identity"
  "route-identity-providers"
  "route-identity-users"
  "route-identity-groups"
  "route-identity-roles"
  "route-identity-agent-roles"
  "route-identity-people"
  "route-org"
  "route-docs"
)

for route in "${required_routes[@]}"; do
  grep -q "id=\"$route\"" "$HTML"
done

required_terms=(
  "On-call Evidence Desk"
  "IBM Plex Sans"
  "IBM Plex Mono"
  "theme-toggle"
  "Global Search"
  "Approval Dialog"
  "Reason / note"
  "Raw Payload Fold"
  "Empty State"
  "Error State"
  "Loading State"
  "Degraded State"
  "Disabled State"
  "@media (max-width: 640px)"
  "390px mobile"
  "future data"
)

for term in "${required_terms[@]}"; do
  grep -q "$term" "$HTML" "$DOC"
done

grep -q "React 19 / swagger-ui-react" "$DOC"
grep -q "web/.gitignore logs" "$DOC"
grep -q "/dashboard -> /runtime" "$DOC"
grep -q "implementation issues" "$DOC"

echo "web-console prototype artifacts verified"
