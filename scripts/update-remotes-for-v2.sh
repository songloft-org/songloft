#!/usr/bin/env bash
# 把所有 git remote 从 mimusic-org 切到 songloft-org，准备 push v2.0 分支。
#
# 前置条件：
#   - 已经在 GitHub 上手工完成 6 个仓库的 transfer + rename
#     （详见 docs/V2_RELEASE_PLAYBOOK.md 的 Step 1-3）
#
# 用法：
#   bash scripts/update-remotes-for-v2.sh           # 仅打印
#   bash scripts/update-remotes-for-v2.sh apply     # 实际改 remote
#
# 安全：
#   - 只修改本地 git 配置，不 push / 不 fetch
#   - pkg/tag 子模块不动（hanxi/tag fork）
#   - 旧 remote URL 保存到 origin-mimusic-org 别名，方便回滚
set -euo pipefail

APPLY=${1:-dry-run}

# 仓库路径 -> 新远端 URL
declare -A REMOTES=(
    ["."]="git@github.com:songloft-org/songloft.git"
    ["songloft-player"]="git@github.com:songloft-org/songloft-player.git"
    ["plugin-toolchain"]="git@github.com:songloft-org/plugin-toolchain.git"
    ["jsplugins"]="git@github.com:songloft-org/jsplugins.git"
    ["jsplugins-src/songloft-plugin-miot"]="git@github.com:songloft-org/songloft-plugin-miot.git"
    ["jsplugins-src/jsplugin-musicsdk"]="git@github.com:songloft-org/jsplugin-musicsdk.git"
)

update_one() {
    local dir="$1"
    local new_url="$2"
    if [[ ! -d "$dir/.git" && ! -f "$dir/.git" ]]; then
        echo "  WARN  no git dir at $dir, skip"
        return
    fi
    local old_url
    old_url=$(cd "$dir" && git remote get-url origin 2>/dev/null || echo "(no origin)")
    if [[ "$old_url" == "$new_url" ]]; then
        echo "  OK    $dir already on new URL"
        return
    fi
    if [[ "$APPLY" == "apply" ]]; then
        (
            cd "$dir"
            # 备份老 URL 到 origin-mimusic-org 别名
            if ! git remote get-url origin-mimusic-org >/dev/null 2>&1; then
                git remote add origin-mimusic-org "$old_url"
            fi
            git remote set-url origin "$new_url"
        )
        echo "  DONE  $dir"
        echo "          old: $old_url (kept as 'origin-mimusic-org')"
        echo "          new: $new_url"
    else
        echo "  [dry] $dir"
        echo "          old: $old_url"
        echo "          new: $new_url"
    fi
}

echo "==> Updating git remotes (mimusic-org -> songloft-org)"
echo

for dir in "${!REMOTES[@]}"; do
    echo "--- $dir ---"
    update_one "$dir" "${REMOTES[$dir]}"
    echo
done

echo "--- pkg/tag ---"
echo "  SKIP  hanxi/tag fork is intentionally not transferred"
echo

if [[ "$APPLY" != "apply" ]]; then
    echo "Dry run complete. Re-run with 'apply' to actually update remotes:"
    echo "  bash scripts/update-remotes-for-v2.sh apply"
else
    echo "All remotes updated. Verify with:"
    echo "  git remote -v   # and inside each submodule"
fi
