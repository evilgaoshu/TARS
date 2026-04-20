#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/scripts/lib/shared_ops_token.sh"

BASE_URL="${TARS_OPS_BASE_URL:-http://127.0.0.1:8081}"
TOKEN="$(shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}" 2>/dev/null || true)"
VM_MAIN_ID="${TARS_VALIDATE_VM_MAIN_ID:-victoriametrics-main}"
VL_MAIN_ID="${TARS_VALIDATE_VL_MAIN_ID:-victorialogs-main}"
VM_BASE_URL="${TARS_VALIDATE_VM_BASE_URL:-http://127.0.0.1:8428}"
VL_BASE_URL="${TARS_VALIDATE_VL_BASE_URL:-http://127.0.0.1:9428}"
VM_QUERY="${TARS_VALIDATE_VM_QUERY:-up{job=\"node_3_100\"}}"
VL_QUERY="${TARS_VALIDATE_VL_QUERY:-tars-observability-host-file-test}"
VL_TIME_RANGE="${TARS_VALIDATE_VL_TIME_RANGE:-168h}"
VL_QUERY_RETRIES="${TARS_VALIDATE_VL_QUERY_RETRIES:-15}"
VL_QUERY_RETRY_DELAY="${TARS_VALIDATE_VL_QUERY_RETRY_DELAY:-1}"
VL_MARKER_REMOTE_HOST="${TARS_VALIDATE_VL_MARKER_REMOTE_HOST:-}"
VL_MARKER_REMOTE_USER="${TARS_VALIDATE_VL_MARKER_REMOTE_USER:-root}"
VL_MARKER_REMOTE_PATH="${TARS_VALIDATE_VL_MARKER_REMOTE_PATH:-/var/log/tars-observability-test.log}"
STAMP="$(date +%s)"
VM_TEMP_ID="${TARS_VALIDATE_VM_TEMP_ID:-victoriametrics-live-${STAMP}}"
VL_TEMP_ID="${TARS_VALIDATE_VL_TEMP_ID:-victorialogs-live-${STAMP}}"

if [[ -z "${TOKEN}" ]]; then
  TOKEN="$(shared_ops_token_resolve 2>/dev/null || true)"
fi

if [[ -z "${TOKEN}" ]]; then
  echo "TARS_OPS_API_TOKEN is required for observability connector validation" >&2
  echo "tip=可显式 export TARS_OPS_API_TOKEN，或设置 TARS_REMOTE_USER 让脚本从共享机 shared-test.env 自动解析。" >&2
  exit 1
fi

auth_header="Authorization: Bearer ${TOKEN}"

json_string() {
  python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$1"
}

call_json() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"

  if [[ -n "${payload}" ]]; then
    curl -fsS -H "${auth_header}" -H 'Content-Type: application/json' -X "${method}" "${BASE_URL}${path}" -d "${payload}"
    return
  fi

  curl -fsS -H "${auth_header}" -X "${method}" "${BASE_URL}${path}"
}

refresh_vl_marker_if_configured() {
  local marker_ts

  if [[ -z "${VL_MARKER_REMOTE_HOST}" ]]; then
    return 0
  fi

  marker_ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "-- refresh victorialogs host-file marker --"
  ssh -o BatchMode=yes -o ConnectTimeout=5 "${VL_MARKER_REMOTE_USER}@${VL_MARKER_REMOTE_HOST}" \
    "printf 'tars-observability-host-file-test host=${VL_MARKER_REMOTE_HOST} ts=%s\n' '${marker_ts}' >> '${VL_MARKER_REMOTE_PATH}'"
}

