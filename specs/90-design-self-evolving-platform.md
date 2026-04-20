# TARS — 受控自进化 / 自扩展平台规范

> **状态**: Next Phase 设计基线  
> **定位**: 规划 TARS 如何在不破坏安全边界和平台治理的前提下，具备受控自扩展、自生成扩展包和自生成文档的能力  
> **关联**: [91-roadmap-post-mvp.md](91-roadmap-post-mvp.md)、[10-platform-components.md](10-platform-components.md)、[30-strategy-platform-config-and-automation.md](30-strategy-platform-config-and-automation.md)

## 1. 目标

随着 TARS 从单一运维 Agent 演进为平台，后续不应只靠人工逐个开发新插件、渠道和文档，而应逐步具备：

1. 生成新的平台扩展草稿
2. 生成新的 Skill / Connector / Channel / Provider / Auth Provider / Docs 包
3. 自动补齐测试、兼容性元数据和文档
4. 在经过校验、测试、审批和导入后，让这些扩展安全地进入平台运行时

一句话定义：

> TARS 后续要具备的不是“无约束自我修改”，而是“受控自扩展”：由 Skill 和自动化流程生成候选扩展，由平台治理链决定是否生效。

## 2. 核心原则

### 2.1 不是自动改核心源码

不建议把“自我进化”定义成：

- 模型直接改生产核心代码
- 模型直接改运行中配置文件
- 模型直接启用新的高权限插件

更合理的定义是：

- 模型 / Skill 生成候选扩展
- 平台完成校验、测试、审查、导入、启用、升级、回滚

### 2.2 Skill 负责生成，平台负责治理

建议职责分层：

- `Skill`
  - 负责理解需求
  - 负责生成草稿、脚手架、文档、测试和 bundle
- `Registry / Platform API`
  - 负责写入、版本治理、状态治理、启停、升级、回滚
- `Automation / Builder Pipeline`
  - 负责校验、测试、签名、兼容性检查、导入流程

### 2.3 生成能力必须受约束

所有“自进化”能力都必须受这些约束：

- 审计
- diff / preview
- validate
- 自动化测试
- 兼容性检查
- 权限控制
- 高风险审批
- 可回滚

## 3. 目标对象

后续可由系统生成或辅助生成的对象建议至少包括：

1. `Connectors`
2. `Channels`
3. `Providers`
4. `Auth Providers`
5. `Skills`
6. `Docs Packs`

其中典型例子包括：

- 默认平台内置少量核心能力，例如：
  - Telegram
  - VictoriaMetrics / Prometheus
  - 本地模型 / 本地运行时
  - 基础认证 provider
- 后续通过 Skill 受控生成新的扩展，例如：
  - Feishu / Discord 渠道
  - Prometheus / SkyWalking 观测连接器
  - OAuth / OIDC / LDAP 认证 provider
  - 官方排障 playbook
  - 对应用户手册 / 管理员手册 / 配置说明

## 4. Extension Bundle 标准

后续建议统一引入 `Extension Bundle` 概念。

每个可安装扩展不应只是零散代码或零散配置，而应至少包含：

- `manifest`
- `version`
- `kind`
- `compatibility`
- `capabilities`
- `config schema`
- `tests`
- `docs`
- `upgrade / rollback metadata`

建议 bundle 至少支持这些 kind：

- `connector`
- `channel`
- `provider`
- `auth_provider`
- `skill`
- `docs_pack`

### 4.1 为什么必须 bundle 化

只有 bundle 化后，平台才容易支持：

- import / export
- versioning
- diff / review
- compatibility check
- signing / trust policy
- upgrade / rollback
- marketplace / source

### 4.2 文档也应进入扩展包

扩展不应只生成运行时对象，也应尽量同时生成：

- 用户文档
- 管理员文档
- 配置说明
- 兼容性说明
- 使用示例

这样新增插件、渠道和认证方式时，文档不会长期缺位。

### 4.3 当前已落地的最小实现（2026-03-22）

当前版本已先落一条 `skill bundle` 最小闭环，边界如下：

- bundle kind 暂仅支持 `skill_bundle`
- 生成入口只产出 `candidate`
- candidate 进入 `validate -> preview -> import` 流程
- candidate 进入 `validate -> preview -> review -> import` 流程
- import 最终复用现有 `Skill Registry / lifecycle / promote / rollback`
- bundle 可携带 `docs` 与 `tests` 元数据
- Web 已新增 `/extensions` 作为最小控制面入口
- candidate 已持久化到 state file，可跨重启保留

