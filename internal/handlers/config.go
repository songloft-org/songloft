package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"mimusic/internal/database"
	"mimusic/internal/models"
	"mimusic/internal/services"

	"github.com/go-chi/chi/v5"
)

// ConfigHandler 配置处理器
type ConfigHandler struct {
	configService   *services.ConfigService
	onConfigChanged func(key string) // 配置变更回调（可选）
}

// NewConfigHandler 创建配置处理器
func NewConfigHandler(configService *services.ConfigService) *ConfigHandler {
	return &ConfigHandler{
		configService: configService,
	}
}

// SetOnConfigChanged 设置配置变更回调
func (h *ConfigHandler) SetOnConfigChanged(callback func(key string)) {
	h.onConfigChanged = callback
}

// ListConfigs 获取配置列表
// @Summary 获取配置列表
// @Description 获取配置列表，支持关键词搜索和分页
// @Tags 配置管理
// @Accept json
// @Produce json
// @Param keyword query string false "搜索关键词"
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{} "成功返回配置列表"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /configs [get]
func (h *ConfigHandler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 解析查询参数
	keyword := r.URL.Query().Get("keyword")
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

	// 构建过滤条件
	filter := &database.ConfigFilter{
		Keyword: keyword,
		Limit:   limit,
		Offset:  offset,
		OrderBy: "key",
		Order:   "ASC",
	}

	// 获取配置列表
	configs, err := h.configService.ListConfigs(ctx, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取配置列表失败", err)
		return
	}

	// 获取总数
	total, err := h.configService.CountConfigs(ctx, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取配置总数失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"configs": configs,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// GetConfig 获取单个配置
// @Summary 获取单个配置详情
// @Description 根据配置键获取详细信息
// @Tags 配置管理
// @Accept json
// @Produce json
// @Param key path string true "配置键"
// @Success 200 {object} models.Config "成功返回配置详情"
// @Failure 404 {object} map[string]string "配置不存在"
// @Security BearerAuth
// @Router /configs/{key} [get]
func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "key")

	if key == "" {
		respondError(w, http.StatusBadRequest, "配置键不能为空", nil)
		return
	}

	config, err := h.configService.GetConfig(ctx, key)
	if err != nil {
		respondError(w, http.StatusNotFound, "配置不存在", err)
		return
	}

	respondJSON(w, http.StatusOK, config)
}

// CreateConfig 创建配置
// @Summary 创建配置
// @Description 创建一个新的配置
// @Tags 配置管理
// @Accept json
// @Produce json
// @Param request body models.CreateConfigRequest true "配置信息"
// @Success 201 {object} models.Config "创建成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 500 {object} map[string]string "创建失败"
// @Security BearerAuth
// @Router /configs [post]
func (h *ConfigHandler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	// 验证请求数据
	if req.Key == "" {
		respondError(w, http.StatusBadRequest, "配置键不能为空", nil)
		return
	}
	if req.Value == "" {
		respondError(w, http.StatusBadRequest, "配置值不能为空", nil)
		return
	}

	// 创建配置
	config := &models.Config{
		Key:   req.Key,
		Value: req.Value,
	}

	if err := h.configService.CreateConfig(ctx, config); err != nil {
		respondError(w, http.StatusInternalServerError, "创建配置失败", err)
		return
	}

	respondJSON(w, http.StatusCreated, config)
}

// UpdateConfig 更新配置
// @Summary 更新配置
// @Description 更新配置信息
// @Tags 配置管理
// @Accept json
// @Produce json
// @Param key path string true "配置键"
// @Param request body models.UpdateConfigRequest true "配置信息"
// @Success 200 {object} models.Config "更新成功"
// @Failure 400 {object} map[string]string "请求数据错误"
// @Failure 404 {object} map[string]string "配置不存在"
// @Security BearerAuth
// @Router /configs/{key} [put]
func (h *ConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "key")

	if key == "" {
		respondError(w, http.StatusBadRequest, "配置键不能为空", nil)
		return
	}

	// 检查配置是否存在
	_, err := h.configService.GetConfig(ctx, key)
	if err != nil {
		respondError(w, http.StatusNotFound, "配置不存在", err)
		return
	}

	var req models.UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求数据", err)
		return
	}

	// 验证请求数据
	if req.Value == "" {
		respondError(w, http.StatusBadRequest, "配置值不能为空", nil)
		return
	}

	// 更新配置
	config := &models.Config{
		Key:   key,
		Value: req.Value,
	}

	if err := h.configService.UpdateConfig(ctx, config); err != nil {
		respondError(w, http.StatusInternalServerError, "更新配置失败", err)
		return
	}

	// 触发配置变更回调
	if h.onConfigChanged != nil {
		go h.onConfigChanged(key)
	}

	respondJSON(w, http.StatusOK, config)
}

// DeleteConfig 删除配置
// @Summary 删除配置
// @Description 根据配置键删除配置
// @Tags 配置管理
// @Accept json
// @Produce json
// @Param key path string true "配置键"
// @Success 200 {object} map[string]string "删除成功"
// @Failure 400 {object} map[string]string "无效的配置键"
// @Failure 500 {object} map[string]string "删除失败"
// @Security BearerAuth
// @Router /configs/{key} [delete]
func (h *ConfigHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "key")

	if key == "" {
		respondError(w, http.StatusBadRequest, "配置键不能为空", nil)
		return
	}

	if err := h.configService.DeleteConfig(ctx, key); err != nil {
		respondError(w, http.StatusInternalServerError, "删除配置失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "配置已删除",
	})
}
