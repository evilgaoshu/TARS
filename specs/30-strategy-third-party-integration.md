# TARS — 第三方系统集成策略与可行性评估

> **状态**: Next Phase 设计基线  
> **定位**: 统一说明 TARS 应如何接入第三方系统，并评估当前平台方案的可行性、边界与落地顺序  
> **关联**: [20-component-connectors.md](20-component-connectors.md)、[10-platform-components.md](10-platform-components.md)、[90-design-self-evolving-platform.md](90-design-self-evolving-platform.md)

## 1. 目标

TARS 后续不应只依赖少量内置系统能力，而应具备统一的第三方系统接入策略，用于：

- 接入观测系统
- 接入执行与堡垒机系统
- 接入交付、变更、CI/CD 系统
- 接入渠道系统
- 接入模型与身份系统
- 接入外部能力源（MCP / Skill Source / Marketplace）

这份文档的目标不是再定义一个新平台，而是明确：

1. 第三方系统应该通过哪些方式接入
2. 当前 TARS 方案为什么可行
3. 哪些类型的系统适合优先接
4. 哪些部分后续会更难，需要先补治理底座

一句话定义：

> TARS 的第三方系统集成应坚持“对象治理 + 运行时能力 + 场景编排 + 扩展治理”四层分离，而不是把所有接入都写成零散适配代码。

## 2. 总体策略

推荐坚持以下 4 层模型：

1. `Registry / Platform Object`
   - 管理对象是谁
   - 配置、版本、状态、启停、导入导出、审计如何治理
2. `Runtime / Capability`
   - 这个对象在运行时能做什么
   - 暴露哪些 `capability`
3. `Skill / Tool-plan`
   - 面对一个具体场景，如何把多个能力组合起来
4. `Bundle / Marketplace / Import-Export`
   - 如何导入、升级、回滚、复制、审核扩展

对应关系：

- `Connector`
  - 解决“如何访问一个系统”
- `Channel`
  - 解决“如何与用户交互”
- `Provider / Auth Provider`
  - 解决“如何接入模型和身份基础设施”
- `Skill`
  - 解决“如何编排这些能力”
- `Extension Bundle`
  - 解决“如何治理这些扩展”

## 3. 第三方系统接入的 6 条主路径

### 3.1 Webhook 接入

适合：

- 告警系统
- 回调系统
- 事件推送系统

典型例子：

- VMAlert
- Alertmanager
- Feishu / Slack / Discord webhook
- CI/CD 回调

特点：

- 成本低
- 上手快
- 非常适合“外部系统主动推事件给 TARS”

适用边界：

- 只负责事件进入，不适合承载复杂查询与状态治理

### 3.2 Connector Runtime 接入

适合：

- 监控系统
- 堡垒机系统
- 日志与 APM 系统
- Git / CI/CD / 交付系统

典型例子：

- VictoriaMetrics / Prometheus
- JumpServer
- SkyWalking
- GitHub / GitLab
- Jenkins / ArgoCD

特点：

- 这是 TARS 当前最核心的南向接入主路径
- 统一通过 Connector Registry + Capability Runtime 暴露系统能力
- 最适合承载查询、执行、观测、交付这类系统能力

适用边界：

- 几乎所有“系统能力型”接入都应优先走这条路径

### 3.3 Channel 接入

适合：

- 用户交互入口
- 审批入口
- 通知入口

典型例子：

- Telegram
- Feishu
- Discord
- Slack
- Web Chat

特点：

- 解决的是“人如何和平台交互”
- 重点是渠道身份、消息模板、能力差异、交互协议
- 不应混进普通 Connector Runtime

### 3.4 Provider / Auth Provider 接入

适合：

- 模型供应商
- 企业身份系统

典型例子：

- OpenAI-compatible / Anthropic / Ollama / LM Studio
- OIDC / OAuth / LDAP / 后续 SAML

特点：

- 这类属于平台底座供应商，不应作为普通业务系统连接器处理
- 应通过 Provider / Auth Provider 平台单独治理

### 3.5 MCP / Skill Source 接入

适合：

- 外部工具源
- 外部技能源
- 第三方生态扩展源

典型例子：

- MCP server
- 外部 skill index
- 模板仓库
- 外部 source registry

特点：

- 更偏“生态与能力源”
- 不是简单连接某个系统，而是接入一类扩展能力

### 3.6 Bundle / Marketplace / Import-Export 接入

适合：

- 官方扩展包
- 第三方扩展包
- 环境迁移
- 环境复制
- 插件市场

典型例子：

- connector bundle
- channel bundle
- provider / auth provider bundle
- skill bundle
- docs pack

特点：

- 解决“如何导入、升级、回滚、复制扩展”
- 是生态化与自扩展治理的关键底座

## 4. 当前方案为什么可行

### 4.1 分层是正确的

当前 TARS 已经形成了相对清晰的分层：

- `Connector`：访问外部系统
- `Skill`：编排外部系统能力
- `Channel`：连接用户
- `Provider / Auth Provider`：连接模型与身份基础设施
- `Bundle / Marketplace`：治理扩展

这比“每接一个系统就加一套特殊逻辑”更容易扩展、测试和治理。

### 4.2 Runtime 与控制面已经开始统一

当前平台已不是纯概念设计，而是已有事实基础：

- discovery
- registry
- enable / disable
- import / export
- lifecycle
- tool-plan runtime
- capability invoke

