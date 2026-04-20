# TARS 部署说明

> **适用范围**: 当前仓库的后端 MVP 部署与试点联调  
> **当前状态**: 代码运行时只读取环境变量；[configs/tars.example.yaml](../configs/tars.example.yaml) 目前是配置设计样例，不会被进程直接加载  
> **对应代码**: [internal/foundation/config/config.go](../internal/foundation/config/config.go)
> **配套资料**: [试点交付包](pilot/README.md) / [团队共享开发测试包](team-shared/README.md) / [团队开发与测试环境手册](../docs/operations/team_dev_test_environment.md) / [192.168.3.100 共享 Lab 运行手册](../docs/operations/shared_lab_192.168.3.100.md) / [试点 Runbook](../docs/operations/pilot_runbook.md) / [发布检查清单](../docs/operations/release_checklist.md) / [MVP 完成清单](../docs/operations/mvp_completion_checklist.md) / [可观测性面板建议](../docs/operations/observability_dashboard.md) / [Grafana 导入物](grafana/tars-mvp-dashboard.json)
> **授权策略演进**: [命令与能力授权策略](../specs/30-strategy-command-authorization.md) / [vNext 授权配置样例](../configs/authorization_policy.vnext.example.yaml)

## 1. 部署目标

当前 MVP 后端的目标链路是：

`VMAlert -> TARS -> AI diagnosis -> Telegram 审批 -> SSH 执行 -> 结果回传 -> Knowledge ingest`

当前 Web Console 的最小体验链路是：

`首次打开 /setup -> 完成管理员/登录方式/主模型/默认通知渠道 -> 登录 Web -> 进入 /runtime-checks -> 触发 Smoke Alert -> Telegram 收到诊断/审批 -> Web/TG 查看结果`

如果只做本地或实验 smoke，可以省略部分真实外部依赖并使用 fallback。
如果要进入试点，下面标记为 `试点必需` 的配置都需要提供。

当前进程默认只监听一个 HTTP 入口：

- 统一入口：`TARS_SERVER_LISTEN`，同时承载 `healthz / readyz / metrics / platform/discovery / vmalert webhook / Telegram webhook`
- 当 `TARS_OPS_API_ENABLED=true` 时，同一入口还会暴露受保护的 Ops API，例如 `summary / setup/status / smoke/alerts / sessions / executions / outbox / reindex`

统一入口还会公开一个平台发现接口：

- `GET /api/v1/platform/discovery`

这个接口用于让外部系统、自动化工具或后续插件市场发现 TARS 当前支持的开放接入模式、连接器类型、Provider 协议和导入导出格式。详细规范见 [20-component-connectors.md](../specs/20-component-connectors.md)。

如果配置了 `TARS_WEB_DIST_DIR` 且目录下存在前端构建产物，统一入口还会直接服务 Web Console：

- `GET /login`
- `GET /setup`
- `GET /runtime-checks`
- `GET /sessions`
- `GET /sessions/:id`
- `GET /executions/:id`
- `GET /outbox`
- `GET /ops`

建议把统一入口放在 `127.0.0.1`、内网地址或 VPN 后面；如果必须对公网开放，请通过反向代理只暴露真正需要的 webhook / Web 页面路径，并保留 Bearer Token 或会话鉴权。

### 1.1 Setup Wizard 第一阶段说明

- 从本阶段开始，`/setup` 只承担“首次安装向导”职责。
- 如果数据库中 `setup_state.initialized=false`，访问 `/setup` 会直接进入 wizard，不要求先登录。
- wizard 会把 `首个管理员 / 本地密码登录方式 / 主模型 provider / 默认通知渠道` 写入运行时配置，并把初始化状态持久化到 Postgres。
- 初始化完成后，运行体检入口切到受保护的 `/runtime-checks`；后续 smoke 触发和运行态观察都在该页面进行。
- secret 仍不直接写入普通配置表。provider API key 等敏感值需要通过 `secret ref` 指向现有 secret store。

### 1.2 Setup Wizard 第二阶段说明

- setup provider step 现在不是“先存后测”，而是“先校验再完成”：
  - `base_url` 必须是合法 URL
  - `api_key_ref` 必须是 `secret://...`
  - 对应 secret 必须已存在于当前 secret store，且有值
  - provider connectivity / availability check 必须成功
- setup complete 只有在 provider check 成功后才允许提交。
- complete 后服务端会返回 `login_hint`，用于前端自动登录或跳转到带预填参数的 `/login`。
- 如果首装选择的是 `local_password`，且前端仍持有刚输入的管理员密码，Web 会优先自动完成一次登录；否则会跳到已预填用户名与 provider 的登录页。
- `connectors / auth providers / channels` 当前主写路径已经收口到 runtime DB；即使未配置相应 YAML path，系统也可从 Postgres 回灌恢复运行时状态。

## 2. 配置总表

### 2.1 核心服务与存储

