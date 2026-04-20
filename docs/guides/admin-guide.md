# TARS Administrator Guide

> **Version**: v1.0
> **Target Version**: TARS MVP (Phase 1)
> **Last Updated**: 2026-03-23

---

## 1. System Architecture In-depth Description

### 1.1 Architecture Overview

TARS adopts a **Modular Monolith** architecture design:

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP Server                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────┐ │
│  │ Alert       │  │ Channel      │  │ Workflow Core       │ │
│  │ Intake      │  │ Adapter      │  │ (State Machine +     │ │
│  │             │  │              │  │  Orchestration)      │ │
│  └─────────────┘  └──────────────┘  └─────────────────────┘ │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────┐ │
│  │ Reasoning   │  │ Action       │  │ Knowledge           │ │
│  │ Service     │  │ Gateway      │  │ Service             │ │
│  └─────────────┘  └──────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                     Foundation Layer                         │
│  (Config / Logger / Metrics / Audit / Tracing / Secrets)     │
├─────────────────────────────────────────────────────────────┤
│  PostgreSQL  │  SQLite-vec  │  External Model │  SSH  │  VM  │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 Core Design Principles

| Principle | Description |
|------|------|
| Centralized State | All business states are uniformly maintained by the Workflow Core |
| Unified Interaction | All user interactions are uniformly converted via the Channel Adapter |
| Unified External Integration | All Southbound calls exit via the Action Gateway |
| Swappable LLM | Reasoning Service only depends on the model interface, unaware of execution and state |
| Independent Knowledge Evolution | Knowledge Service evolves independently through queries and events |

### 1.3 Module Responsibility Boundaries

| Module | Responsibility | Boundary Constraints |
|------|------|----------|
| Alert Intake | Alert ingestion and standardization | No business decisions, no direct LLM calls |
| Channel Adapter | Channel protocol conversion | Does not hold business state, no approval judgment |
| Workflow Core | State machine and orchestration | Only module that can modify business state |
| Reasoning Service | Diagnostic recommendation generation | Does not execute commands, does not modify state |
| Action Gateway | External action execution | Does not own business state, does not decide execution permissions |
| Knowledge Service | Knowledge retrieval and accumulation | Does not participate in execution, does not directly modify approval results |

### 1.4 Call Relationships

```
Alert Intake ──→ Workflow Core
Channel Adapter ──→ Workflow Core
Workflow Core ──→ Channel Adapter
Workflow Core ──→ Reasoning Service
Workflow Core ──→ Action Gateway
Workflow Core ──session_closed event──→ Knowledge Service
Reasoning Service ──→ Knowledge Service
Action Gateway ──→ External Systems
Foundation ──→ Cross-cuts all modules
```

---

## 2. Configuration Management

### 2.1 Authorization Policy Configuration

#### 2.1.1 Configuration File Location

- Environment Variable: `TARS_AUTHORIZATION_CONFIG_PATH`
- Default Path: `./configs/authorization_policy.yaml`

#### 2.1.2 Configuration Structure

```yaml
authorization:
  defaults:
    whitelist_action: direct_execute   # Handling method for whitelisted commands
    blacklist_action: suggest_only     # Handling method for blacklisted commands
    unmatched_action: require_approval # Handling method for other commands

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

#### 2.1.3 Action Type Description

| Action | Description |
|------|------|
| `direct_execute` | Execute directly, no approval needed |
| `require_approval` | Requires manual approval |
| `suggest_only` | Suggest only, do not execute |
| `hard_deny` | Hard denial, execution forbidden |

### 2.2 Approval Route Configuration

#### 2.2.1 Configuration File Location

- Environment Variable: `TARS_APPROVALS_CONFIG_PATH`
- Default Path: `./configs/approvals.yaml`

#### 2.2.2 Configuration Structure

```yaml
approval:
  default_timeout: 15m              # Default approval timeout
  prohibit_self_approval: true      # Whether to prohibit self-approval

  routing:
    service_owner:                  # Route by service owner
      web:
        - "u_alice"
        - "u_bob"
      payment:
        - "u_charlie"

    oncall_group:                   # Route by on-call group
      default:
        - "u_sre_1"
        - "u_sre_2"

  execution:
    command_allowlist:              # Command allowlist
      sshd:
        - "systemctl restart sshd"
        - "systemctl status sshd"
        - "systemctl is-active sshd"
      web:
        - "systemctl restart nginx"
        - "systemctl status nginx"
