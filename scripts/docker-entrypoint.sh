#!/bin/sh
set -e

# Docker 容器启动入口脚本
# 用于实现二进制文件的热替换升级功能

BINARY_SOURCE="/app/mimusic"
BINARY_TARGET="/app/data/mimusic"
BINARY_BACKUP="/app/mimusic.backup"

echo "MiMusic Docker Entrypoint"
echo "=========================="

# 版本比较函数
# 返回 0 如果 version1 > version2，返回 1 如果 version1 <= version2
compare_versions() {
    version1="$1"
    version2="$2"

    # 处理 dev/unknown 等特殊版本
    if [ "$version1" = "dev" ] || [ "$version1" = "unknown" ]; then
        return 1
    fi
    if [ "$version2" = "dev" ] || [ "$version2" = "unknown" ]; then
        return 1
    fi

    # 如果版本相同，返回 1（不升级）
    if [ "$version1" = "$version2" ]; then
        return 1
    fi

    # 使用 awk 进行版本号逐段比较
    # 将版本号按.分割，逐段比较数字大小
    result=$(echo "$version1 $version2" | awk '{
        split($1, v1, ".")
        split($2, v2, ".")

        # 比较每一段
        for (i = 1; i <= (length(v1) > length(v2) ? length(v1) : length(v2)); i++) {
            n1 = (v1[i] ? v1[i] : 0)
            n2 = (v2[i] ? v2[i] : 0)

            # 提取数字部分（去除 -beta 等后缀）
            gsub(/[^0-9].*/, "", n1)
            gsub(/[^0-9].*/, "", n2)

            if (n1+0 > n2+0) {
                print "newer"
                exit
            } else if (n1+0 < n2+0) {
                print "older"
                exit
            }
        }
        print "same"
    }')

    if [ "$result" = "newer" ]; then
        return 0
    else
        return 1
    fi
}

# 获取二进制文件版本号
get_version() {
    binary_path="$1"
    if [ -f "$binary_path" ]; then
        "$binary_path" -version 2>&1 | grep "MiMusic Version:" | awk '{print $3}'
    else
        echo "unknown"
    fi
}

# 获取二进制文件构建类型（full 或 lite）
get_build_type() {
    binary_path="$1"
    if [ -f "$binary_path" ]; then
        build_type=$("$binary_path" -version 2>&1 | grep "Build Type:" | awk '{print $3}')
        if [ -z "$build_type" ]; then
            echo "lite"
        else
            echo "$build_type"
        fi
    else
        echo "unknown"
    fi
}

# 检查目标二进制文件是否存在
if [ ! -f "$BINARY_TARGET" ]; then
    echo "初始化：复制二进制文件到数据目录..."
    cp "$BINARY_SOURCE" "$BINARY_TARGET"
    chmod +x "$BINARY_TARGET"
    echo "✓ 二进制文件已复制到 $BINARY_TARGET"
else
    echo "检测到现有的二进制文件：$BINARY_TARGET"

    # 获取两个二进制文件的版本号
    SOURCE_VERSION=$(get_version "$BINARY_SOURCE")
    TARGET_VERSION=$(get_version "$BINARY_TARGET")

    # 获取构建类型
    SOURCE_BUILD_TYPE=$(get_build_type "$BINARY_SOURCE")
    TARGET_BUILD_TYPE=$(get_build_type "$BINARY_TARGET")

    echo "Docker 镜像版本：$SOURCE_VERSION ($SOURCE_BUILD_TYPE)"
    echo "数据目录版本：$TARGET_VERSION ($TARGET_BUILD_TYPE)"

    # 判断是否需要更新：版本更新 或 构建类型不一致
    NEED_UPDATE=false
    UPDATE_REASON=""
    SKIP_REASON=""

    if compare_versions "$SOURCE_VERSION" "$TARGET_VERSION"; then
        NEED_UPDATE=true
        UPDATE_REASON="检测到新版本"
    elif [ "$SOURCE_BUILD_TYPE" != "$TARGET_BUILD_TYPE" ]; then
        if compare_versions "$TARGET_VERSION" "$SOURCE_VERSION"; then
            SKIP_REASON="构建类型不一致（$TARGET_BUILD_TYPE vs $SOURCE_BUILD_TYPE），但数据目录版本 $TARGET_VERSION 高于底包版本 $SOURCE_VERSION，保留数据目录版本"
        else
            NEED_UPDATE=true
            UPDATE_REASON="检测到构建类型变更（$TARGET_BUILD_TYPE → $SOURCE_BUILD_TYPE）"
        fi
    fi

    if [ "$NEED_UPDATE" = true ]; then
        echo ""
        echo "$UPDATE_REASON，开始热更新..."

        # 备份旧版本
        if [ -f "$BINARY_TARGET" ]; then
            cp "$BINARY_TARGET" "$BINARY_BACKUP"
            echo "✓ 已备份旧版本到 $BINARY_BACKUP"
        fi

        # 复制新版本
        cp "$BINARY_SOURCE" "$BINARY_TARGET"
        chmod +x "$BINARY_TARGET"
        echo "✓ 已更新到版本：$SOURCE_VERSION ($SOURCE_BUILD_TYPE)"
    elif [ -n "$SKIP_REASON" ]; then
        echo "⚠ $SKIP_REASON"
    else
        echo "✓ 数据目录版本更新或相同且构建类型一致，无需升级"
    fi
fi

# 确保二进制文件有执行权限
chmod +x "$BINARY_TARGET"

# 切换到工作目录
cd /app

echo ""
echo "启动 MiMusic..."
echo ""

# 执行二进制文件，传递所有参数
exec "$BINARY_TARGET" "$@"
