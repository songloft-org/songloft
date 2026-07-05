#!/bin/sh
set -e

# Docker 容器启动入口脚本
# 用于实现二进制文件的热替换升级功能

BINARY_SOURCE="/app/songloft"
BINARY_TARGET="/app/data/songloft"
BINARY_BACKUP="/app/songloft.backup"

echo "Songloft Docker Entrypoint"
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
        "$binary_path" -version 2>&1 | grep "Songloft Version:" | awk '{print $3}'
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
            echo "full"
        else
            echo "$build_type"
        fi
    else
        echo "unknown"
    fi
}

# 获取二进制文件构建时间
get_build_time() {
    binary_path="$1"
    if [ -f "$binary_path" ]; then
        build_time=$("$binary_path" -version 2>&1 | grep "Build Time:" | awk '{print $3}')
        if [ -z "$build_time" ]; then
            echo "unknown"
        else
            echo "$build_time"
        fi
    else
        echo "unknown"
    fi
}

# 获取版本通道：dev / stable / unknown
get_version_channel() {
    version="$1"
    if [ "$version" = "dev" ]; then
        echo "dev"
    elif [ "$version" = "unknown" ] || [ -z "$version" ]; then
        echo "unknown"
    else
        echo "stable"
    fi
}

# 构建时间比较函数
# 返回 0 如果 build_time1 > build_time2，返回 1 如果 build_time1 <= build_time2
compare_build_times() {
    build_time1="$1"
    build_time2="$2"

    if [ -z "$build_time1" ] || [ "$build_time1" = "unknown" ]; then
        return 1
    fi
    if [ -z "$build_time2" ] || [ "$build_time2" = "unknown" ]; then
        return 0
    fi

    result=$(awk -v a="$build_time1" -v b="$build_time2" 'BEGIN {
        if (a > b) {
            print "newer"
        } else {
            print "not_newer"
        }
    }')

    if [ "$result" = "newer" ]; then
        return 0
    else
        return 1
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

    # 获取构建时间和版本通道
    SOURCE_BUILD_TIME=$(get_build_time "$BINARY_SOURCE")
    TARGET_BUILD_TIME=$(get_build_time "$BINARY_TARGET")
    SOURCE_CHANNEL=$(get_version_channel "$SOURCE_VERSION")
    TARGET_CHANNEL=$(get_version_channel "$TARGET_VERSION")

    echo "Docker 镜像版本：$SOURCE_VERSION ($SOURCE_BUILD_TYPE, $SOURCE_CHANNEL, $SOURCE_BUILD_TIME)"
    echo "数据目录版本：$TARGET_VERSION ($TARGET_BUILD_TYPE, $TARGET_CHANNEL, $TARGET_BUILD_TIME)"

    # 判断是否需要更新
    # 原则：底包代表用户意图。
    # 通道（dev/stable）或构建类型（full/lite）不一致时，用底包覆盖。
    # 同为 dev 时按构建时间选最新；同为 stable 时按版本号选最新。
    NEED_UPDATE=false
    UPDATE_REASON=""
    SKIP_REASON=""

    if [ "$SOURCE_CHANNEL" = "unknown" ]; then
        SKIP_REASON="无法识别底包版本通道，保留数据目录二进制"
    elif [ "$TARGET_CHANNEL" = "unknown" ]; then
        NEED_UPDATE=true
        UPDATE_REASON="数据目录版本未知，使用底包版本 ${SOURCE_VERSION}"
    elif [ "$SOURCE_CHANNEL" != "$TARGET_CHANNEL" ]; then
        # dev/stable 通道不同：用户换了镜像通道，用底包
        NEED_UPDATE=true
        if [ "$SOURCE_CHANNEL" = "dev" ]; then
            UPDATE_REASON="切换到 dev 版本"
        elif [ "$TARGET_CHANNEL" = "dev" ]; then
            UPDATE_REASON="从 dev 切换到正式版本 ${SOURCE_VERSION}"
        else
            UPDATE_REASON="版本通道变化（${TARGET_CHANNEL} → ${SOURCE_CHANNEL}）"
        fi
    elif [ "$SOURCE_BUILD_TYPE" != "$TARGET_BUILD_TYPE" ]; then
        # 类型不同（full↔lite）：用户换了镜像变体，用底包
        NEED_UPDATE=true
        UPDATE_REASON="构建类型切换（${TARGET_BUILD_TYPE} → ${SOURCE_BUILD_TYPE}）"
    elif [ "$SOURCE_CHANNEL" = "dev" ]; then
        if compare_build_times "$SOURCE_BUILD_TIME" "$TARGET_BUILD_TIME"; then
            # 同通道同类型 + dev：按构建时间选最新
            NEED_UPDATE=true
            UPDATE_REASON="检测到更新的 dev 构建（${TARGET_BUILD_TIME} → ${SOURCE_BUILD_TIME}）"
        else
            SKIP_REASON="数据目录 dev 构建时间（${TARGET_BUILD_TIME}）不低于底包（${SOURCE_BUILD_TIME}），无需替换"
        fi
    elif [ "$SOURCE_CHANNEL" = "stable" ]; then
        if compare_versions "$SOURCE_VERSION" "$TARGET_VERSION"; then
            # 同通道同类型 + release：按版本号选最新
            NEED_UPDATE=true
            UPDATE_REASON="检测到新版本（${TARGET_VERSION} → ${SOURCE_VERSION}）"
        else
            SKIP_REASON="数据目录版本（${TARGET_VERSION}）不低于底包版本（${SOURCE_VERSION}），无需替换"
        fi
    else
        SKIP_REASON="无法识别版本通道（${SOURCE_CHANNEL}），保留数据目录二进制"
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
        echo "✓ 版本相同，无需更新"
    fi
fi

# 确保二进制文件有执行权限
chmod +x "$BINARY_TARGET"

# 切换到工作目录
cd /app

echo ""
echo "启动 Songloft..."
echo ""

# 执行二进制文件，传递所有参数
exec "$BINARY_TARGET" "$@"
