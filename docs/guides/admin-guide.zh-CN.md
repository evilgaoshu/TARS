# TARS 管理员手册

> **版本**: v1.0
> **适用版本**: TARS MVP (Phase 1)
> **最后更新**: 2026-03-13

---

## 1. 系统架构深度说明

### 1.1 架构概览

TARS 采用 **模块化单体 (Modular Monolith)** 架构设计：

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP Server                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────┐ │
│  │ Alert       │  │ Channel      │  │ Workflow Core       │ │
│  │ Intake      │  │ Adapter      │  │ (状态机 + 编排)      │ │
│  └─────────────┘  └──────────────┘  └─────────────────────┘ │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────┐ │
│  │ Reasoning   │  │ Action       │  │ Knowledge           │ │
│  │ Service     │  │ Gateway      │  │ Service             │ │
│  └─────────────┘  └──────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                     Foundation 层                            │
│  (Config / Logger / Metrics / Audit / Tracing / Secrets)     │
├─────────────────────────────────────────────────────────────┤
│  PostgreSQL  │  SQLite-vec  │  外部模型  │  SSH  │  VM      │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 核心设计原则

| 原则 | 说明 |
|------|------|
| 状态集中 | 所有业务状态由 Workflow Core 统一维护 |
| 交互收口 | 所有用户交互通过 Channel Adapter 统一转换 |
| 外部集成收口 | 所有南向调用通过 Action Gateway 出去 |
| LLM 可替换 | Reasoning Service 只依赖模型接口，不感知执行和状态 |
| 知识独立演进 | Knowledge Service 通过查询和事件独立演进 |

### 1.3 模块职责边界

| 模块 | 职责 | 边界约束 |
|------|------|----------|
| Alert Intake | 告警接入与标准化 | 不做业务决策，不直接调用 LLM |
| Channel Adapter | 渠道协议转换 | 不持有业务状态，不做审批判定 |
| Workflow Core | 状态机与编排 | 唯一可修改业务状态的模块 |
| Reasoning Service | 诊断建议生成 | 不执行命令，不修改状态 |
| Action Gateway | 外部动作执行 | 不拥有业务状态，不决定执行权限 |
| Knowledge Service | 知识检索与沉淀 | 不参与执行，不直接修改审批结果 |

### 1.4 调用关系

```
Alert Intake ──→ Workflow Core
Channel Adapter ──→ Workflow Core
Workflow Core ──→ Channel Adapter
Workflow Core ──→ Reasoning Service
Workflow Core ──→ Action Gateway
Workflow Core ──session_closed event──→ Knowledge Service
Reasoning Service ──→ Knowledge Service
Action Gateway ──→ 外部系统
Foundation ──→ 横切所有模块
```

---

## 2. 配置管理

### 2.1 授权策略配置

#### 2.1.1 配置文件位置

- 环境变量: `TARS_AUTHORIZATION_CONFIG_PATH`
- 默认路径: `./configs/authorization_policy.yaml`

#### 2.1.2 配置结构

```yaml
authorization:
  defaults:
    whitelist_action: direct_execute   # 白名单命令处理方式
    blacklist_action: suggest_only     # 黑名单命令处理方式
    unmatched_action: require_approval # 其他命令处理方式

  hard_deny:
    ssh_command:
      - "rm -rf /"
      - "mkfs*"
      - "dd if=/dev/zero*"
    mcp_skill:
      - "shell.exec_root*"

  ssh_command:
    normalize_whitespace: true
    whitelist:
      - "hostname"
      - "uptime"
      - "cat /proc/loadavg"
      - "df -h*"
      - "free -m*"
      - "systemctl status *"
      - "systemctl is-active *"
      - "journalctl -u *"
    blacklist:
      - "systemctl restart *"
      - "systemctl stop *"
      - "reboot*"
      - "shutdown*"
      - "iptables *"
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

  mcp_skill:
    whitelist:
      - "victoriametrics.query_*"
      - "grafana.read_*"
      - "kubernetes.get_*"
    blacklist:
      - "terraform.destroy*"
      - "github.delete_*"
      - "shell.exec_*"
```

