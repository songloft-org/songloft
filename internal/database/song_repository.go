package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"

	"mimusic/internal/database/sqlc"
	"mimusic/internal/models"
)

// SongRepository 歌曲仓储。
// 固定 SQL 走 sqlc.Queries，动态过滤的 List/Count/BatchDelete 走 squirrel；
// 多语句批量操作（BatchCreate）会在底层是 *sql.DB 时自启动事务。
type SongRepository struct {
	db      sqlc.DBTX
	queries *sqlc.Queries
}

// NewSongRepository 用 *sql.DB 或 *sql.Tx 构造仓储。
func NewSongRepository(db sqlc.DBTX) *SongRepository {
	return &SongRepository{db: db, queries: sqlc.New(db)}
}

// GetByID 根据 ID 获取歌曲，找不到返回 ErrNotFound。
func (r *SongRepository) GetByID(ctx context.Context, id int64) (*models.Song, error) {
	row, err := r.queries.GetSongByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get song by id %d: %w", id, err)
	}
	return songRowToModel(row), nil
}

// Create 插入一行 song 并回填 ID/AddedAt/UpdatedAt。
func (r *SongRepository) Create(ctx context.Context, song *models.Song) error {
	id, err := r.queries.CreateSong(ctx, songCreateParams(song))
	if err != nil {
		return fmt.Errorf("insert song %q: %w", song.Title, err)
	}
	song.ID = id
	now := time.Now()
	song.AddedAt = now
	song.UpdatedAt = now
	return nil
}

