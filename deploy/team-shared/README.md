# TARS 团队共享开发测试包

> 这套配置用于团队内部开发、联调、试点测试。  
> 仓库中只保留 template-safe 的占位值，不包含可直接复用的 live secrets。  
> 如果准备接入真实环境，请先在本地或密钥系统中注入实际凭据。
>
> 当前 `192.168.3.100` 共享开发测试机的真实部署落点、启动方式、reset 方法，见：
> [../../docs/operations/shared_lab_192.168.3.100.md](../../docs/operations/shared_lab_192.168.3.100.md)

## 文件说明

- [shared-test.env](shared-test.env)
  - 当前共享测试环境的主环境变量
- [providers.shared.yaml](providers.shared.yaml)
  - 主模型 / 辅助模型 / 备用模型 Provider Registry
- [connectors.shared.yaml](connectors.shared.yaml)
  - Prometheus / VictoriaMetrics / JumpServer 的 Connector Registry
- [access.shared.yaml](access.shared.yaml)
  - 共享 access/identity/channel registry，包含 break-glass auth provider、基础 `local_password` 登录链、Dex OIDC 登录入口与已登记 channel；文件内仅保留占位凭据
- [dex.config.yaml](dex.config.yaml)
  - 共享测试环境 Dex OIDC 配置，占位 secret 与 password hash 需要部署时替换
- [skills.shared.yaml](skills.shared.yaml)
  - 当前共享测试环境的持久化 Skill Registry
- [automations.shared.yaml](automations.shared.yaml)
  - 当前共享测试环境的持久化 Automation Registry
- `extensions.state.yaml`
  - 当前共享测试环境的持久化 Extension Candidate 状态文件（部署后自动创建，不随仓库存放）
- [secrets.shared.yaml](secrets.shared.yaml)
  - sidecar secret store，占位值仅用于模板，不含真实 API key
- [approvals.shared.yaml](approvals.shared.yaml)
  - 当前共享测试审批路由
- [authorization.shared.yaml](authorization.shared.yaml)
  - 当前共享测试授权策略
- [reasoning-prompts.shared.yaml](reasoning-prompts.shared.yaml)
  - 当前共享测试 reasoning prompt
- [desensitization.shared.yaml](desensitization.shared.yaml)
  - 当前共享测试脱敏策略
- [marketplace/index.yaml](marketplace/index.yaml)
  - 当前共享测试环境本地 marketplace / skill index
- [marketplace/disk-space-incident.package.yaml](marketplace/disk-space-incident.package.yaml)
  - 官方 `disk-space-incident` skill 包

## 建议用法

### 本地开发

1. 复制 `shared-test.env` 到你自己的机器并按需改路径。
2. 保持 `providers.shared.yaml / connectors.shared.yaml / access.shared.yaml / approvals.shared.yaml` 与团队基线一致。
3. 如需切回自己的模型或机器人，只改你本地副本，不改仓库基线。
4. 运行 `deploy_team_shared.sh` 前，请显式设置 `TARS_REMOTE_USER`；`TARS_OPS_API_TOKEN` 可以显式覆盖，也可以留空让脚本在 SSH 可用时从共享机的 `shared-test.env` 自动解析。若本地环境变量只是 placeholder，脚本也会自动忽略并继续回退远端 canonical token。

### 共享测试机

推荐使用显式 SSH 用户，并让部署脚本按远端用户的 `$HOME/tars-dev` 推导目录。也可以显式覆盖：

```sh
export TARS_REMOTE_HOST=192.168.3.100
export TARS_REMOTE_USER=<ssh-user>
export TARS_REMOTE_BASE_DIR=/home/<ssh-user>/tars-dev
# 可选：显式覆盖共享 token；不设置时脚本会尝试从远端 shared-test.env 自动解析
# export TARS_OPS_API_TOKEN=<local-secret>
```

模板中的占位路径会在 `deploy_team_shared.sh` 同步时替换为实际远端目录。推荐目录形态：

