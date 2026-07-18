# Scan Management API

This document is based on the following source files:

- `internal/handlers/scan.go` -- Scan handler (scan actions + business settings endpoints)
- `internal/app/routers.go` -- Route registration
- `internal/services/scan_progress.go` -- Scan progress model
- `internal/services/fingerprint.go` -- Fingerprint computation service

## Table of Contents

1. [Overview](#1-overview)
2. [Scan Operation Endpoints](#2-scan-operation-endpoints)
   - [POST /scan -- Scan and Import Local Music](#21-post-scan----scan-and-import-local-music)
   - [GET /scan/progress -- Get Scan Progress](#22-get-scanprogress----get-scan-progress)
   - [POST /scan/cancel -- Cancel Scan](#23-post-scancancel----cancel-scan)
   - [GET /scan/directories -- Get Subdirectory List](#24-get-scandirectories----get-subdirectory-list)
   - [GET /scan/dir-names -- Get All Directory Names](#25-get-scandir-names----get-all-directory-names)
3. [Fingerprint Computation Endpoints](#3-fingerprint-computation-endpoints)
   - [GET /scan/fingerprints/status -- Get Fingerprint Computation Status](#31-get-scanfingerprintsstatus----get-fingerprint-computation-status)
   - [POST /scan/fingerprints -- Trigger Batch Fingerprint Computation](#32-post-scanfingerprints----trigger-batch-fingerprint-computation)
   - [GET /scan/fingerprints/progress -- Get Fingerprint Computation Progress](#33-get-scanfingerprintsprogress----get-fingerprint-computation-progress)
4. [Scan Business Settings Endpoints](#4-scan-business-settings-endpoints)
   - [GET/PUT /settings/music-path -- Music Path Configuration](#41-getput-settingsmusic-path----music-path-configuration)
   - [GET/PUT /settings/scan-playlist-mode -- Playlist Merge Mode](#42-getput-settingsscan-playlist-mode----playlist-merge-mode)
   - [GET/PUT /settings/scan-auto-create-playlists -- Auto-Create Playlists Switch](#43-getput-settingsscan-auto-create-playlists----auto-create-playlists-switch)
   - [GET/PUT /settings/scan-title-source -- Scan Title Source Configuration](#44-getput-settingsscan-title-source----scan-title-source-configuration)
   - [GET/PUT /settings/auto-scan -- Auto Scan Configuration](#45-getput-settingsauto-scan----auto-scan-configuration)

---

## 1. Overview

The scan management module handles the discovery, importing, and directory management of local music files. All endpoints require JWT authentication (`BearerAuth`), with the base path `/api/v1`.

`ScanHandler` carries both the scan action endpoints and the scan-related business settings endpoints (`/settings/*`), consolidating the business-oriented reads/writes of config keys such as `music_path` and `scan_playlist_mode` into a single handler.

---

## 2. Scan Operation Endpoints

### 2.1 POST /scan -- Scan and Import Local Music

**Method:** `POST`
**Path:** `/api/v1/scan`
**Authentication:** BearerAuth required

**Description:** Asynchronously scans the music directory and imports newly discovered music files into the database. Returns immediately; status can be polled via `/scan/progress`.

**Request body:**

```json
{
  "reimport": false
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `reimport` | boolean | No | Whether to re-import files that already exist (default false) |

**Success response (200):**

```json
{
  "message": "扫描任务已启动"
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 409 | A scan is already in progress |
| 500 | Failed to start scan |

---

### 2.2 GET /scan/progress -- Get Scan Progress

**Method:** `GET`
**Path:** `/api/v1/scan/progress`
**Authentication:** BearerAuth required

**Description:** Gets progress information for the current scan task.

**Success response (200):**

```json
{
  "status": "scanning",
  "total_files": 1000,
  "scanned_files": 500,
  "imported_files": 480,
  "skipped_files": 15,
  "failed_files": 5,
  "cleaned_files": 0,
  "current_file": "/music/album/track.mp3",
  "start_time": "2026-06-12T10:00:00Z",
  "end_time": null,
  "error": ""
}
```

| Field | Type | Description |
|------|------|------|
| `status` | string | Current status: `idle` / `scanning` / `importing` / `creating_playlists` / `completed` / `failed` / `cancelling` / `cancelled` |
| `total_files` | int | Total number of files |
| `scanned_files` | int | Number of scanned files |
| `imported_files` | int | Number of imported files |
| `skipped_files` | int | Number of skipped files (already exist) |
| `failed_files` | int | Number of failed files |
| `cleaned_files` | int | Number of stale files cleaned up |
| `current_file` | string | Path of the file currently being processed |
| `start_time` | string/null | Start time (ISO 8601) |
| `end_time` | string/null | End time (ISO 8601) |
| `error` | string | Error message |

---

### 2.3 POST /scan/cancel -- Cancel Scan

**Method:** `POST`
**Path:** `/api/v1/scan/cancel`
**Authentication:** BearerAuth required

**Description:** Cancels the scan task in progress.

**Success response (200):**

```json
{
  "message": "扫描任务已取消"
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 400 | No scan task is in progress |

---

### 2.4 GET /scan/directories -- Get Subdirectory List

**Method:** `GET`
**Path:** `/api/v1/scan/directories`
**Authentication:** BearerAuth required

**Description:** Returns the list of first-level subdirectories under the specified path, used for lazy loading of the directory tree. When `path` is empty, returns the subdirectories under the music root directory.

**Query parameters:**

| Parameter | Type | Required | Description |
|------|------|------|------|
| `path` | string | No | Directory path (uses the music root directory when empty) |

**Success response (200):**

```json
{
  "directories": ["/music/pop", "/music/rock"],
  "root": "/music"
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 400 | The path must be under the music directory (prevents directory traversal attacks) |
| 500 | Failed to read directory |

---

### 2.5 GET /scan/dir-names -- Get All Directory Names

**Method:** `GET`
**Path:** `/api/v1/scan/dir-names`
**Authentication:** BearerAuth required

**Description:** Recursively collects all unique directory names under the music directory, returned in alphabetical order, used for autocompletion of excluded directory names.

**Success response (200):**

```json
{
  "names": ["@eaDir", "Classical", "Pop", "Rock", "tmp"]
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 500 | Failed to collect directory names |

---

## 3. Fingerprint Computation Endpoints

### 3.1 GET /scan/fingerprints/status -- Get Fingerprint Computation Status

**Method:** `GET`
**Path:** `/api/v1/scan/fingerprints/status`
**Authentication:** BearerAuth required

**Description:** Returns ffmpeg chromaprint availability and local song fingerprint computation statistics.

**Success response (200):**

```json
{
  "chromaprint_available": true,
  "total": 1000,
  "computed": 800,
  "missing": 200
}
```

| Field | Type | Description |
|------|------|------|
| `chromaprint_available` | boolean | Whether ffmpeg chromaprint is available |
| `total` | int | Total number of local songs |
| `computed` | int | Number of songs with computed fingerprints |
| `missing` | int | Number of songs missing fingerprints |

---

### 3.2 POST /scan/fingerprints -- Trigger Batch Fingerprint Computation

**Method:** `POST`
**Path:** `/api/v1/scan/fingerprints`
**Authentication:** BearerAuth required

**Description:** Asynchronously computes audio fingerprints for local songs; requires ffmpeg with chromaprint support. If a task is already running, it is interrupted and restarted.

**Request body:**

```json
{
  "recompute_all": false
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `recompute_all` | boolean | No | When true, clears existing fingerprints and recomputes all (default false, computes only the missing ones) |

**Success response (200):**

```json
{
  "status": "started",
  "total": 200
}
```

**Error responses:**

| Status | Description |
|--------|------|
| 400 | ffmpeg chromaprint is unavailable |

---

### 3.3 GET /scan/fingerprints/progress -- Get Fingerprint Computation Progress

**Method:** `GET`
**Path:** `/api/v1/scan/fingerprints/progress`
**Authentication:** BearerAuth required

**Description:** Queries the progress of the current fingerprint computation task.

**Success response (200):**

```json
{
  "status": "running",
  "computed": 150,
  "total": 200,
  "failed": 3
}
```

| Field | Type | Description |
|------|------|------|
| `status` | string | Task status: `idle` / `running` / `done` |
| `computed` | int | Number computed |
| `total` | int | Total number of tasks |
| `failed` | int | Number failed |

---

## 4. Scan Business Settings Endpoints

The following endpoints are business-oriented configuration interfaces that provide strongly-typed JSON, come with built-in defaults, and trigger side effects inline after PUT. They write to the same config row as the generic `/configs/{key}` interface, but frontend business features should always go through these endpoints.

### 4.1 GET/PUT /settings/music-path -- Music Path Configuration

**Path:** `/api/v1/settings/music-path`
**Authentication:** BearerAuth required

#### GET -- Get music path and scan exclusion configuration

**Success response (200):**

```json
{
  "path": "music",
  "exclude_dirs": ["@eaDir", "tmp"],
  "exclude_paths": []
}
```

| Field | Type | Description |
|------|------|------|
| `path` | string | Music directory path (default `"music"`) |
| `exclude_dirs` | string[] | List of excluded directory names |
| `exclude_paths` | string[] | List of excluded specific paths |

#### PUT -- Update music path and scan exclusion configuration

After writing the configuration, asynchronously triggers a Scanner rebuild + cleanup of songs in excluded directories (the side effect is consistent with the admin `/configs/music_path` PUT).

**Request body:** Same format as the GET response. `path` cannot be empty.

**Error responses:**

| Status | Description |
|--------|------|
| 400 | Bad request format or empty path |
| 500 | Failed to save configuration |

---

### 4.2 GET/PUT /settings/scan-playlist-mode -- Playlist Merge Mode

**Path:** `/api/v1/settings/scan-playlist-mode`
**Authentication:** BearerAuth required

Controls the directory merge mode when automatically creating playlists after a scan. Default `directory`.

#### GET response / PUT request body:

```json
{
  "mode": "directory"
}
```

| Field | Type | Allowed values | Description |
|------|------|--------|------|
| `mode` | string | `directory` / `top_level` / `bubble_up` | `directory`: generate an independent playlist for each folder (default); `top_level`: merge playlists by first-level subdirectory; `bubble_up`: songs appear simultaneously in the playlists of all parent folders |

**PUT error responses:**

| Status | Description |
|--------|------|
| 400 | Bad request format or illegal `mode` value (only the three enum values above are accepted) |
| 500 | Failed to save configuration |

---

### 4.3 GET/PUT /settings/scan-auto-create-playlists -- Auto-Create Playlists Switch

**Path:** `/api/v1/settings/scan-auto-create-playlists`
**Authentication:** BearerAuth required

Controls whether playlists are automatically created based on the music directory structure after a scan completes. Enabled by default (`true`). When disabled, scanning only imports songs into the database and no longer auto-creates playlists.

#### GET response / PUT request body:

```json
{
  "enabled": true
}
```

---

### 4.4 GET/PUT /settings/scan-title-source -- Scan Title Source Configuration

**Path:** `/api/v1/settings/scan-title-source`
**Authentication:** BearerAuth required

Configures the source of the song title during scanning. After switching, a scan in "re-import" mode is required for it to take effect. Triggers a Scanner rebuild after PUT completes.

#### GET response / PUT request body:

```json
{
  "title_source": "tag"
}
```

| Field | Type | Allowed values | Description |
|------|------|--------|------|
| `title_source` | string | `tag` / `filename` | `tag`: prefer the title in the audio tags (default); `filename`: always use the file name (without extension) |

**PUT error responses:**

| Status | Description |
|--------|------|
| 400 | Bad request format or invalid `title_source` value |
| 500 | Failed to save configuration |

---

### 4.5 GET/PUT /settings/auto-scan -- Auto Scan Configuration

**Path:** `/api/v1/settings/auto-scan`
**Authentication:** BearerAuth required

Configures the enabled state and scan interval of auto scan. Takes effect immediately after an update (no restart required), asynchronously triggering the auto-scan scheduler to reconfigure.

#### GET response / PUT request body:

```json
{
  "enabled": false,
  "interval_seconds": 3600
}
```

| Field | Type | Description |
|------|------|------|
| `enabled` | boolean | Whether to enable auto scan (default `false`) |
| `interval_seconds` | int | Scan interval in seconds (default 3600, valid range 60-86400) |

**PUT error responses:**

| Status | Description |
|--------|------|
| 400 | Bad request format or `interval_seconds` not in the range 60-86400 |
| 500 | Failed to save configuration |

---

**Section source:** `internal/handlers/scan.go` / `internal/app/routers.go` / `internal/services/scan_progress.go` / `internal/services/fingerprint.go`
