package jsplugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"songloft/internal/database"
	"songloft/internal/jsruntime"
	"songloft/internal/models"
	"songloft/internal/services"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/singleflight"
)

// 懒加载 / EnsureLoaded 的语义错误。路由层据此区分 4xx/5xx。
var (
	// ErrPluginDisabled 插件被用户主动禁用（DB status=inactive）
	ErrPluginDisabled = errors.New("jsplugin: plugin disabled")
	// ErrPluginNotFound 插件不存在
	ErrPluginNotFound = errors.New("jsplugin: plugin not found")
	// ErrPluginErrorState 插件处于 error 状态且尚未到自愈窗口
	ErrPluginErrorState = errors.New("jsplugin: plugin in error state")
)

// Manager 是 JS 插件系统的入口和协调器
type Manager struct {
	repo            Repository
	packager        *PackageManager // 用于启动时从本地 zip 文件重建插件记录
	scheduler       *ServiceScheduler
	jsManager       *jsruntime.JSEnvManager
	services        sync.Map // map[string]*JSService (entryPath -> service)
	pluginsDir      string   // data/jsplugins/
	pluginsDataDir  string   // data/jsplugins_data/
	basePath        string   // URL 基础路径，用于反向代理子路径部署
	router          chi.Router
	db              database.DB               // 数据库访问
	authService     *services.AuthService     // 用于生成插件 JWT Token
	songDownloader  *services.SongDownloader  // 歌曲下载服务（bridge songs.download 用）
	songService     *services.SongService     // 歌曲服务（bridge songs.create/update/delete 用）
	playlistService *services.PlaylistService // 歌单服务（bridge playlists.* 写操作用）
	pluginToken     string                    // 插件专用的永久 JWT Token（启动时生成一次）
	port            string                    // 服务器监听端口
	healthChecker   *HealthChecker
	hotReloader     *HotReloader
	cancelFunc      context.CancelFunc
	mu              sync.RWMutex
	// loadGroup 对懒加载/恢复加载按 entryPath 去重并发，
	// 避免同一插件因高并发请求被并行 LoadPlugin 多次（hash 反复校验、scheduler 重复注册等）。
	loadGroup            singleflight.Group
	publicPathPrefixes   []string // 无需 JWT 的路径前缀（通过 RefreshPublicPaths 从 DB 加载，运行时可刷新）
	playEventSubscribers sync.Map // 已订阅播放事件的插件 entryPath 集合
	lyricProviders       sync.Map // 已注册为歌词提供者的插件 entryPath 集合
}

// NewManager 创建 JS 插件管理器
func NewManager(repo Repository, pluginsDir, pluginsDataDir, basePath string, router chi.Router, db database.DB) *Manager {
	m := &Manager{
		repo:           repo,
		packager:       NewPackageManager(pluginsDir, pluginsDataDir, repo),
		scheduler:      NewServiceScheduler(1),
		jsManager:      jsruntime.NewJSEnvManager(),
		pluginsDir:     pluginsDir,
		pluginsDataDir: pluginsDataDir,
		basePath:       basePath,
		router:         router,
		db:             db,
	}
	return m
}

// Packager 返回内部的 PackageManager（供 handlers 复用，避免重复构造）
func (m *Manager) Packager() *PackageManager {
	return m.packager
}

// DetectPluginIcon 检测插件 static 目录中的图标文件。
// 支持精确匹配和带 content hash 指纹的文件名（如 icon.svg → icon.abc123.svg）。
func (m *Manager) DetectPluginIcon(entryPath string) string {
	return detectIconInDir(filepath.Join(m.pluginsDataDir, entryPath, "static"))
}

