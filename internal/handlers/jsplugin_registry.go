package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"songloft/internal/httputil"
	"songloft/internal/jsplugin"
)

const pluginRegistriesConfigKey = "plugin_registries"

// pluginAutoUpdateConfigKey 自动更新开关配置键。
// 与 jsplugin 包内后台 ticker 读取的键保持一致。
const pluginAutoUpdateConfigKey = "plugin_auto_update"

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
	// 源列表变了：让缓存里按旧配置拉取的结果立刻作废，腾出条目配额。
	if h.registrySvc != nil {
		h.registrySvc.InvalidateCache()
	}
	respondJSON(w, http.StatusOK, req)
}

// --- Settings: GET/PUT /api/v1/settings/plugin-auto-update ---

// pluginAutoUpdateSetting 插件自动更新开关配置。
type pluginAutoUpdateSetting struct {
	Enabled bool `json:"enabled"`
}

// GetPluginAutoUpdateSetting 获取插件自动更新开关
// @Summary 获取插件自动更新开关
// @Description 获取“后台自动更新已安装插件”开关的当前状态。开启后，服务会在启动后延迟数分钟检查一次、之后每 6 小时定时检查所有具有远程更新源的插件并自动更新。默认关闭。
// @Tags 设置
// @Produce json
// @Success 200 {object} pluginAutoUpdateSetting "返回 enabled 字段表示开关状态"
// @Security BearerAuth
// @Router /settings/plugin-auto-update [get]
func (h *JSPluginHandler) GetPluginAutoUpdateSetting(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, pluginAutoUpdateSetting{
		Enabled: h.configService.GetBool(pluginAutoUpdateConfigKey, false),
	})
}

// UpdatePluginAutoUpdateSetting 保存插件自动更新开关
// @Summary 保存插件自动更新开关
// @Description 开启/关闭插件后台自动更新。开启后后台 ticker 会定时对有更新源的插件执行“检查更新 + 下载安装 + 热重载”。开关即时生效，无需重启。
// @Tags 设置
// @Accept json
// @Produce json
// @Param request body pluginAutoUpdateSetting true "开关请求"
// @Success 200 {object} pluginAutoUpdateSetting "返回 enabled 字段表示更新后的开关状态"
// @Failure 400 {object} models.ErrorResponse "请求格式错误"
// @Failure 500 {object} models.ErrorResponse "保存配置失败"
// @Security BearerAuth
// @Router /settings/plugin-auto-update [put]
func (h *JSPluginHandler) UpdatePluginAutoUpdateSetting(w http.ResponseWriter, r *http.Request) {
	var req pluginAutoUpdateSetting
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	value := "false"
	if req.Enabled {
		value = "true"
	}
	if err := h.configService.Set(pluginAutoUpdateConfigKey, value); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, req)
}

// --- Registry: POST /api/v1/jsplugins/registry/refresh ---

// registryRefreshRequest 刷新注册表请求。
type registryRefreshRequest struct {
	RegistryURL string `json:"registry_url"`
	AllSources  bool   `json:"all_sources"`
	Page        int    `json:"page"`
	PageSize    int    `json:"page_size"`
	Search      string `json:"search"`
	GithubProxy string `json:"github_proxy"`
	Token       string `json:"token,omitempty"`
	// Force 为 true 时绕过服务端缓存强制重新拉取。供「刷新」按钮使用；
	// 翻页与搜索不应设置它，否则每翻一页都会重拉整棵注册表树。
	Force bool `json:"force,omitempty"`
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
	// SourceURL 该插件所属订阅源 URL（仅「全部」聚合模式返回），
	// 安装时回传给后端以按源解析私有源 token。
	SourceURL string `json:"source_url,omitempty"`
	// SourceName 该插件所属订阅源名称（仅「全部」聚合模式返回），
	// 供 UI 区分 entry_path 相同的两个条目。
	SourceName string `json:"source_name,omitempty"`
	// Identity 是 entry_path 之外的身份维度（规范化 author，或 GitHub 仓库兜底）。
	// entry_path 撞名时前端据 (entry_path, identity) 做稳定行标识与就地状态更新。
	// 为空表示无法判定身份（此时仅按 entry_path 判定）。
	Identity string `json:"identity,omitempty"`
	// Conflict 为 true 表示本地已装了同 entry_path 但**不同身份**的插件。
	// 此时 installed=false（这不是同一个插件），安装它会覆盖掉本地那个。
	Conflict bool `json:"conflict,omitempty"`
	// ConflictWith 描述占用该 entry_path 的本地插件，可直接展示给用户。
	ConflictWith string `json:"conflict_with,omitempty"`
}

// registryRefreshResponse 刷新注册表响应。
type registryRefreshResponse struct {
	Plugins  []registryPluginEntry `json:"plugins"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Warnings []string              `json:"warnings,omitempty"`
}

// handleRegistryRefresh 拉取订阅源的插件列表
// @Summary 刷新插件注册表
// @Description 拉取订阅源（含递归 includes），去重合并后返回分页的可用插件列表。每个插件标注是否已安装及是否有更新。默认拉取单个 registry_url，可选传入 token 字段访问需要认证的私有源（如 GitHub 私有仓库 PAT）。当 all_sources=true 时忽略 registry_url/token，改为聚合已保存的所有启用订阅源（各源用自身存储的 token）。
// @Description 去重键为 entry_path + identity（identity = 规范化 author，author 为空时用 updateUrl 的 GitHub owner/repo 兜底）：entry_path 相同但作者不同的插件会各自成行，同一插件被多个源收录时仍只显示一条（保留高版本）。
// @Description 若某条目的 entry_path 已被本地一个**不同作者**的插件占用，返回 installed=false、conflict=true，并在 conflict_with 中描述占用者；此时安装该插件需要用户确认覆盖。
// @Description 拉取结果在服务端缓存 5 分钟：分页与搜索都在缓存的完整列表上做切片/过滤，不会重复拉取远端。传 force=true 绕过缓存强制重拉（供「刷新」按钮使用，翻页与搜索不要传）。安装状态（installed/has_update/conflict）不受缓存影响，每次请求都从数据库实时计算。
// @Description Flutter/Lynx 商店通常由客户端逐个拉取清单并渐进展示；此接口保留为 Web 跨域或网络失败的兼容回退。携带源 Token 的远端请求绕过 GitHub 加速代理，此接口不会读取共享的 GitHub 发现 Token。
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
	if !req.AllSources && req.RegistryURL == "" {
		respondError(w, http.StatusBadRequest, "registry_url 不能为空", nil)
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}

	// 注册表拉取可能耗时十余秒（最多 500 个 plugin.json，8 并发，单请求 15s 超时）。
	// 使用脱离请求的 context：即使客户端中途断开，拉取仍会完成并写入缓存，
	// 下一次请求可直接命中缓存而非再等 15 秒。
	fetchCtx := context.WithoutCancel(r.Context())
	fetchCtx, cancel := context.WithTimeout(fetchCtx, 30*time.Second)
	defer cancel()

	// 复用 handler 持有的 RegistryService：结果在 TTL 内缓存，翻页与搜索都在
	// 缓存的完整列表上做切片/过滤，不再重拉整棵注册表树。force=true 时绕过缓存。
	var (
		entries  []jsplugin.RegistryEntry
		warnings []string
	)
	if req.AllSources {
		// 聚合所有启用源：读配置 → 过滤 enabled → 逐源拉取合并去重
		var cfg pluginRegistriesSetting
		if err := h.configService.GetJSON(pluginRegistriesConfigKey, &cfg); err != nil {
			cfg = defaultPluginRegistries
		}
		enabled := make([]jsplugin.RegistryConfig, 0, len(cfg.Registries))
		for _, src := range cfg.Registries {
			if src.Enabled {
				enabled = append(enabled, src)
			}
		}
		entries, warnings = h.registrySvc.FetchAndMergeMultiCached(fetchCtx, enabled, req.GithubProxy, req.Force)
	} else {
		var err error
		entries, warnings, err = h.registrySvc.FetchAndMergeCached(fetchCtx, req.RegistryURL, req.GithubProxy, req.Token, req.Force)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "拉取注册表失败", err)
			return
		}
	}

	// 获取已安装插件，构建 entryPath -> 已安装信息映射。
	// 同样使用脱离请求的 context：这是毫秒级的本地 DB 查询，不应因客户端断开而失败。
	installedMap := h.buildInstalledMap(fetchCtx)
	sourceNames := h.buildSourceNames()

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
			SourceURL:   entry.SourceURL,
			SourceName:  sourceNames[entry.SourceURL],
			Identity:    jsplugin.PluginIdentity(entry.Author, entry.UpdateURL),
		}
		resolveInstallState(&p, installedMap)
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

// resolveInstallState 按 entry_path + 身份判定商店条目的安装状态，就地写入 p。
//
// 本地同一个 entry_path 物理上只能装一个插件，所以只能按 entry_path 查；但查到的
// 未必就是「这一条」——entry_path 可能被另一个作者的插件占用（#339）。三种结果：
//   - 没查到 → 未安装
//   - 查到且身份一致 → 已安装，按版本号判断是否可更新
//   - 查到但身份不同 → conflict（不是同一个插件，安装它会替换本地那个）
func resolveInstallState(p *registryPluginEntry, installed map[string]installedPlugin) {
	inst, ok := installed[p.EntryPath]
	if !ok {
		return
	}
	if !jsplugin.SameIdentity(p.Identity, inst.Identity) {
		p.Conflict = true
		p.ConflictWith = inst.describe()
		return
	}
	p.Installed = true
	p.InstalledVersion = inst.Version
	// 用版本号比较而非字符串不等：撞名时两边版本号不同，字符串比较会永久
	// 显示「可更新」，用户点下去就把本地插件覆盖掉了。
	p.HasUpdate = jsplugin.CompareVersion(p.Version, inst.Version) > 0
}

// installedPlugin 已安装插件在商店比对时需要的信息。
type installedPlugin struct {
	Name     string
	Author   string
	Version  string
	Identity string
}

// describe 返回可直接展示给用户的「谁占用了这个 entry_path」描述。
func (p installedPlugin) describe() string {
	if p.Author != "" {
		return fmt.Sprintf("%s（作者：%s）v%s", p.Name, p.Author, p.Version)
	}
	return fmt.Sprintf("%s v%s", p.Name, p.Version)
}

// buildInstalledMap 构建 entry_path -> 已安装插件信息的映射。
// key 只用 entry_path：本地同一个 entry_path 物理上只能装一个插件
// （DB UNIQUE + 同名 ZIP + 同名 static 目录 + 同一路由前缀）。
func (h *JSPluginHandler) buildInstalledMap(ctx context.Context) map[string]installedPlugin {
	installed := make(map[string]installedPlugin)
	plugins, err := h.repo.GetAll(ctx)
	if err != nil {
		slog.Warn("failed to load installed plugins for registry comparison", "error", err)
		return installed
	}
	for _, p := range plugins {
		installed[p.EntryPath] = installedPlugin{
			Name:     p.Name,
			Author:   p.Author,
			Version:  p.Version,
			Identity: jsplugin.PluginIdentity(p.Author, p.UpdateURL),
		}
	}
	return installed
}

// buildSourceNames 返回订阅源 URL -> 源名称的映射，供商店条目标注来源。
// entry_path 撞名时这是用户区分两条条目的主要依据。
func (h *JSPluginHandler) buildSourceNames() map[string]string {
	var cfg pluginRegistriesSetting
	if err := h.configService.GetJSON(pluginRegistriesConfigKey, &cfg); err != nil {
		cfg = defaultPluginRegistries
	}
	names := make(map[string]string, len(cfg.Registries))
	for _, src := range cfg.Registries {
		if src.Name != "" {
			names[src.URL] = src.Name
		}
	}
	return names
}

// --- Registry: POST /api/v1/jsplugins/registry/install ---

// registryInstallRequest 从注册表安装插件请求。
type registryInstallRequest struct {
	DownloadURL string `json:"download_url"`
	GithubProxy string `json:"github_proxy"`
	Token       string `json:"token,omitempty"`
	// SourceURL 插件所属订阅源 URL。「全部」聚合模式安装时回传：
	// 当未显式提供 token 时，后端据此从 plugin_registries 配置解析该源的 token。
	SourceURL string `json:"source_url,omitempty"`
	// Overwrite 为 true 时允许覆盖掉本地已装的同 entry_path 但不同作者的插件。
	// 默认 false：这种情况返回 409，由前端二次确认后带该字段重发。
	Overwrite bool `json:"overwrite,omitempty"`
}

// resolveSourceToken 根据订阅源 URL 从配置中查出其存储的 token。
// 找不到匹配源时返回空字符串。
func (h *JSPluginHandler) resolveSourceToken(sourceURL string) string {
	if sourceURL == "" {
		return ""
	}
	var cfg pluginRegistriesSetting
	if err := h.configService.GetJSON(pluginRegistriesConfigKey, &cfg); err != nil {
		cfg = defaultPluginRegistries
	}
	for _, src := range cfg.Registries {
		if src.URL == sourceURL {
			return src.Token
		}
	}
	return ""
}

// handleRegistryInstall 从注册表 download_url 安装插件
// @Summary 从注册表安装插件
// @Description 从注册表中的 download_url 下载 ZIP 并安装插件。如果 entry_path 已存在且属于同一作者，则自动走更新路径。匿名下载支持 GitHub 加速代理；携带 Token 的下载始终绕过加速代理，避免凭据泄漏。可选传入 token 字段用于从需要认证的私有源下载；若未提供 token 但提供了 source_url，后端会自动从 plugin_registries 配置解析该源存储的 token。
// @Description 若 entry_path 已被本地一个**不同作者**的插件占用，返回 409 且不做任何写入（不落盘、不动 static 目录、不改数据库）。前端应向用户说明会替换原插件后，带 overwrite=true 重发本请求。
// @Tags JS插件管理
// @Accept json
// @Produce json
// @Param request body registryInstallRequest true "安装请求"
// @Success 200 {object} jsPluginUploadResponse "安装结果（更新已有插件）"
// @Success 201 {object} jsPluginUploadResponse "安装结果（新插件）"
// @Failure 400 {object} models.ErrorResponse "请求格式错误"
// @Failure 409 {object} models.ErrorResponse "entry_path 已被另一个作者的插件占用，需用户确认后带 overwrite=true 重试"
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

	// 「全部」聚合模式：未显式给 token 时，按来源源 URL 从配置解析 token
	if req.Token == "" && req.SourceURL != "" {
		req.Token = h.resolveSourceToken(req.SourceURL)
	}

	var zipData []byte

	// GitHub browser-style release URLs don't accept Bearer tokens for private
	// repos. When a token is provided and the URL matches, use the GitHub API.
	if req.Token != "" {
		if owner, repo, tag, filename, ok := parseGitHubReleaseURL(req.DownloadURL); ok {
			data, err := downloadGitHubReleaseAsset(r.Context(), owner, repo, tag, filename, req.Token, req.GithubProxy)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "下载插件失败", err)
				return
			}
			zipData = data
		}
	}

	if zipData == nil {
		data, err := downloadZIP(r.Context(), req.DownloadURL, req.GithubProxy, req.Token)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "下载插件失败", err)
			return
		}
		zipData = data
	}

	plugin, wasUpdate, err := h.packageMgr.InstallFromUploadWithOptions(zipData, jsplugin.InstallOptions{
		RejectIdentityConflict: !req.Overwrite,
	})
	// entry_path 被另一个作者的插件占用：拒绝并让前端二次确认，而不是静默覆盖。
	var conflict *jsplugin.EntryPathConflictError
	if errors.As(err, &conflict) {
		respondError(w, http.StatusConflict, fmt.Sprintf(
			"插件标识 %q 已被本地插件 %s 占用。继续安装会替换它，且原插件的数据将被新插件继承。",
			conflict.EntryPath, installedPlugin{
				Name:    conflict.ExistingName,
				Author:  conflict.ExistingAuthor,
				Version: conflict.ExistingVersion,
			}.describe()), nil)
		return
	}
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
		activationCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Second)
		defer cancel()
		if wasUpdate && plugin.Status == jsplugin.JSPluginStatusActive {
			if reloadErr := h.manager.ReloadPlugin(activationCtx, plugin.EntryPath); reloadErr != nil {
				slog.Warn("reload plugin after registry install failed", "entryPath", plugin.EntryPath, "error", reloadErr)
			}
		} else if !wasUpdate {
			if enableErr := h.manager.EnablePlugin(activationCtx, plugin.ID); enableErr != nil {
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

func downloadZIP(ctx context.Context, rawURL string, githubProxy string, token string) ([]byte, error) {
	client := httputil.NewClient(60 * time.Second)
	var header http.Header
	if token != "" {
		githubProxy = ""
		header = http.Header{"Authorization": []string{"Bearer " + token}}
	}
	resp, err := httputil.GetWithGithubProxyFallback(ctx, client, rawURL, githubProxy,
		httputil.GithubGetOptions{Header: header})
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d from %s", resp.StatusCode, rawURL)
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

// parseGitHubReleaseURL extracts owner, repo, tag, and filename from a GitHub
// browser-style release download URL. Returns ok=false for non-matching URLs.
func parseGitHubReleaseURL(rawURL string) (owner, repo, tag, filename string, ok bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host != "github.com" {
		return
	}
	// /owner/repo/releases/download/tag/filename
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) == 6 && parts[2] == "releases" && parts[3] == "download" {
		return parts[0], parts[1], parts[4], parts[5], true
	}
	return
}

// downloadGitHubReleaseAsset downloads a release asset from a private GitHub
// repo via the GitHub API. Browser-style release URLs (github.com/.../releases/
// download/...) don't accept Bearer tokens for private repos—only the API does.
// 携带 Token 时，两次 api.github.com 请求都绕过 GitHub 加速代理。
func downloadGitHubReleaseAsset(ctx context.Context, owner, repo, tag, filename, token, githubProxy string) ([]byte, error) {
	client := httputil.NewClient(60 * time.Second)
	return downloadGitHubReleaseAssetWithClient(ctx, client, owner, repo, tag, filename, token, githubProxy)
}

func downloadGitHubReleaseAssetWithClient(ctx context.Context, client *http.Client, owner, repo, tag, filename, token, githubProxy string) ([]byte, error) {
	if token != "" {
		githubProxy = ""
	}

	releaseURL := httputil.ApplyGithubProxy(
		fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, tag),
		githubProxy,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create release request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get release by tag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get release %s/%s@%s: http status %d", owner, repo, tag, resp.StatusCode)
	}

	var release struct {
		Assets []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}

	var assetID int64
	for _, a := range release.Assets {
		if a.Name == filename {
			assetID = a.ID
			break
		}
	}
	if assetID == 0 {
		return nil, fmt.Errorf("asset %q not found in release %s/%s@%s", filename, owner, repo, tag)
	}

	assetURL := httputil.ApplyGithubProxy(
		fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", owner, repo, assetID),
		githubProxy,
	)
	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create asset request: %w", err)
	}
	if token != "" {
		req2.Header.Set("Authorization", "Bearer "+token)
	}
	req2.Header.Set("Accept", "application/octet-stream")

	resp2, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("download asset: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download asset %s/%s@%s/%s: http status %d", owner, repo, tag, filename, resp2.StatusCode)
	}

	const maxZIPSize = 50 << 20
	data, err := io.ReadAll(io.LimitReader(resp2.Body, maxZIPSize+1))
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
// @Description 获取 Songloft 保存的全局 HTTP 代理地址。未配置时返回空字符串，此时插件后端的 fetch 和 httpStream 可使用进程的代理环境变量。
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
// @Description 设置全局 HTTP 代理地址（如 http://192.168.1.1:7890）。设为空字符串会清除保存的地址，此时插件后端的 fetch 和 httpStream 按 HTTP_PROXY、HTTPS_PROXY 和 NO_PROXY 选择代理。保存后即时生效，无需重启。
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

// --- Settings: GET/PUT /api/v1/settings/plugin-keep-alive ---

const pluginKeepAliveConfigKey = "plugin_keep_alive"

// pluginKeepAliveSetting 插件常驻白名单配置。
type pluginKeepAliveSetting struct {
	Plugins []string `json:"plugins"`
}

// GetPluginKeepAliveSetting 获取插件常驻白名单
// @Summary 获取插件常驻白名单
// @Description 获取不会被自动休眠的插件 entryPath 列表。白名单中的插件即使空闲超过 10 分钟也不会被卸载。未配置时返回空列表。
// @Tags 设置
// @Produce json
// @Success 200 {object} pluginKeepAliveSetting "常驻白名单"
// @Security BearerAuth
// @Router /settings/plugin-keep-alive [get]
func (h *JSPluginHandler) GetPluginKeepAliveSetting(w http.ResponseWriter, r *http.Request) {
	var cfg pluginKeepAliveSetting
	if err := h.configService.GetJSON(pluginKeepAliveConfigKey, &cfg); err != nil {
		respondJSON(w, http.StatusOK, pluginKeepAliveSetting{Plugins: []string{}})
		return
	}
	if cfg.Plugins == nil {
		cfg.Plugins = []string{}
	}
	respondJSON(w, http.StatusOK, cfg)
}

// UpdatePluginKeepAliveSetting 保存插件常驻白名单
// @Summary 保存插件常驻白名单
// @Description 设置不会被自动休眠的插件 entryPath 列表。保存后即时生效，白名单中的插件将跳过空闲检查，始终保持运行。
// @Tags 设置
// @Accept json
// @Produce json
// @Param request body pluginKeepAliveSetting true "常驻白名单"
// @Success 200 {object} pluginKeepAliveSetting "保存后的白名单"
// @Failure 400 {object} models.ErrorResponse "请求格式错误"
// @Failure 500 {object} models.ErrorResponse "保存配置失败"
// @Security BearerAuth
// @Router /settings/plugin-keep-alive [put]
func (h *JSPluginHandler) UpdatePluginKeepAliveSetting(w http.ResponseWriter, r *http.Request) {
	var req pluginKeepAliveSetting
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if req.Plugins == nil {
		req.Plugins = []string{}
	}
	if err := h.configService.SetJSON(pluginKeepAliveConfigKey, req); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, req)
}
