package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"songloft/internal/database"
	"songloft/internal/models"
	"songloft/internal/services"

	"github.com/hanxi/tag"
)

// SongTagHandler 自定义标签处理器
type SongTagHandler struct {
	tagService    *services.SongTagService
	songService   *services.SongService
	configService *services.ConfigService
}

// GetPlaylistTags 获取歌单的自定义标签
// @Summary 获取歌单自定义标签
// @Description 返回共享标签定义，不包含 built_in、hidden 等系统标记；歌单不存在返回 404。
// @Tags 标签管理
// @Produce json
// @Param id path int true "歌单 ID"
// @Success 200 {array} models.SongTag "自定义标签"
// @Failure 400 {object} map[string]string "无效的 ID"
// @Failure 404 {object} map[string]string "歌单不存在"
// @Security BearerAuth
// @Router /playlists/{id}/tags [get]
func (h *SongTagHandler) GetPlaylistTags(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, 400, "无效的歌单 ID", err)
		return
	}
	tags, err := h.tagService.GetPlaylistTags(r.Context(), id)
	if errors.Is(err, database.ErrNotFound) {
		respondError(w, 404, "歌单不存在", err)
		return
	}
	if err != nil {
		respondError(w, 500, "获取歌单标签失败", err)
		return
	}
	respondJSON(w, 200, tags)
}

type PlaylistTagsRequest struct {
	TagIDs *[]int64 `json:"tag_ids"`
}

// SetPlaylistTags 设置歌单的自定义标签
// @Summary 设置歌单自定义标签
// @Description 原子替换自定义标签，保留系统标记。tag_ids 必填，空数组清空；无效标签整体失败。
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param id path int true "歌单 ID"
// @Param request body PlaylistTagsRequest true "标签 ID"
// @Success 200 {array} models.SongTag "更新后的标签"
// @Failure 400 {object} map[string]string "请求或标签无效"
// @Failure 404 {object} map[string]string "歌单或标签不存在"
// @Security BearerAuth
// @Router /playlists/{id}/tags [put]
func (h *SongTagHandler) SetPlaylistTags(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, 400, "无效的歌单 ID", err)
		return
	}
	var req PlaylistTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TagIDs == nil {
		respondError(w, 400, "tag_ids 必须是数组", err)
		return
	}
	for _, tagID := range *req.TagIDs {
		if tagID <= 0 {
			respondError(w, 400, "无效的标签 ID", nil)
			return
		}
	}
	if err := h.tagService.SetPlaylistTags(r.Context(), id, *req.TagIDs); err != nil {
		status := 500
		if errors.Is(err, database.ErrNotFound) {
			status = 404
		}
		respondError(w, status, "设置歌单标签失败", err)
		return
	}
	tags, err := h.tagService.GetPlaylistTags(r.Context(), id)
	if err != nil {
		respondError(w, 500, "获取歌单标签失败", err)
		return
	}
	if tags == nil {
		tags = []models.SongTag{}
	}
	respondJSON(w, 200, tags)
}

// NewSongTagHandler 创建标签处理器
func NewSongTagHandler(
	tagService *services.SongTagService,
	songService *services.SongService,
	configService *services.ConfigService,
) *SongTagHandler {
	return &SongTagHandler{
		tagService:    tagService,
		songService:   songService,
		configService: configService,
	}
}

// List 列出所有标签
// @Summary 列出所有标签
// @Description 返回标签列表，含每个标签下的歌曲数量、可见歌单数量和歌曲代表封面。支持关键词搜索、排序和分页。
// @Tags 标签管理
// @Produce json
// @Param keyword query string false "搜索关键词（模糊匹配标签名）"
// @Param sort query string false "排序字段" Enums(id, song_count, playlist_count, name, created_at) default(song_count)
// @Param order query string false "排序方向" Enums(asc, desc) default(desc)
// @Param limit query int false "每页数量" default(60)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]any "成功返回标签列表 {tags, total, limit, offset}"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /song-tags [get]
func (h *SongTagHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	keyword := q.Get("keyword")
	sort := q.Get("sort")
	order := q.Get("order")
	limit, offset := parsePagination(r, 60, 0)

	ctx := r.Context()
	tags, err := h.tagService.List(ctx, keyword, sort, order, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取标签列表失败", err)
		return
	}
	total, err := h.tagService.Count(ctx, keyword)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取标签总数失败", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"tags":   tags,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Create 创建标签
// @Summary 创建标签
// @Description 创建一个新的自定义标签。标签名不能为空、不能包含逗号、不能超过50个字符。标签名大小写不敏感去重。
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param request body object true "创建参数 {name: string, color?: string}"
// @Success 201 {object} models.SongTag "创建成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /song-tags [post]
func (h *SongTagHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	tag, err := h.tagService.Create(r.Context(), req.Name, req.Color)
	if err != nil {
		respondError(w, http.StatusBadRequest, "创建标签失败", err)
		return
	}
	respondJSON(w, http.StatusCreated, tag)
}

// Get 获取标签详情
// @Summary 获取标签详情
// @Description 返回单个标签的详细信息，包含歌曲数量。
// @Tags 标签管理
// @Produce json
// @Param id path int true "标签 ID"
// @Success 200 {object} models.SongTag "成功"
// @Failure 404 {object} map[string]string "标签不存在"
// @Security BearerAuth
// @Router /song-tags/{id} [get]
func (h *SongTagHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的标签 ID", err)
		return
	}
	tag, err := h.tagService.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			respondError(w, http.StatusNotFound, "标签不存在", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "获取标签失败", err)
		return
	}
	respondJSON(w, http.StatusOK, tag)
}

