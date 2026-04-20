# TARS

> TARS v1 baseline: 值班排障闭环助手。
> 当前仓库是可发布 baseline，源码、文档、CI 脚本和 `.example` 配置模板可提交；真实环境变量、共享机密钥、构建产物、运行数据和 agent state 不应进入 Git。

TARS 是一个面向值班 SRE / 运维团队的 AIOps MVP。

它当前最想解决的不是“做一个通用 AI 运维平台”，而是一个更窄、更痛的问题：

**高频告警进来后，尽快给出首个可行动判断。**

换句话说，TARS 的目标不是让团队“多一个 AI 分析面板”，而是让值班同学少在监控、SSH、Runbook 和群消息之间来回切。

## 当前定位

TARS 当前聚焦这条黄金路径：

`告警进入 -> 自动收集上下文 -> AI 给出诊断建议 -> 人工审批 -> 受控执行 -> 回传结果 -> 审计与知识沉淀`

其中真正要验证的价值是：

- 告警来了，能不能更快拿到“现在该做什么”
- 审批人能不能更快判断“这步敢不敢做”
- 团队会不会在真实事故里持续回来用

## 魔法时刻

理想中的 TARS 体验不是“AI 说了一段分析”，而是：

1. 告警进来后，30 秒左右给出首个可行动判断
2. 建议里带上证据、风险和下一步动作
3. 该审批时，审批人能直接看到可执行命令和原因
4. 执行完成后，结果和审计自动回传，而不是靠人补记录

这才是当前版本最该追的止痛药价值。

## 当前状态

- 代码形态：Go 后端 + React 19 Web Console + PostgreSQL 运行时配置与工作流状态
- 产品主线：`/sessions` 和 `/executions` 优先服务值班排障，而不是泛化 Dashboard
- 诊断策略：默认 evidence-first，先看 metrics / logs / traces / 发布变更证据，再进入 SSH 或执行审批
- 安全边界：高危命令必须走审批；真实 secret 只通过运行时注入或本地 ignored 文件维护
- 发布状态：本地质量门已固化，GitHub Actions 只跑 CI baseline，不访问共享机或真实部署凭据

## 一眼看懂

```text
Alert / Telegram / Web request
        |
        v
Session intake
        |
        v
Evidence-first tool plan
  metrics -> logs -> traces / observability -> release evidence -> SSH only when needed
        |
        v
Diagnosis, risk, next action
        |
        v
Approval-gated execution
        |
        v
Observation, audit trail, knowledge capture
```

TARS 的第一屏应该回答值班人真正关心的四件事：

- 当前判断是什么
- 证据在哪里
- 风险有多大
- 下一步该做什么

## 适合什么团队

当前版本最适合：

- 5-15 人的值班 SRE / 运维团队
- 已有 `VMAlert` 或类似告警源
- 已有 Telegram 值班群
- 高频处理标准化、重复出现的基础设施告警
- 接受“AI 先给建议，人再审批执行”的工作方式

## 当前最适合的场景

现阶段最值得先跑通的，不是所有告警，而是 3 类高频、重复、标准化的问题，例如：

- 服务不可用
- CPU / 内存 / 磁盘异常
- 实例 / 主机状态异常

对这类场景，TARS 的目标是缩短首个可行动判断时间，而不是替代整套运维平台。

## 当前 Connector 优先级

在外部系统接入这件事上，TARS 近期不再追求“大而全”。

当前近程会把 3 类对象列为一等 connector：

- `SSH`：作为受控执行对象收口连接、凭据、验证、主机范围和审计，而不是继续只靠环境变量和本地文件
- `VictoriaMetrics`：作为首要 metrics 证据源，优先服务“先看指标再判断下一步”
- `VictoriaLogs`：作为首要 logs 证据源，优先服务“先看日志再决定是否上机”

其他 connector 方向暂时下调优先级：

- `Prometheus` 保留兼容和导入能力，但不再是近程主打对象
- `JumpServer / MCP / 其他生态接入` 继续保留设计与兼容空间，但不抢当前产品和控制面的默认重心

## 当前不做什么

TARS 当前**不**想做这些事情：

- 不做一个“大而全”的可观测平台
- 不做通用 Agent 编排平台
- 不做自动绕过人工审批的执行闭环
- 不靠堆很多控制面能力来证明价值
- 不在 `SSH / VictoriaMetrics / VictoriaLogs` 之外同时铺太多 connector 方向