// ResolvePluginIcon 将 manifest 中声明的 icon 文件名解析为实际磁盘上的文件名。
// 插件构建工具会给静态文件加 content hash 指纹（icon.svg → icon.abc123.svg），
// 此方法匹配带指纹的真实文件名。若精确匹配或 glob 匹配不到则返回空串。
func (m *Manager) ResolvePluginIcon(entryPath, declaredIcon string) string {
	staticDir := filepath.Join(m.pluginsDataDir, entryPath, "static")
	target := filepath.Join(staticDir, declaredIcon)
	if _, err := os.Stat(target); err == nil {
		return declaredIcon
	}
	ext := filepath.Ext(declaredIcon)
	base := strings.TrimSuffix(declaredIcon, ext)
	pattern := filepath.Join(staticDir, base+".*"+ext)
	matches, err := filepath.Glob(pattern)
	if err == nil && len(matches) > 0 {
		return filepath.Base(matches[0])
	}
	return ""
}

func detectIconInDir(dir string) string {
	for _, ext := range []string{".svg", ".png", ".webp"} {
		pattern := filepath.Join(dir, "icon"+ext)
		if _, err := os.Stat(pattern); err == nil {
			return "icon" + ext
		}
		globPattern := filepath.Join(dir, "icon.*"+ext)
		matches, _ := filepath.Glob(globPattern)
		if len(matches) > 0 {
			return filepath.Base(matches[0])
		}
	}
	return ""
}

// SetAuthService 设置认证服务和端口，并生成插件专用的永久 JWT Token（参考 WASM 插件的做法，启动时只生成一次）
func (m *Manager) SetAuthService(authService *services.AuthService, port string) {
	m.authService = authService
	m.port = port

	// 生成插件专用的永久 JWT Token
	if authService != nil {
		token, err := authService.GeneratePluginToken(context.Background())
		if err != nil {
			slog.Error("生成 JS 插件 JWT Token 失败", "error", err)
			return
		}
		m.pluginToken = token
		slog.Info("JS 插件 JWT Token 已生成")
	}
}

// SetSongDownloader 注入歌曲下载服务（bridge songs.download 调用）。
func (m *Manager) SetSongDownloader(d *services.SongDownloader) {
	m.songDownloader = d
}

// SetServices 注入歌曲和歌单服务（bridge songs/playlists 写操作调用）。
func (m *Manager) SetServices(songService *services.SongService, playlistService *services.PlaylistService) {
	m.songService = songService
	m.playlistService = playlistService
}

// Start 启动 JS 插件管理器（清理旧数据 → 重建 → 加载插件 → 健康检查 → 热更新监控）
func (m *Manager) Start(ctx context.Context) error {
	// 创建 HealthChecker 和 HotReloader
	m.healthChecker = NewHealthChecker(m)
	m.hotReloader = NewHotReloader(m)

	// 创建内部 context
	internalCtx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel

	// 从本地 zip 文件同步插件记录：
	//   - 新发现 zip → InstallFromUpload（强制 manifest hash 校验）
	//   - 已有记录但 zipHash 不一致 → Update（重新计算规范化 hash）
	//   - 数据库有但 zip 文件不在 → 删除孤儿记录
	// SyncPluginsFromDirectory 返回完整的插件列表，直接复用，
	// 避免再次调用 repo.GetAll() 引发与 WASM 插件管理器的 SQLITE_BUSY 锁竞争。
	synced, err := m.packager.SyncPluginsFromDirectory()
	if err != nil {
		slog.Error("sync js plugins from directory failed", "error", err)
		// 同步失败不阻断启动
	} else {
		slog.Info("js plugins synced from directory", "count", len(synced))
	}

	// 直接使用 Sync 返回的完整列表加载插件，无需再查 DB
	m.loadPlugins(internalCtx, synced)

	// Sync 可能新增/更新了带 publicPaths 的插件，刷新内存缓存
	m.RefreshPublicPaths()

	// 打印插件的静态页面访问 URL（基于 synced 列表，不依赖插件是否 active）
	m.logPluginStaticURLs(synced)

	// 启动健康检查
	m.healthChecker.Start(internalCtx)

	// 启动热更新文件监控
	go m.hotReloader.WatchForChanges(internalCtx)

	return nil
}

