# AGENTS.md

This document provides AI coding assistants with the **entry-point information** for the Songloft project: project structure, common commands, hard rules, and a summary of pitfalls. For content where the code itself is the source of truth (directory tree, dependencies, API tables, table schemas), read the code directly or the detailed docs linked below.

> **Detailed docs**:
> - Architecture: [Overview](docs/architecture.md) · [Backend](docs/architecture_backend.md) · [Frontend](docs/architecture_frontend.md)
> - Topics: [Database operations](docs/database_migrations.md) · [Color system](docs/color_system.md) · [API response format](docs/api_response.md) · [Quick start](docs/quick-start.md) · [Frontend gotchas](docs/en/frontend_gotchas.md)
> - Plugin development: see `plugin-toolchain/README.md` (separate repo)
> - Plugin registry authoring: [Plugin registry authoring guide](docs/plugin_registry.md)
> - API: after starting in dev mode, visit `/swagger/index.html`

---

## Project Overview

Songloft is a self-hosted local music server that supports both **server deployment** and **Bundle local mode** (embedding the Go backend into the client, so no separate server deployment is required). It has a multi-repo structure:

| Directory | Tech | Description |
|------|------|------|
| `/` | Go 1.26 + Chi v5 + SQLite | Backend API service (default port 58091, account admin/admin) |
| `/mobile` | Go + gomobile | Mobile binding entry point for the Go backend (for gomobile bind; exports Start/Stop/IsRunning/GetPort) |
| `/songloft-player` ([separate repo](https://github.com/songloft-org/songloft-player)) | Flutter 3.29+ / Dart 3.7+ | Cross-platform frontend (6 platforms), supports Bundle local mode |
| `/plugin-toolchain` ([separate repo](https://github.com/songloft-org/plugin-toolchain)) | TS + pnpm | JS plugin development toolchain (SDK / Builder / scaffolding) |
| `/jsplugins-src` | TS | JS plugin source code (a collection of submodules; each plugin distributes releases from its own repo) |
| `/pkg/tag` | Go | Audio metadata **read/write** library (extends the upstream tag library with MP3/FLAC writing) |
| `/addon` | HA add-on | Home Assistant add-on (thin layer reusing the Docker image). Design/pitfalls/release: see [addon/README.md](addon/README.md) |
| `/ffmpeg-builder` ([separate repo](https://github.com/hanxi/ffmpeg-builder)) | Docker | Minimal-image builder for statically compiled ffmpeg/ffprobe (submodule); used for download transcode / audio fingerprinting |
| `/tracely` ([separate repo](https://github.com/hanxi/tracely)) | Go + Vue | Self-hosted frontend monitoring backend (install/upgrade tracking); the backend reports via its Go SDK. The local dir is gitignored, the SDK dependency comes from go.mod |

---

## Common Commands

```bash
# Backend
make run            # Start (dev mode, with Swagger)
make build          # Build dev version (full, embeds frontend)
make build-lite     # Build dev version (slim, no embedded frontend)
make build-prod     # Build production version (full, embeds frontend)
make build-prod-lite # Build production version (slim, no frontend)
make test           # Test
make check          # fmt + vet + test
make sqlc           # Regenerate sqlc code (must run after editing queries/*.sql)
make swagger        # Regenerate API docs

# Frontend build (artifacts land in songloft-player-build/, for backend embedding or standalone deployment)
make build-frontend-web-embedded   # For embedding in the Go binary (hides the API-address UI)
make build-frontend-web            # Standalone web deployment
make build-frontend-{linux,windows,macos,android,ios,all}

# Bundle local mode (Go backend compiled into a mobile library / desktop executable)
make build-go-mobile-android       # Android .aar (gomobile bind, arm64 + arm)
make build-go-mobile-ios           # iOS .xcframework (gomobile bind, arm64, macOS only)
make build-go-desktop-linux        # Linux executable
make build-go-desktop-windows      # Windows .exe
make build-go-desktop-macos        # macOS x86_64
make build-go-desktop-macos-arm64  # macOS ARM64

# Frontend development
cd songloft-player && flutter run -d chrome          # standalone
cd songloft-player && flutter run -d chrome --dart-define=DEPLOY_MODE=embedded
```

---

## Database Conventions (Hard Rules)

> For the complete procedure see [docs/database_migrations.md](docs/database_migrations.md).

Access stack: **goose migrations + sqlc fixed SQL + squirrel dynamic SQL + Repository + UnitOfWork**.

- **Changing the schema** → `internal/database/migrations/000N_xxx.sql`, executed automatically by `goose.Up` at startup; **do not** manually `ALTER data/songloft.db`
- **Adding fixed SQL** → `database/queries/{table}.sql` + `make sqlc`; the generated output `database/sqlc/` must be committed
- **Dynamic SQL (variable-length WHERE/SET)** → use squirrel inside `*_repository.go`; do not concatenate strings
- **Cross-table writes** → `db.RunInTx(ctx, func(ctx, uow))` to obtain `uow.Songs/Playlists/...` under the same `*sql.Tx`; **do not** call `BeginTx` manually in the service layer, or you'll hit SQLITE_BUSY
- **Error semantics** → repository misses uniformly return `database.ErrNotFound`; services distinguish with `errors.Is`
- **Testing** → use `testutil.OpenMemoryDB(t)` to run a real `:memory:` DB + real Repository; **do not** hand-write a mockDB
- **Built-in data** → migrations preload playlists id=1 "Favorites" and id=2 "Radio Favorites" (`labels=["built_in"]`), plus default config for `music_path / jwt_secret / source_*`. Remember to subtract these when asserting row counts in tests

---

## Backend Coding Conventions

- Standard Go layout (`internal/` guards against external dependencies), Chi v5 routing, JWT dual-token
- Dependency injection: the service layer only receives Repository interfaces, **not** the `DB`
- Logging: standard library `slog`; HTTP errors: uniformly `respondError`
- **API response format**: return RESTful data directly, **no** `{code, data, message}` envelope; errors are uniformly `{"error","detail"}`. For the full spec see [docs/api_response.md](docs/api_response.md)
- No ORM: fixed SQL → sqlc, dynamic SQL → squirrel, cross-table writes → `RunInTx + UnitOfWork`
- Test files `*_test.go` live in the same directory as the source

---

## API Documentation Conventions (Hard Rules)

**Every handler method registered in `internal/app/routers.go` (including sub-registration functions such as `RegisterStaticRoutes` / `RegisterAPIRoutes`) must have swag annotations.** The backend API docs are generated by [swaggo/swag](https://github.com/swaggo/swag) from these annotations and are the single source of truth for frontend development and external integration.

### Required fields (every handler has at least these 7)

```go
// @Summary <one-line summary in Chinese>
// @Description <detailed description, may span multiple lines; clarify side effects / defaults / error-code trigger conditions>
// @Tags <business group, in Chinese>
// @Produce json
// @Success 200 {object} <return type> "<description>"
// @Security BearerAuth
// @Router /<path> [<method>]
func (h *XxxHandler) Method(w http.ResponseWriter, r *http.Request) { ... }
```

- Endpoints with a request body additionally add `@Accept json` and `@Param request body <type> true "<description>"`
- Endpoints with obvious error paths add `@Failure 400/404/500 {object} map[string]string "..."`
- Path/query parameters use `@Param <name> path/query <type> true/false "<description>"`
- **Public endpoints** (no token required, e.g. health checks) omit `@Security BearerAuth`
- **Business tag naming**: reuse existing tags (「歌曲管理」「歌单管理」「电台与 HLS」「扫描管理」「配置管理」「缓存管理」「JS 插件」「JS插件管理」「数据备份」「设置」「系统升级」「认证管理」「系统管理」「资源代理」); do not casually invent new tags

### Multi-alias / catch-all routes

- A handler registered under multiple alias paths (e.g. `/songs/{id}/play` and `/songs/{id}/play.m3u8`) → write one `@Router` line per alias
- HEAD is a subset of GET; **do not list it separately**; OpenAPI does not require it
- A catch-all like `r.HandleFunc(...)` that accepts ANY HTTP method → list all methods actually possible (`[get] [post] [put] [delete]`), one `@Router` line each
- Dynamic paths (`{entryPath}` determined at runtime by the installed plugins) → note in `@Description`: "dynamic route, {xxx} is determined at runtime, OpenAPI serves only as a placeholder"

### Must run after changes

After modifying/adding handler annotations you must run `make swagger`: it regenerates `docs/swagger.json`, `docs/swagger.yaml`, and `docs/docs.go`, and **these outputs must be committed**. Otherwise `/swagger/index.html` will be out of sync with the code, and the frontend will hit pitfalls integrating against the stale docs.

### Verification

- Search the `make swagger` output for your newly added `@Router` path, and confirm that `Generating <Type>` includes the request/response types you just wrote
- `grep '<your-new-path>' docs/swagger.json` should return a hit
- Start `make run`, visit `http://localhost:58091/swagger/index.html`, and open the new endpoint in the UI to eyeball it

### No exemptions

"Everything registered in routers must be annotated" is an absolute rule. Even dynamic-route catch-alls, static-resource handlers, and reverse-proxy endpoints must have swag — just make the `@Description` clear about "what it is and why the OpenAPI schema is imprecise."

---

## Configuration Endpoint Conventions (Hard Rules)

The project has two kinds of configuration endpoints. **User-visible feature toggles always go through business endpoints**, while the generic KV store is only an admin entry point.

### `/api/v1/settings/<name>` — Standalone config endpoints (frontend business features default here)

- Path style: `/settings/<kebab-case-name>` (e.g. `/settings/hls-proxy`, `/settings/music-path`, `/settings/http-proxy`, `/settings/library-browse`, `/settings/proxy-private-allowlist`)
- Data shape: **strongly typed** JSON (e.g. `{enabled: bool}` or an aggregate object), not `{value: string}`
- Default values: handled inside the handler (when config is missing, GET returns the business default; PUT just writes directly, **the frontend need not POST-create first**)
- Side effects: triggered directly inside PUT (e.g. after a `music_path` PUT, asynchronously `onMusicPathChanged` rebuilds the Scanner)
- Ownership: placed in the corresponding business module's handler (e.g. hls-proxy in `HLSHandler`, music-path in `ScanHandler`), which also holds `*services.ConfigService` to do the read/write
- Naming pattern: `Is<Name>Enabled() / Set<Name>Enabled(bool)` business methods + `Get<Name>Setting / Update<Name>Setting` HTTP handlers + `/settings/<name>` route

### `/api/v1/<module>/*` — Business module aggregate endpoints (config included)

Some business modules come with an "action endpoint + config endpoint" combo (the canonical example being `/cache-manage/{stats,clean,config}`); in this case the config endpoint **stays under the module prefix** rather than being forcibly split out into `/settings/`.

- When applicable: the config is strongly related to the module's other action endpoints (e.g. cache's `config` shares the same `CacheService` as `stats/clean`)
- Rationale: the industry mainstream (AWS, GitHub, Discord) all aggregate by business module; the GitLab-style hybrid of "globally centralized, module-dispersed" is also acceptable
- Existing example: `/api/v1/cache-manage/config` (GET/PUT)
- **Decision criteria**:
  - **Standalone** config (belongs to no business module, or is shared across modules) → `/settings/<name>`
  - **In-module** config (strongly related to the module's action endpoints) → `/<module>/config` or `/<module>/<sub-name>`

### `/api/v1/configs/{key}` — Generic KV (admin editor only)

- Only for use by frontend **generic config editors** like `config_manager.dart`, letting admins hand-edit arbitrary key/value for debugging
- **New business features must not call `/configs/{key}` directly**: the generic PUT returns 404 when the key doesn't exist, and it has no strong typing, no side effects, no default values
- After a business wrapper is in place, the generic endpoint can still modify the same key (dual entry points are retained), but the side effect must also be hooked into the `configHandler.SetOnConfigChanged` callback (see `musicPathChanged` in `routers.go`), ensuring both entry points have consistent semantics

### Client conventions

- `SettingsApi` (`songloft-player/lib/features/settings/data/settings_api.dart`) wraps all `/settings/*` calls; business-feature Providers always go through it
- `ConfigApi` is used only in `config_manager.dart` and admin UIs like "list all configs"

### Historical decision record

- This convention was introduced in 2026-06. Background: `hls_proxy_enabled` was not preloaded by default, causing PUT `/configs/{key}` to return 404, which revealed that the project had three coexisting styles — `/configs` + `/settings/*` + `/cache-manage/config`
- Chosen direction: business endpoints are the **single source** for user-visible entry points, and the generic KV degrades to an admin back door

---

## Bilingual Documentation Sync (Hard Rule)

Project docs are **maintained in both Chinese and English**. When you change either language version, you **must** apply the corresponding change to the other version — never edit one side only and let the two drift apart.

- **Mappings**:
  - `README.md` ↔ `README.en.md`
  - `AGENTS.md` ↔ `AGENTS.en.md`
  - `docs/<name>.md` ↔ `docs/en/<name>.md` (same filename; English version lives under `docs/en/`)
- **Criterion**: any add / modify / delete of documentation **content, structure, or links** (body text, sections, tables, navigation links, etc.) must land in both language versions; only the wording is localized, the structure stays consistent
- **Check the counterpart exists first**: if a same-named file exists under `docs/en/`, sync it; README and AGENTS always have an `.en.md` counterpart
- **Exception**: some content is inherently single-language (e.g. a community note only in the Chinese version) — no mirror is required, but make sure that is intentional rather than an omission

---

## Docs Site Structure (docs/ — VitePress Custom Theme)

The Songloft docs site (`docs/`) uses **VitePress + a custom theme** (`docs/.vitepress/theme/`), **not the default theme**. Before editing the docs site, first tell apart the two kinds of pages — editing the wrong place wastes the change:

- **Custom landing page (edit data, not markdown)**: the home page `docs/index.md` is a single line `<Landing />`; its content is driven by structured data in `docs/.vitepress/data/*.ts` (install methods `downloads.ts`, features `features.ts`, copy `landing-i18n.ts`) and rendered by `docs/.vitepress/theme/components/landing/*.vue`. To change the landing page → edit `data/*.ts` (bilingual `{zh,en}` fields); align icons with the mapping table inside the component (e.g. `ICONS` in `LandingInstaller.vue`).
- **Auto-generated pages (do NOT hand-edit)**: `docs/quick-start.md`, `docs/en/quick-start.md`, and `docs/changelog.md` are generated by `scripts/sync-docs.mjs` from the root `README.md` / `README.en.md` / `CHANGELOG.md`, and are ignored via `docs/.gitignore`. To change the body → edit the source `README` / `CHANGELOG`; `docs:dev` / `docs:build` runs `sync` first to regenerate. **Manual edits get overwritten and are never committed.**
- **repowiki (`docs/repowiki/` — manually maintained)**: the committed markdown is the **single source of truth**; any tool (AI or human) edits it directly and commits. Keep these pages in sync with code changes as needed, verifying against the code just like any other source doc.

---

## Git Commit Conventions

- **Commit directly to the `main` branch** — do not create feature branches or open PRs (this repo's convention)
- Commit messages **must not** include a `Co-Authored-By` trailer
- Follow the Conventional Commits format: `type(scope): description`
- Commit messages that reference a GitHub issue must include the issue reference
- Issue reference rules: the short form `#123` always points to an issue in **the repo where the commit lives**; whenever the referenced issue is not in the current repo, you must write the full `owner/repo#123`
  - A commit in the parent repo `songloft-org/songloft` referencing a parent-repo issue: may write `#155`, or `songloft-org/songloft#155`
  - A commit in a submodule repo (such as `pkg/tag`, `songloft-player`, `plugin-toolchain`, `jsplugins-src/*`) referencing an issue in its own repo: may write `#14`, or the full repo path
  - A commit in a submodule repo referencing a parent-repo issue: must write the full path, e.g. `songloft-org/songloft#155`, not just `#155` (otherwise GitHub resolves it to an issue in the submodule's own repo)
  - Any cross-repo reference always uses the full path, e.g. `songloft-org/songloft-player#14`

---

## Build and Deployment

- Build tags: `dev` (includes Swagger + pprof) / `lite` (slim version, no embedded frontend) / no tag (full version, embeds Flutter Web)
- When `VERSION=dev`, the Makefile automatically enables `-tags dev` (no need to manually pass `EXTRA_TAGS=dev`)
- Two orthogonal dimensions: **VERSION** (`dev` / `X.Y.Z`) controls whether it's a dev build; **BUILD_TYPE** (`lite` / empty i.e. `full`) controls whether the frontend is embedded. **Do not** use mixed values like `BUILD_TYPE=dev`
- The embed path is `songloft-player-build/web-embedded` (**not** `songloft-player/build/web-embedded`)
- SPA fallback: handled by `internal/app/embed.go`, returning `index.html` when a file doesn't exist
- Deployment mode is switched via `--dart-define=DEPLOY_MODE=embedded|standalone`; `AppConfig.isEmbedded` is a compile-time constant, and tree-shaking removes the API-address UI in standalone mode
- Sub-path deployment: configured at startup via `-base-path /xxx` or `BASE_PATH=/xxx`; the backend strips the prefix at the outermost layer with `http.StripPrefix`, and `embed.go` replaces `<base href="/">` with `<base href="/xxx/">` at runtime; in embedded mode the frontend auto-detects the sub-path from `Uri.base.path`

### Bundle Local Mode (v2.9.0+)

Embeds the Go backend into the Flutter client so users can use it without deploying a separate server. Enabled at compile time with `--dart-define=HAS_BACKEND=true`.

- **Mobile (Android/iOS)**: `gomobile bind` compiles the Go backend into a native library (`.aar` / `.xcframework`), and Flutter calls it via `MethodChannel('com.songloft/backend')`
- **Desktop (macOS/Windows/Linux)**: the Go backend is compiled into a standalone executable `songloft-server`, which Flutter runs as a subprocess at startup
- **Web**: Bundle mode is not supported (remote server only)
- Run mode: `RunMode.local` / `RunMode.remote`, persisted to SharedPreferences and auto-restored at startup
- Local-mode startup flow: request storage permission → start the embedded backend (`127.0.0.1:<port>`) → health-check polling (up to 10 × 300ms) → auto-login with `admin/admin`
- `BackendLifecycle` (WidgetsBindingObserver): auto-restarts the backend when the app returns to the foreground, stops it on detached
- Key entry points: `mobile/mobile.go` (gomobile binding), `songloft-player/lib/core/backend/` (Flutter-side abstraction layer)
- CI artifact naming: `songloft-bundled-{platform}-{arch}.{ext}`, 4 parallel jobs (Android/Linux/Apple/Windows); failures don't block the main Release

### Docker Hot-Swap Rules (`scripts/docker-entrypoint.sh`)

The Docker image contains a base package `/app/songloft`, while the persistent data volume holds the actually-running `/app/data/songloft`. On container startup the entrypoint decides whether to overwrite the data directory with the base package:

**Core principle: the base package represents the user's intent; when dev/release or full/lite differ, overwrite with the base package. Only compare old vs. new when "same channel + same BUILD_TYPE": dev by Build Time, release by version number.**

| Scenario | Behavior | Reason |
|------|------|------|
| dev ↔ release channel differs | Replace | The user switched image channels |
| BUILD_TYPE differs (full↔lite) | Replace | The user switched image variants |
| Both dev + same type + base package Build Time > data Build Time | Replace | Dev rolling builds pick the newest by build time |
| Both dev + same type + data Build Time >= base package Build Time | Don't replace | The data may have been upgraded online via the API |
| Both release + same type + base package version > data version | Replace | Release upgrade |
| Both release + same type + data version >= base package | Don't replace | The data may have been upgraded online via the API |

---

## Platform Adaptation Pitfalls

- The upgrade check (`/api/v1/upgrade/check`) is only available on Docker
- Flutter `secure_storage` automatically falls back to SharedPreferences under an unsigned macOS sandbox
- Before an Android build you need `sdkmanager --licenses`; Android 13+ requires requesting notification permission at runtime
- All native platforms (Win/Linux/macOS/Android/iOS) uniformly use media_kit/libmpv as the audio backend (via `just_audio_media_kit` / the custom `SongloftJustAudioPlatform`), with no native fallback and no kill-switch
- HyperOS3 and similar need `androidStopForegroundOnPause: false` to prevent background reclamation
- **Bundle mode Android**: the CWD is `/`, so the covers directory path must be resolved relative to `DBPath` rather than the CWD (fixed in `da65db1`)
- **Bundle mode native bridging**: Android uses `Class.forName("mobile.Mobile")` reflection to call the gomobile-generated class; when the `.aar` isn't bundled, `isAvailable()` returns false (graceful degradation); iOS likewise uses Swift to call the Objective-C functions like `MobileStart`
- **Bundle desktop subprocess**: `DesktopBackendService` looks for `songloft-server` in the **same directory** as the Flutter executable (on macOS, `Contents/Resources/`), and parses the actual listening port from stdout

---

## JS Plugins

- Source at `jsplugins-src/<name>/`; build artifacts are in each plugin repo's GitHub Releases
- Create a new plugin: `npx create-songloft-plugin@latest` (interactive scaffolding; see `plugin-toolchain/README.md` for details)
- Sandbox: QuickJS, with the `host` bridge provided by `internal/jsruntime` to invoke host capabilities (`http.fetch`, `storage`, `logger`, `songs.*`, `playlists.*`)
- Routing: `/api/v1/jsplugin/{entry_path}/...`
- Common assets: `/api/v1/jsplugin-assets/*` serves the `common.css`/`common.js`/fonts embedded in the Go binary, which `injectHTMLHead` automatically injects into all plugin HTML pages
- Theme sync: `common.js` contains embed detection + theme bridging (URL `?theme=` parameter + real-time `postMessage` updates + `data-theme` attribute + `songloft-theme-change` event), and exposes the `window.SongloftPlugin` global API (`getTheme`/`onThemeChange`/`apiGet`/`apiPost`, etc.)
- `common.css` defines `--md-*` CSS variables (dual light/dark theme); any plugin using these variables automatically follows theme switches
- Permissions: `permissions: ["net", "storage", "fs:music", ...]` in the manifest, validated at runtime by `internal/jsplugin`
- Health checks + file-fingerprint hot updates both happen automatically
- **UDP Socket API** (`songloft.net`, requires `net` permission): the Go side hosts the UDP socket + a message-push model. `udpBind` creates a socket and starts a reader goroutine; received UDP packets are pushed asynchronously to the JS callback (`onData`) via the scheduler queue. Supports multicast groups (`udpJoinMulticast/udpLeaveMulticast`), typical use: SSDP device discovery (DLNA/UPnP). Each plugin gets at most 8 sockets; a plugin with active sockets won't be idle-evicted, and sockets are cleaned up automatically on plugin unload. Implemented in `internal/jsplugin/api_bridge_net.go`
- **TCP Socket API** (`songloft.net.tcpConnect`, requires `net` permission): outbound TCP connections. `tcpConnect(host, port, options?)` returns a socket handle with `send()/onData()/onClose()/close()`. Data reception reuses UDP's Go readLoop + host event queue push model (`postHostEvent("tcp_data")` → JS `__dispatchHostEvent`). **`data` is base64-encoded raw bytes** (`btoa` on send, `atob` on onData, same as UDP): TCP is a byte stream and a single read may split in the middle of a multi-byte UTF-8 character; a raw string would be replaced with U+FFFD by `json.Marshal` and permanently corrupted, so base64 is mandatory. Plugins must accumulate bytes across chunks before UTF-8 decoding. **Only private / loopback / link-local addresses are allowed** (`isPrivateHostAllowed`, anti-SSRF); the 8-sockets-per-plugin quota is counted independently from UDP; a plugin with active TCP connections won't be idle-evicted; sockets are cleaned up on unload. Typical use: controlling a local MPD (idle event push on port 6600). Implemented in `internal/jsplugin/api_bridge_tcp.go`
- **Private registry authentication**: `RegistryConfig` supports a `token` field; when fetching any resource under that registry it automatically carries an `Authorization: Bearer <token>` header, compatible with GitHub private-repo PATs and self-hosted private registries. See [Plugin registry authoring guide · Private registry authentication](docs/plugin_registry.md#私有源认证)
- **Lyrics/Cover providers** (`songloft.lyrics` / `songloft.covers`, no permission required): plugins call `registerProvider()` to register as a lyrics or cover provider. When a song has no lyrics/cover, the host iterates registered providers and calls `/lyric-search` or `/cover-search` endpoints (15s timeout, first-match-wins). Search params include `title/artist/album`; lyrics also carry `duration`; both optionally carry `fingerprint` (Chromaprint) and `isrc`. Found lyrics are cached to DB (`scraped`); local songs also get lyrics embedded into file tags. Found covers are downloaded to `cover_path` for local songs (and embedded into tags); remote songs store `cover_url`. Provider registration survives idle eviction; only cleared on plugin disable. Implementation in `manager.go` (`SearchLyrics/SearchCover`), `api_bridge.go` (JS API), `handlers/music.go` (fallback calls). See [Plugin development guide · Lyrics/Cover providers](docs/en/js-plugin-development-guide.md#songloftlyrics--lyrics-provider)

---

## Business Pitfalls Summary (Important — not in the code)

### Scan title rules

- tag has a title → use `tag.Title` directly
- tag has no title → filename minus extension
- **Do not** apply "longest-common-substring dedup + concatenation" — it produces results like "Artist - Title" that redundantly stuff the artist into the title field
- Video container probe: when scanning containers like mp4/mov/mkv/webm/avi/ts, ffprobe detects whether a real video track is present (excluding the cover attached_pic) to set `songs.is_video`; the client uses this to render the picture / pick the cast mime

### Tag writing (pkg/tag)

- `tag.WriteTag(filePath, opts)` dispatches by file extension; all formats write atomically with a temp file + `os.Rename`
- Support matrix:

| Format | Text fields | Lyrics | Cover |
|------|---------|------|------|
| MP3 | ID3v2.3 text frames | USLT | APIC |
| FLAC | Vorbis Comment | LYRICS | PICTURE block |
| M4A/MP4/M4B/MOV | iTunes atoms (©nam, etc.) | ©lyr | covr |
| OGG(.ogg/.oga) | Vorbis Comment | LYRICS | METADATA_BLOCK_PICTURE (base64) |
| APE | APEv2 text items | Lyrics | Cover Art (Front) (binary item) |
| WAV | RIFF LIST INFO | ICMT | **Not supported** (format limitation) |
| AIFF/AIF | ID3v2.3 (ID3 chunk) + NAME/AUTH | USLT (ID3 chunk) | APIC (ID3 chunk) |
- Unsupported formats → return `ErrUnsupportedWrite`; the caller **must** degrade to a log entry and **must not** block the main flow

### HLS radio proxy mode (/settings/hls-proxy)

- Business toggle endpoint: `GET/PUT /api/v1/settings/hls-proxy` with body `{enabled: bool}`, default `false`
  - `false`: the radio `.m3u8` is 302-redirected straight to the player, which pulls the origin itself. Zero overhead but subject to origin anti-hotlinking/CORS restrictions
  - `true`: the server fetches and rewrites the m3u8 and proxies all segments/keys/init segments. **All segments consume this machine's bandwidth**, so mind the traffic cost
- When to switch: turn on the proxy when origin Referer/UA anti-hotlinking causes playback failures, or when CORS blocks in Web embedded mode
- Reverse-proxy endpoints: `/api/v1/songs/{id}/hls/playlist?u=<base64url>` and `/api/v1/songs/{id}/hls/segment?u=<base64url>`
- HLS radio song.url is forced to carry a `.m3u8` suffix (`/api/v1/songs/{id}/play.m3u8`): ExoPlayer/AVPlayer pick the MediaSource by URL suffix, and without a suffix it falls into ProgressiveMediaSource, making live streams unplayable
- Rewrite rules: classic HLS + the full LL-HLS set (PART/PRELOAD-HINT/RENDITION-REPORT) + `EXT-X-DATERANGE:X-ASSET-URI` (HLS Interstitials single URI). `X-ASSET-LIST` (JSON sub-proxy) is not yet implemented and is passed through verbatim when encountered
- Security: each endpoint entry performs a "same-origin check (scheme+host+port must exactly equal song.URL)" as the first line of defense, with `services.IsHostnameAllowed` as an SSRF backstop. **Non-same-origin URLs are left unchanged and not rewritten**, to avoid becoming an open proxy
- Player cross-origin: all rewritten URLs are relative paths (`playlist?u=...` / `segment?u=...`), sidestepping BASE_PATH sub-path deployment issues
- Upstream 4xx/5xx are passed through to the player; the playlist body is capped at 1 MB; the first line must be `#EXTM3U`

### Generic HTTP Proxy (/settings/http-proxy)

- Business endpoint: `GET/PUT /api/v1/settings/http-proxy` with body `{proxy: string}`, default `""` (direct connection)
- Once set, all outbound HTTP requests from the backend (plugin registry fetching, plugin download/update, system upgrade check/download) are forwarded through the specified HTTP proxy
- Typical value: `http://192.168.1.1:7890` (supports HTTP/HTTPS/SOCKS5 proxies)
- Loopback addresses (`localhost`/`127.0.0.1`/`::1`) automatically bypass the proxy, avoiding interference with internal requests
- **Coexists** with GitHub mirror acceleration (`github_proxy` URL-prefix concatenation): the mirror prefix is concatenated first, then forwarded through the HTTP Proxy
- Implementation: `internal/httputil/proxy.go` provides a global `ProxyConfig` + a shared `*http.Transport`, and `httputil.NewClient(timeout)` creates a proxy-aware client
- The saved proxy address is loaded from the config table at startup (`app.go`); a PUT takes effect immediately without a restart
- Currently integrated services: `jsplugin/registry.go`, `jsplugin/package.go`, `services/upgrade_service.go`, `handlers/jsplugin_registry.go` (downloadZIP)

### Private network proxy allowlist (/settings/proxy-private-allowlist)

- Background: the generic resource proxy `GET /api/v1/proxy?url=` blocks all internal / loopback / link-local addresses via `services.IsHostnameAllowed` by default (anti-SSRF), which rejects the "public Songloft proxying a WebDAV reachable only on the LAN" scenario (songloft-org/songloft#313)
- Business endpoint: `GET/PUT /api/v1/settings/proxy-private-allowlist` with body `{allowlist: []string}`, default `[]` (empty = keep blocking everything, behavior unchanged)
- Each entry is a single IP (`192.168.1.100`) or CIDR range (`192.168.1.0/24`); PUT validates via `services.ParseAllowlist`, returning 400 on invalid entries
- Decision: `services.IsHostnameAllowedWithAllowlist(hostname, allowlist)` — public addresses always pass, private IPs pass only when covered by an allowlist range; `localhost`/`.local`/empty hostnames are still string-blocked (the allowlist matches by IP/CIDR only)
- **Only affects the generic `/proxy`**; HLS reverse proxy (`hls.go`) still uses `IsHostnameAllowed(nil)`, semantics unchanged
- Implementation: `internal/services/whitelist.go` (`ParseAllowlist` / `IsHostnameAllowedWithAllowlist`) + `internal/handlers/proxy.go` (`ProxyHandler` holds a `*ConfigService`, config key `proxy_private_allowlist`)

### Music caching (cache_service)

- When playing a remote song, the upstream audio is streamed and proxied to the client (non-blocking) while being written to the cache asynchronously in the background; subsequent playback hits the cache and is served directly from local
- Streaming proxy `ServeRemoteResourceWithCache`: on 200 OK, a TeeReader both proxies and writes a temp file; on 206 Partial, it proxies normally and triggers an asynchronous full download
- The cache path is persisted in the `songs.cache_path` field (DB level); lookups prefer `cache_path`, falling back to the old hash-bucketed directory format
- The cache directory defaults to `{data_dir}/music_cache/`, and can be customized to an absolute path via the `cache_dir` field of `PUT /api/v1/cache-manage/config`
- At startup the custom directory is read from the `music_cache_config` config; switching directories at runtime automatically rebuilds the LRU index and does not migrate old files
- LRU eviction: when exceeding `max_size` (default 1GB), evict by last access time; `max_size=0` means unlimited
- **Transcode on cache** (`transcode_format` / `transcode_quality`, `PUT /api/v1/cache-manage/config`, songloft-org/songloft#300): defaults to `""`, storing the upstream raw container (YouTube .mkv/.webm, Bilibili .mov); set to `mp3/m4a/ogg/flac/wav` to transcode network songs to that format when caching (fixes devices like Xiao AI speakers that cannot play MKV). Performed by `EnsureCachedFormat` at the two playback-side cache producers — `FinalizeCache` (streaming playback + 206 async full download) and the prefetch prewarm in `prepareSongPlayback` (after `Get`); reuses `runFFmpeg`, deletes the raw file after transcoding and points `cache_path` to the new format. **Gracefully degrades** to the original format when ffmpeg is missing or transcoding fails. Does **not** affect `songs.download`'s explicit format handling (the download path reuses `Get` but carries its own `opts.Format`, so the transcode logic is attached only on the playback side and never touches `moveToCache`/`Get`). **Tradeoff**: `ffmpeg -vn` drops the video track, so once enabled, `media=video` casting of an `is_video` remote song yields audio only (expected, serving the YouTube MKV→mp3 primary need); `EnsureCachedFormat` carries a `cacheTranscodeTimeout` (15min) fallback to prevent a stuck ffmpeg from permanently holding `transcodeSem`
- `POST /api/v1/cache-manage/validate-dir` can validate a directory in advance (auto-create + writability check + return disk space)
- Inflight dedup: concurrent requests for the same `song.ID` download only once; when the first request is `ctx.Canceled`, later waiters retry automatically

### Song persistence (song_downloader — plugin infrastructure)

- **Positioning**: this is a plugin infrastructure capability, not a user-facing feature of the main program. The main program provides the `songs.download` Bridge API, allowing plugins to persist remote songs from the user's own network storage (NAS/WebDAV/Subsonic, etc.) to the server's local `music_path`, converting them to the `local` type. **This capability is only for music the user legally owns and must not be used to download copyright-protected content from third-party commercial music platforms**
- Core service `SongDownloader.Download`: acquire the audio (copy directly on cache hit, otherwise download synchronously) → optional transcode → path-template rendering → optional metadata embedding (all supported formats) → update DB (type=local)
- **Download transcode** (`SongDownloadOptions.Format` / `Quality`): a plugin may pass `format` (mp3/m4a/ogg/flac/wav) plus optional `quality` (128/192/320), reusing the playback path's `CacheService.GetOrTranscode` to transcode into a standard audio container at download time. Typical use: sources like Bilibili produce a `.mov` video container that can't be scraped for lyrics, so transcoding to mp3 lands a scrapeable file. Empty `format` = no transcode, keep the source format; transcoding depends on ffmpeg and degrades gracefully when it's missing/fails (warn only, keep the source format, never block the download)
- **URL lyrics auto-fetch**: when `embed_metadata=true` and `lyric_source=url`, `LyricFetcher` fetches the lyrics → the primary lyrics are written to the file tag → the full payload (including translation/romanization) is cached to the DB → `lyric_source` is updated to `embedded`. A fetch failure only warns and does not block persistence
- Exposed to JS plugins via the Bridge API `songs.download`, with permission mapped to `PermSongsWrite`
- The official plugin `songloft-plugin-downloader` (separate repo `songloft-org/songloft-plugin-downloader`) is built on this API and provides the ability to download remote songs from the user's own network storage to local

### File moving: the cross-device rename trap

- `os.Rename` returns a `syscall.EXDEV` (cross-device link) error when src and dst are not on the same filesystem (mount point)
- Typical scenario: `os.CreateTemp("")` creates the temp file in the system `/tmp` (tmpfs), while the target cache/music directory is mounted on a separate disk or a Docker volume
- **Uniformly use** `internal/services.moveFile(src, dst)` instead of a bare `os.Rename`: it tries rename first and, on EXDEV, automatically falls back to copy + remove
- `pkg/tag`'s atomic write is unaffected: it uses `os.CreateTemp(dir, ...)` to create the temp file in the **same directory** as the source file, so the rename is always same-device
- New download/cache logic that needs to "write a temp file first, then move it to the target location" **must** use `moveFile` and **must not** use a bare `os.Rename`
