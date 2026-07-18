# Songloft 后端架构说明

## 技术栈

- **Go 版本**: 1.26+
- **Web 框架**: Chi v5.2.4
- **认证方式**: JWT 双 Token 认证（Access Token + Refresh Token）
- **数据库**: SQLite 3 (modernc.org/sqlite v1.46.1，纯 Go CGO-free 实现)
- **数据库访问栈**:
  - `pressly/goose v3` — schema 迁移（启动时自动 `Up`，文件在 `migrations/000N_xxx.sql`）
  - `sqlc-dev/sqlc` — 固定 SQL 生成类型安全代码（`queries/*.sql` → `sqlc/*.sql.go`，CLI 时生成）
  - `Masterminds/squirrel v1.5` — 动态 SQL 构造（变长 WHERE/SET/ORDER/分页）
  - Repository + UnitOfWork 自封装层，事务通过 `db.RunInTx(ctx, func(ctx, *UnitOfWork))` 完成
- **元数据读写**: hanxi/tag（dhowden/tag fork，增强编码检测;新增多格式写入:MP3 / FLAC / M4A·MP4 / OGG(.ogg/.oga) / APE / WAV / AIFF)
- **音频分析**: ffprobe（可选，用于获取精确技术参数）
- **JS 运行时**: QuickJS（modernc.org/quickjs，纯 Go 实现，用于 JS 插件脚本执行）
- **插件架构**: JS 脚本插件（QuickJS 沙盒 + 权限模型 + 健康检查 + 热更新）
- **监控**: Tracely 客户端（心跳包、安装/升级统计、panic 捕获）

## 架构设计

### 分层架构

```
HTTP Server (main.go)
  → Config (internal/config/)
  → Routes + Middleware (internal/app/routers.go + internal/middleware/)
  → Handlers (internal/handlers/)
  → Services (internal/services/)
        │
        ├── Database 主路径
        │     → Repository / UnitOfWork (internal/database/*_repository.go, unit_of_work.go)
        │     → sqlc 固定 SQL (internal/database/sqlc/) + squirrel 动态 SQL
        │     → SQLite (data/songloft.db, goose 迁移管理 schema)
        │
        └── JS 插件侧路径（按需）
              → JS Plugin Manager (internal/jsplugin/)
              → JS Runtime — QuickJS 沙盒 (internal/jsruntime/)
```

> Services 与 Database 是核心数据流；JS 插件是侧链能力（HTTP 转发到 `jsplugin.Manager` 后进入 QuickJS 沙盒），不在主写路径上。

## 包结构说明

### `internal/` 目录

存放项目的核心业务逻辑，按照功能模块划分：

#### app/ - 应用程序入口和配置

- `app.go`: 应用程序主结构（`App`）和初始化逻辑，包含依赖注入、服务创建、信号处理
- `routers.go`: 路由配置和注册，定义所有 API 路由及中间件链
- `router_dev.go`: 开发环境路由（包含 Swagger，`-tags dev`）
- `router_prod.go`: 生产环境路由（不包含 Swagger）
- `embed.go`: Flutter Web 前端静态资源服务，支持 SPA 路由回退（请求文件不存在时返回 `index.html`）
- `access_log.go`: HTTP 访问日志中间件
- `compress.go`: 响应压缩中间件
- `db_migration.go`: 启动时执行 goose 数据库迁移的封装
- `pprof_dev.go`: 开发模式下的 pprof 端点（`-tags dev`）

> 历史 `/music/*` `/cover/*` 的 Base62 短链方案已完全下线，统一使用 `/api/v1/songs/{id}/play|cover|lyric`；相关 `embed_common.go` helper 已随路由删除，`routers.go` 仅保留废弃注释。
- `source_adapters.go`: 把 services 层的实现适配为 `services/source/` 子包定义的接口（fetcher / resolver / validator 等）

#### config/ - 配置类型定义

- `types.go`: 应用配置结构体 `AppConfig`（端口、数据库路径、用户名密码等）

#### handlers/ - 请求处理器

