# TARS 规范文档索引

> TARS 平台规范、策略、控制面与设计基线统一入口。

## 命名规则

- `00-` 索引与导航
- `10-` 平台总纲与平台基线
- `20-` 一级组件规范
- `30-` 横切策略、治理与运行时规则
- `40-` Web / UX / 交互规范
- `90-` 设计基线、路线图与下一阶段主题

## 00 索引

- [00-specs-index.md](./00-specs-index.md) - 当前索引
- [00-nav-page-to-spec-map.md](./00-nav-page-to-spec-map.md) - 真实导航 / 页面到 spec 映射清单
- [00-spec-four-part-template.md](./00-spec-four-part-template.md) - 面向对象与页面 spec 的四段式模板（功能 / UX / 内容 / 验收）
- [00-frontend-module-ui-ux-convergence-template.md](./00-frontend-module-ui-ux-convergence-template.md) - 前端模块 UI/UX 收口模板（用于逐模块审查与整改）
- [00-frontend-ux-spec-audit-2026-03-29.md](./00-frontend-ux-spec-audit-2026-03-29.md) - 前端视觉 / 交互 / IA 验收审查报告（基于实际页面与实现的补充审查）

## 10 平台总纲

- [10-platform-components.md](./10-platform-components.md) - 一级组件总览与对象边界
- [10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md) - 对象边界与信息架构总规范
- [10-platform-component-governance.md](./10-platform-component-governance.md) - 一级组件的统一治理要求
- [10-platform-components.zh-CN.md](./10-platform-components.zh-CN.md) - 一级组件总览中文镜像
- [10-platform-dependency-compatibility.md](./10-platform-dependency-compatibility.md) - 平台依赖、兼容性与部署要求
- [10-platform-dependency-compatibility.zh-CN.md](./10-platform-dependency-compatibility.zh-CN.md) - 平台依赖、兼容性与部署要求中文镜像

## 20 一级组件

- [20-component-connectors.md](./20-component-connectors.md) - Connectors 规范
- [20-component-skills.md](./20-component-skills.md) - Skills 规范
- [20-component-providers-and-agent-role-binding.md](./20-component-providers-and-agent-role-binding.md) - Providers 与 Agent Role Binding 规范
- [20-component-channels-and-web-chat.md](./20-component-channels-and-web-chat.md) - Channels 与 Web Chat 规范
- [20-component-notification-templates.md](./20-component-notification-templates.md) - Notification Templates 规范
- [20-component-automations.md](./20-component-automations.md) - Automations 规范
- [20-component-identity-access.md](./20-component-identity-access.md) - Identity / Access 规范
- [20-component-identity-access.zh-CN.md](./20-component-identity-access.zh-CN.md) - Identity / Access 中文镜像
- [20-component-agent-roles.md](./20-component-agent-roles.md) - Agent Roles 规范
- [20-component-observability.md](./20-component-observability.md) - Observability 规范
- [20-component-logging.md](./20-component-logging.md) - Logging 规范
- [20-component-extensions.md](./20-component-extensions.md) - Extensions 规范
- [20-component-knowledge.md](./20-component-knowledge.md) - Knowledge 规范
- [20-component-audit.md](./20-component-audit.md) - Audit 规范
- [20-component-outbox.md](./20-component-outbox.md) - Outbox 规范
- [20-component-org.md](./20-component-org.md) - Org 规范

## 30 横切策略

