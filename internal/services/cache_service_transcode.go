package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"songloft/internal/mediaexec"
	"songloft/internal/models"
)

var ErrUnsupportedTranscodeFormat = errors.New("unsupported transcode format")

// loudnormFilter 是音量均衡（EBU R128）的 ffmpeg 滤镜串，songloft-org/songloft#315。
//
// 这是 I=-16（默认目标响度）时的字面量，保留给测试作默认断言基准。生产代码用
// CacheService.normalizeLoudnessFilter()——它从 config 读取可自定义的目标响度
// （songloft-org/songloft-player-flutter#38），默认 -16 时与该常量逐字相同。
//
// 落盘转码（runFFmpeg）与「均衡产物还没转好时的边转边发」（StreamSeekedMP3 的 Normalize 分支，
// songloft-org/songloft-plugin-miot#61）共用同一方法：两条路径必须给出同一响度，
// 否则同一首歌走缓存与走实时流听起来音量不一样。
//
// 单遍（动态）loudnorm，不做两遍分析——正因如此实时流与落盘结果一致。
// 两遍（linear=true）需先整曲测量，无法用于 pipe 实时流，且会让两条路径动态范围不一致。
const loudnormFilter = "loudnorm=I=-16:LRA=11:TP=-1.5"

// VolumeNormalizeLoudnessKey 是目标响度（LUFS）的 config key，songloft-org/songloft-player-flutter#38。
// 值为字符串形式的浮点数（如 "-16"、"-14"）；缺失或非法时回落 defaultNormalizeLoudness。
// 与 volume_normalize（开关）分两个 key 存：开关仍是 bool，响度是独立可选项，向后兼容。
// 导出：handler 写入需引用同一 key 字符串，避免与 services 读取端漂移。
const VolumeNormalizeLoudnessKey = "volume_normalize_loudness"

// defaultNormalizeLoudness 是目标响度的默认值（LUFS），与 loudnormFilter 的 I=-16 一致。
const defaultNormalizeLoudness = -16.0

// minNormalizeLoudness / maxNormalizeLoudness 是允许的目标响度区间（LUFS）：
// 下限 -40 覆盖极安静内容，上限 -5 防止接近 0 dB 引发削波。
const (
	minNormalizeLoudness = -40.0
	maxNormalizeLoudness = -5.0
)

// ValidateNormalizeLoudness 校验目标响度是否在允许区间内，供 handler 在写入前调用。
// 返回 error 而非夹紧：设置端点应对用户输入报 400，而非静默改成另一个值。
func ValidateNormalizeLoudness(v float64) error {
	if v < minNormalizeLoudness || v > maxNormalizeLoudness {
		return fmt.Errorf("音量均衡响度 %g 越界，需在 %g ~ %g LUFS", v, minNormalizeLoudness, maxNormalizeLoudness)
	}
	return nil
}

// NormalizeLoudness 读取用户自定义的目标响度（LUFS），songloft-org/songloft-player-flutter#38。
// nil configService（测试里 &CacheService{} 直接构造）/ 缺失 key / 解析失败 / 越界都回落
// defaultNormalizeLoudness 或夹到边界，保证默认行为与历史一致、坏配置不崩转码。
// 读 config 而非持有字段：两条写入入口（/settings/volume-normalize 与 admin /configs/{key}）
// 都即时生效，无需额外同步。导出供 handler 回显当前值；CacheService 内部转码路径
// （滤镜串/缓存键）也复用同一读取。
func (c *CacheService) NormalizeLoudness() float64 {
	if c == nil || c.configService == nil {
		return defaultNormalizeLoudness
	}
	s := c.configService.GetString(VolumeNormalizeLoudnessKey, "")
	if s == "" {
		return defaultNormalizeLoudness
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		slog.Warn("音量均衡响度配置非法，使用默认值", "key", VolumeNormalizeLoudnessKey, "value", s, "default", defaultNormalizeLoudness)
		return defaultNormalizeLoudness
	}
	if v < minNormalizeLoudness {
		return minNormalizeLoudness
	}
	if v > maxNormalizeLoudness {
		return maxNormalizeLoudness
	}
	return v
}

// normalizeLoudnessFilter 返回带当前目标响度的 loudnorm 滤镜串。
// 用 %g：-16 → "-16"（与 loudnormFilter 常量逐字相同），-14.5 → "-14.5"。
// LRA/TP 固定（EBU R128 推荐值），只开放 I 给用户自定义。
func (c *CacheService) normalizeLoudnessFilter() string {
	return fmt.Sprintf("loudnorm=I=%g:LRA=11:TP=-1.5", c.NormalizeLoudness())
}

// normalizeLoudnessTag 是缓存文件名/inflight key 里标记均衡产物的片段，含响度值
// （如 "norm-16" / "norm-14"）。不同响度的产物互不共用缓存——用户改响度后旧缓存
// 自然失效（找不到文件即重转），与 norm 标记区分均衡/非均衡产物同理。用 %g 与滤镜串一致。
func (c *CacheService) normalizeLoudnessTag() string {
	return fmt.Sprintf("norm%g", c.NormalizeLoudness())
}

// SetFFmpegPath 注入 ffmpeg 可执行文件路径。
func (c *CacheService) SetFFmpegPath(path string) {
	if path != "" {
		if resolved, err := safeLookPath(path); err == nil {
			c.ffmpegPath.Store(resolved)
		} else {
			slog.Warn("ffmpeg not found for transcoding", "path", path, "error", err)
			c.ffmpegPath.Store("")
		}
	} else {
		c.ffmpegPath.Store("")
	}
}

