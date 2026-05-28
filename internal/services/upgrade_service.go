package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"songloft/internal/models"
	"songloft/internal/version"
)

const (
	// GitHub Release 版本文件 URL
	stableVersionURL = "https://github.com/mimusic-org/mimusic/releases/latest/download/version.json"
	devVersionURL    = "https://github.com/mimusic-org/mimusic/releases/download/main/version.json"

	// 二进制文件路径
	// 注意：临时文件必须与目标文件在同一目录，才能使用原子 rename 替换正在运行的二进制文件
	binarySource = "/app/mimusic" // Docker 镜像中的原始底包
	binaryTarget = "/app/data/mimusic"
	binaryBackup = "/app/data/mimusic.backup"
	binaryTemp   = "/app/data/mimusic.new"
)

// UpgradeService 升级服务
type UpgradeService struct {
	progress               models.UpgradeProgress
	progressMutex          sync.RWMutex
	httpClient             *http.Client
	baseImageBuildType     string
	baseImageBuildTypeOnce sync.Once
}

// NewUpgradeService 创建升级服务实例
func NewUpgradeService() *UpgradeService {
	return &UpgradeService{
		progress: models.UpgradeProgress{
			Status:   models.UpgradeStatusIdle,
			Progress: 0,
		},
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// IsDockerEnvironment 检查是否在 Docker 环境中
func (s *UpgradeService) IsDockerEnvironment() bool {
	return os.Getenv("IN_DOCKER") == "true"
}

// applyProxy 将代理前缀应用到 URL 上
// 代理格式: https://ghproxy.com/ + 原始 URL
func (s *UpgradeService) applyProxy(rawURL, proxyPrefix string) string {
	if proxyPrefix == "" {
		return rawURL
	}
	// 确保代理前缀以 / 结尾
	if proxyPrefix[len(proxyPrefix)-1] != '/' {
		proxyPrefix += "/"
	}
	return proxyPrefix + rawURL
}

// FetchVersionInfo 获取指定版本的信息
// proxyPrefix 为 GitHub 代理前缀，为空则直连
func (s *UpgradeService) FetchVersionInfo(versionType string, proxyPrefix string) (*models.RemoteVersionInfo, error) {
	var rawURL string
	switch versionType {
	case "stable":
		rawURL = stableVersionURL
	case "dev":
		rawURL = devVersionURL
	default:
		return nil, fmt.Errorf("invalid version type: %s", versionType)
	}

	url := s.applyProxy(rawURL, proxyPrefix)

	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch version info: status code %d", resp.StatusCode)
	}

	var versionInfo models.RemoteVersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
		return nil, fmt.Errorf("failed to decode version info: %w", err)
	}

	return &versionInfo, nil
}

// normalizeVersion 去掉版本号前缀 "v"，方便比较
func normalizeVersion(v string) string {
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		return v[1:]
	}
	return v
}

// isNewerVersion 判断远程版本是否比当前版本更新
// 优先用 git_commit 比较，当 commit 为 unknown 时 fallback 到版本号比较
func (s *UpgradeService) isNewerVersion(remoteInfo *models.RemoteVersionInfo) bool {
	currentCommit := version.GitCommit
	currentVersion := version.Version

	// 如果当前 git_commit 已知，直接用 commit 比较
	if currentCommit != "unknown" && currentCommit != "" {
		return remoteInfo.GitCommit != currentCommit
	}

	// git_commit 未知时，用版本号比较（去掉 v 前缀后比较）
	return normalizeVersion(remoteInfo.Version) != normalizeVersion(currentVersion)
}

// CheckForUpdates 检查是否有可用更新
// proxyPrefix 为 GitHub 代理前缀，为空则直连
func (s *UpgradeService) CheckForUpdates(proxyPrefix string) (map[string]*models.RemoteVersionInfo, error) {
	result := make(map[string]*models.RemoteVersionInfo)

	// 检查正式版
	stableInfo, err := s.FetchVersionInfo("stable", proxyPrefix)
	if err == nil && s.isNewerVersion(stableInfo) {
		result["stable"] = stableInfo
	}

	// 检查测试版
	devInfo, err := s.FetchVersionInfo("dev", proxyPrefix)
	if err == nil && s.isNewerVersion(devInfo) {
		result["dev"] = devInfo
	}

	return result, nil
}

