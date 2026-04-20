# TARS 自动化测试策略

> 目标：随着 TARS 平台能力持续扩张，建立一套分层、可复用、可脚本化的自动化测试方案，使后续大量测试工作可以安全下放给能力较弱的 agent，而不依赖口口相传或高水平临场判断。

## 1. 设计目标

测试体系需要同时满足：

- **低门槛执行**：较弱的 agent 也能按固定脚本和固定模板完成测试。
- **高风险隔离**：共享环境配置变更、break-glass、真实外部系统联调只在受控脚本内发生。
- **结果可复核**：每次测试都能产出固定格式的结论，便于 review。
- **覆盖平台化能力**：不仅覆盖 MVP 主链路，还覆盖 Connectors / Skills / Providers / Channels / People / Users / Auth / RBAC 等平台组件。
- **支持持续演进**：随着组件增多，优先通过新增测试层和新增场景，而不是让单个脚本无限膨胀。

## 2. 测试分层

TARS 当前工程化执行统一按 5 层 `L0-L4` 组织。目标不是再发明更多脚本，而是把已有稳定脚本收口成固定入口、固定依赖和固定输出。

### L0. 快速本地预检

目标：用最低成本发现仓库级破坏性改动，适合每次改动后立即执行。

固定入口：

- `make pre-check`
- 底层脚本：`bash scripts/ci/pre-check.sh`

覆盖内容：

- `go build ./...`
- `ruby scripts/validate_openapi.rb`
- 可选 `web lint`（设置 `TARS_PRECHECK_INCLUDE_WEB=1`）

特点：

- 无需共享环境
- 默认耗时约 `2s`
- 失败信息必须直接指向出错步骤

### L1. 标准本地回归

目标：在合并前稳定验证主仓回归，不依赖远端环境。

固定入口：

- `make check-mvp`
- `make full-check`
- 底层脚本：`bash scripts/check_mvp.sh`、`bash scripts/ci/full-check.sh`

覆盖内容：

