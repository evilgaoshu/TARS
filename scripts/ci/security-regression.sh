#!/usr/bin/env bash
# security-regression.sh — TARS 安全与权限固定回归入口（L2 专项）
#
# 覆盖边界：
#   1. 越权访问矩阵 — 无 token 访问关键 API 返回 401
#   2. 角色越权 — viewer 写操作返回 403
#   3. disabled 用户 — 账号停用后 session 认证失败
#   4. ops-token break-glass 边界 — OpsAPI 禁用时 ops-token 被拒绝
#   5. 配置 API 脱敏 — 响应中不暴露明文 secret
#   6. 审批绕过防护 — approve 端点需认证
#   7. automation/trigger 绕过防护 — 触发端点需认证，webhook secret 验证
#   8. 无效 token 格式拒绝
#   9. 公开端点白名单验证
#
# 用法：
#   bash scripts/ci/security-regression.sh
#   make security-regression
#
# 退出码：
#   0 = 全部通过
#   1 = 有测试失败
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STARTED_AT=${SECONDS}
GOCACHE_DIR="${GOCACHE:-/tmp/tars-go-build}"

cd "${ROOT_DIR}"

echo "== TARS 安全与权限回归（security-regression） =="
echo "scope=L2 安全专项 test suite"
echo "target=internal/api/http -run TestSecurity"
echo

# 运行安全回归测试套件
env GOCACHE="${GOCACHE_DIR}" \
  go test ./internal/api/http/... \
  -run "TestSecurity" \
  -v \
  -count=1 \
  -timeout 120s

ELAPSED=$((SECONDS - STARTED_AT))
echo
echo "security-regression=passed total=${ELAPSED}s"
