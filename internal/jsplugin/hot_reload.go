package jsplugin

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HotReloader 管理插件热更新
type HotReloader struct {
	manager *Manager
	mu      sync.Mutex
}

// NewHotReloader 创建热更新管理器
func NewHotReloader(manager *Manager) *HotReloader {
	return &HotReloader{
		manager: manager,
	}
}

// ReloadPlugin 热更新单个插件
// 流程：冻结消息 → 调用 onDeinit 回调 → 销毁旧 JS VM → 重新从 ZIP 加载 → 创建新 VM → 调用 onInit 回调 → 解冻消息恢复
// 错误回滚：如果新版本加载失败，尝试恢复旧版本
func (h *HotReloader) ReloadPlugin(ctx context.Context, pluginID int64) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 获取插件信息
	plugin, err := h.manager.repo.GetByID(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("get plugin by id %d: %w", pluginID, err)
	}

	entryPath := plugin.EntryPath
	slog.Info("hot reload plugin started", "plugin", entryPath, "id", pluginID)

	// 获取旧服务（如果存在）
	oldService, hasOld := h.manager.GetService(entryPath)

	// 冻结旧服务（停止接收新消息）
	if hasOld {
		oldService.mu.Lock()
		oldService.status = ServiceStatusFrozen
		oldService.mu.Unlock()
		slog.Info("plugin frozen for hot reload", "plugin", entryPath)
	}

	// 卸载旧插件
	if hasOld {
		if err := h.manager.UnloadPlugin(ctx, entryPath); err != nil {
			slog.Warn("unload old plugin failed during hot reload", "plugin", entryPath, "error", err)
			// 尝试恢复旧服务状态
			if hasOld {
				oldService.mu.Lock()
				oldService.status = ServiceStatusReady
				oldService.mu.Unlock()
			}
			return fmt.Errorf("unload plugin during hot reload: %w", err)
		}
	}

	// 清除字节码缓存（强制下次加载时重新编译）
	cacheDir := filepath.Join(h.manager.pluginsDataDir, plugin.EntryPath, "cache")
	os.RemoveAll(cacheDir)

	// 重新加载插件
	if err := h.manager.LoadPlugin(ctx, plugin); err != nil {
		slog.Error("load new plugin failed during hot reload, attempting rollback", "plugin", entryPath, "error", err)

		// 回滚：尝试重新加载旧版本
		rollbackErr := h.manager.LoadPlugin(ctx, plugin)
		if rollbackErr != nil {
			slog.Error("rollback failed during hot reload", "plugin", entryPath, "error", rollbackErr)
			// 标记为错误状态
			_ = h.manager.repo.UpdateStatus(ctx, pluginID, JSPluginStatusError)
			return fmt.Errorf("hot reload failed and rollback failed: load=%w, rollback=%v", err, rollbackErr)
		}
		slog.Warn("hot reload failed, rolled back to old version", "plugin", entryPath)
		return fmt.Errorf("hot reload failed (rolled back): %w", err)
	}

	slog.Info("hot reload plugin completed", "plugin", entryPath, "id", pluginID)
	return nil
}

// ReloadAll 热更新所有活跃插件
func (h *HotReloader) ReloadAll(ctx context.Context) []error {
	services := h.manager.ListServices()
	var errs []error

	for _, svc := range services {
		plugin := svc.Plugin()
		if err := h.ReloadPlugin(ctx, plugin.ID); err != nil {
			errs = append(errs, fmt.Errorf("reload %q: %w", plugin.EntryPath, err))
		}
	}

	return errs
}

// WatchForChanges 监控插件目录变化触发热更新（使用 polling，每 30s 检查一次 ZIP 文件 mtime）
func (h *HotReloader) WatchForChanges(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	slog.Info("hot reload watcher started", "interval", "30s", "dir", h.manager.pluginsDir)

	for {
		select {
		case <-ctx.Done():
			slog.Info("hot reload watcher stopped")
			return
		case <-ticker.C:
			h.checkForChanges(ctx)
		}
	}
}

// checkForChanges 检查插件文件变化
func (h *HotReloader) checkForChanges(ctx context.Context) {
	services := h.manager.ListServices()

	for _, svc := range services {
		plugin := svc.Plugin()
		zipPath := filepath.Join(h.manager.pluginsDir, plugin.FilePath)

		info, err := os.Stat(zipPath)
		if err != nil {
			slog.Warn("stat plugin zip file failed", "plugin", plugin.EntryPath, "error", err)
			continue
		}

		currentMtime := info.ModTime().Format(time.RFC3339)
		if plugin.FileModTime != "" && currentMtime != plugin.FileModTime {
			slog.Info("plugin file changed, triggering hot reload",
				"plugin", plugin.EntryPath,
				"oldMtime", plugin.FileModTime,
				"newMtime", currentMtime,
			)
			if err := h.ReloadPlugin(ctx, plugin.ID); err != nil {
				slog.Error("hot reload failed after file change", "plugin", plugin.EntryPath, "error", err)
			}
		}
	}
}
