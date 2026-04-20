# TARS - Web Console 总览与全局壳规范

## 1. 目标

定义 Web Console 的全局壳、导航、主题、语言、搜索和主要文档分工。

## 2. 文档拆分

本规范作为总览，详细内容拆到：

- [40-web-console-pages.md](./40-web-console-pages.md)
- [00-nav-page-to-spec-map.md](./00-nav-page-to-spec-map.md)
- [40-web-console-runtime-dashboard.md](./40-web-console-runtime-dashboard.md)
- [40-web-console-sessions-workbench.md](./40-web-console-sessions-workbench.md)
- [40-web-console-executions-workbench.md](./40-web-console-executions-workbench.md)
- [40-web-console-chat-workbench.md](./40-web-console-chat-workbench.md)
- [40-web-console-inbox-workbench.md](./40-web-console-inbox-workbench.md)
- [40-web-console-setup-and-ops.md](./40-web-console-setup-and-ops.md)
- [40-web-console-setup-workbench.md](./40-web-console-setup-workbench.md)
- [40-web-console-ops-console.md](./40-web-console-ops-console.md)
- [40-ux-design-system.md](./40-ux-design-system.md)
- [40-ux-unified-list-bulk.md](./40-ux-unified-list-bulk.md)

## 3. 全局壳要求

- 支持 `light / dark / follow system`
- 支持 `zh-CN / en-US`
- 支持全局搜索
- 支持 Docs / 帮助统一入口
- 左侧导航按职责组织，不按底层对象堆叠

## 4. 体验优先级

Web Console 优先服务三种心智：

- `Operate`
- `Governance`
- `Ops`

同时在 `Operate` 内区分：

- runtime workbenches
- object centers

## 5. 状态与提示

页面状态要优先传达：

- 现在发生了什么
- 最可能的原因
- 下一步该做什么

避免只堆原始错误。

## 6. 统一动作模型

- 高频动作前置
- 低频动作下沉到更多菜单或高级动作
- 危险动作进一步下沉并需要明确确认

## 7. 当前重点页

- Dashboard / Sessions / Executions / Chat / Inbox
- Connectors / Skills / Extensions / Knowledge
- Identity / Agent Roles / Org
- Setup / Ops

这些页面共同决定平台是否已经从“配置台”走向“工作台”。
