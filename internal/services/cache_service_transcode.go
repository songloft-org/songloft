package services

import (
	"context"
	"encoding/json"
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
//  1. 原格式==目标格式 且 bitrate==0 且未指定抽轨 → 返回 srcPath
//  2. 转码缓存命中 → 返回缓存路径
//  3. miss → ffmpeg 转码 → 写入缓存 → 返回
//
// srcPath 是原始音频文件路径（本地文件路径或已下载的缓存文件路径）。
// targetFormat 是标准化后的格式名（mp3/ogg/m4a/flac/wav）。
// bitrate 为目标码率（kbps），0 表示使用默认最高质量。
// trackIndex 为要抽取的音频流（audio-relative 0-based，对应 ffmpeg -map 0:a:N，
// songloft-org/songloft#298）；< 0 表示不抽轨、保持原有行为。指定时强制走 ffmpeg，
// 且缓存文件名含 track 维度，不同轨缓存互不覆盖。
func (c *CacheService) GetOrTranscode(ctx context.Context, srcPath string, song *models.Song, targetFormat string, bitrate, trackIndex int) (string, error) {
	if song == nil {
		return "", errors.New("song is nil")
	}
	srcFmt := EffectiveSourceFormat(song, srcPath)
	isCue := song.CueSourcePath != ""
	// 抽轨（trackIndex >= 0）必须走 ffmpeg：即使容器/编码相同也要 -map 出单条音轨。
	// CUE track 必须走 ffmpeg 提取对应时间段。
	needsTranscode := NeedsTranscode(srcFmt, targetFormat) || bitrate > 0 || trackIndex >= 0 || isCue
	if !needsTranscode {
		slog.Debug("transcode skipped: same format",
			"songId", song.ID, "songFormat", song.Format,
			"srcFmt", srcFmt, "targetFormat", targetFormat, "bitrate", bitrate, "srcPath", srcPath)
		return srcPath, nil
	}
	slog.Info("transcode needed",
		"songId", song.ID, "songFormat", song.Format,
		"srcFmt", srcFmt, "targetFormat", targetFormat, "bitrate", bitrate, "trackIndex", trackIndex, "srcPath", srcPath)

	// 1. 缓存命中
	if p, ok := c.FindTranscodedFile(song, targetFormat, bitrate, trackIndex); ok {
		return p, nil
	}

	// 2. inflight 去重
	inflightKey := fmt.Sprintf("tc_%d_%s_%d_t%d", song.ID, targetFormat, bitrate, trackIndex)
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
		if p, ok := c.FindTranscodedFile(song, targetFormat, bitrate, trackIndex); ok {
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
	finalPath, err := c.doTranscode(ctx, srcPath, song, targetFormat, bitrate, trackIndex)
	if err != nil {
		dl.err = err
		return "", err
	}

	go c.EvictLRU()
	return finalPath, nil
}

// FindTranscodedFile 查找已转码的缓存文件。
// 文件名形如 "{id}.{key}.tc.{format}"、"{id}.tc.{format}"、含码率的 "{id}.tc.{N}k.{format}"，
// 或含抽轨标记的 "{id}.tc.a{idx}.{format}"（songloft-org/songloft#298）。
func (c *CacheService) FindTranscodedFile(song *models.Song, targetFormat string, bitrate, trackIndex int) (string, bool) {
	if song == nil {
		return "", false
	}
	name := c.transcodedFileName(song, targetFormat, bitrate, trackIndex)
	dir, _ := c.getCachePath(song.ID, "")
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	return "", false
}

// doTranscode 执行 ffmpeg 转码并写入缓存。
func (c *CacheService) doTranscode(ctx context.Context, srcPath string, song *models.Song, targetFormat string, bitrate, trackIndex int) (string, error) {
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

	if err := c.runFFmpeg(ctx, srcPath, tmpPath, song, targetFormat, bitrate, trackIndex); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("ffmpeg transcode: %w", err)
	}

	finalName := c.transcodedFileName(song, targetFormat, bitrate, trackIndex)
	finalPath := filepath.Join(dir, finalName)
	if _, err := os.Stat(finalPath); err == nil {
		os.Remove(finalPath)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename transcoded file: %w", err)
	}

	slog.Info("transcode completed", "songId", song.ID, "format", targetFormat, "bitrate", bitrate, "trackIndex", trackIndex, "path", finalPath)
	return finalPath, nil
}

// runFFmpeg 调用 ffmpeg 执行转码。
// trackIndex >= 0 时抽取指定音轨（-map 0:a:N，songloft-org/songloft#298）。
// CUE track（song.CueStartSeconds/CueEndSeconds > 0）时使用 input seek（-ss 在 -i 之前）
// 提取对应时间段，同格式时 stream copy 免重编码。
func (c *CacheService) runFFmpeg(ctx context.Context, srcPath, dstPath string, song *models.Song, targetFormat string, bitrate, trackIndex int) error {
	isCue := song != nil && song.CueSourcePath != ""
	hasCueTiming := isCue && (song.CueStartSeconds > 0 || song.CueEndSeconds > 0)

	var args []string

	// CUE track: -ss 放在 -i 之前使用 input seek（O(1) 性能）
	if hasCueTiming {
		args = append(args, "-ss", fmt.Sprintf("%.3f", song.CueStartSeconds))
	}

	if trackIndex >= 0 && NormalizeFormat(targetFormat) == "m4a" {
		args = append(args, "-i", srcPath)
		if hasCueTiming && song.CueEndSeconds > song.CueStartSeconds {
			args = append(args, "-t", fmt.Sprintf("%.3f", song.CueEndSeconds-song.CueStartSeconds))
		}
		args = append(args, "-map", fmt.Sprintf("0:a:%d", trackIndex), "-c:a", "copy", "-f", "ipod", "-y", dstPath)
	} else {
		// CUE 同格式提取使用 stream copy 免重编码；APE 源已在上游转为 flac targetFormat
		srcFmt := NormalizeFormat(filepath.Ext(srcPath))
		useCopy := isCue && !NeedsTranscode(srcFmt, targetFormat) && bitrate <= 0 && trackIndex < 0

		var encoder string
		var qualityArgs []string
		var muxer string
		if useCopy {
			encoder = "copy"
			switch targetFormat {
			case "flac":
				muxer = "flac"
			case "wav":
				muxer = "wav"
			case "mp3":
				muxer = "mp3"
			case "m4a":
				muxer = "ipod"
			case "ogg":
				muxer = "ogg"
			default:
				muxer = targetFormat
			}
		} else {
			var err error
			encoder, qualityArgs, muxer, err = ffmpegArgs(targetFormat, bitrate)
			if err != nil {
				return err
			}
		}

		args = append(args, "-i", srcPath)
		if hasCueTiming && song.CueEndSeconds > song.CueStartSeconds {
			args = append(args, "-t", fmt.Sprintf("%.3f", song.CueEndSeconds-song.CueStartSeconds))
		}
		if trackIndex >= 0 {
			args = append(args, "-map", fmt.Sprintf("0:a:%d", trackIndex))
		}
		args = append(args, "-vn", "-codec:a", encoder)
		args = append(args, qualityArgs...)
		args = append(args, "-f", muxer, "-y", dstPath)
	}

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
// trackIndex >= 0 时文件名含抽轨标记，如 "42.tc.a1.m4a"（songloft-org/songloft#298），
// 保证不同音轨的缓存互不覆盖。
func (c *CacheService) transcodedFileName(song *models.Song, targetFormat string, bitrate, trackIndex int) string {
	idStr := strconv.FormatInt(song.ID, 10)
	key := cacheKeyOf(song)
	var base string
	if key != "" {
		base = idStr + "." + key + ".tc."
	} else {
		base = idStr + ".tc."
	}
	if bitrate > 0 {
		base += strconv.Itoa(bitrate) + "k."
	}
	if trackIndex >= 0 {
		base += "a" + strconv.Itoa(trackIndex) + "."
	}
	return base + targetFormat
}

// AudioTrackInfo 单条音频流的元信息（songloft-org/songloft#298）。
// Index 为 audio-relative 0-based 序号（对应 ffmpeg -map 0:a:N）。
type AudioTrackInfo struct {
	Index    int    `json:"index"`
	Title    string `json:"title"`
	Language string `json:"language"`
	Codec    string `json:"codec"`
	Default  bool   `json:"default"`
}

// resolveFFprobePath 推断 ffprobe 可执行文件路径。
// CacheService 仅持有 ffmpegPath（由 app.go 注入）；ffprobe 通常与 ffmpeg 同目录，
// 故优先取其同级 ffprobe，其次在 PATH 中查找，最后回退裸名 "ffprobe"。
func (c *CacheService) resolveFFprobePath() string {
	if c.ffmpegPath != "" {
		dir := filepath.Dir(c.ffmpegPath)
		base := filepath.Base(c.ffmpegPath)
		if probeName := strings.Replace(base, "ffmpeg", "ffprobe", 1); probeName != base {
			cand := filepath.Join(dir, probeName)
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				return cand
			}
		}
	}
	if p, err := safeLookPath("ffprobe"); err == nil {
		return p
	}
	return "ffprobe"
}

// ListAudioTracks 用 ffprobe 探测文件的音频流列表（songloft-org/songloft#298）。
// 只枚举音频流（-select_streams a），返回的 Index 即 audio-relative 0-based 序号。
func (c *CacheService) ListAudioTracks(ctx context.Context, filePath string) ([]AudioTrackInfo, error) {
	cmd := exec.CommandContext(ctx, c.resolveFFprobePath(),
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "a",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe audio tracks: %w", err)
	}
	var parsed struct {
		Streams []FFProbeStream `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}
	tracks := make([]AudioTrackInfo, 0, len(parsed.Streams))
	for i, s := range parsed.Streams {
		var title, lang string
		if s.Tags != nil {
			title = pickTag(s.Tags, "title", "TITLE")
			lang = pickTag(s.Tags, "language", "LANGUAGE")
		}
		tracks = append(tracks, AudioTrackInfo{
			Index:    i, // audio-relative（因 -select_streams a 只返回音频流）
			Title:    title,
			Language: lang,
			Codec:    strings.ToLower(s.CodecName),
			Default:  s.Disposition["default"] == 1,
		})
	}
	return tracks, nil
}

// isWebFriendlyCopyCodec 判断音频编码是否可无损 remux 进 m4a 容器并被 Web 原生播放。
// AAC 是 Web 原生可播的 codec，可直接 -c:a copy 进 m4a；其余编码需重新编码。
func isWebFriendlyCopyCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "aac":
		return true
	default:
		return false
	}
}

// PlanTrackExtraction 决定抽取指定音轨时的目标容器格式（songloft-org/songloft#298）。
// 该轨为 AAC → 返回 "m4a"（runFFmpeg 会走无损 copy-remux）；否则 → "mp3"（编码为 Web 可播）。
// 探测失败或索引越界时保守返回 "mp3"。
func (c *CacheService) PlanTrackExtraction(ctx context.Context, srcPath string, trackIndex int) string {
	if trackIndex < 0 {
		return ""
	}
	if tracks, err := c.ListAudioTracks(ctx, srcPath); err == nil {
		if trackIndex < len(tracks) && isWebFriendlyCopyCodec(tracks[trackIndex].Codec) {
			return "m4a"
		}
	}
	return "mp3"
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
	// Matroska 音频容器（songloft-org/songloft#297）：与 mkv 视频区分，保留 mka 语义。
	case "mka":
		return "mka"
	// 视频容器（songloft-org/songloft#76）：归一化 ffprobe 返回的容器名，使抽音频转码判定稳定。
	case "mkv", "matroska":
		return "mkv"
	case "webm":
		return "webm"
	case "avi":
		return "avi"
	case "ts", "mpegts", "mp2t":
		return "ts"
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
