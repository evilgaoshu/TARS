# 身份与访问平台规范

> **状态**: 设计基线
> **适用范围**: 用户管理、认证、授权、人类访问控制对象与相关审计
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-identity-access.md](./20-component-identity-access.md)、[20-component-org.md](./20-component-org.md)、[20-component-agent-roles.md](./20-component-agent-roles.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### 身份与访问是什么

身份与访问平台负责管理：

- 系统里有哪些人和账号
- 他们如何登录并建立会话
- 他们能看到什么、能做什么

它覆盖 3 类能力：

1. `Users`
2. `Authentication`
3. `Access Control`

#### 身份与访问不是什么

- 不是 `Ops Token` 或 break-glass 配置页
- 不是 `Agent Role` 的 AI 运行时身份模型
- 不是 `Org / People` 的组织治理主入口

#### 与 People 的关系

- `People` 偏向组织 / 业务人物层，主归属在 `Org`
- `Users / Authentication / Access Control` 偏向平台访问控制层

### 1.2 用户目标与关键场景

#### 高频任务

- 建立本地密码或外部单点登录
- 创建用户并分配用户组 / 角色
- 查看当前会话是否有效
- 判断访问失败是认证问题、授权问题还是组织边界问题

#### 关键场景

- 平台完成最小登录路径
- 为平台操作者建立稳定的登录与权限体系
- 把 People、IAM、AI Agent Role 三类对象边界彻底分开

### 1.3 状态模型

#### 认证模式

- `local_token`
- `local_password`
- `oidc / oauth2`
- `ldap`

#### 访问状态

- `active`
- `disabled`
- `revoked`
- `pending_challenge`
- `pending_mfa`

#### 展示优先级

1. 是否存在可用登录方式
2. 用户和角色是否可管理
3. 当前会话是否有效

### 1.4 核心字段与层级

#### L1 默认字段

- 用户名
- 显示名
- 邮箱
- 登录方式
- 用户组 / 角色
- 当前会话状态

#### L2 条件字段

- provider 来源
- challenge / MFA 状态
- permission scope

#### L3 高级字段

- break-glass 信息
- 兼容认证模式

#### L4 系统隐藏字段

- token 明文
- secret 明文
- 原始 callback payload

#### L5 运行诊断字段

- challenge 失败细节
- provider callback 错误
- 原始 session payload

### 1.5 关键规则与约束

#### 权限模型

访问控制采用 4 层模型：

1. `资源 (Resource)`
2. `动作 (Action)`
3. `能力 (Capability)`
4. `风险等级 (Risk)`

#### 关键约束

- 人类 IAM 与 `Agent Role` 必须分离
- `People` 的人物画像与组织归属不应塞回 IAM 默认表单
- 原始认证 payload 和 secret 只进入诊断层，不进入主路径

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 查看当前有没有可用登录方式
- 管理用户、组和角色
- 判断当前会话与访问范围是否正常

#### 首屏必须回答的 3 个问题

1. 平台当前能不能登录
2. 用户、组、角色是否已经建立
3. 当前会话和权限范围是否正常

### 2.2 入口与页面归属

#### `/identity`

作为 IAM 概览入口，负责：

- 登录来源
- 用户、组、角色摘要
- 当前会话

#### `/identity/providers`

负责认证来源的日常配置与启停。

#### `/identity/users` / `/identity/groups` / `/identity/roles`

分别负责用户、用户组与 RBAC 角色管理。

#### `/org`

负责组织、租户、工作空间与 `People` 主治理，不由 IAM 页面接管。

### 2.3 页面结构

推荐结构：

1. IAM 概览摘要
2. Auth Providers 摘要
3. 用户 / 组 / 角色入口
4. 当前会话信息
5. 权限范围摘要

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

- `查看登录诊断`
- `前往 Ops 修复认证`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在高级设置或兼容状态区
- L4/L5 不应默认占据主页面

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Identity & Access`
- 认证来源：`Auth Provider`
- 人类角色：`Role`
- AI 运行时角色：`Agent Role`

#### 页面叙事

- 页面讲“人类访问控制”
- 不讲“AI 角色配置”
- 不把 `People` 讲成 IAM 主对象

### 3.2 页面标题与副标题

#### 概览页

- 标题：`Identity`
- 副标题应表达：管理登录来源、用户目录、角色和当前会话

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

### 3.4 状态文案

#### 尚未初始化

- 结论：`平台还没有完成最小登录配置`
- 动作：`前往 Setup`

#### 没有可用登录来源

- 结论：`当前没有可用登录来源`
- 动作：`前往认证来源`

#### 当前用户范围受限

- 结论：`你只能查看部分 IAM 对象域`
- 动作：`联系管理员`

### 3.5 术语黑名单

以下内容不应默认出现在主路径：

- token 明文
- callback 原始 payload
- 把 `Agent Role` 当成人类 IAM 角色

---

## 4. 验收清单

### 4.1 页面级验收

- `/identity` 已清晰表达为人类 IAM 入口
- `People` 与 `Agent Role` 的边界清晰

### 4.2 交互级验收

- 用户能完成登录来源、用户、组、角色的日常管理
- 日常 IAM 配置不再默认回到 `Ops`

### 4.3 展示级验收

- 登录未初始化、无可用登录来源、范围受限等状态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖登录来源、用户管理和关键状态文案
- 需要截图或浏览器验收确认 IAM 默认叙事已经稳定

### 4.5 剩余限制说明

- 更细的 people / org 关系仍应由 `Org` 域承接
- 更底层认证诊断继续保留在 `Ops`
