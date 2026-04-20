# TARS — Tool-plan 驱动诊断与媒体结果设计

> **状态**: 已实现第一版，继续增强中  
> **日期**: 2026-03-19  
> **适用范围**: 告警分析、对话诊断、外部系统调用、媒体结果返回  
> **关联文档**: [tars_prd.md](../project/tars_prd.md) / [tars_technical_design.md](../project/tars_technical_design.md) / [20-component-connectors.md](20-component-connectors.md)

## 1. 背景

旧实现里，诊断 worker 会在推理前固定做两件事：

- `QueryMetrics`
- `Knowledge.Search`

然后把 `metrics_series / knowledge_hits` 一起塞给 LLM，再由 LLM 输出：

- `summary`
- `execution_hint`

这个模式能支撑 MVP，但它不适合“过去一小时机器负载”“先看监控再决定是否上机”“最好返回图表”这类真实运维问题。

## 2. 目标

把当前 “固定 enrich + 一次推理 + 可选执行” 的模式，升级成：

**`LLM 先产出 tool plan -> 平台执行 plan -> LLM 再总结 -> 必要时才走受控执行`**

核心目标：

- 诊断优先查询监控/APM/日志/变更，不优先上机执行
- 执行动作只在必要时才进入审批和执行通道
- 支持 range query，不再局限于 instant query
- 支持图片、文件等富结果返回，不只返回纯文本
- 让 `VictoriaMetrics / Prometheus / JumpServer / MCP` 等统一成为可规划、可审计、可回放的工具调用
- 让 connector / MCP / skill source 都先作为“系统能力”进入 planner 视野，再由 LLM 判断是否需要使用

## 3. 当前问题总结

| 问题 | 表现 | 影响 |
|------|------|------|
| 固定 enrich | 不管用户问什么都先查一次基础 metrics | 工具调用不够有针对性 |
| metrics query 太弱 | 当前更像 `up` 探测，而不是真正的时序分析 | 无法回答过去一小时趋势问题 |
| 缺少工具规划语义 | LLM 只能产出 `execution_hint`，不能先说 “我要查 VM range query” | 容易过早走执行 |
| 输出形态单一 | 只能返回文本，不支持图表、文件附件 | 运维场景表达力不足 |

## 4. 目标范式

新链路应变成：

1. 用户输入问题或告警
2. LLM 先输出 `summary + tool_plan`
3. 平台按 `tool_plan` 调用 connector runtime
4. 平台把工具结果再交给 LLM 总结
5. 只有必要时才生成执行计划并进入审批

原则：

- 先规划，再调用
- 先查事实，再决定执行
- 只读证据工具优先，执行工具后置
- evidence-first 顺序默认是：`metrics -> observability/logs -> delivery -> knowledge -> execution`
- `logs / observability / delivery` 是只读证据路径，不是默认修复入口
- `SSH / execution` 只在证据不足、需要明确修复动作、或用户明确要求上机验证时才升级
- 所有工具调用都可审计
- 执行动作仍由平台控制

## 5. Tool Plan 模型

建议把当前 `DiagnosisOutput` 扩成：

```json
{
  "summary": "需要先查看目标主机过去一小时负载趋势",
  "tool_plan": [
    {
      "kind": "metrics.query_range",
      "target": "victoriametrics-main",
      "priority": 1,
      "params": {
        "host": "192.168.3.10",
        "metric": "load_average",
        "window": "1h",
        "step": "1m"
      }
    }
  ],
  "execution_hint": ""
}
```

建议支持的 plan 类型：

- `metrics.query_instant`
- `metrics.query_range`
- `knowledge.search`
- `observability.query`
- `delivery.query`
- `execution.plan`
- `execution.run`
- `connector.invoke_capability`

其中执行类 plan 仍必须继续走授权和审批。

> **实现状态**：`metrics.query_instant`、`metrics.query_range`、`observability.query`、`delivery.query`、`connector.invoke_capability` 已实现为真实执行路径。planner 提示词与归一化层都已支持 `observability.query / delivery.query` 直接产出；`observability.query` / `delivery.query` 按 connector type 自动解析目标连接器后调用 `InvokeCapability()`；`connector.invoke_capability` 直接从 step params 提取 `connector_id / capability_id` 调用。结果回填 `output / runtime / attachments / context` 并发射审计事件。2026-03-20 进一步补齐了四点：未显式指定 `connector_id` 的 tool step 会优先从 `context.tool_capabilities` 里选第一个可调用 connector，planner prompt 现在显式支持 `$steps.<step_id>.output...` / `$steps.<step_id>.input...` 多步引用，dispatcher 会在已有 delivery/observability/metrics/knowledge 证据足够时抑制无关的 generic host command，同时 finalizer 侧也新增了 evidence gate，避免模型在已有系统证据时再次生成通用主机命令。
> 2026-03-20 晚些时候又继续收了一轮 capability catalog 语义：disabled / incompatible connector 不再进入 `tool_capabilities`，同类能力会按“健康状态 + 非 stub 优先”的顺序进入目录。因此 planner 在未显式指定 `connector_id` 时，会优先拿到当前真正可用的 connector，而不是被 disabled connector 或 stub runtime 污染选择结果。

