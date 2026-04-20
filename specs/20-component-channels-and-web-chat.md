# TARS — Channels 与 Web Chat 规范

> **状态**: 设计基线
> **适用范围**: 统一 Channel 对象、Web Chat、Inbox、Telegram / Slack 等内外部触达渠道
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md)、[20-component-notification-templates.md](./20-component-notification-templates.md)、[20-component-automations.md](./20-component-automations.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Channels 是什么

`Channels` 是 TARS 的 **统一触达对象域**。

`Web Chat`、`Inbox`、`Telegram`、`Slack` 都属于 `Channel`。产品上不再强制把对象拆成 `surface` / `delivery` 两个互斥 class，而是通过 `kind + usages + capabilities` 表达能力：

- `kind`：`web_chat | inbox | telegram | slack | ...`
- `usages`：`conversation_entry`、`delivery_target`
- `capabilities`：例如 `supports_session_reply`

#### Channels 不是什么

- 不是只有一个 flat `target` 字段的 endpoint 列表
- 不是 adapter/runtime 实现细节的直出视图
- 不是把 `Web Chat`、`Inbox`、`Telegram Chat ID`、`Slack Webhook` 混成一类 flat 字段的对象
- 不是把“入口能力”和“送达能力”写死成两个互斥 class 的僵硬对象树

#### Web Chat 是什么

`Web Chat` 是第一方内置 `Channel.kind`。

它默认回答：

- 用户能否从 Web 进入对话
- 会话入口是否开放
- 默认是否回到当前会话，或在需要时落到 Inbox / 外部送达

#### Web Chat 不是什么

- 不是独立一级模块
- 不是 webhook URL 输入框
- 不是所有通知场景的通用 target id
- 不是必须与 Telegram 使用不同 `Channel` class 才能表达的特殊对象

### 1.2 用户目标与关键场景

#### 高频任务

- 开通 `Web Chat`
- 配置一个送达目标（Inbox / Telegram / Slack 等）
- 测试某个目标是否可送达
- 配置用户对话结束后的默认跟进路径
- 查看某个 Channel 被哪些自动化或通知规则使用

#### 关键场景

- 让用户能够进入会话并获得闭环回复
- 为自动化和通知选择一个稳定可送达的目标
- 区分“当前会话回复”“第一方收件箱”“外部送达”三种不同触达方式

### 1.3 状态模型

#### 生效状态

- `已启用`
- `维护中 / 已静默`
- `已禁用`

#### 可用状态

- `可用`
- `不可用`
- `降级`
- `未验证`
- `未知`

#### 配置状态

- `已配置`
- `缺目标地址`
- `缺凭据`
- `待补充`

#### 验证状态

- `最近验证通过`
- `最近验证失败`
- `未验证`

#### 展示优先级

1. 当前是否启用
2. 当前是否可用 / 可送达
3. 它承担哪些 `usages`
4. 最近验证与默认跟进状态

### 1.4 核心字段与层级

#### L1 默认必填字段

- `名称`
- `Channel kind`
- `启用状态`
- `usages`
- `主要入口 / 目标配置`

#### L2 条件显示字段

- `访问方式 / 入口路径`
- `目标地址 / 收件对象`
- `默认回路方式`
- `欢迎消息模板`
- `会话标题策略`
- `访客 / 登录态限制`
- `受众绑定`
- `凭据`

#### L3 高级设置字段

- `同会话跟进策略`
- `静默时段`
- `重试策略`
- `render profile`
- `附件限制`
- `rate limit`

#### L4 系统隐藏字段

- channel `id`
- secret ref
- 内部 session routing key
- raw capability matrix
- adapter raw config

#### L5 运行时诊断字段

- 最近握手 / 会话创建失败明细
- 最近送达失败原因
- retry history
- raw webhook response
- 事件物化 raw payload

### 1.5 关键规则与约束

#### usage 相关的条件显示规则

- `conversation_entry` 打开时，展示入口路径、访客 / 登录限制、会话策略
- `delivery_target` 打开时，展示目标地址、受众绑定、重试与静默策略
- Channel 声明 `supports_session_reply` 时，展示它能否承接“回当前会话”以及失败时 fallback route
- `reply_current_session` 是送达策略，不是单独的 channel target id；Channel 只声明是否支持该策略

#### 应合并、删除、重命名或下沉的字段

- flat `target` 删除为统一主字段，改成类型化目标配置
- `class: surface | delivery` 不再作为主对象模型，改为 `kind + usages`
- `reply_current_session` 从 Channel `usages` 移出，改为 delivery strategy + channel capability
- `linked_users` 下沉到受众 / 绑定区，不作为所有 channel 的默认主字段
- `默认模板` 从 Channel 主路径移除，改为 `render profile / compatible template kinds`
- raw `capabilities` 下沉高级区

#### 当前最大问题

当前最大问题不是“Channel 类型不够多”，而是：

- `Channel` 被同时当成 adapter、对象模型和 runtime 明细的收纳箱
- flat `target` 字段无法表达 `kind`、`usages` 与类型化配置
- `reply_current_session`、channel capability 和外部送达策略缺少清晰语义，容易被误塞进 `target`

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 开通 Web Chat 作为会话入口
- 创建一个 Telegram / Slack / Inbox 送达目标
- 判断某个 Channel 当前能不能正常工作
- 确认默认跟进路径和当前会话回复能力
- 找到哪些自动化、模板或规则在使用这个 Channel

#### 用户任务映射

| 用户任务 | 主对象 | 不应作为主对象 |
|---------|--------|----------------|
| 开通 Web Chat | `Channel(kind = web_chat)` | raw adapter config |
| 改 Telegram chat_id / webhook | `Channel(kind = telegram)` | `Ops` |
| 调整默认跟进路由 | Reach Governance | 单个 Channel 基础配置页 |
| 决定回复当前会话还是外部送达 | Automation / Reach Governance | 把 `web_chat` 当普通 target id |
| 调试 webhook / adapter | `Ops` | 日常对象页 |

#### 首屏必须回答的 3 个问题

1. 当前有哪些可用的触达对象，入口和送达能力分别是什么
2. 某个 Channel 现在能不能工作，问题出在配置、验证还是送达
3. 默认是回当前会话、走 Inbox，还是走外部目标

### 2.2 入口与页面归属

#### `/channels`

作为日常主入口，继续承载统一 `Channel` 对象。

默认分组应是：

- `First-party Channels`
- `External Channels`

每个 Channel 通过 badge 明确展示 `usages`：

- `会话入口`
- `送达目标`

如支持会话内回复，再显示二级 capability 标识：

- `支持当前会话回复`

`Web Chat` 应在 `First-party Channels` 中前置。

#### `/chat`

`/chat` 是工作台，不是配置入口。

这里负责：

- 开始会话
- 持续对话
- 查看当前会话上下文

#### `/inbox`

`/inbox` 是第一方送达工作台，不是 Web Chat 的子配置页。

#### Reach Governance

治理页负责：

- 默认跟进路由
- 默认送达优先级
- 共享受众 / recipient policy
- 静默规则 / quiet hours
- 默认 render / retry 策略

#### `Ops`

仅保留：

- raw webhook / adapter 配置
- delivery debug
- 事件回放
- 渠道级 emergency disable

### 2.3 页面结构

#### 创建流程

第一步必须先选 `Channel kind`：

1. `Web Chat`
2. `Inbox`
3. `Telegram / Slack / ...`
4. 选择该 kind 支持的 `usages`
5. 进入类型化表单
6. 验证并保存

#### Channels 列表页

必须体现“同一个 Channel class、不同 usages”的心智。

推荐分组：

- `First-party Channels`
- `External Channels`

每组卡片/列表至少显示：

- `名称`
- `kind`
- `usages`
- `生效状态`
- `可达 / 可用状态`
- `最近验证`
- `一句话状态摘要`

#### Web Chat 详情页

首屏优先回答：

1. Web Chat 是否已开放
2. 用户能否正常开始会话
3. 结果默认会送到哪里

主动作：

- `打开预览`
- `启用 / 关闭`
- `测试入口`
- `编辑入口配置`

#### 通用 Channel 详情页

首屏优先回答：

1. 当前这个 Channel 能否正常工作
2. 它承担哪些 `usages`
3. 哪些自动化 / 模板 / 路由在使用它

主动作：

- `测试`
- `编辑配置`
- `启用 / 禁用`

#### 高级动作

- webhook 调试
- 查看 raw 事件
- emergency mute
- raw adapter config

### 2.4 CTA 与操作层级

#### 主动作

- `添加 Channel`
- `测试`
- `编辑配置`
- `启用 / 禁用`

#### 次级动作

- `打开预览`
- `查看使用关系`
- `重新验证`

#### 高风险动作

- `emergency mute`
- raw config / webhook 调试
- adapter 级诊断

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在高级设置或扩展区
- L4/L5 不应默认占据创建流和详情首屏
- raw webhook response、事件 payload、capability matrix 进入高级调试区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Channels`
- 对象名：`Channel`
- 第一方入口对象：`Web Chat`
- 第一方送达工作台：`Inbox`

#### 页面叙事

- 页面讲“统一触达对象”
- 不讲“adapter 列表”
- 不把 Web Chat 讲成独立一级配置模块
- 不把 `reply_current_session` 讲成一个普通 target id

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Channels`
- 副标题应表达：管理会话入口、送达目标和默认触达能力

#### 详情页

- 标题默认使用 Channel 名称
- 副标题应聚焦当前可用性、承担的 `usages` 和默认跟进能力

#### 创建 / 编辑页

- 创建标题：`添加 Channel`
- 编辑标题：`编辑 Channel`
- 文案应围绕入口能力、送达能力和对象配置，而不是 runtime adapter 细节

### 3.3 CTA 文案

主路径默认使用：

- `添加 Channel`
- `测试`
- `编辑配置`
- `启用`
- `禁用`

次级路径默认使用：

- `打开预览`
- `查看使用关系`
- `重新验证`

高级区允许：

- `查看 raw 事件`
- `Webhook 调试`
- `临时静默`

### 3.4 状态文案

#### Channels 空态

- 标题：`还没有可用触达对象`
- 说明：`先开通 Web Chat 或至少一个送达目标，TARS 才能形成完整闭环。`
- 动作：`添加 Channel`

#### Web Chat 未开放

- 结论：`Web Chat 还没有对用户开放`
- 细节：展示入口路径未启用、域名未配置或会话策略阻止
- 动作：`开启入口`

#### Channel 不可送达

- 结论：`消息暂时发不出去`
- 细节：展示 token 过期、chat_id 无效、webhook 返回错误等信息
- 动作：`编辑 Channel`、`重新测试`

#### 默认跟进缺失

- 结论：`当前没有默认跟进路径`
- 细节：说明 Reach Governance 未配置 follow-up route
- 动作：`前往 Reach Governance`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- flat `target`
- `surface / delivery class`
- raw capability matrix
- adapter raw config
- `web_chat` 作为普通 delivery type 的默认叙事

这些内容可留在高级调试区，不应主导 Channels 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/channels` 已清晰表达为统一触达对象主入口
- `Web Chat` 被表达为第一方 `Channel.kind`，不是独立一级模块
- 列表页清晰区分第一方与外部渠道，并明确展示 `usages`

### 4.2 交互级验收

- 创建流先选 `kind`，再进入类型化表单
- 用户能区分“会话入口”“送达目标”“当前会话回复能力”
- Reach Governance 与 `Ops` 已从日常对象配置主路径中分离

### 4.3 展示级验收

- 列表摘要至少包含名称、kind、usages、生效状态、可用状态、最近验证
- 空态、Web Chat 未开放、不可送达、默认跟进缺失这几类状态都有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖列表分组、创建流、详情首屏和关键状态文案
- 需要浏览器或截图验收确认 Web Chat / Inbox / External Channels 的对象边界已经清楚
- 若后端尚未完整支持使用关系反查或高级跟进治理，前端不应伪装成已全量落地

### 4.5 剩余限制说明

- raw webhook / adapter 调试继续放在 `Ops`
- 默认跟进路由与静默策略继续由 Reach Governance 承接
- 运行时诊断细节可以继续保留在补充区块，不必强行塞进首屏
