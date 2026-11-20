package handlers

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
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
	SearchLyrics(ctx context.Context, title, artist, album string, duration float64, fingerprint string, isrc string) (*models.LyricPayload, error)
}

// CoverSearcher 封面搜索接口（由 JS 插件管理器实现）
type CoverSearcher interface {
	SearchCover(ctx context.Context, title, artist, album string, fingerprint string, isrc string) (string, error)
}

// PlayHistoryRecorder 记录「某播放上下文内播了某首歌」（由 PlayHistoryService 实现）
type PlayHistoryRecorder interface {
	Record(ctx context.Context, contextType, contextKey string, songID int64, playedAt time.Time) error
}

// SongHandler 歌曲处理器
type SongHandler struct {
	songService       *services.SongService
	cacheService      *services.CacheService
	reassigner        AsyncReassigner
	lyricFetcher      *services.LyricFetcher // 解包插件 JSON 拿 LRC 文本(歌词 url 分支用)
	hlsHandler        *HLSHandler            // 电台 HLS 流的反代委托（开关在 HLSHandler 内）
	hlsTranscode      *services.HLSTranscodeService
	playActivity      *playactivity.Registry // 跟踪进行中的 play/prefetch/transcode/reassign 工作，用户切歌时一次性 cancel
	getMusicPath      func() string          // 获取 music_path（由 scanner.GetMusicPath 注入）
	playBroadcaster   PlayEventBroadcaster   // JS 插件播放事件广播（可选，nil 安全）
	playHistory       PlayHistoryRecorder    // 播放历史落库（可选，nil 安全）
	lyricSearcher     LyricSearcher          // 歌词提供者搜索（可选，nil 安全）
	coverSearcher     CoverSearcher          // 封面提供者搜索（可选，nil 安全）
	metadataRefresher *services.MetadataRefresher
	configService     *services.ConfigService
	urlResolver       *services.InternalURLResolver // 把插件相对路径解析为本机绝对 URL + access_token（封面代理用）
	radioClient       *http.Client
	downloadActivity  *services.DownloadActivity // 下载活动闸门，导入探测据此让路（issue #265）
	thumbCache        *services.CoverThumbCache  // 缩略图磁盘缓存（可选，nil 安全）
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

func (h *SongHandler) SetHLSTranscodeService(s *services.HLSTranscodeService) {
	h.hlsTranscode = s
}

// SetGetMusicPath 注入 music_path 获取函数。
func (h *SongHandler) SetGetMusicPath(fn func() string) {
	h.getMusicPath = fn
}

// SetPlayBroadcaster 注入 JS 插件播放事件广播器。
func (h *SongHandler) SetPlayBroadcaster(b PlayEventBroadcaster) {
	h.playBroadcaster = b
}

// SetPlayHistoryRecorder 注入播放历史记录器。
func (h *SongHandler) SetPlayHistoryRecorder(r PlayHistoryRecorder) {
	h.playHistory = r
}

// SetLyricSearcher 注入歌词搜索器（由 JS 插件管理器实现）。
func (h *SongHandler) SetLyricSearcher(s LyricSearcher) {
	h.lyricSearcher = s
}

// SetCoverSearcher 注入封面搜索器（由 JS 插件管理器实现）。
func (h *SongHandler) SetCoverSearcher(s CoverSearcher) {
	h.coverSearcher = s
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

// SetThumbCache 注入缩略图磁盘缓存。
func (h *SongHandler) SetThumbCache(tc *services.CoverThumbCache) {
	h.thumbCache = tc
}

const remoteTitleSourceConfigKey = "remote_title_source"
const volumeNormalizeConfigKey = "volume_normalize"

// 目标响度（LUFS）的 config key 在 services.CacheService 里声明并读取（构建 loudnorm 滤镜），
// handler 只负责写入，直接引用 services.volumeNormalizeLoudnessKey 避免字符串漂移。
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

// volumeNormalizeRequest /settings/volume-normalize 请求/响应体
// Loudness 为目标响度（LUFS）：GET 恒返回当前值；PUT 可省略（omitempty + 指针），
// 省略时不改响度配置（向后兼容旧前端只发 {enabled}）。
type volumeNormalizeRequest struct {
	Enabled  bool     `json:"enabled" example:"false"`
	Loudness *float64 `json:"loudness,omitempty" example:"-16"`
}

// GetVolumeNormalizeSetting GET /api/v1/settings/volume-normalize
// @Summary 获取音量均衡配置
// @Description 返回是否启用 EBU R128 音量均衡，以及目标响度（LUFS，默认 -16）。启用后，播放请求未显式携带 normalize 参数时，服务端自动对音频执行 loudnorm 滤镜。默认关闭。
// @Tags 设置
// @Produce json
// @Success 200 {object} volumeNormalizeRequest "当前启用状态与目标响度"
// @Security BearerAuth
// @Router /settings/volume-normalize [get]
func (h *SongHandler) GetVolumeNormalizeSetting(w http.ResponseWriter, r *http.Request) {
	enabled := false
	if h.configService != nil {
		enabled = h.configService.GetBool(volumeNormalizeConfigKey, false)
	}
	// NormalizeLoudness 是 nil-safety 方法：h.cacheService 为 nil（测试场景）时返回默认 -16。
	loudness := h.cacheService.NormalizeLoudness()
	respondJSON(w, http.StatusOK, volumeNormalizeRequest{Enabled: enabled, Loudness: &loudness})
}

// UpdateVolumeNormalizeSetting PUT /api/v1/settings/volume-normalize
// @Summary 更新音量均衡配置
// @Description 启用或关闭 EBU R128 音量均衡，并可选地设置目标响度（LUFS，范围 -40 ~ -5，默认 -16）。启用后，所有不含显式 normalize 查询参数的播放请求将自动应用 loudnorm 滤镜（需要 ffmpeg）。loudness 字段可省略：省略时仅切换开关、不改动响度配置（向后兼容旧前端）。
// @Tags 设置
// @Accept json
// @Produce json
// @Param request body volumeNormalizeRequest true "启用状态与（可选）目标响度"
// @Success 200 {object} volumeNormalizeRequest "更新后的启用状态与目标响度"
// @Failure 400 {object} map[string]string "请求格式错误或响度越界"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/volume-normalize [put]
func (h *SongHandler) UpdateVolumeNormalizeSetting(w http.ResponseWriter, r *http.Request) {
	if h.configService == nil {
		respondError(w, http.StatusInternalServerError, "configService 未注入", nil)
		return
	}
	var req volumeNormalizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	// 可选的目标响度：提供则校验 + 写入；省略则保持现有配置不动。
	if req.Loudness != nil {
		if err := services.ValidateNormalizeLoudness(*req.Loudness); err != nil {
			respondError(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		if err := h.configService.Set(services.VolumeNormalizeLoudnessKey, strconv.FormatFloat(*req.Loudness, 'g', -1, 64)); err != nil {
			respondError(w, http.StatusInternalServerError, "保存响度配置失败", err)
			return
		}
	}
	val := "false"
	if req.Enabled {
		val = "true"
	}
	if err := h.configService.Set(volumeNormalizeConfigKey, val); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	// 回显当前生效值：响度从 config 读回（已含刚写入的值或既有值），保证与下次 GET 一致。
	loudness := h.cacheService.NormalizeLoudness()
	respondJSON(w, http.StatusOK, volumeNormalizeRequest{Enabled: req.Enabled, Loudness: &loudness})
}

// StartMetadataRefresh 触发刷新歌曲元数据
// @Summary 刷新歌曲元数据
// @Description 对所有元数据缺失且本地有文件的歌曲（本地歌曲及已缓存的网络歌曲）从文件提取时长、比特率、采样率、格式及标签并回填。未缓存的网络歌曲不参与，其元数据在播放缓存落盘后自动回填。已在运行时返回 409。
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
		if orderBy == "track" || orderBy == "rating" {
			dir = "ASC"
		}
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

// parseSongTypeQuery validates the comma-separated songs type filter while
// preserving the raw value for the repository filter.
func parseSongTypeQuery(r *http.Request) (string, error) {
	values := r.URL.Query()["type"]
	raw := strings.Join(values, ",")
	_, err := database.ParseSongTypes(raw)
	return raw, err
}

// ListSongs 获取歌曲列表
// @Summary 获取歌曲列表
// @Description 获取歌曲列表，支持按类型过滤、关键词搜索和分页。默认排除隐藏歌单里的歌，传 exclude_playlist_labels=none 显示全部
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param type query string false "歌曲类型，支持 local、remote、radio 的逗号分隔组合；不传表示全部类型"
// @Param keyword query string false "搜索关键词"
// @Param path_prefix query string false "按 file_path 前缀过滤（如 music/Pop）"
// @Param genre query string false "按流派精确过滤"
// @Param artist query string false "按歌手精确过滤"
// @Param album query string false "按专辑精确过滤"
// @Param language query string false "按语种精确过滤"
// @Param style query string false "按风格精确过滤"
// @Param year query int false "按发行年份精确过滤"
// @Param decade query int false "按年代过滤（起始年，如 1990 匹配 1990-1999）"
// @Param rating query int false "按评级精确过滤（0-100；0=未评级；-1 表示只要未评级歌曲）"
// @Param min_rating query int false "按评级下限过滤（如 60 = 至少 3 星）；与 rating 同时出现时 rating=-1 优先生效"
// @Param exclude_playlist_labels query string false "排除属于这些 label 歌单的歌曲(逗号分隔), 默认 hidden; 传 none 显示全部" default(hidden)
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Param sort query string false "排序字段；指定 album 时缺省 track（碟号、音轨号数字排序），其他情况缺省 added_at；未知轨号排在同碟末尾" Enums(id, title, artist, album, duration, added_at, updated_at, file_modified_at, year, genre, file_size, track, rating)
// @Param order query string false "排序方向，track/rating 缺省 asc，其他字段缺省 desc" Enums(asc, desc)
// @Success 200 {object} map[string]any "成功返回歌曲列表"
// @Failure 400 {object} map[string]string "歌曲类型参数无效"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs [get]
func (h *SongHandler) ListSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 解析查询参数
	songType, err := parseSongTypeQuery(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "歌曲类型参数无效", err)
		return
	}
	keyword := r.URL.Query().Get("keyword")
	pathPrefix := r.URL.Query().Get("path_prefix")
	orderBy, order := parseSongSort(r.URL.Query().Get("sort"), r.URL.Query().Get("order"))

	limit, offset := parsePagination(r, models.DefaultPaginationLimit, models.MaxPaginationLimit)

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

// ListRandomSongs 随机返回匹配过滤条件的 N 首歌曲（完整对象）。
// @Summary 随机获取歌曲
// @Description 与 /songs 共享过滤条件，随机返回 limit 首歌曲完整对象。用于「随机播放」场景。
// @Description 返回字段与 GET /songs 一致（数组包裹在 songs 字段中），total 为实际返回数量。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param type query string false "歌曲类型，支持 local、remote、radio 的逗号分隔组合；不传表示全部类型"
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
// @Param limit query int false "随机返回数量，默认 50，上限 500" default(50)
// @Success 200 {object} map[string]any "成功返回随机歌曲列表"
// @Failure 400 {object} map[string]string "歌曲类型参数无效"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/random [get]
func (h *SongHandler) ListRandomSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	songType, err := parseSongTypeQuery(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "歌曲类型参数无效", err)
		return
	}

	limit, _ := parsePagination(r, 50, 500)

	filter := &database.SongFilter{
		Type:                  songType,
		Keyword:               r.URL.Query().Get("keyword"),
		PathPrefix:            r.URL.Query().Get("path_prefix"),
		ExcludePlaylistLabels: parseExcludePlaylistLabels(r.URL.Query().Get("exclude_playlist_labels")),
		Limit:                 limit,
	}
	applySongTagFilters(filter, r.URL.Query())

	songs, err := h.songService.ListRandom(ctx, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取随机歌曲失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"songs": songs,
		"total": len(songs),
	})
}

