# AGENTS.md

本文件为 AI 编程助手提供 Songloft 项目的**入口信息**：项目结构、常用命令、铁律与踩坑总结。代码本身就是真实来源的内容（目录树、依赖、API 表、表结构）请直接看代码或下方链接的详细文档。

> **详细文档**：
> - 架构：[整体](docs/ARCHITECTURE.md) · [后端](docs/ARCHITECTURE_BACKEND.md) · [前端](docs/ARCHITECTURE_FRONTEND.md)
> - 专题：[数据库操作](docs/DATABASE_MIGRATIONS.md) · [颜色系统](docs/COLOR_SYSTEM.md) · [API 响应格式](docs/API_RESPONSE.md) · [快速上手](docs/QUICK_START.md)
> - 插件开发：见 `plugin-toolchain/README.md`（独立仓库）
> - API：开发模式启动后访问 `/swagger/index.html`

---

## 项目概述

Songloft 是自托管本地音乐服务器（v2.0 起，前身为 MiMusic），多仓库结构：

| 目录 | 技术 | 说明 |
|------|------|------|
| `/` | Go 1.26 + Chi v5 + SQLite | 后端 API 服务（默认端口 58091，账号 admin/admin） |
| `/songloft-player` ([独立仓库](https://github.com/songloft-org/songloft-player)) | Flutter 3.29+ / Dart 3.7+ | 跨平台前端（6 平台） |
| `/plugin-toolchain` ([独立仓库](https://github.com/songloft-org/plugin-toolchain)) | TS + pnpm | JS 插件开发工具链（SDK / Builder / 脚手架） |
| `/jsplugins-src` | TS | JS 插件源码（子模块集合，每个插件在自己仓库下分发 release） |
| `/pkg/tag` | Go | 音频元数据**读写**库（基于上游 tag 库扩展 MP3/FLAC 写入） |

---

## 常用命令

```bash
# 后端
make run            # 启动（dev 模式，含 Swagger）
make build          # 编译开发版
make build-prod     # 编译生产轻量版（不含 Swagger/前端）
make test           # 测试
make check          # fmt + vet + test
make sqlc           # 重新生成 sqlc 代码（改了 queries/*.sql 后必跑）
make swagger        # 重新生成 API 文档

# 前端构建（产物落到 songloft-player-build/，供后端嵌入或独立部署）
make build-frontend-web-embedded   # 嵌入 Go 二进制用（隐藏 API 地址 UI）
make build-frontend-web            # 独立部署 web
make build-frontend-{linux,windows,macos,android,ios,all}

# 前端开发
cd songloft-player && flutter run -d chrome          # standalone
cd songloft-player && flutter run -d chrome --dart-define=DEPLOY_MODE=embedded
```

---

## 数据库规范（铁律）

> 完整操作步骤见 [docs/DATABASE_MIGRATIONS.md](docs/DATABASE_MIGRATIONS.md)。

访问栈：**goose 迁移 + sqlc 固定 SQL + squirrel 动态 SQL + Repository + UnitOfWork**。

- **改 schema** → `internal/database/migrations/000N_xxx.sql`，启动时 `goose.Up` 自动执行；**禁止**手动 `ALTER data/songloft.db`
- **加固定 SQL** → `database/queries/{table}.sql` + `make sqlc`；生成产物 `database/sqlc/` 必须入库
- **动态 SQL（变长 WHERE/SET）** → 在 `*_repository.go` 内用 squirrel，禁止拼字符串
- **跨表写** → `db.RunInTx(ctx, func(ctx, uow))` 拿同一 `*sql.Tx` 下的 `uow.Songs/Playlists/...`；**禁止** service 层手 `BeginTx`，否则会 SQLITE_BUSY
- **错误语义** → 仓储未命中统一 `database.ErrNotFound`；service 用 `errors.Is` 判别
- **测试** → `testutil.OpenMemoryDB(t)` 跑真实 `:memory:` + 真实 Repository；**禁止**手写 mockDB
- **内置数据** → 迁移预置歌单 id=1「收藏」、id=2「电台收藏」（`labels=["built_in"]`），及 `music_path / jwt_secret / source_*` 默认 config。测试行数断言记得扣掉

---

## 后端编码约定

- 标准 Go layout（`internal/` 防外部依赖），Chi v5 路由，JWT 双 Token
- 依赖注入：service 层只接收 Repository 接口，**不接收** `DB`
- 日志：标准库 `slog`；HTTP 错误：统一 `respondError`
- **API 响应格式**：RESTful 直返，**禁止** `{code, data, message}` 信封；错误统一 `{"error","detail"}`。完整规范见 [docs/API_RESPONSE.md](docs/API_RESPONSE.md)
- 不用 ORM：固定 SQL → sqlc，动态 SQL → squirrel，跨表写 → `RunInTx + UnitOfWork`
- 测试文件 `*_test.go` 与源码同目录

---

## 构建与部署

- 构建标签：`dev`（含 Swagger，不嵌前端） / `full`（嵌 Flutter Web） / 无标签（轻量）
- 嵌入路径是 `songloft-player-build/web-embedded`（**不是** `songloft-player/build/web-embedded`）
- SPA 回退：`internal/app/embed.go` 处理，文件不存在时返回 `index.html`
- 部署模式由 `--dart-define=DEPLOY_MODE=embedded|standalone` 切换，`AppConfig.isEmbedded` 是编译时常量，tree-shaking 会移除独立模式下的 API 地址 UI

---

## 平台适配踩坑

- 升级检查 (`/api/v1/upgrade/check`) 仅 Docker 可用
- Flutter `secure_storage` 在 macOS 未签名沙盒下自动降级到 SharedPreferences
- Android 构建前需 `sdkmanager --licenses`；Android 13+ 需运行时申请通知权限
- Windows/Linux 音频后端走 `just_audio_media_kit`（libmpv）
- HyperOS3 等需 `androidStopForegroundOnPause: false` 防后台回收

---

## JS 插件

- 源码 `jsplugins-src/<name>/`，构建产物在各插件仓库的 GitHub Releases
- 新建插件：`pnpm create songloft-plugin <name>`（详见 `plugin-toolchain/README.md`）
- 沙盒：QuickJS，通过 `internal/jsruntime` 提供的 `host` 桥接调用宿主能力（`http.fetch`、`storage`、`logger`）
- 路由：`/api/v1/jsplugin/{entry_path}/...`
- 权限：manifest 中 `permissions: ["network", "storage", "fs:music", ...]`，运行时由 `internal/jsplugin` 校验
- 健康检查 + 文件指纹热更新均自动进行

---

## 业务踩坑总结（重要 — 不在代码里）

### scan 标题规则

- tag 有 title → 直接用 `tag.Title`
- tag 没 title → 文件名去扩展名
- **不要**再做"最长公共子串去重 + 拼接"，会产生"艺术家 - 标题"这种把艺术家冗余到标题字段的结果

### tag 写入（pkg/tag）

- `tag.WriteTag(filePath, opts)` 按扩展名 dispatch：MP3 → ID3v2.3，FLAC → Vorbis Comment + PICTURE
- M4A/OGG 暂未实现 → 返回 `ErrUnsupportedWrite`，调用方**必须**降级为日志，**不要**阻塞主流程
- 写入用临时文件 + `os.Rename`，原子化

### 歌单转本地（convert_service）

- 落地路径：`music_path/{清理后歌单名}/{清理后艺术家} - {清理后标题}.{ext}`，与 scanner 保持**相对路径**格式
  - **不要** `filepath.Abs`，否则重扫去重失败，产生重复 song
- cache miss 时**不走** `cache_service.DownloadToCache`，走 `convert_service.fetchToTemp` 直接 HTTP GET
  - 原因：部分 JS 插件的 `music/url/{hash}` 在 cache inflight 时会 302 回 cache endpoint → 自循环死锁
- 转换时把 song.URL 相对路径拼成 `http://127.0.0.1:{port}/...?access_token={plugin_token}`（用 `authService.GeneratePluginToken`）
- 手动批量：cache miss 触发新下载后串行 `sleep 3s + 0-2s jitter` 防风控；cache 命中不限速
- `LyricSource == "url"` 时自动 GET 歌词 URL（期望 `{"code":0,"data":{"lyric":"..."}}`），回填后改 `lyric_source = "cached"`
- 转换后 `tag.WriteTag` 写入 title/artist/album/year/lyrics/cover
- 一致性：`CreateSong + ReplaceSongInPlaylist` 必须**同一 Tx**；原 remote song 仅在 `CountPlaylistsContainingSong == 0` 时清理
- 互斥：同歌单同时只允许一个转换任务，后到者让位

### 文件搬移：跨设备 rename 陷阱

- `os.Rename` 在 src 和 dst 不在同一文件系统（挂载点）时会返回 `syscall.EXDEV`（cross-device link）错误
- 典型场景：`os.CreateTemp("")` 创建在系统 `/tmp`（tmpfs），目标 cache/music 目录挂载在独立磁盘或 Docker volume
- **统一使用** `internal/services.moveFile(src, dst)` 替代裸 `os.Rename`：先尝试 rename，EXDEV 时自动回退 copy + remove
- `pkg/tag` 的原子写不受影响：它用 `os.CreateTemp(dir, ...)` 在源文件**同目录**创建临时文件，rename 一定同设备
- 新增下载/缓存逻辑如果需要"先写临时文件再挪到目标位置"，**必须**用 `moveFile`，**不要**裸 `os.Rename`
