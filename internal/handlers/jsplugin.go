package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"songloft/internal/jsplugin"
	"songloft/internal/models"
	"songloft/internal/services/source"

	"github.com/go-chi/chi/v5"
)

// jsPluginUploadResult 单个 JS 插件上传结果，字段与 Flutter JSPluginUploadResult 对齐
type jsPluginUploadResult struct {
	FileName string             `json:"file_name"`
	Plugin   *jsplugin.JSPlugin `json:"plugin,omitempty"`
	Error    string             `json:"error,omitempty"`
	Success  bool               `json:"success"`
}

// jsPluginUploadResponse 批量响应结构，字段与 Flutter JSPluginUploadResponse 对齐
type jsPluginUploadResponse struct {
	Total   int                    `json:"total"`
	Success int                    `json:"success"`
	Failed  int                    `json:"failed"`
	Results []jsPluginUploadResult `json:"results"`
	Message string                 `json:"message"`
}

// JSPluginHandler 处理 JS 插件管理 API
type JSPluginHandler struct {
	packageMgr    *jsplugin.PackageManager
	repo          jsplugin.Repository
	manager       *jsplugin.Manager
	sourceMetrics *source.SourceMetrics
}

// NewJSPluginHandler 创建 JS 插件管理处理器
func NewJSPluginHandler(packageMgr *jsplugin.PackageManager, repo jsplugin.Repository, manager *jsplugin.Manager, sourceMetrics *source.SourceMetrics) *JSPluginHandler {
	return &JSPluginHandler{
		packageMgr:    packageMgr,
		repo:          repo,
		manager:       manager,
		sourceMetrics: sourceMetrics,
	}
}

// RegisterRoutes 注册 JS 插件管理路由
func (h *JSPluginHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/jsplugins", func(r chi.Router) {
		r.Get("/", h.handleList)
		r.Post("/upload", h.handleUpload)
		r.Get("/{id}", h.handleGet)
		r.Put("/{id}", h.handleUpdate)
		r.Delete("/{id}", h.handleDelete)
		r.Post("/{id}/enable", h.handleEnable)
		r.Post("/{id}/disable", h.handleDisable)
		r.Get("/{id}/check-update", h.handleCheckUpdate)
		r.Post("/{id}/update", h.handleDownloadUpdate)
	})

	// 音源健康度 admin API(供前端展示插件成功率与失败原因,辅助排查)
	r.Get("/api/v1/plugins/health", h.handlePluginHealth)
}

// pluginHealthResponse 音源健康度响应
type pluginHealthResponse struct {
	Plugins []source.PluginHealthSnapshot `json:"plugins"`
}

// handlePluginHealth 返回各插件的下载成功率与最近失败原因
// @Summary 音源健康度
// @Description 返回各音乐源插件的下载成功率、健康度分类(green/yellow/red)与最近 5 条失败原因。
// @Tags JS插件管理
// @Produce json
// @Success 200 {object} pluginHealthResponse
// @Security BearerAuth
// @Router /plugins/health [get]
func (h *JSPluginHandler) handlePluginHealth(w http.ResponseWriter, r *http.Request) {
	_ = r
	const maxFailures = 5
	snap := h.sourceMetrics.Snapshot(maxFailures)
	if snap == nil {
		snap = []source.PluginHealthSnapshot{}
	}
	respondJSON(w, http.StatusOK, pluginHealthResponse{Plugins: snap})
}

// handleList 列出所有 JS 插件
// @Summary 列出所有 JS 插件
// @Description 获取 JS 插件列表
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "JS插件列表"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /jsplugins [get]
func (h *JSPluginHandler) handleList(w http.ResponseWriter, r *http.Request) {
	plugins, err := h.repo.GetAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取插件列表失败", err)
		return
	}

	if plugins == nil {
		plugins = []*models.JSPlugin{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"plugins": plugins,
	})
}

