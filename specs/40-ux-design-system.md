# TARS — Web 设计系统与 UI/UX 规范

> 关联总览：[40-web-console.md](./40-web-console.md)、[40-ux-frontend-optimization-workflow.md](./40-ux-frontend-optimization-workflow.md)、[00-spec-four-part-template.md](./00-spec-four-part-template.md)

## 1. 目标

建立统一的设计系统、页面骨架和交互语言，让 TARS 更像成熟平台，而不是不同页面各长一套后台样式。

## 2. 技术栈基线

- 以现有 React + Tailwind + shadcn/ui 风格体系为主
- 优先复用现有组件与成熟开源能力
- 不重复造轮子，不引入第二套设计系统

## 3. 组件层级

### 通用 UI 基础组件

- Button
- Input / Textarea / Select
- Dialog / Sheet / DropdownMenu
- Badge / Alert / Tooltip
- Table / List shell / Status badge

### 页面公共组件层

- PageShell
- FilterBar
- SummaryGrid
- SplitLayout
- Empty / Error / Success state

## 4. App Shell

- Header、Sidebar、Breadcrumb、Global Search、Theme、Language switch 必须走统一骨架
- 不允许页面自己再长“伪全局”入口

## 5. 视觉语言

- 层次清楚、留白稳定、状态分层明确
- 结论、证据、动作有明显主次
- 机器数据与对象名可使用 Mono / muted 背景增强可读性

## 6. 交互节奏

- 主操作前置，低频动作下沉
- 尽量减少“跳到新页面再返回列表”的上下文切换
- 对高频审阅类页面，允许使用 split view / master-detail

## 7. 设计系统治理

- 页面不应各自维护一套按钮、表格、状态文案
- 新页面优先复用现有 page shell 和列表/详情骨架
- 样式重构以统一性为第一目标，而不是局部炫技
- 模块级优化流程遵循 [40-ux-frontend-optimization-workflow.md](./40-ux-frontend-optimization-workflow.md)
