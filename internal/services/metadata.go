package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hanxi/tag"
)

// MetadataConfig 元数据提取配置
type MetadataConfig struct {
	FFProbePath      string // ffprobe 可执行文件路径
	CoverStoragePath string // 封面存储根目录
}

// MetadataExtractor 元数据提取器
type MetadataExtractor struct {
	config *MetadataConfig
}

// Metadata 音频元数据
type Metadata struct {
	Title       string  // 标题
	Artist      string  // 艺术家
	Album       string  // 专辑
	Duration    float64 // 时长（秒）
	Format      string  // 格式
	BitRate     int     // 比特率（kbps）
	SampleRate  int     // 采样率（Hz）
	HasCover    bool    // 是否有封面
	Lyric       string  // 歌词内容
	LyricSource string  // 歌词来源：file/embedded
	CoverPath   string  // 封面文件存储路径（分层目录）
	CoverData   []byte  // 封面图片数据（用于保存）
	CoverExt    string  // 封面图片扩展名
}

// FFProbeOutput ffprobe 输出结构
type FFProbeOutput struct {
	Format  FFProbeFormat   `json:"format"`
	Streams []FFProbeStream `json:"streams"`
}

// FFProbeFormat 格式信息
type FFProbeFormat struct {
	Duration   string            `json:"duration"`
	FormatName string            `json:"format_name"`
	BitRate    string            `json:"bit_rate"`
	Tags       map[string]string `json:"tags"`
}

// FFProbeStream 流信息
type FFProbeStream struct {
	CodecType  string            `json:"codec_type"`
	CodecName  string            `json:"codec_name"`
	SampleRate string            `json:"sample_rate"`
	Tags       map[string]string `json:"tags"`
}

// NewMetadataExtractor 创建新的元数据提取器
func NewMetadataExtractor(config *MetadataConfig) *MetadataExtractor {
	return &MetadataExtractor{
		config: config,
	}
}

// Extract 提取音频文件的元数据
// 优先使用 tag 库提取所有信息（标签、时长、封面等），仅在 tag 库无法获取时长时回退到 ffprobe。
func (m *MetadataExtractor) Extract(ctx context.Context, filePath string) (*Metadata, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	metadata := &Metadata{}

	// 优先使用 tag 库提取元数据
	file, err := os.Open(filePath)
	if err == nil {
		defer file.Close()

		tagMeta, err := tag.ReadFrom(file)
		if err == nil {
			metadata.Title = tagMeta.Title()
			metadata.Artist = tagMeta.Artist()
			metadata.Album = tagMeta.Album()
			metadata.Format = string(tagMeta.Format())

			if picture := tagMeta.Picture(); picture != nil {
				metadata.HasCover = true
				metadata.CoverData = picture.Data
				metadata.CoverExt = picture.Ext
			}

			if lyrics := tagMeta.Lyrics(); lyrics != "" {
				metadata.Lyric = lyrics
				metadata.LyricSource = "embedded"
			}

			// 从 tag 库提取时长
			if duration := tagMeta.Duration(); duration > 0 {
				metadata.Duration = duration.Seconds()
			}
		}
	}

	// 智能合并文件名和刮削标题
	fileName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	slog.Info("Extract title", "fileName", fileName, "title", metadata.Title)

	// 仅在 tag 库未能获取时长时，回退到 ffprobe 补充技术参数
	if metadata.Duration == 0 {
		probeOutput, err := m.runFFProbe(ctx, filePath)
		if err != nil {
			slog.Error("ffprobe failed", "err", err)
			return nil, fmt.Errorf("failed to run ffprobe: %w", err)
		}

		if probeOutput.Format.Duration != "" {
			if duration, err := parseDuration(probeOutput.Format.Duration); err == nil {
				metadata.Duration = duration
			}
		}

		if probeOutput.Format.BitRate != "" {
			if bitRate, err := parseInteger(probeOutput.Format.BitRate); err == nil {
				metadata.BitRate = bitRate / 1000
			}
		}

		for _, stream := range probeOutput.Streams {
			if stream.CodecType == "audio" && stream.SampleRate != "" {
				if sampleRate, err := parseInteger(stream.SampleRate); err == nil {
					metadata.SampleRate = sampleRate
					break
				}
			}
		}

		if metadata.Format == "" && probeOutput.Format.FormatName != "" {
			formats := strings.Split(probeOutput.Format.FormatName, ",")
			if len(formats) > 0 {
				metadata.Format = formats[0]
			}
		}

		// 当 tag 库未能解析出标签时（如 WMA/APE 等 tag 库不原生支持的格式），从 ffprobe 标签兜底补齐
		if tags := mergeFFProbeTags(probeOutput); len(tags) > 0 {
			if metadata.Title == "" {
				metadata.Title = pickTag(tags, "title", "TITLE")
			}
			if metadata.Artist == "" {
				metadata.Artist = pickTag(tags, "artist", "ARTIST", "album_artist", "ALBUM_ARTIST")
			}
			if metadata.Album == "" {
				metadata.Album = pickTag(tags, "album", "ALBUM")
			}
			if metadata.Lyric == "" {
				if lyric := pickTag(tags, "lyrics", "LYRICS", "unsynced_lyrics"); lyric != "" {
					metadata.Lyric = lyric
					metadata.LyricSource = "embedded"
				}
			}
		}
		slog.Info("Extract format", "format", metadata.Format, "bitRate", metadata.BitRate, "sampleRate", metadata.SampleRate, "duration", metadata.Duration)
	}

	// ffprobe 回退可能补齐了 title，需要在标签提取完成后再进行智能合并
	metadata.Title = mergeTitle(fileName, metadata.Title)
	slog.Info("mergeTitle title", "fileName", fileName, "title", metadata.Title, "duration", metadata.Duration)

	// 提取歌词（优先从 .lrc 文件覆盖内嵌歌词）
	lrcFile, err := m.FindLyricFile(filePath)
	if err == nil && lrcFile != "" {
		lyricContent, err := m.ReadLyricFile(lrcFile)
		if err == nil {
			metadata.Lyric = lyricContent
			metadata.LyricSource = "file"
		}
	}

	return metadata, nil
}

