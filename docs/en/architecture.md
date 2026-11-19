# Songloft Architecture Overview

Songloft is a self-hosted local music server built with a decoupled frontend/backend architecture. It supports two run modes: **Server Mode** (deploying the Go backend standalone) and **Bundle Local Mode** (shipping the Go backend with the client, with no separate server deployment). The Flutter and Lynx client repositories each build and publish their own Bundle packages from the latest root `main` commit at the start of the workflow. Root Releases publish backend, container, and server tools only.

Server Mode supports Flutter, ReactLynx, official Electron Desktop, and Android TV clients. Flutter supports Bundle on Android/iOS/macOS/Windows/Linux; Lynx supports Bundle on Android/iOS/Windows/macOS. Lynx desktop packages are published by the client repository in the shared workflow, with final device acceptance still in progress. The client architecture diagram below describes the Flutter client's own architecture.

## Architecture Documentation Navigation

- **[Backend Architecture](./architecture_backend.md)** - Detailed architecture of the Go backend API service
- **[Frontend Architecture](./architecture_frontend.md)** - Detailed architecture of the Flutter cross-platform frontend
- **[Choosing a Client](./clients.md)** - Flutter, Lynx, Electron Desktop, and TV platforms, capabilities, and downloads
- **[Lynx Client](./player-lynx/index.md)** - Installation, builds, testing, and independent releases
- **[Bundle Mode](./bundle-mode.md)** - Flutter/Lynx package sources, backend pins, and login/settings entry points
- **[Color System](./color_system.md)** - Material 3 color system and theming conventions
- **[Quick Start](./quick-start.md)** - Getting-started guide (generated in sync with README.md)

## Overall Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  Flutter Cross-Platform Frontend                             │
│  /clients/player (standalone repo: github.com/songloft-org/songloft-player) │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │ Android  │ │   iOS    │ │  macOS   │ │ Windows  │       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
│  ┌──────────┐ ┌──────────────────────────────────────┐      │
│  │  Linux   │ │  Web (embedded in Go binary / standalone deploy) │      │
│  └──────────┘ └──────────────────────────────────────┘      │
│  State: Riverpod  Routing: GoRouter  Audio: just_audio      │
│  Bundle Local Mode: embedded Go backend, no external server needed │
└──────────────────────────────────────────────────────────────┘
                        │
          ┌─────────────┴─────────────┐
          │                           │
   Server Mode (HTTP)         Bundle Local Mode
   JWT Bearer Token      (MethodChannel/subprocess)
          │               127.0.0.1:<port>
          │                           │
┌──────────────────────────────────────────────────────────────┐
│                   Go Backend (Chi v5)                        │
│  ┌──────────┐ ┌──────────┐ ┌────────────────────┐ ┌──────┐ │
│  │ Handlers │→│ Services │→│ Repository/UoW     │→│SQLite│ │
│  └──────────┘ └──────────┘ │  (sqlc + squirrel) │ └──────┘ │
│                            └────────────────────┘          │
│                            goose migrations auto Up on start │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                    │
│  │ JSPlugin │ │JS Runtime│ │  Cache   │                    │
│  │ Manager  │ │ (QuickJS)│ │ Service  │                    │
│  └──────────┘ └──────────┘ └──────────┘                    │
│  Static: embed.FS (Lynx Web) + SPA routing fallback      │
│  Monitoring: Tracely (heartbeat + install/upgrade stats + panic capture) │
└──────────────────────────────────────────────────────────────┘
                        │