// handleUpload 上传安装新插件
// @Summary 上传安装 JS 插件
// @Description 上传新的 JS 插件文件（.jsplugin.zip 压缩包）
// @Tags JS插件管理
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "JS插件文件 (.jsplugin.zip)"
// @Success 201 {object} map[string]interface{} "上传成功"
// @Failure 400 {object} models.ErrorResponse "请求数据错误"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /jsplugins/upload [post]
func (h *JSPluginHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	// 限制上传大小 50MB
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	// 解析 multipart form
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "解析上传文件失败", err)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "获取上传文件失败", err)
		return
	}
	defer file.Close()

	fileName := ""
	if header != nil {
		fileName = header.Filename
	}

	zipData, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "读取上传文件失败", err)
		return
	}

	plugin, wasUpdate, err := h.packageMgr.InstallFromUpload(zipData)
	if err != nil {
		respondJSON(w, http.StatusOK, jsPluginUploadResponse{
			Total:   1,
			Success: 0,
			Failed:  1,
			Results: []jsPluginUploadResult{{
				FileName: fileName,
				Error:    err.Error(),
				Success:  false,
			}},
			Message: "安装插件失败",
		})
		return
	}

	// 覆盖更新成功后，若原插件处于活跃状态，热重载使变更立即生效
	if wasUpdate && plugin.Status == jsplugin.JSPluginStatusActive && h.manager != nil {
		if reloadErr := h.manager.ReloadPlugin(r.Context(), plugin.EntryPath); reloadErr != nil {
			slog.Warn("reload plugin after upload-update failed", "entryPath", plugin.EntryPath, "error", reloadErr)
		}
	}

	var (
		message string
		status  int
	)
	if wasUpdate {
		message = fmt.Sprintf("插件已更新到 v%s", plugin.Version)
		status = http.StatusOK
		slog.Info("js plugin updated via upload", "entryPath", plugin.EntryPath, "version", plugin.Version)
	} else {
		message = fmt.Sprintf("插件 %s 安装成功", plugin.EntryPath)
		status = http.StatusCreated
		slog.Info("js plugin uploaded", "entryPath", plugin.EntryPath, "version", plugin.Version)
	}

	respondJSON(w, status, jsPluginUploadResponse{
		Total:   1,
		Success: 1,
		Failed:  0,
		Results: []jsPluginUploadResult{{
			FileName: fileName,
			Plugin:   plugin,
			Success:  true,
		}},
		Message: message,
	})
}

// handleGet 获取单个插件详情
// @Summary 获取 JS 插件详情
// @Description 根据插件ID获取 JS 插件的详细信息
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Param id path int true "插件ID"
// @Success 200 {object} map[string]interface{} "JS插件信息"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 404 {object} models.ErrorResponse "插件不存在"
// @Security BearerAuth
// @Router /jsplugins/{id} [get]
func (h *JSPluginHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	pluginID, err := h.parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的插件ID", err)
		return
	}

	plugin, err := h.repo.GetByID(r.Context(), pluginID)
	if err != nil {
		respondError(w, http.StatusNotFound, "插件不存在", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"plugin": plugin,
	})
}

// handleUpdate 更新插件（上传新 ZIP）
// @Summary 更新 JS 插件
// @Description 上传新的 JS 插件文件以更新现有插件
// @Tags JS插件管理
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "插件ID"
// @Param file formData file true "JS插件文件 (.jsplugin.zip)"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} models.ErrorResponse "请求数据错误"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 404 {object} models.ErrorResponse "插件不存在"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /jsplugins/{id} [put]
func (h *JSPluginHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	pluginID, err := h.parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的插件ID", err)
		return
	}

	// 限制上传大小 50MB
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "解析上传文件失败", err)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "获取上传文件失败", err)
		return
	}
	defer file.Close()

	zipData, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "读取上传文件失败", err)
		return
	}

	plugin, err := h.packageMgr.Update(pluginID, zipData)
	if err != nil {
		respondError(w, http.StatusBadRequest, "更新插件失败", err)
		return
	}

	// 如果插件处于活跃状态，重载它
	if plugin.Status == jsplugin.JSPluginStatusActive && h.manager != nil {
		if reloadErr := h.manager.ReloadPlugin(r.Context(), plugin.EntryPath); reloadErr != nil {
			slog.Warn("reload plugin after update failed", "entryPath", plugin.EntryPath, "error", reloadErr)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"plugin": plugin,
	})
}

// handleDelete 删除插件
// @Summary 删除 JS 插件
// @Description 根据插件ID删除 JS 插件
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Param id path int true "插件ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 404 {object} models.ErrorResponse "插件不存在"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /jsplugins/{id} [delete]
func (h *JSPluginHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	pluginID, err := h.parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的插件ID", err)
		return
	}

	// 先检查插件是否存在，并卸载运行中的服务
	plugin, err := h.repo.GetByID(r.Context(), pluginID)
	if err != nil {
		respondError(w, http.StatusNotFound, "插件不存在", err)
		return
	}

	// 卸载运行中的服务
	if h.manager != nil {
		_ = h.manager.UnloadPlugin(r.Context(), plugin.EntryPath)
	}

	// 执行卸载
	if err := h.packageMgr.Uninstall(pluginID); err != nil {
		respondError(w, http.StatusInternalServerError, "删除插件失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "插件已删除",
	})
}

