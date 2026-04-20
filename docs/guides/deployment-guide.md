# TARS Deployment Guide

> **Version**: v1.0
> **Applicable Version**: TARS MVP (Phase 1)
> **Last Updated**: 2026-03-13

---

## 1. Environment Requirements

### 1.1 System Requirements

| Item | Minimum Requirement | Recommended Configuration |
|------|---------------------|---------------------------|
| CPU | 2 Cores | 4 Cores and above |
| Memory | 4 GB | 8 GB and above |
| Disk | 20 GB | 100 GB SSD and above |
| OS | Linux (Ubuntu 20.04+) | Ubuntu 22.04 LTS |
| Go Version | 1.21+ | 1.22+ |

### 1.2 Dependencies

| Service | Version | Purpose | Required |
|---------|---------|---------|----------|
| PostgreSQL | 14+ | Business data, audit log storage | Yes |
| VictoriaMetrics | 1.90+ | Metrics query | No (Recommended) |
| VMAlert | 1.90+ | Alert triggering | No (Recommended) |
| Telegram Bot | - | Notification channel | No (Recommended) |

### 1.3 Network Requirements

| Direction | Port | Purpose |
|-----------|------|---------|
| Inbound | 8080 | Main service HTTP API |
| Inbound | 8081 | Ops API (Optional) |
| Outbound | 5432 | PostgreSQL |
| Outbound | 22 | SSH Target hosts |
| Outbound | 443 | Telegram API, Model Gateway |

---

## 2. Installation Methods

### 2.1 Docker Compose Deployment (Recommended All-in-One)

#### 2.1.1 Environment Preparation

```bash
# Install Docker and Docker Compose
# Ubuntu
sudo apt-get update
sudo apt-get install -y docker.io docker-compose-plugin

# Verify installation
docker --version
docker compose version
```

#### 2.1.2 Download Deployment Files

```bash
# Clone the repository
git clone <repository-url>
cd TARS
```

#### 2.1.3 Configure Environment Variables

Create `.env` file in `deploy/docker`:

```bash
cat > deploy/docker/.env << EOF
TARS_HOST=$(hostname -I | awk '{print $1}')
EOF
```

#### 2.1.4 Start Services

```bash
cd deploy/docker
docker compose up --build -d

# Check service status
docker compose ps
```

> **Note**: Docker Compose is now the primary deployment path. It builds the TARS image for the host architecture automatically, so the same flow works on both `linux/amd64` and `linux/arm64`.
>
> For prebuilding images manually, use `make docker-build` for the current host architecture, or `make docker-buildx DOCKER_IMAGE=<registry>/tars:tag DOCKER_BUILDX_ARGS=--push` for a multi-arch publish.
>
> TARS will automatically initialize the database schema on startup. Manual migration is no longer required for the baseline setup.


### 2.2 Binary Deployment

#### 2.2.1 Compile Binary

```bash
# Clone code
git clone <repository-url>
cd TARS

# Compile for the current host's Linux architecture
make build-linux

# Rename if you want a generic local filename
cp ./bin/tars-linux-$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/; s/arm64/arm64/; s/amd64/amd64/') ./tars-server

# Explicit cross-compilation (Optional)
make build-linux-amd64
make build-linux-arm64
```

#### 2.2.2 Install Binary

```bash
# Create directories
sudo mkdir -p /opt/tars/bin
sudo mkdir -p /opt/tars/configs
sudo mkdir -p /opt/tars/data
sudo mkdir -p /opt/tars/logs

# Copy binary
sudo cp tars-server /opt/tars/bin/
sudo chmod +x /opt/tars/bin/tars-server

# Create systemd service
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

# Environment variables
Environment="TARS_SERVER_LISTEN=:8080"
Environment="TARS_POSTGRES_DSN=postgres://tars:tars@localhost:5432/tars?sslmode=disable"
Environment="TARS_OPS_API_ENABLED=true"
Environment="TARS_OPS_API_LISTEN=127.0.0.1:8081"
Environment="TARS_OPS_API_TOKEN=your-secure-token"

[Install]
WantedBy=multi-user.target
EOF

# Create user
sudo useradd -r -s /bin/false tars
sudo chown -R tars:tars /opt/tars

# Start service
sudo systemctl daemon-reload
sudo systemctl enable tars
sudo systemctl start tars
sudo systemctl status tars
```

### 2.3 Source Compilation

```bash
# Requires Go 1.21+
go version

# Clone code
git clone <repository-url>
cd TARS

# Download dependencies
go mod download

# Run tests
go test ./...

# Compile
go build -o tars-server ./cmd/tars

# Run in development mode
go run ./cmd/tars
```