// SetBundledFFprobePath sets the host-supplied probe before starting the HTTP server.
func (c *CacheService) SetBundledFFprobePath(path string) {
	c.ffprobePath = path
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
//  1. 原格式==目标格式 且 bitrate==0 且未指定抽轨 且不均衡 → 返回 srcPath
//  2. 转码缓存命中 → 返回缓存路径
//  3. miss → ffmpeg 转码 → 写入缓存 → 返回
//
// srcPath 是原始音频文件路径（本地文件路径或已下载的缓存文件路径）。
// targetFormat 是标准化后的格式名（mp3/ogg/m4a/flac/wav）。
// bitrate 为目标码率（kbps），0 表示使用默认最高质量。
// trackIndex 为要抽取的音频流（audio-relative 0-based，对应 ffmpeg -map 0:a:N，
// songloft-org/songloft#298）；< 0 表示不抽轨、保持原有行为。指定时强制走 ffmpeg，
// 且缓存文件名含 track 维度，不同轨缓存互不覆盖。
// normalize 为 true 时使用 EBU R128 loudnorm 滤镜均衡音量（songloft-org/songloft#315）。
func (c *CacheService) GetOrTranscode(ctx context.Context, srcPath string, song *models.Song, targetFormat string, bitrate, trackIndex int, normalize bool) (string, error) {
	if song == nil {
		return "", errors.New("song is nil")
	}
	srcFmt := EffectiveSourceFormat(song, srcPath)
	isCue := song.CueSourcePath != ""
	// 抽轨（trackIndex >= 0）必须走 ffmpeg：即使容器/编码相同也要 -map 出单条音轨。
	// CUE track 必须走 ffmpeg 提取对应时间段。
	needsTranscode := NeedsTranscode(srcFmt, targetFormat) || bitrate > 0 || trackIndex >= 0 || isCue || normalize
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
	if p, ok := c.FindTranscodedFile(song, targetFormat, bitrate, trackIndex, normalize); ok {
		return p, nil
	}

	// 2. inflight 去重。normFlag 含响度值，保证不同响度的并发转码不共用 inflight 槽
	// （songloft-org/songloft-player-flutter#38）。与 transcodedFileName 的 norm 标记同源。
	normFlag := ""
	if normalize {
		normFlag = "_norm" + c.normalizeLoudnessTag()
	}
	inflightKey := fmt.Sprintf("tc_%d_%s_%d_t%d%s", song.ID, targetFormat, bitrate, trackIndex, normFlag)
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
		if p, ok := c.FindTranscodedFile(song, targetFormat, bitrate, trackIndex, normalize); ok {
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
	finalPath, err := c.doTranscode(ctx, srcPath, song, targetFormat, bitrate, trackIndex, normalize)
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
func (c *CacheService) FindTranscodedFile(song *models.Song, targetFormat string, bitrate, trackIndex int, normalize bool) (string, bool) {
	if song == nil {
		return "", false
	}
	name := c.transcodedFileName(song, targetFormat, bitrate, trackIndex, normalize)
	dir, _ := c.getCachePath(song.ID, "")
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	return "", false
}

// doTranscode 执行 ffmpeg 转码并写入缓存。
func (c *CacheService) doTranscode(ctx context.Context, srcPath string, song *models.Song, targetFormat string, bitrate, trackIndex int, normalize bool) (string, error) {
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

	if err := c.runFFmpeg(ctx, srcPath, tmpPath, song, targetFormat, bitrate, trackIndex, normalize); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("ffmpeg transcode: %w", err)
	}

	finalName := c.transcodedFileName(song, targetFormat, bitrate, trackIndex, normalize)
	finalPath := filepath.Join(dir, finalName)
	if _, err := os.Stat(finalPath); err == nil {
		os.Remove(finalPath)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename transcoded file: %w", err)
	}

	slog.Info("transcode completed", "songId", song.ID, "format", targetFormat, "bitrate", bitrate, "trackIndex", trackIndex, "normalize", normalize, "path", finalPath)
	return finalPath, nil
}

// runFFmpeg 调用 ffmpeg 执行转码。
// trackIndex >= 0 时抽取指定音轨（-map 0:a:N，songloft-org/songloft#298）。
// CUE track（song.CueStartSeconds/CueEndSeconds > 0）时使用 input seek（-ss 在 -i 之前）
// 提取对应时间段，同格式时 stream copy 免重编码。
// normalize 为 true 时插入 -af loudnorm 滤镜均衡音量（songloft-org/songloft#315）。
func (c *CacheService) runFFmpeg(ctx context.Context, srcPath, dstPath string, song *models.Song, targetFormat string, bitrate, trackIndex int, normalize bool) error {
	isCue := song != nil && song.CueSourcePath != ""
	hasCueTiming := isCue && (song.CueStartSeconds > 0 || song.CueEndSeconds > 0)

	var args []string

	// CUE track: -ss 放在 -i 之前使用 input seek（O(1) 性能）
	if hasCueTiming {
		args = append(args, "-ss", fmt.Sprintf("%.3f", song.CueStartSeconds))
	}

	if trackIndex >= 0 && NormalizeFormat(targetFormat) == "m4a" && !normalize {
		args = append(args, "-i", srcPath)
		if hasCueTiming && song.CueEndSeconds > song.CueStartSeconds {
			args = append(args, "-t", fmt.Sprintf("%.3f", song.CueEndSeconds-song.CueStartSeconds))
		}
		args = append(args, "-map", fmt.Sprintf("0:a:%d", trackIndex), "-c:a", "copy", "-f", "ipod", "-y", dstPath)
	} else {
		// CUE 同格式提取使用 stream copy 免重编码；APE 源已在上游转为 flac targetFormat
		// normalize 需要重编码，不能使用 stream copy
		srcFmt := NormalizeFormat(filepath.Ext(srcPath))
		useCopy := isCue && !NeedsTranscode(srcFmt, targetFormat) && bitrate <= 0 && trackIndex < 0 && !normalize

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
		if normalize && encoder != "copy" {
			args = append(args, "-af", c.normalizeLoudnessFilter())
		}
		args = append(args, qualityArgs...)
		args = append(args, "-f", muxer, "-y", dstPath)
	}

	ffmpegPath := c.getFFmpegPath()
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

	cmd := mediaexec.FFmpeg(ctx, ffmpegPath, args...)
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
func (c *CacheService) transcodedFileName(song *models.Song, targetFormat string, bitrate, trackIndex int, normalize bool) string {
	idStr := strconv.FormatInt(song.ID, 10)
	key := cacheKeyOf(song)
	var base string
	if key != "" {
		base = idStr + "." + key + ".tc."
	} else {
		base = idStr + ".tc."
	}
	if normalize {
		base += c.normalizeLoudnessTag() + "."
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
	if c.ffprobePath != "" {
		return c.ffprobePath
	}
	if fp := c.getFFmpegPath(); fp != "" {
		dir := filepath.Dir(fp)
		base := filepath.Base(fp)
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
	cmd := mediaexec.FFprobe(ctx, c.resolveFFprobePath(),
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

// NeedsTranscodeForServe 判断「播放 / 预热」时是否需要转码，是内容感知版的 NeedsTranscode。
//
// 在 NeedsTranscode（仅比对声称格式）基础上，额外用 magic bytes 复核文件真实容器：
// 当源与目标声称同格式、NeedsTranscode 因此短路时，若文件实际内容并非该格式（典型：
// YouTube 的 WebM/Opus 音频被错误地存成 .mp3 扩展名的「伪 mp3」），仍强制转码修正，
// 避免把设备（如小爱音箱）无法解码的 WebM 原样推给客户端（songloft-org/songloft#300）。
//
// 与 EnsureCachedFormat 里的守卫同源（复用 matchesFormatMagic），区别在于这里作用于
// 按需播放路径（miot「强制 MP3」的 ?format= 请求），而 EnsureCachedFormat 作用于
// 全局缓存转码落盘。targetFormat 为空 → 永不转码（不强制格式，透传原码）。
func NeedsTranscodeForServe(song *models.Song, srcPath, targetFormat string) bool {
	if targetFormat == "" {
		return false
	}
	srcFmt := EffectiveSourceFormat(song, srcPath)
	if NeedsTranscode(srcFmt, targetFormat) {
		return true
	}
	// 到这里：srcFmt 未知（"") 或声称与目标同格式。仅在「声称同格式」时用 magic bytes
	// 复核——防伪装扩展名绕过转码；保留「源格式未知不转码」的既有行为不动。
	if srcPath != "" && NormalizeFormat(srcFmt) == NormalizeFormat(targetFormat) &&
		!matchesFormatMagic(srcPath, targetFormat) {
		return true
	}
	return false
}

// EffectiveSourceFormat 计算源格式，优先使用 song.Format，
// 为空时回退到 srcPath 的文件扩展名。
// song.Format 在扫描时由 NormalizeFormat(extension) 或 ffprobe format_name 填充，
// 已是归一化的格式标识（如 "mp3"、"mpg"、"wmv"）；tagFormatToAudioFormat 处理
// 遗留条目中可能存在的 tag 格式名（如 "ID3v2.3" → "mp3"）。
func EffectiveSourceFormat(song *models.Song, srcPath string) string {
	if song != nil && song.Format != "" {
		if af := tagFormatToAudioFormat(song.Format); af != "" {
			return af
		}
		// song.Format 已是归一化的格式标识，直接使用（如 "mpg"、"wmv"、"flv"）。
		return song.Format
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
	case "mp4", "m4a", "aac", "m4b":
		return "m4a"
	case "ogg", "vorbis", "oga":
		return "ogg"
	case "opus":
		return "opus"
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
	case "mpg":
		return "mpg"
	case "flv":
		return "flv"
	case "wmv":
		return "wmv"
	case "rm", "rmvb":
		return "rmvb"
	case "3gp":
		return "3gp"
	case "m4v":
		return "m4v"
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
