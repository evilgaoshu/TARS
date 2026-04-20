# 平台配置、自动化与平台动作总览

## 1. 目标

统一说明：

- 平台配置如何导入导出
- `Operate / Governance / Ops` 如何分层，以及 Automations / Trigger Policy / Hooks 如何归位
- Skill 如何受控创建或变更平台对象
- `Operate`、`Governance` 与 `Ops` 如何分工

## 2. 文档拆分

- [30-strategy-platform-config-bundles.md](./30-strategy-platform-config-bundles.md)
- [30-strategy-automations-and-triggers.md](./30-strategy-automations-and-triggers.md)

## 3. 核心原则

- 日常对象配置归 `Operate`，跨对象默认与高级规则归 `Governance`，平台总控与修复归 `Ops`
- 导入导出是高级能力，不是默认主路径
- Hook 负责内部扩展，Trigger Policy / Event Routing 负责高级治理规则，Automation 负责实际动作
- Skill 发起的平台对象变更必须走受控平台动作链
- 自动化优先只读、低风险路径

## 4. 当前收口方向

- `Automation` 是用户日常主对象
- `Trigger Policy`、`Hooks`、`Event Routing` 下沉到 `Governance / Advanced`
- `Ops` 不再承担这些对象的日常主入口
