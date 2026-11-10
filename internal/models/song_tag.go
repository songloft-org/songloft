package models

import (
	"strconv"
	"strings"
	"time"
)

type SongTag struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Color         string    `json:"color"`
	SongCount     int       `json:"song_count"`
	PlaylistCount int       `json:"playlist_count"`
	CoverURL      string    `json:"cover_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// PlaylistTagLabel references a shared custom tag without changing system labels.
func PlaylistTagLabel(id int64) string { return "tag:" + strconv.FormatInt(id, 10) }

func ParsePlaylistTagLabel(label string) (int64, bool) {
	if !strings.HasPrefix(label, "tag:") {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(label, "tag:"), 10, 64)
	return id, err == nil && id > 0 && label == PlaylistTagLabel(id)
}
