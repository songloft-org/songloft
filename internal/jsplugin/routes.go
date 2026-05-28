package jsplugin

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
)

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
}

// RegisterAPIRoutes 注册 JS 插件 API 转发路由（需要认证，由调用方添加 AuthMiddleware）
//
// 路由结构：
//   - HandleFunc /api/v1/jsplugin/{entryPath}/*    → catch-all，处理 API 转发
//
// 分发逻辑：
//  1. 子路径为空（尾部斜杠）→ 服务 static/index.html
//  2. 其他子路径 → 转发给 JS 运行时处理（API 路由）
//
// 注意：chi 的路由优先级机制确保 GET 请求到 /static/* 路径会优先匹配
// RegisterStaticRoutes 中注册的更具体的路由，不会进入此 catch-all。
func (m *Manager) RegisterAPIRoutes(r chi.Router) {
	r.HandleFunc("/api/v1/jsplugin/{entryPath}/*", m.handlePluginAPIRequest)
}

// handlePluginStatic 处理无尾部斜杠的插件根路径请求，
// 直接服务 static/index.html 并注入 <base> 标签，使浏览器正确解析相对路径。
//
//	GET /api/v1/jsplugin/{entryPath} → 直接返回 index.html（含 <base href="...">）
//
// 注意：静态文件服务不依赖插件 JS 运行时是否就绪，只需要数据目录存在即可。
// 这确保了插件初始化期间（onInit 尚未完成）前端页面仍可正常加载。
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
func (m *Manager) handlePluginStaticSubdir(w http.ResponseWriter, r *http.Request) {
	entryPath := chi.URLParam(r, "entryPath")
	m.servePluginStaticFile(w, r, entryPath, "static")
}

// handlePluginStaticSubdirFiles 处理 GET /api/v1/jsplugin/{entryPath}/static/* 请求
// 从磁盘提供静态资源文件，支持 SPA fallback
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
	if _, err := m.EnsureLoaded(r.Context(), entryPath); err != nil {
		m.writePluginUnavailable(w, r, entryPath, err)
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
	} else if strings.HasPrefix(relPath, "static/") {
		relPath = strings.TrimPrefix(relPath, "static/")
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
		content = injectHTMLHead(content, entryPath)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(content)
		return true
	}

	// 其他静态资源：强缓存
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, absFile)
	return true
}

// injectHTMLHead 在 HTML 的 <head> 后（紧跟开标签）注入 <base> 标签和 auth-bridge 脚本。
// <base> 标签使浏览器以 /api/v1/jsplugin/{entryPath}/ 为基准解析相对路径，
// 从而无需 301 重定向即可正确加载资源。
//
// 关键：<base> 必须在所有使用相对 URL 的元素（<link>, <script> 等）之前注入，
// 否则浏览器的预加载扫描器（preload scanner）会在解析到 <base> 之前就用错误的基准 URL
// 发起资源请求，导致首次加载时 CSS/JS 路径错误（如缺少 entryPath 路径段）。
// 刷新后正常是因为资源已缓存或浏览器重新解析时已知 base。
//
// 如果 HTML 中没有 <head> 标签，则在文件开头注入。
func injectHTMLHead(html []byte, entryPath string) []byte {
	baseTag := []byte(`<base href="/api/v1/jsplugin/` + entryPath + `/">`)
	authScript := []byte(authBridgeScriptTpl)
	injectPayload := make([]byte, 0, len(baseTag)+len(authScript))
	injectPayload = append(injectPayload, baseTag...)
	injectPayload = append(injectPayload, authScript...)

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
	body, _ := io.ReadAll(r.Body)
	reqData := &HTTPRequestData{
		Method:  r.Method,
		Path:    normalizedPath,
		Headers: flattenHeaders(r.Header),
		Query:   r.URL.RawQuery,
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

	// 4. 通过 scheduler.Call 同步调用（等待 JS 处理完）
	resp, err := m.scheduler.Call(r.Context(), entryPath, "", MsgHTTPRequest, reqData, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}

	// 5. 写入 HTTP 响应
	if resp == nil || resp.Data == nil {
		http.Error(w, "empty response from plugin", http.StatusInternalServerError)
		return
	}

	respData, ok := resp.Data.(*HTTPResponseData)
	if !ok {
		http.Error(w, "invalid response type from plugin", http.StatusInternalServerError)
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
