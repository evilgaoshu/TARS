# 平台依赖、兼容性与部署要求规范

> **状态**: Next Phase 设计基线  
> **适用范围**: 平台依赖治理、版本与漏洞管理、兼容性矩阵、部署基线  
> **定位**: 为企业级交付、升级、发布和运维治理提供统一约束

## 1. 目标

随着 TARS 逐步变成平台，后续不能只知道“系统能跑”，还需要回答：
- 平台依赖了哪些组件
- 当前版本是多少
- 是否有可升级版本
- 是否存在已知安全漏洞
- 与哪些第三方系统/版本兼容
- 部署至少需要什么硬件、操作系统和网络条件

## 2. 平台依赖管理

### 2.1 依赖范围
平台依赖管理后续至少应覆盖：
- **应用依赖**: Go modules, npm packages, 构建工具链。
- **运行时依赖**: PostgreSQL, 向量库, 模型供应商, Telegram API。
- **平台组件**: Connectors, Skills, Providers, Channels, Auth Providers。
- **外部系统**: VictoriaMetrics, JumpServer, GitHub, LDAP/OIDC。

## 3. 兼容性列表

### 3.1 兼容性级别
- `supported`: 理论支持。
- `verified`: 已验证。
- `experimental`: 实验性。
- `deprecated`: 已弃用。
- `unsupported`: 不支持。

## 4. 部署要求

### 4.1 硬件要求
- **CPU**: 2 核 (x86_64) 起步。
- **内存**: 4 GB 起步。
- **磁盘**: 20 GB 起步。

### 4.2 网络要求
- 需要明确 Web/Ops 入口、Public API 入口、模型供应商出站、第三方系统出站等访问权限。