// LoadAll 加载所有 active 状态的插件（从 DB 读取，用于热重载等场景）
func (m *Manager) LoadAll(ctx context.Context) error {
	plugins, err := m.repo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("get all js plugins: %w", err)
	}

	m.loadPlugins(ctx, plugins)
	return nil
}

// loadPlugins 加载给定列表中所有 active 状态的插件（内部方法，避免重复查 DB）
func (m *Manager) loadPlugins(ctx context.Context, plugins []*JSPlugin) {
	for _, plugin := range plugins {
		if plugin.Status != JSPluginStatusActive {
			continue
		}
		if err := m.LoadPlugin(ctx, plugin); err != nil {
			slog.Error("failed to load js plugin", "entryPath", plugin.EntryPath, "error", err)
			// 标记为错误状态
			_ = m.repo.UpdateStatus(ctx, plugin.ID, JSPluginStatusError)
			continue
		}
		slog.Info("js plugin loaded", "entryPath", plugin.EntryPath, "name", plugin.Name)
	}
}

// LoadPlugin 加载单个插件
func (m *Manager) LoadPlugin(ctx context.Context, plugin *JSPlugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 确保插件数据目录存在
	dataDir := m.pluginsDataDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create plugins data dir: %w", err)
	}

	// 1. 创建 JSService
	service := NewJSService(plugin, m.scheduler, m.jsManager)

	// 2. 创建并关联 BridgeHandler
	bridgeHandler := NewBridgeHandler(service, dataDir, m.db, m.songDownloader, m.songService, m.playlistService, m.pluginToken, m.getPort())
	bridgeHandler.onPlayEventRegister = m.RegisterPlayEvent
	bridgeHandler.onPlayEventUnregister = m.UnregisterPlayEvent
	bridgeHandler.onLyricProviderRegister = m.RegisterLyricProvider
	bridgeHandler.onLyricProviderUnregister = m.UnregisterLyricProvider
	service.bridgeHandler = bridgeHandler

	// 3. 加载插件（读取 ZIP、校验 hash、创建 JS 环境）
	if err := service.Load(m.pluginsDir, dataDir); err != nil {
		return fmt.Errorf("load plugin %q: %w", plugin.EntryPath, err)
	}

	// 4. 更新 DB 中的 hash
	if err := m.repo.UpdateHashes(ctx, plugin.ID, plugin.ZipHash, plugin.EntryHash, plugin.FileModTime); err != nil {
		slog.Warn("update plugin hashes failed", "entryPath", plugin.EntryPath, "error", err)
	}

	// 5. 在 scheduler 中注册 service
	if err := m.scheduler.RegisterService(plugin.EntryPath, service, 0); err != nil {
		return fmt.Errorf("register service %q: %w", plugin.EntryPath, err)
	}

	// 6. 调用 service.Init()
	if err := service.Init(); err != nil {
		slog.Warn("plugin init failed", "entryPath", plugin.EntryPath, "error", err)
		// Init 失败不算致命错误，插件仍可响应请求
	}

	// 7. 存入 services map
	m.services.Store(plugin.EntryPath, service)

	return nil
}

// UnloadPlugin 卸载单个插件
func (m *Manager) UnloadPlugin(ctx context.Context, entryPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	value, ok := m.services.Load(entryPath)
	if !ok {
		return fmt.Errorf("service %q not found", entryPath)
	}

	service := value.(*JSService)

	// 从 scheduler 注销
	if err := m.scheduler.UnregisterService(entryPath, 10*time.Second); err != nil {
		slog.Warn("unregister service failed", "entryPath", entryPath, "error", err)
	}

	// 停止服务
	if err := service.Stop(); err != nil {
		slog.Warn("stop service failed", "entryPath", entryPath, "error", err)
	}

	// 从 map 中移除
	m.services.Delete(entryPath)

	// 清理播放事件订阅
	m.playEventSubscribers.Delete(entryPath)
	// 清理歌词提供者注册
	m.lyricProviders.Delete(entryPath)

	return nil
}

