package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"songloft/internal/httputil"
	"songloft/internal/models"
)

// AutoDownloadConfig 自动下载配置（由插件通过 bridge API 注册）
type AutoDownloadConfig struct {
	Enabled       bool   `json:"enabled"`
	PathTemplate  string `json:"path_template"`
	EmbedMetadata bool   `json:"embed_metadata"`
}

// SongDownloadOptions 下载选项
type SongDownloadOptions struct {
	TargetDir     string `json:"target_dir"`
	PathTemplate  string `json:"path_template"`
	EmbedMetadata bool   `json:"embed_metadata"`
}

// SongDownloadResult 下载结果
type SongDownloadResult struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// SongDownloader 歌曲下载服务（将远程歌曲下载到本地音乐库）
type SongDownloader struct {
	songService    *SongService
	cacheService   *CacheService
	configService  *ConfigService
	getMusicPath   func() string
	lyricFetcher   *LyricFetcher
	downloadClient *http.Client

	autoDownloadMu     sync.RWMutex
	autoDownloadConfig *AutoDownloadConfig
}

// NewSongDownloader 创建下载服务
func NewSongDownloader(
	songService *SongService,
	cacheService *CacheService,
	configService *ConfigService,
	getMusicPath func() string,
	lyricFetcher *LyricFetcher,
) *SongDownloader {
	return &SongDownloader{
		songService:   songService,
		cacheService:  cacheService,
		configService: configService,
		getMusicPath:  getMusicPath,
		lyricFetcher:  lyricFetcher,
		downloadClient: httputil.NewClient(120 * time.Second),
	}
}

// Download 下载远程歌曲到本地。
//
// 流程：
//  1. 获取歌曲信息并验证为 remote 类型
//  2. 确定目标目录（opts.TargetDir 或 music_path）
//  3. 获取音频文件（缓存命中则 copy，否则同步下载）
//  4. 按路径模板生成目标路径并移入
//  5. 可选嵌入元数据
//  6. 更新 DB：type=local, file_path=目标路径
func (d *SongDownloader) Download(ctx context.Context, songID int64, opts SongDownloadOptions) (*SongDownloadResult, error) {
	song, err := d.songService.GetByID(ctx, songID)
	if err != nil {
		return nil, fmt.Errorf("song not found: %w", err)
	}
	if song.Type != models.TypeRemote {
		return nil, fmt.Errorf("only remote songs can be downloaded, got type=%s", song.Type)
	}

	targetDir, err := d.resolveTargetDir(opts.TargetDir)
	if err != nil {
		return nil, err
	}

	tplStr := opts.PathTemplate
	if tplStr == "" {
		tplStr = defaultPathTemplate
	}
	tpl, err := ParsePathTemplate(tplStr)
	if err != nil {
		return nil, fmt.Errorf("invalid path template: %w", err)
	}

	srcPath, err := d.acquireAudio(ctx, song)
	if err != nil {
		return nil, fmt.Errorf("acquire audio: %w", err)
	}

	ext := filepath.Ext(srcPath)
	rendered := tpl.Render(song)
	relPath := filepath.FromSlash(rendered) + ext
	destPath := filepath.Join(targetDir, relPath)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	if err := copyFile(srcPath, destPath); err != nil {
		return nil, fmt.Errorf("copy to target: %w", err)
	}

	if opts.EmbedMetadata {
		// URL 歌词：下载时拉取并写入文件，同时缓存到 DB
		if song.LyricSource == models.LyricSourceURL && song.LyricRemoteURL != "" && d.lyricFetcher != nil {
			payload, err := d.lyricFetcher.Fetch(ctx, song.LyricRemoteURL)
			if err != nil {
				slog.Warn("download: fetch url lyrics failed, skipping",
					"songId", songID, "url", song.LyricRemoteURL, "error", err)
			} else if payload.Lyric != "" {
				song.Lyric = payload.MarshalString()
				song.LyricSource = models.LyricSourceEmbedded
				song.LyricRemoteURL = ""
			}
		}

		status := WriteCacheSongTags(destPath, song, d.downloadClient)
		slog.Debug("download: metadata embed", "songId", songID, "status", status)
	}

	song.Type = models.TypeLocal
	song.FilePath = destPath
	song.URL = ""
	song.PluginEntryPath = ""
	song.SourceData = ""
	song.CachePath = ""
	if err := d.songService.Update(ctx, song); err != nil {
		return nil, fmt.Errorf("update song: %w", err)
	}

	return &SongDownloadResult{
		Path:   destPath,
		Status: "ok",
	}, nil
}

// resolveTargetDir 解析目标目录并验证安全性。
func (d *SongDownloader) resolveTargetDir(targetDir string) (string, error) {
	musicPath := ""
	if d.getMusicPath != nil {
		musicPath = d.getMusicPath()
	}

	if targetDir == "" {
		if musicPath == "" {
			return "", errors.New("music_path not configured and no target_dir specified")
		}
		return musicPath, nil
	}

	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("invalid target dir: %w", err)
	}

	if musicPath != "" {
		absMusicPath, _ := filepath.Abs(musicPath)
		if !strings.HasPrefix(absTarget+string(filepath.Separator), absMusicPath+string(filepath.Separator)) &&
			absTarget != absMusicPath {
			return "", fmt.Errorf("target_dir must be under music_path (%s)", absMusicPath)
		}
	}

	return absTarget, nil
}

// acquireAudio 获取音频文件路径（缓存命中或同步下载）。
func (d *SongDownloader) acquireAudio(ctx context.Context, song *models.Song) (string, error) {
	if p, ok := d.cacheService.FindCachedFileBySong(song); ok {
		return p, nil
	}
	return d.cacheService.Get(ctx, song)
}

// SetAutoDownloadConfig 设置自动下载配置（由插件通过 bridge API 调用）。
func (d *SongDownloader) SetAutoDownloadConfig(config *AutoDownloadConfig) {
	d.autoDownloadMu.Lock()
	defer d.autoDownloadMu.Unlock()
	d.autoDownloadConfig = config
	slog.Info("auto-download config updated", "enabled", config.Enabled,
		"pathTemplate", config.PathTemplate, "embedMetadata", config.EmbedMetadata)
}

// GetAutoDownloadConfig 获取当前自动下载配置。
func (d *SongDownloader) GetAutoDownloadConfig() AutoDownloadConfig {
	d.autoDownloadMu.RLock()
	defer d.autoDownloadMu.RUnlock()
	if d.autoDownloadConfig == nil {
		return AutoDownloadConfig{}
	}
	return *d.autoDownloadConfig
}

// TryAutoDownload 在缓存完成后尝试自动下载（由 onCacheComplete 回调调用）。
// 仅当自动下载已开启且歌曲来自插件源时才执行。
func (d *SongDownloader) TryAutoDownload(ctx context.Context, song *models.Song) {
	config := d.GetAutoDownloadConfig()
	if !config.Enabled {
		return
	}
	if song.PluginEntryPath == "" {
		return
	}
	if song.Type != models.TypeRemote {
		return
	}

	result, err := d.Download(ctx, song.ID, SongDownloadOptions{
		PathTemplate:  config.PathTemplate,
		EmbedMetadata: config.EmbedMetadata,
	})
	if err != nil {
		slog.Warn("auto-download failed", "songID", song.ID, "title", song.Title, "error", err)
		return
	}
	slog.Info("auto-download completed", "songID", song.ID, "title", song.Title, "path", result.Path)
}

// copyFile 复制文件。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
