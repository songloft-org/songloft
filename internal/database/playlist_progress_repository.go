package database

import (
	"context"
	"database/sql"
	"errors"
	"songloft/internal/database/sqlc"
)

var ErrProgressConflict = errors.New("playback progress changed on another device")

type PlaylistProgressRepository struct{ q *sqlc.Queries }

func NewPlaylistProgressRepository(db sqlc.DBTX) *PlaylistProgressRepository {
	return &PlaylistProgressRepository{sqlc.New(db)}
}
func (r *PlaylistProgressRepository) Get(ctx context.Context, id int64) (sqlc.PlaylistProgress, error) {
	row, err := r.q.GetPlaylistProgress(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return row, err
}
func (r *PlaylistProgressRepository) Save(ctx context.Context, row sqlc.PlaylistProgress) error {
	var n int64
	var err error
	if row.Version == 0 {
		n, err = r.q.CreatePlaylistProgress(ctx, sqlc.CreatePlaylistProgressParams{PlaylistID: row.PlaylistID, SongID: row.SongID, PositionMs: row.PositionMs, PlayMode: row.PlayMode, Completed: row.Completed})
	} else {
		n, err = r.q.UpdatePlaylistProgress(ctx, sqlc.UpdatePlaylistProgressParams{PlaylistID: row.PlaylistID, SongID: row.SongID, PositionMs: row.PositionMs, PlayMode: row.PlayMode, Completed: row.Completed, Version: row.Version})
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrProgressConflict
	}
	return nil
}
func (r *PlaylistProgressRepository) Contains(ctx context.Context, pid, sid int64) (bool, error) {
	_, err := r.q.FindSongPositionInPlaylist(ctx, sqlc.FindSongPositionInPlaylistParams{PlaylistID: pid, SongID: sid})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
func (r *PlaylistProgressRepository) Clear(ctx context.Context, id int64) error {
	return r.q.ClearPlaylistProgress(ctx, id)
}
