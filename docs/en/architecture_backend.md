# Songloft Backend Architecture

## Tech Stack

- **Go version**: 1.26+
- **Web framework**: Chi v5.2.4
- **Authentication**: JWT dual-token authentication (Access Token + Refresh Token)
- **Database**: SQLite 3 (modernc.org/sqlite v1.46.1, pure-Go CGO-free implementation)
- **Database access stack**:
  - `pressly/goose v3` — schema migrations (auto `Up` on startup, files in `migrations/000N_xxx.sql`)
  - `sqlc-dev/sqlc` — generates type-safe code from fixed SQL (`queries/*.sql` → `sqlc/*.sql.go`, generated at CLI time)
  - `Masterminds/squirrel v1.5` — dynamic SQL construction (variable-length WHERE/SET/ORDER/pagination)
  - Self-built Repository + UnitOfWork layer; transactions done via `db.RunInTx(ctx, func(ctx, *UnitOfWork))`
- **Metadata read/write**: hanxi/tag (a dhowden/tag fork, with enhanced encoding detection; adds multi-format writing: MP3 / FLAC / M4A·MP4 / OGG(.ogg/.oga) / APE / WAV / AIFF)
- **Audio analysis**: ffprobe (optional, used to obtain precise technical parameters)
- **JS runtime**: QuickJS (modernc.org/quickjs, pure-Go implementation, used for JS plugin script execution)
- **Plugin architecture**: JS script plugins (QuickJS sandbox + permission model + health checks + hot reload)
- **Monitoring**: Tracely client (heartbeats, install/upgrade statistics, panic capture)

## Architecture Design

### Layered Architecture

```
HTTP Server (main.go)
  → Config (internal/config/)
  → Routes + Middleware (internal/app/routers.go + internal/middleware/)
  → Handlers (internal/handlers/)
  → Services (internal/services/)
        │
        ├── Database main path
        │     → Repository / UnitOfWork (internal/database/*_repository.go, unit_of_work.go)
        │     → sqlc fixed SQL (internal/database/sqlc/) + squirrel dynamic SQL
        │     → SQLite (data/songloft.db, schema managed by goose migrations)
        │
        └── JS plugin path (on demand)
              → JS Plugin Manager (internal/jsplugin/)
              → JS Runtime — QuickJS sandbox (internal/jsruntime/)
```

> Services and Database form the core data flow; JS plugins are a side-chain capability (HTTP is forwarded to `jsplugin.Manager` and then enters the QuickJS sandbox), not on the main write path.

## Package Structure

### `internal/` directory

Holds the project's core business logic, organized by functional module:

#### app/ - Application entry and configuration

- `app.go`: The application's main struct (`App`) and initialization logic, including dependency injection, service creation, and signal handling
- `routers.go`: Route configuration and registration, defining all API routes and the middleware chain
- `router_dev.go`: Development routes (includes Swagger, `-tags dev`)
- `router_prod.go`: Production routes (excludes Swagger)
- `embed.go`: Flutter Web frontend static asset serving, with SPA route fallback (returns `index.html` when the requested file does not exist)
- `access_log.go`: HTTP access-log middleware
- `compress.go`: Response compression middleware
- `db_migration.go`: Wrapper that runs goose database migrations on startup
- `pprof_dev.go`: pprof endpoints in development mode (`-tags dev`)

> The legacy `/music/*` and `/cover/*` Base62 short-link scheme has been fully retired in favor of `/api/v1/songs/{id}/play|cover|lyric`; the related `embed_common.go` helper was removed along with the routes, and `routers.go` only keeps a deprecation comment.
- `source_adapters.go`: Adapts the services-layer implementations to the interfaces defined in the `services/source/` subpackage (fetcher / resolver / validator, etc.)

#### config/ - Configuration type definitions

- `types.go`: The application configuration struct `AppConfig` (port, database path, username/password, etc.)

#### handlers/ - Request handlers