// Update 更新全部可写字段，找不到返回 ErrNotFound。
func (r *SongRepository) Update(ctx context.Context, song *models.Song) error {
	rows, err := r.queries.UpdateSong(ctx, songUpdateParams(song))
	if err != nil {
		return fmt.Errorf("update song %d: %w", song.ID, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete 删除歌曲，找不到返回 ErrNotFound。playlist_songs 由 FK ON DELETE CASCADE 自动清理。
func (r *SongRepository) Delete(ctx context.Context, id int64) error {
	rows, err := r.queries.DeleteSong(ctx, id)
	if err != nil {
		return fmt.Errorf("delete song %d: %w", id, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLyrics 更新歌词字段，找不到返回 ErrNotFound。
// lyric 是 LyricPayload JSON 文本(或空);lyricRemoteURL 仅在 lyricSource="url" 时非空。
func (r *SongRepository) UpdateLyrics(ctx context.Context, id int64, lyric, lyricSource, lyricRemoteURL string) error {
	rows, err := r.queries.UpdateSongLyrics(ctx, sqlc.UpdateSongLyricsParams{
		Lyric:          lyric,
		LyricSource:    lyricSource,
		LyricRemoteUrl: lyricRemoteURL,
		ID:             id,
	})
	if err != nil {
		return fmt.Errorf("update song lyrics %d: %w", id, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateDuration 仅在原 duration 为 0 时回填时长。
func (r *SongRepository) UpdateDuration(ctx context.Context, id int64, duration float64) error {
	if err := r.queries.UpdateSongDuration(ctx, sqlc.UpdateSongDurationParams{
		Duration: duration,
		ID:       id,
	}); err != nil {
		return fmt.Errorf("update song duration %d: %w", id, err)
	}
	return nil
}

// UpdateSource 仅更新 plugin_entry_path 与 source_data。
func (r *SongRepository) UpdateSource(ctx context.Context, id int64, pluginEntryPath, sourceData string) error {
	if err := r.queries.UpdateSongSource(ctx, sqlc.UpdateSongSourceParams{
		PluginEntryPath: pluginEntryPath,
		SourceData:      sourceData,
		ID:              id,
	}); err != nil {
		return fmt.Errorf("update song source %d: %w", id, err)
	}
	return nil
}

// ListLocalPaths 返回所有本地歌曲的 file_path → id 映射，用于扫描去重。
func (r *SongRepository) ListLocalPaths(ctx context.Context) (map[string]int64, error) {
	rows, err := r.queries.ListLocalSongPaths(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local song paths: %w", err)
	}
	paths := make(map[string]int64, len(rows))
	for _, row := range rows {
		paths[row.FilePath] = row.ID
	}
	return paths, nil
}

// CountPlaylistsContaining 统计某首歌曲被多少个歌单引用。
func (r *SongRepository) CountPlaylistsContaining(ctx context.Context, songID int64) (int, error) {
	n, err := r.queries.CountPlaylistsContainingSong(ctx, songID)
	if err != nil {
		return 0, fmt.Errorf("count playlists containing song %d: %w", songID, err)
	}
	return int(n), nil
}

// CountCoverPathReferences 统计 songs + playlists 两表中等于该 cover_path 的总行数。
// 封面按内容哈希共享存储，调用方应在物理删除前确认为 0。
func (r *SongRepository) CountCoverPathReferences(ctx context.Context, coverPath string) (int, error) {
	if coverPath == "" {
		return 0, nil
	}
	songs, err := r.queries.CountSongsByCoverPath(ctx, coverPath)
	if err != nil {
		return 0, fmt.Errorf("count songs by cover_path: %w", err)
	}
	playlists, err := r.queries.CountPlaylistsByCoverPath(ctx, coverPath)
	if err != nil {
		return 0, fmt.Errorf("count playlists by cover_path: %w", err)
	}
	return int(songs + playlists), nil
}

// UpsertRemote 按 (plugin_entry_path, dedup_key) 去重写入远程歌曲；
// 没有 dedup_key 或 plugin_entry_path 的纯外链歌曲直接 INSERT。
func (r *SongRepository) UpsertRemote(ctx context.Context, song *models.Song) error {
	if song.PluginEntryPath == "" || song.DedupKey == "" {
		return r.Create(ctx, song)
	}

	existingID, err := r.queries.FindSongByDedupKey(ctx, sqlc.FindSongByDedupKeyParams{
		PluginEntryPath: song.PluginEntryPath,
		DedupKey:        song.DedupKey,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return r.Create(ctx, song)
		}
		return fmt.Errorf("find song by dedup_key: %w", err)
	}

	// 已存在：source_data / cover_url / 文本元数据始终覆盖；duration / lyric / lyric_remote_url 仅在新值非空时更新。
	if err := r.queries.UpdateRemoteSongMutable(ctx, sqlc.UpdateRemoteSongMutableParams{
		Title:          song.Title,
		Artist:         song.Artist,
		Album:          song.Album,
		CoverUrl:       song.CoverURL,
		SourceData:     song.SourceData,
		Column6:        song.Duration,
		Duration:       song.Duration,
		Column8:        song.Lyric,
		Lyric:          song.Lyric,
		Column10:       song.LyricSource,
		LyricSource:    song.LyricSource,
		Column12:       song.LyricRemoteURL,
		LyricRemoteUrl: song.LyricRemoteURL,
		ID:             existingID,
	}); err != nil {
		return fmt.Errorf("update remote song %q: %w", song.Title, err)
	}

	song.ID = existingID
	ts, err := r.queries.GetSongTimestamps(ctx, existingID)
	if err != nil {
		return fmt.Errorf("get song timestamps %d: %w", existingID, err)
	}
	song.AddedAt = ts.AddedAt
	song.UpdatedAt = ts.UpdatedAt
	return nil
}

// List 按过滤条件 + 白名单排序 + 分页拉取歌曲。
func (r *SongRepository) List(ctx context.Context, filter *SongFilter) ([]*models.Song, error) {
	if filter == nil {
		filter = &SongFilter{}
	}
	sb := songSelectBuilder()
	sb = applySongFilter(sb, filter)
	sb = applyOrder(sb, filter.OrderBy, filter.Order, "added_at DESC", songOrderWhitelist, "")
	sb = applyPagination(sb, filter.Limit, filter.Offset)

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list songs sql: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list songs: %w", err)
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
		return nil, fmt.Errorf("iterate songs: %w", err)
	}
	return songs, nil
}

// Count 与 List 共享过滤条件，返回匹配行数。
func (r *SongRepository) Count(ctx context.Context, filter *SongFilter) (int64, error) {
	if filter == nil {
		filter = &SongFilter{}
	}
	sb := sq.Select("COUNT(*)").From("songs")
	sb = applySongFilter(sb, filter)

	query, args, err := sb.ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count songs sql: %w", err)
	}
	var n int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count songs: %w", err)
	}
	return n, nil
}

// BatchDelete 批量删除歌曲，返回实际删除条数。playlist_songs 由 FK CASCADE 清理。
func (r *SongRepository) BatchDelete(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	query, args, err := sq.Delete("songs").Where(sq.Eq{"id": ids}).ToSql()
	if err != nil {
		return 0, fmt.Errorf("build batch delete songs sql: %w", err)
	}
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("batch delete songs: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return int(affected), nil
}

// BatchCreate 批量插入歌曲。若底层是 *sql.DB 则自启动事务，
// 若已在调用方的事务里则直接顺序 INSERT（不再嵌套事务）。
func (r *SongRepository) BatchCreate(ctx context.Context, songs []*models.Song) error {
	if len(songs) == 0 {
		return nil
	}
	if sqlDB, ok := r.db.(*sql.DB); ok {
		tx, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for batch insert songs: %w", err)
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()
		qtx := r.queries.WithTx(tx)
		if err := batchCreateSongs(ctx, qtx, songs); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit batch insert songs: %w", err)
		}
		committed = true
		return nil
	}
	return batchCreateSongs(ctx, r.queries, songs)
}

func batchCreateSongs(ctx context.Context, q *sqlc.Queries, songs []*models.Song) error {
	now := time.Now()
	for _, song := range songs {
		id, err := q.CreateSong(ctx, songCreateParams(song))
		if err != nil {
			return fmt.Errorf("insert song %q: %w", song.Title, err)
		}
		song.ID = id
		song.AddedAt = now
		song.UpdatedAt = now
	}
	return nil
}

// songSelectBuilder 提供 List 用的 squirrel SELECT 模板，
// COALESCE 列与 sqlc 行模型保持一致。
func songSelectBuilder() sq.SelectBuilder {
	return sq.Select(
		"id", "type", "title", "artist", "album", "duration",
		"file_path", "url", "cover_path", "cover_url",
		"lyric", "lyric_source", "lyric_remote_url", "file_size",
		"format", "bit_rate", "sample_rate", "is_live",
		"COALESCE(plugin_entry_path, '')",
		"COALESCE(source_data, '')",
		"COALESCE(dedup_key, '')",
		"added_at", "updated_at",
	).From("songs")
}

func applySongFilter(sb sq.SelectBuilder, filter *SongFilter) sq.SelectBuilder {
	if filter.Type != "" {
		sb = sb.Where(sq.Eq{"type": filter.Type})
	}
	if filter.Keyword != "" {
		kw := "%" + filter.Keyword + "%"
		sb = sb.Where(sq.Or{
			sq.Like{"title": kw},
			sq.Like{"artist": kw},
			sq.Like{"album": kw},
		})
	}
	return sb
}

func scanSongRow(scanner interface {
	Scan(dest ...any) error
}) (*models.Song, error) {
	s := &models.Song{}
	if err := scanner.Scan(
		&s.ID, &s.Type, &s.Title, &s.Artist, &s.Album, &s.Duration,
		&s.FilePath, &s.URL, &s.CoverPath, &s.CoverURL,
		&s.Lyric, &s.LyricSource, &s.LyricRemoteURL, &s.FileSize,
		&s.Format, &s.BitRate, &s.SampleRate, &s.IsLive,
		&s.PluginEntryPath, &s.SourceData, &s.DedupKey,
		&s.AddedAt, &s.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan song: %w", err)
	}
	return s, nil
}

func songRowToModel(row sqlc.Song) *models.Song {
	return &models.Song{
		ID:              row.ID,
		Type:            row.Type,
		Title:           row.Title,
		Artist:          row.Artist,
		Album:           row.Album,
		Duration:        row.Duration,
		FilePath:        row.FilePath,
		URL:             row.Url,
		CoverPath:       row.CoverPath,
		CoverURL:        row.CoverUrl,
		Lyric:           row.Lyric,
		LyricSource:     row.LyricSource,
		LyricRemoteURL:  row.LyricRemoteUrl,
		FileSize:        row.FileSize,
		Format:          row.Format,
		BitRate:         int(row.BitRate),
		SampleRate:      int(row.SampleRate),
		IsLive:          row.IsLive != 0,
		PluginEntryPath: row.PluginEntryPath,
		SourceData:      row.SourceData,
		DedupKey:        row.DedupKey,
		AddedAt:         row.AddedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func songCreateParams(s *models.Song) sqlc.CreateSongParams {
	return sqlc.CreateSongParams{
		Type:            s.Type,
		Title:           s.Title,
		Artist:          s.Artist,
		Album:           s.Album,
		Duration:        s.Duration,
		FilePath:        s.FilePath,
		Url:             s.URL,
		CoverPath:       s.CoverPath,
		CoverUrl:        s.CoverURL,
		Lyric:           s.Lyric,
		LyricSource:     s.LyricSource,
		LyricRemoteUrl:  s.LyricRemoteURL,
		FileSize:        s.FileSize,
		Format:          s.Format,
		BitRate:         int64(s.BitRate),
		SampleRate:      int64(s.SampleRate),
		IsLive:          boolToInt64(s.IsLive),
		PluginEntryPath: s.PluginEntryPath,
		SourceData:      s.SourceData,
		DedupKey:        s.DedupKey,
	}
}

func songUpdateParams(s *models.Song) sqlc.UpdateSongParams {
	return sqlc.UpdateSongParams{
		Type:            s.Type,
		Title:           s.Title,
		Artist:          s.Artist,
		Album:           s.Album,
		Duration:        s.Duration,
		FilePath:        s.FilePath,
		Url:             s.URL,
		CoverPath:       s.CoverPath,
		CoverUrl:        s.CoverURL,
		Lyric:           s.Lyric,
		LyricSource:     s.LyricSource,
		LyricRemoteUrl:  s.LyricRemoteURL,
		FileSize:        s.FileSize,
		Format:          s.Format,
		BitRate:         int64(s.BitRate),
		SampleRate:      int64(s.SampleRate),
		IsLive:          boolToInt64(s.IsLive),
		PluginEntryPath: s.PluginEntryPath,
		SourceData:      s.SourceData,
		DedupKey:        s.DedupKey,
		ID:              s.ID,
	}
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
