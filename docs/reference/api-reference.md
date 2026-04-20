# TARS API 参考文档

> **版本**: v1.0
> **适用版本**: TARS MVP (Phase 1)
> **最后更新**: 2026-03-13

---

## 1. 认证方式

### 1.1 公共端点 (无需认证)

以下端点无需认证即可访问：

| 端点 | 说明 |
|------|------|
| `/healthz` | 健康检查 |
| `/readyz` | 就绪检查 |
| `/metrics` | Prometheus 指标 |
| `/api/v1/platform/discovery` | 平台发现接口 |
| `/api/v1/webhooks/*` | Webhook 回调 (通过 Secret 验证) |
| `/api/v1/channels/telegram/webhook` | Telegram Webhook |

### 1.2 Ops API (Bearer Token)

Ops API 端点需要 Bearer Token 认证：

```bash
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/summary
```

Token 通过环境变量配置：
- `TARS_OPS_API_TOKEN` - Ops API 认证 Token

### 1.3 Webhook 认证

#### VMAlert Webhook

通过 Header `X-Tars-Secret` 验证：

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/vmalert \
  -H "Content-Type: application/json" \
  -H "X-Tars-Secret: ${VMALERT_WEBHOOK_SECRET}" \
  -d '{...}'
```

#### Telegram Webhook

通过 Telegram 的 `secret_token` 验证。

---

## 2. 公共 API

### 2.1 健康检查

#### GET /healthz

健康检查端点。

**请求示例**:
```bash
curl http://localhost:8080/healthz
```

**响应示例**:
```json
{
  "status": "ok"
}
```

#### GET /readyz

就绪检查端点。

**请求示例**:
```bash
curl http://localhost:8080/readyz
```

**响应示例**:
```json
{
  "status": "ok",
  "degraded": false
}
```

### 2.2 平台发现

#### GET /api/v1/platform/discovery

获取平台支持的集成模式和连接器类型。

**请求示例**:
```bash
curl http://localhost:8080/api/v1/platform/discovery
```

**响应示例**:
```json
{
  "product_name": "TARS",
  "api_base_path": "/api/v1",
  "api_version": "1.0",
  "manifest_version": "1.0",
  "marketplace_package_version": "1.0.0",
  "integration_modes": ["webhook", "ops_api", "connector_manifest"],
  "connector_kinds": ["metrics", "execution", "observability"],
  "registered_connectors_count": 3,
  "supported_provider_protocols": ["openai_compatible", "openrouter", "lmstudio"],
  "supported_provider_vendors": ["openai", "openrouter", "lmstudio"],
  "import_export_formats": ["yaml", "json"],
  "docs": [
    "https://tars.example.com/docs"
  ]
}
```

### 2.3 Webhooks

#### POST /api/v1/webhooks/vmalert

接收 VMAlert 告警 webhook。

**请求头**:
- `Content-Type: application/json`
- `X-Tars-Secret: <webhook_secret>`

**请求体**:
```json
{
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighCPUUsage",
        "severity": "warning",
        "instance": "prod-web-01"
      },
      "annotations": {
        "summary": "High CPU usage detected",
        "description": "CPU usage is above 80%"
      },
      "startsAt": "2026-03-13T10:00:00Z"
    }
  ]
}
```

**响应示例**:
```json
{
  "accepted": true,
  "duplicated": false,
  "event_count": 1,
  "session_ids": ["550e8400-e29b-41d4-a716-446655440000"]
}
```

#### POST /api/v1/webhooks/vmalert/api/v2/alerts

兼容 Alertmanager API v2 格式。

**请求参数**: 同 `/api/v1/webhooks/vmalert`

#### POST /api/v1/channels/telegram/webhook

接收 Telegram Bot webhook 回调。

**请求头**:
- `X-Telegram-Bot-Api-Secret-Token: <secret>`

**请求体**: Telegram Update 对象

**响应示例**:
```json
{
  "accepted": true
}
```

---

## 3. Ops API

### 3.1 Summary/Status

#### GET /api/v1/summary

获取系统概览统计。

**认证**: Bearer Token

**请求示例**:
```bash
curl -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8081/api/v1/summary
```

**响应示例**:
```json
{
  "active_sessions": 5,
  "pending_approvals": 3,
  "executions_total": 120,
  "executions_completed": 115,
  "execution_success_rate": 95,
  "blocked_outbox": 0,
  "failed_outbox": 2,
  "visible_outbox": 10,
  "healthy_connectors": 4,
  "degraded_connectors": 1,
  "configured_secrets": 8,
  "missing_secrets": 0,
  "provider_failures": 0,
  "active_alerts": 3
}
```

#### GET /api/v1/bootstrap/status

匿名获取平台当前所处的引导模式。

**认证**: 无需认证

**响应字段说明**:

| 字段 | 说明 |
|------|------|
| `initialized` | 平台是否已经完成首次安装 |
| `mode` | `wizard` 或 `runtime`，供前端做 `/setup` / `/login` / `/runtime-checks` 分流 |
| `next_step` | 若仍未初始化，返回当前 setup wizard 的下一步 |

#### GET /api/v1/setup/status

获取运行体检所需的系统设置和组件状态。

**认证**: Bearer Token（未初始化时允许首次安装向导匿名读取）

**响应字段说明**:

| 字段 | 说明 |
|------|------|
| `rollout_mode` | 当前部署模式 |
| `features` | 功能开关状态 |
| `telegram` | Telegram 配置和状态 |
| `model` | 主模型配置和状态 |
| `assist_model` | 辅助模型配置和状态 |
| `victoriametrics` | VM 配置和状态 |
| `ssh` | SSH 配置和状态 |
| `providers` | Provider 配置状态 |
| `connectors` | Connector 配置状态 |
| `authorization` | 授权策略配置状态 |
| `approval` | 审批路由配置状态 |
| `reasoning` | Reasoning Prompt 配置状态 |
| `desensitization` | 脱敏配置状态 |
| `latest_smoke` | 最新 Smoke 测试状态 |

### 3.2 Sessions 管理

#### GET /api/v1/sessions

获取会话列表。

**认证**: Bearer Token

**查询参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | int | 页码，默认 1 |
| `limit` | int | 每页数量，默认 20 |
| `q` | string | 搜索关键词 |
| `status` | string | 按状态筛选 |
| `sort_by` | string | 排序字段 |
| `sort_order` | string | 排序方向 (asc/desc) |

**请求示例**:
```bash
curl -H "Authorization: Bearer ${TOKEN}" \
  "http://localhost:8081/api/v1/sessions?page=1&limit=20&status=open"
