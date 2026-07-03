package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"songloft/internal/database"
	"songloft/internal/httputil"
	"songloft/internal/models"
	"songloft/internal/services"
	"songloft/internal/services/playactivity"

	"github.com/go-chi/chi/v5"
)

// PlayEventBroadcaster 向 JS 插件广播播放事件
type PlayEventBroadcaster interface {
	BroadcastPlayEvent(songID int64, title, artist, eventType, source string)
}

// LyricSearcher 歌词搜索接口（由 JS 插件管理器实现）
type LyricSearcher interface {
	SearchLyrics(ctx context.Context, title, artist, album string, duration float64) (*models.LyricPayload, error)
}

// SongHandler 歌曲处理器
type SongHandler struct {
	songService       *services.SongService
	cacheService      *services.CacheService
	reassigner        AsyncReassigner
	lyricFetcher      *services.LyricFetcher // 解包插件 JSON 拿 LRC 文本(歌词 url 分支用)
	hlsHandler        *HLSHandler            // 电台 HLS 流的反代委托（开关在 HLSHandler 内）
	playActivity      *playactivity.Registry // 跟踪进行中的 play/prefetch/transcode/reassign 工作，用户切歌时一次性 cancel
	getMusicPath      func() string          // 获取 music_path（由 scanner.GetMusicPath 注入）
	playBroadcaster   PlayEventBroadcaster   // JS 插件播放事件广播（可选，nil 安全）
	lyricSearcher     LyricSearcher          // 歌词提供者搜索（可选，nil 安全）
	metadataRefresher *services.MetadataRefresher
	configService     *services.ConfigService
	urlResolver       *services.InternalURLResolver // 把插件相对路径解析为本机绝对 URL + access_token（封面代理用）
	radioClient       *http.Client
}

