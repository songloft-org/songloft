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
	"sync"
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
	downloadActivity  *services.DownloadActivity // 下载活动闸门，导入探测据此让路（issue #265）
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
	radioClient := httputil.NewStreamingClient()
	radioClient.CheckRedirect = limitStreamRedirects
	return &SongHandler{
		songService:  songService,
		cacheService: cacheService,
		reassigner:   reassigner,
		lyricFetcher: lyricFetcher,
		hlsHandler:   hlsHandler,
		playActivity: playActivity,
		radioClient:  radioClient,
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

// SetDownloadActivity 注入下载活动闸门，导入探测据此为下载让路。
func (h *SongHandler) SetDownloadActivity(a *services.DownloadActivity) {
	h.downloadActivity = a
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
const songCoverProxyTimeout = 5 * time.Second

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

// parseExcludePlaylistLabels 解析歌曲列表的歌单 label 排除参数。
// 缺省（空串）→ 排除隐藏歌单（hidden）；传 none → 不排除；否则按逗号拆分。
// 与歌单列表 ListPlaylists 的 exclude_labels 约定保持一致。
func parseExcludePlaylistLabels(raw string) []string {
	if raw == "" {
		return []string{models.PlaylistLabelHidden}
	}
	if raw == "none" {
		return nil
	}
	return strings.Split(raw, ",")
}

// ListSongs 获取歌曲列表
// @Summary 获取歌曲列表
// @Description 获取歌曲列表，支持按类型过滤、关键词搜索和分页。默认排除隐藏歌单里的歌，传 exclude_playlist_labels=none 显示全部
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param type query string false "歌曲类型" Enums(local, remote, radio)
// @Param keyword query string false "搜索关键词"
// @Param path_prefix query string false "按 file_path 前缀过滤（如 music/Pop）"
// @Param genre query string false "按流派精确过滤"
// @Param artist query string false "按歌手精确过滤"
// @Param album query string false "按专辑精确过滤"
// @Param language query string false "按语种精确过滤"
// @Param style query string false "按风格精确过滤"
// @Param year query int false "按发行年份精确过滤"
// @Param decade query int false "按年代过滤（起始年，如 1990 匹配 1990-1999）"
// @Param exclude_playlist_labels query string false "排除属于这些 label 歌单的歌曲(逗号分隔), 默认 hidden; 传 none 显示全部" default(hidden)
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Param sort query string false "排序字段，缺省 added_at" Enums(id, title, artist, album, duration, added_at, updated_at, file_modified_at, year, genre)
// @Param order query string false "排序方向，缺省 desc" Enums(asc, desc)
// @Success 200 {object} map[string]any "成功返回歌曲列表"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
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
		Type:                  songType,
		Keyword:               keyword,
		PathPrefix:            pathPrefix,
		ExcludePlaylistLabels: parseExcludePlaylistLabels(r.URL.Query().Get("exclude_playlist_labels")),
		Limit:                 limit,
		Offset:                offset,
		OrderBy:               orderBy,
		Order:                 order,
	}
	applySongTagFilters(filter, r.URL.Query())

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
// @Param genre query string false "按流派精确过滤"
// @Param artist query string false "按歌手精确过滤"
// @Param album query string false "按专辑精确过滤"
// @Param language query string false "按语种精确过滤"
// @Param style query string false "按风格精确过滤"
// @Param year query int false "按发行年份精确过滤"
// @Param decade query int false "按年代过滤（起始年，如 1990 匹配 1990-1999）"
// @Param exclude_playlist_labels query string false "排除属于这些 label 歌单的歌曲(逗号分隔), 默认 hidden; 传 none 显示全部" default(hidden)
// @Param sort query string false "排序字段，缺省 added_at" Enums(id, title, artist, album, duration, added_at, updated_at, file_modified_at, year, genre)
// @Param order query string false "排序方向，缺省 desc" Enums(asc, desc)
// @Success 200 {object} map[string]any "成功返回 ID 列表"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/ids [get]
func (h *SongHandler) ListSongIDs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orderBy, order := parseSongSort(r.URL.Query().Get("sort"), r.URL.Query().Get("order"))
	filter := &database.SongFilter{
		Type:                  r.URL.Query().Get("type"),
		Keyword:               r.URL.Query().Get("keyword"),
		PathPrefix:            r.URL.Query().Get("path_prefix"),
		ExcludePlaylistLabels: parseExcludePlaylistLabels(r.URL.Query().Get("exclude_playlist_labels")),
		OrderBy:               orderBy,
		Order:                 order,
	}
	applySongTagFilters(filter, r.URL.Query())

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

// applySongTagFilters 从 query 解析标签分类过滤参数（流派/歌手/专辑/语种/风格/年份/年代）并写入 filter。
func applySongTagFilters(filter *database.SongFilter, q url.Values) {
	filter.Genre = q.Get("genre")
	filter.Artist = q.Get("artist")
	filter.Album = q.Get("album")
	filter.Language = q.Get("language")
	filter.Style = q.Get("style")
	if y, err := strconv.Atoi(q.Get("year")); err == nil && y > 0 {
		filter.Year = y
	}
	if d, err := strconv.Atoi(q.Get("decade")); err == nil && d > 0 {
		filter.DecadeStart = d
	}
}

// songFacetFields 是 /songs/facets 支持的维度白名单。
var songFacetFields = map[string]struct{}{
	"genre": {}, "artist": {}, "album": {},
	"language": {}, "style": {}, "year": {}, "decade": {},
}

// ListSongFacets 按维度聚合曲库标签，返回该维度下所有取值及计数（按计数降序）。
// @Summary 曲库标签分类聚合
// @Description 按指定维度聚合曲库，返回该维度下所有非空取值及各自的歌曲数量，用于「分类浏览」的清单页。
// @Description 支持维度：genre(流派)/artist(歌手)/album(专辑)/language(语种)/style(风格)/year(年份)/decade(年代)。
// @Description year/decade 的 value 为数字字符串（年代如 "1990" 表示 1990-1999）。取到某取值后可用 /songs?<field>=<value> 拉取该分类下歌曲。
// @Tags 歌曲管理
// @Produce json
// @Param field query string true "聚合维度" Enums(genre, artist, album, language, style, year, decade)
// @Success 200 {object} map[string]any "成功返回聚合结果 {field, facets:[{value,count}]}"
// @Failure 400 {object} map[string]string "缺少或不支持的 field"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/facets [get]
func (h *SongHandler) ListSongFacets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	field := r.URL.Query().Get("field")
	if _, ok := songFacetFields[field]; !ok {
		respondError(w, http.StatusBadRequest, "不支持的聚合维度 field", nil)
		return
	}

	facets, err := h.songService.ListFacet(ctx, field)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取标签分类聚合失败", err)
		return
	}
	if facets == nil {
		facets = []database.Facet{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"field":  field,
		"facets": facets,
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

	// 更新歌曲信息
	existingSong.Title = req.Title
	existingSong.Artist = req.Artist
	existingSong.Album = req.Album
	// URL 仅在显式提供(非空)时更新：插件音源歌曲(URL 为空，靠 source_data 播放)没有可编辑的直链，
	// 前端对这类歌曲不回传 url；此处保留原值(空)，避免被清空或被内部播放端点污染。
	if req.URL != "" {
		existingSong.URL = req.URL
	}
	existingSong.CoverURL = req.CoverURL
	if req.IsLive != nil && existingSong.Type != models.TypeRadio {
		existingSong.IsLive = *req.IsLive
	}

	// 非本地歌曲更新后必须仍有可用音源：直链 URL 或插件 source_data。
	if existingSong.Type != models.TypeLocal && existingSong.URL == "" && !existingSong.IsPluginSourced() {
		respondError(w, http.StatusBadRequest, "URL不能为空", nil)
		return
	}

	if err := h.songService.Update(ctx, existingSong); err != nil {
		respondError(w, http.StatusInternalServerError, "更新歌曲失败", err)
		return
	}

	respondJSON(w, http.StatusOK, existingSong)
}

// AddRemoteSongs 批量添加网络歌曲
// @Summary 批量添加网络歌曲
// @Description 批量添加网络歌曲到数据库。cover_url 支持以 "/" 开头的相对路径（插件场景下由服务端自动解析为内部 URL，与歌词 lyric_remote_url 的解析机制一致）。lyric_remote_url 为歌词远程 URL 直传字段，提供时优先于 lyric + lyric_source=url 的间接方式。副作用：插入成功后，对缺失技术元数据（duration/bitrate/samplerate/format）的歌曲异步探测补齐（限并发后台执行，不阻塞响应），确保 WebDAV 等无法自带时长的音源在首次播放前就落库 duration，供音箱等仅依赖服务端时长的消费端自动切歌。
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

	// 导入即探测：对缺失技术元数据的歌曲异步补齐 duration 等字段。
	// WebDAV 等音源无法自带时长，若等到首次播放才懒探测，音箱开播那一刻 duration 仍为 0，
	// 无法注册切歌定时器。提前到导入时探测，确保播放前 duration 已落库。
	h.probeRemoteSongsMetadata(songs)

	respondJSON(w, http.StatusCreated, map[string]any{
		"songs": songs,
		"count": len(songs),
	})
}