// ListSongIDs 返回匹配 filter 的歌曲 ID 列表（不分页、不带 song 详情）
// @Summary 获取匹配歌曲的 ID 列表
// @Description 与 /songs 共享过滤条件，仅返回 ID。用于「全选当前筛选范围」场景。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param type query string false "歌曲类型，支持 local、remote、radio 的逗号分隔组合；不传表示全部类型"
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
// @Param sort query string false "排序字段；指定 album 时缺省 track（碟号、音轨号数字排序），其他情况缺省 added_at；未知轨号排在同碟末尾" Enums(id, title, artist, album, duration, added_at, updated_at, file_modified_at, year, genre, file_size, track)
// @Param order query string false "排序方向，track 缺省 asc，其他字段缺省 desc" Enums(asc, desc)
// @Success 200 {object} map[string]any "成功返回 ID 列表"
// @Failure 400 {object} map[string]string "歌曲类型参数无效"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/ids [get]
func (h *SongHandler) ListSongIDs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	songType, err := parseSongTypeQuery(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "歌曲类型参数无效", err)
		return
	}

	orderBy, order := parseSongSort(r.URL.Query().Get("sort"), r.URL.Query().Get("order"))
	filter := &database.SongFilter{
		Type:                  songType,
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
	if filter.Album != "" && q.Get("sort") == "" {
		filter.OrderBy, filter.Order = parseSongSort("track", q.Get("order"))
	}
	filter.Language = q.Get("language")
	filter.Style = q.Get("style")
	if y, err := strconv.Atoi(q.Get("year")); err == nil && y > 0 {
		filter.Year = y
	}
	if d, err := strconv.Atoi(q.Get("decade")); err == nil && d > 0 {
		filter.DecadeStart = d
	}
	if tagID, err := strconv.ParseInt(q.Get("tag_id"), 10, 64); err == nil && tagID > 0 {
		filter.TagID = tagID
	}
	filter.TagName = q.Get("tag")
	// 评级过滤：min_rating 为「≥ 下限」（如 20 = 至少 1 星）；
	// rating 为精确匹配，0 合法（未评级），-1 表示「未评级」专用筛选。
	if mr, err := strconv.Atoi(q.Get("min_rating")); err == nil && mr > 0 {
		filter.MinRating = mr
	}
	if raw := q.Get("rating"); raw != "" {
		if rv, err := strconv.Atoi(raw); err == nil {
			if rv >= 0 {
				filter.ExactRating = &rv
			} else {
				zero := 0
				filter.ExactRating = &zero
				filter.MinRating = 0
			}
		}
	}
}

// songFacetFields 是 /songs/facets 支持的维度白名单。
// tag 维度不映射到 songs 的单列，而是按用户自定义标签聚合（见 SongRepository.ListFacet 的 tag 分支）。
var songFacetFields = map[string]struct{}{
	"genre": {}, "artist": {}, "album": {},
	"language": {}, "style": {}, "year": {}, "decade": {},
	"rating": {},
	"tag":    {},
}

// ListSongFacets 按维度聚合曲库标签，返回该维度下的取值 + 计数 + 代表封面（支持搜索/排序/分页）。
// @Summary 曲库标签分类聚合
// @Description 按指定维度聚合曲库，返回该维度下非空取值、各自的歌曲数量及一首代表歌曲的封面 URL，用于「分类浏览」的卡片网格。
// @Description 支持维度：genre(流派)/artist(歌手)/album(专辑)/language(语种)/style(风格)/year(年份)/decade(年代)/rating(评级)/tag(标签)。
// @Description year/decade/rating 的 value 为数字字符串（年代如 "1990" 表示 1990-1999；rating 为 20-100 的评级值，未评级歌曲不参与聚合）。取到某取值后可用 /songs?<field>=<value> 拉取该分类下歌曲（tag 维度除外——tag 的 value 为标签名，需用 /songs?tag_name=<value> 过滤）。
// @Description 支持 keyword 模糊搜索取值、limit/offset 分页、sort(count|name)/order 排序；返回 total 为该维度去重取值总数。
// @Tags 歌曲管理
// @Produce json
// @Param field query string true "聚合维度" Enums(genre, artist, album, language, style, year, decade, rating, tag)
// @Param type query string false "歌曲类型，支持 local、remote、radio 的逗号分隔组合；不传表示全部类型"
// @Param exclude_playlist_labels query string false "排除属于这些 label 歌单的歌曲(逗号分隔), 默认 hidden; 传 none 显示全部"
// @Param keyword query string false "对取值模糊搜索"
// @Param limit query int false "分页大小，缺省 20，上限 100000"
// @Param offset query int false "分页偏移，缺省 0"
// @Param sort query string false "排序维度，缺省 count" Enums(count, name)
// @Param order query string false "排序方向；count 缺省 desc，name 缺省 asc" Enums(asc, desc)
// @Success 200 {object} map[string]any "成功返回聚合结果 {field, facets:[{value,count,cover_url}], total, limit, offset}"
// @Failure 400 {object} map[string]string "缺少或不支持的 field，或歌曲类型参数无效"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/facets [get]
func (h *SongHandler) ListSongFacets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	songType, err := parseSongTypeQuery(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "歌曲类型参数无效", err)
		return
	}

	field := r.URL.Query().Get("field")
	if _, ok := songFacetFields[field]; !ok {
		respondError(w, http.StatusBadRequest, "不支持的聚合维度 field", nil)
		return
	}

	keyword := r.URL.Query().Get("keyword")
	limit, offset := parsePagination(r, models.DefaultPaginationLimit, models.MaxPaginationLimit)

	filter := &database.FacetFilter{
		Type:                  songType,
		ExcludePlaylistLabels: parseExcludePlaylistLabels(r.URL.Query().Get("exclude_playlist_labels")),
		Keyword:               keyword,
		OrderBy:               r.URL.Query().Get("sort"),
		Order:                 r.URL.Query().Get("order"),
		Limit:                 limit,
		Offset:                offset,
	}

	facets, err := h.songService.ListFacet(ctx, field, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取标签分类聚合失败", err)
		return
	}
	if facets == nil {
		facets = []database.Facet{}
	}

	total, err := h.songService.CountFacetFiltered(ctx, field, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取标签分类总数失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"field":  field,
		"facets": facets,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// songNameFields 是 /songs/names 支持的维度白名单。
var songNameFields = map[string]struct{}{
	"title": {}, "artist": {},
}

// ListSongNames 返回曲库中某维度（歌名/歌手名）的全部去重取值，无冗余字段。
// @Summary 曲库歌名/歌手名清单
// @Description 一次性返回曲库中指定维度的全部去重、非空取值（按名称升序），不分页、无计数、无封面等冗余字段。
// @Description 支持维度：title(歌名)/artist(歌手名)。供 TV 等第三方客户端拉取曲库名录后本地搜索匹配，替代「分页拉全部歌曲再自行去重」的浪费。
// @Description artist 按曲库原始整串返回（不按 / 、等分隔符拆分），返回的名字可直接回填 /songs?artist=<value> 精确过滤。
// @Tags 歌曲管理
// @Produce json
// @Param field query string true "维度" Enums(title, artist)
// @Param type query string false "歌曲类型，支持 local、remote、radio 的逗号分隔组合；不传表示全部类型"
// @Param exclude_playlist_labels query string false "排除属于这些 label 歌单的歌曲(逗号分隔), 默认 hidden; 传 none 显示全部"
// @Success 200 {object} map[string]any "成功返回 {field, names:[...], total}"
// @Failure 400 {object} map[string]string "缺少或不支持的 field，或歌曲类型参数无效"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/names [get]
func (h *SongHandler) ListSongNames(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	songType, err := parseSongTypeQuery(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "歌曲类型参数无效", err)
		return
	}

	field := r.URL.Query().Get("field")
	if _, ok := songNameFields[field]; !ok {
		respondError(w, http.StatusBadRequest, "不支持的维度 field", nil)
		return
	}

	names, err := h.songService.ListDistinctNamesFiltered(ctx, field, &database.SongFilter{
		Type:                  songType,
		ExcludePlaylistLabels: parseExcludePlaylistLabels(r.URL.Query().Get("exclude_playlist_labels")),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取曲库名录失败", err)
		return
	}
	if names == nil {
		names = []string{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"field": field,
		"names": names,
		"total": len(names),
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

// songArtistsResponse 歌曲参与歌手响应体。
type songArtistsResponse struct {
	Artists []models.SongArtist `json:"artists"`
}

// songArtistsRequest PUT /songs/{id}/artists 请求体：整组替换的参与歌手输入。
type songArtistsRequest struct {
	Artists []models.ArtistInput `json:"artists"`
}

// GetSongArtists 获取歌曲的参与歌手
// @Summary 获取歌曲参与歌手
// @Description 返回一首歌的全部参与歌手（含角色 artist/album_artist 与顺序）。多值歌手由此拆分展示，供前端编辑界面加载当前状态。
// @Tags 歌曲管理
// @Produce json
// @Param id path int true "歌曲 ID"
// @Success 200 {object} songArtistsResponse "参与歌手列表"
// @Failure 400 {object} map[string]string "无效的歌曲 ID"
// @Failure 404 {object} map[string]string "歌曲不存在"
// @Security BearerAuth
// @Router /songs/{id}/artists [get]
func (h *SongHandler) GetSongArtists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "无效的歌曲 ID", err)
		return
	}
	artists, err := h.songService.GetSongArtists(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			respondError(w, http.StatusNotFound, "歌曲不存在", err)
			return
		}
		respondError(w, http.StatusInternalServerError, "获取参与歌手失败", err)
		return
	}
	if artists == nil {
		artists = []models.SongArtist{}
	}
	respondJSON(w, http.StatusOK, songArtistsResponse{Artists: artists})
}

// SetSongArtists 全量更新歌曲参与歌手
// @Summary 全量更新歌曲参与歌手
// @Description 用请求体整组替换一首歌的参与歌手（删旧建新）。请求体 artists 数组每一项含 name/role/position：role 取 artist（主唱/表演者）或 album_artist（专辑歌手），缺省 artist；position 为同角色内展示顺序（可省略）。主要解决对唱/合唱歌曲只存了一个歌手、按搭档检索不到的问题——可在此手动补录搭档。同时按 role=artist 的名字重建 songs.artist 显示串（对唱得到 "A & B"）。角色非法返回 400；歌曲不存在返回 404。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param id path int true "歌曲 ID"
// @Param artists body songArtistsRequest true "参与歌手全量列表（整组替换）"
// @Success 200 {object} songArtistsResponse "更新后的参与歌手列表"
// @Failure 400 {object} map[string]string "无效的歌曲 ID 或角色非法"
// @Failure 404 {object} map[string]string "歌曲不存在"
// @Security BearerAuth
// @Router /songs/{id}/artists [put]
func (h *SongHandler) SetSongArtists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "无效的歌曲 ID", err)
		return
	}
	var req songArtistsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}
	artists, err := h.songService.SetSongArtists(ctx, id, req.Artists)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			respondError(w, http.StatusNotFound, "歌曲不存在", err)
		case strings.Contains(err.Error(), "invalid artist role"):
			respondError(w, http.StatusBadRequest, err.Error(), err)
		default:
			respondError(w, http.StatusInternalServerError, "更新参与歌手失败", err)
		}
		return
	}
	if artists == nil {
		artists = []models.SongArtist{}
	}
	respondJSON(w, http.StatusOK, songArtistsResponse{Artists: artists})
}

// audioTracksResponse GET /songs/{id}/audio-tracks 响应体。
type audioTracksResponse struct {
	Tracks []services.AudioTrackInfo `json:"tracks"`
}

// GetSongAudioTracks 获取歌曲音频流列表
// @Summary 获取歌曲音频流列表
// @Description 用 ffprobe 探测该歌曲文件的音频流，返回每条流的 audio-relative index（对应 ffmpeg -map 0:a:N）、title、language、codec、default。主要用于 Web 端双音轨（原唱/伴奏 mka）切换：前端据 tracks 数量决定是否显示切轨入口，并用 index 调 /songs/{id}/play?track=N 抽轨播放。仅本地歌曲（或已落地缓存的网络歌曲）有文件可探测；无可探测文件或音频流 < 2 条时也正常返回（前端据此不显示切轨）。运行时按需探测，不落库。
// @Tags 歌曲管理
// @Produce json
// @Param id path int true "歌曲 ID"
// @Success 200 {object} audioTracksResponse "音频流列表"
// @Failure 400 {object} map[string]string "无效的歌曲 ID"
// @Failure 404 {object} map[string]string "歌曲不存在"
// @Security BearerAuth
// @Router /songs/{id}/audio-tracks [get]
func (h *SongHandler) GetSongAudioTracks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "无效的歌曲 ID", err)
		return
	}
	song, err := h.songService.GetByID(ctx, id)
	if err != nil || song == nil {
		respondError(w, http.StatusNotFound, "歌曲不存在", err)
		return
	}

	filePath := h.audioTrackProbePath(song)
	if filePath == "" {
		respondJSON(w, http.StatusOK, audioTracksResponse{Tracks: []services.AudioTrackInfo{}})
		return
	}

	tracks, err := h.cacheService.ListAudioTracks(ctx, filePath)
	if err != nil {
		slog.Warn("probe audio tracks failed", "songId", id, "path", filePath, "error", err)
		respondJSON(w, http.StatusOK, audioTracksResponse{Tracks: []services.AudioTrackInfo{}})
		return
	}
	if tracks == nil {
		tracks = []services.AudioTrackInfo{}
	}
	respondJSON(w, http.StatusOK, audioTracksResponse{Tracks: tracks})
}

// trackItem 是 GET /songs/{id}/tracks 返回数组中的单个元素。
type trackItem struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
}

