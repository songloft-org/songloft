# Play History API

This document is based on the following source files:

- [internal/handlers/play_history.go](https://github.com/songloft-org/songloft/blob/main/internal/handlers/play_history.go) -- Play history handler
- [internal/services/play_history_service.go](https://github.com/songloft-org/songloft/blob/main/internal/services/play_history_service.go) -- Play history service (in-transaction upsert + trim)
- [internal/database/play_history_repository.go](https://github.com/songloft-org/songloft/blob/main/internal/database/play_history_repository.go) -- Repository
- [internal/database/migrations/0030_play_history.sql](https://github.com/songloft-org/songloft/blob/main/internal/database/migrations/0030_play_history.sql) -- Table structure
- [internal/handlers/music.go](https://github.com/songloft-org/songloft/blob/main/internal/handlers/music.go) -- Write entry point (`POST /songs/{id}/played`)

## Table of Contents

1. [Overview](#1-overview)
2. [Playback Context](#2-playback-context)
3. [Endpoints](#3-endpoints)
   - [GET /play-history -- Query play history](#31-get-play-history----query-play-history)
   - [DELETE /play-history -- Clear play history](#32-delete-play-history----clear-play-history)
   - [DELETE /play-history/entry -- Remove a single entry](#33-delete-play-historyentry----remove-a-single-entry)
   - [Writing: POST /songs/{id}/played](#34-writing-post-apiv1songsidplayed)
4. [Design Notes](#4-design-notes)

---

## 1. Overview

Play history records recently played songs separately per "playback context", addressing songloft-org/songloft#333: the client has only one global playback queue, so after switching playlists and switching back, the original playlist's "where did I leave off" is lost and playback can only restart from the first song or a random one.

With history archived per context, users can open the history panel on a playlist / artist / album page and continue from the song they last listened to.

Key points:

- **Not limited to playlists**: artists, albums and all other facet dimensions are supported too
- **Deduplicated by song within a context**: replaying only refreshes the timestamp and increments `play_count`
- **At most 50 entries per context**, so the query endpoint is not paginated
- Entries are written **as a side effect** of the playback-reporting endpoint `POST /songs/{id}/played` when `type=play`; the client needs no extra request

---

## 2. Playback Context

A context is identified by the `context_type` + `context_key` pair:

| context_type | context_key | Example |
|---|---|---|
| `playlist` | Playlist ID | `("playlist", "3")` |
| `artist` | Artist name | `("artist", "Jay Chou")` |
| `album` | Album name | `("album", "Fantasy")` |
| `genre` | Genre | `("genre", "Pop")` |
| `year` | Year | `("year", "2005")` |
| `decade` | Decade start year | `("decade", "2000")` |
| `language` | Language | `("language", "Mandarin")` |
| `style` | Style | `("style", "R&B")` |

Apart from `playlist`, all values come from the song facet dimensions (`songFacetColumn` in `internal/database/filters.go`) -- **no separate enum is defined**. An unsupported `context_type` always returns 400.

**`context_key` is a query parameter rather than a path parameter**: artist / album names may contain `/`, `%` and similar characters, which would create encoding ambiguity in a URL path (the frontend router puts facet values in the query for the same reason).

> The flat library list (a keyword + type filter combination) does not form a stable context, so the client passes no context there and no history is recorded.

---

## 3. Endpoints

### 3.1 GET /play-history -- Query play history

Returns recently played songs within the given context, ordered by last-played time descending, including **full song details**.

- **Authentication**: Bearer Token
- **Query parameters**:
  - `context_type` (string, required)
  - `context_key` (string, required)
  - `limit` (int, optional, defaults to 50, capped at 50)
- **200**:

```json
{
  "items": [
    {
      "song": { "id": 2, "title": "晴天", "artist": "周杰伦", "...": "full Song object" },
      "played_at": "2026-07-30T08:50:44+08:00",
      "play_count": 1
    }
  ],
  "total": 1
}
```

- **400**: unsupported `context_type` or missing `context_key` | **500**: Server error

Returning the full `Song` is deliberate: when the client starts playback from a history entry, it can use this object as the first queue item and start playing with **zero extra requests** (see [Design Notes](#4-design-notes)).

### 3.2 DELETE /play-history -- Clear play history

Deletes all entries within the given context. Returns `deleted: 0` when the context had no entries, which is not treated as an error.

- **Authentication**: Bearer Token
- **Query parameters**: `context_type` (required), `context_key` (required)
- **200**: `{"deleted": 3}`
- **400**: Invalid parameters | **500**: Server error

### 3.3 DELETE /play-history/entry -- Remove a single entry

Removes one song's entry from the given context. Typical use: clearing entries that have been removed from the playlist and show as stale in the history panel.

- **Authentication**: Bearer Token
- **Query parameters**: `context_type` (required), `context_key` (required), `song_id` (int, required)
- **204**: Deleted, no content
- **400**: Invalid parameters | **404**: No entry for that song in this context | **500**: Server error

### 3.4 Writing: POST /api/v1/songs/{id}/played

Play history has no dedicated write endpoint; it hooks into the existing playback-reporting endpoint (see [Song API](歌曲%20API.md)). Two optional parameters are added alongside `source` / `type`:

| Parameter | Description |
|---|---|
| `context_type` | Playback context type; only effective when `type=play` |
| `context_key` | Playback context identifier; only effective when `type=play` |

When both are present and valid, the song is additionally written to that context's play history. **A write failure is only logged and the status code is always 204**, so playback is never affected.

**Only `type=play` is persisted**:

- `type=play` fires after playback succeeds on the client, covering both manual selection and auto-advance -- exactly the "where did I leave off" semantics
- `type=finish` is redundant information about the same song
- `type=skip` reports the **previous** song, and by then the client's context may already have switched to a new playlist, which would file the previous song under the wrong context

The client also omits the context for `finish` / `skip`, and the backend only accepts `play` -- a deliberate double safeguard.

---

## 4. Design Notes

### Deduplication and trimming

Deduplication relies on the database constraint `UNIQUE(context_type, context_key, song_id)` plus an upsert (`ON CONFLICT DO UPDATE` refreshes `played_at` and increments `play_count`) rather than application-level lookups.

The cap `services.MaxPlayHistoryPerContext = 50` is a Go constant rather than a config item (making it configurable would require yet another settings endpoint, which is not worth it). The insert and the trim run in the **same transaction** (`db.RunInTx`) so a mid-way failure cannot leave excess rows.

The trim SQL uses `id NOT IN (... ORDER BY played_at DESC, id DESC LIMIT ?)`: `played_at` has second-level precision and will collide, so `id DESC` provides a deterministic tie-breaker.

### Cleanup semantics

| Scenario | Behavior |
|---|---|
| Song deleted from the library | The `song_id` foreign key `ON DELETE CASCADE` cleans up automatically |
| Playlist deleted | `context_key` is TEXT, so no foreign key to `playlists` is possible; `PlaylistService` explicitly calls `ClearByPlaylist`. **Batch deletion only clears playlists that were actually removed** -- built-in playlists (Favorites / Radio Favorites) are never deleted, so their history must survive |
| Song removed from a playlist but still in the library | The entry is **kept** (history is history). The client reports it as stale when it cannot be located during playback |
| Artist / album renamed | Entries under the old key become stale but are kept; handled as above |

### Client playback strategy: why no "track number" is returned

An intuitive design would have the endpoint return the song's index within the context so the client could page directly to it. **This API deliberately does not**, because such an index is unreliable for facet dimensions:

Facet song lists go through `GET /songs?artist=X`, whose default ordering is `added_at DESC`. But `added_at` is a second-precision `DATETIME`, and a bulk scan import inserts sequentially inside a single transaction, so hundreds or thousands of songs end up with **identical** `added_at` values. `applyOrder` emits a single-column `ORDER BY` with **no `id` tie-breaker**, so the index is mathematically indeterminate -- starting playback from it would play the wrong song.

Facet history playback uses the full `Song` carried by the entry to start immediately, then locates the ID in the ordered facet list and fills a rotated queue `[target, after…, beginning…before]` in the background.

Normal playlists retain the full persisted playlist order and map the selected track to its current index. Their queues load in batches around that track, extending toward later tracks before earlier ones. Explicit selection from history starts the track at zero; Continue restores the separate millisecond position and playlist mode from `/playlists/{id}/playback-progress`. Playback history does not store positions. See [Playlist resume](../../playlist-resume.md).
