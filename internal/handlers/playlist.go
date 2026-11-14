package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"songloft/internal/database"
	"songloft/internal/models"
	"songloft/internal/services"

	"github.com/go-chi/chi/v5"
)

// PlaylistHandler 歌单处理器
type PlaylistHandler struct {
	playlistService *services.PlaylistService
	songService     *services.SongService
	thumbCache      *services.CoverThumbCache // 缩略图磁盘缓存（可选，nil 安全）
	urlResolver     *services.InternalURLResolver
}

// NewPlaylistHandler 创建歌单处理器
func NewPlaylistHandler(playlistService *services.PlaylistService, songService *services.SongService) *PlaylistHandler {
	return &PlaylistHandler{
		playlistService: playlistService,
		songService:     songService,
	}
}

// SetThumbCache 注入缩略图磁盘缓存。
func (h *PlaylistHandler) SetThumbCache(tc *services.CoverThumbCache) {
	h.thumbCache = tc
}

func (h *PlaylistHandler) SetURLResolver(resolver *services.InternalURLResolver) {
	h.urlResolver = resolver
}

// ListPlaylists 获取歌单列表
// @Summary 获取歌单列表
// @Description 获取歌单列表，支持按类型过滤、按歌单内歌曲来源过滤、关键词搜索和分页。默认排除隐藏歌单，传 exclude_labels=none 显示全部。
// @Description song_source 按歌单内歌曲的来源筛选（歌单本身没有来源字段，只能由歌曲反推）：remote=歌单内含网络歌曲（「网络歌单」），local=含本地歌曲（「本地歌单」）。
// @Description 判定是 EXISTS 而非「全部是」，故本地+网络混合的歌单在两个取值下都会出现，空歌单两个取值下都不出现。电台歌曲的 type 是 radio 而非 remote，故电台歌单不会被 song_source=remote 命中。
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param type query string false "歌单类型" Enums(normal, radio)
// @Param song_source query string false "按歌单内歌曲来源过滤: remote=含网络歌曲, local=含本地歌曲" Enums(remote, local)
// @Param keyword query string false "搜索关键词（模糊匹配歌单名称/描述）"
// @Param tag_id query int false "按共享自定义标签筛选"
// @Param exclude_labels query string false "要排除的标签(逗号分隔), 默认排除 hidden; 传 none 显示全部" default(hidden)
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{} "成功返回歌单列表"
// @Failure 400 {object} map[string]string "song_source 取值非法"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /playlists [get]
func (h *PlaylistHandler) ListPlaylists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	playlistType := r.URL.Query().Get("type")
	keyword := r.URL.Query().Get("keyword")

	// song_source 直接进 SQL 的 s.type 比较，必须白名单校验，不能透传任意值。
	songSource := r.URL.Query().Get("song_source")
	if songSource != "" && songSource != models.TypeRemote && songSource != models.TypeLocal {
		respondError(w, http.StatusBadRequest, "无效的 song_source，仅支持 remote 或 local", nil)
		return
	}

	limit, offset := parsePagination(r, models.DefaultPaginationLimit, 0)
	var labels []string
	if raw := r.URL.Query().Get("tag_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			respondError(w, 400, "无效的 tag_id", err)
			return
		}
		labels = []string{models.PlaylistTagLabel(id)}
	}

	var excludeLabels []string
	excludeLabelsStr := r.URL.Query().Get("exclude_labels")
	if excludeLabelsStr == "" {
		excludeLabels = []string{models.PlaylistLabelHidden}
	} else if excludeLabelsStr != "none" {
		excludeLabels = strings.Split(excludeLabelsStr, ",")
	}

	filter := &database.PlaylistFilter{
		Labels:        labels,
		Type:          playlistType,
		SongSource:    songSource,
		Keyword:       keyword,
		ExcludeLabels: excludeLabels,
		Limit:         limit,
		Offset:        offset,
	}

	playlists, err := h.playlistService.List(ctx, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌单列表失败", err)
		return
	}

	countFilter := &database.PlaylistFilter{
		Labels:        labels,
		Type:          filter.Type,
		SongSource:    songSource,
		Keyword:       keyword,
		ExcludeLabels: excludeLabels,
	}
	total, err := h.playlistService.Count(ctx, countFilter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌单总数失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"playlists": playlists,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// GetPlaylist 获取单个歌单
// @Summary 获取单个歌单详情
// @Description 根据歌单ID获取详细信息
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Success 200 {object} models.Playlist "成功返回歌单详情"
// @Failure 400 {object} map[string]string "无效的歌单ID"
// @Failure 404 {object} map[string]string "歌单不存在"
// @Security BearerAuth
// @Router /playlists/{id} [get]
func (h *PlaylistHandler) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单ID", err)
		return
	}

	playlist, err := h.playlistService.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "歌单不存在", err)
		return
	}

	respondJSON(w, http.StatusOK, playlist)
}

