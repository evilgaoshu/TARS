# TARS — Executions Workbench 规范

> **状态**: 设计基线
> **适用范围**: `/executions` 列表与详情、approval / run queue、执行结果审阅
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[40-web-console-sessions-workbench.md](./40-web-console-sessions-workbench.md)、[20-component-audit.md](./20-component-audit.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Executions Workbench 是什么

`Executions Workbench` 是 **approval / run queue 与 execution review workbench**。

它围绕“一个动作是否该执行、执行后结果怎样、还需要谁处理”展开，而不是配置对象。

#### Execution 不是什么

- 不是 connector 配置详情
- 不是 shell output 浏览器而已
- 不是 audit trail 页面

### 1.2 用户目标与关键场景

#### 高频任务

- 查看待审批执行
- 审批、拒绝或请求上下文
- 查看命令或 capability 结果
- 跳回 session 查看原因链

#### 关键场景

- 值班或审核时快速识别哪些执行需要优先审批
- 在单个 execution 内完成“看动作、看结果、看后续”的闭环
- 将 execution 与上游 session、下游 follow-through 以及审计链路串起来

### 1.3 状态模型

- `pending`
- `approved`
- `executing`
- `completed`
- `failed`
- `timeout`
- `rejected`

#### 展示优先级

1. 动作
2. 原因
3. 风险
4. 审批状态
5. 结果
6. 观察建议 / 下一步

### 1.4 核心字段与层级

#### L1 默认字段

- action headline
- why this action exists
- risk
- approval status
- result summary
- target host
- observation suggestion / next action

#### L2 条件字段

- request kind
- execution mode
- approval summary
- output bytes / truncated

#### L3 高级字段

- connector id
- capability id
- approval group
- exit code

#### L4 系统隐藏字段

- raw runtime transport details

#### L5 诊断字段

- full output chunks
- follow-through linkage

### 1.5 关键规则与约束

- `/executions` 承接审批与运行队列，不应被讲成“命令输出页”
- `/executions/{id}` 承接单次执行审阅，不应替代 session 的上游诊断叙事
- `/sessions/{id}` 继续承担原因链与诊断上下文
- execution 页面必须把动作、审批、结果和 follow-through 聚在一个主对象里，而不是逼用户去 audit 或 raw console 拼信息

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 快速判断哪些 execution 需要我处理
- 决定批准、拒绝还是索要更多上下文
- 查看这次 execution 做了什么、输出如何、有没有失败
- 跳回相关 session 了解为什么会触发这个动作

#### 首屏信息顺序

首屏默认顺序必须是：**动作 -> 原因 -> 风险 -> 审批 -> 结果 -> 观察建议**。

#### 首屏必须回答的 3 个问题

1. 当前有哪些 execution 正在等待处理，风险和状态如何
2. 这个 execution 执行了什么、结果怎样、下一步是什么
3. 我可以在哪看到上游原因链、下游跟进和完整输出

### 2.2 入口与页面归属

#### `/executions`

负责：

- approval queue
- run queue
- 状态筛选与排序
- 批量导出

#### `/executions/{id}`

负责：

- 单次 execution review
- 审批动作
- 输出审阅
- follow-through links

#### `/sessions/{id}`

负责：

- 上游诊断原因链
- tool plan / knowledge / incident context

Execution 页面可跳回 session，但不应吞掉 session 的主叙事。

### 2.3 页面结构

#### 列表页

推荐结构：

1. Hero
2. Stats
3. Filters
4. Bulk export
5. Execution queue cards

列表页默认先回答“哪些 execution 等待我处理、当前状态如何、风险多高”，而不是先铺满输出字节或底层 transport 信息。

列表首屏卡片默认就要带出动作、原因、风险、审批状态和结果摘要，不能先把用户带进 raw output 细节。

#### 详情页

推荐结构：

1. Hero
2. Golden path
3. Execution action bar
4. Console output
5. Metadata
6. Follow-through links

详情页首屏必须先讲动作、原因、风险、审批状态、结果和观察建议，再让用户下潜到完整输出和底层细节。

### 2.4 CTA 与操作层级

#### 主动作

- `批准`
- `拒绝`
- `请求更多上下文`

#### 次级动作

- `查看相关 Session`
- `查看输出`
- `导出`

#### 高级动作

- `查看完整 Trace`
- `查看底层传输信息`
- `跳转 Ops`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在补充信息区
- L4/L5 不应默认占据列表页和详情首屏
- full output chunks、raw transport details 进入高级调试区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Executions`
- 列表页：`Executions`
- 详情页：使用 execution headline

#### 页面叙事

- 页面讲“执行队列与执行审阅”
- 不讲“命令日志”
- 不把 Executions 讲成 connector 配置或纯审计流水页

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Executions`
- 副标题应表达：查看待处理执行、审批状态、结果与下一步

#### 详情页

- 标题默认使用 execution headline
- 副标题应聚焦当前状态、当前结果和待处理动作

### 3.3 CTA 文案

主路径默认使用：

- `批准`
- `拒绝`
- `请求更多上下文`

次级路径默认使用：

- `查看相关 Session`
- `查看输出`
- `导出`

高级区允许：

- `查看完整 Trace`
- `查看底层传输信息`

### 3.4 状态文案

#### 无 Executions

- 结论：`当前没有待处理的执行`
- 细节：可先从 session 触发动作，或等待新的审批请求到来
- 动作：`查看 Sessions`

#### 输出缺失

- 结论：`当前没有记录到执行输出`
- 细节：并不一定代表执行失败，可能是无输出或输出未持久化
- 动作：`查看状态与元数据`

#### Detail 不可用

- 结论：`当前执行详情不可用`
- 细节：该 execution 可能已删除、不可访问或后端尚未返回完整信息
- 动作：`返回执行列表`

#### 等待审批

- 结论：`这个执行正在等待处理`
- 细节：请根据上下文决定批准、拒绝或请求更多信息
- 动作：`批准`、`拒绝`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- raw runtime transport details
- full output chunks
- internal approval transport metadata
- “命令输出浏览器”作为 Executions 默认叙事

这些内容可留在高级调试区，不应主导 Executions 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/executions` 已清晰表达为 approval / run queue，而不是日志列表
- `/executions/{id}` 已清晰表达为 execution review workbench，而不是单纯输出页
- 列表与详情首屏都先给动作、原因、风险、审批、结果、观察建议，再展示输出和底层细节

### 4.2 交互级验收

- 用户能从列表页快速判断优先级并进入详情
- 用户能在详情页完成批准、拒绝、请求上下文等主动作
- 用户能顺畅跳回相关 session，但不丢失 execution 主线

### 4.3 展示级验收

- 列表页至少展示 headline、status、risk、target host、next action
- 详情页具备动作栏、输出区、元数据区和 follow-through links
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖列表摘要、审批动作入口和详情主区块
- 需要浏览器或截图验收确认 Executions 默认叙事已经从“日志页”收回到“执行工作台”
- 需要验证 session -> execution 和 execution -> session 的往返链路清晰

### 4.5 剩余限制说明

- 若后端尚未提供完整输出、trace 或 follow-through 数据，前端不应伪装成数据已齐
- audit 和更底层 transport 细节可以继续保留在补充区块或 Ops，不必强行塞进首屏
