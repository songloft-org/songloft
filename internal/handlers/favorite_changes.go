package handlers

import "net/http"

// FavoriteChanges 长轮询内置收藏变更。
// @Summary 等待收藏成员变更
// @Description 首次不传 cursor 或服务端重启后立即返回 changed=true，客户端刷新歌曲和电台收藏缓存。传入上次 cursor 后最多等待 25 秒；成员写入提交后立即唤醒所有等待者，无变更超时返回 changed=false。cursor 不透明，仅用于缓存失效，不是可重放事件日志。需通过正常认证，断开连接取消等待。
// @Tags 歌单管理
// @Produce json
// @Param cursor query string false "上次返回的不透明游标"
// @Success 200 {object} services.FavoriteChangesResponse "收藏缓存失效通知或等待超时"
// @Failure 401 {object} map[string]string "未认证或凭证失效"
// @Failure 503 {object} map[string]string "服务端正在关闭"
// @Security BearerAuth
// @Router /playlists/favorites/changes [get]
func (h *PlaylistHandler) FavoriteChanges(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.playlistService.FavoriteChanges(r.Context(), r.URL.Query().Get("cursor"))
	if err != nil {
		if r.Context().Err() == nil {
			respondError(w, http.StatusServiceUnavailable, "收藏变更通知已关闭", err)
		}
		return
	}
	respondJSON(w, http.StatusOK, result)
}