// CreatePlaylist 创建歌单
// @Summary 创建歌单
// @Description 创建一个新的歌单
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param request body models.Playlist true "歌单信息"
// @Success 201 {object} models.Playlist "创建成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "创建失败"
// @Security BearerAuth
// @Router /playlists [post]
func (h *PlaylistHandler) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var playlist models.Playlist
	if err := json.NewDecoder(r.Body).Decode(&playlist); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if err := h.playlistService.Create(ctx, &playlist); err != nil {
		if errors.Is(err, models.ErrPlaylistNameConflict) {
			respondError(w, http.StatusConflict, "已存在同名歌单", err)
			return
		}
		respondError(w, http.StatusInternalServerError, "创建歌单失败", err)
		return
	}

	respondJSON(w, http.StatusCreated, playlist)
}

// UpdatePlaylist 更新歌单
// @Summary 更新歌单
// @Description 更新歌单信息。支持通过 cover_song_id 从指定歌曲复制封面，与 cover_path/cover_url 互斥且优先级更高
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Param request body models.UpdatePlaylistRequest true "歌单信息"
// @Success 200 {object} models.Playlist "更新成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "更新失败"
// @Security BearerAuth
// @Router /playlists/{id} [put]
func (h *PlaylistHandler) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单ID", err)
		return
	}

	var req models.UpdatePlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	existing, err := h.playlistService.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "歌单不存在", err)
		return
	}

	existing.Name = req.Name
	if req.Audiobook != nil && existing.Type == models.PlaylistTypeNormal {
		labels := make([]string, 0, len(existing.Labels)+1)
		for _, label := range existing.Labels {
			if label != "audiobook" {
				labels = append(labels, label)
			}
		}
		if *req.Audiobook {
			labels = append(labels, "audiobook")
		}
		existing.Labels = labels
	}
	existing.Description = req.Description

	if req.CoverSongID != nil {
		song, err := h.songService.GetByID(ctx, *req.CoverSongID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "封面来源歌曲不存在", err)
			return
		}
		existing.CoverPath = song.CoverPath
		existing.CoverURL = song.CoverURL
	} else {
		if req.CoverPath != nil {
			existing.CoverPath = *req.CoverPath
		}
		if req.CoverURL != nil {
			existing.CoverURL = *req.CoverURL
		}
	}

	if err := h.playlistService.Update(ctx, existing); err != nil {
		if errors.Is(err, models.ErrPlaylistNameConflict) {
			respondError(w, http.StatusConflict, "已存在同名歌单", err)
			return
		}
		respondError(w, http.StatusInternalServerError, "更新歌单失败", err)
		return
	}

	respondJSON(w, http.StatusOK, existing)
}

