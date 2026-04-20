# TARS 部署手册

> **版本**: v1.0
> **适用版本**: TARS MVP (Phase 1)
> **最后更新**: 2026-03-13

---

## 1. 环境要求

### 1.1 系统要求

| 项目 | 最低要求 | 推荐配置 |
|------|----------|----------|
| CPU | 2 核 | 4 核及以上 |
| 内存 | 4 GB | 8 GB 及以上 |
| 磁盘 | 20 GB | 100 GB SSD 及以上 |
| 操作系统 | Linux (Ubuntu 20.04+) | Ubuntu 22.04 LTS |
| Go 版本 | 1.21+ | 1.22+ |

### 1.2 依赖服务

| 服务 | 版本 | 用途 | 是否必需 |
|------|------|------|----------|
| PostgreSQL | 14+ | 业务数据、审计日志存储 | 是 |
| VictoriaMetrics | 1.90+ | 指标查询 | 否 (但推荐) |
| VMAlert | 1.90+ | 告警触发 | 否 (但推荐) |
| Telegram Bot | - | 消息通知渠道 | 否 (但推荐) |

### 1.3 网络要求

| 方向 | 端口 | 用途 |
|------|------|------|
| 入站 | 8080 | 主服务 HTTP 接口 |
| 入站 | 8081 | Ops API (可选) |
| 出站 | 5432 | PostgreSQL |
| 出站 | 22 | SSH 目标主机 |
| 出站 | 443 | Telegram API、模型网关 |

---

## 2. 安装方式

### 2.1 Docker Compose 部署 (推荐一站式方案)

#### 2.1.1 环境准备

```bash
# 安装 Docker 和 Docker Compose
# Ubuntu
sudo apt-get update
sudo apt-get install -y docker.io docker-compose-plugin

# 验证安装
docker --version
docker compose version
```

#### 2.1.2 下载部署文件

```bash
# 克隆代码仓库
git clone <repository-url>
cd TARS
```

#### 2.1.3 配置环境变量

在 `deploy/docker` 目录下创建 `.env` 文件：

```bash
cat > deploy/docker/.env << EOF
TARS_HOST=$(hostname -I | awk '{print $1}')
EOF
```

#### 2.1.4 启动服务

```bash
cd deploy/docker
docker compose up --build -d

# 查看服务状态
docker compose ps
```

> **注意**: 现在 Docker Compose 是默认优先的部署路径，会自动按宿主机架构构建 TARS 镜像，因此 `linux/amd64` 和 `linux/arm64` 都可以使用同一套命令。
>
> 如果要提前构建镜像，当前宿主机架构可用 `make docker-build`；如需发布多架构镜像，可用 `make docker-buildx DOCKER_IMAGE=<registry>/tars:tag DOCKER_BUILDX_ARGS=--push`。
>
> TARS 在启动时会自动初始化数据库表结构，不再需要手动执行 migration 脚本。


### 2.2 二进制部署

#### 2.2.1 编译二进制

```bash
# 克隆代码
git clone <repository-url>
cd TARS

# 为当前宿主机对应的 Linux 架构编译
make build-linux

# 如需通用文件名，可自行复制/重命名
cp ./bin/tars-linux-$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/; s/arm64/arm64/; s/amd64/amd64/') ./tars-server

# 显式交叉编译 (可选)
make build-linux-amd64
make build-linux-arm64
```

#### 2.2.2 安装二进制

```bash
# 创建目录
sudo mkdir -p /opt/tars/bin
sudo mkdir -p /opt/tars/configs
sudo mkdir -p /opt/tars/data
sudo mkdir -p /opt/tars/logs

# 复制二进制
sudo cp tars-server /opt/tars/bin/
sudo chmod +x /opt/tars/bin/tars-server

# 创建 systemd 服务
sudo cat > /etc/systemd/system/tars.service << 'EOF'
[Unit]
Description=TARS AIOps Platform
After=network.target postgresql.service

[Service]
Type=simple
User=tars
Group=tars
WorkingDirectory=/opt/tars
ExecStart=/opt/tars/bin/tars-server
Restart=on-failure
RestartSec=5

# 环境变量
Environment="TARS_SERVER_LISTEN=:8080"
Environment="TARS_POSTGRES_DSN=postgres://tars:tars@localhost:5432/tars?sslmode=disable"
Environment="TARS_OPS_API_ENABLED=true"
Environment="TARS_OPS_API_LISTEN=127.0.0.1:8081"
Environment="TARS_OPS_API_TOKEN=your-secure-token"

[Install]
WantedBy=multi-user.target
EOF

# 创建用户
sudo useradd -r -s /bin/false tars
sudo chown -R tars:tars /opt/tars

# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable tars
sudo systemctl start tars
sudo systemctl status tars
```

