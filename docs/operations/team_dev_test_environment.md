# TARS 团队开发与测试环境手册

> 适用时间点：2026-04-07  
> 目的：让团队成员在不依赖口口相传的情况下，快速对齐当前代码进度、共享测试环境、部署方式和联调基线。  
> 当前默认本地开发测试主机：`192.168.3.100`（AMD64）  
> 配套共享配置包：[../../deploy/team-shared](../../deploy/team-shared)
> 当前共享 lab 实操手册：[shared_lab_192.168.3.100.md](./shared_lab_192.168.3.100.md)

## 1. 当前进度判断

截至当前仓库状态，TARS 已经不只是“MVP 主链路可跑”，而是进入了“试点可持续使用 + 平台化能力开始成形”的阶段。

当前已经稳定存在并可验证的能力：

- Web Console、Setup、Runtime Checks、Sessions、Executions、Outbox、Audit、Knowledge、Connectors 页面
- Web Console、Setup、Runtime Checks、Sessions、Executions、Outbox、Audit、Knowledge、Connectors、Automations 页面
- Telegram 对话、告警诊断、审批、执行、结果回传
- SSH 执行、verification、知识沉淀、审计留痕
- Provider Registry：主模型 + 辅助模型、多协议、模型列表探测、可用性检查
- Connector Registry：发现、列表、详情、导出、启停、Prometheus/VictoriaMetrics runtime smoke
- Tool-plan 第一版：`planner -> execute tool steps -> final summarizer`
- `tool_plan / attachments` 已持久化到 Postgres session 主路径，并能在 Session Detail 查看
- 控制面：authorization、approval routing、reasoning prompts、desensitization、providers、connectors、secrets inventory、connector templates
- 统一列表协议与批量操作底座，已覆盖 `sessions / executions / outbox / audit / knowledge`
- Extensions Center：`skill_bundle` candidate 的 generate / validate / review / import，且 candidate 可跨重启保留

和现有跟踪文档相比，代码里的真实进度已经更靠前，尤其是这几块：

- Secret Store 控制面已经落地
- Connector Templates 已经落地
- Dashboard Health 与平台硬化接口已经落地
- JumpServer 官方 execution connector 已完成正常 workflow 样本验收，可走 `diagnosis -> pending_approval -> approve -> execute -> verify -> resolved`

## 2. 当前最值得做的 5 个开发方向

### 2.1 Tool-plan 驱动诊断与媒体结果

这是当前最高优先级。第一版已落地，下一步目标不是回退到固定 enrich，而是继续增强 “LLM 先判断，再决定调用 `VictoriaMetrics / Prometheus / APM / JumpServer`”。

优先补：

- `tool_plan`
- `metrics.query_range`
- 图片/文件附件协议
- 监控优先、执行后置的决策范式

### 2.2 Connector 生命周期补完整

当前已有 discovery、enable/disable、export、运行时 smoke。下一步应该补：

- 版本升级
- 版本回滚
- 生命周期历史
- 健康历史持久化
- 导入兼容校验

### 2.3 外部系统接入继续平台化

后续不能只依赖 SSH。要继续把“查询监控/变更/执行”的统一模型做实，优先顺序建议：

1. `Prometheus / VictoriaMetrics`
2. `JumpServer`
3. `APM / Tracing / Logging`
4. `Git / CI/CD / 发布系统`

### 2.4 MCP / Skill Source 做成可导入的平台能力

这块已经进了连接器规范，但运行时还没完整落地。下一阶段要补：

- 外部源注册
- package 导入导出
- 版本兼容与升级
- marketplace 兼容的 source registry

### 2.5 平台化硬化

优先做这些不花哨但很值钱的项：

- secret 管理与轮换
- provider / connector 健康历史
- 模板 / 预设
- 审计查询与导出
- 运维与发布基线固化

## 3. 本地开发基线

### 3.1 必备依赖

- Go toolchain
- Node.js + npm
- PostgreSQL
- 可选：SQLite 向量库文件路径
- 可选：本地模型服务或可访问的模型网关

### 3.2 本地校验命令

当前推荐的分层入口如下：

