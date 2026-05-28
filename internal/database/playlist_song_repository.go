package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"mimusic/internal/database/sqlc"
	"mimusic/internal/models"
)

// PlaylistSongRepository 负责 playlist_songs 关联表的读写。
type PlaylistSongRepository struct {
	db      sqlc.DBTX
	queries *sqlc.Queries
}

// NewPlaylistSongRepository 创建仓储实例。
func NewPlaylistSongRepository(db sqlc.DBTX) *PlaylistSongRepository {
	return &PlaylistSongRepository{db: db, queries: sqlc.New(db)}
}

// AddSong 把一首歌曲添加到歌单末尾给定位置。
func (r *PlaylistSongRepository) AddSong(ctx context.Context, playlistID, songID int64, position int) error {
	if err := r.queries.AddSongToPlaylist(ctx, sqlc.AddSongToPlaylistParams{
		PlaylistID: playlistID,
		SongID:     songID,
		Position:   int64(position),
	}); err != nil {
		return fmt.Errorf("add song to playlist: %w", err)
	}
	return nil
}

// RemoveSong 从歌单中移除指定歌曲，不存在时返回 ErrNotFound。
func (r *PlaylistSongRepository) RemoveSong(ctx context.Context, playlistID, songID int64) error {
	rows, err := r.queries.RemoveSongFromPlaylist(ctx, sqlc.RemoveSongFromPlaylistParams{
		PlaylistID: playlistID,
		SongID:     songID,
	})
	if err != nil {
		return fmt.Errorf("remove song from playlist: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetSongs 按 position 升序返回歌单内的所有歌曲。
func (r *PlaylistSongRepository) GetSongs(ctx context.Context, playlistID int64) ([]*models.Song, error) {
	rows, err := r.queries.GetPlaylistSongs(ctx, playlistID)
	if err != nil {
		return nil, fmt.Errorf("get playlist songs: %w", err)
	}
	songs := make([]*models.Song, 0, len(rows))
	for _, row := range rows {
		songs = append(songs, songRowToModel(row))
	}
	return songs, nil
}

// GetSongsPaginated 按 position 升序分页返回歌单内的歌曲。
func (r *PlaylistSongRepository) GetSongsPaginated(ctx context.Context, playlistID int64, limit, offset int) ([]*models.Song, error) {
	rows, err := r.queries.GetPlaylistSongsPaginated(ctx, sqlc.GetPlaylistSongsPaginatedParams{
		PlaylistID: playlistID,
		Limit:      int64(limit),
		Offset:     int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("get playlist songs paginated: %w", err)
	}
	songs := make([]*models.Song, 0, len(rows))
	for _, row := range rows {
		songs = append(songs, songRowToModel(row))
	}
	return songs, nil
}

// CountSongs 返回歌单内的歌曲总数。
func (r *PlaylistSongRepository) CountSongs(ctx context.Context, playlistID int64) (int, error) {
	count, err := r.queries.CountPlaylistSongs(ctx, playlistID)
	if err != nil {
		return 0, fmt.Errorf("count playlist songs: %w", err)
	}
	return int(count), nil
}

// ListPlaylistsContainingSong 返回包含给定歌曲的所有 normal 歌单 ID。
// 用于自动转换：网络歌曲缓存完成后，找到引用它的所有歌单触发本地化。
func (r *PlaylistSongRepository) ListPlaylistsContainingSong(ctx context.Context, songID int64) ([]int64, error) {
	ids, err := r.queries.ListPlaylistsContainingSong(ctx, songID)
	if err != nil {
		return nil, fmt.Errorf("list playlists containing song: %w", err)
	}
	return ids, nil
}

// ReplaceSong 用 newSongID 替换 oldSongID 并保留 position。整体走事务避免长事务锁等待。
func (r *PlaylistSongRepository) ReplaceSong(ctx context.Context, playlistID, oldSongID, newSongID int64) error {
	return r.runInTx(ctx, func(q *sqlc.Queries) error {
		return replaceSongInPlaylistTx(ctx, q, playlistID, oldSongID, newSongID)
	})
}

// BatchUpdatePositions 按给定顺序重写歌单的所有 position（1 起步）。
func (r *PlaylistSongRepository) BatchUpdatePositions(ctx context.Context, playlistID int64, songIDs []int64) error {
	return r.runInTx(ctx, func(q *sqlc.Queries) error {
		for i, songID := range songIDs {
			rows, err := q.UpdateSongPositionInPlaylist(ctx, sqlc.UpdateSongPositionInPlaylistParams{
				Position:   int64(i + 1),
				PlaylistID: playlistID,
				SongID:     songID,
			})
			if err != nil {
				return fmt.Errorf("update position for song %d: %w", songID, err)
			}
			if rows == 0 {
				return fmt.Errorf("song %d not found in playlist", songID)
			}
		}
		return nil
	})
}

// replaceSongInPlaylistTx 是 ReplaceSong 的 SQL 主体，
// 也复用给 convert_service 的外层事务（避免与外层未提交事务的写锁冲突）。
func replaceSongInPlaylistTx(ctx context.Context, q *sqlc.Queries, playlistID, oldSongID, newSongID int64) error {
	position, err := q.FindSongPositionInPlaylist(ctx, sqlc.FindSongPositionInPlaylistParams{
		PlaylistID: playlistID,
		SongID:     oldSongID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("find old song in playlist: %w", err)
	}

	rows, err := q.RemoveSongFromPlaylist(ctx, sqlc.RemoveSongFromPlaylistParams{
		PlaylistID: playlistID,
		SongID:     oldSongID,
	})
	if err != nil {
		return fmt.Errorf("remove old song from playlist: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	if err := q.AddSongToPlaylist(ctx, sqlc.AddSongToPlaylistParams{
		PlaylistID: playlistID,
		SongID:     newSongID,
		Position:   position,
	}); err != nil {
		return fmt.Errorf("insert new song into playlist: %w", err)
	}
	return nil
}

func (r *PlaylistSongRepository) runInTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	if sqlDB, ok := r.db.(*sql.DB); ok {
		tx, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer func() {
			if p := recover(); p != nil {
				_ = tx.Rollback()
				panic(p)
			}
		}()
		if err := fn(r.queries.WithTx(tx)); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	return fn(r.queries)
}
