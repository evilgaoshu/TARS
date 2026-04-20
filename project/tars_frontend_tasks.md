# TARS — 前端交付任务拆解 v0.1

> **对应文档**: [tars_prd.md](tars_prd.md) v2.5, [tars_technical_design.md](tars_technical_design.md) v1.4  
> **日期**: 2026-03-11  
> **定位**: 将前端和交互层任务从主 WBS 中独立出来，交给单独负责人推进  
> **范围说明**: Phase 1 以前端交互面为主，不以独立 Web SPA 为目标；Phase 2a 再进入 Web Console

> **⚠️ 状态说明（2026-04-11 更新）**  
> FE-13 至 FE-29 的任务已全量完成（Web Console 基础框架、主要页面、设计系统）。  
> 本文档已降级为历史任务列表，不再指导当前前端打磨方向。  
> 当前前端欠账与 Tier 1~4 打磨优先级以  
> [`docs/operations/spec_focus_review_2026-04-11.md`](../docs/operations/spec_focus_review_2026-04-11.md) 为准。  
> 原始 UX Audit 详见 [`specs/00-frontend-ux-spec-audit-2026-03-29.md`](../specs/00-frontend-ux-spec-audit-2026-03-29.md)。

---

## 1. 前端定位

TARS 的前端不是单一 Web 页面，而是分两层交付：

- `Phase 1 交互前端`: Telegram 内的诊断消息、审批消息、结果消息和回调体验
- `Phase 2a Web Console`: 运维查询和管理后台，包括 session、execution、outbox、reindex 等页面

前端团队只负责：

- 交互呈现和可用性
- API 契约消费
- 消息模板、页面结构、状态展示
- 基础前端埋点和错误上报

前端团队不负责：

- Workflow 状态机
- 风险分级、审批路由、命令执行
- 幂等、审计、事务、outbox 分发
- Telegram / SSH / VM SDK 的底层接入

---

## 2. 交付边界

### 2.1 Phase 1: Telegram 交互前端

目标：

- 在 Telegram 内完成诊断查看、审批操作、执行结果查看
- 不要求跳转后台，不依赖独立 Web UI

交付物：

- 诊断消息模板
- 审批消息模板和按钮交互
- 执行结果消息模板
- 文案、状态、错误提示规范
- Telegram callback payload 对齐样例

### 2.2 Phase 2a: Web Console

目标：

- 提供面向 SRE / 运维负责人 / 知识管理员的最小查询控制台

首批页面：

- Session List
- Session Detail
- Execution Detail
- Outbox List / Replay
- Document Reindex Trigger

暂不纳入：

- 完整 Ticket 工作台
- 多租户后台
- 权限管理页面
- 可视化流程编排

---

## 3. 信息架构

### 3.1 Telegram 交互对象

- `Diagnosis Message`
  - 告警摘要
  - AI 诊断结论
  - 引用信息
  - 执行建议摘要
- `Approval Message`
  - 服务名 / 主机
  - 风险等级
  - 待执行命令
  - 审批来源
  - SLA / 超时
  - 操作按钮
- `Execution Result Message`
  - 执行结果
  - 退出码
  - 截断提示
  - 下一步建议

### 3.2 Web Console 页面

- `Session List`
  - 按状态、主机、时间过滤
  - 快速查看诊断摘要和最近事件
- `Session Detail`

## 4. 进度追踪

### Phase 2a 平台能力
- [x] Theme 基础能力 (Light/Dark/System)
- [x] 国际化基础框架 (zh-CN/en-US)
- [x] 文档中心全量补齐 (User/Admin/Deployment/Architecture)
- [x] API Reference (Swagger UI 集成)
- [x] 核心组件剥离与重构 (Channels, Providers)
- [x] Web Console 运行主链 IA 重构（2026-03-24）
  - Dashboard 改为 runtime command center，而不是单纯 platform health 汇总
  - Sessions / Executions 改为 operator queue / workbench 心智
  - Setup 弱化为 first-run + runtime checks，不再承担长期一级中心职责
  - Inbox / Chat 从 header/FAB 占位升级为独立产品页
  - Command Hub 统一 Header Action / Cmd+K / quick actions 入口
- [x] 第一方触达能力落地（2026-03-24）
  - `/inbox` 站内信工作台接入 `/api/v1/inbox`
  - `/chat` Web Chat 产品壳落地，等待后端 runtime API 接线
  - `/triggers` 页面接入 `/api/v1/triggers`，形成 Governance & Signals 控制面
  - Alert 原文
  - Diagnosis 摘要和引用
  - Timeline
  - Execution 列表
- [x] 全站双语收口第一阶段（2026-03-25）
  - I18n 收敛到 `react-i18next` 单一来源，`web/src/locales/{zh-CN,en-US}.json` 作为唯一翻译资源
  - 全局布局、登录页、Sessions、Executions、Providers、Channels、Skills、Identity 概览完成双语化
  - `web` 本地 `build` 通过，保留原有 lint warning 作为后续优化项
