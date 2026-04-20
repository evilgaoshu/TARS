# TARS Post-MVP 路线设计

> 状态：讨论基线  
> 适用范围：MVP 完成后的产品与平台演进方向  
> 关联文档：[tars_prd.md](../project/tars_prd.md) / [tars_technical_design.md](../project/tars_technical_design.md) / [docs/20-component-connectors.md](20-component-connectors.md)

## 1. 目标

MVP 阶段，TARS 已经验证了主链路价值：

`输入事件/对话 -> AI 分析 -> 受控审批 -> 执行 -> 校验 -> 审计/知识沉淀`

Post-MVP 的目标不是“继续堆更多功能”，而是把 TARS 逐步演进成一个**可积累经验、可接入生态、可扩展交互、可平台化开放**的运维 Agent 平台。

本阶段重点关注 4 个方向：

1. 多层记忆系统
2. 自我升级 / 自我扩展
3. Agent 交互范式扩展
4. MCP 开放接口

## 2. 总体判断与优先级

建议优先级如下：

1. 多层记忆系统
2. MCP 接口与生态兼容
3. Agent 交互范式扩展
4. 自我升级 / 自我扩展

优先级依据：

- 多层记忆系统最直接增强当前诊断和执行主链路
- MCP 能同时增强“接入别人”和“被别人接入”的能力
- 交互范式扩展会提升产品体验，但不应该先于记忆和开放协议
- 自我升级价值很大，但风险也最高，必须在平台和治理基础较稳后再推进

补充约束（2026-04-02）：

- 在外部系统接入维度，近期一等对象先收敛为 `SSH / VictoriaMetrics / VictoriaLogs`
- `Prometheus / JumpServer / MCP` 继续保留兼容和设计空间，但不再作为近程默认重心
- 后续任何 connector 相关设计，都不应抢过这 3 类对象的模板、验证与控制面优先级

## 3. 方向一：多层记忆系统

### 3.1 设计目标

TARS 后续不应只有单一的 RAG 层，而应该至少分成“会话记忆、经验记忆、检索记忆、原始事实层”四个层级。

核心原则：

- 事实层与总结层分离
- 自动沉淀与人工审核分离
- 检索层与执行建议层分离
- 任何可影响执行的记忆都必须可追溯来源

### 3.2 建议的 4 层模型

#### L0：会话记忆

面向当前交互或当前告警会话的短期记忆。

内容包括：

- 最近几轮 Telegram / Web Chat 对话
- 当前 session 的 alert/context/execution/verification 状态
- 用户临时补充的信息

特点：

- 生命周期短
- 强时序性
- 主要服务当前会话推理

#### L1：经验记忆

面向复用和偏好的中期记忆。

内容包括：

- 已闭环故障总结
- 用户偏好
- 经审核的操作建议
- Skill 草稿 / 模板

特点：

- 可沉淀、可审核、可淘汰
- 会直接影响后续诊断和建议质量
- 不应与原始事实混存

#### L2：检索记忆

面向召回和问答的知识索引层。

内容包括：

- 文档 chunk
- 闭环记录的结构化切片
- 规则索引
- citation / ACL / version 元数据

特点：

- 是 RAG 层的主要运行时入口
- 可以融合向量和 lexical 检索

#### L3：原始事实层

面向审计、取证、再加工的原始数据层。

内容包括：

- Confluence 原文
- IM 原始聊天记录
- 告警原始 payload
- 工单、变更记录、执行原始输出
- 监控/日志/APM 原始事实引用

特点：

- 这是 source of truth
- 不允许被模型直接改写
- 只能被索引、引用、摘要，不应被覆盖

### 3.3 关键约束

- L1 不得反向覆盖 L3
- L2 必须带来源和版本
- L1 的自动写入建议至少要有“审核前草稿态”
- 影响命令建议的记忆项必须保留来源与写入主体

### 3.4 对现有系统的影响

- `Knowledge Service` 需要从“单一知识服务”演进成“多层记忆服务”
- `Session Trace` 需要支持记忆来源标记
- `SkillDraft` 需要正式纳入经验记忆层
- `Audit / Knowledge / Trace` 需要可查看“来自哪一层”

## 4. 方向二：自我升级 / 自我扩展

### 4.1 总体判断

这个方向值得做，但不应该直接做“系统自动修改自己的核心模块”。更稳妥的路径是：

1. 自我扩展
2. 受控自我改进
3. 有边界的自我升级

### 4.2 建议分阶段

后续更稳妥的主路径应是：

- 先做受控自扩展
- 再做可审查的自生成扩展包
- 最后才考虑有限自升级

对应实现载体应优先是：

- `Skill`
- `Extension Bundle`
- `Registry / Platform API`
- `Automation / Builder Pipeline`

而不是让模型直接修改平台核心模块。

当前进展（2026-03-22）：

- 已落第一条 `Skill -> Extension Bundle Candidate -> Validate / Preview / Import -> Skill Registry` 最小链路
- 已补 `Review -> Import` 显式门槛，候选扩展必须先批准再导入
- 当前 bundle 先只开放 `skill_bundle`，并携带 docs/tests metadata
- `/extensions` 已作为 Web 控制面入口进入产品基线

#### Phase A：Self-extension

允许模型生成这些低风险产物：

- 连接器草稿
- 渠道草稿
- Provider / Auth Provider 草稿
- MCP tool adapter 草稿
- 查询模板
- Prompt / policy / rule 建议
- Skill package 草稿
- 文档包草稿

要求：

