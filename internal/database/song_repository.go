package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"

	"songloft/internal/database/sqlc"
	"songloft/internal/models"
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

// UpdateCoverURL 更新封面 URL，找不到返回 ErrNotFound。
func (r *SongRepository) UpdateCoverURL(ctx context.Context, id int64, coverURL string) error {
	rows, err := r.queries.UpdateSongCoverURL(ctx, sqlc.UpdateSongCoverURLParams{
		CoverUrl: coverURL,
		ID:       id,
	})
	if err != nil {
		return fmt.Errorf("update song cover_url %d: %w", id, err)
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

// ListTypesByIDs 批量查询给定 song id 的 type 字段，返回 id → type 映射。
// 用于歌单批量加歌前的类型兼容性预检查（避免逐首 SELECT）。
// 不存在的 id 不会出现在返回 map 中。
// 内部按 sqlBatchSize 分片查询以避免超过 SQLite 的变量数上限。
func (r *SongRepository) ListTypesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}
	result := make(map[int64]string, len(ids))
	for start := 0; start < len(ids); start += sqlBatchSize {
		end := start + sqlBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		query, args, err := sq.Select("id", "type").From("songs").Where(sq.Eq{"id": chunk}).ToSql()
		if err != nil {
			return nil, fmt.Errorf("build list song types sql: %w", err)
		}
		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("list song types: %w", err)
		}
		for rows.Next() {
			var id int64
			var typ string
			if err := rows.Scan(&id, &typ); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan song type: %w", err)
			}
			result[id] = typ
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate song types: %w", err)
		}
		rows.Close()
	}
	return result, nil
}

// LocalPathInfo 本地歌曲路径信息，用于扫描去重与不完整记录检测。
type LocalPathInfo struct {
	SongID        int64
	Duration      float64
	CueSourcePath string
}