| 层级 | 命令 | 用途 | 依赖 |
|------|------|------|------|
| `L0` | `make pre-check` | Go compile + OpenAPI 快速预检 | 无外部依赖 |
| `L1` | `make check-mvp` | 标准本地回归 | Go / Ruby / Node |
| `L1` | `make full-check` | 标准本地回归 + 多架构部署静态回归 + linux/amd64,linux/arm64 交叉编译 | Go / Ruby / Node |
| `L2` | `make web-smoke` | Playwright 控制面 smoke | 目标控制面 URL + Playwright browser |
| `L3` | `make smoke-remote` | shared readiness + hygiene | SSH + ops token |
| `L3` | `make live-validate` | shared tool-plan live validation | ops token |
| `L3` | `make deploy` | build -> deploy -> smoke -> live validate 完整闭环 | SSH + shared env |
| `L4` | `make live-validate-auth` / `make live-validate-extensions` / `bash scripts/run_demo_smoke.sh` | 高成本专项验收 | 会改变共享环境状态 |

```sh
make pre-check
make full-check
make deploy-sync
make smoke-remote
make live-validate
```

当前仓库在本机执行 `bash scripts/check_mvp.sh` 为全绿。

GitHub Actions 只承担 L0 / L1 / L2；需要共享主机、root SSH 或 live connector 的校验仍放在本地或受控环境。

## 4. 本地开发测试环境

### 4.1 本地开发测试机

- 主机：`192.168.3.100`
- 架构：AMD64
- 统一入口：`http://192.168.3.100:8081`
- 当前同时承载 Web/Ops、Telegram long polling 配套入口、公开 discovery 和健康检查
- 共享/试点环境应开启 `TARS_RUNTIME_CONFIG_REQUIRE_POSTGRES=true`，确保 runtime config 不会在缺少 Postgres 时静默落到内存。

补充说明：

- 团队日常联调统一走 `8081`
- 如需验证远端本机探针，也可通过 SSH 到 `192.168.3.100` 后访问 `127.0.0.1:8081`
- 历史共享机 `192.168.3.106` 仅保留在旧记录里，不再作为新的默认目标
- 2026-03-20 历史共享环境实测：`jumpserver-main` 已作为默认执行 connector 纳入自动选择，但当时共享环境缺少 `connector/jumpserver-main/access_key` 和 `connector/jumpserver-main/secret_key`，所以 `/setup` 会显示 execution path 回退到 `ssh`

当前这台机器的真实部署落点不是默认示例里的 `$HOME/tars-dev`，而是：

- 远端根目录：`/data/tars-setup-lab`
- 二进制：`/data/tars-setup-lab/bin/tars-linux-amd64-dev`
- Web dist：`/data/tars-setup-lab/web-dist`
- 共享配置：`/data/tars-setup-lab/team-shared`

新加入开发测试的同学或 agent，优先看这份短手册：

- [shared_lab_192.168.3.100.md](./shared_lab_192.168.3.100.md)

### 4.2 当前共享模型基线

- 主模型：LM Studio
  - 地址：`http://192.168.1.132:1234`
  - 模型：`qwen/qwen3-4b-2507`
- 辅助模型：Google Gemini
  - 地址：`https://generativelanguage.googleapis.com/v1beta`
  - 模型：`gemini-flash-lite-latest`
- 额外保留的备用网关：DashScope OpenAI-compatible
  - 地址：`https://coding.dashscope.aliyuncs.com/v1`
  - 模型：`kimi-k2.5`

### 4.3 当前共享 Telegram 基线

- 测试 chat_id：`445308292`
- 当前采用 long polling，不依赖 Telegram webhook 公网入口

## 5. 团队共享配置包

团队共享配置包放在：