// NewSongHandler 创建歌曲处理器
func NewSongHandler(
	songService *services.SongService,
	cacheService *services.CacheService,
	reassigner AsyncReassigner,
	lyricFetcher *services.LyricFetcher,
	hlsHandler *HLSHandler,
	playActivity *playactivity.Registry,
) *SongHandler {
	return &SongHandler{
		songService:  songService,
		cacheService: cacheService,
		reassigner:   reassigner,
		lyricFetcher: lyricFetcher,
		hlsHandler:   hlsHandler,
		playActivity: playActivity,
		radioClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

// SetGetMusicPath 注入 music_path 获取函数。
func (h *SongHandler) SetGetMusicPath(fn func() string) {
	h.getMusicPath = fn
}

// SetPlayBroadcaster 注入 JS 插件播放事件广播器。
func (h *SongHandler) SetPlayBroadcaster(b PlayEventBroadcaster) {
	h.playBroadcaster = b
}

// SetLyricSearcher 注入歌词搜索器（由 JS 插件管理器实现）。
func (h *SongHandler) SetLyricSearcher(s LyricSearcher) {
	h.lyricSearcher = s
}

// SetMetadataRefresher 注入元数据刷新器。
func (h *SongHandler) SetMetadataRefresher(d *services.MetadataRefresher) {
	h.metadataRefresher = d
}

// SetConfigService 注入配置服务（远程标题来源设置用）。
func (h *SongHandler) SetConfigService(cs *services.ConfigService) {
	h.configService = cs
}

// SetURLResolver 注入内部 URL 解析器，用于将插件相对路径（如封面 URL）解析为本机可访问的绝对 URL。
func (h *SongHandler) SetURLResolver(r *services.InternalURLResolver) {
	h.urlResolver = r
}

const remoteTitleSourceConfigKey = "remote_title_source"

// remoteTitleSourceRequest /settings/remote-title-source 请求/响应体
type remoteTitleSourceRequest struct {
	TitleSource string `json:"title_source" example:"filename" enums:"tag,filename"`
}

// GetRemoteTitleSourceSetting GET /api/v1/settings/remote-title-source
// @Summary 获取网络歌曲标题来源配置
// @Description tag：元数据刷新时用音频标签覆盖标题；filename（默认）：保持文件名作为标题，不覆盖。
// @Tags 歌曲管理
// @Produce json
// @Success 200 {object} remoteTitleSourceRequest "返回 title_source 字段"
// @Security BearerAuth
// @Router /settings/remote-title-source [get]
func (h *SongHandler) GetRemoteTitleSourceSetting(w http.ResponseWriter, r *http.Request) {
	titleSource := "filename"
	if h.configService != nil {
		titleSource = h.configService.GetString(remoteTitleSourceConfigKey, "filename")
	}
	respondJSON(w, http.StatusOK, remoteTitleSourceRequest{TitleSource: titleSource})
}

// UpdateRemoteTitleSourceSetting PUT /api/v1/settings/remote-title-source
// @Summary 更新网络歌曲标题来源配置
// @Description tag：元数据刷新时用音频标签覆盖标题；filename（默认）：保持文件名作为标题，不覆盖。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body remoteTitleSourceRequest true "标题来源配置"
// @Success 200 {object} remoteTitleSourceRequest "返回 title_source 字段"
// @Failure 400 {object} map[string]string "请求格式错误或参数无效"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/remote-title-source [put]
func (h *SongHandler) UpdateRemoteTitleSourceSetting(w http.ResponseWriter, r *http.Request) {
	if h.configService == nil {
		respondError(w, http.StatusInternalServerError, "configService 未注入", nil)
		return
	}
	var req remoteTitleSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if req.TitleSource != "tag" && req.TitleSource != "filename" {
		respondError(w, http.StatusBadRequest, "title_source 必须为 tag 或 filename", nil)
		return
	}
	if err := h.configService.Set(remoteTitleSourceConfigKey, req.TitleSource); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, remoteTitleSourceRequest{TitleSource: req.TitleSource})
}

// StartMetadataRefresh 触发刷新远程歌曲元数据
// @Summary 刷新远程歌曲元数据
// @Description 对所有元数据缺失的远程歌曲，通过 ffprobe 探测时长、比特率、采样率、格式及标签并回填。已在运行时返回 409。
// @Tags 歌曲管理
// @Produce json
// @Success 202 {object} map[string]string "已启动"
// @Failure 409 {object} map[string]string "已在运行"
// @Failure 500 {object} map[string]string "启动失败"
// @Security BearerAuth
// @Router /songs/refresh-metadata [post]
func (h *SongHandler) StartMetadataRefresh(w http.ResponseWriter, r *http.Request) {
	if h.metadataRefresher == nil {
		respondError(w, http.StatusInternalServerError, "metadata refresher not configured", nil)
		return
	}
	if err := h.metadataRefresher.Start(); err != nil {
		respondError(w, http.StatusConflict, err.Error(), nil)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

// GetMetadataRefreshProgress 获取元数据刷新进度
// @Summary 获取元数据刷新进度
// @Description 轮询远程歌曲元数据刷新的执行状态和进度
// @Tags 歌曲管理
// @Produce json
// @Success 200 {object} services.MetadataRefreshProgress "进度信息"
// @Security BearerAuth
// @Router /songs/refresh-metadata/progress [get]
func (h *SongHandler) GetMetadataRefreshProgress(w http.ResponseWriter, r *http.Request) {
	if h.metadataRefresher == nil {
		respondJSON(w, http.StatusOK, services.MetadataRefreshProgress{Status: "idle"})
		return
	}
	respondJSON(w, http.StatusOK, h.metadataRefresher.GetProgress())
}

// CancelMetadataRefresh 取消元数据刷新
// @Summary 取消元数据刷新
// @Description 取消正在执行的远程歌曲元数据刷新任务
// @Tags 歌曲管理
// @Produce json
// @Success 204 "已取消"
// @Security BearerAuth
// @Router /songs/refresh-metadata/cancel [post]
func (h *SongHandler) CancelMetadataRefresh(w http.ResponseWriter, r *http.Request) {
	if h.metadataRefresher != nil {
		h.metadataRefresher.Cancel()
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListSongs 获取歌曲列表
// @Summary 获取歌曲列表
// @Description 获取歌曲列表，支持按类型过滤、关键词搜索和分页
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param type query string false "歌曲类型" Enums(local, remote, radio)
// @Param keyword query string false "搜索关键词"
// @Param path_prefix query string false "按 file_path 前缀过滤（如 music/Pop）"
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Param sort query string false "排序字段，缺省 added_at" Enums(id, title, artist, album, duration, added_at, updated_at, file_modified_at)
// @Param order query string false "排序方向，缺省 desc" Enums(asc, desc)
// @Success 200 {object} map[string]any "成功返回歌曲列表"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// parseSongSort 解析歌曲列表排序参数，缺省按 added_at DESC。
// 非法字段/方向由 repository 层白名单兜底，这里仅负责默认值。
func parseSongSort(sort, order string) (orderBy, dir string) {
	orderBy = sort
	if orderBy == "" {
		orderBy = "added_at"
	}
	dir = order
	if dir == "" {
		dir = "DESC"
	}
	return orderBy, dir
}

// @Router /songs [get]
func (h *SongHandler) ListSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 解析查询参数
	songType := r.URL.Query().Get("type")
	keyword := r.URL.Query().Get("keyword")
	pathPrefix := r.URL.Query().Get("path_prefix")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	orderBy, order := parseSongSort(r.URL.Query().Get("sort"), r.URL.Query().Get("order"))

	limit := models.DefaultPaginationLimit
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	// 限制最大分页大小，防止过大的查询导致性能问题
	if limit > models.MaxPaginationLimit {
		limit = models.MaxPaginationLimit
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	// 构建过滤条件
	filter := &database.SongFilter{
		Type:       songType,
		Keyword:    keyword,
		PathPrefix: pathPrefix,
		Limit:      limit,
		Offset:     offset,
		OrderBy:    orderBy,
		Order:      order,
	}

	// 获取歌曲列表
	songs, err := h.songService.List(ctx, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌曲列表失败", err)
		return
	}

	// 获取总数
	total, err := h.songService.Count(ctx, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌曲总数失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"songs":  songs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// ListSongIDs 返回匹配 filter 的歌曲 ID 列表（不分页、不带 song 详情）
// @Summary 获取匹配歌曲的 ID 列表
// @Description 与 /songs 共享过滤条件，仅返回 ID。用于「全选当前筛选范围」场景。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param type query string false "歌曲类型"
// @Param keyword query string false "搜索关键词"
// @Param path_prefix query string false "按 file_path 前缀过滤"
// @Param sort query string false "排序字段，缺省 added_at" Enums(id, title, artist, album, duration, added_at, updated_at, file_modified_at)
// @Param order query string false "排序方向，缺省 desc" Enums(asc, desc)
// @Success 200 {object} map[string]any "成功返回 ID 列表"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/ids [get]
func (h *SongHandler) ListSongIDs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orderBy, order := parseSongSort(r.URL.Query().Get("sort"), r.URL.Query().Get("order"))
	filter := &database.SongFilter{
		Type:       r.URL.Query().Get("type"),
		Keyword:    r.URL.Query().Get("keyword"),
		PathPrefix: r.URL.Query().Get("path_prefix"),
		OrderBy:    orderBy,
		Order:      order,
	}

	ids, err := h.songService.ListIDs(ctx, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌曲ID列表失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"ids":   ids,
		"total": len(ids),
	})
}

// GetSong 获取单个歌曲
// @Summary 获取单个歌曲详情
// @Description 根据歌曲ID获取详细信息
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param id path int true "歌曲ID"
// @Success 200 {object} models.Song "成功返回歌曲详情"
// @Failure 400 {object} map[string]string "无效的歌曲ID"
// @Failure 404 {object} map[string]string "歌曲不存在"
// @Security BearerAuth
// @Router /songs/{id} [get]
func (h *SongHandler) GetSong(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌曲ID", err)
		return
	}

	song, err := h.songService.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "歌曲不存在", err)
		return
	}

	respondJSON(w, http.StatusOK, song)
}

// DeleteSong 删除歌曲
// @Summary 删除歌曲
// @Description 根据歌曲ID删除歌曲。设置 delete_files=true 时同步删除本地音频文件
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param id path int true "歌曲ID"
// @Param delete_files query bool false "是否同时删除本地音频文件"
// @Success 200 {object} map[string]string "删除成功"
// @Failure 400 {object} map[string]string "无效的歌曲ID"
// @Failure 500 {object} map[string]string "删除失败"
// @Security BearerAuth
// @Router /songs/{id} [delete]
func (h *SongHandler) DeleteSong(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌曲ID", err)
		return
	}

	deleteFiles := r.URL.Query().Get("delete_files") == "true"

	if err := h.songService.Delete(ctx, id, deleteFiles); err != nil {
		respondError(w, http.StatusInternalServerError, "删除歌曲失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "歌曲已删除",
	})
}

// BatchDeleteSongs 批量删除歌曲
// @Summary 批量删除歌曲
// @Description 根据歌曲 ID 列表批量删除歌曲。设置 delete_files=true 时同步删除本地音频文件（用于去重等场景）
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body models.BatchDeleteSongsRequest true "批量删除请求"
// @Success 200 {object} models.BatchDeleteSongsResponse "删除成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "删除失败"
// @Security BearerAuth
// @Router /songs/batch-delete [post]
func (h *SongHandler) BatchDeleteSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.BatchDeleteSongsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "ID 列表不能为空", nil)
		return
	}

	deleted, err := h.songService.BatchDelete(ctx, req.IDs, req.DeleteFiles)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "批量删除歌曲失败", err)
		return
	}

	respondJSON(w, http.StatusOK, models.BatchDeleteSongsResponse{
		Deleted: deleted,
	})
}

