package app

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"songloft/internal/config"
	"songloft/internal/database"
	"songloft/internal/handlers"
	"songloft/internal/httputil"
	"songloft/internal/jsplugin"
	"songloft/internal/logging"
	"songloft/internal/models"
	"songloft/internal/services"
	"songloft/internal/services/playactivity"
	"songloft/internal/services/source"
	"songloft/internal/tracelycfg"
	"songloft/internal/version"

	"github.com/hanxi/tracely/sdk/go/tracely"

	"github.com/go-chi/chi/v5"
)

// App 应用程序结构
type App struct {
	config             *config.AppConfig
	router             *chi.Mux
	db                 database.DB
	configService      *services.ConfigService
	songService        *services.SongService
	playlistService    *services.PlaylistService
	authService        *services.AuthService
	upgradeService     *services.UpgradeService
	cacheService       *services.CacheService
	backupService      *services.BackupService
	urlResolver        *services.InternalURLResolver // 共享:把 JS 插件相对路径解析为本机绝对 URL + access_token
	lyricFetcher       *services.LyricFetcher        // 共享:解包插件歌词 JSON 拿 LRC 文本
	scanner            *services.Scanner
	autoScanner        *services.AutoScanner
	metadataExtractor  *services.MetadataExtractor
	jsPluginManager    *jsplugin.Manager
	sourceMetrics      *source.SourceMetrics
	sourceOrchestrator *source.SourceOrchestrator
	metadataRefresher  *services.MetadataRefresher
	downloadActivity   *services.DownloadActivity // 下载活动闸门，导入探测据此让路（issue #265）
	playActivity       *playactivity.Registry     // 跨 song/会话 cancel 的全局表，处理快速切歌时旧请求的让位（issue #79）
	webDist            embed.FS
	tracelyClient      *tracely.Client
	logLevelVar        *slog.LevelVar        // 全局 slog 等级动态切换；由 /settings/log-level 即时 Set
	logWriter          *logging.RotateWriter // 日志落盘 writer（<data_dir>/logs/），供 /logs/export 读取；stdout 采集不受影响
	server             *http.Server
}

// NewApp 创建新的应用程序实例
func NewApp(cfg *config.AppConfig, webDist embed.FS) *App {
	router := chi.NewRouter()

	return &App{
		config:  cfg,
		router:  router,
		webDist: webDist,
	}
}

// Close 关闭应用程序资源
func (a *App) Close() error {
	// 关闭 JS 插件管理器（健康检查 + 热更新 + 所有服务）
	if a.jsPluginManager != nil {
		a.jsPluginManager.Close()
	}
	if a.autoScanner != nil {
		a.autoScanner.Stop()
	}
	if a.authService != nil {
		a.authService.Close()
	}
	if a.db != nil {
		slog.Info("关闭数据库连接")
		err := a.db.Close()
		if a.logWriter != nil {
			_ = a.logWriter.Close()
		}
		return err
	}
	if a.logWriter != nil {
		_ = a.logWriter.Close()
	}
	return nil
}

// Shutdown 优雅关闭 HTTP 服务器并释放所有资源
func (a *App) Shutdown(ctx context.Context) error {
	if a.server != nil {
		if err := a.server.Shutdown(ctx); err != nil {
			slog.Warn("HTTP server shutdown error", "error", err)
		}
	}
	return a.Close()
}

