# TARS — Ops Console 规范

> **状态**: 设计基线
> **适用范围**: `/ops` 平台总控、raw config、repair、secrets、advanced actions
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md)、[20-component-observability.md](./20-component-observability.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Ops Console 是什么

`Ops Console` 是 TARS 的 **平台总控与修复控制台**。

它面向低频、跨对象、紧急或 raw 级任务，而不是日常对象配置。

#### Ops 不是什么

- 不是 Providers / Connectors / Auth Providers 的常规编辑页
- 不是 `Governance` 的别名
- 不是 Setup 的延伸步骤

#### 当前真实心智

真实页面 `web/src/pages/ops/OpsActionView.tsx` 已把 `/ops` 收成 tabbed control console：

- auth
- approval
- secrets
- providers
- connectors
- prompts
- desense
- advanced

### 1.2 用户目标与关键场景

#### 高频但低频率的任务

- 编辑 raw auth / approval config
- 管理 secrets inventory
- 导入 connector sample 或修改 raw providers / connectors config
- 调整 reasoning prompts 与 desensitization
- 触发 reindex / raw queue repair / 高级修复

#### 不应继续由 Ops 承担的任务

- 日常改某个 Provider 地址
- 日常编辑 Channel 或 Web Chat
- 日常创建 / 编辑 Automation 主逻辑
- 日常修改通知模板正文

#### 关键场景

- 当对象页不够用时，进行跨对象或 raw 级修复
- 对 Secret、reindex、transport repair 等敏感能力做集中控制
- 将使用者明确引导回对象页，而不是把 `/ops` 变成日常总入口

### 1.3 状态模型

#### 配置状态

- `configured`
- `not_configured`
- `loaded`
- `not_loaded`

#### 操作状态

- `saving`
- `saved`
- `failed`
- `repair_required`

#### secret 状态

- `set`
- `missing`
- `unknown`

#### 展示优先级

1. 当前 tab 是否已加载、已配置
2. 是否存在需要立即修复的问题
3. 是否应 handoff 回对象页或修复台

### 1.4 核心字段与层级

#### L1 默认字段

- configured / loaded 状态
- path
- updated_at
- tab 内主要控制项

#### L2 条件字段

- form 与 YAML 编辑模式
- override / route entries
- secret inventory item set / missing 状态

#### L3 高级字段

- connector sample import
- local LLM assist
- advanced reindex / raw repair payload

#### L4 系统隐藏字段

- secret 明文回显
- 过大 raw payload
- internal repair context

#### L5 诊断字段

- load / save failed detail
- raw replay / import diagnostics
- runtime health related raw context

### 1.5 关键规则与约束

- `/ops` 仅承接 raw config、diagnostics、raw transport / queue repair、secret inventory、advanced actions
- `/ops` 内必须持续提示用户回到对象页或专门修复台
- Governance 负责平台默认与跨对象规则；Ops 负责 raw config 与紧急修复
- secret 为 write-only 设计，避免明文回显
- 已物化的 delivery residue replay 应优先通过 `/outbox` 处理；`/ops` 只保留 raw repair 与深诊断

#### 当前实现事实

- `/ops` 已承担 raw config、secret inventory、advanced reindex 与 connector sample import
- providers / connectors tab 已明确加入 handoff 链接，提示回到对象页做日常配置

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 找到某类 raw config 或高级修复动作
- 判断当前 tab 是否已加载、已配置
- 完成保存、导入、reindex 或密钥补齐
- 被引导回对象页继续日常工作

#### 首屏必须回答的 3 个问题

1. 当前是不是应该在 `/ops` 处理，而不是在对象页处理
2. 这个 tab 当前是否已配置、是否有错误
3. 我应该在这里保存 / 修复，还是 handoff 回对象页或 `/outbox`

### 2.2 入口与页面归属

#### `/ops`

作为总控台，仅承接：

- raw config
- diagnostics
- raw transport / queue repair
- secret write-only inventory
- advanced actions

#### 对象页与治理页的回指

`/ops` 内必须持续提示用户回到对象页或专门修复台：

- Providers -> `/providers`
- Connectors -> `/connectors`
- Delivery residue replay -> `/outbox`

#### 与 Governance 的区别

- Governance 负责平台默认与跨对象规则
- Ops 负责 raw config 与紧急修复

### 2.3 页面结构

#### 顶层结构

推荐保持当前 tab 架构：

1. `auth`
2. `approval`
3. `secrets`
4. `providers`
5. `connectors`
6. `prompts`
7. `desense`
8. `advanced`

#### 设计原则

- 每个 tab 都先告诉用户是否已加载、是否已配置
- 优先提供跳回对象页的 handoff link
- secret 只写不读
- 有边界的 delivery residue replay 不在 `/ops` 首屏处理，优先 handoff 到 `/outbox`

### 2.4 CTA 与操作层级

#### 主动作

- `保存`
- `导入`
- `重建索引`

#### 次级动作

- `前往对象页`
- `查看缺失 Secret`
- `查看加载状态`

#### 高级动作

- `执行高级修复`
- `查看原始配置`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在高级动作或导入区
- L4/L5 不应默认占据 tab 首屏
- internal repair context、超大 payload 与明文 secret 永不默认暴露

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Ops`
- 页面名：`Ops Console`

#### 页面叙事

- 页面讲“平台总控与修复”
- 不讲“日常配置中心”
- 不把 Ops 讲成 Governance 或 Setup 的延伸页

### 3.2 页面标题与副标题

#### 页面标题

- 标题：`Ops`
- 副标题应表达：处理 raw 配置、Secrets 与高级修复动作

### 3.3 CTA 文案

主路径默认使用：

- `保存`
- `导入`
- `重建索引`

次级路径默认使用：

- `前往对象页`
- `查看缺失 Secret`
- `查看加载状态`

高级区允许：

- `执行高级修复`
- `查看原始配置`

### 3.4 状态文案

#### 当前没有修复项

- 结论：`当前没有必须立即处理的 Ops 事项`
- 细节：仍可查看 raw config、Secrets 或 advanced actions
- 动作：`切换到对应 Tab`

#### raw config 加载失败

- 结论：`当前无法加载配置文件`
- 细节：展示 path 与错误原因
- 动作：`修复文件` 或 `前往对象页`

#### secret 缺失

- 结论：`存在关键 Secret 未设置`
- 细节：按 owner_type / owner_id 展示缺失项
- 动作：`补 Secret`

#### 高级动作失败

- 结论：`平台级修复动作执行失败`
- 细节：如 reindex、import、update config 失败
- 动作：`查看错误并重试`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- secret 明文
- 过大 raw payload
- internal repair context
- 把 Ops 讲成“普通对象管理页”

这些内容可留在高级区，不应主导 Ops 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/ops` 已清晰表达为平台总控与修复控制台
- 每个 tab 先给配置状态和 handoff 方向，而不是先暴露 raw payload
- `Ops`、`Governance`、对象页、`/outbox` 的边界清晰

### 4.2 交互级验收

- 用户能完成保存、导入、reindex、补 secret 等主动作
- 日常配置任务会被明确 handoff 回对象页
- delivery residue replay 不再被默认引导留在 `/ops`

### 4.3 展示级验收

- 各 tab 至少展示 loaded / configured 状态、主要控制项与更新时间
- “无修复项”“raw config 加载失败”“secret 缺失”“高级动作失败”等状态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖 tab 切换、handoff 链接和关键错误态
- 需要浏览器或截图验收确认 Ops 默认叙事已经从“杂糅后台”收口为“总控与修复控制台”
- 若后端尚未提供更丰富的 diff / diagnostics，前端不应伪装成已有完整修复分析能力

### 4.5 剩余限制说明

- 更复杂的 raw diff、repair simulation 和跨 tab 依赖分析可作为下一阶段增强项
- 底层 secret、transport、repair 细节继续保留在高级区
