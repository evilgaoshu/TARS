#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
PILOT_DIR="${ROOT_DIR}/deploy/pilot"

OPS_BASE_URL="${TARS_OPS_BASE_URL:-http://127.0.0.1:8081}"
SERVER_BASE_URL="${TARS_SERVER_BASE_URL:-http://127.0.0.1:8081}"
OPS_TOKEN="${TARS_OPS_API_TOKEN:-}"
VMALERT_SECRET="${TARS_VMALERT_WEBHOOK_SECRET:-}"
TELEGRAM_SECRET="${TARS_TELEGRAM_WEBHOOK_SECRET:-}"
ALERT_FIXTURE="${TARS_GOLDEN_ALERT_FIXTURE:-${PILOT_DIR}/golden_path_alert_v1.json}"
CALLBACK_FIXTURE="${TARS_GOLDEN_CALLBACK_FIXTURE:-${PILOT_DIR}/golden_path_telegram_callback_v1.json}"
AUTO_APPROVE="${TARS_GOLDEN_AUTO_APPROVE:-0}"
POLL_SECONDS="${TARS_GOLDEN_POLL_SECONDS:-180}"

require_bin() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required binary: %s\n' "$1" >&2
    exit 1
  fi
}
require_bin python3
require_bin curl

if [ -z "$OPS_TOKEN" ]; then
  printf 'TARS_OPS_API_TOKEN is required\n' >&2
  exit 1
fi

if [ ! -f "$ALERT_FIXTURE" ]; then
  printf 'alert fixture not found: %s\n' "$ALERT_FIXTURE" >&2
  exit 1
fi

auth_header="Authorization: Bearer $OPS_TOKEN"

fetch_ops() {
  curl -fsS -H "$auth_header" "$OPS_BASE_URL$1"
}

post_alert() {
  if [ -n "$VMALERT_SECRET" ]; then
    curl -fsS -H "X-Tars-Signature: $VMALERT_SECRET" -H 'Content-Type: application/json' -X POST "$SERVER_BASE_URL/api/v1/webhooks/vmalert" --data-binary "@$ALERT_FIXTURE"
  else
    curl -fsS -H 'Content-Type: application/json' -X POST "$OPS_BASE_URL/api/v1/smoke/alerts" -d "$(python3 - "$ALERT_FIXTURE" <<'PY'
import json
import sys
path = sys.argv[1]
payload = json.load(open(path))
alert = payload["alerts"][0]
labels = alert.get("labels", {})
annotations = alert.get("annotations", {})
print(json.dumps({
    "alertname": labels.get("alertname", "GoldenPathManual"),
    "service": labels.get("service", "api"),
    "host": labels.get("host") or labels.get("instance") or "host-3",
    "severity": labels.get("severity", "critical"),
    "summary": annotations.get("summary", "golden path smoke")
}))
PY
)"
  fi
}

printf '== TARS golden path replay ==\n'
printf 'ops_base_url=%s\n' "$OPS_BASE_URL"
printf 'server_base_url=%s\n' "$SERVER_BASE_URL"
printf 'alert_fixture=%s\n' "$ALERT_FIXTURE"
printf 'callback_fixture=%s\n' "$CALLBACK_FIXTURE"

response="$(post_alert)"
session_id="$(printf '%s' "$response" | python3 -c 'import sys,json; data=json.load(sys.stdin); items=data.get("session_ids") or []; print(items[0] if items else data.get("session_id",""))')"
if [ -z "$session_id" ]; then
  printf 'failed to obtain session id from response: %s\n' "$response" >&2
  exit 1
fi

printf 'session_id=%s\n' "$session_id"
printf 'web_session_url=%s/sessions/%s\n' "$OPS_BASE_URL" "$session_id"

detail="$(fetch_ops "/api/v1/sessions/$session_id")"
status_line="$(printf '%s' "$detail" | python3 -c 'import sys,json; data=json.load(sys.stdin); gs=data.get("golden_summary") or {}; executions=data.get("executions") or []; print("|".join([data.get("status",""), gs.get("headline",""), gs.get("conclusion",""), gs.get("next_action",""), str(len(executions))]))')"
status="$(printf '%s' "$status_line" | cut -d '|' -f 1)"
headline="$(printf '%s' "$status_line" | cut -d '|' -f 2)"
conclusion="$(printf '%s' "$status_line" | cut -d '|' -f 3)"
next_action="$(printf '%s' "$status_line" | cut -d '|' -f 4)"
execution_count="$(printf '%s' "$status_line" | cut -d '|' -f 5)"

