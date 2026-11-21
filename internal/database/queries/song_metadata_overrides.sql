-- name: MarkSongMetadataOverrides :exec
UPDATE songs SET
    metadata_override_mask = metadata_override_mask | CAST(sqlc.arg(mask) AS INTEGER),
    metadata_edited_at = MAX(metadata_edited_at, CAST(sqlc.arg(edited_at) AS INTEGER))
WHERE id = sqlc.arg(id);

-- name: ClearSongMetadataOverrides :exec
UPDATE songs SET
    metadata_override_mask = metadata_override_mask & ~CAST(sqlc.arg(mask) AS INTEGER),
    metadata_edited_at = CASE
        WHEN (metadata_override_mask & ~CAST(sqlc.arg(mask) AS INTEGER)) = 0 THEN 0
        ELSE metadata_edited_at
    END
WHERE id = sqlc.arg(id);