// GetPlaylistSongIDs 返回歌单内全部歌曲 ID（有序、不分页）
// @Summary 获取歌单歌曲 ID 列表
// @Description 返回歌单内全部歌曲的 ID，不分页。sort/order 语义与 GET /playlists/{id}/songs 完全一致（同一套排序逻辑），省略时默认 position 升序。
// @Description title（歌曲名称）/artist/album 按不区分大小写的拼音字母序排列，忽略首尾空白；中英文混排，多音字使用字典默认读音。排序键相同时按歌单 position 同方向排列。
// @Description 用途：客户端需要知道「某首歌在歌单里排第几」时（如从播放历史里的某首歌接着往下播），用本端点拿到有序 ID 数组后取下标，即可直接作为 /playlists/{id}/songs 的 offset 使用（传相同的 sort/order），避免为此拉取全部歌曲对象。
// @Description 形态与 GET /songs/ids 对齐。
// @Tags 歌单管理
// @Produce json
// @Param id path int true "歌单 ID"
// @Param sort query string false "排序字段: position(默认)/added_at/title/artist/album/duration/updated_at/file_modified_at/file_size"
// @Param order query string false "排序方向: asc(默认)/desc"
// @Success 200 {object} map[string]any "成功返回 {ids:[1,2,3], total:3}"
// @Failure 400 {object} map[string]string "无效的歌单 ID"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /playlists/{id}/song-ids [get]
func (h *PlaylistHandler) GetPlaylistSongIDs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "无效的歌单ID", err)
		return
	}

	q := r.URL.Query()
	ids, err := h.playlistService.SongIDsOrdered(r.Context(), id, q.Get("sort"), q.Get("order"))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌单歌曲 ID 列表失败", err)
		return
	}
	if ids == nil {
		ids = []int64{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"ids":   ids,
		"total": len(ids),
	})
}

// TouchPlaylist 更新歌单的最后播放时间
// @Summary 更新歌单最后播放时间
// @Description 仅更新歌单的 updated_at 字段，作为歌单级的粗粒度「最后播放时间」，也会被改名/换封面等更新操作刷新。
// @Description 歌曲级的精确播放历史见 GET /play-history?context_type=playlist&context_key={id}。
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Success 200 {object} map[string]string "更新成功"
// @Failure 400 {object} map[string]string "无效的歌单ID"
// @Failure 500 {object} map[string]string "更新失败"
// @Security BearerAuth
// @Router /playlists/{id}/touch [post]
func (h *PlaylistHandler) TouchPlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单ID", err)
		return
	}

	if err := h.playlistService.Touch(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, "更新歌单播放时间失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "歌单播放时间已更新",
	})
}

