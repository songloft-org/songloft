package models

// PlaylistProgress 是每个歌单独立保存的收听位置；Song 为 nil 表示没有有效位置。
type PlaylistProgress struct {
	Song       *Song  `json:"song"`
	PositionMS int64  `json:"position_ms"`
	PlayMode   string `json:"play_mode"`
	Completed  bool   `json:"completed"`
	Invalid    bool   `json:"invalid"`
	Version    int64  `json:"version"`
	UpdatedAt  string `json:"updated_at"`
}

// UpdatePlaylistProgress 保持未提供的字段，支持单独保存模式。
type UpdatePlaylistProgress struct {
	SongID     *int64  `json:"song_id"`
	PositionMS *int64  `json:"position_ms"`
	PlayMode   *string `json:"play_mode"`
	Completed  *bool   `json:"completed"`
	Version    int64   `json:"version"`
}
