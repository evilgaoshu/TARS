# TARS — Agent Roles 规范

> **状态**: 设计基线
> **适用范围**: AI 角色画像、能力边界、风险策略、模型绑定、运行时继承与回退
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-providers-and-agent-role-binding.md](./20-component-providers-and-agent-role-binding.md)、[20-component-identity-access.md](./20-component-identity-access.md)、[20-component-automations.md](./20-component-automations.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Agent Role 是什么

`Agent Role` 是 TARS 的 **AI 运行时身份对象**。

它回答：

- 这个 AI 以什么角色工作
- 可以调用哪些能力、技能与动作
- 最大风险边界是什么
- 默认使用哪个 `model_binding`
- 在 session / execution / automation 中如何被继承

#### Agent Role 不是什么

- 不是人类 `RBAC Role`
- 不是 `Provider` 连接配置
- 不是单独一份 prompt 文件
- 不是 `Skill` 本身

#### 与 Provider 的关系

- `Provider` 负责 AI 后端连接、模型目录、健康与凭据
- `Agent Role` 负责 persona、capability boundary、policy boundary、`model_binding`
- 运行时默认继承 role binding，再按对象显式配置覆盖

### 1.2 用户目标与平台目标

#### 高频任务

- 新建一个诊断或执行角色
- 设置角色的 system prompt / persona tags
- 限定角色允许的 skills 或 connector capabilities
- 配置最大风险等级与审批边界
- 给角色绑定默认 Provider / Model

#### 平台目标

- 把 AI 运行时身份与人类 IAM 彻底分开
- 让风险边界、能力边界和模型绑定在一个主对象里闭环
- 保证 session / execution / automation 的角色继承可追踪、可解释、可回退

### 1.3 状态模型

#### 生命周期状态

- `enabled`
- `disabled`
- `builtin`
- `custom`

#### 绑定状态

- `inherits platform default`
- `bound`
- `fallback only`
- `invalid binding`

#### 运行时决策状态

- `direct_execute`
- `require_approval`
- `suggest_only`
- `deny`

#### 展示优先级

1. 角色是否启用
2. 风险边界是否可接受
3. `model_binding` 是否有效
4. capability restrictions 是否命中

### 1.4 核心字段与层级

#### L1 默认字段

- `role_id`
- `display_name`
- `description`
- `status`
- `profile.system_prompt`
- `capability_binding.mode`
- `policy_binding.max_risk_level`
- `policy_binding.max_action`
- `model_binding`

#### L2 条件字段

- `persona_tags`
- `allowed_skills`
- `allowed_skill_tags`
- `allowed_connector_capabilities`
- `denied_connector_capabilities`
- `require_approval_for`
- `hard_deny`

#### L3 高级字段

- fallback model binding
- org / tenant scope
- binding template
- runtime matching tags

#### L4 系统隐藏字段

- internal version / revision metadata
- provider secret refs
- runtime resolver diagnostics

#### L5 运行时诊断字段

- 本次 session / execution / automation 实际解析到的 role
- role binding 命中顺序
- 被策略拒绝的 capability 明细

### 1.5 关键规则与约束

#### 当前实现事实

当前 API、前端与运行时已经统一使用结构化 `model_binding`：

- `model_binding.primary.provider_id`
- `model_binding.primary.model`
- `model_binding.fallback.provider_id`
- `model_binding.fallback.model`
- `model_binding.inherit_platform_default`

#### 关键约束

- 产品文案统一使用 `model_binding`
- API 不再接受 `provider_preference`
- 不再把 provider 绑定语义塞回 Provider 对象
- `Agent Role` 是 AI 角色边界主对象，不应再被讲成 IAM 附属配置

#### 推荐演进方向

- 在 session / execution / automation 详情中补充“当前对象实际命中的 Agent Role”
- 增加角色使用关系视图，展示最近有哪些 sessions / automations 正在消费该角色

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 新建一个诊断或执行角色
- 调整角色的人格、技能和动作边界
- 配置风险等级与审批要求
- 给角色绑定默认模型
- 判断这个角色是否太宽、太严或绑定失效

#### 任务映射

| 用户任务 | 主入口 | 不应作为主入口 |
|---------|--------|----------------|
| 给角色选模型 | `/identity/agent-roles` | `/providers` |
| 设定 AI 行为边界 | `/identity/agent-roles` | `/identity/roles` |
| 调整平台默认模型 | Governance | Agent Role 详情首屏 |
| 调试 provider 凭据 | `/providers` 或 `Ops` | Agent Role 页面 |

#### 首屏必须回答的 3 个问题

1. 这个角色当前是启用还是禁用，适合做什么
2. 它能执行什么、不能执行什么、风险边界在哪
3. 它默认会用哪个模型绑定，是否依赖平台默认回退

### 2.2 入口与页面归属

#### `/identity/agent-roles`

作为日常主入口，负责：

- 管理角色目录
- 编辑运行时边界
- 查看内置角色与自定义角色

当前真实 route 仍挂在 `identity` 树下；但产品归属应视为 `Operate / AI / Agent Roles`，不再把它解释成人类 IAM 子对象。

#### Providers

Providers 只负责：

- 连接状态
- model discovery
- 健康与验证

不负责角色选择与人格边界。

#### Sessions / Executions / Automations

这些页面消费 `agent_role_id`，但不承担角色主配置入口：

- Session 继承或解析角色
- Execution 展示角色边界下的动作结果
- Automation 指定 `run as` 角色

#### Governance / Ops

- 治理页负责共享 binding 模板、默认风险模板
- `Ops` 仅处理 raw import/export、修复与诊断

### 2.3 页面结构

#### 列表页

必须区分：

- Built-in Roles
- Custom Roles

每项至少展示：

- 角色名
- 状态
- mode
- max risk
- 当前 `model_binding` 摘要

#### 详情页

首屏优先顺序：

1. Profile
2. Capability Binding
3. Policy Binding
4. Model Binding
5. Metadata

#### 关联视图

后续应补充：

- 哪些 automations 正在使用该角色
- 哪些 sessions / executions 最近继承了该角色

### 2.4 CTA 与操作层级

#### 主动作

- `创建 Agent Role`
- `保存`
- `启用 / 停用`

#### 次级动作

- `复制`
- `查看使用关系`
- `查看运行时解析`

#### 高风险动作

- `删除` 仅对 custom role 开放
- raw import/export 与 resolver 诊断下沉到 `Ops`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在高级设置或扩展区
- L4/L5 不应默认占据列表页和详情首屏
- runtime resolver diagnostics 应进入使用关系、诊断或高级调试区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Agent Roles`
- 对象名：`Agent Role`
- 模型绑定字段：`Model Binding`

#### 页面叙事

- 页面讲“AI 运行时身份”
- 不讲“人类 RBAC”
- 不把 Agent Roles 讲成 Provider 设置页或 Prompt 文件夹

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Agent Roles`
- 副标题应表达：为诊断、执行与自动化配置 AI 运行时身份、边界和模型绑定

#### 详情页

- 标题默认使用角色显示名
- 副标题应聚焦角色用途、风险边界和模型绑定状态

#### 创建 / 编辑页

- 创建标题：`创建 Agent Role`
- 编辑标题：`编辑 Agent Role`
- 文案应围绕 persona、capability、policy、model binding，而不是 provider 凭据

### 3.3 CTA 文案

主路径默认使用：

- `创建 Agent Role`
- `保存`
- `启用`
- `停用`

次级路径默认使用：

- `复制`
- `查看使用关系`
- `查看运行时解析`

高风险区允许：

- `删除`
- `导出配置`
- `查看调试信息`

### 3.4 状态文案

#### 没有自定义角色

- 结论：`当前只有内置角色`
- 细节：可直接复用 builtin，也可创建 custom role
- 动作：`创建 Agent Role`

#### 角色未绑定模型

- 结论：`当前角色没有可用模型绑定`
- 细节：运行时将回退到平台默认或解析失败
- 动作：`编辑 Model Binding`

#### 角色被策略锁死

- 结论：`当前角色几乎无法执行动作`
- 细节：`max_action`、`hard_deny` 或 capability 白名单过严
- 动作：`调整策略边界`

#### 运行时解析失败

- 结论：`当前对象无法解析到可用 Agent Role`
- 细节：显式 role 不存在、已禁用或 binding 无效
- 动作：`修复角色配置`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- `provider_preference`
- provider secret refs
- runtime resolver internal diagnostics
- 人类 IAM 术语作为 Agent Role 默认叙事

这些内容可留在高级调试、治理或 Ops，不应主导 Agent Roles 默认叙事。

---

## 4. 验收清单

### 4.1 页面级验收

- `/identity/agent-roles` 已被清晰表达为 AI 运行时身份主入口
- 列表页清晰区分 builtin 与 custom
- 详情页首屏按 Profile、Capability、Policy、Model、Metadata 组织
- 页面默认不再把 Agent Role 讲成 Provider 子配置或人类 IAM 角色

### 4.2 交互级验收

- 用户能在一个主页面里完成 persona、capability、policy、model binding 编辑
- 用户不需要跳到 Providers 页面给角色选模型
- 删除动作只对 custom role 暴露
- 使用关系、调试信息和高级诊断已下沉，不打断主编辑流

### 4.3 展示级验收

- 列表摘要至少包含角色名、状态、mode、max risk、model binding 摘要
- 空态、未绑定模型、策略锁死、解析失败几类状态都有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖列表分组、详情主区块和关键状态文案
- 需要交互测试覆盖创建、自定义角色编辑、启用停用和 custom 删除
- 需要浏览器或截图验收确认页面默认叙事已经摆脱 Provider / IAM 混淆

### 4.5 剩余限制说明

- 角色使用关系视图可以作为下一阶段增强项
- resolver 诊断和 raw import/export 继续放在 `Ops`
- 若后端尚未提供完整使用关系查询，前端不应伪装成已有完整反查能力
