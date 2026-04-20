# Pilot Core Decision Gate: Knowledge / Vector / Outbox

日期：2026-04-02

## 1. 这份文档是做什么的

这不是一份立即删改 `knowledge / vector / outbox` 的实施方案。

它是一份试点后的决策门模板，用来回答三个问题：

1. `knowledge` 是否真的缩短了首个可行动判断时间。
2. `vector` 是否提供了足够的额外命中价值，值得继续保留。
3. `outbox` 的复杂度是否换来了当前试点场景真正需要的可靠性。

在没有真实试点证据前，不允许因为架构偏好提前“强留”或“强砍”这三块能力。

## 2. 当前状态快照

### 2.1 Knowledge

- `knowledge` 已经从默认试点闭环中收敛为 optional service；当前装配边界见 `internal/app/bootstrap_optional.go`。
- worker 路径已经允许 `knowledge` 缺失时继续跑核心闭环；当前 nil-safe 处理见 `internal/app/workers.go` 与 `internal/events/dispatcher.go`。
- 这意味着：`knowledge` 现在可以被度量价值，而不是继续被默认视为“必须存在”。

### 2.2 Vector

- 当前向量后端仍然只有 `sqlite-vec` 路径，入口在 `internal/app/bootstrap_shared.go`，实现位于 `internal/repo/sqlitevec/store.go`。
- 当前配置入口仍然是 `TARS_VECTOR_SQLITE_PATH`，定义在 `internal/foundation/config/config.go`。
- PostgreSQL schema 当前还没有 `pgvector` extension 或 `chunk_vectors` 表定义；`internal/repo/postgres/schema.go` 里也没有对应迁移。
- 这意味着：向量检索当前仍处于“可用但不是未来默认路线”的状态。

### 2.3 Outbox

- `outbox` 仍然是当前核心可靠性机制的一部分，默认由 `workflow -> dispatcher` 这条链路驱动。
- 运行面和运维面都已经依赖它：
  - 存储与查询：`internal/modules/workflow/service.go`、`internal/repo/postgres/workflow.go`
  - worker 处理：`internal/events/dispatcher.go`
  - Web / Ops API：`internal/api/http/ops_handler.go`
  - 指标：`internal/foundation/metrics/metrics.go`
- 这意味着：`outbox` 不是一个可以随手拔掉的 optional toy，而是一个需要判断“是否继续扩大投资面”的核心机制。

## 3. 当前默认立场

在试点证据出现前，默认立场如下：

- `knowledge`：保留为 optional，允许试点度量，不继续扩平台面。
- `vector`：不继续加深 `sqlite-vec` 路径，不做预先迁移。
- `outbox`：保留当前核心可靠性职责，但暂停新增围绕它的平台复杂度。

一句话说：先测价值，再决定保留深度。

## 4. 试点输入模板

在填写这份决策门前，先补齐下面这些上下文：

| 字段 | 说明 | 待填写 |
| --- | --- | --- |
| 试点团队 | 真实团队名称 | |
| 值班角色 | 谁在一线使用 | |
| 试点周期 | 开始 / 结束日期 | |
| 目标告警类型 | 最多 3 类高频告警 | |
| 主要渠道 | Telegram / Web / 其它 | |
| 核心成功标准 | “首个可行动判断”如何定义 | |
| 样本量 | 真实告警 / 真实对话次数 | |

## 5. 必填证据字段

以下 7 个字段必须填写，不能只凭主观印象做结论。

| 证据项 | 如何定义 | 建议数据来源 | 待填写 |
| --- | --- | --- | --- |
| 首个可行动判断时间 | 从告警进入到值班人拿到“可执行下一步”所用时间 | Session / Audit / 试点复盘记录 | |
| 建议采纳率 | AI 建议或命令被实际采纳的比例 | Session Trace / 审批记录 / 执行记录 | |
| 审批通过率 | 进入审批后最终通过的比例 | Execution / Approval 数据 | |
| 一周主动复用率 | 试点一周后是否仍主动回来使用 | 用户行为日志 / 使用记录 | |
| Knowledge 命中率 | 检索是否返回了被值班人认为有帮助的证据 | `tars_knowledge_search_total` + 试点标注 | |
| Outbox replay / dead-letter 率 | 是否频繁需要人工重放、是否常进入失败/阻塞 | `tars_outbox_events_total`、`tars_event_bus_deliveries_total`、Outbox 页面 | |
| 操作员额外负担 | 需要开发/平台同学介入多少次 | 试点日报 / 复盘记录 | |

## 6. 组件级决策规则

### 6.1 Knowledge：Keep / Defer / Disable

#### Keep

满足以下条件时，`knowledge` 可以继续保留并进入下一轮优化：

- 在目标告警类型里，知识证据多次帮助值班人更快形成首个可行动判断。
- 使用者会主动引用知识片段来支撑“为什么这样诊断 / 为什么敢批这个动作”。
- ingest 和 reindex 没有持续制造人工值守负担。

#### Defer

出现以下情况时，保留 optional 形态，但不继续扩平台化能力：

