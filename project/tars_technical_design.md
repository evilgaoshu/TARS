# TARS — 技术方案设计文档 (TSD) v1.8

> **版本**: v1.8  
> **日期**: 2026-03-23  
> **对应 PRD**: [tars_prd.md](tars_prd.md) v2.6  
> **目标范围**: Phase 1 MVP 为主，兼容 Phase 2a / 2b 扩展

---

## 1. 设计目标

### 1.1 设计目标

- 支持 MVP 最短闭环：`VMAlert -> AlertSession -> AI 诊断 -> 审批 -> SSH 执行 -> 结果回传`
- 在单体部署下保持模块边界清楚，后续可平滑拆分服务
- 所有状态变更集中在 `Workflow Core`
- 所有外部动作统一经 `Action Gateway`
- 关键数据可审计、可追踪、可回放
- Phase 1 优先保证实现速度和可靠性，Phase 2 再补产品化与企业化能力

### 1.2 非目标

- Phase 1 不实现完整 Ticket 系统
- Phase 1 不实现多租户、RBAC、OIDC
- Phase 1 不实现外置插件进程隔离
- Phase 1 不实现内网模型与公网模型双路径并行适配，但模型协议适配层支持 `openai_compatible / anthropic / ollama / lmstudio`

### 1.3 总体实现策略

采用 **模块化单体 (Modular Monolith)**：

- 单个 Go 服务进程承载核心业务模块
- PostgreSQL 存储业务状态、审计和元数据
- SQLite-vec 存储向量索引
- 通过内部接口和事件 outbox 保持模块解耦
- Phase 3 之后再考虑将插件执行、知识处理等模块拆成独立进程

### 1.4 Setup Wizard 与运行时配置第一阶段（2026-03-23）

- 新增 `runtime_config_documents` 与 `setup_state` 两类 Postgres 持久化对象，用于承载第一阶段 DB-backed runtime config 与初始化状态。
- 当前阶段不重写全部 file-backed manager，而是通过 `RuntimeConfigStore` 在 bootstrap 时优先回灌 DB 中的 `access/providers` 配置，并在 setup wizard / providers config 写入时同步持久化到 DB。
- `/api/v1/setup/wizard*` 提供 first-run 向导接口；新增 `GET /api/v1/bootstrap/status` 作为匿名轻量探测接口；`GET /api/v1/setup/status` 保留给首次安装详情与初始化后的运行体检数据读取。
- 未初始化时允许匿名访问 `/setup` wizard；初始化完成后，`/setup` 只作为分流路由，已登录用户进入 `/runtime-checks`，未登录用户进入 `/login`。
- `secrets.Store` 仍保持 file-backed，providers 的敏感字段只保存 `secret_ref`，不直接写入普通 runtime config 表。

### 1.5 Setup Wizard 与运行时配置第二阶段（2026-03-23）

- 第二阶段延续“运行时 manager 不重写、先补 DB-backed 主路径”的演进策略：通过给 manager 增加 persistence hook，把 `providers / connectors / auth providers / channels / setup state` 的主读写路径切到 Postgres。
- 当前新增的 persistence hook 包括：
  - `access.Manager.SetPersistence(func(Config) error)`
  - `reasoning.ProviderManager.SetPersistence(func(ProvidersConfig) error)`
  - `connectors.Manager.SetPersistence(func(Config, map[string]LifecycleState) error)`
- bootstrap 启动时会先从 `runtime_config_documents` 加载 `access / providers / connectors`，再回灌到内存 manager；回灌过程中会临时关闭 persistence hook，避免把同一份配置再次写回 DB。
- `connectors` 新增独立文档键与 lifecycle 持久化能力；在 `path` 为空时仍可执行 `SaveConfig / Save / SetEnabled / Upgrade / Rollback / RecordHealth / SetAvailableVersion`。
- 第二阶段仍不引入 YAML 导入桥，也不要求一次把所有平台配置改成结构化专用表；`iam_auth_providers` 等结构化表保留为下一层演进空间。
- `/setup` 的职责收窄为“首次安装”；`/runtime-checks` 承接初始化后的运行体检；`/ops` 保留控制面与高级运维入口，但不再承担首次安装主入口职责。

### 1.5.1 Runtime Config 收敛第三阶段（2026-04-08）

- 第三阶段继续沿用 `runtime_config_documents` 作为低风险收敛层，新增 `authorization / approval_routing / org / reasoning_prompts / desensitization / agent_roles` 文档键。
- bootstrap 会给这些 manager 绑定 persistence hook，并在启动时从 Postgres 回灌 runtime state；`TARS_RUNTIME_CONFIG_REQUIRE_POSTGRES=true` 可用于试点/生产环境，防止缺少 `TARS_POSTGRES_DSN` 时静默降级到内存。
- `secrets.Store` 仍不进入普通 runtime config 文档。模型 key、SSH 密码、SSH 私钥、JumpServer key 等继续通过 secret ref 与私有 secret backend/file 注入；后续如需 PG 托管，必须先设计 envelope encryption / KMS / audit 边界。
- `automations / skills / extensions` 暂不纳入本轮迁移，因为它们同时携带 lifecycle / marketplace / state 文件语义；后续应按使用证据决定 JSON document、专用表或保持 optional 文件态。

### 1.6 Setup Wizard 第二阶段校验与登录引导

- provider step 不再只保存静态配置，而必须先完成：
  - `base_url` URL 格式校验
  - `api_key_ref` 必须为 `secret://...` 格式
  - `secret ref` 在当前 secret store 中存在且非空
  - 真实 provider connectivity / availability check
- provider connectivity check 结果会写入 `setup_state`，包括：
  - `provider_checked`
  - `provider_check_ok`
  - `provider_check_note`
- channel step 当前补齐 Telegram target 的最小格式校验，接受 `@channel`、`-100...` 与纯数字 chat id。
- complete step 只有在 provider check 成功后才允许完成初始化，并会把 `login_hint` 写入 `setup_state`，用于前端自动登录或跳转登录页预填。
- 若首装选择 `local_password` 且前端仍持有刚录入的管理员密码，完成后优先自动调用登录接口；若失败，则至少跳转到带 `provider_id / username` 查询参数的 `/login`。

---

## 2. 总体架构

### 2.1 运行时架构

```mermaid
graph TB
    subgraph 外部输入
        VMA[VMAlert Webhook]
        TG[Telegram Webhook]
        DOC[本地文档目录]
    end

    subgraph TARS 单体进程
        HTTP[HTTP Server]
        AI[Alert Intake]
        CA[Channel Adapter]
        WF[Workflow Core]
        RS[Reasoning Service]
        AG[Action Gateway]
        KS[Knowledge Service]
        FD[Foundation]
        WK[Background Workers]
    end

    subgraph 存储
        PG[(PostgreSQL)]
        VEC[(SQLite-vec)]
    end

    subgraph 外部依赖
        MODEL["Model Gateway (OpenAI / Anthropic / Ollama / LM Studio)"]
        VM[VictoriaMetrics]
        SSH[SSH Target Hosts]
    end

    VMA --> HTTP --> AI --> WF
    TG --> HTTP --> CA --> WF
    WF --> CA
    WF --> RS --> KS
    WF --> AG
    AG --> VM
    AG --> SSH
    RS --> MODEL
    KS --> VEC
    AI --> PG
    WF --> PG
    AG --> PG
    KS --> PG
    FD --> PG
    WK --> PG
    WK --> CA
    WK --> KS
```

### 2.2 模块职责

| 模块 | 责任 | 输入 | 输出 |
|------|------|------|------|
| Alert Intake | 接收告警并标准化 | VMAlert Webhook | `AlertEvent` |
| Channel Adapter | 渠道协议适配、收发消息 | Telegram Webhook / Workflow 通知 | 用户交互事件 / 渠道消息 |
| Workflow Core | 唯一状态机 owner，推进闭环 | `AlertEvent`、交互事件、执行/查询结果 | 诊断请求、查询请求、执行请求、通知、异步事件 |
| Reasoning Service | 诊断建议和执行建议生成 | `AlertSession`、查询结果、知识结果 | 诊断建议、`ExecutionRequestDraft` |
| Action Gateway | 查询插件和执行通道统一出口 | 查询请求、已审批执行请求 | 查询结果、执行结果 |
| Knowledge Service | 文档检索、闭环沉淀、Skill 草稿 | 文档、查询关键词、`session_closed` 事件 | 检索结果、`KnowledgeRecord`、`SkillDraft` |
| Foundation | 配置、审计、观测、认证基础能力 | 全模块 | 横切能力 |

### 2.3 关键架构决策

| 决策 | 方案 | 原因 |
|------|------|------|
| 服务形态 | 模块化单体 | MVP 实现快、部署简单、跨模块调试成本低 |
| 异步机制 | PostgreSQL outbox + worker | 保证 `session_closed`、消息重试、审批超时等任务可靠 |
| 异步演进 | 先补 Event Bus 抽象，后续按需接正式 MQ / Bus | 避免业务代码直接绑死 Postgres outbox，实现平滑演进 |
| 幂等机制 | PostgreSQL `idempotency_keys` + 业务唯一约束 | webhook / callback 可重放，必须可判重且可追踪 |
| 并发控制 | 乐观锁 (Optimistic Locking) | 针对核心状态表引入 `version` 字段，防并发冲刷 |
| 关系存储 | PostgreSQL | 状态事务、审计查询、后续多租户都依赖关系数据库 |
| 向量存储 | SQLite-vec | MVP 成本低、无需引入额外服务 |
| 执行通道 | Connector-first + SSH fallback | `jumpserver_api` 已作为 execution 主路径，workflow / Action 会优先自动选择兼容 connector；仅在未注册可用 connector 时回退到 SSH |
| 渠道 | Telegram | 单渠道先验证闭环 |
| 模型接入 | 统一 Provider Registry + `primary/assist` 绑定 | 支持 OpenAI / Claude / Gemini / OpenRouter / Ollama / LM Studio 等多协议供应商，同时区分主分析模型和辅助安全模型 |

### 2.4 模块交互契约

为避免模块边界在实现阶段失守，所有核心模块只通过以下 3 类契约交互：

| 契约类型 | 发起方 | 接收方 | 同步/异步 | 说明 |
|----------|--------|--------|-----------|------|
| Command | API / Worker / Workflow | Workflow / Knowledge / Channel | 同步 | 会改变当前模块拥有的状态 |
| Query | Workflow / Reasoning | Action / Knowledge | 同步 | 只读调用，不允许写状态 |
| Domain Event | Workflow | Worker / Knowledge / Channel | 异步 | 通过 outbox 可靠分发 |

补充（2026-03-23 当前实现）：

- `Domain Event` 运行时已新增统一 Event Bus 抽象层，核心契约包括：
  - `EventPublishRequest`
  - `EventEnvelope`
  - `DeliveryPolicy`
  - `DeliveryDecision = ack / retry / dead_letter`
- 当前 `Workflow Core` 与 Postgres store 均已实现：
  - `PublishEvent`
  - `ClaimEvents`
  - `ResolveEvent`
  - `RecoverPendingEvents`
- 当前 dispatcher 已改为消费 `EventEnvelope`，而不是直接 claim 后手工 `CompleteOutbox / MarkOutboxFailed`
- 已切到新抽象的真实 topic：
  - `session.analyze_requested`
  - `session.closed`
  - `telegram.send`
- 当前默认投递策略：
  - `session.analyze_requested`: 单次投递，失败即 dead-letter
  - `session.closed`: 单次投递，失败即 dead-letter
  - `telegram.send`: 最多 3 次，backoff 为 `1s -> 5s`
- 当前底层仍保持 `outbox_events` 表与 polling worker，不改变现有单体部署模型

MVP 必须实现的 application service 接口：

| 模块 | 接口 | 输入 | 输出 |
|------|------|------|------|
| Alert Intake | `IngestVMAlert(ctx, rawPayload)` | 原始 webhook body | `[]AlertEvent` |
| Workflow Core | `HandleAlertEvent(ctx, event)` | `AlertEvent` | `SessionMutationResult` |
| Workflow Core | `HandleChannelEvent(ctx, event)` | `ChannelEvent` | `WorkflowDispatchResult` |
| Workflow Core | `HandleExecutionResult(ctx, result)` | `ExecutionResult` | `SessionMutationResult` |
| Reasoning Service | `BuildDiagnosis(ctx, input)` | `DiagnosisInput` | `DiagnosisOutput` |
| Action Gateway | `QueryMetrics(ctx, query)` | `MetricsQuery` | `MetricsResult` |
| Action Gateway | `ExecuteApproved(ctx, req)` | `ApprovedExecutionRequest` | `ExecutionResult` |
| Action Gateway | `InvokeCapability(ctx, req)` | `CapabilityInvokeRequest` | `CapabilityInvokeResult` |
| Knowledge Service | `Search(ctx, query)` | `KnowledgeQuery` | `[]KnowledgeHit` |
| Knowledge Service | `IngestResolvedSession(ctx, event)` | `SessionClosedEvent` | `KnowledgeIngestResult` |
| Channel Adapter | `SendMessage(ctx, msg)` | `ChannelMessage` | `SendResult` |

约束：
- `Workflow Core` 之外的模块不得直接更新 `alert_sessions`、`execution_requests`、`tickets`
- `Action Gateway` 只能接收已审批的执行请求；风险分级和审批判断不得下沉到 SSH / JumpServer / 外部执行 runtime 内部
- `Knowledge Service` 只允许通过 `Search` 同步被调用；知识沉淀必须通过 `session_closed` 异步事件触发
- 当 feature flag 关闭某阶段能力时（如 `execution_enabled=false`），对应的 Command 和 Domain Event 必须被拦截或暂存，而不是报错

### 2.5 下一阶段诊断范式调整

当前实现仍以“固定 enrich + 一次推理 + 可选执行”为主，这适合 MVP，但不适合“过去一小时负载”“先看监控再决定是否上机”这类问题。

下一阶段目标范式应调整为：

- `LLM 先产出 tool plan`
- `Workflow / Action 按 plan 调用 connector runtime`
- `必要时才进入 execution`

换句话说，`Prometheus / VictoriaMetrics / JumpServer / APM / MCP` 等不应只作为固定 enrich 或控制面能力存在，而应成为 planner 可调用的统一工具集合。

