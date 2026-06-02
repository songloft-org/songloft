package services

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"

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
	// FileWriteSkipped 因前置条件不满足而未尝试回写：
	//   - 不是本地歌曲 / file_path 为空
	//   - lyric_source = url（运行时拉取，不缓存到本地文件）
	//   - 文件扩展名不在 pkg/tag 支持的写入清单（pkg/tag 返回 ErrUnsupportedWrite）
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
		return FileWriteSkipped
	}

	// song.Lyric 是 LyricPayload JSON；tag 只能写纯 LRC 文本，取主歌词字段
	mainLyric := models.UnmarshalLyric(song.Lyric).Lyric

	opts := tag.WriteOptions{
		Title:       song.Title,
		Artist:      song.Artist,
		AlbumArtist: song.Artist, // 大多数情况下专辑艺术家与艺术家一致
		Album:       song.Album,
		Lyrics:      mainLyric,
	}

	// 解析 added_at 年份作为发行年（保守兜底；网络歌曲的 Song 模型没有专门的 year 字段）
	if !song.AddedAt.IsZero() {
		opts.Year = song.AddedAt.Year()
	}

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

	if err := tag.WriteTag(filePath, opts); err != nil {
		if errors.Is(err, tag.ErrUnsupportedWrite) {
			slog.Debug("tag write skipped for unsupported format",
				"path", filePath, "error", err)
			return FileWriteSkipped
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
