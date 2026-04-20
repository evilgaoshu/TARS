# TARS 性能调优指南

> **版本**: v1.0
> **适用版本**: TARS MVP (Phase 1)
> **最后更新**: 2026-03-13

---

## 目录

1. [性能概述](#1-性能概述)
2. [基准测试](#2-基准测试)
3. [数据库优化](#3-数据库优化)
4. [Go 运行时优化](#4-go-运行时优化)
5. [缓存策略](#5-缓存策略)
6. [并发优化](#6-并发优化)
7. [内存优化](#7-内存优化)
8. [网络优化](#8-网络优化)
9. [水平扩展](#9-水平扩展)
10. [监控和告警](#10-监控和告警)

---

## 1. 性能概述

### 1.1 性能指标

| 指标 | 目标值 | 说明 |
|------|--------|------|
| API 响应时间 | P99 < 100ms | 不包括模型调用 |
| 告警处理延迟 | P99 < 5s | 从接收到诊断完成 |
| 命令执行延迟 | P99 < 30s | 包括审批和执行 |
| 并发会话数 | > 1000 | 同时活跃会话 |
| 执行吞吐率 | > 100/min | 每分钟执行数 |
| 数据库连接数 | < 100 | 最大连接数 |

### 1.2 性能瓶颈识别

```
┌─────────────────────────────────────────────────────────────┐
│                     性能瓶颈分析                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐ │
│  │   Webhook    │    │     AI       │    │    SSH       │ │
│  │   接收       │    │   诊断       │    │   执行       │ │
│  │  < 10ms      │    │  1-5s        │    │  5-30s       │ │
│  └──────────────┘    └──────────────┘    └──────────────┘ │
│         │                   │                   │          │
│         ▼                   ▼                   ▼          │
│  ┌──────────────────────────────────────────────────────┐ │
│  │                    数据库 I/O                         │ │
│  │               可能成为瓶颈                            │ │
│  └──────────────────────────────────────────────────────┘ │
│                                                             │
└─────────────────────────────────────────────────────────────┘

主要瓶颈排序：
1. AI 模型调用 (1-5s)
2. SSH 命令执行 (5-30s)
3. 数据库查询 (1-100ms)
4. Webhook 处理 (< 10ms)
```

---

## 2. 基准测试

### 2.1 负载测试

#### 使用 k6 进行负载测试

```javascript
// load-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '2m', target: 100 },   // 逐渐增加到 100 用户
    { duration: '5m', target: 100 },   // 保持 100 用户
    { duration: '2m', target: 200 },   // 增加到 200 用户
    { duration: '5m', target: 200 },   // 保持 200 用户
    { duration: '2m', target: 0 },     // 逐渐下降
  ],
  thresholds: {
    http_req_duration: ['p(95)<200'],   // 95% 请求 < 200ms
    http_req_failed: ['rate<0.1'],      // 错误率 < 10%
  },
};

const BASE_URL = 'http://localhost:8080';
const OPS_TOKEN = __ENV.OPS_API_TOKEN;

export default function () {
  const res = http.get(`${BASE_URL}/api/v1/sessions`, {
    headers: {
      'Authorization': `Bearer ${OPS_TOKEN}`,
    },
  });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 200ms': (r) => r.timings.duration < 200,
  });

  sleep(1);
}
```

运行测试：

```bash
k6 run load-test.js
```

#### 结果分析

```
     data_received..................: 1.2 MB  4.0 kB/s
     data_sent......................: 450 kB  1.5 kB/s
     http_req_blocked...............: avg=12.3µs min=1.2µs  med=5.1µs  max=1.2ms
     http_req_connecting............: avg=8.5µs  min=0s     med=0s     max=1.1ms
     http_req_duration..............: avg=85.2ms min=12.1ms med=78.3ms max=245.6ms
       { expected_response:true }...: avg=85.2ms min=12.1ms med=78.3ms max=245.6ms
     http_req_failed................: 0.00%   ✓ 0    ✗ 10000
     http_req_receiving.............: avg=45.2µs min=8.1µs  med=32.1µs max=2.3ms
     http_req_sending...............: avg=15.6µs min=3.2µs  med=12.1µs max=1.1ms
     http_req_waiting...............: avg=85.1ms min=12.0ms med=78.2ms max=245.5ms
     http_reqs......................: 10000   33.33/s
```

### 2.2 压力测试

```javascript
// stress-test.js
import http from 'k6/http';
import { check } from 'k6';

export const options = {
  stages: [
    { duration: '2m', target: 100 },
    { duration: '5m', target: 100 },
    { duration: '2m', target: 400 },   // 超过正常负载
    { duration: '5m', target: 400 },
    { duration: '10m', target: 0 },    // 恢复阶段
  ],
};

export default function () {
  // 模拟完整告警处理流程
  // 1. 发送告警
  // 2. 查询会话
  // 3. 获取执行
}
```

### 2.3 持续性能测试

```bash
# 使用 wrk 进行持续测试
wrk -t12 -c400 -d30s \
  -H "Authorization: Bearer ${OPS_API_TOKEN}" \
  http://localhost:8081/api/v1/sessions

# 使用 vegeta
echo "GET http://localhost:8081/api/v1/sessions" | \
  vegeta attack -rate 100 -duration 30s | \
  vegeta report
```

---

## 3. 数据库优化

### 3.1 PostgreSQL 配置

#### 推荐配置

```conf
# /etc/postgresql/14/main/postgresql.conf

# 连接
max_connections = 200
superuser_reserved_connections = 3

# 内存
shared_buffers = 4GB                    # 25% of RAM
effective_cache_size = 12GB             # 75% of RAM
work_mem = 64MB                         # per operation
maintenance_work_mem = 1GB              # maintenance operations

# WAL
checkpoint_completion_target = 0.9
wal_buffers = 16MB
max_wal_size = 4GB
min_wal_size = 1GB

# 查询优化
random_page_cost = 1.1                  # SSD
effective_io_concurrency = 200          # SSD
max_worker_processes = 8
max_parallel_workers_per_gather = 4
max_parallel_workers = 8

# 日志
log_min_duration_statement = 1000       # log slow queries
log_checkpoints = on
log_connections = on
log_lock_waits = on
log_temp_files = 0
```

#### 连接池配置

```yaml
# configs/tars.yaml
postgres:
  dsn: "postgres://user:pass@host:5432/tars?sslmode=disable&pool_max_conns=50&pool_min_conns=10"
```

### 3.2 索引优化

#### 核心索引

```sql
-- 已有索引（ migrations 中创建）
CREATE INDEX idx_alert_events_fingerprint ON alert_events (fingerprint);
CREATE INDEX idx_alert_sessions_status ON alert_sessions (status);
CREATE INDEX idx_execution_requests_session_id ON execution_requests (session_id);

-- 额外性能索引
CREATE INDEX CONCURRENTLY idx_alert_sessions_updated_at
ON alert_sessions (updated_at DESC);

CREATE INDEX CONCURRENTLY idx_execution_requests_created_at
ON execution_requests (created_at DESC);

-- 复合索引
CREATE INDEX CONCURRENTLY idx_sessions_status_updated
ON alert_sessions (status, updated_at DESC)
WHERE status IN ('open', 'analyzing', 'pending_approval');

-- 部分索引（只索引活跃数据）
CREATE INDEX CONCURRENTLY idx_pending_executions
ON execution_requests (created_at)
WHERE status = 'pending';
```

#### 索引维护

```bash
# 查看索引使用情况
psql -d tars -c "
SELECT schemaname, tablename, indexname, idx_scan, idx_tup_read
FROM pg_stat_user_indexes
WHERE schemaname = 'public'
ORDER BY idx_scan DESC;
"

# 重建索引（低峰期执行）
psql -d tars -c "REINDEX INDEX CONCURRENTLY idx_alert_sessions_updated_at;"

# 分析表
psql -d tars -c "ANALYZE alert_sessions;"
psql -d tars -c "ANALYZE execution_requests;"
```

### 3.3 查询优化

#### 慢查询分析

```sql
-- 查看慢查询
SELECT query, calls, mean_exec_time, total_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- 重置统计
SELECT pg_stat_statements_reset();
```

#### 优化示例

```go
// 优化前：N+1 查询
func GetSessionsWithExecutions(ctx context.Context) ([]Session, error) {
    sessions, _ := repo.ListSessions(ctx)
    for _, s := range sessions {
        executions, _ := repo.ListExecutions(ctx, s.ID) // N 次查询
        s.Executions = executions
    }
}

// 优化后：JOIN 查询
func GetSessionsWithExecutions(ctx context.Context) ([]Session, error) {
    query := `
        SELECT s.*, e.*
        FROM alert_sessions s
        LEFT JOIN execution_requests e ON s.id = e.session_id
        WHERE s.status IN ('open', 'analyzing')
        ORDER BY s.updated_at DESC
        LIMIT 100
    `
    // 单次查询
}
```

### 3.4 分区表

```sql
-- 审计日志分区（按时间）
CREATE TABLE audit_logs (
    id BIGSERIAL,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    trace_id TEXT,
    actor_id TEXT,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    action TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- 创建分区
CREATE TABLE audit_logs_2026_03 PARTITION OF audit_logs
FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE audit_logs_2026_04 PARTITION OF audit_logs
FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
```

---

## 4. Go 运行时优化

### 4.1 GC 调优

```go
// 环境变量调优
// 减少 GC 频率，增加内存使用
export GOGC=200        # 默认 100，堆增长 100% 触发 GC
export GOMEMLIMIT=4GiB # 软内存限制
```

#### 运行时配置

```go
// internal/foundation/config/config.go

// 设置 GOMAXPROCS
func init() {
    // 自动检测 CPU 核心数
    // 或使用 ulimit 限制
}
```

### 4.2 Goroutine 优化

#### Worker Pool 模式

```go
// internal/app/workers.go
type WorkerPool struct {
    workers int
    queue   chan func()
}

func NewWorkerPool(workers int) *WorkerPool {
    pool := &WorkerPool{
        workers: workers,
        queue:   make(chan func(), 1000),
    }

    for i := 0; i < workers; i++ {
        go pool.worker()
    }

    return pool
}

func (p *WorkerPool) worker() {
    for task := range p.queue {
        task()
    }
}

func (p *WorkerPool) Submit(task func()) {
    p.queue <- task
}
```

#### Goroutine 数量监控

```go
// 暴露 Goroutine 数量指标
import "runtime"

func init() {
    go func() {
        for {
            n := runtime.NumGoroutine()
            metrics.GaugeSet("goroutines", float64(n))
            time.Sleep(10 * time.Second)
        }
    }()
}
```

### 4.3 内存分配优化

#### 对象池

```go
// internal/foundation/pool/buffer.go
import "sync"

var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 4096)
    },
}

func GetBuffer() []byte {
    return bufferPool.Get().([]byte)
}

func PutBuffer(b []byte) {
    if cap(b) <= 4096 {
        bufferPool.Put(b)
    }
}
```

#### 预分配切片

```go
// 优化前
var results []Result
for _, item := range items {
    results = append(results, process(item)) // 多次扩容
}

// 优化后
results := make([]Result, 0, len(items)) // 预分配容量
for _, item := range items {
    results = append(results, process(item))
}
```

### 4.4 HTTP 优化

```go
// internal/api/http/server.go

import (
    "net/http"
    "time"
)

func NewServer() *http.Server {
    return &http.Server{
        Addr:         ":8080",
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  120 * time.Second,
        MaxHeaderBytes: 1 << 20, // 1MB
    }
}
```

---

## 5. 缓存策略

### 5.1 配置缓存

```go
// internal/foundation/config/cache.go

type ConfigCache struct {
    mu      sync.RWMutex
    configs map[string]interface{}
    ttl     time.Duration
}

func (c *ConfigCache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    val, ok := c.configs[key]
    return val, ok
}

func (c *ConfigCache) Set(key string, val interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.configs[key] = val
}
```

### 5.2 会话缓存

```go
// internal/modules/workflow/cache.go

type SessionCache struct {
    cache *lru.Cache // github.com/hashicorp/golang-lru
}

func NewSessionCache(size int) *SessionCache {
    cache, _ := lru.New(size)
    return &SessionCache{cache: cache}
}

func (c *SessionCache) Get(sessionID string) (*Session, bool) {
    val, ok := c.cache.Get(sessionID)
    if !ok {
        return nil, false
    }
    return val.(*Session), true
}
```

### 5.3 Redis 缓存（可选）

```go
import "github.com/redis/go-redis/v9"

var redisClient *redis.Client

func initRedis() {
    redisClient = redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
        PoolSize: 100,
    })
}

func CacheSession(ctx context.Context, session *Session) error {
    data, _ := json.Marshal(session)
    return redisClient.Set(ctx,
        "session:"+session.ID,
        data,
        5*time.Minute,
    ).Err()
}
```

---

## 6. 并发优化

### 6.1 连接池

#### HTTP 连接池

```go
// 复用 HTTP 连接
var httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 100,
        MaxConnsPerHost:     100,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
    },
    Timeout: 30 * time.Second,
}
```

#### PostgreSQL 连接池

```go
// 使用 pgx 连接池
import "github.com/jackc/pgx/v5/pgxpool"

poolConfig, _ := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
poolConfig.MaxConns = 50
poolConfig.MinConns = 10
poolConfig.MaxConnLifetime = time.Hour
poolConfig.MaxConnIdleTime = 30 * time.Minute

pool, _ := pgxpool.NewWithConfig(context.Background(), poolConfig)
```

### 6.2 异步处理

```go
// internal/events/dispatcher.go

type EventDispatcher struct {
    handlers map[string][]EventHandler
}

func (d *EventDispatcher) Dispatch(ctx context.Context, event Event) {
    // 异步处理事件
    handlers := d.handlers[event.Type]
    for _, handler := range handlers {
        go func(h EventHandler) {
            if err := h.Handle(ctx, event); err != nil {
                log.Error("event handler failed", "error", err)
            }
        }(handler)
    }
}
```

### 6.3 限流和熔断

```go
// internal/api/http/middleware/ratelimit.go

import "golang.org/x/time/rate"

type RateLimiter struct {
    limiter *rate.Limiter
}

func NewRateLimiter(rps int) *RateLimiter {
    return &RateLimiter{
        limiter: rate.NewLimiter(rate.Limit(rps), rps*2),
    }
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        if !rl.limiter.Allow() {
            c.AbortWithStatus(http.StatusTooManyRequests)
            return
        }
        c.Next()
    }
}
```

---

## 7. 内存优化

### 7.1 内存分析

```bash
# 生成 Heap Profile
curl http://localhost:8080/debug/pprof/heap > heap.prof

# 分析
 go tool pprof heap.prof

# 查看 Top 10
go tool pprof -top heap.prof

# 查看分配
 go tool pprof -alloc_space heap.prof

# 生成 SVG 图
 go tool pprof -svg heap.prof > heap.svg
```

### 7.2 内存优化技巧

#### 避免字符串拼接

```go
// 优化前
var result string
for _, s := range strings {
    result += s // 多次分配
}

// 优化后
var builder strings.Builder
builder.Grow(totalSize) // 预分配
for _, s := range strings {
    builder.WriteString(s)
}
result := builder.String()
```

#### 复用缓冲区

```go
// 使用 bytes.Buffer
var buf bytes.Buffer
buf.Grow(4096)

// 执行命令时使用
buf.Reset()
buf.Write(output)
```

#### 及时释放

```go
// 大对象使用后及时释放
func ProcessLargeData() {
    data := make([]byte, 100*1024*1024) // 100MB
    // 处理...

    // 处理完成后
    data = nil // 帮助 GC
}
```

---

## 8. 网络优化

### 8.1 TCP 优化

```bash
# 系统参数
# /etc/sysctl.conf

# 增加连接跟踪表
net.netfilter.nf_conntrack_max = 1000000

# TCP 优化
net.ipv4.tcp_max_syn_backlog = 65536
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 30
net.ipv4.tcp_keepalive_time = 1200
net.ipv4.tcp_max_tw_buckets = 5000

# 缓冲区
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216

# 应用
sysctl -p
```

### 8.2 Keep-Alive

```go
// 启用 HTTP Keep-Alive
client := &http.Client{
    Transport: &http.Transport{
        DisableKeepAlives: false, // 启用
        MaxIdleConns:      100,
        IdleConnTimeout:   90 * time.Second,
    },
}
```

---

## 9. 水平扩展

### 9.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                        负载均衡器                            │
│                       (Nginx/HAProxy)                        │
└─────────────────────────────────────────────────────────────┘
                             │
            ┌────────────────┼────────────────┐
            │                │                │
     ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐
     │  TARS-1     │  │  TARS-2     │  │  TARS-3     │
     │  :8080      │  │  :8080      │  │  :8080      │
     └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
            │                │                │
            └────────────────┼────────────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
       ┌──────┴──────┐             ┌──────┴──────┐
       │ PostgreSQL  │             │   Redis     │
       │  (主从)      │             │  (可选)      │
       └─────────────┘             └─────────────┘
```

### 9.2 无状态设计

```go
// 避免本地状态
// 使用数据库存储会话状态
type SessionRepo struct {
    db *sql.DB
}

func (r *SessionRepo) Get(ctx context.Context, id string) (*Session, error) {
    // 从数据库获取
    // 任何实例都可以处理
}
```

### 9.3 会话亲和性（可选）

```nginx
# Nginx 配置
upstream tars {
    ip_hash;  # 基于 IP 的哈希
    server tars-1:8080;
    server tars-2:8080;
    server tars-3:8080;
}

server {
    listen 80;
    location / {
        proxy_pass http://tars;
    }
}
```

### 9.4 数据库读写分离

```go
// 主从配置
type DBCluster struct {
    Master *sql.DB
    Slaves []*sql.DB
}

func (c *DBCluster) Query(ctx context.Context, query string) (*sql.Rows, error) {
    // 读操作使用从库
    slave := c.Slaves[rand.Intn(len(c.Slaves))]
    return slave.QueryContext(ctx, query)
}

func (c *DBCluster) Exec(ctx context.Context, query string) (sql.Result, error) {
    // 写操作使用主库
    return c.Master.ExecContext(ctx, query)
}
```

---

## 10. 监控和告警

### 10.1 性能指标

```go
// internal/foundation/metrics/metrics.go

var (
    // 请求延迟
    RequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "tars_request_duration_seconds",
            Help:    "Request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path", "status"},
    )

    // 数据库查询
    DBQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "tars_db_query_duration_seconds",
            Help:    "Database query duration",
            Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
        },
        []string{"table", "operation"},
    )

    // Goroutine 数量
    Goroutines = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "tars_goroutines",
            Help: "Number of goroutines",
        },
    )

    // 内存使用
    MemoryUsage = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "tars_memory_bytes",
            Help: "Memory usage in bytes",
        },
    )
)
```

### 10.2 性能告警规则

```yaml
# prometheus-rules.yml
groups:
  - name: tars_performance
    rules:
      # API 延迟告警
      - alert: TARSHighLatency
        expr: histogram_quantile(0.99, rate(tars_request_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "TARS API latency is high"

      # 数据库慢查询
      - alert: TARSSlowDBQueries
        expr: rate(tars_db_query_duration_seconds_count[5m]) > 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "TARS database queries are slow"

      # 内存使用
      - alert: TARSHighMemory
        expr: tars_memory_bytes / tars_memory_limit_bytes > 0.85
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "TARS memory usage is high"

      # Goroutine 数量
      - alert: TARSHighGoroutines
        expr: tars_goroutines > 10000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "TARS goroutine count is high"

      # 执行队列堆积
      - alert: TARSExecutionBacklog
        expr: tars_execution_queue_size > 100
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "TARS execution queue is backing up"
```

### 10.3 Grafana Dashboard

```json
{
  "dashboard": {
    "title": "TARS Performance",
    "panels": [
      {
        "title": "Request Duration (P99)",
        "targets": [
          {
            "expr": "histogram_quantile(0.99, rate(tars_request_duration_seconds_bucket[5m]))",
            "legendFormat": "{{method}} {{path}}"
          }
        ]
      },
      {
        "title": "Database Query Duration",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(tars_db_query_duration_seconds_bucket[5m]))",
            "legendFormat": "{{table}}"
          }
        ]
      },
      {
        "title": "Memory Usage",
        "targets": [
          {
            "expr": "tars_memory_bytes",
            "legendFormat": "Used"
          },
          {
            "expr": "tars_memory_limit_bytes",
            "legendFormat": "Limit"
          }
        ]
      }
    ]
  }
}
```

---

## 11. 性能测试报告模板

```markdown
# TARS 性能测试报告

## 测试环境
- 版本: 1.0.0
- 部署: Docker Compose
- CPU: 4 cores
- 内存: 8GB
- 数据库: PostgreSQL 14

## 测试场景
- 并发用户: 100-500
- 持续时间: 30 分钟
- 测试类型: 负载测试

## 结果
| 指标 | 目标 | 实际 | 状态 |
|------|------|------|------|
| P99 响应时间 | < 200ms | 150ms | ✅ |
| 错误率 | < 1% | 0.1% | ✅ |
| 吞吐量 | > 50 RPS | 80 RPS | ✅ |
| CPU 使用率 | < 70% | 65% | ✅ |
| 内存使用 | < 4GB | 3.2GB | ✅ |

## 瓶颈分析
1. 数据库连接池在高并发下成为瓶颈
2. AI 模型调用延迟影响整体响应时间

## 优化建议
1. 增加数据库连接池大小
2. 实施 AI 诊断结果缓存
3. 考虑水平扩展

## 结论
系统满足性能要求，建议在生产环境部署。
```

---

*本文档适用于 TARS MVP 版本，优化建议可能会随版本更新调整。*
