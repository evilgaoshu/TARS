# TARS — 权限颗粒度模型

> **状态**: Next Phase 设计基线  
> **适用范围**: 平台 RBAC、资源权限、能力调用授权、审批边界  
> **定位**: Users / Authentication / Authorization 平台的细粒度授权模型补充规范

---

## 1. 目标

随着 TARS 从 MVP 演进为平台，权限模型不能只停留在：

- 页面能不能打开
- 某个 token 能不能访问
- 某个高风险动作是否进入审批

企业级平台还需要回答：

- 谁能查看某类平台对象
- 谁能修改某类平台对象
- 谁能调用某类系统能力
- 谁能执行只读动作，谁能执行写动作
- 哪些动作必须审批，哪些动作可以直执

本规范的目标是把权限颗粒度统一到一套可实现、可审计、可扩展的模型中。

---

## 2. 总体原则

### 2.1 不按底层 HTTP endpoint 直接授权

第一阶段不建议把权限颗粒度直接做到：

- 每个接口路径
- 每个 HTTP 方法
- 每个请求参数组合

这类模型过早复杂化，难以维护，也不利于前后端统一。

更合适的做法是：

1. `资源（resource）`
2. `动作（action）`
3. `能力（capability）`
4. `风险等级（risk）`

按这 4 层组合决定授权。

### 2.2 RBAC 解决粗粒度，Capability/Risk 解决细粒度

建议模型：

- `RBAC` 决定“这个角色大体能碰哪些对象和能力”
- `Capability Authorization` 决定“这个具体能力能不能调用”
- `Approval` 决定“这一次高风险动作要不要放行”

也就是：

- `RBAC` 不是审批的替代
- `审批` 也不是权限的替代

### 2.3 先按业务能力拆，再按系统类型细化

这个原则适用于所有第三方系统，不只是监控类系统。

例如不建议先定义：

- `victoriametrics.allow`
- `jumpserver.allow`
- `github.allow`
- `feishu.allow`
- `telegram.allow`

更合适的第一层能力定义是：

- `metrics.query_instant`
- `metrics.query_range`
- `metrics.capacity_forecast`
- `observability.query`
- `delivery.query`
- `channel.message.send`
- `channel.message.read`
- `people.profile.read`
- `people.profile.update`
- `execution.run_command`
- `connector.invoke_capability`

如果未来某个系统支持写能力，再单独拆：

- `alert.silence.create`
- `alert.rule.update`
- `datasource.write`
- `delivery.deploy.start`
- `delivery.release.rollback`
- `channel.webhook.update`

---

## 3. 四层权限模型

### 3.1 资源层（Resource）

建议至少覆盖：

- `connectors`
- `skills`
- `providers`
- `channels`
- `people`
- `users`
- `groups`
- `roles`
- `auth_providers`
- `auth`
- `sessions`
- `executions`
- `audit`
- `knowledge`
- `configs`
- `automations`

资源层主要回答：

- 谁能看
- 谁能改
- 谁能启停
- 谁能导入导出

### 3.2 动作层（Action）

建议统一动作语义：

- `read`
- `create`
- `update`
- `enable_disable`
- `promote_rollback`
- `export_import`
- `invoke`
- `approve`
- `admin`

说明：

- 对大多数平台对象，`delete` 不建议一开始做成硬删除权限
- 更稳的语义通常是 `disable/archive`
- 当前第一版 HTTP/API 落地中，很多控制面先收口为 `read/write`，`enable_disable` 作为后续进一步细化动作语义保留

### 3.2.1 当前最小平台权限矩阵

当前第一版要求最少能稳定表达：

- `connectors.read` / `connectors.write`
- `skills.read` / `skills.write`
- `providers.read` / `providers.write`
- `channels.read` / `channels.write`
- `people.read` / `people.write`
- `users.read` / `users.write`
- `groups.read` / `groups.write`
- `roles.read` / `roles.write`
- `auth.read` / `auth.write`
- `configs.read` / `configs.write`
- `sessions.read` / `sessions.write`
- `executions.read` / `executions.write`
- `audit.read`
- `knowledge.read` / `knowledge.write`
- `outbox.read` / `outbox.write`

### 3.3 能力层（Capability）

能力层用于描述“系统实际能做什么”，它通常用于：

