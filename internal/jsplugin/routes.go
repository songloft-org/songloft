package jsplugin

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"songloft/internal/services"
)

//go:embed assets/*
var pluginAssets embed.FS

const maxPluginBodySize = 50 << 20 // 50MB，对齐 multipart 上传需求

func init() {
	// GENA uses extension methods. Register before any chi router is built;
	// registration changes chi's process-wide method table.
	chi.RegisterMethod("SUBSCRIBE")
	chi.RegisterMethod("UNSUBSCRIBE")
}

// assetVersions 缓存公共资源（theme.css / components.css / common.js）内容哈希的
// 前 8 位 hex，用于给 injectHTMLHead 注入的资源 URL 加 ?v=<hash> 做 cache-busting。
//
// 背景（#278）：jsplugin-assets 用固定无版本 URL（/api/v1/jsplugin-assets/<name>）
// + "immutable, max-age=1年" 长缓存服务。immutable 意味着浏览器连重新验证都不做——
// 一旦某个浏览器缓存了旧资源，之后再改（如滚动条抖动修复）也永远到不了该用户，
// 表现为"修了还抖"。加内容哈希版本号后，文件一变 URL 就变，老缓存自然失效；
// 承载页 HTML 是 no-cache，每次都会带出最新版本号，故修复能即时下发。内容不变时 URL 恒定，
// immutable 长缓存依旧生效。
var assetVersions = computeAssetVersions()

func computeAssetVersions() map[string]string {
	versions := make(map[string]string, 5)
	for _, name := range []string{"theme.css", "components.css", "common.js", "lynx-plugin-bridge-shim.js", "lynx-plugin-bridge-module.js"} {
		data, err := pluginAssets.ReadFile("assets/" + name)
		if err != nil {
			continue
		}
		versions[name] = sha256HexSum(data)[:8]
	}
	return versions
}

// assetURL 返回带内容哈希版本号的公共资源 URL；无对应版本时回退到无版本 URL。
func assetURL(assetsBase, name string) string {
	if v := assetVersions[name]; v != "" {
		return assetsBase + name + "?v=" + v
	}
	return assetsBase + name
}

// authBridgeScriptTpl 注入到每个插件 HTML 页面 <head> 顶部的小脚本：
//  1. authBridge：从 URL query parameter 读取 access_token 并存入 localStorage，
//     使插件 JS 通过 ?access_token=xxx 传递的 token 可被读取；执行后通过
//     history.replaceState 清理 URL 中的 token 参数。
//  2. fetchRetry：包装全局 fetch，对插件 API 路径下的 503 plugin_unavailable
//     响应自动等待 200ms 重试一次。配合后端的懒加载 / 自愈机制：
//     第一次请求触发后端冷启动加载（通常 100~500ms），自动重试拿到结果，
//     用户感知正常。仅重试 errCode=plugin_unavailable，不重试 plugin_disabled
//     / 4xx / 200，避免无效重试。
const authBridgeScriptTpl = `<script>(function(){var p=new URLSearchParams(window.location.search);var t=p.get("access_token");if(t){localStorage.setItem("songloft-auth",JSON.stringify({accessToken:t}));p.delete("access_token");var u=window.location.pathname;var r=p.toString();if(r)u+="?"+r;history.replaceState(null,"",u)}var __of=window.fetch.bind(window);window.fetch=function(input,init){return __of(input,init).then(function(resp){if(resp.status!==503)return resp;var u=typeof input==="string"?input:(input&&input.url)||"";if(u.indexOf("/api/v1/jsplugin/")<0)return resp;var ct=resp.headers.get("content-type")||"";if(ct.indexOf("application/json")<0)return resp;var c=resp.clone();return c.json().then(function(j){if(!j||j.error!=="plugin_unavailable")return resp;return new Promise(function(res){setTimeout(function(){res(__of(input,init))},200)})}).catch(function(){return resp})})}})();</script>`

// RegisterStaticRoutes 注册 JS 插件静态资源路由（无需认证）
//
// 路由结构（参考 WASM 插件 plugin_static.go 的设计）：
//   - GET /api/v1/jsplugin/{entryPath}              → 直接服务 index.html（注入 <base> 标签）
//   - GET /api/v1/jsplugin/{entryPath}/             → 同上（带尾斜杠，防止被 catch-all 匹配到 auth 路由）
//   - GET /api/v1/jsplugin/{entryPath}/static       → 静态目录根（服务 index.html）
//   - GET /api/v1/jsplugin/{entryPath}/static/*     → 静态资源文件
//
// 这些路由不需要认证，与 WASM 插件的静态资源路由一致。
func (m *Manager) RegisterStaticRoutes(r chi.Router) {
	r.Get("/api/v1/jsplugin/{entryPath}", m.handlePluginStatic)
	r.Get("/api/v1/jsplugin/{entryPath}/", m.handlePluginStatic)
	r.Get("/api/v1/jsplugin/{entryPath}/static", m.handlePluginStaticSubdir)
	r.Get("/api/v1/jsplugin/{entryPath}/static/*", m.handlePluginStaticSubdirFiles)
	r.Get("/api/v1/jsplugin-assets/web-core/*", handleWebCoreAssets)
	r.Get("/api/v1/jsplugin-assets/*", handlePluginAssets)

	// publicPaths：为声明了 publicPaths 的插件注册无需 JWT 的路由
	m.RefreshPublicPaths()
}