// UpdatePlaylistSort 更新歌单视图排序偏好
// @Summary 更新歌单视图排序偏好
// @Description 保存歌单的视图排序设置（非破坏性排序），下次打开歌单时恢复该排序
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Param request body object true "排序设置 {sort_by, sort_order}"
// @Success 200 {object} map[string]string "更新成功"
// @Failure 400 {object} map[string]string "无效参数"
// @Failure 500 {object} map[string]string "更新失败"
// @Security BearerAuth
// @Router /playlists/{id}/sort [put]
func (h *PlaylistHandler) UpdatePlaylistSort(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "无效的歌单ID", err)
		return
	}

	var req struct {
		SortBy    string `json:"sort_by"`
		SortOrder string `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if err := h.playlistService.UpdateSort(r.Context(), id, req.SortBy, req.SortOrder); err != nil {
		respondError(w, http.StatusInternalServerError, "更新排序偏好失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "ok"})
}

// DeletePlaylist 删除歌单
// @Summary 删除歌单
// @Description 根据歌单ID删除歌单。delete_songs=true 时，同时删除仅属于本歌单的孤儿歌曲（不属于任何其他歌单，含内置的收藏/电台收藏保护）——本地歌曲连同磁盘文件一并删除，网络/电台歌曲清理数据库记录与缓存。
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Param delete_songs query bool false "是否一并删除仅属于本歌单的孤儿歌曲（含本地文件），默认 false"
// @Success 200 {object} map[string]interface{} "删除成功，含连带清理的歌曲数 deleted_songs"
// @Failure 400 {object} map[string]string "无效的歌单ID"
// @Failure 500 {object} map[string]string "删除失败"
// @Security BearerAuth
// @Router /playlists/{id} [delete]
func (h *PlaylistHandler) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单ID", err)
		return
	}

	deleteSongs := r.URL.Query().Get("delete_songs") == "true"

	// 删歌单前先收集歌单内全部歌曲 ID：歌单删除后 playlist_songs 关联被 FK CASCADE 清空，
	// 需在此之前拿到候选，才能在删后判定哪些歌曲已无任何归属（孤儿）。
	var candidateIDs []int64
	if deleteSongs {
		if ids, err := h.playlistService.SongIDsInPlaylist(ctx, id); err != nil {
			slog.Warn("收集歌单歌曲失败，跳过孤儿清理", "playlistId", id, "error", err)
		} else {
			candidateIDs = ids
		}
	}

	if err := h.playlistService.Delete(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, "删除歌单失败", err)
		return
	}

	deletedSongs := 0
	if deleteSongs && len(candidateIDs) > 0 {
		if n, err := h.songService.DeleteOrphanSongs(ctx, candidateIDs, true); err != nil {
			slog.Warn("清理孤儿歌曲失败", "playlistId", id, "error", err)
		} else {
			deletedSongs = n
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message":       "歌单已删除",
		"deleted_songs": deletedSongs,
	})
}

// BatchDeletePlaylists 批量删除歌单
// @Summary 批量删除歌单
// @Description 根据歌单 ID 列表批量删除歌单，内置歌单会被跳过。请求体 delete_songs=true 时，同时删除仅属于这些歌单的孤儿歌曲（不属于任何其他歌单）——本地歌曲连同磁盘文件一并删除。
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param request body models.BatchDeletePlaylistsRequest true "批量删除请求"
// @Success 200 {object} models.BatchDeletePlaylistsResponse "删除成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "删除失败"
// @Security BearerAuth
// @Router /playlists/batch-delete [post]
func (h *PlaylistHandler) BatchDeletePlaylists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.BatchDeletePlaylistsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "请提供要删除的歌单 ID 列表", nil)
		return
	}

	// 删歌单前收集所有待删歌单内歌曲 ID 的并集（去重）作为孤儿清理候选。
	// 内置歌单会被 BatchDelete 跳过，其歌曲仍在 playlist_songs 中，故不会被误判为孤儿。
	var candidateIDs []int64
	if req.DeleteSongs {
		seen := make(map[int64]struct{})
		for _, pid := range req.IDs {
			ids, err := h.playlistService.SongIDsInPlaylist(ctx, pid)
			if err != nil {
				slog.Warn("收集歌单歌曲失败，跳过该歌单孤儿清理", "playlistId", pid, "error", err)
				continue
			}
			for _, sid := range ids {
				if _, ok := seen[sid]; !ok {
					seen[sid] = struct{}{}
					candidateIDs = append(candidateIDs, sid)
				}
			}
		}
	}

	deleted, err := h.playlistService.BatchDelete(ctx, req.IDs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "批量删除歌单失败", err)
		return
	}

	deletedSongs := 0
	if req.DeleteSongs && len(candidateIDs) > 0 {
		if n, err := h.songService.DeleteOrphanSongs(ctx, candidateIDs, true); err != nil {
			slog.Warn("清理孤儿歌曲失败", "playlistIds", req.IDs, "error", err)
		} else {
			deletedSongs = n
		}
	}

	respondJSON(w, http.StatusOK, models.BatchDeletePlaylistsResponse{
		Deleted:      deleted,
		DeletedSongs: deletedSongs,
	})
}

// GetPlaylistSongs 获取歌单中的歌曲
// @Summary 获取歌单中的歌曲
// @Description 获取指定歌单中的歌曲，支持分页、排序和搜索
// @Description playlist_added_at 是歌曲加入当前歌单的时间；added_at 保持曲库入库时间，二者不互相覆盖。
// @Description title 为歌曲名称；title/artist/album 按不区分大小写的拼音字母序排列，忽略首尾空白；中英文混排，多音字使用字典默认读音。先对全部匹配歌曲排序再分页，排序键相同时按歌单 position 同方向排列。
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单 ID"
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Param sort query string false "排序字段: position(默认)/added_at/title/artist/album/duration/updated_at/file_modified_at/file_size"
// @Param order query string false "排序方向: asc(默认)/desc"
// @Param keyword query string false "搜索关键词（匹配标题/艺术家/专辑）"
// @Success 200 {object} map[string]interface{} "成功返回歌曲列表"
// @Failure 400 {object} map[string]string "无效的歌单 ID"
// @Failure 500 {object} map[string]string "获取失败"
// @Security BearerAuth
// @Router /playlists/{id}/songs [get]
func (h *PlaylistHandler) GetPlaylistSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单 ID", err)
		return
	}

	q := r.URL.Query()

	limit, offset := parsePagination(r, models.DefaultPaginationLimit, 0)

	filter := database.PlaylistSongFilter{
		Keyword: q.Get("keyword"),
		OrderBy: q.Get("sort"),
		Order:   q.Get("order"),
		Limit:   limit,
		Offset:  offset,
	}

	songs, err := h.playlistService.GetSongs(ctx, id, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌单歌曲失败", err)
		return
	}

	total, err := h.playlistService.CountSongs(ctx, id, filter.Keyword)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌曲总数失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"songs":  songs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// AddSongToPlaylist 批量添加歌曲到歌单
// @Summary 批量添加歌曲到歌单
// @Description 将多首歌曲添加到指定歌单，跳过已存在的歌曲
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单 ID"
// @Param request body object{song_ids=[]int64} true "歌曲 ID 列表"
// @Success 200 {object} map[string]interface{} "添加成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "添加失败"
// @Security BearerAuth
// @Router /playlists/{id}/songs [post]
func (h *PlaylistHandler) AddSongToPlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	playlistID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单 ID", err)
		return
	}

	var req struct {
		SongIDs []int64 `json:"song_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if len(req.SongIDs) == 0 {
		respondError(w, http.StatusBadRequest, "请提供 song_ids", nil)
		return
	}

	added, skipped, err := h.playlistService.AddSongs(ctx, playlistID, req.SongIDs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "添加歌曲到歌单失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "歌曲已添加到歌单",
		"added":   added,
		"skipped": skipped,
	})
}