- `go test ./...`
- `go build ./...`
- `ruby scripts/validate_openapi.rb`
- `cd web && npm run lint`
- `cd web && npm run build`
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build`

特点：

- `make check-mvp` 作为标准本地回归
- `make full-check` 在标准回归上补 linux/amd64 交叉编译产物
- 适合作为未来 PR 主检查 job 的直接映射

### L2. 定向平台回归

目标：围绕具体模块、契约、fixture 或控制面改动做更聚焦的回归，而不是每次都堆到共享环境。

推荐入口：

- `make test-go`
- `make validate-openapi`
- `make web-smoke`
- 需要时直接运行对应 `go test ./internal/...` 或 fixture 脚本

覆盖对象：

- reasoning / tool-plan parser
- connector runtime selection
- authorization / approval routing
- users/auth/rbac checks
- handler / DTO / OpenAPI / JSON shape
- fixture 驱动的 `delivery-main` / `observability-main` / marketplace skill 样本
- Playwright 控制面 smoke

原则：

- 新增平台组件必须至少补一条 L2 回归路径
- 能用 fixture 和契约证明的，不先上共享环境
- `make web-smoke` 依赖一个已准备好的控制面 URL，但入口保持统一，便于本地与 CI 复用

### L3. 共享环境部署与 Live Validation

目标：在 `192.168.3.106` 上跑通可重复的 `build -> deploy -> readiness -> smoke -> live-validate` 闭环。

固定入口：

- `make deploy-sync`
- `make smoke-remote`
- `make live-validate`
- `make live-validate-smoke`
- `make deploy`

底层脚本：

- `bash scripts/deploy_team_shared.sh`
- `bash scripts/ci/smoke-remote.sh`
- `bash scripts/ci/live-validate.sh`
- `bash scripts/validate_tool_plan_live.sh`

覆盖重点：

- 远端 `healthz / readyz / platform/discovery`
- hygiene summary / outbox 健康状态
- `metrics.query_range`
- capability `202 pending_approval`
- capability `403 denied`
- `observability.query`
- `delivery.query`
- 可选 tool-plan smoke 样本

原则：

- 共享环境验证一律走脚本入口，不再手写命令串
- 部署脚本只做收口，不重写成熟底层逻辑
- 失败时优先给出可读 tip，例如 token、SSH、共享环境 readiness、远端日志位置
- 部署时自动保留上一版远端二进制副本，便于最小恢复

### L4. 手工 / 高成本验收

目标：覆盖真实业务剧本、真实审批路径和高风险变更，不把这类操作误降级成普通 CI。

典型入口：

- `bash scripts/run_demo_smoke.sh`
- `bash scripts/run_golden_path_replay.sh`
- `make live-validate-auth`
- `make live-validate-extensions`
- 官方 playbook 手工验收与演练记录

典型场景：

- 官方黄金路径 v1：`VMAlert -> Session -> diagnosis -> approval -> execution -> verification -> resolved`
- 磁盘空间不足告警
- 发布后报错归因
- 负载趋势分析与图片附件
- JumpServer 审批执行闭环
- 登录增强、MFA、扩展导入等会真实改变共享状态的验证

原则：

- L4 只在有明确场景目标时执行，不作为每次提交的默认门槛
- 高风险 live 配置切换、真实外部系统变更、验收演练仍由强 agent 或人工把关
- 验收样本必须回写 tracker、runbook 或报告

## 3. 哪类 agent 跑哪层

### 3.1 能力一般的 agent

默认只做：

- `L0` 快速本地预检
- `L1` 标准本地回归
- `L2` 定向模块 / 契约 / fixture / web smoke
- 按固定脚本执行的 `L3` shared deploy / smoke / live validation

不应直接做：

- 手工改共享环境配置
- 临时替换 secrets
- 手工编辑远端 config 并恢复
- 设计新的高风险演练路径
- 未经脚本保护的 break-glass 操作

### 3.2 能力较强的 agent

负责：

- `L4` 官方场景验收
- 新测试脚本设计与扩展
- 共享环境配置临时切换与恢复
- 高风险失败场景注入
- 测试体系扩展与诊断

## 4. 标准化测试入口

后续统一维护以下入口，不允许再出现“某人知道一串命令但文档里没有”的情况。

### 4.1 本地快速预检

```bash
make pre-check
```

### 4.2 本地完整回归

```bash
make check-mvp
make full-check
```

### 4.3 共享环境部署与验证闭环

```bash
make deploy-sync
make smoke-remote
make live-validate
make deploy
```

### 4.4 共享环境 tool-plan smoke 样本

```bash
TARS_VALIDATE_RUN_SMOKE=1 make live-validate
```

### 4.5 Playwright 控制面 smoke

```bash
make web-smoke
```

### 4.6 高成本专项验证

```bash
make live-validate-auth
make live-validate-extensions
bash scripts/run_demo_smoke.sh
bash scripts/run_golden_path_replay.sh
```

说明：

- `run_demo_smoke.sh` 偏向“快速演示 / ad-hoc smoke”。
- `run_golden_path_replay.sh` 偏向“官方黄金路径回放 / 验收”，固定使用 `deploy/pilot/golden_path_alert_v1.json` 与 `deploy/pilot/golden_path_telegram_callback_v1.json` 作为样本源。

### 4.7 GitHub 迁移映射

当前阶段仍以本地与共享环境脚本为主，但入口已经按 GitHub Actions 可直接复用的方式收口。建议映射如下：

| 本地入口 | 建议 GitHub Job | 触发方式 | Secrets / Environment |
|----------|-----------------|----------|------------------------|
| `make pre-check` | `pre-check` | `pull_request`、`push` | 无 |
| `make full-check` | `full-check` | `pull_request`、`push` | 无 |
| `make web-smoke` | `web-smoke` | `workflow_dispatch`、后续可加 nightly | `TARS_PLAYWRIGHT_BASE_URL`、`TARS_PLAYWRIGHT_TOKEN` |
| `make deploy-sync` | `shared-deploy` | `workflow_dispatch`，后续可按 `main` merge 受控启用 | `TARS_REMOTE_HOST`、`TARS_REMOTE_USER`、SSH 私钥、`TARS_OPS_API_TOKEN` |
| `make smoke-remote` | `shared-smoke` | `workflow_dispatch`、后续可加 nightly | `team-shared` environment、SSH 私钥、`TARS_OPS_API_TOKEN` |
| `make live-validate` | `shared-live` | `workflow_dispatch`；nightly 建议仅跑 `core` profile | `team-shared` environment、`TARS_OPS_API_TOKEN` |
| `make live-validate-auth` / `make live-validate-extensions` | `shared-extended-live` | `workflow_dispatch` only | 共享测试账号 / MFA / 扩展导入权限 |

迁移原则：

- GitHub workflow 只负责准备运行环境和注入 secrets，不重写业务测试逻辑。
- `pull_request` 默认只跑 `L0-L1`，避免把共享环境变成每个 PR 的硬依赖。
- `main` merge 后如需共享环境联调，优先启用 `workflow_dispatch` 或受控 environment approval，而不是默认自动部署。
- nightly 仅建议跑 `make smoke-remote` 与 `TARS_LIVE_VALIDATE_PROFILE=core make live-validate`，不默认触发 auth/extensions 这类高成本样本。
- `.github/workflows/ci-layered.yml` 作为本轮迁移草案，保留现有 `mvp-checks.yml` 作为稳定基线。

## 5. 平台组件测试矩阵

后续所有平台组件都要映射到统一测试矩阵。

| 组件 | L1 | L2 | L3 | L4 |
|------|----|----|----|----|
| Connectors | runtime selection / fallback | detail/health/enable-disable/export API + fixture runtime | metrics/execution live validation | 官方诊断剧本 |
| Skills | expansion / revision / policy | registry CRUD / publish / rollback API + skill package fixtures | active skill 可见性验证 | 官方 playbook |
| Providers | protocol adapters / failover | providers config API + local model fixtures | provider health/list/check | 真实模型场景 |
| Channels | formatter / renderer | login/session/message API + mock channel fixtures | TG/Web live checks | 多渠道交互演示 |
| People | profile merge / routing | people CRUD API + fixture org/oncall data | routing sanity checks | 真实 oncall/approval 流 |
| Users/Auth/AuthZ | RBAC / mapping / token/session | users/auth/roles API + local auth fixtures | OIDC/LDAP live validation | 企业登录验收 |

## 6. 测试通过门槛

### 6.1 普通功能改动

至少满足：

- L0 全绿
- 覆盖本次改动相关的 L1/L2
- 如改动影响 runtime 或配置中心，再补一条 L3

### 6.2 平台控制面改动

至少满足：

- L0
- L1
- L2
- 一个共享环境 L3 验证

### 6.3 官方 playbook 或真实外部系统改动

至少满足：

- L0
- L1/L2/L3
- 至少一条 L4 样本

## 7. 输出模板

为了让较弱 agent 也能稳定产出，后续测试结果统一按下面模板汇报：

### 7.1 测试结果模板

```text
本轮测试范围：
- …

