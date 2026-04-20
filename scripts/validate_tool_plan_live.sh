#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "${SCRIPT_DIR}/.." && pwd)"
source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"

BASE_URL="${TARS_OPS_BASE_URL:-http://127.0.0.1:8081}"
TOKEN="$(shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}" 2>/dev/null || true)"
PROFILE="${TARS_VALIDATE_PROFILE:-all}"
VALIDATOR_PATH="${SCRIPT_DIR}/lib/validate_tool_plan_smoke.py"
HOST="${TARS_VALIDATE_HOST:-127.0.0.1:9100}"
SERVICE="${TARS_VALIDATE_SERVICE:-api}"
METRICS_CONNECTOR="${TARS_VALIDATE_METRICS_CONNECTOR:-victoriametrics-main}"
METRICS_QUERY="${TARS_VALIDATE_METRICS_QUERY:-up}"
LOGS_CONNECTOR="${TARS_VALIDATE_LOGS_CONNECTOR:-victorialogs-main}"
LOGS_QUERY="${TARS_VALIDATE_LOGS_QUERY:-tars-observability-host-file-test}"
LOGS_TIME_RANGE="${TARS_VALIDATE_LOGS_TIME_RANGE:-168h}"
LOGS_QUERY_RETRIES="${TARS_VALIDATE_LOGS_QUERY_RETRIES:-15}"
LOGS_QUERY_RETRY_DELAY="${TARS_VALIDATE_LOGS_QUERY_RETRY_DELAY:-2}"
LOGS_MARKER_REMOTE_HOST="${TARS_VALIDATE_LOGS_MARKER_REMOTE_HOST:-}"
LOGS_MARKER_REMOTE_USER="${TARS_VALIDATE_LOGS_MARKER_REMOTE_USER:-root}"
LOGS_MARKER_REMOTE_PATH="${TARS_VALIDATE_LOGS_MARKER_REMOTE_PATH:-/var/log/tars-observability-test.log}"
CAPABILITY_CONNECTOR="${TARS_VALIDATE_CAPABILITY_CONNECTOR:-skill-source-main}"
CAPABILITY_ID="${TARS_VALIDATE_CAPABILITY_ID:-source.sync}"
EXPECTED_CAPABILITY_HTTP="${TARS_VALIDATE_EXPECTED_CAPABILITY_HTTP:-202}"
OBSERVABILITY_CONNECTOR="${TARS_VALIDATE_OBSERVABILITY_CONNECTOR:-observability-main}"
DELIVERY_CONNECTOR="${TARS_VALIDATE_DELIVERY_CONNECTOR:-delivery-main}"
OBSERVABILITY_QUERY="${TARS_VALIDATE_OBSERVABILITY_QUERY:-disk}"
DELIVERY_QUERY="${TARS_VALIDATE_DELIVERY_QUERY:-release}"
RUN_DENY="${TARS_VALIDATE_RUN_DENY:-1}"
EXPECTED_DENIED_HTTP="${TARS_VALIDATE_EXPECTED_DENIED_HTTP:-403}"
DENY_PATTERN="${TARS_VALIDATE_DENY_PATTERN:-$CAPABILITY_ID}"
SMOKE_HOST="${TARS_VALIDATE_SMOKE_HOST:-}"
SMOKE_ALERTNAME="${TARS_VALIDATE_SMOKE_ALERTNAME:-TarsToolPlanLiveValidation}"
SMOKE_SUMMARY="${TARS_VALIDATE_SMOKE_SUMMARY:-过去一小时机器负载怎么样}"
SMOKE_LOGS_SUMMARY="${TARS_VALIDATE_SMOKE_LOGS_SUMMARY:-先看 tars-observability-host-file-test 共享机日志 marker，确认 logs evidence path}"
SMOKE_OBSERVABILITY_SUMMARY="${TARS_VALIDATE_SMOKE_OBSERVABILITY_SUMMARY:-trace api latency root cause，先看 traces 和 observability evidence}"
SMOKE_DELIVERY_SUMMARY="${TARS_VALIDATE_SMOKE_DELIVERY_SUMMARY:-最近 api 报错和最近一次发布有关系吗，先查 release correlation}"
POLL_SECONDS="${TARS_VALIDATE_POLL_SECONDS:-90}"
AUTH_SNAPSHOT_FILE=""