// ListLocalPaths 返回所有本地歌曲的 file_path → LocalPathInfo 映射，用于扫描去重。
func (r *SongRepository) ListLocalPaths(ctx context.Context) (map[string]LocalPathInfo, error) {
	rows, err := r.queries.ListLocalSongPaths(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local song paths: %w", err)
	}
	paths := make(map[string]LocalPathInfo, len(rows))
	for _, row := range rows {
		if row.CueSourcePath != "" {
			continue
		}
		paths[row.FilePath] = LocalPathInfo{SongID: row.ID, Duration: row.Duration, CueSourcePath: row.CueSourcePath}
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

// FindByDedupKey 按 (plugin_entry_path, dedup_key) 查找歌曲 ID，找不到返回 ErrNotFound。
func (r *SongRepository) FindByDedupKey(ctx context.Context, pluginEntryPath, dedupKey string) (int64, error) {
	id, err := r.queries.FindSongByDedupKey(ctx, sqlc.FindSongByDedupKeyParams{
		PluginEntryPath: pluginEntryPath,
		DedupKey:        dedupKey,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("find song by dedup key: %w", err)
	}
	return id, nil
}

// UpsertRemote 按 (plugin_entry_path, dedup_key) 去重写入远程歌曲；
// 没有 dedup_key 或 plugin_entry_path 的纯外链歌曲直接 INSERT。
// 命中已存在的行时：
//   - 若 existing 已是 local（远程歌之前已被 convert_service 转为本地，但保留了 dedup 字段）：
//     仅复用 id + 回写 timestamps，不动任何字段，避免远程入参污染本地化元数据
//   - 若 existing 仍是 remote：走 UpdateRemoteSongMutable 路径刷新可变字段
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

	// 已 local：仅复用 id，不覆盖任何字段
	existing, err := r.GetByID(ctx, existingID)
	if err != nil {
		return fmt.Errorf("load existing song %d: %w", existingID, err)
	}
	if existing.Type == models.TypeLocal {
		song.ID = existingID
		song.AddedAt = existing.AddedAt
		song.UpdatedAt = existing.UpdatedAt
		return nil
	}

	// 仍是 remote：source_data / cover_url / 文本元数据始终覆盖；duration / lyric / lyric_remote_url / year / genre 仅在新值非空时更新。
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
		Column14:       int64(song.Year),
		Year:           int64(song.Year),
		Column16:       song.Genre,
		Genre:          song.Genre,
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

// ListIDs 与 List 共享过滤条件，仅返回匹配的歌曲 ID 列表（按 added_at DESC 排序，无分页）。
// 用于「全选当前筛选范围」场景：避免拉取完整 song 对象的带宽与渲染成本。
func (r *SongRepository) ListIDs(ctx context.Context, filter *SongFilter) ([]int64, error) {
	if filter == nil {
		filter = &SongFilter{}
	}
	sb := sq.Select("id").From("songs")
	sb = applySongFilter(sb, filter)
	sb = applyOrder(sb, filter.OrderBy, filter.Order, "added_at DESC", songOrderWhitelist, "")

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list song ids sql: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list song ids: %w", err)
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan song id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate song ids: %w", err)
	}
	return ids, nil
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
		"year", "genre", "language", "style",
		"fingerprint", "fingerprint_duration",
		"isrc", "track",
		"cue_source_path", "cue_track_index", "cue_audio_path",
		"file_modified_at", "is_video",
		"cue_start_seconds", "cue_end_seconds",
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
	if filter.PathPrefix != "" {
		sb = sb.Where(sq.Expr(`file_path LIKE ? ESCAPE '\'`, escapeLikeLiteral(filter.PathPrefix)+"%"))
	}
	// 标签分类精确过滤
	if filter.Genre != "" {
		sb = sb.Where(sq.Eq{"genre": filter.Genre})
	}
	if filter.Artist != "" {
		sb = sb.Where(sq.Eq{"artist": filter.Artist})
	}
	if filter.Album != "" {
		sb = sb.Where(sq.Eq{"album": filter.Album})
	}
	if filter.Language != "" {
		sb = sb.Where(sq.Eq{"language": filter.Language})
	}
	if filter.Style != "" {
		sb = sb.Where(sq.Eq{"style": filter.Style})
	}
	if filter.Year > 0 {
		sb = sb.Where(sq.Eq{"year": filter.Year})
	}
	if filter.DecadeStart > 0 {
		sb = sb.Where(sq.And{
			sq.GtOrEq{"year": filter.DecadeStart},
			sq.Lt{"year": filter.DecadeStart + 10},
		})
	}
	// 排除属于「带指定 label 的歌单」的歌曲：只要歌在任一匹配歌单里就被过滤掉。
	for _, label := range filter.ExcludePlaylistLabels {
		sb = sb.Where(`id NOT IN (
			SELECT ps.song_id FROM playlist_songs ps
			JOIN playlists p ON p.id = ps.playlist_id
			WHERE EXISTS (SELECT 1 FROM json_each(p.labels) WHERE value = ?))`, label)
	}
	return sb
}

// escapeLikeLiteral 转义 LIKE 表达式中的通配符 % _ \，用于把字符串当字面量匹配。
// 配合 SQL 中的 ESCAPE '\\' 使用。
func escapeLikeLiteral(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func scanSongRow(scanner interface {
	Scan(dest ...any) error
}) (*models.Song, error) {
	s := &models.Song{}
	var fileModifiedAt sql.NullTime
	var isVideo int64
	if err := scanner.Scan(
		&s.ID, &s.Type, &s.Title, &s.Artist, &s.Album, &s.Duration,
		&s.FilePath, &s.URL, &s.CoverPath, &s.CoverURL,
		&s.Lyric, &s.LyricSource, &s.LyricRemoteURL, &s.FileSize,
		&s.Format, &s.BitRate, &s.SampleRate, &s.IsLive,
		&s.PluginEntryPath, &s.SourceData, &s.DedupKey,
		&s.AddedAt, &s.UpdatedAt,
		&s.Year, &s.Genre, &s.Language, &s.Style,
		&s.Fingerprint, &s.FingerprintDuration,
		&s.ISRC, &s.Track,
		&s.CueSourcePath, &s.CueTrackIndex, &s.CueAudioPath,
		&fileModifiedAt, &isVideo,
		&s.CueStartSeconds, &s.CueEndSeconds,
	); err != nil {
		return nil, fmt.Errorf("scan song: %w", err)
	}
	if fileModifiedAt.Valid {
		t := fileModifiedAt.Time
		s.FileModifiedAt = &t
	}
	s.IsVideo = isVideo != 0
	return s, nil
}

func songRowToModel(row sqlc.Song) *models.Song {
	return &models.Song{
		ID:                  row.ID,
		Type:                row.Type,
		Title:               row.Title,
		Artist:              row.Artist,
		Album:               row.Album,
		Year:                int(row.Year),
		Genre:               row.Genre,
		Language:            row.Language,
		Style:               row.Style,
		Duration:            row.Duration,
		FilePath:            row.FilePath,
		CachePath:           row.CachePath,
		URL:                 row.Url,
		CoverPath:           row.CoverPath,
		CoverURL:            row.CoverUrl,
		Lyric:               row.Lyric,
		LyricSource:         row.LyricSource,
		LyricRemoteURL:      row.LyricRemoteUrl,
		FileSize:            row.FileSize,
		Format:              row.Format,
		BitRate:             int(row.BitRate),
		SampleRate:          int(row.SampleRate),
		IsLive:              row.IsLive != 0,
		IsVideo:             row.IsVideo != 0,
		PluginEntryPath:     row.PluginEntryPath,
		SourceData:          row.SourceData,
		DedupKey:            row.DedupKey,
		Fingerprint:         row.Fingerprint,
		FingerprintDuration: row.FingerprintDuration,
		ISRC:                row.Isrc,
		Track:               row.Track,
		CueSourcePath:       row.CueSourcePath,
		CueTrackIndex:       int(row.CueTrackIndex),
		CueAudioPath:        row.CueAudioPath,
		CueStartSeconds:     row.CueStartSeconds,
		CueEndSeconds:       row.CueEndSeconds,
		AddedAt:             row.AddedAt,
		UpdatedAt:           row.UpdatedAt,
		FileModifiedAt:      nullTimeToPtr(row.FileModifiedAt),
	}
}

// nullTimeToPtr 把 sql.NullTime 转成 *time.Time（无效为 nil）。
func nullTimeToPtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// nullTimeFromPtr 把 *time.Time 转成 sql.NullTime（nil 为无效）。
func nullTimeFromPtr(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func songCreateParams(s *models.Song) sqlc.CreateSongParams {
	return sqlc.CreateSongParams{
		Type:                s.Type,
		Title:               s.Title,
		Artist:              s.Artist,
		Album:               s.Album,
		Duration:            s.Duration,
		FilePath:            s.FilePath,
		Url:                 s.URL,
		CoverPath:           s.CoverPath,
		CoverUrl:            s.CoverURL,
		Lyric:               s.Lyric,
		LyricSource:         s.LyricSource,
		LyricRemoteUrl:      s.LyricRemoteURL,
		FileSize:            s.FileSize,
		Format:              s.Format,
		BitRate:             int64(s.BitRate),
		SampleRate:          int64(s.SampleRate),
		IsLive:              boolToInt64(s.IsLive),
		IsVideo:             boolToInt64(s.IsVideo),
		PluginEntryPath:     s.PluginEntryPath,
		SourceData:          s.SourceData,
		DedupKey:            s.DedupKey,
		Year:                int64(s.Year),
		Genre:               s.Genre,
		Language:            s.Language,
		Style:               s.Style,
		Fingerprint:         s.Fingerprint,
		FingerprintDuration: s.FingerprintDuration,
		Isrc:                s.ISRC,
		Track:               s.Track,
		CueSourcePath:       s.CueSourcePath,
		CueTrackIndex:       int64(s.CueTrackIndex),
		CueAudioPath:        s.CueAudioPath,
		CueStartSeconds:     s.CueStartSeconds,
		CueEndSeconds:       s.CueEndSeconds,
		FileModifiedAt:      nullTimeFromPtr(s.FileModifiedAt),
	}
}

func songUpdateParams(s *models.Song) sqlc.UpdateSongParams {
	return sqlc.UpdateSongParams{
		Type:                s.Type,
		Title:               s.Title,
		Artist:              s.Artist,
		Album:               s.Album,
		Duration:            s.Duration,
		FilePath:            s.FilePath,
		Url:                 s.URL,
		CoverPath:           s.CoverPath,
		CoverUrl:            s.CoverURL,
		Lyric:               s.Lyric,
		LyricSource:         s.LyricSource,
		LyricRemoteUrl:      s.LyricRemoteURL,
		FileSize:            s.FileSize,
		Format:              s.Format,
		BitRate:             int64(s.BitRate),
		SampleRate:          int64(s.SampleRate),
		IsLive:              boolToInt64(s.IsLive),
		IsVideo:             boolToInt64(s.IsVideo),
		PluginEntryPath:     s.PluginEntryPath,
		SourceData:          s.SourceData,
		DedupKey:            s.DedupKey,
		Year:                int64(s.Year),
		Genre:               s.Genre,
		Language:            s.Language,
		Style:               s.Style,
		Fingerprint:         s.Fingerprint,
		FingerprintDuration: s.FingerprintDuration,
		Isrc:                s.ISRC,
		Track:               s.Track,
		CueSourcePath:       s.CueSourcePath,
		CueTrackIndex:       int64(s.CueTrackIndex),
		CueAudioPath:        s.CueAudioPath,
		CueStartSeconds:     s.CueStartSeconds,
		CueEndSeconds:       s.CueEndSeconds,
		FileModifiedAt:      nullTimeFromPtr(s.FileModifiedAt),
		ID:                  s.ID,
	}
}

// ListCueSources 返回所有已存在的 CUE 来源路径集合。
func (r *SongRepository) ListCueSources(ctx context.Context) (map[string]bool, error) {
	rows, err := r.queries.ListCueSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("list cue sources: %w", err)
	}
	sources := make(map[string]bool, len(rows))
	for _, row := range rows {
		sources[row] = true
	}
	return sources, nil
}

// ListCueAudioPaths 返回某个 CUE 来源下所有引用的音频文件路径。
func (r *SongRepository) ListCueAudioPaths(ctx context.Context, cueSourcePath string) ([]string, error) {
	return r.queries.ListCueAudioPaths(ctx, cueSourcePath)
}

// DeleteByCueSource 按 cue_source_path 批量删除所有 track。
func (r *SongRepository) DeleteByCueSource(ctx context.Context, cueSourcePath string) (int, error) {
	n, err := r.queries.DeleteByCueSource(ctx, cueSourcePath)
	if err != nil {
		return 0, fmt.Errorf("delete by cue source: %w", err)
	}
	return int(n), nil
}

// UpdateFingerprint 更新歌曲的音频指纹。
func (r *SongRepository) UpdateFingerprint(ctx context.Context, id int64, fingerprint string, duration float64) error {
	return r.queries.UpdateSongFingerprint(ctx, sqlc.UpdateSongFingerprintParams{
		Fingerprint:         fingerprint,
		FingerprintDuration: duration,
		ID:                  id,
	})
}

// ClearAllFingerprints 清空所有本地歌曲的指纹数据。
func (r *SongRepository) ClearAllFingerprints(ctx context.Context) error {
	return r.queries.ClearAllFingerprints(ctx)
}

// SongIDPath 是 (id, file_path) 的轻量对。
type SongIDPath struct {
	ID       int64
	FilePath string
}

// ListLocalWithoutFingerprint 返回所有尚无指纹的本地歌曲 (id, file_path)。
func (r *SongRepository) ListLocalWithoutFingerprint(ctx context.Context) ([]SongIDPath, error) {
	rows, err := r.queries.ListLocalWithoutFingerprint(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local without fingerprint: %w", err)
	}
	result := make([]SongIDPath, len(rows))
	for i, row := range rows {
		result[i] = SongIDPath{ID: row.ID, FilePath: row.FilePath}
	}
	return result, nil
}

// CountLocalFingerprints 返回本地歌曲总数和已计算指纹数。
func (r *SongRepository) CountLocalFingerprints(ctx context.Context) (total, computed int64, err error) {
	row, err := r.queries.CountLocalFingerprints(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("count local fingerprints: %w", err)
	}
	return row.Total, row.Computed, nil
}

// DuplicateGroup 表示一组指纹相同的歌曲。
type DuplicateGroup struct {
	Fingerprint string
	Songs       []*models.Song
}

// ListDuplicateGroups 查询所有指纹重复的本地歌曲组。
func (r *SongRepository) ListDuplicateGroups(ctx context.Context) ([]DuplicateGroup, error) {
	fps, err := r.queries.ListDuplicateFingerprints(ctx)
	if err != nil {
		return nil, fmt.Errorf("list duplicate fingerprints: %w", err)
	}
	if len(fps) == 0 {
		return nil, nil
	}

	groups := make([]DuplicateGroup, 0, len(fps))
	for _, fp := range fps {
		rows, err := r.queries.ListSongsByFingerprint(ctx, fp.Fingerprint)
		if err != nil {
			return nil, fmt.Errorf("list songs by fingerprint: %w", err)
		}
		songs := make([]*models.Song, len(rows))
		for i, row := range rows {
			songs[i] = &models.Song{
				ID:                  row.ID,
				Type:                row.Type,
				Title:               row.Title,
				Artist:              row.Artist,
				Album:               row.Album,
				Duration:            row.Duration,
				FilePath:            row.FilePath,
				Format:              row.Format,
				BitRate:             int(row.BitRate),
				SampleRate:          int(row.SampleRate),
				FileSize:            row.FileSize,
				FingerprintDuration: row.FingerprintDuration,
				CoverPath:           row.CoverPath,
				CoverURL:            row.CoverUrl,
				AddedAt:             row.AddedAt,
			}
		}
		groups = append(groups, DuplicateGroup{Fingerprint: fp.Fingerprint, Songs: songs})
	}
	return groups, nil
}

// Facet 标签分类的一个取值及其歌曲数量（如 genre="Rock", count=42）。
// CoverURL 为该取值下一首带封面歌曲的封面端点（无则为空，前端回退占位图标）。
type Facet struct {
	Value    string `json:"value"`
	Count    int64  `json:"count"`
	CoverURL string `json:"cover_url"`
}

// ListFacet 按维度聚合曲库标签，返回该维度下的取值 + 计数 + 代表封面（支持搜索/排序/分页）。
// field 支持：genre / artist / album / language / style / year / decade。
// 未知 field 返回 ErrNotFound，交由 handler 转 400。
//
// 说明：这是本项目首个用 squirrel 编写的聚合（GROUP BY）查询——因为 facet 需要
// 可选 keyword（变长 WHERE）+ 动态排序 + 分页，sqlc 固定查询无法表达，按铁律「动态 SQL→squirrel」实现。
func (r *SongRepository) ListFacet(ctx context.Context, field string, f *FacetFilter) ([]Facet, error) {
	col, ok := songFacetColumn[field]
	if !ok {
		return nil, ErrNotFound
	}

	// CAST(... AS TEXT) 让文本/数字维度都统一以字符串取回，year/decade 得到如 "1990"。
	// cover_song_id 取组内任意一首「有封面」（本地 cover_path 或远程 cover_url 非空）的歌曲。
	sb := sq.Select(
		"CAST("+col+" AS TEXT) AS value",
		"COUNT(*) AS count",
		"MAX(CASE WHEN cover_path != '' OR cover_url != '' THEN id END) AS cover_song_id",
	).From("songs").Where(facetBaseCond(field, col))

	if f != nil && f.Keyword != "" {
		sb = sb.Where(sq.Expr("CAST("+col+" AS TEXT) LIKE ?", "%"+f.Keyword+"%"))
	}
	sb = sb.GroupBy(col)
	sb = applyFacetOrder(sb, f)
	if f != nil {
		sb = applyPagination(sb, f.Limit, f.Offset)
	}

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build facet %s sql: %w", field, err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("facet %s: %w", field, err)
	}
	defer rows.Close()

	out := []Facet{}
	for rows.Next() {
		var value string
		var count int64
		var coverID sql.NullInt64
		if err := rows.Scan(&value, &count, &coverID); err != nil {
			return nil, fmt.Errorf("scan facet %s: %w", field, err)
		}
		facet := Facet{Value: value, Count: count}
		if coverID.Valid && coverID.Int64 > 0 {
			facet.CoverURL = fmt.Sprintf("/api/v1/songs/%d/cover", coverID.Int64)
		}
		out = append(out, facet)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate facet %s: %w", field, err)
	}
	return out, nil
}

// CountFacet 返回某维度去重取值的总数（用于前端分页判断），与 ListFacet 共享 keyword 过滤。
func (r *SongRepository) CountFacet(ctx context.Context, field, keyword string) (int64, error) {
	col, ok := songFacetColumn[field]
	if !ok {
		return 0, ErrNotFound
	}
	inner := sq.Select("1").From("songs").Where(facetBaseCond(field, col))
	if keyword != "" {
		inner = inner.Where(sq.Expr("CAST("+col+" AS TEXT) LIKE ?", "%"+keyword+"%"))
	}
	inner = inner.GroupBy(col)

	innerSQL, innerArgs, err := inner.ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count facet %s sql: %w", field, err)
	}
	var n int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+innerSQL+")", innerArgs...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count facet %s: %w", field, err)
	}
	return n, nil
}

