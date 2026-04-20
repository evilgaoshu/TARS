# TARS — Observability 规范

> **状态**: 设计基线
> **适用范围**: built-in observability summary、metrics / trace / event 聚合、对外标准暴露
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-logging.md](./20-component-logging.md)、[40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Observability 是什么

`Observability` 是 TARS 的 **平台健康与观测摘要域**。

它回答：

- 平台自己现在健康不健康
- logs、metrics、trace / event 是否在持续产生
- retention 与 exporter 是否正常
- 是否存在需要进一步跳转到 logs、sessions、executions 的异常线索

#### Observability 不是什么

- 不是原始日志检索页
- 不是完整 APM 产品
- 不是单个 connector 的调试页
- 不是审计轨迹页

#### 当前真实心智

真实页面 `web/src/pages/ops/ObservabilityPage.tsx` 说明它当前是 **built-in observability summary / platform health workbench**：

- Recent Logs
- Trace / Event Samples
- Retention & OTLP
- Health Context

### 1.2 用户目标与关键场景

#### 高频任务

- 快速判断平台是否出现观测退化
- 查看最近 logs / trace / event 样本
- 确认 retention 路径和容量是否正常
- 检查 OTLP / exporter 配置是否开启
- 从摘要跳转到更细的 Logs、Sessions、Executions

#### 关键场景

- 先从观测摘要判断问题是在“平台健康”还是“业务对象”
- 快速确认是本地 built-in 观测退化，还是 exporter / retention 配置异常
- 把异常线索导向正确的工作台，而不是在一个页面里做完所有调查

### 1.3 状态模型

#### 观测信号状态

- `healthy`
- `degraded`
- `missing`
- `unknown`

#### 存储状态

- `within_retention`
- `approaching_limit`
- `truncated`
- `misconfigured`

#### exporter 状态

- `disabled`
- `configured_not_enabled`
- `enabled`
- `unreachable`

#### 展示优先级

1. 是否存在明显退化
2. 哪种信号退化
3. 是否影响调查入口

### 1.4 核心字段与层级

#### L1 默认字段

- `log_entries_24h`
- `error_entries_24h`
- `event_entries_24h`
- `active_traces`
- `metrics_endpoint`
- `storage_bytes`

#### L2 条件字段

- recent logs 样本
- trace samples
- exporter list
- tracing provider
- healthy connectors / provider failures

#### L3 高级字段

- retention hours
- file paths
- OTLP endpoint / protocol / insecure

#### L4 系统隐藏字段

- exporter raw payload
- full trace storage internals
- debug counters

#### L5 运行诊断字段

- trace grouping detail
- recent component failures
- file rotation / truncation detail

### 1.5 关键规则与约束

- `/ops/observability` 当前虽在 `Ops` 路由下，但产品心智应是平台观测摘要页
- `/logs` 负责原始运行日志检索，不替代 observability summary
- Dashboard / Sessions / Executions 消费观测线索，但不取代 observability 作为健康摘要入口
- Observability 只负责“看总况并分发线索”，不应膨胀成完整 APM

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 看平台观测总况
- 判断最近是否有明显退化
- 了解日志、事件、trace 是否还在持续产生
- 决定下一步该跳到 Logs、Sessions、Executions 还是 Ops

#### 任务映射

| 用户任务 | 主入口 | 不应作为主入口 |
|---------|--------|----------------|
| 看平台观测总况 | `/ops/observability` | `/logs` |
| 查单条运行日志 | `/logs` | `/ops/observability` |
| 查单次执行证据 | `/sessions` 或 `/executions` | `/ops/observability` |
| 调整 exporter / retention 原始配置 | `Ops` | Observability 首屏 |

#### 首屏必须回答的 3 个问题

1. 观测链有没有断
2. 最近有没有明显错误或退化
3. 下一步该去哪个工作台深挖

### 2.2 入口与页面归属

#### `/ops/observability`

当前路由虽带 `/ops`，但导航分组心智应视为平台观测摘要页。

#### `/logs`

负责原始运行日志检索，不替代 observability summary。

#### Dashboard / Sessions / Executions

这些工作台消费 observability 线索：

- Dashboard 看总体健康信号
- Sessions 看具体诊断链路
- Executions 看动作结果

#### `Ops`

仅负责 retention、OTLP、raw config 等低频控制。

### 2.3 页面结构

#### 首屏结构

推荐与当前实现一致：

1. summary stats
2. recent logs
3. retention & OTLP
4. trace / event samples
5. health context

#### 页面优先级

首屏先回答：

1. 观测链有没有断
2. 最近有没有明显错误
3. 下一步该去哪个工作台深挖

### 2.4 CTA 与操作层级

#### 主动作

- `刷新`
- `前往 Logs`
- `前往 Sessions`
- `前往 Executions`

#### 次级动作

- `查看 Trace / Event 样本`
- `查看 Retention 与 OTLP`

#### 高级动作

- `前往 Ops 调整配置`
- `查看原始 exporter 信息`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在 retention / exporter 设置区
- L4/L5 不应默认占据首屏
- raw payload、trace internals、debug counters 进入高级区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Observability`
- 页面名：`Observability`

#### 页面叙事

- 页面讲“平台健康与观测摘要”
- 不讲“日志搜索”
- 不把 Observability 讲成完整 APM 产品

### 3.2 页面标题与副标题

#### 页面标题

- 标题：`Observability`
- 副标题应表达：查看平台观测健康、最近样本和调查入口

### 3.3 CTA 文案

主路径默认使用：

- `刷新`
- `前往 Logs`
- `前往 Sessions`
- `前往 Executions`

次级路径默认使用：

- `查看 Trace / Event 样本`
- `查看 Retention 与 OTLP`

高级区允许：

- `前往 Ops 调整配置`
- `查看原始 exporter 信息`

### 3.4 状态文案

#### 还没有观测样本

- 结论：`当前还没有足够的 built-in observability 数据`
- 细节：可能是新部署、尚无流量，或存储路径未写入
- 动作：`触发运行检查` 或 `前往 Logs`

#### exporter 未启用

- 结论：`当前只启用了本地 built-in observability`
- 细节：OTLP endpoint 未配置或 signals 未开启
- 动作：`前往 Ops 调整 exporter`

#### 观测链退化

- 结论：`当前有观测信号缺失或异常`
- 细节：logs、metrics、trace 中一项或多项异常
- 动作：`前往 Logs`、`前往 Sessions` 或 `前往 Executions`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- exporter raw payload
- full trace storage internals
- debug counters
- 把 Observability 讲成“原始日志页”

这些内容可留在高级区，不应主导 Observability 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/ops/observability` 已清晰表达为平台观测摘要页
- 页面默认先给健康总况、最近样本和调查入口，而不是原始配置细节
- `/logs`、Sessions、Executions 与 `Ops` 的边界清晰

### 4.2 交互级验收

- 用户能从摘要快速跳转到正确的深挖工作台
- 用户能区分“查看健康摘要”和“调整 exporter / retention 配置”
- exporter / retention 的底层调优不再打断主摘要体验

### 4.3 展示级验收

- 页面至少展示 summary stats、recent logs、retention & OTLP、trace / event samples、health context
- 空态、exporter 未启用、观测链退化等状态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖摘要、跳转入口和关键状态文案
- 需要浏览器或截图验收确认 Observability 默认叙事已经从“日志/配置混合页”收口为“平台观测摘要页”
- 若后端尚未提供更丰富的聚合或健康上下文，前端不应伪装成已有完整分析能力

### 4.5 剩余限制说明

- 对外标准暴露仍优先沿 Prometheus / OTLP 演进
- 更深的 retention、exporter、trace 存储调试继续保留在高级区或 `Ops`
