# Home Assistant 加载项薄层：复用已发布的 songloft/songloft 镜像，
# 仅叠加一个 run.sh，把 HA 选项（/data/options.json）转成后端认识的环境变量/flag。
# 不重新编译 Go，构建极轻。
ARG BUILD_FROM
FROM ${BUILD_FROM}

# 基础镜像为 alpine，无 bashio；用 jq 手动解析 HA 选项。
RUN apk add --no-cache jq

COPY run.sh /run.sh
RUN chmod a+x /run.sh

# 重置基础镜像的 docker-entrypoint.sh（其二进制热替换逻辑在 HA 场景不需要，
# HA 通过重建镜像升级），改由 run.sh 直接启动后端。
ENTRYPOINT []
CMD ["/run.sh"]
