package services

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hanxi/tag"
)

// TestNewMetadataExtractor 测试创建元数据提取器
func TestNewMetadataExtractor(t *testing.T) {
	config := &MetadataConfig{
		FFProbePath: "ffprobe",
	}

	extractor := NewMetadataExtractor(config)
	if extractor == nil {
		t.Error("NewMetadataExtractor() returned nil")
	}
}

// TestParseDuration 测试解析时长
func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		want     float64
		wantErr  bool
	}{
		{"valid duration", "180.5", 180.5, false},
		{"integer duration", "120", 120.0, false},
		{"zero duration", "0", 0.0, false},
		{"invalid duration", "invalid", 0.0, true},
		{"empty duration", "", 0.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.duration)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseInteger 测试解析整数
func TestParseInteger(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{"valid integer", "320", 320, false},
		{"zero", "0", 0, false},
		{"negative", "-1", -1, false},
		{"invalid", "abc", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInteger(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInteger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseInteger() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFindLyricFile 测试查找歌词文件
func TestFindLyricFile(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "lyric_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	audioFile := filepath.Join(tmpDir, "song.mp3")
	lrcFile := filepath.Join(tmpDir, "song.lrc")

	if err := os.WriteFile(audioFile, []byte("audio"), 0644); err != nil {
		t.Fatalf("failed to create audio file: %v", err)
	}
	if err := os.WriteFile(lrcFile, []byte("[00:00.00]Test lyric"), 0644); err != nil {
		t.Fatalf("failed to create lrc file: %v", err)
	}

	config := &MetadataConfig{
		FFProbePath: "ffprobe",
	}
	extractor := NewMetadataExtractor(config)

	// 测试找到歌词文件
	found, err := extractor.FindLyricFile(audioFile)
	if err != nil {
		t.Errorf("FindLyricFile() error = %v", err)
	}
	if found != lrcFile {
		t.Errorf("FindLyricFile() = %v, want %v", found, lrcFile)
	}

	// 测试找不到歌词文件
	audioFile2 := filepath.Join(tmpDir, "song2.mp3")
	if err := os.WriteFile(audioFile2, []byte("audio"), 0644); err != nil {
		t.Fatalf("failed to create audio file: %v", err)
	}

	found2, err := extractor.FindLyricFile(audioFile2)
	if err != nil {
		t.Errorf("FindLyricFile() error = %v", err)
	}
	if found2 != "" {
		t.Errorf("FindLyricFile() = %v, want empty string", found2)
	}
}

// TestReadLyricFile 测试读取歌词文件
func TestReadLyricFile(t *testing.T) {
	// 创建临时歌词文件
	tmpDir, err := os.MkdirTemp("", "lyric_read_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lrcFile := filepath.Join(tmpDir, "test.lrc")
	lrcContent := "[00:00.00]Line 1\n[00:05.00]Line 2\n"

	if err := os.WriteFile(lrcFile, []byte(lrcContent), 0644); err != nil {
		t.Fatalf("failed to create lrc file: %v", err)
	}

	config := &MetadataConfig{
		FFProbePath: "ffprobe",
	}
	extractor := NewMetadataExtractor(config)

	content, err := extractor.ReadLyricFile(lrcFile)
	if err != nil {
		t.Errorf("ReadLyricFile() error = %v", err)
	}
	if content != lrcContent {
		t.Errorf("ReadLyricFile() = %v, want %v", content, lrcContent)
	}

	// 测试读取不存在的文件
	_, err = extractor.ReadLyricFile("/non/existent/file.lrc")
	if err == nil {
		t.Error("ReadLyricFile() should return error for non-existent file")
	}
}

// TestBuildFFProbeCommandContext 测试构建 ffprobe 命令
func TestBuildFFProbeCommandContext(t *testing.T) {
	config := &MetadataConfig{
		FFProbePath: "ffprobe",
	}
	extractor := NewMetadataExtractor(config)

	filePath := "/music/test.mp3"
	ctx := context.Background()
	cmd := extractor.buildFFProbeCommandContext(ctx, filePath)

	if cmd == nil {
		t.Fatal("buildFFProbeCommandContext() returned nil")
	}

	// 验证命令参数包含文件路径
	found := false
	for _, arg := range cmd.Args {
		if arg == filePath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("command args should contain file path %v, got %v", filePath, cmd.Args)
	}
}

// TestMetadataExtraction 测试元数据提取（集成测试，需要 ffprobe）
func TestMetadataExtraction(t *testing.T) {
	// 检查 ffprobe 是否可用
	config := &MetadataConfig{
		FFProbePath: "ffprobe",
	}
	extractor := NewMetadataExtractor(config)

	// 这是一个集成测试，需要真实的音频文件和 ffprobe
	// 在 CI 环境中可能会跳过
	if !extractor.IsFFProbeAvailable() {
		t.Skip("ffprobe not available, skipping integration test")
	}

	// 如果有真实的测试音频文件，可以在这里测试
	// 这里只是一个框架示例
	t.Log("ffprobe is available for integration testing")
}

// TestIsFFProbeAvailable 测试 ffprobe 可用性检查
func TestIsFFProbeAvailable(t *testing.T) {
	tests := []struct {
		name        string
		ffprobePath string
		wantErr     bool
	}{
		{"valid ffprobe", "ffprobe", false},
		{"invalid path", "/non/existent/ffprobe", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &MetadataConfig{
				FFProbePath: tt.ffprobePath,
			}
			extractor := NewMetadataExtractor(config)
			available := extractor.IsFFProbeAvailable()

			if tt.wantErr && available {
				t.Error("IsFFProbeAvailable() should return false for invalid path")
			}
		})
	}
}

// TestExtractWithInvalidFile 测试提取无效文件的元数据
func TestExtractWithInvalidFile(t *testing.T) {
	config := &MetadataConfig{
		FFProbePath: "ffprobe",
	}
	extractor := NewMetadataExtractor(config)

	ctx := context.Background()
	_, err := extractor.Extract(ctx, "/non/existent/file.mp3")
	if err == nil {
		t.Error("Extract() should return error for non-existent file")
	}
}

// TestFindLyricFileWithDifferentAudioFormats 测试不同音频格式的歌词文件查找
func TestFindLyricFileWithDifferentAudioFormats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lyric_format_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		audioFile string
		lrcFile   string
	}{
		{"song.mp3", "song.lrc"},
		{"song.flac", "song.lrc"},
		{"song.m4a", "song.lrc"},
	}

	config := &MetadataConfig{
		FFProbePath: "ffprobe",
	}
	extractor := NewMetadataExtractor(config)

	for _, tt := range tests {
		t.Run(tt.audioFile, func(t *testing.T) {
			audioPath := filepath.Join(tmpDir, tt.audioFile)
			lrcPath := filepath.Join(tmpDir, tt.lrcFile)

			// 创建文件
			if err := os.WriteFile(audioPath, []byte("audio"), 0644); err != nil {
				t.Fatalf("failed to create audio file: %v", err)
			}
			if err := os.WriteFile(lrcPath, []byte("[00:00.00]Test"), 0644); err != nil {
				t.Fatalf("failed to create lrc file: %v", err)
			}

			// 查找歌词文件
			found, err := extractor.FindLyricFile(audioPath)
			if err != nil {
				t.Errorf("FindLyricFile() error = %v", err)
			}
			if found != lrcPath {
				t.Errorf("FindLyricFile() = %v, want %v", found, lrcPath)
			}

			// 清理
			os.Remove(audioPath)
			os.Remove(lrcPath)
		})
	}
}

// TestProbeForValidationVsFFProbe 把 tag 库提取的 duration/bitrate/sample_rate
// 与 ffprobe 直接结果对比,验证 tag 库实现的准确性。
//
// 样本文件来自 pkg/tag/testdata,覆盖 MP3(ID3v1/v2.2/v2.3/v2.4/VBR)、FLAC、OGG、MP4、DSF。
// 容忍度:
//   - SampleRate: 必须完全一致(0 表示 tag 库无法提供,跳过对比)
//   - BitRate: 允许 ±15% 或 ±5kbps,VBR 估算与 ffprobe 平均值有差异
//   - Duration: 允许 ±1s,采样数取整可能差 0.5s
func TestProbeForValidationVsFFProbe(t *testing.T) {
	config := &MetadataConfig{FFProbePath: "ffprobe"}
	extractor := NewMetadataExtractor(config)

	if !extractor.IsFFProbeAvailable() {
		t.Skip("ffprobe not available, skipping integration test")
	}

	testdataDir := filepath.Join("..", "..", "pkg", "tag", "testdata", "with_tags")
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Skipf("testdata dir not found: %v", err)
	}

	ctx := context.Background()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext == "" || ext == ".md" {
			continue
		}

		t.Run(name, func(t *testing.T) {
			filePath := filepath.Join(testdataDir, name)

			// 1) tag 库 + ffprobe 混合(ProbeForValidation 内部已经在缺失字段时回退 ffprobe)
			info, err := extractor.ProbeForValidation(ctx, filePath)
			if err != nil {
				t.Fatalf("ProbeForValidation() error: %v", err)
			}

			// 2) 纯 ffprobe 基线
			probe, err := extractor.runFFProbe(ctx, filePath)
			if err != nil {
				t.Fatalf("runFFProbe() error: %v", err)
			}

			var ffDuration float64
			if probe.Format.Duration != "" {
				ffDuration, _ = parseDuration(probe.Format.Duration)
			}
			var ffBitRate int
			if probe.Format.BitRate != "" {
				if br, err := parseInteger(probe.Format.BitRate); err == nil {
					ffBitRate = br / 1000
				}
			}
			var ffSampleRate int
			for _, stream := range probe.Streams {
				if stream.CodecType == "audio" && stream.SampleRate != "" {
					if sr, err := parseInteger(stream.SampleRate); err == nil {
						ffSampleRate = sr
						break
					}
				}
			}

			t.Logf("file=%s\n  ProbeForValidation: duration=%.3fs bitrate=%dkbps sampleRate=%dHz format=%s\n  ffprobe baseline:   duration=%.3fs bitrate=%dkbps sampleRate=%dHz",
				name,
				info.Duration, info.BitRate, info.SampleRate, info.Format,
				ffDuration, ffBitRate, ffSampleRate,
			)

			// Duration:允许 ±1s 偏差
			if ffDuration > 0 && math.Abs(info.Duration-ffDuration) > 1.0 {
				t.Errorf("duration mismatch: got %.3fs, ffprobe %.3fs (delta=%.3fs)",
					info.Duration, ffDuration, info.Duration-ffDuration)
			}

			// SampleRate:必须完全一致。
			// DSF 例外:tag 库暴露的是 DSD raw 频率(2822400Hz 等),ffprobe 5.x 报 PCM-equivalent(raw/8)。
			// 允许 ratio 为 1x 或 8x。
			if ffSampleRate > 0 && info.SampleRate != ffSampleRate {
				if ext == ".dsf" && info.SampleRate == ffSampleRate*8 {
					t.Logf("DSF sample rate: tag=%dHz (DSD raw) vs ffprobe=%dHz (PCM-equivalent) — accepted",
						info.SampleRate, ffSampleRate)
				} else {
					t.Errorf("sample rate mismatch: got %dHz, ffprobe %dHz", info.SampleRate, ffSampleRate)
				}
			}

			// BitRate:容忍 ±15% 或 ±5kbps。
			// OGG 例外:tag 暴露的是 vorbis_id 的 nominal bitrate,与 ffprobe 的实测平均值可能大差异(尤其 VBR/multipage),放宽到 ±80%。
			tolerance := 0.15
			if ext == ".ogg" {
				tolerance = 0.80
			}
			if ffBitRate > 0 && info.BitRate > 0 {
				delta := math.Abs(float64(info.BitRate - ffBitRate))
				if delta > 5 && delta/float64(ffBitRate) > tolerance {
					t.Errorf("bitrate mismatch: got %dkbps, ffprobe %dkbps (delta=%.0fkbps, %.1f%%)",
						info.BitRate, ffBitRate, delta, delta/float64(ffBitRate)*100)
				}
			}
		})
	}
}

