# TARS — Identity / Access 规范

> **状态**: 设计基线
> **适用范围**: `/identity` 概览、Auth Providers、Users、Groups、Roles、People 关联视图、登录会话与访问控制
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-agent-roles.md](./20-component-agent-roles.md)、[20-component-org.md](./20-component-org.md)、[20-component-audit.md](./20-component-audit.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Identity / Access 是什么

`Identity / Access` 是 TARS 的 **人类访问控制对象域**。

它定义并管理：

- 谁能登录控制面和 API
- 用户属于哪些用户组与 RBAC 角色
- 使用哪种认证来源建立会话
- 当前会话是否有效、可追溯、可撤销

它在产品上不是单个 registry，而是以 `/identity` 为入口的一组相关对象域：

- `Auth Provider`
- `User`
- `Group`
- `Role`
- `Auth Session`

#### Identity / Access 不是什么

- 不是 `Ops Token` 配置页的别名
- 不是 `Org / Tenant / Workspace` 组织治理页
- 不是 `Agent Role` 的 AI 运行时身份模型
- 不是把 `People`、组织结构、轮值画像与 IAM 强行混成一个对象

#### People 与 Org 的边界

当前真实路由仍有 `/identity/people`，因此 `/identity` 概览会统计 `people` 数量；但规范边界上：

- `People` 更接近组织人物层与业务人物层
- `Identity / Access` 只把 `People` 当作访问治理的关联对象，不把它当 IAM 核心对象
- 组织、租户、工作空间与策略主配置归 [20-component-org.md](./20-component-org.md)

#### Agent Role 与 IAM 的边界

- `RBAC Role` 作用于人类用户访问控制
- `Agent Role` 作用于 AI 的运行时能力边界
- 两者可关联，但不互相替代

### 1.2 用户目标与关键场景

#### 高频任务

- 查看当前平台是否已建立可用登录方式
- 创建或禁用用户
- 维护用户组与角色绑定
- 启用或停用某个 Auth Provider
- 查看当前活跃会话与访问范围

#### 当前页面心智

真实页面 `web/src/pages/identity/IdentityOverview.tsx` 表明 `/identity` 当前不是单纯目录页，而是 **IAM 概览工作台**：

- 汇总 providers / users / groups / roles / people / sessions
- 提供快速跳转到分项对象页
- 展示当前用户对各对象域的可见范围

### 1.3 状态模型

#### Auth Provider 状态

- `enabled`
- `disabled`
- `configured`
- `misconfigured`
- `degraded`

#### User / Group / Role 状态

- `active`
- `disabled`
- `invited`
- `builtin`

#### Session 状态

- `active`
- `expired`
- `revoked`
- `pending_challenge`
- `pending_mfa`

#### Identity 概览展示优先级

1. 登录是否可用
2. 目录对象是否可管理
3. 当前会话是否正常
4. 当前用户有哪些访问范围

### 1.4 核心字段与层级

#### Identity 概览字段

##### L1 默认摘要

- providers 数量
- users / groups / roles 数量
- people 关联数量
- active sessions 数量
- auth config 是否已配置 / 已加载

##### L2 条件显示

- provider 启停按钮
- 当前用户对各对象域的 permission scope
- 最近会话过期时间、provider 来源

##### L3 高级字段

- config path
- auth mode 兼容状态
- break-glass 保留信息

##### L4 系统隐藏字段

- token 明文
- provider secret ref 明文
- 原始 claim / callback payload

##### L5 运行诊断字段

- challenge / MFA 中间态
- provider callback 错误
- raw session inventory payload

#### 子对象字段原则

- `Auth Provider` 首屏展示：类型、启用状态、登录方式、连接摘要
- `User` 首屏展示：用户名、显示名、邮箱、状态、组 / 角色
- `Group` 首屏展示：名称、成员数、状态
- `Role` 首屏展示：权限集合与绑定关系

### 1.5 关键规则与约束

- raw OIDC metadata / JWKS / callback payload 下沉到诊断层
- `operator_reason` 不要求日常 UI 首屏输入
- `People` 的画像、排班、业务归属不进入 IAM 默认表单
- `ops-token` 仍保留 break-glass 语义，但不再等价于 IAM 产品主路径

#### 当前实现事实

- `/identity` 概览直接并行拉取 providers、config、users、groups、roles、people、sessions
- `local_password`、challenge、MFA 基础链已落地

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 快速判断登录能力是否可用
- 找到用户、组、角色、认证来源的入口
- 查看当前会话与权限可见范围
- 判断某个访问问题应该去 `/identity`、`/org` 还是 `Ops`

#### 任务映射

| 用户任务 | 主入口 | 不应作为主入口 |
|---------|--------|----------------|
| 开通本地密码或 OIDC 登录 | `/identity/providers` | `/ops` |
| 查看当前可见权限范围 | `/identity` | raw config |
| 管理用户、组、角色 | `/identity/users`、`/identity/groups`、`/identity/roles` | `/org` |
| 查看当前登录 session | `/identity` 概览 | `/sessions` |
| 调整组织/租户策略 | `/org` | `/identity` |

#### 首屏必须回答的 3 个问题

1. 当前有没有可用登录方式
2. 我能管理哪些 IAM 对象域
3. 当前会话和权限范围是否正常

### 2.2 入口与页面归属

#### `/identity`

作为 IAM 概览工作台，负责：

- 汇总认证来源、用户目录、角色目录、活跃会话
- 告诉操作者当前有哪些对象域可见
- 提供去各子页面的入口

#### `/identity/providers`

负责 Auth Provider 的日常配置与启停：

- `local_token`
- `local_password`
- `oidc`
- `oauth2`
- 后续 `ldap`

#### `/identity/users` / `/identity/groups` / `/identity/roles`

分别负责：

- 用户生命周期
- 用户组与成员关系
- RBAC 角色定义与绑定

#### `/identity/agent-roles`

虽然路径位于 `identity` 组下，但它属于 AI 运行时身份域；规范由 [20-component-agent-roles.md](./20-component-agent-roles.md) 独立描述。

#### `/org`

组织、租户、工作空间与组织策略不归 IAM 主路径，而是顶层组织治理入口。

#### `Ops`

仅保留：

- raw auth config
- break-glass 修复
- secret inventory
- 导入导出与紧急诊断

### 2.3 页面结构

#### Identity 概览页

推荐结构与当前实现保持一致：

1. 顶部 summary stats
2. Auth Providers 摘要卡
3. Auth Config 状态卡
4. Active Sessions 摘要
5. Scope / Visibility 摘要

#### Auth Providers 页

应优先回答：

1. 当前有哪些登录来源
2. 哪些已启用
3. 默认登录路径是什么
4. 哪些需要补充配置

#### Users / Groups / Roles 页

默认采用 registry / split layout：

- 左侧列表看对象、状态、数量
- 右侧详情看主要字段与绑定
- 主动作是创建、编辑、启用、禁用

#### 会话信息展示

会话不是独立主导航页，但 Identity 概览必须让用户快速看到：

- 当前会话来自哪个 provider
- 创建时间与过期时间
- 是否仍有效

### 2.4 CTA 与操作层级

#### 主动作

- `添加登录方式`
- `创建用户`
- `创建用户组`
- `创建角色`

#### 次级动作

- `启用 / 停用`
- `查看会话`
- `查看权限范围`

#### 高级动作

- `前往 Ops 修复认证`
- `查看登录诊断`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在高级设置或状态摘要区
- L4/L5 不应默认占据概览页和对象详情首屏
- raw claim、callback payload、secret ref 明文进入高级诊断区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Identity & Access`
- 概览页：`Identity`
- 人类角色：`Role`
- AI 运行时角色：`Agent Role`

#### 页面叙事

- 页面讲“人类访问控制”
- 不讲“AI 角色配置”
- 不把 `/identity` 讲成组织治理入口或 break-glass 修复台

### 3.2 页面标题与副标题

#### 概览页

- 标题：`Identity`
- 副标题应表达：管理登录来源、用户目录、角色和当前会话

#### 子页面

- `/identity/providers`：`Auth Providers`
- `/identity/users`：`Users`
- `/identity/groups`：`Groups`
- `/identity/roles`：`Roles`

说明文案应围绕登录、访问控制和当前会话，而不是 raw auth config。

### 3.3 CTA 文案

主路径默认使用：

- `添加登录方式`
- `创建用户`
- `创建用户组`
- `创建角色`

次级路径默认使用：

- `启用`
- `停用`
- `查看会话`
- `查看权限范围`

高级区允许：

- `前往 Ops 修复认证`
- `查看登录诊断`

### 3.4 状态文案

#### Identity 未初始化

- 结论：`平台还没有完成最小登录配置`
- 细节：缺少管理员、本地密码或外部 IdP
- 动作：`前往 Setup`

#### 没有可用 Auth Provider

- 结论：`当前没有可用登录来源`
- 细节：provider 全部禁用或配置缺失
- 动作：`前往 Auth Providers`

#### 当前用户范围受限

- 结论：`你只能查看部分 IAM 对象域`
- 细节：基于 `users.read`、`groups.read`、`roles.read`、`auth.read` 等权限显隐
- 动作：`联系管理员`

#### 登录链异常

- 结论：`认证流程未完成`
- 细节：challenge、MFA 或 callback 失败
- 动作：`重试登录`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- `ops-token` 作为 IAM 主对象叙事
- raw OIDC metadata / callback payload
- 把 `Agent Role` 讲成 IAM 角色
- People 的业务画像字段

这些内容可留在高级诊断区，不应主导 Identity / Access 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/identity` 已清晰表达为 IAM 概览工作台
- 登录来源、用户目录、角色目录和当前会话边界清晰
- `/identity/agent-roles` 与人类 IAM 角色边界清晰

### 4.2 交互级验收

- 用户能从概览页快速进入 providers / users / groups / roles
- 用户能判断登录是否可用、当前范围是否受限
- 日常 IAM 配置不再被引导到 `Ops`

### 4.3 展示级验收

- 概览默认展示 providers、users、groups、roles、people、sessions 摘要
- 未初始化、无可用登录来源、权限受限、登录链异常等状态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖概览摘要、对象页入口和关键状态文案
- 需要浏览器或截图验收确认 Identity 默认叙事已经从“杂糅 auth/config 页”收口为“人类访问治理工作台”
- 若后端尚未提供更细的 people / session 关系细节，前端不应伪装成已有完整人物治理能力

### 4.5 剩余限制说明

- `People` 的长期主归属仍应迁往 `Org / People` 域
- 更底层的 raw auth config、secret inventory 与 break-glass 修复继续保留在 `Ops`
