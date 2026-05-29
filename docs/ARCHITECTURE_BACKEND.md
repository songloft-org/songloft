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
- **元数据读写**: hanxi/tag（dhowden/tag fork，增强编码检测;新增 MP3 ID3v2.3 与 FLAC Vorbis Comment 写入)
- **音频分析**: ffprobe（可选，用于获取精确技术参数）
- **JS 运行时**: QuickJS（modernc.org/quickjs，纯 Go 实现，用于 JS 插件脚本执行）
- **插件架构**: JS 脚本插件（QuickJS 沙盒 + 权限模型 + 健康检查 + 热更新）
- **监控**: Tracely 客户端（心跳包、错误上报、panic 捕获）

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
- `embed_common.go`: 静态文件服务公共工具（Base62 解码、路径安全校验等内部 helper；历史 `/music/*` `/cover/*` Base62 路径已下线，统一使用 `/api/v1/songs/{id}/play|cover|lyric`）
- `pprof_dev.go`: 开发模式下的 pprof 端点（`-tags dev`）
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
- `upgrade.go`: 版本升级（检查更新、执行升级、重置基础镜像）
- `proxy.go`: 资源代理（解决外部 CDN 的 CORS 限制，支持流式转发和 Range 请求）
- `cache.go`: 音乐缓存（按 hash 缓存网络歌曲，支持 HEAD 探测和 GET 下载）
- `convert.go`: 网络歌曲→本地歌曲转换（启动整歌单转换、进度查询、取消、自动转换开关）
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
- `cache_service.go`: 音乐缓存服务（按 hash 缓存网络歌曲文件，支持并发下载去重）
- `cache_service_song.go`: 缓存服务针对 song 维度的辅助（命中查找、关联清理等）
- `convert_service.go`: 网络歌曲→本地歌曲转换服务（按歌单分目录存储、文件名安全清理、限速防风控、自动模式 worker 限流、URL 歌词自动下载并 cache、转换后写入文件 tag/封面/歌词）
- `convert_progress.go`: 转换进度跟踪（按 playlistID 隔离的状态管理，支持多歌单并行进度）
- `internal_url.go`: 内部回环 URL 构造（把相对 URL 拼成 `http://127.0.0.1:{port}/...?access_token=...`，给 convert/cache 调插件用）
- `whitelist.go`: 域名白名单校验（SSRF 防护，阻止内网地址访问）
- `source/`: 音源适配子包 — `fetcher`（HTTP 取数据）、`resolver`（URL 解析）、`validator`（参数校验）、`orchestrator`（编排）、`metrics`（指标）。具体实现见 `internal/app/source_adapters.go` 的接口绑定

#### jsplugin/ - JS 插件管理层

- `plugin.go`: JS 插件运行时模型与状态机
- `manager.go`: JS 插件管理器（生命周期、异步加载、子路由注册）
- `loader.go`: 解包 `.jsplugin.zip` / 校验 manifest / 权限解析
- `package.go`: 安装/更新/卸载流程（含 hash 校验）
- `repository.go`: 仓储接口（实现见 `database/jsplugin_repository.go`）
- `api_bridge.go`: 宿主 API 桥接（http、storage、logger 等向 QuickJS 暴露）
- `communication.go`: 宿主 ↔ 插件 调用协议封装（请求/响应序列化）
- `invoke.go`: 调用插件入口函数的统一封装（带超时与错误规范化）
- `hash.go`: 文件指纹工具（用于 hot_reload 与 package 校验）
- `scheduler.go`: 调度器（避免 VM 并发竞态）
- `health.go`: 健康检查（通过 `jsruntime.HealthProbe` 探测，失败自动隔离）
- `hot_reload.go`: 热更新（基于文件 hash 指纹自动重载）
- `permissions.go`: 权限模型校验
- `service.go`: 插件实例服务壳层
- `routes.go`: 子路由挂载

#### jsruntime/ - JavaScript 运行时

- `runtime.go`: QuickJS 运行时环境管理（`JSEnv`），支持并行调用、事件收集、超时控制
- `polyfill.go`: JS Polyfill 代码（console、setTimeout/setInterval、Function.toString 等）
- `pendingjob.go`: 底层 `JS_ExecutePendingJob` 调用（处理 Promise 微任务）

#### version/ - 版本信息

- `version.go`: 版本号、Git Commit、构建时间、构建类型（通过 `-ldflags` 注入）

### `pkg/` 目录

存放可复用的公共包：

#### tag/ - 音频元数据读写库

- **读取**:MP3（ID3v1/ID3v2.2/2.3/2.4）、FLAC、OGG/Vorbis、M4A/MP4、WAV、DSF 格式;封面图片、歌词、编码检测
- **写入**(`WriteTag(filePath, opts)`):
  - ✅ MP3 ID3v2.3:TIT2/TPE1/TPE2/TALB/TYER/TCON/USLT/APIC
  - ✅ FLAC:Vorbis Comment + PICTURE block
  - ⚠️ M4A/MP4:返回 `ErrUnsupportedWrite`(TODO,需重组 moov + 更新 stco)
  - ⚠️ OGG:返回 `ErrUnsupportedWrite`(TODO,需重新分页 + 重算 CRC)
  - 临时文件 + `os.Rename`,原子化覆盖
- 命令行工具:`cmd/tag`、`cmd/sum`、`cmd/check`

## 构建系统

### 构建标签（Build Tags）

| 标签 | 说明 | 用途 |
|------|------|------|
| `dev` | 开发模式 | 包含 Swagger 文档，跳过前端嵌入 |
| `full` | 完整模式 | 嵌入 Flutter Web 构建产物到二进制 |
| 无标签 | 轻量模式 | 不嵌入前端，不含 Swagger |

### 前端嵌入机制

```
web_embed.go      (build tag: !full)  → 空 embed.FS
web_embed_full.go  (build tag: full)   → //go:embed all:songloft-player-build/web-embedded
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

Service 层注入 `database.DB` 接口；单表写直接拿 `db.SongRepository().Create(...)`，跨表写走 `db.RunInTx(ctx, func(ctx, uow *UnitOfWork) error { uow.Songs.Create(...); uow.PlaylistSongs.ReplaceSong(...) })`，详见 [DATABASE_MIGRATIONS.md](DATABASE_MIGRATIONS.md)。

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
- `/api/v1/cache/{hash}` - 音乐缓存接口（按 hash 缓存网络歌曲）
- `/api/v1/playlists/{id}/convert-to-local` - 启动歌单网络歌曲→本地歌曲转换
- `/api/v1/playlists/{id}/convert-progress` - 查询转换进度（GET）/ 取消（POST `/cancel`）
- `/api/v1/settings/auto-convert` - 自动转换开关（GET/PUT）
- `/api/v1/version` - 版本信息接口
- `/api/v1/health` - 健康检查接口

此外，音乐文件、封面图片、歌词通过歌曲 ID 端点访问（需 `access_token` query 参数认证）：
- `/api/v1/songs/{id}/play` — 流式返回音频（支持 local / remote / radio 三种类型 + Range）
- `/api/v1/songs/{id}/cover` — 歌曲封面（本地歌曲走本端，网络歌曲由 `MarshalJSON` 直出原始 CDN URL）
- `/api/v1/songs/{id}/lyric` — 纯文本 LRC 歌词

> 旧的 `/music/*` 和 `/cover/*` Base62 编码路径方案已下线，`embed_common.go` 内的 Base62 helper 已退化为内部工具，不再注册路由。

详细 API 文档请参考 Swagger 文档（开发环境下访问 `/swagger/index.html`）。
