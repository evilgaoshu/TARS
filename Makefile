.PHONY: \
	pre-check \
	check-mvp \
	check-core-coverage \
	full-check \
	security-regression \
	test-go \
	build-go \
	build-linux \
	build-linux-amd64 \
	build-linux-arm64 \
	build-linux-all \
	docker-build \
	docker-buildx \
	multiarch-regression \
	validate-openapi \
	web-lint \
	web-build \
	secret-scan \
	secret-scan-regression \
	static-demo-build \
	web-install \
	web-smoke \
	deploy-sync \
	deploy \
	smoke-remote \
	live-validate \
	live-validate-smoke \
	live-validate-auth \
	live-validate-extensions \
	help

# ─── 核心变量 ──────────────────────────────────────────────────────────────────
ROOT_DIR := $(shell pwd)
SCRIPTS  := $(ROOT_DIR)/scripts
CI_DIR   := $(SCRIPTS)/ci
HOST_LINUX_ARCH := $(shell arch="$$(uname -m)"; if [ "$$arch" = "x86_64" ] || [ "$$arch" = "amd64" ]; then echo amd64; elif [ "$$arch" = "arm64" ] || [ "$$arch" = "aarch64" ]; then echo arm64; else echo unknown; fi)
TARGET_LINUX_ARCH ?= $(HOST_LINUX_ARCH)
DOCKER_IMAGE ?= tars-local:latest
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64
DOCKER_BUILDX_ARGS ?=

# ─── L0: 快速本地预检（~2s，无外部依赖，推荐每次改代码后执行）────────────────
pre-check:
	@bash $(CI_DIR)/pre-check.sh

# ─── L1: MVP 标准本地回归（~20s，无外部依赖，合并提交前必跑）────────────────
check-mvp:
	@bash $(SCRIPTS)/check_mvp.sh

check-core-coverage:
	@bash $(SCRIPTS)/check_core_coverage.sh

# ─── L1: 完整本地回归（~30s，包含 linux 交叉编译）─────────────────────────────
full-check:
	@bash $(CI_DIR)/full-check.sh

# ─── L2: 安全与权限固定回归子集（~3s，独立可重复执行）────────────────────────
# 覆盖：越权访问矩阵 / 角色越权 / disabled 用户 / break-glass 边界
#       配置脱敏 / 审批绕过防护 / automation 绕过防护 / token 格式验证
security-regression:
	@bash $(CI_DIR)/security-regression.sh

# ─── 子步骤: 可单独调用 ────────────────────────────────────────────────────────
test-go:
	GOCACHE="$${GOCACHE:-/tmp/tars-go-build}" go test ./...

build-go:
	GOCACHE="$${GOCACHE:-/tmp/tars-go-build}" go build ./...

build-linux:
	@if [ "$(TARGET_LINUX_ARCH)" = "unknown" ]; then \
		echo "unsupported host arch for build-linux: $$(uname -m). Use make build-linux-amd64 or make build-linux-arm64."; \
		exit 1; \
	fi
	GOOS=linux GOARCH=$(TARGET_LINUX_ARCH) CGO_ENABLED=0 go build -o bin/tars-linux-$(TARGET_LINUX_ARCH) ./cmd/tars

build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/tars-linux-amd64 ./cmd/tars

build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/tars-linux-arm64 ./cmd/tars

build-linux-all: build-linux-amd64 build-linux-arm64

docker-build:
	docker build -f deploy/docker/Dockerfile -t $(DOCKER_IMAGE) .

docker-buildx:
	docker buildx build --platform $(DOCKER_PLATFORMS) -f deploy/docker/Dockerfile -t $(DOCKER_IMAGE) $(DOCKER_BUILDX_ARGS) .

multiarch-regression:
	@bash $(CI_DIR)/multiarch-regression.sh

validate-openapi:
	@ruby $(SCRIPTS)/validate_openapi.rb

web-install:
	cd web && npm ci

web-lint:
	cd web && npm run lint

web-build:
	cd web && npm run build

secret-scan:
	@bash $(CI_DIR)/secret-scan.sh

secret-scan-regression:
	@bash $(CI_DIR)/secret-scan-regression.sh

static-demo-build:
	@bash $(CI_DIR)/static-demo-build.sh

# ─── L3: Playwright UI smoke（依赖共享环境）──────────────────────────────────
web-smoke:
	@bash $(CI_DIR)/web-smoke.sh

# ─── 部署流程 ─────────────────────────────────────────────────────────────────
deploy-sync:
	@TARS_DEPLOY_SKIP_VALIDATE=1 bash $(SCRIPTS)/deploy_team_shared.sh

deploy:
	@bash $(SCRIPTS)/deploy_team_shared.sh

