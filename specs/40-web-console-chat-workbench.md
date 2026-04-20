# TARS — Chat Workbench 规范

> **状态**: 设计基线
> **适用范围**: `/chat` 第一方 Web Chat 工作台
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-channels-and-web-chat.md](./20-component-channels-and-web-chat.md)、[40-web-console-sessions-workbench.md](./40-web-console-sessions-workbench.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Chat Workbench 是什么

`Chat Workbench` 是 `Channel.kind = web_chat` 的第一方会话入口工作台。

它承接用户发起自然语言运维请求、获得会话 ack，并在对话中继续推进 session。

#### Chat Workbench 不是什么

- 不是 Channel 配置页
- 不是普通 IM 客户端
- 不是 session 详情页的浅壳

### 1.2 用户目标与关键场景

#### 高频任务

- 发送自然语言运维请求
- 补充 host / service 上下文
- 查看 ack 与生成的 session
- 处理 chat 中出现的 pending execution

#### 关键场景

- 用户从第一方 Web 入口快速发起诊断会话
- 在聊天中补充上下文并拿到 session 级响应
- 当会话里出现待审批执行时，在对话里继续推进而不必先跳出工作流

### 1.3 状态模型

- `sending`
- `acknowledged`
- `duplicated`
- `error`

#### 展示优先级

1. 当前消息是否发送成功
2. 是否生成或命中了 session
3. 是否有 pending execution 需要处理

### 1.4 核心字段与层级

#### L1 默认字段

- `message`
- `host`
- `service`
- `session ack`

#### L2 条件字段

- `duplicated`
- `pending execution bar`
- `recent chat sessions`

#### L3 高级字段

- `workflow explanation`

#### L4 系统隐藏字段

- channel config

#### L5 运行诊断字段

- send error
- ack diagnostics

### 1.5 关键规则与约束

- `/chat` 是工作台，不是 Web Chat 的配置入口
- `/channels` 承接 `web_chat` 配置和入口治理
- Chat Workbench 讲的是“发起和延续会话”，不是“管理 channel”
- pending execution 应作为对话内上下文能力出现，而不是把用户强制踢出到其它配置页

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 发起一次新的自然语言请求
- 为请求补充 host / service 线索
- 确认消息是否被接受并转成 session
- 从最近会话继续接力

#### 首屏必须回答的 3 个问题

1. 我能不能直接从这里发起运维会话
2. 这条消息是否已成功被系统接收，并生成了什么 session
3. 如果出现 pending execution 或 recent sessions，我下一步去哪里

### 2.2 入口与页面归属

#### `/chat`

作为第一方 Web Chat 工作台，负责：

- 输入消息
- 获取 ack
- 查看最近会话
- 继续对话或衔接 session

#### `/channels`

负责：

- `web_chat` 是否开放
- 入口路径与访问限制
- Channel 级能力与治理

Chat Workbench 不承担 Channel 配置职责。

### 2.3 页面结构

推荐结构：

1. Hero
2. Chat panel
3. Recent sessions
4. How it works

页面默认先回答“如何发起会话、当前是否发送成功、最近有哪些可续接的会话”，而不是先讲 channel 或 session internals。

### 2.4 CTA 与操作层级

#### 主动作

- `发送`
- `继续会话`

#### 次级动作

- `补充主机`
- `补充服务`
- `查看最近会话`

#### 高级动作

- `查看发送诊断`
- `前往 Session 详情`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在辅助说明区
- L4/L5 不应默认占据主聊天区
- send error 与 ack diagnostics 进入局部错误提示或补充区

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Chat`
- 页面名：`Chat Workbench`

#### 页面叙事

- 页面讲“第一方会话入口”
- 不讲“Web Chat 配置”
- 不把 Chat Workbench 讲成通用聊天软件

### 3.2 页面标题与副标题

#### 页面标题

- 标题：`Chat`
- 副标题应表达：发起新的运维会话并继续最近的诊断对话

### 3.3 CTA 文案

主路径默认使用：

- `发送`
- `继续会话`

次级路径默认使用：

- `补充主机`
- `补充服务`
- `查看最近会话`

高级区允许：

- `查看发送诊断`
- `前往 Session 详情`

### 3.4 状态文案

#### 无消息

- 结论：`从这里开始一次新的运维会话`
- 细节：先描述问题，再补充主机或服务上下文

#### send 失败

- 结论：`消息发送失败`
- 细节：当前请求未成功进入会话链路，请检查网络或入口状态
- 动作：`重试发送`

#### 无 recent sessions

- 结论：`还没有最近会话`
- 细节：发送第一条消息后，最近会话会出现在这里

#### duplicated

- 结论：`系统检测到相似请求`
- 细节：可继续已有 session，也可确认创建新的会话
- 动作：`继续已有会话`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- channel config
- raw ack payload
- Web Chat adapter 术语
- 把 Chat 讲成普通 IM 客户端

这些内容可留在高级区，不应主导 Chat Workbench 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/chat` 已清晰表达为第一方会话入口工作台
- 页面默认先给消息输入、ack、最近会话和 pending execution 上下文
- `/chat` 与 `/channels` 的边界清晰

### 4.2 交互级验收

- 用户能顺畅发起消息并看到 session ack
- 用户能从 recent sessions 继续对话
- pending execution 的出现不会把用户直接扔进配置视角

### 4.3 展示级验收

- 主区至少展示消息输入、host/service 补充、ack 和 recent sessions
- 无消息、send 失败、无 recent sessions、duplicated 等状态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖发送、ack、recent sessions 和关键错误态
- 需要浏览器或截图验收确认 Chat 默认叙事已经从“配置或普通 IM”收口为“第一方会话入口”
- 若后端尚未提供更丰富的 chat diagnostics，前端不应伪装成已有完整会话调试能力

### 4.5 剩余限制说明

- Channel 级配置、入口开放与治理继续由 `/channels` 承接
- 更深的 ack 诊断和入口适配问题继续保留在高级区或 `Ops`
