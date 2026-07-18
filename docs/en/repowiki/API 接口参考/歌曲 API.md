# Song API

This document is based on the following source files:

- `internal/handlers/music.go` -- Song CRUD, playback stream, covers, lyrics, tag writing, file organization, and duplicate detection handlers
- `internal/handlers/hls.go` -- HLS radio reverse proxy handler (playlist rewriting + segment passthrough)
- `internal/app/routers.go` -- Song-related route registration
- `internal/models/models.go` -- Song / BatchDeleteSongsRequest / BatchDeleteSongsResponse
- `internal/models/lyric.go` -- LyricPayload
- `internal/services/song_service.go` -- OrganizeItem / OrganizeResult / CleanResult

## Table of Contents

1. [Overview](#1-overview)
2. [List and Query](#2-list-and-query)
   - [GET /songs -- Paginated List](#21-get-songs----paginated-list)
   - [GET /songs/ids -- ID List](#22-get-songsids----id-list)
   - [GET /songs/duplicates -- Duplicate Detection](#23-get-songsduplicates----duplicate-detection)
   - [GET /songs/facets -- Facet Aggregation](#24-get-songsfacets----facet-aggregation)
3. [Create, Update, Delete](#3-create-update-delete)
   - [GET /songs/{id} -- Get Details](#31-get-songsid----get-details)
   - [PUT /songs/{id} -- Update Song](#32-put-songsid----update-song)
   - [DELETE /songs/{id} -- Delete Song](#33-delete-songsid----delete-song)
   - [POST /songs/remote -- Add Remote Songs](#34-post-songsremote----add-remote-songs)
   - [POST /songs/radio -- Add Radio](#35-post-songsradio----add-radio)
   - [POST /songs/batch-delete -- Batch Delete](#36-post-songsbatch-delete----batch-delete)
   - [POST /songs/clean -- Clean Invalid Songs](#37-post-songsclean----clean-invalid-songs)
   - [Remote Song Metadata Refresh](#38-remote-song-metadata-refresh)
4. [Lyrics and Tags](#4-lyrics-and-tags)
   - [PUT /songs/{id}/lyrics -- Update Lyrics](#41-put-songsidlyrics----update-lyrics)
   - [GET /songs/{id}/lyric -- Get Lyrics](#42-get-songsidlyric----get-lyrics)
   - [PUT /songs/{id}/tags -- Write Audio Tags](#43-put-songsidtags----write-audio-tags)
5. [File Organization](#5-file-organization)
   - [POST /songs/organize -- Batch Organize Files](#51-post-songsorganize----batch-organize-files)
   - [POST /songs/organize/preview -- Preview Batch Organization](#52-post-songsorganizepreview----preview-batch-organization)
6. [Playback and Media](#6-playback-and-media)
   - [POST /songs/{id}/activate -- Activate/Prefetch](#61-post-songsidactivate----activateprefetch)
   - [POST /songs/{id}/played -- Play Event Notification](#62-post-songsidplayed----play-event-notification)
   - [GET /songs/{id}/play -- Playback Stream](#63-get-songsidplay----playback-stream)
   - [GET /songs/{id}/play.m3u8 -- HLS Alias](#64-get-songsidplaym3u8----hls-alias)
   - [GET /songs/{id}/cover -- Cover](#65-get-songsidcover----cover)
7. [HLS Reverse Proxy](#7-hls-reverse-proxy)
   - [GET /songs/{id}/hls/playlist -- Proxy Playlist](#71-get-songsidhlsplaylist----proxy-playlist)
   - [GET /songs/{id}/hls/segment -- Proxy Segment](#72-get-songsidhlssegment----proxy-segment)

---

## 1. Overview

**Section source**: `internal/handlers/music.go`, `internal/app/routers.go`

The song management module is Songloft's core API, covering song CRUD, playback stream distribution, covers/lyrics, audio tag writing, file organization, and duplicate detection. All endpoints require JWT authentication (`BearerAuth`), with the base path `/api/v1`.

**Song types**: `local` (local files), `remote` (external URL / plugin source), `radio` (online broadcast / live stream).

**Song JSON serialization rules** (handled uniformly by `MarshalJSON`):
- `url` -- Unified to `/api/v1/songs/{id}/play` (HLS radio uses `.../play.m3u8`)
- `cover_url` -- `/api/v1/songs/{id}/cover` when a cover exists, otherwise empty
- `lyric_url` -- `/api/v1/songs/{id}/lyric` when lyrics exist, otherwise empty
- `source_url` -- The original stream address, returned only for remote/radio types

**Some read-only fields** (returned with the Song object in details/lists):
- `is_live` -- Whether it is a live stream (semantics apply only to the remote type)
- `is_video` -- bool, whether it contains a real video track (probed by ffprobe during scanning, excluding cover-only images); the client uses this to render video / choose the casting MIME (introduced by the `0024` migration)

**Error response format**: `{"error":"...", "detail":"..."}` (detail is optional).

---

## 2. List and Query

**Section source**: `internal/handlers/music.go`, `internal/database/filters.go`

### 2.1 GET /songs -- Paginated List

**Path:** `/api/v1/songs` | **Authentication:** BearerAuth

Gets the song list, supporting filtering by type, keyword search, path-prefix filtering, and pagination.

**Query parameters:**

| Parameter | Type | Required | Default | Description |
|------|------|------|--------|------|
| `type` | string | No | -- | `local` / `remote` / `radio` |
| `keyword` | string | No | -- | Search keyword (matches title, artist, album) |
| `path_prefix` | string | No | -- | Filter by `file_path` prefix |
| `limit` | int | No | `20` | Items per page, upper limit `100000` |
| `offset` | int | No | `0` | Offset |

**Success response (200):** `{"songs": [Song...], "total": 100, "limit": 20, "offset": 0}`

**Errors:** `500` Failed to get song list / total

---

### 2.2 GET /songs/ids -- ID List

**Path:** `/api/v1/songs/ids` | **Authentication:** BearerAuth

Shares filter conditions with `/songs` (`type`, `keyword`, `path_prefix`), but returns only IDs. Used for batch operation scenarios such as "select all within the current filter range"; not paginated.

**Success response (200):** `{"ids": [1, 2, 3], "total": 3}`

**Errors:** `500` Failed to get song ID list

---

### 2.3 GET /songs/duplicates -- Duplicate Detection

**Path:** `/api/v1/songs/duplicates` | **Authentication:** BearerAuth

Queries groups of local songs with identical content via audio fingerprints (Chromaprint). Fingerprint computation must be completed first (`POST /scan/fingerprints`).

**Success response (200):**

```json
{
  "groups": [
    {
      "fingerprint": "AQADtNIyR...",
      "songs": [
        {"id": 1, "title": "夜曲", "artist": "周杰伦", "album": "十一月的萧邦",
         "duration": 253.5, "file_path": "/music/夜曲.mp3", "format": "mp3",
         "bit_rate": 320, "file_size": 10485760, "cover_url": "/api/v1/songs/1/cover",
         "added_at": "2024-01-01T12:00:00Z"}
      ]
    }
  ],
  "total_groups": 1,
  "total_duplicates": 2
}
```

**Errors:** `500` Failed to query duplicate songs

---

### 2.4 GET /songs/facets -- Facet Aggregation

**Path:** `/api/v1/songs/facets` | **Authentication:** BearerAuth

Aggregates the library by a specified dimension, returning the non-empty values under that dimension, the song count for each, and the cover URL of one representative song, used for the card grid of "browse by category". After obtaining a value, use `/songs?<field>=<value>` to fetch songs under that category.

**Query parameters:**

| Parameter | Type | Required | Default | Description |
|------|------|------|--------|------|
| `field` | string | Yes | -- | Aggregation dimension: `genre` / `artist` / `album` / `language` / `style` / `year` / `decade` (the value for `year`/`decade` is a numeric string; a decade such as `"1990"` means 1990-1999) |
| `keyword` | string | No | -- | Fuzzy search over values |
| `limit` | int | No | `20` | Page size, upper limit `100000` |
| `offset` | int | No | `0` | Pagination offset |
| `sort` | string | No | `count` | Sort dimension: `count` / `name` |
| `order` | string | No | -- | Sort direction: `asc` / `desc` (`count` defaults to `desc`, `name` defaults to `asc`) |

**Success response (200):**

```json
{
  "field": "artist",
  "facets": [
    {"value": "周杰伦", "count": 42, "cover_url": "/api/v1/songs/1/cover"}
  ],
  "total": 128,
  "limit": 20,
  "offset": 0
}
```

`total` is the total number of distinct values for that dimension.

**Errors:** `400` Missing or unsupported `field` | `500` Server error

---

## 3. Create, Update, Delete

**Section source**: `internal/handlers/music.go`, `internal/models/models.go`

### 3.1 GET /songs/{id} -- Get Details

**Path:** `/api/v1/songs/{id}` | **Authentication:** BearerAuth

**Path parameter:** `id` (int, required) -- Song ID

**Success response (200):** The full Song object.

**Errors:** `400` Invalid ID | `404` Song does not exist

---

### 3.2 PUT /songs/{id} -- Update Song

**Path:** `/api/v1/songs/{id}` | **Authentication:** BearerAuth

**Path parameter:** `id` (int, required)

**Request body:**

| Field | Type | Required | Description |
|------|------|------|------|
| `title` | string | Yes | Title (cannot be empty) |
| `artist` | string | No | Artist |
| `album` | string | No | Album |
| `url` | string | Conditionally required | Playback address (required for non-local songs) |
| `cover_url` | string | No | Cover URL |
| `is_live` | bool | No | Whether it is a live stream (applies only to the remote type) |

**Success response (200):** The updated Song object.

**Errors:** `400` Invalid ID / empty title / empty URL | `404` Does not exist | `500` Update failed

---

### 3.3 DELETE /songs/{id} -- Delete Song

**Path:** `/api/v1/songs/{id}` | **Authentication:** BearerAuth

**Path parameter:** `id` (int, required)

**Success response (200):** `{"message": "歌曲已删除"}`

**Errors:** `400` Invalid ID | `500` Delete failed

---

### 3.4 POST /songs/remote -- Add Remote Songs

**Path:** `/api/v1/songs/remote` | **Authentication:** BearerAuth

Batch-adds network songs. Each entry must provide at least a `url` or the `plugin_entry_path` + `source_data` combination.

**Request body** (array):

| Field | Type | Required | Description |
|------|------|------|------|
| `url` | string | Conditional | Direct audio link (for pure external links; leave empty for plugin sources) |
| `title` | string | Yes | Title |
| `artist` | string | No | Artist |
| `album` | string | No | Album |
| `cover_url` | string | No | Cover URL |
| `duration` | number | No | Duration (seconds) |
| `plugin_entry_path` | string | Conditional | Plugin entryPath (e.g., `"subsonic"`) |
| `source_data` | string | Conditional | Plugin source metadata JSON |
| `dedup_key` | string | No | Deduplication key (e.g., `"nas1:001234"`); no deduplication when empty |
| `lyric` | string | No | Lyric content or URL |
| `lyric_source` | string | No | Lyric source type |

**Success response (201):** `{"songs": [Song...], "count": 2}`

**Errors:** `400` Invalid request / empty list / empty title / missing source identifier | `500` Add failed

---

### 3.5 POST /songs/radio -- Add Radio

**Path:** `/api/v1/songs/radio` | **Authentication:** BearerAuth

Batch-adds radio/broadcast stations.

**Request body** (array):

| Field | Type | Required | Description |
|------|------|------|------|
| `url` | string | Yes | Radio stream address |
| `title` | string | Yes | Radio name |
| `artist` | string | No | Description |
| `cover_url` | string | No | Cover URL |

**Success response (201):** `{"songs": [Song...], "count": 1}`

**Errors:** `400` Invalid request / empty list / empty URL or title | `500` Add failed

---

### 3.6 POST /songs/batch-delete -- Batch Delete

**Path:** `/api/v1/songs/batch-delete` | **Authentication:** BearerAuth

**Request body:**

| Field | Type | Required | Description |
|------|------|------|------|
| `ids` | int64[] | Yes | List of song IDs to delete |
| `delete_files` | bool | No | Whether to also delete local audio files (default `false`) |

**Success response (200):** `{"deleted": 3}`

**Errors:** `400` Invalid request / empty ID list | `500` Delete failed

---

### 3.7 POST /songs/clean -- Clean Invalid Songs

**Path:** `/api/v1/songs/clean` | **Authentication:** BearerAuth

Cleans up records of local songs whose files no longer exist or reside in excluded directories, and deletes the associated cover files.

**Success response (200):**

```json
{"message": "清理完成", "total": 5, "file_not_found": 3, "in_excluded_dir": 2}
```

**Errors:** `500` Clean failed

---

### 3.8 Remote Song Metadata Refresh

For all remote songs with missing metadata, probes duration, bit rate, sample rate, format, and tags via ffprobe and backfills them, executed asynchronously.

#### POST /songs/refresh-metadata -- Start Refresh

**Path:** `/api/v1/songs/refresh-metadata` | **Authentication:** BearerAuth

**Success response (202):** `{"status": "started"}`

**Errors:** `409` Already running | `500` Refresher not configured / start failed

#### GET /songs/refresh-metadata/progress -- Query Progress

**Path:** `/api/v1/songs/refresh-metadata/progress` | **Authentication:** BearerAuth

**Success response (200):** `MetadataRefreshProgress` (contains fields such as `status`; returns `{"status": "idle"}` when not configured).

#### POST /songs/refresh-metadata/cancel -- Cancel Refresh

**Path:** `/api/v1/songs/refresh-metadata/cancel` | **Authentication:** BearerAuth

**Success response:** `204 No Content` (idempotent; also returns 204 when there is no task).

---

## 4. Lyrics and Tags

**Section source**: `internal/handlers/music.go`, `internal/models/lyric.go`

### 4.1 PUT /songs/{id}/lyrics -- Update Lyrics

**Path:** `/api/v1/songs/{id}/lyrics` | **Authentication:** BearerAuth

Updates the lyric content and source. The input shape is determined by `lyric_source`:
- `"url"` -- Pass `lyric_remote_url`, fetched lazily at runtime
- Others (`scraped`/`file`/`embedded`/`cached`/`manual`) -- Pass `lyric`/`tlyric`/`rlyric`/`lxlyric`

`lyric_source=manual` marks a user's manual adjustment; a scanner re-scan will not overwrite it.

**Path parameter:** `id` (int, required)

**Request body:**

| Field | Type | Required | Description |
|------|------|------|------|
| `lyric_source` | string | No | Source type |
| `lyric` | string | Conditional | Main lyrics LRC (non-url source) |
| `tlyric` | string | No | Translated lyrics |
| `rlyric` | string | No | Romanized lyrics |
| `lxlyric` | string | No | Word-by-word lyrics |
| `lyric_remote_url` | string | Conditional | Remote lyrics URL (url source) |

**Success response (200):** `{"message": "歌词已更新", "file_write_status": "written"}`

`file_write_status`: `written` / `unchanged` / `skipped` / `failed`

**Errors:** `400` Invalid ID / invalid request | `404` Song does not exist | `500` Update failed

---

### 4.2 GET /songs/{id}/lyric -- Get Lyrics

**Path:** `/api/v1/songs/{id}/lyric` | **Authentication:** BearerAuth

Returns a LyricPayload JSON. Internally dispatches to database unpacking or remote URL fetching based on `lyric_source`.

**Path parameter:** `id` (int, required)

**Success response (200):**

```json
{"lyric": "[00:00.00]夜曲...", "tlyric": "", "rlyric": "", "lxlyric": ""}
```

**Errors:** `400` Invalid ID | `404` Song/lyrics do not exist | `502` Remote fetch failed

---

### 4.3 PUT /songs/{id}/tags -- Write Audio Tags

**Path:** `/api/v1/songs/{id}/tags` | **Authentication:** BearerAuth

Writes metadata to the database and the local audio file tags (local songs only). Non-empty fields overwrite; empty values are preserved. `cover_data` (base64) takes precedence over `cover_url`.

**Path parameter:** `id` (int, required)

**Request body:**

| Field | Type | Required | Description |
|------|------|------|------|
| `title` | string | No | Title |
| `artist` | string | No | Artist |
| `album` | string | No | Album |
| `year` | int | No | Year (overwrites when >0) |
| `genre` | string | No | Genre |
| `lyrics` | string | No | Lyrics LRC (after writing, `lyric_source=manual`) |
| `cover_data` | string | No | Cover base64 (takes precedence) |
| `cover_url` | string | No | Cover URL (written after download) |
| `clear_cover` | bool | No | Explicitly clears the cover |

**Success response (200):** `{"song": Song, "file_write": "written"}`

`file_write`: `written` / `unchanged` / `skipped` / `failed` / `unsupported`

**Errors:** `400` Invalid ID / non-local song / invalid base64 | `404` Does not exist | `500` Update failed

---

## 5. File Organization

**Section source**: `internal/handlers/music.go`, `internal/services/song_service.go`

### 5.1 POST /songs/organize -- Batch Organize Files

**Path:** `/api/v1/songs/organize` | **Authentication:** BearerAuth

Batch-moves/renames local song files. `target_path` is a path relative to `music_path` (including directory and file name), and the extension must match the original file.

**Request body** (array):

| Field | Type | Required | Description |
|------|------|------|------|
| `id` | int64 | Yes | Song ID |
| `target_path` | string | Yes | Target path (relative to `music_path`) |

**Success response (200):**

```json
[
  {"id": 1, "status": "ok", "file_path": "/music/周杰伦/夜曲.mp3"},
  {"id": 2, "status": "error", "error": "not a local song"}
]
```

**Errors:** `400` Invalid request / empty list / `music_path` not set | `500` Not configured

---

### 5.2 POST /songs/organize/preview -- Preview Batch Organization

**Path:** `/api/v1/songs/organize/preview` | **Authentication:** BearerAuth

Dry-run preview of directory organization changes; **does not move any files or modify the database**. Returns each item's `old_path`→`new_path` and status. `target_path` is a path relative to `music_path` (`music_path` is obtained by the server itself). CUE songs return `skip`; a target that already exists or a name collision within the batch returns `conflict`.

**Request body** (array): Same as `/songs/organize` (`id` + `target_path`).

**Success response (200):** An array of `OrganizePreviewResult`, each item containing `status` (`ok` / `conflict` / `skip` / `error`), `old_path`, and `new_path`.

**Errors:** `400` Invalid request / empty list

---

## 6. Playback and Media

**Section source**: `internal/handlers/music.go`

### 6.1 POST /songs/{id}/activate -- Activate/Prefetch

**Path:** `/api/v1/songs/{id}/activate` | **Authentication:** BearerAuth

Called by the client before switching songs. The backend cancels in-progress work for other songs in the same session (prefetch / transcode / reassign), yielding the plugin worker and transcode semaphore to the new song. Idempotent; other client sessions are not affected.

**Path parameter:** `id` (int, required)

**Success response:** `204 No Content`

**Errors:** `400` Invalid song_id

---

### 6.2 POST /songs/{id}/played -- Play Event Notification

**Path:** `/api/v1/songs/{id}/played` | **Authentication:** BearerAuth

Called by the client when a song starts playing, finishes playing, or is skipped; the backend broadcasts the event to JS plugins that have subscribed to play events (registered via `songloft.events.onPlayEvent`).

**Path parameter:** `id` (int, required)

**Query parameters:**

| Parameter | Type | Required | Description |
|------|------|------|------|
| `source` | string | No | Caller source identifier, e.g., `songloft-player` (official client), `miot` (Xiao AI speaker plugin) |
| `type` | string | No | Event type: `play` (start playing) / `finish` (finished playing, default) / `skip` (user skipped) |

**Success response:** `204 No Content`

**Errors:** `400` Invalid song ID / illegal event type | `404` Song does not exist

---

### 6.3 GET /songs/{id}/play -- Playback Stream

**Path:** `/api/v1/songs/{id}/play` | **Method:** `GET` / `HEAD` | **Authentication:** BearerAuth

Streams audio by song ID, internally dispatching automatically by `song.type`:

| Type | Behavior |
|------|------|
| `local` | `ServeFile` (supports Range/seek) |
| `remote` (plugin) | Returned after CacheService download caching |
| `remote` (external link) | Directly proxied and forwarded |
| `radio` (HLS) | Reverse proxy or 302 (depending on the hls-proxy switch) |
| `radio` (non-HLS) | Directly proxies the upstream stream |

**Path parameter:** `id` (int, required)

**Query parameters:**

| Parameter | Type | Description |
|------|------|------|
| `format` | string | Target format (e.g., `opus`); when different, goes through ffmpeg transcoding |
| `prefetch` | string | When `"1"`, prewarms asynchronously and returns 202 immediately |

**Success:** `200` Audio stream | `202` Prefetch triggered | `206` Range partial content | `302` HLS redirect

**Errors:** `400` Invalid ID | `404` Does not exist | `502` Source unavailable

---

### 6.4 GET /songs/{id}/play.m3u8 -- HLS Alias

**Path:** `/api/v1/songs/{id}/play.m3u8` | **Method:** `GET` / `HEAD` | **Authentication:** BearerAuth

An HLS-radio-specific alias that shares the handler with `/play`. Ending the URL with `.m3u8` makes ExoPlayer/AVPlayer correctly select the HlsMediaSource. The client automatically obtains the correct URL via `Song.PlaybackURL()`. Parameters and responses are the same as [6.3](#63-get-songsidplay----playback-stream).

---

### 6.5 GET /songs/{id}/cover -- Cover

**Path:** `/api/v1/songs/{id}/cover` | **Authentication:** BearerAuth

Gets the cover image. Returns the local cover first, and proxies the external `CoverURL` when it does not exist.

**Path parameter:** `id` (int, required)

**Success response (200):** Image binary, with `Content-Type` set automatically by extension (jpeg/png/gif/bmp/webp), `Cache-Control: public, max-age=31536000`.

**Errors:** `400` Invalid ID | `404` Song/cover does not exist | `500` Read failed

---

## 7. HLS Reverse Proxy

**Section source**: `internal/handlers/hls.go`

The HLS reverse proxy takes effect after the `hls_proxy` switch is enabled (`PUT /settings/hls-proxy`). Flow: `serveRadio` -> `ServeProxy` fetches the upstream m3u8 and rewrites URIs to local relative paths -> the player calls back `playlist`/`segment` to fetch sub-layer resources. Each entry performs a same-origin check + `IsHostnameAllowed` SSRF protection; non-same-origin URLs are left unchanged.

### 7.1 GET /songs/{id}/hls/playlist -- Proxy Playlist

**Path:** `/api/v1/songs/{id}/hls/playlist` | **Method:** `GET` / `HEAD` | **Authentication:** BearerAuth

Reverse-proxies the HLS sub-layer m3u8. Fetch upstream -> same-origin check -> rewrite URIs -> write back. Upstream errors are passed through; the playlist size limit is 1 MB.

**Parameters:** `id` (path, int) | `u` (query, string, required, base64url-encoded upstream URL) | `access_token` (query, string, optional)

**Success (200):** Rewritten m3u8 text, `Content-Type: application/vnd.apple.mpegurl`, `Cache-Control: no-store`

**Errors:** `400` Invalid ID / missing u | `403` Non-same-origin / host not allowed | `404` Song does not exist | `502` Upstream unavailable

---

### 7.2 GET /songs/{id}/hls/segment -- Proxy Segment

**Path:** `/api/v1/songs/{id}/hls/segment` | **Method:** `GET` / `HEAD` | **Authentication:** BearerAuth

Reverse-proxies audio segments/key/init segments. Passes through Range and upstream response headers.

**Parameters:** `id` (path, int) | `u` (query, string, required, base64url-encoded upstream URL) | `access_token` (query, string, optional)

**Success:** `200` Segment content | `206` Range partial content. `Cache-Control: no-store`

**Errors:** `400` Invalid ID / missing u | `403` Non-same-origin / host not allowed | `404` Song does not exist | `502` Upstream unavailable
