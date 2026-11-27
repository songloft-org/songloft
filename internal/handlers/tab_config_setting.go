package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"songloft/internal/models"
)

const tabConfigKey = "tab_config"

// maxOptionalTabs 可选项（曲库 + 已启用插件 Tab）的数量上限。
// 首页与设置不计入该上限。
const maxOptionalTabs = 10

// tabConfigSetting 底部导航栏 Tab 配置
type tabConfigSetting struct {
	ShowHome      bool             `json:"show_home"`
	ShowLibrary   bool             `json:"show_library"`
	ShowPlaylists bool             `json:"show_playlists"`
	PluginTabs    []pluginTabEntry `json:"plugin_tabs"`
}

// pluginTabEntry 插件 Tab 条目
type pluginTabEntry struct {
	PluginID  int    `json:"plugin_id"`
	EntryPath string `json:"entry_path"`
	Name      string `json:"name"`
}

// tabConfigPluginSource 提供插件安装状态，用于 Tab 配置的孤儿条目清理与限额统计。
type tabConfigPluginSource interface {
	GetAll(ctx context.Context) ([]*models.JSPlugin, error)
}

var defaultTabConfig = tabConfigSetting{
	ShowHome:      true,
	ShowLibrary:   true,
	ShowPlaylists: true,
	PluginTabs:    []pluginTabEntry{},
}

func (h *ConfigHandler) readTabConfig() tabConfigSetting {
	cfg := tabConfigSetting{ShowHome: true}
	if err := h.configService.GetJSON(tabConfigKey, &cfg); err != nil {
		return defaultTabConfig
	}
	if cfg.PluginTabs == nil {
		cfg.PluginTabs = []pluginTabEntry{}
	}
	return cfg
}

// GetTabConfigSetting 获取底部导航栏 Tab 配置
// @Summary 获取底部导航栏 Tab 配置
// @Description 获取用户自定义的底部导航栏 Tab 配置。设置固定显示，首页（show_home）和曲库可关闭。首页默认显示，隐藏时客户端进入第一个可见标签。可选项（曲库 + 已启用插件 Tab）总数不超过 10 个。未配置时返回默认值（首页、曲库、设置）。移动端超过 5 个时自动折叠到「更多」菜单，桌面端侧边栏可全部展示。show_playlists 字段仅为兼容旧配置保留（歌单已并入曲库），不参与任何展示与计数。
// @Tags 设置
// @Produce json
// @Success 200 {object} tabConfigSetting "Tab 配置"
// @Security BearerAuth
// @Router /settings/tab-config [get]
func (h *ConfigHandler) GetTabConfigSetting(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.readTabConfig())
}

// UpdateTabConfigSetting 保存底部导航栏 Tab 配置
// @Summary 保存底部导航栏 Tab 配置
// @Description 保存用户自定义的底部导航栏 Tab 配置。设置固定显示，首页可通过 show_home 关闭，缺省时保留当前首页设置。可选项为曲库和已启用（active）插件 Tab，总数不超过 10 个。插件已不存在的孤儿条目会在保存时被静默清理（保存即自愈）；禁用插件的条目保留但不占名额，重新启用后自动恢复。每个插件 Tab 的 entry_path 和 name 不能为空，且不能重复。show_playlists 字段仅为兼容旧配置保留（歌单已并入曲库），不参与任何展示与计数。移动端超过 5 个时自动折叠到「更多」菜单。
// @Tags 设置
// @Accept json
// @Produce json
// @Param request body tabConfigSetting true "Tab 配置"
// @Success 200 {object} tabConfigSetting "保存后的 Tab 配置（孤儿条目已清理）"
// @Failure 400 {object} models.ErrorResponse "请求格式错误或校验失败"
// @Failure 500 {object} models.ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /settings/tab-config [put]
func (h *ConfigHandler) UpdateTabConfigSetting(w http.ResponseWriter, r *http.Request) {
	req := tabConfigSetting{ShowHome: h.readTabConfig().ShowHome}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if req.PluginTabs == nil {
		req.PluginTabs = []pluginTabEntry{}
	}

	seen := make(map[string]bool, len(req.PluginTabs))
	for _, pt := range req.PluginTabs {
		if pt.EntryPath == "" {
			respondError(w, http.StatusBadRequest, "插件 Tab 的 entry_path 不能为空", nil)
			return
		}
		if pt.Name == "" {
			respondError(w, http.StatusBadRequest, "插件 Tab 的 name 不能为空", nil)
			return
		}
		if seen[pt.EntryPath] {
			respondError(w, http.StatusBadRequest, "插件 Tab 的 entry_path 不能重复", nil)
			return
		}
		seen[pt.EntryPath] = true
	}

	plugins, err := h.pluginRepo.GetAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取插件列表失败", err)
		return
	}
	pluginStatus := make(map[string]models.JSPluginStatus, len(plugins))
	for _, p := range plugins {
		pluginStatus[p.EntryPath] = p.Status
	}

	// 清理插件已不存在的孤儿条目：卸载插件不会反向通知本配置，
	// 条目会永久占着名额且首页不渲染，导致计数与可见 Tab 数不符（#416）。
	// 保存即自愈：响应返回清理后的配置，客户端应以响应为准。
	// 禁用插件的条目刻意保留：不占名额，重新启用后 Tab 自动恢复。
	cleaned := make([]pluginTabEntry, 0, len(req.PluginTabs))
	optionalCount := 0
	if req.ShowLibrary {
		optionalCount++
	}
	for _, pt := range req.PluginTabs {
		status, ok := pluginStatus[pt.EntryPath]
		if !ok {
			continue
		}
		cleaned = append(cleaned, pt)
		if status == models.JSPluginStatusActive {
			optionalCount++
		}
	}
	req.PluginTabs = cleaned

	if optionalCount > maxOptionalTabs {
		respondError(w, http.StatusBadRequest, "可选标签总数不能超过 10 个", nil)
		return
	}

	if err := h.configService.SetJSON(tabConfigKey, req); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, req)
}

// Ensure models.ErrorResponse is referenced for swagger generation.
var _ = models.ErrorResponse{}