本地验证：
- make pre-check
- make full-check
- …

共享环境验证：
- make smoke-remote
- make live-validate
- session_id=…
- connector=…

结果：
- 通过：
  - …
- 失败：
  - …

风险：
- …

是否阻塞：
- blocker / non-blocker
```

### 7.2 Review 规则

- 没有跑 L0，不接受“基本没问题”类结论
- 涉及 API 变更但没更新 OpenAPI，视为未完成
- 涉及共享环境但没有 session_id / endpoint / connector 证据，视为验证不足
- 涉及高风险链路但没有审批/拒绝路径验证，视为验证不足

## 8. 后续自动化建设优先级

### Priority A

- 补 `skill` 平台测试脚手架
- 补 `users/auth/rbac` 平台测试脚手架
- 把 `validate_tool_plan_live.sh` 拆成可选场景子命令

### Priority B

- 增加 Web 控制面 Playwright 冒烟套件
- 继续扩充官方 playbook 的场景回放数据集（当前已落地黄金路径 v1 replay seed）
- 增加 fixture seed 校验

### Priority C（已落地：安全回归子集）

- 加入 nightly 共享环境巡检
- 加入性能基线回归
- ~~加入安全回归子集（越权、脱敏、审批绕过）~~ **✅ 已落地**

#### 安全与权限固定回归子集（已落地，2026-03-27）

**测试文件**：`internal/api/http/security_regression_test.go`（15 个测试，全部通过）

**运行入口**：

```bash
# 专项安全回归（L2，~3s）
make security-regression

