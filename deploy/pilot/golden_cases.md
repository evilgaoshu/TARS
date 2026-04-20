# TARS 试点 Golden Cases

这些用例用于试点验收，不追求覆盖所有功能，重点是确认价值链和风险边界。

## 官方黄金路径 v1

- 官方主链定义：`VMAlert / 告警进入 -> Session 建立 -> 证据查询与聚合 -> 诊断结论 -> 审批 / 执行 -> 结果回传 -> Inbox / Telegram 通知 -> 可回放 / 可验收`
- 官方回放入口：`bash scripts/run_golden_path_replay.sh`
- 官方样本：
  - `deploy/pilot/golden_path_alert_v1.json`
  - `deploy/pilot/golden_path_telegram_callback_v1.json`
- 官方验收关注点：
  - `/sessions` 和 `/executions` 列表能直接看到 headline / conclusion / risk / next action
  - `/sessions/:id` 能看到 `Golden Path Snapshot` 与 `Notification Reasons`
  - `/executions/:id` 能看到 `Execution Golden Path`
  - Session detail 返回 `golden_summary` 与 `notifications`
  - Execution detail 返回 `session_id` 与 `golden_summary`

## 1. Web Setup -> Runtime Checks

- 路径：`/setup -> /login -> /runtime-checks`
- 预期：
  - Telegram / Model / VM / SSH 显示为 `configured`
  - recent smoke 卡片可见
  - 触发 smoke 后生成新 session，并带 `SMOKE` 标记

## 2. VMAlert 到审批

- 输入：一条 `service=sshd` 的低阈值 vmalert 告警
- 预期：
  - 生成 session
  - Telegram 收到 diagnosis 或审批消息
  - Session timeline 出现 `diagnosis_requested / approval_message_prepared`

## 3. Telegram 对话：只读查询

- 输入：
  - `host=192.168.3.106 看系统负载`
  - `host=192.168.3.106 看一下 sshd 状态`
- 预期：
  - 生成 `telegram_chat` session
  - LLM 产生只读命令候选
  - 命中 `direct_execute` 或 `require_approval`
  - Telegram 返回结果，Session Trace 能看到对话与命令留痕

## 4. Telegram 对话：审批执行

- 输入：`host=192.168.3.106 重启 sshd`
- 预期：
  - 命令不直接执行
  - 进入 `pending_approval`
  - 点击批准后进入 `executing -> verifying -> resolved`

### 4A. 官方黄金路径回放

- 输入：`deploy/pilot/golden_path_alert_v1.json`
- 执行：
  - 手工审批：`TARS_OPS_API_TOKEN=... TARS_OPS_BASE_URL=http://127.0.0.1:8081 TARS_SERVER_BASE_URL=http://127.0.0.1:8081 bash scripts/run_golden_path_replay.sh`
  - 自动审批：在上面基础上增加 `TARS_GOLDEN_AUTO_APPROVE=1`
- 预期：
  - 脚本输出 `headline / conclusion / next_action / snapshot`
  - 会话进入 `pending_approval` 或 `resolved`
  - Session detail 的 `notifications` 至少说明“发送诊断结论 / 请求人工审批”
  - 自动审批开启后，最终应进入 `resolved` 或明确 `failed`，不得卡在无解释的中间态

## 5. Provider Failover

- 操作：临时让 primary provider 不可达
- 预期：
  - `providers/check` 对 assist 仍可用
  - 实际 diagnosis 触发 `chat_completions_failover`
  - session 仍能走通，不因 primary 超时而中断

## 6. 脱敏与审计

- 输入：包含 `token=...`、路径、IP 的请求
- 预期：
  - Audit Trail 可同时看到 `RAW / SENT`
  - `request_sent` 中 secret 已变为 `[REDACTED]`
  - `HOST / IP / PATH` 允许有限回填
  - secret 不会在 summary / execution hint 中回填

## 7. 知识沉淀

- 条件：至少完成一条 `resolved` session
- 预期：
  - 生成 `documents / document_chunks / knowledge_records`
  - Session Detail -> Trace 可看到 `Knowledge Trace`
  - 相似后续请求出现 `knowledge:` 引用
