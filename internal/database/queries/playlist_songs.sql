-- name: AddSongToPlaylist :exec
INSERT INTO playlist_songs (playlist_id, song_id, position)
VALUES (?, ?, ?);

-- name: RemoveSongFromPlaylist :execrows
DELETE FROM playlist_songs WHERE playlist_id = ? AND song_id = ?;

-- name: GetPlaylistSongs :many
SELECT s.id, s.type, s.title, s.artist, s.album, s.duration,
    s.file_path, s.url, s.cover_path, s.cover_url, s.lyric,
    s.lyric_source, s.file_size, s.format, s.bit_rate,
    s.sample_rate, s.is_live,
    s.plugin_entry_path, s.source_data, s.dedup_key,
    s.added_at, s.updated_at, s.lyric_remote_url
FROM songs s
INNER JOIN playlist_songs ps ON s.id = ps.song_id
WHERE ps.playlist_id = ?
ORDER BY ps.position ASC;

-- name: GetPlaylistSongsPaginated :many
SELECT s.id, s.type, s.title, s.artist, s.album, s.duration,
    s.file_path, s.url, s.cover_path, s.cover_url, s.lyric,
    s.lyric_source, s.file_size, s.format, s.bit_rate,
    s.sample_rate, s.is_live,
    s.plugin_entry_path, s.source_data, s.dedup_key,
    s.added_at, s.updated_at, s.lyric_remote_url
FROM songs s
INNER JOIN playlist_songs ps ON s.id = ps.song_id
WHERE ps.playlist_id = ?
ORDER BY ps.position ASC
LIMIT ? OFFSET ?;

-- name: CountPlaylistSongs :one
SELECT COUNT(*) FROM playlist_songs WHERE playlist_id = ?;

-- name: ListPlaylistsContainingSong :many
SELECT ps.playlist_id
FROM playlist_songs ps
INNER JOIN playlists p ON p.id = ps.playlist_id
WHERE ps.song_id = ? AND p.type = 'normal';

-- name: FindSongPositionInPlaylist :one
SELECT position FROM playlist_songs WHERE playlist_id = ? AND song_id = ?;

-- name: UpdateSongPositionInPlaylist :execrows
UPDATE playlist_songs SET position = ? WHERE playlist_id = ? AND song_id = ?;
