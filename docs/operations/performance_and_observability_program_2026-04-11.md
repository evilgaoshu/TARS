# 持续性能保障与观测体系改进方案

> 日期：2026-04-11
> 状态：DRAFT
> 触发事件：共享测试机 192.168.3.100 上 tars-linux-amd64-dev 进程 CPU 占用 60-160%（根因已修复）

---

## 1. 执行摘要

### 1.1 本次事件回顾

2026-04-11 发现共享测试机上 TARS 进程持续 CPU 占用 60-160%。经 systematic debugging 四阶段排查，确认双重根因并完成修复：

| 维度 | 修复前 | 修复后 |
|------|--------|--------|
| CPU% | 58.9%（采样时）/ 峰值 160% | **1.0%** |
| 累计 CPU 时间 | 3h55m（6h40m 运行期间） | 1s（2min 运行期间） |
| telegram 错误日志/10s | ~4 条 | **0 条** |
| runtime.jsonl 增长速率 | ~4 行/10s（不间断） | **0**（稳态） |

### 1.2 根因

**根因 1（核心）：`observability/store.go` append() 方法 O(N²) 全量文件扫描**

每次 `AppendLog()` 或 `AppendEvent()` 调用时，`append()` 会：
1. 写入 1 行 JSONL（O(1)）
2. `applyGovernanceLocked()` → `readRecords()` 全量读取 + JSON 反序列化 + sort（O(N)）
3. `rebuildSummaryLocked()` → `readRecords()` × 2（logs + traces 各一次）= O(N) × 2

每次 append = 3 次全量扫描。当 runtime.jsonl 达到 55K 行 / 23.5MB 时，单次扫描耗时 ~2.75s，三次 ~8s。写入间隔 2.4s < 扫描时间 → CPU 持续饱和。

**根因 2（放大器）：telegram 占位符 token 触发无限轮询失败**

`TARS_TELEGRAM_BOT_TOKEN=REPLACE_WITH_TELEGRAM_BOT_TOKEN`（占位符），但 `TARS_TELEGRAM_POLLING_ENABLED=true`。`StartPolling()` 只检查 token 是否为空字符串，不检查占位符。每 ~2.4s 产生一条 error 日志 → 触发 `store.AppendLog()` → 触发全量扫描。93.7% 的日志是 telegram 404 错误。

### 1.3 修复清单

| # | 文件 | 改动 | 效果 |
|---|------|------|------|
| 1 | `internal/foundation/observability/store.go` | append() 改为 O(1) 增量更新 + 定期 governance（5min 间隔） | CPU 从 ~60% 降至 ~1% |
| 2 | `internal/modules/channel/telegram/service.go` | 占位符 token 检测 + 指数退避（最大 60s） | 消除无效 HTTP 请求和错误日志洪流 |

---

## 2. 内置观测系统改进

### 2.1 当前架构问题

| 问题 | 位置 | 严重度 | 状态 |
|------|------|--------|------|
| append() 每次写入全量扫描 | `store.go:168` | P0 | **已修复** |
| queryFile() 全量读取 + 反序列化 | `store.go:399` | P1 | 待优化 |
| rebuildSummaryLocked() 双文件全量读取 | `store.go:340` | P1 | **已缓解**（仅定期运行） |
| API 查询 readRecords() 全量扫描 | `observability_handler.go` | P2 | 待优化 |
| 无文件轮转机制 | store.go | P2 | 待实现 |
| Summary 计数器 24h 窗口不精确（增量模式下只加不减） | `store.go:368` | P3 | 可接受（活跃写入时约 5 分钟校准，低流量时由每小时 retention worker 校准） |

### 2.2 短期改进（P1，Sprint 内）

1. **queryFile() 流式读取 + 提前终止**：对于有 limit 的查询，从文件尾部反向扫描而不是全量读取后反向遍历
2. **JSONL 文件轮转**：按日期或大小自动轮转（如 runtime-2026-04-11.jsonl），governance 只扫描当前文件
3. **启动时异步 rebuild**：`NewStore()` 中的 `rebuildSummary()` 改为异步执行，避免阻塞启动

