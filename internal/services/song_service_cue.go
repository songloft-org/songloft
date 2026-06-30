package services

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hanxi/tag"

	"songloft/internal/database"
	"songloft/internal/models"
	"songloft/pkg/cue"
)

// processCueFiles 处理外部 .cue 文件：解析、切片、入库。
func (s *SongService) processCueFiles(ctx context.Context, cueFiles []string, reimport bool) {
	if len(cueFiles) == 0 {
		return
	}

	existingSources := s.listCueSourceSet(ctx)

	for _, cuePath := range cueFiles {
		if !reimport {
			if existingSources[cuePath] {
				continue
			}
		}

		sheet, err := cue.ParseFile(cuePath)
		if err != nil {
			slog.Warn("CUE 解析失败", "path", cuePath, "error", err)
			continue
		}

		s.processCueSheet(ctx, cuePath, sheet, reimport)
	}
}

// processEmbeddedCueSheets 检测 FLAC 文件中的内嵌 CUESHEET block。
func (s *SongService) processEmbeddedCueSheets(ctx context.Context, audioFiles []string, reimport bool) {
	existingSources := s.listCueSourceSet(ctx)

	for _, filePath := range audioFiles {
		if !strings.HasSuffix(strings.ToLower(filePath), ".flac") {
			continue
		}

		// 外部 .cue 优先：如果同名 .cue 文件存在，跳过内嵌
		baseName := strings.TrimSuffix(filePath, filepath.Ext(filePath))
		hasCueFile := false
		for _, ext := range []string{".cue", ".CUE", ".Cue"} {
			if _, err := os.Stat(baseName + ext); err == nil {
				hasCueFile = true
				break
			}
		}
		if hasCueFile {
			continue
		}

		// 增量跳过
		if !reimport {
			if existingSources[filePath] {
				continue
			}
		}

		f, err := os.Open(filePath)
		if err != nil {
			continue
		}
		tagMeta, err := tag.ReadFrom(f)
		f.Close()
		if err != nil {
			continue
		}

		provider, ok := tagMeta.(tag.CUESheetProvider)
		if !ok {
			continue
		}
		cueData := provider.CUESheetBlock()
		if cueData == nil || len(cueData.Tracks) == 0 {
			continue
		}

		sampleRate := tagMeta.SampleRate()
		if sampleRate <= 0 {
			continue
		}

		sheet := flacCueDataToSheet(cueData, sampleRate, filePath)
		if sheet == nil {
			continue
		}

		// 对于内嵌 CUESHEET，cueSourcePath 是 FLAC 文件本身
		s.processCueSheet(ctx, filePath, sheet, reimport)
	}
}

// flacCueDataToSheet 将 pkg/tag 的 CUESheetData 转换为 pkg/cue 的 CUESheet
func flacCueDataToSheet(data *tag.CUESheetData, sampleRate int, flacPath string) *cue.CUESheet {
	sheet := &cue.CUESheet{}
	cueFile := cue.CUEFile{
		Filename: filepath.Base(flacPath),
		FileType: "WAVE",
	}

	for _, t := range data.Tracks {
		startSeconds := float64(t.OffsetSample) / float64(sampleRate)
		ct := secondsToCueTime(startSeconds)
		track := cue.CUETrack{
			Number: t.Number,
			ISRC:   t.ISRC,
			Start:  ct,
		}
		cueFile.Tracks = append(cueFile.Tracks, track)
	}

	if len(cueFile.Tracks) == 0 {
		return nil
	}

	sheet.Files = append(sheet.Files, cueFile)
	return sheet
}

func secondsToCueTime(seconds float64) cue.CUETime {
	totalFrames := int(seconds * 75)
	m := totalFrames / (75 * 60)
	totalFrames -= m * 75 * 60
	sec := totalFrames / 75
	f := totalFrames % 75
	return cue.CUETime{Minutes: m, Seconds: sec, Frames: f}
}

