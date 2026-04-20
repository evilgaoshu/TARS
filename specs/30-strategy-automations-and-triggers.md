# TARS - Automations 与 Advanced Trigger Governance 规范

> **状态**: 设计基线
> **适用范围**: Automations 支撑规则、Triggers、Hooks、Trigger Policy、Event Routing、Advanced Rules
> **关联**: [20-component-automations.md](./20-component-automations.md), [40-web-console-governance-vs-ops.md](./40-web-console-governance-vs-ops.md)

---

## 1. 对象定义

### 1.1 这组对象是什么

这不是日常主对象，而是 `Automation` 背后的 **高级规则治理层**。

包含：

- `Trigger`
- `Trigger Policy`
- `Hook`
- `Event Routing`
- `Advanced Rules`

### 1.2 这组对象不是什么

- 不是普通用户创建自动化的主入口
- 不是要长期保留在日常主导航的高频对象
- 不是 `Ops` 的 raw debug 杂项区

### 1.3 与 Automation 的关系

- `Automation` 是用户主对象
- 本文对象是其支撑规则层
- 它们应下沉到 `Governance / Advanced`
- 本文对象可以产出可复用 policy / rule reference，但不直接拥有普通自动化的通知目标、通知受众或模板身份

---

## 2. 用户任务

### 2.1 高频但高级的任务

- 维护 built-in trigger matching 规则
- 管理跨对象 trigger policy 模板
- 调整 event routing 与 hook 扩展点
- 处理历史兼容触发逻辑

### 2.2 不应作为普通任务的内容

- 创建“定时巡检自动化”
- 配置通知谁
- 选择执行角色

这些都应在 `Automations` 主对象里完成。

---

## 3. 入口归属

### 3.1 Governance / Advanced

这组对象应下沉到 `Governance / Advanced`。

### 3.2 当前现实兼容

当前真实导航仍有 `/triggers` 独立入口；该入口应视为历史兼容与高级规则编辑器，而不是长期主 IA。

### 3.3 `Ops`

只处理 raw replay、worker repair、事件诊断，不负责规则主编辑。

---

## 4. 字段分层

### 4.1 L1 默认字段

- rule name
- event / schedule source
- match condition summary
- target automation / action
- enabled status

### 4.2 L2 条件字段

- dedupe / cooldown
- policy template reference
- execution guard / approval template reference

### 4.3 L3 高级字段

- raw event types
- hook binding
- transform / payload mapping
- automation selection precedence
- fallback rule reference

### 4.4 L4 系统隐藏字段

- worker shard metadata
- internal lifecycle hook names

### 4.5 L5 运行诊断字段

- last matched payload
- replay trace
- rule evaluation detail

---

## 5. 页面结构

### 5.1 Advanced Rules 页

推荐按以下区块组织，而不是平铺 raw rules：

1. Trigger Policies
2. Event Routing
3. Hooks
4. Compatibility Triggers
5. Evaluation Diagnostics

### 5.2 与 Automations 的协作

- Automation 默认引用 policy / rule 模板
- 只有需要越过普通表单能力时，用户才进入 Advanced
- 具体 `delivery.targets / delivery.audience / delivery.template_id / reply_current_session` 仍留在 Automation 主对象

---

## 6. 状态模型

- `enabled`
- `disabled`
- `cooling_down`
- `invalid`
- `conflicting`
- `compatibility_mode`

---

## 7. 空态 / 错误态

### 7.1 没有高级规则

- 结论：`当前没有额外高级规则`
- 细节：普通自动化仍可运行
- 动作：返回 `Automations`

### 7.2 规则冲突

- 结论：`当前高级规则存在冲突`
- 细节：多个 policy / routing 命中同一事件但动作矛盾
- 动作：调整 rule precedence

### 7.3 Hook / routing 失效

- 结论：`事件已到达，但高级规则没有正确接住`
- 细节：hook 未注册、event type 不匹配或 target 不存在
- 动作：查看 diagnostics / replay

---

## 8. API 映射附录

### 8.1 当前 API

- `GET /api/v1/triggers`
- `GET /api/v1/triggers/{id}`
- `POST /api/v1/triggers`
- `PUT /api/v1/triggers/{id}`
- `POST /api/v1/triggers/{id}/enable`
- `POST /api/v1/triggers/{id}/disable`

### 8.2 推荐演进方向

- Trigger API 继续保留作兼容层
- 新产品口径统一为 `Governance / Advanced`
- Trigger Policy、Hooks、Event Routing 应逐步拆出更清晰的治理资源
- 具体通知目标、受众、模板身份继续收敛在 Automation payload，而不是回流到 Trigger 主编辑面