// GetSongTracks 枚举歌曲文件中的音频轨道
// @Summary 枚举歌曲音频轨道
// @Description 用 ffprobe 探测该歌曲文件的所有音频流，返回每条流的 index、codec、language、title。如果 ffprobe 不可用或执行失败，返回空数组而不报错。
// @Tags 歌曲管理
// @Produce json
// @Param id path int true "歌曲 ID"
// @Success 200 {array} trackItem "音频轨道列表"
// @Failure 400 {object} map[string]string "无效的歌曲 ID"
// @Failure 404 {object} map[string]string "歌曲不存在"
// @Security BearerAuth
// @Router /songs/{id}/tracks [get]
func (h *SongHandler) GetSongTracks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "无效的歌曲 ID", err)
		return
	}
	song, err := h.songService.GetByID(ctx, id)
	if err != nil || song == nil {
		respondError(w, http.StatusNotFound, "歌曲不存在", err)
		return
	}

	filePath := h.audioTrackProbePath(song)
	if filePath == "" {
		respondJSON(w, http.StatusOK, []trackItem{})
		return
	}

	tracks, err := h.cacheService.ListAudioTracks(ctx, filePath)
	if err != nil {
		slog.Warn("probe tracks failed", "songId", id, "path", filePath, "error", err)
		respondJSON(w, http.StatusOK, []trackItem{})
		return
	}

	result := make([]trackItem, 0, len(tracks))
	for _, t := range tracks {
		result = append(result, trackItem{
			Index:    t.Index,
			Codec:    t.Codec,
			Language: t.Language,
			Title:    t.Title,
		})
	}
	if len(result) == 0 {
		result = []trackItem{}
	}
	respondJSON(w, http.StatusOK, result)
}

