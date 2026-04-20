# TARS — 平台依赖、兼容性与部署要求规范

> **状态**: Next Phase 设计基线  
> **适用范围**: 平台依赖治理、版本与漏洞管理、兼容性矩阵、部署基线  
> **定位**: 为企业级交付、升级、发布和运维治理提供统一约束

---

## 1. 目标

随着 TARS 逐步变成平台，后续不能只知道“系统能跑”，还需要回答：

- 平台依赖了哪些组件
- 当前版本是多少
- 是否有可升级版本
- 是否存在已知安全漏洞
- 与哪些第三方系统/版本兼容
- 部署至少需要什么硬件、操作系统和网络条件

这些信息必须进入正式文档与后续控制面，而不是继续依赖口口相传。

---

## 2. 平台依赖管理

### 2.1 依赖范围

平台依赖管理后续至少应覆盖：

- `Application Dependencies`
  - Go modules
  - npm packages
  - 构建工具链
- `Runtime Dependencies`
  - PostgreSQL
  - 向量库/嵌入库
  - 模型供应商与模型网关
  - Telegram / 渠道 SDK 与 API
- `Platform Components`
  - Connectors
  - Skills
  - Providers
  - Channels
  - Auth Providers
- `External Systems`
  - VictoriaMetrics / Prometheus
  - JumpServer
  - GitHub / GitLab / CI/CD
  - 目录服务 / OIDC / LDAP

### 2.2 最低能力

后续平台至少应具备：

- 依赖清单（inventory）
- 当前版本
- 可升级版本
- 是否有安全漏洞
- 是否存在 break change 风险
- 关联影响面（哪个模块/页面/连接器依赖它）

### 2.3 建议展示字段

每条依赖后续建议至少展示：

- `name`
- `category`
- `current_version`
- `latest_version`
- `update_available`
- `security_status`
- `advisory_refs`
- `owner`
- `impact_scope`
- `last_checked_at`

### 2.4 风险等级建议

建议统一风险级别：

- `ok`
- `update_available`
- `security_warning`
- `security_critical`
- `deprecated`
- `unsupported`

---

## 3. 兼容性列表

### 3.1 为什么需要独立兼容性矩阵

TARS 后续会接越来越多第三方系统，不能只写“理论支持”。

后续应明确区分：

- `supported`
- `verified`
- `experimental`
- `deprecated`
- `unsupported`

### 3.2 兼容性矩阵应覆盖的对象

至少应覆盖：

- `Connectors`
  - 支持哪些第三方系统
  - 支持哪些版本区间
- `Providers`
  - 支持哪些模型供应商协议
  - 支持哪些模型/API 版本
- `Channels`
  - 支持哪些渠道平台与消息能力
- `Authentication`
  - 支持哪些 OIDC / OAuth / LDAP / SAML 供应商
- `Deployment`
  - 支持哪些 OS / CPU / 架构 / 数据库版本

### 3.3 兼容性矩阵建议字段

后续每条兼容性记录建议至少包含：

- `component_type`
- `component_id`
- `target_system`
- `target_version_range`
- `compatibility_level`
- `verified_in`
- `notes`
- `last_verified_at`

### 3.4 兼容性示例

例如：

- `victoriametrics-main`
  - target: `VictoriaMetrics`
  - range: `v1.x`
  - level: `verified`
- `jumpserver-main`
  - target: `JumpServer`
  - range: `3.x / 4.x`
  - level: `supported`
- `telegram-main`
  - target: `Telegram Bot API`
  - range: `current cloud API`
  - level: `supported`

---

## 4. 部署要求

### 4.1 必须明确的最低要求

企业交付时，平台必须明确至少这些部署要求：

- `CPU Architecture`
  - `x86_64`
  - 后续是否支持 `arm64`
- `Operating System`
  - Linux 发行版与最低版本
- `Memory`
  - 最低 / 推荐内存
- `Disk`
  - 二进制与 Web 静态资源
  - 数据库存储
  - 执行输出 / 日志 / 附件
- `Network`
  - 出站访问需求
  - 入站端口
  - 内外网依赖

### 4.2 建议部署基线

后续至少应定义：

- `Dev / Test`
- `Shared Environment`
- `Production Small`
- `Production Medium`
- `Production HA`

每档至少说明：

- CPU
- 内存
- 磁盘
- PostgreSQL 要求
- 网络连通性
- 是否需要公网模型访问
- 是否需要对象存储/外部日志系统

### 4.3 网络要求

至少需要明确：

- Web/Ops 入口
- Public API / webhook 入口
- 模型供应商出站访问
- 第三方系统出站访问
- Telegram / Feishu / Slack 等渠道出站访问
- JumpServer / SSH / 内部系统访问要求

### 4.4 资源要求不应只写“能跑”

后续文档与控制面应区分：

- `minimum`
- `recommended`
- `production baseline`

避免部署文档只有“本地能启动”的要求，而没有生产建议。

---

## 5. 控制面方向

后续控制面应补齐至少 3 类能力：

### 5.1 Dependency Center

展示：

- 依赖清单
- 当前版本
- 是否可升级
- 是否存在安全风险

### 5.2 Compatibility Center

展示：

- 平台组件和第三方系统的兼容性矩阵
- 已验证版本
- 风险等级

### 5.3 Deployment Requirements

展示：

- 支持架构
- 最低/推荐资源
- 外部依赖
- 网络要求

---

## 6. 与现有平台组件的关系

依赖、兼容性、部署要求并不是独立于平台组件之外的“附录信息”，而是：

- Connectors 的运行边界
- Providers 的协议边界
- Channels 的能力边界
- Authentication 的企业接入边界
- Deployment 的基础设施边界

因此后续应把它们做成：

- 文档真值
- API 真值
- 控制面可视

而不是只停留在 README。

---

## 7. 一句话结论

后续企业级 TARS 不只需要“平台对象 CRUD”，还必须补齐：

- 依赖管理
- 兼容性矩阵
- 部署要求

这三者共同决定平台是否可升级、可交付、可运维、可合规。
