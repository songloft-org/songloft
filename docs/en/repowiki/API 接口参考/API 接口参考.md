# API Reference

This document is based on the following source files:

- `internal/app/routers.go` -- Complete route registration (API v1 route group, middleware mounting, JS plugin routes)
- `internal/handlers/response.go` -- respondJSON / respondError response helper functions
- `internal/middleware/auth.go` -- JWT authentication middleware (Bearer Token + access_token query parameter fallback)
- `docs/api_response.md` -- API response format specification

## Table of Contents

1. [Basic Information](#1-basic-information)
   - [API Version and Base URL](#11-api-version-and-base-url)
   - [Authentication](#12-authentication)
   - [Response Format](#13-response-format)
   - [Content-Type Conventions](#14-content-type-conventions)
   - [Pagination](#15-pagination)
2. [Public and Authenticated Endpoints](#2-public-and-authenticated-endpoints)
3. [API Group Index](#3-api-group-index)
   - [Song Management](#31-song-management)
   - [Playlist Management](#32-playlist-management)
   - [Authentication](#33-authentication)
   - [Configuration and Settings](#34-configuration-and-settings)
   - [Scan Management](#35-scan-management)
   - [Cache Management](#36-cache-management)
   - [Plugin Management](#37-plugin-management)
   - [System Management](#38-system-management)

---

## 1. Basic Information

### 1.1 API Version and Base URL

**Section source**: `internal/app/routers.go`

All business endpoints are registered uniformly under the `/api/v1` route group. Full URL format:

```
{scheme}://{host}:{port}{base_path}/api/v1/{endpoint}
```

| Component | Default | Description |
|----------|--------|------|
| `scheme` | `http` | Can be `https` behind a reverse proxy |
| `host:port` | `localhost:58091` | Configured at startup via the `-addr` parameter |
| `base_path` | empty | Configured for sub-path deployment via `-base-path /xxx` or the environment variable `BASE_PATH=/xxx`; the backend strips the prefix with `http.StripPrefix` |

Example: with the default startup, the song list endpoint is `http://localhost:58091/api/v1/songs`; if `BASE_PATH=/music` is configured, it becomes `http://localhost:58091/music/api/v1/songs`.

The current API version is **v1**, and all endpoints are prefixed with `/api/v1/`. In development mode, the interactive API documentation is available at `/swagger/index.html`.

### 1.2 Authentication

**Section source**: `internal/middleware/auth.go`

Songloft uses a **JWT dual-token mechanism** (Access Token + Refresh Token). The authentication middleware obtains the token in the following order of priority:

1. **Authorization request header** (preferred): `Authorization: Bearer <access_token>`
2. **URL query parameter** (fallback): `?access_token=<token>`, used for scenarios that cannot set a custom header such as `<img>` tags and audio players

On authentication failure, a JSON-formatted error is returned (rather than plain text), keeping it consistent with business endpoints:

```json
{"error": "缺少认证信息"}
{"error": "无效的 token", "detail": "token has expired"}
```

> **Special handling**: Xiaomi (Xiao AI) speaker firmware replaces `&` in URLs with spaces; the middleware automatically splits and restores the swallowed parameters.

### 1.3 Response Format

**Section source**: `internal/handlers/response.go`, `docs/api_response.md`

The project adopts a **RESTful direct-return style** and does not use a unified `{code, data, message}` envelope.

#### Success Response

| Scenario | Format | Example |
|------|------|------|
| Single entity | Returns the model object directly | `{"id":1, "title":"Sample Track", ...}` |
| List (with pagination) | Collection name + pagination metadata | `{"songs":[...], "total":100, "limit":20, "offset":0}` |
| Operation result | `{"message": "..."}` | `{"message": "歌曲已删除"}` |

#### Error Response

Errors for all API endpoints are returned uniformly through `respondError` in a fixed format:

```json
{"error": "人类可读的错误信息", "detail": "可选的技术细节"}
```

- `error` (required): a short, user-facing description
- `detail` (optional): the underlying error message, output only when the internal `err != nil`

Middleware-level errors (such as authentication failures) are also returned in JSON format, using the same `{error, detail}` field structure.

#### Exception: Binary Stream Endpoints

Binary stream endpoints such as playback (`/songs/{id}/play`), resource proxy (`/proxy`), and cover images (`/songs/{id}/cover`) may return plain text errors (`http.Error`), because the client does not expect a JSON body.

### 1.4 Content-Type Conventions

**Section source**: `internal/handlers/response.go`, `internal/app/routers.go`

The default is `application/json`. Exceptions: audio playback (`audio/*`, supports Range), cover images (`image/*`), playlist export and plugin files (`application/octet-stream`), plugin pages (`text/html`), and HLS playlists (`application/vnd.apple.mpegurl`). Upload endpoints (plugin installation, playlist covers) use `multipart/form-data`.

### 1.5 Pagination

**Section source**: `internal/handlers/music.go`

List endpoints that support pagination use the `limit` (default 20, automatically truncated when exceeding the upper limit) and `offset` (default 0) query parameters. The response body contains four fields: the collection name, `total`, `limit`, and `offset`.

---

## 2. Public and Authenticated Endpoints

**Section source**: `internal/app/routers.go`

Route registration is divided into two layers: public endpoints (no authentication required) and authorized endpoints (JWT required).

**Public endpoints** (no Bearer Token required):

| Endpoint | Description |
|------|------|
| `POST /api/v1/auth/login` | User login, obtains a token pair |
| `POST /api/v1/auth/refresh` | Exchanges a refresh token for a new access token |
| `GET /api/v1/version` | Gets service version information |
| `GET /api/v1/health` | Health check |
| `GET /api/v1/jsplugin/{entryPath}` | Plugin static page (HTML) |
| `GET /api/v1/jsplugin/{entryPath}/static/*` | Plugin static resources (CSS/JS/images) |
| `GET /api/v1/jsplugin-assets/*` | Plugin common resources (common.css/common.js/fonts) |
| Paths declared in a plugin's `publicPaths` | API paths declared in a plugin manifest that require no authentication |

**Authenticated endpoints** (Bearer Token required):

All `/api/v1/*` endpoints other than the public endpoints above require authentication. Authentication failure returns `401 Unauthorized`.

---

## 3. API Group Index

**Diagram source**: `internal/app/routers.go`, `internal/handlers/jsplugin.go`, `internal/jsplugin/routes.go`

Below are all API groups listed by business module. The endpoint count marked for each group is the **logical endpoint count** (HEAD aliases and `.m3u8` suffix variants are not counted separately).

### 3.1 Song Management

> See [Song API](歌曲%20API.md) for details

| Prefix | Endpoints | Description |
|------|--------|------|
| `/api/v1/songs` | 19 | Song CRUD, batch operations, audio playback, covers, lyrics, HLS radio proxy |

Covers: CRUD and batch operations (list/filter/add remote songs/radio/clean/batch delete/duplicate detection), metadata writing (lyrics/tag write-back/organization), media endpoints (audio streaming/covers/lyrics, supports Range requests and local/remote/radio dispatch), and HLS radio reverse proxy (m3u8 rewriting + segment forwarding).

### 3.2 Playlist Management

> See [Playlist API](歌单%20API.md) for details

| Prefix | Endpoints | Description |
|------|--------|------|
| `/api/v1/playlists` | 16 | Playlist CRUD, song management, reordering, covers, import/export |

Covers: CRUD and batch operations (list/create/update/delete/batch delete/reorder), playlist song management (add/reorder/remove/last played time), cover upload and retrieval, and data import/export (JSON format, with song deduplication matching).

### 3.3 Authentication

> See [Authentication API](认证%20API.md) for details

| Prefix | Endpoints | Description |
|------|--------|------|
| `/api/v1/auth` | 6 | Login, token refresh, logout, token management |

Covers: public endpoints (login to obtain a token pair, refresh token) and authorized endpoints (logout, list/view/revoke active tokens).

### 3.4 Configuration and Settings

> See [Configuration and Settings API](配置与设置%20API.md) for details

| Prefix | Endpoints | Description |
|------|--------|------|
| `/api/v1/configs` | 5 | Generic KV configuration (admin editor) |
| `/api/v1/settings/*` | 20 | Business feature settings (10 items, each with GET + PUT) |

Covers: generic KV configuration CRUD (admin editor), 10 business settings (HLS proxy/music path/scan strategy/auto scan/log level/plugin registries/HTTP proxy/tab configuration). Business settings are strongly-typed JSON with built-in defaults, and PUT can trigger side effects.

### 3.5 Scan Management

> See [Scan Management API](扫描管理%20API.md) for details

| Prefix | Endpoints | Description |
|------|--------|------|
| `/api/v1/scan` | 8 | Music directory scanning, progress querying, audio fingerprint computation |

Covers: scan control (trigger/progress/cancel), directory browsing (directory tree/directory name list), and batch audio fingerprint computation (status/start/progress).

### 3.6 Cache Management

> See [Cache Management API](缓存管理%20API.md) for details

| Prefix | Endpoints | Description |
|------|--------|------|
| `/api/v1/cache-manage` | 5 | Remote song cache statistics, cleanup, configuration |

Covers: cache statistics and manual cleanup, reading/updating cache configuration (directory path, LRU limit), and pre-validating a cache directory (writability + disk space).

### 3.7 Plugin Management

> See [Plugin Management API](插件管理%20API.md) for details

| Prefix | Endpoints | Description |
|------|--------|------|
| `/api/v1/jsplugins` | 13 | Plugin lifecycle management (install, update, enable/disable) |
| `/api/v1/jsplugin/{entryPath}` | 8 | Plugin runtime routes (static pages, API forwarding, file access) |
| `/api/v1/jsplugin-assets` | 1 | Plugin common resources (CSS/JS/fonts) |
| `/api/v1/plugins/health` | 1 | Plugin runtime health check |

Covers: lifecycle management (install/update/delete/enable/disable/check for updates/batch update), plugin registry refresh and installation, runtime routes (static page SPA fallback + QuickJS sandbox API forwarding + direct file access), and common resources (theme CSS/JS/fonts).

### 3.8 System Management

> See [System Management API](系统管理%20API.md) for details

| Prefix | Endpoints | Description |
|------|--------|------|
| `/api/v1/version` | 1 | Version information (public) |
| `/api/v1/health` | 1 | Health check (public) |
| `/api/v1/upgrade` | 5 | System upgrade (Docker only) |
| `/api/v1/proxy` | 1 | Resource proxy (resolves CDN CORS) |

Covers: version information and health check (public), online upgrade (version list/check/start/rollback/progress, available only on Docker), and resource proxy (resolves external CDN CORS restrictions).

---

> **Full API documentation**: After starting in development mode, visit `http://localhost:58091/swagger/index.html` to view the interactive Swagger UI generated by swaggo from source code annotations.