当前仍明确不做：

- 直接改平台核心源码
- 绕过 Registry 直接写底层配置
- 自动改写 `workflow / approval / auth / desensitization`

## 5. Builder Pipeline

推荐的“受控自扩展”流水线如下：

```text
需求/意图
-> Skill 生成扩展草稿
-> 生成脚手架 / manifest / config schema / docs / tests
-> validate
-> compatibility check
-> security / trust check
-> automated tests
-> review / approval
-> import into registry
-> enable / rollout
```

例如新增 `Feishu` 渠道：

1. 用户或内部自动化触发一个 `create_feishu_channel_bundle` skill
2. Skill 生成：
   - channel manifest
   - callback / message template skeleton
   - config schema
   - docs
   - tests
3. 平台执行：
   - validate
   - contract tests
   - compatibility checks
   - approval
4. 通过后导入 `Channel Registry`
5. 管理员或策略再启用该渠道

## 6. 自进化分级

建议不要一步做到“自动升级一切”，而是分三级推进。

### L1：自生成低风险产物

允许系统生成：

- Skill 草稿
- Prompt / rule / query template
- 文档草稿
- 低风险 connector/channel/provider manifest 草稿

特点：

- 风险最低
- 最适合最先落地
- 默认不直接生效

### L2：自生成可导入扩展包

允许系统生成：

- connector bundle
- channel bundle
- provider / auth provider bundle
- docs pack

特点：

- 进入 validate / test / review / import 流程
- 允许成为正式平台对象
- 默认仍不直接上线

### L3：有限自升级

允许系统升级边界明确、回滚明确的对象，例如：

- connector package
- channel package
- provider package
- skill package
- docs pack

但不建议自动升级这些核心对象：

- workflow core
- authorization final enforcement
- approval core
- desensitization hard boundary

## 7. 与 Skill、Registry、Automations 的关系

### 7.1 Skill 的角色

Skill 后续应成为：

- 场景编排器
- 扩展生成器
- 扩展导入流程的上层入口

但 Skill 不应直接：

- 写底层配置文件
- 绕过 Registry 直接修改运行时对象
- 跳过审批和审计

### 7.2 Registry 的角色

所有新增对象最终都应落到对应 Registry：

- Connector Registry
- Channel Registry
- Provider Registry
- Auth Provider Registry
- Skill Registry
- Docs Pack Registry（如后续引入）

### 7.3 Automations 的角色

Automations 可以负责：

- 周期性检测外部源是否有新版本
- 周期性生成扩展升级草稿
- 周期性生成兼容性报告
- 周期性生成文档同步草稿

但高风险导入和启用仍应保留审批边界。

## 8. 导入导出、升级、回滚

“自进化”能力必须与平台导入导出和版本治理打通。

至少需要：

- bundle export
- bundle import
- version history
- compatibility report
- upgrade plan
- rollback plan
- import / upgrade audit trail

这意味着：

- 自生成扩展不只是“生成完就结束”
- 必须成为平台治理体系中的正式资产

## 9. 安全与风险边界

### 9.1 不允许的路径

不允许：

- 模型直接修改生产核心代码并立即上线
- 生成的新扩展自动获得高权限
- 绕过审批启用执行类/写类扩展
- 绕过审计导入第三方扩展

### 9.2 必须具备的能力

至少需要：

- trust / signing
- source allowlist
- compatibility matrix
- automated tests
- sandbox / isolation（后续逐步增强）
- resource / action / capability / risk 权限控制

### 9.3 文档生成也必须受治理

文档自动生成虽然风险较低，但仍应保留：

- source / generator metadata
- review status
- publish / rollback

避免平台文档和真实运行时长期不一致。

## 10. 推荐落地顺序

建议按这个顺序推进：

1. 完整平台对象和 Registry 基础
2. Extension Bundle 标准
3. Skill 驱动的扩展草稿生成
4. validate / test / import / approval 流水线
5. 扩展升级 / 回滚 / 兼容性治理
6. 最后再做更强的有限自升级

一句话：

> 先把“可治理的扩展平台”做出来，再让系统具备“受控自扩展”能力，而不是反过来。

## 11. 与现有路线的关系

这条能力并不替代当前路线，而是建立在这些基础之上：

- Skill Platform
- Provider / Channel / Auth Provider 平台化
- Platform Bundle / Import / Export
- Automations 平台
- Package / Marketplace Trust
- 企业级权限治理

因此它更适合作为 **Post-MVP 中后段的高价值方向**，而不是当前最先落地的第一优先项。