// ReloadPlugin 重载插件（unload + 清除字节码缓存 + load）
func (m *Manager) ReloadPlugin(ctx context.Context, entryPath string) error {
	// 先获取插件信息
	plugin, err := m.repo.GetByEntryPath(ctx, entryPath)
	if err != nil {
		return fmt.Errorf("get plugin by entry_path %q: %w", entryPath, err)
	}

	// 卸载（忽略 not found 错误，因为可能尚未加载）
	_ = m.UnloadPlugin(ctx, entryPath)

	// 清除字节码缓存（强制下次加载时重新编译）
	cacheDir := filepath.Join(m.pluginsDataDir, entryPath, "cache")
	os.RemoveAll(cacheDir)

	// 重新加载
	if err := m.LoadPlugin(ctx, plugin); err != nil {
		return err
	}
	m.RefreshPublicPaths()
	return nil
}

// EnablePlugin 启用插件（更新状态 + 加载）
func (m *Manager) EnablePlugin(ctx context.Context, id int64) error {
	plugin, err := m.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get plugin by id %d: %w", id, err)
	}

	// 更新数据库状态
	if err := m.repo.UpdateStatus(ctx, id, JSPluginStatusActive); err != nil {
		return fmt.Errorf("update plugin status: %w", err)
	}

	// 加载插件
	if err := m.LoadPlugin(ctx, plugin); err != nil {
		// 加载失败，回滚状态
		_ = m.repo.UpdateStatus(ctx, id, JSPluginStatusError)
		return fmt.Errorf("load plugin: %w", err)
	}

	// 用户主动开启视为对此插件的"信任重置"：清空自愈退避计数，
	// 万一后续再次进入 error 状态时从最短的 1 分钟重新开始尝试，而不是沿用上次的长延迟。
	if m.healthChecker != nil {
		m.healthChecker.ClearRecovery(plugin.EntryPath)
	}

	m.RefreshPublicPaths()
	return nil
}

// DisablePlugin 禁用插件（卸载 + 更新状态）
func (m *Manager) DisablePlugin(ctx context.Context, id int64) error {
	plugin, err := m.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get plugin by id %d: %w", id, err)
	}

	// 卸载插件
	_ = m.UnloadPlugin(ctx, plugin.EntryPath)

	// 清理自愈/唤醒计划，避免 disable 后被自动恢复或唤醒。
	if m.healthChecker != nil {
		m.healthChecker.cancelWakeup(plugin.EntryPath)
		m.healthChecker.ClearRecovery(plugin.EntryPath)
	}

	// 更新数据库状态
	if err := m.repo.UpdateStatus(ctx, id, JSPluginStatusInactive); err != nil {
		return fmt.Errorf("update plugin status: %w", err)
	}

	m.RefreshPublicPaths()
	return nil
}

// GetService 获取运行中的服务
func (m *Manager) GetService(entryPath string) (*JSService, bool) {
	value, ok := m.services.Load(entryPath)
	if !ok {
		return nil, false
	}
	return value.(*JSService), true
}