// AudioInfo 下载文件校验所需的精简探测结果。
// 与 Metadata 的区别:不读取封面/歌词/标签,只关心格式与时长等技术指标。
//
// 实现了 source.AudioInfoLike 接口(GetDuration / GetSize),
// 可直接传给 source.Validate 做校验,避免跨包数据结构转换。
type AudioInfo struct {
	Duration   float64 // 实测时长(秒)
	Format     string  // mp3 / flac / ...
	BitRate    int     // kbps(可能为 0)
	SampleRate int     // Hz(可能为 0)
	Size       int64   // 文件字节数
}

// GetDuration 实现 source.AudioInfoLike
func (a *AudioInfo) GetDuration() float64 { return a.Duration }

// GetSize 实现 source.AudioInfoLike
func (a *AudioInfo) GetSize() int64 { return a.Size }

// ProbeForValidation 探测下载文件的关键技术指标,供下载完整性校验使用。
//
// 与 Extract 的区别:不读取封面/歌词/标签元数据,只关心格式相关指标。
// 与 ExtractDuration 的区别:一次性返回多个维度(duration/format/bitrate/sample_rate/size),
// 避免下游(SourceFetcher、SourceMetrics)再独立调用一次 ffprobe。
//
// 策略:
//   - tag.ReadFrom 优先:能拿到 duration + format 时就用它(快、无需 ffprobe)
//   - tag 拿不全的字段(bitrate / sample_rate / 或者 tag 解析失败)→ 回退 runFFProbe
//   - os.Stat 拿 size
//
// 若 ffprobe 也失败,返回 error,调用方应将其作为"无法校验"处理(reason=probe_failed)。
func (m *MetadataExtractor) ProbeForValidation(ctx context.Context, filePath string) (*AudioInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	info := &AudioInfo{Size: stat.Size()}

	// 优先用 tag 库(快)
	if file, err := os.Open(filePath); err == nil {
		if tagMeta, err := tag.ReadFrom(file); err == nil {
			info.Format = string(tagMeta.Format())
			if d := tagMeta.Duration(); d > 0 {
				info.Duration = d.Seconds()
			}
			if br := tagMeta.BitRate(); br > 0 {
				info.BitRate = br
			}
			if sr := tagMeta.SampleRate(); sr > 0 {
				info.SampleRate = sr
			}
		}
		file.Close()
	}

	// duration / bitrate / sample_rate 任一缺失就回退 ffprobe
	needProbe := info.Duration == 0 || info.BitRate == 0 || info.SampleRate == 0
	if needProbe {
		probe, err := m.runFFProbe(ctx, filePath)
		if err != nil {
			// tag 已拿到 duration 时容忍 ffprobe 失败
			if info.Duration > 0 {
				return info, nil
			}
			return nil, fmt.Errorf("ffprobe: %w", err)
		}

		if info.Duration == 0 && probe.Format.Duration != "" {
			if d, err := parseDuration(probe.Format.Duration); err == nil {
				info.Duration = d
			}
		}
		if info.BitRate == 0 && probe.Format.BitRate != "" {
			if br, err := parseInteger(probe.Format.BitRate); err == nil {
				info.BitRate = br / 1000
			}
		}
		if info.SampleRate == 0 {
			for _, stream := range probe.Streams {
				if stream.CodecType == "audio" && stream.SampleRate != "" {
					if sr, err := parseInteger(stream.SampleRate); err == nil {
						info.SampleRate = sr
						break
					}
				}
			}
		}
		if info.Format == "" && probe.Format.FormatName != "" {
			formats := strings.Split(probe.Format.FormatName, ",")
			if len(formats) > 0 {
				info.Format = formats[0]
			}
		}
	}

	return info, nil
}

