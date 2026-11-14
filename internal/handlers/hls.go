package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"songloft/internal/httputil"
	"songloft/internal/models"
	"songloft/internal/services"

	"github.com/go-chi/chi/v5"
)

const (
	// maxPlaylistBytes m3u8 响应体上限。HLS 主播放列表 + 媒体播放列表都不会超过这个值；
	// 设上限防止恶意上游返回巨型文件耗尽内存。
	maxPlaylistBytes = 1 * 1024 * 1024

	// hlsContentType m3u8 标准 MIME。
	hlsContentType = "application/vnd.apple.mpegurl"

	// streamUserAgent 浏览器风格 UA，仅用于 HLS 反代（hls.go）：反代是替浏览器拉 m3u8/切片
	// 解决 CORS，模拟浏览器请求头对少数按 UA 校验的 HLS 源更稳。
	streamUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36 Songloft/1.0"

	// radioStreamUserAgent 直连电台/Shoutcast 流（serveRadio）专用的媒体播放器风格 UA。
	// 绝不能用浏览器 UA：streamtheworld 等大量 Shoutcast/CDN 电台带防盗链，检测到浏览器 UA
	// 会只回一段约 32KB 的预览音频就断流（约 3 秒），迫使网页改用其官方播放器嵌入。
	// 这会让所有经本机代理播放的客户端（web/桌面/小爱音箱）都在约 3 秒后断开（songloft#275）。
	// Songloft/1.0 是 2.10.0 之前一直在用、经真实电台验证可长播的值。
	radioStreamUserAgent = "Songloft/1.0"

	streamAccept = "*/*"
	hlsAccept    = "application/vnd.apple.mpegurl, application/x-mpegURL, application/x-mpegurl, */*"
)

var (
	errNotM3U8      = errors.New("not a valid m3u8: missing #EXTM3U")
	errEmptyContent = errors.New("empty m3u8 content")
)

// hlsAttr 单个 HLS 属性键值对。保留原顺序与引号风格，便于"原样透传 + 仅改 URI"。
type hlsAttr struct {
	Key    string
	Value  string
	Quoted bool
}

// parseAttrLine 解析 attribute list 形式的 HLS 行。
//
// 输入: `#EXT-X-MEDIA:TYPE=AUDIO,URI="a.m3u8",NAME="en"`
// 返回 tag="EXT-X-MEDIA", attrs=[{TYPE,AUDIO,false},{URI,a.m3u8,true},{NAME,en,true}]
//
// 对 `#EXTINF:5.0,title` 这种非 attribute list 形式，attrs 返回 nil。
// 对 `#EXTM3U` 这种无冒号行，attrs 也返回 nil。
func parseAttrLine(line string) (tag string, attrs []hlsAttr) {
	if !strings.HasPrefix(line, "#") {
		return "", nil
	}
	before, rest, ok := strings.Cut(line, ":")
	if !ok {
		return strings.TrimPrefix(line, "#"), nil
	}
	tag = strings.TrimPrefix(before, "#")

	parsed := parseAttrList(rest)
	// 全部 attr 都没有 '='（如 #EXTINF:5.0,title）→ 非 attribute list，返回 nil
	hasEq := false
	for _, a := range parsed {
		if a.Key != "" && (a.Value != "" || a.Quoted) {
			hasEq = true
			break
		}
	}
	if !hasEq {
		return tag, nil
	}
	return tag, parsed
}

// parseAttrList 按"逗号在引号外"切分属性列表。
// 这是手写 HLS 解析最容易出错的点：CODECS="avc1.42c01e,mp4a.40.2" 引号内的逗号不能切。
func parseAttrList(s string) []hlsAttr {
	var out []hlsAttr
	n := len(s)
	i := 0
	for i < n {
		// 跳过前导空白
		for i < n && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}
		// 读 key 到 '=' 或 ','
		keyStart := i
		for i < n && s[i] != '=' && s[i] != ',' {
			i++
		}
		key := strings.TrimSpace(s[keyStart:i])
		if i < n && s[i] == ',' {
			if key != "" {
				out = append(out, hlsAttr{Key: key})
			}
			i++
			continue
		}
		if i >= n {
			if key != "" {
				out = append(out, hlsAttr{Key: key})
			}
			break
		}
		i++ // skip '='
		// 读 value
		quoted := false
		var val string
		if i < n && s[i] == '"' {
			quoted = true
			i++
			start := i
			for i < n && s[i] != '"' {
				i++
			}
			val = s[start:i]
			if i < n {
				i++ // skip closing quote
			}
		} else {
			start := i
			for i < n && s[i] != ',' {
				i++
			}
			val = strings.TrimSpace(s[start:i])
		}
		out = append(out, hlsAttr{Key: key, Value: val, Quoted: quoted})
		if i < n && s[i] == ',' {
			i++
		}
	}
	return out
}

