package handlers

import (
	"net/http"
)

// HealthHandler 健康检查处理器
type HealthHandler struct{}

// NewHealthHandler 创建健康检查处理器
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// CheckHealth 检查应用健康状态
// @Summary 检查应用健康状态
// @Description 检查应用是否正常运行
// @Tags 系统管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string "成功返回健康状态"
// @Router /health [get]
func (h *HealthHandler) CheckHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
