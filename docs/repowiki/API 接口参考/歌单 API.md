# 歌单 API

本文档基于以下源文件编写：

- `internal/handlers/playlist.go` -- 歌单 CRUD、歌曲管理、排序、封面处理器
- `internal/handlers/backup.go` -- 歌单备份导出与导入处理器
- `internal/app/routers.go` -- 歌单路由注册（`/api/v1/playlists` 路由组）
- `internal/models/models.go` -- Playlist / BatchDeletePlaylistsRequest/Response 结构体
- `internal/models/backup.go` -- BackupData / ImportResult 结构体

## 目录

1. [歌单 CRUD](#1-歌单-crud)
2. [歌单排序](#2-歌单排序)
3. [歌单内歌曲管理](#3-歌单内歌曲管理)
4. [歌单辅助操作](#4-歌单辅助操作)
5. [歌单备份与还原](#5-歌单备份与还原)

---

## 1. 歌单 CRUD

**章节来源**: `internal/handlers/playlist.go`、`internal/app/routers.go`

### GET /api/v1/playlists

获取歌单列表，支持按类型过滤和分页。

- **认证**: Bearer Token
- **查询参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 否 | 歌单类型：`normal` / `radio` |
| `limit` | int | 否 | 每页数量，默认 20 |
| `offset` | int | 否 | 偏移量，默认 0 |

- **200**: `{"playlists": [Playlist], "total": 5, "limit": 20, "offset": 0}`
- **500**: 服务器错误

### POST /api/v1/playlists

创建新歌单。

- **认证**: Bearer Token
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 歌单名称 |
| `type` | string | 是 | 歌单类型：`normal` 或 `radio` |
| `description` | string | 否 | 歌单描述 |
| `cover_url` | string | 否 | 封面图片 URL |

- **201**: 返回 `Playlist` 对象
- **400**: 请求数据错误 | **409**: 同名歌单已存在 | **500**: 创建失败

### GET /api/v1/playlists/{id}

根据 ID 获取歌单详情。

- **认证**: Bearer Token
- **路径参数**: `id`（int，歌单 ID）
- **200**: 返回 `Playlist` 对象
- **400**: 无效 ID | **404**: 歌单不存在

### PUT /api/v1/playlists/{id}

更新歌单信息（不支持修改 `type`）。

- **认证**: Bearer Token
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 歌单名称 |
| `description` | string | 否 | 歌单描述 |
| `cover_path` | *string | 否 | 封面本地路径，传 null 不修改 |
| `cover_url` | *string | 否 | 封面 URL，传 null 不修改 |

- **200**: 返回更新后的 `Playlist`
- **400**: 请求数据错误 | **404**: 歌单不存在 | **409**: 同名冲突 | **500**: 更新失败

### DELETE /api/v1/playlists/{id}

删除歌单。

- **认证**: Bearer Token
- **路径参数**: `id`（int）
- **200**: `{"message": "歌单已删除"}`
- **400**: 无效 ID | **500**: 删除失败

### POST /api/v1/playlists/batch-delete

批量删除歌单。内置歌单（id=1 收藏、id=2 电台收藏，带 `built_in` 标签）会被自动跳过。

- **认证**: Bearer Token
- **请求体**: `{"ids": [1, 2, 3]}`（int64 数组，必填）
- **200**: `{"deleted": 3}`
- **400**: ID 列表为空 | **500**: 删除失败

---

## 2. 歌单排序

**章节来源**: `internal/handlers/playlist.go`

### PUT /api/v1/playlists/reorder

按传入的 ID 顺序重新排列歌单列表。

- **认证**: Bearer Token
- **请求体**: `{"playlist_ids": [3, 1, 2]}`（int64 数组，必填）
- **200**: `{"message": "歌单已重新排序"}`
- **400**: ID 列表为空 | **500**: 排序失败

---

## 3. 歌单内歌曲管理

**章节来源**: `internal/handlers/playlist.go`、`internal/app/routers.go`

### GET /api/v1/playlists/{id}/songs

获取歌单中的歌曲列表，支持分页。

- **认证**: Bearer Token
- **路径参数**: `id`（int，歌单 ID）
- **查询参数**: `limit`（int，默认 20）、`offset`（int，默认 0）
- **200**:

```json
{
  "songs": [
    {
      "id": 1, "type": "local", "title": "夜曲",
      "artist": "周杰伦", "url": "/api/v1/songs/1/play",
      "cover_url": "/api/v1/songs/1/cover", "duration": 253.5
    }
  ],
  "total": 10, "limit": 20, "offset": 0
}
```

- **400**: 无效 ID | **500**: 获取失败

### POST /api/v1/playlists/{id}/songs

批量添加歌曲到歌单。已存在的歌曲自动跳过。普通歌单仅接受 `local`/`remote` 类型，电台歌单仅接受 `radio` 类型。

- **认证**: Bearer Token
- **路径参数**: `id`（int，歌单 ID）
- **请求体**: `{"song_ids": [1, 2, 3]}`（int64 数组，必填）
- **200**: `{"message": "歌曲已添加到歌单", "added": 3, "skipped": 1}`
- **400**: song_ids 为空 | **500**: 添加失败

### PUT /api/v1/playlists/{id}/songs/reorder

重新排序歌单中的歌曲。

- **认证**: Bearer Token
- **路径参数**: `id`（int，歌单 ID）
- **请求体**: `{"song_ids": [3, 1, 2]}`（int64 数组）
- **200**: `{"message": "歌单歌曲已重新排序"}`
- **400**: 请求数据错误 | **500**: 排序失败

### DELETE /api/v1/playlists/{id}/songs/{songId}

从歌单移除一首歌曲。

- **认证**: Bearer Token
- **路径参数**: `id`（int，歌单 ID）、`songId`（int，歌曲 ID）
- **200**: `{"message": "歌曲已从歌单移除"}`
- **400**: 无效 ID | **500**: 移除失败

---

## 4. 歌单辅助操作

**章节来源**: `internal/handlers/playlist.go`

### POST /api/v1/playlists/{id}/touch

更新歌单的 `updated_at` 字段，记录最后播放时间。前端在播放歌单时调用，实现"最近播放"排序。

- **认证**: Bearer Token
- **路径参数**: `id`（int）
- **200**: `{"message": "歌单播放时间已更新"}`
- **400**: 无效 ID | **500**: 更新失败

### POST /api/v1/playlists/{id}/cover

上传本地图片作为歌单封面（multipart/form-data）。

- **认证**: Bearer Token
- **路径参数**: `id`（int）
- **表单字段**: `file`（file，必填）
- **文件限制**: 最大 10MB
- **支持格式**: jpg、jpeg、png、gif、bmp、webp
- **200**: 返回更新后的 `Playlist` 对象
- **400**: 格式不支持或解析失败 | **500**: 上传失败

### GET /api/v1/playlists/{id}/cover

返回歌单封面图片。回退策略：本地封面文件 > 远程封面 URL 代理 > 歌单内前 20 首歌曲的封面。

- **认证**: Bearer Token
- **路径参数**: `id`（int）
- **200**: 图片二进制内容（`Content-Type` 按扩展名设置，`Cache-Control: public, max-age=31536000`）
- **404**: 封面不存在（歌单不存在或无任何封面来源）

---

## 5. 歌单备份与还原

**章节来源**: `internal/handlers/backup.go`、`internal/models/backup.go`

### GET /api/v1/playlists/export

导出所有歌单（含曲目关联）为 JSON 文件。响应头 `Content-Disposition` 触发浏览器下载，文件名格式 `songloft-backup-YYYYMMDD.json`。

- **认证**: Bearer Token
- **200**: BackupData JSON 文件

```json
{
  "version": 1,
  "exported_at": "2024-06-01T12:00:00Z",
  "playlists": [{
    "name": "我的最爱", "type": "normal", "description": "收藏", "labels": [],
    "songs": [
      {"type":"local", "title":"夜曲", "artist":"周杰伦", "album":"十一月的萧邦",
       "duration":253.5, "file_path":"/music/夜曲.mp3", "format":"mp3"}
    ]
  }]
}
```

- **500**: 导出失败

### POST /api/v1/playlists/import

从 JSON 备份文件还原歌单（multipart/form-data）。已存在的歌单按名称合并，曲目按内容匹配去重。

- **认证**: Bearer Token
- **表单字段**: `file`（file，必填，JSON 备份文件）
- **文件限制**: 最大 32MB
- **200**:

```json
{
  "playlists_created": 2,
  "playlists_merged": 1,
  "songs_created": 15,
  "songs_matched": 8,
  "songs_skipped": 3
}
```

| 字段 | 说明 |
|------|------|
| `playlists_created` | 新建的歌单数 |
| `playlists_merged` | 合并到已有歌单的数 |
| `songs_created` | 新创建的歌曲数 |
| `songs_matched` | 匹配到已有歌曲的数 |
| `songs_skipped` | 跳过的歌曲数 |

- **400**: 格式错误/缺少文件/文件无效 | **500**: 导入失败
