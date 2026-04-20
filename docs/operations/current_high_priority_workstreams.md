# 当前高优先级主线

> 这是一页“现在先做什么”的入口文档。  
> 面向正在参与 TARS 开发、联调、共享测试或 GitHub 迁移的同学与 agent。

## 先说结论

当前不要从 `docs/` 里随便挑一份长文开始读。  
先按下面这个顺序进入：

1. 读这页，确认当前主线。
2. 如果要动 `192.168.3.100`，读 [shared_lab_192.168.3.100.md](./shared_lab_192.168.3.100.md)。
3. 如果要准备 GitHub，读 [github_migration_prep_runbook.md](./github_migration_prep_runbook.md)。
4. 如果要理解产品边界和长期方向，再读 [../../project/tars_prd.md](../../project/tars_prd.md)。

## 当前产品焦点（先统一口径）

- 当前 TARS 不是“大而全 DevOps 平台”。
- 当前更准确的产品焦点是：**值班排障闭环助手**，围绕 `incident -> diagnosis -> approval -> execution -> verification -> evidence / handoff` 收口。
- `Sessions` 是 incident queue / diagnosis workbench，首屏先看**结论、风险、下一步、状态、证据**。
- `Executions` 是 approval / run queue，首屏先看**动作、原因、风险、审批、结果、观察建议**。
- Tool-plan 走 evidence-first：先看 `metrics / logs / observability / delivery / knowledge`，证据不足或明确需要修复时才升级到 `SSH / execution`。
- shared-lab 当前质量门只服务这条闭环主链，不把 post-MVP 的大平台能力重新拉回当前 scope。

## 当前主线 1：Connector 创建与探测可信化

**目标**

- `SSH / VictoriaMetrics / VictoriaLogs` 作为一等 connector 时，创建流必须可信。
- `Base URL`、鉴权字段、`测试连接` 结果必须和真实运行时一致。
- 不能再出现“示例缺失”或“无论填什么都测试成功”的假阳性。

**先看**

- [../../specs/20-component-connectors.md](../../specs/20-component-connectors.md)
- [../../project/tars_prd.md](../../project/tars_prd.md)
- [shared_lab_192.168.3.100.md](./shared_lab_192.168.3.100.md)

**完成标准**

- 模板会给出合理的示例 URL / hint。
- `测试连接` 对空值、非法 URL、不可达地址都会失败。
- 只有真实 probe 成功时才显示 success。
- 在 `192.168.3.100` 上完成真实验证。

## 当前主线 2：First-run Setup 与 Runtime Checks 的 E2E 质量门

**目标**

- `/setup` 只承担首次安装。
- `/runtime-checks` 只承担初始化完成后的运行体检。
- 未初始化时任何业务路径都应统一进入 `/setup`。
- 初始化完成后，`/setup` 不再承担运行态控制台职责。

**先看**

- [../../specs/40-web-console-setup-and-ops.md](../../specs/40-web-console-setup-and-ops.md)
- [../../project/tars_prd.md](../../project/tars_prd.md)
- [mvp_completion_checklist.md](./mvp_completion_checklist.md)

**完成标准**

- setup 首跑、provider check、complete、登录引导都被自动化测试锁住。
- 登录页和路由守卫不再靠 `/api/v1/setup/wizard` 的 `401` 猜系统状态。
- Setup 完成后进入 `/runtime-checks`。

## 当前主线 3：共享测试机 `192.168.3.100` 首跑与联调闭环

**目标**

- 这台机器是当前最重要的受控联调环境。
- 它需要稳定承载：首跑、Runtime Checks、connector 创建、共享配置、观测栈联调。

**先看**

- [shared_lab_192.168.3.100.md](./shared_lab_192.168.3.100.md)
- [team_dev_test_environment.md](./team_dev_test_environment.md)
- [records/local_observability_lab_2026-04-08.md](./records/local_observability_lab_2026-04-08.md)

**完成标准**

- agent 知道正确部署、重启、reset 方式。
- agent 不会再手工裸起二进制绕过 `shared-test.env`。
- `VictoriaMetrics / VictoriaLogs` 与 TARS connector 主路径可在这台机器上真实联调。
- 本地开发机没有显式 token，不应阻塞 shared smoke / live / profile 验证；以共享机托管环境证据为准。
- `logs / observability / delivery` 仍是只读证据路径，不作为共享机上的直接写入或修复入口。

