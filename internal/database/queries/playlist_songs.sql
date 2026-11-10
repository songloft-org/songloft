-- name: AddSongToPlaylist :exec
INSERT INTO playlist_songs (playlist_id, song_id, position)
VALUES (?, ?, ?);

-- name: RemoveSongFromPlaylist :execrows
DELETE FROM playlist_songs WHERE playlist_id = ? AND song_id = ?;

-- name: GetPlaylistSongs :many
SELECT sqlc.embed(s), ps.added_at AS playlist_added_at
FROM songs s
INNER JOIN playlist_songs ps ON s.id = ps.song_id
WHERE ps.playlist_id = ?
ORDER BY ps.position ASC;

-- name: GetPlaylistSongsPaginated :many
SELECT sqlc.embed(s), ps.added_at AS playlist_added_at
FROM songs s
INNER JOIN playlist_songs ps ON s.id = ps.song_id
WHERE ps.playlist_id = ?
ORDER BY ps.position ASC
LIMIT ? OFFSET ?;

-- name: CountPlaylistSongs :one
SELECT COUNT(*) FROM playlist_songs WHERE playlist_id = ?;

-- name: GetPlaylistCoverCandidates :many
SELECT s.id, s.cover_path, s.cover_url
FROM playlist_songs ps JOIN songs s ON s.id = ps.song_id
WHERE ps.playlist_id = ? AND (s.cover_path != '' OR s.cover_url != '')
ORDER BY ps.position, s.id
LIMIT 20;

-- name: ListPlaylistsContainingSong :many
SELECT ps.playlist_id
FROM playlist_songs ps
INNER JOIN playlists p ON p.id = ps.playlist_id
WHERE ps.song_id = ? AND p.type = 'normal';

-- name: FindSongPositionInPlaylist :one
SELECT position FROM playlist_songs WHERE playlist_id = ? AND song_id = ?;

-- name: UpdateSongPositionInPlaylist :execrows
UPDATE playlist_songs SET position = ? WHERE playlist_id = ? AND song_id = ?;

-- name: AddSongToPlaylistIgnore :execrows
INSERT OR IGNORE INTO playlist_songs (playlist_id, song_id, position)
VALUES (?, ?, ?);

-- name: MaxPositionInPlaylist :one
SELECT CAST(COALESCE(MAX(position), 0) AS INTEGER) FROM playlist_songs WHERE playlist_id = ?;

-- name: DeletePlaylistSongsByPlaylistID :exec
DELETE FROM playlist_songs WHERE playlist_id = ?;
