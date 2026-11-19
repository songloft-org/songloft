# AGENTS.md

This is Songloft's AI development entry point: shared hard rules, common commands, and task navigation, with business details loaded on demand. Code is the source of truth for directories, dependencies, APIs, and schemas. Before editing, read applicable `LESSONS.md` entries, local submodule instructions, and relevant sections from the [business-constraint index](#business-constraints) below. See the [Chinese version](AGENTS.md).

## Documentation Map

| Need | Go to |
|---|---|
| AI workflow, validation evidence, Issue replies, retrospectives | [docs/en/ai-development.md](docs/en/ai-development.md), [LESSONS.en.md](LESSONS.en.md) |
| Business constraints and historical rationale | [docs/en/pitfalls.md](docs/en/pitfalls.md) (task index at the end) |
| Architecture / build / deployment | [Overview](docs/en/architecture.md), [Backend](docs/en/architecture_backend.md), [Frontend](docs/en/architecture_frontend.md) |
| Database operations | [docs/en/database_migrations.md](docs/en/database_migrations.md) |
| API response format / Swagger UI | [docs/en/api_response.md](docs/en/api_response.md) / `/swagger/index.html` in dev mode |
| Frontend gotchas / UI verification | [docs/en/frontend_gotchas.md](docs/en/frontend_gotchas.md) |
| Docs site maintenance | [docs/en/development.md](docs/en/development.md) |
| JS plugins / plugin registry | [Plugin development guide](docs/en/js-plugin-development-guide.md), [Plugin registry authoring](docs/en/plugin_registry.md), `plugins/toolchain/README.md` |

---

## Project Overview

Songloft is a self-hosted local music server that supports both **server deployment** and **Bundle local mode** (embedding the Go backend into the client, so no separate server deployment is required). It has a multi-repo structure:

| Directory | Tech | Description |
|------|------|------|
| `/` | Go 1.26 + Chi v5 + SQLite | Backend API service (default port 58091, account admin/admin) |
| `/mobile` | Go + gomobile | Mobile binding entry point for the Go backend (gomobile bind; exports Start/Stop/IsRunning/GetPort) |
| `/clients/player` ([separate repo](https://github.com/songloft-org/songloft-player)) | Flutter 3.41+ / Dart 3.7+ | Cross-platform frontend (6 platforms); its repository builds its own Bundle client packages |
| `/clients/player-lynx` ([separate repo](https://github.com/songloft-org/songloft-player-lynx)) | TS + ReactLynx | Android/iOS/HarmonyOS/Web client; its repository builds Android/iOS Bundle packages; Windows/macOS candidates are pending consolidated acceptance; separate main dev builds and v* releases (submodule) |
| `/clients/tv` ([separate repo](https://github.com/songloft-org/songloft-tv)) | Kotlin | Android TV client (submodule) |
| `/plugins/toolchain` ([separate repo](https://github.com/songloft-org/plugin-toolchain)) | TS + pnpm | JS plugin development toolchain (SDK / Builder / scaffolding) |
| `/plugins/src` | TS | JS plugin source (a collection of submodules; each plugin distributes releases from its own repo) |
| `/pkg/tag` | Go | Audio metadata **read/write** library (extends the upstream tag library with MP3/FLAC writing) |
| `/integrations/home-assistant` ([separate repo](https://github.com/songloft-org/home-assistant-addon)) | HA add-on | Home Assistant add-on (thin layer reusing the Docker image). **The `repository.yaml` manifest must sit at that repo's root**, which is exactly why it was split out (#340); design/pitfalls: see [README](integrations/home-assistant/README.md) |
| `/tools/ffmpeg-builder` ([separate repo](https://github.com/hanxi/ffmpeg-builder)) | Docker | Minimal-image builder for statically compiled ffmpeg/ffprobe (submodule); used for download transcode / audio fingerprinting |
| `/tools/tracely` ([separate repo](https://github.com/hanxi/tracely)) | Go + Vue | Self-hosted frontend monitoring backend (install/upgrade tracking); the backend reports via its Go SDK (submodule; the SDK dependency comes from go.mod) |

The parent repository documents backend implementation, shared host capabilities, interface contracts, and conventions used across repositories. Each submodule keeps its business implementation, device compatibility, historical issues, and validation matrix in its own root `AGENTS.md` (with a synchronized `AGENTS.en.md`). Read local instructions before editing a submodule and check the parent instructions when host interfaces are involved. Commit the submodule first, then update its pointer in the parent.

## AI Development Workflow

- Once assigned a task, investigate, implement, validate, review, and deliver continuously without separate stage approvals. Ask for essential missing information, product decisions, or scope expansion. Commits, pushes, formal releases, and Issue writes follow distinct authorization; do not ask again for already authorized actions.
- After pushing an Issue-related fix, proactively prepare and show the reply draft and target Issue for comment approval; post directly when applicable comment authorization already exists. Update drafts as CI and release status change without waiting for the user to ask. See [Linked Issue Replies](docs/en/ai-development.md#linked-issue-replies).
- Read applicable `LESSONS.md` entries before work and record evidence-backed lessons afterward, synchronizing English/Chinese. Three independent events on distinct dates make promotion candidates; re-reviewing one incident does not increase counts. Existing hard rules apply immediately.
- Unresolved Issue triage includes parent and all submodules, with recursive discovery, deduplication, and full pagination. Use `owner/repo#number` and report gaps; assess bodies, all comments, current code, and validation rather than treating open status as proof of an unfixed bug.
- Bind validation to repository, SHA or uncommitted-worktree fingerprint, environment, commands, and results; list relevant `Not run` checks. Changes require rerunning affected checks; follow required CI for the same SHA, and never count cancelled/skipped as passed. See the [AI workflow](docs/en/ai-development.md).

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
make build-frontend-web-embedded   # Lynx Web embedded in Go (hides the API-address UI)
make build-frontend-web            # Standalone Flutter Web
make build-frontend-flutter-web-embedded # Legacy embedded Flutter Web (overwrites shared output)
make build-frontend-{linux,windows,macos,android,ios,all}

# Go mobile binding reference targets (client repositories pin the backend SHA and package apps)
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

## Code Formatting (Hard Rule)

After every code change, you **must** format the code before committing:

- **Go**: run `gofmt -w .` from the project root
- **Dart**: run `dart format lib/ test/` from `clients/player/`

## Database Conventions (Hard Rules)

Access stack: **goose migrations + sqlc fixed SQL + squirrel dynamic SQL + Repository + UnitOfWork**. For the complete procedure see the [database operations guide](docs/en/database_migrations.md).

- **Changing the schema** → `internal/database/migrations/000N_xxx.sql`, executed automatically by `goose.Up` at startup; **do not** manually `ALTER data/songloft.db`
- **Adding fixed SQL** → `internal/database/queries/{table}.sql` + `make sqlc`; the generated output `internal/database/sqlc/` must be committed
- **Dynamic SQL (variable-length WHERE/SET)** → use squirrel inside `*_repository.go`; do not concatenate strings
- **Cross-table writes** → `db.RunInTx(ctx, func(ctx, uow))` to obtain `uow.Songs/Playlists/...` under the same `*sql.Tx`; **do not** call `BeginTx` manually in the service layer, or you'll hit SQLITE_BUSY
- **Error semantics** → repository misses uniformly return `database.ErrNotFound`; services distinguish with `errors.Is`
- **Testing** → use `testutil.OpenMemoryDB(t)` to run a real `:memory:` DB + real Repository; **do not** hand-write a mockDB
- **Built-in data** → migrations preload playlists id=1 "Favorites" and id=2 "Radio Favorites" (`labels=["built_in"]`), plus default config for `music_path / jwt_secret / source_*`. Remember to subtract these when asserting row counts in tests
- Frequent sqlc failures (CJK comments after `-- name:`, outer-column qualification in self-referencing subqueries) are covered in [Common sqlc Errors](docs/en/database_migrations.md#sqlc-common-errors)

## Backend Coding Conventions

- Standard Go layout (`internal/` guards against external dependencies), Chi v5 routing, JWT dual-token
- Dependency injection: ordinary reads/writes use Repository interfaces; cross-table transactions reuse `RunInTx + UnitOfWork` (for example, `PlayHistoryService` receives `database.DB`). Services **must not** call `BeginTx` themselves or bypass repositories with SQL
- Logging: standard library `slog`; HTTP errors: uniformly `respondError`
- **API response format**: return RESTful data directly, **no** `{code, data, message}` envelope; errors are uniformly `{"error","detail"}`. For the full spec see [docs/en/api_response.md](docs/en/api_response.md)
- Test files `*_test.go` live in the same directory as the source

## API Documentation Conventions (Hard Rules)

**Every handler registered in `internal/app/routers.go` (including sub-registration functions such as `RegisterStaticRoutes` / `RegisterAPIRoutes`) must have swag annotations.** The API docs are generated by [swaggo/swag](https://github.com/swaggo/swag) from these annotations and are the single source of truth for frontend development and external integration.

Required fields (every handler has at least these 7):

```go
// @Summary <one-line summary in Chinese>
// @Description <detailed description; clarify side effects / defaults / error-code trigger conditions>
// @Tags <business group, in Chinese>
// @Produce json
// @Success 200 {object} <return type> "<description>"
// @Security BearerAuth
// @Router /<path> [<method>]
```

- Endpoints with a request body add `@Accept json` and `@Param request body <type> true "<description>"`; path/query parameters use `@Param <name> path/query <type> true/false "<description>"`; obvious error paths add `@Failure 400/404/500 {object} map[string]string "..."`
- **Public endpoints** (no token required, e.g. health checks) omit `@Security BearerAuth`
- Reuse existing business tags (「歌曲管理」「歌单管理」「电台与 HLS」「扫描管理」「配置管理」「缓存管理」「JS 插件」「JS插件管理」「数据备份」「设置」「系统升级」「认证管理」「系统管理」「资源代理」「主题包」「播放历史」「标签管理」); do not casually invent new tags
- **`@Router` paths must NOT contain the `/api/v1` prefix**: `main.go` already declares `// @BasePath /api/v1`; including it generates `/api/v1/api/v1/...`
- Multi-alias / catch-all: write one `@Router` line per alias; do not list HEAD separately; for `r.HandleFunc`, list all methods actually possible; note runtime-determined dynamic paths in `@Description`
- **No exemptions**: dynamic-route catch-alls, static-resource handlers, and reverse-proxy endpoints must also have swag; use `@Description` to explain what it is and why the OpenAPI schema is imprecise

After changes you must run `make swagger` and commit `docs/swagger.json`, `docs/swagger.yaml`, and `docs/docs.go`; confirm in the output that the new `@Router` and request/response types were generated, check `grep '<path>' docs/swagger.json`, and eyeball the endpoint in `/swagger/index.html` when needed.

## Configuration Endpoint Conventions (Hard Rules)

**User-visible feature toggles always go through business endpoints; the generic KV store is only an admin back door.**

- Standalone config defaults to `/api/v1/settings/<kebab-case-name>`: strongly typed JSON (not `{value: string}`); defaults and side effects are handled inside the handler, so the frontend need not POST-create first; naming pattern `Is<Name>Enabled/Set<Name>Enabled` + `Get<Name>Setting/Update<Name>Setting`
- Config strongly coupled to a module's action endpoints stays under `/api/v1/<module>/config` (e.g. `/cache-manage/config` shares `CacheService` with `stats/clean`); do not force it into `/settings/`
- `/api/v1/configs/{key}` is only for generic editors such as `config_manager.dart`; new business features must not call it directly (PUT returns 404 for a missing key and has no strong typing, defaults, or side effects). When keeping dual entry points, the side effect must also be hooked into `configHandler.SetOnConfigChanged`
- Client: business Providers go through `SettingsApi`; `ConfigApi` is only for admin UI
- Full path styles, data shapes, and historical decisions: [Business Constraints · Configuration endpoints](docs/en/pitfalls.md#config-endpoints)

## Documentation Conventions (Hard Rules)

- **Bilingual**: `README.md` ↔ `README.en.md`, `AGENTS.md` ↔ `AGENTS.en.md`, `docs/<name>.md` ↔ `docs/en/<name>.md`. Any add/modify/delete of content, structure, or links must land in both versions; confirm the counterpart exists first, and make single-language exceptions intentional
- **Docs site**: read [docs/en/development.md](docs/en/development.md) before editing. `quick-start` / `changelog` are generated from the root `README` / `CHANGELOG` and must not be hand-edited; edit submodule-synced pages in the source repo, then bump the pointer; `docs/repowiki/` is tracked hand-maintained Markdown and is edited directly

## Git Commit Conventions

- **Commit directly to `main`**; do not create feature branches or open PRs
- Commit messages **must not** include a `Co-Authored-By` trailer
- Follow Conventional Commits: `type(scope): description`, preferring Chinese for the description and body
- Commits referencing a GitHub Issue must include the reference. The short form `#123` only points to an Issue in **the current repo**; cross-repo references always use the full `owner/repo#123` (submodules referencing parent-repo Issues must also use the full path)

## Build and Deployment

- Build tags: `dev` (includes Swagger + pprof) / `lite` (no embedded frontend) / no tag (full, embeds Lynx Web); `VERSION=dev` automatically enables `-tags dev`
- `VERSION` and `BUILD_TYPE` are orthogonal (`lite` / empty i.e. `full`); **do not** use mixed values such as `BUILD_TYPE=dev`. The frontend embed path is always `clients/player-build/web-embedded`
- Full build parameters, SPA fallback, and sub-path deployment: [Architecture · Build System](docs/en/architecture.md#build-system); Bundle deployment modes: [Frontend Architecture · Deployment Modes](docs/en/architecture_frontend.md#deployment-modes); Docker hot swap and PUID/PGID: [Architecture · Docker](docs/en/architecture.md#docker-entrypoint) and [Quick Start](docs/en/quick-start.md)
- The upgrade check `/api/v1/upgrade/check` also works outside Docker and returns `is_docker`; upgrade execution and rollback remain Docker-only
- Platform differences (audio backend, Android/iOS/macOS, etc.): [Frontend Gotchas](docs/en/frontend_gotchas.md) and [Frontend Architecture · Platform Adaptation](docs/en/architecture_frontend.md#platform-adaptation)

## Frontend UI Verification

When you need to actually exercise a change in the UI, use Dockerized headless Chrome; the full commands, script shape, and assertion requirements are in [AI Development · Frontend UI Verification](docs/en/ai-development.md#frontend-ui-verification). For Flutter Web semantics, merged labels, deep links, and other driving pitfalls, see [Frontend Gotchas · Driving Flutter Web](docs/en/frontend_gotchas.md#drive-flutter-web).

## JS Plugins

- Source at `plugins/src/<name>/`; artifacts are in each plugin repo's GitHub Releases; create a plugin with `npx create-songloft-plugin@latest` (WebView / Lynx templates)
- Sandbox: QuickJS. The bridge is dispatched by `HandleBridgeCall` in `internal/jsplugin/api_bridge.go`; namespaces: `storage`, `persistent-storage`, `songs`, `playlists`, `tags`, `comm`, `plugin`, `command`, `jsenv`, `fs`, `net`, `websocket`. `fetch`, `console`, and timers are host built-ins, not bridge calls
- Routing `/api/v1/jsplugin/{entry_path}/...`; common assets `/api/v1/jsplugin-assets/*` are injected by `injectHTMLHead` in the order theme.css → components.css → common.js; new assets must be registered in `assetVersions`
- The object literal at the end of `common.js` is the single source of truth for what `window.SongloftPlugin` exposes; plugins use `@songloft/client-sdk` types instead of hand-writing host API declarations. Host bridge namespaces: `host`, `player`, `cookies`, `favorite` (plugins must call `favorite.refresh` after changing favorites)
- Lynx plugins use `@songloft/lynx-plugin-sdk` and `NativeModules.SongloftPluginBridge`, not `window.SongloftPlugin`
- Permissions: manifest `permissions` are validated at runtime; **a new permission must be synced in three places: host `AllPermissions`, toolchain `VALID_PERMISSIONS`, and scaffold `AVAILABLE_PERMISSIONS`**. `net` does not cover `net:insecure-tls`
- The background pump must stay event-driven (`asyncSignal` + `GetNextTimerDeadline`); do not revert to fixed polling. `executeInEnv` must call `signalAsync` on return
- UDP/TCP each count toward an independent 8-sockets-per-plugin quota; active sockets prevent idle eviction. TCP only allows private/loopback/link-local addresses, and data is base64
- Lyrics/cover providers (`songloft.lyrics` / `songloft.covers`) need no permission, use first-match-wins, and survive idle eviction
- Full APIs, templates, and host implementation constraints: [Plugin Development Guide](docs/en/js-plugin-development-guide.md); private registry auth: [Plugin Registry Authoring](docs/en/plugin_registry.md#private-registry-authentication)

---

<a id="business-constraints"></a>

## Business Constraints (Required by Task)

Before editing these modules, **read the linked sections**; read all applicable sections for cross-module changes. This table is only a reminder; full constraints and historical evidence live in [Business Constraints and Pitfalls](docs/en/pitfalls.md). Follow the `LESSONS.md` workflow for new lessons and maintain topic details in the relevant documents, not the entry point.

| Change area | Key reminder | Required reading |
|---|---|---|
| Plugin store identity | Return on conflicts before writing; match install state by entry_path + identity. | [Details](docs/en/pitfalls.md#plugin-identity) |
| Plugin store cache | Cache registry data, not install state; hash tokens in keys and isolate proxyDown per call. | [Details](docs/en/pitfalls.md#registry-cache) |
| Scan titles and test directories | Prefer tags by default, honoring filename mode; keep test music outside /tmp. | [Details](docs/en/pitfalls.md#scan-title) |
| Streaming scan imports | Finalize by directory; preserve extracted work on cancel with a separate flushCtx. | [Details](docs/en/pitfalls.md#scan-batches) |
| Directory traversal | Avoid per-entry Stat; preserve directory-name caching, single-flight, and cancellation detachment. | [Details](docs/en/pitfalls.md#directory-walk) |
| DSF / DSD | Keep scan defaults and migrations aligned; players without DSD support need explicit transcoding. | [Details](docs/en/pitfalls.md#dsf) |
| Sidecar lyrics | Probe fixed candidate paths; never overwrite manual lyrics or embed sidecar lyrics into audio tags. | [Details](docs/en/pitfalls.md#sidecar-lyrics) |
| Tag writing | Write atomically through sibling temp files; unsupported formats must not block the main flow. | [Details](docs/en/pitfalls.md#tag-writing) |
| Audio fingerprints | Off by default; cap samples at 120 seconds, persist failures, support cancellation and CPU-scaled concurrency. | [Details](docs/en/pitfalls.md#fingerprints) |
| Play history | Record only play events per context; upsert and trim to 50 in one transaction, never trust ordinals as stable positions. | [Details](docs/en/pitfalls.md#play-history) |
| HLS proxy | Off by default; preserve same-origin and SSRF checks, using relative rewritten URLs. | [Details](docs/en/pitfalls.md#hls-proxy) |
| Outbound HTTP proxy | Use proxy-aware clients; ffmpeg supports only HTTP/HTTPS proxies. | [Details](docs/en/pitfalls.md#http-proxy) |
| Private proxy allowlist | An empty allowlist keeps private networks blocked; it affects resource/transcode proxies, not HLS. | [Details](docs/en/pitfalls.md#private-allowlist) |
| Remote URL transcoding | No disk or DB persistence; share 4 slots with seek/normalization streams and return 503 when full. | [Details](docs/en/pitfalls.md#url-transcode) |
| Music cache | Stream without waiting for full downloads; failed cache transcodes keep originals and do not change explicit downloads. | [Details](docs/en/pitfalls.md#music-cache) |
| Normalization and live transcoding | Share configured loudness and cache keys; preserve Range, exact byte counts, deferred 206 and Flush. | [Details](docs/en/pitfalls.md#normalization) |
| Server-side seek | Combine seek with Range; preserve CBR, the 3-second end guard and the unknown-duration timeout. | [Details](docs/en/pitfalls.md#seek-stream) |
| Song persistence | Only for music the user legally owns; transcode or lyric-fetch failures must not block persistence. | [Details](docs/en/pitfalls.md#song-download) |
| Cross-device file moves | Use internal/fileutil.MoveFile, not bare os.Rename, to move temp files across mounts. | [Details](docs/en/pitfalls.md#file-move) |