## 当前主线 4：GitHub 首次基线与 CI/CD 边界

> **状态（2026-04-11）：研究与技术 gate 已基本完成，仍待团队 sign-off。**  
> 凭据轮换审计于 2026-04-08 完成：全部凭据类均已确认为非生产值（`invalid/non-live`），当前已无工程阻塞项。  
> 进入实际 push 前，仍需确认 `main` 分支保护 / required checks、rotation window 批准状态与首次 baseline 版本。

**目标**

- 先把仓库安全、清晰地迁到 GitHub。
- GitHub Actions 只做可重复、无共享机依赖的 required checks。
- 共享测试机 `192.168.3.100` 不进入 GitHub required runtime。

**先看**

- [github_migration_prep_runbook.md](./github_migration_prep_runbook.md)
- [github_publishable_baseline.md](./github_publishable_baseline.md)
- [github_actions_baseline_scope.md](./github_actions_baseline_scope.md)
- [github_cicd_and_secret_management_plan.md](./github_cicd_and_secret_management_plan.md)
- [records/credential_rotation_execution_tracker_2026-04-08.md](./records/credential_rotation_execution_tracker_2026-04-08.md)

**完成标准**

- 仓库能区分"可发布内容"和"共享机本地状态"。✅（研究已完成）
- required checks 只跑 L0 / L1 / L2 / static demo build。✅（workflow 已收口）
- 公开仓库前能先看懂"还差哪些内部信息卫生清理"。✅（凭据扫描已完成）
- branch policy / baseline sign-off / rotation window 这类人工门槛已明确。⬜（待团队确认）

## 当前主线 5：SSH 凭据托管

> **状态（2026-04-11）：主链已落地，剩治理尾项。**  
> 已完成 metadata + encrypted secret storage、API、`/ops?tab=secrets` 入口、`ssh_native` runtime 接入、fail-closed 默认行为。  
> 下一步聚焦 `rotation_required` 治理、break-glass、dashboard / dashboard-like 可见性与后续轮换运营，而不是从零开始设计。

**目标**

- 后续支持用户输入 SSH 密码 / 私钥并由系统托管。
- 但不能把明文密码/私钥塞进普通 runtime config JSON 文档。

**先看**

- [../superpowers/specs/2026-04-08-ssh-credential-custody-design.md](../superpowers/specs/2026-04-08-ssh-credential-custody-design.md)
- [../../project/tars_prd.md](../../project/tars_prd.md)

**完成标准**

- 明确加密、权限、审计、脱敏、执行时注入边界。✅（主链已落地）
- GitHub demo 和未配置环境默认 fail closed。✅（已实现）
- 存储 / API / UI 入口 / runtime 注入主路径完成。✅（已实现）
- rotation / break-glass / dashboard 等治理尾项补齐。⬜（待继续收口）

## 不要从哪里开始

这些文档很有价值，但不适合作为“现在要做什么”的第一入口：

- `docs/reports/*`：更多是阶段性报告、调研、记录，不是当前执行入口。
- `project/tars_dev_tracker.md`：信息量很大，适合回溯，不适合新 agent 快速上手。
- `docs/reference/*`：适合查接口、配置、schema，不适合先判断优先级。

## 如果你是另一个 agent

### 你要部署或重启共享机

只要先读：

- [shared_lab_192.168.3.100.md](./shared_lab_192.168.3.100.md)

### 你要做 GitHub 相关工作

按这个顺序读：

1. [github_migration_prep_runbook.md](./github_migration_prep_runbook.md)
2. [github_actions_baseline_scope.md](./github_actions_baseline_scope.md)
3. [github_cicd_and_secret_management_plan.md](./github_cicd_and_secret_management_plan.md)

### 你要做产品/架构判断

先读：

- [../../project/tars_prd.md](../../project/tars_prd.md)
- [../../project/tars_technical_design.md](../../project/tars_technical_design.md)

## 相关入口

- 文档总入口：[../README.md](../README.md)
- 运维与交付入口：[README.md](./README.md)
- 用户与管理员手册入口：[../guides/README.md](../guides/README.md)
- API / 配置 / Schema 入口：[../reference/README.md](../reference/README.md)
