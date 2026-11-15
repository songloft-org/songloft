# API 接口设计

本文档基于以下源文件编写：

- [internal/app/routers.go](https://github.com/songloft-org/songloft/blob/main/internal/app/routers.go) -- 完整路由注册（Chi v5 路由树）
- [internal/handlers/response.go](https://github.com/songloft-org/songloft/blob/main/internal/handlers/response.go) -- 响应辅助函数（respondJSON / respondError）
- [internal/handlers/*.go](https://github.com/songloft-org/songloft/blob/main/internal/handlers) -- 各业务 Handler 定义与 Swagger 注释
- [internal/middleware/auth.go](https://github.com/songloft-org/songloft/blob/main/internal/middleware/auth.go) -- JWT 认证中间件（Bearer + query param）
- [internal/jsplugin/routes.go](https://github.com/songloft-org/songloft/blob/main/internal/jsplugin/routes.go) -- JS 插件静态/API 路由注册
- [docs/api_response.md](https://github.com/songloft-org/songloft/blob/main/docs/api_response.md) -- API 响应格式规范
- [AGENTS.md](https://github.com/songloft-org/songloft/blob/main/AGENTS.md) -- API 文档规范 + 配置接口规范铁律

## 目录

1. [设计原则](#1-设计原则)
2. [响应格式规范](#2-响应格式规范)
3. [路由组织结构](#3-路由组织结构)
4. [认证机制](#4-认证机制)
5. [Handler 创建模式](#5-handler-创建模式)
6. [三种配置接口对比](#6-三种配置接口对比)
7. [Swagger 文档规范](#7-swagger-文档规范)
8. [完整路由清单](#8-完整路由清单)

---

## 1. 设计原则

**章节来源**: `docs/api_response.md`、`AGENTS.md`（后端编码约定）

Songloft 后端 API 遵循以下核心原则：

- **RESTful 直返**: 成功响应直接返回业务模型或集合，不使用 `{code, data, message}` 统一信封。HTTP 状态码即语义，`code` 字段与之重复属于冗余设计。
- **错误格式统一**: 所有 JSON API 端点的错误响应统一为 `{"error": "...", "detail": "..."}`，禁止使用 `message`、`msg`、`reason` 等替代字段名。
- **资源导向路径**: URL 路径以名词复数表示资源集合（`/songs`、`/playlists`），HTTP 方法表达操作语义（GET 查、POST 增、PUT 改、DELETE 删）。
- **分页标准化**: 列表接口通过 `limit` + `offset` 查询参数分页，响应包含 `total`、`limit`、`offset` 元数据。
- **二进制流例外**: 播放（`/songs/{id}/play`）、代理（`/proxy`）、静态文件等二进制流端点的错误可使用 `http.Error()` 纯文本，因为客户端不期望 JSON body。

---

## 2. 响应格式规范

**章节来源**: `internal/handlers/response.go`、`docs/api_response.md`

### 2.1 响应辅助函数

所有 handler 通过 `respondJSON(w, status, data)` 输出成功响应（data 直接序列化为顶层 JSON），通过 `respondError(w, status, message, err)` 输出错误（自动构建 `{"error","detail"}`）。中间件层使用独立的 `respondAuthError`，格式一致。

### 2.2 三种成功响应形态

| 场景 | 格式 | 示例 |
|------|------|------|
| 单个实体 | 模型直接序列化 | `{"id":1, "title":"Track", ...}` |
| 分页列表 | 集合名 + 分页元数据 | `{"songs":[...], "total":100, "limit":20, "offset":0}` |
| 操作结果 | 消息字符串 | `{"message": "歌曲已删除"}` |

### 2.3 错误响应格式

```json
{
  "error": "人类可读的错误信息",
  "detail": "可选的底层技术细节"
}
```

- `error` -- 必有，面向用户的简短描述
- `detail` -- 可选，仅当 `err != nil` 时输出底层错误信息

对应结构体 `models.ErrorResponse`，禁止自定义字段名。

---

## 3. 路由组织结构

**章节来源**: `internal/app/routers.go`、`internal/jsplugin/routes.go`

### 3.1 路由树概览

路由注册分三层，由 `App.setupRouter()` 统一编排：

```
chi.Router (根)
├── 全局中间件: Compress → Logger → Recoverer → RequestID → CORS
├── 前端静态文件 / Swagger (dev 构建)
├── /api/v1 ─┬─ [公开] /auth/login, /auth/refresh, /version, /health
│            └─ [Bearer] /auth/*, /songs/*, /playlists/*, /settings/*,
│                        /configs/*, /scan/*, /cache-manage/*, /upgrade/*, /proxy
├── /api/v1/jsplugins/* [Bearer] 插件 CRUD + 注册表 + /plugins/health
├── /api/v1/jsplugin/{entryPath}[/static/*] [公开] 插件静态资源
├── /api/v1/jsplugin-assets/* [公开] 公共 CSS/JS/字体
└── /api/v1/jsplugin/{entryPath}/* [Bearer+PublicPathChecker] API 转发
```

### 3.2 中间件栈

全局中间件在 `setupBaseRouter()` 中注册，按执行顺序：Compress(Gzip) -> RequestLogger(slog) -> Tracely(panic 上报) -> Recoverer(500) -> RequestID -> CORS -> CORP。认证中间件 `AuthMiddleware` 不是全局中间件，而是在各路由组内部按需添加。

### 3.3 路由分组策略

Chi v5 的 `r.Group()` 划分认证边界：同一 `/api/v1` 前缀下，公开端点直接注册，需认证端点通过 `r.Group` + `r.Use(AuthMiddleware)` 包裹。JS 插件路由独立注册，静态资源无需认证，API 转发支持 `PublicPathChecker` 接口豁免插件声明的公开路径。

---

## 4. 认证机制

**章节来源**: `internal/middleware/auth.go`

### 4.1 JWT 双 Token 机制

系统使用 JWT 双 Token 认证：
- **Access Token** -- 短期有效，用于 API 请求认证
- **Refresh Token** -- 长期有效，用于刷新 access token

### 4.2 Token 传递方式

认证中间件按优先级依次尝试两种方式获取 token：

1. **Authorization Header**（优先）: `Authorization: Bearer <token>`
2. **Query Parameter**（回退）: `?access_token=<token>`

query parameter 回退主要服务于无法自定义 Header 的场景：`<img>` 标签加载封面、`<audio>` 标签加载音频、`CachedNetworkImage` 等。

### 4.3 公开端点

以下端点无需认证，直接在 AuthMiddleware 外注册：

| 端点 | 用途 |
|------|------|
| `POST /api/v1/auth/login` | 用户登录 |
| `POST /api/v1/auth/refresh` | 刷新 token |
| `GET /api/v1/version` | 版本信息 |
| `GET /api/v1/health` | 健康检查 |
| `GET /api/v1/jsplugin/{entryPath}` | 插件静态页面 |
| `GET /api/v1/jsplugin/{entryPath}/static/*` | 插件静态资源 |
| `GET /api/v1/jsplugin-assets/*` | 插件公共资源 |

此外，插件通过 manifest 中 `publicPaths` 声明的 API 路径，由 `PublicPathChecker` 接口在认证中间件内部豁免。

---

## 5. Handler 创建模式

**章节来源**: `internal/handlers/*.go`、`internal/app/routers.go`

### 5.1 工厂函数模式

每个 Handler 遵循三步创建：(1) 结构体持有 service 依赖；(2) `NewXxxHandler(...)` 工厂函数接收 service 返回指针；(3) 可选 `SetXxx(fn)` Setter 解决循环依赖或延迟绑定。以 `SongHandler` 为例，工厂函数接收 `SongService`、`CacheService`、`AsyncReassigner` 等 6 个依赖，创建后通过 `SetGetMusicPath` 注入 Scanner 的路径获取函数。

### 5.2 Handler 清单

主要 Handler 位于 `internal/handlers/`：

- **业务核心**: `AuthHandler`(AuthService)、`SongHandler`(SongService + CacheService + 4 依赖)、`PlaylistHandler`(PlaylistService)、`BackupHandler`(BackupService)
- **配置管理**: `ConfigHandler`(ConfigService)、`ScanHandler`(SongService + Scanner + ConfigService)、`HLSHandler`(SongService + ConfigService)、`CacheHandler`(CacheService + ConfigService)、`LogHandler`(ConfigService + LevelVar)
- **插件/升级**: `JSPluginHandler`(PackageManager + Repository + Manager + SourceMetrics + ConfigService)、`UpgradeHandler`(UpgradeService)
- **工具类**: `ProxyHandler`(无依赖)、`VersionHandler`(无依赖)、`HealthHandler`(无依赖)

### 5.3 回调注入

部分 handler 通过 Setter 绑定跨模块回调：`configHandler.SetOnConfigChanged` 在通用 KV 写入后触发副作用，`scanHandler.SetOnMusicPathChanged` / `SetOnAutoScanChanged` 绑定配置变更后的重建逻辑，`songHandler.SetGetMusicPath` 延迟注入 Scanner 的路径函数。

---

## 6. 三种配置接口对比

**章节来源**: `AGENTS.md`（配置接口规范铁律）、`internal/app/routers.go`

项目中存在三种配置接口风格，各有明确分工：

| 维度 | `/settings/<name>` | `/<module>/config` | `/configs/{key}` |
|------|--------------------|--------------------|------------------|
| 定位 | 业务功能开关（用户可见） | 模块聚合配置 | admin 通用 KV 编辑 |
| 路径风格 | `/settings/<kebab-case>` | `/<module>/config` | `/configs/{key}` |
| 数据形态 | 强类型 JSON | 强类型 JSON | `{key, value}` 字符串 |
| 默认值 | handler 内部承担 | handler 内部承担 | 无（key 不存在返回 404） |
| 副作用 | PUT 内部直接触发 | PUT 内部直接触发 | 需挂 `onConfigChanged` 回调 |
| 归属 | 对应业务模块 handler | 模块 handler | ConfigHandler |
| 适用场景 | 孤立配置或跨模块共享 | 与模块动作端点强相关 | admin 调试/手编 |

### 6.1 业务端点 `/settings/<name>`

业务设置覆盖扫描、播放、插件、日志与客户端偏好。当前路径以 `internal/app/routers.go` 和[后端接口概览](../../architecture_backend.md)为准。数据均为强类型 JSON，如 `{enabled: bool}`、`{proxy: string}`；handler 承担默认值与副作用。

### 6.2 模块聚合端点

典型例子是缓存管理 `/cache-manage/*`，`config`（GET/PUT）与 `stats`、`clean`、`validate-dir` 共用前缀和 `CacheService`。适用于配置与模块动作端点强相关的场景。

### 6.3 通用 KV `/configs/{key}`

仅供前端通用配置编辑器（admin 手编），无强类型、无副作用（除非挂 `onConfigChanged` 回调）、key 不存在时 PUT 返回 404。新业务功能禁止直调。

### 6.4 双入口一致性

部分配置同时被业务端点和通用 KV 修改（如 `music_path`）。`routers.go` 中的 `musicPathChanged` 闭包确保两条入口共享同一副作用函数 -- 业务端点在 PUT handler 内直接触发，通用 KV 通过 `onConfigChanged` 回调触发。

---

## 7. Swagger 文档规范

**章节来源**: `AGENTS.md`（API 文档规范铁律）

### 7.1 铁律

凡在 `routers.go` 中注册的 handler 方法，必须有 swag 注释。没有豁免。

### 7.2 必填字段

每个 handler 至少包含以下 7 项 swag 注释：

| 字段 | 说明 |
|------|------|
| `@Summary` | 一行中文摘要 |
| `@Description` | 详细描述（副作用/默认值/错误码触发条件） |
| `@Tags` | 业务分组（中文），复用现有 tag |
| `@Produce` | 响应格式（通常 `json`） |
| `@Success` | 成功响应类型与说明 |
| `@Security` | `BearerAuth`（公开端点省略） |
| `@Router` | 路径与方法 |

有请求体的接口额外加 `@Accept json` 和 `@Param request body`。

### 7.3 现有业务 Tag

```
歌曲管理 | 歌单管理 | 电台与 HLS | 扫描管理 | 配置管理
缓存管理 | JS插件管理 | JS 插件 | 数据备份 | 设置
升级 | 认证
```

禁止随手创建新 tag。

### 7.4 多别名路由与验证

- 多条 alias 路径（如 `/songs/{id}/play` 与 `/songs/{id}/play.m3u8`）每条单写一行 `@Router`；HEAD 不单独列
- catch-all 路由列出所有实际方法；动态路由在 `@Description` 注明占位性质
- 修改注释后必须 `make swagger` 重新生成，产物（`docs/swagger.json`、`docs/swagger.yaml`、`docs/docs.go`）必须入库
- 验证：输出含新 `@Router` 路径 + `grep` swagger.json 命中 + 启动后 `/swagger/index.html` 目测

---

## 8. 完整路由清单

下表来自当前 `docs/swagger.json`，已对照路由注册；不单列 HEAD。动态路径由已安装插件决定，不能把 OpenAPI 占位当成固定业务接口。

### 歌曲

| 路径 | 方法 |
| --- | --- |
| `/api/v1/songs` | GET |
| `/api/v1/songs/batch-delete` | POST |
| `/api/v1/songs/clean` | POST |
| `/api/v1/songs/duplicates` | GET |
| `/api/v1/songs/facets` | GET |
| `/api/v1/songs/folders` | GET |
| `/api/v1/songs/ids` | GET |
| `/api/v1/songs/names` | GET |
| `/api/v1/songs/organize` | POST |
| `/api/v1/songs/organize/preview` | POST |
| `/api/v1/songs/radio` | POST |
| `/api/v1/songs/random` | GET |
| `/api/v1/songs/refresh-metadata` | POST |
| `/api/v1/songs/refresh-metadata/cancel` | POST |
| `/api/v1/songs/refresh-metadata/progress` | GET |
| `/api/v1/songs/remote` | POST |
| `/api/v1/songs/stats` | GET |
| `/api/v1/songs/{id}` | GET, PUT, DELETE |
| `/api/v1/songs/{id}/activate` | POST |
| `/api/v1/songs/{id}/artists` | GET, PUT |
| `/api/v1/songs/{id}/audio-tracks` | GET |
| `/api/v1/songs/{id}/cover` | GET |
| `/api/v1/songs/{id}/hls/playlist` | GET |
| `/api/v1/songs/{id}/hls/segment` | GET |
| `/api/v1/songs/{id}/lyric` | GET |
| `/api/v1/songs/{id}/lyrics` | PUT |
| `/api/v1/songs/{id}/play` | GET |
| `/api/v1/songs/{id}/play.m3u8` | GET |
| `/api/v1/songs/{id}/played` | POST |
| `/api/v1/songs/{id}/song-tags` | GET, PUT |
| `/api/v1/songs/{id}/tags` | PUT |
| `/api/v1/songs/{id}/tracks` | GET |
| `/api/v1/songs/{id}/video-hls/playlist.m3u8` | GET |
| `/api/v1/songs/{id}/video-hls/{path}` | GET |

### 歌单

| 路径 | 方法 |
| --- | --- |
| `/api/v1/playlists` | GET, POST |
| `/api/v1/playlists/batch-delete` | POST |
| `/api/v1/playlists/export` | GET |
| `/api/v1/playlists/import` | POST |
| `/api/v1/playlists/reorder` | PUT |
| `/api/v1/playlists/{id}` | GET, PUT, DELETE |
| `/api/v1/playlists/{id}/cover` | GET, POST |
| `/api/v1/playlists/{id}/pin` | PUT |
| `/api/v1/playlists/{id}/song-ids` | GET |
| `/api/v1/playlists/{id}/songs` | GET, POST |
| `/api/v1/playlists/{id}/songs/move` | PUT |
| `/api/v1/playlists/{id}/songs/reorder` | PUT |
| `/api/v1/playlists/{id}/songs/sort` | POST |
| `/api/v1/playlists/{id}/songs/{songId}` | DELETE |
| `/api/v1/playlists/{id}/sort` | PUT |
| `/api/v1/playlists/{id}/touch` | POST |
| `/api/v1/playlists/{id}/visibility` | PUT |

### 播放历史

| 路径 | 方法 |
| --- | --- |
| `/api/v1/play-history` | GET, DELETE |
| `/api/v1/play-history/entry` | DELETE |

### 认证

| 路径 | 方法 |
| --- | --- |
| `/api/v1/auth/login` | POST |
| `/api/v1/auth/logout` | POST |
| `/api/v1/auth/refresh` | POST |
| `/api/v1/auth/tokens` | GET |
| `/api/v1/auth/tokens/{token_id}` | DELETE |

### 通用配置

| 路径 | 方法 |
| --- | --- |
| `/api/v1/configs` | GET, POST |
| `/api/v1/configs/{key}` | GET, PUT, DELETE |

### 业务设置

| 路径 | 方法 |
| --- | --- |
| `/api/v1/settings/auto-scan` | GET, PUT |
| `/api/v1/settings/equalizer` | GET, PUT |
| `/api/v1/settings/github-discovery-token` | GET, PUT |
| `/api/v1/settings/github-proxy` | GET, PUT |
| `/api/v1/settings/hls-proxy` | GET, PUT |
| `/api/v1/settings/http-proxy` | GET, PUT |
| `/api/v1/settings/library-browse` | GET, PUT |
| `/api/v1/settings/log-level` | GET, PUT |
| `/api/v1/settings/music-path` | GET, PUT |
| `/api/v1/settings/plugin-auto-update` | GET, PUT |
| `/api/v1/settings/plugin-keep-alive` | GET, PUT |
| `/api/v1/settings/plugin-order` | GET, PUT |
| `/api/v1/settings/plugin-registries` | GET, PUT |
| `/api/v1/settings/proxy-private-allowlist` | GET, PUT |
| `/api/v1/settings/remote-title-source` | GET, PUT |
| `/api/v1/settings/scan-auto-create-playlists` | GET, PUT |
| `/api/v1/settings/scan-auto-fingerprint` | GET, PUT |
| `/api/v1/settings/scan-playlist-mode` | GET, PUT |
| `/api/v1/settings/scan-title-source` | GET, PUT |
| `/api/v1/settings/tab-config` | GET, PUT |
| `/api/v1/settings/tag-sync-to-file` | GET, PUT |
| `/api/v1/settings/theme-catalog-url` | GET, PUT |
| `/api/v1/settings/user-preferences` | GET, PUT |
| `/api/v1/settings/volume-normalize` | GET, PUT |

### 扫描与指纹

| 路径 | 方法 |
| --- | --- |
| `/api/v1/scan` | POST |
| `/api/v1/scan/cancel` | POST |
| `/api/v1/scan/dir-names` | GET |
| `/api/v1/scan/directories` | GET |
| `/api/v1/scan/fingerprints` | POST |
| `/api/v1/scan/fingerprints/cancel` | POST |
| `/api/v1/scan/fingerprints/failed` | GET |
| `/api/v1/scan/fingerprints/progress` | GET |
| `/api/v1/scan/fingerprints/status` | GET |
| `/api/v1/scan/progress` | GET |

### 缓存

| 路径 | 方法 |
| --- | --- |
| `/api/v1/cache-manage/clean` | POST |
| `/api/v1/cache-manage/config` | GET, PUT |
| `/api/v1/cache-manage/stats` | GET |
| `/api/v1/cache-manage/validate-dir` | POST |

### 插件管理

| 路径 | 方法 |
| --- | --- |
| `/api/v1/jsplugins` | GET |
| `/api/v1/jsplugins/registry/install` | POST |
| `/api/v1/jsplugins/registry/refresh` | POST |
| `/api/v1/jsplugins/storage/cleanup` | POST |
| `/api/v1/jsplugins/update-all` | POST |
| `/api/v1/jsplugins/upload` | POST |
| `/api/v1/jsplugins/{id}` | GET, PUT, DELETE |
| `/api/v1/jsplugins/{id}/check-update` | GET |
| `/api/v1/jsplugins/{id}/disable` | POST |
| `/api/v1/jsplugins/{id}/enable` | POST |
| `/api/v1/jsplugins/{id}/update` | POST |

### 插件运行时

| 路径 | 方法 |
| --- | --- |
| `/api/v1/jsplugin/{entryPath}` | GET |
| `/api/v1/jsplugin/{entryPath}/files/{path}` | GET, HEAD |
| `/api/v1/jsplugin/{entryPath}/static` | GET |
| `/api/v1/jsplugin/{entryPath}/static/{path}` | GET |
| `/api/v1/jsplugin/{entryPath}/{path}` | GET, PUT, POST, DELETE |

### 插件公共资源

| 路径 | 方法 |
| --- | --- |
| `/api/v1/jsplugin-assets/web-core/{path}` | GET |
| `/api/v1/jsplugin-assets/{path}` | GET |

### 插件健康度

| 路径 | 方法 |
| --- | --- |
| `/api/v1/plugins/health` | GET |

### 自定义标签

| 路径 | 方法 |
| --- | --- |
| `/api/v1/song-tags` | GET, POST |
| `/api/v1/song-tags/{id}` | GET, PUT, DELETE |
| `/api/v1/song-tags/{id}/bind` | POST |
| `/api/v1/song-tags/{id}/song-ids` | GET |
| `/api/v1/song-tags/{id}/songs` | GET |
| `/api/v1/song-tags/{id}/unbind` | POST |

### 主题包

| 路径 | 方法 |
| --- | --- |
| `/api/v1/theme-packs` | GET, POST |
| `/api/v1/theme-packs/active` | GET, PUT, DELETE |
| `/api/v1/theme-packs/catalog/install` | POST |
| `/api/v1/theme-packs/catalog/refresh` | POST |
| `/api/v1/theme-packs/{themeID}` | GET, DELETE |

### 后端升级

| 路径 | 方法 |
| --- | --- |
| `/api/v1/upgrade/check` | GET |
| `/api/v1/upgrade/progress` | GET |
| `/api/v1/upgrade/reset` | POST |
| `/api/v1/upgrade/start` | POST |
| `/api/v1/upgrade/upload` | POST |
| `/api/v1/upgrade/upload/confirm` | POST |
| `/api/v1/upgrade/versions` | GET |

### 资源与转码代理

| 路径 | 方法 |
| --- | --- |
| `/api/v1/proxy` | GET |
| `/api/v1/proxy/transcode` | GET |

### 日志导出

| 路径 | 方法 |
| --- | --- |
| `/api/v1/logs/export` | GET |

### 健康检查

| 路径 | 方法 |
| --- | --- |
| `/api/v1/health` | GET |

### 版本

| 路径 | 方法 |
| --- | --- |
| `/api/v1/version` | GET |