// audioTrackProbePath 返回可供 ffprobe 探测音频流的本地文件路径。
// 本地歌曲 → FilePath；网络歌曲 → 已落地缓存（cache_path 或旧格式缓存）；均不可用时返回空串。
func (h *SongHandler) audioTrackProbePath(song *models.Song) string {
	if song.Type == models.TypeLocal {
		return song.FilePath
	}
	if p, ok := h.cacheService.FindCachedFileBySong(song); ok {
		return p
	}
	return ""
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
// @Description 更新网络歌曲或电台。电台可将 radio_mode 设为 hls、flv 或 audio_only（服务端输出 MP3），空字符串恢复按后缀自动识别，省略则保留原模式。纯音频模式清除 is_video。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param id path int true "歌曲ID"
// @Param request body object{title=string,artist=string,album=string,url=string,cover_url=string,is_live=boolean,is_video=boolean,radio_mode=string} true "歌曲信息"
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
		Title     string            `json:"title"`
		Artist    string            `json:"artist"`
		Album     string            `json:"album"`
		URL       string            `json:"url"`
		CoverURL  string            `json:"cover_url"`
		IsLive    *bool             `json:"is_live"`
		IsVideo   *bool             `json:"is_video"` // 是否含视频画面;仅在显式提供时更新
		RadioMode *models.RadioMode `json:"radio_mode"`
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
	// is_video 适用于网络歌曲与电台(视频画面/视频电台),仅在显式提供时更新。
	if req.IsVideo != nil {
		existingSong.IsVideo = *req.IsVideo
	}
	if req.RadioMode != nil {
		if existingSong.Type != models.TypeRadio {
			respondError(w, http.StatusBadRequest, "radio_mode 仅适用于电台", nil)
			return
		}
		existingSong.RadioMode = *req.RadioMode
	}
	if !existingSong.RadioMode.ValidForSource(existingSong.URL) {
		respondError(w, http.StatusBadRequest, "电台播放模式或 HTTP/HTTPS 源地址无效", nil)
		return
	}
	if existingSong.RadioMode == models.RadioModeAudioOnly {
		existingSong.IsVideo = false
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
// @Param request body []object{url=string,title=string,artist=string,album=string,cover_url=string,duration=number,plugin_entry_path=string,source_data=string,dedup_key=string,lyric=string,lyric_source=string,lyric_remote_url=string,is_video=boolean} true "网络歌曲列表"
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
		IsVideo         bool    `json:"is_video"` // 是否含视频画面;网络歌曲不走扫描 ffprobe,由调用方(客户端开关)显式声明
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
			IsVideo:         req.IsVideo,
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
// 实际逻辑见 MetadataRefresher.RefreshSongsBackground（HTTP 导入与 jsplugin 桥接导入共用）。
func (h *SongHandler) probeRemoteSongsMetadata(songs []*models.Song) {
	h.metadataRefresher.RefreshSongsBackground(songs, h.waitForDownloadIdle)
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
// @Description 批量添加电台。radio_mode 缺省时按 URL 后缀自动识别；hls 和 flv 可显式标记无后缀的直播入口；audio_only 从 HTTP/HTTPS HLS、TS、MP4 或 FLV 源实时提取第一条音轨并输出 MP3，需 FFmpeg。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body []object{url=string,title=string,artist=string,cover_url=string,is_video=boolean,radio_mode=string} true "电台/广播列表"
// @Success 201 {object} object{songs=[]models.Song,count=int} "添加成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "添加失败"
// @Security BearerAuth
// @Router /songs/radio [post]
func (h *SongHandler) AddRadios(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var reqs []struct {
		URL       string           `json:"url"`
		Title     string           `json:"title"`
		Artist    string           `json:"artist"`
		CoverURL  string           `json:"cover_url"`
		IsVideo   bool             `json:"is_video"` // 是否为视频电台(直播画面);由调用方(客户端开关)显式声明
		RadioMode models.RadioMode `json:"radio_mode"`
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
		if !req.RadioMode.ValidForSource(req.URL) {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("第 %d 条：电台播放模式或 HTTP/HTTPS 源地址无效", i+1), nil)
			return
		}
		inputs = append(inputs, services.RadioInput{
			URL:       req.URL,
			Title:     req.Title,
			Artist:    req.Artist,
			CoverURL:  req.CoverURL,
			IsVideo:   req.IsVideo,
			RadioMode: req.RadioMode,
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
// @Description 根据歌曲 ID 获取封面图片。优先使用本地封面文件（CoverPath），其次代理 CoverURL。CoverURL 支持以 "/" 开头的相对路径，服务端自动经 InternalURLResolver 解析为内部 URL（含 access_token），用于插件歌曲封面代理。可选 query 参数 w：把本地封面等比缩放到该宽度（物理像素，绝不放大、上限 1024）后以 JPEG 返回，用于 Web 端降低 GPU 纹理体积（songloft-org/songloft#309）；缺省或非法时返回原图。缩略仅作用于本地封面，远程代理封面忽略 w。
// @Tags 歌曲管理
// @Produce image/jpeg
// @Param id path int true "歌曲 ID"
// @Param w query int false "本地封面缩略目标宽度（物理像素，绝不放大，上限 1024）"
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
		ServeRemoteResourceWithOptions(w, requestWithoutCoverRange(r), coverURL, RemoteResourceOptions{
			Timeout:      songCoverProxyTimeout,
			ErrorStatus:  http.StatusNotFound,
			ErrorMessage: "cover fetch failed",
		})
		return
	}

	// 本地封面和远程 URL 都不存在时，尝试从已注册的封面提供者插件获取
	if h.coverSearcher != nil {
		if coverURL, err := h.coverSearcher.SearchCover(ctx, song.Title, song.Artist, song.Album, song.Fingerprint, song.ISRC); err == nil && coverURL != "" {
			go h.songService.UpdateCoverURL(context.Background(), song.ID, coverURL)
			resolvedURL := coverURL
			if h.urlResolver != nil {
				resolvedURL = h.urlResolver.Resolve(resolvedURL)
			}
			ServeRemoteResourceWithOptions(w, requestWithoutCoverRange(r), resolvedURL, RemoteResourceOptions{
				Timeout:      songCoverProxyTimeout,
				ErrorStatus:  http.StatusNotFound,
				ErrorMessage: "cover fetch failed",
			})
			return
		}
	}

	respondError(w, http.StatusNotFound, "封面不存在", nil)
}

// serveLocalCover 返回本地封面文件（支持 ?w= 服务端缩略，见 serveCoverFile）。
func (h *SongHandler) serveLocalCover(w http.ResponseWriter, r *http.Request, song *models.Song) {
	serveCoverFile(w, r, song.CoverPath, h.thumbCache)
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
		if errors.Is(err, database.ErrNotFound) {
			respondError(w, http.StatusNotFound, "歌曲不存在", nil)
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
// @Description 按 song.ID 流式播放。内部根据 song.type 分发到本地文件、网络歌曲或电台。HLS 电台返回播放列表或 302；HTTP-FLV 电台通过 play.flv 返回原始 FLV；配置为 audio_only 的电台返回首条音轨转码得到的 MP3。
// @Tags 歌曲管理
// @Produce application/octet-stream
// @Param id path int true "歌曲 ID"
// @Param format query string false "目标转码格式（如 mp3、ogg），用于平台兼容性转码"
// @Param quality query string false "目标音质码率（128/192/320），不传或不合法值表示原始音质。指定后默认转码为 mp3（除非同时指定了 format）"
// @Param track query int false "抽取指定音频流播放（audio-relative 0-based，对应 ffmpeg -map 0:a:N）。用于 Web 端双音轨（原唱/伴奏 mka）切轨：后端抽出单条音轨，AAC 编码时无损 remux 成 m4a、否则转 mp3。缺省/负数=不抽轨；与 media=video 互斥"
// @Param prefetch query string false "传 1 时异步预热缓存/转码，立即返回 202"
// @Param media query string false "传 video 时按视频播放：直出原容器（忽略 format/quality 转码，避免 -vn 丢画面），并按容器真实类型返回 Content-Type（如 video/mp4）。用于应用内视频画面渲染与 DLNA 视频投屏"
// @Param hls query string false "仅电台(HLS)有效。传 direct 时强制 302 直连源站、绕过本机 HLS 反代（即使 /settings/hls-proxy 已开）。原生 player 无 CORS 限制，直连可避免直播切片经反代往返后过期(404)；浏览器不传此参数以继续走反代解决 CORS"
// @Param normalize query string false "传 1 显式开启、0 显式关闭 EBU R128 音量均衡；不传时由服务端 /settings/volume-normalize 配置决定（默认关闭）。启用后使用 ffmpeg loudnorm=I=-16:LRA=11:TP=-1.5 消除不同音源之间的响度落差。需要重编码，未同时指定 format 时默认转为 mp3。均衡产物有独立缓存（文件名带 norm. 标记）；产物尚未生成时服务端边转边发一条 chunked MP3 流（无 Content-Length、不可 Range、Cache-Control 为 no-store），因此首字节不必等整首转完。media=video 忽略（-vn 会丢画面）；缺 ffmpeg 时优雅降级为原始音频"
// @Param radio_transcode query string false "仅电台有效。传目标格式（如 mp3）时，服务端用 ffmpeg 把电台流实时转码为该格式（HLS 与裸流均适用）。用于只支持 MP3、无法解码 AAC/HE-AAC 或不支持 HLS 的音箱。缺 ffmpeg 或坏源时优雅降级为原样代理/302。与 format 分离：电台侧忽略 format，只认此参数"
// @Param seek query number false "从第 N 秒起播。面向不支持 HTTP Range seek 的推流客户端（如小爱音箱经 player_play_url 只会从头拉流）：服务端用 ffmpeg input seek 产出一条以第 N 秒为开头的 chunked MP3 流，因此响应无 Content-Length、不可 Range、Cache-Control 为 no-store；浏览器等支持 Range 的客户端请用 Range 而非此参数。仅本地歌曲与已缓存的网络歌曲有效（电台是直播、未缓存的网络歌曲会阻塞整首下载，均忽略）；media=video 与 HEAD 忽略；缺 ffmpeg / seek 越过时长时优雅降级为从头完整播放"
// @Param speed query number false "播放倍速，取值夹到 [0.5, 2.0]（超出该区间需要链式拼接多个 atempo，暂不支持）。服务端用 ffmpeg atempo 滤镜实时变速不变调，产出 chunked MP3 流（同 seek，无 Content-Length、不可 Range）；可与 seek/normalize 组合，由同一条 ffmpeg 一起处理。仅本地歌曲与已缓存的网络歌曲有效；电台、media=video、HEAD 忽略；缺 ffmpeg 或值非法时优雅降级为原速播放"
// @Success 200 {file} binary "音频文件"
// @Success 202 {string} string "预拉取已触发"
// @Success 302 {string} string "电台流重定向"
// @Failure 404 {string} string "歌曲不存在"
// @Failure 403 {object} map[string]string "纯音频转码的源或其子资源不在允许范围内"
// @Failure 502 {string} string "音源不可用或电台纯音频转码失败"
// @Failure 503 {object} map[string]string "纯音频转码工具不可用或并发已满"
// @Failure 504 {object} map[string]string "纯音频转码启动超时"
// @Security BearerAuth
// @Router /songs/{id}/play [get]
// @Router /songs/{id}/play.m3u8 [get]
// @Router /songs/{id}/play.flv [get]
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
	if strings.HasSuffix(r.URL.Path, "/play.flv") && !song.IsFLVStream() {
		http.NotFound(w, r)
		return
	}

	targetFormat := r.URL.Query().Get("format")
	bitrate := services.ParseBitrate(r.URL.Query().Get("quality"))
	if bitrate > 0 && targetFormat == "" {
		targetFormat = "mp3"
	}

	// media=video：按视频画面播放。强制直出原容器——忽略 format/quality 转码，
	// 因为转码走 ffmpeg -vn 会把视频轨丢掉；同时让 serveLocal 按容器真实类型给 Content-Type。
	videoIntent := r.URL.Query().Get("media") == "video"
	if videoIntent {
		targetFormat = ""
		bitrate = 0
	}

	// track=N：抽取指定音频流播放（audio-relative 0-based，songloft-org/songloft#298）。
	// Web 端双音轨（原唱/伴奏 mka）切轨用：后端 -map 出单条音轨并（AAC 时）无损 remux 成 m4a。
	// 与 media=video 互斥（抽单条音轨会丢画面）；缺省/负数=不抽轨。
	trackIndex := -1
	if !videoIntent {
		if ts := r.URL.Query().Get("track"); ts != "" {
			if n, err := strconv.Atoi(ts); err == nil && n >= 0 {
				trackIndex = n
			}
		}
	}

	// normalize：启用 EBU R128 音量均衡（songloft-org/songloft#315, songloft-org/songloft-player-flutter#33）。
	// 需要转码，若未指定 format 则默认使用 mp3。
	//
	// 优先级：?normalize=1 显式开启；?normalize=0 显式关闭；
	// 无参数时由服务端 volume_normalize 配置决定（默认 false）。
	//
	// videoIntent 下必须强制关掉：上面刚为了保护画面清空了 targetFormat，若这里又把它填成 mp3，
	// 转码的 `-vn` 会把视频轨切掉，`media=video` 就只剩纯音频——正是那段代码要避免的结果。
	// 均衡本身也没有「保留画面」的实现路径（`-vn` 是 loudnorm 这条链的固定前提）。
	normalizeParam := r.URL.Query().Get("normalize")
	var normalize bool
	if normalizeParam != "" {
		normalize = normalizeParam == "1"
	} else if h.configService != nil {
		normalize = h.configService.GetBool(volumeNormalizeConfigKey, false)
	}
	normalize = normalize && !videoIntent
	if normalize && targetFormat == "" {
		targetFormat = "mp3"
	}

	// seek=N：从第 N 秒起播，服务端产出以该位置为开头的 MP3 流（songloft-plugin-miot#60）。
	// videoIntent 下忽略（seek 流一律 -vn 出 MP3，会丢画面）；HEAD 忽略（探测请求不该起 ffmpeg）。
	var seekSeconds float64
	if !videoIntent && r.Method != http.MethodHead {
		seekSeconds = parseSeekSeconds(r.URL.Query().Get("seek"), song.Duration)
	}

	// speed=N：播放倍速（0.5–2.0），服务端用 ffmpeg atempo 滤镜实时变速（不变调）。
	// 与 seek 同理：videoIntent 下忽略（-vn 会丢画面，且视频变速需要 setpts，不是本参数的职责范围）；
	// HEAD 忽略（探测请求不该起 ffmpeg）；电台是直播流，serveRadio 不读取 opts，天然忽略。
	var speed float64 = 1.0
	if !videoIntent && r.Method != http.MethodHead {
		speed = parseSpeed(r.URL.Query().Get("speed"))
	}

	// 预拉取模式：异步触发缓存 + 转码预热，立即返回 202。
	// 不能用 r.Context()，否则 202 发出后客户端断开会 Kill ffmpeg，预热失败。
	// 通过 playActivity.Track 注册进 registry（CatPrefetch），但 Activate 不会取消 prefetch
	// （songloft-org/songloft#300）：prefetch 天然为「下一首」预热，切到当前歌时不能连带杀掉
	// 下一首的预热转码。转码跑完后 prepareSongPlayback 返回、defer release() 注销 entry。
	if r.URL.Query().Get("prefetch") == "1" {
		go func() {
			pctx, release := h.trackActivity(context.Background(), sk, song.ID, playactivity.CatPrefetch)
			defer release()
			h.prepareSongPlayback(pctx, song, targetFormat, bitrate, normalize)
		}()
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// 把正式播放请求纳入 playActivity，让同会话切到其他歌曲时可以停止旧流，避免多个连接
	// 同时读取外接磁盘。保留同曲请求，兼容同一客户端向多台音箱的同步播放。
	var releasePlay func()
	if h.playActivity != nil && r.Method != http.MethodHead {
		playCtx, release := h.trackActivity(r.Context(), sk, song.ID, playactivity.CatPlay)
		releasePlay = release
		r = r.WithContext(playCtx)
	}
	if releasePlay != nil {
		defer releasePlay()
	}

	opts := servePlayOptions{
		targetFormat: targetFormat,
		bitrate:      bitrate,
		videoIntent:  videoIntent,
		trackIndex:   trackIndex,
		normalize:    normalize,
		seekSeconds:  seekSeconds,
		speed:        speed,
	}

	switch song.Type {
	case models.TypeLocal:
		h.serveLocal(w, r, song, opts)
	case models.TypeRadio:
		// 电台是直播流，没有「曲内位置」可言，seek 天然不适用（opts 里的值被忽略）。
		h.serveRadio(w, r, song)
	case models.TypeRemote:
		h.serveRemote(w, r, song, opts)
	default:
		respondError(w, http.StatusInternalServerError, "unsupported song type", nil)
	}
}

// servePlayOptions 是 serveLocal / serveRemote / serveCachedFile 共用的播放参数。
// 收成结构体而非位置参数：这些字段里相邻的 int/bool/float64 用位置传参极易调错且编译器不报。
type servePlayOptions struct {
	targetFormat string  // 目标转码格式（format 参数，已含 quality/normalize 推导出的默认值）
	bitrate      int     // 目标码率 kbps，0=原始音质
	videoIntent  bool    // media=video：直出原容器保留画面
	trackIndex   int     // 抽轨（audio-relative 0-based），< 0 = 不抽轨
	normalize    bool    // EBU R128 音量均衡
	seekSeconds  float64 // 从第 N 秒起播，0 = 从头
	speed        float64 // 播放倍速 [0.5, 2.0]，1.0 = 不变速
}

// parseSeekSeconds 解析 seek 查询参数。
//
// duration > 0 时把接近/超过时长的值夹成 0：seek 越过文件尾会让 ffmpeg 零输出，
// 进而触发「无损降级」把整首歌从头重播一遍——比忽略 seek 更糟。duration == 0（时长未知）
// 时不夹紧，交由 ffmpeg 零输出后的降级兜底。
func parseSeekSeconds(raw string, duration float64) float64 {
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0
	}
	if duration > 0 && v >= duration-seekTailGuardSeconds {
		return 0
	}
	return v
}

// seekTailGuardSeconds seek 距歌曲结尾的最小保留秒数，见 parseSeekSeconds。
const seekTailGuardSeconds = 3

// speedMin / speedMax 播放倍速允许范围，对齐 ffmpeg atempo 单滤镜原生支持区间 [0.5, 2.0]。
// 超出该区间需要链式拼接多个 atempo 才能实现，目前不支持，越界值直接夹紧。
const speedMin = 0.5
const speedMax = 2.0

// parseSpeed 解析 speed 查询参数。非法/缺省值回退到 1.0（不变速）；合法值夹到 [speedMin, speedMax]。
func parseSpeed(raw string) float64 {
	if raw == "" {
		return 1.0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return 1.0
	}
	if v < speedMin {
		return speedMin
	}
	if v > speedMax {
		return speedMax
	}
	return v
}

// trySeekStream 尝试以「从 opts.seekSeconds 起、按 opts.speed 变速的 MP3 流」提供 path，
// 成功接管响应返回 true。
//
// 返回 false 表示尚未向响应写入任何字节（含未请求 seek/变速、ffmpeg 缺失/并发满/零输出等），
// 调用方应继续走原本的 http.ServeFile 从头、原速提供文件。
func (h *SongHandler) trySeekStream(w http.ResponseWriter, r *http.Request, song *models.Song, path string, opts servePlayOptions) bool {
	if (opts.seekSeconds <= 0 && !speedRequested(opts.speed)) || h.cacheService == nil {
		return false
	}

	plan := planCBRRange(r, song, opts)
	if plan != nil && plan.unsatisfiable {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", plan.totalBytes))
		respondError(w, http.StatusRequestedRangeNotSatisfiable, "range not satisfiable", nil)
		return true
	}

	startSecond := opts.seekSeconds
	if plan != nil && plan.partial {
		startSecond += plan.startSecond
	}

	return h.streamPipedMP3(r.Context(), r.Context(), w, song, services.SeekStreamOptions{
		SourcePath:      path,
		StartSecond:     startSecond,
		RemainingSecond: remainingAfter(song, startSecond),
		Speed:           opts.speed,
	}, plan, "seek stream")
}

// tryLiveTranscodeStream 在「这次播放需要转码但转码产物还没落盘」时边转边发一条 MP3 流，
// 成功接管响应返回 true。必须调用在原有的阻塞转码分支 **之前**。
//
// 存在的理由：GetOrTranscode 是同步的，首字节 = 整首转码的墙钟时间。两处实测：
//   - 音量均衡：整首 loudnorm 要 20 多秒（dur_ms=22392 / 24348 / 22381），音箱那端表现为
//     「前 20 多秒空白」，而插件的自动切歌定时器在推 URL 那一刻就起算，尾部还会被砍掉同样长度
//     （songloft-org/songloft-plugin-miot#61）。
//   - 普通 ?format= / ?quality= 转码：弱 CPU（N5095）无损转 mp3 要等 5 秒以上才出声
//     （songloft-org/songloft#442）。
//
// 两者是同一个根因，也用同一条 pipe 解决：边转边发让首字节 1 秒内出来。
//
// 转码产物已存在时返回 false 走 http.ServeFile——那条路支持 Range 且更快；产物由
// `?prefetch=1`（带同样的 format/quality/normalize）提前热好，客户端在播上一首时就会预热下一首。
//
// 刻意不顺手起一个后台转码去补缓存文件：同一首歌同时跑两个 ffmpeg 会在弱 NAS 上把 CPU 翻倍，
// 而用户正等着这一秒出声。缓存产物交给预热生成。
//
// 这条流**支持 Range**（`cbrRangePlan`）：拖动进度条会被真正满足，而不是像最初那样静默卡死。
// 剩余差异只有「Content-Length 是按 CBR 估算的、`Cache-Control: no-store`」，见 exactLengthWriter。
func (h *SongHandler) tryLiveTranscodeStream(w http.ResponseWriter, r *http.Request, song *models.Song, path string, opts servePlayOptions) bool {
	if h.cacheService == nil {
		return false
	}
	// 下面这些一律留给原来的阻塞转码路径，语义不变：
	//   videoIntent —— 本函数一律 -vn，会丢画面；
	//   trackIndex  —— 抽轨的目标容器由后端探测决定，不一定是 mp3；
	//   CUE 轨      —— 必须先按 CueStart/End 提取成独立文件，否则 -ss 会叠到整轨镜像的绝对位置；
	//   HEAD        —— 探测请求不该起 ffmpeg；
	//   非 mp3 目标 —— 本函数固定输出 MP3，冒充不了 m4a/ogg/flac/wav。
	//                  （码率不再是排除项：SeekStreamOptions.Bitrate 已能如实表达 ?quality=。）
	if opts.videoIntent || opts.trackIndex >= 0 || song.CueSourcePath != "" || r.Method == http.MethodHead {
		return false
	}
	if services.NormalizeFormat(opts.targetFormat) != "mp3" {
		return false
	}
	// 自己复核「这次是否真的需要转码」，不依赖调用点的 if 条件——两个调用点（serveLocal /
	// serveCachedFile）各自算过一遍，但把判断收在函数内才能保证将来新增调用点不会误起 ffmpeg。
	// 三个理由与调用点、与 GetOrTranscode 的 needsTranscode 同源：格式不符（含伪装扩展名）、
	// 指定码率、音量均衡。
	needsTranscode := opts.normalize || opts.bitrate > 0 ||
		services.NeedsTranscodeForServe(song, path, opts.targetFormat)
	if !needsTranscode {
		return false
	}
	// 转码产物已就绪 → 交给 ServeFile。缓存键含 bitrate 与 normalize 维度，
	// 各档 quality、均衡/非均衡的产物互不通用，这里的查找天然按当次请求的组合命中。
	if _, ok := h.cacheService.FindTranscodedFile(song, "mp3", opts.bitrate, -1, opts.normalize); ok {
		return false
	}
	// 注册进 playActivity（与被它替代的阻塞转码分支同一 Category），让 ActivateSong 能在用户切歌时
	// 掐掉这条流的 ffmpeg，不再白烧 CPU 去编码一首已经不放了的歌。
	//
	// 注意它**不能**提前归还 seekStreamSem 槽位：槽位由 StreamSeekedMP3 的 io.Copy 持有，
	// 而 io.Copy 只在「读端 EOF」或「写响应失败」时返回，不看 ctx。客户端断开属于后者（自动归还），
	// 但「客户端还在慢慢读 + 用户已切歌」这种组合下槽位仍要等它读完。实测音箱是贪婪缓冲、
	// 4 分钟的歌 10 秒就流完，占不满 4 个槽；真占满也只是降级回阻塞转码，不会更坏。
	sk := playactivity.SessionFromContext(r.Context())
	trackedCtx, release := h.trackActivity(r.Context(), sk, song.ID, playactivity.CatTranscode)
	defer release()

	// Range 应答计划：让客户端能真正拖动这条实时流。不满足前提时为 nil，退回 chunked 无长度。
	plan := planCBRRange(r, song, opts)
	if plan != nil && plan.unsatisfiable {
		// start 越过了资源末尾。此时一个字节都还没写，直接以 416 接管响应。
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", plan.totalBytes))
		respondError(w, http.StatusRequestedRangeNotSatisfiable, "range not satisfiable", nil)
		return true
	}

	// 起播位置：?seek= 的歌曲绝对位置 + Range 的流内字节偏移换算。两者正交叠加。
	startSecond := opts.seekSeconds
	if plan != nil && plan.partial {
		startSecond += plan.startSecond
	}

	return h.streamPipedMP3(trackedCtx, r.Context(), w, song, services.SeekStreamOptions{
		SourcePath:      path,
		StartSecond:     startSecond, // 可为 0（纯转码/纯均衡）；> 0 时与转码由同一条 ffmpeg 一起做
		RemainingSecond: remainingAfter(song, startSecond),
		Normalize:       opts.normalize,
		Bitrate:         opts.bitrate, // 0 = 沿用 320k CBR；?quality= 时按请求码率出 CBR
		// 必须显式置 true：普通 ?format=mp3（不 seek、不均衡、不变速、未指定码率）下
		// StreamSeekedMP3 的守卫会认为「无事可做」而拒绝，而伪 mp3（扩展名 .mp3 内容是 WebM，
		// songloft-org/songloft#300）还会被 copy 快路径吃掉。见 SeekStreamOptions.ForceTranscode。
		ForceTranscode: true,
		Speed:          opts.speed, // 转码/均衡与变速可与同一条 ffmpeg 一起做，见 StreamSeekedMP3 的滤镜拼接
	}, plan, "live transcode stream")
}

// cbrRangePlan 是一次 CBR pipe 流的 Range 应答计划（songloft-org/songloft#442）。
//
// **为什么 pipe 流必须支持 Range**：pipe 响应原本是 chunked、无 `Content-Length`，浏览器据此
// 判定该资源不可 seek——实测 Chrome 拖动进度条后**不发任何新请求**，`readyState` 从 4 掉到 1、
// buffered 零增长、`error` 为 null，播放**静默卡死**，只能重新点播。而进度条的总时长来自 DB
// （`song.duration`），UI 上完全看不出这条流不能拖，用户必然会去拖。
//
// 只补 `Accept-Ranges: bytes` 不够（实测同样只发一个请求就卡死）：浏览器需要**总字节数**
// 才能把「第 N 秒」映射成「第几个字节」，从而构造 Range 请求。
//
// **为什么能算得出来**：pipe 一律输出 CBR（`-b:a`），所以字节与时间是线性的，
// 再加上已知的 `song.duration` 就能同时给出总长度与任意字节偏移对应的秒数；而「从第 N 秒起流」
// 正是 `StreamSeekedMP3` 已有的能力（`-ss` input seek）。于是 Range 请求可以被**真正**满足，
// 而不是靠「忽略 Range 从头重发」糊过去（那会让进度条与实际声音错位）。
type cbrRangePlan struct {
	totalBytes    int64   // 按 duration 估算的总字节数
	start, end    int64   // 本次要送出的闭区间字节范围
	partial       bool    // true → 206 + Content-Range；false → 200 全量
	unsatisfiable bool    // true → 416，start 越过了总长度
	startSecond   float64 // start 换算出的时间偏移，喂给 ffmpeg -ss
}

// contentLength 返回本次应答承诺的字节数。承诺了就必须精确写出这么多，见 exactLengthWriter。
func (p *cbrRangePlan) contentLength() int64 { return p.end - p.start + 1 }

// planCBRRange 计算 pipe 流的 Range 应答计划；返回 nil 表示**不启用** Range 模式，
// 保持原来的 chunked 无 Content-Length 行为。
//
// 三个前提缺一不可：
//   - `song.Duration > 0`：总时长未知就估不出总字节（远程歌曲元数据未刷新时是常态）；
//   - 不变速：atempo 改变输出时长，字节↔时间要再套一层换算，风险不值当，保持现状；
//   - seek 后剩余时长 > 0：seekSeconds=0 时退化为全量。
//
// `?seek=` 与 Range 不再互斥：`?seek=N` 让 ffmpeg 从第 N 秒起产流，Range 在此基础上做
// HTTP 层缓冲——两者正交。MIoT 音箱等推流客户端**会**发 Range（实测 206），
// 缺少 Content-Length 时固件直接报播放失败（songloft-org/songloft#488）。
//
// seek 流走 `-c:a copy` 时输出可能是 VBR MP3，CBR 估算会偏差几个百分点，
// 由 exactLengthWriter 兜底（多截少补零），与转码流已承认的残留同性质。
func planCBRRange(r *http.Request, song *models.Song, opts servePlayOptions) *cbrRangePlan {
	if song == nil || song.Duration <= 0 || song.Duration > maxRangeDurationSeconds ||
		speedRequested(opts.speed) {
		return nil
	}
	effectiveDuration := song.Duration - opts.seekSeconds
	if effectiveDuration <= 0 {
		return nil
	}
	bitrate := opts.bitrate
	if bitrate <= 0 {
		bitrate = services.DefaultPipeBitrateKbps
	}
	bps := int64(bitrate) * 1000 / 8
	total := int64(effectiveDuration * float64(bps))
	if total <= 0 {
		return nil
	}
	p := &cbrRangePlan{totalBytes: total, start: 0, end: total - 1}

	start, end, ok, unsatisfiable := parseFirstByteRange(r.Header.Get("Range"), total)
	switch {
	case unsatisfiable:
		p.unsatisfiable = true
	case ok:
		p.start, p.end = start, end
		// `bytes=0-` 覆盖整个资源（Chrome 的首个请求就长这样）：按 200 全量应答。
		// RFC 7233 允许服务器忽略 Range，而 200 + Content-Length 已经足够让客户端后续算出偏移。
		p.partial = !(start == 0 && end == total-1)
		p.startSecond = float64(start) / float64(bps)
	}
	return p
}

// maxRangeDurationSeconds 是启用 Range 模式的时长上限（24 小时，足够覆盖有声书）。
//
// `song.duration` 是 DB 里的 float64，损坏的元数据会让「时长 × 码率」溢出 int64，
// 而 float64→int64 的溢出结果在 Go 里是实现定义的：万一落成一个巨大正数，就会给客户端
// 承诺一个天文数字的 Content-Length，把它挂在等永远不会来的字节上。超限时退回
// chunked 无长度（只是不能拖动，不会挂住）。
const maxRangeDurationSeconds = 24 * 3600

// parseFirstByteRange 解析 `Range: bytes=...` 的**第一个**区间，返回闭区间 [start, end]。
//
// ok=false 表示「按 200 全量应答」：无 Range 头、语法不认识、或多区间请求（`bytes=0-99,200-299`）
// ——多区间要 multipart/byteranges 应答，对音频播放没有实际需求，RFC 7233 也允许服务器
// 忽略 Range 返回 200，故刻意不支持。
// unsatisfiable=true 表示 start 越过了资源末尾，调用方应回 416。
func parseFirstByteRange(header string, total int64) (start, end int64, ok, unsatisfiable bool) {
	const prefix = "bytes="
	// total <= 0 时任何区间都无从计算（调用方已挡住，这里兜住将来的新调用点）。
	// 前缀比对大小写敏感，与 stdlib http.ServeContent 的口径一致。
	if total <= 0 || !strings.HasPrefix(header, prefix) {
		return 0, 0, false, false
	}
	spec := strings.TrimPrefix(header, prefix)
	if strings.Contains(spec, ",") {
		return 0, 0, false, false
	}
	spec = strings.TrimSpace(spec)
	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, false, false
	}
	startStr, endStr := strings.TrimSpace(spec[:dash]), strings.TrimSpace(spec[dash+1:])

	// `bytes=-N`：末尾 N 字节。N=0 无意义（RFC 明确不满足）。
	if startStr == "" {
		n, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false, false
		}
		if n > total {
			n = total
		}
		return total - n, total - 1, true, false
	}

	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 {
		return 0, 0, false, false
	}
	if start >= total {
		return 0, 0, false, true
	}
	end = total - 1
	if endStr != "" {
		e, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || e < start {
			return 0, 0, false, false
		}
		if e < end {
			end = e
		}
	}
	return start, end, true, false
}

// exactLengthWriter 把写出的字节数**精确**约束成 want 字节：超出的丢弃，
// 不足的在 fill() 里补零。
//
// 承诺了 `Content-Length` 就必须写够，否则客户端把响应视为连接提前断开（报错或无限重试）；
// 写多了则违反 HTTP/1.1 的长度契约。而 ffmpeg 的实际输出量不会精确等于「码率 × 时长」：
// LAME 的 CBR 靠帧 padding 贴合目标码率（每帧在 N/N+1 字节间浮动）、末尾可能是不完整帧、
// `-ss` 又只能对齐到帧边界，累计误差通常在几帧（几 KB）以内。
//
// 补零而不是提前收尾：mp3 解码器会把全零字节当作无效帧跳过，末尾几 KB 静音远好于
// 「客户端认为下载失败」。截断同理——宁可少送最后不到一帧的音频。
type exactLengthWriter struct {
	w       io.Writer
	want    int64
	written int64
}

// errExactLengthReached 表示已按承诺的 Content-Length 写满，ffmpeg 可以停了。
//
// 不把多余输出静默丢弃、而是主动报错中断 io.Copy：闭区间 Range（`bytes=N-M`）只要一小段，
// 而 ffmpeg 会一路转到源结尾，静默丢弃等于让一个 `bytes=0-100` 请求白转整首——弱 CPU 上
// 这既是浪费也是一个放大面。返回 error 让 services 侧 cancel 掉 runCtx、及时回收 ffmpeg。
//
// 浏览器与 just_audio 的媒体请求都是开放式的 `bytes=N-`（want 正好到结尾），所以这条路
// 平时不会触发；它守的是闭区间与异常客户端。
var errExactLengthReached = errors.New("exact length reached")

func (ew *exactLengthWriter) Write(p []byte) (int, error) {
	remaining := ew.want - ew.written
	if remaining <= 0 {
		return 0, errExactLengthReached
	}
	truncated := int64(len(p)) > remaining
	if truncated {
		p = p[:remaining]
	}
	n, err := ew.w.Write(p)
	ew.written += int64(n)
	if err != nil {
		return n, err
	}
	if truncated {
		// 本次写入成功但已触到上限。必须连同 n 一起返回 error：只返回 (n, nil) 且 n < len(p)
		// 会让 io.Copy 报 ErrShortWrite，那是个会被记成「中途失败」的假错误。
		return n, errExactLengthReached
	}
	return n, nil
}

// Flush 必须转发：services 侧是 `io.Copy(&flushingWriter{w: w}, ...)`，而 flushingWriter 靠
// **类型断言** `w.(http.Flusher)` 决定要不要 flush。少了这个方法，包装层会把断言挡掉，
// 字节攒在缓冲里不下发——首字节延迟原地回归，本次优化白做。
func (ew *exactLengthWriter) Flush() {
	if f, ok := ew.w.(http.Flusher); ok {
		f.Flush()
	}
}

// fill 在 ffmpeg 输出不足承诺长度时补零补满。只在流正常结束后调用。
func (ew *exactLengthWriter) fill() {
	if ew.written >= ew.want {
		return // 常见情况：ffmpeg 输出刚好够或已被截断，不必分配补零缓冲
	}
	const chunk = 32 * 1024
	zero := make([]byte, chunk)
	for ew.written < ew.want {
		n := ew.want - ew.written
		if n > chunk {
			n = chunk
		}
		written, err := ew.w.Write(zero[:n])
		ew.written += int64(written)
		if err != nil {
			return
		}
	}
}

// deferredStatusWriter 把 WriteHeader 推迟到**第一次真的有字节要写**的时刻。
//
// 206 必须显式 WriteHeader，而一旦调用响应就提交了、再也无法降级。StreamSeekedMP3 的契约是
// 「Peek(1) 确认 ffmpeg 真有输出后才开始写」，所以「首次 Write」正是那个安全时刻：
// 在此之前失败仍可 Del 掉响应头、无损降级为阻塞转码路径。
type deferredStatusWriter struct {
	w      http.ResponseWriter
	status int
	sent   bool
}

func (dw *deferredStatusWriter) Write(p []byte) (int, error) {
	dw.commit()
	return dw.w.Write(p)
}

// Flush 同 exactLengthWriter.Flush：类型断言必须能穿透包装层。
// 先 commit 再 flush——ResponseWriter.Flush 本身会隐式提交 200，抢在它之前写下真正的状态码。
func (dw *deferredStatusWriter) Flush() {
	dw.commit()
	if f, ok := dw.w.(http.Flusher); ok {
		f.Flush()
	}
}

func (dw *deferredStatusWriter) commit() {
	if !dw.sent {
		dw.sent = true
		dw.w.WriteHeader(dw.status)
	}
}

// speedRequested 判断倍速是否需要实际生效（区分「未指定」与「显式传 1.0」，两者效果一致）。
// 与 services.speedActive 同源；handlers 侧原先有两处内联的同款判断。
func speedRequested(speed float64) bool {
	return speed != 0 && speed != 1.0
}

// streamPipedMP3 调用 StreamSeekedMP3 并处理「先设响应头、降级时原样还原」的契约。
// 返回 true 表示已接管响应（正常结束，或已写出部分字节后失败、无法再降级）。
//
// ctx 驱动 ffmpeg 生命周期，可能被 playActivity.Activate 提前取消；connCtx 是真正的
// HTTP 请求/连接 ctx（seek 流直接用 r.Context() 传两次，均衡流的 ctx 是 trackedCtx、
// connCtx 是 r.Context()），只在客户端真的断开时才会 Done——两者的区分详见
// services.StreamSeekedMP3 注释与 songloft-org/songloft-player-flutter#35。
//
// plan 非 nil 时按 Range 语义应答（`Content-Length` + `Accept-Ranges`，206 时另加
// `Content-Range`），让客户端能真正拖动进度条，见 cbrRangePlan。为 nil 时保持
// chunked 无长度的原行为（seek 流、变速流、时长未知的歌走这条）。
func (h *SongHandler) streamPipedMP3(ctx, connCtx context.Context, w http.ResponseWriter, song *models.Song, sopts services.SeekStreamOptions, plan *cbrRangePlan, reason string) bool {
	// 先设响应头：StreamSeekedMP3 一旦写出字节就无法再改。降级时下面会原样删掉。
	// 降级时 Del 是**必需**的而非可选：http.ServeFile 只在 Content-Type 缺失时才按扩展名推断，
	// 残留的 audio/mpeg 会让降级后提供的 mp4/flac 拿到错误的 Content-Type。
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-store")

	// dst 是真正交给 StreamSeekedMP3 的写入端。Range 模式下要在响应与 ffmpeg 之间插两层：
	// 精确长度约束（承诺了 Content-Length 就必须写够写准）与延迟状态码（206 一旦发出就无法降级）。
	var dst io.Writer = w
	var exact *exactLengthWriter
	if plan != nil {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.FormatInt(plan.contentLength(), 10))
		status := http.StatusOK
		if plan.partial {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", plan.start, plan.end, plan.totalBytes))
			status = http.StatusPartialContent
		}
		exact = &exactLengthWriter{w: &deferredStatusWriter{w: w, status: status}, want: plan.contentLength()}
		dst = exact
	}

	err := h.cacheService.StreamSeekedMP3(ctx, connCtx, dst, sopts)
	// 写满承诺长度而主动中断 ffmpeg 的情况：响应已完整送出，是正常结束而非失败。
	if errors.Is(err, errExactLengthReached) {
		return true
	}
	if err == nil {
		// 正常结束：ffmpeg 的实际输出量与「码率 × 时长」有几帧误差，补零补满承诺的长度，
		// 否则客户端会把响应当成连接提前断开。
		if exact != nil {
			exact.fill()
		}
		return true
	}
	if errors.Is(err, services.ErrSeekStreamAborted) {
		// ffmpeg 被 playActivity.Activate 提前掐掉，但连接仍存活：绝不能让 chunked 响应
		// 优雅收尾（客户端会把截断的音频误判为下载成功并永久缓存）。强制切断底层连接，
		// 让客户端的 HTTP 层感知为读取失败（songloft-org/songloft-player-flutter#35）。
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, herr := hj.Hijack(); herr == nil {
				_ = conn.Close()
			}
		}
		return true
	}
	if !errors.Is(err, services.ErrSeekStreamUnavailable) {
		// 已写出部分字节后中途失败：响应已提交，无法再降级。
		return true
	}
	// 一个字节都没写出：清掉预设的响应头，让调用方降级。
	// Range 模式那几个头同样必须清掉——残留的 Content-Length/Content-Range 会和降级后
	// http.ServeFile 自己算出的长度冲突，客户端拿到的是长度对不上的响应。
	w.Header().Del("Content-Type")
	w.Header().Del("Cache-Control")
	w.Header().Del("Accept-Ranges")
	w.Header().Del("Content-Length")
	w.Header().Del("Content-Range")
	slog.Warn(reason+" unavailable, falling back",
		"songId", song.ID, "seek", sopts.StartSecond, "normalize", sopts.Normalize,
		"path", sopts.SourcePath, "error", err)
	return false
}

// remainingAfter 返回从 startSecond 起的预计剩余音频时长；时长未知或已越过结尾时返回 0
// （交由 StreamSeekedMP3 用固定兜底超时）。
func remainingAfter(song *models.Song, startSecond float64) float64 {
	if song.Duration > startSecond {
		return song.Duration - startSecond
	}
	return 0
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
//
// normalize 必须与真实播放请求一致：均衡产物的缓存键带 "norm." 标记
// （见 transcodedFileName），预热成非均衡产物等于白热，真实播放仍会冷启动整首 loudnorm
// 并把设备的首个 play 请求阻塞 20+ 秒（songloft-org/songloft-plugin-miot#61）。
func (h *SongHandler) prepareSongPlayback(ctx context.Context, song *models.Song, targetFormat string, bitrate int, normalize bool) {
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
		// 按配置的缓存转码格式统一基础缓存格式（如 mp3），使真实播放直接命中目标格式；
		// 未配置 / 失败时原样返回。随后的 NeedsTranscode 判断会因基础缓存已是目标格式而短路。
		srcPath = h.cacheService.EnsureCachedFormat(ctx, song, path)
	default:
		return
	}

	// CUE track: 修正 format（APE → FLAC），始终走 GetOrTranscode 预热提取缓存
	if song.CueSourcePath != "" {
		if targetFormat == "" || services.NormalizeFormat(targetFormat) == "ape" {
			f := services.NormalizeFormat(filepath.Ext(song.FilePath))
			if f == "ape" {
				f = "flac"
			}
			targetFormat = f
		}
	}

	// normalize 单独作为「必须转码」的理由：mp3 源 + format=mp3 时 NeedsTranscodeForServe 为 false，
	// 少了这一项就会在最需要预热的均衡场景直接 return，一行没干。
	if song.CueSourcePath == "" && !normalize && !services.NeedsTranscodeForServe(song, srcPath, targetFormat) && bitrate == 0 {
		return
	}
	if _, err := h.cacheService.GetOrTranscode(ctx, srcPath, song, services.NormalizeFormat(targetFormat), bitrate, -1, normalize); err != nil {
		slog.Warn("prefetch transcode failed", "songId", song.ID, "format", targetFormat, "bitrate", bitrate, "normalize", normalize, "error", err)
	} else {
		slog.Info("prefetch ready", "songId", song.ID, "format", targetFormat, "bitrate", bitrate, "normalize", normalize)
	}
}

// serveLocal 本地歌曲:直接 ServeFile(支持 Range,客户端 seek 可用)。
// targetFormat 非空且与原格式不同时，或 bitrate > 0 时，或 trackIndex >= 0（抽轨）时，走 ffmpeg 转码后返回。
// videoIntent=true（media=video）时上游已清空 targetFormat/bitrate/trackIndex，此处按容器真实类型给 video mime。
// trackIndex >= 0（songloft-org/songloft#298）时抽取指定音轨：由后端探测该轨编码决定目标容器
// （AAC → m4a 无损 remux，否则 → mp3），忽略传入的 targetFormat/bitrate。
func (h *SongHandler) serveLocal(w http.ResponseWriter, r *http.Request, song *models.Song, opts servePlayOptions) {
	if song.FilePath == "" {
		http.NotFound(w, r)
		return
	}
	targetFormat, bitrate, trackIndex, normalize := opts.targetFormat, opts.bitrate, opts.trackIndex, opts.normalize
	srcPath := song.FilePath
	if song.CueSourcePath != "" {
		// CUE track: FilePath 指向共享的整轨音频，必须经 ffmpeg 按需提取对应片段。
		// APE 不支持 stream copy，自动转为 FLAC。
		if targetFormat == "" || services.NormalizeFormat(targetFormat) == "ape" {
			f := services.NormalizeFormat(filepath.Ext(song.FilePath))
			if f == "ape" {
				f = "flac"
			}
			targetFormat = f
		}
		tcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		sk := playactivity.SessionFromContext(r.Context())
		trackedCtx, release := h.trackActivity(tcCtx, sk, song.ID, playactivity.CatTranscode)
		defer release()
		path, err := h.cacheService.GetOrTranscode(trackedCtx, srcPath, song, services.NormalizeFormat(targetFormat), bitrate, -1, normalize)
		if err != nil {
			slog.Warn("CUE track extraction failed", "songId", song.ID, "error", err)
			respondError(w, http.StatusInternalServerError, "CUE track extraction failed", nil)
			return
		}
		srcPath = path
	} else if trackIndex >= 0 {
		// 抽轨播放：目标容器由后端按该轨编码决定，覆盖入参的 format/quality。
		tcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if f := h.cacheService.PlanTrackExtraction(tcCtx, srcPath, trackIndex); f != "" {
			targetFormat = f
		}
		bitrate = 0
		sk := playactivity.SessionFromContext(r.Context())
		trackedCtx, release := h.trackActivity(tcCtx, sk, song.ID, playactivity.CatTranscode)
		defer release()
		path, err := h.cacheService.GetOrTranscode(trackedCtx, srcPath, song, services.NormalizeFormat(targetFormat), bitrate, trackIndex, normalize)
		if err != nil {
			slog.Warn("track extraction failed, serving original", "songId", song.ID, "trackIndex", trackIndex, "error", err)
		} else {
			srcPath = path
		}
	} else if services.NeedsTranscodeForServe(song, srcPath, targetFormat) || bitrate > 0 || normalize {
		// 转码产物还没落盘时先试边转边发，避免整首转码把这个请求挂住数秒到 20+ 秒
		// （无损转 mp3：songloft-org/songloft#442；整首 loudnorm：songloft-plugin-miot#61）。
		// 它内部已把 seek 一起做掉，接管成功就不再走下面的 trySeekStream。
		if h.tryLiveTranscodeStream(w, r, song, srcPath, opts) {
			return
		}
		tcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		sk := playactivity.SessionFromContext(r.Context())
		trackedCtx, release := h.trackActivity(tcCtx, sk, song.ID, playactivity.CatTranscode)
		defer release()
		path, err := h.cacheService.GetOrTranscode(trackedCtx, srcPath, song, services.NormalizeFormat(targetFormat), bitrate, -1, normalize)
		if err != nil {
			slog.Warn("transcode failed, serving original", "songId", song.ID, "format", targetFormat, "bitrate", bitrate, "error", err)
		} else {
			srcPath = path
		}
	}
	// seek 放在最后一步、作用在已定型的 srcPath 上（CUE 提取 / 抽轨 / 转码 / normalize 之后），
	// 这样 seek 语义恒为「曲内偏移」，与上面各分支自动叠加：CUE 轨先被提取成独立文件再 seek，
	// 不会与 -ss CueStartSeconds 叠成整轨镜像的绝对位置；转码产物已是 mp3 则走无损 copy。
	if h.trySeekStream(w, r, song, srcPath, opts) {
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000")
	if opts.videoIntent {
		// 视频画面播放:按容器真实类型给 Content-Type(如 video/mp4),供 Web <video> 与 DLNA 正确识别;
		// videoIntent 下上游已禁用转码,srcPath 一定是原容器。未知扩展名交由 http.ServeFile 决定。
		if ct := videoContentType(srcPath); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
	} else {
		// ISO-BMFF 音频容器（mp4/mov/m4a/m4b）显式声明为音频类型:
		// stdlib http.ServeFile 会按扩展名把 .mp4 标成 video/mp4、.mov 标成 video/quicktime,
		// 音频播放路径只取其中的音频轨,显式设 audio/mp4 可提升 Web <audio> 及部分客户端按音频处理的稳健性。
		// 基于最终 srcPath 判断:若已转码为 .mp3 等,则不覆盖,交由 http.ServeFile 给出正确类型。
		switch strings.ToLower(filepath.Ext(srcPath)) {
		case ".mp4", ".mov", ".m4a", ".m4b":
			w.Header().Set("Content-Type", "audio/mp4")
		case ".mka":
			// Matroska 音频容器（songloft-org/songloft#297）:stdlib 不识别 .mka,
			// 显式声明为 audio/x-matroska（转码失败回退原始文件时才会命中）。
			w.Header().Set("Content-Type", "audio/x-matroska")
		}
	}
	serveAudioFile(w, r, srcPath)
}

// serveAudioFile 与 http.ServeFile 保持 Range、条件请求和 MIME 推断语义，同时让播放活动取消
// 能在下一次读取前停止文件流。http.ServeFile 直接持有 *os.File，无法观察派生 context。
func serveAudioFile(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentWriter := &contextResponseWriter{ResponseWriter: w, ctx: r.Context()}
	http.ServeContent(contentWriter, r, info.Name(), info.ModTime(), &contextReadSeeker{
		ctx:  r.Context(),
		file: f,
	})
}

type contextResponseWriter struct {
	http.ResponseWriter
	ctx context.Context
}

func (w *contextResponseWriter) Write(p []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
	}
	return w.ResponseWriter.Write(p)
}

func (w *contextResponseWriter) ReadFrom(src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		select {
		case <-w.ctx.Done():
			return total, w.ctx.Err()
		default:
		}
		n, err := src.Read(buf)
		if n > 0 {
			written, writeErr := w.Write(buf[:n])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return total, nil
			}
			return total, err
		}
	}
}