---

## 3. Configuration Details

### 3.1 Complete List of Environment Variables

#### 3.1.1 Server Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_SERVER_LISTEN` | `:8080` | Main service listen address |
| `TARS_SERVER_PUBLIC_BASE_URL` | `https://tars.example.com` | Public access URL |
| `TARS_WEB_DIST_DIR` | `./web/dist` | Web Console static files directory |
| `TARS_LOG_LEVEL` | `INFO` | Log level (DEBUG/INFO/WARN/ERROR) |

#### 3.1.2 Database Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_POSTGRES_DSN` | - | PostgreSQL connection string |
| `TARS_VECTOR_SQLITE_PATH` | - | SQLite vector database path |

#### 3.1.3 Ops API Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_OPS_API_ENABLED` | `false` | Whether to enable Ops API |
| `TARS_OPS_API_LISTEN` | `127.0.0.1:8081` | Ops API listen address |
| `TARS_OPS_API_TOKEN` | - | Ops API authentication token |

#### 3.1.4 Telegram Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_TELEGRAM_BOT_TOKEN` | - | Bot Token |
| `TARS_TELEGRAM_WEBHOOK_SECRET` | - | Webhook signature secret |
| `TARS_TELEGRAM_BASE_URL` | `https://api.telegram.org` | API base URL |
| `TARS_TELEGRAM_POLLING_ENABLED` | `false` | Whether to use polling mode |
| `TARS_TELEGRAM_POLL_TIMEOUT` | `30s` | Polling timeout |
| `TARS_TELEGRAM_POLL_INTERVAL` | `1s` | Polling interval |

#### 3.1.5 VMAlert Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_VMALERT_WEBHOOK_SECRET` | - | Webhook signature secret |

#### 3.1.6 Model Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_MODEL_PROTOCOL` | `openai_compatible` | Model protocol |
| `TARS_MODEL_BASE_URL` | - | Model gateway address |
| `TARS_MODEL_API_KEY` | - | API Key |
| `TARS_MODEL_NAME` | `gpt-4o-mini` | Model name |
| `TARS_MODEL_TIMEOUT` | `30s` | Request timeout |

#### 3.1.7 VictoriaMetrics Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_VM_BASE_URL` | - | VM query URL |
| `TARS_VM_TIMEOUT` | `15s` | Query timeout |

#### 3.1.8 SSH Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_SSH_USER` | - | SSH username |
| `TARS_SSH_PRIVATE_KEY_PATH` | - | Private key file path |
| `TARS_SSH_CONNECT_TIMEOUT` | `10s` | Connection timeout |
| `TARS_SSH_COMMAND_TIMEOUT` | `5m` | Command execution timeout |
| `TARS_SSH_ALLOWED_HOSTS` | - | Allowed hosts list (comma separated) |
| `TARS_SSH_ALLOWED_COMMAND_PREFIXES` | - | Allowed command prefixes |
| `TARS_SSH_BLOCKED_COMMAND_FRAGMENTS` | - | Blocked command fragments |
| `TARS_SSH_DISABLE_HOST_KEY_CHECKING` | `false` | Whether to disable host key checking |

#### 3.1.9 Execution Output Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_EXECUTION_OUTPUT_SPOOL_DIR` | `./data/execution_output` | Output storage directory |
| `TARS_EXECUTION_OUTPUT_MAX_PERSISTED_BYTES` | `262144` | Max storage bytes (256KB) |
| `TARS_EXECUTION_OUTPUT_CHUNK_BYTES` | `16384` | Chunk size (16KB) |
| `TARS_EXECUTION_OUTPUT_RETENTION` | `168h` | Retention period (7 days) |

#### 3.1.10 Approval Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_APPROVAL_TIMEOUT` | `15m` | Approval timeout |
| `TARS_APPROVALS_CONFIG_PATH` | - | Approval configuration file path |