#### 2.1.3 动作类型说明

| 动作 | 说明 |
|------|------|
| `direct_execute` | 直接执行，无需审批 |
| `require_approval` | 需要人工审批 |
| `suggest_only` | 仅建议，不执行 |
| `hard_deny` | 硬性拒绝，禁止执行 |

### 2.2 审批路由配置

#### 2.2.1 配置文件位置

- 环境变量: `TARS_APPROVALS_CONFIG_PATH`
- 默认路径: `./configs/approvals.yaml`

#### 2.2.2 配置结构

```yaml
approval:
  default_timeout: 15m              # 默认审批超时
  prohibit_self_approval: true      # 是否禁止自审批

  routing:
    service_owner:                  # 按服务 owner 路由
      web:
        - "u_alice"
        - "u_bob"
      payment:
        - "u_charlie"

    oncall_group:                   # 按值班组路由
      default:
        - "u_sre_1"
        - "u_sre_2"

  execution:
    command_allowlist:              # 命令白名单
      sshd:
        - "systemctl restart sshd"
        - "systemctl status sshd"
        - "systemctl is-active sshd"
      web:
        - "systemctl restart nginx"
        - "systemctl status nginx"
```

#### 2.2.3 路由匹配规则

1. 优先按 `service` 标签匹配 `service_owner`
2. 未匹配时回退到 `oncall_group`
3. `critical` 级别需要双人审批

### 2.3 Reasoning Prompt 配置

#### 2.3.1 配置文件位置

- 环境变量: `TARS_REASONING_PROMPTS_CONFIG_PATH`
- 默认路径: `./configs/reasoning_prompts.yaml`

#### 2.3.2 配置结构

```yaml
reasoning:
  system_prompt: |
    You are TARS, an operations copilot.
    Return ONLY strict JSON with fields: summary, execution_hint.
    Do not use markdown, code fences, or extra prose.
    summary must be concise and actionable.
    execution_hint must be a single shell command candidate.
    Prefer read-only commands for operator chat requests.
    ...

  user_prompt_template: |
    session_id={{ .SessionID }}
    context={{ .ContextJSON }}
```

#### 2.3.3 Prompt 优化建议

- 保持 System Prompt 简洁明确
- 使用严格的 JSON 输出格式要求
- 明确指定命令偏好和约束
- 提供示例输出格式

### 2.4 脱敏规则配置

#### 2.4.1 配置文件位置

- 环境变量: `TARS_DESENSITIZATION_CONFIG_PATH`
- 默认路径: `./configs/desensitization.yaml`

#### 2.4.2 配置结构

```yaml
desensitization:
  enabled: true

  secrets:
    key_names:                      # 敏感 key 名
      - password
      - passwd
      - token
      - secret
      - api_key
    query_key_names:                # 查询参数中的敏感 key
      - access_token
      - refresh_token
    additional_patterns:            # 额外匹配模式
      - "corp-[A-Z0-9]{6}"
    redact_bearer: true             # 脱敏 Bearer Token
    redact_basic_auth_url: true     # 脱敏 URL 中的认证信息
    redact_sk_tokens: true          # 脱敏 sk- 开头的 token

  placeholders:
    host_key_fragments:             # 主机相关 key
      - host
      - hostname
      - instance
      - node
    path_key_fragments:             # 路径相关 key
      - path
      - file
      - filename
    replace_inline_ip: true         # 替换内联 IP
    replace_inline_host: true       # 替换内联主机名
    replace_inline_path: true       # 替换内联路径

  rehydration:
    host: true                      # 回水主机名
    ip: true                        # 回水 IP
    path: true                      # 回水路径

  local_llm_assist:
    enabled: false
    provider: "openai_compatible"
    base_url: "http://127.0.0.1:11434/v1"
    model: "qwen2.5"
    mode: "detect_only"
```

#### 2.4.3 脱敏规则说明

