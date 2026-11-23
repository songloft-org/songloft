package services

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"songloft/internal/httputil"
)

// hlsPullGateway 只监听回环地址，用随机能力路径把每个媒体引用转为受控 HTTP 拉流。
// ffmpeg 禁止 file/https/udp 等输入协议；HLS 的所有 URI 都由此处改写与校验。
type hlsPullGateway struct {
	ctx       context.Context
	cancel    context.CancelFunc
	server    *http.Server
	listener  net.Listener
	client    *http.Client
	transport *http.Transport
	opts      HLSTranscodeOptions
	allowlist func() []string
	base      string
	mu        sync.Mutex
	err       error
	pending   *liveFLVPull
	liveFLV   atomic.Bool
}

type liveFLVPull struct {
	rangeHeader string
	cancel      context.CancelFunc
	response    chan *http.Response
}

func validateHLSURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" || len(raw) > 8192 {
		return hlsError(400, "仅支持不含用户凭证和片段的 HTTP/HTTPS URL")
	}
	return nil
}

func newHLSPullGateway(ctx context.Context, opts HLSTranscodeOptions, allowlist func() []string) (*hlsPullGateway, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, hlsError(503, "无法初始化直播拉流服务")
	}
	var key [24]byte
	if _, err := rand.Read(key[:]); err != nil {
		listener.Close()
		return nil, err
	}
	g := &hlsPullGateway{listener: listener, opts: opts, allowlist: allowlist,
		base: "http://" + listener.Addr().String() + "/" + hex.EncodeToString(key[:]) + "/"}
	if err := g.initSourceClient(); err != nil {
		listener.Close()
		return nil, err
	}
	g.ctx, g.cancel = context.WithCancel(ctx)
	g.server = &http.Server{Handler: http.HandlerFunc(g.serve), ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(net.Listener) context.Context { return g.ctx }}
	go func() { _ = g.server.Serve(listener) }()
	return g, nil
}

// initSourceClient also serves short radio probes, which do not need a gateway listener.
func (g *hlsPullGateway) initSourceClient() error {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // 不读取环境变量代理；仅使用 Songloft 明确配置的代理。
	transport.ResponseHeaderTimeout = 15 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.MaxResponseHeaderBytes = 1 << 20
	transport.DisableCompression = true
	transport.DialContext = g.dial
	if proxyRaw := httputil.GetGlobalProxy(); proxyRaw != "" {
		proxy, err := url.Parse(proxyRaw)
		if err != nil || (proxy.Scheme != "http" && proxy.Scheme != "https") {
			return hlsError(503, "直播转码仅支持 HTTP/HTTPS 上游代理")
		}
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			if net.ParseIP(req.URL.Hostname()) != nil && net.ParseIP(req.URL.Hostname()).IsLoopback() {
				return nil, nil
			}
			return proxy, nil
		}
		// 代理是管理员配置的可信连接目标；媒体目的地址仍在每次请求前校验。
		transport.DialContext = (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	}
	g.transport = transport
	g.client = &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return g.check(req.Context(), req.URL)
	}}
	return nil
}

func (g *hlsPullGateway) URL(raw string) string {
	// 固定媒体后缀帮助 FFmpeg 识别 HLS；实际内容仍由响应体识别。
	suffix := "resource.ts"
	if u, err := url.Parse(raw); err == nil {
		switch ext := strings.ToLower(path.Ext(u.Path)); ext {
		case ".m3u8", ".ts", ".mp4", ".m4s", ".aac", ".key", ".flv":
			suffix = "resource" + ext
		}
	}
	return g.base + base64.RawURLEncoding.EncodeToString([]byte(raw)) + "/" + suffix
}

func (g *hlsPullGateway) Close() {
	g.cancel()
	_ = g.server.Close()
	g.transport.CloseIdleConnections()
}

func (g *hlsPullGateway) failure(message string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.err != nil {
		return g.err
	}
	return hlsError(502, message)
}

func (g *hlsPullGateway) remember(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.err == nil {
		g.err = err
	}
}

