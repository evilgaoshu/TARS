# TARS Spec Focus Review — 2026-04-11

> **版本**: 1.0  
> **日期**: 2026-04-11  
> **方法**: 全量 spec inventory + 逐份阅读分类，覆盖 specs/、docs/superpowers/specs/、project/、docs/operations/、docs/reports/、docs/superpowers/plans/  
> **目的**: 重新收拢项目焦点，为下一轮前端打磨提供结论性指引，并把当前口径收回到“值班排障闭环助手”  

---

## 1. Review 范围与方法

### 1.1 文件总数

| 目录 | 文件数 |
|------|--------|
| `specs/` | 56 |
| `docs/superpowers/specs/` | 5 |
| `project/` | 6 |
| `docs/operations/` | 6（含 records/）|
| `docs/reports/` | 7 |
| `docs/superpowers/plans/` | 7 |
| **合计** | **~87 个 .md 文件** |

### 1.2 Read 范围

以下文件已全文或重点读取（不抽样，逐份有记录）：

**project/**
- `tars_prd.md`（全文 1304 行）
- `tars_technical_design.md`（全文 2733 行）
- `tars_dev_tasks.md`（全文 761 行）
- `tars_dev_tracker.md`（全文 912 行）
- `tars_frontend_tasks.md`（全文 303 行）
- `current_project_entry.md`

**docs/operations/**
- `current_high_priority_workstreams.md`
- `team_dev_test_environment.md`
- `shared_lab_192.168.3.100.md`
- `mvp_completion_checklist.md`
- `records/credential_rotation_execution_tracker_2026-04-08.md`

**specs/**（重点）
- `00-specs-index.md`、`00-nav-page-to-spec-map.md`、`00-frontend-ux-spec-audit-2026-03-29.md`（全文 511 行）
- `10-platform-object-boundaries-and-ia.md`
- `20-component-connectors.md`
- `40-web-console-setup-workbench.md`、`-runtime-dashboard.md`、`-sessions-workbench.md`、`-ops-console.md`、`-executions-workbench.md`、`-governance-vs-ops.md`
- `91-roadmap-post-mvp.md`、`92-enterprise-platform-next-phase-topics.md`

**docs/superpowers/specs/**（全部）
- `2026-03-27-connector-ux-field-model-design.md`
- `2026-03-27-ops-ia-refactor-design.md`（全文 97 行）
- `2026-04-02-priority-reset-and-vector-gate-design.md`
- `2026-04-08-ssh-credential-custody-design.md`

**docs/superpowers/plans/**
- `2026-04-02-five-priority-execution-program.md`（全文 271 行）

**docs/reports/**
- `frontend-priority-backlog-2026-03-29.md`
- `github-migration-and-free-test-env-research-2026-04-02.md`

---

## 2. Spec Inventory 总表

说明：
- `keep-now`：当前主线，必须继续维护和对齐
- `merge`：内容有价值，建议合并到对应主文档，不再独立维护
- `park`：留作参考，但不是当前主要驱动力
- `archive`：历史参考，不应继续驱动当前优先级

### 2.1 project/ 主文档

| 文件 | 分类 | 说明 |
|------|------|------|
| `tars_prd.md` | keep-now（§1-7）/ park（§8+） | §1-7 的产品定位、安全边界和 MVP 设计仍是当前基线；§8 迭代规划及其后的企业化/参考内容当前不指导执行优先级 |
| `tars_technical_design.md` | keep-now（§1-14）/ park（§15.2+） | §1-14 架构主干已全量落地并仍准确；§15.8A Agent Role、§15.5 Connector 已落地；§15.2~15.14 平台化路线部分是 post-MVP 内容 |
| `tars_dev_tasks.md` | merge/park | Sprint 1+2 + P4 大部分任务已完成；与 tracker、workstreams 严重重叠；继续以 `current_high_priority_workstreams.md` 为主线；本文档可降为历史 WBS 归档 |
| `tars_dev_tracker.md` | keep-now | 最有时序价值的活跃记录，但 2026-03-29 至今约 2 周无更新；SSH 凭据托管（2026-04-08 落地）未体现 |
| `tars_frontend_tasks.md` | park | FE-13~29 全部完成，任务已被 `frontend-priority-backlog-2026-03-29.md` 和 `00-frontend-ux-spec-audit-2026-03-29.md` 接管 |
| `current_project_entry.md` | keep-now | 仍是主入口，需保持与其他文档同步 |

### 2.2 docs/operations/

| 文件 | 分类 | 说明 |
|------|------|------|
| `current_high_priority_workstreams.md` | keep-now | 当前最权威的主线状态文档；应持续反映“GitHub 技术 gate 已清、仍待人工 sign-off”和“SSH 凭据主链已落地、剩治理尾项”的口径 |
| `team_dev_test_environment.md` | keep-now | 环境参考，与 `shared_lab_192.168.3.100.md` 互补，仍准确 |
| `shared_lab_192.168.3.100.md` | keep-now | 2026-04-08 校验，最新最准确的共享机运行手册 |
| `mvp_completion_checklist.md` | keep-now | 试点 Go/No-Go 门控，当前部分条目尚未勾选；需持续维护 |
| `records/credential_rotation_execution_tracker_2026-04-08.md` | archive | 所有凭据类都已确认 local-only/non-live，GitHub 技术 gate 已清；最终 push 仍取决于 branch policy、baseline 版本和团队 sign-off |
| `records/local_observability_lab_2026-04-08.md` | keep-now | 观测栈 VM/VL 本机部署参考 |

### 2.3 specs/（按层级分组）

**模板与索引（6 个）**

| 文件 | 分类 | 说明 |
|------|------|------|
| `00-specs-index.md` | keep-now | 主索引 |
| `00-nav-page-to-spec-map.md` | keep-now | 页面→spec 映射，前端主要参考 |
| `00-spec-four-part-template.md` | keep-now | 写新 spec 的模板 |
| `00-frontend-module-ui-ux-convergence-template.md` | keep-now | 前端审计模板 |
| `00-frontend-ux-spec-audit-2026-03-29.md` | keep-now | **最重要的前端审计文档**；当前最权威的 gap 清单，比任何旧 gap 表都强 |
| `README.md` | keep-now | 目录说明 |

**平台对象层（11 个）**

| 文件 | 分类 | 说明 |
|------|------|------|
| `10-platform-components.md` | keep-now | 平台对象总览 |
| `10-platform-components.zh-CN.md` | keep-now | 中文同步版 |
| `10-platform-component-governance.md` | keep-now | 治理模型 |
| `10-platform-dependency-compatibility.md` | keep-now | 依赖兼容矩阵 |
| `10-platform-dependency-compatibility.zh-CN.md` | keep-now | 中文同步版 |
| `10-platform-object-boundaries-and-ia.md` | keep-now | **关键 IA 文档**；Providers / Channel / Automation / Agent Role 边界定义 |
| `20-component-connectors.md` | keep-now | 一等对象 SSH/VM/VL，设计基线 |
| `20-component-providers-and-agent-role-binding.md` | keep-now | Provider + Role 绑定边界 |
| `20-component-agent-roles.md` | keep-now | 已全量落地 |
| `20-component-channels-and-web-chat.md` | keep-now | Web Chat 归属 Channel；设计基线 |
| `20-component-automations.md` | keep-now | Automation 是 trigger 的上级结构 |
| `20-component-skills.md` | keep-now | Skill Registry 设计基线 |
| `20-component-knowledge.md` | keep-now | 知识沉淀设计，与 decision gate 关联 |
| `20-component-outbox.md` | keep-now | 异步事件送达 |
| `20-component-notification-templates.md` | keep-now | 通知模板边界 |
| `20-component-identity-access.md` | keep-now | IAM 设计基线 |
| `20-component-identity-access.zh-CN.md` | keep-now | 中文版 |
| `20-component-audit.md` | keep-now | 审计设计 |
| `20-component-logging.md` | keep-now | 日志设计 |
| `20-component-observability.md` | keep-now | 观测设计 |
| `20-component-org.md` | keep-now | Org 对象设计 |
| `20-component-extensions.md` | keep-now | Extension 设计 |

**策略层（8 个）**

| 文件 | 分类 | 说明 |
|------|------|------|
| `30-strategy-automations-and-triggers.md` | keep-now | Trigger 归属 Automation |
| `30-strategy-authorization-granularity.md` | keep-now | 权限粒度策略 |
| `30-strategy-command-authorization.md` | keep-now | 命令授权白名单/黑名单策略 |
| `30-strategy-async-eventing.md` | keep-now | 异步事件策略 |
| `30-strategy-desensitization.md` | keep-now | 脱敏策略 |
| `30-strategy-reasoning-prompt-injection.md` | keep-now | Prompt 注入策略 |
| `30-strategy-platform-config-and-automation.md` | keep-now | 配置中心策略 |
| `30-strategy-platform-config-and-automation.zh-CN.md` | keep-now | 中文版 |
| `30-strategy-platform-config-bundles.md` | keep-now | 配置 bundle 策略 |
| `30-strategy-automated-testing.md` | keep-now | 自动化测试策略 |
| `30-strategy-third-party-integration.md` | keep-now | 第三方集成策略 |

**UX/Web Console 层（14 个）**

| 文件 | 分类 | 说明 |
|------|------|------|
| `40-web-console.md` | keep-now | Web Console 总规范 |
| `40-web-console-pages.md` | keep-now | 页面列表 |
| `40-web-console-setup-workbench.md` | keep-now | Setup 是 first-run wizard；/runtime-checks 是运行体检 |
| `40-web-console-runtime-dashboard.md` | keep-now | Dashboard 是 operator command center |
| `40-web-console-sessions-workbench.md` | keep-now | Sessions 是诊断队列+诊断台 |
| `40-web-console-executions-workbench.md` | keep-now | Executions 是审批+运行队列 |
| `40-web-console-ops-console.md` | keep-now | Ops 是平台级 raw/repair/diagnostic |
| `40-web-console-governance-vs-ops.md` | keep-now | **关键**：Governance vs Ops 分工边界 |
| `40-web-console-setup-and-ops.md` | keep-now | Setup/Ops IA 边界 |
| `40-web-console-chat-workbench.md` | keep-now | Web Chat 工作台 |
| `40-web-console-inbox-workbench.md` | keep-now | Inbox 工作台 |
| `40-ux-design-system.md` | keep-now | 设计系统已落地 shadcn/ui |
| `40-ux-frontend-optimization-workflow.md` | keep-now | 前端优化工作流 |
| `40-ux-unified-list-bulk.md` | keep-now | 列表 & 批量操作规范 |
| `40-ux-telegram.md` | park | Telegram UX 规范；当前已实现，优先级低于 Web Console 完善 |

**设计专题（4 个）**

| 文件 | 分类 | 说明 |
|------|------|------|
| `90-design-tool-plan-diagnosis.md` | keep-now | Tool plan 诊断流程，当前能力 |
| `90-design-self-evolving-platform.md` | park | 自进化能力，post-MVP 方向 |
| `91-roadmap-post-mvp.md` | park | 多层记忆/MCP/Agent 范式扩展；非当前主线，保留作规划参考 |
| `92-enterprise-platform-next-phase-topics.md` | archive | 企业级多租户/HA/SCIM 等纯 next-phase 主题；当前不驱动任何优先级 |

**specs/ 内的设计稿（已移入 specs/）**

| 文件 | 分类 | 说明 |
|------|------|------|
| `2026-03-27-connector-ux-field-model-design.md` | keep-now | Connector 字段模型重构；仍是设计基线 |
| `2026-03-27-ops-ia-refactor-design.md` | keep-now | Ops IA 重构设计；仍是设计基线 |
| `2026-03-27-specs-reorg-design.md` | archive | 已完成的 spec 重组方案 |

### 2.4 docs/superpowers/specs/

| 文件 | 分类 | 说明 |
|------|------|------|
| `2026-03-27-connector-ux-field-model-design.md` | keep-now | 与 specs/ 同步 |
| `2026-03-27-ops-ia-refactor-design.md` | keep-now | 与 specs/ 同步 |
| `2026-03-27-specs-reorg-design.md` | archive | 已完成 |
| `2026-04-02-priority-reset-and-vector-gate-design.md` | keep-now | **当前优先级合同**；WS1-3 已完成，WS4-5 执行状态需确认 |
| `2026-04-08-ssh-credential-custody-design.md` | merge | Phase 0~4 已全量落地；关键事实应合并入 tars_technical_design.md §15.x 或 ops 入口文档 |

### 2.5 docs/reports/

| 文件 | 分类 | 说明 |
|------|------|------|
| `frontend-priority-backlog-2026-03-29.md` | keep-now | P1/P2 前端欠账，与 audit 互补 |
| `github-migration-and-free-test-env-research-2026-04-02.md` | archive | 研究完成，决策已落实（GitHub-first 但先本地，CF Pages 做 demo，Supabase 不用于共享集成）|
| `github-migration-and-free-test-env-research-2026-04-02.zh-CN.md` | archive | 中文版同上 |
| `pilot-core-decision-gate-template.md` | keep-now | 试点结束后填写的 decision gate 模板 |
| `secret-scan-and-rotation-2026-04-07.md` | archive | 安全扫描完成，凭据已确认 non-live |
| `fault_injection_report.md` | archive | 一次性故障注入报告，历史参考 |
| `README.md` | keep-now | 目录说明 |

### 2.6 docs/superpowers/plans/

| 文件 | 分类 | 说明 |
|------|------|------|
| `2026-04-02-five-priority-execution-program.md` | park | WS1（Green Trunk）、WS2（Coverage Gate）、WS3（GitHub Research）已完成；WS4（Bootstrap Slimming）、WS5（Decision Gate）状态未确认；降为参考 |
| `2026-04-02-green-trunk-bootstrap-and-core-coverage.md` | archive | 详细执行计划，WS1/2 已完成 |
| `2026-03-29-frontend-ui-ux-reconcile-and-priority-fixes.md` | archive | 已被 audit 和 backlog 文档接管 |
| `2026-03-29-remaining-p1-and-docker-deploy.md` | archive | 已完成或被接管 |
| `2026-03-27-ops-ia-refactor.md` | archive | 执行计划已完成，设计基线在 specs/ 中 |
| `2026-03-27-specs-reorg-and-repo-move.md` | archive | 已完成 |

---

## 3. 发现的主要冲突与重复

### 3.1 文档时效性冲突

**问题 1：tars_dev_tracker.md 断层 2 周**  
最后一条有效记录：2026-04-02。  
2026-04-08 落地了 SSH 凭据托管（Phase 0~4）、凭据轮换审计、本地观测栈记录，但均未体现在 tracker。  
→ tracker 需补记，或由 `current_high_priority_workstreams.md` 承担主线跟踪职责。

**问题 2：tars_dev_tasks.md 大量 P4 任务已完成但未标注**  
P4-1~P4-46 大部分已完成（Agent Role 全量落地、设计系统统一、Web Chat 接入等），但文档未同步。  
→ 降为历史 WBS，主线以 workstreams 为准。

**问题 3：tars_prd.md 的后续规划与参考章节篇幅过大，容易让执行焦点外溢**  
§8 迭代规划及其后的参考/评审内容对当前阶段不是直接执行入口，但篇幅占比高，容易分散注意力。  
→ 在 PRD 头部明确"当前适用范围：§1-7"，将 §8 及其后内容明确标成后续规划 / 参考层。

**问题 4：five-priority-execution-program.md WS4/5 执行状态不明**  
WS1（Green Trunk）、WS2（Core Coverage Gate）、WS3（GitHub Research）均已完成并有 artifact 证据。  
WS4（Bootstrap Slimming）和 WS5（Knowledge/Vector/Outbox Decision Gate）执行状态未有记录。  
→ 确认后更新 `current_high_priority_workstreams.md`。

### 3.2 信息架构重复与分裂

**问题 5：frontend 欠账有两份互补文档，需共同参考**  
- `docs/reports/frontend-priority-backlog-2026-03-29.md`：按页面组织的 P1/P2 欠账
- `specs/00-frontend-ux-spec-audit-2026-03-29.md`：更权威、更结构化的审计报告（含 Top 5 优先级）  
两者并非完全重叠，各有独特条目，需同时参考。本文档前端章节以 audit 文档为主，backlog 为补充。

**问题 6：Connector 设计基线分布两处**  
- `specs/2026-03-27-connector-ux-field-model-design.md`（在 specs/）  
- `docs/superpowers/specs/2026-03-27-connector-ux-field-model-design.md`（在 superpowers/specs/）  
内容完全相同，是两份拷贝。→ 以 `specs/` 中的为主，`superpowers/specs/` 保留引用即可。

**问题 7：Ops IA 设计基线同样分布两处**  
同上，`2026-03-27-ops-ia-refactor-design.md` 在 `specs/` 和 `docs/superpowers/specs/` 各一份。

**问题 8：SSH 凭据托管设计已落地，但设计文档仍停在设计稿形态**  
`2026-04-08-ssh-credential-custody-design.md` 描述的 Phase 0~4 均已实现，但技术设计文档（tars_technical_design.md）中尚无对应 §。  
→ 建议将关键 API 和 schema 归并入 tars_technical_design.md。

### 3.3 前端实现与 Spec 的结构性冲突

详见下文第 5 节。核心冲突集中在 3 个区域：
1. 对象边界叙事不统一（Setup 混淆 provider/model-binding；People 归属混乱）
2. 核心语义字段仍用原始字符串（usages/capabilities/template_id）
3. 治理与 Ops 职责混淆（Ops 像配置面而非修复面；Triggers 独立于 Automation）

---

## 4. 统一后的 Product Spine

### 4.1 当前阶段定位

> **阶段**：已超越功能型 MVP，但当前收口目标不是继续铺开“大而全平台”，而是把“值班排障闭环助手”做扎实  
> **机器**：共享测试机 `192.168.3.100`（主力联调机），入口 `http://192.168.3.100:8081`  
> **参照**: `mvp_completion_checklist.md` § Go/No-Go

### 4.2 核心价值链（已验证）

```
输入事件/对话
  → AI 分析（diagnosis）
  → 受控审批（approval policy）
  → SSH 执行（connector + command auth）
  → 校验（verifier）
  → 审计 / 知识沉淀
```

每个环节都已有真实的首跑证据（在 `192.168.3.106`）。`192.168.3.100` 是当前主力联调机。

### 4.2A 当前验收口径（面向值班排障闭环）

- `Sessions` 入口是 incident queue / diagnosis workbench，不是聊天记录或大盘杂烩；首屏先看**结论、风险、下一步、状态、证据**。
- `Executions` 入口是 approval / run queue，不是命令输出列表；首屏先看**动作、原因、风险、审批、结果、观察建议**。
- Tool-plan 走 evidence-first：优先 `metrics / observability / delivery / knowledge` 取证，只有证据不足、需要修复动作、或明确要求上机时，才升级到 `execution / SSH`。
- shared-lab 质量门以共享机托管环境为准，本地未显式配置 token 不应阻塞 `shared smoke / live / profile`；`logs / observability / delivery` 继续作为只读证据路径，而不是写入面。

### 4.3 当前 5 条主线（来自 current_high_priority_workstreams.md）

| 主线 | 状态 | 说明 |
|------|------|------|
| **主线1：Connector 创建与探测可信化** | 进行中 | SSH/VictoriaMetrics/VictoriaLogs 创建流必须可信，测试连接无假阳性；`live-validate.sh` 已包含 VM/VL 联调验证 |
| **主线2：Setup & Runtime Checks E2E 质量门** | 进行中 | `/setup` 是首次安装 wizard（4步），`/runtime-checks` 是运行体检（5区块）；不能重叠 |
| **主线3：192.168.3.100 首跑与联调闭环** | 进行中 | 重置验收标准已明确；`deploy_team_shared.sh` 完整链路 |
| **主线4：GitHub 首次基线与 CI/CD 边界** | **技术 gate 基本完成** | 决策：GitHub-first，先本地，CF Pages 做 demo 预览；当前无工程阻塞，但仍待 branch policy / rotation window / baseline sign-off |
| **主线5：SSH 凭据托管** | **Phase 0~4 已落地** | `encrypted_secrets` 表、API、`/ops?tab=secrets` 入口、ssh_native connector runtime 接入均已实现 |

**补充约束（2026-04-02 优先级收敛）：**
- 外部系统接入：SSH / VictoriaMetrics / VictoriaLogs 是**一等对象**
- Prometheus / JumpServer / MCP 是**次级**，不争抢模板、验证、控制面资源

### 4.4 近期不应驱动优先级的内容

- 多租户 / LDAP / SCIM / OIDC 企业级能力（`92-enterprise-platform-next-phase-topics.md`）
- 多层记忆系统 L1/L2/L3（`91-roadmap-post-mvp.md`）
- MCP 生态兼容（`91-roadmap-post-mvp.md`）
- 自我升级 / 自我扩展（`90-design-self-evolving-platform.md`）
- Bootstrap Slimming（WS4，状态不明，不应与当前前端打磨争资源）

这些能力可以继续保留为 post-MVP / next-phase 参考，但不应反向改写当前“值班排障闭环助手”的 scope。

---

## 5. 前端打磨优先级

> **信息来源**：`specs/00-frontend-ux-spec-audit-2026-03-29.md`（主）+ `docs/reports/frontend-priority-backlog-2026-03-29.md`（补）  
> **分层原则**：按"影响试点可用性"优先，然后按"对象边界叙事"，最后按"体验完善"

### Tier 1：试点运行前必须修复（阻塞质量门）

这些问题在首跑 / 试点演示中会直接暴露，或会误导操作员：

| # | 问题 | 页面 | 关键文件 |
|---|------|------|----------|
| T1-1 | Login 默认显示 break-glass token 路径（应降级为次选） | `/login` | `LoginView.tsx` |
| T1-2 | `login.redirectNotice` i18n key 损坏 | `/login` | `LoginView.tsx` |
| T1-3 | Ops page 是配置面感觉而非修复控制台（和 spec 正好相反） | `/ops` | `OpsActionView.tsx` |
| T1-4 | `/logs` 路由存在但页面实现缺失 | `/logs` | `App.tsx`、`pages/logs/` |
| T1-5 | Executions 审批缺少 reason/note 输入（governance 核心）| `/executions/{id}` | `ExecutionActionBar.tsx` |
| T1-6 | Setup Step 3 把 provider setup 和 model binding 混在一起 | `/setup` | `SetupSmokeView.tsx` |

### Tier 2：对象边界叙事修正（影响操作员心智模型）

这些问题会让操作员对"谁管什么"产生持续困惑，会在试点中积累负反馈：

| # | 问题 | 页面 | 关键文件 |
|---|------|------|----------|
| T2-1 | `AuthProvidersPage` 缺少 `local_password` provider 管理 | `/identity/auth-providers` | `AuthProvidersPage.tsx` |
| T2-2 | Automations / Triggers IA 分裂：Trigger 仍是独立顶级页 | `/automations`、`/triggers` | `AutomationsPage.tsx`、`TriggersPage.tsx` |
| T2-3 | Channels `usages` / `capabilities` 仍是 CSV 文本输入 | `/channels` | `ChannelsPage.tsx` |
| T2-4 | Setup Step 4 `Usages` 是文本而非结构化 multi-select | `/setup` | `SetupSmokeView.tsx` |
| T2-5 | People 页面叙事、导航位置、输入模型三者不一致 | `/identity/people` | `PeoplePage.tsx`、`navigation.tsx` |
| T2-6 | Message Templates 仍用旧 `msg template` 叙事而非 Notification Template | `/msg-templates` | `MsgTemplatesPage.tsx` |
| T2-7 | Templates lifecycle 混用 `enabled` 和 `status` | `/msg-templates` | `MsgTemplatesPage.tsx` |
| T2-8 | Triggers `template_id` 仍是自由文本 | `/triggers` | `TriggersPage.tsx` |
| T2-9 | Command modification input 常驻显示（应折叠在触发后） | `/executions/{id}` | `ExecutionActionBar.tsx` |

### Tier 3：诊断与治理信号完善（影响操作效率）

这些问题让操作员完成任务更费力，但不会阻塞主链路：

| # | 问题 | 页面 | 关键文件 |
|---|------|------|----------|
| T3-1 | Dashboard 执行队列面板是 recent list，不是真实 pending queue | `/` | `DashboardView.tsx` |
| T3-2 | Dashboard 关键指标依赖当前列表窗口而非稳定聚合 | `/` | `DashboardView.tsx`、`api/ops.ts` |
| T3-3 | Inbox failed load 退化为误导性空状态 | `/inbox` | `InboxPage.tsx` |
| T3-4 | Outbox event id 显示太弱，难以 troubleshoot | `/outbox` | `OutboxConsole.tsx` |
| T3-5 | Outbox error detail 缺少展开/面板 | `/outbox` | `OutboxConsole.tsx` |
| T3-6 | Templates 缺少 `variable_schema` 结构化编辑 | `/msg-templates` | `MsgTemplatesPage.tsx` |
| T3-7 | Templates `usage_refs` 未展示 | `/msg-templates` | `MsgTemplatesPage.tsx` |
| T3-8 | Skills list 缺少 `governance.execution_policy` 显示 | `/skills` | `SkillsList.tsx` |
| T3-9 | Skills 缺 `deprecated` / `archived` 状态 | `/skills` | `SkillsList.tsx`、`SkillDetail.tsx` |
| T3-10 | Connectors detail 对 scope/permission boundary 表达弱 | `/connectors/{id}` | `ConnectorDetail.tsx` |
| T3-11 | Session `session_id` 可见性弱，不利于 debug | `/sessions`、`/sessions/{id}` | `SessionDetail.tsx` |

### Tier 4：全局 Shell 与 Docs 完善（体验质量）

| # | 问题 | 关键文件 |
|---|------|----------|
| T4-1 | Global search 只索引 4 个文档，截断激进，只显示 3 条结果 | `GlobalSearch.tsx` |
| T4-2 | Mobile shell 隐藏 Theme、Language、Docs、Search | `AppLayout.tsx`、`navigation.tsx` |
| T4-3 | Docs dropdown 和 DocsView 结构不一致 | `navigation.tsx`、`DocsView.tsx` |
| T4-4 | `/docs` 跳过 landing page 直接进 user guide | `DocsView.tsx` |
| T4-5 | Breadcrumbs 降级时暴露原始 slug | `Breadcrumbs.tsx` |
| T4-6 | Chat 页面仍用 "Terminal Chat / 终端对话" 旧叙事 | `ChatPage.tsx` |
| T4-7 | Users 缺少 MFA / 安全态势信号 | `UsersPage.tsx` |
| T4-8 | Org 页面 `default_locale` / `default_timezone` 等旧字段未更新 | `OrgPage.tsx` |
| T4-9 | Identity navigation 缺少关键子模块直接入口 | `navigation.tsx`、`IdentityOverview.tsx` |
| T4-10 | Setup runtime checks 仍是 Telegram-first 文案 | `SetupSmokeView.tsx` |

---

## 6. 建议下一步执行顺序

### Phase A：试点闭环 + 主线收口（立即）

**目标**：确保 `192.168.3.100` 能跑完完整首跑，前端不出现卡住操作员的 P1 问题。

1. **Connector 创建流可信化**（主线1）：对照 `2026-03-27-connector-ux-field-model-design.md`，确认 SSH/VM/VL 创建 UX 符合设计（模板选择→最少字段→测试→保存）
2. **修复 T1-4：实现 LogsPage 或 guard 路由**（低成本，避免 404 尴尬）
3. **修复 T1-3：Ops page 框架改为修复/诊断优先**（按 `40-web-console-ops-console.md` 的 IA 结构）
4. **修复 T1-1 + T1-2：Login 默认路径和 i18n key**（低成本，高可见度）
5. **Setup E2E 验收**（主线2）：走完 4 步 wizard，确认 `/runtime-checks` 5 区块正确

### Phase B：对象边界叙事修正（试点开始后、稳定后）

**目标**：消除操作员心智模型混乱，对齐 IA 规范。

6. **T1-6 + T2-1：Setup Step 3 拆分 provider/model-binding；补 `local_password` 管理**
7. **T2-2：Automations / Triggers IA 合并**（Trigger 作为 Automation 子结构展示）
8. **T2-3 + T2-4：Channels usages/capabilities 改结构化控件**
9. **T2-6 + T2-7：Message Templates 叙事和生命周期模型统一**
10. **T1-5 + T2-9：Executions 审批加 reason/note；modification input 改为折叠**

### Phase C：诊断信号完善（持续迭代）

11. T3-1 + T3-2：Dashboard 真实 queue + 稳定聚合指标
12. T3-3 + T3-4 + T3-5：Inbox / Outbox 错误状态和诊断改善
13. T3-6 + T3-7：Templates variable_schema 结构化编辑
14. T3-8 + T3-9 + T3-10 + T3-11：Skills、Connectors、Sessions 治理信号补全

### Phase D：全局 Shell 与 Discovery（设计打磨轮次）

15. T4-1：Global search 扩大索引、改善结果深度
16. T4-2：Mobile shell 保留关键控件
17. T4-3 + T4-4：Docs 一致性和 landing page
18. T4-5 ~ T4-10：文案、breadcrumb、navigation 小修

---

## 7. 文档治理建议

### 7.1 立即动作

- [x] 在 `tars_prd.md` 头部 §0 加"当前适用范围"说明，§8 迭代规划加 `[POST-MVP-ONLY]` 口径
- [x] 在 `current_high_priority_workstreams.md` 更新：主线4 GitHub 技术 gate 状态已明确，主线5 SSH 凭据主链已落地
- [x] 在 `tars_dev_tracker.md` 补记 2026-04-02 至今的主要进展

### 7.2 后续建议

- `tars_dev_tasks.md`：加 header 说明"本文档已降为历史 WBS，主线以 workstreams 为准"
- `tars_frontend_tasks.md`：加 header 说明"FE-13~29 全部完成，前端欠账参见 `00-frontend-ux-spec-audit-2026-03-29.md`"
- `2026-04-08-ssh-credential-custody-design.md` 关键 API / schema 建议归并入 `tars_technical_design.md` §15 或独立 §16

### 7.3 无需动作的文档

所有 `archive` 类文档：保留在 `docs/reports/` 和 `docs/superpowers/plans/` 中，不删除，不主动引用。

---

## 8. 附录：Review 证据

### 8.1 specs/ 文件总清单（56 个）

```
specs/00-frontend-module-ui-ux-convergence-template.md
specs/00-frontend-ux-spec-audit-2026-03-29.md
specs/00-nav-page-to-spec-map.md
specs/00-spec-four-part-template.md
specs/00-specs-index.md
specs/10-platform-component-governance.md
specs/10-platform-components.md
specs/10-platform-components.zh-CN.md
specs/10-platform-dependency-compatibility.md
specs/10-platform-dependency-compatibility.zh-CN.md
specs/10-platform-object-boundaries-and-ia.md
specs/20-component-agent-roles.md
specs/20-component-audit.md
specs/20-component-automations.md
specs/20-component-channels-and-web-chat.md
specs/20-component-connectors.md
specs/20-component-extensions.md
specs/20-component-identity-access.md
specs/20-component-identity-access.zh-CN.md
specs/20-component-knowledge.md
specs/20-component-logging.md
specs/20-component-notification-templates.md
specs/20-component-observability.md
specs/20-component-org.md
specs/20-component-outbox.md
specs/20-component-providers-and-agent-role-binding.md
specs/20-component-skills.md
specs/30-strategy-async-eventing.md
specs/30-strategy-authorization-granularity.md
specs/30-strategy-automated-testing.md
specs/30-strategy-automations-and-triggers.md
specs/30-strategy-command-authorization.md
specs/30-strategy-desensitization.md
specs/30-strategy-platform-config-and-automation.md
specs/30-strategy-platform-config-and-automation.zh-CN.md
specs/30-strategy-platform-config-bundles.md
specs/30-strategy-reasoning-prompt-injection.md
specs/30-strategy-third-party-integration.md
specs/40-ux-design-system.md
specs/40-ux-frontend-optimization-workflow.md
specs/40-ux-telegram.md
specs/40-ux-unified-list-bulk.md
specs/40-web-console-chat-workbench.md
specs/40-web-console-executions-workbench.md
specs/40-web-console-governance-vs-ops.md
specs/40-web-console-inbox-workbench.md
specs/40-web-console-ops-console.md
specs/40-web-console-pages.md
specs/40-web-console-runtime-dashboard.md
specs/40-web-console-sessions-workbench.md
specs/40-web-console-setup-and-ops.md
specs/40-web-console-setup-workbench.md
specs/40-web-console.md
specs/90-design-self-evolving-platform.md
specs/90-design-tool-plan-diagnosis.md
specs/91-roadmap-post-mvp.md
specs/92-enterprise-platform-next-phase-topics.md
specs/README.md
specs/2026-03-27-connector-ux-field-model-design.md
specs/2026-03-27-ops-ia-refactor-design.md
specs/2026-03-27-specs-reorg-design.md
```

### 8.2 分类汇总

| 分类 | 数量 | 说明 |
|------|------|------|
| keep-now | ~70 | 当前主线和设计基线 |
| merge | 2 | SSH 凭据设计文档、tars_frontend_tasks.md |
| park | 6 | tars_prd.md §8+、91-roadmap、90-self-evolving、40-ux-telegram、five-priority WS4/5、tars_dev_tasks.md |
| archive | ~12 | 已完成的执行计划、已完成的研究报告、已完成的安全扫描 |