// processCueSheet 处理一个 CUE Sheet 的切片和入库。
func (s *SongService) processCueSheet(ctx context.Context, cueSourcePath string, sheet *cue.CUESheet, reimport bool) {
	cueDir := filepath.Dir(cueSourcePath)

	// 获取引用音频文件的时长
	totalDurations := make(map[string]float64)
	for _, f := range sheet.Files {
		audioPath := f.Filename
		if !filepath.IsAbs(audioPath) {
			audioPath = filepath.Join(cueDir, audioPath)
		}
		metadata, err := s.metadataExtractor.Extract(ctx, audioPath)
		if err == nil && metadata.Duration > 0 {
			totalDurations[audioPath] = metadata.Duration
		}
	}

	// 解析 tracks
	tracks, err := cue.ResolveTracks(sheet, cueDir, totalDurations)
	if err != nil || len(tracks) == 0 {
		slog.Warn("CUE track 解析失败", "source", cueSourcePath, "error", err)
		return
	}

	// 重新导入时先删除旧记录和切片
	if reimport {
		if n, err := s.songs.DeleteByCueSource(ctx, cueSourcePath); err == nil && n > 0 {
			slog.Info("重新导入 CUE，删除旧记录", "source", cueSourcePath, "deleted", n)
		}
		s.cueSplitter.RemoveSplitDir(cueSourcePath)
	}

	// ffmpeg 切片
	results, err := s.cueSplitter.SplitTracks(ctx, cueSourcePath, tracks)
	if err != nil {
		slog.Warn("CUE 切片失败", "source", cueSourcePath, "error", err)
		return
	}
	if len(results) == 0 {
		slog.Warn("CUE 切片无结果", "source", cueSourcePath)
		return
	}

	// 提取封面（从第一个整轨文件提取一次）
	coverPath := s.extractCueCover(ctx, results[0].Track.AudioFilePath)

	// 入库
	now := time.Now()
	songs := make([]*models.Song, 0, len(results))
	for _, r := range results {
		song := &models.Song{
			Type:          models.TypeLocal,
			Title:         r.Track.Title,
			Artist:        r.Track.Performer,
			Album:         r.Track.Album,
			Genre:         r.Track.Genre,
			Duration:      r.Duration,
			FilePath:      r.SplitFilePath,
			Format:        r.Format,
			FileSize:      r.FileSize,
			CoverPath:     coverPath,
			ISRC:          r.Track.ISRC,
			CueSourcePath: cueSourcePath,
			CueTrackIndex: r.Track.TrackNumber,
			CueAudioPath:  r.Track.AudioFilePath,
			AddedAt:       now,
			UpdatedAt:     now,
		}
		if r.Track.Year != "" {
			song.Year, _ = strconv.Atoi(r.Track.Year)
		}
		songs = append(songs, song)
	}

	// 批量入库
	err = s.tx.RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
		for _, song := range songs {
			if err := uow.Songs.Create(ctx, song); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Warn("CUE 歌曲入库失败", "source", cueSourcePath, "error", err)
		return
	}

	slog.Info("CUE 整轨导入完成",
		"source", cueSourcePath,
		"album", sheet.Title,
		"tracks", len(songs))
}

// extractCueCover 从整轨音频文件提取封面
func (s *SongService) extractCueCover(ctx context.Context, audioPath string) string {
	metadata, err := s.metadataExtractor.Extract(ctx, audioPath)
	if err != nil || !metadata.HasCover || metadata.CoverData == nil {
		return ""
	}

	coverPath, err := s.metadataExtractor.SaveCover(0, metadata)
	if err != nil {
		slog.Warn("CUE 封面保存失败", "audio", audioPath, "error", err)
		return ""
	}
	return coverPath
}

// cleanStaleCueRecords 清理已删除 CUE 源文件对应的切片和 DB 记录。
func (s *SongService) cleanStaleCueRecords(ctx context.Context) {
	sources := s.listCueSourceSet(ctx)

	for sourcePath := range sources {
		shouldClean := false

		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			shouldClean = true
		}

		// 检查该 CUE 来源引用的所有音频文件是否存在
		if !shouldClean {
			audioPaths, err := s.songs.ListCueAudioPaths(ctx, sourcePath)
			if err == nil {
				for _, audioPath := range audioPaths {
					if _, err := os.Stat(audioPath); os.IsNotExist(err) {
						shouldClean = true
						break
					}
				}
			}
		}

		if shouldClean {
			deleted, _ := s.songs.DeleteByCueSource(ctx, sourcePath)
			s.cueSplitter.RemoveSplitDir(sourcePath)
			slog.Info("清理已删除 CUE 源", "source", sourcePath, "deleted", deleted)
		}
	}
}

func (s *SongService) listCueSourceSet(ctx context.Context) map[string]bool {
	sources, err := s.songs.ListCueSources(ctx)
	if err != nil {
		slog.Warn("获取已有 CUE 来源失败", "error", err)
		return make(map[string]bool)
	}
	return sources
}