// getBaseImageBuildType 获取 Docker 底包的构建类型
// 通过执行底包二进制的 -version 命令解析 Build Type，结果在容器生命周期内缓存
func (s *UpgradeService) getBaseImageBuildType() string {
	s.baseImageBuildTypeOnce.Do(func() {
		// 检查底包是否存在
		if _, err := os.Stat(binarySource); err != nil {
			slog.Warn("base image binary not accessible, falling back to current build type",
				"path", binarySource, "error", err, "fallback", version.BuildType)
			s.baseImageBuildType = version.BuildType
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binarySource, "-version")
		output, err := cmd.Output()
		if err != nil {
			slog.Warn("failed to get base image version, falling back to current build type",
				"error", err, "fallback", version.BuildType)
			s.baseImageBuildType = version.BuildType
			return
		}

		// 解析 Build Type: 行
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Build Type:") {
				buildType := strings.TrimSpace(strings.TrimPrefix(line, "Build Type:"))
				if buildType != "" {
					slog.Info("detected base image build type", "buildType", buildType)
					s.baseImageBuildType = buildType
					return
				}
			}
		}

		slog.Warn("Build Type not found in base image version output, falling back to current build type",
			"fallback", version.BuildType)
		s.baseImageBuildType = version.BuildType
	})
	return s.baseImageBuildType
}

// getPlatformSuffix 获取当前平台的二进制文件后缀
func (s *UpgradeService) getPlatformSuffix() string {
	// 当前只支持 Linux amd64（Docker 环境）
	suffix := "-linux-amd64"
	if s.getBaseImageBuildType() == "full" {
		suffix += "-full"
	}
	return suffix
}

// DownloadBinary 下载二进制文件
// proxyPrefix 为 GitHub 代理前缀，为空则直连
func (s *UpgradeService) DownloadBinary(urlPrefix, targetPath, proxyPrefix string) error {
	s.updateProgress(models.UpgradeStatusDownloading, 0, "正在下载新版本...")

	// 根据平台拼接完整的下载 URL
	rawURL := urlPrefix + s.getPlatformSuffix()
	url := s.applyProxy(rawURL, proxyPrefix)

	// 创建临时文件
	out, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// 发起下载请求
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: status code %d", resp.StatusCode)
	}

	// 获取文件大小
	totalSize := resp.ContentLength

	// 创建进度读取器
	reader := &progressReader{
		reader: resp.Body,
		total:  totalSize,
		onProgress: func(current, total int64) {
			if total > 0 {
				progress := int(float64(current) / float64(total) * 100)
				s.updateProgress(models.UpgradeStatusDownloading, progress, fmt.Sprintf("正在下载新版本... %d%%", progress))
			}
		},
	}

	// 下载文件
	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// TestBinary 测试二进制文件是否可用
func (s *UpgradeService) TestBinary(binaryPath string) error {
	s.updateProgress(models.UpgradeStatusTesting, 0, "正在测试新版本...")

	// 设置可执行权限
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permission: %w", err)
	}

	// 执行 -help 命令测试
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "-help")
	if err := cmd.Run(); err != nil {
		// -help 命令通常会返回非零退出码，这是正常的
		// 只要命令能执行就说明二进制文件可用
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("binary test timeout")
		}
	}

	return nil
}

// UpgradeBinary 执行升级流程
// proxyPrefix 为 GitHub 代理前缀，为空则直连
func (s *UpgradeService) UpgradeBinary(versionType, proxyPrefix string) error {
	// 重置进度
	s.updateProgress(models.UpgradeStatusDownloading, 0, "开始升级...")

	// 1. 获取版本信息
	versionInfo, err := s.FetchVersionInfo(versionType, proxyPrefix)
	if err != nil {
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("获取版本信息失败: %v", err))
		return err
	}

	// 2. 下载新版本
	if err := s.DownloadBinary(versionInfo.DownloadURLPrefix, binaryTemp, proxyPrefix); err != nil {
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("下载失败: %v", err))
		return err
	}

	// 3. 测试新版本
	if err := s.TestBinary(binaryTemp); err != nil {
		os.Remove(binaryTemp)
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("测试失败: %v", err))
		return err
	}

	// 4. 备份当前版本
	s.updateProgress(models.UpgradeStatusReplacing, 50, "正在备份当前版本...")
	if err := s.backupCurrentBinary(); err != nil {
		os.Remove(binaryTemp)
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("备份失败: %v", err))
		return err
	}

	// 5. 替换二进制文件
	s.updateProgress(models.UpgradeStatusReplacing, 75, "正在替换二进制文件...")
	if err := os.Rename(binaryTemp, binaryTarget); err != nil {
		// 替换失败，尝试还原
		s.restoreBackup()
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("替换失败: %v", err))
		return err
	}

	// 6. 设置可执行权限
	if err := os.Chmod(binaryTarget, 0755); err != nil {
		s.restoreBackup()
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("设置权限失败: %v", err))
		return err
	}

	// 7. 升级完成，准备重启
	s.updateProgress(models.UpgradeStatusRestarting, 100, "升级完成，服务即将重启...")

	// 延迟 5 秒后退出，让响应能够返回给客户端
	go func() {
		time.Sleep(5 * time.Second)
		os.Exit(0) // 退出进程，Docker 会自动重启容器
	}()

	return nil
}