| 环境变量 | 是否必需 | 示例 | 说明 |
|---|---|---|---|
| `TARS_SERVER_LISTEN` | 基础必需 | `:8081` | TARS 统一 HTTP 服务监听地址 |
| `TARS_SERVER_PUBLIC_BASE_URL` | 仅 webhook 模式必需 | `https://tars.example.com` | Telegram webhook / OIDC callback 对外可达地址 |
| `TARS_WEB_DIST_DIR` | Web Console 建议提供 | `/var/lib/tars/web-dist` | 前端构建产物目录；配置后统一 listener 可直接服务登录页、首次安装页与 Runtime Checks 页面 |
| `TARS_POSTGRES_DSN` | 基础必需 | `postgres://tars:tars@postgres:5432/tars?sslmode=disable` | 主业务库，必须可写 |
| `TARS_RUNTIME_CONFIG_REQUIRE_POSTGRES` | 试点/生产建议开启 | `true` | 防止 runtime config 静默降级到内存；未配置 Postgres 时启动失败 |
| `TARS_VECTOR_SQLITE_PATH` | 知识检索建议开启 | `/var/lib/tars/tars_vec.db` | 向量索引 SQLite 文件；不配时只走 lexical search |
| `TARS_LOG_LEVEL` | 可选 | `INFO` | 日志级别，建议 `INFO` 或 `DEBUG` |
| `TARS_OPS_API_ENABLED` | 试点建议开启 | `true` | 是否在统一 listener 上启用受保护的 Ops API |
| `TARS_OPS_API_TOKEN` | 试点建议开启 | `change-me` | 运维接口 Bearer Token |

说明补充：

- 虽然 wizard 支持未登录访问，但试点/生产环境仍然依赖 `TARS_POSTGRES_DSN` 来持久化 setup 状态与 runtime config 文档，并建议开启 `TARS_RUNTIME_CONFIG_REQUIRE_POSTGRES=true`。
- `/runtime-checks` 页面及其依赖的 `/api/v1/setup/status` 读写接口在初始化完成后仍受现有认证与权限控制。
- 当前仍然必须保留 env 的能力包括：`TARS_POSTGRES_DSN`、`TARS_RUNTIME_CONFIG_REQUIRE_POSTGRES`、监听地址、`TARS_SERVER_PUBLIC_BASE_URL`、`TARS_WEB_DIST_DIR`、secret backend、本地路径/spool/retention，以及可选 bootstrap admin 初始凭据。

说明：

- 当前向量检索实现使用本地 SQLite 向量索引文件和应用内稳定 embedding，不额外依赖外部 embedding provider
- 如果不配置 `TARS_VECTOR_SQLITE_PATH`，系统仍可运行，但知识检索只保留 lexical fallback

### 2.2 Telegram 渠道

| 环境变量 | 是否必需 | 示例 | 说明 |
|---|---|---|---|
| `TARS_TELEGRAM_BOT_TOKEN` | 试点必需 | `123456:ABCDEF...` | BotFather 创建机器人后得到 |
| `TARS_TELEGRAM_WEBHOOK_SECRET` | 仅 webhook 模式必需 | `4b8f4e...` | webhook 模式下用于校验 `X-Telegram-Bot-Api-Secret-Token` |
| `TARS_TELEGRAM_BASE_URL` | 可选 | `https://api.telegram.org` | 如走代理/网关可改成自定义地址 |
| `TARS_TELEGRAM_POLLING_ENABLED` | long polling 模式必需 | `true` | 开启后，TARS 会主动调用 Telegram `getUpdates` |
| `TARS_TELEGRAM_POLL_TIMEOUT` | long polling 建议提供 | `30s` | 单次 long polling 挂起时长 |
| `TARS_TELEGRAM_POLL_INTERVAL` | long polling 可选 | `1s` | polling 出错后的退避间隔 |

### 2.3 VMAlert / VictoriaMetrics

| 环境变量 | 是否必需 | 示例 | 说明 |
|---|---|---|---|
| `TARS_VMALERT_WEBHOOK_SECRET` | 试点建议开启 | `smoke` | 若配置则要求 `X-Tars-Signature` 完全匹配；留空时允许内网直连 smoke |
| `TARS_VM_BASE_URL` | 试点必需 | `http://vm.example.com/select/0/prometheus` | VictoriaMetrics 查询入口；不配时仅使用 stub metrics |
| `TARS_VM_TIMEOUT` | 可选 | `15s` | 查询超时 |

### 2.4 模型网关

| 环境变量 | 是否必需 | 示例 | 说明 |
|---|---|---|---|
| `TARS_MODEL_PROTOCOL` | 试点建议提供 | `openai_compatible` | 支持 `openai_compatible`、`anthropic`、`gemini`、`openrouter`、`ollama`、`lmstudio` |
| `TARS_MODEL_BASE_URL` | 试点必需 | `https://model-gateway.example.com/v1` | 模型接口根路径；不同协议会自动补齐 `/v1/chat/completions`、`/v1/messages` 或 `/api/chat` |
| `TARS_MODEL_API_KEY` | 按网关要求 | `sk-...` | 若网关要求鉴权则必须提供 |
| `TARS_MODEL_NAME` | 建议提供 | `gpt-4o-mini` | 默认 `gpt-4o-mini`，若网关要求显式模型名请配置 |
| `TARS_MODEL_TIMEOUT` | 可选 | `30s` | 模型调用超时 |
| `TARS_REASONING_PROMPTS_CONFIG_PATH` | 建议提供 | `/etc/tars/reasoning-prompts.yaml` | Reasoning prompt 注入文件；用于控制 LLM 如何生成 `summary/execution_hint` |
| `TARS_DESENSITIZATION_CONFIG_PATH` | 建议提供 | `/etc/tars/desensitization.yaml` | 脱敏规则配置文件；用于控制 secret 检测、HOST/IP/PATH 占位与回填策略 |
| `TARS_PROVIDERS_CONFIG_PATH` | 建议提供 | `/etc/tars/providers.yaml` | 统一模型 Provider 配置文件；用于维护主模型/辅助模型绑定和 provider entries |
| `TARS_CONNECTORS_CONFIG_PATH` | 平台化阶段建议提供 | `/etc/tars/connectors.yaml` | 统一连接器注册表配置文件；用于管理 Prometheus / JumpServer / MCP source 等外部系统 manifest |
| `TARS_REASONING_LOCAL_COMMAND_FALLBACK_ENABLED` | 默认 `false` | `false` | 是否开启本地命令生成兜底；建议保持关闭，让命令生成完全由 LLM 负责 |

