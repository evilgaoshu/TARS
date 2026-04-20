#!/usr/bin/env sh
set -eu

OPS_BASE_URL="${TARS_OPS_BASE_URL:-http://127.0.0.1:8081}"
SERVER_BASE_URL="${TARS_SERVER_BASE_URL:-}"
TOKEN="${TARS_OPS_API_TOKEN:-}"

if [ -z "$TOKEN" ]; then
  echo "TARS_OPS_API_TOKEN is required" >&2
  exit 1
fi

auth_header="Authorization: Bearer $TOKEN"

fetch_public() {
  curl -fsS "$SERVER_BASE_URL$1"
}

fetch_ops() {
  curl -fsS -H "$auth_header" "$OPS_BASE_URL$1"
}

warn=0
public_health_warning=""

echo "== TARS pilot hygiene check =="
echo "ops_base_url=$OPS_BASE_URL"
if [ -n "$SERVER_BASE_URL" ]; then
  echo "server_base_url=$SERVER_BASE_URL"
fi

health=""
ready=""
if [ -n "$SERVER_BASE_URL" ]; then
  if health="$(curl -fsS --max-time 5 "$SERVER_BASE_URL/healthz" 2>/dev/null)"; then
    :
  else
    public_health_warning="unable to fetch public /healthz from $SERVER_BASE_URL"
  fi
  if ready="$(curl -fsS --max-time 5 "$SERVER_BASE_URL/readyz" 2>/dev/null)"; then
    :
  else
    if [ -n "$public_health_warning" ]; then
      public_health_warning="$public_health_warning; unable to fetch public /readyz from $SERVER_BASE_URL"
    else
      public_health_warning="unable to fetch public /readyz from $SERVER_BASE_URL"
    fi
  fi
fi
summary="$(fetch_ops /api/v1/summary)"
setup="$(fetch_ops /api/v1/setup/status)"

echo
echo "-- public health --"
if [ -n "$SERVER_BASE_URL" ]; then
  if [ -n "$health" ]; then
    printf '%s\n' "$health"
  fi
  if [ -n "$ready" ]; then
    printf '%s\n' "$ready"
  fi
  if [ -n "$public_health_warning" ]; then
    echo "warning=$public_health_warning"
    echo "tip=如果 public server listener 不对外暴露，这是可接受的；Ops 基线仍以 /api/v1/summary 和 /api/v1/setup/status 为准。"
  fi
else
  echo "skipped (set TARS_SERVER_BASE_URL to enable /healthz and /readyz checks)"
fi

echo
echo "-- runtime summary --"
printf '%s' "$summary" | python3 -c 'import sys,json; data=json.load(sys.stdin); print("active_sessions=%s pending_approvals=%s executions_total=%s failed_outbox=%s blocked_outbox=%s" % (data.get("active_sessions",0), data.get("pending_approvals",0), data.get("executions_total",0), data.get("failed_outbox",0), data.get("blocked_outbox",0)))'

echo
echo "-- setup status --"
printf '%s' "$setup" | python3 -c 'import sys,json; data=json.load(sys.stdin); print("rollout_mode=%s telegram=%s model=%s assist=%s" % (data.get("rollout_mode","unknown"), data.get("telegram",{}).get("last_result") or "unknown", data.get("model",{}).get("provider_id") or data.get("model",{}).get("protocol") or "unconfigured", data.get("assist_model",{}).get("provider_id") or data.get("assist_model",{}).get("protocol") or "none"))'

failed_outbox="$(printf '%s' "$summary" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("failed_outbox", 0))')"
blocked_outbox="$(printf '%s' "$summary" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("blocked_outbox", 0))')"

if [ "$failed_outbox" -gt 0 ]; then
  warn=1
  echo
  echo "-- warning: failed outbox detected --"
  failed_payload="$(fetch_ops '/api/v1/outbox?status=failed')"
  printf '%s' "$failed_payload" | python3 -c 'import sys,json; data=json.load(sys.stdin); items=data.get("items", [])[:10]; print("failed_count=%s" % len(data.get("items", []))); [print("- %s aggregate=%s retry=%s error=%s" % (item.get("topic"), item.get("aggregate_id"), item.get("retry_count"), item.get("last_error","")[:160])) for item in items]'
fi

if [ "$blocked_outbox" -gt 0 ]; then
  warn=1
  echo
  echo "-- warning: blocked outbox detected --"
  blocked_payload="$(fetch_ops '/api/v1/outbox?status=blocked')"
  printf '%s' "$blocked_payload" | python3 -c 'import sys,json; data=json.load(sys.stdin); items=data.get("items", [])[:10]; print("blocked_count=%s" % len(data.get("items", []))); [print("- %s aggregate=%s reason=%s" % (item.get("topic"), item.get("aggregate_id"), item.get("blocked_reason",""))) for item in items]'
fi

echo
if [ "$warn" -eq 0 ]; then
  echo "result=clean"
else
  echo "result=attention_needed"
  echo "tip=演示前先确认 failed/blocked outbox 是否为历史样本，必要时人工 replay 或清理。"
fi