// handlePluginAssets 服务插件公共资源（CSS/JS/字体）。
//
// @Summary     插件公共资源
// @Description 服务由主程序嵌入的插件公共资源（theme.css / components.css / common.js 及字体），自动注入到所有插件 HTML 页面。
// @Tags        JS 插件
// @Produce     octet-stream
// @Param       * path string true "资源路径"
// @Success     200 "资源文件"
// @Failure     404 {object} map[string]string "资源不存在"
// @Router      /jsplugin-assets/{path} [get]
func handlePluginAssets(w http.ResponseWriter, r *http.Request) {
	subPath := chi.URLParam(r, "*")
	if subPath == "" {
		http.NotFound(w, r)
		return
	}

	filePath := "assets/" + subPath
	f, err := pluginAssets.Open(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if (subPath == "lynx-plugin-bridge-shim.js" || subPath == "lynx-plugin-bridge-module.js") && r.URL.Query().Get("v") == "" {
		w.Header().Set("Cache-Control", "no-cache")
	}

	seeker, ok := f.(io.ReadSeeker)
	if !ok {
		data, err := fs.ReadFile(pluginAssets, filePath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		seeker = bytes.NewReader(data)
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), seeker)
}

// webCoreDir 缓存 web-core 静态资源目录路径（Lynx 插件 Flutter 兼容用）。
// 首次请求时自动探测：优先 data/web-core/，其次可执行文件同目录的 web-core/。
var (
	webCoreDirOnce sync.Once
	webCoreDir     string
)

func resolveWebCoreDir() string {
	webCoreDirOnce.Do(func() {
		// 优先数据目录
		candidates := []string{
			"data/web-core",
			"web-core",
		}
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "web-core"))
		}
		for _, dir := range candidates {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				webCoreDir = dir
				return
			}
		}
	})
	return webCoreDir
}

// handleWebCoreAssets 服务 @lynx-js/web-core 的静态资源（Lynx 插件在 Flutter
// WebView 中渲染时需要）。资源从磁盘目录读取（不嵌入二进制，约 620KB）。
//
// @Summary     Web-core 静态资源
// @Description 服务 Lynx 渲染引擎的 Web 端运行时（CSS/JS/WASM），供声明 renderEngine=lynx 的插件在系统 WebView 中使用。
// @Tags        JS 插件
// @Produce     octet-stream
// @Param       * path string true "资源路径（如 static/js/client.js）"
// @Success     200 "资源文件"
// @Failure     404 {object} map[string]string "资源不存在或 web-core 目录未配置"
// @Router      /jsplugin-assets/web-core/{path} [get]
func handleWebCoreAssets(w http.ResponseWriter, r *http.Request) {
	dir := resolveWebCoreDir()
	if dir == "" {
		http.NotFound(w, r)
		return
	}

	subPath := chi.URLParam(r, "*")
	if subPath == "" {
		http.NotFound(w, r)
		return
	}

	// 安全：防止路径遍历
	clean := filepath.Clean(subPath)
	if strings.Contains(clean, "..") {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(dir, clean)
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filePath)
}

// RegisterAPIRoutes 注册 JS 插件 API 转发路由（需要认证，由调用方添加 AuthMiddleware）
//
// 路由结构：
//   - GET/HEAD /api/v1/jsplugin/{entryPath}/files/* → 文件 serve（Go 原生 ServeFile）
//   - HandleFunc /api/v1/jsplugin/{entryPath}/*     → catch-all，处理 API 转发
//
// 分发逻辑：
//  1. /files/* → 直接 serve 插件可访问范围内的文件（支持 Range、HTTP 缓存）
//  2. 子路径为空（尾部斜杠）→ 服务 static/index.html
//  3. 其他子路径 → 转发给 JS 运行时处理（API 路由）
//
// 注意：chi 的路由优先级机制确保 GET 请求到 /static/* 或 /files/* 路径会优先匹配
// 更具体的路由，不会进入 catch-all。
func (m *Manager) RegisterAPIRoutes(r chi.Router) {
	r.Get("/api/v1/jsplugin/{entryPath}/files/*", m.handlePluginFileServe)
	r.Head("/api/v1/jsplugin/{entryPath}/files/*", m.handlePluginFileServe)
	r.HandleFunc("/api/v1/jsplugin/{entryPath}/*", m.handlePluginAPIRequest)
}

// handlePluginStatic 处理无尾部斜杠的插件根路径请求，
// 直接服务 static/index.html 并注入 <base> 标签，使浏览器正确解析相对路径。
//
//	GET /api/v1/jsplugin/{entryPath} → 直接返回 index.html（含 <base href="...">）
//
// 注意：静态文件服务不依赖插件 JS 运行时是否就绪，只需要数据目录存在即可。
// 这确保了插件初始化期间（onInit 尚未完成）前端页面仍可正常加载。
// @Summary 插件根页面（动态路由）
// @Description JS 插件入口 HTML。{entryPath} 由运行时按已安装插件决定，注入 <base> 标签和 auth-bridge 脚本后返回 static/index.html。无需认证。
// @Tags JS 插件
// @Produce html
// @Param entryPath path string true "插件入口标识（运行时动态）"
// @Success 200 {string} string "插件 index.html（已注入 <base> 与 auth-bridge）"
// @Failure 404 {string} string "插件未安装或缺 static/index.html"
// @Router /jsplugin/{entryPath} [get]
func (m *Manager) handlePluginStatic(w http.ResponseWriter, r *http.Request) {
	entryPath := chi.URLParam(r, "entryPath")
	staticRoot := filepath.Join(m.pluginsDataDir, entryPath, "static")
	absStaticRoot, err := filepath.Abs(staticRoot)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// 验证静态目录存在（替代 GetService 检查，使静态文件不依赖 JS 运行时）
	info, statErr := os.Stat(absStaticRoot)
	if statErr != nil || !info.IsDir() {
		http.Error(w, "plugin not found", http.StatusNotFound)
		return
	}
	if !m.tryServeStaticFile(w, r, absStaticRoot, "index.html", entryPath) {
		http.NotFound(w, r)
	}
}

