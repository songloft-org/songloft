# AGENTS.md

本文件为 AI 编程助手提供 Songloft 项目的**入口信息**：项目结构、常用命令、铁律与踩坑总结。代码本身就是真实来源的内容（目录树、依赖、API 表、表结构）请直接看代码或下方链接的详细文档。

> **详细文档**：
> - 架构：[整体](docs/architecture.md) · [后端](docs/architecture_backend.md) · [前端](docs/architecture_frontend.md)
> - 专题：[数据库操作](docs/database_migrations.md) · [颜色系统](docs/color_system.md) · [API 响应格式](docs/api_response.md) · [快速上手](docs/quick-start.md)
> - 插件开发：见 `plugin-toolchain/README.md`（独立仓库）
> - 插件源制作：[插件源制作指南](docs/plugin_registry.md)
> - API：开发模式启动后访问 `/swagger/index.html`

---

## 项目概述

Songloft 是自托管本地音乐服务器，支持**服务器部署**和**Bundle 本地模式**（将 Go 后端嵌入客户端，无需单独部署服务器）。多仓库结构：

| 目录 | 技术 | 说明 |
|------|------|------|
| `/` | Go 1.26 + Chi v5 + SQLite | 后端 API 服务（默认端口 58091，账号 admin/admin） |
| `/mobile` | Go + gomobile | Go 后端的移动端绑定入口（gomobile bind 用，导出 Start/Stop/IsRunning/GetPort） |
| `/songloft-player` ([独立仓库](https://github.com/songloft-org/songloft-player)) | Flutter 3.29+ / Dart 3.7+ | 跨平台前端（6 平台），支持 Bundle 本地模式 |
| `/plugin-toolchain` ([独立仓库](https://github.com/songloft-org/plugin-toolchain)) | TS + pnpm | JS 插件开发工具链（SDK / Builder / 脚手架） |
| `/jsplugins-src` | TS | JS 插件源码（子模块集合，每个插件在自己仓库下分发 release） |
| `/pkg/tag` | Go | 音频元数据**读写**库（基于上游 tag 库扩展 MP3/FLAC 写入） |
| `/addon` | HA add-on | Home Assistant 加载项（薄层复用 Docker 镜像）。设计/踩坑/发版见 [addon/README.md](addon/README.md) |

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

# 前端构建（产物落到 songloft-player-build/，供后端嵌入或独立部署）
make build-frontend-web-embedded   # 嵌入 Go 二进制用（隐藏 API 地址 UI）
make build-frontend-web            # 独立部署 web
make build-frontend-{linux,windows,macos,android,ios,all}

# Bundle 本地模式（Go 后端编译为移动端库 / 桌面端可执行文件）
make build-go-mobile-android       # Android .aar（gomobile bind，arm64 + arm）
make build-go-mobile-ios           # iOS .xcframework（gomobile bind，arm64，仅 macOS）
make build-go-desktop-linux        # Linux 可执行文件
make build-go-desktop-windows      # Windows .exe
make build-go-desktop-macos        # macOS x86_64
make build-go-desktop-macos-arm64  # macOS ARM64

# 前端开发
cd songloft-player && flutter run -d chrome          # standalone
cd songloft-player && flutter run -d chrome --dart-define=DEPLOY_MODE=embedded
```

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
- **业务 tag 命名**：复用现有 tag（「歌曲管理」「歌单管理」「电台与 HLS」「扫描管理」「配置管理」「缓存管理」「JS 插件」「数据备份」「设置」「升级」「认证」），不要随手造新 tag

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

- 路径风格：`/settings/<kebab-case-name>`（如 `/settings/hls-proxy`、`/settings/music-path`、`/settings/http-proxy`）
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

- `SettingsApi`（`songloft-player/lib/features/settings/data/settings_api.dart`）封装所有 `/settings/*` 调用，业务功能 Provider 一律走它
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

---

## Git 提交约定

- 提交信息**禁止**添加 `Co-Authored-By` 尾部标记
- 遵循 Conventional Commits 格式：`type(scope): description`
- 关联 GitHub issue 的提交信息必须带 issue 引用
- issue 引用规则：短写 `#123` 永远指向**当前 commit 所在仓库**的 issue；只要引用的不是当前仓库的 issue，就必须写完整 `owner/repo#123`
  - 父仓库 `songloft-org/songloft` 的 commit 引用父仓库 issue：可写 `#155`，也可写 `songloft-org/songloft#155`
  - 子仓库（如 `pkg/tag`、`songloft-player`、`plugin-toolchain`、`jsplugins-src/*`）的 commit 引用自身仓库 issue：可写 `#14`，也可写完整仓库路径
  - 子仓库的 commit 引用父仓库 issue：必须写完整路径，如 `songloft-org/songloft#155`，不能只写 `#155`（否则 GitHub 会解析为子仓库自身的 issue）
  - 任意跨仓库引用一律写完整路径，如 `songloft-org/songloft-player#14`

---

## 构建与部署

- 构建标签：`dev`（含 Swagger + pprof） / `lite`（精简版，不嵌前端） / 无标签（完整版，嵌 Flutter Web）
- `VERSION=dev` 时 Makefile 自动启用 `-tags dev`（无需手动传 `EXTRA_TAGS=dev`）
- 两个正交维度：**VERSION**（`dev` / `X.Y.Z`）控制是否为开发版；**BUILD_TYPE**（`lite` / 空即 `full`）控制是否嵌入前端。**禁止** `BUILD_TYPE=dev` 等混合值
- 嵌入路径是 `songloft-player-build/web-embedded`（**不是** `songloft-player/build/web-embedded`）
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
- 关键入口：`mobile/mobile.go`（gomobile 绑定）、`songloft-player/lib/core/backend/`（Flutter 侧抽象层）
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

---

## 平台适配踩坑

- 升级检查 (`/api/v1/upgrade/check`) 仅 Docker 可用
- Flutter `secure_storage` 在 macOS 未签名沙盒下自动降级到 SharedPreferences
- Android 构建前需 `sdkmanager --licenses`；Android 13+ 需运行时申请通知权限
- Windows/Linux 音频后端走 `just_audio_media_kit`（libmpv）
- HyperOS3 等需 `androidStopForegroundOnPause: false` 防后台回收
- **Bundle 模式 Android**：CWD 是 `/`，covers 目录路径必须相对于 `DBPath` 而非 CWD 解析（`da65db1` 修复）
- **Bundle 模式原生桥接**：Android 用 `Class.forName("mobile.Mobile")` 反射调用 gomobile 生成类，未打包 `.aar` 时 `isAvailable()` 返回 false（优雅降级）；iOS 同理用 Swift 调用 `MobileStart` 等 Objective-C 函数
- **Bundle 桌面子进程**：`DesktopBackendService` 在 Flutter 可执行文件**同目录**（macOS 在 `Contents/Resources/`）查找 `songloft-server`，通过 stdout 解析实际监听端口

---

## JS 插件

- 源码 `jsplugins-src/<name>/`，构建产物在各插件仓库的 GitHub Releases
- 新建插件：`npx create-songloft-plugin@latest`（交互式脚手架，详见 `plugin-toolchain/README.md`）
- 沙盒：QuickJS，通过 `internal/jsruntime` 提供的 `host` 桥接调用宿主能力（`http.fetch`、`storage`、`logger`、`songs.*`、`playlists.*`）
- 路由：`/api/v1/jsplugin/{entry_path}/...`
- 公共资源：`/api/v1/jsplugin-assets/*` 提供嵌入在 Go 二进制中的 `common.css`/`common.js`/字体，`injectHTMLHead` 自动注入到所有插件 HTML 页面
- 主题同步：`common.js` 内含 embed 检测 + 主题桥接（URL `?theme=` 参数 + `postMessage` 实时更新 + `data-theme` 属性 + `songloft-theme-change` 事件），暴露 `window.SongloftPlugin` 全局 API（`getTheme`/`onThemeChange`/`apiGet`/`apiPost` 等）
- `common.css` 定义 `--md-*` CSS 变量（亮/暗双主题），所有使用这些变量的插件自动跟随主题切换
- 权限：manifest 中 `permissions: ["net", "storage", "fs:music", ...]`，运行时由 `internal/jsplugin` 校验
- 健康检查 + 文件指纹热更新均自动进行
- **UDP Socket API**（`songloft.net`，需 `net` 权限）：Go 侧托管 UDP socket + 消息推送模式。`udpBind` 创建 socket 并启动 reader goroutine，收到的 UDP 包通过 scheduler 队列异步推送到 JS 回调（`onData`）。支持多播组（`udpJoinMulticast/udpLeaveMulticast`），典型用途：SSDP 设备发现（DLNA/UPnP）。每插件最多 8 个 socket，有活跃 socket 的插件不会被空闲驱逐，插件卸载时自动清理。实现在 `internal/jsplugin/api_bridge_net.go`
- **私有源认证**：`RegistryConfig` 支持 `token` 字段，拉取该源下所有资源时自动携带 `Authorization: Bearer <token>` 头，兼容 GitHub 私有仓库 PAT 和自托管私有源。详见 [插件源制作指南 · 私有源认证](docs/plugin_registry.md#私有源认证)

---

## 业务踩坑总结（重要 — 不在代码里）

### scan 标题规则

- tag 有 title → 直接用 `tag.Title`
- tag 没 title → 文件名去扩展名
- **不要**再做"最长公共子串去重 + 拼接"，会产生"艺术家 - 标题"这种把艺术家冗余到标题字段的结果

### tag 写入（pkg/tag）

- `tag.WriteTag(filePath, opts)` 按扩展名 dispatch，所有格式均使用临时文件 + `os.Rename` 原子写入
- 支持矩阵：

| 格式 | 文本字段 | 歌词 | 封面 |
|------|---------|------|------|
| MP3 | ID3v2.3 text frames | USLT | APIC |
| FLAC | Vorbis Comment | LYRICS | PICTURE block |
| M4A/MP4/M4B/MOV | iTunes atoms (©nam 等) | ©lyr | covr |
| OGG/Opus | Vorbis Comment | LYRICS | METADATA_BLOCK_PICTURE (base64) |
| APE | APEv2 text items | Lyrics | Cover Art (Front) (binary item) |
| WAV | RIFF LIST INFO | ICMT | **不支持**（格式限制） |
| AIFF/AIF | ID3v2.3 (ID3 chunk) + NAME/AUTH | USLT (ID3 chunk) | APIC (ID3 chunk) |
- 不支持的格式 → 返回 `ErrUnsupportedWrite`，调用方**必须**降级为日志，**不要**阻塞主流程

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

### 音乐缓存（cache_service）

- 播放远程歌曲时流式代理上游音频到客户端（不阻塞），同时后台异步写入缓存；后续播放缓存命中后直接从本地返回
- 流式代理 `ServeRemoteResourceWithCache`：200 OK 时 TeeReader 同时代理+写临时文件，206 Partial 时正常代理并触发异步全量下载
- 缓存路径持久化在 `songs.cache_path` 字段（DB 级别），查找时优先 `cache_path`，fallback 到旧格式哈希分桶目录
- 缓存目录默认 `{data_dir}/music_cache/`，可通过 `PUT /api/v1/cache-manage/config` 的 `cache_dir` 字段自定义为绝对路径
- 启动时从 `music_cache_config` 配置读取自定义目录；运行时切换目录会自动重建 LRU 索引，不迁移旧文件
- LRU 淘汰：超出 `max_size`（默认 1GB）时按最后访问时间淘汰，`max_size=0` 表示不限制
- `POST /api/v1/cache-manage/validate-dir` 可预先验证目录（自动创建 + 可写性检查 + 返回磁盘空间）
- inflight 去重：同 `song.ID` 的并发请求只下载一次；首请求被 `ctx.Canceled` 时后续等待者自动重试

### 歌曲持久化（song_downloader — 插件基础设施）

- **定位**：插件基础设施能力，不是主程序面向用户的功能。主程序提供 `songs.download` Bridge API，允许插件将用户自有网络存储（NAS/WebDAV/Subsonic 等）中的远程歌曲持久化到服务端本地 `music_path`，转为 `local` 类型。**此能力仅用于用户合法拥有的音乐资源，不得用于下载第三方商业音乐平台的受版权保护内容**
- 核心服务 `SongDownloader.Download`：获取音频（缓存命中直接 copy，否则同步下载）→ 路径模板渲染 → 可选元数据嵌入（所有支持的格式）→ 更新 DB（type=local）
- **URL 歌词自动拉取**：`embed_metadata=true` 且 `lyric_source=url` 时，通过 `LyricFetcher` 拉取歌词 → 主歌词写入文件标签 → 完整 payload（含翻译/罗马音）缓存到 DB → `lyric_source` 更新为 `embedded`。拉取失败仅 warn 不阻塞持久化
- 通过 Bridge API `songs.download` 暴露给 JS 插件，权限映射到 `PermSongsWrite`
- 官方插件 `songloft-plugin-downloader`（独立仓库 `songloft-org/songloft-plugin-downloader`）基于此 API，提供将用户自有网络存储中的远程歌曲下载到本地的功能

### 文件搬移：跨设备 rename 陷阱

- `os.Rename` 在 src 和 dst 不在同一文件系统（挂载点）时会返回 `syscall.EXDEV`（cross-device link）错误
- 典型场景：`os.CreateTemp("")` 创建在系统 `/tmp`（tmpfs），目标 cache/music 目录挂载在独立磁盘或 Docker volume
- **统一使用** `internal/services.moveFile(src, dst)` 替代裸 `os.Rename`：先尝试 rename，EXDEV 时自动回退 copy + remove
- `pkg/tag` 的原子写不受影响：它用 `os.CreateTemp(dir, ...)` 在源文件**同目录**创建临时文件，rename 一定同设备
- 新增下载/缓存逻辑如果需要"先写临时文件再挪到目标位置"，**必须**用 `moveFile`，**不要**裸 `os.Rename`
