package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"songloft/internal/models"

	"github.com/hanxi/tag"
)

const (
	coverDownloadTimeout = 15 * time.Second
	maxCoverSize         = 10 << 20 // 10MB
)

// WriteCacheSongTags 把明确提供的歌曲元数据写入下载文件，保留已有的其他标签。
// CoverPath 为空时尝试下载 CoverURL。
func WriteCacheSongTags(filePath string, song *models.Song, httpClient *http.Client) FileWriteStatus {
	if filePath == "" || song == nil {
		return FileWriteUnchanged
	}

	mainLyric := models.UnmarshalLyric(song.Lyric).Lyric
	if song.LyricSource == models.LyricSourceURL {
		mainLyric = ""
	}

	opts := tag.WriteOptions{
		Incremental: true,
		Title:       song.Title,
		Artist:      song.Artist,
		Album:       song.Album,
		Lyrics:      mainLyric,
		Genre:       song.Genre,
		Track:       song.Track,
		Disc:        song.Disc,
	}

	if song.Year > 0 {
		opts.Year = song.Year
	}

	// 封面：优先本地文件，否则尝试下载远程封面
	if song.CoverPath != "" {
		if data, err := os.ReadFile(song.CoverPath); err == nil {
			opts.Picture = &tag.Picture{
				MIMEType: tag.MIMETypeFromExt(filepath.Ext(song.CoverPath)),
				Data:     data,
			}
		}
	} else if song.CoverURL != "" && httpClient != nil {
		tmpPath, err := downloadCoverToTemp(song.CoverURL, httpClient)
		if err != nil {
			slog.Debug("download cover for cache tag failed, skipping",
				"coverURL", song.CoverURL, "error", err)
		} else {
			defer os.Remove(tmpPath)
			if data, err := os.ReadFile(tmpPath); err == nil {
				opts.Picture = &tag.Picture{
					MIMEType: tag.MIMETypeFromExt(filepath.Ext(tmpPath)),
					Data:     data,
				}
			}
		}
	}

	if err := tag.WriteTag(filePath, opts); err != nil {
		if errors.Is(err, tag.ErrUnsupportedWrite) {
			slog.Debug("cache tag write skipped for unsupported format",
				"path", filePath, "error", err)
			return FileWriteUnchanged
		}
		slog.Warn("cache tag write failed", "path", filePath, "error", err)
		return FileWriteFailed
	}

	slog.Debug("cache tag written", "path", filePath,
		"title", opts.Title, "artist", opts.Artist,
		"hasPicture", opts.Picture != nil,
		"lyricsLen", len(opts.Lyrics))
	return FileWriteWritten
}

// downloadCoverToTemp 下载远程封面到临时文件。调用方负责 os.Remove。
func downloadCoverToTemp(coverURL string, client *http.Client) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), coverDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "image/*")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") && ct != "application/octet-stream" {
		return "", fmt.Errorf("unexpected content-type: %s", ct)
	}

	ext := coverExtFromContentType(ct)
	tmp, err := os.CreateTemp("", "songloft-cover-*"+ext)
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	_, err = io.Copy(tmp, io.LimitReader(resp.Body, maxCoverSize))
	closeErr := tmp.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("write temp: %w", err)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close temp: %w", closeErr)
	}
	return tmpPath, nil
}

func coverExtFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	default:
		return ".jpg"
	}
}