// handlePluginStaticSubdir 处理 GET /api/v1/jsplugin/{entryPath}/static 请求
// 服务 static/index.html（静态目录根）
// @Summary 插件 static 目录根（动态路由）
// @Description 服务 static/index.html，是 handlePluginStatic 的「带 /static 后缀」变体。无需认证。
// @Tags JS 插件
// @Produce html
// @Param entryPath path string true "插件入口标识（运行时动态）"
// @Success 200 {string} string "插件 index.html"
// @Failure 404 {string} string "插件未安装或缺 static/index.html"
// @Router /jsplugin/{entryPath}/static [get]
func (m *Manager) handlePluginStaticSubdir(w http.ResponseWriter, r *http.Request) {
	entryPath := chi.URLParam(r, "entryPath")
	m.servePluginStaticFile(w, r, entryPath, "static")
}

// handlePluginStaticSubdirFiles 处理 GET /api/v1/jsplugin/{entryPath}/static/* 请求
// 从磁盘提供静态资源文件，支持 SPA fallback
// @Summary 插件静态资源文件（动态路由）
// @Description 从插件磁盘目录返回 CSS/JS/图片等静态资源；未命中且非 index.html 时 SPA fallback 到 index.html。无需认证。
// @Tags JS 插件
// @Produce octet-stream
// @Param entryPath path string true "插件入口标识（运行时动态）"
// @Success 200 {file} binary "静态资源文件内容"
// @Failure 404 {string} string "文件不存在且无 index.html fallback"
// @Router /jsplugin/{entryPath}/static/{path} [get]
func (m *Manager) handlePluginStaticSubdirFiles(w http.ResponseWriter, r *http.Request) {
	entryPath := chi.URLParam(r, "entryPath")
	subPath := "static/" + chi.URLParam(r, "*")
	m.servePluginStaticFile(w, r, entryPath, subPath)
}

// handlePluginAPIRequest 处理 JS 插件的 API 转发请求（需要认证）
//
// 分发逻辑：
//   - 子路径为空（即 /api/v1/jsplugin/{entryPath}/） → 直接服务 static/index.html
//   - 子路径以 "static/" 开头或等于 "static" → 静态文件直通（POST/PUT 等非 GET 方法的兜底）
//   - 其他子路径 → 转发到 JS 运行时处理（API 路由）
//
// @Summary 插件 API 转发 catch-all（动态路由）
// @Description 接受任意 HTTP 方法，分发到插件 static 兜底、入站 WebSocket upgrade（调用 onWebSocket），或转发到 QuickJS 沙盒中的插件代码。{entryPath} 和子路径均由运行时决定，OpenAPI 仅作占位。需要 BearerAuth。
// @Description 支持 UPnP 事件订阅的 SUBSCRIBE/UNSUBSCRIBE 扩展方法（OpenAPI 2 无对应方法，仅在此说明）。插件 publicPaths 中声明的路径免 JWT，由插件自行校验协议访问范围。
// @Tags JS 插件
// @Accept json
// @Produce json
// @Param entryPath path string true "插件入口标识（运行时动态）"
// @Param X-Plugin-Timeout-Ms header int false "可选：放宽本次插件调用的超时（毫秒），宿主 clamp 到 [30000, 300000]。慢端点（如 yt-dlp 展开歌单）用于避免默认 30s 超时"
// @Success 200 {object} map[string]interface{} "插件自定义响应"
// @Failure 403 {object} map[string]string "插件未启用"
// @Failure 404 {object} map[string]string "插件不存在"
// @Failure 503 {object} map[string]string "插件不可用或运行异常（健康检查会自愈）"
// @Failure 504 {object} map[string]string "JS 运行时调用超时"
// @Security BearerAuth
// @Router /jsplugin/{entryPath}/{path} [get]
// @Router /jsplugin/{entryPath}/{path} [post]
// @Router /jsplugin/{entryPath}/{path} [put]
// @Router /jsplugin/{entryPath}/{path} [delete]
func (m *Manager) handlePluginAPIRequest(w http.ResponseWriter, r *http.Request) {
	entryPath := chi.URLParam(r, "entryPath")
	subPath := chi.URLParam(r, "*")

	// 根路径（带尾部斜杠）：直接返回 static/index.html（不依赖 JS 运行时）
	if subPath == "" {
		staticRoot := filepath.Join(m.pluginsDataDir, entryPath, "static")
		absStaticRoot, err := filepath.Abs(staticRoot)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		info, statErr := os.Stat(absStaticRoot)
		if statErr != nil || !info.IsDir() {
			http.Error(w, "plugin not found", http.StatusNotFound)
			return
		}
		if !m.tryServeStaticFile(w, r, absStaticRoot, "index.html", entryPath) {
			http.NotFound(w, r)
		}
		return
	}

	// 静态资源兜底：非 GET 方法访问 static/ 路径时的安全处理
	if subPath == "static" || strings.HasPrefix(subPath, "static/") {
		m.servePluginStaticFile(w, r, entryPath, subPath)
		return
	}

	// 非 static 路径 → 需要 JS 运行时，按需懒加载。
	// 空闲驱逐场景下 service 不在内存但 DB status=active，EnsureLoaded 会自动重新加载。
	service, err := m.EnsureLoaded(r.Context(), entryPath)
	if err != nil {
		m.writePluginUnavailable(w, r, entryPath, err)
		return
	}

	if isWebSocketUpgrade(r) {
		m.handlePluginWebSocket(w, r, service, entryPath, subPath)
		return
	}

	// 转发给 JS 运行时处理（API 路由）
	m.forwardToJSRuntime(w, r, entryPath, subPath)
}

// writePluginUnavailable 在 JS 运行时缺失时返回结构化错误响应。
// 根据 EnsureLoaded 返回的语义错误（或路由层兜底回退）选择 4xx/5xx 状态码：
//   - ErrPluginDisabled → 403 + plugin_disabled
//   - ErrPluginNotFound → 404 + plugin_not_found
//   - ErrPluginErrorState 或其他 → 503 + plugin_unavailable
//
// body 统一为 JSON，避免前端 response.json() 解析纯文本时抛 SyntaxError。
func (m *Manager) writePluginUnavailable(w http.ResponseWriter, r *http.Request, entryPath string, cause error) {
	status := http.StatusServiceUnavailable
	errCode := "plugin_unavailable"
	message := "插件暂不可用，请稍后重试"

	switch {
	case errors.Is(cause, ErrPluginDisabled):
		status = http.StatusForbidden
		errCode = "plugin_disabled"
		message = "插件未启用，请前往设置启用"
	case errors.Is(cause, ErrPluginNotFound):
		status = http.StatusNotFound
		errCode = "plugin_not_found"
		message = "插件不存在"
	case errors.Is(cause, ErrPluginErrorState):
		// status=error 走 503，由 HealthChecker 的指数退避自愈策略负责重试，
		// 前端可以通过单次重试或定时刷新等待自愈，无需用户介入。
		status = http.StatusServiceUnavailable
		errCode = "plugin_unavailable"
		message = "插件运行异常，正在自动恢复中"
	}

	if cause != nil {
		slog.Info("plugin request rejected",
			"entryPath", entryPath,
			"path", r.URL.Path,
			"errCode", errCode,
			"cause", cause,
		)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":  message,
		"detail": errCode,
	})
}

// servePluginStaticFile 处理插件静态资源请求（参考 WASM 插件 servePluginStatic）
// 查找顺序：磁盘命中 → SPA fallback 到 index.html → 404
//
// URL 路径规范：
//
//	GET /api/v1/jsplugin/{entryPath}/static/css/xx.css  → static/css/xx.css
//	GET /api/v1/jsplugin/{entryPath}/static              → static/index.html
//	GET /api/v1/jsplugin/{entryPath}/static/some/route   → 未命中 → fallback 到 index.html
func (m *Manager) servePluginStaticFile(w http.ResponseWriter, r *http.Request, entryPath, subPath string) {
	// 磁盘根目录：jsplugins_data/<entryPath>/static/
	staticRoot := filepath.Join(m.pluginsDataDir, entryPath, "static")
	absStaticRoot, err := filepath.Abs(staticRoot)
	if err != nil {
		slog.Warn("jsplugin-static: 无法获取 staticRoot 绝对路径",
			"entryPath", entryPath, "subPath", subPath, "error", err)
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(absStaticRoot)
	if err != nil || !info.IsDir() {
		slog.Debug("jsplugin-static: 插件无 static 目录",
			"entryPath", entryPath, "staticRoot", absStaticRoot)
		http.NotFound(w, r)
		return
	}

	// 剥掉 "static/" 前缀，避免拼成 .../static/static/xxx
	relPath := subPath
	if relPath == "static" {
		relPath = ""
	} else if after, ok := strings.CutPrefix(relPath, "static/"); ok {
		relPath = after
	}

	slog.Debug("jsplugin-static: 收到请求",
		"entryPath", entryPath, "subPath", subPath, "relPath", relPath)

	// 1. 尝试命中具体文件
	if m.tryServeStaticFile(w, r, absStaticRoot, relPath, entryPath) {
		return
	}

	// 2. SPA fallback：未命中且非 index.html 时回退到 index.html
	if relPath != "" && relPath != "index.html" {
		slog.Debug("jsplugin-static: 未命中，fallback 到 index.html",
			"entryPath", entryPath, "relPath", relPath)
		if m.tryServeStaticFile(w, r, absStaticRoot, "index.html", entryPath) {
			return
		}
	}

	slog.Warn("jsplugin-static: 文件未找到且无 index.html",
		"entryPath", entryPath, "relPath", relPath, "staticRoot", absStaticRoot)
	http.NotFound(w, r)
}

// tryServeStaticFile 尝试从磁盘返回一个静态文件（参考 WASM 插件 tryServeStaticFile）
// 返回 true 表示已成功写响应；false 表示文件不存在。
//
// 安全性：通过 filepath.Abs + HasPrefix 防止路径穿越
// 行为：HTML 文件注入 <base> 标签和 auth-bridge 脚本 + no-cache；其他资源强缓存
func (m *Manager) tryServeStaticFile(w http.ResponseWriter, r *http.Request, staticRoot, relPath, entryPath string) bool {
	if relPath == "" || relPath == "/" {
		relPath = "index.html"
	}

	filePath := filepath.Join(staticRoot, relPath)
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}

	// 路径穿越防御
	sep := string(filepath.Separator)
	if absFile != staticRoot && !strings.HasPrefix(absFile, staticRoot+sep) {
		slog.Warn("jsplugin-static: 路径穿越被拦截",
			"staticRoot", staticRoot, "relPath", relPath, "absFile", absFile)
		return false
	}

	info, err := os.Stat(absFile)
	if err != nil || info.IsDir() {
		return false
	}

	lower := strings.ToLower(absFile)

	// HTML 文件：读入内存、注入 auth-bridge、no-cache
	if strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm") {
		content, readErr := os.ReadFile(absFile)
		if readErr != nil {
			slog.Warn("jsplugin-static: 读取 HTML 失败", "absFile", absFile, "error", readErr)
			return false
		}
		content = injectHTMLHead(content, entryPath, m.basePath)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		// 主动启用了 COEP: require-corp 的宿主页嵌跨源 iframe 时，文档响应自身也
		// 必须携带 COEP 头，否则 Chrome/Firefox 报 ERR_BLOCKED_BY_RESPONSE
		// 拒绝加载。corpMiddleware 的 CORP 头只覆盖子资源（img/audio），盖
		// 不住 iframe 文档这一层，所以插件 HTML 要单独补。
		//
		// credentialless 而非 require-corp：插件页引用的第三方跨源资源
		// （如 fonts.googleapis.com）没有 CORP 头，require-corp 会把它们也拦
		// 掉。这个响应头只让跨源 no-cors 子资源请求不带凭证，仍保留插件
		// 文档导航及同源资源的 Cookie；不要与 iframe 的 credentialless 属性
		// 混淆，后者会隔离整个页面的凭证，导致 FN Connect 等网关登录失效。
		// Firefox 119+ 支持此响应头；Safari 不支持，启用隔离的 standalone
		// 宿主在 Safari 下跨源嵌入仍受限（embedded 部署同源，无此问题）。
		w.Header().Set("Cross-Origin-Embedder-Policy", "credentialless")
		// Web-core's srcdoc MTS realm does not finish its bootstrap under COEP
		// in Chromium 121 (the WebView fallback baseline). Native Lynx frames
		// load bundles directly; only their HTML fallback needs this exception.
		if plugin, err := m.repo.GetByEntryPath(r.Context(), entryPath); err == nil && plugin.RenderEngine == RenderEngineLynx {
			w.Header().Del("Cross-Origin-Embedder-Policy")
		}
		_, _ = w.Write(content)
		return true
	}

	// 其他静态资源：强缓存
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, absFile)
	return true
}

// injectHTMLHead 在 HTML 的 <head> 后（紧跟开标签）注入 <base> 标签、auth bridge 脚本和公共资源引用。
//
// 注入顺序：base → auth bridge → theme.css → components.css → common.js。
//   - CSS 两层：theme.css（令牌契约/字体/reset）→ components.css（组件库）。
//   - JS：common.js（运行时核心，建 window.SongloftPlugin 与内部句柄）。
//
// 全部 render-blocking，保证在页面内容前执行。
//
// <base> 标签使浏览器以 /api/v1/jsplugin/{entryPath}/ 为基准解析相对路径。
// 关键：<base> 必须在所有使用相对 URL 的元素（<link>, <script> 等）之前注入，
// 否则浏览器的预加载扫描器（preload scanner）会用错误的基准 URL 发起资源请求。
//
// 如果 HTML 中没有 <head> 标签，则在文件开头注入。
func injectHTMLHead(html []byte, entryPath, basePath string) []byte {
	baseTag := []byte(`<base href="` + basePath + `/api/v1/jsplugin/` + entryPath + `/">`)
	authScript := []byte(authBridgeScriptTpl)
	assetsBase := basePath + "/api/v1/jsplugin-assets/"
	themeLink := []byte(`<link rel="stylesheet" href="` + assetURL(assetsBase, "theme.css") + `">`)
	componentsLink := []byte(`<link rel="stylesheet" href="` + assetURL(assetsBase, "components.css") + `">`)
	jsScript := []byte(`<script src="` + assetURL(assetsBase, "common.js") + `"></script>`)

	injectPayload := make([]byte, 0, len(baseTag)+len(authScript)+len(themeLink)+len(componentsLink)+len(jsScript))
	injectPayload = append(injectPayload, baseTag...)
	injectPayload = append(injectPayload, authScript...)
	injectPayload = append(injectPayload, themeLink...)
	injectPayload = append(injectPayload, componentsLink...)
	injectPayload = append(injectPayload, jsScript...)

	// 优先在 <head> 开标签之后注入（确保 <base> 出现在所有 <link>/<script> 之前）
	headOpenIdx := bytes.Index(html, []byte("<head>"))
	if headOpenIdx == -1 {
		// 尝试匹配带属性的 <head ...>
		headOpenIdx = bytes.Index(html, []byte("<head "))
		if headOpenIdx != -1 {
			// 找到 '>' 结束位置
			closeIdx := bytes.IndexByte(html[headOpenIdx:], '>')
			if closeIdx != -1 {
				headOpenIdx = headOpenIdx + closeIdx + 1
			} else {
				headOpenIdx = -1
			}
		}
	} else {
		headOpenIdx += len("<head>")
	}

	if headOpenIdx == -1 {
		// 无 <head> 标签，在文件开头注入
		result := make([]byte, 0, len(html)+len(injectPayload))
		result = append(result, injectPayload...)
		result = append(result, html...)
		return result
	}

	result := make([]byte, 0, len(html)+len(injectPayload))
	result = append(result, html[:headOpenIdx]...)
	result = append(result, injectPayload...)
	result = append(result, html[headOpenIdx:]...)
	return result
}