type contextReadSeeker struct {
	ctx  context.Context
	file io.ReadSeeker
}

func (s *contextReadSeeker) Read(p []byte) (int, error) {
	select {
	case <-s.ctx.Done():
		return 0, s.ctx.Err()
	default:
	}
	return s.file.Read(p)
}

func (s *contextReadSeeker) Seek(offset int64, whence int) (int64, error) {
	select {
	case <-s.ctx.Done():
		return 0, s.ctx.Err()
	default:
	}
	return s.file.Seek(offset, whence)
}

// videoContentType 按视频容器扩展名返回对应的 MIME 类型;非视频容器返回空字符串。
func videoContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".ts":
		return "video/mp2t"
	case ".mpg", ".mpeg":
		return "video/mpeg"
	case ".flv":
		return "video/x-flv"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".rm", ".rmvb":
		return "application/vnd.rn-realmedia-vbr"
	case ".3gp":
		return "video/3gpp"
	}
	return ""
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
	if song.RadioMode == models.RadioModeAudioOnly {
		h.serveRadioAudio(w, r, song)
		return
	}

	// radio_transcode=<fmt>：把电台流实时转码为目标格式（典型 mp3）。用于只支持 MP3、无法解码
	// AAC/HE-AAC 或不支持 HLS 的音箱（songloft-org/songloft#275）。由客户端按设备能力下发；
	// 浏览器/桌面自带解码能力，不传此参数以保留原码最高音质。
	// 与 format 参数刻意分离：format 面向本地/网络歌曲的通用转码，电台侧一律忽略 format，
	// 只认 radio_transcode，避免「统一转 MP3」开关误连带影响电台。
	if transcodeFmt := services.NormalizeFormat(r.URL.Query().Get("radio_transcode")); transcodeFmt != "" && h.cacheService != nil {
		w.Header().Set("Content-Type", radioTranscodeContentType(transcodeFmt))
		w.Header().Set("Cache-Control", "no-cache, no-store")
		var referer string
		if u, err := url.Parse(song.URL); err == nil && u.Scheme != "" && u.Host != "" {
			referer = u.Scheme + "://" + u.Host + "/"
		}
		err := h.cacheService.StreamTranscodedRadio(r.Context(), w, services.RadioTranscodeOptions{
			UpstreamURL: song.URL,
			Format:      transcodeFmt,
			UserAgent:   radioStreamUserAgent,
			Referer:     referer,
		})
		if err == nil {
			return
		}
		if !errors.Is(err, services.ErrRadioTranscodeUnavailable) {
			// 转码已开始后中途失败：响应已提交，无法再降级，直接结束。
			return
		}
		// 转码未产出任何字节即失败（缺 ffmpeg / 坏源等）：此时尚未写出 body，
		// 清掉预设的响应头，降级为下面的原样代理 / 302。
		w.Header().Del("Content-Type")
		w.Header().Del("Cache-Control")
		slog.Warn("radio transcode unavailable, falling back to passthrough", "songId", song.ID, "url", song.URL, "error", err)
	}

	if song.IsHLSStream() {
		// hls=direct：原生 player（mpv/ExoPlayer/AVPlayer）自带 HLS 解析且无 CORS 限制，
		// 让它直接 302 到源站自行拉取切片。经本机反代会多一次「拉列表→改写→客户端回访切片」
		// 往返，对文件名带时间戳、窗口很短的直播源(#249 brtv-radiolive)会导致切片在回访前
		// 就滚出直播窗口(404 / expired from playlists)。反代仅为浏览器 CORS 而存在，
		// 原生端显式请求 direct 即绕过。防盗链 Referer/UA 由原生 player 自身请求头闭环。
		wantsDirect := r.URL.Query().Get("hls") == "direct"
		// HLS 反代开关由 HLSHandler 业务封装管理（/settings/hls-proxy），默认 false 走 302
		if !wantsDirect && h.hlsHandler != nil && h.hlsHandler.IsEnabled() {
			h.hlsHandler.ServeProxy(w, r, song)
			return
		}
		http.Redirect(w, r, song.URL, http.StatusFound)
		return
	}

	isFLV := song.IsFLVStream()
	if isFLV && r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "video/x-flv")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		return
	}
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, song.URL, nil)
	if err != nil {
		slog.Warn("radio stream request failed", "url", song.URL, "error", err)
		respondError(w, http.StatusInternalServerError, "resource fetch failed", nil)
		return
	}
	// 直连电台流用媒体播放器风格 UA，绝不用浏览器 UA：streamtheworld 等防盗链电台
	// 检测到浏览器 UA 会只回约 32KB 预览就断流（约 3 秒，songloft#275）。见 radioStreamUserAgent 注释。
	upstreamReq.Header.Set("User-Agent", radioStreamUserAgent)
	upstreamReq.Header.Set("Accept", streamAccept)
	// Icy-MetaData 透传:仅在客户端显式请求时才向上游要 ICY 元数据。
	// 浏览器 <audio> 既不发此头也不解析交织在音频里的元数据块;若无条件强制 Icy-MetaData:1,
	// 上游会按 icy-metaint 每隔 N 字节插入一段元数据,这些字节被浏览器当作音频解码,
	// 播放约 1 秒(16000 字节 ≈ 1.4s@88kbps)后即崩断(#275)。
	// 原生播放器(mpv/ExoPlayer 等)需要元数据时会自带此头,由下面的 icy-* 头透传闭环。
	clientWantsMeta := !isFLV && r.Header.Get("Icy-MetaData") != ""
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
		respondError(w, http.StatusBadGateway, "resource fetch failed", nil)
		return
	}
	defer resp.Body.Close()

	if isFLV && resp.StatusCode == http.StatusOK {
		body := bufio.NewReader(resp.Body)
		if magic, _ := body.Peek(3); string(magic) != "FLV" {
			respondError(w, http.StatusBadGateway, "电台未返回 FLV 直播流", nil)
			return
		}
		w.Header().Set("Content-Type", "video/x-flv")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, body); err != nil && !errors.Is(err, context.Canceled) {
			slog.Debug("FLV radio stream copy failed", "error", err)
		}
		return
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", normalizeAudioContentType(ct))
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
	// 电台是无限直播流，w.WriteHeader 已调用，流式输出已开始
	if _, err := io.Copy(w, body); err != nil && !errors.Is(err, context.Canceled) {
		slog.Debug("radio stream copy failed", "error", err)
	}
}

