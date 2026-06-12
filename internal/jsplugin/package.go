package jsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"songloft/internal/httputil"
)

// UpdateInfo 远程更新信息
type UpdateInfo struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	DownloadURL    string `json:"download_url,omitempty"`
	ChangeLog      string `json:"change_log,omitempty"`
}

// PackageManager 管理 .jsplugin.zip 的安装、更新、发现
type PackageManager struct {
	pluginsDir string // data/jsplugins/ 存放 zip 的目录
	dataDir    string // 插件数据目录（static 解压位置）
	repo       Repository
}

// NewPackageManager 构造函数
func NewPackageManager(pluginsDir, dataDir string, repo Repository) *PackageManager {
	return &PackageManager{
		pluginsDir: pluginsDir,
		dataDir:    dataDir,
		repo:       repo,
	}
}

// InstallFromUpload 从上传的字节安装插件
// 流程：解析 ZIP → 读 plugin.json → 验证 manifest → 计算双层 hash → 保存 ZIP 到 pluginsDir → 写入数据库
// 返回值 wasUpdate=true 表示 entryPath 已存在，本次是覆盖更新（保留原 ID 与状态）。
func (pm *PackageManager) InstallFromUpload(zipData []byte) (*JSPlugin, bool, error) {
	// [1] 解析 plugin.json
	manifest, err := readPluginManifestFromZip(zipData)
	if err != nil {
		return nil, false, fmt.Errorf("read manifest from zip: %w", err)
	}

	// [2] 验证 manifest
	if err := ValidateManifest(manifest); err != nil {
		return nil, false, fmt.Errorf("validate manifest: %w", err)
	}

	// [3] 验证权限
	if err := ValidatePermissions(manifest.Permissions); err != nil {
		return nil, false, fmt.Errorf("validate permissions: %w", err)
	}

	// [4] 检查 entryPath 是否已存在：存在则走更新路径，作为手动更新
	ctx := context.Background()
	existing, err := pm.repo.GetByEntryPath(ctx, manifest.EntryPath)
	if err == nil && existing != nil {
		updated, updErr := pm.Update(existing.ID, zipData)
		if updErr != nil {
			return nil, false, updErr
		}
		return updated, true, nil
	}

	// [5] 读取入口文件并计算 entry_hash
	entryContent, _, err := readEntryFromZip(zipData, manifest.Main)
	if err != nil {
		return nil, false, fmt.Errorf("read entry from zip: %w", err)
	}
	entryHash := sha256Hex(entryContent)

	// [6] 规范化计算 zip_hash（排除 plugin.json 自身）。
	zipHash, err := ComputeCanonicalZipHash(zipData)
	if err != nil {
		return nil, false, fmt.Errorf("compute canonical zip hash: %w", err)
	}

	// [6.1] 静态校验：manifest 内声明的 hash 必须与实际内容一致。
	if manifest.EntryHash != entryHash {
		return nil, false, fmt.Errorf("%w: entryHash declared=%s actual=%s", ErrManifestHashMismatch, manifest.EntryHash, entryHash)
	}
	if manifest.ZipHash != zipHash {
		return nil, false, fmt.Errorf("%w: zipHash declared=%s actual=%s", ErrManifestHashMismatch, manifest.ZipHash, zipHash)
	}

	// [7] 确保 pluginsDir 存在
	if err := os.MkdirAll(pm.pluginsDir, 0o755); err != nil {
		return nil, false, fmt.Errorf("create plugins dir: %w", err)
	}

	// [8] 保存 ZIP 到 pluginsDir
	zipFileName := manifest.EntryPath + ".jsplugin.zip"
	zipFilePath := filepath.Join(pm.pluginsDir, zipFileName)
	if err := os.WriteFile(zipFilePath, zipData, 0o644); err != nil {
		return nil, false, fmt.Errorf("write zip file: %w", err)
	}

	// [9] 获取文件修改时间
	info, err := os.Stat(zipFilePath)
	if err != nil {
		return nil, false, fmt.Errorf("stat zip file: %w", err)
	}
	fileModTime := info.ModTime().Format(time.RFC3339)

	// [10] 解压 static/ 到 dataDir
	staticDir := filepath.Join(pm.dataDir, manifest.EntryPath, "static")
	if err := extractStaticFromZip(zipData, staticDir); err != nil {
		slog.Warn("extract static files failed (non-fatal)", "plugin", manifest.EntryPath, "error", err)
	}

	// [11] 构建 JSPlugin 对象
	plugin := &JSPlugin{
		Name:           manifest.Name,
		Version:        manifest.Version,
		Description:    manifest.Description,
		Author:         manifest.Author,
		Homepage:       manifest.Homepage,
		License:        manifest.License,
		EntryPath:      manifest.EntryPath,
		Main:           manifest.Main,
		MinHostVersion: manifest.MinHostVersion,
		Permissions:    manifest.Permissions,
		PublicPaths:    manifest.PublicPaths,
		ExternalPaths:  manifest.ExternalPaths,
		Icon:           manifest.Icon,
		UpdateURL:      manifest.UpdateURL,
		DownloadURL:    manifest.DownloadURL,
		Status:         JSPluginStatusActive,
		ZipHash:        zipHash,
		EntryHash:      entryHash,
		FileModTime:    fileModTime,
		FilePath:       zipFileName,
	}

	// [12] 写入数据库
	if err := pm.repo.Create(ctx, plugin); err != nil {
		// 回滚：删除已保存的 ZIP 文件
		_ = os.Remove(zipFilePath)
		return nil, false, fmt.Errorf("create plugin record: %w", err)
	}

	slog.Info("plugin installed", "entryPath", plugin.EntryPath, "version", plugin.Version)
	return plugin, false, nil
}

