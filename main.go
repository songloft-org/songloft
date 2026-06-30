package main

import (
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"songloft/internal/app"
	"syscall"
)

func init() {
	// 设置内存软限制：防止 OOM，Go 运行时会在接近限制时更积极地 GC
	// 可通过环境变量 GOMEMLIMIT 覆盖（如 GOMEMLIMIT=512MiB）
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(2 * 1024 * 1024 * 1024) // 默认 2GB 软限制
	}

	// 设置 GC 目标百分比：更频繁的 GC 减少内存峰值，代价是轻微 CPU 开销
	// 可通过环境变量 GOGC 覆盖
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(50) // 默认 100，降低为 50 使 GC 更频繁
	}
}

// @title Songloft API
// @version 2.9.3
// @description 轻量级音乐服务器 API 文档，支持本地音乐管理、网络歌曲、电台和歌单功能

// @contact.name API Support
// @contact.url https://github.com/songloft-org/songloft
// @contact.email im.hanxi@gmail.com

// @license.name Apache 2.0
// @license.url https://www.apache.org/licenses/LICENSE-2.0

// @host localhost:58091
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 输入 "Bearer {token}" 进行认证

func main() {
	// 解析配置
	cfg, err := app.ParseConfig()
	if err != nil {
		slog.Error("解析配置失败", "error", err)
		return
	}

	// 创建应用实例并启动
	a := app.NewApp(cfg, WebDist)
	err = a.Init()
	if err != nil {
		slog.Error("应用初始化失败", "error", err)
		return
	}

	// 设置信号处理，确保程序优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		slog.Info("收到退出信号，正在关闭...")
		if err := a.Close(); err != nil {
			slog.Error("关闭应用失败", "error", err)
		}
		os.Exit(0)
	}()

	err = a.Start()
	if err != nil {
		slog.Error("应用启动失败", "error", err)
		return
	}
}