- `Execution Detail`
  - 命令、主机、状态、输出摘要
  - 审批与回放信息
- `Outbox Console`
  - blocked / failed 事件列表
  - 重放操作
  - blocked reason / retry count 展示
- `Ops Action`
  - Reindex documents

---

## 4. 前端模块划分

建议单独前端负责人按以下模块拆：

| 模块 | 职责 | 依赖 | 不拥有的逻辑 |
|------|------|------|--------------|
| `Message Templates` | Telegram 诊断/审批/结果消息结构和文案 | 后端 DTO、产品文案 | 状态推进 |
| `Telegram Callback UX` | 按钮语义、回调映射、幂等提示 | Telegram callback API | 审批路由 |
| `Ops Console Shell` | 页面骨架、路由、鉴权入口 | Ops API | 数据写入规则 |
| `Session Views` | Session 列表和详情展示 | `/api/v1/sessions*` | 业务状态机 |
| `Execution Views` | 执行详情页 | `/api/v1/executions/{id}` | 风险分级 |
| `Outbox Views` | outbox 列表、手动 replay | `/api/v1/outbox*` | replay 判定逻辑 |

原则：

- 前端只做显示和交互，不缓存业务真相
- 所有状态以服务端返回为准
- 不在前端拼装审批规则或执行规则

---

## 5. API 依赖清单

前端主要依赖以下接口：

| 用途 | 接口 | 说明 |
|------|------|------|
| Session 列表 | `GET /api/v1/sessions` | 支持 `status`、`host` 过滤 |
| Session 详情 | `GET /api/v1/sessions/{session_id}` | 展示 alert / diagnosis / timeline |
| Execution 详情 | `GET /api/v1/executions/{execution_id}` | 展示执行信息 |
| Outbox 列表 | `GET /api/v1/outbox` | 查看 blocked / failed |
| Outbox 重放 | `POST /api/v1/outbox/{event_id}/replay` | 运维恢复 |
| 文档重建 | `POST /api/v1/reindex/documents` | 运维动作 |
| Telegram 回调 | `POST /api/v1/channels/telegram/webhook` | 审批交互入口 |

前端必须消费的错误码：

- `ops_api_disabled`
- `unauthorized`
- `not_found`
- `validation_failed`
- `blocked_by_feature_flag`

---

## 6. 里程碑

| 里程碑 | 时间 | 目标 | 出口标准 |
|--------|------|------|----------|
| `F0 Frontend Contract Freeze` | Day 1 | 冻结消息模板字段和页面依赖接口 | DTO 字段和错误码样例确认 |
| `F1 Telegram UX Ready` | Week 2 | 诊断消息、审批消息、结果消息可联调 | 能和后端跑通告警到诊断消息 |
| `F2 Approval UX Ready` | Week 3 | 审批按钮和 callback 行为可联调 | 能完成 approve / reject / timeout 展示 |
| `F3 Ops Console Ready` | Phase 2a | Session / Execution / Outbox 页面可用 | 查询和 replay 可演示 |

---

## 7. 任务拆解

### 7.1 F0 — 契约与交互冻结

| ID | 任务 | 输出 | 估时 | 前置 | 验收 |
|----|------|------|------|------|------|
| `FE-1` | 梳理 Telegram 诊断消息字段 | message field spec | 0.5d | PRD/TSD | 字段和文案冻结 |
| `FE-2` | 梳理审批消息字段和按钮语义 | callback contract | 0.5d | `FE-1` | approve / reject / escalate / edit / view 明确 |
| `FE-3` | 梳理执行结果消息字段 | result message spec | 0.5d | `FE-1` | 成功/失败/人工接管三态明确 |
| `FE-4` | 梳理 Web Console 页面和接口映射 | page-to-api matrix | 0.5d | TSD API | 每个页面都有对应接口 |

### 7.2 F1 — Telegram 诊断与结果交互

| ID | 任务 | 输出 | 估时 | 前置 | 验收 |
|----|------|------|------|------|------|
| `FE-5` | 诊断消息模板设计 | message mock / render rules | 1d | `FE-1` | 告警摘要、诊断、引用展示清楚 |
| `FE-6` | 结果消息模板设计 | result template | 0.5d | `FE-3` | 执行结果和后续动作清楚 |
| `FE-7` | golden snapshot 样例维护 | json fixtures / screenshots | 0.5d | `FE-5`,`FE-6` | 关键消息有固定样例 |
| `FE-8` | Telegram 联调支持 | callback payload / render feedback | 1d | `FE-7` | 与后端字段对齐 |

### 7.3 F2 — 审批交互前端

