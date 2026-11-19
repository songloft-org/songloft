# Songloft Frontend Gotchas

This document captures pitfalls in the Flutter client (`songloft-player-flutter`) that have been **diagnosed and fixed, but whose root causes are broadly applicable** — mostly in Flutter Web (CanvasKit) rendering, platform views (plugin iframes), and the audio backend. These burned through many rounds of investigation; archiving them here prevents repeating that.

> Backend/business-side pitfalls are in [Business Constraints and Pitfalls](pitfalls.md); see the repository-root `AGENTS.en.md` for entry-point routing.

---

## 1. Cover / image rendering (hard rule)

### Rule: all covers must go through `CoverImage` / `NetworkCoverImage`; never use bare `CachedNetworkImage` / `Image.network`

- Unified wrappers: `lib/shared/widgets/cover_image.dart` (`CoverImage`) and `lib/shared/widgets/network_cover_image.dart` (`NetworkCoverImage`). On Web they force `imageRenderMethodForWeb: HttpGet` + `memCacheWidth` (decode a thumbnail scaled to the displayed physical pixels).
- **Every new cover render site** must use one of these two components; don't write `CachedNetworkImage(...)` or `Image.network(...)` directly.

### Why (the Web CanvasKit GPU-memory trap)

On Web, covers occasionally turn **solid black** (pure black, not the load-failure placeholder icon) or degrade to a placeholder icon, triggered by **in-app navigation / tab switch / filter and back**:

- A bare `CachedNetworkImage` on Web defaults to `ImageRenderMethodForWeb.HtmlImage`, uploading the image at **full resolution** (up to ~1.5 MB each) as a GPU texture.
- CanvasKit has a single WebGL context (Chrome uses `OffscreenCanvasRasterizer`). Platform views like plugin iframes / video each consume GPU resources; **full-resolution cover textures accumulate → exhaust GPU memory → the WebGL context is dropped** (`ImageCodecException: Failed to create image` / `MakeLazyImageFromTextureSourceWithInfo` returns null). After the context dies, reused `ui.Image` textures paint black; new decodes during the dead window can't get a texture and degrade to the placeholder icon.
- `memCacheWidth` **only takes effect on the `HttpGet` path**; on the default `HtmlImage` path it does not downscale — so you must explicitly set `imageRenderMethodForWeb: HttpGet`, shrinking each cover from ~1.5 MB to tens/hundreds of KB and drastically reducing memory pressure.

### Investigation lessons (avoid the 9-round detour)

- **Pure black ≠ load failure**: pure black is at the GPU texture layer and `errorWidget` can't catch it; the placeholder icon is a load/decode failure. Distinguish first.
- `imageCache.evict` / changing the key / attaching a `NavigatorObserver` to force rebuilds are **only probabilistic mitigations** and can even worsen the re-decode storm. GPU texture lifetime issues are only truly fixed by: shrinking decode size (`HttpGet`+`memCacheWidth`), reducing concurrent platform views, or upgrading the engine.
- `canvasKitForceCpuOnly` (CPU software rendering) removes GPU-texture black but is **very laggy** and covers still get lost at the loading layer — net negative, ruled out.
- To fix a class of image problem, **grep all render sites** (`CachedNetworkImage` / `Image.network`); don't assume fixing one shared component covers everything — cover rendering is spread across `CoverImage` plus several card components.
- The decisive breakthrough came from **having the user paste real-device console logs** (the exact error string + `[Cover] OK yet renders black` + cache not evicted), not from many rounds of pure inference.

---

## 2. Web plugin-tab iframe reloading / jitter

The `<iframe>` (`HtmlElementView` platform view) hosting a plugin page suffered repeated reloads (the entry page requested 25–40 times/sec) and visual jitter. Fixed; there were four layers of root cause (layer 4 is not limited to plugin pages — **the whole app shakes**):

### 1. Widget tree: iframe reload ⟺ its hosting widget element is disposed+rebuilt

