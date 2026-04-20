# TARS — Inbox Workbench 规范

> **状态**: 设计基线
> **适用范围**: `/inbox` 第一方通知与审批消息工作台
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-channels-and-web-chat.md](./20-component-channels-and-web-chat.md)、[40-web-console-executions-workbench.md](./40-web-console-executions-workbench.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Inbox Workbench 是什么

`Inbox Workbench` 是第一方 `in_app_inbox` / inbox channel 的消息工作台。

它承接审批提醒、执行结果、诊断更新等第一方送达消息，并支持在消息上下文中继续动作。

#### Inbox Workbench 不是什么

- 不是 Channel 配置页
- 不是通用邮件客户端
- 不是 session / execution 详情页的替代品

### 1.2 用户目标与关键场景

#### 高频任务

- 查看未读审批提醒、执行结果、诊断更新
- 标记已读或全部已读
- 通过消息直接进入 session / execution
- 在消息上下文内处理 pending execution

#### 关键场景

- 用户通过第一方工作台接收需要尽快处理的内部消息
- 用一条消息直接衔接到 session 或 execution
- 在不跳出收件箱视角的前提下完成一部分内联处理

### 1.3 状态模型

- `unread`
- `read`
- `actionable`
- `archived`

#### 展示优先级

1. 是否未读
2. 是否可操作
3. 消息来源和时间

### 1.4 核心字段与层级

#### L1 默认字段

- `subject`
- `read / unread`
- `source`
- `created_at`

#### L2 条件字段

- `channel`
- `ref_type`
- `ref_id`
- `body`

#### L3 高级字段

- inline execution action

#### L4 系统隐藏字段

- raw delivery metadata

#### L5 运行诊断字段

- mark read error detail
- fetch error detail

### 1.5 关键规则与约束

- `/inbox` 是第一方送达工作台
- `/channels` 承接 inbox channel 配置，不承接消息消费主体验
- Inbox 讲的是“收到什么、要不要处理、跳到哪里”，不应回退成消息配送底层调试页

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 快速看有哪些未读消息
- 判断哪些消息需要立即处理
- 标记已读或全部已读
- 从消息跳到相关 session / execution

#### 首屏必须回答的 3 个问题

1. 当前有哪些未读或待处理消息
2. 哪些消息可以直接行动
3. 我应该从这里继续处理，还是跳去 session / execution

### 2.2 入口与页面归属

#### `/inbox`

作为第一方送达工作台，负责：

- 消息流
- 未读状态
- 内联动作
- 跳转相关对象

#### `/channels`

作为 inbox channel 配置入口，负责 channel 级配置、能力和验证。

### 2.3 页面结构

推荐结构：

1. Hero
2. Stats
3. Message stream
4. Inline actions

页面默认先回答“有哪些消息、是否未读、是否可处理”，而不是先讲 raw delivery。

### 2.4 CTA 与操作层级

#### 主动作

- `标记已读`
- `全部标记已读`
- `继续处理`

#### 次级动作

- `查看 Session`
- `查看 Execution`
- `筛选未读`

#### 高级动作

- `查看送达诊断`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在消息内联动作区
- L4/L5 不应默认占据消息流首屏
- raw metadata 与错误明细进入补充区块

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Inbox`
- 页面名：`Inbox Workbench`

#### 页面叙事

- 页面讲“第一方送达工作台”
- 不讲“channel 配置”
- 不把 Inbox 讲成普通邮箱客户端

### 3.2 页面标题与副标题

#### 页面标题

- 标题：`Inbox`
- 副标题应表达：查看内部通知、审批提醒和执行更新

### 3.3 CTA 文案

主路径默认使用：

- `标记已读`
- `全部标记已读`
- `继续处理`

次级路径默认使用：

- `查看 Session`
- `查看 Execution`
- `筛选未读`

高级区允许：

- `查看送达诊断`

### 3.4 状态文案

#### 无消息

- 结论：`当前没有消息`
- 细节：新的诊断更新、审批提醒和执行结果会显示在这里

#### fetch 失败

- 结论：`当前无法加载 Inbox 消息`
- 细节：请刷新或稍后重试
- 动作：`刷新`

#### 仅 unread 筛选为空

- 结论：`当前没有未读消息`
- 细节：可以切换到全部消息查看历史记录
- 动作：`查看全部消息`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- raw delivery metadata
- channel adapter 术语
- 把 Inbox 讲成“配置页”或“普通邮箱客户端”

这些内容可留在高级区，不应主导 Inbox 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/inbox` 已清晰表达为第一方送达工作台
- 页面默认先给消息、状态和行动入口，而不是 raw delivery 信息
- `/inbox` 与 `/channels`、session / execution 详情边界清晰

### 4.2 交互级验收

- 用户能顺畅标记已读、全部已读和进入相关对象
- 消息可操作性清晰
- 内联动作不会把用户直接带入配置视角

### 4.3 展示级验收

- 消息流默认展示 subject、read 状态、source、created_at
- 无消息、fetch 失败、仅 unread 为空等状态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖 unread、mark-read、跳转和关键错误态
- 需要浏览器或截图验收确认 Inbox 默认叙事已经从“配置或底层配送页”收口为“第一方消息工作台”
- 若后端尚未提供更丰富的归档或批量处理语义，前端不应伪装成已有完整消息中心能力

### 4.5 剩余限制说明

- `archived` 和更复杂的消息分类可作为下一阶段增强项
- raw delivery 诊断继续保留在高级区或 `Ops`
