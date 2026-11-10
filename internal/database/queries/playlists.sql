-- name: GetPlaylistByID :one
SELECT p.id, p.type, p.name, p.description, p.cover_path, p.cover_url, p.labels,
    p.position, p.sort_by, p.sort_order, p.pinned_at, p.created_at, p.updated_at,
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

-- name: UpdatePlaylistSort :execrows
UPDATE playlists SET sort_by = ?, sort_order = ? WHERE id = ?;

-- name: TouchPlaylist :execrows
UPDATE playlists SET updated_at = ? WHERE id = ?;

-- name: DeletePlaylist :execrows
DELETE FROM playlists WHERE id = ?;

-- name: UpdatePlaylistPosition :execrows
UPDATE playlists SET position = ? WHERE id = ?;

-- name: SetPlaylistPinned :execrows
UPDATE playlists SET pinned_at = ? WHERE id = ?;

-- name: ListAllPlaylistNames :many
SELECT name FROM playlists;

-- name: InsertAutoCreatedPlaylist :execlastid
INSERT INTO playlists (type, name, description, cover_path, cover_url, labels)
VALUES (?, ?, ?, ?, ?, ?);

-- name: UpdatePlaylistLabels :execrows
UPDATE playlists SET labels = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: ListAutoCreatedPlaylists :many
SELECT id, name, cover_path, cover_url FROM playlists
WHERE EXISTS (SELECT 1 FROM json_each(labels) WHERE value = 'auto_created');

-- name: UpdateAutoCreatedPlaylistMeta :execrows
UPDATE playlists SET name = ?, description = ?, cover_path = ?, cover_url = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;
