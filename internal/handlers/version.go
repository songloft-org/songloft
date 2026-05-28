package handlers

import (
	"net/http"

	"songloft/internal/version"
)

// VersionHandler 版本处理器
type VersionHandler struct{}

// NewVersionHandler 创建版本处理器
func NewVersionHandler() *VersionHandler {
	return &VersionHandler{}
}

// GetVersion 获取应用版本信息
// @Summary 获取应用版本信息
// @Description 获取应用的版本信息，包括版本号、Git提交哈希和构建时间
// @Tags 系统管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string "成功返回版本信息"
// @Router /version [get]
func (h *VersionHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	versionInfo := map[string]string{
		"version":    version.GetVersion(),
		"full":       version.GetFullVersion(),
		"git_commit": version.GitCommit,
		"build_time": version.BuildTime,
	}

	respondJSON(w, http.StatusOK, versionInfo)
}
