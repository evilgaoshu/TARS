# TARS Webhook 集成指南

> **版本**: v1.0
> **适用版本**: TARS MVP (Phase 1)
> **最后更新**: 2026-03-13

---

## 目录

1. [概述](#1-概述)
2. [VMAlert 集成](#2-vmalert-集成)
3. [Alertmanager 集成](#3-alertmanager-集成)
4. [自定义 Webhook 集成](#4-自定义-webhook-集成)
5. [签名验证机制](#5-签名验证机制)
6. [故障排查](#6-故障排查)
7. [最佳实践](#7-最佳实践)

---

## 1. 概述

### 1.1 Webhook 类型

| 类型 | 路径 | 用途 | 认证方式 |
|------|------|------|----------|
| VMAlert | `/api/v1/webhooks/vmalert` | 接收 VMAlert 告警 | Secret Header |
| VMAlert V2 | `/api/v1/webhooks/vmalert/api/v2/alerts` | 兼容 Alertmanager | Secret Header |
| Telegram | `/api/v1/channels/telegram/webhook` | Telegram 回调 | Telegram Secret |
| Custom | `/api/v1/webhooks/custom` | 自定义集成 | API Key |

### 1.2 通用特性

- **幂等性**: 相同告警多次发送不会重复创建会话
- **异步处理**: Webhook 接收后立即返回，后台异步处理
- **去重**: 基于告警指纹自动去重
- **签名验证**: 支持 HMAC 签名验证防止伪造

---

## 2. VMAlert 集成

### 2.1 配置 VMAlert

#### 2.1.1 基础配置

编辑 `vmalert-rules.yml`:

```yaml
groups:
  - name: tars_alerts
    rules:
      # CPU 告警
      - alert: HighCPUUsage
        expr: 100 - (avg by(instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 80
        for: 5m
        labels:
          severity: warning
          service: "{{ $labels.service | default \"unknown\" }}"
          team: sre
        annotations:
          summary: "High CPU usage on {{ $labels.instance }}"
          description: "CPU usage is above 80% (current value: {{ $value }}%)"
          runbook_url: "https://wiki.company.com/runbooks/high-cpu"

      # 内存告警
      - alert: HighMemoryUsage
        expr: (node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes * 100 > 85
        for: 5m
        labels:
          severity: warning
          service: "{{ $labels.service }}"
        annotations:
          summary: "High memory usage"
          description: "Memory usage is above 85%"

      # 磁盘告警
      - alert: DiskSpaceLow
        expr: (node_filesystem_avail_bytes{fstype!="tmpfs"} / node_filesystem_size_bytes) * 100 < 10
        for: 5m
        labels:
          severity: critical
          service: "{{ $labels.service }}"
        annotations:
          summary: "Disk space is running low"
          description: "Less than 10% space left"

      # 服务不可用
      - alert: ServiceDown
        expr: up == 0
        for: 1m
        labels:
          severity: critical
          service: "{{ $labels.job }}"
        annotations:
          summary: "Service {{ $labels.job }} is down"
```

#### 2.1.2 路由配置

编辑 `vmalert.yml`:

```yaml
# 告警路由配置
route:
  group_by: ['alertname', 'service']
  group_wait: 10s
  group_interval: 30s
  repeat_interval: 1h
  receiver: 'tars'

receivers:
  - name: 'tars'
    webhook_configs:
      - url: 'http://tars:8080/api/v1/webhooks/vmalert'
        send_resolved: true
        headers:
          X-Tars-Secret: 'your-vmalert-secret-here'
```

#### 2.1.3 启动参数

```bash
vmalert \
  -rule=/etc/vmalert/rules/*.yml \
  -datasource.url=http://victoriametrics:8428 \
  -notifier.url=http://tars:8080/api/v1/webhooks/vmalert \
  -notifier.headers="X-Tars-Secret:your-vmalert-secret" \
  -remoteWrite.url=http://victoriametrics:8428 \
  -external.url=http://vmalert:8880 \
  -loggerLevel=INFO
```

### 2.2 Webhook 消息格式

#### 2.2.1 告警触发 (firing)

```json
{
  "receiver": "tars",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighCPUUsage",
        "severity": "warning",
        "instance": "prod-web-01.example.com",
        "service": "web",
        "team": "sre"
      },
      "annotations": {
        "summary": "High CPU usage on prod-web-01.example.com",
        "description": "CPU usage is above 80% (current value: 85%)",
        "runbook_url": "https://wiki.company.com/runbooks/high-cpu"
      },
      "startsAt": "2026-03-13T10:00:00.000Z",
      "endsAt": "0001-01-01T00:00:00.000Z",
      "generatorURL": "http://victoriametrics:8428/vmalert/...",
      "fingerprint": "8b5d0e5c3a7b2f1e"
    }
  ],
  "groupLabels": {
    "alertname": "HighCPUUsage",
    "service": "web"
  },
  "commonLabels": {
    "team": "sre"
  },
  "commonAnnotations": {},
  "externalURL": "http://vmalert:8880",
  "version": "1",
  "groupKey": "{}/{}:{alertname=\"HighCPUUsage\", service=\"web\"}",
  "truncatedAlerts": 0
}
```

#### 2.2.2 告警恢复 (resolved)

```json
{
  "receiver": "tars",
  "status": "resolved",
  "alerts": [
    {
      "status": "resolved",
      "labels": { ... },
      "annotations": { ... },
      "startsAt": "2026-03-13T10:00:00.000Z",
      "endsAt": "2026-03-13T10:15:00.000Z",
      "generatorURL": "...",
      "fingerprint": "8b5d0e5c3a7b2f1e"
    }
  ],
  ...
}
```

### 2.3 字段映射

| VMAlert 字段 | TARS 字段 | 说明 |
|-------------|-----------|------|
| `labels.alertname` | `alert.name` | 告警名称 |
| `labels.severity` | `alert.severity` | 告警级别 |
| `labels.instance` | `alert.target_host` | 目标主机 |
| `labels.service` | `alert.service_name` | 服务名称 |
| `labels.*` | `alert.labels` | 所有标签 |
| `annotations.summary` | `alert.summary` | 告警摘要 |
| `annotations.description` | `alert.description` | 告警描述 |
| `annotations.*` | `alert.annotations` | 所有注释 |
| `startsAt` | `alert.fired_at` | 触发时间 |
| `endsAt` | `alert.resolved_at` | 恢复时间 |
| `fingerprint` | `alert.fingerprint` | 告警指纹 |
| `generatorURL` | `alert.source_url` | 告警源 URL |

### 2.4 高级标签处理

#### 2.4.1 提取主机名

```yaml
# VMAlert 规则
- alert: HostDown
  expr: up{job="node"} == 0
  labels:
    severity: critical
    # 提取短主机名
    host_short: "{{ $labels.instance | regexReplace \"^([^.]+).*\" \"\\1\" }}"
  annotations:
    # 提取域名
    domain: "{{ $labels.instance | regexReplace \"^[^.]+\\.\" \"\" }}"
```

#### 2.4.2 条件标签

```yaml
labels:
  # 根据环境设置不同标签
  env: "{{ if match \"prod-.*\" .Labels.instance }}prod{{ else }}staging{{ end }}"
  # 设置默认标签
  team: "{{ $labels.team | default \"sre\" }}"
```

---

## 3. Alertmanager 集成

### 3.1 Alertmanager 配置

编辑 `alertmanager.yml`:

```yaml
global:
  resolve_timeout: 5m

route:
  receiver: 'tars'
  group_by: ['alertname', 'service']
  group_wait: 10s
  group_interval: 30s
  repeat_interval: 1h

receivers:
  - name: 'tars'
    webhook_configs:
      - url: 'http://tars:8080/api/v1/webhooks/vmalert/api/v2/alerts'
        send_resolved: true
        http_config:
          headers:
            X-Tars-Secret: 'your-vmalert-secret-here'

inhibit_rules:
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'instance']
```

### 3.2 与 VMAlert 的区别

| 特性 | VMAlert | Alertmanager |
|------|---------|--------------|
| 路径 | `/api/v1/webhooks/vmalert` | `/api/v1/webhooks/vmalert/api/v2/alerts` |
| 分组 | 服务端支持 | 客户端分组 |
| 抑制 | 服务端支持 | 客户端抑制 |
| 静默 | 需额外配置 | 原生支持 |
| 去重 | 服务端指纹 | 服务端指纹 |

---

## 4. 自定义 Webhook 集成

### 4.1 通用 Webhook 格式

TARS 支持标准 Alertmanager 格式的 Webhook：

```json
{
  "receiver": "tars",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "CustomAlert",
        "severity": "warning"
      },
      "annotations": {
        "summary": "Custom alert summary",
        "description": "Custom alert description"
      },
      "startsAt": "2026-03-13T10:00:00.000Z",
      "endsAt": "0001-01-01T00:00:00.000Z",
      "fingerprint": "unique-fingerprint"
    }
  ],
  "groupLabels": {},
  "commonLabels": {},
  "commonAnnotations": {},
  "externalURL": "",
  "version": "1"
}
```

### 4.2 Python 发送示例

```python
import requests
import json
from datetime import datetime

def send_alert_to_tars():
    webhook_url = "http://tars:8080/api/v1/webhooks/vmalert"
    secret = "your-vmalert-secret"

    payload = {
        "receiver": "tars",
        "status": "firing",
        "alerts": [
            {
                "status": "firing",
                "labels": {
                    "alertname": "ApplicationErrorRateHigh",
                    "severity": "critical",
                    "service": "api-gateway",
                    "instance": "api-01.example.com",
                    "team": "backend"
                },
                "annotations": {
                    "summary": "High error rate on API Gateway",
                    "description": "Error rate is above 5% for 5 minutes",
                    "runbook_url": "https://wiki.company.com/runbooks/api-errors"
                },
                "startsAt": datetime.utcnow().isoformat() + "Z",
                "endsAt": "0001-01-01T00:00:00Z",
                "fingerprint": "app-error-001"
            }
        ],
        "groupLabels": {"alertname": "ApplicationErrorRateHigh"},
        "commonLabels": {"team": "backend"},
        "commonAnnotations": {},
        "externalURL": "",
        "version": "1"
    }

    response = requests.post(
        webhook_url,
        headers={
            "Content-Type": "application/json",
            "X-Tars-Secret": secret
        },
        json=payload
    )

    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")

if __name__ == "__main__":
    send_alert_to_tars()
```

### 4.3 Shell 发送示例

```bash
#!/bin/bash

TARS_URL="http://tars:8080/api/v1/webhooks/vmalert"
TARS_SECRET="your-vmalert-secret"

# 构建告警 JSON
ALERT_JSON=$(cat <<EOF
{
  "receiver": "tars",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "ManualTestAlert",
        "severity": "warning",
        "service": "test-service",
        "instance": "test-host"
      },
      "annotations": {
        "summary": "Manual test alert",
        "description": "This is a test alert"
      },
      "startsAt": "$(date -u +%Y-%m-%dT%H:%M:%S)Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "fingerprint": "manual-test-$(date +%s)"
    }
  ],
  "groupLabels": {},
  "commonLabels": {},
  "commonAnnotations": {},
  "externalURL": "",
  "version": "1"
}
EOF
)

# 发送请求
curl -X POST "$TARS_URL" \
  -H "Content-Type: application/json" \
  -H "X-Tars-Secret: $TARS_SECRET" \
  -d "$ALERT_JSON"
```

### 4.4 Go 发送示例

```go
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
    "time"
)

type Alert struct {
    Status      string            `json:"status"`
    Labels      map[string]string `json:"labels"`
    Annotations map[string]string `json:"annotations"`
    StartsAt    time.Time         `json:"startsAt"`
    EndsAt      time.Time         `json:"endsAt"`
    Fingerprint string            `json:"fingerprint"`
}

type WebhookPayload struct {
    Receiver       string            `json:"receiver"`
    Status         string            `json:"status"`
    Alerts         []Alert           `json:"alerts"`
    GroupLabels    map[string]string `json:"groupLabels"`
    CommonLabels   map[string]string `json:"commonLabels"`
    CommonAnnotations map[string]string `json:"commonAnnotations"`
    ExternalURL    string            `json:"externalURL"`
    Version        string            `json:"version"`
}

func sendAlert() error {
    payload := WebhookPayload{
        Receiver: "tars",
        Status:   "firing",
        Alerts: []Alert{
            {
                Status: "firing",
                Labels: map[string]string{
                    "alertname": "CustomAlert",
                    "severity":  "warning",
                    "service":   "my-service",
                },
                Annotations: map[string]string{
                    "summary":     "Custom alert",
                    "description": "This is a custom alert",
                },
                StartsAt:    time.Now(),
                EndsAt:      time.Time{},
                Fingerprint: "custom-001",
            },
        },
        Version: "1",
    }

    jsonData, _ := json.Marshal(payload)

    req, _ := http.NewRequest("POST", "http://tars:8080/api/v1/webhooks/vmalert", bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Tars-Secret", "your-vmalert-secret")

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    return nil
}
```

---

## 5. 签名验证机制

### 5.1 HMAC 签名验证

TARS 使用 HMAC-SHA256 验证 Webhook 签名（可选）。

#### 5.1.1 签名生成

```python
import hmac
import hashlib

def generate_signature(payload: str, secret: str) -> str:
    """生成 HMAC-SHA256 签名"""
    signature = hmac.new(
        secret.encode('utf-8'),
        payload.encode('utf-8'),
        hashlib.sha256
    ).hexdigest()
    return f"sha256={signature}"

# 使用示例
secret = "your-webhook-secret"
payload = '{"status": "firing", ...}'
signature = generate_signature(payload, secret)
# 结果: sha256=abc123...
```

#### 5.1.2 发送带签名的请求

```python
import requests

payload = {"status": "firing", "alerts": [...]}
payload_json = json.dumps(payload)
signature = generate_signature(payload_json, "your-secret")

response = requests.post(
    "http://tars:8080/api/v1/webhooks/vmalert",
    headers={
        "Content-Type": "application/json",
        "X-Tars-Secret": "your-vmalert-secret",
        "X-Tars-Signature": signature
    },
    data=payload_json
)
```

### 5.2 Secret 验证

#### 5.2.1 Header 验证

TARS 通过 `X-Tars-Secret` Header 验证来源：

```bash
# 配置环境变量
export TARS_VMALERT_WEBHOOK_SECRET="your-secret"

# 发送请求时必须包含
-H "X-Tars-Secret: your-secret"
```

#### 5.2.2 Query 参数验证（备选）

```bash
# 通过 URL 参数传递（不推荐，仅用于测试）
curl "http://tars:8080/api/v1/webhooks/vmalert?secret=your-secret" \
  -H "Content-Type: application/json" \
  -d '{...}'
```

---

## 6. 故障排查

### 6.1 Webhook 接收失败

#### 症状：VMAlert 日志显示发送失败

```
error: unexpected status code: 401
```

**排查步骤**：

1. 检查 Secret 是否匹配
   ```bash
   # 查看 TARS 配置
curl -H "Authorization: Bearer ${TOKEN}" \
    http://tars:8081/api/v1/setup/status | jq .vmalert
   ```

2. 测试 Webhook 端点
   ```bash
   curl -v http://tars:8080/api/v1/webhooks/vmalert \
     -H "Content-Type: application/json" \
     -H "X-Tars-Secret: your-secret" \
     -d '{"receiver":"test","status":"firing","alerts":[],"version":"1"}'
   ```

#### 症状：告警发送成功但未创建会话

**排查步骤**：

1. 检查 TARS 日志
   ```bash
   docker logs tars | grep "webhook"
   ```

2. 检查去重逻辑
   ```sql
   -- 查询告警指纹
   SELECT fingerprint, received_at
   FROM alert_events
   ORDER BY received_at DESC
   LIMIT 10;
   ```

3. 检查幂等键
   ```sql
   SELECT * FROM idempotency_keys
   WHERE scope = 'webhook'
   ORDER BY last_seen_at DESC
   LIMIT 10;
   ```

### 6.2 重复告警

#### 症状：相同告警多次创建会话

**解决方案**：

1. 确保 VMAlert 配置正确的分组
   ```yaml
   route:
     group_by: ['alertname', 'service']
     group_wait: 10s
   ```

2. 检查告警指纹生成
   ```bash
   # 确保 labels 中包含足够区分度的字段
   labels:
     alertname: "..."
     instance: "..."
     service: "..."
   ```

### 6.3 网络问题

#### 症状：连接超时

**排查步骤**：

1. 测试连通性
   ```bash
   # 从 VMAlert 容器测试
   docker exec vmalert curl -I http://tars:8080/healthz
   ```

2. 检查防火墙规则
   ```bash
   # 检查 TARS 监听地址
   netstat -tlnp | grep 8080
   ```

3. 使用 Docker 网络
   ```yaml
   # docker-compose.yml
   services:
     tars:
       networks:
         - tars-net
     vmalert:
       networks:
         - tars-net
   networks:
     tars-net:
   ```

---

## 7. 最佳实践

### 7.1 告警设计

#### 7.1.1 告警分级

```yaml
# Critical - 立即处理
- alert: ServiceDown
  labels:
    severity: critical
  annotations:
    runbook_url: "https://wiki.company.com/runbooks/service-down"

# Warning - 需要关注
- alert: HighCPUUsage
  labels:
    severity: warning
  annotations:
    runbook_url: "https://wiki.company.com/runbooks/high-cpu"

# Info - 参考信息
- alert: DeploymentCompleted
  labels:
    severity: info
```

#### 7.1.2 告警模板

```yaml
# 使用模板变量
annotations:
  summary: "{{ $labels.alertname }} on {{ $labels.instance }}"
  description: |
    {{ $labels.alertname }} has been firing for more than {{ $value }} minutes.

    Current value: {{ $value }}
    Threshold: {{ $labels.threshold }}

    Runbook: {{ $labels.runbook_url }}
```

### 7.2 安全建议

#### 7.2.1 Secret 管理

```bash
# 使用 Docker Secrets
docker secret create tars_vmalert_secret -
# 输入 secret 内容

# 在 docker-compose.yml 中使用
secrets:
  vmalert_secret:
    external: true

services:
  tars:
    secrets:
      - vmalert_secret
    environment:
      - TARS_VMALERT_WEBHOOK_SECRET_FILE=/run/secrets/vmalert_secret
```

#### 7.2.2 TLS 加密

```yaml
# VMAlert 配置
receivers:
  - name: 'tars'
    webhook_configs:
      - url: 'https://tars.company.com/api/v1/webhooks/vmalert'
        http_config:
          tls_config:
            ca_file: '/etc/ssl/certs/ca.crt'
          headers:
            X-Tars-Secret: 'your-secret'
```

### 7.3 高可用配置

```yaml
# VMAlert 高可用
route:
  receiver: 'tars-ha'

receivers:
  - name: 'tars-ha'
    webhook_configs:
      - url: 'http://tars-1:8080/api/v1/webhooks/vmalert'
        send_resolved: true
        headers:
          X-Tars-Secret: 'secret'
      - url: 'http://tars-2:8080/api/v1/webhooks/vmalert'
        send_resolved: true
        headers:
          X-Tars-Secret: 'secret'
```

### 7.4 监控 Webhook 健康

```bash
# 检查 Webhook 成功率
curl -H "Authorization: Bearer ${TOKEN}" \
  http://tars:8081/api/v1/dashboard/health | jq '.webhook'

# 预期输出:
# {
#   "status": "healthy",
#   "total_received": 1000,
#   "total_accepted": 998,
#   "success_rate": 99.8
# }
```

---

## 8. 参考链接

- [VMAlert 官方文档](https://docs.victoriametrics.com/vmalert/)
- [Alertmanager 官方文档](https://prometheus.io/docs/alerting/latest/configuration/)
- [Prometheus Alerting](https://prometheus.io/docs/practices/alerting/)
- [TARS 部署手册](./deployment-guide.md)
- [TARS API 文档](../reference/api-reference.md)

---

*本文档适用于 TARS MVP 版本，Webhook 格式可能会在未来版本中调整。*