- 偶尔有帮助，但没有形成稳定复用。
- 命中质量不稳定，更多像“锦上添花”而不是止痛药。
- 当前试点里，真正决定价值的还是 diagnosis / approval / execute，而不是知识召回。

#### Disable by Default

出现以下情况时，默认关闭 knowledge ingest：

- 试点中几乎没有人主动参考知识证据。
- ingest / reindex 失败或维护成本显著高于收益。
- 关闭后核心闭环速度和采纳率几乎不受影响。

### 6.2 Vector：Migrate / Defer / Drop

`vector` 只有在 `knowledge` 先通过价值验证时才值得单独评估。

#### Migrate

只有同时满足以下条件时，才进入向量迁移计划：

- 词法检索已经明确不够用，真实案例里出现明显漏召回。
- 向量命中相比 lexical-only 有稳定增益。
- 运维复杂度是可接受的。

#### Defer

出现以下情况时，不做迁移：

- `knowledge` 自身价值还没站稳。
- 向量和词法的差异还没有被真实试点证明。
- 当前只是“看起来以后可能需要”，而不是“现在已经被证据逼出来”。

#### Drop Current Path

如果决定不保留向量路线：

- 不再继续加深 `sqlite-vec` 路径。
- 默认走 lexical-only。
- 不把向量索引作为 MVP 或试点必选项。

### 6.3 Outbox：Deepen / Hold / Freeze

`outbox` 的决策重点不是“要不要今天删掉”，而是“值不值得继续扩大它的产品和平台复杂度”。

#### Deepen

只有在以下情况明显成立时，才继续扩大 outbox 投入：

- 真实试点里确实出现了网络抖动、重试、消息补偿等场景，而且 outbox 带来了直接价值。
- 重放、失败处理、阻塞排查是值班流程里真实会用到的能力。
- 没有 outbox 时，通知/审批/回传链路会明显不稳。

#### Hold

如果 outbox 当前有价值，但还没证明需要更多产品表面：

- 保留当前机制。
- 不继续加新页面、新配置层或更重的抽象。
- 只做必要的可靠性修复和指标补齐。

#### Freeze

如果试点显示当前场景其实不需要这么重的可靠性机制：

- 暂停围绕 outbox 扩展新能力。
- 不新增依赖 outbox 的平台面。
- 只把它保留在当前最必要的链路里。

## 7. 硬约束

### 7.1 Vector Hard Constraint

如果向量检索最终决定继续保留：

- 只支持 PostgreSQL `pgvector`
- 不再保留 `sqlite-vec` fallback

后续迁移计划至少要覆盖：

- 配置项从 `TARS_VECTOR_SQLITE_PATH` 收敛为 PostgreSQL 单一路径
- PostgreSQL schema 增加 `pgvector` extension 与向量表
- 删除 `internal/repo/sqlitevec`
- 更新相关 troubleshooting / admin 文档

### 7.2 Gate Failure Fallback

如果这轮决策门没有通过：

- 优先退回 lexical-only
- 默认关闭 knowledge ingest
- 将 `knowledge / vector / outbox` 继续保持为 non-default 或最小职责形态
- 不让它们继续主导后续 roadmap

## 8. 当前可直接采集的数据源

### 8.1 Metrics

- `tars_knowledge_ingest_total`
- `tars_knowledge_search_total`
- `tars_outbox_events_total`
- `tars_event_bus_deliveries_total`

### 8.2 Web / Ops

- `/sessions`
- `/executions`
- `/outbox`
- Session Trace / Audit Trail / Knowledge Trace

### 8.3 试点人工记录

- 值班人第一次说出“我知道下一步该做什么了”的时间点
- 审批人为什么通过 / 拒绝
- 哪些建议被直接忽略
- 哪些操作还需要开发现场介入

## 9. 决策记录模板

### Pilot Summary

- 试点团队：
- 试点周期：
- 告警类型：
- 样本量：

### Evidence Summary

- 首个可行动判断时间：
- 建议采纳率：
- 审批通过率：
- 一周主动复用率：
- Knowledge 命中率：
- Outbox replay / dead-letter 率：
- 操作员额外负担：

### Decision

| 能力 | 结论 | 说明 |
| --- | --- | --- |
| Knowledge | Keep / Defer / Disable | |
| Vector | Migrate / Defer / Drop | |
| Outbox | Deepen / Hold / Freeze | |

### Next Action

- [ ] 如果 `Knowledge = Keep`，定义下一轮最小优化点
- [ ] 如果 `Vector = Migrate`，新开 `pgvector-only` 实施 plan
- [ ] 如果 `Vector != Migrate`，保持 lexical-only 默认路线
- [ ] 如果 `Outbox = Hold/Freeze`，暂停新增 outbox 平台面

## 10. 当前阶段的明确结论

截至 `2026-04-02`，当前允许下的最强结论只有：

- 试点核心闭环已经可以不强依赖 `knowledge`
- `vector` 还没有拿到继续投资的证据
- `outbox` 仍是当前默认可靠性机制，但“是否继续加深”必须等真实试点数据

所以这一阶段的正确动作不是继续拍板，而是把试点证据收上来后，再按这份模板做一次正式 go/no-go。
