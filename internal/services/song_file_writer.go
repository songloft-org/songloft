package services

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"songloft/internal/models"

	"github.com/hanxi/tag"
)

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
	// FileWriteSkipped 文件标签与待写入内容一致，跳过写入。
	FileWriteSkipped FileWriteStatus = "skipped"
	// FileWriteFailed 实际写入时报错（IO / 解析失败等）。
	FileWriteFailed FileWriteStatus = "failed"
)

// WriteSongTags 把 song 的元数据完整回写到 song.FilePath。
//
// 关键约束：pkg/tag.WriteTag 是「重建标签块」模式，未填充的字段会被清空。
// 因此本函数要求传入完整的 *models.Song，把 Title/Artist/Album/Lyrics/Picture
// 等所有想保留的字段一次性写回；不要在调用前只填一两个字段。
//
// Lyrics 字段：song.Lyric 在 DB 里是 LyricPayload JSON，本函数只取主歌词
// （tag 库只能写一段 LRC 文本，tlyric/rlyric/lxlyric 不会写入音频文件）。
//
// 失败处理：仅返回状态码，不返回 error。失败/跳过都会打 log。
func WriteSongTags(filePath string, song *models.Song) FileWriteStatus {
	if filePath == "" || song == nil {
		return FileWriteUnchanged
	}

	// song.Lyric 是 LyricPayload JSON；tag 只能写纯 LRC 文本，取主歌词字段
	mainLyric := models.UnmarshalLyric(song.Lyric).Lyric

	opts := tag.WriteOptions{
		Title:       song.Title,
		Artist:      song.Artist,
		AlbumArtist: song.Artist, // 大多数情况下专辑艺术家与艺术家一致
		Album:       song.Album,
		Lyrics:      mainLyric,
		Track:       song.Track,
		Disc:        song.Disc,
	}

	if song.Year > 0 {
		opts.Year = song.Year
	} else if !song.AddedAt.IsZero() {
		opts.Year = song.AddedAt.Year()
	}
	opts.Genre = song.Genre
	opts.Language = song.Language
	opts.Style = song.Style

	// 防御性处理：lyric_source=url 不应该被回写到文件
	if song.LyricSource == models.LyricSourceURL {
		opts.Lyrics = ""
	}

	// 读取封面（优先 cover_path 本地文件）
	if song.CoverPath != "" {
		if data, err := os.ReadFile(song.CoverPath); err == nil {
			opts.Picture = &tag.Picture{
				MIMEType:    tag.MIMETypeFromExt(filepath.Ext(song.CoverPath)),
				Data:        data,
				Description: "",
			}
		} else {
			slog.Debug("read cover failed, skip embedding",
				"coverPath", song.CoverPath, "error", err)
		}
	}

	if tagsUnchanged(filePath, opts) {
		slog.Debug("tag unchanged, skip write", "path", filePath)
		return FileWriteSkipped
	}

	if err := tag.WriteTag(filePath, opts); err != nil {
		if errors.Is(err, tag.ErrUnsupportedWrite) {
			slog.Debug("tag write skipped for unsupported format",
				"path", filePath, "error", err)
			return FileWriteUnchanged
		}
		slog.Warn("write tag failed", "path", filePath, "error", err)
		return FileWriteFailed
	}

	slog.Debug("tag written", "path", filePath,
		"title", opts.Title, "artist", opts.Artist,
		"hasPicture", opts.Picture != nil,
		"lyricsLen", len(opts.Lyrics))
	return FileWriteWritten
}

// tagsUnchanged 读取文件现有标签，与待写入的 opts 逐字段比较。
// 全部一致返回 true；任何字段不同、或读取失败则返回 false（继续写入）。
func tagsUnchanged(filePath string, opts tag.WriteOptions) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return false
	}

	if m.Title() != opts.Title ||
		m.Artist() != opts.Artist ||
		m.AlbumArtist() != opts.AlbumArtist ||
		m.Album() != opts.Album ||
		m.Genre() != opts.Genre ||
		m.Language() != opts.Language ||
		m.Style() != opts.Style ||
		m.Lyrics() != opts.Lyrics ||
		m.Year() != opts.Year {
		return false
	}

	if opts.Disc != "" && canonicalTrack(m.Disc()) != normalizeTrack(opts.Disc) {
		return false
	}
	fileTrackNum, fileTrackTotal := m.Track()
	if canonicalTrack(fileTrackNum, fileTrackTotal) != normalizeTrack(opts.Track) {
		return false
	}

	return pictureEqual(m.Picture(), opts.Picture)
}

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
	opts := tag.WriteOptions{Incremental: true}
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
	err := tag.WriteTag(song.FilePath, opts)
	if errors.Is(err, tag.ErrUnsupportedWrite) {
		return FileWriteUnchanged
	}
	if err != nil {
		slog.Warn("write edited tags", "path", song.FilePath, "error", err)
		return FileWriteFailed
	}
	return FileWriteWritten
}