func (a *App) Init() error {
	// 初始化 slog：用 LevelVar 让 /settings/log-level 可在运行时切换等级。
	// 默认 LevelInfo（与旧的 nil HandlerOptions 行为一致）；DB 中持久化的等级在 configService 就绪后再 apply。
	a.logLevelVar = new(slog.LevelVar)
	// 日志同时写 stdout（Docker/systemd 采集）与轮转文件（供 /logs/export 导出给用户提交 issue）。
	// 落盘目录随 data 目录派生：<data_dir>/logs/。落盘 writer 创建失败时降级为仅 stdout，不阻塞启动。
	var logOut io.Writer = os.Stdout
	logDir := filepath.Join(filepath.Dir(a.config.DBPath), "logs")
	if lw, err := logging.NewRotateWriter(logDir, 0, 0); err != nil {
		slog.New(slog.NewTextHandler(os.Stdout, nil)).Warn("日志落盘初始化失败，降级为仅 stdout", "dir", logDir, "error", err)
	} else {
		a.logWriter = lw
		logOut = io.MultiWriter(os.Stdout, lw)
	}
	logger := slog.New(slog.NewTextHandler(logOut, &slog.HandlerOptions{Level: a.logLevelVar}))
	slog.SetDefault(logger)

	// 确保数据库目录存在
	dbDir := filepath.Dir(a.config.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 兼容老安装（一次性）：若目标 DB 不存在但同目录下有 mimusic.db，则自动 rename。
	// 这是 MiMusic → Songloft v2.0 重命名中唯一保留的兼容点。
	if err := migrateLegacyDB(a.config.DBPath); err != nil {
		return fmt.Errorf("迁移老数据库失败: %w", err)
	}

	// 初始化数据库
	db, err := database.Open(a.config.DBPath)
	if err != nil {
		return fmt.Errorf("数据库初始化失败: %w", err)
	}
	a.db = db
	slog.Info("数据库初始化成功", "path", a.config.DBPath)

	// 创建配置服务
	configRepo := db.ConfigRepository()
	a.configService = services.NewConfigService(configRepo)

	// 应用持久化的日志等级（缺失时保持默认 LevelInfo）
	if levelStr := a.configService.GetString("log_level", "info"); levelStr != "" {
		if lvl, ok := handlers.ParseLogLevel(levelStr); ok {
			a.logLevelVar.Set(lvl)
			slog.Info("日志等级已应用", "level", levelStr)
		} else {
			slog.Warn("日志等级配置无效，使用默认 info", "value", levelStr)
		}
	}

	// 应用持久化的 HTTP 代理
	var httpProxyCfg struct {
		Proxy string `json:"proxy"`
	}
	if err := a.configService.GetJSON("http_proxy", &httpProxyCfg); err == nil && httpProxyCfg.Proxy != "" {
		if err := httputil.SetGlobalProxy(httpProxyCfg.Proxy); err != nil {
			slog.Warn("HTTP 代理配置无效，已忽略", "proxy", httpProxyCfg.Proxy, "error", err)
		} else {
			slog.Info("HTTP 代理已应用", "proxy", httpProxyCfg.Proxy)
		}
	}

	// 初始化JWT密钥
	if err := a.initJWTSecret(configRepo); err != nil {
		return fmt.Errorf("初始化JWT密钥失败: %w", err)
	}

	// 从数据库读取音乐路径配置
	var musicPathConfig struct {
		Path         string   `json:"path"`
		ExcludeDirs  []string `json:"exclude_dirs"`
		ExcludePaths []string `json:"exclude_paths"`
	}
	if err := a.configService.GetJSON("music_path", &musicPathConfig); err != nil {
		slog.Warn("读取音乐路径配置失败，使用默认值", "error", err)
		musicPathConfig.Path = "music"
		musicPathConfig.ExcludeDirs = []string{"@eaDir", "tmp"}
		musicPathConfig.ExcludePaths = []string{}
	}
	// 移动端传入的音乐目录优先级最高（每次启动时由客户端指定）
	// 同步写回 DB，确保 GET /settings/music-path 和 onMusicPathConfigChanged 读到正确值
	if a.config.MusicDir != "" && musicPathConfig.Path != a.config.MusicDir {
		musicPathConfig.Path = a.config.MusicDir
		if err := a.configService.SetJSON("music_path", musicPathConfig); err != nil {
			slog.Warn("同步移动端音乐目录到配置失败", "error", err)
		}
	}

	// 从数据库读取扫描配置
	var scanConfigData struct {
		AutoScan         bool     `json:"auto_scan"`
		ScanInterval     int      `json:"scan_interval"`
		SupportedFormats []string `json:"supported_formats"`
	}
	if err := a.configService.GetJSON("scan_config", &scanConfigData); err != nil {
		slog.Warn("读取扫描配置失败，使用默认值", "error", err)
		scanConfigData.SupportedFormats = []string{"mp3", "flac", "wav", "ape", "ogg", "m4a", "mp4", "mov", "wma", "aif", "aiff", "mka", "mkv", "webm", "avi", "ts"}
	}

	// 读取标题来源配置
	titleSource := a.configService.GetString("scan_title_source", "tag")

	// 从数据库读取 ffprobe 路径配置
	var ffprobeConfig struct {
		Path string `json:"path"`
	}
	if err := a.configService.GetJSON("ffprobe_path", &ffprobeConfig); err != nil {
		slog.Warn("读取 ffprobe 配置失败，使用默认值", "error", err)
		ffprobeConfig.Path = "ffprobe"
	}

	// 从数据库读取封面存储路径配置
	var coverStorageConfig struct {
		Path string `json:"path"`
	}
	if err := a.configService.GetJSON("cover_storage_path", &coverStorageConfig); err != nil {
		slog.Warn("读取封面存储路径配置失败，使用默认值", "error", err)
		coverStorageConfig.Path = "covers"
	}

	// 确保封面存储目录存在
	coverStoragePath := coverStorageConfig.Path
	if !filepath.IsAbs(coverStoragePath) {
		coverStoragePath = filepath.Join(filepath.Dir(a.config.DBPath), coverStoragePath)
	}
	if err := os.MkdirAll(coverStoragePath, 0755); err != nil {
		return fmt.Errorf("创建封面存储目录失败：%w", err)
	}
	slog.Info("封面存储目录已创建", "path", coverStoragePath)

	// 初始化服务层
	scanConfig := &services.ScanConfig{
		MusicPath:        musicPathConfig.Path,
		ExcludeDirs:      musicPathConfig.ExcludeDirs,
		ExcludePaths:     musicPathConfig.ExcludePaths,
		SupportedFormats: scanConfigData.SupportedFormats,
	}
	slog.Info("音乐目录", "path", scanConfig.MusicPath)
	a.scanner = services.NewScanner(scanConfig)

	metadataConfig := &services.MetadataConfig{
		FFProbePath:      ffprobeConfig.Path,
		CoverStoragePath: coverStoragePath,
		TitleSource:      titleSource,
	}
	slog.Info("封面存储路径", "path", metadataConfig.CoverStoragePath)
	a.metadataExtractor = services.NewMetadataExtractor(metadataConfig)

	a.playlistService = services.NewPlaylistService(db.PlaylistRepository(), db.PlaylistSongRepository(), db.SongRepository(), a.metadataExtractor)
	a.songService = services.NewSongService(db.SongRepository(), db, a.metadataExtractor, a.scanner, a.configService, db.PlaylistRepository())
	a.backupService = services.NewBackupService(db)

	// 创建认证服务
	authService, err := services.NewAuthService(configRepo, db.TokenRepository(), a.config.Username, a.config.Password)
	if err != nil {
		return fmt.Errorf("创建认证服务失败: %w", err)
	}
	a.authService = authService

	// 创建升级服务
	a.upgradeService = services.NewUpgradeService()

	// 创建缓存服务
	cacheDir := filepath.Join(filepath.Dir(a.config.DBPath), "music_cache")
	a.cacheService = services.NewCacheService(cacheDir, a.configService)

	// 注入 ffmpeg 路径(用于音频转码)
	var ffmpegConfig struct {
		Path string `json:"path"`
	}
	if err := a.configService.GetJSON("ffmpeg_path", &ffmpegConfig); err != nil {
		slog.Debug("读取 ffmpeg 配置失败，使用默认值", "error", err)
		ffmpegConfig.Path = "ffmpeg"
	}
	a.cacheService.SetFFmpegPath(ffmpegConfig.Path)
	a.metadataExtractor.SetFFMpegPath(ffmpegConfig.Path)
	a.metadataExtractor.SetHTTPClient(httputil.NewClient(30 * time.Second))

	// 让 SongService.Delete/BatchDelete 联动清理 cache,避免 ID 复用时旧 cache 被新 song 误命中
	a.songService.SetCacheService(a.cacheService)

	// 一次性迁移：将旧版预分割的 CUE 记录迁移为按需提取模式
	a.migrateCueSplitsToOnTheFly(db)

	// 创建音源健康度指标收集器(纯内存滚动窗口,Fetcher 上报、Resolver 排序、admin API 消费)
	a.sourceMetrics = source.NewSourceMetrics(source.DefaultMetricsOpts())

	// 为内部 HTTP 调用准备 access_token,用于解析 JS 插件代理的相对路径
	internalToken, err := a.authService.GeneratePluginToken(context.Background())
	if err != nil {
		return fmt.Errorf("生成内部 token 失败: %w", err)
	}

	// 解析端口
	internalServerPort := 58091
	if p, err := strconv.Atoi(a.config.Port); err == nil {
		internalServerPort = p
	}

	a.urlResolver = services.NewInternalURLResolver(internalServerPort, internalToken)
	a.lyricFetcher = services.NewLyricFetcher(a.urlResolver, nil)

	// 初始化 JS 插件管理器（必须在 setupRouter 之前，因为路由注册需要访问 jsPluginManager）
	jsPluginsDir, err := filepath.Abs(filepath.Join(filepath.Dir(a.config.DBPath), "jsplugins"))
	if err != nil {
		return fmt.Errorf("解析 JS 插件目录绝对路径失败: %w", err)
	}
	if err := os.MkdirAll(jsPluginsDir, 0755); err != nil {
		return fmt.Errorf("创建 JS 插件目录失败: %w", err)
	}
	jsPluginsDataDir, err := filepath.Abs(filepath.Join(filepath.Dir(a.config.DBPath), "jsplugins_data"))
	if err != nil {
		return fmt.Errorf("解析 JS 插件数据目录绝对路径失败: %w", err)
	}
	if err := os.MkdirAll(jsPluginsDataDir, 0755); err != nil {
		return fmt.Errorf("创建 JS 插件数据目录失败: %w", err)
	}
	jsPluginRepo := a.db.JSPluginRepository()
	a.jsPluginManager = jsplugin.NewManager(jsPluginRepo, jsPluginsDir, jsPluginsDataDir, a.config.BasePath, a.router, a.db)
	a.jsPluginManager.SetAuthService(a.authService, a.config.Port)

	// 注入歌词提供者探测钩子：让 models.LyricURLPath 在存在歌词插件时,
	// 对本地无歌词歌曲也放行歌词 URL,从而触发 GetSongLyric 的自动搜索 fallback(#303)。
	models.HasLyricProvider = a.jsPluginManager.HasLyricProvider

	// 创建歌曲下载服务并注入到 JS 插件管理器（bridge songs.download 调用）
	a.downloadActivity = &services.DownloadActivity{}
	songDownloader := services.NewSongDownloader(a.songService, a.cacheService, a.configService, a.scanner.GetMusicPath, a.lyricFetcher, a.downloadActivity)
	a.jsPluginManager.SetSongDownloader(songDownloader)
	a.jsPluginManager.SetServices(a.songService, a.playlistService)
	a.jsPluginManager.SetConfigService(a.configService)

	// 装配音源处理链:Fetcher → Resolver → Orchestrator
	// 三个组件都通过接口注入,与具体类型(jsplugin.Manager / services.MetadataExtractor)解耦。
	// 必须在 cacheService + convertService + jsPluginManager 都创建完后再装配。
	proberAdapter := &proberAdapter{m: a.metadataExtractor}
	invokerAdapter := &jsPluginInvokerAdapter{m: a.jsPluginManager}
	listerAdapter := &jsPluginListerAdapter{m: a.jsPluginManager}
	songUpdaterAdapter := &songUpdaterAdapter{s: a.songService}

	sourceFetcher := source.NewSourceFetcher(source.FetcherOpts{
		Prober:        proberAdapter,
		PluginInvoker: invokerAdapter,
		Metrics:       a.sourceMetrics,
		HTTPClient:    httputil.NewDownloadClient(),
		StallTimeout:  120 * time.Second,
		LoadValidationOpts: func() source.ValidationOpts {
			opts := source.DefaultValidationOpts()
			// 读 source_validation 配置;失败则用默认值(灰度降级安全)
			_ = a.configService.GetJSON("source_validation", &opts)
			return opts
		},
	})
	sourceResolver := source.NewSourceResolver(listerAdapter, invokerAdapter, a.sourceMetrics, source.DefaultResolverOpts())
	// playActivity 跟踪所有"和某首歌相关"的进行中工作（play/prefetch/transcode/reassign），
	// 让用户切歌时同会话下旧工作集体退场。issue #79：快速切歌时旧请求一直占着 plugin worker。
	a.playActivity = playactivity.New()

	sourceOrchestrator := source.NewSourceOrchestrator(source.OrchestratorOpts{
		Fetcher:          sourceFetcher,
		Resolver:         sourceResolver,
		SongUpdater:      songUpdaterAdapter,
		ActivityRegistry: &playActivityReassignTracker{reg: a.playActivity},
	})
	a.sourceOrchestrator = sourceOrchestrator
	a.cacheService.SetOrchestrator(sourceOrchestrator)

	// 注入缓存路径回调和歌曲查询
	songRepo := a.db.SongRepository()
	a.cacheService.SetCachePathCallbacks(
		songRepo.UpdateCachePath,
		songRepo.ClearCachePath,
		songRepo.ClearAllCachePaths,
		songRepo.ListSongsWithCache,
	)
	a.metadataRefresher = services.NewMetadataRefresher(
		songRepo.ListSongsNeedingMetadata,
		songRepo.UpdateMetadata,
		songRepo.UpdateTagFields,
		func(ctx context.Context, song *models.Song) (string, error) {
			if song.IsPluginSourced() {
				resolved, err := a.cacheService.ResolveURL(ctx, song)
				if err != nil {
					return "", err
				}
				return resolved.URL, nil
			}
			return song.URL, nil
		},
		a.metadataExtractor,
	)
	a.metadataRefresher.SetRemoteTitleSource(func() string {
		return a.configService.GetString("remote_title_source", "filename")
	})
	a.cacheService.SetCacheCompleteCallback(
		func(ctx context.Context, song *models.Song, filePath string) {
			if services.NeedsMetadata(song) {
				a.metadataRefresher.RefreshSongFromFile(ctx, song, filePath)
			}
			songDownloader.TryAutoDownload(ctx, song)
		},
	)

	// 初始化 Tracely 监控客户端（仅在编译时注入了 AppSecret 与 Host 时启用）
	if tracelycfg.Enabled() {
		a.tracelyClient = tracely.New(tracely.Config{
			AppID:             tracelycfg.AppID,
			AppSecret:         tracelycfg.AppSecret,
			Host:              tracelycfg.Host,
			EnableHeartbeat:   true,
			HeartbeatInterval: 60 * time.Second,
			Tags: map[string]string{
				"version": version.GetFullVersion(),
			},
		})
		slog.Info("Tracely 监控初始化成功")

		// 上报安装或升级事件
		serverPlatform := runtime.GOOS + "-" + runtime.GOARCH
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "unknown"
		}
		lastVersion := a.configService.GetString("tracely_reported_version", "")
		currentVersion := version.Version
		if lastVersion == "" {
			a.tracelyClient.ReportInstall(currentVersion, serverPlatform, hostname)
			slog.Info("Tracely 上报安装事件", "version", currentVersion)
		} else if lastVersion != currentVersion {
			a.tracelyClient.ReportUpgrade(lastVersion, currentVersion, serverPlatform, hostname)
			slog.Info("Tracely 上报升级事件", "from", lastVersion, "to", currentVersion)
		}
		if lastVersion != currentVersion {
			if err := a.configService.Set("tracely_reported_version", currentVersion); err != nil {
				slog.Warn("写入 Tracely 上报版本失败", "error", err)
			}
		}
	} else {
		slog.Info("Tracely 监控未启用（编译时未注入 AppSecret/Host）")
	}

	// 将监听端口写入 configs 数据库（只写入，下次启动不读取）
	if err := a.configService.Set("server_port", a.config.Port); err != nil {
		slog.Warn("写入监听端口配置失败", "error", err)
	}
	slog.Info("监听端口已写入配置", "port", a.config.Port)

	// 将服务器平台信息写入 configs 数据库（供插件读取）
	serverPlatform := runtime.GOOS + "-" + runtime.GOARCH
	if err := a.configService.Set("server_platform", serverPlatform); err != nil {
		slog.Warn("写入服务器平台配置失败", "error", err)
	}
	slog.Info("服务器平台已写入配置", "platform", serverPlatform)

	a.setupRouter()

	// 启动自动扫描调度（从持久化配置恢复）
	a.autoScanner = services.NewAutoScanner(a.songService, a.configService)
	autoScanCfg := a.autoScanner.GetConfig()
	a.autoScanner.ApplyConfig(autoScanCfg)

	// 异步启动 JS 插件管理器（加载插件 + 健康检查 + 热更新监控）
	go func() {
		if err := a.jsPluginManager.Start(context.Background()); err != nil {
			slog.Error("failed to start js plugin manager", "error", err)
		}
	}()
	slog.Info("JS 插件异步加载已启动（含健康检查和热更新监控）")

	return nil
}