// formatAttrLine 把 (tag, attrs) 重新拼回 HLS 行，保留原始属性顺序与引号风格。
func formatAttrLine(tag string, attrs []hlsAttr) string {
	var b strings.Builder
	b.WriteByte('#')
	b.WriteString(tag)
	if len(attrs) == 0 {
		return b.String()
	}
	b.WriteByte(':')
	for i, a := range attrs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(a.Key)
		if a.Value == "" && !a.Quoted {
			continue
		}
		b.WriteByte('=')
		if a.Quoted {
			b.WriteByte('"')
			b.WriteString(a.Value)
			b.WriteByte('"')
		} else {
			b.WriteString(a.Value)
		}
	}
	return b.String()
}

// isSameOrigin 比较 scheme + hostname(小写) + port。
func isSameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) &&
		a.Port() == b.Port()
}

// looksLikePlaylist 判断 URL path 后缀是否为 .m3u8 / .m3u（忽略 query）。
// 与 music.go:isHLSURL 同义，独立实现避免循环 + 便于 hls_test 单测。
func looksLikePlaylist(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	return ext == ".m3u8" || ext == ".m3u"
}

func refLooksLikePlaylist(ref string, base *url.URL) bool {
	refURL, err := url.Parse(ref)
	if err != nil {
		return false
	}
	return looksLikePlaylist(base.ResolveReference(refURL).String())
}