// probeRemoteSongsMetadata 对刚导入、缺失技术元数据的网络歌曲发起后台异步探测补齐。
// 限并发（避免整目录导入时打爆服务端与上游音源），每首独立超时；RefreshSong 自带 inflight 去重。
func (h *SongHandler) probeRemoteSongsMetadata(songs []*models.Song) {
	if h.metadataRefresher == nil {
		return
	}

	pending := make([]*models.Song, 0, len(songs))
	for _, song := range songs {
		if services.NeedsMetadata(song) {
			// 复制一份，避免后台 goroutine 与调用方共享指针
			copied := *song
			pending = append(pending, &copied)
		}
	}
	if len(pending) == 0 {
		return
	}

	go func() {
		// issue #265：探测走 ffprobe + ytdlp 插件唯一 worker，与批量下载撞车会打满 CPU 并把
		// 下载解析挤到 30s 超时判死。故 (1) 降并发 4→2 从源头收敛占用；(2) 每首探测前若有活跃
		// 下载则退避让路，把 worker 让给下载解析。探测是尽力而为的后台补齐，让路/延后无副作用。
		const maxConcurrent = 2
		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup
		for _, song := range pending {
			h.waitForDownloadIdle()
			wg.Add(1)
			sem <- struct{}{}
			go func(s *models.Song) {
				defer wg.Done()
				defer func() { <-sem }()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				h.metadataRefresher.RefreshSong(ctx, s, "")
			}(song)
		}
		wg.Wait()
		slog.Info("导入歌曲元数据探测完成", "count", len(pending))
	}()
}