printf 'initial_status=%s\n' "$status"
printf 'headline=%s\n' "$headline"
printf 'conclusion=%s\n' "$conclusion"
printf 'next_action=%s\n' "$next_action"
printf 'execution_count=%s\n' "$execution_count"

if [ "$AUTO_APPROVE" = "1" ]; then
  if [ ! -f "$CALLBACK_FIXTURE" ]; then
    printf 'callback fixture not found: %s\n' "$CALLBACK_FIXTURE" >&2
    exit 1
  fi
  execution_id="$(printf '%s' "$detail" | python3 -c 'import sys,json; data=json.load(sys.stdin); executions=data.get("executions") or []; print(executions[0].get("execution_id","") if executions else "")')"
  if [ -z "$execution_id" ]; then
    printf 'no execution draft available for auto-approve\n' >&2
    exit 1
  fi
  callback_chat_id="$(printf '%s' "$detail" | python3 -c 'import sys,json; data=json.load(sys.stdin); alert=data.get("alert") or {}; labels=alert.get("labels") or {}; print(labels.get("telegram_target") or labels.get("chat_id") or "-1001001")')"
  callback_payload="$(python3 - "$CALLBACK_FIXTURE" "$execution_id" "$callback_chat_id" <<'PY'
import json
import sys
path = sys.argv[1]
execution_id = sys.argv[2]
chat_id = sys.argv[3]
payload = json.load(open(path))
payload["callback_query"]["data"] = payload["callback_query"]["data"].replace("{{EXECUTION_ID}}", execution_id)
payload["callback_query"]["message"]["chat"]["id"] = str(chat_id)
print(json.dumps(payload))
PY
)"
  if [ -n "$TELEGRAM_SECRET" ]; then
    curl -fsS -H "X-Telegram-Bot-Api-Secret-Token: $TELEGRAM_SECRET" -H 'Content-Type: application/json' -X POST "$SERVER_BASE_URL/api/v1/channels/telegram/webhook" -d "$callback_payload" >/dev/null
  else
    curl -fsS -H 'Content-Type: application/json' -X POST "$SERVER_BASE_URL/api/v1/channels/telegram/webhook" -d "$callback_payload" >/dev/null
  fi
  printf 'auto_approve=triggered execution_id=%s\n' "$execution_id"
fi

if [ "$POLL_SECONDS" -le 0 ]; then
  printf 'polling=disabled\n'
  exit 0
fi

deadline=$(( $(date +%s) + POLL_SECONDS ))
last_snapshot=""
while [ "$(date +%s)" -lt "$deadline" ]; do
  detail="$(fetch_ops "/api/v1/sessions/$session_id")"
  snapshot="$(printf '%s' "$detail" | python3 -c 'import sys,json; data=json.load(sys.stdin); gs=data.get("golden_summary") or {}; executions=data.get("executions") or []; execution=executions[-1] if executions else {}; verification=data.get("verification") or {}; notifications=data.get("notifications") or []; print("|".join([data.get("status",""), execution.get("status",""), verification.get("status",""), gs.get("notification_headline",""), gs.get("execution_headline",""), str(len(notifications))]))')"
  if [ "$snapshot" != "$last_snapshot" ]; then
    printf 'snapshot=%s\n' "$snapshot"
    last_snapshot="$snapshot"
  fi
  current_status="$(printf '%s' "$snapshot" | cut -d '|' -f 1)"
  case "$current_status" in
    resolved|failed)
      printf 'result=finished status=%s\n' "$current_status"
      exit 0
      ;;
  esac
  sleep 5
done

printf 'result=timeout\n'
printf 'tip=如果仍停在 pending_approval，可设 TARS_GOLDEN_AUTO_APPROVE=1 或在 Telegram 手工批准。\n'