> 一般情况下，模型密钥属于运行时 secret。不要把 `TARS_MODEL_API_KEY` 写入公开仓库、截图或对外资料，建议只通过环境变量、`.env.local`（不提交）或 secret manager 注入。
>
> 当前仓库中的 [deploy/team-shared](team-shared) 是一个明确的例外：它只用于团队内部开发和共享测试，包含当前联调所需的内部凭据。请把它视为“内部测试包”，不要继续向更大范围分发；如果仓库权限范围扩大，应先轮换其中的密钥。

> 当前推荐模式是 `LLM-only command generation`。也就是平台不再本地 hardcode “负载/磁盘/出口 IP” 这类命令映射，而是通过 injected prompt 让模型产生命令候选；平台后续只做结构化解析、授权匹配、审批和执行。

> 现在模型接入支持两种配置方式：
>
> 1. 直接通过 `TARS_MODEL_PROTOCOL / TARS_MODEL_BASE_URL / TARS_MODEL_NAME / TARS_MODEL_API_KEY`
> 2. 通过 `TARS_PROVIDERS_CONFIG_PATH` 维护统一 provider registry，并绑定：
>    - `primary`: 主模型，用于 diagnosis / command generation
>    - `assist`: 辅助模型，用于本地可信边界内的脱敏辅助、fallback 或后续安全增强
>
> 如果配置了 `TARS_PROVIDERS_CONFIG_PATH` 且绑定了 `primary/assist`，运行时会优先使用 registry 结果覆盖静态 `TARS_MODEL_*` 配置。

> 如果同时启用了 Web Console 和配置文件路径，当前可直接在 `/ops` 页面维护三类运行时配置并热加载，无需重启：
>
> - `TARS_AUTHORIZATION_CONFIG_PATH`
> - `TARS_APPROVALS_CONFIG_PATH`
> - `TARS_REASONING_PROMPTS_CONFIG_PATH`
> - `TARS_DESENSITIZATION_CONFIG_PATH`
>
> `/ops` 默认提供引导式表单，仍保留 Advanced YAML 模式以支持复杂规则。

> `/ops` 现在还支持统一 Provider 配置中心：
>
> - `GET/PUT /api/v1/config/providers`
> - `POST /api/v1/config/providers/models`
> - `POST /api/v1/config/providers/check`
> - `GET/PUT /api/v1/config/connectors`
> - `POST /api/v1/config/connectors/import`
> - `GET /api/v1/connectors`
> - `GET /api/v1/connectors/{id}`
> - `GET /api/v1/connectors/{id}/export`
> - `POST /api/v1/connectors/{id}/enable`
> - `POST /api/v1/connectors/{id}/disable`
> - `POST /api/v1/connectors/{id}/metrics/query`
>
> 你可以在页面里维护 OpenAI / Claude / Gemini / OpenRouter / Ollama / LM Studio 等 provider entry，再点选绑定 `primary` 和 `assist`。模型列表拉取和可用性检查也通过同一入口完成。
>
> Connector Registry 则用于维护平台化外部系统接入基线：`Prometheus / VictoriaMetrics / JumpServer / MCP skill source` 等都将逐步收敛到统一 manifest 入口。当前 `/connectors/:id` 已支持 manifest 导出、enable/disable，以及对 `prometheus_http / victoriametrics_http` 连接器执行 runtime metrics smoke。共享测试基线中，execution 默认优先选择健康的 `jumpserver-main`；若 JumpServer connector 未通过 health probe 或缺少 secret refs，则 workflow 会自动回退 `ssh`。
>
> 当前 metrics runtime 会直接读取 connector 实例自身的 `config.values.base_url / bearer_token`。公开 `GET /api/v1/connectors/{id}` 与 `GET /api/v1/connectors/{id}/export` 默认不回显 runtime connection values；真实连接值只通过受 `ops-token` 保护的 `/api/v1/config/connectors` 控制面维护。
>
> 对 LM Studio / 本地 Ollama 这类本地小模型，如果 prompt 较长或带 few-shot examples，默认 `TARS_MODEL_TIMEOUT=30s` 往往偏紧。试点建议先提高到 `60s-90s`，再观察真实延迟。
>
> 注意：`List models` 依赖上游 provider 暴露相应的模型列表接口（例如 `/models`、`/tags`）。有些兼容网关只支持推理、不支持列模型；这种情况下 `/api/v1/config/providers/check` 仍可用于验证“当前 model 是否可用”。

> 脱敏配置中的 `local_llm_assist` 当前已支持 `detect_only`，可使用本地可信边界内的 `openai_compatible`、`anthropic`、`ollama` 或 `lmstudio` 协议；如果本地辅助模型不可达、配置不完整或返回非法 JSON，主链路会自动回退到纯规则式脱敏。只有在 `local_llm_assist.enabled=true` 时，辅助模型才会参与这条链路；单独配置 `assist` 绑定不会自动启用 raw-context 脱敏检测。

### 2.5 SSH 执行