```

#### 2.2.3 Routing Match Rules

1. Priority matching of `service_owner` by `service` label.
2. Fallback to `oncall_group` when no match.
3. `critical` level requires dual-person approval.

### 2.3 Reasoning Prompt Configuration

#### 2.3.1 Configuration File Location

- Environment Variable: `TARS_REASONING_PROMPTS_CONFIG_PATH`
- Default Path: `./configs/reasoning_prompts.yaml`

#### 2.3.2 Configuration Structure

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

#### 2.3.3 Prompt Optimization Suggestions

- Keep System Prompt concise and clear.
- Use strict JSON output format requirements.
- Explicitly specify command preferences and constraints.
- Provide example output formats.

### 2.4 Desensitization Rules Configuration

#### 2.4.1 Configuration File Location

- Environment Variable: `TARS_DESENSITIZATION_CONFIG_PATH`
- Default Path: `./configs/desensitization.yaml`

#### 2.4.2 Configuration Structure

```yaml
desensitization:
  enabled: true

  secrets:
    key_names:                      # Sensitive key names
      - password
      - passwd
      - token
      - secret
      - api_key
    query_key_names:                # Sensitive keys in query parameters
      - access_token
      - refresh_token
    additional_patterns:            # Extra matching patterns
      - "corp-[A-Z0-9]{6}"
    redact_bearer: true             # Desensitize Bearer Token
    redact_basic_auth_url: true     # Desensitize authentication info in URL
    redact_sk_tokens: true          # Desensitize tokens starting with sk-

  placeholders:
    host_key_fragments:             # Host-related keys
      - host
      - hostname
      - instance
      - node
    path_key_fragments:             # Path-related keys
      - path
      - file
      - filename
    replace_inline_ip: true         # Replace inline IP
    replace_inline_host: true       # Replace inline hostname
    replace_inline_path: true       # Replace inline path

  rehydration:
    host: true                      # Rehydrate hostname
    ip: true                        # Rehydrate IP
    path: true                      # Rehydrate path

  local_llm_assist:
    enabled: false
    provider: "openai_compatible"
    base_url: "http://127.0.0.1:11434/v1"
    model: "qwen2.5"
    mode: "detect_only"
```

#### 2.4.3 Desensitization Rule Description

| Type | Handling Method | Rehydratable |
|------|----------|------------|
| Secrets | Replaced by `[REDACTED]` | No (Permanent desensitization) |
| IP Address | Replaced by placeholder | Yes |
| Hostname | Replaced by placeholder | Yes |
| Path | Replaced by placeholder | Yes |

### 2.5 Provider Configuration

#### 2.5.1 Configuration File Location

- Environment Variable: `TARS_PROVIDERS_CONFIG_PATH`
- Default Path: `./configs/providers.yaml`

#### 2.5.2 Configuration Structure

```yaml
providers:
  primary:                          # Primary model
    provider_id: "primary-openrouter"
    model: "openai/gpt-4.1-mini"

  assist:                           # Assistant model
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

## 2A. Golden Path Replay and Acceptance

### 2A.1 Official Golden Path v1 Assets

TARS now ships an official replay entry for the alert-diagnosis closed loop:

- Alert fixture: `deploy/pilot/golden_path_alert_v1.json`
- Telegram callback fixture: `deploy/pilot/golden_path_telegram_callback_v1.json`
- Replay script: `scripts/run_golden_path_replay.sh`

Recommended usage:

```bash
TARS_OPS_API_TOKEN=... \
TARS_OPS_BASE_URL=http://127.0.0.1:8081 \
TARS_SERVER_BASE_URL=http://127.0.0.1:8080 \
bash scripts/run_golden_path_replay.sh
```

If you want the script to auto-send the Telegram approval callback, add:

```bash
TARS_GOLDEN_AUTO_APPROVE=1
```

### 2A.2 Replay Output Contract

The replay script prints stable fields for acceptance and troubleshooting:

- `session_id`
- `headline`
- `conclusion`
- `next_action`
- `snapshot=status|execution_status|verification_status|notification_headline|execution_headline|notification_count`

This is the preferred acceptance path for the MVP alert workflow, instead of ad-hoc manual command sequences.

### 2A.3 Golden Summary Read Model

For operator-facing views, TARS now derives a structured read model from session and execution data:

- `SessionDetail.golden_summary`
- `SessionDetail.notifications`
- `ExecutionDetail.session_id`
- `ExecutionDetail.golden_summary`

These fields are derived from existing diagnosis, verification, execution, and timeline data. They are not stored as separate database columns.

---

## 3. Monitoring and Alerting

### 3.1 Log Analysis

#### 3.1.1 Log Levels

| Level | Usage |
|------|------|
| DEBUG | Detailed debugging information |
| INFO | Normal operation logs |
| WARN | Warning information |
| ERROR | Error information |

#### 3.1.2 Log Configuration

```bash
# Environment variable sets log level
export TARS_LOG_LEVEL=INFO

# Docker Compose
docker compose logs -f tars

# Systemd
sudo journalctl -u tars -f
```

#### 3.1.3 Key Log Patterns

```bash
# Search error logs
grep "ERROR" /var/log/tars/*.log

# Search specific session
grep "session_id=xxx" /var/log/tars/*.log

# Search approval events
grep "approval" /var/log/tars/*.log
```

### 3.2 Metrics Monitoring

#### 3.2.1 Prometheus Metrics

TARS exposes the following metric endpoints:

```bash
# Get all metrics
curl http://localhost:8080/metrics
```

#### 3.2.2 Key Metrics

| Metric Name | Type | Description |
|--------|------|------|
| `tars_sessions_total` | Counter | Total number of sessions |
| `tars_sessions_active` | Gauge | Number of active sessions |
| `tars_executions_total` | Counter | Total number of executions |
| `tars_approvals_pending` | Gauge | Number of pending approvals |
| `tars_webhook_requests_total` | Counter | Number of Webhook requests |
| `tars_telegram_messages_sent` | Counter | Number of Telegram messages sent |
| `http_requests_total` | Counter | Number of HTTP requests |
| `http_request_duration_seconds` | Histogram | HTTP request duration |

#### 3.2.3 Grafana Dashboard

Use the provided Dashboard configuration:

```bash
# Import Dashboard
curl -X POST http://grafana:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @deploy/grafana/tars-mvp-dashboard.json
```

### 3.3 Health Checks

#### 3.3.1 Health Check Endpoints

```bash
# Health check
curl http://localhost:8080/healthz
# Response: ok

# Readiness check
curl http://localhost:8080/readyz
# Response: ok

# Detailed health status (Ops API)
curl -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8081/api/v1/dashboard/health
```

#### 3.3.2 Health Check Components

| Component | Check Content |
|------|----------|
| PostgreSQL | Connection availability |
| SQLite-vec | Vector store availability |
| Telegram | API connectivity |
| Model Gateway | Model service availability |
| VictoriaMetrics | Query service availability |

---

## 4. Security Management

### 4.1 SSH Security

#### 4.1.1 Key Management

```bash
# Generate dedicated key
ssh-keygen -t ed25519 -f /etc/tars/id_ed25519 -C "tars@example.com"

# Set permissions
chmod 600 /etc/tars/id_ed25519
chmod 644 /etc/tars/id_ed25519.pub

# Regular rotation (recommended every 90 days)
```

#### 4.1.2 Host Access Control