On Flutter Web the iframe DOM element is cached once per platform view `viewId` and projected through a shadow-DOM `<slot>` — a normal rebuild / relocation **won't** touch the iframe; only **destroying and rebuilding the viewId** re-fetches it.

- **Rule**: in the plugin keep-alive Stack (`shell_layout.dart`), anything that disposes+rebuilds the `PluginTabPage` element triggers an iframe reload — reordering, reparenting (a local `ValueKey` can't move across parents), or a provider that momentarily goes empty dropping the active tab.
- Fix: (1) `PluginTabPage` uses a **stable `GlobalKey` cached by entryPath** (moves across the tree instead of rebuilding); (2) keep-alive pruning keys only off the stable `tabConfig.pluginTabs` and only when `tabConfigAsync.hasValue`, never off the momentarily-empty `jsPluginsProvider` snapshot. Caching the iframe inside the factory is **useless** (when the engine rebuilds the viewId it moves the iframe to a new wrapper, and the move still forces a browser reload).

### 2. CSS: the document-level scrollbar add/remove reflow loop

When content height ≈ viewport, "scrollbar appears → content narrows → reflow → height changes → scrollbar disappears → …" flips every frame.

- Fix: in `internal/jsplugin/assets/theme.css`, give the **embedded** case (`html.embed`) `overflow-y: scroll` (scrollbar always present, constant width, loop broken), plus `scrollbar-width: none` + `::-webkit-scrollbar { display: none }` so it takes no layout width (otherwise `100vw`-sized elements inside plugins overflow horizontally, #341). Only forced in the embedded case: a plugin page opened directly in a browser can be resized freely and has no such loop.
- Note: `scrollbar-gutter: stable` **only works on scroll containers (`overflow:auto/scroll`)**; it's a no-op when `html` is `visible` — which is why an earlier attempt didn't fix it. Linux/headless with overlay (0-width) scrollbars can never reproduce it; only Windows Chrome (classic ~15px scrollbar) does.

### 3. Cache layer (hard rule): immutable long-cache assets must use versioned URLs

Even after the CSS was correct, users still saw jitter — the real culprit was cache headers. `jsplugin-assets/*` (`common.css`/`common.js` at the time, now split into `theme.css` / `components.css` / `common.js`) previously used a **fixed, unversioned URL** + `Cache-Control: immutable`, so browsers didn't even revalidate and cached the old file for a year; the fix could never reach users.

- **General rule**: any `immutable` long-cache asset **must use a content-hash versioned URL** (e.g. `?v=<first 8 of sha256>`), or any later change is unreachable for existing users.
- Implementation: `injectHTMLHead` appends `?v=<hash>` to every injected common-asset URL (`assetVersions` in `internal/jsplugin/routes.go`); the hosting HTML is `no-cache` so it always carries the latest version. When content is unchanged the URL is stable and the long cache still applies. Any new common asset must be registered in `assetVersions` too.

### 4. Host page (hard rule): `<html>` must have viewport scrollbars disabled, or the whole page shakes (#439)

Layer 2 only fixed the plugin iframe's **own document**; the exact same loop also applies to the **Flutter host page** — symptom: home / library / plugin store / plugin pages **all shake**, with no plugin involved.

- **Mechanism**: the engine's `FullPageEmbeddingStrategy` only styles `<body>` (`position:fixed; inset:0; overflow:hidden`) and leaves **`<html>` at the default `overflow:visible`**. As soon as anything overflows the document by a few pixels (e.g. an overlay a browser extension injects into `<html>` — our own DOM all lives inside the fixed body and never contributes), a classic scrollbar appears and eats 15px of the viewport → the viewport-sized body narrows → Flutter relayouts the whole app → the overflow is gone → the scrollbar is removed → flip every frame, i.e. a ~15Hz whole-page shake.
- **Fix**: `clients/player-flutter/web/index.html` sets `html { overflow: hidden; scrollbar-width: none }` + `html::-webkit-scrollbar { display: none }`. The host page never needs document-level scrolling (the Flutter view is a fixed full-screen element; all scrolling happens inside the app), so disabling it makes the loop structurally impossible — no need to first identify what overflowed the document.
- **How to diagnose** (the visual shake cannot be reproduced where scrollbars are overlay-style, but the mechanism is measurable):
  - Frame-diff a screen recording: in a shaking frame pair a large left-hand region is **pixel-identical** (left-aligned content doesn't move) while right-aligned elements translate by exactly one scrollbar width (~15px for a classic scrollbar), and a scrollbar thumb appears along the right and bottom edges (when the overflow is only a few pixels the thumb nearly fills the track). That combination is the fingerprint of "the viewport lost a slice to a scrollbar", unlike an app animation (which wouldn't leave the left side bit-identical);
  - Measure in-page: `innerWidth - document.documentElement.clientWidth` (>0 means a scrollbar is taking width) and whether `scrollWidth/scrollHeight` exceed the client sizes.
- **Don't** try to work around it with `scrollbar-gutter` or app-side `Scrollbar` config: the former is a no-op while `<html>` is `visible` (same trap as layer 2), and the latter isn't a variable in this loop at all.

---

## 3. Web plugin iframe blocked by semantics nodes (unclickable)

After the Web player bar appears, `flt-semantics` nodes (`pointer-events:auto`) cover the plugin iframe and steal clicks.

- **Root cause**: `main.dart` forces a persistent semantics tree on Web (`ensureSemantics`, an accessibility improvement) and hits a lingering engine bug [flutter/flutter#175119]: `ensureSemantics` + go_router + platform-view iframe — after closing a dialog with a barrier, a semantics node stays at `pointer-events:auto` layered over the platform view.
- **Fix (option A)**: temporarily release the semantics handle to close the semantics tree when entering a plugin iframe page, and restore it on leave. `lib/core/a11y/web_semantics_controller.dart` is a singleton holding a `SemanticsHandle`; `shell_layout.dart` suspends/resumes on the `isPluginTab` edge. When a screen reader is active the platform holds its own handle, so releasing ours doesn't close the tree — AT users are unaffected.

### The persistent semantics tree is desktop-Web only (on mobile browsers it kills the soft keyboard)

On iOS Safari, tapping the username/password field on the login page **does not bring up the keyboard**; only after switching to the home screen and back to Safari can you type, and by then the page is zoomed in so you have to type blind (songloft-org/songloft-player-flutter#26).

- **Root cause**: the engine's `HybridTextEditing.strategy` is `late final` — it is fixed **once, the first time text editing is used**, based on `semanticsEnabled` at that moment. Calling `ensureSemantics()` at startup pins the whole session to `SemanticsTextEditingStrategy`, bypassing `IOSTextEditingStrategy` — the only one carrying the iOS workarounds (move offscreen before focusing, place 100 ms later, tap listener) plus the iOS 26 "field is blurred right before autofill" mitigation (which only listens for `focusin` on `flt-text-editing-host`, while in semantics mode the input lives in `flt-semantics-host` and is out of reach). Long-standing upstream bugs: [flutter/flutter#129324] (open, still reproducible on 3.41), [#123338], [#141975], [#154741] — all "semantics mode + mobile browser → keyboard never shows or closes immediately", and all tied to `InputDecoration` (label/hint/suffixIcon) plus a scroll container, which is exactly the shape of the login page.
- **Fix**: `WebSemanticsController.enableByDefault()` skips mobile browsers (`defaultTargetPlatform` of iOS / Android); only desktop Web keeps the tree resident. Mobile falls back to the engine's on-demand path (`MobileSemanticsEnabler` enables semantics once it detects the screen-reader gesture), so AT users are still served.
- **Zoom fixed as a side effect**: in semantics mode the input has `font-size: 13.33px` inside a host scaled by `scale(1/dPR)`, so iOS zooms the page on focus; the non-semantics hidden input is `font-size: 16px` — exactly the threshold at which iOS does not zoom.
- **How to verify**: in headless Chrome the engine's OS detection reads `navigator.platform`, so **changing the UA alone does nothing** — use `evaluateOnNewDocument` to override `platform: 'iPhone'` and `maxTouchPoints > 2`. Criteria: with the mobile UA there are 0 `flt-semantics` nodes and a `flt-semantics-placeholder` is present, and tapping a field puts a focused `<input>` inside `flt-text-editing-host`; with the desktop UA the semantics nodes are still there.

---

## 4. Web mobile black screen after returning from background (status: accepted, awaiting engine fix)

Android Chrome shows a black screen after returning from background.

- **Root cause**: while backgrounded the browser discards CanvasKit's WebGL context; the engine sets `_forceNewContext` then **passively waits for `webglcontextrestored`** — which Android Chrome often never fires. Deeper still is engine bug [flutter/flutter#184683]: in the new surface architecture `onContextLost` fires before a `late` field is assigned → `LateInitializationError` → render frame crashes → white screen. The fix PR [#185116] is not yet in the latest stable.
- **Current decision**: all platforms (including Web) pin to stable Flutter 3.44.6 with GPU rendering; 3.44.6 lacks #185116, so mobile Web background black screen is **accepted as-is**, to be truly fixed by upgrading once the fix lands in stable.
- **Don't retry** these directions: source backport is useless (the web engine is precompiled into `.dill`; release dart2js doesn't read `lib/_engine` sources); CDN vs offline wasm is unrelated to the black screen (it only changes the initial load source, not the GPU context lifetime); pure JS can't force Flutter to produce a frame — recovery must be on the Dart side.

---

## 5. Audio playback backend

### All native platforms unified on media_kit/libmpv, no kill-switch

- All native platforms (Win/Linux/macOS/Android/iOS) always use media_kit (libmpv); the native ExoPlayer/AVPlayer fallback and the `SONGLOFT_MEDIAKIT_*` switch have been **removed** (`AudioBackend.usesMediaKit => !kIsWeb`). EQ always goes through mpv `af` (`MpvEqualizerService`).
- ⚠️ There is no kill-switch anymore; if media_kit breaks on a platform you **can no longer fall back via `--dart-define` — you must change code**.
- Historical pitfalls (`SongloftMediaKitPlayer` implementation notes):
  - Must override `setAndroidAudioAttributes` as a safe no-op — the base class throws by default, just_audio calls it only on Android, otherwise every track on Android crashes right at `setAudioSource` (`UnimplementedError`).
  - On mobile, **do not pre-create** a `VideoController` — pre-creating makes every `open()/play()/seek()` `await` video-texture readiness, and on Android without a Video widget that Future never completes → every track hangs. Build it lazily only when a video source is detected.

### Self-signed / incomplete-chain HTTPS playback: `AudioSource.uri(headers=null)` bypasses the local proxy

- **Root cause**: SSL bypass is only done at the Dart layer (`HttpOverrides` trust-all + mpv `tls-verify`). But when `AudioSource.uri` is passed `headers=null`, just_audio **skips the local plaintext loopback proxy** and hands the URL straight to the native player, whose TLS handshake is outside `dart:io` where `HttpOverrides` can't reach → self-signed fails. (just_audio only engages the local proxy when `headers!=null || userAgent!=null`.)
- **Fix**: when `insecureTls==true`, attach a non-empty header to `AudioSource.uri` to force the local proxy — the native player only connects to `127.0.0.1` in plaintext while upstream HTTPS is fetched by just_audio's Dart `HttpClient` (trust-all). Mobile HLS with self-signed certs needs a custom HLS-aware trust-all local proxy (`lib/core/network/insecure_media_proxy_native.dart`) that recursively rewrites m3u8 sub-resources.

### Desktop HLS radio failures

- **Live HLS segment expiry**: desktop mpv has no CORS, but when the user has the global HLS proxy on, timestamped short-window segments have already rolled out of the window after a round trip through the local proxy → `404`. Fix: the backend `serveRadio` supports `?hls=direct` to force a 302 bypass of the proxy, and the desktop frontend appends it for live playback (desktop carries its own Referer/UA); mobile does not (its native player sends no Referer/UA, so it keeps the proxy).
- **HE-AAC client decode crash**: HE-AAC stations like `streamtheworld` crash after ~3.7s on both Web and mpv with `aac: decode_band_types: Input buffer exhausted`. **Confirmed by testing that the backend is not at fault** (both direct ffmpeg and a faithful proxy replica decode cleanly); the real cause is the client decoder failing on HE-AAC. Partial mitigation: backend maps `audio/aacp`→`audio/aac`, mpv adds `network-timeout`; a full fix needs backend transcoding fallback (not done).

### Windows plugin page requires explicit WebView2 initialization

- The Windows portable build never initializes `WebViewEnvironment`; the default user-data folder lands in a read-only directory → `Cannot create the InAppWebView instance!`, and on failure the controller stays null and `reload()` is a no-op that never self-heals.
- Fix: `core/utils/webview_environment.dart` builds a global singleton using `getApplicationSupportDirectory()/webview2` as a writable `userDataFolder`, passed to both `InAppWebView` sites, with retry changed to rebuild the widget via a new `ValueKey`.
- Related: in cleanup that hard-kills the process before exit (`TerminateProcess`), **tray-icon/resource removal must come before the hard-kill call**, or `destroy()` becomes dead code (the hard-kill never returns).

---

## 6. Playing a single song from a paginated list (hard rule)

### Rule: any "tap a single song to play" in a paginated list must `playPlaylist(loaded, startIndex)` + backfill in the background with the same filter

- **Pitfall**: a paginated list (playlist >100 songs, category pages, etc.) calling `playPlaylist(songs, startIndex)` directly, where `songs` is only the currently-loaded page → the play queue is truncated to the loaded count (e.g. max 100) and only loops within it.
- **Fix**: use `PlayerNotifier.playPlaylistFromLoaded` — play immediately with the loaded page + startIndex, and when `total > loaded count` backfill in the background with the same sort/order/keyword (reusing `_loadRemainingSongsById`).
- **Any new "tap single song in a paginated list" page** must include the backfill logic; category/facet filter pages must keep the field→getSongs parameter mapping consistent (`categorySongsFilter`).

---

<a id="drive-flutter-web"></a>

## 7. Driving Flutter Web (headless verification)

The complete Docker headless Chrome workflow is in [AI Development · Frontend UI Verification](ai-development.md#frontend-ui-verification). When driving Flutter Web:

- **Flutter Web renders to a canvas, so the DOM has no buttons.** Interactive elements are exposed only as `<flt-semantics>` accessibility nodes: read `getBoundingClientRect()`, then `page.mouse.click(x, y)` on the center. If `<flt-semantics-placeholder>` appears on first paint, click it first
- **Semantics labels get merged**: a whole screen often collapses into one long aria-label node, and a button's text is not necessarily its own node. When you can't find it, screenshot first and click by coordinates instead of fighting label matching
- **Text fields**: `<input aria-label="Username">` is real DOM, but clicking by coordinates plus `keyboard.type` is more reliable; leave ~800ms between the click and the typing so Flutter establishes focus, or the input is dropped
- **Verifying both languages**: override `navigator.language`/`languages` to `zh-CN` via `evaluateOnNewDocument`; Flutter l10n follows
- **Deep links**: `page.goto('/settings/category/2')` is a full reload and can produce a bogus "Failed to load" mid-boot. Navigate by clicking inside the app instead of using goto for nested routes

---

## Related module references

- Plugin common assets and theme bridge: `internal/jsplugin/assets/` (`theme.css` / `components.css` / `common.js`), `injectHTMLHead`.
- Play activity / prefetch transcoding: prefetch transcoding must not be canceled by activating the current song via `playactivity.Activate` (`Activate` skips `CatPrefetch`); otherwise the next track's prefetch ffmpeg is SIGKILLed and playback still transcodes in real time.
- Sources without duration (e.g. WebDAV): `song.duration=0` makes miot speakers not advance tracks (advancing relies on server-side duration); probe and backfill on import (`RefreshSong` with bounded concurrency after `AddRemoteSongs`).