详细设计见 [90-design-tool-plan-diagnosis.md](../specs/90-design-tool-plan-diagnosis.md)。

---

## 3. 关键流程设计

### 3.1 告警到诊断建议

```mermaid
sequenceDiagram
    participant VMAlert
    participant HTTP
    participant AlertIntake
    participant Workflow
    participant Action
    participant Reasoning
    participant Knowledge
    participant Channel

    VMAlert->>HTTP: POST /api/v1/webhooks/vmalert
    HTTP->>AlertIntake: parse + validate
    AlertIntake->>Workflow: AlertEvent
    Workflow->>Workflow: create AlertSession(open)
    Workflow->>Action: query metrics context
    Action->>Workflow: metrics result
    Workflow->>Reasoning: diagnose(session + context)
    Reasoning->>Knowledge: search(query)
    Knowledge->>Reasoning: citations
    Reasoning->>Workflow: diagnosis + execution draft
    Workflow->>Channel: send diagnosis message
```

### 3.2 审批到执行

```mermaid
sequenceDiagram
    participant User
    participant Telegram
    participant Channel
    participant Workflow
    participant Action
    participant Runtime

    User->>Telegram: 点击批准/修改命令
    Telegram->>Channel: webhook callback
    Channel->>Workflow: approval event
    Workflow->>Workflow: update ExecutionRequest(approved)
    Workflow->>Action: execute(approved request)
    Action->>Runtime: route by connector runtime (SSH / JumpServer)
    Runtime->>Action: stdout/stderr/exit code/output ref
    Action->>Workflow: execution result
    Workflow->>Channel: send execution result
    Workflow->>Workflow: move session to verifying/resolved/failed
```

### 3.3 闭环沉淀到知识

```mermaid
sequenceDiagram
    participant Workflow
    participant Outbox
    participant Worker
    participant Knowledge
    participant Postgres
    participant VecDB

    Workflow->>Postgres: mark session resolved
    Workflow->>Outbox: insert session_closed event
    Worker->>Outbox: poll event
    Worker->>Knowledge: consume session_closed
    Knowledge->>Postgres: upsert documents(source_type=session)
    Knowledge->>Postgres: write KnowledgeRecord(document_id)
    Knowledge->>Postgres: write document_chunks
    Knowledge->>VecDB: write embeddings
```

---

## 4. API 设计

### 4.1 API 约定

- 协议：HTTP/JSON
- 时间格式：RFC3339 UTC
- 请求 ID：支持 `X-Request-Id`
- 幂等策略：
  - 外部 webhook / callback 不依赖调用方自带幂等头，由平台内部生成或提取 `idempotency_key`
  - 所有幂等键先写 `idempotency_keys`，以 `(scope, idempotency_key)` 唯一约束拦截重复请求
  - 若同一 `(scope, idempotency_key)` 命中了不同 `request_hash`，视为异常重放，记录审计并返回 `409 Conflict`
  - 幂等记录至少保存 `request_hash`、`resource_type`、`resource_id`、`status`，用于排障和重放分析
- 响应错误格式统一为：

```json
{
  "error": {
    "code": "invalid_signature",
    "message": "signature verification failed",
    "details": {}
  }
}
```

- MVP 认证策略：
  - `VMAlert Webhook`：通过共享 secret 校验
  - `Telegram Webhook`：通过 webhook secret/header 校验
  - 内部运维 API：默认关闭，仅在 `ops_api.enabled=true` 时开启；必须绑定内网监听地址，并要求 `Bearer Token + 反向代理来源约束`

### 4.2 MVP 公共入口 API

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| `POST` | `/api/v1/webhooks/vmalert` | 接收 VMAlert 告警 | Webhook secret |
| `POST` | `/api/v1/channels/telegram/webhook` | 接收 Telegram 消息和回调 | Telegram secret |
| `GET` | `/healthz` | 存活检查 | 无 |
| `GET` | `/readyz` | 就绪检查 | 无 |
| `GET` | `/metrics` | Prometheus 指标 | 内网/采集端访问 |

### 4.3 内部运维 API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/sessions/{session_id}` | 查询单个会话详情 |
| `GET` | `/api/v1/sessions` | 按状态、时间、主机查询会话 |
| `GET` | `/api/v1/executions/{execution_id}` | 查询单个执行请求 |
| `GET` | `/api/v1/outbox` | 查询失败/阻塞的 outbox 事件 |
| `POST` | `/api/v1/outbox/{event_id}/replay` | 手动重放失败/阻塞事件 |
| `POST` | `/api/v1/reindex/documents` | 手动触发文档重建索引 |

约束：
- `ops_api.enabled=false` 时，这组接口不注册路由
- `ops_api.enabled=true` 时，仅监听独立内网地址，例如 `127.0.0.1:8081` 或内网 Service
- 不允许直接暴露在公网 Ingress；生产环境要求至少满足以下两项：
  - 经过 VPN / 内网 LB / 反向代理访问
  - `Authorization: Bearer <OPS_API_TOKEN>`
  - 反向代理注入可信身份头或 mTLS 终止
- 所有运维 API 访问必须写入 `audit_logs`
- `POST /api/v1/reindex/documents` 属于运维写接口，除鉴权外还必须记录完整后台审计；不要求用户在界面填写 `operator_reason`

> `GET /api/v1/sessions*` 在 MVP 阶段主要用于调试和试点支持，不代表完整产品 API 已开放；生产试点默认关闭，需按环境显式启用。

### 4.4 `POST /api/v1/webhooks/vmalert`

**请求头**

```http
X-Tars-Signature: sha256=...
Content-Type: application/json
```

**请求体示例**

```json
{
  "status": "firing",
  "alerts": [
    {
      "labels": {
        "alertname": "HighCPUUsage",
        "instance": "prod-web-01",
        "severity": "warning",
        "service": "web"
      },
      "annotations": {
        "summary": "CPU usage > 90%",
        "description": "instance prod-web-01 cpu is high"
      },
      "startsAt": "2026-03-11T07:12:00Z"
    }
  ]
}
```

**响应**

```json
{
  "accepted": true,
  "event_count": 1,
  "session_ids": ["ses_01HQ..."]
}
```

**服务端处理**
- 验证签名提取 Payload
- 根据 `fingerprint` + `startsAt`（或收到时间 Hash）计算唯一幂等键，scope 固定为 `vmalert_webhook`
- 在同一事务内先写 `idempotency_keys`；若命中唯一冲突，则直接返回上次处理结果或 `accepted=true, duplicated=true`
- 映射成内部 `AlertEvent`
- 创建或关联 `AlertSession`
- 返回 `session_id`

### 4.5 `POST /api/v1/channels/telegram/webhook`

**输入类型**
- 普通消息
- `/approve <execution_id>` 风格命令
- Inline keyboard callback

**统一输出事件**

```json
{
  "event_type": "approval",
  "channel": "telegram",
  "user_id": "tg:123456",
  "chat_id": "987654",
  "action": "approve",
  "execution_id": "exe_01HQ...",
  "message_id": "13579",
  "idempotency_key": "update_id_817234"
}
```
**幂等与并发控制**
- Telegram 会重发未响应的 webhook，平台使用 `update_id` 作为 `telegram_update` scope 下的幂等键。
- 幂等判定成功后才允许落 `execution_approvals`；重复 callback 直接返回 200，避免 Telegram 持续重试。
- Action 提交到 Workflow Core 时，必须配合乐观锁避免同一个 Execution 被并发状态推进。

### 4.6 `GET /api/v1/sessions/{session_id}`

**响应示例**

```json
{
  "session_id": "ses_01HQ...",
  "status": "pending_approval",
  "alert": {
    "source": "vmalert",
    "severity": "warning",
    "labels": {
      "instance": "prod-web-01",
      "service": "web"
    }
  },
  "diagnosis_summary": "CPU 异常，建议先检查 TOP 进程与最近 1h CPU 曲线",
  "golden_summary": {
    "headline": "HighCPU @ prod-web-01",
    "conclusion": "CPU 持续升高，当前更像是应用进程占满资源而不是宿主机抖动",
    "risk": "critical",
    "next_action": "等待审批，目标命令：top -bn1 | head -20",
    "notification_headline": "最近通知 请求人工审批，目标 owner-room"
  },
  "notifications": [
    {
      "stage": "diagnosis",
      "target": "owner-room",
      "reason": "发送诊断结论",
      "preview": "[TARS] 诊断 ..."
    },
    {
      "stage": "approval",
      "target": "owner-room",
      "reason": "请求人工审批",
      "preview": "[TARS] 待审批 ..."
    }
  ],
  "executions": [
    {
      "execution_id": "exe_01HQ...",
      "session_id": "ses_01HQ...",
      "status": "pending",
      "risk_level": "info",
      "command": "top -bn1 | head -20",
      "golden_summary": {
        "headline": "执行 top -bn1 | head -20 @ prod-web-01",
        "approval": "待审批，审批组 service_owner:web",
        "result": "尚未开始执行",
        "next_action": "在 Telegram / Inbox 中完成审批或拒绝"
      }
    }
  ],
  "timeline": [
    {
      "type": "session_created",
      "at": "2026-03-11T07:12:02Z"
    }
  ]
}
```

补充说明：

- `diagnosis_summary` 继续保留，作为完整 AI 输出与 Markdown 渲染来源。
- `golden_summary` 是专门面向值班 UX、脚本回放与验收的表达层，不替代原始诊断文本。
- `notifications` 来自 session timeline 的结构化提取，当前至少覆盖 `diagnosis_message_prepared` 与 `approval_message_prepared`。
- `ExecutionDetail.session_id` 必须返回，便于 `/executions` 和 `/executions/:id` 与 session 主链建立可跳转关系。

### 4.6A `GET /api/v1/executions/{execution_id}`

执行详情除原始 `command / output_ref / runtime / status` 外，还应返回：

- `session_id`：让执行记录能回跳到主会话
- `golden_summary.headline`：一句话说明本次动作是什么
- `golden_summary.approval`：当前审批结论或审批归属
- `golden_summary.result`：执行结果的简短结论
- `golden_summary.next_action`：下一步是等待审批、等待校验，还是已经闭环

这层表达仍是读模型派生，不改变原始执行记录结构，也不替代完整 output。

### 4.7 Phase 2 API 预留

| 方法 | 路径 | 阶段 | 说明 |
|------|------|------|------|
| `POST` | `/api/v1/alerts` | Phase 2a | 开放 REST 告警接入 |
| `GET` | `/api/v1/tickets/{ticket_id}` | Phase 2a | 查询工单 |
| `POST` | `/api/v1/skills` | Phase 2a | 手动创建或导入 Skill Registry 条目 |
| `POST` | `/api/v1/auth/login` | Phase 2b | OIDC / LDAP 登录 |

### 4.9 多记录列表统一查询框架（后续阶段）

详细规范见：[docs/40-ux-unified-list-bulk.md](../specs/40-ux-unified-list-bulk.md)

`sessions / executions / outbox / audit / trace / knowledge` 等多记录接口，后续需要收敛成统一查询契约，而不是每个接口单独发明参数。

建议统一约定：

| 参数 | 说明 |
|------|------|
| `page` | 页码，从 `1` 开始 |
| `limit` | 每页条数，默认 `20`，可选 `20 / 50 / 100` |
| `q` | 通用搜索关键字 |
| `sort_by` | 排序字段，例如 `created_at / updated_at / status` |
| `sort_order` | `asc / desc` |
| `filters.*` | 结构化过滤条件，例如 `status / host / service / aggregate_id` |

响应建议统一为：

```json
{
  "items": [],
  "page": 1,
  "limit": 20,
  "total": 135,
  "has_next": true
}
```

批量操作也应统一为集合动作模型，例如：

- `POST /api/v1/outbox/batch/replay`
- `POST /api/v1/outbox/batch/delete`
- `POST /api/v1/sessions/batch/archive`

目标不是先扩 API 数量，而是先统一分页、搜索、排序、批量动作的交互模式和返回结构。

### 4.10 Tool Plan 与媒体结果（已实现第一版）

为支持“先查监控，再决定是否执行”的交互模式，当前实现已落地第一版：

- `DiagnosisOutput.tool_plan`
- `metrics.query_range`
- 统一附件协议 `attachments[]`

最低要求：

| 能力 | 用途 |
|------|------|
| `tool_plan` | 让 LLM 指示平台优先调用哪些 connector |
| `metrics.query_range` | 支持过去一小时等时序查询 |
| `attachments.image/file` | 返回图表、报告、原始结果文件 |

约束：

- LLM 不能直接调用外部系统，只能返回 plan
- execution 类 plan 仍必须走授权和审批
- 统一通过 connector runtime 执行，不新增旁路调用

当前实现边界：

- diagnosis 主链路已改成 `planner -> execute tool steps -> final summarizer`
- 当前已支持 `metrics.query_range / metrics.query_instant / execution.run_command(plan only)` 三类工具
- planner 输入会注入平台能力目录：
  - `tool_capabilities`
  - `tool_capabilities_summary`
- discovery 已同步暴露 `tool_plan_capabilities`，供控制台和外部系统理解当前可规划的系统能力
- connector / MCP / skill source 都统一按“系统能力”进入 planner 视野
- 非标准能力当前统一归入 `connector.invoke_capability`，先完成目录发现，再逐步补 runtime
- planner 提示词与归一化层现已同时支持 `observability.query / delivery.query`
- `execution.run_command` 仍只作为计划步骤和后续 `execution_hint` 来源，不会在 planner 阶段直接执行

当前已实现的 tool plan 执行路径：

- `observability.query`：按 connector type 解析 observability connector → `InvokeCapability()` → 回填 `result.Context["observability_query_result"]`
- `delivery.query`：按 connector type 解析 delivery connector → `InvokeCapability()` → 回填 `result.Context["delivery_query_result"]`
- planner 归一化层会在模型未指定 `connector_id` 时，从 `tool_capabilities` 中为 `metrics.query_* / observability.query / delivery.query / connector.invoke_capability` 选择首个可调用 connector，避免 tool-plan 主路径静默落回 legacy/stub
- planner prompt 现已显式支持 `$steps.<step_id>.output...` / `$steps.<step_id>.input...`，用于多步 tool-plan 的 step 间数据传递
- dispatcher 会在 `delivery / observability / metrics / knowledge` 已提供足够证据时抑制不相干的 generic host execution hint，避免“先查系统再无意义上机”的回退
- `connector.invoke_capability`：从 step params 提取 `connector_id / capability_id` → `InvokeCapability()` → 回填 output / runtime / attachments / context → 发射审计事件
- 三者统一走 `action.CapabilityRuntime` 接口，runtime 按 connector type 注册（observability / delivery / mcp / skill）
- `mcp_tool / skill_source` 在运行时统一归一化到 `mcp / skill`，确保 capability catalog、runtime 选择和 `hard_deny.mcp_skill` 授权来源一致