- 只产出 proposal / patch / manifest
- 默认不直接生效

#### Phase B：Self-improvement with review

允许系统生成可提交的改动，但必须走审核和测试。

例如：

- 自动生成 connector manifest
- 自动生成 channel/provider/auth provider bundle 草稿
- 自动生成 provider/connector 适配层代码草稿
- 自动生成文档、测试和 config schema 草稿
- 自动补规则和测试草稿

要求：

- 必须有测试和审查门槛
- 必须可以回滚

#### Phase C：Limited self-upgrade

只允许升级低风险、边界明确的对象：

- connector package
- channel package
- provider / auth provider package
- prompt package
- policy template
- skill source manifest
- docs pack

不建议自动改的对象：

- workflow core
- auth / approval / execution 核心
- desensitization final enforcement

### 4.3 风险边界

- 自我升级不能绕过审批和审计
- 不能直接让模型修改核心安全边界
- 不能让生成的适配器自动获得高权限
- 生成的新扩展应先成为 `Extension Bundle`，再通过测试、兼容性检查、导入和启用流程进入平台
- 文档自动生成也应纳入 review / publish / rollback，而不是直接覆盖已有手册

## 5. 方向三：Agent 交互范式扩展

### 5.1 总体判断

TARS 当前的主要交互入口是 Telegram。后续应该从“单一 IM 交互”演进到“统一交互协议 + 多前端入口”。

建议优先顺序：

1. Web Chat
2. 语音输入
3. 实时语音 / 多模态

### 5.2 统一交互协议

建议统一抽象这些交互语义：

- `chat_message`
- `approval_action`
- `ask_for_context`
- `execution_result`
- `trace_reference`
- `citation_reference`

然后再通过 `Channel Adapter` 适配：

- Telegram
- Web Chat
- Voice Gateway
- 未来 Slack / 飞书 / 电话网关

### 5.3 设计原则

- 状态机只在 `Workflow Core`
- 交互层只负责协议转换
- 语音只是输入/输出适配器，不应改业务编排语义

### 5.4 对现有系统的影响

- `Channel Adapter` 要升级成统一交互入口
- `Session` 需要支持多交互源
- `Trace / Audit` 需要保留语音转写来源与置信度

## 6. 方向四：系统暴露 MCP 接口

### 6.1 总体判断

MCP 非常值得做，而且它和平台化、插件市场、外部系统接入是同一条线。

建议同时考虑两个方向，但分阶段推进：

- `MCP Client`
- `MCP Server`

### 6.2 MCP Client

TARS 作为客户端去接别人暴露的 MCP Server。

适用场景：

- 导入工具能力
- 导入外部查询能力
- 导入 Skill/模板源

当前基础设施已就绪：Capability Runtime 接口已定义，stub runtime 已按 connector type（mcp / skill）注册，`invocable` 字段和 `POST .../capabilities/invoke` 入口已实现。后续 MCP Client 可在此基础上实现真实协议调用。

### 6.3 MCP Server

TARS 作为服务端暴露自己的能力。

建议优先暴露的能力：

- session / execution / trace 查询
- connector registry 查询
- smoke trigger
- 只读诊断入口
- 审批动作提交

不建议一开始直接暴露的能力：

- 无审批的真实执行
- 高风险写操作
- 原始 secret 读取

### 6.4 与连接器平台的关系

MCP 不应另起一套模型，而应该复用当前连接器平台：

- MCP Tool 可视为 `mcp_tool` connector
- 外部 Skill Source 可视为 `skill_source` connector
- Marketplace / package / import-export 统一走 connector registry 语义

当前 `connector.invoke_capability` 已作为统一桥梁实现：tool plan 中非标准能力通过此入口调用，Capability Runtime 按 connector type 路由到对应 runtime（包括 MCP / Skill 的 stub runtime）。后续 MCP Tool / Skill Source 接入时直接替换 stub 为真实协议实现即可。

## 7. 建议的落地顺序

### Phase 1：Memory + MCP 基础

- 建立 L0-L3 记忆边界
- 让 Knowledge Service 支持多层标识
- 增加 MCP Server 最小只读接口
- 增加 MCP Client 的最小 source registry

> 部分已完成：Capability Runtime 基础设施已就绪（接口定义 + stub runtime 注册 + `invocable` 标记 + HTTP 入口 + 能力级授权）。MCP Client 的真实协议调用尚未实现，当前 MCP/Skill stub runtime 仅完成框架接入。

### Phase 2：交互扩展 + 可导入扩展

- Web Chat
- MCP tool/source 注册
- Skill Source 导入
- experience memory 的审核流

### Phase 3：受控自我扩展

- connector / prompt / skill 草稿生成
- manifest/package proposal
- 自测 + 审核 + 回滚

### Phase 4：有限自我升级

- 低风险 package 升级
- 连接器版本迁移
- 模板升级与回滚

## 8. 不建议过早做的事情

- 让模型直接修改核心执行或审批模块
- 让经验记忆直接覆盖原始事实层
- 让语音入口先于 Web Chat 成为主入口
- 一开始就暴露高风险 MCP 执行能力

## 9. 对当前规划的建议

如果把这 4 个方向合并成一句话，建议是：

**先把 TARS 做成“有记忆、可被接入、可扩交互”的平台，再谨慎推进“自我升级”。**

当前最合理的路线不是全面并行，而是：

1. 多层记忆系统
2. MCP 基础
3. Web Chat
4. Skill/MCP source 导入
5. 受控自我扩展
6. 有边界的自我升级
