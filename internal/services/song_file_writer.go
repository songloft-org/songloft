package services

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"songloft/internal/models"

	"github.com/hanxi/tag"
)

func (s *SongService) writeTrackedSongTags(ctx context.Context, song *models.Song, fields []string) FileWriteStatus {
	status := writeEditedSongTags(song, fields)
	return s.trackSongTagWrite(ctx, song, status)
}

// SyncSongTagsToFile writes the current custom label list without changing other metadata.
func (s *SongService) SyncSongTagsToFile(ctx context.Context, id int64) {
	tags := s.songTagService.Load()
	if tags == nil {
		return
	}
	lock := &s.metadataEditLocks[uint64(id)%uint64(len(s.metadataEditLocks))]
	lock.Lock()
	defer lock.Unlock()
	song, err := s.songs.GetByID(ctx, id)
	if err != nil || !SupportsMetadataOverrides(song) {
		return
	}
	bound, err := tags.GetSongTags(ctx, id)
	if err != nil {
		return
	}
	names := make([]string, 0, len(bound))
	for _, label := range bound {
		names = append(names, label.Name)
	}
	opts := tag.WriteOptions{CustomTags: map[string]string{"SONGLOFT_TAGS": strings.Join(names, ",")}}
	s.trackSongTagWrite(ctx, song, writeIncrementalSongTags(song.FilePath, opts))
}

func (s *SongService) trackSongTagWrite(ctx context.Context, song *models.Song, status FileWriteStatus) FileWriteStatus {
	if status == FileWriteWritten {
		if info, err := os.Stat(song.FilePath); err == nil {
			if err := s.songs.UpdateFileObservation(context.WithoutCancel(ctx), song.ID, song.FilePath, info.Size(), info.ModTime()); err != nil {
				slog.Warn("update song file observation", "songId", song.ID, "error", err)
			}
		}
	}
	return status
}

// FileWriteStatus 表示把 song 元数据回写到音频文件后的结果。
//
// 设计目的：UpdateLyrics 这种「DB 必须成功，文件失败可降级」的接口
// 不能用 error 表达「DB 已写入，但文件没改」这种半成功语义；返回 status
// 让调用方决定是否在响应里告知客户端，而不是 5xx。
type FileWriteStatus string

const (
	// FileWriteWritten 文件回写成功。
	FileWriteWritten FileWriteStatus = "written"
	// FileWriteUnchanged 因前置条件不满足而未尝试回写（文件未变更）：
	//   - 不是本地歌曲 / file_path 为空
	//   - lyric_source = url（运行时拉取，不缓存到本地文件）
	//   - 文件扩展名不在 pkg/tag 支持的写入清单（pkg/tag 返回 ErrUnsupportedWrite）
	FileWriteUnchanged FileWriteStatus = "unchanged"
	// FileWriteFailed 实际写入时报错（IO / 解析失败等）。
	FileWriteFailed FileWriteStatus = "failed"
)

// canonicalTrack 把 (number, total) 规范化为 "3"/"3/12"/"" 形态。
func canonicalTrack(number, total int) string {
	if number <= 0 {
		return ""
	}
	if total > 0 {
		return strconv.Itoa(number) + "/" + strconv.Itoa(total)
	}
	return strconv.Itoa(number)
}

// normalizeTrack 把音轨号字符串（"3" 或 "3/12"）规范化到与 canonicalTrack 一致的形态，
// 便于与文件读回的音轨号做等值比较。
func normalizeTrack(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	numStr, totalStr, _ := strings.Cut(s, "/")
	number, _ := strconv.Atoi(strings.TrimSpace(numStr))
	total, _ := strconv.Atoi(strings.TrimSpace(totalStr))
	return canonicalTrack(number, total)
}

func pictureEqual(a, b *tag.Picture) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.MIMEType == b.MIMEType && bytes.Equal(a.Data, b.Data)
}

func writeEditedSongTags(song *models.Song, fields []string) FileWriteStatus {
	if len(fields) == 0 || song.FilePath == "" {
		return FileWriteUnchanged
	}
	var opts tag.WriteOptions
	for _, field := range fields {
		var empty bool
		switch field {
		case tag.FieldTitle:
			opts.Title = song.Title
			empty = opts.Title == ""
		case tag.FieldArtist:
			opts.Artist = song.Artist
			empty = opts.Artist == ""
		case tag.FieldAlbum:
			opts.Album = song.Album
			empty = opts.Album == ""
		case tag.FieldYear:
			opts.Year = song.Year
			empty = opts.Year == 0
		case tag.FieldGenre:
			opts.Genre = song.Genre
			empty = opts.Genre == ""
		case tag.FieldLanguage:
			opts.Language = song.Language
			empty = opts.Language == ""
		case tag.FieldStyle:
			opts.Style = song.Style
			empty = opts.Style == ""
		case tag.FieldTrack:
			opts.Track = song.Track
			empty = opts.Track == ""
		case tag.FieldDisc:
			opts.Disc = song.Disc
			empty = opts.Disc == ""
		case tag.FieldLyrics:
			if song.LyricSource == models.LyricSourceURL {
				continue
			}
			opts.Lyrics = models.UnmarshalLyric(song.Lyric).Lyric
			empty = opts.Lyrics == ""
		case tag.FieldPicture:
			if song.CoverPath != "" {
				data, err := os.ReadFile(song.CoverPath)
				if err != nil {
					slog.Warn("read cover for tag write", "error", err)
					return FileWriteFailed
				}
				opts.Picture = &tag.Picture{MIMEType: tag.MIMETypeFromExt(filepath.Ext(song.CoverPath)), Data: data}
			}
			empty = song.CoverPath == "" && song.CoverURL == ""
		}
		if empty {
			opts.ClearFields = append(opts.ClearFields, field)
		}
	}
	return writeIncrementalSongTags(song.FilePath, opts)
}

func writeIncrementalSongTags(path string, opts tag.WriteOptions) FileWriteStatus {
	opts.Incremental = true
	err := tag.WriteTag(path, opts)
	if errors.Is(err, tag.ErrUnsupportedWrite) {
		return FileWriteUnchanged
	}
	if err != nil {
		slog.Warn("write edited tags", "path", path, "error", err)
		return FileWriteFailed
	}
	return FileWriteWritten
}