其余已有实现：

- `tool_plan / attachments` 已落到 PostgreSQL `alert_sessions.tool_plan / attachments`
- `/api/v1/sessions/{id}` 与 Session Detail 已可查看 `tool_plan / attachments`
- `/api/v1/setup/status` 已改为 connector-first 语义，legacy fallback 仅保留兼容字段
- `metrics-range.png` 第一版已带图表标题、查询/窗口副标题、X/Y 轴标签和时间/数值刻度
- capability invoke 已采用统一状态语义：
  - `completed` -> HTTP `200`
  - `pending_approval` -> HTTP `202`
  - `denied` -> HTTP `403`

### 4.11 Skill Registry 与 Skill Runtime（已实现第一版）

Skill 已从“设计中 / package 附属物”推进为与 Connector Registry 同级的平台对象，当前实现边界如下：

- 控制面：
  - `GET /api/v1/skills`
  - `POST /api/v1/skills`
  - `GET /api/v1/skills/{id}`
  - `PUT /api/v1/skills/{id}`
  - `POST /api/v1/skills/{id}/enable`
  - `POST /api/v1/skills/{id}/disable`
  - `POST /api/v1/skills/{id}/promote`
  - `POST /api/v1/skills/{id}/rollback`
  - `GET /api/v1/skills/{id}/export`
  - `POST /api/v1/config/skills/import`
- 数据面：manifest、lifecycle、history、revisions、compatibility、marketplace load/import 已落地
- 运行时：dispatcher 现优先做 `skill match -> skill expanded to tool-plan -> execute tool plan`，只有未命中 skill 时才回退到 reasoning planner
- discovery：已补 `skill_manifest_version`、`/api/v1/skills*` docs，以及 `skill.select` planner capability descriptor

设计约束：

- Skill 不新增第二套执行器，只负责把场景剧本展开为统一 tool-plan
- Skill 不保存系统连接凭据，也不绕过 connector runtime
- Skill 的高风险步骤仍必须经过授权与审批
- 官方 playbook（如 `disk-space-incident`）优先走 Skill Runtime；reasoning 中的同类硬编码仅保留 fallback 价值

### 4.12 Extensions Manager（2026-03-22 最小版）

为承接受控自扩展，当前新增一层轻量 `Extensions Manager`：

- 核心对象：`ExtensionCandidate`
- 当前 bundle kind：`skill_bundle`
- 当前流程：`generate -> validate -> preview -> import`
- 当前流程：`generate -> validate -> preview -> review -> import`
- import 落点：复用现有 `Skill Registry`

当前实现边界：

- Candidate 当前持久化到 state file，用于跨重启保留治理状态
- docs/tests metadata 随 bundle 一并保存和展示
- review state 与 review history 已纳入 candidate 对象
- enable / disable / promote / rollback 继续由 Skill Manager 承担
- 不允许绕过 Registry 直接写技能配置文件

### 4.8 错误码与返回约定

| 场景 | HTTP | `error.code` | 说明 |
|------|------|--------------|------|
| 签名校验失败 | `401` | `invalid_signature` | webhook secret 不匹配 |
| 请求体校验失败 | `400` | `validation_failed` | 缺字段、格式错误、非法 action |
| 幂等冲突 | `409` | `idempotency_conflict` | 相同 key 命中了不同 payload hash |
| 状态冲突 | `409` | `state_conflict` | version 不匹配或非法状态转移 |
| 资源不存在 | `404` | `not_found` | session / execution / document 不存在 |
| 运维 API 未开启 | `404` | `ops_api_disabled` | 路由未暴露或环境关闭 |
| 能力被开关关闭 | `409` | `blocked_by_feature_flag` | 当前事件或动作被 feature flag 暂停 |
| 下游依赖暂不可用 | `503` | `dependency_degraded` | 模型、VM、Telegram 暂不可用 |
| 系统内部错误 | `500` | `internal_error` | 未归类异常 |

特殊约定：
- Telegram webhook 对“重复回调”始终返回 `200`，避免平台因幂等命中触发 Telegram 重试风暴
- `POST /api/v1/webhooks/vmalert` 在幂等重复时返回 `200`，并在 body 中附带 `duplicated=true`
- 所有 `409` 必须写审计并带上 `trace_id`

---

## 5. 数据库 Schema

### 5.1 存储总览

| 存储 | 用途 |
|------|------|
| PostgreSQL | 业务状态、审计、元数据、outbox |
| SQLite-vec | 文档 chunk 向量索引 |
| 本地文件系统 | 导入文档源文件、临时执行输出缓存 |

### 5.2 PostgreSQL 枚举

```sql
CREATE TYPE session_status AS ENUM (
  'open',
  'analyzing',
  'pending_approval',
  'executing',
  'verifying',
  'resolved',
  'failed'
);

CREATE TYPE execution_status AS ENUM (
  'pending',
  'approved',
  'executing',
  'completed',
  'failed',
  'timeout',
  'rejected'
);

CREATE TYPE risk_level AS ENUM ('info', 'warning', 'critical');

CREATE TYPE outbox_status AS ENUM ('pending', 'processing', 'done', 'failed', 'blocked');
```

### 5.3 核心表

#### `idempotency_keys`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `scope` | `text` | not null |
| `idempotency_key` | `text` | not null |
| `request_hash` | `text` | not null |
| `resource_type` | `text` | 可空 |
| `resource_id` | `text` | 可空 |
| `status` | `text` | `processing` / `completed` / `failed` |
| `response_payload` | `jsonb` | 可空 |
| `first_seen_at` | `timestamptz` | not null |
| `last_seen_at` | `timestamptz` | not null |
| `expires_at` | `timestamptz` | not null |

索引 / 约束：
- `uniq_idempotency_scope_key` unique (`scope`, `idempotency_key`)
- `idx_idempotency_expires_at` (`expires_at`)

说明：
- `vmalert_webhook` 默认保留 7 天
- `telegram_update` 默认保留 3 天
- 定时任务可清理已过期记录

#### `alert_events`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `tenant_id` | `text` | MVP 固定 `default` |
| `external_alert_id` | `text` | 可空 |
| `source` | `text` | not null |
| `severity` | `text` | not null |
| `labels` | `jsonb` | not null |
| `annotations` | `jsonb` | not null |
| `raw_payload` | `jsonb` | not null |
| `fingerprint` | `text` | not null |
| `received_at` | `timestamptz` | not null |

索引：
- `idx_alert_events_fingerprint`
- `idx_alert_events_received_at`

#### `alert_sessions`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `tenant_id` | `text` | not null |
| `alert_event_id` | `uuid` | FK -> `alert_events.id` |
| `status` | `session_status` | not null |
| `service_name` | `text` | 可空 |
| `target_host` | `text` | 可空 |
| `diagnosis_summary` | `text` | 可空 |
| `verification_result` | `jsonb` | 可空 |
| `desense_map` | `jsonb` | 可空 (持久化存储当前会话的脱敏字典映射) |
| `version` | `integer` | not null default 1 (乐观锁并发控制) |
| `opened_at` | `timestamptz` | not null |
| `resolved_at` | `timestamptz` | 可空 |
| `updated_at` | `timestamptz` | not null |

索引：
- `idx_alert_sessions_status`
- `idx_alert_sessions_target_host`
- `idx_alert_sessions_updated_at`

读模型说明：

- `SessionDetail.golden_summary`
- `SessionDetail.notifications`

两者由 `diagnosis_summary / verification_result / executions / session_events` 派生生成，不作为独立数据库列持久化。

#### `session_events`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `session_id` | `uuid` | FK -> `alert_sessions.id` |
| `event_type` | `text` | not null |
| `payload` | `jsonb` | not null |
| `created_at` | `timestamptz` | not null |

说明：
- 作为时间线来源
- 也是问题排查和审计补充

#### `execution_requests`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `tenant_id` | `text` | not null |
| `session_id` | `uuid` | FK -> `alert_sessions.id` |
| `target_host` | `text` | not null |
| `command` | `text` | not null |
| `command_source` | `text` | not null |
| `risk_level` | `risk_level` | not null |
| `requested_by` | `text` | not null |
| `approved_by` | `text` | 可空 |
| `approval_group` | `text` | 可空 |
| `status` | `execution_status` | not null |
| `timeout_seconds` | `integer` | not null default 300 |
| `connector_id` | `text` | 可空，指向选中的 execution connector |
| `connector_type` | `text` | 可空，通常为 `execution` |
| `connector_vendor` | `text` | 可空 |
| `protocol` | `text` | not null，默认 `ssh` |
| `execution_mode` | `text` | not null，默认 `ssh` |
| `output_ref` | `text` | 可空 |
| `output_bytes` | `bigint` | not null default 0 |
| `output_truncated` | `boolean` | not null default false |
| `version` | `integer` | not null default 1 (乐观锁) |
| `created_at` | `timestamptz` | not null |
| `approved_at` | `timestamptz` | 可空 |
| `completed_at` | `timestamptz` | 可空 |

索引：
- `idx_execution_requests_session_id`

读模型说明：

- `ExecutionDetail.session_id` 会显式返回
- `ExecutionDetail.golden_summary` 由 `status / approval_group / output_ref / verification_result / session context` 派生生成，不额外持久化
- `idx_execution_requests_status`
- `idx_execution_requests_created_at`

说明：
- `connector_* / protocol / execution_mode` 用于把 diagnosis draft、审批、执行结果、trace、knowledge 全部对齐到统一 connector runtime，而不是只保留 SSH 语义
- 当未匹配到 execution connector 时，仍允许以 `ssh / ssh` 作为 fallback，保持 MVP 可回退

#### `execution_approvals`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `execution_request_id` | `uuid` | FK -> `execution_requests.id` |
| `action` | `text` | `approve` / `reject` / `modify_approve` / `reassign` / `request_context` |
| `actor_id` | `text` | not null |
| `actor_role` | `text` | 可空 |
| `original_command` | `text` | 可空 |
| `final_command` | `text` | 可空 |
| `comment` | `text` | 可空 |
| `created_at` | `timestamptz` | not null |

#### `execution_output_chunks`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `bigserial` | PK |
| `execution_request_id` | `uuid` | FK -> `execution_requests.id` |
| `seq` | `integer` | not null |
| `stream_type` | `text` | `stdout` / `stderr` |
| `content` | `text` | not null |
| `byte_size` | `integer` | not null |
| `retention_until` | `timestamptz` | not null |
| `created_at` | `timestamptz` | not null |

存储策略：
- 单次执行默认最多在 PostgreSQL 保留 `256KB` 输出，按 `16KB` 分块写入
- 超过阈值后，继续流式写入本地 spool 文件或对象存储，并将路径记录到 `execution_requests.output_ref`
- `execution_requests.output_truncated=true` 表示数据库仅保留了前缀输出，不代表底层执行被截断
- `execution_output_chunks` 默认保留 7 天，之后由后台任务清理；审计表只保留摘要、退出码、是否截断

索引 / 约束：
- `uniq_execution_output_seq` unique (`execution_request_id`, `seq`)
- `idx_execution_output_retention` (`retention_until`)

#### 知识落库统一规则

- `documents` / `document_chunks` 是所有可检索知识的唯一检索源
- `knowledge_records` 是闭环会话的结构化摘要，不直接参与向量检索
- 当 `session_closed` 事件到达时，Knowledge Service 必须按以下顺序执行：
  1. 以 `source_type=session`、`source_ref=<session_id>` 对 `documents` 做幂等 upsert
  2. 生成或重建该 `document_id` 下的 `document_chunks`
  3. 写入或更新 `knowledge_records(document_id, session_id, ...)`
  4. 写入 / 重建 SQLite-vec 向量
- 文档型知识源和闭环会话知识共用同一套 `documents -> document_chunks -> chunk_vectors` 检索链路

#### `documents`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `tenant_id` | `text` | not null |
| `source_type` | `text` | `file` / `session` |
| `source_ref` | `text` | not null |
| `title` | `text` | not null |
| `content_hash` | `text` | not null |
| `status` | `text` | `active` / `deleted` |
| `created_at` | `timestamptz` | not null |
| `updated_at` | `timestamptz` | not null |

索引 / 约束：
- `uniq_documents_source` unique (`tenant_id`, `source_type`, `source_ref`)
- `idx_documents_status` (`status`, `updated_at`)

#### `document_chunks`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `document_id` | `uuid` | FK -> `documents.id` |
| `tenant_id` | `text` | not null |
| `chunk_index` | `integer` | not null |
| `content` | `text` | not null |
| `token_count` | `integer` | 可空 |
| `citation` | `jsonb` | not null |
| `created_at` | `timestamptz` | not null |

索引 / 约束：
- `uniq_document_chunk_index` unique (`document_id`, `chunk_index`)

#### `knowledge_records`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `tenant_id` | `text` | not null |
| `session_id` | `uuid` | FK -> `alert_sessions.id` |
| `document_id` | `uuid` | FK -> `documents.id` |
| `summary` | `text` | not null |
| `content` | `jsonb` | not null |
| `status` | `text` | `active` / `revoked` |
| `created_at` | `timestamptz` | not null |

索引 / 约束：
- `uniq_knowledge_record_session` unique (`tenant_id`, `session_id`)

说明：
- `revoked` 触发条件：Session 被人工标记为误报或无效时，关联的 `KnowledgeRecord` 自动置为 `revoked`，对应的 `document_chunks` 和 `chunk_vectors` 不再参与检索
- 已 `revoked` 的记录不会被物理删除，保留审计可追溯性

