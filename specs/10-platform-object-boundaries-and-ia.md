# TARS — 对象边界与信息架构总规范

> **状态**: 设计基线  
> **适用范围**: Providers、Agent Role Binding、Channels、Web Chat、Notification Templates、Automations  
> **关联**: [10-platform-components.md](./10-platform-components.md)、[30-strategy-platform-config-and-automation.md](./30-strategy-platform-config-and-automation.md)、[40-web-console.md](./40-web-console.md)、[40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md)

---

## 1. 对象定义

### 1.1 这组模块是什么

这组模块共同构成 TARS 的 **AI 接入、用户触达、通知内容与自动执行闭环层**。

它回答的不是“底层配置文件有哪些字段”，而是以下产品问题：

- TARS 接哪个 AI 后端
- 哪个 Agent Role 用哪个模型
- 用户从哪里进入对话
- 结果发到哪里
- 什么情况下自动执行并通知谁

### 1.2 这组模块不是什么

- 不是 raw YAML / manifest 浏览器
- 不是 runtime 调试对象的大杂烩
- 不是所有配置都堆进 `Ops` 的临时收纳箱
- 不是把平台元字段、运行时字段、用户配置字段平铺在一个表单里

### 1.3 默认服务用户

- 平台管理员
- AI / 产品 owner
- 自动化 owner
- 运维负责人

### 1.4 默认高频任务

- 接入一个可用的 AI Provider，并让某个 Agent Role 使用它
- 开通一个会话入口或送达目标，尤其是 `Web Chat`
- 创建一个“会执行、会通知、可追踪”的 Automation

### 1.5 核心对象边界

#### Provider

`Provider` 是 **可复用的 AI 后端连接对象**。

它只回答：

- 接的是哪类 AI 服务
- 地址和认证是否正确
- 模型是否可发现/可用
- 当前是否健康

它不回答：

- 哪个 Agent Role 应该用它
- 平台当前主模型/辅助模型是谁
- 这个模型承担什么人格或职责

#### Agent Role Binding

`Agent Role Binding` 是 **角色到模型的绑定关系**。

它只回答：

- 哪个 Agent Role 用哪个 Provider / Model
- 这是主绑定、兜底绑定还是继承平台默认
- 当前绑定是否有效

它不保存：

- Provider 凭据
- Provider 连接参数
- runtime 探测日志

#### Channel

`Channel` 是 **统一的触达对象**。

它承载 TARS 与第一方或第三方界面的连接能力，但产品上不再强制拆成两个互斥对象 class。

同一个 `Channel` 可以通过 `kind + usages + capabilities` 表达不同用法：

- `kind`：例如 `web_chat`、`inbox`、`telegram`、`slack`
- `usages`：`conversation_entry`、`delivery_target`
- `capabilities`：例如是否支持当前会话内回复、附件、富文本等能力

`reply_current_session` 是 **delivery strategy**，定义在 `Automation / delivery binding`，而不是 `Channel` 自身的 usage。

### 1.6 Web Chat 的定位

`Web Chat` 是 `Channel.kind = web_chat` 的第一方内置 channel。

它通常承担默认会话入口，并可作为 `reply_current_session` 的首选闭环体验；但它仍属于 `Channels`，不是独立一级模块。

它不是：

- 一个普通的 webhook target
- 所有通知场景的通用 target id
- 应与 Telegram Chat ID、Slack Webhook、Inbox 收件目标混成同一 flat `target` 字段的对象

### 1.7 Notification Template 的定位

`Notification Template` 是 **通知内容资产**。

它不是：

- 推理 prompt
- trigger DSL
- channel 配置

### 1.8 Automation 的定位

`Automation` 是 **受治理的闭环规则对象**：

> 在某个条件下，以某个 Agent Role，执行某个动作，并按指定方式通知。

`Trigger`、`Hook`、`TriggerPolicy`、runtime worker 都是 Automation 的支撑结构，不应该成为日常主心智。

### 1.9 当前最大问题

当前最大问题不是“缺字段”，而是：

- 对象心智混乱
- 主入口归属错误
- 平台字段和用户字段混层
- runtime / manifest / platform defaults 被直接暴露给日常用户

---

## 2. 用户任务

### 2.1 用户最想完成的 4 个任务

1. 让 TARS 有可用模型，并知道“谁用什么模型”
2. 让用户能从 `Web Chat` 或其他入口和 TARS 交互
3. 让结果能通过合适渠道送达，并保证通知内容一致
4. 让自动化能够按预期执行、通知、追踪和治理

### 2.2 任务到对象的映射