┌──────────────────────────────────────────────────────────────┐
│              SQLite Database (modernc.org/sqlite)            │
│              Pure-Go CGO-free implementation, no external deps │
└──────────────────────────────────────────────────────────────┘
```

## Tech Stack Overview

### Backend

| Technology | Version | Description |
|------|------|------|
| Go | 1.26+ | Backend language |
| Chi | v5.2.4 | HTTP routing framework |
| SQLite | modernc.org/sqlite v1.49.1 | Pure-Go database driver |
| goose | v3 | SQL schema migrations (automatically applied with Up on startup) |
| sqlc | - | Generates type-safe Go code from static SQL (CLI) |
| squirrel | v1.5 | Dynamic SQL construction (variable-length WHERE/SET/ORDER) |
| JWT | golang-jwt/jwt v5 | Dual-token authentication |
| QuickJS | modernc.org/quickjs | JS runtime (JS plugin sandbox) |
| hanxi/tag | - | Audio metadata read/write |
| ffprobe | Optional | Audio technical parameters |
| Tracely | v1.1.3 | Monitoring reporting (heartbeat, install/upgrade stats, panic capture) |

### Flutter Frontend

| Technology | Version | Description |
|------|------|------|
| Flutter | 3.41+ | Cross-platform UI framework |
| Dart | 3.7+ | Programming language |
| Riverpod | ^3.1.0 | State management |
| GoRouter | ^17.1.0 | Declarative routing |
| Dio | ^5.7.0 | HTTP client |
| just_audio | ^0.10.5 | Audio playback engine |
| audio_service | ^0.18.17 | System notification-bar controls |
| Material 3 | - | UI design system |

## Project Directory Structure

```
songloft/
├── main.go                     # Main program entry point
├── web_embed.go                # Full build (embeds Lynx Web, build tag: !lite)
├── web_embed_lite.go           # Lite build (empty embed.FS, build tag: lite)
├── mobile/                     # gomobile binding entry (Bundle Local Mode)
│   └── mobile.go               # Exports Start/Stop/IsRunning/GetPort for mobile calls
├── Makefile                    # Build and test commands
├── go.mod                      # Go module definition (Go 1.26)
├── Dockerfile                  # Docker configuration
├── internal/                   # Backend core code
│   ├── app/                    # App initialization, route registration, static file serving, source adaptation
│   ├── config/                 # Configuration type definitions
│   ├── handlers/               # HTTP request handlers
│   ├── middleware/             # JWT authentication middleware
│   ├── models/                 # Data models and constants
│   ├── database/               # SQLite database layer (Repository + UnitOfWork + sqlc + goose migrations + testutil)
│   ├── services/               # Business logic layer (includes source/ subpackage: fetcher / resolver / validator / orchestrator / metrics)
│   ├── jsplugin/               # JS plugin management (lifecycle, health checks, hot updates)
│   ├── jsruntime/              # QuickJS JavaScript runtime
│   ├── httputil/               # Global proxy-aware HTTP Transport / Client
│   ├── tracelycfg/             # Tracely monitoring-report configuration
│   └── version/                # Version information
├── pkg/                        # Public packages
│   └── tag/                    # Audio metadata read/write library
├── clients/                    # Client collection
│   ├── player/                 # Flutter frontend (standalone subrepo)
│   │   └── lib/                # Dart source code
│   │       ├── config/         # API configuration, deployment mode
│   │       ├── core/           # Networking, routing, theming, storage, audio
│   │       ├── features/       # Feature modules (auth / startup / home / library / player / playlist / settings / jsplugin / dlna)
│   │       └── shared/         # Shared layouts, models, components
│   ├── player-lynx/            # Lynx client (submodule)
│   ├── player-build/           # Lynx Web build artifacts (go:embed)
│   └── tv/                     # Android TV client (submodule)
├── plugins/                    # Plugin ecosystem
│   ├── toolchain/              # JS plugin development toolchain (SDK + Builder + scaffolding)
│   └── src/                    # JS plugin source collection (submodules; build artifacts published in each plugin's GitHub Releases)
├── tools/                      # Standalone tools
│   ├── tracely/                # Observability monitoring tool (submodule, separate Go module)
│   └── ffmpeg-builder/         # FFmpeg builder (submodule)
├── integrations/               # Third-party integrations
│   └── home-assistant/         # Home Assistant add-on (submodule)
├── scripts/                    # Build and release scripts
└── docs/                       # Project documentation
```

<a id="build-system"></a>

## Build System

### Build Tags

| Command | Tag | Description |
|------|------|------|
| `make run` | `-tags dev` | Development mode, includes Swagger, embeds the frontend |
| `make build-prod` | No tag | Production full build (default), embeds Lynx Web |
| `make build-prod-lite` | `-tags lite` | Production lite build, without the frontend |
| `make build-go-mobile-android` | `-tags lite` | Compiles the Go backend into an Android .aar (gomobile bind) |
| `make build-go-mobile-ios` | `-tags lite` | Compiles the Go backend into an iOS .xcframework (gomobile bind) |
| `make build-go-desktop-{linux,windows,macos}` | `-tags lite` | Compiles the Go backend into a desktop executable (for Bundle mode) |

### Build Parameters and Embedding

- `VERSION` and `BUILD_TYPE` are orthogonal: `VERSION=dev` automatically enables `-tags dev` (Swagger + pprof), while releases use `X.Y.Z`; `BUILD_TYPE=lite` embeds no frontend, and empty means full. **Do not** use mixed values such as `BUILD_TYPE=dev`
- The frontend embed path is always `clients/player-build/web-embedded` (**not** `clients/player/build/web-embedded`)
- SPA fallback returns `index.html` only for missing paths whose final segment contains no `.`; missing static assets must return 404, never HTML masquerading as JS/JSON
- Sub-path deployment: pass `-base-path /xxx` or `BASE_PATH=/xxx` at startup; the entry point strips the prefix with `http.StripPrefix`, `embed.go` replaces `<base href="/">` with `<base href="/xxx/">` at runtime, and the Lynx host injects its deployment directory through `webBaseUrl` for API, Worker, and static URLs
- Full builds use Node.js 22 + pnpm 11.15.1 for Lynx Web, apply engine patches explicitly, and precompress assets; `embedded-build.json` records client and server identities separately

### Frontend Build

```bash
make build-frontend-web-embedded   # Embedded mode (hides API address UI)
make build-frontend-web            # Standalone deployment build
make build-frontend-all            # All platforms supported by the current system
```

<a id="docker-entrypoint"></a>

## Docker Entry and Hot Swap

The image contains a base package at `/app/songloft`, while the persistent data volume runs `/app/data/songloft`. On startup, `scripts/docker-entrypoint.sh` decides whether to overwrite data with the base package:

**Core principle: the base package represents the user's intent; when the dev/release channel or the full/lite variant differs, overwrite with the base package. Only compare old vs. new when "same channel + same BUILD_TYPE": dev by Build Time, release by version number.**

Earlier branches: initialize directly when the target file is missing; an unknown base channel preserves the target, while an unknown target channel is replaced by a recognized base; when both are stable but either version fails the `is_semver` check (such as a custom tag), use the base without comparing versions.

| Scenario | Behavior | Reason |
|------|------|------|
| dev ↔ release channel differs | Replace | The user switched image channels |
| BUILD_TYPE differs (full↔lite) | Replace | The user switched image variants |
| Both dev + same type + base Build Time > data Build Time | Replace | Dev rolling builds pick the newest by build time |
| Both dev + same type + data Build Time >= base Build Time | Don't replace | Data may have been upgraded online via the API |
| Both release + same type + base version > data version | Replace | Release upgrade |
| Both release + same type + data version >= base | Don't replace | Data may have been upgraded online via the API |

See the Docker sections in the [quick start](quick-start.md) for non-root operation (`PUID`/`PGID`), the `/app/data` and `/app/music` chown strategy, and `FIX_MUSIC_PERMISSIONS`. Leaving them unset keeps root operation, identical to prior versions.

## Technical Highlights

### Backend

1. **Pure-Go implementation**: Audio metadata extraction, the SQLite driver, and the QuickJS runtime are all pure-Go implementations, requiring no CGO, making deployment simple
2. **Bundle Local Mode**: Embeds the Go backend into the Flutter client via gomobile (mobile) or a subprocess (desktop), allowing use without deploying a separate server
3. **JS plugin system**: A QuickJS-based script plugin architecture that supports dynamically extending music-source capabilities, with sandbox isolation + permission model + health checks + hot updates
4. **JWT dual tokens**: Access Token + Refresh Token, supporting token revocation and management
5. **Music caching**: When playing remote songs, streams a proxy to the client while caching in the background asynchronously, with an LRU eviction policy and support for custom cache directory and capacity limit
6. **Audio tag read/write**: pkg/tag extends the original dhowden/tag with multi-format writing (MP3 / FLAC / M4A·MP4 / OGG(.ogg/.oga) / APE / WAV / AIFF), pure Go with no external dependencies
7. **Resource proxy**: Built-in CORS proxy with SSRF protection
8. **Database-driven configuration**: Configuration is stored in SQLite, supporting JSON format and dynamic updates via the API
9. **Tracely monitoring**: Heartbeat packets, install/upgrade statistics, panic capture
10. **Library faceted browsing**: `GET /api/v1/songs/facets` aggregates dimensions such as artist/album, paired with `/api/v1/settings/library-browse` to configure browsing behavior
11. **Video-container scanning**: Supported scan formats now include video containers (mp4/mov/mkv/webm/avi/ts); files containing a video track are probed with ffprobe and flagged via `songs.is_video`

### Frontend

1. **Consistent cross-platform experience**: A single codebase adapts to 6 platforms
2. **Bundle Local Mode**: Embeds the Go backend, supporting local/remote dual-mode switching, allowing local music playback without a server
3. **Three-form responsive layout**: Mobile / Tablet / Desktop adaptive
4. **Feature-First architecture**: Organized by feature module, each containing data / domain / presentation
5. **Audio playback**: just_audio + audio_service, supporting notification-bar controls and background playback
6. **Lyrics display**: LRC lyrics parsing and synchronized display
7. **Cover color extraction**: Extracts the dominant color from the cover image for dynamic theming
8. **TV client**: Recommend [songloft-tv](https://github.com/songloft-org/songloft-tv)

## Database Design

### Table Schema

| Table | Description | Key Fields |
|------|------|---------|
| **songs** | Songs/radio | type(local/remote/radio), title, artist, album, duration, file_path, url, cover_path, lyric, lyric_source, plugin_entry_path, source_data, dedup_key, cache_path, is_video |
| **playlists** | Playlists | type(normal/radio), name, labels, cover_path, cover_url |
| **playlist_songs** | Playlist-song associations | playlist_id, song_id, position |
| **configs** | System configuration | key(unique), value(JSON) |
| **auth_tokens** | Authentication tokens | token_id, token_type(access/refresh), expires_at, revoked_at |
| **js_plugins** | JS plugin info | name, version, entry_path, main, permissions, file_path, status(active/inactive/error), zip_hash, entry_hash |

### Index Design

- Songs: type, title, artist, added time; (plugin_entry_path, dedup_key) partial unique index (`WHERE dedup_key != ''`), used to deduplicate imports of network songs by plugin identity
- Playlists: type, labels
- Playlist songs: playlist_id, position
- Configs: key
- Tokens: token_id, token_type, expires_at, revoked_at
- JS plugins: status, entry_path

### Triggers

All tables are configured with an `updated_at` auto-update trigger.

### Seed Data

- Built-in playlists: Favorites (id=1), Radio Favorites (id=2), both with `labels=["built_in"]`
- Default configuration: `music_path`, `cover_storage_path`, `scan_config`, `ffprobe_path`, `jwt_secret`, `source_validation`, `source_fallback`, `source_metrics`
- `music_cache_config` and similar are not preseeded in migrations; they are written on demand by the corresponding service on first use

## Extensibility Design

### Easy to Add New Features
- New API: add a method in the corresponding handler → register the route in `routers.go`
- New model: define it in `models/` → implement CRUD in `database/`
- New service: implement it in `services/` → inject it into the handler via the constructor
- New plugin: extend via the JS plugin system (scaffolding: `pnpm create songloft-plugin`), no need to modify host code

### Easy to Test
- The database layer uses `database/testutil.OpenMemoryDB(t)` to spin up a `:memory:` SQLite + real Repository, avoiding hand-written mocks (which have been uniformly removed)
- Services inject interfaces rather than concrete types, separating business logic from HTTP handling
- Each module has a single responsibility
- Complete unit tests and integration tests

### Easy to Maintain
- The `internal/` directory prevents external dependencies on internal implementations
- Low coupling between modules
- Follows Go community standards and conventions
- The plugin system is decoupled from core functionality