兼容性约束：

- 如果模型不确定系统里配置的精确 `connector_id`，应优先省略 `connector_id`，不要发明泛化短名。
- 平台需要兼容 planner 把 `priority` 输出成 JSON 字符串的情况，例如 `"1"`。
- 对 metrics 工具，如果模型只给出裸指标名（如 `node_load1`），平台可以在执行前按 `host/service` 自动补齐 `instance/job` 选择器，避免把查询打到过宽的时序集合上。

### 5.1 系统能力目录（Capability Catalog）

当前 planner 已支持从平台注入能力目录，而不是只依赖少量硬编码工具：

- 内置能力：
  - `knowledge.search`
- connector 暴露能力：
  - `metrics.query_instant`
  - `metrics.query_range`
  - `execution.run_command`
- 非标准 connector / MCP / skill 能力：
  - 当前先统一以 `connector.invoke_capability` 进入目录

输入形态：

- `context.tool_capabilities`
- `context.tool_capabilities_summary`

规则：

- `invocable=true` 的能力可以直接进入 `tool_plan`
- `invocable=false` 的能力只作为目录知识暴露给 LLM，不得直接被 planner 规划成可执行步骤
- disabled / incompatible connector 不会进入能力目录
- 同类能力在目录中的顺序带有平台偏好语义：优先 healthy，再优先真实 runtime，最后才是 stub
- `observability.query` / `delivery.query` 当前默认优先真实 connector（例如 `log_file` / `delivery_git`），只有明确配置 stub 时才会落到 stub
- planner 生成的 `connector_id` 支持别名归一化，但不允许模糊或随机命中
- `mcp_tool / skill_source` 在平台运行时统一归一化成 `mcp / skill`，这样 capability catalog、runtime 选择和授权规则使用的是同一套分类

## 6. 外部系统调用优先级

默认优先级建议：

1. `metrics`
2. `observability`
3. `delivery`
4. `knowledge`
5. `execution`（含 `SSH / JumpServer`，默认最后升级）

典型场景：

| 用户意图 | 首选调用 | 次选调用 | 最后手段 |
|----------|----------|----------|----------|
| 过去一小时负载 | `metrics.query_range` | `knowledge.search` | `JumpServer / SSH` |
| 服务当前状态 | `metrics.query_instant` + `delivery.query` | `observability.query` | `JumpServer / SSH` |
| 故障原因 | `knowledge.search` + `observability.query` | `delivery.query` | `JumpServer / SSH` |
| 明确修复请求 | `knowledge.search` | `execution.plan` | `execution.run` |

升级边界：

- 只要 `metrics / observability / delivery / knowledge` 已能形成足够证据，就不应因为“也许上机更直接”而默认升级到 `SSH`。
- `SSH / execution.run` 进入主路径前，必须满足以下至少一项：
  - 现有证据仍不足以支撑结论或下一步
  - 用户明确要求执行修复或主机侧验证
  - 平台已经产出受控 `execution.plan`，并进入审批/授权通道
- 当前目标是把 tool-plan 服务于“值班排障闭环助手”，而不是把所有平台能力都提前拉进默认诊断链路。

## 7. Metrics 能力升级

当前 `MetricsQuery` 更偏基础 enrich，缺少：

- 自定义 PromQL
- range query
- `start/end/step/window`
- 结果序列化为图表/文件

下一阶段至少要支持：

```json
{
  "connector_id": "victoriametrics-main",
  "query_type": "range",
  "query": "node_load1{instance=\"192.168.3.10:9100\"}",
  "window": "1h",
  "step": "1m",
  "format": "timeseries"
}
```

结果格式建议支持：

- `timeseries`
- `table`
- `chart_png`
- `chart_svg`

## 8. 媒体结果与附件协议

当前 `ChannelMessage` 只有：

- `Body`
- `Actions`

还没有统一附件模型。下一阶段建议扩成：

```json
{
  "channel": "telegram",
  "target": "445308292",
  "body": "过去一小时负载趋势如下",
  "attachments": [
    {
      "kind": "image",
      "name": "load-1h.png",
      "mime_type": "image/png",
      "ref": "file://..."
    }
  ]
}
```

附件类型优先级：

| kind | 说明 | 优先级 |
|------|------|--------|
| `image` | 图表、截图 | P0 |
| `file` | JSON、CSV、报告 | P0 |
| `audio` | 语音回复 | P2 |
| `video` | 录屏、演示 | P3 |

## 9. 审计与追踪

所有 tool plan 调用都必须留痕：

- `tool_plan_generated`
- `tool_plan_step_started`
- `tool_plan_step_completed`
- `tool_plan_step_failed`
- `tool_plan_fallback`

每条记录至少包含：

- `session_id`
- `tool_kind`
- `connector_id`
- `query_type`
- `range window/step`
- `result_format`
- `fallback_reason`