### 2.3 源码编译

```bash
# 要求 Go 1.21+
go version

# 克隆代码
git clone <repository-url>
cd TARS

# 下载依赖
go mod download

# 运行测试
go test ./...

# 编译
go build -o tars-server ./cmd/tars

# 开发模式运行
go run ./cmd/tars
```

---

## 3. 配置详解

### 3.1 环境变量完整列表

#### 3.1.1 服务器配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_SERVER_LISTEN` | `:8080` | 主服务监听地址 |
| `TARS_SERVER_PUBLIC_BASE_URL` | `https://tars.example.com` | 公网访问地址 |
| `TARS_WEB_DIST_DIR` | `./web/dist` | Web Console 静态文件目录 |
| `TARS_LOG_LEVEL` | `INFO` | 日志级别 (DEBUG/INFO/WARN/ERROR) |

#### 3.1.2 数据库配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_POSTGRES_DSN` | - | PostgreSQL 连接字符串 |
| `TARS_VECTOR_SQLITE_PATH` | - | SQLite 向量数据库路径 |

#### 3.1.3 Ops API 配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_OPS_API_ENABLED` | `false` | 是否启用 Ops API |
| `TARS_OPS_API_LISTEN` | `127.0.0.1:8081` | Ops API 监听地址 |
| `TARS_OPS_API_TOKEN` | - | Ops API 认证 Token |

#### 3.1.4 Telegram 配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_TELEGRAM_BOT_TOKEN` | - | Bot Token |
| `TARS_TELEGRAM_WEBHOOK_SECRET` | - | Webhook 签名密钥 |
| `TARS_TELEGRAM_BASE_URL` | `https://api.telegram.org` | API 基础地址 |
| `TARS_TELEGRAM_POLLING_ENABLED` | `false` | 是否使用轮询模式 |
| `TARS_TELEGRAM_POLL_TIMEOUT` | `30s` | 轮询超时时间 |
| `TARS_TELEGRAM_POLL_INTERVAL` | `1s` | 轮询间隔 |

#### 3.1.5 VMAlert 配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_VMALERT_WEBHOOK_SECRET` | - | Webhook 签名密钥 |

#### 3.1.6 模型配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_MODEL_PROTOCOL` | `openai_compatible` | 模型协议 |
| `TARS_MODEL_BASE_URL` | - | 模型网关地址 |
| `TARS_MODEL_API_KEY` | - | API Key |
| `TARS_MODEL_NAME` | `gpt-4o-mini` | 模型名称 |
| `TARS_MODEL_TIMEOUT` | `30s` | 请求超时 |

#### 3.1.7 VictoriaMetrics 配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_VM_BASE_URL` | - | VM 查询地址 |
| `TARS_VM_TIMEOUT` | `15s` | 查询超时 |

#### 3.1.8 SSH 配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_SSH_USER` | - | SSH 用户名 |
| `TARS_SSH_PRIVATE_KEY_PATH` | - | 私钥文件路径 |
| `TARS_SSH_CONNECT_TIMEOUT` | `10s` | 连接超时 |
| `TARS_SSH_COMMAND_TIMEOUT` | `5m` | 命令执行超时 |
| `TARS_SSH_ALLOWED_HOSTS` | - | 允许的主机列表 (逗号分隔) |
| `TARS_SSH_ALLOWED_COMMAND_PREFIXES` | - | 允许的命令前缀 |
| `TARS_SSH_BLOCKED_COMMAND_FRAGMENTS` | - | 禁止的命令片段 |
| `TARS_SSH_DISABLE_HOST_KEY_CHECKING` | `false` | 是否禁用主机密钥检查 |