| 环境变量 | 是否必需 | 示例 | 说明 |
|---|---|---|---|
| `TARS_SSH_USER` | 试点必需 | `root` | SSH 登录用户 |
| `TARS_SSH_PRIVATE_KEY_PATH` | 试点必需 | `/etc/tars/id_rsa` | 私钥路径；当前实现不支持密码式登录 |
| `TARS_SSH_ALLOWED_HOSTS` | 试点必需 | `192.168.3.106,192.168.3.107` | 允许执行的目标主机白名单 |
| `TARS_SSH_CONNECT_TIMEOUT` | 可选 | `10s` | SSH 建连超时 |
| `TARS_SSH_COMMAND_TIMEOUT` | 可选 | `5m` | 单次命令执行超时 |
| `TARS_SSH_DISABLE_HOST_KEY_CHECKING` | 试点建议 `false` | `false` | 生产建议关闭该开关并使用系统 `known_hosts` |
| `TARS_SSH_ALLOWED_COMMAND_PREFIXES` | 可选 | `hostname,uptime,systemctl status` | 命令前缀白名单；不配置则使用内置默认值 |
| `TARS_SSH_BLOCKED_COMMAND_FRAGMENTS` | 可选 | `rm -rf,mkfs,shutdown` | 命令黑名单片段；不配置则使用内置默认值 |

### 2.6 执行输出与审批

| 环境变量 | 是否必需 | 示例 | 说明 |
|---|---|---|---|
| `TARS_EXECUTION_OUTPUT_SPOOL_DIR` | 建议提供 | `/var/lib/tars/execution_output` | 执行输出落盘目录 |
| `TARS_EXECUTION_OUTPUT_MAX_PERSISTED_BYTES` | 建议提供 | `262144` | 单次执行在数据库中保留的最大输出字节数；超出部分仅保留前缀并标记 `output_truncated=true` |
| `TARS_EXECUTION_OUTPUT_CHUNK_BYTES` | 建议提供 | `16384` | `execution_output_chunks` 单块最大字节数 |
| `TARS_EXECUTION_OUTPUT_RETENTION` | 建议提供 | `168h` | `execution_output_chunks` 与本地 spool file 的保留期 |
| `TARS_APPROVAL_TIMEOUT` | 建议提供 | `15m` | 待审批执行超时后会自动拒绝 |
| `TARS_APPROVALS_CONFIG_PATH` | 试点建议提供 | `/etc/tars/approvals.yaml` | 审批路由配置文件路径；不配时审批消息回退到默认目标；当前可通过 Web `/ops` 页面读写 |
| `TARS_AUTHORIZATION_CONFIG_PATH` | 建议提供 | `/etc/tars/authorization.yaml` | 命令授权策略文件；当前已支持 `ssh_command` 的 glob + direct/approval/suggest/deny，且可通过 Web `/ops` 页面读写 |

### 2.7 Feature Flags

| 环境变量 | 是否必需 | 试点推荐值 | 说明 |
|---|---|---|---|
| `TARS_ROLLOUT_MODE` | 试点建议提供 | `approval_beta` | 预设开关组合；支持 `diagnosis_only` / `approval_beta` / `execution_beta` / `knowledge_on` |
| `TARS_FEATURES_DIAGNOSIS_ENABLED` | 必需 | `true` | 是否启用 diagnosis worker |
| `TARS_FEATURES_APPROVAL_ENABLED` | 必需 | `true` | 是否启用审批流 |
| `TARS_FEATURES_EXECUTION_ENABLED` | 必需 | `true` | 是否启用真实执行 |
| `TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED` | 必需 | `true` | 是否启用 `session.closed` 知识沉淀 |

说明：

- `TARS_ROLLOUT_MODE` 会先应用一组预设开关，再允许 `TARS_FEATURES_*` 做逐项覆盖
- 如果 `TARS_ROLLOUT_MODE` 留空或填了未识别值，系统会回退到 `custom`
- 当前预设含义：
  - `diagnosis_only`: 只做告警接入、诊断和消息通知
  - `approval_beta`: 开启审批，但不做真实执行和知识沉淀
  - `execution_beta`: 开启审批和真实执行，知识沉淀仍关闭
  - `knowledge_on`: 四项能力全开

## 3. 哪些配置必须由你来提供

以下项无法从代码推断，必须由部署或产品环境决定：

- `TARS_TELEGRAM_BOT_TOKEN`
- `TARS_TELEGRAM_WEBHOOK_SECRET` 如果你使用 webhook 模式
- `TARS_SERVER_PUBLIC_BASE_URL` 如果你使用 webhook 模式
- `TARS_TELEGRAM_POLLING_ENABLED` 以及 polling 参数，如果你使用 long polling 模式
- `TARS_VMALERT_WEBHOOK_SECRET`
- `TARS_VM_BASE_URL`
- `TARS_MODEL_PROTOCOL`
- `TARS_MODEL_BASE_URL`
- `TARS_MODEL_API_KEY` 如果模型网关需要鉴权
- `TARS_MODEL_NAME` 如果模型网关要求显式模型名
- `TARS_PROVIDERS_CONFIG_PATH` 如果你希望通过统一 Provider Registry 管理主模型和辅助模型
- `TARS_CONNECTORS_CONFIG_PATH` 如果你希望通过统一 Connector Registry 管理平台化外部系统接入
- `TARS_VECTOR_SQLITE_PATH` 如果你希望启用向量检索
- `TARS_SSH_USER`
- `TARS_SSH_PRIVATE_KEY_PATH`
- `TARS_SSH_ALLOWED_HOSTS`
- `TARS_OPS_API_TOKEN`
- `TARS_APPROVALS_CONFIG_PATH`
- `TARS_ROLLOUT_MODE` 如果你希望用统一预设控制试点开关

## 4. 获取与设置方法

### 4.1 Telegram