case "$PROFILE" in
  all|metrics|logs|observability|delivery)
    ;;
  *)
    echo "unsupported TARS_VALIDATE_PROFILE: $PROFILE" >&2
    exit 1
    ;;
esac

if [ "${TARS_VALIDATE_RUN_SMOKE+set}" = "set" ]; then
  RUN_SMOKE="$TARS_VALIDATE_RUN_SMOKE"
else
  case "$PROFILE" in
    metrics|logs|observability|delivery)
      RUN_SMOKE=1
      ;;
    *)
      RUN_SMOKE=0
      ;;
  esac
fi

if [ "${TARS_VALIDATE_SMOKE_SCENARIOS+set}" = "set" ]; then
  SMOKE_SCENARIOS="$TARS_VALIDATE_SMOKE_SCENARIOS"
else
  case "$PROFILE" in
    all)
      SMOKE_SCENARIOS="logs,observability,delivery"
      ;;
    metrics)
      SMOKE_SCENARIOS="metrics"
      ;;
    logs)
      SMOKE_SCENARIOS="logs"
      ;;
    observability)
      SMOKE_SCENARIOS="observability"
      ;;
    delivery)
      SMOKE_SCENARIOS="delivery"
      ;;
  esac
fi

if [ -z "$LOGS_MARKER_REMOTE_HOST" ] && [ "${TARS_REMOTE_HOST:-}" = "192.168.3.100" ]; then
  LOGS_MARKER_REMOTE_HOST="192.168.3.9"
fi

if [ -z "$TOKEN" ]; then
  TOKEN="$(shared_ops_token_resolve 2>/dev/null || true)"
fi

if [ -z "$TOKEN" ]; then
  echo "failed to resolve TARS_OPS_API_TOKEN for tool-plan live validation" >&2
  echo "tip=脚本已自动尝试远端 canonical shared-test.env 回退；192.168.3.100 默认使用 root。若不是默认 SSH 用户，再显式设置 TARS_REMOTE_USER。" >&2
  exit 1
fi

auth_header="Authorization: Bearer $TOKEN"

json_string() {
  python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$1"
}

cleanup() {
  if [ -n "$AUTH_SNAPSHOT_FILE" ] && [ -f "$AUTH_SNAPSHOT_FILE" ]; then
    restore_authorization_config || true
    rm -f "$AUTH_SNAPSHOT_FILE"
  fi
}

trap cleanup EXIT INT TERM

call_json() {
  method="$1"
  path="$2"
  payload="${3:-}"
  if [ -n "$payload" ]; then
    curl -fsS -H "$auth_header" -H 'Content-Type: application/json' -X "$method" "$BASE_URL$path" -d "$payload"
  else
    curl -fsS -H "$auth_header" -X "$method" "$BASE_URL$path"
  fi
}

call_with_status() {
  method="$1"
  path="$2"
  payload="${3:-}"
  tmp="$(mktemp)"
  code="$(curl -sS -o "$tmp" -w '%{http_code}' -H "$auth_header" -H 'Content-Type: application/json' -X "$method" "$BASE_URL$path" -d "$payload")"
  cat "$tmp"
  rm -f "$tmp"
  printf '\n%s' "$code"
}

snapshot_authorization_config() {
  AUTH_SNAPSHOT_FILE="$(mktemp)"
  call_json GET /api/v1/config/authorization >"$AUTH_SNAPSHOT_FILE"
}