#### 3.1.9 执行输出配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_EXECUTION_OUTPUT_SPOOL_DIR` | `./data/execution_output` | 输出存储目录 |
| `TARS_EXECUTION_OUTPUT_MAX_PERSISTED_BYTES` | `262144` | 最大存储字节数 (256KB) |
| `TARS_EXECUTION_OUTPUT_CHUNK_BYTES` | `16384` | 分块大小 (16KB) |
| `TARS_EXECUTION_OUTPUT_RETENTION` | `168h` | 保留时间 (7天) |

#### 3.1.10 审批配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_APPROVAL_TIMEOUT` | `15m` | 审批超时时间 |
| `TARS_APPROVALS_CONFIG_PATH` | - | 审批配置文件路径 |

#### 3.1.11 功能开关

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_ROLLOUT_MODE` | `diagnosis_only` | 部署模式 |
| `TARS_FEATURES_DIAGNOSIS_ENABLED` | `true` | 启用诊断 |
| `TARS_FEATURES_APPROVAL_ENABLED` | `false` | 启用审批 |
| `TARS_FEATURES_EXECUTION_ENABLED` | `false` | 启用执行 |
| `TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED` | `false` | 启用知识沉淀 |

#### 3.1.12 GC 配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TARS_GC_ENABLED` | `true` | 启用垃圾回收 |
| `TARS_GC_INTERVAL` | `1h` | GC 间隔 |
| `TARS_EXECUTION_OUTPUT_RETENTION` | `168h` | 执行输出保留时间 |

### 3.2 配置文件说明

#### 3.2.1 主配置文件 (`tars.yaml`)

```yaml
server:
  listen: ":8080"
  public_base_url: "https://tars.example.com"

postgres:
  dsn: "postgres://user:pass@localhost:5432/tars?sslmode=disable"

vector:
  sqlite_path: "./data/tars_vec.db"

model:
  protocol: "openai_compatible"
  base_url: "https://api.openai.com/v1"
  api_key: "${MODEL_API_KEY}"
  model: "gpt-4o-mini"
  timeout: "30s"

reasoning:
  prompts_config_path: "./configs/reasoning_prompts.yaml"
  desensitization_config_path: "./configs/desensitization.yaml"
  local_command_fallback_enabled: false

ops_api:
  enabled: true
  listen: "127.0.0.1:8081"
  bearer_token: "${OPS_API_TOKEN}"

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
  command_timeout: "300s"
  disable_host_key_checking: false
  allowed_hosts: ["192.168.1.0/24"]
  allowed_command_prefixes:
    - "hostname"
    - "uptime"
    - "systemctl status"
  blocked_command_fragments:
    - "rm -rf"
    - "mkfs"

execution_output:
  max_persisted_bytes: 262144
  chunk_bytes: 16384
  retention: "168h"
  spool_dir: "./data/execution_output"

approval:
  timeout: "15m"
  config_path: "./configs/approvals.yaml"

features:
  rollout_mode: "execution_beta"
  diagnosis_enabled: true
  approval_enabled: true
  execution_enabled: true
  knowledge_ingest_enabled: false
```

### 3.3 部署模式说明

| 模式 | 诊断 | 审批 | 执行 | 知识 |
|------|------|------|------|------|
| `diagnosis_only` | ✅ | ❌ | ❌ | ❌ |
| `approval_beta` | ✅ | ✅ | ❌ | ❌ |
| `execution_beta` | ✅ | ✅ | ✅ | ❌ |
| `knowledge_on` | ✅ | ✅ | ✅ | ✅ |

---

## 4. 外部系统集成

### 4.1 Telegram Bot 配置

#### 4.1.1 创建 Bot

1. 在 Telegram 中搜索 `@BotFather`
2. 发送 `/newbot` 命令
3. 按提示设置 Bot 名称和用户名
4. 保存获得的 Bot Token

#### 4.1.2 获取 Chat ID

```bash
# 发送消息给 Bot
# 然后访问 (替换 YOUR_BOT_TOKEN):
curl https://api.telegram.org/botYOUR_BOT_TOKEN/getUpdates

# 从响应中提取 chat.id
```

#### 4.1.3 配置 Webhook

```bash
# 设置 webhook (替换相关变量)
curl -X POST "https://api.telegram.org/bot${BOT_TOKEN}/setWebhook" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://tars.example.com/api/v1/channels/telegram/webhook",
    "secret_token": "your-webhook-secret"
  }'

# 验证 webhook 状态
curl "https://api.telegram.org/bot${BOT_TOKEN}/getWebhookInfo"
```

