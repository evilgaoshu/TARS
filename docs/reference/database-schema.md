# TARS 数据库 Schema 文档

> **版本**: v1.0
> **适用版本**: TARS MVP (Phase 1)
> **最后更新**: 2026-03-13

---

## 目录

1. [概述](#1-概述)
2. [ER 关系图](#2-er-关系图)
3. [表结构详解](#3-表结构详解)
4. [枚举类型](#4-枚举类型)
5. [索引设计](#5-索引设计)
6. [数据流关系](#6-数据流关系)
7. [分区策略](#7-分区策略)
8. [维护操作](#8-维护操作)

---

## 1. 概述

### 1.1 数据库选型

TARS 使用 **PostgreSQL** 作为主存储，存储：
- 业务数据（告警、会话、执行请求）
- 审计日志
- 知识库元数据

使用 **SQLite**（带 sqlite-vec 扩展）作为向量存储，存储：
- 文档向量索引
- 知识记录向量

### 1.2 Schema 设计原则

- **单租户设计**: MVP 阶段使用 `tenant_id='default'`，预留多租户扩展
- **软删除**: 使用状态字段代替物理删除
- **审计追踪**: 关键操作记录审计日志
- **乐观锁**: 使用 `version` 字段实现乐观锁

### 1.3 数据库版本

| 组件 | 版本 | 说明 |
|------|------|------|
| PostgreSQL | 14+ | 主存储 |
| SQLite | 3+ | 向量存储 |
| sqlite-vec | 0.1+ | 向量扩展 |

---

## 2. ER 关系图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Entity Relationship                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌──────────────────┐         ┌──────────────────┐         ┌──────────────────┐
│  idempotency_    │         │   alert_events   │         │ alert_sessions   │
│      keys        │         ├──────────────────┤         ├──────────────────┤
├──────────────────┤         │ id (PK)          │         │ id (PK)          │
│ id (PK)          │         │ tenant_id        │         │ tenant_id        │
│ scope            │         │ external_alert_id│         │ alert_event_id   │──┐
│ idempotency_key  │         │ source           │         │ status           │  │
│ request_hash     │         │ severity         │         │ service_name     │  │
│ resource_type    │         │ labels (JSONB)   │         │ target_host      │  │
│ resource_id      │         │ annotations      │         │ diagnosis_summary│  │
│ status           │         │ raw_payload      │         │ verification     │  │
│ response_payload │         │ fingerprint      │         │ desense_map      │  │
│ timestamps       │         │ received_at      │         │ version          │  │
└──────────────────┘         └──────────────────┘         │ timestamps       │  │
                                                          └──────────────────┘  │
                                                                     │          │
                                                                     │ 1:N      │
                                                                     ▼          │
┌──────────────────┐         ┌──────────────────┐         ┌──────────────────┐  │
│ document_chunks  │         │    documents     │         │ session_events   │  │
├──────────────────┤         ├──────────────────┤         ├──────────────────┤  │
│ id (PK)          │         │ id (PK)          │         │ id (PK)          │  │
│ document_id (FK) │────────▶│ tenant_id        │         │ session_id (FK)  │──┘
│ tenant_id        │    1:N  │ source_type      │         │ event_type       │
│ chunk_index      │         │ source_ref       │         │ payload (JSONB)  │
│ content          │         │ title            │         │ created_at       │
│ token_count      │         │ content_hash     │         └──────────────────┘
│ citation (JSONB) │         │ status           │
│ created_at       │         │ timestamps       │
└──────────────────┘         └──────────────────┘
                                      │
                                      │ 1:1
                                      ▼
                          ┌──────────────────┐
                          │knowledge_records │
                          ├──────────────────┤
                          │ id (PK)          │
                          │ tenant_id        │
                          │ session_id (FK)  │
                          │ document_id (FK) │
                          │ summary          │
                          │ content (JSONB)  │
                          │ status           │
                          │ created_at       │
                          └──────────────────┘

┌──────────────────┐         ┌──────────────────┐         ┌──────────────────┐
│ execution_       │         │ execution_       │         │ execution_output │
│    approvals     │         │   requests       │         │     _chunks      │
├──────────────────┤         ├──────────────────┤         ├──────────────────┤
│ id (PK)          │         │ id (PK)          │         │ id (PK)          │
│ execution_req_id │────────▶│ tenant_id        │         │ execution_req_id │
│ action           │    N:1  │ session_id (FK)  │────────▶│ seq              │
│ actor_id         │         │ target_host      │    1:N  │ stream_type      │
│ actor_role       │         │ command          │         │ content          │
│ original_cmd     │         │ command_source   │         │ byte_size        │
│ final_cmd        │         │ risk_level       │         │ retention_until  │
│ comment          │         │ requested_by     │         │ created_at       │
│ created_at       │         │ approved_by      │         └──────────────────┘
└──────────────────┘         │ approval_group   │
                             │ status           │
                             │ timeout_seconds  │
                             │ output_ref       │
                             │ exit_code        │
                             │ output_bytes     │
                             │ output_truncated │
                             │ version          │
                             │ timestamps       │
                             └──────────────────┘

┌──────────────────┐         ┌──────────────────┐
│   outbox_events  │         │    audit_logs    │
├──────────────────┤         ├──────────────────┤
│ id (PK)          │         │ id (PK)          │
│ topic            │         │ tenant_id        │
│ aggregate_id     │         │ trace_id         │
│ payload (JSONB)  │         │ actor_id         │
│ status           │         │ resource_type    │
│ available_at     │         │ resource_id      │
│ retry_count      │         │ action           │
│ last_error       │         │ payload (JSONB)  │
│ blocked_reason   │         │ created_at       │
│ created_at       │         └──────────────────┘
└──────────────────┘
```

---

## 3. 表结构详解

### 3.1 idempotency_keys

幂等键表，用于防止重复处理。

```sql
CREATE TABLE idempotency_keys (
  id UUID PRIMARY KEY,
  scope TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  resource_type TEXT,
  resource_id TEXT,
  status TEXT NOT NULL,
  response_payload JSONB,
  first_seen_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT uniq_idempotency_scope_key UNIQUE (scope, idempotency_key)
);

CREATE INDEX idx_idempotency_expires_at ON idempotency_keys (expires_at);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `scope` | TEXT | 作用域（如 webhook、api） |
| `idempotency_key` | TEXT | 幂等键（客户端提供） |
| `request_hash` | TEXT | 请求内容哈希 |
| `resource_type` | TEXT | 资源类型 |
| `resource_id` | TEXT | 资源 ID |
| `status` | TEXT | 状态（pending/completed） |
| `response_payload` | JSONB | 响应内容缓存 |
| `first_seen_at` | TIMESTAMPTZ | 首次收到时间 |
| `last_seen_at` | TIMESTAMPTZ | 最后收到时间 |
| `expires_at` | TIMESTAMPTZ | 过期时间 |

### 3.2 alert_events

告警事件表，存储接收到的原始告警。

```sql
CREATE TABLE alert_events (
  id UUID PRIMARY KEY,
  tenant_id TEXT NOT NULL DEFAULT 'default',
  external_alert_id TEXT,
  source TEXT NOT NULL,
  severity TEXT NOT NULL,
  labels JSONB NOT NULL,
  annotations JSONB NOT NULL,
  raw_payload JSONB NOT NULL,
  fingerprint TEXT NOT NULL,
  received_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_alert_events_fingerprint ON alert_events (fingerprint);
CREATE INDEX idx_alert_events_received_at ON alert_events (received_at);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `tenant_id` | TEXT | 租户 ID（单租户为 'default'） |
| `external_alert_id` | TEXT | 外部系统告警 ID |
| `source` | TEXT | 告警来源（vmalert） |
| `severity` | TEXT | 告警级别（critical/warning/info） |
| `labels` | JSONB | 告警标签 |
| `annotations` | JSONB | 告警注释 |
| `raw_payload` | JSONB | 原始请求体 |
| `fingerprint` | TEXT | 告警指纹（用于去重） |
| `received_at` | TIMESTAMPTZ | 接收时间 |

**Labels 示例**:
```json
{
  "alertname": "HighCPUUsage",
  "instance": "prod-web-01",
  "service": "web",
  "severity": "warning"
}
```

### 3.3 alert_sessions

告警会话表，核心业务表。

```sql
CREATE TABLE alert_sessions (
  id UUID PRIMARY KEY,
  tenant_id TEXT NOT NULL DEFAULT 'default',
  alert_event_id UUID NOT NULL REFERENCES alert_events (id),
  status session_status NOT NULL,
  service_name TEXT,
  target_host TEXT,
  diagnosis_summary TEXT,
  verification_result JSONB,
  desense_map JSONB,
  version INTEGER NOT NULL DEFAULT 1,
  opened_at TIMESTAMPTZ NOT NULL,
  resolved_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_alert_sessions_status ON alert_sessions (status);
CREATE INDEX idx_alert_sessions_target_host ON alert_sessions (target_host);
CREATE INDEX idx_alert_sessions_updated_at ON alert_sessions (updated_at);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键（会话 ID） |
| `tenant_id` | TEXT | 租户 ID |
| `alert_event_id` | UUID | 关联告警事件 ID |
| `status` | session_status | 会话状态 |
| `service_name` | TEXT | 服务名称 |
| `target_host` | TEXT | 目标主机 |
| `diagnosis_summary` | TEXT | AI 诊断摘要 |
| `verification_result` | JSONB | 验证结果 |
| `desense_map` | JSONB | 脱敏映射（用于回水） |
| `version` | INTEGER | 乐观锁版本号 |
| `opened_at` | TIMESTAMPTZ | 会话创建时间 |
| `resolved_at` | TIMESTAMPTZ | 解决时间 |
| `updated_at` | TIMESTAMPTZ | 最后更新时间 |

**verification_result 示例**:
```json
{
  "status": "success",
  "summary": "CPU usage returned to normal",
  "metric_value": 45.2,
  "checked_at": "2026-03-13T10:05:00Z"
}
```

### 3.4 session_events

会话事件表，记录会话生命周期事件。

```sql
CREATE TABLE session_events (
  id UUID PRIMARY KEY,
  session_id UUID NOT NULL REFERENCES alert_sessions (id),
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `session_id` | UUID | 关联会话 ID |
| `event_type` | TEXT | 事件类型 |
| `payload` | JSONB | 事件负载 |
| `created_at` | TIMESTAMPTZ | 创建时间 |

**Event Types**:
- `session_created` - 会话创建
- `diagnosis_started` - 开始诊断
- `diagnosis_completed` - 诊断完成
- `execution_requested` - 请求执行
- `approval_requested` - 请求审批
- `execution_started` - 开始执行
- `execution_completed` - 执行完成
- `session_resolved` - 会话解决

### 3.5 execution_requests

执行请求表，存储待执行/已执行的命令。

```sql
CREATE TABLE execution_requests (
  id UUID PRIMARY KEY,
  tenant_id TEXT NOT NULL DEFAULT 'default',
  session_id UUID NOT NULL REFERENCES alert_sessions (id),
  target_host TEXT NOT NULL,
  command TEXT NOT NULL,
  command_source TEXT NOT NULL,
  risk_level risk_level NOT NULL,
  requested_by TEXT NOT NULL,
  approved_by TEXT,
  approval_group TEXT,
  status execution_status NOT NULL,
  timeout_seconds INTEGER NOT NULL DEFAULT 300,
  output_ref TEXT,
  exit_code INTEGER NOT NULL DEFAULT 0,
  output_bytes BIGINT NOT NULL DEFAULT 0,
  output_truncated BOOLEAN NOT NULL DEFAULT FALSE,
  version INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL,
  approved_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
);

CREATE INDEX idx_execution_requests_session_id ON execution_requests (session_id);
CREATE INDEX idx_execution_requests_status ON execution_requests (status);
CREATE INDEX idx_execution_requests_created_at ON execution_requests (created_at);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键（执行 ID） |
| `tenant_id` | TEXT | 租户 ID |
| `session_id` | UUID | 关联会话 ID |
| `target_host` | TEXT | 目标主机 |
| `command` | TEXT | 执行命令 |
| `command_source` | TEXT | 命令来源（ai_suggest/manual/skill） |
| `risk_level` | risk_level | 风险级别 |
| `requested_by` | TEXT | 请求人 |
| `approved_by` | TEXT | 审批人 |
| `approval_group` | TEXT | 审批组 |
| `status` | execution_status | 执行状态 |
| `timeout_seconds` | INTEGER | 超时时间（秒） |
| `output_ref` | TEXT | 输出文件引用 |
| `exit_code` | INTEGER | 退出码 |
| `output_bytes` | BIGINT | 输出字节数 |
| `output_truncated` | BOOLEAN | 是否被截断 |
| `version` | INTEGER | 乐观锁版本号 |
| `created_at` | TIMESTAMPTZ | 创建时间 |
| `approved_at` | TIMESTAMPTZ | 审批时间 |
| `completed_at` | TIMESTAMPTZ | 完成时间 |

### 3.6 execution_approvals

执行审批表，记录审批操作历史。

```sql
CREATE TABLE execution_approvals (
  id UUID PRIMARY KEY,
  execution_request_id UUID NOT NULL REFERENCES execution_requests (id),
  action TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  actor_role TEXT,
  original_command TEXT,
  final_command TEXT,
  comment TEXT,
  created_at TIMESTAMPTZ NOT NULL
);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `execution_request_id` | UUID | 关联执行请求 ID |
| `action` | TEXT | 操作（approve/reject/modify） |
| `actor_id` | TEXT | 操作人 ID |
| `actor_role` | TEXT | 操作人角色 |
| `original_command` | TEXT | 原始命令 |
| `final_command` | TEXT | 修改后的命令 |
| `comment` | TEXT | 审批备注 |
| `created_at` | TIMESTAMPTZ | 创建时间 |

### 3.7 execution_output_chunks

执行输出分块表，存储命令执行输出。

```sql
CREATE TABLE execution_output_chunks (
  id BIGSERIAL PRIMARY KEY,
  execution_request_id UUID NOT NULL REFERENCES execution_requests (id),
  seq INTEGER NOT NULL,
  stream_type TEXT NOT NULL,
  content TEXT NOT NULL,
  byte_size INTEGER NOT NULL,
  retention_until TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT uniq_execution_output_seq UNIQUE (execution_request_id, seq)
);

CREATE INDEX idx_execution_output_retention ON execution_output_chunks (retention_until);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGSERIAL | 主键 |
| `execution_request_id` | UUID | 关联执行请求 ID |
| `seq` | INTEGER | 序列号（分块顺序） |
| `stream_type` | TEXT | 流类型（stdout/stderr） |
| `content` | TEXT | 内容 |
| `byte_size` | INTEGER | 字节大小 |
| `retention_until` | TIMESTAMPTZ | 保留截止时间 |
| `created_at` | TIMESTAMPTZ | 创建时间 |

### 3.8 documents

文档表，存储知识库文档元数据。

```sql
CREATE TABLE documents (
  id UUID PRIMARY KEY,
  tenant_id TEXT NOT NULL DEFAULT 'default',
  source_type TEXT NOT NULL,
  source_ref TEXT NOT NULL,
  title TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT uniq_documents_source UNIQUE (tenant_id, source_type, source_ref)
);

CREATE INDEX idx_documents_status ON documents (status, updated_at);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `tenant_id` | TEXT | 租户 ID |
| `source_type` | TEXT | 来源类型（file/url） |
| `source_ref` | TEXT | 来源引用（文件路径/URL） |
| `title` | TEXT | 标题 |
| `content_hash` | TEXT | 内容哈希（用于检测变更） |
| `status` | TEXT | 状态（pending/active/archived） |
| `created_at` | TIMESTAMPTZ | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 更新时间 |

### 3.9 document_chunks

文档分块表，存储文档的分块内容。

```sql
CREATE TABLE document_chunks (
  id UUID PRIMARY KEY,
  document_id UUID NOT NULL REFERENCES documents (id),
  tenant_id TEXT NOT NULL DEFAULT 'default',
  chunk_index INTEGER NOT NULL,
  content TEXT NOT NULL,
  token_count INTEGER,
  citation JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT uniq_document_chunk_index UNIQUE (document_id, chunk_index)
);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `document_id` | UUID | 关联文档 ID |
| `tenant_id` | TEXT | 租户 ID |
| `chunk_index` | INTEGER | 分块索引 |
| `content` | TEXT | 分块内容 |
| `token_count` | INTEGER | Token 数 |
| `citation` | JSONB | 引用信息 |
| `created_at` | TIMESTAMPTZ | 创建时间 |

### 3.10 knowledge_records

知识记录表，存储会话沉淀的知识。

```sql
CREATE TABLE knowledge_records (
  id UUID PRIMARY KEY,
  tenant_id TEXT NOT NULL DEFAULT 'default',
  session_id UUID NOT NULL REFERENCES alert_sessions (id),
  document_id UUID NOT NULL REFERENCES documents (id),
  summary TEXT NOT NULL,
  content JSONB NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT uniq_knowledge_record_session UNIQUE (tenant_id, session_id)
);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `tenant_id` | TEXT | 租户 ID |
| `session_id` | UUID | 关联会话 ID |
| `document_id` | UUID | 关联文档 ID |
| `summary` | TEXT | 摘要 |
| `content` | JSONB | 内容 |
| `status` | TEXT | 状态 |
| `created_at` | TIMESTAMPTZ | 创建时间 |

### 3.11 audit_logs

审计日志表，记录所有操作。

```sql
CREATE TABLE audit_logs (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL DEFAULT 'default',
  trace_id TEXT,
  actor_id TEXT,
  resource_type TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  action TEXT NOT NULL,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGSERIAL | 主键 |
| `tenant_id` | TEXT | 租户 ID |
| `trace_id` | TEXT | 追踪 ID |
| `actor_id` | TEXT | 操作人 ID |
| `resource_type` | TEXT | 资源类型 |
| `resource_id` | TEXT | 资源 ID |
| `action` | TEXT | 操作 |
| `payload` | JSONB | 操作详情 |
| `created_at` | TIMESTAMPTZ | 创建时间 |

### 3.12 outbox_events

Outbox 事件表，用于可靠消息投递。

```sql
CREATE TABLE outbox_events (
  id UUID PRIMARY KEY,
  topic TEXT NOT NULL,
  aggregate_id TEXT NOT NULL,
  payload JSONB NOT NULL,
  status outbox_status NOT NULL DEFAULT 'pending',
  available_at TIMESTAMPTZ NOT NULL,
  retry_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  blocked_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_outbox_poll ON outbox_events (status, available_at);
CREATE INDEX idx_outbox_failed_blocked ON outbox_events (status, created_at);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `topic` | TEXT | 主题 |
| `aggregate_id` | TEXT | 聚合 ID |
| `payload` | JSONB | 消息内容 |
| `status` | outbox_status | 状态 |
| `available_at` | TIMESTAMPTZ | 可处理时间 |
| `retry_count` | INTEGER | 重试次数 |
| `last_error` | TEXT | 最后错误 |
| `blocked_reason` | TEXT | 阻塞原因 |
| `created_at` | TIMESTAMPTZ | 创建时间 |

---

## 4. 枚举类型

### 4.1 session_status

```sql
CREATE TYPE session_status AS ENUM (
  'open',              -- 刚创建
  'analyzing',         -- AI 诊断中
  'pending_approval',  -- 等待审批
  'executing',         -- 执行中
  'verifying',         -- 验证中
  'resolved',          -- 已解决
  'failed'             -- 失败
);
```

### 4.2 execution_status

```sql
CREATE TYPE execution_status AS ENUM (
  'pending',    -- 待审批
  'approved',   -- 已批准
  'executing',  -- 执行中
  'completed',  -- 已完成
  'failed',     -- 失败
  'timeout',    -- 超时
  'rejected'    -- 被拒绝
);
```

### 4.3 risk_level

```sql
CREATE TYPE risk_level AS ENUM (
  'info',      -- 信息
  'warning',   -- 警告
  'critical'   -- 危险
);
```

### 4.4 outbox_status

```sql
CREATE TYPE outbox_status AS ENUM (
  'pending',    -- 待处理
  'processing', -- 处理中
  'done',       -- 完成
  'failed',     -- 失败
  'blocked'     -- 阻塞
);
```

---

## 5. 索引设计

### 5.1 核心索引

| 表 | 索引 | 类型 | 用途 |
|----|------|------|------|
| alert_events | fingerprint | B-tree | 告警去重 |
| alert_events | received_at | B-tree | 时间查询 |
| alert_sessions | status | B-tree | 状态筛选 |
| alert_sessions | target_host | B-tree | 主机查询 |
| alert_sessions | updated_at | B-tree | 时间排序 |
| execution_requests | session_id | B-tree | 关联查询 |
| execution_requests | status | B-tree | 状态筛选 |
| execution_requests | created_at | B-tree | 时间排序 |
| execution_output_chunks | retention_until | B-tree | 清理查询 |
| documents | status + updated_at | B-tree | 状态+时间筛选 |
| idempotency_keys | expires_at | B-tree | 过期清理 |
| outbox_events | status + available_at | B-tree | 轮询查询 |
| outbox_events | status + created_at | B-tree | 失败查询 |

### 5.2 复合索引建议

```sql
-- 活跃会话查询优化
CREATE INDEX idx_sessions_active
ON alert_sessions (status, updated_at)
WHERE status IN ('open', 'analyzing', 'pending_approval', 'executing');

-- 待审批执行查询
CREATE INDEX idx_executions_pending
ON execution_requests (status, risk_level, created_at)
WHERE status = 'pending';

-- 审计日志时间范围查询
CREATE INDEX idx_audit_created_at
ON audit_logs (created_at DESC);
```

---

## 6. 数据流关系

### 6.1 告警处理流程

```
alert_events (INSERT)
    ↓
alert_sessions (INSERT) status='open'
    ↓
session_events (INSERT) event='session_created'
    ↓
alert_sessions (UPDATE) status='analyzing'
    ↓
session_events (INSERT) event='diagnosis_completed'
    ↓
execution_requests (INSERT) status='pending' (可选)
    ↓
execution_approvals (INSERT) (审批后)
    ↓
execution_requests (UPDATE) status='completed'
    ↓
execution_output_chunks (INSERT) (执行后)
    ↓
alert_sessions (UPDATE) status='resolved'
    ↓
knowledge_records (INSERT) (知识沉淀)
```

### 6.2 数据保留策略

| 数据类型 | 保留时间 | 清理方式 |
|----------|----------|----------|
| alert_events | 90 天 | 自动归档 |
| alert_sessions | 90 天 | 自动归档 |
| session_events | 90 天 | 级联删除 |
| execution_requests | 90 天 | 自动归档 |
| execution_output_chunks | 7 天 | GC 清理 |
| execution_approvals | 90 天 | 级联删除 |
| audit_logs | 1 年 | 分区归档 |
| outbox_events | 30 天 | 自动清理 |
| documents | 永久 | 手动管理 |
| knowledge_records | 永久 | 手动管理 |

---

## 7. 分区策略

### 7.1 建议分区表

#### audit_logs 分区（按时间）

```sql
-- 创建分区表
CREATE TABLE audit_logs (
  id BIGSERIAL,
  tenant_id TEXT NOT NULL DEFAULT 'default',
  trace_id TEXT,
  actor_id TEXT,
  resource_type TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  action TEXT NOT NULL,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- 创建月度分区
CREATE TABLE audit_logs_2026_03
PARTITION OF audit_logs
FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

-- 自动化分区脚本
-- scripts/create_partition.sh
```

### 7.2 分区管理

```bash
# 创建新分区
psql -d tars -c "
CREATE TABLE audit_logs_2026_04
PARTITION OF audit_logs
FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
"

# 归档旧分区
psql -d tars -c "
ALTER TABLE audit_logs DETACH PARTITION audit_logs_2026_01;
"
pg_dump -t audit_logs_2026_01 tars > audit_logs_2026_01.sql

# 删除旧分区
psql -d tars -c "DROP TABLE audit_logs_2026_01;"
```

---

## 8. 维护操作

### 8.1 常用查询

```sql
-- 查看活跃会话
SELECT id, status, target_host, updated_at
FROM alert_sessions
WHERE status IN ('open', 'analyzing', 'pending_approval', 'executing')
ORDER BY updated_at DESC;

-- 查看待审批执行
SELECT er.id, er.command, er.risk_level, er.target_host, er.created_at
FROM execution_requests er
WHERE er.status = 'pending'
ORDER BY er.created_at;

-- 查看会话时间线
SELECT event_type, payload, created_at
FROM session_events
WHERE session_id = 'xxx'
ORDER BY created_at;

-- 统计今日告警
SELECT COUNT(*) as count, severity
FROM alert_events
WHERE received_at >= CURRENT_DATE
GROUP BY severity;

-- 查看执行成功率
SELECT
  status,
  COUNT(*) as count,
  ROUND(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER(), 2) as percentage
FROM execution_requests
WHERE created_at >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY status;

-- 慢查询分析
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
```

### 8.2 数据清理

```sql
-- 清理过期输出（通过应用 GC，不推荐直接操作）
-- 应用会自动清理 retention_until < NOW() 的记录

-- 手动归档旧会话（保留 90 天）
INSERT INTO alert_sessions_archive
SELECT * FROM alert_sessions
WHERE resolved_at < CURRENT_DATE - INTERVAL '90 days';

DELETE FROM alert_sessions
WHERE resolved_at < CURRENT_DATE - INTERVAL '90 days';

-- 清理孤立事件
DELETE FROM session_events
WHERE session_id NOT IN (SELECT id FROM alert_sessions);
```

### 8.3 性能监控

```sql
-- 表大小统计
SELECT
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- 索引使用情况
SELECT
  schemaname,
  tablename,
  indexname,
  idx_scan,
  idx_tup_read,
  idx_tup_fetch
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC;

-- 连接数
SELECT count(*), state
FROM pg_stat_activity
GROUP BY state;
```

### 8.4 备份策略

```bash
# 全量备份
pg_dump -Fc tars > tars_backup_$(date +%Y%m%d).dump

# 特定表备份
pg_dump -t alert_sessions -t execution_requests tars > core_tables.dump

# 增量备份（基于时间戳）
pg_dump -t alert_sessions --where "updated_at > '2026-03-01'" tars > incremental.dump
```

---

## 9. 参考链接

- [管理员手册](../guides/admin-guide.md)
- [部署手册](../guides/deployment-guide.md)
- [API 文档](./api-reference.md)
- [PostgreSQL 官方文档](https://www.postgresql.org/docs/)

---

*本文档适用于 TARS MVP 版本，Schema 可能会在未来版本中调整。*