set_deny_authorization_rule() {
  payload="$(python3 - "$AUTH_SNAPSHOT_FILE" "$DENY_PATTERN" <<'PY'
import json
import sys

snapshot_path, pattern = sys.argv[1], sys.argv[2]
with open(snapshot_path, "r", encoding="utf-8") as fh:
    snapshot = json.load(fh)
config = snapshot.get("config") or {}
entries = list(config.get("hard_deny_mcp_skill") or [])
if pattern not in entries:
    entries.append(pattern)
config["hard_deny_mcp_skill"] = entries
print(json.dumps({
    "config": config,
    "operator_reason": "validate_tool_plan_live.sh temporary hard deny check"
}, ensure_ascii=False))
PY
)"
  call_json PUT /api/v1/config/authorization "$payload" >/dev/null
}

restore_authorization_config() {
  payload="$(python3 - "$AUTH_SNAPSHOT_FILE" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    snapshot = json.load(fh)
print(json.dumps({
    "config": snapshot.get("config") or {},
    "operator_reason": "validate_tool_plan_live.sh restore original authorization config"
}, ensure_ascii=False))
PY
)"
  call_json PUT /api/v1/config/authorization "$payload" >/dev/null
}

echo "== TARS tool-plan live validation =="
echo "base_url=$BASE_URL"
echo "profile=$PROFILE"
echo "smoke_enabled=$RUN_SMOKE scenarios=$SMOKE_SCENARIOS evidence_only=executions=0"
echo "host=$HOST service=$SERVICE metrics_connector=$METRICS_CONNECTOR logs_connector=$LOGS_CONNECTOR capability_connector=$CAPABILITY_CONNECTOR metrics_query=$METRICS_QUERY"

echo "-- setup/status --"
setup="$(call_json GET /api/v1/setup/status)"
printf '%s' "$setup" | python3 -c 'import json,sys; data=json.load(sys.stdin); connectors=data.get("connectors") or {}; metrics=connectors.get("metrics_runtime") or {}; execution=connectors.get("execution_runtime") or {}; smoke=data.get("smoke_defaults") or {}; print("metrics_primary=", ((metrics.get("primary") or {}).get("connector_id") or "")); print("metrics_component=", metrics.get("component") or ""); print("execution_component=", execution.get("component") or ""); print("smoke_hosts=", ",".join(smoke.get("hosts") or []))'
if [ -z "$SMOKE_HOST" ]; then
  SMOKE_HOST="$(printf '%s' "$setup" | python3 -c 'import json,sys; data=json.load(sys.stdin); hosts=((data.get("smoke_defaults") or {}).get("hosts") or []); print(hosts[0] if hosts else "")')"
fi
case "$SMOKE_HOST" in
  ""|REPLACE_WITH_*)
    if [ -n "${TARS_REMOTE_HOST:-}" ]; then
      SMOKE_HOST="${TARS_REMOTE_HOST}:9100"
    fi
    ;;
esac
if [ -z "$SMOKE_HOST" ]; then
  SMOKE_HOST="$HOST"
fi

if [ "$PROFILE" = "all" ] || [ "$PROFILE" = "metrics" ] || [ "$PROFILE" = "logs" ] || [ "$PROFILE" = "observability" ] || [ "$PROFILE" = "delivery" ]; then
  echo "-- explicit metrics.query_range --"
  metrics_payload="$(printf '{"connector_id":%s,"mode":"range","window":"1h","step":"5m","query":%s}' "$(json_string "$METRICS_CONNECTOR")" "$(json_string "$METRICS_QUERY")")"
  metrics_response="$(call_json POST "/api/v1/connectors/$METRICS_CONNECTOR/metrics/query" "$metrics_payload")"
  if ! printf '%s' "$metrics_response" | python3 -c 'import json,sys; data=json.load(sys.stdin); runtime=data.get("runtime") or {}; series=data.get("series") or []; points=max((len((item or {}).get("values") or []) for item in series), default=0); print("runtime=", runtime.get("connector_id") or runtime.get("protocol") or ""); print("series_count=", len(series)); print("points=", points); sys.exit(0 if len(series) > 0 and points > 0 else 1)'; then
    echo "metrics.query_range returned no real series data" >&2
    exit 1
  fi