- `REPLACE_WITH_REMOTE_SHARED_DIR/shared-test.env`
- `REPLACE_WITH_REMOTE_SHARED_DIR/providers.shared.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/connectors.shared.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/access.shared.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/dex.config.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/skills.shared.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/automations.shared.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/extensions.state.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/secrets.shared.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/approvals.shared.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/authorization.shared.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/reasoning-prompts.shared.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/desensitization.shared.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/marketplace/index.yaml`
- `REPLACE_WITH_REMOTE_SHARED_DIR/marketplace/disk-space-incident.package.yaml`

然后：

```sh
set -a
source REPLACE_WITH_REMOTE_SHARED_DIR/shared-test.env
set +a
REPLACE_WITH_REMOTE_BINARY
```

不要直接执行裸命令：

```sh
nohup ./bin/tars-linux-amd64-dev &
```

这会跳过 `shared-test.env`，导致共享环境最常见的故障之一：

- Web 首页返回 `503 web_ui_unavailable`
- 服务虽然存活，但前端入口、`/setup`、`/login` 全部打不开

共享测试环境里，“先 source env，再启动二进制”是硬要求，不是建议项。

当前建议把共享机 `Ops API token` 视为一份统一的 break-glass 凭据：

- 唯一事实来源：远端 `shared-test.env`
- 仓库模板只保留 placeholder
- `deploy_team_shared.sh / smoke-remote.sh / live-validate.sh / web-smoke.sh` 在未显式设置 `TARS_OPS_API_TOKEN` 时，会优先通过 SSH 读取远端这份共享 token

或者直接用自动化脚本：

```sh
bash ../../scripts/deploy_team_shared.sh
```

## 共享测试基线

- 测试机：`192.168.3.100`（AMD64）
- 统一入口（Web / Ops / healthz / readyz / discovery）：`http://192.168.3.100:8081`
- Dex OIDC：`http://192.168.3.100:15556/dex`
- Telegram 测试 chat_id：`445308292`
- Dex 测试账号：`tars-demo@example.com / REPLACE_WITH_DEX_PASSWORD`
- 本地密码测试账号：`shared-admin / REPLACE_WITH_LOCAL_PASSWORD`
- TOTP 测试 secret：`REPLACE_WITH_TOTP_SECRET`
- 主模型：`lmstudio-local / qwen/qwen3-4b-2507`
- 辅助模型：`gemini-backup / gemini-flash-lite-latest`
- 备用模型：`dashscope-kimi / kimi-k2.5`
- metrics 默认主路径：`victoriametrics-main`
- observability 默认主路径：`observability-main`（真实 `observability_http` runtime，读取共享 `vmalert` alerts/rules API）
- delivery 默认主路径：`delivery-main`（真实 `delivery_github` runtime，读取 GitHub commits API）
- execution 默认主路径：`jumpserver-main`（若 connector health 不健康或 secret 未配置，则自动回退 `ssh`）
- capability 验收连接器：`skill-source-main`
- Access Registry：`REPLACE_WITH_REMOTE_SHARED_DIR/access.shared.yaml`
- Skill Registry：`REPLACE_WITH_REMOTE_SHARED_DIR/skills.shared.yaml`
- Automation Registry：`REPLACE_WITH_REMOTE_SHARED_DIR/automations.shared.yaml`
- 本地 marketplace index：`REPLACE_WITH_REMOTE_SHARED_DIR/marketplace/index.yaml`
- 官方 incident skill 包：`disk-space-incident`

## Fixtures

共享测试环境建议在同步配置后再执行一次：

```sh
bash REPLACE_WITH_REMOTE_SHARED_DIR/seed_team_shared_fixtures.sh
```

对应仓库脚本是：

- [seed_team_shared_fixtures.sh](../../scripts/seed_team_shared_fixtures.sh)

它会创建：