- `auth.go`: 认证相关请求（登录、刷新令牌、登出、令牌管理）
- `music.go`: 歌曲 CRUD、批量删除、歌词更新
- `playlist.go`: 歌单 CRUD、歌曲排序、封面上传、自动创建歌单
- `config.go`: 配置管理
- `scan.go`: 扫描管理（异步扫描、进度查询、取消扫描）
- `jsplugin.go`: JS 插件管理（上传 `.jsplugin.zip`、启用/禁用、删除、更新检查）
- `jsplugin_registry.go`: JS 插件订阅源管理（源列表读写、拉取源清单、下载 ZIP 安装）
- `upgrade.go`: 版本升级（检查更新、执行升级、重置基础镜像）
- `proxy.go`: 资源代理（解决外部 CDN 的 CORS 限制，支持流式转发和 Range 请求）。含 `ServeRemoteResourceWithCache` 流式代理上游音频到客户端并触发后台缓存
- `hls.go`: HLS 电台代理（服务端拉取并改写 m3u8、代理切片/key/init 段；`/settings/hls-proxy` 开关）
- `cache.go`: 音乐缓存管理（统计、清理、配置、自定义目录验证）
- `backup.go`: 数据备份与恢复（歌单/歌曲导出导入）
- `log.go`: 日志等级读写（`/settings/log-level`）
- `equalizer_setting.go` / `library_browse_setting.go` / `tab_config_setting.go` / `user_preferences_setting.go`: 各孤立配置端点（`/settings/*` 强类型配置）
- `version.go`: 版本信息
- `health.go`: 健康检查
- `response.go`: 统一 JSON 响应和错误响应工具函数

#### middleware/ - 中间件

- `auth.go`: JWT 认证中间件，验证 Access Token
- `auth_test.go`: 认证中间件测试

#### models/ - 数据模型

- `models.go`: 核心数据结构（Song、Playlist、Config、AuthToken、JSPlugin 等）及验证逻辑
- `constant.go`: 分页限制常量（DefaultPaginationLimit、MaxPaginationLimit）
- `models_test.go`: 模型验证测试

#### database/ - 数据库层（Repository + UnitOfWork + sqlc + goose）

- `database.go`: `DB` 接口（`Close / RunInTx / 各 *Repository()` getter）
- `sqlite.go`: `SQLiteDB` 实现（`Open()` 含 goose Up + WAL/busy_timeout 等 pragma，`RunInTx` 事务封装）
- `unit_of_work.go`: `UnitOfWork` 结构，事务作用域内的 Repository 集合（`Songs / Playlists / PlaylistSongs` 字段，绑定到同一 `*sql.Tx`）
- `errors.go`: 领域错误（`ErrNotFound` / `ErrConflict` 等哨兵）
- `filters.go`: squirrel 共用辅助（排序白名单、`applyOrder`、`applyPagination`）
- `config_repository.go`: 配置仓储（`ConfigRepository`）
- `song_repository.go`: 歌曲仓储（含 `UpsertRemoteSong`：按 `(plugin_entry_path, dedup_key)` 命中复用 ID，空 dedup_key 时退化为直接 INSERT）
- `playlist_repository.go`: 歌单仓储
- `playlist_song_repository.go`: 歌单-歌曲关联仓储（含 `ReplaceSong` 等）
- `token_repository.go`: 认证令牌仓储
- `jsplugin_repository.go`: JS 插件仓储
- `plugin_storage_repository.go`: JS 插件 KV 存储仓储（`host.storage` 桥接的后端存储）
- `migrations/`: goose 迁移源文件（`0001_init.sql` 等，通过 `embed.FS` 打包，启动时自动 Up）
- `queries/`: sqlc 输入（每张表一个 `*.sql`，跑 `make sqlc` 生成代码）
- `sqlc/`: sqlc 输出（`*.sql.go`，**已入库**，运行时不依赖 sqlc CLI）
- `testutil/`: `OpenMemoryDB(t)` 启动 `:memory:` SQLite 跑真实迁移 + 真实 Repository，供测试使用
- `sqlite_test.go`: 数据库层集成测试

#### services/ - 业务逻辑层

- `auth_service.go`: 认证服务（JWT 双 Token 生成/验证、令牌管理、密钥生成）
- `config_service.go`: 配置服务（数据库配置管理，支持 JSON 格式读写）
- `metadata.go`: 元数据提取服务（使用 hanxi/tag 提取标签和封面，ffprobe 获取技术参数）。标题策略:tag 有 title 优先用,缺失才用文件名(不再做最长公共子串拼接)
- `scanner.go`: 文件扫描服务（递归扫描音乐目录，支持排除目录和格式过滤）
- `scan_progress.go`: 扫描进度追踪（异步扫描状态管理）
- `song_service.go`: 歌曲服务（CRUD、批量操作、时长回填）
- `playlist_service.go`: 歌单服务（CRUD、歌曲管理、自动创建）
- `upgrade_service.go`: 版本升级服务（获取版本信息、执行升级、重置）
- `cache_service.go`: 音乐缓存服务（LRU 淘汰、自定义缓存目录、容量上限配置）
- `cache_service_song.go`: 缓存服务针对 song 维度的辅助（命中查找、并发下载去重、关联清理、流式代理回调等）
- `cache_path_template.go`: 路径模板渲染（`{artist}-{album}/{title}` 等占位符，供缓存与插件持久化使用）
- `cache_metadata_writer.go`: 文件元数据嵌入（标签写入 + 远程封面获取，供插件持久化使用）
- `song_downloader.go`: 歌曲持久化服务（插件基础设施：通过 `songs.download` Bridge API 将远程歌曲持久化到本地 `music_path`）
- `internal_url.go`: 内部回环 URL 构造（把相对 URL 拼成 `http://127.0.0.1:{port}/...?access_token=...`，给 convert/cache 调插件用）
- `whitelist.go`: 域名白名单校验（SSRF 防护，阻止内网地址访问）
- `source/`: 音源适配子包 — `fetcher`（HTTP 取数据 + URL 解析）、`resolver`（跨插件 fallback）、`validator`（参数校验）、`orchestrator`（编排，含 `ResolveURL` 仅解析不下载）、`metrics`（指标）。具体实现见 `internal/app/source_adapters.go` 的接口绑定

