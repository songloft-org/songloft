# Songloft 前端踩坑与铁律（Frontend Gotchas）

本文沉淀 Flutter 客户端（`songloft-player-flutter`）里**已定位并修复、但根因具有普适性**的坑，主要集中在 Flutter Web（CanvasKit）渲染、平台视图（插件 iframe）、音频后端。这些坑反复消耗过大量排查回合，归档在此避免重蹈。

> 后端/业务侧踩坑见[业务约束与踩坑](pitfalls.md)；入口导航见项目根 `AGENTS.md`。

---

## 一、封面 / 图片渲染（铁律）

### 铁律：所有封面必须走 `CoverImage` / `NetworkCoverImage`，禁止裸 `CachedNetworkImage` / `Image.network`

- 统一封装：`lib/shared/widgets/cover_image.dart`（`CoverImage`）和 `lib/shared/widgets/network_cover_image.dart`（`NetworkCoverImage`），Web 下强制 `imageRenderMethodForWeb: HttpGet` + `memCacheWidth`（按显示物理像素缩略解码）。
- **新增任何封面渲染点**都必须用这两个组件之一；不要直接写 `CachedNetworkImage(...)` 或 `Image.network(...)`。

### 为什么（Web CanvasKit 的 GPU 显存陷阱）

Web 端封面偶发**变黑**（纯黑，不是加载失败占位图标）或退化成占位图标，触发场景是**应用内切页/切 tab/筛选后回来**：

- 裸 `CachedNetworkImage` 在 Web 默认走 `ImageRenderMethodForWeb.HtmlImage`，**全分辨率**（单张可达 ~1.5MB）上传为 GPU 纹理。
- CanvasKit 只有单一 WebGL context（Chrome 走 `OffscreenCanvasRasterizer`）。插件 iframe / 视频等 platform view 各占用 GPU 资源，**满分辨率封面纹理累积 → 挤爆 GPU 显存 → WebGL context 被丢弃**（`ImageCodecException: Failed to create image` / `MakeLazyImageFromTextureSourceWithInfo` 返回 null）。context 死亡后复用的 `ui.Image` 纹理直接绘制成黑；context 失效期新解码拿不到纹理则退化成占位图标。
- `memCacheWidth` **只在 `HttpGet` 路径生效**，在默认 `HtmlImage` 路径不缩略——所以必须显式设 `imageRenderMethodForWeb: HttpGet`，把每张封面从 ~1.5MB 缩到数十~百 KB，大幅降低显存压力。

### 排查教训（避免重蹈 9 轮弯路）

- **纯黑 ≠ 加载失败**：纯黑是 GPU 纹理层，`errorWidget` 捕获不到；占位图标才是加载/解码失败。先分清。
- `imageCache.evict` / 换 key / 挂 `NavigatorObserver` 主动重建都**只是概率缓解**，甚至会加剧重解码风暴。GPU 纹理生命周期问题的确定根治只有：缩小解码尺寸（`HttpGet`+`memCacheWidth`）、减少并存 platform view、或升级引擎。
- `canvasKitForceCpuOnly`（CPU 软件渲染）能消掉 GPU 纹理黑，但**非常卡**且封面仍会因加载层问题丢失，是负收益，已排除。
- 修某类图片问题要 **grep 全部渲染点**（`CachedNetworkImage` / `Image.network`），别只改一个共享组件就以为覆盖全——本项目封面渲染散落在 `CoverImage` + 多处卡片组件。
- 诊断决定性突破来自**让真机贴 console 日志**（具体错误串 + `[Cover] OK 但显示黑` + cache 未淘汰），别连续多轮纯推断改代码。

---

## 二、Web 插件 Tab 的 iframe 反复重载 / 抖动

承载插件页的 `<iframe>`（`HtmlElementView` 平台视图）曾出现反复重新加载（入口页被 25~40 次/秒重复请求）与视觉抖动。已修复，涉及四层根因（第 4 层不止插件页，**整个 App 都会抖**）：

### 1. widget 树层：iframe 重载 ⟺ 承载它的 widget 元素 dispose+重建