当前实现已经支持两种模式：

1. `webhook`
   - 需要 Telegram 公网回调到 TARS
   - 需要 `TARS_SERVER_PUBLIC_BASE_URL`
   - 需要 `TARS_TELEGRAM_WEBHOOK_SECRET`
2. `long polling`
   - **不需要** Telegram 主动访问 TARS
   - 不依赖公网 HTTPS webhook
   - 只要 TARS 自己能主动访问 Telegram API 即可

如果产品要求是“平台不直接暴露公网”，当前建议优先使用 **long polling**。

### 4.1.1 推荐的试点开关组合

如果我们按“先诊断、再审批、最后执行”的试点节奏推进，推荐这样配：

```sh
TARS_ROLLOUT_MODE=approval_beta
TARS_FEATURES_EXECUTION_ENABLED=false
TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED=false
```

说明：

- 只配 `TARS_ROLLOUT_MODE=approval_beta` 也可以
- 额外显式写 `TARS_FEATURES_EXECUTION_ENABLED=false` / `TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED=false` 的作用是把当前试点意图写死，便于排障
- 等执行链验证稳定后，再把 `TARS_ROLLOUT_MODE` 切到 `execution_beta` 或 `knowledge_on`

#### 推荐模式

- 内网/试点优先：`long polling`
- 需要标准 Telegram webhook 运维模式时：`webhook`

### 4.0 Web Console 配置中心

当前 `/ops` 页面已支持：

- 命令授权策略：引导式表单 + Advanced YAML
- 审批路由配置：引导式表单 + Advanced YAML
- 统一 Provider Registry：引导式表单 + Advanced YAML + 模型列表拉取 + 可用性检查
- Reasoning prompt 配置：表单编辑 + Advanced YAML
- 脱敏规则配置：引导式表单 + Advanced YAML

这些页面都会把配置写回各自的运行时文件，并立即热加载到当前进程。页面不会展示任何敏感 secret 明文，也不负责编辑 Bot Token、模型 API Key 这类 secret。

#### 模式 A：long polling

推荐环境变量：

```sh
TARS_TELEGRAM_BOT_TOKEN=replace-with-bot-token
TARS_TELEGRAM_BASE_URL=https://api.telegram.org
TARS_TELEGRAM_POLLING_ENABLED=true
TARS_TELEGRAM_POLL_TIMEOUT=30s
TARS_TELEGRAM_POLL_INTERVAL=1s
```

说明：

- 启动后 TARS 会自动调用 Telegram `deleteWebhook(drop_pending_updates=false)`，切换到 polling
- 后台 worker 会持续调用 `getUpdates`
- 目前只消费 `callback_query`
- 这已经足够支持 MVP 的审批按钮回调

最小检查：

```sh
curl "https://api.telegram.org/bot${TARS_TELEGRAM_BOT_TOKEN}/getMe"
```

#### 如何拿到测试 `chat_id`

建议优先用 **私聊 chat_id** 做第一轮联调，最简单。

1. 在 Telegram 里找到机器人并点击 `Start`
2. 再给机器人发一条消息，比如 `ping`
3. 在任意终端执行：

```sh
export BOT_TOKEN=替换成你的机器人 token
curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates" | jq '
  .result[]
  | select(.message.chat.type == "private")
  | {
      chat_id: .message.chat.id,
      username: .message.from.username,
      text: .message.text
    }'
```

你只需要把输出里的 `chat_id` 发给我，不要把 `BOT_TOKEN` 再贴出来。

如果要用 **测试群**：

1. 新建一个测试群，或者选一个专用测试群
2. 把机器人拉进群
3. 在群里发一条命令，比如 `/ping`
4. 然后执行：

```sh
export BOT_TOKEN=替换成你的机器人 token
curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates" | jq '
  .result[]
  | select(.message.chat.type == "group" or .message.chat.type == "supergroup")
  | {
      chat_id: .message.chat.id,
      title: .message.chat.title,
      text: .message.text
    }'
```

注意：

- 私聊 `chat_id` 通常是正整数，例如 `123456789`

#### Telegram 对话模式

当前版本除了审批按钮回调，也支持用户直接给机器人发送自然语言请求。推荐先从只读查询开始，例如：

```text
看系统负载
host=192.168.3.106 看系统负载
查看磁盘使用情况
看一下 sshd 状态
看一下你的出口IP是多少
service=sshd host=192.168.3.106 看一下服务状态
```

行为说明：

- 如果 `TARS_SSH_ALLOWED_HOSTS` 只有一台主机，机器人会把它当作默认目标主机
- 如果白名单里有多台主机，而消息里没有主机名或 `host=...`，机器人只返回引导信息，不会创建 session
- 对话请求会复用现有 `diagnosis -> approval -> execution -> verification` 主链路
- 当前建议只让模型给出只读命令，例如 `uptime && cat /proc/loadavg`、`free -m`、`df -h`、`systemctl status <service>`、`curl -fsS https://api.ipify.org && echo`
- 真正执行前仍然需要 Telegram 审批，不会因为“只是聊天请求”而跳过审批
- 对话请求、建议命令、审批动作、结果消息会同时保留在 `audit_logs`、`alert/approval` 相关表和知识沉淀文档里，便于后续学习改进与审计复盘
- 群 / 超级群 `chat_id` 通常是负数，超级群常见格式是 `-100...`
- 如果机器人还没接到 TARS，最好先用这个方式取 `chat_id`，再把 token 配给 TARS；这样不会和 long polling 抢 `getUpdates`
- 对现在这版后端来说，审批路由配置里填的目标值本质上就是这个 `chat_id`