// EnsureLoaded 确保插件已加载并返回其 Service。
//
// 用于请求路径的"按需懒加载"：
//   - services 中已有 → 直接返回。
//   - DB 中存在且 status=active 但 services 缺失（被 idle eviction 卸载） → 触发 LoadPlugin 后返回。
//   - status=inactive → 返回 ErrPluginDisabled，路由响应 403。
//   - status=error → 返回 ErrPluginErrorState，路由响应 503，由 HealthChecker 的指数退避自愈机制负责恢复。
//   - DB 中不存在 → 返回 ErrPluginNotFound。
//
// 并发去重：用 singleflight 按 entryPath 合并同时进入的多次 LoadPlugin，
// 防止 50 个请求并发触发空闲驱逐插件时同时跑 50 次 hash 校验/scheduler 注册。
func (m *Manager) EnsureLoaded(ctx context.Context, entryPath string) (*JSService, error) {
	// 快速路径：已加载直接返回。
	if svc, ok := m.GetService(entryPath); ok {
		return svc, nil
	}

	// singleflight 去重：同 key 的并发只跑一次 fn，其余等待复用结果。
	v, err, _ := m.loadGroup.Do(entryPath, func() (any, error) {
		// 双检查：抢到 singleflight 之前可能已被另一个请求加载完。
		if svc, ok := m.GetService(entryPath); ok {
			return svc, nil
		}

		plugin, err := m.repo.GetByEntryPath(ctx, entryPath)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrPluginNotFound
			}
			return nil, fmt.Errorf("get plugin by entry_path %q: %w", entryPath, err)
		}
		if plugin == nil {
			return nil, ErrPluginNotFound
		}

		switch plugin.Status {
		case JSPluginStatusInactive:
			return nil, ErrPluginDisabled
		case JSPluginStatusError:
			return nil, ErrPluginErrorState
		case JSPluginStatusActive:
			// 期望路径：DB active 但未加载（idle eviction）→ 重新加载。
			start := time.Now()
			if err := m.LoadPlugin(ctx, plugin); err != nil {
				return nil, fmt.Errorf("lazy load plugin %q: %w", entryPath, err)
			}
			slog.Info("plugin lazy loaded on demand",
				"plugin", entryPath,
				"costMs", time.Since(start).Milliseconds(),
			)
			svc, ok := m.GetService(entryPath)
			if !ok {
				return nil, fmt.Errorf("plugin %q lazy-loaded but service still missing", entryPath)
			}
			return svc, nil
		default:
			return nil, fmt.Errorf("plugin %q has unknown status %q", entryPath, plugin.Status)
		}
	})
	if err != nil {
		return nil, err
	}
	return v.(*JSService), nil
}

// ListServices 列出所有运行中的服务
func (m *Manager) ListServices() []*JSService {
	var result []*JSService
	m.services.Range(func(key, value interface{}) bool {
		result = append(result, value.(*JSService))
		return true
	})
	return result
}

// RegisterPlayEvent 将插件注册为播放事件订阅者
func (m *Manager) RegisterPlayEvent(entryPath string) {
	m.playEventSubscribers.Store(entryPath, true)
	slog.Info("plugin registered for play events", "plugin", entryPath)
}

// UnregisterPlayEvent 取消插件的播放事件订阅
func (m *Manager) UnregisterPlayEvent(entryPath string) {
	m.playEventSubscribers.Delete(entryPath)
	slog.Info("plugin unregistered from play events", "plugin", entryPath)
}

// RegisterLyricProvider 将插件注册为歌词提供者
func (m *Manager) RegisterLyricProvider(entryPath string) {
	m.lyricProviders.Store(entryPath, true)
	slog.Info("plugin registered as lyric provider", "plugin", entryPath)
}

// UnregisterLyricProvider 取消插件的歌词提供者注册
func (m *Manager) UnregisterLyricProvider(entryPath string) {
	m.lyricProviders.Delete(entryPath)
	slog.Info("plugin unregistered as lyric provider", "plugin", entryPath)
}

// SearchLyrics 遍历已注册的歌词提供者插件，通过 InvokeHTTP 调用其 /lyric-search 端点。
// 第一个返回非空歌词的结果即停止。
func (m *Manager) SearchLyrics(ctx context.Context, title, artist, album string, duration float64) (*models.LyricPayload, error) {
	query := url.Values{
		"title":    {title},
		"artist":   {artist},
		"album":    {album},
		"duration": {strconv.FormatFloat(duration, 'f', 1, 64)},
	}

	var result *models.LyricPayload
	m.lyricProviders.Range(func(key, _ interface{}) bool {
		entryPath := key.(string)
		searchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		statusCode, _, body, err := m.InvokeHTTP(searchCtx, entryPath, "GET", "/lyric-search", query, nil)
		if err != nil {
			slog.Debug("lyric search failed", "plugin", entryPath, "error", err)
			return true
		}
		if statusCode != http.StatusOK || len(body) == 0 {
			return true
		}

		var payload models.LyricPayload
		if err := json.Unmarshal(body, &payload); err != nil || payload.IsEmpty() {
			return true
		}
		result = &payload
		return false
	})

	if result != nil {
		return result, nil
	}
	return nil, errors.New("no lyrics found from any provider")
}