# ─── L3: 部署后 smoke / readiness 验证 ───────────────────────────────────────
smoke-remote:
	@bash $(CI_DIR)/smoke-remote.sh

# ─── L3: 完整 live validation（tool-plan / metrics / approval / deny / observability / delivery）
live-validate:
	@bash $(CI_DIR)/live-validate.sh

# ─── L3: 含 smoke 样本的完整 live validation ─────────────────────────────────
live-validate-smoke:
	@TARS_VALIDATE_RUN_SMOKE=1 bash $(CI_DIR)/live-validate.sh

# ─── L3/L4: Auth 增强 live validation ────────────────────────────────────────
live-validate-auth:
	@bash $(SCRIPTS)/validate_auth_enhancements_live.sh

# ─── L3/L4: Extensions live validation ───────────────────────────────────────
live-validate-extensions:
	@bash $(SCRIPTS)/validate_extensions_live.sh

# ─── 帮助 ─────────────────────────────────────────────────────────────────────
help:
	@echo ""
	@echo "TARS 本地测试与 CI/CD 入口"
	@echo ""
	@echo "  推荐顺序:"
	@echo "    make pre-check           # L0 快速预检"
	@echo "    make full-check          # L1 标准本地回归"
	@echo "    make deploy              # shared 完整闭环（deploy + smoke + live-validate）"
	@echo "    make smoke-remote        # shared readiness + hygiene"
	@echo "    make live-validate       # shared live validation"
	@echo ""
	@echo "  本地快速检查（无外部依赖）:"
	@echo "    make pre-check           # Go compile + OpenAPI"
	@echo "    make check-mvp           # Go test/build + OpenAPI + web lint/build"
	@echo "    make check-core-coverage # core modules package-level coverage gate"
	@echo "    make full-check          # check-mvp + multi-arch regression + linux/amd64,linux/arm64 交叉编译"
	@echo "    make security-regression # L2 安全与权限固定回归子集（~3s）"
	@echo ""
	@echo "  子步骤（可单独运行）:"
	@echo "    make test-go             # go test ./..."
	@echo "    make build-go            # go build ./..."
	@echo "    make build-linux         # 为当前主机对应 Linux 架构构建二进制（amd64/arm64）"
	@echo "    make build-linux-amd64   # 交叉编译 linux/amd64 二进制"
	@echo "    make build-linux-arm64   # 交叉编译 linux/arm64 二进制"
	@echo "    make build-linux-all     # 同时构建 linux/amd64 和 linux/arm64 二进制"
	@echo "    make docker-build        # 构建当前主机架构的 Docker 镜像"
	@echo "    make docker-buildx       # 使用 buildx 构建多架构 Docker 镜像"
	@echo "    make multiarch-regression # 部署链路多架构静态回归"
	@echo "    make validate-openapi    # OpenAPI 文档校验"
	@echo "    make web-install         # npm ci"
	@echo "    make web-lint            # eslint"
	@echo "    make web-build           # vite build"
	@echo "    make secret-scan         # publishable non-test tree secret scan"
	@echo "    make secret-scan-regression # secret-scan scope/allowlist regression"
	@echo "    make static-demo-build   # web build for static demo / Pages"
	@echo "    make web-smoke           # Playwright UI smoke (默认 shared env)"
	@echo ""
	@echo "  部署与共享环境验证:"
	@echo "    make deploy-sync         # 构建 + 部署到共享环境（不自动跑验证）"
	@echo "    make deploy              # 构建 + 部署 + readiness/smoke + live validation"
	@echo "    make smoke-remote        # 共享环境 readiness + hygiene 验证"
	@echo "    make live-validate       # L3 默认 live validation（tool-plan 主链）"
	@echo "    make live-validate-smoke # L3 含 smoke 样本的 live validation"
	@echo "    make live-validate-auth  # Auth 增强 live validation"
	@echo "    make live-validate-extensions  # Extensions live validation"
	@echo ""
	@echo "  环境变量（共享环境验证）:"
	@echo "    TARS_OPS_BASE_URL        # 默认 http://192.168.3.100:8081"
	@echo "    TARS_OPS_API_TOKEN       # 必填，不再提供默认 token"
	@echo "    TARS_REMOTE_HOST         # 默认 192.168.3.100"
	@echo "    TARS_REMOTE_USER         # smoke/deploy 远端用户，必填"
	@echo "    TARS_LIVE_VALIDATE_PROFILE   # core/full/exhaustive"
	@echo "    TARS_SMOKE_REMOTE_RUN_WEB    # 设为 1 时在 smoke-remote 中带 Playwright"
	@echo "    TARS_PLAYWRIGHT_BASE_URL     # web-smoke 默认跟随 TARS_OPS_BASE_URL"
	@echo ""
