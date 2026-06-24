package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"songloft/internal/models"
)

var ErrUnsupportedTranscodeFormat = errors.New("unsupported transcode format")

// SetFFmpegPath 注入 ffmpeg 可执行文件路径。
func (c *CacheService) SetFFmpegPath(path string) {
	if path != "" {
		if resolved, err := safeLookPath(path); err == nil {
			c.ffmpegPath = resolved
		} else {
			slog.Warn("ffmpeg not found for transcoding", "path", path, "error", err)
			c.ffmpegPath = ""
		}
	} else {
		c.ffmpegPath = ""
	}
}

// ParseBitrate 解析 quality 参数值为 kbps int。
// 仅接受 128/192/320，支持可选的 "k"/"K" 后缀。其他值返回 0（原始音质）。
func ParseBitrate(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "kK")
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	switch n {
	case 128, 192, 320:
		return n
	default:
		return 0
	}
}

// GetOrTranscode 获取转码后的文件路径。
//  1. 原格式==目标格式 且 bitrate==0 → 返回 srcPath
//  2. 转码缓存命中 → 返回缓存路径
//  3. miss → ffmpeg 转码 → 写入缓存 → 返回
//
// srcPath 是原始音频文件路径（本地文件路径或已下载的缓存文件路径）。
// targetFormat 是标准化后的格式名（mp3/ogg/m4a/flac/wav）。
// bitrate 为目标码率（kbps），0 表示使用默认最高质量。
func (c *CacheService) GetOrTranscode(ctx context.Context, srcPath string, song *models.Song, targetFormat string, bitrate int) (string, error) {
	if song == nil {
		return "", errors.New("song is nil")
	}
	srcFmt := EffectiveSourceFormat(song, srcPath)
	needsTranscode := NeedsTranscode(srcFmt, targetFormat) || bitrate > 0
	if !needsTranscode {
		slog.Debug("transcode skipped: same format",
			"songId", song.ID, "songFormat", song.Format,
			"srcFmt", srcFmt, "targetFormat", targetFormat, "bitrate", bitrate, "srcPath", srcPath)
		return srcPath, nil
	}
	slog.Info("transcode needed",
		"songId", song.ID, "songFormat", song.Format,
		"srcFmt", srcFmt, "targetFormat", targetFormat, "bitrate", bitrate, "srcPath", srcPath)

	// 1. 缓存命中
	if p, ok := c.FindTranscodedFile(song, targetFormat, bitrate); ok {
		return p, nil
	}

	// 2. inflight 去重
	inflightKey := fmt.Sprintf("tc_%d_%s_%d", song.ID, targetFormat, bitrate)
	state := getSongState()
	state.transcodeInflightMu.Lock()
	if dl, ok := state.transcodeInflight[inflightKey]; ok {
		state.transcodeInflightMu.Unlock()
		select {
		case <-dl.done:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if dl.err != nil {
			return "", dl.err
		}
		if p, ok := c.FindTranscodedFile(song, targetFormat, bitrate); ok {
			return p, nil
		}
		return "", fmt.Errorf("transcoded file not found after wait")
	}
	dl := &inflightDownload{done: make(chan struct{})}
	state.transcodeInflight[inflightKey] = dl
	state.transcodeInflightMu.Unlock()
	defer func() {
		state.transcodeInflightMu.Lock()
		delete(state.transcodeInflight, inflightKey)
		state.transcodeInflightMu.Unlock()
		close(dl.done)
	}()

	// 3. 转码
	finalPath, err := c.doTranscode(ctx, srcPath, song, targetFormat, bitrate)
	if err != nil {
		dl.err = err
		return "", err
	}

	c.touchSongLRU(song.ID)
	go c.EvictLRU()
	return finalPath, nil
}

// FindTranscodedFile 查找已转码的缓存文件。
// 文件名形如 "{id}.{key}.tc.{format}"、"{id}.tc.{format}" 或含码率的 "{id}.tc.{N}k.{format}"。
func (c *CacheService) FindTranscodedFile(song *models.Song, targetFormat string, bitrate int) (string, bool) {
	if song == nil {
		return "", false
	}
	name := c.transcodedFileName(song, targetFormat, bitrate)
	dir, _ := c.getCachePath(song.ID, "")
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		c.touchSongLRU(song.ID)
		return path, true
	}
	return "", false
}

// doTranscode 执行 ffmpeg 转码并写入缓存。
func (c *CacheService) doTranscode(ctx context.Context, srcPath string, song *models.Song, targetFormat string, bitrate int) (string, error) {
	dir, _ := c.getCachePath(song.ID, "")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir transcode cache dir: %w", err)
	}

	// 临时文件放在目标目录（同设备，rename 无 EXDEV）
	tmp, err := os.CreateTemp(dir, "tc-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()

	if err := c.runFFmpeg(ctx, srcPath, tmpPath, targetFormat, bitrate); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("ffmpeg transcode: %w", err)
	}

	finalName := c.transcodedFileName(song, targetFormat, bitrate)
	finalPath := filepath.Join(dir, finalName)
	if _, err := os.Stat(finalPath); err == nil {
		os.Remove(finalPath)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename transcoded file: %w", err)
	}

	slog.Info("transcode completed", "songId", song.ID, "format", targetFormat, "bitrate", bitrate, "path", finalPath)
	return finalPath, nil
}