// waitForDownloadIdle 在有活跃下载时退避，把插件 worker 让给下载解析（issue #265）。
// 探测是后台尽力而为的任务，可以等；但设总上限防止下载长时间不停导致探测无限饥饿——
// 到上限后仍继续探测（此时 A 的下载重试 + C 的更长解析超时会兜底瞬时争用）。
func (h *SongHandler) waitForDownloadIdle() {
	if h.downloadActivity == nil {
		return
	}
	const (
		pollInterval = 500 * time.Millisecond
		maxWait      = 5 * time.Minute
	)
	waited := time.Duration(0)
	for h.downloadActivity.Active() && waited < maxWait {
		time.Sleep(pollInterval)
		waited += pollInterval
	}
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
		ServeRemoteResourceWithOptions(w, r, coverURL, RemoteResourceOptions{
			Timeout:      songCoverProxyTimeout,
			ErrorStatus:  http.StatusNotFound,
			ErrorMessage: "cover fetch failed",
		})
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
	// ISO-BMFF 音频容器（mp4/mov/m4a/m4b）显式声明为音频类型:
	// stdlib http.ServeFile 会按扩展名把 .mp4 标成 video/mp4、.mov 标成 video/quicktime,
	// 我们只播放其中的音频轨,显式设 audio/mp4 可提升 Web <audio> 及部分客户端按音频处理的稳健性。
	// 基于最终 srcPath 判断:若已转码为 .mp3 等,则不覆盖,交由 http.ServeFile 给出正确类型。
	switch strings.ToLower(filepath.Ext(srcPath)) {
	case ".mp4", ".mov", ".m4a", ".m4b":
		w.Header().Set("Content-Type", "audio/mp4")
	}
	http.ServeFile(w, r, srcPath)
}