// rewriteRef 解析 HTTP(S) ref（可能是相对 URL）为绝对 URL，再调签名改写回调。
// 非 HTTP(S)（如 data: / skd:）或解析失败时原样返回 ref。
func rewriteRef(ref string, base *url.URL, rewrite func(absURL string, isPlaylist bool) string, isPlaylist bool) string {
	if ref == "" {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	abs := base.ResolveReference(refURL)
	if (abs.Scheme != "http" && abs.Scheme != "https") || abs.Hostname() == "" {
		return ref
	}
	return rewrite(abs.String(), isPlaylist)
}

// rewriteAttrTagURI 在 attribute list 标签行里替换 URI 类属性，其它属性保持原样。
func rewriteAttrTagURI(line string, base *url.URL, rewrite func(string, bool) string) string {
	tag, attrs := parseAttrLine(line)
	if attrs == nil {
		return line
	}
	changed := false
	for idx, a := range attrs {
		var isPlaylist bool
		var handle bool
		switch tag {
		case "EXT-X-KEY", "EXT-X-SESSION-KEY", "EXT-X-MAP",
			"EXT-X-PRELOAD-HINT", "EXT-X-PART", "EXT-X-SESSION-DATA":
			if a.Key == "URI" {
				handle, isPlaylist = true, false
			}
		case "EXT-X-MEDIA", "EXT-X-RENDITION-REPORT", "EXT-X-I-FRAME-STREAM-INF":
			if a.Key == "URI" {
				handle, isPlaylist = true, true
			}
		case "EXT-X-DATERANGE":
			if a.Key == "X-ASSET-URI" {
				refURL, err := url.Parse(a.Value)
				if err != nil {
					continue
				}
				abs := base.ResolveReference(refURL).String()
				handle, isPlaylist = true, looksLikePlaylist(abs)
			}
			// X-ASSET-LIST 暂不改写：JSON 子代理在 MVP 之外，原样透传 fail-open 到客户端直连
		}
		if !handle {
			continue
		}
		newVal := rewriteRef(a.Value, base, rewrite, isPlaylist)
		if newVal != a.Value {
			attrs[idx].Value = newVal
			changed = true
		}
	}
	if !changed {
		return line
	}
	return formatAttrLine(tag, attrs)
}

// rewriteM3U8 改写 m3u8 内容中所有 URI 指向本机代理。
//
// base: 上游 m3u8 的最终 URL（解析相对 URL 用，注意若上游 redirect 需用 resp.Request.URL）
// rewrite: URI 改写回调；absURL 是绝对 URL，isPlaylist=true 表示该 URI 指向 m3u8
//
// 跨源 HTTP(S) URL 一并改写；生产回调签名，端点校验签名并独立拦截私网。
// 未识别标签原样透传（HLS 草案每 6 个月迭代，未来标签不应导致播放失败）。
func rewriteM3U8(content []byte, base *url.URL, rewrite func(absURL string, isPlaylist bool) string) ([]byte, error) {
	// 跳过 UTF-8 BOM
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	if len(content) == 0 {
		return nil, errEmptyContent
	}

	lines := splitLines(content)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "#EXTM3U" {
		return nil, errNotM3U8
	}

	var out bytes.Buffer
	out.Grow(len(content) + 256)

	streamURLPending := false
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)

		// STREAM-INF 紧跟的下一行是 playlist URL（HLS spec 强制紧邻）
		if streamURLPending {
			streamURLPending = false
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				out.WriteString(rewriteRef(trimmed, base, rewrite, true))
				out.WriteByte('\n')
				continue
			}
			// 不规范：STREAM-INF 后不是 URL 行。原样输出，下面继续处理
		}

		switch {
		case strings.HasPrefix(trimmed, "#EXT-X-STREAM-INF"):
			streamURLPending = true
			out.WriteString(raw)
			out.WriteByte('\n')

		case strings.HasPrefix(trimmed, "#EXT-X-I-FRAME-STREAM-INF:"),
			strings.HasPrefix(trimmed, "#EXT-X-MEDIA:"),
			strings.HasPrefix(trimmed, "#EXT-X-RENDITION-REPORT:"),
			strings.HasPrefix(trimmed, "#EXT-X-KEY:"),
			strings.HasPrefix(trimmed, "#EXT-X-SESSION-KEY:"),
			strings.HasPrefix(trimmed, "#EXT-X-MAP:"),
			strings.HasPrefix(trimmed, "#EXT-X-PRELOAD-HINT:"),
			strings.HasPrefix(trimmed, "#EXT-X-PART:"),
			strings.HasPrefix(trimmed, "#EXT-X-SESSION-DATA:"),
			strings.HasPrefix(trimmed, "#EXT-X-DATERANGE:"):
			out.WriteString(rewriteAttrTagURI(raw, base, rewrite))
			out.WriteByte('\n')

		case strings.HasPrefix(trimmed, "#") || trimmed == "":
			out.WriteString(raw)
			out.WriteByte('\n')

		default:
			// 裸 URL 行：通常是媒体切片；少数源会不规范地把子 m3u8 直接放在裸行里。
			out.WriteString(rewriteRef(trimmed, base, rewrite, refLooksLikePlaylist(trimmed, base)))
			out.WriteByte('\n')
		}
	}

	return out.Bytes(), nil
}

// ============================================================
// HLSHandler: 反向代理 m3u8 + 切片
// ============================================================

// hlsProxyConfigKey 是 HLS 反代开关在 configs 表中的 key。
// 业务封装（IsEnabled / SetEnabled / GetProxySetting / UpdateProxySetting）是唯一访问入口，
// 通用 /api/v1/configs/{key} 不预置此 key，避免双入口造成不一致。
const hlsProxyConfigKey = "hls_proxy_enabled"

// HLSHandler 处理 /api/v1/songs/{id}/hls/{playlist,segment} 端点，并暴露 /settings/hls-proxy
// 业务化开关 API。
//
// 当 hls 代理开启时由 serveRadio 直调 ServeProxy；
// player 拉到的 m3u8 内 URL 全部指向本机端点，后续切片/key/init 由 player 自行回访。
type HLSHandler struct {
	songService   *services.SongService
	configService *services.ConfigService
	client        *http.Client
	allowHost     func(host string) bool // 默认 services.IsHostnameAllowed；测试可替换
	signingKey    string                 // 进程内随机密钥；重启后重新获取播放列表
}

// NewHLSHandler 构造 HLSHandler。
// client 无整请求 Timeout：直播切片可能持续数十秒至数分钟，传输过程由客户端断连+r.Context() 取消接管。
// Transport 只设置 ResponseHeaderTimeout，避免坏源在建连后迟迟不回响应头导致播放协程一直挂起。
// configService 可为 nil（测试场景下不走 IsEnabled 判定时），此时 IsEnabled 返回 false。
func NewHLSHandler(songService *services.SongService, configService *services.ConfigService) *HLSHandler {
	client := httputil.NewStreamingClient()
	h := &HLSHandler{
		songService:   songService,
		configService: configService,
		client:        client,
		allowHost:     services.IsHostnameAllowed,
		signingKey:    rand.Text(),
	}
	client.CheckRedirect = h.checkRedirect
	return h
}

