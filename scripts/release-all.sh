#!/usr/bin/env bash

# Songloft 一键发布：Flutter 客户端 → Lynx 客户端 → 提交子模块引用 → 父仓库后端
#
# 用法: ./scripts/release-all.sh [major|minor|patch|release|X.Y.Z[-prerelease]] [--dry-run] [--yes] [--no-push]
# 示例:
#   ./scripts/release-all.sh patch            # 三个仓库统一 patch 升版
#   ./scripts/release-all.sh release --dry-run
#   ./scripts/release-all.sh 2.14.0 --yes
#
# 三个 bump-version 入口共享同一发布契约（参数与预检完全一致），本脚本把同一组参数原样透传。
#
# 顺序约束：
#   1. 子仓库脚本各自 commit + tag + push（触发各自 CI）。
#   2. 父仓库用独立 commit 收录两个 gitlink（pathspec 限定，不裹带无关改动）。
#   3. 父仓库 bump 脚本要求干净工作树，因此 gitlink 必须先落地；其 push 一并带走该 commit。
#
# 失败即停、不回滚：已推送的 tag/commit 保留现场，按各仓库发布文档人工处理。
# --no-push 时所有 commit/tag 只留在本地，gitlink 同样只本地提交。

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${BLUE}$1${NC}" >&2; }
log_ok()   { echo -e "${GREEN}✓${NC} $1" >&2; }
log_warn() { echo -e "${YELLOW}警告：$1${NC}" >&2; }
log_err()  { echo -e "${RED}错误：$1${NC}" >&2; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
FLUTTER_SCRIPT="$ROOT_DIR/clients/player-flutter/scripts/bump-version.sh"
LYNX_SCRIPT="$ROOT_DIR/clients/player-lynx/scripts/bump-version.sh"
PARENT_SCRIPT="$ROOT_DIR/scripts/bump-version.sh"
FLUTTER_DIR="$ROOT_DIR/clients/player-flutter"
LYNX_DIR="$ROOT_DIR/clients/player-lynx"

usage() {
    echo "Songloft 一键发布（Flutter → Lynx → gitlink → 父仓库）"
    echo ""
    echo "用法:"
    echo "  $0 [major|minor|patch|release|X.Y.Z[-prerelease]] [--dry-run] [--yes] [--no-push]"
    echo ""
    echo "同一组参数原样透传给三个仓库的 bump-version 脚本（三者契约一致）。"
    echo "注意：三个仓库版本号将各自按当前版本独立计算；如需对齐版本，请显式传 X.Y.Z。"
    echo ""
    echo "示例:"
    echo "  $0 patch --dry-run     # 预览三个仓库的计划，零写入零联网"
    echo "  $0 2.14.0              # 三个仓库统一发布 2.14.0"
}

# 参数校验（与 bump-version 相同的接受集；具体语义由各 bump 脚本再校验一次）
BUMP_SEEN=false
DRY_RUN=false
for arg in "$@"; do
    case "$arg" in
        major|minor|patch|release)
            [ "$BUMP_SEEN" = true ] && { log_err "只能指定一个升级类型"; exit 1; }
            BUMP_SEEN=true ;;
        [0-9]*)
            [[ "$arg" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$ ]] || { log_err "无效参数 '$arg'"; usage >&2; exit 1; }
            [ "$BUMP_SEEN" = true ] && { log_err "只能指定一个升级类型"; exit 1; }
            BUMP_SEEN=true ;;
        --dry-run) DRY_RUN=true ;;
        --yes|--no-push) ;;
        -h|--help) usage; exit 0 ;;
        *) log_err "无效参数 '$arg'"; usage >&2; exit 1 ;;
    esac
done

cd "$ROOT_DIR"

# ---------- 预检（先于任何子脚本调用，避免客户端 tag 已推送后才发现父仓库不可发布） ----------

if ! git rev-parse --git-dir > /dev/null 2>&1; then
    log_err "当前目录不是 git 仓库"; exit 1
fi
if [ "$(git branch --show-current)" != "main" ]; then
    log_err "一键发布必须在父仓库 main 分支"; exit 1
