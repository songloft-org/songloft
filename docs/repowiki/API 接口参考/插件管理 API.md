# 插件管理 API

本文档基于以下源文件编写：

- `internal/handlers/jsplugin.go` -- JS 插件管理 handler（CRUD、启禁用、更新）
- `internal/handlers/jsplugin_registry.go` -- 插件订阅源与注册表安装
- `internal/jsplugin/routes.go` -- JS 插件运行时路由（静态资源、API 转发、文件访问）
- `internal/app/routers.go` -- 路由注册

## 目录

1. [概述](#1-概述)
2. [插件管理端点](#2-插件管理端点)
   - [GET /jsplugins -- 列出所有插件](#21-get-jsplugins----列出所有插件)
   - [POST /jsplugins/upload -- 上传安装插件](#22-post-jsplugins-upload----上传安装插件)
   - [GET /jsplugins/{id} -- 获取插件详情](#23-get-jspluginsid----获取插件详情)
   - [PUT /jsplugins/{id} -- 更新插件](#24-put-jspluginsid----更新插件)
   - [DELETE /jsplugins/{id} -- 删除插件](#25-delete-jspluginsid----删除插件)
   - [POST /jsplugins/{id}/enable -- 启用插件](#26-post-jspluginsidenable----启用插件)
   - [POST /jsplugins/{id}/disable -- 禁用插件](#27-post-jspluginsiddisable----禁用插件)
3. [插件更新端点](#3-插件更新端点)
   - [GET /jsplugins/{id}/check-update -- 检查单个插件更新](#31-get-jspluginsidcheck-update----检查单个插件更新)
   - [POST /jsplugins/{id}/update -- 下载并更新单个插件](#32-post-jspluginsidupdate----下载并更新单个插件)
   - [POST /jsplugins/update-all -- 批量更新所有插件](#33-post-jspluginsupdate-all----批量更新所有插件)
4. [注册表端点](#4-注册表端点)
   - [POST /jsplugins/registry/refresh -- 刷新注册表](#41-post-jspluginsregistryrefresh----刷新注册表)
   - [POST /jsplugins/registry/install -- 从注册表安装插件](#42-post-jspluginsregistryinstall----从注册表安装插件)
5. [音源健康度](#5-音源健康度)
   - [GET /plugins/health -- 插件健康度](#51-get-pluginshealth----插件健康度)
6. [插件设置端点](#6-插件设置端点)
   - [GET/PUT /settings/plugin-registries -- 订阅源列表配置](#61-getput-settingsplugin-registries----订阅源列表配置)
   - [GET/PUT /settings/http-proxy -- HTTP 代理配置](#62-getput-settingshttp-proxy----http-代理配置)
7. [插件运行时路由](#7-插件运行时路由)
   - [GET /jsplugin/{entryPath} -- 插件根页面](#71-get-jspluginentrypath----插件根页面)
   - [GET /jsplugin/{entryPath}/static/* -- 插件静态资源](#72-get-jspluginent-rypathstatic----插件静态资源)
   - [ANY /jsplugin/{entryPath}/* -- 插件 API 转发](#73-any-jspluginentrypath----插件-api-转发)
   - [GET/HEAD /jsplugin/{entryPath}/files/* -- 插件文件访问](#74-gethead-jspluginentrypathfiles----插件文件访问)
   - [GET /jsplugin-assets/* -- 插件公共资源](#75-get-jsplugin-assets----插件公共资源)

---

## 1. 概述

JS 插件系统是 Songloft 的扩展机制，基于 QuickJS 沙盒运行。插件管理 API 分为三层：

- **管理端点**（`/api/v1/jsplugins/*`）：插件的安装、卸载、启禁用、更新，需要 JWT 认证
- **设置端点**（`/api/v1/settings/*`）：订阅源和代理等全局配置，需要 JWT 认证
- **运行时路由**（`/api/v1/jsplugin/{entryPath}/*`）：插件的静态资源服务和 API 转发，认证规则取决于是否声明了 `publicPaths`

管理端点由 `JSPluginHandler.RegisterRoutes` 注册，运行时路由由 `Manager.RegisterStaticRoutes` / `RegisterAPIRoutes` 注册。

---

## 2. 插件管理端点

### 2.1 GET /jsplugins -- 列出所有插件

**方法:** `GET`
**路径:** `/api/v1/jsplugins`
**认证:** 需要 BearerAuth

**描述:** 获取所有已安装的 JS 插件列表。

**成功响应 (200):**

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

### 2.2 POST /jsplugins/upload -- 上传安装插件

**方法:** `POST`
**路径:** `/api/v1/jsplugins/upload`
**认证:** 需要 BearerAuth

**描述:** 上传 `.jsplugin.zip` 压缩包安装新插件。如果 `entry_path` 已存在则自动走覆盖更新路径，并在插件活跃时热重载。上传大小限制 50MB。

**请求:** `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | file | 是 | JS 插件文件（.jsplugin.zip） |

**成功响应 (201 新装 / 200 更新):**

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

**安装失败时 (200):**

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

### 2.3 GET /jsplugins/{id} -- 获取插件详情

**方法:** `GET`
**路径:** `/api/v1/jsplugins/{id}`
**认证:** 需要 BearerAuth

**路径参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | int | 插件 ID |

**成功响应 (200):**

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

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 404 | 插件不存在 |

---

### 2.4 PUT /jsplugins/{id} -- 更新插件

**方法:** `PUT`
**路径:** `/api/v1/jsplugins/{id}`
**认证:** 需要 BearerAuth

**描述:** 上传新的 `.jsplugin.zip` 文件以更新现有插件。活跃状态的插件更新后自动重载。

**请求:** `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | file | 是 | JS 插件文件（.jsplugin.zip） |

**路径参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | int | 插件 ID |

**成功响应 (200):**

```json
{
  "plugin": { ... }
}
```

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 400 | 请求数据错误或更新失败 |
| 404 | 插件不存在 |

---

### 2.5 DELETE /jsplugins/{id} -- 删除插件

**方法:** `DELETE`
**路径:** `/api/v1/jsplugins/{id}`
**认证:** 需要 BearerAuth

**描述:** 删除指定插件。先卸载运行中的服务，再删除文件和数据库记录，最后刷新 publicPaths 缓存。

**路径参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | int | 插件 ID |

**成功响应 (200):**

```json
{
  "message": "插件已删除"
}
```

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 404 | 插件不存在 |
| 500 | 删除插件失败 |

---

### 2.6 POST /jsplugins/{id}/enable -- 启用插件

**方法:** `POST`
**路径:** `/api/v1/jsplugins/{id}/enable`
**认证:** 需要 BearerAuth

**描述:** 启用指定的 JS 插件，加载 JS 运行时。

**路径参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | int | 插件 ID |

**成功响应 (200):**

```json
{
  "plugin": { ... }
}
```

---

### 2.7 POST /jsplugins/{id}/disable -- 禁用插件

**方法:** `POST`
**路径:** `/api/v1/jsplugins/{id}/disable`
**认证:** 需要 BearerAuth

**描述:** 禁用指定的 JS 插件，卸载 JS 运行时。

**路径参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | int | 插件 ID |

**成功响应 (200):**

```json
{
  "plugin": { ... }
}
```

---

## 3. 插件更新端点

### 3.1 GET /jsplugins/{id}/check-update -- 检查单个插件更新

**方法:** `GET`
**路径:** `/api/v1/jsplugins/{id}/check-update`
**认证:** 需要 BearerAuth

**描述:** 检查指定插件是否有远程更新。

**路径参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | int | 插件 ID |

**查询参数:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `github_proxy` | string | 否 | GitHub 代理前缀 |

**成功响应 (200):**

```json
{
  "has_update": true,
  "current_version": "1.0.0",
  "remote_version": "1.1.0",
  "download_url": "https://github.com/.../releases/download/v1.1.0/plugin.zip"
}
```

---

### 3.2 POST /jsplugins/{id}/update -- 下载并更新单个插件

**方法:** `POST`
**路径:** `/api/v1/jsplugins/{id}/update`
**认证:** 需要 BearerAuth

**描述:** 从远程下载并更新指定插件。活跃状态的插件更新后自动重载。

**路径参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | int | 插件 ID |

**请求体（可选）:**

```json
{
  "github_proxy": "",
  "force": false
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `github_proxy` | string | GitHub 代理前缀 |
| `force` | boolean | 设为 `true` 可跳过版本检查强制重新下载安装 |

**成功响应 (200):**

```json
{
  "plugin": { ... }
}
```

---

### 3.3 POST /jsplugins/update-all -- 批量更新所有插件

**方法:** `POST`
**路径:** `/api/v1/jsplugins/update-all`
**认证:** 需要 BearerAuth

**描述:** 检查并更新所有具有远程更新源的插件。跳过无 `update_url` 的插件和已是最新版的插件。逐个下载并安装更新，单个失败不中断其他插件的更新流程。

**请求体（可选）:**

```json
{
  "github_proxy": "",
  "force": false
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `github_proxy` | string | GitHub 代理前缀 |
| `force` | boolean | 设为 `true` 可跳过版本检查强制重新下载安装所有插件 |

**成功响应 (200):**

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

## 4. 注册表端点

### 4.1 POST /jsplugins/registry/refresh -- 刷新注册表

**方法:** `POST`
**路径:** `/api/v1/jsplugins/registry/refresh`
**认证:** 需要 BearerAuth

**描述:** 拉取指定订阅源 URL（含递归 includes），去重合并后返回分页的可用插件列表。每个插件标注是否已安装及是否有更新。

**请求体:**

```json
{
  "registry_url": "https://raw.githubusercontent.com/.../registry.json",
  "page": 1,
  "page_size": 20,
  "search": "",
  "github_proxy": ""
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `registry_url` | string | 是 | 订阅源 URL |
| `page` | int | 否 | 页码（默认 1） |
| `page_size` | int | 否 | 每页数量（默认 20，最大 100） |
| `search` | string | 否 | 搜索关键词（匹配名称、描述、作者、entry_path） |
| `github_proxy` | string | 否 | GitHub 代理前缀 |

**成功响应 (200):**

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

### 4.2 POST /jsplugins/registry/install -- 从注册表安装插件

**方法:** `POST`
**路径:** `/api/v1/jsplugins/registry/install`
**认证:** 需要 BearerAuth

**描述:** 从注册表中的 `download_url` 下载 ZIP 并安装插件。如果 `entry_path` 已存在则自动走更新路径。支持 GitHub 代理。ZIP 下载大小限制 50MB。

**请求体:**

```json
{
  "download_url": "https://github.com/.../releases/download/v1.0.0/plugin.zip",
  "github_proxy": ""
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `download_url` | string | 是 | 插件 ZIP 下载地址 |
| `github_proxy` | string | 否 | GitHub 代理前缀（拼接在 download_url 前面） |

**成功响应 (201 新装 / 200 更新):**

响应格式与 `POST /jsplugins/upload` 相同（`jsPluginUploadResponse`）。

---

## 5. 音源健康度

### 5.1 GET /plugins/health -- 插件健康度

**方法:** `GET`
**路径:** `/api/v1/plugins/health`
**认证:** 需要 BearerAuth

**描述:** 返回各音乐源插件的下载成功率、健康度分类和最近失败原因，辅助排查插件问题。

**成功响应 (200):**

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

## 6. 插件设置端点

### 6.1 GET/PUT /settings/plugin-registries -- 订阅源列表配置

**路径:** `/api/v1/settings/plugin-registries`
**认证:** 需要 BearerAuth

管理用户配置的插件注册表订阅源 URL 列表。

#### GET 响应 / PUT 请求体:

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

| 字段 | 类型 | 说明 |
|------|------|------|
| `registries[].url` | string | 订阅源 URL |
| `registries[].name` | string | 订阅源名称 |
| `registries[].enabled` | boolean | 是否启用 |

未配置时 GET 返回内置默认值（Songloft 官方插件源）。

---

### 6.2 GET/PUT /settings/http-proxy -- HTTP 代理配置

**路径:** `/api/v1/settings/http-proxy`
**认证:** 需要 BearerAuth
**Tag:** 设置

配置全局 HTTP 代理地址。所有后端外发请求（插件下载、注册表拉取、升级检查等）通过此代理转发。

#### GET 响应 / PUT 请求体:

```json
{
  "proxy": "http://192.168.1.1:7890"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `proxy` | string | 代理地址（支持 HTTP/HTTPS/SOCKS5），空字符串表示直连 |

保存后即时生效，无需重启。loopback 地址（`localhost`/`127.0.0.1`/`::1`）自动跳过代理。

**PUT 错误响应:**

| 状态码 | 说明 |
|--------|------|
| 400 | 请求格式错误或代理地址无效 |
| 500 | 保存配置失败 |

---

## 7. 插件运行时路由

以下路由由 `Manager.RegisterStaticRoutes` 和 `Manager.RegisterAPIRoutes` 注册，服务插件的前端页面和 API 调用。`{entryPath}` 由运行时按已安装插件决定，OpenAPI schema 仅作占位。

### 7.1 GET /jsplugin/{entryPath} -- 插件根页面

**方法:** `GET`
**路径:** `/api/v1/jsplugin/{entryPath}` 和 `/api/v1/jsplugin/{entryPath}/`
**认证:** 无需认证

**描述:** JS 插件入口 HTML。注入 `<base>` 标签（使相对路径正确解析）和 auth-bridge 脚本后返回 `static/index.html`。静态文件服务不依赖 JS 运行时是否就绪，确保插件初始化期间页面仍可加载。

**成功响应 (200):** HTML 页面
**错误响应 (404):** 插件未安装或缺少 `static/index.html`

---

### 7.2 GET /jsplugin/{entryPath}/static/* -- 插件静态资源

**方法:** `GET`
**路径:** `/api/v1/jsplugin/{entryPath}/static` 和 `/api/v1/jsplugin/{entryPath}/static/*`
**认证:** 无需认证

**描述:** 从插件磁盘目录返回 CSS/JS/图片等静态资源。未命中的路径 SPA fallback 到 `index.html`。HTML 文件注入 `<base>` 标签并设置 no-cache；其他资源设置强缓存（1 年）。

---

### 7.3 ANY /jsplugin/{entryPath}/* -- 插件 API 转发

**方法:** `GET` / `POST` / `PUT` / `DELETE`（catch-all）
**路径:** `/api/v1/jsplugin/{entryPath}/*`
**认证:** 需要 BearerAuth（声明了 `publicPaths` 的路径除外）

**描述:** 接受任意 HTTP 方法，分发到插件 static 兜底或转发到 QuickJS 沙盒中的插件代码。非 static 路径触发按需懒加载 -- 空闲驱逐后首次请求会自动重新加载插件。

请求体通过 JSON 序列化传递给 JS 运行时；当 body 含非 UTF-8 字节（如 multipart 上传）时自动使用 base64 编码透传。

**错误响应:**

| 状态码 | 错误码 | 说明 |
|--------|--------|------|
| 403 | `plugin_disabled` | 插件未启用 |
| 404 | `plugin_not_found` | 插件不存在 |
| 503 | `plugin_unavailable` | 插件不可用或运行异常（健康检查会自愈） |
| 504 | - | JS 运行时调用超时 |

> 前端对 503 `plugin_unavailable` 自动重试一次（200ms 延迟），配合后端懒加载/自愈机制。

---

### 7.4 GET/HEAD /jsplugin/{entryPath}/files/* -- 插件文件访问

**方法:** `GET` / `HEAD`
**路径:** `/api/v1/jsplugin/{entryPath}/files/*`
**认证:** 需要 BearerAuth

**描述:** 通过 Go 原生 `http.ServeFile` 直接返回插件可访问范围内的文件，支持 Range 请求和 HTTP 缓存。

**路径解析规则:**

| 路径格式 | 说明 | 所需权限 |
|----------|------|----------|
| `relative/path` | 相对于插件 data 目录 | `fs` |
| `/absolute/path` | 绝对路径，校验在配置的目录内 | `fs:external` |
| `music://xxx` | 解析为 `{music_path}/xxx` | `fs:music` |

安全措施：`filepath.Abs + HasPrefix` 防止路径穿越，包含 `..` 的路径直接拒绝。

---

### 7.5 GET /jsplugin-assets/* -- 插件公共资源

**方法:** `GET`
**路径:** `/api/v1/jsplugin-assets/*`
**认证:** 无需认证

**描述:** 服务由主程序嵌入的插件通用 CSS、JS 和字体文件。`injectHTMLHead` 自动注入到所有插件 HTML 页面。

包含的资源：
- `common.css` -- 定义 `--md-*` CSS 变量（亮/暗双主题）
- `common.js` -- embed 检测 + 主题桥接（`postMessage` 实时更新 + `data-theme` 属性），暴露 `window.SongloftPlugin` 全局 API

资源设置强缓存（1 年，immutable）。

---

**章节来源：** `internal/handlers/jsplugin.go` / `internal/handlers/jsplugin_registry.go` / `internal/jsplugin/routes.go` / `internal/app/routers.go`