// onMusicPathConfigChanged 处理 music_path 配置变更
// 重建 Scanner（使用新的排除配置）并触发清理排除目录中的歌曲
func (a *App) onMusicPathConfigChanged(scanHandler *handlers.ScanHandler) {
	// 重新读取 music_path 配置
	var musicPathConfig struct {
		Path         string   `json:"path"`
		ExcludeDirs  []string `json:"exclude_dirs"`
		ExcludePaths []string `json:"exclude_paths"`
	}
	if err := a.configService.GetJSON("music_path", &musicPathConfig); err != nil {
		slog.Error("配置变更回调：读取 music_path 配置失败", "error", err)
		return
	}

	// 重新读取扫描配置（获取 SupportedFormats）
	var scanConfigData struct {
		SupportedFormats []string `json:"supported_formats"`
	}
	if err := a.configService.GetJSON("scan_config", &scanConfigData); err != nil {
		slog.Warn("配置变更回调：读取 scan_config 失败，使用默认值", "error", err)
		scanConfigData.SupportedFormats = []string{"mp3", "flac", "wav", "ape", "ogg", "m4a", "mp4", "mov", "wma", "aif", "aiff", "mka", "mkv", "webm", "avi", "ts"}
	}

	// 重建 Scanner
	newScanConfig := &services.ScanConfig{
		MusicPath:        musicPathConfig.Path,
		ExcludeDirs:      musicPathConfig.ExcludeDirs,
		ExcludePaths:     musicPathConfig.ExcludePaths,
		SupportedFormats: scanConfigData.SupportedFormats,
	}
	a.scanner = services.NewScanner(newScanConfig)

	// 更新 ScanHandler 中的 Scanner 引用
	scanHandler.SetScanner(a.scanner)

	// 更新 SongService 中的 Scanner 引用
	a.songService.SetScanner(a.scanner)

	// 更新 MetadataExtractor 的 TitleSource 配置
	a.metadataExtractor.SetTitleSource(a.configService.GetString("scan_title_source", "tag"))

	slog.Info("配置变更回调：Scanner 已重建",
		"musicPath", musicPathConfig.Path,
		"excludeDirs", musicPathConfig.ExcludeDirs,
		"excludePaths", musicPathConfig.ExcludePaths,
	)

	// 异步清理排除目录中的歌曲
	go func() {
		result, err := a.songService.CleanInvalidSongs(context.Background())
		if err != nil {
			slog.Error("配置变更回调：清理无效歌曲失败", "error", err)
			return
		}
		if result.Total > 0 {
			slog.Info("配置变更回调：清理无效歌曲完成",
				"total", result.Total,
				"fileNotFound", result.FileNotFound,
				"inExcludedDir", result.InExcludedDir,
			)
		}
	}()
}