```

**响应示例**:
```json
{
  "items": [
    {
      "session_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "open",
      "diagnosis_summary": "CPU usage is high on prod-web-01",
      "alert": {
        "alertname": "HighCPUUsage",
        "severity": "warning",
        "instance": "prod-web-01"
      },
      "executions": [],
      "timeline": [
        {
          "event": "session_created",
          "message": "Session created from alert",
          "created_at": "2026-03-13T10:00:00Z"
        }
      ]
    }
  ],
  "page": 1,
  "limit": 20,
  "total": 100,
  "has_next": true
}
```

#### GET /api/v1/sessions/{id}

获取单个会话详情。

**认证**: Bearer Token

**响应示例**:
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending_approval",
  "diagnosis_summary": "CPU usage is high on prod-web-01",
  "alert": {
    "alertname": "HighCPUUsage",
    "severity": "warning",
    "instance": "prod-web-01"
  },
  "verification": {
    "status": "pending",
    "summary": "Waiting for execution"
  },
  "executions": [
    {
      "execution_id": "660e8400-e29b-41d4-a716-446655440001",
      "status": "pending",
      "risk_level": "info",
      "command": "top -bn1 | head -20",
      "target_host": "prod-web-01",
      "requested_by": "tars",
      "created_at": "2026-03-13T10:01:00Z"
    }
  ],
  "timeline": [...]
}
```

#### GET /api/v1/sessions/{id}/trace

获取会话审计追踪。

**认证**: Bearer Token

**响应字段**:

| 字段 | 说明 |
|------|------|
| `session_id` | 会话 ID |
| `audit_entries` | 审计记录列表 |
| `knowledge` | 关联的知识记录 |

