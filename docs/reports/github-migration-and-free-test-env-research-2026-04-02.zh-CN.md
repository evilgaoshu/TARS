# GitHub 迁移与免费测试环境调研

日期：2026-04-02

## 执行摘要

TARS 应该逐步切到以 GitHub 为中心的开发模式，但不能把当前这棵代码树原样直接推上去。

当前最直接的阻塞点都很具体：

- 仓库还没有稳定的 tracked history
- `deploy/team-shared` 历史上曾经承载真实密钥和地址，所以任何仍然存在的副本或历史记录都必须视为需要轮换
- 当前共享环境脚本默认依赖一台长期在线机器、`root` SSH 和长期有效 token

下一阶段最合适的“免费优先”组合是：

- GitHub 负责源码与协作
- GitHub Actions 负责 L0/L1/L2 校验
- Cloudflare Pages 只负责前端 / 演示预览
- Supabase free 只用于轻量 demo 或一次性测试数据，不承担主共享集成环境
- VictoriaMetrics 公共 play 环境只用于演示和人工验证，不进入自动化测试依赖

短期内，完整的 TARS 后端和共享集成链路仍然应该留在受控机器上，等 secrets、主机耦合和部署假设先清理干净再迁。

补充规划假设：当前主要本地开发测试主机是 `192.168.3.100`，架构为 AMD64；GitHub 应当作为演示环境，通过 GitHub-hosted CI 加静态 demo 页面来承接，除非后续另选独立 backend host。

## 当前状态盘点

已经具备 GitHub 化基础的部分：

- 基础 CI 已经存在于 [mvp-checks.yml](/Users/yue/TARS/.github/workflows/mvp-checks.yml)
- 分层 CI 的方向稿已经存在于 [ci-layered.yml](/Users/yue/TARS/.github/workflows/ci-layered.yml)
- 本地严格检查已经可以脚本化运行： [pre-check.sh](/Users/yue/TARS/scripts/ci/pre-check.sh)、 [full-check.sh](/Users/yue/TARS/scripts/ci/full-check.sh)、 [security-regression.sh](/Users/yue/TARS/scripts/ci/security-regression.sh)、 [check_mvp.sh](/Users/yue/TARS/scripts/check_mvp.sh)
- repo 内已经有一部分 secrets 外置模式，例如 [providers.shared.yaml](/Users/yue/TARS/deploy/team-shared/providers.shared.yaml) 和 [connectors.shared.yaml](/Users/yue/TARS/deploy/team-shared/connectors.shared.yaml)

还不是 GitHub-first 的部分：

- 仓库还没有稳定 baseline。此前本地检查里，`git rev-parse --verify HEAD` 失败，整棵树更像未提交工作区而不是正式仓库历史。
- 共享环境脚本依赖固定机器和硬编码环境默认值，集中体现在 [smoke-remote.sh](/Users/yue/TARS/scripts/ci/smoke-remote.sh)、 [live-validate.sh](/Users/yue/TARS/scripts/ci/live-validate.sh)、 [web-smoke.sh](/Users/yue/TARS/scripts/ci/web-smoke.sh)、 [deploy_team_shared.sh](/Users/yue/TARS/scripts/deploy_team_shared.sh)
- 当前 `team-shared` 包在发布树里应按模板安全策略处理，但 [README.md](/Users/yue/TARS/deploy/team-shared/README.md) 里的历史说明仍然重要，因为任何旧副本里如果还留着真实地址或凭据，都必须先轮换

## 安全发现

严重：

- 历史上真实 secrets 曾出现在 [shared-test.env](/Users/yue/TARS/deploy/team-shared/shared-test.env)、 [secrets.shared.yaml](/Users/yue/TARS/deploy/team-shared/secrets.shared.yaml)、 [access.shared.yaml](/Users/yue/TARS/deploy/team-shared/access.shared.yaml)、 [dex.config.yaml](/Users/yue/TARS/deploy/team-shared/dex.config.yaml)；如果还有任何残留副本，仍应视为需要轮换
- `team-shared` 文档本身也已经写明，这批值在更大范围共享前必须轮换

高风险：

- 共享部署现在要求显式设置 `TARS_REMOTE_USER`，并且 checked-in 的 `team-shared` 模板默认开启 host key checking；任何仍使用 `root` SSH 或关闭 host key checking 的旧本地副本都应替换
- [deploy_team_shared.sh](/Users/yue/TARS/scripts/deploy_team_shared.sh) 现在要求显式设置 `TARS_OPS_API_TOKEN`；该 token 只能从操作者环境或 secret manager 注入，不能来自 checked-in tree

中风险：

- [providers.example.yaml](/Users/yue/TARS/deploy/pilot/providers.example.yaml) 仍然展示了 inline provider credential，这和 repo 自己的 secret-ref 方向不一致
- 当前 GitHub workflows baseline 已经按 SHA pin；后续新增 workflow 也必须保持这一规则

## 免费优先方案评估

| 方案 | 适配度 | 现在是否采用 | 说明 |
| --- | --- | --- | --- |
| GitHub Actions | 高 | 是 | 很适合 repo 检查和分层校验。公开仓库可免费使用 GitHub-hosted runners；私有仓库按配额计。 |
| Cloudflare Pages | 高 | 是 | 很适合前端预览和演示页面。预览部署默认公开，但可结合 Access 控制。 |
| Cloudflare Workers | 低 | 不作为核心后端 | 适合小型边缘辅助逻辑，不适合 TARS 这种长生命周期 Go 后端和 worker 模型。这里是基于现状代码形态的判断。 |
| Vercel Hobby | 中 | 也许后面 | 适合个人或公开 demo 的快速前端预览，但如果 repo 放在 GitHub org 私有仓库里，适配性较差。 |
| Supabase Free | 中 | 可有限采用 | 适合一次性 demo 或轻量 Postgres 试验，不够承担长期共享集成环境。 |
| VictoriaMetrics Play | 仅 demo | 仅 demo 场景采用 | 适合人工探索、界面验证和讲解，不适合进入自动化测试合同。 |

