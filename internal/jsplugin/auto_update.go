package jsplugin

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// pluginAutoUpdateConfigKey 自动更新开关配置键（与 handlers 层保持一致）。
const pluginAutoUpdateConfigKey = "plugin_auto_update"

// githubProxyConfigKey GitHub 代理地址配置键（与 handlers/upgrade.go 保持一致）。
const githubProxyConfigKey = "github_proxy"

const (
	// autoUpdateInitialDelay 服务启动后首次自动更新检查的延迟。
	autoUpdateInitialDelay = 5 * time.Minute
	// autoUpdateInterval 自动更新检查的固定周期。
	autoUpdateInterval = 6 * time.Hour
)

// PluginUpdateResult 单个插件的更新结果。
type PluginUpdateResult struct {
	PluginID       int64
	PluginName     string
	EntryPath      string
	Success        bool
	HasUpdate      bool
	CurrentVersion string
	NewVersion     string
	Error          string
}

// UpdateAllResult 批量更新的聚合结果。
type UpdateAllResult struct {
	Total   int
	Updated int
	Failed  int
	Skipped int
	Results []PluginUpdateResult
}

// RunUpdateAll 检查并更新所有具有远程更新源的插件。
//   - 跳过无 UpdateURL 的插件
//   - force=false 时跳过已是最新版的插件；force=true 强制重新下载安装
//   - 单个插件失败不中断其他插件
//   - active 插件更新后自动热重载
//
// 该方法同时被批量更新 HTTP 端点与后台自动更新 ticker 复用。
func (m *Manager) RunUpdateAll(ctx context.Context, githubProxy string, force bool) (UpdateAllResult, error) {
	plugins, err := m.repo.GetAll(ctx)
	if err != nil {
		return UpdateAllResult{}, fmt.Errorf("get all js plugins: %w", err)
	}

	result := UpdateAllResult{Total: len(plugins)}

	for _, plugin := range plugins {
		item := PluginUpdateResult{
			PluginID:       plugin.ID,
			PluginName:     plugin.Name,
			EntryPath:      plugin.EntryPath,
			CurrentVersion: plugin.Version,
		}

		if plugin.UpdateURL == "" {
			result.Skipped++
			result.Results = append(result.Results, item)
			continue
		}

		updateInfo, err := m.packager.CheckUpdate(plugin.ID, githubProxy)
		if err != nil {
			item.Error = fmt.Sprintf("检查更新失败: %v", err)
			result.Failed++
			result.Results = append(result.Results, item)
			continue
		}

		if !force && !updateInfo.HasUpdate {
			result.Skipped++
			result.Results = append(result.Results, item)
			continue
		}

		item.HasUpdate = true

		updatedPlugin, err := m.packager.DownloadUpdate(plugin.ID, githubProxy, force)
		if err != nil {
			item.Error = fmt.Sprintf("下载更新失败: %v", err)
			result.Failed++
			result.Results = append(result.Results, item)
			continue
		}

		if updatedPlugin.Status == JSPluginStatusActive {
			if reloadErr := m.ReloadPlugin(ctx, updatedPlugin.EntryPath); reloadErr != nil {
				slog.Warn("reload plugin after update failed", "entryPath", updatedPlugin.EntryPath, "error", reloadErr)
			}
		}

		item.Success = true
		item.NewVersion = updatedPlugin.Version
		result.Updated++
		result.Results = append(result.Results, item)
	}

	return result, nil
}

// AutoUpdater 后台定时自动更新插件的调度器。
// 复刻 HotReloader.WatchForChanges 的 ticker 模式：启动后延迟首跑，之后按固定周期触发；
// 每次触发前读取 plugin_auto_update 开关，关闭则跳过。
type AutoUpdater struct {
	manager *Manager
}

// NewAutoUpdater 创建自动更新调度器。
func NewAutoUpdater(m *Manager) *AutoUpdater {
	return &AutoUpdater{manager: m}
}

// Run 阻塞运行自动更新循环，直到 ctx 取消。应在独立 goroutine 中调用。
func (a *AutoUpdater) Run(ctx context.Context) {
	timer := time.NewTimer(autoUpdateInitialDelay)
	defer timer.Stop()

	// 首次延迟触发，避免与启动阶段的插件加载/同步争抢。
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		a.runOnce(ctx)
	}

	ticker := time.NewTicker(autoUpdateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.runOnce(ctx)
		}
	}
}

// runOnce 执行一次自动更新检查（若开关开启）。
func (a *AutoUpdater) runOnce(ctx context.Context) {
	m := a.manager
	if m.configService == nil {
		return
	}
	if !m.configService.GetBool(pluginAutoUpdateConfigKey, false) {
		return
	}

	// github_proxy 由业务端点以 JSON {"proxy":"..."} 形式存储（handlers/upgrade.go
	// 的 UpdateGithubProxySetting via SetJSON），必须用 GetJSON 解析。旧代码用
	// GetString 会把整个 JSON 串当作代理前缀，导致自动更新走代理时 URL 拼接错误。
	var proxyCfg struct {
		Proxy string `json:"proxy"`
	}
	_ = m.configService.GetJSON(githubProxyConfigKey, &proxyCfg)
	result, err := m.RunUpdateAll(ctx, proxyCfg.Proxy, false)
	if err != nil {
		slog.Warn("auto-update run failed", "error", err)
		return
	}
	slog.Info("auto-update completed",
		"total", result.Total,
		"updated", result.Updated,
		"failed", result.Failed,
		"skipped", result.Skipped)
}