fi

if [ "$PROFILE" = "all" ]; then
echo "-- capability invoke --"
capability_payload="$(cat <<JSON
{
  "capability_id": "$CAPABILITY_ID",
  "params": {"source":"live-validation"},
  "operator_reason": "live validation from validate_tool_plan_live.sh"
}
JSON
)"
capability_combined="$(call_with_status POST "/api/v1/connectors/$CAPABILITY_CONNECTOR/capabilities/invoke" "$capability_payload")"
capability_http="$(printf '%s' "$capability_combined" | tail -n 1)"
capability_body="$(printf '%s' "$capability_combined" | sed '$d')"
echo "capability_http=$capability_http"
printf '%s' "$capability_body" | python3 -c 'import json,sys; data=json.load(sys.stdin); print("capability_status=", data.get("status") or ""); print("capability_rule=", ((data.get("metadata") or {}).get("rule_id") or ""))'
if [ "$capability_http" != "$EXPECTED_CAPABILITY_HTTP" ]; then
  echo "unexpected capability invoke HTTP status: expected $EXPECTED_CAPABILITY_HTTP got $capability_http" >&2
  exit 1
fi

if [ "$RUN_DENY" = "1" ]; then
  echo "-- capability invoke (deny path) --"
  snapshot_authorization_config
  set_deny_authorization_rule
  deny_combined="$(call_with_status POST "/api/v1/connectors/$CAPABILITY_CONNECTOR/capabilities/invoke" "$capability_payload")"
  deny_http="$(printf '%s' "$deny_combined" | tail -n 1)"
  deny_body="$(printf '%s' "$deny_combined" | sed '$d')"
  echo "deny_http=$deny_http"
  printf '%s' "$deny_body" | python3 -c 'import json,sys; data=json.load(sys.stdin); print("deny_status=", data.get("status") or ""); print("deny_rule=", ((data.get("metadata") or {}).get("rule_id") or ""))'
  if [ "$deny_http" != "$EXPECTED_DENIED_HTTP" ]; then
    echo "unexpected capability deny HTTP status: expected $EXPECTED_DENIED_HTTP got $deny_http" >&2
    exit 1
  fi
  restore_authorization_config
  rm -f "$AUTH_SNAPSHOT_FILE"
  AUTH_SNAPSHOT_FILE=""
fi
fi

if [ "$PROFILE" = "all" ] || [ "$PROFILE" = "logs" ]; then
  echo "-- logs.query --"
  if [ -n "$LOGS_MARKER_REMOTE_HOST" ]; then
    ssh -o BatchMode=yes -o ConnectTimeout=5 "$LOGS_MARKER_REMOTE_USER@$LOGS_MARKER_REMOTE_HOST" \
      "printf 'tars-observability-host-file-test host=%s ts=%s\n' '$LOGS_MARKER_REMOTE_HOST' '$(date -u +%Y-%m-%dT%H:%M:%SZ)' >> '$LOGS_MARKER_REMOTE_PATH'"
  fi
  logs_payload="$(cat <<JSON
{
  "capability_id": "logs.query",
  "params": {
    "query": "$LOGS_QUERY",
    "limit": 5,
    "time_range": "$LOGS_TIME_RANGE"
  },
  "operator_reason": "live logs validation"
}
JSON
)"
  logs_ok=0
  logs_attempt=1
  while [ "$logs_attempt" -le "$LOGS_QUERY_RETRIES" ]; do
    logs_response="$(call_json POST "/api/v1/connectors/$LOGS_CONNECTOR/capabilities/invoke" "$logs_payload")"
    if printf '%s' "$logs_response" | python3 -c 'import json,sys; data=json.load(sys.stdin); output=data.get("output") or {}; arts=data.get("artifacts") or []; logs=output.get("logs") or []; matched=any("tars-observability-host-file-test" in (((item or {}).get("_msg") or "")) for item in logs); print("logs_status=", data.get("status") or ""); print("logs_count=", output.get("result_count") or 0); print("logs_artifacts=", len(arts)); print("logs_summary=", output.get("summary") or ""); sys.exit(0 if data.get("status")=="completed" and int(output.get("result_count") or 0) > 0 and matched and len(arts) > 0 else 1)'; then
      logs_ok=1
      break
    fi
    logs_attempt=$((logs_attempt + 1))
    sleep "$LOGS_QUERY_RETRY_DELAY"
  done
  if [ "$logs_ok" -ne 1 ]; then
    echo "logs.query failed to return fresh shared marker evidence" >&2
    exit 1
  fi