#### `skill_drafts` (Phase 2a)

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `tenant_id` | `text` | not null |
| `name` | `text` | not null |
| `status` | `text` | `draft` / `reviewed` / `active` |
| `yaml_content` | `text` | not null |
| `review_owner` | `text` | not null |
| `created_by` | `text` | not null |
| `reviewed_by` | `text` | 可空 |
| `created_at` | `timestamptz` | not null |
| `reviewed_at` | `timestamptz` | 可空 |

#### `audit_logs`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `bigserial` | PK |
| `tenant_id` | `text` | not null |
| `trace_id` | `text` | 可空 |
| `actor_id` | `text` | 可空 |
| `resource_type` | `text` | not null |
| `resource_id` | `text` | not null |
| `action` | `text` | not null |
| `payload` | `jsonb` | not null |
| `created_at` | `timestamptz` | not null |

#### `outbox_events`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `topic` | `text` | `session_closed` / `telegram_send` / `approval_timeout` / `reindex_document` |
| `aggregate_id` | `text` | not null |
| `payload` | `jsonb` | not null |
| `status` | `outbox_status` | not null default `pending` |
| `available_at` | `timestamptz` | not null |
| `retry_count` | `integer` | not null default 0 |
| `last_error` | `text` | 可空 |
| `blocked_reason` | `text` | 可空 |
| `created_at` | `timestamptz` | not null |

索引：
- `idx_outbox_poll` (`status`, `available_at`) （用于 Worker 轮询优化）
- `idx_outbox_failed_blocked` (`status`, `created_at`) （用于运维查询和人工重放）

### 5.4 SQLite-vec Schema

```sql
CREATE TABLE chunk_vectors (
  chunk_id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  document_id TEXT NOT NULL,
  embedding BLOB NOT NULL
);
```

约束：
- `chunk_id` 与 PostgreSQL `document_chunks.id` 一一对应
- 删除或重建文档时，先更新 PostgreSQL 状态，再异步更新 SQLite-vec

### 5.5 Phase 2a 扩展表

#### `tickets`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `tenant_id` | `text` | not null |
| `status` | `text` | `pending` / `in_progress` / `verifying` / `resolved` / `closed` |
| `title` | `text` | not null |
| `service_name` | `text` | 可空 |
| `env` | `text` | 可空 |
| `aggregation_key` | `text` | not null |
| `opened_at` | `timestamptz` | not null |
| `resolved_at` | `timestamptz` | 可空 |
| `closed_at` | `timestamptz` | 可空 |
| `created_at` | `timestamptz` | not null |
| `updated_at` | `timestamptz` | not null |

#### `ticket_sessions`

| 列 | 类型 | 约束 |
|----|------|------|
| `ticket_id` | `uuid` | FK -> `tickets.id` |
| `session_id` | `uuid` | FK -> `alert_sessions.id` |
| `relation_type` | `text` | `primary` / `merged` / `split_from` |
| `created_at` | `timestamptz` | not null |

索引 / 约束：
- `uniq_ticket_session` unique (`ticket_id`, `session_id`)

#### `channel_identities`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `tenant_id` | `text` | not null |
| `channel` | `text` | `telegram` / `feishu` |
| `external_user_id` | `text` | not null |
| `display_name` | `text` | 可空 |
| `mapped_actor_id` | `text` | not null |
| `created_at` | `timestamptz` | not null |
| `updated_at` | `timestamptz` | not null |

索引 / 约束：
- `uniq_channel_identity` unique (`tenant_id`, `channel`, `external_user_id`)

### 5.6 Phase 2b 路由持久化表

#### `service_ownership_rules`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `tenant_id` | `text` | not null |
| `service_name` | `text` | not null |
| `owner_type` | `text` | `user` / `group` |
| `owner_ref` | `text` | not null |
| `priority` | `integer` | not null default 100 |
| `is_active` | `boolean` | not null default true |
| `created_at` | `timestamptz` | not null |
| `updated_at` | `timestamptz` | not null |

索引 / 约束：
- `idx_service_owner_active` (`tenant_id`, `service_name`, `is_active`, `priority`)

#### `approval_groups`

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | `uuid` | PK |
| `tenant_id` | `text` | not null |
| `group_key` | `text` | not null |
| `display_name` | `text` | not null |
| `group_type` | `text` | `oncall` / `service_owner` / `knowledge_admin` |
| `member_refs` | `jsonb` | not null |
| `is_active` | `boolean` | not null default true |
| `created_at` | `timestamptz` | not null |
| `updated_at` | `timestamptz` | not null |

索引 / 约束：
- `uniq_approval_group_key` unique (`tenant_id`, `group_key`)

> MVP 中 `服务 owner / 值班组` 路由先用配置文件实现；Phase 2b 再迁移到以上数据库表，但字段和优先级规则在本设计中已固定。

### 5.7 事务边界与一致性规则

核心原则：
- 单个数据库事务内只做本地状态变更，不跨外部网络调用
- 所有“状态变更后需要异步副作用”的场景统一通过 outbox 保证至少一次投递
- 所有写路径必须先做幂等校验，再做状态变更

关键事务定义：

| 事务名 | 包含写入 | 不包含 | 失败处理 |
|--------|----------|--------|----------|
| `TX_ALERT_INGEST` | `idempotency_keys`、`alert_events`、`alert_sessions`、`session_events` | 模型调用、VM 查询、Telegram 发送 | 整体回滚，允许 webhook 重试 |
| `TX_APPROVAL_APPLY` | `idempotency_keys`、`execution_approvals`、`execution_requests`、`session_events` | SSH 执行、消息发送 | 整体回滚，重复 callback 可安全重放 |
| `TX_SESSION_RESOLVE` | `alert_sessions`、`session_events`、`outbox_events(session_closed)` | 知识入库、embedding | 整体回滚；成功后由 Worker 重试异步任务 |
| `TX_REINDEX_REQUEST` | `audit_logs`、`outbox_events(reindex_document)` | 实际切 chunk / embedding | 失败即返回错误，不做部分成功 |

实现约束：
- `Workflow Core` 对 `alert_sessions` / `execution_requests` 的更新必须使用 `WHERE id=? AND version=?`
- 外部调用结果回写前必须重新读取最新状态，确认请求仍处于预期状态
- `Knowledge Ingest` 必须设计为幂等 upsert；重复消费 `session_closed` 不得产生重复 chunk 或重复 record

---

## 6. 状态机设计

### 6.1 `AlertSession` 状态机

| 当前状态 | 事件 | 条件 | 下一个状态 | 动作 |
|----------|------|------|------------|------|
| `open` | `session_created` | 自动 | `analyzing` | 触发上下文收集 |
| `analyzing` | `diagnosis_ready` | 无需执行 | `resolved` | 发送诊断结果并结束 |
| `analyzing` | `execution_draft_ready` | 需要执行 | `pending_approval` | 发送审批消息 |
| `pending_approval` | `approved` | 审批通过 | `executing` | 下发执行 |
| `pending_approval` | `rejected` | 审批拒绝 | `analyzing` | 记录原因，重新分析 |
| `pending_approval` | `request_context` | 需要补充 | `analyzing` | 触发补充查询 |
| `executing` | `execution_completed` | 成功 | `verifying` | 触发校验 |
| `executing` | `execution_failed` | 失败/超时 | `failed` | 通知失败 |
| `verifying` | `verify_success` | 检查通过 | `resolved` | 写 outbox `session_closed` |
| `verifying` | `verify_failed` | 未恢复 | `analyzing` | 回到分析 |
| `failed` | `retry` | 人工或系统重试 | `analyzing` | 重新分析 |
| `failed` | `manual_resolve` | 人工确认已解决 | `resolved` | 写 outbox `session_closed` |

不变量：
- 任意时刻只有一个活动中的 `ExecutionRequest`
- 只有 `Workflow Core` 可以写 `alert_sessions.status`
- **并发控制：必须携带预期 `version` 发起 `UPDATE`，失败则触发重试，防并发丢状态。**
- `resolved` 状态不可直接跳回 `executing`

### 6.2 `ExecutionRequest` 状态机

| 当前状态 | 事件 | 下一个状态 |
|----------|------|------------|
| `pending` | `approve` | `approved` |
| `pending` | `reject` | `rejected` |
| `pending` | `approve_and_modify` | `approved` |
| `approved` | `execution_started` | `executing` |
| `executing` | `execution_success` | `completed` |
| `executing` | `execution_error` | `failed` |
| `executing` | `execution_timeout` | `timeout` |

不变量：
- `approved_by` 仅在 `approved` 及之后状态可非空
- `critical` 风险必须满足双审批后才能进入 `approved`
- 修改命令后批准必须保留原始命令和最终命令

### 6.3 `Ticket` 状态机（Phase 2a）

| 状态 | 进入条件 |
|------|----------|
| `pending` | 新 Ticket 创建 |
| `in_progress` | 至少有一个关联 Session 进入 `analyzing/executing` |
| `verifying` | 所有关联 Session 均进入校验中 |
| `resolved` | 所有关联 Session 已解决 |
| `closed` | 人工确认关闭 |

聚合规则（Phase 2a 默认）：
- 以 `service + env + 30min 时间窗` 为默认聚合键
- 支持人工拆分和人工合并

---

## 7. 审批与路由设计

### 7.1 审批路由优先级

1. 按告警 `labels.service` 命中 `service owner`
2. 若无 `service owner`，回退到当前 `oncall group`
3. `critical` 需要双审批，其中至少一位来自 `service owner / oncall lead`
4. 提议者和审批人默认不能为同一人；若命中同一人，则自动回退到 `oncall group`
5. 审批转交后 SLA 继承原请求，不重新起算，仅记录新的 `approval_group`

### 7.2 MVP 路由配置示例

```yaml
approval:
  default_timeout: 15m
  prohibit_self_approval: true  # 提议者与审批人不可为同一人
  routing:
    service_owner:
      web: ["u_alice", "u_bob"]
      payment: ["u_charlie"]
    oncall_group:
      default: ["u_sre_1", "u_sre_2"]
```

### 7.3 审批消息模板

```text
[TARS 审批请求]
告警: HighCPUUsage / prod-web-01
风险: warning
建议命令: systemctl restart nginx
原因: CPU 持续高于 90%，Nginx worker 异常
回滚提示: systemctl status nginx && journalctl -u nginx -n 100
时限: 15 分钟
来源: service owner(web)
操作: [批准执行] [拒绝] [修改后批准] [要求补充信息]
```

### 7.4 命令与能力授权策略演进

MVP 当前已经具备：

- `TARS_SSH_ALLOWED_HOSTS`
- `TARS_SSH_ALLOWED_COMMAND_PREFIXES`
- `TARS_SSH_BLOCKED_COMMAND_FRAGMENTS`
- `approval.execution.command_allowlist.<service>`

这套机制足够支撑 MVP，但它还不是统一的授权模型。后续建议把 SSH 命令和 MCP skill 都统一到同一套策略语言中。

推荐默认语义：

- 白名单：`direct_execute`
- 黑名单：`suggest_only`
- 其他：`require_approval`

同时保留两点扩展：

- 少量 `hard_deny` 规则不可被普通策略覆盖
- 更具体的覆盖规则可以把某些黑名单项改为 `require_approval`

策略规则建议支持：

- glob 通配符匹配
- 按服务、主机、渠道、skill/server 维度覆盖
- 审计记录 `rule_id / final_action / approval_route`

详细设计见：

- [命令与能力授权策略](../specs/30-strategy-command-authorization.md)
- [vNext 授权配置样例](../configs/authorization_policy.vnext.example.yaml)

当前实现补充：

- `/ops` 已支持通过引导式表单或 Advanced YAML 热更新：
  - `authorization`
  - `approval routing`
  - `reasoning prompts`
- 这些配置都会写回运行时文件并在当前进程内立即生效，不需要重启

能力级授权（已实现）：

- 授权 `Evaluator` 接口新增 `EvaluateCapability(CapabilityInput)` 方法，统一评估连接器能力的调用权限
- `read_only: true` 的能力默认授予 `direct_execute`
- 非只读能力默认进入 `require_approval`
- MCP / Skill 类能力当前默认 `hard_deny_mcp_skill`，直到显式配置放行策略
- 审计记录包含 `capability_id / connector_id / final_action`

### 7.5 MCP Skill 外部源预留设计

MCP skill 后续不应仅支持“本地静态注册”，还需要支持通过外部源地址导入。建议在技术上预留独立的 source registry：

| 字段 | 说明 |
|------|------|
| `source_id` | 外部源唯一标识 |
| `source_type` | `http_index` / `git_repo` / 后续 `yum_like_repo` |
| `base_url` | 源地址 |
| `auth_ref` | 凭据引用，不直接存明文 secret |
| `enabled` | 是否启用 |
| `last_synced_at` | 最近同步时间 |

设计原则：

- source registry 负责“从哪里发现和同步 skill”
- authorization policy 负责“导入后的 skill 如何授权”
- 两层解耦，避免把仓库同步和执行授权混在一起

---

## 8. 脱敏与模型调用设计

### 8.1 脱敏范围

| 类型 | 策略 |
|------|------|
| IPv4 / IPv6 | 替换为稳定占位符 |
| 主机名 / 域名 | 替换为稳定占位符 |
| 路径 / 文件 | 替换为稳定占位符 |
| 密码 / Token / Secret | 替换为 `[REDACTED]`，永不回填 |

### 8.2 配置化控制面

当前运行时已支持通过 `TARS_DESENSITIZATION_CONFIG_PATH` 加载并热更新脱敏规则。配置维度包括：

- `secrets`
  - `key_names`
  - `query_key_names`
  - `additional_patterns`
  - `redact_bearer`
  - `redact_basic_auth_url`
  - `redact_sk_tokens`
- `placeholders`
  - `host_key_fragments`
  - `path_key_fragments`
  - `replace_inline_ip / host / path`
- `rehydration`
  - `host / ip / path`
- `local_llm_assist`
  - `enabled / provider / base_url / model / mode`

### 8.3 调用流程

```text
原始上下文
-> 提取需脱敏目标，生成占用符字典 -> 写入 AlertSession 的 desense_map
-> 执行正则替换，获取脱敏串
-> 按协议调用模型（`openai_compatible / anthropic / ollama / lmstudio`）
-> 返回脱敏建议
-> 根据 Session 的 desense_map 有限回填
-> 返回 Workflow Core
```