fi
for f in "$FLUTTER_SCRIPT" "$LYNX_SCRIPT" "$PARENT_SCRIPT"; do
    if [ ! -f "$f" ]; then
        log_err "缺少发布脚本 $f（子模块是否已初始化？）"; exit 1
    fi
done
for d in "$FLUTTER_DIR" "$LYNX_DIR"; do
    if ! git -C "$d" rev-parse --git-dir > /dev/null 2>&1; then
        log_err "子模块 $d 未初始化，请先 git submodule update --init $d"; exit 1
    fi
    dirty=$(git -C "$d" status --porcelain)
    if [ -n "$dirty" ]; then
        log_err "子模块 $d 工作树不干净："
        printf '%s\n' "$dirty" | sed 's/^/    /' >&2
        exit 1
    fi
done
# 父仓库除两个 gitlink 外必须干净（无 staged、无其他未提交改动）
parent_dirty=$(git status --porcelain | grep -v '^.. clients/player-flutter$' | grep -v '^.. clients/player-lynx$' || true)
if [ -n "$(git diff --cached --name-only)" ]; then
    log_err "父仓库暂存区不干净，请先处理："; git diff --cached --name-only | sed 's/^/    /' >&2; exit 1
fi
if [ -n "$parent_dirty" ]; then
    log_err "父仓库存在与本次发布无关的改动："
    printf '%s\n' "$parent_dirty" | sed 's/^/    /' >&2
    exit 1
fi

# ---------- 执行 ----------

run_step() {
    local label="$1"; shift
    log_info ""
    log_info "=== ${label} ==="
    "$@"
}

capture_version() {
    # bump 脚本日志走 stderr，stdout 末行即 v<version>
    local version
    version=$(printf '%s' "$2" | tail -n 1)
    if [[ ! "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$ ]]; then
        log_err "$1 未返回有效版本号，停止后续发布"
        return 1
    fi
    printf '%s\n' "$version"
}

flutter_out=$(cd "$FLUTTER_DIR" && ./scripts/bump-version.sh "$@")
flutter_ver=$(capture_version "Flutter 客户端" "$flutter_out")
log_ok "Flutter 客户端: ${flutter_ver}"

lynx_out=$(cd "$LYNX_DIR" && ./scripts/bump-version.sh "$@")
lynx_ver=$(capture_version "Lynx 客户端" "$lynx_out")
log_ok "Lynx 客户端: ${lynx_ver}"

# gitlink 提交（pathspec 限定，绝不裹带其他改动）
if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}[dry-run]${NC} git commit -m 'chore(clients): 同步 Flutter 与 Lynx 客户端发布版本' -- clients/player-flutter clients/player-lynx" >&2
elif git diff --quiet HEAD -- clients/player-flutter clients/player-lynx; then
    log_warn "两个子模块引用均无变化，跳过 gitlink 提交"
else
    log_info ""
    log_info "=== 提交子模块引用 ==="
    git diff HEAD --submodule=short -- clients/player-flutter clients/player-lynx | grep -E '^(diff|Subproject)' | sed 's/^/    /' >&2 || true
    git commit -m "chore(clients): 同步 Flutter 与 Lynx 客户端发布版本" -- clients/player-flutter clients/player-lynx
    log_ok "gitlink 已提交（仅 clients/player-flutter、clients/player-lynx）"
fi

parent_out=$("$PARENT_SCRIPT" "$@")
parent_ver=$(capture_version "父仓库后端" "$parent_out")
log_ok "父仓库后端: ${parent_ver}"

log_info ""
log_info "=== 发布完成 ===" >&2
echo "Flutter: ${flutter_ver}  https://github.com/songloft-org/songloft-player-flutter/releases/tag/${flutter_ver}" >&2
echo "Lynx:    ${lynx_ver}  https://github.com/songloft-org/songloft-player-lynx/releases/tag/${lynx_ver}" >&2
echo "后端:    ${parent_ver}  https://github.com/songloft-org/songloft/releases/tag/${parent_ver}" >&2

# 最后一行 stdout 输出父仓库新版本号，方便链式调用
echo "$parent_ver"
