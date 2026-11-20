# AGENTS.md

本文件是 Songloft 的 AI 开发入口：通用铁律、常用命令与任务导航，业务细节按需读取。目录、依赖、API 和表结构以代码为事实来源；修改前先读适用的 `LESSONS.md`、子模块本地规范及文末[业务约束索引](#business-constraints)。英文版见 [AGENTS.en.md](AGENTS.en.md)。

## 文档导航

| 需要 | 去哪里 |
|---|---|
| AI 流程、验证记录、Issue 回复、复盘 | [docs/ai-development.md](docs/ai-development.md)、[LESSONS.md](LESSONS.md) |
| 业务约束与历史依据 | [docs/pitfalls.md](docs/pitfalls.md)（文末索引按任务跳转） |
| 架构 / 构建 / 部署 | [整体](docs/architecture.md)、[后端](docs/architecture_backend.md)、[前端](docs/architecture_frontend.md) |
| 数据库操作 | [docs/database_migrations.md](docs/database_migrations.md) |
| API 响应格式 / Swagger UI | [docs/api_response.md](docs/api_response.md) / 开发模式 `/swagger/index.html` |
| 前端踩坑 / UI 验证 | [docs/frontend_gotchas.md](docs/frontend_gotchas.md) |
| 文档站维护 | [docs/development.md](docs/development.md) |
| JS 插件 / 插件源 | [插件开发指南](docs/js-plugin-development-guide.md)、[插件源制作](docs/plugin_registry.md)、`plugins/toolchain/README.md` |

---

## 项目概述

Songloft 是自托管本地音乐服务器，支持**服务器部署**和 **Bundle 本地模式**（将 Go 后端嵌入客户端，无需单独部署服务器）。多仓库结构：

| 目录 | 技术 | 说明 |
|------|------|------|
| `/` | Go 1.26 + Chi v5 + SQLite | 后端 API 服务（默认端口 58091，账号 admin/admin） |
| `/mobile` | Go + gomobile | Go 后端的移动端绑定入口（gomobile bind，导出 Start/Stop/IsRunning/GetPort） |
| `/clients/player-flutter` ([独立仓库](https://github.com/songloft-org/songloft-player-flutter)) | Flutter 3.41+ / Dart 3.7+ | 跨平台前端（6 平台）；由 Flutter 仓库构建自己的 Bundle 客户端包 |
| `/clients/player-lynx` ([独立仓库](https://github.com/songloft-org/songloft-player-lynx)) | TS + ReactLynx | Android/iOS/HarmonyOS/Web 客户端；由 Lynx 仓库构建 Android/iOS Bundle，Windows/macOS 候选待统一验收；独立 main dev 构建与 v* 发版（子模块） |
| `/clients/tv` ([独立仓库](https://github.com/songloft-org/songloft-tv)) | Kotlin | Android TV 客户端（子模块） |
| `/plugins/toolchain` ([独立仓库](https://github.com/songloft-org/plugin-toolchain)) | TS + pnpm | JS 插件开发工具链（SDK / Builder / 脚手架） |
| `/plugins/src` | TS | JS 插件源码（子模块集合，各插件在自己仓库分发 release） |
| `/pkg/tag` | Go | 音频元数据**读写**库（基于上游 tag 库扩展 MP3/FLAC 写入） |
| `/integrations/home-assistant` ([独立仓库](https://github.com/songloft-org/home-assistant-addon)) | HA add-on | Home Assistant 加载项（薄层复用 Docker 镜像）。**清单 `repository.yaml` 必须在该仓库根目录**，这是拆出独立仓库的原因（#340）；设计/踩坑见 [README](integrations/home-assistant/README.md) |
| `/tools/ffmpeg-builder` ([独立仓库](https://github.com/hanxi/ffmpeg-builder)) | Docker | 静态编译 ffmpeg/ffprobe 的最小镜像构建器（子模块），供下载转码 / 音频指纹用 |
| `/tools/tracely` ([独立仓库](https://github.com/hanxi/tracely)) | Go + Vue | 自托管前端监控后端（安装/升级追踪），后端经其 Go SDK 上报（子模块，SDK 依赖走 go.mod） |

主仓库记录后端实现、公共宿主能力、接口契约及跨仓库通用约定；子模块的业务实现、设备兼容、历史问题与验证矩阵写在各自根目录的 `AGENTS.md`（同步 `AGENTS.en.md`）。修改子模块前先读其本地规范，涉及宿主接口时同时核对主仓库；子模块先提交，父仓库再更新指针。

## AI 开发流程

- 任务交办后连续完成调查、实现、验证、自审和交付，不逐阶段确认；关键缺失信息、产品决定或扩大范围才询问。提交、推送、正式发布和 Issue 写操作分别按用户授权，已有授权不重复询问。
- 关联 Issue 的修复推送后，主动准备并展示回复草稿和目标 Issue，等待评论确认；已有适用的评论授权则直接回复。草稿随 CI、发布状态更新，不等用户追问；详见 [关联 Issue 回复](docs/ai-development.md#关联-issue-回复)。
- 开始前读适用的 `LESSONS.md`，结束记录有证据的新经验并同步英文。三个不同日期的独立事件提出规范升级，同一事故重复复盘不递增；已有铁律立即执行。
- 排查未解决 Issue 包含主仓库和全部子模块，递归发现、去重、完整分页。统一 `owner/repo#编号`，缺口明确报告；结合正文、全部评论、最新代码与验证判断，不把 open 当未修复证明。
- 验证绑定仓库、SHA 或未提交工作树标识、环境、命令和结果；相关未跑项写 `Not run`。代码/子模块变化重跑受影响检查；追踪同一 SHA 的必要 CI，取消/跳过不算通过。详见 [AI 开发流程](docs/ai-development.md)。

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
make build-frontend-web-embedded   # Lynx Web，嵌入 Go 二进制（隐藏 API 地址 UI）
make build-frontend-web            # 独立部署 Flutter Web
make build-frontend-flutter-web-embedded # 旧 Flutter 嵌入版（覆盖同目录产物）
make build-frontend-{linux,windows,macos,android,ios,all}

# Go 移动绑定参考目标（客户端流水线开始打包时解析主仓库 main 的最新 SHA）
make build-go-mobile-android       # Android .aar（gomobile bind，arm64 + arm + x86_64）
make build-go-mobile-ios SONGLOFT_FFMPEG_IOS_ASSET=/path/to/staged/ios-ffmpeg  # iOS arm64 .xcframework，仅 macOS
make build-go-desktop-linux        # Linux 可执行文件
make build-go-desktop-windows      # Windows .exe
make build-go-desktop-macos        # macOS x86_64
make build-go-desktop-macos-arm64  # macOS ARM64

# 前端开发
cd clients/player-flutter && flutter run -d chrome          # standalone
cd clients/player-flutter && flutter run -d chrome --dart-define=DEPLOY_MODE=embedded
```

## 代码格式化（铁律）

每次修改代码后**必须**格式化，提交前确认无格式差异：

- **Go**：项目根目录执行 `gofmt -w .`
- **Dart**：`clients/player-flutter/` 下执行 `dart format lib/ test/`

## 数据库规范（铁律）

访问栈：**goose 迁移 + sqlc 固定 SQL + squirrel 动态 SQL + Repository + UnitOfWork**。完整步骤见 [数据库操作指南](docs/database_migrations.md)。

- **改 schema** → `internal/database/migrations/000N_xxx.sql`，启动时 `goose.Up` 自动执行；**禁止**手动 `ALTER data/songloft.db`
- **加固定 SQL** → `internal/database/queries/{table}.sql` + `make sqlc`；生成产物 `internal/database/sqlc/` 必须入库
- **动态 SQL（变长 WHERE/SET）** → 在 `*_repository.go` 内用 squirrel，禁止拼字符串
- **跨表写** → `db.RunInTx(ctx, func(ctx, uow))` 拿同一 `*sql.Tx` 下的 `uow.Songs/Playlists/...`；**禁止** service 层手 `BeginTx`，否则会 SQLITE_BUSY
- **错误语义** → 仓储未命中统一 `database.ErrNotFound`；service 用 `errors.Is` 判别
- **测试** → `testutil.OpenMemoryDB(t)` 跑真实 `:memory:` + 真实 Repository；**禁止**手写 mockDB
- **内置数据** → 迁移预置歌单 id=1「收藏」、id=2「电台收藏」（`labels=["built_in"]`），及 `music_path / jwt_secret / source_*` 默认 config；测试行数断言记得扣掉
- sqlc 高频失败（`-- name:` 后的中文注释、自引用子查询外层列限定）见[常见错误](docs/database_migrations.md#sqlc-common-errors)

## 后端编码约定

- 标准 Go layout（`internal/` 防外部依赖），Chi v5 路由，JWT 双 Token
- 依赖注入：普通读写走 Repository 接口；跨表事务复用 `RunInTx + UnitOfWork`（如 `PlayHistoryService` 接收 `database.DB`）。**禁止** service 自行 `BeginTx` 或绕过仓储写 SQL
- 日志用标准库 `slog`；HTTP 错误统一 `respondError`
- **API 响应格式**：RESTful 直返，**禁止** `{code, data, message}` 信封；错误统一 `{"error","detail"}`。完整规范见 [docs/api_response.md](docs/api_response.md)
- 测试文件 `*_test.go` 与源码同目录

## API 文档规范（铁律）

**所有在 `internal/app/routers.go`（含 `RegisterStaticRoutes` / `RegisterAPIRoutes` 等子注册函数）里注册的 handler 必须有 swag 注释**。API 文档由 [swaggo/swag](https://github.com/swaggo/swag) 生成，是前端开发与外部集成的唯一来源。

必填字段（每个 handler 至少 7 项）：

```go
// @Summary <一行中文摘要>
// @Description <详细描述；说清副作用 / 默认值 / 错误码触发条件>
// @Tags <业务分组，中文>
// @Produce json
// @Success 200 {object} <返回类型> "<说明>"
// @Security BearerAuth
// @Router /<path> [<method>]
```

- 有请求体加 `@Accept json` 和 `@Param request body <type> true "<说明>"`；路径/查询参数用 `@Param <name> path/query <type> true/false "<说明>"`；明显错误路径加 `@Failure 400/404/500 {object} map[string]string "..."`
- **公开端点**（无需 token，如健康检查）省略 `@Security BearerAuth`
- 复用现有业务 tag（「歌曲管理」「歌单管理」「电台与 HLS」「扫描管理」「配置管理」「缓存管理」「JS 插件」「JS插件管理」「数据备份」「设置」「系统升级」「认证管理」「系统管理」「资源代理」「主题包」「播放历史」「标签管理」），不要随手造新 tag
- **`@Router` 路径禁止包含 `/api/v1` 前缀**：`main.go` 已声明 `// @BasePath /api/v1`，写了会生成 `/api/v1/api/v1/...`
- 多别名 / catch-all：每条 alias 单写一行 `@Router`；HEAD 不单独列；`r.HandleFunc` 列出所有实际方法；动态路径在 `@Description` 注明由运行时决定
- **没有豁免**：catch-all、静态资源、反代端点也要写；`@Description` 说明它是什么、OpenAPI schema 为何不精确

改完必跑 `make swagger` 并入库 `docs/swagger.json`、`docs/swagger.yaml`、`docs/docs.go`；在输出里确认新 `@Router` 与请求/响应类型已生成，`grep '<path>' docs/swagger.json` 有命中，必要时在 `/swagger/index.html` 目测。

## 配置接口规范（铁律）

**用户可见的功能开关一律走业务端点；通用 KV 只是 admin 后门。**

- 孤立配置默认放 `/api/v1/settings/<kebab-case-name>`：强类型 JSON（非 `{value: string}`）；默认值与副作用都在 handler 内处理，前端无需先 POST 创建；命名套路 `Is<Name>Enabled/Set<Name>Enabled` + `Get<Name>Setting/Update<Name>Setting`
- 与模块动作端点强相关的配置留在 `/api/v1/<module>/config`（如 `/cache-manage/config` 与 `stats/clean` 共用 `CacheService`），不要强拆到 `/settings/`
- `/api/v1/configs/{key}` 仅限 `config_manager.dart` 等通用编辑器；新业务功能禁止直调（PUT 不存在的 key 返回 404，且无强类型/默认值/副作用）。保留双入口时，副作用必须同时挂在 `configHandler.SetOnConfigChanged`
- 客户端：业务 Provider 走 `SettingsApi`；`ConfigApi` 仅用于 admin UI
- 完整路径风格、数据形态与历史决策见[业务约束 · 配置接口](docs/pitfalls.md#config-endpoints)

## 文档规范（铁律）

- **中英双语**：`README.md` ↔ `README.en.md`，`AGENTS.md` ↔ `AGENTS.en.md`，`docs/<name>.md` ↔ `docs/en/<name>.md`。新增/修改/删除内容、结构或链接都必须同步另一版本；改前先确认对应文件存在，例外须是有意为之
- **文档站**：改前读 [docs/development.md](docs/development.md)。`quick-start` / `changelog` 由根 `README` / `CHANGELOG` 生成，禁止手改；子模块同步页去源仓库改再 bump 指针；`docs/repowiki/` 是入库的手工文档，直接改

## Git 提交约定

- **直接提交到 `main`**，不新建功能分支、不走 PR 流程
- 提交信息**禁止**添加 `Co-Authored-By` 尾部标记
- 遵循 Conventional Commits：`type(scope): description`，description 和 body 尽量用中文
- 关联 GitHub Issue 的提交必须带引用。`#123` 只指向**当前仓库**的 Issue；跨仓库一律写完整 `owner/repo#123`（子仓库引用父仓库 Issue 也必须完整路径）

## 构建与部署

- 构建标签：`dev`（含 Swagger + pprof）/ `lite`（不嵌前端）/ 无标签（完整版，嵌 Lynx Web）；`VERSION=dev` 自动启用 `-tags dev`
- `VERSION` 与 `BUILD_TYPE` 正交（`lite` / 空即 `full`）；**禁止** `BUILD_TYPE=dev` 等混合值。前端嵌入路径固定为 `clients/player-build/web-embedded`
- 完整构建参数、SPA 回退、子路径部署见[架构 · 构建系统](docs/architecture.md#build-system)；Bundle 部署模式见[前端架构 · 部署模式](docs/architecture_frontend.md#deployment-modes)；Docker 热替换与 PUID/PGID 见[架构 · Docker](docs/architecture.md#docker-entrypoint)和[快速开始](docs/quick-start.md)
- 升级检查 `/api/v1/upgrade/check` 非 Docker 也可调用并返回 `is_docker`；执行升级/回退仅 Docker 可用
- 平台差异（音频后端、Android/iOS/macOS 等）见[前端踩坑](docs/frontend_gotchas.md)与[前端架构 · 平台适配](docs/architecture_frontend.md#platform-adaptation)

## 前端 UI 验证

需要真的在界面跑一遍时，用 Docker 无头 Chrome；完整命令、脚本形态与断言要求见 [AI 开发流程 · 前端 UI 验证](docs/ai-development.md#frontend-ui-verification)。Flutter Web 的语义节点、label 合并、深链等驱动坑见[前端踩坑 · 驱动 Flutter Web](docs/frontend_gotchas.md#drive-flutter-web)。

## JS 插件

- 源码 `plugins/src/<name>/`，产物在各插件仓库 GitHub Releases；新建插件用 `npx create-songloft-plugin@latest`（WebView / Lynx 模板）
- 沙盒 QuickJS；桥接由 `internal/jsplugin/api_bridge.go` 的 `HandleBridgeCall` 分发，namespace：`storage`、`persistent-storage`、`songs`、`playlists`、`tags`、`comm`、`plugin`、`command`、`jsenv`、`fs`、`net`、`websocket`。`fetch`、`console`、定时器是宿主内置能力，不是桥接
- 路由 `/api/v1/jsplugin/{entry_path}/...`；公共资源 `/api/v1/jsplugin-assets/*` 由 `injectHTMLHead` 按 theme.css → components.css → common.js 注入，新增资源必须登记 `assetVersions`
- `window.SongloftPlugin` 的公开成员以 `common.js` 末尾对象字面量为唯一真实来源；插件用 `@songloft/client-sdk` 的类型，不要手写宿主 API 声明。宿主桥接 namespace：`host`、`player`、`cookies`、`favorite`（插件改收藏后必须 `favorite.refresh`）
- Lynx 插件用 `@songloft/lynx-plugin-sdk` 与 `NativeModules.SongloftPluginBridge`，不使用 `window.SongloftPlugin`
- 权限：manifest 的 `permissions` 由运行时校验；**新增权限必须同步宿主 `AllPermissions`、toolchain `VALID_PERMISSIONS`、脚手架 `AVAILABLE_PERMISSIONS` 三处**。`net` 不覆盖 `net:insecure-tls`
- 后台 pump 必须事件驱动（`asyncSignal` + `GetNextTimerDeadline`），不要退回固定轮询；`executeInEnv` 返回时必须 `signalAsync`
- UDP/TCP 各自独立计每插件 8 个 socket 配额，活跃 socket 阻止空闲驱逐；TCP 仅允许私有/回环/链路本地地址，数据为 base64
- 歌词/封面提供者（`songloft.lyrics` / `songloft.covers`）无需权限，first-match-wins，注册在空闲驱逐后不丢失
- 完整 API、模板与宿主实现约束见[插件开发指南](docs/js-plugin-development-guide.md)；私有源认证见[插件源制作](docs/plugin_registry.md#私有源认证)

---

<a id="business-constraints"></a>

## 业务约束（按任务必读）

修改下列模块前，**必须先读对应章节**；涉及多个模块时一并阅读。这里只保留提醒，完整约束与历史依据见 [业务约束与踩坑](docs/pitfalls.md)。新经验遵循 `LESSONS.md` 流程，专题细节维护在对应文档，不再追加到入口。

| 修改范围 | 关键提醒 | 必读章节 |
|---|---|---|
| 插件商店身份 | 冲突必须在写入前返回；安装态按 entry_path + identity 匹配。 | [详情](docs/pitfalls.md#plugin-identity) |
| 插件商店缓存 | 仅缓存源数据，不缓存安装态；token 哈希入键，proxyDown 按次隔离。 | [详情](docs/pitfalls.md#registry-cache) |
| 扫描标题与验证目录 | 默认标签优先，尊重 filename 配置；验证曲库不能放在 /tmp 下。 | [详情](docs/pitfalls.md#scan-title) |
| 流式扫描入库 | 按目录定稿；取消保留已提取成果，写库使用独立 flushCtx。 | [详情](docs/pitfalls.md#scan-batches) |
| 目录遍历 | 避免逐项 Stat；保留目录名缓存、单飞与脱离请求取消的语义。 | [详情](docs/pitfalls.md#directory-walk) |
| DSF / DSD | 扫描默认值与迁移同步；不支持 DSD 的播放器需显式转码。 | [详情](docs/pitfalls.md#dsf) |
| 旁挂歌词 | 固定候选路径探测；不覆盖 manual，不把旁挂歌词回写音频标签。 | [详情](docs/pitfalls.md#sidecar-lyrics) |
| 标签写入 | 同目录临时文件原子写入；不支持的格式降级，不阻塞主流程。 | [详情](docs/pitfalls.md#tag-writing) |
| 音频指纹 | 默认不自动计算；采样 ≤120 秒、失败落库、可取消、并发随 CPU 收敛。 | [详情](docs/pitfalls.md#fingerprints) |
| 播放历史 | 按上下文仅记录 play；upsert 与 50 条裁剪同事务，不把序数当稳定位置。 | [详情](docs/pitfalls.md#play-history) |
| HLS 代理 | 默认关闭；保留同源校验与 SSRF 防护，改写 URL 使用相对路径。 | [详情](docs/pitfalls.md#hls-proxy) |
| 外发 HTTP 代理 | 使用代理感知客户端；ffmpeg 仅支持 HTTP/HTTPS 代理。 | [详情](docs/pitfalls.md#http-proxy) |
| 私网代理白名单 | 空白名单维持私网封禁；仅影响资源及转码代理，不放宽 HLS。 | [详情](docs/pitfalls.md#private-allowlist) |
| 远程 URL 转码 | 不落盘、不入库；与 seek/均衡流共享 4 个槽位，满则 503。 | [详情](docs/pitfalls.md#url-transcode) |
| 音乐缓存 | 流式代理不等整首下载；落盘转码失败保留原码，不改变显式下载语义。 | [详情](docs/pitfalls.md#music-cache) |
| 均衡与实时转码 | 共用可配置响度及缓存键；保留 Range、精确字节数、延迟 206 与 Flush。 | [详情](docs/pitfalls.md#normalization) |
| 服务端 seek | seek 与 Range 可组合；保留 CBR、尾部 3 秒护栏和未知时长超时。 | [详情](docs/pitfalls.md#seek-stream) |
| 歌曲持久化 | 仅用于用户合法拥有的音乐；转码或歌词获取失败不阻塞持久化。 | [详情](docs/pitfalls.md#song-download) |
| 跨设备文件搬移 | 使用 internal/fileutil.MoveFile；不要用裸 os.Rename 搬临时文件到其他挂载点。 | [详情](docs/pitfalls.md#file-move) |
