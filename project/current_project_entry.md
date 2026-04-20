# 当前项目文档入口

> 这是一页“在 `project/` 里先看什么”的入口文档。  
> 它的目标不是替代 PRD / TSD / WBS，而是告诉新同学或新 agent 哪些文档是**当前基线**，哪些更适合作为**历史回溯**。

## 先看什么

### 你想先理解产品边界

- [tars_prd.md](./tars_prd.md)
- [incident-copilot-focus.md](./incident-copilot-focus.md)

### 你想先理解系统设计

- [tars_technical_design.md](./tars_technical_design.md)

### 你想知道“现在优先做什么”

- [../docs/operations/current_high_priority_workstreams.md](../docs/operations/current_high_priority_workstreams.md)

### 你想看任务基线

- [tars_dev_tasks.md](./tars_dev_tasks.md)

## 各文档现在分别承担什么职责

### `tars_prd.md`

产品边界、目标场景、一等 connector、路线收口都以它为准。

### `incident-copilot-focus.md`

这是当前阶段的聚焦版产品定义。  
适合回答“如果不想再做大而全，TARS 现在到底应该围绕什么赢”。

### `tars_technical_design.md`

系统结构、核心数据流、模块职责、运行约束以它为准。

### `tars_dev_tasks.md`

这是**任务基线 / WBS**。  
适合回答“这类工作原本怎么拆”“哪些任务属于同一条主线”。

### `tars_dev_tracker.md`

这是**历史执行跟踪**。  
信息量很大，适合查“某件事什么时候做过、远端验收过什么、踩过什么坑”，但不适合作为新 agent 的第一入口。

### `tars_frontend_tasks.md`

这是前端专项任务基线。  
适合当前端工作需要对照历史拆分和专项 backlog 时再读。

## 当前建议阅读顺序

1. [tars_prd.md](./tars_prd.md)
2. [incident-copilot-focus.md](./incident-copilot-focus.md)
3. [tars_technical_design.md](./tars_technical_design.md)
4. [../docs/operations/current_high_priority_workstreams.md](../docs/operations/current_high_priority_workstreams.md)
5. [tars_dev_tasks.md](./tars_dev_tasks.md)

如果确实需要回溯历史，再看：

5. [tars_dev_tracker.md](./tars_dev_tracker.md)
6. [tars_frontend_tasks.md](./tars_frontend_tasks.md)

## 不建议从哪里开始

- 不建议一上来先读 [tars_dev_tracker.md](./tars_dev_tracker.md)
  - 它很有价值，但信息密度太高，更像项目日志。
- 不建议把 [tars_frontend_tasks.md](./tars_frontend_tasks.md) 当成全项目总入口
  - 它是前端专项，不是整体主线。

## 相关入口

- 项目文档索引：[README.md](./README.md)
- 文档总入口：[../docs/README.md](../docs/README.md)
- 当前高优先级主线：[../docs/operations/current_high_priority_workstreams.md](../docs/operations/current_high_priority_workstreams.md)
