package database

import (
	"context"

	"songloft/internal/database/sqlc"
	"songloft/internal/models"
)

func (r *SongRepository) MarkMetadataOverrides(ctx context.Context, id int64, mask models.SongMetadataMask, editedAt int64) error {
	return r.queries.MarkSongMetadataOverrides(ctx, sqlc.MarkSongMetadataOverridesParams{
		ID: id, Mask: int64(mask), EditedAt: editedAt,
	})
}

func (r *SongRepository) ClearMetadataOverrides(ctx context.Context, id int64, mask models.SongMetadataMask) error {
	return r.queries.ClearSongMetadataOverrides(ctx, sqlc.ClearSongMetadataOverridesParams{
		ID: id, Mask: int64(mask),
	})
}