// TestTagLibCoverageVsFFProbe 单独验证 tag 库本身(不走 ffprobe 回退)对各格式的提取覆盖度。
// 这个测试不要求 tag 库覆盖所有字段(MP4/ID3v1 wrapper 不提供 BitRate/SampleRate 是已知情况),
// 而是把"tag 库能拿到什么"明确记录下来,便于后续回归发现退化。
func TestTagLibCoverageVsFFProbe(t *testing.T) {
	config := &MetadataConfig{FFProbePath: "ffprobe"}
	extractor := NewMetadataExtractor(config)

	if !extractor.IsFFProbeAvailable() {
		t.Skip("ffprobe not available, skipping integration test")
	}

	testdataDir := filepath.Join("..", "..", "pkg", "tag", "testdata", "with_tags")
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Skipf("testdata dir not found: %v", err)
	}

	ctx := context.Background()

	// 期望 tag 库直接(不走 ffprobe)能提供 SampleRate / BitRate 的格式
	// 注:m4a/mp4 暂未解析 stsd box,ogg multipage 可能 bitrate 不准
	expectTagSampleRate := map[string]bool{
		".mp3":  true,
		".flac": true,
		".ogg":  true,
		".dsf":  true,
	}
	expectTagBitRate := map[string]bool{
		".mp3": true,
		".dsf": true,
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext == "" || ext == ".md" {
			continue
		}

		t.Run(name, func(t *testing.T) {
			filePath := filepath.Join(testdataDir, name)

			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("open file: %v", err)
			}
			defer f.Close()

			tagMeta, err := tag.ReadFrom(f)
			if err != nil {
				t.Fatalf("tag.ReadFrom: %v", err)
			}

			// 同时跑一次 ffprobe 作为参考
			probe, err := extractor.runFFProbe(ctx, filePath)
			if err != nil {
				t.Fatalf("runFFProbe: %v", err)
			}
			var ffSampleRate int
			for _, stream := range probe.Streams {
				if stream.CodecType == "audio" && stream.SampleRate != "" {
					if sr, err := parseInteger(stream.SampleRate); err == nil {
						ffSampleRate = sr
						break
					}
				}
			}
			var ffBitRate int
			if probe.Format.BitRate != "" {
				if br, err := parseInteger(probe.Format.BitRate); err == nil {
					ffBitRate = br / 1000
				}
			}

			tagSR := tagMeta.SampleRate()
			tagBR := tagMeta.BitRate()

			t.Logf("%s: tag(SR=%d BR=%d)  ffprobe(SR=%d BR=%d)",
				name, tagSR, tagBR, ffSampleRate, ffBitRate)

			if expectTagSampleRate[ext] {
				if tagSR == 0 {
					t.Errorf("tag lib should provide SampleRate for %s, got 0 (ffprobe=%d)", ext, ffSampleRate)
				}
				// DSF:tag 暴露的是 DSD raw 频率,ffprobe 报 PCM-equivalent(raw/8)
				if tagSR != 0 && ffSampleRate > 0 && tagSR != ffSampleRate {
					if ext == ".dsf" && tagSR == ffSampleRate*8 {
						t.Logf("DSF SampleRate: tag=%d (DSD raw) vs ffprobe=%d (PCM-equivalent) — accepted",
							tagSR, ffSampleRate)
					} else {
						t.Errorf("tag SampleRate=%d disagrees with ffprobe=%d", tagSR, ffSampleRate)
					}
				}
			}

			if expectTagBitRate[ext] {
				if tagBR == 0 {
					t.Errorf("tag lib should provide BitRate for %s, got 0 (ffprobe=%d)", ext, ffBitRate)
				}
				if tagBR != 0 && ffBitRate > 0 {
					delta := math.Abs(float64(tagBR - ffBitRate))
					if delta > 5 && delta/float64(ffBitRate) > 0.15 {
						t.Errorf("tag BitRate=%dkbps disagrees with ffprobe=%dkbps (delta=%.0fkbps)",
							tagBR, ffBitRate, delta)
					}
				}
			}
		})
	}
}

// TestReadLyricFileWithLargeFile 测试读取大歌词文件
func TestReadLyricFileWithLargeFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "large_lyric_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lrcFile := filepath.Join(tmpDir, "large.lrc")

	// 创建一个较大的歌词文件
	var content string
	for i := 0; i < 1000; i++ {
		content += fmt.Sprintf("[%02d:%02d.00]Line %d\n", i/60, i%60, i)
	}

	if err := os.WriteFile(lrcFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create lrc file: %v", err)
	}

	config := &MetadataConfig{
		FFProbePath: "ffprobe",
	}
	extractor := NewMetadataExtractor(config)

	readContent, err := extractor.ReadLyricFile(lrcFile)
	if err != nil {
		t.Errorf("ReadLyricFile() error = %v", err)
	}
	if readContent != content {
		t.Error("ReadLyricFile() content mismatch")
	}
}
