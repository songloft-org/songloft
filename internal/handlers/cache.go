package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"mimusic/internal/services"
)

// CacheHandler 音乐缓存管理处理器。
//
// 注:播放 URL 端点已迁移到 SongHandler.GetSongURL (/songs/{id}/url)
type CacheHandler struct {
	cacheService  *services.CacheService
	configService *services.ConfigService
}

// AsyncReassigner 抽象 SourceOrchestrator.AsyncReassign(避免 handlers 依赖 source 包)
type AsyncReassigner interface {
	AsyncReassign(songID int64)
}

// NewCacheHandler 创建缓存管理处理器。
func NewCacheHandler(
	cacheService *services.CacheService,
	configService *services.ConfigService,
) *CacheHandler {
	return &CacheHandler{
		cacheService:  cacheService,
		configService: configService,
	}
}

// HandleGetCacheStats 获取缓存统计信息
// @Summary 获取缓存统计信息
// @Description 获取服务端音乐缓存的统计信息,包括总大小、文件数量和最大缓存限制
// @Tags 缓存管理
// @Accept json
// @Produce json
// @Success 200 {object} services.CacheStats "缓存统计信息"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /cache-manage/stats [get]
func (h *CacheHandler) HandleGetCacheStats(w http.ResponseWriter, r *http.Request) {
	stats := h.cacheService.GetCacheStats()
	respondJSON(w, http.StatusOK, stats)
}

// HandleCleanCache 清理全部缓存
// @Summary 清理全部音乐缓存
// @Description 删除服务端所有已缓存的音乐文件,清理后需要重新下载
// @Tags 缓存管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string "清理成功"
// @Failure 500 {object} map[string]string "清理失败"
// @Security BearerAuth
// @Router /cache-manage/clean [post]
func (h *CacheHandler) HandleCleanCache(w http.ResponseWriter, r *http.Request) {
	if err := h.cacheService.CleanCache(); err != nil {
		slog.Error("清理缓存失败", "error", err)
		respondError(w, http.StatusInternalServerError, "清理缓存失败", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "缓存已清理"})
}

// HandleGetCacheConfig 获取缓存配置
// @Summary 获取缓存配置
// @Description 获取服务端音乐缓存的配置信息,包括最大缓存大小限制
// @Tags 缓存管理
// @Accept json
// @Produce json
// @Success 200 {object} services.CacheConfig "缓存配置"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /cache-manage/config [get]
func (h *CacheHandler) HandleGetCacheConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.cacheService.GetCacheConfig()
	respondJSON(w, http.StatusOK, cfg)
}

// HandleUpdateCacheConfig 更新缓存配置
// @Summary 更新缓存配置
// @Description 更新服务端音乐缓存的配置,如最大缓存大小。更新后会自动触发 LRU 淘汰检查。
// @Tags 缓存管理
// @Accept json
// @Produce json
// @Param request body services.CacheConfig true "缓存配置"
// @Success 200 {object} services.CacheConfig "更新后的缓存配置"
// @Failure 400 {object} map[string]string "请求参数无效"
// @Failure 500 {object} map[string]string "更新失败"
// @Security BearerAuth
// @Router /cache-manage/config [put]
func (h *CacheHandler) HandleUpdateCacheConfig(w http.ResponseWriter, r *http.Request) {
	var cfg services.CacheConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		respondError(w, http.StatusBadRequest, "请求参数无效", err)
		return
	}

	if cfg.MaxSize < 0 {
		respondError(w, http.StatusBadRequest, "最大缓存大小不能为负数", nil)
		return
	}

	if err := h.cacheService.UpdateCacheConfig(cfg); err != nil {
		slog.Error("更新缓存配置失败", "error", err)
		respondError(w, http.StatusInternalServerError, "更新缓存配置失败", err)
		return
	}

	respondJSON(w, http.StatusOK, cfg)
}
