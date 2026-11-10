package models

// PlaylistSongsChangeEvent describes one committed membership change, not a reorder.
type PlaylistSongsChangeEvent struct {
	PlaylistID int64             `json:"playlistId"`
	SongID     int64             `json:"songId"`
	Type       string            `json:"type"`      // add or remove
	Timestamp  int64             `json:"timestamp"` // Unix milliseconds
	Song       PlaylistEventSong `json:"song"`
}

// PlaylistEventSong preserves metadata even if an orphan song is deleted afterwards.
type PlaylistEventSong struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
}
