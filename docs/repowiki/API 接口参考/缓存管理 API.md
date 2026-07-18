# 缓存管理 API

本文档基于以下源文件编写：

- `internal/handlers/cache.go` -- 缓存管理处理器
- `internal/app/routers.go` -- 路由注册
- `internal/services/cache_service.go` -- 缓存服务与数据模型

## 目录

1. [概述](#1-概述)
2. [端点列表](#2-端点列表)
   - [GET /cache-manage/stats -- 获取缓存统计信息](#21-get-cache-managestats----获取缓存统计信息)
   - [POST /cache-manage/clean -- 清理全部缓存](#22-post-cache-manageclean----清理全部缓存)
   - [GET /cache-manage/config -- 获取缓存配置](#23-get-cache-manageconfig----获取缓存配置)
   - [PUT /cache-manage/config -- 更新缓存配置](#24-put-cache-manageconfig----更新缓存配置)
   - [POST /cache-manage/validate-dir -- 验证缓存目录](#25-post-cache-managevalidate-dir----验证缓存目录)
3. [设计说明](#3-设计说明)

---

## 1. 概述

缓存管理模块负责服务端音乐缓存的查看、配置和清理。播放远程歌曲时系统透明缓存音频文件到服务端，缓存命中时直接返回本地文件，减少外部网络请求。

所有端点均需 JWT 认证（`BearerAuth`），基础路径为 `/api/v1/cache-manage`。

缓存管理采用「业务模块聚合端点」模式 -- 配置（`/config`）与动作（`/stats`、`/clean`）端点共用同一前缀，而非拆分到 `/settings/` 下。

---

## 2. 端点列表

### 2.1 GET /cache-manage/stats -- 获取缓存统计信息

**方法:** `GET`
**路径:** `/api/v1/cache-manage/stats`
**认证:** 需要 BearerAuth

**描述:** 获取服务端音乐缓存的统计信息，包括总大小、文件数量和最大缓存限制。

**成功响应 (200):**

```json
{
  "total_size": 536870912,
  "file_count": 128,
  "max_size": 1073741824
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `total_size` | int64 | 当前缓存总大小（字节） |
| `file_count` | int | 缓存文件数量 |
| `max_size` | int64 | 最大缓存大小（字节），`0` 表示无限制 |

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 500 | 服务器错误 |

---

### 2.2 POST /cache-manage/clean -- 清理全部缓存

**方法:** `POST`
**路径:** `/api/v1/cache-manage/clean`
**认证:** 需要 BearerAuth

**描述:** 删除服务端所有已缓存的音乐文件。清理后远程歌曲需要重新下载缓存。

**成功响应 (200):**

```json
{
  "message": "缓存已清理"
}
```

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 500 | 清理缓存失败 |

---

### 2.3 GET /cache-manage/config -- 获取缓存配置

**方法:** `GET`
**路径:** `/api/v1/cache-manage/config`
**认证:** 需要 BearerAuth

**描述:** 获取服务端音乐缓存的配置信息，包括最大缓存大小限制和缓存目录路径。`cache_dir` 为空表示使用 `default_cache_dir`。

**成功响应 (200):**

```json
{
  "max_size": 1073741824,
  "cache_dir": "",
  "default_cache_dir": "/app/data/music_cache"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `max_size` | int64 | 最大缓存大小（字节），`0` 表示无限制（默认 1GB） |
| `cache_dir` | string | 自定义缓存目录（空字符串表示使用默认目录） |
| `default_cache_dir` | string | 只读，默认缓存目录路径（`{data_dir}/music_cache/`） |

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 500 | 服务器错误 |

---

### 2.4 PUT /cache-manage/config -- 更新缓存配置

**方法:** `PUT`
**路径:** `/api/v1/cache-manage/config`
**认证:** 需要 BearerAuth

**描述:** 更新服务端音乐缓存的配置。`cache_dir` 为空字符串时恢复使用默认目录。更新后自动触发 LRU 淘汰检查。切换目录时不会自动迁移旧缓存文件。

**请求体:**

```json
{
  "max_size": 2147483648,
  "cache_dir": "/mnt/ssd/cache"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `max_size` | int64 | 是 | 最大缓存大小（字节），`0` 表示不限制，不能为负数 |
| `cache_dir` | string | 是 | 自定义缓存目录（必须为绝对路径），空字符串恢复默认 |

**成功响应 (200):**

返回更新后的完整配置（同 GET 响应格式，含 `default_cache_dir`）。

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 400 | 请求参数无效 / 最大缓存大小不能为负数 / 缓存目录必须为绝对路径 / 缓存目录不可用 |
| 500 | 更新缓存配置失败 |

**验证规则:**

- `max_size` 不能为负数
- `cache_dir` 非空时必须为绝对路径
- `cache_dir` 非空时会检查目录可写性（不存在时自动创建）

---

### 2.5 POST /cache-manage/validate-dir -- 验证缓存目录

**方法:** `POST`
**路径:** `/api/v1/cache-manage/validate-dir`
**认证:** 需要 BearerAuth

**描述:** 验证指定目录是否可用作缓存目录。目录不存在时自动创建，检查可写性并返回磁盘空间信息。用于在正式切换目录前预先验证。

**请求体:**

```json
{
  "path": "/mnt/ssd/cache"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `path` | string | 是 | 待验证的目录路径（必须为绝对路径） |

**成功响应 (200) -- 验证通过:**

```json
{
  "valid": true,
  "created": true,
  "total_size": 107374182400,
  "free_size": 53687091200
}
```

**成功响应 (200) -- 验证失败:**

```json
{
  "valid": false,
  "created": false,
  "total_size": 0,
  "free_size": 0,
  "error": "目录不可写: permission denied"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `valid` | boolean | 目录是否可用 |
| `created` | boolean | 目录是否为本次新创建 |
| `total_size` | int64 | 磁盘总容量（字节） |
| `free_size` | int64 | 磁盘可用空间（字节） |
| `error` | string | 验证失败时的错误原因（可选） |

**错误响应:**

| 状态码 | 说明 |
|--------|------|
| 400 | 请求参数无效 / 路径不能为空 |

> 注意：路径不是绝对路径时，响应中 `error` 字段返回 `"必须为绝对路径"`，HTTP 状态码仍为 200。

---

## 3. 设计说明

### LRU 淘汰策略

当缓存总大小超出 `max_size` 时，系统按最后访问时间淘汰最久未使用的文件。`max_size=0` 表示不限制缓存大小。

### Inflight 去重

同一 `song.ID` 的并发请求只下载一次。首请求被 `ctx.Canceled` 时后续等待者自动重试，避免重复下载浪费带宽。

### 跨设备文件搬移

缓存下载使用 `moveFile` 替代裸 `os.Rename`，自动处理跨文件系统（如 `/tmp` 到独立挂载点）的 `EXDEV` 错误，回退为 copy + remove。

---

**章节来源：** `internal/handlers/cache.go` / `internal/app/routers.go` / `internal/services/cache_service.go`
