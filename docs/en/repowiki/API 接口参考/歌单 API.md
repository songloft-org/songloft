# Playlist API

This document is based on the following source files:

- `internal/handlers/playlist.go` -- Playlist CRUD, song management, reordering, and cover handlers
- `internal/handlers/backup.go` -- Playlist backup export and import handlers
- `internal/app/routers.go` -- Playlist route registration (`/api/v1/playlists` route group)
- `internal/models/models.go` -- Playlist / BatchDeletePlaylistsRequest/Response structs
- `internal/models/backup.go` -- BackupData / ImportResult structs

## Table of Contents

1. [Playlist CRUD](#1-playlist-crud)
2. [Playlist Reordering](#2-playlist-reordering)
3. [Songs in a Playlist](#3-songs-in-a-playlist)
4. [Playlist Auxiliary Operations](#4-playlist-auxiliary-operations)
5. [Playlist Backup and Restore](#5-playlist-backup-and-restore)

---

## 1. Playlist CRUD

**Section source**: `internal/handlers/playlist.go`, `internal/app/routers.go`

### GET /api/v1/playlists

Gets the playlist list, supporting filtering by type and pagination.

- **Authentication**: Bearer Token
- **Query parameters**:

| Parameter | Type | Required | Description |
|------|------|------|------|
| `type` | string | No | Playlist type: `normal` / `radio` |
| `limit` | int | No | Items per page, default 20 |
| `offset` | int | No | Offset, default 0 |

- **200**: `{"playlists": [Playlist], "total": 5, "limit": 20, "offset": 0}`
- **500**: Server error

### POST /api/v1/playlists

Creates a new playlist.

- **Authentication**: Bearer Token
- **Request body**:

| Field | Type | Required | Description |
|------|------|------|------|
| `name` | string | Yes | Playlist name |
| `type` | string | Yes | Playlist type: `normal` or `radio` |
| `description` | string | No | Playlist description |
| `cover_url` | string | No | Cover image URL |

- **201**: Returns a `Playlist` object
- **400**: Bad request data | **409**: A playlist with the same name already exists | **500**: Creation failed

### GET /api/v1/playlists/{id}

Gets playlist details by ID.

- **Authentication**: Bearer Token
- **Path parameter**: `id` (int, playlist ID)
- **200**: Returns a `Playlist` object
- **400**: Invalid ID | **404**: Playlist does not exist

### PUT /api/v1/playlists/{id}

Updates playlist information (modifying `type` is not supported).

- **Authentication**: Bearer Token
- **Request body**:

| Field | Type | Required | Description |
|------|------|------|------|
| `name` | string | Yes | Playlist name |
| `description` | string | No | Playlist description |
| `cover_path` | *string | No | Local cover path; pass null to leave unchanged |
| `cover_url` | *string | No | Cover URL; pass null to leave unchanged |

- **200**: Returns the updated `Playlist`
- **400**: Bad request data | **404**: Playlist does not exist | **409**: Name conflict | **500**: Update failed

### DELETE /api/v1/playlists/{id}

Deletes a playlist.

- **Authentication**: Bearer Token
- **Path parameter**: `id` (int)
- **200**: `{"message": "歌单已删除"}`
- **400**: Invalid ID | **500**: Delete failed

### POST /api/v1/playlists/batch-delete

Batch-deletes playlists. Built-in playlists (id=1 Favorites, id=2 Radio Favorites, with the `built_in` label) are automatically skipped.

- **Authentication**: Bearer Token
- **Request body**: `{"ids": [1, 2, 3]}` (int64 array, required)
- **200**: `{"deleted": 3}`
- **400**: Empty ID list | **500**: Delete failed

---

## 2. Playlist Reordering

**Section source**: `internal/handlers/playlist.go`

### PUT /api/v1/playlists/reorder

Reorders the playlist list according to the order of the IDs passed in.

- **Authentication**: Bearer Token
- **Request body**: `{"playlist_ids": [3, 1, 2]}` (int64 array, required)
- **200**: `{"message": "歌单已重新排序"}`
- **400**: Empty ID list | **500**: Reordering failed

---

## 3. Songs in a Playlist

**Section source**: `internal/handlers/playlist.go`, `internal/app/routers.go`

### GET /api/v1/playlists/{id}/songs

Gets the list of songs in a playlist, supporting pagination.

- **Authentication**: Bearer Token
- **Path parameter**: `id` (int, playlist ID)
- **Query parameters**: `limit` (int, default 20), `offset` (int, default 0)
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

- **400**: Invalid ID | **500**: Retrieval failed

### POST /api/v1/playlists/{id}/songs

Batch-adds songs to a playlist. Songs that already exist are automatically skipped. Normal playlists accept only `local`/`remote` types; radio playlists accept only the `radio` type.

- **Authentication**: Bearer Token
- **Path parameter**: `id` (int, playlist ID)
- **Request body**: `{"song_ids": [1, 2, 3]}` (int64 array, required)
- **200**: `{"message": "歌曲已添加到歌单", "added": 3, "skipped": 1}`
- **400**: Empty song_ids | **500**: Add failed

### PUT /api/v1/playlists/{id}/songs/reorder

Reorders the songs in a playlist.

- **Authentication**: Bearer Token
- **Path parameter**: `id` (int, playlist ID)
- **Request body**: `{"song_ids": [3, 1, 2]}` (int64 array)
- **200**: `{"message": "歌单歌曲已重新排序"}`
- **400**: Bad request data | **500**: Reordering failed

### DELETE /api/v1/playlists/{id}/songs/{songId}

Removes a song from a playlist.

- **Authentication**: Bearer Token
- **Path parameters**: `id` (int, playlist ID), `songId` (int, song ID)
- **200**: `{"message": "歌曲已从歌单移除"}`
- **400**: Invalid ID | **500**: Remove failed

---

## 4. Playlist Auxiliary Operations

**Section source**: `internal/handlers/playlist.go`

### POST /api/v1/playlists/{id}/touch

Updates the playlist's `updated_at` field, recording the last played time. Called by the frontend when playing a playlist, to implement "recently played" sorting.

- **Authentication**: Bearer Token
- **Path parameter**: `id` (int)
- **200**: `{"message": "歌单播放时间已更新"}`
- **400**: Invalid ID | **500**: Update failed

### POST /api/v1/playlists/{id}/cover

Uploads a local image as the playlist cover (multipart/form-data).

- **Authentication**: Bearer Token
- **Path parameter**: `id` (int)
- **Form field**: `file` (file, required)
- **File limit**: Maximum 10MB
- **Supported formats**: jpg, jpeg, png, gif, bmp, webp
- **200**: Returns the updated `Playlist` object
- **400**: Unsupported format or parsing failed | **500**: Upload failed

### GET /api/v1/playlists/{id}/cover

Returns the playlist cover image. Fallback strategy: local cover file > remote cover URL proxy > covers of the first 20 songs in the playlist.

- **Authentication**: Bearer Token
- **Path parameter**: `id` (int)
- **200**: Image binary content (`Content-Type` set by extension, `Cache-Control: public, max-age=31536000`)
- **404**: Cover does not exist (playlist does not exist or has no cover source)

---

## 5. Playlist Backup and Restore

**Section source**: `internal/handlers/backup.go`, `internal/models/backup.go`

### GET /api/v1/playlists/export

Exports all playlists (including track associations) as a JSON file. The `Content-Disposition` response header triggers a browser download, with the file name format `songloft-backup-YYYYMMDD.json`.

- **Authentication**: Bearer Token
- **200**: A BackupData JSON file

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

- **500**: Export failed

### POST /api/v1/playlists/import

Restores playlists from a JSON backup file (multipart/form-data). Existing playlists are merged by name, and tracks are deduplicated by content matching.

- **Authentication**: Bearer Token
- **Form field**: `file` (file, required, JSON backup file)
- **File limit**: Maximum 32MB
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

| Field | Description |
|------|------|
| `playlists_created` | Number of newly created playlists |
| `playlists_merged` | Number merged into existing playlists |
| `songs_created` | Number of newly created songs |
| `songs_matched` | Number matched to existing songs |
| `songs_skipped` | Number of skipped songs |

- **400**: Format error / missing file / invalid file | **500**: Import failed
