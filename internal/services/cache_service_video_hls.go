package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"songloft/internal/mediaexec"
	"songloft/internal/models"
)

// VideoHLSTranscode 将视频文件转码为 HLS（H.264+AAC）供 Web 端播放。
// 返回 playlist.m3u8 的文件路径。首次调用启动 ffmpeg 转码，后续命中缓存直接返回。
// 边转边播：等待首个 segment 生成后即可返回 playlist（ffmpeg 持续追加 segment）。
func (c *CacheService) VideoHLSTranscode(ctx context.Context, srcPath string, song *models.Song) (string, error) {
	if song == nil {
		return "", fmt.Errorf("song is nil")
	}
	if c.getFFmpegPath() == "" {
		return "", fmt.Errorf("ffmpeg not available")
	}

	outDir := c.videoHLSDir(song.ID)
	playlistPath := filepath.Join(outDir, "master.m3u8")

	// 1. 缓存命中：playlist 存在且含 #EXT-X-ENDLIST（转码已完成）
	if isHLSComplete(playlistPath) {
		return playlistPath, nil
	}

	// 2. 转码进行中：playlist 文件存在且目录中已有 .ts segment（ffmpeg 后台仍在写入），
	// 直接返回当前 playlist 让 hls.js 渐进播放，不重启转码。
	if fileExists(playlistPath) && hasAnyTSFile(outDir) {
		return playlistPath, nil
	}

	// 3. inflight 去重
	key := fmt.Sprintf("video_hls_%d", song.ID)
	state := getSongState()
	state.transcodeInflightMu.Lock()
	if dl, ok := state.transcodeInflight[key]; ok {
		state.transcodeInflightMu.Unlock()
		// 等待首个 segment 出现或转码完成
		if err := c.waitForFirstSegment(ctx, outDir, dl.done); err != nil {
			return "", err
		}
		return playlistPath, nil
	}
	dl := &inflightDownload{done: make(chan struct{})}
	state.transcodeInflight[key] = dl
	state.transcodeInflightMu.Unlock()

	// 4. 执行转码
	if err := os.MkdirAll(outDir, 0755); err != nil {
		state.transcodeInflightMu.Lock()
		delete(state.transcodeInflight, key)
		state.transcodeInflightMu.Unlock()
		close(dl.done)
		dl.err = fmt.Errorf("mkdir video_hls dir: %w", err)
		return "", dl.err
	}

	// 清理可能残留的不完整文件（上次被 kill 的遗留）
	os.Remove(playlistPath)

	// 启动 ffmpeg 后台转码（占用转码信号量）
	select {
	case c.transcodeSem <- struct{}{}:
	case <-ctx.Done():
		state.transcodeInflightMu.Lock()
		delete(state.transcodeInflight, key)
		state.transcodeInflightMu.Unlock()
		close(dl.done)
		return "", ctx.Err()
	}

	// 使用 CacheService 的 shutdownCtx 运行 ffmpeg（不绑定 HTTP request 生命周期，
	// 但绑定 App 生命周期：App 关闭时 Shutdown 取消该 ctx，后台 ffmpeg 随之退出，
	// 不再泄漏成孤儿进程）。采用「边转边播」策略：等待首个 segment 就绪后立即返回
	// playlist，hls.js 以 EVENT 模式播放，定期重新拉取 playlist 发现新 segment，
	// 单 video 元素架构下音画天然同步、无 idle 问题。
	// shutdownCtx 为 nil（未经 NewCacheService 构造，如部分单测）时回退到 Background。
	bgCtx := c.shutdownCtx
	if bgCtx == nil {
		bgCtx = context.Background()
	}
	go func() {
		defer func() {
			<-c.transcodeSem
			state.transcodeInflightMu.Lock()
			delete(state.transcodeInflight, key)
			state.transcodeInflightMu.Unlock()
			close(dl.done)
		}()
		if err := c.runVideoHLSFFmpeg(bgCtx, srcPath, outDir); err != nil {
			dl.err = err
			slog.Warn("video HLS transcode failed", "songId", song.ID, "error", err)
		} else {
			slog.Info("video HLS transcode completed", "songId", song.ID, "dir", outDir)
		}
	}()

	// 5. 等待首个 segment 出现（边转边播：首个 segment 就绪即可播放）
	// 传入 dl.done 以便 ffmpeg 快速失败时提前退出（不用等 30s 超时）
	if err := c.waitForFirstSegment(ctx, outDir, dl.done); err != nil {
		return "", err
	}

	return playlistPath, nil
}

