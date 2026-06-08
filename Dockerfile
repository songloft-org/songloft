# syntax=docker/dockerfile:1
# 启用 BuildKit 高级特性，支持缓存挂载

FROM golang:1.26-alpine AS go-builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev make upx git || \
    (sleep 5 && apk add --no-cache gcc musl-dev make upx git) || \
    (sleep 10 && apk add --no-cache gcc musl-dev make upx git)

# 设置 Go 缓存目录（使用标准路径）
ENV GOCACHE=/root/.cache/go-build
ENV GOMODCACHE=/go/pkg/mod

# 构建参数 - 通过 --build-arg 传入
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
ARG GOPROXY=https://goproxy.cn,https://goproxy.io,direct
ENV GOPROXY=${GOPROXY}

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
ARG LITE_BUILD=false
ARG VERSION
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -x && \
    echo "VERSION=[${VERSION}] LITE_BUILD=[${LITE_BUILD}]" && \
    if [ "$LITE_BUILD" = "true" ]; then \
        make build-prod-lite GIT_COMMIT="${GIT_COMMIT}" BUILD_TIME="${BUILD_TIME}" BUILD_TYPE=lite ${VERSION:+VERSION=${VERSION}}; \
    else \
        make build-prod GIT_COMMIT="${GIT_COMMIT}" BUILD_TIME="${BUILD_TIME}" ${VERSION:+VERSION=${VERSION}}; \
    fi

FROM alpine:latest

# 增加 ALSA 用户态运行时，解决容器内 MPD 打开 ALSA 设备时报
# "No such file or directory" 的问题
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    alsa-lib \
    alsa-plugins \
    alsa-utils \
    alsa-ucm-conf

# 设置默认时区为东八区
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=hanxi/ffmpeg /ffmpeg /bin/ffmpeg
COPY --from=hanxi/ffmpeg /ffprobe /bin/ffprobe
COPY --from=go-builder /app/songloft /app/songloft
COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh

# 创建挂载目录
# /app/music - 音乐文件存储目录
# /app/data - 应用数据存储目录
RUN mkdir -p /app/music /app/data && \
    chmod +x /app/docker-entrypoint.sh

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