最小给我这 2 个值就够了：

- `private_chat_id` 或 `group_chat_id`
- 你希望先把哪条路由指到它，例如 `service_owner.sshd` 或 `oncall_group.default`

#### 模式 B：webhook

1. 在 BotFather 创建机器人，拿到 `TARS_TELEGRAM_BOT_TOKEN`。
2. 生成一个随机 secret 作为 `TARS_TELEGRAM_WEBHOOK_SECRET`。示例命令：

```sh
openssl rand -hex 16
```

3. 确保 `TARS_SERVER_PUBLIC_BASE_URL` 可以被 Telegram 公网访问，且带有效 HTTPS 证书。
4. 设置 Telegram webhook：

```sh
curl -X POST "https://api.telegram.org/bot${TARS_TELEGRAM_BOT_TOKEN}/setWebhook" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://tars.example.com/api/v1/channels/telegram/webhook",
    "secret_token": "替换成 TARS_TELEGRAM_WEBHOOK_SECRET"
  }'
```

5. 验证：

```sh
curl "https://api.telegram.org/bot${TARS_TELEGRAM_BOT_TOKEN}/getWebhookInfo"
```

### 4.2 VMAlert

TARS 当前不是做 HMAC，而是校验固定请求头：

- 请求头名：`X-Tars-Signature`
- 如果配置了 `TARS_VMALERT_WEBHOOK_SECRET`，请求头值必须等于该 secret
- 如果未配置 `TARS_VMALERT_WEBHOOK_SECRET`，则不会强制校验该请求头，适合纯内网 smoke

如果 `vmalert` 不能直接加这个头，建议在 ingress / nginx / gateway 层补一个固定 header。

回调地址：

```text
POST https://tars.example.com/api/v1/webhooks/vmalert
```

兼容性说明：

- TARS 同时兼容 `vmalert` 直接拼出的 Alertmanager 路径 `.../api/v1/webhooks/vmalert/api/v2/alerts`
- payload 同时兼容当前自定义对象格式和 Alertmanager 风格数组格式

最小 smoke 示例：

```sh
curl -X POST "https://tars.example.com/api/v1/webhooks/vmalert" \
  -H "Content-Type: application/json" \
  -H "X-Tars-Signature: 替换成 TARS_VMALERT_WEBHOOK_SECRET" \
  -d '{
    "status": "firing",
    "alerts": [
      {
        "labels": {
          "alertname": "HighCPU",
          "instance": "192.168.3.106",
          "service": "sshd",
          "severity": "critical"
        },
        "annotations": {
          "summary": "cpu too high"
        }
      }
    ]
  }'
```

### 4.3 在 `192.168.3.106` 自建 VictoriaMetrics / vmalert 做 smoke

这是当前最推荐的测试方式，因为你已经确认 `192.168.3.106` 可用，而且 TARS 已经在这台机上完成过 SSH / Postgres smoke。

建议最小拓扑：

- `victoria-metrics`
- `vmalert`
- `node-exporter`

推荐目标：

- `node-exporter` 抓本机指标
- `vmalert` 阈值设低，保证很容易触发
- alert label 里的 `instance` 直接写成 `192.168.3.106`，这样能直接命中当前 SSH allowlist

一个可行的最小 `docker-compose` 思路如下：

```yaml
docker compose -f /path/to/vm-smoke-compose.yml up -d
```

仓库里已经提供了可直接使用的文件：

- [vm-smoke-compose.yml](docker/vm-smoke-compose.yml)
- [vmagent.yml](docker/vmagent.yml)
- [vmalert-rules.yml](docker/vmalert-rules.yml)

默认规则已经内置在 [vmalert-rules.yml](docker/vmalert-rules.yml)，使用 `up{job="node"} >= 1`，启动后即可快速触发。

在 `192.168.3.106` 的实际 smoke 中，已验证：

- `vmalert -> TARS` 通过 Alertmanager 兼容路径成功入库
- 生成告警 `TarsSmokeNodeUp`
- `approval_group` 正确写成 `service_owner:sshd`
- 审批消息下发到 `sshd-owner`，并带 3 个动作按钮

如果 `vmalert` 不能直接打 `X-Tars-Signature`，建议在 `192.168.3.106` 本机再加一层 nginx / caddy，把固定 header 补上后再转给 TARS。

### 4.4 模型网关

当前代码 **已经支持**：

- `openai_compatible`
- `anthropic`
- `ollama`
- `lmstudio`

配置项：

- `TARS_MODEL_PROTOCOL`
- `TARS_MODEL_BASE_URL`
- `TARS_MODEL_NAME`
- `TARS_MODEL_API_KEY`  
  仅在协议需要鉴权时提供，LM Studio / 本地 Ollama 可留空。

协议约定：

- `openai_compatible`
  - 自动调用 `POST {base}/v1/chat/completions`
  - 如果 `base_url` 已经以 `/v1` 结尾，则直接补 `/chat/completions`
- `anthropic`
  - 自动调用 `POST {base}/v1/messages`
  - 自动附带 `anthropic-version: 2023-06-01`
- `ollama`
  - 自动调用 `POST {base}/api/chat`
- `lmstudio`
  - 走 OpenAI-compatible 路径
  - 对 `http://host:1234` 这类裸地址会自动补成 `/v1/chat/completions`

最小检查：

1. `openai_compatible` / `lmstudio`

