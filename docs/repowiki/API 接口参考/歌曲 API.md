# 歌曲 API

本文档基于以下源文件编写：

- `internal/handlers/music.go` -- 歌曲 CRUD、播放流、封面、歌词、标签写入、文件整理、重复检测处理器
- `internal/handlers/hls.go` -- HLS 电台反代处理器（播放列表改写 + 切片透传）
- `internal/app/routers.go` -- 歌曲相关路由注册
- `internal/models/models.go` -- Song / BatchDeleteSongsRequest / BatchDeleteSongsResponse
- `internal/models/lyric.go` -- LyricPayload
- `internal/services/song_service.go` -- OrganizeItem / OrganizeResult / CleanResult

## 目录

1. [概述](#1-概述)
2. [列表与查询](#2-列表与查询)
   - [GET /songs -- 分页列表](#21-get-songs----分页列表)
   - [GET /songs/ids -- ID 列表](#22-get-songsids----id-列表)
   - [GET /songs/duplicates -- 重复检测](#23-get-songsduplicates----重复检测)
   - [GET /songs/facets -- 标签分类聚合](#24-get-songsfacets----标签分类聚合)
3. [增删改](#3-增删改)
   - [GET /songs/{id} -- 获取详情](#31-get-songsid----获取详情)
   - [PUT /songs/{id} -- 更新歌曲](#32-put-songsid----更新歌曲)
   - [DELETE /songs/{id} -- 删除歌曲](#33-delete-songsid----删除歌曲)
   - [POST /songs/remote -- 添加远程歌曲](#34-post-songsremote----添加远程歌曲)
   - [POST /songs/radio -- 添加电台](#35-post-songsradio----添加电台)
   - [POST /songs/batch-delete -- 批量删除](#36-post-songsbatch-delete----批量删除)
   - [POST /songs/clean -- 清理无效歌曲](#37-post-songsclean----清理无效歌曲)
   - [远程歌曲元数据刷新](#38-远程歌曲元数据刷新)
4. [歌词与标签](#4-歌词与标签)
   - [PUT /songs/{id}/lyrics -- 更新歌词](#41-put-songsidlyrics----更新歌词)
   - [GET /songs/{id}/lyric -- 获取歌词](#42-get-songsidlyric----获取歌词)
   - [PUT /songs/{id}/tags -- 写入音频标签](#43-put-songsidtags----写入音频标签)
5. [文件整理](#5-文件整理)
   - [POST /songs/organize -- 批量整理文件](#51-post-songsorganize----批量整理文件)
   - [POST /songs/organize/preview -- 预览批量整理](#52-post-songsorganizepreview----预览批量整理)
6. [播放与媒体](#6-播放与媒体)
   - [POST /songs/{id}/activate -- 激活/预取](#61-post-songsidactivate----激活预取)
   - [POST /songs/{id}/played -- 播放事件通知](#62-post-songsidplayed----播放事件通知)
   - [GET /songs/{id}/play -- 播放流](#63-get-songsidplay----播放流)
   - [GET /songs/{id}/play.m3u8 -- HLS 别名](#64-get-songsidplaym3u8----hls-别名)
   - [GET /songs/{id}/cover -- 封面](#65-get-songsidcover----封面)
7. [HLS 反向代理](#7-hls-反向代理)
   - [GET /songs/{id}/hls/playlist -- 代理播放列表](#71-get-songsidhlsplaylist----代理播放列表)
   - [GET /songs/{id}/hls/segment -- 代理切片](#72-get-songsidhlssegment----代理切片)

---

## 1. 概述

**章节来源**: `internal/handlers/music.go`、`internal/app/routers.go`

歌曲管理模块是 Songloft 核心 API，覆盖歌曲增删改查、播放流分发、封面/歌词、音频标签写入、文件整理和重复检测。所有端点均需 JWT 认证（`BearerAuth`），基础路径 `/api/v1`。

**歌曲类型**: `local`（本地文件）、`remote`（外部 URL / 插件音源）、`radio`（在线广播/直播流）。

**Song JSON 序列化规则**（`MarshalJSON` 统一处理）:
- `url` -- 统一为 `/api/v1/songs/{id}/play`（HLS 电台为 `.../play.m3u8`）
- `cover_url` -- 有封面时 `/api/v1/songs/{id}/cover`，否则空
- `lyric_url` -- 有歌词时 `/api/v1/songs/{id}/lyric`，否则空
- `source_url` -- 仅 remote/radio 类型返回原始流地址

**部分只读字段**（Song 对象随详情/列表返回）:
- `is_live` -- 是否直播流（仅 remote 类型语义生效）
- `is_video` -- bool，是否含真实视频轨（扫描时 ffprobe 探测，排除仅封面图片）；客户端据此渲染画面 / 选择投屏 MIME（`0024` 迁移引入）

**错误响应格式**: `{"error":"...", "detail":"..."}`（detail 可选）。

---

## 2. 列表与查询

**章节来源**: `internal/handlers/music.go`、`internal/database/filters.go`

### 2.1 GET /songs -- 分页列表

**路径:** `/api/v1/songs` | **认证:** BearerAuth

获取歌曲列表，支持按类型过滤、关键词搜索、路径前缀过滤和分页。

**查询参数:**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `type` | string | 否 | -- | `local` / `remote` / `radio` |
| `keyword` | string | 否 | -- | 搜索关键词（匹配标题、艺术家、专辑） |
| `path_prefix` | string | 否 | -- | 按 `file_path` 前缀过滤 |
| `limit` | int | 否 | `20` | 每页数量，上限 `100000` |
| `offset` | int | 否 | `0` | 偏移量 |

**成功响应 (200):** `{"songs": [Song...], "total": 100, "limit": 20, "offset": 0}`

**错误:** `500` 获取歌曲列表/总数失败

---

### 2.2 GET /songs/ids -- ID 列表

**路径:** `/api/v1/songs/ids` | **认证:** BearerAuth

与 `/songs` 共享过滤条件（`type`、`keyword`、`path_prefix`），仅返回 ID。用于「全选当前筛选范围」等批量操作场景，不分页。

**成功响应 (200):** `{"ids": [1, 2, 3], "total": 3}`

**错误:** `500` 获取歌曲 ID 列表失败

---

### 2.3 GET /songs/duplicates -- 重复检测

**路径:** `/api/v1/songs/duplicates` | **认证:** BearerAuth

通过音频指纹（Chromaprint）查询本地歌曲中内容相同的重复组。需先完成指纹计算（`POST /scan/fingerprints`）。

**成功响应 (200):**

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

**错误:** `500` 查询重复歌曲失败

---

### 2.4 GET /songs/facets -- 标签分类聚合

**路径:** `/api/v1/songs/facets` | **认证:** BearerAuth

按指定维度聚合曲库，返回该维度下非空取值、各自的歌曲数量及一首代表歌曲的封面 URL，用于「分类浏览」的卡片网格。取到某取值后可用 `/songs?<field>=<value>` 拉取该分类下歌曲。

**查询参数:**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `field` | string | 是 | -- | 聚合维度：`genre` / `artist` / `album` / `language` / `style` / `year` / `decade`（`year`/`decade` 的 value 为数字字符串，年代如 `"1990"` 表示 1990-1999） |
| `keyword` | string | 否 | -- | 对取值模糊搜索 |
| `limit` | int | 否 | `20` | 分页大小，上限 `100000` |
| `offset` | int | 否 | `0` | 分页偏移 |
| `sort` | string | 否 | `count` | 排序维度：`count` / `name` |
| `order` | string | 否 | -- | 排序方向：`asc` / `desc`（`count` 缺省 `desc`，`name` 缺省 `asc`） |

**成功响应 (200):**

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

`total` 为该维度去重取值总数。

**错误:** `400` 缺少或不支持的 `field` | `500` 服务器错误

---

## 3. 增删改

**章节来源**: `internal/handlers/music.go`、`internal/models/models.go`

### 3.1 GET /songs/{id} -- 获取详情

**路径:** `/api/v1/songs/{id}` | **认证:** BearerAuth

**路径参数:** `id` (int, 必填) -- 歌曲 ID

**成功响应 (200):** 完整 Song 对象。

**错误:** `400` 无效 ID | `404` 歌曲不存在

---

### 3.2 PUT /songs/{id} -- 更新歌曲

**路径:** `/api/v1/songs/{id}` | **认证:** BearerAuth

**路径参数:** `id` (int, 必填)

**请求体:**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `title` | string | 是 | 标题（不能为空） |
| `artist` | string | 否 | 艺术家 |
| `album` | string | 否 | 专辑 |
| `url` | string | 条件必填 | 播放地址（非本地歌曲必填） |
| `cover_url` | string | 否 | 封面 URL |
| `is_live` | bool | 否 | 是否直播流（仅 remote 类型生效） |

**成功响应 (200):** 更新后的 Song 对象。

**错误:** `400` 无效 ID / 标题为空 / URL 为空 | `404` 不存在 | `500` 更新失败

---

### 3.3 DELETE /songs/{id} -- 删除歌曲

**路径:** `/api/v1/songs/{id}` | **认证:** BearerAuth

**路径参数:** `id` (int, 必填)

**成功响应 (200):** `{"message": "歌曲已删除"}`

**错误:** `400` 无效 ID | `500` 删除失败

---

### 3.4 POST /songs/remote -- 添加远程歌曲

**路径:** `/api/v1/songs/remote` | **认证:** BearerAuth

批量添加网络歌曲。每条至少提供 `url` 或 `plugin_entry_path` + `source_data` 组合。

**请求体** (数组):

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | 条件 | 音频直链（纯外链用；插件来源留空） |
| `title` | string | 是 | 标题 |
| `artist` | string | 否 | 艺术家 |
| `album` | string | 否 | 专辑 |
| `cover_url` | string | 否 | 封面 URL |
| `duration` | number | 否 | 时长（秒） |
| `plugin_entry_path` | string | 条件 | 插件 entryPath（如 `"subsonic"`） |
| `source_data` | string | 条件 | 插件音源元数据 JSON |
| `dedup_key` | string | 否 | 去重 key（如 `"nas1:001234"`）；空时不去重 |
| `lyric` | string | 否 | 歌词内容或 URL |
| `lyric_source` | string | 否 | 歌词来源类型 |

**成功响应 (201):** `{"songs": [Song...], "count": 2}`

**错误:** `400` 请求无效 / 列表为空 / 标题为空 / 缺少音源标识 | `500` 添加失败

---

### 3.5 POST /songs/radio -- 添加电台

**路径:** `/api/v1/songs/radio` | **认证:** BearerAuth

批量添加电台/广播。

**请求体** (数组):

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 电台流地址 |
| `title` | string | 是 | 电台名称 |
| `artist` | string | 否 | 描述 |
| `cover_url` | string | 否 | 封面 URL |

**成功响应 (201):** `{"songs": [Song...], "count": 1}`

**错误:** `400` 请求无效 / 列表为空 / URL 或标题为空 | `500` 添加失败

---

### 3.6 POST /songs/batch-delete -- 批量删除

**路径:** `/api/v1/songs/batch-delete` | **认证:** BearerAuth

**请求体:**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | int64[] | 是 | 要删除的歌曲 ID 列表 |
| `delete_files` | bool | 否 | 是否同步删除本地音频文件（默认 `false`） |

**成功响应 (200):** `{"deleted": 3}`

**错误:** `400` 请求无效 / ID 列表为空 | `500` 删除失败

---

### 3.7 POST /songs/clean -- 清理无效歌曲

**路径:** `/api/v1/songs/clean` | **认证:** BearerAuth

清理本地歌曲中文件已不存在或位于排除目录中的记录，同时删除关联封面文件。

**成功响应 (200):**

```json
{"message": "清理完成", "total": 5, "file_not_found": 3, "in_excluded_dir": 2}
```

**错误:** `500` 清理失败

---

### 3.8 远程歌曲元数据刷新

对所有元数据缺失的远程歌曲，通过 ffprobe 探测时长、比特率、采样率、格式及标签并回填，异步执行。

#### POST /songs/refresh-metadata -- 启动刷新

**路径:** `/api/v1/songs/refresh-metadata` | **认证:** BearerAuth

**成功响应 (202):** `{"status": "started"}`

**错误:** `409` 已在运行 | `500` 未配置刷新器 / 启动失败

#### GET /songs/refresh-metadata/progress -- 查询进度

**路径:** `/api/v1/songs/refresh-metadata/progress` | **认证:** BearerAuth

**成功响应 (200):** `MetadataRefreshProgress`（含 `status` 等字段；未配置时返回 `{"status": "idle"}`）。

#### POST /songs/refresh-metadata/cancel -- 取消刷新

**路径:** `/api/v1/songs/refresh-metadata/cancel` | **认证:** BearerAuth

**成功响应:** `204 No Content`（幂等，无任务时也返回 204）。

---

## 4. 歌词与标签

**章节来源**: `internal/handlers/music.go`、`internal/models/lyric.go`

### 4.1 PUT /songs/{id}/lyrics -- 更新歌词

**路径:** `/api/v1/songs/{id}/lyrics` | **认证:** BearerAuth

更新歌词内容和来源。入参形态由 `lyric_source` 决定：
- `"url"` -- 传 `lyric_remote_url`，运行时延迟拉取
- 其它（`scraped`/`file`/`embedded`/`cached`/`manual`）-- 传 `lyric`/`tlyric`/`rlyric`/`lxlyric`

`lyric_source=manual` 标记用户手动调整，scanner 重扫不覆盖。

**路径参数:** `id` (int, 必填)

**请求体:**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `lyric_source` | string | 否 | 来源类型 |
| `lyric` | string | 条件 | 主歌词 LRC（非 url 来源） |
| `tlyric` | string | 否 | 翻译歌词 |
| `rlyric` | string | 否 | 罗马音歌词 |
| `lxlyric` | string | 否 | 逐字歌词 |
| `lyric_remote_url` | string | 条件 | 歌词远程 URL（url 来源） |

**成功响应 (200):** `{"message": "歌词已更新", "file_write_status": "written"}`

`file_write_status`: `written` / `unchanged` / `skipped` / `failed`

**错误:** `400` 无效 ID / 请求无效 | `404` 歌曲不存在 | `500` 更新失败

---

### 4.2 GET /songs/{id}/lyric -- 获取歌词

**路径:** `/api/v1/songs/{id}/lyric` | **认证:** BearerAuth

返回 LyricPayload JSON。内部根据 `lyric_source` 分发到数据库解包或远程 URL 拉取。

**路径参数:** `id` (int, 必填)

**成功响应 (200):**

```json
{"lyric": "[00:00.00]夜曲...", "tlyric": "", "rlyric": "", "lxlyric": ""}
```

**错误:** `400` 无效 ID | `404` 歌曲/歌词不存在 | `502` 远程拉取失败

---

### 4.3 PUT /songs/{id}/tags -- 写入音频标签

**路径:** `/api/v1/songs/{id}/tags` | **认证:** BearerAuth

将元数据写入数据库和本地音频文件标签（仅本地歌曲）。非空字段覆盖，空值保留。`cover_data`(base64) 优先于 `cover_url`。

**路径参数:** `id` (int, 必填)

**请求体:**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `title` | string | 否 | 标题 |
| `artist` | string | 否 | 艺术家 |
| `album` | string | 否 | 专辑 |
| `year` | int | 否 | 年份（>0 时覆盖） |
| `genre` | string | 否 | 流派 |
| `lyrics` | string | 否 | 歌词 LRC（写入后 `lyric_source=manual`） |
| `cover_data` | string | 否 | 封面 base64（优先） |
| `cover_url` | string | 否 | 封面 URL（下载后写入） |
| `clear_cover` | bool | 否 | 显式清空封面 |

**成功响应 (200):** `{"song": Song, "file_write": "written"}`

`file_write`: `written` / `unchanged` / `skipped` / `failed` / `unsupported`

**错误:** `400` 无效 ID / 非本地歌曲 / 无效 base64 | `404` 不存在 | `500` 更新失败

---

## 5. 文件整理

**章节来源**: `internal/handlers/music.go`、`internal/services/song_service.go`

### 5.1 POST /songs/organize -- 批量整理文件

**路径:** `/api/v1/songs/organize` | **认证:** BearerAuth

批量移动/重命名本地歌曲文件。`target_path` 为相对于 `music_path` 的路径（含目录和文件名），扩展名必须与原文件一致。

**请求体** (数组):

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | int64 | 是 | 歌曲 ID |
| `target_path` | string | 是 | 目标路径（相对于 `music_path`） |

**成功响应 (200):**

```json
[
  {"id": 1, "status": "ok", "file_path": "/music/周杰伦/夜曲.mp3"},
  {"id": 2, "status": "error", "error": "not a local song"}
]
```

**错误:** `400` 请求无效 / 列表为空 / `music_path` 未设置 | `500` 未配置

---

### 5.2 POST /songs/organize/preview -- 预览批量整理

**路径:** `/api/v1/songs/organize/preview` | **认证:** BearerAuth

dry-run 预览目录整理变更，**不移动任何文件、不改数据库**。返回每项 `old_path`→`new_path` 与状态。`target_path` 为相对 `music_path` 的路径（`music_path` 由服务端自取）。CUE 歌曲返回 `skip`；目标已存在或批内撞名返回 `conflict`。

**请求体** (数组): 同 `/songs/organize`（`id` + `target_path`）。

**成功响应 (200):** `OrganizePreviewResult` 数组，每项含 `status`（`ok` / `conflict` / `skip` / `error`）、`old_path`、`new_path`。

**错误:** `400` 请求无效 / 列表为空

---

## 6. 播放与媒体

**章节来源**: `internal/handlers/music.go`

### 6.1 POST /songs/{id}/activate -- 激活/预取

**路径:** `/api/v1/songs/{id}/activate` | **认证:** BearerAuth

客户端切歌前调用。后端取消同一会话下其他歌曲的进行中工作（prefetch / transcode / reassign），让插件 worker 与转码信号量让位给新歌。幂等，其他客户端会话不受影响。

**路径参数:** `id` (int, 必填)

**成功响应:** `204 No Content`

**错误:** `400` 无效 song_id

---

### 6.2 POST /songs/{id}/played -- 播放事件通知

**路径:** `/api/v1/songs/{id}/played` | **认证:** BearerAuth

客户端在歌曲开始播放、播放完成或被跳过时调用，后端将事件广播给已订阅播放事件的 JS 插件（通过 `songloft.events.onPlayEvent` 注册）。

**路径参数:** `id` (int, 必填)

**查询参数:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `source` | string | 否 | 调用来源标识，如 `songloft-player`（官方客户端）、`miot`（小爱音箱插件） |
| `type` | string | 否 | 事件类型：`play`（开始播放）/ `finish`（播放完成，默认）/ `skip`（用户跳过） |

**成功响应:** `204 No Content`

**错误:** `400` 无效歌曲 ID / 事件类型非法 | `404` 歌曲不存在

---

### 6.3 GET /songs/{id}/play -- 播放流

**路径:** `/api/v1/songs/{id}/play` | **方法:** `GET` / `HEAD` | **认证:** BearerAuth

按歌曲 ID 流式返回音频，内部按 `song.type` 自动分发：

| 类型 | 行为 |
|------|------|
| `local` | `ServeFile`（支持 Range/seek） |
| `remote`（插件） | CacheService 下载缓存后返回 |
| `remote`（外链） | 直接代理转发 |
| `radio`（HLS） | 反代或 302（取决于 hls-proxy 开关） |
| `radio`（非 HLS） | 直接代理上游流 |

**路径参数:** `id` (int, 必填)

**查询参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| `format` | string | 目标格式（如 `opus`）；不同时走 ffmpeg 转码 |
| `prefetch` | string | `"1"` 时异步预热，立即返回 202 |

**成功:** `200` 音频流 | `202` 预拉取已触发 | `206` Range 部分内容 | `302` HLS 重定向

**错误:** `400` 无效 ID | `404` 不存在 | `502` 音源不可用

---

### 6.4 GET /songs/{id}/play.m3u8 -- HLS 别名

**路径:** `/api/v1/songs/{id}/play.m3u8` | **方法:** `GET` / `HEAD` | **认证:** BearerAuth

与 `/play` 共享 handler 的 HLS 电台专用别名。URL 以 `.m3u8` 结尾使 ExoPlayer/AVPlayer 正确选择 HlsMediaSource。客户端通过 `Song.PlaybackURL()` 自动获取正确 URL。参数与响应同 [6.3](#63-get-songsidplay----播放流)。

---

### 6.5 GET /songs/{id}/cover -- 封面

**路径:** `/api/v1/songs/{id}/cover` | **认证:** BearerAuth

获取封面图片。优先返回本地封面，不存在时代理外部 `CoverURL`。

**路径参数:** `id` (int, 必填)

**成功响应 (200):** 图片二进制，`Content-Type` 按扩展名自动设置（jpeg/png/gif/bmp/webp），`Cache-Control: public, max-age=31536000`。

**错误:** `400` 无效 ID | `404` 歌曲/封面不存在 | `500` 读取失败

---

## 7. HLS 反向代理

**章节来源**: `internal/handlers/hls.go`

HLS 反代在 `hls_proxy` 开关开启（`PUT /settings/hls-proxy`）后生效。流程：`serveRadio` -> `ServeProxy` 拉取上游 m3u8 并改写 URI 为本机相对路径 -> 播放器回访 `playlist`/`segment` 拉取子层资源。每次入口做同源校验 + `IsHostnameAllowed` SSRF 防护，非同源 URL 保持原样不改写。

### 7.1 GET /songs/{id}/hls/playlist -- 代理播放列表

**路径:** `/api/v1/songs/{id}/hls/playlist` | **方法:** `GET` / `HEAD` | **认证:** BearerAuth

反代 HLS 子层 m3u8。拉取上游 -> 同源校验 -> 改写 URI -> 回写。上游错误透传，播放列表上限 1 MB。

**参数:** `id` (path, int) | `u` (query, string, 必填, base64url 编码的上游 URL) | `access_token` (query, string, 可选)

**成功 (200):** 改写后 m3u8 文本，`Content-Type: application/vnd.apple.mpegurl`，`Cache-Control: no-store`

**错误:** `400` 无效 ID / 缺少 u | `403` 非同源 / 主机不允许 | `404` 歌曲不存在 | `502` 上游不可用

---

### 7.2 GET /songs/{id}/hls/segment -- 代理切片

**路径:** `/api/v1/songs/{id}/hls/segment` | **方法:** `GET` / `HEAD` | **认证:** BearerAuth

反代音频切片/key/init 段。透传 Range 和上游响应头。

**参数:** `id` (path, int) | `u` (query, string, 必填, base64url 编码的上游 URL) | `access_token` (query, string, 可选)

**成功:** `200` 切片内容 | `206` Range 部分内容。`Cache-Control: no-store`

**错误:** `400` 无效 ID / 缺少 u | `403` 非同源 / 主机不允许 | `404` 歌曲不存在 | `502` 上游不可用
