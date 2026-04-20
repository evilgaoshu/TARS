# Frontend Main Menu UI/UX Optimization Prompt

下面这段 prompt 可直接交给另一个 agent 使用。

```text
请按 `specs/00-spec-four-part-template.md` 和 `specs/00-frontend-module-ui-ux-convergence-template.md`，对 Web Console 主菜单所有模块做一轮 UI/UX 收口，并直接改代码完成优化，不要只写 review。

工作方式：
- 以真实页面效果、实际交互、当前导航结构为准
- 以 spec / PRD / technical design 为设计基线
- 如某个模块 spec 仍是旧结构，先按四段式补齐：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
- 目标是“让用户看到的页面心智正确”，不是只做 DTO / handler 对齐
- 保持现有站点视觉语言统一，不做脱离现有风格的大改版
- 中途不要停在分析，发现能前端独立修复的问题就直接修
- 如果某项后端暂不支持，前端不要伪装成已支持；改成正确展示、禁用或明确说明

必须覆盖的主菜单模块：

Runtime
- Dashboard
- Sessions
- Approvals & Runs
- Runtime Checks

AI & Delivery
- In-app Inbox
- Terminal Chat
- AI Providers
- Channels
- Notification Templates

Platform
- Connectors
- Skills
- Automations
- Extensions
- Knowledge

Governance & Signals
- Metrics
- Audit Trail
- Logs
- Governance Rules
- Outbox Rescue
- Settings (Ops)

Identity & Org
- Identity
- Agent Roles
- Tenants

每个模块都必须检查并收口：
1. 导航归属、页面标题、分组命名是否符合 spec
2. 首屏是否先回答用户任务，而不是先暴露实现细节
3. 列表 / 详情 / 创建 / 编辑 / 修复动作是否在正确位置
4. L1/L2/L3/L4 字段分层是否正确，是否把高级字段错误上浮
5. 表单与交互是否仍在暴露 raw string / CSV / internal token / 兼容层命名
6. empty / error / loading / degraded / disabled 状态是否清晰
7. CTA 层级、文案和页面叙事是否统一
8. 是否存在对象边界串线，例如把别的模块心智讲进当前页面

执行要求：
- 按模块分批推进，但最终把主菜单模块都覆盖到
- 优先修 P1 / P2 的用户理解与主任务问题，再做展示完善
- 能抽公共 UI 约束就抽，但不要为了抽象而抽象
- 每修一个模块，都补对应的前端测试；必要时补页面级回归测试
- 跑 `npm run test:unit`、相关定向测试、`npm run build`
- 如果某个模块涉及真实页面验证，补浏览器检查或截图验收

输出要求：
- 最后按模块汇报：修了什么、为什么这样修、哪些是前端可独立完成、哪些仍受后端限制
- 单独列出剩余 blocker
- 给出已跑测试与结果
```
