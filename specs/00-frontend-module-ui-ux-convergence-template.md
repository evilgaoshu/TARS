# 前端模块 UI/UX 收口规范模板

> **状态**: Active Template
> **用途**: 作为前端模块对齐 spec、设计与真实页面行为时的统一收口模板
> **适用对象**: Web Console 所有主菜单模块、详情页、创建流、编辑流、空态 / 错误态 / 引导页
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)

---

## 1. 目标

这份模板不取代模块 spec 本身。  
模块 spec 应优先按 [00-spec-four-part-template.md](./00-spec-four-part-template.md) 编写；本模板用于审查、整改与页面级收口。

这份模板不记录“代码和 spec 的机械差异”，而是要求按**用户真正看到的页面与交互**收口。

评审和整改都应优先回答：

- 页面首先在讲什么
- 用户最常做的任务能不能顺手完成
- 命名、分组、字段层级是否和 spec 一致
- 是否仍在向用户暴露内部 DTO / manifest / 兼容层心智
- 是否存在“看起来能配、实际上难理解或容易配错”的地方

---

## 2. 必查输入

每个模块都必须同时对照以下输入，而不是只看其中一类：

1. 组件 / 页面 spec
2. 总体 IA / object boundary spec
3. PRD / technical design 中的产品叙事
4. 当前真实导航与路由
5. 当前前端实现
6. 当前运行中的实际页面效果

推荐输入：

- `specs/10-platform-object-boundaries-and-ia.md`
- `specs/40-web-console*.md`
- 对应模块的 `specs/20-component-*.md`
- `project/tars_prd.md`
- `project/tars_technical_design.md`
- `web/src/pages/**`
- `web/src/components/**`
- 运行中的页面截图 / 浏览器检查结果

---

## 3. 模块收口流程

### 3.1 先看真实页面

不要先读 DTO，也不要先看接口类型。

必须先回答：

- 用户从导航进入后，第一眼看到什么
- 首屏主结论是什么
- 主 CTA 是什么
- 页面是在讲“对象”，还是在讲“底层实现”

### 3.2 再对照 spec

逐项核对：

- 对象边界
- 页面入口归属
- 字段分层
- 创建 / 编辑 / 查看 / 修复任务
- 状态模型
- 空态 / 错误态 / 不可用态

### 3.3 最后再看代码

代码检查只用于解释：

- 为什么页面现在这样
- 哪些地方是组件复用导致的偏差
- 哪些地方是 API 不支持导致前端只能妥协

---

## 4. 每个模块都要产出的内容

每个模块必须按下面的固定结构输出。

### 4.1 模块卡片

- 模块名
- 路由
- 所属主菜单分组
- 对应 spec
- 主要页面
- 当前状态：`Aligned / Partial / Off-spec`

### 4.2 用户任务

必须列出：

- 高频任务
- 次级任务
- 首屏必须先回答的 3 个问题

### 4.3 对象与边界检查

必须明确：

- 这个页面在讲哪个对象
- 不应该混进来的对象是什么
- 当前是否有边界串线

### 4.4 信息架构检查

必须明确：

- 导航位置是否对
- 页面标题 / 分组 / 面包屑是否对
- 列表、详情、创建、编辑、修复动作是否在正确位置

### 4.5 字段分层检查

按层级拆：

- `L1` 默认必须可见
- `L2` 条件显示
- `L3` 高级区
- `L4` 系统实现细节，不应默认暴露

同时明确当前页面是否把 `L3/L4` 字段错误上浮。

### 4.6 交互检查

逐项检查：

- 创建流是否合理
- 编辑流是否合理
- 是否应单页完成还是多步流程
- 是否存在 CSV / raw string / internal token 暴露
- 是否有“可见但不可理解”的操作

### 4.7 文案与命名检查

逐项检查：

- 是否仍在讲旧命名
- 是否还带兼容层语言
- 是否把系统实现词暴露给普通用户
- CTA 是否明确、统一、无歧义

### 4.8 状态与反馈检查

必须覆盖：

- loading
- empty
- error
- degraded
- disabled
- success / saved / tested

### 4.9 测试与验证检查

必须写出：

- 应补的页面级测试
- 应补的交互级测试
- 是否需要浏览器截图或实际运行验收

---

## 5. 问题记录格式

每条整改项必须用下面格式。

### 5.1 字段

- 模块
- 问题标题
- 当前实际表现
- spec / design 期望
- 差异说明
- 严重级别：`P1 / P2 / P3`
- 变更类型：`Display / Copy / Interaction / IA / Contract`
- 建议整改方向
- 涉及文件

### 5.2 级别定义

- `P1`: 明显影响用户理解、配置正确性、边界心智或主任务完成
- `P2`: 不会立即阻断，但会持续造成误解、低效或错误操作
- `P3`: 补充可见性、信息完整度或一致性问题

---

## 6. Definition Of Done

模块只有同时满足下面条件，才算“收口完成”：

- 主菜单入口、页面标题、对象边界与 spec 一致
- 首屏先讲用户任务，不先讲实现细节
- 创建 / 编辑 / 查看 / 修复流符合该对象的真实工作流
- L1/L2/L3/L4 字段分层明确，默认区不再暴露系统实现细节
- loading / empty / error / degraded / disabled 状态都可理解
- 命名、文案、CTA 与全站语言统一
- 页面测试或浏览器验收补齐
- 若后端暂不支持，前端不会伪装成“已支持”

---

## 7. 推荐输出物

每轮模块整改推荐产出三份东西：

1. 模块整改报告
2. 可直接执行的整改清单
3. 对应实现 PR / patch + 页面级验证结果

---

## 8. 禁止事项

- 不要把“接口字段不同”直接当成 UI 问题
- 不要只写代码结构问题，不写用户可见后果
- 不要为了对齐 spec 去伪装后端并未支持的能力
- 不要把所有模块揉成一份大杂烩报告
- 不要只跑单测，不看真实页面

---

## 9. 主菜单模块默认清单

做全站收口时，默认按主菜单逐模块推进：

- Dashboard
- Sessions
- Approvals & Runs
- Runtime Checks
- In-app Inbox
- Terminal Chat
- AI Providers
- Channels
- Notification Templates
- Connectors
- Skills
- Automations
- Extensions
- Knowledge
- Metrics
- Audit Trail
- Logs
- Governance Rules
- Outbox Rescue
- Settings (Ops)
- Identity
- Agent Roles
- Tenants

如果导航结构变化，以实际侧边栏为准。
