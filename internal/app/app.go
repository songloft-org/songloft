package app

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"songloft/internal/config"
	"songloft/internal/database"
	"songloft/internal/handlers"
	"songloft/internal/jsplugin"
	"songloft/internal/models"
	"songloft/internal/services"
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
	convertService     *services.ConvertService
	urlResolver        *services.InternalURLResolver // 共享:把 JS 插件相对路径解析为本机绝对 URL + access_token
	lyricFetcher       *services.LyricFetcher        // 共享:解包插件歌词 JSON 拿 LRC 文本
	scanner            *services.Scanner
	metadataExtractor  *services.MetadataExtractor
	jsPluginManager    *jsplugin.Manager
	sourceMetrics      *source.SourceMetrics
	sourceOrchestrator *source.SourceOrchestrator
	webDist            embed.FS
	tracelyClient      *tracely.Client
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
	if a.db != nil {
		slog.Info("关闭数据库连接")
		return a.db.Close()
	}
	return nil
}

func (a *App) Init() error {
	// 初始化slog
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 确保数据库目录存在
	dbDir := filepath.Dir(a.config.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 兼容老安装（一次性）：若目标 DB 不存在但同目录下有 mimusic.db，则自动 rename。
	// 这是 MiMusic → Songloft v2.0 重命名中唯一保留的兼容点（详见 MIGRATION.md）。
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

	// 从数据库读取扫描配置
	var scanConfigData struct {
		AutoScan         bool     `json:"auto_scan"`
		ScanInterval     int      `json:"scan_interval"`
		SupportedFormats []string `json:"supported_formats"`
	}
	if err := a.configService.GetJSON("scan_config", &scanConfigData); err != nil {
		slog.Warn("读取扫描配置失败，使用默认值", "error", err)
		scanConfigData.SupportedFormats = []string{"mp3", "flac", "wav", "ape", "ogg", "m4a", "wma"}
	}

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
		coverStorageConfig.Path = "data/covers"
	}

	// 确保封面存储目录存在
	coverStoragePath := coverStorageConfig.Path
	if !filepath.IsAbs(coverStoragePath) {
		// 如果是相对路径，转换为绝对路径（相对于工作目录）
		absPath, err := filepath.Abs(coverStoragePath)
		if err != nil {
			return fmt.Errorf("获取封面存储目录绝对路径失败：%w", err)
		}
		coverStoragePath = absPath
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
	}
	slog.Info("封面存储路径", "path", metadataConfig.CoverStoragePath)
	a.metadataExtractor = services.NewMetadataExtractor(metadataConfig)

	a.playlistService = services.NewPlaylistService(db.PlaylistRepository(), db.PlaylistSongRepository(), db.SongRepository(), a.metadataExtractor)
	a.songService = services.NewSongService(db.SongRepository(), db, a.metadataExtractor, a.scanner, a.configService, db.PlaylistRepository())

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

	// 让 SongService.Delete/BatchDelete 联动清理 cache,避免 ID 复用时旧 cache 被新 song 误命中
	a.songService.SetCacheService(a.cacheService)

	// 创建音源健康度指标收集器(纯内存滚动窗口,Fetcher 上报、Resolver 排序、admin API 消费)
	a.sourceMetrics = source.NewSourceMetrics(source.DefaultMetricsOpts())

	// 为内部 HTTP 调用准备 access_token,用于解析 JS 插件代理的相对路径
	// (convert_service 下载歌曲、song_handler 拉歌词等)
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

	// 创建转换服务：把网络歌曲落地到本地音乐库
	a.convertService = services.NewConvertService(
		db,
		a.songService,
		a.playlistService,
		a.cacheService,
		a.configService,
		a.metadataExtractor,
		func() string {
			var cfg struct {
				Path string `json:"path"`
			}
			if err := a.configService.GetJSON("music_path", &cfg); err != nil {
				return "music"
			}
			return cfg.Path
		},
		a.urlResolver,
		a.lyricFetcher,
	)

	// 初始化 JS 插件管理器（必须在 setupRouter 之前，因为路由注册需要访问 jsPluginManager）
	jsPluginsDir := filepath.Join(filepath.Dir(a.config.DBPath), "jsplugins")
	if err := os.MkdirAll(jsPluginsDir, 0755); err != nil {
		return fmt.Errorf("创建 JS 插件目录失败: %w", err)
	}
	jsPluginsDataDir := filepath.Join(filepath.Dir(a.config.DBPath), "jsplugins_data")
	if err := os.MkdirAll(jsPluginsDataDir, 0755); err != nil {
		return fmt.Errorf("创建 JS 插件数据目录失败: %w", err)
	}
	jsPluginRepo := a.db.JSPluginRepository()
	a.jsPluginManager = jsplugin.NewManager(jsPluginRepo, jsPluginsDir, jsPluginsDataDir, a.router, a.db)
	a.jsPluginManager.SetAuthService(a.authService, a.config.Port)

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
		HTTPTimeout:   120 * time.Second,
		LoadValidationOpts: func() source.ValidationOpts {
			opts := source.DefaultValidationOpts()
			// 读 source_validation 配置;失败则用默认值(灰度降级安全)
			_ = a.configService.GetJSON("source_validation", &opts)
			return opts
		},
	})
	sourceResolver := source.NewSourceResolver(listerAdapter, invokerAdapter, a.sourceMetrics, source.DefaultResolverOpts())
	sourceOrchestrator := source.NewSourceOrchestrator(source.OrchestratorOpts{
		Fetcher:     sourceFetcher,
		Resolver:    sourceResolver,
		SongUpdater: songUpdaterAdapter,
	})
	a.sourceOrchestrator = sourceOrchestrator
	a.cacheService.SetOrchestrator(sourceOrchestrator)
	a.convertService.SetOrchestrator(sourceOrchestrator)

	// TODO(orchestrator): 缓存下载完成回调链已废弃,由 SourceOrchestrator 在 Fetch 成功后
	// 内联调用 UpdateSongDuration 和触发自动转换。当前过渡期内,网络歌曲下载与自动转换功能尚未恢复,
	// 待 SourceOrchestrator/SourceFetcher 等模块实现后接通。

	// 初始化 Tracely 监控客户端（仅在编译时注入了 AppSecret 与 Host 时启用）
	if tracelycfg.Enabled() {
		a.tracelyClient = tracely.New(tracely.Config{
			AppID:             "songloft",
			AppSecret:         tracelycfg.AppSecret,
			Host:              tracelycfg.Host,
			EnableHeartbeat:   true,
			HeartbeatInterval: 60 * time.Second,
			Tags: map[string]string{
				"version": version.GetFullVersion(),
			},
		})
		slog.Info("Tracely 监控初始化成功")
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
		scanConfigData.SupportedFormats = []string{"mp3", "flac", "wav", "ape", "ogg", "m4a", "wma"}
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

// Start 启动应用程序
func (a *App) Start() error {
	if a.config.UsingDefaultCredentials {
		slog.Info("使用默认管理员账号密码启动")
		slog.Info(fmt.Sprintf("默认管理员账号: %s，默认密码: %s", a.config.Username, a.config.Password))
	}
	slog.Info(fmt.Sprintf("HTTP 访问地址: http://localhost:%s", a.config.Port))
	slog.Info("服务器启动",
		"version", version.GetVersion(),
		"commit", version.GitCommit,
		"build_time", version.BuildTime,
		"port", a.config.Port)
	err := http.ListenAndServe(":"+a.config.Port, a.router)
	if err != nil {
		return err
	}
	return nil
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
	fmt.Println()
	fmt.Println("注意: 其他配置（如音乐目录、扫描配置等）存储在数据库 config 表中")
}

// ParseConfig 解析配置（从命令行参数和环境变量）
func ParseConfig() (*config.AppConfig, error) {
	// 定义命令行参数
	var (
		port     = flag.String("port", "58091", "监听端口")
		dbPath   = flag.String("db", "data/songloft.db", "数据库文件路径")
		username = flag.String("username", "", "管理员用户名")
		password = flag.String("password", "", "管理员密码")
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
			fmt.Printf("Build Type: lite\n")
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
	if adminUsername == "" || adminPassword == "" {
		adminUsername = "admin"
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

	return &config.AppConfig{
		Port:                    listenPort,
		DBPath:                  finalDBPath,
		Username:                adminUsername,
		Password:                adminPassword,
		UsingDefaultCredentials: usingDefaultCredentials,
	}, nil
}
