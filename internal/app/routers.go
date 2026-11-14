package app

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime/debug"
	"strings"

	"songloft/internal/handlers"
	app_middleware "songloft/internal/middleware"
	"songloft/internal/services"

	"github.com/hanxi/tracely/sdk/go/tracely"

	"github.com/go-chi/chi/v5"
	chi_middleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func (a *App) setupRouter() {
	// 设置基础路由（含中间件注册）
	a.setupBaseRouter()

	// API v1 路由组
	a.setupAPIV1Router()

	// JS 插件运行时路由（必须在中间件注册之后）
	// 静态资源路由（无需认证）+ 加载 publicPaths 缓存
	a.jsPluginManager.RegisterStaticRoutes(a.router)
	// API 转发路由（需要认证，publicPaths 声明的路径除外）
	a.router.Group(func(r chi.Router) {
		r.Use(app_middleware.AuthMiddleware(a.authService, a.jsPluginManager))
		a.jsPluginManager.RegisterAPIRoutes(r)
	})
}

func (a *App) setupAPIV1Router() {
	authHandler := handlers.NewAuthHandler(a.authService)
	hlsHandler := handlers.NewHLSHandler(a.songService, a.configService)
	videoHLSHandler := handlers.NewVideoHLSHandler(a.songService, a.cacheService)
	songHandler := handlers.NewSongHandler(
		a.songService,
		a.cacheService,
		&reassignAdapter{orch: a.sourceOrchestrator, s: a.songService},
		a.lyricFetcher,
		hlsHandler,
		a.playActivity,
	)
	songHandler.SetGetMusicPath(a.scanner.GetMusicPath)
	songHandler.SetPlayBroadcaster(&playEventBroadcastAdapter{m: a.jsPluginManager})
	songHandler.SetLyricSearcher(a.jsPluginManager)
	songHandler.SetCoverSearcher(a.jsPluginManager)
	songHandler.SetMetadataRefresher(a.metadataRefresher)
	songHandler.SetDownloadActivity(a.downloadActivity)
	songHandler.SetConfigService(a.configService)
	songHandler.SetURLResolver(a.urlResolver)
	songHandler.SetPlayHistoryRecorder(a.playHistoryService)
	thumbCache := services.NewCoverThumbCache(filepath.Dir(a.config.DBPath))
	songHandler.SetThumbCache(thumbCache)
	playlistHandler := handlers.NewPlaylistHandler(a.playlistService, a.songService)
	playlistHandler.SetURLResolver(a.urlResolver)
	playlistHandler.SetThumbCache(thumbCache)
	playHistoryHandler := handlers.NewPlayHistoryHandler(a.playHistoryService)
	progressHandler := handlers.NewPlaylistProgressHandler(services.NewPlaylistProgressService(a.db))
	themePackHandler := handlers.NewThemePackHandler(a.themePackService, a.configService)
	configHandler := handlers.NewConfigHandler(a.configService, a.db.JSPluginRepository())
	scanHandler := handlers.NewScanHandler(a.songService, a.scanner, a.configService)
	fingerprintService := services.NewFingerprintService(a.db.SongRepository())
	scanHandler.SetFingerprintService(fingerprintService)
	a.songService.SetFingerprintService(fingerprintService)

	// music_path 写后回调：重建 Scanner 并清理排除目录中的歌曲。
	// 两条入口都触发同一副作用，保持 admin /configs PUT 与业务 /settings/music-path PUT 行为对齐。
	musicPathChanged := func() { a.onMusicPathConfigChanged(scanHandler) }
	scanHandler.SetOnMusicPathChanged(musicPathChanged)
	scanHandler.SetOnAutoScanChanged(func(cfg services.AutoScanConfig) {
		a.autoScanner.ApplyConfig(cfg)
	})
	configHandler.SetOnConfigChanged(func(key string) {
		switch key {
		case "music_path":
			musicPathChanged()
		case "auto_scan":
			cfg := a.autoScanner.GetConfig()
			a.autoScanner.ApplyConfig(cfg)
		}
	})
	versionHandler := handlers.NewVersionHandler()
	healthHandler := handlers.NewHealthHandler()
	upgradeHandler := handlers.NewUpgradeHandler(a.upgradeService, a.configService, filepath.Dir(a.config.DBPath))
	proxyHandler := handlers.NewProxyHandler(a.configService)
	proxyHandler.SetCacheService(a.cacheService)
	hlsTranscodeHandler := handlers.NewHLSTranscodeHandler(a.hlsTranscodeService, a.config.BasePath)

	// 创建缓存处理器（使用 App 的 cacheService 和 configService）
	cacheHandler := handlers.NewCacheHandler(
		a.cacheService,
		a.configService,
	)

	// 创建日志等级处理器（持有 App 的 LevelVar，PUT 时即时切换运行时等级）
	logHandler := handlers.NewLogHandler(a.configService, a.logLevelVar)

	// 创建日志导出处理器（读取落盘日志目录，脱敏后供下载）。
	// logDir 恒按 data 目录派生，即使 logWriter 初始化失败仍可导出已有文件。
	logExportHandler := handlers.NewLogExportHandler(filepath.Join(filepath.Dir(a.config.DBPath), "logs"))

	// 创建 JS 插件管理处理器
	jsPluginHandler := handlers.NewJSPluginHandler(
		a.jsPluginManager.Packager(),
		a.db.JSPluginRepository(),
		a.jsPluginManager,
		a.sourceMetrics,
		a.configService,
		a.db,
	)

	loginLimiter := app_middleware.NewLoginRateLimiter()

	a.router.Route("/api/v1", func(r chi.Router) {
		// 认证模块路由：登录接口按源 IP 限流，防爆破（回环豁免，不影响本地自动登录）
		r.With(loginLimiter.Middleware).Post("/auth/login", authHandler.Login)
		r.Post("/auth/refresh", authHandler.RefreshToken)

		// 版本信息
		r.Get("/version", versionHandler.GetVersion)

		// 健康检查
		r.Get("/health", healthHandler.CheckHealth)

		// 需要授权的路由组
		r.Group(func(r chi.Router) {
			r.Use(app_middleware.AuthMiddleware(a.authService))

			// 认证相关
			r.With(app_middleware.RequireLoginSession).Post("/auth/logout", authHandler.Logout)
			r.With(app_middleware.RequireLoginSession).Get("/auth/tokens", authHandler.ListTokens)
			r.With(app_middleware.RequireLoginSession).Delete("/auth/tokens/{token_id}", authHandler.RevokeToken)
			r.With(app_middleware.RequireLoginSession).Post("/auth/personal-tokens", authHandler.CreatePersonalToken)
			r.With(app_middleware.RequireLoginSession).Get("/auth/personal-tokens", authHandler.ListPersonalTokens)
			r.With(app_middleware.RequireLoginSession).Delete("/auth/personal-tokens/{id}", authHandler.RevokePersonalToken)

			// 歌曲管理模块
			r.Get("/songs", songHandler.ListSongs)
			r.Get("/songs/ids", songHandler.ListSongIDs)
			r.Get("/songs/random", songHandler.ListRandomSongs)
			r.Post("/songs/remote", songHandler.AddRemoteSongs)
			r.Post("/songs/radio", songHandler.AddRadios)
			r.Post("/songs/clean", songHandler.CleanInvalidSongs)
			r.Post("/songs/batch-delete", songHandler.BatchDeleteSongs)
			r.Get("/songs/duplicates", songHandler.GetDuplicates)
			r.Get("/songs/facets", songHandler.ListSongFacets)
			r.Get("/songs/folders", songHandler.ListFolders)
			r.Get("/songs/names", songHandler.ListSongNames)
			r.Get("/songs/stats", songHandler.GetLibraryStats)
			r.Get("/songs/{id}", songHandler.GetSong)
			r.Get("/songs/{id}/artists", songHandler.GetSongArtists)
			r.Get("/songs/{id}/audio-tracks", songHandler.GetSongAudioTracks)
			r.Get("/songs/{id}/tracks", songHandler.GetSongTracks)
			r.Put("/songs/{id}", songHandler.UpdateSong)
			r.Delete("/songs/{id}", songHandler.DeleteSong)
			r.Put("/songs/{id}/artists", songHandler.SetSongArtists)
			r.Put("/songs/{id}/lyrics", songHandler.UpdateSongLyrics)
			r.Put("/songs/{id}/tags", songHandler.WriteTags)
			r.Post("/songs/organize", songHandler.OrganizeSongs)
			r.Post("/songs/organize/preview", songHandler.PreviewOrganizeSongs)
			r.Post("/songs/{id}/activate", songHandler.ActivateSong)
			r.Post("/songs/{id}/played", songHandler.SongPlayed)
			r.Get("/settings/remote-title-source", songHandler.GetRemoteTitleSourceSetting)
			r.Put("/settings/remote-title-source", songHandler.UpdateRemoteTitleSourceSetting)
			r.Get("/settings/volume-normalize", songHandler.GetVolumeNormalizeSetting)
			r.Put("/settings/volume-normalize", songHandler.UpdateVolumeNormalizeSetting)
			r.Post("/songs/refresh-metadata", songHandler.StartMetadataRefresh)
			r.Get("/songs/refresh-metadata/progress", songHandler.GetMetadataRefreshProgress)
			r.Post("/songs/refresh-metadata/cancel", songHandler.CancelMetadataRefresh)

			// 歌单管理模块
			backupHandler := handlers.NewBackupHandler(a.backupService)
			r.Get("/playlists/export", backupHandler.ExportPlaylists)
			r.Post("/playlists/import", backupHandler.ImportPlaylists)
			r.Get("/playlists", playlistHandler.ListPlaylists)
			r.Get("/playlists/favorites/changes", playlistHandler.FavoriteChanges)
			r.Post("/playlists", playlistHandler.CreatePlaylist)
			r.Put("/playlists/reorder", playlistHandler.ReorderPlaylists)
			r.Get("/playlists/{id}", playlistHandler.GetPlaylist)
			r.Put("/playlists/{id}", playlistHandler.UpdatePlaylist)
			r.Delete("/playlists/{id}", playlistHandler.DeletePlaylist)
			r.Post("/playlists/batch-delete", playlistHandler.BatchDeletePlaylists)

			// 歌单内歌曲操作
			r.Get("/playlists/{id}/songs", playlistHandler.GetPlaylistSongs)
			r.Get("/playlists/{id}/song-ids", playlistHandler.GetPlaylistSongIDs)
			r.Post("/playlists/{id}/songs", playlistHandler.AddSongToPlaylist)
			r.Put("/playlists/{id}/songs/reorder", playlistHandler.ReorderPlaylistSongs)
			r.Post("/playlists/{id}/songs/sort", playlistHandler.SortPlaylistSongs)
			r.Put("/playlists/{id}/songs/move", playlistHandler.MovePlaylistSong)
			r.Delete("/playlists/{id}/songs/{songId}", playlistHandler.RemoveSongFromPlaylist)
			r.Put("/playlists/{id}/visibility", playlistHandler.SetPlaylistVisibility)
			r.Put("/playlists/{id}/pin", playlistHandler.SetPlaylistPinned)
			r.Post("/playlists/{id}/touch", playlistHandler.TouchPlaylist)
			r.Put("/playlists/{id}/sort", playlistHandler.UpdatePlaylistSort)
			r.Get("/playlists/{id}/playback-progress", progressHandler.Handle)
			r.Put("/playlists/{id}/playback-progress", progressHandler.Handle)
			r.Delete("/playlists/{id}/playback-progress", progressHandler.Handle)
			r.Post("/playlists/{id}/cover", playlistHandler.UploadPlaylistCover)
			r.Get("/playlists/{id}/cover", playlistHandler.GetPlaylistCover)

			// 自定义标签管理
			tagHandler := handlers.NewSongTagHandler(a.songTagService, a.songService, a.configService)
			r.Get("/playlists/{id}/tags", tagHandler.GetPlaylistTags)
			r.Put("/playlists/{id}/tags", tagHandler.SetPlaylistTags)
			r.Get("/song-tags", tagHandler.List)
			r.Post("/song-tags", tagHandler.Create)
			r.Get("/song-tags/{id}", tagHandler.Get)
			r.Put("/song-tags/{id}", tagHandler.Update)
			r.Delete("/song-tags/{id}", tagHandler.Delete)
			r.Get("/song-tags/{id}/songs", tagHandler.ListSongs)
			r.Get("/song-tags/{id}/song-ids", tagHandler.ListSongIDs)
			r.Post("/song-tags/{id}/bind", tagHandler.BatchBind)
			r.Post("/song-tags/{id}/unbind", tagHandler.BatchUnbind)

			r.Get("/songs/{id}/song-tags", tagHandler.GetSongTags)
			r.Put("/songs/{id}/song-tags", tagHandler.SetSongTags)
			r.Get("/settings/tag-sync-to-file", tagHandler.GetTagSyncToFile)
			r.Put("/settings/tag-sync-to-file", tagHandler.UpdateTagSyncToFile)

			// 播放历史（按播放上下文：歌单 / 歌手 / 专辑 / 其余分面维度）
			r.Get("/play-history", playHistoryHandler.GetPlayHistory)
			r.Delete("/play-history", playHistoryHandler.ClearPlayHistory)
			r.Delete("/play-history/entry", playHistoryHandler.DeletePlayHistoryEntry)

			// 主题包管理
			r.Get("/theme-packs", themePackHandler.ListThemePacks)
			r.Post("/theme-packs", themePackHandler.ImportThemePack)
			r.Get("/theme-packs/active", themePackHandler.GetActiveThemePack)
			r.Put("/theme-packs/active", themePackHandler.SetActiveThemePack)
			r.Delete("/theme-packs/active", themePackHandler.ClearActiveThemePack)
			r.Post("/theme-packs/catalog/refresh", themePackHandler.RefreshCatalog)
			r.Post("/theme-packs/catalog/install", themePackHandler.InstallFromCatalog)
			r.Get("/theme-packs/{themeID}", themePackHandler.GetThemePack)
			r.Delete("/theme-packs/{themeID}", themePackHandler.DeleteThemePack)
			r.Get("/settings/theme-catalog-url", themePackHandler.GetCatalogURLSetting)
			r.Put("/settings/theme-catalog-url", themePackHandler.UpdateCatalogURLSetting)

			r.Get("/settings/hls-proxy", hlsHandler.GetProxySetting)
			r.Put("/settings/hls-proxy", hlsHandler.UpdateProxySetting)
			r.Get("/settings/proxy-private-allowlist", proxyHandler.GetProxyAllowlistSetting)
			r.Put("/settings/proxy-private-allowlist", proxyHandler.UpdateProxyAllowlistSetting)
			r.Get("/settings/music-path", scanHandler.GetMusicPathSetting)
			r.Put("/settings/music-path", scanHandler.UpdateMusicPathSetting)
			r.Get("/settings/scan-playlist-mode", scanHandler.GetPlaylistModeSetting)
			r.Put("/settings/scan-playlist-mode", scanHandler.UpdatePlaylistModeSetting)
			r.Get("/settings/scan-auto-create-playlists", scanHandler.GetAutoCreatePlaylistsSetting)
			r.Put("/settings/scan-auto-create-playlists", scanHandler.UpdateAutoCreatePlaylistsSetting)
			r.Get("/settings/scan-title-source", scanHandler.GetScanTitleSourceSetting)
			r.Put("/settings/scan-title-source", scanHandler.UpdateScanTitleSourceSetting)
			r.Get("/settings/scan-auto-fingerprint", scanHandler.GetScanAutoFingerprintSetting)
			r.Put("/settings/scan-auto-fingerprint", scanHandler.UpdateScanAutoFingerprintSetting)
			r.Get("/settings/auto-scan", scanHandler.GetAutoScanSetting)
			r.Put("/settings/auto-scan", scanHandler.UpdateAutoScanSetting)
			r.Get("/settings/log-level", logHandler.GetLevelSetting)
			r.Put("/settings/log-level", logHandler.UpdateLevelSetting)
			r.Get("/logs/export", logExportHandler.ExportLogs)
			r.Get("/settings/plugin-registries", jsPluginHandler.GetRegistriesSetting)
			r.Put("/settings/plugin-registries", jsPluginHandler.UpdateRegistriesSetting)
			r.Get("/settings/github-discovery-token", jsPluginHandler.GetGithubDiscoveryTokenSetting)
			r.Put("/settings/github-discovery-token", jsPluginHandler.UpdateGithubDiscoveryTokenSetting)
			r.Get("/settings/http-proxy", jsPluginHandler.GetHttpProxySetting)
			r.Put("/settings/http-proxy", jsPluginHandler.UpdateHttpProxySetting)
			r.Get("/settings/github-proxy", upgradeHandler.GetGithubProxySetting)
			r.Put("/settings/github-proxy", upgradeHandler.UpdateGithubProxySetting)
			r.Get("/settings/plugin-keep-alive", jsPluginHandler.GetPluginKeepAliveSetting)
			r.Put("/settings/plugin-keep-alive", jsPluginHandler.UpdatePluginKeepAliveSetting)
			r.Get("/settings/plugin-auto-update", jsPluginHandler.GetPluginAutoUpdateSetting)
			r.Put("/settings/plugin-auto-update", jsPluginHandler.UpdatePluginAutoUpdateSetting)
			r.Get("/settings/tab-config", configHandler.GetTabConfigSetting)
			r.Put("/settings/tab-config", configHandler.UpdateTabConfigSetting)
			r.Get("/settings/plugin-order", configHandler.GetPluginOrderSetting)
			r.Put("/settings/plugin-order", configHandler.UpdatePluginOrderSetting)
			r.Get("/settings/library-browse", configHandler.GetLibraryBrowseSetting)
			r.Put("/settings/library-browse", configHandler.UpdateLibraryBrowseSetting)
			r.Get("/settings/user-preferences", configHandler.GetUserPreferencesSetting)
			r.Put("/settings/user-preferences", configHandler.UpdateUserPreferencesSetting)
			r.Get("/settings/equalizer", configHandler.GetEqualizerSetting)
			r.Put("/settings/equalizer", configHandler.UpdateEqualizerSetting)

			// 配置管理模块
			r.Get("/configs", configHandler.ListConfigs)
			r.Post("/configs", configHandler.CreateConfig)
			r.Get("/configs/{key}", configHandler.GetConfig)
			r.Put("/configs/{key}", configHandler.UpdateConfig)
			r.Delete("/configs/{key}", configHandler.DeleteConfig)

			// 扫描管理模块
			r.Post("/scan", scanHandler.ScanAndImport)
			r.Get("/scan/progress", scanHandler.GetScanProgress)
			r.Post("/scan/cancel", scanHandler.CancelScan)
			r.Get("/scan/directories", scanHandler.ListDirectories)
			r.Get("/scan/dir-names", scanHandler.ListDirNames)
			r.Get("/scan/fingerprints/status", scanHandler.GetFingerprintStatus)
			r.Post("/scan/fingerprints", scanHandler.StartFingerprintCompute)
			r.Get("/scan/fingerprints/progress", scanHandler.GetFingerprintProgress)
			r.Post("/scan/fingerprints/cancel", scanHandler.CancelFingerprintCompute)
			r.Get("/scan/fingerprints/failed", scanHandler.GetFailedFingerprints)

			// 资源代理模块（解决外部 CDN 的 CORS 问题）
			r.Get("/proxy", proxyHandler.Proxy)
			// 转码代理：服务端拉远程音频→ffmpeg 实时转 mp3 流式返回（no-import 场景下音箱不可解码的直链）
			r.Get("/proxy/transcode", proxyHandler.Transcode)
			r.Post("/proxy/transcode/hls/sessions", hlsTranscodeHandler.Create)
			r.Get("/proxy/transcode/hls/sessions/{id}/playlist.m3u8", hlsTranscodeHandler.Playlist)
			r.Head("/proxy/transcode/hls/sessions/{id}/playlist.m3u8", hlsTranscodeHandler.Playlist)
			r.Get("/proxy/transcode/hls/sessions/{id}/segments/{name}", hlsTranscodeHandler.Segment)
			r.Head("/proxy/transcode/hls/sessions/{id}/segments/{name}", hlsTranscodeHandler.Segment)
			r.Delete("/proxy/transcode/hls/sessions/{id}", hlsTranscodeHandler.Delete)

			// 歌曲播放端点（流式返回音频，支持 local/remote/radio 三种类型）
			r.Get("/songs/{id}/play", songHandler.GetSongPlay)
			r.Head("/songs/{id}/play", songHandler.GetSongPlay)
			// HLS 电台专用别名:URL 必须以 .m3u8 结尾，否则 ExoPlayer/AVPlayer 不识别为 HLS。
			// 仅作字面后缀变体存在,handler 内部按 song.Type 走同一分发逻辑。
			r.Get("/songs/{id}/play.m3u8", songHandler.GetSongPlay)
			r.Head("/songs/{id}/play.m3u8", songHandler.GetSongPlay)

			// HLS 反向代理端点（hls_proxy_mode=proxy 时启用，由 serveRadio 改写后的 m3u8 内回链触发）
			r.Get("/songs/{id}/hls/playlist", hlsHandler.HandlePlaylist)
			r.Head("/songs/{id}/hls/playlist", hlsHandler.HandlePlaylist)
			r.Get("/songs/{id}/hls/segment", hlsHandler.HandleSegment)
			r.Head("/songs/{id}/hls/segment", hlsHandler.HandleSegment)

			// 视频 HLS 转码端点（Web 端播放浏览器不原生支持的视频格式）
			r.Get("/songs/{id}/video-hls/playlist.m3u8", videoHLSHandler.GetPlaylist)
			r.Get("/songs/{id}/video-hls/*", videoHLSHandler.GetResource)

			// 歌曲封面端点（本地歌曲返回封面文件，网络歌曲由 CoverURL 直接指向外部 CDN）
			r.Get("/songs/{id}/cover", songHandler.GetSongCover)

			// 歌曲歌词端点（根据 lyric_source 分发到 URL 下载或直接返回缓存文本）
			r.Get("/songs/{id}/lyric", songHandler.GetSongLyric)

			// 音乐缓存管理（独立前缀，避免与 /cache/{hash} 冲突）
			r.Get("/cache-manage/stats", cacheHandler.HandleGetCacheStats)
			r.Post("/cache-manage/clean", cacheHandler.HandleCleanCache)
			r.Get("/cache-manage/config", cacheHandler.HandleGetCacheConfig)
			r.Put("/cache-manage/config", cacheHandler.HandleUpdateCacheConfig)
			r.Post("/cache-manage/validate-dir", cacheHandler.HandleValidateCacheDir)

			// 升级管理模块
			r.Get("/upgrade/versions", upgradeHandler.GetVersions)
			r.Get("/upgrade/check", upgradeHandler.CheckUpdate)
			r.Post("/upgrade/start", upgradeHandler.StartUpgrade)
			r.Post("/upgrade/upload", upgradeHandler.UploadBinary)
			r.Post("/upgrade/upload/confirm", upgradeHandler.ConfirmUploadUpgrade)
			r.Post("/upgrade/reset", upgradeHandler.ResetToBaseImage)
			r.Get("/upgrade/progress", upgradeHandler.GetUpgradeProgress)
		})
	})

	// JS 插件管理（RegisterRoutes 内部已定义完整路径 /api/v1/jsplugins，需在根路由上注册）
	a.router.Group(func(r chi.Router) {
		r.Use(app_middleware.AuthMiddleware(a.authService))
		jsPluginHandler.RegisterRoutes(r)
	})
}

