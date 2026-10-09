# AGENTS.md

This document provides AI coding assistants with the **entry-point information** for the Songloft project: project structure, common commands, hard rules, and a summary of pitfalls. For content where the code itself is the source of truth (directory tree, dependencies, API tables, table schemas), read the code directly or the detailed docs linked below.

> **Detailed docs**:
> - Architecture: [Overview](docs/architecture.md) · [Backend](docs/architecture_backend.md) · [Frontend](docs/architecture_frontend.md)
> - Topics: [Database operations](docs/database_migrations.md) · [Color system](docs/color_system.md) · [API response format](docs/api_response.md) · [Quick start](docs/quick-start.md) · [Frontend gotchas](docs/en/frontend_gotchas.md)
> - Plugin development: see `plugins/toolchain/README.md` (separate repo)
> - Plugin registry authoring: [Plugin registry authoring guide](docs/plugin_registry.md)
> - API: after starting in dev mode, visit `/swagger/index.html`

---

## Project Overview

Songloft is a self-hosted local music server that supports both **server deployment** and **Bundle local mode** (embedding the Go backend into the client, so no separate server deployment is required). It has a multi-repo structure:

| Directory | Tech | Description |
|------|------|------|
| `/` | Go 1.26 + Chi v5 + SQLite | Backend API service (default port 58091, account admin/admin) |
| `/mobile` | Go + gomobile | Mobile binding entry point for the Go backend (for gomobile bind; exports Start/Stop/IsRunning/GetPort) |
| `/clients/player` ([separate repo](https://github.com/songloft-org/songloft-player)) | Flutter 3.29+ / Dart 3.7+ | Cross-platform frontend (6 platforms), supports Bundle local mode |
| `/clients/player-lynx` ([separate repo](https://github.com/songloft-org/songloft-player-lynx)) | TS + ReactLynx | Android/iOS/HarmonyOS/Web preview client, separate main dev builds and v* releases (submodule; no desktop/Bundle) |
| `/clients/tv` ([separate repo](https://github.com/songloft-org/songloft-tv)) | Kotlin | Android TV client (submodule) |
| `/plugins/toolchain` ([separate repo](https://github.com/songloft-org/plugin-toolchain)) | TS + pnpm | JS plugin development toolchain (SDK / Builder / scaffolding) |
| `/plugins/src` | TS | JS plugin source code (a collection of submodules; each plugin distributes releases from its own repo) |
| `/pkg/tag` | Go | Audio metadata **read/write** library (extends the upstream tag library with MP3/FLAC writing) |
| `/integrations/home-assistant` ([separate repo](https://github.com/songloft-org/home-assistant-addon)) | HA add-on | Home Assistant add-on (thin layer reusing the Docker image; submodule). **The `repository.yaml` manifest must sit at that repo's root** — which is exactly why it was split out (#340). Design/pitfalls/version sync: see [integrations/home-assistant/README.md](integrations/home-assistant/README.md) |
| `/tools/ffmpeg-builder` ([separate repo](https://github.com/hanxi/ffmpeg-builder)) | Docker | Minimal-image builder for statically compiled ffmpeg/ffprobe (submodule); used for download transcode / audio fingerprinting |
| `/tools/tracely` ([separate repo](https://github.com/hanxi/tracely)) | Go + Vue | Self-hosted frontend monitoring backend (install/upgrade tracking); the backend reports via its Go SDK. Submodule; the SDK dependency comes from go.mod |

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

# Frontend build (artifacts land in clients/player-build/, for backend embedding or standalone deployment)
make build-frontend-web-embedded   # For embedding in the Go binary (hides the API-address UI)
make build-frontend-web            # Standalone web deployment
make build-frontend-{linux,windows,macos,android,ios,all}

# Bundle local mode (Go backend compiled into a mobile library / desktop executable)
make build-go-mobile-android       # Android .aar (gomobile bind, arm64 + arm + x86_64)
make build-go-mobile-ios           # iOS .xcframework (gomobile bind, arm64, macOS only)
make build-go-desktop-linux        # Linux executable
make build-go-desktop-windows      # Windows .exe
make build-go-desktop-macos        # macOS x86_64
make build-go-desktop-macos-arm64  # macOS ARM64

# Frontend development
cd clients/player && flutter run -d chrome          # standalone
cd clients/player && flutter run -d chrome --dart-define=DEPLOY_MODE=embedded
```

---

## Code Formatting (Hard Rule)

After every code change, you **must** format the code before committing:

- **Go**: Run `gofmt -w .` from the project root
- **Dart**: Run `dart format lib/ test/` from the `clients/player/` directory

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
- **Never write CJK comments after `-- name:` in `queries/*.sql`** → sqlc embeds query comments into generated code as doc comments **byte-wise**; multi-byte characters get truncated into invalid UTF-8 and `make sqlc` fails outright (`error generating code: illegal UTF-8 encoding`). Put such comments on the Go repository method instead
- **Qualify outer columns in self-referencing subqueries** → in `DELETE FROM t WHERE col = ? AND id NOT IN (SELECT x.id FROM t x ...)`, a bare outer `col` is rejected by sqlc as `column reference is ambiguous`; write `t.col`

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
- **Business tag naming**: reuse existing tags (「歌曲管理」「歌单管理」「电台与 HLS」「扫描管理」「配置管理」「缓存管理」「JS 插件」「JS插件管理」「数据备份」「设置」「系统升级」「认证管理」「系统管理」「资源代理」「主题包」「播放历史」「标签管理」); do not casually invent new tags
- **`@Router` paths must NOT contain the `/api/v1` prefix**: `main.go` already declares `// @BasePath /api/v1`, and swag automatically prepends this prefix to every `@Router` path. Including `/api/v1` in the annotation will cause the generated docs to have paths like `/api/v1/api/v1/...`. Always use relative paths (e.g. `/songs/{id}/tags`), never `/api/v1/songs/{id}/tags`.

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

- `SettingsApi` (`clients/player/lib/features/settings/data/settings_api.dart`) wraps all `/settings/*` calls; business-feature Providers always go through it
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
- **Submodule-synced pages (also do NOT hand-edit — but the source lives in another repo)**: `docs/addon/`, `docs/player/`, `docs/player-lynx/` (and English `docs/en/player-lynx/`), and `docs/plugin-toolchain/` are synced by `sync-docs.mjs` from the `integrations/home-assistant/`, `clients/player/docs/cn/`, `clients/player-lynx/`, and `plugins/toolchain/` **submodules** respectively, and are likewise ignored via `docs/.gitignore`. To change the body → **edit it in the corresponding submodule repo, then come back and bump the submodule pointer** (`git submodule update --remote <path>` + commit), otherwise the docs site keeps showing stale content. Two things to note: (1) the `to:` target path is **deliberately decoupled** from the source path (e.g. `integrations/home-assistant/README.md` → `docs/addon/index.md`), because public URLs like `/addon/` are already in the sitemap and must not follow source-repo renames; (2) when a submodule is not checked out, `sync` only warns instead of failing and the page **silently disappears** — which is why the submodule init list in `static.yml` must include them.
- **repowiki (`docs/repowiki/` — AI-generated by hand, manually maintained)**: this wiki (system architecture / data model / backend / frontend / API / plugins / deployment / troubleshooting / dev guide, etc.) is **generated by hand from source files by an AI** — there is **no generator script** in the repo and it is not an external-service artifact; the **committed markdown is the single source of truth**. Any tool (AI or human) edits it directly and commits — **do not wait for a "regeneration"**. Keep these pages in sync with code changes as needed, verifying against the code just like any other source doc (e.g. after renaming public assets in `internal/jsplugin/assets/`, update any repowiki references to the old file name too).

---

## Git Commit Conventions

- **Commit directly to the `main` branch** — do not create feature branches or open PRs (this repo's convention)
- Commit messages **must not** include a `Co-Authored-By` trailer
- Follow the Conventional Commits format: `type(scope): description`, prefer Chinese for description and body
- Commit messages that reference a GitHub issue must include the issue reference
- Issue reference rules: the short form `#123` always points to an issue in **the repo where the commit lives**; whenever the referenced issue is not in the current repo, you must write the full `owner/repo#123`
  - A commit in the parent repo `songloft-org/songloft` referencing a parent-repo issue: may write `#155`, or `songloft-org/songloft#155`
  - A commit in a submodule repo (such as `pkg/tag`, `clients/player`, `plugins/toolchain`, `plugins/src/*`) referencing an issue in its own repo: may write `#14`, or the full repo path
  - A commit in a submodule repo referencing a parent-repo issue: must write the full path, e.g. `songloft-org/songloft#155`, not just `#155` (otherwise GitHub resolves it to an issue in the submodule's own repo)
  - Any cross-repo reference always uses the full path, e.g. `songloft-org/songloft-player#14`

---

## Build and Deployment

- Build tags: `dev` (includes Swagger + pprof) / `lite` (slim version, no embedded frontend) / no tag (full version, embeds Flutter Web)
- When `VERSION=dev`, the Makefile automatically enables `-tags dev` (no need to manually pass `EXTRA_TAGS=dev`)
- Two orthogonal dimensions: **VERSION** (`dev` / `X.Y.Z`) controls whether it's a dev build; **BUILD_TYPE** (`lite` / empty i.e. `full`) controls whether the frontend is embedded. **Do not** use mixed values like `BUILD_TYPE=dev`
- The embed path is `clients/player-build/web-embedded` (**not** `clients/player/build/web-embedded`)
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
- Key entry points: `mobile/mobile.go` (gomobile binding), `clients/player/lib/core/backend/` (Flutter-side abstraction layer)
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

### Docker Non-Root Operation (PUID/PGID, songloft-org/songloft#380)

- **Unset by default = keeps running as root**, identical to prior behavior with zero migration risk. Non-root mode only activates when `PUID` or `PGID` is explicitly set (either one is enough; the unset one defaults to `1000`); the entrypoint drops privileges to that uid:gid via Alpine's built-in `su-exec` (lighter than `gosu`, ships in the official repo with no extra download) right before `exec`ing the main program
- **`/app/data` is recursively `chown`ed on every startup, `/app/music` only has its top-level directory `chown`ed, never recursively**: `/app/data` is small (db, covers, cache, etc.) and must be fixed up to remove ownership left over from a prior root-based run, otherwise the new user can't open the old database; `/app/music` can be a personal library with hundreds of thousands of files / multiple TB, and the IO cost of recursively walking it on every startup is unacceptable. Once the top-level directory is writable, newly downloaded/written files are already created with the correct target uid:gid — no follow-up fix needed
- **Pre-existing root-owned files left inside `/app/music` from before the upgrade are not auto-fixed** (e.g. old files that had tags written into them) — this is a deliberate performance trade-off, not an oversight. Set `FIX_MUSIC_PERMISSIONS=true` when you need a one-time recursive fixup; this is meant to be run manually once after switching to non-root, not as a default behavior
- **`integrations/home-assistant` is unaffected**: its `run.sh` overrides the image's `ENTRYPOINT` entirely, bypassing `docker-entrypoint.sh`; its permission model is managed separately by the HA supervisor

---

## Frontend UI Verification (Dockerized Headless Browser)

When you need to **actually exercise a change in the UI** (rather than only running `flutter test`), use a
headless Chrome inside Docker: on this repo's dev machine the host's `google-chrome --headless` core dumps
and never binds its CDP port, so don't waste time on it.

```bash
# 1. Build the frontend in embedded mode so the Go backend serves it same-origin
#    (skips the standalone API-address configuration step)
make build-frontend-web-embedded
go build -tags dev -o /tmp/songloft-full .
/tmp/songloft-full -port 58191 -db <tmpdir>/test.db -music <musicdir>

# 2. Start the browser container. --network host is what lets the containerized Chrome
#    reach the host's 127.0.0.1:58191
docker run -d --name uichrome --network host browserless/chrome:latest

# 3. Drive it through the /function endpoint (Content-Type must be application/javascript)
curl -s -X POST http://127.0.0.1:3000/function \
  -H 'Content-Type: application/javascript' --data-binary @script.js
```

Scripts look like `module.exports = async ({ page }) => { ...; return { data: {...}, type: 'application/json' } }`.
Take screenshots with `page.screenshot({ encoding: 'base64' })`, return them inside `data`, and base64-decode
them into PNGs locally.

### Pitfalls when driving Flutter Web

- **Flutter Web renders to a canvas, so there are no buttons in the DOM.** Interactive elements are only
  exposed as `<flt-semantics>` accessibility nodes: read `getBoundingClientRect()`, then
  `page.mouse.click(x, y)` on the center. The semantics tree is usually already enabled; if a
  `<flt-semantics-placeholder>` shows up on first paint, `.click()` it first
- **Semantics labels get merged**: a whole screen often collapses into one long aria-label node, and a
  button's text is not necessarily its own node. When you can't find it, **screenshot first and click by
  coordinates** instead of fighting label matching
- **Text fields**: `<input aria-label="Username">` is real DOM, but clicking by coordinates plus
  `keyboard.type` is more reliable; leave ~800ms between the click and the typing, otherwise Flutter has not
  established focus yet and the input is dropped
- **Verifying both languages**: override `navigator.language`/`languages` to `zh-CN` via
  `evaluateOnNewDocument`; Flutter l10n follows
- **Deep links**: `page.goto('/settings/category/2')` is a full reload and can produce a bogus
  "Failed to load" while the app is mid-boot. Navigate by clicking inside the app instead of using goto for
  nested routes

### Anchor assertions on backend-observable state

A screenshot only proves "it rendered correctly". Whether the interaction actually took effect needs separate
evidence — e.g. after flipping a toggle, `curl` the matching `/settings/<name>` endpoint and check the value
changed; after clicking "Stop computing", use `pgrep -x ffmpeg` to confirm the child processes hit zero.

- **Do not count processes with `ps -ef | grep <keyword> | wc -l`**: the current shell's own command line
  contains that keyword, so you consistently over-count by 1–2 and can easily conclude "it didn't stop".
  Use `pgrep -x <executable>`

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

- Source at `plugins/src/<name>/`; build artifacts are in each plugin repo's GitHub Releases
- Create a new plugin: `npx create-songloft-plugin@latest` (interactive scaffolding; supports WebView / Lynx rendering engine templates; see `plugins/toolchain/README.md` for details)
- Sandbox: QuickJS. `fetch` is a built-in global function in the runtime (uses `__go_fetch_async`), and the `console` family (`songloft.log`, etc.) are log wrappers — neither are bridge calls. The bridge is dispatched by `HandleBridgeCall` in `internal/jsplugin/api_bridge.go`; namespaces include: `storage`, `persistent-storage`, `songs`, `playlists`, `tags`, `comm`, `plugin`, `command`, `jsenv`, `fs`, `net`, `websocket`
- Routing: `/api/v1/jsplugin/{entry_path}/...`
- Common assets: `/api/v1/jsplugin-assets/*` serves the `theme.css`/`components.css`/`common.js`/fonts embedded in the Go binary, which `injectHTMLHead` automatically injects into all plugin HTML pages (injection order: theme.css → components.css → common.js)
- Theme sync: `common.js` contains embed detection + theme bridging (URL `?theme=` parameter + real-time `postMessage` updates + `data-theme` attribute + `songloft-theme-change` event), and exposes the `window.SongloftPlugin` global API (`getTheme`/`onThemeChange`/`apiGet`/`apiPost`/`getCookies`, etc.)
- **Client host bridge (WebView)** (`@songloft/client-sdk`, built into `common.js`): Plugin frontend pages call Flutter client host capabilities via `window.SongloftPlugin.host` / `player` / `getCookies` / `invokeHost`. Native platforms use `flutter_inappwebview.callHandler('songloftHost', {ns, method, params})`, Web/iframe uses `postMessage` to parent window. Dispatch logic in `clients/player/lib/features/home/presentation/plugin_host_dispatch.dart` (transport-agnostic, web-safe), native bridge in `plugin_host_bridge.dart` (mixin, registers callHandler + injects platform-specific callbacks). Registered namespaces: `host` (getInfo), `player` (playback control), `cookies` (cookie reading), `favorite` (favorite state sync — `refresh` method, pass `{songId, isFavorited}` for incremental update of Flutter-side FavoriteNotifier cache, or omit params for full reload). **The object literal at the end of `common.js` is the single source of truth for what `window.SongloftPlugin` exposes** — `invokeHost` once existed only on `window.__SongloftInternal` (marked "plugins must not depend on this") and was absent from the public object, yet miot hand-wrote an `invokeHost?` declaration in its own `frontend/env.d.ts`. So `window.SongloftPlugin?.invokeHost?.(...)` type-checked fine and was **silently swallowed** by the optional call at runtime — the entire favorite-sync feature never sent a single byte (the root cause of the second regression in songloft-org/songloft-plugin-miot#86). **Plugins must not hand-write type declarations for host APIs**; use `SongloftPluginGlobal` from `@songloft/client-sdk`, or if you must hand-write them, check that literal in `common.js` first
- **Cookie reading bridge** (`window.SongloftPlugin.getCookies(origin)`): Reads cookies for a specified origin from the host WebView Cookie Store (including HttpOnly), returns `{name: value}` map. **Native clients only** (Android/iOS/macOS/Windows; Linux desktop has no WebView implementation) — Web platform cannot implement this due to browser same-origin policy; calls will reject. Implementation path: `common.js getCookies()` → `invokeHost('cookies', 'get', {origin})` → Flutter `PluginHostDispatcher` → `cookieProvider` callback → `CookieManager.instance().getCookies(url: WebUri(origin))`. Origin must include protocol+host (e.g. `https://example.com`); invalid formats are rejected by validation. Typical use case: session reuse for third-party gateways like FN Connect (user logs in within the app's WebView, plugin reads cookies for subsequent API calls)
- **Lynx native rendering bridge** (`@songloft/lynx-plugin-sdk`, for `renderEngine: "lynx"` only): plugins declaring `renderEngine: "lynx"` use ReactLynx to build UI, compiled into `.lynx.bundle`; the host loads them natively via Lynx `<frame>` elements. Communication goes through `NativeModules.SongloftPluginBridge` (three-platform native modules: Android `SongloftPluginBridgeModule.kt` / iOS `SongloftPluginBridgeModule.swift` / HarmonyOS `SongloftPluginBridgeModule.ets`), routing parent–child frame RPC + event push by `frameId`. The SDK provides `invokeHost(ns, method, params)` / `onPlayerState` / `onThemeChange` / `onPush` etc. **Does not use** `window.SongloftPlugin` / `@songloft/client-sdk` (those are WebView-only). At build time an `index.html` + `.web.bundle` is auto-generated as a WebView fallback for the Flutter client
- `theme.css` defines `--md-*` CSS variables (dual light/dark theme), font declarations and reset styles; `components.css` provides component library styles. Any plugin using these variables automatically follows theme switches
- Permissions: `permissions: ["net", "storage", "fs:music", ...]` in the manifest, validated at runtime by `internal/jsplugin`
- **fetch internal control headers**: `X-Fetch-No-Redirect` (don't follow redirects; required to collect `Set-Cookie` from intermediate hops), `X-Fetch-Timeout-Ms` (per-request timeout 100–30000ms), `X-Fetch-Insecure` (skip TLS verification, **requires the `net:insecure-tls` permission**). None of the three are ever forwarded upstream — `X-Fetch-Insecure` used to leak upstream as an ordinary header because it was unimplemented (songloft-org/songloft#401)
- **A new permission constant must be synced in three places, or plugins can't use it at all**: (1) `AllPermissions` in the host's `internal/jsplugin/permissions.go`; (2) `VALID_PERMISSIONS` in `plugins/toolchain/packages/plugin-builder/src/manifest.ts` — **this one hard-fails at build time** (`unknown permission: xxx`), so missing it means the plugin can't even produce a build artifact; (3) `AVAILABLE_PERMISSIONS` in `plugins/toolchain/packages/create-songloft-plugin/src/index.ts` (the scaffold's checklist). Separately, `ValidatePermissions` (host side) **deliberately only warns on unknown permissions rather than rejecting**: it sits on the mandatory install/update path, and rejecting would make a plugin that declares a new permission uninstallable on **older hosts**, taking down the plugin's otherwise-working features too; leniency is security-neutral (`CheckPermission` only honors strings it knows). **But older hosts with the strict check are already released**, so even after a new permission lands you must wait for host adoption before shipping the plugin — `minHostVersion` is **purely informational and never enforced by the host**, so don't expect it to gate anyone
- **`net:insecure-tls` permission (skip TLS certificate verification)**: the `jsruntime` layer **has no concept of permissions**, and `fetch` itself doesn't require `net`; this permission is projected by `service.go` calling `jsManager.SetAllowInsecureTLS(envID, ...)` after env creation into `JSEnv.allowInsecureTLS` (default false) — that flag is the permission's only runtime-layer manifestation. Key points: (1) **`CheckPermission` does no colon-prefix matching, so `net` does not cover `net:insecure-tls`** — deliberately, otherwise every existing plugin that already declares `net` (miot/bili/dav/pcyear-bridge) would silently gain verification-skipping on upgrade while the user sees no change in the permission list; (2) a dedicated `insecureTransport` — do **not** reuse `sharedTransport` and mutate `TLSClientConfig` per request, since connection pools are per-Transport and that would poison connection reuse for every plugin; (3) child environments (created via `songloft.jsenv`) must inherit the parent plugin's policy, or "plugin issues a request from a child worker" fails verification for no visible reason; (4) when the header is present without the permission, emit `slog.Warn` rather than dropping it silently — plugin authors commonly assume "if the host doesn't recognize it, it's a no-op", and one log line saves a whole debugging round. **Why it exists**: self-hosted NAS devices (fnOS port 5667) ship self-signed certs by default and plugins reach them by **bare IP**; even with a properly CA-signed cert the subject is a hostname, so connecting by IP always mismatches — structurally unsolvable for these targets
- Health checks + file-fingerprint hot updates both happen automatically
- **UDP Socket API** (`songloft.net`, requires `net` permission): the Go side hosts the UDP socket + a message-push model. `udpBind` creates a socket and starts a reader goroutine; received UDP packets are pushed asynchronously to the JS callback (`onData`) via the scheduler queue. Supports multicast groups (`udpJoinMulticast/udpLeaveMulticast`), typical use: SSDP device discovery (DLNA/UPnP). Each plugin gets at most 8 sockets; a plugin with active sockets won't be idle-evicted, and sockets are cleaned up automatically on plugin unload. Implemented in `internal/jsplugin/api_bridge_net.go`
- **TCP Socket API** (`songloft.net.tcpConnect`, requires `net` permission): outbound TCP connections. `tcpConnect(host, port, options?)` returns a socket handle with `send()/onData()/onClose()/close()`. Data reception reuses UDP's Go readLoop + host event queue push model (`postHostEvent("tcp_data")` → JS `__dispatchHostEvent`). **`data` is base64-encoded raw bytes** (`btoa` on send, `atob` on onData, same as UDP): TCP is a byte stream and a single read may split in the middle of a multi-byte UTF-8 character; a raw string would be replaced with U+FFFD by `json.Marshal` and permanently corrupted, so base64 is mandatory. Plugins must accumulate bytes across chunks before UTF-8 decoding. **Only private / loopback / link-local addresses are allowed** (`isPrivateHostAllowed`, anti-SSRF); the 8-sockets-per-plugin quota is counted independently from UDP; a plugin with active TCP connections won't be idle-evicted; sockets are cleaned up on unload. Typical use: controlling a local MPD (idle event push on port 6600). Implemented in `internal/jsplugin/api_bridge_tcp.go`
- **Private registry authentication**: `RegistryConfig` supports a `token` field; when fetching any resource under that registry it automatically carries an `Authorization: Bearer <token>` header, compatible with GitHub private-repo PATs and self-hosted private registries. See [Plugin registry authoring guide · Private registry authentication](docs/plugin_registry.md#私有源认证)
- **Lyrics/Cover providers** (`songloft.lyrics` / `songloft.covers`, no permission required): plugins call `registerProvider()` to register as a lyrics or cover provider. When a song has no lyrics/cover, the host iterates registered providers and calls `/lyric-search` or `/cover-search` endpoints (15s timeout, first-match-wins). Search params include `title/artist/album`; lyrics also carry `duration`; both optionally carry `fingerprint` (Chromaprint) and `isrc`. Found lyrics are cached to DB (`scraped`); local songs also get lyrics embedded into file tags. Found covers are downloaded to `cover_path` for local songs (and embedded into tags); remote songs store `cover_url`. Provider registration survives idle eviction; only cleared on plugin disable. Implementation in `manager.go` (`SearchLyrics/SearchCover`), `api_bridge.go` (JS API), `handlers/music.go` (fallback calls). See [Plugin development guide · Lyrics/Cover providers](docs/en/js-plugin-development-guide.md#songloftlyrics--lyrics-provider)

---

## Business Pitfalls Summary (Important — not in the code)

### Plugin store entry_path collisions (identity)

`entry_path` is nowhere near as simple as a "plugin ID" — it is simultaneously the registry
dedup key, the install-state match key, the `js_plugins.entry_path` UNIQUE constraint, the ZIP
filename (`<entryPath>.jsplugin.zip`), the static directory name
(`jsplugins_data/<entryPath>/static`), the route prefix (`/api/v1/jsplugin/{entryPath}/*`), the
manager/scheduler in-memory map key, and the ownership key behind
`plugin_storage.plugin_entry_path` / `songs.plugin_entry_path`. Two plugins from different
authors can absolutely collide on one entry_path (songloft-org/songloft#339). Read all of the
below before touching this area.

- **Two plugins sharing an entry_path cannot coexist locally** — that is a data-layer fact, not
  a UI limitation. #339 only fixed the store layer (show both rows, resolve install state per
  row, require confirmation before replacing). Real coexistence needs a genuine plugin id with
  entry_path demoted to a disambiguable route prefix, which means touching the DB constraint,
  the on-disk layout, routing, `EntryPathFromZipName`, the orphan cleanup in
  `SyncPluginsFromDirectory`, plus migrations for `plugin_storage` / `songs`
- **Identity = normalized author, falling back to the GitHub `owner/repo` from `update_url`**
  (`internal/jsplugin/identity.go`). Author normalization must strip `<email>` and `(notes)`
  before lowercasing — the same plugin is commonly written as `hanxi` in one registry and
  `Hanxi <a@b.com>` in another, and skipping normalization splits it into two store rows.
  **Non-GitHub self-hosted URLs are deliberately not inferred**: their path layout is arbitrary,
  so `/plugins/a/` and `/plugins/b/` would read as two repos and split one plugin's two mirrors
- **`SameIdentity` returns true when either side is empty** (treated as the same plugin). Better
  to miss a conflict than to invent one and block a legitimate upgrade just because the other
  side has no author
- **The cross-registry dedup key deliberately excludes the source URL**
  (`FetchAndMergeMulti`): the official registry and community aggregators routinely list the
  same plugin, and folding the source into the key would show it several times in "All" mode.
  Identity already separates genuinely different plugins
- **The manual-upload path deliberately skips conflict detection**: `InstallFromUpload` keeps its
  original behavior, and only store installs go through
  `InstallFromUploadWithOptions(RejectIdentityConflict: true)`. A manual upload is an explicit
  user-chosen file, and an author-spelling change during normal plugin iteration must not fail it
- **On conflict, return before writing anything**: `package.go` used to silently fall through to
  `Update` on a collision — overwriting the ZIP, re-extracting after `os.RemoveAll(staticDir)`,
  and rewriting the DB row in place (keeping the original ID and status). The original plugin was
  destroyed without a word, the newcomer inherited all of its `plugin_storage` data, and songs
  imported by the original (`songs.plugin_entry_path`) were reattributed to the newcomer
- **The store's `has_update` must use `CompareVersion(...) > 0`, never string `!=`**: on a
  collision the two versions differ by definition, so string comparison shows "update available"
  forever, and clicking it swaps the local plugin for a different author's plugin
- **Frontend in-place install-state updates must match on `(entryPath, identity)`**
  (`RegistryPluginEntry.matches`): matching on entryPath alone lights up every same-named row as
  "installed". List item `key`s use `rowKey` (`entryPath|identity`) rather than the index —
  `_RegistryPluginItem` holds local `_installing` state, and index-based Element reuse makes the
  spinner jump to another row

### Plugin store result cache

Store paging and search both operate on the **complete plugin list** (server-side slicing +
substring filtering), so every page turn and every search-term edit used to trigger a full
registry fetch — up to 500 `plugin.json` files, 8 concurrent, 15s timeout each.
`registry_cache.go` adds a 5-minute in-process TTL cache over fetch results.

- **`RegistryService` must be held long-term by the handler** (`JSPluginHandler.registrySvc`).
  It used to be `NewRegistryService()` per HTTP request, which would never hit the cache
- **`proxyDown` therefore must be a per-call local**, not a `RegistryService` field. It remembers
  "the GitHub proxy has failed during this fetch"; as a singleton field it would pin the service
  to **direct connections forever** after one failure (never retrying even once the proxy
  recovers) and let concurrent requests interfere. That's why the private method signatures all
  carry `proxyDown *atomic.Bool`
- **The cache key must include every input that affects the result**: mode (single/all), registry
  URLs, tokens, source order, and `github_proxy`. Source order cannot be omitted —
  `FetchAndMergeMulti` uses it to decide which registry wins for equal versions. **Hash the token
  before putting it in the key**: cache keys reach logs and debug output and must not carry
  plaintext credentials
- **Install state is deliberately not cached**: `installed` / `has_update` / `conflict` are
  recomputed from the DB by `buildInstalledMap` on every request, so paging right after an
  install still shows fresh state and no invalidation is needed
- **Failures are not cached** (and don't disturb an existing entry): otherwise one network blip
  would pin the error for a whole TTL. `FetchAndMergeMulti` returns no error (per-source failures
  only become warnings), so that path instead **declines to cache an empty result**
- **Only "refresh" and "retry after failure" pass `force: true`** on the frontend; paging,
  search, initial load, and source switching do not. Source switching and config edits re-fetch
  naturally because the cache key changes; `UpdateRegistriesSetting` additionally calls
  `InvalidateCache()` to free up entry quota

### Scan title rules

- tag has a title → use `tag.Title` directly
- tag has no title → filename minus extension
- **Do not** apply "longest-common-substring dedup + concatenation" — it produces results like "Artist - Title" that redundantly stuff the artist into the title field
- Video container probe: when scanning containers like mp4/mov/m4v/mkv/webm/avi/ts/mpg/mpeg/flv/wmv/rm/rmvb/3gp, ffprobe detects whether a real video track is present (excluding the cover attached_pic) to set `songs.is_video`; the client uses this to render the picture / pick the cast mime
- **When verifying locally, don't put the music directory under `/tmp`**: the default `exclude_dirs` for
  `music_path` includes `tmp`, and `ShouldExcludeDir` matches **any path segment by directory name**, so
  the entire `/tmp/...` root gets excluded. The symptom is a scan that "completes successfully" with
  `discovered_files=0` — **no error, no warning** — easy to misread as your own change breaking things

### Streaming scan imports (large-library iron rule — songloft-org/songloft#447)

`doScanAndImport` imports **incrementally**: `scanBatchAccumulator` (`scan_batch.go`) groups results by
directory and calls `flushScanBatch` as soon as a directory is finalized. Do not revert to
"accumulate everything, then write once" — on a library of tens of thousands of tracks, progress stays
at 0 and not a single row is written for the entire extraction phase (tens of minutes), which users can
only read as a hang.

- **The finalization unit must be the directory, not a fixed count**: `fixSpamTags` groups by directory
  and flags spam tags when "more than half the files in this directory share the same
  `(title, artist)`", so it needs the whole directory as its sample. Cutting batches every 50 results
  splits one directory across several batches and makes the verdict drift with the cut points
- **`scanDirGroupCap` is the memory backstop**: a directory holding thousands of flat files finalizes
  early once it hits the cap, otherwise this degrades back to whole-library accumulation. The cost is
  that such a directory's spam-tag verdict uses a sub-sample, which is acceptable (the test is >50%
  and ≥3 matches)
- **Progress counters only advance after a batch lands in the DB** (`UpdateProgress` inside
  `flushScanBatch`). The extraction phase reports the current file via `SetCurrentFile` instead — never
  call `UpdateProgress` from the extraction phase, that double-counts
- **The write context and the cancellation context must be separate**: `doScanAndImport` derives a
  cancellable ctx from its argument (to kill ffmpeg subprocesses on cancel), but `flushScanBatch` must
  use the pre-derivation `flushCtx`. On cancel we still persist what was already extracted; using the
  cancelled ctx makes the transaction fail immediately, exhaust its three retries and mark everything
  failed — so "cancel keeps the work" silently does nothing
- **Cancel no longer discards extracted results** (behavior change after v2.12.0): results produced
  before the workers exit are still imported, and `sealAll` flushes leftover directory groups whose
  pending count never reaches zero. Previously the whole batch was dropped, so two consecutive cancels
  threw away twenty minutes of work

### Never `os.Stat` every directory entry (songloft-org/songloft#447)

`scanDirWithCue` / `collectDirNames` determine entry types via `resolveEntry` (`scanner.go`):
`DirEntry.Type()` from `os.ReadDir` comes straight from `d_type` on Linux at zero extra syscall cost,
and only symlinks and unknown types fall back to `os.Stat` (following symlinks, preserving
"symlinked directories are still recursed, dangling entries are skipped").

- A per-entry `os.Stat` adds tens of thousands of syscalls on a large library. That is exactly why
  `GET /scan/dir-names` exceeded the client's 10s timeout → `r.Context()` was cancelled → the walk
  aborted mid-way → 500, and **every retry started over from scratch**
- That endpoint therefore has two more layers of protection: a 60s TTL cache in `CollectAllDirNames`
  (`dirNamesMu` doubles as a single-flight lock against the frontend's retry storm), and
  `context.WithoutCancel(r.Context())` on the handler side — detaching from client cancellation so the
  first walk finishes and populates the cache, making retries instant. The cache lives on `Scanner`,
  and `app.go` rebuilds the Scanner when `music_path` changes, so it invalidates naturally
- On filesystems without `d_type`, `ReadDir` fills the type in via its own lstat, which is equivalent
  to the old behavior — no regression

### DSF (DSD) scanning

- `dsf` is a default scan format; keep both fallback lists in `app.go` aligned with migration `0039_scan_support_dsf.sql`, which appends it to existing configs idempotently (case-insensitive).
- `pkg/tag` reads DSF metadata and cover art; the raw DSD sample rate may be 8 times the PCM-equivalent value reported by ffprobe. Existing metadata tests handle this difference.
- Playback uses the existing libmpv / ffmpeg path; browsers and DLNA devices without DSD support need an explicit transcoding parameter such as `?format=mp3`; the current Flutter Web `AudioFormatHelper` does not add one automatically for DSF. DSF is outside the tag-writing matrix; use the existing fallback handling when writes are unsupported.

### Sidecar lyrics (.lrc)

- **Matching rules** (`FindSidecarLyricFile`): `<base>.lrc` / `.LRC` / `.Lrc`, then `<full filename>.lrc` / `.LRC` / `.Lrc`.
  The base variant wins. **Do NOT** switch to `ReadDir` + `EqualFold` traversal (O(songs × dir entries) is unacceptable).
- **Empty files are treated as not found**: `st.Size()==0` or a same-named directory is skipped. Reason: prevents
  an empty lrc from setting `lyric_source=file` + `lyric=""` which blocks frontend requests and plugin fallback.
- **Encoding handling** (`ReadSidecarLyric`): UTF-16 LE/BE detected by BOM → decoded via `x/text/encoding/unicode`;
  otherwise `tag.FixEncoding` (GBK fix).
- **Three-level scan skip short-circuit** (`needsSidecarLyricImport`):
  ① `LyricSource ∈ {file, manual}` → false (no IO);
  ② directory not in `ScanResult.LyricDirs` → false (in-memory map);
  ③ lrc file actually exists for this song → true (at most 6 Stats).
  Convergence: once imported, `lyric_source=file` → next scan hits ① short-circuit.
- **Runtime priority chain** (`GetSongLyric` handler):
  sidecar .lrc > DB url > DB payload > lyric search plugin. `manual` is not overridden by sidecar
  (`SidecarLyricForSong` excludes it).
- **`SyncSidecarLyric` does not write back to audio tags**: the .lrc file IS the persistent copy; embedding
  it into tags would leave a stale copy if the user deletes the .lrc. Also `WriteSongTags` is a full
  rebuild mode that reads/writes cover binary — heavy for a GET request.
- **`shouldApplyScanLyric` guard**: `manual` is never overwritten; empty new lyric does not wipe existing
  DB lyric/remote URL. This is a behavior change (previously re-import could clear lyrics), documented in CHANGELOG.
- **Sidecar hit replaces plugin translations** (`tlyric`/`rlyric`/`lxlyric`) entirely — no "main lyric from
  file + translation from plugin" merging because misaligned timelines are worse than missing translation.

### Tag writing (pkg/tag)

- `tag.WriteTag(filePath, opts)` dispatches by file extension; all formats write atomically with a temp file + `os.Rename`
- Support matrix:

| Format | Text fields | Lyrics | Cover |
|------|---------|------|------|
| MP3 | ID3v2.3 text frames | USLT | APIC |
| FLAC | Vorbis Comment | LYRICS | PICTURE block |
| M4A/MP4/M4B/M4V/MOV/3GP | iTunes atoms (©nam, etc.) | ©lyr | covr |
| OGG(.ogg/.oga/.opus) | Vorbis Comment | LYRICS | METADATA_BLOCK_PICTURE (base64) |
| APE | APEv2 text items | Lyrics | Cover Art (Front) (binary item) |
| WAV | RIFF LIST INFO | ICMT | **Not supported** (format limitation) |
| AIFF/AIF | ID3v2.3 (ID3 chunk) + NAME/AUTH | USLT (ID3 chunk) | APIC (ID3 chunk) |
- Unsupported formats → return `ErrUnsupportedWrite`; the caller **must** degrade to a log entry and **must not** block the main flow

### Audio fingerprints (fingerprint — cost-control rules)

Fingerprints (ffmpeg chromaprint) serve only two purposes: the "Duplicate detection" settings page and
an **optional** parameter for plugin lyric/cover search. It is an on-demand feature, **not** a required
step of scanning. Read the rules below before touching this area — `songloft-org/songloft#323` is exactly
what happens when they are missing: "scan reports completed but the CPU stays pinned at 100% forever".

- **Auto-fingerprint after scan is off by default**: business endpoint `GET/PUT /api/v1/settings/scan-auto-fingerprint`
  with body `{enabled: bool}`, config key `scan_auto_fingerprint`, default `false`. Checked at the top of
  `runAutoFingerprint` (`song_service.go`), symmetric with `scan_auto_create_playlists`. While off, users
  trigger `POST /scan/fingerprints` manually from the duplicate detection page
- **Failures must be persisted**: `songs.fingerprint_attempted_at` (unix seconds, 0 = never attempted).
  `ListLocalWithoutFingerprint` filters on `fingerprint = '' AND fingerprint_attempted_at = 0`.
  **Never** just log on failure — without the marker, AutoScanner (default 3600s) re-queues the same batch of
  doomed long/audio-less files for a full ffmpeg decode on every round. `ClearAllFingerprints` resets the
  marker, so "Recompute all" is the only way to retry failed items
- **120-second sampling cap**: `ExtractFingerprint` passes `-t 120` (constant `fingerprintSampleSeconds`),
  which is also the AcoustID de-facto standard. **Do not** remove it — decoding a 30-minute audiobook in
  full inevitably times out on a weak NAS; measured on one file: 3.8s full decode vs 0.35s for 120s sampling.
  30s timeout plus `cmd.WaitDelay` keeps a stuck ffmpeg child from pinning a worker
- **CUE tracks sample by range**: a CUE track's `file_path` points at the whole-disc image, so
  `cue_start_seconds/cue_end_seconds` must be passed through `-ss`; otherwise every track under the same
  image gets an **identical** fingerprint and they all flag each other as duplicates
- **Concurrency adapts to CPU**: `fpWorkerCount()` = `clamp(GOMAXPROCS/4, 1, 4)`. **Do not** revert to a
  hard-coded 4 — Go's GOMAXPROCS is cgroup-aware, so a CPU-limited Docker container converges correctly
- **Cancellable**: `POST /api/v1/scan/fingerprints/cancel` → `FingerprintService.Cancel()`. The fingerprint
  task uses its own `context.Background()` and **cannot** reuse the scan's cancelCh — `scanProgressManager.Complete()`
  already did `close(cancel); cancel = nil`, after which `GetCancelChannel()` returns nil. Songs cancelled
  mid-flight are not marked as attempted, so the next run resumes them
- **Dedup has a duration guard**: within one fingerprint, `ListDuplicateGroups` further clusters by
  `fingerprint_duration` (full-file length) using a 30-second tolerance between **adjacent** entries (0 = unknown never splits), because only the first 120 seconds are
  sampled and "audiobooks with a shared intro" can collide
- Migration `0029` clears legacy full-length fingerprints once (they are not comparable with 120s sampling);
  after upgrading they need to be recomputed
- `IsChromaprintAvailable` honors the `ffmpeg_path` config injected via `SetFingerprintFFmpegPath` instead of
  only searching PATH

### Play history (play_history -- per playback context)

Each "playback context" independently remembers the songs recently played in it (songloft-org/songloft#333).
A context is `(context_type, context_key)`: playlists are `("playlist", "<id>")`, facet dimensions are
`("artist", "Jay Chou")` and the like, reusing the 7 dimensions of `songFacetColumn` -- **no separate enum**.
Read all of the following before touching this area.

- **Ordinals are unreliable, so the read endpoint does not return "which track number"**: facet lists go through
  `GET /songs?artist=X` with a default ordering of `added_at DESC`, but `added_at` is a second-precision
  `DATETIME` and a bulk scan inserts sequentially in one transaction → hundreds or thousands of songs end up with
  **identical** `added_at`; `applyOrder` (`filters.go`) emits a single-column `ORDER BY` with **no `id` tie-breaker**.
  Computing "the Nth song" from that is mathematically indeterminate and would start the wrong song. **Do not**
  add a tie-breaker to `applyOrder` for this -- that changes existing pagination behavior and is a separate issue
- **`absIndex` matching the pagination offset relies on the SQLite planner**: `/songs/ids` (one full sort) and
  `/songs?limit&offset` (may use a bounded sorter) are two different queries, and their relative order among tied
  `added_at` values holds only because the planner happens to agree (verified consistent at 350 and 30000 songs).
  Tightening this would require adding `, id DESC` to `applyOrder`'s default ordering -- which affects every `/songs`
  pagination caller and is a separate issue. Likewise `ListSongIDsOrdered` and `GetPlaylistSongsPaginated` both order
  by `position ASC` with no secondary key; they only diverge if a playlist ends up with duplicate positions
  (e.g. an interrupted concurrent reorder)
- **Client playback = play the first song immediately, fill circularly in the background**: the history entry carries
  the full `Song`, so it becomes the queue and starts playing with **zero requests**; the background then fetches the
  ordered ID list (`GET /playlists/{id}/song-ids` for playlists, `GET /songs/ids` for facets), locates it via
  `indexOf`, and appends "everything after the target" followed by the wrapped "beginning…up to the target".
  `currentIndex` stays 0 throughout, so random mode's `_playedIndices` / `_preSelectedNextIndex` are untouched;
  playlists and all 7 facets share one code path
- **Only `type=play` is persisted**: writing hooks into the `context_type`/`context_key` parameters of the existing
  reporting endpoint `POST /songs/{id}/played` (no second reporting channel, zero extra client requests). `finish`
  is a duplicate write for the same song; **`skip` is especially dangerous** -- it reports the **previous** song, and
  by then the client's context may have switched to a new playlist, filing the previous song under the wrong context.
  The frontend omits the context for skip/finish and the backend only accepts `play` -- a double safeguard. A write
  failure only produces `slog.Warn`; **the status code is always 204**
- **50-entry cap, upsert + trim in one transaction**: deduplication relies on `UNIQUE(context_type, context_key, song_id)`
  plus `ON CONFLICT DO UPDATE`, not application-level lookups. The trim uses
  `id NOT IN (... ORDER BY played_at DESC, id DESC LIMIT ?)` -- `played_at` collides at second precision, so `id DESC`
  is the deterministic tie-breaker. `MaxPlayHistoryPerContext` is a Go constant; **do not** turn it into a config item
- **`context_key` is TEXT, so no foreign key to playlists is possible**: playlist deletion relies on
  `PlaylistService.clearPlayHistory` clearing it explicitly. **Batch deletion must only clear playlists that were
  actually removed** -- the repository layer skips `built_in`, so blindly iterating the input ids would also wipe the
  history of "Favorites" (hit during implementation; now covered by `TestPlaylistDeleteClearsPlayHistory`)
- **`sourcePlaylistId` must keep its signature**: it is a public JS-plugin contract
  (`plugin_host_dispatch.dart` exports `source_playlist_id`) and has 5 "now playing" highlight consumers. When the
  frontend generalized to `PlaybackContext` it was demoted to a **derived getter**, so those consumers needed zero
  changes. Prefs reads fall back to the legacy `player_source_playlist_id` and writes maintain both keys, keeping
  Android hot-update rollback safe

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
- ffmpeg remote-pull transcode paths (`services/radio_transcode.go` radio transcode, `services/url_transcode.go` transcode proxy) are also integrated: passed to ffmpeg via `-http_proxy`. **HTTP/HTTPS proxies only**; SOCKS5 is not supported (ffmpeg `-http_proxy` limitation)

### Private network proxy allowlist (/settings/proxy-private-allowlist)

- Background: the generic resource proxy `GET /api/v1/proxy?url=` blocks all internal / loopback / link-local addresses via `services.IsHostnameAllowed` by default (anti-SSRF), which rejects the "public Songloft proxying a WebDAV reachable only on the LAN" scenario (songloft-org/songloft#313)
- Business endpoint: `GET/PUT /api/v1/settings/proxy-private-allowlist` with body `{allowlist: []string}`, default `[]` (empty = keep blocking everything, behavior unchanged)
- Each entry is a single IP (`192.168.1.100`) or CIDR range (`192.168.1.0/24`); PUT validates via `services.ParseAllowlist`, returning 400 on invalid entries
- Decision: `services.IsHostnameAllowedWithAllowlist(hostname, allowlist)` — public addresses always pass, private IPs pass only when covered by an allowlist range; `localhost`/`.local`/empty hostnames are still string-blocked (the allowlist matches by IP/CIDR only)
- **Only affects the generic `/proxy`**; HLS reverse proxy (`hls.go`) still uses `IsHostnameAllowed(nil)`, semantics unchanged
- Implementation: `internal/services/whitelist.go` (`ParseAllowlist` / `IsHostnameAllowedWithAllowlist`) + `internal/handlers/proxy.go` (`ProxyHandler` holds a `*ConfigService`, config key `proxy_private_allowlist`)

### Audio transcode proxy (/proxy/transcode)

- Business endpoint: `GET /api/v1/proxy/transcode?url=&format=mp3[&bitrate=&duration=&user_agent=&referer=]` (`@Security BearerAuth`, token required; token must be the first query param to dodge speaker-firmware replacing `&` with space)
- The server fetches a remote audio URL and transcodes it to mp3 (CBR 320k) on the fly, streamed back. Use case: in miot's "no-import direct play" scenario, when an external search source returns a direct link in a format the speaker can't decode (webm/opus, songloft-org/songloft#394), the client pushes this endpoint URL to the speaker
- No disk persistence, no DB entry, no cache (consistent with no-import semantics; youtube direct links are re-signed each search, so caching yields zero benefit — replay benefit goes through the import path)
- SSRF protection: reuses `/settings/proxy-private-allowlist`, the same `IsHostnameAllowedWithAllowlist` check as the generic `/proxy`
- Output is mp3 CBR + `-write_xing 0` + `-map 0:a:0 -vn`: the pipe is non-seekable, the speaker estimates duration from byte count, CBR avoids premature track-switching (same trade-off as the seek stream)
- Concurrency is shared with seek/normalize streams via `seekStreamSem` (cap=4); returns 503 when full, no queueing
- ffmpeg pulls the remote URL directly (`-i <url>`), reusing the global http-proxy via `-http_proxy` (HTTP/HTTPS only)
- Implementation: `internal/services/url_transcode.go` (`StreamTranscodedURL`) + `internal/handlers/proxy.go` (`Transcode`; `ProxyHandler` holds a `*CacheService`)

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

### Volume normalization (`/songs/{id}/play?normalize=1`)

EBU R128 loudness normalization (songloft-org/songloft#315), removing loudness gaps between sources.
Currently only the miot plugin uses it (settings page "volume normalization" toggle → config key
`volume_normalize` → `buildSongURL` appends `&normalize=1&format=mp3`).

- The filter string is the package-level constant `loudnormFilter` in `internal/services`
  (`loudnorm=I=-16:LRA=11:TP=-1.5`, **single-pass** dynamic mode). On-disk transcoding and the live
  stream share it — **don't** inline it in either place: both paths must produce the same loudness
- Artifacts are keyed with a `norm.` marker (`transcodedFileName`) and are **not interchangeable**
  with non-normalized ones
- **Streams while transcoding when the artifact isn't ready** (`tryLiveTranscodeStream` →
  `StreamSeekedMP3`, `internal/handlers/music.go`): `GetOrTranscode` is synchronous, so
  time-to-first-byte equals the wall-clock time of transcoding the whole track. Two measurements: a
  whole-track loudnorm takes 20+ seconds and stalls the device's first play request for that entire
  time (songloft-org/songloft-plugin-miot#61 measured `dur_ms=22392/24348/22381`) — on the speaker
  this is "the first 20-odd seconds are blank", and since the plugin's auto-next timer starts the
  moment the URL is pushed, the tail gets cut by the same amount; and plain `?format=` / `?quality=`
  transcoding takes 5+ seconds before any sound on a weak CPU (N5095) when converting lossless to
  mp3 (songloft-org/songloft#442). Piping directly took time-to-first-byte from 10.03s to 0.088s in
  the normalize case. **Both `format` and `quality` are covered** (`SeekStreamOptions.Bitrate` lets
  the pipe faithfully express `?quality=` instead of being pinned to 320k); only applies when the
  target format is mp3 and it's not `media=video` / track extraction / CUE / HEAD, all other cases
  keep the original blocking path
- **`ForceTranscode` must be passed explicitly, never inferred from "the source extension isn't
  mp3"** (`SeekStreamOptions`): a plain `?format=mp3` neither seeks nor normalizes nor changes speed,
  so `StreamSeekedMP3`'s guard would judge it as "nothing to do" and refuse; and a fake mp3
  (extension `.mp3` but WebM/Opus content, songloft-org/songloft#300) is precisely a case where the
  extension *is* mp3 yet transcoding is mandatory — inferring from the extension would drop it into
  the `-c:a copy` fast path → the mp3 muxer rejects Opus → zero output → fallback, burning an ffmpeg
  process for no latency improvement at all
- **The generalized guard must be self-contained**: `tryLiveTranscodeStream` computes
  `normalize || bitrate > 0 || NeedsTranscodeForServe(...)` itself rather than relying on the `if`
  conditions at its two call sites (`serveLocal` / `serveCachedFile`) — getting it wrong burns a
  libmp3lame process on every single playback
- **Pipe streams must support Range, or seeking silently hangs** (`cbrRangePlan`,
  songloft-org/songloft#442): a chunked response with no `Content-Length` is judged un-seekable by
  browsers. Measured in Docker headless Chrome: after dragging the progress bar the browser issues
  **no new request at all**, `readyState` drops from 4 to 1, buffered stops growing, and `error` stays
  null — **playback silently hangs and the track must be re-started**. The progress bar's total length
  comes from the DB's `song.duration`, so nothing in the UI hints that this stream can't be seeked.
  Adding just `Accept-Ranges: bytes` does not help (measured: still one request, still hangs) — the
  browser needs the **total byte count** to map "second N" onto a byte offset. **Why it's computable**:
  the pipe always emits CBR (`-b:a`), so bytes and time are linear; combined with the known
  `song.duration` that yields both the total length and the seconds corresponding to any offset, and
  "start the stream at second N" is exactly `StreamSeekedMP3`'s `-ss` input seek. Range requests are
  therefore **genuinely** satisfied rather than papered over by "ignore Range, resend from the start"
  (which would desync the progress bar from the actual audio)
- **Four preconditions for enabling Range, all required** (see `planCBRRange`): `song.Duration > 0`
  (an unknown duration makes the total size unknowable, which is routine for remote tracks whose
  metadata hasn't been refreshed), re-encoding rather than `-c:a copy` (a copy source may be VBR mp3,
  where bytes and time aren't linear — guaranteed by `ForceTranscode`), no speed change (atempo alters
  the output duration, requiring a second layer of conversion), and no `?seek=` (that's a separate
  position mechanism for push-streaming clients; combining them only causes interference). When unmet,
  fall back to chunked with no length
- **Promising a `Content-Length` means writing exactly that many bytes** (`exactLengthWriter`):
  ffmpeg's actual output never exactly equals "bitrate × duration" — LAME's CBR uses frame padding to
  track the target bitrate, the last frame may be partial, and `-ss` only aligns to frame boundaries,
  leaving an error of a few frames. Writing less makes the client treat it as a prematurely closed
  connection; writing more violates the HTTP/1.1 length contract. Hence truncate the overflow and pad
  the shortfall with zeros. **On overflow it must return an error to break `io.Copy`**
  (`errExactLengthReached`) rather than silently discarding: a closed range (`bytes=N-M`) wants only a
  slice while ffmpeg runs to the end of the source, so discarding silently lets a `bytes=0-100`
  request burn a full-track transcode
- **A 206's `WriteHeader` must be deferred until the first byte is actually written**
  (`deferredStatusWriter`): once WriteHeader is called the response is committed and can no longer be
  downgraded, while `StreamSeekedMP3`'s contract is "only write after `Peek(1)` confirms ffmpeg really
  produced output" — "first Write" is precisely that safe moment. For the same reason the fallback path
  must Del `Content-Length` / `Content-Range` / `Accept-Ranges`; a leftover length would conflict with
  the one `http.ServeFile` computes itself
- **Wrapper writers must forward `Flush()`**: the services side is `io.Copy(&flushingWriter{w: w}, ...)`
  and `flushingWriter` decides whether to flush via the **type assertion** `w.(http.Flusher)`. Without
  that method on `exactLengthWriter` / `deferredStatusWriter` the assertion is blocked, bytes pile up
  in the buffer instead of going out — and **the time-to-first-byte win evaporates**
- **The live stream deliberately does not kick off a background transcode** to fill the cache:
  running two ffmpeg processes over the same track doubles CPU on a weak NAS while the user is
  waiting for sound right now. Cache artifacts are produced by `?prefetch=1` (carrying the same
  format/quality/normalize)
- **`seekStreamMaxConcurrent` was *not* raised (still 4) when #442 was wired in**: plain transcoded
  playback happens an order of magnitude more often than seek/normalize so the slots fill up more
  easily, but #442's reported environment is exactly a weak CPU — widening concurrency hurts it,
  while falling back to the blocking path when full is itself lossless (just the pre-fix behavior)
- **Prewarming must carry normalize**: `prepareSongPlayback`'s `normalize` parameter must not be
  dropped, and the short-circuit check needs `!normalize` — for an mp3 source with `format=mp3`,
  `NeedsTranscodeForServe` is false, so without it prewarming returns immediately and does nothing
  (the other half of #61's root cause; not a single `prefetch ready` line appears in the log)
- Plugin-side counterpart: in random mode `PlaylistManager.reserveNextIndex()` locks "the track that
  was prewarmed" to "the track that will actually play". `getNextIndex()` re-rolls on every call, so
  prewarming and advancing each calling it once inevitably warms the wrong track (measured 492/500 mismatches)

### Server-side seek stream (`/songs/{id}/play?seek=<seconds>`)

Exists to express a resume position for push-style clients that can only pull a URL from the
beginning and don't support HTTP Range. A Xiaomi speaker handed a URL via `player_play_url` can
only play from the start of the stream, so "resume at second N" can only be expressed by having
the server produce a stream that **begins at second N** (songloft-org/songloft-plugin-miot#60:
firmware ignores pause → escalate to stop → device-side media context is lost → resuming can
only re-push the URL). Implemented in `internal/services/seek_stream.go`, structurally identical
to `radio_transcode.go` (`Peek(1)` confirms real output before committing the response,
otherwise a sentinel error lets the handler degrade losslessly to serving the whole file).

- **Always outputs MP3**: mp3 sources use `-c:a copy` (input seek; measured 0.14s for a whole
  track, near-zero CPU), everything else `libmp3lame`. **Don't** grow this into a multi-format
  matrix — browsers have Range; this parameter only serves push-style clients
- **Re-encoding must be CBR (`-b:a 320k`)**: a pipe isn't seekable, so the Xing frame can't be
  rewritten (hence the explicit `-write_xing 0`) and clients can only estimate duration from
  "first-frame bitrate × byte count". **Don't** switch to `-q:a 0` like elsewhere in the project:
  a measured 105.4s stream gets estimated as 97.6s, and a speaker may declare it finished early
- **`-map 0:a:0` must stay**: the mp3 muxer accepts a single audio stream; a dual-track `.mka`
  (#298) without explicit track selection fails outright, showing up as a silent fallback to
  "play from the start" with a single warn line — extremely hard to attribute
- **Only applies to local songs and already-cached remote songs**: on a cache miss, obtaining a
  local file requires synchronously downloading the whole track, which would stall the resume
  keypress for an entire download. Radio (live, no position), `media=video` (`-vn` drops the
  picture) and HEAD all ignore it
- **seek must be clamped to at least 3s before the end** (the same guard in `parseSeekSeconds`
  and in the plugin's `playCurrent`): past the end of file → ffmpeg produces nothing → the
  fallback kicks in → **the whole track replays from the start**, which is worse than ignoring seek
- **Doesn't hold `transcodeSem`** (the process lives for the remaining track duration and would
  starve other transcodes); uses a separate `cap=4` semaphore in the same file that degrades
  immediately instead of queueing. `exec.CommandContext` also carries a "remaining duration +
  5min" hard timeout to reap orphaned processes. That semaphore is **shared** with the live
  normalization stream above, the speed stream, and the plain transcode stream (#442) —
  `StreamSeekedMP3` is the same function — so 4 is the total across all of them.
  How long a slot is held depends on how fast the client drains the stream, not on track duration —
  speakers buffer greedily; a measured 4-minute track streamed out and exited in 10 seconds.
  The slot is held by `io.Copy`, and **cancelling the ctx does not interrupt it** (io.Copy only
  watches for read-side EOF / write-side error), so registering with playActivity can only kill
  the ffmpeg process to save CPU — it can't release the slot early
- **When the remaining duration is unknown (`songs.duration == 0`) the hard timeout must not fall
  back to 5 minutes** (see `seekStreamUnknownDurationTimeout`): for a whole-track stream that's a
  hard ceiling rather than a grace period, so a client reading in real time gets killed at the
  5-minute mark while the response closes normally and the client thinks playback finished.
  `duration == 0` is a normal occurrence (remote song metadata not yet refreshed; a local file
  where neither the tag nor ffprobe yields a duration)
- The response is chunked: no `Content-Length`, no `Accept-Ranges`, `Cache-Control: no-store`
- Plugin-side counterpart: `PlaylistManager.streamSeekOffsetSec` remembers where the current
  stream starts. To the device a seek stream is "a new stream starting at 0", so the
  `play_song_detail.position` it reports is only an in-stream offset and **every consumer must
  add the offset back** (`resolvePlayerStatus` in `handlers/playlist.ts`, `resetAutoNextTimer` in
  `voicecmd/engine.ts`), otherwise the web progress bar drops back to 0 after resuming and
  auto-advance is delayed by a whole seek duration

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