#### POST /api/v1/sessions/bulk/export

批量导出生成的会话。

**认证**: Bearer Token

**请求体**:
```json
{
  "ids": ["550e8400-e29b-41d4-a716-446655440000"],
  "operator_reason": "Backup before cleanup"
}
```

**响应示例**:
```json
{
  "resource_type": "sessions",
  "exported_at": "2026-03-13T12:00:00Z",
  "operator_reason": "Backup before cleanup",
  "total_requested": 1,
  "exported_count": 1,
  "failed_count": 0,
  "items": [...],
  "failures": []
}
```

### 3.3 Executions 管理

#### GET /api/v1/executions

获取执行请求列表。

**认证**: Bearer Token

**查询参数**: 同 Sessions API

**响应示例**:
```json
{
  "items": [
    {
      "execution_id": "660e8400-e29b-41d4-a716-446655440001",
      "status": "completed",
      "risk_level": "info",
      "command": "top -bn1 | head -20",
      "target_host": "prod-web-01",
      "requested_by": "tars",
      "exit_code": 0,
      "output_bytes": 1024,
      "output_truncated": false,
      "created_at": "2026-03-13T10:01:00Z",
      "approved_at": "2026-03-13T10:02:00Z",
      "completed_at": "2026-03-13T10:02:05Z"
    }
  ],
  "page": 1,
  "limit": 20,
  "total": 50,
  "has_next": true
}
```

#### GET /api/v1/executions/{id}

获取执行请求详情。

**认证**: Bearer Token

#### GET /api/v1/executions/{id}/output

获取执行输出内容。

**认证**: Bearer Token

**响应示例**:
```json
{
  "execution_id": "660e8400-e29b-41d4-a716-446655440001",
  "chunks": [
    {
      "seq": 1,
      "stream_type": "stdout",
      "content": "top - 10:00:00 up 5 days...",
      "byte_size": 1024,
      "created_at": "2026-03-13T10:02:05Z"
    }
  ]
}
```

#### POST /api/v1/executions/bulk/export

批量导出执行记录。

**认证**: Bearer Token

### 3.4 Audit Logs

#### GET /api/v1/audit

获取审计日志列表。

**认证**: Bearer Token

**查询参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | int | 页码 |
| `limit` | int | 每页数量 |
| `resource_type` | string | 资源类型筛选 |
| `resource_id` | string | 资源 ID 筛选 |
| `action` | string | 操作类型筛选 |
| `actor` | string | 操作人筛选 |

**响应示例**:
```json
{
  "items": [
    {
      "id": "42",
      "resource_type": "execution_request",
      "resource_id": "660e8400-e29b-41d4-a716-446655440001",
      "action": "approved",
      "actor": "u_alice",
      "metadata": {
        "command": "top -bn1 | head -20"
      },
      "created_at": "2026-03-13T10:02:00Z"
    }
  ],
  "page": 1,
  "limit": 20,
  "total": 200,
  "has_next": true
}
```

#### POST /api/v1/audit/bulk/export

批量导出审计记录。

**认证**: Bearer Token

**请求体**:
```json
{
  "ids": ["42", "43"],
  "operator_reason": "Export audit evidence"
}
```

**响应示例**:
```json
{
  "resource_type": "audit_log",
  "exported_at": "2026-03-19T12:00:00Z",
  "operator_reason": "Export audit evidence",
  "total_requested": 2,
  "exported_count": 1,
  "failed_count": 1,
  "items": [
    {
      "id": "42",
      "resource_type": "execution_request",
      "resource_id": "660e8400-e29b-41d4-a716-446655440001",
      "action": "approved",
      "actor": "u_alice",
      "metadata": {},
      "created_at": "2026-03-13T10:02:00Z"
    }
  ],
  "failures": [
    {
      "id": "43",
      "success": false,
      "code": "not_found",
      "message": "resource not found"
    }
  ]
}
```

### 3.5 Knowledge Base

#### GET /api/v1/knowledge

获取知识记录列表。

**认证**: Bearer Token

