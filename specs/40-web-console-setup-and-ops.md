# TARS — Setup 与 Ops 分流规范

> **状态**: 设计基线
> **适用范围**: `/setup` 与 `/ops` 的职责切分、导航归属与页面边界
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[40-web-console-setup-workbench.md](./40-web-console-setup-workbench.md)、[40-web-console-ops-console.md](./40-web-console-ops-console.md)、[40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Setup 是什么

`Setup` 是 **first-run onboarding + runtime checks 工作台**。

#### Ops 是什么

`Ops` 是 **平台总控与修复控制台**。

#### 这份规范是什么

这份文档不是单独工作台，而是用来明确两者的分层边界；Setup 与 Ops 的 dedicated spec 分别见：

- [40-web-console-setup-workbench.md](./40-web-console-setup-workbench.md)
- [40-web-console-ops-console.md](./40-web-console-ops-console.md)

### 1.2 用户目标与关键场景

#### Setup 负责

- 建立最小管理员、登录方式、主模型、第一方通知入口
- 运行 smoke / runtime checks

#### Ops 负责

- raw config
- import / export
- secrets inventory
- repair / replay / reindex

#### 关键场景

- 用户判断当前是在“起飞阶段 / 体检阶段”，还是在“低频 raw 修复阶段”
- 防止 first-run 表单无限膨胀成通用对象配置页
- 防止日常高频操作持续回流到 `Ops`

### 1.3 状态模型

#### Setup 状态

- `wizard`
- `runtime`
- `initialized`

#### Ops 状态

- `configured`
- `loaded`
- `repair needed`

#### 展示优先级

1. 这是 Setup 还是 Ops 场景
2. 当前是初始配置、运行体检还是平台修复
3. 下一步应该继续本页还是跳去对象页

### 1.4 核心字段与层级

#### Setup 默认字段

- 最小初始化字段
- runtime check 结果

#### Ops 默认字段

- raw YAML / JSON
- diagnostics / repair payload

#### 明确混层边界

- Setup 不承接长期日常对象编辑
- Ops 不承接 first-run 向导字段
- 两者都不替代对象主入口

### 1.5 关键规则与约束

- `/setup` 只服务初始化与运行体检
- `/ops` 只服务平台总控与低频修复
- 不把日常对象编辑长期塞进 `Ops`
- 不把 first-run 表单扩成通用对象配置页

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 判断当前应该进入 Setup 还是 Ops
- 在 Setup 完成起步配置与运行检查
- 在 Ops 完成 raw 修复与平台级诊断

#### 首屏必须回答的 3 个问题

1. 现在是“还没起飞”还是“平台出故障了”
2. 这件事该在 Setup、Ops 还是对象页解决
3. 下一步是继续 wizard / checks，还是执行 repair

### 2.2 入口与页面归属

#### `/setup`

只服务初始化与运行体检。

#### `/ops`

只服务平台总控与低频修复。

#### 不允许的混层

- Setup 不长期承接 Providers / Channels / Identity 的日常配置
- Ops 不默认承接 Providers / Channels / Automations 的高频主任务

### 2.3 页面结构原则

#### Setup

- first-run wizard
- runtime mode checks

#### Ops

- auth
- approval
- secrets
- providers
- connectors
- prompts
- desense
- advanced

### 2.4 CTA 与操作层级

#### Setup 主动作

- `保存并继续`
- `检查连通性`
- `完成初始化`
- `触发运行检查`

#### Ops 主动作

- `保存`
- `导入`
- `诊断`
- `修复`

### 2.5 页面字段裁剪规则

- Setup 只暴露最小启动和健康检查所需信息
- Ops 默认不暴露与当前 tab 无关的日常对象字段
- 对象级编辑和高频配置要 handoff 回对象页

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- `Setup`：起飞与体检
- `Ops`：总控与修复

#### 页面叙事

- Setup 讲“平台是否已起飞”
- Ops 讲“平台是否需要修复”
- 不把两者讲成“后台设置不同 tab”

### 3.2 页面标题与副标题

#### Setup

- 标题：`Setup`
- 副标题应表达：完成最小启动路径或检查关键链路

#### Ops

- 标题：`Ops`
- 副标题应表达：处理 raw config、诊断和低频修复

### 3.3 CTA 文案

Setup 默认使用：

- `保存并继续`
- `完成初始化`
- `触发运行检查`

Ops 默认使用：

- `保存`
- `导入`
- `诊断`
- `修复`

### 3.4 状态文案

#### Setup 未完成

- 结论：`平台还没完成最小启动路径`
- 动作：`继续初始化`

#### Ops 无修复项

- 结论：`当前没有需要处理的修复项`
- 动作：`查看 raw config`

### 3.5 术语黑名单

以下内容不应默认主导页面叙事：

- 把 Setup 讲成“日常配置中心”
- 把 Ops 讲成“对象配置页”
- 把 first-run 表单和 raw repair 混在一个入口里

---

## 4. 验收清单

### 4.1 页面级验收

- `/setup` 与 `/ops` 的职责切分清晰
- Setup 默认讲起飞与体检，Ops 默认讲修复与总控

### 4.2 交互级验收

- 日常对象任务不再默认回流到 Setup / Ops
- first-run 与 raw repair 的操作节奏明显不同

### 4.3 展示级验收

- Setup 与 Ops 的标题、副标题、CTA 和状态文案都能体现分流边界
- 混层字段不再默认上浮

### 4.4 测试与验证要求

- 需要截图或浏览器验收确认两者边界表达清晰
- 新增 setup / ops 相关改动时应同时回看这份分流规范

### 4.5 剩余限制说明

- 未来可以继续细化 handoff 规则
- 但 Setup 与 Ops 的基本职责边界不应再次混回同一入口
