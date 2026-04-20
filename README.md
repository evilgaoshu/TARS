# TARS

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

如果你想判断 TARS 现在适不适合你的团队，建议按这个顺序阅读：

1. 当前主线与开发入口：[docs/operations/current_high_priority_workstreams.md](docs/operations/current_high_priority_workstreams.md)
2. 项目文档索引：[project/README.md](project/README.md)
3. 产品目标：[project/tars_prd.md](project/tars_prd.md)
4. 技术方案：[project/tars_technical_design.md](project/tars_technical_design.md)
5. 试点交付包：[deploy/pilot/README.md](deploy/pilot/README.md)
6. MVP 完成清单：[docs/operations/mvp_completion_checklist.md](docs/operations/mvp_completion_checklist.md)
7. 用户手册：[docs/guides/user-guide.md](docs/guides/user-guide.md)
8. 部署手册：[docs/guides/deployment-guide.md](docs/guides/deployment-guide.md)

本地常用命令：

```sh
make pre-check
make full-check
make deploy-sync
make smoke-remote
make live-validate
bash scripts/run_golden_path_replay.sh
```

推荐执行顺序：

- `make pre-check`：L0 快速预检，适合每次改动后立即运行
- `make full-check`：L1 标准本地回归，包含 `check_mvp` 和 `linux/amd64` 交叉编译
- `make deploy-sync`：仅部署到共享环境，不自动验证
- `make deploy`：共享环境完整闭环，默认串起 `deploy -> smoke-remote -> live-validate`
- `make web-smoke`：Playwright 控制面 smoke，默认指向本地开发测试机 `http://192.168.3.100:8081`

## 当前已经具备的核心能力

- 告警和 Telegram 对话都可以进入统一 session
- AI 可生成 diagnosis 和 command candidate
- 命令执行走人工审批边界，支持审计和结果回传
- `sessions / executions / audit / knowledge / outbox` 已统一分页协议
- `sessions / executions` 已优先展示结论、风险、下一步与通知原因
- 试点交付包已包含 `run_golden_path_replay.sh` 和官方 replay fixture

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
