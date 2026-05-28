-- name: GetSongByID :one
SELECT id, type, title, artist, album, duration, file_path, url,
    cover_path, cover_url, lyric, lyric_source, file_size,
    format, bit_rate, sample_rate, is_live,
    plugin_entry_path, source_data, dedup_key,
    added_at, updated_at, lyric_remote_url
FROM songs WHERE id = ?;

-- name: CreateSong :execlastid
INSERT INTO songs (
    type, title, artist, album, duration, file_path, url,
    cover_path, cover_url, lyric, lyric_source, lyric_remote_url,
    file_size, format, bit_rate, sample_rate, is_live,
    plugin_entry_path, source_data, dedup_key
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateSong :execrows
UPDATE songs SET
    type = ?, title = ?, artist = ?, album = ?, duration = ?,
    file_path = ?, url = ?, cover_path = ?, cover_url = ?,
    lyric = ?, lyric_source = ?, lyric_remote_url = ?,
    file_size = ?, format = ?, bit_rate = ?, sample_rate = ?, is_live = ?,
    plugin_entry_path = ?, source_data = ?, dedup_key = ?
WHERE id = ?;

-- name: DeleteSong :execrows
DELETE FROM songs WHERE id = ?;

-- name: UpdateSongLyrics :execrows
UPDATE songs SET lyric = ?, lyric_source = ?, lyric_remote_url = ? WHERE id = ?;

-- name: UpdateSongDuration :exec
UPDATE songs SET duration = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND (duration = 0 OR duration IS NULL);

-- name: UpdateSongSource :exec
UPDATE songs SET plugin_entry_path = ?, source_data = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ListLocalSongPaths :many
SELECT id, file_path FROM songs WHERE type = 'local';

-- name: CountPlaylistsContainingSong :one
SELECT COUNT(*) FROM playlist_songs WHERE song_id = ?;

-- name: CountSongsByCoverPath :one
SELECT COUNT(*) FROM songs WHERE cover_path = ?;

-- name: CountPlaylistsByCoverPath :one
SELECT COUNT(*) FROM playlists WHERE cover_path = ?;

-- name: FindSongByDedupKey :one
SELECT id FROM songs WHERE plugin_entry_path = ? AND dedup_key = ?;

-- name: UpdateRemoteSongMutable :exec
UPDATE songs SET
    title = ?,
    artist = ?,
    album = ?,
    cover_url = ?,
    source_data = ?,
    duration = CASE WHEN ? > 0 THEN ? ELSE duration END,
    lyric = CASE WHEN ? != '' THEN ? ELSE lyric END,
    lyric_source = CASE WHEN ? != '' THEN ? ELSE lyric_source END,
    lyric_remote_url = CASE WHEN ? != '' THEN ? ELSE lyric_remote_url END
WHERE id = ?;

-- name: GetSongTimestamps :one
SELECT added_at, updated_at FROM songs WHERE id = ?;
