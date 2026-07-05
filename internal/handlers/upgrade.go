package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"songloft/internal/models"
	"songloft/internal/services"
	"songloft/internal/version"
)

// UpgradeHandler 升级处理器
type UpgradeHandler struct {
	upgradeService *services.UpgradeService
}

// NewUpgradeHandler 创建升级处理器
func NewUpgradeHandler(upgradeService *services.UpgradeService) *UpgradeHandler {
	return &UpgradeHandler{
		upgradeService: upgradeService,
	}
}

// GetVersions 获取可用版本信息
// @Summary 获取可用版本信息
// @Description 获取正式版和测试版的版本信息
// @Tags 系统升级
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功返回版本信息"
// @Failure 403 {object} models.ErrorResponse "非 Docker 环境不支持升级"
// @Failure 500 {object} models.ErrorResponse "获取版本信息失败"
// @Security BearerAuth
// @Router /upgrade/versions [get]
func (h *UpgradeHandler) GetVersions(w http.ResponseWriter, r *http.Request) {
	// 检查是否在 Docker 环境
	if !h.upgradeService.IsDockerEnvironment() {
		respondError(w, http.StatusForbidden, "升级功能仅在 Docker 环境下可用", nil)
		return
	}

	// 从查询参数获取 GitHub 代理前缀
	githubProxy := r.URL.Query().Get("github_proxy")

	// 获取正式版信息
	stableInfo, stableErr := h.upgradeService.FetchVersionInfo("stable", githubProxy)

	// 获取测试版信息
	devInfo, devErr := h.upgradeService.FetchVersionInfo("dev", githubProxy)

	// 构建响应
	response := map[string]interface{}{
		"current": map[string]string{
			"version":    version.GetVersion(),
			"git_commit": version.GitCommit,
			"build_time": version.BuildTime,
			"channel":    h.upgradeService.CurrentVersionType(),
			"build_type": h.upgradeService.CurrentBuildType(),
		},
	}

	if stableErr == nil {
		response["stable"] = stableInfo
	} else {
		response["stable"] = map[string]string{
			"error": stableErr.Error(),
		}
	}

	if devErr == nil {
		response["dev"] = devInfo
	} else {
		response["dev"] = map[string]string{
			"error": devErr.Error(),
		}
	}

	slog.Info("GetVersions", "response", response)

	respondJSON(w, http.StatusOK, response)
}

// CheckUpdate 检查是否有新版本
// @Summary 检查更新
// @Description 检查是否有可用的新版本
// @Tags 系统升级
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功返回更新检查结果"
// @Failure 403 {object} models.ErrorResponse "非 Docker 环境不支持升级"
// @Failure 500 {object} models.ErrorResponse "检查更新失败"
// @Security BearerAuth
// @Router /upgrade/check [get]
func (h *UpgradeHandler) CheckUpdate(w http.ResponseWriter, r *http.Request) {
	isDocker := h.upgradeService.IsDockerEnvironment()

	// 从查询参数获取 GitHub 代理前缀
	githubProxy := r.URL.Query().Get("github_proxy")

	// 检查更新
	updates, err := h.upgradeService.CheckForUpdates(githubProxy)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "检查更新失败", err)
		return
	}

	// 提取最新版本信息（优先 stable，其次 dev）
	latestVersion := ""
	releaseNotes := ""
	if stableUpdate, ok := updates["stable"]; ok {
		latestVersion = stableUpdate.Version
		releaseNotes = stableUpdate.ReleaseNotes
	} else if devUpdate, ok := updates["dev"]; ok {
		latestVersion = devUpdate.Version
		releaseNotes = devUpdate.ReleaseNotes
	}

	// 构建响应（同时提供嵌套和扁平字段，方便前端解析）
	response := map[string]interface{}{
		"is_docker":          isDocker,
		"has_update":         len(updates) > 0,
		"current_version":    version.GetVersion(),
		"current_channel":    h.upgradeService.CurrentVersionType(),
		"current_build_type": h.upgradeService.CurrentBuildType(),
		"latest_version":     latestVersion,
		"release_notes":      releaseNotes,
		"current": map[string]string{
			"version":    version.GetVersion(),
			"git_commit": version.GitCommit,
			"build_time": version.BuildTime,
			"channel":    h.upgradeService.CurrentVersionType(),
			"build_type": h.upgradeService.CurrentBuildType(),
		},
		"updates": updates,
	}

	respondJSON(w, http.StatusOK, response)
}

