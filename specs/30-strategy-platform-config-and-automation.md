# TARS — 平台配置、自动化与平台动作总览

## 1. 目标

统一定义：

- 平台配置如何导入导出
- Operate / Governance / Ops 如何分层，Automations / Trigger Policy / Hooks 如何归位
- Skill 如何受控创建或变更平台对象
- `Operate`、`Governance` 与 `Ops` 的职责边界

## 2. 文档拆分

本规范作为总览，详细内容拆到：

- [30-strategy-platform-config-bundles.md](./30-strategy-platform-config-bundles.md)
- [30-strategy-automations-and-triggers.md](./30-strategy-automations-and-triggers.md)

## 3. 核心原则

- 日常对象配置归 `Operate`，跨对象默认与高级规则归 `Governance`，平台总控与修复归 `Ops`
- 导入导出是高级能力，不是默认主路径
- Hook 负责内部生命周期扩展，Trigger Policy / Event Routing 负责高级治理规则，Automation 负责实际动作
- Skill 发起的平台对象变更必须走受控平台动作链
- 自动化默认优先只读、低风险路径

## 4. 与一级组件的关系

- `Connectors`、`Providers`、`Channels`、`Automations`、`Agent Roles` 都是平台对象
- Bundle / Import / Export 作用于这些对象，但不改变它们的日常配置主入口
- `Triggers` 不再作为日常主对象；相关能力下沉到 `Governance / Advanced`
- 相关对象边界见 [10-platform-components.md](./10-platform-components.md)

## 5. 下一阶段重点

1. 继续收紧对象配置入口，显式区分 `Operate / Governance / Ops`。
2. 先做低风险自动化与巡检闭环，再把 Trigger Policy / Hooks / Event Routing 收口到 `Governance / Advanced`。
3. 把 bundle/import/export 收成统一高级能力，而不是让各页各自长实现。