// BuildHandler 构建 HTTP Handler（含 BasePath 处理），供外部自行绑定 http.Server
func (a *App) BuildHandler() http.Handler {
	var handler http.Handler = a.router
	if a.config.BasePath != "" {
		mux := http.NewServeMux()
		mux.Handle(a.config.BasePath+"/", http.StripPrefix(a.config.BasePath, a.router))
		mux.HandleFunc(a.config.BasePath, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, a.config.BasePath+"/", http.StatusMovedPermanently)
		})
		handler = mux
	}
	return handler
}

// Start 启动应用程序（阻塞监听）
func (a *App) Start() error {
	if a.config.UsingDefaultCredentials {
		// 不打印密码明文：日志可能被导出提交到 issue，凭证不应落盘。
		// 使用默认凭证时账号/密码均为 admin，用户已知；此处仅提示。
		slog.Info("使用默认管理员账号启动（账号密码均为 admin，请尽快修改）", "username", a.config.Username)
	}

	// 显式创建 listener：支持 port=0（由系统自动分配空闲端口，本地/Bundle 模式用），
	// 并能在监听后拿到真实端口。ListenAndServe 无法回报自动分配的端口。
	ln, err := net.Listen("tcp", ":"+a.config.Port)
	if err != nil {
		return err
	}
	actualPort := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	if actualPort != a.config.Port {
		a.config.Port = actualPort
		// 回写真实端口（server_port 供插件/调试读取；Init 阶段写的是请求值，可能为 0）
		if a.configService != nil {
			if err := a.configService.Set("server_port", actualPort); err != nil {
				slog.Warn("回写监听端口配置失败", "error", err)
			}
		}
	}

	// 该行会被桌面端 DesktopBackendService 解析以获取实际端口，格式勿轻易变更
	slog.Info(fmt.Sprintf("HTTP 访问地址: http://localhost:%s%s/", a.config.Port, a.config.BasePath))
	slog.Info("服务器启动",
		"version", version.GetVersion(),
		"commit", version.GitCommit,
		"build_time", version.BuildTime,
		"port", a.config.Port,
		"base_path", a.config.BasePath)

	a.server = &http.Server{
		Handler: a.BuildHandler(),
	}
	return a.server.Serve(ln)
}