### 8.4 本地 LLM 辅助脱敏

本地 LLM 辅助脱敏不应替代规则式脱敏。

当前设计原则：

- 规则式脱敏是安全边界
- 本地 LLM 仅作为增强层，用于补充识别非标准敏感信息
- 平台最终仍负责替换、回填和审计

后续推荐模式：

```text
原始上下文
-> 规则式脱敏
-> 可选：本地 LLM detect_only 返回敏感值列表（`secrets / hosts / ips / paths`）
-> 平台统一替换并合并映射
-> 外部/主模型调用
```

说明：

- `local_llm_assist` 当前已支持 `detect_only`
- 失败时会直接回退到纯规则式脱敏
- 当前主链路仍不会把它作为唯一脱敏器
- 审计继续保留 `request_raw / request_sent`

### 8.5 失败策略

- 模型调用超时：直接返回 `diagnosis_unavailable`
- 脱敏失败：拒绝调用外部模型，记录审计并退化为人工接管

---

## 9. 项目结构

```text
TARS/
├── cmd/
│   └── tars/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── bootstrap.go
│   │   ├── router.go
│   │   └── workers.go
│   ├── api/
│   │   ├── http/
│   │   │   ├── middleware/
│   │   │   ├── webhook_handler.go
│   │   │   ├── telegram_handler.go
│   │   │   ├── session_handler.go
│   │   │   └── ops_handler.go
│   │   └── dto/
│   ├── contracts/
│   │   ├── command.go
│   │   ├── event.go
│   │   └── query.go
│   ├── foundation/
│   │   ├── audit/
│   │   ├── auth/
│   │   ├── config/
│   │   ├── logger/
│   │   ├── metrics/
│   │   └── tracing/
│   ├── domain/
│   │   ├── alert/
│   │   ├── session/
│   │   ├── execution/
│   │   ├── knowledge/
│   │   └── ticket/
│   ├── modules/
│   │   ├── alertintake/
│   │   ├── channel/
│   │   │   └── telegram/
│   │   ├── workflow/
│   │   ├── reasoning/
│   │   ├── action/
│   │   │   ├── ssh/      # Command Executor with Timeout Wrapping
│   │   │   └── provider/
│   │   └── knowledge/
│   ├── repo/
│   │   ├── postgres/
│   │   └── sqlitevec/
│   └── events/
│       ├── outbox.go
│       └── dispatcher.go
├── api/
│   └── openapi/
│       └── tars-mvp.yaml
├── migrations/
│   └── postgres/
├── configs/
│   ├── tars.example.yaml
│   ├── approvals.example.yaml
│   └── authorization_policy.vnext.example.yaml
 ├── deploy/
 │   ├── docker/
 │   │   └── docker-compose.yml
 │   └── systemd/
 ├── scripts/
 │   ├── ci/
 │   │   ├── pre-check.sh
 │   │   ├── full-check.sh
 │   │   ├── smoke-remote.sh
 │   │   ├── live-validate.sh
 │   │   └── web-smoke.sh
 │   ├── init_db.sh
 │   └── load_docs.sh
 ├── .github/
 │   └── workflows/
 │       ├── mvp-checks.yml
 │       └── ci-layered.yml
 └── tars_prd.md
```

### 9.1 依赖规则

- `internal/domain/*` 不依赖 `modules/*`
- `modules/*` 可依赖 `domain/*`、`repo/*`、`foundation/*`
- `contracts/*` 只定义跨模块 DTO / interface，不依赖 `repo/*` 和 `api/*`
- `api/http/*` 只能调用 `modules/*` 暴露的 application service
- `repo/*` 不依赖 `api/*`

---

## 10. 配置设计

### 10.1 `configs/tars.example.yaml`

```yaml
server:
  listen: ":8080"
  public_base_url: "https://tars.example.com"

postgres:
  dsn: "postgres://user:pass@localhost:5432/tars?sslmode=disable"

vector:
  sqlite_path: "./data/tars_vec.db"

model:
  provider: "openai_compatible"
  base_url: "https://model-gateway.example.com/v1"
  api_key: "${MODEL_API_KEY}"
  timeout: "30s"

ops_api:
  enabled: false
  listen: "127.0.0.1:8081"
  bearer_token: "${OPS_API_TOKEN}"
  trusted_proxy_cidrs: ["10.0.0.0/8"]
  require_gateway_identity: true

telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  webhook_secret: "${TELEGRAM_WEBHOOK_SECRET}"

vmalert:
  webhook_secret: "${VMALERT_WEBHOOK_SECRET}"

victoriametrics:
  query_base_url: "https://vm.example.com/select/0/prometheus"
  timeout: "15s"

ssh:
  user: "sre"
  private_key_path: "/etc/tars/id_rsa"
  connect_timeout: "10s"
  command_timeout: "300s" # Required: SSH 调用需被 `context.WithTimeout` 包裹，且底层 Shell 封套 `timeout -k 10s 300s`

execution_output:
  max_persisted_bytes: 262144
  chunk_bytes: 16384
  retention: "168h"
  spool_dir: "./data/execution_output"

approval:
  timeout: "15m"
  critical_dual_approval: true
```

> **安全与重载约束**:
> - 从环境变量读取注入（如 `$MODEL_API_KEY`）的隐私字段，严禁使用基础 `%+v` 序列化写入日志；
> - 对于 `approvals.yaml` 这种动态团队归属设置，要求通过 `fsnotify` 监听变化或提供系统接收 `SIGHUP` 信号以零停机热加载。

---

## 11. 后台任务与异步处理

| Worker | 触发方式 | 职责 |
|--------|----------|------|
| `Outbox Dispatcher` | 常驻轮询 | 分发事件，必须使用 `SELECT FOR UPDATE SKIP LOCKED` 保证原子性防止多 Worker 竞争 |
| `Approval Timeout` | 定时 | 扫描超时审批并执行升级/拒绝 |
| `Message Retry` | 依赖 outbox | 重试 Telegram 发送失败消息 |
| `Knowledge Ingest`| 依赖 outbox | 消费闭环事件并写知识库/向量索引 |
| `Reindex Worker` | 手动触发 | 重建文档 chunk 和 embedding |
| `Idempotency GC` | 定时 | 清理过期 `idempotency_keys` |
| `Execution Output GC` | 定时 | 清理过期输出 chunk 和 spool 文件 |

### 11.1 Outbox 重试策略

统一策略：
- 初始重试延迟 `5s`
- 指数退避，最大退避 `15m`
- 默认最大重试次数 `20`
- `retry_count` 达到上限后将 `outbox_events.status` 置为 `failed`，保留 `last_error`
- 若事件因 feature flag 或运维冻结被暂停，则置为 `blocked`，不再消耗重试次数

事件级规则：
- `telegram_send` 失败后允许自动重试；最终失败只影响通知，不回滚业务状态
- `session_closed` 失败后必须持续重试或人工重放，因为它影响知识沉淀一致性
- `approval_timeout` 失败后必须重试；若最终失败，将审批消息标记为 `escalation_failed` 并通知管理员
- `reindex_document` 失败后由运维 API 触发人工重放
- 每次重试都必须记录 metrics 和 `audit_logs`

阻塞与恢复：
- `knowledge_ingest_enabled=false` 时，新产生的 `session_closed` 事件写入后立即标记为 `blocked`，`blocked_reason=feature_disabled:knowledge_ingest`
- `execution_enabled=false` 时，不生成新的执行类 outbox 事件；已有事件若尚未投递，可转为 `blocked`
- `POST /api/v1/outbox/{event_id}/replay` 只允许对 `failed` 或 `blocked` 事件操作，动作是清空 `last_error/blocked_reason`，重置 `status=pending`
- 人工重放必须写审计，记录操作者、原因、原状态和事件 topic

### 11.2 Event Bus / Message Bus 演进策略

当前阶段，TARS 继续以 `PostgreSQL outbox + worker` 作为主异步底座，这是当前最合适的实现：

- 与 PostgreSQL 事务一致性天然协同
- 运维成本较低
- 适合当前模块化单体架构
- 足以支撑当前通知、知识沉淀、审批超时、自动化等异步副作用

但从当前阶段开始，业务模型不应继续直接依赖 `outbox_events` 表与 SQL polling 语义。后续应补齐统一 `Event Bus` 抽象，至少收口：

- `Publish(topic, event)`
- `Subscribe(topic, handler)`
- `Ack`
- `Retry`
- `DeadLetter`

推荐演进路线：

1. 当前继续强化 PostgreSQL outbox
2. 现在补齐 Event Bus 抽象
3. 当以下条件明显成立时，再正式接入消息系统：
   - 多实例 worker 竞争明显增多
   - fan-out 消费者和 consumer group 成为刚需
   - 通知 / 自动化 / 扩展构建 / 审计导出开始拆成独立进程或服务
   - 需要更强的 replay / retention / cross-service consumption

后续优先评估顺序建议为：

1. `NATS JetStream`
2. `RabbitMQ`
3. `Kafka`（仅在更大规模事件平台化阶段考虑）

也就是说，当前不建议过早直接上重型 MQ，但必须现在就把异步接口收口好，为未来演进留出明确边界。

---

## 12. 观测与审计

### 12.1 核心指标

- `tars_alert_events_total`
- `tars_alert_sessions_total`
- `tars_session_status_total{status=...}`
- `tars_execution_requests_total`
- `tars_execution_duration_seconds`
- `tars_model_call_duration_seconds`
- `tars_plugin_call_total{provider=...}`
- `tars_telegram_send_failures_total`
- `tars_outbox_backlog_total`
- `tars_idempotency_duplicates_total{scope=...}`
- `tars_execution_output_truncated_total`
- `tars_outbox_retries_total{topic=...}`
- `tars_outbox_final_failures_total{topic=...}`

### 12.2 审计要求

必须审计以下动作：
- 告警接入
- Session 状态变更
- 执行请求创建
- 审批动作
- 执行开始 / 完成 / 失败
- 模型调用（脱敏后的上下文摘要）
- 知识沉淀和 SkillDraft 审核

---

## 13. 部署方案

### 13.1 MVP 部署拓扑

```text
1 x tars service
1 x postgresql
1 x local sqlite-vec file
1 x ingress / reverse proxy
1 x internal-only ops listener (optional)
external: Telegram / VMAlert / Model Gateway / VictoriaMetrics / SSH hosts
```

### 13.2 Docker Compose 组件

- `tars`
- `postgres`
- `init-db`（可选）

### 13.3 就绪检查

`/readyz` 需检查：
- PostgreSQL 可连接
- SQLite-vec 文件可打开
- 配置加载成功
- Telegram 和模型网关不作为 hard dependency；不可用时服务仍可 ready，但会在健康详情中标记 degraded
- 若 `ops_api.enabled=true`，仅检查内部监听是否绑定成功，不校验外部代理可达性

### 13.4 发布与回滚策略

MVP 试点采用分阶段开关发布：

1. `diagnosis_only`
   - 开启告警接入、VM 查询、模型诊断、Telegram 发送
   - 关闭审批、SSH 执行、知识沉淀
2. `approval_beta`
   - 开启审批消息、审批回调、超时升级
   - SSH 执行仍只对白名单服务开放
3. `execution_beta`
   - 开启 SSH 执行和验证闭环
   - 知识沉淀延后到闭环稳定后开启
4. `knowledge_on`
   - 开启 `session_closed` ingest 和向量更新

建议的 feature flags：
- `features.diagnosis_enabled`
- `features.approval_enabled`
- `features.execution_enabled`
- `features.knowledge_ingest_enabled`

说明：
- `Ops API` 不走 `features.*` 控制面，只由 `ops_api.enabled` 单独控制，避免两套开关互相打架
- `approval_enabled=false` 时，新的执行建议不进入 `pending_approval`，而是作为“需人工线下处理”结果返回
- `execution_enabled=false` 时，审批消息仍可查看，但通过后的请求不会进入真实执行
- `knowledge_ingest_enabled=false` 时，`session_closed` 事件不会丢弃，而是进入 `outbox_events.status=blocked`

回滚原则：
- 优先关 feature flag，不先回滚数据库 schema
- `approval_enabled=false` 后，新的执行建议不再投递审批消息，已有 `pending_approval` 请求转人工接管
- `execution_enabled=false` 后，已有 `pending_approval` 请求只允许拒绝或转人工，不再执行
- `knowledge_ingest_enabled=false` 后，`session_closed` 事件转入 `blocked`，待恢复后由 Worker 或运维 API 重放
- 任一外部依赖抖动时允许降级到 `diagnosis_only`

---

## 14. 测试方案

自动化测试的完整分层策略见 [30-strategy-automated-testing.md](../specs/30-strategy-automated-testing.md)。这里保留高层结构，作为 TSD 的测试基线。

### 14.1 单元测试

- Alert mapping / fingerprint
- Session 状态机转移
- Execution risk classifier
- Desensitizer masking / refill
- Approval routing

### 14.2 集成测试

- VMAlert webhook -> session created
- Telegram callback -> approval applied
- SSH execute -> output stored
- session_closed -> knowledge ingest
- duplicate webhook / callback -> dedupe and no duplicate session / approval
- same `(scope, idempotency_key)` with different payload hash -> `409 Conflict` + audit
- oversized command output -> DB truncation + spool fallback
- session knowledge materialization -> `documents` + `document_chunks` + `knowledge_records` 一致

### 14.3 端到端验收

- 从真实告警样本触发完整闭环
- 观察消息下发、审批、执行、审计、知识沉淀是否全部完成

### 14.4 发布前演练

- 关闭 `execution_enabled` 的只读诊断演练
- 开启 `execution_enabled` 的白名单服务演练
- 模型超时 / VM 超时 / Telegram 回调重放 / SSH 超时四类故障注入
- 数据库恢复演练：验证 outbox 未丢消息、幂等表未失效

### 14.5 自动化测试梯度

当前统一按以下梯度组织自动化测试：

- `L0` 快速本地预检：`make pre-check` -> `scripts/ci/pre-check.sh`
- `L1` 标准本地回归：`make check-mvp` / `make full-check`
- `L2` 定向平台回归：模块单测、契约测试、fixture 回归、`make web-smoke`
- `L3` 共享环境 live validation：`make deploy-sync` / `make smoke-remote` / `make live-validate` / `make deploy`
- `L4` 手工 / 高成本验收：`run_demo_smoke.sh`、auth/extensions live validation、官方 playbook 演练