```yaml
ssh:
  allowed_hosts:
    - "192.168.1.0/24"      # Allowed internal network segments
    - "10.0.0.0/8"
    - "prod-web-*.example.com"  # Allowed host patterns
```

#### 4.1.3 Command Allowlist

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

### 4.2 Command Authorization

#### 4.2.1 Risk Grading

| Level | Definition | Example Command |
|------|------|----------|
| info | Read-only commands | `uptime`, `df -h`, `free -m` |
| warning | Recoverable write operations | `systemctl restart`, `kill` |
| critical | High-impact/Irreversible | `reboot`, `mkfs`, `rm -rf` |

#### 4.2.2 Authorization Policy

```yaml
authorization:
  defaults:
    whitelist_action: direct_execute
    blacklist_action: suggest_only
    unmatched_action: require_approval
```

### 4.3 Approval Policy

#### 4.3.1 Approval Timeout

```yaml
approval:
  default_timeout: 15m
  prohibit_self_approval: true
```

#### 4.3.2 Dual-person Approval

- Critical level commands require dual-person approval.
- Approvers must come from different approval groups.
- Self-approval is prohibited.

### 4.4 Data Desensitization

#### 4.4.1 Desensitization Scope

| Data Type | Desensitization Method | Example |
|----------|----------|------|
| IP Address | Replaced by `[IP:1]` | `192.168.1.1` → `[IP:1]` |
| Hostname | Replaced by `[HOST:1]` | `prod-web-01` → `[HOST:1]` |
| Secrets | Replaced by `[REDACTED]` | `token=xxx` → `token=[REDACTED]` |
| Password | Fully desensitized | `password=xxx` → `password=[REDACTED]` |

#### 4.4.2 Rehydration Mechanism

Desensitized data needs to be rehydrated to its original value during execution:

```yaml
desensitization:
  rehydration:
    host: true
    ip: true
    path: true
```

---

## 5. Fault Handling

### 5.1 Common Problem Troubleshooting

#### 5.1.1 Service Fails to Start

**Troubleshooting Steps**:

1. Check configuration file format:
   ```bash
   yamllint configs/*.yaml
   ```

2. Check port usage:
   ```bash
   netstat -tlnp | grep 8080
   ```

3. Check database connection:
   ```bash
   psql "${TARS_POSTGRES_DSN}" -c "SELECT 1"
   ```

4. View detailed error logs:
   ```bash
   docker compose logs tars
   ```

#### 5.1.2 AI Diagnosis Failure

**Troubleshooting Steps**:

1. Check model gateway connectivity:
   ```bash
   curl ${TARS_MODEL_BASE_URL}/models \
     -H "Authorization: Bearer ${TARS_MODEL_API_KEY}"
   ```

2. Check Provider configuration:
   ```bash
   curl -H "Authorization: Bearer ${OPS_TOKEN}" \
     http://localhost:8081/api/v1/config/providers/check
   ```

3. Check if desensitization configuration is excessive.

#### 5.1.3 Execution Failure

**Troubleshooting Steps**:

1. Check SSH connection:
   ```bash
   ssh -i ${TARS_SSH_PRIVATE_KEY_PATH} \
     ${TARS_SSH_USER}@target-host "hostname"
   ```

2. Check authorization policy:
   ```bash
   curl -H "Authorization: Bearer ${OPS_TOKEN}" \
     http://localhost:8081/api/v1/config/authorization
   ```

3. View execution output:
   ```bash
   curl -H "Authorization: Bearer ${OPS_TOKEN}" \
     http://localhost:8081/api/v1/executions/{id}
   ```

#### 5.1.4 Telegram Notification Failure

**Troubleshooting Steps**:

1. Check Webhook configuration:
   ```bash
   curl https://api.telegram.org/bot${TOKEN}/getWebhookInfo
   ```

2. Verify Bot Token:
   ```bash
   curl https://api.telegram.org/bot${TOKEN}/getMe
   ```

3. Check network connectivity:
   ```bash
   curl -I https://api.telegram.org
   ```