- `auth.go`: Authentication-related requests (login, token refresh, logout, token management)
- `music.go`: Song CRUD, bulk deletion, lyric updates
- `playlist.go`: Playlist CRUD, song ordering, cover upload, auto-created playlists
- `config.go`: Configuration management
- `scan.go`: Scan management (async scanning, progress query, scan cancellation)
- `jsplugin.go`: JS plugin management (upload `.jsplugin.zip`, enable/disable, delete, check for updates)
- `jsplugin_registry.go`: JS plugin subscription-source management (source list read/write, fetch source manifest, download ZIP to install)
- `upgrade.go`: Version upgrades (check for updates, perform upgrade, reset base image)
- `proxy.go`: Resource proxying (works around external CDN CORS restrictions, supports streaming forwarding and Range requests). Includes `ServeRemoteResourceWithCache`, which streams upstream audio to the client and triggers background caching
- `hls.go`: HLS radio proxy (server-side fetches and rewrites the m3u8, proxies segments/key/init segments; `/settings/hls-proxy` toggle)
- `cache.go`: Music cache management (statistics, cleanup, configuration, custom directory validation)
- `backup.go`: Data backup and restore (playlist/song export and import)
- `log.go`: Log-level read/write (`/settings/log-level`)
- `equalizer_setting.go` / `library_browse_setting.go` / `tab_config_setting.go` / `user_preferences_setting.go`: Isolated config endpoints (strongly-typed `/settings/*` configuration)
- `version.go`: Version information
- `health.go`: Health checks
- `response.go`: Utility functions for unified JSON responses and error responses

#### middleware/ - Middleware

- `auth.go`: JWT authentication middleware, validates the Access Token
- `auth_test.go`: Authentication middleware tests

#### models/ - Data models

- `models.go`: Core data structures (Song, Playlist, Config, AuthToken, JSPlugin, etc.) and validation logic
- `constant.go`: Pagination limit constants (DefaultPaginationLimit, MaxPaginationLimit)
- `models_test.go`: Model validation tests

#### database/ - Database layer (Repository + UnitOfWork + sqlc + goose)