#### 3.1.11 Feature Toggles

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_ROLLOUT_MODE` | `diagnosis_only` | Rollout mode |
| `TARS_FEATURES_DIAGNOSIS_ENABLED` | `true` | Enable diagnosis |
| `TARS_FEATURES_APPROVAL_ENABLED` | `false` | Enable approval |
| `TARS_FEATURES_EXECUTION_ENABLED` | `false` | Enable execution |
| `TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED` | `false` | Enable knowledge ingestion |

#### 3.1.12 GC Configuration

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `TARS_GC_ENABLED` | `true` | Enable garbage collection |
| `TARS_GC_INTERVAL` | `1h` | GC interval |
| `TARS_EXECUTION_OUTPUT_RETENTION` | `168h` | Execution output retention period |

### 3.2 Configuration File Description

#### 3.2.1 Main Configuration File (`tars.yaml`)

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

### 3.3 Rollout Mode Description

| Mode | Diagnosis | Approval | Execution | Knowledge |
|------|-----------|----------|-----------|-----------|
| `diagnosis_only` | ✅ | ❌ | ❌ | ❌ |
| `approval_beta` | ✅ | ✅ | ❌ | ❌ |
| `execution_beta` | ✅ | ✅ | ✅ | ❌ |
| `knowledge_on` | ✅ | ✅ | ✅ | ✅ |

---

## 4. External System Integration

### 4.1 Telegram Bot Configuration

#### 4.1.1 Create Bot

1. Search for `@BotFather` in Telegram
2. Send `/newbot` command
3. Set Bot name and username as prompted
4. Save the obtained Bot Token

#### 4.1.2 Get Chat ID

```bash
# Send a message to the Bot
# Then access (replace YOUR_BOT_TOKEN):
curl https://api.telegram.org/botYOUR_BOT_TOKEN/getUpdates

# Extract chat.id from the response
```

#### 4.1.3 Configure Webhook

```bash
# Set webhook (replace relevant variables)
curl -X POST "https://api.telegram.org/bot${BOT_TOKEN}/setWebhook" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://tars.example.com/api/v1/channels/telegram/webhook",
    "secret_token": "your-webhook-secret"
  }'

# Verify webhook status
curl "https://api.telegram.org/bot${BOT_TOKEN}/getWebhookInfo"
```

### 4.2 VictoriaMetrics Configuration

#### 4.2.1 VMAlert Configuration

Edit `vmalert-rules.yml`:

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

#### 4.2.2 VMAlert Startup Parameters

```bash
vmalert \
  -rule=vmalert-rules.yml \
  -datasource.url=http://victoriametrics:8428 \
  -notifier.url=http://tars:8080/api/v1/webhooks/vmalert \
  -notifier.headers="X-Tars-Secret:your-vmalert-secret" \
  -external.url=http://vmalert:8880
```

### 4.3 Model Gateway Configuration

#### 4.3.1 OpenAI Compatible Interface

```yaml
model:
  protocol: "openai_compatible"
  base_url: "https://api.openai.com/v1"
  api_key: "sk-..."
  model: "gpt-4o-mini"
```

#### 4.3.2 Local Model (LM Studio)

```yaml
model:
  protocol: "openai_compatible"
  base_url: "http://localhost:1234/v1"
  api_key: "not-needed"
  model: "local-model"
```

#### 4.3.3 Multi-Provider Configuration

Using `providers.yaml`:

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

### 4.4 SSH Key Configuration

#### 4.4.1 Generate Key Pair

```bash
# Generate ED25519 key (Recommended)
ssh-keygen -t ed25519 -f /etc/tars/id_ed25519 -C "tars@example.com"

# Or generate RSA key
ssh-keygen -t rsa -b 4096 -f /etc/tars/id_rsa -C "tars@example.com"
```

#### 4.4.2 Distribute Public Key

```bash
# Copy public key to target host
ssh-copy-id -i /etc/tars/id_ed25519.pub user@target-host

# Or add manually
# On target host:
echo "ssh-ed25519 AAAAC3NzaC... tars@example.com" >> ~/.ssh/authorized_keys
```

#### 4.4.3 Verify Connection

```bash
ssh -i /etc/tars/id_ed25519 -o ConnectTimeout=10 user@target-host "hostname"
```

---

## 5. Startup and Verification

### 5.1 Database Initialization

#### 5.1.1 Create Database

```bash
# Connect to PostgreSQL
psql -U postgres

# Create database and user
CREATE USER tars WITH PASSWORD 'tars';
CREATE DATABASE tars OWNER tars;
GRANT ALL PRIVILEGES ON DATABASE tars TO tars;

# Exit
\q
```

#### 5.1.2 Execute Migrations

```bash
# Run initialization script
psql -U tars -d tars -f migrations/postgres/0001_init.sql
```

### 5.2 Service Startup

#### 5.2.1 Docker Compose

```bash
# Start
docker compose up -d

# Stop
docker compose down

# Restart
docker compose restart

# View logs
docker compose logs -f
```

#### 5.2.2 Systemd

```bash
# Start
sudo systemctl start tars

# Stop
sudo systemctl stop tars

# Restart
sudo systemctl restart tars

# Check status
sudo systemctl status tars

# View logs
sudo journalctl -u tars -f
```

### 5.3 Health Check

#### 5.3.1 Basic Health Check

```bash
# Health check endpoint
curl http://localhost:8080/healthz
# Expected output: ok

