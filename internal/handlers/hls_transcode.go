package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"songloft/internal/middleware"
	"songloft/internal/services"
)

type HLSTranscodeHandler struct {
	service  *services.HLSTranscodeService
	basePath string
}

func NewHLSTranscodeHandler(service *services.HLSTranscodeService, basePath string) *HLSTranscodeHandler {
	return &HLSTranscodeHandler{service: service, basePath: strings.TrimRight(basePath, "/")}
}

type hlsTranscodeSessionResponse struct {
	ID                 string `json:"id"`
	PlaylistURL        string `json:"playlist_url"`
	IdleTimeoutSeconds int    `json:"idle_timeout_seconds" example:"120"`
}

func respondHLSTranscodeError(w http.ResponseWriter, err error) {
	status := services.HLSTranscodeStatus(err)
	message := err.Error()
	if status == 500 {
		message = "直播转码服务内部错误"
	}
	respondError(w, status, message, nil)
}

// Create 创建远程视频 HLS 转码会话。
// @Summary 创建远程直播 HLS 转码会话
// @Description 拉取 HTTP/HTTPS TS 或普通 HLS 源，复制第一条逐行 H.264 视频轨，第一条音频轨转 AAC-LC 双声道 48kHz。audio_bitrate 默认 128，仅支持 64/96/128/192/256 kbps。仅生成临时滚动分片，不入库；等待首个完整分片就绪，启动超时 30 秒。共享 seek/均衡/URL 转码的 4 个槽位。会话绑定 JWT ClientID，120 秒无播放请求自动回收。playlist_url 包含 BASE_PATH、不含 token；Safari 使用时需追加 access_token，同一 token 会透传到分片链接。源视频的 profile/level 与关键帧仍需满足播放器要求。需 ffmpeg 和 ffprobe。
// @Tags 资源代理
// @Accept json
// @Produce json
// @Param request body services.HLSTranscodeOptions true "直播源和可选拉流参数"
// @Success 201 {object} hlsTranscodeSessionResponse "首个分片已就绪的会话"
// @Failure 400 {object} map[string]string "参数无效"
// @Failure 401 {object} map[string]string "未认证"
// @Failure 403 {object} map[string]string "源或其子资源不在允许范围内"
// @Failure 422 {object} map[string]string "缺少音视频轨或视频格式不支持"
// @Failure 502 {object} map[string]string "上游或转码失败"
// @Failure 503 {object} map[string]string "转码工具不可用或并发已满"
// @Failure 504 {object} map[string]string "启动超时"
// @Security BearerAuth
// @Router /proxy/transcode/hls/sessions [post]
func (h *HLSTranscodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var opts services.HLSTranscodeOptions
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&opts); err != nil {
		respondError(w, 400, "无效的转码请求", nil)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		respondError(w, 400, "请求必须是单个 JSON 对象", nil)
		return
	}
	if opts.UserAgent == "" {
		opts.UserAgent = defaultProxyUserAgent
	}
	id, err := h.service.Create(r.Context(), middleware.ClientIDFromContext(r.Context()), opts)
	h.respondCreated(w, id, err)
}

// CreateRadio creates a compatibility stream for a saved HTTP-FLV station.
// @Summary 为 HTTP-FLV 电台创建 HLS 播放会话
// @Description 从该电台保存的入口拉取 FLV，复制逐行 H.264 视频并输出 AAC-LC 音频及滚动 MPEG-TS 分片。首片就绪后返回会话。会话绑定 JWT ClientID，播放列表包含部署前缀，分片继承播放列表地址的 access_token。共用直播转码的资源上限和删除接口。
// @Tags 电台与 HLS
// @Produce json
// @Param id path int true "HTTP-FLV 电台 ID"
// @Success 201 {object} hlsTranscodeSessionResponse "首个分片已就绪的会话"
// @Failure 400 {object} map[string]string "电台 ID 无效"
// @Failure 401 {object} map[string]string "未认证"
// @Failure 403 {object} map[string]string "源地址不在允许范围内"
// @Failure 404 {object} map[string]string "电台不存在"
// @Failure 422 {object} map[string]string "电台类型或源媒体不支持"
// @Failure 502 {object} map[string]string "上游或转码失败"
// @Failure 503 {object} map[string]string "转码工具不可用或并发已满"
// @Failure 504 {object} map[string]string "启动超时"
// @Security BearerAuth
// @Router /songs/{id}/hls/sessions [post]
func (h *HLSTranscodeHandler) CreateRadio(songs *services.SongService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		songID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || songID <= 0 {
			respondError(w, http.StatusBadRequest, "无效的电台 ID", nil)
			return
		}
		song, err := songs.GetByID(r.Context(), songID)
		if err != nil || song == nil {
			respondError(w, http.StatusNotFound, "电台不存在", nil)
			return
		}
		if !song.IsFLVStream() {
			respondError(w, http.StatusUnprocessableEntity, "仅 HTTP-FLV 电台可创建 HLS 播放会话", nil)
			return
		}
		if h.service == nil {
			respondError(w, http.StatusServiceUnavailable, "直播转码不可用", nil)
			return
		}
		id, err := h.service.CreateFLV(r.Context(), middleware.ClientIDFromContext(r.Context()), radioPullOptions(song.URL))
		h.respondCreated(w, id, err)
	}
}