// BroadcastPlayEvent 向所有已订阅的插件异步发送播放事件
func (m *Manager) BroadcastPlayEvent(song *PlayEventSong, eventType, source string) {
	eventData := &PlayEventData{
		Type:      eventType,
		Song:      *song,
		Source:    source,
		Timestamp: time.Now().UnixMilli(),
	}

	m.playEventSubscribers.Range(func(key, _ interface{}) bool {
		entryPath := key.(string)
		if err := m.scheduler.Send(&Message{
			Type:   MsgPlayEvent,
			Target: entryPath,
			Data:   eventData,
		}); err != nil {
			slog.Debug("broadcast play event: send failed",
				"plugin", entryPath, "error", err)
		}
		return true
	})
}

// Close 关闭管理器（停止所有服务）
func (m *Manager) Close() error {
	// 先广播 JS 关闭信号：让 ExecuteJSAndWaitEvents（批量加载音源时的 polling）
	// 等阻塞操作尽快退出，释放 env.mu，避免后续 service.Stop → Deinit → ExecuteJS
	// 死等主 env 锁，导致 Ctrl+C 卡死。
	if m.jsManager != nil {
		m.jsManager.SignalShutdown()
	}

	// 停止健康检查
	if m.healthChecker != nil {
		m.healthChecker.Stop()
	}

	// 取消内部 context（停止热更新监控等后台 goroutine）
	if m.cancelFunc != nil {
		m.cancelFunc()
	}

	// 停止所有服务
	m.services.Range(func(key, value interface{}) bool {
		entryPath := key.(string)
		service := value.(*JSService)

		// 从 scheduler 注销
		_ = m.scheduler.UnregisterService(entryPath, 5*time.Second)

		// 停止服务
		_ = service.Stop()

		m.services.Delete(key)
		return true
	})

	// 关闭 scheduler
	if err := m.scheduler.Close(); err != nil {
		slog.Warn("close scheduler failed", "error", err)
	}

	// 关闭 JS 运行时管理器
	if m.jsManager != nil {
		if err := m.jsManager.Close(); err != nil {
			slog.Warn("close js manager failed", "error", err)
		}
	}

	slog.Info("JS plugin manager closed")
	return nil
}

// logPluginStaticURLs 打印插件的静态页面访问 URL（含 access_token）
// 基于 synced 列表遍历，不依赖插件是否 active（静态文件访问不需要插件运行）
func (m *Manager) logPluginStaticURLs(plugins []*JSPlugin) {
	if len(plugins) == 0 {
		return
	}

	port := m.getPort()

	for _, plugin := range plugins {
		// 检查插件是否有 static/index.html
		staticIndex := filepath.Join(m.pluginsDataDir, plugin.EntryPath, "static", "index.html")
		if _, err := os.Stat(staticIndex); err != nil {
			continue
		}

		url := fmt.Sprintf("http://localhost:%s/api/v1/jsplugin/%s", port, plugin.EntryPath)
		if m.pluginToken != "" {
			url += "?access_token=" + m.pluginToken
		}
		slog.Info("JS plugin static page available", "plugin", plugin.EntryPath, "url", url)
	}
}

// getPort 返回服务器监听端口，未配置时使用默认端口
func (m *Manager) getPort() string {
	if m.port != "" {
		return m.port
	}
	return "58091"
}