### 2.3 中期改进（P2，2-4 周）

1. **索引 / 倒排索引**：为高频查询字段（timestamp, level, component）维护内存索引
2. **分页查询**：API 层支持 cursor-based pagination，避免大结果集
3. **压缩存储**：JSONL → 压缩 JSONL 或 columnar 格式（如 Parquet）用于归档

---

## 3. Metrics 体系

### 3.1 现有 Metrics

TARS 已通过 `internal/foundation/metrics/metrics.go` 暴露 Prometheus 格式指标至 `/metrics` endpoint，由 VictoriaMetrics 采集。

当前关键指标：
- `tars_dispatcher_cycles_total{result}` — 调度器轮询计数
- `tars_observability_logs_total{component,level}` — 日志计数
- `tars_observability_events_total{component}` — 事件计数
- `tars_channel_messages_total{channel,kind,status}` — 通道消息计数
- `tars_http_requests_total{method,path,status}` — HTTP 请求计数
- `tars_http_request_duration_seconds` — HTTP 请求延迟直方图

### 3.2 需新增的 Metrics（P1）

| 指标名 | 类型 | 用途 |
|--------|------|------|
| `tars_observability_store_append_duration_seconds` | Histogram | 追踪 append 全路径延迟，覆盖写入、周期性 governance 与 summary rebuild 的同步尖峰 |
| `tars_observability_store_governance_duration_seconds` | Histogram | governance 全量扫描的耗时 |
| `tars_observability_store_file_bytes{signal}` | Gauge | 各 JSONL 文件大小，用于容量告警 |
| `tars_observability_store_records_total{signal}` | Gauge | 各信号类型的记录数 |
| `tars_process_cpu_seconds_total` | Counter | 进程级 CPU 使用（Go runtime 可暴露） |
| `tars_process_resident_memory_bytes` | Gauge | RSS 内存 |
| `tars_go_goroutines` | Gauge | goroutine 数量 |

### 3.3 SLI / SLO 定义

| SLI | 计算方式 | SLO 目标 |
|-----|----------|----------|
| API 可用性 | `healthz` + `readyz` 200 响应率 | ≥ 99.5%（测试环境） |
| API P95 延迟 | `tars_http_request_duration_seconds` P95 | ≤ 500ms |
| 观测写入延迟 | `tars_observability_store_append_duration_seconds` P99 | ≤ 10ms |
| Governance 延迟 | `tars_observability_store_governance_duration_seconds` P99 | ≤ 5s |
| 错误日志比率 | `rate(tars_observability_logs_total{level="error"})` / total | ≤ 5% |
| 进程 CPU 使用 | `rate(process_cpu_seconds_total[5m])` | ≤ 0.3（30% 单核） |

---

## 4. Logs 体系

### 4.1 当前架构

- **结构化日志**：使用 `log/slog` 输出 JSON 格式到 stdout / file
- **内置存储**：`observability/store.go` 写入 JSONL 文件
- **外部推送**：OTLP exporter 可选推送到 VictoriaLogs（`127.0.0.1:9428`）
- **文件日志**：`tars-dev.log`（服务主日志），`runtime.jsonl`（结构化观测日志）

### 4.2 改进项

| 改进 | 优先级 | 描述 |
|------|--------|------|
| 日志采样 | P1 | 对高频重复日志（如 dispatcher idle cycles）实施采样，减少写入量 |
| 日志级别动态调整 | P2 | 通过 API 动态调整组件日志级别，无需重启 |
| 日志上下文丰富 | P2 | 自动注入 trace_id、span_id、request_id |
| 敏感信息脱敏 | P1 | 自动检测并脱敏 token、password 等字段 |

---

## 5. Traces 体系

### 5.1 当前状态

- 内置 event 追踪通过 `observability/audit_logger.go` 写入 `events.jsonl`
- 支持 trace_id、session_id、execution_id 关联
- OTLP traces exporter 可选启用

### 5.2 改进项

