# TARS 可观测性面板建议

> 适用范围：后端 MVP 试点  
> 指标入口：`GET /metrics`
> Grafana 导入物： [tars-mvp-dashboard.json](../../deploy/grafana/tars-mvp-dashboard.json)

> 说明：本文偏“当前试点阶段 dashboard 建议”。若需要平台级 built-in observability 与 OpenTelemetry/exporter 方向，请参考 [20-component-observability.md](../../specs/20-component-observability.md)。

## 1. 面板目标

试点阶段建议先关注 4 类问题：

- 告警是否稳定进入 TARS
- outbox / diagnosis / 通知链路是否有失败
- 审批和执行是否出现异常或超时
- 运行时开关和 GC 是否处于预期状态

## 2. 推荐面板

### 2.1 HTTP 请求总量

用途：确认 webhook、Ops API、Telegram callback 是否持续有流量

```promql
sum by (route, code) (increase(tars_http_requests_total[5m]))
```

### 2.2 告警接入量

用途：确认 vmalert 是否正常推送，是否出现大量重复或签名错误

```promql
sum by (result) (increase(tars_alert_events_total{source="vmalert"}[5m]))
```

关注项：

- `accepted`
- `duplicated`
- `invalid_signature`
- `validation_failed`
- `internal_error`

### 2.3 Outbox 状态变化

用途：观察 diagnosis 和 knowledge ingest 是否有堆积或失败

```promql
sum by (topic, result) (increase(tars_outbox_events_total[5m]))
```

重点关注：

- `result="failed"`
- `result="replay_blocked"`

### 2.4 Telegram 通知结果

用途：确认审批消息、结果消息是否送达稳定

```promql
sum by (kind, result) (increase(tars_channel_messages_total{channel="telegram"}[5m]))
```

重点关注：

- `kind="interactive"` 审批消息
- `result="error"` 发送失败

### 2.5 Telegram callback ack

用途：确认按钮点击后 callback ack 是否正常

```promql
sum by (result) (increase(tars_channel_callbacks_total[5m]))
```

### 2.6 执行状态分布

用途：确认执行是稳定完成，还是出现大量失败 / timeout

```promql
sum by (status) (increase(tars_execution_requests_total[15m]))
```

重点关注：

- `status="failed"`
- `status="timeout"`

### 2.6.1 输出截断次数

用途：观察执行输出是否经常超过当前数据库保留阈值

```promql
increase(tars_execution_output_truncated_total[15m])
```

### 2.7 审批超时

用途：观察是不是审批链或消息触达有问题

```promql
increase(tars_approval_timeouts_total[15m])
```

### 2.8 知识检索与知识沉淀

用途：观察知识链是否命中、是否正常沉淀

```promql
sum by (result) (increase(tars_knowledge_search_total[15m]))
```

```promql
sum by (result) (increase(tars_knowledge_ingest_total[15m]))
```

### 2.9 GC 运行与删除量

用途：确认 GC worker 正常运行，没有持续报错

```promql
sum by (result) (increase(tars_gc_runs_total[1h]))
```

```promql
sum by (kind) (increase(tars_gc_deleted_total[1h]))
```

### 2.10 当前 rollout mode 与 feature flags

用途：排查“为什么这次没有进审批 / 没有执行 / 没有知识沉淀”

```promql
tars_rollout_mode_info
```

```promql
tars_feature_flag_enabled
```

## 3. 试点期间推荐告警

建议至少接这 4 条：

### 3.1 Telegram 发送失败

```promql
increase(tars_channel_messages_total{channel="telegram",result="error"}[5m]) > 0
```

### 3.2 Outbox 失败

```promql
increase(tars_outbox_events_total{result="failed"}[10m]) > 0
```

### 3.3 审批超时

```promql
increase(tars_approval_timeouts_total[15m]) > 0
```

### 3.4 GC 连续报错

```promql
increase(tars_gc_runs_total{result="error"}[30m]) > 0
```

### 3.5 执行输出频繁截断

```promql
increase(tars_execution_output_truncated_total[30m]) > 5
```

## 4. 试点观察建议

- 每次变更 rollout mode 后，先看 `tars_rollout_mode_info`
- 如果用户说没收到审批消息，先看 `tars_channel_messages_total`
- 如果 session 卡在 `analyzing`，先看 `tars_outbox_events_total`
- 如果执行一直没有回结果，先看 `tars_execution_requests_total` 和 Telegram 消息结果
