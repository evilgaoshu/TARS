# TARS — 异步事件与消息总线演进策略

> **状态**: Next Phase 设计基线  
> **定位**: 定义 TARS 从 `PostgreSQL outbox + worker` 演进到受控 `Event Bus / Message Bus` 的路线  
> **目标**: 在不引入过早复杂度的前提下，保证平台后续可平滑扩展到更强的异步处理能力

---

## 1. 结论

当前阶段，TARS **不必立刻引入重型消息队列**，但应立即统一异步架构口径：

1. **当前主异步底座继续使用 `PostgreSQL outbox + worker`**
2. **现在就补 `Event Bus` 抽象**
3. **当规模和消费者模型真正上来时，再接正式 MQ / Bus**

一句话定义：

> 先把异步语义做对，再把底层传输从 Postgres outbox 平滑升级到更强的消息系统。

---

## 2. 当前方案为什么还能继续用

当前 TARS 已具备一条可工作的异步主链：

- `session_closed`
- `telegram.send`
- `knowledge_ingest`
- `approval_timeout`
- `reindex_document`
- 各类 retry / replay / blocked / failed outbox

这条链路目前的优势是：

- 实现简单
- 运维成本低
- 与 PostgreSQL 事务一致性天然协同
- 适合当前模块化单体架构
- 对 MVP / 当前平台化阶段足够稳定

因此短期内，继续以 outbox 为主是合理的。

---

## 3. 当前方案的边界

`PostgreSQL outbox + worker` 不是终局方案，后续会遇到这些瓶颈：

- fan-out 消费者越来越多
- 多实例 worker 并行更复杂
- 需要多个独立 consumer group
- 需要跨服务 / 跨语言消费
- 需要更强的实时流处理
- 需要更长时间的事件保留、回放与重放

如果不提前抽象，后面业务代码会直接绑死在：

- `outbox_events`
- dispatcher/worker
- PostgreSQL polling 语义

这样未来切 MQ 的成本会很高。

---

## 4. 推荐演进路线

### 4.1 阶段一：继续强化 PostgreSQL outbox

当前继续做的事情应包括：

- topic 命名统一
- retry / backoff / blocked / failed 语义统一
- replay / delete / dead-letter 语义清楚
- 观测指标统一
- outbox topic 与业务事件清单标准化

建议继续把这些事件收敛到统一异步主链：

- notifications
- knowledge ingest
- approval timeout
- automations
- trigger policy evaluation / governance fire
- extension build / import pipeline

### 4.2 阶段二：补 Event Bus 抽象

在代码层引入统一异步接口，而不是让业务直接依赖 Postgres outbox：

- `Publish(topic, event)`
- `Subscribe(topic, handler)`
- `Ack`
- `Retry`
- `DeadLetter`

这样当前仍可由 `PostgreSQL outbox adapter` 落地，但业务模型已经不依赖底层实现。

### 4.3 阶段三：在合适时机接正式消息系统

当以下条件明显出现时，再正式引入 MQ / Bus：

- 多实例 worker 竞争明显增多
- 事件 fan-out 显著增加
- 多个独立 consumer group 成为刚需
- 需要把通知、自动化、扩展构建、审计导出拆成独立进程
- 需要跨服务 / 跨语言消费
- 需要更强的保留 / replay / near-real-time streaming

---

## 5. 推荐技术路线

### 5.1 当前默认实现

- `PostgreSQL outbox + worker`

适用：

- 当前单体 / 模块化单体
- 事务一致性优先
- 事件量中等
- 以可靠异步副作用为主

### 5.2 后续优先候选

#### A. `NATS JetStream`

推荐作为后续第一优先的正式消息总线候选。

原因：

- 轻量
- 适合平台型产品
- 支持 stream / consumer / ack / replay
- 比 Kafka 更适合“小而美”的平台

#### B. `RabbitMQ`

适合：

- 任务路由清晰
- 工作队列模型明确
- 多种 routing key / exchange 需求明显

#### C. `Kafka`

只建议在这些场景下考虑：

- 事件量非常大
- retention / replay 要求高
- 多 consumer group / 数据流平台属性明显
- 需要跨团队、跨服务的大规模事件平台

当前阶段不建议直接上 Kafka。

---

## 6. Event Bus 抽象建议

后续建议统一抽象出：

- `EventBus`
- `Publisher`
- `Subscriber`
- `DeliveryPolicy`
- `RetryPolicy`
- `DeadLetterPolicy`

至少要统一这些概念：

- `topic`
- `event_id`
- `aggregate_id`
- `payload`
- `headers / metadata`
- `attempt`
- `available_at`
- `status`

不让业务代码直接感知：

- SQL polling
- `outbox_events` 表结构
- 数据库 reclaim / recover 细节

---

## 7. 事件分类建议

建议把异步事件先分成 4 类：

### A. 业务状态事件

- `session.closed`
- `session.analyze_requested`
- `execution.completed`
- `approval.timeout`