// Update 更新标签
// @Summary 更新标签
// @Description 更新标签名称或颜色。
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param id path int true "标签 ID"
// @Param request body object true "更新参数 {name?: string, color?: string}"
// @Success 204 "更新成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 404 {object} map[string]string "标签不存在"
// @Security BearerAuth
// @Router /song-tags/{id} [put]
func (h *SongTagHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的标签 ID", err)
		return
	}
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	if err := h.tagService.Update(r.Context(), id, req.Name, req.Color); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			respondError(w, http.StatusNotFound, "标签不存在", nil)
			return
		}
		respondError(w, http.StatusBadRequest, "更新标签失败", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete 删除标签
// @Summary 删除标签
// @Description 删除标签，自动解除所有歌曲关联（CASCADE）。歌曲本身不会被删除。
// @Tags 标签管理
// @Produce json
// @Param id path int true "标签 ID"
// @Success 204 "删除成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Security BearerAuth
// @Router /song-tags/{id} [delete]
func (h *SongTagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的标签 ID", err)
		return
	}
	if err := h.tagService.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "删除标签失败", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListSongs 获取标签下的歌曲列表
// @Summary 获取标签下的歌曲列表
// @Description 返回某个标签关联的歌曲列表，支持分页。
// @Tags 标签管理
// @Produce json
// @Param id path int true "标签 ID"
// @Param limit query int false "每页数量" default(100)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]any "成功返回 {songs, total, limit, offset}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /song-tags/{id}/songs [get]
func (h *SongTagHandler) ListSongs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的标签 ID", err)
		return
	}
	limit, offset := parsePagination(r, 100, 0)

	ctx := r.Context()
	songIDs, err := h.tagService.ListSongIDs(ctx, id, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌曲列表失败", err)
		return
	}
	total, err := h.tagService.CountSongs(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌曲总数失败", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"song_ids": songIDs,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// ListSongIDs 获取标签下所有歌曲 ID
// @Summary 获取标签下所有歌曲 ID
// @Description 返回某个标签关联的所有歌曲 ID 列表，不分页，用于播放全部场景。
// @Tags 标签管理
// @Produce json
// @Param id path int true "标签 ID"
// @Success 200 {array} int "歌曲 ID 列表"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /song-tags/{id}/song-ids [get]
func (h *SongTagHandler) ListSongIDs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的标签 ID", err)
		return
	}
	songIDs, err := h.tagService.ListSongIDs(r.Context(), id, 0, 0)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌曲 ID 列表失败", err)
		return
	}
	respondJSON(w, http.StatusOK, songIDs)
}

// GetSongTags 获取歌曲的自定义标签
// @Summary 获取歌曲的自定义标签
// @Description 返回某首歌曲关联的所有自定义标签列表。
// @Tags 标签管理
// @Produce json
// @Param id path int true "歌曲 ID"
// @Success 200 {array} models.SongTag "标签列表"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/{id}/song-tags [get]
func (h *SongTagHandler) GetSongTags(w http.ResponseWriter, r *http.Request) {
	songID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌曲 ID", err)
		return
	}
	tags, err := h.tagService.GetSongTags(r.Context(), songID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取歌曲标签失败", err)
		return
	}
	respondJSON(w, http.StatusOK, tags)
}