- tool-plan step
- connector capability invoke
- 平台 automation / skill runtime

当前和近期应重点覆盖：

- `metrics.query_instant`
- `metrics.query_range`
- `metrics.capacity_forecast`
- `observability.query`
- `delivery.query`
- `channel.message.send`
- `channel.message.read`
- `people.profile.read`
- `people.profile.update`
- `knowledge.search`
- `execution.run_command`
- `connector.invoke_capability`
- `skill.select`
- `platform_action.*`

能力层比资源层更贴近真实操作风险。

### 3.4 风险层（Risk）

建议至少分为：

- `read_only`
- `mutating`
- `high_risk`

推荐语义：

- `read_only`
  - 默认可在 RBAC 允许范围内直执
- `mutating`
  - 默认需要更高权限，部分动作进入审批
- `high_risk`
  - 默认需要审批，或在无显式放行时拒绝

---

## 4. 与当前系统的衔接

当前系统已经有一些基础能力，可以作为正式模型的起点：

- `invocable`
- `read_only`
- `EvaluateCapability(CapabilityInput)`
- `require_approval`
- `hard_deny_mcp_skill`

认证增强本轮与权限模型的衔接点也已明确：

- `local_password + challenge + TOTP MFA` 只负责“是谁、是否已通过额外校验”
- 平台对象访问控制仍以后端 RBAC 与 permission matrix 为准
- `/login` 前端只做基础步骤显隐与状态提示，不直接决定授权结果
- `ops-token` break-glass 继续保留，但不替代正式角色权限判断

这说明能力级授权并不是从零开始做，而是要把现有零散规则提升为正式平台模型。

建议演进方式：

1. 保留现有 capability runtime 授权
2. 在其上补统一的资源/动作/能力矩阵
3. 再让 Web/API/控制面与审批链都使用同一套授权语义

---

## 5. 第三方系统能力的权限拆分建议

所有第三方系统都不建议只给一个系统级粗权限：

- `victoriametrics.allow`
- `jumpserver.allow`
- `github.allow`
- `telegram.allow`
- `feishu.allow`

更合理的是先按能力拆分，再按读写/风险分级。

### 5.1 监控 / Metrics 类系统

以 VictoriaMetrics / Prometheus 类连接器为例，不建议只给一个粗权限：

- `vm.allow`

更合理的是拆成：

- `metrics.query_instant`
- `metrics.query_range`
- `metrics.capacity_forecast`

这些能力通常都属于：

- `read_only`

如果未来监控/告警平台支持写操作，再单独拆：

- `alert.silence.create`
- `alert.rule.update`
- `alert.datasource.write`

这些能力应被标记为：

- `mutating`
- 或 `high_risk`

这样就能实现：

- 查询监控可直执
- 改规则/加静默默认审批

### 5.2 执行 / 主机接入类系统

以 JumpServer / SSH / 远程执行系统为例，建议至少拆成：

- `execution.run_command`
- `execution.file.read`
- `execution.file.write`
- `execution.service.restart`

其中：

- 命令读取型能力可按策略放宽
- 写文件、重启服务、批量执行通常应标记为 `high_risk`

### 5.3 交付 / 变更类系统

以 GitHub / GitLab / CI/CD / 发布平台为例，建议至少拆成：

- `delivery.query`
- `delivery.commit.read`
- `delivery.release.read`
- `delivery.deploy.start`
- `delivery.rollback`

其中：

- 查询类能力通常 `read_only`
- 发布、回滚类能力通常 `mutating` 或 `high_risk`

### 5.4 渠道 / 通知类系统

以 Telegram / Feishu / Slack 等渠道系统为例，建议至少拆成：

- `channel.message.read`
- `channel.message.send`
- `channel.subscription.update`
- `channel.webhook.update`

其中：

- 发消息不一定等于高风险
- 改 webhook、改订阅、改路由通常应更严格控制

### 5.5 人物 / 通讯录 / 组织类系统

以 People / 外部目录 / 值班平台为例，建议至少拆成：

- `people.profile.read`
- `people.profile.update`
- `people.oncall.read`
- `people.oncall.update`
- `people.approval_route.update`

这类能力虽然不一定触发系统执行，但会影响审批、通知和责任归属，也应进入细粒度权限控制。

---

## 6. 建议的角色 × 权限关系

