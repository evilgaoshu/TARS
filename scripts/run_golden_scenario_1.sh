#!/usr/bin/env bash
# Golden Scenario 1: Web Chat + Bash(SSH) 低风险诊断 → Inbox
#
# 流程：
#   1. 通过 /api/v1/chat/messages 发送一条诊断请求
#   2. 轮询 /api/v1/sessions/{session_id} 等待 session 进入终态（resolved/failed）
#   3. 验证 /api/v1/inbox 中存在关联该 session 的 inbox 消息
#
# 环境变量（所有变量均可通过环境注入，无硬编码地址）：
#   TARS_OPS_BASE_URL   - TARS OPS API 地址，默认 http://127.0.0.1:8081
#   TARS_OPS_API_TOKEN  - 必填；通过 TARS 管理后台生成的 API Token
#   TARS_CHAT_HOST      - 诊断目标主机（必填，无默认值）；例如 localhost 或 192.168.x.x
#   TARS_CHAT_MESSAGE   - 可选，自定义诊断消息；默认附带时间戳避免复用历史 session
#   TARS_POLL_SECONDS   - 轮询超时，默认 180
#
# 快速示例：
#   TARS_OPS_API_TOKEN=mytoken TARS_CHAT_HOST=localhost ./scripts/run_golden_scenario_1.sh

set -euo pipefail

OPS_BASE_URL="${TARS_OPS_BASE_URL:-http://127.0.0.1:8081}"
OPS_TOKEN="${TARS_OPS_API_TOKEN:-}"
CHAT_HOST="${TARS_CHAT_HOST:-}"
CHAT_MESSAGE="${TARS_CHAT_MESSAGE:-检查系统负载和 CPU 使用情况 #$(date +%s)}"
POLL_SECONDS="${TARS_POLL_SECONDS:-180}"

if [ -z "$OPS_TOKEN" ]; then
  printf 'ERROR: TARS_OPS_API_TOKEN is required\n' >&2
  printf '提示: 通过 TARS 管理后台生成 API Token，然后设置环境变量 TARS_OPS_API_TOKEN=<token>\n' >&2
  exit 1
fi

if [ -z "$CHAT_HOST" ]; then
  printf 'ERROR: TARS_CHAT_HOST is required (诊断目标主机)\n' >&2
  printf '提示: 设置环境变量 TARS_CHAT_HOST=<hostname_or_ip>，例如 TARS_CHAT_HOST=localhost\n' >&2
  exit 1
fi

auth_header="Authorization: Bearer $OPS_TOKEN"

ops_get() {
  curl -fsS -H "$auth_header" "${OPS_BASE_URL}${1}"
}

ops_post() {
  local path="$1"
  local body="$2"
  curl -fsS -H "$auth_header" -H 'Content-Type: application/json' \
    -X POST "${OPS_BASE_URL}${path}" -d "$body"
}

printf '== Golden Scenario 1: Web Chat + SSH 低风险诊断 → Inbox ==\n'
printf 'ops_base_url=%s\n' "$OPS_BASE_URL"
printf 'chat_host=%s\n' "$CHAT_HOST"
printf 'chat_message=%s\n' "$CHAT_MESSAGE"
printf '\n'

# Step 1: 发送 web chat 消息
printf '[Step 1] 发送 web chat 诊断请求...\n'
chat_resp="$(ops_post "/api/v1/chat/messages" \
  "{\"message\": \"${CHAT_MESSAGE}\", \"host\": \"${CHAT_HOST}\", \"severity\": \"info\"}")"
session_id="$(printf '%s' "$chat_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("session_id",""))')"
if [ -z "$session_id" ]; then
  printf 'ERROR: 未能从响应中获取 session_id\n响应: %s\n' "$chat_resp" >&2
  exit 1
fi
printf 'session_id=%s\n' "$session_id"
printf 'session_url=%s/sessions/%s\n' "$OPS_BASE_URL" "$session_id"
printf '\n'

# Step 2: 轮询 session 直到终态
printf '[Step 2] 轮询 session 状态（超时 %ss）...\n' "$POLL_SECONDS"
deadline=$(( $(date +%s) + POLL_SECONDS ))
last_status=""
while [ "$(date +%s)" -lt "$deadline" ]; do
  detail="$(ops_get "/api/v1/sessions/${session_id}")"
  current_status="$(printf '%s' "$detail" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("status",""))')"
  if [ "$current_status" != "$last_status" ]; then
    printf 'session_status=%s\n' "$current_status"
    last_status="$current_status"
  fi
  case "$current_status" in
    resolved|failed|closed)
      printf 'session_finished status=%s\n' "$current_status"
      break
      ;;
  esac
  sleep 5
done

if [ "$last_status" != "resolved" ] && [ "$last_status" != "failed" ] && [ "$last_status" != "closed" ]; then
  printf 'WARN: session 未在 %ss 内进入终态，当前状态=%s，继续检查 inbox...\n' "$POLL_SECONDS" "$last_status"
fi

# Step 3: 验证 inbox 中存在关联该 session 的消息
printf '\n[Step 3] 验证 inbox 中存在关联 session 的消息...\n'
# 稍等片刻确保触发器已异步投递
sleep 3
inbox_resp="$(ops_get "/api/v1/inbox?limit=50")"
found="$(printf '%s' "$inbox_resp" | python3 -c 'import sys, json; data=json.load(sys.stdin); session_id=sys.argv[1] if len(sys.argv)>1 else ""; items=data.get("items") or [];
for item in items:
    ref_id=item.get("ref_id") or ""; body=item.get("body") or ""; subject=item.get("subject") or ""; ref_type=item.get("ref_type") or "";
    if ref_id==session_id or (session_id and session_id in body):
        print(f"MATCH ref_type={ref_type} ref_id={ref_id} subject={subject!r}"); raise SystemExit(0)
for item in items:
    ref_type=item.get("ref_type") or ""; subject=item.get("subject") or "";
    if ref_type=="session" or "会话" in subject:
        print(f"SESSION_TYPE_MATCH ref_type={ref_type} subject={subject!r}"); raise SystemExit(0)
print("NOT_FOUND total_items=" + str(len(items))); raise SystemExit(1)' "$session_id")" || inbox_exit=$?

inbox_exit="${inbox_exit:-0}"
printf 'inbox_check=%s\n' "$found"

if [ "${inbox_exit:-0}" -ne 0 ]; then
  printf '\nERROR: inbox 中未找到关联 session 的消息。\n'
  printf '提示: 确认触发器 trg_session_closed 已启用，且 channel=in_app_inbox。\n'
  printf '当前 inbox 消息数: %s\n' \
    "$(printf '%s' "$inbox_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(len(d.get("items") or []))')"
  exit 1
fi

printf '\n== Golden Scenario 1: PASSED ==\n'
printf 'session_id=%s  status=%s\n' "$session_id" "$last_status"
printf 'inbox_message=%s\n' "$found"
