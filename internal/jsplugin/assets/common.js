/**
 * Songloft Plugin Common JS — 由主程序自动注入到所有插件 HTML 页面
 * 职责：embed 检测、主题桥接、API 工具、宿主桥接、a11y（window.SongloftPlugin）
 */
(function() {
    'use strict';

    // ── Embed 检测 ──
    if (new URLSearchParams(window.location.search).has('embed')) {
        document.documentElement.classList.add('embed');
    }

    // ── 主题桥接 ──
    var params = new URLSearchParams(window.location.search);
    var initialTheme = params.get('theme') || localStorage.getItem('songloft-theme') || 'light';

    function applyTheme(th) {
        var d = document.documentElement;
        // 用 setAttribute 而不是 d.dataset.theme：阻塞脚本执行期 dataset 的可用性
        // 没有跨环境契约，赋值一旦抛 TypeError 会**中断整个 IIFE** ——
        // window.SongloftPlugin 压根不会
        // 定义，宿主桥连带全废（songloft-org/songloft#341 实测）。
        // setAttribute 语义等价，且不依赖 dataset 何时就绪。
        d.setAttribute('data-theme', th);
        d.classList.remove('theme-light', 'theme-dark');
        d.classList.add('theme-' + th);
        localStorage.setItem('songloft-theme', th);
        document.dispatchEvent(new CustomEvent('songloft-theme-change', { detail: { theme: th } }));
    }

    // 刻意包 try/catch：本文件是一个 IIFE，任何早期异常都会中断其余全部代码，
    // 包括最后那段 window.SongloftPlugin 的定义 —— 表现是宿主桥整体静默失效，极难归因。
    // 主题失效只是外观问题，不该连带打掉插件的宿主能力。
    try {
        applyTheme(initialTheme);
    } catch (e) {
        console.warn('[songloft] applyTheme failed, continuing:', e);
    }

    if (params.has('theme')) {
        params.delete('theme');
        var cleanUrl = window.location.pathname;
        var remaining = params.toString();
        if (remaining) cleanUrl += '?' + remaining;
        history.replaceState(null, '', cleanUrl);
    }

    // ── 宿主真实色板（songloft-org/songloft#341）───────────────────────────
    //
    // theme.css 的 `--md-*` 只是**静态兜底**（由默认 seed 导出）。宿主在页面就绪后
    // 随 `songloft-theme` 消息把**真实的** ColorScheme 推来（含用户自定义 ThemePack），
    // 这里写成 documentElement 的**内联**自定义属性覆盖兜底值 —— 内联优先级最高，
    // 连插件自己在 `:root` 里重定义的同名变量也压得住。Dart 侧见
    // `clients/player-flutter/lib/features/home/presentation/render/plugin_color_scheme.dart`。
    //
    // **为什么颜色挂在 `songloft-theme` 上而不单开一条消息**：亮暗标记（`data-theme`）
    // 与色值必须**同时**生效。分两条消息就会有一帧「`data-theme=dark` 但变量还是
    // 亮色」的错色闪烁。
    //
    // ⚠️ 插件想用 **JS** 读色必须调 `SongloftPlugin.getColorScheme()`，不能读 CSS：
    // `getComputedStyle` 对自定义属性不一定返回解析值，而宿主原生控件
    // 的属性值也不展开 `var()`、只吃字面 hex。
    //
    // **色板必须像 `songloft-theme`（亮暗）一样持久化到 localStorage 并在加载时恢复**
    // （songloft-org/songloft#341）。宿主的色板下推是一条 **fire-and-forget** 的
    // `window.postMessage`，没有回执；页面加载早于本脚本注册 `message` 监听器时，
    // 推送会被静默丢弃，页面就会停在 `theme.css` 的**默认 seed 静态兜底色**，
    // 而不是用户自定义 ThemePack 的颜色。
    //
    // 恢复上一次的色板让页面**自给自足**、不再依赖推送时序：宿主推送退化为「运行中
    // 切主题」时的更新（去重后按需重推），加载首帧则直接用持久化的真实色板 ——
    // 对自定义主题包用户，这也顺带修掉了「首帧闪一下默认色再跳到自定义色」。
    var COLOR_SCHEME_STORAGE_KEY = 'songloft-color-scheme';

    function readPersistedColorScheme() {
        try {
            var raw = localStorage.getItem(COLOR_SCHEME_STORAGE_KEY);
            if (!raw) return null;
            var parsed = JSON.parse(raw);
            if (parsed && typeof parsed === 'object') return parsed;
        } catch (e) {
            // 损坏 / 不可用：当作没有持久化色板，回退到 theme.css 兜底 + 等宿主推送。
        }
        return null;
    }

    var lastColorScheme = readPersistedColorScheme();
    var HEX_RE = /^#[0-9a-fA-F]{6}$/;
    var RGBA_RE = /^rgba\(\s*([0-9.]+)\s*,\s*([0-9.]+)\s*,\s*([0-9.]+)\s*,\s*([0-9.]+)\s*\)$/;

    // ── 宿主主题外观参数 ──────────────────────────────────────────────────
    // `ColorScheme` 只能描述颜色，无法表达主题包的标准/胶囊导航样式。将这些
    // 非颜色视觉参数随同一条 `songloft-theme` 消息下推并持久化，插件首帧即可
    // 使用与主程序一致的迷你播放器形态。
    var THEME_APPEARANCE_STORAGE_KEY = 'songloft-theme-appearance';
    var DEFAULT_THEME_APPEARANCE = {
        navigationStyle: 'standard',
        cardRadius: 12,
        controlRadius: 12,
        navigationRadius: 12,
        playerGradient: [],
        glassFill: null,
        glassBorder: null,
        reduceTransparency: null,
        increaseContrast: null
    };

    function normalizeRadius(value, fallback) {
        var n = Number(value);
        return Number.isFinite(n) ? Math.max(0, Math.min(64, n)) : fallback;
    }

    function normalizeOptionalColor(value) {
        if (typeof value !== 'string') return null;
        var match = value.match(RGBA_RE);
        if (!match) return null;
        var r = Number(match[1]);
        var g = Number(match[2]);
        var b = Number(match[3]);
        var a = Number(match[4]);
        if (![r, g, b, a].every(Number.isFinite)) return null;
        if (r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255) return null;
        if (a < 0 || a > 1) return null;
        return 'rgba(' + r + ', ' + g + ', ' + b + ', ' + a + ')';
    }

    function normalizeThemeAppearance(value) {
        var source = value && typeof value === 'object' ? value : {};
        var gradient = Array.isArray(source.playerGradient)
            ? source.playerGradient.filter(function(color) {
                return typeof color === 'string' && HEX_RE.test(color);
            }).slice(0, 8)
            : [];
        if (gradient.length === 1) gradient.push(gradient[0]);
        return {
            navigationStyle: source.navigationStyle === 'capsule' ? 'capsule' : 'standard',
            cardRadius: normalizeRadius(source.cardRadius, DEFAULT_THEME_APPEARANCE.cardRadius),
            controlRadius: normalizeRadius(source.controlRadius, DEFAULT_THEME_APPEARANCE.controlRadius),
            navigationRadius: normalizeRadius(source.navigationRadius, DEFAULT_THEME_APPEARANCE.navigationRadius),
            playerGradient: gradient,
            glassFill: normalizeOptionalColor(source.glassFill),
            glassBorder: normalizeOptionalColor(source.glassBorder),
            reduceTransparency: typeof source.reduceTransparency === 'boolean' ? source.reduceTransparency : null,
            increaseContrast: typeof source.increaseContrast === 'boolean' ? source.increaseContrast : null
        };
    }

    function readPersistedThemeAppearance() {
        try {
            var raw = localStorage.getItem(THEME_APPEARANCE_STORAGE_KEY);
            var restored = raw ? normalizeThemeAppearance(JSON.parse(raw)) : DEFAULT_THEME_APPEARANCE;
            // Accessibility belongs to the current host session, never a cached host.
            restored.reduceTransparency = null;
            restored.increaseContrast = null;
            return restored;
        } catch (e) {
            return DEFAULT_THEME_APPEARANCE;
        }
    }

    var lastThemeAppearance = readPersistedThemeAppearance();

    // camelCase → `--md-kebab-case`。key 用 Flutter `ColorScheme` 的字段名，
    // 好让两个仓库的表能逐字段对照审计，不必另记一套映射。
    function cssVarName(key) {
        return '--md-' + key.replace(/[A-Z]/g, function (m) { return '-' + m.toLowerCase(); });
    }

    // 兼容别名：`--md-surface-variant` 不是 `ColorScheme` 的字段，但组件在用
    // （switch 轨道 / progress 底）。不一起写就会出现「一半变量跟随主题、一半停在
    // 静态兜底值」的割裂配色。
    var COLOR_ALIASES = {
        '--md-surface-variant': 'surfaceContainerHighest'
    };

    function applyColorScheme(colors) {
        if (!colors || typeof colors !== 'object') return;
        lastColorScheme = colors;
        // 持久化，供下次全新加载时同步恢复（见 lastColorScheme 声明处的注释）。
        // 失败只吞掉：localStorage 满 / 隐私模式下写不进不该连带打掉当次的色板落地。
        try {
            localStorage.setItem(COLOR_SCHEME_STORAGE_KEY, JSON.stringify(colors));
        } catch (e) {
            // ignore
        }
        var de = document.documentElement;
        // 特性探测而不是假定：setProperty 拿不到就静默留给 ready 相重试。
        if (!de || !de.style || typeof de.style.setProperty !== 'function') return;
        var key;
        for (key in colors) {
            if (!Object.prototype.hasOwnProperty.call(colors, key)) continue;
            var v = colors[key];
            // 只接受 `#RRGGBB`。非法值一律跳过而不是写进去 —— 免得把坏值盖在
            // theme.css 的合法兜底值上。
            if (typeof v !== 'string' || !HEX_RE.test(v)) continue;
            de.style.setProperty(cssVarName(key), v);
        }
        for (key in COLOR_ALIASES) {
            if (!Object.prototype.hasOwnProperty.call(COLOR_ALIASES, key)) continue;
            var src = colors[COLOR_ALIASES[key]];
            if (typeof src !== 'string' || !HEX_RE.test(src)) continue;
            de.style.setProperty(key, src);
        }
        // 必须在派发事件**之前**：插件的监听器可能会去量元素（如按新底色重算控件配色），
        // 那时布局/样式应当已经是新的。
        document.dispatchEvent(new CustomEvent('songloft-color-scheme-change', {
            detail: { colors: colors }
        }));
    }

    function applyThemeAppearance(appearance) {
        var normalized = normalizeThemeAppearance(appearance);
        lastThemeAppearance = normalized;
        try {
            var persisted = Object.assign({}, normalized);
            delete persisted.reduceTransparency;
            delete persisted.increaseContrast;
            localStorage.setItem(THEME_APPEARANCE_STORAGE_KEY, JSON.stringify(persisted));
        } catch (e) {
            // ignore
        }

        var de = document.documentElement;
        if (!de || !de.style || typeof de.style.setProperty !== 'function') return;
        ['reduceTransparency', 'increaseContrast'].forEach(function(key) {
            var attribute = key === 'reduceTransparency' ? 'data-reduce-transparency' : 'data-increase-contrast';
            if (typeof normalized[key] === 'boolean') {
                de.setAttribute(attribute, String(normalized[key]));
            } else {
                de.removeAttribute(attribute);
            }
        });
        de.setAttribute('data-navigation-style', normalized.navigationStyle);
        de.style.setProperty('--sl-theme-card-radius', normalized.cardRadius + 'px');
        de.style.setProperty('--sl-theme-control-radius', normalized.controlRadius + 'px');
        de.style.setProperty('--sl-theme-navigation-radius', normalized.navigationRadius + 'px');
        if (normalized.playerGradient.length > 0) {
            de.style.setProperty(
                '--sl-theme-player-gradient',
                'linear-gradient(180deg, ' + normalized.playerGradient.join(', ') + ')'
            );
            de.setAttribute('data-player-gradient', 'custom');
        } else {
            de.style.removeProperty('--sl-theme-player-gradient');
            de.removeAttribute('data-player-gradient');
        }
        if (typeof normalized.glassFill === 'string') {
            de.style.setProperty('--sl-theme-glass-fill', normalized.glassFill);
        } else {
            de.style.removeProperty('--sl-theme-glass-fill');
        }
        if (typeof normalized.glassBorder === 'string') {
            de.style.setProperty('--sl-theme-glass-border', normalized.glassBorder);
        } else {
            de.style.removeProperty('--sl-theme-glass-border');
        }
        document.dispatchEvent(new CustomEvent('songloft-theme-appearance-change', {
            detail: { appearance: normalized }
        }));
    }

    // 色板与主题外观的 ready 相补写。
    function applyThemeDetailsOnReady() {
        if (lastColorScheme) applyColorScheme(lastColorScheme);
        applyThemeAppearance(lastThemeAppearance);
    }
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', applyThemeDetailsOnReady);
    } else {
        applyThemeDetailsOnReady();
    }

    // ── 宿主消息通道 ──
    window.addEventListener('message', function(e) {
        if (!e.data || !e.data.type) return;
        if (e.data.type === 'songloft-theme' && (e.data.theme === 'light' || e.data.theme === 'dark')) {
            // ⚠️ 顺序不能反：色板必须先落地，再 applyTheme。插件是靠
            // `songloft-theme-change` 事件重算原生控件颜色的（如下载器插件的
            // SlSwitch 要把主色算成字面 hex 喂给 <flutter-cupertino-switch>），
            // 先 applyTheme 会让那些监听器读到**上一轮**的旧色板。
            //
            // try/catch：本监听器里抛出会吞掉同一条消息的后续处理 —— 色板失败不该
            // 连带把亮暗切换也打掉（兜底色还在 theme.css 里）。
            if (e.data.colors) {
                try {
                    applyColorScheme(e.data.colors);
                } catch (err) {
                    console.warn('[songloft] color-scheme apply failed:', err);
                }
            }
            try {
                // 老宿主没有 appearance 字段时显式回退 standard，避免复用新宿主
                // 留在 localStorage 里的 capsule 样式。
                applyThemeAppearance(e.data.appearance || DEFAULT_THEME_APPEARANCE);
            } catch (err) {
                console.warn('[songloft] theme appearance apply failed:', err);
            }
            applyTheme(e.data.theme);
        } else if (e.data.type === 'songloft-player-state') {
            dispatchPlayerState(e.data.state);
        } else if (e.data.type === 'songloft-host-reply') {
            // 安全：host 回执只接受来自父窗口的消息（native 顶层 parent===self 亦成立）。
            if (e.source && e.source !== window.parent) return;
            resolveHostReply(e.data);
        }
    });

    // ── API 工具 ──
    var API_BASE = '.';

    /**
     * 从 localStorage 或 URL 参数获取 Songloft 认证 Token
     * @returns {string}
     */
    function getAuthToken() {
        try {
            // 优先从 localStorage 读取（Flutter/浏览器环境）
            var authData = localStorage.getItem('songloft-auth');
            if (authData) {
                var auth = JSON.parse(authData);
                if (auth.accessToken) return auth.accessToken;
            }
            // HarmonyOS webview localStorage 隔离，从 URL 参数读取作为 fallback
            var urlParams = new URLSearchParams(window.location.search);
            var token = urlParams.get('access_token');
            if (token) return token;
        } catch (e) {
            // ignore
        }
        return '';
    }

    function buildHeaders() {
        var headers = { 'Content-Type': 'application/json' };
        var token = getAuthToken();
        if (token) {
            headers['Authorization'] = 'Bearer ' + token;
        }
        return headers;
    }

    function parseResponse(response) {
        if (!response.ok) {
            return response.text().then(function(text) {
                var msg = response.statusText || ('HTTP ' + response.status);
                try {
                    var body = JSON.parse(text);
                    if (body && (body.message || body.error)) {
                        msg = body.message || body.error;
                    }
                } catch (_) {}
                throw new Error(msg);
            });
        }
        return response.text().then(function(text) {
            if (!text) return null;
            return JSON.parse(text);
        });
    }

    /**
     * 发送 GET 请求并返回 JSON
     * @param {string} path
     * @returns {Promise<any>}
     */
    function apiGet(path) {
        return fetch(API_BASE + path, {
            method: 'GET',
            headers: buildHeaders()
        }).then(parseResponse);
    }

    /**
     * 发送 POST 请求并返回 JSON
     * @param {string} path
     * @param {any} body
     * @returns {Promise<any>}
     */
    function apiPost(path, body) {
        return fetch(API_BASE + path, {
            method: 'POST',
            headers: buildHeaders(),
            body: JSON.stringify(body)
        }).then(parseResponse);
    }

    /**
     * 发送 PUT 请求并返回 JSON
     * @param {string} path
     * @param {any} body
     * @returns {Promise<any>}
     */
    function apiPut(path, body) {
        return fetch(API_BASE + path, {
            method: 'PUT',
            headers: buildHeaders(),
            body: JSON.stringify(body)
        }).then(parseResponse);
    }

    /**
     * 发送 DELETE 请求并返回 JSON
     * @param {string} path
     * @returns {Promise<any>}
     */
    function apiDelete(path) {
        return fetch(API_BASE + path, {
            method: 'DELETE',
            headers: buildHeaders()
        }).then(parseResponse);
    }

    /**
     * Blob → `data:` URL（songloft-org/songloft#341）。
     *
     * 「带鉴权头 fetch 一张图 → 显示」是插件常见写法（fetch 拿到的是 Blob，
     * `<img src>` 不能直接吃 Blob），转成 `data:` URL 即可嵌入。
     *
     * ⚠️ **本函数是异步的，而 `createObjectURL` 是同步的** —— blob → base64 只能经
     * `arrayBuffer()` / `FileReader`（都是异步）。所以插件**必须改调用点**。
     * 实现选 `blob.arrayBuffer()`（各环境通用）。
     *
     * ⚠️⚠️ **刻意不用 `btoa`，自带 base64 编码表。** 部分 btoa 实现不是二进制安全的：
     * 它把 > 0x7F 的码点当字符先做了一次 UTF-8 编码，不仅值错还静默丢字节。
     * `atob` 方向是正确的，故只需自己实现 encode 方向。
     * 分块处理（3 字节一组、每 8 KB 拼一次）避免 RangeError 与 O(n²)。
     *
     * @param {Blob} blob
     * @param {string} [mimeType] 覆盖 blob.type（部分环境 blob.type 恒为空串）
     * @returns {Promise<string>} 形如 `data:image/jpeg;base64,...`
     */
    var B64_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';

    /** Uint8Array → base64。不依赖 btoa，见 blobToDataURL 的注释。 */
    function bytesToBase64(bytes) {
        var out = '';
        var buf = [];
        var i = 0;
        var len = bytes.length;
        // 每次吃 3 字节产 4 个 base64 字符；余数在循环外单独补 padding
        var limit = len - (len % 3);
        for (i = 0; i < limit; i += 3) {
            var n = (bytes[i] << 16) | (bytes[i + 1] << 8) | bytes[i + 2];
            buf.push(
                B64_CHARS.charAt((n >> 18) & 63),
                B64_CHARS.charAt((n >> 12) & 63),
                B64_CHARS.charAt((n >> 6) & 63),
                B64_CHARS.charAt(n & 63)
            );
            // 攒够一批再 join，避免 O(n²) 的字符串拼接
            if (buf.length >= 8192) {
                out += buf.join('');
                buf.length = 0;
            }
        }
        var rem = len % 3;
        if (rem === 1) {
            var a = bytes[len - 1];
            buf.push(
                B64_CHARS.charAt((a >> 2) & 63),
                B64_CHARS.charAt((a << 4) & 63),
                '=', '='
            );
        } else if (rem === 2) {
            var b0 = bytes[len - 2], b1 = bytes[len - 1];
            buf.push(
                B64_CHARS.charAt((b0 >> 2) & 63),
                B64_CHARS.charAt(((b0 << 4) | (b1 >> 4)) & 63),
                B64_CHARS.charAt((b1 << 2) & 63),
                '='
            );
        }
        out += buf.join('');
        return out;
    }

    function blobToDataURL(blob, mimeType) {
        if (!blob) return Promise.reject(new Error('blobToDataURL: no blob'));
        if (typeof blob.arrayBuffer !== 'function') {
            return Promise.reject(new Error('blobToDataURL: Blob.arrayBuffer unavailable'));
        }
        var mime = mimeType || blob.type || 'application/octet-stream';
        return blob.arrayBuffer().then(function(buf) {
            return 'data:' + mime + ';base64,' + bytesToBase64(new Uint8Array(buf));
        });
    }

    /**
     * 获取当前主题
     * @returns {'light' | 'dark'}
     */
    function getTheme() {
        // 与 applyTheme 对称，不走 dataset（见 applyTheme 注释）
        return document.documentElement.getAttribute('data-theme') || 'light';
    }

    /**
     * 监听主题变化
     * @param {(theme: 'light' | 'dark') => void} callback
     */
    function onThemeChange(callback) {
        document.addEventListener('songloft-theme-change', function(e) {
            callback(e.detail.theme);
        });
    }

    /**
     * 宿主真实色板。key 是 Flutter `ColorScheme` 的字段名（camelCase），
     * 值是 `#RRGGBB`，例如 `{primary: '#415F91', surfaceContainer: '#EDEDF4', ...}`。
     *
     * **这是插件用 JS 读色的唯一正确途径** —— `getComputedStyle` 对自定义属性的
     * 支持没有跨环境契约，直接读 CSS 变量不可靠。
     *
     * 宿主还没推到时返回 `null`，此时页面用的是 `theme.css` 的静态兜底色。想在到达
     * 时收到通知就监听 `document` 上的 `songloft-color-scheme-change` 事件，或用
     * `onThemeChange` —— 色板保证在 `songloft-theme-change` 派发**之前**就已落地。
     *
     * 返回浅拷贝：调用方改了也不会污染内部缓存。
     * @returns {Object|null}
     */
    function getColorScheme() {
        if (!lastColorScheme) return null;
        var out = {};
        for (var k in lastColorScheme) {
            if (Object.prototype.hasOwnProperty.call(lastColorScheme, k)) out[k] = lastColorScheme[k];
        }
        return out;
    }

    // ── Accessibility ──

    function hideDecorationIcons() {
        document.querySelectorAll('.material-symbols-outlined, .mi').forEach(function(el) {
            if (!el.getAttribute('aria-hidden')) {
                el.setAttribute('aria-hidden', 'true');
            }
        });
    }

    function enhanceClickableElements() {
        document.querySelectorAll('[onclick]').forEach(function(el) {
            var tag = el.tagName.toLowerCase();
            if (tag !== 'button' && tag !== 'a' && tag !== 'input' && tag !== 'select') {
                if (!el.getAttribute('role')) el.setAttribute('role', 'button');
                if (!el.getAttribute('tabindex')) el.setAttribute('tabindex', '0');
                el.addEventListener('keydown', function(e) {
                    if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        el.click();
                    }
                });
            }
        });
    }

    function announce(message, priority) {
        var region = document.getElementById('songloft-a11y-live');
        if (!region) {
            region = document.createElement('div');
            region.id = 'songloft-a11y-live';
            region.className = 'sr-only';
            region.setAttribute('aria-live', priority || 'polite');
            region.setAttribute('aria-atomic', 'true');
            document.body.appendChild(region);
        }
        region.textContent = '';
        setTimeout(function() { region.textContent = message; }, 100);
    }

    function initAccessibility() {
        hideDecorationIcons();
        enhanceClickableElements();
        var snackbar = document.getElementById('snackbar');
        if (snackbar && !snackbar.getAttribute('role')) {
            snackbar.setAttribute('role', 'status');
            snackbar.setAttribute('aria-live', 'polite');
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initAccessibility);
    } else {
        initAccessibility();
    }

    // ── 宿主客户端桥接（Flutter / Lynx WebView 和 Web iframe）──
    //
    // 让 webview 打开的插件页调用 Flutter 宿主能力（改写正在播放队列、播放控制、
    // 状态订阅等）。请求走 flutter_inappwebview 的 callHandler（原生 Promise 返回值），
    // 事件（播放状态变更）复用上面的 postMessage 通道。
    // 独立浏览器页面无宿主时 isHostAvailable() 返回 false，调用会 reject。

    var HOST_HANDLER = 'songloftHost';
    var HOST_CALL_TIMEOUT_MS = 10000;

    // native（Android/iOS/桌面）webview：flutter_inappwebview 提供请求/响应式 callHandler。
    function isNativeHost() {
        return !!(window.flutter_inappwebview &&
            typeof window.flutter_inappwebview.callHandler === 'function');
    }

    // The Lynx XElement browser has a message bridge instead of Flutter's callHandler.
    // The host marks only its embedded URL; external browser URLs omit this flag.
    var lynxHost = /(?:^|[?&])songloft_host=lynx(?:&|$)/.test(window.location.search);
    function isLynxHost() { return lynxHost; }

    // Web：插件页运行在宿主 iframe 内，走 postMessage 与父窗口通信。
    // 独立浏览器标签（parent === self）没有宿主，返回 false。
    function isIframeHost() {
        try {
            return !!window.parent && window.parent !== window;
        } catch (e) {
            return true; // 跨域访问 parent 抛错 → 视为嵌入
        }
    }

    function isHostAvailable() {
        return isNativeHost() || isLynxHost() || isIframeHost();
    }

    // ── Web/iframe postMessage 传输：请求/响应关联 ──
    var hostPending = {};
    var hostCallSeq = 0;

    function sendHostMessage(message) {
        if (isLynxHost()) {
            var json = JSON.stringify(message);
            if (window.LynxWebViewBridge && typeof window.LynxWebViewBridge.onMessageReceived === 'function') {
                window.LynxWebViewBridge.onMessageReceived(json);
            } else if (window.LynxJSBridge && typeof window.LynxJSBridge.onMessageReceived === 'function') {
                window.LynxJSBridge.onMessageReceived(json);
            } else if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.nativeApp) {
                window.webkit.messageHandlers.nativeApp.postMessage(json);
            } else {
                throw new Error('songloft Lynx WebView bridge unavailable');
            }
        } else {
            window.parent.postMessage(message, '*');
        }
    }

    function invokeViaPostMessage(ns, method, params) {
        return new Promise(function(resolve, reject) {
            var id = 'c' + (++hostCallSeq) + '_' + Date.now();
            var timer = setTimeout(function() {
                delete hostPending[id];
                reject(new Error('songloft host call timeout: ' + ns + '.' + method));
            }, HOST_CALL_TIMEOUT_MS);
            hostPending[id] = { resolve: resolve, reject: reject, timer: timer };
            if (ns === 'player' && method === 'openVideo') {
                params = Object.assign({}, params, { __songloftReplyAck: true });
            }
            var message = { type: 'songloft-host-call', id: id, ns: ns, method: method, params: params || null };
            try {
                sendHostMessage(message);
            } catch (error) {
                clearTimeout(timer);
                delete hostPending[id];
                reject(error);
            }
        });
    }

    function resolveHostReply(msg) {
        var p = hostPending[msg.id];
        if (!p) return;
        clearTimeout(p.timer);
        delete hostPending[msg.id];
        if (!msg.ok) {
            p.reject(new Error(msg.error || 'songloft host call failed'));
            return;
        }
        p.resolve(msg.data);
        if (msg.acknowledge === true) {
            Promise.resolve().then(function() {
                sendHostMessage({
                    type: 'songloft-host-call',
                    id: msg.id,
                    ns: 'lifecycle',
                    method: 'hostReplyReceived'
                });
            }).catch(function(error) { console.error('Could not confirm host reply', error); });
        }
    }

    /**
     * 调用宿主能力。约定返回 { ok, data } 或 { ok:false, error }。
     * native 走 callHandler，Web/iframe 走 postMessage 关联。
     * @returns {Promise<any>}
     */
    function invokeHost(ns, method, params) {
        if (isNativeHost()) {
            return window.flutter_inappwebview
                .callHandler(HOST_HANDLER, { ns: ns, method: method, params: params || null })
                .then(function(res) {
                    if (res && res.ok) return res.data;
                    throw new Error((res && res.error) || 'songloft host call failed');
                });
        }
        if (isLynxHost() || isIframeHost()) {
            return invokeViaPostMessage(ns, method, params);
        }
        return Promise.reject(new Error('songloft host bridge unavailable (not running in a Songloft client webview)'));
    }

    // 播放状态订阅
    var playerStateListeners = [];

    function dispatchPlayerState(state) {
        for (var i = 0; i < playerStateListeners.length; i++) {
            try { playerStateListeners[i](state); } catch (e) { /* ignore */ }
        }
        document.dispatchEvent(new CustomEvent('songloft-player-state-change', { detail: state }));
    }

    var host = {
        isAvailable: isHostAvailable,
        getInfo: function() { return invokeHost('host', 'getInfo'); },
        openExternalUrl: function(url) {
            return invokeHost('plugin', 'openUrl', { url: url }).then(function() {});
        }
    };

    var player = {
        getState: function() { return invokeHost('player', 'getState'); },
        openPlayer: function() {
            return invokeHost('player', 'openPlayer').then(function() {});
        },
        openQueue: function() {
            return invokeHost('player', 'openQueue').then(function() {});
        },
        openVideo: function(options) {
            return invokeHost('player', 'openVideo', options).then(function() {});
        },
        getAudioEffects: function() { return invokeHost('player', 'getAudioEffects'); },
        applyAudioEffects: function(options) { return invokeHost('player', 'applyAudioEffects', options); },
        clearAudioEffects: function(options) {
            return invokeHost('player', 'clearAudioEffects', options).then(function() {});
        },
        setQueue: function(ids, options) {
            options = options || {};
            return invokeHost('player', 'setQueue', {
                ids: ids,
                startIndex: options.startIndex,
                sourcePlaylistId: options.sourcePlaylistId
            });
        },
        addToQueue: function(ids) { return invokeHost('player', 'addToQueue', { ids: ids }); },
        insertToQueue: function(index, id) { return invokeHost('player', 'insertToQueue', { index: index, id: id }); },
        removeFromQueue: function(index) { return invokeHost('player', 'removeFromQueue', { index: index }); },
        reorderQueue: function(oldIndex, newIndex) { return invokeHost('player', 'reorderQueue', { oldIndex: oldIndex, newIndex: newIndex }); },
        clearQueue: function() { return invokeHost('player', 'clearQueue'); },
        play: function(id) { return invokeHost('player', 'play', { id: id }); },
        pause: function() { return invokeHost('player', 'pause'); },
        togglePlay: function() { return invokeHost('player', 'togglePlay'); },
        next: function() { return invokeHost('player', 'next'); },
        prev: function() { return invokeHost('player', 'prev'); },
        seek: function(seconds) { return invokeHost('player', 'seek', { seconds: seconds }); },
        setVolume: function(volume) { return invokeHost('player', 'setVolume', { volume: volume }); },
        setPlayMode: function(mode) { return invokeHost('player', 'setPlayMode', { mode: mode }); },
        playPlaylistById: function(playlistId) { return invokeHost('player', 'playPlaylistById', { playlistId: playlistId }); },
        onStateChange: function(handler) {
            playerStateListeners.push(handler);
            return function() {
                var idx = playerStateListeners.indexOf(handler);
                if (idx >= 0) playerStateListeners.splice(idx, 1);
            };
        }
    };

    /**
     * 读取指定 origin 的 Cookie（仅原生客户端可用，Web 不支持）。
     * @param {string} origin - 目标站点 origin，如 "https://example.com"
     * @returns {Promise<Record<string, string>>} name→value 映射
     */
    function getCookies(origin) {
        return invokeHost('cookies', 'get', { origin: origin });
    }

    var favorite = {
        /**
         * 通知宿主刷新收藏状态缓存。
         *
         * 插件改完收藏（如自己 POST `/playlists/1/songs`）后必须调一次，否则
         * Flutter 侧曲库的红心读的是 `FavoriteNotifier` 的旧缓存、不会跟着变
         * （songloft-org/songloft-plugin-miot#86）。
         *
         * 两种用法刻意都保留：带参是增量更新（宿主只改这一首的归属，不重拉全表），
         * 不带参是全量重载。**能带参就带参** —— 曲库上千首时全量重载是一次
         * 完整的 `/playlists/1/songs` 往返。
         *
         * @param {number} [songId] 歌曲 ID
         * @param {boolean} [isFavorited] 操作后的收藏态
         * @returns {Promise<void>}
         */
        refresh: function(songId, isFavorited) {
            if (songId === undefined || songId === null || isFavorited === undefined || isFavorited === null) {
                return invokeHost('favorite', 'refresh');
            }
            return invokeHost('favorite', 'refresh', { songId: songId, isFavorited: isFavorited });
        }
    };

    window.SongloftPlugin = {
        getAuthToken: getAuthToken,
        apiGet: apiGet,
        apiPost: apiPost,
        apiPut: apiPut,
        apiDelete: apiDelete,
        getTheme: getTheme,
        onThemeChange: onThemeChange,
        getColorScheme: getColorScheme,
        // Blob → data: URL（浏览器 / WebView 通用）。
        blobToDataURL: blobToDataURL,
        announce: announce,
        hideDecorationIcons: hideDecorationIcons,
        enhanceClickableElements: enhanceClickableElements,
        host: host,
        player: player,
        favorite: favorite,
        getCookies: getCookies,
        // 通用宿主调用出口。上面那些命名空间都是它的 typed wrapper；这里公开它
        // 是为了让插件能触达尚未被 wrapper 覆盖的 namespace（宿主分发表在
        // 客户端侧，可能比服务端这份 common.js 更新）。
        //
        // ⚠️ 没有 wrapper 的那层类型约束：ns / method 拼错只会在运行时 reject。
        // 有对应 wrapper 时优先用 wrapper。
        invokeHost: invokeHost
    };
})();
