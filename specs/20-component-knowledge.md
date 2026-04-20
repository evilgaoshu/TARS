# TARS — Knowledge 规范

> **状态**: 设计基线
> **适用范围**: knowledge records inventory、检索、导出、session 关联
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[40-web-console-sessions-workbench.md](./40-web-console-sessions-workbench.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Knowledge 是什么

`Knowledge` 是 **知识记录 inventory 对象域**，用于沉淀诊断过程中的可检索知识资产。

它默认回答：

- 这条知识记录是什么
- 来自哪个 session
- 最近何时更新
- 是否可以导出或回看

#### Knowledge 不是什么

- 不是通用文档中心
- 不是 session 详情页的附属碎片
- 不是原始向量索引或底层 embedding 调试面板

### 1.2 用户目标与关键场景

#### 高频任务

- 浏览知识记录
- 按标题、session、更新时间搜索
- 批量导出记录
- 跳回关联 session

#### 关键场景

- 从历史诊断中回看已沉淀的知识结论
- 快速定位某个 session 生成过哪些知识记录
- 批量导出知识资产给外部团队复核或归档

### 1.3 状态模型

- `loaded`
- `empty`
- `selected`
- `exported`

#### 展示优先级

1. 是否有记录
2. 记录标题与更新时间
3. 与 session 的关联
4. 当前导出状态

### 1.4 核心字段与层级

#### L1 默认字段

- `title`
- `document_id`
- `updated_at`
- `session_id`

#### L2 条件字段

- `summary`

#### L3 高级字段

- export selection
- sort mode

#### L4 系统隐藏字段

- raw content

#### L5 运行诊断字段

- export errors

### 1.5 关键规则与约束

- `/knowledge` 是正式 inventory 页
- `Sessions Detail` 消费单条 knowledge trace，但不替代 knowledge inventory 主入口
- 原始内容和底层检索细节默认隐藏，不应主导日常浏览体验

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 找到某条知识记录
- 判断它来自哪个 session、最近是否更新
- 导出一批记录
- 跳回 session 看完整诊断上下文

#### 首屏必须回答的 3 个问题

1. 当前有多少知识记录，最近更新了什么
2. 我能否按标题、session 或更新时间快速找到目标记录
3. 这条记录对应哪个 session，是否可导出

### 2.2 入口与页面归属

#### `/knowledge`

作为正式 inventory 页，负责：

- summary stats
- 搜索 / 排序
- 批量导出
- 记录浏览

#### `Sessions Detail`

负责消费单条 knowledge trace，提供上下文回链，不承担 knowledge inventory 主配置职责。

### 2.3 页面结构

推荐结构：

1. Summary stats
2. Bulk export
3. Search / sort
4. Table
5. Pagination

列表页默认先回答“有哪些知识记录、最近更新了什么、能否快速定位”，而不是先暴露 raw content。

### 2.4 CTA 与操作层级

#### 主动作

- `导出所选`
- `查看关联 Session`

#### 次级动作

- `搜索`
- `排序`
- `清空选择`

#### 高级动作

- `查看原始内容`
- `查看导出错误`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在批量导出和排序控制区
- L4/L5 不应默认占据列表首屏
- raw content 与导出错误进入补充区块

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Knowledge`
- 对象名：`Knowledge Record`

#### 页面叙事

- 页面讲“知识沉淀资产”
- 不讲“原始检索库”
- 不把 Knowledge 讲成 Session 详情里的附属表格

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Knowledge`
- 副标题应表达：浏览诊断过程沉淀的知识记录并回看关联 session

### 3.3 CTA 文案

主路径默认使用：

- `导出所选`
- `查看关联 Session`

次级路径默认使用：

- `搜索`
- `排序`
- `清空选择`

高级区允许：

- `查看原始内容`
- `查看导出错误`

### 3.4 状态文案

#### 无记录

- 结论：`当前还没有知识记录`
- 细节：可先运行 diagnosis 流程，知识会在诊断过程中沉淀
- 动作：`前往 Sessions`

#### 导出失败

- 结论：`当前导出未完成`
- 细节：请查看失败原因并重试
- 动作：`查看导出错误`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- raw content payload
- 向量索引或 embedding 调试术语
- 把 Knowledge 讲成底层检索引擎输出

这些内容可留在高级区，不应主导 Knowledge 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/knowledge` 已清晰表达为知识记录 inventory
- 页面默认先给统计、搜索、导出和列表，而不是 raw 内容
- session 关联清晰，但 session 页面未反噬 knowledge 主入口

### 4.2 交互级验收

- 用户能按标题、session、更新时间快速筛选记录
- 用户能选择并导出多条记录
- 用户能顺畅跳回关联 session

### 4.3 展示级验收

- 列表至少展示 title、document_id、updated_at、session_id
- 空态和导出失败态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖搜索、导出、session 跳转和空态
- 需要浏览器或截图验收确认 Knowledge 默认叙事已经从“底层检索结果”收回到“知识记录 inventory”
- 若后端尚未提供更多摘要字段，前端不应伪装成已有丰富内容层

### 4.5 剩余限制说明

- 更细的知识分类、标签和回放能力可作为下一阶段增强项
- 原始内容和检索调试细节继续保留在高级区或诊断区
