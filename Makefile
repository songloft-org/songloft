# 变量定义
BINARY_NAME=songloft
GO=CGO_ENABLED=0 GOAMD64=v1 go
GO_VERSION=1.26
GOFLAGS=-v

# 版本信息
VERSION ?= 2.6.0
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
BUILD_TYPE ?=

# Tracely 监控（私有构建时注入；开源构建默认两者为空，运行时不会初始化客户端）
TRACELY_APP_SECRET ?=
TRACELY_HOST ?=

# 构建标志
LDFLAGS=-s -w \
	-X songloft/internal/version.Version=$(VERSION) \
	-X songloft/internal/version.GitCommit=$(GIT_COMMIT) \
	-X songloft/internal/version.BuildTime=$(BUILD_TIME) \
	$(if $(BUILD_TYPE),-X songloft/internal/version.BuildType=$(BUILD_TYPE)) \
	$(if $(TRACELY_APP_SECRET),-X songloft/internal/tracelycfg.AppSecret=$(TRACELY_APP_SECRET)) \
	$(if $(TRACELY_HOST),-X songloft/internal/tracelycfg.Host=$(TRACELY_HOST))

# 构建标签说明：
#   dev  - 开发模式（含 Swagger + pprof）
#   lite - 精简版（不嵌入前端资源）
# 默认（无标签）= 完整版（嵌入 Flutter Web 前端）
# 使用 -tags "tag1 tag2" 语法组合多个标签
#
# VERSION=dev 时自动启用 dev 编译标签（Swagger + pprof）
ifeq ($(VERSION),dev)
  _DEV_TAG = dev
endif

# 默认目标
.DEFAULT_GOAL := help

