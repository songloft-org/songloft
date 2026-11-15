# Tags and themes API

Sources: `internal/handlers/song_tag.go`, `theme_pack.go`, `theme_pack_catalog.go`, and `internal/app/routers.go`. Field definitions are in `docs/swagger.json`. Every interface requires a Bearer token.

Tags link to songs through `song_tag_links` and do not replace `genre` / `style`. Theme packs are declarative JSON. Catalog installation and manual import share validation; separate endpoints manage the active pack.

### GET /api/v1/song-tags

GET /song-tags.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `keyword` | query | `string` | false |
| `sort` | query | `string` | false |
| `order` | query | `string` | false |
| `limit` | query | `integer` | false |
| `offset` | query | `integer` | false |

| Status | Response type |
| --- | --- |
| 200 | `object` |
| 500 | `object` |

### POST /api/v1/song-tags

POST /song-tags.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `request` | body | `object` | true |

| Status | Response type |
| --- | --- |
| 201 | `SongTag` |
| 400 | `object` |
| 500 | `object` |
### GET /api/v1/song-tags/{id}

GET /song-tags/{id}.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |

| Status | Response type |
| --- | --- |
| 200 | `SongTag` |
| 404 | `object` |

### PUT /api/v1/song-tags/{id}

PUT /song-tags/{id}.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |
| `request` | body | `object` | true |

| Status | Response type |
| --- | --- |
| 204 | `object` |
| 400 | `object` |
| 404 | `object` |

### DELETE /api/v1/song-tags/{id}

DELETE /song-tags/{id}.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |

| Status | Response type |
| --- | --- |
| 204 | `object` |
| 400 | `object` |
### POST /api/v1/song-tags/{id}/bind

POST /song-tags/{id}/bind.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |
| `request` | body | `object` | true |

| Status | Response type |
| --- | --- |
| 200 | `object` |
| 400 | `object` |
| 500 | `object` |
### GET /api/v1/song-tags/{id}/song-ids

GET /song-tags/{id}/song-ids.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |

| Status | Response type |
| --- | --- |
| 200 | `array<integer>` |
| 400 | `object` |
| 500 | `object` |
### GET /api/v1/song-tags/{id}/songs

GET /song-tags/{id}/songs.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |
| `limit` | query | `integer` | false |
| `offset` | query | `integer` | false |

| Status | Response type |
| --- | --- |
| 200 | `object` |
| 400 | `object` |
| 500 | `object` |
### POST /api/v1/song-tags/{id}/unbind

POST /song-tags/{id}/unbind.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |
| `request` | body | `object` | true |

| Status | Response type |
| --- | --- |
| 200 | `object` |
| 400 | `object` |
| 500 | `object` |
### GET /api/v1/theme-packs

GET /theme-packs.

| Status | Response type |
| --- | --- |
| 200 | `array<ThemePackListItem>` |

### POST /api/v1/theme-packs

POST /theme-packs.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `request` | body | `object` | true |

| Status | Response type |
| --- | --- |
| 200 | `ThemePackResponse` |
| 400 | `ErrorResponse` |
### GET /api/v1/theme-packs/active

GET /theme-packs/active.

| Status | Response type |
| --- | --- |
| 200 | `ThemePackResponse` |

### PUT /api/v1/theme-packs/active

PUT /theme-packs/active.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `request` | body | `object` | true |

| Status | Response type |
| --- | --- |
| 200 | `SuccessResponse` |
| 400 | `ErrorResponse` |
| 404 | `ErrorResponse` |

### DELETE /api/v1/theme-packs/active

DELETE /theme-packs/active.

| Status | Response type |
| --- | --- |
| 200 | `SuccessResponse` |
### POST /api/v1/theme-packs/catalog/install

POST /theme-packs/catalog/install.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `request` | body | `object` | true |

| Status | Response type |
| --- | --- |
| 200 | `ThemePackResponse` |
| 400 | `ErrorResponse` |
### POST /api/v1/theme-packs/catalog/refresh

POST /theme-packs/catalog/refresh.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `request` | body | `object` | true |

| Status | Response type |
| --- | --- |
| 200 | `object` |
### GET /api/v1/theme-packs/{themeID}

GET /theme-packs/{themeID}.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `themeID` | path | `string` | true |

| Status | Response type |
| --- | --- |
| 200 | `ThemePackResponse` |
| 404 | `ErrorResponse` |

### DELETE /api/v1/theme-packs/{themeID}

DELETE /theme-packs/{themeID}.

| Parameter | Location | Type | Required |
| --- | --- | --- | --- |
| `themeID` | path | `string` | true |

| Status | Response type |
| --- | --- |
| 200 | `SuccessResponse` |
| 404 | `ErrorResponse` |

## Shared Playlist Tags

Songs and playlists share the `/song-tags` definitions. Song bindings use `song_tag_links`; playlist references use `tag:<id>` in `labels`. `GET/PUT /api/v1/playlists/{id}/tags` returns shared definitions or replaces references with required `{"tag_ids":[1,2]}`; an empty array clears custom tags and preserves system labels. Invalid IDs fail the entire update.

`SongTag` includes separate `song_count` and `playlist_count`; the latter excludes hidden playlists. `/song-tags?sort=playlist_count` sorts by visible playlist count. Deleting a definition cleans up both song and playlist references transactionally.
