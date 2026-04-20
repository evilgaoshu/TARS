# TARS 安全加固指南

> **版本**: v1.0
> **适用版本**: TARS MVP (Phase 1)
> **最后更新**: 2026-03-13

---

## 目录

1. [安全概述](#1-安全概述)
2. [网络安全](#2-网络安全)
3. [SSH 安全](#3-ssh-安全)
4. [Secrets 管理](#4-secrets-管理)
5. [访问控制](#5-访问控制)
6. [命令授权](#6-命令授权)
7. [数据脱敏](#7-数据脱敏)
8. [审计日志](#8-审计日志)
9. [安全监控](#9-安全监控)
10. [渗透测试检查清单](#10-渗透测试检查清单)

---

## 1. 安全概述

### 1.1 安全原则

TARS 遵循以下安全原则：

| 原则 | 说明 |
|------|------|
| **最小权限** | 服务只拥有必要的权限 |
| **纵深防御** | 多层安全控制 |
| **零信任** | 不信任任何内部通信 |
| **安全默认** | 默认安全配置 |
| **审计追踪** | 所有操作可审计 |

### 1.2 攻击面分析

```
┌─────────────────────────────────────────────────────────────┐
│                       攻击面分析                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐             │
│  │ Telegram │    │ VMAlert  │    │  Ops API │             │
│  │ Webhook  │    │ Webhook  │    │  (管理)   │             │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘             │
│       │               │               │                    │
│       └───────────────┼───────────────┘                    │
│                       │                                    │
│              ┌────────┴────────┐                          │
│              │   TARS 服务     │                          │
│              │   (核心防护)    │                          │
│              └────────┬────────┘                          │
│                       │                                    │
│       ┌───────────────┼───────────────┐                   │
│       │               │               │                    │
│  ┌────┴────┐   ┌─────┴────┐   ┌──────┴──────┐            │
│  │PostgreSQL│   │  SSH     │   │  VM Query  │            │
│  │(数据)    │   │(执行)    │   │(查询)      │            │
│  └─────────┘   └──────────┘   └─────────────┘            │
│                                                             │
└─────────────────────────────────────────────────────────────┘

风险等级:
- 🔴 高风险: SSH 执行、Ops API
- 🟡 中风险: Telegram Webhook、VMAlert
- 🟢 低风险: VM Query (只读)
```

---

## 2. 网络安全

### 2.1 TLS 配置

#### 2.1.1 启用 HTTPS

**Nginx 反向代理**:

```nginx
# /etc/nginx/conf.d/tars.conf
server {
    listen 443 ssl http2;
    server_name tars.company.com;

    # SSL 证书
    ssl_certificate /etc/ssl/certs/tars.crt;
    ssl_certificate_key /etc/ssl/private/tars.key;

    # SSL 配置（安全等级 A+）
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

# 跳转 HTTP 到 HTTPS
server {
    listen 80;
    server_name tars.company.com;
    return 301 https://$server_name$request_uri;
}
```

#### 2.1.2 证书管理

```bash
# 使用 Let's Encrypt
certbot --nginx -d tars.company.com

# 自动续期
echo "0 2 * * * certbot renew --quiet" | crontab

# 证书监控
#!/bin/bash
EXPIRY=$(openssl x509 -in /etc/ssl/certs/tars.crt -noout -enddate | cut -d= -f2)
EXPIRY_EPOCH=$(date -d "$EXPIRY" +%s)
NOW_EPOCH=$(date +%s)
DAYS_LEFT=$(( (EXPIRY_EPOCH - NOW_EPOCH) / 86400 ))

if [ $DAYS_LEFT -lt 7 ]; then
    echo "WARNING: SSL certificate expires in $DAYS_LEFT days"
fi
```

### 2.2 防火墙配置

#### 2.2.1 iptables 规则

```bash
#!/bin/bash
# /etc/iptables/tars.rules

# 清空规则
iptables -F
iptables -X

# 默认拒绝
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT ACCEPT

# 允许本地回环
iptables -A INPUT -i lo -j ACCEPT

# 允许已建立的连接
iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

# 允许 SSH
iptables -A INPUT -p tcp --dport 22 -s YOUR_ADMIN_IP -j ACCEPT

# 允许 HTTPS
iptables -A INPUT -p tcp --dport 443 -j ACCEPT

# 允许 TARS 主服务（仅内部网络）
iptables -A INPUT -p tcp --dport 8080 -s 10.0.0.0/8 -j ACCEPT

# 允许 TARS Ops API（仅管理网络）
iptables -A INPUT -p tcp --dport 8081 -s 10.0.1.0/24 -j ACCEPT

# PostgreSQL（仅内部网络）
iptables -A INPUT -p tcp --dport 5432 -s 10.0.0.0/8 -j ACCEPT

# 拒绝其他
iptables -A INPUT -j LOG --log-prefix "IPTABLES-DROP: "
iptables -A INPUT -j DROP
```

#### 2.2.2 CloudFlare 配置

```nginx
# 限制仅允许 CloudFlare IP
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

### 2.3 网络隔离

#### 2.3.1 Docker 网络隔离

```yaml
# docker-compose.yml
version: "3.9"

networks:
  # 前端网络（对外）
  frontend:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/24

  # 后端网络（内部）
  backend:
    driver: bridge
    internal: true  # 无外部访问
    ipam:
      config:
        - subnet: 172.20.1.0/24

  # 数据库网络
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

## 3. SSH 安全

### 3.1 SSH 密钥管理

#### 3.1.1 专用密钥生成

```bash
# 为 TARS 生成专用密钥对
ssh-keygen -t ed25519 -f /etc/tars/ssh/id_ed25519 -C "tars@company.com"

# 设置权限
chmod 600 /etc/tars/ssh/id_ed25519
chmod 644 /etc/tars/ssh/id_ed25519.pub
chown -R tars:tars /etc/tars/ssh

# 禁止 root 使用
chmod 700 /etc/tars/ssh
```

#### 3.1.2 密钥轮换策略

```bash
#!/bin/bash
# /usr/local/bin/rotate-ssh-keys.sh

TARS_KEY_DIR="/etc/tars/ssh"
BACKUP_DIR="/etc/tars/ssh/.backup/$(date +%Y%m%d)"
ROTATION_DAYS=90

# 检查是否需要轮换
if [ -f "$TARS_KEY_DIR/id_ed25519" ]; then
    KEY_AGE=$(( ($(date +%s) - $(stat -c %Y "$TARS_KEY_DIR/id_ed25519")) / 86400 ))

    if [ $KEY_AGE -gt $ROTATION_DAYS ]; then
        echo "Rotating SSH keys (age: $KEY_AGE days)"

        # 备份旧密钥
        mkdir -p "$BACKUP_DIR"
        cp "$TARS_KEY_DIR"/* "$BACKUP_DIR/"

        # 生成新密钥
        ssh-keygen -t ed25519 -f "$TARS_KEY_DIR/id_ed25519_new" -C "tars@company.com" -N ""

        # 分发新公钥到目标主机
        for host in $(cat /etc/tars/ssh/allowed_hosts); do
            ssh-copy-id -i "$TARS_KEY_DIR/id_ed25519_new.pub" \
                -o "IdentityFile=$TARS_KEY_DIR/id_ed25519" \
                tars@$host
        done

        # 切换密钥
        mv "$TARS_KEY_DIR/id_ed25519" "$TARS_KEY_DIR/id_ed25519.old"
        mv "$TARS_KEY_DIR/id_ed25519_new" "$TARS_KEY_DIR/id_ed25519"
        mv "$TARS_KEY_DIR/id_ed25519_new.pub" "$TARS_KEY_DIR/id_ed25519.pub"

        echo "Key rotation complete. Remove old keys after verification."
    fi
fi
```

### 3.2 目标主机安全配置

#### 3.2.1 SSH 服务器配置

```bash
# /etc/ssh/sshd_config (目标主机)

# 禁用 root 登录
PermitRootLogin no

# 仅允许密钥认证
PasswordAuthentication no
PubkeyAuthentication yes

# 限制用户
tarsuser

# 限制来源
AllowUsers tarsuser@10.0.0.*

# 其他安全设置
MaxAuthTries 3
ClientAliveInterval 300
ClientAliveCountMax 2
LoginGraceTime 60

# 使用 chroot jail（可选）
# Match User tarsuser
#     ChrootDirectory /var/tars-jail
#     ForceCommand internal-sftp
```

#### 3.2.2 命令白名单（目标主机）

```bash
# /etc/sudoers.d/tars (目标主机)
# 限制 tars 用户只能执行特定命令

# 允许只读命令
Cmnd_Alias TARS_READ_ONLY = /usr/bin/uptime, /bin/df, /usr/bin/free, /bin/cat /proc/loadavg

# 允许服务状态查询
Cmnd_Alias TARS_SERVICE_STATUS = /bin/systemctl status *, /bin/systemctl is-active *

# 允许特定写操作（需审批）
Cmnd_Alias TARS_RESTART_SSHD = /bin/systemctl restart sshd
Cmnd_Alias TARS_RESTART_NGINX = /bin/systemctl restart nginx

# 授权
tarsuser ALL=(root) NOPASSWD: TARS_READ_ONLY, TARS_SERVICE_STATUS
tarsuser ALL=(root) NOPASSWD: TARS_RESTART_SSHD, TARS_RESTART_NGINX
```

### 3.3 TARS SSH 配置

#### 3.3.1 严格主机检查

```yaml
# configs/tars.yaml
ssh:
  user: "tarsuser"
  private_key_path: "/etc/tars/ssh/id_ed25519"
  connect_timeout: "10s"
  command_timeout: "300s"
  disable_host_key_checking: false  # 生产环境禁用
  allowed_hosts:
    - "10.0.1.0/24"      # 仅允许特定网段
    - "prod-web-[0-9]*.company.com"  # 允许的主机模式
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

#### 3.3.2 Known Hosts 管理

```bash
# 预填充 known_hosts
ssh-keyscan -H prod-web-01.company.com >> /etc/tars/ssh/known_hosts
ssh-keyscan -H prod-web-02.company.com >> /etc/tars/ssh/known_hosts

# 定期更新
0 0 * * * /usr/bin/ssh-keyscan -H prod-web-*.company.com > /etc/tars/ssh/known_hosts.new && mv /etc/tars/ssh/known_hosts.new /etc/tars/ssh/known_hosts
```

---

## 4. Secrets 管理

### 4.1 环境变量加密

#### 4.1.1 使用 Docker Secrets

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
# 创建 secrets
echo "postgres://tars:secretpass@postgres:5432/tars" | docker secret create postgres_password -
echo "123456:ABC-DEF1234..." | docker secret create telegram_bot_token -
echo "sk-..." | docker secret create model_api_key -
openssl rand -base64 32 | docker secret create ops_api_token -
```

#### 4.1.2 使用 HashiCorp Vault

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

### 4.2 配置文件加密

#### 4.2.1 使用 SOPS

```bash
# 安装 sops
wget https://github.com/mozilla/sops/releases/download/v3.7.3/sops-v3.7.3.linux.amd64 -O /usr/local/bin/sops
chmod +x /usr/local/bin/sops

# 加密配置文件
sops --encrypt --in-place configs/tars.yaml

# 编辑加密文件
sops configs/tars.yaml

# 部署时解密
sops --decrypt configs/tars.yaml > configs/tars.decrypted.yaml
```

#### 4.2.2 使用 Ansible Vault

```bash
# 创建加密文件
ansible-vault create secrets.yml

# 编辑加密文件
ansible-vault edit secrets.yml

# 部署时解密
ansible-playbook --ask-vault-pass deploy.yml
```

---

## 5. 访问控制

### 5.1 Ops API 认证

#### 5.1.1 Token 管理

```bash
# 生成强 Token
openssl rand -base64 48

# 定期轮换
0 0 1 * * /usr/local/bin/rotate-ops-token.sh
```

```yaml
# configs/tars.yaml
ops_api:
  enabled: true
  listen: "127.0.0.1:8081"  # 仅本地访问
  bearer_token: "${OPS_API_TOKEN}"  # 从环境变量读取
  trusted_proxy_cidrs: ["10.0.0.0/8", "172.16.0.0/12"]
  require_gateway_identity: true
```

#### 5.1.2 IP 白名单

```bash
# Nginx 层限制
location /api/v1/ {
    # 只允许管理网络
    allow 10.0.1.0/24;
    deny all;

    proxy_pass http://localhost:8081;
}
```

### 5.2 Telegram Webhook 安全

#### 5.2.1 Secret Token

```bash
# 生成强 Secret
export TELEGRAM_WEBHOOK_SECRET=$(openssl rand -hex 32)

# 设置 Webhook
curl -X POST "https://api.telegram.org/bot${BOT_TOKEN}/setWebhook" \
  -H "Content-Type: application/json" \
  -d "{
    \"url\": \"https://tars.company.com/api/v1/channels/telegram/webhook\",
    \"secret_token\": \"${TELEGRAM_WEBHOOK_SECRET}\",
    \"max_connections\": 40
  }"
```

#### 5.2.2 IP 限制

```bash
# Telegram IP 段（官方）
# https://core.telegram.org/bots/webhooks

# Nginx 层限制
location /api/v1/channels/telegram/ {
    # Telegram IP 段
    allow 149.154.160.0/20;
    allow 91.108.4.0/22;
    deny all;

    proxy_pass http://localhost:8080;
}
```

---

## 6. 命令授权

### 6.1 授权策略配置

```yaml
# configs/authorization_policy.yaml
authorization:
  defaults:
    whitelist_action: direct_execute
    blacklist_action: suggest_only
    unmatched_action: require_approval

  hard_deny:
    ssh_command:
      # 危险命令 - 永不允许
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
      # 提权命令
      - "sudo*"
      - "su -"
      # 网络危险
      - "iptables -F"
      - "iptables -X"
      # 数据操作
      - "drop*database*"
      - "drop*table*"

  ssh_command:
    normalize_whitespace: true
    whitelist:
      # 只读信息收集
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
      # 网络
      - "ss -tlnp"
      - "netstat -tlnp"
      - "ip addr*"
      - "ip route*"
      # 服务状态
      - "systemctl status *"
      - "systemctl is-active *"
      - "systemctl list-units*"
      - "journalctl -u *"
      - "journalctl --since*"
      # 日志
      - "tail -n*"
      - "grep -i*"
      # HTTP 检查
      - "curl -fsS*"
      - "curl -s*"
      - "wget -qO-*"

    blacklist:
      # 服务控制
      - "systemctl restart *"
      - "systemctl stop *"
      - "systemctl disable *"
      - "kill *"
      - "pkill *"
      # 网络修改
      - "iptables *"
      - "ip link set*"
      - "ip addr add*"
      # 磁盘操作
      - "fdisk *"
      - "parted *"
      - "mount *"
      - "umount *"
      # 包管理
      - "apt-get *"
      - "yum *"
      - "pip *"
      - "npm *"

    overrides:
      # 特定服务允许特定操作
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

### 6.2 风险分级策略

```yaml
# configs/approvals.yaml
approval:
  default_timeout: 15m
  prohibit_self_approval: true  # 禁止自审批
  critical_dual_approval: true  # Critical 需双人审批

  routing:
    service_owner:
      # 按服务路由到 owner
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

  # 特定命令的审批策略
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

## 7. 数据脱敏

### 7.1 脱敏配置

```yaml
# configs/desensitization.yaml
desensitization:
  enabled: true

  secrets:
    key_names:
      # 敏感键名
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
      # 数据库
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
      # 公司特定模式
      - "[A-Z]{3}-[0-9]{6}"  # 内部编号
      - "corp-[a-z0-9]{8}"   # 公司 ID
    redact_bearer: true
    redact_basic_auth_url: true
    redact_sk_tokens: true

  placeholders:
    # 替换为占位符的字段
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
    # 执行时回水
    host: true
    ip: true
    path: true

  local_llm_assist:
    # 本地模型辅助检测敏感信息
    enabled: false
    provider: "openai_compatible"
    base_url: "http://127.0.0.1:11434/v1"
    model: "qwen2.5"
    mode: "detect_only"
```

### 7.2 脱敏示例

| 原始数据 | 脱敏后 |
|---------|-------|
| `password=secret123` | `password=[REDACTED]` |
| `api_key=sk-abc123` | `api_key=[REDACTED]` |
| `host=prod-web-01` | `host=[HOST:1]` |
| `ip=192.168.1.100` | `ip=[IP:1]` |
| `/var/log/app.log` | `/[PATH:1]/app.log` |

### 7.3 脱敏审计

```go
// 脱敏审计日志
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

## 8. 审计日志

### 8.1 审计配置

```yaml
# configs/tars.yaml - 审计配置
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

### 8.2 审计事件类型

| 事件 | 级别 | 记录内容 |
|------|------|----------|
| `session_created` | INFO | 告警接收、来源 |
| `diagnosis_completed` | INFO | AI 建议内容 |
| `execution_requested` | INFO | 命令、风险等级 |
| `execution_approved` | WARN | 审批人、原始/最终命令 |
| `execution_rejected` | WARN | 拒绝原因 |
| `execution_completed` | INFO | 退出码、输出大小 |
| `execution_failed` | ERROR | 错误信息 |
| `config_reloaded` | WARN | 配置变更 |
| `login_attempt` | WARN | 成功/失败 |
| `permission_denied` | ERROR | 拒绝的访问 |

### 8.3 审计日志保护

```bash
# 审计日志权限
chmod 600 /var/log/tars/audit.log
chown tars:tars /var/log/tars/audit.log

# 日志不可修改
chattr +a /var/log/tars/audit.log

# 定期归档
0 0 * * * /usr/local/bin/archive-audit-logs.sh
```

---

## 9. 安全监控

### 9.1 安全告警规则

```yaml
# 安全相关 Prometheus 规则
groups:
  - name: tars_security
    rules:
      # 失败登录尝试
      - alert: TARSFailedLogins
        expr: rate(tars_login_failures_total[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Multiple failed login attempts"

      # 未授权命令尝试
      - alert: TARSUnauthorizedCommands
        expr: rate(tars_command_blocked_total[5m]) > 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Unauthorized command attempts detected"

      # SSH 失败连接
      - alert: TARSSSHFailures
        expr: rate(tars_ssh_connection_failed_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Multiple SSH connection failures"

      # API 异常访问
      - alert: TARSAPIAbuse
        expr: rate(tars_http_requests_total{status=~"4..|5.."}[5m]) > 10
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High rate of failed API requests"
```

### 9.2 Falco 安全检测

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

### 9.3 安全扫描

```bash
#!/bin/bash
# security-scan.sh

# 依赖漏洞扫描
docker run --rm -v $(pwd):/app aquasec/trivy fs /app

# 容器镜像扫描
docker run --rm aquasec/trivy image tars:latest

# 密钥扫描
truffleHog --regex --entropy=False .

# SAST 扫描
docker run --rm -v $(pwd):/src securego/gosec /src/...
```

---

## 10. 渗透测试检查清单

### 10.1 认证与授权

- [ ] Ops API Token 强度检查
- [ ] Token 传输加密（HTTPS）
- [ ] Token 过期机制
- [ ] Telegram Webhook Secret 验证
- [ ] VMAlert Webhook Secret 验证
- [ ] 会话固定攻击防护
- [ ] 权限提升测试

### 10.2 注入攻击

- [ ] SQL 注入测试
- [ ] 命令注入测试
- [ ] 路径遍历测试
- [ ] JSON 注入测试
- [ ] 日志注入测试

### 10.3 敏感信息

- [ ] 配置文件权限检查
- [ ] Secrets 存储安全
- [ ] 日志敏感信息过滤
- [ ] 错误信息泄露检查
- [ ] 堆栈信息泄露检查

### 10.4 网络与通信

- [ ] TLS 配置检查
- [ ] 证书有效性验证
- [ ] 中间人攻击防护
- [ ] Webhook 来源验证
- [ ] CORS 配置检查

### 10.5 SSH 安全

- [ ] 密钥强度检查
- [ ] 主机密钥验证
- [ ] 命令白名单测试
- [ ] 权限最小化验证
- [ ] SSH 隧道检测

### 10.6 业务逻辑

- [ ] 重复审批绕过
- [ ] 审批超时绕过
- [ ] 命令修改绕过
- [ ] 越权访问测试
- [ ] 竞态条件测试

---

## 11. 应急响应

### 11.1 安全事件响应流程

```
1. 发现 -> 记录时间、现象、影响范围
2. 遏制 -> 隔离受影响系统
3. 分析 -> 确定攻击路径
4. 修复 -> 消除漏洞
5. 恢复 -> 恢复服务
6. 复盘 -> 更新安全措施
```

### 11.2 紧急禁用功能

```bash
# 立即禁用执行功能
export TARS_ROLLOUT_MODE=diagnosis_only
systemctl restart tars

# 禁用特定渠道
export TARS_TELEGRAM_ENABLED=false

# 启用只读模式
export TARS_FEATURES_EXECUTION_ENABLED=false
```

### 11.3 取证保留

```bash
#!/bin/bash
# incident-response.sh

INCIDENT_ID="$(date +%Y%m%d-%H%M%S)"
BACKUP_DIR="/var/incidents/$INCIDENT_ID"
mkdir -p "$BACKUP_DIR"

# 备份日志
cp /var/log/tars/*.log "$BACKUP_DIR/"
cp /var/log/tars/audit.log "$BACKUP_DIR/"

# 备份数据库
pg_dump tars > "$BACKUP_DIR/tars-dump.sql"

# 备份配置
cp -r /etc/tars "$BACKUP_DIR/configs"

# 系统状态
ps aux > "$BACKUP_DIR/processes.txt"
netstat -tlnp > "$BACKUP_DIR/ports.txt"

# 生成报告
echo "Incident $INCIDENT_ID captured at $(date)" > "$BACKUP_DIR/README.txt"
```

---

## 12. 参考链接

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CIS Docker Benchmark](https://www.cisecurity.org/benchmark/docker)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [Telegram Bot Security](https://core.telegram.org/bots/faq#security)
- [SSH Security](https://www.ssh.com/academy/ssh/security)

---

*本文档适用于 TARS MVP 版本，安全建议可能会随版本更新调整。*