设计目标：

- 让较弱的 agent 可以安全承担 `L0-L2` 与脚本化 `L3`
- 只把高风险 live 配置切换、`L4` 官方演练留给强 agent 或人工把关

### 14.6 工程化执行入口

本轮工程化收口后，测试与共享环境验证统一分成三层：

| 层 | 入口 | 说明 |
|----|------|------|
| `Makefile` | `make pre-check/full-check/deploy/...` | 对开发者和弱 agent 暴露稳定命令名 |
| `scripts/ci/` | `pre-check.sh/full-check.sh/smoke-remote.sh/live-validate.sh/web-smoke.sh` | 负责编排已有脚本、输出更适合 CI 的日志 |
| 底层成熟脚本 | `check_mvp.sh`、`deploy_team_shared.sh`、`validate_openapi.rb`、`validate_tool_plan_live.sh` | 保留为事实标准，不轻易重写 |

设计约束：

- GitHub Actions 未来只调用 `make` 或 `scripts/ci/*`，不重复拼接业务逻辑
- `deploy_team_shared.sh` 默认串起 `deploy -> smoke-remote -> live-validate`，同时保留 `make deploy-sync` 作为只部署不验证入口
- 部署脚本自动保留远端上一版二进制副本，作为最小恢复手段
- 所有新入口都要输出步骤名、耗时和失败提示，便于 CLI 与 CI 日志排查

---

## 15. Phase 2 / 3 扩展点

### 15.1 Phase 2a

- Ticket 聚合对象
- Web 管理后台
- 手动 SkillDraft 管理
- 飞书渠道
- Ollama 适配

### 15.2 Phase 2b

- LDAP / OIDC
- 多租户
- 审批路由配置持久化
- 审计检索增强

### 15.3 Phase 3

- 插件进程隔离 (gRPC)
- APM / tracing / logging Provider
- Git / CI-CD Provider
- 更丰富的知识源（Confluence / Jira）

### 15.4 外部系统集成框架

当前实现已经从“`VictoriaMetrics + SSH` 为主”推进到“`Prometheus / VictoriaMetrics` 官方 metrics connector + `JumpServer` 官方 execution connector 已可运行”的阶段，但产品化仍不能长期维护 provider/connector 双轨。后续需要把外部系统接入继续收敛成统一连接器框架：

| 连接器类型 | 目标系统 | 作用 |
|------------|----------|------|
| MetricsProvider | `VictoriaMetrics`、`Prometheus` | 查询告警上下文、趋势、历史窗口指标 |
| ObservabilityProvider | APM / tracing / logging | 查询 trace、span、错误、日志片段 |
| ExecutionProvider | `SSH`、`JumpServer` | 执行命令、运行脚本、读取主机状态 |
| DeliveryProvider | `Git`、CI/CD、发布系统 | 查询变更、流水线状态、最近发布 |

设计要求：

- Workflow / Reasoning 只依赖统一 Runtime 能力接口，不直接绑定某个厂商 SDK
- 诊断链优先查询监控/APM/交付系统拿上下文，SSH 仅作为 execution connector 不可用时的 fallback
- 执行类 Runtime 必须继续走授权策略和审批链，不因接入 JumpServer/CI 系统而绕开控制面
- 后续目标是完全以 Connector Registry 取代 legacy provider side path，避免一半走 provider、一半走 connector 的双轨维护

### 15.5 连接器平台与开放接口

为了让其他系统能主动接入 TARS，而不是每种系统都单独写定制适配，后续要把外部接入平台化：

- 提供公开发现入口：`GET /api/v1/platform/discovery`
- 定义统一连接器清单：`tars.connector/v1alpha1`
- 定义市场包版本：`tars.marketplace/v1alpha1`
- 统一支持导入、导出、升级、回滚

连接器平台的关键目标：

| 目标 | 说明 |
|------|------|
| 可发现 | 外部系统或自动化工具能发现 TARS 支持哪些接入能力 |
| 可注册 | 新系统通过 manifest 声明能力和兼容关系 |
| 可升级 | 系统升级时可通过导入导出和版本校验完成迁移 |
| 可审计 | 导入、导出、启停、升级和回滚都进入审计 |

统一 manifest 至少声明：

| 字段 | 说明 |
|------|------|
| `metadata.id` | 连接器唯一标识 |
| `spec.type` | `metrics / execution / observability / delivery / mcp_tool / skill_source` |
| `spec.protocol` | 协议或接入方式 |
| `capabilities` | 暴露能力、是否只读、作用域 |
| `connection_form` | 控制台自动生成配置表单所需字段 |
| `compatibility` | TARS 主版本和上游系统主版本兼容矩阵 |
| `import_export` | 是否支持导入导出及包格式 |

平台开放模式：

- `webhook`
- `ops_api`
- `connector_manifest`
- `mcp`

平台要求：

- Workflow / Reasoning 只依赖抽象能力，不依赖具体厂商 SDK
- 连接器升级必须有版本兼容校验，避免上游大版本变化直接打爆线上集成
- 导出包不能回显 secret 明文
- MCP / Skill 外部源纳入同一连接器平台，而不是再发明一套并行体系

第三方系统接入的完整方法分层、可行性评估与推荐顺序见 [docs/30-strategy-third-party-integration.md](../specs/30-strategy-third-party-integration.md)。

当前已实现的 capability runtime 基础设施：

- `CapabilityRuntime` 接口：`Invoke(ctx, manifest, capabilityID, params)` → `(output, error)`
- runtime 按 connector type 注册（observability / delivery / mcp / skill），bootstrap 时注入 stub runtime
- `POST /api/v1/connectors/{id}/capabilities/invoke` 作为统一能力调用 HTTP 入口
- `invocable` 字段已加入 ConnectorCapability manifest model、DTO、TypeScript 类型和 OpenAPI schema
- tool plan step 中 `observability.query` / `delivery.query` / `connector.invoke_capability` 已通过此 runtime 体系执行

详细规范见 [20-component-connectors.md](../specs/20-component-connectors.md)。

### 15.6 Skill 平台（与 Connector Registry 同级）

当前实现里已经有：

- `SkillDraft`
- `skill_source`
- marketplace package 雏形
- 官方 skill package（例如 `disk-space-incident`）

但这些仍不足以说明 Skill 已经成为平台级组件。下一阶段需要把 Skill 独立提升为与 Connector Registry **同级**的系统：

| 能力 | Connector Registry | Skill Registry |
|------|--------------------|----------------|
| 核心对象 | `connector manifest` | `skill package / skill revision` |
| 关注点 | 如何访问一个系统、暴露哪些能力 | 如何把系统能力编排成场景剧本 |
| 控制面 | enable/disable、health、upgrade/rollback、export | create/edit、enable/disable、publish/rollback、export |
| 运行时角色 | 暴露 capability runtime | 展开成 tool-plan / playbook |
| 治理 | 兼容版本、连接配置、secret | 版本、审核、发布、回滚、来源 |

Skill 平台需要至少包含：

- Skill Registry
- Skill CRUD
- Skill Version / Revision
- Skill Publish / Rollback
- Skill Runtime（可展开为 tool-plan）
- Skill 审计与审核

关键约束：

- Skill 不能绕过 Connector Runtime
- Skill 不能直接持有外部系统 secret
- Skill 中的高风险步骤仍必须进入授权/审批链
- `skill_source` 只负责导入来源，不等于已安装且生效的 Skill

建议的 API 轮廓：

- `GET /api/v1/skills`
- `POST /api/v1/skills`
- `GET /api/v1/skills/{id}`
- `PUT /api/v1/skills/{id}`
- `POST /api/v1/skills/{id}/enable`
- `POST /api/v1/skills/{id}/disable`
- `POST /api/v1/skills/{id}/promote`
- `POST /api/v1/skills/{id}/rollback`
- `GET /api/v1/skills/{id}/export`

建议的运行时关系：

- planner 可以先选中 skill
- skill 再展开成标准 tool-plan steps
- tool-plan 继续通过现有 connector / capability runtime 执行

因此，`disk-space-incident` 这类官方剧本的长期目标不应是保留在 reasoning 中的硬编码策略，而应切到 Skill Runtime 主路径。

详细规范见 [20-component-skills.md](../specs/20-component-skills.md)。

### 15.7 其他一级平台组件：Providers / Channels / People

除了 Connector 与 Skill，后续还需要把下面 3 类对象也提升为同级平台组件：

共享 IA 规则补充（2026-03-27）：

- `/connectors` 与 `/providers` 对象页承担日常高频 CRUD / binding / edit 主路径
- `/ops` 收口为平台级低频 raw config、import/export、diagnostics/repair、emergency actions 与 Secrets Inventory
- `/identity*` 是 IAM 日常主入口，不再把 `/ops` 作为 auth/user/provider CRUD 的日常首页

#### Provider Platform

当前实现已经有：

- Provider Registry
- 主模型 / 辅助模型绑定
- models list / availability check
- failover

但它仍更多表现为“配置中心”，而不是完整平台组件。后续需要补齐：

- Provider Registry 作为一等控制面对象
- Provider Role Binding（`primary / assist / local_desensitizer / fallback`）
- Provider Capability / Version Metadata
- Provider Health / Availability History
- Provider 审计与导入导出

当前控制面收口要求：

- `/providers` 负责 provider entry CRUD、primary/assist 绑定、models/check 日常操作
- `/api/v1/config/providers` 继续保留为 `/ops` 下的 raw/high-advanced config API，而不是对象页主入口

#### Channel Platform

当前实现中的 Channel 仍主要体现为 `Channel Adapter + Telegram`。后续应提升为独立平台组件：

- Channel Registry
- 多渠道配置（站内信 / Telegram / Web Chat / 后续 Slack/Feishu/Voice）
- Channel Identity Mapping
- Channel Template / Capability Matrix
- Channel Health / Delivery Status
- Channel Notification / Delivery Capability
- Channel Trigger Binding / Notification Policy

Channel 后续的正式职责不应只限于用户交互入口，还应同时承担：

- `Inbound`
  - 用户消息入口、审批回调、渠道回调
- `Outbound`
  - 诊断结果、审批请求、执行结果、skill/automation 完成通知、周期性报告投递
  - 其中 `站内信 / In-App Inbox` 应作为默认内建通知渠道，优先保证平台内可达和可追溯

建议统一补齐的 channel capability 至少包括：

- `channel.message.text.in`
- `channel.message.text.out`
- `channel.message.image.in`
- `channel.message.image.out`
- `channel.message.audio.in`
- `channel.message.audio.out`
- `channel.message.send`
- `channel.message.preview`
- `channel.template.render`
- `channel.recipient.resolve`
- `channel.delivery.status`

Skill / Automation 不应直接操作 SMTP、Webhook 或 Bot API，而应继续通过 Channel Registry 与 capability runtime 完成送达。

`Web Chat` 后续应作为第一方 Channel，而不只是单独的页面模块。对应的消息模型后续应统一支持：

- text
- image
- audio
- attachments

是否允许某种输入/输出能力，应由以下三者共同决定：

- Channel capability
- Provider / LLM capability
- policy / risk boundary

也就是说，多模态能力后续不应写死在单个模型或单个前端页面里，而应做成统一 capability negotiation。

#### People Platform

当前实现中的 `channel_identities`、审批路由与零散 owner/oncall 信息，还不足以构成统一人物层。后续应提升为独立平台组件：

- People Registry
- 跨渠道 Identity Unification
- Team / Role / Oncall Mapping
- Approval Persona
- User Profile / Preference / Skill Affinity

建议进一步把 People 拆成两类数据：

1. `事实层`
   - team / role / owner / oncall / approval target
2. `画像层`
   - skill_level
   - answer_depth_preference
   - evidence_first vs summary_first
   - preferred_language
   - command/chart/raw-log tolerance

画像层来源建议分为：

- `explicit`
  - 用户主动设置
  - 管理员配置
- `inferred`
  - 从对话记录、审批行为、编辑行为、反馈行为推断
  - 必须保留 `source / confidence / updated_at`

People 平台的目标不是做人力资源系统，而是让 TARS 理解：

- 谁在发起请求
- 谁是审批人
- 谁属于哪个服务/团队
- 对谁应该给什么样的提示、建议、剧本和交互方式

边界约束：

- 推断画像影响回答风格、交互层次、推荐顺序
- 推断画像不直接决定授权或审批权限
- 权限仍由 Users / Authentication / Authorization 平台控制

这三类平台的统一定位见 [10-platform-components.md](../specs/10-platform-components.md)。

### 15.8 用户、认证与权限平台

截至本轮，原始判断已不再成立，当前实现已经进入“access 模块平台化增强”阶段：

- Ops Token
- 默认单租户
- 默认仍兼容单租户，但对象模型已预留 `org_id / tenant_id / workspace_id`
- 已有正式 RBAC 最小角色集与权限矩阵
- `POST /api/v1/auth/login`、`GET /api/v1/auth/callback/{provider}`、`POST /api/v1/auth/logout`、`GET /api/v1/me` 已落地
- `GET /api/v1/auth/sessions` 已落地
- 本轮已增量补齐：
  - `local_password`
  - `POST /api/v1/auth/challenge`
  - `POST /api/v1/auth/verify`
  - `POST /api/v1/auth/mfa/verify`
  - 基于 `bcrypt` 的密码校验
  - 基于 `TOTP` 的 MFA 基础链
  - `/login` 的 password/challenge/MFA 多步骤前端交互

这不足以支撑企业平台化。下一阶段需要补上三层能力：

#### Users Platform

至少应包含：

- User Registry
- User CRUD
- Group / Membership
- Identity Link

#### Authentication Platform

当前基础版已支持或已明确接入：

- `local_token`（break-glass）
- `local_password`
- `password_plus_code`
- `password_plus_2fa`
- `oidc`
- `oauth2`
- `ldap`

当前基础版已具备或已进入 MVP 运行态：

- Auth Provider Registry
- Callback / session / logout
- 外部 subject 与平台 user 的映射
- 密码策略
- 验证码挑战/校验
- MFA 策略

当前边界说明：