cleanup_temp_connectors() {
  local current payload

  current="$(call_json GET /api/v1/config/connectors)"
  payload="$(printf '%s' "${current}" | python3 -c 'import json,sys; data=json.load(sys.stdin); temp_ids={item.strip() for item in sys.argv[1:3] if item.strip()}; prefixes=tuple(item for item in sys.argv[3:] if item); config=data.get("config") or {}; entries=[entry for entry in (config.get("entries") or []) if ((entry.get("metadata") or {}).get("id") or "").strip() not in temp_ids and not ((entry.get("metadata") or {}).get("id") or "").strip().startswith(prefixes)]; print(json.dumps({"config":{"entries":entries},"operator_reason":"validate_observability_connectors_live.sh cleanup temporary observability connectors"}, ensure_ascii=False))' "${VM_TEMP_ID}" "${VL_TEMP_ID}" "victoriametrics-live-" "victorialogs-live-")"
  call_json PUT /api/v1/config/connectors "${payload}" >/dev/null
}

cleanup() {
  cleanup_temp_connectors >/dev/null 2>&1 || true
}

trap cleanup EXIT INT TERM

build_vm_manifest() {
  local connector_id="$1"
  local description="$2"

  cat <<JSON
{
  "api_version": "tars.connector/v1alpha1",
  "kind": "connector",
  "enabled": true,
  "metadata": {
    "id": "${connector_id}",
    "name": "victoriametrics",
    "display_name": "VictoriaMetrics Live Validate",
    "vendor": "victoriametrics",
    "version": "1.0.0",
    "description": "${description}"
  },
  "spec": {
    "type": "metrics",
    "protocol": "victoriametrics_http",
    "capabilities": [
      {
        "id": "query.instant",
        "action": "query",
        "read_only": true,
        "invocable": true,
        "scopes": ["metrics.read"]
      },
      {
        "id": "query.range",
        "action": "query",
        "read_only": true,
        "invocable": true,
        "scopes": ["metrics.read"]
      }
    ],
    "connection_form": [
      {
        "key": "base_url",
        "label": "Base URL",
        "type": "string",
        "required": true
      }
    ],
    "import_export": {
      "exportable": true,
      "importable": true,
      "formats": ["yaml", "json"]
    }
  },
  "config": {
    "values": {
      "base_url": "${VM_BASE_URL}"
    }
  },
  "compatibility": {
    "tars_major_versions": ["1"],
    "upstream_major_versions": ["1"],
    "modes": ["managed"]
  },
  "marketplace": {
    "category": "observability",
    "tags": ["metrics", "live-validate"],
    "source": "official"
  }
}
JSON
}

build_vl_manifest() {
  local connector_id="$1"
  local description="$2"

  cat <<JSON
{
  "api_version": "tars.connector/v1alpha1",
  "kind": "connector",
  "enabled": true,
  "metadata": {
    "id": "${connector_id}",
    "name": "victorialogs",
    "display_name": "VictoriaLogs Live Validate",
    "vendor": "victoriametrics",
    "version": "1.0.0",
    "description": "${description}"
  },
  "spec": {
    "type": "logs",
    "protocol": "victorialogs_http",
    "capabilities": [
      {
        "id": "logs.query",
        "action": "query",
        "read_only": true,
        "invocable": true,
        "scopes": ["logs.read"]
      },
      {
        "id": "victorialogs.query",
        "action": "query",
        "read_only": true,
        "invocable": true,
        "scopes": ["logs.read"]
      }
    ],
    "connection_form": [
      {
        "key": "base_url",
        "label": "Base URL",
        "type": "string",
        "required": true
      }
    ],
    "import_export": {
      "exportable": true,
      "importable": true,
      "formats": ["yaml", "json"]
    }
  },
  "config": {
    "values": {
      "base_url": "${VL_BASE_URL}"
    }
  },
  "compatibility": {
    "tars_major_versions": ["1"],
    "upstream_major_versions": ["1"],
    "modes": ["managed"]
  },
  "marketplace": {
    "category": "observability",
    "tags": ["logs", "live-validate"],
    "source": "official"
  }
}
JSON
}

