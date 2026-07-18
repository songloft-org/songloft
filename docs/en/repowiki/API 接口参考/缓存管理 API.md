# Cache Management API

This document is based on the following source files:

- `internal/handlers/cache.go` -- Cache management handler
- `internal/app/routers.go` -- Route registration
- `internal/services/cache_service.go` -- Cache service and data models

## Table of Contents

1. [Overview](#1-overview)
2. [Endpoint List](#2-endpoint-list)
   - [GET /cache-manage/stats -- Get Cache Statistics](#21-get-cache-managestats----get-cache-statistics)
   - [POST /cache-manage/clean -- Clear All Cache](#22-post-cache-manageclean----clear-all-cache)
   - [GET /cache-manage/config -- Get Cache Configuration](#23-get-cache-manageconfig----get-cache-configuration)
   - [PUT /cache-manage/config -- Update Cache Configuration](#24-put-cache-manageconfig----update-cache-configuration)
   - [POST /cache-manage/validate-dir -- Validate Cache Directory](#25-post-cache-managevalidate-dir----validate-cache-directory)
3. [Design Notes](#3-design-notes)

---

## 1. Overview

The cache management module handles viewing, configuring, and clearing the server-side music cache. When playing remote songs, the system transparently caches audio files to the server, and on a cache hit returns the local file directly, reducing external network requests.

All endpoints require JWT authentication (`BearerAuth`), with the base path `/api/v1/cache-manage`.

Cache management adopts the "business module aggregation endpoint" pattern -- the configuration (`/config`) and action (`/stats`, `/clean`) endpoints share the same prefix rather than being split under `/settings/`.

---

## 2. Endpoint List

### 2.1 GET /cache-manage/stats -- Get Cache Statistics

**Method:** `GET`
**Path:** `/api/v1/cache-manage/stats`
**Authentication:** BearerAuth required

**Description:** Gets statistics for the server-side music cache, including total size, file count, and the maximum cache limit.

**Success response (200):**

```json
{
  "total_size": 536870912,
  "file_count": 128,
  "max_size": 1073741824
}
```

| Field | Type | Description |
|------|------|------|
| `total_size` | int64 | Current total cache size (bytes) |
| `file_count` | int | Number of cache files |
| `max_size` | int64 | Maximum cache size (bytes); `0` means unlimited |

**Error responses:**

| Status | Description |
|--------|------|
| 500 | Server error |

---

### 2.2 POST /cache-manage/clean -- Clear All Cache

**Method:** `POST`
**Path:** `/api/v1/cache-manage/clean`
**Authentication:** BearerAuth required

**Description:** Deletes all cached music files on the server. After clearing, remote songs need to be downloaded and cached again.

**Success response (200):**

```json
{
  "message": "缓存已清理"
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 500 | Failed to clear cache |

---

### 2.3 GET /cache-manage/config -- Get Cache Configuration

**Method:** `GET`
**Path:** `/api/v1/cache-manage/config`
**Authentication:** BearerAuth required

**Description:** Gets the configuration of the server-side music cache, including the maximum cache size limit and the cache directory path. An empty `cache_dir` means the `default_cache_dir` is used.

**Success response (200):**

```json
{
  "max_size": 1073741824,
  "cache_dir": "",
  "default_cache_dir": "/app/data/music_cache"
}
```

| Field | Type | Description |
|------|------|------|
| `max_size` | int64 | Maximum cache size (bytes); `0` means unlimited (default 1GB) |
| `cache_dir` | string | Custom cache directory (empty string means the default directory is used) |
| `default_cache_dir` | string | Read-only, the default cache directory path (`{data_dir}/music_cache/`) |

**Error responses:**

| Status | Description |
|--------|------|
| 500 | Server error |

---

### 2.4 PUT /cache-manage/config -- Update Cache Configuration

**Method:** `PUT`
**Path:** `/api/v1/cache-manage/config`
**Authentication:** BearerAuth required

**Description:** Updates the configuration of the server-side music cache. When `cache_dir` is an empty string, the default directory is restored. After updating, an LRU eviction check is automatically triggered. When switching directories, old cache files are not automatically migrated.

**Request body:**

```json
{
  "max_size": 2147483648,
  "cache_dir": "/mnt/ssd/cache"
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `max_size` | int64 | Yes | Maximum cache size (bytes); `0` means unlimited, cannot be negative |
| `cache_dir` | string | Yes | Custom cache directory (must be an absolute path); an empty string restores the default |

**Success response (200):**

Returns the complete updated configuration (same format as the GET response, including `default_cache_dir`).

**Error responses:**

| Status | Description |
|--------|------|
| 400 | Invalid request parameters / maximum cache size cannot be negative / cache directory must be an absolute path / cache directory unavailable |
| 500 | Failed to update cache configuration |

**Validation rules:**

- `max_size` cannot be negative
- `cache_dir`, when non-empty, must be an absolute path
- `cache_dir`, when non-empty, is checked for directory writability (created automatically if it does not exist)

---

### 2.5 POST /cache-manage/validate-dir -- Validate Cache Directory

**Method:** `POST`
**Path:** `/api/v1/cache-manage/validate-dir`
**Authentication:** BearerAuth required

**Description:** Validates whether the specified directory can be used as a cache directory. Creates the directory automatically if it does not exist, checks writability, and returns disk space information. Used to pre-validate before formally switching directories.

**Request body:**

```json
{
  "path": "/mnt/ssd/cache"
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `path` | string | Yes | The directory path to validate (must be an absolute path) |

**Success response (200) -- Validation passed:**

```json
{
  "valid": true,
  "created": true,
  "total_size": 107374182400,
  "free_size": 53687091200
}
```

**Success response (200) -- Validation failed:**

```json
{
  "valid": false,
  "created": false,
  "total_size": 0,
  "free_size": 0,
  "error": "目录不可写: permission denied"
}
```

| Field | Type | Description |
|------|------|------|
| `valid` | boolean | Whether the directory is usable |
| `created` | boolean | Whether the directory was newly created this time |
| `total_size` | int64 | Total disk capacity (bytes) |
| `free_size` | int64 | Free disk space (bytes) |
| `error` | string | The reason for validation failure (optional) |

**Error responses:**

| Status | Description |
|--------|------|
| 400 | Invalid request parameters / path cannot be empty |

> Note: When the path is not an absolute path, the `error` field in the response returns `"必须为绝对路径"`, and the HTTP status code is still 200.

---

## 3. Design Notes

### LRU Eviction Strategy

When the total cache size exceeds `max_size`, the system evicts the least recently used files by last access time. `max_size=0` means the cache size is unlimited.

### Inflight Deduplication

Concurrent requests for the same `song.ID` download only once. When the first request is cancelled by `ctx.Canceled`, subsequent waiters automatically retry, avoiding wasting bandwidth on duplicate downloads.

### Cross-Device File Moving

Cache downloads use `moveFile` instead of a bare `os.Rename`, automatically handling the `EXDEV` error across file systems (such as from `/tmp` to a separate mount point) by falling back to copy + remove.

---

**Section source:** `internal/handlers/cache.go` / `internal/app/routers.go` / `internal/services/cache_service.go`
