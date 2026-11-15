# 标签与主题 API

源码：`internal/handlers/song_tag.go`、`theme_pack.go`、`theme_pack_catalog.go`、`internal/app/routers.go`；字段定义见 `docs/swagger.json`。所有接口均要求 Bearer Token。

标签通过 `song_tag_links` 与歌曲关联，不替代原有 `genre` / `style` 字段。主题包为声明式 JSON，目录安装与手动导入共用校验流程；当前激活主题由独立端点管理。

### GET /api/v1/song-tags

列出所有标签。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `keyword` | query | `string` | false |
| `sort` | query | `string` | false |
| `order` | query | `string` | false |
| `limit` | query | `integer` | false |
| `offset` | query | `integer` | false |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `object` |
| 500 | `object` |

### POST /api/v1/song-tags

创建标签。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `request` | body | `object` | true |

| 状态 | 响应类型 |
| --- | --- |
| 201 | `SongTag` |
| 400 | `object` |
| 500 | `object` |
### GET /api/v1/song-tags/{id}

获取标签详情。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `SongTag` |
| 404 | `object` |

### PUT /api/v1/song-tags/{id}

更新标签。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |
| `request` | body | `object` | true |

| 状态 | 响应类型 |
| --- | --- |
| 204 | `object` |
| 400 | `object` |
| 404 | `object` |

### DELETE /api/v1/song-tags/{id}

删除标签。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |

| 状态 | 响应类型 |
| --- | --- |
| 204 | `object` |
| 400 | `object` |
### POST /api/v1/song-tags/{id}/bind

批量绑定歌曲到标签。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |
| `request` | body | `object` | true |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `object` |
| 400 | `object` |
| 500 | `object` |
### GET /api/v1/song-tags/{id}/song-ids

获取标签下所有歌曲 ID。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `array<integer>` |
| 400 | `object` |
| 500 | `object` |
### GET /api/v1/song-tags/{id}/songs

获取标签下的歌曲列表。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |
| `limit` | query | `integer` | false |
| `offset` | query | `integer` | false |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `object` |
| 400 | `object` |
| 500 | `object` |
### POST /api/v1/song-tags/{id}/unbind

批量解绑歌曲。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `id` | path | `integer` | true |
| `request` | body | `object` | true |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `object` |
| 400 | `object` |
| 500 | `object` |
### GET /api/v1/theme-packs

列出主题包。

| 状态 | 响应类型 |
| --- | --- |
| 200 | `array<ThemePackListItem>` |

### POST /api/v1/theme-packs

导入主题包。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `request` | body | `object` | true |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `ThemePackResponse` |
| 400 | `ErrorResponse` |
### GET /api/v1/theme-packs/active

获取激活主题包。

| 状态 | 响应类型 |
| --- | --- |
| 200 | `ThemePackResponse` |

### PUT /api/v1/theme-packs/active

设置激活主题包。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `request` | body | `object` | true |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `SuccessResponse` |
| 400 | `ErrorResponse` |
| 404 | `ErrorResponse` |

### DELETE /api/v1/theme-packs/active

恢复默认主题。

| 状态 | 响应类型 |
| --- | --- |
| 200 | `SuccessResponse` |
### POST /api/v1/theme-packs/catalog/install

从在线目录安装主题包。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `request` | body | `object` | true |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `ThemePackResponse` |
| 400 | `ErrorResponse` |
### POST /api/v1/theme-packs/catalog/refresh

获取在线主题目录。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `request` | body | `object` | true |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `object` |
### GET /api/v1/theme-packs/{themeID}

获取主题包详情。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `themeID` | path | `string` | true |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `ThemePackResponse` |
| 404 | `ErrorResponse` |

### DELETE /api/v1/theme-packs/{themeID}

删除主题包。

| 参数 | 位置 | 类型 | 必填 |
| --- | --- | --- | --- |
| `themeID` | path | `string` | true |

| 状态 | 响应类型 |
| --- | --- |
| 200 | `SuccessResponse` |
| 404 | `ErrorResponse` |

## 歌单共享标签

歌曲与歌单共用 `/song-tags` 标签定义。歌曲绑定使用 `song_tag_links`，歌单引用保存在 `labels` 的 `tag:<id>` 中。`GET/PUT /api/v1/playlists/{id}/tags` 返回共享定义或以必填 `{"tag_ids":[1,2]}` 替换引用；空数组清空自定义标签并保留系统标记，无效 ID 导致整次更新失败。

`SongTag` 分别返回 `song_count` 与 `playlist_count`，后者不含隐藏歌单。`/song-tags?sort=playlist_count` 可按可见歌单数排序。删除定义在同一事务内清理歌曲与歌单引用。