#### jsplugin/ - JS 插件管理层

- `plugin.go`: JS 插件运行时模型与状态机
- `manager.go`: JS 插件管理器（生命周期、异步加载、子路由注册）
- `loader.go`: 解包 `.jsplugin.zip` / 校验 manifest / 权限解析
- `package.go`: 安装/更新/卸载流程（含 hash 校验）
- `repository.go`: 仓储接口（实现见 `database/jsplugin_repository.go`）
- `registry.go`: 插件订阅源解析（拉取源清单、版本比对，供更新检查/安装用）
- `auto_update.go`: 插件自动更新（后台按订阅源检查并升级）
- `api_bridge.go`: 宿主 API 桥接总入口（向 QuickJS 暴露 http、storage、logger、songs（含 `songs.download` 持久化）、playlists 等能力）
- `api_bridge_net.go`: `songs.net` UDP Socket API（udpBind/多播 + reader goroutine 推送 onData）
- `api_bridge_tcp.go`: `songs.net.tcpConnect` 出站 TCP Socket API（仅私有/回环/链路本地地址，防 SSRF）
- `api_bridge_websocket.go`: WebSocket 客户端 API（连接/收发/事件推送）
- `api_bridge_fs.go`: 文件系统桥接（`fs:music` 等权限下的受限读写）
- `api_bridge_command.go`: 外部命令调用桥接
- `communication.go`: 宿主 ↔ 插件 调用协议封装（请求/响应序列化）
- `invoke.go`: 调用插件入口函数的统一封装（带超时与错误规范化）
- `hash.go`: 文件指纹工具（用于 hot_reload 与 package 校验）
- `scheduler.go`: 调度器（避免 VM 并发竞态）
- `health.go`: 健康检查（通过 `jsruntime.HealthProbe` 探测，失败自动隔离）
- `hot_reload.go`: 热更新（基于文件 hash 指纹自动重载）
- `permissions.go`: 权限模型校验
- `service.go`: 插件实例服务壳层
- `routes.go`: 子路由挂载
- `assets/`: 嵌入的插件公共资源（`common.css`/`common.js`/字体，经 `/api/v1/jsplugin-assets/*` 提供并自动注入插件页面）

#### jsruntime/ - JavaScript 运行时

- `runtime.go`: QuickJS 运行时环境管理（`JSEnv`），支持并行调用、事件收集、超时控制
- `polyfill.go`: JS Polyfill 代码（console、setTimeout/setInterval、Function.toString 等）
- `pendingjob.go`: 底层 `JS_ExecutePendingJob` 调用（处理 Promise 微任务）

#### version/ - 版本信息

- `version.go`: 版本号、Git Commit、构建时间、构建类型（通过 `-ldflags` 注入）

### `pkg/` 目录

存放可复用的公共包：

#### tag/ - 音频元数据读写库

- **读取**:MP3（ID3v1/ID3v2.2/2.3/2.4）、FLAC、OGG/Vorbis、M4A/MP4、WAV、APE、AIFF、DSF 格式;封面图片、歌词、编码检测
- **写入**(`WriteTag(filePath, opts)`,按扩展名 dispatch,均为临时文件 + `os.Rename` 原子写入):

  | 格式 | 文本字段 | 歌词 | 封面 |
  |------|---------|------|------|
  | MP3 | ID3v2.3 text frames | USLT | APIC |
  | FLAC | Vorbis Comment | LYRICS | PICTURE block |
  | M4A/MP4/M4B/MOV | iTunes atoms (©nam 等) | ©lyr | covr |
  | OGG(.ogg/.oga) | Vorbis Comment | LYRICS | METADATA_BLOCK_PICTURE (base64) |
  | APE | APEv2 text items | Lyrics | Cover Art (Front) |
  | WAV | RIFF LIST INFO | ICMT | 不支持（格式限制） |
  | AIFF/AIF | ID3v2.3 (ID3 chunk) + NAME/AUTH | USLT (ID3 chunk) | APIC (ID3 chunk) |

  - 其它扩展名返回 `ErrUnsupportedWrite`,调用方降级为日志、不阻塞主流程