平台化、扩展性、治理能力都重要，但现在都应该退到支撑位置，而不是抢产品主叙事。

## 快速开始

### 先读什么

如果你想判断 TARS 现在适不适合你的团队，建议按这个顺序阅读：

1. 当前主线与开发入口：[docs/operations/current_high_priority_workstreams.md](docs/operations/current_high_priority_workstreams.md)
2. 项目文档索引：[project/README.md](project/README.md)
3. 产品目标：[project/tars_prd.md](project/tars_prd.md)
4. 技术方案：[project/tars_technical_design.md](project/tars_technical_design.md)
5. 试点交付包：[deploy/pilot/README.md](deploy/pilot/README.md)
6. MVP 完成清单：[docs/operations/mvp_completion_checklist.md](docs/operations/mvp_completion_checklist.md)
7. 用户手册：[docs/guides/user-guide.md](docs/guides/user-guide.md)
8. 部署手册：[docs/guides/deployment-guide.md](docs/guides/deployment-guide.md)

### 环境要求

- Go `1.25`
- Node.js `20.19+` 或 `22.12+`
- npm
- Ruby，用于 OpenAPI 校验脚本
- Docker，可选，仅在本地容器或共享部署链路中需要

### 干净 checkout 后的最小验证

```sh
git clone https://github.com/evilgaoshu/TARS.git
cd TARS

make web-install
make secret-scan
make pre-check
make check-mvp
```

`make check-mvp` 会执行 Go tests、core coverage、Go build、OpenAPI 校验、Web lint 和 Web build。

如果你刚 clone，先跑 `make web-install`，因为 `check-mvp` 假设 `web/node_modules` 已存在。

### 常用质量门

```sh
make web-install
make secret-scan
make pre-check
make security-regression
make check-mvp
cd web && npm run test:unit
make static-demo-build
```

推荐执行顺序：

- `make secret-scan`：扫描 publishable non-test tree，确保没有真实密钥进入可提交集合
- `make pre-check`：L0 快速预检，适合每次改动后立即运行
- `make security-regression`：权限、审批、token、越权访问专项回归
- `make check-mvp`：L1 MVP 严格门禁，合并或 push 前必跑
- `cd web && npm run test:unit`：完整前端单测
- `make static-demo-build`：静态演示构建，适合 GitHub Pages / demo artifact 验证
- `make full-check`：L1 标准本地回归，包含 `check_mvp` 和 `linux/amd64` 交叉编译

### 共享环境验证

共享测试机默认是 `192.168.3.100`。这些命令会访问远端服务或共享机：

```sh
make deploy-sync
make deploy
make smoke-remote
make live-validate
make live-validate-smoke
make web-smoke
```

- `make deploy-sync`：仅部署到共享环境，不自动验证
- `make deploy`：共享环境完整闭环，默认串起 `deploy -> smoke-remote -> live-validate`
- `make web-smoke`：Playwright 控制面 smoke，默认指向本地开发测试机 `http://192.168.3.100:8081`
- `make smoke-remote`：共享环境 health / readiness / hygiene
- `make live-validate`：tool-plan、metrics、approval、deny、observability、delivery 的 live validation

共享环境 token 不应该写进仓库。脚本会优先使用当前 shell 的显式 token；本地没有 token 时，符合条件的共享机脚本会从远端 canonical env 读取。详细说明见 [docs/operations/shared_lab_192.168.3.100.md](docs/operations/shared_lab_192.168.3.100.md)。

## 配置与 Secret 边界

仓库只提交模板和占位值：

- `configs/*.example.yaml`
- `deploy/pilot/*.example`
- `deploy/team-shared/shared-test.env.example`
- `deploy/team-shared/secrets.shared.yaml.example`

不要提交这些内容：

- `.env`、`.env.*`
- `deploy/team-shared/shared-test.env`
- `deploy/team-shared/secrets.shared.yaml`
- 私钥、真实 API key、Bot token、Ops token、数据库密码
- `bin/`、`dist/`、`web/dist/`、`web/node_modules/`、`data/`
- agent state，例如 `.claude/`、`.codex-tmp/`、`.gemini/`、`.playwright-cli/`、`.superpowers/`、`.alma/`

首次配置请从 `.example` 复制到本地 ignored 文件，再填入真实值。不要反向把本地真实配置覆盖回仓库模板。

## 当前已经具备的核心能力

