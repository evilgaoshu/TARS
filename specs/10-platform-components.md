# TARS - 平台一级组件总览

## 1. 目标

明确 TARS 里哪些对象必须作为一级平台组件治理，而不是长期停留在配置文件、局部模块或隐藏在 `Ops` 里的内部能力。

## 2. 十类一级组件

1. `Connectors`
2. `Skills`
3. `Providers`
4. `Channels`
5. `People`
6. `Users`
7. `Authentication`
8. `Authorization`
9. `Automations`
10. `Agent Roles`

这些组件都应具备独立的：

- registry / inventory
- 控制面
- 状态与运行视图
- 审计
- 导入导出（按需）
- 版本治理（按需）

## 3. 组件边界

### Connectors

接入外部系统并暴露可验证能力。详见 [20-component-connectors.md](./20-component-connectors.md)。

### Skills

编排平台能力并把场景剧本治理为可发布对象。详见 [20-component-skills.md](./20-component-skills.md)。

### Providers

承载 AI 后端连接、模型发现、健康与可复用模型库存，不应继续与平台默认绑定或角色模型选择混层。详见 [20-component-providers-and-agent-role-binding.md](./20-component-providers-and-agent-role-binding.md)。

### Channels

承载统一会话入口与送达目标；`Web Chat`、`Inbox`、`Telegram`、`Slack` 同属 `Channel`，通过 `kind + usages` 表达能力。详见 [20-component-channels-and-web-chat.md](./20-component-channels-and-web-chat.md)。

### People / Users / Authentication / Authorization

共同构成人类身份与访问治理底座；其中 Org / Tenant / Workspace 治理独立归 [20-component-org.md](./20-component-org.md)。详见 [20-component-identity-access.md](./20-component-identity-access.md)。

### Automations

承载定时巡检、事件触发、受控执行与通知闭环。`Trigger` / `Hook` 应作为支撑结构下沉到 `Governance / Advanced`，而不是日常主心智。详见 [20-component-automations.md](./20-component-automations.md) 与 [30-strategy-automations-and-triggers.md](./30-strategy-automations-and-triggers.md)。

### Agent Roles

承载 AI 角色的能力绑定、策略边界、运行时选择与模型绑定。详见 [20-component-agent-roles.md](./20-component-agent-roles.md) 和 [20-component-providers-and-agent-role-binding.md](./20-component-providers-and-agent-role-binding.md)。

## 4. 统一原则

- 对象配置归对象页，治理配置归治理页，平台总控归 `Ops`
- 一级组件默认都需要明确的状态、验证和主操作
- 如果一个对象只能在 `Ops` 中管理，它应视为“尚未完成产品化”

## 5. 进一步阅读

- 统一治理要求见 [10-platform-component-governance.md](./10-platform-component-governance.md)
- 对象边界与 IA 总规范见 [10-platform-object-boundaries-and-ia.md](./10-platform-object-boundaries-and-ia.md)
- 配置、导入导出和自动化见 [30-strategy-platform-config-and-automation.md](./30-strategy-platform-config-and-automation.md)
- 控制面与页面心智见 [40-web-console.md](./40-web-console.md)