// SetSongTags 设置歌曲的标签
// @Summary 设置歌曲的标签
// @Description 全量替换歌曲的自定义标签关联。传入 tag_ids 数组，替换当前所有关联。
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param id path int true "歌曲 ID"
// @Param request body object true "标签 ID 列表 {tag_ids: [1,2,3]}"
// @Success 204 "设置成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /songs/{id}/song-tags [put]
func (h *SongTagHandler) SetSongTags(w http.ResponseWriter, r *http.Request) {
	songID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌曲 ID", err)
		return
	}
	var req struct {
		TagIDs []int64 `json:"tag_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	if err := h.tagService.SetSongTags(r.Context(), songID, req.TagIDs); err != nil {
		respondError(w, http.StatusInternalServerError, "设置歌曲标签失败", err)
		return
	}
	go h.syncTagsToFile(songID)
	w.WriteHeader(http.StatusNoContent)
}

// syncTagsToFile 将歌曲的自定义标签同步写入音频文件的 SONGLOFT_TAGS 字段。
// 仅当配置 tag_sync_to_file 开启时执行。
func (h *SongTagHandler) syncTagsToFile(songID int64) {
	if !h.configService.GetBool("tag_sync_to_file", false) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	song, err := h.songService.GetByID(ctx, songID)
	if err != nil || song.FilePath == "" {
		return
	}
	tags, err := h.tagService.GetSongTags(ctx, songID)
	if err != nil {
		return
	}
	var names []string
	for _, t := range tags {
		names = append(names, t.Name)
	}
	tagsValue := strings.Join(names, ",")

	opts := tag.WriteOptions{
		Title:      song.Title,
		Artist:     song.Artist,
		Album:      song.Album,
		Genre:      song.Genre,
		Language:   song.Language,
		Style:      song.Style,
		Track:      song.Track,
		CustomTags: map[string]string{"SONGLOFT_TAGS": tagsValue},
	}
	if song.Year > 0 {
		opts.Year = song.Year
	}
	if err := tag.WriteTag(song.FilePath, opts); err != nil {
		slog.Debug("sync tags to file failed", "songID", songID, "error", err)
	}
}

// BatchBind 批量绑定歌曲到标签
// @Summary 批量绑定歌曲到标签
// @Description 将多首歌曲关联到指定标签。已存在的关联会被忽略。
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param id path int true "标签 ID"
// @Param request body object true "歌曲 ID 列表 {song_ids: [1,2,3]}"
// @Success 200 {object} map[string]int "绑定结果 {bound: N}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /song-tags/{id}/bind [post]
func (h *SongTagHandler) BatchBind(w http.ResponseWriter, r *http.Request) {
	tagID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的标签 ID", err)
		return
	}
	var req struct {
		SongIDs []int64 `json:"song_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	bound, err := h.tagService.BatchBind(r.Context(), tagID, req.SongIDs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "批量绑定失败", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]int{"bound": bound})
}

// BatchUnbind 批量解绑歌曲
// @Summary 批量解绑歌曲
// @Description 将多首歌曲从指定标签中移除关联。
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param id path int true "标签 ID"
// @Param request body object true "歌曲 ID 列表 {song_ids: [1,2,3]}"
// @Success 200 {object} map[string]int "解绑结果 {unbound: N}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /song-tags/{id}/unbind [post]
func (h *SongTagHandler) BatchUnbind(w http.ResponseWriter, r *http.Request) {
	tagID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的标签 ID", err)
		return
	}
	var req struct {
		SongIDs []int64 `json:"song_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	unbound, err := h.tagService.BatchUnbind(r.Context(), tagID, req.SongIDs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "批量解绑失败", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]int{"unbound": unbound})
}

const tagSyncToFileConfigKey = "tag_sync_to_file"

// GetTagSyncToFile 获取标签同步到文件开关
// @Summary 获取标签同步到文件开关
// @Description 获取“自定义标签同步写入音频文件 SONGLOFT_TAGS 字段”开关的当前状态。开启后，每次修改歌曲标签时会同步写入音频文件元数据。默认关闭。
// @Tags 歌曲管理
// @Produce json
// @Success 200 {object} map[string]bool "开关状态 {enabled: bool}"
// @Security BearerAuth
// @Router /settings/tag-sync-to-file [get]
func (h *SongTagHandler) GetTagSyncToFile(w http.ResponseWriter, r *http.Request) {
	enabled := h.configService.GetBool(tagSyncToFileConfigKey, false)
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}

// UpdateTagSyncToFile 保存标签同步到文件开关
// @Summary 保存标签同步到文件开关
// @Description 开启/关闭“自定义标签同步写入音频文件”功能。开启后，对歌曲执行 SetSongTags 时会将标签列表写入音频文件的 SONGLOFT_TAGS 自定义字段。
// @Tags 歌曲管理
// @Accept json
// @Produce json
// @Param request body object true "开关设置 {enabled: bool}"
// @Success 200 {object} map[string]bool "保存后的开关状态 {enabled: bool}"
// @Failure 400 {object} map[string]string "请求格式错误"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/tag-sync-to-file [put]
func (h *SongTagHandler) UpdateTagSyncToFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	val := "false"
	if req.Enabled {
		val = "true"
	}
	if err := h.configService.Set(tagSyncToFileConfigKey, val); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": req.Enabled})
}