Flutter Web 里 iframe DOM 元素按 platform view 的 `viewId` 缓存一次，经 shadow-DOM `<slot>` 投影——普通 rebuild / 重定位**不会**动 iframe，只有 **viewId 被销毁并重建**才重新拉取。

- **铁律**：插件保活 Stack（`shell_layout.dart`）里，凡让 `PluginTabPage` 元素 dispose+重建的都会触发 iframe 重载——重排、换父（local `ValueKey` 不能跨父移动）、依赖会瞬时为空的 provider 误删激活 tab。
- 修复：① `PluginTabPage` 用**按 entryPath 缓存的稳定 `GlobalKey`**（跨树移动而非重建）；② 保活裁剪只按稳定的 `tabConfig.pluginTabs` 且仅 `tabConfigAsync.hasValue` 时执行，不依赖会瞬时为空的 `jsPluginsProvider` 快照。工厂内缓存 iframe **无效**（引擎重建 viewId 时会把 iframe 挪到新 wrapper，move 仍触发浏览器重载）。

### 2. CSS 层：文档级滚动条增删的重排回路

内容高度 ≈ 视口时「滚动条出现→内容变窄→重排→高度变化→滚动条消失→…」每帧翻转。

- 修复：`internal/jsplugin/assets/theme.css` 给**嵌入态**的 `html.embed` 加 `overflow-y: scroll`（滚动条常驻、宽度恒定，打断回路），并配 `scrollbar-width: none` + `::-webkit-scrollbar { display: none }` 让它不占布局宽度（否则插件里 `100vw` 定宽元素会横向溢出，#341）。只在嵌入态强制：普通浏览器直开插件页窗口可自由伸缩、无此回路。
- 注意：`scrollbar-gutter: stable` **只对滚动容器（`overflow:auto/scroll`）生效**，`html` 是 `visible` 时无效——这是它一度没修好的原因。Linux/headless 用覆盖式（0 宽度）滚动条永远复现不了，Windows Chrome（经典 ~15px 滚动条）才复现。

### 3. 缓存层（铁律）：immutable 长缓存资源必须走版本化 URL

CSS 改对了用户却仍抖——真凶是缓存头。`jsplugin-assets/*`（当时是 `common.css`/`common.js`，现已拆为 `theme.css` / `components.css` / `common.js`）原用**固定无版本 URL** + `Cache-Control: immutable`，浏览器连重新验证都不做，把旧文件缓存一年，修复永远到不了用户。

- **通用铁律**：凡 `immutable` 长缓存的资源，**必须走内容哈希版本化 URL**（如 `?v=<sha256前8位>`），否则任何后续修改对老用户都不可达。
- 实现：`injectHTMLHead` 给注入的每个公共资源 URL 加 `?v=<hash>`（`internal/jsplugin/routes.go` 的 `assetVersions`），承载页 HTML 为 `no-cache` 每次带出最新版本号；内容不变时 URL 恒定，长缓存照旧。新增公共资源必须同时登记进 `assetVersions`。

### 4. 宿主页层（铁律）：`<html>` 必须禁掉视口滚动条，否则整页抖（#439）

第 2 层只修了插件 iframe 的**文档**，同一条回路在 **Flutter 宿主页**上也成立——表现是首页 / 曲库 / 插件商店 / 插件页**全都抖**，与插件无关。

- **机制**：引擎的 `FullPageEmbeddingStrategy` 只给 `<body>` 设 `position:fixed; inset:0; overflow:hidden`，**`<html>` 保持默认 `overflow:visible`**。只要有任何东西把文档撑破几像素（浏览器扩展注入到 `<html>` 的浮层等，我们自己的 DOM 全在 fixed 的 body 里、不参与），经典滚动条一出现就吃掉 15px 视口 → 跟着视口定尺寸的 body 变窄 → Flutter 整页按新尺寸重排 → 不再溢出 → 滚动条消失 → 每帧翻转，肉眼 ~15Hz 抖动。
- **修复**：`clients/player-flutter/web/index.html` 加 `html { overflow: hidden; scrollbar-width: none }` + `html::-webkit-scrollbar { display: none }`。宿主页永远不需要文档级滚动（Flutter 视图是 fixed 全屏、滚动全在应用内），禁掉即让回路结构上不成立，且不依赖「查出谁撑破了文档」。
- **诊断口径**（视觉抖动无法在覆盖式滚动条环境复现，但机制可量化）：
  - 录屏逐帧差分：抖动帧对里左侧大片像素**完全一致**（左对齐内容不动），只有右对齐元素整体平移一个滚动条宽度（经典滚动条约 15px），同时右 / 下边缘各出现一条滚动条滑块（溢出只有几像素时滑块几乎占满轨道）——这组合是「视口被滚动条吃掉一截」的指纹，区别于应用自身动画（后者不会让左侧像素逐位相同）；
  - 页面里直接测 `innerWidth - document.documentElement.clientWidth`（>0 即滚动条占了宽度）与 `scrollWidth/scrollHeight` 是否超出 client 尺寸。
