# TARS 试点 Runbook

> 适用范围：后端 MVP 试点环境  
> 目标：让值班 / 平台同学按固定步骤完成部署、冒烟、灰度、回滚和问题定位

## 1. 试点前提

- 目标环境中只保留**一个**会消费同一套 Postgres / outbox 的 TARS 实例；如有旧版本实例，先停掉
- PostgreSQL 已可用，且 [0001_init.sql](../../migrations/postgres/0001_init.sql) 已执行
- Telegram bot 已创建，机器人 token 已通过运行时 secret 注入
- 试点 `chat_id` 已确认可接收机器人消息
- SSH 私钥已放置到目标环境，且目标主机在 `TARS_SSH_ALLOWED_HOSTS` 白名单内
- `VictoriaMetrics + vmalert` 已就绪，或已按 [vm-smoke-compose.yml](../../deploy/docker/vm-smoke-compose.yml) 拉起 smoke 环境
- 模型网关可从 TARS 运行节点访问

## 2. 推荐发布顺序

### 2.1 Phase A：只诊断

```sh
TARS_ROLLOUT_MODE=diagnosis_only
TARS_TELEGRAM_POLLING_ENABLED=true
```

验证目标：

- vmalert 能创建 session
- Telegram 能收到 diagnosis 消息
- 不会创建 `pending_approval` execution

### 2.2 Phase B：审批 Beta

```sh
TARS_ROLLOUT_MODE=approval_beta
```

验证目标：

- Telegram 能收到自动审批消息
- 点击 `批准执行 / 拒绝 / 要求补充信息` 能立即结束 loading
- `pending_approval` 超时会自动拒绝

### 2.3 Phase C：执行 Beta

```sh
TARS_ROLLOUT_MODE=execution_beta
```

验证目标：

- `批准执行` 后会进入 `executing`
- SSH 命令结果能回传 Telegram
- `output_ref` 对应 spool file 实际存在

### 2.4 Phase D：知识沉淀

```sh
TARS_ROLLOUT_MODE=knowledge_on
```

验证目标：

- `resolved` session 会写入 `documents / document_chunks / knowledge_records`
- 后续相近告警的 diagnosis 消息可带 `knowledge:` 引用

## 3. 启动步骤

1. 准备环境变量  
   参考 [deploy/README.md](../../deploy/README.md)
2. 执行数据库初始化

```sh
export DATABASE_URL="$TARS_POSTGRES_DSN"
./scripts/init_db.sh
```

3. 启动主服务

```sh
./tars
```

4. 健康检查

```sh
curl -fsS "http://127.0.0.1:8081/healthz"
curl -fsS "http://127.0.0.1:8081/readyz"
curl -fsS -H "Authorization: Bearer ${TARS_OPS_API_TOKEN}" "http://127.0.0.1:8081/api/v1/sessions"
```

## 4. 冒烟步骤

### 4.0.1 演示前环境卫生检查

如果目标是对外演示或正式试点验收，建议先跑一次：

```sh
TARS_SERVER_BASE_URL=http://127.0.0.1:8081 \
TARS_OPS_BASE_URL=http://127.0.0.1:8081 \
TARS_OPS_API_TOKEN="${TARS_OPS_API_TOKEN}" \
./scripts/pilot_hygiene_check.sh
```

如果脚本输出 `failed_outbox > 0` 或 `result=attention_needed`，优先确认这些失败是否为历史样本，再开始正式演示。

- 需要保留的失败样本：先记录 aggregate_id / error，再决定是否 replay
- 明确属于历史残留、且不准备 replay 的样本：可以直接在 Web 的 `/outbox` 页面点击 `Delete` 清理

### 4.0 Web 体验入口

- 如果 Ops/Web 入口已开放到局域网，直接打开：

```text
http://<tars-host>:8081/login
```

- 使用当前环境的 `TARS_OPS_API_TOKEN` 登录
- 初始化完成后进入 `Runtime Checks`
- 先确认 Telegram / Model / VictoriaMetrics / SSH 都显示为 `configured`
- 再触发一条 smoke alert，观察：
  - `latest smoke` 卡片是否更新
  - Session 是否被打上 `SMOKE` 标记
  - Telegram 是否收到 diagnosis / 审批消息

### 4.1 vmalert 路径

- 启动 [vm-smoke-compose.yml](../../deploy/docker/vm-smoke-compose.yml)
- 等待 `TarsSmokeNodeUp` 或自定义低阈值规则触发
- 确认：
  - Session 状态从 `analyzing/open` 进入 `pending_approval` 或 `resolved`
  - Telegram 收到 diagnosis 或审批消息

### 4.2 Telegram 审批路径

- 在 Telegram 点击 `批准执行`
- 确认：
  - 按钮不持续 loading
  - 先收到 “starting command” 消息
  - 再收到执行结果消息

补充验收样本：

- 2026-03-19 已在 `192.168.3.106` 完成一条走正常 workflow 的 JumpServer 审批样本：
  - Session: `6e190d44-d6c2-496a-bb26-8999a3cfda97`
  - Execution: `4d95b605-9003-4da4-80e1-3c78521d2965`
  - Connector: `jumpserver-main`
  - 最终状态：`resolved`
  - Verification：`success`
