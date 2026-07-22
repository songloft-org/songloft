package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"songloft/internal/httputil"
	"songloft/internal/services"
)

// RemoteResourceOptions controls the behavior of ServeRemoteResourceWithOptions.
type RemoteResourceOptions struct {
	Timeout      time.Duration
	ErrorStatus  int
	ErrorMessage string
}

// ProxyHandler 通用资源代理处理器
// 用于代理外部资源（图片、音频、视频流等），解决浏览器 CORS 限制
type ProxyHandler struct {
	client *http.Client
}

// NewProxyHandler 创建代理处理器
func NewProxyHandler() *ProxyHandler {
	return &ProxyHandler{
		client: &http.Client{
			Timeout: 60 * time.Second,
			// 不自动跟随重定向，手动处理以透传给客户端
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

// Proxy 代理外部资源
// @Summary 代理外部资源
// @Description 代理外部资源（图片、音频、视频流等），解决浏览器 CORS 限制。支持流式转发、Range 请求透传、Content-Type 透传和域名白名单校验
// @Tags 资源代理
// @Produce application/octet-stream
// @Param url query string true "目标资源的 URL（URL 编码）"
// @Success 200 {file} binary "代理的资源内容"
// @Success 206 {file} binary "部分内容（Range 请求）"
// @Failure 400 {object} map[string]string "缺少 url 参数或 URL 无效"
// @Failure 403 {object} map[string]string "域名不在白名单中"
// @Failure 502 {object} map[string]string "上游请求失败"
// @Security BearerAuth
// @Router /proxy [get]
func (h *ProxyHandler) Proxy(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		respondError(w, http.StatusBadRequest, "缺少 url 参数", nil)
		return
	}

	// 解析并验证目标 URL
	parsed, err := url.Parse(targetURL)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的 URL", err)
		return
	}

	// 仅允许 HTTP/HTTPS 协议
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		respondError(w, http.StatusBadRequest, "仅支持 http/https 协议", nil)
		return
	}

	// 域名白名单校验
	hostname := strings.ToLower(parsed.Hostname())
	if !services.IsHostnameAllowed(hostname) {
		slog.Warn("代理请求被拒绝：域名不在白名单中", "hostname", hostname, "url", targetURL)
		respondError(w, http.StatusForbidden, "该域名不允许代理", nil)
		return
	}

	// 调用通用代理服务（支持 Range、流式转发）
	ServeRemoteResource(w, r, targetURL)
}

// forwardResponseHeaders 透传上游响应的关键头部
func forwardResponseHeaders(w http.ResponseWriter, resp *http.Response) {
	// 需要透传的响应头列表
	headersToForward := []string{
		"Content-Type",
		"Content-Length",
		"Content-Range",
		"Accept-Ranges",
		"Cache-Control",
		"ETag",
		"Last-Modified",
	}

	for _, header := range headersToForward {
		if value := resp.Header.Get(header); value != "" {
			w.Header().Set(header, value)
		}
	}

	// 设置缓存头：对图片资源缓存较长时间
	if w.Header().Get("Cache-Control") == "" {
		contentType := resp.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "image/") {
			w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(86400*7))
		}
	}
}

// ServeRemoteResource 通用远程资源代理服务（公开方法，用于封面、歌词等）
// 使用流式转发，支持 Range 请求，不需要域名校验
//
// 参数:
//   - w: HTTP 响应写入器
//   - r: HTTP 请求(用于 context 和 Range/Accept 头透传)
//   - resourceURL: 目标资源 URL
func ServeRemoteResource(w http.ResponseWriter, r *http.Request, resourceURL string) {
	ServeRemoteResourceWithOptions(w, r, resourceURL, RemoteResourceOptions{
		Timeout:      60 * time.Second,
		ErrorStatus:  http.StatusBadGateway,
		ErrorMessage: "resource fetch failed",
	})
}

