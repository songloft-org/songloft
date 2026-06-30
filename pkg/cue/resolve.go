package cue

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolvedTrack 解析后的 track，已关联到实际音频文件路径并计算好 start/end 时间
type ResolvedTrack struct {
	AudioFilePath string
	TrackNumber   int
	Title         string
	Performer     string
	Album         string
	Genre         string
	Year          string
	ISRC          string
	StartSeconds  float64
	EndSeconds    float64 // 0 表示到文件末尾
}

// Duration 返回 track 时长（秒）。EndSeconds 为 0 时返回 0（需要外部传入总时长）。
func (t *ResolvedTrack) Duration() float64 {
	if t.EndSeconds <= 0 {
		return 0
	}
	d := t.EndSeconds - t.StartSeconds
	if d < 0 {
		return 0
	}
	return d
}

// ResolveTracks 将 CUESheet 解析为 ResolvedTrack 列表。
// cueDir 是 .cue 文件所在目录（用于解析 FILE 中的相对路径）。
// totalDurations 是各音频文件的总时长映射 (absolutePath -> seconds)。
func ResolveTracks(sheet *CUESheet, cueDir string, totalDurations map[string]float64) ([]ResolvedTrack, error) {
	if sheet == nil || len(sheet.Files) == 0 {
		return nil, fmt.Errorf("empty CUE sheet")
	}

	var result []ResolvedTrack
	var skippedFiles int

	for _, f := range sheet.Files {
		audioPath := resolveFilePath(cueDir, f.Filename)

		if _, err := os.Stat(audioPath); err != nil {
			skippedFiles++
			continue
		}

		totalDur := totalDurations[audioPath]

		for i, track := range f.Tracks {
			rt := ResolvedTrack{
				AudioFilePath: audioPath,
				TrackNumber:   track.Number,
				Title:         track.Title,
				Performer:     trackPerformer(track, sheet),
				Album:         sheet.Title,
				Genre:         sheet.Genre,
				Year:          sheet.Date,
				ISRC:          track.ISRC,
				StartSeconds:  track.Start.ToSeconds(),
			}

			if i+1 < len(f.Tracks) {
				rt.EndSeconds = f.Tracks[i+1].Start.ToSeconds()
			} else if totalDur > 0 {
				rt.EndSeconds = totalDur
			}

			if rt.Title == "" {
				rt.Title = fmt.Sprintf("Track %02d", track.Number)
			}

			result = append(result, rt)
		}
	}

	if len(result) == 0 {
		if skippedFiles > 0 {
			return nil, fmt.Errorf("all %d referenced audio files not found", skippedFiles)
		}
		return nil, fmt.Errorf("no tracks resolved")
	}

	return result, nil
}

func resolveFilePath(cueDir, filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join(cueDir, filename)
}

func trackPerformer(track CUETrack, sheet *CUESheet) string {
	if track.Performer != "" {
		return track.Performer
	}
	return sheet.Performer
}
