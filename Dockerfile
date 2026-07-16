# syntax=docker/dockerfile:1
# 启用 BuildKit 高级特性，支持缓存挂载

# 关键：固定在原生构建平台（amd64）编译，靠 Go 交叉编译产出目标架构二进制，
# 避免在 QEMU 模拟下跑 Go 编译 + UPX（arm64/armv7 会慢 6~8 倍）。
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS go-builder

WORKDIR /app

# CGO_ENABLED=0 交叉编译无需 gcc/musl-dev；仅保留 make/upx/git
RUN apk add --no-cache make upx git || \
    (sleep 5 && apk add --no-cache make upx git) || \
    (sleep 10 && apk add --no-cache make upx git)

# 设置 Go 缓存目录（使用标准路径）
ENV GOCACHE=/root/.cache/go-build
ENV GOMODCACHE=/go/pkg/mod

# 构建参数 - 通过 --build-arg 传入
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
ARG GOPROXY=https://goproxy.cn,https://goproxy.io,direct
ENV GOPROXY=${GOPROXY}
ARG TRACELY_APP_ID=
ARG TRACELY_APP_SECRET=
ARG TRACELY_HOST=

# buildx 自动注入的目标平台参数（用于 Go 交叉编译）
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

# 先复制 go.mod 和 go.sum，利用 Docker 层缓存加速依赖下载
COPY go.mod go.sum ./
# 创建目录并复制子模块的 go.mod/go.sum
RUN mkdir -p pkg/tag
COPY pkg/tag/go.mod pkg/tag/go.sum ./pkg/tag/
# 仅下载依赖，此层会被缓存（除非 go.mod/go.sum 变化）
RUN go mod download && go mod verify

# 再复制其余源码
COPY . .

# 使用缓存挂载加速编译（Go 编译缓存会被保留）
# 同时挂载 GOMODCACHE 和 GOCACHE
# Makefile 根据 VERSION=dev 自动启用 dev 编译标签（含 Swagger + pprof）
# 用 build-cross 交叉编译到 $TARGETOS/$TARGETARCH（GOARM 由 $TARGETVARIANT 推导），
# 始终在原生 amd64 上编译 + UPX，产物统一输出为 /app/songloft
ARG LITE_BUILD=false
ARG VERSION
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -x && \
    echo "VERSION=[${VERSION}] LITE_BUILD=[${LITE_BUILD}] TARGET=[${TARGETOS}/${TARGETARCH}/${TARGETVARIANT}]" && \
    case "${TARGETVARIANT}" in v7) GOARM=7;; v6) GOARM=6;; *) GOARM=;; esac && \
    if [ "$LITE_BUILD" = "true" ]; then \
        make build-cross GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" GOARM="${GOARM}" OUTPUT=songloft \
            EXTRA_TAGS=lite BUILD_TYPE=lite \
            GIT_COMMIT="${GIT_COMMIT}" BUILD_TIME="${BUILD_TIME}" \
            TRACELY_APP_ID="${TRACELY_APP_ID}" \
            TRACELY_APP_SECRET="${TRACELY_APP_SECRET}" \
            TRACELY_HOST="${TRACELY_HOST}" \
            ${VERSION:+VERSION=${VERSION}}; \
    else \
        make build-cross GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" GOARM="${GOARM}" OUTPUT=songloft \
            GIT_COMMIT="${GIT_COMMIT}" BUILD_TIME="${BUILD_TIME}" \
            TRACELY_APP_ID="${TRACELY_APP_ID}" \
            TRACELY_APP_SECRET="${TRACELY_APP_SECRET}" \
            TRACELY_HOST="${TRACELY_HOST}" \
            ${VERSION:+VERSION=${VERSION}}; \
    fi

FROM alpine:latest

# 分层顺序按「变更频率」由低到高排列：越少变动的层越靠前，
# 保证用户 docker pull 更新时只需下载末尾变动的二进制层，前面的大层命中本地缓存。

# 增加 ALSA 用户态运行时，解决容器内 MPD 打开 ALSA 设备时报
# "No such file or directory" 的问题
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    alsa-lib \
    alsa-plugins \
    alsa-utils \
    alsa-ucm-conf \
    pulseaudio-utils

# 设置默认时区为东八区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 创建挂载目录（静态，无外部依赖）
# /app/music - 音乐文件存储目录
# /app/data - 应用数据存储目录
RUN mkdir -p /app/music /app/data

# ffmpeg/ffprobe 体积大且极少变动 → 前置，长期命中缓存
COPY --from=hanxi/ffmpeg /ffmpeg /bin/ffmpeg
COPY --from=hanxi/ffmpeg /ffprobe /bin/ffprobe

# 启动脚本小、极少变动（--chmod 合并原独立 chmod 层）
COPY --chmod=0755 scripts/docker-entrypoint.sh /app/docker-entrypoint.sh

# 主程序二进制每次构建都变 → 放在最后一个 content 层，更新时仅此层需重新下载
COPY --from=go-builder /app/songloft /app/songloft

EXPOSE 58091

# 挂载点说明：
# /app/music - 音乐文件目录，建议挂载: -v /your/music/path:/app/music
# /app/data - 数据目录，建议挂载: -v /your/data/path:/app/data
ENV ADMIN_USERNAME=admin
ENV ADMIN_PASSWORD=admin
ENV IN_DOCKER=true

VOLUME ["/app/music", "/app/data"]

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD []
