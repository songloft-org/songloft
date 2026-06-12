package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"songloft/internal/httputil"
	"songloft/internal/jsplugin"
)

const pluginRegistriesConfigKey = "plugin_registries"

var defaultPluginRegistries = pluginRegistriesSetting{
	Registries: []jsplugin.RegistryConfig{
		{
			URL:     "https://raw.githubusercontent.com/songloft-org/songloft-plugin-registry/main/registry.json",
			Name:    "Songloft 官方插件",
			Enabled: true,
		},
	},
}

// --- Settings: GET/PUT /api/v1/settings/plugin-registries ---

// pluginRegistriesSetting 订阅源列表配置。
type pluginRegistriesSetting struct {
	Registries []jsplugin.RegistryConfig `json:"registries"`
}

// GetRegistriesSetting 获取插件订阅源列表
// @Summary 获取插件订阅源列表
// @Description 获取用户保存的所有插件注册表订阅源 URL。未配置时返回空列表。
// @Tags JS插件管理
// @Produce json
// @Success 200 {object} pluginRegistriesSetting "订阅源列表"
// @Security BearerAuth
// @Router /settings/plugin-registries [get]
func (h *JSPluginHandler) GetRegistriesSetting(w http.ResponseWriter, r *http.Request) {
	var cfg pluginRegistriesSetting
	if err := h.configService.GetJSON(pluginRegistriesConfigKey, &cfg); err != nil {
		respondJSON(w, http.StatusOK, defaultPluginRegistries)
		return
	}
	if cfg.Registries == nil {
		cfg.Registries = []jsplugin.RegistryConfig{}
	}
	respondJSON(w, http.StatusOK, cfg)
}

// UpdateRegistriesSetting 保存插件订阅源列表
// @Summary 保存插件订阅源列表
// @Description 保存用户配置的插件注册表订阅源 URL 列表。每个源包含 URL、名称和是否启用。
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Param request body pluginRegistriesSetting true "订阅源列表"
// @Success 200 {object} pluginRegistriesSetting "保存后的订阅源列表"
// @Failure 400 {object} models.ErrorResponse "请求格式错误"
// @Failure 500 {object} models.ErrorResponse "保存配置失败"
// @Security BearerAuth
// @Router /settings/plugin-registries [put]
func (h *JSPluginHandler) UpdateRegistriesSetting(w http.ResponseWriter, r *http.Request) {
	var req pluginRegistriesSetting
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if req.Registries == nil {
		req.Registries = []jsplugin.RegistryConfig{}
	}
	if err := h.configService.SetJSON(pluginRegistriesConfigKey, req); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, req)
}

// --- Registry: POST /api/v1/jsplugins/registry/refresh ---

// registryRefreshRequest 刷新注册表请求。
type registryRefreshRequest struct {
	RegistryURL string `json:"registry_url"`
	Page        int    `json:"page"`
	PageSize    int    `json:"page_size"`
	Search      string `json:"search"`
	GithubProxy string `json:"github_proxy"`
}

// registryPluginEntry 注册表插件条目（含安装状态）。
type registryPluginEntry struct {
	Name             string `json:"name"`
	EntryPath        string `json:"entry_path"`
	Version          string `json:"version"`
	Description      string `json:"description,omitempty"`
	Author           string `json:"author,omitempty"`
	Homepage         string `json:"homepage,omitempty"`
	Icon             string `json:"icon,omitempty"`
	DownloadURL      string `json:"download_url"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installed_version,omitempty"`
	HasUpdate        bool   `json:"has_update"`
}