// RemoveSongFromPlaylist 从歌单移除歌曲
// @Summary 从歌单移除歌曲
// @Description 从指定歌单移除歌曲
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单 ID"
// @Param songId path int true "歌曲 ID"
// @Success 200 {object} map[string]string "移除成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "移除失败"
// @Security BearerAuth
// @Router /playlists/{id}/songs/{songId} [delete]
func (h *PlaylistHandler) RemoveSongFromPlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	songIdStr := chi.URLParam(r, "songId")

	playlistID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单 ID", err)
		return
	}

	songID, err := strconv.ParseInt(songIdStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌曲 ID", err)
		return
	}

	if err := h.playlistService.RemoveSong(ctx, playlistID, songID); err != nil {
		respondError(w, http.StatusInternalServerError, "从歌单移除歌曲失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "歌曲已从歌单移除",
	})
}

// ReorderPlaylistSongs 重新排序歌单中的歌曲
// @Summary 重新排序歌单中的歌曲
// @Description 按完整歌曲 ID 列表更新成员位置，支持普通歌单及内置收藏／电台收藏歌单，不改变歌单身份或收藏成员。
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单 ID"
// @Param request body object{song_ids=[]int64} true "歌曲 ID 列表"
// @Success 200 {object} map[string]string "排序成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "排序失败"
// @Security BearerAuth
// @Router /playlists/{id}/songs/reorder [put]
func (h *PlaylistHandler) ReorderPlaylistSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	playlistID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单 ID", err)
		return
	}

	var req struct {
		SongIDs []int64 `json:"song_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if err := h.playlistService.ReorderSongs(ctx, playlistID, req.SongIDs); err != nil {
		respondError(w, http.StatusInternalServerError, "重新排序歌单歌曲失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "歌单歌曲已重新排序",
	})
}

// SortPlaylistSongs 服务端排序歌单内歌曲（永久排序）
// @Summary 服务端排序歌单内歌曲
// @Description 根据 action 参数对歌单内歌曲进行永久排序（更新 position）。
// @Description action 取值：name_asc（按名称 A→Z）、name_desc（按名称 Z→A）、number_prefix（按标题数字前缀）、shuffle（随机打乱）。
// @Description name_asc/name_desc 使用与 title 视图排序相同的歌曲名称拼音字母序，忽略大小写和首尾空白，多音字使用字典默认读音。
// @Description 与 /playlists/{id}/songs/reorder 不同，本端点无需客户端传入完整歌曲 ID 列表，排序由服务端完成。
// @Description 支持内置收藏／电台收藏歌单，仅调整成员位置，不改变歌单身份或收藏成员。
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单 ID"
// @Param request body object{action=string} true "排序动作: name_asc / name_desc / number_prefix / shuffle"
// @Success 200 {object} map[string]string "排序成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "排序失败"
// @Security BearerAuth
// @Router /playlists/{id}/songs/sort [post]
func (h *PlaylistHandler) SortPlaylistSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	playlistID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单 ID", err)
		return
	}

	var req struct {
		Action string `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if err := h.playlistService.SortSongs(ctx, playlistID, req.Action); err != nil {
		respondError(w, http.StatusInternalServerError, "排序歌单歌曲失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "歌单歌曲已排序",
	})
}