// InstallFromFile 从本地文件路径安装
func (pm *PackageManager) InstallFromFile(zipPath string) (*JSPlugin, bool, error) {
	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		return nil, false, fmt.Errorf("read zip file %q: %w", zipPath, err)
	}
	return pm.InstallFromUpload(zipData)
}

// Update 更新已有插件
// 流程：解析新 ZIP → 验证 → 覆盖旧 ZIP → 更新数据库
func (pm *PackageManager) Update(pluginID int64, zipData []byte) (*JSPlugin, error) {
	ctx := context.Background()

	// [1] 获取已有插件
	existing, err := pm.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, fmt.Errorf("get plugin by id %d: %w", pluginID, err)
	}

	// [2] 解析新 ZIP 中的 plugin.json
	manifest, err := readPluginManifestFromZip(zipData)
	if err != nil {
		return nil, fmt.Errorf("read manifest from zip: %w", err)
	}

	// [3] 验证 manifest
	if err := ValidateManifest(manifest); err != nil {
		return nil, fmt.Errorf("validate manifest: %w", err)
	}

	// [4] 验证权限
	if err := ValidatePermissions(manifest.Permissions); err != nil {
		return nil, fmt.Errorf("validate permissions: %w", err)
	}

	// [5] entryPath 必须匹配
	if manifest.EntryPath != existing.EntryPath {
		return nil, fmt.Errorf("entryPath mismatch: existing=%q, new=%q", existing.EntryPath, manifest.EntryPath)
	}

	// [6] 读取入口文件并计算 hash（规范化 zip_hash 排除 plugin.json）。
	entryContent, _, err := readEntryFromZip(zipData, manifest.Main)
	if err != nil {
		return nil, fmt.Errorf("read entry from zip: %w", err)
	}
	entryHash := sha256Hex(entryContent)
	zipHash, err := ComputeCanonicalZipHash(zipData)
	if err != nil {
		return nil, fmt.Errorf("compute canonical zip hash: %w", err)
	}

	// [6.1] 静态校验：manifest 内声明的 hash 必须与实际内容一致。
	if manifest.EntryHash != entryHash {
		return nil, fmt.Errorf("%w: entryHash declared=%s actual=%s", ErrManifestHashMismatch, manifest.EntryHash, entryHash)
	}
	if manifest.ZipHash != zipHash {
		return nil, fmt.Errorf("%w: zipHash declared=%s actual=%s", ErrManifestHashMismatch, manifest.ZipHash, zipHash)
	}

	// [7] 覆盖旧 ZIP 文件
	zipFilePath := filepath.Join(pm.pluginsDir, existing.FilePath)
	if err := os.WriteFile(zipFilePath, zipData, 0o644); err != nil {
		return nil, fmt.Errorf("write zip file: %w", err)
	}

	// [8] 获取文件修改时间
	info, err := os.Stat(zipFilePath)
	if err != nil {
		return nil, fmt.Errorf("stat zip file: %w", err)
	}
	fileModTime := info.ModTime().Format(time.RFC3339)

	// [9] 重新解压 static/
	staticDir := filepath.Join(pm.dataDir, existing.EntryPath, "static")
	// 清理旧的 static 目录
	_ = os.RemoveAll(staticDir)
	if err := extractStaticFromZip(zipData, staticDir); err != nil {
		slog.Warn("extract static files failed (non-fatal)", "plugin", existing.EntryPath, "error", err)
	}

	// [10] 更新数据库记录
	existing.Name = manifest.Name
	existing.Version = manifest.Version
	existing.Description = manifest.Description
	existing.Author = manifest.Author
	existing.Homepage = manifest.Homepage
	existing.License = manifest.License
	existing.Main = manifest.Main
	existing.MinHostVersion = manifest.MinHostVersion
	existing.Permissions = manifest.Permissions
	existing.PublicPaths = manifest.PublicPaths
	existing.ExternalPaths = manifest.ExternalPaths
	existing.Icon = manifest.Icon
	existing.UpdateURL = manifest.UpdateURL
	existing.DownloadURL = manifest.DownloadURL
	existing.ZipHash = zipHash
	existing.EntryHash = entryHash
	existing.FileModTime = fileModTime

	if err := pm.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update plugin record: %w", err)
	}

	slog.Info("plugin updated", "entryPath", existing.EntryPath, "version", existing.Version)
	return existing, nil
}

