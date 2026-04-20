# TARS — 开发执行跟踪表 v0.1

> **对应 WBS**: [tars_dev_tasks.md](tars_dev_tasks.md) v1.3  
> **对应 TSD**: [tars_technical_design.md](tars_technical_design.md) v1.5  
> **日期**: 2026-03-12  
> **用途**: 作为执行态文档，记录任务推进状态、测试状态、阻塞项和交付备注  
> **原则**: `tars_dev_tasks.md` 负责计划基线；本文档负责执行跟踪

---

## 1. 状态定义

### 1.1 开发状态

| 状态 | 说明 |
|------|------|
| `未开始` | 尚未进入开发 |
| `开发中` | 正在编码或联调 |
| `联调中` | 代码基本完成，正在跨模块验证 |
| `阻塞` | 因前置依赖或外部条件卡住 |
| `已完成` | 开发完成并满足本任务 DoD |

### 1.2 测试状态

| 状态 | 说明 |
|------|------|
| `未测试` | 尚未进入测试 |
| `测试中` | 正在进行单测、集成测或验收 |
| `已通过` | 对应验收项已验证通过 |
| `失败待修复` | 已发现问题，需回到开发 |
| `不适用` | 纯文档或暂不需要测试 |

### 1.3 建议使用规则

- `开发状态` 只反映当前推进阶段，不表达测试结论
- `测试状态` 必须独立维护，避免“开发完成 = 测试完成”
- `阻塞项` 只写当前最核心 blocker，不堆长日志
- `备注` 用于放 PR、commit、联调说明、验收链接

---

## 2. 跟踪字段

| 字段 | 说明 |
|------|------|
| `ID` | 对应 WBS 任务 ID |
| `任务` | 简写，完整描述以 WBS 为准 |
| `Owner` | 当前负责人 |
| `开发状态` | `未开始 / 开发中 / 联调中 / 阻塞 / 已完成` |
| `测试状态` | `未测试 / 测试中 / 已通过 / 失败待修复 / 不适用` |
| `阻塞项` | 当前 blocker |
| `备注` | PR、联调结论、风险说明 |

---

## 3. M0 — Design Freeze

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `M0-1` | 冻结 API DTO、错误码、幂等与事务边界 | `TL/Core` | `已完成` | `不适用` | — | OpenAPI 和错误码已落文档/草案 |
| `M0-2` | 冻结 PostgreSQL / SQLite-vec migration baseline | `Platform` | `已完成` | `已通过` | — | migration 已在 `192.168.3.106` Docker Postgres 上执行通过 |
| `M0-3` | 准备样例数据和 golden cases | `Integration` | `已完成` | `已通过` | — | 已新增 [deploy/pilot/golden_cases.md](../deploy/pilot/golden_cases.md) 与 [deploy/pilot/pilot.env.example](../deploy/pilot/pilot.env.example)，把 Setup -> Runtime Checks、VMAlert、Telegram 对话、Provider failover、脱敏审计和知识沉淀整理成试点 golden cases |
| `M0-4` | 定义 feature flags 与发布顺序 | `TL/Core` | `已完成` | `不适用` | — | 已体现在 TSD / WBS |
| `M0-5` | 建立契约测试基线 | `TL/Core`,`Integration` | `已完成` | `已通过` | — | 当前已补最小 HTTP 集成测试，并新增 [validate_openapi.rb](../scripts/validate_openapi.rb) 与 [check_mvp.sh](../scripts/check_mvp.sh)；同时已接入 GitHub Actions 工作流 [mvp-checks.yml](../.github/workflows/mvp-checks.yml) 统一执行 Go / OpenAPI / Web 校验 |

---

## 4. Sprint 1 — 告警到诊断链路

## 4.1 Setup / Runtime Config 第一阶段（2026-03-23）

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `CFG-1` | setup_state / runtime_config_documents schema | `Platform` | `已完成` | `已通过` | — | 已在 `internal/repo/postgres/schema.go` 与 `internal/repo/postgres/runtime_config.go` 落地 |
| `CFG-2` | setup wizard 最小闭环 API | `Platform` | `已完成` | `已通过` | — | 已补 `/api/v1/setup/wizard*`、`setup/status.initialization` 与 HTTP 测试 |
| `CFG-3` | `/setup` 与 `/runtime-checks` 分流 | `Frontend` | `已完成` | `已通过` | — | 未初始化时任意业务路径统一进入 `/setup`；初始化完成后 `/setup` 分流到 `/login` 或 `/runtime-checks`，并新增匿名 `/api/v1/bootstrap/status` 供前端探测 |
| `CFG-4` | providers / access 第一阶段 DB-backed 同步 | `Platform` | `已完成` | `测试中` | 需做全量基线回归 | bootstrap 会优先回灌 DB runtime config，wizard/provider 更新会同步写 DB |
| `CFG-5` | 文档与全量回归 | `All` | `开发中` | `未测试` | 待完成最终基线 | 当前已更新 PRD/TSD/WBS/Tracker/Web/deploy 说明 |

## 4.2 Setup / Runtime Config 第二阶段（2026-03-23）

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `CFG2-1` | manager persistence hook 收口 | `Platform` | `已完成` | `已通过` | — | `access/providers/connectors` 已支持 persistence hook，避免继续扩大 file-backed path 依赖 |
| `CFG2-2` | connectors 切到 runtime DB 主路径 | `Platform`,`TL/Core` | `已完成` | `已通过` | — | connectors config + lifecycle 已落到 runtime DB，`/api/v1/config/connectors` 无 file path 也可用 |
| `CFG2-3` | setup provider step 强校验 | `Platform`,`AI/Knowledge` | `已完成` | `已通过` | — | 已补 `base_url`、`secret ref`、secret existence 与 provider connectivity check，相关 HTTP 测试已通过 |
| `CFG2-4` | setup 完成后登录引导 | `Frontend`,`Platform` | `已完成` | `已通过` | — | `login_hint`、自动登录、`/login?provider_id&username` 预填已落地，前端 lint 已通过 |
| `CFG2-5` | 文档与全量回归收口 | `All` | `开发中` | `未测试` | 待执行 `go test ./...`、`check_mvp`、`openapi`、`web build` | 当前已完成代码主线与文档补写，待最终基线验证 |

### 4.1 平台基础与契约落地

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S1-1` | Go module、目录骨架、contracts 包 | `Platform` | `已完成` | `已通过` | — | 代码骨架已建立 |
| `S1-2` | 配置加载、环境变量注入、feature flags | `Platform` | `已完成` | `已通过` | — | 基础 env config 可用，已补 SSH / 输出落盘相关配置 |
| `S1-3` | HTTP server、公共路由、中间件 | `Platform` | `已完成` | `已通过` | — | `/healthz` `/readyz` `/metrics` 可用 |
| `S1-4` | 初版 migration、repo 骨架、事务 helper | `Platform` | `已完成` | `已通过` | — | Postgres workflow store 与 app wiring 已落地，并在 `192.168.3.106` Docker Postgres 上完成 migration / smoke / 重启后持久化验证 |
| `S1-5` | 结构化日志、Prometheus metrics、审计基础 | `Platform` | `已完成` | `已通过` | — | `/metrics` 已由占位实现升级为 Prometheus 指标输出，覆盖 HTTP / alert ingest / outbox / Telegram / execution / knowledge / GC / feature flags；远端 `192.168.3.106` 已验证指标实时增长；结构化启动日志已补 rollout / feature flags，Ops 读写接口、配置中心读取与 trace 查询均已接入 audit logger，配套 [tars-mvp-dashboard.json](../deploy/grafana/tars-mvp-dashboard.json) 已可直接导入 |
| `S1-6` | Docker Compose、本地开发脚本 | `Platform` | `已完成` | `未测试` | 本地依赖联调未回归 | compose 与脚本已存在 |

### 4.2 Alert Intake + Workflow 骨架

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S1-7` | VMAlert webhook handler + 签名校验 | `Integration` | `已完成` | `已通过` | — | 已支持配置化 webhook secret 校验 |
| `S1-8` | AlertEvent 标准化、fingerprint、payload hash | `Integration` | `已完成` | `已通过` | — | 已补 payload hash / `IdempotencyKey` 生成，并在 `192.168.3.106` 验证重复告警复用同一 session |
| `S1-9` | `idempotency_keys` 机制落地 | `Platform` | `已完成` | `已通过` | — | webhook / callback 两条路径都已接入幂等；`192.168.3.106` 已验证重复 callback no-op、重复 alert no-op |
| `S1-10` | Workflow Core application service 与状态机骨架 | `TL/Core` | `已完成` | `已通过` | — | 内存版状态流和基本状态推进已跑通 |
| `S1-11` | Session / session_events / outbox 基础写入 | `TL/Core` | `已完成` | `已通过` | — | Postgres workflow 已完成 session/execution/outbox 基础写入，远端验证到 `pending_approval -> resolved` 并在重启后保持可查询 |
| `S1-12` | 乐观锁更新 helper | `TL/Core` | `未开始` | `未测试` | 非阻塞 MVP，已由 `SELECT ... FOR UPDATE` + 状态守卫覆盖主链路并发安全 | 保留为后续产品化增强项，若需要真正的版本冲突语义，再补 `state_conflict` 返回与显式 version compare |

### 4.3 Action Gateway 查询链路

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S1-13` | VictoriaMetrics provider 接口与实现 | `Integration` | `已完成` | `已通过` | — | 已补 HTTP provider，并在 `192.168.3.106` 自建 `VictoriaMetrics + vmagent + node-exporter + vmalert` 完成真实 endpoint 验证 |
| `S1-14` | Action Gateway query path | `TL/Core` | `已完成` | `已通过` | — | dispatcher 已接入 query path，且 `vmalert -> TARS` 远端 smoke 已通过真实 VM 数据源跑通 |
| `S1-15` | provider 失败降级与指标埋点 | `Integration` | `已完成` | `已通过` | — | 模型请求失败已回退本地 diagnosis，VictoriaMetrics 查询失败已降级为 stub metrics，不再打崩 diagnosis 主链路；同时补齐 `tars_external_provider_requests_total`，`192.168.3.106` 已验证两类 provider 成功计数实时增长 |

### 4.4 Reasoning + 基础 Knowledge

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S1-16` | 脱敏引擎与 `desense_map` 持久化 | `AI/Knowledge` | `已完成` | `已通过` | — | 已完成模型前递归脱敏、secret 永不回填、host/IP/path 有限回填与 `desense_map` 持久化；`192.168.3.106` 已验证 `diagnosis_summary` 保持原始 IP，同时数据库写入 `{\"[IP_1]\": ...}`；2026-03-12 继续补强 `password / token / api key / bearer / basic auth` 脱敏，发给 LLM 前统一替换为 `[REDACTED]`，并新增规则保证带 `[REDACTED]` 或明显密钥片段的 `execution_hint` 不会进入审批/执行草稿；同日已部署 `v11` 到 `192.168.3.106` 并通过 `healthz/readyz` 验证；本轮继续将路径类值单独归类为 `PATH_*` 占位符，避免 `/tmp/1.txt` 这类文件路径被误识别成 `HOST_*`；同日已部署 `v12` 到 `192.168.3.106`，新会话 `a34eeadf-c88d-4146-9844-f4035ab23ad3` 已验证 `desense_map={\"[IP_1]\":\"192.168.3.106\",\"[PATH_1]\":\"/tmp/4.txt\"}`，审批草稿命令为真实值 `cat /tmp/4.txt` |
| `S1-17` | OpenAI-compatible client + timeout/retry | `AI/Knowledge` | `已完成` | `已通过` | — | 已完成 OpenAI-compatible `chat/completions` client、timeout/retry 与远端真实网关接入；失败时自动回退 deterministic diagnosis，密钥未写入仓库 |
| `S1-18` | Prompt builder + DiagnosisOutput 解析 | `AI/Knowledge` | `已完成` | `已通过` | — | 已支持模型 JSON 输出解析、包裹文本提取和 execution hint 安全兜底；`192.168.3.106` 已验证真实模型返回 diagnosis 摘要 |
| `S1-19` | 文档导入与基础 RAG 检索 | `AI/Knowledge` | `已完成` | `已通过` | — | 已补纯 Go SQLite 向量索引、`chunk_vectors` 真落库和 hybrid search；不配置 `TARS_VECTOR_SQLITE_PATH` 时保留 lexical fallback |
| `S1-20` | Knowledge `Search` 契约与集成 | `AI/Knowledge` | `已完成` | `已通过` | — | 已完成 Search 契约接线、lexical search 与 diagnosis 引用集成；`192.168.3.106` 已验证后续相似告警带 `knowledge:` 引用 |

### 4.5 Telegram 诊断下发

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S1-21` | Telegram bot 初始化与发送客户端 | `Integration` | `已完成` | `已通过` | — | 已用真实 bot token 调通 `getMe` 和 `sendMessage`，远端 `v7` 已开启 long polling |
| `S1-22` | Channel Adapter `SendMessage` 契约实现 | `Integration` | `已完成` | `已通过` | — | `SendMessage` 已支持 inline keyboard；真实 `chat_id=445308292` 已验证直发消息、自动审批消息和结果消息下发 |
| `S1-23` | 诊断消息模板与引用展示 | `AI/Knowledge` | `已完成` | `已通过` | — | 已把诊断消息统一成 `[TARS 诊断]` 模板，补齐告警/服务/级别/来源/建议命令/知识引用字段，并新增工作流单测覆盖消息正文与引用截断展示 |
| `S1-24` | Message retry via outbox | `Platform` | `已完成` | `已通过` | — | 已完成 `telegram.send` outbox topic、payload 持久化、失败后自动重试和 fallback 入队；本地已覆盖 diagnosis 消息首次发送失败后由 dispatcher 重试送达，以及 callback 结果消息发送失败后入 outbox 再补发 |

### 4.6 Sprint 1 验收与缓冲

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S1-25` | 单测：fingerprint / desense / state machine | `TL/Core`,`AI/Knowledge` | `已完成` | `已通过` | — | 当前已覆盖 HTTP 集成测试、action 风险/输出、reasoning 脱敏/回填、workflow desense_map 存储、诊断消息模板与引用截断等关键分支；2026-03-13 继续补上 provider probe 边界回归：当上游不支持列模型时，`CheckProviderAvailability` 会回退到最小推理探活；同时新增 `setup/status` 主/辅模型运行态分离测试，验证 `model_primary / model_assist` 不会再相互串位 |
| `S1-26` | 契约测：VMAlert/Telegram DTO 与错误码 | `TL/Core`,`Integration` | `已完成` | `已通过` | — | webhook/ops 路径测试、[validate_openapi.rb](../scripts/validate_openapi.rb)、[check_mvp.sh](../scripts/check_mvp.sh) 与 GitHub Actions [mvp-checks.yml](../.github/workflows/mvp-checks.yml) 已统一接通 |
| `S1-27` | 集成测：webhook -> session -> diagnosis -> Telegram | `Integration` | `已完成` | `已通过` | — | 本地测试已覆盖 webhook / diagnosis / approval；`192.168.3.106` 已验证真实 Telegram long polling 下发诊断/审批/结果消息，Postgres 路径可到 `resolved` 且重启后状态保留 |
| `S1-28` | Sprint 1 缓冲与缺陷修复 | `全员` | `已完成` | `已通过` | — | Sprint 1 范围内的主链路缺陷已完成收口，当前遗留项已转为 Sprint 2 试点包装和非阻塞增强 |

---

## 5. Sprint 2 — 审批到执行闭环

### 5.1 审批交互与路由

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S2-1` | Telegram callback handler + webhook secret 校验 | `Integration` | `已完成` | `已通过` | — | webhook + long polling 双路径已接通；真实 Telegram callback 已完成 `approve -> execute -> result` 闭环，且补齐 `answerCallbackQuery` 后按钮不再持续 loading |
| `S2-2` | `telegram_update` 幂等处理 | `Platform` | `已完成` | `已通过` | — | Postgres workflow 已接 `idempotency_keys`，并在 `192.168.3.106` 验证重复 callback 使用同一 `update_id` 时返回 no-op |
| `S2-3` | `ChannelEvent` -> `HandleChannelEvent` 映射 | `TL/Core` | `已完成` | `已通过` | — | `approve / reject / request_context` 已完成映射，真实 Telegram long polling callback 已验证执行状态推进与重复 update 幂等 |
| `S2-4` | 审批路由实现 | `TL/Core` | `已完成` | `已通过` | — | 已支持 `service_owner -> oncall(default)` 路由，`approval_group` 写入 execution，并在 `192.168.3.106` 远端 smoke 中验证 `service_owner:sshd` |
| `S2-5` | 审批模板、路由来源、SLA 展示 | `Integration` | `已完成` | `已通过` | — | 审批消息已展示来源与时限，并通过 Telegram inline keyboard 下发 3 个审批动作 |
| `S2-6` | Approval timeout worker + 升级/拒绝逻辑 | `Platform` | `已完成` | `已通过` | — | 已补 timeout worker；修复 Postgres `bad connection` 后，远端 `v7` 已实际触发 timeout 拒绝与通知 |

### 5.2 SSH 执行与风险控制

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S2-7` | SSH executor | `Integration` | `已完成` | `已通过` | — | 已接真实 `ssh` 命令执行，`192.168.3.106` smoke 通过 |
| `S2-8` | 命令风险分级、黑名单、白名单服务约束 | `TL/Core` | `已完成` | `已通过` | — | 已有命令前缀 allowlist + blocked fragments + host allowlist；新增 reasoning 侧 `execution_hint` 安全兜底，模型返回 `ssh/sudo/自然语言` 时自动降回本地安全模板；2026-03-12 继续补上审批配置里的 `execution.command_allowlist.<service>`，允许按服务放开修复类命令前缀，并新增 action/approvalrouting 回归测试 |
| `S2-9` | 执行输出分块、截断、spool fallback | `Platform` | `已完成` | `已通过` | — | 已支持 `output_ref/output_bytes/output_truncated`、Postgres `execution_output_chunks`、spool fallback 与 GC；`192.168.3.106` 已验证长输出被截断为 4 chunks / 64 bytes 持久化后再恢复到默认配置 |
| `S2-10` | ExecutionResult 回传与状态推进 | `TL/Core` | `已完成` | `已通过` | — | 真实执行结果已回写 execution/session 状态，并在 smoke 中验证到 `resolved` |

### 5.3 闭环验证与知识沉淀

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S2-11` | 验证器：`executing -> verifying` 自动检查 | `Integration` | `已完成` | `已通过` | — | 已实现执行后内联 verifier：默认用 `systemctl is-active <service>` 检查恢复结果；本地已覆盖 success/failed/skipped 分支，`192.168.3.106` 已验证 `approve -> verify_success -> resolved` |
| `S2-12` | `resolved` / `failed` 路径实现 | `TL/Core` | `已完成` | `已通过` | — | `completed -> verifying -> resolved/analyzing` 和 `failed/timeout -> failed` 都已接通；`192.168.3.106` 已验证 `verification.status=success`、`verify_success` timeline 以及既有失败链路 |
| `S2-13` | Knowledge ingest worker | `AI/Knowledge` | `已完成` | `已通过` | — | `session.closed` 已真实落 `documents / document_chunks / knowledge_records`，并在 `192.168.3.106` Docker Postgres 上验证 |
| `S2-14` | Session -> document -> chunks -> vectors 统一落库 | `AI/Knowledge` | `已完成` | `已通过` | — | 已完成 `document / chunks / knowledge_records / chunk_vectors` 统一落库；本地已通过 sqlite 向量索引、ReplaceDocument/Search 与 hybrid search 回归；2026-03-12 已在 `192.168.3.106` 打开 `TARS_VECTOR_SQLITE_PATH`、执行 `reindex` 并确认 `chunk_vectors=37 documents=10` |
| `S2-15` | Execution/resolve result 消息下发 | `Integration` | `已完成` | `已通过` | — | 真实 Telegram 私聊已验证自动审批消息送达、`answerCallbackQuery` 去除按钮 loading、以及“批准执行”后的开始执行消息与结果消息下发 |

### 5.4 运维支持、发布与硬化

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S2-16` | Ops API 独立监听、Bearer Token 鉴权、审计 | `Platform` | `已完成` | `已通过` | — | 已拆成独立 Ops listener，主入口不再暴露 sessions/executions/outbox/reindex；Bearer Token 鉴权保留，读写接口均接入 audit logger，`192.168.3.106` 已验证 `18081` 主入口返回 `404`、`8081` Ops 入口可鉴权访问并写入 `audit event` |
| `S2-17` | 运维查询接口 | `Platform` | `已完成` | `已通过` | — | sessions / session detail / execution detail / outbox 查询均可用，且 execution 读侧已包含 `approval_group/output_bytes/output_truncated` 等真实数据 |
| `S2-18` | Outbox replay 接口 + 审计 | `Platform` | `已完成` | `已通过` | — | blocked replay 已实现，且 `outbox replay` 已接入基础 audit logger 和 metrics 计数 |
| `S2-19` | Outbox `blocked` 状态与 feature flag 暂存策略 | `TL/Core`,`Platform` | `已完成` | `已通过` | — | 已完成 Postgres + 内存双实现的 `blocked/processing/pending/failed/delivered` 路径、手动 replay 和启动恢复；`192.168.3.106` 已验证 rollout blocking 与 stuck `processing` 事件恢复 |
| `S2-20` | Idempotency GC、Execution Output GC | `Platform` | `已完成` | `已通过` | — | 已新增 GC worker，覆盖 Postgres `idempotency_keys` 过期清理和 execution output spool 目录按保留期删除；本地格式化、定向测试与全量构建已通过 |
| `S2-21` | Feature flags + rollout mode 实现 | `TL/Core` | `已完成` | `已通过` | — | 已支持 `TARS_ROLLOUT_MODE=diagnosis_only/approval_beta/execution_beta/knowledge_on`，并允许 `TARS_FEATURES_*` 逐项覆盖；补齐配置单测与部署说明 |
| `S2-22` | TARS 自身 dashboard + 告警规则 | `Platform` | `已完成` | `已通过` | — | 已补齐 [observability_dashboard.md](../docs/operations/observability_dashboard.md) 面板建议、[tars-self-rules.yml](../deploy/docker/tars-self-rules.yml) 最小告警规则，以及可直接导入 Grafana 的 [tars-mvp-dashboard.json](../deploy/grafana/tars-mvp-dashboard.json)；远端 `192.168.3.106` 已验证 `/metrics` 输出、HTTP/alert/outbox/Telegram/knowledge 计数增长，以及 `tars_external_provider_requests_total` 记录 VictoriaMetrics 和模型网关请求 |
| `S2-23` | 部署文档、回滚手册、试点 runbook | `TL/Core`,`Platform` | `已完成` | `不适用` | — | 已补齐 [deploy/README.md](../deploy/README.md) 、[pilot_runbook.md](../docs/operations/pilot_runbook.md) 与 [release_checklist.md](../docs/operations/release_checklist.md)；覆盖 webhook / long polling、审批路由、VM smoke、rollout mode、试点步骤与回滚动作；模型密钥未写入仓库 |