// IsEnabled 返回 HLS 反代开关当前状态。默认 false（关闭）：
// 反代会把所有切片字节走本机带宽，需用户在源站防盗链/CORS 阻塞时再手动开启。
func (h *HLSHandler) IsEnabled() bool {
	if h.configService == nil {
		return false
	}
	return h.configService.GetBool(hlsProxyConfigKey, false)
}

// SetEnabled 持久化 HLS 反代开关。值以 "true"/"false" 字符串存储，
// 与 ConfigService.GetBool 的 strconv.ParseBool 解析对齐。
func (h *HLSHandler) SetEnabled(enabled bool) error {
	if h.configService == nil {
		return fmt.Errorf("configService 未注入，无法持久化 hls 代理开关")
	}
	value := "false"
	if enabled {
		value = "true"
	}
	return h.configService.Set(hlsProxyConfigKey, value)
}

// hlsProxySettingRequest /settings/hls-proxy PUT 请求体。
type hlsProxySettingRequest struct {
	Enabled bool `json:"enabled"`
}

// GetProxySetting 处理 GET /api/v1/settings/hls-proxy
// @Summary 获取 HLS 代理开关
// @Description 获取“HLS 电台流通过本机反代回客户端”开关的当前状态
// @Tags 电台与 HLS
// @Produce json
// @Success 200 {object} map[string]bool "返回 enabled 字段表示开关状态"
// @Security BearerAuth
// @Router /settings/hls-proxy [get]
func (h *HLSHandler) GetProxySetting(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": h.IsEnabled()})
}

// UpdateProxySetting 处理 PUT /api/v1/settings/hls-proxy
// @Summary 更新 HLS 代理开关
// @Description 开启/关闭 HLS 反代。开启时电台切片字节全部经本机转发（解决源站 Referer/CORS 拦截），关闭时仅 302 给 player。
// @Tags 电台与 HLS
// @Accept json
// @Produce json
// @Param request body hlsProxySettingRequest true "开关请求"
// @Success 200 {object} map[string]bool "返回 enabled 字段表示更新后的开关状态"
// @Failure 400 {object} map[string]string "请求格式错误"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/hls-proxy [put]
func (h *HLSHandler) UpdateProxySetting(w http.ResponseWriter, r *http.Request) {
	var req hlsProxySettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if err := h.SetEnabled(req.Enabled); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": req.Enabled})
}

// ServeProxy 是 serveRadio 在 hls_proxy_mode=proxy 时的入口：直接代理 song.URL 作为顶层 m3u8。
// 后续子 m3u8 / 切片由 player 通过 HandlePlaylist / HandleSegment 端点回访。
//
// 顶层入口的当前请求 URL 是 /api/v1/songs/{id}/play，player 解析 m3u8 内的相对 URL 时
// 会按 RFC 3986 替换最后一段 path，因此改写后的相对路径必须带 "hls/" 前缀，
// 否则解析结果是 /api/v1/songs/{id}/playlist 而非 /api/v1/songs/{id}/hls/playlist。
func (h *HLSHandler) ServeProxy(w http.ResponseWriter, r *http.Request, song *models.Song) {
	h.servePlaylist(w, r, song, song.URL, "hls/")
}