// ExtractDuration 提取音频文件时长（秒）
// 优先使用 tag 库，失败时回退到 ffprobe。
func (m *MetadataExtractor) ExtractDuration(ctx context.Context, filePath string) (float64, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return 0, fmt.Errorf("file does not exist: %s", filePath)
	}

	// 优先使用 tag 库提取时长
	file, err := os.Open(filePath)
	if err == nil {
		defer file.Close()

		tagMeta, err := tag.ReadFrom(file)
		if err == nil {
			if duration := tagMeta.Duration(); duration > 0 {
				return duration.Seconds(), nil
			}
		}
	}

	// tag 库无法获取时长，回退到 ffprobe
	probeOutput, err := m.runFFProbe(ctx, filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to run ffprobe: %w", err)
	}

	if probeOutput.Format.Duration != "" {
		return parseDuration(probeOutput.Format.Duration)
	}

	return 0, nil
}

// runFFProbe 执行 ffprobe 并解析 JSON 输出
func (m *MetadataExtractor) runFFProbe(ctx context.Context, filePath string) (*FFProbeOutput, error) {
	cmd := m.buildFFProbeCommandContext(ctx, filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe execution failed: %w", err)
	}

	var probeOutput FFProbeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	return &probeOutput, nil
}

// SaveCover 保存封面图片到分层目录
// 使用 Metadata 中已提取的封面数据，无需再次解析文件
// 返回封面存储路径（绝对路径），如果没有封面则返回空字符串
func (m *MetadataExtractor) SaveCover(songID int64, metadata *Metadata) (string, error) {
	if !metadata.HasCover || metadata.CoverData == nil {
		return "", nil
	}
	return m.SaveCoverData(metadata.CoverData, metadata.CoverExt)
}

