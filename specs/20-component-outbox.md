# TARS — Outbox 规范

> **状态**: 设计基线
> **适用范围**: failed / blocked delivery queue、replay、delete、批量处理
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-channels-and-web-chat.md](./20-component-channels-and-web-chat.md)、[40-web-console-ops-console.md](./40-web-console-ops-console.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Outbox 是什么

`Outbox` 是 **送达失败与阻塞事件修复对象域**。

它承接的是已经完成对象解析后的 materialized delivery residue，例如某条消息在目标渠道送达失败、阻塞或等待重放。

#### Outbox 不是什么

- 不是 Channel 配置页
- 不是 raw transport debug console
- 不是平台级 queue repair / emergency disable 面板

### 1.2 用户目标与关键场景

#### 高频任务

- 查看 failed / blocked delivery events
- 手工 replay 单条或批量 replay
- 删除历史残留事件

#### 关键场景

- 排查为什么消息没有真正送达
- 快速恢复一批可重放的通知事件
- 清理已经无意义的历史送达残留

### 1.3 状态模型

- `failed`
- `blocked`
- `replayed`
- `deleted`

#### 展示优先级

1. 当前是否失败或阻塞
2. 阻塞原因或最近错误
3. 事件年龄与是否适合重放

### 1.4 核心字段与层级

#### L1 默认字段

- event id
- topic
- status
- age

#### L2 条件字段

- blocked reason
- last error

#### L3 高级字段

- bulk actions
- sort
- filter

#### L4 系统隐藏字段

- materialized delivery payload

#### L5 运行诊断字段

- replay error detail
- delete error detail

### 1.5 关键规则与约束

- `/outbox` 是 delivery repair console
- `/channels` 负责渠道配置，不负责已失败残留的主修复
- `/ops` 负责 raw queue / transport repair、reindex 与 emergency actions
- Outbox 应始终讲“已物化的送达残留”，不应回退成原始 transport 调试页

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 找到送达失败或阻塞的事件
- 判断它为什么失败、是否值得重放
- 对单条或多条事件执行 replay
- 删除不再需要的残留项

#### 首屏必须回答的 3 个问题

1. 当前有哪些送达残留事件需要我处理
2. 它们为什么失败或阻塞
3. 我应该重放、删除，还是跳去 `Ops`

### 2.2 入口与页面归属

#### `/outbox`

作为正式修复台，负责：

- failed / blocked inventory
- replay / bulk replay
- delete / bulk delete
- 基于错误的快速判断

#### `/channels`

负责渠道配置和验证，不承担已物化送达残留的修复主入口。

#### `/ops`

负责：

- raw queue / transport repair
- reindex
- emergency actions

### 2.3 页面结构

推荐结构：

1. Title
2. Bulk actions
3. Search / filter / sort
4. Table
5. Error detail drawer
6. Pagination

列表页默认先回答“哪些送达残留需要处理、失败原因是什么、是否适合重放”，而不是先展开原始 payload。

### 2.4 CTA 与操作层级

#### 主动作

- `重放`
- `批量重放`
- `删除`

#### 次级动作

- `筛选`
- `排序`
- `查看错误详情`

#### 高级动作

- `前往 Ops`
- `查看原始负载`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在列表工具栏
- L4/L5 不应默认占据列表首屏
- raw payload 与更底层错误明细进入详情抽屉或高级区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Outbox`
- 对象名：`Delivery Residue`

#### 页面叙事

- 页面讲“送达残留修复”
- 不讲“消息队列调试”
- 不把 Outbox 讲成 Channels 的子页面

### 3.2 页面标题与副标题

#### 列表页

- 标题：`Outbox`
- 副标题应表达：处理送达失败或阻塞的残留事件

### 3.3 CTA 文案

主路径默认使用：

- `重放`
- `批量重放`
- `删除`

次级路径默认使用：

- `筛选`
- `排序`
- `查看错误详情`

高级区允许：

- `前往 Ops`
- `查看原始负载`

### 3.4 状态文案

#### 无残留事件

- 结论：`当前没有需要处理的送达残留`
- 细节：所有已物化送达事件都已处理完成或已清空

#### replay 失败

- 结论：`重放未成功`
- 细节：请查看错误原因，决定重试还是转到 `Ops`
- 动作：`查看错误详情`

#### delete 失败

- 结论：`删除未成功`
- 细节：残留事件仍保留在列表中，可稍后重试
- 动作：`重试删除`

#### 需要更底层修复

- 结论：`当前问题需要更底层修复`
- 细节：这不是单纯的送达残留问题，可能需要队列或 transport 级干预
- 动作：`前往 Ops`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- queue shard metadata
- raw transport debug console
- internal delivery engine payload naming

这些内容可留在高级区，不应主导 Outbox 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/outbox` 已清晰表达为送达残留修复台
- 页面默认先给失败状态、原因与修复动作，而不是原始 payload
- `Channels` 与 `Ops` 的边界清晰

### 4.2 交互级验收

- 用户能对单条和批量事件执行 replay / delete
- 用户能快速判断是否应跳转 `Ops`
- replay / delete 失败不会导致事件静默消失

### 4.3 展示级验收

- 列表至少展示 event id、topic、status、age、blocked reason / last error
- 空态、replay 失败、delete 失败、需前往 `Ops` 等状态都有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖列表摘要、单条/批量动作和关键错误态
- 需要浏览器或截图验收确认 Outbox 默认叙事已经从“底层队列页”收口为“修复台”
- 若后端尚未提供更丰富的错误分类，前端不应伪装成已有完整根因诊断

### 4.5 剩余限制说明

- raw queue repair、transport repair 和 emergency disable 继续留在 `Ops`
- 更深的 delivery 调试细节可继续保留在高级区或抽屉中