// UpdateSong 更新歌曲信息
// @Summary 更新歌曲信息
// @Description 更新歌曲信息（仅支持网络歌曲和电台）
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param id path int true "歌曲ID"
// @Param request body object{title=string,artist=string,album=string,url=string,cover_url=string} true "歌曲信息"
// @Success 200 {object} models.Song "更新成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 404 {object} map[string]string "歌曲不存在"
// @Failure 500 {object} map[string]string "更新失败"
// @Security BearerAuth
// @Router /songs/{id} [put]
func (h *SongHandler) UpdateSong(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌曲ID", err)
		return
	}

	// 获取现有歌曲
	existingSong, err := h.songService.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "歌曲不存在", err)
		return
	}

	// 解析请求
	var req struct {
		Title    string `json:"title"`
		Artist   string `json:"artist"`
		Album    string `json:"album"`
		URL      string `json:"url"`
		CoverURL string `json:"cover_url"`
		IsLive   *bool  `json:"is_live"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	// 验证必填字段
	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "标题不能为空", nil)
		return
	}
	// 非本地歌曲的URL不能为空
	if req.URL == "" && existingSong.Type != models.TypeLocal {
		respondError(w, http.StatusBadRequest, "URL不能为空", nil)
		return
	}

	// 更新歌曲信息
	existingSong.Title = req.Title
	existingSong.Artist = req.Artist
	existingSong.Album = req.Album
	existingSong.URL = req.URL
	existingSong.CoverURL = req.CoverURL
	if req.IsLive != nil && existingSong.Type != models.TypeRadio {
		existingSong.IsLive = *req.IsLive
	}

	if err := h.songService.Update(ctx, existingSong); err != nil {
		respondError(w, http.StatusInternalServerError, "更新歌曲失败", err)
		return
	}

	respondJSON(w, http.StatusOK, existingSong)
}

// AddRemoteSongs 批量添加网络歌曲
// @Summary 批量添加网络歌曲
// @Description 批量添加网络歌曲到数据库。cover_url 支持以 "/" 开头的相对路径（插件场景下由服务端自动解析为内部 URL，与歌词 lyric_remote_url 的解析机制一致）。lyric_remote_url 为歌词远程 URL 直传字段，提供时优先于 lyric + lyric_source=url 的间接方式。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body []object{url=string,title=string,artist=string,album=string,cover_url=string,duration=number,plugin_entry_path=string,source_data=string,dedup_key=string,lyric=string,lyric_source=string,lyric_remote_url=string} true "网络歌曲列表"
// @Success 201 {object} object{songs=[]models.Song,count=int} "添加成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "添加失败"
// @Security BearerAuth
// @Router /songs/remote [post]
func (h *SongHandler) AddRemoteSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var reqs []struct {
		URL             string  `json:"url"` // 仅纯外链歌曲(直接 http(s) URL)使用;插件来源歌曲应留空
		Title           string  `json:"title"`
		Artist          string  `json:"artist"`
		Album           string  `json:"album"`
		CoverURL        string  `json:"cover_url"`
		Duration        float64 `json:"duration"`
		PluginEntryPath string  `json:"plugin_entry_path"` // 音源插件 entryPath(如 "subsonic");纯外链留空
		SourceData      string  `json:"source_data"`       // 音源元数据 JSON(opaque);纯外链留空
		DedupKey        string  `json:"dedup_key"`         // 去重 key(由插件定义);空时不去重直接 INSERT
		Lyric           string  `json:"lyric"`
		LyricSource     string  `json:"lyric_source"`
		LyricRemoteURL  string  `json:"lyric_remote_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if len(reqs) == 0 {
		respondError(w, http.StatusBadRequest, "请求列表不能为空", nil)
		return
	}

	inputs := make([]services.RemoteSongInput, 0, len(reqs))
	for i, req := range reqs {
		// 至少要有一种音源标识:URL 或 (plugin_entry_path + source_data)
		if req.Title == "" {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("第 %d 条:标题不能为空", i+1), nil)
			return
		}
		hasPlugin := req.PluginEntryPath != "" && req.SourceData != ""
		if req.URL == "" && !hasPlugin {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("第 %d 条:必须提供 url 或 (plugin_entry_path + source_data)", i+1), nil)
			return
		}
		inputs = append(inputs, services.RemoteSongInput{
			URL:             req.URL,
			Title:           req.Title,
			Artist:          req.Artist,
			Album:           req.Album,
			CoverURL:        req.CoverURL,
			Duration:        req.Duration,
			PluginEntryPath: req.PluginEntryPath,
			SourceData:      req.SourceData,
			DedupKey:        req.DedupKey,
			Lyric:           req.Lyric,
			LyricSource:     req.LyricSource,
			LyricRemoteURL:  req.LyricRemoteURL,
		})
	}

	songs, err := h.songService.AddRemoteSongs(ctx, inputs)
	if err != nil {
		slog.Info("批量添加网络歌曲失败", "err", err)
		respondError(w, http.StatusInternalServerError, "批量添加网络歌曲失败", err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"songs": songs,
		"count": len(songs),
	})
}

