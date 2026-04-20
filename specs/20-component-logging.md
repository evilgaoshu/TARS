# TARS — Logging 规范

> **状态**: 设计基线
> **适用范围**: runtime logs 检索、日志关联字段、检索与导出能力
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-observability.md](./20-component-observability.md)、[20-component-audit.md](./20-component-audit.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Logging 是什么

`Logging` 是 TARS 的 **原始运行日志对象域**。

它回答：

- 平台后台实际打印了什么
- 哪个组件报错
- 某次 session / execution / trace 的底层日志是什么

#### Logging 不是什么

- 不是审计轨迹
- 不是 metrics / trace summary
- 不是执行输出本身
- 不是只看 stdout 的外部部署约定

#### 当前真实心智

真实页面 `web/src/pages/logs/LogsPage.tsx` 是 **runtime logs 检索工作台**：

- 关键字检索
- level / component 过滤
- 按 page 分页
- 展示 trace / session / execution 关联键

### 1.2 用户目标与关键场景

#### 高频任务

- 搜索最近错误日志
- 看某个 component 的运行日志
- 用 `trace_id / session_id / execution_id` 关联调查
- 判断是平台错误还是业务对象状态问题

#### 关键场景

- 从业务对象异常回溯到底层平台日志
- 快速找到某个组件在某个时间段内的错误输出
- 判断问题应该继续去 Audit、Observability、Sessions 还是 Executions 深挖

### 1.3 状态模型

#### 日志查询状态

- `loading`
- `loaded`
- `empty`
- `failed`

#### 日志等级状态

- `debug`
- `info`
- `warn`
- `error`

#### 存储状态

- `healthy`
- `retained`
- `truncated`
- `unavailable`

#### 展示优先级

1. 当前有没有错误
2. 错误来自哪个 component
3. 有没有可关联的 trace / session / execution

### 1.4 核心字段与层级

#### L1 默认字段

- `timestamp`
- `level`
- `component`
- `message`
- `trace_id`

#### L2 条件字段

- `route`
- `actor`
- `session_id`
- `execution_id`
- `resource_type / resource_id`

#### L3 高级字段

- `metadata`
- `duration_ms`
- file path / storage bytes
- query time range

#### L4 系统隐藏字段

- rotation internals
- ingestion cursor
- raw sink metadata

#### L5 运行诊断字段

- stack trace
- blob / truncated payload detail
- collector / export failure detail

### 1.5 关键规则与约束

- `/logs` 作为运行日志主入口，负责搜索、过滤、分页和关联跳转
- `/ops/observability` 只做摘要与样本，不替代 `/logs` 的检索职责
- Sessions / Executions / Audit 可以通过关联键消费 logs，但不承担原始日志全局检索职责
- runtime logs 与 audit 必须持续分离

#### 推荐演进方向

- 统一关联字段：`trace_id / session_id / execution_id`
- 后续补更强的时间过滤、导出与外部 sink 治理

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 查原始后台日志
- 缩小到某个 level、component 或关联对象
- 通过关联键跳去对应 session / execution / trace
- 判断这是平台故障还是业务对象状态问题

#### 任务映射

| 用户任务 | 主入口 | 不应作为主入口 |
|---------|--------|----------------|
| 查原始后台日志 | `/logs` | `/audit` |
| 查操作审计 | `/audit` | `/logs` |
| 看平台观测摘要 | `/ops/observability` | `/logs` |
| 查命令输出 | Execution Detail | `/logs` |

#### 首屏必须回答的 3 个问题

1. 当前有没有错误日志需要优先处理
2. 这些错误来自哪个 component、是否能快速缩小范围
3. 我能不能通过关联键继续跳到 session / execution / trace

### 2.2 入口与页面归属

#### `/logs`

作为运行日志主入口，负责：

- 搜索
- 过滤
- 分页
- 关联跳转

#### `/ops/observability`

只做摘要与样本，不替代 `/logs` 的检索职责。

#### Sessions / Executions / Audit

这些页通过关联键消费 logs，但不承担原始日志全局检索职责。

### 2.3 页面结构

#### 列表结构

当前结构应保持：

1. summary stats
2. 搜索与过滤条
3. 日志表格
4. 分页

#### 页面优先级

首屏先回答：

1. 当前有没有错误
2. 错误来自哪个 component
3. 有没有可关联的 trace / session / execution

### 2.4 CTA 与操作层级

#### 主动作

- `搜索`
- `筛选`
- `刷新`

#### 次级动作

- `查看 Session`
- `查看 Execution`
- `查看 Trace`

#### 高级动作

- `展开元数据`
- `查看完整堆栈`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在筛选条和补充信息区
- L4/L5 不应默认占据表格首屏
- stack trace、rotation internals、sink metadata 进入展开区或高级区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Logs`
- 对象名：`Runtime Log Entry`

#### 页面叙事

- 页面讲“原始运行日志”
- 不讲“审计记录”
- 不把 Logs 讲成执行输出页或 observability 摘要页

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Logs`
- 副标题应表达：搜索平台原始运行日志并沿关联键继续调查

### 3.3 CTA 文案

主路径默认使用：

- `搜索`
- `筛选`
- `刷新`

次级路径默认使用：

- `查看 Session`
- `查看 Execution`
- `查看 Trace`

高级区允许：

- `展开元数据`
- `查看完整堆栈`

### 3.4 状态文案

#### 没有命中日志

- 结论：`当前筛选条件下没有日志`
- 细节：可放宽关键字、level 或 component
- 动作：`清空筛选`

#### 日志后端不可用

- 结论：`当前无法读取 runtime logs`
- 细节：本地 JSONL store、文件轮转或权限路径异常
- 动作：`前往 Observability` 或 `Ops`

#### 只有审计没有原始日志

- 结论：`当前调查只能看到审计证据，缺少 runtime logs`
- 细节：可能是 retention 已清理或采集链异常
- 动作：`检查 Logging / Observability 配置`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- rotation internals
- ingestion cursor
- raw sink metadata
- 把 Logs 讲成“审计”或“执行输出”

这些内容可留在高级区，不应主导 Logging 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/logs` 已清晰表达为原始运行日志检索页
- 页面默认先给错误、component 和关联键，而不是深层元数据
- `/logs`、`/audit`、`/ops/observability`、Execution Detail 的边界清晰

### 4.2 交互级验收

- 用户能按关键字、level、component 和关联键快速过滤
- 用户能顺畅跳去相关 session / execution / trace
- 查看高级字段不会打断主检索体验

### 4.3 展示级验收

- 表格至少展示 timestamp、level、component、message、trace_id
- 空态、日志后端不可用、只有审计没有原始日志等状态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖搜索、过滤、关联跳转和关键状态文案
- 需要浏览器或截图验收确认 Logging 默认叙事已经从“杂项技术页”收口为“原始运行日志检索台”
- 若后端尚未提供更强的时间范围与导出，前端不应伪装成已有完整日志分析能力

### 4.5 剩余限制说明

- 时间范围过滤、导出和外部 sink 治理仍可作为下一阶段增强项
- 更深的 collector/export 调试继续保留在高级区或 `Ops`