// initJWTSecret 初始化JWT密钥
func (a *App) initJWTSecret(configs *database.ConfigRepository) error {
	// 检查是否已有JWT密钥
	_, err := configs.Get(context.Background(), "jwt_secret")
	if err == nil {
		// 已存在JWT密钥，无需重新生成
		return nil
	}

	// 生成新的JWT密钥
	secret, err := services.GenerateSecret()
	if err != nil {
		return fmt.Errorf("生成JWT密钥失败: %w", err)
	}

	// 保存JWT密钥到数据库
	if err := configs.Set(context.Background(), &models.Config{
		Key:   "jwt_secret",
		Value: secret,
	}); err != nil {
		return fmt.Errorf("保存JWT密钥失败: %w", err)
	}

	return nil
}

// showHelp 显示帮助信息
func (a *App) showHelp() {
	flag.Usage()
	fmt.Println()
	fmt.Println("示例用法:")
	fmt.Println("  ./songloft -username admin -password admin -port 58091")
	fmt.Println("  ./songloft -username admin -password admin -db data/songloft.db")
	fmt.Println("  ./songloft -username admin -password admin -port 58091 -db data/songloft.db")
	fmt.Println()
	fmt.Println("环境变量:")
	fmt.Println("  ADMIN_USERNAME  - 管理员用户名（可通过 -username 参数指定）")
	fmt.Println("  ADMIN_PASSWORD  - 管理员密码（可通过 -password 参数指定）")
	fmt.Println("  LISTEN_PORT     - 监听端口（默认: 58091，可通过 -port 参数指定）")
	fmt.Println("  DB_PATH         - 数据库文件路径（默认: data/songloft.db，可通过 -db 参数指定）")
	fmt.Println("  BASE_PATH       - URL 基础路径，用于反向代理子路径部署（如 /songloft，可通过 -base-path 参数指定）")
	fmt.Println("  MUSIC_DIR       - 音乐目录（非空时覆盖 DB 中的默认值，可通过 -music 参数指定）")
	fmt.Println()
	fmt.Println("注意: 其他配置（扫描配置等）存储在数据库 config 表中")
}

