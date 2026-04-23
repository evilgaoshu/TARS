# TARS MVP 完成清单

> 目标：把“已经做了很多”收敛成一份能用于试点 go/no-go 的正式清单。

## 1. MVP 范围定义

当前 MVP 的目标不是完整平台化配置中心，而是打通这条价值链：

`Web 登录 -> /runtime-checks -> 触发测试告警或 Telegram 对话 -> AI diagnosis -> 命令授权匹配 -> Telegram 审批 -> SSH 执行 -> verifier -> 结果回传 -> 审计与知识沉淀`

## 2. 已完成项

### 2.1 核心业务闭环

- [x] vmalert / webhook 可创建 session
- [x] Telegram 对话请求可创建 `telegram_chat` session
- [x] 主模型可生成 diagnosis 和 command candidate
- [x] 平台按授权策略决定 `direct_execute / require_approval / suggest_only / deny`
- [x] Telegram 审批按钮可回调且不再持续 loading
- [x] SSH 执行、输出截断、spool 落盘、verification 全链路已通
- [x] 结果可同时回传 Telegram 和 Web

### 2.2 安全与审计

- [x] 请求在发给 LLM 前做规则式脱敏
- [x] secret 永不回填
- [x] `RAW / SENT` 双视图审计已落地
- [x] Session Trace 可查看 Audit Trail / Knowledge Trace
- [x] 命令授权支持白名单、黑名单、override、glob 匹配

### 2.3 模型与提供方

- [x] Provider Registry 统一入口
- [x] 支持 `openai_compatible / anthropic / gemini / openrouter / ollama / lmstudio`
- [x] 支持 `primary / assist` 双模型绑定
- [x] 支持列模型和可用性检查
- [x] 当上游不支持列模型时，`check` 可回退到最小推理探活
- [x] 已完成 primary 故障时自动 failover 到 assist 的真实演练

### 2.4 Web 体验

- [x] `/login` Bearer Token 登录
- [x] `/setup` 首次安装入口与 `/runtime-checks` 运行体检入口
- [x] `/sessions` / `/sessions/:id` / `/executions/:id` / `/outbox`
- [x] `/ops` 可编辑授权策略、审批路由、prompt、脱敏规则、providers

### 2.5 观测与运维

- [x] `/healthz` / `/readyz` / `/metrics`
- [x] 统一 `8081` 入口，受保护的 `Ops API` 由路由开关控制
- [x] Grafana dashboard 导入物
- [x] fault injection 报告、runbook、release checklist

## 3. 已实证验收

以下能力不只是代码存在，而是已经在共享联调环境做过真实验证（当前环境为 `192.168.3.100`，原验证记录在 `192.168.3.106` 已停用）：

- [x] `webhook -> diagnosis -> approve -> execute -> resolved`
- [x] `modify_approve -> failed`
- [x] `diagnosis_only / approval_beta / execution_beta / knowledge_on` rollout 切换
- [x] Telegram long polling
- [x] Web Setup -> Runtime Checks 直通体验
- [x] LM Studio 作为 primary 真实生成命令
- [x] Gemini 作为 assist/provider failover 真实接管
- [x] 知识沉淀、向量索引、reindex
- [x] 故障注入：模型超时、VM 超时、SSH 超时、Telegram replay

## 4. 当前非阻塞限制

这些不阻塞 MVP 试点，但仍然属于下一阶段演进项：

- [ ] 真正的账号体系（当前仍是 Ops Token）
- [ ] secret manager / key rotation（当前为运行时文件 + env 注入）
- [ ] provider / component 健康历史持久化（当前运行态会随进程重启丢失）
- [ ] 更完整的 Web 配置模板库与预设
- [ ] 多记录场景的分页 / 搜索 / limit / 排序 / 批量操作
- [ ] 外部系统集成框架（JumpServer / Prometheus / VictoriaMetrics / APM / Git / CI/CD）
- [ ] MCP skill 外部源导入能力（当前已进入文档设计，但未实现）
- [ ] 更完整的 golden snapshot 前端交互样例

## 5. Go / No-Go 建议

### 可 Go（试点）

满足以下条件即可进入试点：

- [x] 本地基线入口明确：`make check-mvp` 为主入口，子步骤包含 `go test ./...`、`cd web && npm run test`、`cd web && npm run lint`、`cd web && npm run build`
- [x] 分层验证入口明确：`make smoke-remote` 负责共享环境 readiness/hygiene，`make live-validate` 负责 shared live validation，`bash scripts/ci/web-smoke.sh` 仍负责共享环境 Playwright smoke
- [x] `scripts/check_mvp.sh` 通过
- [ ] 目标环境的 providers / SSH / Telegram / VM 全部连通
- [ ] 至少完成一次 `/runtime-checks -> Telegram 审批 -> resolved`
- [ ] 至少完成一次 Telegram 对话式请求验收
- [x] 试点使用的授权策略和审批路由已固化到配置文件
- [x] 已明确试点后的 `knowledge / vector / outbox` 保留决策将按 `docs/reports/pilot-core-decision-gate-template.md` 填写，不在试点开始前拍脑袋决定

当前 EVI-13 证据说明：

- `make check-mvp`、`make smoke-remote`、`make live-validate` 已在本轮通过。
- `192.168.3.100` 的 SSH allowlist 基线已修复并验证为 `192.168.3.100,127.0.0.1`。
- VM / VL 连通正常，但 Telegram 仍是 `stub`，reasoning provider baseline 仍不足以支撑真实审批执行闭环。
- Telegram 对话路径已验证：缺少 host 时给出引导且不创建 session；指定 host 时会创建 `telegram_chat` session。
- 指定 host 的会话当前仍停留在 diagnosis-only closeout，尚未拿到 required `approval -> execute -> verifier` 证据。

### 暂不建议 Go

出现以下任一情况，先不要继续扩大试点：

- [ ] primary/assist 模型均不稳定
- [ ] Telegram 结果消息存在明显丢失
- [ ] SSH 白名单或授权策略仍频繁误判
- [ ] knowledge ingest 持续失败
- [ ] 试点团队仍依赖开发现场介入才能完成基本操作

## 5.1 试点后的保留决策门

MVP 可进入试点，不等于 `knowledge / vector / outbox` 已经被判定为长期保留。

试点结束后，需要按 [pilot-core-decision-gate-template.md](/Users/yue/TARS/docs/reports/pilot-core-decision-gate-template.md) 补齐以下证据：

- 首个可行动判断时间
- 建议采纳率
- 审批通过率
- 一周主动复用率
- Knowledge 命中率
- Outbox replay / dead-letter 率
- 操作员额外负担

## 6. 结论

截至当前仓库状态，TARS 已达到：

- 功能型 MVP：已完成
- 试点型 MVP：可进入试点
- 正式产品化上线：仍需一轮密钥治理、模板化和平台化硬化
