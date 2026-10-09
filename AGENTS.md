# AGENTS.md

本文件为 AI 编程助手提供 Songloft 项目的**入口信息**：项目结构、常用命令、铁律与踩坑总结。代码本身就是真实来源的内容（目录树、依赖、API 表、表结构）请直接看代码或下方链接的详细文档。

> **详细文档**：
> - 架构：[整体](docs/architecture.md) · [后端](docs/architecture_backend.md) · [前端](docs/architecture_frontend.md)
> - 专题：[数据库操作](docs/database_migrations.md) · [颜色系统](docs/color_system.md) · [API 响应格式](docs/api_response.md) · [快速上手](docs/quick-start.md) · [前端踩坑与铁律](docs/frontend_gotchas.md)
> - 插件开发：见 `plugins/toolchain/README.md`（独立仓库）
> - 插件源制作：[插件源制作指南](docs/plugin_registry.md)
> - API：开发模式启动后访问 `/swagger/index.html`

---

## 项目概述

Songloft 是自托管本地音乐服务器，支持**服务器部署**和**Bundle 本地模式**（将 Go 后端嵌入客户端，无需单独部署服务器）。多仓库结构：

| 目录 | 技术 | 说明 |
|------|------|------|
| `/` | Go 1.26 + Chi v5 + SQLite | 后端 API 服务（默认端口 58091，账号 admin/admin） |
| `/mobile` | Go + gomobile | Go 后端的移动端绑定入口（gomobile bind 用，导出 Start/Stop/IsRunning/GetPort） |
| `/clients/player` ([独立仓库](https://github.com/songloft-org/songloft-player)) | Flutter 3.29+ / Dart 3.7+ | 跨平台前端（6 平台），支持 Bundle 本地模式 |
| `/clients/player-lynx` ([独立仓库](https://github.com/songloft-org/songloft-player-lynx)) | TS + ReactLynx | Android/iOS/HarmonyOS/Web 预览客户端，独立 main dev 构建与 v* 发版（子模块，无桌面/Bundle） |
| `/clients/tv` ([独立仓库](https://github.com/songloft-org/songloft-tv)) | Kotlin | Android TV 客户端（子模块） |
| `/plugins/toolchain` ([独立仓库](https://github.com/songloft-org/plugin-toolchain)) | TS + pnpm | JS 插件开发工具链（SDK / Builder / 脚手架） |
| `/plugins/src` | TS | JS 插件源码（子模块集合，每个插件在自己仓库下分发 release） |
| `/pkg/tag` | Go | 音频元数据**读写**库（基于上游 tag 库扩展 MP3/FLAC 写入） |
| `/integrations/home-assistant` ([独立仓库](https://github.com/songloft-org/home-assistant-addon)) | HA add-on | Home Assistant 加载项（薄层复用 Docker 镜像，子模块）。**清单 `repository.yaml` 必须在那个仓库的根目录**，这就是拆出独立仓库的原因（#340）。设计/踩坑/版本同步见 [integrations/home-assistant/README.md](integrations/home-assistant/README.md) |
| `/tools/ffmpeg-builder` ([独立仓库](https://github.com/hanxi/ffmpeg-builder)) | Docker | 静态编译 ffmpeg/ffprobe 最小镜像构建器（子模块），供下载转码 / 音频指纹用 |
| `/tools/tracely` ([独立仓库](https://github.com/hanxi/tracely)) | Go + Vue | 自托管前端监控后端（安装/升级追踪），后端经其 Go SDK 上报；子模块，SDK 依赖走 go.mod |

---

## 常用命令

```bash
# 后端
make run            # 启动（dev 模式，含 Swagger）
make build          # 编译开发版（完整版，嵌入前端）
make build-lite     # 编译开发版（精简版，不嵌入前端）
make build-prod     # 编译生产版（完整版，嵌入前端）
make build-prod-lite # 编译生产版（精简版，不含前端）
make test           # 测试
make check          # fmt + vet + test
make sqlc           # 重新生成 sqlc 代码（改了 queries/*.sql 后必跑）
make swagger        # 重新生成 API 文档

# 前端构建（产物落到 clients/player-build/，供后端嵌入或独立部署）
make build-frontend-web-embedded   # 嵌入 Go 二进制用（隐藏 API 地址 UI）
make build-frontend-web            # 独立部署 web
make build-frontend-{linux,windows,macos,android,ios,all}

# Bundle 本地模式（Go 后端编译为移动端库 / 桌面端可执行文件）
make build-go-mobile-android       # Android .aar（gomobile bind，arm64 + arm + x86_64）
make build-go-mobile-ios           # iOS .xcframework（gomobile bind，arm64，仅 macOS）
make build-go-desktop-linux        # Linux 可执行文件
make build-go-desktop-windows      # Windows .exe
make build-go-desktop-macos        # macOS x86_64
make build-go-desktop-macos-arm64  # macOS ARM64

# 前端开发
cd clients/player && flutter run -d chrome          # standalone
cd clients/player && flutter run -d chrome --dart-define=DEPLOY_MODE=embedded
```

---

## 代码格式化（铁律）

每次修改代码后**必须**格式化，提交前确认无格式差异：

- **Go**：在项目根目录执行 `gofmt -w .`
- **Dart**：在 `clients/player/` 目录执行 `dart format lib/ test/`

---

## 数据库规范（铁律）

> 完整操作步骤见 [docs/database_migrations.md](docs/database_migrations.md)。

访问栈：**goose 迁移 + sqlc 固定 SQL + squirrel 动态 SQL + Repository + UnitOfWork**。

- **改 schema** → `internal/database/migrations/000N_xxx.sql`，启动时 `goose.Up` 自动执行；**禁止**手动 `ALTER data/songloft.db`
- **加固定 SQL** → `database/queries/{table}.sql` + `make sqlc`；生成产物 `database/sqlc/` 必须入库
- **动态 SQL（变长 WHERE/SET）** → 在 `*_repository.go` 内用 squirrel，禁止拼字符串
- **跨表写** → `db.RunInTx(ctx, func(ctx, uow))` 拿同一 `*sql.Tx` 下的 `uow.Songs/Playlists/...`；**禁止** service 层手 `BeginTx`，否则会 SQLITE_BUSY
- **错误语义** → 仓储未命中统一 `database.ErrNotFound`；service 用 `errors.Is` 判别
- **测试** → `testutil.OpenMemoryDB(t)` 跑真实 `:memory:` + 真实 Repository；**禁止**手写 mockDB
- **内置数据** → 迁移预置歌单 id=1「收藏」、id=2「电台收藏」（`labels=["built_in"]`），及 `music_path / jwt_secret / source_*` 默认 config。测试行数断言记得扣掉
- **`queries/*.sql` 里 `-- name:` 之后不要写中文注释** → sqlc 会把 query 注释当 doc comment 按**字节**嵌进生成代码，中文被截断成非法 UTF-8，`make sqlc` 直接失败（`error generating code: illegal UTF-8 encoding`）。注释写在 Go 仓储方法上
- **同表自引用子查询要给外层列限定表名** → `DELETE FROM t WHERE col = ? AND id NOT IN (SELECT x.id FROM t x ...)` 里外层裸 `col` 会被 sqlc 判为 `column reference is ambiguous`，写成 `t.col` 才过

---

## 后端编码约定

- 标准 Go layout（`internal/` 防外部依赖），Chi v5 路由，JWT 双 Token
- 依赖注入：service 层只接收 Repository 接口，**不接收** `DB`
- 日志：标准库 `slog`；HTTP 错误：统一 `respondError`
- **API 响应格式**：RESTful 直返，**禁止** `{code, data, message}` 信封；错误统一 `{"error","detail"}`。完整规范见 [docs/api_response.md](docs/api_response.md)
- 不用 ORM：固定 SQL → sqlc，动态 SQL → squirrel，跨表写 → `RunInTx + UnitOfWork`
- 测试文件 `*_test.go` 与源码同目录

---

## API 文档规范（铁律）

**所有在 `internal/app/routers.go`（含 `RegisterStaticRoutes` / `RegisterAPIRoutes` 等子注册函数）里注册的 handler 方法，必须有 swag 注释**。后端 API 文档由 [swaggo/swag](https://github.com/swaggo/swag) 从注释生成，是前端开发与外部集成的唯一来源。

### 必填字段（每个 handler 至少有这 7 项）

```go
// @Summary <一行中文摘要>
// @Description <详细描述，可多行；说清楚副作用 / 默认值 / 错误码触发条件>
// @Tags <业务分组，中文>
// @Produce json
// @Success 200 {object} <返回类型> "<说明>"
// @Security BearerAuth
// @Router /<path> [<method>]
func (h *XxxHandler) Method(w http.ResponseWriter, r *http.Request) { ... }
```

- 有请求体的接口额外加 `@Accept json` 和 `@Param request body <type> true "<说明>"`
- 错误路径明显的接口加 `@Failure 400/404/500 {object} map[string]string "..."`
- 路径参数 / 查询参数用 `@Param <name> path/query <type> true/false "<说明>"`
- **公开端点**（无需 token，如健康检查）省略 `@Security BearerAuth`
- **业务 tag 命名**：复用现有 tag（「歌曲管理」「歌单管理」「电台与 HLS」「扫描管理」「配置管理」「缓存管理」「JS 插件」「JS插件管理」「数据备份」「设置」「系统升级」「认证管理」「系统管理」「资源代理」「主题包」「播放历史」「标签管理」），不要随手造新 tag
- **`@Router` 路径禁止包含 `/api/v1` 前缀**：`main.go` 已声明 `// @BasePath /api/v1`，swag 会自动给所有 `@Router` 路径加上该前缀。如果写了 `/api/v1`，生成的文档路径会变成 `/api/v1/api/v1/...`。正确写法是相对路径（如 `/songs/{id}/tags`），而非 `/api/v1/songs/{id}/tags`。

### 多别名 / catch-all 路由

- 一个 handler 注册了多条 alias 路径（如 `/songs/{id}/play` 与 `/songs/{id}/play.m3u8`）→ 每条 alias 单写一行 `@Router`
- HEAD 是 GET 的子集，**不单独列**；OpenAPI 不强制
- `r.HandleFunc(...)` 这种接受 ANY HTTP 方法的 catch-all → 列出所有实际可能的方法（`[get] [post] [put] [delete]`），每个一行 `@Router`
- 动态路径（`{entryPath}` 由运行时按已安装插件决定的）→ 在 `@Description` 里注明「动态路由，{xxx} 由运行时决定，OpenAPI 仅作占位」

### 改完必跑

修改 / 新增 handler 注释后必须跑 `make swagger`：会重新生成 `docs/swagger.json`、`docs/swagger.yaml`、`docs/docs.go`，**这些产物必须入库**。否则 `/swagger/index.html` 与代码不同步，前端按旧文档对接会踩坑。

### 验证

- `make swagger` 输出里搜索新加的 `@Router` 路径，确认 `Generating <Type>` 包含你新写的请求/响应类型
- `grep '<your-new-path>' docs/swagger.json` 应有命中
- 启动 `make run`，访问 `http://localhost:58091/swagger/index.html` 在 UI 里点开新端点目测

### 没有豁免

「凡 routers 注册即必注释」是绝对规则。哪怕是动态路由 catch-all、静态资源 handler、反代端点，也要写 swag——`@Description` 里把"它是什么、为什么 OpenAPI schema 不精确"说清楚即可。

---

## 配置接口规范（铁律）

项目里有两类配置接口，**用户可见的功能开关一律走业务端点**，通用 KV 仅作 admin 入口。

### `/api/v1/settings/<name>` — 孤立配置端点（前端业务功能默认走这里）

- 路径风格：`/settings/<kebab-case-name>`（如 `/settings/hls-proxy`、`/settings/music-path`、`/settings/http-proxy`、`/settings/library-browse`、`/settings/proxy-private-allowlist`）
- 数据形态：**强类型** JSON（如 `{enabled: bool}` 或聚合对象），不是 `{value: string}`
- 默认值：handler 内部承担（配置缺失时 GET 返回业务默认，PUT 时直接写入即可，**前端无需先 POST 创建**）
- 副作用：在 PUT 内部直接触发（如 `music_path` PUT 完异步 `onMusicPathChanged` 重建 Scanner）
- 归属：放进对应业务模块的 handler（如 hls-proxy 在 `HLSHandler`，music-path 在 `ScanHandler`），handler 同时持有 `*services.ConfigService` 完成读写
- 命名套路：`Is<Name>Enabled() / Set<Name>Enabled(bool)` 业务方法 + `Get<Name>Setting / Update<Name>Setting` HTTP handler + `/settings/<name>` 路由

### `/api/v1/<module>/*` — 业务模块聚合端点（含配置）

某些业务模块自带"动作端点+配置端点"组合（典型例子 `/cache-manage/{stats,clean,config}`），此时配置端点**保留在模块前缀下**，不强行拆到 `/settings/`。

- 适用场景：配置与该模块的其他动作端点强相关（如 cache 的 `config` 跟 `stats/clean` 共用同一个 `CacheService`）
- 选择依据：业界主流（AWS、GitHub、Discord）都是业务模块聚合；GitLab 那种"全局集中、模块分散"的混合模式同样接受
- 已有的例子：`/api/v1/cache-manage/config`（GET/PUT）
- **判定准则**：
  - **孤立**配置（不属于任何业务模块、或跨模块共享）→ `/settings/<name>`
  - **模块内**配置（与该模块动作端点强相关）→ `/<module>/config` 或 `/<module>/<sub-name>`

### `/api/v1/configs/{key}` — 通用 KV（admin 编辑器专用）

- 仅供前端 `config_manager.dart` 这种**通用配置编辑器**使用，让管理员手编任意 key/value 调试
- **新加业务功能不要直调** `/configs/{key}`：通用 PUT 在 key 不存在时返回 404，且没有强类型、没有副作用、没有默认值
- 业务化封装后，通用接口仍可改同一 key（保留双入口），但副作用必须同时挂在 `configHandler.SetOnConfigChanged` 回调里（参考 `routers.go` 里 `musicPathChanged`），保证两条入口语义一致

### 客户端约定

- `SettingsApi`（`clients/player/lib/features/settings/data/settings_api.dart`）封装所有 `/settings/*` 调用，业务功能 Provider 一律走它
- `ConfigApi` 只在 `config_manager.dart` 与「列出所有配置」这类 admin UI 里使用

### 历史决策记录

- 该规范在 2026-06 引入，背景：`hls_proxy_enabled` 默认未预置导致 PUT `/configs/{key}` 返回 404，发现项目里 `/configs` + `/settings/*` + `/cache-manage/config` 三种风格并存
- 选定方向：业务端点是用户可见入口的**唯一来源**，通用 KV 退化为 admin 后门

---

## 文档双语同步规范（铁律）

项目文档为**中英双语并存**，改任一语言版本时**必须同步改另一版本**，禁止只改一边导致中英内容漂移。

- **映射关系**：
  - `README.md` ↔ `README.en.md`
  - `AGENTS.md` ↔ `AGENTS.en.md`
  - `docs/<name>.md` ↔ `docs/en/<name>.md`（同名文件，英文版在 `docs/en/` 下）
- **判定准则**：凡是新增、修改、删除文档中的**内容/结构/链接**（正文、章节、表格、导航链接等），中英两版都要落地对应改动；仅英文表述本地化，结构保持一致
- **改前先确认对应文件是否存在**：`docs/en/` 下有同名文件则必须同步；README 一律有 `.en.md` 对应
- **例外**：某些内容天然只属于单一语言版本（如仅中文版的社区说明），无对应版本时不强制镜像，但应确保是有意为之，而非遗漏

---

## 文档站结构（docs/ — VitePress 自定义主题）

Songloft 文档站（`docs/`）用 **VitePress + 自定义主题**（`docs/.vitepress/theme/`），**不是默认主题**。改文档站前必须先分清两类页面，改错地方会白改：

- **自定义落地页（改数据，不改 markdown）**：首页 `docs/index.md` 仅一行 `<Landing />`，内容由结构化数据 `docs/.vitepress/data/*.ts`（安装方式 `downloads.ts`、功能 `features.ts`、文案 `landing-i18n.ts`）驱动，由 `docs/.vitepress/theme/components/landing/*.vue` 渲染。改落地页 → 改 `data/*.ts`（双语 `{zh,en}` 字段）；图标要对齐组件里的映射表（如 `LandingInstaller.vue` 的 `ICONS`）。
- **自动生成页（禁止手改）**：`docs/quick-start.md`、`docs/en/quick-start.md`、`docs/changelog.md` 由 `scripts/sync-docs.mjs` 从根 `README.md` / `README.en.md` / `CHANGELOG.md` 生成，已被 `docs/.gitignore` 忽略。要改正文 → 改源 `README` / `CHANGELOG`，`docs:dev` / `docs:build` 会先跑 `sync` 重新生成。**手改会被覆盖且不入库**。
- **子模块同步页（同样禁止手改，但源在别的仓库）**：`docs/addon/`、`docs/player/`、`docs/player-lynx/`（及英文 `docs/en/player-lynx/`）、`docs/plugin-toolchain/` 由 `sync-docs.mjs` 分别从 `integrations/home-assistant/`、`clients/player/docs/cn/`、`clients/player-lynx/`、`plugins/toolchain/` **子模块**同步，也都被 `docs/.gitignore` 忽略。要改正文 → **去对应子模块仓库改，再回主仓库 bump 子模块指针**（`git submodule update --remote <path>` + commit），否则文档站永远显示旧内容。两个要点：① `to:` 目标路径与源路径**刻意解耦**（如 `integrations/home-assistant/README.md` → `docs/addon/index.md`），因为 `/addon/` 这类对外 URL 已进 sitemap，不能跟着源仓库改名；② 子模块未 checkout 时 `sync` 只 warn 不 fail，页面会**静默消失**，所以 `static.yml` 的 submodule init 列表必须包含它们。
- **repowiki（`docs/repowiki/` — AI 手动生成，手动维护）**：这套 wiki（系统架构/数据模型/后端/前端/API/插件/部署/故障排除/开发指南等）是 **AI 基于源文件手动生成的**，仓内**没有生成脚本**、也不是外部服务自动产物，**入库的 markdown 即唯一真实来源**。因此任何工具（AI/人）直接编辑并 commit 即可，**不要等"重新生成"**。改代码相关内容时按需同步这些页面，与其他源文档一样对照代码保持准确（例如 `internal/jsplugin/assets/` 的公共资源改名后，repowiki 里引用旧文件名的地方也要一并改）。

---

## Git 提交约定

- **直接提交到 `main` 分支**，不新建功能分支、不走 PR 流程（本仓库约定）
- 提交信息**禁止**添加 `Co-Authored-By` 尾部标记
- 遵循 Conventional Commits 格式：`type(scope): description`，description 和 body 尽量用中文
- 关联 GitHub issue 的提交信息必须带 issue 引用
- issue 引用规则：短写 `#123` 永远指向**当前 commit 所在仓库**的 issue；只要引用的不是当前仓库的 issue，就必须写完整 `owner/repo#123`
  - 父仓库 `songloft-org/songloft` 的 commit 引用父仓库 issue：可写 `#155`，也可写 `songloft-org/songloft#155`
  - 子仓库（如 `pkg/tag`、`clients/player`、`plugins/toolchain`、`plugins/src/*`）的 commit 引用自身仓库 issue：可写 `#14`，也可写完整仓库路径
  - 子仓库的 commit 引用父仓库 issue：必须写完整路径，如 `songloft-org/songloft#155`，不能只写 `#155`（否则 GitHub 会解析为子仓库自身的 issue）
  - 任意跨仓库引用一律写完整路径，如 `songloft-org/songloft-player#14`

---

## 构建与部署

- 构建标签：`dev`（含 Swagger + pprof） / `lite`（精简版，不嵌前端） / 无标签（完整版，嵌 Flutter Web）
- `VERSION=dev` 时 Makefile 自动启用 `-tags dev`（无需手动传 `EXTRA_TAGS=dev`）
- 两个正交维度：**VERSION**（`dev` / `X.Y.Z`）控制是否为开发版；**BUILD_TYPE**（`lite` / 空即 `full`）控制是否嵌入前端。**禁止** `BUILD_TYPE=dev` 等混合值
- 嵌入路径是 `clients/player-build/web-embedded`（**不是** `clients/player/build/web-embedded`）
- SPA 回退：`internal/app/embed.go` 处理，文件不存在时返回 `index.html`
- 部署模式由 `--dart-define=DEPLOY_MODE=embedded|standalone` 切换，`AppConfig.isEmbedded` 是编译时常量，tree-shaking 会移除独立模式下的 API 地址 UI
- 子路径部署：启动时通过 `-base-path /xxx` 或 `BASE_PATH=/xxx` 配置；后端用 `http.StripPrefix` 在最外层剥离前缀，`embed.go` 运行时将 `<base href="/">` 替换为 `<base href="/xxx/">`；前端嵌入模式从 `Uri.base.path` 自动检测子路径

### Bundle 本地模式（v2.9.0+）

将 Go 后端嵌入 Flutter 客户端，用户无需单独部署服务器即可使用。编译时 `--dart-define=HAS_BACKEND=true` 启用。

- **移动端（Android/iOS）**：通过 `gomobile bind` 将 Go 后端编译为原生库（`.aar` / `.xcframework`），Flutter 通过 `MethodChannel('com.songloft/backend')` 调用
- **桌面端（macOS/Windows/Linux）**：Go 后端编译为独立可执行文件 `songloft-server`，Flutter 启动时作为子进程运行
- **Web**：不支持 Bundle 模式（仅远程服务器）
- 运行模式：`RunMode.local`（本地）/ `RunMode.remote`（远程），持久化到 SharedPreferences，启动时自动恢复
- 本地模式启动流程：申请存储权限 → 启动嵌入后端（`127.0.0.1:<port>`）→ 健康检查轮询（最多 10 次 × 300ms）→ 自动使用 `admin/admin` 登录
- `BackendLifecycle`（WidgetsBindingObserver）：App 前台恢复时自动重启后端，detached 时停止
- 关键入口：`mobile/mobile.go`（gomobile 绑定）、`clients/player/lib/core/backend/`（Flutter 侧抽象层）
- CI 产物命名：`songloft-bundled-{platform}-{arch}.{ext}`，4 个并行 Job（Android/Linux/Apple/Windows），失败不阻塞主 Release

### Docker 热替换规则（`scripts/docker-entrypoint.sh`）

Docker 镜像内含底包 `/app/songloft`，持久化 data 卷存放实际运行的 `/app/data/songloft`。容器启动时 entrypoint 决定是否用底包覆盖 data 目录：

**核心原则：底包代表用户意图；dev/正式或 full/lite 不一致时用底包覆盖。只有「同通道 + 同 BUILD_TYPE」时才比较新旧：dev 按 Build Time，release 按版本号。**

| 场景 | 行为 | 原因 |
|------|------|------|
| dev ↔ release 通道不同 | 替换 | 用户换了镜像通道 |
| BUILD_TYPE 不同（full↔lite） | 替换 | 用户换了镜像变体 |
| 同为 dev + 同类型 + 底包 Build Time > data Build Time | 替换 | dev 滚动构建按构建时间选最新 |
| 同为 dev + 同类型 + data Build Time >= 底包 Build Time | 不替换 | data 可能通过 API 在线升级过 |
| 同为 release + 同类型 + 底包版本 > data 版本 | 替换 | 正式版升级 |
| 同为 release + 同类型 + data 版本 >= 底包 | 不替换 | data 可能通过 API 在线升级过 |

### Docker 非 root 运行（PUID/PGID，songloft-org/songloft#380）

- **默认不设置 = 保持 root 运行**，与旧版本行为完全一致，零迁移风险。只有显式设置 `PUID` 或 `PGID`（任一即可，另一个默认补 `1000`）才启用降权，entrypoint 末尾用 Alpine 自带的 `su-exec`（比 `gosu` 轻，官方仓库自带无需额外下载）切到该 uid:gid 再 `exec` 主程序
- **`/app/data` 每次启动都递归 `chown`，`/app/music` 默认只 chown 顶层目录，不递归**：`/app/data` 体量小（db、封面、缓存等）且必须修复旧 root 运行遗留的属主，否则新用户打不开旧数据库；`/app/music` 可能是几十万文件/数 TB 的个人曲库，每次启动递归扫描的 IO 代价不可接受。顶层目录可写之后，新下载/新写入的文件本身就会以目标 uid:gid 创建，天然正确，不需要事后再修一遍
- **升级前遗留在 `/app/music` 内的历史 root 属主文件不会被自动修复**（如插入/覆盖标签写入过的旧文件），这是刻意的性能取舍而非遗漏。需要时设置 `FIX_MUSIC_PERMISSIONS=true` 显式触发一次递归修复，仅推荐在切换为非 root 运行后手动跑一次，不要做成默认行为
- **`integrations/home-assistant` 不受影响**：其 `run.sh` 直接覆盖了镜像的 `ENTRYPOINT`，完全绕开 `docker-entrypoint.sh`，权限模型由 HA supervisor 另行管理

---

## 前端 UI 验证（Docker 无头浏览器）

需要**真的把改动在界面上跑一遍**（而不是只跑 `flutter test`）时，走 Docker 里的无头 Chrome：
本仓库开发机上宿主的 `google-chrome --headless` 会 core dump，CDP 端口也起不来，别在那上面浪费时间。

```bash
# 1. 前端编成嵌入模式，由 Go 后端同源提供（省掉 standalone 的 API 地址配置步骤）
make build-frontend-web-embedded
go build -tags dev -o /tmp/songloft-full .
/tmp/songloft-full -port 58191 -db <tmpdir>/test.db -music <musicdir>

# 2. 起浏览器容器。--network host 才能让容器里的 Chrome 访问宿主 127.0.0.1:58191
docker run -d --name uichrome --network host browserless/chrome:latest

# 3. 用 /function 端点跑 puppeteer 脚本（Content-Type 必须是 application/javascript）
curl -s -X POST http://127.0.0.1:3000/function \
  -H 'Content-Type: application/javascript' --data-binary @script.js
```

脚本形如 `module.exports = async ({ page }) => { ...; return { data: {...}, type: 'application/json' } }`，
截图用 `page.screenshot({ encoding: 'base64' })` 塞进 `data` 里带回来再本地 base64 解码成 png。

### 驱动 Flutter Web 的踩坑

- **Flutter Web 是 canvas 渲染，DOM 里没有按钮**。可交互元素只以 `<flt-semantics>` 无障碍节点暴露：
  用 `getBoundingClientRect()` 拿到中心点后 `page.mouse.click(x, y)`。语义树默认已开启；
  若首屏出现 `<flt-semantics-placeholder>` 需先 `.click()` 它
- **语义 label 会被合并**：一屏内容常聚合成一个长 aria-label 节点，按钮文字未必是独立节点。
  找不到时**先截图看坐标再点**，不要在 label 匹配上死磕
- **文本框**：`<input aria-label="Username">` 是真实 DOM，但用坐标点击 + `keyboard.type` 更稳；
  点击与输入之间要留 ~800ms，否则 Flutter 焦点还没建立、输入会丢
- **验证双语**：用 `evaluateOnNewDocument` 覆写 `navigator.language`/`languages` 为 `zh-CN`，
  Flutter l10n 会跟着切
- **深链**：`page.goto('/settings/category/2')` 是整页刷新，会在 boot 到一半时产生假的
  "Failed to load"。用应用内点击导航，别用 goto 跳嵌套路由

### 断言要落在后端可观测状态上

截图只能证明「渲染对了」。交互是否真的生效必须另找证据，例如点完开关 `curl` 对应
`/settings/<name>` 端点看值有没有变、点完「停止计算」用 `pgrep -x ffmpeg` 看子进程有没有归零。

- **数进程别用 `ps -ef | grep <关键字> | wc -l`**：当前 shell 自己的命令行里就含那个关键字，
  会稳定多算 1~2 个，很容易得出「没停掉」的错误结论。用 `pgrep -x <可执行名>`

---

## 平台适配踩坑

- 升级检查 (`/api/v1/upgrade/check`) 仅 Docker 可用
- Flutter `secure_storage` 在 macOS 未签名沙盒下自动降级到 SharedPreferences
- Android 构建前需 `sdkmanager --licenses`；Android 13+ 需运行时申请通知权限
- 所有原生平台（Win/Linux/macOS/Android/iOS）音频后端统一走 media_kit/libmpv（经 `just_audio_media_kit` / 自定义 `SongloftJustAudioPlatform`），无回退原生、无 kill-switch
- HyperOS3 等需 `androidStopForegroundOnPause: false` 防后台回收
- **Bundle 模式 Android**：CWD 是 `/`，covers 目录路径必须相对于 `DBPath` 而非 CWD 解析（`da65db1` 修复）
- **Bundle 模式原生桥接**：Android 用 `Class.forName("mobile.Mobile")` 反射调用 gomobile 生成类，未打包 `.aar` 时 `isAvailable()` 返回 false（优雅降级）；iOS 同理用 Swift 调用 `MobileStart` 等 Objective-C 函数
- **Bundle 桌面子进程**：`DesktopBackendService` 在 Flutter 可执行文件**同目录**（macOS 在 `Contents/Resources/`）查找 `songloft-server`，通过 stdout 解析实际监听端口

---

## JS 插件

- 源码 `plugins/src/<name>/`，构建产物在各插件仓库的 GitHub Releases
- 新建插件：`npx create-songloft-plugin@latest`（交互式脚手架，支持 WebView / Lynx 两种渲染引擎模板，详见 `plugins/toolchain/README.md`）
- 沙盒：QuickJS。`fetch` 是运行时内置的全局函数（走 `__go_fetch_async`），`console` 系列（`songloft.log` 等）是日志包装——两者均非桥接调用。桥接由 `internal/jsplugin/api_bridge.go` 的 `HandleBridgeCall` 分发，namespace 包括：`storage`、`persistent-storage`、`songs`、`playlists`、`tags`、`comm`、`plugin`、`command`、`jsenv`、`fs`、`net`、`websocket`
- 路由：`/api/v1/jsplugin/{entry_path}/...`
- 公共资源：`/api/v1/jsplugin-assets/*` 提供嵌入在 Go 二进制中的 `theme.css`/`components.css`/`common.js`/字体，`injectHTMLHead` 自动注入到所有插件 HTML 页面（注入顺序：theme.css → components.css → common.js）
- 主题同步：`common.js` 内含 embed 检测 + 主题桥接（URL `?theme=` 参数 + `postMessage` 实时更新 + `data-theme` 属性 + `songloft-theme-change` 事件），暴露 `window.SongloftPlugin` 全局 API（`getTheme`/`onThemeChange`/`apiGet`/`apiPost`/`getCookies` 等）
- **客户端宿主桥接（WebView）**（`@songloft/client-sdk`，`common.js` 内含）：插件前端页面通过 `window.SongloftPlugin.host` / `player` / `getCookies` / `invokeHost` 等调用 Flutter 客户端宿主能力。native 平台走 `flutter_inappwebview.callHandler('songloftHost', {ns, method, params})`，Web/iframe 走 `postMessage` 到父窗口。分发逻辑在 `clients/player/lib/features/home/presentation/plugin_host_dispatch.dart`（传输无关、web-safe），native 桥接在 `plugin_host_bridge.dart`（mixin，注册 callHandler + 注入平台相关回调）。已注册的 namespace：`host`（getInfo）、`player`（播放控制）、`cookies`（Cookie 读取）、`favorite`（收藏状态同步，`refresh` 方法，传 `{songId, isFavorited}` 增量更新 Flutter 侧 FavoriteNotifier 缓存，不传参则全量重载）。**`window.SongloftPlugin` 的公开成员以 `common.js` 末尾那个对象字面量为唯一真实来源**——`invokeHost` 一度只存在于 `window.__SongloftInternal`（标注"插件请勿依赖"）而公开对象里没有，miot 却在自己的 `frontend/env.d.ts` 里手写了 `invokeHost?` 声明，于是 `window.SongloftPlugin?.invokeHost?.(...)` 通过 TS 编译、运行时被可选调用**静默吞掉**，收藏同步整个功能一个字节都没发出去（songloft-org/songloft-plugin-miot#86 第二次复发的根因）。**插件不要手写宿主 API 的类型声明**，用 `@songloft/client-sdk` 的 `SongloftPluginGlobal`；非要手写就先去 `common.js` 核对那个字面量
- **Cookie 读取桥**（`window.SongloftPlugin.getCookies(origin)`）：读取宿主 WebView Cookie Store 中指定 origin 的 Cookie（含 HttpOnly），返回 `{name: value}` 映射。**仅原生客户端可用**（Android/iOS/macOS/Windows；Linux 桌面无 WebView 实现），Web 端因浏览器同源策略无法实现，调用会 reject。实现路径：`common.js getCookies()` → `invokeHost('cookies', 'get', {origin})` → Flutter `PluginHostDispatcher` → `cookieProvider` 回调 → `CookieManager.instance().getCookies(url: WebUri(origin))`。origin 必须含协议+主机（如 `https://example.com`），无效格式会被校验拒绝。典型用途：FN Connect 等第三方网关的会话复用（用户在应用内 WebView 登录后，插件读取 Cookie 用于后续 API 调用）
- **Lynx 原生渲染桥接**（`@songloft/lynx-plugin-sdk`，`renderEngine: "lynx"` 专用）：声明 `renderEngine: "lynx"` 的插件使用 ReactLynx 编写 UI，编译为 `.lynx.bundle`，宿主通过 Lynx `<frame>` 元素原生加载。通信走 `NativeModules.SongloftPluginBridge`（三端原生模块：Android `SongloftPluginBridgeModule.kt` / iOS `SongloftPluginBridgeModule.swift` / HarmonyOS `SongloftPluginBridgeModule.ets`），按 `frameId` 路由父子 frame 间 RPC + 事件推送。SDK 提供 `invokeHost(ns, method, params)` / `onPlayerState` / `onThemeChange` / `onPush` 等 API。**不使用** `window.SongloftPlugin` / `@songloft/client-sdk`（那些是 WebView 专用）。构建时自动生成 `index.html` + `.web.bundle` 供 Flutter 客户端 WebView 回退
- `theme.css` 定义 `--md-*` CSS 变量（亮/暗双主题）、字体声明与 reset 样式；`components.css` 提供组件库样式。所有使用这些变量的插件自动跟随主题切换
- 权限：manifest 中 `permissions: ["net", "storage", "fs:music", ...]`，运行时由 `internal/jsplugin` 校验
- **fetch 内部控制头**：`X-Fetch-No-Redirect`（不跟随重定向，收集中间跳 `Set-Cookie` 必需）、`X-Fetch-Timeout-Ms`（单次超时 100–30000ms）、`X-Fetch-Insecure`（跳过 TLS 校验，**需 `net:insecure-tls` 权限**）。三者一律不转发给上游——`X-Fetch-Insecure` 曾因未实现而被当普通头透给上游（songloft-org/songloft#401）
- **新增权限常量要同步三处，否则插件根本用不上**：① 宿主 `internal/jsplugin/permissions.go` 的 `AllPermissions`；② `plugins/toolchain/packages/plugin-builder/src/manifest.ts` 的 `VALID_PERMISSIONS`——**它在 build 时硬失败**（`unknown permission: xxx`），漏了这处插件连编译产物都出不来；③ `plugins/toolchain/packages/create-songloft-plugin/src/index.ts` 的 `AVAILABLE_PERMISSIONS`（脚手架勾选列表）。另外 `ValidatePermissions`（宿主侧）**刻意对未知权限只 warn 不拒绝**：它挂在安装/更新的必经路径上，拒绝会让声明了新权限的插件在**旧宿主**上直接装不上，砸掉的还是插件原本正常的其他功能；宽容是安全中立的（`CheckPermission` 只认它认识的字符串）。**但旧宿主的严格校验已经发布出去了**，所以新权限落地后仍需等宿主版本铺开才能发插件——`minHostVersion` **纯属信息字段、宿主从不强制**，别指望它拦住谁
- **`net:insecure-tls` 权限（跳过 TLS 证书校验）**：`jsruntime` 层**没有权限概念**，`fetch` 本身也不需要 `net` 权限；该权限靠 `service.go` 在建环境后调 `jsManager.SetAllowInsecureTLS(envID, ...)` 投影成 `JSEnv.allowInsecureTLS`（默认 false），是权限在运行时层的唯一体现。几个要点：① **`CheckPermission` 无冒号前缀匹配，`net` 不覆盖 `net:insecure-tls`**——刻意如此，否则所有已声明 `net` 的存量插件（miot/bili/dav/pcyear-bridge）升级后会静默获得跳过校验的能力，而用户在权限列表里看不到任何变化；② 独立的 `insecureTransport`，**不能**复用 `sharedTransport` 再改单次请求的 `TLSClientConfig`——连接池按 Transport 隔离，那样会污染所有插件的连接复用；③ 子环境（`songloft.jsenv` 创建的）必须继承父插件的策略，否则「插件在子 worker 里发请求」会莫名校验失败；④ 无权限却带了该头时打 `slog.Warn` 而非静默丢弃——插件作者普遍误以为「宿主不识别就零副作用」，一条日志能省掉一整轮排查。**存在理由**：自建 NAS（飞牛 fnOS 5667）默认自签，且插件按**裸 IP** 访问，即便装了 CA 正式证书主体也是域名，按 IP 连必然 hostname mismatch——这类目标结构性无解
- 健康检查 + 文件指纹热更新均自动进行
- **UDP Socket API**（`songloft.net`，需 `net` 权限）：Go 侧托管 UDP socket + 消息推送模式。`udpBind` 创建 socket 并启动 reader goroutine，收到的 UDP 包通过 scheduler 队列异步推送到 JS 回调（`onData`）。支持多播组（`udpJoinMulticast/udpLeaveMulticast`），典型用途：SSDP 设备发现（DLNA/UPnP）。每插件最多 8 个 socket，有活跃 socket 的插件不会被空闲驱逐，插件卸载时自动清理。实现在 `internal/jsplugin/api_bridge_net.go`
- **TCP Socket API**（`songloft.net.tcpConnect`，需 `net` 权限）：出站 TCP 连接，`tcpConnect(host, port, options?)` 返回带 `send()/onData()/onClose()/close()` 的 socket 句柄。数据接收复用 UDP 的 Go readLoop + host event 队列推送模式（`postHostEvent("tcp_data")` → JS `__dispatchHostEvent`）。**data 为 base64 编码的原始字节**（send 侧 `btoa`、onData 侧 `atob`，同 UDP）：TCP 是字节流，一次读取可能在多字节 UTF-8 字符中间截断，raw string 会被 `json.Marshal` 替换为 U+FFFD 而永久损坏，故必须 base64；插件需累积字节跨 chunk 拼接后再 UTF-8 解码。**仅允许连接私有 / 回环 / 链路本地地址**（`isPrivateHostAllowed`，防 SSRF）；与 UDP 各自独立计每插件 8 个 socket 配额；有活跃 TCP 连接的插件不被空闲驱逐；卸载时自动清理。典型用途：控制本机 MPD（6600 端口 idle 事件推送）。实现在 `internal/jsplugin/api_bridge_tcp.go`
- **私有源认证**：`RegistryConfig` 支持 `token` 字段，拉取该源下所有资源时自动携带 `Authorization: Bearer <token>` 头，兼容 GitHub 私有仓库 PAT 和自托管私有源。详见 [插件源制作指南 · 私有源认证](docs/plugin_registry.md#私有源认证)
- **歌词/封面提供者**（`songloft.lyrics` / `songloft.covers`，无需权限）：插件通过 `registerProvider()` 注册为歌词或封面提供者。歌曲无歌词/封面时，宿主自动遍历已注册的提供者，调用 `/lyric-search` 或 `/cover-search` 端点（15s 超时，first-match-wins）。搜索参数包含 `title/artist/album`，歌词额外带 `duration`，两者均可选带 `fingerprint`（Chromaprint）和 `isrc`。搜到的歌词存入 DB（`scraped`），本地歌曲还嵌入文件标签；封面对本地歌曲下载到 `cover_path` 并嵌入标签，远程歌曲存 `cover_url`。提供者注册在空闲驱逐时不丢失，仅禁用插件时清理。实现在 `manager.go`（`SearchLyrics/SearchCover`）、`api_bridge.go`（JS API）、`handlers/music.go`（fallback 调用）。详见 [插件开发指南 · 歌词/封面提供者](docs/js-plugin-development-guide.md#songloftlyrics--歌词提供者)

---

## 业务踩坑总结（重要 — 不在代码里）

### 插件商店的 entry_path 撞名（identity）

`entry_path` 不是「插件 ID」那么单纯 —— 它同时是 registry 去重键、安装态匹配键、
`js_plugins.entry_path` UNIQUE 约束、ZIP 文件名（`<entryPath>.jsplugin.zip`）、static 目录名
（`jsplugins_data/<entryPath>/static`）、路由前缀（`/api/v1/jsplugin/{entryPath}/*`）、
manager/scheduler 的内存 map 键，以及 `plugin_storage.plugin_entry_path` /
`songs.plugin_entry_path` 的归属键。两个不同作者的插件完全可能撞上同一个 entry_path
（songloft-org/songloft#339）。改这块前先读完下面几条。

- **本地无法共存同 entry_path 的两个插件**，这是数据层事实而非 UI 限制。#339 只修了商店层
  （让两条都显示、安装态各自准确、覆盖前二次确认）。真要共存得引入真正的 plugin id、把
  entry_path 降级为可消歧的路由前缀，要动 DB 约束、磁盘布局、路由、`EntryPathFromZipName`、
  `SyncPluginsFromDirectory` 的孤儿清理，还要迁移 `plugin_storage` / `songs`
- **身份（identity）= 规范化 author，author 为空时用 `update_url` 的 GitHub `owner/repo` 兜底**
  （`internal/jsplugin/identity.go`）。author 规范化要剥掉 `<邮箱>` 与 `(备注)` 再转小写
  ——同一插件在不同源里写成 `hanxi` / `Hanxi <a@b.com>` 很常见，不归一会把它分裂成两条商店条目。
  **非 GitHub 的自托管 URL 刻意不推断仓库**：那种路径布局任意，`/plugins/a/` 与 `/plugins/b/`
  会被当成两个仓库，把同一插件的两个镜像地址分裂开
- **`SameIdentity` 任一方为空时返回 true**（视为同一插件）。宁可漏报冲突，也不能因为对方缺
  author 就误报冲突拦住用户的正常升级
- **跨源去重键刻意不含订阅源 URL**（`FetchAndMergeMulti`）：官方源与社区聚合源经常同时收录
  同一个插件，把源并入键会让它在「全部」模式里重复显示多遍。identity 已足够区分真正不同的插件
- **手动上传路径刻意不做冲突检测**：`InstallFromUpload` 保持原行为，只有商店安装走
  `InstallFromUploadWithOptions(RejectIdentityConflict: true)`。手动上传是用户明确指定文件、
  意图清楚，且插件迭代中 author 写法变动不应导致上传失败
- **冲突时必须在写任何东西之前返回**：以前 `package.go` 撞名就静默走 `Update`——覆盖 ZIP、
  `os.RemoveAll(staticDir)` 后重新解压、原地改写 DB 记录（保留原 ID 与 status）。结果是原插件被
  无声销毁，新插件继承了它在 `plugin_storage` 里的全部数据，原插件导入的歌曲
  （`songs.plugin_entry_path`）也被记账到新插件名下
- **商店的 `has_update` 必须用 `CompareVersion(...) > 0`，不能用字符串 `!=`**：撞名时两边版本号
  本就不同，字符串比较会永久显示「可更新」，用户点下去就把本地插件换成了另一个作者的插件
- **前端就地更新安装态要按 `(entryPath, identity)` 匹配**（`RegistryPluginEntry.matches`）：
  只比 entryPath 会把同名的其他条目一起点亮成「已安装」。列表项的 `key` 用 `rowKey`
  （`entryPath|identity`）而非 index——`_RegistryPluginItem` 持有 `_installing` 本地状态，
  按 index 复用 Element 会让 loading 圈跑到别的条目上

### 插件商店的结果缓存

商店的分页与搜索都在**完整插件列表**上做（服务端切片 + 子串过滤），所以每次翻页、每次改搜索词
都会触发一次完整的注册表拉取——最多 500 个 `plugin.json`、8 并发、单请求 15s 超时。
`registry_cache.go` 给拉取结果加了 5 分钟的进程内 TTL 缓存。

- **`RegistryService` 必须由 handler 长生命周期持有**（`JSPluginHandler.registrySvc`）。
  以前是每个 HTTP 请求 `NewRegistryService()`，那样缓存永远命不中
- **`proxyDown` 因此必须是每次调用的局部变量**，不能挂在 `RegistryService` 上。它记忆
  「GitHub 代理本次已失效」，作为单例字段会让代理一次失败后**永久直连**（代理恢复也不再尝试），
  且并发请求互相干扰。私有方法签名都带 `proxyDown *atomic.Bool` 就是为此
- **缓存键必须含所有影响结果的输入**：模式（单源/全部）、源 URL、token、源顺序、`github_proxy`。
  源顺序不能省——`FetchAndMergeMulti` 按源顺序决定同版本插件由哪个源胜出。**token 进键前要
  哈希**，缓存键会进日志与调试输出，不能带明文凭据
- **刻意不缓存安装状态**：`installed` / `has_update` / `conflict` 每次请求都由
  `buildInstalledMap` 从 DB 实时算，所以装完插件立刻翻页也能看到状态更新，无需失效缓存
- **失败不写缓存**（也不动既有缓存）：否则一次网络抖动会把错误状态粘住整个 TTL。
  `FetchAndMergeMulti` 不返回 error（单源失败只进 warnings），所以那条路径改为**空结果不缓存**
- **前端只有「刷新」与「拉取失败重试」传 `force: true`**；翻页、搜索、首次加载、源切换都不传。
  源切换与改源配置靠缓存键变化自然重拉，`UpdateRegistriesSetting` 另外显式 `InvalidateCache()`
  腾出条目配额

### scan 标题规则

- tag 有 title → 直接用 `tag.Title`
- tag 没 title → 文件名去扩展名
- **不要**再做"最长公共子串去重 + 拼接"，会产生"艺术家 - 标题"这种把艺术家冗余到标题字段的结果
- 视频容器探测：扫描 mp4/mov/m4v/mkv/webm/avi/ts/mpg/mpeg/flv/wmv/rm/rmvb/3gp 等容器时用 ffprobe 探测是否含真实视频轨（排除封面 attached_pic）来置 `songs.is_video`，客户端据此渲染画面 / 选择投屏 mime
- **本地起服务验证时，music 目录不要放在 `/tmp` 下**：`music_path` 的默认 `exclude_dirs` 含 `tmp`，
  而 `ShouldExcludeDir` 是**按路径任一层级的目录名**匹配的，于是整个 `/tmp/...` 根目录被排除。
  表现是扫描「成功完成」但 `discovered_files=0`，**不报错、不打 warn**，极易误判为自己的改动坏了

### scan 流式入库（大曲库铁律 — songloft-org/songloft#447）

`doScanAndImport` 的入库是**流式**的：`scanBatchAccumulator`（`scan_batch.go`）按目录分组，
目录定稿即 `flushScanBatch`。别改回「全量累积后一次性写」——几万首的库会在整个提取阶段
（数十分钟）里进度恒为 0、数据库一条不写，用户只能判定为卡死。

- **定稿单位必须是目录，不能是固定条数**：`fixSpamTags` 按目录分组、以「同目录内超过一半文件
  共享同一 `(title, artist)`」判定垃圾 tag，需要整个目录作样本。按 50 条切批会把一个目录劈成
  几段，判定结果随切分位置漂移
- **`scanDirGroupCap` 是内存兜底**：平铺几千文件的目录到达上限就提前定稿，否则退化成全库累积。
  代价是该目录的垃圾 tag 判定改用子样本，可接受（判据是 >50% 且 ≥3 条）
- **进度计数只在批次落库后累加**（`flushScanBatch` 里的 `UpdateProgress`）。提取阶段单独调
  `SetCurrentFile` 上报当前文件，别在提取阶段也走 `UpdateProgress`，会双重计数
- **写库的 ctx 与取消的 ctx 必须分开**：`doScanAndImport` 会把入参 ctx 派生成可取消 ctx（取消时
  杀 ffmpeg 子进程），但 `flushScanBatch` 必须用派生前的 `flushCtx`。取消时要把已提取的成果落库，
  用被取消的 ctx 会让事务立刻失败、重试三次后全部记为 failed，「取消保留成果」静默不生效
- **取消不再丢弃已提取结果**（v2.12.0 之后的行为变更）：worker 退出前的结果照样入库，
  `sealAll` 负责冲出 pending 归不了零的残余目录组。以前整批丢弃，用户连续取消两次就白干二十分钟

### 目录遍历不要逐条目 `os.Stat`（songloft-org/songloft#447）

`scanDirWithCue` / `collectDirNames` 用 `resolveEntry`（`scanner.go`）判断目录项类型：
`os.ReadDir` 返回的 `DirEntry.Type()` 在 Linux 上直接来自 `d_type`，零额外系统调用；
只有软链接与未知类型才回退 `os.Stat`（跟随软链接，保持「软链接目录仍递归、断链跳过」的语义）。

- 逐条目 `os.Stat` 会让万首级曲库多出几万次系统调用。`GET /scan/dir-names` 就是因此超过客户端
  10s 超时 → `r.Context()` 被取消 → 遍历半途中止 → 500，且**每次重试都从零重来**
- 该端点因此还有两层保护：`CollectAllDirNames` 的 60s TTL 缓存（`dirNamesMu` 兼作单飞锁，
  挡住前端重试风暴），以及 handler 侧的 `context.WithoutCancel(r.Context())`——脱开客户端取消，
  让首次遍历跑完并填上缓存，重试立刻命中。缓存挂在 `Scanner` 上，`music_path` 变更时
  `app.go` 重建 Scanner，缓存自然失效
- d_type 不可用的文件系统上 `ReadDir` 内部会自行 lstat 补齐类型，等价于原行为，不会变慢

### DSF（DSD）扫描

- `dsf` 是默认扫描格式；`app.go` 两处回退列表和迁移 `0039_scan_support_dsf.sql` 保持同步，迁移为已有配置幂等追加（大小写无关）。
- `pkg/tag` 可读取 DSF 元数据与封面；DSD raw 采样率可能是 ffprobe PCM-equivalent 的 8 倍，已有元数据测试处理该差异。
- 播放沿用 libmpv / ffmpeg；浏览器与不支持 DSD 的 DLNA 设备需显式传 `?format=mp3` 等转码参数，当前 Flutter Web 的 `AudioFormatHelper` 不会为 DSF 自动追加。DSF 不在标签写入矩阵中，写入失败应沿用现有降级处理。

### 旁挂歌词（.lrc）

- **匹配规则**（`FindSidecarLyricFile`）：`<base>.lrc` / `.LRC` / `.Lrc`，然后 `<含扩展名>.lrc` / `.LRC` / `.Lrc`。
  前者优先。**禁止**改成 `ReadDir` + `EqualFold` 遍历（O(歌曲数×目录项数) 不可接受）。
- **空文件视作未命中**：`st.Size()==0` 或目录同名均跳过。理由：防止空 lrc 把 `lyric_source` 打成 `file` +
  `lyric=""` 而前端永不请求、插件也无法兜底的死角。
- **编码处理**（`ReadSidecarLyric`）：UTF-16 LE/BE 按 BOM 识别 → `x/text/encoding/unicode` 解码；
  其余走 `tag.FixEncoding`（GBK 系修正）。
- **扫描跳过的三级短路**（`needsSidecarLyricImport`）：
  ① `LyricSource ∈ {file, manual}` → false（无 IO）；
  ② 目录不在 `ScanResult.LyricDirs` → false（内存 map）；
  ③ 该歌曲对应的 lrc 确实存在 → true（最多 6 次 Stat）。
  收敛性：命中后 `lyric_source=file` → 下一轮走 ① 短路。
- **运行时优先级链**（`GetSongLyric` handler）：
  sidecar .lrc > DB url > DB payload > 歌词搜索插件。`manual` 不被旁挂覆盖（`SidecarLyricForSong` 排除）。
- **`SyncSidecarLyric` 不回写音频标签**：.lrc 本身就是持久化载体，嵌入标签后用户删 lrc 会留下删不掉的过期副本；
  且 `WriteSongTags` 是重建模式会读写封面二进制，性能代价大。
- **`shouldApplyScanLyric` 护栏**：`manual` 不覆盖；新歌词为空时不抹掉库中已有歌词/远程URL。
  这是行为变更（以往重新导入可清空歌词），已写入 CHANGELOG。
- **旁挂命中会整体替换**插件带来的翻译/罗马音（`tlyric`/`rlyric`/`lxlyric`），不做"主歌词用文件+翻译沿用插件"的合并——
  两份歌词时间轴不同源，错位比没有翻译更糟。

### tag 写入（pkg/tag）

- `tag.WriteTag(filePath, opts)` 按扩展名 dispatch，所有格式均使用临时文件 + `os.Rename` 原子写入
- 支持矩阵：

| 格式 | 文本字段 | 歌词 | 封面 |
|------|---------|------|------|
| MP3 | ID3v2.3 text frames | USLT | APIC |
| FLAC | Vorbis Comment | LYRICS | PICTURE block |
| M4A/MP4/M4B/M4V/MOV/3GP | iTunes atoms (©nam 等) | ©lyr | covr |
| OGG(.ogg/.oga/.opus) | Vorbis Comment | LYRICS | METADATA_BLOCK_PICTURE (base64) |
| APE | APEv2 text items | Lyrics | Cover Art (Front) (binary item) |
| WAV | RIFF LIST INFO | ICMT | **不支持**（格式限制） |
| AIFF/AIF | ID3v2.3 (ID3 chunk) + NAME/AUTH | USLT (ID3 chunk) | APIC (ID3 chunk) |
- 不支持的格式 → 返回 `ErrUnsupportedWrite`，调用方**必须**降级为日志，**不要**阻塞主流程

### 音频指纹（fingerprint — 开销控制铁律）

指纹（ffmpeg chromaprint）只服务两处：设置页「重复歌曲检测」和插件歌词/封面搜索的**可选**参数。
它是按需功能，**不是**扫描的必要环节。改这块前先读完下面几条，`songloft-org/songloft#323`
就是这些约束缺位叠加出的「扫描显示完成但 CPU 永久 100%」。

- **扫描后自动指纹默认关闭**：业务端点 `GET/PUT /api/v1/settings/scan-auto-fingerprint` 体 `{enabled: bool}`，
  config key `scan_auto_fingerprint`，默认 `false`。`runAutoFingerprint`（`song_service.go`）开头判定，
  与 `scan_auto_create_playlists` 对称。关闭时用户在重复检测页手动 `POST /scan/fingerprints`
- **失败必须落库标记**：`songs.fingerprint_attempted_at`（unix 秒，0 = 未尝试）。
  `ListLocalWithoutFingerprint` 的条件是 `fingerprint = '' AND fingerprint_attempted_at = 0`。
  **绝不能**只在失败时打日志——没有标记，AutoScanner 每轮（默认 3600s）都会把同一批注定失败的
  长音频/无音轨文件重新捞出来跑 ffmpeg 全解码。`ClearAllFingerprints` 会重置该标记，
  所以「重新计算全部」是重试失败项的唯一入口
- **采样上限 120 秒**：`ExtractFingerprint` 带 `-t 120`（常量 `fingerprintSampleSeconds`），
  这也是 AcoustID 的事实标准。**不要**去掉——30 分钟有声书全解码在弱 NAS 上必然超时，
  实测同一文件全长 3.8s vs 120s 采样 0.35s。超时 30s + `cmd.WaitDelay` 防 ffmpeg 子进程挂住 worker
- **CUE 轨按区间采样**：CUE 轨的 `file_path` 指向整轨镜像，必须传 `cue_start_seconds/cue_end_seconds`
  走 `-ss`，否则同一镜像下所有 track 拿到**完全相同**的指纹并互判重复
- **并发按 CPU 自适应**：`fpWorkerCount()` = `clamp(GOMAXPROCS/4, 1, 4)`。**不要**改回硬编码 4，
  Go 的 GOMAXPROCS 感知 cgroup 限额，Docker 限核能正确收敛
- **可取消**：`POST /api/v1/scan/fingerprints/cancel` → `FingerprintService.Cancel()`。
  指纹任务用独立的 `context.Background()`，**不能**复用扫描的 cancelCh——`scanProgressManager.Complete()`
  已经 `close(cancel); cancel = nil`，之后 `GetCancelChannel()` 返回 nil。取消中途的歌曲不打
  attempted 标记，下次续算
- **去重带时长护栏**：`ListDuplicateGroups` 在同指纹内还按 `fingerprint_duration`（全片时长）
  以 30 秒容差**相邻**聚簇（时长为 0 = 未知则不切）。因为只采样前 120 秒，「统一片头的有声书」存在指纹碰撞可能；
  护栏刻意保守——漏报真重复比误报更不可接受，用户会照列表删文件
- 迁移 `0029` 已把旧的全长指纹一次性清空（与 120 秒采样不可比），升级后需重算一次
- `IsChromaprintAvailable` 走 `SetFingerprintFFmpegPath` 注入的 `ffmpeg_path` 配置，不再只查 PATH

### 播放历史（play_history — 按播放上下文）

每个「播放上下文」独立记住最近播放过的歌曲（songloft-org/songloft#333）。上下文 = `(context_type, context_key)`：
歌单是 `("playlist", "<id>")`，分面维度是 `("artist", "周杰伦")` 这类，取值复用 `songFacetColumn` 的
7 个维度，**不另立枚举**。改这块前先读完下面几条。

- **序数不可靠，所以读端点不返回「第几首」**：分面列表走 `GET /songs?artist=X`，默认排序 `added_at DESC`，
  而 `added_at` 是秒级 `DATETIME`、批量扫描在单事务里顺序 INSERT → 成百上千首歌 `added_at` **完全相同**；
  `applyOrder`（`filters.go`）只产出单列 `ORDER BY`、**无 `id` tie-break**。据此算「第 N 首」在数学上不确定，
  会起播到错的歌。**不要**为此去改 `applyOrder` 加 tie-break——那会动既有分页行为，属独立 issue
- **`absIndex` 与分页 offset 的一致性依赖 SQLite planner**：`/songs/ids`（一次全排）与
  `/songs?limit&offset`（可能走 bounded sorter）是两个查询，并列 `added_at` 下的相对顺序全靠
  planner 恰好一致（实测 350 / 30000 首均一致）。真要收紧就得给 `applyOrder` 默认排序补 `, id DESC`
  ——那会动所有 `/songs` 分页调用方，属独立 issue。同理 `ListSongIDsOrdered` 与 `GetPlaylistSongsPaginated`
  都只按 `position ASC` 无次级键，仅在同歌单出现重复 position（并发 reorder 中断）时才会错位
- **客户端起播 = 首曲直起 + 后台环形补齐**：历史条目自带完整 `Song`，先用它当队列**零请求**出声，
  再后台拉有序 ID 列表（歌单 `GET /playlists/{id}/song-ids`、分面 `GET /songs/ids`）`indexOf` 定位，
  依次补「目标之后」与回卷的「开头…目标之前」。`currentIndex` 全程为 0 不动，
  不牵连随机模式的 `_playedIndices` / `_preSelectedNextIndex`；歌单与 7 个分面走同一条代码路径
- **只有 `type=play` 落库**：写入挂在既有打点端点 `POST /songs/{id}/played` 的 `context_type`/`context_key`
  参数上（不新开第二条打点通道，客户端零额外请求）。`finish` 是同一首歌的重复写；**`skip` 尤其危险**——
  它上报的是**上一首**歌，此时客户端上下文可能已切到新歌单，会把上一首错记到新上下文名下。
  前端对 skip/finish 不传 context，后端也只认 `play`，双重保险。落库失败只 `slog.Warn`，**响应码永远 204**
- **上限 50 条，upsert + 裁剪同事务**：去重靠 `UNIQUE(context_type, context_key, song_id)` + `ON CONFLICT DO UPDATE`，
  不在应用层查重。裁剪用 `id NOT IN (... ORDER BY played_at DESC, id DESC LIMIT ?)`——`played_at` 秒级会撞，
  `id DESC` 做确定性 tie-break。`MaxPlayHistoryPerContext` 是 Go 常量，**不要**做成配置项
- **`context_key` 是 TEXT，无法对 playlists 建外键**：删歌单靠 `PlaylistService.clearPlayHistory` 显式清理。
  **批量删除时只能清理真正被删掉的歌单**——仓储层会跳过 `built_in`，若无脑遍历入参 ids 就会把「收藏」
  的历史一并清空（实现时踩过，已由 `TestPlaylistDeleteClearsPlayHistory` 覆盖）
- **`sourcePlaylistId` 不能改签名**：它是 JS 插件公开契约（`plugin_host_dispatch.dart` 导出 `source_playlist_id`）
  且有 5 处「正在播放」高亮消费点。前端泛化成 `PlaybackContext` 时把它降级为**派生 getter**，
  故上述消费点零改动。prefs 读兼容旧 `player_source_playlist_id`、写时双 key 并存，保证 Android 热更回滚安全

### HLS 电台代理模式（/settings/hls-proxy）

- 业务开关端点：`GET/PUT /api/v1/settings/hls-proxy` 体 `{enabled: bool}`，默认 `false`
  - `false`：电台 `.m3u8` 直接 302 给 player，由 player 自己拉源站。零开销但受源站防盗链/CORS 限制
  - `true`：服务端拉取并改写 m3u8、代理所有切片/key/init 段。**所有切片走本机带宽**，注意流量成本
- 切换时机：源站 Referer/UA 防盗链导致播放失败 / Web 嵌入模式 CORS 阻塞时，开启代理
- 反代端点：`/api/v1/songs/{id}/hls/playlist?u=<base64url>` 和 `/api/v1/songs/{id}/hls/segment?u=<base64url>`
- HLS 电台 song.url 强制带 `.m3u8` 后缀（`/api/v1/songs/{id}/play.m3u8`）：ExoPlayer/AVPlayer 按 URL 后缀选 MediaSource，无后缀会落到 ProgressiveMediaSource 导致直播无法播
- 改写规则：经典 HLS + LL-HLS 全集（PART/PRELOAD-HINT/RENDITION-REPORT）+ `EXT-X-DATERANGE:X-ASSET-URI`（HLS Interstitials 单 URI）。`X-ASSET-LIST`（JSON 子代理）暂未实现，遇到时原样透传
- 安全：每次端点入口做"同源校验（scheme+host+port 与 song.URL 严格相等）"作第一道防线，`services.IsHostnameAllowed` 作 SSRF 兜底。**非同源 URL 保持原样不改写**，避免成为开放代理
- player 跨域：改写后的 URL 全部是相对路径（`playlist?u=...` / `segment?u=...`），规避 BASE_PATH 子路径部署问题
- 上游 4xx/5xx 透传给 player；playlist 体上限 1 MB；首行必须 `#EXTM3U`

### 通用 HTTP Proxy（/settings/http-proxy）

- 业务端点：`GET/PUT /api/v1/settings/http-proxy` 体 `{proxy: string}`，默认 `""`（直连）
- 设置后所有后端外发 HTTP 请求（插件注册表拉取、插件下载/更新、系统升级检查/下载）通过指定的 HTTP 代理转发
- 典型值：`http://192.168.1.1:7890`（支持 HTTP/HTTPS/SOCKS5 代理）
- loopback 地址（`localhost`/`127.0.0.1`/`::1`）自动跳过代理，避免影响内部请求
- 与 GitHub 镜像加速（`github_proxy` URL 前缀拼接）**共存**：先拼接镜像前缀再经 HTTP Proxy 转发
- 实现：`internal/httputil/proxy.go` 提供全局 `ProxyConfig` + 共享 `*http.Transport`，`httputil.NewClient(timeout)` 创建代理感知的 client
- 启动时从 config 表加载已保存的代理地址（`app.go`）；PUT 时即时生效无需重启
- 当前已接入的 service：`jsplugin/registry.go`、`jsplugin/package.go`、`services/upgrade_service.go`、`handlers/jsplugin_registry.go`（downloadZIP）
- ffmpeg 远程拉流转码路径（`services/radio_transcode.go` 电台转码、`services/url_transcode.go` 转码代理）同样接入：经 `-http_proxy` 传给 ffmpeg。**仅 http/https 代理**，SOCKS5 不支持（ffmpeg `-http_proxy` 限制）

### 私网代理白名单（/settings/proxy-private-allowlist）

- 背景：通用资源代理 `GET /api/v1/proxy?url=` 默认用 `services.IsHostnameAllowed` 封禁一切内网 / 回环 / 链路本地地址（防 SSRF），导致「公网 Songloft 代理仅私网可达的 WebDAV」被拒（songloft-org/songloft#313）
- 业务端点：`GET/PUT /api/v1/settings/proxy-private-allowlist` 体 `{allowlist: []string}`，默认 `[]`（空 = 维持全阻断，行为不变）
- 每条为单个 IP（`192.168.1.100`）或 CIDR 网段（`192.168.1.0/24`），PUT 时 `services.ParseAllowlist` 校验，非法条目返回 400
- 判定：`services.IsHostnameAllowedWithAllowlist(hostname, allowlist)`——外网恒放行，私网 IP 仅当命中白名单某条网段才放行；`localhost`/`.local`/空主机名仍字符串级封禁（白名单只按 IP/CIDR 匹配）
- **仅影响通用 `/proxy`**；HLS 反代（`hls.go`）仍走 `IsHostnameAllowed(nil)`，语义不变
- 实现：`internal/services/whitelist.go`（`ParseAllowlist` / `IsHostnameAllowedWithAllowlist`）+ `internal/handlers/proxy.go`（`ProxyHandler` 持有 `*ConfigService`，config key `proxy_private_allowlist`）

### 音频转码代理（/proxy/transcode）

- 业务端点：`GET /api/v1/proxy/transcode?url=&format=mp3[&bitrate=&duration=&user_agent=&referer=]`（`@Security BearerAuth`，需 token；token 作为首个查询参数，规避音箱固件把 `&` 替换为空格的坑）
- 服务端拉取远程音频 URL，经 ffmpeg 实时转码为 mp3（CBR 320k）流式返回。用途：miot「不入库直接播放」场景下，外部搜索源返回 webm/opus 等音箱无法解码的直链（songloft-org/songloft#394），客户端把本端点 URL 推给音箱即可播放
- 不落盘、不入库、不缓存（与 no-import 语义一致；youtube 直链每次重签，缓存零收益，重播收益走入库路径）
- SSRF 防护：复用 `/settings/proxy-private-allowlist`，与通用 `/proxy` 同一 `IsHostnameAllowedWithAllowlist` 校验
- 输出 mp3 CBR + `-write_xing 0` + `-map 0:a:0 -vn`：pipe 不可 seek，音箱靠字节估算时长，CBR 避免提前切歌（同 seek 流取舍）
- 并发与 seek/均衡流共享 `seekStreamSem`（cap=4），满则 503 不排队
- ffmpeg 直拉远程 URL（`-i <url>`），经 `-http_proxy` 复用全局 http-proxy（仅 http/https）
- 实现：`internal/services/url_transcode.go`（`StreamTranscodedURL`）+ `internal/handlers/proxy.go`（`Transcode`，`ProxyHandler` 持有 `*CacheService`）

### 音乐缓存（cache_service）

- 播放远程歌曲时流式代理上游音频到客户端（不阻塞），同时后台异步写入缓存；后续播放缓存命中后直接从本地返回
- 流式代理 `ServeRemoteResourceWithCache`：200 OK 时 TeeReader 同时代理+写临时文件，206 Partial 时正常代理并触发异步全量下载
- 缓存路径持久化在 `songs.cache_path` 字段（DB 级别），查找时优先 `cache_path`，fallback 到旧格式哈希分桶目录
- 缓存目录默认 `{data_dir}/music_cache/`，可通过 `PUT /api/v1/cache-manage/config` 的 `cache_dir` 字段自定义为绝对路径
- 启动时从 `music_cache_config` 配置读取自定义目录；运行时切换目录会自动重建 LRU 索引，不迁移旧文件
- LRU 淘汰：超出 `max_size`（默认 1GB）时按最后访问时间淘汰，`max_size=0` 表示不限制
- **缓存落盘转码**（`transcode_format` / `transcode_quality`，`PUT /api/v1/cache-manage/config`，songloft-org/songloft#300）：默认 `""` 按上游原码落盘（YouTube .mkv/.webm、B站 .mov）；设为 `mp3/m4a/ogg/flac/wav` 后，缓存网络歌曲落盘时统一转码为该格式（解决小爱音箱等设备无法播放 MKV）。由 `EnsureCachedFormat` 在两个播放侧产出点执行——`FinalizeCache`（流式播放 + 206 异步全量）与 `prepareSongPlayback` 的 prefetch 预热（`Get` 之后）；复用 `runFFmpeg`，转码后删原码、`cache_path` 指向新格式。缺 ffmpeg 或转码失败时**优雅降级保留原格式**。**不影响** `songs.download` 的显式格式处理（下载路径复用 `Get` 但自带 `opts.Format`，故转码逻辑只挂在播放侧，不动 `moveToCache`/`Get`）。**取舍**：`ffmpeg -vn` 会丢视频轨，故对 `is_video` 的远程歌开启后 `media=video` 投屏只得纯音频（属预期，满足 YouTube MKV→mp3 主诉求）；`EnsureCachedFormat` 内含 `cacheTranscodeTimeout`（15min）兜底，防坏源卡死永久占用 `transcodeSem`
- `POST /api/v1/cache-manage/validate-dir` 可预先验证目录（自动创建 + 可写性检查 + 返回磁盘空间）
- inflight 去重：同 `song.ID` 的并发请求只下载一次；首请求被 `ctx.Canceled` 时后续等待者自动重试

### 音量均衡（`/songs/{id}/play?normalize=1`）

EBU R128 响度均衡（songloft-org/songloft#315），消除不同音源之间的响度落差。目前只有 miot 插件用
（设置页「音量均衡」开关 → config key `volume_normalize` → `buildSongURL` 追加 `&normalize=1&format=mp3`）。

- 滤镜串是 `internal/services` 的包级常量 `loudnormFilter`（`loudnorm=I=-16:LRA=11:TP=-1.5`，**单遍**动态模式）。
  落盘转码与实时流共用它，**不要**在任一处内联写死——两条路径必须给出同一响度
- 产物缓存键带 `norm.` 标记（`transcodedFileName`），与不均衡的产物**互不通用**
- **转码产物未就绪时边转边发**（`tryLiveTranscodeStream` → `StreamSeekedMP3`，`internal/handlers/music.go`）：
  `GetOrTranscode` 是同步的，首字节 = 整首转码的墙钟时间。两处实测：整首 loudnorm 要 20+ 秒，
  会把设备的首个 play 请求整段挂住（songloft-org/songloft-plugin-miot#61 实测 `dur_ms=22392/24348/22381`），
  音箱那端是「前 20 多秒空白」，而插件的自动切歌定时器在推 URL 那一刻就起算，尾部又被砍掉同样长度；
  普通 `?format=` / `?quality=` 转码在弱 CPU（N5095）上无损转 mp3 也要等 5 秒以上才出声
  （songloft-org/songloft#442）。改为 pipe 直出后实测均衡场景首字节 10.03s → 0.088s。
  **`format` 与 `quality` 两条路都覆盖**（`SeekStreamOptions.Bitrate` 让 pipe 能如实表达 `?quality=`，
  不再固定 320k），仅当目标格式是 mp3、非 `media=video` / 抽轨 / CUE / HEAD 时生效，其余场景保持原阻塞路径
- **`ForceTranscode` 必须显式传，不能靠「源扩展名不是 mp3」推断**（`SeekStreamOptions`）：纯 `?format=mp3`
  既不 seek 也不均衡不变速，`StreamSeekedMP3` 的守卫会判它「没活干」而拒绝；而伪 mp3（扩展名 `.mp3`
  内容是 WebM/Opus，songloft-org/songloft#300）恰恰是扩展名为 mp3 却必须转码的一类，靠扩展名推断
  会把它放进 `-c:a copy` 快路径 → mp3 muxer 拒收 Opus → 零输出 → 降级，白起一个 ffmpeg 且延迟一点没改善
- **泛化后的守卫必须自洽**：`tryLiveTranscodeStream` 自己算 `normalize || bitrate > 0 ||
  NeedsTranscodeForServe(...)`，不依赖两个调用点（`serveLocal` / `serveCachedFile`）的 `if` 条件
  ——判错的代价是每次播放都白烧一个 libmp3lame 进程
- **pipe 流必须支持 Range，否则拖动会静默卡死**（`cbrRangePlan`，songloft-org/songloft#442）：
  chunked 无 `Content-Length` 的响应会被浏览器判定为不可 seek。Docker 无头 Chrome 实测拖动进度条后
  **一个新请求都不发**，`readyState` 从 4 掉到 1、buffered 零增长、`error` 为 null——**播放静默卡住，
  只能重新点播**。而进度条总时长取自 DB 的 `song.duration`，UI 上完全看不出这条流不能拖。
  只补 `Accept-Ranges: bytes` 没用（实测同样只发一个请求就卡死）：浏览器需要**总字节数**才能把
  「第 N 秒」映射成字节偏移。**能算得出来的原因**：pipe 一律 CBR（`-b:a`），字节与时间成线性，
  配合已知的 `song.duration` 即可给出总长度与任意偏移对应的秒数；而「从第 N 秒起流」正是
  `StreamSeekedMP3` 的 `-ss` input seek。于是 Range 被**真正**满足，而不是「忽略 Range 从头重发」
  （那会让进度条与实际声音错位）
- **启用 Range 的四个前提，缺一不可**（见 `planCBRRange`）：`song.Duration > 0`（时长未知就估不出
  总字节，远程歌曲元数据未刷新时是常态）、必须重编码而非 `-c:a copy`（copy 的源可能是 VBR mp3，
  字节与时间不成线性；由 `ForceTranscode` 保证）、不变速（atempo 改变输出时长，要再套一层换算）、
  无 `?seek=`（那是给推流客户端的另一套位置语义，叠加只会互相干扰）。不满足时退回 chunked 无长度
- **承诺了 `Content-Length` 就必须写够写准**（`exactLengthWriter`）：ffmpeg 的实际输出量不会精确等于
  「码率 × 时长」——LAME 的 CBR 靠帧 padding 贴合码率、末尾可能是不完整帧、`-ss` 只能对齐到帧边界，
  累计误差在几帧内。写少了客户端当作连接提前断开，写多了违反 HTTP/1.1 长度契约，所以超出截断、
  不足补零。**超出时必须返回 error 中断 io.Copy**（`errExactLengthReached`）而非静默丢弃：
  闭区间 Range（`bytes=N-M`）只要一小段而 ffmpeg 会转到源结尾，静默丢弃等于让一个 `bytes=0-100`
  请求白转整首
- **206 的 `WriteHeader` 必须推迟到首次真的写字节时**（`deferredStatusWriter`）：一旦 WriteHeader
  响应就提交了、再也无法降级，而 `StreamSeekedMP3` 的契约是「`Peek(1)` 确认 ffmpeg 真有输出后才写」
  ——「首次 Write」正是那个安全时刻。同理**降级时要 Del 掉 `Content-Length`/`Content-Range`/
  `Accept-Ranges`**，残留的长度会和 `http.ServeFile` 自己算的冲突
- **包装 writer 必须转发 `Flush()`**：services 侧是 `io.Copy(&flushingWriter{w: w}, ...)`，而
  `flushingWriter` 靠**类型断言** `w.(http.Flusher)` 决定要不要 flush。`exactLengthWriter` /
  `deferredStatusWriter` 少了这个方法就会把断言挡掉，字节攒在缓冲里不下发——**首字节延迟原地回归**
- **实时流刻意不顺手起后台转码补缓存**：同一首歌并跑两个 ffmpeg 会在弱 NAS 上把 CPU 翻倍，
  而用户正等着这一秒出声。缓存产物交给 `?prefetch=1`（带同样的 format/quality/normalize）生成
- **接入 #442 后没有提高 `seekStreamMaxConcurrent`（仍是 4）**：普通转码播放的频率比 seek/均衡高一个
  数量级，槽位更容易占满，但 #442 的报告环境正是弱 CPU——放大并发对它是伤害，而占满时降级回
  阻塞路径本身无损（只是回到修复前的行为）
- **预热必须带 normalize**：`prepareSongPlayback` 的 `normalize` 参数不能丢，且短路判断里要有 `!normalize`
  ——mp3 源 + `format=mp3` 时 `NeedsTranscodeForServe` 为 false，少了这一项预热会直接 return、一行没干
  （#61 的另一半根因，日志里连一条 `prefetch ready` 都不会出现）
- 插件侧配套：随机模式下 `PlaylistManager.reserveNextIndex()` 把「预热的那首」和「真会播的那首」
  锁到同一首。`getNextIndex()` 每次调用都重新摇骰子，预热与切歌各调一次就必然热错人（实测 492/500 不一致）

### 服务端 seek 流（`/songs/{id}/play?seek=<秒>`）

给「只会从头拉 URL、不支持 HTTP Range」的推流客户端表达续播位置用。小爱音箱经
`player_play_url` 拿到 URL 后只能从流的开头播，所以「从第 N 秒续播」只能由服务端产出一条
**以第 N 秒为开头**的流（songloft-org/songloft-plugin-miot#60：暂停被固件忽略 → 升级 stop →
设备端媒体上下文丢失 → 续播只能重推 URL）。实现在 `internal/services/seek_stream.go`，
骨架与 `radio_transcode.go` 同构（`Peek(1)` 确认有输出才提交响应，否则返回哨兵错误让
handler 无损降级为从头 `ServeFile`）。

- **一律输出 MP3**：源是 mp3 走 `-c:a copy`（input seek，实测整首 0.14s、近零 CPU），否则
  `libmp3lame`。**不要**扩成多格式矩阵——浏览器有 Range 可用，这个参数只服务推流客户端
- **重编码必须 CBR（`-b:a 320k`）**：pipe 不可 seek → 无法回写 Xing 帧（故显式 `-write_xing 0`），
  客户端只能按「首帧码率 × 字节数」估时长。**不要**改成项目别处的 `-q:a 0`：实测 105.4s 的流
  会被估成 97.6s，音箱据此可能提前判定播完
- **`-map 0:a:0` 不能删**：mp3 muxer 只接单条音频流，双音轨 `.mka`（#298）不选轨会直接失败，
  表现为静默降级回「从头播」，只有一行 warn，极难归因
- **只对本地歌曲与已缓存的网络歌曲生效**：未缓存时拿到本地文件要先同步下载整首，会让「续播」
  这一下按键卡住一整首下载时长。电台（直播无位置）、`media=video`（`-vn` 会丢画面）、HEAD 均忽略
- **seek 必须夹到距结尾 3 秒之外**（`parseSeekSeconds` 与插件侧 `playCurrent` 同源守卫）：
  越过文件尾 → ffmpeg 零输出 → 触发降级 → **整首从头重播**，比忽略 seek 更糟
- **不占 `transcodeSem`**（进程存活整首剩余时长，会饿死其他转码），改用本文件内 `cap=4` 的独立
  信号量，满了直接降级不排队；`exec.CommandContext` 另带「剩余时长 + 5min」硬超时回收孤儿进程。
  该信号量与上面的实时均衡流、变速流、普通转码流（#442）**共用**（`StreamSeekedMP3` 是同一个函数），
  所以 4 是这几类流的总额。
  实际占用时长取决于客户端读得多快而非歌曲时长——音箱是贪婪缓冲，实测 4 分钟的歌 10 秒就流完退出。
  槽位由 `io.Copy` 持有，**ctx 取消不会打断它**（io.Copy 只看读端 EOF / 写端出错），
  所以注册 playActivity 只能掐掉 ffmpeg 省 CPU，不能提前归还槽位
- **剩余时长未知（`songs.duration == 0`）时硬超时不能退化成 5 分钟**（见 `seekStreamUnknownDurationTimeout`）：
  对整首流那不是宽限而是硬上限，客户端边播边读会在第 5 分钟被杀掉，而响应正常闭合、客户端以为播完了。
  `duration == 0` 是常态（远程歌曲元数据未刷新、本地文件 tag 与 ffprobe 都拿不到时长）
- 响应是 chunked：无 `Content-Length`、不设 `Accept-Ranges`、`Cache-Control: no-store`
- 插件侧配套：`PlaylistManager.streamSeekOffsetSec` 记住当前流的起点。带 seek 的流对设备是
  「从 0 开始的新流」，它上报的 `play_song_detail.position` 只是流内偏移，**消费点必须补上偏移**
  （`handlers/playlist.ts` 的 `resolvePlayerStatus`、`voicecmd/engine.ts` 的 `resetAutoNextTimer`），
  否则续播后网页进度条会掉回 0、自动切歌被推迟一整个 seek 时长

### 歌曲持久化（song_downloader — 插件基础设施）

- **定位**：插件基础设施能力，不是主程序面向用户的功能。主程序提供 `songs.download` Bridge API，允许插件将用户自有网络存储（NAS/WebDAV/Subsonic 等）中的远程歌曲持久化到服务端本地 `music_path`，转为 `local` 类型。**此能力仅用于用户合法拥有的音乐资源，不得用于下载第三方商业音乐平台的受版权保护内容**
- 核心服务 `SongDownloader.Download`：获取音频（缓存命中直接 copy，否则同步下载）→ 可选转码 → 路径模板渲染 → 可选元数据嵌入（所有支持的格式）→ 更新 DB（type=local）
- **下载转码**（`SongDownloadOptions.Format` / `Quality`）：插件可传 `format`（mp3/m4a/ogg/flac/wav）+ 可选 `quality`（128/192/320），复用播放路径的 `CacheService.GetOrTranscode` 在下载时转成标准音频容器。典型用途：B 站等源产出 `.mov` 视频容器无法刮削歌词，转 mp3 后落地。`format` 为空=不转码保留源格式；转码依赖 ffmpeg，缺失/失败时优雅降级（仅 warn，保留源格式，不阻塞下载）
- **URL 歌词自动拉取**：`embed_metadata=true` 且 `lyric_source=url` 时，通过 `LyricFetcher` 拉取歌词 → 主歌词写入文件标签 → 完整 payload（含翻译/罗马音）缓存到 DB → `lyric_source` 更新为 `embedded`。拉取失败仅 warn 不阻塞持久化
- 通过 Bridge API `songs.download` 暴露给 JS 插件，权限映射到 `PermSongsWrite`
- 官方插件 `songloft-plugin-downloader`（独立仓库 `songloft-org/songloft-plugin-downloader`）基于此 API，提供将用户自有网络存储中的远程歌曲下载到本地的功能

### 文件搬移：跨设备 rename 陷阱

- `os.Rename` 在 src 和 dst 不在同一文件系统（挂载点）时会返回 `syscall.EXDEV`（cross-device link）错误
- 典型场景：`os.CreateTemp("")` 创建在系统 `/tmp`（tmpfs），目标 cache/music 目录挂载在独立磁盘或 Docker volume
- **统一使用** `internal/services.moveFile(src, dst)` 替代裸 `os.Rename`：先尝试 rename，EXDEV 时自动回退 copy + remove
- `pkg/tag` 的原子写不受影响：它用 `os.CreateTemp(dir, ...)` 在源文件**同目录**创建临时文件，rename 一定同设备
- 新增下载/缓存逻辑如果需要"先写临时文件再挪到目标位置"，**必须**用 `moveFile`，**不要**裸 `os.Rename`

