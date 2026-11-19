#!/usr/bin/env bash

# 自动升版号脚本（统一发布契约：与 clients/player-flutter、clients/player-lynx 的 bump-version 完全一致）
#
# 用法: ./scripts/bump-version.sh [major|minor|patch|release|X.Y.Z[-prerelease]] [--dry-run] [--yes] [--no-push]
# 示例:
#   ./scripts/bump-version.sh patch           # 1.2.4 -> 1.2.5，确认后执行
#   ./scripts/bump-version.sh release         # 1.3.0-rc.1 -> 1.3.0（去掉预发布后缀）
#   ./scripts/bump-version.sh 2.0.0 --dry-run # 仅打印计划，不修改、不联网
#
# 契约：
#   1. 预检（先于任何写操作）：main 分支、工作树干净、新版本高于当前版本、
#      v<新> tag 本地与远程均不存在、本地 main 包含 origin/main。
#   2. 更新 Makefile 的 VERSION 与 main.go 的 Swagger @version，make swagger 重新生成文档。
#   3. commit + annotated tag；git push --atomic 一次推送 main 与该 tag（--no-push 跳过）。
#      已发布的 tag 永不覆盖；失败后保留现场，不自动回滚。
#   4. 最后一行 stdout 输出新版本号（带 v 前缀），方便链式调用。

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

usage() {
    echo "自动升版号脚本（统一发布契约）"
    echo ""
    echo "用法:"
    echo "  $0 [major|minor|patch|release|X.Y.Z[-prerelease]] [--dry-run] [--yes] [--no-push]"
    echo ""
    echo "参数:"
    echo "  major|minor|patch  - 对应位 +1，低位清零（默认 patch）"
    echo "  release            - 去掉当前版本的预发布后缀（1.3.0-rc.1 -> 1.3.0）"
    echo "  X.Y.Z[-prerelease] - 直接指定新版本，必须高于当前版本"
    echo "  --dry-run          - 仅打印计划，不修改文件、不打 tag、不联网"
    echo "  --yes              - 跳过交互确认"
    echo "  --no-push          - 只创建本地 commit/tag，不推送"
    echo ""
    echo "输出:"
    echo "  最后一行 stdout 输出新版本号（带 v 前缀），如：v1.2.5"
    echo ""
    echo "示例:"
    echo "  $0 patch                 # 1.2.4 -> 1.2.5"
    echo "  $0 release --dry-run     # 仅预览"
    echo "  NEW_VER=\$($0 minor)     # 链式调用获取新版本号"
}

BUMP=""
DRY_RUN=false
ASSUME_YES=false
NO_PUSH=false
for arg in "$@"; do
    case "$arg" in
        major|minor|patch|release) BUMP="$arg" ;;
        [0-9]*)
            # 显式版本号 X.Y.Z[-prerelease]
            if [[ "$arg" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$ ]]; then
                BUMP="$arg"
            else
                log_err "未知参数 '$arg'"; usage >&2; exit 1
            fi
            ;;
        --dry-run) DRY_RUN=true ;;
        --yes) ASSUME_YES=true ;;
        --no-push) NO_PUSH=true ;;
        -h|--help) usage; exit 0 ;;
        *) log_err "未知参数 '$arg'"; usage >&2; exit 1 ;;
    esac
done
BUMP="${BUMP:-patch}"

# ---------- semver 工具（与 clients/player-lynx/scripts/release-lib.mjs 语义一致） ----------

validate_version() {
    local v="$1"
    if ! printf '%s' "$v" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$'; then
        log_err "无效的版本号格式 '$v'"
        exit 1
    fi
    if [[ "$v" == *-* ]]; then
        local pre="${v#*-}" part
        local -a parts
        IFS=. read -ra parts <<< "$pre"
        for part in "${parts[@]}"; do
            if [[ "$part" =~ ^0[0-9]+$ ]]; then
                log_err "无效的预发布标识 '$v'"
                exit 1
            fi
        done
    fi
}

# base_of / pre_of
base_of() { echo "${1%%-*}"; }
pre_of()  { case "$1" in *-*) echo "${1#*-}";; *) echo "";; esac; }