| 改进 | 优先级 | 描述 |
|------|--------|------|
| 端到端 trace 串联 | P2 | 从 HTTP 请求 → 调度器 → agent 执行 → connector 调用的完整 trace |
| 采样策略 | P2 | 基于错误率的自适应采样 |
| 延迟热图 | P3 | 在观测 dashboard 中展示延迟分布热图 |

---

## 6. Event Profiling

### 6.1 dispatcher 轮询优化（P2）

当前 dispatcher 以 200ms 固定间隔轮询数据库，空闲时每秒 5 次 DB 查询。

改进方案：
- 引入自适应轮询：空闲时指数退避至 5s，有任务时恢复 200ms
- 新增 `tars_dispatcher_poll_interval_seconds` gauge 追踪当前轮询间隔
- 长期：改为事件驱动（DB LISTEN/NOTIFY 或内存 channel）

### 6.2 connector 健康检测

- 定期探测各 connector 端点可达性
- 记录连接延迟和失败率
- 在 dashboard 中展示 connector 健康状态时间线

---

## 7. 研发流程集成

### 7.1 CI/CD 性能门禁

| 阶段 | 检查项 | 阈值 |
|------|--------|------|
| PR 构建 | `go vet` + `go test -race` | 零错误 |
| PR 构建 | benchmark 基线对比 | 回归 ≤ 10% |
| 部署前 | smoke-remote.sh | healthz/readyz/discovery 全通过 |
| 部署后 | 5min CPU 采样 | ≤ 30% 单核 |
| 部署后 | live-validate.sh | 核心验证通过 |

### 7.2 性能测试规范

1. **基准测试**：为 `store.go` 的 `append()`、`queryFile()`、`rebuildSummaryLocked()` 编写 `Benchmark*` 测试
2. **压力测试**：模拟高频写入场景（1000 writes/s），验证 CPU 和内存稳定性
3. **长稳测试**：72h 持续运行，监控 CPU/MEM/FD/goroutine 趋势

---

## 8. 数据保留策略

### 8.1 当前配置

| 信号类型 | 保留时间 | 最大大小 | 文件路径 |
|----------|----------|----------|----------|
| Logs | 配置项 `TARS_OBSERVABILITY_LOGS_RETENTION` | 配置项 `TARS_OBSERVABILITY_LOGS_MAX_SIZE_BYTES` | `data/observability/logs/runtime.jsonl` |
| Traces | 配置项 `TARS_OBSERVABILITY_TRACES_RETENTION` | 配置项同上 | `data/observability/traces/events.jsonl` |
| Metrics | 配置项同上 | 配置项同上 | `data/observability/metrics/snapshots.jsonl` |

### 8.2 推荐默认值

| 信号 | 保留 | 最大大小 | 理由 |
|------|------|----------|------|
| Logs | 72h | 50MB | 平衡诊断需求与磁盘开销 |
| Traces | 168h (7d) | 100MB | 事件量较小，保留更长 |
| Metrics snapshots | 720h (30d) | 20MB | 趋势分析需要更长窗口 |

### 8.3 外部存储

- VictoriaMetrics：长期 metrics 存储（推荐保留 90d）
- VictoriaLogs：长期日志存储（推荐保留 30d）
- 内置 JSONL：短期热数据 + 应急诊断

---

## 9. 执行路线图

### Phase 1: 紧急修复（已完成 ✅）

- [x] store.go append() O(1) 增量更新
- [x] telegram 占位符 token 检测 + 指数退避
- [x] 共享机部署验证（CPU 1.0%）
- [x] smoke-remote.sh 通过

### Phase 2: 短期加固（1-2 周）

- [x] 为 store.go 编写 benchmark 测试（`BenchmarkAppend`, `BenchmarkQuery`, `BenchmarkRebuildSummary`）
- [x] 为 store.go 编写单元测试（覆盖增量 summary、governance 校准、低流量重建、跨轮转查询、retention/rotation 边界）
- [x] 新增 `tars_observability_store_*` metrics
- [x] queryFile() 优化（limit 优先走尾部反向扫描，异常输入回退全量路径）
- [x] JSONL 文件轮转（按日期）
- [x] 日志采样（telegram `getUpdates failed` 高频重复日志）
- [x] CI / 共享机验证链路增加部署后 CPU spot-check（超阈值 fail）