- 这条样本不是直接调控制面 `execution/execute`，而是完整经过 `Telegram 对话 -> diagnosis -> pending_approval -> approve -> JumpServer execute -> verify -> resolved`

### 4.2.1 Telegram 对话路径

如果要验证“值班同学直接对机器人提问”，可以走下面这条路径：

1. 在 Telegram 里直接给机器人发自然语言请求，例如：
   - `看系统负载`
   - `host=192.168.3.106 看系统负载`
   - `看一下 sshd 状态`
   - `看一下你的出口IP是多少`
2. 确认机器人先返回一条 `[TARS 对话]` 确认消息
3. 等待后续诊断消息和审批消息到达
4. 点击 `批准执行`
5. 确认：
   - Session 进入 `pending_approval -> executing -> resolved`
   - 审批消息和结果消息仍回到同一个聊天窗口
   - 如果是多主机白名单而用户未指定 `host`，机器人只返回引导信息，不创建 session

### 4.3 SSH 路径

- 通过 Ops API 查询 execution

```sh
curl -fsS \
  -H "Authorization: Bearer ${TARS_OPS_API_TOKEN}" \
  "http://127.0.0.1:8081/api/v1/executions/<execution_id>"
```

- 确认：
  - `status=completed`
  - `output_ref` 非空
  - spool 文件存在

### 4.4 Setup -> Runtime Checks 直通路径

如果目标是验证“用户第一次打开平台就能体验价值”，建议优先走这条路径，而不是先手工发 webhook：

1. 未初始化时先打开 `/setup` 完成首次安装
2. 初始化完成并登录后进入 `Runtime Checks`
3. 触发一条 smoke alert，例如：
   - `alertname=TarsSmokeManual`
   - `service=sshd`
   - `host=192.168.3.106`
   - `severity=critical`
4. 在 Telegram 中批准执行
5. 回到 Web 确认：
   - Session 状态进入 `resolved`
   - `verification.status=success`
   - Execution Detail 可看到输出

这条路径在 2026-03-12 已在 `192.168.3.106:8081` 实测通过。

### 4.5 正式演示脚本

如果需要一键触发并跟踪整条 smoke 演示链路，可以直接运行：

```sh
TARS_OPS_BASE_URL=http://127.0.0.1:8081 \
TARS_OPS_API_TOKEN="${TARS_OPS_API_TOKEN}" \
TARS_DEMO_ALERTNAME=TarsDemoManual \
TARS_DEMO_SERVICE=sshd \
TARS_DEMO_HOST=192.168.3.106 \
./scripts/run_demo_smoke.sh
```

配套说明见 [demo_acceptance_script.md](demo_acceptance_script.md)。

## 5. 常见问题与处置

### 5.1 没有收到 Telegram 自动审批消息

排查顺序：

1. 确认 `TARS_TELEGRAM_POLLING_ENABLED=true`
2. 确认机器人已和目标 `chat_id` 发生过对话
3. 通过 Ops API 查看 session timeline 是否有 `approval_route_selected`
4. 检查日志里是否有 Telegram send error

临时绕过：

- 先用手工补发审批消息验证 callback / execution 链路

### 5.2 批准后没有进入执行

排查顺序：

1. 确认当前不是 `diagnosis_only` 或 `approval_beta`
2. 确认 `TARS_FEATURES_EXECUTION_ENABLED=true`
3. 确认目标主机在 `TARS_SSH_ALLOWED_HOSTS`
4. 确认命令没有命中 blocked fragments

### 5.3 执行输出没有落盘

排查顺序：

1. 检查 `TARS_EXECUTION_OUTPUT_SPOOL_DIR` 是否存在且可写
2. 检查服务进程用户是否有目录权限
3. 通过 Ops API 检查 `output_ref` 是否为空

### 5.4 知识沉淀未发生

排查顺序：

1. 确认当前 rollout mode 或 feature flag 已开启 knowledge ingest
2. 确认 session 已进入 `resolved`
3. 检查 outbox 是否存在 `session.closed` 且被 `blocked`
4. 必要时用 Ops API 手动 replay outbox

### 5.5 rollout mode 看起来没生效

排查顺序：

1. 检查当前进程环境里的 `TARS_ROLLOUT_MODE` 和 `TARS_FEATURES_*`
2. 确认没有旧版 TARS 实例仍连接同一套 Postgres / outbox
3. 确认重启后只有当前目标实例在监听主入口和 Ops 入口
4. 再触发一条新的 smoke 告警验证，不要复用旧 session

## 6. 试点期间建议观察项

- 新增 session 数
- `pending_approval` 数
- auto-reject 超时数
- execution `failed/timeout` 数
- blocked outbox 数
- spool 目录大小增长

## 7. 快速回滚

优先按开关回滚，不先动数据库：

1. 执行风险上升：切到

```sh
TARS_ROLLOUT_MODE=approval_beta
```

2. 审批链异常：切到

```sh
TARS_ROLLOUT_MODE=diagnosis_only
```

3. 知识链异常但主链路正常：

```sh
TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED=false
```

4. 重新启动 TARS 进程
5. 复测 `/healthz`、Telegram diagnosis 消息、Ops API