// AddRadios 批量添加电台/广播
// @Summary 批量添加电台/广播
// @Description 批量添加电台/广播到数据库
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body []object{url=string,title=string,cover_url=string} true "电台/广播列表"
// @Success 201 {object} object{songs=[]models.Song,count=int} "添加成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "添加失败"
// @Security BearerAuth
// @Router /songs/radio [post]
func (h *SongHandler) AddRadios(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var reqs []struct {
		URL      string `json:"url"`
		Title    string `json:"title"`
		Artist   string `json:"artist"`
		CoverURL string `json:"cover_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if len(reqs) == 0 {
		respondError(w, http.StatusBadRequest, "请求列表不能为空", nil)
		return
	}

	inputs := make([]services.RadioInput, 0, len(reqs))
	for i, req := range reqs {
		if req.URL == "" || req.Title == "" {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("第 %d 条：URL 和标题不能为空", i+1), nil)
			return
		}
		inputs = append(inputs, services.RadioInput{
			URL:      req.URL,
			Title:    req.Title,
			Artist:   req.Artist,
			CoverURL: req.CoverURL,
		})
	}

	songs, err := h.songService.AddRadios(ctx, inputs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "批量添加电台失败", err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"songs": songs,
		"count": len(songs),
	})
}

// GetSongCover 获取歌曲封面图片
// @Summary 获取歌曲封面图片
// @Description 根据歌曲 ID 获取封面图片。优先使用本地封面文件（CoverPath），其次代理 CoverURL。CoverURL 支持以 "/" 开头的相对路径，服务端自动经 InternalURLResolver 解析为内部 URL（含 access_token），用于插件歌曲封面代理。
// @Tags 歌曲管理
// @Produce image/jpeg
// @Param id path int true "歌曲 ID"
// @Success 200 {file} binary "封面图片"
// @Failure 400 {object} map[string]string "无效的歌曲 ID"
// @Failure 404 {object} map[string]string "歌曲或封面不存在"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/{id}/cover [get]
func (h *SongHandler) GetSongCover(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的 ID", err)
		return
	}

	// 获取歌曲信息
	song, err := h.songService.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "歌曲不存在", err)
		return
	}

	// 优先使用本地封面
	if song.CoverPath != "" {
		h.serveLocalCover(w, r, song)
		return
	}

	// 本地封面不存在时,代理转发外部 URL
	// 支持插件相对路径:以 "/" 开头的 URL 经 InternalURLResolver 解析为本机绝对 URL + access_token,
	// 与歌词的 LyricFetcher 解析机制一致;绝对 URL 原样透传。
	if song.CoverURL != "" {
		coverURL := song.CoverURL
		if h.urlResolver != nil {
			coverURL = h.urlResolver.Resolve(coverURL)
		}
		ServeRemoteResource(w, r, coverURL)
		return
	}

	respondError(w, http.StatusNotFound, "封面不存在", nil)
}

// serveLocalCover 返回本地封面文件
func (h *SongHandler) serveLocalCover(w http.ResponseWriter, r *http.Request, song *models.Song) {
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	http.ServeFile(w, r, song.CoverPath)
}

// CleanInvalidSongs 清理无效的本地歌曲
// @Summary 清理无效的本地歌曲
// @Description 清理本地歌曲中文件已不存在或位于排除目录中的记录，同时删除关联的封面文件
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]any "清理成功"
// @Failure 500 {object} map[string]string "清理失败"
// @Security BearerAuth
// @Router /songs/clean [post]
func (h *SongHandler) CleanInvalidSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	result, err := h.songService.CleanInvalidSongs(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "清理无效歌曲失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message":         "清理完成",
		"total":           result.Total,
		"file_not_found":  result.FileNotFound,
		"in_excluded_dir": result.InExcludedDir,
	})
}

// UpdateSongLyrics 更新歌曲歌词
//
// 入参形态二选一,由 lyric_source 决定:
//
//  1. lyric_source = "url":写 lyric_remote_url 列(运行时由 LyricFetcher 拉取),
//     字段:lyric_remote_url。
//
//  2. 其它来源(scraped/file/embedded/cached):写 LyricPayload JSON 入 lyric 列,
//     字段:lyric / tlyric / rlyric / lxlyric。
//
// @Summary 更新歌曲歌词
// @Description 更新指定歌曲的歌词内容和来源。url 来源传 lyric_remote_url,其它来源传 lyric/tlyric/rlyric/lxlyric 四字段。响应里的 file_write_status 表示是否把元数据回写到本地音频文件:written=已写入,unchanged=未变更(非本地歌曲/无文件路径/不支持的扩展名/url 来源),skipped=标签已一致无需写入,failed=尝试写入但失败(DB 已成功)。lyric_source=manual 用于标记用户手动调整,scanner 重扫时不会覆盖
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param id path int true "歌曲 ID"
// @Param request body object{lyric_source=string,lyric=string,tlyric=string,rlyric=string,lxlyric=string,lyric_remote_url=string} true "歌词信息"
// @Success 200 {object} object{message=string,file_write_status=string} "更新成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 404 {object} map[string]string "歌曲不存在"
// @Failure 500 {object} map[string]string "更新失败"
// @Security BearerAuth
// @Router /songs/{id}/lyrics [put]
func (h *SongHandler) UpdateSongLyrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌曲 ID", err)
		return
	}

	var req struct {
		LyricSource    string `json:"lyric_source"`
		LyricRemoteURL string `json:"lyric_remote_url"`
		Lyric          string `json:"lyric"`
		Tlyric         string `json:"tlyric"`
		Rlyric         string `json:"rlyric"`
		Lxlyric        string `json:"lxlyric"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	var lyricCol, lyricURLCol string
	if req.LyricSource == models.LyricSourceURL {
		lyricURLCol = req.LyricRemoteURL
	} else {
		lyricCol = models.LyricPayload{
			Lyric:   req.Lyric,
			Tlyric:  req.Tlyric,
			Rlyric:  req.Rlyric,
			Lxlyric: req.Lxlyric,
		}.MarshalString()
	}

	status, err := h.songService.UpdateLyrics(ctx, id, lyricCol, req.LyricSource, lyricURLCol)
	if err != nil {
		if err.Error() == "song not found" {
			respondError(w, http.StatusNotFound, "歌曲不存在", err)
			return
		}
		respondError(w, http.StatusInternalServerError, "更新歌词失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message":           "歌词已更新",
		"file_write_status": string(status),
	})
}

// GetSongPlay 按 song.ID 流式返回音频。
//
// 路径:GET /api/v1/songs/{id}/play
//
// 客户端拿到的 song.url 永远是这个端点(由 Song.PlaybackURL() 统一填),
// 不需要判断 song.type/source — 所有分发逻辑都集中在这里。
//
// @Summary 流式播放歌曲
// @Description 按 song.ID 流式返回音频。内部根据 song.type 分发到本地文件 / 缓存下载 / 直链下载 / 电台 302。
// @Tags 歌曲管理
// @Produce application/octet-stream
// @Param id path int true "歌曲 ID"
// @Param format query string false "目标转码格式（如 mp3、ogg），用于平台兼容性转码"
// @Param quality query string false "目标音质码率（128/192/320），不传或不合法值表示原始音质。指定后默认转码为 mp3（除非同时指定了 format）"
// @Param prefetch query string false "传 1 时异步预热缓存/转码，立即返回 202"
// @Success 200 {file} binary "音频文件"
// @Success 202 {string} string "预拉取已触发"
// @Success 302 {string} string "电台流重定向"
// @Failure 404 {string} string "歌曲不存在"
// @Failure 502 {string} string "音源不可用"
// @Security BearerAuth
// @Router /songs/{id}/play [get]
// @Router /songs/{id}/play.m3u8 [get]
func (h *SongHandler) GetSongPlay(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	songID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || songID <= 0 {
		respondError(w, http.StatusBadRequest, "无效的 song_id", err)
		return
	}

	// 用户进入正式播放路径时，让该客户端会话下其他歌曲的进行中工作集体退场
	// （prefetch / transcode / reassign），避免它们继续占用 plugin worker / 转码 sem。
	// 仅 prefetch 旁路跳过 Activate，因为 prefetch 自己也注册到 registry，
	// 让它由后续真实播放或下一次 prefetch 触发清理。
	sk := playactivity.SessionFromContext(r.Context())
	if r.URL.Query().Get("prefetch") != "1" && h.playActivity != nil {
		h.playActivity.Activate(sk, songID)
	}

	ctx := r.Context()
	song, err := h.songService.GetByID(ctx, songID)
	if err != nil || song == nil {
		http.NotFound(w, r)
		return
	}

	targetFormat := r.URL.Query().Get("format")
	bitrate := services.ParseBitrate(r.URL.Query().Get("quality"))
	if bitrate > 0 && targetFormat == "" {
		targetFormat = "mp3"
	}

	// 预拉取模式：异步触发缓存 + 转码预热，立即返回 202。
	// 不能用 r.Context()，否则 202 发出后客户端断开会 Kill ffmpeg，预热失败。
	// 但通过 playActivity.Track 让 prefetch 能在下一次 Activate 时被 cancel，
	// 避免占着 plugin worker 跑完整 30s。
	if r.URL.Query().Get("prefetch") == "1" {
		go func() {
			pctx, release := h.trackActivity(context.Background(), sk, song.ID, playactivity.CatPrefetch)
			defer release()
			h.prepareSongPlayback(pctx, song, targetFormat, bitrate)
		}()
		w.WriteHeader(http.StatusAccepted)
		return
	}

	switch song.Type {
	case models.TypeLocal:
		h.serveLocal(w, r, song, targetFormat, bitrate)
	case models.TypeRadio:
		h.serveRadio(w, r, song)
	case models.TypeRemote:
		h.serveRemote(w, r, song, targetFormat, bitrate)
	default:
		http.Error(w, "unsupported song type", http.StatusInternalServerError)
	}
}

// trackActivity 是 playActivity.Track 的兜底封装：当 registry 未注入（旧测试 / lite 模式）时
// 退化为返回 parent ctx + no-op release，调用方代码无需到处加 nil 判断。
func (h *SongHandler) trackActivity(parent context.Context, sk playactivity.SessionKey, songID int64, cat playactivity.Category) (context.Context, func()) {
	if h.playActivity == nil {
		return parent, func() {}
	}
	return h.playActivity.Track(parent, sk, songID, cat)
}

// ActivateSong 把指定歌曲标记为该客户端会话的"当前活跃歌曲"。
//
// 客户端在切歌前调用一次：后端会立刻 cancel 该会话下其他歌曲的进行中工作
// （prefetch / transcode / reassign），让插件 worker 与转码 sem 立即让位给新歌。
// 不依赖客户端关闭旧的 HTTP 流（just_audio LockCachingAudioSource 不会主动 abort），
// 是 issue #79 残留卡顿的关键解药。
//
// 幂等：重复调用同一 songID 无副作用；调用时如果该会话桶已经空了也不报错。
//
// @Summary 标记当前活跃歌曲
// @Description 客户端切歌前调用，让后端 cancel 同一会话下其他歌曲的进行中工作（prefetch/transcode/reassign）。其他客户端会话不受影响。
// @Tags 歌曲管理
// @Produce json
// @Param id path int true "歌曲 ID"
// @Success 204 "无内容"
// @Failure 400 {object} map[string]string "无效的 song_id"
// @Security BearerAuth
// @Router /songs/{id}/activate [post]
func (h *SongHandler) ActivateSong(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	songID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || songID <= 0 {
		respondError(w, http.StatusBadRequest, "无效的 song_id", err)
		return
	}
	if h.playActivity != nil {
		sk := playactivity.SessionFromContext(r.Context())
		h.playActivity.Activate(sk, songID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// prepareSongPlayback 后台预热一首歌曲：拉取到缓存 + 必要时转码。
// 判断与 serveLocal/serveRemote 保持一致，失败仅警告不报错。
func (h *SongHandler) prepareSongPlayback(ctx context.Context, song *models.Song, targetFormat string, bitrate int) {
	if song == nil {
		return
	}
	var srcPath string
	switch song.Type {
	case models.TypeLocal:
		if song.FilePath == "" {
			return
		}
		srcPath = song.FilePath
	case models.TypeRemote:
		if !song.IsPluginSourced() {
			return
		}
		path, err := h.cacheService.Get(ctx, song)
		if err != nil {
			slog.Warn("prefetch cache get failed", "songId", song.ID, "error", err)
			return
		}
		srcPath = path
	default:
		return
	}

	if !services.NeedsTranscode(services.EffectiveSourceFormat(song, srcPath), targetFormat) && bitrate == 0 {
		return
	}
	if _, err := h.cacheService.GetOrTranscode(ctx, srcPath, song, services.NormalizeFormat(targetFormat), bitrate); err != nil {
		slog.Warn("prefetch transcode failed", "songId", song.ID, "format", targetFormat, "bitrate", bitrate, "error", err)
	} else {
		slog.Info("prefetch ready", "songId", song.ID, "format", targetFormat, "bitrate", bitrate)
	}
}

// serveLocal 本地歌曲:直接 ServeFile(支持 Range,客户端 seek 可用)。
// targetFormat 非空且与原格式不同时，或 bitrate > 0 时，走 ffmpeg 转码后返回。
func (h *SongHandler) serveLocal(w http.ResponseWriter, r *http.Request, song *models.Song, targetFormat string, bitrate int) {
	if song.FilePath == "" {
		http.NotFound(w, r)
		return
	}
	srcPath := song.FilePath
	if services.NeedsTranscode(services.EffectiveSourceFormat(song, srcPath), targetFormat) || bitrate > 0 {
		tcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		sk := playactivity.SessionFromContext(r.Context())
		trackedCtx, release := h.trackActivity(tcCtx, sk, song.ID, playactivity.CatTranscode)
		defer release()
		path, err := h.cacheService.GetOrTranscode(trackedCtx, srcPath, song, services.NormalizeFormat(targetFormat), bitrate)
		if err != nil {
			slog.Warn("transcode failed, serving original", "songId", song.ID, "format", targetFormat, "bitrate", bitrate, "error", err)
		} else {
			srcPath = path
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	http.ServeFile(w, r, srcPath)
}

// serveRadio 电台/直播流:专用代理，不设超时、不缓存。
// 与 ServeRemoteResource 不同:客户端断开时由 r.Context() 取消上游请求，不受 60s 硬超时限制。
// HLS (m3u8) 走 302 重定向给前端 player 自己解析:m3u8 内含相对路径 .ts 切片,
// 服务端透传会导致客户端按本机 URL 错误解析切片路径。
func (h *SongHandler) serveRadio(w http.ResponseWriter, r *http.Request, song *models.Song) {
	if song.URL == "" {
		http.NotFound(w, r)
		return
	}

	if isHLSURL(song.URL) {
		// HLS 反代开关由 HLSHandler 业务封装管理（/settings/hls-proxy），默认 false 走 302
		if h.hlsHandler != nil && h.hlsHandler.IsEnabled() {
			h.hlsHandler.ServeProxy(w, r, song)
			return
		}
		http.Redirect(w, r, song.URL, http.StatusFound)
		return
	}

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, song.URL, nil)
	if err != nil {
		slog.Warn("radio stream request failed", "url", song.URL, "error", err)
		http.Error(w, "resource fetch failed", http.StatusInternalServerError)
		return
	}
	upstreamReq.Header.Set("User-Agent", "Songloft/1.0")
	if accept := r.Header.Get("Accept"); accept != "" {
		upstreamReq.Header.Set("Accept", accept)
	}
	httputil.ApplyBasicAuthFromURL(upstreamReq)

	resp, err := h.radioClient.Do(upstreamReq)
	if err != nil {
		slog.Warn("radio stream fetch failed", "url", song.URL, "error", err)
		http.Error(w, "resource fetch failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// isHLSURL 判断 URL 是否指向 HLS 播放列表(.m3u8/.m3u),忽略大小写与查询串。
func isHLSURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	return ext == ".m3u8" || ext == ".m3u"
}

// serveRemote 网络歌曲:根据音源类型分发到缓存或代理。
// - 插件来源歌曲:走 CacheService.Get(下载缓存)
// - 纯外链歌曲:走 ServeRemoteResource(直接代理)
// 失败时:返回 502,后台异步切源(若注入了 reassigner),客户端下次播放该 song 会用新源。
// targetFormat 非空且与原格式不同时,对已缓存文件走 ffmpeg 转码。
func (h *SongHandler) serveRemote(w http.ResponseWriter, r *http.Request, song *models.Song, targetFormat string, bitrate int) {
	// 1. 缓存命中 → 直接 ServeFile
	if song.CachePath != "" {
		if _, err := os.Stat(song.CachePath); err == nil {
			h.serveCachedFile(w, r, song, song.CachePath, targetFormat, bitrate)
			return
		}
		h.cacheService.ClearStaleCachePath(song.ID)
	}

	// fallback: 旧格式缓存（兼容升级过渡）
	if cachedPath, ok := h.cacheService.FindCachedFileBySong(song); ok {
		h.serveCachedFile(w, r, song, cachedPath, targetFormat, bitrate)
		return
	}

	// 2. 缓存未命中：解析播放 URL
	var playURL string
	var upstreamHeaders map[string]string
	if song.IsPluginSourced() {
		resolved, err := h.cacheService.ResolveURL(r.Context(), song)
		if err != nil {
			slog.Warn("resolve url failed", "songId", song.ID, "error", err)
			sk := playactivity.SessionFromContext(r.Context())
			if h.reassigner != nil {
				h.reassigner.AsyncReassign(song.ID, sk)
			}
			http.Error(w, "source unavailable: "+err.Error(), http.StatusBadGateway)
			return
		}
		playURL = resolved.URL
		upstreamHeaders = resolved.Headers
	} else if song.URL != "" {
		playURL = song.URL
	} else {
		http.NotFound(w, r)
		return
	}

	// 3. 播放时异步提取元数据（首次播放触发，后续跳过）
	if h.metadataRefresher != nil && services.NeedsMetadata(song) {
		refreshCopy := *song
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			h.metadataRefresher.RefreshSong(ctx, &refreshCopy, playURL)
		}()
	}

	// 4. 流式代理 + 后台缓存
	songCopy := *song
	ServeRemoteResourceWithCache(w, r, playURL, upstreamHeaders,
		func(tmpPath, contentType string) {
			ext := services.GetExtFromContentType(contentType)
			h.cacheService.FinalizeCache(context.Background(), &songCopy, tmpPath, ext)
		},
		func() {
			h.cacheService.AsyncDownloadAndCache(context.Background(), &songCopy, playURL, upstreamHeaders)
		},
	)
}

// serveCachedFile 从缓存文件提供服务,支持转码。
func (h *SongHandler) serveCachedFile(w http.ResponseWriter, r *http.Request, song *models.Song, cachedPath, targetFormat string, bitrate int) {
	if services.NeedsTranscode(services.EffectiveSourceFormat(song, cachedPath), targetFormat) || bitrate > 0 {
		sk := playactivity.SessionFromContext(r.Context())
		tcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		trackedCtx, releaseTc := h.trackActivity(tcCtx, sk, song.ID, playactivity.CatTranscode)
		defer releaseTc()
		path, err := h.cacheService.GetOrTranscode(trackedCtx, cachedPath, song, services.NormalizeFormat(targetFormat), bitrate)
		if err != nil {
			slog.Warn("transcode failed, serving original", "songId", song.ID, "format", targetFormat, "bitrate", bitrate, "error", err)
		} else {
			cachedPath = path
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeFile(w, r, cachedPath)
}

// GetSongLyric 获取歌曲歌词。
//
// 路径:GET /api/v1/songs/{id}/lyric
//
// 直接返回 LyricPayload JSON:
//
//		{"lyric": "...", "tlyric": "...", "rlyric": "...", "lxlyric": "..."}
//
//	  - cached/file/embedded/scraped:解包 songs.lyric 中存的 LyricPayload JSON
//	  - url:走 LyricFetcher 解包插件返回的 envelope,取出 data
//
// @Summary 获取歌曲歌词
// @Description 根据 song.ID 返回 LyricPayload JSON，含 lyric/tlyric/rlyric/lxlyric。
// @Tags 歌曲管理
// @Produce json
// @Param id path int true "歌曲 ID"
// @Success 200 {object} map[string]any "LyricPayload"
// @Failure 404 {string} string "歌曲或歌词不存在"
// @Failure 502 {string} string "歌词获取失败"
// @Security BearerAuth
// @Router /songs/{id}/lyric [get]
func (h *SongHandler) GetSongLyric(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	songID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || songID <= 0 {
		respondError(w, http.StatusBadRequest, "无效的 song_id", err)
		return
	}

	ctx := r.Context()
	song, err := h.songService.GetByID(ctx, songID)
	if err != nil || song == nil {
		http.NotFound(w, r)
		return
	}

	var payload models.LyricPayload
	if song.LyricSource == models.LyricSourceURL {
		if song.LyricRemoteURL != "" && h.lyricFetcher != nil {
			p, err := h.lyricFetcher.Fetch(ctx, song.LyricRemoteURL)
			if err != nil {
				respondError(w, http.StatusBadGateway, "歌词获取失败", err)
				return
			}
			payload = p
		}
	} else if song.Lyric != "" {
		payload = models.UnmarshalLyric(song.Lyric)
	}

	// 歌词为空时，尝试从已注册的歌词提供者插件获取
	if payload.IsEmpty() && h.lyricSearcher != nil {
		if found, err := h.lyricSearcher.SearchLyrics(ctx, song.Title, song.Artist, song.Album, song.Duration); err == nil && found != nil && !found.IsEmpty() {
			go h.songService.UpdateLyrics(context.Background(), song.ID, found.MarshalString(), models.LyricSourceScraped, "")
			payload = *found
		}
	}

	if payload.IsEmpty() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000")
	respondJSON(w, http.StatusOK, payload)
}

// WriteSongTagsRequest 写入歌曲标签的请求体。
type WriteSongTagsRequest struct {
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	Year       int    `json:"year"`
	Genre      string `json:"genre"`
	Lyrics     string `json:"lyrics"`
	CoverData  string `json:"cover_data"`
	CoverURL   string `json:"cover_url"`
	ClearCover bool   `json:"clear_cover"`
}

// WriteTags 写入歌曲标签
// @Summary 写入歌曲标签
// @Description 将元数据写入数据库和本地音频文件标签（仅本地歌曲）。cover_data(base64) 优先于 cover_url。非空字段覆盖，空值保留原值。设置 clear_cover=true 可显式清空封面。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param id path int true "歌曲ID"
// @Param request body WriteSongTagsRequest true "标签数据"
// @Success 200 {object} object{song=models.Song,file_write=string} "写入结果"
// @Failure 400 {object} map[string]string "请求错误"
// @Failure 404 {object} map[string]string "歌曲不存在"
// @Security BearerAuth
// @Router /api/v1/songs/{id}/tags [put]
func (h *SongHandler) WriteTags(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌曲ID", err)
		return
	}

	song, err := h.songService.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "歌曲不存在", err)
		return
	}

	if song.Type != models.TypeLocal {
		respondError(w, http.StatusBadRequest, "仅支持本地歌曲", nil)
		return
	}

	var req WriteSongTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if req.Title != "" {
		song.Title = req.Title
	}
	if req.Artist != "" {
		song.Artist = req.Artist
	}
	if req.Album != "" {
		song.Album = req.Album
	}
	if req.Year > 0 {
		song.Year = req.Year
	}
	if req.Genre != "" {
		song.Genre = req.Genre
	}
	if req.Lyrics != "" {
		song.Lyric = models.LyricPayloadFromLRC(req.Lyrics).MarshalString()
		song.LyricSource = models.LyricSourceManual
	}

	if req.CoverData != "" {
		data, err := base64.StdEncoding.DecodeString(req.CoverData)
		if err != nil {
			respondError(w, http.StatusBadRequest, "无效的 cover_data base64", err)
			return
		}
		ext := "jpg"
		if len(data) > 8 {
			ext = detectImageExt(data)
		}
		if coverPath, err := h.songService.SaveCoverFromData(data, ext); err != nil {
			slog.Warn("save cover from data failed", "error", err)
			song.CoverPath = ""
			song.CoverURL = ""
		} else {
			song.CoverPath = coverPath
		}
	} else if req.CoverURL != "" {
		if coverPath, err := h.songService.DownloadCover(ctx, req.CoverURL); err != nil {
			slog.Warn("download cover failed", "url", req.CoverURL, "error", err)
			song.CoverPath = ""
			song.CoverURL = ""
		} else {
			song.CoverPath = coverPath
			song.CoverURL = req.CoverURL
		}
	} else if req.ClearCover {
		song.CoverPath = ""
		song.CoverURL = ""
	}

	if err := h.songService.Update(ctx, song); err != nil {
		respondError(w, http.StatusInternalServerError, "更新歌曲失败", err)
		return
	}

	fileWrite := services.WriteSongTags(song.FilePath, song)

	respondJSON(w, http.StatusOK, map[string]any{
		"song":       song,
		"file_write": string(fileWrite),
	})
}

func detectImageExt(data []byte) string {
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "png"
	}
	if len(data) >= 4 && data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' {
		return "webp"
	}
	if len(data) >= 3 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
		return "gif"
	}
	return "jpg"
}

// OrganizeSongs 批量整理歌曲文件
// @Summary 批量整理歌曲文件
// @Description 批量移动/重命名本地歌曲文件到指定目录结构。target_path 为相对于 music_path 的路径（含目录和文件名），扩展名必须与原文件一致。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body []services.OrganizeItem true "整理项目列表"
// @Success 200 {array} services.OrganizeResult "整理结果"
// @Failure 400 {object} map[string]string "请求错误"
// @Security BearerAuth
// @Router /api/v1/songs/organize [post]
func (h *SongHandler) OrganizeSongs(w http.ResponseWriter, r *http.Request) {
	if h.getMusicPath == nil {
		respondError(w, http.StatusInternalServerError, "music path not configured", nil)
		return
	}
	musicPath := h.getMusicPath()
	if musicPath == "" {
		respondError(w, http.StatusBadRequest, "music_path 未设置", nil)
		return
	}

	var items []services.OrganizeItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}
	if len(items) == 0 {
		respondError(w, http.StatusBadRequest, "列表不能为空", nil)
		return
	}

	results := h.songService.OrganizeSongs(r.Context(), musicPath, items)
	respondJSON(w, http.StatusOK, results)
}

// duplicateSongResponse 重复歌曲的 JSON 响应结构。
type duplicateSongResponse struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Album    string  `json:"album"`
	Duration float64 `json:"duration"`
	FilePath string  `json:"file_path"`
	Format   string  `json:"format"`
	BitRate  int     `json:"bit_rate"`
	FileSize int64   `json:"file_size"`
	CoverURL string  `json:"cover_url"`
	AddedAt  string  `json:"added_at"`
}