- 命令行工具:`cmd/tag`、`cmd/sum`、`cmd/check`

## 构建系统

### 构建标签（Build Tags）

| 标签 | 说明 | 用途 |
|------|------|------|
| `dev` | 开发模式 | 包含 Swagger 文档 + pprof |
| `lite` | 精简模式 | 不嵌入前端，体积更小 |
| 无标签 | 完整模式（默认） | 嵌入 Flutter Web 构建产物到二进制 |

### 前端嵌入机制

```
web_embed.go      (build tag: !lite)  → //go:embed all:songloft-player-build/web-embedded
web_embed_lite.go  (build tag: lite)   → 空 embed.FS
```

## 设计模式

### 依赖注入

```go
// 通过构造函数注入依赖
func NewAuthHandler(authService *services.AuthService) *AuthHandler {
    return &AuthHandler{
        authService: authService,
    }
}
```

### 接口抽象

`database.DB` 只暴露事务入口和各 Repository getter，CRUD 逻辑全部下沉到 Repository：

```go
type DB interface {
    Close() error
    RunInTx(ctx context.Context, fn func(context.Context, *UnitOfWork) error) error

    SongRepository() *SongRepository
    PlaylistRepository() *PlaylistRepository
    PlaylistSongRepository() *PlaylistSongRepository
    ConfigRepository() *ConfigRepository
    TokenRepository() *TokenRepository
    JSPluginRepository() *JSPluginRepository
}
```

Service 层注入 `database.DB` 接口；单表写直接拿 `db.SongRepository().Create(...)`，跨表写走 `db.RunInTx(ctx, func(ctx, uow *UnitOfWork) error { uow.Songs.Create(...); uow.PlaylistSongs.ReplaceSong(...) })`，详见 [database_migrations.md](database_migrations.md)。

> 测试不再手写 mock，统一使用 `database/testutil.OpenMemoryDB(t)` 起 `:memory:` SQLite 跑真实迁移与真实 Repository。

## API 设计

后端提供 RESTful API，主要包括：

- `/api/v1/auth/*` - 认证相关接口（登录、刷新、登出、令牌管理）
- `/api/v1/songs/*` - 歌曲管理接口（CRUD、批量删除、歌词更新）
- `/api/v1/playlists/*` - 歌单管理接口（CRUD、歌曲排序、封面上传、自动创建）
- `/api/v1/configs/*` - 配置管理接口
- `/api/v1/jsplugins/*` - JS 插件管理接口（上传 `.jsplugin.zip`、启用/禁用、删除、更新检查）
- `/api/v1/jsplugin/{entry_path}/*` - JS 插件运行时路由（由插件 main.js 通过 SDK Router 注册）
- `/api/v1/scan/*` - 扫描管理接口（异步扫描、进度查询、取消）
- `/api/v1/upgrade/*` - 版本升级接口（仅 Docker 环境可用，含重置功能）
- `/api/v1/proxy` - 资源代理接口（解决 CORS，含 SSRF 防护）
- `/api/v1/cache-manage/*` - 音乐缓存管理（统计/清理/配置/目录验证）
- `/api/v1/settings/hls-proxy` - HLS 电台代理开关（GET/PUT）
- `/api/v1/settings/http-proxy` - 通用 HTTP 代理配置（GET/PUT）
- `/api/v1/settings/music-path` - 音乐路径与扫描排除（GET/PUT）
- `/api/v1/settings/plugin-registries` - 插件订阅源列表（GET/PUT）
- `/api/v1/settings/log-level` - 日志等级（GET/PUT）
- `/api/v1/settings/scan-auto-create-playlists` - 扫描后是否自动创建目录歌单（GET/PUT）
- `/api/v1/settings/scan-playlist-mode` - 目录歌单归并模式 directory/top_level/bubble_up（GET/PUT）
- `/api/v1/version` - 版本信息接口
- `/api/v1/health` - 健康检查接口

此外，音乐文件、封面图片、歌词通过歌曲 ID 端点访问（需 `access_token` query 参数认证）：
- `/api/v1/songs/{id}/play` — 流式返回音频（支持 local / remote / radio 三种类型 + Range）
- `/api/v1/songs/{id}/cover` — 歌曲封面（本地歌曲走本端，网络歌曲由 `MarshalJSON` 直出原始 CDN URL）
- `/api/v1/songs/{id}/lyric` — 纯文本 LRC 歌词

> 旧的 `/music/*` 和 `/cover/*` Base62 编码短链方案已完全下线，相关 helper 已随路由一并删除（`routers.go` 仅保留废弃注释）。

详细 API 文档请参考 Swagger 文档（开发环境下访问 `/swagger/index.html`）。
