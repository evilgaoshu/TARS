# TARS — Org 规范

> **状态**: 设计基线
> **适用范围**: organizations、tenants、workspaces、people、org / tenant policy
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-identity-access.md](./20-component-identity-access.md)、[20-component-agent-roles.md](./20-component-agent-roles.md)、[40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Org 是什么

`Org` 是 **组织治理对象域**，覆盖：

- `Organization`
- `Tenant`
- `Workspace`
- `People`
- `Org Policy`
- `Tenant Policy`

它不是纯 IAM 页，也不只是“公司信息”页面；`People` 这类组织 / 业务人物对象主归属在这里，而不是 `Identity` 主域。

#### Org 不是什么

- 不是人类访问控制的 RBAC 主对象
- 不是 `Agent Role` 的运行时身份模型
- 不是 raw policy 编辑器

### 1.2 用户目标与关键场景

#### 高频任务

- 管理 organization / tenant / workspace 层级
- 管理 people registry 与组织归属
- 启用或停用组织层对象
- 编辑 org policy 与 tenant override
- 查看 resolved effective policy

#### 关键场景

- 为平台建立组织层级和空间边界
- 将人物、组织归属和有效策略放在同一个治理视角里看清楚
- 在不进入 raw policy 的前提下，判断当前 effective policy 是怎样算出来的

### 1.3 状态模型

- `active`
- `disabled`
- `unlinked_person`
- `policy_overridden`
- `policy_resolved`
- `policy_conflicted`

#### 展示优先级

1. 组织对象是否启用
2. people 是否已正确归属
3. policy 是否冲突或已解析

### 1.4 核心字段与层级

#### L1 默认字段

- object id
- name
- type
- status
- hierarchy
- people display_name / primary org binding

#### L2 条件字段

- slug
- domain
- locale
- timezone
- description
- team / title / contact info

#### L3 高级字段

- policy booleans
- lists
- overrides
- resolved policy
- directory sync mapping

#### L4 系统隐藏字段

- raw policy internals
- sync payload
- migration metadata

#### L5 运行诊断字段

- save / load error detail
- resolve error detail

### 1.5 关键规则与约束

- `/org` 作为顶层组织治理入口，承接 org objects、people 与 policy
- `/identity/people` 仍可作为当前兼容路径，但对象定义和主编辑职责归 `Org`
- `/identity` 仅引用 people 作为访问治理关联对象，不接管其主配置
- `Ops` 仅负责 raw policy 导入导出、恢复与紧急修复

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 找到某个 organization / tenant / workspace
- 查看 people 与组织归属关系
- 编辑组织策略并查看 effective policy
- 判断这件事应该在 `/org`、`/identity` 还是 `Ops` 里处理

#### 首屏必须回答的 3 个问题

1. 当前组织层级是什么，哪些对象处于启用状态
2. people 是否已经正确归属到组织层级
3. 当前 policy 是否有覆盖、冲突或已解析结果

### 2.2 入口与页面归属

#### `/org`

作为顶层组织治理入口，承接：

- org objects
- people
- policy

#### `/identity/people`

仍可作为兼容路径，但对象定义和主编辑职责归 `Org`。

#### `/identity`

只引用 people 作为访问治理关联对象，不接管其主配置。

#### `Ops`

仅负责 raw policy 导入导出、恢复与紧急修复。

### 2.3 页面结构

推荐结构：

1. Tabs: organizations / tenants / workspaces / people / policy
2. organizations、tenants、workspaces tab 使用 registry + detail
3. people tab 使用 registry + profile side panel，并显示 org / tenant / workspace 归属
4. policy tab 展示结构化 policy editor 与 resolved effective policy
5. raw export / restore / emergency repair 跳转到 `Ops`

页面默认先回答“组织和人物关系是否清楚、策略是否已解析”，而不是先暴露 raw policy。

### 2.4 CTA 与操作层级

#### 主动作

- `创建 Organization`
- `创建 Tenant`
- `创建 Workspace`
- `创建 People Profile`
- `保存策略`

#### 次级动作

- `启用 / 停用`
- `查看 Effective Policy`
- `查看归属关系`

#### 高级动作

- `前往 Ops 导入 / 恢复`
- `查看策略冲突`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在策略区和高级属性区
- L4/L5 不应默认占据列表页和 profile 首屏
- raw policy internals、sync payload、migration metadata 进入高级区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Org`
- 对象名：`Organization`、`Tenant`、`Workspace`、`People`

#### 页面叙事

- 页面讲“组织治理”
- 不讲“IAM”
- 不把 `People` 讲成 `/identity` 的长期主对象

### 3.2 页面标题与副标题

#### 顶层页面

- 标题：`Org`
- 副标题应表达：管理组织层级、人物归属与组织策略

#### policy tab

- 标题：`Policy`
- 文案应围绕默认值、覆盖与 effective policy，而不是 raw policy 文件

### 3.3 CTA 文案

主路径默认使用：

- `创建 Organization`
- `创建 Tenant`
- `创建 Workspace`
- `创建 People Profile`
- `保存策略`

次级路径默认使用：

- `启用`
- `停用`
- `查看 Effective Policy`
- `查看归属关系`

高级区允许：

- `前往 Ops 导入 / 恢复`
- `查看策略冲突`

### 3.4 状态文案

#### 无组织对象

- 结论：`当前还没有组织对象`
- 细节：先创建 organization、tenant 或 workspace 建立组织结构
- 动作：`创建 Organization`

#### 无 people 记录

- 结论：`当前还没有 People 记录`
- 细节：可以创建人物资料，或从目录系统导入
- 动作：`创建 People Profile`

#### policy 保存失败

- 结论：`策略保存未成功`
- 细节：请检查表单冲突或必填项
- 动作：`重试保存`

#### resolved policy 缺失或解析失败

- 结论：`当前无法得到有效策略结果`
- 细节：请先选择 org / tenant，或修复策略冲突
- 动作：`查看策略冲突`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- raw policy internals
- sync payload
- migration metadata
- 把 `People` 讲成 IAM 主对象

这些内容可留在高级区，不应主导 Org 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/org` 已清晰表达为组织治理主入口
- organizations / tenants / workspaces / people / policy 边界清晰
- `/identity/people` 仅作为兼容路径，不再承担主叙事

### 4.2 交互级验收

- 用户能在一个主入口里完成层级管理、人物归属和 policy 编辑
- raw 导入导出与恢复已 handoff 到 `Ops`
- effective policy 可见，不必依赖 raw policy 视角

### 4.3 展示级验收

- 列表和详情默认展示层级、状态、归属和策略结论
- 无组织对象、无 people、policy 保存失败、策略解析失败等状态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖层级对象、people、policy 与关键错误态
- 需要浏览器或截图验收确认 Org 默认叙事已经从“杂糅 IAM / raw policy 页”收口为“组织治理工作台”
- 若后端尚未提供更完整的 effective policy 解析，前端不应伪装成已有完整治理解释器

### 4.5 剩余限制说明

- directory sync、advanced conflict analysis 可作为下一阶段增强项
- raw 导入导出、恢复与紧急修复继续留在 `Ops`