fi

if [ "$PROFILE" = "all" ] || [ "$PROFILE" = "observability" ] || [ "$PROFILE" = "delivery" ]; then
echo "-- observability.query --"
observability_payload="$(cat <<JSON
{
  "capability_id": "observability.query",
  "params": {
    "mode": "alerts",
    "query": "$OBSERVABILITY_QUERY",
    "service": "$SERVICE",
    "limit": 20
  },
  "operator_reason": "live observability validation"
}
JSON
)"
observability_response="$(call_json POST "/api/v1/connectors/$OBSERVABILITY_CONNECTOR/capabilities/invoke" "$observability_payload")"
printf '%s' "$observability_response" | python3 -c 'import json,sys; data=json.load(sys.stdin); artifacts=data.get("artifacts") or []; output=data.get("output") or {}; print("observability_status=", data.get("status") or ""); print("observability_count=", output.get("result_count") or 0); print("observability_artifacts=", len(artifacts)); print("observability_source=", output.get("source") or ""); sys.exit(0 if data.get("status")=="completed" and int(output.get("result_count") or 0) > 0 and len(artifacts) > 0 else 1)'
fi

if [ "$PROFILE" = "all" ] || [ "$PROFILE" = "delivery" ]; then
echo "-- delivery.query --"
delivery_payload="$(cat <<JSON
{
  "capability_id": "delivery.query",
  "params": {
    "query": "$DELIVERY_QUERY",
    "service": "$SERVICE",
    "limit": 5
  },
  "operator_reason": "live delivery validation"
}
JSON
)"
delivery_response="$(call_json POST "/api/v1/connectors/$DELIVERY_CONNECTOR/capabilities/invoke" "$delivery_payload")"
printf '%s' "$delivery_response" | python3 -c 'import json,sys; data=json.load(sys.stdin); artifacts=data.get("artifacts") or []; output=data.get("output") or {}; print("delivery_status=", data.get("status") or ""); print("delivery_count=", output.get("result_count") or 0); print("delivery_artifacts=", len(artifacts)); print("delivery_source=", output.get("source") or ""); sys.exit(0 if data.get("status")=="completed" and int(output.get("result_count") or 0) > 0 and len(artifacts) > 0 else 1)'
fi

if [ "$RUN_SMOKE" != "1" ]; then
  echo "smoke=skipped"
  exit 0
fi

echo "-- smoke tool-plan scenarios --"
BASE_URL="$BASE_URL" \
TOKEN="$TOKEN" \
SERVICE="$SERVICE" \
SMOKE_HOST="$SMOKE_HOST" \
SMOKE_ALERTNAME="$SMOKE_ALERTNAME" \
SMOKE_SUMMARY="$SMOKE_SUMMARY" \
SMOKE_LOGS_SUMMARY="$SMOKE_LOGS_SUMMARY" \
SMOKE_OBSERVABILITY_SUMMARY="$SMOKE_OBSERVABILITY_SUMMARY" \
SMOKE_DELIVERY_SUMMARY="$SMOKE_DELIVERY_SUMMARY" \
SMOKE_SCENARIOS="$SMOKE_SCENARIOS" \
POLL_SECONDS="$POLL_SECONDS" \
VALIDATOR_PATH="$VALIDATOR_PATH" \
python3 - <<'PY'
import importlib.util
import json
import os
import sys
import time
import urllib.error
import urllib.request
from http.client import RemoteDisconnected

