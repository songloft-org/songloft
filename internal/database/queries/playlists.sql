-- name: GetPlaylistByID :one
SELECT p.id, p.type, p.name, p.description, p.cover_path, p.cover_url, p.labels,
    p.position, p.created_at, p.updated_at,
    COALESCE(cnt.song_count, 0) as song_count
FROM playlists p
LEFT JOIN (SELECT playlist_id, COUNT(*) as song_count FROM playlist_songs WHERE playlist_id = ? GROUP BY playlist_id) cnt
ON p.id = cnt.playlist_id
WHERE p.id = ?;

-- name: FindPlaylistByName :one
SELECT id FROM playlists WHERE name = ? LIMIT 1;

-- name: FindPlaylistByNameExcludeID :one
SELECT id FROM playlists WHERE name = ? AND id != ? LIMIT 1;

-- name: GetMaxPlaylistPosition :one
SELECT CAST(COALESCE(MAX(position), 0) AS INTEGER) FROM playlists;

-- name: CreatePlaylist :execlastid
INSERT INTO playlists (type, name, description, cover_path, cover_url, labels, position)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdatePlaylist :execrows
UPDATE playlists SET
    name = ?, description = ?, cover_path = ?, cover_url = ?, labels = ?, updated_at = ?
WHERE id = ?;

-- name: TouchPlaylist :execrows
UPDATE playlists SET updated_at = ? WHERE id = ?;

-- name: DeletePlaylist :execrows
DELETE FROM playlists WHERE id = ?;

-- name: UpdatePlaylistPosition :execrows
UPDATE playlists SET position = ? WHERE id = ?;

-- name: ListAllPlaylistNames :many
SELECT name FROM playlists;

-- name: InsertAutoCreatedPlaylist :execlastid
INSERT INTO playlists (type, name, description, cover_path, cover_url, labels)
VALUES (?, ?, ?, ?, ?, ?);