// normalizeBasePath 规范化 base path：确保以 / 开头、不以 / 结尾，空或 "/" 返回空串
func normalizeBasePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "/" {
		return "", nil
	}
	if strings.Contains(raw, "?") || strings.Contains(raw, "#") || strings.Contains(raw, "..") {
		return "", fmt.Errorf("base-path 不能包含 '?', '#' 或 '..': %q", raw)
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	raw = strings.TrimRight(raw, "/")
	return raw, nil
}

// ParseConfig 解析配置（从命令行参数和环境变量）
func ParseConfig() (*config.AppConfig, error) {
	// 定义命令行参数
	var (
		port     = flag.String("port", "58091", "监听端口")
		dbPath   = flag.String("db", "data/songloft.db", "数据库文件路径")
		musicDir = flag.String("music", "", "音乐目录（Bundle 桌面模式由客户端传入，非空时覆盖 DB 中的 music_path）")
		username = flag.String("username", "", "管理员用户名")
		password = flag.String("password", "", "管理员密码")
		basePath = flag.String("base-path", "", "URL 基础路径，用于反向代理子路径部署（如 /songloft）")
		help     = flag.Bool("help", false, "显示帮助信息")
		showVer  = flag.Bool("version", false, "显示版本信息")
	)

	// 解析命令行参数
	flag.Parse()

	// 显示版本信息
	if *showVer {
		fmt.Printf("Songloft Version: %s\n", version.GetVersion())
		fmt.Printf("Git Commit: %s\n", version.GitCommit)
		fmt.Printf("Build Time: %s\n", version.BuildTime)
		if version.BuildType != "" {
			fmt.Printf("Build Type: %s\n", version.BuildType)
		} else {
			fmt.Printf("Build Type: full\n")
		}
		os.Exit(0)
	}

	// 显示帮助信息
	if *help {
		a := &App{}
		a.showHelp()
		os.Exit(0)
	}

	// 检查必要凭证（优先使用命令行参数，其次使用环境变量）
	adminUsername := *username
	if adminUsername == "" {
		adminUsername = os.Getenv("ADMIN_USERNAME")
	}

	adminPassword := *password
	if adminPassword == "" {
		adminPassword = os.Getenv("ADMIN_PASSWORD")
	}

	usingDefaultCredentials := false
	if adminUsername == "" {
		adminUsername = "admin"
		usingDefaultCredentials = true
	}
	if adminPassword == "" {
		adminPassword = "admin"
		usingDefaultCredentials = true
	}

	// 获取数据库路径（优先使用命令行参数，其次使用环境变量）
	finalDBPath := *dbPath
	if envDBPath := os.Getenv("DB_PATH"); envDBPath != "" && *dbPath == "data/songloft.db" {
		finalDBPath = envDBPath
	}

	// 获取端口（优先使用命令行参数，其次使用环境变量）
	listenPort := *port
	if listenPort == "58091" {
		if envPort := os.Getenv("LISTEN_PORT"); envPort != "" {
			listenPort = envPort
		}
	}

	// 获取 base path（优先使用命令行参数，其次使用环境变量）
	finalBasePath := *basePath
	if finalBasePath == "" {
		if envBasePath := os.Getenv("BASE_PATH"); envBasePath != "" {
			finalBasePath = envBasePath
		}
	}
	normalizedBasePath, err := normalizeBasePath(finalBasePath)
	if err != nil {
		return nil, err
	}

	// 音乐目录：非空时覆盖 DB 中的相对默认值 "music"（相对路径按进程 CWD 解析）。
	// 优先使用命令行参数 -music（Bundle 桌面模式由客户端传入），其次使用环境变量
	// MUSIC_DIR（服务器/容器部署，如 Home Assistant 加载项把 /media 传入）。
	finalMusicDir := *musicDir
	if finalMusicDir == "" {
		if envMusicDir := os.Getenv("MUSIC_DIR"); envMusicDir != "" {
			finalMusicDir = envMusicDir
		}
	}
	return &config.AppConfig{
		Port:                    listenPort,
		DBPath:                  finalDBPath,
		Username:                adminUsername,
		Password:                adminPassword,
		BasePath:                normalizedBasePath,
		UsingDefaultCredentials: usingDefaultCredentials,
		MusicDir:                finalMusicDir,
	}, nil
}