| ID | 任务 | 输出 | 估时 | 前置 | 验收 |
|----|------|------|------|------|------|
| `FE-9` | 审批消息信息层级设计 | approval template | 1d | `FE-2` | 风险、命令、审批来源、SLA 一屏可见 |
| `FE-10` | 按钮交互与文案约束 | action matrix | 0.5d | `FE-9` | 误操作风险低 |
| `FE-11` | 超时、转交、blocked 状态展示 | state variants | 0.5d | `FE-10` | 非 happy path 可理解 |
| `FE-12` | 审批回调联调 | callback validation | 1d | `FE-10` | 5 类动作联调通过 |

### 7.4 F3 — Web Console 最小集

| ID | 任务 | 输出 | 估时 | 前置 | 验收 |
|----|------|------|------|------|------|
| `FE-13` | 选型并初始化前端工程 | `web/` or standalone repo | 1d | `FE-4` | 能本地启动 |
| `FE-14` | Console shell + 登录态占位 | app shell | 1d | `FE-13` | 可承载 Session/Outbox 页面 |
| `FE-15` | Session List 页面 | list page | 1.5d | `FE-14` | 可筛选、可进入详情 |
| `FE-16` | Session Detail 页面 | detail page | 1.5d | `FE-15` | timeline / diagnosis / executions 清楚 |
| `FE-17` | Execution Detail 页面 | execution page | 1d | `FE-16` | 执行摘要和状态可读 |
| `FE-18` | Outbox Console 页面 | outbox page | 1.5d | `FE-14` | blocked/failed 可查看和 replay |
| `FE-19` | Reindex 操作入口与确认框 | ops action ui | 0.5d | `FE-18` | 运维操作有确认保护 |
| `FE-20` | 错误态、空态、加载态统一 | ui states | 0.5d | `FE-15`-`FE-19` | 页面行为一致 |

### 7.5 F4 — Web Shell 体验基线

实现原则：

- Theme / Docs Center / I18N / 文档搜索优先评估并复用符合要求的成熟开源项目
- 避免重复造轮子，除非现有开源方案无法满足 TARS 的控制面约束
- 自研部分应聚焦：
  - TARS 导航与控制面壳整合
  - 文档中心信息架构
  - 业务文案与多语言内容
  - 与现有页面和平台状态的接缝
- `API Reference` 采用局部升级方案：Docs Center 继续统一承载，但 `/docs/api-reference` 页面优先切换为内嵌 `Swagger UI`；其它文档继续保持 Markdown 渲染

