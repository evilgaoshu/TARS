# TARS 错误码和故障排查手册

> **版本**: v1.0
> **适用版本**: TARS MVP (Phase 1)
> **最后更新**: 2026-03-13

---

## 目录

1. [错误码参考](#1-错误码参考)
2. [启动故障](#2-启动故障)
3. [数据库问题](#3-数据库问题)
4. [AI 诊断问题](#4-ai-诊断问题)
5. [执行问题](#5-执行问题)
6. [Telegram 问题](#6-telegram-问题)
7. [Webhook 问题](#7-webhook-问题)
8. [性能问题](#8-性能问题)
9. [安全相关问题](#9-安全相关问题)
10. [诊断工具](#10-诊断工具)

---

## 1. 错误码参考

### 1.1 HTTP 状态码

| 状态码 | 含义 | 常见场景 | 解决方案 |
|--------|------|----------|----------|
| 200 | 成功 | 请求正常处理 | - |
| 201 | 已创建 | 资源创建成功 | - |
| 204 | 无内容 | 删除成功 | - |
| 400 | 请求参数错误 | 缺少必需字段、格式错误 | 检查请求体 |
| 401 | 未认证 | Token 无效或缺失 | 检查认证信息 |
| 403 | 无权限 | 权限不足 | 检查角色权限 |
| 404 | 资源不存在 | ID 错误或已删除 | 确认资源存在 |
| 409 | 资源冲突 | 重复创建、并发冲突 | 检查幂等键 |
| 422 | 请求无法处理 | 业务逻辑错误 | 检查业务规则 |
| 429 | 请求过于频繁 | 触发限流 | 降低请求频率 |
| 500 | 服务器内部错误 | 未捕获异常 | 查看日志 |
| 503 | 服务不可用 | 依赖服务故障 | 检查依赖服务 |

### 1.2 业务错误码

#### 系统级错误 (S001-S099)

| 错误码 | 说明 | 原因 | 解决方案 |
|--------|------|------|----------|
| `S001` | 配置加载失败 | 配置文件格式错误、路径不存在 | 检查配置文件语法 |
| `S002` | 数据库连接失败 | DSN 错误、网络不通、PostgreSQL 未启动 | 检查数据库配置和连接 |
| `S003` | 向量存储初始化失败 | SQLite 路径错误、权限不足 | 检查路径和权限 |
| `S004` | 端口占用 | 8080 或 8081 被占用 | 更换端口或停止占用进程 |
| `S005` | 配置文件权限错误 | 配置文件可读性不足 | 检查文件权限 |
| `S006` | 内存不足 | 系统内存耗尽 | 增加内存或优化配置 |
| `S007` | 磁盘空间不足 | 日志或数据目录满 | 清理磁盘或扩容 |
| `S008` | 服务启动超时 | 依赖服务响应慢 | 检查依赖服务状态 |
| `S009` | 证书加载失败 | TLS 证书路径错误或过期 | 检查证书配置 |
| `S010` | 插件加载失败 | 连接器配置错误 | 检查连接器配置 |

#### 会话错误 (E001-E099)

| 错误码 | 说明 | 原因 | 解决方案 |
|--------|------|------|----------|
| `E001` | 会话不存在 | session_id 错误或已过期 | 确认 session_id 正确 |
| `E002` | 会话状态非法 | 当前状态不允许此操作 | 查看会话状态机 |
| `E003` | 会话已关闭 | 会话已解决或失败 | 创建新会话 |
| `E004` | 会话创建失败 | 告警解析错误或数据库写入失败 | 检查告警格式和数据库 |
| `E005` | 会话更新冲突 | 乐观锁版本冲突 | 重试操作 |
| `E006` | 会话去重失败 | 幂等键冲突 | 检查去重配置 |
| `E007` | 会话超时 | 处理时间超过阈值 | 优化处理逻辑或增加超时 |
| `E008` | 会话关联失败 | 告警指纹匹配失败 | 检查告警标签 |
| `E009` | 会话归档失败 | 归档配置错误 | 检查归档设置 |
| `E010` | 会话恢复失败 | 恢复数据不完整 | 检查备份数据 |

#### 执行错误 (E101-E199)

| 错误码 | 说明 | 原因 | 解决方案 |
|--------|------|------|----------|
| `E101` | 执行请求不存在 | execution_id 错误 | 确认 execution_id |
| `E102` | 审批超时 | 超过审批时限 | 重新发起执行请求 |
| `E103` | 命令未授权 | 命中黑名单或策略拒绝 | 检查授权策略 |
| `E104` | SSH 连接失败 | 网络不通、主机不可达、密钥错误 | 检查 SSH 配置 |
| `E105` | SSH 认证失败 | 密钥错误或权限不足 | 检查 SSH 密钥和授权 |
| `E106` | 命令执行超时 | 命令执行时间超过阈值 | 增加超时时间或优化命令 |
| `E107` | 命令执行失败 | 命令返回非零退出码 | 检查命令语法和目标主机状态 |
| `E108` | 输出写入失败 | 磁盘满或权限不足 | 检查磁盘空间和权限 |
| `E109` | 输出读取失败 | 文件不存在或权限不足 | 检查输出文件 |
| `E110` | 主机不在白名单 | 目标主机未授权 | 更新 SSH 白名单配置 |
| `E111` | 命令被拦截 | 包含危险片段 | 检查命令内容 |
| `E112` | 审批被拒绝 | 审批人拒绝执行 | 修改命令后重新申请 |
| `E113` | 双人审批未完成 | Critical 级别需双人审批 | 等待第二审批人 |
| `E114` | 自审批被禁止 | prohibit_self_approval 启用 | 转交其他审批人 |
| `E115` | 审批路由失败 | 未找到审批人 | 检查审批路由配置 |
| `E116` | 执行被中断 | 服务重启或手动取消 | 重新发起执行 |

#### AI 诊断错误 (E201-E299)

| 错误码 | 说明 | 原因 | 解决方案 |
|--------|------|------|----------|
| `E201` | 模型调用失败 | 模型网关不可达、API Key 错误 | 检查模型配置 |
| `E202` | 模型响应超时 | 模型处理时间过长 | 增加超时时间或降级处理 |
| `E203` | 模型响应格式错误 | 返回非预期格式 | 检查 prompt 配置 |
| `E204` | 脱敏失败 | 敏感信息检测错误 | 检查脱敏配置 |
| `E205` | 知识检索失败 | 向量库查询错误 | 检查向量库状态 |
| `E206` | 上下文组装失败 | 会话数据不完整 | 检查会话数据 |
| `E207` | 模型配额耗尽 | API 调用次数限制 | 等待配额恢复或切换模型 |
| `E208` | 模型内容过滤 | 内容触发安全策略 | 调整 prompt 或手动处理 |
| `E209` | 降级处理失败 | 本地模型也失败 | 检查本地模型配置 |
| `E210` | Provider 切换失败 | 备用 Provider 不可用 | 检查 Provider 配置 |

#### Telegram 错误 (E301-E399)

| 错误码 | 说明 | 原因 | 解决方案 |
|--------|------|------|----------|
| `E301` | Telegram API 调用失败 | Token 错误、网络问题 | 检查 Token 和网络 |
| `E302` | Webhook 设置失败 | URL 不可达、Secret 错误 | 检查 Webhook 配置 |
| `E303` | 消息发送失败 | 用户已屏蔽、消息过长 | 检查用户状态和消息长度 |
| `E304` | 回调处理失败 | 回调数据格式错误 | 检查回调数据 |
| `E305` | Chat ID 无效 | 用户未与 Bot 交互 | 让用户先发送 /start |
| `E306` | 轮询超时 | 长时间未收到消息 | 检查网络连接 |
| `E307` | 消息编辑失败 | 消息不存在或已过期 | 检查消息 ID |
| `E308` | 文件上传失败 | 文件过大或格式不支持 | 检查文件大小和格式 |
| `E309` | 键盘设置失败 | 按钮数据格式错误 | 检查键盘配置 |
| `E310` | Telegram 限流 | 请求频率过高 | 降低发送频率 |

#### Webhook 错误 (E401-E499)

| 错误码 | 说明 | 原因 | 解决方案 |
|--------|------|------|----------|
| `E401` | Webhook 签名验证失败 | Secret 不匹配 | 检查 Secret 配置 |
| `E402` | Webhook 解析失败 | JSON 格式错误 | 检查请求体格式 |
| `E403` | Webhook 重复 | 幂等键已存在 | 检查去重配置 |
| `E404` | 告警来源不支持 | 未知的 source 字段 | 检查告警格式 |
| `E405` | 告警指纹生成失败 | 标签数据异常 | 检查告警标签 |
| `E406` | VMAlert 格式错误 | 不符合预期格式 | 检查 VMAlert 配置 |
| `E407` | Alertmanager 格式错误 | V2 API 格式错误 | 检查 Alertmanager 版本 |
| `E408` | Webhook 处理超时 | 处理时间过长 | 优化处理逻辑 |
| `E409` | Webhook 队列满 | 并发请求过多 | 增加队列大小或限流 |
| `E410` | Webhook 重试耗尽 | 多次重试失败 | 检查目标服务状态 |

#### 配置错误 (E501-E599)

| 错误码 | 说明 | 原因 | 解决方案 |
|--------|------|------|----------|
| `E501` | 配置文件不存在 | 路径错误或文件缺失 | 检查配置文件路径 |
| `E502` | YAML 解析失败 | 语法错误 | 检查 YAML 语法 |
| `E503` | 配置验证失败 | 值不符合规范 | 检查配置值 |
| `E504` | 配置热重载失败 | 运行时加载错误 | 检查配置文件并手动重启 |
| `E505` | 授权策略配置错误 | 策略语法错误 | 检查授权配置文件 |
| `E506` | 审批路由配置错误 | 路由规则错误 | 检查审批配置文件 |
| `E507` | Provider 配置错误 | Provider 定义不完整 | 检查 Provider 配置 |
| `E508` | SSH 配置错误 | 密钥路径错误或权限不足 | 检查 SSH 配置 |
| `E509` | 脱敏配置错误 | 正则表达式错误 | 检查脱敏配置 |
| `E510` | 环境变量解析错误 | 格式错误 | 检查环境变量值 |

### 1.3 错误响应格式

```json
{
  "error": {
    "code": "E104",
    "message": "SSH connection failed",
    "details": {
      "execution_id": "550e8400-e29b-41d4-a716-446655440000",
      "target_host": "prod-web-01",
      "error": "connection refused",
      "suggestions": [
        "Check if the target host is reachable",
        "Verify SSH service is running on the target",
        "Check firewall rules"
      ]
    },
    "trace_id": "abc123-def456",
    "timestamp": "2026-03-13T10:00:00Z"
  }
}
```

---

## 2. 启动故障

### 2.1 服务无法启动

#### 症状：启动后立即退出

**排查步骤**：

1. 查看详细日志
   ```bash
   # Docker
   docker logs tars --tail 100

   # Systemd
   sudo journalctl -u tars -n 100

   # 直接运行
   ./tars 2>&1 | tee tars.log
   ```

2. 检查配置文件语法
   ```bash
   # YAML 语法检查
   python3 -c "import yaml; yaml.safe_load(open('configs/tars.yaml'))"

   # 或使用 yamllint
   yamllint configs/*.yaml
   ```

3. 检查必需环境变量
   ```bash
   # 检查必需变量
   env | grep TARS_POSTGRES_DSN
   env | grep TARS_MODEL_BASE_URL
   ```

**常见原因和解决方案**：

| 原因 | 错误信息 | 解决方案 |
|------|----------|----------|
| PostgreSQL 未启动 | `connection refused` | 启动 PostgreSQL 服务 |
| DSN 格式错误 | `invalid connection string` | 检查 DSN 格式 |
| 端口被占用 | `address already in use` | 更换端口或停止占用进程 |
| 权限不足 | `permission denied` | 使用 sudo 或修复权限 |
| 配置文件不存在 | `no such file or directory` | 创建配置文件 |

### 2.2 启动超时

**排查步骤**：

```bash
# 检查依赖服务健康状态
pg_isready -h localhost -p 5432
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz

# 检查数据库连接超时
psql "$TARS_POSTGRES_DSN" -c "SELECT 1"

# 检查模型网关连通性
curl -H "Authorization: Bearer $TARS_MODEL_API_KEY" \
  "$TARS_MODEL_BASE_URL/models"
```

**解决方案**：

1. 增加启动超时时间
2. 配置健康检查延迟
3. 使用 Docker 的 `depends_on` 确保依赖先启动

---

## 3. 数据库问题

### 3.1 数据库连接失败

#### 症状：日志显示 `connection refused` 或 `timeout`

**排查步骤**：

```bash
# 1. 检查 PostgreSQL 服务状态
sudo systemctl status postgresql

# 2. 测试连接
psql "postgres://user:pass@localhost:5432/tars" -c "SELECT 1"

# 3. 检查端口监听
netstat -tlnp | grep 5432
ss -tlnp | grep 5432

# 4. 检查防火墙
sudo iptables -L | grep 5432
sudo ufw status

# 5. 检查连接数
psql -c "SELECT count(*) FROM pg_stat_activity;"
```

**解决方案**：

| 原因 | 解决方案 |
|------|----------|
| PostgreSQL 未启动 | `sudo systemctl start postgresql` |
| 防火墙阻止 | 开放 5432 端口 |
| 连接数超限 | 增加 `max_connections` |
| SSL 模式不匹配 | 调整 `sslmode` 参数 |
| 用户权限不足 | 授予数据库权限 |

### 3.2 数据库性能问题

**排查步骤**：

```sql
-- 检查慢查询
SELECT query, mean_exec_time, calls, rows
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- 检查锁
SELECT * FROM pg_locks WHERE NOT granted;

-- 检查长事务
SELECT * FROM pg_stat_activity
WHERE state = 'idle in transaction'
AND xact_start < NOW() - INTERVAL '5 minutes';

-- 检查表大小
SELECT schemaname, tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

**解决方案**：

1. 添加缺失的索引
2. 优化慢查询
3. 定期清理过期数据
4. 调整 PostgreSQL 配置参数

### 3.3 数据不一致

**排查步骤**：

```sql
-- 检查孤立记录
SELECT * FROM execution_requests
WHERE session_id NOT IN (SELECT id FROM alert_sessions);

-- 检查重复指纹
SELECT fingerprint, COUNT(*)
FROM alert_events
GROUP BY fingerprint
HAVING COUNT(*) > 1;

-- 检查状态不一致
SELECT * FROM alert_sessions
WHERE status = 'executing'
AND id NOT IN (SELECT session_id FROM execution_requests WHERE status = 'executing');
```

**修复方法**：

```sql
-- 删除孤立记录（谨慎操作）
BEGIN;
DELETE FROM execution_requests
WHERE session_id NOT IN (SELECT id FROM alert_sessions);
COMMIT;

-- 修复会话状态
UPDATE alert_sessions
SET status = 'open'
WHERE status = 'executing'
AND updated_at < NOW() - INTERVAL '1 hour';
```

---

## 4. AI 诊断问题

### 4.1 模型调用失败

#### 症状：诊断消息显示 "AI 分析暂不可用"

**排查步骤**：

```bash
# 1. 检查模型网关连通性
curl -v "${TARS_MODEL_BASE_URL}/models" \
  -H "Authorization: Bearer ${TARS_MODEL_API_KEY}"

# 2. 检查 Provider 状态
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/config/providers/check \
  -H "Content-Type: application/json" \
  -d '{"provider_id": "primary"}'

# 3. 查看模型相关日志
docker logs tars 2>&1 | grep -i "model\|reasoning"
```

**常见原因和解决方案**：

| 原因 | 解决方案 |
|------|----------|
| API Key 无效 | 检查并更新 API Key |
| 网络不通 | 检查防火墙和代理设置 |
| 模型配额耗尽 | 等待配额恢复或切换模型 |
| 请求格式错误 | 检查 prompt 配置 |
| 超时 | 增加 `TARS_MODEL_TIMEOUT` |

### 4.2 诊断质量差

**排查步骤**：

```bash
# 查看脱敏后的上下文
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/sessions/{session_id} | jq .desense_map

# 检查知识检索结果
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/knowledge?q={query}
```

**优化建议**：

1. 优化 Reasoning Prompt
2. 检查脱敏是否过度
3. 增加知识库文档
4. 调整模型参数（temperature、max_tokens）

### 4.3 脱敏问题

**排查步骤**：

```bash
# 测试脱敏配置
curl -X POST http://localhost:8081/api/v1/config/desensitization/test \
  -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"content": "password=secret123, host=prod-web-01"}'
```

**常见问题**：

- 脱敏不足：增加敏感 key 或正则表达式
- 脱敏过度：调整脱敏规则或使用本地 LLM 辅助
- 回水失败：检查 `desense_map` 是否完整

---

## 5. 执行问题

### 5.1 SSH 连接失败

#### 症状：执行状态显示 `failed`，错误为 `connection refused` 或 `timeout`

**排查步骤**：

```bash
# 1. 检查目标主机可达性
ping prod-web-01
nc -zv prod-web-01 22

# 2. 手动测试 SSH 连接
ssh -i /etc/tars/id_rsa \
  -o ConnectTimeout=10 \
  -o StrictHostKeyChecking=no \
  tars@prod-web-01 "hostname"

# 3. 检查 SSH 密钥权限
ls -la /etc/tars/id_rsa
ssh-keygen -l -f /etc/tars/id_rsa

# 4. 检查目标主机 SSH 服务
ssh -v tars@prod-web-01

# 5. 检查 authorized_keys
cat ~/.ssh/authorized_keys | grep "tars@"
```

**解决方案**：

| 原因 | 解决方案 |
|------|----------|
| 主机不可达 | 检查网络连接 |
| SSH 服务未启动 | 启动 sshd 服务 |
| 密钥错误 | 重新分发公钥 |
| 权限错误 | 修复密钥权限为 600 |
| 主机不在白名单 | 更新 `TARS_SSH_ALLOWED_HOSTS` |

### 5.2 命令执行失败

**排查步骤**：

```bash
# 1. 检查命令语法
ssh user@host "echo 'test'"  # 简单命令测试

# 2. 检查命令权限
ssh user@host "which command"

# 3. 查看详细错误
ssh user@host "command 2>&1"  # 捕获 stderr

# 4. 检查 sudo 权限（如需要）
ssh user@host "sudo -l"
```

**常见原因**：

- 命令不存在
- 权限不足
- 环境变量缺失
- 工作目录错误
- 依赖文件不存在

### 5.3 输出截断

**症状**：执行输出被截断

**排查步骤**：

```bash
# 检查输出大小
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/executions/{id} | jq .output_bytes, .output_truncated

# 检查输出目录
ls -lh /data/tars/output/
du -sh /data/tars/output/
```

**解决方案**：

1. 增加 `TARS_EXECUTION_OUTPUT_MAX_PERSISTED_BYTES`
2. 优化命令输出（使用 `head` 等）
3. 增加 chunk 大小

---

## 6. Telegram 问题

### 6.1 收不到消息

**排查步骤**：

```bash
# 1. 检查 Bot Token
curl "https://api.telegram.org/bot${TARS_TELEGRAM_BOT_TOKEN}/getMe"

# 2. 检查 Webhook 配置
curl "https://api.telegram.org/bot${TARS_TELEGRAM_BOT_TOKEN}/getWebhookInfo"

# 3. 测试发送消息
curl -X POST \
  "https://api.telegram.org/bot${TARS_TELEGRAM_BOT_TOKEN}/sendMessage" \
  -d "chat_id=${CHAT_ID}&text=Test message"

# 4. 检查 Outbox
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/outbox?status=failed
```

**解决方案**：

| 原因 | 解决方案 |
|------|----------|
| Bot Token 错误 | 更新正确的 Token |
| Chat ID 错误 | 获取正确的 Chat ID |
| Webhook URL 不可达 | 确保公网可访问 |
| 用户已屏蔽 Bot | 让用户检查隐私设置 |
| 消息被 Telegram 限制 | 降低发送频率 |

### 6.2 回调无响应

**症状**：点击 Telegram 按钮无反应

**排查步骤**：

```bash
# 1. 检查 Webhook 接收日志
docker logs tars 2>&1 | grep -i telegram

# 2. 检查回调处理
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/audit?action=telegram_callback

# 3. 检查会话状态
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/sessions/{id}
```

**常见问题**：

- Webhook 处理超时
- 会话状态已变更
- 按钮已过期

---

## 7. Webhook 问题

### 7.1 VMAlert Webhook 失败

**排查步骤**：

```bash
# 1. 检查 VMAlert 日志
docker logs vmalert 2>&1 | grep webhook

# 2. 测试 Webhook 端点
curl -X POST http://tars:8080/api/v1/webhooks/vmalert \
  -H "Content-Type: application/json" \
  -H "X-Tars-Secret: ${TARS_VMALERT_WEBHOOK_SECRET}" \
  -d '{"receiver":"test","status":"firing","alerts":[],"version":"1"}'

# 3. 检查幂等键
psql -d tars -c "SELECT * FROM idempotency_keys WHERE scope='webhook' ORDER BY last_seen_at DESC LIMIT 10;"
```

**解决方案**：

1. 确保 `X-Tars-Secret` 正确
2. 检查告警格式
3. 查看幂等键是否冲突

### 7.2 Webhook 重复告警

**原因**：
- 幂等键生成错误
- VMAlert 分组配置不当
- 告警指纹冲突

**解决方案**：

```yaml
# VMAlert 配置优化
route:
  group_by: ['alertname', 'service', 'instance']
  group_wait: 10s
  group_interval: 30s
  repeat_interval: 4h  # 增加重复间隔
```

---

## 8. 性能问题

### 8.1 响应慢

**诊断步骤**：

```bash
# 1. 检查资源使用
top
htop
free -m
df -h

# 2. 检查 Go runtime
curl http://localhost:8080/metrics | grep go_

# 3. 检查数据库慢查询
# （见 3.2 节）

# 4. 检查 Goroutine 数量
curl http://localhost:8080/metrics | grep go_goroutines
```

**优化建议**：

1. 增加 CPU/内存资源
2. 优化数据库查询
3. 启用连接池
4. 增加缓存
5. 水平扩展

### 8.2 内存泄漏

**诊断步骤**：

```bash
# 1. 监控内存使用
watch -n 1 'curl -s http://localhost:8080/metrics | grep "go_memstats_alloc_bytes"'

# 2. 生成 Heap Profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# 3. 查看 Goroutine
curl http://localhost:8080/debug/pprof/goroutine?debug=1
```

---

## 9. 安全相关问题

### 9.1 未授权访问

**症状**：日志显示 `permission denied` 或 `unauthorized`

**排查步骤**：

```bash
# 1. 检查审计日志
psql -d tars -c "SELECT * FROM audit_logs WHERE action LIKE '%denied%' ORDER BY created_at DESC LIMIT 10;"

# 2. 检查 Token 配置
env | grep TARS_OPS_API_TOKEN

# 3. 检查授权策略
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/config/authorization
```

### 9.2 可疑活动

**症状**：大量失败登录或异常命令

**排查步骤**：

```bash
# 1. 统计失败登录
psql -d tars -c "SELECT actor_id, COUNT(*) FROM audit_logs WHERE action='login_failed' GROUP BY actor_id;"

# 2. 检查被拦截的命令
psql -d tars -c "SELECT * FROM audit_logs WHERE action='command_blocked' ORDER BY created_at DESC LIMIT 10;"

# 3. 检查异常 IP
cat /var/log/nginx/access.log | awk '{print $1}' | sort | uniq -c | sort -rn | head -20
```

---

## 10. 诊断工具

### 10.1 健康检查脚本

```bash
#!/bin/bash
# tars-health-check.sh

echo "=== TARS Health Check ==="

# 1. HTTP 健康检查
echo -n "HTTP Health: "
if curl -fsS http://localhost:8080/healthz > /dev/null; then
    echo "OK"
else
    echo "FAILED"
fi

# 2. 就绪检查
echo -n "Readiness: "
if curl -fsS http://localhost:8080/readyz > /dev/null; then
    echo "OK"
else
    echo "FAILED"
fi

# 3. 数据库连接
echo -n "Database: "
if pg_isready -q; then
    echo "OK"
else
    echo "FAILED"
fi

# 4. 内存使用
echo "Memory Usage:"
free -h | grep Mem

# 5. 磁盘使用
echo "Disk Usage:"
df -h | grep -E 'Filesystem|/data'

# 6. 活跃会话
echo -n "Active Sessions: "
curl -fsS http://localhost:8081/api/v1/sessions?status=open 2>/dev/null | jq '.items | length'

echo "=== Check Complete ==="
```

### 10.2 日志分析工具

```bash
# 实时错误监控
docker logs -f tars 2>&1 | grep -i error

# 统计错误类型
docker logs tars 2>&1 | grep "error" | awk -F': ' '{print $2}' | sort | uniq -c | sort -rn

# 查找特定会话日志
docker logs tars 2>&1 | grep "session_id=xxx"

# 性能分析
docker logs tars 2>&1 | grep "duration" | awk '{print $NF}' | sort -n | tail -20
```

### 10.3 数据库诊断脚本

```bash
#!/bin/bash
# pg-diagnostics.sh

echo "=== PostgreSQL Diagnostics ==="

# 连接数
echo "Connection Count:"
psql -d tars -c "SELECT count(*), state FROM pg_stat_activity GROUP BY state;"

# 数据库大小
echo "Database Size:"
psql -d tars -c "SELECT pg_size_pretty(pg_database_size('tars'));"

# 表大小
echo "Table Sizes:"
psql -d tars -c "
SELECT schemaname, tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 10;
"

# 慢查询
echo "Slow Queries (Top 5):"
psql -d tars -c "
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 5;
"

echo "=== Diagnostics Complete ==="
```

### 10.4 调试模式

```bash
# 启用调试日志
export TARS_LOG_LEVEL=DEBUG

# 启用性能分析
export TARS_ENABLE_PPROF=true

# 启动服务
./tars

# 访问 pprof
curl http://localhost:8080/debug/pprof/
curl http://localhost:8080/debug/pprof/heap
curl http://localhost:8080/debug/pprof/profile?seconds=30
```

---

## 11. 获取帮助

### 11.1 报告问题

报告问题时请提供：

1. **环境信息**
   - TARS 版本
   - 部署方式（Docker/二进制/Kubernetes）
   - 操作系统版本

2. **配置文件**（脱敏后）

3. **日志片段**
   - 相关错误日志
   - 时间范围

4. **复现步骤**

5. **已尝试的解决方案**

### 11.2 社区支持

- GitHub Issues: <repository-url>/issues
- Discussions: <repository-url>/discussions
- 文档: <docs-url>

---

*本文档适用于 TARS MVP 版本，错误码和排查方法可能会在未来版本中调整。*
