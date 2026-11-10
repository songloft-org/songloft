-- name: CreateSongTag :execlastid
INSERT INTO song_tags (name, color) VALUES (?, ?);

-- name: GetSongTagByID :one
SELECT id, name, color, created_at FROM song_tags WHERE id = ?;

-- name: GetSongTagByName :one
SELECT id, name, color, created_at FROM song_tags WHERE name = ?;

-- name: UpdateSongTag :exec
UPDATE song_tags SET name = ?, color = ? WHERE id = ?;

-- name: DeleteSongTag :exec
DELETE FROM song_tags WHERE id = ?;

-- name: LinkSongTag :exec
INSERT OR IGNORE INTO song_tag_links (song_id, tag_id) VALUES (?, ?);

-- name: UnlinkSongTag :exec
DELETE FROM song_tag_links WHERE song_id = ? AND tag_id = ?;

-- name: UnlinkAllBySong :exec
DELETE FROM song_tag_links WHERE song_id = ?;

-- name: GetTagsBySongID :many
SELECT t.id, t.name, t.color, t.created_at
FROM song_tags t
JOIN song_tag_links l ON t.id = l.tag_id
WHERE l.song_id = ?
ORDER BY t.name;

-- name: CountSongsByTag :one
SELECT COUNT(*) FROM song_tag_links WHERE tag_id = ?;

-- name: GetTagsByPlaylistID :many
SELECT t.id, t.name, t.color, t.created_at
FROM song_tags t
WHERE EXISTS (
    SELECT 1 FROM playlists p, json_each(p.labels) j
    WHERE p.id = ? AND j.value = 'tag:' || t.id
)
ORDER BY t.name;

-- name: CountVisiblePlaylistsByTag :one
SELECT COUNT(*) FROM playlists p
WHERE EXISTS (SELECT 1 FROM json_each(p.labels) j WHERE j.value = CAST(sqlc.arg(label) AS TEXT))
AND NOT EXISTS (SELECT 1 FROM json_each(p.labels) h WHERE h.value = 'hidden');

-- name: ClearPlaylistTagReferences :exec
UPDATE playlists
SET labels = (SELECT json_group_array(j.value) FROM json_each(playlists.labels) j WHERE j.value != CAST(sqlc.arg(label) AS TEXT)),
    updated_at = CURRENT_TIMESTAMP
WHERE EXISTS (SELECT 1 FROM json_each(playlists.labels) j WHERE j.value = CAST(sqlc.arg(label) AS TEXT));
