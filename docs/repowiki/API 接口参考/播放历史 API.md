# 播放历史 API

本文档基于以下源文件编写：

- [internal/handlers/play_history.go](https://github.com/songloft-org/songloft/blob/main/internal/handlers/play_history.go) -- 播放历史处理器
- [internal/services/play_history_service.go](https://github.com/songloft-org/songloft/blob/main/internal/services/play_history_service.go) -- 播放历史服务（事务内 upsert + 裁剪）
- [internal/database/play_history_repository.go](https://github.com/songloft-org/songloft/blob/main/internal/database/play_history_repository.go) -- 仓储
- [internal/database/migrations/0030_play_history.sql](https://github.com/songloft-org/songloft/blob/main/internal/database/migrations/0030_play_history.sql) -- 表结构
- [internal/handlers/music.go](https://github.com/songloft-org/songloft/blob/main/internal/handlers/music.go) -- 写入入口（`POST /songs/{id}/played`）

## 目录

1. [概述](#1-概述)
2. [播放上下文](#2-播放上下文)
3. [端点列表](#3-端点列表)
   - [GET /play-history -- 查询播放历史](#31-get-play-history----查询播放历史)
   - [DELETE /play-history -- 清空播放历史](#32-delete-play-history----清空播放历史)
   - [DELETE /play-history/entry -- 删除单条记录](#33-delete-play-historyentry----删除单条记录)
   - [写入：POST /songs/{id}/played](#34-写入post-apiv1songsidplayed)
4. [设计说明](#4-设计说明)

---

## 1. 概述

播放历史按「播放上下文」分别记录最近播放过的歌曲，解决 songloft-org/songloft#333：客户端的播放队列全局只有一份，切换歌单后再切回来，原歌单「播到哪了」就丢了，只能从第一首或随机起播。

有了按上下文归档的历史，用户在歌单 / 歌手 / 专辑页可以打开历史面板，从上次听的那首接着往下播。

要点：

- **不限于歌单**：歌手、专辑等全部分面维度同样支持
- **同一上下文内按歌曲去重**：重复播放只刷新时间并累加 `play_count`
- **每上下文最多保留 50 条**，因此查询端点不分页
- 记录由播放打点端点 `POST /songs/{id}/played` 在 `type=play` 时**顺带写入**，客户端无需额外请求

---

## 2. 播放上下文

上下文由 `context_type` + `context_key` 二元组标识：

| context_type | context_key | 示例 |
|---|---|---|
| `playlist` | 歌单 ID | `("playlist", "3")` |
| `artist` | 歌手名 | `("artist", "周杰伦")` |
| `album` | 专辑名 | `("album", "范特西")` |
| `genre` | 流派 | `("genre", "Pop")` |
| `year` | 年份 | `("year", "2005")` |
| `decade` | 年代起始年 | `("decade", "2000")` |
| `language` | 语种 | `("language", "国语")` |
| `style` | 风格 | `("style", "R&B")` |

除 `playlist` 外全部取自歌曲分面维度（`internal/database/filters.go` 的 `songFacetColumn`），**不另立枚举**。非法 `context_type` 一律返回 400。

**`context_key` 走 query 参数而非路径参数**：歌手 / 专辑名可能含 `/`、`%` 等字符，放进 URL 路径会有编解码歧义（前端路由出于同样原因也把分面取值放在 query）。

> 曲库扁平列表（keyword + type 组合筛选）不构成稳定上下文，客户端在那种场景不传 context，因此不产生历史记录。

---

## 3. 端点列表

### 3.1 GET /play-history -- 查询播放历史

返回指定上下文内最近播放过的歌曲，按最后播放时间倒序，含**完整歌曲详情**。

- **认证**: Bearer Token
- **查询参数**:
  - `context_type`（string，必填）
  - `context_key`（string，必填）
  - `limit`（int，可选，缺省 50，上限 50）
- **200**:

```json
{
  "items": [
    {
      "song": { "id": 2, "title": "晴天", "artist": "周杰伦", "...": "完整 Song 对象" },
      "played_at": "2026-07-30T08:50:44+08:00",
      "play_count": 1
    }
  ],
  "total": 1
}
```

- **400**: `context_type` 不支持或缺少 `context_key` | **500**: 服务器错误

返回完整 `Song` 是刻意设计：客户端点某条历史起播时，直接用这个对象当队列首曲即可**零额外请求**立即出声（见[设计说明](#4-设计说明)）。

### 3.2 DELETE /play-history -- 清空播放历史

删除指定上下文内的全部记录。上下文本就没有记录时返回 `deleted: 0`，不视为错误。

- **认证**: Bearer Token
- **查询参数**: `context_type`（必填）、`context_key`（必填）
- **200**: `{"deleted": 3}`
- **400**: 参数非法 | **500**: 服务器错误

### 3.3 DELETE /play-history/entry -- 删除单条记录

从指定上下文中删除某首歌的播放记录。典型用途：清理已被移出歌单、在历史面板里显示为失效的条目。

- **认证**: Bearer Token
- **查询参数**: `context_type`（必填）、`context_key`（必填）、`song_id`（int，必填）
- **204**: 删除成功，无内容
- **400**: 参数非法 | **404**: 该上下文中不存在此歌曲的记录 | **500**: 服务器错误

### 3.4 写入：POST /api/v1/songs/{id}/played

播放历史没有独立的写入端点，而是挂在既有的播放打点端点上（详见[歌曲 API](歌曲%20API.md)）。在原有 `source` / `type` 之外新增两个可选参数：

| 参数 | 说明 |
|---|---|
| `context_type` | 播放上下文类型，仅 `type=play` 时生效 |
| `context_key` | 播放上下文标识，仅 `type=play` 时生效 |

两者都传且合法时，额外把该歌曲写入对应上下文的播放历史。**落库失败只记日志，响应码永远 204**，不影响播放主流程。

**只有 `type=play` 会落库**：

- `type=play` 在客户端播放成功后触发，覆盖手动点播与自动续播，正是「播到哪了」的准确语义
- `type=finish` 是同一首歌的重复信息，无新增价值
- `type=skip` 上报的是**上一首**歌，此时客户端的上下文可能已切到新歌单，带上会把上一首错记到新上下文名下

客户端对 `finish` / `skip` 也不传 context，后端只认 `play`，构成双重保险。

---

## 4. 设计说明

### 去重与裁剪

去重靠数据库约束 `UNIQUE(context_type, context_key, song_id)` + upsert（`ON CONFLICT DO UPDATE` 刷新 `played_at`、累加 `play_count`），不在应用层查重。

上限 `services.MaxPlayHistoryPerContext = 50` 是 Go 常量而非配置项（做成配置就得再开一个配置端点，收益不匹配）。写入与裁剪在**同一事务**内（`db.RunInTx`），避免中途失败留下超额记录。

裁剪 SQL 用 `id NOT IN (... ORDER BY played_at DESC, id DESC LIMIT ?)`：`played_at` 是秒级精度会撞，`id DESC` 做确定性 tie-break。

### 清理语义

| 场景 | 行为 |
|---|---|
| 歌曲从库中删除 | `song_id` 外键 `ON DELETE CASCADE` 自动清理 |
| 删除歌单 | `context_key` 是 TEXT，无法对 `playlists` 建外键，由 `PlaylistService` 显式调 `ClearByPlaylist`。**批量删除时只清理真正被删掉的歌单** —— 内置歌单（收藏 / 电台收藏）不会被删除，其历史必须存活 |
| 歌曲被移出歌单但仍在库里 | 历史行**保留**（历史就是历史）。客户端起播时定位不到即判定失效并提示 |
| 歌手 / 专辑改名 | 旧 key 下的历史失效但保留，同上处理 |

### 客户端起播策略：为何不返回「第几首」

一个直觉的设计是让端点返回该歌曲在上下文中的序号，客户端据此分页定位。**本 API 刻意不这么做**，因为序号在分面维度下并不可靠：

分面歌曲列表走 `GET /songs?artist=X`，默认排序是 `added_at DESC`；而 `added_at` 是秒级精度的 `DATETIME`，批量扫描导入在单事务里顺序 INSERT，成百上千首歌的 `added_at` 会**完全相同**。`applyOrder` 只产出单列 `ORDER BY`，**没有 `id` tie-break**，因此序号在数学上不确定 —— 据此起播会播到错的歌。

分面历史使用条目自带的完整 `Song` 立即起播，再在分面有序 ID 列表定位，后台补齐环形队列 `[目标歌, 目标之后…, 开头…目标之前]`。

普通歌单保持完整的持久顺序，将所选歌曲映射到当前索引；队列从该曲附近分批加载，优先向后再向前补齐。明确点击历史歌曲从零起播；「继续播放」通过独立的 `/playlists/{id}/playback-progress` 恢复毫秒位置和歌单模式，播放历史本身不存秒数。详见[歌单续播](../../playlist-resume.md)。
