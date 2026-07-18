# API 接口设计

<cite>
本文档基于以下源文件编写：

- `internal/app/routers.go` -- 完整路由注册（Chi v5 路由树）
- `internal/handlers/response.go` -- 响应辅助函数（respondJSON / respondError）
- `internal/handlers/*.go` -- 各业务 Handler 定义与 Swagger 注释
- `internal/middleware/auth.go` -- JWT 认证中间件（Bearer + query param）
- `internal/jsplugin/routes.go` -- JS 插件静态/API 路由注册
- `docs/api_response.md` -- API 响应格式规范
- `AGENTS.md` -- API 文档规范 + 配置接口规范铁律
</cite>

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

全局中间件在 `setupBaseRouter()` 中注册，按执行顺序：Compress(Gzip) -> RequestLogger(slog) -> Tracely(panic 上报) -> Recoverer(500) -> RequestID -> CORS。认证中间件 `AuthMiddleware` 不是全局中间件，而是在各路由组内部按需添加。

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

项目共 14 个 Handler，均位于 `internal/handlers/`：

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

当前 17 对 GET/PUT 端点，分布在 SongHandler（remote-title-source）、HLSHandler（hls-proxy）、ScanHandler（music-path、scan-playlist-mode、scan-auto-create-playlists、scan-title-source、auto-scan 共 5 个扫描相关）、LogHandler（log-level）、JSPluginHandler（plugin-registries、http-proxy、plugin-keep-alive、plugin-auto-update）、UpgradeHandler（github-proxy）、ConfigHandler（tab-config、library-browse、user-preferences、equalizer）。数据均为强类型 JSON，如 `{enabled: bool}`、`{proxy: string}` 等，handler 内部承担默认值和副作用。

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

**图表来源**: `internal/app/routers.go`、`internal/handlers/jsplugin.go`、`internal/jsplugin/routes.go`

以下路由清单涵盖 `routers.go`、`JSPluginHandler.RegisterRoutes`、`jsplugin/routes.go` 中注册的全部端点。除特别标注外，均需 Bearer 认证。

### 8.1 认证 (AuthHandler)

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | `/auth/login` | 无 | 用户登录 |
| POST | `/auth/refresh` | 无 | 刷新 token |
| POST | `/auth/logout` | Bearer | 登出 |
| GET | `/auth/tokens` | Bearer | 列出所有 token |
| GET | `/auth/tokens/{token_id}` | Bearer | token 详情 |
| DELETE | `/auth/tokens/{token_id}` | Bearer | 撤销 token |

### 8.2 歌曲 (SongHandler + HLSHandler)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/songs` | 歌曲列表（分页+过滤） |
| GET | `/songs/ids` | 歌曲 ID 列表 |
| POST | `/songs/remote` | 添加远程歌曲 |
| POST | `/songs/radio` | 添加电台 |
| POST | `/songs/clean` | 清理无效歌曲 |
| POST | `/songs/batch-delete` | 批量删除 |
| POST | `/songs/organize` | 整理歌曲文件 |
| POST | `/songs/organize/preview` | 预览批量整理（dry-run） |
| GET | `/songs/duplicates` | 重复歌曲检测 |
| GET | `/songs/facets` | 标签分类聚合 |
| POST | `/songs/refresh-metadata` | 启动远程元数据刷新 |
| GET | `/songs/refresh-metadata/progress` | 元数据刷新进度 |
| POST | `/songs/refresh-metadata/cancel` | 取消元数据刷新 |
| GET | `/songs/{id}` | 获取歌曲详情 |
| PUT | `/songs/{id}` | 更新歌曲信息 |
| DELETE | `/songs/{id}` | 删除歌曲 |
| PUT | `/songs/{id}/lyrics` | 更新歌词 |
| PUT | `/songs/{id}/tags` | 写入音频 tag |
| POST | `/songs/{id}/activate` | 激活歌曲 |
| POST | `/songs/{id}/played` | 播放事件通知（广播给插件） |
| GET/HEAD | `/songs/{id}/play` | 播放音频流（二进制） |
| GET/HEAD | `/songs/{id}/play.m3u8` | HLS 电台别名（同 handler） |
| GET | `/songs/{id}/cover` | 歌曲封面 |
| GET | `/songs/{id}/lyric` | 歌曲歌词 |
| GET/HEAD | `/songs/{id}/hls/playlist` | HLS 播放列表代理 |
| GET/HEAD | `/songs/{id}/hls/segment` | HLS 切片代理 |

### 8.3 歌单 (PlaylistHandler + BackupHandler)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/playlists/export` | 导出歌单 |
| POST | `/playlists/import` | 导入歌单 |
| GET | `/playlists` | 歌单列表 |
| POST | `/playlists` | 创建歌单 |
| PUT | `/playlists/reorder` | 歌单排序 |
| GET | `/playlists/{id}` | 歌单详情 |
| PUT | `/playlists/{id}` | 更新歌单 |
| DELETE | `/playlists/{id}` | 删除歌单 |
| POST | `/playlists/batch-delete` | 批量删除歌单 |
| GET | `/playlists/{id}/songs` | 歌单内歌曲列表 |
| POST | `/playlists/{id}/songs` | 添加歌曲到歌单 |
| PUT | `/playlists/{id}/songs/reorder` | 歌单歌曲排序 |
| DELETE | `/playlists/{id}/songs/{songId}` | 移除歌单歌曲 |
| POST | `/playlists/{id}/touch` | 更新歌单访问时间 |
| POST | `/playlists/{id}/cover` | 上传歌单封面 |
| GET | `/playlists/{id}/cover` | 获取歌单封面 |