// HandlePlaylist 处理 GET/HEAD /api/v1/songs/{id}/hls/playlist?u=<base64url>
//
// 子层入口的当前请求 URL 已位于 /api/v1/songs/{id}/hls/playlist，相对路径无需前缀，
// 解析后自动落到同目录 /api/v1/songs/{id}/hls/{playlist,segment}。
// @Summary 反代 HLS 子层 m3u8
// @Description HLS 反代开启时拉取并改写子 m3u8。同源 URL 可直接请求；跨源 URL 必须使用服务端从上游播放列表生成的签名链接。入口及每次重定向均禁止私网地址。
// @Tags 电台与 HLS
// @Produce application/vnd.apple.mpegurl
// @Param id path int true "歌曲 ID"
// @Param u query string true "上游 m3u8 URL（base64url 编码）"
// @Param sig query string false "服务端生成的资源签名（跨源必填，绑定歌曲及 playlist 端点）"
// @Success 200 {string} string "改写后的 m3u8 文本"
// @Failure 400 {object} map[string]string "song_id 或 u 参数无效"
// @Failure 403 {object} map[string]string "跨源 URL 缺少有效签名、签名无效或目标主机不允许"
// @Failure 404 {string} string "歌曲不存在"
// @Failure 502 {object} map[string]string "上游不可用"
// @Security BearerAuth
// @Router /songs/{id}/hls/playlist [get]
func (h *HLSHandler) HandlePlaylist(w http.ResponseWriter, r *http.Request) {
	song, upstreamURL, ok := h.resolveEndpoint(w, r)
	if !ok {
		return
	}
	h.servePlaylist(w, r, song, upstreamURL, "")
}

// HandleSegment 处理 GET/HEAD /api/v1/songs/{id}/hls/segment?u=<base64url>
// @Summary 反代 HLS 切片 / key / init 段
// @Description 反代播放列表中引用的切片、key、init 段，透传 Range 请求。跨源 URL 必须使用服务端生成且绑定歌曲及 segment 端点的签名链接；入口及每次重定向均禁止私网地址。
// @Tags 电台与 HLS
// @Produce application/octet-stream
// @Param id path int true "歌曲 ID"
// @Param u query string true "上游切片 URL（base64url 编码）"
// @Param sig query string false "服务端生成的资源签名（跨源必填）"
// @Success 200 {file} binary "切片二进制内容"
// @Success 206 {file} binary "Range 请求的部分内容"
// @Failure 400 {object} map[string]string "song_id 或 u 参数无效"
// @Failure 403 {object} map[string]string "跨源 URL 缺少有效签名、签名无效或目标主机不允许"
// @Failure 404 {string} string "歌曲不存在"
// @Failure 502 {object} map[string]string "上游不可用"
// @Security BearerAuth
// @Router /songs/{id}/hls/segment [get]
func (h *HLSHandler) HandleSegment(w http.ResponseWriter, r *http.Request) {
	song, upstreamURL, ok := h.resolveEndpoint(w, r)
	if !ok {
		return
	}
	h.serveSegment(w, r, song, upstreamURL)
}

// resolveEndpoint 共用：取 song id → 加载 song → base64 解码 u。
func (h *HLSHandler) resolveEndpoint(w http.ResponseWriter, r *http.Request) (*models.Song, string, bool) {
	idStr := chi.URLParam(r, "id")
	songID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || songID <= 0 {
		respondError(w, http.StatusBadRequest, "无效的 song_id", err)
		return nil, "", false
	}

	encoded := r.URL.Query().Get("u")
	if encoded == "" {
		respondError(w, http.StatusBadRequest, "缺少 u 参数", nil)
		return nil, "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		respondError(w, http.StatusBadRequest, "u 参数解码失败", err)
		return nil, "", false
	}

	song, err := h.songService.GetByID(r.Context(), songID)
	if err != nil || song == nil {
		http.NotFound(w, r)
		return nil, "", false
	}
	return song, string(decoded), true
}

