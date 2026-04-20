# TARS 发布检查清单

## 1. 发布前

- [ ] 当前分支 `go test ./...` 通过
- [ ] 当前分支 `go build ./...` 通过
- [ ] [0001_init.sql](../../migrations/postgres/0001_init.sql) 已在目标库执行
- [ ] `TARS_POSTGRES_DSN` 已验证可连通
- [ ] `TARS_OPS_API_TOKEN` 已设置，且不为空
- [ ] Telegram token / model API key / SSH 私钥均通过运行时 secret 注入
- [ ] 已按 [deploy/pilot/README.md](../../deploy/pilot/README.md) 准备试点样例包和 golden cases
- [ ] `TARS_TELEGRAM_POLLING_ENABLED` 模式已确认
- [ ] 试点 `chat_id` 已验证
- [ ] `TARS_APPROVALS_CONFIG_PATH` 对应文件存在且格式正确
- [ ] 目标主机已加入 `TARS_SSH_ALLOWED_HOSTS`
- [ ] `TARS_EXECUTION_OUTPUT_SPOOL_DIR` 目录存在且可写
- [ ] 当前 rollout mode 已确认
- [ ] 没有旧版本 TARS 进程继续连接同一套 Postgres / outbox

## 2. 发布时

- [ ] 先记录当前 rollout mode 和 feature flags
- [ ] 启动或重启 TARS 进程
- [ ] 确认统一入口监听在 `TARS_SERVER_LISTEN`
- [ ] 确认 `TARS_OPS_API_ENABLED=true` 时，Ops API 可在统一入口上鉴权访问
- [ ] `/healthz` 返回 200
- [ ] `/readyz` 返回 200
- [ ] Ops API `/api/v1/sessions` 可鉴权访问

## 3. 发布后 10 分钟内

- [ ] 至少触发 1 条测试告警
- [ ] 确认生成 session
- [ ] 确认 Telegram diagnosis 或审批消息送达
- [ ] 如果当前允许执行，确认 `批准执行` 后能收到结果消息
- [ ] 检查没有异常增长的 failed/blocked outbox
- [ ] 检查 spool 目录可写且产生文件

## 4. 回滚条件

满足任一条件就优先按 feature flag / rollout mode 回滚：

- [ ] Telegram 消息持续无法送达
- [ ] callback 大面积卡住或按钮无响应
- [ ] SSH 执行出现明显误触发风险
- [ ] 模型持续超时导致 diagnosis 明显不可用
- [ ] outbox `failed/blocked` 快速堆积

## 5. 回滚动作

### 5.1 执行链回滚

- [ ] 切 `TARS_ROLLOUT_MODE=approval_beta`
- [ ] 重启服务
- [ ] 复测 diagnosis 和审批消息

### 5.2 审批链回滚

- [ ] 切 `TARS_ROLLOUT_MODE=diagnosis_only`
- [ ] 重启服务
- [ ] 确认不再创建新的 `pending_approval`

### 5.3 知识链回滚

- [ ] 保持当前 rollout mode
- [ ] 设置 `TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED=false`
- [ ] 重启服务
- [ ] 通过 Ops API 检查 `session.closed` 是否转为 `blocked`

## 6. 发布结束后记录

- [ ] 记录本次 rollout mode
- [ ] 记录 smoke session_id / execution_id
- [ ] 记录是否使用真实模型
- [ ] 记录是否触发回滚或降级
- [ ] 更新 [tars_dev_tracker.md](../../project/tars_dev_tracker.md)
