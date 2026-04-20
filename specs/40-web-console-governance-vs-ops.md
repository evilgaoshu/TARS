# TARS — Governance 与 Ops 分工规范

> **状态**: 设计基线
> **适用范围**: Providers、Agent Roles、Channels、Web Chat、Notification Templates、Automations 的入口分工与控制面边界
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md)、[40-web-console.md](./40-web-console.md)、[40-web-console-setup-and-ops.md](./40-web-console-setup-and-ops.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Governance 是什么

`Governance` 是跨对象的默认、策略、复用规则与平台边界层。

它回答：

- 平台默认是什么
- 多个对象之间共享的策略是什么
- 哪些规则应该统一控制而不是每个对象各配一遍

#### Governance 不是什么

- 不是对象详情页的另一个别名
- 不是 raw config 编辑器
- 不是临时堆放高级字段的隐藏 tab

#### Ops 是什么

`Ops` 是平台级总控与修复面板。

它回答：

- 平台 raw config 在哪里
- 导入导出如何做
- 诊断、修复、回放、紧急停用怎么做

#### Ops 不是什么

- 不是日常对象配置总入口
- 不是每个主流程的最后兜底跳板
- 不是因为对象页没想清楚就把字段全部塞进去的收纳箱

### 1.2 用户目标与关键场景

#### Governance 负责的任务

- 配平台默认模型绑定
- 配共享安全 / 辅助模型
- 配默认 follow-up 路由和送达策略
- 配模板 locale / render policy
- 配自动化复用规则、默认审批和静默策略

#### Ops 负责的任务

- 看 raw config
- 做导入导出
- 看调试数据与回放
- 做 emergency disable / repair
- 平台级 smoke 与修复

#### 不应再由 Ops 负责的任务

- 日常改 Provider 地址或 token
- 日常给角色选模型
- 日常开关 Web Chat
- 日常编辑通知模板正文
- 日常编辑 Automation 的主逻辑

#### 当前最大问题

当前最大问题不是 `Ops` 页面字段不够多，而是：

- 治理和运维职责混在一起
- 高频配置仍然绕回 `Ops`
- 平台默认、对象字段、raw config 三层经常混成一个视图

### 1.3 状态模型

#### 治理状态

- `已配置`
- `未配置`
- `部分覆盖`
- `存在冲突`

#### Ops 状态

- `raw 已加载`
- `raw 未加载`
- `诊断正常`
- `诊断异常`
- `存在待修复项`

#### 展示优先级

- 治理页先说“默认是否已建立”
- Ops 先说“平台现在需要不需要修复”

### 1.4 核心字段与层级

#### 对象页允许出现的字段

- 完成主任务所需字段
- 对象级状态与验证字段
- 少量必要高级字段

#### 治理页允许出现的字段

- 平台默认值
- 跨对象策略
- 共享路由、共享模板、共享兜底规则

#### Ops 允许出现的字段

- raw YAML / JSON
- secret refs / path
- import / export payload
- replay payload
- worker / scheduler / webhook raw error

#### 明确禁止的混层

- 不把 `secret_ref` 放进对象默认编辑表单
- 不把平台默认模型放进 Provider 详情首屏
- 不把 raw webhook payload 放进 Web Chat 首屏
- 不把 internal event type 平铺进 Automation 默认表单

### 1.5 关键规则与约束

#### 推荐映射

| 域 | 对象页 | 治理页 | Ops |
|----|--------|--------|-----|
| Providers | 连接、探测、模型发现 | 平台默认模型、共享辅助模型 | raw providers config |
| Agent Roles | 角色画像、模型绑定 | 绑定模板、风险策略模板 | 原始角色导入修复 |
| Channels / Web Chat | Channel 配置与验证 | 默认跟进路由、送达优先级、共享 recipient policy | webhook / debug / replay |
| Notification Templates | 正文、预览、启停 | locale / render policy | 导入导出 / raw restore |
| Automations | When / Do / Run As / Notify | 复用 trigger policy、Hooks / 事件路由、默认审批 | scheduler repair / replay |

#### 实施准则

- 如果某个字段只在低频修复场景才需要，就放 `Ops`
- 如果某个字段影响多个对象，优先放治理页
- 如果某个字段属于跨对象高级规则，优先放 `Governance / Advanced`，而不是 `Ops`
- 如果某个字段是完成单对象主任务必需的，就必须留在对象页

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 判断一件事情该在对象页、治理页还是 `Ops` 里处理
- 在治理页配置跨对象默认值
- 在 `Ops` 完成低频 raw 修复
- 避免为了日常配置被迫进入 `Ops`

#### 首屏必须回答的 3 个问题

1. 这是单对象任务、跨对象规则，还是 raw 修复任务
2. 如果是跨对象规则，应该去哪个治理入口
3. 如果是低频修复或 emergency，应该去哪个 `Ops` 分区

### 2.2 入口与页面归属

#### 推荐规则

控制面导航应显式提供 `Governance` 顶层入口，并与对象主路径（`Operate`）和 `Ops` 并列。

#### 对象页

负责高频单对象任务：

- 创建
- 编辑
- 验证
- 启停
- 查看状态

#### 治理页

负责跨对象默认与规则：

- 默认值
- 复用策略
- 跨对象绑定
- 风险与审批边界
- `Advanced` 分区下的 Trigger Policy、Hooks、事件路由等高级规则

#### Ops

负责低频平台操作：

- raw config
- import / export
- diagnostics
- repair
- emergency actions

### 2.3 页面结构

#### 对象页结构原则

- 先看结论
- 再改主配置
- 再做验证
- 高级动作下沉

#### 治理页结构原则

- 按“跨对象决策”组织，而不是按对象复制一遍字段
- 明确影响范围
- 明确默认值与覆盖关系
- `Advanced` 分区留给 Trigger Policy、Hooks、事件路由等跨对象高级规则，而不是放进 `Ops`

#### Ops 结构原则

- 按 raw / diagnostics / repair / emergency 分类
- 稳定支持深链
- 只服务低频和紧急场景

### 2.4 CTA 与操作层级

#### 对象页主动作

- `创建`
- `编辑`
- `验证`
- `启用 / 停用`

#### 治理页主动作

- `配置默认规则`
- `保存治理策略`
- `查看覆盖关系`

#### Ops 主动作

- `查看 raw config`
- `导入`
- `诊断`
- `修复`

### 2.5 页面字段裁剪规则

- 对象页默认不出现 raw config、secret refs、导入导出 payload
- 治理页默认不承担对象级连接、凭据和正文编辑
- `Ops` 默认不承担日常高频主任务

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 跨对象默认与策略：`Governance`
- 低频 raw 修复与总控：`Ops`
- 高频单对象任务：对象页本身

#### 页面叙事

- Governance 讲“默认、策略、复用规则”
- Ops 讲“raw config、诊断、修复、emergency”
- 不把 Governance 讲成高级字段堆放页
- 不把 Ops 讲成万能后台

### 3.2 页面标题与副标题

#### Governance

- 标题：`Governance`
- 副标题应表达：配置跨对象默认值、复用规则与高级平台边界

#### Ops

- 标题：`Ops`
- 副标题应表达：处理 raw config、导入导出、诊断与修复动作

### 3.3 CTA 文案

治理路径默认使用：

- `配置默认规则`
- `保存治理策略`
- `查看覆盖关系`

Ops 路径默认使用：

- `查看 raw config`
- `导入`
- `诊断`
- `修复`

对象页仍沿用各自对象主动作，不借 Governance / Ops 代称。

### 3.4 状态文案

#### Governance 空态

- 标题：`平台默认还未建立`
- 说明：`先建立一套平台默认值，角色、渠道和自动化才能在未显式配置时有稳定兜底。`
- 动作：`配置默认规则`

#### Ops 空态

- 标题：`当前没有需要处理的修复项`
- 说明：`如需查看 raw config、导入导出或调试数据，可切换到对应分区。`
- 动作：`查看 raw config`

#### 治理冲突

- 结论：`当前默认策略存在冲突`
- 细节：说明哪些对象设置覆盖或相互矛盾
- 动作：`调整治理规则`

#### Ops 诊断失败

- 结论：`当前无法完成平台诊断`
- 细节：说明是配置加载失败、权限不足还是 worker 异常
- 动作：`查看 raw 配置`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- 把 `Ops` 讲成“日常配置入口”
- 把 `Governance` 讲成“高级字段收纳箱”
- 把 raw payload、secret ref 平铺进对象主表单

这些混层表达不应成为控制面默认叙事。

---

## 4. 验收清单

### 4.1 页面级验收

- Governance、对象页、`Ops` 三层边界清晰
- 高频单对象任务不再默认绕回 `Ops`
- Trigger Policy、Hooks、事件路由等跨对象高级规则已优先归入 `Governance / Advanced`

### 4.2 交互级验收

- 用户能判断某项配置该去对象页、治理页还是 `Ops`
- 平台默认与对象配置不再混成一个表单
- raw 修复动作不会打断日常主任务路径

### 4.3 展示级验收

- Governance 默认先讲默认值和覆盖关系
- `Ops` 默认先讲是否存在待修复项
- secret refs、raw payload、internal event type 等字段不再默认上浮到对象主路径

### 4.4 测试与验证要求

- 需要页面级测试覆盖 handoff 链接、默认值表达和关键空态 / 错误态
- 需要浏览器或截图验收确认控制面边界已经从“杂糅后台”收口为“对象页 / Governance / Ops”三层结构
- 若后端尚未补齐某些治理资源，前端不应伪装成已有完整治理体系

### 4.5 剩余限制说明

- 更多治理入口和高级修复入口仍可在后续阶段细化
- 但三层边界不应再退化回“对象页不清楚就扔进 Ops”
