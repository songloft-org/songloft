package models

import "time"

const BackupVersion = 2

type BackupData struct {
	Version    int              `json:"version"`
	ExportedAt time.Time        `json:"exported_at"`
	Playlists  []BackupPlaylist `json:"playlists"`
	Tags       []BackupTag      `json:"tags,omitempty"`
}

type BackupTag struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type BackupPlaylist struct {
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Description string       `json:"description"`
	Labels      []string     `json:"labels"`
	Songs       []BackupSong `json:"songs"`
	TagIDs      []int64      `json:"tag_ids,omitempty"`
}

type BackupSong struct {
	TagIDs          []int64 `json:"tag_ids,omitempty"`
	Type            string  `json:"type"`
	Title           string  `json:"title"`
	Artist          string  `json:"artist"`
	Album           string  `json:"album"`
	Duration        float64 `json:"duration"`
	FilePath        string  `json:"file_path"`
	URL             string  `json:"url"`
	CoverURL        string  `json:"cover_url"`
	FileSize        int64   `json:"file_size"`
	Format          string  `json:"format"`
	BitRate         int     `json:"bit_rate"`
	SampleRate      int     `json:"sample_rate"`
	IsLive          bool    `json:"is_live"`
	PluginEntryPath string  `json:"plugin_entry_path"`
	SourceData      string  `json:"source_data"`
	DedupKey        string  `json:"dedup_key"`
}

type ImportResult struct {
	PlaylistsCreated int `json:"playlists_created"`
	PlaylistsMerged  int `json:"playlists_merged"`
	SongsCreated     int `json:"songs_created"`
	SongsMatched     int `json:"songs_matched"`
	SongsSkipped     int `json:"songs_skipped"`
}
