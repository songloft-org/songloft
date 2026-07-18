# Plugin Management API

This document is based on the following source files:

- `internal/handlers/jsplugin.go` -- JS plugin management handler (CRUD, enable/disable, update)
- `internal/handlers/jsplugin_registry.go` -- Plugin registry sources and registry installation
- `internal/jsplugin/routes.go` -- JS plugin runtime routes (static resources, API forwarding, file access)
- `internal/app/routers.go` -- Route registration

## Table of Contents

1. [Overview](#1-overview)
2. [Plugin Management Endpoints](#2-plugin-management-endpoints)
   - [GET /jsplugins -- List All Plugins](#21-get-jsplugins----list-all-plugins)
   - [POST /jsplugins/upload -- Upload and Install Plugin](#22-post-jspluginsupload----upload-and-install-plugin)
   - [GET /jsplugins/{id} -- Get Plugin Details](#23-get-jspluginsid----get-plugin-details)
   - [PUT /jsplugins/{id} -- Update Plugin](#24-put-jspluginsid----update-plugin)
   - [DELETE /jsplugins/{id} -- Delete Plugin](#25-delete-jspluginsid----delete-plugin)
   - [POST /jsplugins/{id}/enable -- Enable Plugin](#26-post-jspluginsidenable----enable-plugin)
   - [POST /jsplugins/{id}/disable -- Disable Plugin](#27-post-jspluginsiddisable----disable-plugin)
3. [Plugin Update Endpoints](#3-plugin-update-endpoints)
   - [GET /jsplugins/{id}/check-update -- Check a Single Plugin for Updates](#31-get-jspluginsidcheck-update----check-a-single-plugin-for-updates)
   - [POST /jsplugins/{id}/update -- Download and Update a Single Plugin](#32-post-jspluginsidupdate----download-and-update-a-single-plugin)
   - [POST /jsplugins/update-all -- Batch Update All Plugins](#33-post-jspluginsupdate-all----batch-update-all-plugins)
4. [Registry Endpoints](#4-registry-endpoints)
   - [POST /jsplugins/registry/refresh -- Refresh Registry](#41-post-jspluginsregistryrefresh----refresh-registry)
   - [POST /jsplugins/registry/install -- Install Plugin from Registry](#42-post-jspluginsregistryinstall----install-plugin-from-registry)
5. [Source Health](#5-source-health)
   - [GET /plugins/health -- Plugin Health](#51-get-pluginshealth----plugin-health)
6. [Plugin Settings Endpoints](#6-plugin-settings-endpoints)
   - [GET/PUT /settings/plugin-registries -- Registry List Configuration](#61-getput-settingsplugin-registries----registry-list-configuration)
   - [GET/PUT /settings/http-proxy -- HTTP Proxy Configuration](#62-getput-settingshttp-proxy----http-proxy-configuration)
7. [Plugin Runtime Routes](#7-plugin-runtime-routes)
   - [GET /jsplugin/{entryPath} -- Plugin Root Page](#71-get-jspluginentrypath----plugin-root-page)
   - [GET /jsplugin/{entryPath}/static/* -- Plugin Static Resources](#72-get-jspluginentrypathstatic----plugin-static-resources)
   - [ANY /jsplugin/{entryPath}/* -- Plugin API Forwarding](#73-any-jspluginentrypath----plugin-api-forwarding)
   - [GET/HEAD /jsplugin/{entryPath}/files/* -- Plugin File Access](#74-gethead-jspluginentrypathfiles----plugin-file-access)
   - [GET /jsplugin-assets/* -- Plugin Common Resources](#75-get-jsplugin-assets----plugin-common-resources)

---

## 1. Overview

The JS plugin system is Songloft's extension mechanism, running on a QuickJS sandbox. The plugin management API is divided into three layers:

- **Management endpoints** (`/api/v1/jsplugins/*`): plugin installation, uninstallation, enable/disable, and update; require JWT authentication
- **Settings endpoints** (`/api/v1/settings/*`): global configuration such as registry sources and proxy; require JWT authentication
- **Runtime routes** (`/api/v1/jsplugin/{entryPath}/*`): a plugin's static resource serving and API forwarding; authentication rules depend on whether `publicPaths` is declared

Management endpoints are registered by `JSPluginHandler.RegisterRoutes`, and runtime routes are registered by `Manager.RegisterStaticRoutes` / `RegisterAPIRoutes`.

---

## 2. Plugin Management Endpoints

### 2.1 GET /jsplugins -- List All Plugins

**Method:** `GET`
**Path:** `/api/v1/jsplugins`
**Authentication:** BearerAuth required

**Description:** Gets the list of all installed JS plugins.

**Success response (200):**

```json
{
  "plugins": [
    {
      "id": 1,
      "name": "示例插件",
      "entry_path": "example-plugin",
      "version": "1.0.0",
      "status": "active",
      "icon": "icon.png",
      ...
    }
  ]
}
```

---

### 2.2 POST /jsplugins/upload -- Upload and Install Plugin

**Method:** `POST`
**Path:** `/api/v1/jsplugins/upload`
**Authentication:** BearerAuth required

**Description:** Uploads a `.jsplugin.zip` archive to install a new plugin. If the `entry_path` already exists, it automatically follows the overwrite-update path and hot-reloads the plugin if it is active. The upload size limit is 50MB.

**Request:** `multipart/form-data`

| Field | Type | Required | Description |
|------|------|------|------|
| `file` | file | Yes | JS plugin file (.jsplugin.zip) |

**Success response (201 new install / 200 update):**

```json
{
  "total": 1,
  "success": 1,
  "failed": 0,
  "results": [
    {
      "file_name": "example.jsplugin.zip",
      "plugin": { ... },
      "success": true
    }
  ],
  "message": "插件 example-plugin 安装成功"
}
```

**On installation failure (200):**

```json
{
  "total": 1,
  "success": 0,
  "failed": 1,
  "results": [
    {
      "file_name": "bad.zip",
      "error": "manifest 缺失",
      "success": false
    }
  ],
  "message": "安装插件失败"
}
```

---

### 2.3 GET /jsplugins/{id} -- Get Plugin Details

**Method:** `GET`
**Path:** `/api/v1/jsplugins/{id}`
**Authentication:** BearerAuth required

**Path parameter:**

| Parameter | Type | Description |
|------|------|------|
| `id` | int | Plugin ID |

**Success response (200):**

```json
{
  "plugin": {
    "id": 1,
    "name": "示例插件",
    "entry_path": "example-plugin",
    "version": "1.0.0",
    "status": "active",
    ...
  }
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 404 | Plugin does not exist |

---

### 2.4 PUT /jsplugins/{id} -- Update Plugin

**Method:** `PUT`
**Path:** `/api/v1/jsplugins/{id}`
**Authentication:** BearerAuth required

**Description:** Uploads a new `.jsplugin.zip` file to update an existing plugin. Plugins in the active state are automatically reloaded after the update.

**Request:** `multipart/form-data`

| Field | Type | Required | Description |
|------|------|------|------|
| `file` | file | Yes | JS plugin file (.jsplugin.zip) |

**Path parameter:**

| Parameter | Type | Description |
|------|------|------|
| `id` | int | Plugin ID |

**Success response (200):**

```json
{
  "plugin": { ... }
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 400 | Bad request data or update failed |
| 404 | Plugin does not exist |

---

### 2.5 DELETE /jsplugins/{id} -- Delete Plugin

**Method:** `DELETE`
**Path:** `/api/v1/jsplugins/{id}`
**Authentication:** BearerAuth required

**Description:** Deletes the specified plugin. First uninstalls the running service, then deletes the files and database record, and finally refreshes the publicPaths cache.

**Path parameter:**

| Parameter | Type | Description |
|------|------|------|
| `id` | int | Plugin ID |

**Success response (200):**

```json
{
  "message": "插件已删除"
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 404 | Plugin does not exist |
| 500 | Failed to delete plugin |

---

### 2.6 POST /jsplugins/{id}/enable -- Enable Plugin

**Method:** `POST`
**Path:** `/api/v1/jsplugins/{id}/enable`
**Authentication:** BearerAuth required

**Description:** Enables the specified JS plugin, loading the JS runtime.

**Path parameter:**

| Parameter | Type | Description |
|------|------|------|
| `id` | int | Plugin ID |

**Success response (200):**

```json
{
  "plugin": { ... }
}
```

---

### 2.7 POST /jsplugins/{id}/disable -- Disable Plugin

**Method:** `POST`
**Path:** `/api/v1/jsplugins/{id}/disable`
**Authentication:** BearerAuth required

**Description:** Disables the specified JS plugin, unloading the JS runtime.

**Path parameter:**

| Parameter | Type | Description |
|------|------|------|
| `id` | int | Plugin ID |

**Success response (200):**

```json
{
  "plugin": { ... }
}
```

---

## 3. Plugin Update Endpoints

### 3.1 GET /jsplugins/{id}/check-update -- Check a Single Plugin for Updates

**Method:** `GET`
**Path:** `/api/v1/jsplugins/{id}/check-update`
**Authentication:** BearerAuth required

**Description:** Checks whether the specified plugin has a remote update.

**Path parameter:**

| Parameter | Type | Description |
|------|------|------|
| `id` | int | Plugin ID |

**Query parameters:**

| Parameter | Type | Required | Description |
|------|------|------|------|
| `github_proxy` | string | No | GitHub proxy prefix |

**Success response (200):**

```json
{
  "has_update": true,
  "current_version": "1.0.0",
  "remote_version": "1.1.0",
  "download_url": "https://github.com/.../releases/download/v1.1.0/plugin.zip"
}
```

---

### 3.2 POST /jsplugins/{id}/update -- Download and Update a Single Plugin

**Method:** `POST`
**Path:** `/api/v1/jsplugins/{id}/update`
**Authentication:** BearerAuth required

**Description:** Downloads and updates the specified plugin from the remote. Plugins in the active state are automatically reloaded after the update.

**Path parameter:**

| Parameter | Type | Description |
|------|------|------|
| `id` | int | Plugin ID |

**Request body (optional):**

```json
{
  "github_proxy": "",
  "force": false
}
```

| Field | Type | Description |
|------|------|------|
| `github_proxy` | string | GitHub proxy prefix |
| `force` | boolean | Set to `true` to skip the version check and force a re-download and reinstall |

**Success response (200):**

```json
{
  "plugin": { ... }
}
```

---

### 3.3 POST /jsplugins/update-all -- Batch Update All Plugins

**Method:** `POST`
**Path:** `/api/v1/jsplugins/update-all`
**Authentication:** BearerAuth required

**Description:** Checks and updates all plugins that have a remote update source. Skips plugins without an `update_url` and plugins already on the latest version. Downloads and installs updates one by one; a single failure does not interrupt the update flow of other plugins.

**Request body (optional):**

```json
{
  "github_proxy": "",
  "force": false
}
```

| Field | Type | Description |
|------|------|------|
| `github_proxy` | string | GitHub proxy prefix |
| `force` | boolean | Set to `true` to skip the version check and force a re-download and reinstall of all plugins |

**Success response (200):**

```json
{
  "total": 5,
  "updated": 2,
  "failed": 1,
  "skipped": 2,
  "results": [
    {
      "plugin_id": 1,
      "plugin_name": "Plugin A",
      "entry_path": "plugin-a",
      "success": true,
      "has_update": true,
      "current_version": "1.0.0",
      "new_version": "1.1.0"
    },
    {
      "plugin_id": 2,
      "plugin_name": "Plugin B",
      "entry_path": "plugin-b",
      "success": false,
      "has_update": true,
      "current_version": "2.0.0",
      "error": "下载更新失败: http status 404"
    }
  ],
  "message": "批量更新完成：2 已更新，1 失败，2 无需更新"
}
```

---

## 4. Registry Endpoints

### 4.1 POST /jsplugins/registry/refresh -- Refresh Registry

**Method:** `POST`
**Path:** `/api/v1/jsplugins/registry/refresh`
**Authentication:** BearerAuth required

**Description:** Fetches the specified registry source URL (including recursive includes), deduplicates and merges, then returns a paginated list of available plugins. Each plugin is marked with whether it is installed and whether it has an update.

**Request body:**

```json
{
  "registry_url": "https://raw.githubusercontent.com/.../registry.json",
  "page": 1,
  "page_size": 20,
  "search": "",
  "github_proxy": ""
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `registry_url` | string | Yes | Registry source URL |
| `page` | int | No | Page number (default 1) |
| `page_size` | int | No | Items per page (default 20, maximum 100) |
| `search` | string | No | Search keyword (matches name, description, author, entry_path) |
| `github_proxy` | string | No | GitHub proxy prefix |

**Success response (200):**

```json
{
  "plugins": [
    {
      "name": "MiOT 音源",
      "entry_path": "songloft-plugin-miot",
      "version": "1.2.0",
      "description": "小米 IoT 设备音源",
      "author": "Songloft",
      "homepage": "https://github.com/...",
      "icon": "icon.png",
      "download_url": "https://github.com/.../releases/download/v1.2.0/plugin.zip",
      "installed": true,
      "installed_version": "1.1.0",
      "has_update": true
    }
  ],
  "total": 15,
  "page": 1,
  "page_size": 20,
  "warnings": []
}
```

---

### 4.2 POST /jsplugins/registry/install -- Install Plugin from Registry

**Method:** `POST`
**Path:** `/api/v1/jsplugins/registry/install`
**Authentication:** BearerAuth required

**Description:** Downloads the ZIP from the `download_url` in the registry and installs the plugin. If the `entry_path` already exists, it automatically follows the update path. Supports a GitHub proxy. The ZIP download size limit is 50MB.

**Request body:**

```json
{
  "download_url": "https://github.com/.../releases/download/v1.0.0/plugin.zip",
  "github_proxy": ""
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `download_url` | string | Yes | Plugin ZIP download address |
| `github_proxy` | string | No | GitHub proxy prefix (prepended to download_url) |

**Success response (201 new install / 200 update):**

The response format is the same as `POST /jsplugins/upload` (`jsPluginUploadResponse`).

---

## 5. Source Health

### 5.1 GET /plugins/health -- Plugin Health

**Method:** `GET`
**Path:** `/api/v1/plugins/health`
**Authentication:** BearerAuth required

**Description:** Returns the download success rate, health category, and most recent failure reasons for each music source plugin, to help troubleshoot plugin issues.

**Success response (200):**

```json
{
  "plugins": [
    {
      "entry_path": "songloft-plugin-miot",
      "name": "MiOT 音源",
      "total_requests": 100,
      "success_count": 95,
      "failure_count": 5,
      "success_rate": 0.95,
      "health": "green",
      "recent_failures": [
        {
          "error": "connection timeout",
          "time": "2026-06-12T10:00:00Z"
        }
      ]
    }
  ]
}
```

---

## 6. Plugin Settings Endpoints

### 6.1 GET/PUT /settings/plugin-registries -- Registry List Configuration

**Path:** `/api/v1/settings/plugin-registries`
**Authentication:** BearerAuth required

Manages the list of plugin registry source URLs configured by the user.

#### GET response / PUT request body:

```json
{
  "registries": [
    {
      "url": "https://raw.githubusercontent.com/songloft-org/songloft-plugin-registry/main/registry.json",
      "name": "Songloft 官方插件",
      "enabled": true
    }
  ]
}
```

| Field | Type | Description |
|------|------|------|
| `registries[].url` | string | Registry source URL |
| `registries[].name` | string | Registry source name |
| `registries[].enabled` | boolean | Whether it is enabled |

When not configured, GET returns the built-in default value (the Songloft official plugin source).

---

### 6.2 GET/PUT /settings/http-proxy -- HTTP Proxy Configuration

**Path:** `/api/v1/settings/http-proxy`
**Authentication:** BearerAuth required
**Tag:** Settings

Configures the global HTTP proxy address. All outbound backend requests (plugin downloads, registry fetching, upgrade checks, etc.) are forwarded through this proxy.

#### GET response / PUT request body:

```json
{
  "proxy": "http://192.168.1.1:7890"
}
```

| Field | Type | Description |
|------|------|------|
| `proxy` | string | Proxy address (supports HTTP/HTTPS/SOCKS5); an empty string means a direct connection |

Takes effect immediately after saving, no restart required. Loopback addresses (`localhost`/`127.0.0.1`/`::1`) automatically skip the proxy.

**PUT error responses:**

| Status | Description |
|--------|------|
| 400 | Bad request format or invalid proxy address |
| 500 | Failed to save configuration |

---

## 7. Plugin Runtime Routes

The following routes are registered by `Manager.RegisterStaticRoutes` and `Manager.RegisterAPIRoutes`, serving a plugin's frontend pages and API calls. `{entryPath}` is determined at runtime by the installed plugins; the OpenAPI schema is a placeholder only.

### 7.1 GET /jsplugin/{entryPath} -- Plugin Root Page

**Method:** `GET`
**Path:** `/api/v1/jsplugin/{entryPath}` and `/api/v1/jsplugin/{entryPath}/`
**Authentication:** None required

**Description:** The JS plugin entry HTML. Injects the `<base>` tag (so relative paths resolve correctly) and the auth-bridge script, then returns `static/index.html`. Static file serving does not depend on whether the JS runtime is ready, ensuring the page can still load during plugin initialization.

**Success response (200):** HTML page
**Error response (404):** Plugin not installed or missing `static/index.html`

---

### 7.2 GET /jsplugin/{entryPath}/static/* -- Plugin Static Resources

**Method:** `GET`
**Path:** `/api/v1/jsplugin/{entryPath}/static` and `/api/v1/jsplugin/{entryPath}/static/*`
**Authentication:** None required

**Description:** Returns static resources such as CSS/JS/images from the plugin's disk directory. Paths that do not match SPA-fall back to `index.html`. HTML files inject the `<base>` tag and are set to no-cache; other resources are set to strong caching (1 year).

---

### 7.3 ANY /jsplugin/{entryPath}/* -- Plugin API Forwarding

**Method:** `GET` / `POST` / `PUT` / `DELETE` (catch-all)
**Path:** `/api/v1/jsplugin/{entryPath}/*`
**Authentication:** BearerAuth required (except for paths declared in `publicPaths`)

**Description:** Accepts any HTTP method, dispatching to the plugin's static fallback or forwarding to the plugin code in the QuickJS sandbox. Non-static paths trigger on-demand lazy loading -- the first request after idle eviction automatically reloads the plugin.

The request body is passed to the JS runtime via JSON serialization; when the body contains non-UTF-8 bytes (such as a multipart upload), base64 encoding is used automatically for passthrough.

**Error responses:**

| Status | Error code | Description |
|--------|--------|------|
| 403 | `plugin_disabled` | Plugin not enabled |
| 404 | `plugin_not_found` | Plugin does not exist |
| 503 | `plugin_unavailable` | Plugin unavailable or running abnormally (the health check will self-heal) |
| 504 | - | JS runtime call timed out |

> The frontend automatically retries a 503 `plugin_unavailable` once (200ms delay), in coordination with the backend's lazy-loading/self-healing mechanism.

---

### 7.4 GET/HEAD /jsplugin/{entryPath}/files/* -- Plugin File Access

**Method:** `GET` / `HEAD`
**Path:** `/api/v1/jsplugin/{entryPath}/files/*`
**Authentication:** BearerAuth required

**Description:** Returns files within the plugin's accessible scope directly via Go's native `http.ServeFile`, supporting Range requests and HTTP caching.

**Path resolution rules:**

| Path format | Description | Required permission |
|----------|------|----------|
| `relative/path` | Relative to the plugin's data directory | `fs` |
| `/absolute/path` | Absolute path, validated to be within the configured directory | `fs:external` |
| `music://xxx` | Resolved as `{music_path}/xxx` | `fs:music` |

Security measures: `filepath.Abs + HasPrefix` prevents path traversal; paths containing `..` are directly rejected.

---

### 7.5 GET /jsplugin-assets/* -- Plugin Common Resources

**Method:** `GET`
**Path:** `/api/v1/jsplugin-assets/*`
**Authentication:** None required

**Description:** Serves the plugin common CSS, JS, and font files embedded by the main program. `injectHTMLHead` automatically injects them into all plugin HTML pages.

Included resources:
- `common.css` -- Defines the `--md-*` CSS variables (light/dark dual themes)
- `common.js` -- embed detection + theme bridging (`postMessage` real-time updates + `data-theme` attribute), exposes the `window.SongloftPlugin` global API

Resources are set to strong caching (1 year, immutable).

---

**Section source:** `internal/handlers/jsplugin.go` / `internal/handlers/jsplugin_registry.go` / `internal/jsplugin/routes.go` / `internal/app/routers.go`