// MovePlaylistSong 移动歌单中单首歌曲的位置
// @Summary 移动歌单内歌曲位置
// @Description 将指定歌曲移到 after_song_id 之后（after_song_id 为 null 则移到第一位），不改变其余歌曲的相对顺序
// @Description 支持普通歌单及内置收藏／电台收藏歌单，不改变歌单身份或收藏成员。
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单 ID"
// @Param request body object{song_id=int64,after_song_id=int64} true "移动参数（after_song_id 为 null 或省略表示置顶）"
// @Success 200 {object} map[string]string "移动成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "移动失败"
// @Security BearerAuth
// @Router /playlists/{id}/songs/move [put]
func (h *PlaylistHandler) MovePlaylistSong(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	playlistID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单 ID", err)
		return
	}

	var req struct {
		SongID      int64  `json:"song_id"`
		AfterSongID *int64 `json:"after_song_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if req.SongID == 0 {
		respondError(w, http.StatusBadRequest, "请提供 song_id", nil)
		return
	}

	if err := h.playlistService.MoveSong(ctx, playlistID, req.SongID, req.AfterSongID); err != nil {
		respondError(w, http.StatusInternalServerError, "移动歌曲位置失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "歌曲位置已更新",
	})
}

// ReorderPlaylists 重新排序歌单列表
// @Summary 重新排序歌单列表
// @Description 重新排序歌单列表
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param request body object{playlist_ids=[]int64} true "歌单 ID 列表"
// @Success 200 {object} map[string]string "排序成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "排序失败"
// @Security BearerAuth
// @Router /playlists/reorder [put]
func (h *PlaylistHandler) ReorderPlaylists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		PlaylistIDs []int64 `json:"playlist_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	if len(req.PlaylistIDs) == 0 {
		respondError(w, http.StatusBadRequest, "请提供 playlist_ids", nil)
		return
	}

	if err := h.playlistService.ReorderPlaylists(ctx, req.PlaylistIDs); err != nil {
		respondError(w, http.StatusInternalServerError, "重新排序歌单失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "歌单已重新排序",
	})
}

