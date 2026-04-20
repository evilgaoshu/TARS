# TARS — Skills 规范

> **状态**: 设计基线
> **适用范围**: skill registry、版本与修订、启停、导入导出、运行时编排边界
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-connectors.md](./20-component-connectors.md)、[20-component-extensions.md](./20-component-extensions.md)、[20-component-agent-roles.md](./20-component-agent-roles.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Skill 是什么

`Skill` 是 TARS 的 **场景化能力编排对象**。

它回答：

- 面对某个场景，应该优先调用哪些能力
- 哪些步骤是只读诊断，哪些需要审批
- 这个场景的运行摘要、文档与测试资产是什么

#### Skill 不是什么

- 不是 connector 的别名
- 不是 extension candidate 本身
- 不是另起一套执行状态机
- 不是绕过授权和审批的捷径

#### 当前真实心智

当前 `/skills` 更接近 **registry / package editor**：

- 管理 skill list 与详情
- 做 import/export 与 enable/disable
- 保留 revision / promote / rollback 的治理语义

#### 与 Extensions 的关系

- `Extensions` 管候选 bundle intake / review / import
- `Skills` 管已进入平台 registry 的正式 skill 对象

### 1.2 用户目标与关键场景

#### 高频任务

- 浏览现有 skills
- 创建或导入一个新 skill
- 编辑 skill 核心描述与运行边界
- 启用、停用或导出 skill
- 查看某个 skill 是否适合 planner 使用

#### 与 Connector 的职责分工

- Connector 负责“系统能做什么”
- Skill 负责“在这个场景下如何组合这些能力”

#### 关键场景

- 把一个成熟操作流程沉淀为可复用 skill
- 判断某个 skill 是否已通过治理、可被 planner 发现
- 追溯一个 skill 的来源、版本、文档和测试资产

### 1.3 状态模型

#### 生命周期状态

- `draft`
- `review`
- `active`
- `disabled`
- `deprecated`

#### 来源状态

- `official`
- `imported`
- `custom`
- `generated`

#### 运行可见状态

- `planner_visible`
- `tool_callable`
- `disabled`

#### 治理状态

- `approved`
- `pending`
- `changes_requested`
- `rejected`

#### 展示优先级

1. 当前是否启用
2. 当前来源和版本
3. 是否可被 planner 使用
4. 治理是否通过

### 1.4 核心字段与层级

#### L1 默认字段

- `skill_id`
- `display_name`
- `status`
- `source`
- `current_version`
- `summary`
- `runtime visibility`

#### L2 条件字段

- `preferred_tools`
- `trigger intents`
- `docs`
- `tests`
- `review_state`

#### L3 高级字段

- revision history
- package metadata
- governance policy
- export format

#### L4 系统隐藏字段

- raw package payload
- source sync metadata
- internal planner debug info

#### L5 运行诊断字段

- skill selected / expanded trace
- step-level validation warnings
- import / review history detail

### 1.5 关键规则与约束

- skill 不直接保存外部凭据
- skill 只描述能力编排与治理，不替代 connector config
- 高风险步骤必须能映射回 authorization / approval
- `/skills` 继续承接正式 skill registry
- `Extensions` 继续承接 bundle intake 与治理审核

#### 当前实现事实

- skill registry、enable/disable、import/export 已落地
- extension center 已承担 candidate validate / review / import 前置链路
- runtime 主链继续通过 tool-plan 与 connector capability 执行

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 找到某个 skill 是否存在、是否可用
- 浏览 skill 的摘要、来源、版本与治理状态
- 编辑 skill 的描述、工具偏好和运行边界
- 导出 skill 或查看其文档 / 测试资产

#### 任务映射

| 用户任务 | 主入口 | 不应作为主入口 |
|---------|--------|----------------|
| 管理已安装 skill | `/skills` | `Extensions` |
| 导入候选 skill bundle | `Extensions` | `/skills` |
| 调整运行边界 | `/skills/{id}` | connector 详情 |
| 查看 bundle 文档与测试资产 | `/skills/{id}` | `/ops` |

#### 首屏必须回答的 3 个问题

1. 这个 skill 是做什么的、当前是否启用
2. 它来自哪里、版本是什么、是否通过治理
3. planner 能不能看到它、运行时会不会调用它

### 2.2 入口与页面归属

#### `/skills`

作为日常主入口，负责：

- registry inventory
- 详情查看与编辑
- 启停与导出

#### `Extensions`

负责新 bundle 的：

- validate
- review
- import

#### Sessions / Executions

这些工作台消费 skill 运行结果，但不承担 skill 主配置职责。

#### `Ops`

仅保留 raw import 与兼容修复，不承担日常 skill 管理。

### 2.3 页面结构

#### 列表页

必须展示：

- 名称
- 状态
- 来源
- 当前版本
- 一句话摘要

#### 详情页

首屏优先顺序：

1. 核心摘要与状态
2. planner / preferred tools
3. governance 边界
4. docs / tests 资产
5. revisions / export

#### 关联视图

后续应补充：

- 最近哪些 sessions / executions 选择了该 skill
- 哪些 connectors / capabilities 是该 skill 的主要依赖

### 2.4 CTA 与操作层级

#### 主动作

- `创建 Skill`
- `编辑`
- `启用 / 停用`
- `导出`

#### 次级动作

- `查看文档`
- `查看测试`
- `查看 revision`

#### 高级动作

- `Promote`
- `Rollback`
- `Import from bundle`
- `Validate / Simulate`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在高级设置、修订或导出区
- L4/L5 不应默认占据列表页和详情首屏
- raw package、planner debug、step warnings 进入高级调试区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Skills`
- 对象名：`Skill`
- 候选导入中心：`Extensions`

#### 页面叙事

- 页面讲“场景化能力编排资产”
- 不讲“connector 列表”
- 不把 Skills 讲成 extension candidate 收纳箱或原始 bundle 管理器

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Skills`
- 副标题应表达：管理正式注册的技能资产、版本与运行边界

#### 详情页

- 标题默认使用 skill 显示名
- 副标题应聚焦 skill 摘要、来源、版本与治理状态

#### 创建 / 编辑页

- 创建标题：`创建 Skill`
- 编辑标题：`编辑 Skill`
- 文案应围绕场景、工具组合、治理边界和资产说明，而不是 raw package 细节

### 3.3 CTA 文案

主路径默认使用：

- `创建 Skill`
- `编辑`
- `启用`
- `停用`
- `导出`

次级路径默认使用：

- `查看文档`
- `查看测试`
- `查看 revision`

高级区允许：

- `Promote`
- `Rollback`
- `Validate / Simulate`

### 3.4 状态文案

#### 没有 skill

- 结论：`当前还没有可用 skill`
- 细节：可以从官方 bundle 或 extension candidate 导入
- 动作：`前往 Extensions` 或 `创建 Skill`

#### skill 已安装但未启用

- 结论：`该 skill 不会参与 planner`
- 细节：当前状态为 `disabled` 或 `review`
- 动作：`启用 Skill`

#### skill 版本冲突

- 结论：`当前版本或来源存在冲突`
- 细节：revision、import 来源或 bundle 不一致
- 动作：`查看 revision`、`回滚`

#### skill 治理未通过

- 结论：`skill 还不能进入正式 registry`
- 细节：validation 或 review 未通过
- 动作：`前往 Extensions 修复`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- raw package payload
- internal planner debug info
- source sync metadata
- 把 `Extensions` 和 `Skills` 混成同一对象的叙事

这些内容可留在高级调试区，不应主导 Skills 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/skills` 已清晰表达为正式 skill registry
- `Extensions` 与 `Skills` 的对象边界清晰
- 详情页首屏先给摘要、状态、工具组合和治理边界，再展示 revision / export

### 4.2 交互级验收

- 用户能在主路径里完成创建、编辑、启停、导出
- 导入候选 bundle 的动作仍然回到 `Extensions`
- 高风险和底层治理动作已下沉，不打断日常 skill 管理

### 4.3 展示级验收

- 列表摘要至少包含名称、状态、来源、版本、摘要
- 空态、未启用、版本冲突、治理未通过几类状态都有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖列表摘要、详情主区块和关键状态文案
- 需要浏览器或截图验收确认 Skills 默认叙事已经从“bundle / connector 混合页”收口
- 若后端尚未提供完整 usage relationship 或 simulate 能力，前端不应伪装成已落地

### 4.5 剩余限制说明

- revision/history、usage relationship、validate/simulate 仍可作为下一阶段增强项
- raw import/export 和兼容修复继续保留在 `Extensions` 或 `Ops`