**响应示例**:
```json
{
  "items": [
    {
      "document_id": "770e8400-e29b-41d4-a716-446655440002",
      "session_id": "550e8400-e29b-41d4-a716-446655440000",
      "title": "High CPU Usage Resolution",
      "summary": "Resolved by restarting nginx service",
      "updated_at": "2026-03-13T11:00:00Z"
    }
  ],
  "page": 1,
  "limit": 20,
  "total": 50,
  "has_next": true
}
```

#### POST /api/v1/knowledge/bulk/export

批量导出知识记录。

**认证**: Bearer Token

**请求体**:
```json
{
  "ids": ["770e8400-e29b-41d4-a716-446655440002"],
  "operator_reason": "Export knowledge evidence"
}
```

**响应示例**:
```json
{
  "resource_type": "knowledge_record",
  "exported_at": "2026-03-19T12:00:00Z",
  "operator_reason": "Export knowledge evidence",
  "total_requested": 1,
  "exported_count": 1,
  "failed_count": 0,
  "items": [
    {
      "document_id": "770e8400-e29b-41d4-a716-446655440002",
      "session_id": "550e8400-e29b-41d4-a716-446655440000",
      "title": "High CPU Usage Resolution",
      "summary": "Resolved by restarting nginx service",
      "updated_at": "2026-03-13T11:00:00Z"
    }
  ],
  "failures": []
}
```

#### POST /api/v1/reindex/documents

重新索引知识文档。

**认证**: Bearer Token

**请求体**:
```json
{
  "document_ids": ["doc-1", "doc-2"]
}
```

**响应示例**:
```json
{
  "accepted": true
}
```

### 3.6 Outbox 管理

#### GET /api/v1/outbox

获取 Outbox 事件列表。

**认证**: Bearer Token

**查询参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| `status` | string | 状态筛选 (pending/processing/done/failed/blocked) |
| `topic` | string | Topic 筛选 |

**响应示例**:
```json
{
  "items": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "topic": "telegram.notification",
      "status": "pending",
      "aggregate_id": "550e8400-e29b-41d4-a716-446655440000",
      "retry_count": 0,
      "created_at": "2026-03-13T10:00:00Z"
    }
  ],
  "page": 1,
  "limit": 20,
  "total": 30,
  "has_next": true
}
```

#### POST /api/v1/outbox/bulk/replay

批量重放 Outbox 事件。

**认证**: Bearer Token

**请求体**:
```json
{
  "ids": ["880e8400-e29b-41d4-a716-446655440003"]
}
```

**响应示例**:
```json
{
  "operation": "replay",
  "resource_type": "outbox",
  "total": 1,
  "succeeded": 1,
  "failed": 0,
  "results": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "success": true,
      "code": "ok",
      "message": "Requeued for processing"
    }
  ]
}
```

#### POST /api/v1/outbox/bulk/delete

批量删除 Outbox 事件。

**认证**: Bearer Token

### 3.7 Config 管理

#### GET /api/v1/config/authorization

获取授权策略配置。

**认证**: Bearer Token

**响应示例**:
```json
{
  "configured": true,
  "loaded": true,
  "path": "/etc/tars/authorization_policy.yaml",
  "updated_at": "2026-03-13T09:00:00Z",
  "content": "# YAML content...",
  "config": {
    "whitelist_action": "direct_execute",
    "blacklist_action": "suggest_only",
    "unmatched_action": "require_approval",
    "normalize_whitespace": true,
    "hard_deny_ssh_command": ["rm -rf /", "mkfs*"],
    "whitelist": ["hostname", "uptime"],
    "blacklist": ["reboot*", "shutdown*"],
    "overrides": []
  }
}
```

#### GET /api/v1/config/approval-routing

获取审批路由配置。

**认证**: Bearer Token

#### GET /api/v1/config/reasoning-prompts

获取 Reasoning Prompt 配置。

**认证**: Bearer Token

#### GET /api/v1/config/desensitization

获取脱敏配置。

**认证**: Bearer Token

#### GET /api/v1/config/providers

获取 Provider 配置。

**认证**: Bearer Token

#### GET /api/v1/config/providers/models

