# TARS — Setup Workbench 规范

> **状态**: 设计基线
> **适用范围**: `/setup` first-run wizard 与 runtime checks
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-identity-access.md](./20-component-identity-access.md)、[20-component-providers-and-agent-role-binding.md](./20-component-providers-and-agent-role-binding.md)、[20-component-channels-and-web-chat.md](./20-component-channels-and-web-chat.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Setup Workbench 是什么

`Setup Workbench` 是 TARS 的 **初始化与运行体检工作台**。

它不是某个一级配置对象，而是围绕“平台是否已经能跑起来”建立的工作台对象域。

#### Setup 不是什么

- 不是所有对象的超级配置页
- 不是长期替代 Providers / Channels / Identity 的日常入口
- 不是 Telegram-only 或某单一渠道导向的启动页

#### 双模式

- `first-run wizard`
- `runtime checks`

真实页面 `web/src/pages/setup/SetupSmokeView.tsx` 已清楚体现这两个模式。

### 1.2 用户目标与关键场景

#### first-run 阶段

- 创建首个管理员
- 建立最小登录方式
- 配置主模型
- 配置第一方入口与默认跟进

#### runtime checks 阶段

- 触发 smoke alert
- 查看 metrics / execution path 是否健康
- 确认 diagnosis、approval、execution、knowledge feature 是否正常

#### 关键场景

- 新部署后快速完成最小可运行路径
- 升级或变更后确认关键运行链路仍然健康
- 在平台已上线后，用 setup 检查“还活着没有”，而不是回到对象页挨个点

### 1.3 状态模型

#### 初始化状态

- `admin_configured`
- `auth_configured`
- `provider_ready`
- `channel_ready`
- `initialized`

#### 页面模式状态

- `wizard`
- `runtime`

#### runtime check 状态

- `healthy`
- `degraded`
- `missing_secret`
- `unconfigured`
- `pending_smoke`

#### 展示优先级

1. 平台是否已完成最小初始化
2. 当前处于 wizard 还是 runtime checks
3. 关键运行链是否健康

### 1.4 核心字段与层级

#### L1 默认字段

- admin username / password
- auth type
- provider id / model / base_url
- default conversation entry
- default first-party delivery target
- preferred follow-up mode
- runtime check host / service / severity

#### L2 条件字段

- email / display name
- secret ref
- provider connectivity note
- latest smoke result

#### L3 高级字段

- rollout mode
- connector counts
- feature toggles
- fallback runtime path

#### L4 系统隐藏字段

- raw secret values
- internal initialization hints
- runtime probe payload

#### L5 诊断字段

- component runtime last error
- missing secret detail
- last smoke session and follow-up outcome

### 1.5 关键规则与约束

- `/setup` 只负责 first-run onboarding 与 runtime smoke / checks
- 初始化完成后，日常配置应回到 `/identity`、`/providers`、`/channels`、`/connectors`
- Dashboard 是长期 runtime command center；Setup 只负责“平台是否已经起飞”和“关键链路是否还活着”
- `wizard/channel` 步骤应理解为选择第一方入口与默认跟进，不再要求 flat channel target

#### 当前实现事实

- 启动后先根据 initialization mode 决定显示 wizard 还是 runtime
- 完成 setup 后会引导登录或直接进入 runtime checks
- runtime checks 会聚合 connectors、model、第一方 entry / delivery、feature toggles 与 latest smoke

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 完成首次启动向导
- 检查主模型和入口是否连通
- 触发一次 smoke / runtime check
- 查看最近一次 smoke 的结果并继续追查

#### 首屏必须回答的 3 个问题

1. 平台是不是已经完成最小启动
2. 当前缺的是哪一步，还是只是运行链健康待验证
3. 我下一步该继续 wizard、触发 smoke，还是跳去某个对象页修复

### 2.2 入口与页面归属

#### `/setup`

只负责：

- first-run onboarding
- runtime smoke / checks

#### 对象主入口的切换

初始化完成后，日常配置应回到：

- `/identity`
- `/providers`
- `/channels`
- `/connectors`

#### 与 Dashboard 的关系

Dashboard 是长期 runtime command center；Setup 只负责“平台是否已经起飞”和“关键链路是否还活着”。

### 2.3 页面结构

#### first-run wizard

固定四步：

1. 首个管理员
2. 最小登录方式
3. 主模型
4. 第一方入口与默认跟进

#### runtime checks

推荐结构：

1. metrics / execution path
2. 关键组件状态
3. manual runtime check 表单
4. latest smoke card
5. control plane / runtime features 摘要

### 2.4 CTA 与操作层级

#### 主动作

- `保存并继续`
- `检查连通性`
- `完成初始化`
- `触发运行检查`
- `刷新`

#### 次级动作

- `返回上一步`
- `查看最近 Smoke`
- `打开相关 Session`

#### 高级动作

- `前往 Ops 查看 Secret`
- `查看运行诊断`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在高级设置或运行摘要区
- L4/L5 不应默认占据 wizard 主流程或 runtime checks 首屏
- raw secret、probe payload、内部 hint 进入高级区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Setup`
- 页面名：`Setup Workbench`
- runtime checks 模式：`Runtime Checks`

#### 页面叙事

- 页面讲“首次起飞与运行体检”
- 不讲“超级配置中心”
- 不把 Setup 讲成某个单一 provider / channel 的安装向导

### 3.2 页面标题与副标题

#### wizard 模式

- 标题：`Setup`
- 副标题应表达：完成平台最小启动路径

#### runtime 模式

- 标题：`Runtime Checks`
- 副标题应表达：确认关键链路仍然健康并查看最近 smoke 结果

### 3.3 CTA 文案

主路径默认使用：

- `保存并继续`
- `检查连通性`
- `完成初始化`
- `触发运行检查`
- `刷新`

次级路径默认使用：

- `返回上一步`
- `查看最近 Smoke`
- `打开相关 Session`

高级区允许：

- `前往 Ops 查看 Secret`
- `查看运行诊断`

### 3.4 状态文案

#### 尚未初始化

- 结论：`平台还没完成最小启动路径`
- 细节：显示当前缺失的 step
- 动作：`继续下一步`

#### provider connectivity 失败

- 结论：`主模型当前不可达`
- 细节：展示最近 check 结论
- 动作：`修改 Provider 配置`

#### runtime path 缺密钥

- 结论：`执行链缺少必要 Secret`
- 细节：例如 JumpServer connector secret 未设置
- 动作：`前往 Ops 查看 Secret`

#### smoke 未通过

- 结论：`平台已初始化，但运行链路还未验证通过`
- 细节：展示 latest smoke session 与失败点
- 动作：`打开相关 Session`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- raw secret values
- internal initialization hints
- runtime probe payload
- 把 Setup 讲成“日常对象配置页”

这些内容可留在高级区，不应主导 Setup 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/setup` 已清晰表达为首次起飞与运行体检工作台
- wizard 与 runtime checks 两种模式清晰分离
- 初始化完成后，页面明确 handoff 到对象主入口

### 4.2 交互级验收

- 用户能顺畅完成四步 wizard
- 用户能在 runtime 模式触发 smoke 并查看最近结果
- 缺 secret、模型不可达、smoke 失败时，能被引导到正确修复入口

### 4.3 展示级验收

- wizard 默认只展示最小启动所需字段
- runtime checks 默认先给健康结论、关键状态和最近 smoke
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖 wizard 步骤、runtime checks 和关键错误态
- 需要浏览器或截图验收确认 Setup 默认叙事已经从“杂糅配置页”收口为“起飞与体检工作台”
- 若后端尚未提供更丰富的 smoke 细节，前端不应伪装成已有完整根因分析

### 4.5 剩余限制说明

- 日常对象配置继续回归各自对象页
- 更底层的 secret、raw config 和 repair 继续留在 `Ops`
