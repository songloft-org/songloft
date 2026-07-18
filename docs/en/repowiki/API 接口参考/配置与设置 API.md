# Configuration and Settings API

This document is based on the following source files:

- `internal/handlers/config.go` -- Generic KV configuration CRUD handler
- `internal/handlers/tab_config_setting.go` -- Bottom navigation tab configuration settings endpoint
- `internal/handlers/log.go` -- Log level settings endpoint
- `internal/handlers/scan.go` -- Music path, auto scan, scan title source, and other settings endpoints
- `internal/handlers/hls.go` -- HLS proxy switch settings endpoint
- `internal/handlers/jsplugin_registry.go` -- Plugin registry sources and HTTP proxy settings endpoints
- `internal/app/routers.go` -- Configuration and settings route registration
- `internal/models/models.go` -- Config / CreateConfigRequest / UpdateConfigRequest structs

## Table of Contents

1. [Design Overview](#1-design-overview)
2. [Generic Configuration Management /configs](#2-generic-configuration-management-configs)
3. [Business Settings /settings](#3-business-settings-settings)
   - [Music Path](#31-music-path)
   - [HLS Proxy Switch](#32-hls-proxy-switch)
   - [Auto Scan](#33-auto-scan)
   - [Scan Title Source](#34-scan-title-source)
   - [Scan Auto-Create Playlists](#35-scan-auto-create-playlists)
   - [Scan Playlist Merge Mode](#36-scan-playlist-merge-mode)
   - [Log Level](#37-log-level)
   - [Plugin Registry Sources](#38-plugin-registry-sources)
   - [HTTP Proxy](#39-http-proxy)
   - [Bottom Navigation Tab Configuration](#310-bottom-navigation-tab-configuration)
   - [Remote Song Title Source](#311-remote-song-title-source)
   - [GitHub Update Proxy](#312-github-update-proxy)
   - [Plugin Keep-Alive Whitelist](#313-plugin-keep-alive-whitelist)
   - [Plugin Auto-Update](#314-plugin-auto-update)
   - [Library Browse Views](#315-library-browse-views)
   - [User Preferences](#316-user-preferences)
   - [Equalizer](#317-equalizer)

---

## 1. Design Overview

**Section source**: `AGENTS.md` (configuration interface specification), `internal/app/routers.go`

Songloft has two categories of configuration interfaces, and user-facing feature switches always go through the business endpoints:

| Category | Path style | Purpose | Characteristics |
|------|----------|------|------|
| **Business settings** | `/settings/<name>` | User-facing feature switches | Strongly-typed JSON, built-in defaults, triggers side effects after PUT |
| **Generic KV** | `/configs/{key}` | Admin editor only | Pure string KV, PUT returns 404 when the key does not exist, no defaults |

Client business features always go through the `/settings/*` endpoints (`SettingsApi`); `/configs/{key}` is used only by the frontend `config_manager.dart` generic configuration editor (`ConfigApi`). Both can read/write the same underlying config key (dual entry retained), and side effects are triggered uniformly by the `configHandler.SetOnConfigChanged` callback.

---

## 2. Generic Configuration Management /configs

**Section source**: `internal/handlers/config.go`

Generic KV configuration endpoints, for admin tools to edit arbitrary configuration items. New business features should prefer the `/settings/*` endpoints.

### GET /api/v1/configs

Gets the configuration list, ordered by key ascending, supporting keyword search and pagination.

- **Authentication**: Bearer Token
- **Query parameters**:

| Parameter | Type | Required | Description |
|------|------|------|------|
| `keyword` | string | No | Search keyword (matches by key) |
| `limit` | int | No | Items per page, default 20 |
| `offset` | int | No | Offset, default 0 |

- **200**: `{"configs": [Config], "total": 5, "limit": 20, "offset": 0}`
- **500**: Server error

### POST /api/v1/configs

Creates a configuration item.

- **Authentication**: Bearer Token
- **Request body**: `{"key": "music_path", "value": "{\"path\":\"/music\"}"}` (both key and value are required)
- **201**: Returns a `Config` object (containing `id`/`key`/`value`/`updated_at`)
- **400**: Empty key or value | **500**: Creation failed

### GET /api/v1/configs/{key}

Gets a single configuration.

- **Authentication**: Bearer Token
- **Path parameter**: `key` (string)
- **200**: Returns a `Config` object
- **404**: Configuration does not exist

### PUT /api/v1/configs/{key}

Updates an existing configuration. The configuration must already exist, otherwise it returns 404. After the update, asynchronously triggers the `onConfigChanged` callback (`music_path` rebuilds the Scanner, `auto_scan` restarts the scheduler).

- **Authentication**: Bearer Token
- **Path parameter**: `key` (string)
- **Request body**: `{"value": "new_value"}` (value is required)
- **200**: Returns the updated `Config`
- **400**: Empty value | **404**: Key does not exist | **500**: Update failed

### DELETE /api/v1/configs/{key}

Deletes a configuration item.

- **Authentication**: Bearer Token
- **Path parameter**: `key` (string)
- **200**: `{"message": "配置已删除"}`
- **400**: Empty key | **500**: Delete failed

---

## 3. Business Settings /settings

**Section source**: `internal/handlers/scan.go`, `hls.go`, `log.go`, `jsplugin_registry.go`, `tab_config_setting.go`

All business settings endpoints follow a uniform pattern: GET returns the current configuration (returning the business default when not configured), and PUT writes the configuration and triggers the related side effects. All endpoints require Bearer Token authentication.

### 3.1 Music Path

**`GET /api/v1/settings/music-path`** -- Gets the music path and scan exclusion configuration.

**`PUT /api/v1/settings/music-path`** -- Updates the configuration; `path` cannot be empty.

```json
{
  "path": "music",
  "exclude_dirs": ["@eaDir", "tmp"],
  "exclude_paths": []
}
```

| Field | Type | Default | Description |
|------|------|--------|------|
| `path` | string | `"music"` | Music directory path |
| `exclude_dirs` | string[] | `["@eaDir","tmp"]` | Exclude by directory name |
| `exclude_paths` | string[] | `[]` | Exclude by full path |

- **PUT side effect**: Asynchronously triggers `onMusicPathChanged` (rebuilds the Scanner + cleans up songs in excluded directories)
- **400**: Empty path | **500**: Save failed

### 3.2 HLS Proxy Switch

**`GET/PUT /api/v1/settings/hls-proxy`** -- `{"enabled": false}`

Disabled by default. When disabled, the radio `.m3u8` is directly redirected (302) to the player; when enabled, radio segment bytes are all forwarded through the local machine, resolving source-site Referer/CORS blocking, but all segment traffic goes through the local machine's bandwidth.

- **400**: Bad request format | **500**: Save failed

### 3.3 Auto Scan

**`GET/PUT /api/v1/settings/auto-scan`**

```json
{"enabled": false, "interval_seconds": 3600}
```

Disabled by default, with an interval of 3600 seconds (1 hour). PUT validates that `interval_seconds` is in the range [60, 86400]; takes effect immediately after the update without a restart.

- **PUT side effect**: Asynchronously triggers `onAutoScanChanged` (restarts the auto-scan scheduler)
- **400**: interval_seconds out of range | **500**: Save failed

### 3.4 Scan Title Source

**`GET/PUT /api/v1/settings/scan-title-source`** -- `{"title_source": "tag"}`

| Value | Description |
|------|------|
| `tag` | Prefer the title in the audio tags (default) |
| `filename` | Always use the file name (without extension) as the title |

After switching, a scan in "re-import" mode is required for it to take effect.

- **PUT side effect**: Asynchronously triggers a Scanner rebuild
- **400**: title_source is not `tag` or `filename`

### 3.5 Scan Auto-Create Playlists

**`GET/PUT /api/v1/settings/scan-auto-create-playlists`** -- `{"enabled": true}`

Enabled by default. When enabled, playlists are automatically created based on the music directory structure after a scan completes; when disabled, scanning only imports songs into the database and does not create playlists.

### 3.6 Scan Playlist Merge Mode

**`GET/PUT /api/v1/settings/scan-playlist-mode`** -- `{"mode": "directory"}`

Controls the directory merge mode when automatically creating playlists after a scan. Default `directory`.

| Value | Description |
|------|------|
| `directory` | Generate an independent playlist for each folder (default) |
| `top_level` | Merge playlists by first-level subdirectory |
| `bubble_up` | Songs appear simultaneously in the playlists of all parent folders |

- **400**: Illegal `mode` value (only `directory` / `top_level` / `bubble_up` are accepted) | **500**: Save failed

### 3.7 Log Level

**`GET/PUT /api/v1/settings/log-level`** -- `{"level": "info"}`

Allowed values: `debug` / `info` / `warn` / `error`, default `info`. PUT switches the runtime log level immediately via a shared `slog.LevelVar` while also persisting it to the DB, so it is automatically restored after a restart.

- **400**: Illegal level value (only the four enum values above are accepted)
- **500**: Save failed

### 3.8 Plugin Registry Sources

**`GET /api/v1/settings/plugin-registries`** -- Gets the registry source list (returns the built-in default source when not configured).

**`PUT /api/v1/settings/plugin-registries`** -- Saves the registry source list.

```json
{
  "registries": [
    {"url": "https://example.com/registry.json", "name": "官方插件源", "enabled": true}
  ]
}
```

Each item contains: `url` (registry JSON URL), `name` (name), `enabled` (whether it is enabled).

- **400**: Bad request format | **500**: Save failed

### 3.9 HTTP Proxy

**`GET/PUT /api/v1/settings/http-proxy`** -- `{"proxy": ""}`

Empty string by default (direct connection). Once set, all outbound backend HTTP requests are forwarded through the proxy, including: plugin registry fetching, plugin download/update, and system upgrade check/download. Supports the HTTP/HTTPS/SOCKS5 protocols. Loopback addresses automatically skip the proxy.

- Typical value: `http://192.168.1.1:7890`
- **PUT side effect**: Calls `httputil.SetGlobalProxy` to immediately update the globally shared `*http.Transport`
- **400**: Invalid proxy address format | **500**: Save failed

### 3.10 Bottom Navigation Tab Configuration

**`GET /api/v1/settings/tab-config`** -- Gets the tab configuration (returns the default value when not configured: 4 tabs).

**`PUT /api/v1/settings/tab-config`** -- Saves the tab configuration.

```json
{
  "show_library": true,
  "show_playlists": true,
  "plugin_tabs": [
    {"plugin_id": 1, "entry_path": "myplugin", "name": "我的插件"}
  ]
}
```

| Field | Type | Default | Description |
|------|------|--------|------|
| `show_library` | bool | `true` | Whether to show the library tab |
| `show_playlists` | bool | `true` | Whether to show the playlists tab |
| `plugin_tabs` | array | `[]` | Plugin tab list |

The home and settings tabs are always shown (not in the configuration); the optional items are the library, playlists, and plugin tabs.

**PUT validation rules**:
- The total number of optional items (`show_library` + `show_playlists` count as 1 each + the number of plugin tabs) must not exceed 3, plus the fixed home and settings for a total of 5
- The `entry_path` and `name` of each plugin tab cannot be empty
- `entry_path` cannot be duplicated
- **400**: Validation failed | **500**: Save failed

### 3.11 Remote Song Title Source

**`GET/PUT /api/v1/settings/remote-title-source`** -- `{"title_source": "filename"}` (belongs to `SongHandler`)

Controls the source of the remote song title during metadata refresh. Default `filename`.

| Value | Description |
|------|------|
| `tag` | During metadata refresh, overwrite with the title in the audio tags |
| `filename` | Keep the file name as the title, do not overwrite (default) |

- **400**: title_source is not `tag` or `filename` | **500**: Save failed

### 3.12 GitHub Update Proxy

**`GET/PUT /api/v1/settings/github-proxy`** -- `{"proxy": ""}` (belongs to `UpgradeHandler`)

The GitHub proxy prefix (e.g., `https://ghfast.top/`) used when checking for updates / upgrading. Empty string by default (direct connection). Persisted only; does not affect the global HTTP proxy of other modules (see 3.9).

- **400**: Bad request format | **500**: Save failed

### 3.13 Plugin Keep-Alive Whitelist

**`GET/PUT /api/v1/settings/plugin-keep-alive`** -- `{"plugins": []}` (belongs to `JSPluginHandler`)

A list of plugin `entryPath` values that will not be automatically put to sleep. Plugins in the whitelist are not unloaded even if idle for more than 10 minutes. Returns an empty list when not configured; takes effect immediately after saving.

- **400**: Bad request format | **500**: Save failed

### 3.14 Plugin Auto-Update

**`GET/PUT /api/v1/settings/plugin-auto-update`** -- `{"enabled": false}` (belongs to `JSPluginHandler`)

The "automatically update installed plugins in the background" switch, disabled by default. When enabled, the service checks once after a delay of several minutes following startup, and thereafter performs "check for updates + download and install + hot reload" every 6 hours for plugins that have a remote update source. The switch takes effect immediately, no restart required.

- **400**: Bad request format | **500**: Save failed

### 3.15 Library Browse Views

**`GET/PUT /api/v1/settings/library-browse`** -- The view visibility and order of the unified library browse page (belongs to `ConfigHandler`).

```json
{
  "views": [
    {"key": "all", "visible": true},
    {"key": "artist", "visible": true}
  ]
}
```

There are 14 view keys in total, in three groups:

- **Song group**: `all` (all) / `local` (local) / `remote` (remote) / `radio` (radio)
- **Category group**: `artist` (artist) / `album` (album) / `genre` (genre) / `year` (year) / `decade` (decade) / `language` (language) / `style` (style)
- **Playlist group**: `playlist` (all playlists) / `playlist_normal` (normal playlists) / `playlist_radio` (radio playlists)

Returns the default when not configured (all visible, default order). The response always contains the complete 14 items: illegal keys are removed, and missing keys are appended to the end in default order (`visible=true`).

- **400**: Illegal or duplicate view key | **500**: Save failed

### 3.16 User Preferences

**`GET/PUT /api/v1/settings/user-preferences`** -- Cross-device synced user preferences (belongs to `ConfigHandler`).

```json
{
  "theme_mode": "system",
  "play_mode": "order",
  "playlist_view_mode": "grid",
  "audio_quality": "original",
  "local_cache_max_size": 1073741824,
  "volume": 50.0
}
```

| Field | Type | Default | Description |
|------|------|--------|------|
| `theme_mode` | string | `"system"` | Theme mode |
| `play_mode` | string | `"order"` | Play mode |
| `playlist_view_mode` | string | `"grid"` | Playlist view mode |
| `audio_quality` | string | `"original"` | Audio quality |
| `local_cache_max_size` | int64 | `1073741824` | Local cache limit (bytes, default 1GB) |
| `volume` | float | `50.0` | Volume |

The client fetches these after login and pushes them when preferences are modified, achieving preference synchronization across multiple devices. Returns the default value when not configured.

- **400**: Bad request format | **500**: Save failed

### 3.17 Equalizer

**`GET/PUT /api/v1/settings/equalizer`** -- The global equalizer (EQ) configuration (belongs to `ConfigHandler`).

```json
{
  "enabled": false,
  "preset": "flat",
  "bands": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
}
```

| Field | Type | Default | Description |
|------|------|--------|------|
| `enabled` | bool | `false` | Whether it is enabled |
| `preset` | string | `"flat"` | Preset name (`flat` / `rock` / `pop` / `jazz` / `classical` / `bass_boost` / `treble_boost` / `vocal` / `custom`) |
| `bands` | float[] | all 0 | 10-band gains (31Hz-16kHz, in dB, range -12 ~ +12) |

Returns the default value when not configured (disabled + flat preset + all 0).

- **400**: `bands` is not 10 elements / a band exceeds the -12 ~ +12 range | **500**: Save failed
