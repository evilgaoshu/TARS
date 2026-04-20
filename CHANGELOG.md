# Changelog

> 所有显著的变更都将记录在此文件中。
> 格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
> 版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

---

## [Unreleased]

### Added
- 新增知识沉淀功能，支持将闭环会话自动转换为知识记录
- 新增 Webhook 集成指南文档
- 新增安全加固指南文档
- 新增数据库 Schema 文档
- 新增配置参考手册

### Changed
- 优化 AI 诊断响应速度
- 改进脱敏算法，支持更多敏感信息模式

### Fixed
- 修复 Telegram 消息编辑超时问题
- 修复 SSH 连接偶发超时问题

---

## [1.0.0] - 2026-03-13

### Added - MVP 正式发布

#### 核心功能
- **告警接收**: 支持 VMAlert Webhook 接收告警
- **AI 诊断**: 基于 OpenAI 兼容接口的智能诊断建议
- **审批流程**: 支持单人/双人审批机制
- **命令执行**: SSH 远程命令执行
- **Telegram 集成**: 完整的 Telegram Bot 交互
- **知识库**: 文档向量化存储和检索

#### API
- 完整的 RESTful API 接口
- Ops API 用于管理操作
- Webhook 接口用于告警接收
- 健康检查和监控接口

#### Web Console
- 会话管理和查看
- 执行记录查询
- 审批操作界面
- 系统状态监控

#### 配置管理
- 环境变量配置
- YAML 配置文件
- 授权策略配置
- 审批路由配置
- Provider 配置
- 脱敏规则配置

#### 安全特性
- Bearer Token 认证
- Webhook 签名验证
- 命令白名单/黑名单
- 风险分级审批
- 数据脱敏
- 审计日志

### Documentation
- 用户使用手册
- 部署手册
- 管理员手册
- API 参考文档
- 配置参考手册
- 数据库 Schema 文档
- Webhook 集成指南
- 安全加固指南
- 故障排查手册
- 开发贡献指南

---

## [0.9.0] - 2026-03-01

### Added
- 新增知识沉淀基础功能
- 新增批量操作 API
- 新增导出功能

### Changed
- 重构执行模块，支持更灵活的 Provider 扩展
- 优化数据库查询性能

### Fixed
- 修复高并发下的竞态条件
- 修复 Telegram 回调处理问题

---

## [0.8.0] - 2026-02-15

### Added
- 新增 Alertmanager V2 API 兼容
- 新增 Provider 健康检查
- 新增配置热重载

### Changed
- 改进审批消息格式
- 优化会话状态机

### Fixed
- 修复会话状态转换异常
- 修复输出截断问题

---

## [0.7.0] - 2026-02-01

### Added
- 新增 SSH 命令执行
- 新增命令授权策略
- 新增审批路由配置

### Changed
- 重构 Action Gateway 模块
- 改进错误处理

---

## [0.6.0] - 2026-01-15

### Added
- 新增审批流程
- 新增双人审批机制
- 新增审批超时处理

### Changed
- 改进 Telegram 交互体验
- 优化消息格式

---

## [0.5.0] - 2026-01-01

### Added
- 新增 AI 诊断功能
- 新增模型 Provider 支持
- 新增脱敏功能

### Changed
- 重构 Reasoning 模块
- 优化上下文组装

---

## [0.4.0] - 2025-12-15

### Added
- 新增 Telegram 渠道适配
- 新增 Webhook 接收
- 新增轮询模式

### Changed
- 重构 Channel Adapter 模块

---

## [0.3.0] - 2025-12-01

### Added
- 新增会话管理
- 新增执行请求管理
- 新增审计日志

### Changed
- 重构 Workflow Core 模块

---

## [0.2.0] - 2025-11-15

### Added
- 新增告警接收
- 新增 VMAlert 集成
- 新增告警去重

### Changed
- 重构 Alert Intake 模块

---

## [0.1.0] - 2025-11-01

### Added
- 项目初始化
- 基础架构搭建
- PostgreSQL 集成
- SQLite-vec 集成
- 基础 HTTP API

---

## 版本号说明

### 格式
```
主版本号.次版本号.修订号
```

### 版本递增规则

- **主版本号**：不兼容的 API 修改
- **次版本号**：向下兼容的功能新增
- **修订号**：向下兼容的问题修复

### 预发布版本

- `1.0.0-alpha` - 内部测试版
- `1.0.0-beta` - 公开测试版
- `1.0.0-rc.1` - 候选发布版 1

---

## 迁移指南

### 从 0.x 升级到 1.0

#### 配置变更

```yaml
# 旧配置 (0.x)
model:
  base_url: "..."
  api_key: "..."

# 新配置 (1.0)
providers:
  primary:
    provider_id: "primary"
    model: "gpt-4o-mini"
  entries:
    - id: "primary"
      base_url: "..."
      api_key: "..."
```

#### 数据库迁移

```sql
-- 执行迁移脚本
psql -d tars -f migrations/postgres/0002_upgrade_v1.sql
```

#### API 变更

| 旧端点 | 新端点 | 变更说明 |
|--------|--------|----------|
| `/api/v1/sessions/{id}/approve` | `/api/v1/executions/{id}/approve` | 审批端点独立 |
| `/api/v1/config/model` | `/api/v1/config/providers` | 配置结构调整 |

---

## 兼容性说明

### API 兼容性

| 版本 | 状态 | 说明 |
|------|------|------|
| 1.0 | Current | 当前版本，完全支持 |
| 0.9 | Deprecated | 建议升级，即将停止支持 |
| 0.8 | End of Life | 不再支持 |

### 数据库兼容性

| 版本 | 最低 PostgreSQL | 最低 SQLite |
|------|----------------|-------------|
| 1.0 | 14+ | 3.35+ |
| 0.9 | 13+ | 3.30+ |

### 浏览器兼容性 (Web Console)

| 浏览器 | 最低版本 |
|--------|----------|
| Chrome | 90+ |
| Firefox | 88+ |
| Safari | 14+ |
| Edge | 90+ |

---

## 已知问题

### 1.0.0

- **Issue #123**: Telegram 消息偶尔重复发送
  - 影响：低
  - 解决：将在 1.0.1 修复

- **Issue #124**: 大输出执行可能导致内存峰值
  - 影响：中
  - 解决：限制并发执行数量

---

## 即将废弃的功能

### 1.1.0 (计划)

- `TARS_MODEL_BASE_URL` 环境变量 → 使用 `TARS_PROVIDERS_CONFIG_PATH`
- `/api/v1/config/model` API → 使用 `/api/v1/config/providers`

---

## 致谢

感谢所有为 TARS 做出贡献的开发者：

- [贡献者列表](https://github.com/xxx/tars/graphs/contributors)

---

## 参考

- [Semantic Versioning 2.0.0](https://semver.org/)
- [Keep a Changelog](https://keepachangelog.com/)

---

*本 Changelog 最后更新于 2026-03-13*