- `database.go`: The `DB` interface (`Close / RunInTx / each *Repository()` getter)
- `sqlite.go`: The `SQLiteDB` implementation (`Open()` runs goose Up plus WAL/busy_timeout and other pragmas; `RunInTx` transaction wrapper)
- `unit_of_work.go`: The `UnitOfWork` struct, a set of Repositories scoped to a transaction (the `Songs / Playlists / PlaylistSongs` fields, all bound to the same `*sql.Tx`)
- `errors.go`: Domain errors (sentinels such as `ErrNotFound` / `ErrConflict`)
- `filters.go`: Shared squirrel helpers (sort whitelist, `applyOrder`, `applyPagination`)
- `config_repository.go`: Config repository (`ConfigRepository`)
- `song_repository.go`: Song repository (includes `UpsertRemoteSong`: reuses an existing ID on a `(plugin_entry_path, dedup_key)` hit, and falls back to a direct INSERT when dedup_key is empty)
- `playlist_repository.go`: Playlist repository
- `playlist_song_repository.go`: Playlist-song association repository (includes `ReplaceSong`, etc.)
- `token_repository.go`: Authentication token repository
- `jsplugin_repository.go`: JS plugin repository
- `plugin_storage_repository.go`: JS plugin KV storage repository (backend store behind the `host.storage` bridge)
- `migrations/`: goose migration source files (`0001_init.sql`, etc., bundled via `embed.FS` and auto-Up'd on startup)
- `queries/`: sqlc inputs (one `*.sql` per table; run `make sqlc` to generate code)
- `sqlc/`: sqlc outputs (`*.sql.go`, **checked into the repo**, no sqlc CLI dependency at runtime)
- `testutil/`: `OpenMemoryDB(t)` starts a `:memory:` SQLite that runs real migrations plus real Repositories, for use in tests
- `sqlite_test.go`: Database-layer integration tests

#### services/ - Business logic layer

- `auth_service.go`: Authentication service (JWT dual-token generation/verification, token management, secret generation)
- `config_service.go`: Configuration service (database config management, supports reading/writing JSON format)
- `metadata.go`: Metadata extraction service (uses hanxi/tag to extract tags and covers, ffprobe for technical parameters). Title strategy: prefer the tag's title when present, and fall back to the filename only when it's missing (no more longest-common-substring concatenation)
- `scanner.go`: File scanning service (recursively scans the music directory, supports directory exclusion and format filtering)
- `scan_progress.go`: Scan progress tracking (async scan state management)
- `song_service.go`: Song service (CRUD, bulk operations, duration backfill)
- `playlist_service.go`: Playlist service (CRUD, song management, auto-creation)
- `upgrade_service.go`: Version upgrade service (fetch version info, perform upgrade, reset)
- `cache_service.go`: Music cache service (LRU eviction, custom cache directory, capacity limit configuration)
- `cache_service_song.go`: Song-level helpers for the cache service (hit lookup, concurrent-download deduplication, associated cleanup, streaming-proxy callbacks, etc.)
- `cache_path_template.go`: Path template rendering (placeholders such as `{artist}-{album}/{title}`, used by caching and plugin persistence)
- `cache_metadata_writer.go`: File metadata embedding (tag writing + remote cover fetching, used by plugin persistence)
- `song_downloader.go`: Song persistence service (plugin infrastructure: persists remote songs to the local `music_path` via the `songs.download` Bridge API)
- `internal_url.go`: Internal loopback URL construction (assembles relative URLs into `http://127.0.0.1:{port}/...?access_token=...`, for convert/cache to call plugins)
- `whitelist.go`: Domain whitelist validation (SSRF protection, blocks access to intranet addresses)
- `source/`: Audio source adapter subpackage — `fetcher` (HTTP data retrieval + URL parsing), `resolver` (cross-plugin fallback), `validator` (parameter validation), `orchestrator` (orchestration, includes `ResolveURL` which only resolves without downloading), `metrics` (metrics). See the interface bindings in `internal/app/source_adapters.go` for the concrete implementations

#### jsplugin/ - JS plugin management layer

- `plugin.go`: JS plugin runtime model and state machine
- `manager.go`: JS plugin manager (lifecycle, async loading, sub-route registration)
- `loader.go`: Unpacks `.jsplugin.zip` / validates the manifest / parses permissions
- `package.go`: Install/update/uninstall flow (includes hash verification)
- `repository.go`: Repository interface (implementation in `database/jsplugin_repository.go`)
- `registry.go`: Plugin subscription-source resolution (fetch source manifests, version comparison; used by update checks/installs)
- `auto_update.go`: Plugin auto-update (background checks and upgrades against subscription sources)
- `api_bridge.go`: Host API bridge entry point (exposes http, storage, logger, songs (incl. `songs.download` persistence), playlists, etc. to QuickJS)
- `api_bridge_net.go`: `songs.net` UDP Socket API (udpBind/multicast + reader goroutine pushing onData)
- `api_bridge_tcp.go`: `songs.net.tcpConnect` outbound TCP Socket API (private/loopback/link-local addresses only, SSRF protection)
- `api_bridge_websocket.go`: WebSocket client API (connect/send/receive/event push)
- `api_bridge_fs.go`: Filesystem bridge (restricted read/write under permissions such as `fs:music`)
- `api_bridge_command.go`: External command invocation bridge
- `communication.go`: Host ↔ plugin call protocol wrapper (request/response serialization)
- `invoke.go`: Unified wrapper for calling plugin entry functions (with timeout and error normalization)
- `hash.go`: File fingerprint utility (used for hot_reload and package verification)
- `scheduler.go`: Scheduler (avoids VM concurrency races)
- `health.go`: Health checks (probes via `jsruntime.HealthProbe`, auto-isolates on failure)
- `hot_reload.go`: Hot reload (auto-reloads based on file hash fingerprints)
- `permissions.go`: Permission model validation
- `service.go`: Plugin instance service shell
- `routes.go`: Sub-route mounting
- `assets/`: Embedded plugin common assets (`common.css`/`common.js`/fonts, served via `/api/v1/jsplugin-assets/*` and auto-injected into plugin pages)

#### jsruntime/ - JavaScript runtime

- `runtime.go`: QuickJS runtime environment management (`JSEnv`), supports parallel calls, event collection, and timeout control
- `polyfill.go`: JS polyfill code (console, setTimeout/setInterval, Function.toString, etc.)
- `pendingjob.go`: Low-level `JS_ExecutePendingJob` calls (handles Promise microtasks)

#### version/ - Version information

- `version.go`: Version number, Git Commit, build time, build type (injected via `-ldflags`)

### `pkg/` directory

Holds reusable public packages:

#### tag/ - Audio metadata read/write library

- **Reading**: MP3 (ID3v1/ID3v2.2/2.3/2.4), FLAC, OGG/Vorbis, M4A/MP4, WAV, APE, AIFF, DSF formats; cover images, lyrics, encoding detection
- **Writing** (`WriteTag(filePath, opts)`, dispatched by extension, all using temp file + `os.Rename` atomic writes):

  | Format | Text fields | Lyrics | Cover |
  |------|---------|------|------|
  | MP3 | ID3v2.3 text frames | USLT | APIC |
  | FLAC | Vorbis Comment | LYRICS | PICTURE block |
  | M4A/MP4/M4B/MOV | iTunes atoms (©nam, etc.) | ©lyr | covr |
  | OGG(.ogg/.oga) | Vorbis Comment | LYRICS | METADATA_BLOCK_PICTURE (base64) |
  | APE | APEv2 text items | Lyrics | Cover Art (Front) |
  | WAV | RIFF LIST INFO | ICMT | Not supported (format limitation) |
  | AIFF/AIF | ID3v2.3 (ID3 chunk) + NAME/AUTH | USLT (ID3 chunk) | APIC (ID3 chunk) |

  - Other extensions return `ErrUnsupportedWrite`; callers degrade to a log entry and do not block the main flow
- Command-line tools: `cmd/tag`, `cmd/sum`, `cmd/check`

## Build System

### Build Tags

| Tag | Description | Purpose |
|------|------|------|
| `dev` | Development mode | Includes Swagger docs + pprof |
| `lite` | Lite mode | Does not embed the frontend, smaller binary |
| No tag | Full mode (default) | Embeds the Flutter Web build output into the binary |

### Frontend Embedding Mechanism

```
web_embed.go      (build tag: !lite)  → //go:embed all:songloft-player-build/web-embedded
web_embed_lite.go  (build tag: lite)   → empty embed.FS
```

## Design Patterns

### Dependency Injection

```go
// Inject dependencies through the constructor
func NewAuthHandler(authService *services.AuthService) *AuthHandler {
    return &AuthHandler{
        authService: authService,
    }
}
```

### Interface Abstraction

`database.DB` only exposes the transaction entry point and the per-Repository getters; all CRUD logic is pushed down into the Repositories:

```go
type DB interface {
    Close() error
    RunInTx(ctx context.Context, fn func(context.Context, *UnitOfWork) error) error

    SongRepository() *SongRepository
    PlaylistRepository() *PlaylistRepository
    PlaylistSongRepository() *PlaylistSongRepository
    ConfigRepository() *ConfigRepository
    TokenRepository() *TokenRepository
    JSPluginRepository() *JSPluginRepository
}
```

The service layer injects the `database.DB` interface; single-table writes go directly through `db.SongRepository().Create(...)`, while cross-table writes go through `db.RunInTx(ctx, func(ctx, uow *UnitOfWork) error { uow.Songs.Create(...); uow.PlaylistSongs.ReplaceSong(...) })`. See [database_migrations.md](database_migrations.md) for details.

> Tests no longer hand-write mocks; they uniformly use `database/testutil.OpenMemoryDB(t)` to spin up a `:memory:` SQLite that runs real migrations and real Repositories.

## API Design

The backend provides a RESTful API, mainly including:

- `/api/v1/auth/*` - Authentication endpoints (login, refresh, logout, token management)
- `/api/v1/songs/*` - Song management endpoints (CRUD, bulk deletion, lyric updates)
- `/api/v1/playlists/*` - Playlist management endpoints (CRUD, song ordering, cover upload, auto-creation)
- `/api/v1/configs/*` - Configuration management endpoints
- `/api/v1/jsplugins/*` - JS plugin management endpoints (upload `.jsplugin.zip`, enable/disable, delete, check for updates)
- `/api/v1/jsplugin/{entry_path}/*` - JS plugin runtime routes (registered by the plugin's main.js via the SDK Router)
- `/api/v1/scan/*` - Scan management endpoints (async scanning, progress query, cancellation)
- `/api/v1/upgrade/*` - Version upgrade endpoints (available only in Docker environments, includes reset)
- `/api/v1/proxy` - Resource proxy endpoint (works around CORS, includes SSRF protection)
- `/api/v1/cache-manage/*` - Music cache management (statistics/cleanup/configuration/directory validation)
- `/api/v1/settings/hls-proxy` - HLS radio proxy toggle (GET/PUT)
- `/api/v1/settings/http-proxy` - General HTTP proxy configuration (GET/PUT)
- `/api/v1/settings/music-path` - Music path and scan exclusions (GET/PUT)
- `/api/v1/settings/plugin-registries` - Plugin subscription source list (GET/PUT)
- `/api/v1/settings/log-level` - Log level (GET/PUT)
- `/api/v1/settings/scan-auto-create-playlists` - Whether to auto-create directory playlists after scanning (GET/PUT)
- `/api/v1/settings/scan-playlist-mode` - Directory playlist grouping mode: directory/top_level/bubble_up (GET/PUT)
- `/api/v1/version` - Version information endpoint
- `/api/v1/health` - Health check endpoint

In addition, music files, cover images, and lyrics are accessed via song-ID endpoints (which require the `access_token` query parameter for authentication):
- `/api/v1/songs/{id}/play` — Streams the audio (supports the local / remote / radio types + Range)
- `/api/v1/songs/{id}/cover` — Song cover (local songs are served by this backend; for network songs, `MarshalJSON` returns the original CDN URL directly)
- `/api/v1/songs/{id}/lyric` — Plain-text LRC lyrics

> The old `/music/*` and `/cover/*` Base62-encoded short-link scheme has been fully retired; the related helper was removed along with the routes (`routers.go` only keeps a deprecation comment).

For detailed API documentation, refer to the Swagger docs (visit `/swagger/index.html` in a development environment).