// handleEnable 启用插件
// @Summary 启用 JS 插件
// @Description 启用指定的 JS 插件
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Param id path int true "插件ID"
// @Success 200 {object} map[string]interface{} "启用成功"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 404 {object} models.ErrorResponse "插件不存在"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /jsplugins/{id}/enable [post]
func (h *JSPluginHandler) handleEnable(w http.ResponseWriter, r *http.Request) {
	pluginID, err := h.parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的插件ID", err)
		return
	}

	if h.manager != nil {
		if err := h.manager.EnablePlugin(r.Context(), pluginID); err != nil {
			respondError(w, http.StatusInternalServerError, "启用插件失败", err)
			return
		}
	} else {
		// 无 manager 时仅更新状态
		if err := h.repo.UpdateStatus(r.Context(), pluginID, jsplugin.JSPluginStatusActive); err != nil {
			respondError(w, http.StatusInternalServerError, "更新插件状态失败", err)
			return
		}
	}

	plugin, err := h.repo.GetByID(r.Context(), pluginID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取插件信息失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"plugin": plugin,
	})
}

// handleDisable 禁用插件
// @Summary 禁用 JS 插件
// @Description 禁用指定的 JS 插件
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Param id path int true "插件ID"
// @Success 200 {object} map[string]interface{} "禁用成功"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 404 {object} models.ErrorResponse "插件不存在"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /jsplugins/{id}/disable [post]
func (h *JSPluginHandler) handleDisable(w http.ResponseWriter, r *http.Request) {
	pluginID, err := h.parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的插件ID", err)
		return
	}

	if h.manager != nil {
		if err := h.manager.DisablePlugin(r.Context(), pluginID); err != nil {
			respondError(w, http.StatusInternalServerError, "禁用插件失败", err)
			return
		}
	} else {
		if err := h.repo.UpdateStatus(r.Context(), pluginID, jsplugin.JSPluginStatusInactive); err != nil {
			respondError(w, http.StatusInternalServerError, "更新插件状态失败", err)
			return
		}
	}

	plugin, err := h.repo.GetByID(r.Context(), pluginID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取插件信息失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"plugin": plugin,
	})
}

// handleCheckUpdate 检查远程更新
// @Summary 检查 JS 插件更新
// @Description 检查指定 JS 插件的远程更新
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Param id path int true "插件ID"
// @Success 200 {object} map[string]interface{} "更新信息"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 404 {object} models.ErrorResponse "插件不存在"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /jsplugins/{id}/check-update [get]
func (h *JSPluginHandler) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	pluginID, err := h.parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的插件ID", err)
		return
	}

	githubProxy := r.URL.Query().Get("github_proxy")

	updateInfo, err := h.packageMgr.CheckUpdate(pluginID, githubProxy)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "检查更新失败", err)
		return
	}

	// 字段名与 Flutter JSPluginUpdateCheck 对齐：has_update / current_version / remote_version / download_url
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"has_update":      updateInfo.HasUpdate,
		"current_version": updateInfo.CurrentVersion,
		"remote_version":  updateInfo.LatestVersion,
		"download_url":    updateInfo.DownloadURL,
	})
}

// handleDownloadUpdate 执行远程更新
// @Summary 下载并更新 JS 插件
// @Description 从远程下载并更新指定的 JS 插件
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Param id path int true "插件ID"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 401 {object} models.ErrorResponse "未授权"
// @Failure 404 {object} models.ErrorResponse "插件不存在"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /jsplugins/{id}/update [post]
func (h *JSPluginHandler) handleDownloadUpdate(w http.ResponseWriter, r *http.Request) {
	pluginID, err := h.parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "无效的插件ID", err)
		return
	}

	// 允许 body 为空（不使用代理）
	var req struct {
		GithubProxy string `json:"github_proxy"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	plugin, err := h.packageMgr.DownloadUpdate(pluginID, req.GithubProxy)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "下载更新失败", err)
		return
	}

	// 如果插件处于活跃状态，重载它
	if plugin.Status == jsplugin.JSPluginStatusActive && h.manager != nil {
		if reloadErr := h.manager.ReloadPlugin(r.Context(), plugin.EntryPath); reloadErr != nil {
			slog.Warn("reload plugin after download update failed", "entryPath", plugin.EntryPath, "error", reloadErr)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"plugin": plugin,
	})
}

// parseID 从 URL 参数解析插件 ID
func (h *JSPluginHandler) parseID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}
