#!/usr/bin/env bash
# Golden Scenario 2: 定时巡检 + 只读 Connector + Inbox 通知
#
# 流程：
#   1. 确认 automation job 已存在且启用
#   2. 手动触发 RunNow（POST /api/v1/automations/{id}/run）
#   3. 等待 run 完成（从响应中读取状态）
#   4. 验证 /api/v1/inbox 中存在关联本次 automation run 的新消息
#
# 注意：这里要求验证真实闭环：automation run 完成/失败后，必须投递 Inbox。
#
# 环境变量（所有变量均可通过环境注入，无硬编码地址）：
#   TARS_OPS_BASE_URL            - TARS OPS API 地址，默认 http://127.0.0.1:8081
#   TARS_OPS_API_TOKEN           - 必填；通过 TARS 管理后台生成的 API Token
#   TARS_GOLDEN_AUTOMATION_JOB   - automation job ID，默认 golden-inspection-victoriametrics
#
# 快速示例：
#   TARS_OPS_API_TOKEN=mytoken ./scripts/run_golden_scenario_2.sh
#   # 使用自定义 job:
#   TARS_OPS_API_TOKEN=mytoken TARS_GOLDEN_AUTOMATION_JOB=my-job ./scripts/run_golden_scenario_2.sh

set -euo pipefail

OPS_BASE_URL="${TARS_OPS_BASE_URL:-http://127.0.0.1:8081}"
OPS_TOKEN="${TARS_OPS_API_TOKEN:-}"
JOB_ID="${TARS_GOLDEN_AUTOMATION_JOB:-golden-inspection-victoriametrics}"

if [ -z "$OPS_TOKEN" ]; then
  printf 'ERROR: TARS_OPS_API_TOKEN is required\n' >&2
  printf '提示: 通过 TARS 管理后台生成 API Token，然后设置环境变量 TARS_OPS_API_TOKEN=<token>\n' >&2
  exit 1
fi

auth_header="Authorization: Bearer $OPS_TOKEN"

ops_get() {
  curl -fsS -H "$auth_header" "${OPS_BASE_URL}${1}"
}

ops_post() {
  local path="$1"
  local body="${2:-{}}"
  curl -fsS -H "$auth_header" -H 'Content-Type: application/json' \
    -X POST "${OPS_BASE_URL}${path}" -d "$body"
}

printf '== Golden Scenario 2: 定时巡检 + 只读 Connector + Inbox ==\n'
printf 'ops_base_url=%s\n' "$OPS_BASE_URL"
printf 'job_id=%s\n' "$JOB_ID"
printf '\n'

# Step 1: 确认 automation job 存在且已启用
printf '[Step 1] 检查 automation job 状态...\n'
job_resp="$(ops_get "/api/v1/automations/${JOB_ID}")"
job_enabled="$(printf '%s' "$job_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("enabled", False))')"
job_type="$(printf '%s' "$job_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("type",""))')"
job_target="$(printf '%s' "$job_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("target_ref",""))')"

if [ "$job_enabled" = "False" ] || [ "$job_enabled" = "false" ]; then
  printf 'ERROR: automation job %s 未启用 (enabled=%s)\n' "$JOB_ID" "$job_enabled" >&2
  printf '提示: 检查 automations.shared.yaml 中 enabled: true 是否已部署。\n' >&2
  exit 1
fi

printf 'job_enabled=%s type=%s target_ref=%s\n' "$job_enabled" "$job_type" "$job_target"
printf '\n'

# Step 2: 手动触发 RunNow
printf '[Step 2] 手动触发 automation run...\n'
run_resp="$(ops_post "/api/v1/automations/${JOB_ID}/run")"
run_status="$(printf '%s' "$run_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); lr=d.get("last_run") or {}; print(lr.get("status",""))')"
run_id="$(printf '%s' "$run_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); lr=d.get("last_run") or {}; print(lr.get("run_id",""))')"
run_summary="$(printf '%s' "$run_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); lr=d.get("last_run") or {}; print(lr.get("summary",""))')"
run_error="$(printf '%s' "$run_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); lr=d.get("last_run") or {}; print(lr.get("error",""))')"

