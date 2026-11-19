package app

import (
	"bytes"
	"html"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"
)

const webEmbedRoot = "clients/player-build/web-embedded"

// registerWebStatic 注册嵌入的 Lynx Web 前端静态文件服务。
// 精简构建（lite build tag）时 webDist 为空 embed.FS，不挂载根路由，以纯 API 模式运行。
func (a *App) registerWebStatic() {
	// 加 charset=utf-8，防止显式覆盖 .js MIME 时丢掉 charset 导致浏览器用默认编码解码 Unicode 字符为乱码。
	mime.AddExtensionType(".js", "application/javascript; charset=utf-8")
	mime.AddExtensionType(".css", "text/css; charset=utf-8")
	mime.AddExtensionType(".svg", "image/svg+xml")
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".wasm", "application/wasm")
	mime.AddExtensionType(".otf", "font/otf")

	distFS, err := fs.Sub(a.webDist, webEmbedRoot)
	if err != nil {
		slog.Warn("registerWebStatic: fs.Sub 失败，跳过前端挂载", "error", err)
		return
	}

	// 轻量构建：embed 里没有 index.html，根路径返回提示页，其它路径交给 chi 默认 404
	if _, err := fs.Stat(distFS, "index.html"); err != nil {
		slog.Info("registerWebStatic: 未嵌入前端资源，以纯 API 模式运行")
		a.router.Get("/", a.serveLitePage)
		return
	}

	// 读取 index.html 并在需要时注入 base path
	indexBytes, _ := fs.ReadFile(distFS, "index.html")
	if a.config.BasePath != "" {
		indexBytes = bytes.Replace(
			indexBytes,
			[]byte(`<base href="/">`),
			[]byte(`<base href="`+a.config.BasePath+`/">`),
			1,
		)
	}

	// 加载构建时生成的 .br/.gz 预压缩文件
	pfs := newPrecompressedFS(distFS)
	// index.html 经过 base-path 注入时内容已变，需运行时重新压缩（仅此一个文件）
	if a.config.BasePath != "" {
		pfs.addCustomEntry("index.html", indexBytes, ".html")
	}

	fileServer := http.FileServer(http.FS(distFS))
	a.router.Mount("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath := strings.TrimPrefix(r.URL.Path, "/")

		if urlPath == "" {
			w.Header().Set("Cache-Control", "no-cache")
			if !pfs.serve(w, r, "index.html") {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(indexBytes)
			}
			return
		}

		if _, err := fs.Stat(distFS, urlPath); err != nil {
			// 静态资源（带扩展名）找不到时直接 404，避免前端把 HTML 当 JS/JSON 解析
			if strings.Contains(path.Base(urlPath), ".") {
				http.NotFound(w, r)
				return
			}
			// SPA 回退：返回 index.html
			w.Header().Set("Cache-Control", "no-cache")
			if !pfs.serve(w, r, "index.html") {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(indexBytes)
			}
			return
		}

		// Lynx 的入口 bundle、宿主模块和部分引擎脚本使用固定文件名。
		// 每次校验版本，配合预压缩资源的 ETag 避免升级后继续运行旧代码。
		w.Header().Set("Cache-Control", "no-cache")

		// 优先从预压缩缓存返回，未命中（如 woff2 字体）fallback 到 http.FileServer
		if pfs.serve(w, r, urlPath) {
			return
		}
		fileServer.ServeHTTP(w, r)
	}))
}

const litePageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<title>Songloft · 轻量版</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
         max-width: 560px; margin: 12vh auto; padding: 0 24px; color: #2c2c2c; line-height: 1.7; }
  h1 { font-size: 22px; margin-bottom: 8px; }
  .tag { display: inline-block; padding: 2px 8px; border-radius: 4px;
         background: #eef3ff; color: #2456d6; font-size: 12px; vertical-align: middle; margin-left: 8px; }
  p { color: #555; }
  code { background: #f4f4f6; padding: 2px 6px; border-radius: 4px; font-size: 90%; }
  a { color: #2456d6; text-decoration: none; }
  a:hover { text-decoration: underline; }
  .links { margin-top: 24px; }
  .links a { margin-right: 16px; }
</style>
</head>
<body>
  <h1>Songloft 服务正在运行<span class="tag">轻量版</span></h1>
  <p>当前构建未嵌入 Web 前端，仅提供 API 服务。请使用 Songloft 客户端连接本服务器。</p>
  <p>客户端配置服务器地址：<code id="server-addr"></code></p>
  <div class="links">
    <a href="https://github.com/songloft-org/songloft-player-flutter/releases" target="_blank" rel="noopener">下载客户端</a>
    <a href="https://github.com/songloft-org/songloft" target="_blank" rel="noopener">项目主页</a>
  </div>
  <script>
    document.getElementById('server-addr').textContent = window.location.origin + '%s';
  </script>
</body>
</html>`

func (a *App) serveLitePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	page := strings.Replace(litePageHTML, "%s", html.EscapeString(a.config.BasePath), 1)
	_, _ = w.Write([]byte(page))
}