func (a *App) setupBaseRouter() {
	// gzip 压缩中间件（对 JS/CSS/JSON 等静态资源压缩，大幅减少传输体积）
	a.router.Use(chi_middleware.Compress(5,
		"text/html",
		"text/css",
		"text/plain",
		"text/javascript",
		"application/javascript",
		"application/json",
		"application/wasm",
		"image/svg+xml",
		"font/otf",
	))

	// 基础中间件：access log 走 slog，受 /settings/log-level 控制
	a.router.Use(chi_middleware.RequestLogger(slogLogFormatter{}))

	// Tracely panic 捕获中间件（在 Recoverer 之前，确保 panic 能被上报）
	a.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					if a.tracelyClient != nil {
						a.tracelyClient.ReportError(tracely.ErrorPayload{
							Type:    "panic",
							Message: fmt.Sprintf("%v", err),
							Stack:   string(debug.Stack()),
							URL:     r.URL.String(),
						})
					}
					// 重新 panic，让 chi_middleware.Recoverer 继续处理响应
					panic(err)
				}
			}()
			next.ServeHTTP(w, r)
		})
	})

	a.router.Use(chi_middleware.Recoverer)
	a.router.Use(chi_middleware.RequestID)

	// CORS 中间件配置
	a.router.Use(cors.Handler(cors.Options{
		// 使用自定义函数验证来源，支持更灵活的域名匹配
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			// 检查是否为空
			if origin == "" {
				return false
			}

			// 允许 localhost 和 127.0.0.1（任意端口）
			if strings.HasPrefix(origin, "http://localhost") ||
				strings.HasPrefix(origin, "http://127.0.0.1") {
				return true
			}

			// 允许局域网段
			if strings.HasPrefix(origin, "http://192.168.") ||
				strings.HasPrefix(origin, "http://10.") ||
				strings.HasPrefix(origin, "http://172.16.") {
				return true
			}

			// 允许 hanxi.cc 主域名（HTTP 和 HTTPS）
			if origin == "http://hanxi.cc" || origin == "https://hanxi.cc" ||
				strings.HasPrefix(origin, "http://hanxi.cc:") ||
				strings.HasPrefix(origin, "https://hanxi.cc:") {
				return true
			}

			// 允许 hanxi.cc 所有子域名（HTTP 和 HTTPS，任意端口）
			// 匹配格式：http://xxx.hanxi.cc 或 http://xxx.hanxi.cc:port
			if strings.Contains(origin, ".hanxi.cc") {
				if strings.HasPrefix(origin, "http://") || strings.HasPrefix(origin, "https://") {
					// 提取域名部分（去掉协议和端口）
					domain := origin
					if strings.HasPrefix(domain, "http://") {
						domain = domain[7:]
					} else if strings.HasPrefix(domain, "https://") {
						domain = domain[8:]
					}

					// 去掉端口号
					if idx := strings.Index(domain, ":"); idx != -1 {
						domain = domain[:idx]
					}

					// 检查是否以 .hanxi.cc 结尾
					if strings.HasSuffix(domain, ".hanxi.cc") {
						return true
					}
				}
			}

			return false
		},
		AllowedMethods:   []string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	a.router.Use(corpMiddleware())

	// 注册前端静态文件服务
	a.registerWebStatic()

	// /music/* 和 /cover/* 静态路由已废弃:
	// - 客户端统一通过 /api/v1/songs/{id}/play 拉本地音频文件
	// - 本地歌曲封面统一通过 /api/v1/songs/{id}/cover 端点
	// - 网络歌曲封面直接使用原始 CoverURL (外部 CDN)
	// 旧的 Base62 路径编码方案不再使用。

	// 注册 Swagger 路由（根据构建标签条件编译）
	a.registerSwagger()
}

// corpMiddleware 给所有响应加上 Cross-Origin-Resource-Policy: cross-origin，
// 让浏览器允许在开了 COEP: require-corp 的页面里嵌入本服务的资源。
//
// 适用场景：独立 Lynx Web 开发服务器等宿主可选择开启 COOP/COEP。
// 当前 web-core 0.23.1 的内部 RPC 均为异步，完整版并不强制隔离或 HTTPS。
// 但已开启隔离的宿主仍需要这些资源头。COEP: require-corp
// 要求每个跨源子资源显式声明「我可以被嵌入」，否则浏览器直接阻断，报
// ERR_BLOCKED_BY_RESPONSE.NotSameOriginAfterDefaultedToSameOriginByCoep。
//
// 失败模式是静默的，这是它值得一条长注释的原因：fetch 走 CORS 不受影响（歌单、歌曲
// 列表、设置全都正常），但 <img>/<audio>/<video> 是 no-cors 模式，于是 standalone
// 部署下（前端 :3000 与后端 :58091 不同源）封面、插件图标、音频流全部失败，而
// DevTools 里状态码显示的是 200 (OK) —— 对用户的表现只是「封面永远是占位图」。
//
// 为什么全局加而不是逐端点：CORP 不像 CORS 支持按来源白名单，只有 same-origin /
// same-site / cross-origin 三档，且必须落在每一个被嵌入的响应上。逐端点加必然漏 ——
// 封面（songs/playlists）、插件静态资源、音频流、通用代理分散在四个 handler 里，且
// 以后任何新的可嵌入端点一旦忘记，又是一次静默失败。
//
// 安全上这不是新开的洞：这些端点的访问控制靠 token（Authorization 头或 access_token
// 查询参数），没有 token 一律 401，嵌进别的页面也取不到内容。cross-origin 只是恢复
// CORP 这个头出现之前浏览器本来就允许的嵌入行为。
func corpMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
			next.ServeHTTP(w, r)
		})
	}
}
