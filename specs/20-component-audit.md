# TARS — Audit 规范

> **状态**: 设计基线
> **适用范围**: audit trail search、批量导出、session 关联
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-logging.md](./20-component-logging.md)、[40-web-console-sessions-workbench.md](./40-web-console-sessions-workbench.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Audit 是什么

`Audit` 是 **平台操作审计对象域**，回答“谁对什么对象做了什么”。

#### Audit 不是什么

- 不是应用日志搜索页
- 不是 Sessions 的附属表格
- 不是底层数据库变更记录浏览器

### 1.2 用户目标与关键场景

#### 高频任务

- 搜索某类资源的审计记录
- 查看 actor、action、resource 与 metadata
- 批量导出审计结果
- 跳回相关 session

#### 关键场景

- 追查某个资源何时被谁修改
- 在合规或复盘场景中导出一批操作审计
- 从 session 关联线索回看对应操作链路

### 1.3 状态模型

- `loaded`
- `empty`
- `selected`
- `exported`

#### 展示优先级

1. 是否有结果
2. actor、action、resource 是否清楚
3. 是否可导出

### 1.4 核心字段与层级

#### L1 默认字段

- `created_at`
- `resource_type`
- `resource_id`
- `action`
- `actor`

#### L2 条件字段

- `session link`

#### L3 高级字段

- sort
- filters
- export

#### L4 系统隐藏字段

- raw metadata

#### L5 运行诊断字段

- export error
- query error

### 1.5 关键规则与约束

- `/audit` 是正式审计检索页
- `Sessions Detail` 展示关联 audit trace 子集，但不替代 audit inventory 主入口
- raw metadata 默认隐藏，不应主导日常审计浏览体验
- Audit 讲的是“可追责的操作记录”，不是一般运行日志

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 搜索特定资源类型或资源 id 的审计记录
- 按 actor 或 action 过滤结果
- 导出当前结果用于审计或复盘
- 从记录跳转到相关 session

#### 首屏必须回答的 3 个问题

1. 当前有哪些审计记录与我关心的资源或操作相关
2. 谁在什么时候做了什么
3. 我能否导出结果或跳回相关 session

### 2.2 入口与页面归属

#### `/audit`

为正式审计检索页，负责：

- 搜索
- 筛选
- 批量导出
- 列表浏览

#### `Sessions Detail`

展示关联 audit trace 子集，作为上下文补充，不承担完整审计检索职责。

### 2.3 页面结构

推荐结构：

1. Section title
2. Filters
3. Bulk export
4. Stats
5. Data table
6. Pagination

页面默认先回答“谁对什么对象做了什么”，而不是先暴露 raw metadata。

### 2.4 CTA 与操作层级

#### 主动作

- `导出结果`
- `查看关联 Session`

#### 次级动作

- `筛选`
- `排序`
- `清空筛选`

#### 高级动作

- `查看原始元数据`
- `查看查询错误`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在筛选、排序和导出工具区
- L4/L5 不应默认占据列表首屏
- raw metadata 与错误信息进入补充区块

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Audit`
- 对象名：`Audit Record`

#### 页面叙事

- 页面讲“操作审计记录”
- 不讲“日志搜索”
- 不把 Audit 讲成 Session 的附属时间线

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Audit`
- 副标题应表达：搜索平台操作记录并追查 actor、action 与 resource

### 3.3 CTA 文案

主路径默认使用：

- `导出结果`
- `查看关联 Session`

次级路径默认使用：

- `筛选`
- `排序`
- `清空筛选`

高级区允许：

- `查看原始元数据`
- `查看查询错误`

### 3.4 状态文案

#### 无记录

- 结论：`当前没有匹配的审计记录`
- 细节：请调整筛选条件，或稍后再试

#### 导出失败

- 结论：`导出未完成`
- 细节：请查看失败原因并重试
- 动作：`查看查询错误`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- raw metadata field dump
- 数据库内部审计表字段名
- 把 Audit 讲成“日志搜索”

这些内容可留在高级区，不应主导 Audit 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/audit` 已清晰表达为操作审计检索页
- 页面默认先给 actor、action、resource 与时间，而不是 raw metadata
- session 关联清晰，但 session 页面未反噬 audit 主入口

### 4.2 交互级验收

- 用户能按资源、actor、action 快速筛选记录
- 用户能导出当前结果
- 用户能从记录跳回关联 session

### 4.3 展示级验收

- 列表至少展示 created_at、resource_type、resource_id、action、actor
- 空态和导出失败态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖筛选、导出、session 跳转和空态
- 需要浏览器或截图验收确认 Audit 默认叙事已经从“日志或元数据页”收回到“操作审计”
- 若后端尚未提供更多统计与聚合，前端不应伪装成已有完整分析能力

### 4.5 剩余限制说明

- 更高级的审计聚合、告警与合规报表可作为下一阶段增强项
- raw metadata 和更底层错误细节继续保留在高级区