获取 Provider 可用模型列表。

**认证**: Bearer Token

**请求参数**:
```json
{
  "provider_id": "primary-openrouter"
}
```

#### POST /api/v1/config/providers/check

检查 Provider 可用性。

**认证**: Bearer Token

**请求参数**:
```json
{
  "provider_id": "primary-openrouter",
  "model": "openai/gpt-4.1-mini"
}
```

**响应示例**:
```json
{
  "provider_id": "primary-openrouter",
  "available": true,
  "detail": "Model is available and responding"
}
```

#### GET /api/v1/config/secrets

获取 Secrets 清单。

**认证**: Bearer Token

**响应示例**:
```json
{
  "configured": true,
  "loaded": true,
  "path": "/etc/tars/secrets.yaml",
  "updated_at": "2026-03-13T09:00:00Z",
  "items": [
    {
      "ref": "telegram.bot_token",
      "owner_type": "config",
      "owner_id": "telegram",
      "key": "bot_token",
      "set": true,
      "updated_at": "2026-03-13T09:00:00Z",
      "source": "env"
    }
  ]
}
```

### 3.8 Runtime Checks 测试

#### GET /api/v1/setup/status

获取详细设置状态。

**认证**: Bearer Token

#### POST /api/v1/smoke/alerts

发送测试告警。

**认证**: Bearer Token

**请求体**:
```json
{
  "alert_name": "TestAlert",
  "severity": "warning",
  "target_host": "test-host",
  "description": "Smoke test alert",
  "service": "test-service"
}
```

**响应示例**:
```json
{
  "accepted": true,
  "session_id": "990e8400-e29b-41d4-a716-446655440004",
  "status": "open",
  "duplicated": false,
  "tg_target": "@test_group"
}
```

### 3.9 Dashboard Health

#### GET /api/v1/dashboard/health

获取 Dashboard 健康状态。

**认证**: Bearer Token

**响应字段**:

| 字段 | 说明 |
|------|------|
| `summary` | 健康摘要统计 |
| `resources` | 运行时资源信息 |
| `connectors` | 连接器健康状态 |
| `providers` | Provider 健康状态 |
| `secrets` | Secrets 清单 |
| `alerts` | 活跃告警列表 |

### 3.10 Connectors

#### GET /api/v1/connectors

获取连接器列表。

**认证**: 无需认证

#### GET /api/v1/connectors/{connector_id}

获取单个连接器公开详情。

**认证**: 无需认证

#### GET /api/v1/connectors/{connector_id}/export

导出连接器公开 manifest，支持 `yaml` / `json`。

**认证**: 无需认证

#### GET /api/v1/connectors/templates

获取连接器模板列表。

**认证**: Bearer Token

#### POST /api/v1/connectors/{connector_id}/enable

启用连接器运行时。

**认证**: Bearer Token

#### POST /api/v1/connectors/{connector_id}/disable

停用连接器运行时。

**认证**: Bearer Token

#### POST /api/v1/connectors/{connector_id}/metrics/query

对 metrics 类连接器执行 runtime 查询。

**认证**: Bearer Token

#### POST /api/v1/connectors/{connector_id}/health

执行连接器健康检查并返回 lifecycle/health 快照。

**认证**: Bearer Token

#### POST /api/v1/connectors/{connector_id}/execution/execute

对 execution 类连接器执行命令。当前 JumpServer 官方链路会返回 `output_ref / output_bytes / output_truncated / output_preview`，便于 UI 展示和后续输出追踪。

**认证**: Bearer Token

#### POST /api/v1/connectors/{connector_id}/upgrade

升级连接器 manifest，并刷新 `available_version / history / revisions` 生命周期字段。

**认证**: Bearer Token

#### POST /api/v1/connectors/{connector_id}/rollback

按 revision 快照回滚连接器版本。

**认证**: Bearer Token

#### POST /api/v1/config/connectors/import

导入连接器配置。

**认证**: Bearer Token

---

## 4. 错误码说明