// UploadPlaylistCover 上传歌单封面图片
// @Summary 上传歌单封面
// @Description 上传本地图片作为歌单封面
// @Tags 歌单管理
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "歌单ID"
// @Param file formData file true "封面图片文件"
// @Success 200 {object} models.Playlist "上传成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "上传失败"
// @Security BearerAuth
// @Router /playlists/{id}/cover [post]
func (h *PlaylistHandler) UploadPlaylistCover(w http.ResponseWriter, r *http.Request) {
	// 1. 解析歌单 ID
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单ID", err)
		return
	}

	// 2. 解析 multipart form-data（限制 10MB）
	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		respondError(w, http.StatusBadRequest, "解析表单数据失败", err)
		return
	}

	// 3. 获取上传文件
	file, handler, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "获取上传文件失败", err)
		return
	}
	defer file.Close()

	// 4. 验证文件格式
	ext := strings.ToLower(filepath.Ext(handler.Filename))
	allowedExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true,
		".gif": true, ".bmp": true, ".webp": true,
	}
	if !allowedExts[ext] {
		respondError(w, http.StatusBadRequest, "不支持的图片格式，仅支持 jpg, jpeg, png, gif, bmp, webp", nil)
		return
	}

	// 5. 读取文件内容
	coverData, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "读取文件失败", err)
		return
	}

	// 6. 调用 Service 层保存封面
	// ext 去掉前面的点号
	coverExt := strings.TrimPrefix(ext, ".")
	playlist, err := h.playlistService.UploadCover(r.Context(), id, coverData, coverExt)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "上传封面失败", err)
		return
	}

	// 7. 返回更新后的歌单
	respondJSON(w, http.StatusOK, playlist)
}

// GetPlaylistCover 获取歌单封面
// @Summary 获取歌单封面
// @Description 无独立封面时回退到前 20 个有封面的成员候选，支持远程与插件相对地址；远程回退共用 5 秒预算，动态成员封面要求缓存重新验证。
// @Description 返回歌单封面图片文件。可选 query 参数 w：把本地封面等比缩放到该宽度（物理像素，绝不放大、上限 1024）后以 JPEG 返回，用于 Web 端降低 GPU 纹理体积（songloft-org/songloft#309）；缺省或非法时返回原图。缩略仅作用于本地封面，远程代理封面忽略 w。
// @Tags 歌单管理
// @Produce image/jpeg
// @Param id path int true "歌单ID"
// @Param w query int false "本地封面缩略目标宽度（物理像素，绝不放大，上限 1024）"
// @Success 200 {file} binary "封面图片"
// @Failure 404 {object} map[string]string "封面不存在"
// @Failure 500 {object} map[string]string "读取失败"
// @Security BearerAuth
// @Router /playlists/{id}/cover [get]
func (h *PlaylistHandler) GetPlaylistCover(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的 ID", err)
		return
	}

	// 获取歌单信息
	playlist, err := h.playlistService.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "歌单不存在", err)
		return
	}

	// 优先使用本地封面。文件缺失时（如源歌曲被移除、封面被引用计数清理）
	// 不直接 404，而是继续回退到远程 URL / 歌曲封面。
	if playlist.CoverPath != "" && coverFileExists(playlist.CoverPath) {
		h.serveLocalCover(w, r, playlist)
		return
	}

	remoteCtx, cancel := context.WithTimeout(r.Context(), songCoverProxyTimeout)
	defer cancel()
	remoteRequest := r.WithContext(remoteCtx)
	if playlist.CoverURL != "" && h.tryRemoteCover(w, remoteRequest, playlist.CoverURL) {
		return
	}

	// Dynamic member covers must be revalidated even if only a song's artwork changed.
	// A different candidate may have an older mtime, so date validators from the
	// previous candidate cannot identify the current representation. Thumb ETags
	// include the file identity and remain safe to use.
	r = r.Clone(r.Context())
	r.Header.Del("If-Modified-Since")
	w = revalidatingCoverWriter{w}
	songs, err := h.playlistService.GetCoverCandidates(r.Context(), id)
	if err == nil {
		for _, s := range songs {
			if s.CoverPath != "" && coverFileExists(s.CoverPath) {
				h.serveLocalCover(w, r, &models.Playlist{CoverPath: s.CoverPath})
				return
			}
			if s.CoverURL != "" && remoteCtx.Err() == nil && h.tryRemoteCover(w, remoteRequest, s.CoverURL) {
				return
			}
		}
	}

	respondError(w, http.StatusNotFound, "封面不存在", nil)
}

