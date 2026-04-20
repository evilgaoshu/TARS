# GitHub CI/CD 与密钥管理规划

> 基线策略：GitHub 只承担可公开、可重复、无共享主机依赖的校验与静态 demo。`192.168.3.100` AMD64 是本地受控开发测试环境，不作为 GitHub Actions 的远端部署目标。

## 环境分层

| 环境 | 用途 | 边界 |
| --- | --- | --- |
| 本地开发测试机 `192.168.3.100` AMD64 | 主要集成测试、远端 smoke、live validation、手工 demo 前验收 | 只在本地可信网络访问；通过本地 shell 注入 secrets；不由 GitHub Actions 直接 SSH |
| GitHub CI | PR 与 `main` 的 L0/L1/L2 校验 | GitHub-hosted runner；只跑无共享主机依赖的测试、构建、静态检查 |
| GitHub demo | 对外演示用静态页面或静态产物 | 优先 GitHub Pages；构建过程不读取 runtime secrets；不承诺后端 live 能力 |

如果后续确实需要“带后端的公网 demo”，应单独建 `demo-backend` 环境和 threat model，不要把本地测试机或 GitHub Pages 直接升级成准生产环境。

## 推荐流水线

### PR Validation

- 触发：pull request。
- 执行：`make pre-check`、`make check-mvp`、`make security-regression`、`make secret-scan`、`make static-demo-build`。
- 约束：不得访问 `192.168.3.100`，不得读取 Telegram / model provider / SSH / Dex runtime secrets。
- 产物：测试结果、Web build artifact；不部署。

### Main Validation

- 触发：合并到 `main`。
- 执行：重复 PR validation，并产出可下载构建产物。
- 约束：仍不部署到 `192.168.3.100`。
- 产物：release candidate artifact；可选发布静态 demo。

### GitHub Demo

- 触发：`main` validation 通过后。
- 执行：只构建并发布静态前端/demo 产物到 GitHub Pages 或等价静态托管。
- 约束：Pages build 不注入 `TARS_OPS_API_TOKEN`、模型 API key、Telegram token、SSH 私钥、Dex client secret。
- 预期：用于展示 UI、文档和静态交互，不作为真实告警闭环验收。
- 可选的 preview 数据可以来自 Supabase Free 或其他一次性后端，但这条路径只能用于预览，不得进入 required checks。

### 本地部署到 `192.168.3.100`

- 触发：开发者在本地手动运行。
- 执行：`make deploy-sync` 或 `make deploy`。
- 变量：本地 shell 显式设置 `TARS_REMOTE_HOST=192.168.3.100`、`TARS_REMOTE_USER`、`TARS_OPS_API_TOKEN`；如需固定目录，再设置 `TARS_REMOTE_BASE_DIR`。
- 密钥：只从本地私有 env、系统 keychain、1Password/Bitwarden、或后续 secret manager 注入；不得回写仓库。
- 验证：`make smoke-remote`、`make live-validate`、必要时 `make web-smoke`。

## 密钥管理

- 仓库内只保留 `REPLACE_WITH_*` 和 `REPLACE_WITH_REMOTE_*` 占位模板。
- 本地测试机密钥由本地操作员注入，不放进 GitHub secrets，也不放进 GitHub Pages build。
- GitHub repo secrets 只用于 CI 真正需要的最小权限值；当前阶段原则上不需要 required-check 的 runtime provider secrets。
- GitHub environment secrets 只在未来需要受保护 demo backend 时启用，并绑定环境审批。
- 曾经出现在仓库或共享包里的真实值必须先轮换，再扩大仓库可见范围。
- `TARS_OPS_API_TOKEN`、Telegram token、Dex client secret、模型 API key、TOTP seed、JumpServer key、SSH 私钥都按“暴露过即轮换”处理。

## 当前禁止放进 GitHub Actions 的内容

- SSH 到 `192.168.3.100`。
- root SSH、host-key bypass、`sshpass`。
- Telegram long polling / webhook live 校验。
- VictoriaMetrics / VictoriaLogs public playground 作为必过自动化依赖。
- 任何会改变共享测试机状态的 live validation。

## 下一步

1. 推 GitHub 前先完成当前树 secret scan，并保留扫描报告。
2. 轮换已经出现过的真实凭据；不能轮换的，必须由 owner 明确确认已失效或仅为 fixture。
3. 建立第一次 GitHub tracked baseline，但在轮换确认前不要 push。
4. 配置 GitHub branch protection：PR-only、required checks（`MVP Checks` 的 L0/L1/L2/Secret Scan/Static Demo Build jobs）、最小 workflow 权限。
5. 需要静态 demo 时再接 GitHub Pages；需要后端 demo 时另开设计，不复用本地测试机。