| 类型 | 处理方式 | 是否可回水 |
|------|----------|------------|
| Secrets | 替换为 `[REDACTED]` | 否 (永久脱敏) |
| IP 地址 | 替换为占位符 | 是 |
| 主机名 | 替换为占位符 | 是 |
| 路径 | 替换为占位符 | 是 |

### 2.5 Provider 配置

#### 2.5.1 配置文件位置

- 环境变量: `TARS_PROVIDERS_CONFIG_PATH`
- 默认路径: `./configs/providers.yaml`

#### 2.5.2 配置结构

```yaml
providers:
  primary:                          # 主模型
    provider_id: "primary-openrouter"
    model: "openai/gpt-4.1-mini"

  assist:                           # 辅助模型
    provider_id: "assist-lmstudio"
    model: "qwen/qwen3-4b-2507"

  entries:
    - id: "primary-openrouter"
      vendor: "openrouter"
      protocol: "openrouter"
      base_url: "https://openrouter.ai/api/v1"
      api_key: "..."
      enabled: true

    - id: "assist-lmstudio"
      vendor: "lmstudio"
      protocol: "lmstudio"
      base_url: "http://192.168.1.132:1234"
      enabled: true
```

---

## 3. 监控和告警

### 3.1 日志分析

#### 3.1.1 日志级别

| 级别 | 用途 |
|------|------|
| DEBUG | 详细调试信息 |
| INFO | 正常操作日志 |
| WARN | 警告信息 |
| ERROR | 错误信息 |

#### 3.1.2 日志配置

```bash
# 环境变量设置日志级别
export TARS_LOG_LEVEL=INFO

# Docker Compose
docker compose logs -f tars

# Systemd
sudo journalctl -u tars -f
```

#### 3.1.3 关键日志模式

```bash
# 搜索错误日志
grep "ERROR" /var/log/tars/*.log

# 搜索特定会话
grep "session_id=xxx" /var/log/tars/*.log

# 搜索审批事件
grep "approval" /var/log/tars/*.log
```

### 3.2 指标监控

#### 3.2.1 Prometheus Metrics

TARS 暴露以下指标端点：

```bash
# 获取所有指标
curl http://localhost:8080/metrics
```

#### 3.2.2 关键指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `tars_sessions_total` | Counter | 总会话数 |
| `tars_sessions_active` | Gauge | 活动会话数 |
| `tars_executions_total` | Counter | 总执行数 |
| `tars_approvals_pending` | Gauge | 待审批数 |
| `tars_webhook_requests_total` | Counter | Webhook 请求数 |
| `tars_telegram_messages_sent` | Counter | Telegram 发送消息数 |
| `http_requests_total` | Counter | HTTP 请求数 |
| `http_request_duration_seconds` | Histogram | HTTP 请求耗时 |

#### 3.2.3 Grafana Dashboard

使用提供的 Dashboard 配置：

```bash
# 导入 Dashboard
curl -X POST http://grafana:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @deploy/grafana/tars-mvp-dashboard.json
```

### 3.3 健康检查

#### 3.3.1 健康检查端点

```bash
# 健康检查
curl http://localhost:8080/healthz
# 响应: ok

# 就绪检查
curl http://localhost:8080/readyz
# 响应: ok

# 详细健康状态 (Ops API)
curl -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8081/api/v1/dashboard/health
```

#### 3.3.2 健康检查组件

| 组件 | 检查内容 |
|------|----------|
| PostgreSQL | 连接可用性 |
| SQLite-vec | 向量存储可用性 |
| Telegram | API 连通性 |
| Model Gateway | 模型服务可用性 |
| VictoriaMetrics | 查询服务可用性 |

---

## 4. 安全管理

### 4.1 SSH 安全

#### 4.1.1 密钥管理

```bash
# 生成专用密钥
ssh-keygen -t ed25519 -f /etc/tars/id_ed25519 -C "tars@example.com"

# 设置权限
chmod 600 /etc/tars/id_ed25519
chmod 644 /etc/tars/id_ed25519.pub

# 定期轮换 (建议每 90 天)
```

#### 4.1.2 主机访问控制