| ID | 任务 | 输出 | 估时 | 前置 | 验收 |
|----|------|------|------|------|------|
| `FE-21` | 主题系统（白天/黑夜/跟随系统） | global theme state | 1d | `FE-14` | 导航、图表、详情页、弹窗统一响应主题切换 |
| `FE-22` | 文档中心入口与内置文档接入 | docs center | 1d | `FE-14` | 右上角可打开用户手册/管理员手册 |
| `FE-23` | 中英文支持 | i18n shell + key pages | 1.5d | `FE-14` | 导航、状态、空态、错误提示可切中英文 |
| `FE-24` | 文档搜索 | docs search | 1d | `FE-22` | 用户手册/管理员手册/部署手册/API 参考可搜索 |
| `FE-24A` | API Reference 局部升级为 Swagger UI | embedded swagger page | 1d | `FE-22`,`FE-24` | `/docs/api-reference` 使用 Swagger UI 渲染 OpenAPI，其他文档继续 Markdown |
 | `FE-25` | 消息模板管理台 | template editor / preview | 1.5d | `FE-14` | ✅ 已实现：`/msg-templates` 页面，3 类型 × 2 语言，预览、变量白名单、测试发送占位、localStorage 持久化 |
 | `FE-26` | 主导航分组重构 + 全局入口 | nav groups + inbox + chat fab | 1d | `FE-14`,`FE-23` | ✅ 已实现（2026-03-23）：左栏 5 区分组折叠导航（总览/运行中心/平台构件/身份与组织/系统与治理）；右上角 Inbox Bell 占位面板；右下角 Chat FAB 占位浮层；工作台→总览改名；Logs/Outbox/Org/MsgTemplates 已下沉归组 |
 | `FE-27` | 前端设计系统统一 + 全站页面迁移 | shadcn/ui 风格组件体系 + 全站页面迁移 | 3d | `FE-14`,`FE-26` | ✅ 已实现（2026-03-24）：建立 shadcn/ui 风格基础组件层（Badge / Separator / Alert / Select / Tooltip / Dialog / Sheet / DropdownMenu）；建立 `page-components.tsx` 统一公共组件层（PageShell / FilterBar / StatusBadge / SummaryGrid / InlineStatus 等 20+ 组件）；重写 AppLayout 使用新组件；全站 20 个页面完成迁移；SkillsList.tsx 完整 UI 层重写；`npm run lint` 与 `npm run build` 通过；规范更新至 `40-web-console.md` §1.4C |
 | `FE-28` | 升级为真正的 Radix UI / shadcn/ui 组件 | 用真实 Radix-backed 实现替换全部手写 UI 组件 | 1d | `FE-27` | ✅ 已实现（2026-03-24）：安装缺失的 Radix UI 包（react-select / react-tooltip / react-dropdown-menu / react-separator / react-scroll-area / react-popover / react-alert-dialog / react-tabs）以及 sonner；用 Radix/shadcn 标准实现替换 Select（含 NativeSelect 别名）、Dialog、Sheet、Tooltip、DropdownMenu、Separator；新增 Command（cmdk 封装）和 SonnerToaster 组件；更新 tailwind.config.js 在根层级暴露标准 shadcn 色彩 token（primary / secondary / destructive / muted / accent / popover / card），保留 shadcn.* 命名空间向后兼容；更新 Button（使用标准 primary / destructive / accent token，TARS amber 迁移至 tars-primary）和 Badge（使用标准 success/warning/danger/info/muted 语义色）；`npm run lint` ✅ `npm run build` ✅ |
 | `FE-29` | 全站 UI/UX 大重构 — 设计系统建立 + 组件层统一 | 建立正式设计系统规范；统一三重组件重复；修复 CSS 架构问题 | 2d | `FE-28` | ✅ Phase 1+2+3 已完成（2026-03-24）：**Phase 1** — card.tsx 重写为标准 shadcn Card，旧版迁移至 card-legacy.tsx；删除死代码 registry-ui.tsx + registry-page.tsx；tailwind.config.js 清理 shadcn.* 别名；index.css 清理 .badge-* CSS 类（11 消费者全部迁移到 shadcn Badge/StatusBadge）；40-web-console.md 新增 §6 设计系统规范。**Phase 2** — 全量 btn→Button 迁移（~91 处 / 20 文件）；全量 input-field→Input/Textarea/NativeSelect 迁移（~158 处 / 21 文件）；新增 textarea.tsx 标准 shadcn Textarea 组件；全量 registry-* CSS 迁移至 Tailwind 内联（~15 处 / 7 文件，仅保留 .registry-table 子选择器）；删除 .btn/.btn-primary/.btn-secondary/.input-field CSS 定义及 10+ registry-* 无消费者 CSS 规则；CSS 产物 71.21KB→69.93KB。**Phase 3A** — AppLayout.tsx 全量 Tailwind 重写（CSS Modules 已删），移动端 Sheet 侧边栏、桌面折叠侧边栏（60px icon-only + Tooltip 标签）、SidebarNav 共享组件、TooltipProvider 全局包裹；Breadcrumbs.tsx 内联样式→Tailwind + Radix DropdownMenu；GlobalSearch.tsx cmdk 样式迁出至 index.css。**Phase 3B** — LoginView 全量无障碍修复（6 处 inline style→Tailwind，label htmlFor，aria-label，role）；page-components 语义化修复（SearchBar/FilterBar/SplitLayout/CardRow）；SessionList + ExecutionList 复选框 aria-label；ExecutionList 冗余类清理 + NativeSelect aria-label；DashboardView border-white/5→border-border（6 处），字体下限统一 text-xs（4 处）。`npm run lint` ✅ `npm run build` ✅（67.58 KB CSS）。|

---

## 8. 交互验收标准

- 诊断消息必须在 Telegram 内直接看懂，不依赖跳转后台
- 审批消息必须明确显示命令、风险等级、审批来源、超时信息
- blocked / failed / timeout / degraded 等异常状态必须有稳定提示文案
- Web Console 所有页面都必须能映射回服务端 API，不允许前端自造状态
- replay / reindex 这类运维操作必须有二次确认

---

## 9. 与主 WBS 的协作接口

前端负责人需要重点对齐这些后端任务：

- 主 WBS 的 Telegram 发送与 callback 基础能力
- 主 WBS 的 Ops API 查询接口
- OpenAPI 草案和错误码表
- 试点样本和 golden payload

建议协作方式：

- 前端不等待所有后端完成后再开始
- 先基于 frozen DTO、mock data、golden cases 开始模板和页面工作
- 每周至少一次按真实 payload 联调

---

## 10. 建议交接方式

如果交给另一个同学做，建议交付以下输入：

- [tars_prd.md](tars_prd.md)
- [tars_technical_design.md](tars_technical_design.md)
- [tars_dev_tasks.md](tars_dev_tasks.md)
- 本文档 [tars_frontend_tasks.md](tars_frontend_tasks.md)
- OpenAPI 草案 `api/openapi/tars-mvp.yaml`

这套输入足够让前端负责人在不持有 Workflow 细节的情况下独立推进交互层。
