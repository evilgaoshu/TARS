# TARS 配置参考手册

> **版本**: v1.0
> **适用版本**: TARS MVP (Phase 1)
> **最后更新**: 2026-03-13

---

## 目录

1. [配置概述](#1-配置概述)
2. [环境变量完整参考](#2-环境变量完整参考)
3. [配置文件详解](#3-配置文件详解)
4. [配置依赖关系](#4-配置依赖关系)
5. [部署场景配置示例](#5-部署场景配置示例)
6. [配置验证方法](#6-配置验证方法)
7. [配置热重载](#7-配置热重载)

---

## 1. 配置概述

### 1.1 配置加载优先级

TARS 配置按以下优先级加载（高优先级覆盖低优先级）：

```
1. 环境变量（最高优先级）
2. 配置文件
3. 默认值（最低优先级）
```

### 1.2 配置分类

| 类别 | 说明 | 主要配置项 |
|------|------|-----------|
| 基础配置 | 服务运行时配置 | Server、Log、Web |
| 存储配置 | 数据持久化配置 | PostgreSQL、SQLite-vec |
| 渠道配置 | 通知渠道配置 | Telegram |
| 模型配置 | AI 模型配置 | Model、Providers |
| 执行配置 | 命令执行配置 | SSH、VictoriaMetrics |
| 安全配置 | 安全策略配置 | Approval、Authorization |
| 功能配置 | 功能开关 | Features |

### 1.3 配置验证

启动时会自动验证配置，无效配置会导致启动失败：

```bash
# 查看配置验证错误
./tars
# 输出: failed to bootstrap app: invalid config: TARS_POSTGRES_DSN is required
```

---

## 2. 环境变量完整参考

### 2.1 基础配置

#### Server 配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_SERVER_LISTEN` | string | `:8081` | 否 | 统一 HTTP 入口监听地址 |
| `TARS_SERVER_PUBLIC_BASE_URL` | string | `https://tars.example.com` | 否 | 公网访问 URL |
| `TARS_WEB_DIST_DIR` | string | `./web/dist` | 否 | Web Console 静态文件目录 |
| `TARS_LOG_LEVEL` | string | `INFO` | 否 | 日志级别 (DEBUG/INFO/WARN/ERROR) |

**示例**:
```bash
export TARS_SERVER_LISTEN=":8081"
export TARS_SERVER_PUBLIC_BASE_URL="https://tars.company.com"
export TARS_LOG_LEVEL="INFO"
```

### 2.2 数据库配置

#### PostgreSQL

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_POSTGRES_DSN` | string | - | 是 | PostgreSQL 连接字符串 |
| `TARS_RUNTIME_CONFIG_REQUIRE_POSTGRES` | bool | `false` | 生产建议 | 开启后 runtime config 不允许静默降级到内存；未配置 `TARS_POSTGRES_DSN` 时启动失败 |
| `TARS_SECRET_CUSTODY_KEY` | string | - | SSH 凭据托管必需 | 32 字节或 base64 编码 32 字节 master key；用于 PG encrypted secret backend |
| `TARS_SECRET_CUSTODY_KEY_ID` | string | `local` | 否 | 当前 custody key 的版本标识，便于后续轮换 |

**DSN 格式**:
```
postgres://[user]:[password]@[host]:[port]/[dbname]?[params]
```

**示例**:
```bash
# 基础连接
export TARS_POSTGRES_DSN="postgres://tars:tars@localhost:5432/tars?sslmode=disable"

# 带连接池参数
export TARS_POSTGRES_DSN="postgres://tars:tars@localhost:5432/tars?sslmode=disable&pool_max_conns=20&pool_min_conns=5"

# 使用 Unix Socket
export TARS_POSTGRES_DSN="postgres:///tars?host=/var/run/postgresql"
```

**Runtime config 持久化边界**:
- `access / providers / connectors / authorization / approval_routing / org / reasoning_prompts / desensitization / agent_roles / setup_state` 走 PostgreSQL runtime config 主路径。
- SSH 密码/私钥不直接写入普通 runtime config 表；启用 `TARS_SECRET_CUSTODY_KEY` 后会进入 `encrypted_secrets` PG 密文表，`ssh_credentials` 只保存 metadata 和 `secret_ref`。
- 推理 Provider、Telegram 等其它 secrets 暂仍使用 secret ref 与私有 secret backend/file 注入，避免把模型 key 等凭据塞进普通 JSON 文档。
- `automations / skills / extensions` 暂保留现有文件或 state 文件语义，后续再按 lifecycle 需求决定 JSON document 或专用表。

#### SQLite-vec (向量存储)

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_VECTOR_SQLITE_PATH` | string | - | 是 | SQLite 数据库文件路径 |

**示例**:
```bash
export TARS_VECTOR_SQLITE_PATH="/data/tars/vector.db"
```

### 2.3 Ops API 配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_OPS_API_ENABLED` | bool | `false` | 否 | 是否在统一入口上启用受保护的 Ops API |
| `TARS_OPS_API_LISTEN` | string | `127.0.0.1:8081` | 否 | 兼容旧双端口部署的遗留配置；当前单 listener 运行时不会再启动独立 Ops server |
| `TARS_OPS_API_TOKEN` | string | - | 条件 | Ops API 认证 Token (启用时必需) |

**示例**:
```bash
export TARS_OPS_API_ENABLED="true"
export TARS_OPS_API_TOKEN="secure-token-here"
```

**安全建议**:
- Token 长度至少 32 字符
- 定期轮换 Token
- 生产环境建议通过 Secrets Manager 注入

### 2.4 Telegram 配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_TELEGRAM_BOT_TOKEN` | string | - | 条件 | Bot Token (启用 Telegram 时必需) |
| `TARS_TELEGRAM_WEBHOOK_SECRET` | string | - | 条件 | Webhook 签名密钥 (Webhook 模式必需) |
| `TARS_TELEGRAM_BASE_URL` | string | `https://api.telegram.org` | 否 | Telegram API 基础 URL |
| `TARS_TELEGRAM_POLLING_ENABLED` | bool | `false` | 否 | 是否使用轮询模式 |
| `TARS_TELEGRAM_POLL_TIMEOUT` | duration | `30s` | 否 | 轮询超时时间 |
| `TARS_TELEGRAM_POLL_INTERVAL` | duration | `1s` | 否 | 轮询间隔 |

**示例**:
```bash
# Webhook 模式（生产推荐）
export TARS_TELEGRAM_BOT_TOKEN="123456:ABC-DEF..."
export TARS_TELEGRAM_WEBHOOK_SECRET="webhook-secret-here"

# Polling 模式（开发/测试）
export TARS_TELEGRAM_BOT_TOKEN="123456:ABC-DEF..."
export TARS_TELEGRAM_POLLING_ENABLED="true"
```

### 2.5 VMAlert 配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_VMALERT_WEBHOOK_SECRET` | string | - | 条件 | VMAlert Webhook 签名密钥 |

**示例**:
```bash
export TARS_VMALERT_WEBHOOK_SECRET="vmalert-secret"
```

### 2.6 模型配置

#### 基础模型配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_MODEL_PROTOCOL` | string | `openai_compatible` | 否 | 模型协议 |
| `TARS_MODEL_BASE_URL` | string | - | 条件 | 模型网关地址 |
| `TARS_MODEL_API_KEY` | string | - | 条件 | API Key |
| `TARS_MODEL_NAME` | string | `gpt-4o-mini` | 否 | 模型名称 |
| `TARS_MODEL_TIMEOUT` | duration | `30s` | 否 | 请求超时 |

**支持协议**:
- `openai_compatible` - OpenAI 兼容接口
- `openrouter` - OpenRouter API
- `lmstudio` - LM Studio 本地模型

**示例**:
```bash
# OpenAI
export TARS_MODEL_BASE_URL="https://api.openai.com/v1"
export TARS_MODEL_API_KEY="sk-..."
export TARS_MODEL_NAME="gpt-4o-mini"

# LM Studio 本地
export TARS_MODEL_BASE_URL="http://localhost:1234/v1"
export TARS_MODEL_API_KEY="not-needed"
export TARS_MODEL_NAME="local-model"
```

#### Providers 配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_PROVIDERS_CONFIG_PATH` | string | - | 否 | Providers 配置文件路径 |

### 2.7 VictoriaMetrics 配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_VM_BASE_URL` | string | - | 条件 | VM 查询地址 |
| `TARS_VM_TIMEOUT` | duration | `15s` | 否 | 查询超时 |

**示例**:
```bash
export TARS_VM_BASE_URL="http://victoriametrics:8428/select/0/prometheus"
export TARS_VM_TIMEOUT="15s"
```

### 2.8 SSH 配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_SSH_USER` | string | - | 条件 | SSH 用户名 |
| `TARS_SSH_PRIVATE_KEY_PATH` | string | - | 条件 | 私钥文件路径 |
| `TARS_SSH_CONNECT_TIMEOUT` | duration | `10s` | 否 | 连接超时 |
| `TARS_SSH_COMMAND_TIMEOUT` | duration | `5m` | 否 | 命令执行超时 |
| `TARS_SSH_ALLOWED_HOSTS` | CSV | - | 条件 | 允许的主机列表 |
| `TARS_SSH_ALLOWED_COMMAND_PREFIXES` | CSV | - | 否 | 允许的命令前缀 |
| `TARS_SSH_BLOCKED_COMMAND_FRAGMENTS` | CSV | - | 否 | 禁止的命令片段 |
| `TARS_SSH_DISABLE_HOST_KEY_CHECKING` | bool | `false` | 否 | 禁用主机密钥检查 |

**CSV 格式**:
逗号分隔的字符串，如 `"host1,host2,host3"`

**示例**:
```bash
export TARS_SSH_USER="sre"
export TARS_SSH_PRIVATE_KEY_PATH="/etc/tars/id_rsa"
export TARS_SSH_ALLOWED_HOSTS="192.168.1.0/24,10.0.0.0/8,prod-*"
export TARS_SSH_ALLOWED_COMMAND_PREFIXES="uptime,df -h,systemctl status"
export TARS_SSH_BLOCKED_COMMAND_FRAGMENTS="rm -rf,mkfs,shutdown"
export TARS_SSH_DISABLE_HOST_KEY_CHECKING="false"
```

### 2.9 执行输出配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_EXECUTION_OUTPUT_SPOOL_DIR` | string | `./data/execution_output` | 否 | 输出存储目录 |
| `TARS_EXECUTION_OUTPUT_MAX_PERSISTED_BYTES` | int | `262144` | 否 | 最大存储字节数 (256KB) |
| `TARS_EXECUTION_OUTPUT_CHUNK_BYTES` | int | `16384` | 否 | 分块大小 (16KB) |
| `TARS_EXECUTION_OUTPUT_RETENTION` | duration | `168h` | 否 | 保留时间 (7天) |

**示例**:
```bash
export TARS_EXECUTION_OUTPUT_SPOOL_DIR="/data/tars/output"
export TARS_EXECUTION_OUTPUT_MAX_PERSISTED_BYTES="524288"  # 512KB
export TARS_EXECUTION_OUTPUT_RETENTION="336h"  # 14天
```

### 2.10 审批配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_APPROVAL_TIMEOUT` | duration | `15m` | 否 | 审批超时时间 |
| `TARS_APPROVALS_CONFIG_PATH` | string | - | 否 | 审批配置文件路径 |

**示例**:
```bash
export TARS_APPROVAL_TIMEOUT="30m"
export TARS_APPROVALS_CONFIG_PATH="/etc/tars/approvals.yaml"
```

### 2.11 授权配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_AUTHORIZATION_CONFIG_PATH` | string | - | 否 | 授权策略配置文件路径 |

### 2.12 Reasoning 配置

| 变量 | 类型 | 默认值 | 必需 | 说明 |
|------|------|--------|------|------|
| `TARS_REASONING_PROMPTS_CONFIG_PATH` | string | - | 否 | Prompt 配置文件路径 |
| `TARS_DESENSITIZATION_CONFIG_PATH` | string | - | 否 | 脱敏配置文件路径 |
| `TARS_REASONING_LOCAL_COMMAND_FALLBACK_ENABLED` | bool | `false` | 否 | 本地命令回退 |

### 2.13 功能开关

#### 部署模式

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `TARS_ROLLOUT_MODE` | string | `diagnosis_only` | 部署模式 |

**支持模式**:
- `diagnosis_only` - 仅诊断
- `approval_beta` - 诊断 + 审批
- `execution_beta` - 诊断 + 审批 + 执行
- `knowledge_on` - 完整功能

#### 独立开关

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `TARS_FEATURES_DIAGNOSIS_ENABLED` | bool | `true` | 启用诊断 |
| `TARS_FEATURES_APPROVAL_ENABLED` | bool | `false` | 启用审批 |
| `TARS_FEATURES_EXECUTION_ENABLED` | bool | `false` | 启用执行 |
| `TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED` | bool | `false` | 启用知识沉淀 |

**示例**:
```bash
# 方式 1：使用部署模式
export TARS_ROLLOUT_MODE="execution_beta"

# 方式 2：独立开关（覆盖部署模式）
export TARS_ROLLOUT_MODE="custom"
export TARS_FEATURES_DIAGNOSIS_ENABLED="true"
export TARS_FEATURES_APPROVAL_ENABLED="true"
export TARS_FEATURES_EXECUTION_ENABLED="true"
```

### 2.14 GC 配置

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `TARS_GC_ENABLED` | bool | `true` | 启用垃圾回收 |
| `TARS_GC_INTERVAL` | duration | `1h` | GC 间隔 |

### 2.15 Connector 配置

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `TARS_CONNECTORS_CONFIG_PATH` | string | - | Connectors 配置文件路径 |
| `TARS_CONNECTORS_SECRETS_PATH` | string | - | Connectors secrets 路径 |
| `TARS_CONNECTORS_TEMPLATES_PATH` | string | - | Connectors 模板路径 |

---

## 3. 配置文件详解

### 3.1 主配置文件 (tars.yaml)

```yaml
# 服务器配置
server:
  listen: ":8081"
  public_base_url: "https://tars.example.com"

# 数据库配置
postgres:
  dsn: "postgres://user:pass@localhost:5432/tars?sslmode=disable"

# 向量存储
vector:
  sqlite_path: "./data/tars_vec.db"

# 模型配置
model:
  protocol: "openai_compatible"
  base_url: "https://api.openai.com/v1"
  api_key: "${MODEL_API_KEY}"  # 从环境变量读取
  model: "gpt-4o-mini"
  timeout: "30s"

# Reasoning 配置
reasoning:
  prompts_config_path: "./configs/reasoning_prompts.yaml"
  desensitization_config_path: "./configs/desensitization.yaml"
  local_command_fallback_enabled: false

# Ops API
ops_api:
  enabled: true
  listen: "127.0.0.1:8081"
  bearer_token: "${OPS_API_TOKEN}"
  trusted_proxy_cidrs: ["10.0.0.0/8"]
  require_gateway_identity: true

# Telegram 配置
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  webhook_secret: "${TELEGRAM_WEBHOOK_SECRET}"
  base_url: "https://api.telegram.org"
  polling_enabled: false
  poll_timeout: "30s"
  poll_interval: "1s"

# VMAlert 配置
vmalert:
  webhook_secret: "${VMALERT_WEBHOOK_SECRET}"

# VictoriaMetrics 配置
victoriametrics:
  query_base_url: "https://vm.example.com/select/0/prometheus"
  timeout: "15s"

# SSH 配置
ssh:
  user: "sre"
  private_key_path: "/etc/tars/id_rsa"
  connect_timeout: "10s"
  command_timeout: "300s"
  disable_host_key_checking: false
  allowed_hosts: ["192.168.1.0/24", "10.0.0.0/8"]
  allowed_command_prefixes:
    - "hostname"
    - "uptime"
    - "systemctl status"
  blocked_command_fragments:
    - "rm -rf"
    - "mkfs"
    - "shutdown"

# 执行输出配置
execution_output:
  max_persisted_bytes: 262144
  chunk_bytes: 16384
  retention: "168h"
  spool_dir: "./data/execution_output"

# 审批配置
approval:
  timeout: "15m"
  config_path: "./configs/approvals.yaml"

# 授权配置
authorization:
  config_path: "./configs/authorization_policy.yaml"

# 功能开关
features:
  rollout_mode: "execution_beta"
  diagnosis_enabled: true
  approval_enabled: true
  execution_enabled: true
  knowledge_ingest_enabled: false

# GC 配置
gc:
  enabled: true
  interval: "1h"
  execution_output_retain: "168h"
```

### 3.2 审批路由配置 (approvals.yaml)

```yaml
approval:
  # 默认审批超时
  default_timeout: 15m

  # 禁止自审批
  prohibit_self_approval: true

  # 审批路由
  routing:
    # 按服务 owner 路由
    service_owner:
      web:
        - "u_alice"
        - "u_bob"
      payment:
        - "u_charlie"
      api:
        - "u_dave"

    # 按值班组路由
    oncall_group:
      default:
        - "u_sre_1"
        - "u_sre_2"
      emergency:
        - "u_manager"

  # 执行命令白名单
  execution:
    command_allowlist:
      sshd:
        - "systemctl restart sshd"
        - "systemctl status sshd"
        - "systemctl is-active sshd"
      web:
        - "systemctl restart nginx"
        - "systemctl status nginx"
        - "systemctl reload nginx"
      database:
        - "systemctl status postgresql"
```

### 3.3 授权策略配置 (authorization_policy.yaml)

```yaml
authorization:
  # 默认动作
  defaults:
    whitelist_action: direct_execute   # 白名单命令
    blacklist_action: suggest_only     # 黑名单命令
    unmatched_action: require_approval # 其他命令

  # 硬性拒绝（永不执行）
  hard_deny:
    ssh_command:
      - "rm -rf /"
      - "mkfs*"
      - "dd if=/dev/zero*"
      - ">/dev/sda"
    mcp_skill:
      - "shell.exec_root*"
      - "kubernetes.delete*"

  # SSH 命令策略
  ssh_command:
    normalize_whitespace: true
    whitelist:
      - "hostname"
      - "uptime"
      - "whoami"
      - "cat /proc/loadavg"
      - "df -h*"
      - "free -m*"
      - "ps aux*"
      - "systemctl status *"
      - "systemctl is-active *"
      - "journalctl -u *"
      - "ss -tlnp"
      - "netstat -tlnp"
    blacklist:
      - "systemctl restart *"
      - "systemctl stop *"
      - "systemctl disable *"
      - "reboot*"
      - "shutdown*"
      - "init 0"
      - "init 6"
      - "iptables *"
      - "fdisk *"
      - "parted *"
    overrides:
      - id: "sshd-restart-with-approval"
        services:
          - "sshd"
        hosts:
          - "192.168.3.*"
          - "prod-*"
        command_globs:
          - "systemctl restart sshd*"
        action: require_approval
        approval_route: service_owner
        approval_timeout: 15m

  # MCP Skill 策略
  mcp_skill:
    whitelist:
      - "victoriametrics.query_*"
      - "grafana.read_*"
      - "kubernetes.get_*"
      - "kubernetes.describe_*"
      - "kubernetes.logs_*"
    blacklist:
      - "terraform.destroy*"
      - "github.delete_*"
      - "shell.exec_*"
      - "kubernetes.delete_*"
      - "kubernetes.exec_*"
```

### 3.4 脱敏配置 (desensitization.yaml)

```yaml
desensitization:
  enabled: true

  # Secrets 脱敏
  secrets:
    key_names:
      - password
      - passwd
      - token
      - secret
      - api_key
      - api_secret
      - private_key
      - credential
    query_key_names:
      - access_token
      - refresh_token
      - token
      - secret
      - api_key
    additional_patterns:
      - "corp-[A-Z0-9]{6}"  # 公司特定模式
    redact_bearer: true
    redact_basic_auth_url: true
    redact_sk_tokens: true

  # 占位符配置
  placeholders:
    host_key_fragments:
      - host
      - hostname
      - instance
      - node
      - address
      - fqdn
    path_key_fragments:
      - path
      - file
      - filename
      - dir
      - directory
    replace_inline_ip: true
    replace_inline_host: true
    replace_inline_path: true

  # 回水配置
  rehydration:
    host: true
    ip: true
    path: true

  # 本地 LLM 辅助
  local_llm_assist:
    enabled: false
    provider: "openai_compatible"
    base_url: "http://127.0.0.1:11434/v1"
    model: "qwen2.5"
    mode: "detect_only"
```

### 3.5 Provider 配置 (providers.yaml)

```yaml
providers:
  # 主模型
  primary:
    provider_id: "primary-openrouter"
    model: "openai/gpt-4.1-mini"

  # 辅助模型（用于本地推理）
  assist:
    provider_id: "assist-lmstudio"
    model: "qwen/qwen3-4b-2507"

  # Provider 列表
  entries:
    - id: "primary-openrouter"
      vendor: "openrouter"
      protocol: "openrouter"
      base_url: "https://openrouter.ai/api/v1"
      api_key: "replace-with-openrouter-api-key"
      enabled: true

    - id: "assist-lmstudio"
      vendor: "lmstudio"
      protocol: "lmstudio"
      base_url: "http://192.168.1.132:1234"
      enabled: true

    - id: "backup-ollama"
      vendor: "ollama"
      protocol: "openai_compatible"
      base_url: "http://localhost:11434/v1"
      api_key: "not-needed"
      enabled: false
```

---

## 4. 配置依赖关系

### 4.1 必需配置依赖图

```
┌─────────────────────────────────────────────────────┐
│                    Core Services                     │
├─────────────────────────────────────────────────────┤
│  PostgreSQL (TARS_POSTGRES_DSN)                     │
│  └── Required by: All modules                        │
│                                                      │
│  SQLite-vec (TARS_VECTOR_SQLITE_PATH)               │
│  └── Required by: Knowledge Service                  │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│                   Feature Modules                    │
├─────────────────────────────────────────────────────┤
│  Telegram                                            │
│  ├── Requires: TARS_TELEGRAM_BOT_TOKEN              │
│  └── Optional: TARS_TELEGRAM_WEBHOOK_SECRET         │
│                                                      │
│  AI Diagnosis                                        │
│  ├── Requires: TARS_MODEL_BASE_URL                  │
│  ├── Requires: TARS_MODEL_API_KEY                   │
│  └── Optional: TARS_PROVIDERS_CONFIG_PATH           │
│                                                      │
│  Command Execution                                   │
│  ├── Requires: TARS_SSH_USER                        │
│  ├── Requires: TARS_SSH_PRIVATE_KEY_PATH            │
│  └── Requires: TARS_SSH_ALLOWED_HOSTS               │
│                                                      │
│  VM Query                                            │
│  └── Requires: TARS_VM_BASE_URL                     │
└─────────────────────────────────────────────────────┘
```

### 4.2 配置组合矩阵

| 功能 | 必需配置 | 可选配置 |
|------|----------|----------|
| 基础服务 | `POSTGRES_DSN`, `SERVER_LISTEN` | `LOG_LEVEL`, `WEB_DIST_DIR` |
| Telegram 通知 | `TELEGRAM_BOT_TOKEN` | `TELEGRAM_WEBHOOK_SECRET` |
| AI 诊断 | `MODEL_BASE_URL`, `MODEL_API_KEY` | `PROVIDERS_CONFIG_PATH` |
| 审批流程 | `APPROVALS_CONFIG_PATH` | `APPROVAL_TIMEOUT` |
| 命令执行 | `SSH_USER`, `SSH_PRIVATE_KEY_PATH`, `SSH_ALLOWED_HOSTS` | `SSH_COMMAND_TIMEOUT` |
| VM 查询 | `VM_BASE_URL` | `VM_TIMEOUT` |
| 知识沉淀 | `VECTOR_SQLITE_PATH` | `KNOWLEDGE_CONFIG_PATH` |

---

## 5. 部署场景配置示例

### 5.1 本地开发环境

```bash
#!/bin/bash
# dev.env

# 基础
export TARS_SERVER_LISTEN=":8081"
export TARS_LOG_LEVEL="DEBUG"

# 数据库
export TARS_POSTGRES_DSN="postgres://localhost/tars?sslmode=disable"
export TARS_VECTOR_SQLITE_PATH="./data/vector.db"

# Telegram（使用 Polling 模式）
export TARS_TELEGRAM_BOT_TOKEN="your-bot-token"
export TARS_TELEGRAM_POLLING_ENABLED="true"

# 模型（使用本地 LM Studio）
export TARS_MODEL_BASE_URL="http://localhost:1234/v1"
export TARS_MODEL_NAME="local-model"

# 功能
export TARS_ROLLOUT_MODE="execution_beta"

# SSH（可选）
# export TARS_SSH_USER="sre"
# export TARS_SSH_PRIVATE_KEY_PATH="~/.ssh/id_rsa"
```

### 5.2 Docker 生产环境

```yaml
# docker-compose.yml 环境变量部分
services:
  tars:
    environment:
      # 基础
      - TARS_SERVER_LISTEN=:8081
      - TARS_SERVER_PUBLIC_BASE_URL=https://tars.company.com
      - TARS_LOG_LEVEL=INFO

      # 数据库
      - TARS_POSTGRES_DSN=postgres://tars:${DB_PASSWORD}@postgres:5432/tars?sslmode=require
      - TARS_VECTOR_SQLITE_PATH=/data/vector.db

      # Ops API
      - TARS_OPS_API_ENABLED=true
      - TARS_OPS_API_TOKEN=${OPS_API_TOKEN}

      # Telegram
      - TARS_TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
      - TARS_TELEGRAM_WEBHOOK_SECRET=${TELEGRAM_WEBHOOK_SECRET}

      # VMAlert
      - TARS_VMALERT_WEBHOOK_SECRET=${VMALERT_WEBHOOK_SECRET}

      # 模型（使用 OpenRouter）
      - TARS_PROVIDERS_CONFIG_PATH=/etc/tars/providers.yaml

      # VM
      - TARS_VM_BASE_URL=http://victoriametrics:8428/select/0/prometheus

      # SSH
      - TARS_SSH_USER=sre
      - TARS_SSH_PRIVATE_KEY_PATH=/etc/tars/ssh_key
      - TARS_SSH_ALLOWED_HOSTS=10.0.0.0/8,192.168.0.0/16

      # 功能
      - TARS_ROLLOUT_MODE=execution_beta

      # 配置路径
      - TARS_APPROVALS_CONFIG_PATH=/etc/tars/approvals.yaml
      - TARS_AUTHORIZATION_CONFIG_PATH=/etc/tars/authorization_policy.yaml
```

### 5.3 Kubernetes 环境

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tars
spec:
  template:
    spec:
      containers:
        - name: tars
          env:
            - name: TARS_SERVER_LISTEN
              value: ":8081"
            - name: TARS_POSTGRES_DSN
              valueFrom:
                secretKeyRef:
                  name: tars-secrets
                  key: postgres-dsn
            - name: TARS_OPS_API_TOKEN
              valueFrom:
                secretKeyRef:
                  name: tars-secrets
                  key: ops-api-token
            - name: TARS_TELEGRAM_BOT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: tars-secrets
                  key: telegram-token
```

### 5.4 高可用部署

```bash
# ha.env
# 多个实例共享同一个数据库

# 实例 1
export TARS_SERVER_LISTEN=":8081"
export TARS_POSTGRES_DSN="postgres://tars:pass@pg-cluster:5432/tars"

# 实例 2（只读）
export TARS_SERVER_LISTEN=":8081"
export TARS_POSTGRES_DSN="postgres://tars:pass@pg-cluster:5432/tars"
export TARS_OPS_API_ENABLED="false"  # 只在主实例启用
```

---

## 6. 配置验证方法

### 6.1 启动前验证

```bash
# 检查必需环境变量
#!/bin/bash

required_vars=(
  "TARS_POSTGRES_DSN"
)

for var in "${required_vars[@]}"; do
  if [ -z "${!var}" ]; then
    echo "ERROR: $var is not set"
    exit 1
  fi
done

echo "All required variables are set"
```

### 6.2 启动时验证

```bash
# TARS 启动时会自动验证配置
./tars

# 查看验证错误
# 日志输出: failed to bootstrap app: invalid config: ...
```

### 6.3 运行时验证

```bash
# 检查健康状态
curl http://localhost:8081/healthz

# 检查配置状态（Ops API）
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/setup/status
```

### 6.4 配置文件语法验证

```bash
# YAML 语法检查
yamllint configs/*.yaml

# 或使用 Python
python3 -c "import yaml; yaml.safe_load(open('configs/tars.yaml'))"
```

---

## 7. 配置热重载

### 7.1 支持热重载的配置

以下配置支持热重载（无需重启）：

- `reasoning_prompts.yaml` - Reasoning Prompt
- `desensitization.yaml` - 脱敏规则
- `providers.yaml` - Provider 配置
- `approvals.yaml` - 审批路由

### 7.2 热重载方法

```bash
# 方式 1：通过 Ops API 触发重载
curl -X POST \
  -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/config/reload

# 方式 2：修改文件后自动检测（如果启用）
# 需要配置 TARS_CONFIG_AUTO_RELOAD=true
```

### 7.3 配置生效验证

```bash
# 验证配置已更新
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/config/reasoning-prompts

# 检查更新时间
# 响应中的 updated_at 字段
```

### 7.4 不支持热重载的配置

以下配置需要重启服务：

- 数据库连接配置
- 服务器监听地址
- 功能开关
- SSH 配置
- Telegram 配置

---

## 8. 配置安全最佳实践

### 8.1 Secrets 管理

```bash
# 不推荐：直接设置环境变量
export TARS_MODEL_API_KEY="sk-..."

# 推荐：使用 Secrets Manager
export TARS_MODEL_API_KEY="$(aws secretsmanager get-secret-value ...)"

# 推荐：使用 Docker Secrets
docker secret create tars_model_api_key -
```

### 8.2 配置文件权限

```bash
# 设置适当的权限
chmod 600 /etc/tars/*.yaml
chown tars:tars /etc/tars/*.yaml

# 私钥权限
chmod 600 /etc/tars/id_rsa
```

### 8.3 敏感信息过滤

```bash
# 日志中过滤敏感信息
export TARS_LOG_SENSITIVE_MASK=true

# 审计日志中排除敏感字段
```

---

## 9. 参考链接

- [部署手册](../guides/deployment-guide.md)
- [管理员手册](../guides/admin-guide.md)
- [API 文档](./api-reference.md)
- [产品需求文档](../../project/tars_prd.md)

---

*本文档适用于 TARS MVP 版本，配置项可能会在未来版本中调整。*