- 告警和 Telegram 对话都可以进入统一 session
- AI 可生成 diagnosis 和 command candidate
- 命令执行走人工审批边界，支持审计和结果回传
- `sessions / executions / audit / knowledge / outbox` 已统一分页协议
- `sessions / executions` 已优先展示结论、风险、下一步与通知原因
- 试点交付包已包含 `run_golden_path_replay.sh` 和官方 replay fixture

## Web Console 入口

当前 Web Console 的默认叙事已经收口到值班排障：

- `/sessions`：默认入口，作为 incident queue
- `/sessions/:id`：诊断首页，优先展示结论、证据、风险、下一步
- `/executions`：审批和执行队列，默认 triage 排序
- `/runtime-checks`：初始化完成后的运行体检
- `/runtime`：运行态 Dashboard，退到支撑面
- `/connectors`、`/ops`、`/identity`：后台治理和运行配置

## 仓库包含什么

当前仓库同时包含：

- 后端服务
- Web Console
- 部署资料
- 产品 / 技术文档

目录总览：

```text
api/                   OpenAPI 契约
cmd/tars/              服务启动入口
configs/               样例配置
deploy/                部署说明、docker 资源、Grafana 导入物
deploy/pilot/          试点交付包与 golden cases
docs/                  用户、管理员、参考、运维与报告文档
internal/api/          HTTP API、DTO、路由
internal/app/          组装与启动
internal/contracts/    模块间契约
internal/domain/       领域对象
internal/events/       dispatcher / timeout / GC worker
internal/foundation/   配置、日志、审计、指标等基础设施
internal/modules/      action / reasoning / workflow / knowledge / channel
internal/repo/         PostgreSQL / SQLiteVec 持久化
migrations/            数据库初始化脚本
project/               PRD、技术设计、任务与执行跟踪
scripts/               本地校验、初始化、辅助脚本
specs/                 平台规范、策略、设计与路线
web/                   Web Console 源码
```

## 当前关键入口

- 后端启动入口：[cmd/tars/main.go](cmd/tars/main.go)
- 应用组装：[internal/app/bootstrap.go](internal/app/bootstrap.go)
- HTTP 路由：[internal/api/http/routes.go](internal/api/http/routes.go)
- 工作流核心：[internal/modules/workflow/service.go](internal/modules/workflow/service.go)
- Telegram 交互：[internal/api/http/telegram_handler.go](internal/api/http/telegram_handler.go)
- 平台发现接口：[internal/api/http/platform_handler.go](internal/api/http/platform_handler.go)
- 连接器注册表与 runtime 路由：[internal/api/http/connector_registry_handler.go](internal/api/http/connector_registry_handler.go)
- 知识服务：[internal/modules/knowledge/service.go](internal/modules/knowledge/service.go)

## 更多文档

- 文档索引：[docs/README.md](docs/README.md)
- 当前高优先级主线：[docs/operations/current_high_priority_workstreams.md](docs/operations/current_high_priority_workstreams.md)
- API 文档：[docs/reference/api-reference.md](docs/reference/api-reference.md)
- 管理员手册：[docs/guides/admin-guide.md](docs/guides/admin-guide.md)
- 团队开发与测试环境：[docs/operations/team_dev_test_environment.md](docs/operations/team_dev_test_environment.md)
- 团队共享开发测试包：[deploy/team-shared/README.md](deploy/team-shared/README.md)
- 共享环境历史记录入口：[docs/operations/records/README.md](docs/operations/records/README.md)
- Post-MVP 路线设计：[specs/91-roadmap-post-mvp.md](specs/91-roadmap-post-mvp.md)

## 开发约定

- `internal/modules/` 放业务运行时模块
- `internal/foundation/` 放跨模块基础设施
- `internal/repo/` 放具体持久化实现
- `docs/` 只放长期维护文档，不放临时草稿
- `specs/` 只放开发者 / 架构 / 平台规范文档
- `project/` 只放 PRD、技术设计、任务与执行跟踪
- `web/dist`、`web/node_modules`、运行时日志和数据库文件都不应提交

## GitHub 发布前检查

首次 push 或每次发布前，至少确认：

```sh
git status --short
make secret-scan
make security-regression
make check-mvp
cd web && npm run test:unit
```

GitHub Actions 只应依赖可公开的 CI 输入，不应访问共享机 SSH、真实 token 或部署环境。分支保护、required checks、密钥轮换窗口和首次 baseline 签字见 [docs/operations/github_migration_prep_runbook.md](docs/operations/github_migration_prep_runbook.md)。