### 4.2 VictoriaMetrics 配置

#### 4.2.1 VMAlert 配置

编辑 `vmalert-rules.yml`：

```yaml
groups:
  - name: tars_alerts
    rules:
      - alert: HighCPUUsage
        expr: cpu_usage_percent > 80
        for: 5m
        labels:
          severity: warning
          service: "{{ $labels.service }}"
        annotations:
          summary: "High CPU usage detected"
          description: "CPU usage is above 80%"
```

#### 4.2.2 VMAlert 启动参数

```bash
vmalert \
  -rule=vmalert-rules.yml \
  -datasource.url=http://victoriametrics:8428 \
  -notifier.url=http://tars:8080/api/v1/webhooks/vmalert \
  -notifier.headers="X-Tars-Secret:your-vmalert-secret" \
  -external.url=http://vmalert:8880
```

### 4.3 模型网关配置

#### 4.3.1 OpenAI 兼容接口

```yaml
model:
  protocol: "openai_compatible"
  base_url: "https://api.openai.com/v1"
  api_key: "sk-..."
  model: "gpt-4o-mini"
```

#### 4.3.2 本地模型 (LM Studio)

```yaml
model:
  protocol: "openai_compatible"
  base_url: "http://localhost:1234/v1"
  api_key: "not-needed"
  model: "local-model"
```

#### 4.3.3 多 Provider 配置

使用 `providers.yaml`：

```yaml
providers:
  primary:
    provider_id: "primary-openrouter"
    model: "openai/gpt-4.1-mini"

  assist:
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

### 4.4 SSH 密钥配置

#### 4.4.1 生成密钥对

```bash
# 生成 ED25519 密钥 (推荐)
ssh-keygen -t ed25519 -f /etc/tars/id_ed25519 -C "tars@example.com"

# 或生成 RSA 密钥
ssh-keygen -t rsa -b 4096 -f /etc/tars/id_rsa -C "tars@example.com"
```

#### 4.4.2 分发公钥

```bash
# 将公钥复制到目标主机
ssh-copy-id -i /etc/tars/id_ed25519.pub user@target-host

# 或手动添加
# 在目标主机上:
echo "ssh-ed25519 AAAAC3NzaC... tars@example.com" >> ~/.ssh/authorized_keys
```

#### 4.4.3 验证连接

```bash
ssh -i /etc/tars/id_ed25519 -o ConnectTimeout=10 user@target-host "hostname"
```

---

## 5. 启动和验证

### 5.1 数据库初始化

#### 5.1.1 创建数据库

```bash
# 连接 PostgreSQL
psql -U postgres

# 创建数据库和用户
CREATE USER tars WITH PASSWORD 'tars';
CREATE DATABASE tars OWNER tars;
GRANT ALL PRIVILEGES ON DATABASE tars TO tars;

# 退出
\q
```

#### 5.1.2 执行迁移

```bash
# 执行初始化脚本
psql -U tars -d tars -f migrations/postgres/0001_init.sql
```

### 5.2 服务启动

#### 5.2.1 Docker Compose

```bash
# 启动
docker compose up -d

# 停止
docker compose down

# 重启
docker compose restart

# 查看日志
docker compose logs -f
```

#### 5.2.2 Systemd

```bash
# 启动
sudo systemctl start tars

# 停止
sudo systemctl stop tars

# 重启
sudo systemctl restart tars

# 查看状态
sudo systemctl status tars

# 查看日志
sudo journalctl -u tars -f
```

### 5.3 健康检查

#### 5.3.1 基础健康检查

```bash
# 健康检查端点
curl http://localhost:8080/healthz
# 预期输出: ok

# 就绪检查端点
curl http://localhost:8080/readyz
# 预期输出: ok

# 查看 metrics
curl http://localhost:8080/metrics
```

#### 5.3.2 组件健康检查

```bash
# 通过 Ops API 查看详细状态
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/dashboard/health
```

### 5.4 Smoke 测试

#### 5.4.1 发送测试告警

```bash
# 使用 curl 发送测试告警
curl -X POST http://localhost:8080/api/v1/smoke/alerts \
  -H "Content-Type: application/json" \
  -d '{
    "alert_name": "TestAlert",
    "severity": "warning",
    "target_host": "test-host",
    "description": "Smoke test alert"
  }'