- [30-strategy-platform-config-and-automation.md](./30-strategy-platform-config-and-automation.md) - 配置、自动化与平台动作总览
- [30-strategy-platform-config-bundles.md](./30-strategy-platform-config-bundles.md) - 平台配置导入导出与 bundle 规范
- [30-strategy-automations-and-triggers.md](./30-strategy-automations-and-triggers.md) - Automations 与 Advanced Trigger Governance 规范
- [30-strategy-platform-config-and-automation.zh-CN.md](./30-strategy-platform-config-and-automation.zh-CN.md) - 配置与自动化中文镜像
- [30-strategy-async-eventing.md](./30-strategy-async-eventing.md) - 异步事件与消息总线演进策略
- [30-strategy-automated-testing.md](./30-strategy-automated-testing.md) - 自动化测试策略
- [30-strategy-authorization-granularity.md](./30-strategy-authorization-granularity.md) - 权限颗粒度模型
- [30-strategy-command-authorization.md](./30-strategy-command-authorization.md) - 命令与能力授权策略
- [30-strategy-desensitization.md](./30-strategy-desensitization.md) - 数据脱敏与本地 LLM 辅助策略
- [30-strategy-third-party-integration.md](./30-strategy-third-party-integration.md) - 第三方系统接入策略
- [30-strategy-reasoning-prompt-injection.md](./30-strategy-reasoning-prompt-injection.md) - 推理 Prompt 注入与脱敏协同

## 40 控制面与 UX

- [40-web-console.md](./40-web-console.md) - Web Console 总览与全局壳
- [40-web-console-pages.md](./40-web-console-pages.md) - 主要页面与业务工作台规范
- [40-web-console-runtime-dashboard.md](./40-web-console-runtime-dashboard.md) - Runtime Dashboard 规范
- [40-web-console-sessions-workbench.md](./40-web-console-sessions-workbench.md) - Sessions Workbench 规范
- [40-web-console-executions-workbench.md](./40-web-console-executions-workbench.md) - Executions Workbench 规范
- [40-web-console-chat-workbench.md](./40-web-console-chat-workbench.md) - Chat Workbench 规范
- [40-web-console-inbox-workbench.md](./40-web-console-inbox-workbench.md) - Inbox Workbench 规范
- [40-web-console-setup-and-ops.md](./40-web-console-setup-and-ops.md) - Setup 与 Ops 分流规范
- [40-web-console-setup-workbench.md](./40-web-console-setup-workbench.md) - Setup Workbench 规范
- [40-web-console-ops-console.md](./40-web-console-ops-console.md) - Ops Console 规范
- [40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md) - Governance 与 Ops 分工规范
- [40-ux-design-system.md](./40-ux-design-system.md) - 设计系统、设计语言与页面骨架
- [40-ux-frontend-optimization-workflow.md](./40-ux-frontend-optimization-workflow.md) - 前端 UI/UX 优化工作流与团队执行规范
- [40-ux-unified-list-bulk.md](./40-ux-unified-list-bulk.md) - 统一列表与批量操作框架
- [40-ux-telegram.md](./40-ux-telegram.md) - Telegram 交互规范

## 90 设计 / 路线 / 缺口

- [90-design-tool-plan-diagnosis.md](./90-design-tool-plan-diagnosis.md) - Tool-plan 诊断设计
- [90-design-self-evolving-platform.md](./90-design-self-evolving-platform.md) - 受控自进化 / 自扩展平台规范
- [91-roadmap-post-mvp.md](./91-roadmap-post-mvp.md) - Post-MVP 路线
- [92-enterprise-platform-next-phase-topics.md](./92-enterprise-platform-next-phase-topics.md) - 企业级平台下一阶段主题

## 推荐阅读顺序

1. 从 [10-platform-components.md](./10-platform-components.md) 和 [10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md) 建立平台对象边界与信息架构。
2. 再读 Providers / Channels / Notification Templates / Automations / Identity / Agent Roles / Org 各组件规范。
3. 需要理解高级治理时，读 [30-strategy-platform-config-and-automation.md](./30-strategy-platform-config-and-automation.md)、[30-strategy-automations-and-triggers.md](./30-strategy-automations-and-triggers.md)、[30-strategy-async-eventing.md](./30-strategy-async-eventing.md)。
4. 需要理解真实导航、真实 route 与 spec 的对应关系时，读 [00-nav-page-to-spec-map.md](./00-nav-page-to-spec-map.md)。
5. 需要理解控制面工作台时，读 Dashboard / Sessions / Executions / Chat / Inbox / Setup / Ops 相关 40- 文档。
