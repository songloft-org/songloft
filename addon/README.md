# Songloft — Home Assistant 加载项

本目录是 Songloft 的 **Home Assistant OS（HAOS）加载项仓库**。用户在 HA「加载项商店 → 仓库」添加本仓库地址即可一键安装。

> **本文件面向维护者 / AI**，讲清楚这套加载项怎么设计、为什么这么设计、怎么本地验证、怎么发版。
> **终端用户文档**（安装步骤、选项说明）在 [`songloft/DOCS.md`](songloft/DOCS.md)（双语，HA 详情页展示）；
> README 主站也有「Home Assistant 加载项」小节。

## 目录结构

```
addon/
├── repository.yaml            # 加载项仓库元信息（供用户「添加仓库」识别）
└── songloft/
    ├── config.yaml            # 加载项核心元数据（arch/ports/options/schema/map/webui）
    ├── build.yaml             # 各架构 base image = songloft/songloft:<tag>
    ├── Dockerfile             # FROM base image + run.sh 薄层
    ├── run.sh                 # 读 /data/options.json → 转 env → 启动后端
    ├── DOCS.md                # 用户向文档（双语，HA 详情页）
    ├── icon.png / logo.png    # 商店展示图（由 docs/public/logo.png 生成）
    └── translations/{en,zh}.yaml  # 选项字段的本地化标签
```

## 设计决策与踩坑（重要）

### 1. 薄层复用已发布镜像，不重新编译 Go
`Dockerfile` 只做 `FROM songloft/songloft:<tag>` + 叠一个 `run.sh`，**不重新编译 Go**。base image 由 `build.yaml` 指定，复用 CI 已推送到 Docker Hub 的多架构 manifest。构建极轻（只加一层脚本）。

### 2. 音乐目录走 `MUSIC_DIR` 环境变量，不用 `-music` flag
`run.sh` 用 `MUSIC_DIR` env 传音乐目录（默认 `/media`），**不用 `-music` flag**。原因：
- `-music` flag 曾长期未进入发布版，用未知 flag 启动会让后端**直接崩溃**（`flag provided but not defined`）。
- 基础镜像声明了 `VOLUME /app/music`，容器内该路径是挂载点，`rm`/`ln -s` 会 `Resource busy`，**无法用 symlink 把 `/app/music` 重定向到 `/media`**。
- **env 的优势：未知环境变量会被旧后端静默忽略，不崩溃** → 优雅降级。不支持 `MUSIC_DIR` 的镜像仍能正常启动（音乐目录回落默认），用户可在 Web UI 手动设 `/media`。

后端侧入口在 `internal/app/app.go` 的 `ParseConfig`（`MUSIC_DIR` 与 `LISTEN_PORT`/`BASE_PATH`/`DB_PATH` 同款 env fallback，优先级低于 `-music` flag）。

### 3. 不设 `image:` 字段 → 本地自构建
`config.yaml` **故意不设 `image:`**，从而走 `build.yaml` 本地自构建。若用纯 `image:` 引用预构建镜像，就**无法把 HA 选项页填的账号密码/音乐路径注入进去**（HA 只把选项写到 `/data/options.json`，得靠 `run.sh` 读取转 env）。

### 4. 数据持久化到 `/data`，音乐映射 `/media` `/share`
`run.sh` 设 `DB_PATH=/data/songloft.db`，DB/缓存/插件数据落到 HA 持久化目录 `/data`，卸载重装不丢。音乐来源目录 `map: [media:rw, share:rw]`。

### 5. `ENTRYPOINT []` 重置基础镜像入口
基础镜像的 `docker-entrypoint.sh` 含二进制热替换逻辑（Docker 场景用），HA 场景不需要（HA 靠重建镜像升级）。`Dockerfile` 用 `ENTRYPOINT []` 重置，改由 `run.sh`（`CMD`）直接启动。

### 6. 架构对应
`config.yaml` 的 `arch: [amd64, aarch64, armv7]` 对应镜像 manifest 的 `linux/amd64`、`linux/arm64`、`linux/arm/v7`（已由 CI `--platform` 确认齐全）。

## CI 版本同步

`.github/workflows/release.yml` 的 `create-release` job 在**正式发布（tag 触发）**时，用 `sed` 把：
- `config.yaml` 的 `version:` → 本次发布版本号（决定 HA「有可用更新」提示）
- `build.yaml` 的三处 `songloft/songloft:<tag>` → 本次版本（让自构建拉该版本 base image，而非漂移的 `:latest`）

然后 commit 回 `main`。dev/dispatch 构建不同步，保持占位。

## 本地验证

不装 HA 也能验证核心逻辑：

```sh
docker build -t songloft-addon --build-arg BUILD_FROM=songloft/songloft:latest addon/songloft
mkdir -p /tmp/sl-data /tmp/sl-media
echo '{"admin_username":"admin","admin_password":"test123","music_path":"/media","base_path":""}' > /tmp/sl-data/options.json
docker run --rm -p 58091:58091 -v /tmp/sl-data:/data -v /tmp/sl-media:/media songloft-addon
curl -s http://localhost:58091/api/v1/version   # 应返回版本；确认 DB 落在 /data/songloft.db
```

HA 端真机：加载项商店 → 添加仓库 `https://github.com/songloft-org/songloft` → 安装 → 配置页填选项 → 启动 → 「打开 Web UI」。

## 生效前提

音乐目录在 HA 端全自动，需 base image 是**包含 `MUSIC_DIR` 支持的发布版**。旧镜像上加载项仍可安装运行，仅音乐目录需在 Web UI 手动设 `/media`（优雅降级，见上文第 2 点）。