这意味着当前方案是“已经被部分验证的路线”，不是从零起步。

### 4.3 Skill 作为编排层是正确方向

TARS 后续不应把“如何使用第三方系统能力”继续硬编码在 reasoning 或 handler 中，而应：

- 让第三方能力先作为系统能力进入 planner 视野
- 由 Skill / tool-plan 组合这些能力
- 在需要时再触发执行、审批和回滚

这对于后续接入：

- Feishu
- Discord
- SkyWalking
- OAuth / OIDC
- CI/CD

都是更稳定的路线。

## 5. 哪些第三方系统最容易接

### 5.1 高可行类型

这些系统最适合优先接入：

- HTTP API 型系统
- Webhook 型系统
- PromQL / REST 查询型系统
- Git / GitHub / GitLab
- JumpServer 这类清晰 API 系统
- OIDC / OAuth 这类成熟标准协议

原因：

- 协议标准化程度高
- 易于做 connector / capability 抽象
- 易于测试
- 易于复用成熟开源库

### 5.2 中高可行但需要更多治理支持

这些系统可接，但更依赖平台底座完善：

- Feishu / Discord / Slack 全渠道能力
- SkyWalking / APM / tracing 深层查询
- LDAP / SAML 这类企业身份系统
- Jenkins / ArgoCD / 内部发布系统

难点通常不在“能不能连上”，而在：

- 权限边界
- 回调模型
- 速率限制
- SDK 复杂度
- 版本兼容
- 配置治理
- secret 管理

## 6. 当前方案的主要优势

### 6.1 优势一：适合先查询、后决策、再执行

当前 TARS 已经从“固定 enrich + 直接建议命令”演进到：

- planner
- tool-plan
- capability runtime
- final summarizer

这使得第三方系统接入可以先作为“查询能力”进入平台，而不是一上来就 SSH 上机。

### 6.2 优势二：治理链天然可扩展

因为现在已经有：

- registry
- lifecycle
- import / export
- health
- compatibility
- audit

所以新增第三方系统时，至少有机会进入统一治理，而不是永久停在“脚本 + 配置 + 文档约定”。

### 6.3 优势三：便于后续自扩展

如果后续想让系统自动生成新插件、渠道或文档，当前路线也更容易演进到：

- Skill 生成草稿
- Bundle 承载扩展
- Registry 治理扩展
- 测试 / 审批 / 回滚决定是否生效

这条能力的详细边界见 [90-design-self-evolving-platform.md](90-design-self-evolving-platform.md)。

## 7. 当前方案的主要风险

### 7.1 风险一：组件平台化进度不均衡

当前：

- Connector 平台化程度最高
- Skill 已进入主路径
- Provider / Channel / Auth Provider / Users/Auth/AuthZ 仍在继续补

这意味着：

- 单个系统能接入
- 但并不代表所有类型的系统都已经有成熟的治理路径

### 7.2 风险二：自动生成新扩展最容易失控

例如：

- 自动生成 Feishu / Discord / SkyWalking / OAuth 插件
- 自动生成对应文档

技术上可行，但如果没有：

- Extension Bundle
- Builder Pipeline
- validate / test / review / import
- trust / signing
- rollback

就很容易失控。

### 7.3 风险三：治理能力不足时，接得越多越乱

如果这些底座不及时补齐：

- Users / Authentication / Authorization
- Provider / Channel 平台化
- Secret / KMS / Encryption Governance
- Compatibility Matrix
- Bundle Import / Export

那第三方系统接得越多，平台越容易回到“多个特殊接入堆在一起”的状态。

## 8. 可行性评估

### 8.1 总体评级

| 维度 | 评级 | 说明 |
|------|------|------|
| 架构方向 | 高 | 分层清晰，治理路径正确 |
| 短期落地性 | 高 | HTTP / Webhook / 查询型系统接入可快速推进 |
| 中期生态扩展性 | 高 | Connector / Skill / Bundle 路线适合继续扩展 |
| 自动自进化能力 | 中 | 可行，但必须走受控自扩展 |
| 企业级治理成熟度 | 中 | 仍需补 IAM、Secret、Compatibility、Trust 等地基 |

### 8.2 结论

结论很明确：

- 当前方案是可行的
- 而且方向是正确的
- 最应该坚持的是“统一对象治理 + 统一能力调用 + 统一扩展治理”

不建议回到：

- 每个系统一套特殊适配
- 每个插件一套独立生命周期
- Skill 直接写底层配置
- 模型直接改核心模块

## 9. 推荐实现顺序

如果目标是“后续可持续接第三方系统，甚至自动长出新插件和文档”，建议按这个顺序推进：

1. 继续补齐一级平台组件
   - Providers
   - Channels
   - Users / Authentication / Authorization
2. 统一 Extension Bundle
3. 完善 Import / Export / Upgrade / Rollback
4. 建立 Builder Pipeline
5. 再做 Skill 驱动的扩展生成

一句话：

> 先把“可治理的集成平台”做完整，再做“可自扩展的平台”。

## 10. 边界结论

后续 TARS 的第三方系统接入应坚持以下边界：

- `Connector` 负责访问系统并暴露能力
- `Channel` 负责用户交互入口
- `Provider / Auth Provider` 负责平台基础供应商
- `Skill` 负责场景编排
- `Extension Bundle` 负责扩展治理

在这个前提下，TARS 既可以持续接入新的第三方系统，也能逐步获得“受控自进化”的能力，而不会失去平台治理边界。