| 用户任务 | 主对象 | 次对象 | 不应该成为主对象 |
|---------|--------|--------|------------------|
| 接入 AI 后端 | Providers | Agent Roles | `Ops`、raw config |
| 给角色选模型 | Agent Roles | Providers | Providers 列表页中的全局 `primary/assist` |
| 开通用户入口 | Channels（尤其是 `Web Chat`） | Notification Templates | flat `target` 字段 |
| 配置送达与消息 | Automations / Channels / Notification Templates | Reach Governance | Trigger page、raw `template_id` |
| 创建自动闭环 | Automations | Agent Roles / Channels / Templates | Hook / worker / raw event names |

### 2.3 任务优先级

- 第一优先级：完成最小闭环
  - Provider 可用
  - Web Chat 可用
  - 至少一个 Notification Template 可用
  - 至少一个 Automation 可用
- 第二优先级：治理和扩展
- 第三优先级：raw config、导入导出和诊断

---

## 3. 入口归属

### 3.1 总体入口原则

- 高频对象配置归对象页
- 跨对象默认与策略归治理页
- raw config、导入导出、修复、紧急操作归 `Ops`

### 3.2 推荐导航分组

- `Operate`
  - `AI`
    - `Providers`
    - `Agent Roles`
  - `Reach`
    - `Channels`
    - `Notification Templates`
  - `Automations`
- `Governance`
- `Ops`

### 3.3 对象页、治理页、Ops 分工

| 域 | 对象页负责 | 治理页负责 | `Ops` 负责 |
|----|------------|------------|------------|
| AI | Provider 连接、健康、模型发现；Role 模型绑定 | 平台默认绑定、共享安全模型、成本/路由策略 | raw providers config、导入导出、修复 |
| Reach | Channel 创建、类型化配置、验证、启停 | 默认跟进路由、共享受众策略、送达策略 | raw webhook/debug、事件回放 |
| Notifications | 模板内容、预览、启停、使用关系 | 默认 locale、渲染策略、全局变量策略 | 导出、恢复、诊断 |
| Automations | 创建/编辑自动化、查看运行、启停 | 复用触发策略、Hooks / 高级事件规则、审批默认、静默时窗 | scheduler/raw jobs、手工修复、紧急停用 |

### 3.4 不应放在 `Ops` 的内容

- 日常改 Provider 地址/认证
- 日常给 Agent Role 选模型
- 日常开关 Web Chat
- 日常编辑通知模板正文
- 日常编辑 Automation 的 schedule / action / notification

---

## 4. 字段分层

### 4.1 统一分层模型

所有对象都遵循同一套字段层级：

#### L1 默认必填字段

用户为了完成主任务必须立刻填写或理解的字段。

#### L2 条件显示字段

只有在特定类型、模式或场景下才显示的字段。

#### L3 高级设置字段

低频、复杂、环境相关或治理相关的字段。

#### L4 系统隐藏字段

系统标识、secret ref、manifest 元数据、导入导出元信息等默认不应打扰用户的字段。

#### L5 运行时诊断字段

probe raw、worker payload、审计原文、错误栈、兼容性 raw detail 等，仅在诊断模式展开。

### 4.2 全局裁剪规则

- 不把平台默认绑定和对象连接配置混在一个表单
- 不把 manifest 字段和用户业务字段混在一个层级
- 不把 runtime payload 和对象状态摘要混在首屏
- 不把 secret ref 暴露成用户主字段

### 4.3 推荐重命名与下沉

- `Msg Templates` -> `Notification Templates`
- `provider_preference` -> `model_binding`
- Providers 上的 `primary_model / assist_model` -> 下沉为平台治理或角色绑定
- Channels 上的 flat `target` -> 改为 `kind + usages + typed config + capabilities`
- `surface / delivery` -> 不再作为互斥对象 class，改为 channel 的 usage/capabilities
- Automations 上的“通知谁” -> 显式化为 `delivery.targets / delivery.audience / reply_current_session`
- Automations 上的 `target_ref / runtime_mode` -> 改为用户可理解的 `动作目标 / 执行方式`，原字段下沉

---

## 5. 页面结构

### 5.1 统一创建流程原则

所有对象的创建流程都应遵循：

> 先选对象类型或模板，再填最少字段，再验证，再保存。

不应从空白大表单开始。

### 5.2 统一列表页原则

列表页必须优先展示：

- 名称
- 类型/用途
- 生效状态
- 健康/可用状态
- 最近验证或最近运行
- 一句话状态摘要

### 5.3 统一详情页原则

详情页首屏必须优先回答：

1. 这个对象现在是否生效
2. 它现在是否可用
3. 用户下一步最可能做什么

### 5.4 动作层级原则

#### 主动作

