# TARS 前端协作与工程规范 (Enterprise-Grade)

> **版本**: v1.2.0 (2026-03-22)  
> **核心原则**: 逻辑抽象化、UI 标准化、反馈全量化、类型严谨化。

为了保持 TARS 平台的高 SNR (信噪比) 研发效率，所有前端开发必须遵循以下架构规范：

## 1. 逻辑抽象规范

### 1.1 数据请求与状态
- **禁止**：在 Page 级别手动写 `useState` 管理请求数据，禁止手动写 `useEffect` 触发 API。
- **强制**：列表页必须使用 `useRegistry` 钩子。它统一集成了分页、搜索、过滤和 TanStack Query 缓存逻辑。
- **详情页**：优先使用 `useQuery` 进行数据绑定，享受自动缓存与重试能力。

### 1.2 用户反馈
- **禁止**：在 Page 内自定义提示文案状态（如 `const [message, setMessage] = useState('')`）。
- **强制**：所有操作反馈必须使用 `useNotify` 钩子。
    - `notify.success("Saved")`: 成功反馈。
    - `notify.error(err)`: 自动解析 API 报错并弹出。
    - `notify.warn("Msg")`: 提示性反馈。

## 2. UI 与布局规范

### 2.1 布局模式 (Layout Patterns)
- **列表页**：统一使用 `@/components/layout/patterns/RegistryLayout`。
- **列表渲染**：强制使用 `@/components/list/DataTable`。禁止手动写 `<table>`、`<thead>` 或 `<td>` 循环。必须通过 `ColumnDef<T>` 进行配置驱动开发。
- **侧边分栏详情页**：统一使用 `@/components/layout/patterns/SplitDetailLayout`。
- **高级卡片**：统一使用 `.glass-card` 类名，禁止手动写背景透明度和模糊值。

### 2.2 原子组件
- 优先从 `@/components/ui/` 引入 Shadcn 定制化组件（如 `Button`, `Input`, `Form`）。
- 表单必须使用 `react-hook-form` + `zod` 进行 Schema 驱动开发。

## 3. 运行时稳定性

### 3.1 防御性编程
- 在进行 `.includes()`, `.map()`, `.some()` 操作前，必须进行 `null`/`undefined` 检查或使用可选链。
- 搜索过滤逻辑必须包含类型守卫，防止非字符串字段引起崩溃。

### 3.2 自动填充规避
- 所有敏感输入框必须带上 `autoComplete="off"` 或 `autoComplete="new-password"`，防止第三方插件注入导致的 DOM 树同步错误。

---

**违反上述规范的代码将无法通过 MVP Check 脚本，且在 Code Review 阶段会被 Block。**