```sh
curl -X POST "${TARS_MODEL_BASE_URL%/}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  ${TARS_MODEL_API_KEY:+-H "Authorization: Bearer ${TARS_MODEL_API_KEY}"} \
  -d '{
    "model": "'"${TARS_MODEL_NAME}"'",
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

2. `anthropic`

```sh
curl -X POST "${TARS_MODEL_BASE_URL%/}/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: ${TARS_MODEL_API_KEY}" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "'"${TARS_MODEL_NAME}"'",
    "max_tokens": 64,
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

3. `ollama`

```sh
curl -X POST "${TARS_MODEL_BASE_URL%/}/api/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"${TARS_MODEL_NAME}"'",
    "stream": false,
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

### 4.5 VictoriaMetrics

`TARS_VM_BASE_URL` 需要直接指向 VictoriaMetrics 根地址，例如：

```text
http://vm.example.com:8428
```

最小检查：

```sh
curl "${TARS_VM_BASE_URL}/api/v1/query?query=up"
```

### 4.6 SSH

当前执行通道使用私钥认证。需要保证：

- `TARS_SSH_PRIVATE_KEY_PATH` 存在且权限正确
- `TARS_SSH_USER` 对目标主机可登录
- 目标主机在 `TARS_SSH_ALLOWED_HOSTS` 白名单里
- 生产建议设置 `TARS_SSH_DISABLE_HOST_KEY_CHECKING=false`

最小检查：

```sh
ssh -i /etc/tars/id_rsa root@192.168.3.106 "hostname && uptime"
```

### 4.7 审批路由配置

`TARS_APPROVALS_CONFIG_PATH` 指向一份 YAML 文件。MVP 最小例子：

```yaml
approval:
  default_timeout: 15m
  prohibit_self_approval: true
  routing:
    service_owner:
      sshd:
        - "sshd-owner"
    oncall_group:
      default:
        - "ops-room"
  execution:
    command_allowlist:
      sshd:
        - "systemctl restart sshd"
        - "systemctl status sshd"
        - "systemctl is-active sshd"
```

说明：

- `service_owner.<service>` 优先级高于 `oncall_group.default`
- `execution.command_allowlist.<service>` 用于补充该服务允许的修复类命令前缀；它和全局只读 allowlist 叠加生效
- 值本质上是当前渠道的目标标识；在 Telegram 场景下建议填可直接发送的 chat / user target
- 未配置文件时，审批消息会回退到默认通知目标

### 4.7.1 当前运行时边界

当前二进制对命令执行的控制是“两层并行”：

- 外层边界：`TARS_SSH_ALLOWED_HOSTS`
- 全局只读 allowlist：`TARS_SSH_ALLOWED_COMMAND_PREFIXES`
- 全局危险片段 blocklist：`TARS_SSH_BLOCKED_COMMAND_FRAGMENTS`
- 服务级补充 allowlist：`approval.execution.command_allowlist.<service>`
- 统一授权策略：`TARS_AUTHORIZATION_CONFIG_PATH`

需要特别注意：

- 当前运行时已经支持 `ssh_command` 的统一通配符授权策略文件
- env allowlist/blocklist 仍保留，优先作为兼容配置与底层 hard guardrails
- MCP skill 和插件动作的统一授权还没有落地

### 4.7.2 后续策略方向

后续建议把 SSH 命令和 MCP skill 都纳入统一授权模型：

- 白名单：`direct_execute`
- 黑名单：默认 `suggest_only`
- 其他：`require_approval`
- 少量高风险项继续保留 `hard_deny`

详细设计见：

- [命令与能力授权策略](../specs/30-strategy-command-authorization.md)
- [vNext 授权配置样例](../configs/authorization_policy.vnext.example.yaml)

## 5. MVP 试点推荐环境变量

下面是进入试点前建议至少配置的 `.env` 样例。这里默认使用 **Telegram long polling**：

```sh
TARS_LOG_LEVEL=INFO
TARS_SERVER_LISTEN=:8081
TARS_POSTGRES_DSN=postgres://tars:tars@postgres:5432/tars?sslmode=disable
TARS_VECTOR_SQLITE_PATH=/var/lib/tars/tars_vec.db

TARS_OPS_API_ENABLED=true
TARS_OPS_API_TOKEN=replace-with-random-token

TARS_TELEGRAM_BOT_TOKEN=replace-with-bot-token
TARS_TELEGRAM_BASE_URL=https://api.telegram.org
TARS_TELEGRAM_POLLING_ENABLED=true
TARS_TELEGRAM_POLL_TIMEOUT=30s
TARS_TELEGRAM_POLL_INTERVAL=1s

TARS_VMALERT_WEBHOOK_SECRET=replace-with-random-secret
TARS_VM_BASE_URL=http://vm.example.com:8428
TARS_VM_TIMEOUT=15s

TARS_MODEL_PROTOCOL=openai_compatible
TARS_MODEL_BASE_URL=https://model-gateway.example.com/v1
TARS_MODEL_API_KEY=replace-with-api-key
TARS_MODEL_NAME=gpt-4o-mini
TARS_MODEL_TIMEOUT=30s
TARS_REASONING_PROMPTS_CONFIG_PATH=/etc/tars/reasoning-prompts.yaml
TARS_DESENSITIZATION_CONFIG_PATH=/etc/tars/desensitization.yaml
TARS_REASONING_LOCAL_COMMAND_FALLBACK_ENABLED=false

# LM Studio sample
# TARS_MODEL_PROTOCOL=lmstudio
# TARS_MODEL_BASE_URL=http://192.168.1.132:1234
# TARS_MODEL_NAME=qwen/qwen3-4b-2507
# TARS_MODEL_API_KEY=
# TARS_MODEL_TIMEOUT=90s

