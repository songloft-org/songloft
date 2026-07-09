#!/bin/sh
# Home Assistant 加载项启动脚本：
# 读取 HA 写入的 /data/options.json，转成 Songloft 后端认识的环境变量/flag 后启动。
set -e

OPT=/data/options.json

# 账号密码：对应后端 ADMIN_USERNAME / ADMIN_PASSWORD 环境变量。
export ADMIN_USERNAME="$(jq -r '.admin_username // "admin"' "$OPT")"
export ADMIN_PASSWORD="$(jq -r '.admin_password // "admin"' "$OPT")"

# 数据库落到 HA 持久化目录 /data，加载项卸载/重装数据不丢。
export DB_PATH=/data/songloft.db

# 音乐目录：默认映射到 HA 的 /media，通过 MUSIC_DIR 环境变量覆盖 DB 中的默认值。
# 用 env 而非 flag：旧版后端会静默忽略未知 env（不会崩溃），实现优雅降级——
# 不支持 MUSIC_DIR 的镜像仍能正常启动，用户可在 Web UI 手动设置音乐目录。
export MUSIC_DIR="$(jq -r '.music_path // "/media"' "$OPT")"

# 可选：反向代理子路径部署时设置 BASE_PATH。
BASE="$(jq -r '.base_path // ""' "$OPT")"
[ -n "$BASE" ] && export BASE_PATH="$BASE"

exec /app/songloft
