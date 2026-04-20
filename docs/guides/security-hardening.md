# TARS Security Hardening Guide

> **Version**: v1.0
> **Applicable Version**: TARS MVP (Phase 1)
> **Last Updated**: 2026-03-13

---

## Table of Contents

1. [Security Overview](#1-security-overview)
2. [Network Security](#2-network-security)
3. [SSH Security](#3-ssh-security)
4. [Secrets Management](#4-secrets-management)
5. [Access Control](#5-access-control)
6. [Command Authorization](#6-command-authorization)
7. [Data Desensitization](#7-data-desensitization)
8. [Audit Logs](#8-audit-logs)
9. [Security Monitoring](#9-security-monitoring)
10. [Penetration Testing Checklist](#10-penetration-testing-checklist)

---

## 1. Security Overview

### 1.1 Security Principles

TARS follows these security principles:

| Principle | Description |
|-----------|-------------|
| **Least Privilege** | Services only have the necessary permissions |
| **Defense in Depth** | Multiple layers of security control |
| **Zero Trust** | Do not trust any internal communication |
| **Secure by Default** | Default secure configurations |
| **Audit Trail** | All operations are auditable |

### 1.2 Attack Surface Analysis

```
┌─────────────────────────────────────────────────────────────┐
│                       Attack Surface Analysis               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐             │
│  │ Telegram │    │ VMAlert  │    │  Ops API │             │
│  │ Webhook  │    │ Webhook  │    │  (Admin) │             │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘             │
│       │               │               │                    │
│       └───────────────┼───────────────┘                    │
│                       │                                    │
│              ┌────────┴────────┐                          │
│              │   TARS Service  │                          │
│              │   (Core Protection)                        │
│              └────────┬────────┘                          │
│                       │                                    │
│       ┌───────────────┼───────────────┐                   │
│       │               │               │                    │
│  ┌────┴────┐   ┌─────┴────┐   ┌──────┴──────┐            │
│  │PostgreSQL│   │  SSH     │   │  VM Query  │            │
│  │(Data)    │   │(Execute) │   │(Query)     │            │
│  └─────────┘   └──────────┘   └─────────────┘            │
│                                                             │
└─────────────────────────────────────────────────────────────┘

Risk Levels:
- 🔴 High Risk: SSH Execution, Ops API
- 🟡 Medium Risk: Telegram Webhook, VMAlert
- 🟢 Low Risk: VM Query (Read-only)
```

---

## 2. Network Security

### 2.1 TLS Configuration

#### 2.1.1 Enable HTTPS

**Nginx Reverse Proxy**:

```nginx
# /etc/nginx/conf.d/tars.conf
server {
    listen 443 ssl http2;
    server_name tars.company.com;

    # SSL Certificates
    ssl_certificate /etc/ssl/certs/tars.crt;
    ssl_certificate_key /etc/ssl/private/tars.key;

    # SSL Configuration (Security Grade A+)
    ssl_protocols TLSv1.3;
    ssl_ciphers 'TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256';
    ssl_prefer_server_ciphers off;
    ssl_session_timeout 1d;
    ssl_session_cache shared:SSL:50m;

    # HSTS
    add_header Strict-Transport-Security "max-age=63072000" always;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name tars.company.com;
    return 301 https://$server_name$request_uri;
}
```

#### 2.1.2 Certificate Management

```bash
# Using Let's Encrypt
certbot --nginx -d tars.company.com

# Auto-renewal
echo "0 2 * * * certbot renew --quiet" | crontab

# Certificate Monitoring
#!/bin/bash
EXPIRY=$(openssl x509 -in /etc/ssl/certs/tars.crt -noout -enddate | cut -d= -f2)
EXPIRY_EPOCH=$(date -d "$EXPIRY" +%s)
NOW_EPOCH=$(date +%s)
DAYS_LEFT=$(( (EXPIRY_EPOCH - NOW_EPOCH) / 86400 ))

if [ $DAYS_LEFT -lt 7 ]; then
    echo "WARNING: SSL certificate expires in $DAYS_LEFT days"
fi
```

### 2.2 Firewall Configuration

#### 2.2.1 iptables Rules

```bash
#!/bin/bash
# /etc/iptables/tars.rules

# Clear rules
iptables -F
iptables -X

# Default deny
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT ACCEPT

# Allow loopback
iptables -A INPUT -i lo -j ACCEPT

# Allow established connections
iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

# Allow SSH
iptables -A INPUT -p tcp --dport 22 -s YOUR_ADMIN_IP -j ACCEPT

# Allow HTTPS
iptables -A INPUT -p tcp --dport 443 -j ACCEPT

# Allow TARS main service (Internal network only)
iptables -A INPUT -p tcp --dport 8080 -s 10.0.0.0/8 -j ACCEPT

# Allow TARS Ops API (Management network only)
iptables -A INPUT -p tcp --dport 8081 -s 10.0.1.0/24 -j ACCEPT

# PostgreSQL (Internal network only)
iptables -A INPUT -p tcp --dport 5432 -s 10.0.0.0/8 -j ACCEPT

# Deny others
iptables -A INPUT -j LOG --log-prefix "IPTABLES-DROP: "
iptables -A INPUT -j DROP
```

#### 2.2.2 CloudFlare Configuration

```nginx
# Restrict to CloudFlare IPs only
# /etc/nginx/cloudflare-ips.conf

# IPv4
allow 173.245.48.0/20;
allow 103.21.244.0/22;
allow 103.22.200.0/22;
allow 103.31.4.0/22;
allow 141.101.64.0/18;
allow 108.162.192.0/18;
allow 190.93.240.0/20;
allow 188.114.96.0/20;
allow 197.234.240.0/22;
allow 198.41.128.0/17;
allow 162.158.0.0/15;
allow 104.16.0.0/13;
allow 104.24.0.0/14;
allow 172.64.0.0/13;
allow 131.0.72.0/22;
deny all;
```

### 2.3 Network Isolation

#### 2.3.1 Docker Network Isolation

```yaml
# docker-compose.yml
version: "3.9"

networks:
  # Frontend network (External)
  frontend:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/24

  # Backend network (Internal)
  backend:
    driver: bridge
    internal: true  # No external access
    ipam:
      config:
        - subnet: 172.20.1.0/24

  # Database network
  database:
    driver: bridge
    internal: true
    ipam:
      config:
        - subnet: 172.20.2.0/24

services:
  tars:
    networks:
      - frontend
      - backend
      - database

  postgres:
    networks:
      - database

  victoriametrics:
    networks:
      - backend
```

---

## 3. SSH Security

### 3.1 SSH Key Management

#### 3.1.1 Dedicated Key Generation

```bash
# Generate dedicated key pair for TARS
ssh-keygen -t ed25519 -f /etc/tars/ssh/id_ed25519 -C "tars@company.com"

# Set permissions
chmod 600 /etc/tars/ssh/id_ed25519
chmod 644 /etc/tars/ssh/id_ed25519.pub
chown -R tars:tars /etc/tars/ssh

# Forbid root usage
chmod 700 /etc/tars/ssh
```

#### 3.1.2 Key Rotation Strategy

```bash
#!/bin/bash
# /usr/local/bin/rotate-ssh-keys.sh

TARS_KEY_DIR="/etc/tars/ssh"
BACKUP_DIR="/etc/tars/ssh/.backup/$(date +%Y%m%d)"
ROTATION_DAYS=90

# Check if rotation is needed
if [ -f "$TARS_KEY_DIR/id_ed25519" ]; then
    KEY_AGE=$(( ($(date +%s) - $(stat -c %Y "$TARS_KEY_DIR/id_ed25519")) / 86400 ))

    if [ $KEY_AGE -gt $ROTATION_DAYS ]; then
        echo "Rotating SSH keys (age: $KEY_AGE days)"

        # Backup old keys
        mkdir -p "$BACKUP_DIR"
        cp "$TARS_KEY_DIR"/* "$BACKUP_DIR/"

        # Generate new keys
        ssh-keygen -t ed25519 -f "$TARS_KEY_DIR/id_ed25519_new" -C "tars@company.com" -N ""

        # Distribute new public keys to target hosts
        for host in $(cat /etc/tars/ssh/allowed_hosts); do
            ssh-copy-id -i "$TARS_KEY_DIR/id_ed25519_new.pub" \
                -o "IdentityFile=$TARS_KEY_DIR/id_ed25519" \
                tars@$host
        done

        # Switch keys
        mv "$TARS_KEY_DIR/id_ed25519" "$TARS_KEY_DIR/id_ed25519.old"
        mv "$TARS_KEY_DIR/id_ed25519_new" "$TARS_KEY_DIR/id_ed25519"
        mv "$TARS_KEY_DIR/id_ed25519_new.pub" "$TARS_KEY_DIR/id_ed25519.pub"

        echo "Key rotation complete. Remove old keys after verification."
    fi
fi
```

### 3.2 Target Host Security Configuration

#### 3.2.1 SSH Server Configuration

```bash
# /etc/ssh/sshd_config (Target Host)

# Disable root login
PermitRootLogin no

# Only allow key authentication
PasswordAuthentication no
PubkeyAuthentication yes

# Restrict users
AllowUsers tarsuser

# Restrict sources
AllowUsers tarsuser@10.0.0.*

# Other security settings
MaxAuthTries 3
ClientAliveInterval 300
ClientAliveCountMax 2
LoginGraceTime 60

# Use chroot jail (Optional)
# Match User tarsuser
#     ChrootDirectory /var/tars-jail
#     ForceCommand internal-sftp
```

#### 3.2.2 Command Whitelist (Target Host)

```bash
# /etc/sudoers.d/tars (Target Host)
# Restrict tars user to specific commands only

# Allow read-only commands
Cmnd_Alias TARS_READ_ONLY = /usr/bin/uptime, /bin/df, /usr/bin/free, /bin/cat /proc/loadavg

# Allow service status queries
Cmnd_Alias TARS_SERVICE_STATUS = /bin/systemctl status *, /bin/systemctl is-active *

# Allow specific write operations (requires approval)
Cmnd_Alias TARS_RESTART_SSHD = /bin/systemctl restart sshd
Cmnd_Alias TARS_RESTART_NGINX = /bin/systemctl restart nginx

# Authorization
tarsuser ALL=(root) NOPASSWD: TARS_READ_ONLY, TARS_SERVICE_STATUS
tarsuser ALL=(root) NOPASSWD: TARS_RESTART_SSHD, TARS_RESTART_NGINX
```

### 3.3 TARS SSH Configuration

#### 3.3.1 Strict Host Checking

```yaml
# configs/tars.yaml
ssh:
  user: "tarsuser"
  private_key_path: "/etc/tars/ssh/id_ed25519"
  connect_timeout: "10s"
  command_timeout: "300s"
  disable_host_key_checking: false  # Disabled in production
  allowed_hosts:
    - "10.0.1.0/24"      # Only allow specific network segment
    - "prod-web-[0-9]*.company.com"  # Allowed host patterns
  allowed_command_prefixes:
    - "uptime"
    - "df -h"
    - "free -m"
    - "cat /proc/loadavg"
    - "systemctl status"
    - "systemctl is-active"
  blocked_command_fragments:
    - "rm -rf"
    - "mkfs"
    - "dd if=/dev/zero"
    - "shutdown"
    - "reboot"
    - ">/dev/sda"
    - "init 0"
    - "init 6"
```

#### 3.3.2 Known Hosts Management

```bash
# Pre-fill known_hosts
ssh-keyscan -H prod-web-01.company.com >> /etc/tars/ssh/known_hosts
ssh-keyscan -H prod-web-02.company.com >> /etc/tars/ssh/known_hosts

# Periodic update
0 0 * * * /usr/bin/ssh-keyscan -H prod-web-*.company.com > /etc/tars/ssh/known_hosts.new && mv /etc/tars/ssh/known_hosts.new /etc/tars/ssh/known_hosts
```

---

## 4. Secrets Management

### 4.1 Environment Variable Encryption

#### 4.1.1 Using Docker Secrets

```yaml
# docker-compose.yml
version: "3.9"

secrets:
  postgres_password:
    external: true
  telegram_bot_token:
    external: true
  model_api_key:
    external: true
  ops_api_token:
    external: true

services:
  tars:
    image: tars:latest
    secrets:
      - postgres_password
      - telegram_bot_token
      - model_api_key
      - ops_api_token
    environment:
      - TARS_POSTGRES_DSN_FILE=/run/secrets/postgres_password
      - TARS_TELEGRAM_BOT_TOKEN_FILE=/run/secrets/telegram_bot_token
      - TARS_MODEL_API_KEY_FILE=/run/secrets/model_api_key
      - TARS_OPS_API_TOKEN_FILE=/run/secrets/ops_api_token
```

```bash
# Create secrets
echo "postgres://tars:secretpass@postgres:5432/tars" | docker secret create postgres_password -
echo "123456:ABC-DEF1234..." | docker secret create telegram_bot_token -
echo "sk-..." | docker secret create model_api_key -
openssl rand -base64 32 | docker secret create ops_api_token -
```

#### 4.1.2 Using HashiCorp Vault

```go
// internal/foundation/secrets/vault.go
package secrets

import (
    "fmt"
    "github.com/hashicorp/vault/api"
)

type VaultStore struct {
    client *api.Client
}

func NewVaultStore(addr string, token string) (*VaultStore, error) {
    config := &api.Config{
        Address: addr,
    }
    client, err := api.NewClient(config)
    if err != nil {
        return nil, err
    }
    client.SetToken(token)
    return &VaultStore{client: client}, nil
}

func (v *VaultStore) Get(key string) (string, error) {
    secret, err := v.client.Logical().Read(fmt.Sprintf("secret/data/tars/%s", key))
    if err != nil {
        return "", err
    }
    if secret == nil {
        return "", fmt.Errorf("secret not found: %s", key)
    }
    data := secret.Data["data"].(map[string]interface{})
    return data["value"].(string), nil
}
```

### 4.2 Configuration File Encryption

#### 4.2.1 Using SOPS

```bash
# Install sops
wget https://github.com/mozilla/sops/releases/download/v3.7.3/sops-v3.7.3.linux.amd64 -O /usr/local/bin/sops
chmod +x /usr/local/bin/sops

# Encrypt config file
sops --encrypt --in-place configs/tars.yaml

# Edit encrypted file
sops configs/tars.yaml

# Decrypt during deployment
sops --decrypt configs/tars.yaml > configs/tars.decrypted.yaml
```

#### 4.2.2 Using Ansible Vault

```bash
# Create encrypted file
ansible-vault create secrets.yml

# Edit encrypted file
ansible-vault edit secrets.yml

# Decrypt during deployment
ansible-playbook --ask-vault-pass deploy.yml
```

---

## 5. Access Control

### 5.1 Ops API Authentication

#### 5.1.1 Token Management

```bash
# Generate strong Token
openssl rand -base64 48

# Periodic rotation
0 0 1 * * /usr/local/bin/rotate-ops-token.sh
```

```yaml
# configs/tars.yaml
ops_api:
  enabled: true
  listen: "127.0.0.1:8081"  # Local access only
  bearer_token: "${OPS_API_TOKEN}"  # Read from environment variable
  trusted_proxy_cidrs: ["10.0.0.0/8", "172.16.0.0/12"]
  require_gateway_identity: true
```

#### 5.1.2 IP Whitelisting

```bash
# Nginx layer restriction
location /api/v1/ {
    # Only allow management network
    allow 10.0.1.0/24;
    deny all;

    proxy_pass http://localhost:8081;
}
```

### 5.2 Telegram Webhook Security

#### 5.2.1 Secret Token

```bash
# Generate strong Secret
export TELEGRAM_WEBHOOK_SECRET=$(openssl rand -hex 32)

# Set Webhook
curl -X POST "https://api.telegram.org/bot${BOT_TOKEN}/setWebhook" \
  -H "Content-Type: application/json" \
  -d "{
    \"url\": \"https://tars.company.com/api/v1/channels/telegram/webhook\",
    \"secret_token\": \"${TELEGRAM_WEBHOOK_SECRET}\",
    \"max_connections\": 40
  }"
```

#### 5.2.2 IP Restrictions

```bash
# Telegram IP ranges (Official)
# https://core.telegram.org/bots/webhooks

# Nginx layer restriction
location /api/v1/channels/telegram/ {
    # Telegram IP ranges
    allow 149.154.160.0/20;
    allow 91.108.4.0/22;
    deny all;

    proxy_pass http://localhost:8080;
}
```

---

## 6. Command Authorization

### 6.1 Authorization Policy Configuration

```yaml
# configs/authorization_policy.yaml
authorization:
  defaults:
    whitelist_action: direct_execute
    blacklist_action: suggest_only
    unmatched_action: require_approval

  hard_deny:
    ssh_command:
      # Dangerous commands - Never allowed
      - "rm -rf /"
      - "rm -rf /*"
      - "mkfs*"
      - "dd if=/dev/zero*"
      - ">/dev/sda"
      - "shutdown*"
      - "reboot*"
      - "init 0"
      - "init 6"
      - "poweroff"
      - "halt"
      # Privilege escalation
      - "sudo*"
      - "su -"
      # Network hazards
      - "iptables -F"
      - "iptables -X"
      # Data operations
      - "drop*database*"
      - "drop*table*"

  ssh_command:
    normalize_whitespace: true
    whitelist:
      # Read-only information gathering
      - "hostname"
      - "uptime"
      - "whoami"
      - "cat /proc/loadavg"
      - "cat /proc/cpuinfo"
      - "cat /proc/meminfo"
      - "df -h*"
      - "df -T*"
      - "free -m*"
      - "free -h*"
      - "ps aux*"
      - "top -bn*"
      - "iostat*"
      - "vmstat*"
      # Network
      - "ss -tlnp"
      - "netstat -tlnp"
      - "ip addr*"
      - "ip route*"
      # Service status
      - "systemctl status *"
      - "systemctl is-active *"
      - "systemctl list-units*"
      - "journalctl -u *"
      - "journalctl --since*"
      # Logs
      - "tail -n*"
      - "grep -i*"
      # HTTP check
      - "curl -fsS*"
      - "curl -s*"
      - "wget -qO-*"

    blacklist:
      # Service control
      - "systemctl restart *"
      - "systemctl stop *"
      - "systemctl disable *"
      - "kill *"
      - "pkill *"
      # Network modification
      - "iptables *"
      - "ip link set*"
      - "ip addr add*"
      # Disk operations
      - "fdisk *"
      - "parted *"
      - "mount *"
      - "umount *"
      # Package management
      - "apt-get *"
      - "yum *"
      - "pip *"
      - "npm *"

    overrides:
      # Specific operations allowed for specific services
      - id: "sshd-restart-allowed"
        services:
          - "sshd"
        hosts:
          - "10.0.1.*"
        command_globs:
          - "systemctl restart sshd"
          - "systemctl reload sshd"
        action: require_approval
        approval_route: oncall_group
        approval_timeout: 15m
        note: "SSHD restart allowed with approval"

      - id: "nginx-restart-allowed"
        services:
          - "web"
          - "nginx"
        command_globs:
          - "systemctl restart nginx"
          - "systemctl reload nginx"
          - "nginx -t"
        action: require_approval
        approval_route: service_owner
```

### 6.2 Risk Leveling Policy

```yaml
# configs/approvals.yaml
approval:
  default_timeout: 15m
  prohibit_self_approval: true  # Prohibit self-approval
  critical_dual_approval: true  # Critical requires dual approval

  routing:
    service_owner:
      # Route to owner by service
      web:
        - "u_alice"
        - "u_bob"
      api:
        - "u_charlie"
      database:
        - "u_dave"
        - "u_eve"

    oncall_group:
      default:
        - "u_sre_1"
        - "u_sre_2"
      emergency:
        - "u_manager"
        - "u_director"

  # Approval policies for specific commands
  command_policies:
    - pattern: "systemctl restart *"
      risk_level: warning
      default_timeout: 10m

    - pattern: "systemctl stop *"
      risk_level: critical
      require_dual_approval: true
      default_timeout: 30m
```

---

## 7. Data Desensitization

### 7.1 Desensitization Configuration

```yaml
# configs/desensitization.yaml
desensitization:
  enabled: true

  secrets:
    key_names:
      # Sensitive key names
      - password
      - passwd
      - pwd
      - token
      - secret
      - api_key
      - api_secret
      - access_key
      - private_key
      - credential
      - auth
      - bearer
      # Database
      - db_password
      - db_host
      - redis_password
      - mongo_password
    query_key_names:
      - access_token
      - refresh_token
      - token
      - secret
      - api_key
      - session_id
    additional_patterns:
      # Company specific patterns
      - "[A-Z]{3}-[0-9]{6}"  # Internal ID
      - "corp-[a-z0-9]{8}"   # Company ID
    redact_bearer: true
    redact_basic_auth_url: true
    redact_sk_tokens: true

  placeholders:
    # Fields to be replaced with placeholders
    host_key_fragments:
      - host
      - hostname
      - instance
      - node
      - address
      - fqdn
      - server
      - target
    path_key_fragments:
      - path
      - file
      - filename
      - filepath
      - dir
      - directory
      - log_path
      - data_path
    replace_inline_ip: true
    replace_inline_host: true
    replace_inline_path: true

  rehydration:
    # Rehydrate during execution
    host: true
    ip: true
    path: true

  local_llm_assist:
    # Assist sensitive info detection with local model
    enabled: false
    provider: "openai_compatible"
    base_url: "http://127.0.0.1:11434/v1"
    model: "qwen2.5"
    mode: "detect_only"
```

### 7.2 Desensitization Examples

| Original Data | Desensitized |
|---------------|--------------|
| `password=secret123` | `password=[REDACTED]` |
| `api_key=sk-abc123` | `api_key=[REDACTED]` |
| `host=prod-web-01` | `host=[HOST:1]` |
| `ip=192.168.1.100` | `ip=[IP:1]` |
| `/var/log/app.log` | `/[PATH:1]/app.log` |

### 7.3 Desensitization Audit

```go
// Desensitization audit log
type DesensitizationAudit struct {
    SessionID    string    `json:"session_id"`
    OriginalKeys []string  `json:"original_keys"`
    RedactedKeys []string  `json:"redacted_keys"`
    HostMappings map[string]string `json:"host_mappings"`
    IPMappings   map[string]string `json:"ip_mappings"`
    CreatedAt    time.Time `json:"created_at"`
}
```

---

## 8. Audit Logs

### 8.1 Audit Configuration

```yaml
# configs/tars.yaml - Audit configuration
audit:
  enabled: true
  level: "all"  # all | write | none
  destinations:
    - type: postgres
      table: audit_logs
    - type: file
      path: /var/log/tars/audit.log
      rotation: daily
```

### 8.2 Audit Event Types

| Event | Level | Content |
|-------|-------|---------|
| `session_created` | INFO | Alert reception, source |
| `diagnosis_completed` | INFO | AI suggestion content |
| `execution_requested` | INFO | Command, risk level |
| `execution_approved` | WARN | Approver, original/final command |
| `execution_rejected` | WARN | Rejection reason |
| `execution_completed` | INFO | Exit code, output size |
| `execution_failed` | ERROR | Error message |
| `config_reloaded` | WARN | Configuration change |
| `login_attempt` | WARN | Success/Failure |
| `permission_denied` | ERROR | Denied access |

### 8.3 Audit Log Protection

```bash
# Audit log permissions
chmod 600 /var/log/tars/audit.log
chown tars:tars /var/log/tars/audit.log

# Log immutable
chattr +a /var/log/tars/audit.log

# Periodic archiving
0 0 * * * /usr/local/bin/archive-audit-logs.sh
```

---

## 9. Security Monitoring

### 9.1 Security Alert Rules

```yaml
# Security related Prometheus rules
groups:
  - name: tars_security
    rules:
      # Failed login attempts
      - alert: TARSFailedLogins
        expr: rate(tars_login_failures_total[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Multiple failed login attempts"

      # Unauthorized command attempts
      - alert: TARSUnauthorizedCommands
        expr: rate(tars_command_blocked_total[5m]) > 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Unauthorized command attempts detected"

      # SSH connection failures
      - alert: TARSSSHFailures
        expr: rate(tars_ssh_connection_failed_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Multiple SSH connection failures"

      # API abnormal access
      - alert: TARSAPIAbuse
        expr: rate(tars_http_requests_total{status=~"4..|5.."}[5m]) > 10
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High rate of failed API requests"
```

### 9.2 Falco Security Detection

```yaml
# /etc/falco/rules.d/tars.yaml
- rule: TARS Unauthorized Config Access
  desc: Detect unauthorized access to TARS config files
  condition: >
    open_read and
    (fd.name contains "/etc/tars/" or
     fd.name contains "/opt/tars/configs/") and
    not (user.name in (tars, root))
  output: >
    Unauthorized access to TARS config
    user=%user.name file=%fd.name
  priority: WARNING

- rule: TARS SSH Key Access
  desc: Detect access to TARS SSH keys
  condition: >
    open_read and
    fd.name contains "/etc/tars/ssh/" and
    not proc.name in (tars, ssh-keyscan)
  output: >
    Unauthorized access to SSH keys
    user=%user.name file=%fd.name
  priority: CRITICAL
```

### 9.3 Security Scanning

```bash
#!/bin/bash
# security-scan.sh

# Dependency vulnerability scanning
docker run --rm -v $(pwd):/app aquasec/trivy fs /app

# Container image scanning
docker run --rm aquasec/trivy image tars:latest

# Secret scanning
truffleHog --regex --entropy=False .

# SAST scanning
docker run --rm -v $(pwd):/src securego/gosec /src/...
```

---

## 10. Penetration Testing Checklist

### 10.1 Authentication & Authorization

- [ ] Ops API Token strength check
- [ ] Token transmission encryption (HTTPS)
- [ ] Token expiration mechanism
- [ ] Telegram Webhook Secret verification
- [ ] VMAlert Webhook Secret verification
- [ ] Session fixation protection
- [ ] Privilege escalation testing

### 10.2 Injection Attacks

- [ ] SQL injection testing
- [ ] Command injection testing
- [ ] Path traversal testing
- [ ] JSON injection testing
- [ ] Log injection testing

### 10.3 Sensitive Information

- [ ] Configuration file permission check
- [ ] Secrets storage security
- [ ] Log sensitive information filtering
- [ ] Error message leakage check
- [ ] Stack trace leakage check

### 10.4 Network & Communication

- [ ] TLS configuration check
- [ ] Certificate validity verification
- [ ] MitM protection
- [ ] Webhook source verification
- [ ] CORS configuration check

### 10.5 SSH Security

- [ ] Key strength check
- [ ] Host key verification
- [ ] Command whitelist testing
- [ ] Privilege minimization verification
- [ ] SSH tunnel detection

### 10.6 Business Logic

- [ ] Repeated approval bypass
- [ ] Approval timeout bypass
- [ ] Command modification bypass
- [ ] Broken access control testing
- [ ] Race condition testing

---

## 11. Emergency Response

### 11.1 Security Incident Response Flow

```
1. Detection -> Record time, phenomenon, impact scope
2. Containment -> Isolate affected systems
3. Analysis -> Determine attack path
4. Remediation -> Eliminate vulnerabilities
5. Recovery -> Restore services
6. Post-mortem -> Update security measures
```

### 11.2 Emergency Feature Disable

```bash
# Disable execution feature immediately
export TARS_ROLLOUT_MODE=diagnosis_only
systemctl restart tars

# Disable specific channel
export TARS_TELEGRAM_ENABLED=false

# Enable read-only mode
export TARS_FEATURES_EXECUTION_ENABLED=false
```

### 11.3 Forensic Preservation

```bash
#!/bin/bash
# incident-response.sh

INCIDENT_ID="$(date +%Y%m%d-%H%M%S)"
BACKUP_DIR="/var/incidents/$INCIDENT_ID"
mkdir -p "$BACKUP_DIR"

# Backup logs
cp /var/log/tars/*.log "$BACKUP_DIR/"
cp /var/log/tars/audit.log "$BACKUP_DIR/"

# Backup database
pg_dump tars > "$BACKUP_DIR/tars-dump.sql"

# Backup configuration
cp -r /etc/tars "$BACKUP_DIR/configs"

# System status
ps aux > "$BACKUP_DIR/processes.txt"
netstat -tlnp > "$BACKUP_DIR/ports.txt"

# Generate report
echo "Incident $INCIDENT_ID captured at $(date)" > "$BACKUP_DIR/README.txt"
```

---

## 12. Reference Links

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CIS Docker Benchmark](https://www.cisecurity.org/benchmark/docker)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [Telegram Bot Security](https://core.telegram.org/bots/faq#security)
- [SSH Security](https://www.ssh.com/academy/ssh/security)

---

*This document is applicable to the TARS MVP version. Security recommendations may be adjusted with version updates.*
