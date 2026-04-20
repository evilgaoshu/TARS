# TARS — Web Console 主要页面与业务工作台规范

> **状态**: 设计基线
> **适用范围**: 控制面主菜单页面、核心工作台、对象页与总览入口
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联总览**: [40-web-console.md](./40-web-console.md)、[00-nav-page-to-spec-map.md](./00-nav-page-to-spec-map.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### 这份规范是什么

这份文档定义 Web Console 主要业务页面的默认心智、主操作和信息优先级，避免控制面继续退化为“后台管理页面集合”。

它不是单一对象规范，而是“主菜单与关键工作台的总览规范”。

#### 当前页面类型

控制面主要由三类页面构成：

- Runtime Workbenches
- 对象页 / Registry
- 总控与治理页

### 1.2 用户目标与关键场景

#### 关键目标

- 让用户先找到正确入口，再完成主任务
- 保证工作台页服务业务流程，而不是字段堆砌
- 把对象页、治理页、Ops 页的边界稳定下来

#### 关键页面簇

##### Runtime Workbenches

- Dashboard：运行时 command center，详见 [40-web-console-runtime-dashboard.md](./40-web-console-runtime-dashboard.md)
- Sessions：诊断队列与诊断工作台，详见 [40-web-console-sessions-workbench.md](./40-web-console-sessions-workbench.md)
- Executions：审批与执行审阅工作台，详见 [40-web-console-executions-workbench.md](./40-web-console-executions-workbench.md)
- Chat：第一方 Web Chat 工作台，详见 [40-web-console-chat-workbench.md](./40-web-console-chat-workbench.md)
- Inbox：第一方送达工作台，详见 [40-web-console-inbox-workbench.md](./40-web-console-inbox-workbench.md)

##### 核心对象页

- Connectors：接入对象运行台，而不是 manifest 编辑器
- Skills：已安装能力包与运行策略对象
- Extensions：候选扩展 intake / validate / review / import 中心
- Knowledge：知识记录 inventory 与导出页
- Automations：默认心智是闭环自动执行对象，而不是 scheduler 字段页

##### 组织与平台对象页

- Providers：模型接入、健康、模型发现；角色模型绑定不在这里
- Channels：统一入口与送达对象，`reply_current_session` 是策略，不是 target id
- Notification Templates：通知内容资产
- Identity：人类 IAM 概览与对象页
- Org：组织、租户、工作空间与策略主配置
- Agent Roles：profile、capability binding、policy binding、model binding

##### 总控页

- Setup：first-run onboarding + runtime checks，详见 [40-web-console-setup-workbench.md](./40-web-console-setup-workbench.md)
- Ops：平台总控与修复台，详见 [40-web-console-ops-console.md](./40-web-console-ops-console.md)

### 1.3 状态模型

#### 页面类型状态

- `工作台页`
- `对象页`
- `治理页`
- `总控页`

#### 展示优先级

1. 页面先回答“这是哪个类型的入口”
2. 再回答“这里的主任务是什么”
3. 最后才暴露高级动作与实现细节

### 1.4 核心字段与层级

#### L1 默认层

- 页面标题
- 页面副标题
- 主 CTA
- 状态摘要
- 主任务相关信息

#### L2 条件层

- 次级筛选
- 补充摘要
- 关联对象入口

#### L3 高级层

- 调试信息
- 高级规则
- 兼容状态

#### L4 系统隐藏层

- raw payload
- 内部 DTO 名
- secret refs

### 1.5 关键规则与约束

- 工作台页优先服务业务工作流，而不是对象字段穷举
- 工作台页也使用统一四段式 spec 模板，但“功能 Spec”应定义工作台域
- 高风险和低频动作下沉
- 状态、验证和下一步建议前置
- Triggers / Hooks / Event Routing 下沉为 `Governance / Advanced`，不再作为日常主导航对象心智

---

## 2. UX Spec

### 2.1 用户任务

#### 核心任务

- 识别当前页面是工作台、对象页还是总控页
- 在主菜单里快速进入正确入口
- 在同类页面之间获得一致的操作节奏和信息层次

#### 首屏必须回答的 3 个问题

1. 这是哪类页面
2. 在这里能完成什么主任务
3. 下一步该去哪个关联页面或工作台

### 2.2 入口与页面归属

#### Runtime Workbenches

负责运行中业务流：

- Dashboard
- Sessions
- Executions
- Chat
- Inbox

#### 对象页 / Registry

负责对象的创建、编辑、启停、验证与使用关系：

- Connectors
- Skills
- Extensions
- Knowledge
- Automations
- Providers
- Channels
- Notification Templates
- Identity
- Org
- Agent Roles

#### 治理与总控

- Governance：跨对象默认与策略
- Setup：首次起飞与运行体检
- Ops：raw config、诊断与修复

### 2.3 页面结构原则

#### 工作台页

- 先给结论
- 再给当前任务
- 再给证据和关联对象

#### 对象页

- 先给对象状态
- 再给主配置
- 再给验证与使用关系

#### 总控页

- 先说是否需要处理
- 再给修复动作
- raw / diagnostics 下沉

### 2.4 CTA 与操作层级

#### 工作台页主动作

- 继续处理
- 查看详情
- 跳转相关对象

#### 对象页主动作

- 创建
- 编辑
- 验证
- 启用 / 停用

#### 总控页主动作

- 配置默认规则
- 触发检查
- 诊断
- 修复

### 2.5 页面字段裁剪规则

- 默认层只放完成主任务所需信息
- 高级字段不应抢占首屏
- raw payload、secret refs、内部 DTO 名不应默认出现在控制面主路径

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名原则

- 工作台页用“业务对象或工作流名”
- 对象页用“对象名”
- 治理页与总控页用“Governance / Setup / Ops”这类明确职责名

#### 页面叙事

- 页面名称和副标题必须先讲用户任务
- 不讲内部实现类型
- 不把工作台讲成“后台管理页”

### 3.2 页面标题与副标题

#### 工作台页

- 标题直接使用工作台名称
- 副标题应表达当前任务流和下一步动作

#### 对象页

- 标题直接使用对象域名称
- 副标题应表达对象职责、状态与主动作

#### 总控页

- 标题应明确是 `Governance`、`Setup` 还是 `Ops`
- 副标题应表达“默认策略 / 起飞检查 / 修复控制”的差异

### 3.3 CTA 文案

优先使用：

- `创建`
- `编辑`
- `验证`
- `启用`
- `停用`
- `继续处理`
- `查看详情`

避免使用：

- 内部 DTO 名
- 含糊的“高级操作”作为唯一主动作

### 3.4 状态文案

#### 空态

- 要先告诉用户“这里为什么是空的”
- 再告诉用户“下一步做什么”

#### 错误态

- 先讲结论
- 再讲影响
- 最后给出下一步动作

### 3.5 术语黑名单

以下内容不应默认主导控制面叙事：

- manifest
- raw config
- internal event type
- secret ref
- DTO / registry / payload 术语

---

## 4. 验收清单

### 4.1 页面级验收

- 主菜单页面都能被清晰归类为工作台、对象页、治理页或总控页
- 标题、副标题和主 CTA 与页面类型一致
- 控制面不再退化成“后台管理页面集合”

### 4.2 交互级验收

- 用户能从总览快速判断该去哪个入口
- 工作台页、对象页、总控页之间 handoff 清晰
- 高风险与低频动作已经下沉

### 4.3 展示级验收

- 默认层只展示主任务相关信息
- 高级字段与 raw 实现细节不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试和截图验收确认入口分类与页面叙事一致
- 新增页面或改版页面应先对齐此总览规范，再落到子 spec

### 4.5 剩余限制说明

- 更细的 IA 调整仍可继续迭代
- 但“工作台 / 对象页 / Governance / Ops”四类心智不应再混回一页