# 等价命令
go test ./internal/api/http/... -run TestSecurity -v -count=1
```

**覆盖的安全边界**：

| 类别 | 测试函数 | 覆盖点 |
|------|----------|--------|
| 未认证访问矩阵 | `TestSecurityUnauthorizedAccessMatrix` | 17 个受保护端点无 token → 401 |
| 未认证写操作矩阵 | `TestSecurityUnauthorizedWriteAccessMatrix` | 7 个写端点无 token → 401 |
| Viewer 越权写 | `TestSecurityViewerCannotWriteConfigs` | viewer PUT 配置 → 403 |
| Viewer 触发自动化 | `TestSecurityViewerCannotRunAutomations` | viewer trigger automation → 403 |
| Viewer 读写一致性 | `TestSecurityViewerCanReadButNotWriteConnectors` | viewer 读 200、写 403 |
| 账号禁用 | `TestSecurityDisabledUserCannotAuthenticate` | disabled 用户 session → 401 |
| Break-glass 禁用 | `TestSecurityOpsTokenRejectedWhenOpsAPIDisabled` | OpsAPI disabled → ops-token 失效 |
| Break-glass 正常 | `TestSecurityOpsTokenGrantsFullAccess` | ops-token 有完整权限 |
| 配置脱敏 | `TestSecurityConfigAPIDoesNotExposeSecrets` | 配置 API 不暴露明文 secret 前缀 |
| 审批端点认证 | `TestSecurityApprovalEndpointRequiresAuth` | approve 端点无 token → 401 |
| Viewer 不能审批 | `TestSecurityViewerCannotApproveExecution` | viewer approve → 403/404 |
| Automation 认证 | `TestSecurityAutomationRunRequiresAuth` | automation run 无 token → 401 |
| Webhook 签名 | `TestSecurityWebhookRequiresValidSecretWhenConfigured` | 配置 secret 后强制验证 |
| 无效 token 格式 | `TestSecurityInvalidTokenFormatsAreRejected` | 5 种畸形 token → 401 |
| 公开端点白名单 | `TestSecurityPublicEndpointsWhitelist` | 5 个公开端点无 token 可访问 |

**CI 脚本**：`scripts/ci/security-regression.sh`

这组测试在 L2 层级固定运行，不依赖共享环境，约 3 秒完成，适合每次 PR 和每日 CI 执行。

## 9. 对较弱 agent 的实际分工建议

最适合交给较弱 agent 的测试工作：

- 跑 L0 基线并整理输出
- 跑指定模块的单元/契约测试
- 跑共享环境脚本并收集 session/connector 证据
- 对照固定 checklist 更新 tracker
- 回归已知 bug 样本

不适合直接交给较弱 agent 的：

- 重新设计测试方案
- 手工修共享环境
- 设计新的安全边界测试
- 做真实高风险命令演练
- 做没有脚本保护的 live 配置切换

## 10. 当前结论

TARS 后续测试不应继续依赖“懂上下文的人手工点点点”。更稳的方向是：

- 把测试拆成分层梯度
- 把共享环境入口脚本化
- 把官方场景样本固化
- 把低风险测试下放给普通 agent
- 只把高风险联调留给强 agent 或人工把关

这样平台继续变大时，测试成本不会线性失控。
