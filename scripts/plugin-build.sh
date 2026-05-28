#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PLUGINS_DIR="$ROOT_DIR/jsplugins-src"
OUTPUT_DIR="$ROOT_DIR/data/jsplugins"

mkdir -p "$OUTPUT_DIR"

build_plugin() {
    local plugin_dir="$1"
    local plugin_name="$(basename "$plugin_dir")"
    echo "==> Building $plugin_name ..."
    cd "$plugin_dir"
    npm i
    npm run build && cp dist/*.jsplugin.zip "$OUTPUT_DIR/"
    echo "==> $plugin_name done."
    cd "$ROOT_DIR"
}

if [ $# -eq 0 ]; then
    # 无参数：构建所有插件
    for plugin_dir in "$PLUGINS_DIR"/mimusic-jsplugin-*/; do
        [ -d "$plugin_dir" ] || continue
        build_plugin "$plugin_dir"
    done
else
    # 有参数：按名字构建指定插件
    for name in "$@"; do
        plugin_dir="$PLUGINS_DIR/mimusic-jsplugin-$name"
        if [ ! -d "$plugin_dir" ]; then
            echo "Error: plugin '$name' not found at $plugin_dir" >&2
            exit 1
        fi
        build_plugin "$plugin_dir"
    done
fi

echo "All plugins built successfully."