### 4.1 HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 成功 |
| 201 | 已创建 |
| 204 | 无内容 (删除成功) |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 409 | 资源冲突 |
| 422 | 请求无法处理 |
| 429 | 请求过于频繁 |
| 500 | 服务器内部错误 |
| 503 | 服务不可用 |

### 4.2 业务错误码

| 错误码 | 说明 |
|--------|------|
| `E001` | 会话不存在 |
| `E002` | 执行请求不存在 |
| `E003` | 审批超时 |
| `E004` | 命令未授权 |
| `E005` | SSH 连接失败 |
| `E006` | 模型调用失败 |
| `E007` | Telegram 发送失败 |
| `E008` | 配置加载失败 |
| `E009` | 数据库操作失败 |
| `E010` | 重复请求 |

### 4.3 错误响应格式

```json
{
  "error": {
    "code": "E001",
    "message": "Session not found",
    "details": {
      "session_id": "xxx"
    }
  }
}
```

---

## 5. 请求/响应示例

### 5.1 完整告警处理流程

#### 步骤 1: 接收告警

```bash
# VMAlert 发送告警
curl -X POST http://localhost:8080/api/v1/webhooks/vmalert \
  -H "Content-Type: application/json" \
  -H "X-Tars-Secret: secret123" \
  -d '{
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "HighCPUUsage",
        "severity": "warning",
        "instance": "prod-web-01"
      },
      "annotations": {
        "summary": "CPU > 80%"
      }
    }]
  }'
```

**响应**:
```json
{
  "accepted": true,
  "session_ids": ["550e8400-e29b-41d4-a716-446655440000"]
}
```

#### 步骤 2: 查询会话状态

```bash
curl -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8081/api/v1/sessions/550e8400-e29b-41d4-a716-446655440000
```

**响应**:
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "analyzing",
  "diagnosis_summary": "Analyzing high CPU usage...",
  "alert": {...},
  "executions": [],
  "timeline": [...]
}
```

#### 步骤 3: 等待 AI 诊断完成

状态变为 `pending_approval`，包含执行建议。

#### 步骤 4: 在 Telegram 审批

用户收到 Telegram 消息，点击"批准执行"。

#### 步骤 5: 查询执行状态

```bash
curl -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8081/api/v1/executions/660e8400-e29b-41d4-a716-446655440001
```

**响应**:
```json
{
  "execution_id": "660e8400-e29b-41d4-a716-446655440001",
  "status": "executing",
  "risk_level": "info",
  "command": "top -bn1 | head -20",
  "target_host": "prod-web-01"
}
```

#### 步骤 6: 获取执行输出

```bash
curl -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8081/api/v1/executions/660e8400-e29b-41d4-a716-446655440001/output
```

**响应**:
```json
{
  "execution_id": "660e8400-e29b-41d4-a716-446655440001",
  "chunks": [
    {
      "seq": 1,
      "stream_type": "stdout",
      "content": "...",
      "byte_size": 1024
    }
  ]
}
```

#### 步骤 7: 查看审计日志

```bash
curl -H "Authorization: Bearer ${TOKEN}" \
  "http://localhost:8081/api/v1/audit?resource_id=550e8400-e29b-41d4-a716-446655440000"
```

---

## 6. 分页和排序

### 6.1 分页参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `page` | int | 1 | 页码 |
| `limit` | int | 20 | 每页数量 (最大 100) |

### 6.2 分页响应

```json
{
  "items": [...],
  "page": 1,
  "limit": 20,
  "total": 100,
  "has_next": true
}
```

### 6.3 排序

| 参数 | 说明 |
|------|------|
| `sort_by` | 排序字段 (created_at/updated_at/status) |
| `sort_order` | 排序方向 (asc/desc) |

---

## 7. API 版本控制

TARS API 使用 URI 版本控制：

```
/api/v1/...
/api/v2/... (未来版本)
```

---

## 8. 参考链接

- [用户手册](../guides/user-guide.md)
- [部署手册](../guides/deployment-guide.md)
- [管理员手册](../guides/admin-guide.md)
- [产品需求文档](../../project/tars_prd.md)
- [技术设计文档](../../project/tars_technical_design.md)

---

*本文档适用于 TARS MVP 版本，API 可能会在未来版本中调整。*
