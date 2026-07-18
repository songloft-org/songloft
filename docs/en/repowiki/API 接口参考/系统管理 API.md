# System Management API

This document is based on the following source files:

- `internal/handlers/health.go` -- Health check handler
- `internal/handlers/version.go` -- Version information handler
- `internal/handlers/upgrade.go` -- System upgrade handler
- `internal/handlers/proxy.go` -- Resource proxy handler
- `internal/app/routers.go` -- Route registration
- `internal/models/models.go` -- Data models such as upgrade progress

## Table of Contents

1. [Overview](#1-overview)
2. [Public Endpoints](#2-public-endpoints)
   - [GET /health -- Health Check](#21-get-health----health-check)
   - [GET /version -- Version Information](#22-get-version----version-information)
3. [Upgrade Management Endpoints](#3-upgrade-management-endpoints)
   - [GET /upgrade/versions -- Get Available Version Information](#31-get-upgradeversions----get-available-version-information)
   - [GET /upgrade/check -- Check for Updates](#32-get-upgradecheck----check-for-updates)
   - [POST /upgrade/start -- Start Upgrade](#33-post-upgradestart----start-upgrade)
   - [POST /upgrade/reset -- Roll Back to the Base Package Version](#34-post-upgradereset----roll-back-to-the-base-package-version)
   - [GET /upgrade/progress -- Get Upgrade Progress](#35-get-upgradeprogress----get-upgrade-progress)
4. [Resource Proxy Endpoint](#4-resource-proxy-endpoint)
   - [GET /proxy -- Proxy External Resources](#41-get-proxy----proxy-external-resources)

---

## 1. Overview

The system management module provides the application's health check, version information, online upgrade, and resource proxy functions. Among these, the health check and version information are public endpoints (no authentication required); the remaining endpoints require JWT authentication.

The upgrade function is available only in a Docker environment; calling upgrade-related endpoints in a non-Docker environment returns 403.

The base path for all endpoints is `/api/v1`.

---

## 2. Public Endpoints

### 2.1 GET /health -- Health Check

**Method:** `GET`
**Path:** `/api/v1/health`
**Authentication:** None required

**Description:** Checks whether the application is running normally.

**Success response (200):**

```json
{
  "status": "ok"
}
```

---

### 2.2 GET /version -- Version Information

**Method:** `GET`
**Path:** `/api/v1/version`
**Authentication:** None required

**Description:** Gets the application's version information, including the version number, Git commit hash, and build time.

**Success response (200):**

```json
{
  "version": "1.2.0",
  "full": "v1.2.0-abc1234",
  "git_commit": "abc1234",
  "build_time": "2026-06-12_10:00:00"
}
```

| Field | Type | Description |
|------|------|------|
| `version` | string | Version number (e.g., `1.2.0` or `dev`) |
| `full` | string | Full version string (including commit hash) |
| `git_commit` | string | Git commit hash |
| `build_time` | string | Build time |

---

## 3. Upgrade Management Endpoints

All upgrade endpoints require JWT authentication and are available only in a Docker environment. A non-Docker environment returns `403 Forbidden`.

### 3.1 GET /upgrade/versions -- Get Available Version Information

**Method:** `GET`
**Path:** `/api/v1/upgrade/versions`
**Authentication:** BearerAuth required

**Description:** Gets the remote version information for the stable release (stable) and the test build (dev).

**Query parameters:**

| Parameter | Type | Required | Description |
|------|------|------|------|
| `github_proxy` | string | No | GitHub proxy prefix |

**Success response (200):**

```json
{
  "current": {
    "version": "1.2.0",
    "git_commit": "abc1234",
    "build_time": "2026-06-12_10:00:00",
    "channel": "stable",
    "build_type": "full"
  },
  "stable": {
    "version": "1.3.0",
    "git_commit": "def5678",
    "build_time": "2026-06-15_10:00:00",
    "download_url_prefix": "https://github.com/.../releases/download/v1.3.0/songloft",
    "release_notes": "正式版更新说明"
  },
  "dev": {
    "version": "dev-20260615",
    "git_commit": "ghi9012",
    "build_time": "2026-06-15_12:00:00",
    "download_url_prefix": "https://github.com/.../releases/download/dev/songloft",
    "release_notes": "测试版更新说明"
  }
}
```

When a channel fails to be fetched, its corresponding field returns `{"error": "错误信息"}`.

**Error responses:**

| Status | Description |
|--------|------|
| 403 | Upgrade is not supported in a non-Docker environment |
| 500 | Failed to get version information |

---

### 3.2 GET /upgrade/check -- Check for Updates

**Method:** `GET`
**Path:** `/api/v1/upgrade/check`
**Authentication:** BearerAuth required

**Description:** Checks whether a new version is available on the current channel. dev only checks dev, and release only checks stable; dev is judged by `build_time`, and release is judged by version number. Provides both nested and flat fields for easy frontend parsing.

**Query parameters:**

| Parameter | Type | Required | Description |
|------|------|------|------|
| `github_proxy` | string | No | GitHub proxy prefix |

**Success response (200):**

```json
{
  "is_docker": true,
  "has_update": true,
  "current_version": "1.2.0",
  "current_channel": "stable",
  "current_build_type": "full",
  "latest_version": "1.3.0",
  "release_notes": "修复了若干问题...",
  "current": {
    "version": "1.2.0",
    "git_commit": "abc1234",
    "build_time": "2026-06-12_10:00:00",
    "channel": "stable",
    "build_type": "full"
  },
  "updates": {
    "stable": {
      "version": "1.3.0",
      "release_notes": "...",
      ...
    }
  }
}
```

| Field | Type | Description |
|------|------|------|
| `is_docker` | boolean | Whether it is a Docker environment |
| `has_update` | boolean | Whether an update is available |
| `current_version` | string | Current version number |
| `current_channel` | string | Current channel: `stable` or `dev` |
| `current_build_type` | string | Current build type: `full` or `lite` |
| `latest_version` | string | The latest version number within the current channel |
| `release_notes` | string | Release notes of the latest version |
| `current` | object | Detailed information about the current version |
| `updates` | object | Available update details (contains only the channel currently allowed to upgrade) |

**Error responses:**

| Status | Description |
|--------|------|
| 500 | Failed to check for updates |

---

### 3.3 POST /upgrade/start -- Start Upgrade

**Method:** `POST`
**Path:** `/api/v1/upgrade/start`
**Authentication:** BearerAuth required

**Description:** Starts upgrading to the specified version. The upgrade executes asynchronously in the background; progress can be polled via `/upgrade/progress`.

**Request body:**

```json
{
  "version_type": "stable",
  "github_proxy": ""
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `version_type` | string | Yes | Version type: `stable` or `dev`, must match the currently running channel |
| `github_proxy` | string | No | GitHub proxy prefix |

**Success response (200):**

```json
{
  "message": "升级已开始，请稍候..."
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 400 | Invalid request parameters or version type |
| 403 | Upgrade is not supported in a non-Docker environment |
| 500 | Upgrade failed |

---

### 3.4 POST /upgrade/reset -- Roll Back to the Base Package Version

**Method:** `POST`
**Path:** `/api/v1/upgrade/reset`
**Authentication:** BearerAuth required

**Description:** Rolls back the binary to the original version in the Docker image (the base package), then restarts the service. Executes asynchronously in the background.

**Success response (200):**

```json
{
  "message": "回退已开始，服务即将重启..."
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 403 | Rollback is not supported in a non-Docker environment |

---

### 3.5 GET /upgrade/progress -- Get Upgrade Progress

**Method:** `GET`
**Path:** `/api/v1/upgrade/progress`
**Authentication:** BearerAuth required

**Description:** Gets progress information for the current upgrade task.

**Success response (200):**

```json
{
  "status": "downloading",
  "progress": 50,
  "current_step": "正在下载新版本...",
  "error": ""
}
```

| Field | Type | Description |
|------|------|------|
| `status` | string | Upgrade status: `idle` / `downloading` / `testing` / `replacing` / `resetting` / `restarting` / `failed` |
| `progress` | int | Progress percentage (0-100) |
| `current_step` | string | Description of the current step |
| `error` | string | Error message (if any) |

**Error responses:**

| Status | Description |
|--------|------|
| 403 | Upgrade is not supported in a non-Docker environment |

---

## 4. Resource Proxy Endpoint

### 4.1 GET /proxy -- Proxy External Resources

**Method:** `GET`
**Path:** `/api/v1/proxy`
**Authentication:** BearerAuth required

**Description:** Proxies external resources (images, audio, video streams, etc.), resolving browser CORS restrictions. Supports streaming forwarding, Range request passthrough, Content-Type passthrough, and domain whitelist validation.

**Query parameters:**

| Parameter | Type | Required | Description |
|------|------|------|------|
| `url` | string | Yes | The URL of the target resource (URL-encoded) |

**Success responses:**

| Status | Description |
|--------|------|
| 200 | Proxied resource content (streaming forwarding) |
| 206 | Partial content (Range request) |

**Passed-through response headers:**
- `Content-Type`
- `Content-Length`
- `Content-Range`
- `Accept-Ranges`
- `Cache-Control`
- `ETag`
- `Last-Modified`

Automatically sets a 7-day cache for image resources (`max-age=604800`).

**Error responses:**

| Status | Description |
|--------|------|
| 400 | Missing url parameter / invalid URL / only http/https protocols are supported |
| 403 | Domain is not in the whitelist |
| 502 | Upstream request failed |

**Security mechanisms:**

- Only the HTTP/HTTPS protocols are allowed
- Domain whitelist validation (`services.IsHostnameAllowed`); domains that do not pass are logged with a warning
- Does not automatically follow more than 10 redirects
- Sets a generic User-Agent to avoid being rejected by upstream CDNs
- Supports Basic Auth in the upstream URL (automatically extracted and set, cleaning credentials from the URL)

---

**Section source:** `internal/handlers/health.go` / `internal/handlers/version.go` / `internal/handlers/upgrade.go` / `internal/handlers/proxy.go` / `internal/app/routers.go` / `internal/models/models.go`