- 测试 / 运行 / 编辑 / 启停

#### 次级动作

- 保存并复测
- 复制
- 查看使用关系

#### 高级动作

- 导入导出
- raw config
- 版本历史
- 回放 / 诊断 / 修复

### 5.5 Web Chat 的页面归属

- `/chat` 是工作台，不是配置页
- `Web Chat` 配置在 `/channels` 中，以 `kind = web_chat` 的第一方 channel 展示
- `reply_current_session` 是送达策略，不是独立 channel id
- `Inbox` 是第一方送达页，不应和 `Web Chat` 混成同一个配置对象

---

## 6. 状态模型

### 6.1 统一展示优先级

所有对象统一按以下优先级展示状态：

1. 生效状态
2. 健康/可达状态
3. 配置状态
4. 兼容/渲染状态
5. 最近测试/最近运行结果

### 6.2 不允许的反模式

- 把所有状态揉成一个 badge
- 用 `enabled` 同时表达“已配置”“可连接”“测试通过”
- 用一条 raw error 顶替业务结论

### 6.3 建议的统一状态维度

| 状态维度 | 回答的问题 |
|----------|------------|
| 生效状态 | 这个对象是否参与运行 |
| 健康状态 | 参与运行时是否正常 |
| 配置状态 | 用户信息是否补齐 |
| 兼容状态 | 当前模式是否可支持 |
| 测试/运行状态 | 最近一次主动验证或执行结果 |

---

## 7. 空态 / 错误态

### 7.1 统一文案原则

所有提示都遵循：

1. 先给业务结论
2. 再给技术细节
3. 最后给下一步动作

### 7.2 统一空态原则

- 空态要解释“为什么这件事值得先做”
- 空态要给单一明确主动作
- 空态不要把用户直接送去 `Ops`

### 7.3 统一错误态原则

- 区分“未配置”与“真实故障”
- 区分“对象不可用”与“治理策略阻止”
- 区分“当前页面可修复”与“需要去治理页或 Ops”

### 7.4 页面跳转原则

只有在以下场景才把用户送往 `Ops`：

- 需要 raw config 修复
- 需要导入导出
- 需要诊断原始 payload / worker / webhook
- 需要平台级紧急操作

---

## 8. API 映射附录

### 8.1 当前对象 API

- Providers
  - `GET /api/v1/providers`
  - `GET /api/v1/providers/{id}`
  - `POST /api/v1/providers`
  - `PUT /api/v1/providers/{id}`
  - `GET /api/v1/providers/bindings`
  - `PUT /api/v1/providers/bindings`
- Agent Roles
  - `GET /api/v1/agent-roles`
  - `GET /api/v1/agent-roles/{role_id}`
  - `PUT /api/v1/agent-roles/{role_id}`
- Channels
  - `GET /api/v1/channels`
  - `GET /api/v1/channels/{id}`
  - `POST /api/v1/channels`
  - `PUT /api/v1/channels/{id}`
- Notification Templates
  - `GET /api/v1/msg-templates`
  - `GET /api/v1/msg-templates/{id}`
  - `POST /api/v1/msg-templates`
  - `PUT /api/v1/msg-templates/{id}`
  - `POST /api/v1/msg-templates/{id}/render`
- Automations
  - `GET /api/v1/automations`
  - `GET /api/v1/automations/{id}`
  - `POST /api/v1/automations`
  - `PUT /api/v1/automations/{id}`
  - `POST /api/v1/automations/{id}/run`
  - `POST /api/v1/automations/{id}/enable`
  - `POST /api/v1/automations/{id}/disable`

### 8.2 当前原始配置 / Ops API

- `GET /api/v1/config/providers`
- `PUT /api/v1/config/providers`
- `GET /api/v1/config/connectors`
- `PUT /api/v1/config/connectors`
- `GET /api/v1/config/auth`
- `PUT /api/v1/config/auth`

### 8.3 推荐 API 归属规则

- 对象 CRUD 继续走对象 API
- 平台默认绑定、跨对象策略、复用规则走治理 API 或治理域内资源
- raw config、导入导出、修复继续留在 `/api/v1/config/*` 或 `Ops` 专用接口

### 8.4 推荐演进方向

- Providers 不再通过对象 DTO 暴露 `primary_model / assist_model`
- Agent Roles 改为显式暴露 `model_binding`
- Channels 改为 `kind + usages + typed config`，不再依赖单一 `target`
- Notification Templates 继续兼容 `msg-templates` 路由，但产品文案统一改为 `Notification Templates`
- Automations 继续保留对象 API，但把 trigger / delivery / audience / role 信息统一收口到 Automation 主对象中
