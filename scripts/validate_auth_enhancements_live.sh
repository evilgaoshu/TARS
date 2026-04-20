#!/usr/bin/env sh
set -eu

BASE_URL="${TARS_OPS_BASE_URL:-http://127.0.0.1:8081}"
TOKEN="${TARS_OPS_API_TOKEN:-}"
USERNAME="${TARS_VALIDATE_AUTH_USERNAME:-shared-admin}"
PASSWORD="${TARS_VALIDATE_AUTH_PASSWORD:-}"
TOTP_SECRET="${TARS_VALIDATE_AUTH_TOTP_SECRET:-}"

if [ -z "$TOKEN" ]; then
  echo "TARS_OPS_API_TOKEN is required" >&2
  exit 1
fi

if [ -z "$PASSWORD" ]; then
  echo "TARS_VALIDATE_AUTH_PASSWORD is required" >&2
  exit 1
fi

if [ -z "$TOTP_SECRET" ]; then
  echo "TARS_VALIDATE_AUTH_TOTP_SECRET is required" >&2
  exit 1
fi

auth_header="Authorization: Bearer $TOKEN"

call_json() {
  method="$1"
  path="$2"
  payload="${3:-}"
  if [ -n "$payload" ]; then
    curl -fsS -H 'Content-Type: application/json' -X "$method" "$BASE_URL$path" -d "$payload"
  else
    curl -fsS -X "$method" "$BASE_URL$path"
  fi
}

call_auth_json() {
  method="$1"
  path="$2"
  curl -fsS -H "$auth_header" -X "$method" "$BASE_URL$path"
}

totp_code="$(TOTP_SECRET="$TOTP_SECRET" python3 - <<'PY'
import base64, hashlib, hmac, os, struct, time
secret = os.environ['TOTP_SECRET'].strip().upper()
key = base64.b32decode(secret + '=' * ((8 - len(secret) % 8) % 8))
counter = int(time.time()) // 30
msg = struct.pack('>Q', counter)
digest = hmac.new(key, msg, hashlib.sha1).digest()
offset = digest[-1] & 0x0F
code = (struct.unpack('>I', digest[offset:offset+4])[0] & 0x7fffffff) % 1000000
print(f"{code:06d}")
PY
)"

echo "== auth enhancements live validation =="
echo "base_url=$BASE_URL"

login_payload="$(USERNAME="$USERNAME" PASSWORD="$PASSWORD" python3 - <<'PY'
import json, os
print(json.dumps({
    "provider_id": "local_password",
    "username": os.environ["USERNAME"],
    "password": os.environ["PASSWORD"],
}))
PY
)"
login_response="$(call_json POST /api/v1/auth/login "$login_payload")"
pending_token="$(printf '%s' "$login_response" | python3 -c 'import sys,json; data=json.load(sys.stdin); print(data.get("pending_token", ""))')"
challenge_id="$(printf '%s' "$login_response" | python3 -c 'import sys,json; data=json.load(sys.stdin); print(data.get("challenge_id", ""))')"
challenge_code="$(printf '%s' "$login_response" | python3 -c 'import sys,json; data=json.load(sys.stdin); print(data.get("challenge_code", ""))')"
next_step="$(printf '%s' "$login_response" | python3 -c 'import sys,json; data=json.load(sys.stdin); print(data.get("next_step", ""))')"
echo "login_next_step=$next_step"

verify_payload="$(PENDING_TOKEN="$pending_token" CHALLENGE_ID="$challenge_id" CHALLENGE_CODE="$challenge_code" python3 - <<'PY'
import json, os
print(json.dumps({
    "pending_token": os.environ["PENDING_TOKEN"],
    "challenge_id": os.environ["CHALLENGE_ID"],
    "code": os.environ["CHALLENGE_CODE"],
}))
PY
)"
verify_response="$(PENDING_TOKEN="$pending_token" CHALLENGE_ID="$challenge_id" CHALLENGE_CODE="$challenge_code" call_json POST /api/v1/auth/verify "$verify_payload")"
pending_token_2="$(printf '%s' "$verify_response" | python3 -c 'import sys,json; data=json.load(sys.stdin); print(data.get("pending_token", ""))')"
verify_next_step="$(printf '%s' "$verify_response" | python3 -c 'import sys,json; data=json.load(sys.stdin); print(data.get("next_step", ""))')"
echo "verify_next_step=$verify_next_step"

mfa_payload="$(PENDING_TOKEN="$pending_token_2" TOTP_CODE="$totp_code" python3 - <<'PY'
import json, os
print(json.dumps({
    "pending_token": os.environ["PENDING_TOKEN"],
    "code": os.environ["TOTP_CODE"],
}))
PY
)"
mfa_response="$(PENDING_TOKEN="$pending_token_2" TOTP_CODE="$totp_code" call_json POST /api/v1/auth/mfa/verify "$mfa_payload")"
session_token="$(printf '%s' "$mfa_response" | python3 -c 'import sys,json; data=json.load(sys.stdin); print(data.get("session_token", ""))')"
echo "session_issued=$( [ -n "$session_token" ] && printf yes || printf no )"

me_response="$(curl -fsS -H "Authorization: Bearer $session_token" "$BASE_URL/api/v1/me")"
echo "me_user_id=$(printf '%s' "$me_response" | python3 -c 'import sys,json; data=json.load(sys.stdin); print(data.get("user", {}).get("user_id", ""))')"

call_auth_json GET /api/v1/me >/dev/null
echo "ops_token_fallback=ok"

curl -fsS -H "Authorization: Bearer $session_token" -H 'Content-Type: application/json' -X POST "$BASE_URL/api/v1/auth/logout" -d '{}' >/dev/null
echo "logout=ok"