### B. 通知与送达事件

- `channel.send`
- `channel.retry`
- `notification.dispatch`

### C. 平台自动化事件

- `automation.run_requested`
- `governance.trigger_policy_fired`
- `platform_action.requested`

### D. 扩展与治理事件

- `extension.validate_requested`
- `extension.import_requested`
- `skill_bundle.generated`

---

## 8. Hook / Trigger / Event Bus 的职责边界

为了避免把内部扩展逻辑、规则匹配逻辑和实际动作混在一起，后续应明确采用以下分层：

- `Hook`
  - 系统内部生命周期扩展点
  - 负责 `before_* / after_*` 这类内部挂接
  - 负责补上下文、预校验、后处理、派生标准事件
- `Event Bus`
  - 负责标准事件发布、传递、重试与死信
- `Trigger`
  - 负责基于事件/条件进行规则匹配
  - 负责决定是否发送通知、触发 automation、执行高层动作
- `Automation`
  - 负责真正执行 `skill / connector capability / platform action`

建议主链路统一为：

1. lifecycle node changed
2. hook executes internal extension logic
3. hook emits standard event
4. event enters event bus
5. governance rule matches
6. automation / notification / delivery executes

这条边界的目的不是增加概念，而是避免：

- 把内部后处理逻辑直接写死在治理规则里
- 把通知和自动化规则继续散落在 workflow 内部
- 让 automation 直接承担事件解释职责

---

## 9. 与当前平台能力的关系

后续这条异步演进策略需要和以下系统对齐：

- `Channels`
  - outbound notification / delivery
- `Automations`
  - schedule / run history / retry
- `Automation Governance`
  - trigger policy / hook / event routing / schedule governance
- `Skills`
  - 受控编排和生成
- `Self-Evolving Platform`
  - bundle generate / validate / import

也就是说，后续异步主链不应只服务“通知重试”，而应成为平台级异步治理底座。

---

## 10. 什么时候应该明确上 MQ / Bus

建议至少满足下列 3 项中的 2 项，再进入正式选型与接入阶段：

1. 异步事件 topic 明显增多，且消费者角色已经分离
2. 单进程 worker 已不适合承担主要吞吐
3. 通知 / 自动化 / 扩展构建 / 审计导出 已开始拆成独立服务或独立部署单元

补充触发条件：

- 需要独立 consumer group
- 需要跨语言消费
- 需要事件保留 / replay / backfill
- 需要降低 Postgres polling 压力

---

## 11. 当前建议

当前阶段的最优解是：

1. 继续使用 `PostgreSQL outbox + worker`
2. 立即补 `Event Bus` 抽象
3. 先让 Automations、Channels、Extensions 与 `Governance / Advanced` 规则统一走这条异步主链
4. 等平台真正长大后，再优先评估 `NATS JetStream`

一句话总结：

> 现在先把异步语义和接口收对，等业务体量和消费模型真正上来，再把底层从 Postgres outbox 平滑切到正式消息总线。

---

## 11. 2026-03-23 当前落地状态

本轮已把 Event Bus 抽象正式落到运行时主链，但底层仍保持 `PostgreSQL outbox + worker`：

- 已新增统一契约：
  - `EventPublishRequest`
  - `EventEnvelope`
  - `DeliveryPolicy`
  - `DeliveryDecision(ack/retry/dead_letter)`
  - `EventPublisher / EventConsumer / EventBus`
- 已让 `workflow.Service` 与 `postgres.Store` 同时实现：
  - `PublishEvent`
  - `ClaimEvents`
  - `ResolveEvent`
  - `RecoverPendingEvents`
- 已把 dispatcher 从“直接依赖 outbox 细节”收口为“消费 EventEnvelope 并返回 DeliveryResult”模型
- 已接入并验证的真实链路：
  - `session.analyze_requested`
  - `session.closed`
  - `telegram.send`

### 11.1 当前统一语义

- `ack`
  - 事件标记为 `done`
- `retry`
  - 事件回到 `pending`
  - 增加 `retry_count`
  - 写回 `last_error`
  - 根据策略设置新的 `available_at`
- `dead_letter`
  - 事件标记为 `failed`
  - 增加 `retry_count`
  - 保留 `last_error`

### 11.2 当前默认策略

- `session.analyze_requested`
  - `MaxAttempts = 1`
  - 失败直接进入 `dead_letter`
- `session.closed`
  - `MaxAttempts = 1`
  - 失败直接进入 `dead_letter`
- `telegram.send`
  - `MaxAttempts = 3`
  - 默认 backoff：`1s -> 5s`

### 11.3 当前边界

- 业务发布侧已不再需要直接拼装 outbox claim/ack/fail 语义
- 运行时仍真实复用现有 outbox 表与 polling worker
- `approval_timeout` 仍是独立定时 worker，本轮未并入统一总线消费链
- `headers / metadata` 已进入统一 envelope 契约；当前 PostgreSQL adapter 先保留接口形态，后续再视需要补持久化列
