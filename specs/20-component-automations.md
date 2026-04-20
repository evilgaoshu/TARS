# TARS — Automations 规范

> **状态**: 设计基线
> **适用范围**: 定时任务、事件触发、执行动作、角色边界、送达配置、运行追踪
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md)、[30-strategy-automations-and-triggers.md](./30-strategy-automations-and-triggers.md)、[20-component-providers-and-agent-role-binding.md](./20-component-providers-and-agent-role-binding.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Automation 是什么

`Automation` 是 **受治理的闭环规则对象**。

它默认回答：

- 什么时候触发
- 要执行什么动作
- 以哪个 Agent Role 运行
- 结果要通知谁
- 结果是回当前会话、送到外部渠道，还是同时做两者

#### Automation 不是什么

- 不是只有 cron 的 scheduler 对象
- 不是只读 capability 调用清单
- 不是把 Trigger、Channel、Template 分散到多个页面才能拼起来的半成品
- 不是 Hook / worker / raw event name 的直出控制台

#### 当前最大问题

当前最大问题不是“Automation 字段不够”，而是：

- 用户要做一个完整自动化，需要跨 `Automations`、`Triggers`、`Channels`、`Notification Templates`
- 日常对象心智被 runtime `target_ref / event_type / worker` 细节打断
- `Triggers / Hooks` 在当前产品里被错误抬成了主导航对象

### 1.2 对象边界

#### Trigger / TriggerPolicy / Hook 的关系

- `Automation` 是用户主对象
- `Trigger / TriggerPolicy` 是 Automation 的规则底层，直接编辑时应进入 `Governance / Advanced`
- `Hook` 是系统内部扩展点，归 `Governance / Advanced` 或 `Ops`，不进入日常主 IA

#### 默认服务用户

- 自动化 owner
- 平台管理员
- 运维负责人

### 1.3 用户目标与关键场景

#### 高频任务

- 创建一个定时巡检自动化
- 创建一个事件触发的通知/跟进自动化
- 给自动化指定执行角色
- 给自动化配置通知渠道和模板
- 查看最近一次运行发生了什么

#### 关键场景

- 定时巡检与汇总通知
- 事件触发的审批/执行后续动作
- 会话关闭或任务完成后的消息推送

### 1.4 状态模型

#### 生命周期状态

- `草稿`
- `已启用`
- `已暂停`
- `已归档`

#### Trigger 状态

- `按计划运行`
- `等待事件`
- `冷却中`
- `配置无效`

#### Execution 状态

- `运行中`
- `等待审批`
- `被策略阻止`
- `最近成功`
- `最近失败`

#### Notification / Delivery 状态

- `回复当前会话`
- `外部送达`
- `双通道`
- `通知异常`
- `未配置通知`

#### 展示优先级

- 先显示生命周期
- 再显示最近运行结论
- 再显示通知方式与补充状态
- 最后展示 trigger 高级细节

### 1.5 核心字段与层级

#### L1 默认必填字段

- `名称`
- `触发方式`
- `动作类型`
- `动作目标`
- `执行角色`
- `通知方式`
- `启用状态`

#### L2 条件显示字段

- `schedule`
- `event filter`
- `通知目标`
- `通知受众`
- `通知模板`
- `审批要求`

#### L3 高级设置字段

- `重试`
- `超时`
- `冷却`
- `去重`
- `fallback delivery`
- `payload transform`

#### L4 系统隐藏字段

- `target_ref`
- `runtime_mode`
- internal `event_type`
- queue / shard metadata

#### L5 运行时诊断字段

- last matched event raw payload
- worker failure detail
- retry history
- scheduler heartbeat detail

### 1.6 关键规则与约束

#### 应合并、删除、重命名或下沉的字段

- `target_ref` -> 产品层重命名为 `动作目标`
- Trigger 页里的 `channel + template_id` 主编辑入口 -> 下沉为 Automation 的 delivery 区
- `reply_current_session` -> 作为 delivery strategy，而不是 channel target id
- `Owner` 保留为治理/审计字段，不作为首屏核心字段

#### 关键约束

- `Automation` 必须是用户主对象
- 通知渠道、模板、受众与 delivery strategy 应回到 Automation 主对象中表达
- Trigger / Hook / raw event routing 不应成为普通用户创建自动化的主入口

### 1.7 API 映射与演进方向

#### 当前 API

- `GET /api/v1/automations`
- `GET /api/v1/automations/{id}`
- `POST /api/v1/automations`
- `PUT /api/v1/automations/{id}`
- `POST /api/v1/automations/{id}/enable`
- `POST /api/v1/automations/{id}/disable`
- `POST /api/v1/automations/{id}/run`

当前列表/详情还以以下字段为主：

- `id`
- `display_name`
- `type`
- `schedule`
- `target_ref`
- `skill`
- `connector_capability`
- `agent_role_id`

#### 当前 API 的问题

- delivery / notification 信息不在 Automation 主对象里
- `target_ref` 暴露过强实现心智
- `reply_current_session`、受众和 delivery mode 缺少显式语义
- Trigger 规则仍有独立编辑路径，打散用户任务

#### 推荐 API 对齐方式

Automation payload 应显式包含：

- `trigger`
- `action`
- `execution.agent_role_id`
- `delivery.mode`
- `delivery.targets`
- `delivery.audience`
- `delivery.template_id`
- `delivery.locale`

#### Trigger / Hook API 的推荐定位

现有 Trigger / Hook API 可以继续保留，但建议产品定位为：

- 高级规则层
- 兼容历史 built-in 事件路由
- Governance / Advanced 入口
- Delivery / event runtime 调试入口

而不是 Automation 的主创建入口。

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 创建一个定时巡检自动化
- 创建一个事件触发的通知 / 跟进自动化
- 给自动化指定执行角色
- 给自动化配置通知渠道和模板
- 查看最近一次运行发生了什么

#### 任务边界

| 用户任务 | 主对象 | 不应作为主对象 |
|---------|--------|----------------|
| 设定何时执行 | Automation | Trigger 页面 |
| 选择执行角色 | Automation | Agent Role 页面 |
| 选择通知渠道和模板 | Automation | Trigger 页面 |
| 决定回复当前会话还是外部送达 | Automation | 把 `web_chat` 当通用 target id |
| 调试 worker / scheduler | `Ops` | Automation 首屏 |

#### 首屏必须回答的 3 个问题

1. 这个自动化什么时候、因为什么触发
2. 它会执行什么动作、以哪个角色运行
3. 结果会通知到哪里、最近一次运行怎样

### 2.2 入口与页面归属

#### `/automations`

作为绝对主入口，负责：

- 创建 / 编辑自动化
- 启停
- 查看最近运行
- Dry run / 手动运行
- 管理通知与执行角色

#### `Triggers / Hooks / Advanced Rules`

不再作为日常主导航对象。

应进入 `Governance / Advanced`，负责：

- 高级规则页
- Trigger Policy 治理页
- Hooks / 事件路由高级配置

#### Governance

治理页负责：

- 默认审批策略
- 冷却、去重、静默策略模板
- 复用 Trigger Policy
- Hooks / 事件路由等高级规则入口

#### `Ops`

仅保留：

- raw jobs config
- scheduler repair
- worker replay
- emergency disable

### 2.3 创建流程

推荐流程：

1. 选择场景模板
   - `定时巡检`
   - `事件通知`
   - `跟进执行`
2. 配置 `When`
3. 配置 `Do`
4. 配置 `Run As`
5. 配置 `Notify`
6. Review 并保存

创建流应围绕完整闭环对象展开，不要求用户跳去 Trigger / Channel / Template 页面拼装。

### 2.4 列表页

列表页必须显示：

- `名称`
- `触发摘要`
- `动作摘要`
- `执行角色`
- `生命周期`
- `最近运行`
- `下一次运行 / 等待事件`

列表页默认先回答“这个自动化会不会运行、最近运行得怎么样”，而不是先展示 raw trigger 细节。

### 2.5 详情页

详情页首屏优先顺序：

1. 状态摘要
2. When
3. Do
4. Run As / Risk
5. Notify
6. Run History

#### 主动作

- `立即运行`
- `启用 / 暂停`
- `编辑`

#### 次级动作

- `Dry Run`
- `复制`
- `查看最近运行`

#### 高级动作

- `回放 payload`
- `查看 raw rule`
- `归档`

### 2.6 Governance / Advanced 中的 Triggers / Hooks

`Triggers / Hooks` 若继续保留，应只承载：

- 高级规则治理
- built-in trigger policy 管理
- Hooks / 事件路由高级配置

不再承担“普通用户创建通知自动化”的主流程。

### 2.7 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在高级设置或扩展区
- L4/L5 不应默认占据创建流和详情首屏
- 运行时诊断信息进入最近运行、诊断或高级调试区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Automations`
- 对象名：`Automation`
- 高级规则区：`Governance / Advanced`

#### 页面叙事

- 页面讲“闭环自动执行对象”
- 不讲“只有 cron 的 scheduler”
- 不把 Automations 讲成 Trigger / worker 控制台

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Automations`
- 副标题应表达：何时触发、执行什么、以谁执行、通知谁

#### 详情页

- 标题默认使用自动化名称
- 副标题应聚焦当前状态、最近运行和下一步动作

#### 创建 / 编辑页

- 创建标题：`创建自动化`
- 编辑标题：`编辑自动化`
- 文案应围绕 When / Do / Run As / Notify，而不是 raw trigger 字段

### 3.3 CTA 文案

主路径默认使用：

- `创建自动化`
- `立即运行`
- `启用`
- `暂停`
- `编辑`

次级路径默认使用：

- `Dry Run`
- `复制`
- `查看最近运行`

高级区允许：

- `回放 payload`
- `查看 raw rule`
- `归档`

### 3.4 状态文案

#### 空态

- 标题：`还没有自动化`
- 说明：`先从定时巡检或事件通知模板创建一个自动化，形成最小闭环。`
- 动作：`创建自动化`

#### 自动化当前不会执行

- 结论：`这个自动化当前不会执行`
- 细节：说明是未启用、schedule 无效、事件源缺失还是被策略阻止
- 动作：`编辑触发条件`

#### 运行被角色策略阻止

- 结论：`当前执行角色不允许这个动作`
- 细节：说明是风险等级过高、需要审批或被 hard deny
- 动作：`更换执行角色`、`补审批链`

#### 通知配置不完整

- 结论：`自动化可以运行，但通知配置不完整`
- 细节：说明缺少渠道、模板或受众配置
- 动作：`补充通知配置`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- `target_ref`
- raw `event_type`
- worker / shard metadata
- scheduler heartbeat detail
- queue internals

这些内容可留在高级规则、诊断或 Ops，不应主导 Automations 的默认叙事。

---

## 4. 验收清单

### 4.1 页面级验收

- `/automations` 成为普通用户的绝对主入口
- 首屏默认先讲闭环对象，而不是 trigger/runtime 细节
- 详情页首屏按状态摘要、When、Do、Run As、Notify、Run History 组织
- 通知、执行角色与 delivery strategy 均在 Automation 主对象中表达

### 4.2 交互级验收

- 创建流按场景模板 -> When -> Do -> Run As -> Notify -> Review 组织
- 用户不需要跳到 Trigger 页面完成普通自动化创建
- `reply_current_session` 被当作 delivery strategy，而不是 channel target id
- 高级规则编辑已下沉到 Governance / Advanced

### 4.3 展示级验收

- 列表页至少展示名称、触发摘要、动作摘要、执行角色、生命周期、最近运行、下一次运行 / 等待事件
- 空态、不会执行、角色策略阻止、通知配置不完整这几类状态都有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖列表摘要、创建流、详情主区块和关键状态文案
- 需要浏览器或截图验收确认 Automations 不再要求跨多个页面拼装主任务
- 如果后端暂未把 delivery 完整收进 Automation payload，前端不应伪装成完全闭环已落地

### 4.5 剩余限制说明

- Trigger / Hook API 可以继续作为兼容层存在
- 高级规则治理继续放在 `Governance / Advanced`
- 在 Automation payload 尚未完全补齐前，前端应通过命名与 IA 减少用户对 runtime 细节的直接暴露