### 5.5 测试、演练与稳定性缓冲

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `S2-24` | 单测补齐：审批路由、风险分级、幂等冲突 | `TL/Core`,`Platform` | `已完成` | `已通过` | — | 已新增 approval routing、Alertmanager payload、approval timeout/invalid state、action 风险/输出、execution error fallback、verifier success/failed/skipped、provider failover 和 provider probe 边界测试；本轮继续补上 `/api/v1/config/providers/check` 在“列模型失败但推理可用”场景下的回退回归，以及 `setup/status` 主/辅模型运行态断言 |
| `S2-25` | 集成测：重复 callback、输出截断、知识幂等、outbox replay | `Integration`,`AI/Knowledge` | `已完成` | `已通过` | — | 已覆盖重复 callback、重复 alert、webhook->approval->execution、本地真实 SSH smoke、verification failure 回到 `analyzing`、diagnosis/result 消息发送失败后的 outbox fallback 与自动补发；`192.168.3.106` 已完成 Postgres 持久化重启验证、webhook payload-hash 幂等验证、输出截断分块落库验证、outbox 恢复验证，以及双 `reindex` 前后 `document_chunks` 指纹 / `knowledge_records` / `chunk_vectors` 数量一致的知识幂等验证 |
| `S2-26` | 故障注入演练 | `全员` | `已完成` | `已通过` | — | 已在 `192.168.3.106` 完成模型不可达、VictoriaMetrics 不可达、SSH 执行超时、Telegram callback replay 演练，结果见 [fault_injection_report.md](../docs/reports/fault_injection_report.md) |
| `S2-27` | 发布检查清单与回滚演练 | `TL/Core`,`Platform` | `已完成` | `已通过` | — | 已新增 [release_checklist.md](../docs/operations/release_checklist.md) ，并在 `192.168.3.106` 完成一次真实 rollout/rollback drill：切到 `diagnosis_only` 后新告警停在 `open + executions=0`，恢复全开后新告警回到 `pending_approval`；过程同时发现并清理了残留 `v6` 实例串库消费问题 |
| `S2-28` | 真实样本端到端验收 | `全员` | `已完成` | `已通过` | — | 已在 `192.168.3.106` 完成 Docker Postgres + SSH 目标机 smoke，链路达到 `webhook -> diagnosis -> approve -> execute -> verify_success -> resolved -> restart`，并补测 `modify_approve -> failed`；同时完成 `VictoriaMetrics + vmalert` 自触发 smoke、真实 Telegram 私聊自动审批送达与按钮回调、真实模型网关 diagnosis、脱敏链路 `desense_map` 持久化验证、长输出截断分块落库验证、向量库启用后的 `reindex -> chunk_vectors=37 documents=10` 验证，以及模型/VM/SSH/replay 故障注入演练；2026-03-12 额外完成 `diagnosis_only -> execution_beta` 回滚演练并定位清理残留 `v6` 实例，并部署包含 `telegram.send retry/outbox` 与向量检索代码的最新二进制到远端，验证 `healthz/readyz` 与 Ops 只读查询正常；随后又完成了“Web 登录 -> Setup/Smoke -> Telegram 审批 -> SSH 执行 -> verification -> resolved”的直接体验链路验收：远端 `192.168.3.106:8081/login` 可直接访问，live smoke `1133216e-9634-4ff7-aa24-0524cb74703e` 经 Telegram 批准后已 `resolved`，并校验 `verification.status=success`；用户从 Web 触发的 smoke `70587972-055f-477f-b207-f5c833d4092c` 也已完成 `resolved` |
| `S2-29` | Sprint 2 缓冲与缺陷修复 | `全员` | `已完成` | `已通过` | — | 已完成 Web Console review、Setup/Smoke 体验、Telegram 对话入口、provider registry、trace 可视化、LM Studio primary + Gemini assist failover、试点交付包 [deploy/pilot/README.md](../deploy/pilot/README.md) 与正式 [mvp_completion_checklist.md](../docs/operations/mvp_completion_checklist.md) 整理；本轮继续把 [check_mvp.sh](../scripts/check_mvp.sh) 收口为 `go test + go build + OpenAPI + web lint/build` 一键回归脚本 |

补充记录（2026-03-13）：
- 已将“本地 LLM 辅助脱敏设计”正式补入 [tars_technical_design.md](tars_technical_design.md) 第 8 节，明确开关、接口、信任边界、失败回退和审计策略。
- 已新增 [desensitization.example.yaml](../configs/desensitization.example.yaml) 与 [30-strategy-desensitization.md](../specs/30-strategy-desensitization.md)，并把 `TARS_DESENSITIZATION_CONFIG_PATH`、`GET/PUT /api/v1/config/desensitization`、`/ops` 引导式配置页和 `/setup` 状态展示全部落到运行时与前端。
- 远端 `192.168.3.106` 已部署 `v15` 并加载 `/root/tars-desensitization-v1.yaml`，验证 `GET /api/v1/setup/status` 与 `GET /api/v1/config/desensitization` 正常返回，说明脱敏规则配置化已可用；`local_llm_assist` 目前进入配置与控制面，但仍保持 `design / reserved`，未替代规则式脱敏主链路。
- 2026-03-13 已继续把 `local_llm_assist` 从“纯控制面预留”推进为真实 `detect_only` 实现：本地可信边界内的 OpenAI-compatible 模型可先接收原始上下文，返回 `secrets / hosts / ips / paths` 四类精确值，平台再统一执行替换；失败时自动回退到纯规则式脱敏，不阻断主诊断链路。已补 reasoning 单测覆盖“辅助识别生效 / 辅助模型失败回退”，全量 `go test ./...`、前端 `lint/build`、OpenAPI 校验通过；远端 `192.168.3.106` 已部署 `v16` 并验证 `healthz/readyz`、`GET /api/v1/setup/status`、`GET /api/v1/config/desensitization` 正常。
- 2026-03-13 已继续把主模型协议适配层扩展为 `openai_compatible / anthropic / ollama / lmstudio` 四种协议：新增 `TARS_MODEL_PROTOCOL`，OpenAI-compatible 与 LM Studio 自动补齐 `/v1/chat/completions`，Anthropic 走 `/v1/messages`，Ollama 走 `/api/chat`；并补了 Anthropic/Ollama/LM Studio 单测，`/setup` 页面现会显示当前模型协议。远端 `192.168.3.106` 已部署 `v17` 并验证 `setup/status` 中 `model.protocol=openai_compatible`。另已从远端实测你提供的 LM Studio 地址 `http://192.168.1.132:1234`，当前 `192.168.3.106 -> 192.168.1.132:1234` 为超时，说明协议已支持但网络尚未打通，因此暂未切换试点环境模型配置。
- 2026-03-13 已补齐试点交付收尾脚本 [pilot_hygiene_check.sh](../scripts/pilot_hygiene_check.sh) 与 [run_demo_smoke.sh](../scripts/run_demo_smoke.sh)，并在远端 `192.168.3.106` 完成真实卫生检查：Ops 维度健康，`active_sessions=31 / executions_total=54 / blocked_outbox=0`，但存在 `failed_outbox=6` 的历史样本。脚本已收敛为“public `/healthz` / `/readyz` 检查失败仅告警、不阻断报告”的行为，适配 server listener 仅内网暴露的试点环境。
- 2026-03-13 已重新执行 [check_mvp.sh](../scripts/check_mvp.sh)，结果通过：`go test`、`go build`、OpenAPI 校验、Web `lint/build` 全部为绿；同时 `pilot_hygiene_check.sh` 与 `run_demo_smoke.sh` 均已通过 `sh -n` 语法检查。
- 2026-03-13 已新增 Outbox 删除能力：后端补充 `DELETE /api/v1/outbox/{event_id}`，只允许删除 `failed / blocked` 条目且要求 `operator_reason`；Web `/outbox` 已新增 `Delete` 按钮，用于清理已确认不再 replay 的历史残留。远端 `192.168.3.106` 已部署 `v22` 并完成最小验证：删除历史失败条目 `1a18146b-2af1-491c-87bc-d76cab412390` 成功，`failed_outbox` 已由 `6` 降为 `5`。
- 2026-03-13 已把“多记录场景的分页 / 搜索 / limit / 排序 / 批量操作”记为下一阶段产品化要求，覆盖 `sessions / executions / outbox / audit / knowledge trace` 等列表视图；当前不阻塞 MVP 试点，但在试点扩大前需要纳入下一阶段范围。
- 2026-03-13 已完成一轮正式验收演示：基线卫生检查 `result=clean`，随后通过 [run_demo_smoke.sh](../scripts/run_demo_smoke.sh) 触发 `TarsFormalAcceptance20260313`，会话 `07131c12-b3c6-44db-8f9a-0dcdacc51f95` 最终 `resolved`，执行 `b4291304-acac-4c30-b617-96009ff87db4` 为 `completed`，verification=`success`。本次样本因命中白名单策略走 `policy:direct_execute`，未经过 Telegram 审批，但 Telegram diagnosis / 策略消息 / 结果消息、Web trace / audit / knowledge 均已落地可查。
- 2026-03-13 已把两条后续产品化方向正式写入 PRD/TSD：1) 外部系统集成框架，要求后续接入 `JumpServer / VictoriaMetrics / Prometheus / APM / Git / CI/CD`，不能只依赖 SSH 上机排障；2) 多记录列表统一框架，要求 `sessions / executions / outbox / audit / knowledge trace` 等全部支持统一的分页、搜索、limit、排序和批量操作，而不是按页面零散实现。
- 2026-03-13 已完成统一列表框架第一段落地：`sessions / executions / outbox` 已接入统一的 `page / limit / q / sort_by / sort_order` 查询协议，后端返回统一分页元信息，Web 已增加 `Executions` 列表页并为 `sessions / executions / outbox` 接入分页、搜索、limit 和排序。批量操作仍待作为下一段产品化工作推进。
- 2026-03-13 已部署 `v23` 到 `192.168.3.106`，并验证 `/executions` Web 路由和 `sessions / executions / outbox` 列表 API 的统一分页协议在真实环境可用；期间修复了 Postgres 列表查询里的两处 schema 漂移：`uuid ILIKE` 需要显式 `::text`，以及 `execution_requests.command` 列名不应误写成 `command_text`。
- 2026-03-13 已开始统一批量操作框架的第二段落地：新增 `outbox` 批量 `replay / delete` API 和前端批量选择工具条，作为后续扩展到 `sessions / executions / audit / knowledge trace` 的共用底座。
- 2026-03-13 已部署 `v24` 到 `192.168.3.106`，并验证 `outbox` 批量操作接口可用；当前远端 `POST /api/v1/outbox/bulk/replay` 已返回统一的批量结果结构，且非法 ID 会被收口成 `validation_failed`，不再直接把数据库错误暴露给前端。
- 2026-03-13 已新增统一规范文档 [40-ux-unified-list-bulk.md](../specs/40-ux-unified-list-bulk.md)，明确“后端协议是主底座、前端组件是交互壳”的职责划分，并定死统一查询参数、统一分页返回、统一批量请求/结果结构、当前页选择语义和分阶段落地顺序。
- 2026-03-13 已完成统一列表框架第二段：新增 `POST /api/v1/sessions/bulk/export` 与 `POST /api/v1/executions/bulk/export`，返回 JSON attachment 并支持部分成功/失败明细；Web `sessions / executions` 页面已接通当前页勾选、全选和批量导出，导出后会直接下载 JSON，并提示导出成功/失败数量。
- 2026-03-13 已部署 `v25` 到 `192.168.3.106`，并完成最小验证：`/sessions` 与 `/executions` 页面可直接勾选当前页记录执行批量导出；远端批量导出接口在有效 ID 与非法 ID 混合情况下会返回 attachment，同时把非法 ID 收口为 `validation_failed`，保持与统一批量框架规范一致。
- 2026-03-13 已完成统一列表框架第三段：新增 `GET /api/v1/audit` 与 `GET /api/v1/knowledge`，两者均接入统一的 `page / limit / q / sort_by / sort_order` 协议；Web 新增 `/audit` 与 `/knowledge` 页面，可分页查看审计记录和知识沉淀记录，并跳转回关联 Session。
- 2026-03-13 已修正 `audit / knowledge` 列表总数语义：后端不再提前以固定 500 条截断结果集，`total` 现在反映真实匹配数量，避免统一分页框架在记录较多时误导用户。
- 2026-03-13 已部署 `v27` 到 `192.168.3.106`，并完成最小验证：`/audit` 与 `/knowledge` 页面可直接打开，`GET /api/v1/audit?limit=2&page=1` 已返回 `total=542`，`GET /api/v1/knowledge?limit=2&page=1` 已返回 `total=38`，说明统一分页元信息已在真实环境生效。
- 2026-03-13 已补平台化开放接口基线：新增公开 discovery 接口 `GET /api/v1/platform/discovery`、连接器 manifest 数据结构 `tars.connector/v1alpha1`，并补齐 [20-component-connectors.md](../specs/20-component-connectors.md)。这为后续 `JumpServer / Prometheus / MCP / 插件市场` 统一接入提供了公共入口与规范基础。
- 2026-03-13 已补三份连接器 manifest 样例：[prometheus.connector.example.yaml](../configs/connectors/prometheus.connector.example.yaml)、[jumpserver.connector.example.yaml](../configs/connectors/jumpserver.connector.example.yaml)、[skill-source.connector.example.yaml](../configs/connectors/skill-source.connector.example.yaml)，用于后续连接器注册、导入导出和市场包格式对齐。
- 2026-03-13 已部署 `v28` 到 `192.168.3.106`，并在远端本机验证 `GET http://127.0.0.1:18081/api/v1/platform/discovery` 返回 discovery 元数据、`GET http://127.0.0.1:18081/healthz` 返回 `{\"status\":\"ok\"}`。当前对外可直接访问的仍是 `8081` Web/Ops 入口，`18081` 主业务入口继续按试点环境约束使用。
- 2026-03-13 已继续把 Connector Registry 从“配置中心”推进为“平台公开只读能力”：新增 `GET /api/v1/connectors`、`GET /api/v1/connectors/{id}`，Web 新增 `/connectors` 与 `/connectors/:id` 页面，并让 `/api/v1/platform/discovery` 动态返回 `registered_connectors_count / registered_connector_ids / registered_connector_kinds`。本地全量 `check_mvp.sh` 已通过；远端已部署 `v29` 到 `192.168.3.106`，并补齐 `TARS_CONNECTORS_CONFIG_PATH=/root/tars-connectors-v1.yaml`，验证 `GET http://127.0.0.1:18081/api/v1/connectors?limit=10&page=1` 返回 `prometheus-main / jumpserver-main` 两条注册表记录，`GET http://127.0.0.1:18081/api/v1/connectors/prometheus-main` 返回 manifest 详情，`GET http://127.0.0.1:8081/api/v1/setup/status` 中 `connectors.total_entries=2`，同时 `http://192.168.3.106:8081/connectors` 已可直接访问前端入口。
- 2026-03-13 已把 Connector Registry 扩成真正的控制面：新增 `GET /api/v1/connectors/{id}/export?format=yaml|json`、`POST /api/v1/connectors/{id}/enable`、`POST /api/v1/connectors/{id}/disable`，并让 `/connectors/:id` 页面支持 manifest 导出、启用/停用和 metrics runtime smoke；同时在 connector manifest 中补齐 `config.values`，使 metrics 连接器运行时直接读取实例自身的 `base_url / bearer_token`，不再依赖全局 `TARS_VM_BASE_URL`。公开 detail/export 已收口为不回显 runtime connection values 或 secret。
- 2026-03-13 已部署 `v30` 到 `192.168.3.106`，并把远端 `/root/tars-connectors-v1.yaml` 升级为 3 条注册表记录：`prometheus-main / victoriametrics-main / jumpserver-main`。远端已验证：`GET http://127.0.0.1:18081/api/v1/platform/discovery` 返回 `registered_connectors_count=3`；`GET http://127.0.0.1:18081/api/v1/connectors` 返回三条 manifest；`GET /api/v1/connectors/prometheus-main/export?format=yaml` 的公开导出不包含 runtime config；`POST /api/v1/connectors/prometheus-main/disable|enable` 成功切换状态；`POST /api/v1/connectors/victoriametrics-main/metrics/query` 与 `POST /api/v1/connectors/prometheus-main/metrics/query` 使用 `host=127.0.0.1:9100` 时均成功返回 `up{instance=\"127.0.0.1:9100\",job=\"node\"}`，证明第一条官方 `Prometheus / VictoriaMetrics` 连接器运行时链路已打通。
- 2026-03-19 已继续把连接器平台推进到真实 runtime/lifecycle 阶段：`JumpServer` execution connector 不再是 synthetic stub，现已走真实 HTTP 链路（资产查找、作业提交、轮询、结果/日志抓取），并把输出通过既有 execution output path 持久化回 `output_ref / output_bytes / output_truncated / output_preview`。
- 2026-03-19 已为 Connector Registry 补齐生命周期控制面：新增 `POST /api/v1/connectors/{id}/health`、`POST /api/v1/connectors/{id}/upgrade`、`POST /api/v1/connectors/{id}/rollback` 与 `POST /api/v1/connectors/{id}/execution/execute`；lifecycle DTO 已补 `available_version / history / health_history / revisions / from_version / to_version`，Web `/connectors/:id` 已开始展示 timeline、upgrade 与 rollback 入口。
- 2026-03-19 已把统一批量框架扩展到 `audit / knowledge`：新增 `POST /api/v1/audit/bulk/export`、`POST /api/v1/knowledge/bulk/export`，`audit` 记录已补稳定 `id`，Web `/audit` 与 `/knowledge` 已接通当前页勾选、全选与批量 JSON 导出。
- 2026-03-19 已完成本地回归：`go test ./...`、`ruby scripts/validate_openapi.rb`、`cd web && npm run lint`、`cd web && npm run build` 全部通过。远端 `192.168.3.106` 尚未执行本轮 JumpServer runtime / connector lifecycle / audit+knowledge bulk export 的部署与联调验证。
- 2026-03-25 已开始全站中英双语收口：I18n 已切到 `react-i18next` + `web/src/locales/{zh-CN,en-US}.json` 单一资源源，`useI18n` 仅保留兼容壳；已覆盖全局布局、登录页、Sessions、Executions，以及 Providers / Channels / Skills / Identity 概览等主路径。`web` 本地 `build` 通过，`lint` 仅保留原有 hooks warnings；浏览器验收因当前环境缺少 `agent-browser` 可执行文件未能继续，待在具备该命令的环境中补做逐页验收。
- 2026-03-25 已完成 `192.168.3.9` 共享环境两条黄金场景的真实闭环收口：1) 修复 Postgres workflow 中 `web_chat / ops_api` 不自动收口的问题，新的 Web Chat 会话 `9e8dd6ab-c43d-4a9c-af96-b822a37301c9` 已实际走到 `resolved`，timeline 出现 `chat_answer_completed`，并通过 Inbox 收到 `ref_type=session`、`ref_id=9e8dd6ab-c43d-4a9c-af96-b822a37301c9` 的“会话已关闭”消息；2) automations manager 已新增最小 `RunNotifier` 桥接，自动化巡检 run 完成后会复用现有 trigger/channel/inbox 主链投递消息，远端手动 run `da9b9f73-520a-4f89-94c6-0b82d9810e0d` 已在 `/api/v1/inbox` 产出 `ref_type=automation_run` 的 Inbox 消息。期间还修复了 Web Chat fingerprint 过粗导致跨 host/session 误复用的问题（现包含 `user + host + service + message`），并把 `scripts/run_golden_scenario_{1,2}.sh` 改成严格校验本次 session/run 的 Inbox 关联消息。
- 2026-03-25 已补远端共享环境的 observability live validation 支撑：`scripts/seed_team_shared_fixtures.sh` 现在会生成 `fixtures/observability-http/api/v1/{alerts,rules}` 样本响应，`scripts/deploy_team_shared.sh` 会在远端拉起 `python3 -m http.server 8880` 作为只读 observability fixture server；随后已把 `observability-main` connector 指向 `http://127.0.0.1:8880`，并重新跑通 `scripts/ci/live-validate.sh`，结果为 `live-validate=passed`。当前共享环境仍存在历史 `telegram.send` failed outbox（`chat not found`），但不再阻塞 Inbox 默认通知链路与本次黄金场景验收。
- 2026-03-25 已完成黄金场景最后一轮收口：`internal/modules/workflow/service.go` 与 `internal/repo/postgres/workflow.go` 现在会按告警来源选择通知渠道，`web_chat / ops_api` 的诊断结果、策略消息、审批超时与能力审批消息改走 `in_app_inbox`，`telegram_chat` 与传统告警仍保留 Telegram；`internal/events/dispatcher.go` 的直接执行结果通知也已同步切到同一策略，并补了定向回归。重新部署到 `192.168.3.9` 后，用带时间戳的新 Web Chat 请求跑出会话 `de22fbea-f887-4dbf-b7e9-553a6690ff0c`，真实状态为 `resolved`，Inbox 命中 `ref_type=session` 关联消息，且未再制造新的 `telegram.send failed outbox`。
- 2026-03-25 已把 `observability-main` 的 team-shared runtime 基线固化到部署流程：`scripts/deploy_team_shared.sh` 默认远端改为 `192.168.3.9`，并在重启就绪后自动调用 `PUT /api/v1/config/connectors` 把 `deploy/team-shared/connectors.shared.yaml` 同步进 runtime DB。重新部署后，远端 `GET /api/v1/config/connectors` 返回的 `observability-main.config.values.base_url` 已稳定为 `http://127.0.0.1:8880`，`scripts/ci/live-validate.sh`、`scripts/run_golden_scenario_2.sh`、`scripts/run_golden_scenario_1.sh` 均再次通过，无需再手工修 runtime connectors state。
- 2026-03-25 已清理 `192.168.3.9` 上 5 条历史 failed outbox（均为 `web_chat -> telegram.send -> Bad Request: chat not found` 残留），使用 `POST /api/v1/outbox/bulk/delete` 批量删除，operator reason 为 `cleanup historical web_chat telegram failures after inbox-default routing`。清理后重新执行 `scripts/ci/smoke-remote.sh`，远端 hygiene 已回到 `failed_outbox=0 blocked_outbox=0`、`result=clean`。
- 2026-03-19 已继续把 execution connector 接回 workflow 主链路：内存版与 Postgres 版 `ApplyDiagnosis` 现在会优先选择第一个 enabled 且兼容当前 TARS major 的 `jumpserver_api` execution connector，并把 `connector_id / connector_type / connector_vendor / protocol / execution_mode` 写进 execution draft、审批结果和直接执行请求；未命中 connector 时仍回退到 `ssh`。
- 2026-03-19 已新增 Postgres migration `migrations/postgres/0002_execution_connector_metadata.sql`，为 `execution_requests` 补齐 `connector_id / connector_type / connector_vendor / protocol / execution_mode` 持久化字段；`loadSessionDetail / loadExecutionDetail / lockExecution` 已同步读出这些元数据。
- 2026-03-19 已把 metrics query path 从 HTTP handler 临时 provider 收口到 `Action` connector runtime：`/api/v1/connectors/{id}/metrics/query` 与后续 runtime 复用同一 `Action.QueryMetrics` 入口，`prometheus_http / victoriametrics_http` 现通过统一 query runtime 执行。
- 2026-03-19 已继续收口 connector lifecycle 与 runtime selector：`GET /api/v1/config/connectors` 现在返回真实 lifecycle state（含 `available_version / history / health_history / revisions / installed_at`），`PUT/import/upgrade` 统一执行兼容性校验，`metrics/query`、`execution/execute`、`health` 也会拒绝 disabled / incompatible connector；同时新增 `internal/modules/connectors/runtime_selection.go`，让 `workflow`、Postgres workflow 和 `Action` 共用默认 connector 选择逻辑，metrics / execution 在未显式给 `connector_id` 时会优先选择兼容的官方 connector，旧 VM / SSH 路径仅作为 fallback。
- 2026-03-19 已把 connector health 从派生状态推进为 runtime probe：`prometheus_http / victoriametrics_http` 走真实 `/api/v1/query?query=up` 探测，`jumpserver_api` 走 API 探活；runtime 成功/失败也会写回 `health_history`。Web `/connectors/:id` 已新增 `Run health check`、compatibility reasons、health history 和 revisions 展示。本地定向回归已通过：`go test ./internal/modules/connectors ./internal/modules/action ./internal/modules/workflow ./internal/repo/postgres ./internal/api/http`、`cd web && npm run lint`。
- 2026-03-19 已收敛 connector lifecycle 噪音：`SaveConfig` 不再因为每次普通保存都无条件追加 health history；`Upgrade()` 也不再重复追加与 `syncLifecycleState()` 重叠的 upgrade/revision 记录，只在纯配置更新且带 operator reason 时补 `update_plan` 语义。
- 2026-03-19 已修复 Connector 平台化验收中的 1 个 P1 + 2 个 P2：显式 `connector_id` 的 metrics runtime 调用失败不再静默回退到 legacy provider/stub；`upgrade / rollback` 后 lifecycle `health` 会先进入 `unknown / runtime health check required after connector change`，避免误导运维把兼容性派生状态当成 runtime probe 结果；`revisions` 现在会保留 `install -> upgrade -> rollback` 的多快照历史，不再因为回滚到旧版本覆盖最初安装快照。本地已补充 manager/service/http 回归并通过 `go test ./...`、`ruby scripts/validate_openapi.rb`、`bash scripts/check_mvp.sh`。
- 2026-03-19 已完成远端验证补充：重新部署修复版 `linux/amd64` 二进制到 `192.168.3.106` 后，使用 `victoriametrics-main` 做临时坏地址升级验证，确认显式 connector metrics 查询失败会直接返回 `HTTP 500`，不再静默回退到 legacy provider/stub；随后回滚到 `1.0.0`，验证 lifecycle `health` 先进入 `unknown / runtime health check required after connector change`，`revisions` 保留 `install -> upgrade -> rollback` 三类快照，再通过显式 `metrics/query` 和 `health` 恢复到 `healthy`，证明本次 P1/P2 修复在共享测试环境中已按预期生效。
- 2026-03-19 本轮仍仅完成本地代码与文档同步，远端 `192.168.3.106` 尚未部署 migration `0002`、也未完成“diagnosis -> approval -> JumpServer execute”正式联调验证。
- 2026-03-22 已继续推进 IAM 平台化第一版：修复并恢复 `/identity` 相关前端页，新增 IAM overview、`/identity/providers` 路由、provider-aware Login 体验、session inventory 展示与前端显隐保护；后端把 groups/roles/auth 相关 API 权限从 `platform.*`/`configs.write` 收口到 `groups.*`/`roles.*`/`auth.*`，同时保留 `ops-token` break-glass fallback。
- 2026-03-22 已补共享环境 Dex 方案基线：新增 [deploy/team-shared/dex.config.yaml](../deploy/team-shared/dex.config.yaml)，`access.shared.yaml` 增加 `dex-local` OIDC provider，`shared-test.env` 增加 `TARS_SERVER_PUBLIC_BASE_URL`，`deploy_team_shared.sh` 增加 Dex 容器拉起逻辑，用于在 `192.168.3.106` 上验证真实 redirect auth path。
- 2026-03-22 已继续推进受控自扩展第一版：`ExtensionCandidate` 已从内存态升级为 state file 持久化，默认走 `TARS_EXTENSIONS_STATE_PATH`；新增 review state 与 review history，当前支持 `pending / changes_requested / approved / rejected / imported`，且 import 必须先经过 `approved`。Web `/extensions` 已补审查按钮与历史展示。待办只剩远端 `192.168.3.106` 的 live candidate generate/validate/review/import 样本验收。
- 2026-03-23 已完成 Event Bus 基础版主链收口：新增 `internal/contracts/event_bus.go` 统一 `EventPublishRequest / EventEnvelope / DeliveryPolicy / DeliveryDecision(ack/retry/dead_letter)`；内存 workflow 与 PostgreSQL workflow store 都已实现 `PublishEvent / ClaimEvents / ResolveEvent / RecoverPendingEvents`；dispatcher 已改为消费 envelope 并返回 delivery result。当前已切换的真实链路为 `session.analyze_requested / session.closed / telegram.send`，其中 `telegram.send` 已统一到最多 3 次 + backoff 的 retry 语义。本地定向回归 `go test ./internal/events ./internal/modules/workflow ./internal/repo/postgres ./internal/app` 已通过；全量回归待本轮收尾时再次执行。
- 2026-03-23 已继续把“告警诊断闭环”收口成官方黄金路径 v1：后端新增 `golden_summary / notifications` 读模型表达层，`/api/v1/sessions`、`/api/v1/sessions/{id}`、`/api/v1/executions`、`/api/v1/executions/{id}` 都会返回面向值班视角的 headline / conclusion / risk / next action / notification reason；Web `/sessions`、`/sessions/:id`、`/executions`、`/executions/:id` 已切到结论前置展示。同时新增 `deploy/pilot/golden_path_alert_v1.json`、`deploy/pilot/golden_path_telegram_callback_v1.json` 与 `scripts/run_golden_path_replay.sh`，把官方黄金路径沉淀为固定 replay / acceptance 入口。