- challenge 当前为平台内置 `builtin` 渠道
- API 当前返回 `challenge_code` 作为共享环境验收辅助，不代表最终企业短信/邮件分发形态
- 会话签发仍统一走现有 `issueSession()` 主链，保持 `ops-token`、OIDC/OAuth、审批链、connector runtime、tool-plan 主链兼容

#### Authorization Platform

至少应覆盖：

- 平台 RBAC
- 资源级权限
- 审批权限与平台管理权限边界
- 能力级授权
- 风险等级与审批边界

建议的最小角色：

- `platform_admin`
- `ops_admin`
- `approver`
- `operator`
- `viewer`
- `knowledge_admin`

建议把权限模型进一步标准化为 4 层：

1. `resource`
2. `action`
3. `capability`
4. `risk`

其中：

- `resource + action` 用于平台对象访问控制
- `capability + risk` 用于 tool-plan、connector runtime、skill runtime、automation runtime 的细粒度授权

典型 capability 包括：

- `metrics.query_instant`
- `metrics.query_range`
- `metrics.capacity_forecast`
- `observability.query`
- `delivery.query`
- `channel.message.send`
- `channel.message.read`
- `people.profile.read`
- `people.profile.update`
- `execution.run_command`
- `connector.invoke_capability`

这个模型适用于所有第三方系统，而不只是 VictoriaMetrics / Prometheus。对外部系统接入，不建议只定义粗粒度的系统级 allow/deny，而应优先按“能力 + 风险”等级拆分；未来若接入写动作，应单独归为 `mutating` 或 `high_risk` capability。

实现原则：

- 优先复用成熟开源库，不重复实现 LDAP / OIDC / OAuth / RBAC 协议栈
- 平台只聚焦：
  - identity mapping
  - session integration
  - role binding
  - permission checks
  - 审计

当前技术选型与收口方式：

- OIDC/OAuth: 复用 `golang.org/x/oauth2` + `github.com/coreos/go-oidc/v3`
- RBAC: 复用 `github.com/casbin/casbin/v2`
- Session token: 复用 `github.com/golang-jwt/jwt/v5`

### 15.9 Trigger / Notification / Automation 统一模型

随着 Channel 逐步承担 outbound notification，Skill 和 Automation 也逐步承担更多主动执行职责，触发条件不应继续散落在：

- skill metadata
- channel 配置
- automation 私有 schedule 字段
- 各类页面的零散条件判断

后续建议统一引入 `Trigger / Trigger Policy` 模型，并共享给：

- Channel Notification
- Skill Runtime
- Automation Runtime
- Session / Execution / Approval 事件

第一版建议统一支持：

1. `event`
2. `state`
3. `schedule`
4. `expression`

每条 trigger 至少应包含：

- `source`
- `type`
- `match`
- `filters`
- `cooldown`
- `enabled`
- `action`

收口原则：

- `Channel`
  - 负责怎么发
- `Template`
  - 负责发什么
- `Trigger`
  - 负责什么时候发
- `Skill / Automation`
  - 负责为什么发、整体流程怎么编排

治理要求至少包括：

- dedupe
- rate limit
- silence / cooldown
- preview / dry-run
- audit
- enable / disable

**当前落地状态（2026-03-23）：**

- `trigger.FireEvent` struct 新增 `TemplateData map[string]string`，支持投递时传入渲染变量
- `TriggerWorker` 注入 `*msgtpl.Manager`，`FireEvent()` 当 `TemplateID != ""` 时自动调用 `Render(TemplateData)`，模板不存在时 warn 并 fallback
- 消息模板已从进程内存升级为 PostgreSQL 持久化（`msg_templates` 表），`msgtpl.NewManager(db)` 传入真实 DB
- Skill / Approval 事件接线已完成：
  - `on_approval_requested`：`runImmediateExecution()` 在 `mutation.Status == "pending_approval"` 时调用 `FireApprovalRequested()`
  - `on_skill_completed` / `on_skill_failed`：`dispatchDiagnosis()` 在 skill 执行路径完成后调用 `FireSkillEvent()`，根据步骤失败数自动选择事件类型
- 相关文件：`internal/modules/trigger/manager.go`、`internal/events/trigger_worker.go`、`internal/events/dispatcher.go`、`internal/app/bootstrap.go`、`internal/app/workers.go`

### 15.10 Web Chat 与平台动作调用

后续建议把 `Web Chat` 明确定义为第一方 Channel，并逐步具备：

- 文本输入/输出
- 图片输入/输出
- 语音输入/输出（通过 ASR / TTS / LLM 链路拆分实现）

相关设计约束：

- `ASR`
  - 负责语音转文字
- `LLM`
  - 负责理解、规划、tool-plan / skill 选择
- `TTS`
  - 负责文字转语音

这样可以继续复用文本主链，而不是把语音交互做成单独黑盒。

同时，后续应支持 LLM 通过正式 `platform_action` 调用平台自身能力，例如：

- `platform.automations.create`
- `platform.channels.send`
- `platform.skills.run`
- `platform.notifications.create`

但必须遵守：

- 不直接改底层配置文件
- 不绕过 Registry / Platform API
- 不绕过审批与授权边界

例如“语音设置一个定时任务”的正式形态应是：

- Channel 收到音频
- ASR 产出文本
- LLM 解析出意图与参数
- 通过 `platform.automations.create` 创建候选任务或正式任务
- 按风险等级决定是否需要审批
- 数据与控制面继续收口在 `internal/modules/access`、`internal/api/http/access_handler.go`、`web/src/pages/identity/*`
- `ops-token` 继续作为 break-glass 超级入口，不破坏审批链、connector runtime、tool-plan 主链

详细规范见 [20-component-identity-access.md](../specs/20-component-identity-access.md) 与 [30-strategy-authorization-granularity.md](../specs/30-strategy-authorization-granularity.md)。

### 15.8A 智能体角色模块 (Agent Role Module)

平台中的 AI 智能体（诊断 Agent、自动化 Agent、审核 Agent 等）不应以"无差别全能身份"运行，而应像人类用户一样具备明确的角色定义、能力边界和策略绑定。`Agent Role` 模块为平台提供统一的智能体角色管理能力，使每个 Agent 在运行时拥有清晰的身份画像、能力约束和模型偏好。

#### 模块位置

```text
internal/modules/agentrole/
├── types.go           # AgentRole 核心数据结构与枚举定义
├── manager.go         # CRUD、生命周期管理、运行时解析、内置角色初始化
└── manager_test.go    # 单元测试覆盖
```

依赖规则遵循现有模块化单体约定：

- `agentrole` 可依赖 `foundation/*`（config、audit、logger）
- `agentrole` 不直接依赖 `repo/postgres`；持久化采用 YAML 文件，与 `connectors / skills / automations` 等模块一致
- `workflow / reasoning / automations` 通过注入 `agentrole.Manager` 接口消费角色信息，不直接操作角色存储

#### REST API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/agent-roles` | 列出所有智能体角色（支持分页与过滤） |
| `POST` | `/api/v1/agent-roles` | 创建自定义智能体角色 |
| `GET` | `/api/v1/agent-roles/{id}` | 查询单个角色详情 |
| `PUT` | `/api/v1/agent-roles/{id}` | 更新角色配置 |
| `DELETE` | `/api/v1/agent-roles/{id}` | 删除自定义角色（内置角色不可删除） |
| `POST` | `/api/v1/agent-roles/{id}/enable` | 启用角色 |
| `POST` | `/api/v1/agent-roles/{id}/disable` | 禁用角色 |

API 遵循 §4.1 统一约定与 §4.8 错误码规范。内置角色的 `DELETE` 请求返回 `409 state_conflict`。

#### 数据结构

```go
type AgentRole struct {
    ID          string          `yaml:"id"   json:"id"`
    Name        string          `yaml:"name" json:"name"`
    Description string          `yaml:"description" json:"description"`
    Builtin     bool            `yaml:"builtin" json:"builtin"`
    Enabled     bool            `yaml:"enabled" json:"enabled"`

    // 角色画像：定义 Agent 的行为风格与交互基调
    Profile     AgentProfile    `yaml:"profile" json:"profile"`

    // 能力绑定：声明该角色可调用的 connector capability 与 skill
    Capabilities []CapabilityBinding `yaml:"capabilities" json:"capabilities"`

    // 策略绑定：覆盖默认授权、审批、脱敏策略
    Policies    []PolicyBinding     `yaml:"policies" json:"policies"`

    // 模型绑定：指定该角色的 primary / fallback provider+model 以及是否继承平台默认
    ModelBinding ModelBinding `yaml:"model_binding,omitempty" json:"model_binding,omitempty"`

    CreatedAt   time.Time `yaml:"created_at" json:"created_at"`
    UpdatedAt   time.Time `yaml:"updated_at" json:"updated_at"`
}

type AgentProfile struct {
    Persona         string   `yaml:"persona" json:"persona"`               // 角色定位描述，注入 system prompt
    ResponseStyle   string   `yaml:"response_style" json:"response_style"` // brief / detailed / evidence_first
    AllowedDomains  []string `yaml:"allowed_domains" json:"allowed_domains"` // 该角色可处理的业务域
    RiskTolerance   string   `yaml:"risk_tolerance" json:"risk_tolerance"` // conservative / moderate / aggressive
}

type CapabilityBinding struct {
    CapabilityID string `yaml:"capability_id" json:"capability_id"` // e.g. metrics.query_range
    ConnectorID  string `yaml:"connector_id,omitempty" json:"connector_id,omitempty"`
    Action       string `yaml:"action" json:"action"` // direct_execute / require_approval / deny
}

type PolicyBinding struct {
    PolicyType string `yaml:"policy_type" json:"policy_type"` // authorization / approval / desensitization
    PolicyRef  string `yaml:"policy_ref" json:"policy_ref"`   // 指向具体策略标识
    Override   bool   `yaml:"override" json:"override"`       // 是否覆盖默认策略
}

type ModelTargetBinding struct {
    ProviderID string `yaml:"provider_id,omitempty" json:"provider_id,omitempty"`
    Model      string `yaml:"model,omitempty" json:"model,omitempty"`
}

type ModelBinding struct {
    Primary                *ModelTargetBinding `yaml:"primary,omitempty" json:"primary,omitempty"`
    Fallback               *ModelTargetBinding `yaml:"fallback,omitempty" json:"fallback,omitempty"`
    InheritPlatformDefault bool                `yaml:"inherit_platform_default,omitempty" json:"inherit_platform_default,omitempty"`
}
```

#### 持久化

与 `connectors / skills / automations` 等模块一致，当前阶段采用 YAML 文件持久化：

- 配置路径由 `TARS_AGENT_ROLES_CONFIG_PATH` 环境变量指定
- Manager 启动时加载 YAML，运行时写回
- 内置角色以 seed 方式在首次启动时写入，后续升级时按 `builtin=true` 做幂等合并
- 后续可通过与 §1.5 相同的 persistence hook 机制迁移到 PostgreSQL `runtime_config_documents`

#### 内置角色初始化

首次启动时，Manager 自动注册以下内置角色：

| 角色 ID | 名称 | 定位 | 关键能力 |
|---------|------|------|----------|
| `diagnosis` | 诊断智能体 | 告警诊断主链路中的分析角色 | `metrics.query_range`、`observability.query`、`knowledge.search` |
| `automation_operator` | 自动化执行体 | Automation / Schedule 运行时的执行角色 | `connector.invoke_capability`、`skill.expand`，非只读能力受 `require_approval` 约束 |
| `reviewer` | 审核智能体 | 扩展审核、Skill 审核、知识审核 | `knowledge.search`、`skill.validate`，不具备执行类能力 |
| `knowledge_curator` | 知识策展体 | 知识沉淀、文档处理、向量索引维护 | `knowledge.ingest`、`knowledge.reindex`，不具备执行或通知类能力 |

内置角色可被 disable 但不可被 delete；管理员可覆盖内置角色的 `ModelBinding` 和 `Policies`，但不可移除其 `Builtin` 标记。

#### 运行时角色解析

Manager 提供两个运行时解析方法，供 Workflow / Reasoning / Automation 在创建会话或执行任务时确定当前 Agent 身份：

```go
// ResolveForSession 根据 AlertSession 上下文解析应使用的 AgentRole
// 优先匹配 session 已绑定的 agent_role_id，其次按 alert labels / service 匹配
func (m *Manager) ResolveForSession(ctx context.Context, session *session.AlertSession) (*AgentRole, error)

// ResolveForAutomation 根据 Automation Job 配置解析应使用的 AgentRole
// 优先读取 job.agent_role_id，其次回退到 automation_operator 内置角色
func (m *Manager) ResolveForAutomation(ctx context.Context, job *automations.Job) (*AgentRole, error)
```

解析结果影响：

- system prompt 中注入角色 `Profile.Persona`
- planner / reasoning 的 capability 可见范围受 `Capabilities` 约束
- 授权评估优先使用角色绑定的 `Policies`，再回退到平台默认策略
- 模型调用优先使用角色的 `ModelBinding.primary`
- 当 `ModelBinding.primary` 为空且 `inherit_platform_default=true` 时，回退到平台默认 primary
- 当前选择不可用时，优先尝试 `ModelBinding.fallback`，再回退到平台 assist/shared fallback

实现状态备注（2026-03-29）：

- `diagnosis` 运行时已经落地上述模型选择语义：`dispatcher` 会把角色绑定传给 `reasoning` 的 planner / finalizer，运行时优先按角色绑定解析 provider / model。
- `automation` 的 `skill` 运行时现在也已进入真实 reasoning / model invocation path：执行前会走 role-bound planner，执行后会走 role-bound finalizer，因此结构化 `model_binding` 已对 skill automation 生效。
- `connector_capability` automation 仍保持非 LLM 直调执行链；这一类 run 会沿用 Agent Role 的 policy / capability / audit 语义，但不强行引入无意义的模型调用。

#### 与 Session / Execution / Automation 的关联

| 对象 | 字段 | 说明 |
|------|------|------|
| `alert_sessions` | `agent_role_id text` | 当前会话使用的智能体角色，可空（回退到默认 `diagnosis`） |
| `execution_requests` | `agent_role_id text` | 发起执行的智能体角色，用于审计追溯 |
| `automations.Job` | `agent_role_id string` | 自动化任务绑定的角色，YAML 配置级别 |

