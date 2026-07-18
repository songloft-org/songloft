# API Interface Design

This document is based on the following source files:

- `internal/app/routers.go` -- complete route registration (Chi v5 routing tree)
- `internal/handlers/response.go` -- response helper functions (respondJSON / respondError)
- `internal/handlers/*.go` -- the various business Handler definitions and Swagger annotations
- `internal/middleware/auth.go` -- JWT authentication middleware (Bearer + query param)
- `internal/jsplugin/routes.go` -- JS plugin static/API route registration
- `docs/api_response.md` -- API response format specification
- `AGENTS.md` -- API documentation conventions + config interface conventions (hard rules)

## Table of Contents

1. [Design Principles](#1-design-principles)
2. [Response Format Specification](#2-response-format-specification)
3. [Route Organization Structure](#3-route-organization-structure)
4. [Authentication Mechanism](#4-authentication-mechanism)
5. [Handler Creation Pattern](#5-handler-creation-pattern)
6. [Comparison of the Three Config Interfaces](#6-comparison-of-the-three-config-interfaces)
7. [Swagger Documentation Specification](#7-swagger-documentation-specification)
8. [Complete Route List](#8-complete-route-list)

---

## 1. Design Principles

**Section sources**: `docs/api_response.md`, `AGENTS.md` (backend coding conventions)

The Songloft backend API follows these core principles:

- **RESTful direct return**: successful responses return the business model or collection directly, without a unified `{code, data, message}` envelope. The HTTP status code carries the semantics; a `code` field duplicating it is redundant.
- **Unified error format**: error responses from all JSON API endpoints are uniformly `{"error": "...", "detail": "..."}`; using alternative field names such as `message`, `msg`, or `reason` is forbidden.
- **Resource-oriented paths**: URL paths use plural nouns to represent resource collections (`/songs`, `/playlists`), and HTTP methods express the operation semantics (GET to read, POST to create, PUT to modify, DELETE to delete).
- **Standardized pagination**: list endpoints paginate via the `limit` + `offset` query parameters, and responses include `total`, `limit`, and `offset` metadata.
- **Binary stream exception**: errors from binary-stream endpoints such as play (`/songs/{id}/play`), proxy (`/proxy`), and static files may use plaintext `http.Error()`, since the client does not expect a JSON body.

---

## 2. Response Format Specification

**Section sources**: `internal/handlers/response.go`, `docs/api_response.md`

### 2.1 Response Helper Functions

All handlers output success responses via `respondJSON(w, status, data)` (data is serialized directly as top-level JSON) and output errors via `respondError(w, status, message, err)` (which automatically builds `{"error","detail"}`). The middleware layer uses a separate `respondAuthError` with a consistent format.

### 2.2 Three Forms of Success Response

| Scenario | Format | Example |
|------|------|------|
| Single entity | Model serialized directly | `{"id":1, "title":"Track", ...}` |
| Paginated list | Collection name + pagination metadata | `{"songs":[...], "total":100, "limit":20, "offset":0}` |
| Operation result | Message string | `{"message": "Song deleted"}` |

### 2.3 Error Response Format

```json
{
  "error": "Human-readable error message",
  "detail": "Optional low-level technical detail"
}
```

- `error` -- always present, a short user-facing description
- `detail` -- optional, output only when `err != nil`, containing the low-level error information

Corresponds to the `models.ErrorResponse` struct; custom field names are forbidden.

---

## 3. Route Organization Structure

**Section sources**: `internal/app/routers.go`, `internal/jsplugin/routes.go`

### 3.1 Routing Tree Overview

Route registration is split into three layers, orchestrated uniformly by `App.setupRouter()`:

```
chi.Router (root)
├── Global middleware: Compress → Logger → Recoverer → RequestID → CORS
├── Frontend static files / Swagger (dev build)
├── /api/v1 ─┬─ [Public] /auth/login, /auth/refresh, /version, /health
│            └─ [Bearer] /auth/*, /songs/*, /playlists/*, /settings/*,
│                        /configs/*, /scan/*, /cache-manage/*, /upgrade/*, /proxy
├── /api/v1/jsplugins/* [Bearer] plugin CRUD + registry + /plugins/health
├── /api/v1/jsplugin/{entryPath}[/static/*] [Public] plugin static assets
├── /api/v1/jsplugin-assets/* [Public] common CSS/JS/fonts
└── /api/v1/jsplugin/{entryPath}/* [Bearer+PublicPathChecker] API forwarding
```

### 3.2 Middleware Stack

Global middleware is registered in `setupBaseRouter()`, in execution order: Compress(Gzip) -> RequestLogger(slog) -> Tracely(panic reporting) -> Recoverer(500) -> RequestID -> CORS. The authentication middleware `AuthMiddleware` is not global middleware; it is added on demand inside each route group.

### 3.3 Route Grouping Strategy

Chi v5's `r.Group()` divides authentication boundaries: under the same `/api/v1` prefix, public endpoints are registered directly, while authenticated endpoints are wrapped by `r.Group` + `r.Use(AuthMiddleware)`. JS plugin routes are registered separately; static assets require no authentication, and API forwarding supports the `PublicPathChecker` interface to exempt plugin-declared public paths.

---

## 4. Authentication Mechanism

**Section sources**: `internal/middleware/auth.go`

### 4.1 JWT Dual-Token Mechanism

The system uses JWT dual-token authentication:
- **Access Token** -- short-lived, used for API request authentication
- **Refresh Token** -- long-lived, used to refresh the access token

### 4.2 Token Passing Methods

The authentication middleware tries two methods, in priority order, to obtain the token:

1. **Authorization Header** (priority): `Authorization: Bearer <token>`
2. **Query Parameter** (fallback): `?access_token=<token>`

The query parameter fallback mainly serves scenarios where a custom header cannot be set: `<img>` tags loading cover art, `<audio>` tags loading audio, `CachedNetworkImage`, etc.

### 4.3 Public Endpoints

The following endpoints require no authentication and are registered directly outside the AuthMiddleware:

| Endpoint | Purpose |
|------|------|
| `POST /api/v1/auth/login` | User login |
| `POST /api/v1/auth/refresh` | Refresh token |
| `GET /api/v1/version` | Version information |
| `GET /api/v1/health` | Health check |
| `GET /api/v1/jsplugin/{entryPath}` | Plugin static page |
| `GET /api/v1/jsplugin/{entryPath}/static/*` | Plugin static assets |
| `GET /api/v1/jsplugin-assets/*` | Plugin common assets |

In addition, API paths declared by a plugin via `publicPaths` in its manifest are exempted by the `PublicPathChecker` interface inside the authentication middleware.

---

## 5. Handler Creation Pattern

**Section sources**: `internal/handlers/*.go`, `internal/app/routers.go`

### 5.1 Factory Function Pattern

Each Handler follows a three-step creation: (1) a struct holding service dependencies; (2) a `NewXxxHandler(...)` factory function that receives services and returns a pointer; (3) an optional `SetXxx(fn)` setter to resolve circular dependencies or deferred binding. Taking `SongHandler` as an example, the factory function receives 6 dependencies such as `SongService`, `CacheService`, and `AsyncReassigner`, and after creation injects the Scanner's path-getter function via `SetGetMusicPath`.

### 5.2 Handler List

The project has 14 Handlers in total, all located in `internal/handlers/`:

- **Business core**: `AuthHandler`(AuthService), `SongHandler`(SongService + CacheService + 4 dependencies), `PlaylistHandler`(PlaylistService), `BackupHandler`(BackupService)
- **Config management**: `ConfigHandler`(ConfigService), `ScanHandler`(SongService + Scanner + ConfigService), `HLSHandler`(SongService + ConfigService), `CacheHandler`(CacheService + ConfigService), `LogHandler`(ConfigService + LevelVar)
- **Plugin/upgrade**: `JSPluginHandler`(PackageManager + Repository + Manager + SourceMetrics + ConfigService), `UpgradeHandler`(UpgradeService)
- **Utility**: `ProxyHandler`(no dependencies), `VersionHandler`(no dependencies), `HealthHandler`(no dependencies)

### 5.3 Callback Injection

Some handlers bind cross-module callbacks via setters: `configHandler.SetOnConfigChanged` triggers side effects after a generic KV write, `scanHandler.SetOnMusicPathChanged` / `SetOnAutoScanChanged` bind rebuild logic after config changes, and `songHandler.SetGetMusicPath` lazily injects the Scanner's path function.

---

## 6. Comparison of the Three Config Interfaces

**Section sources**: `AGENTS.md` (config interface conventions, hard rules), `internal/app/routers.go`

Three config interface styles exist in the project, each with a clear division of labor:

| Dimension | `/settings/<name>` | `/<module>/config` | `/configs/{key}` |
|------|--------------------|--------------------|------------------|
| Positioning | Business feature toggle (user-facing) | Module aggregate config | admin generic KV editing |
| Path style | `/settings/<kebab-case>` | `/<module>/config` | `/configs/{key}` |
| Data form | Strongly typed JSON | Strongly typed JSON | `{key, value}` string |
| Default value | Handled internally by the handler | Handled internally by the handler | None (returns 404 if the key does not exist) |
| Side effects | Triggered directly inside PUT | Triggered directly inside PUT | Requires an `onConfigChanged` callback |
| Ownership | Corresponding business module handler | Module handler | ConfigHandler |
| Use case | Isolated config or cross-module sharing | Strongly related to module action endpoints | admin debugging / hand-editing |

### 6.1 Business Endpoints `/settings/<name>`

Currently 17 GET/PUT endpoint pairs, distributed across SongHandler (remote-title-source), HLSHandler (hls-proxy), ScanHandler (music-path, scan-playlist-mode, scan-auto-create-playlists, scan-title-source, auto-scan -- 5 scan-related), LogHandler (log-level), JSPluginHandler (plugin-registries, http-proxy, plugin-keep-alive, plugin-auto-update), UpgradeHandler (github-proxy), and ConfigHandler (tab-config, library-browse, user-preferences, equalizer). The data is all strongly typed JSON, such as `{enabled: bool}`, `{proxy: string}`, etc., with the handler handling defaults and side effects internally.

### 6.2 Module Aggregate Endpoints

A typical example is cache management `/cache-manage/*`, where `config` (GET/PUT) shares the prefix and `CacheService` with `stats`, `clean`, and `validate-dir`. Applicable to scenarios where config is strongly related to module action endpoints.

### 6.3 Generic KV `/configs/{key}`

Used only by the frontend generic config editor (admin hand-editing); no strong typing, no side effects (unless an `onConfigChanged` callback is attached), and PUT returns 404 when the key does not exist. New business features are forbidden from calling it directly.

### 6.4 Dual-Entry Consistency

Some config is modified by both a business endpoint and generic KV (such as `music_path`). The `musicPathChanged` closure in `routers.go` ensures both entries share the same side-effect function -- the business endpoint triggers it directly inside the PUT handler, and generic KV triggers it via the `onConfigChanged` callback.

---

## 7. Swagger Documentation Specification

**Section sources**: `AGENTS.md` (API documentation conventions, hard rules)

### 7.1 Hard Rules

Every handler method registered in `routers.go` must have swag annotations. No exemptions.

### 7.2 Required Fields

Each handler contains at least the following 7 swag annotations:

| Field | Description |
|------|------|
| `@Summary` | A one-line summary in Chinese |
| `@Description` | Detailed description (side effects/defaults/error-code triggers) |
| `@Tags` | Business grouping (in Chinese), reusing existing tags |
| `@Produce` | Response format (usually `json`) |
| `@Success` | Success response type and description |
| `@Security` | `BearerAuth` (omitted for public endpoints) |
| `@Router` | Path and method |

Endpoints with a request body additionally add `@Accept json` and `@Param request body`.

### 7.3 Existing Business Tags

```
歌曲管理 | 歌单管理 | 电台与 HLS | 扫描管理 | 配置管理
缓存管理 | JS插件管理 | JS 插件 | 数据备份 | 设置
升级 | 认证
```

Creating new tags on a whim is forbidden.

### 7.4 Multi-Alias Routes and Validation

- Multiple alias paths (such as `/songs/{id}/play` and `/songs/{id}/play.m3u8`) each get their own `@Router` line; HEAD is not listed separately
- Catch-all routes list all actual methods; dynamic routes note their placeholder nature in `@Description`
- After modifying annotations you must run `make swagger` to regenerate; the artifacts (`docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go`) must be committed
- Validation: output contains the new `@Router` path + `grep` hits in swagger.json + visual inspection at `/swagger/index.html` after startup

---

## 8. Complete Route List

**Diagram sources**: `internal/app/routers.go`, `internal/handlers/jsplugin.go`, `internal/jsplugin/routes.go`

The following route list covers all endpoints registered in `routers.go`, `JSPluginHandler.RegisterRoutes`, and `jsplugin/routes.go`. Unless otherwise noted, all require Bearer authentication.

### 8.1 Authentication (AuthHandler)

| Method | Path | Auth | Description |
|------|------|------|------|
| POST | `/auth/login` | None | User login |
| POST | `/auth/refresh` | None | Refresh token |
| POST | `/auth/logout` | Bearer | Logout |
| GET | `/auth/tokens` | Bearer | List all tokens |
| GET | `/auth/tokens/{token_id}` | Bearer | Token details |
| DELETE | `/auth/tokens/{token_id}` | Bearer | Revoke token |

### 8.2 Songs (SongHandler + HLSHandler)

| Method | Path | Description |
|------|------|------|
| GET | `/songs` | Song list (pagination + filtering) |
| GET | `/songs/ids` | Song ID list |
| POST | `/songs/remote` | Add remote song |
| POST | `/songs/radio` | Add radio |
| POST | `/songs/clean` | Clean up invalid songs |
| POST | `/songs/batch-delete` | Batch delete |
| POST | `/songs/organize` | Organize song files |
| POST | `/songs/organize/preview` | Preview batch organize (dry-run) |
| GET | `/songs/duplicates` | Duplicate song detection |
| GET | `/songs/facets` | Tag category aggregation |
| POST | `/songs/refresh-metadata` | Start remote metadata refresh |
| GET | `/songs/refresh-metadata/progress` | Metadata refresh progress |
| POST | `/songs/refresh-metadata/cancel` | Cancel metadata refresh |
| GET | `/songs/{id}` | Get song details |
| PUT | `/songs/{id}` | Update song information |
| DELETE | `/songs/{id}` | Delete song |
| PUT | `/songs/{id}/lyrics` | Update lyrics |
| PUT | `/songs/{id}/tags` | Write audio tags |
| POST | `/songs/{id}/activate` | Activate song |
| POST | `/songs/{id}/played` | Play event notification (broadcast to plugins) |
| GET/HEAD | `/songs/{id}/play` | Play audio stream (binary) |
| GET/HEAD | `/songs/{id}/play.m3u8` | HLS radio alias (same handler) |
| GET | `/songs/{id}/cover` | Song cover art |
| GET | `/songs/{id}/lyric` | Song lyrics |
| GET/HEAD | `/songs/{id}/hls/playlist` | HLS playlist proxy |
| GET/HEAD | `/songs/{id}/hls/segment` | HLS segment proxy |

### 8.3 Playlists (PlaylistHandler + BackupHandler)

| Method | Path | Description |
|------|------|------|
| GET | `/playlists/export` | Export playlists |
| POST | `/playlists/import` | Import playlists |
| GET | `/playlists` | Playlist list |
| POST | `/playlists` | Create playlist |
| PUT | `/playlists/reorder` | Reorder playlists |
| GET | `/playlists/{id}` | Playlist details |
| PUT | `/playlists/{id}` | Update playlist |
| DELETE | `/playlists/{id}` | Delete playlist |
| POST | `/playlists/batch-delete` | Batch delete playlists |
| GET | `/playlists/{id}/songs` | Songs in a playlist |
| POST | `/playlists/{id}/songs` | Add song to playlist |
| PUT | `/playlists/{id}/songs/reorder` | Reorder songs in a playlist |
| DELETE | `/playlists/{id}/songs/{songId}` | Remove a song from a playlist |
| POST | `/playlists/{id}/touch` | Update playlist access time |
| POST | `/playlists/{id}/cover` | Upload playlist cover art |
| GET | `/playlists/{id}/cover` | Get playlist cover art |

### 8.4 Config and Settings

| Method | Path | Handler | Description |
|------|------|---------|------|
| GET/PUT | `/settings/remote-title-source` | SongHandler | Network song title source |
| GET/PUT | `/settings/hls-proxy` | HLSHandler | HLS proxy toggle |
| GET/PUT | `/settings/music-path` | ScanHandler | Music library path |
| GET/PUT | `/settings/scan-playlist-mode` | ScanHandler | Playlist merge mode |
| GET/PUT | `/settings/scan-auto-create-playlists` | ScanHandler | Auto-create playlists |
| GET/PUT | `/settings/scan-title-source` | ScanHandler | Title source |
| GET/PUT | `/settings/auto-scan` | ScanHandler | Auto scan |
| GET/PUT | `/settings/log-level` | LogHandler | Log level |
| GET/PUT | `/settings/plugin-registries` | JSPluginHandler | Plugin registries |
| GET/PUT | `/settings/http-proxy` | JSPluginHandler | HTTP proxy |
| GET/PUT | `/settings/plugin-keep-alive` | JSPluginHandler | Plugin keep-alive allowlist |
| GET/PUT | `/settings/plugin-auto-update` | JSPluginHandler | Plugin auto-update |
| GET/PUT | `/settings/github-proxy` | UpgradeHandler | GitHub update proxy |
| GET/PUT | `/settings/tab-config` | ConfigHandler | Tab page config |
| GET/PUT | `/settings/library-browse` | ConfigHandler | Library browse view |
| GET/PUT | `/settings/user-preferences` | ConfigHandler | User preferences |
| GET/PUT | `/settings/equalizer` | ConfigHandler | Equalizer |
| GET | `/configs` | ConfigHandler | Config list (generic KV) |
| POST | `/configs` | ConfigHandler | Create config |
| GET | `/configs/{key}` | ConfigHandler | Get config |
| PUT | `/configs/{key}` | ConfigHandler | Update config |
| DELETE | `/configs/{key}` | ConfigHandler | Delete config |

### 8.5 Scan (ScanHandler)

| Method | Path | Description |
|------|------|------|
| POST | `/scan` | Scan and import |
| GET | `/scan/progress` | Scan progress |
| POST | `/scan/cancel` | Cancel scan |
| GET | `/scan/directories` | Directory list |
| GET | `/scan/dir-names` | Directory name list |
| GET | `/scan/fingerprints/status` | Fingerprint status |
| POST | `/scan/fingerprints` | Start fingerprint computation |
| GET | `/scan/fingerprints/progress` | Fingerprint computation progress |

### 8.6 Cache (CacheHandler)

| Method | Path | Description |
|------|------|------|
| GET | `/cache-manage/stats` | Cache statistics |
| POST | `/cache-manage/clean` | Clean cache |
| GET | `/cache-manage/config` | Cache config |
| PUT | `/cache-manage/config` | Update cache config |
| POST | `/cache-manage/validate-dir` | Validate cache directory |

### 8.7 Upgrade (UpgradeHandler)

| Method | Path | Description |
|------|------|------|
| GET | `/upgrade/versions` | Version list |
| GET | `/upgrade/check` | Check for updates (Docker only) |
| POST | `/upgrade/start` | Start upgrade |
| POST | `/upgrade/reset` | Reset to base image |
| GET | `/upgrade/progress` | Upgrade progress |

### 8.8 JS Plugin Management (JSPluginHandler)

| Method | Path | Description |
|------|------|------|
| GET | `/jsplugins` | Plugin list |
| POST | `/jsplugins/upload` | Upload plugin |
| POST | `/jsplugins/update-all` | Batch update |
| POST | `/jsplugins/storage/cleanup` | Clean up orphaned persistent storage |
| POST | `/jsplugins/registry/refresh` | Refresh registry |
| POST | `/jsplugins/registry/install` | Install from registry |
| GET | `/jsplugins/{id}` | Plugin details |
| PUT | `/jsplugins/{id}` | Update plugin |
| DELETE | `/jsplugins/{id}` | Delete plugin |
| POST | `/jsplugins/{id}/enable` | Enable plugin |
| POST | `/jsplugins/{id}/disable` | Disable plugin |
| GET | `/jsplugins/{id}/check-update` | Check for plugin update |
| POST | `/jsplugins/{id}/update` | Download update |
| GET | `/plugins/health` | Audio source health |

### 8.9 JS Plugin Runtime (jsplugin.Manager)

| Method | Path | Auth | Description |
|------|------|------|------|
| GET | `/jsplugin/{entryPath}[/]` | None | Plugin entry HTML |
| GET | `/jsplugin/{entryPath}/static[/*]` | None | Plugin static assets |
| GET | `/jsplugin-assets/*` | None | Common CSS/JS/fonts |
| GET/HEAD | `/jsplugin/{entryPath}/files/*` | Bearer | Plugin file serving |
| ANY | `/jsplugin/{entryPath}/*` | Bearer* | API catch-all forwarding |

*Note: paths declared in the plugin manifest's `publicPaths` are exempted from authentication via `PublicPathChecker`.

### 8.10 Other

| Method | Path | Auth | Description |
|------|------|------|------|
| GET | `/version` | None | Version information |
| GET | `/health` | None | Health check |
| GET | `/proxy` | Bearer | External resource CORS proxy |

> All paths above omit the `/api/v1` prefix. The full URL is `http://<host>:58091/api/v1/<path>`.