// Uninstall 卸载插件（删除 ZIP + 数据库记录 + static 目录）
func (pm *PackageManager) Uninstall(pluginID int64) error {
	ctx := context.Background()

	// [1] 获取插件信息
	plugin, err := pm.repo.GetByID(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("get plugin by id %d: %w", pluginID, err)
	}

	// [2] 删除 ZIP 文件
	zipFilePath := filepath.Join(pm.pluginsDir, plugin.FilePath)
	if err := os.Remove(zipFilePath); err != nil && !os.IsNotExist(err) {
		slog.Warn("remove zip file failed", "path", zipFilePath, "error", err)
	}

	// [3] 删除 static 目录
	staticDir := filepath.Join(pm.dataDir, plugin.EntryPath)
	if err := os.RemoveAll(staticDir); err != nil {
		slog.Warn("remove static dir failed", "path", staticDir, "error", err)
	}

	// [4] 删除数据库记录
	if err := pm.repo.Delete(ctx, pluginID); err != nil {
		return fmt.Errorf("delete plugin record: %w", err)
	}

	slog.Info("plugin uninstalled", "entryPath", plugin.EntryPath)
	return nil
}

// SyncPluginsFromDirectory 启动时扫描 pluginsDir 发现新/已有插件
// 对比数据库与目录，新 zip 自动安装，已有 zip 校验 hash
func (pm *PackageManager) SyncPluginsFromDirectory() ([]*JSPlugin, error) {
	ctx := context.Background()

	// [1] 确保目录存在
	if err := os.MkdirAll(pm.pluginsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create plugins dir: %w", err)
	}

	// [2] 扫描目录中的 .jsplugin.zip 文件
	entries, err := os.ReadDir(pm.pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("read plugins dir: %w", err)
	}

	// [3] 获取数据库中所有已知插件
	dbPlugins, err := pm.repo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all plugins from db: %w", err)
	}

	// 构建 entryPath -> plugin 映射
	dbMap := make(map[string]*JSPlugin, len(dbPlugins))
	for _, p := range dbPlugins {
		dbMap[p.EntryPath] = p
	}

	var result []*JSPlugin

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsplugin.zip") {
			continue
		}

		entryPath := EntryPathFromZipName(name)
		zipFilePath := filepath.Join(pm.pluginsDir, name)

		if existing, ok := dbMap[entryPath]; ok {
			// 已有插件：校验 hash
			zipData, err := os.ReadFile(zipFilePath)
			if err != nil {
				slog.Warn("read existing zip failed", "path", zipFilePath, "error", err)
				continue
			}
			currentHash, err := ComputeCanonicalZipHash(zipData)
			if err != nil {
				slog.Warn("compute canonical zip hash failed", "path", zipFilePath, "error", err)
				continue
			}
			if currentHash != existing.ZipHash {
				// Hash 不一致，可能是外部更新了 ZIP，执行更新
				slog.Info("zip hash mismatch, updating plugin", "entryPath", entryPath)
				updated, err := pm.Update(existing.ID, zipData)
				if err != nil {
					slog.Error("sync update failed", "entryPath", entryPath, "error", err)
					continue
				}
				result = append(result, updated)
			} else {
				// Hash 一致但 manifest 元数据（如 icon）可能变化（zipHash 排除 plugin.json）
				pm.syncManifestMetadata(ctx, existing, zipData)
				result = append(result, existing)
			}
			// 从 map 中移除已处理的
			delete(dbMap, entryPath)
		} else {
			// 新发现的 ZIP，自动安装
			slog.Info("discovered new plugin zip", "file", name)
			plugin, _, err := pm.InstallFromFile(zipFilePath)
			if err != nil {
				slog.Error("sync install failed", "file", name, "error", err)
				continue
			}
			result = append(result, plugin)
		}
	}

	// dbMap 中剩余的是数据库有记录但 ZIP 文件不存在的插件：
	// 开发期直接删除孤儿记录（连同 static/ 目录一起清理），保持 DB 与本地目录完全一致。
	for entryPath, plugin := range dbMap {
		zipFilePath := filepath.Join(pm.pluginsDir, plugin.FilePath)
		if _, err := os.Stat(zipFilePath); os.IsNotExist(err) {
			slog.Info("plugin zip missing, removing orphan record", "entryPath", entryPath)
			if err := pm.Uninstall(plugin.ID); err != nil {
				slog.Warn("remove orphan plugin failed", "entryPath", entryPath, "error", err)
			}
		}
	}

	return result, nil
}