base_url = os.environ["BASE_URL"]
token = os.environ["TOKEN"]
service = os.environ["SERVICE"]
smoke_host = os.environ["SMOKE_HOST"]
alertname = os.environ["SMOKE_ALERTNAME"]
poll_seconds = int(os.environ["POLL_SECONDS"])
smoke_scenarios = (os.environ.get("SMOKE_SCENARIOS") or "single").strip() or "single"
validator_path = os.environ["VALIDATOR_PATH"]

spec = importlib.util.spec_from_file_location("tool_plan_smoke_validator", validator_path)
validator = importlib.util.module_from_spec(spec)
spec.loader.exec_module(validator)

headers = {
    "Authorization": f"Bearer {token}",
    "Content-Type": "application/json",
}

scenario_defs = {
    "metrics": {
        "summary": os.environ["SMOKE_SUMMARY"],
    },
    "logs": {
        "summary": os.environ["SMOKE_LOGS_SUMMARY"],
    },
    "observability": {
        "summary": os.environ["SMOKE_OBSERVABILITY_SUMMARY"],
    },
    "delivery": {
        "summary": os.environ["SMOKE_DELIVERY_SUMMARY"],
    },
}

scenarios = [item.strip() for item in smoke_scenarios.split(",") if item.strip()]
unknown = [item for item in scenarios if item not in scenario_defs]
if unknown:
    raise SystemExit(f"unsupported smoke scenarios: {','.join(unknown)}")


def api_request(path, *, method="GET", payload=None):
    data = None
    if payload is not None:
        data = json.dumps(payload).encode()
    req = urllib.request.Request(base_url + path, data=data, headers=headers, method=method)
    return json.loads(urllib.request.urlopen(req, timeout=30).read().decode())


def fetch_session(session_id):
    last_error = None
    for _ in range(3):
        try:
            return api_request(f"/api/v1/sessions/{session_id}")
        except (urllib.error.URLError, RemoteDisconnected) as err:
            last_error = err
            time.sleep(1)
    raise SystemExit(f"failed to fetch session {session_id}: {last_error}")


def wait_for_session(session_id):
    deadline = time.time() + poll_seconds
    last_detail = None
    while time.time() < deadline:
        detail = fetch_session(session_id)
        last_detail = detail
        steps = detail.get("tool_plan") or []
        terminal = {"completed", "failed", "skipped", "pending_approval"}
        if steps and detail.get("status") in {"resolved", "open", "failed"} and all((step or {}).get("status") in terminal for step in steps):
            return detail
        time.sleep(2)
    raise SystemExit(f"timed out waiting for evidence-first tool plan output for {session_id}: {json.dumps(last_detail or {}, ensure_ascii=False)}")
for name in scenarios:
    payload = {
        "alertname": alertname,
        "service": service,
        "host": smoke_host,
        "severity": "warning",
        "summary": scenario_defs[name]["summary"],
    }
    created = api_request("/api/v1/smoke/alerts", method="POST", payload=payload)
    session_id = created["session_id"]
    detail = wait_for_session(session_id)
    validator.validate_scenario(name, detail)
    print(
        "scenario=%s session_id=%s status=%s attachments=%s executions=%s tools=%s summary=%s"
        % (
            name,
            session_id,
            detail.get("status") or "",
            ",".join((item or {}).get("name") or "" for item in (detail.get("attachments") or [])),
            len(detail.get("executions") or []),
            ",".join((step or {}).get("tool") or "" for step in (detail.get("tool_plan") or [])),
            (detail.get("diagnosis_summary") or "").strip(),
        )
    )
PY