`agent_role_id` 在审计记录中作为 `actor_id` 的补充维度，使得"哪个角色做了什么"可追溯。

#### 前端

| 路径 | 说明 |
|------|------|
| `/identity/agent-roles` | 智能体角色管理页面 |

技术方案：

- React + split layout（左侧角色列表，右侧详情/编辑面板）
- 与 `/identity/users`、`/identity/auth-providers` 共享 Identity 导航分组
- 列表支持 enabled/disabled 状态筛选、内置/自定义分类标签
- 详情面板包含：基础信息、Profile 配置、Capabilities 绑定表格、Policies 绑定、Model Binding 表单
- 内置角色详情面板中 `Name / Description / Builtin` 字段为只读，`Policies / ModelBinding` 可编辑
- 创建/编辑表单支持 JSON/YAML 高级编辑模式

#### 设计约束

- Agent Role 不等于 §15.8 中的 RBAC 角色（`platform_admin / operator / viewer` 等）；RBAC 角色管控人类用户的平台权限，Agent Role 管控 AI 智能体的行为边界
- Agent Role 不持有 secret 或连接凭据
- Agent Role 的 capability binding 不能绕过 §7.4 中的 `hard_deny` 规则
- 禁用某 Agent Role 后，已绑定该角色的 running session / automation 应在下次 resolve 时回退到对应内置角色

#### 运行时集成（已实现）

以下运行时集成点已全部落地并通过验证：

| 集成点 | 实现文件 | 说明 |
|--------|----------|------|
| SystemPrompt 注入 | `events/dispatcher.go` → `reasoning/service.go` | Dispatcher 在 `dispatchDiagnosis()` 中调用 `agentrole.Manager.ResolveForSession()` 获取角色，将 `profile.system_prompt` 注入上下文；Reasoning 的 `buildFromModel()` 自动将其前置到 LLM 系统提示词 |
| Auto-assign | `workflow/service.go`、`repo/postgres/workflow.go` | Session 创建时默认赋值 `AgentRoleID: "diagnosis"`（in-memory 和 Postgres 两条路径） |
| Authorization 叠加 | `agentrole/manager.go`、`workflow/service.go` | `EnforcePolicy()` 在现有 `resolveAuthorizationDecision()` 之上叠加 PolicyBinding 约束，取最严格结果 |
| Automation UI | `web/src/pages/automations/AutomationsPage.tsx` | Advanced 区新增 Agent Role 下拉选择器，从 `/api/v1/agent-roles` 拉取可用角色 |
| Postgres 持久化 | `repo/postgres/schema.go`、`repo/postgres/workflow.go` | `agent_roles` 表 DDL，`alert_sessions`/`execution_requests` 新增 `agent_role_id` 列 |

### 15.9 企业级平台治理缺口

即使 `Connectors / Skills / Providers / Channels / People / Users / Authentication / Authorization` 都陆续落地，企业级平台仍会缺一层更高阶的治理能力。

后续需要正式规划的高优先级治理主题包括：

- `Organization / Workspace / Tenant Admin`
- `SCIM / Directory Sync / Identity Lifecycle`
- `Secret Manager / KMS / BYOK / Encryption Governance`
- `Audit Compliance / Legal Hold / SIEM Export`
- `Logging Platform / Log Search / Runtime Log Governance`
- `Built-in Observability / OTLP Export / External Observability Integration`
- `Package Trust / Marketplace Signing / Supply Chain Governance`
- `Session / Device / Access Hardening`
- `Quota / Cost / Usage Governance`
- `Maintenance Window / Silence / Change Window`
- `Data Residency / Air-gapped / Private Deployment Tier`
- `Config Governance / Diff / Drift / Rollback`

这批能力的共同特点是：

- 不直接体现为某个告警剧本
- 但决定平台是否能进入企业长期运行

下一阶段主题见 [92-enterprise-platform-next-phase-topics.md](../specs/92-enterprise-platform-next-phase-topics.md)。

### 15.10 当前平台化阶段判断与后续优先级

截至 2026-03-19，平台化第一阶段的真实状态如下：

- `Provider Registry` 已演进为更通用的 `Connector Registry` 控制面，支持 discovery、list/detail、export、enable/disable、health、upgrade、rollback
- `Prometheus / VictoriaMetrics` 官方 metrics connector 已可通过 runtime query path 查询真实指标
- `JumpServer` 官方 execution connector 已可通过正常 workflow 链路完成 `diagnosis -> pending_approval -> approve -> execute -> verify -> resolved`
- Web 控制台已具备 connectors、secrets、templates、dashboard health 与统一列表能力

因此，后续优先级不再是“是否接入 JumpServer/Prometheus”，而是：

1. 先做 `ToolPlan + metrics.query_range + 媒体附件协议`
2. 收口 Connector 生命周期：`upgrade / rollback / history / health history / compatibility` 的控制面与审计闭环。当前已补齐两条关键语义：显式 `connector_id` 的 metrics 调用失败不再静默回退；`upgrade / rollback` 后当前 health 会先进入 `unknown / runtime health check required after connector change`，并保留 install/rollback 的多快照 revisions
3. 继续消除 legacy provider 双轨，把 metrics / execution 完整收敛到 connector runtime
4. 再推进 `APM / CI-CD / Git / MCP Source / marketplace package`
5. 把 Skill 提升为与 Connector Registry 同级的平台系统，收口 Skill CRUD、revision、publish/rollback 与 Skill Runtime 主路径
6. 把 Providers / Channels / People 逐步提升为一等平台组件，避免它们长期停留在配置或局部模块层
7. 把 Users / Authentication / Authorization 提升为正式平台能力，支持 LDAP / OIDC / OAuth 与平台 RBAC
8. 在上述平台对象逐步成型后，再进入 `Organization / SCIM / KMS / Compliance / Marketplace Trust / Quota` 这类企业级治理层

### 15.11 平台配置导出导入与自动化

随着平台对象越来越多，后续需要把以下三类能力纳入正式技术规划：

1. 全量平台 bundle export/import
2. 模块级 export/import
3. Automations / Scheduled Jobs

配置导出导入不应只是把 yaml 文件打包，而应至少包含：

- schema version
- compatibility check
- diff / preview
- selective apply
- import report
- secret handling mode

同时，后续应引入独立的 `Automation Platform`：

- job registry
- schedule registry
- run history
- skill / connector capability / platform action 调度

这层能力的关键边界是：

- 只读自动化可以按策略自动执行
- 高风险自动化不得绕过审批

补充（2026-03-22 当前已落地基础版）：

- 后端已新增 `internal/modules/automations`
- 配置入口为 `TARS_AUTOMATIONS_CONFIG_PATH`
- 当前采用 `YAML config + state file` 持久化 job/state/run history
- 当前调度模型为进程内 scheduler，支持 cron descriptor、`@every`、retry/backoff、audit
- 执行路径不另起一套 runtime：
  - `skill` 复用 `skills.Manager.Expand`
  - `connector_capability` 复用 `Action.InvokeCapability`
- 控制面接口已落地：
  - `GET/POST /api/v1/automations`
  - `GET/PUT /api/v1/automations/{id}`
  - `POST /api/v1/automations/{id}/enable`
  - `POST /api/v1/automations/{id}/disable`
  - `POST /api/v1/automations/{id}/run`
- Web `/automations` 已可直接操作 create / edit / enable-disable / run now
- 当前安全边界明确为：
  - 非 read-only connector capability 直接 `blocked`
  - capability runtime 若返回 `pending_approval`，automation run 记为 `blocked`
  - `execution.run_command` 在 skill automation 中直接 `blocked`
  - 当前不会主动创建新的 workflow approval session 来替 schedule 穿透审批链

因此当前技术判断是：

- 自动化基础版已经正式进入控制面与运行时
- 但高风险 schedule 仍采取保守阻断，而不是自动拉起完整审批流
- 下一阶段重点应转向 `platform_action`、统一 Trigger runtime，以及高风险 schedule 与 approval workflow 的主链衔接

补充（2026-03-23 第二阶段闭环）：

- **Trigger → Template → Delivery 闭环已完成**：Channel Platform 通知投递现在真正打通，`trigger.FireEvent` 可通过 `TemplateID + TemplateData` 渲染后投递
- **msgtpl PostgreSQL 持久化已完成**：`msg_templates` 表已落库，`msgtpl.NewManager(db)` 传入真实 DB，消息模板持久化不依赖进程重启
- **Skill / Approval 事件接线已完成**：三类事件（`on_approval_requested` / `on_skill_completed` / `on_skill_failed`）均已接入 trigger runtime，可通过 Trigger Policy 绑定 Channel 投递
- **Metrics on-disk 持久化已完成**：`RunRetention()` 每小时写入 `data/observability/metrics/snapshots.jsonl`，`ConfigStatus().Metrics.CurrentBytes` 使用真实磁盘值，metrics 纳入 retention 治理
- **OTLP 最小可验证 exporter 路径已完成**：`tracing.go` 新增 `Ping(ctx) error`（TCP dial 验证 endpoint 可达性）和 `EnabledSignals() []string`，零外部依赖

### 15.12 Skill 驱动的平台对象创建

后续 `connector / channel / provider / people / skill` 等平台对象应支持受控自动创建与更新。

推荐路径：

- Skill 负责编排
- Platform API / Registry 负责真正落库和治理

不推荐路径：

- Skill 直接写底层配置文件
- Skill 绕过平台对象模型直接改内部状态

建议后续引入统一 `platform_action` 能力，例如：

### 15.13 受控自进化 / 自扩展

后续 TARS 可以具备“通过 Skill 和自动化流程生成新扩展”的能力，但不应直接演进成“模型自动改平台核心代码”。

更推荐的技术路径是：

1. `Skill` 生成扩展草稿
2. 产出 `Extension Bundle`
3. 自动补齐 manifest / config schema / tests / docs
4. 进入 validate / compatibility / security / automated tests / review
5. 通过后导入对应 Registry 并启用

建议首批纳入该路径的对象包括：

- connectors
- channels
- providers
- auth providers
- skills
- docs packs

关键边界：

- 生成能力不能绕过审批和审计
- 不能直接改 workflow / auth / approval / desensitization 核心边界
- 高风险扩展不得自动启用
- 文档自动生成也应纳入 publish / rollback 治理

详细规范见 [90-design-self-evolving-platform.md](../specs/90-design-self-evolving-platform.md)。

- `platform.channels.create`
- `platform.providers.create`
- `platform.people.create`
- `platform.skills.import`

这些动作仍需经过：

- validate
- preview / dry-run
- approval（高风险）
- audit

### 15.13 Web Console 全局体验基线

随着控制面扩展到 `Connectors / Skills / Providers / Channels / Identity / People / Automations`，前端需要具备统一的全局壳能力，而不只是页面集合。

后续至少需要补齐：

#### 主题系统

- `light`
- `dark`
- `follow system`

要求：

- 主题状态为全局可持久化状态
- 图表、状态色、弹窗、代码块、表单、详情页统一响应主题切换
- 优先复用成熟的主题方案、设计 token 与现成组件能力，而不是从零自造基础设施

#### 文档中心

右上角统一入口，至少聚合：

- 用户手册
- 管理员手册

后续可扩展：

- 部署手册
- API 参考
- 故障排查

技术要求：

- 文档中心优先读取系统自带文档
- 支持应用内查看或稳定内链跳转
- 页面可附带 context-aware doc shortcuts
- 文档中心后续必须支持搜索，至少覆盖用户手册、管理员手册、部署手册、API 参考与故障排查
- `API 参考` 采用局部升级：保留 Docs Center 路由与目录入口，但 `/docs/api-reference` 页面优先使用内嵌 `Swagger UI` 渲染 `api/openapi/tars-mvp.yaml`
- 除 `API 参考` 外，其它文档继续使用 Markdown 渲染；不要把整套文档中心统一替换成 Swagger 页面
- 文档渲染、全文搜索、目录索引等能力应优先评估并复用成熟开源项目

#### 国际化

至少支持：

- `zh-CN`
- `en-US`

要求：

- 语言状态为全局可持久化状态
- 导航、按钮、状态、错误提示、空态、文档中心入口均需国际化
- 默认可按浏览器/系统语言初始化
- 国际化能力应优先使用成熟 i18n 方案，不建议自造 message catalog 和格式化基础设施

建议后续把这些能力作为 Web Shell 的正式组成部分统一实现，而不是由各页面局部拼装。

### 15.14 平台依赖、兼容性与部署要求

随着平台组件增多，后续还需要把以下 3 类运维与交付信息平台化：

1. 依赖管理
2. 兼容性列表
3. 部署要求

至少应明确：

- 当前依赖清单、版本、可升级状态
- 已知安全漏洞与安全公告
- 各平台组件与第三方系统的兼容版本区间
- 支持的 CPU 架构、操作系统、内存、磁盘、网络要求

这类信息后续不应只停留在 README，应逐步进入：

- 文档真值
- API 真值
- 控制面可视化

详细规范见 [10-platform-dependency-compatibility.md](../specs/10-platform-dependency-compatibility.md)。

详细规划见 [30-strategy-platform-config-and-automation.md](../specs/30-strategy-platform-config-and-automation.md)。

---

## 16. 风险与取舍

| 风险 | 影响 | 当前取舍 |
|------|------|----------|
| 公网模型延迟/不稳定 | 影响诊断速度 | 通过降级策略允许人工接管 |
| SQLite-vec 可扩展性有限 | 文档规模增大后性能下降 | MVP 先接受，Phase 3 再评估外部向量库 |
| SSH 直连审计能力有限 | 企业环境可能要求更强管控 | 当前已优先走 JumpServer execution connector，SSH 保留为 fallback |
| 执行输出过大拖垮主库 | PostgreSQL 膨胀、查询变慢 | 限制 DB 保留大小，超限转 spool / 对象存储 |
| 内部运维 API 暴露不当 | 敏感会话和执行信息泄漏 | 默认关闭，要求独立监听和代理约束 |
| 单体进程边界失守 | 后期代码耦合 | 通过目录结构和依赖规则约束 |

---

*本设计文档用于指导 MVP 到 Phase 2 的研发实现。若 PRD 发生变化，需同步更新本文档中的 API、Schema 和状态机定义。*