// parsePluginCallTimeout 解析 X-Plugin-Timeout-Ms 头为 scheduler.Call 超时。
// 空 / 非法 / <=0 → 返回 0（scheduler 使用 defaultCallTimeout）。
// 合法值 clamp 到 [defaultCallTimeout, maxCallTimeout]：低于默认无意义，高于上限防滥用。
func parsePluginCallTimeout(header string) time.Duration {
	if header == "" {
		return 0
	}
	ms, err := strconv.Atoi(strings.TrimSpace(header))
	if err != nil || ms <= 0 {
		return 0
	}
	d := time.Duration(ms) * time.Millisecond
	if d < defaultCallTimeout {
		return defaultCallTimeout
	}
	if d > maxCallTimeout {
		return maxCallTimeout
	}
	return d
}

func writeJSONError(w http.ResponseWriter, status int, errMsg, detail string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": errMsg, "detail": detail})
}

// forwardToJSRuntime 将请求转发给 JS 运行时处理
func (m *Manager) forwardToJSRuntime(w http.ResponseWriter, r *http.Request, entryPath, subPath string) {
	// 1. 通过 EnsureLoaded 拿到 service（与 handlePluginAPIRequest 入口保持一致语义，
	// 同时也覆盖 forwardToJSRuntime 被外部直接调用的场景）。
	service, err := m.EnsureLoaded(r.Context(), entryPath)
	if err != nil {
		m.writePluginUnavailable(w, r, entryPath, err)
		return
	}
	_ = service // service 存在性验证

	// 2. 路径规范化：确保始终带前导斜杠，与插件 handler 注册的路径格式一致
	// chi wildcard 返回的 subPath 不带前导斜杠（如 "playlists"），
	// 但插件 handler 注册的路径带前导斜杠（如 "/playlists"）。
	// SDK router 的 split('/').filter(Boolean) 能兼容两种格式，
	// 但显式统一为带前导斜杠可避免潜在的字符串比较问题。
	normalizedPath := subPath
	if normalizedPath != "" && normalizedPath[0] != '/' {
		normalizedPath = "/" + normalizedPath
	}

	// 3. 构建 HTTPRequestData
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPluginBodySize+1))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "read body failed", err.Error())
		return
	}
	if int64(len(body)) > maxPluginBodySize {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large",
			fmt.Sprintf("max %d bytes", maxPluginBodySize))
		return
	}
	reqData := &HTTPRequestData{
		Method:     r.Method,
		Path:       normalizedPath,
		Headers:    flattenHeaders(r.Header),
		Query:      r.URL.RawQuery,
		RemoteAddr: r.RemoteAddr,
	}

	// 当 body 包含非 UTF-8 字节（如 multipart 上传的二进制文件）时，
	// json.Marshal 会将无效 UTF-8 替换为 \ufffd 导致数据损坏。
	// 此时使用 base64 编码透传，JS 侧在调用 onHTTPRequest 前解码。
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	isMultipart := strings.HasPrefix(contentType, "multipart/")

	// multipart 请求始终使用 base64 透传，保持 JS 侧拿到的是「按字节的 latin1 字符串」，
	// 与含二进制 body 的 base64 路径一致。若按直传 UTF-8 路径走，multipart 的文本部分会变成
	// 已解码的 JS 字符串，而 JS 侧 parseMultipartFile 默认按 latin1 字节处理，会对 UTF-8
	// 内容做二次解码导致乱码（单个 .js 音源导入会复现，因为整段 body 通常都是合法 UTF-8）。
	if len(body) > 0 && (isMultipart || !utf8.Valid(body)) {
		reqData.Body = base64.StdEncoding.EncodeToString(body)
		reqData.BodyEncoding = "base64"
		slog.Info("jsplugin-forward: using base64 body encoding",
			"entryPath", entryPath, "path", normalizedPath,
			"multipart", isMultipart,
			"rawLen", len(body), "base64Len", len(reqData.Body))
	} else {
		reqData.Body = string(body)
	}

	// 4. 通过 scheduler.Call 同步调用（等待 JS 处理完）。
	// 默认 30s（scheduler 内部 defaultCallTimeout）。慢端点（如 yt-dlp 展开 radio/mix
	// 歌单，命令本身允许跑到 5min）可通过 X-Plugin-Timeout-Ms 头显式放宽，宿主 clamp 到
	// [defaultCallTimeout, maxCallTimeout]。客户端断连时 r.Context() 会 cancel，长超时
	// 不会导致 worker 无限占用。
	callTimeout := parsePluginCallTimeout(r.Header.Get("X-Plugin-Timeout-Ms"))
	resp, err := m.scheduler.Call(r.Context(), entryPath, "", MsgHTTPRequest, reqData, callTimeout)
	if err != nil {
		writeJSONError(w, http.StatusGatewayTimeout, "plugin call failed", err.Error())
		return
	}

	// 5. 写入 HTTP 响应
	if resp == nil || resp.Data == nil {
		writeJSONError(w, http.StatusInternalServerError, "empty response from plugin", "runtime_error")
		return
	}

	respData, ok := resp.Data.(*HTTPResponseData)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "invalid response type from plugin", "runtime_error")
		return
	}

	// 如果 JS 返回了 serveFile 指令，由 Go 层直接 serve 文件
	if respData.ServeFile != nil {
		m.handleServeFileDirective(w, r, entryPath, respData)
		return
	}

	for k, v := range respData.Headers {
		w.Header().Set(k, v)
	}
	if respData.StatusCode > 0 {
		w.WriteHeader(respData.StatusCode)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	w.Write([]byte(respData.Body))
}

