#!/usr/bin/env sh
set -eu

BASE_URL="${TARS_OPS_BASE_URL:-http://127.0.0.1:8081}"
TOKEN="${TARS_OPS_API_TOKEN:-}"
PREFIX="${TARS_VALIDATE_EXT_PREFIX:-live-extension}"
SUFFIX="${TARS_VALIDATE_EXT_SUFFIX:-$(date +%Y%m%d%H%M%S)}"
SKILL_ID="${TARS_VALIDATE_EXT_SKILL_ID:-${PREFIX}-${SUFFIX}}"
DISPLAY_NAME="${TARS_VALIDATE_EXT_DISPLAY_NAME:-Live Extension ${SUFFIX}}"
export SKILL_ID DISPLAY_NAME

if [ -z "$TOKEN" ]; then
  echo "TARS_OPS_API_TOKEN is required" >&2
  exit 1
fi

auth_header="Authorization: Bearer $TOKEN"

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

bundle_payload="$(python3 - <<'PY'
import json, os
skill_id = os.environ["SKILL_ID"]
display_name = os.environ["DISPLAY_NAME"]
payload = {
    "bundle": {
        "api_version": "tars.extension/v1alpha1",
        "kind": "skill_bundle",
        "metadata": {
            "id": skill_id,
            "display_name": display_name,
            "version": "1.0.0",
            "summary": "Live validation extension bundle",
            "source": "live-validation",
            "generated_by": "validate_extensions_live.sh",
        },
        "skill": {
            "api_version": "tars.skill/v1alpha1",
            "kind": "skill_package",
            "enabled": False,
            "metadata": {
                "id": skill_id,
                "name": skill_id,
                "display_name": display_name,
                "version": "1.0.0",
                "category": "incident-response",
                "vendor": "tars",
                "description": "Validate extension candidate persistence and import.",
                "source": "live-validation",
            },
            "spec": {
                "type": "incident_skill",
                "triggers": {
                    "alerts": ["LiveExtensionValidation"],
                    "intents": ["validate extension live"]
                },
                "planner": {
                    "summary": "Run a safe knowledge-first validation plan.",
                    "preferred_tools": ["knowledge.search"],
                    "steps": [
                        {
                            "id": "step_1",
                            "tool": "knowledge.search",
                            "required": True,
                            "reason": "Load the baseline validation guidance.",
                            "params": {"query": "extension live validation"}
                        }
                    ]
                },
                "outputs": {
                    "expected_summary_sections": ["Diagnosis", "Evidence", "Next Actions"]
                },
                "governance": {
                    "execution_policy": "approval_first",
                    "read_only_first": True
                }
            }
        },
        "docs": [
            {
                "id": skill_id,
                "slug": skill_id,
                "title": display_name,
                "format": "markdown",
                "summary": "Live extension validation doc",
                "content": "# Live Extension\n\nThis bundle validates generate/validate/review/import."
            }
        ],
        "tests": [
            {
                "id": "go-test",
                "name": "Go test baseline",
                "kind": "smoke",
                "command": "go test ./..."
            }
        ]
    },
    "operator_reason": "live extension candidate generation"
}
print(json.dumps(payload, ensure_ascii=False))
PY
)"

echo "== TARS extensions live validation =="
echo "base_url=$BASE_URL"
echo "skill_id=$SKILL_ID"

candidate_response="$(SKILL_ID="$SKILL_ID" DISPLAY_NAME="$DISPLAY_NAME" call_json POST /api/v1/extensions "$bundle_payload")"
candidate_id="$(printf '%s' "$candidate_response" | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"])')"
echo "candidate_id=$candidate_id"

call_json POST "/api/v1/extensions/$candidate_id/validate" >/dev/null
echo "validated=ok"

review_payload='{"review_state":"approved","operator_reason":"live extension approval"}'
review_response="$(call_json POST "/api/v1/extensions/$candidate_id/review" "$review_payload")"
review_state="$(printf '%s' "$review_response" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("review_state") or "")')"
echo "review_state=$review_state"

import_payload='{"operator_reason":"live extension import"}'
import_response="$(call_json POST "/api/v1/extensions/$candidate_id/import" "$import_payload")"
imported_skill_id="$(printf '%s' "$import_response" | python3 -c 'import sys,json; print((json.load(sys.stdin).get("manifest") or {}).get("metadata", {}).get("id", ""))')"
echo "imported_skill_id=$imported_skill_id"

skill_detail="$(call_json GET "/api/v1/skills/$SKILL_ID")"
printf '%s' "$skill_detail" | python3 -c 'import sys,json; data=json.load(sys.stdin); print("skill_version=", data.get("metadata",{}).get("version","")); print("skill_source=", data.get("metadata",{}).get("source","")); print("skill_status=", (data.get("lifecycle") or {}).get("status", ""))'

echo "validation_complete=ok"