// serveRadio 电台/直播流:专用代理，不设整请求超时、不缓存。
// 与 ServeRemoteResource 不同:客户端断开时由 r.Context() 取消上游请求，不受 60s 硬超时限制。
// Transport 只限制等待响应头的时间，坏源不会让播放器永远转圈。
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
	upstreamReq.Header.Set("User-Agent", streamUserAgent)
	upstreamReq.Header.Set("Accept", streamAccept)
	// Icy-MetaData 透传:仅在客户端显式请求时才向上游要 ICY 元数据。
	// 浏览器 <audio> 既不发此头也不解析交织在音频里的元数据块;若无条件强制 Icy-MetaData:1,
	// 上游会按 icy-metaint 每隔 N 字节插入一段元数据,这些字节被浏览器当作音频解码,
	// 播放约 1 秒(16000 字节 ≈ 1.4s@88kbps)后即崩断(#275)。
	// 原生播放器(mpv/ExoPlayer 等)需要元数据时会自带此头,由下面的 icy-* 头透传闭环。
	clientWantsMeta := r.Header.Get("Icy-MetaData") != ""
	if clientWantsMeta {
		upstreamReq.Header.Set("Icy-MetaData", r.Header.Get("Icy-MetaData"))
	}
	if songURL, err := url.Parse(song.URL); err == nil && songURL.Scheme != "" && songURL.Host != "" {
		upstreamReq.Header.Set("Referer", songURL.Scheme+"://"+songURL.Host+"/")
	}
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

	// body 与 icy-metaint 头的处理分三种情况:
	//   - 客户端请求了 Icy-MetaData(原生播放器)→ 原样透传交织流 + icy-metaint,原生自己解析。
	//   - 客户端没请求但上游仍无条件交织(icy-metaint>0)→ 代理侧去交织,只吐纯音频,
	//     且不转发 icy-metaint;否则浏览器 <audio> 会把元数据块当音频解码而崩断(#275)。
	//   - 客户端没请求且上游也没交织 → 纯 copy。
	var body io.Reader = resp.Body
	forwardMetaint := clientWantsMeta
	if !clientWantsMeta {
		if metaint, err := strconv.Atoi(resp.Header.Get("icy-metaint")); err == nil && metaint > 0 {
			body = httputil.NewICYDeinterleaveReader(resp.Body, metaint)
		}
	}
	// 透传 ICY 头:icy-metaint 仅在原生路径转发(浏览器路径已去交织,转发反而误导);
	// 其余 icy-* 是纯 HTTP 头,对浏览器无害,一律透传。
	for _, hdr := range []string{"icy-metaint", "icy-name", "icy-genre", "icy-br", "icy-description", "icy-url", "icy-pub", "icy-audio-info"} {
		if hdr == "icy-metaint" && !forwardMetaint {
			continue
		}
		if v := resp.Header.Get(hdr); v != "" {
			w.Header().Set(hdr, v)
		}
	}
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, body)
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
	Language   string `json:"language"`
	Style      string `json:"style"`
	Track      string `json:"track"`
	Lyrics     string `json:"lyrics"`
	CoverData  string `json:"cover_data"`
	CoverURL   string `json:"cover_url"`
	ClearCover bool   `json:"clear_cover"`
	// RenameFile 为 true 时按新标题重命名本地音频文件（保留原目录与扩展名），仅对本地非 CUE 歌曲生效。
	RenameFile bool `json:"rename_file"`
}