// registryRefreshResponse 刷新注册表响应。
type registryRefreshResponse struct {
	Plugins  []registryPluginEntry `json:"plugins"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Warnings []string              `json:"warnings,omitempty"`
}

// handleRegistryRefresh 拉取指定订阅源的插件列表
// @Summary 刷新插件注册表
// @Description 拉取指定订阅源 URL（含递归 includes），去重合并后返回分页的可用插件列表。每个插件标注是否已安装及是否有更新。
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Param request body registryRefreshRequest true "刷新请求"
// @Success 200 {object} registryRefreshResponse "插件列表"
// @Failure 400 {object} models.ErrorResponse "请求格式错误"
// @Failure 500 {object} models.ErrorResponse "拉取注册表失败"
// @Security BearerAuth
// @Router /jsplugins/registry/refresh [post]
func (h *JSPluginHandler) handleRegistryRefresh(w http.ResponseWriter, r *http.Request) {
	var req registryRefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if req.RegistryURL == "" {
		respondError(w, http.StatusBadRequest, "registry_url 不能为空", nil)
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}

	svc := jsplugin.NewRegistryService()
	entries, warnings, err := svc.FetchAndMerge(r.Context(), req.RegistryURL, req.GithubProxy)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "拉取注册表失败", err)
		return
	}

	// 获取已安装插件，构建 entryPath -> version 映射
	installedMap := h.buildInstalledMap(r.Context())

	// 搜索过滤
	search := strings.ToLower(strings.TrimSpace(req.Search))
	var filtered []registryPluginEntry
	for _, entry := range entries {
		if search != "" {
			if !strings.Contains(strings.ToLower(entry.Name), search) &&
				!strings.Contains(strings.ToLower(entry.Description), search) &&
				!strings.Contains(strings.ToLower(entry.Author), search) &&
				!strings.Contains(strings.ToLower(entry.EntryPath), search) {
				continue
			}
		}
		p := registryPluginEntry{
			Name:        entry.Name,
			EntryPath:   entry.EntryPath,
			Version:     entry.Version,
			Description: entry.Description,
			Author:      entry.Author,
			Homepage:    entry.Homepage,
			Icon:        entry.Icon,
			DownloadURL: entry.DownloadURL,
		}
		if installedVer, ok := installedMap[entry.EntryPath]; ok {
			p.Installed = true
			p.InstalledVersion = installedVer
			p.HasUpdate = entry.Version != installedVer
		}
		filtered = append(filtered, p)
	}

	total := len(filtered)
	start := (req.Page - 1) * req.PageSize
	if start >= total {
		filtered = nil
	} else {
		end := min(start+req.PageSize, total)
		filtered = filtered[start:end]
	}
	if filtered == nil {
		filtered = []registryPluginEntry{}
	}

	respondJSON(w, http.StatusOK, registryRefreshResponse{
		Plugins:  filtered,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Warnings: warnings,
	})
}

func (h *JSPluginHandler) buildInstalledMap(ctx context.Context) map[string]string {
	installed := make(map[string]string)
	plugins, err := h.repo.GetAll(ctx)
	if err != nil {
		slog.Warn("failed to load installed plugins for registry comparison", "error", err)
		return installed
	}
	for _, p := range plugins {
		installed[p.EntryPath] = p.Version
	}
	return installed
}

// --- Registry: POST /api/v1/jsplugins/registry/install ---

// registryInstallRequest 从注册表安装插件请求。
type registryInstallRequest struct {
	DownloadURL string `json:"download_url"`
	GithubProxy string `json:"github_proxy"`
}

// handleRegistryInstall 从注册表 download_url 安装插件
// @Summary 从注册表安装插件
// @Description 从注册表中的 download_url 下载 ZIP 并安装插件。如果 entry_path 已存在则自动走更新路径。支持 GitHub 代理。
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Param request body registryInstallRequest true "安装请求"
// @Success 200 {object} jsPluginUploadResponse "安装结果（更新已有插件）"
// @Success 201 {object} jsPluginUploadResponse "安装结果（新插件）"
// @Failure 400 {object} models.ErrorResponse "请求格式错误"
// @Failure 500 {object} models.ErrorResponse "下载或安装失败"
// @Security BearerAuth
// @Router /jsplugins/registry/install [post]
func (h *JSPluginHandler) handleRegistryInstall(w http.ResponseWriter, r *http.Request) {
	var req registryInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if req.DownloadURL == "" {
		respondError(w, http.StatusBadRequest, "download_url 不能为空", nil)
		return
	}

	downloadURL := req.DownloadURL
	if req.GithubProxy != "" {
		proxyPrefix := req.GithubProxy
		if proxyPrefix[len(proxyPrefix)-1] != '/' {
			proxyPrefix += "/"
		}
		downloadURL = proxyPrefix + downloadURL
	}

	zipData, err := downloadZIP(r.Context(), downloadURL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "下载插件失败", err)
		return
	}

	plugin, wasUpdate, err := h.packageMgr.InstallFromUpload(zipData)
	if err != nil {
		respondJSON(w, http.StatusOK, jsPluginUploadResponse{
			Total:   1,
			Success: 0,
			Failed:  1,
			Results: []jsPluginUploadResult{{
				FileName: req.DownloadURL,
				Error:    err.Error(),
				Success:  false,
			}},
			Message: "安装插件失败",
		})
		return
	}

	if h.manager != nil {
		if wasUpdate && plugin.Status == jsplugin.JSPluginStatusActive {
			if reloadErr := h.manager.ReloadPlugin(r.Context(), plugin.EntryPath); reloadErr != nil {
				slog.Warn("reload plugin after registry install failed", "entryPath", plugin.EntryPath, "error", reloadErr)
			}
		} else if !wasUpdate {
			if enableErr := h.manager.EnablePlugin(r.Context(), plugin.ID); enableErr != nil {
				slog.Warn("auto-enable plugin after registry install failed", "entryPath", plugin.EntryPath, "error", enableErr)
			} else {
				plugin.Status = jsplugin.JSPluginStatusActive
			}
		}
	}

	var (
		message string
		status  int
	)
	if wasUpdate {
		message = fmt.Sprintf("插件已更新到 v%s", plugin.Version)
		status = http.StatusOK
	} else {
		message = fmt.Sprintf("插件 %s 安装成功", plugin.EntryPath)
		status = http.StatusCreated
	}

	respondJSON(w, status, jsPluginUploadResponse{
		Total:   1,
		Success: 1,
		Failed:  0,
		Results: []jsPluginUploadResult{{
			FileName: req.DownloadURL,
			Plugin:   plugin,
			Success:  true,
		}},
		Message: message,
	})
}

func downloadZIP(ctx context.Context, url string) ([]byte, error) {
	client := httputil.NewClient(60 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d from %s", resp.StatusCode, url)
	}

	const maxZIPSize = 50 << 20 // 50 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxZIPSize+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(data) > maxZIPSize {
		return nil, fmt.Errorf("zip file exceeds %d bytes", maxZIPSize)
	}
	return data, nil
}

// --- Settings: GET/PUT /api/v1/settings/http-proxy ---

const httpProxyConfigKey = "http_proxy"

// httpProxySetting HTTP 代理配置。
type httpProxySetting struct {
	Proxy string `json:"proxy"`
}

// GetHttpProxySetting 获取 HTTP 代理配置
// @Summary 获取 HTTP 代理配置
// @Description 获取全局 HTTP 代理地址。所有后端外发请求（插件下载、注册表拉取、升级检查等）会通过此代理转发。未配置时返回空字符串（直连）。
// @Tags 设置
// @Produce json
// @Success 200 {object} httpProxySetting "代理配置"
// @Security BearerAuth
// @Router /settings/http-proxy [get]
func (h *JSPluginHandler) GetHttpProxySetting(w http.ResponseWriter, r *http.Request) {
	var cfg httpProxySetting
	if err := h.configService.GetJSON(httpProxyConfigKey, &cfg); err != nil {
		respondJSON(w, http.StatusOK, httpProxySetting{Proxy: ""})
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

// UpdateHttpProxySetting 保存 HTTP 代理配置
// @Summary 保存 HTTP 代理配置
// @Description 设置全局 HTTP 代理地址（如 http://192.168.1.1:7890）。设为空字符串则关闭代理。保存后即时生效，无需重启。
// @Tags 设置
// @Accept json
// @Produce json
// @Param request body httpProxySetting true "代理配置"
// @Success 200 {object} httpProxySetting "保存后的代理配置"
// @Failure 400 {object} models.ErrorResponse "请求格式错误或代理地址无效"
// @Failure 500 {object} models.ErrorResponse "保存配置失败"
// @Security BearerAuth
// @Router /settings/http-proxy [put]
func (h *JSPluginHandler) UpdateHttpProxySetting(w http.ResponseWriter, r *http.Request) {
	var req httpProxySetting
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if err := httputil.SetGlobalProxy(req.Proxy); err != nil {
		respondError(w, http.StatusBadRequest, "代理地址无效", err)
		return
	}
	if err := h.configService.SetJSON(httpProxyConfigKey, req); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	slog.Info("HTTP 代理已更新", "proxy", req.Proxy)
	respondJSON(w, http.StatusOK, req)
}