# Readiness check endpoint
curl http://localhost:8080/readyz
# Expected output: ok

# View metrics
curl http://localhost:8080/metrics
```

#### 5.3.2 Component Health Check

```bash
# View detailed status via Ops API
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/dashboard/health
```

### 5.4 Smoke Testing

#### 5.4.1 Send Test Alert

```bash
# Send test alert using curl
curl -X POST http://localhost:8080/api/v1/smoke/alerts \
  -H "Content-Type: application/json" \
  -d '{
    "alert_name": "TestAlert",
    "severity": "warning",
    "target_host": "test-host",
    "description": "Smoke test alert"
  }'
```

#### 5.4.2 View Test Sessions

```bash
# List sessions
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/sessions

# View specific session
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/sessions/{session_id}
```

#### 5.4.3 Verify Telegram Notification

1. Execute Smoke test
2. Confirm Telegram received the alert message
3. Verify message format is correct

---

## 6. Upgrade and Rollback

### 6.1 Upgrade Steps

#### 6.1.1 Docker Compose Upgrade

```bash
# 1. Backup data
docker compose exec postgres pg_dump -U tars tars > backup_$(date +%Y%m%d).sql

# 2. Pull new version
docker compose pull

# 3. Stop service
docker compose down

# 4. Start new version
docker compose up -d

# 5. Verify
curl http://localhost:8080/healthz
```

#### 6.1.2 Binary Upgrade

```bash
# 1. Backup data
pg_dump -U tars tars > backup_$(date +%Y%m%d).sql

# 2. Stop service
sudo systemctl stop tars

# 3. Backup old version
sudo mv /opt/tars/bin/tars-server /opt/tars/bin/tars-server.backup

# 4. Deploy new version
sudo cp tars-server /opt/tars/bin/
sudo chmod +x /opt/tars/bin/tars-server

# 5. Start service
sudo systemctl start tars

# 6. Verify
sudo systemctl status tars
curl http://localhost:8080/healthz
```

### 6.2 Rollback Steps

#### 6.2.1 Docker Compose Rollback

```bash
# 1. Stop service
docker compose down

# 2. Use old version image
docker compose up -d

# 3. Restore data if needed
docker compose exec postgres psql -U tars -d tars < backup_YYYYMMDD.sql
```

#### 6.2.2 Binary Rollback

```bash
# 1. Stop service
sudo systemctl stop tars

# 2. Restore old version
sudo mv /opt/tars/bin/tars-server.backup /opt/tars/bin/tars-server

# 3. Start service
sudo systemctl start tars

# 4. Restore data if needed
dropdb -U postgres tars
createdb -U postgres -O tars tars
psql -U tars -d tars < backup_YYYYMMDD.sql
```

### 6.3 Database Migration

```bash
# Check current version
psql -U tars -d tars -c "SELECT * FROM schema_migrations;"

# Execute migration (if new migration files exist)
psql -U tars -d tars -f migrations/postgres/0002_xxx.sql
```

---

## 7. Troubleshooting

### 7.1 Startup Failure

**Problem**: Service fails to start

**Troubleshooting Steps**:
1. Check logs: `docker compose logs` or `journalctl -u tars`
2. Verify configuration file format
3. Check port occupation: `netstat -tlnp | grep 8080`
4. Confirm database connection is normal

### 7.2 Database Connection Failure

**Problem**: Unable to connect to PostgreSQL

**Troubleshooting Steps**:
1. Check PostgreSQL service status
2. Verify connection string is correct
3. Confirm network connectivity: `telnet postgres-host 5432`
4. Check firewall rules
5. Confirm user permissions are correct

### 7.3 Telegram Notification Failure

**Problem**: Not receiving Telegram messages

**Troubleshooting Steps**:
1. Verify Bot Token is correct
2. Check Webhook configuration: `curl https://api.telegram.org/bot${TOKEN}/getWebhookInfo`
3. Confirm TARS service is accessible from the public internet
4. Check webhook secret matches
5. View error messages in TARS logs

### 7.4 Model Call Failure

**Problem**: AI diagnosis cannot be generated

**Troubleshooting Steps**:
1. Verify Model Gateway is accessible
2. Check API Key validity
3. Confirm model name is correct
4. Check network connectivity
5. View error details in TARS logs

---

## 8. Reference Links

- [User Guide](./user-guide.md)
- [Admin Guide](./admin-guide.md)
- [API Documentation](../reference/api-reference.md)
- [Product Requirements Document](../../project/tars_prd.md)
- [Technical Design Document](../../project/tars_technical_design.md)

---

*This document is applicable to the TARS MVP version. Please adjust the configuration according to your actual situation during deployment.*