// runFFmpeg 调用 ffmpeg 执行转码。
func (c *CacheService) runFFmpeg(ctx context.Context, srcPath, dstPath, targetFormat string, bitrate int) error {
	encoder, qualityArgs, muxer, err := ffmpegArgs(targetFormat, bitrate)
	if err != nil {
		return err
	}

	args := []string{"-i", srcPath, "-vn", "-codec:a", encoder}
	args = append(args, qualityArgs...)
	args = append(args, "-f", muxer, "-y", dstPath)

	ffmpegPath := c.ffmpegPath
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	// 串行化转码，避免并发 ffmpeg 占满 CPU 影响当前播放
	if c.transcodeSem != nil {
		select {
		case c.transcodeSem <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		defer func() { <-c.transcodeSem }()
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("ffmpeg failed", "args", args, "output", string(output), "error", err)
		return fmt.Errorf("ffmpeg exit: %w", err)
	}
	return nil
}

// transcodedFileName 生成转码缓存文件名。
// bitrate > 0 时文件名含码率标记，如 "42.tc.128k.mp3"。
func (c *CacheService) transcodedFileName(song *models.Song, targetFormat string, bitrate int) string {
	idStr := strconv.FormatInt(song.ID, 10)
	key := cacheKeyOf(song)
	var base string
	if key != "" {
		base = idStr + "." + key + ".tc."
	} else {
		base = idStr + ".tc."
	}
	if bitrate > 0 {
		return base + strconv.Itoa(bitrate) + "k." + targetFormat
	}
	return base + targetFormat
}

// NeedsTranscode 判断是否需要转码。
func NeedsTranscode(srcFormat, targetFormat string) bool {
	if targetFormat == "" {
		return false
	}
	normSrc := NormalizeFormat(srcFormat)
	if normSrc == "" {
		return false // 无法识别源格式时不转码，避免对未知/已是同格式文件做无意义转码
	}
	return normSrc != NormalizeFormat(targetFormat)
}

// EffectiveSourceFormat 计算源格式，优先使用 song.Format，
// 为空时回退到 srcPath 的文件扩展名。
// song.Format 存的是 tag 库返回的元数据格式名（如 "ID3v2.3"、"VORBIS"、"MP4"），
// 需要先映射为音频格式；无法确定时回退到文件扩展名。
func EffectiveSourceFormat(song *models.Song, srcPath string) string {
	if song != nil && song.Format != "" {
		if af := tagFormatToAudioFormat(song.Format); af != "" {
			return af
		}
	}
	if srcPath != "" {
		return strings.TrimPrefix(filepath.Ext(srcPath), ".")
	}
	return ""
}

// tagFormatToAudioFormat 将 tag 库返回的元数据格式名映射为音频格式。
// 无法确定（如 VORBIS 可能是 OGG 也可能是 FLAC）时返回空字符串。
func tagFormatToAudioFormat(tagFmt string) string {
	lower := strings.ToLower(tagFmt)
	if strings.HasPrefix(lower, "id3v") {
		return "mp3"
	}
	switch lower {
	case "mp4":
		return "m4a"
	}
	return ""
}

// NormalizeFormat 统一格式名称，处理别名。
func NormalizeFormat(f string) string {
	f = strings.ToLower(strings.TrimPrefix(f, "."))
	switch f {
	case "mpeg", "mp3":
		return "mp3"
	case "mp4", "m4a", "aac":
		return "m4a"
	case "ogg", "vorbis":
		return "ogg"
	case "flac":
		return "flac"
	case "wav", "wave":
		return "wav"
	case "wma", "asf":
		return "wma"
	case "ape":
		return "ape"
	case "aif", "aiff":
		return "aiff"
	}
	return f
}

// ffmpegArgs 根据目标格式和码率返回 ffmpeg 编码器、质量参数和 muxer 格式名。
// bitrate > 0 时有损格式使用 CBR（-b:a Nk），bitrate == 0 时使用默认 VBR 最高质量。
// 无损格式（flac/wav）忽略 bitrate。
func ffmpegArgs(targetFormat string, bitrate int) (encoder string, qualityArgs []string, muxer string, err error) {
	bitrateArg := strconv.Itoa(bitrate) + "k"
	switch NormalizeFormat(targetFormat) {
	case "mp3":
		if bitrate > 0 {
			return "libmp3lame", []string{"-b:a", bitrateArg}, "mp3", nil
		}
		return "libmp3lame", []string{"-q:a", "0"}, "mp3", nil
	case "ogg":
		if bitrate > 0 {
			return "libvorbis", []string{"-b:a", bitrateArg}, "ogg", nil
		}
		return "libvorbis", []string{"-q:a", "6"}, "ogg", nil
	case "m4a":
		if bitrate > 0 {
			return "aac", []string{"-b:a", bitrateArg}, "ipod", nil
		}
		return "aac", []string{"-b:a", "256k"}, "ipod", nil
	case "flac":
		return "flac", nil, "flac", nil
	case "wav":
		return "pcm_s16le", nil, "wav", nil
	default:
		return "", nil, "", ErrUnsupportedTranscodeFormat
	}
}
