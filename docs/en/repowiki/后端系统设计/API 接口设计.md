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

Global middleware is registered in `setupBaseRouter()`, in execution order: Compress(Gzip) -> RequestLogger(slog) -> Tracely(panic reporting) -> Recoverer(500) -> RequestID -> CORS -> CORP. The authentication middleware `AuthMiddleware` is not global middleware; it is added on demand inside each route group.

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

The main Handlers are located in `internal/handlers/`:

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

Business settings cover scanning, playback, plugins, logging and client preferences. See `internal/app/routers.go` and [the backend endpoint overview](../../architecture_backend.md) for current paths. Responses are typed JSON such as `{enabled: bool}` or `{proxy: string}`; handlers own defaults and side effects.

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

## 8. Complete route inventory

The following inventory comes from the current `docs/swagger.json` and was checked against route registration. HEAD is omitted. Installed plugins determine dynamic paths; OpenAPI placeholders are not fixed business APIs.

### Songs

| Path | Methods |
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

### Playlists

| Path | Methods |
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

### Play history

| Path | Methods |
| --- | --- |
| `/api/v1/play-history` | GET, DELETE |
| `/api/v1/play-history/entry` | DELETE |

### Authentication

| Path | Methods |
| --- | --- |
| `/api/v1/auth/login` | POST |
| `/api/v1/auth/logout` | POST |
| `/api/v1/auth/refresh` | POST |
| `/api/v1/auth/tokens` | GET |
| `/api/v1/auth/tokens/{token_id}` | DELETE |

### Generic configuration

| Path | Methods |
| --- | --- |
| `/api/v1/configs` | GET, POST |
| `/api/v1/configs/{key}` | GET, PUT, DELETE |

### Business settings

| Path | Methods |
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

### Scanning and fingerprints

| Path | Methods |
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

### Cache

| Path | Methods |
| --- | --- |
| `/api/v1/cache-manage/clean` | POST |
| `/api/v1/cache-manage/config` | GET, PUT |
| `/api/v1/cache-manage/stats` | GET |
| `/api/v1/cache-manage/validate-dir` | POST |

### Plugin management

| Path | Methods |
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

### Plugin runtime

| Path | Methods |
| --- | --- |
| `/api/v1/jsplugin/{entryPath}` | GET |
| `/api/v1/jsplugin/{entryPath}/files/{path}` | GET, HEAD |
| `/api/v1/jsplugin/{entryPath}/static` | GET |
| `/api/v1/jsplugin/{entryPath}/static/{path}` | GET |
| `/api/v1/jsplugin/{entryPath}/{path}` | GET, PUT, POST, DELETE |

### Shared plugin assets

| Path | Methods |
| --- | --- |
| `/api/v1/jsplugin-assets/web-core/{path}` | GET |
| `/api/v1/jsplugin-assets/{path}` | GET |

### Plugin health

| Path | Methods |
| --- | --- |
| `/api/v1/plugins/health` | GET |

### Custom tags

| Path | Methods |
| --- | --- |
| `/api/v1/song-tags` | GET, POST |
| `/api/v1/song-tags/{id}` | GET, PUT, DELETE |
| `/api/v1/song-tags/{id}/bind` | POST |
| `/api/v1/song-tags/{id}/song-ids` | GET |
| `/api/v1/song-tags/{id}/songs` | GET |
| `/api/v1/song-tags/{id}/unbind` | POST |

### Theme packs

| Path | Methods |
| --- | --- |
| `/api/v1/theme-packs` | GET, POST |
| `/api/v1/theme-packs/active` | GET, PUT, DELETE |
| `/api/v1/theme-packs/catalog/install` | POST |
| `/api/v1/theme-packs/catalog/refresh` | POST |
| `/api/v1/theme-packs/{themeID}` | GET, DELETE |

### Backend upgrades

| Path | Methods |
| --- | --- |
| `/api/v1/upgrade/check` | GET |
| `/api/v1/upgrade/progress` | GET |
| `/api/v1/upgrade/reset` | POST |
| `/api/v1/upgrade/start` | POST |
| `/api/v1/upgrade/upload` | POST |
| `/api/v1/upgrade/upload/confirm` | POST |
| `/api/v1/upgrade/versions` | GET |

### Resource and transcode proxy

| Path | Methods |
| --- | --- |
| `/api/v1/proxy` | GET |
| `/api/v1/proxy/transcode` | GET |

### Log export

| Path | Methods |
| --- | --- |
| `/api/v1/logs/export` | GET |

### Health check

| Path | Methods |
| --- | --- |
| `/api/v1/health` | GET |

### Version

| Path | Methods |
| --- | --- |
| `/api/v1/version` | GET |