// WriteTags 写入歌曲标签
// @Summary 写入歌曲标签
// @Description 将元数据写入数据库和本地音频文件标签（仅本地歌曲）。cover_data(base64) 优先于 cover_url。非空字段覆盖，空值保留原值。设置 clear_cover=true 可显式清空封面。rename_file=true 时按新标题重命名本地音频文件（保留原目录与扩展名，仅本地非 CUE 歌曲生效）；标题清理后为空或目标文件名已存在时返回 400，与原文件同名则不移动仅写库。
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
	if req.Language != "" {
		song.Language = req.Language
	}
	if req.Style != "" {
		song.Style = req.Style
	}
	if req.Track != "" {
		song.Track = req.Track
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

	// rename_file=true 且为本地非 CUE 歌曲时，按新标题重命名文件（内部完成文件移动 + DB 写回）；
	// 否则走普通 DB 更新。两种路径完成后都用最新 FilePath 写文件标签。
	if req.RenameFile && song.Type == models.TypeLocal && song.CueSourcePath == "" {
		if _, err := h.songService.RenameLocalSongFile(ctx, song, song.Title); err != nil {
			respondError(w, http.StatusBadRequest, "重命名文件失败", err)
			return
		}
	} else if err := h.songService.Update(ctx, song); err != nil {
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
// @Description 批量移动/重命名本地歌曲文件到指定目录结构。target_path 为相对于 music_path 的路径（含目录和文件名），扩展名必须与原文件一致。CUE 拆分歌曲会被跳过（status=skip）；目标文件已存在时拒绝覆盖（status=error）。music_path 由服务端自取。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body []services.OrganizeItem true "整理项目列表"
// @Success 200 {array} services.OrganizeResult "整理结果"
// @Failure 400 {object} map[string]string "请求错误"
// @Security BearerAuth
// @Router /api/v1/songs/organize [post]
func (h *SongHandler) OrganizeSongs(w http.ResponseWriter, r *http.Request) {
	var items []services.OrganizeItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}
	if len(items) == 0 {
		respondError(w, http.StatusBadRequest, "列表不能为空", nil)
		return
	}

	results := h.songService.OrganizeSongs(r.Context(), items)
	respondJSON(w, http.StatusOK, results)
}

// PreviewOrganizeSongs 预览批量整理
// @Summary 预览批量整理歌曲文件
// @Description dry-run 预览目录整理变更，返回每项 old_path→new_path 与状态（ok/conflict/skip/error），不移动任何文件、不改数据库。target_path 为相对 music_path 的路径。CUE 歌曲 skip；目标已存在或批内撞名 conflict。music_path 由服务端自取。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body []services.OrganizeItem true "整理项目列表"
// @Success 200 {array} services.OrganizePreviewResult "预览结果"
// @Failure 400 {object} map[string]string "请求错误"
// @Security BearerAuth
// @Router /api/v1/songs/organize/preview [post]
func (h *SongHandler) PreviewOrganizeSongs(w http.ResponseWriter, r *http.Request) {
	var items []services.OrganizeItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}
	if len(items) == 0 {
		respondError(w, http.StatusBadRequest, "列表不能为空", nil)
		return
	}

	results := h.songService.PreviewOrganize(r.Context(), items)
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
