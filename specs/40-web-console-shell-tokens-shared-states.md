# TARS - Web Console Shell、Tokens 与共享状态组件

> **状态**: EVI-25 implementation baseline
> **适用范围**: Web Console 全局壳、light/dark token、共享状态组件、页面级共享 UI 模式
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联文档**: [40-web-console.md](./40-web-console.md)、[40-ux-design-system.md](./40-ux-design-system.md)、[40-web-console-pages.md](./40-web-console-pages.md)、`docs/design/web-console-product-prototype-2026-04-28.md`

---

## 1. 功能 Spec

### 1.1 对象定义

这份规范定义 Web Console 基础 UI 层：

- 全局 App Shell
- light / dark / system tokens
- 共享状态语言与复用组件
- 页面级共享 Hero / Filter / Action / Fold 模式

它不是某一个业务页的内容层重构 spec，也不是第二套设计系统。实现必须继续基于现有 React + Tailwind + shadcn/ui 组件体系。

### 1.2 用户目标与平台目标

基础层需要先解决以下问题：

- 用户在任一主路由下都能识别统一的控制台壳与导航分组
- light / dark 主题都能维持一致的证据台视觉语言，而不是局部样式补丁
- 状态、风险、空态、错误态、降级态、禁用态使用统一语义，不再各页各自发明文案和颜色
- 原始 payload、console output、manifest、transport detail 默认折叠，不抢首屏

### 1.3 状态模型

#### 共享状态 tone

- `warning`: `open`、`pending`、`degraded`、`pending_approval`、`blocked`
- `active`: `analyzing`、`executing`、`processing`、`verifying`、`reviewing`
- `success`: `resolved`、`completed`、`healthy`、`approved`、`enabled`
- `danger`: `failed`、`rejected`、`critical`、`error`、`missing`
- `muted`: `disabled`、`offline`、`unknown`

#### 风险 tone

- `critical` / `high` -> `danger`
- `warning` / `medium` -> `warning`
- `healthy` / `low` -> `success`
- 其余未识别值 -> `muted`

#### 页面状态组件

- `empty`
- `error`
- `loading`
- `degraded`
- `disabled`

这些状态由共享状态组件统一承载，页面只传递标题、描述、可选动作和可选图标。

### 1.4 核心字段与层级

#### Shell L1 默认层

- 分组导航
- 顶部全局搜索入口
- theme / language 切换
- docs / inbox / chat 快捷入口
- 当前环境或认证来源

#### Shared Pattern L1 默认层

- Page hero 标题、副标题、主 CTA
- Filter bar 搜索与主筛选
- Action bar 的主操作与次操作
- Status badge / risk badge

#### Shared Pattern L4 默认隐藏层

- raw payload
- manifest 原文
- command / transport 细节
- full console output

这些信息必须通过折叠组件呈现，默认不展开。

### 1.5 关键规则与约束

- 不引入第二套 design system
- 不暴露 secret 明文或 secret ref 细节作为主内容
- 不把审批、高危动作、raw payload 上浮到首屏主路径
- 主要路由必须继续可访问，shell 重构不能导致白屏或导航断链
- Docs 保持主导航可达，不仅存在于 header dropdown
- 共享状态组件必须可用于 Sessions / Executions / Setup / Registry / Governance 页面，而不是为单一页面定制

---

## 2. UX Spec

### 2.1 用户任务

首屏必须先回答：

1. 当前在什么工作域
2. 当前最值得处理的对象或动作是什么
3. 如果需要更多细节，应该去哪个页面或折叠块看

### 2.2 入口与页面归属

导航分组固定为：

- Runtime
- Delivery
- Platform
- Governance
- Identity
- Docs

Docs 既保留 header dropdown，也保留左侧导航入口，避免只有二级入口才能到达文档。

### 2.3 页面结构

#### App Shell

- Desktop: 固定左侧导航 + 顶部 toolbar + 可滚动主内容区
- Mobile: sheet 导航降级，不产生页面级横向溢出
- Breadcrumb 始终位于页面内容顶部，作为内容域层级提示

#### Shared Layout Pattern

- Hero 负责结论、上下文和主 CTA
- Filter bar 负责搜索和主筛选
- Action bar 负责 list/detail 级操作，不重复做伪 header
- Raw payload fold 负责 manifest / payload / output 的折叠承载

### 2.4 CTA 与操作层级

- 主 CTA 进入 workbench 或主要处理流
- 次级 CTA 进入 docs、chat、inbox、export
- 危险动作继续下沉到 confirm dialog 或 ops repair path

### 2.5 字段分层

- `L1`: 标题、结论、状态、风险、下一步
- `L2`: 服务、主机、更新时间、对象关系
- `L3`: 调试摘要、验证输出、历史信息
- `L4`: raw payload / manifest / command / transport / console output

---

## 3. 内容 Spec

### 3.1 命名

- Shell 叙事固定为 `On-call Evidence Desk`
- 统一使用 `Documentation` / `平台文档` 作为 docs 主导航命名
- 风险 badge 使用 `critical risk`、`warning risk` 等明确表达，不混用普通状态 badge

### 3.2 页面标题与副标题

- 壳层品牌副标题只表达产品心智，不表达某个页面内容
- 页面标题仍由各业务页负责，基础层只提供承载模式

### 3.3 CTA 文案

- Header CTA 优先使用 `Action Hub`、`Open Chat`、`Open Inbox`、`Docs`
- Theme / language toggle 不出现工程化命名，如 `set dark mode`

### 3.4 状态文案

- empty: 明确说明当前没有对象，或为什么当前筛选下为空
- error: 明确说明加载失败 / API 失败，而不是伪装成 empty
- degraded: 明确说明部分可用、能力不完整
- disabled: 明确说明为什么被禁用以及是否需要审批/配置
- loading: 表达正在等待哪类基础信息，如 provider metadata / runtime state

### 3.5 术语黑名单

- 不把 raw payload 默认作为正文标题
- 不在主路径文案里突出 DTO / manifest / internal token
- 不把 secret ref、approval bypass、raw transport 作为面向操作员的默认语言

---

## 4. 验收清单

### 4.1 页面级验收

- App Shell 使用统一分组导航和顶部工具栏
- Docs 在 header dropdown 与侧边导航都可达
- light / dark 主题都使用同一套 evidence desk token 体系
- 主要路由访问不白屏、不丢失 breadcrumb 或主内容区

### 4.2 交互级验收

- Mobile 导航通过 sheet 降级，主要页面无横向溢出
- Header theme / language toggle 可切换
- Raw payload / manifest / console output 默认折叠
- 状态 badge 与风险 badge 语义分离，tone 映射稳定

### 4.3 组件级验收

- `StatusBadge` 输出共享 `data-tone`
- `RiskBadge` 输出独立风险 tone
- `StatePanel` 可承载 empty / error / loading / degraded / disabled
- `RawPayloadFold` 可复用于 payload / manifest / output 折叠区

### 4.4 守卫项

- 不新增第二套 UI framework
- 不把 raw payload 默认上浮到 Hero 或首屏摘要
- 不暴露 secret 明文
- 不改变审批与危险动作边界
