#!/usr/bin/env sh
set -eu

BASE_URL="${TARS_OPS_BASE_URL:-http://127.0.0.1:8081}"
TOKEN="${TARS_OPS_API_TOKEN:-}"

if [ -z "$TOKEN" ]; then
  echo "TARS_OPS_API_TOKEN is required" >&2
  exit 1
fi

ALERTNAME="${TARS_DEMO_ALERTNAME:-TarsDemoManual}"
SERVICE="${TARS_DEMO_SERVICE:-sshd}"
HOST="${TARS_DEMO_HOST:-192.168.3.106}"
SEVERITY="${TARS_DEMO_SEVERITY:-critical}"
SUMMARY="${TARS_DEMO_SUMMARY:-manual demo smoke from run_demo_smoke.sh}"
POLL_SECONDS="${TARS_DEMO_POLL_SECONDS:-180}"

auth_header="Authorization: Bearer $TOKEN"

fetch_ops() {
  curl -fsS -H "$auth_header" "$BASE_URL$1"
}

echo "== TARS demo smoke =="
echo "base_url=$BASE_URL"
echo "alertname=$ALERTNAME service=$SERVICE host=$HOST severity=$SEVERITY"

response="$(curl -fsS -H "$auth_header" -H 'Content-Type: application/json' \
  -X POST "$BASE_URL/api/v1/smoke/alerts" \
  -d "{
    \"alertname\":\"$ALERTNAME\",
    \"service\":\"$SERVICE\",
    \"host\":\"$HOST\",
    \"severity\":\"$SEVERITY\",
    \"summary\":\"$SUMMARY\"
  }")"

session_id="$(printf '%s' "$response" | python3 -c 'import sys,json; print(json.load(sys.stdin)["session_id"])')"
session_status="$(printf '%s' "$response" | python3 -c 'import sys,json; print(json.load(sys.stdin)["status"])')"
tg_target="$(printf '%s' "$response" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("tg_target",""))')"

echo "session_id=$session_id"
echo "initial_status=$session_status"
echo "telegram_target=$tg_target"
echo "web_session_url=$BASE_URL/sessions/$session_id"

if [ "$POLL_SECONDS" -le 0 ]; then
  echo "polling=disabled"
  exit 0
fi

deadline=$(( $(date +%s) + POLL_SECONDS ))
last_status=""
while [ "$(date +%s)" -lt "$deadline" ]; do
  detail="$(fetch_ops /api/v1/sessions/$session_id)"
  current="$(printf '%s' "$detail" | python3 -c 'import sys,json; data=json.load(sys.stdin); execution=data.get("executions",[{}])[0] if data.get("executions") else {}; verification=data.get("verification") or {}; print("|".join([data.get("status",""), execution.get("status",""), verification.get("status",""), data.get("diagnosis_summary","")[:120]]))')"
  status="$(printf '%s' "$current" | cut -d '|' -f 1)"
  execution="$(printf '%s' "$current" | cut -d '|' -f 2)"
  verification="$(printf '%s' "$current" | cut -d '|' -f 3)"
  summary="$(printf '%s' "$current" | cut -d '|' -f 4-)"

  if [ "$status" != "$last_status" ]; then
    echo "status=$status execution=$execution verification=$verification summary=$summary"
    last_status="$status"
  fi

  case "$status" in
    resolved|failed)
      echo "result=finished"
      exit 0
      ;;
    pending_approval)
      echo "waiting=telegram_approval"
      ;;
  esac
  sleep 5
done

echo "result=timeout"
echo "tip=如果当前停在 pending_approval，请到 Telegram 批准执行后再观察。"