- [../../deploy/team-shared/README.md](../../deploy/team-shared/README.md)
- [../../deploy/team-shared/shared-test.env](../../deploy/team-shared/shared-test.env)
- [../../deploy/team-shared/providers.shared.yaml](../../deploy/team-shared/providers.shared.yaml)
- [../../deploy/team-shared/connectors.shared.yaml](../../deploy/team-shared/connectors.shared.yaml)
- [../../deploy/team-shared/access.shared.yaml](../../deploy/team-shared/access.shared.yaml)
- [../../deploy/team-shared/automations.shared.yaml](../../deploy/team-shared/automations.shared.yaml)
- [../../deploy/team-shared/secrets.shared.yaml](../../deploy/team-shared/secrets.shared.yaml)
- [../../deploy/team-shared/approvals.shared.yaml](../../deploy/team-shared/approvals.shared.yaml)
- [../../deploy/team-shared/authorization.shared.yaml](../../deploy/team-shared/authorization.shared.yaml)
- [../../deploy/team-shared/reasoning-prompts.shared.yaml](../../deploy/team-shared/reasoning-prompts.shared.yaml)
- [../../deploy/team-shared/desensitization.shared.yaml](../../deploy/team-shared/desensitization.shared.yaml)
- [../../deploy/team-shared/marketplace/index.yaml](../../deploy/team-shared/marketplace/index.yaml)
- [../../deploy/team-shared/marketplace/disk-space-incident.package.yaml](../../deploy/team-shared/marketplace/disk-space-incident.package.yaml)

这套配置的定位非常明确：

- 只用于团队内部开发 / 联调 / 试点测试
- 不是生产配置
- 这套配置现在按模板安全策略处理，仓库发布树里不应再保留真实内部凭据
- 历史上如果曾经存在真实内部地址或凭据，那些值必须视为需要轮换的旧材料，后续如果仓库访问范围扩大，必须先轮换

## 6. 推荐部署方式

### 6.1 编译

```sh
go build -o ./bin/tars ./cmd/tars
cd web && npm run build
```

### 6.2 同步到测试机

推荐使用显式 SSH 用户，不再把 root SSH 写成默认路径。部署脚本会默认使用远端用户的 `$HOME/tars-dev` 作为基准目录；如果你想固定目录，可以显式设置：

```sh
export TARS_REMOTE_HOST=192.168.3.100
export TARS_REMOTE_USER=<ssh-user>
export TARS_REMOTE_BASE_DIR=/home/<ssh-user>/tars-dev
# 可选：仅当你要临时覆盖共享 token 时才显式设置
# export TARS_OPS_API_TOKEN=<local-secret>
```

手工同步时推荐把共享包放到 `$TARS_REMOTE_BASE_DIR/team-shared/`：

```sh
ssh "$TARS_REMOTE_USER@$TARS_REMOTE_HOST" "mkdir -p '$TARS_REMOTE_BASE_DIR/team-shared' '$TARS_REMOTE_BASE_DIR/web-dist' '$TARS_REMOTE_BASE_DIR/bin'"
scp ./bin/tars-linux-amd64 "$TARS_REMOTE_USER@$TARS_REMOTE_HOST:$TARS_REMOTE_BASE_DIR/bin/tars-linux-amd64-dev"
scp ./deploy/team-shared/* "$TARS_REMOTE_USER@$TARS_REMOTE_HOST:$TARS_REMOTE_BASE_DIR/team-shared/"
scp -r ./web/dist/* "$TARS_REMOTE_USER@$TARS_REMOTE_HOST:$TARS_REMOTE_BASE_DIR/web-dist/"
```

共享环境当前还应额外包含：

- `TARS_ACCESS_CONFIG_PATH=$TARS_REMOTE_BASE_DIR/team-shared/access.shared.yaml`
- `TARS_AUTOMATIONS_CONFIG_PATH=$TARS_REMOTE_BASE_DIR/team-shared/automations.shared.yaml`

### 6.3 启动

```sh
source scripts/lib/shared_remote_service.sh
shared_remote_service_restart \
  "$TARS_REMOTE_USER@$TARS_REMOTE_HOST" \
  "$TARS_REMOTE_BASE_DIR/team-shared" \
  "$TARS_REMOTE_BASE_DIR/bin/tars-linux-amd64-dev" \
  "$TARS_REMOTE_BASE_DIR/team-shared/tars-dev.log"
```

不要简化成：

