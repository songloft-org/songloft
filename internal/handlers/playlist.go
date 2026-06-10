package handlers

import (
	"encoding/json"
	"errors"
	"io"
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
}

// NewPlaylistHandler 创建歌单处理器
func NewPlaylistHandler(playlistService *services.PlaylistService) *PlaylistHandler {
	return &PlaylistHandler{
		playlistService: playlistService,
	}
}

// ListPlaylists 获取歌单列表
// @Summary 获取歌单列表
// @Description 获取歌单列表，支持按类型过滤和分页
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param type query string false "歌单类型" Enums(normal, radio)
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{} "成功返回歌单列表"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /playlists [get]
func (h *PlaylistHandler) ListPlaylists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	playlistType := r.URL.Query().Get("type")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := models.DefaultPaginationLimit
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	filter := &database.PlaylistFilter{
		Type:   playlistType,
		Limit:  limit,
		Offset: offset,
	}

	playlists, err := h.playlistService.List(ctx, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌单列表失败", err)
		return
	}

	// 获取歌单总数（使用相同的过滤条件，不含分页）
	countFilter := &database.PlaylistFilter{
		Type: filter.Type,
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
// @Description 更新歌单信息
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Param request body models.Playlist true "歌单信息"
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

	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		CoverPath   *string `json:"cover_path"`
		CoverURL    *string `json:"cover_url"`
	}
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
	existing.Description = req.Description
	if req.CoverPath != nil {
		existing.CoverPath = *req.CoverPath
	}
	if req.CoverURL != nil {
		existing.CoverURL = *req.CoverURL
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

// TouchPlaylist 更新歌单的最后播放时间
// @Summary 更新歌单最后播放时间
// @Description 仅更新歌单的 updated_at 字段，用于记录最后播放时间
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

// DeletePlaylist 删除歌单
// @Summary 删除歌单
// @Description 根据歌单ID删除歌单
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Success 200 {object} map[string]string "删除成功"
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

	if err := h.playlistService.Delete(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, "删除歌单失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "歌单已删除",
	})
}

// BatchDeletePlaylists 批量删除歌单
// @Summary 批量删除歌单
// @Description 根据歌单 ID 列表批量删除歌单，内置歌单会被跳过
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

	deleted, err := h.playlistService.BatchDelete(ctx, req.IDs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "批量删除歌单失败", err)
		return
	}

	respondJSON(w, http.StatusOK, models.BatchDeletePlaylistsResponse{
		Deleted: deleted,
	})
}

// GetPlaylistSongs 获取歌单中的歌曲
// @Summary 获取歌单中的歌曲
// @Description 获取指定歌单中的歌曲，支持分页
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单 ID"
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
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

	// 解析分页参数
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := models.DefaultPaginationLimit
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	// 获取歌曲列表
	songs, err := h.playlistService.GetSongs(ctx, id, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌单歌曲失败", err)
		return
	}

	// 获取歌曲总数
	total, err := h.playlistService.CountSongs(ctx, id)
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
// @Description 重新排序歌单中的歌曲
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
// @Description 返回歌单封面图片文件
// @Tags 歌单管理
// @Produce image/jpeg
// @Param id path int true "歌单ID"
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

	// 优先使用本地封面
	if playlist.CoverPath != "" {
		h.serveLocalCover(w, playlist)
		return
	}

	// 本地封面不存在时,代理转发外部 URL
	if playlist.CoverURL != "" {
		ServeRemoteResource(w, r, playlist.CoverURL)
		return
	}

	// fallback: get cover from first song with valid local CoverPath (issue #147)
	songs, err := h.playlistService.GetSongs(r.Context(), id, 20, 0)
	if err == nil {
		for _, s := range songs {
			if s.CoverPath != "" {
				if _, e := os.Stat(s.CoverPath); e == nil {
					// Format a fake playlist to reuse serveLocalCover
					fakePl := &models.Playlist{CoverPath: s.CoverPath}
					h.serveLocalCover(w, fakePl)
					return
				}
			}
		}
	}

	respondError(w, http.StatusNotFound, "封面不存在", nil)
}

// serveLocalCover 返回本地封面文件
func (h *PlaylistHandler) serveLocalCover(w http.ResponseWriter, playlist *models.Playlist) {
	coverPath := playlist.CoverPath

	// 检查封面文件是否存在
	if _, err := os.Stat(coverPath); os.IsNotExist(err) {
		respondError(w, http.StatusNotFound, "封面文件不存在", err)
		return
	}

	// 读取封面文件
	coverData, err := os.ReadFile(coverPath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "读取封面文件失败", err)
		return
	}

	// 根据文件扩展名设置 Content-Type
	ext := filepath.Ext(coverPath)
	contentType := "image/jpeg" // 默认
	switch ext {
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".bmp":
		contentType = "image/bmp"
	case ".webp":
		contentType = "image/webp"
	}

	// 返回封面图片
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000") // 缓存一年
	w.WriteHeader(http.StatusOK)
	w.Write(coverData)
}
