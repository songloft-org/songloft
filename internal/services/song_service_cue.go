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

// processCueFiles 处理外部 .cue 文件：解析、入库（按需提取替代预分割）。
func (s *SongService) processCueFiles(ctx context.Context, cueFiles []string, reimport bool) {
	if len(cueFiles) == 0 {
		return
	}

	existingSources := s.listCueSourceSet(ctx)

	for _, cuePath := range cueFiles {
		select {
		case <-ctx.Done():
			return
		default:
		}

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
		select {
		case <-ctx.Done():
			return
		default:
		}

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

// processCueSheet 处理一个 CUE Sheet：解析 track 起止时间并入库。
// 不再预分割音频文件；播放时由 CacheService 按需提取。
func (s *SongService) processCueSheet(ctx context.Context, cueSourcePath string, sheet *cue.CUESheet, reimport bool) {
	s.scanProgressManager.UpdateCueProgress(cueSourcePath)

	cueDir := filepath.Dir(cueSourcePath)

	// 获取引用音频文件的时长。同时缓存整轨文件的元数据，
	// 供后续提取封面复用，避免对大 CD 镜像重复 Extract（重复读取整个文件）。
	totalDurations := make(map[string]float64)
	metaCache := make(map[string]*Metadata)
	for _, f := range sheet.Files {
		audioPath := f.Filename
		if !filepath.IsAbs(audioPath) {
			audioPath = filepath.Join(cueDir, audioPath)
		}
		metadata, err := s.metadataExtractor.Extract(ctx, audioPath)
		if err == nil {
			metaCache[audioPath] = metadata
			if metadata.Duration > 0 {
				totalDurations[audioPath] = metadata.Duration
			}
		}
	}

	// 解析 tracks
	tracks, err := cue.ResolveTracks(sheet, cueDir, totalDurations)
	if err != nil || len(tracks) == 0 {
		slog.Warn("CUE track 解析失败", "source", cueSourcePath, "error", err)
		return
	}

	// 重新导入时先删除旧记录
	if reimport {
		if n, err := s.songs.DeleteByCueSource(ctx, cueSourcePath); err == nil && n > 0 {
			slog.Info("重新导入 CUE，删除旧记录", "source", cueSourcePath, "deleted", n)
		}
	}

	// 提取封面（从第一个整轨文件提取一次）；优先复用已缓存的元数据。
	firstAudioPath := tracks[0].AudioFilePath
	var coverPath string
	if md, ok := metaCache[firstAudioPath]; ok {
		coverPath = s.saveCueCover(firstAudioPath, md)
	} else {
		coverPath = s.extractCueCover(ctx, firstAudioPath)
	}

	// 入库：直接引用原始整轨音频，记录 track 起止时间
	now := time.Now()
	songs := make([]*models.Song, 0, len(tracks))
	for _, track := range tracks {
		ext := strings.ToLower(filepath.Ext(track.AudioFilePath))
		format := NormalizeFormat(ext)

		duration := track.Duration()

		var bitRate, sampleRate int
		if md, ok := metaCache[track.AudioFilePath]; ok {
			bitRate = md.BitRate
			sampleRate = md.SampleRate
		}

		song := &models.Song{
			Type:            models.TypeLocal,
			Title:           track.Title,
			Artist:          track.Performer,
			Album:           track.Album,
			Genre:           track.Genre,
			Duration:        duration,
			FilePath:        track.AudioFilePath,
			Format:          format,
			BitRate:         bitRate,
			SampleRate:      sampleRate,
			CoverPath:       coverPath,
			ISRC:            track.ISRC,
			CueSourcePath:   cueSourcePath,
			CueTrackIndex:   track.TrackNumber,
			CueAudioPath:    track.AudioFilePath,
			CueStartSeconds: track.StartSeconds,
			CueEndSeconds:   track.EndSeconds,
			AddedAt:         now,
			UpdatedAt:       now,
		}
		if track.Year != "" {
			song.Year, _ = strconv.Atoi(track.Year)
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

// extractCueCover 从整轨音频文件提取并保存封面（无缓存时的回退路径）。
func (s *SongService) extractCueCover(ctx context.Context, audioPath string) string {
	metadata, err := s.metadataExtractor.Extract(ctx, audioPath)
	if err != nil {
		return ""
	}
	return s.saveCueCover(audioPath, metadata)
}

// saveCueCover 从已提取的元数据保存封面，返回封面路径（无封面时返回空串）。
func (s *SongService) saveCueCover(audioPath string, metadata *Metadata) string {
	if metadata == nil || !metadata.HasCover || metadata.CoverData == nil {
		return ""
	}
	coverPath, err := s.metadataExtractor.SaveCover(0, metadata)
	if err != nil {
		slog.Warn("CUE 封面保存失败", "audio", audioPath, "error", err)
		return ""
	}
	return coverPath
}

// cleanStaleCueRecords 清理已删除 CUE 源文件对应的 DB 记录。
func (s *SongService) cleanStaleCueRecords(ctx context.Context, scopeRoots []string) {
	sources := s.listCueSourceSet(ctx)

	for sourcePath := range sources {
		// 定向扫描：只清理落在本次作用域内的 CUE 源，作用域外一律不动（Issue #262）。
		if !isUnderScope(sourcePath, scopeRoots) {
			continue
		}

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