// flattenHeaders 将 http.Header 转为 map[string]string（取第一个值）
func flattenHeaders(h http.Header) map[string]string {
	result := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

// handlePluginFileServe 直接 serve 插件可访问范围内的文件（Go 原生 http.ServeFile）。
// 支持 Range 请求、HTTP 缓存、Content-Type 自动推断。
//
// 路径解析规则：
//   - 不以 "/" 开头 → 相对于插件 data 目录（需 fs 权限）
//   - 以 "/" 开头 → 绝对路径，校验在 fs:external 配置的目录内
//   - "music://" 前缀 → 解析为 music_path 下的相对路径（需 fs:music 权限）
//
// @Summary 插件文件直接访问
// @Description 通过 Go 原生 http.ServeFile 直接返回插件可访问范围内的文件，支持 Range 请求和 HTTP 缓存。
// @Tags JS 插件
// @Produce octet-stream
// @Param entryPath path string true "插件入口标识"
// @Param path path string true "文件路径"
// @Success 200 {file} binary "文件内容"
// @Success 206 {file} binary "部分文件内容（Range 请求）"
// @Failure 404 {string} string "文件不存在或权限不足"
// @Security BearerAuth
// @Router /jsplugin/{entryPath}/files/{path} [get]
// @Router /jsplugin/{entryPath}/files/{path} [head]
func (m *Manager) handlePluginFileServe(w http.ResponseWriter, r *http.Request) {
	entryPath := chi.URLParam(r, "entryPath")
	filePath := chi.URLParam(r, "*")

	if filePath == "" {
		http.NotFound(w, r)
		return
	}

	absPath, err := m.resolveServeFilePath(entryPath, filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, absPath)
}

// handleServeFileDirective 处理 JS 返回的 serveFile 指令，由 Go 层直接 serve 文件。
func (m *Manager) handleServeFileDirective(w http.ResponseWriter, r *http.Request, entryPath string, resp *HTTPResponseData) {
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}

	directive := resp.ServeFile

	if directive.SongID > 0 {
		if !m.pluginHasPermission(entryPath, PermSongsRead) {
			http.Error(w, "permission denied", http.StatusForbidden)
			return
		}
		song, err := m.db.SongRepository().GetByID(r.Context(), directive.SongID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if song.FilePath != "" && song.CueSourcePath == "" {
			http.ServeFile(w, r, song.FilePath)
			return
		}
		// CUE track 或非本地歌曲（remote/radio）：内部转发到 play 端点，
		// 由 SongHandler 处理 CUE 提取/缓存/代理/HLS 等完整播放逻辑。
		playPath := fmt.Sprintf("/api/v1/songs/%d/play", song.ID)
		if song.IsHLSStream() {
			playPath += ".m3u8"
		}
		target := playPath + "?access_token=" + m.pluginToken
		// 清除 chi RouteContext 让 router 重新路由到 play 端点
		cleanCtx := context.WithValue(r.Context(), chi.RouteCtxKey, nil)
		proxyReq, _ := http.NewRequestWithContext(cleanCtx, r.Method, target, nil)
		if rng := r.Header.Get("Range"); rng != "" {
			proxyReq.Header.Set("Range", rng)
		}
		m.router.ServeHTTP(w, proxyReq)
		return
	}

	if directive.FilePath != "" {
		absPath, err := m.resolveServeFilePath(entryPath, directive.FilePath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if directive.SeekSeconds > 0 && m.cacheService != nil {
			if directive.DurationSeconds <= 0 || directive.SeekSeconds < directive.DurationSeconds-3 {
				remaining := float64(0)
				if directive.DurationSeconds > directive.SeekSeconds {
					remaining = directive.DurationSeconds - directive.SeekSeconds
				}
				w.Header().Set("Content-Type", "audio/mpeg")
				w.Header().Set("Cache-Control", "no-store")
				sErr := m.cacheService.StreamSeekedMP3(r.Context(), r.Context(), w, services.SeekStreamOptions{
					SourcePath:      absPath,
					StartSecond:     directive.SeekSeconds,
					RemainingSecond: remaining,
				})
				if sErr == nil {
					return
				}
				if !errors.Is(sErr, services.ErrSeekStreamUnavailable) {
					return
				}
				w.Header().Del("Content-Type")
				w.Header().Del("Cache-Control")
			}
		}

		http.ServeFile(w, r, absPath)
		return
	}

	http.NotFound(w, r)
}

// resolveServeFilePath 解析文件路径并校验权限和安全边界。
// 路径规则：
//   - "music://xxx" → {music_path}/xxx（需 fs:music 权限）
//   - "/absolute/path" → 绝对路径（需 fs:external 权限 + 在配置的目录内）
//   - "relative/path" → {pluginsDataDir}/{entryPath}/relative/path（需 fs 权限）
func (m *Manager) resolveServeFilePath(entryPath, filePath string) (string, error) {
	if strings.Contains(filePath, "..") {
		return "", errors.New("path traversal rejected")
	}

	if strings.HasPrefix(filePath, "music://") {
		if !m.pluginHasPermission(entryPath, PermFSMusic) {
			return "", errors.New("requires fs:music permission")
		}
		musicPath := m.getMusicPath()
		if musicPath == "" {
			return "", errors.New("music_path not configured")
		}
		rel := strings.TrimPrefix(filePath, "music://")
		return resolveInDir(musicPath, rel)
	}

	if filepath.IsAbs(filePath) {
		if !m.pluginHasPermission(entryPath, PermFSExternal) {
			return "", errors.New("requires fs:external permission")
		}
		allowedPaths := m.getPluginExternalPaths(entryPath)
		return resolveInAllowedDirs(allowedPaths, filePath)
	}

	if !m.pluginHasPermission(entryPath, PermFS) {
		return "", errors.New("requires fs permission")
	}
	pluginDir := filepath.Join(m.pluginsDataDir, entryPath)
	return resolveInDir(pluginDir, filePath)
}

// resolveInDir 解析 rel 到 baseDir 下，确保结果不逃出 baseDir。
func resolveInDir(baseDir, rel string) (string, error) {
	absResolved, err := filepath.Abs(filepath.Join(baseDir, rel))
	if err != nil {
		return "", err
	}
	baseDirResolved, _ := filepath.Abs(baseDir)
	sep := string(filepath.Separator)
	if absResolved != baseDirResolved && !strings.HasPrefix(absResolved, baseDirResolved+sep) {
		return "", errors.New("path outside allowed directory")
	}
	info, err := os.Stat(absResolved)
	if err != nil || info.IsDir() {
		return "", errors.New("file not found")
	}
	return absResolved, nil
}

// resolveInAllowedDirs 校验绝对路径在已配置的某个 allowed dir 内。
func resolveInAllowedDirs(allowedDirs []string, absPath string) (string, error) {
	resolved, err := filepath.Abs(absPath)
	if err != nil {
		return "", err
	}
	sep := string(filepath.Separator)
	for _, dir := range allowedDirs {
		dirResolved, _ := filepath.Abs(dir)
		if strings.HasPrefix(resolved, dirResolved+sep) || resolved == dirResolved {
			info, err := os.Stat(resolved)
			if err != nil || info.IsDir() {
				return "", errors.New("file not found")
			}
			return resolved, nil
		}
	}
	return "", errors.New("path not in allowed directories")
}

// pluginHasPermission 检查指定插件是否声明了某权限。
func (m *Manager) pluginHasPermission(entryPath, perm string) bool {
	plugin, err := m.repo.GetByEntryPath(context.Background(), entryPath)
	if err != nil {
		return false
	}
	return CheckPermission(plugin.Permissions, perm)
}

// getMusicPath 从配置表读取 music_path（JSON 格式 {"path":"..."}）。
func (m *Manager) getMusicPath() string {
	cfg, err := m.db.ConfigRepository().Get(context.Background(), "music_path")
	if err != nil {
		return ""
	}
	var data struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(cfg.Value), &data) != nil {
		return cfg.Value
	}
	return data.Path
}

// getPluginExternalPaths 从插件 manifest 声明的 externalPaths 字段读取外部目录配置。
func (m *Manager) getPluginExternalPaths(entryPath string) []string {
	plugin, err := m.repo.GetByEntryPath(context.Background(), entryPath)
	if err != nil {
		return nil
	}
	return plugin.ExternalPaths
}

// RefreshPublicPaths 从 DB 重新加载所有插件的 publicPaths 到内存缓存。
// 在插件安装/卸载/启用/禁用/热重载等生命周期事件后调用，确保 AuthMiddleware 实时感知变更。
func (m *Manager) RefreshPublicPaths() {
	plugins, err := m.repo.GetAll(context.Background())
	if err != nil {
		slog.Warn("RefreshPublicPaths: failed to load plugins", "error", err)
		return
	}

	var prefixes []string
	for _, p := range plugins {
		if len(p.PublicPaths) == 0 {
			continue
		}
		for _, pp := range p.PublicPaths {
			pp = strings.TrimSuffix(pp, "/")
			if pp == "" {
				continue
			}
			prefix := "/api/v1/jsplugin/" + p.EntryPath + pp
			prefixes = append(prefixes, prefix)
		}
	}

	m.mu.Lock()
	m.publicPathPrefixes = prefixes
	m.mu.Unlock()

	if len(prefixes) > 0 {
		slog.Info("public path prefixes refreshed", "count", len(prefixes))
	}
}

// IsPublicPath 检查请求路径是否匹配某个插件的 publicPaths（供 AuthMiddleware 跳过认证）。
func (m *Manager) IsPublicPath(path string) bool {
	m.mu.RLock()
	prefixes := m.publicPathPrefixes
	m.mu.RUnlock()

	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