# 颜色输出
BLUE=\033[0;34m
GREEN=\033[0;32m
RED=\033[0;31m
NC=\033[0m # No Color

.PHONY: help
help: ## 显示帮助信息
	@echo "$(BLUE)MiMusic - Makefile 命令$(NC)"
	@echo ""
	@echo "$(BLUE)Go 版本要求: $(GO_VERSION)$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;32m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""

.PHONY: build-frontend-web-embedded
build-frontend-web-embedded: ## 构建 Flutter Web（嵌入模式）：隐藏 API 地址 UI，输出至 songloft-player-build/web-embedded
	@bash songloft-player/scripts/build-frontend.sh web-embedded $(if $(OUTPUT_DIR),$(OUTPUT_DIR),songloft-player-build)

.PHONY: build-frontend-web-embedded-debug
build-frontend-web-embedded-debug: ## 构建 Flutter Web（嵌入模式，含 source map）：仅本地调试用，产物体积会显著增大
	@DEBUG=1 bash songloft-player/scripts/build-frontend.sh web-embedded $(if $(OUTPUT_DIR),$(OUTPUT_DIR),songloft-player-build)

.PHONY: build-frontend-web
build-frontend-web: ## 构建 Flutter Web 独立部署版（standalone）
	@bash songloft-player/scripts/build-frontend.sh web $(if $(OUTPUT_DIR),$(OUTPUT_DIR),songloft-player-build)

.PHONY: build-frontend-web-debug
build-frontend-web-debug: ## 构建 Flutter Web 独立部署版（含 source map）：仅本地调试用
	@DEBUG=1 bash songloft-player/scripts/build-frontend.sh web $(if $(OUTPUT_DIR),$(OUTPUT_DIR),songloft-player-build)

.PHONY: build-frontend-linux
build-frontend-linux: ## 构建 Flutter Linux 桌面版
	@bash songloft-player/scripts/build-frontend.sh linux $(if $(OUTPUT_DIR),$(OUTPUT_DIR),songloft-player-build)

.PHONY: build-frontend-windows
build-frontend-windows: ## 构建 Flutter Windows 桌面版
	@bash songloft-player/scripts/build-frontend.sh windows $(if $(OUTPUT_DIR),$(OUTPUT_DIR),songloft-player-build)

.PHONY: build-frontend-macos
build-frontend-macos: ## 构建 Flutter macOS 桌面版
	@bash songloft-player/scripts/build-frontend.sh macos $(if $(OUTPUT_DIR),$(OUTPUT_DIR),songloft-player-build)

.PHONY: build-frontend-android
build-frontend-android: ## 构建 Flutter Android 版（APK + AAB）
	@bash songloft-player/scripts/build-frontend.sh android $(if $(OUTPUT_DIR),$(OUTPUT_DIR),songloft-player-build)

.PHONY: build-frontend-ios
build-frontend-ios: ## 构建 Flutter iOS 版（仅 macOS）
	@bash songloft-player/scripts/build-frontend.sh ios $(if $(OUTPUT_DIR),$(OUTPUT_DIR),songloft-player-build)

.PHONY: build-frontend-all
build-frontend-all: ## 构建 Flutter 前端当前系统支持的所有平台
	@bash songloft-player/scripts/build-frontend.sh all $(if $(OUTPUT_DIR),$(OUTPUT_DIR),songloft-player-build)

.PHONY: build
build: swagger ## 编译项目（开发环境，完整版本，嵌入前端）
	@echo "$(BLUE)正在编译 $(BINARY_NAME)...$(NC)"
	$(GO) build $(GOFLAGS) -tags dev -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo "$(GREEN)✓ 编译完成: $(BINARY_NAME)$(NC)"

.PHONY: build-lite
build-lite: swagger ## 编译项目（开发环境，lite 版本，不嵌入前端）
	@echo "$(BLUE)正在编译精简版 $(BINARY_NAME)...$(NC)"
	$(GO) build $(GOFLAGS) -tags "dev,lite" -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo "$(GREEN)✓ 编译完成: $(BINARY_NAME)$(NC)"

.PHONY: build-prod
build-prod: ## 编译项目（生产环境，完整版本，嵌入前端；VERSION=dev 时自动含 Swagger + pprof）
	@echo "$(BLUE)正在编译生产环境版本 $(BINARY_NAME)...$(NC)"
	$(GO) build $(GOFLAGS) $(if $(_DEV_TAG),-tags "$(_DEV_TAG)") -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .
	@if command -v upx >/dev/null 2>&1; then \
		echo "$(BLUE)正在使用 UPX 压缩...$(NC)"; \
		upx -9 $(BINARY_NAME); \
		echo "$(GREEN)✓ UPX 压缩完成$(NC)"; \
	else \
		echo "$(YELLOW)警告：UPX 未安装，跳过压缩$(NC)"; \
	fi
	@echo "$(GREEN)✓ 编译完成：$(BINARY_NAME)$(NC)"

.PHONY: build-prod-lite
build-prod-lite: ## 编译项目（生产环境，lite 版本，不嵌入前端；VERSION=dev 时自动含 Swagger + pprof）
	@echo "$(BLUE)正在编译生产环境精简版 $(BINARY_NAME)...$(NC)"
	$(GO) build $(GOFLAGS) -tags "lite $(_DEV_TAG)" -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .
	@if command -v upx >/dev/null 2>&1; then \
		echo "$(BLUE)正在使用 UPX 压缩...$(NC)"; \
		upx -9 $(BINARY_NAME); \
		echo "$(GREEN)✓ UPX 压缩完成$(NC)"; \
	else \
		echo "$(YELLOW)警告：UPX 未安装，跳过压缩$(NC)"; \
	fi
	@echo "$(GREEN)✓ 编译完成：$(BINARY_NAME)$(NC)"

.PHONY: build-linux-prod
build-linux-prod: ## 编译 Linux 版本（生产环境，完整版）
	@echo "$(BLUE)正在编译生产环境 Linux 版本...$(NC)"
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux .
	@if command -v upx >/dev/null 2>&1; then \
		echo "$(BLUE)正在使用 UPX 压缩...$(NC)"; \
		upx -9 $(BINARY_NAME)-linux; \
		echo "$(GREEN)✓ UPX 压缩完成$(NC)"; \
	else \
		echo "$(YELLOW)警告：UPX 未安装，跳过压缩$(NC)"; \
	fi
	@echo "$(GREEN)✓ 编译完成：$(BINARY_NAME)-linux$(NC)"

.PHONY: build-linux-prod-lite
build-linux-prod-lite: ## 编译 Linux 版本（生产环境，精简版）
	@echo "$(BLUE)正在编译生产环境 Linux 精简版...$(NC)"
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -tags lite -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux .
	@if command -v upx >/dev/null 2>&1; then \
		echo "$(BLUE)正在使用 UPX 压缩...$(NC)"; \
		upx -9 $(BINARY_NAME)-linux; \
		echo "$(GREEN)✓ UPX 压缩完成$(NC)"; \
	else \
		echo "$(YELLOW)警告：UPX 未安装，跳过压缩$(NC)"; \
	fi
	@echo "$(GREEN)✓ 编译完成：$(BINARY_NAME)-linux$(NC)"

.PHONY: build-windows-prod
build-windows-prod: ## 编译 Windows 版本（生产环境，完整版）
	@echo "$(BLUE)正在编译生产环境 Windows 版本...$(NC)"
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME).exe .
	@echo "$(GREEN)✓ 编译完成: $(BINARY_NAME).exe$(NC)"

.PHONY: build-windows-prod-lite
build-windows-prod-lite: ## 编译 Windows 版本（生产环境，精简版）
	@echo "$(BLUE)正在编译生产环境 Windows 精简版...$(NC)"
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -tags lite -ldflags="$(LDFLAGS)" -o $(BINARY_NAME).exe .
	@echo "$(GREEN)✓ 编译完成: $(BINARY_NAME).exe$(NC)"

.PHONY: build-darwin-prod
build-darwin-prod: ## 编译 macOS 版本（生产环境，完整版）
	@echo "$(BLUE)正在编译生产环境 macOS 版本...$(NC)"
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-darwin .
	@echo "$(GREEN)✓ 编译完成: $(BINARY_NAME)-darwin$(NC)"

.PHONY: build-darwin-prod-lite
build-darwin-prod-lite: ## 编译 macOS 版本（生产环境，精简版）
	@echo "$(BLUE)正在编译生产环境 macOS 精简版...$(NC)"
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -tags lite -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-darwin .
	@echo "$(GREEN)✓ 编译完成: $(BINARY_NAME)-darwin$(NC)"

.PHONY: build-all-prod
build-all-prod: build-linux-prod build-windows-prod build-darwin-prod ## 编译所有平台版本（生产环境，完整版）
	@echo "$(GREEN)✓ 所有平台编译完成$(NC)"

.PHONY: build-all-prod-lite
build-all-prod-lite: build-linux-prod-lite build-windows-prod-lite build-darwin-prod-lite ## 编译所有平台版本（生产环境，精简版）
	@echo "$(GREEN)✓ 所有平台精简版编译完成$(NC)"

.PHONY: build-cross
build-cross: ## 交叉编译（用法：make build-cross GOOS=linux GOARCH=amd64 OUTPUT=songloft-linux-amd64 [EXTRA_TAGS=lite]）
	@echo "$(BLUE)正在编译 $(GOOS)/$(GOARCH)...$(NC)"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(if $(GOARM),GOARM=$(GOARM)) GOAMD64=v1 go build $(GOFLAGS) $(if $(strip $(_DEV_TAG) $(EXTRA_TAGS)),-tags "$(strip $(_DEV_TAG) $(EXTRA_TAGS))") -ldflags="$(LDFLAGS)" -o $(OUTPUT) .
	@if command -v upx >/dev/null 2>&1 && echo " linux/amd64 linux/arm64 linux/arm windows/amd64 " | grep -q " $(GOOS)/$(GOARCH) "; then \
		echo "$(BLUE)正在使用 UPX 压缩...$(NC)"; \
		upx -9 $(OUTPUT) >/dev/null 2>&1 || true; \
		echo "$(GREEN)✓ $(OUTPUT) (UPX 压缩)$(NC)"; \
	else \
		echo "$(GREEN)✓ $(OUTPUT)$(NC)"; \
	fi

.PHONY: test
test: ## 运行所有测试
	@echo "$(BLUE)正在运行测试...$(NC)"
	$(GO) test -v ./...
	@echo "$(GREEN)✓ 测试完成$(NC)"

.PHONY: test-coverage
test-coverage: ## 运行测试并生成覆盖率报告
	@echo "$(BLUE)正在运行测试并生成覆盖率报告...$(NC)"
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ 覆盖率报告已生成: coverage.html$(NC)"

.PHONY: test-short
test-short: ## 运行快速测试（跳过集成测试）
	@echo "$(BLUE)正在运行快速测试...$(NC)"
	$(GO) test -short -v ./...
	@echo "$(GREEN)✓ 快速测试完成$(NC)"

.PHONY: test-unit
test-unit: ## 仅运行单元测试
	@echo "$(BLUE)正在运行单元测试...$(NC)"
	$(GO) test -v ./internal/...
	@echo "$(GREEN)✓ 单元测试完成$(NC)"

.PHONY: bench
bench: ## 运行性能测试
	@echo "$(BLUE)正在运行性能测试...$(NC)"
	$(GO) test -bench=. -benchmem ./...
	@echo "$(GREEN)✓ 性能测试完成$(NC)"

.PHONY: clean
clean: ## 清理编译产物
	@echo "$(BLUE)正在清理...$(NC)"
	@rm -f $(BINARY_NAME)
	@rm -f $(BINARY_NAME)-linux
	@rm -f $(BINARY_NAME).exe
	@rm -f $(BINARY_NAME)-darwin
	@rm -f coverage.out coverage.html
	@rm -f songloft.db mimusic.db
	@rm -rf web/dist
	@echo "$(GREEN)✓ 清理完成$(NC)"

.PHONY: run
run: ## 运行项目（开发环境，使用默认配置：admin/admin/58091）
	@echo "$(BLUE)正在启动服务...$(NC)"
	@echo "$(BLUE)Username: admin, Password: admin, 端口: 58091$(NC)"
	$(GO) run -tags dev . -username admin -password admin -port 58091

.PHONY: run-prod
run-prod: ## 运行项目（生产环境）
	@echo "$(BLUE)正在启动生产环境服务...$(NC)"
	$(GO) run . -username admin -password admin -port 58091

.PHONY: fmt
fmt: ## 格式化代码
	@echo "$(BLUE)正在格式化代码...$(NC)"
	$(GO) fmt ./...
	@echo "$(GREEN)✓ 代码格式化完成$(NC)"

.PHONY: vet
vet: ## 运行 go vet 检查
	@echo "$(BLUE)正在运行 go vet...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)✓ go vet 检查完成$(NC)"

.PHONY: lint
lint: ## 运行 golangci-lint（需要先安装）
	@echo "$(BLUE)正在运行 golangci-lint...$(NC)"
	@which golangci-lint > /dev/null || (echo "$(RED)错误: golangci-lint 未安装$(NC)" && exit 1)
	golangci-lint run ./...
	@echo "$(GREEN)✓ lint 检查完成$(NC)"

.PHONY: version
version: ## 检查 Go 版本
	@echo "$(BLUE)当前 Go 版本:$(NC)"
	@$(GO) version
	@echo ""
	@echo "$(BLUE)项目要求 Go 版本: $(GO_VERSION)$(NC)"

.PHONY: deps
deps: ## 下载依赖
	@echo "$(BLUE)正在下载依赖...$(NC)"
	$(GO) mod download
	@echo "$(GREEN)✓ 依赖下载完成$(NC)"

.PHONY: tidy
tidy: ## 整理依赖
	@echo "$(BLUE)正在整理依赖...$(NC)"
	$(GO) mod tidy
	@echo "$(GREEN)✓ 依赖整理完成$(NC)"

.PHONY: docker-build
docker-build: ## 构建 Docker 测试镜像（VERSION=dev 自动启用 Swagger + pprof）
	@echo "$(BLUE)正在构建 Docker 测试镜像...$(NC)"
	docker build \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg VERSION=dev \
		-t $(BINARY_NAME):latest .
	@echo "$(GREEN)✓ Docker 测试镜像构建完成（版本: $(TEST_VERSION)）$(NC)"

.PHONY: docker-run
docker-run: ## 运行 Docker 容器
	@echo "$(BLUE)正在启动 Docker 容器...$(NC)"
	docker run -p 58091:58091 -e ADMIN_USERNAME=admin -e ADMIN_PASSWORD=admin $(BINARY_NAME):latest

.PHONY: check
check: fmt vet test ## 运行所有检查（格式化、vet、测试）
	@echo "$(GREEN)✓ 所有检查通过$(NC)"

.PHONY: sqlc
sqlc: ## 重新生成 sqlc 代码（修改 internal/database/queries/*.sql 后执行）
	@echo "$(BLUE)正在生成 sqlc 代码...$(NC)"
	@which sqlc > /dev/null || (echo "$(RED)错误: sqlc 未安装，请运行: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest$(NC)" && exit 1)
	sqlc generate
	@echo "$(GREEN)✓ sqlc 代码生成完成（请 git add internal/database/sqlc/）$(NC)"

.PHONY: sqlc-verify
sqlc-verify: ## 校验 queries/*.sql 语法，不生成文件（CI 用）
	@which sqlc > /dev/null || (echo "$(RED)错误: sqlc 未安装$(NC)" && exit 1)
	sqlc vet
	@echo "$(GREEN)✓ sqlc 校验通过$(NC)"

.PHONY: swagger
swagger: ## 生成/更新 Swagger API 文档
	@echo "$(BLUE)正在更新 main.go 中的版本号...$(NC)"
	@sed -i '' 's|^// @version .*|// @version $(VERSION)|' main.go 2>/dev/null || \
		sed -i 's|^// @version .*|// @version $(VERSION)|' main.go
	@echo "$(BLUE)正在生成 Swagger 文档...$(NC)"
	@which swag > /dev/null || (echo "$(BLUE)swag 未安装，正在自动安装...$(NC)" && go install github.com/swaggo/swag/cmd/swag@latest)
	swag init
	@echo "$(GREEN)✓ Swagger 文档生成完成（版本: $(VERSION)）$(NC)"

.PHONY: bump
bump: ## 升级版本号并打 tag（push 后由 .github/workflows/release.yml 完成构建发布）
	@bash scripts/bump-version.sh $(or $(TYPE),patch)

.PHONY: sync-repowiki
sync-repowiki: ## 同步 Qoder repowiki 到 docs/repowiki/
	./scripts/sync-repowiki.sh

.PHONY: all
all: clean deps build test ## 完整构建流程（清理、下载依赖、编译、测试）
	@echo "$(GREEN)✓ 完整构建完成$(NC)"
