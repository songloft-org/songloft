package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"songloft/internal/database/sqlc"
	"songloft/internal/models"
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

// GetSongsFiltered 按过滤条件分页返回歌单内的歌曲（支持搜索和排序）。
func (r *PlaylistSongRepository) GetSongsFiltered(ctx context.Context, playlistID int64, filter PlaylistSongFilter) ([]*models.Song, error) {
	sb := sq.Select(
		"s.id", "s.type", "s.title", "s.artist", "s.album", "s.duration",
		"s.file_path", "s.url", "s.cover_path", "s.cover_url",
		"s.lyric", "s.lyric_source", "s.lyric_remote_url", "s.file_size",
		"s.format", "s.bit_rate", "s.sample_rate", "s.is_live",
		"COALESCE(s.plugin_entry_path, '')",
		"COALESCE(s.source_data, '')",
		"COALESCE(s.dedup_key, '')",
		"s.added_at", "s.updated_at",
		"s.year", "s.genre", "s.language", "s.style",
		"s.fingerprint", "s.fingerprint_duration",
		"s.isrc", "s.track",
		"s.cue_source_path", "s.cue_track_index", "s.cue_audio_path",
		"s.file_modified_at", "s.is_video",
		"s.cue_start_seconds", "s.cue_end_seconds",
	).From("songs s").
		InnerJoin("playlist_songs ps ON s.id = ps.song_id").
		Where(sq.Eq{"ps.playlist_id": playlistID})

	if filter.Keyword != "" {
		kw := "%" + filter.Keyword + "%"
		sb = sb.Where(sq.Or{
			sq.Like{"s.title": kw},
			sq.Like{"s.artist": kw},
			sq.Like{"s.album": kw},
		})
	}

	sb = applyPlaylistSongOrder(sb, filter.OrderBy, filter.Order)
	sb = applyPagination(sb, filter.Limit, filter.Offset)

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build playlist songs sql: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get playlist songs filtered: %w", err)
	}
	defer rows.Close()

	songs := []*models.Song{}
	for rows.Next() {
		s, err := scanSongRow(rows)
		if err != nil {
			return nil, err
		}
		songs = append(songs, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate playlist songs: %w", err)
	}
	return songs, nil
}

// CountSongsFiltered 返回歌单内满足过滤条件的歌曲总数。
func (r *PlaylistSongRepository) CountSongsFiltered(ctx context.Context, playlistID int64, keyword string) (int, error) {
	sb := sq.Select("COUNT(*)").
		From("playlist_songs ps").
		InnerJoin("songs s ON s.id = ps.song_id").
		Where(sq.Eq{"ps.playlist_id": playlistID})

	if keyword != "" {
		kw := "%" + keyword + "%"
		sb = sb.Where(sq.Or{
			sq.Like{"s.title": kw},
			sq.Like{"s.artist": kw},
			sq.Like{"s.album": kw},
		})
	}

	query, args, err := sb.ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count playlist songs sql: %w", err)
	}
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count playlist songs filtered: %w", err)
	}
	return count, nil
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

// MaxPosition 返回歌单当前最大 position；歌单为空时返回 0。
// 用于批量追加歌曲前一次性计算起始位置，避免逐首拉全表。
func (r *PlaylistSongRepository) MaxPosition(ctx context.Context, playlistID int64) (int, error) {
	n, err := r.queries.MaxPositionInPlaylist(ctx, playlistID)
	if err != nil {
		return 0, fmt.Errorf("max position in playlist %d: %w", playlistID, err)
	}
	return int(n), nil
}

// AddSongsBatch 在单一事务里把多首歌曲连续追加到歌单末尾，已存在的静默跳过。
// position 从 startPos+1 开始累加；只有实际插入成功的行才前进 position。
// 返回 (added, skipped, err)。
func (r *PlaylistSongRepository) AddSongsBatch(ctx context.Context, playlistID int64, startPos int, songIDs []int64) (added int, skipped int, err error) {
	if len(songIDs) == 0 {
		return 0, 0, nil
	}
	err = r.runInTx(ctx, func(q *sqlc.Queries) error {
		pos := startPos
		for _, songID := range songIDs {
			pos++
			rows, ierr := q.AddSongToPlaylistIgnore(ctx, sqlc.AddSongToPlaylistIgnoreParams{
				PlaylistID: playlistID,
				SongID:     songID,
				Position:   int64(pos),
			})
			if ierr != nil {
				return fmt.Errorf("insert song %d into playlist %d: %w", songID, playlistID, ierr)
			}
			if rows > 0 {
				added++
			} else {
				pos--
				skipped++
			}
		}
		return nil
	})
	return added, skipped, err
}

// AddSongIgnore 把歌曲添加到歌单，已存在时静默跳过。
// 返回是否实际插入（true=新增，false=已存在被忽略）。
func (r *PlaylistSongRepository) AddSongIgnore(ctx context.Context, playlistID, songID int64, position int) (bool, error) {
	rows, err := r.queries.AddSongToPlaylistIgnore(ctx, sqlc.AddSongToPlaylistIgnoreParams{
		PlaylistID: playlistID,
		SongID:     songID,
		Position:   int64(position),
	})
	if err != nil {
		return false, fmt.Errorf("add song ignore: %w", err)
	}
	return rows > 0, nil
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