func radioPullOptions(source string) services.HLSTranscodeOptions {
	opts := services.HLSTranscodeOptions{URL: source, UserAgent: radioStreamUserAgent}
	if u, err := url.Parse(source); err == nil {
		opts.Referer = u.Scheme + "://" + u.Host + "/"
	}
	return opts
}

func (h *HLSTranscodeHandler) respondCreated(w http.ResponseWriter, id string, err error) {
	if err != nil {
		respondHLSTranscodeError(w, err)
		return
	}
	playlistURL := h.basePath + "/api/v1/proxy/transcode/hls/sessions/" + id + "/playlist.m3u8"
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Location", playlistURL)
	respondJSON(w, http.StatusCreated, hlsTranscodeSessionResponse{ID: id, PlaylistURL: playlistURL, IdleTimeoutSeconds: int(services.HLSIdleTimeout.Seconds())})
}

// Playlist 返回当前会话的滚动播放列表。
// @Summary 获取远程直播转码播放列表
// @Description 返回包含最近 6 个分片的 HLS 媒体播放列表，目标分片时长 4 秒（实际依赖源关键帧）。只读取现有会话，不新建进程；相对分片链接保留子路径部署，并透传查询 access_token。Authorization 认证的播放器需自行向分片发送认证头。成功请求续期 120 秒闲置计时。
// @Tags 资源代理
// @Produce application/vnd.apple.mpegurl
// @Param id path string true "会话 ID"
// @Param access_token query string false "无法发送 Authorization 头时使用的访问 token"
// @Success 200 {string} string "HLS 媒体播放列表"
// @Failure 401 {object} map[string]string "未认证"
// @Failure 404 {object} map[string]string "会话已回收或不属于当前客户端"
// @Failure 502 {object} map[string]string "直播转码失败"
// @Security BearerAuth
// @Router /proxy/transcode/hls/sessions/{id}/playlist.m3u8 [get]
func (h *HLSTranscodeHandler) Playlist(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Playlist(chi.URLParam(r, "id"), middleware.ClientIDFromContext(r.Context()))
	if err != nil {
		respondHLSTranscodeError(w, err)
		return
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines[i] = "segments/" + line
		if token := r.URL.Query().Get("access_token"); token != "" {
			lines[i] += "?access_token=" + url.QueryEscape(token)
		}
	}
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, strings.Join(lines, "\n"))
	}
}

// Segment 返回已经完成的 MPEG-TS 分片。
// @Summary 获取远程直播转码分片
// @Description 只返回指定会话已完成的 segment-N.ts 分片，禁止目录遍历和读取临时文件。会话必须属于当前 JWT ClientID；支持 Range。成功打开分片续期闲置计时。
// @Tags 资源代理
// @Produce video/mp2t
// @Param id path string true "会话 ID"
// @Param name path string true "分片文件名，例如 segment-000000001.ts"
// @Param access_token query string false "无法发送 Authorization 头时使用的访问 token"
// @Success 200 {file} binary "MPEG-TS 分片"
// @Success 206 {file} binary "分片的指定字节范围"
// @Failure 400 {object} map[string]string "分片名称非法"
// @Failure 401 {object} map[string]string "未认证"
// @Failure 404 {object} map[string]string "会话或分片不存在/已过期"
// @Failure 502 {object} map[string]string "直播转码失败"
// @Security BearerAuth
// @Router /proxy/transcode/hls/sessions/{id}/segments/{name} [get]
func (h *HLSTranscodeHandler) Segment(w http.ResponseWriter, r *http.Request) {
	f, closeFile, err := h.service.OpenSegment(chi.URLParam(r, "id"), middleware.ClientIDFromContext(r.Context()), chi.URLParam(r, "name"))
	if err != nil {
		respondHLSTranscodeError(w, err)
		return
	}
	defer closeFile()
	info, err := f.Stat()
	if err != nil {
		respondError(w, 502, "读取直播分片失败", nil)
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// Delete 结束直播转码会话。
// @Summary 结束远程直播转码会话
// @Description 取消 FFmpeg 并等待进程退出，归还转码槽位并清理临时分片。已打开的分片在传输关闭后清理。仅允许创建会话的 JWT ClientID 操作。
// @Tags 资源代理
// @Produce json
// @Param id path string true "会话 ID"
// @Success 204 "会话已结束"
// @Failure 401 {object} map[string]string "未认证"
// @Failure 404 {object} map[string]string "会话已回收或不属于当前客户端"
// @Security BearerAuth
// @Router /proxy/transcode/hls/sessions/{id} [delete]
func (h *HLSTranscodeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(chi.URLParam(r, "id"), middleware.ClientIDFromContext(r.Context())); err != nil {
		respondHLSTranscodeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