// SaveCoverData 保存任意来源的封面数据到分层目录，按内容哈希自动去重。
// ext 不含点（如 "jpg"），为空时按 generateCoverPath 默认走 jpg。
// data 为空时返回空字符串，不报错。
func (m *MetadataExtractor) SaveCoverData(data []byte, ext string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	coverPath := m.generateCoverPath(data, ext)

	if err := os.MkdirAll(filepath.Dir(coverPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(coverPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write cover image: %w", err)
	}

	return coverPath, nil
}

// generateCoverPath 生成分层目录的封面文件路径
// 使用图片内容的哈希值创建两层子目录，避免单目录文件数过多
// 相同封面图片只会保存一份，实现去重
func (m *MetadataExtractor) generateCoverPath(coverData []byte, ext string) string {
	// 计算图片内容的哈希值
	hash := sha256.Sum256(coverData)
	hashStr := hex.EncodeToString(hash[:])

	// 使用哈希值的前两个字符作为第一层目录
	dir1 := hashStr[0:2]
	// 使用哈希值的第 3-4 个字符作为第二层目录
	dir2 := hashStr[2:4]

	// 确定文件扩展名
	fileExt := ".jpg" // 默认
	if ext != "" {
		fileExt = "." + strings.ToLower(ext)
	}

	// 构建完整路径：/app/data/covers/{hash2}/{hash4}/{content_hash}.{ext}
	// 使用完整的内容哈希作为文件名，相同封面自动去重
	filename := fmt.Sprintf("%s%s", hashStr, fileExt)
	return filepath.Join(m.config.CoverStoragePath, dir1, dir2, filename)
}

// FindLyricFile 查找对应的歌词文件
func (m *MetadataExtractor) FindLyricFile(audioFilePath string) (string, error) {
	// 构建 .lrc 文件路径
	ext := filepath.Ext(audioFilePath)
	lrcPath := strings.TrimSuffix(audioFilePath, ext) + ".lrc"

	// 检查文件是否存在
	if _, err := os.Stat(lrcPath); err == nil {
		return lrcPath, nil
	}

	return "", nil
}

// ReadLyricFile 读取歌词文件内容
func (m *MetadataExtractor) ReadLyricFile(lrcPath string) (string, error) {
	content, err := os.ReadFile(lrcPath)
	if err != nil {
		return "", fmt.Errorf("failed to read lyric file: %w", err)
	}

	return string(content), nil
}

// IsFFProbeAvailable 检查 ffprobe 是否可用
func (m *MetadataExtractor) IsFFProbeAvailable() bool {
	cmd := exec.Command(m.config.FFProbePath, "-version")
	return cmd.Run() == nil
}

// buildFFProbeCommandContext 构建带上下文的 ffprobe 命令
func (m *MetadataExtractor) buildFFProbeCommandContext(ctx context.Context, filePath string) *exec.Cmd {
	return exec.CommandContext(
		ctx,
		m.config.FFProbePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)
}

// parseDuration 解析时长字符串
func parseDuration(duration string) (float64, error) {
	if duration == "" {
		return 0, fmt.Errorf("empty duration")
	}

	value, err := strconv.ParseFloat(duration, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format: %w", err)
	}

	return value, nil
}

// parseInteger 解析整数字符串
func parseInteger(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("empty value")
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer format: %w", err)
	}

	return intValue, nil
}

// mergeTitle 决定最终标题:tag 里有标题就用 tag 的,否则用文件名
//
// 历史版本会做"最长公共子串去重 + 拼接",但实测会出现:
//   - "周杰伦 - 晴天.mp3" + tag.Title="晴天"  → "周杰伦 - 晴天"(可以接受,但 tag 信息已经在 Artist 里)
//   - "01.song.mp3"   + tag.Title="Song Name" → "01 - Song Name"(意义不大,反而把前缀带入标题)
//   - 当文件名与 tag 标题没有公共子串时,强行拼成 "文件名 - tag 标题",制造冗余
//
// 大多数情况下,tag 里的标题已经是最准确的;只有 tag 缺失时才退而求其次用文件名。
func mergeTitle(fileName, scrapedTitle string) string {
	if scrapedTitle != "" {
		return scrapedTitle
	}
	return fileName
}

// mergeFFProbeTags 合并 ffprobe 输出中 format.tags 与音频流 tags
// format.tags 优先级更高；仅当 format.tags 未提供某个 key 时才从音频流 tags 补充
func mergeFFProbeTags(probe *FFProbeOutput) map[string]string {
	if probe == nil {
		return nil
	}
	merged := make(map[string]string)
	for k, v := range probe.Format.Tags {
		if v != "" {
			merged[k] = v
		}
	}
	for _, stream := range probe.Streams {
		if stream.CodecType != "audio" {
			continue
		}
		for k, v := range stream.Tags {
			if v == "" {
				continue
			}
			if _, ok := merged[k]; !ok {
				merged[k] = v
			}
		}
	}
	return merged
}

// pickTag 按候选 key 顺序从标签 map 中查找首个非空值
// tags map 的键大小写由 ffprobe 决定（常见为小写），同时容忍大写变体
func pickTag(tags map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := tags[k]; ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