### 8.4 配置与设置

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| GET/PUT | `/settings/remote-title-source` | SongHandler | 网络歌曲标题来源 |
| GET/PUT | `/settings/hls-proxy` | HLSHandler | HLS 代理开关 |
| GET/PUT | `/settings/music-path` | ScanHandler | 音乐库路径 |
| GET/PUT | `/settings/scan-playlist-mode` | ScanHandler | 歌单归并模式 |
| GET/PUT | `/settings/scan-auto-create-playlists` | ScanHandler | 自动创建歌单 |
| GET/PUT | `/settings/scan-title-source` | ScanHandler | 标题来源 |
| GET/PUT | `/settings/auto-scan` | ScanHandler | 自动扫描 |
| GET/PUT | `/settings/log-level` | LogHandler | 日志等级 |
| GET/PUT | `/settings/plugin-registries` | JSPluginHandler | 插件注册表 |
| GET/PUT | `/settings/http-proxy` | JSPluginHandler | HTTP 代理 |
| GET/PUT | `/settings/plugin-keep-alive` | JSPluginHandler | 插件常驻白名单 |
| GET/PUT | `/settings/plugin-auto-update` | JSPluginHandler | 插件自动更新 |
| GET/PUT | `/settings/github-proxy` | UpgradeHandler | GitHub 更新代理 |
| GET/PUT | `/settings/tab-config` | ConfigHandler | Tab 页配置 |
| GET/PUT | `/settings/library-browse` | ConfigHandler | 曲库浏览视图 |
| GET/PUT | `/settings/user-preferences` | ConfigHandler | 用户偏好设置 |
| GET/PUT | `/settings/equalizer` | ConfigHandler | 均衡器 |
| GET | `/configs` | ConfigHandler | 配置列表（通用 KV） |
| POST | `/configs` | ConfigHandler | 创建配置 |
| GET | `/configs/{key}` | ConfigHandler | 获取配置 |
| PUT | `/configs/{key}` | ConfigHandler | 更新配置 |
| DELETE | `/configs/{key}` | ConfigHandler | 删除配置 |

### 8.5 扫描 (ScanHandler)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/scan` | 扫描并导入 |
| GET | `/scan/progress` | 扫描进度 |
| POST | `/scan/cancel` | 取消扫描 |
| GET | `/scan/directories` | 目录列表 |
| GET | `/scan/dir-names` | 目录名列表 |
| GET | `/scan/fingerprints/status` | 指纹状态 |
| POST | `/scan/fingerprints` | 启动指纹计算 |
| GET | `/scan/fingerprints/progress` | 指纹计算进度 |

### 8.6 缓存 (CacheHandler)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/cache-manage/stats` | 缓存统计 |
| POST | `/cache-manage/clean` | 清理缓存 |
| GET | `/cache-manage/config` | 缓存配置 |
| PUT | `/cache-manage/config` | 更新缓存配置 |
| POST | `/cache-manage/validate-dir` | 验证缓存目录 |

### 8.7 升级 (UpgradeHandler)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/upgrade/versions` | 版本列表 |
| GET | `/upgrade/check` | 检查更新（仅 Docker） |
| POST | `/upgrade/start` | 开始升级 |
| POST | `/upgrade/reset` | 重置到底包 |
| GET | `/upgrade/progress` | 升级进度 |

### 8.8 JS 插件管理 (JSPluginHandler)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/jsplugins` | 插件列表 |
| POST | `/jsplugins/upload` | 上传插件 |
| POST | `/jsplugins/update-all` | 批量更新 |
| POST | `/jsplugins/storage/cleanup` | 清理孤儿持久化存储 |
| POST | `/jsplugins/registry/refresh` | 刷新注册表 |
| POST | `/jsplugins/registry/install` | 从注册表安装 |
| GET | `/jsplugins/{id}` | 插件详情 |
| PUT | `/jsplugins/{id}` | 更新插件 |
| DELETE | `/jsplugins/{id}` | 删除插件 |
| POST | `/jsplugins/{id}/enable` | 启用插件 |
| POST | `/jsplugins/{id}/disable` | 禁用插件 |
| GET | `/jsplugins/{id}/check-update` | 检查插件更新 |
| POST | `/jsplugins/{id}/update` | 下载更新 |
| GET | `/plugins/health` | 音源健康度 |

### 8.9 JS 插件运行时 (jsplugin.Manager)

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/jsplugin/{entryPath}[/]` | 无 | 插件入口 HTML |
| GET | `/jsplugin/{entryPath}/static[/*]` | 无 | 插件静态资源 |
| GET | `/jsplugin-assets/*` | 无 | 公共 CSS/JS/字体 |
| GET/HEAD | `/jsplugin/{entryPath}/files/*` | Bearer | 插件文件服务 |
| ANY | `/jsplugin/{entryPath}/*` | Bearer* | API catch-all 转发 |

*注: 插件 manifest 中 `publicPaths` 声明的路径通过 `PublicPathChecker` 豁免认证。

### 8.10 其他

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/version` | 无 | 版本信息 |
| GET | `/health` | 无 | 健康检查 |
| GET | `/proxy` | Bearer | 外部资源 CORS 代理 |

> 以上路径均省略 `/api/v1` 前缀。完整 URL 为 `http://<host>:58091/api/v1/<path>`。