// StartUpgrade 开始升级
// @Summary 开始升级
// @Description 开始升级到指定版本
// @Tags 系统升级
// @Accept json
// @Produce json
// @Param request body map[string]string true "升级请求 {version_type: stable|dev}"
// @Success 200 {object} models.SuccessResponse "升级已开始"
// @Failure 400 {object} models.ErrorResponse "请求参数错误"
// @Failure 403 {object} models.ErrorResponse "非 Docker 环境不支持升级"
// @Failure 500 {object} models.ErrorResponse "升级失败"
// @Security BearerAuth
// @Router /upgrade/start [post]
func (h *UpgradeHandler) StartUpgrade(w http.ResponseWriter, r *http.Request) {
	// 检查是否在 Docker 环境
	if !h.upgradeService.IsDockerEnvironment() {
		respondError(w, http.StatusForbidden, "升级功能仅在 Docker 环境下可用", nil)
		return
	}

	// 解析请求
	var req struct {
		VersionType string `json:"version_type"`
		GithubProxy string `json:"github_proxy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求参数", err)
		return
	}

	// 验证版本类型
	if req.VersionType != "stable" && req.VersionType != "dev" {
		respondError(w, http.StatusBadRequest, "无效的版本类型，必须是 stable 或 dev", nil)
		return
	}

	if err := h.upgradeService.ValidateVersionTypeForUpgrade(req.VersionType); err != nil {
		respondError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	// 在后台执行升级
	go func() {
		if err := h.upgradeService.UpgradeBinary(req.VersionType, req.GithubProxy); err != nil {
			// 升级失败，错误信息已在 UpgradeProgress 中记录
		}
	}()

	respondJSON(w, http.StatusOK, models.SuccessResponse{
		Message: "升级已开始，请稍候...",
	})
}

// ResetToBaseImage 回退到底包版本
// @Summary 回退到底包版本
// @Description 将二进制文件回退到 Docker 镜像中的原始版本，然后重启服务
// @Tags 系统升级
// @Accept json
// @Produce json
// @Success 200 {object} models.SuccessResponse "回退已开始"
// @Failure 403 {object} models.ErrorResponse "非 Docker 环境不支持回退"
// @Security BearerAuth
// @Router /upgrade/reset [post]
func (h *UpgradeHandler) ResetToBaseImage(w http.ResponseWriter, r *http.Request) {
	// 检查是否在 Docker 环境
	if !h.upgradeService.IsDockerEnvironment() {
		respondError(w, http.StatusForbidden, "回退功能仅在 Docker 环境下可用", nil)
		return
	}

	// 在后台执行回退
	go func() {
		if err := h.upgradeService.ResetToBaseImage(); err != nil {
			slog.Error("ResetToBaseImage failed", "error", err)
		}
	}()

	respondJSON(w, http.StatusOK, models.SuccessResponse{
		Message: "回退已开始，服务即将重启...",
	})
}

// GetUpgradeProgress 获取升级进度
// @Summary 获取升级进度
// @Description 获取当前升级任务的进度信息
// @Tags 系统升级
// @Accept json
// @Produce json
// @Success 200 {object} models.UpgradeProgress "成功返回升级进度"
// @Failure 403 {object} models.ErrorResponse "非 Docker 环境不支持升级"
// @Security BearerAuth
// @Router /upgrade/progress [get]
func (h *UpgradeHandler) GetUpgradeProgress(w http.ResponseWriter, r *http.Request) {
	// 检查是否在 Docker 环境
	if !h.upgradeService.IsDockerEnvironment() {
		respondError(w, http.StatusForbidden, "升级功能仅在 Docker 环境下可用", nil)
		return
	}

	progress := h.upgradeService.GetProgress()
	respondJSON(w, http.StatusOK, progress)
}
