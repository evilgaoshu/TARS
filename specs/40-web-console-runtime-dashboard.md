# TARS - Runtime Dashboard 规范

> **状态**: 设计基线
> **适用范围**: `/` dashboard 运行总览、incident queue、execution queue、signal posture
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[40-web-console-sessions-workbench.md](./40-web-console-sessions-workbench.md)、[40-web-console-executions-workbench.md](./40-web-console-executions-workbench.md)

---

## 1. 功能 Spec

### 1.1 工作台定义

#### Runtime Dashboard 是什么

`Runtime Dashboard` 是 TARS 的 **operator command center**。

它不是某个配置对象，而是围绕当前运行态建立的工作台对象域：

- 活跃 incidents
- 活跃 executions
- 平台健康与信号摘要
- 需要人工处理的下一步动作

#### Dashboard 不是什么

- 不是对象列表页拼盘
- 不是 `Ops Summary` 的原样回显
- 不是 observability 页面副本

### 1.2 工作台目标

Runtime Dashboard 的目标是让值班者在最短时间内判断：

- 当前是否有需要人工介入的运行事件
- 风险集中在哪一类工作流
- 应优先进入 Sessions、Executions 还是 Setup / Runtime Checks

### 1.3 状态模型

Dashboard 状态优先按“是否需要人工介入”表达，而不是对象生命周期。

#### 状态枚举

- `healthy`
- `warning`
- `danger`
- `muted`

#### 状态判断优先级

1. 是否有活跃 incidents
2. 是否有待审批或失败 executions
3. 平台信号是否异常
4. 相关 summary / health 数据是否加载失败

### 1.4 核心字段与层级

#### L1 默认字段

- active alerts
- pending approvals
- provider failures
- active incidents
- failed runs

#### L2 条件字段

- recent incident cards
- recent execution cards
- hot alerts
- signal posture rows

#### L3 高级字段

- connector health ratio
- blocked / failed outbox pressure

#### L4 系统隐藏字段

- raw summary payload
- low-level health detail

#### L5 运行诊断字段

- surfaced alert metadata
- provider failure detail

### 1.5 关键规则与约束

- Dashboard 首屏必须先讲“是否需要人工介入”
- 它是值班工作台，不是治理配置页
- 默认区只展示当前运行风险与下一步入口
- 低层健康细节和原始 summary payload 不应主导首屏

### 1.6 API 映射

- `GET /api/v1/summary`
- `GET /api/v1/dashboard/health`
- `GET /api/v1/sessions`
- `GET /api/v1/executions`

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 看当前有多少活跃 incidents
- 看待审批与失败 executions
- 看平台信号是否异常
- 进入 sessions / executions 深挖

#### 首屏必须回答的 3 个问题

1. 现在有没有需要我马上处理的事
2. 问题更偏 incident、execution，还是平台信号异常
3. 下一步我应该进入哪个工作台

### 2.2 入口与页面归属

#### `/`

作为 Runtime 顶层首页，优先服务值班与操作者，而不是治理人员。

#### 深链目标

- incidents -> `/sessions`
- executions -> `/executions`
- setup/runtime checks -> `/setup`

### 2.3 页面结构

推荐结构：

1. Hero
2. Runtime alert
3. Stats
4. Incident queue
5. Execution queue
6. Signal posture
7. Hot alerts

#### 结构规则

- Hero 先给运行总判断和主入口
- Stats 先回答当前压力与风险等级
- Incident / Execution queue 必须保留明确深链入口
- Signal posture 作为辅助判断，不应压过当前待处理队列

### 2.4 CTA 与操作层级

#### 主 CTA

- `Review Incidents`
- `Open Sessions`
- `Open Runs`

#### 次 CTA

- `Refresh`
- `Runtime Checks`

#### 层级规则

- 主 CTA 直接进入需要人工处理的工作台
- 刷新和其他低风险动作下沉为次级动作
- Dashboard 不应承担复杂配置或修复编辑入口

### 2.5 字段分层

- L1 用于首屏判断当前是否需要人工介入
- L2 用于帮助定位问题来源与工作台
- L3 仅作平台健康补充
- L4/L5 仅在 drill-down 或关联页面中呈现

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Runtime Dashboard`
- 首页心智：`operator command center`
- incident 区：`Incident Queue`
- execution 区：`Execution Queue`
- 信号区：`Signal Posture`

#### 页面叙事

- 页面应该讲“当前运行态”
- 不讲“系统配置总览”
- 不把 dashboard 写成“运维后台首页拼盘”

### 3.2 页面标题与副标题

#### 页面标题

- 标题：`Runtime Dashboard`

#### 页面副标题

- 应表达：当前运行态、待处理事项、平台信号与下一步入口
- 语气应偏值班与操作，不偏治理与配置

### 3.3 CTA 文案

默认使用：

- `Review Incidents`
- `Open Sessions`
- `Open Runs`
- `Refresh`
- `Runtime Checks`

CTA 文案应直接表达用户接下来会进入的工作台，不使用模糊的“查看更多”。

### 3.4 状态文案

#### 无 incident

- 结论：`当前没有待诊断事件`
- 细节：系统处于平稳态，可继续关注 signal posture

#### 无 execution

- 结论：`当前没有待处理动作`
- 细节：审批与执行队列为空

#### runtime degraded

- 结论：`运行总览当前不完整`
- 细节：summary 或 health 数据加载失败，需要进一步检查 runtime checks 或相关工作台

### 3.5 术语黑名单

以下词不应主导首屏内容：

- raw summary payload
- low-level health detail
- observability debug terms
- 配置对象字段名

这些词可以出现在更深层页面，但不应成为 Dashboard 首屏叙事。

---

## 4. 验收清单

### 4.1 页面级验收

- `/` 首屏明确是运行指挥台，而不是配置首页
- 首屏先回答“现在是否需要人工介入”
- Hero、Stats、Queue、Signal posture 的信息优先级清楚
- Dashboard 与 Ops / Observability 的对象边界清楚

### 4.2 交互级验收

- 从 incident 区能直接进入 `/sessions`
- 从 execution 区能直接进入 `/executions`
- Runtime Checks 深链明确存在
- 刷新是次级动作，不抢主任务路径

### 4.3 展示级验收

- 默认区只保留 L1 / L2 级运行信息
- 无 incident、无 execution、runtime degraded 都有清晰状态文案
- Signal posture 是辅助判断，不挤占当前待处理队列的主位置

### 4.4 测试与验证要求

- 需要页面级测试覆盖 stats、incident queue、execution queue、empty state 和 degraded state
- 需要浏览器或截图验收确认首页主叙事正确
- 如果 summary 指标来自局部窗口而不是全局聚合，UI 不应把它伪装成稳定总量

### 4.5 剩余限制说明

- Dashboard 可以继续依赖现有 summary / health API
- 如果后端尚未提供稳定聚合指标，前端应通过命名与文案避免误导为“全局真实总量”
