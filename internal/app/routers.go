package app

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"mimusic/internal/handlers"
	app_middleware "mimusic/internal/middleware"

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
	// 静态资源路由（无需认证）
	a.jsPluginManager.RegisterStaticRoutes(a.router)
	// API 转发路由（需要认证）
	a.router.Group(func(r chi.Router) {
		r.Use(app_middleware.AuthMiddleware(a.authService))
		a.jsPluginManager.RegisterAPIRoutes(r)
	})
}

func (a *App) setupAPIV1Router() {
	authHandler := handlers.NewAuthHandler(a.authService)
	songHandler := handlers.NewSongHandler(
		a.songService,
		a.cacheService,
		a.configService,
		&reassignAdapter{orch: a.sourceOrchestrator, s: a.songService},
		a.lyricFetcher,
	)
	playlistHandler := handlers.NewPlaylistHandler(a.playlistService)
	configHandler := handlers.NewConfigHandler(a.configService)
	scanHandler := handlers.NewScanHandler(a.songService, a.scanner)

	// 注册配置变更回调：当 music_path 配置变更时，重建 Scanner 并清理排除目录中的歌曲
	configHandler.SetOnConfigChanged(func(key string) {
		if key != "music_path" {
			return
		}
		a.onMusicPathConfigChanged(scanHandler)
	})
	versionHandler := handlers.NewVersionHandler()
	healthHandler := handlers.NewHealthHandler()
	upgradeHandler := handlers.NewUpgradeHandler(a.upgradeService)
	proxyHandler := handlers.NewProxyHandler()

	// 创建缓存处理器（使用 App 的 cacheService 和 configService）
	cacheHandler := handlers.NewCacheHandler(
		a.cacheService,
		a.configService,
	)

	// 创建转换处理器（网络歌曲→本地歌曲）
	convertHandler := handlers.NewConvertHandler(a.convertService)

	// 创建 JS 插件管理处理器
	jsPluginHandler := handlers.NewJSPluginHandler(
		a.jsPluginManager.Packager(),
		a.db.JSPluginRepository(),
		a.jsPluginManager,
		a.sourceMetrics,
	)

	a.router.Route("/api/v1", func(r chi.Router) {
		// 认证模块路由
		r.Post("/auth/login", authHandler.Login)
		r.Post("/auth/refresh", authHandler.RefreshToken)

		// 版本信息
		r.Get("/version", versionHandler.GetVersion)

		// 健康检查
		r.Get("/health", healthHandler.CheckHealth)

		// 需要授权的路由组
		r.Group(func(r chi.Router) {
			r.Use(app_middleware.AuthMiddleware(a.authService))

			// 认证相关
			r.Post("/auth/logout", authHandler.Logout)
			r.Get("/auth/tokens", authHandler.ListTokens)
			r.Get("/auth/tokens/{token_id}", authHandler.GetTokenInfo)
			r.Delete("/auth/tokens/{token_id}", authHandler.RevokeToken)

			// 歌曲管理模块
			r.Get("/songs", songHandler.ListSongs)
			r.Post("/songs/remote", songHandler.AddRemoteSongs)
			r.Post("/songs/radio", songHandler.AddRadios)
			r.Post("/songs/clean", songHandler.CleanInvalidSongs)
			r.Post("/songs/batch-delete", songHandler.BatchDeleteSongs)
			r.Get("/songs/{id}", songHandler.GetSong)
			r.Put("/songs/{id}", songHandler.UpdateSong)
			r.Delete("/songs/{id}", songHandler.DeleteSong)
			r.Put("/songs/{id}/lyrics", songHandler.UpdateSongLyrics)

			// 歌单管理模块
			r.Get("/playlists", playlistHandler.ListPlaylists)
			r.Post("/playlists", playlistHandler.CreatePlaylist)
			r.Put("/playlists/reorder", playlistHandler.ReorderPlaylists)
			r.Get("/playlists/{id}", playlistHandler.GetPlaylist)
			r.Put("/playlists/{id}", playlistHandler.UpdatePlaylist)
			r.Delete("/playlists/{id}", playlistHandler.DeletePlaylist)
			r.Post("/playlists/batch-delete", playlistHandler.BatchDeletePlaylists)

			// 歌单内歌曲操作
			r.Get("/playlists/{id}/songs", playlistHandler.GetPlaylistSongs)
			r.Post("/playlists/{id}/songs", playlistHandler.AddSongToPlaylist)
			r.Put("/playlists/{id}/songs/reorder", playlistHandler.ReorderPlaylistSongs)
			r.Delete("/playlists/{id}/songs/{songId}", playlistHandler.RemoveSongFromPlaylist)
			r.Post("/playlists/{id}/touch", playlistHandler.TouchPlaylist)
			r.Post("/playlists/{id}/cover", playlistHandler.UploadPlaylistCover)
			r.Get("/playlists/{id}/cover", playlistHandler.GetPlaylistCover)

			// 网络歌曲→本地歌曲转换
			r.Post("/playlists/{id}/convert-to-local", convertHandler.ConvertPlaylist)
			r.Get("/playlists/{id}/convert-progress", convertHandler.GetConvertProgress)
			r.Post("/playlists/{id}/convert-progress/cancel", convertHandler.CancelConvert)
			r.Get("/settings/auto-convert", convertHandler.GetAutoConvertSetting)
			r.Put("/settings/auto-convert", convertHandler.UpdateAutoConvertSetting)

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

			// 资源代理模块（解决外部 CDN 的 CORS 问题）
			r.Get("/proxy", proxyHandler.Proxy)

			// 歌曲播放端点（流式返回音频，支持 local/remote/radio 三种类型）
			r.Get("/songs/{id}/play", songHandler.GetSongPlay)
			r.Head("/songs/{id}/play", songHandler.GetSongPlay)

			// 歌曲封面端点（本地歌曲返回封面文件，网络歌曲由 CoverURL 直接指向外部 CDN）
			r.Get("/songs/{id}/cover", songHandler.GetSongCover)

			// 歌曲歌词端点（根据 lyric_source 分发到 URL 下载或直接返回缓存文本）
			r.Get("/songs/{id}/lyric", songHandler.GetSongLyric)

			// 音乐缓存管理（独立前缀，避免与 /cache/{hash} 冲突）
			r.Get("/cache-manage/stats", cacheHandler.HandleGetCacheStats)
			r.Post("/cache-manage/clean", cacheHandler.HandleCleanCache)
			r.Get("/cache-manage/config", cacheHandler.HandleGetCacheConfig)
			r.Put("/cache-manage/config", cacheHandler.HandleUpdateCacheConfig)

			// 升级管理模块
			r.Get("/upgrade/versions", upgradeHandler.GetVersions)
			r.Get("/upgrade/check", upgradeHandler.CheckUpdate)
			r.Post("/upgrade/start", upgradeHandler.StartUpgrade)
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
		"image/svg+xml",
		"font/woff2",
	))

	// 基础中间件
	a.router.Use(chi_middleware.Logger)

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
