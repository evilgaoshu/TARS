# Web 控制台使用手册

TARS Web 控制台是平台管理员和操作人员的中央管理界面。它提供事件响应会话、执行日志、知识库管理和平台配置的可视化。

## 1. 仪表盘 (Dashboard)

仪表盘提供平台健康状态的高层概览：
- **指标 (Metrics)**：已连接指标提供者的实时状态。
- **执行 (Executions)**：近期自动化和手动操作的摘要。
- **会话 (Sessions)**：活动的和最近的事件恢复会话。

## 2. 会话管理 (Session Management)

会话页面列出了由告警或手动冒烟测试触发的所有事件恢复工作流。
- **会话详情 (Session Detail)**：查看事件的完整追踪，包括 AI 诊断、工具计划和执行结果。
- **审计轨迹 (Audit Trail)**：平台或操作人员采取的每一项操作都会出于合规目的被记录下来。

## 3. 连接器中心 (Connector Center)

管理外部集成，例如：
- **指标连接器 (Metrics Connectors)**：Prometheus、VictoriaMetrics。
- **执行连接器 (Execution Connectors)**：JumpServer、SSH。
- **观测连接器 (Observation Connectors)**：日志系统、APM。

## 4. 技能注册表 (Skill Registry)

技能 (Skills) 是 TARS 用于解决特定问题的“剧本 (playbooks)”。
- **起草 (Drafting)**：使用内置编辑器创建新技能。
- **版本控制 (Version Control)**：将草稿晋升到生产环境或回滚到以前的版本。

## 5. 身份与访问 (Identity & Access)

管理谁可以访问平台以及他们可以做什么：
- **认证提供者 (Auth Providers)**：配置 OIDC、OAuth2 或本地 Token 访问。
- **用户和组 (Users & Groups)**：管理本地帐户和组成员身份。
- **角色和权限 (Roles & Permissions)**：定义细粒度的 RBAC 策略。

## 6. 文档中心 (Documentation Center)

直接从右上角菜单访问本指南和其他技术参考资料。
- **搜索 (Search)**：使用 `Cmd+K` 在所有内置文档中进行搜索。
- **离线支持 (Offline Support)**：即使平台断开互联网连接，也可以使用文档。
