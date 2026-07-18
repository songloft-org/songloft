# API 接口参考

本文档基于以下源文件编写：

- `internal/app/routers.go` -- 完整路由注册（API v1 路由组、中间件挂载、JS 插件路由）
- `internal/handlers/response.go` -- respondJSON / respondError 响应辅助函数
- `internal/middleware/auth.go` -- JWT 认证中间件（Bearer Token + access_token 查询参数回退）
- `docs/api_response.md` -- API 响应格式规范

## 目录

1. [基础信息](#1-基础信息)
   - [API 版本与基础 URL](#11-api-版本与基础-url)
   - [认证方式](#12-认证方式)
   - [响应格式](#13-响应格式)
   - [Content-Type 约定](#14-content-type-约定)
   - [分页](#15-分页)
2. [公开端点与认证端点](#2-公开端点与认证端点)
3. [API 分组索引](#3-api-分组索引)
   - [歌曲管理](#31-歌曲管理)
   - [歌单管理](#32-歌单管理)
   - [认证](#33-认证)
   - [配置与设置](#34-配置与设置)
   - [扫描管理](#35-扫描管理)
   - [缓存管理](#36-缓存管理)
   - [插件管理](#37-插件管理)
   - [系统管理](#38-系统管理)

---

## 1. 基础信息

### 1.1 API 版本与基础 URL

**章节来源**: `internal/app/routers.go`

所有业务接口统一注册在 `/api/v1` 路由组下。完整 URL 格式：

```
{scheme}://{host}:{port}{base_path}/api/v1/{endpoint}
```

| 组成部分 | 默认值 | 说明 |
|----------|--------|------|
| `scheme` | `http` | 反代后可为 `https` |
| `host:port` | `localhost:58091` | 启动时通过 `-addr` 参数配置 |
| `base_path` | 空 | 通过 `-base-path /xxx` 或环境变量 `BASE_PATH=/xxx` 配置子路径部署；后端用 `http.StripPrefix` 剥离前缀 |

示例：默认启动时，歌曲列表接口为 `http://localhost:58091/api/v1/songs`；若配置 `BASE_PATH=/music`，则为 `http://localhost:58091/music/api/v1/songs`。

当前 API 版本为 **v1**，所有端点均以 `/api/v1/` 为前缀。开发模式下可通过 `/swagger/index.html` 查看交互式 API 文档。

### 1.2 认证方式

**章节来源**: `internal/middleware/auth.go`

Songloft 使用 **JWT 双令牌机制**（Access Token + Refresh Token）。认证中间件按以下优先级获取令牌：

1. **Authorization 请求头**（首选）：`Authorization: Bearer <access_token>`
2. **URL 查询参数**（回退）：`?access_token=<token>`，用于 `<img>` 标签、音频播放器等无法自定义 Header 的场景

认证失败时返回 JSON 格式错误（而非纯文本），保持与业务端点一致：

```json
{"error": "缺少认证信息"}
{"error": "无效的 token", "detail": "token has expired"}
```

> **特殊处理**: 小爱音箱固件会将 URL 中的 `&` 替换为空格，中间件会自动拆分并还原被吞掉的参数。

### 1.3 响应格式

**章节来源**: `internal/handlers/response.go`、`docs/api_response.md`

项目采用 **RESTful 直返风格**，不使用 `{code, data, message}` 统一信封。

#### 成功响应

| 场景 | 格式 | 示例 |
|------|------|------|
| 单个实体 | 直接返回模型对象 | `{"id":1, "title":"Sample Track", ...}` |
| 列表（含分页） | 集合名 + 分页元数据 | `{"songs":[...], "total":100, "limit":20, "offset":0}` |
| 操作结果 | `{"message": "..."}` | `{"message": "歌曲已删除"}` |

#### 错误响应

所有 API 端点的错误统一通过 `respondError` 返回，格式固定：

```json
{"error": "人类可读的错误信息", "detail": "可选的技术细节"}
```

- `error`（必有）：面向用户的简短描述
- `detail`（可选）：底层错误信息，仅当内部 `err != nil` 时输出

中间件层错误（认证失败等）同样返回 JSON 格式，使用相同的 `{error, detail}` 字段结构。

#### 例外：二进制流端点

播放（`/songs/{id}/play`）、资源代理（`/proxy`）、封面图片（`/songs/{id}/cover`）等二进制流端点，错误可能返回纯文本（`http.Error`），因为客户端不期望 JSON body。

### 1.4 Content-Type 约定

**章节来源**: `internal/handlers/response.go`、`internal/app/routers.go`

默认 `application/json`。例外情况：音频播放（`audio/*`，支持 Range）、封面图片（`image/*`）、歌单导出与插件文件（`application/octet-stream`）、插件页面（`text/html`）、HLS 播放列表（`application/vnd.apple.mpegurl`）。上传接口（插件安装、歌单封面）使用 `multipart/form-data`。

### 1.5 分页

**章节来源**: `internal/handlers/music.go`

支持分页的列表接口使用 `limit`（默认 20，超出上限自动截断）和 `offset`（默认 0）查询参数。响应体包含集合名、`total`、`limit`、`offset` 四个字段。

---

## 2. 公开端点与认证端点

**章节来源**: `internal/app/routers.go`

路由注册分为两层：公开端点（无需认证）和授权端点（需 JWT）。

**公开端点**（无需 Bearer Token）：

| 端点 | 说明 |
|------|------|
| `POST /api/v1/auth/login` | 用户登录，获取令牌对 |
| `POST /api/v1/auth/refresh` | 用刷新令牌换取新的访问令牌 |
| `GET /api/v1/version` | 获取服务版本信息 |
| `GET /api/v1/health` | 健康检查 |
| `GET /api/v1/jsplugin/{entryPath}` | 插件静态页面（HTML） |
| `GET /api/v1/jsplugin/{entryPath}/static/*` | 插件静态资源（CSS/JS/图片） |
| `GET /api/v1/jsplugin-assets/*` | 插件公共资源（common.css/common.js/字体） |
| 插件 `publicPaths` 声明的路径 | 插件 manifest 中声明的无需认证的 API 路径 |

**认证端点**（需 Bearer Token）：

除上述公开端点外的所有 `/api/v1/*` 端点均需认证。认证失败返回 `401 Unauthorized`。

---

## 3. API 分组索引

**图表来源**: `internal/app/routers.go`、`internal/handlers/jsplugin.go`、`internal/jsplugin/routes.go`

以下按业务模块列出所有 API 分组。每组标注的端点数量为**逻辑端点数**（HEAD 别名和 `.m3u8` 后缀变体不重复计数）。

### 3.1 歌曲管理

> 详见 [歌曲 API](歌曲%20API.md)

| 前缀 | 端点数 | 说明 |
|------|--------|------|
| `/api/v1/songs` | 19 | 歌曲 CRUD、批量操作、音频播放、封面、歌词、HLS 电台代理 |

覆盖：CRUD 与批量操作（列表/筛选/添加远程歌曲/电台/清理/批量删除/重复检测）、元数据写入（歌词/标签回写/整理）、媒体端点（音频流/封面/歌词，支持 Range 请求与 local/remote/radio 分发）、HLS 电台反向代理（m3u8 改写 + 切片转发）。

### 3.2 歌单管理

> 详见 [歌单 API](歌单%20API.md)

| 前缀 | 端点数 | 说明 |
|------|--------|------|
| `/api/v1/playlists` | 16 | 歌单 CRUD、歌曲管理、排序、封面、导入导出 |

覆盖：CRUD 与批量操作（列表/创建/更新/删除/批量删除/排序）、歌单歌曲管理（添加/排序/移除/最后播放时间）、封面上传与获取、数据导入导出（JSON 格式，含歌曲去重匹配）。

### 3.3 认证

> 详见 [认证 API](认证%20API.md)

| 前缀 | 端点数 | 说明 |
|------|--------|------|
| `/api/v1/auth` | 6 | 登录、令牌刷新、登出、令牌管理 |

覆盖：公开端点（登录获取令牌对、刷新令牌）、需授权端点（登出、列出/查看/撤销活跃令牌）。

### 3.4 配置与设置

> 详见 [配置与设置 API](配置与设置%20API.md)

| 前缀 | 端点数 | 说明 |
|------|--------|------|
| `/api/v1/configs` | 5 | 通用 KV 配置（admin 编辑器） |
| `/api/v1/settings/*` | 20 | 业务功能设置（10 项，各含 GET + PUT） |

覆盖：通用 KV 配置 CRUD（admin 编辑器）、10 项业务设置（HLS 代理/音乐目录/扫描策略/自动扫描/日志等级/插件源/HTTP 代理/标签页配置）。业务设置为强类型 JSON，内置默认值，PUT 可触发副作用。

### 3.5 扫描管理

> 详见 [扫描管理 API](扫描管理%20API.md)

| 前缀 | 端点数 | 说明 |
|------|--------|------|
| `/api/v1/scan` | 8 | 音乐目录扫描、进度查询、音频指纹计算 |

覆盖：扫描控制（触发/进度/取消）、目录浏览（目录树/目录名列表）、音频指纹批量计算（状态/启动/进度）。

### 3.6 缓存管理

> 详见 [缓存管理 API](缓存管理%20API.md)

| 前缀 | 端点数 | 说明 |
|------|--------|------|
| `/api/v1/cache-manage` | 5 | 远程歌曲缓存统计、清理、配置 |

覆盖：缓存统计与手动清理、读取/更新缓存配置（目录路径、LRU 上限）、预验证缓存目录（可写性 + 磁盘空间）。

### 3.7 插件管理

> 详见 [插件管理 API](插件管理%20API.md)

| 前缀 | 端点数 | 说明 |
|------|--------|------|
| `/api/v1/jsplugins` | 13 | 插件生命周期管理（安装、更新、启用/禁用） |
| `/api/v1/jsplugin/{entryPath}` | 8 | 插件运行时路由（静态页面、API 转发、文件访问） |
| `/api/v1/jsplugin-assets` | 1 | 插件公共资源（CSS/JS/字体） |
| `/api/v1/plugins/health` | 1 | 插件运行时健康检查 |

覆盖：生命周期管理（安装/更新/删除/启用/禁用/检查更新/批量更新）、插件源注册表刷新与安装、运行时路由（静态页面 SPA fallback + QuickJS 沙盒 API 转发 + 文件直接访问）、公共资源（主题 CSS/JS/字体）。

### 3.8 系统管理

> 详见 [系统管理 API](系统管理%20API.md)

| 前缀 | 端点数 | 说明 |
|------|--------|------|
| `/api/v1/version` | 1 | 版本信息（公开） |
| `/api/v1/health` | 1 | 健康检查（公开） |
| `/api/v1/upgrade` | 5 | 系统升级（仅 Docker） |
| `/api/v1/proxy` | 1 | 资源代理（解决 CDN CORS） |

覆盖：版本信息与健康检查（公开）、在线升级（版本列表/检查/启动/回滚/进度，仅 Docker 可用）、资源代理（解决外部 CDN CORS 限制）。

---

> **完整 API 文档**：开发模式启动后访问 `http://localhost:58091/swagger/index.html` 查看由 swaggo 从源码注释生成的交互式 Swagger UI。