### Phase 3: 中期完善（2-4 周）

- [ ] SLI/SLO 仪表盘（VictoriaMetrics + Grafana）
- [ ] dispatcher 自适应轮询
- [ ] 端到端 trace 串联
- [ ] 动态日志级别调整 API
- [ ] 进程级 metrics 暴露（CPU/MEM/goroutine）

### Phase 4: 长期演进（1-3 月）

- [ ] 内存索引加速查询
- [ ] 日志/事件压缩归档
- [ ] 分布式部署下的观测聚合
- [ ] 告警规则引擎（基于 SLO 违约自动告警）

---

## 10. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| governance 5min 间隔内文件超限 | 磁盘空间突增 | `RunRetention()` 仍可手动/定时触发；日志采样减少写入量 |
| 增量 Summary 24h 计数器漂移 | Dashboard 数字不精确 | 活跃写入时由 5 分钟 governance 校准；低流量时由每小时 retention worker 全量校准，偏差量级可控 |
| JSONL 文件损坏（断电/crash） | 部分记录丢失 | JSONL 天然容错（逐行追加）；readRecords() 已跳过解析失败行 |
| 外部观测栈不可用 | 丢失长期数据 | 内置存储作为降级方案；connector 健康检测告警 |
| live-validate 401 token 问题 | 部署验证不完整 | 需排查 ops token 配置（独立于本修复） |

---

## 附录 A: 修复前后对比证据

```
# 修复前 (2026-04-11T19:44 CST)
PID 81435  %CPU=58.9  ELAPSED=06:40:19  TIME=03:55:56
runtime.jsonl: 55,207 行, 23.5MB (持续增长 ~4行/10s)
telegram error 日志: 9,867 条 (93.7% of all logs)

# 修复后 (2026-04-11T19:49 CST)
PID 87493  %CPU=1.0   ELAPSED=00:02:05  TIME=00:00:01
runtime.jsonl: 55,370 行, 23MB (不再增长)
telegram 日志: 1 条 WARN (占位符检测)
```

## 附录 B: 修改文件清单

| 文件 | 行号 | 改动摘要 |
|------|------|----------|
| `internal/foundation/observability/store.go` | 109-122 | 新增 `governanceInterval` 常量 + `lastGovernance` 字段 |
| `internal/foundation/observability/store.go` | 143 | `NewStore()` 初始化 `lastGovernance` |
| `internal/foundation/observability/store.go` | 168-199 | `append()` 改为 O(1) 增量更新 + 定期 governance |
| `internal/foundation/observability/store_test.go` | 新增 | 覆盖增量更新、governance 校准、低流量 summary 重建、轮转/retention/query 边界、metrics 暴露 |
| `internal/foundation/observability/store_benchmark_test.go` | 新增 | 覆盖 append/query/rebuildSummary 热路径 benchmark |
| `internal/modules/channel/telegram/service.go` | 356-438 | `StartPolling()` 添加占位符检测 + 指数退避 |
| `internal/modules/channel/telegram/service.go` | 590-631 | 新增 `isPlaceholderToken()` + `computeBackoff()` |
| `scripts/ci/performance-spot-check.sh` | 新增 | 共享机部署后自动收集 CPU / goroutine / store metrics / observability 文件大小证据 |
| `scripts/ci/smoke-remote.sh` | 更新 | 默认串联 performance spot-check |

## 附录 C: 共享机验证命令与结果

```bash
# 部署
TARS_REMOTE_USER=root bash scripts/deploy_team_shared.sh

# Smoke 验证
TARS_REMOTE_USER=root bash scripts/ci/smoke-remote.sh
# 结果: healthz=ok, readyz=ready, discovery=ok (8 connectors)

# CPU 验证
ssh root@192.168.3.100 'ps -p $(pgrep -f tars-linux-amd64-dev) -o pid,pcpu,etime,cputime'
# 结果: %CPU=1.0, ELAPSED=02:05, TIME=00:00:01
```