## 官方资料要点

GitHub：

- GitHub-hosted runners 是按 job 临时创建的： [GitHub Docs](https://docs.github.com/actions/using-github-hosted-runners/about-github-hosted-runners)
- 免费额度和计费受公开 / 私有仓库状态影响： [GitHub Docs](https://docs.github.com/en/billing/managing-billing-for-github-actions/about-billing-for-github-actions)
- Environments 和 environment secrets 是正确的部署边界，但 GitHub Free 对私有 / internal repo 有限制： [GitHub Docs](https://docs.github.com/actions/reference/workflows-and-actions/deployments-and-environments) 与 [GitHub Docs](https://docs.github.com/en/actions/configuring-and-managing-workflows/creating-and-storing-encrypted-secrets)

Cloudflare：

- Pages preview 默认公开，并带 `noindex` 支持： [Cloudflare Docs](https://developers.cloudflare.com/pages/configuration/preview-deployments/)
- Pages / Workers 限制： [Cloudflare Pages Limits](https://developers.cloudflare.com/pages/platform/limits/) 与 [Cloudflare Workers Limits](https://developers.cloudflare.com/workers/platform/limits/)

Vercel：

- Hobby 计划限制和 Git 部署约束： [Vercel Hobby](https://vercel.com/docs/accounts/plans/hobby)、 [Vercel Limits](https://vercel.com/docs/limits/overview)、 [Vercel Git Deployments](https://vercel.com/docs/deployments/git)

Supabase：

- 免费计划额度和安全建议： [Billing](https://supabase.com/docs/guides/platform/billing-on-supabase)、 [Database Size](https://supabase.com/docs/guides/database/database-size)、 [RLS Hardening](https://supabase.com/docs/guides/database/hardening-data-api)、 [Secure Data](https://supabase.com/docs/guides/database/secure-data)

VictoriaMetrics：

- 公共 playground 在官方文档里就是作为 playground / demo 环境出现的： [VictoriaMetrics Playground](https://docs.victoriametrics.com/playgrounds/victoriametrics/)、 [VictoriaLogs Playground](https://docs.victoriametrics.com/playgrounds/victorialogs/)、 [VictoriaTraces Docs](https://docs.victoriametrics.com/victoriatraces/)

## 推荐的目标操作模型

### 源码管理与 PR 模型

- 先建立干净的 tracked baseline，再把活跃开发迁到 GitHub
- 所有功能开发统一走 branch + PR
- 保持 `main` 始终处于可发布状态

### CI 分层

放在 GitHub-hosted runners 上跑：

- L0：format、lint、静态检查
- L1：`go test ./...`
- L2：`bash scripts/check_mvp.sh`

暂时继续保留为手动或共享机流程：

- L3：依赖真实 connector、共享主机访问或受保护凭据的集成校验

### 环境与 Secrets

- runtime secrets 从 repo 中移出
- CI secrets 放入 GitHub secrets 或 GitHub environments；如果受套餐 / repo 可见性限制，则用外部 secrets manager
- 比 CI token 更敏感的值，优先考虑外部 secret manager 或机器本地注入
- GitHub demo 面必须保持无 secret；如果某个值只服务于本地验证，就留在本地主机，不要进 GitHub Actions

### 现在仍应留在共享基础设施上的部分

- 完整后端集成测试
- 共享主机 smoke validation
- 依赖高权限 provider 的 live connector 校验
- 一切需要长期 SSH 或内网访问的路径

### 可以先迁出的部分

- 源码管理和 PR 协作
- 不依赖共享主机权限的 unit / integration CI
- 前端预览构建
- 基于公共 playground 的 demo 级 observability 验证

## 建议的下一步

按这个顺序做：

1. 建立初始 tracked Git history
2. 在任何推送到 GitHub 之前，把 `deploy/team-shared` 模板化和脱敏
3. 轮换树里现有的 Telegram、Ops、OIDC、模型和 provider 凭据
4. 把 GitHub Actions 固定到 SHA，并补显式 workflow permissions
5. 先把 GitHub-hosted CI 限定在 L0 / L1 / L2
6. 在移除主机耦合前，把当前 shared-machine 流程继续视作 protected / manual
7. 如果短期需要 preview URL，用 Cloudflare Pages 承接前端 / demo 预览

## 目前不应该迁移的部分

- 当前原样的 `deploy/team-shared`
- 任何依赖 `root` SSH 和关闭 host key checking 的 workflow
- 整个 shared-environment 部署直接搬到免费 preview 平台
- 依赖 VictoriaMetrics 公共 playground 稳定性的自动化测试

## 决策结论

现在推荐：

- GitHub 负责源码和协作
- GitHub Actions 负责核心校验
- Cloudflare Pages 负责预览 / demo UI
- Supabase free 仅用于窄场景 demo 试验

以后再考虑：

- Cloudflare Workers 或其他 edge runtime 辅助能力
- 更正式的非前端 preview 环境

当前阶段不建议：

- 用 Vercel Hobby 承担 TARS 主后端
- 把 VictoriaMetrics playground 当作测试基础设施
- 在没先清理 repo 内 secrets 之前，就把当前 shared-machine 工作流迁到 GitHub