```sh
ssh "$TARS_REMOTE_USER@$TARS_REMOTE_HOST" "cd '$TARS_REMOTE_BASE_DIR' && nohup './bin/tars-linux-amd64-dev' &"
```

这个写法会绕过 `shared-test.env`，让服务退回默认 `./web/dist` 和默认配置路径。共享环境里的典型故障现象是：

- `GET /` 返回 `503 web_ui_unavailable`
- `GET /login`、`GET /setup` 无法加载前端
- 实际 `web-dist/index.html` 明明存在，但服务仍声称 `web ui index is not available`

部署或重启后，建议立即验证进程环境：

```sh
ssh "$TARS_REMOTE_USER@$TARS_REMOTE_HOST" '
  pid=$(ps -ef | awk "/tars-linux-amd64-dev/{print \$2; exit}")
  tr "\0" "\n" < /proc/$pid/environ | grep "^TARS_WEB_DIST_DIR="
'
```

如果没有看到共享环境里的 `TARS_WEB_DIST_DIR`，说明启动方式不对，应按上面的显式 `source` 命令重启。

或者直接使用自动化脚本：

```sh
make deploy-sync
make deploy
```

补充说明：

- `make deploy-sync`：只做构建、同步、重启，不自动验证。
- `make deploy`：默认串起 `deploy -> smoke-remote -> live-validate`，适合作为共享环境完整闭环入口。
- 部署脚本会自动探测远端 CPU 架构；如需手动覆盖，可传 `TARS_TARGET_ARCH=amd64|arm64`。
- 部署脚本现在会在远端保留上一版二进制副本 `$TARS_REMOTE_BASE_DIR/bin/tars-linux-<amd64|arm64>-dev.prev`，供失败时快速恢复。
- `192.168.3.100` 的部署脚本默认使用 `/data/tars-setup-lab`，并写入 `$TARS_REMOTE_BASE_DIR/team-shared/runtime_git_head` 供 `scripts/check-shared-lab.sh` 与 PR/head commit 对齐。

## 7. 部署后检查

### 7.1 基础接口

```sh
curl -H "Authorization: Bearer $TARS_OPS_API_TOKEN" http://192.168.3.100:8081/api/v1/setup/status
ssh "$TARS_REMOTE_USER@$TARS_REMOTE_HOST" 'curl http://127.0.0.1:8081/healthz'
ssh "$TARS_REMOTE_USER@$TARS_REMOTE_HOST" 'curl http://127.0.0.1:8081/readyz'
ssh "$TARS_REMOTE_USER@$TARS_REMOTE_HOST" 'curl http://127.0.0.1:8081/api/v1/platform/discovery'
```

### 7.2 一键回归

```sh
make full-check
make smoke-remote
make live-validate
```

### 7.3 人工体验链路

1. 打开 `http://192.168.3.100:8081/login`
2. 使用本地注入的 `TARS_OPS_API_TOKEN` 登录
3. 进入 `/runtime-checks` 检查 Telegram / model / providers / connectors
4. 触发 smoke alert 或直接给 Telegram 机器人发请求
5. 在 Telegram 完成审批
6. 回 Web 查看 session / execution / trace / audit / knowledge

### 7.4 平台控制面浏览器手工验收清单

建议每次部署完 Web 控制面后，至少按下面顺序走一轮浏览器验收：

1. 登录 `http://192.168.3.100:8081/login`，使用本地注入的 `TARS_OPS_API_TOKEN`
2. 打开 `http://192.168.3.100:8081/identity`
3. 验证 `Auth Providers` 页面：
   - 左侧列表可见已有条目
   - 点击 `New Provider` 后，右侧必须立即出现可编辑详情表单
   - `local_token` / OIDC 表单切换后，字段与提示文案正确变化
   - `client_secret_set / missing secret / local_token` 提示正确
   - create / edit / disable / enable 都要求填写 `operator_reason`
4. 打开 `http://192.168.3.100:8081/identity/people`
5. 验证 `People` 页面：
   - 点击 `New Person` 后，右侧立即出现表单
   - `channel_ids` 快捷选择区在条目较多时可折叠
   - create / edit / disable / enable 正常回显
   - 未填 `approval_target / oncall_schedule / channel_ids` 时出现风险提示