// ServeRemoteResourceWithOptions 通用远程资源代理服务，支持按调用场景配置超时和错误状态。
func ServeRemoteResourceWithOptions(w http.ResponseWriter, r *http.Request, resourceURL string, opts RemoteResourceOptions) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	errorStatus := opts.ErrorStatus
	if errorStatus == 0 {
		errorStatus = http.StatusBadGateway
	}
	errorMessage := opts.ErrorMessage
	if errorMessage == "" {
		errorMessage = "resource fetch failed"
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// 构建上游请求
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, nil)
	if err != nil {
		slog.Warn("remote resource request creation failed", "url", resourceURL, "error", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return
	}

	// 处理 Basic Auth
	if upstreamReq.URL.User != nil {
		password, _ := upstreamReq.URL.User.Password()
		upstreamReq.SetBasicAuth(upstreamReq.URL.User.Username(), password)
		upstreamReq.URL.User = nil // 清除以防止泄露
	}

	// 透传客户端的 Range 请求头（支持断点续传、分段加载）
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		upstreamReq.Header.Set("Range", rangeHeader)
	}

	// 设置合理的 User-Agent,避免被上游 CDN 拒绝
	upstreamReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	// 透传 Accept 头
	if accept := r.Header.Get("Accept"); accept != "" {
		upstreamReq.Header.Set("Accept", accept)
	}
	httputil.ApplyBasicAuthFromURL(upstreamReq)

	// 发起请求（走全局 HTTP 代理）
	client := httputil.NewClient(timeout)

	resp, err := client.Do(upstreamReq)
	if err != nil {
		slog.Warn("remote resource fetch failed", "url", resourceURL, "error", err)
		http.Error(w, errorMessage, errorStatus)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("remote resource upstream error", "url", upstreamReq.URL.String(), "status", resp.StatusCode)
	}

	// 透传关键响应头（Content-Type、Content-Range、Accept-Ranges、Cache-Control 等）
	forwardResponseHeaders(w, resp)

	// 透传上游状态码（支持 200、206 Partial Content 等）
	w.WriteHeader(resp.StatusCode)

	// 流式转发响应体
	io.Copy(w, resp.Body)
}

// ServeRemoteResourceWithCache 流式代理上游音频到客户端，并触发后台缓存。
//
// 缓存策略：
//   - 200 OK：TeeReader 同时代理+写临时文件，完成后调 onCached
//   - 206 Partial：正常代理给客户端 + 调 onCacheMiss 触发异步全量下载
//   - 其他状态码 / 流中断：不缓存
func ServeRemoteResourceWithCache(
	w http.ResponseWriter,
	r *http.Request,
	resourceURL string,
	headers map[string]string,
	onCached func(tmpPath, contentType string),
	onCacheMiss func(),
) {
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, resourceURL, nil)
	if err != nil {
		slog.Warn("remote resource request creation failed", "url", resourceURL, "error", err)
		http.Error(w, "resource fetch failed", http.StatusInternalServerError)
		return
	}

	if upstreamReq.URL.User != nil {
		password, _ := upstreamReq.URL.User.Password()
		upstreamReq.SetBasicAuth(upstreamReq.URL.User.Username(), password)
		upstreamReq.URL.User = nil
	}

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		upstreamReq.Header.Set("Range", rangeHeader)
	}

	upstreamReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if accept := r.Header.Get("Accept"); accept != "" {
		upstreamReq.Header.Set("Accept", accept)
	}
	// 应用插件返回的自定义头(如 Referer / User-Agent)，覆盖默认值
	for k, v := range headers {
		upstreamReq.Header.Set(k, v)
	}
	httputil.ApplyBasicAuthFromURL(upstreamReq)

	// 流式播放代理不能用「整请求硬超时」的 client：客户端（尤其小爱音箱等硬件播放器）
	// 是按播放进度渐进地读代理流，慢读时上游连接会被 120s 整请求超时提前掐断，只送到
	// 约 2~3 分钟音频，音箱缓冲耗尽后从头重拉同一首、随后被本地切歌定时器提前切下一首
	// （songloft-org/songloft-plugin-miot#55）。改用 streaming client：无整请求超时，
	// 仅用 ResponseHeaderTimeout 防坏源永久转圈；客户端断开由 r.Context() 取消上游请求。
	// 与 serveRadio 的处理保持一致。
	client := httputil.NewStreamingClient()

	resp, err := client.Do(upstreamReq)
	if err != nil {
		slog.Warn("remote resource fetch failed", "url", resourceURL, "error", err)
		http.Error(w, "resource fetch failed", http.StatusBadGateway)
		if r.Context().Err() != nil && onCacheMiss != nil {
			go onCacheMiss()
		}
		return
	}
	defer resp.Body.Close()

	forwardResponseHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)

	switch resp.StatusCode {
	case http.StatusOK:
		contentType := resp.Header.Get("Content-Type")
		ext := services.GetExtFromContentType(contentType)
		tmpFile, tmpErr := os.CreateTemp("", "songloft-proxy-cache-*"+ext)
		if tmpErr != nil {
			io.Copy(w, resp.Body)
			return
		}
		tmpPath := tmpFile.Name()

		tee := io.TeeReader(resp.Body, tmpFile)
		written, copyErr := io.Copy(w, tee)
		tmpFile.Close()

		if copyErr == nil && written >= services.MinAudioSize {
			go onCached(tmpPath, contentType)
		} else {
			os.Remove(tmpPath)
			if copyErr != nil && r.Context().Err() != nil && onCacheMiss != nil {
				go onCacheMiss()
			}
		}
	case http.StatusPartialContent:
		io.Copy(w, resp.Body)
		if onCacheMiss != nil {
			go onCacheMiss()
		}
	default:
		slog.Warn("remote resource upstream error", "url", upstreamReq.URL.String(), "status", resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