// GetProgress 获取当前升级进度
func (s *UpgradeService) GetProgress() models.UpgradeProgress {
	s.progressMutex.RLock()
	defer s.progressMutex.RUnlock()
	return s.progress
}

// updateProgress 更新升级进度
func (s *UpgradeService) updateProgress(status string, progress int, step string) {
	s.progressMutex.Lock()
	defer s.progressMutex.Unlock()
	s.progress = models.UpgradeProgress{
		Status:      status,
		Progress:    progress,
		CurrentStep: step,
	}
}

// backupCurrentBinary 备份当前二进制文件
func (s *UpgradeService) backupCurrentBinary() error {
	// 如果目标文件不存在，无需备份
	if _, err := os.Stat(binaryTarget); os.IsNotExist(err) {
		return nil
	}

	// 读取当前文件
	data, err := os.ReadFile(binaryTarget)
	if err != nil {
		return fmt.Errorf("failed to read current binary: %w", err)
	}

	// 写入备份文件
	if err := os.WriteFile(binaryBackup, data, 0755); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	return nil
}

// restoreBackup 从备份还原
func (s *UpgradeService) restoreBackup() error {
	// 检查备份文件是否存在
	if _, err := os.Stat(binaryBackup); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found")
	}

	// 读取备份文件
	data, err := os.ReadFile(binaryBackup)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// 还原到目标位置
	if err := os.WriteFile(binaryTarget, data, 0755); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	return nil
}

// ResetToBaseImage 回退到 Docker 镜像底包
// 将 /app/mimusic（Docker 镜像中的原始二进制）复制回 /app/data/mimusic，然后重启服务
func (s *UpgradeService) ResetToBaseImage() error {
	s.updateProgress(models.UpgradeStatusResetting, 0, "开始回退到底包版本...")

	// 1. 检查底包文件是否存在
	if _, err := os.Stat(binarySource); os.IsNotExist(err) {
		s.updateProgress(models.UpgradeStatusFailed, 0, "底包文件不存在，无法回退")
		return fmt.Errorf("base image binary not found: %s", binarySource)
	}

	// 2. 备份当前版本
	s.updateProgress(models.UpgradeStatusResetting, 25, "正在备份当前版本...")
	if err := s.backupCurrentBinary(); err != nil {
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("备份失败: %v", err))
		return err
	}

	// 3. 读取底包文件
	s.updateProgress(models.UpgradeStatusResetting, 50, "正在复制底包文件...")
	sourceData, err := os.ReadFile(binarySource)
	if err != nil {
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("读取底包文件失败: %v", err))
		return fmt.Errorf("failed to read base image binary: %w", err)
	}

	// 4. 先写入临时文件，再原子替换
	if err := os.WriteFile(binaryTemp, sourceData, 0755); err != nil {
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("写入临时文件失败: %v", err))
		return fmt.Errorf("failed to write temp binary: %w", err)
	}

	s.updateProgress(models.UpgradeStatusResetting, 75, "正在替换二进制文件...")
	if err := os.Rename(binaryTemp, binaryTarget); err != nil {
		os.Remove(binaryTemp)
		s.restoreBackup()
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("替换失败: %v", err))
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// 5. 设置可执行权限
	if err := os.Chmod(binaryTarget, 0755); err != nil {
		s.restoreBackup()
		s.updateProgress(models.UpgradeStatusFailed, 0, fmt.Sprintf("设置权限失败: %v", err))
		return fmt.Errorf("failed to set executable permission: %w", err)
	}

	// 6. 回退完成，准备重启
	s.updateProgress(models.UpgradeStatusRestarting, 100, "回退完成，服务即将重启...")

	// 延迟 5 秒后退出，让响应能够返回给客户端
	go func() {
		time.Sleep(5 * time.Second)
		os.Exit(0) // 退出进程，Docker 会自动重启容器
	}()

	return nil
}

// progressReader 带进度的读取器
type progressReader struct {
	reader     io.Reader
	total      int64
	current    int64
	onProgress func(current, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)
	if pr.onProgress != nil {
		pr.onProgress(pr.current, pr.total)
	}
	return n, err
}