// duplicateGroupResponse 重复组的 JSON 响应结构。
type duplicateGroupResponse struct {
	Fingerprint string                  `json:"fingerprint"`
	Songs       []duplicateSongResponse `json:"songs"`
}

// GetDuplicates 获取重复歌曲组
// @Summary 获取重复歌曲组
// @Description 通过音频指纹查询本地歌曲中内容相同的重复组
// @Tags 歌曲管理
// @Produce json
// @Success 200 {object} map[string]interface{} "重复歌曲组列表"
// @Security BearerAuth
// @Router /songs/duplicates [get]
func (h *SongHandler) GetDuplicates(w http.ResponseWriter, r *http.Request) {
	groups, err := h.songService.GetDuplicateGroups(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "查询重复歌曲失败", err)
		return
	}

	result := make([]duplicateGroupResponse, 0, len(groups))
	totalDuplicates := 0
	for _, g := range groups {
		songs := make([]duplicateSongResponse, len(g.Songs))
		for i, s := range g.Songs {
			coverURL := ""
			if s.CoverPath != "" || s.CoverURL != "" {
				coverURL = fmt.Sprintf("/api/v1/songs/%d/cover", s.ID)
			}
			songs[i] = duplicateSongResponse{
				ID:       s.ID,
				Title:    s.Title,
				Artist:   s.Artist,
				Album:    s.Album,
				Duration: s.Duration,
				FilePath: s.FilePath,
				Format:   s.Format,
				BitRate:  s.BitRate,
				FileSize: s.FileSize,
				CoverURL: coverURL,
				AddedAt:  s.AddedAt.Format("2006-01-02T15:04:05Z"),
			}
		}
		totalDuplicates += len(songs)
		result = append(result, duplicateGroupResponse{
			Fingerprint: g.Fingerprint,
			Songs:       songs,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"groups":           result,
		"total_groups":     len(result),
		"total_duplicates": totalDuplicates,
	})
}