probe_manifest() {
  local label="$1"
  local manifest="$2"
  local response

  echo "-- ${label} draft probe --"
  response="$(call_json POST /api/v1/connectors/probe "$(printf '{"manifest":%s}' "${manifest}")")"
  printf '%s' "${response}" | python3 -c 'import json,sys; label=sys.argv[1]; data=json.load(sys.stdin); health=data.get("health") or {}; compat=data.get("compatibility") or {}; status=(health.get("status") or "").strip(); summary=(health.get("summary") or "").strip(); compatible=bool(compat.get("compatible", False)); print("status=%s" % status); print("summary=%s" % summary); print("compatible=%s" % compatible); sys.exit(0 if status == "healthy" and compatible else "%s draft probe did not return healthy+compatible" % label)' "${label}"
}

create_connector() {
  local connector_id="$1"
  local manifest="$2"
  local response

  echo "-- create ${connector_id} --"
  response="$(call_json POST /api/v1/connectors "$(printf '{"manifest":%s,"operator_reason":"validate observability connector create"}' "${manifest}")")"
  printf '%s' "${response}" | python3 -c 'import json,sys; connector_id=sys.argv[1]; data=json.load(sys.stdin); metadata=data.get("metadata") or {}; enabled=data.get("enabled"); actual=(metadata.get("id") or "").strip(); print("connector_id=%s" % actual); print("enabled=%s" % enabled); sys.exit(0 if actual == connector_id else "create returned unexpected connector id for %s" % connector_id)' "${connector_id}"
}

update_connector() {
  local connector_id="$1"
  local manifest="$2"
  local response

  echo "-- update ${connector_id} --"
  response="$(call_json PUT "/api/v1/connectors/${connector_id}" "$(printf '{"manifest":%s,"operator_reason":"validate observability connector update"}' "${manifest}")")"
  printf '%s' "${response}" | python3 -c 'import json,sys; connector_id=sys.argv[1]; data=json.load(sys.stdin); metadata=data.get("metadata") or {}; actual=(metadata.get("id") or "").strip(); description=(metadata.get("description") or "").strip(); print("connector_id=%s" % actual); print("description=%s" % description); sys.exit(0 if actual == connector_id and "updated" in description else "update validation failed for %s" % connector_id)' "${connector_id}"
}

health_connector() {
  local connector_id="$1"
  local label="$2"
  local response

  echo "-- ${label} health --"
  response="$(call_json POST "/api/v1/connectors/${connector_id}/health")"
  printf '%s' "${response}" | python3 -c 'import json,sys; label=sys.argv[1]; data=json.load(sys.stdin); health=data.get("health") or {}; status=(health.get("status") or "").strip(); summary=(health.get("summary") or "").strip(); print("status=%s" % status); print("summary=%s" % summary); sys.exit(0 if status == "healthy" else "%s health check did not return healthy" % label)' "${label}"
}

query_vm() {
  local connector_id="$1"
  local label="$2"
  local response payload

  echo "-- ${label} metrics query --"
  payload="$(printf '{"query":%s,"mode":"range","window":"1h","step":"5m"}' "$(json_string "${VM_QUERY}")")"
  response="$(call_json POST "/api/v1/connectors/${connector_id}/metrics/query" "${payload}")"
  printf '%s' "${response}" | python3 -c 'import json,sys; label=sys.argv[1]; data=json.load(sys.stdin); runtime=data.get("runtime") or {}; series=data.get("series") or []; points=max((len((item or {}).get("values") or []) for item in series), default=0); first=series[0] if series else {}; print("runtime=%s" % ((runtime.get("connector_id") or runtime.get("protocol") or "").strip(),)); print("series_count=%s" % len(series)); print("points=%s" % points); print("first_job=%s" % ((first.get("job") or "").strip(),)) if first else None; print("first_host=%s" % ((first.get("host") or "").strip(),)) if first else None; sys.exit(0 if series and points > 0 else "%s metrics query returned no time-series points" % label)' "${label}"
}

