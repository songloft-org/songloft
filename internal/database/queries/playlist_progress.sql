-- name: GetPlaylistProgress :one
SELECT * FROM playlist_progress WHERE playlist_id = ?;

-- name: CreatePlaylistProgress :execrows
INSERT INTO playlist_progress (playlist_id, song_id, position_ms, play_mode, completed)
VALUES (?, ?, ?, ?, ?) ON CONFLICT(playlist_id) DO NOTHING;

-- name: UpdatePlaylistProgress :execrows
UPDATE playlist_progress SET song_id = ?, position_ms = ?, play_mode = ?, completed = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE playlist_id = ? AND version = ?;

-- name: ClearPlaylistProgress :exec
UPDATE playlist_progress SET song_id = NULL, position_ms = 0, completed = 0, version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE playlist_id = ?;