// UpdateCachePath 更新歌曲的缓存文件路径。
func (r *SongRepository) UpdateCachePath(ctx context.Context, id int64, cachePath string) error {
	return r.queries.UpdateCachePath(ctx, sqlc.UpdateCachePathParams{
		CachePath: cachePath,
		ID:        id,
	})
}

// ClearCachePath 清除歌曲的缓存文件路径。
func (r *SongRepository) ClearCachePath(ctx context.Context, id int64) error {
	return r.queries.ClearCachePath(ctx, id)
}

// ClearAllCachePaths 清除所有歌曲的缓存文件路径。
func (r *SongRepository) ClearAllCachePaths(ctx context.Context) error {
	return r.queries.ClearAllCachePaths(ctx)
}

// ListSongsWithCache 列出所有有缓存文件的歌曲。
func (r *SongRepository) ListSongsWithCache(ctx context.Context) ([]*models.Song, error) {
	rows, err := r.queries.ListSongsWithCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("list songs with cache: %w", err)
	}
	songs := make([]*models.Song, len(rows))
	for i, row := range rows {
		songs[i] = songRowToModel(row)
	}
	return songs, nil
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// ListSongsNeedingMetadata 返回所有元数据缺失的远程歌曲（用于批量元数据探测）。
func (r *SongRepository) ListSongsNeedingMetadata(ctx context.Context) ([]sqlc.ListSongsNeedingMetadataRow, error) {
	return r.queries.ListSongsNeedingMetadata(ctx)
}

// UpdateMetadata 条件更新远程歌曲的多个元数据字段（仅在原值为空时填充）。
func (r *SongRepository) UpdateMetadata(ctx context.Context, params sqlc.UpdateSongMetadataParams) error {
	return r.queries.UpdateSongMetadata(ctx, params)
}

// UpdateTagFields 用 tag 值覆盖 artist/album（播放时自动提取用）。
func (r *SongRepository) UpdateTagFields(ctx context.Context, params sqlc.UpdateSongTagFieldsParams) error {
	return r.queries.UpdateSongTagFields(ctx, params)
}
