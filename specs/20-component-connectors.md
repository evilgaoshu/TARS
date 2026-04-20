# TARS - Connectors 规范

> **状态**: 设计基线
> **适用范围**: 外部系统接入对象、模板化创建、验证、生命周期、运行时能力调用
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-skills.md](./20-component-skills.md)、[20-component-extensions.md](./20-component-extensions.md)、[20-component-observability.md](./20-component-observability.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Connector 是什么

`Connector` 是 TARS 的 **外部系统接入对象**。

它回答：

- 接的是哪个系统
- 用什么协议接
- 当前是否可用
- 暴露哪些可验证能力
- 生命周期现在走到哪一步

#### Connector 不是什么

- 不是 raw manifest 浏览器
- 不是把 capability runtime 细节直接平铺给用户的 debug 页面
- 不是 secret ref 清单
- 不是 `Skill` 或 `Extension` 的替身

#### 当前真实心智

当前页面和 API 表明 Connectors 已经是 **对象 registry + 运行验证详情页** 的混合体：

- 列表页看 inventory 与健康
- 详情页做 health / metrics query / execution / capability invoke
- 仍保留 export / upgrade / rollback 等高级动作

### 1.2 平台目标与近期重点对象

#### 平台目标

Connectors 的平台目标是把“外部系统接入”从底层 manifest 配置提升为可验证、可治理、可运行的正式对象。

#### 近期一等对象

- `SSH Connector` — 协议：`ssh_native`，类型：`execution`，capability：`command.execute` / `host.verify`
- `VictoriaMetrics API` — 协议：`victoriametrics_http`，类型：`metrics`，capability：`query.instant` / `query.range`
- `VictoriaLogs API` — 协议：`victorialogs_http`，类型：`logs`，capability：`logs.query` / `victorialogs.query`
  - Demo 环境：https://play-vmlogs.victoriametrics.com（无需认证）
  - 私有部署：使用 `secret_ref` 配置 `bearer_token`，不允许明文存储

#### 次级对象

- `Prometheus API` — 协议：`prometheus_http`（disabled by default）
- `JumpServer API` — 协议：`jumpserver_api`（managed execution bastion）
- `MCP Connector` — 协议：`mcp_stdio`

#### 一等对象的含义

被列为一等对象，意味着它们需要优先获得：

- 独立模板与对象语义（connector-samples.ts 前三位）
- 默认搜索入口与推荐卡片（ConnectorsList 展示前 3 个模板）
- 专门的连接/验证流程（health probe + capability invoke）
- 更靠前的文档、文案与控制面位置
- 独立的 runtime 实现（不共享 generic observability/execution 适配器）

次级对象可以继续兼容、导入和保留扩展空间，但近期不抢默认入口和设计资源。

#### 模板与接入方式

产品上必须先理解为对象，而不是 manifest；创建心智应从模板开始：

- `SSH Connector`
- `API Connector`
- `MCP Connector`
- 后续 `skill_source` / 其他 source 型 connector

### 1.3 状态模型

#### 生命周期状态

- `enabled`
- `disabled`
- `installing`
- `upgraded`
- `rolled_back`

#### 健康状态

- `healthy`
- `degraded`
- `failed`
- `unknown`
- `unverified`

#### 配置状态

- `configured`
- `missing_credentials`
- `missing_required_fields`
- `incompatible`

#### 运行时状态

- `real`
- `stub`
- `degraded`

#### 展示优先级

1. enabled
2. health
3. config completeness
4. compatibility
5. recent runtime result

### 1.4 核心字段与层级

#### L1 默认字段

- `name`
- `type`
- `protocol`
- `enabled`
- `health`
- `credential status`
- `last check`
- `summary`

#### L2 条件字段

- `base_url`
- `auth mode`
- `target org / tenant`
- `connection form` 的必要字段
- `compatibility result`

#### L3 高级字段

- TLS / timeout
- compatibility override
- lifecycle revision
- available version
- capability catalog

#### L4 系统隐藏字段

- raw secret ref
- manifest version
- marketplace metadata
- raw compatibility arrays

#### L5 运行诊断字段

- raw probe response
- capability invoke runtime
- health history
- upgrade / rollback history

### 1.5 关键规则与约束

- 默认先讲“系统接好了没”，再讲 manifest
- secret 默认展示为 `已配置 / 缺失 / 无效`
- capability 原文与 raw manifest 下沉高级区
- `/connectors` 必须保持为对象主入口，而不是退回 `/ops`
- 模板与 capability catalog 可以继续兼容，但默认区不应暴露 raw 元字段

### 1.6 API 映射与当前实现事实

#### 当前 API

- `GET /api/v1/connectors`
- `GET /api/v1/connectors/{id}`
- `POST /api/v1/connectors`
- `PUT /api/v1/connectors/{id}`
- `POST /api/v1/connectors/{id}/enable`
- `POST /api/v1/connectors/{id}/disable`
- `GET /api/v1/connectors/{id}/export`
- `POST /api/v1/connectors/{id}/metrics/query`
- `POST /api/v1/connectors/{id}/execution/execute`
- `POST /api/v1/connectors/{id}/health`
- `POST /api/v1/connectors/{id}/upgrade`
- `POST /api/v1/connectors/{id}/rollback`
- `POST /api/v1/connectors/{id}/capabilities/invoke`
- `GET /api/v1/connectors/templates`
- `GET /api/v1/platform/discovery`
- `GET /api/v1/config/connectors`
- `PUT /api/v1/config/connectors`
- `POST /api/v1/config/connectors/import`

#### 当前实现事实

- `metrics` 与 `execution` 已有真实 runtime 主链
- `logs` (VictoriaLogs) 已有独立 runtime：`victorialogs_http` 协议，支持 `/health` probe 和 `/select/logsql/query` 查询
- SSH 已有 `ssh_native` 协议模板，runtime adapter 待接入（secret store 集成后）
- discovery 已公开暴露 connector kinds 与 tool-plan capabilities
- lifecycle sidecar 已记录 health / revisions / available_version
- VictoriaLogs play demo：https://play-vmlogs.victoriametrics.com（可直接用 tool-plan 调用）

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 新建一个 SSH / VictoriaMetrics / VictoriaLogs connector
- 测试连接是否成功
- 查看当前健康状态与最近检查结果
- 启用或禁用 connector
- 在详情页直接做一次 metrics query 或 execution smoke

#### 任务映射

| 用户任务 | 主入口 | 不应作为主入口 |
|---------|--------|----------------|
| 接入监控系统 | `/connectors` | `/ops` |
| 校验执行链可用性 | `/connectors/{id}` | raw YAML |
| 导入一个官方样例 | `/connectors` 或 `Extensions` | 手写 manifest |
| 查看 capability 调用结果 | Connector 详情 | Skills 页 |

#### 首屏必须回答的 3 个问题

1. 这个 connector 现在能不能用
2. 它接的是哪个系统、用什么接入方式
3. 下一步更可能是测试、编辑、启用还是修复

### 2.2 入口与页面归属

#### `/connectors`

作为正式主入口，负责：

- connector inventory
- 模板化创建
- 启停与验证
- 详情与生命周期查看

#### Connector 详情页

负责：

- 基础连接信息
- 健康与验证
- 生命周期与兼容状态
- 高级动作（export / upgrade / rollback / capability invoke）

#### `Extensions`

负责候选 bundle 的导入与评审，不替代 connector 主对象页。

#### `Ops`

仅保留：

- raw connector config
- sample import
- secret inventory
- emergency repair

### 2.3 创建流程

推荐流程：

1. 搜索系统或选择模板
2. 选择接入方式（API / MCP）
3. 填最少连接字段
4. 测试连接
5. 保存并决定是否启用

创建流默认应是 object-first，而不是 manifest-first。

### 2.4 列表页

列表页必须展示：

- 名称
- 类型 / 接入方式
- 启用状态
- 健康状态
- 凭据状态
- 最近检查
- 一句话摘要

列表页默认要先回答“这套外部系统接得怎么样”，而不是先展开高级配置。

### 2.5 详情页

详情页首屏优先回答：

1. 这个 connector 现在是否可用
2. 它能做什么类型的能力调用
3. 下一步最可能是测试、编辑还是修复

详情默认区以连接状态、健康、验证和下一步建议为主；raw manifest、capability 原文、runtime 细节下沉高级区。

### 2.6 CTA 与操作层级

#### 主动作

- `测试连接`
- `编辑连接`
- `启用 / 禁用`

#### 次级 / 高级动作

- `metrics query`
- `execution smoke`
- `capability invoke`
- `导出`
- `升级`
- `回滚`
- `查看 raw manifest`

#### 操作层级规则

- 主动作前置，保持对象运维闭环
- 低频和高风险动作下沉到详情页高级区
- 调试型动作不能挤占默认主路径

### 2.7 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放入高级设置或高级详情
- L4/L5 不应默认出现在创建流和详情首屏
- 凭据相关信息默认展示状态，不直接暴露 raw secret ref

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Connectors`
- 对象名：`Connector`
- 创建动作：`新建连接器`
- 详情动作：`编辑连接`

#### 页面叙事

- 列表页讲“外部系统接入对象”
- 详情页讲“连接状态、验证能力与下一步动作”
- 不把 Connectors 讲成 manifest 或 capability debug 工具

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Connectors`
- 副标题应表达：管理外部系统接入，回答“接的是哪个系统、用什么协议、当前是否可用”

#### 详情页

- 标题默认使用 connector 显示名称
- 副标题应聚焦当前可用性、能力类型与建议下一步

#### 创建 / 编辑页

- 创建标题：`新建连接器`
- 编辑标题：`编辑连接`
- 标题和说明应围绕“接入对象”而不是“manifest 字段”

### 3.3 CTA 文案

默认主路径使用以下文案：

- `添加 Connector` / `新建连接器`
- `测试连接`
- `编辑连接`
- `启用`
- `禁用`
- `重新测试`

高级区允许：

- `升级`
- `回滚`
- `导出`
- `查看 raw manifest`
- `执行 smoke`

### 3.4 状态文案

#### 没有 connector

- 结论：`还没有接入外部系统`
- 细节：先接一个 `VictoriaMetrics`、`VictoriaLogs` 或 `SSH` connector 建立最小诊断闭环
- 动作：`添加 Connector`

#### connector 不健康

- 结论：`当前连接不可用或已降级`
- 细节：展示最近 probe、凭据缺失或版本不兼容
- 动作：`编辑连接`、`重新测试`

#### runtime 不支持

- 结论：`当前对象已注册，但运行时还不能执行该能力`
- 细节：manifest 有 capability，runtime 没有对应 adapter
- 动作：`查看 capability 兼容性`

#### 升级后待探活

- 结论：`连接已变更，等待新的健康检查`
- 细节：upgrade / rollback 后 health 进入 `unknown`
- 动作：`重新 health check`

### 3.5 术语黑名单

以下词汇不应默认出现在主路径：

- raw manifest
- raw secret ref
- marketplace metadata
- compatibility arrays
- runtime adapter internals

这些内容可以出现在高级区、导出区或诊断区，但不应主导默认页面叙事。

---

## 4. 验收清单

### 4.1 页面级验收

- `/connectors` 仍是 connector 对象主入口，而不是回退到 `/ops`
- 列表页默认先回答“接入对象现在怎么样”，而不是先展示底层字段
- 详情首屏先回答可用性、能力类型和建议下一步
- 默认区不再把 Connectors 讲成 manifest 编辑器

### 4.2 交互级验收

- 创建流是 object-first，不是 raw manifest-first
- 创建流按“搜索/模板 -> 最少字段 -> 测试 -> 保存”组织
- 启用 / 禁用 / 测试连接保持主路径前置
- raw manifest、capability 原文、runtime 细节已下沉高级区

### 4.3 展示级验收

- 列表页至少展示名称、类型 / 接入方式、启用状态、健康状态、凭据状态、最近检查、一句话摘要
- secret 默认展示为 `已配置 / 缺失 / 无效`
- 空态、错误态、degraded 态文案与主动作清楚

### 4.4 测试与验证要求

- 需要页面级测试覆盖创建流、列表主字段与详情主动作
- 需要浏览器或截图验收确认默认区没有错误上浮的实现细节
- 若后端暂不支持某些高级 runtime 能力，前端不应伪装成已支持

### 4.5 剩余限制说明

- API 和 runtime 可以继续兼容现有模板、导出与高级能力路径
- 前端在默认区应坚持 object-first 心智，即使底层仍保留 manifest / lifecycle 兼容结构
