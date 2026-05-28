package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"songloft/internal/services"

	"github.com/go-chi/chi/v5"
)

// ConvertHandler 网络歌曲→本地歌曲转换处理器
type ConvertHandler struct {
	convertService *services.ConvertService
}

// NewConvertHandler 创建转换处理器
func NewConvertHandler(convertService *services.ConvertService) *ConvertHandler {
	return &ConvertHandler{convertService: convertService}
}

// AutoConvertRequest 自动转换开关请求
type AutoConvertRequest struct {
	Enabled bool `json:"enabled" example:"true"` // 是否启用自动转换
}

// ConvertPlaylist 启动整歌单的网络歌曲→本地歌曲转换
// @Summary 启动歌单转换为本地歌单
// @Description 异步将歌单内所有网络歌曲下载到本地音乐库（按 music_path/{歌单名}/{艺术家 - 标题} 命名），立即返回，可通过进度接口查询状态。同一歌单同时只允许一个任务运行。
// @Tags 歌单转换
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Success 202 {object} map[string]interface{} "转换任务已启动"
// @Failure 400 {object} map[string]string "无效的歌单ID 或 歌单中没有需要转换的网络歌曲"
// @Failure 409 {object} map[string]string "该歌单已有转换任务在运行"
// @Failure 500 {object} map[string]string "启动转换失败"
// @Security BearerAuth
// @Router /playlists/{id}/convert-to-local [post]
func (h *ConvertHandler) ConvertPlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单 ID", err)
		return
	}

	if err := h.convertService.ConvertPlaylistToLocal(r.Context(), playlistID); err != nil {
		if errors.Is(err, services.ErrAlreadyRunning) {
			respondError(w, http.StatusConflict, "该歌单已有转换任务在运行", err)
			return
		}
		if errors.Is(err, services.ErrNoRemoteSongs) {
			respondError(w, http.StatusBadRequest, "该歌单没有需要转换的网络歌曲", err)
			return
		}
		respondError(w, http.StatusInternalServerError, "启动转换失败", err)
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"message":     "转换任务已启动",
		"playlist_id": playlistID,
	})
}

// GetConvertProgress 获取歌单的转换进度
// @Summary 获取歌单转换进度
// @Description 获取指定歌单的网络歌曲→本地歌曲转换进度,无运行任务时返回 idle 状态
// @Tags 歌单转换
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Success 200 {object} services.ConvertProgress "转换进度信息"
// @Failure 400 {object} map[string]string "无效的歌单ID"
// @Security BearerAuth
// @Router /playlists/{id}/convert-progress [get]
func (h *ConvertHandler) GetConvertProgress(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单 ID", err)
		return
	}
	progress := h.convertService.GetProgress(playlistID)
	respondJSON(w, http.StatusOK, progress)
}

// CancelConvert 取消歌单转换
// @Summary 取消歌单转换
// @Description 取消指定歌单的网络歌曲→本地歌曲转换任务,返回 cancelled 字段表示是否成功取消
// @Tags 歌单转换
// @Accept json
// @Produce json
// @Param id path int true "歌单ID"
// @Success 200 {object} map[string]bool "取消结果,cancelled 为 true 表示成功取消"
// @Failure 400 {object} map[string]string "无效的歌单ID"
// @Security BearerAuth
// @Router /playlists/{id}/convert-progress/cancel [post]
func (h *ConvertHandler) CancelConvert(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	playlistID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的歌单 ID", err)
		return
	}
	ok := h.convertService.CancelConvert(playlistID)
	respondJSON(w, http.StatusOK, map[string]bool{"cancelled": ok})
}

// GetAutoConvertSetting 获取自动转换开关状态
// @Summary 获取自动转换开关
// @Description 获取“网络歌曲缓存完成后自动转为本地”开关的当前状态
// @Tags 歌单转换
// @Accept json
// @Produce json
// @Success 200 {object} map[string]bool "返回 enabled 字段表示开关状态"
// @Security BearerAuth
// @Router /settings/auto-convert [get]
func (h *ConvertHandler) GetAutoConvertSetting(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]bool{
		"enabled": h.convertService.IsAutoConvertEnabled(),
	})
}

// UpdateAutoConvertSetting 更新自动转换开关
// @Summary 更新自动转换开关
// @Description 开启或关闭“网络歌曲缓存完成后自动转为本地”功能。开启后,任何 remote 类型歌曲缓存下载完成时,会异步在其所在的所有普通歌单中触发转换。
// @Tags 歌单转换
// @Accept json
// @Produce json
// @Param request body AutoConvertRequest true "开关请求"
// @Success 200 {object} map[string]bool "返回 enabled 字段表示更新后的开关状态"
// @Failure 400 {object} map[string]string "请求格式错误"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/auto-convert [put]
func (h *ConvertHandler) UpdateAutoConvertSetting(w http.ResponseWriter, r *http.Request) {
	var req AutoConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if err := h.convertService.SetAutoConvertEnabled(req.Enabled); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": req.Enabled})
}