### 5.2 Rollback Operations

#### 5.2.1 Configuration Rollback

```bash
# Back up current configuration
cp configs/authorization_policy.yaml \
   configs/authorization_policy.yaml.backup

# Restore configuration
cp configs/authorization_policy.yaml.backup \
   configs/authorization_policy.yaml

# Reload (if hot reload is supported) or restart service
```

#### 5.2.2 Database Rollback

```bash
# Restore using backup
pg_dump -U tars tars > backup_before_change.sql

# Restore
psql -U tars -d tars < backup_before_change.sql
```

### 5.3 Data Recovery

#### 5.3.1 Execution Output Recovery

```bash
# Execution output stored in file system
ls -la ${TARS_EXECUTION_OUTPUT_SPOOL_DIR}

# Query output reference from database
psql -U tars -d tars -c \
  "SELECT id, output_ref FROM execution_requests WHERE id='xxx'"
```

#### 5.3.2 Session Data Recovery

```bash
# Query session data
psql -U tars -d tars -c \
  "SELECT * FROM alert_sessions WHERE id='xxx'"

# Query associated events
psql -U tars -d tars -c \
  "SELECT * FROM session_events WHERE session_id='xxx'"
```

---

## 6. Performance Optimization

### 6.1 Database Optimization

#### 6.1.1 Index Optimization

```sql
-- Check slow queries
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- Analyze tables
ANALYZE alert_sessions;
ANALYZE execution_requests;
```

#### 6.1.2 Connection Pool Configuration

```yaml
# Configure connection parameters in PostgreSQL DSN
postgres:
  dsn: "postgres://user:pass@host/db?sslmode=disable&pool_max_conns=20&pool_min_conns=5"
```

### 6.2 Caching Strategy

#### 6.2.1 Knowledge Base Cache

- Document vector index cache.
- Query result cache.

#### 6.2.2 Configuration Cache

- Authorization policy cache.
- Approval route cache.

### 6.3 Resource Limits

#### 6.3.1 Execution Timeout

```yaml
ssh:
  connect_timeout: "10s"
  command_timeout: "300s"  # Max 5 minutes

model:
  timeout: "30s"  # Model call timeout
```

#### 6.3.2 Output Limits

```yaml
execution_output:
  max_persisted_bytes: 262144  # 256KB
  chunk_bytes: 16384          # 16KB chunks
  retention: "168h"           # 7 days retention
```

### 6.4 Garbage Collection

```yaml
gc:
  enabled: true
  interval: "1h"
  execution_output_retain: "168h"
```

---

## 7. Maintenance Operations

### 7.1 Daily Checklist

- [ ] Check service health status.
- [ ] Check disk space.
- [ ] Check database connection count.
- [ ] Check logs for errors.
- [ ] Check pending approval requests.
- [ ] Check failed executions.

### 7.2 Regular Maintenance Tasks

| Task | Frequency | Command |
|------|------|------|
| Back up database | Daily | `pg_dump` |
| Clean up old execution output | Daily | GC Automatic |
| Rotate SSH keys | Quarterly | `ssh-keygen` |
| Review audit logs | Weekly | Web Console |
| Update model configuration | As needed | Configuration file |

### 7.3 Capacity Planning

| Resource | Monitoring Metric | Expansion Threshold |
|------|----------|----------|
| CPU | `process_cpu_seconds_total` | > 80% |
| Memory | `process_resident_memory_bytes` | > 80% |
| Disk | `node_filesystem_avail_bytes` | < 20% |
| Database | `pg_stat_activity_count` | > 80% |

---

## 8. Reference Links

- [Deployment Guide](./deployment-guide.md)
- [User Guide](./user-guide.md)
- [API Documentation](../reference/api-reference.md)
- [Product Requirements Document](../../project/tars_prd.md)
- [Technical Design Document](../../project/tars_technical_design.md)

---

*This document applies to the TARS MVP version; administrators should adjust configuration according to the actual deployment environment.*
