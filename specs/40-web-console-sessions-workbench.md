# TARS — Sessions Workbench 规范

> **状态**: 设计基线
> **适用范围**: `/sessions` 列表与详情、incident queue / diagnosis workbench、黄金路径、证据与审计关联
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[40-web-console-executions-workbench.md](./40-web-console-executions-workbench.md)、[20-component-audit.md](./20-component-audit.md)、[20-component-knowledge.md](./20-component-knowledge.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Sessions Workbench 是什么

`Sessions Workbench` 是 **诊断队列与诊断工作台对象域**。

它围绕单次 incident / diagnosis thread 展开，而不是配置对象。

#### Session 不是什么

- 不是普通聊天记录列表
- 不是审计表
- 不是 execution 列表的附庸

### 1.2 用户目标与关键场景

#### 高频任务

- 排队查看活跃 incident
- 打开单个 session 看黄金路径
- 查看 tool plan、knowledge、attachments、notifications、audit trace
- 跳到相关 executions

#### 关键场景

- 值班期间快速筛出最需要处理的诊断队列
- 在单个 session 内完成“看结论、看证据、看执行动作”的闭环
- 把 session 里的诊断结论与 execution、audit、knowledge 串起来

### 1.3 状态模型

- `open`
- `analyzing`
- `pending_approval`
- `executing`
- `verifying`
- `resolved`
- `failed`

#### 展示优先级

1. 结论
2. 风险
3. 下一步
4. 状态
5. 证据摘要与最近更新时间

### 1.4 核心字段与层级

#### L1 默认字段

- alert title
- conclusion / headline
- risk
- next action
- status
- service / host
- evidence summary

#### L2 条件字段

- smoke badge
- timeline last update
- execution count
- verification summary

#### L3 高级字段

- raw alert context
- materialized knowledge preview
- attachment payloads

#### L4 系统隐藏字段

- backend raw payload internals

#### L5 诊断字段

- audit metadata
- tool step inputs / outputs

### 1.5 关键规则与约束

- `/sessions` 是 diagnosis queue 主入口
- `/sessions/{id}` 是 diagnosis workbench 主入口
- `/executions` 承接动作执行细节，不应反过来替代 session 主叙事
- 审计、knowledge、attachments、notifications 都应围绕单个 session 汇聚，而不是把用户赶去多个控制台拼信息

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 快速判断哪些 session 需要优先处理
- 打开某个 session，先看结论，再看依据，再看动作
- 查看这个 session 是否已执行、是否还缺审批或验证
- 跳去 execution 看具体动作细节

#### 首屏信息顺序

首屏默认顺序必须是：**结论 -> 风险 -> 下一步 -> 状态 -> 证据**。

#### 首屏必须回答的 3 个问题

1. 当前有哪些 session 正在等待我关注
2. 这个 session 现在处于什么阶段、风险多高、下一步是什么
3. 我能在哪看到结论、证据、执行与审计链路

### 2.2 入口与页面归属

#### `/sessions`

负责：

- diagnosis queue
- 队列筛选与排序
- 值班态概览
- 批量导出或交接

#### `/sessions/{id}`

负责：

- 单个 diagnosis workbench
- 黄金路径
- narrative / tool plan / knowledge / attachments / timeline

#### `/executions`

负责：

- 具体动作执行细节
- 结果、输出、审批、重试、失败原因

不应替代 session 主页面的结论与证据叙事。

### 2.3 页面结构

#### 列表页

推荐结构：

1. Hero
2. Stats
3. Filters
4. Bulk export
5. Diagnosis queue cards

列表页默认先回答“哪个 session 最值得处理、状态如何、下一步是什么”，而不是先展示原始消息或审计流水。

列表首屏卡片默认就要带出结论、风险、下一步和证据摘要，不能把这些信息埋到二跳详情里。

#### 详情页

推荐结构：

1. Hero
2. Golden path
3. Diagnosis narrative
4. Tool plan
5. Knowledge context
6. Attachments
7. Raw alert context
8. Verification / Notifications / Linked executions / Timeline / Audit trace

详情页首屏必须先给结论、风险、下一步、当前状态和证据摘要，再让用户下潜到 tool plan、timeline 与原始上下文。

### 2.4 CTA 与操作层级

#### 主动作

- `查看详情`
- `打开相关执行`
- `导出`

#### 次级动作

- `查看 Tool Plan`
- `查看时间线`
- `查看审计链路`

#### 高级动作

- `查看原始上下文`
- `查看完整 Trace`
- `跳转 Ops`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在折叠区、标签页或补充区块
- L4/L5 不应默认占据列表页和详情首屏
- 原始 payload、完整 trace 与 step IO 进入高级调试区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Sessions`
- 列表页：`Sessions`
- 详情页：使用 incident / session 标题

#### 页面叙事

- 页面讲“诊断工作台”
- 不讲“聊天记录”
- 不把 Sessions 讲成 execution 附件页

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Sessions`
- 副标题应表达：查看诊断队列、当前状态、风险与下一步

#### 详情页

- 标题默认使用 alert title 或 session headline
- 副标题应聚焦当前状态、当前结论和下一步动作

### 3.3 CTA 文案

主路径默认使用：

- `查看详情`
- `打开相关执行`
- `导出`

次级路径默认使用：

- `查看 Tool Plan`
- `查看时间线`
- `查看审计链路`

高级区允许：

- `查看原始上下文`
- `查看完整 Trace`

### 3.4 状态文案

#### 无 Sessions

- 结论：`当前没有待处理的诊断会话`
- 细节：可先完成 setup 或触发 runtime checks 生成会话
- 动作：`前往 Setup`

#### Narrative 缺失

- 结论：`诊断摘要暂不可用`
- 细节：系统还在生成 narrative，或当前只拿到了部分证据
- 动作：`查看时间线`

#### Knowledge / Tool Plan 缺失

- 结论：`当前还没有补充上下文`
- 细节：knowledge 或 tool plan 尚未生成，不代表 session 无法处理
- 动作：`查看原始上下文`

#### Trace 不可用

- 结论：`完整 Trace 暂不可用`
- 细节：保留当前结论和时间线，稍后再查看高级诊断
- 动作：`查看时间线`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- backend raw payload internals
- audit metadata field names
- tool step raw IO dump
- “聊天记录”作为 Sessions 默认叙事

这些内容可留在高级调试区，不应主导 Sessions 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/sessions` 已清晰表达为 diagnosis queue，而不是聊天列表
- `/sessions/{id}` 已清晰表达为 diagnosis workbench，而不是 execution 附件页
- 列表与详情首屏都先给结论、风险、下一步、状态、证据，再展示更深层上下文

### 4.2 交互级验收

- 用户能从列表页快速判断优先级并进入详情
- 用户能从详情页顺畅跳到相关 executions，而不丢失 session 主线
- knowledge、attachments、audit、notifications 都围绕 session 聚合，而不是分散到多个页面完成主任务

### 4.3 展示级验收

- 列表页至少展示标题、状态、风险、headline、next action、service/host
- 详情页具备黄金路径、narrative、tool plan、knowledge、attachments、linked executions、timeline、audit trace 等关键区块
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖列表摘要、详情主区块和关键局部空态
- 需要浏览器或截图验收确认 Sessions 仍是“诊断工作台”而不是“聊天记录”
- 需要验证从 session 跳 execution 的链路清晰，但 execution 细节未反噬 session 主叙事

### 4.5 剩余限制说明

- 若后端尚未提供完整 tool plan、knowledge 或 trace，前端不应伪装成数据已齐
- 审计与高级诊断细节可以继续保留在补充区块，不必强行塞进首屏
