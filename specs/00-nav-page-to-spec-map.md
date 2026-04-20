# TARS - 导航与页面到 Spec 映射清单

> **事实源**: `web/src/components/layout/navigation.tsx`、`web/src/App.tsx`
> **目的**: 用真实导航、真实路由和当前目标 IA 之间的映射，指导后续逐页实现与校对。

## 1. 使用原则

- 先以真实 route 和真实页面为准，再映射到目标 spec 体系。
- 每个导航入口至少对应一个主 spec；必要时补充次级关联 spec。
- 工作台页也使用统一四段式 spec 模板，但“功能 Spec”描述的是工作台域，不是假装成配置对象。
- 真实 route / 页面是链接事实源；目标 IA / 对象归属是产品语义事实源。
- 兼容路径可以保留，但产品文案、对象边界和后续实现应以目标 IA 为准。
- 若真实 route 与目标 IA 冲突，以本清单的“实施规则”作为唯一解释，不在各组件 spec 再发明第三套归属。

## 2. Runtime

| 导航 / 路由 | 页面心智 | 主 spec | 关联 spec |
|------------|----------|---------|-----------|
| `/` | Runtime command center | [40-web-console-runtime-dashboard.md](./40-web-console-runtime-dashboard.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/sessions` | diagnosis queue + diagnosis workbench | [40-web-console-sessions-workbench.md](./40-web-console-sessions-workbench.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/sessions/:id` | diagnosis detail workbench | [40-web-console-sessions-workbench.md](./40-web-console-sessions-workbench.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/executions` | approval/run queue + execution review workbench | [40-web-console-executions-workbench.md](./40-web-console-executions-workbench.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/executions/:id` | execution detail review workbench | [40-web-console-executions-workbench.md](./40-web-console-executions-workbench.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/setup` | first-run onboarding + runtime checks | [40-web-console-setup-workbench.md](./40-web-console-setup-workbench.md) | [40-web-console-setup-and-ops.md](./40-web-console-setup-and-ops.md) |

## 3. Delivery / Reach

| 导航 / 路由 | 页面心智 | 主 spec | 关联 spec |
|------------|----------|---------|-----------|
| `/inbox` | 第一方送达工作台 | [40-web-console-inbox-workbench.md](./40-web-console-inbox-workbench.md) | [20-component-channels-and-web-chat.md](./20-component-channels-and-web-chat.md) |
| `/chat` | 第一方 Web Chat 工作台 | [40-web-console-chat-workbench.md](./40-web-console-chat-workbench.md) | [20-component-channels-and-web-chat.md](./20-component-channels-and-web-chat.md) |
| `/providers` | AI provider registry + health + model discovery | [20-component-providers-and-agent-role-binding.md](./20-component-providers-and-agent-role-binding.md) | [10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md) |
| `/channels` | unified channel registry + typed config | [20-component-channels-and-web-chat.md](./20-component-channels-and-web-chat.md) | [10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md) |
| `/msg-templates` | notification template registry/editor | [20-component-notification-templates.md](./20-component-notification-templates.md) | [10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md) |

## 4. Platform

| 导航 / 路由 | 页面心智 | 主 spec | 关联 spec |
|------------|----------|---------|-----------|
| `/connectors` | connector object registry | [20-component-connectors.md](./20-component-connectors.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/connectors/:id` | connector runtime verification detail | [20-component-connectors.md](./20-component-connectors.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/skills` | installed skill registry | [20-component-skills.md](./20-component-skills.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/skills/:id` | skill detail / package editor | [20-component-skills.md](./20-component-skills.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/automations` | governed automation object center | [20-component-automations.md](./20-component-automations.md) | [30-strategy-automations-and-triggers.md](./30-strategy-automations-and-triggers.md) |
| `/extensions` | extension intake / validate / review / import center | [20-component-extensions.md](./20-component-extensions.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/knowledge` | knowledge record inventory / export | [20-component-knowledge.md](./20-component-knowledge.md) | [40-web-console-pages.md](./40-web-console-pages.md) |

## 5. Governance / Ops

| 导航 / 路由 | 页面心智 | 主 spec | 关联 spec |
|------------|----------|---------|-----------|
| `/ops/observability` | built-in observability summary / platform health | [20-component-observability.md](./20-component-observability.md) | [40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md) |
| `/audit` | audit trail search/export | [20-component-audit.md](./20-component-audit.md) | [40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md) |
| `/logs` | runtime logs search | [20-component-logging.md](./20-component-logging.md) | [40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md) |
| `/triggers` | 当前兼容入口；目标应收敛为 Governance / Advanced | [30-strategy-automations-and-triggers.md](./30-strategy-automations-and-triggers.md) | [40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md) |
| `/outbox` | failed/blocked delivery replay console | [20-component-outbox.md](./20-component-outbox.md) | [30-strategy-async-eventing.md](./30-strategy-async-eventing.md) |
| `/ops` | raw config / repair / reindex / emergency console | [40-web-console-ops-console.md](./40-web-console-ops-console.md) | [40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md) |

## 6. Identity / Org

| 导航 / 路由 | 页面心智 | 主 spec | 关联 spec |
|------------|----------|---------|-----------|
| `/identity` | IAM overview | [20-component-identity-access.md](./20-component-identity-access.md) | [40-web-console-pages.md](./40-web-console-pages.md) |
| `/identity/providers` | auth provider registry | [20-component-identity-access.md](./20-component-identity-access.md) | [20-component-org.md](./20-component-org.md) |
| `/identity/users` | user registry | [20-component-identity-access.md](./20-component-identity-access.md) | [20-component-org.md](./20-component-org.md) |
| `/identity/groups` | group registry | [20-component-identity-access.md](./20-component-identity-access.md) | [20-component-org.md](./20-component-org.md) |
| `/identity/roles` | access role registry | [20-component-identity-access.md](./20-component-identity-access.md) | [20-component-org.md](./20-component-org.md) |
| `/identity/agent-roles` | agent role profile + model binding | [20-component-agent-roles.md](./20-component-agent-roles.md) | [20-component-providers-and-agent-role-binding.md](./20-component-providers-and-agent-role-binding.md) |
| `/identity/people` | 当前挂在 identity 下的 people registry；目标主归属是 org domain | [20-component-org.md](./20-component-org.md) | [20-component-identity-access.md](./20-component-identity-access.md) |
| `/org` | organization / tenant / workspace / policy governance | [20-component-org.md](./20-component-org.md) | [40-web-console-pages.md](./40-web-console-pages.md) |

## 7. 当前导航与目标 IA 的偏差与实施规则

| 当前 route / 分组 | 目标归属 | 实施规则 |
|-------------------|----------|----------|
| `/msg-templates` / delivery | `Notification Templates` 对象域 | 当前继续复用 `/msg-templates` route 与 API；UI 文案、对象名、字段命名统一按 `Notification Templates`。 |
| `/triggers` / governance | `Governance / Advanced` 高级治理子模块 | `/triggers` 视为兼容入口和深链目标，不再被解释为独立日常主对象。 |
| `/ops/observability` / governance | `Governance / Observability` | 保留兼容 path；产品心智和页面文案按平台观测摘要页处理，不把它并回 `Ops Console`。 |
| `/providers` / delivery | `Operate / AI / Providers` | 当前导航分组不改变对象语义；Providers 相关 spec、字段和页面职责统一按 AI 域解释。 |
| `/identity/agent-roles` / identity | `Operate / AI / Agent Roles` | 继续保留兼容 path；对象语义、模型绑定和后续实现对照统一按 Agent Roles 独立 AI 域处理。 |
| `/identity/people` / identity | `Org / People` | 继续保留兼容 path；People 的对象定义、字段和主编辑职责统一收敛到 `Org`。 |

## 8. 维护规则

- 每次导航或路由调整后，先更新本清单，再检查主 spec 是否需要同步。
- 每次新增页面时，必须补一条 route -> page mind -> spec 映射。
- 如果真实 route 与目标 IA 不一致，要同时写明“兼容路径”“目标归属”“实施规则”，避免实现继续固化旧心智。
