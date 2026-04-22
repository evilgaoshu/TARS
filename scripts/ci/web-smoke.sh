#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"
WEB_DIR="${ROOT_DIR}/web"
STARTED_AT=${SECONDS}
BASE_URL="${TARS_PLAYWRIGHT_BASE_URL:-${TARS_OPS_BASE_URL:-http://192.168.3.100:8081}}"
TOKEN="$(shared_ops_token_normalize "${TARS_PLAYWRIGHT_TOKEN:-}" 2>/dev/null || true)"
if [[ -z "${TOKEN}" ]]; then
  TOKEN="$(shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}" 2>/dev/null || true)"
fi
MODE="${TARS_WEB_SMOKE_MODE:-headless}"

if [[ -z "${TOKEN}" ]] && TOKEN="$(shared_ops_token_resolve 2>/dev/null || true)"; then
  :
fi

if [[ -z "${TOKEN}" ]]; then
  echo "TARS_PLAYWRIGHT_TOKEN or TARS_OPS_API_TOKEN is required for web-smoke" >&2
  echo "tip=可显式 export TARS_PLAYWRIGHT_TOKEN/TARS_OPS_API_TOKEN，或设置 TARS_REMOTE_USER 让脚本从共享机 shared-test.env 自动解析。" >&2
  exit 1
fi

if [[ ! -d "${WEB_DIR}/node_modules" ]]; then
  echo "missing ${WEB_DIR}/node_modules" >&2
  echo "tip=先执行 make web-install，再重试 make web-smoke。" >&2
  exit 1
fi

export TARS_PLAYWRIGHT_BASE_URL="${BASE_URL}"
export TARS_PLAYWRIGHT_TOKEN="${TOKEN}"

cd "${WEB_DIR}"

echo "== TARS web smoke =="
echo "scope=L2/L3 Playwright control-plane smoke"
echo "note=jsdom/vitest coverage runs separately via 'cd web && npm run test'"
echo "base_url=${BASE_URL}"
echo "mode=${MODE}"
echo

if [[ "${MODE}" == "headed" || "${TARS_PLAYWRIGHT_HEADED:-0}" == "1" ]]; then
  if ! npm run test:smoke:headed; then
    echo "web_smoke=failed" >&2
    echo "tip=如果错误提示缺少浏览器，先执行 cd web && npx playwright install chromium。" >&2
    exit 1
  fi
else
  if ! npm run test:smoke; then
    echo "web_smoke=failed" >&2
    echo "tip=如果错误提示缺少浏览器，先执行 cd web && npx playwright install chromium。" >&2
    exit 1
  fi
fi

echo
echo "web-smoke=passed total=$((SECONDS - STARTED_AT))s"