// syncManifestMetadata 在 zipHash 匹配时比对 plugin.json 元数据字段，有变化则更新 DB。
// zipHash 排除 plugin.json，因此仅改 manifest（如新增 icon）不会触发 Update，需此方法补偿。
func (pm *PackageManager) syncManifestMetadata(ctx context.Context, existing *JSPlugin, zipData []byte) {
	manifest, err := readPluginManifestFromZip(zipData)
	if err != nil {
		return
	}
	dirty := false
	if existing.Icon != manifest.Icon {
		existing.Icon = manifest.Icon
		dirty = true
	}
	if existing.Name != manifest.Name {
		existing.Name = manifest.Name
		dirty = true
	}
	if existing.Description != manifest.Description {
		existing.Description = manifest.Description
		dirty = true
	}
	if existing.Homepage != manifest.Homepage {
		existing.Homepage = manifest.Homepage
		dirty = true
	}
	if existing.UpdateURL != manifest.UpdateURL {
		existing.UpdateURL = manifest.UpdateURL
		dirty = true
	}
	if dirty {
		if err := pm.repo.Update(ctx, existing); err != nil {
			slog.Warn("sync manifest metadata failed", "entryPath", existing.EntryPath, "error", err)
		} else {
			slog.Info("synced manifest metadata", "entryPath", existing.EntryPath)
		}
	}
}

// applyProxy 将代理前缀应用到 URL 上，与 handlers.applyGithubProxy 行为一致。
func applyProxy(rawURL, proxyPrefix string) string {
	if proxyPrefix == "" {
		return rawURL
	}
	if proxyPrefix[len(proxyPrefix)-1] != '/' {
		proxyPrefix += "/"
	}
	return proxyPrefix + rawURL
}

// CheckUpdate 检查远程更新（通过 plugin.json 中的 update_url 或 download_url）。
// githubProxy 用于在请求 update_url 与最终 download_url 前加上 GitHub 加速代理前缀。
func (pm *PackageManager) CheckUpdate(pluginID int64, githubProxy string) (*UpdateInfo, error) {
	ctx := context.Background()

	plugin, err := pm.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, fmt.Errorf("get plugin by id %d: %w", pluginID, err)
	}

	updateURL := plugin.UpdateURL
	if updateURL == "" {
		// 无更新检查 URL
		return &UpdateInfo{
			CurrentVersion: plugin.Version,
			LatestVersion:  plugin.Version,
			HasUpdate:      false,
		}, nil
	}

	// 请求远程更新信息（应用代理）
	requestURL := applyProxy(updateURL, githubProxy)
	client := httputil.NewClient(15 * time.Second)
	resp, err := client.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("fetch update info from %q: %w", requestURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update check returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read update response: %w", err)
	}

	// 解析远程 plugin.json 或更新信息
	var remoteManifest PluginManifest
	if err := json.Unmarshal(body, &remoteManifest); err != nil {
		return nil, fmt.Errorf("parse update info: %w", err)
	}

	hasUpdate := remoteManifest.Version != plugin.Version
	info := &UpdateInfo{
		CurrentVersion: plugin.Version,
		LatestVersion:  remoteManifest.Version,
		HasUpdate:      hasUpdate,
		DownloadURL:    applyProxy(remoteManifest.DownloadURL, githubProxy),
	}

	return info, nil
}

// DownloadUpdate 下载远程更新包并安装。
// githubProxy 同时用于检查更新与下载新 ZIP。
// force 为 true 时跳过版本比较，强制重新下载安装。
func (pm *PackageManager) DownloadUpdate(pluginID int64, githubProxy string, force bool) (*JSPlugin, error) {
	// [1] 先检查更新（已在内部将代理应用到 download_url 上）
	updateInfo, err := pm.CheckUpdate(pluginID, githubProxy)
	if err != nil {
		return nil, fmt.Errorf("check update: %w", err)
	}

	if !force && !updateInfo.HasUpdate {
		return nil, fmt.Errorf("no update available")
	}

	downloadURL := updateInfo.DownloadURL
	if downloadURL == "" {
		return nil, fmt.Errorf("no download URL available for update")
	}

	// [2] 下载新 ZIP
	client := httputil.NewClient(60 * time.Second)
	resp, err := client.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("download update from %q: %w", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read download body: %w", err)
	}

	// [3] 执行更新
	return pm.Update(pluginID, zipData)
}