- **不要**再靠 `scrollbar-gutter` 或应用侧 `Scrollbar` 配置绕：`<html>` 是 `visible` 时前者无效（同第 2 层的坑），后者根本不是这条回路的变量。

---

## 三、Web 插件 iframe 被语义节点遮挡（无法点击）

Web 端播放栏出现后，`flt-semantics` 节点（`pointer-events:auto`）盖住插件 iframe 抢走点击。

- **根因**：`main.dart` 在 Web 强制常驻语义树（`ensureSemantics`，无障碍改进）命中 Flutter 引擎残留 bug [flutter/flutter#175119]：`ensureSemantics` + go_router + 平台视图 iframe，关闭带 barrier 的对话框后语义节点卡在 `pointer-events:auto` 叠在平台视图之上。
- **修复（方案 A）**：进入插件 iframe 页时临时释放语义句柄关闭语义树、离开时恢复。`lib/core/a11y/web_semantics_controller.dart` 单例持 `SemanticsHandle`；`shell_layout.dart` 按 `isPluginTab` 边沿 suspend/resume。读屏器激活时平台持独立句柄，释放我们的句柄不关语义树，AT 用户不受损。

### 常驻语义树只在桌面 Web（移动浏览器会让软键盘弹不出来）

iOS Safari 打开 Web 端，点登录页用户名/密码框**不弹输入法**；切到桌面再切回 Safari 才能输入，且此时页面被放大只能盲输（songloft-org/songloft-player-flutter#26）。

- **根因**：引擎 `HybridTextEditing.strategy` 是 `late final`，**首次用到文本输入时**按当时的 `semanticsEnabled` 一次性定型。启动即 `ensureSemantics()` 会把整个会话钉死在 `SemanticsTextEditingStrategy`，从此绕开 `IOSTextEditingStrategy` —— 后者才带 iOS 的全部绕法（聚焦前先移出屏外、100ms 后再定位、tap 监听），以及 iOS 26「自动填充前先 blur 输入框」的补救（该补救只监听 `flt-text-editing-host` 的 focusin，语义模式下输入框在 `flt-semantics-host` 里，够不着）。上游长期未修：[flutter/flutter#129324]（open，3.41 仍可复现）、[#123338]、[#141975]、[#154741]，症状均为「语义模式 + 移动浏览器 → 键盘不弹或弹起立刻收起」，且与 `InputDecoration`（label/hint/suffixIcon）+ 滚动容器组合相关，正是登录页的形状。
- **修复**：`WebSemanticsController.enableByDefault()` 在移动端浏览器（`defaultTargetPlatform` 为 iOS / Android）直接跳过，只有桌面 Web 常驻。移动端回落到引擎默认的按需启用（`MobileSemanticsEnabler` 检测读屏器手势后自动开语义树），AT 用户仍可用。
- **顺带修掉放大**：语义模式下的输入框 `font-size: 13.33px` 且宿主被 `scale(1/dPR)`，iOS 聚焦时会自动放大页面；非语义路径的隐藏输入框是 `font-size: 16px`（正好是 iOS 不放大的阈值）。
- **验证口径**：headless Chrome 里 `navigator.platform` 才决定引擎的 OS 判定，**只改 UA 无效**，需 `evaluateOnNewDocument` 覆写 `platform: 'iPhone'` + `maxTouchPoints > 2`。判据：移动 UA 下 `flt-semantics` 节点数为 0 且出现 `flt-semantics-placeholder`，点击输入框后 `flt-text-editing-host` 内出现聚焦的 `<input>`；桌面 UA 下语义节点照旧存在。

---

## 四、Web 移动端切后台回来黑屏（现状：接受，待引擎修复）

Android Chrome 切后台回来黑屏。

- **根因**：后台时浏览器丢弃 CanvasKit 的 WebGL context，引擎置 `_forceNewContext` 后**被动等 `webglcontextrestored`**——Android Chrome 常不 fire 该事件。更深一层是引擎 bug [flutter/flutter#184683]：新 surface 架构里 `onContextLost` 在 `late` 字段赋值前 fire → `LateInitializationError` → 渲染帧崩 → 白屏。修复 PR [#185116] 至今未进最新 stable。
- **现状决策**：全平台（含 Web）统一 stable Flutter 3.44.6、保持 GPU 渲染；3.44.6 不含 #185116，移动端切后台白屏**按现状接受**，待修复进 stable 后升级根治。
- **不要再试**的方向：源码 backport 无效（web 引擎预编进 `.dill`，release dart2js 不读 `lib/_engine` 源码）；CDN vs 离线 wasm 与黑屏无关（只改初始加载来源，不改 GPU context 生命周期）；纯 JS 侧无法强制 Flutter 产帧，恢复必须在 Dart 侧。

---

## 五、音频播放后端

### 所有原生平台统一 media_kit/libmpv，无 kill-switch

- 所有原生平台（Win/Linux/macOS/Android/iOS）音频后端恒用 media_kit（libmpv），已**删除**原生 ExoPlayer/AVPlayer 回退与 `SONGLOFT_MEDIAKIT_*` 开关（`AudioBackend.usesMediaKit => !kIsWeb`）。EQ 一律走 mpv `af`（`MpvEqualizerService`）。
- ⚠️ 已无 kill-switch，某平台 media_kit 出问题时**不能再靠 `--dart-define` 回退，需改代码**。
- 历史坑（`SongloftMediaKitPlayer` 实现要点）：
  - 必须重写 `setAndroidAudioAttributes` 为安全 no-op——基类默认 throw，just_audio 仅 Android 调它，否则 Android 全曲在 `setAudioSource` 阶段就炸（`UnimplementedError`）。
  - 移动端构造时**不要预建** `VideoController`——预建会让每个 `open()/play()/seek()` 都 `await` 视频纹理就绪，Android 无 Video widget 时该 Future 永不完成 → 全曲挂住。改为判定视频源时才惰性建。

### 自签 / 不完整链 HTTPS 播放：`AudioSource.uri(headers=null)` 会绕过本地代理

- **根因**：SSL 忽略只做在 Dart 层（`HttpOverrides` trust-all + mpv `tls-verify`）。但 `AudioSource.uri` 传 `headers=null` 时 just_audio **跳过本地明文回环代理**，把 URL 直接交给原生播放器，其 TLS 握手在 `dart:io` 之外，`HttpOverrides` 管不到 → 自签失败。（just_audio 仅当 `headers!=null || userAgent!=null` 才 engage 本地代理。）
- **修复**：`insecureTls==true` 时给 `AudioSource.uri` 附非空 header，强制走本地代理——原生只连 `127.0.0.1` 明文，上游 HTTPS 由 just_audio 的 Dart `HttpClient`（trust-all）拉取。移动端 HLS 自签需自研 HLS-aware trust-all 本地代理（`lib/core/network/insecure_media_proxy_native.dart`）递归改写 m3u8 子资源。

### 桌面端 HLS 电台失败

- **HLS 直播切片过期**：桌面端 mpv 无 CORS，但用户全局开着 HLS 代理时，带时间戳的短窗口切片经本机反代往返后已滚出窗口 → `404`。修复：后端 `serveRadio` 支持 `?hls=direct` 强制 302 绕过反代，前端桌面 live 播放追加该参数（桌面自带 Referer/UA）；移动端不加（其原生 player 不发 Referer/UA，保留反代）。
- **HE-AAC 客户端解码崩**：`streamtheworld` 等 HE-AAC 电台 web+mpv 都播 ~3.7s 后 `aac: decode_band_types: Input buffer exhausted` 崩。**已实测坐实后端无责**（ffmpeg 直连 + 复刻代理都干净解码），真因是客户端解码器扛不住 HE-AAC。部分缓解：后端 `audio/aacp`→`audio/aac`、mpv 加 `network-timeout`；彻底解决需后端转码兜底（未做）。

### Windows 插件页 WebView2 需显式初始化

- Windows 免安装版从不初始化 `WebViewEnvironment`，默认用户数据目录落在只读目录 → `Cannot create the InAppWebView instance!`，且实例创建失败时 controller 恒 null、`reload()` 是 no-op 永不自愈。
- 修复：`core/utils/webview_environment.dart` 用 `getApplicationSupportDirectory()/webview2` 作可写 `userDataFolder` 建全局单例，两处 `InAppWebView` 都传入，重试改为换 `ValueKey` 重建部件。
- 相关：退出前要硬杀进程（`TerminateProcess`）的清理里，**托盘图标/资源移除必须排在硬杀调用之前**，否则 `destroy()` 成死代码（硬杀永不返回）。

---

## 六、分页列表点击单曲播放（铁律）

### 铁律：任何分页列表「点击单曲播放」都要 `playPlaylist(已加载, startIndex)` + 按同筛选后台补齐

- **坑**：分页列表（歌单 >100 首、分类页等）直接 `playPlaylist(songs, startIndex)`，`songs` 只是当前已加载页 → 播放队列被截断到已加载数（如最长 100），只在其中循环。
- **修复**：用 `PlayerNotifier.playPlaylistFromLoaded`——以已加载分页 + startIndex 立即播放，`total > 已加载数` 时按相同 sort/order/keyword 后台补齐（复用 `_loadRemainingSongsById`）。
- **新增任何「分页列表点单曲」页面**务必带补齐逻辑；分类/facet 过滤页要保证 field→getSongs 参数映射一致（`categorySongsFilter`）。

---

<a id="drive-flutter-web"></a>

## 七、驱动 Flutter Web（无头浏览器验证）

用 Docker 无头 Chrome 做真实 UI 验证的完整流程见 [AI 开发流程 · 前端 UI 验证](ai-development.md#frontend-ui-verification)。驱动 Flutter Web 时注意：

- **Flutter Web 是 canvas 渲染，DOM 里没有按钮**。可交互元素只以 `<flt-semantics>` 无障碍节点暴露：用 `getBoundingClientRect()` 取中心点后 `page.mouse.click(x, y)`；首屏出现 `<flt-semantics-placeholder>` 时先点击它
- **语义 label 会被合并**：一屏内容常聚合成一个长 aria-label 节点，按钮文字未必是独立节点。找不到时先截图看坐标再点，不要在 label 匹配上死磕
- **文本框**：`<input aria-label="Username">` 是真实 DOM，但坐标点击 + `keyboard.type` 更稳；点击与输入之间留 ~800ms，等 Flutter 建立焦点，否则输入会丢
- **验证双语**：用 `evaluateOnNewDocument` 覆写 `navigator.language`/`languages` 为 `zh-CN`，Flutter l10n 会跟着切
- **深链**：`page.goto('/settings/category/2')` 是整页刷新，会在启动中途产生假的 "Failed to load"；用应用内点击导航，不要用 goto 跳嵌套路由

---

## 相关模块参考

- 插件公共资源与主题桥接：`internal/jsplugin/assets/`（`theme.css` / `components.css` / `common.js`）、`injectHTMLHead`
- 播放活动 / 预热转码：预热（prefetch）转码不应被切当前歌的 `playactivity.Activate` 取消（`Activate` 跳过 `CatPrefetch`），否则下一首预热 ffmpeg 被 SIGKILL、播放仍实时转码。
- WebDAV 等无时长源：`song.duration=0` 会导致 miot 音箱不切歌（切歌依赖服务端 duration），导入时应探测补齐（`AddRemoteSongs` 后台限并发 `RefreshSong`）。