// SongPlayed 通知歌曲播放事件
// @Summary 通知歌曲播放事件
// @Description 客户端在歌曲开始播放、播放完成或被跳过时调用此端点，后端将事件广播给已订阅播放事件的 JS 插件（通过 songloft.events.onPlayEvent 注册）。source 参数标识调用来源，如 songloft-player（官方客户端）、miot（小爱音箱插件）等。type 参数标识事件类型：play（开始播放）、finish（播放完成）、skip（用户跳过）。
// @Tags 歌曲管理
// @Produce json
// @Param id path int true "歌曲 ID"
// @Param source query string false "调用来源标识，如 songloft-player、miot"
// @Param type query string false "事件类型：play、finish、skip，默认 finish" Enums(play, finish, skip)
// @Success 204 "无内容"
// @Failure 400 {object} models.ErrorResponse "无效的歌曲 ID 或事件类型"
// @Failure 404 {object} models.ErrorResponse "歌曲不存在"
// @Security BearerAuth
// @Router /songs/{id}/played [post]
func (h *SongHandler) SongPlayed(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "无效的歌曲 ID", err)
		return
	}

	eventType := r.URL.Query().Get("type")
	if eventType == "" {
		eventType = "finish"
	}
	if eventType != "play" && eventType != "finish" && eventType != "skip" {
		respondError(w, http.StatusBadRequest, "无效的事件类型，必须是 play、finish 或 skip", nil)
		return
	}

	song, err := h.songService.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "歌曲不存在", err)
		return
	}

	if h.playBroadcaster != nil {
		source := r.URL.Query().Get("source")
		go h.playBroadcaster.BroadcastPlayEvent(song.ID, song.Title, song.Artist, eventType, source)
	}

	w.WriteHeader(http.StatusNoContent)
}