```

#### 5.4.2 查看测试会话

```bash
# 列出会话
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/sessions

# 查看特定会话
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/sessions/{session_id}
```

#### 5.4.3 验证 Telegram 通知

1. 执行 Smoke 测试
2. 确认 Telegram 收到告警消息
3. 验证消息格式正确

---

## 6. 升级和回滚

### 6.1 升级步骤

#### 6.1.1 Docker Compose 升级

```bash
# 1. 备份数据
docker compose exec postgres pg_dump -U tars tars > backup_$(date +%Y%m%d).sql

# 2. 拉取新版本
docker compose pull

# 3. 停止服务
docker compose down

# 4. 启动新版本
docker compose up -d

# 5. 验证
curl http://localhost:8080/healthz
```

#### 6.1.2 二进制升级

```bash
# 1. 备份数据
pg_dump -U tars tars > backup_$(date +%Y%m%d).sql

# 2. 停止服务
sudo systemctl stop tars

# 3. 备份旧版本
sudo mv /opt/tars/bin/tars-server /opt/tars/bin/tars-server.backup

# 4. 部署新版本
sudo cp tars-server /opt/tars/bin/
sudo chmod +x /opt/tars/bin/tars-server

# 5. 启动服务
sudo systemctl start tars

# 6. 验证
sudo systemctl status tars
curl http://localhost:8080/healthz
```

### 6.2 回滚步骤

#### 6.2.1 Docker Compose 回滚

```bash
# 1. 停止服务
docker compose down

# 2. 使用旧版本镜像
docker compose up -d

# 3. 如需恢复数据
docker compose exec postgres psql -U tars -d tars < backup_YYYYMMDD.sql
```

#### 6.2.2 二进制回滚

```bash
# 1. 停止服务
sudo systemctl stop tars

# 2. 恢复旧版本
sudo mv /opt/tars/bin/tars-server.backup /opt/tars/bin/tars-server

# 3. 启动服务
sudo systemctl start tars

# 4. 如需恢复数据
dropdb -U postgres tars
createdb -U postgres -O tars tars
psql -U tars -d tars < backup_YYYYMMDD.sql
```

### 6.3 数据库迁移

```bash
# 查看当前版本
psql -U tars -d tars -c "SELECT * FROM schema_migrations;"

# 执行迁移 (如果有新的迁移文件)
psql -U tars -d tars -f migrations/postgres/0002_xxx.sql
```

---

## 7. 故障排查

### 7.1 启动失败

**问题**: 服务无法启动

**排查步骤**:
1. 检查日志: `docker compose logs` 或 `journalctl -u tars`
2. 验证配置文件格式
3. 检查端口占用: `netstat -tlnp | grep 8080`
4. 确认数据库连接正常

### 7.2 数据库连接失败

**问题**: 无法连接 PostgreSQL

**排查步骤**:
1. 检查 PostgreSQL 服务状态
2. 验证连接字符串正确
3. 确认网络连通性: `telnet postgres-host 5432`
4. 检查防火墙规则
5. 确认用户权限正确

### 7.3 Telegram 通知失败

**问题**: 收不到 Telegram 消息

**排查步骤**:
1. 验证 Bot Token 正确
2. 检查 Webhook 配置: `curl https://api.telegram.org/bot${TOKEN}/getWebhookInfo`
3. 确认公网可访问 TARS 服务
4. 检查 webhook secret 匹配
5. 查看 TARS 日志中的错误信息

### 7.4 模型调用失败

**问题**: AI 诊断无法生成

**排查步骤**:
1. 验证模型网关可访问
2. 检查 API Key 有效
3. 确认模型名称正确
4. 检查网络连通性
5. 查看 TARS 日志中的错误详情

---

## 8. 参考链接

- [用户手册](./user-guide.md)
- [管理员手册](./admin-guide.md)
- [API 文档](../reference/api-reference.md)
- [产品需求文档](../../project/tars_prd.md)
- [技术设计文档](../../project/tars_technical_design.md)

---

*本文档适用于 TARS MVP 版本，部署时请根据实际情况调整配置。*