## 10. 和连接器平台的关系

这个设计不单独发明一套工具体系，而是复用连接器平台：

- `metrics.query_*` -> `metrics` connector
- `observability.query` -> `observability` connector
- `delivery.query` -> `delivery` connector
- `execution.*` -> `execution` connector
- `mcp_tool` -> MCP connector

因此，后续 tool plan 执行器只依赖：

- connector selector
- connector capability
- connector runtime

显式 `connector_id` 的处理原则：

- 优先使用精确 `connector_id`
- 若未命中，但能唯一匹配到 vendor/protocol 别名，则允许确定性归一化，例如：
  - `prometheus -> prometheus-main`
  - `victoriametrics -> victoriametrics-main`
- 若别名匹配不唯一，必须显式失败，不允许静默选择任意 connector

## 11. 排期建议

### P0

- `ToolPlan` 输出结构
- `metrics.query_range`
- `VictoriaMetrics / Prometheus` range query runtime
- 图片/文件附件协议
- Telegram / Web 的图片和文件返回

### P1

- `observability.query` ✅ 已实现（按 connector type 解析 observability connector → `InvokeCapability()` → 回填 `observability_query_result`）
- `delivery.query` ✅ 已实现（按 connector type 解析 delivery connector → `InvokeCapability()` → 回填 `delivery_query_result`）
- 多步 tool plan
- step 级审计和 trace

### P2

- MCP tool plan
- 语音/音视频附件
- 更复杂的多轮 planner

## 12. 当前实现状态

截至 2026-03-19，第一版已经落地：

- diagnosis 已切成 `planner -> execute tool steps -> final summarizer`
- 平台会审计 `tool_plan_generated / tool_plan_step_started / tool_plan_step_completed / tool_plan_step_failed`
- 需要审批或被拒绝的 capability step 也会单独审计：
  - `tool_plan_step_pending_approval`
  - `tool_plan_step_denied`
- `tool_plan / attachments` 已持久化到 PostgreSQL session 主路径，并在 Session Detail 返回
- `metrics.query_range` 已通过 `Prometheus / VictoriaMetrics` connector runtime 执行
- 图片/文件附件第一版已接入 Web Session Detail，并在 Telegram adapter 中支持 `sendPhoto / sendDocument`
- planner 已开始读取 `tool_capabilities / tool_capabilities_summary`，让 connector / MCP / skill source 统一作为系统能力进入规划上下文
- `metrics-range.png` 第一版已补齐图表标题、查询/窗口副标题、X 轴起止时间和 Y 轴数值标签，避免只有折线没有坐标信息

当前仍在继续增强的点：

- 更丰富的工具类型（如 `knowledge.search`）
- 更强的 host/service 到 metrics label 映射
- 更好的时序图表与附件格式
- planner 多步工具调用优化
- setup/status 中更丰富的 connector 运行态历史展示

截至 2026-03-21 新增实现：

- `observability.query` / `delivery.query` / `connector.invoke_capability` 三种 tool plan step 已从"待实现"切到真实执行路径
- 执行路径统一走 `action.InvokeCapability()`：按 connector type 自动解析连接器 → 调用 Capability Runtime → 回填 result.Context / output / runtime / attachments
- Capability Runtime 接口已定义并注册 stub runtime（observability / delivery / mcp / skill）
- 能力级授权已接入：`EvaluateCapability()` → read-only 直行、非 read-only `pending_approval`、MCP/Skill 命中 `hard_deny.mcp_skill` 时 `denied`
- `POST /api/v1/connectors/{id}/capabilities/invoke` HTTP 端点已上线，并与上述状态语义对齐：
  - `200 completed`
  - `202 pending_approval`
  - `403 denied`
- `invocable` 字段已加入 ConnectorCapability manifest、DTO、TypeScript 类型与 OpenAPI schema

## 13. 共享环境验收样本

截至 2026-03-20，共享测试环境 `192.168.3.106` 已验证正式样本：

- `session_id = 36adf308-ae66-41df-86de-650b80cefff8`
- 用户问题：`过去一小时机器负载怎么样`
- planner 生成 `metrics.query_range`
- runtime 选中 `prometheus-main`
- 查询命中 `node_load1{instance="127.0.0.1:9100"}`
- 返回 `series_count = 1 / points = 13`
- finalizer 直接给出趋势总结，`execution_hint = ""`
- Postgres session detail 已返回：
  - `tool_plan`
  - `metrics-range.json`
  - `metrics-range.png`
- trace 中已可见：
  - `tool_plan_generated`
  - `tool_plan_step_started`
  - `tool_plan_step_completed`
  - `chat_completions_send(finalizer)`

## 14. 结论

这项能力不应继续放在遥远的 Post-MVP 阶段，而应作为**当前高优先级下一阶段需求**推进。它直接决定 TARS 能不能优先利用监控/APM/外部系统，而不是过早上机；也直接决定系统能不能对“过去一小时负载”“给我图表”这类真实运维问题给出更像产品的回答。
