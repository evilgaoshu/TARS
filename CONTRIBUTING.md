# TARS 贡献指南

> **版本**: v1.0
> **最后更新**: 2026-03-13

感谢您对 TARS 项目的关注！本文档将指导您如何参与项目开发。

---

## 目录

1. [开发环境搭建](#1-开发环境搭建)
2. [代码规范](#2-代码规范)
3. [项目结构](#3-项目结构)
4. [开发流程](#4-开发流程)
5. [测试要求](#5-测试要求)
6. [提交规范](#6-提交规范)
7. [PR 流程](#7-pr-流程)
8. [发布流程](#8-发布流程)

---

## 1. 开发环境搭建

### 1.1  prerequisites

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | 1.22+ | 后端开发 |
| Node.js | 18+ | 前端开发 |
| PostgreSQL | 14+ | 数据库 |
| SQLite | 3+ | 向量存储 |
| Make | - | 构建工具 |
| Docker | 20+ | 容器化部署 |

### 1.2 克隆代码

```bash
git clone <repository-url>
cd TARS
```

### 1.3 安装依赖

```bash
# Go 依赖
go mod download

# 前端依赖
cd web && npm install
```

### 1.4 配置开发环境

```bash
# 复制示例配置
cp configs/tars.example.yaml configs/tars.yaml
cp configs/approvals.example.yaml configs/approvals.yaml

# 设置环境变量
export TARS_POSTGRES_DSN="postgres://localhost/tars?sslmode=disable"
export TARS_TELEGRAM_BOT_TOKEN="your-bot-token"
export TARS_MODEL_API_KEY="your-api-key"
```

### 1.5 初始化数据库

```bash
# 创建数据库
createdb tars

# 执行迁移
psql -d tars -f migrations/postgres/0001_init.sql
```

### 1.6 启动开发服务

```bash
# 启动后端
go run ./cmd/tars

# 启动前端 (开发模式)
cd web && npm run dev
```

---

## 2. 代码规范

### 2.1 Go 代码规范

#### 2.1.1 命名规范

| 类型 | 规范 | 示例 |
|------|------|------|
| 包名 | 小写，短名称 | `workflow`, `session` |
| 文件名 | 小写，下划线分隔 | `session_handler.go` |
| 结构体 | PascalCase | `AlertSession`, `ExecutionRequest` |
| 接口 | PascalCase，动词结尾 | `Handler`, `Provider` |
| 函数 | PascalCase (导出) / camelCase (内部) | `NewService()`, `processItem()` |
| 常量 | CamelCase 或 UPPER_CASE | `DefaultTimeout`, `MaxRetries` |
| 变量 | camelCase | `sessionID`, `targetHost` |

#### 2.1.2 代码格式

- 使用 `gofmt` 格式化代码
- 使用 `goimports` 管理 imports
- 行长度建议不超过 120 字符
- 函数长度建议不超过 50 行
- 文件长度建议不超过 500 行

```bash
# 格式化代码
gofmt -w .

# 整理 imports
goimports -w .
```

#### 2.1.3 代码组织

```go
// 包注释
package workflow

// 标准库 imports
import (
    "context"
    "time"
)

// 第三方 imports
import (
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

// 项目内部 imports
import (
    "tars/internal/domain/session"
    "tars/internal/foundation/logger"
)
```

#### 2.1.4 错误处理

```go
// 使用明确的错误处理
if err != nil {
    return fmt.Errorf("failed to create session: %w", err)
}

// 不忽略错误
// 错误：
// _ = doSomething()

// 正确：
if err := doSomething(); err != nil {
    return err
}
```

#### 2.1.5 上下文使用

```go
// 函数第一个参数应该是 context.Context
func Process(ctx context.Context, sessionID string) error {
    // 使用 ctx 进行超时控制
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
}
```

### 2.2 前端代码规范 (TypeScript/React)

#### 2.2.1 命名规范

| 类型 | 规范 | 示例 |
|------|------|------|
| 组件 | PascalCase | `SessionList`, `ExecutionDetail` |
| Hooks | camelCase，以 use 开头 | `useSession`, `useExecution` |
| 工具函数 | camelCase | `formatDate`, `parseJSON` |
| 类型/接口 | PascalCase | `SessionData`, `ExecutionStatus` |
| 常量 | UPPER_SNAKE_CASE | `API_BASE_URL`, `DEFAULT_TIMEOUT` |

#### 2.2.2 代码格式

使用项目配置的 ESLint 和 Prettier：

```bash
cd web
npm run lint
npm run format
```

#### 2.2.3 组件结构

```tsx
// 1. imports
import React from 'react';
import { useSession } from '@/hooks/useSession';

// 2. 类型定义
interface SessionCardProps {
  sessionId: string;
}

// 3. 组件定义
export const SessionCard: React.FC<SessionCardProps> = ({ sessionId }) => {
  // hooks
  const { session, loading } = useSession(sessionId);

  // render
  return (
    <div className="session-card">
      {/* ... */}
    </div>
  );
};
```

---

## 3. 项目结构

```
TARS/
├── api/                    # OpenAPI 契约
├── cmd/tars/              # 服务启动入口
│   └── main.go
├── configs/               # 配置文件示例
├── deploy/                # 部署资源
│   ├── docker/
│   ├── grafana/
│   └── pilot/
├── docs/                  # 文档
├── internal/
│   ├── api/               # HTTP API
│   │   ├── dto/          # 数据传输对象
│   │   └── http/         # HTTP 处理程序
│   ├── app/              # 应用组装
│   ├── contracts/        # 模块间契约
│   ├── domain/           # 领域对象
│   ├── events/           # 事件处理
│   ├── foundation/       # 基础设施
│   ├── modules/          # 业务模块
│   └── repo/             # 持久化实现
├── migrations/            # 数据库迁移
├── scripts/               # 辅助脚本
├── web/                   # Web Console
└── Makefile
```

### 3.1 模块划分原则

- **高内聚低耦合**：每个模块有明确的职责边界
- **依赖方向**：`modules` -> `contracts` -> `foundation`
- **禁止循环依赖**

### 3.2 添加新模块的步骤

1. 在 `internal/modules/` 下创建目录
2. 定义模块接口（放在 `internal/contracts/`）
3. 实现业务逻辑
4. 添加单元测试
5. 在 `internal/app/bootstrap.go` 中注册

---

## 4. 开发流程

### 4.1 分支策略

采用 Git Flow 简化版：

```
main          # 生产分支
  ↓
develop       # 开发分支
  ↓
feature/*     # 功能分支
  ↓
hotfix/*      # 紧急修复分支
```

### 4.2 开发工作流

```bash
# 1. 从 main 创建功能分支
git checkout -b feature/add-new-provider

# 2. 开发并提交
git add .
git commit -m "feat: add support for new model provider"

# 3. 推送到远程
git push origin feature/add-new-provider

# 4. 创建 PR，经过 Code Review 后合并
```

### 4.3 代码审查检查清单

- [ ] 代码符合规范
- [ ] 有适当的单元测试
- [ ] 错误处理完善
- [ ] 无安全漏洞
- [ ] 性能影响已评估
- [ ] 文档已更新

---

## 5. 测试要求

### 5.1 测试覆盖率

| 类型 | 覆盖率要求 |
|------|-----------|
| 核心业务逻辑 | ≥ 80% |
| HTTP 处理程序 | ≥ 60% |
| 工具函数 | ≥ 70% |
| 基础设施 | ≥ 50% |

### 5.2 测试组织

```
module/
├── service.go
├── service_test.go        # 单元测试
└── integration_test.go    # 集成测试 (可选)
```

### 5.3 测试规范

```go
// 测试函数命名
func TestService_CreateSession(t *testing.T) {
    // 准备
    ctx := context.Background()
    svc := NewService(mockRepo)

    // 执行
    session, err := svc.CreateSession(ctx, alert)

    // 验证
    require.NoError(t, err)
    assert.NotNil(t, session)
    assert.Equal(t, "open", session.Status)
}

// 表驱动测试
func TestRiskClassifier_Classify(t *testing.T) {
    tests := []struct {
        name     string
        command  string
        expected RiskLevel
    }{
        {"read-only", "uptime", Info},
        {"write", "rm -rf /", Critical},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Classify(tt.command)
            assert.Equal(t, tt.expected, got)
        })
    }
}
```

### 5.4 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./internal/modules/workflow/...

# 运行并查看覆盖率
go test -cover ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 5.5 集成测试

```bash
# 启动测试环境
docker compose -f deploy/docker/docker-compose.yml up -d

# 运行集成测试
go test -tags=integration ./...
```

---

## 6. 提交规范

### 6.1 Commit Message 格式

采用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>(<scope>): <subject>

<body>

<footer>
```

### 6.2 Type 类型

| Type | 说明 |
|------|------|
| `feat` | 新功能 |
| `fix` | 修复 Bug |
| `docs` | 文档更新 |
| `style` | 代码格式（不影响功能） |
| `refactor` | 重构 |
| `perf` | 性能优化 |
| `test` | 测试相关 |
| `chore` | 构建/工具相关 |

### 6.3 Scope 范围

| Scope | 说明 |
|-------|------|
| `workflow` | 工作流模块 |
| `session` | 会话模块 |
| `execution` | 执行模块 |
| `api` | HTTP API |
| `web` | Web Console |
| `config` | 配置相关 |
| `docs` | 文档 |

### 6.4 示例

```bash
# 功能提交
feat(workflow): add support for execution timeout

- Add timeout configuration
- Implement timeout handler
- Add retry logic

Closes #123

# 修复提交
fix(api): resolve memory leak in session handler

The handler was not releasing resources properly.

Fixes #456

# 文档提交
docs(api): update webhook integration guide

Add examples for custom webhook format.
```

---

## 7. PR 流程

### 7.1 创建 PR

1. 确保代码已通过本地测试
2. 更新相关文档
3. 在 PR 描述中填写：
   - 变更摘要
   - 相关 Issue
   - 测试情况
   - 截图（如需要）

### 7.2 PR 模板

```markdown
## 变更摘要
<!-- 简要描述本次变更 -->

## 相关 Issue
<!-- 关联的 Issue 编号 -->
Fixes #123

## 测试情况
- [ ] 单元测试通过
- [ ] 集成测试通过
- [ ] 手动测试验证

## 检查清单
- [ ] 代码符合规范
- [ ] 文档已更新
- [ ] 测试覆盖率达标

## 截图（如适用）
<!-- UI 变更请提供截图 -->
```

### 7.3 Code Review

- 至少需要 1 个 Approver
- 所有 CI 检查必须通过
- 解决所有评论后才能合并

### 7.4 合并策略

使用 **Squash and Merge**：

```bash
# PR 合并后删除分支
git push origin --delete feature/xxx
```

---

## 8. 发布流程

### 8.1 版本号规则

采用语义化版本：[Semantic Versioning](https://semver.org/)

```
主版本.次版本.修订号

例如：1.2.3
- 1：主版本（重大变更）
- 2：次版本（新功能）
- 3：修订号（Bug 修复）
```

### 8.2 发布步骤

1. 更新版本号
2. 更新 CHANGELOG.md
3. 创建 Release PR
4. 合并到 main
5. 打标签
6. 构建并发布

```bash
# 打标签
git tag -a v1.2.3 -m "Release version 1.2.3"
git push origin v1.2.3
```

### 8.3 发布检查清单

- [ ] 所有测试通过
- [ ] 文档已更新
- [ ] CHANGELOG 已更新
- [ ] 版本号已更新
- [ ] Docker 镜像已构建
- [ ] 发布说明已准备

---

## 9. 常见问题

### 9.1 如何调试

```bash
# 设置调试日志
export TARS_LOG_LEVEL=DEBUG

# 使用 delve 调试
dlv debug ./cmd/tars
```

### 9.2 如何添加新的 Provider

1. 在 `internal/modules/reasoning/` 添加 provider 实现
2. 实现 `Provider` 接口
3. 在配置中添加 provider 定义
4. 添加单元测试

### 9.3 如何更新数据库 Schema

1. 创建新的迁移文件：`migrations/postgres/0002_xxx.sql`
2. 使用事务包裹变更
3. 测试迁移脚本
4. 更新相关代码

---

## 10. 获取帮助

- 技术讨论：创建 GitHub Discussion
- Bug 报告：创建 GitHub Issue
- 安全问题：发送邮件到 security@example.com

---

感谢您为 TARS 做出贡献！
