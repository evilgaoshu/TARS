# TARS Error Codes and Troubleshooting Guide

> **Version**: v1.0
> **Applicable Version**: TARS MVP (Phase 1)
> **Last Updated**: 2026-03-13

---

## Table of Contents

1. [Error Code Reference](#1-error-code-reference)
2. [Startup Failures](#2-startup-failures)
3. [Database Issues](#3-database-issues)
4. [AI Diagnosis Issues](#4-ai-diagnosis-issues)
5. [Execution Issues](#5-execution-issues)
6. [Telegram Issues](#6-telegram-issues)
7. [Webhook Issues](#7-webhook-issues)
8. [Performance Issues](#8-performance-issues)
9. [Security Related Issues](#9-security-related-issues)
10. [Diagnostic Tools](#10-diagnostic-tools)

---

## 1. Error Code Reference

### 1.1 HTTP Status Codes

| Status Code | Meaning | Common Scenarios | Solution |
|-------------|---------|-----------------|----------|
| 200 | OK | Request processed normally | - |
| 201 | Created | Resource created successfully | - |
| 204 | No Content | Deletion successful | - |
| 400 | Bad Request | Missing required fields, malformed format | Check request body |
| 401 | Unauthorized | Invalid or missing token | Check authentication info |
| 403 | Forbidden | Insufficient permissions | Check role permissions |
| 404 | Not Found | Incorrect ID or resource deleted | Confirm resource exists |
| 409 | Conflict | Duplicate creation, concurrency conflict | Check idempotency key |
| 422 | Unprocessable Entity | Business logic error | Check business rules |
| 429 | Too Many Requests | Rate limit triggered | Reduce request frequency |
| 500 | Internal Server Error | Uncaught exception | Check logs |
| 503 | Service Unavailable | Dependency service failure | Check dependency services |

### 1.2 Business Error Codes

#### System-level Errors (S001-S099)

| Error Code | Description | Cause | Solution |
|------------|-------------|-------|----------|
| `S001` | Configuration load failed | Malformed config file, path does not exist | Check config file syntax |
| `S002` | Database connection failed | Incorrect DSN, network unreachable, PostgreSQL not started | Check database config and connection |
| `S003` | Vector store initialization failed | Incorrect SQLite path, insufficient permissions | Check path and permissions |
| `S004` | Port occupied | 8080 or 8081 already in use | Change port or stop occupying process |
| `S005` | Config file permission error | Config file not readable | Check file permissions |
| `S006` | Out of memory | System memory exhausted | Increase memory or optimize config |
| `S007` | Out of disk space | Log or data directory full | Clean up disk or expand capacity |
| `S008` | Service startup timeout | Dependency service slow to respond | Check dependency service status |
| `S009` | Certificate load failed | Incorrect TLS cert path or expired | Check certificate config |
| `S010` | Plugin load failed | Connector configuration error | Check connector configuration |

#### Session Errors (E001-E099)

| Error Code | Description | Cause | Solution |
|------------|-------------|-------|----------|
| `E001` | Session does not exist | Incorrect session_id or expired | Confirm session_id is correct |
| `E002` | Invalid session status | Current status does not allow this operation | Check session state machine |
| `E003` | Session closed | Session resolved or failed | Create a new session |
| `E004` | Session creation failed | Alert parsing error or database write failure | Check alert format and database |
| `E005` | Session update conflict | Optimistic locking version conflict | Retry operation |
| `E006` | Session deduplication failed | Idempotency key conflict | Check deduplication config |
| `E007` | Session timeout | Processing time exceeded threshold | Optimize processing logic or increase timeout |
| `E008` | Session association failed | Alert fingerprint match failed | Check alert labels |
| `E009` | Session archive failed | Archive configuration error | Check archive settings |
| `E010` | Session recovery failed | Incomplete recovery data | Check backup data |

#### Execution Errors (E101-E199)

| Error Code | Description | Cause | Solution |
|------------|-------------|-------|----------|
| `E101` | Execution request does not exist | Incorrect execution_id | Confirm execution_id |
| `E102` | Approval timeout | Approval time limit exceeded | Re-initiate execution request |
| `E103` | Command not authorized | Hit blacklist or policy denied | Check authorization policy |
| `E104` | SSH connection failed | Network unreachable, host unreachable, incorrect key | Check SSH config |
| `E105` | SSH authentication failed | Incorrect key or insufficient permissions | Check SSH key and authorization |
| `E106` | Command execution timeout | Command execution time exceeded threshold | Increase timeout or optimize command |
| `E107` | Command execution failed | Command returned non-zero exit code | Check command syntax and target host status |
| `E108` | Output write failed | Disk full or insufficient permissions | Check disk space and permissions |
| `E109` | Output read failed | File does not exist or insufficient permissions | Check output file |
| `E110` | Host not in whitelist | Target host not authorized | Update SSH whitelist config |
| `E111` | Command intercepted | Contains dangerous fragments | Check command content |
| `E112` | Approval rejected | Approver rejected execution | Modify command and re-apply |
| `E113` | Dual approval incomplete | Critical level requires two approvers | Wait for second approver |
| `E114` | Self-approval forbidden | prohibit_self_approval enabled | Transfer to another approver |
| `E115` | Approval routing failed | Approver not found | Check approval routing config |
| `E116` | Execution interrupted | Service restart or manual cancellation | Re-initiate execution |

#### AI Diagnosis Errors (E201-E299)

| Error Code | Description | Cause | Solution |
|------------|-------------|-------|----------|
| `E201` | Model call failed | Model gateway unreachable, incorrect API Key | Check model config |
| `E202` | Model response timeout | Model processing time too long | Increase timeout or downgrade |
| `E203` | Model response format error | Returned unexpected format | Check prompt config |
| `E204` | Desensitization failed | Sensitive info detection error | Check desensitization config |
| `E205` | Knowledge retrieval failed | Vector store query error | Check vector store status |
| `E206` | Context assembly failed | Incomplete session data | Check session data |
| `E207` | Model quota exhausted | API call limit reached | Wait for quota recovery or switch model |
| `E208` | Model content filtered | Content triggered safety policy | Adjust prompt or handle manually |
| `E209` | Fallback processing failed | Local model also failed | Check local model config |
| `E210` | Provider switch failed | Backup provider unavailable | Check provider config |

#### Telegram Errors (E301-E399)

| Error Code | Description | Cause | Solution |
|------------|-------------|-------|----------|
| `E301` | Telegram API call failed | Incorrect Token, network issues | Check Token and network |
| `E302` | Webhook setup failed | URL unreachable, incorrect Secret | Check Webhook config |
| `E303` | Message send failed | User blocked, message too long | Check user status and message length |
| `E304` | Callback processing failed | Callback data format error | Check callback data |
| `E305` | Invalid Chat ID | User hasn't interacted with Bot | Ask user to send /start first |
| `E306` | Polling timeout | No message received for a long time | Check network connection |
| `E307` | Message edit failed | Message doesn't exist or expired | Check message ID |
| `E308` | File upload failed | File too large or format not supported | Check file size and format |
| `E309` | Keyboard setup failed | Button data format error | Check keyboard config |
| `E310` | Telegram rate limit | Request frequency too high | Reduce sending frequency |

#### Webhook Errors (E401-E499)

| Error Code | Description | Cause | Solution |
|------------|-------------|-------|----------|
| `E401` | Webhook signature verification failed | Secret mismatch | Check Secret config |
| `E402` | Webhook parsing failed | JSON format error | Check request body format |
| `E403` | Duplicate Webhook | Idempotency key already exists | Check deduplication config |
| `E404` | Unsupported alert source | Unknown source field | Check alert format |
| `E405` | Alert fingerprint generation failed | Abnormal label data | Check alert labels |
| `E406` | VMAlert format error | Does not meet expected format | Check VMAlert config |
| `E407` | Alertmanager format error | V2 API format error | Check Alertmanager version |
| `E408` | Webhook processing timeout | Processing time too long | Optimize processing logic |
| `E409` | Webhook queue full | Too many concurrent requests | Increase queue size or rate limit |
| `E410` | Webhook retry exhausted | Multiple retries failed | Check target service status |

#### Configuration Errors (E501-E599)

| Error Code | Description | Cause | Solution |
|------------|-------------|-------|----------|
| `E501` | Configuration file not found | Incorrect path or file missing | Check config file path |
| `E502` | YAML parsing failed | Syntax error | Check YAML syntax |
| `E503` | Configuration validation failed | Value does not meet specification | Check config values |
| `E504` | Configuration hot reload failed | Runtime loading error | Check config file and restart manually |
| `E505` | Authorization policy config error | Policy syntax error | Check authorization config file |
| `E506` | Approval routing config error | Routing rule error | Check approval config file |
| `E507` | Provider config error | Provider definition incomplete | Check Provider configuration |
| `E508` | SSH config error | Incorrect key path or insufficient permissions | Check SSH configuration |
| `E509` | Desensitization config error | Regular expression error | Check desensitization config |
| `E510` | Environment variable parsing error | Malformed format | Check environment variable values |

### 1.3 Error Response Format

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

## 2. Startup Failures

### 2.1 Service Fails to Start

#### Symptom: Exits immediately after startup

**Troubleshooting Steps**:

1. Check detailed logs
   ```bash
   # Docker
   docker logs tars --tail 100

   # Systemd
   sudo journalctl -u tars -n 100

   # Running directly
   ./tars 2>&1 | tee tars.log
   ```

2. Check configuration file syntax
   ```bash
   # YAML syntax check
   python3 -c "import yaml; yaml.safe_load(open('configs/tars.yaml'))"

   # Or use yamllint
   yamllint configs/*.yaml
   ```

3. Check required environment variables
   ```bash
   # Check required variables
   env | grep TARS_POSTGRES_DSN
   env | grep TARS_MODEL_BASE_URL
   ```

**Common Causes and Solutions**:

| Cause | Error Message | Solution |
|-------|---------------|----------|
| PostgreSQL not started | `connection refused` | Start PostgreSQL service |
| DSN format error | `invalid connection string` | Check DSN format |
| Port occupied | `address already in use` | Change port or stop occupying process |
| Insufficient permissions | `permission denied` | Use sudo or fix permissions |
| Config file not found | `no such file or directory` | Create configuration file |

### 2.2 Startup Timeout

**Troubleshooting Steps**:

```bash
# Check health status of dependency services
pg_isready -h localhost -p 5432
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz

# Check database connection timeout
psql "$TARS_POSTGRES_DSN" -c "SELECT 1"

# Check Model Gateway connectivity
curl -H "Authorization: Bearer $TARS_MODEL_API_KEY" \
  "$TARS_MODEL_BASE_URL/models"
```

**Solutions**:

1. Increase startup timeout
2. Configure health check delay
3. Use Docker's `depends_on` to ensure dependencies start first

---

## 3. Database Issues

### 3.1 Database Connection Failure

#### Symptom: Logs show `connection refused` or `timeout`

**Troubleshooting Steps**:

```bash
# 1. Check PostgreSQL service status
sudo systemctl status postgresql

# 2. Test connection
psql "postgres://user:pass@localhost:5432/tars" -c "SELECT 1"

# 3. Check port listening
netstat -tlnp | grep 5432
ss -tlnp | grep 5432

# 4. Check firewall
sudo iptables -L | grep 5432
sudo ufw status

# 5. Check connection count
psql -c "SELECT count(*) FROM pg_stat_activity;"
```

**Solutions**:

| Cause | Solution |
|-------|----------|
| PostgreSQL not started | `sudo systemctl start postgresql` |
| Blocked by firewall | Open port 5432 |
| Connection limit reached | Increase `max_connections` |
| SSL mode mismatch | Adjust `sslmode` parameter |
| Insufficient user permissions | Grant database permissions |

### 3.2 Database Performance Issues

**Troubleshooting Steps**:

```sql
-- Check slow queries
SELECT query, mean_exec_time, calls, rows
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- Check locks
SELECT * FROM pg_locks WHERE NOT granted;

-- Check long transactions
SELECT * FROM pg_stat_activity
WHERE state = 'idle in transaction'
AND xact_start < NOW() - INTERVAL '5 minutes';

-- Check table size
SELECT schemaname, tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

**Solutions**:

1. Add missing indexes
2. Optimize slow queries
3. Periodically clean up expired data
4. Adjust PostgreSQL configuration parameters

### 3.3 Data Inconsistency

**Troubleshooting Steps**:

```sql
-- Check orphan records
SELECT * FROM execution_requests
WHERE session_id NOT IN (SELECT id FROM alert_sessions);

-- Check duplicate fingerprints
SELECT fingerprint, COUNT(*)
FROM alert_events
GROUP BY fingerprint
HAVING COUNT(*) > 1;

-- Check state inconsistency
SELECT * FROM alert_sessions
WHERE status = 'executing'
AND id NOT IN (SELECT session_id FROM execution_requests WHERE status = 'executing');
```

**Repair Methods**:

```sql
-- Delete orphan records (Operate with caution)
BEGIN;
DELETE FROM execution_requests
WHERE session_id NOT IN (SELECT id FROM alert_sessions);
COMMIT;

-- Fix session status
UPDATE alert_sessions
SET status = 'open'
WHERE status = 'executing'
AND updated_at < NOW() - INTERVAL '1 hour';
```

---

## 4. AI Diagnosis Issues

### 4.1 Model Call Failure

#### Symptom: Diagnosis message shows "AI analysis temporarily unavailable"

**Troubleshooting Steps**:

```bash
# 1. Check Model Gateway connectivity
curl -v "${TARS_MODEL_BASE_URL}/models" \
  -H "Authorization: Bearer ${TARS_MODEL_API_KEY}"

# 2. Check Provider status
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/config/providers/check \
  -H "Content-Type: application/json" \
  -d '{"provider_id": "primary"}'

# 3. View model-related logs
docker logs tars 2>&1 | grep -i "model\|reasoning"
```

**Common Causes and Solutions**:

| Cause | Solution |
|-------|----------|
| Invalid API Key | Check and update API Key |
| Network unreachable | Check firewall and proxy settings |
| Model quota exhausted | Wait for quota recovery or switch model |
| Malformed request format | Check prompt configuration |
| Timeout | Increase `TARS_MODEL_TIMEOUT` |

### 4.2 Poor Diagnosis Quality

**Troubleshooting Steps**:

```bash
# View desensitized context
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/sessions/{session_id} | jq .desense_map

# Check knowledge retrieval results
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/knowledge?q={query}
```

**Optimization Suggestions**:

1. Optimize Reasoning Prompt
2. Check if desensitization is excessive
3. Add more knowledge base documents
4. Adjust model parameters (temperature, max_tokens)

### 4.3 Desensitization Issues

**Troubleshooting Steps**:

```bash
# Test desensitization configuration
curl -X POST http://localhost:8081/api/v1/config/desensitization/test \
  -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"content": "password=secret123, host=prod-web-01"}'
```

**Common Issues**:

- Insufficient desensitization: Add sensitive keys or regular expressions
- Excessive desensitization: Adjust rules or use local LLM for assistance
- Recovery failure: Check if `desense_map` is complete

---

## 5. Execution Issues

### 5.1 SSH Connection Failure

#### Symptom: Execution status shows `failed`, error is `connection refused` or `timeout`

**Troubleshooting Steps**:

```bash
# 1. Check target host reachability
ping prod-web-01
nc -zv prod-web-01 22

# 2. Test SSH connection manually
ssh -i /etc/tars/id_rsa \
  -o ConnectTimeout=10 \
  -o StrictHostKeyChecking=no \
  tars@prod-web-01 "hostname"

# 3. Check SSH key permissions
ls -la /etc/tars/id_rsa
ssh-keygen -l -f /etc/tars/id_rsa

# 4. Check target host SSH service
ssh -v tars@prod-web-01

# 5. Check authorized_keys
cat ~/.ssh/authorized_keys | grep "tars@"
```

**Solutions**:

| Cause | Solution |
|-------|----------|
| Host unreachable | Check network connection |
| SSH service not started | Start sshd service |
| Incorrect key | Re-distribute public key |
| Permission error | Fix key permission to 600 |
| Host not in whitelist | Update `TARS_SSH_ALLOWED_HOSTS` |

### 5.2 Command Execution Failure

**Troubleshooting Steps**:

```bash
# 1. Check command syntax
ssh user@host "echo 'test'"  # Simple command test

# 2. Check command permissions
ssh user@host "which command"

# 3. View detailed error
ssh user@host "command 2>&1"  # Capture stderr

# 4. Check sudo permissions (if needed)
ssh user@host "sudo -l"
```

**Common Causes**:

- Command does not exist
- Insufficient permissions
- Missing environment variables
- Incorrect working directory
- Missing dependency files

### 5.3 Output Truncation

**Symptom**: Execution output is truncated

**Troubleshooting Steps**:

```bash
# Check output size
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/executions/{id} | jq .output_bytes, .output_truncated

# Check output directory
ls -lh /data/tars/output/
du -sh /data/tars/output/
```

**Solutions**:

1. Increase `TARS_EXECUTION_OUTPUT_MAX_PERSISTED_BYTES`
2. Optimize command output (use `head`, etc.)
3. Increase chunk size

---

## 6. Telegram Issues

### 6.1 Not Receiving Messages

**Troubleshooting Steps**:

```bash
# 1. Check Bot Token
curl "https://api.telegram.org/bot${TARS_TELEGRAM_BOT_TOKEN}/getMe"

# 2. Check Webhook configuration
curl "https://api.telegram.org/bot${TARS_TELEGRAM_BOT_TOKEN}/getWebhookInfo"

# 3. Test sending message
curl -X POST \
  "https://api.telegram.org/bot${TARS_TELEGRAM_BOT_TOKEN}/sendMessage" \
  -d "chat_id=${CHAT_ID}&text=Test message"

# 4. Check Outbox
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/outbox?status=failed
```

**Solutions**:

| Cause | Solution |
|-------|----------|
| Incorrect Bot Token | Update with correct Token |
| Incorrect Chat ID | Get correct Chat ID |
| Webhook URL unreachable | Ensure public accessibility |
| User blocked Bot | Ask user to check privacy settings |
| Message rate limited by Telegram | Reduce sending frequency |

### 6.2 Callback No Response

**Symptom**: No response after clicking Telegram button

**Troubleshooting Steps**:

```bash
# 1. Check Webhook reception logs
docker logs tars 2>&1 | grep -i telegram

# 2. Check callback processing
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/audit?action=telegram_callback

# 3. Check session status
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/sessions/{id}
```

**Common Issues**:

- Webhook processing timeout
- Session status changed
- Button expired

---

## 7. Webhook Issues

### 7.1 VMAlert Webhook Failure

**Troubleshooting Steps**:

```bash
# 1. Check VMAlert logs
docker logs vmalert 2>&1 | grep webhook

# 2. Test Webhook endpoint
curl -X POST http://tars:8080/api/v1/webhooks/vmalert \
  -H "Content-Type: application/json" \
  -H "X-Tars-Secret: ${TARS_VMALERT_WEBHOOK_SECRET}" \
  -d '{"receiver":"test","status":"firing","alerts":[],"version":"1"}'

# 3. Check idempotency keys
psql -d tars -c "SELECT * FROM idempotency_keys WHERE scope='webhook' ORDER BY last_seen_at DESC LIMIT 10;"
```

**Solutions**:

1. Ensure `X-Tars-Secret` is correct
2. Check alert format
3. Check for idempotency key conflicts

### 7.2 Duplicate Webhook Alerts

**Cause**:
- Incorrect idempotency key generation
- Improper VMAlert grouping configuration
- Alert fingerprint collision

**Solutions**:

```yaml
# VMAlert configuration optimization
route:
  group_by: ['alertname', 'service', 'instance']
  group_wait: 10s
  group_interval: 30s
  repeat_interval: 4h  # Increase repeat interval
```

---

## 8. Performance Issues

### 8.1 Slow Response

**Diagnostic Steps**:

```bash
# 1. Check resource usage
top
htop
free -m
df -h

# 2. Check Go runtime
curl http://localhost:8080/metrics | grep go_

# 3. Check database slow queries
# (See Section 3.2)

# 4. Check Goroutine count
curl http://localhost:8080/metrics | grep go_goroutines
```

**Optimization Suggestions**:

1. Increase CPU/Memory resources
2. Optimize database queries
3. Enable connection pooling
4. Increase caching
5. Horizontal scaling

### 8.2 Memory Leak

**Diagnostic Steps**:

```bash
# 1. Monitor memory usage
watch -n 1 'curl -s http://localhost:8080/metrics | grep "go_memstats_alloc_bytes"'

# 2. Generate Heap Profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# 3. View Goroutines
curl http://localhost:8080/debug/pprof/goroutine?debug=1
```

---

## 9. Security Related Issues

### 9.1 Unauthorized Access

**Symptom**: Logs show `permission denied` or `unauthorized`

**Troubleshooting Steps**:

```bash
# 1. Check audit logs
psql -d tars -c "SELECT * FROM audit_logs WHERE action LIKE '%denied%' ORDER BY created_at DESC LIMIT 10;"

# 2. Check Token configuration
env | grep TARS_OPS_API_TOKEN

# 3. Check authorization policy
curl -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/config/authorization
```

### 9.2 Suspicious Activity

**Symptom**: Large number of failed logins or abnormal commands

**Troubleshooting Steps**:

```bash
# 1. Count failed logins
psql -d tars -c "SELECT actor_id, COUNT(*) FROM audit_logs WHERE action='login_failed' GROUP BY actor_id;"

# 2. Check intercepted commands
psql -d tars -c "SELECT * FROM audit_logs WHERE action='command_blocked' ORDER BY created_at DESC LIMIT 10;"

# 3. Check abnormal IPs
cat /var/log/nginx/access.log | awk '{print $1}' | sort | uniq -c | sort -rn | head -20
```

---

## 10. Diagnostic Tools

### 10.1 Health Check Script

```bash
#!/bin/bash
# tars-health-check.sh

echo "=== TARS Health Check ==="

# 1. HTTP Health Check
echo -n "HTTP Health: "
if curl -fsS http://localhost:8080/healthz > /dev/null; then
    echo "OK"
else
    echo "FAILED"
fi

# 2. Readiness Check
echo -n "Readiness: "
if curl -fsS http://localhost:8080/readyz > /dev/null; then
    echo "OK"
else
    echo "FAILED"
fi

# 3. Database Connection
echo -n "Database: "
if pg_isready -q; then
    echo "OK"
else
    echo "FAILED"
fi

# 4. Memory Usage
echo "Memory Usage:"
free -h | grep Mem

# 5. Disk Usage
echo "Disk Usage:"
df -h | grep -E 'Filesystem|/data'

# 6. Active Sessions
echo -n "Active Sessions: "
curl -fsS http://localhost:8081/api/v1/sessions?status=open 2>/dev/null | jq '.items | length'

echo "=== Check Complete ==="
```

### 10.2 Log Analysis Tools

```bash
# Real-time error monitoring
docker logs -f tars 2>&1 | grep -i error

# Count error types
docker logs tars 2>&1 | grep "error" | awk -F': ' '{print $2}' | sort | uniq -c | sort -rn

# Find logs for specific session
docker logs tars 2>&1 | grep "session_id=xxx"

# Performance analysis
docker logs tars 2>&1 | grep "duration" | awk '{print $NF}' | sort -n | tail -20
```

### 10.3 Database Diagnostic Script

```bash
#!/bin/bash
# pg-diagnostics.sh

echo "=== PostgreSQL Diagnostics ==="

# Connection count
echo "Connection Count:"
psql -d tars -c "SELECT count(*), state FROM pg_stat_activity GROUP BY state;"

# Database size
echo "Database Size:"
psql -d tars -c "SELECT pg_size_pretty(pg_database_size('tars'));"

# Table sizes
echo "Table Sizes:"
psql -d tars -c "
SELECT schemaname, tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 10;
"

# Slow queries
echo "Slow Queries (Top 5):"
psql -d tars -c "
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 5;
"

echo "=== Diagnostics Complete ==="
```

### 10.4 Debug Mode

```bash
# Enable debug logs
export TARS_LOG_LEVEL=DEBUG

# Enable performance profiling
export TARS_ENABLE_PPROF=true

# Start service
./tars

# Access pprof
curl http://localhost:8080/debug/pprof/
curl http://localhost:8080/debug/pprof/heap
curl http://localhost:8080/debug/pprof/profile?seconds=30
```

---

## 11. Getting Help

### 11.1 Reporting Issues

When reporting an issue, please provide:

1. **Environment Information**
   - TARS version
   - Deployment method (Docker/Binary/Kubernetes)
   - OS version

2. **Configuration Files** (Desensitized)

3. **Log Snippets**
   - Relevant error logs
   - Time range

4. **Reproduction Steps**

5. **Attempted Solutions**

### 11.2 Community Support

- GitHub Issues: <repository-url>/issues
- Discussions: <repository-url>/discussions
- Documentation: <docs-url>

---

*This document is applicable to the TARS MVP version. Error codes and troubleshooting methods may be adjusted in future versions.*