```yaml
ssh:
  allowed_hosts:
    - "192.168.1.0/24"      # 允许的内网段
    - "10.0.0.0/8"
    - "prod-web-*.example.com"  # 允许的主机模式
```

#### 4.1.3 命令白名单

```yaml
ssh:
  allowed_command_prefixes:
    - "hostname"
    - "uptime"
    - "systemctl status"
    - "cat /proc/loadavg"
  blocked_command_fragments:
    - "rm -rf"
    - "mkfs"
    - "shutdown"
    - "dd if=/dev/zero"
```

### 4.2 命令授权

#### 4.2.1 风险分级

| 级别 | 定义 | 示例命令 |
|------|------|----------|
| info | 只读命令 | `uptime`, `df -h`, `free -m` |
| warning | 可恢复写操作 | `systemctl restart`, `kill` |
| critical | 高影响/不可逆 | `reboot`, `mkfs`, `rm -rf` |

#### 4.2.2 授权策略

```yaml
authorization:
  defaults:
    whitelist_action: direct_execute
    blacklist_action: suggest_only
    unmatched_action: require_approval
```

### 4.3 审批策略

#### 4.3.1 审批超时

```yaml
approval:
  default_timeout: 15m
  prohibit_self_approval: true
```

#### 4.3.2 双人审批

- Critical 级别命令需要双人审批
- 审批人必须来自不同的审批组
- 禁止自审批

### 4.4 数据脱敏

#### 4.4.1 脱敏范围

| 数据类型 | 脱敏方式 | 示例 |
|----------|----------|------|
| IP 地址 | 替换为 `[IP:1]` | `192.168.1.1` → `[IP:1]` |
| 主机名 | 替换为 `[HOST:1]` | `prod-web-01` → `[HOST:1]` |
| Secrets | 替换为 `[REDACTED]` | `token=xxx` → `token=[REDACTED]` |
| 密码 | 完全脱敏 | `password=xxx` → `password=[REDACTED]` |

#### 4.4.2 回水机制

脱敏后的数据在执行时需要回水为原始值：

```yaml
desensitization:
  rehydration:
    host: true
    ip: true
    path: true
```

---

## 5. 故障处理

### 5.1 常见问题排查

#### 5.1.1 服务无法启动

**排查步骤**:

1. 检查配置文件格式
   ```bash
   yamllint configs/*.yaml
   ```

2. 检查端口占用
   ```bash
   netstat -tlnp | grep 8080
   ```

3. 检查数据库连接
   ```bash
   psql "${TARS_POSTGRES_DSN}" -c "SELECT 1"
   ```

4. 查看详细错误日志
   ```bash
   docker compose logs tars
   ```

#### 5.1.2 AI 诊断失败

**排查步骤**:

1. 检查模型网关连通性
   ```bash
   curl ${TARS_MODEL_BASE_URL}/models \
     -H "Authorization: Bearer ${TARS_MODEL_API_KEY}"
   ```

2. 检查 Provider 配置
   ```bash
   curl -H "Authorization: Bearer ${OPS_TOKEN}" \
     http://localhost:8081/api/v1/config/providers/check
   ```

3. 查看脱敏配置是否过度

#### 5.1.3 执行失败

**排查步骤**:

1. 检查 SSH 连接
   ```bash
   ssh -i ${TARS_SSH_PRIVATE_KEY_PATH} \
     ${TARS_SSH_USER}@target-host "hostname"
   ```

2. 检查授权策略
   ```bash
   curl -H "Authorization: Bearer ${OPS_TOKEN}" \
     http://localhost:8081/api/v1/config/authorization
   ```

3. 查看执行输出
   ```bash
   curl -H "Authorization: Bearer ${OPS_TOKEN}" \
     http://localhost:8081/api/v1/executions/{id}
   ```

#### 5.1.4 Telegram 通知失败

**排查步骤**:

1. 检查 Webhook 配置
   ```bash
   curl https://api.telegram.org/bot${TOKEN}/getWebhookInfo
   ```

2. 验证 Bot Token
   ```bash
   curl https://api.telegram.org/bot${TOKEN}/getMe
   ```