# semver_cmp A B -> 输出 -1 / 0 / 1
semver_cmp() {
    local a="$1" b="$2"
    local ab bb apre bpre
    ab=$(base_of "$a"); bb=$(base_of "$b")
    apre=$(pre_of "$a"); bpre=$(pre_of "$b")
    local -a A=() B=()
    IFS=. read -ra A <<< "$ab"
    IFS=. read -ra B <<< "$bb"
    local i
    for i in 0 1 2; do
        if (( A[i] > B[i] )); then echo 1; return; fi
        if (( A[i] < B[i] )); then echo -1; return; fi
    done
    if [[ -z "$apre" && -z "$bpre" ]]; then echo 0; return; fi
    if [[ -z "$apre" ]]; then echo 1; return; fi
    if [[ -z "$bpre" ]]; then echo -1; return; fi
    local -a PA=() PB=()
    IFS=. read -ra PA <<< "$apre"
    IFS=. read -ra PB <<< "$bpre"
    local n=$(( ${#PA[@]} > ${#PB[@]} ? ${#PA[@]} : ${#PB[@]} ))
    for ((i = 0; i < n; i++)); do
        local x="${PA[i]:-}" y="${PB[i]:-}"
        if [[ -z "$x" ]]; then echo -1; return; fi
        if [[ -z "$y" ]]; then echo 1; return; fi
        local xn=false yn=false
        [[ "$x" =~ ^[0-9]+$ ]] && xn=true
        [[ "$y" =~ ^[0-9]+$ ]] && yn=true
        if $xn && $yn; then
            if (( 10#$x > 10#$y )); then echo 1; return; fi
            if (( 10#$x < 10#$y )); then echo -1; return; fi
        elif $xn; then echo -1; return
        elif $yn; then echo 1; return
        else
            if [[ "$x" > "$y" ]]; then echo 1; return; fi
            if [[ "$x" < "$y" ]]; then echo -1; return; fi
        fi
    done
    echo 0
}

bump_version() {
    local version="$1" kind="$2"
    local base major minor patch
    base=$(base_of "$version")
    IFS=. read -r major minor patch <<< "$base"
    case "$kind" in
        release) ;;
        major) major=$((major + 1)); minor=0; patch=0 ;;
        minor) minor=$((minor + 1)); patch=0 ;;
        patch) patch=$((patch + 1)) ;;
        *) echo "$kind"; return ;;
    esac
    echo "${major}.${minor}.${patch}"
}

# ---------- 主流程 ----------

main() {
    log_info "=== Songloft 自动升版号工具 ==="

    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        log_err "当前目录不是 git 仓库"; exit 1
    fi
    if [ ! -f Makefile ]; then
        log_err "未找到 Makefile，请在项目根目录运行此脚本"; exit 1
    fi
    local current
    current=$(grep '^VERSION ?=' Makefile | sed 's/VERSION ?= //' | tr -d '[:space:]')
    if [ -z "$current" ]; then
        log_err "无法从 Makefile 读取版本号（格式：VERSION ?= x.x.x）"; exit 1
    fi
    validate_version "$current"

    local new
    new=$(bump_version "$current" "$BUMP")
    validate_version "$new"
    local cmp
    cmp=$(semver_cmp "$new" "$current")
    if [ "$cmp" != "1" ]; then
        log_err "新版本 ${new} 必须高于当前版本 ${current}"; exit 1
    fi
    local tag="v${new}"

    log_info "当前版本: ${current}"
    log_info "新版本:   ${new}"
    log_info "升级类型: ${BUMP}"
    echo "文件: Makefile, main.go, docs/（make swagger 产物）" >&2
    echo "Commit: chore: release version ${new}" >&2
    echo "Tag: ${tag}" >&2
    echo "Push: $([ "$NO_PUSH" = true ] && echo disabled || echo 'main + tag (atomic)')" >&2
    if [ "$DRY_RUN" = true ]; then
        log_warn "DRY-RUN 模式：不会修改任何文件、不打 tag、不联网"
        echo "$tag"
        exit 0
    fi

    # 写操作前的预检
    if [ "$(git branch --show-current)" != "main" ]; then
        log_err "发布必须在 main 分支"; exit 1
    fi
    if [ -n "$(git status --porcelain)" ]; then
        log_err "工作树必须干净，请先提交或保留你的改动"; exit 1
    fi
    if git rev-parse "$tag" > /dev/null 2>&1; then
        log_err "本地已存在 tag ${tag}；已发布的版本永不覆盖"; exit 1
    fi
    local remote_tags
    if ! remote_tags=$(git ls-remote --tags origin "refs/tags/${tag}" 2>&1); then
        log_err "无法检查远程 tag：${remote_tags}"; exit 1
    fi
    if [ -n "$remote_tags" ]; then
        log_err "远程已存在 tag ${tag}；已发布的版本永不覆盖"; exit 1
    fi
    if ! git fetch origin main > /dev/null 2>&1; then
        log_err "无法 fetch origin/main（离线或无权限）"; exit 1
    fi
    if ! git merge-base --is-ancestor origin/main HEAD; then
        log_err "本地 main 落后或与 origin/main 分叉，请先同步"; exit 1
    fi

    if [ "$ASSUME_YES" != true ]; then
        read -r -p "确认发布版本 ${new}？(y/N) " reply </dev/tty || reply=""
        if [[ ! "$reply" =~ ^[Yy] ]]; then
            log_err "已取消"; exit 1
        fi
    fi

    # CI 环境：自动配置 git user
    if [ -z "$(git config user.email 2>/dev/null)" ]; then
        log_warn "git user.email 未设置，自动配置为 CI 用户"
        git config user.email 'ci@songloft'
        git config user.name 'Songloft CI'
    fi

    log_info "[1/4] 更新 Makefile 中的版本号..."
    sed -i "s/^VERSION ?= .*/VERSION ?= ${new}/" Makefile
    log_ok "Makefile 已更新"

    log_info "[2/4] 更新 main.go 中的 Swagger 版本..."
    if [ -f main.go ]; then
        sed -i "s|^// @version .*|// @version ${new}|" main.go
        log_ok "main.go 已更新"
    else
        log_warn "main.go 不存在，跳过"
    fi

    log_info "[3/4] 重新生成 Swagger 文档..."
    make swagger
    log_ok "Swagger 文档已更新"

    log_info "[4/4] 提交更改并创建 git tag..."
    git add Makefile main.go docs/
    git commit -m "chore: release version ${new}"
    git tag -a "$tag" -m "Release version ${new}"
    log_ok "git commit 和 tag ${tag} 已创建"

    if [ "$NO_PUSH" = true ]; then
        log_warn "--no-push：commit 与 tag 仅保留在本地"
    else
        log_info "正在原子推送 main 与 ${tag}..."
        git push --atomic origin "HEAD:refs/heads/main" "refs/tags/${tag}"
        log_ok "已推送到远程仓库"
    fi

    echo "" >&2
    log_info "Release URL: https://github.com/songloft-org/songloft/releases/tag/${tag}"

    # 最后一行 stdout 输出新版本号（带 v 前缀），方便链式调用
    echo "$tag"
}

main
