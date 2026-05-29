#!/usr/bin/env bash
# Transfer 6 个 mimusic-org/* 仓库到 songloft-org.
#
# 前置条件：
#   - gh auth login 完成，账号同时是 mimusic-org owner + songloft-org owner
#   - 已经按 V2_RELEASE_PLAYBOOK.md Step 1 处理了 songloft-org/songloft placeholder
#   - 已经按 Step 2 在 mimusic-org 下完成 3 个 repo 的 rename
#
# 用法：
#   bash scripts/transfer-repos-to-songloft-org.sh           # 仅打印
#   bash scripts/transfer-repos-to-songloft-org.sh apply     # 实际 transfer
#
# 顺序按 plan：先小后大（先 transfer 内容仓库，最后 transfer 主仓库）
# 用 gh api（gh CLI 没有 'repo transfer' 子命令，只能走 REST endpoint）
set -euo pipefail

NEW_OWNER=songloft-org
REPOS=(
    mimusic-org/jsplugins
    mimusic-org/jsplugin-musicsdk
    mimusic-org/plugin-toolchain
    mimusic-org/songloft-plugin-miot
    mimusic-org/songloft-player
    mimusic-org/songloft
)

APPLY=${1:-dry-run}

if ! command -v gh >/dev/null 2>&1; then
    echo "ERROR: gh CLI not installed. Install: https://cli.github.com/" >&2
    exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
    echo "ERROR: gh not authenticated. Run: gh auth login" >&2
    exit 1
fi

# 校验 repo 存在性（避免 placeholder 冲突或 rename 漏做）
echo "==> Pre-flight: check source repos exist"
for repo in "${REPOS[@]}"; do
    if gh api "repos/$repo" --silent 2>/dev/null; then
        echo "  OK    $repo"
    else
        echo "  MISS  $repo  (does this repo exist? did you skip Step 2 rename?)"
    fi
done
echo

echo "==> Check target conflicts in $NEW_OWNER"
for repo in "${REPOS[@]}"; do
    target="$NEW_OWNER/${repo##*/}"
    if gh api "repos/$target" --silent 2>/dev/null; then
        echo "  ⚠️  WARN $target already exists in $NEW_OWNER — transfer will fail"
    else
        echo "  OK    $target available"
    fi
done
echo

echo "==> Transfer plan"
for repo in "${REPOS[@]}"; do
    echo "  $repo  ->  $NEW_OWNER/${repo##*/}"
done
echo

if [[ "$APPLY" != "apply" ]]; then
    echo "Dry run complete. Re-run with 'apply' to actually transfer:"
    echo "  bash scripts/transfer-repos-to-songloft-org.sh apply"
    exit 0
fi

echo "==> Transferring"
for repo in "${REPOS[@]}"; do
    echo
    echo "--- transfer: $repo -> $NEW_OWNER ---"
    if gh api -X POST "repos/$repo/transfer" -f new_owner="$NEW_OWNER" 2>&1 | tail -1; then
        echo "  DONE"
    else
        echo "  FAILED — re-run this script after fixing the cause"
        exit 1
    fi
done

echo
echo "All transfers submitted. Note: GitHub processes transfers asynchronously;"
echo "the new URLs may take 10-30 seconds to become reachable. Verify with:"
echo "  gh repo list $NEW_OWNER"
