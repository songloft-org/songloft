-- name: GetSongByID :one
SELECT id, type, title, artist, album, duration, file_path, url,
    cover_path, cover_url, lyric, lyric_source, file_size,
    format, bit_rate, sample_rate, is_live,
    plugin_entry_path, source_data, dedup_key,
    added_at, updated_at, lyric_remote_url,
    year, genre,
    fingerprint, fingerprint_duration,
    isrc, cache_path,
    cue_source_path, cue_track_index, cue_audio_path,
    file_modified_at, track, language, style, is_video,
    cue_start_seconds, cue_end_seconds
FROM songs WHERE id = ?;

-- name: CreateSong :execlastid
INSERT INTO songs (
    type, title, artist, album, duration, file_path, url,
    cover_path, cover_url, lyric, lyric_source, lyric_remote_url,
    file_size, format, bit_rate, sample_rate, is_live,
    plugin_entry_path, source_data, dedup_key,
    year, genre, language, style,
    fingerprint, fingerprint_duration,
    isrc, track,
    cue_source_path, cue_track_index, cue_audio_path,
    file_modified_at, is_video,
    cue_start_seconds, cue_end_seconds
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateSong :execrows
UPDATE songs SET
    type = ?, title = ?, artist = ?, album = ?, duration = ?,
    file_path = ?, url = ?, cover_path = ?, cover_url = ?,
    lyric = ?, lyric_source = ?, lyric_remote_url = ?,
    file_size = ?, format = ?, bit_rate = ?, sample_rate = ?, is_live = ?,
    plugin_entry_path = ?, source_data = ?, dedup_key = ?,
    year = ?, genre = ?, language = ?, style = ?,
    fingerprint = ?, fingerprint_duration = ?,
    isrc = ?, track = ?,
    cue_source_path = ?, cue_track_index = ?, cue_audio_path = ?,
    file_modified_at = ?, is_video = ?,
    cue_start_seconds = ?, cue_end_seconds = ?
WHERE id = ?;

-- name: DeleteSong :execrows
DELETE FROM songs WHERE id = ?;

-- name: UpdateSongLyrics :execrows
UPDATE songs SET lyric = ?, lyric_source = ?, lyric_remote_url = ? WHERE id = ?;

-- name: UpdateSongCoverURL :execrows
UPDATE songs SET cover_url = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UpdateSongDuration :exec
UPDATE songs SET duration = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND (duration = 0 OR duration IS NULL);

-- name: UpdateSongSource :exec
UPDATE songs SET plugin_entry_path = ?, source_data = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ListLocalSongPaths :many
SELECT id, file_path, duration, cue_source_path FROM songs WHERE type = 'local';

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
    lyric_remote_url = CASE WHEN ? != '' THEN ? ELSE lyric_remote_url END,
    year = CASE WHEN ? > 0 THEN ? ELSE year END,
    genre = CASE WHEN ? != '' THEN ? ELSE genre END
WHERE id = ?;

-- name: GetSongTimestamps :one
SELECT added_at, updated_at FROM songs WHERE id = ?;

-- name: UpdateSongFingerprint :exec
UPDATE songs SET fingerprint = ?, fingerprint_duration = ? WHERE id = ?;

-- name: ClearAllFingerprints :exec
UPDATE songs SET fingerprint = '', fingerprint_duration = 0 WHERE type = 'local' AND fingerprint != '';

-- name: ListLocalWithoutFingerprint :many
SELECT id, file_path FROM songs WHERE type = 'local' AND fingerprint = '';

-- name: CountLocalFingerprints :one
SELECT
    COUNT(*) AS total,
    CAST(COALESCE(SUM(CASE WHEN fingerprint != '' THEN 1 ELSE 0 END), 0) AS INTEGER) AS computed
FROM songs WHERE type = 'local';

-- name: ListDuplicateFingerprints :many
SELECT fingerprint, COUNT(*) AS cnt
FROM songs
WHERE fingerprint != '' AND type = 'local'
GROUP BY fingerprint
HAVING COUNT(*) > 1;

-- name: ListSongsByFingerprint :many
SELECT id, type, title, artist, album, duration, file_path,
    format, bit_rate, sample_rate, file_size, fingerprint_duration,
    cover_path, cover_url, added_at
FROM songs
WHERE fingerprint = ?;

-- name: UpdateCachePath :exec
UPDATE songs SET cache_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: ClearCachePath :exec
UPDATE songs SET cache_path = '', updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: ClearAllCachePaths :exec
UPDATE songs SET cache_path = '', updated_at = CURRENT_TIMESTAMP WHERE cache_path != '';

-- name: ListSongsWithCache :many
SELECT id, type, title, artist, album, duration, file_path, url,
    cover_path, cover_url, lyric, lyric_source, file_size,
    format, bit_rate, sample_rate, is_live,
    plugin_entry_path, source_data, dedup_key,
    added_at, updated_at, lyric_remote_url,
    year, genre,
    fingerprint, fingerprint_duration,
    isrc, cache_path,
    cue_source_path, cue_track_index, cue_audio_path,
    file_modified_at, track, language, style, is_video,
    cue_start_seconds, cue_end_seconds
FROM songs WHERE cache_path != '';

-- name: ListSongsNeedingMetadata :many
SELECT id, plugin_entry_path, source_data, url,
    title, artist, album, duration,
    bit_rate, sample_rate, format, cover_path, cover_url
FROM songs
WHERE type = 'remote' AND (
    duration = 0 OR duration IS NULL
    OR bit_rate = 0 OR sample_rate = 0 OR format = ''
);

-- name: UpdateSongMetadata :exec
UPDATE songs SET
    duration    = CASE WHEN duration = 0    AND ? > 0   THEN ? ELSE duration END,
    bit_rate    = CASE WHEN bit_rate = 0    AND ? > 0   THEN ? ELSE bit_rate END,
    sample_rate = CASE WHEN sample_rate = 0 AND ? > 0   THEN ? ELSE sample_rate END,
    format      = CASE WHEN format = ''     AND ? != '' THEN ? ELSE format END,
    title       = CASE WHEN title = ''      AND ? != '' THEN ? ELSE title END,
    artist      = CASE WHEN artist = ''     AND ? != '' THEN ? ELSE artist END,
    album       = CASE WHEN album = ''      AND ? != '' THEN ? ELSE album END,
    cover_path  = CASE WHEN cover_path = '' AND ? != '' THEN ? ELSE cover_path END,
    updated_at  = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateSongTagFields :exec
UPDATE songs SET
    title  = CASE WHEN ? != '' THEN ? ELSE title END,
    artist = CASE WHEN ? != '' THEN ? ELSE artist END,
    album  = CASE WHEN ? != '' THEN ? ELSE album END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ListCueSources :many
SELECT DISTINCT cue_source_path
FROM songs WHERE cue_source_path != '';

-- name: ListCueAudioPaths :many
SELECT DISTINCT cue_audio_path
FROM songs WHERE cue_source_path = ? AND cue_audio_path != '';

-- name: DeleteByCueSource :execrows
DELETE FROM songs WHERE cue_source_path = ?;

-- 标签分类聚合（facet）已改为 song_repository.go 内的 squirrel 动态查询，
-- 以支持可选 keyword / 动态排序 / 分页 / 代表封面。sqlc 固定查询无法表达故此处不再定义。