TARS_SSH_USER=root
TARS_SSH_PRIVATE_KEY_PATH=/etc/tars/id_rsa
TARS_SSH_ALLOWED_HOSTS=192.168.3.106
TARS_SSH_CONNECT_TIMEOUT=10s
TARS_SSH_COMMAND_TIMEOUT=5m
TARS_SSH_DISABLE_HOST_KEY_CHECKING=false

TARS_EXECUTION_OUTPUT_SPOOL_DIR=/var/lib/tars/execution_output
TARS_APPROVAL_TIMEOUT=15m
TARS_APPROVALS_CONFIG_PATH=/etc/tars/approvals.yaml
TARS_AUTHORIZATION_CONFIG_PATH=/etc/tars/authorization.yaml

TARS_FEATURES_DIAGNOSIS_ENABLED=true
TARS_FEATURES_APPROVAL_ENABLED=true
TARS_FEATURES_EXECUTION_ENABLED=true
TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED=true
```

## 6. 启动步骤

### 6.1 启动 PostgreSQL

本地 Docker 快速启动：

```sh
cd "$(git rev-parse --show-toplevel)"
docker compose -f deploy/docker/docker-compose.yml up -d postgres
```

### 6.2 执行 migration

注意：当前初始化脚本读取的是 `DATABASE_URL`，不是 `TARS_POSTGRES_DSN`。

```sh
cd "$(git rev-parse --show-toplevel)"
export DATABASE_URL="postgres://tars:tars@127.0.0.1:5432/tars?sslmode=disable"
./scripts/init_db.sh
```

### 6.3 启动 TARS

```sh
cd "$(git rev-parse --show-toplevel)"
source .env
go run ./cmd/tars
```

### 6.4 基础健康检查

```sh
curl http://127.0.0.1:8081/healthz
curl http://127.0.0.1:8081/readyz
curl -H "Authorization: Bearer ${TARS_OPS_API_TOKEN}" http://127.0.0.1:8081/api/v1/sessions
```

### 6.5 Setup / Smoke 快速体验

如果你已经配置好了 Telegram / Model / VM / SSH，可以直接走一轮最短体验：

1. 查看只读运行时状态：

```sh
curl -H "Authorization: Bearer ${TARS_OPS_API_TOKEN}" \
  http://127.0.0.1:8081/api/v1/setup/status
```

2. 触发一条 Smoke Alert：

```sh
curl -X POST "http://127.0.0.1:8081/api/v1/smoke/alerts" \
  -H "Authorization: Bearer ${TARS_OPS_API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "alertname": "TarsSmokeManual",
    "service": "sshd",
    "host": "192.168.3.106",
    "severity": "critical",
    "summary": "Manual smoke triggered from setup page."
  }'
```

返回体会给出：

- `session_id`
- `status`
- `duplicated`
- `tg_target`

3. 在 Telegram 中完成审批，然后回到 Web Console 或 Ops API 查看同一条会话：

```sh
curl -H "Authorization: Bearer ${TARS_OPS_API_TOKEN}" \
  "http://127.0.0.1:8081/api/v1/sessions/<session_id>"
```

## 7. 当前实现的 fallback 行为

以下情况不会阻止进程启动，但会降级：

- 未配置 `TARS_TELEGRAM_BOT_TOKEN`
  - Telegram 发送退化为 stub，polling 也不会启动
- `TARS_TELEGRAM_POLLING_ENABLED=false` 且未配置 webhook
  - Telegram 审批回调无法进入 TARS
- 未配置 `TARS_MODEL_BASE_URL`
  - diagnosis 使用 deterministic fallback，不调用真实模型
- 未配置 `TARS_VM_BASE_URL`
  - metrics 查询走 stub，不访问真实 VictoriaMetrics
- 未配置 `TARS_OPS_API_TOKEN` 且开启了 Ops API
  - 运维接口仍会暴露，但无法安全用于试点；不建议这样部署

## 8. 试点前检查清单

- `PostgreSQL` 可连接，migration 已执行
- `healthz` 与 `readyz` 正常
- 如果使用 polling：`getMe` 可用，且 worker 启动后无报错
- 如果使用 webhook：`Telegram webhook` 已设置成功
- `VMAlert` 已能带 `X-Tars-Signature`
- `SSH` 私钥可登录目标主机
- `TARS_FEATURES_APPROVAL_ENABLED=true`
- `TARS_FEATURES_EXECUTION_ENABLED=true`
- `TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED=true`
- `Ops API` 只绑定内网地址，且 token 已设置
- `execution_output` 目录存在并可写

## 9. 当前仍待你补给的外部配置

如果要把 MVP 从“可验证”推进到“试点可用”，还需要你提供：

- `TARS_SERVER_PUBLIC_BASE_URL`
- `TARS_TELEGRAM_BOT_TOKEN`
- `TARS_TELEGRAM_WEBHOOK_SECRET` 如果你使用 webhook
- `TARS_TELEGRAM_POLLING_ENABLED=true` 如果你使用 polling
- `TARS_VMALERT_WEBHOOK_SECRET`
- `TARS_VM_BASE_URL`
- `TARS_MODEL_PROTOCOL`
- `TARS_MODEL_BASE_URL`
- `TARS_MODEL_API_KEY` 如果你的模型网关需要鉴权
- `TARS_MODEL_NAME` 如果网关不接受默认值
- `TARS_SSH_USER`
- `TARS_SSH_PRIVATE_KEY_PATH`
- `TARS_SSH_ALLOWED_HOSTS`
- `TARS_OPS_API_TOKEN`