func (h *SongHandler) serveRadioAudio(w http.ResponseWriter, r *http.Request, song *models.Song) {
	if h.hlsTranscode == nil {
		respondError(w, http.StatusServiceUnavailable, "电台纯音频转码不可用", nil)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	err := h.hlsTranscode.StreamAudio(r.Context(), w, radioPullOptions(song.URL))
	var transcodeErr *services.HLSTranscodeError
	if errors.As(err, &transcodeErr) && r.Context().Err() == nil {
		w.Header().Del("Content-Type")
		respondHLSTranscodeError(w, transcodeErr)
	}
}

// normalizeAudioContentType 把上游返回的非标准音频 MIME 归一化为浏览器 / 解码器能识别的标准值。
// 典型:Shoutcast/streamtheworld 类 HE-AAC 电台返回 `audio/aacp`(遗留 MIME),浏览器 <audio>
// 与部分播放器无法据此选对解码器;实际负载是标准 ADTS AAC,改标 `audio/aac` 更兼容。(#275)
// 只改 MIME 主类型,保留可能存在的参数(如 charset);未命中的一律原样透传。
func normalizeAudioContentType(ct string) string {
	base, params, _ := strings.Cut(ct, ";")
	switch strings.ToLower(strings.TrimSpace(base)) {
	case "audio/aacp", "audio/x-aac", "audio/x-aacp":
		if params != "" {
			return "audio/aac;" + params
		}
		return "audio/aac"
	}
	return ct
}

// radioTranscodeContentType 返回电台实时转码目标格式对应的响应 Content-Type。
func radioTranscodeContentType(format string) string {
	switch format {
	case "mp3":
		return "audio/mpeg"
	case "ogg":
		return "audio/ogg"
	case "m4a":
		return "audio/mp4"
	case "flac":
		return "audio/flac"
	case "wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

// serveRemote 网络歌曲:根据音源类型分发到缓存或代理。
// - 已缓存歌曲:校验后提供本地文件，无效缓存淘汰后重新解析 URL。
// - 未缓存歌曲:解析 URL 后流式代理，完整文件异步校验落盘。
// 失败时:返回 502,后台异步切源(若注入了 reassigner),客户端下次播放该 song 会用新源。
// targetFormat 非空且与原格式不同时,对已缓存文件走 ffmpeg 转码。
func (h *SongHandler) serveRemote(w http.ResponseWriter, r *http.Request, song *models.Song, opts servePlayOptions) {
	// 1. 缓存命中 → 直接 ServeFile
	if cachedPath, ok := h.cacheService.FindCachedFileBySong(song); ok {
		h.serveCachedFile(w, r, song, cachedPath, opts)
		return
	}

	// 到这里缓存未命中：seek 只服务已缓存的网络歌曲。未命中时拿到本地文件必须先同步下载整首，
	// 会让「续播」这一下按键卡住一整首歌的下载时长；而刚在播的歌几乎必然已缓存，代价可忽略。
	// 变速同理：atempo 也只服务已缓存的网络歌曲，未命中时直接原速代理原始流。
	if opts.seekSeconds > 0 {
		slog.Info("seek ignored for uncached remote song", "songId", song.ID, "seek", opts.seekSeconds)
	}
	if speedRequested(opts.speed) {
		slog.Info("speed ignored for uncached remote song", "songId", song.ID, "speed", opts.speed)
	}

	// 2. 缓存未命中：解析播放 URL
	var playURL string
	var upstreamHeaders map[string]string
	if song.IsPluginSourced() {
		// 解析插件直链不能绑定客户端连接：libmpv 等播放器对「已连接但迟迟无数据」的
		// 连接有 ~5s 硬上限，会在慢音源（如 B站 music/url 解析要 ~9s）出结果前主动断开，
		// 令 r.Context() cancel → scheduler 立刻 ErrCallTimeout → 502（songloft#271）。
		// 改用 background 派生 ctx + 服务端预算，并注册进 playActivity（CatPlay）让用户
		// 切到其他歌时仍能被 Activate 取消，只是不再受本连接断开牵连。
		sk := playactivity.SessionFromContext(r.Context())
		resolveCtx, cancelResolve := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancelResolve()
		trackedCtx, releaseResolve := h.trackActivity(resolveCtx, sk, song.ID, playactivity.CatPlay)
		defer releaseResolve()
		resolved, err := h.cacheService.ResolveURL(trackedCtx, song)
		if err != nil {
			slog.Warn("resolve url failed", "songId", song.ID, "error", err)
			sk := playactivity.SessionFromContext(r.Context())
			if h.reassigner != nil {
				h.reassigner.AsyncReassign(song.ID, sk)
			}
			respondError(w, http.StatusBadGateway, "source unavailable", nil)
			return
		}
		playURL = resolved.URL
		upstreamHeaders = resolved.Headers

		// 解析期间客户端可能已超时断开（Windows Flutter 播放器 ~10s 硬上限），
		// 此时 r.Context() 已取消，继续走 ServeRemoteResourceWithCache 只会
		// 立刻失败且不留缓存。改为触发后台异步下载，下次重试即命中缓存秒开。
		if r.Context().Err() != nil {
			slog.Info("client disconnected during resolve, triggering background cache", "songId", song.ID)
			songCopy := *song
			go h.cacheService.AsyncDownloadAndCache(context.Background(), &songCopy, playURL, upstreamHeaders)
			return
		}
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
			h.metadataRefresher.RefreshSong(ctx, &refreshCopy, playURL, nil)
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
func (h *SongHandler) serveCachedFile(w http.ResponseWriter, r *http.Request, song *models.Song, cachedPath string, opts servePlayOptions) {
	targetFormat, bitrate, normalize := opts.targetFormat, opts.bitrate, opts.normalize
	if services.NeedsTranscodeForServe(song, cachedPath, targetFormat) || bitrate > 0 || normalize {
		// 同 serveLocal：转码产物未就绪时先边转边发，不让整首转码挂住这个请求。
		if h.tryLiveTranscodeStream(w, r, song, cachedPath, opts) {
			return
		}
		sk := playactivity.SessionFromContext(r.Context())
		tcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		trackedCtx, releaseTc := h.trackActivity(tcCtx, sk, song.ID, playactivity.CatTranscode)
		defer releaseTc()
		path, err := h.cacheService.GetOrTranscode(trackedCtx, cachedPath, song, services.NormalizeFormat(targetFormat), bitrate, -1, normalize)
		if err != nil {
			slog.Warn("transcode failed, serving original", "songId", song.ID, "format", targetFormat, "bitrate", bitrate, "error", err)
		} else {
			cachedPath = path
		}
	}
	// 同 serveLocal：seek 作用在已定型的 cachedPath 上，与转码/均衡自动叠加。
	if h.trySeekStream(w, r, song, cachedPath, opts) {
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=604800")
	serveAudioFile(w, r, cachedPath)
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
// @Description 根据 song.ID 返回 LyricPayload JSON，含 lyric/tlyric/rlyric/lxlyric。优先级：旁挂 .lrc 文件 > DB url > DB payload > 歌词搜索插件。manual 歌词不被旁挂覆盖。传 refresh=1 时强制重新抓取：跳过库中自动获取的旧歌词(空/scraped/cached)重跑歌词搜索插件，响应挂 no-store 不缓存；file/embedded/manual 等权威歌词不被覆盖。
// @Tags 歌曲管理
// @Produce json
// @Param id path int true "歌曲 ID"
// @Param refresh query bool false "为 true 时绕过缓存强制重新抓取歌词（重跑歌词搜索插件，不覆盖 file/embedded/manual 歌词）"
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

	// refresh=1：用户在播放页手动触发的强制重抓。跳过库中自动获取的旧歌词
	// (空/scraped/cached)直接重跑歌词搜索插件；file/embedded/manual 为权威来源
	// (用户手动/文件/内嵌)，仍直接返回、不被搜索结果覆盖。
	refresh := r.URL.Query().Get("refresh") != "" && r.URL.Query().Get("refresh") != "false"
	authoritative := song.LyricSource == models.LyricSourceFile ||
		song.LyricSource == models.LyricSourceEmbedded ||
		song.LyricSource == models.LyricSourceManual

	var payload models.LyricPayload
	var sidecarHit bool

	// Sidecar .lrc file takes priority over all DB sources (except manual).
	if sidecar := services.SidecarLyricForSong(song); sidecar != "" {
		sidecarHit = true
		payload = models.LyricPayloadFromLRC(sidecar)
		if m := payload.MarshalString(); m != song.Lyric || song.LyricSource != models.LyricSourceFile {
			go func() {
				if err := h.songService.SyncSidecarLyric(context.Background(), song.ID, m); err != nil {
					slog.Warn("SyncSidecarLyric failed", "songId", song.ID, "error", err)
				}
			}()
		}
	} else if song.LyricSource == models.LyricSourceURL {
		if song.LyricRemoteURL != "" && h.lyricFetcher != nil {
			p, err := h.lyricFetcher.Fetch(ctx, song.LyricRemoteURL)
			if err != nil {
				// 手动刷新时 url 拉取失败不直接 502，继续走歌词搜索插件兜底
				if !refresh {
					respondError(w, http.StatusBadGateway, "歌词获取失败", err)
					return
				}
			} else {
				payload = p
			}
		}
	} else if song.Lyric != "" && (!refresh || authoritative) {
		payload = models.UnmarshalLyric(song.Lyric)
	}

	// 歌词为空时（或手动刷新且非权威来源时），尝试从已注册的歌词提供者插件获取
	if payload.IsEmpty() && h.lyricSearcher != nil {
		if found, err := h.lyricSearcher.SearchLyrics(ctx, song.Title, song.Artist, song.Album, song.Duration, song.Fingerprint, song.ISRC); err == nil && found != nil && !found.IsEmpty() {
			go h.songService.UpdateLyrics(context.Background(), song.ID, found.MarshalString(), models.LyricSourceScraped, "")
			payload = *found
		}
	}

	// 手动刷新的响应一律禁缓存，避免浏览器/客户端把「本次结果」再缓存一年，
	// 导致下次刷新仍拿到旧值（Web 上尤其明显）。
	if refresh {
		w.Header().Set("Cache-Control", "no-store")
	}

	if payload.IsEmpty() {
		http.NotFound(w, r)
		return
	}

	if !refresh {
		if sidecarHit {
			w.Header().Set("Cache-Control", "public, max-age=600")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000")
		}
	}
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
	Disc       string `json:"disc"`
	Lyrics     string `json:"lyrics"`
	CoverData  string `json:"cover_data"`
	CoverURL   string `json:"cover_url"`
	ClearCover bool   `json:"clear_cover"`
	// RenameFile 为 true 时按新标题重命名本地音频文件（保留原目录与扩展名），仅对本地非 CUE 歌曲生效。
	RenameFile bool `json:"rename_file"`
}

// WriteTags 写入歌曲标签
// @Summary 写入歌曲标签
// @Description 将元数据写入数据库和本地音频文件标签（仅本地歌曲）。cover_data(base64) 优先于 cover_url。非空字段覆盖，空值保留原值。设置 clear_cover=true 可显式清空封面。rename_file=true 时按 "{artist} - {title}" 重命名本地音频文件（保留原目录与扩展名，artist 为空则回退为仅 title，多歌手由 artist 中的 " & " 分隔符体现；仅本地非 CUE 歌曲生效）；标题清理后为空或目标文件名已存在时返回 400，与原文件同名则不移动仅写库。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param id path int true "歌曲ID"
// @Param request body WriteSongTagsRequest true "标签数据"
// @Success 200 {object} object{song=models.Song,file_write=string} "写入结果"
// @Failure 400 {object} map[string]string "请求错误"
// @Failure 404 {object} map[string]string "歌曲不存在"
// @Security BearerAuth
// @Router /songs/{id}/tags [put]
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
	if req.Disc != "" {
		song.Disc = req.Disc
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

// UpdateSongRating 设置单首歌曲的评级。
// @Summary 设置歌曲评级
// @Description 设置单首歌曲的评级（0-100 整数；0 表示清除评级，未评级歌曲缺省 0）。评级是用户主观标注，仅存数据库，不写入音频文件标签。星级 UI 按 20/颗映射（如 80 = 4 星）。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param id path int true "歌曲ID"
// @Param request body object{rating=int} true "评级值 0-100，0 表示清除评级"
// @Success 200 {object} map[string]any "成功返回 {id, rating}"
// @Failure 400 {object} map[string]string "评级值超出 0-100 范围"
// @Failure 404 {object} map[string]string "歌曲不存在"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/{id}/rating [put]
func (h *SongHandler) UpdateSongRating(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌曲ID", err)
		return
	}

	var req struct {
		Rating *int `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}
	if req.Rating == nil {
		respondError(w, http.StatusBadRequest, "缺少 rating 字段", nil)
		return
	}

	if err := h.songService.UpdateRating(ctx, id, *req.Rating); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			respondError(w, http.StatusNotFound, "歌曲不存在", err)
			return
		}
		respondError(w, http.StatusBadRequest, "设置评级失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"id": id, "rating": *req.Rating})
}

// BatchUpdateSongRating 批量设置歌曲评级。
// @Summary 批量设置歌曲评级
// @Description 按 ID 列表批量设置评级（0-100；0 表示清除评级）。列表中不存在的 ID 静默跳过，updated 返回实际更新数量。评级仅存数据库，不写入音频文件标签。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body models.BatchUpdateRatingRequest true "批量评级请求"
// @Success 200 {object} models.BatchUpdateRatingResponse "成功返回 {updated}"
// @Failure 400 {object} map[string]string "请求数据错误或评级值超出范围"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/batch-rating [post]
func (h *SongHandler) BatchUpdateSongRating(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.BatchUpdateRatingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}
	if len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "ID 列表不能为空", nil)
		return
	}
	if req.Rating < 0 || req.Rating > 100 {
		respondError(w, http.StatusBadRequest, "评级值超出 0-100 范围", nil)
		return
	}

	updated, err := h.songService.UpdateRatingBatch(ctx, req.IDs, req.Rating)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "批量设置评级失败", err)
		return
	}

	respondJSON(w, http.StatusOK, models.BatchUpdateRatingResponse{Updated: int(updated)})
}

// BatchUpdateRadioMode 批量设置电台播放模式。
// @Summary 批量设置电台播放模式
// @Description 按 ID 列表批量设置电台播放模式。radio_mode 支持空字符串（自动）、hls、flv、audio_only；is_video 省略时保留各电台原值，仅音频模式始终清除视频内容。不存在的 ID 静默跳过，非电台歌曲或无效模式会拒绝整批更新。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body models.BatchUpdateRadioModeRequest true "批量电台模式请求"
// @Success 200 {object} models.BatchUpdateRadioModeResponse "更新成功"
// @Failure 400 {object} map[string]string "请求数据错误或包含非电台歌曲"
// @Failure 500 {object} map[string]string "更新失败"
// @Security BearerAuth
// @Router /songs/batch-radio-mode [post]
func (h *SongHandler) BatchUpdateRadioMode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.BatchUpdateRadioModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}
	if len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "ID 列表不能为空", nil)
		return
	}

	updated, err := h.songService.UpdateRadioModeBatch(ctx, req.IDs, req.RadioMode, req.IsVideo)
	if err != nil {
		if errors.Is(err, services.ErrInvalidRadioModeBatch) {
			respondError(w, http.StatusBadRequest, "电台播放模式或歌曲类型无效", err)
			return
		}
		respondError(w, http.StatusInternalServerError, "批量设置电台播放模式失败", err)
		return
	}

	respondJSON(w, http.StatusOK, models.BatchUpdateRadioModeResponse{Updated: int(updated)})
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
// @Router /songs/organize [post]
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
// @Router /songs/organize/preview [post]
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

	respondJSON(w, http.StatusOK, map[string]any{
		"groups":           result,
		"total_groups":     len(result),
		"total_duplicates": totalDuplicates,
	})
}

// SongPlayed 通知歌曲播放事件
// @Summary 通知歌曲播放事件
// @Description 客户端在歌曲开始播放、播放完成或被跳过时调用此端点，后端将事件广播给已订阅播放事件的 JS 插件（通过 songloft.events.onPlayEvent 注册）。source 参数标识调用来源，如 songloft-player（官方客户端）、miot（小爱音箱插件）等。type 参数标识事件类型：play（开始播放）、finish（播放完成）、skip（用户跳过）。
// @Description 副作用：当 type=play 且同时传入合法的 context_type + context_key 时，额外把该歌曲写入对应播放上下文的播放历史（见 GET /play-history），同一上下文内按歌曲去重、只保留最近 50 条。仅 type=play 会落库：finish 是同一首歌的重复信息，而 skip 上报的是上一首歌、此时上下文可能已切换，会记错归属。落库失败只记日志，不影响响应码。
// @Tags 歌曲管理
// @Produce json
// @Param id path int true "歌曲 ID"
// @Param source query string false "调用来源标识，如 songloft-player、miot"
// @Param type query string false "事件类型：play、finish、skip，默认 finish" Enums(play, finish, skip)
// @Param context_type query string false "播放上下文类型，仅 type=play 时生效：playlist、tag 或分面维度（artist/album/genre/year/decade/language/style）" Enums(playlist, tag, artist, album, genre, year, decade, language, style)
// @Param context_key query string false "播放上下文标识，仅 type=play 时生效：playlist 传歌单 ID，tag 传标签 ID，分面维度传该维度取值（如歌手名）"
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

	// 播放历史只认 type=play：finish 是同一首歌的重复写入，而 skip 上报的是上一首歌，
	// 此时客户端的播放上下文可能已经切换，会把上一首错记到新上下文名下。
	// 同步执行（2 条 SQL，亚毫秒）——放进 goroutine 会让 r.Context() 提前取消。
	if h.playHistory != nil && eventType == "play" {
		contextType := r.URL.Query().Get("context_type")
		contextKey := r.URL.Query().Get("context_key")
		if contextType != "" && contextKey != "" {
			if err := h.playHistory.Record(r.Context(), contextType, contextKey, song.ID, time.Now()); err != nil {
				slog.Warn("记录播放历史失败",
					"song_id", song.ID, "context_type", contextType, "context_key", contextKey, "error", err)
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetLibraryStats 获取曲库汇总统计
// @Summary 获取曲库汇总统计
// @Description 返回曲库汇总信息，包括歌曲总数、各类型数量、总时长、总文件大小、歌手/专辑/流派数等
// @Tags 歌曲管理
// @Produce json
// @Success 200 {object} database.LibraryStats "曲库汇总统计"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/stats [get]
func (h *SongHandler) GetLibraryStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.songService.GetLibraryStats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取曲库统计失败", err)
		return
	}
	respondJSON(w, http.StatusOK, stats)
}