// VideoHLSDir 返回视频 HLS 转码输出目录。
func (c *CacheService) VideoHLSDir(songID int64) string {
	return c.videoHLSDir(songID)
}

// videoHLSDir 内部方法，返回视频 HLS 缓存目录。
func (c *CacheService) videoHLSDir(songID int64) string {
	return filepath.Join(c.getCacheDir(), "video_hls", fmt.Sprintf("%d", songID))
}

// runVideoHLSFFmpeg 执行 ffmpeg 将视频转码为 HLS 分片。
// 输出视频流 + 首条音频流到单个 HLS playlist。
// 多音轨切换由前端通过 ?track=N 参数重新请求实现（后续增强）。
func (c *CacheService) runVideoHLSFFmpeg(ctx context.Context, srcPath, outDir string) error {
	playlistPath := filepath.Join(outDir, "master.m3u8")
	segmentPattern := filepath.Join(outDir, "%04d.ts")

	args := []string{
		"-i", srcPath,
		"-map", "0:v:0",
		"-map", "0:a:0",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		"-f", "hls",
		"-hls_time", "4",
		"-hls_list_size", "0",
		// EVENT 类型：进行中的 playlist 带 #EXT-X-PLAYLIST-TYPE:EVENT，
		// hls.js 从 0 秒顺序播放（EVENT 语义：只追加不删除），而非按 LIVE 处理
		// 跳到直播边缘 + 每 targetduration 轮询追帧——后者在边转边播场景下
		// 分片补给慢于消耗，导致首次播放必然反复卡顿。
		// 转码完成后 ffmpeg 自动追加 #EXT-X-ENDLIST，hls.js 无缝转为 VOD。
		"-hls_playlist_type", "event",
		"-hls_segment_filename", segmentPattern,
		"-y",
		playlistPath,
	}

	cmd := mediaexec.FFmpeg(ctx, c.getFFmpegPath(), args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	slog.Info("video HLS transcode started", "src", srcPath, "outDir", outDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg video HLS: %w", err)
	}
	return nil
}

// waitForFirstSegment 轮询等待输出目录中出现首个 .ts 文件。
// doneCh 可选：若外部转码已完成（channel closed）则提前退出。
func (c *CacheService) waitForFirstSegment(ctx context.Context, outDir string, doneCh <-chan struct{}) error {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for first HLS segment")
		case <-ticker.C:
			if hasAnyTSFile(outDir) {
				return nil
			}
		}
		// 如果有 doneCh 且已关闭，再检查一次
		if doneCh != nil {
			select {
			case <-doneCh:
				if hasAnyTSFile(outDir) {
					return nil
				}
				return fmt.Errorf("transcode completed but no segment found")
			default:
			}
		}
	}
}

// hasAnyTSFile 检查目录下是否存在 .ts 文件。
func hasAnyTSFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".ts" {
			return true
		}
	}
	return false
}

// isHLSComplete 检查 HLS 转码是否已完成。
// 对于 master.m3u8：
//   - 单音轨模式：master.m3u8 本身就是 media playlist，含 #EXT-X-ENDLIST
//   - 多音轨模式：master.m3u8 是 master playlist（引用子 playlist），不含 ENDLIST；
//     此时检查子 playlist 是否存在且含 ENDLIST
func isHLSComplete(playlistPath string) bool {
	data, err := os.ReadFile(playlistPath)
	if err != nil {
		return false
	}
	// 如果 master.m3u8 自身含 ENDLIST（单音轨模式），直接返回 true
	if containsEndList(data) {
		return true
	}
	// 多音轨模式：master.m3u8 引用子 playlist，检查第一个子 playlist 是否含 ENDLIST
	dir := filepath.Dir(playlistPath)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && strings.HasSuffix(trimmed, ".m3u8") {
			subPath := filepath.Join(dir, filepath.FromSlash(trimmed))
			subData, err := os.ReadFile(subPath)
			if err != nil {
				return false
			}
			return containsEndList(subData)
		}
	}
	return false
}

// containsEndList 检测 HLS playlist 是否包含 #EXT-X-ENDLIST 标记。
func containsEndList(data []byte) bool {
	return strings.Contains(string(data), "#EXT-X-ENDLIST")
}

// CleanVideoHLS 清理指定歌曲的视频 HLS 缓存目录。
func (c *CacheService) CleanVideoHLS(songID int64) error {
	dir := c.videoHLSDir(songID)
	return os.RemoveAll(dir)
}
