package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"songloft/internal/database"
	"songloft/internal/database/sqlc"
	"songloft/internal/models"
)

var ErrInvalidProgress = errors.New("invalid playback progress")

type PlaylistProgressService struct{ db database.DB }

func NewPlaylistProgressService(db database.DB) *PlaylistProgressService {
	return &PlaylistProgressService{db}
}
func (s *PlaylistProgressService) Get(ctx context.Context, id int64) (*models.PlaylistProgress, error) {
	return readPlaylistProgress(ctx, id, s.db.PlaylistRepository(), s.db.SongRepository(), s.db.PlaylistProgressRepository())
}
func readPlaylistProgress(ctx context.Context, id int64, playlists *database.PlaylistRepository, songs *database.SongRepository, progress *database.PlaylistProgressRepository) (*models.PlaylistProgress, error) {
	p, err := playlists.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Type != models.PlaylistTypeNormal {
		return nil, ErrInvalidProgress
	}
	row, err := progress.Get(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return &models.PlaylistProgress{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := &models.PlaylistProgress{PositionMS: row.PositionMs, PlayMode: row.PlayMode, Completed: row.Completed != 0, Version: row.Version, UpdatedAt: row.UpdatedAt}
	if row.SongID.Valid {
		ok, err := progress.Contains(ctx, id, row.SongID.Int64)
		if err != nil {
			return nil, err
		}
		if ok {
			out.Song, err = songs.GetByID(ctx, row.SongID.Int64)
			if err != nil && !errors.Is(err, database.ErrNotFound) {
				return nil, err
			}
		}
		if out.Song == nil {
			out.Invalid = true
			out.PositionMS = 0
			out.Completed = false
		}
	}
	return out, nil
}
func (s *PlaylistProgressService) Save(ctx context.Context, id int64, in models.UpdatePlaylistProgress) (*models.PlaylistProgress, error) {
	if in.Version < 0 || (in.PositionMS != nil && *in.PositionMS < 0) || (in.SongID != nil && *in.SongID <= 0) {
		return nil, ErrInvalidProgress
	}
	if in.PlayMode != nil {
		switch *in.PlayMode {
		case "order", "loop", "single", "random", "singlePlay":
		default:
			return nil, ErrInvalidProgress
		}
	}
	var out *models.PlaylistProgress
	err := s.db.RunInTx(ctx, func(ctx context.Context, u *database.UnitOfWork) error {
		p, err := u.Playlists.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if p.Type != models.PlaylistTypeNormal {
			return ErrInvalidProgress
		}
		row, err := u.PlaylistProgress.Get(ctx, id)
		if errors.Is(err, database.ErrNotFound) {
			row = sqlc.PlaylistProgress{PlaylistID: id}
		} else if err != nil {
			return err
		}
		if row.Version != in.Version {
			return database.ErrProgressConflict
		}
		if in.SongID != nil {
			ok, err := u.PlaylistProgress.Contains(ctx, id, *in.SongID)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("%w: song not in playlist", ErrInvalidProgress)
			}
			row.SongID = sql.NullInt64{Int64: *in.SongID, Valid: true}
		}
		if in.PositionMS != nil {
			row.PositionMs = *in.PositionMS
		}
		if in.PlayMode != nil {
			row.PlayMode = *in.PlayMode
		}
		if in.Completed != nil {
			row.Completed = 0
			if *in.Completed {
				row.Completed = 1
			}
		}
		if !row.SongID.Valid && (row.PositionMs != 0 || row.Completed != 0) {
			return ErrInvalidProgress
		}
		if err := u.PlaylistProgress.Save(ctx, row); err != nil {
			return err
		}
		out, err = readPlaylistProgress(ctx, id, u.Playlists, u.Songs, u.PlaylistProgress)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
func (s *PlaylistProgressService) Clear(ctx context.Context, id int64) error {
	return s.db.RunInTx(ctx, func(ctx context.Context, u *database.UnitOfWork) error {
		if _, err := readPlaylistProgress(ctx, id, u.Playlists, u.Songs, u.PlaylistProgress); err != nil {
			return err
		}
		if _, err := u.PlaylistProgress.Get(ctx, id); errors.Is(err, database.ErrNotFound) {
			if err := u.PlaylistProgress.Save(ctx, sqlc.PlaylistProgress{PlaylistID: id}); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		return u.PlaylistProgress.Clear(ctx, id)
	})
}