6. 打开 `http://192.168.3.100:8081/channels`
7. 验证 `Channels` 页面：
   - 点击 `New Channel` 后，右侧立即出现表单
   - `known users` 辅助区在条目较多时可折叠
   - create / edit / disable / enable 正常回显
   - 缺失 `target` 或无 `linked_users` 时提示正确
8. 打开 `http://192.168.3.100:8081/providers`
9. 验证 `Providers` 页面：
   - 点击 `New Provider` 后，右侧立即出现表单
   - provider entry 保存与 Primary / Assist 绑定保存是两个独立动作
   - `List models` / `Check availability` 可正常触发
    - fetched models 超过 3 条时有折叠入口
    - `missing secret`、`Primary bound`、`Assist bound` 提示正确
10. 打开 `http://192.168.3.100:8081/automations`
11. 验证 `Automation Jobs` 页面：
    - 列表页可正常展示现有 job、next run、last outcome、latest run
    - 可创建 `skill` job 和 `connector_capability` job
    - create / edit / enable / disable / run now 回显正常
    - 只读 capability 可成功完成 run now
    - 高风险 capability 或包含 `execution.run_command` 的 skill 不会自动穿透审批，而应显示 `blocked`
12. 若本轮新建了临时测试条目，验收结束后应立即清理；当前推荐做法是恢复 `$TARS_REMOTE_BASE_DIR/team-shared/automations.shared.yaml` 基线并删除对应 `.state.yaml`，再重启服务，避免长期污染共享环境

### 7.6 Playwright smoke（control-plane）

- 目录：`web/tests/control-plane.smoke.spec.ts`
- 配置：`web/playwright.config.ts`
- 默认 base URL：`http://192.168.3.100:8081`
- 默认 token：可留空；脚本会先尝试使用已归一化的 `TARS_PLAYWRIGHT_TOKEN / TARS_OPS_API_TOKEN`，若为空或仍是 placeholder，再通过 SSH 从共享机 `shared-test.env` 自动解析 canonical token
- 执行前首次安装浏览器：`cd web && npx playwright install chromium`
- 统一入口：`make web-smoke`
- 无头执行：`cd web && npm run test:smoke`
- 有头执行：`cd web && npm run test:smoke:headed`
- 当前覆盖：
  - `/identity`：Auth Provider create / edit / disable / enable
  - `/channels`：Channel create / edit / disable / enable
  - `/providers`：Provider create / edit / bind primary / bind assist / disable / enable
- 清理策略：每条用例前后都通过 `PUT /api/v1/config/auth` 与 `PUT /api/v1/config/providers` 回写配置，移除 `pw-smoke-*` 临时样本，并在必要时清空被临时样本占用的 `primary / assist` 绑定

### 7.5 历史共享环境 live 验收补充

- 2026-03-20 已确认 capability invoke 的 live 审批/拒绝语义：
  - `POST /api/v1/connectors/skill-source-main/capabilities/invoke`
  - 默认共享授权策略下返回 `202 pending_approval`
  - 临时加入 `hard_deny.mcp_skill: [source.sync]` 后返回 `403 denied`
  - 验收结束后已恢复远端共享目录内的 `authorization.shared.yaml`
- 2026-03-20 已确认真实 metrics 历史 + 图片附件链路可用：
  - 会话 `f95fbeed-84d8-4680-b2c8-df5848a23800`
  - `tool_plan` 执行 `metrics.query_range`
  - 返回 `metrics-range.json` 和 `metrics-range.png`
  - `executions=0`
  - 该样本是通过临时把 `prometheus-main` 指到 VM 兼容接口 `http://127.0.0.1:8428` 跑出的，验收结束后已恢复远端共享目录内的 `connectors.shared.yaml`
- 当前共享基线里：
  - `victoriametrics-main` 是启用中的真实 metrics connector
  - `victorialogs-main` 是启用中的真实 logs connector，默认指向共享机本地 `http://127.0.0.1:9428`
  - `prometheus-main` 为 disabled
  - `observability-main` 现在改为真实 `observability_http` connector，默认查询共享 `vmalert`：`http://127.0.0.1:8880/api/v1/alerts`
  - `delivery-main` 现在改为真实 `delivery_github` connector，默认查询公共 GitHub 仓库 `https://github.com/VictoriaMetrics/VictoriaMetrics.git`