query_vl() {
  local connector_id="$1"
  local label="$2"
  local response payload attempt total_attempts

  echo "-- ${label} logs query --"
  payload="$(printf '{"capability_id":"logs.query","params":{"query":%s,"limit":2,"time_range":%s}}' "$(json_string "${VL_QUERY}")" "$(json_string "${VL_TIME_RANGE}")")"
  total_attempts="${VL_QUERY_RETRIES}"
  if [[ "${total_attempts}" -lt 1 ]]; then
    total_attempts=1
  fi

  for attempt in $(seq 1 "${total_attempts}"); do
    response="$(call_json POST "/api/v1/connectors/${connector_id}/capabilities/invoke" "${payload}")"
    if printf '%s' "${response}" | python3 -c 'import json,sys; expected_base=sys.argv[1].rstrip("/"); data=json.load(sys.stdin); status=(data.get("status") or "").strip(); output=data.get("output") or {}; metadata=data.get("metadata") or {}; request_url=(output.get("request_url") or metadata.get("request_url") or "").strip(); result_count=int(output.get("result_count") or 0); ok = status == "completed" and result_count > 0 and (not expected_base or request_url.startswith(expected_base)); sys.exit(0 if ok else 1)' "${VL_BASE_URL}"; then
      break
    fi
    if [[ "${attempt}" -lt "${total_attempts}" ]]; then
      sleep "${VL_QUERY_RETRY_DELAY}"
    fi
  done

  printf '%s' "${response}" | python3 -c 'import json,sys; label=sys.argv[1]; expected_base=sys.argv[2].rstrip("/"); data=json.load(sys.stdin); status=(data.get("status") or "").strip(); output=data.get("output") or {}; metadata=data.get("metadata") or {}; request_url=(output.get("request_url") or metadata.get("request_url") or "").strip(); result_count=int(output.get("result_count") or 0); logs=output.get("logs") or []; print("status=%s" % status); print("result_count=%s" % result_count); print("request_url=%s" % request_url); print("first_msg=%s" % (((logs[0] or {}).get("_msg") or "").strip(),)) if logs else None; ok = status == "completed" and result_count > 0 and (not expected_base or request_url.startswith(expected_base)); sys.exit(0 if ok else "%s logs query validation failed" % label)' "${label}" "${VL_BASE_URL}"
}

echo "== TARS observability connector live validation =="
echo "base_url=${BASE_URL}"
echo "vm_main=${VM_MAIN_ID} vl_main=${VL_MAIN_ID} vm_query=${VM_QUERY} vl_query=${VL_QUERY}"
refresh_vl_marker_if_configured

health_connector "${VM_MAIN_ID}" "baseline victoriametrics-main"
query_vm "${VM_MAIN_ID}" "baseline victoriametrics-main"
health_connector "${VL_MAIN_ID}" "baseline victorialogs-main"
query_vl "${VL_MAIN_ID}" "baseline victorialogs-main"

vm_create_manifest="$(build_vm_manifest "${VM_TEMP_ID}" "temporary observability live validation connector")"
vm_update_manifest="$(build_vm_manifest "${VM_TEMP_ID}" "temporary observability live validation connector updated")"
vl_create_manifest="$(build_vl_manifest "${VL_TEMP_ID}" "temporary observability live validation connector")"
vl_update_manifest="$(build_vl_manifest "${VL_TEMP_ID}" "temporary observability live validation connector updated")"

probe_manifest "victoriametrics temp" "${vm_create_manifest}"
create_connector "${VM_TEMP_ID}" "${vm_create_manifest}"
update_connector "${VM_TEMP_ID}" "${vm_update_manifest}"
health_connector "${VM_TEMP_ID}" "temp victoriametrics"
query_vm "${VM_TEMP_ID}" "temp victoriametrics"

probe_manifest "victorialogs temp" "${vl_create_manifest}"
create_connector "${VL_TEMP_ID}" "${vl_create_manifest}"
update_connector "${VL_TEMP_ID}" "${vl_update_manifest}"
health_connector "${VL_TEMP_ID}" "temp victorialogs"
query_vl "${VL_TEMP_ID}" "temp victorialogs"

cleanup_temp_connectors

echo "cleanup=ok"
echo "observability-connectors-live-validate=passed"
