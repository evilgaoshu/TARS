# TARS — Notification Templates 规范

> **状态**: 设计基线
> **适用范围**: 通知内容资产、模板预览、变量管理、渠道渲染覆写、与 Channels / Automations 的关系
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md)、[20-component-channels-and-web-chat.md](./20-component-channels-and-web-chat.md)、[20-component-automations.md](./20-component-automations.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Notification Template 是什么

`Notification Template` 是 **可复用的通知内容资产**。

它默认回答：

- 这条通知要说什么
- 面向什么通知场景
- 用什么语言
- 需要哪些变量

#### Notification Template 不是什么

- 不是推理 prompt
- 不是 Trigger 规则
- 不是 Channel endpoint 配置
- 不是把 Telegram HTML 直接塞进正文的渠道耦合配置

#### 命名原则

产品文案统一使用 `Notification Templates`。

当前 API 路由仍可兼容保留 `msg-templates`，但 UI 与 spec 不再强化 `Msg Templates` 这一偏实现命名。

### 1.2 用户目标与关键场景

#### 高频任务

- 创建一个通知模板
- 复制内置模板并做本地化调整
- 预览模板在真实变量下的效果
- 查看哪些自动化 / 送达路径在使用这个模板
- 停用或替换一个不再推荐的模板

#### 模板默认服务的通知类型

至少覆盖：

- `diagnosis_summary`
- `approval_request`
- `execution_result`
- `session_closed`
- `automation_run_result`

#### 当前最大问题

当前最大问题不是“模板种类太少”，而是：

- 模板只被当成 `type + locale + body` 的存储对象
- 真正的使用关系藏在 Trigger 里
- 渠道格式要求开始泄漏到正文层
- `template_id` 仍以 raw 输入框形式出现在主路径

### 1.3 状态模型

#### 生命周期状态

- `草稿`
- `生效`
- `已弃用`
- `已归档`

#### 渲染状态

- `可渲染`
- `变量缺失`
- `渠道覆写异常`
- `未知`

#### 使用状态

- `使用中`
- `未使用`

#### 展示优先级

1. 先看是否生效
2. 再看是否能渲染
3. 再看是否仍被使用

### 1.4 核心字段与层级

#### L1 默认必填字段

- `名称`
- `通知类型`
- `语言`
- `正文`

#### L2 条件显示字段

- `标题 / Subject`
- `CTA 文案`
- `渠道覆写`

#### L3 高级设置字段

- `变量白名单 / schema`
- `示例数据`
- `fallback locale`
- `render policy`

#### L4 系统隐藏字段

- `id`
- `compiled version`
- `storage metadata`
- raw export metadata

#### L5 运行时诊断字段

- 渲染错误明细
- 缺失变量清单
- 渠道覆写冲突明细

### 1.5 关键规则与约束

#### 应合并、删除、重命名或下沉的字段

- `type` -> `通知类型`
- `enabled` 不再承担全部生命周期语义，补充 `draft / active / deprecated / archived`
- raw `template_id` 输入框从 Trigger / Channel 主路径移除
- Channel 上的 `默认模板` 改为 `render profile / compatible template kinds`
- 渠道专属格式片段默认下沉到 `channel overrides`

#### 模板选择与覆盖优先级

实际发送时，模板身份的解析顺序应是：

1. `Automation.delivery.template_id`
2. 显式的 delivery binding 默认模板引用
3. 对应 `notification_kind` 的 built-in fallback

`Channel` 只影响 render profile、渠道覆写和兼容性，不直接决定模板身份。

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 创建并编辑通知正文
- 复制一个内置模板做局部变体
- 在真实变量下预览模板效果
- 找到引用该模板的自动化与送达链路
- 将旧模板替换成新的生效版本

#### 任务边界

| 用户任务 | 主对象 | 不应作为主对象 |
|---------|--------|----------------|
| 改通知正文 | Notification Template | Trigger |
| 选送达渠道 | Channel / Automation | Notification Template |
| 选具体模板实例 | Automation / Delivery Binding | Channel |
| 改 AI Prompt | Prompt / Ops | Notification Template |
| 调整模板默认 locale 策略 | Notification Governance | `Ops` |

#### 首屏必须回答的 3 个问题

1. 这个模板用于哪类通知、是什么语言、是否仍可用
2. 当前内容能不能正常渲染，缺哪些变量
3. 哪些自动化或送达链路还在使用它

### 2.2 入口与页面归属

#### `Notification Templates`

产品对象名统一为 `Notification Templates`。

当前实现仍可复用 `/msg-templates` route 与对应 API；在导航和页面真正迁移前，spec、文案和对象边界一律按 `Notification Templates` 解释。

作为日常主入口，负责：

- 创建 / 编辑模板
- 预览和校验变量
- 启停与弃用
- 查看使用关系

#### `Automations`

自动化页负责选择“用哪个通知模板”，不负责直接编辑正文。

#### `Channels`

Channels 负责“往哪发、怎么发、怎么渲染”，不负责决定“默认用哪一份正文模板”。

#### Governance

治理页负责：

- 默认 locale 策略
- 全局变量约定
- per-channel render policy
- built-in / override 策略

#### `Ops`

仅保留：

- 导入导出
- 渲染诊断
- 原始模板恢复

### 2.3 页面结构

#### 创建流程

推荐流程：

1. 选择通知类型
2. 选择内置模板或空白模板
3. 编辑正文
4. 预览变量
5. 保存

#### 列表页

必须显示：

- `名称`
- `通知类型`
- `语言`
- `生命周期`
- `使用数`
- `最近更新`

#### 详情页

首屏优先顺序：

1. 实时预览
2. 正文与标题
3. 变量说明
4. 使用关系
5. 高级设置

主动作：

- `编辑模板`
- `预览渲染`
- `启用 / 弃用`

次级动作：

- `复制模板`
- `查看使用关系`

高级动作：

- `导出`
- `查看 raw`
- `恢复内置默认`

#### 与 Automations / Channels 的联动

- 在 Automation 编辑器中使用 picker 选择模板
- 在 Channel 详情页只展示“兼容的模板类型”“render profile”和预览，不提供正文主编辑入口

### 2.4 CTA 与操作层级

#### 主动作

- `创建模板`
- `编辑模板`
- `预览渲染`
- `启用 / 弃用`

#### 次级动作

- `复制模板`
- `查看使用关系`
- `替换模板`

#### 高风险动作

- `归档`
- `恢复内置默认`
- raw 导出 / 诊断

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在高级设置区
- L4/L5 不应默认占据列表页和详情首屏
- 渲染错误、缺失变量、override 冲突进入补充诊断区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Notification Templates`
- 对象名：`Notification Template`
- 通知类型字段：`通知类型`

#### 页面叙事

- 页面讲“通知内容资产”
- 不讲“Msg Templates”
- 不把 Notification Templates 讲成 Channel 配置或 Trigger 附属字段

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Notification Templates`
- 副标题应表达：管理通知正文、变量与多语言模板资产

#### 详情页

- 标题默认使用模板名称
- 副标题应聚焦通知类型、语言、生效状态与渲染状态

#### 创建 / 编辑页

- 创建标题：`创建模板`
- 编辑标题：`编辑模板`
- 文案应围绕正文、变量、预览和使用关系，而不是 raw `template_id`

### 3.3 CTA 文案

主路径默认使用：

- `创建模板`
- `编辑模板`
- `预览渲染`
- `启用`
- `弃用`

次级路径默认使用：

- `复制模板`
- `查看使用关系`
- `替换模板`

高级区允许：

- `导出`
- `查看 raw`
- `恢复内置默认`

### 3.4 状态文案

#### 空态

- 标题：`还没有通知模板`
- 说明：`先从系统模板复制一份，就能快速搭建诊断、审批和执行结果通知。`
- 动作：`创建模板`

#### 模板无法渲染

- 结论：`这个模板当前不能正常发送`
- 细节：说明缺少哪些变量或哪里存在渲染冲突
- 动作：`修复并预览`

#### 模板已弃用但仍被使用

- 结论：`这个模板仍被自动化或渠道使用`
- 细节：列出引用它的对象，并提示运行期仍会继续发送但应尽快替换
- 动作：`查看使用关系`、`替换模板`

#### 模板已归档但仍被引用

- 结论：`这个模板已不能继续用于活跃通知`
- 细节：列出仍在引用它的自动化或 delivery binding
- 动作：`替换模板`

#### 渠道覆写冲突

- 结论：`当前渠道覆写存在冲突`
- 细节：说明哪个 channel override 覆盖失败
- 动作：`编辑高级设置`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- `Msg Templates`
- raw `template_id`
- 渠道 endpoint 配置词汇
- 渲染引擎内部字段名

这些内容可留在高级诊断区，不应主导 Notification Templates 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- 模块默认叙事已经统一为 `Notification Templates`
- 列表页清晰表达模板类型、语言、生命周期、使用数
- 详情页首屏先给预览、正文、变量和使用关系，再展示高级设置

### 4.2 交互级验收

- 创建流围绕“通知类型 -> 模板内容 -> 预览 -> 保存”组织
- Automation 使用 picker 选模板，而不是手输 `template_id`
- Channel 页面不再承担正文主编辑入口

### 4.3 展示级验收

- 空态、无法渲染、已弃用但仍被使用、已归档仍被引用、覆写冲突等状态都有清晰文案
- 使用关系可见，不再把真实引用藏在 Trigger 底层
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖列表摘要、详情预览区、模板创建流和关键状态文案
- 需要浏览器或截图验收确认模块默认叙事已经从 `Msg Templates` 和 raw id 输入收口
- 若后端尚未提供完整使用关系、变量 schema 或生命周期状态，前端不应伪装成已全部落地

### 4.5 剩余限制说明

- API route 可继续兼容 `/msg-templates`，但产品文案不再强调该命名
- Governance 继续承接 locale、render policy 和 built-in override 策略
- 渲染诊断与 raw 导出可继续放在高级区或 `Ops`