- `REPLACE_WITH_REMOTE_SHARED_DIR/fixtures/observability-main.log`
- `REPLACE_WITH_REMOTE_SHARED_DIR/fixtures/delivery-main-repo`

说明：fixtures 仍保留给 fallback / 对照测试，但共享默认 live 验收现在优先使用：

- `http://127.0.0.1:8880/api/v1/alerts`
- `http://127.0.0.1:8880/api/v1/rules`
- `https://github.com/VictoriaMetrics/VictoriaMetrics.git`

## 共享环境自动化

建议优先用：

- [deploy_team_shared.sh](../../scripts/deploy_team_shared.sh)

### 前端功能开关与安全边界 (Feature Gating)

为了确保 Demo 与共享测试环境的安全性与可用性，前端实现了基于 `fetchSetupStatus` 的 **Feature Gating**：

- **默认关闭 (Fail Closed)**：未配置真实凭据的功能（如 Telegram、模型 Provider、OIDC）在 UI 中默认隐藏或禁用。
- **禁止硬编码 (No Hardcoded Secrets)**：前端代码中严禁出现 `ops-token` 等字面量 fallback；所有 Token 与 Key 必须由用户显式输入或通过环境变量注入。
- **状态感知**：
  - **Telegram**：若缺 Bot Token，显示 `Requires Telegram token`。
  - **Connectors**：SSH / VictoriaMetrics / VictoriaLogs 为一等公民；VictoriaLogs 默认可用 play-vmlogs 演示路径，私有鉴权实例必须通过 `secret_ref` 或私有注入配置。
  - **SSH 安全**：禁止把私钥/密码写入 manifest 或普通 YAML；必须先启用 `TARS_SECRET_CUSTODY_KEY`，通过 `/ops?tab=secrets` 的 SSH Credential Custody 写入加密后端，再在 SSH connector 中引用 `credential_id`。
- **模板化**：未实现的功能在导航中降级或标为 `Beta` / `Coming soon`。

默认会执行：

- 自动探测远端架构并构建对应的 `linux/amd64` 或 `linux/arm64` 二进制
- 构建 Web dist
- 同步 `deploy/team-shared/*.yaml`、marketplace 包和 fixtures 脚本
- 同步 `shared-test.env` 时，会保留远端真实 secret / placeholder 覆盖项，同时继续把仓库中的 host/path 等模板化字段推进到共享机
- 生成共享 fixtures
- 重启远端服务
- 运行 live validation

历史上曾在 `192.168.3.106` 实机跑通过；当前默认本地开发测试机已切到 `192.168.3.100`。当前脚本还额外包含：

- 远端 readiness 等待：`127.0.0.1:8081/healthz`
- 避免 `pkill -f` 误杀当前 SSH 会话的重启逻辑
- 本地 `TARS_OPS_API_TOKEN` 若为空或仍是 placeholder，会自动回退到远端 canonical `shared-test.env` 中的真实 token
- `shared-test.env` 会继续同步，但 placeholder / 空值字段会自动继承远端真实值，避免把共享机 canonical secret 冲回模板占位值
- 默认 live validation：
  - `metrics.query_range -> victoriametrics-main`
  - `skill-source-main capability invoke -> 202 pending_approval`
- [validate_tool_plan_live.sh](../../scripts/validate_tool_plan_live.sh) 现已支持：
  - 默认自动验证 capability 的 `202 pending_approval`
  - 默认自动验证 capability 的 `403 denied`（通过临时热更新 authorization config，验后自动恢复）
  - 可选 `TARS_VALIDATE_RUN_SMOKE=1`，验证 smoke 是否真正产出 `tool_plan` step

## 轮换建议

建议后续至少轮换这些项：

- Telegram Bot Token
- Gemini API Key
- DashScope API Key
- Ops API Token

这些示例名和占位值是为了降低团队内部联调门槛，不是长期安全策略。
当前仓库只保留模板和占位值，实际部署时请由本地配置或密钥管理系统注入真实凭据。