// servePlaylist 拉上游 m3u8 → 同源/签名及主机校验 → 改写 URI → 回写。
// upstreamURL 来自 ServeProxy(=song.URL) 或 HandlePlaylist 的 ?u=...
// pathPrefix 由调用方按当前请求 URL 决定（详见 ServeProxy / HandlePlaylist 的注释）。
func (h *HLSHandler) servePlaylist(w http.ResponseWriter, r *http.Request, song *models.Song, upstreamURL, pathPrefix string) {
	songOrigin, _, err := h.checkOrigin(song, upstreamURL, r.URL.Query().Get("sig"), true)
	if err != nil {
		respondError(w, http.StatusForbidden, err.Error(), nil)
		return
	}

	req, err := buildUpstreamRequest(r.Context(), upstreamURL, songOrigin, r.Header.Get("Range"))
	if err != nil {
		respondError(w, http.StatusBadGateway, "构建上游请求失败", err)
		return
	}

	resp, err := h.client.Do(req)
	if err != nil {
		slog.Warn("hls playlist upstream fetch failed", "url", upstreamURL, "error", err)
		respondError(w, http.StatusBadGateway, "playlist fetch failed", nil)
		return
	}
	defer resp.Body.Close()

	// 上游 4xx/5xx：透传 status + body 给 player
	if resp.StatusCode >= 400 {
		slog.Debug("hls playlist upstream error", "url", upstreamURL, "status", resp.StatusCode)
		w.Header().Set("Cache-Control", "no-store")
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.WriteHeader(resp.StatusCode)
		// 上游 4xx/5xx 透传，w.WriteHeader 已调用，流式输出已开始
		if _, err := io.Copy(w, resp.Body); err != nil && !errors.Is(err, context.Canceled) {
			slog.Debug("hls playlist upstream error copy failed", "error", err)
		}
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPlaylistBytes+1))
	if err != nil {
		slog.Warn("hls playlist read failed", "url", upstreamURL, "error", err)
		respondError(w, http.StatusBadGateway, "playlist read failed", nil)
		return
	}
	if len(body) > maxPlaylistBytes {
		slog.Warn("hls playlist too large", "url", upstreamURL, "limit", maxPlaylistBytes)
		respondError(w, http.StatusBadGateway, "playlist too large", nil)
		return
	}

	// base = resp.Request.URL 处理上游 redirect 后的最终位置；Go 内置 redirect 会更新 Request.URL
	base := resp.Request.URL

	// access_token 透传：player（just_audio / libmpv）跟随 m3u8 内部 URL 时不会
	// 复用原请求的 Authorization header，相对 URL 解析也会丢失 base 的 query。
	// 必须把当前请求里的 access_token 注入到每条改写出的子 URL，否则鉴权中间件直接 401。
	accessToken := r.URL.Query().Get("access_token")
	rewritten, err := rewriteM3U8(body, base, h.makeRewriter(song, pathPrefix, accessToken))
	if err != nil {
		slog.Warn("hls playlist rewrite failed", "url", upstreamURL, "error", err)
		respondError(w, http.StatusBadGateway, "playlist rewrite failed", nil)
		return
	}

	w.Header().Set("Content-Type", hlsContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	// HEAD 时 net/http 自动 discard body
	w.Write(rewritten)
}

// serveSegment 透传上游切片字节。无整请求 Timeout，client 断连由 r.Context() 取消上游。
func (h *HLSHandler) serveSegment(w http.ResponseWriter, r *http.Request, song *models.Song, upstreamURL string) {
	songOrigin, _, err := h.checkOrigin(song, upstreamURL, r.URL.Query().Get("sig"), false)
	if err != nil {
		respondError(w, http.StatusForbidden, err.Error(), nil)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "segment request build failed", nil)
		return
	}
	req.Header.Set("User-Agent", streamUserAgent)
	req.Header.Set("Accept", streamAccept)
	req.Header.Set("Referer", songOrigin.Scheme+"://"+songOrigin.Host+"/")
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	httputil.ApplyBasicAuthFromURL(req)

	resp, err := h.client.Do(req)
	if err != nil {
		slog.Debug("hls segment upstream fetch failed", "url", upstreamURL, "error", err)
		respondError(w, http.StatusBadGateway, "segment fetch failed", nil)
		return
	}
	defer resp.Body.Close()

	// 透传字节级 header；强制 no-store（直播流不能缓存）
	for _, hh := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if v := resp.Header.Get(hh); v != "" {
			w.Header().Set(hh, v)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	// w.WriteHeader 已调用，流式转发切片字节
	if _, err := io.Copy(w, resp.Body); err != nil && !errors.Is(err, context.Canceled) {
		slog.Debug("hls segment copy failed", "error", err)
	} // HEAD 时 stdlib 自动 discard
}

// checkOrigin 保留无签名请求的同源校验；签名仅授权指定歌曲和端点的完整 URL。
// 签名不能绕过 IsHostnameAllowed，也不能在修改 song.URL 后继续复用。
// 返回 song origin 和 upstream 解析结果，便于复用。
func (h *HLSHandler) checkOrigin(song *models.Song, upstreamURL, signature string, isPlaylist bool) (songOrigin, up *url.URL, err error) {
	if song.URL == "" {
		return nil, nil, fmt.Errorf("song.URL 为空")
	}
	songOrigin, err = url.Parse(song.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("song.URL 解析失败")
	}
	if songOrigin.Scheme != "http" && songOrigin.Scheme != "https" {
		return nil, nil, fmt.Errorf("不支持的 song.URL scheme")
	}

	up, err = url.Parse(upstreamURL)
	if err != nil {
		return nil, nil, fmt.Errorf("upstream URL 解析失败")
	}
	if up.Scheme != "http" && up.Scheme != "https" {
		return nil, nil, fmt.Errorf("不支持的 upstream scheme")
	}
	if signature != "" || !isSameOrigin(up, songOrigin) {
		if h.signingKey == "" || !hmac.Equal([]byte(signature), []byte(h.resourceSignature(song, upstreamURL, isPlaylist))) {
			return nil, nil, fmt.Errorf("资源签名无效或跨源 URL 缺少签名")
		}
	}
	if h.allowHost != nil && !h.allowHost(up.Hostname()) {
		return nil, nil, fmt.Errorf("目标主机不允许")
	}
	return songOrigin, up, nil
}