---

## 6. Frontend: Phase 1 & 2a 交互与 Web Console

| ID | 任务 | Owner | 开发状态 | 测试状态 | 阻塞项 | 备注 |
|----|------|-------|----------|----------|--------|------|
| `FE-1` | 梳理 Telegram 诊断消息字段 | `Frontend` | `已完成` | `不适用` | — | 见 `40-ux-telegram.md` |
| `FE-2` | 梳理审批消息字段和按钮语义 | `Frontend` | `已完成` | `不适用` | — | 见 `40-ux-telegram.md` |
| `FE-3` | 梳理执行结果消息字段 | `Frontend` | `已完成` | `不适用` | — | 见 `40-ux-telegram.md` |
| `FE-4` | 梳理 Web Console 页面和接口映射 | `Frontend` | `已完成` | `不适用` | — | 见 `40-web-console.md` |
| `FE-5` | 诊断消息模板设计 | `Frontend` | `未开始` | `未测试` | — | — |
| `FE-6` | 结果消息模板设计 | `Frontend` | `未开始` | `未测试` | — | — |
| `FE-7` | golden snapshot 样例维护 | `Frontend` | `已完成` | `已通过` | — | 已补 `deploy/pilot/golden_path_alert_v1.json`、`deploy/pilot/golden_path_telegram_callback_v1.json` 与 `scripts/run_golden_path_replay.sh`，并让 Web `/sessions`、`/executions` 使用真实 `golden_summary` 作为黄金样例基线 |
| `FE-8` | Telegram 联调支持 | `Frontend` | `未开始` | `未测试` | — | — |
| `FE-9` | 审批消息信息层级设计 | `Frontend` | `未开始` | `未测试` | — | — |
| `FE-10` | 按钮交互与文案约束 | `Frontend` | `未开始` | `未测试` | — | — |
| `FE-11` | 超时、转交、blocked 状态展示 | `Frontend` | `未开始` | `未测试` | — | — |
| `FE-12` | 审批回调联调 | `Frontend` | `未开始` | `未测试` | — | — |
| `FE-13` | 选型并初始化前端工程 | `Frontend` | `已完成` | `已通过` | — | Vite/React/TS 初始化完成，依赖清理完成 |
| `FE-14` | Console shell + 登录态占位 | `Frontend` | `已完成` | `已通过` | — | LoginView 与 App.tsx ProtectedRoute 已接通；2026-03-12 已补 Bearer Token 真校验与 Logout 清理 |
| `FE-15` | Session List 页面 | `Frontend` | `已完成` | `已通过` | — | SessionList 已改为真实读取 Ops API，支持状态过滤、统一分页/搜索/排序，以及当前页勾选后的批量 JSON 导出；2026-03-23 进一步切到 `golden_summary`，列表直接展示 headline / conclusion / risk / next action，减少原始字段噪音 |
| `FE-16` | Session Detail 页面 | `Frontend` | `已完成` | `已通过` | — | 已接真实 `/sessions/:id`，并支持 timeline/message/verification 展示与 markdown diagnosis 渲染；2026-03-12 为减小 bundle 移除了 `react-syntax-highlighter` 依赖的重渲染路径；本轮继续接通 `/sessions/:id/trace`，页面已可直接展示 `Audit Trail` 与 `Knowledge Trace`，不再需要额外查库确认对话/命令/结果沉淀情况；远端 `192.168.3.106` 已验证会话 `a7b4b899-baef-4de5-9127-37fb5e73deb8` 的 trace 返回 Telegram 对话/审批/结果审计和知识文档预览；2026-03-23 已新增 `Golden Path Snapshot` 与 `Notification Reasons`，把结论/风险/下一步/通知原因前置 |
| `FE-17` | Execution Detail 页面 | `Frontend` | `已完成` | `已通过` | — | 已接真实 `/executions/:id` + `/executions/:id/output`，不再依赖 mock 日志；2026-03-23 已新增 `Execution Golden Path`，前置展示 headline / approval / result / next action |
| `FE-18` | Outbox Console 页面 | `Frontend` | `已完成` | `已通过` | — | 已接真实 `/outbox` 列表与 replay 动作，保留 operator reason |
| `FE-19` | Reindex 操作入口与确认框 | `Frontend` | `已完成` | `已通过` | — | 已接真实 `/reindex/documents`，危险操作确认和错误提示可用 |
| `FE-20` | 错误态、空态、加载态统一 | `Frontend` | `已完成` | `已通过` | — | 2026-03-12 已补 Dashboard 实数化、`/api/v1/summary` 汇总读取以及页面级错误/空态/加载态对齐；前端构建已通过拆包和简化代码块渲染移除大 chunk 告警 |
| `FE-21` | Setup / Smoke 页面 | `Frontend` | `已完成` | `已通过` | — | 已新增 Setup / Smoke 入口，接通真实 `/api/v1/setup/status` 与 `/api/v1/smoke/alerts`，支持只读依赖状态、手工触发 smoke alert、最近 smoke 会话卡片，以及 Session List / Detail 中的 smoke 标记；2026-03-12 又补齐了 `TARS_WEB_DIST_DIR` 静态产物托管，远端 `192.168.3.106` 的 `/login` 和 `/setup` 现已可直接打开 |

---

## 7. 当前执行结论

截至 2026-03-12，当前更接近以下状态：

- `M0` 基线文档和初始骨架大体已完成
- `Sprint 1` 仍处于“骨架已起、真实依赖未接”的中前期
- `Workflow + Ops API + Postgres workflow store + VM/model fallback + diagnosis/approval/timeout/knowledge worker + SSH execution` 已形成可运行的 MVP 后端闭环
- 已验证真实链路：`webhook -> diagnosis -> pending_approval -> approve -> SSH execute -> resolved -> process restart -> state retained`
- 已验证失败链路：`webhook -> diagnosis -> pending_approval -> modify_approve(hostname && false) -> SSH execute -> failed`
- 已验证防重行为：重复 alert 复用同一 session 且不再追加 `alert_repeated`，重复 Telegram callback 被幂等吞掉
- 已验证知识沉淀链路：`session.closed -> documents / document_chunks / knowledge_records`；后续相似告警的 diagnosis stub message 已带 `knowledge:` 引用
- 当前主链路与试点交付物已收口完成；剩余工作转为下一阶段产品化增强，例如 secret manager、provider 健康历史持久化、MCP skill 外部源等

## 7. 外部待配置事项

### 7.1 Telegram

- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_WEBHOOK_SECRET` 如果使用 webhook 模式
- `TARS_TELEGRAM_POLLING_ENABLED=true` 如果使用 long polling
- `TARS_TELEGRAM_POLL_TIMEOUT`
- `TARS_TELEGRAM_POLL_INTERVAL`
- `TARS_TELEGRAM_BASE_URL` 若需要代理或自建网关
- 试点群 / 测试 chat_id
- 如果使用 webhook：需要 webhook 暴露地址和反向代理配置

### 7.2 测试机 `root@192.168.3.106`

- 已确认 SSH 可达，hostname=`openclaw`
- 架构为 `x86_64`，已安装 Docker；未发现 `go`、`psql` 预装
- 已通过本地交叉编译上传 Linux 二进制并完成 smoke test
- 已通过本地 smoke 实例直连该主机完成真实 SSH 执行验证
- 已使用 Docker 启动 `postgres:16` 容器、执行 migration，并跑通 Postgres workflow store
- smoke 结果：`webhook -> pending_approval -> approve -> resolved -> process restart -> state retained`
- 已在新二进制上验证 webhook payload-hash 幂等：同一 payload 连发两次返回同一 session，timeline 不追加 `alert_repeated`
- 已补测失败链路：`modify_approve` 为 `hostname && false` 后，execution/session 均进入 `failed`
- 已验证知识入库：resolved session 会在 Postgres 中生成 `documents`、`document_chunks`、`knowledge_records`
- 已验证知识检索：后续同主机/同服务告警的 diagnosis stub message 中出现 `knowledge:` 引用
- 已验证重建入口：删除已有 `document_chunks` 后，`POST /api/v1/reindex/documents` 能恢复 chunks
- 执行输出已落远端 spool file，并确认拿到 `hostname` / `uptime` / `systemctl status sshd`
- 已验证远端 VM smoke：`VictoriaMetrics + vmagent + node-exporter + vmalert` 可自触发 `TarsSmokeNodeUp`，且 TARS 兼容 Alertmanager 风格数组 payload
- 已验证审批路由：远端 smoke 中审批消息目标为 `sshd-owner`，并持久化 `approval_group=service_owner:sshd`

### 7.3 外部依赖联调状态

- 测试环境已完成 `Telegram bot + chat_id`、`VictoriaMetrics`、`OpenAI-compatible 模型网关` 三类联调
- 2026-03-13 已继续验证统一 Provider Registry 的真实多协议接入：`192.168.3.106` 当前已热加载 `lmstudio-local(primary=qwen/qwen3-4b-2507)`、`gemini-backup(assist=gemini-flash-lite-latest)` 和保留的 `env-primary`，远端 `/api/v1/config/providers/models` 与 `/api/v1/config/providers/check` 已分别验证 LM Studio 和 Gemini 返回可用；其中 LM Studio 的 `v1/models` 可返回本地模型列表，Gemini 可返回 `models/*` 列表并成功执行 availability check
- 2026-03-13 已确认本地小模型需要更宽的推理超时窗口：远端将 `TARS_MODEL_TIMEOUT` 从 `30s` 提高到 `90s` 后，Telegram 对话 `host=192.168.3.106 看系统负载` 会话 `e1dad29b-30c5-43cb-b1c6-9ff6a6a5ae17`、`ccc779f6-3c51-4665-a7d3-47fc33124fd6` 已由 `lmstudio / qwen/qwen3-4b-2507` 成功生成 `uptime && cat /proc/loadavg`，并命中 `whitelist -> direct_execute -> resolved`
- 2026-03-13 已修复一个高优先级信任边界问题：此前只要 Provider Registry 绑定了 `assist`，即使 `desensitization.local_llm_assist.enabled=false`，系统也会错误地把 raw context 发给辅助模型做敏感值检测；现已在 `v20` 修正为“只有显式开启 `local_llm_assist` 才允许辅助模型接触 raw context”，远端最新会话 `ccc779f6-3c51-4665-a7d3-47fc33124fd6` 的审计中已验证只出现 `chat_completions_send(lmstudio)`，不再出现 `local_llm_desensitization_detect_*`
- 2026-03-13 已修复 LM Studio 中文会话导致的知识沉淀 UTF-8 截断问题：知识模块中的 `makeSnippet / compactKnowledgeOutput / chunkText` 已改为合法 UTF-8 / rune 级截断；远端 `v20` 上最新会话 `ccc779f6-3c51-4665-a7d3-47fc33124fd6` 已验证 `session.closed -> knowledge ingest` 不再写入 `outbox_failed=invalid byte sequence for encoding "UTF8"`
- 2026-03-13 已完成一次真实 failover 演练：临时把 `lmstudio-local` primary 指到 `http://127.0.0.1:9` 强制失败，随后触发 Telegram 对话 `host=192.168.3.106 看系统负载`；远端会话 `083465f5-c87e-4475-8e54-9df2f267c75a` 已验证 `chat_completions_failover(from=lmstudio-local,to=gemini-backup)`，并由 `gemini-flash-lite-latest` 成功生成 `uptime && cat /proc/loadavg`，最终 `direct_execute -> resolved`。演练后 Provider Registry 已恢复为 `LM Studio 主 / Gemini 备`，两条 `/api/v1/config/providers/check` 再次返回 `available=true`
- 2026-03-13 已将 `/setup` 中的 provider 运行时状态显示改为按角色区分：primary 使用 `model_primary`，assist 使用 `model_assist`，避免 failover 成功后把辅助模型的成功结果误展示在主模型卡片上；远端已升级到 `v21`
- 当前 `/ops` 已支持三类运行时配置热加载：`authorization`、`approval routing`、`reasoning prompts`；授权策略默认提供引导式表单，并保留 Advanced YAML 模式
- 默认文档与样例配置仍不建议记录真实密钥或真实 endpoint 凭据；但自 2026-03-19 起，仓库中已新增 `deploy/team-shared/` 作为团队内部开发测试包，明确用于保存共享联调所需的内部测试凭据与地址
- 正式试点前仍建议准备：
  - 机器人 token 的轮换方案
  - 模型网关 API key 的 secret 管理方式
  - 试点群 / 服务 owner 路由的正式映射
- 2026-03-19 已重新盘点当前仓库，确认代码实现已经明显走在本 tracker 之前：除 MVP 主链路外，还已落地 `secrets inventory/update`、`connector templates/apply`、`dashboard health`、`connectors public registry + export + enable/disable`、`providers/models/check`、以及 `audit / knowledge` 的统一列表能力。为减少团队协作时的信息差，已补入 [docs/team_dev_test_environment.md](../docs/operations/team_dev_test_environment.md) 和 [deploy/team-shared/README.md](../deploy/team-shared/README.md)，并将当前共享测试环境所需的实际联调配置、Provider/Connector Registry、审批路由、授权策略、脱敏配置与内部测试凭据整理入库，供团队共同开发与测试。
- 2026-03-19 已补 JumpServer 连接器 health 语义回归：`POST /api/v1/connectors/{id}/health` 现在会优先返回 runtime probe 摘要，而不是退回通用的 “connector is enabled and compatible”；同时补了兼容性时间戳刷新和回归测试，避免控制面看起来“健康”但看不到真实协议探活结果。
- 2026-03-19 已完成一条走正常 workflow 的 JumpServer 正式验收样本：通过 Telegram 对话 `host=192.168.3.106 重启 sshd` 创建会话 `6e190d44-d6c2-496a-bb26-8999a3cfda97`，workflow 生成 execution `4d95b605-9003-4da4-80e1-3c78521d2965`，并选择 `jumpserver-main` 作为 execution connector。实际审批由真实 Telegram callback 消费后，链路成功走到 `pending_approval -> executing -> verifying -> resolved`，最终 `verification=success`。这条样本证明 JumpServer 官方执行连接器已经不仅可通过控制面 `execution/execute` 单独调用，也能在正常诊断/审批/执行/校验/trace/knowledge 主链路中端到端工作。
- 2026-03-19 已正式确认下一阶段最高优先级需求：把诊断链从“固定 enrich + 一次推理 + 可选执行”升级为“LLM 先产出 tool plan -> 平台调用 connector runtime -> 仅在必要时才进入 execution”。原因是当前真实需求已经明确出现“过去一小时机器负载”“优先查 VM/Prometheus 再决定是否执行”“希望返回图表/文件”这类问题。已新增设计文档 [90-design-tool-plan-diagnosis.md](../specs/90-design-tool-plan-diagnosis.md)，并同步把 `metrics.query_range`、图片/文件附件协议、监控优先于 JumpServer/SSH 的决策模型纳入 PRD/TSD/WBS。
- 2026-03-19 已完成 tool-plan 第一版主链路收口：diagnosis 从固定 enrich 改为 `planner -> execute tool steps -> final summarizer`；`tool_plan / attachments` 已持久化到 PostgreSQL `alert_sessions` 并通过 `/api/v1/sessions/{id}` 返回；`/setup/status` 已改成 connector-first 默认语义，legacy fallback 不再作为默认 host 和主展示来源。本地 `bash scripts/check_mvp.sh` 全绿；远端 `192.168.3.106` 已验证样本 `b25fd2bf-0fb5-40f0-883c-52c28e55c5be`（tool plan 与审计主路径可见）和 `c9ac7705-57ab-45b3-8342-96ebd4fd08a3`（tool plan + image/file attachments 主路径可见）。
- 2026-03-20 已继续收口 tool-plan 远端真实链路中的最后一段抖动：修复了 planner 对字符串 `priority` 的 JSON 兼容性、补充了显式 `connector_id` 的别名归一化（如 `prometheus -> prometheus-main`），并在 metrics tool step 中对纯指标名查询补齐 `instance/job` 选择器，避免模型给出 `node_load1` 这类过宽查询时落到无关序列或直接报 `connector not found`。对应本地回归已补到 `internal/modules/reasoning/service_test.go` 和 `internal/modules/action/service_test.go`。
- 2026-03-20 已继续补强 tool-plan 的 planner/runtime 收口：planner prompt 现在明确支持 `$steps.<step_id>.output...` / `$steps.<step_id>.input...` 多步引用；归一化层会在模型未写 `connector_id` 时，优先从 `tool_capabilities` 中为 `metrics.query_* / observability.query / delivery.query / connector.invoke_capability` 选择首个可调用 connector，避免 planner 主路径静默落回 legacy/stub；dispatcher 也新增了对 generic host execution hint 的硬收口，当 delivery/observability/metrics/knowledge 证据已足够回答请求时，会抑制诸如 `hostname && uptime`、`uptime && cat /proc/loadavg` 这类无关上机命令。对应本地回归已补到 `internal/modules/reasoning/service_test.go` 和 `internal/events/dispatcher_test.go`。
- 2026-03-20 已完成远端正式验收样本 `36adf308-ae66-41df-86de-650b80cefff8`：用户请求“过去一小时机器负载怎么样”后，planner 生成 `metrics.query_range`，runtime 选中 `prometheus-main`，成功查询 `node_load1{instance="127.0.0.1:9100"}` 最近 1 小时数据（`series_count=1, points=13`），finalizer 返回“负载趋势正常”总结且 `execution_hint` 为空，不再无谓掉回 `execution.run_command`。Session detail 已在 Postgres 主路径上持久化并返回 `tool_plan`、`metrics-range.json`、`metrics-range.png` 两个附件；trace 中可见 `tool_plan_generated / tool_plan_step_started / tool_plan_step_completed / chat_completions_send(finalizer)` 等完整审计事件。
- 2026-03-20 已继续补齐 tool-plan 第一版的平台化细节：planner 输入现在会带 `tool_capabilities / tool_capabilities_summary`，来源包括内置 `knowledge.search` 与 connector 能力目录；public discovery 也已返回 `tool_plan_capabilities`，用于控制台和外部系统发现当前系统可被 LLM 规划使用的能力。非标准 connector / MCP / skill 能力当前先统一映射为 `connector.invoke_capability`，后续再逐步补 runtime。
- 2026-03-20 已补齐 `metrics-range.png` 的可读性：当前时序图附件包含图表标题、查询/窗口副标题、X 轴起止时间和 Y 轴数值标签，解决“图片缺少坐标轴及标题”的可用性问题。本地已再次通过 `ruby scripts/validate_openapi.rb` 和 `bash scripts/check_mvp.sh`。
- 2026-03-20 已继续收口 `/setup` 的 connector-first 语义：`smokeDefaultHosts()` 现在优先返回 SSH allowlist/执行主机，再附加最近一次 smoke host，避免 metrics-only smoke 把 `127.0.0.1:9100` 之类的监控实例地址顶到默认 host；`runtimeSetupStatus()` 也不再在 primary connector 已选中时预填 `connector_runtime_failed`，而是把 fallback 仅作为 standby 信息展示，`component/component_runtime` 改为真实选中的 connector（例如 `prometheus-main`）。本地已补 `/api/v1/setup/status` HTTP 回归与前端文案收口，并通过 `go test ./internal/api/http`、`bash scripts/check_mvp.sh`；远端 `192.168.3.106` 已部署新版本并验证 `/api/v1/setup/status` 返回 `smoke_defaults.hosts=["192.168.3.106","127.0.0.1:9100"]`，同时 `metrics_runtime.component=prometheus-main` 且 `fallback_reason` 为空。

- 2026-03-21 已完成 Capability Runtime 系统：定义 `CapabilityRuntime` 接口（`Invoke(ctx, manifest, capabilityID, params)`），按 connector type（observability / delivery / mcp / skill）注册 stub runtime，bootstrap 时统一注入。
- 2026-03-21 已实现 `POST /api/v1/connectors/{id}/capabilities/invoke` 统一能力调用入口：接收 `capability_id / params / session_id / caller`，返回 `status / output / artifacts / metadata / error / runtime`。OpenAPI schema 已同步新增 `ConnectorInvokeCapabilityRequest` / `ConnectorInvokeCapabilityResponse`。
- 2026-03-21 已将 `observability.query` / `delivery.query` / `connector.invoke_capability` 三种 tool plan step 从"待实现"切到真实执行路径：统一调用 `action.InvokeCapability()`，按 connector type 自动解析目标连接器，回填 output / runtime / attachments / context，并发射审计事件。
- 2026-03-20 已继续收口 capability runtime 平台语义：planner 提示词与归一化层现已同时支持 `observability.query / delivery.query`；`mcp_tool / skill_source` 在运行时统一归一化到 `mcp / skill`，保证 capability catalog、runtime 选择与 `hard_deny.mcp_skill` 授权来源一致；`POST /api/v1/connectors/{id}/capabilities/invoke` 对高风险 capability 不再误报 `500`，而是按统一状态返回 `200 completed / 202 pending_approval / 403 denied`。本地已补 reasoning / action / connectors / http 回归并通过 `go test ./...`、`ruby scripts/validate_openapi.rb`、`bash scripts/check_mvp.sh`。
- 2026-03-20 已完成 capability invoke 远端实机验收：在 `192.168.3.106` 上通过 Ops 热更新临时注入 `skill-source-main(type=skill_source, capability=source.sync)`，验证 `POST /api/v1/connectors/skill-source-main/capabilities/invoke` 在默认策略下返回 `202 pending_approval`，加入 `hard_deny.mcp_skill: [source.sync]` 后返回 `403 denied`，并在恢复共享配置后确认 `/root/tars-team-shared/connectors.shared.yaml` 与 `/root/tars-team-shared/authorization.shared.yaml` 已回到基线；同时在远端本机 `127.0.0.1:18081` 验证 `GET /api/v1/platform/discovery` 已暴露 `connector.invoke_capability` for `skill-source-main`。这次验收也确认当前测试环境的 `18081` 仅监听 `127.0.0.1`，对外协作统一走 `8081`，公开入口若需验证 discovery/health 需通过 SSH 到远端本机访问。 
- 2026-03-20 已补一条真实 metrics 历史 + 附件 live 验收样本：由于共享测试环境当前基线中 `prometheus-main` 为 disabled，而 `victoriametrics-main` 使用真实 VM runtime，planner 在 ad-hoc smoke 中仍可能优先点名 `prometheus-main`。本次通过 Ops 临时把 `prometheus-main` 指到 VM 兼容接口 `http://127.0.0.1:8428`，成功触发会话 `f95fbeed-84d8-4680-b2c8-df5848a23800`，tool plan 执行 `metrics.query_range`（`connector_id=prometheus-main`）后返回 `series_count=1 / points=121`，summary 明确给出“过去一小时机器负载波动但整体可控”，且 Session Detail 已返回 `metrics-range.json`、`metrics-range.png` 两个附件，`executions=0`。验收结束后共享 connectors 配置已恢复原样。
- 2026-03-20 当前共享测试环境的一个已知联调特征：`victoriametrics-main` 是默认启用的真实 metrics connector，而 `prometheus-main` 在共享基线里是 disabled。对于需要“真实 metrics 时序 + 图片附件”的正式验收，如果 planner 仍点名 `prometheus-main`，可以临时把它指到 VM 兼容接口完成 smoke；这属于测试环境选择顺序问题，不代表 `metrics.query_range` runtime 本身回退到了 legacy/stub。
- 2026-03-20 已修复上述 capability catalog 选择问题：disabled / incompatible connector 不再进入 `tool_plan_capabilities`，同类能力会按“healthy 优先、真实 runtime 优先、stub 后置”的顺序排序。远端新样本 `a037a72c-a982-4c64-ba4a-6daab37daadb` 已确认在共享基线不改配置的情况下直接选择 `victoriametrics-main` 执行 `metrics.query_range`，返回 `attachments=2` 且 `executions=0`；远端 `platform/discovery` 也已确认 `metrics.query_range` 目录里不再暴露 disabled 的 `prometheus-main`。
- 2026-03-20 已新增 [scripts/validate_tool_plan_live.sh](../scripts/validate_tool_plan_live.sh)，用于团队快速验证当前共享基线下的 `/api/v1/setup/status`、显式 `metrics.query_range`、`skill-source-main` capability invoke（默认 `202 pending_approval`），并可通过 `TARS_VALIDATE_RUN_SMOKE=1` 触发一条 tool-plan smoke 样本。
- 2026-03-21 已为 ConnectorCapability 新增 `invocable` 字段：manifest model、DTO、TypeScript 类型和 OpenAPI schema 同步更新。标记 `invocable: true` 的能力可通过统一 capability runtime 调用。
- 2026-03-21 已实现能力级授权：授权 `Evaluator` 接口新增 `EvaluateCapability(CapabilityInput)` 方法；read-only 能力默认 `direct_execute`，非只读默认 `require_approval`，MCP/Skill 默认 `hard_deny_mcp_skill`。
- 2026-03-21 已更新 Web ConnectorDetail：capabilities 区域新增 `invocable` / `read-only` 徽章，新增 "Invoke Capability" 面板（选择能力、输入 params JSON、调用）。
- 2026-03-21 已更新 Web SessionDetail ToolPlanCard：当 tool plan step 携带 runtime metadata 时，展示 runtime name / selection / protocol。
- 2026-03-20 晚间已继续收口 execution 默认路径与操作体验：runtime 自动选择现在会优先使用健康的 connector；对 execution 场景而言，`jumpserver-main` 已作为共享基线中的默认 managed path 启用，但只有在 connector health 为 `healthy` 且对应 secret refs 已设置时才会真正接管执行，否则 workflow 会自动回退 `ssh`。同时 Telegram 诊断/审批/结果消息已改为更适合移动端扫读的精简模板，`/setup` 页面也改为优先展示 Telegram / model / metrics path / execution path 四张主卡片，并把 registry / approval / prompts 等次级配置收纳到 `Control Plane` 区域。
- 2026-03-20 晚间已完成 live 回归：新版本已部署到 `192.168.3.106`。`/api/v1/setup/status` 现在明确显示 `execution_runtime` 为 `fallback only: ssh`，原因是 `jumpserver-main` 当前在共享环境中仍缺少 `connector/jumpserver-main/access_key` 与 `connector/jumpserver-main/secret_key`，health probe 返回 `degraded`，因此不会被误选为主执行链。Telegram 诊断消息 live 样本 `9b939a62-5e79-4156-a3e5-72dfac01c4b8` 的 audit preview 已切到新短版 `[TARS] 诊断 ...`，而 Web `/setup` 页面也已在浏览器中验证为新的 connector-first 焦点布局。
- 2026-03-20 深夜已修复 tool-plan finalizer 的“证据失败仍过度下结论”问题：`FinalizeDiagnosis()` 现在会在关键系统工具（`metrics.query_* / observability.query / delivery.query / knowledge.search`）失败时，对包含“无直接关联 / no direct relation / unrelated”等结论的 summary 进行硬收口，改写为“已获取部分系统证据，但某关键查询失败，目前无法确认/给出确定结论”。本地已补回归 `TestFinalizeDiagnosisRewritesOverconfidentSummaryWhenCriticalEvidenceFails`。
- 2026-03-20 深夜已把共享测试基线的 `observability-main / delivery-main` 从不稳定外部依赖切到可复现 fixtures：
  - `observability-main` 继续使用真实 `log_file` runtime，但默认文件改为 `/root/tars-team-shared/fixtures/observability-main.log`
  - `delivery-main` 从 `delivery_github` 改为真实 `delivery_git`，默认仓库改为 `/root/tars-team-shared/fixtures/delivery-main-repo`
  - 新增 [scripts/seed_team_shared_fixtures.sh](../scripts/seed_team_shared_fixtures.sh) 用于生成这两份 fixture
  同时补了 observability log filtering：默认优先返回非 audit 业务日志，只有查询明确提到 `audit` 时才回退审计行。
- 2026-03-20 深夜已完成远端 live 回归：
  - `POST /api/v1/connectors/delivery-main/capabilities/invoke` 现在稳定返回本地 git fixture 中的 3 条 `api` 发布事实（deploy / rollback / baseline）
  - `POST /api/v1/connectors/observability-main/capabilities/invoke` 现在稳定返回 fixture 日志中的真实 `api` 错误与回滚记录，不再主要是 TARS 审计噪音
  - 新 smoke 样本 `a0739da8-10f3-4c49-94aa-b61adb7a24fd`（问题：“最近 api 报错和最近一次发布有关系吗”）已验证：
    - planner 执行 `observability.query -> delivery.query -> knowledge.search`
    - `executions = 0`
    - final summary 使用真实发布/观测证据得出“发布后立即出现错误并触发回滚”的结论
    - 不再追加无关的 generic host command
- 2026-03-21 已把“磁盘空间不足”官方剧本先沉成可导入 skill 包，而不是继续写死在主诊断链中：
  - 新增 [configs/marketplace/skills/disk-space-incident/package.yaml](../configs/marketplace/skills/disk-space-incident/package.yaml)
  - skill 包定义了 `DiskSpaceLow / DiskUsageHigh / DiskWillFillSoon` 触发、优先 `metrics.query_range -> knowledge.search -> execution.run_command` 的编排，以及 `predict_linear(node_filesystem_avail_bytes[1h], 4h)` 这类容量预测查询模板
  - 共享测试环境同步带上本地 marketplace index [deploy/team-shared/marketplace/index.yaml](../deploy/team-shared/marketplace/index.yaml) 和 `disk-space-incident.package.yaml`
- 2026-03-21 已把共享测试环境流程收成自动化脚本 [scripts/deploy_team_shared.sh](../scripts/deploy_team_shared.sh)，默认串起：
  - 构建 `linux/amd64` 二进制
  - 构建 Web dist
  - 同步 `deploy/team-shared` 与 marketplace 包
  - 生成 shared fixtures
  - 重启远端服务
  - 运行 live validation
- 2026-03-21 上午已把共享环境自动化真实跑通：
  - 修复了 `restart_remote()` 中 `pkill -f` 误杀当前 SSH 会话的问题
  - 增加了远端 `127.0.0.1:18081/healthz` readiness 等待，避免服务刚启动就进入验证导致假失败
  - `TARS_DEPLOY_SKIP_BUILD=1 TARS_DEPLOY_SKIP_WEB=1 bash scripts/deploy_team_shared.sh` 已在 `192.168.3.106` 成功完成同步、fixture 生成、服务重启与 live validation
  - 本轮 live validation 结果：
    - `metrics.query_range -> victoriametrics-main`
    - `skill-source-main capability invoke -> 202 pending_approval`
    - `execution_component -> jumpserver-main`
- 2026-03-21 已完成全量 Web UX 打磨：
  - **Session Detail**: 实现战况摘要（Evidence/Conclusion/Action）、工具路径图优化、附件预览与代码高亮。
  - **Setup / Smoke**: 实现 Path Visualization 可视化链路、修复建议（Access Hint）自动弹出、焦点卡片布局。
  - **Connector Detail**: 增强 Health Analysis 状态卡片、Health History 历史轨迹流、JumpServer 专项修复引导。
  - 视觉体系全面接入 Tailwind CSS，彻底移除旧有复杂内联样式，实现 100% 响应式。
  - 全量通过 `npm run lint`、`npm run build` 与 `check_mvp.sh` 校验。
- 2026-03-21 已完成 Web UX 验收补丁：
  - 修复 `/connectors` 页 Discovery snapshot 在 Ops/Web 入口 `:8081` 上错误读取 SPA HTML 的问题；`/api/v1/platform/discovery` 现已同时暴露给 Ops handler 和 public handler，前端统计卡片能返回真实 connector/capability 数。
  - 修复 `Connector Detail` 的 Runtime Smoke Verification 交互：metrics query 结果会回显到页面，execution smoke 也会在填写理由后真实调用并展示返回结果，不再是“按钮可点但结果丢失”的死交互。
- 2026-03-21 已将上述 Web UX 验收补丁通过 [scripts/deploy_team_shared.sh](../scripts/deploy_team_shared.sh) 部署到 `192.168.3.106` 并完成远端回归：
  - live validation 通过 `metrics.query_range / capability 202 / capability 403 / observability.query / delivery.query`
  - 浏览器实测 `/connectors` 的 Discovery snapshot 现显示 `6` 个 connectors、`8` 个 tool-plan capabilities
  - 浏览器实测 `/connectors/victoriametrics-main` 的 Runtime Smoke Verification 现能回显 query 结果 JSON，不再丢失返回值
- 2026-03-21 已明确下一阶段平台边界：Skill 不再只视为 `SkillDraft`、package 或 `skill_source` 附属能力，而应提升为与 Connector Registry 同级的平台系统。已新增 [docs/20-component-skills.md](../specs/20-component-skills.md)，并同步更新 PRD / TSD / WBS：
  - Skill 平台后续需要独立的 `Skill Registry / Skill CRUD / Skill Revision / Publish / Rollback / Skill Runtime`
  - `skill_source` 仅负责“从哪里导入”，不等于平台内部已安装且生效的 Skill
  - 官方 playbook（例如 `disk-space-incident`）的长期目标是切到 Skill Runtime 主路径，而不是继续保留在 reasoning 硬编码策略中
- 2026-03-21 已继续明确平台一级组件边界：除了 `Connectors` 与 `Skills`，`Providers / Channels / People` 也应作为同级平台系统推进，而不是长期停留在配置文件、局部模块或零散表结构层。已新增 [docs/10-platform-components.md](../specs/10-platform-components.md)，并同步更新 PRD / TSD / WBS：
  - Provider 后续需提升为一等控制面对象，补 `Provider Registry / Role Binding / Health History / 导入导出`
  - Channel 后续需提升为 `Channel Registry`，支持 Telegram / Web Chat / 后续多渠道的统一治理
  - People 后续需提升为 `People Registry`，统一 identity、角色、值班、审批与画像偏好
- 2026-03-21 已进一步补齐平台访问控制路线：新增 [docs/20-component-identity-access.md](../specs/20-component-identity-access.md)，把 `Users / Authentication / Authorization` 明确提升为与其他平台组件同级的系统能力，并同步更新 PRD / TSD / WBS：
  - Users：平台账号、组、identity link
  - Authentication：`local_token / oidc / oauth2 / ldap`、auth provider registry、session/callback/logout
  - Authorization：平台 RBAC、角色绑定、资源权限、审批权限边界
  - 实现原则明确为“优先复用成熟开源库，不重复实现 LDAP / OIDC / OAuth / RBAC 基础协议栈”
- 2026-03-21 已新增 [docs/30-strategy-automated-testing.md](../specs/30-strategy-automated-testing.md)，正式把自动化测试方案提升成平台级规划，并同步更新 TSD / WBS / 文档索引：
  - 当时先按更宽泛的 `L0-L5` 梯度规划，避免继续依赖单条 smoke 或零散手工回归
  - 明确较弱 agent 默认承担低风险层级与脚本化共享环境验证，高风险官方场景演练只交给强 agent 或人工把关
  - 将 `check_mvp.sh`、`deploy_team_shared.sh`、`validate_tool_plan_live.sh` 固定为共享环境自动化入口
  - 补入平台组件测试矩阵与标准测试输出模板，减少“会跑命令但不会给可信结论”的波动
- 2026-03-21 已完成 Skill 平台第一轮正式收口：
  - 后端新增 `internal/modules/skills`，包含 manifest、manager、runtime、revision/history/lifecycle 能力
  - HTTP 已落 `GET/POST /api/v1/skills`、`GET/PUT /api/v1/skills/{id}`、`enable/disable/promote/rollback/export`、`POST /api/v1/config/skills/import`
  - Web 已落 `/skills` 与 `/skills/:id`，支持列表检索、详情查看、lifecycle/revisions 展示
  - discovery 已补 `skill_manifest_version`、`/api/v1/skills*` docs 与 `skill.select` 能力目录
  - dispatcher 已优先走 `skill_selected -> skill_expanded_to_tool_plan -> executeToolPlan`，`disk-space-incident` 已切到 Skill Runtime 主路径，reasoning 中旧 disk 官方策略降为 fallback
  - 本地新增/补强回归：`internal/modules/skills/runtime_test.go`、`internal/events/dispatcher_test.go`、`internal/api/http/routes_test.go`、`internal/api/http/routes_split_test.go`
- 2026-03-21 已同步收口文档与契约：
  - OpenAPI `api/openapi/tars-mvp.yaml` 已补齐 Skill Registry / import endpoints 与相关 schema
  - [docs/40-web-console.md](../specs/40-web-console.md) 已补 `/skills`、`/skills/:id` 与 Skill Registry 交互约束
  - [tars_technical_design.md](tars_technical_design.md) 已补 Skill Registry + Skill Runtime 第一版实现边界
  - [tars_dev_tasks.md](tars_dev_tasks.md) 已把 `P4-1` ~ `P4-5` 标记为已完成/基本完成，`P4-6` 标记为部分完成
- 2026-03-21 已继续修 Skill 平台收口问题：
  - `skills.Rollback()` 在 `target_version` 留空时，现已真正回到“上一版 revision”，不再把当前版本当作默认回滚目标
  - Skill Registry 写接口现统一要求 `operator_reason` 必填：`create / update / enable / disable / promote / rollback / import`
  - OpenAPI 已同步把相关 `operator_reason` 标记为 required
  - 共享测试环境已补 `TARS_SKILLS_CONFIG_PATH=/root/tars-team-shared/skills.shared.yaml`，Skill Registry 不再停留在 marketplace-only 内存态
  - 团队文档已同步更新：共享环境现在应通过持久化 Skill Registry 暴露 `/api/v1/skills*` 与 discovery 的 `skill.select`
  - 同时修复了共享部署脚本遗漏：`scripts/deploy_team_shared.sh` 现会同步 `shared-test.env`，避免远端继续沿用旧环境变量导致 Skill Registry、后续 platform component 配置出现“本地已配、远端未生效”的假对齐
- 2026-03-21 已补齐 Skill Registry 的 Web 手工创建入口：
  - `/skills` 新增 `New Skill` 按钮与内嵌 draft 表单，不再只能导入 package 或直接调 API
  - 表单支持录入基础 metadata、triggers、planner summary、preferred tools 与 planner steps JSON
  - 创建成功后会直接跳转到 `/skills/:id`，后续可继续 promote / rollback / edit
- 2026-03-21 已补齐 Skill Registry 的 Web 手工编辑入口：
  - `/skills/:id` 新增 `Edit Skill` 按钮与内嵌表单，不再只能查看 manifest 后再回到 API/配置层修改
  - 页面可直接更新 display name、description、triggers、planner summary、preferred tools 与 planner steps JSON
  - 编辑保存统一调用 `PUT /api/v1/skills/{id}`，强制填写 `operator_reason`
  - `skill id` 在创建后保持不可变，页面中仅展示为只读字段，避免出现“表单允许改但后端禁止重命名”的假入口
  - 该入口已部署到 `192.168.3.106` 并完成远端浏览器验证：创建 `ui-editable-skill-20260321` draft 后，可直接在 `/skills/ui-editable-skill-20260321` 修改 description 并保存；Revision History 会新增 `skill_updated`
- 2026-03-21 已修复 Web 登录回归：
  - `/login` 现在对 `local_token` 优先走 break-glass 直连校验，`ops-token` 不再依赖 `/auth/login` session issuance 成功后才能进入系统
  - 登录页外层布局改成显式居中容器，不再依赖 Tailwind utility 是否生效，避免卡片平铺在页面上方
  - 已部署到 `192.168.3.106`，并用浏览器实际验证 `ops-token -> /` 登录成功
- 2026-03-21 已重构 Web 控制面的平台信息架构：
  - `Identity / Users / Groups / Roles / People` 现统一收口到 `web/src/pages/identity` 与 `/identity*` 路由下
  - `/channels` 与 `Providers` 已从旧的混合 identity/access 页面中拆出，成为与 `Skills / Connectors` 同级的一级页面入口（`/channels`、`/providers`）
  - `/auth /users /groups /roles /people` 保留兼容跳转，避免旧书签和现有操作手册失效
  - 侧边栏导航已同步调整，便于把 IAM 类页面与平台组件页面分开理解
  - 2026-03-21 已完成 Web 基础平台能力建设（Theme / I18N / Docs）：
  - **Theme (主题)**: 实现 Light/Dark/System 主题切换，接入 `useTheme` 全局上下文，持久化至 localStorage。
  - **I18N (国际化)**: 实现基础 `zh-CN`/`en-US` 切换，封装 `useI18n` Hook，并覆盖了顶部栏、侧边栏导航、登录页和部分通用状态词。
  - **Docs Center**: 在顶部栏新增统一入口下拉菜单，提供 User Guide、Admin Guide 和 GitHub 库快捷链接。
  - 修复了 `internal/modules/access/manager_test.go` 测试基线失败问题，补充了新增的默认角色。
  - 修复了 `OrgPage.tsx` 中的 React effect 闭包 setState 警告，保持代码质量健康。
  - 2026-03-21 下午已完成内置文档中心补全：
  - **内容扩展**: 接入了平台依赖、兼容性矩阵、配置自动化、身份访问规范等 4 篇高价值已有文档。
  - **新增文档**: 新撰写了 Web Console 指南、部署要求、兼容性矩阵等 3 篇双语文档。
  - **多语言增强**: 实现了文档中心的全面双语支持（zh-CN / en-US），内容随全局语言状态自动切换。
  - **快捷入口**: 补齐了顶部 Docs 下拉菜单的快捷链接，覆盖核心手册与系统架构文档。
  - **代码质量**: 修复了 `access/manager_test.go` 中的回归失败，并确保所有新增代码符合严格的 Lint 与 TS 校验。
- 2026-03-22 傍晚已完成 API 参考升级：
  - **Swagger 集成**: 将 `/docs/api-reference` 从静态 Markdown 升级为 `swagger-ui-react` 内嵌渲染。
  - **实时同步**: 直接读取并解析 `api/openapi/tars-mvp.yaml` 作为唯一事实源。
  - **搜索兼容**: Orama 搜索已兼容 Swagger 页面，并增加了基于字符串匹配的 CJK 检索补丁。
  - **快捷键修复**: 修复了 `Cmd+K` / `Ctrl+K` 全局快捷键在部分浏览器下的失效问题。
  - **UI 适配**: 针对深色模式为 Swagger UI 增加了 CSS 滤镜适配，确保视觉体验与 Web Shell 统一。
  - 全量通过 `npm run lint`、`npm run build` 与 `check_mvp.sh` 校验。

### 7.4 本地持久化验证阻塞
- `docker` 已安装，但当前 Docker daemon 未运行
- 本机 `127.0.0.1:5432` 当前无 PostgreSQL 服务
- 本地环境仍不适合直接跑 Postgres smoke，但该项已经在 `192.168.3.106` Docker 环境完成验证
- 若要在该机直接联调，需要二选一：
  - 安装运行环境（Go / Docker / PostgreSQL 客户端）
  - 或由本地交叉编译后上传二进制和配置

建议日常使用方式：

- 每日站会只更新本文档，不改 WBS 基线
- 每周评审时同步回写里程碑结论到 `tars_dev_tasks.md`
- 若任务拆分变化，先改 WBS；若仅状态变化，只改本文档
- 2026-03-19 已把 Post-MVP 的 4 个新方向正式整理成路线文档 [docs/91-roadmap-post-mvp.md](../specs/91-roadmap-post-mvp.md)：包括多层记忆系统、自我升级/自我扩展、Agent 交互范式扩展、MCP 暴露接口，并明确建议优先级为 `多层记忆 -> MCP 基础 -> Web Chat/交互扩展 -> 受控自我扩展 -> 有边界的自我升级`

- 2026-03-21 已把侧边栏 Identity 调整为可展开父级菜单，子项现为 Auth Providers / Users / Groups / Roles / People，不再和顶层平台组件平铺同级。
- 2026-03-21 已补共享环境 Access Registry 对齐：新增 [deploy/team-shared/access.shared.yaml](../deploy/team-shared/access.shared.yaml)，登记 `local_token` 与 `telegram-main`；`shared-test.env` 已接入 `TARS_ACCESS_CONFIG_PATH`。这样 `/channels` 与 `/identity` 不再只读取空的 in-memory access 默认配置，而会显示共享环境实际使用的 break-glass auth provider 和 Telegram channel。
- 2026-03-21 已继续补完平台控制面第一组管理页：`/identity` 已切到新的 `Auth Providers` 管理视图，`/identity/people`、`/channels`、`/providers` 均改成左侧 list + 右侧 detail/editor 结构，支持 `New / Edit / Enable-Disable / Detail`，不再停留在只读列表或仅启停状态。
- 2026-03-21 前端 API 已补齐对应 CRUD/详情能力：`web/src/lib/api/access.ts` 新增 `fetch/create/update` for `auth providers / people / channels / providers`，并把 `/api/v1/config/providers`、`/models`、`/check` 收口到平台控制面可直接复用的 API 层。
- 2026-03-21 已补共享表单基础设施：新增 [web/src/pages/identity/registry-utils.ts](../web/src/pages/identity/registry-utils.ts) 承载 `splitCSV / joinCSV / parseKeyValueText / formatKeyValueText`，并将 `StatusMessage / FieldHint / LabeledField / DetailHeader / EmptyDetailState / SplitLayout` 等骨架收敛进 [web/src/components/ui/](../web/src/components/ui)，用于统一 People / Channels / Providers / Auth Providers 四类页面的交互骨架。
- 2026-03-21 已补 Auth Provider secret 状态回显链路：后端 DTO/OpenAPI/Web types 新增 `client_secret_set`，因此 `/identity` 页面现在可以区分“local_token 模式 / secret 已配置 / secret 缺失”，并对 `client_secret` 保持写入型字段语义（留空保留现值）。
- 2026-03-21 本地回归已重新执行并通过：`cd web && npm run lint`、`cd web && npm run build`、`bash scripts/check_mvp.sh` 全绿；其中 `check_mvp.sh` 内的 `go test ./...`、`go build`、OpenAPI 校验、Web lint/build 均通过。
- 2026-03-21 本轮远端 `192.168.3.106` 尚未执行新的平台控制面部署与浏览器验收，因此 `/channels`、`/providers`、`/identity`、`/identity/people` 的 create/edit/enable-disable 真机验证仍待下一步完成；当前风险主要集中在 `Providers` 页对 `primary/assist` 绑定与 registry entry 同屏保存的交互语义，还需远端真实数据再验一轮。
- 2026-03-21 已继续收口这轮平台控制面问题：修复了 `New Provider` / `New Channel`（以及同类 `New Person` / `New Auth Provider`）在已有选中项时不会真正进入创建态的问题；当前用 `isCreating` 显式保护创建模式，避免 `load()` 自动把焦点抢回列表首项。
- 2026-03-21 已把 `Providers` 页的全局模型路由收敛为显式操作：Provider Entry 保存与 `Primary / Assist` 绑定保存现已拆开，绑定区单独要求 `operator_reason`，并提供 `Bind / Update / Clear` 动作，不再通过保存 provider entry 隐式改写全局路由。
- 2026-03-27 已完成 Ops IA refactor 文档/契约收口：统一改口径为 `/providers` 与 `/connectors` 承担日常高频 CRUD / binding / edit，`/identity*` 承担 IAM 日常管理，`/ops` 仅保留 raw config、import/export、diagnostics/repair、平台级高级控制与 Secrets Inventory；同时补齐 OpenAPI 的 `GET/PUT /api/v1/providers/bindings`、`POST /api/v1/connectors`、`PUT /api/v1/connectors/{connector_id}`。
- 2026-03-21 已继续统一 identity/control-plane 前端风格：以 [web/src/components/ui/](../web/src/components/ui) 为统一骨架层，`Users / Groups / Roles / Auth Providers / People / Channels / Providers` 现统一使用 SectionTitle、SummaryGrid、RegistrySidebar/Detail、StatusMessage 等模式。
- 2026-03-21 已补“条目超过 3 条时的可控展示”要求：`Users / Groups / People / Channels / Providers` 列表页现支持分页；`Roles` 目录、`Auth Providers` 列表，以及 `known users / available channels / fetched models / available groups` 等辅助项在数量超过 3 条时改为折叠展开，避免页面无限拉长。
- 2026-03-21 已补一轮浏览器级手工验收清单到 [docs/team_dev_test_environment.md](../docs/operations/team_dev_test_environment.md) 第 7.4 节，覆盖 `/identity`、`/identity/people`、`/channels`、`/providers` 的 create/edit/enable-disable 与提示文案检查点。
- 2026-03-21 已将上述新前端版本再次通过 [scripts/deploy_team_shared.sh](../scripts/deploy_team_shared.sh) 部署到 `192.168.3.106`；live validation 仍通过 `metrics.query_range / capability 202 / capability 403 / observability.query / delivery.query`。
- 2026-03-21 已清理上一轮远端真实 API 验证留下的 4 条临时样本：通过 `PUT /api/v1/config/auth` 移除 `ui-local-token-* / ui-person-* / ui-channel-*`，通过 `PUT /api/v1/config/providers` 移除 `ui-provider-*`，共享环境现已恢复到 `local_token + telegram-main + lmstudio/gemini/dashscope` 的基线状态。
- 2026-03-21 已继续推进第二轮全站样式统一：`/sessions`、`/connectors`、`/audit`、`/knowledge` 的筛选区与表格区现统一到 registry 风格的 toolbar/table/feedback 节奏，`/setup` 的 smoke 提交错误提示也已收口为统一 `StatusMessage`。
- 2026-03-21 已补首条 Playwright 级 control-plane smoke：新增 `web/playwright.config.ts`、`web/tests/control-plane.smoke.spec.ts` 与 `web/package.json` smoke scripts，默认直连 `192.168.3.106:8081`，通过真实 `/login -> /identity -> /channels -> /providers` UI 路径覆盖 create / edit / enable-disable / provider bindings，并在用例前后通过 `PUT /api/v1/config/auth`、`PUT /api/v1/config/providers` 清理 `pw-smoke-*` 样本。
- 2026-03-21 已重新校正文档口径：当前 control-plane 中凡后端要求 `operator_reason` 的写操作，前端仍需显式输入，不能静默代填；后台审计留痕是额外保障，不替代该输入要求。
- 2026-03-27 已进一步收紧一致性口径：`/identity/users`、`/identity/groups`、`/identity/roles`、`/identity/people`、`/identity/providers` 这 5 个 IAM 日常页可不要求用户显式输入 `operator_reason`，由后端推断/补默认审计原因；`/providers`、`/connectors` 及其他 control-plane 写接口若契约仍要求 audited reason fields，则继续显式输入，不因 IA 调整而放宽。
- 2026-03-21 已把三类后续平台治理能力正式纳入规划并写入文档：[30-strategy-platform-config-and-automation.md](../specs/30-strategy-platform-config-and-automation.md)。当前已明确支持方向包括：全量平台 bundle export/import、模块级 export/import、Automations / Scheduled Jobs 平台，以及 Skill 通过正式 `platform_action` / Registry API 受控创建和更新 `connector / channel / provider / people / skill` 等平台对象。
- 2026-03-21 已新增 [30-strategy-authorization-granularity.md](../specs/30-strategy-authorization-granularity.md)，把权限颗粒度正式收口为 `resource / action / capability / risk` 四层模型，并同步挂到 Identity & Access 规范、PRD、TSD、任务基线与文档索引。当前明确建议：不要直接按底层 HTTP endpoint 或粗粒度 `vm.allow` 建模，而应优先把 VM/监控类能力拆成 `metrics.query_instant / metrics.query_range / metrics.capacity_forecast` 等只读 capability，并为未来的静默/规则变更等写能力单独定义 `mutating / high_risk` capability。
- 2026-03-21 已进一步收紧该文档口径：VM 只是例子，细粒度权限控制应覆盖所有第三方系统接入。文档现已明确：监控、执行、交付、渠道、People/目录等外部系统都应优先按 `capability + risk` 建模，而不是只给系统级 allow/deny 开关。
- 2026-03-21 已把三项前端平台体验基线正式纳入文档：Web Console 后续必须支持 `light / dark / follow system` 三种主题模式、右上角统一 `Docs / 文档中心`（至少内置用户手册与管理员手册）、以及 `zh-CN / en-US` 双语支持。对应规范已同步到 `40-web-console.md`、PRD、TSD、任务基线和前端任务拆解，后续应按 Web Shell 能力统一实现，而不是由单页各自拼装。
- 2026-03-21 已补充 People 平台的正式定位：People 不再只表示 owner/oncall/审批通讯录，还应包含“人物事实层 + 动态画像层 + 偏好层”。文档现已明确：可基于对话记录、审批行为、编辑行为、反馈行为推断用户技术水平、回答深度偏好、语言偏好、证据展示偏好等画像，但这些推断必须带 `source / confidence`，且只影响回答风格与推荐顺序，不直接替代权限模型。
- 2026-03-22 已把一组新的平台与前端要求正式纳入文档：1) Channels/Telegram 后续应支持消息模板自定义（诊断/审批/执行结果模板、预览、测试发送、多语言模板）；2) Authentication 平台后续应补密码登录、验证码校验与 2FA/MFA；3) 文档中心后续必须支持搜索；4) 已新增 [10-platform-dependency-compatibility.md](../specs/10-platform-dependency-compatibility.md)，统一规划平台依赖管理、版本/漏洞状态、第三方系统兼容性列表以及 CPU/OS/memory/disk/network 等部署要求。
- 2026-03-22 已统一补充前端实现原则：Theme / Docs Center / I18N / 文档搜索这类基础能力，后续应优先评估并复用符合要求的成熟开源项目，避免重复造轮子；自研部分应聚焦 TARS 控制面壳整合、信息架构、业务文案和平台状态接缝。
- 2026-03-22 已明确文档中心的 `API Reference` 采用局部升级方案：继续保留在 Docs Center 统一目录中，但 `/docs/api-reference` 页面后续优先切换为内嵌 `Swagger UI` 渲染 `api/openapi/tars-mvp.yaml`；其它文档继续保持 Markdown 渲染，避免把整套文档中心改成 Swagger 风格。
- 2026-03-22 已把“平台受控自进化 / 自扩展”正式整理成设计基线 [90-design-self-evolving-platform.md](../specs/90-design-self-evolving-platform.md)。统一口径为：TARS 后续应具备通过 Skill 生成 Connector / Channel / Provider / Auth Provider / Skill / Docs 扩展草稿与 bundle 的能力，但这些对象必须继续走 `Extension Bundle -> validate/test/review -> Registry import -> enable` 的治理链；不将“自进化”定义成模型直接修改平台核心模块，也不允许绕过审批、审计、兼容性检查和回滚机制。
- 2026-03-22 已新增 [docs/30-strategy-third-party-integration.md](../specs/30-strategy-third-party-integration.md)，统一整理了第三方系统接入的 6 条主路径（Webhook / Connector Runtime / Channel / Provider/Auth Provider / MCP & Skill Source / Bundle & Marketplace），并对当前 TARS 方案的可行性给出判断：架构方向和短期落地性为高，可持续扩展性为高，但企业级治理成熟度仍取决于 Users/Auth/AuthZ、Provider/Channel 平台化、Secret/KMS、Compatibility/Trust 等底座继续补齐。
- 2026-03-22 已新增 [docs/20-component-logging.md](../specs/20-component-logging.md)，统一收口日志系统的当前实现与下一阶段规划：
  - 当前运行日志主链为 `slog JSON -> stdout`
  - 当前审计日志为 `slog + PostgreSQL.audit_logs`
  - 当前 Web 控制面能查看 `/audit` 与 `/sessions/:id/trace`，但还不能查看平台自身原始服务日志
  - 当前未内建 Loki / Elasticsearch / OpenSearch / SIEM / 文件轮转 sink
  - 后续日志平台方向明确为：统一日志字段、统一 sink 抽象、接正式检索后端、补 `/logs` Web 控制面、再补 retention / archive / SIEM export
- 2026-03-22 已新增 [docs/20-component-observability.md](../specs/20-component-observability.md)，把“内建轻量观测 + 标准化对外暴露 + 外部观测系统接入”正式收成设计基线：
  - TARS 后续应先具备一套小而美的 built-in observability，能看自身 metrics / logs / traces / events
  - 内建观测目标是帮助平台自诊断和值班排障，不是重做一个大而全的 Grafana / ELK / APM 产品
  - 平台后续应继续通过标准接口暴露：
    - `Prometheus /metrics`
    - structured JSON logs
    - `OTLP metrics / logs / traces`
  - 同时继续支持通过 Connector / Capability Runtime 接入外部观测系统，如 Prometheus / VictoriaMetrics / Loki / SkyWalking / OpenSearch 等
  - 推荐实现顺序已明确为：统一 built-in observability 模型 -> 轻量观测页面 -> `/logs` -> OTLP exporter -> 外部观测系统深化接入
- 2026-03-22 已把 `Channels` 的正式职责进一步补齐到平台文档：
  - Channel 不再只表示用户交互入口，也应承担正式的 outbound notification / delivery 能力
  - `站内信 / In-App Inbox` 已明确为最基础的内建通知渠道，不应在后续 Channel 规划中缺席
  - 后续应支持 skill 完成通知、automation 完成通知、审批请求、执行结果、告警摘要、周期性报告等主动送达
  - 建议统一收口的 channel capability 包括：
    - `channel.message.send`
    - `channel.message.preview`
    - `channel.template.render`
    - `channel.recipient.resolve`
    - `channel.delivery.status`
  - Skill / Automation 负责“为什么发、何时发、发给谁”，Channel 负责“通过哪个渠道送达”
- 2026-03-22 已把统一 `Trigger / Trigger Policy` 模型正式纳入规划并写入 [30-strategy-platform-config-and-automation.md](../specs/30-strategy-platform-config-and-automation.md) 与相关主文档：
  - 触发条件后续不应分散在 skill、channel、automation 私有字段里
  - 第一版统一支持：
    - `event`
    - `state`
    - `schedule`
    - `expression`
  - 建议统一字段包括：
    - `source`
    - `type`
    - `match`
    - `filters`
    - `cooldown`
    - `enabled`
    - `action`
  - 后续统一治理要求包括：
    - dedupe
    - rate limit
    - silence / cooldown
    - preview / dry-run
    - audit
    - enable / disable
- 2026-03-22 已把 `Web Chat = 第一方 Channel` 与多模态能力正式纳入规划：
  - `/chat` 后续不应只是临时聊天页，而应作为正式 Web Chat channel
  - Web / Channel 后续应逐步支持：
    - 文本
    - 图片
    - 语音
  - 具体能力不应写死在单个模型或单个页面里，而应由：
    - Channel capability
    - Provider / LLM capability
    - policy / risk boundary
    三者共同决定
  - 同时已明确：LLM 后续可以通过正式 `platform_action` 调平台自身能力，例如创建定时任务、发送通知或触发低风险 skill；但不得直接改底层配置，也不得绕过 Registry / 审批 / 授权边界
- 2026-03-22 已新增 [docs/30-strategy-async-eventing.md](../specs/30-strategy-async-eventing.md)，正式收口异步与消息总线演进策略：
  - 当前阶段继续以 `PostgreSQL outbox + worker` 作为主异步底座
  - 当前不建议过早直接引入重型 MQ
  - 但从现在起应补 `Event Bus` 抽象，统一 `Publish / Subscribe / Ack / Retry / DeadLetter`
  - 后续当 fan-out、consumer group、跨服务消费与高 replay/retention 成为刚需时，再正式接入消息系统
  - 当前优先评估顺序明确为：`NATS JetStream -> RabbitMQ -> Kafka`
- 2026-03-22 已完成消息模板管理台（FE-25）：新增 `/msg-templates` 页面，挂接到左侧导航（Channels 之后），支持 diagnosis / approval / execution_result 三类模板 × zh-CN / en-US 两种语言变体，共内置 6 条默认模板。功能包括：编辑主题与正文（`{{变量}}` 占位语法）、每类型完整变量白名单说明、基于示例数据的一键预览渲染、测试发送前端占位（后端 API 上线后开放）、Reset All to Defaults、localStorage 持久化（MVP 阶段；后端接口上线后对接远端持久化）。样式完全复用 registry-ui / registry-page 体系，与 Channels / Providers / Auth Providers 风格一致。本地验证：lint ✅、build ✅、check_mvp.sh 全 5 项 ✅；MsgTemplatesPage-xxx.js 已出现在产物中。
- 2026-03-22 已补 Identity & Access 现状口径到规范与任务基线：
  - 当前真实已实现的认证主路径是 `local_token`、`oidc`、`oauth2`
  - 当前已落地 `POST /api/v1/auth/login`、`GET /api/v1/auth/callback/{provider}`、`POST /api/v1/auth/logout`、`GET /api/v1/me`、`GET /api/v1/auth/sessions`
  - `ops-token` 仍保留为 break-glass 管理入口
  - 当前尚未实现：
    - 本地用户名密码登录
    - 验证码挑战/校验
    - 2FA / MFA
    - `POST /api/v1/auth/challenge`
    - `POST /api/v1/auth/verify`
    - `POST /api/v1/auth/mfa/verify`
  - `ldap` 当前主要停留在 provider/config/model 预留层，不应误判为完整目录登录已落地
- 2026-03-22 已完成 Identity Control Panel Phase 2 — Users / Groups / Roles 全功能管理页：
  - **后端新增**：`internal/modules/access/manager.go` 新增 `RoleBindings` struct 与 `GetRoleBindings(roleID)` 方法（扫描 users/groups 返回当前绑定）；`internal/api/dto/types.go` 新增 `RoleBindingsResponse`；`internal/api/http/access_handler.go` 新增 `GET /api/v1/roles/{id}/bindings` 路由；`api/openapi/tars-mvp.yaml` 同步新增 endpoint 与 schema。
  - **前端新增**：`web/src/lib/api/types.ts` 新增 `RoleBindingsResponse` 接口；`web/src/lib/api/access.ts` 新增 `fetchUser / updateUser / fetchGroup / updateGroup / fetchRole / updateRole / fetchRoleBindings` 七个 API 函数。
  - **`/identity/users`**：完整重写，支持左侧列表 + 右侧详情/编辑面板；点击用户 fetch detail，展示 username/display_name/email/status/source/group memberships/role bindings（chips 展示）；Edit 按钮进入内联编辑表单；enable/disable 按当前 IAM 日常页规则执行，不要求用户显式输入 `operator_reason`；选中项高亮轮廓；`PanelMode: 'none' | 'create' | 'detail' | 'edit'`。
  - **`/identity/groups`**：完整重写，同上架构；成员与角色采用 `ChipInput` 组件（输入后回车添加、× 移除）实现关系可视化编辑；enable/disable 按当前 IAM 日常页规则执行，不要求用户显式输入 `operator_reason`。
  - **`/identity/roles`**：完整重写，五模式面板 `'none' | 'create' | 'detail' | 'edit' | 'bind'`；detail 展示 permissions（chips）+ 当前 user/group bindings；edit 用 `ChipInput` 编辑 permissions；"Manage Bindings" 进入绑定面板，支持添加/清除 user/group bindings，按当前 IAM 日常页规则不要求用户显式输入 `operator_reason`；bindings 通过 `fetchRoleBindings()` 加载。
  - 全量本地校验通过：`go test ./...` ✅、`ruby scripts/validate_openapi.rb` ✅（133 operations / 610 refs）、`bash scripts/check_mvp.sh` 全 5 项 ✅（含 web lint/build）。
  - 已部署到 `192.168.3.106` 并完成 live validation：`GET /api/v1/roles/approver/bindings` 返回 `{role_id, user_ids, group_ids}`；roles/users/groups 列表 API 均可用；web build 产物 `UsersPage / GroupsPage / RolesPage` 已出现在 dist。
   - `40-web-console.md` 已同步更新 `/identity/users|groups|roles` 的接口列表与交互描述。
- 2026-03-22 已发现并修复角色绑定移除 Bug（`POST /api/v1/roles/{id}/bindings` 原只调用 `BindRole`，只追加不删除）：
  - **Root Cause**：`BindRole()` 只做 append；前端 `RolesPage` binding panel 提交的是"期望最终集合"，而不是增量 add/remove 列表，后端需要做 diff 并执行移除。
  - **修复**：`manager.go` 新增 `UnbindRole(roleID, userIDs, groupIDs)` 与 `SetRoleBindings(roleID, userIDs, groupIDs)`；`access_handler.go` 中 `POST bindings` 改调 `SetRoleBindings`；`RolesPage.tsx` hint 文案同步更新为"sets exact bindings"。
  - **说明**：`viewer` 是系统最低权限角色，`normalizeUser` 对无角色用户自动补 `viewer`，因此无法通过 `SetRoleBindings("viewer", [], [])` 清空 viewer 绑定——这是设计预期，不是 bug。
  - **新增测试**：`manager_test.go` 新增 `TestSetRoleBindingsAddAndRemove`（使用 `approver` 非默认角色）与 `TestUnbindRoleRemovesFromUser`，均通过。
  - **全量验证**：`go test ./...` ✅、`bash scripts/check_mvp.sh` 全 5 项 ✅。
  - **远端验证**：deploy 后 `POST /api/v1/roles/approver/bindings` 传入 `{"user_ids":["alice"],...}` 成功将 carol/bob 从 approver 直接绑定移除，carol 恢复为仅 `viewer`，alice 保留 approver ✅。
- 2026-03-22 已完成全部控制面页面 API 级远端验证（`192.168.3.106:8081`）：
  - **`/channels`**：list ✅、detail ✅、create ✅、update ✅、enable/disable ✅
  - **`/providers`**：list ✅、detail ✅、create（with operator_reason）✅、update ✅、enable/disable ✅
  - **`/identity`（Auth Providers）**：list ✅、detail ✅、create ✅、update ✅、enable/disable ✅
  - **`/identity/people`**：list ✅、detail ✅、create ✅、update ✅、enable/disable（status 字段）✅
  - **`/identity/users|groups|roles`**：已在上轮验证 ✅（本轮再次确认角色绑定移除修复后仍正常）
  - **发现**：POST /api/v1/users 和 /api/v1/providers 等写操作需要 `{"user":{...}}` / `{"provider":{...}}` 包装体，不能直接裸发字段——已记录，前端实现正确，API 测试脚本需注意。
- 2026-03-22 已完成认证增强基础版本地实现：
  - **后端**：`internal/modules/access/manager.go` 新增 `local_password`、pending auth flow、challenge state、`bcrypt` 密码校验、`TOTP` MFA 校验；`internal/api/http/access_handler.go` 新增 `POST /api/v1/auth/challenge`、`POST /api/v1/auth/verify`、`POST /api/v1/auth/mfa/verify`，并扩展 `POST /api/v1/auth/login` 支持 password/challenge/MFA 多阶段返回。
  - **前端**：`web/src/pages/ops/LoginView.tsx` 现已支持 `local_password` 的用户名/邮箱 + 密码、多步骤 challenge 与 TOTP MFA；继续兼容 `local_token` 与 OIDC/OAuth redirect。
  - **配置**：`deploy/team-shared/access.shared.yaml` 已新增 `local_password` provider 与共享测试用户；当前 challenge 渠道为 `builtin`，返回 `challenge_code` 仅用于 MVP 验收辅助。
  - **开源库**：引入 `golang.org/x/crypto/bcrypt` 与 `github.com/pquerna/otp/totp`，避免自研密码哈希与 TOTP。
  - **本地测试**：已补 auth flow 自动化测试，覆盖 password -> challenge -> mfa -> /me 主链。
- 2026-03-22 已把 Automations 平台基础版正式补入契约与文档基线：
  - **OpenAPI**：`api/openapi/tars-mvp.yaml` 已新增 `/api/v1/automations`、`/api/v1/automations/{id}`、`/enable`、`/disable`、`/run` 以及 `AutomationJob / AutomationRun / AutomationJobState` 等 schema。
  - **文档同步**：已更新 `30-strategy-platform-config-and-automation.md`、`10-platform-components.md`、`40-web-console.md`、`tars_prd.md`、`tars_technical_design.md`、`tars_dev_tasks.md`、`docs/README.md`、`docs/team_dev_test_environment.md`，把“基础版已落地什么、审批边界如何收口、当前未实现什么”写清楚。
  - **共享环境配置**：新增 `deploy/team-shared/automations.shared.yaml`，并在 `shared-test.env` 中增加 `TARS_AUTOMATIONS_CONFIG_PATH=/root/tars-team-shared/automations.shared.yaml`；`deploy/team-shared/README.md` 与团队共享环境手册已同步。
  - **当前真实边界**：Automation 只正式支持 `skill` 与 `connector_capability`；非只读 capability 与 skill 中的 `execution.run_command` 都会停在 `blocked`，不会因 schedule 或 run now 绕过审批。
   - **待完成**：将最新版本部署到 `192.168.3.106`，完成 create / edit / enable-disable / run now 与审批边界 live 验证，并把结果回填到本跟踪表。
- 2026-03-23 已完成 Web Console 主导航与全局入口重构（目标 B）：
  - **核心变更**：`AppLayout.tsx` 导航从平铺式改为可展开分组折叠式，共 4 个 NavGroup + 2 个顶层直达项。
  - **菜单结构**：
    - 顶层直达：总览（`/`）、环境与自检（`/setup`）
    - 运行中心（默认展开）：Sessions / Executions / Knowledge / Automations
    - 平台构件（默认展开）：Connectors / Skills / Extensions / Providers / Channels / Msg Templates
    - 身份与组织（按权限显示）：Identity Overview / Auth Providers / Users / Groups / Roles / People / Org
    - 系统与治理：Ops / Observability / Audit / Logs / Outbox
  - **工作台改名**：`nav.dashboard` 从 `Dashboard/仪表盘` 改为 `Overview/总览`
  - **下沉条目**：Logs、Outbox、Org、Msg Templates 已归入对应分组，不再与平台核心项平铺混排
  - **右上角 Inbox 入口**：新增 `InboxDropdown` 组件（Bell 图标），挂接在顶部 chrome Docs 之前；当前为占位面板，展示"平台内建通知渠道，即将上线"；为后续未读数角标、消息详情、跳转 session/execution 留好扩展空间
  - **右下角 Chat FAB**：新增 `ChatFab` 组件（固定定位，右下角 48px 圆形按钮），点击展开占位浮层；与 `/channels` 中的 `web_chat` 条目对应，为后续多模态能力留扩展空间
  - **I18n 键新增**：`nav.group.*`（5 个分组标题）、`nav.inbox`、`nav.chat`、`inbox.*`、`chat.*` 中英文全量覆盖
  - **CSS 新增**：`navDivider`（分隔线）、`navGroupActive`（分组激活状态）、`navGroupLabelText`（大写小号分组标题）
  - **不破坏**：路由、权限、Docs Center、Theme、I18N、登录链路均未改动
  - **本地验证**：`bash scripts/check_mvp.sh` 全 5 项 ✅（含 go test / go build / openapi / web lint / web build）
  - **远端验证**：已部署 `192.168.3.106:8081`；bundle 中确认包含 `nav.group.*` 5 组键、`inbox.placeholder`、`chat.placeholder`、`navDivider` CSS 类；HTTP 200 正常服务
- 2026-03-23 已完成 Channel Delivery 第二阶段 — Inbox / Trigger 后端 + 前端联通（模块 A/B/C 基础版）：
  - **后端新增（编译与测试均通过）**：
    - `internal/modules/inbox/manager.go`：完整 InboxManager（in-memory + Postgres 双模式），支持 Create / List / Get / MarkRead / MarkAllRead；`inbox_messages` 表已加入 `repo/postgres/schema.go`。
    - `internal/modules/trigger/manager.go`：完整 TriggerManager，内置 4 条默认触发器（execution.completed / execution.failed / approval.requested / session.closed）；支持 IsOnCooldown / MatchEnabled / RecordFired / Enable / Disable / Upsert；`triggers` 表已加入 schema。
- 2026-03-23 已完成控制面收口第一批前端增强（本地 lint 已通过）：
  - **Groups**：`/identity/groups` 已补前端 enable / disable，详情区新增状态/成员/角色概览。
  - **Automations**：`/automations` 已补摘要卡（enabled/failing/due soon）、scheduler error 展示，并修复 JSON 参数输入静默吞错问题。
  - **Connectors**：`/connectors/:id` 已接上 export yaml/json、upgrade、rollback、capability invoke。
  - **Org**：`/org` 的 Policy 页已支持 tenant policy 编辑保存，不再只是 raw JSON 查看。
  - **Msg Templates**：`Test Send` 已明确收口为 preview-first 提示，避免把未实现的后端能力误导成可用。
    - `internal/modules/channel/service.go`：CompositeChannelService，路由 `telegram` / `in_app_inbox` 两种 channel，暴露 `TelegramService()` 供其他模块调用。
    - `internal/events/trigger_worker.go`：TriggerWorker，实现 `FireEvent` / `FireApprovalRequested` / `FireExecutionCompleted` / `FireSessionClosed`；每次触发均检查 cooldown、投递对应 channel、写入 inbox。
    - `internal/events/dispatcher.go`：扩展接受 `*TriggerWorker` 可选参数，在 `dispatchSessionClosed` 和 `runImmediateExecution` 完成后自动调用 TriggerWorker。
    - `internal/app/bootstrap.go` / `workers.go`：注入 inboxManager / triggerManager / compositeSvc，创建 TriggerWorker 并传入 NewDispatcher。
    - `internal/api/http/inbox_handler.go`：`GET/POST /api/v1/inbox`、`POST /api/v1/inbox/mark-all-read`、`GET /api/v1/inbox/{id}`、`POST /api/v1/inbox/{id}/read`。
    - `internal/api/http/trigger_handler.go`：`GET/POST /api/v1/triggers`、`GET/PUT /api/v1/triggers/{id}`、`POST /api/v1/triggers/{id}/enable|disable`。
    - `internal/api/dto/types.go`：新增 `InboxMessage / InboxListResponse / InboxCreateRequest / TriggerDTO / TriggerListResponse / TriggerUpsertRequest`。
  - **前端新增**：
    - `web/src/lib/api/types.ts`：新增 `InboxMessage / InboxListResponse / TriggerDTO / TriggerListResponse / TriggerUpsertRequest` TypeScript 接口。
    - `web/src/lib/api/inbox.ts`：`listInbox / getInboxMessage / markInboxRead / markAllInboxRead` API 函数。
    - `web/src/lib/api/triggers.ts`：`listTriggers / getTrigger / upsertTrigger / updateTrigger / setTriggerEnabled` API 函数。
    - `web/src/components/layout/AppLayout.tsx` `InboxDropdown`：完全重写，接真实 `/api/v1/inbox` API；每 30 秒轮询未读数；Bell 按钮右上角显示红色角标（>99 时显示 99+）；下拉列表展示最新 20 条消息（subject/body/时间/已读状态）；点击未读消息自动标记已读；支持"全部已读"按钮；空状态保持原有占位图。
  - **OpenAPI**：`api/openapi/tars-mvp.yaml` 新增 `/api/v1/inbox`、`/api/v1/inbox/mark-all-read`、`/api/v1/inbox/{inbox_id}`、`/api/v1/triggers`、`/api/v1/triggers/{trigger_id}`、`/enable`、`/disable` 共 9 个 path；新增 `InboxMessage / InboxListResponse / InboxCreateRequest / Trigger / TriggerListResponse / TriggerUpsertRequest` 共 6 个 schema。
  - **本地验证**：`go test ./...` ✅、`bash scripts/check_mvp.sh` 全 5 项 ✅（go test / go build / openapi / web lint / web build）
  - **远端验证**：已部署 `192.168.3.106`；`deploy_team_shared.sh` live validation 全项通过；capability / observability / delivery 验证链路均正常。
- 2026-03-23 已完成 Channel Platform 第二阶段关键闭环 + Observability 最小治理（断裂点全清）：
  - **A. Trigger → Template → Delivery 真正打通**：
    - `internal/modules/trigger/manager.go`：`FireEvent` struct 新增 `TemplateData map[string]string` 字段，供渲染时传入变量。
    - `internal/events/trigger_worker.go`：`TriggerWorker` 注入 `*msgtpl.Manager`；`FireEvent()` 在执行每条匹配 trigger 时，若 `trigger.TemplateID != ""` 且 Manager 可用，则调用 `tpl.Render(evt.TemplateData)` 替换 subject/body；模板不存在时 fallback 到默认消息并 warn 日志，不中断投递。`NewTriggerWorker` 签名增加 `templates *msgtpl.Manager` 参数。
    - `internal/app/workers.go`：`StartWorkers()` 传入 `a.services.MsgTemplates` 到 `NewTriggerWorker`。
  - **B. skill/approval 事件接入 trigger runtime**：
    - `internal/events/dispatcher.go`：`runImmediateExecution()` 中当 `mutation.Status == "pending_approval"` 时调用 `d.triggers.FireApprovalRequested()`；`dispatchDiagnosis()` 中当 `forcedPlan != nil`（skill 路径）且 tool plan 执行完成后调用 `d.triggers.FireSkillEvent()`，根据步骤失败数自动判断 `on_skill_completed` / `on_skill_failed`。
    - `internal/events/trigger_worker.go`：新增 `FireSkillEvent()` 方法（fire `on_skill_completed` 或 `on_skill_failed`）及 `formatSkillEventMessage()` 辅助函数。
  - **C. 消息模板 PostgreSQL 持久化**：
    - `internal/modules/msgtpl/manager.go`：上轮已完成双模式（in-memory fallback + PostgreSQL）的实现；本轮完成收尾——`internal/app/bootstrap.go` 将 `msgtpl.NewManager()` 改为 `msgtpl.NewManager(db)`，正式接入数据库。
    - `internal/modules/msgtpl/manager_test.go`：所有 `NewManager()` 调用改为 `NewManager(nil)` 以匹配新签名。
  - **D. Metrics retention 最小落地（CurrentBytes 有真实值）**：
    - `internal/foundation/observability/store.go`：新增 `metricsSignalName = "metrics"` 常量、`Store.metricsPath` 字段；`NewStore()` 创建 `data/observability/metrics/` 目录及 `snapshots.jsonl` 路径；`RunRetention()` 每次执行时写一行 metrics snapshot（JSON 包含时间戳与 24h 统计），并对 metrics 文件执行 retention 治理；`Summary` 新增 `MetricsStorageBytes`；`ConfigStatus().Metrics.CurrentBytes` 改用 `s.summary.MetricsStorageBytes`（真实 on-disk 值），`FilePath` 指向快照文件。
  - **E. OTLP 最小可验证 exporter 路径**：
    - `internal/foundation/tracing/tracing.go` 完全重写：`Provider` 新增 `insecure` 字段；新增 `EnabledSignals() []string` 方法；新增 `Ping(ctx) error` 方法——当 OTLP Enabled 时对 endpoint 做 TCP dial（5 s 超时），返回 nil 表示可达，返回 error 表示不可达；此为零外部依赖的最小可验证 exporter 路径，升级到官方 OTLP SDK 时只需替换此方法体。
  - **本地验证**：`go test ./...` ✅、`bash scripts/check_mvp.sh` 全 5 项 ✅
  - **远端验证**：`deploy_team_shared.sh` live validation 全项通过（capability / observability / delivery 均正常）
- 2026-03-23 已完成“测试体系与本地 CI/CD 收口”第一阶段工程化：
  - **分层固化**：将测试体系从概念性 `L0-L5` 收口为更适合当前阶段执行的 `L0-L4`：`L0` 快速预检、`L1` 标准本地回归、`L2` 定向平台回归、`L3` 共享环境部署与 live validation、`L4` 手工/高成本验收。
  - **统一入口**：Makefile 已新增/收口 `pre-check / full-check / deploy-sync / deploy / smoke-remote / live-validate / web-smoke`，弱 agent 可按固定命令执行，不再依赖口头命令串。
  - **脚本分层**：新增 `scripts/ci/pre-check.sh`、`scripts/ci/full-check.sh`、`scripts/ci/smoke-remote.sh`、`scripts/ci/live-validate.sh`、`scripts/ci/web-smoke.sh`，负责组合既有成熟脚本并输出 CI 友好的步骤日志。
  - **底层复用**：保留 `scripts/check_mvp.sh`、`scripts/deploy_team_shared.sh`、`scripts/validate_openapi.rb`、`scripts/validate_tool_plan_live.sh` 作为事实标准，不重写核心逻辑。
  - **部署闭环**：`make deploy` 现默认串起 `deploy -> readiness/discovery/hygiene -> live validation`；`make deploy-sync` 保留为只同步/重启不验证入口。
  - **最小恢复**：`deploy_team_shared.sh` 现会在覆盖远端二进制前保留 `${REMOTE_BINARY}.prev`，作为共享环境失败时的最小恢复抓手。
  - **文档同步**：已同步更新测试策略、TSD、WBS、共享环境手册、README，并新增 GitHub 分层 workflow 草案占位结构，作为后续迁移基础。
- 2026-03-23 已将“按场景重排优先级”的执行策略正式写入 [tars_dev_tasks.md](tars_dev_tasks.md)：
  - 当前阶段不再默认按模块横向铺开，而改为围绕 1 到 2 条黄金场景收口。
  - 第一优先场景明确为“告警诊断闭环”：`告警 -> 会话/证据 -> 诊断/技能 -> 审批/执行 -> 结果通知`。
  - 第二优先场景明确为“定时巡检闭环”：`自动化触发 -> skill/检查执行 -> 结果沉淀 -> Inbox / Telegram 通知`。
  - 后续模块任务需说明其直接服务哪条场景链路；与两条优先场景无直接关系的任务默认降级排期。
  - 当前阶段的 UX 与测试也按场景收口，优先打造官方黄金路径与回放集，而不是继续平铺零散模块能力。
- 2026-03-23 已完成"控制面收口"第二轮改动（Msg Templates / Connectors / Extensions）：
  - **后端新增两个 Msg Templates 端点**：
    - `POST /api/v1/msg-templates/{id}/render`：接收可选 `vars` map，调用 `tpl.Render()` 做服务端变量替换，返回 `{ template_id, subject, body }`。
    - `GET /api/v1/msg-templates/{id}/export?format=json|yaml`：以 `Content-Disposition: attachment` 方式导出模板为 JSON 或 YAML 文件。
    - 两端点均已同步更新 `api/openapi/tars-mvp.yaml`（OpenAPI 168 operations，validate_openapi.rb ✅）。
  - **前端 MsgTemplates（FE-25）**：
    - `web/src/lib/api/msgtpl.ts` 新增 `renderMsgTemplate()` 和 `exportMsgTemplate()`。
    - `MsgTemplatesPage` Preview 按钮改为优先调用服务端 render（fallback 到本地 SAMPLE_DATA 渲染）；DetailHeader actions 区新增 "Export JSON" 和 "Export YAML" 两个下载按钮。
  - **前端 ConnectorDetail**：新增 "Template Bootstrap" section，调用 `fetchConnectorTemplates()` 拉取适用于当前 connector 的模板列表，展示 name/description/values 预览，"Apply Template" 按钮调用 `applyConnectorTemplate()` 并 invalidate query 刷新。
  - **前端 Extensions**：从单列卡片式彻底重构为 master-detail 分栏；左侧候选列表（状态/review 状态双 badge、搜索过滤）；右侧选中候选完整 detail（Skill Metadata、Governance Policy、Validation Report 含错误/警告列表、Docs Assets、Tests、Review History 时间线）；动作按钮（Validate / Approve / Request Changes / Reject / Import）统一收口到 header 操作区，Import 按钮在 `valid && review_state === 'approved'` 时才激活；Composer 改为独立可折叠面板。
  - **本地验证**：`go test ./...` ✅、`npm run lint` ✅、`npm run build` ✅、`bash scripts/check_mvp.sh` 全 5 项 ✅、`ruby scripts/validate_openapi.rb` ✅
- 2026-03-24 已完成前端设计系统统一与全站页面迁移（FE-27）：
  - **Phase 1 — 基础 UI 组件**：在 `web/src/components/ui/` 下新建 8 个 shadcn/ui 风格基础组件：`badge.tsx`（语义变体 success/warning/danger/info/muted）、`separator.tsx`、`alert.tsx`（语义变体）、`select.tsx`（带 chevron 原生下拉）、`tooltip.tsx`、`dialog.tsx`（基于 React state，无第三方依赖）、`sheet.tsx`（侧边抽屉）、`dropdown-menu.tsx`（基于 React state）。
  - **Phase 2 — AppLayout 重构**：`AppLayout.tsx` 使用新组件重写；新增侧边栏折叠/展开切换；右上角 Inbox Bell 使用标准 `Badge` 组件显示未读数；`AppLayout.module.css` 补充 `.sidebarCollapsed` / `.iconNavItem` / `.iconNavItemActive` 样式。
  - **Phase 3 — 统一公共组件层**：新建 `web/src/components/ui/page-components.tsx`，沉淀 20+ 控制面公共组件：`PageShell`（替代旧 `RegistryLayout`）、`PageHeader`、`StatusBadge`（`{status:string}` 语义 API）、`ActiveBadge`（兼容旧 `{active:boolean,label:string}` API）、`SummaryGrid`/`StatCard`、`EmptyState`、`SearchBar`、`FilterBar`、`SplitLayout`、`RegistrySidebar`/`RegistryDetail`、`Panel`、`CardRow`、`SectionTitle`、`InlineStatus`（替代旧 `StatusMessage`，接受 `{type,message}`）、`CreateButton`、`LabeledField`、`FieldHint`、`DetailHeader`、`EmptyDetailState`、`RegistryCard`、`RegistryPanel`、`CollapsibleRegistryList`、`ListPagination`。
  - **Phase 4 — 全站页面迁移**：共 20 个页面完成导入迁移，从旧 `registry-ui.tsx`/`registry-page.tsx`/`RegistryLayout.tsx` 重定向到 `page-components.tsx`。`SkillsList.tsx` 完成完整 UI 层重写，从原始内联样式彻底重构为新组件体系。`SessionList.tsx` 与 `ConnectorsList.tsx` 升级为 `PageShell`/`FilterBar`/`StatusBadge`/`SummaryGrid`/`InlineStatus` 新结构。
  - **遗留文件**：`registry-ui.tsx`、`registry-page.tsx`、`RegistryLayout.tsx` 已保留（不删除），但不再被任何页面直接导入，可按需清理。
  - **验证**：`npm run lint` ✅、`npm run build` ✅（仅预有的 swagger chunk 大小告警，非本轮引入）
  - **文档同步**：`40-web-console.md` 已新增 §1.4C 前端设计系统重构章节；`tars_frontend_tasks.md` 已新增 FE-27 条目。
- 2026-03-24 已完成 FE-28 — 升级为真正的 Radix UI / shadcn/ui 标准组件：
  - **安装新 Radix 包**：`@radix-ui/react-select`、`@radix-ui/react-tooltip`、`@radix-ui/react-dropdown-menu`、`@radix-ui/react-separator`、`@radix-ui/react-scroll-area`、`@radix-ui/react-popover`、`@radix-ui/react-alert-dialog`、`@radix-ui/react-tabs`、`sonner`。
  - **重写为 Radix 真实实现**：`select.tsx`（含 NativeSelect 别名）、`dialog.tsx`（Radix Dialog）、`sheet.tsx`（基于 Radix Dialog 的侧边抽屉，含 top/bottom/left/right 四向）、`tooltip.tsx`（TooltipProvider + Tooltip + TooltipTrigger + TooltipContent）、`dropdown-menu.tsx`（含 CheckboxItem/RadioItem/SubTrigger/SubContent 全套）、`separator.tsx`（Radix Separator）。
  - **新增组件**：`command.tsx`（cmdk 封装，含 CommandDialog/Input/List/Group/Item 等）、`sonner.tsx`（Sonner toast，集成 TARS 主题系统）。
  - **Tailwind 配置升级**：`tailwind.config.js` 在根层级暴露标准 shadcn HSL token（`bg-primary`、`bg-muted`、`bg-accent`、`bg-popover` 等）；TARS amber 品牌色迁移至 `tars-primary`；`shadcn.*` 命名空间保留向后兼容。
  - **Button / Badge 更新**：Button 使用标准 shadcn token，`primary` variant 保留向后兼容别名（实现为 `bg-tars-primary`）；Badge 使用标准 success/warning/danger/info/muted 语义色（`border-success/30 bg-success/10 text-success` 等）。
  - **AppLayout.tsx 升级**：Docs Center 下拉菜单从手写 `position: absolute` div 替换为真正的 Radix `DropdownMenu`（自动处理定位、键盘导航、Portal 层级、点击外部关闭）；`InboxDropdown` 同样升级为 Radix `DropdownMenu`，移除手动 `useRef`/`mousedown` 监听器；移除已废弃的 `DocCategoryTitle` 和 `DocLink` 子组件。
  - **page-components.tsx**：`FilterBar` 中的 `Select` 改为 `NativeSelect`（原生 `<select>` 封装，适合简单筛选场景）。
  - **验证**：`npm run lint` ✅、`npm run build` ✅。
  - **文档同步**：`tars_frontend_tasks.md` 已新增 FE-28 条目。
- 2026-03-24 已完成 FE-29 Phase 1 — 全站 UI/UX 重构（设计系统清理 + badge 迁移）：
  - **平台全貌理解**：通读 PRD / TSD / 5 份领域 specs / `40-web-console.md`（总计 5000+ 行），建立完整平台认知图谱。
  - **设计系统生成**：通过 ui-ux-pro-max 工具生成 TARS 平台配色、字体、间距、动效规范。
  - **card.tsx 重写**：标准 shadcn Card（Card/CardHeader/CardTitle/CardDescription/CardContent/CardFooter）；旧自定义组件迁移至 `card-legacy.tsx` 并通过 re-export 保持向后兼容。
  - **死代码删除**：`registry-ui.tsx` + `registry-page.tsx`（确认 0 消费者后删除）。
  - **tailwind.config.js 清理**：删除 `shadcn.*` aliases（确认 0 消费者）。
  - **index.css 清理**：删除 `.badge-*` 全套 CSS 类；删除与 Tailwind 冲突的 `.text-primary`/`.bg-primary` 等颜色工具类；`.glass-card:hover` 改为简洁背景变化，新增 `.glass-card-interactive:hover` 作为 opt-in 完整悬浮效果。
  - **badge-* 消费者全量迁移**：8 个文件共 11 处 badge-* 用法全部迁移至 shadcn `<Badge>`/`<StatusBadge>`（ConnectorDetail / SessionDetail / DocsView / SkillDetail / ExecutionList / ExecutionDetail / OutboxConsole / SetupSmokeView）。
  - **清理无用导入**：移除 `getStatusBadgeStyle`（SessionDetail）、`clsx`（SkillDetail）等废弃导入。
  - **`getStatusBadgeStyle()` 标记 `@deprecated`**：定义保留但消费者已归零。
  - **文档同步**：`40-web-console.md` 新增 §6 设计系统与 UI/UX 规范；`tars_frontend_tasks.md` 新增 FE-29 条目。
  - **验证**：`npm run lint` ✅、`npm run build` ✅。
- 2026-03-24 已完成 FE-29 Phase 2 — 全站 UI/UX 重构（遗留 CSS 全量迁移）：
  - **btn→Button 全量迁移验证**：`npm run lint` 发现 ChannelsPage.tsx 遗留 clsx 无用导入，已修复；`npm run build` 通过，0 错误。
  - **删除 .btn CSS 定义**：index.css 中 .btn / .btn-primary / .btn-secondary 全套 CSS（60 行）已删除，0 消费者。
  - **新增 textarea.tsx**：标准 shadcn Textarea 组件（forwardRef、cn()、ring-offset 焦点样式）。
  - **input-field→Input/Textarea/NativeSelect 全量迁移**：21 个文件共 ~158 处 input-field 用法全部消除——`<input>` → shadcn `<Input>`、`<textarea>` → shadcn `<Textarea>`、`<select>` → `<NativeSelect>`（来自 select.tsx）。最大文件：OpsActionView.tsx（38 处）、AuthProvidersPage.tsx（28 处）、OrgPage.tsx（22 处）、AutomationsPage.tsx（15 处）、PeoplePage.tsx（11 处）。
  - **删除 .input-field CSS 定义**：index.css 中 .input-field 及 [data-theme="light"] .input-field / .input-field:focus（23 行）已删除，0 消费者。
  - **registry-* CSS 全量迁移**：7 个文件共 ~15 处 registry-* CSS 类用法迁移至 Tailwind 内联——`registry-split-layout` → grid 列模板、`registry-toolbar` → flex 布局、`registry-table-wrap` → overflow-x-auto + rounded-xl + border、`registry-chip` → inline-flex pill 样式、`registry-feedback` → text-center p-12。`registry-table` 保留为 CSS 类（仅 thead/th/td/tbody tr 子选择器无法用 Tailwind 表达），同时在 `<table>` 上叠加 Tailwind 基础样式。
  - **删除无消费者 registry-* CSS 定义**：index.css 中 .registry-split-layout / .registry-summary-grid / .registry-sidebar / .registry-detail / .registry-chip-list / .registry-chip / .registry-collapsible-block / .registry-toolbar / .registry-table-wrap / .registry-table（基础属性）/ .registry-feedback 共 11 条规则已删除。仅保留 .registry-table 子选择器（5 条规则）。
  - **CSS 产物瘦身**：71.21 KB → 69.93 KB（-1.28 KB gzip 后 -0.22 KB）。
  - **遗留 CSS 类清零情况**：`btn` ✅ 0 消费者、`btn-primary` ✅ 0 消费者、`btn-secondary` ✅ 0 消费者、`input-field` ✅ 0 消费者、`badge-*` ✅ 0 消费者、`registry-split-layout` ✅ 0 消费者、`registry-toolbar` ✅ 0 消费者、`registry-table-wrap` ✅ 0 消费者、`registry-chip` ✅ 0 消费者、`registry-feedback` ✅ 0 消费者。
  - **验证**：`npm run lint` ✅、`npm run build` ✅。
   - **待排期**：glass-card 清理（~28 文件）、AppLayout 重构、核心页面逐页打磨。
- 2026-03-24 已完成 FE-29 Phase 3A — AppLayout / Breadcrumbs / GlobalSearch 重构：
  - **AppLayout.tsx**：彻底移除 `AppLayout.module.css` 依赖，全面改用 Tailwind classes；移动端侧边栏改用 shadcn `<Sheet side="left">`，导航键 `key={location.pathname}` 关闭时自动重置；桌面侧边栏新增折叠/展开按钮（`PanelLeftClose`/`PanelLeftOpen` 图标），折叠状态仅显示图标（60px 宽），配合 shadcn `<Tooltip>` 在折叠时展示菜单项标签；提取 `SidebarNav` 共享组件，桌面与移动端共用同一导航结构；`TooltipProvider` 包裹 `AppLayoutContent`，保证全局 Tooltip 可用。
  - **AppLayout.module.css**：已删除（0 引用）。
  - **Breadcrumbs.tsx**：移除全部 `style={}` 内联样式，改用纯 Tailwind；自定义 Portal dropdown 替换为 Radix `DropdownMenu`；移除 `Search`/`X` 过滤输入框（简化交互）；移除 `useRef`/`useState`/`useEffect` 用于 click-outside 的全部代码；移除 `LayoutTemplate` 导入。
  - **GlobalSearch.tsx**：移除页面内 `<style>` 注入块（cmdk 样式）；cmdk 相关样式（`[cmdk-*]` 选择器）统一迁移至 `index.css`。
  - **index.css**：新增 cmdk 样式区块；删除 `.nav-item-active`（CSS Modules 遗留产物，0 消费者）。
  - **验证**：`npm run lint` ✅、`npm run build` ✅（CSS 产物 67.33 KB）。
- 2026-03-24 已完成 FE-29 Phase 3B — 核心页面无障碍与语义化修复（LoginView / page-components / SessionList / ExecutionList / DashboardView）：
  - **LoginView.tsx**：6 处 `style={}` 内联样式全部迁移至 Tailwind；表单 `<label>` 补齐 `htmlFor` + 对应 `id`（屏幕阅读器可用）；语言切换按钮补 `aria-label`；状态/错误消息补 `role="status"`/`role="alert"`。
  - **page-components.tsx**：`SearchBar` — `border-white/10` → `border-[var(--border-color)]`，input/clear 按钮补 `aria-label`；`FilterBar` — 移除多余包裹 `<div style={{ width }}>`，NativeSelect 补 `aria-label`；`SplitLayout` — 响应式布局 `grid-cols-1 lg:grid-cols-[var(--split-sidebar)_1fr]`；`CardRow` — 补 `role="button"`/`tabIndex={0}`/`onKeyDown`，`border-white/5` → `border-[var(--border-color)]`，仅有 onClick 时才加 `cursor-pointer`。
  - **SessionList.tsx**：表格外层冗余类清理；全选复选框补 `aria-label="Select all sessions"`；行级复选框补 `aria-label="Select session {id}"`。
  - **ExecutionList.tsx**：移除 `<div>` 冗余/冲突类；全选复选框补 `aria-label="Select all executions"`；三个 `NativeSelect` 分别补 `aria-label`（状态过滤/排序字段/排序方向）；行级复选框补 `aria-label="Select execution {id}"`。
  - **DashboardView.tsx**：全站 `border-white/5` 和 `border-white/10` 替换为语义化 `border-border`（6 处）；字体尺寸下限统一到 12px：`text-[0.7rem]`/`text-[0.65rem]` 全部提升为 `text-xs`（4 处）。
  - **验证**：`npm run lint` ✅、`npm run build` ✅（CSS 产物 67.58 KB）。
- 2026-03-24 已完成 Web Console runtime/product IA 重构（Phase 0 → Phase 4 一次推进中的首轮落地）：
  - **Phase 0 — IA / 导航 / 全局入口**：`AppLayout.tsx` 去掉重复 Theme/I18n provider 包裹；侧边栏重组为 `运行主链 / AI 与触达 / 平台构件 / 治理与信号 / 身份与组织`；新增 `navigation.tsx` 统一导航源；Header 增加 `运行 / 提问` Action Hub 入口；`GlobalSearch.tsx` 升级为真正的 command hub（动作项 + 导航 + 文档搜索）；`Breadcrumbs.tsx` 与新 IA 对齐；新增独立 `/inbox` 和 `/chat` 路由，右下角 FAB 不再弹占位浮层而是直达 `/chat`。
  - **Phase 1 — Runtime chain 重做**：`DashboardView.tsx` 从 control-plane health 改为 incident/approval/execution 导向的 command center；`SessionList.tsx` 改为诊断工作队列；`SessionDetail.tsx` 改为 golden summary / evidence / conclusion / next action 为主的 diagnosis workbench；`ExecutionList.tsx` 改为 approvals & runs 队列；`ExecutionDetail.tsx` 改为 execution workbench；`SetupSmokeView.tsx` 重命名并弱化为 first-run + runtime checks，默认 first-run channel 改为 `in_app_inbox` 心智。
  - **Phase 2 — 统一交互模型**：新增 `web/src/components/operator/OperatorActionBar.tsx`、`web/src/components/operator/ConfirmActionDialog.tsx`、`web/src/components/operator/ActionResultNotice.tsx`，统一确认框、结果反馈、动作栏模式，为危险操作、启停切换、批量动作提供一致基座。
  - **Phase 3 — Providers / Channels / Templates / Ops / Identity / Org 重组**：调整 `ProvidersPage.tsx`、`ChannelsPage.tsx`、`MsgTemplatesPage.tsx`、`IdentityOverview.tsx`、`OpsActionView.tsx` 等页文案和产品定位；Channels 引入 `in_app_inbox` / `web_chat` 类型；新增 `/triggers` 页面 `TriggersPage.tsx`，把后端 trigger 控制面接回 UI，形成 Governance & Signals 子域。
  - **Phase 4 — Inbox / Web Chat / Action Hub 产品化**：新增 `InboxPage.tsx` 作为完整第一方通知工作台（列表、已读、全读、跳转 Session/Execution）；新增 `ChatPage.tsx` 作为第一方 Web Chat 产品壳，明确后续与 backend web chat runtime 的接线位；Header / Cmd+K / FAB 三个入口统一收束到 Action Hub 与 `/chat` 心智。
  - **文档同步**：本条 tracker 已记录；`40-web-console.md` 与 `tars_frontend_tasks.md` 同步补充新的 IA / runtime/product 化说明。
- 2026-03-25 已完成 Agent Role 平台化全量实现（Phase 1 规范 + Phase 2 工程落地）：
  - **Phase 1 — 规范与契约定义**：
    - 明确 Agent Role 作为一等平台对象的数据模型、生命周期与 API 契约。
    - Agent Role 定义运行时角色绑定，可影响诊断策略、审批路由、执行边界等行为。
  - **Phase 2 — 后端模块**：
    - `internal/modules/agentrole/`：完整 manager 实现，包含 registry、lifecycle（enable/disable）、DTO 与持久化。
  - **Phase 2 — API 路由**：
    - `/api/v1/agent-roles` 全套 CRUD：`GET`（list）、`POST`（create）、`GET /{id}`（detail）、`PUT /{id}`（update）、`POST /{id}/enable`、`POST /{id}/disable`、`DELETE /{id}`。
    - 统一鉴权与审计已接入。
  - **Phase 2 — 前端控制面**：
    - Web `/agent-roles` 列表页与 `/agent-roles/:id` 详情页已落地。
    - 支持列表检索、详情查看、创建、编辑、启停操作。
  - **Phase 2 — OpenAPI 契约同步**：
    - `api/openapi/tars-mvp.yaml` 已补齐 Agent Role 相关 path 与 schema 定义。
    - `validate_openapi.rb` 校验通过。
  - **Phase 2 — Struct Hooks 与平台集成**：
    - Agent Role struct hooks 已接入 workflow/dispatcher/session 上下文。
    - 运行时可根据 Agent Role 绑定影响诊断策略、审批路由与执行边界。
  - **Phase 2 — 运行时注入与执行层集成**（2026-03-25 续）：
    - SystemPrompt 注入：Dispatcher 在 `dispatchDiagnosis()` 中从 `agentrole.Manager.ResolveForSession()` 获取角色，将 `agent_role_system_prompt` 注入上下文；Reasoning 的 `buildFromModel()` 自动将其前置到 LLM 系统提示词。
    - Auto-assign：工作流 session 创建时（in-memory `service.go` 和 Postgres `workflow.go` 两条路径）默认赋值 `AgentRoleID: "diagnosis"`。
    - Authorization 叠加：`agentrole.EnforcePolicy()` 与 `EnforceCapabilityBinding()` 在 `resolveAuthorizationDecision()` 之上叠加 AgentRole 的 PolicyBinding 约束，取最严格结果（`max_action` / `max_risk_level` / `hard_deny` / `require_approval_for`）。
    - Automation UI：`AutomationsPage.tsx` Advanced 区新增 Agent Role 下拉选择器，从 `/api/v1/agent-roles` 拉取可用角色列表，支持为自动化任务指定运行角色。
    - Postgres 持久化：`agent_roles` 表 DDL（UUID PK、tenant_id、JSONB 列）、`alert_sessions`/`execution_requests` 新增 `agent_role_id` 列，SELECT/INSERT 同步更新。
    - Skills 类型补齐：`skills.Metadata` 新增 `Version`、`skills.Spec` 新增 `Type`/`Triggers`/`Planner` 字段，`Expand()` 重实现读取 planner steps，`matchesManifest()` 增强支持 `Spec.Triggers.Alerts`，DTO 与 handler 映射同步，`skillManifestFromDTO` 补齐 `Version` 映射修复 rollback 测试。
  - **验证**：`check_mvp.sh` 全部 5 步通过（go test ✅、go build ✅、openapi ✅、web lint ✅、web build ✅）。
- 2026-03-28 已关闭一条过期的 Agent Role model-binding review finding，并补齐证据链：
  - `diagnosis` 路径中，`dispatcher` 已将角色模型绑定传入 `reasoning` 的 planner / finalizer，`reasoning` 运行时会优先按角色绑定选择 provider / model。
  - 新增/补强测试覆盖 `BuildDiagnosis / PlanDiagnosis / FinalizeDiagnosis` 三条路径，以及 dispatcher 对 planner / finalizer 的绑定透传，避免后续再次把该问题误报为“runtime 不生效”。
  - 后续仍需跟进的不是 `diagnosis`，而是 `automation`：当前 Automation 已继承 Agent Role 的 policy / capability / audit 语义，但由于尚无独立的 LLM / model invocation path，不应宣称已经完整消费 `model_binding`。
- 2026-03-29 已完成 Agent Role / Provider / Automation 这一轮剩余收口：
  - **Automation 真正消费 Agent Role.model_binding**：`internal/modules/automations/manager.go` 为 `skill` automation 接入 role-bound `reasoning.PlanDiagnosis` + `FinalizeDiagnosis`，planner 输出会先经过 skill manifest 工具集过滤再执行；回归测试覆盖 planner/finalizer 均收到 `RoleModelBinding`，并验证实际执行参数来自 planner 输出。
  - **Provider 边界收口**：`web/src/pages/providers/ProvidersPage.tsx` 移除 Provider 页上的 legacy primary/assist 编辑控件，仅保留平台默认绑定只读展示并引导到 `Agent Roles` / Advanced Provider Ops；`internal/api/http/access_handler.go` 也不再在 Provider Registry DTO 中回填 `primary_model` / `assist_model`。
  - **Agent Role 治理 UI 补齐**：`web/src/pages/identity/AgentRolesPage.tsx` 新增 `allowed_connector_capabilities`、`denied_connector_capabilities`、`allowed_skill_tags`、`require_approval_for`，并将模型编辑器切到结构化 `model_binding.primary / fallback / inherit_platform_default`。
  - **Automation UI 收口**：`web/src/pages/automations/AutomationsPage.tsx` 新增 `governance_policy` 控件与列表展示，并把默认 Agent Role 文案切到 `automation_operator`。
  - **验证**：`go test ./internal/modules/automations ./internal/modules/reasoning ./internal/api/http ./internal/events` ✅；`npm run test:unit -- api-compatibility.test.ts ui-governance.test.ts locale-cleanup.test.ts channels-page-react.test.tsx` ✅；`npm run build` ✅。
- 2026-03-29 继续完成 `provider_preference -> model_binding` 最终收敛：
  - **输入校验收紧**：`internal/api/http/agent_role_handler.go` 与 `internal/modules/agentrole/manager.go` 现在会拒绝 partial target、fallback-only-without-primary/inherit 等半坏 `model_binding`。
  - **Postgres live migration 验证**：新增 `internal/repo/postgres/schema_integration_test.go`，在真实 Postgres 上验证旧 `provider_preference` 行会迁移为结构化 `model_binding`，并验证 role-only legacy binding 无法解析时会 fail fast。
  - **前端交互回归**：新增 `web/tests/agent-roles-page-react.test.tsx`，覆盖 AgentRolesPage 对 structured primary/fallback/inherit 的编辑与详情展示。
- 2026-03-24 已完成 Web Console Phase 5 — Web Chat 后端接入 + 弹窗 CRUD + Runtime Checks 风格统一：
  - **后端 Web Chat API**：
    - `internal/api/dto/types.go`：新增 `ChatMessageRequest / ChatMessageResponse / ChatSessionSummary / ChatSessionsResponse` 四个 DTO 类型。
    - `internal/api/http/chat_handler.go`：已实现 `POST /api/v1/chat/messages`（接收自然语言请求 → 构造 AlertEvent → 触发 workflow → 返回 ack + session_id）和 `GET /api/v1/chat/sessions`（列出 tars_generated=web_chat 的历史 sessions）。
    - `internal/api/http/routes.go`：在 `RegisterRoutes / RegisterPublicRoutes / RegisterOpsRoutes` 三个入口中均注册 `registerChatRoutes`。
  - **前端 Web Chat 工作台**：
    - 新增 `web/src/lib/api/chat.ts`：提供 `sendChatMessage()` 和 `listChatSessions()` 两个 API 函数及对应 TypeScript 类型。
    - 升级 `web/src/pages/chat/ChatPage.tsx` 为真实对话工作台：左侧消息面板（可选 host/service、消息列表、ack 气泡 + session 跳转链接）+ 右侧 Web Chat Sessions 历史列表；⌘+Enter 发送；加载态、错误提示、重复请求提示全部覆盖。
  - **弹窗 CRUD 基础组件**：
    - 新增 `web/src/components/operator/GuidedFormDialog.tsx`：基于 Radix Dialog，统一 modal 引导式编辑体验，支持 `wide` 模式和 `loading` 状态。
  - **ChannelsPage 改造**：
    - 重写 `web/src/pages/channels/ChannelsPage.tsx`：list 页改为 grid 卡片（带 hover 操作按钮），create/edit 改为 `GuidedFormDialog` 弹窗；页头升级为 `OperatorHero + OperatorStats`。
  - **SetupSmokeView 风格统一**：
    - 重写 `web/src/pages/setup/SetupSmokeView.tsx` 的全部子组件（`WizardMode / RuntimeMode / RuntimePathCard / ComponentStatusCard / LatestSmokeCard`），以 `OperatorHero + Card + Badge` 替换旧式 `PanelCard / GlassPanel`；错误提示改为语义化 inline banner；所有逻辑保持不变。
  - **验证**：`npm run build` ✅（0 TypeScript 错误，65.22 KB CSS）。
- 2026-03-26 已完成 P1/P2 风险纠偏（健康审计后续行动）：
  - **P1 — Workflow 双模式对齐（postgres.Store 补齐 AgentRoleManager）**：
    - `internal/repo/postgres/workflow.go`：`Options` 新增 `AgentRoleManager *agentrole.Manager` 字段；`resolveAuthorizationDecision()` 增加与 in-memory `service.go` 对应的 AgentRole 策略叠加逻辑（`agentrole.EnforcePolicy` 取最严格结果）。
    - `internal/app/bootstrap.go`：`postgresrepo.NewStore()` 调用中补入 `AgentRoleManager: agentRoleManager`，消除 Postgres 路径上 AgentRole 约束缺失的静默降级。
    - 验证：`go build ./...` ✅，`go test ./...` ✅。
  - **P2 — 补充 inbox 模块单元测试**：
    - 新增 `internal/modules/inbox/manager_test.go`（18 个测试），覆盖：`Create`（ID 生成、默认值、时钟注入、Action 序列化）、`Get`（命中/未命中）、`List`（全量、按 TenantID 过滤、UnreadOnly、分页）、`MarkRead`（IsRead 更新、ReadAt 时钟、未命中错误）、`MarkAllRead`（批量标读、跨租户隔离）、`CountUnread`（计数正确性、空租户）。
    - 从 0 → 18 tests，`tars/internal/modules/inbox` 从 `[no test files]` → `ok`。
  - **P2 — 补充 channel 模块单元测试**：
    - 新增 `internal/modules/channel/service_test.go`（11 个测试），通过 `stubInbox` 替身覆盖：路由到 inbox、Subject 推断（多行 Body 取首行）、显式 Subject 保留、Source 默认值/保留、Actions 透传、nil inbox 返回错误、inbox Create 失败透传错误；以及 `firstLine` 截断和 `fallback` 兜底逻辑。
    - 从 0 → 11 tests，`tars/internal/modules/channel` 从 `[no test files]` → `ok`。
  - **整体验证**：
    - `bash scripts/check_mvp.sh` ✅ ALL MVP checks passed (Strict Mode)
    - `go test ./...` 全绿：automations ✅（23）、trigger ✅（22）、inbox ✅（18）、channel ✅（11）及全部已有测试。
- 2026-03-26 已完成 健康审计 Action 2/3（补齐 Domain 层测试 + Golden Scenario 可移植性 + Workflow 存储警告）：
  - **Action 3 — Domain 层单元测试（Finding 4）**：
    - 新增 `internal/domain/alert/types_test.go`（4 个测试）：Event 字段构造、零值断言、Severity 枚举轮转、Fingerprint 唯一性。
    - 新增 `internal/domain/execution/types_test.go`（6 个测试）：全部 7 个 Status 常量值、唯一性、Request 字段构造与零值、终态/活跃状态分类、相等性。
    - 新增 `internal/domain/knowledge/types_test.go`（6 个测试）：Document/Record 字段构造与零值、SourceType 枚举轮转、Document → Record 关联完整性。
    - 新增 `internal/domain/session/types_test.go`（5 个测试）：全部 7 个 Status 常量值、唯一性、Aggregate 字段构造与零值、6 步生命周期顺序无重复。
    - 新增 `internal/domain/ticket/types_test.go`（5 个测试）：全部 5 个 Status 常量值、唯一性、Aggregate 字段构造与零值、5 步生命周期顺序无重复。
    - 所有 5 个 domain 包从 `[no test files]` → `ok`，`go test ./internal/domain/...` 全绿（26 个测试，0 失败）。
  - **Action 2 — Golden Scenario 可移植性（Finding 1）**：
    - `scripts/run_golden_scenario_1.sh`：移除硬编码 fallback `192.168.3.9` 及 `TARS_REMOTE_HOST` 引用；`TARS_CHAT_HOST` 改为必填（缺失则提示并 exit 1）；补充 Quick Example 示例行；增强两条必填变量的错误提示文案。
    - `scripts/run_golden_scenario_2.sh`：无硬编码 IP（原本已无）；补充 `TARS_GOLDEN_AUTOMATION_JOB` 文档注释；增强 `TARS_OPS_API_TOKEN` 错误提示；补充 Quick Example 与自定义 job 示例行。
    - 两脚本现均可在任意环境通过环境变量完全配置，无需修改脚本本身。
  - **Action 1 — Workflow 持久化警告（Finding 3）**：
    - `internal/app/bootstrap.go`：当 `cfg.Postgres.DSN == ""` 时，在 in-memory workflow 服务初始化后立即输出 `slog.Warn`，明确提示数据不持久化且建议生产环境配置 `TARS_POSTGRES_DSN`。
  - **验证**：`go build ./...` ✅，`go test ./internal/domain/...` ✅ 全绿（26 tests）。
- 2026-03-27 已完成 安全与权限回归护栏（固定回归子集落地）：
  - **目标**：建立安全与权限的固定回归护栏，防止越权访问、审批绕过、脱敏退化、高风险能力误暴露、API 权限表现不一致因代码变更静默退化。
  - **新建 `internal/api/http/security_regression_test.go`**（15 个测试，全部通过）：
    - `TestSecurityUnauthorizedAccessMatrix`：17 个受保护端点无 token 均返回 401。
    - `TestSecurityUnauthorizedWriteAccessMatrix`：7 个写端点无 token 均返回 401。
    - `TestSecurityViewerCannotWriteConfigs`：viewer 角色 PUT 配置返回 403。
    - `TestSecurityViewerCannotRunAutomations`：viewer 触发 automation 返回 403。
    - `TestSecurityViewerCanReadButNotWriteConnectors`：viewer 读 connectors 返回 200、写配置返回 403。
    - `TestSecurityDisabledUserCannotAuthenticate`：账号禁用后现有 session token 返回 401。
    - `TestSecurityOpsTokenRejectedWhenOpsAPIDisabled`：OpsAPI 禁用时 ops-token 被拒（404）。
    - `TestSecurityOpsTokenGrantsFullAccess`：ops-token 正常时有完整权限（200）。
    - `TestSecurityConfigAPIDoesNotExposeSecrets`：配置 API 响应不含 5 种明文 secret 前缀。
    - `TestSecurityApprovalEndpointRequiresAuth`：approve 端点无 token 返回 401。
    - `TestSecurityViewerCannotApproveExecution`：viewer 无法 approve（403/404）。
    - `TestSecurityAutomationRunRequiresAuth`：automation run 端点无 token 返回 401。
    - `TestSecurityWebhookRequiresValidSecretWhenConfigured`：配置 secret 后无效签名返回 401。
    - `TestSecurityInvalidTokenFormatsAreRejected`：5 种畸形 token（空、Bearer-only、短值、空格、中文）均返回 401。
    - `TestSecurityPublicEndpointsWhitelist`：5 个公开端点（health/ready/connectors list/version/login）无 token 可访问（200/201/404）。
  - **新建 `scripts/ci/security-regression.sh`**：安全回归专属 CI 脚本，执行 `go test ./internal/api/http/... -run TestSecurity`。
  - **更新 `Makefile`**：新增 `make security-regression` target（L2 专项，约 3s），更新 `.PHONY` 列表和 help 文本。
  - **同步更新 5 个 spec 文档**：`30-strategy-automated-testing.md`（Priority C 已落地标记 + 安全回归子集详表）、`20-component-identity-access.md`（新增第 12 节）、`30-strategy-authorization-granularity.md`（新增第 11 节）、`30-strategy-command-authorization.md`（新增第 15 节）、`30-strategy-desensitization.md`（新增安全回归覆盖节）。
  - **验证**：`bash scripts/check_mvp.sh` ✅ ALL MVP checks passed (Strict Mode)、`go test ./...` 全绿（含 15 个新安全回归测试）。
- 2026-04-02 新增四条待办补记（仅记录 backlog，暂不拆分具体工作项）：
  - **SSH 凭据托管能力**：系统后续需要支持用户输入并托管 SSH 密码 / 私钥，不再只依赖 `env` 或本地文件路径注入；相关设计需要同时覆盖安全存储、最小暴露面、审计留痕、权限边界和运行时使用方式。
  - **迁移到 GitHub 的前置调研**：近期开发计划迁移到 GitHub，需先统一调研迁移前准备项、仓库与协作流调整、CI/CD 与 secrets 管理、测试环境搭建方案，以及免费优先的测试基础设施选择（优先考虑 Cloudflare / Vercel / Supabase 等）是否满足当前安全要求；观测类试验环境可一并评估复用 VictoriaMetrics 官方 play 环境：`https://play.victoriametrics.com/`、`https://play-vmlogs.victoriametrics.com/`、`https://play-vtraces.victoriametrics.com/`。当前已提升为整体路线的第三优先级，但暂不在这里拆成详细任务。
  - **设计审计与设计语言收紧**：后续需要安排一次正式的 Web 设计审计，统一处理当前产品在品牌辨识度、字体方案、glass 效果使用密度、导航信息架构、核心与高级页面分层，以及整体视觉语言"更像工具而不是产品"的问题；这一项先作为整体待办保留，暂不拆成具体视觉或前端实现任务。
  - **Connector 优先级重置**：后续 connector 路线按 `SSH / VictoriaMetrics / VictoriaLogs` 三类一等对象收敛，优先保证它们获得独立模板、验证、控制面入口和运行时打磨；`Prometheus / JumpServer / MCP` 等其它对象先保留兼容与设计空间，但默认优先级下调，不再抢近程产品和交付资源。
- 2026-04-08 完成 SSH 凭据托管设计（Phase 0~4 完整设计文档）：
  - **设计文档落地**：`docs/superpowers/specs/2026-04-08-ssh-credential-custody-design.md` 已写出，覆盖：
    - 双层模型（PG 元数据 + encrypted secret backend），SSH 密码/私钥不进入 runtime_config_documents
    - 数据模型（`ssh_credentials` 元数据表 + secret material 分离）
    - API 边界（create/update/delete/list/use，secret value 永不回显）
    - 权限与审计（credential scope、执行时短暂解析、审计日志）
    - Fail-closed 默认（GitHub demo 和未配置环境默认不启用真实 SSH 执行）
    - 凭据轮换、禁用、过期提示的完整生命周期
  - **状态**：设计完成，前端 UI 和后端 API 实现尚未展开
- 2026-04-08 完成 GitHub push 前凭据轮换审计（`records/credential_rotation_execution_tracker_2026-04-08.md`）：
  - 扫描范围：`docs/operations`, `deploy/team-shared`, `scripts`, `deploy/docker`, `web/src`（排除历史报告）
  - 全部 13 类凭据（Ops API token、Telegram bot token、Telegram webhook secret、VMAlert webhook secret、Gemini API key、DashScope API key、Dex/OIDC client secret、Dex/local password hash、local token auth secret、TOTP seed、JumpServer access/secret key、SSH key、GitHub/deploy tokens）均已确认为 `invalid/non-live`（非生产值）
  - 结论：GitHub push 无外部凭据阻塞项
- 2026-04-11 完成全量 spec focus review（`docs/operations/spec_focus_review_2026-04-11.md`）：
  - 完整盘点约 87 个文档，分类为 keep-now / merge / park / archive
  - 确认当前 product spine（5 条主线 + 前端 Tier 1~4 欠账清单）
  - 入口文档最小同步更新（tars_prd.md 头部适用范围说明、Phase 2b/3/4 标注、本 tracker 补记、WBS 降级说明、frontend_tasks 降级说明）