3. 检查网络连通性
   ```bash
   curl -I https://api.telegram.org
   ```

### 5.2 回滚操作

#### 5.2.1 配置回滚

```bash
# 备份当前配置
cp configs/authorization_policy.yaml \
   configs/authorization_policy.yaml.backup

# 恢复配置
cp configs/authorization_policy.yaml.backup \
   configs/authorization_policy.yaml

# 重新加载 (如支持热重载) 或重启服务
```

#### 5.2.2 数据库回滚

```bash
# 使用备份恢复
pg_dump -U tars tars > backup_before_change.sql

# 恢复
psql -U tars -d tars < backup_before_change.sql
```

### 5.3 数据恢复

#### 5.3.1 执行输出恢复

```bash
# 执行输出存储在文件系统中
ls -la ${TARS_EXECUTION_OUTPUT_SPOOL_DIR}

# 从数据库查询输出引用
psql -U tars -d tars -c \
  "SELECT id, output_ref FROM execution_requests WHERE id='xxx'"
```

#### 5.3.2 会话数据恢复

```bash
# 查询会话数据
psql -U tars -d tars -c \
  "SELECT * FROM alert_sessions WHERE id='xxx'"

# 查询关联事件
psql -U tars -d tars -c \
  "SELECT * FROM session_events WHERE session_id='xxx'"
```

---

## 6. 性能优化

### 6.1 数据库优化

#### 6.1.1 索引优化

```sql
-- 检查慢查询
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- 分析表
ANALYZE alert_sessions;
ANALYZE execution_requests;
```

#### 6.1.2 连接池配置

```yaml
# 在 PostgreSQL DSN 中配置连接参数
postgres:
  dsn: "postgres://user:pass@host/db?sslmode=disable&pool_max_conns=20&pool_min_conns=5"
```

### 6.2 缓存策略

#### 6.2.1 知识库缓存

- 文档向量索引缓存
- 查询结果缓存

#### 6.2.2 配置缓存

- 授权策略缓存
- 审批路由缓存

### 6.3 资源限制

#### 6.3.1 执行超时

```yaml
ssh:
  connect_timeout: "10s"
  command_timeout: "300s"  # 最长 5 分钟

model:
  timeout: "30s"  # 模型调用超时
```

#### 6.3.2 输出限制

```yaml
execution_output:
  max_persisted_bytes: 262144  # 256KB
  chunk_bytes: 16384          # 16KB 分块
  retention: "168h"           # 7 天保留
```

### 6.4 垃圾回收

```yaml
gc:
  enabled: true
  interval: "1h"
  execution_output_retain: "168h"
```

---

## 7. 维护操作

### 7.1 日常检查清单

- [ ] 检查服务健康状态
- [ ] 检查磁盘空间
- [ ] 检查数据库连接数
- [ ] 检查日志错误
- [ ] 检查待审批请求
- [ ] 检查失败执行

### 7.2 定期维护任务

| 任务 | 频率 | 命令 |
|------|------|------|
| 备份数据库 | 每日 | `pg_dump` |
| 清理旧执行输出 | 每日 | GC 自动 |
| 轮换 SSH 密钥 | 每季度 | `ssh-keygen` |
| 审查审计日志 | 每周 | Web Console |
| 更新模型配置 | 按需 | 配置文件 |

### 7.3 容量规划

| 资源 | 监控指标 | 扩容阈值 |
|------|----------|----------|
| CPU | `process_cpu_seconds_total` | > 80% |
| 内存 | `process_resident_memory_bytes` | > 80% |
| 磁盘 | `node_filesystem_avail_bytes` | < 20% |
| 数据库 | `pg_stat_activity_count` | > 80% |

---

## 8. 参考链接

- [部署手册](./deployment-guide.md)
- [用户手册](./user-guide.md)
- [API 文档](../reference/api-reference.md)
- [产品需求文档](../../project/tars_prd.md)
- [技术设计文档](../../project/tars_technical_design.md)

---

*本文档适用于 TARS MVP 版本，管理员应根据实际部署环境调整配置。*