func (h *HLSHandler) resourceSignature(song *models.Song, upstreamURL string, isPlaylist bool) string {
	mac := hmac.New(sha256.New, []byte(h.signingKey))
	fmt.Fprintf(mac, "%d\x00%s\x00%t\x00%s", song.ID, song.URL, isPlaylist, upstreamURL)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// checkRedirect 校验每次跳转，避免公开源或 CDN 重定向到私网。
func (h *HLSHandler) checkRedirect(req *http.Request, via []*http.Request) error {
	if err := limitStreamRedirects(req, via); err != nil {
		return err
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("不支持的 redirect scheme")
	}
	if h.allowHost != nil && !h.allowHost(req.URL.Hostname()) {
		return fmt.Errorf("重定向目标主机不允许")
	}
	return nil
}

// makeRewriter 返回闭包：把绝对上游 URL 改写为本机相对路径。
// 用相对路径（而非绝对路径）规避 BASE_PATH 子路径部署拼接问题——
// player 用当前请求 URL 作为 base 解析相对 URL，BASE_PATH 由浏览器/客户端自然继承。
//
// pathPrefix:
//   - "hls/" 用于顶层 ServeProxy（当前请求 .../play，必须前缀才能跳到 .../hls/）
//   - ""     用于子层 HandlePlaylist（当前请求已在 .../hls/playlist，同目录解析即可）
//
// accessToken 透传：相对 URL 解析不继承 base 的 query，必须显式追加到每条改写后的 URL，
// 否则鉴权中间件拒绝 player 跟随回来的子 playlist / segment 请求。
func (h *HLSHandler) makeRewriter(song *models.Song, pathPrefix, accessToken string) func(absURL string, isPlaylist bool) string {
	return func(absURL string, isPlaylist bool) string {
		encoded := base64.RawURLEncoding.EncodeToString([]byte(absURL))
		target := pathPrefix + "segment"
		if isPlaylist {
			target = pathPrefix + "playlist"
		}
		out := target + "?u=" + encoded + "&sig=" + h.resourceSignature(song, absURL, isPlaylist)
		if accessToken != "" {
			out += "&access_token=" + url.QueryEscape(accessToken)
		}
		return out
	}
}

// buildUpstreamRequest 构造带 UA + Referer 的 GET 请求；Range 透传（playlist 通常没用，segment 关键）。
// Referer 用 song origin 的根（scheme + host）—— 多数防盗链只校验 host，不要求具体页面 URL。
func buildUpstreamRequest(ctx context.Context, upstreamURL string, songOrigin *url.URL, rangeHeader string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", streamUserAgent)
	req.Header.Set("Accept", hlsAccept)
	req.Header.Set("Referer", songOrigin.Scheme+"://"+songOrigin.Host+"/")
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	httputil.ApplyBasicAuthFromURL(req)
	return req, nil
}

func limitStreamRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return http.ErrUseLastResponse
	}
	return nil
}

// splitLines 按 \r\n / \n / \r 三种行尾切分内容。返回的行不含行尾。
// 末尾若有空行（如以 \n 结尾）会被丢弃，重组时统一用 \n。
func splitLines(content []byte) []string {
	var lines []string
	i := 0
	n := len(content)
	for i < n {
		start := i
		for i < n && content[i] != '\n' && content[i] != '\r' {
			i++
		}
		lines = append(lines, string(content[start:i]))
		if i < n && content[i] == '\r' {
			i++
			if i < n && content[i] == '\n' {
				i++ // \r\n 算一个行尾
			}
		} else if i < n && content[i] == '\n' {
			i++
		}
	}
	return lines
}