func (g *hlsPullGateway) allowedIPs(ctx context.Context, host string) ([]net.IP, error) {
	host = strings.ToLower(host)
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".local") {
		return nil, hlsError(403, "直播源地址不允许访问")
	}
	var entries []string
	if g.allowlist != nil {
		entries = g.allowlist()
	}
	allowlist, err := ParseAllowlist(entries)
	if err != nil {
		return nil, hlsError(403, "私网代理白名单无效")
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, hlsError(502, "无法解析直播源地址")
	}
	for _, ip := range ips {
		if ip.IsUnspecified() || ip.IsMulticast() || (isPrivateIP(ip) && !allowlistContains(allowlist, ip)) {
			return nil, hlsError(403, "直播源地址不在允许范围内")
		}
	}
	return ips, nil
}

func (g *hlsPullGateway) check(ctx context.Context, u *url.URL) error {
	if err := validateHLSURL(u.String()); err != nil {
		return err
	}
	_, err := g.allowedIPs(ctx, u.Hostname())
	if err != nil {
		g.remember(err)
	}
	return err
}

func (g *hlsPullGateway) dial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := g.allowedIPs(ctx, host)
	if err != nil {
		g.remember(err)
		return nil, err
	}
	var last error
	for _, ip := range ips {
		conn, err := (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	return nil, last
}

// FFmpeg may still be encoding buffered packets after the gateway reads upstream EOF.
// Open the next connection during that time, using the station entry for a fresh redirect.
func (g *hlsPullGateway) preopenFLV(req *http.Request) {
	if g.ctx.Err() != nil {
		return
	}
	g.mu.Lock()
	if g.pending != nil {
		g.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(g.ctx)
	pull := &liveFLVPull{rangeHeader: req.Header.Get("Range"), cancel: cancel, response: make(chan *http.Response)}
	g.pending = pull
	g.mu.Unlock()

	slog.Debug("live FLV upstream ended, preparing the next pull")
	go func() {
		defer close(pull.response)
		next := req.Clone(ctx)
		if err := g.check(ctx, next.URL); err != nil {
			cancel()
			return
		}
		resp, err := g.client.Do(next)
		if err != nil {
			cancel()
			return
		}
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case pull.response <- resp:
			// The receiving gateway request owns the body and cancellation.
		case <-timer.C:
			cancel()
			resp.Body.Close()
		case <-ctx.Done():
			resp.Body.Close()
		}
	}()
}

func (g *hlsPullGateway) takeFLVPull(req *http.Request) (*http.Response, func()) {
	g.mu.Lock()
	pull := g.pending
	g.pending = nil
	g.mu.Unlock()
	if pull == nil {
		return nil, nil
	}
	if pull.rangeHeader != req.Header.Get("Range") {
		pull.cancel()
		return nil, nil
	}
	stop := context.AfterFunc(req.Context(), pull.cancel)
	release := func() {
		stop()
		pull.cancel()
	}
	select {
	case resp := <-pull.response:
		if resp != nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent) {
			return resp, release
		}
		if resp != nil {
			resp.Body.Close()
		}
	case <-req.Context().Done():
	}
	release()
	return nil, nil
}

