// Package mobile 提供 gomobile 可调用的入口函数，用于将 Go 后端嵌入移动客户端。
// 通过 gomobile bind 编译为 Android .aar / iOS .xcframework 后，
// Flutter 经 MethodChannel 调用这些函数在本机启动 HTTP 服务器。
package mobile

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"songloft/internal/app"
	"songloft/internal/config"
)

var (
	mu       sync.Mutex
	instance *mobileServer
)

type mobileServer struct {
	app      *app.App
	server   *http.Server
	listener net.Listener
	port     int
}

// Start 启动内嵌后端。
//   - dataDir: 应用沙盒数据目录（DB/封面/缓存/插件都放这里）
//   - musicDir: 音乐扫描目录
//   - port: 监听端口（0 = 自动分配）
//
// 返回实际监听端口号。
func Start(dataDir, musicDir string, port int) (int, error) {
	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
		return instance.port, nil
	}

	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(512 * 1024 * 1024)
	}
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(50)
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return -1, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// Android 的 Go 默认临时目录 /data/local/tmp 不允许普通应用写入。
	// 在初始化任何服务前设置沙盒路径，同时覆盖 HLS、封面和下载等临时文件。
	// TMPDIR 属于进程级配置，Stop 后保留，避免尚在退出的任务退回不可写目录。
	tmpDir := filepath.Join(dataDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return -1, fmt.Errorf("创建临时目录失败: %w", err)
	}
	if err := os.Setenv("TMPDIR", tmpDir); err != nil {
		return -1, fmt.Errorf("设置临时目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, "songloft.db")
	portStr := "0"
	if port > 0 {
		portStr = fmt.Sprintf("%d", port)
	}

	// 移动端监听 0.0.0.0（见下方 DLNA 说明），局域网可达，绝不能用默认 admin/admin：
	// 同网段任何人都能登入并经 /jsplugins 拿到 QuickJS 代码执行。为每个安装生成一次
	// 随机密码并落盘复用，客户端从同一 dataDir 读取后登录。
	adminPassword, err := ensureAdminPassword(dataDir)
	if err != nil {
		return -1, fmt.Errorf("准备管理员密码失败: %w", err)
	}

	cfg := config.NewAppConfig(portStr, dbPath, "admin", adminPassword, "")
	cfg.MusicDir = musicDir

	var emptyFS embed.FS
	a := app.NewApp(cfg, emptyFS)
	if err := a.Init(); err != nil {
		return -1, fmt.Errorf("后端初始化失败: %w", err)
	}

	// 监听所有网卡（而非 127.0.0.1）：DLNA 投屏等场景需要局域网设备（如小爱音箱）
	// 能直接拉取播放流，否则投屏 URL 只能解析到设备自身的回环地址。
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		a.Close()
		return -1, fmt.Errorf("监听端口失败: %w", err)
	}

	actualPort := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: a.BuildHandler()}

	instance = &mobileServer{
		app:      a,
		server:   srv,
		listener: ln,
		port:     actualPort,
	}

	go func() {
		slog.Info("移动端后端已启动", "port", actualPort)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务异常退出", "error", err)
		}
	}()

	return actualPort, nil
}

// ensureAdminPassword 读取 dataDir 下的管理员密码文件，不存在则生成随机密码并以
// 0600 落盘后复用。客户端从同一 dataDir 读取该文件完成登录。
func ensureAdminPassword(dataDir string) (string, error) {
	pwPath := filepath.Join(dataDir, "admin_password")
	if b, err := os.ReadFile(pwPath); err == nil {
		if pw := strings.TrimSpace(string(b)); pw != "" {
			return pw, nil
		}
	}

	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	pw := base64.RawURLEncoding.EncodeToString(buf)

	if err := os.WriteFile(pwPath, []byte(pw), 0600); err != nil {
		return "", err
	}
	return pw, nil
}

// Stop 优雅停止后端
func Stop() {
	mu.Lock()
	defer mu.Unlock()

	if instance == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	instance.server.Shutdown(ctx)
	instance.app.Close()
	instance = nil
	slog.Info("移动端后端已停止")
}

// IsRunning 检查后端是否在运行
func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return instance != nil
}

// GetPort 获取当前监听端口，未运行时返回 0
func GetPort() int {
	mu.Lock()
	defer mu.Unlock()
	if instance == nil {
		return 0
	}
	return instance.port
}