在当前最小角色集基础上：

- `platform_admin`
- `ops_admin`
- `approver`
- `operator`
- `viewer`
- `knowledge_admin`

建议进一步约束为：

### 6.1 `viewer`

- 可 `read`
- 不可 `update/create/invoke high-risk`

### 6.2 `operator`

- 可查看平台对象
- 可调用低风险 read-only capability
- 可发起需要审批的操作
- 不一定拥有审批权

### 6.3 `approver`

- 不一定能改平台配置
- 但能审批被授权范围内的高风险动作

### 6.4 `ops_admin`

- 可管理运行时与平台配置
- 可创建/更新 connectors/providers/channels/skills 等
- 高风险运行动作仍应保留审批边界

### 6.5 `platform_admin`

- 拥有平台级管理能力
- 但审计仍应完整记录，不意味着跳过留痕

---

## 7. 审批与权限的关系

统一规则建议为：

- `权限` 决定“是否有资格触发某类操作”
- `审批` 决定“某次高风险实例是否放行”

所以：

- 无权限的操作，不应靠审批补齐
- 有权限但高风险的操作，仍应进入审批

示例：

- `metrics.query_range`
  - `read_only`
  - RBAC 允许即可直执
- `execution.run_command`
  - 多数情况下 `high_risk`
  - 有发起权 ≠ 可直接执行
- `connector.invoke_capability`
  - 取决于具体 capability 的 risk

---

## 8. 与 Skill / Connector / Automation 的关系

### 8.1 Skill 不绕过权限模型

Skill 只是“如何编排能力”，不是额外权限来源。

因此：

- Skill 中调用的每一个 step
- 仍应走 capability authorization
- 高风险 step 仍应进入审批

### 8.2 Connector 不直接决定权限模型

Connector 决定：

- 能力从哪里来
- 如何调用系统

权限模型决定：

- 谁能调用
- 在什么风险级别下调用

### 8.3 Automation 不绕过审批

定时任务 / 自动化执行的 step 也必须走同一套：

- resource + action + capability + risk

高风险自动化必须：

- 显式批准
- 或受更严格的 allowlist 约束

---

## 9. 推荐落地顺序

建议按这个顺序实现：

1. 平台 RBAC 与资源动作矩阵
2. Capability 分类与 risk 标记标准化
3. Web/API/审批链统一使用同一授权判断
4. 所有第三方系统能力的读写/风险拆分
5. Skill / Automation / Platform Action 全面接入同一模型

---

## 10. 一句话结论

TARS 的权限颗粒度应优先收敛为：

`资源 + 动作 + 能力 + 风险`

而不是直接跳到：

- 每个系统一个粗权限
- 或每个底层接口一个过细权限

这套模型既能支撑当前 connector/tool-plan/runtime，也能支撑后续企业级 RBAC、审批、Skill 和 Automation 平台化。

---

## 11. 安全回归测试覆盖（已落地，2026-03-27）

授权颗粒度模型的关键边界已有固定自动化验证：

**测试文件**：`internal/api/http/security_regression_test.go`

**运行入口**：`make security-regression`

**覆盖的授权边界**：

| 颗粒度层 | 测试 | 说明 |
|----------|------|------|
| RBAC 角色边界（viewer vs admin） | `TestSecurityViewerCannotWriteConfigs`、`TestSecurityViewerCanReadButNotWriteConnectors` | viewer 只有 read 权限，write 操作返回 403 |
| 端点级权限矩阵 | `TestSecurityUnauthorizedAccessMatrix`、`TestSecurityUnauthorizedWriteAccessMatrix` | 所有受保护端点在无认证时拒绝 |
| Automation/Trigger 不绕过审批 | `TestSecurityViewerCannotRunAutomations`、`TestSecurityAutomationRunRequiresAuth` | automation trigger 同样需要认证和权限 |
| Approval 端点隔离 | `TestSecurityApprovalEndpointRequiresAuth`、`TestSecurityViewerCannotApproveExecution` | 审批操作需要 executions.write 权限 |
| Break-glass ops-token 边界 | `TestSecurityOpsTokenRejectedWhenOpsAPIDisabled`、`TestSecurityOpsTokenGrantsFullAccess` | ops-token 受 OpsAPI.Enabled 开关控制 |
