-- +goose Up
CREATE TABLE playlist_progress (
 playlist_id INTEGER PRIMARY KEY REFERENCES playlists(id) ON DELETE CASCADE,
 song_id INTEGER,
 position_ms INTEGER NOT NULL DEFAULT 0,
 play_mode TEXT NOT NULL DEFAULT '',
 completed INTEGER NOT NULL DEFAULT 0,
 version INTEGER NOT NULL DEFAULT 1,
 updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose Down
DROP TABLE playlist_progress;
