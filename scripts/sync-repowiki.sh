#!/bin/bash
# 把 Qoder IDE 生成的 repowiki 同步到 docs/repowiki/，供 VitePress 文档站收录。
#
# 用法：
#   ./scripts/sync-repowiki.sh
#   或 make sync-repowiki
#
# 源：.qoder/repowiki/zh/content/   （Qoder 写入位置，被 .gitignore 忽略）
# 目标：docs/repowiki/               （入仓 + VitePress auto-sidebar 自动收录）
#
# 同步后自行 git diff 查看变更，并按需 commit。

set -e

SRC=".qoder/repowiki/zh/content"
DST="docs/repowiki"

if [ ! -d "$SRC" ]; then
    echo "源目录不存在：$SRC"
    echo "请先用 Qoder IDE 生成 repowiki，或确认你在仓库根目录下运行。"
    exit 1
fi

mkdir -p "$DST"

# rsync --delete 会清理 DST 中源里已不存在的文件（处理 Qoder 改名/删除）
rsync -av --delete "$SRC/" "$DST/"

echo ""
echo "同步完成。运行 git status 查看变更。"