- `jumpserver-main` 作为 execution 候选路径保留，但只有在真实执行成功并记录 execution health 后才会真正接管执行链；API probe 成功只会记为 `degraded`（`jumpserver API probe succeeded; execution not yet verified`），因此 workflow 会继续优先选择健康的 `ssh-main`
  - `TARS_SKILLS_CONFIG_PATH=$TARS_REMOTE_BASE_DIR/team-shared/skills.shared.yaml` 已启用，官方 skill 会以持久化 Skill Registry 方式加载，而不是仅依赖 marketplace-only 的内存态
  - `TARS_AUTOMATIONS_CONFIG_PATH=$TARS_REMOTE_BASE_DIR/team-shared/automations.shared.yaml` 可用于共享测试环境的 automation registry 基线
  - 因此如果 ad-hoc planner 样本仍点名 `prometheus-main`，并不代表 runtime 自身不可用；这是当前测试环境下的 connector 选择顺序特征，做正式 metrics 附件验收时需要注意
- 2026-03-20 晚些时候已修复上述选择顺序污染：disabled / incompatible connector 不再进入 tool-plan capability catalog，当前共享基线上 planner 已会优先选择启用中的 `victoriametrics-main`。新的远端样本 `a037a72c-a982-4c64-ba4a-6daab37daadb` 已验证 `metrics.query_range -> victoriametrics-main -> attachments(2) -> executions=0`。
- 新增脚本 [scripts/validate_tool_plan_live.sh](../../scripts/validate_tool_plan_live.sh)，用于快速验证：
  - `/api/v1/setup/status`
  - 显式 `metrics.query_range`
  - `skill-source-main` capability invoke 的 `202 pending_approval`
  - 默认自动验证临时 `hard_deny_mcp_skill` 下的 `403 denied`，并在验后恢复原 authorization config
  - 可选 smoke 样本（设置 `TARS_VALIDATE_RUN_SMOKE=1`），当前会优先使用 `smoke_defaults.hosts[0]` 作为 smoke host
- 新增共享 fixture 脚本 [scripts/seed_team_shared_fixtures.sh](../../scripts/seed_team_shared_fixtures.sh)，用于生成：
  - `$TARS_REMOTE_BASE_DIR/team-shared/fixtures/observability-main.log`
  - `$TARS_REMOTE_BASE_DIR/team-shared/fixtures/delivery-main-repo`
  建议每次同步 `connectors.shared.yaml` 后执行一次。

- 2026-03-21 起新的共享默认主路径已改为真实 API：
  - `observability.query` 默认走 `observability-main -> observability_http -> vmalert /api/v1/alerts|/api/v1/rules`
  - `delivery.query` 默认走 `delivery-main -> delivery_github -> GitHub commits API`
  - 两条 capability 都会把原始结果写入 session attachments，便于 Session Detail / Trace / Attachments 联调验收
  - `disk-space-incident` 已作为官方 skill 同步进入共享 Skill Registry；共享环境后续应优先通过 Skill Runtime 主路径触发，再退回 reasoning fallback
  - `metrics.query_range` 在磁盘上下文里会额外产出 `disk-space-analysis-*.json` 附件，写入当前 usage、增长速度和 forecast 风险
