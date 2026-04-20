# TARS — Providers 与 Agent Role Binding 规范

> **状态**: 设计基线
> **适用范围**: AI Provider inventory、模型发现、角色模型绑定、平台默认模型治理
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md)、[20-component-agent-roles.md](./20-component-agent-roles.md)、[40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Provider 是什么

`Provider` 是 **可复用的 AI 后端连接对象**。

它默认回答：

- 这是哪类 AI 服务
- 地址和认证是否正确
- 可发现哪些模型
- 当前是否健康可用

#### Provider 不是什么

- 不是 Agent Role 人格定义
- 不是全局 `primary / assist` 路由表
- 不是 Prompt 管理对象
- 不是把平台默认模型直接焊死在对象上的配置片段

#### Agent Role Binding 是什么

`Agent Role Binding` 是 **角色到模型的绑定关系**。

它属于 `Agent Role`，并被运行时 agent / session / automation run 默认继承；只有高级执行路径才允许临时 override。

它默认回答：

- 这个角色用哪个 Provider / Model
- 是否有兜底模型
- 是显式绑定还是继承平台默认

#### Agent Role Binding 不是什么

- 不是 Provider 凭据对象
- 不是平台 raw providers config
- 不是隐藏在 `provider_preference` 下的一段弱语义字段

### 1.2 用户目标与关键场景

#### 高频任务

- 新增一个 AI Provider
- 验证 Provider 能否连接并发现模型
- 给某个 Agent Role 绑定主模型和兜底模型
- 在不改 Provider 连接信息的前提下，切换角色所用模型
- 查看某个 Provider 正被哪些角色使用

#### 当前最大问题

当前最大问题不是“Provider 字段不够多”，而是：

- `Provider` 被混成了连接对象和路由对象
- `primary / assist` 被错误地挂在 Provider 心智上
- Agent Role 的模型选择没有成为真正的主入口
- `provider_preference` 语义过弱，既不像绑定，也不像治理对象
- 缺少清晰的解析 / 回退顺序，难以解释运行时实际用了哪个模型

#### 当前实现收敛事实

- `diagnosis` session 的 planner / finalizer 已经会按 Agent Role 绑定选择 runtime
- `automation` 的 `skill` 运行时已经会真实继承 `Agent Role.model_binding`
- `connector_capability` automation 仍保持非模型直调路径，但会保留角色绑定用于审计和未来扩展

### 1.3 状态模型

#### Provider 状态

##### 生效状态

- `已启用`
- `已暂停`
- `已禁用`

##### 健康状态

- `可连接`
- `不可达`
- `认证失败`
- `降级`
- `未知`

##### 配置状态

- `已配置`
- `缺凭据`
- `待补充`

##### 模型目录状态

- `已发现`
- `发现失败`
- `未发现`

##### 测试状态

- `最近测试通过`
- `最近测试失败`
- `未测试`

#### Agent Role Binding 状态

- `已绑定`
- `继承默认`
- `未绑定`
- `绑定失效`

#### 展示优先级

1. 首先显示 Provider 健康
2. 其次显示绑定是否有效
3. 最后才展示兼容、目录和历史信息

### 1.4 核心字段与层级

#### Provider 字段分层

##### L1 默认必填字段

- `名称`
- `Provider 类型 / Vendor`
- `服务地址`
- `认证方式`
- `凭据`

##### L2 条件显示字段

- `组织 / 项目 / Tenant`
- `手动模型列表`
- `自定义 Header`
- `区域 / Workspace`

##### L3 高级设置字段

- `请求超时`
- `TLS 校验`
- `代理`
- `默认调用参数`
- `兼容模式`

##### L4 系统隐藏字段

- `id`
- `api_key_ref`
- `secret_ref`
- `protocol` raw value
- `vendor / protocol / version` 原始实现字段

##### L5 运行时诊断字段

- `probe raw response`
- `model discovery raw payload`
- `最近 N 次探测明细`

#### Agent Role Binding 字段分层

##### L1 默认必填字段

- `主模型 Provider`
- `主模型 Model`

##### L2 条件显示字段

- `兜底 Provider`
- `兜底 Model`
- `继承平台默认`

##### L3 高级设置字段

- `切换条件`
- `成本 / 延迟偏好`
- `只读标签`

##### L4 系统隐藏字段

- binding `id`
- 历史版本元数据
- 内部路由权重

##### L5 运行时诊断字段

- 绑定失效原因
- 最后一次降级 / 回退记录

### 1.5 关键规则与约束

- `provider_preference` 已重命名为 `model_binding`
- Providers DTO 上的 `primary_model / assist_model` 已删除
- `/api/v1/providers/bindings` 的日常编辑入口已下沉为平台治理，不再是对象主入口
- `api_key_ref` / secret path 默认隐藏

#### 解析与回退顺序

默认解析顺序应是：

1. 若本次 execution 明确指定 override，优先使用 override
2. 否则使用 `Agent Role.model_binding.primary`
3. 若角色设为 `inherit_platform_default` 或未显式绑定，则使用平台默认主模型
4. 当前选择不可用时，优先尝试 `Agent Role.model_binding.fallback`
5. 若角色未配置 fallback，则尝试平台共享 fallback / safety model
6. 若仍不可用，则标记 binding invalid 并 fail closed

#### 平台默认绑定

`/api/v1/providers/bindings` 保留，但定位改为：

- 平台默认绑定
- 平台共享安全 / 辅助模型绑定

不再作为日常角色模型选择主入口。

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 接入一个新的 AI Provider
- 测试连接并发现模型
- 在 Agent Role 页面选择主模型和兜底模型
- 查看某个 Provider 被哪些角色使用
- 判断模型问题应该在 Provider、Agent Role 还是治理页修复

#### 任务边界

| 任务 | 主对象 | 不应作为主对象 |
|------|--------|----------------|
| 改 AI 地址 / token | Provider | Agent Role |
| 给诊断角色选模型 | Agent Role Binding | Provider 列表页 |
| 配平台默认安全辅助模型 | AI Governance | Provider 详情页 |
| 看 Provider 健康 | Provider | `Ops` |

#### 首屏必须回答的 3 个问题

1. 当前有哪些 Provider 可用，健康是否正常
2. 某个角色当前绑定了什么模型，是否仍有效
3. 如果当前模型不可用，我应该改 Provider、改角色绑定，还是改平台默认

### 2.2 入口与页面归属

#### `/providers`

作为日常主入口，负责：

- 新增 / 编辑 Provider
- 查看健康与认证状态
- 模型发现与测试
- 查看被哪些角色使用

当前真实导航暂挂在 delivery 分组，但对象语义应视为 `Operate / AI / Providers`。

#### `/identity/agent-roles`

作为角色主入口，负责：

- 角色画像
- 能力 / 策略边界
- 模型绑定

模型绑定应是角色详情页中的主 section，而不是隐藏在高级字段里。

#### AI Governance

治理页负责：

- 平台默认模型绑定
- 平台共享安全 / 脱敏辅助模型
- 成本、路由、兜底与降级策略

#### `Ops`

仅保留：

- raw providers config
- 批量导入导出
- 配置修复
- 探测诊断与紧急切换

#### 明确不该放的位置

- 不把角色模型绑定放进 `/providers`
- 不把 Provider 地址和凭据编辑留在 `Ops`
- 不把平台默认模型和角色模型混成一个表单

### 2.3 页面结构

#### Provider 创建流程

推荐流程：

1. 选择 Provider 模板
2. 填写最少连接信息
3. 测试连接
4. 发现模型
5. 保存
6. 可选跳转到“绑定给角色”

#### Providers 列表页

必须显示：

- `名称`
- `类型`
- `启用状态`
- `健康状态`
- `模型目录状态`
- `最近检查`
- `被多少角色使用`

#### Provider 详情页

首屏优先顺序：

1. 状态摘要
2. 连接信息
3. 模型发现结果
4. 使用关系
5. 高级信息

主动作：

- `测试连接`
- `发现模型`
- `编辑连接`
- `启用 / 禁用`

#### Agent Role 详情中的模型绑定区

应优先展示：

- 当前主模型
- 当前兜底模型
- 是否继承平台默认
- 绑定有效性

主动作：

- `更换主模型`
- `设置兜底模型`
- `移除显式绑定并继承默认`

### 2.4 CTA 与操作层级

#### 主动作

- `添加 Provider`
- `测试连接`
- `发现模型`
- `绑定模型`

#### 次级动作

- `编辑连接`
- `查看使用关系`
- `设置兜底模型`

#### 高级动作

- `查看 raw config`
- `前往 AI Governance`
- `紧急降级`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在高级设置或治理补充区
- L4/L5 不应默认占据 Provider 列表和角色详情首屏
- raw probe、发现载荷、内部路由权重进入高级诊断区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Providers`
- 对象名：`Provider`
- 角色模型区：`Model Binding`
- 平台级入口：`AI Governance`

#### 页面叙事

- 页面讲“AI 后端连接对象”和“角色模型绑定”
- 不讲 `primary / assist` provider 路由
- 不把平台默认模型和角色模型混成一个对象

### 3.2 页面标题与副标题

#### Providers 列表页

- 标题：`Providers`
- 副标题应表达：管理 AI 连接、健康、模型发现和角色使用关系

#### Provider 详情页

- 标题默认使用 Provider 名称
- 副标题应聚焦连接状态、模型目录和角色使用情况

#### Agent Role 模型绑定区

- 标题：`Model Binding`
- 说明文案应表达：为这个角色选择主模型、兜底模型或继承平台默认

### 3.3 CTA 文案

主路径默认使用：

- `添加 Provider`
- `测试连接`
- `发现模型`
- `绑定模型`

次级路径默认使用：

- `编辑连接`
- `查看使用关系`
- `设置兜底模型`

高级区允许：

- `前往 AI Governance`
- `查看 raw config`
- `紧急降级`

### 3.4 状态文案

#### Providers 空态

- 标题：`还没有 AI Provider`
- 说明：`先接入至少一个可用模型服务，角色才能选择运行模型。`
- 动作：`添加 Provider`

#### Role Binding 空态

- 标题：`这个角色还没有模型绑定`
- 说明：`先为这个角色选择主模型，TARS 才能按角色语义运行。`
- 动作：`绑定模型`

#### Provider 不可达

- 结论：`这个 Provider 当前不可用`
- 细节：展示超时、DNS、TLS 或网络原因
- 动作：`编辑连接`

#### 凭据无效

- 结论：`认证失败`
- 细节：展示 `401` / `403` 或 token 失效
- 动作：`更新凭据`

#### 模型绑定失效

- 结论：`角色绑定的模型当前不可用`
- 细节：展示 Provider 被禁用、Model 不存在或发现失败
- 动作：`更换模型`、`继承平台默认`

#### 平台默认缺失

- 结论：`角色当前没有可继承的默认模型`
- 细节：说明平台默认绑定未配置
- 动作：`前往 AI Governance`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- `provider_preference`
- `primary_model / assist_model`
- `api_key_ref`
- 把 Provider 讲成“模型路由对象”

这些内容可留在高级诊断区，不应主导 Providers 与 Model Binding 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/providers` 已清晰表达为 AI 连接对象主入口
- Agent Role 页面中的 `Model Binding` 已清晰成为角色模型选择主入口
- 平台默认绑定已从对象主路径中下沉到治理页

### 4.2 交互级验收

- 用户能在 Provider 页完成接入、测试、发现模型
- 用户能在 Agent Role 页完成主模型、兜底模型与继承默认的编辑
- 用户不再被引导去 `/providers` 页面给角色选模型

### 4.3 展示级验收

- Providers 列表至少展示名称、类型、启用状态、健康状态、模型目录状态、最近检查、使用关系
- 空态、Provider 不可达、凭据无效、模型绑定失效、平台默认缺失等状态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖 Provider 创建流、测试连接、模型发现和角色绑定区
- 需要浏览器或截图验收确认 Provider、角色绑定、平台默认三层边界清晰
- 若后端尚未提供更完整的使用关系或 live migration 反馈，前端不应伪装成已有完整治理闭环

### 4.5 剩余限制说明

- `/api/v1/providers/bindings` 继续保留为平台默认绑定入口
- live migration、页面交互和运行时回退契约仍需持续验证
- 更底层的批量导入导出、探测诊断与紧急切换继续保留在 `Ops`