func (h *PlaylistHandler) tryRemoteCover(w http.ResponseWriter, r *http.Request, rawURL string) bool {
	return tryServeRemoteResource(w, r, h.urlResolver.Resolve(rawURL), RemoteResourceOptions{
		Timeout: songCoverProxyTimeout, AllowFallback: true,
	})
}

type revalidatingCoverWriter struct{ http.ResponseWriter }

func (w revalidatingCoverWriter) WriteHeader(status int) {
	w.Header().Set("Cache-Control", "no-cache")
	w.ResponseWriter.WriteHeader(status)
}
func (w revalidatingCoverWriter) Write(b []byte) (int, error) {
	w.Header().Set("Cache-Control", "no-cache")
	return w.ResponseWriter.Write(b)
}

// serveLocalCover 返回本地封面文件（支持 ?w= 服务端缩略，见 serveCoverFile）。
func (h *PlaylistHandler) serveLocalCover(w http.ResponseWriter, r *http.Request, playlist *models.Playlist) {
	serveCoverFile(w, r, playlist.CoverPath, h.thumbCache)
}

// coverFileExists 判断封面文件是否存在（非目录），供读路径在封面丢失时回退。
func coverFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// SetPlaylistVisibility 设置歌单可见性
// @Summary 设置歌单可见性
// @Description 切换歌单的隐藏状态。内置歌单（收藏、电台收藏）不允许隐藏
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Param request body models.SetPlaylistVisibilityRequest true "可见性设置"
// @Success 200 {object} models.Playlist "更新后的歌单"
// @Failure 400 {object} map[string]string "请求错误或内置歌单不可隐藏"
// @Failure 404 {object} map[string]string "歌单不存在"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /playlists/{id}/visibility [put]
func (h *PlaylistHandler) SetPlaylistVisibility(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单ID", err)
		return
	}

	var req models.SetPlaylistVisibilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	playlist, err := h.playlistService.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			respondError(w, http.StatusNotFound, "歌单不存在", err)
			return
		}
		respondError(w, http.StatusInternalServerError, "获取歌单失败", err)
		return
	}

	if playlist.IsBuiltIn() && req.Hidden {
		respondError(w, http.StatusBadRequest, "内置歌单不可隐藏", nil)
		return
	}

	if req.Hidden {
		if !playlist.HasLabel(models.PlaylistLabelHidden) {
			playlist.Labels = append(playlist.Labels, models.PlaylistLabelHidden)
		}
	} else {
		filtered := make([]string, 0, len(playlist.Labels))
		for _, l := range playlist.Labels {
			if l != models.PlaylistLabelHidden {
				filtered = append(filtered, l)
			}
		}
		playlist.Labels = filtered
	}

	if err := h.playlistService.Update(ctx, playlist); err != nil {
		respondError(w, http.StatusInternalServerError, "更新歌单可见性失败", err)
		return
	}

	updated, err := h.playlistService.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取更新后歌单失败", err)
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

// SetPlaylistPinned 设置歌单置顶状态
// @Summary 设置歌单置顶状态
// @Description 置顶/取消置顶歌单。置顶歌单在歌单列表中始终排在最前，多个置顶歌单按置顶时间倒序（最近置顶的排最前）。内置歌单（收藏、电台收藏）同样允许置顶，不做特殊限制。
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Param request body models.SetPlaylistPinnedRequest true "置顶设置"
// @Success 200 {object} models.Playlist "更新后的歌单"
// @Failure 400 {object} map[string]string "无效的歌单ID或请求数据"
// @Failure 404 {object} map[string]string "歌单不存在"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /playlists/{id}/pin [put]
func (h *PlaylistHandler) SetPlaylistPinned(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单ID", err)
		return
	}

	var req models.SetPlaylistPinnedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	playlist, err := h.playlistService.SetPinned(r.Context(), id, req.Pinned)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			respondError(w, http.StatusNotFound, "歌单不存在", err)
			return
		}
		respondError(w, http.StatusInternalServerError, "更新歌单置顶状态失败", err)
		return
	}

	respondJSON(w, http.StatusOK, playlist)
}