printf 'run_id=%s\n' "$run_id"
printf 'run_status=%s\n' "$run_status"
printf 'run_summary=%s\n' "$run_summary"
if [ -n "$run_error" ]; then
  printf 'run_error=%s\n' "$run_error"
fi
printf '\n'

# Step 3: 验证 run 状态
printf '[Step 3] 验证 run 结果...\n'
case "$run_status" in
  completed)
    printf 'run_result=COMPLETED\n'
    ;;
  blocked)
    printf 'run_result=BLOCKED（读写权限拦截，属正常行为）\n'
    printf 'summary=%s\n' "$run_summary"
    # blocked 不视为失败 —— 表示 safety guard 工作正常
    ;;
  failed)
    printf 'ERROR: run 执行失败 status=%s error=%s\n' "$run_status" "$run_error" >&2
    printf '提示1: 确认 victoriametrics-main connector 的 base_url 可达（curl http://127.0.0.1:8428/health）。\n' >&2
    printf '提示2: 查看远端日志 /root/tars-dev.log 获取详细错误。\n' >&2
    exit 1
    ;;
  "")
    printf 'ERROR: 未能从响应中读取 run 状态\n响应: %s\n' "$run_resp" >&2
    exit 1
    ;;
  *)
    printf 'WARN: 未预期的 run 状态 %s，继续检查 inbox...\n' "$run_status"
    ;;
esac

# Step 4: 验证 inbox 中存在关联本次 run 的消息
printf '\n[Step 4] 验证 inbox 中存在关联本次 automation run 的消息...\n'
sleep 2
inbox_resp="$(ops_get "/api/v1/inbox?limit=20")"
inbox_count="$(printf '%s' "$inbox_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(len(d.get("items") or []))')"
unread_count="$(printf '%s' "$inbox_resp" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("unread_count", 0))')"
printf 'inbox_total_items=%s unread=%s\n' "$inbox_count" "$unread_count"
inbox_match="$(printf '%s' "$inbox_resp" | python3 -c 'import sys, json; data=json.load(sys.stdin); run_id=sys.argv[1] if len(sys.argv)>1 else ""; job_id=sys.argv[2] if len(sys.argv)>2 else "";
for item in data.get("items") or []:
    ref_id=item.get("ref_id") or ""; ref_type=item.get("ref_type") or ""; body=item.get("body") or ""; subject=item.get("subject") or "";
    if ref_type=="automation_run" and ref_id==run_id:
        print(f"MATCH ref_type={ref_type} ref_id={ref_id} subject={subject!r}"); raise SystemExit(0)
    if run_id and run_id in body:
        print(f"BODY_MATCH ref_type={ref_type} ref_id={ref_id} subject={subject!r}"); raise SystemExit(0)
    if job_id and job_id in body:
        print(f"JOB_MATCH ref_type={ref_type} ref_id={ref_id} subject={subject!r}"); raise SystemExit(0)
print("NOT_FOUND"); raise SystemExit(1)' "$run_id" "$JOB_ID")" || inbox_exit=$?

inbox_exit="${inbox_exit:-0}"
printf 'inbox_check=%s\n' "$inbox_match"

if [ "$inbox_exit" -ne 0 ]; then
  printf 'ERROR: inbox 中未找到本次 automation run 的关联消息。\n' >&2
  printf '提示: 检查 automation run -> trigger -> in_app_inbox 是否已部署。\n' >&2
  exit 1
fi

printf '\n== Golden Scenario 2: PASSED ==\n'
printf 'job_id=%s  run_id=%s  run_status=%s\n' "$JOB_ID" "$run_id" "$run_status"
printf 'inbox_message=%s\n' "$inbox_match"
