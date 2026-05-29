#!/bin/bash

# 自动升版号脚本
# 用法: ./scripts/bump-version.sh [major|minor|patch] [--dry-run]
# 示例:
#   ./scripts/bump-version.sh patch          # 1.2.4 -> 1.2.5，直接执行
#   ./scripts/bump-version.sh minor --dry-run # 仅打印，不实际修改
#
# 流程：
#   1. 更新 Makefile 中的 VERSION
#   2. 更新 main.go 中的 Swagger @version
#   3. git commit + tag + push（push tag 后由 .github/workflows/release.yml 完成
#      多平台构建、Docker 镜像、GitHub Release，以及 CHANGELOG.md 更新并回写到 main）
#
# 最后一行 stdout 输出新版本号（带 v 前缀），方便链式调用。

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# ============================================================
# 参数解析
# ============================================================
BUMP_TYPE=""
DRY_RUN=false

for arg in "$@"; do
    case "$arg" in
        major|minor|patch)
            BUMP_TYPE="$arg"
            ;;
        --dry-run)
            DRY_RUN=true
            ;;
        -h|--help)
            echo "自动升版号脚本（无交互版）"
            echo ""
            echo "用法:"
            echo "  $0 [major|minor|patch] [--dry-run]"
            echo ""
            echo "参数:"
            echo "  major     - 主版本号升级 (1.0.0 -> 2.0.0)"
            echo "  minor     - 次版本号升级 (1.0.0 -> 1.1.0)"
            echo "  patch     - 补丁版本号升级 (1.0.0 -> 1.0.1，默认)"
            echo "  --dry-run - 仅打印将要执行的操作，不实际修改文件"
            echo ""
            echo "输出:"
            echo "  最后一行 stdout 输出新版本号（带 v 前缀），如：v1.2.5"
            echo ""
            echo "示例:"
            echo "  $0 patch              # 1.2.4 -> 1.2.5"
            echo "  $0 minor --dry-run    # 仅预览，不修改"
            echo "  NEW_VER=\$($0 patch)  # 链式调用获取新版本号"
            exit 0
            ;;
        *)
            echo -e "${RED}错误：未知参数 '$arg'${NC}" >&2
            echo "用法：$0 [major|minor|patch] [--dry-run]" >&2
            exit 1
            ;;
    esac
done

# 默认 patch
BUMP_TYPE="${BUMP_TYPE:-patch}"

# ============================================================
# 工具函数
# ============================================================

log_info() {
    echo -e "${BLUE}$1${NC}" >&2
}

log_ok() {
    echo -e "${GREEN}✓${NC} $1" >&2
}

log_warn() {
    echo -e "${YELLOW}警告：$1${NC}" >&2
}

log_err() {
    echo -e "${RED}错误：$1${NC}" >&2
}

# 获取当前版本号（从 Makefile）
get_current_version() {
    grep '^VERSION ?=' Makefile | sed 's/VERSION ?= //' | tr -d '[:space:]'
}

# 升级版本号
bump_version() {
    local version="$1"
    local bump_type="$2"

    # 去掉 v 前缀
    version="${version#v}"

    local major minor patch
    major=$(echo "$version" | cut -d. -f1)
    minor=$(echo "$version" | cut -d. -f2)
    patch=$(echo "$version" | cut -d. -f3)

    case "$bump_type" in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
    esac

    echo "${major}.${minor}.${patch}"
}

# 执行或打印命令（dry-run 模式下只打印）
run_cmd() {
    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}[dry-run]${NC} $*" >&2
    else
        eval "$@"
    fi
}

# ============================================================
# 主流程
# ============================================================
main() {
    log_info "=== Songloft 自动升版号工具 ==="
    if [ "$DRY_RUN" = true ]; then
        log_warn "DRY-RUN 模式：不会实际修改任何文件"
    fi
    echo "" >&2

    # 检查是否在 git 仓库中
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        log_err "当前目录不是 git 仓库"
        exit 1
    fi

    # 检查 Makefile 是否存在
    if [ ! -f Makefile ]; then
        log_err "未找到 Makefile，请在项目根目录运行此脚本"
        exit 1
    fi

    # 获取当前版本
    local current_version
    current_version=$(get_current_version)
    if [ -z "$current_version" ]; then
        log_err "无法从 Makefile 读取版本号（格式：VERSION ?= x.x.x）"
        exit 1
    fi

    # 计算新版本
    local new_version
    new_version=$(bump_version "$current_version" "$BUMP_TYPE")

    log_info "当前版本: ${current_version}"
    log_info "新版本:   ${new_version}"
    log_info "升级类型: ${BUMP_TYPE}"
    echo "" >&2

    if [ "$DRY_RUN" = false ]; then
        read -p "确认升级版本吗？(y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo -e "${RED}已取消${NC}"
            exit 1
        fi
    fi

    # CI 环境：自动配置 git user
    if [ -z "$(git config user.email 2>/dev/null)" ]; then
        log_warn "git user.email 未设置，自动配置为 CI 用户"
        run_cmd "git config user.email 'ci@mimusic'"
        run_cmd "git config user.name 'Songloft CI'"
    fi

    # 1. 更新 Makefile
    log_info "[1/3] 更新 Makefile 中的版本号..."
    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}[dry-run]${NC} sed: VERSION ?= ${current_version} -> VERSION ?= ${new_version}" >&2
    else
        sed -i "s/^VERSION ?= .*/VERSION ?= ${new_version}/" Makefile
    fi
    log_ok "Makefile 已更新"

    # 2. 更新 main.go 中的 Swagger 版本
    log_info "[2/3] 更新 main.go 中的 Swagger 版本..."
    if [ -f main.go ]; then
        if [ "$DRY_RUN" = true ]; then
            echo -e "${YELLOW}[dry-run]${NC} sed: @version -> ${new_version}" >&2
        else
            sed -i "s|^// @version .*|// @version ${new_version}|" main.go
        fi
        log_ok "main.go 已更新"
    else
        log_warn "main.go 不存在，跳过"
    fi

    # 3. git commit + tag（CHANGELOG.md 由 release.yml 在 CI 中更新并回写到 main）
    log_info "[3/3] 提交更改并创建 git tag..."
    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}[dry-run]${NC} git add Makefile main.go" >&2
        echo -e "${YELLOW}[dry-run]${NC} git commit -m 'chore: release version ${new_version}'" >&2
        echo -e "${YELLOW}[dry-run]${NC} git tag -a 'v${new_version}' -m 'Release version ${new_version}'" >&2
    else
        git add Makefile main.go
        git commit -m "chore: release version ${new_version}"
        git tag -a "v${new_version}" -m "Release version ${new_version}"
    fi
    log_ok "git commit 和 tag v${new_version} 已创建（未 push）"

    # 直接push
    log_info "正在推送更改和 tag..."
    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}[dry-run]${NC} git push --follow-tags" >&2
    else
        git push --follow-tags
    fi

    # 最后一行 stdout 输出新版本号（带 v 前缀），方便链式调用
    echo "v${new_version}"
}

main
