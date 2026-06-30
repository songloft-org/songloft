package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"songloft/pkg/cue"
)

// CueSplitter 使用 ffmpeg 将 CUE 整轨音频切片为独立文件。
type CueSplitter struct {
	ffmpegPath string
	splitDir   string // {data_dir}/cue_splits/
}

// SplitResult 单个 track 的切片结果
type SplitResult struct {
	Track         cue.ResolvedTrack
	SplitFilePath string
	Duration      float64
	FileSize      int64
	Format        string
}

// NewCueSplitter 创建 CUE 切片器
func NewCueSplitter(ffmpegPath, splitDir string) *CueSplitter {
	return &CueSplitter{
		ffmpegPath: ffmpegPath,
		splitDir:   splitDir,
	}
}

// SplitDir 返回切片文件存储目录
func (s *CueSplitter) SplitDir() string {
	return s.splitDir
}

// SplitTracks 对 CUE 的所有 track 执行 ffmpeg 切片。
// cueSourcePath 用于计算切片子目录。
// 单个 track 失败不中断其余 track。
func (s *CueSplitter) SplitTracks(ctx context.Context, cueSourcePath string, tracks []cue.ResolvedTrack) ([]SplitResult, error) {
	if s.ffmpegPath == "" {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	subDir := s.splitSubDir(cueSourcePath)
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return nil, fmt.Errorf("create split dir: %w", err)
	}

	var results []SplitResult
	for _, track := range tracks {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		result, err := s.splitOne(ctx, subDir, track)
		if err != nil {
			slog.Warn("CUE track 切片失败",
				"track", track.TrackNumber,
				"audio", track.AudioFilePath,
				"error", err)
			continue
		}
		results = append(results, *result)
	}

	return results, nil
}

func (s *CueSplitter) splitOne(ctx context.Context, subDir string, track cue.ResolvedTrack) (*SplitResult, error) {
	ext := strings.ToLower(filepath.Ext(track.AudioFilePath))
	format := strings.TrimPrefix(ext, ".")
	codec := "copy"

	// APE 不支持 stream copy，转码为 FLAC
	if format == "ape" {
		format = "flac"
		ext = ".flac"
		codec = "flac"
	}

	outputPath := filepath.Join(subDir, fmt.Sprintf("track_%02d%s", track.TrackNumber, ext))

	args := []string{"-i", track.AudioFilePath}
	args = append(args, "-ss", fmt.Sprintf("%.3f", track.StartSeconds))

	if track.EndSeconds > 0 {
		args = append(args, "-to", fmt.Sprintf("%.3f", track.EndSeconds))
	}

	args = append(args, "-vn", "-codec:a", codec, "-y", outputPath)

	cmd := exec.CommandContext(ctx, s.ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %w, output: %s", err, string(output))
	}

	fi, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("stat split file: %w", err)
	}

	duration := track.Duration()
	if duration <= 0 {
		// EndSeconds 为 0 时（最后一个 track，到文件末尾），用 ffprobe 探测实际时长更准确，
		// 但这里先用 0 占位，后续扫描器会通过 ffprobe 回填。
		duration = 0
	}

	return &SplitResult{
		Track:         track,
		SplitFilePath: outputPath,
		Duration:      duration,
		FileSize:      fi.Size(),
		Format:        format,
	}, nil
}

func (s *CueSplitter) splitSubDir(cueSourcePath string) string {
	h := sha256.Sum256([]byte(cueSourcePath))
	return filepath.Join(s.splitDir, fmt.Sprintf("%x", h[:8]))
}

// RemoveSplitDir 删除某个 CUE 来源的切片文件目录
func (s *CueSplitter) RemoveSplitDir(cueSourcePath string) {
	dir := s.splitSubDir(cueSourcePath)
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("删除 CUE 切片目录失败", "dir", dir, "error", err)
	}
}