- 新增共享环境自动化脚本 [scripts/deploy_team_shared.sh](../../scripts/deploy_team_shared.sh)，默认会完成：
  - 自动探测远端架构并构建对应的 `linux/amd64` 或 `linux/arm64` 二进制
  - 构建 Web dist
  - 同步 shared configs / marketplace / fixtures 脚本
  - 同步 `shared-test.env` 时，会保留远端真实 secret / placeholder 覆盖项，同时继续把仓库中的 host/path 等模板化字段推进到共享机
  - 生成 shared fixtures
  - 重启远端服务
  - 运行 `validate_tool_plan_live.sh`
  - 脚本现在会显式等待远端 `127.0.0.1:8081/healthz` 就绪，并避免 `pkill -f` 误杀当前 SSH 会话
  - 本地 `TARS_OPS_API_TOKEN` 若为空或仍是 placeholder，会自动回退到远端 canonical `shared-test.env` 中的真实 token
  - 2026-03-21 已在历史共享机 `192.168.3.106` 实机跑通一轮；当前默认目标已切到 `192.168.3.100`：
    - `metrics.query_range -> victoriametrics-main`
    - `skill-source-main capability invoke -> 202 pending_approval`
    - `execution_component -> jumpserver-main`
  - 同日已补验增强版 live validation：
    - `403 denied`
    - smoke 样本 `546c2051-46c4-4125-9f6b-244df54dbf21` 成功产出 `tool_plan` step
- 当前共享环境已带本地 marketplace index：
  - `$TARS_REMOTE_BASE_DIR/team-shared/marketplace/index.yaml`
  - `disk-space-incident` 作为首个官方 incident skill 包
- 当前共享环境已带持久化 Skill Registry：
  - `$TARS_REMOTE_BASE_DIR/team-shared/skills.shared.yaml`
  - `$TARS_REMOTE_BASE_DIR/team-shared/automations.shared.yaml`
  - `/api/v1/skills`、`/api/v1/skills/{id}`、promote/rollback/import 等接口应在 `:8081` 正常可用

## 8. 前端功能开关与安全边界 (Feature Gating)

为了确保 GitHub Demo 与本地测试机的前端体验不因未配置外部系统而“点哪里哪错”，前端引入了 **Capability Gating** 机制：

- **主入口收紧**：在首次安装阶段，任意业务路径都会被统一收口到 `/setup`；初始化完成后，运行态检查集中到 `/runtime-checks`，而 `/channels`、`/providers` 等页面若未探测到有效配置（如 `telegram.configured=false` 或存在 `REPLACE_WITH_*` 占位符），对应功能将自动 **Hidden** 或 **Disabled**。
- **Fail Closed 原则**：未实现或依赖外部 Runtime Secret 的功能（如 Dex/OIDC、私有鉴权 VictoriaLogs 实例）默认标记为 `Coming soon` 或 `Requires configuration`；VictoriaLogs 的基础 `victorialogs_http` 查询/health 路径已作为一等 connector 暴露。
- **凭据注入规范**：
  - **No Hardcoded Defaults**：严禁在前端代码中使用 `ops-token` 字面量作为 fallback。
  - **SSH Credential Custody**：SSH 密码/私钥通过 `TARS_SECRET_CUSTODY_KEY` + PostgreSQL encrypted backend 托管，connector 只引用 `credential_id`；推理 Provider 等其它凭据仍优先使用 `secret_ref`。
  - **Custody Key Rotation**：轮换 `TARS_SECRET_CUSTODY_KEY` 时必须同时更新 `TARS_SECRET_CUSTODY_KEY_ID`，并重新上传受影响 SSH credential 材料；旧 `key_id` 的材料现在会 fail-closed，而不是静默继续使用。
  - **Break-glass Boundary**：`ops-token` 只允许走显式 approval endpoint 做紧急 approve/deny/request-context/modify-approve；不得读取 SSH password/private-key 明文，不得绕过 approval endpoint。
  - **Audit Requirement**：break-glass approval 必须在审计中标记 source=`ops-token`、actor、action、execution_id；共享环境验证时要一并核对这条审计证据。
- **导航分层**：不稳定或未对接的能力在侧边栏导航中会自动降级或隐藏。

## 9. 当前注意事项

- 当前共享包已按模板安全方向收敛；如果任何历史副本里还残留内部开发测试密钥，不能继续向更大范围扩散，必须先轮换
- Telegram Bot Token 目前仍由环境变量直接读取，暂未接入 sidecar secret ref
- JumpServer 当前已完成一条正式 workflow 验收样本，但 lifecycle history / rollback / health history 这类平台化能力仍需继续完善
- `docs/` 中部分通用手册仍偏“产品/部署模板视角”，这份手册更适合团队协作和真实联调
