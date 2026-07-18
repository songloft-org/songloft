# 系统管理 API

本文档基于以下源文件编写：

- `internal/handlers/health.go` -- 健康检查处理器
- `internal/handlers/version.go` -- 版本信息处理器
- `internal/handlers/upgrade.go` -- 系统升级处理器
- `internal/handlers/proxy.go` -- 资源代理处理器
- `internal/app/routers.go` -- 路由注册
- `internal/models/models.go` -- 升级进度等数据模型

## 目录

1. [概述](#1-概述)
2. [公开端点](#2-公开端点)
   - [GET /health -- 健康检查](#21-get-health----健康检查)
   - [GET /version -- 版本信息](#22-get-version----版本信息)
3. [升级管理端点](#3-升级管理端点)
   - [GET /upgrade/versions -- 获取可用版本信息](#31-get-upgradeversions----获取可用版本信息)
   - [GET /upgrade/check -- 检查更新](#32-get-upgradecheck----检查更新)
   - [POST /upgrade/start -- 开始升级](#33-post-upgradestart----开始升级)
   - [POST /upgrade/reset -- 回退到底包版本](#34-post-upgradereset----回退到底包版本)
   - [GET /upgrade/progress -- 获取升级进度](#35-get-upgradeprogress----获取升级进度)
4. [资源代理端点](#4-资源代理端点)
   - [GET /proxy -- 代理外部资源](#41-get-proxy----代理外部资源)

---

## 1. 概述

系统管理模块提供应用的健康检查、版本信息、在线升级和资源代理功能。其中健康检查和版本信息是公开端点（无需认证），其余端点需要 JWT 认证。

升级功能仅在 Docker 环境下可用，非 Docker 环境调用升级相关端点会返回 403。

所有端点基础路径为 `/api/v1`。

---

## 2. 公开端点

### 2.1 GET /health -- 健康检查

**方法:** `GET`
**路径:** `/api/v1/health`
**认证:** 无需认证

**描述:** 检查应用是否正常运行。

**成功响应 (200):**

```json
{
  "status": "ok"
}
```

---

### 2.2 GET /version -- 版本信息

**方法:** `GET`
**路径:** `/api/v1/version`
**认证:** 无需认证

**描述:** 获取应用的版本信息，包括版本号、Git 提交哈希和构建时间。

**成功响应 (200):**

```json
{
  "version": "1.2.0",
  "full": "v1.2.0-abc1234",
  "git_commit": "abc1234",
  "build_time": "2026-06-12_10:00:00"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `version` | string | 版本号（如 `1.2.0` 或 `dev`） |
| `full` | string | 完整版本字符串（含 commit hash） |
| `git_commit` | string | Git 提交哈希 |
| `build_time` | string | 构建时间 |

---

## 3. 升级管理端点

所有升级端点需要 JWT 认证且仅在 Docker 环境下可用。非 Docker 环境返回 `403 Forbidden`。

### 3.1 GET /upgrade/versions -- 获取可用版本信息

**方法:** `GET`
**路径:** `/api/v1/upgrade/versions`
**认证:** 需要 BearerAuth

**描述:** 获取正式版（stable）和测试版（dev）的远程版本信息。

**查询参数:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `github_proxy` | string | 否 | GitHub 代理前缀 |

**成功响应 (200):**

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

当某个渠道获取失败时，对应字段返回 `{"error": "错误信息"}`。

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 403 | 非 Docker 环境不支持升级 |
| 500 | 获取版本信息失败 |

---

### 3.2 GET /upgrade/check -- 检查更新

**方法:** `GET`
**路径:** `/api/v1/upgrade/check`
**认证:** 需要 BearerAuth

**描述:** 检查当前通道是否有可用的新版本。dev 只检查 dev，release 只检查 stable；dev 按 `build_time` 判断，release 按版本号判断。同时提供嵌套和扁平字段，方便前端解析。

**查询参数:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `github_proxy` | string | 否 | GitHub 代理前缀 |

**成功响应 (200):**

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

| 字段 | 类型 | 说明 |
|------|------|------|
| `is_docker` | boolean | 是否为 Docker 环境 |
| `has_update` | boolean | 是否有可用更新 |
| `current_version` | string | 当前版本号 |
| `current_channel` | string | 当前通道：`stable` 或 `dev` |
| `current_build_type` | string | 当前构建类型：`full` 或 `lite` |
| `latest_version` | string | 当前通道内的最新版本号 |
| `release_notes` | string | 最新版本发布说明 |
| `current` | object | 当前版本详细信息 |
| `updates` | object | 可用更新详情（只包含当前允许升级的渠道） |

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 500 | 检查更新失败 |

---

### 3.3 POST /upgrade/start -- 开始升级

**方法:** `POST`
**路径:** `/api/v1/upgrade/start`
**认证:** 需要 BearerAuth

**描述:** 开始升级到指定版本。升级在后台异步执行，可通过 `/upgrade/progress` 轮询进度。

**请求体:**

```json
{
  "version_type": "stable",
  "github_proxy": ""
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `version_type` | string | 是 | 版本类型：`stable` 或 `dev`，必须与当前运行通道一致 |
| `github_proxy` | string | 否 | GitHub 代理前缀 |

**成功响应 (200):**

```json
{
  "message": "升级已开始，请稍候..."
}
```

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 400 | 无效的请求参数或版本类型 |
| 403 | 非 Docker 环境不支持升级 |
| 500 | 升级失败 |

---

### 3.4 POST /upgrade/reset -- 回退到底包版本

**方法:** `POST`
**路径:** `/api/v1/upgrade/reset`
**认证:** 需要 BearerAuth

**描述:** 将二进制文件回退到 Docker 镜像中的原始版本（底包），然后重启服务。在后台异步执行。

**成功响应 (200):**

```json
{
  "message": "回退已开始，服务即将重启..."
}
```

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 403 | 非 Docker 环境不支持回退 |

---

### 3.5 GET /upgrade/progress -- 获取升级进度

**方法:** `GET`
**路径:** `/api/v1/upgrade/progress`
**认证:** 需要 BearerAuth

**描述:** 获取当前升级任务的进度信息。

**成功响应 (200):**

```json
{
  "status": "downloading",
  "progress": 50,
  "current_step": "正在下载新版本...",
  "error": ""
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | 升级状态：`idle` / `downloading` / `testing` / `replacing` / `resetting` / `restarting` / `failed` |
| `progress` | int | 进度百分比（0-100） |
| `current_step` | string | 当前步骤描述 |
| `error` | string | 错误信息（如有） |

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 403 | 非 Docker 环境不支持升级 |

---

## 4. 资源代理端点

### 4.1 GET /proxy -- 代理外部资源

**方法:** `GET`
**路径:** `/api/v1/proxy`
**认证:** 需要 BearerAuth

**描述:** 代理外部资源（图片、音频、视频流等），解决浏览器 CORS 限制。支持流式转发、Range 请求透传、Content-Type 透传和域名白名单校验。

**查询参数:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 目标资源的 URL（URL 编码） |

**成功响应:**

| 状态码 | 说明 |
|--------|------|
| 200 | 代理的资源内容（流式转发） |
| 206 | 部分内容（Range 请求） |

**透传的响应头:**
- `Content-Type`
- `Content-Length`
- `Content-Range`
- `Accept-Ranges`
- `Cache-Control`
- `ETag`
- `Last-Modified`

对图片资源自动设置 7 天缓存（`max-age=604800`）。

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 400 | 缺少 url 参数 / URL 无效 / 仅支持 http/https 协议 |
| 403 | 域名不在白名单中 |
| 502 | 上游请求失败 |

**安全机制:**

- 仅允许 HTTP/HTTPS 协议
- 域名白名单校验（`services.IsHostnameAllowed`），未通过的域名记录告警日志
- 不自动跟随超过 10 次的重定向
- 设置通用 User-Agent 避免被上游 CDN 拒绝
- 支持上游 URL 中的 Basic Auth（自动提取并设置，清理 URL 中的凭据）

---

**章节来源：** `internal/handlers/health.go` / `internal/handlers/version.go` / `internal/handlers/upgrade.go` / `internal/handlers/proxy.go` / `internal/app/routers.go` / `internal/models/models.go`