func (g *hlsPullGateway) serve(w http.ResponseWriter, r *http.Request) {
	prefix := strings.TrimPrefix(g.base, "http://"+g.listener.Addr().String())
	if !strings.HasPrefix(r.URL.Path, prefix) || (r.Method != "GET" && r.Method != "HEAD") {
		http.NotFound(w, r)
		return
	}
	encoded, _, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, prefix), "/")
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if !ok || err != nil {
		http.NotFound(w, r)
		return
	}
	u, err := url.Parse(string(raw))
	if err != nil {
		http.Error(w, "invalid source", 400)
		return
	}
	if err := g.check(r.Context(), u); err != nil {
		g.remember(err)
		http.Error(w, "source denied", HLSTranscodeStatus(err))
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), nil)
	if err != nil {
		http.Error(w, "invalid source", 400)
		return
	}
	req.Header.Set("User-Agent", g.opts.UserAgent)
	if g.opts.Referer != "" {
		req.Header.Set("Referer", g.opts.Referer)
	}
	if r.Header.Get("Range") != "" {
		req.Header.Set("Range", r.Header.Get("Range"))
	}
	root := string(raw) == g.opts.URL
	var resp *http.Response
	if root && g.opts.preopenLiveFLV && r.Method == http.MethodGet {
		var release func()
		resp, release = g.takeFLVPull(req)
		if release != nil {
			defer release()
		}
	}
	if resp == nil {
		resp, err = g.client.Do(req)
	}
	if err != nil {
		http.Error(w, "upstream unavailable", 502)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		g.remember(hlsError(502, "直播源请求失败"))
		http.Error(w, "upstream rejected request", 502)
		return
	}
	if r.Method == "HEAD" {
		for _, key := range []string{"Content-Type", "Content-Length", "Accept-Ranges"} {
			if value := resp.Header.Get(key); value != "" {
				w.Header().Set(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		return
	}
	reader := bufio.NewReader(resp.Body)
	first, _ := reader.Peek(512)
	first = bytes.TrimSpace(bytes.TrimPrefix(first, []byte{0xef, 0xbb, 0xbf}))
	isFLV := bytes.HasPrefix(first, []byte("FLV"))
	liveFLV := isFLV && resp.ContentLength < 0
	if root {
		g.liveFLV.Store(liveFLV)
	}
	declaredPlaylist := strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") || strings.HasSuffix(strings.ToLower(resp.Request.URL.Path), ".m3u8")
	if bytes.HasPrefix(first, []byte("#EXTM3U")) || (declaredPlaylist && !isFLV) {
		data, err := io.ReadAll(io.LimitReader(reader, (1<<20)+1))
		if err != nil || len(data) > 1<<20 {
			g.remember(hlsError(502, "直播播放列表过大或读取失败"))
			http.Error(w, "bad playlist", 502)
			return
		}
		data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
		if !bytes.HasPrefix(data, []byte("#EXTM3U")) {
			g.remember(hlsError(502, "无效的 HLS 播放列表"))
			http.Error(w, "bad playlist", 502)
			return
		}
		data, err = g.rewrite(data, resp.Request.URL)
		if err != nil {
			g.remember(err)
			http.Error(w, "unsupported playlist", HLSTranscodeStatus(err))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write(data)
		return
	}
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if value := resp.Header.Get(key); value != "" {
			w.Header().Set(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, copyErr := io.Copy(&flushingWriter{w: w}, reader)
	if copyErr == nil && root && liveFLV && g.opts.preopenLiveFLV && r.Context().Err() == nil {
		g.preopenFLV(req)
	}
}

var hlsPullURI = regexp.MustCompile(`URI="([^"]*)"`)

func (g *hlsPullGateway) rewrite(data []byte, base *url.URL) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	resolve := func(ref string) (string, error) {
		u, err := url.Parse(ref)
		if err != nil || strings.Contains(ref, "{$") {
			return "", hlsError(422, "不支持的 HLS 资源引用")
		}
		absolute := base.ResolveReference(u)
		if err := g.check(g.ctx, absolute); err != nil {
			return "", err
		}
		return g.URL(absolute.String()), nil
	}
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			replacement, err := resolve(line)
			if err != nil {
				return nil, err
			}
			lines[i] = replacement
			continue
		}
		// 变量替换、内容导向和外部 JSON 引用不属于首版的普通 HLS 输入。
		if strings.HasPrefix(line, "#EXT-X-DEFINE:") || strings.HasPrefix(line, "#EXT-X-CONTENT-STEERING:") {
			return nil, hlsError(422, "不支持此 HLS 播放列表扩展")
		}
		var rewriteErr error
		lines[i] = hlsPullURI.ReplaceAllStringFunc(line, func(match string) string {
			ref := hlsPullURI.FindStringSubmatch(match)[1]
			replacement, err := resolve(ref)
			if err != nil {
				rewriteErr = err
				return match
			}
			return `URI="` + replacement + `"`
		})
		if rewriteErr != nil {
			return nil, rewriteErr
		}
		if strings.Contains(hlsPullURI.ReplaceAllString(line, ""), "URI=") {
			return nil, hlsError(422, "无效的 HLS URI 属性")
		}
	}
	return []byte(strings.Join(lines, "\n")), nil
}
