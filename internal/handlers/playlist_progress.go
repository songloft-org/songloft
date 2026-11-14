package handlers

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"net/http"
	"songloft/internal/database"
	"songloft/internal/models"
	"songloft/internal/services"
	"strconv"
)

type PlaylistProgressHandler struct {
	s *services.PlaylistProgressService
}

func NewPlaylistProgressHandler(s *services.PlaylistProgressService) *PlaylistProgressHandler {
	return &PlaylistProgressHandler{s}
}

// Handle 管理歌单收听位置。
// @Summary 读取、更新或清除歌单收听进度
// @Description 普通歌单共享歌曲、毫秒位置及模式；GET 无记录返回 version=0；PUT 使用 version 乐观锁，冲突返回 409。省略字段保留原值。DELETE 清除位置并保留模式、递增版本。歌曲移出歌单后 GET 返回 invalid=true。无需依赖播放历史。
// @Tags 歌单管理
// @Accept json
// @Produce json
// @Param id path int true "歌单 ID"
// @Param request body models.UpdatePlaylistProgress false "PUT 的状态及版本"
// @Success 200 {object} models.PlaylistProgress
// @Success 204 "已清除位置"
// @Failure 400 {object} models.ErrorResponse "非法位置、模式或非普通歌单"
// @Failure 404 {object} models.ErrorResponse "歌单不存在"
// @Failure 409 {object} models.ErrorResponse "其他设备已更新进度"
// @Failure 500 {object} models.ErrorResponse "保存失败"
// @Security BearerAuth
// @Router /playlists/{id}/playback-progress [get]
// @Router /playlists/{id}/playback-progress [put]
// @Router /playlists/{id}/playback-progress [delete]
func (h *PlaylistProgressHandler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, 400, "无效歌单 ID", err)
		return
	}
	var out *models.PlaylistProgress
	switch r.Method {
	case "GET":
		out, err = h.s.Get(r.Context(), id)
	case "PUT":
		var in models.UpdatePlaylistProgress
		if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
			respondError(w, 400, "无效请求", e)
			return
		}
		out, err = h.s.Save(r.Context(), id, in)
	case "DELETE":
		err = h.s.Clear(r.Context(), id)
	}
	if err != nil {
		code := 500
		if errors.Is(err, database.ErrNotFound) {
			code = 404
		}
		if errors.Is(err, services.ErrInvalidProgress) {
			code = 400
		}
		if errors.Is(err, database.ErrProgressConflict) {
			code = 409
		}
		respondError(w, code, "歌单播放状态操作失败", err)
		return
	}
	if r.Method == "DELETE" {
		w.WriteHeader(204)
		return
	}
	respondJSON(w, 200, out)
}
