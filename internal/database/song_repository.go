package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
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

// FilterOrphanSongIDs 从 ids 中筛出"孤儿"歌曲——即在 playlist_songs 里没有任何关联行的歌曲。
// 供删除歌单时清理"仅属于被删歌单"的残留歌曲用（歌单删除后其 playlist_songs 已被 FK CASCADE 清空，
// 故此处判定 NOT EXISTS 即等价于"不属于任何其他歌单"）。按 sqlBatchSize 分片规避 SQLite 变量上限。
func (r *SongRepository) FilterOrphanSongIDs(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	orphans := make([]int64, 0, len(ids))
	for start := 0; start < len(ids); start += sqlBatchSize {
		end := start + sqlBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		query, args, err := sq.Select("s.id").From("songs s").
			Where(sq.Eq{"s.id": chunk}).
			Where("NOT EXISTS (SELECT 1 FROM playlist_songs ps WHERE ps.song_id = s.id)").
			ToSql()
		if err != nil {
			return nil, fmt.Errorf("build filter orphan song ids sql: %w", err)
		}
		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("filter orphan song ids: %w", err)
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan orphan song id: %w", err)
			}
			orphans = append(orphans, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate orphan song ids: %w", err)
		}
		rows.Close()
	}
	return orphans, nil
}

// LocalPathInfo 本地歌曲路径信息，用于扫描去重与不完整记录检测。
type LocalPathInfo struct {
	SongID         int64
	Duration       float64
	CueSourcePath  string
	LyricSource    string
	FileModifiedAt *time.Time
}

// RelativePathSong 用于路径规范化清理。
type RelativePathSong struct {
	ID       int64
	FilePath string
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
		info := LocalPathInfo{SongID: row.ID, Duration: row.Duration, CueSourcePath: row.CueSourcePath, LyricSource: row.LyricSource}
		if row.FileModifiedAt.Valid {
			t := row.FileModifiedAt.Time
			info.FileModifiedAt = &t
		}
		paths[row.FilePath] = info
	}
	return paths, nil
}

// ListLocalSongsWithRelativePaths 返回所有 file_path 非绝对路径的本地歌曲。
func (r *SongRepository) ListLocalSongsWithRelativePaths(ctx context.Context) ([]RelativePathSong, error) {
	rows, err := r.queries.ListLocalSongsWithRelativePaths(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local songs with relative paths: %w", err)
	}
	result := make([]RelativePathSong, len(rows))
	for i, row := range rows {
		result[i] = RelativePathSong{ID: row.ID, FilePath: row.FilePath}
	}
	return result, nil
}

// UpdateSongFilePath 仅更新歌曲的 file_path 字段。
func (r *SongRepository) UpdateSongFilePath(ctx context.Context, id int64, newPath string) error {
	rows, err := r.queries.UpdateSongFilePath(ctx, sqlc.UpdateSongFilePathParams{
		FilePath: newPath,
		ID:       id,
	})
	if err != nil {
		return fmt.Errorf("update song file_path %d: %w", id, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
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

// CountSongsByFilePath 统计有多少 song 行引用了同一 file_path。
// 删除本地歌曲的物理文件前用它做引用计数：多条歌曲行指向同一文件时（异常但可能，
// 如手动重复导入 / 插件路径模板碰撞），只删 DB 行不删文件，避免误删仍被引用的音频。
// file_path 为空直接返回 0（远程 / 电台歌曲无本地文件）。
func (r *SongRepository) CountSongsByFilePath(ctx context.Context, filePath string) (int, error) {
	if filePath == "" {
		return 0, nil
	}
	n, err := r.queries.CountSongsByFilePath(ctx, filePath)
	if err != nil {
		return 0, fmt.Errorf("count songs by file_path: %w", err)
	}
	return int(n), nil
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

// ListRandom 与 List 共享过滤条件，随机返回 limit 首歌曲（完整对象）。
// 使用 SQLite 的 ORDER BY RANDOM() 实现；limit <= 0 时默认返回 50 首。
func (r *SongRepository) ListRandom(ctx context.Context, filter *SongFilter) ([]*models.Song, error) {
	if filter == nil {
		filter = &SongFilter{}
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	sb := songSelectBuilder()
	sb = applySongFilter(sb, filter)
	sb = sb.OrderBy("RANDOM()").Limit(uint64(limit))

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list random songs sql: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list random songs: %w", err)
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
		return nil, fmt.Errorf("iterate random songs: %w", err)
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

// ListByIDs 按主键批量拉取歌曲，返回顺序由 SQLite 决定（调用方需要特定顺序时自行重排）。
// ids 为空时返回空切片而不查库。
func (r *SongRepository) ListByIDs(ctx context.Context, ids []int64) ([]*models.Song, error) {
	if len(ids) == 0 {
		return []*models.Song{}, nil
	}
	sb := songSelectBuilder().Where(sq.Eq{"id": ids})
	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list songs by ids sql: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list songs by ids: %w", err)
	}
	defer rows.Close()

	songs := make([]*models.Song, 0, len(ids))
	for rows.Next() {
		s, err := scanSongRow(rows)
		if err != nil {
			return nil, err
		}
		songs = append(songs, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate songs by ids: %w", err)
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
			sq.Expr("id IN (SELECT l.song_id FROM song_tag_links l JOIN song_tags t ON l.tag_id = t.id WHERE t.name LIKE ?)", kw),
			// 多值歌手：关键词命中任一参与歌手名（song_artists 关联表），使对唱歌曲也能搜到。
			sq.Expr("id IN (SELECT sa.song_id FROM song_artists sa JOIN artists a ON a.id = sa.artist_id WHERE a.name LIKE ?)", kw),
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
		// 按歌手过滤走 song_artists 关联表（归一键匹配），命中该歌的任一参与歌手，
		// 而非 songs.artist 整串精确匹配——对唱歌曲因此可被任一搭档检索到。
		key := NormalizeArtistKey(filter.Artist)
		sb = sb.Where("id IN (SELECT sa.song_id FROM song_artists sa JOIN artists a ON a.id = sa.artist_id WHERE a.normalized_key = ? AND sa.role = 'artist')", key)
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
	// 按自定义标签 ID 过滤
	if filter.TagID > 0 {
		sb = sb.Where("id IN (SELECT song_id FROM song_tag_links WHERE tag_id = ?)", filter.TagID)
	}
	// 按自定义标签名模糊匹配
	if filter.TagName != "" {
		sb = sb.Where(`id IN (SELECT l.song_id FROM song_tag_links l JOIN song_tags t ON l.tag_id = t.id WHERE t.name LIKE ?)`, "%"+filter.TagName+"%")
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
}, extra ...any) (*models.Song, error) {
	s := &models.Song{}
	var fileModifiedAt sql.NullTime
	var isVideo int64
	dest := []any{
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
	}
	if err := scanner.Scan(append(dest, extra...)...); err != nil {
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

// UpdateFingerprint 更新歌曲的音频指纹，并把 attemptedAt 记为已尝试时间戳（unix 秒）。
func (r *SongRepository) UpdateFingerprint(ctx context.Context, id int64, fingerprint string, duration float64, attemptedAt int64) error {
	return r.queries.UpdateSongFingerprint(ctx, sqlc.UpdateSongFingerprintParams{
		Fingerprint:            fingerprint,
		FingerprintDuration:    duration,
		FingerprintAttemptedAt: attemptedAt,
		ID:                     id,
	})
}

// MarkFingerprintAttempted 记录「已尝试计算指纹」的时间戳和失败原因。
// 用于计算失败（超时 / 无音轨 / 文件损坏）的场景：没有这个标记，
// 每轮扫描都会把同一批注定失败的文件重新捞出来跑 ffmpeg 全解码（songloft-org/songloft#323）。
func (r *SongRepository) MarkFingerprintAttempted(ctx context.Context, id int64, attemptedAt int64, errMsg string) error {
	return r.queries.MarkFingerprintAttempted(ctx, sqlc.MarkFingerprintAttemptedParams{
		FingerprintAttemptedAt: attemptedAt,
		FingerprintError:       errMsg,
		ID:                     id,
	})
}

// FailedFingerprintRow 指纹计算失败的歌曲信息。
type FailedFingerprintRow struct {
	ID                     int64
	Title                  string
	Artist                 string
	FilePath               string
	FingerprintError       string
	FingerprintAttemptedAt int64
}

// ListFailedFingerprints 返回所有指纹计算失败的本地歌曲。
func (r *SongRepository) ListFailedFingerprints(ctx context.Context) ([]FailedFingerprintRow, error) {
	rows, err := r.queries.ListFailedFingerprints(ctx)
	if err != nil {
		return nil, fmt.Errorf("list failed fingerprints: %w", err)
	}
	result := make([]FailedFingerprintRow, len(rows))
	for i, row := range rows {
		result[i] = FailedFingerprintRow{
			ID:                     row.ID,
			Title:                  row.Title,
			Artist:                 row.Artist,
			FilePath:               row.FilePath,
			FingerprintError:       row.FingerprintError,
			FingerprintAttemptedAt: row.FingerprintAttemptedAt,
		}
	}
	return result, nil
}

// ClearAllFingerprints 清空所有本地歌曲的指纹数据，并重置「已尝试」标记，
// 让此前失败的歌曲在「重新计算全部」时能被重试。
func (r *SongRepository) ClearAllFingerprints(ctx context.Context) error {
	return r.queries.ClearAllFingerprints(ctx)
}

// ResetFailedFingerprintAttempts 仅重置失败项（无指纹且已标记尝试）的「已尝试」标记，
// 已算好的指纹不受影响。用于 ffmpeg 能力升级（如新增 mpeg 解复用器）后
// 单独重试此前因解码器缺失而失败的歌曲，避免为此清空全库重算。
func (r *SongRepository) ResetFailedFingerprintAttempts(ctx context.Context) error {
	return r.queries.ResetFailedFingerprintAttempts(ctx)
}

// SongIDPath 是指纹计算所需的歌曲最小信息。
// CueStartSeconds / CueEndSeconds 仅 CUE 轨非零：CUE 轨的 FilePath 指向整轨镜像，
// 必须按区间采样，否则同一镜像下的所有 track 会拿到完全相同的指纹。
type SongIDPath struct {
	ID              int64
	FilePath        string
	CueStartSeconds float64
	CueEndSeconds   float64
}

// ListLocalWithoutFingerprint 返回所有尚无指纹、且从未尝试过计算的本地歌曲。
func (r *SongRepository) ListLocalWithoutFingerprint(ctx context.Context) ([]SongIDPath, error) {
	rows, err := r.queries.ListLocalWithoutFingerprint(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local without fingerprint: %w", err)
	}
	result := make([]SongIDPath, len(rows))
	for i, row := range rows {
		result[i] = SongIDPath{
			ID:              row.ID,
			FilePath:        row.FilePath,
			CueStartSeconds: row.CueStartSeconds,
			CueEndSeconds:   row.CueEndSeconds,
		}
	}
	return result, nil
}

// CountLocalFingerprints 返回本地歌曲总数、已计算指纹数，以及尝试过但失败的数量。
func (r *SongRepository) CountLocalFingerprints(ctx context.Context) (total, computed, failed int64, err error) {
	row, err := r.queries.CountLocalFingerprints(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("count local fingerprints: %w", err)
	}
	return row.Total, row.Computed, row.Failed, nil
}

// DuplicateGroup 表示一组指纹相同的歌曲。
type DuplicateGroup struct {
	Fingerprint string
	Songs       []*models.Song
}

// duplicateDurationTolerance 是判定「同指纹即重复」时允许的全片时长差（秒）。
// 指纹只采样前 fingerprintSampleSeconds 秒，理论上存在「统一片头的有声书/系列节目
// 前若干秒完全一致」的碰撞，用全片时长做二次护栏把这类误判排除。
//
// 取 30 秒而不是 2 秒：要拦的是「集与集之间差几分钟」的长音频碰撞，
// 而同一首歌的不同封装（CUE 轨长 vs 独立文件解码长度、尾部静音、容器时长精度）
// 差几秒是常态，阈值太小会把真重复切掉。
const duplicateDurationTolerance = 30.0

// ListDuplicateGroups 查询所有指纹重复的本地歌曲组。
// 同一指纹内还会按全片时长（fingerprint_duration）以 duplicateDurationTolerance
// 容差聚簇，只有簇内 ≥2 首才算重复组。
func (r *SongRepository) ListDuplicateGroups(ctx context.Context) ([]DuplicateGroup, error) {
	rows, err := r.queries.ListAllDuplicateSongs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all duplicate songs: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	// 按 fingerprint 分组（SQL 已按 fingerprint, fingerprint_duration 排序）
	grouped := make(map[string][]*models.Song)
	var order []string
	for _, row := range rows {
		if _, exists := grouped[row.Fingerprint]; !exists {
			order = append(order, row.Fingerprint)
		}
		grouped[row.Fingerprint] = append(grouped[row.Fingerprint], &models.Song{
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
		})
	}

	var groups []DuplicateGroup
	for _, fp := range order {
		for _, cluster := range clusterByFingerprintDuration(grouped[fp]) {
			groups = append(groups, DuplicateGroup{Fingerprint: fp, Songs: cluster})
		}
	}
	return groups, nil
}

// clusterByFingerprintDuration 把同指纹的歌曲按全片时长聚簇，返回簇内 ≥2 首的那些簇。
//
// 护栏刻意做得**保守**：这里的漏判（真重复没被报出来）比误判（把不同的歌报成重复）
// 更可接受不了——用户会照着这个列表删文件。所以只在两首歌的时长「明显不是一回事」时才切开：
//   - 相邻切分而非跟簇首比较：否则 300/301.5/303 这种链会被切成 [300,301.5] + 孤立的 303，
//     后者被丢弃，而它和 301.5 只差 1.5 秒
//   - 时长为 0 表示未知（stderr 没有 Duration 行，如某些 ADTS/裸流），一律不切，
//     否则「一份 0 + 一份 210」会被拆成两个单元素簇导致整组消失
func clusterByFingerprintDuration(songs []*models.Song) [][]*models.Song {
	if len(songs) < 2 {
		return nil
	}
	sorted := make([]*models.Song, len(songs))
	copy(sorted, songs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FingerprintDuration < sorted[j].FingerprintDuration
	})

	var clusters [][]*models.Song
	current := []*models.Song{sorted[0]}
	for i := 1; i < len(sorted); i++ {
		prev, cur := sorted[i-1], sorted[i]
		splittable := prev.FingerprintDuration > 0 && cur.FingerprintDuration > 0 &&
			cur.FingerprintDuration-prev.FingerprintDuration > duplicateDurationTolerance
		if !splittable {
			current = append(current, cur)
			continue
		}
		if len(current) > 1 {
			clusters = append(clusters, current)
		}
		current = []*models.Song{cur}
	}
	if len(current) > 1 {
		clusters = append(clusters, current)
	}
	return clusters
}

// Facet 标签分类的一个取值及其歌曲数量（如 genre="Rock", count=42）。
// CoverURL 为该取值下一首带封面歌曲的封面端点（无则为空，前端回退占位图标）。
type Facet struct {
	Value    string `json:"value"`
	Count    int64  `json:"count"`
	CoverURL string `json:"cover_url"`
}

// buildFacetSelect 构造某维度的 facet 聚合 SelectBuilder（已含过滤/排序/分页）。
// 列维度走 songs 单列 GROUP BY；tag 维度走 song_tags join（见 buildTagFacetSelect）。
// 未知维度返回 ErrNotFound，交由调用方（ListFacet/CountFacet）透传。
//
// 说明：这是本项目首个用 squirrel 编写的聚合（GROUP BY）查询——因为 facet 需要
// 可选 keyword（变长 WHERE）+ 动态排序 + 分页，sqlc 固定查询无法表达，按铁律「动态 SQL→squirrel」实现。
func buildFacetSelect(field string, f *FacetFilter) (sq.SelectBuilder, error) {
	if field == songFacetTagField {
		return buildTagFacetSelect(f), nil
	}
	if field == songFacetArtistField {
		return buildArtistFacetSelect(f), nil
	}
	col, ok := songFacetColumn[field]
	if !ok {
		return sq.SelectBuilder{}, ErrNotFound
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
	return sb, nil
}

// buildTagFacetSelect 构造 tag 维度的 facet 聚合：按用户自定义标签聚合歌曲数 + 代表封面。
// tag 不在 songs 表，需 join song_tags ↔ song_tag_links ↔ songs。
// value=标签名，count=挂在该标签下的不同歌曲数（COUNT DISTINCT——一首歌挂多标签不会重复计）；
// 内连接天然排除无关联歌曲的标签，故无需额外「取值非空」过滤（区别于列维度的 facetBaseCond）。
// 复用 applyFacetOrder（value/count 别名一致）与 cover_url 构造（见 ListFacet 的 scan）。
func buildTagFacetSelect(f *FacetFilter) sq.SelectBuilder {
	sb := sq.Select(
		"t.name AS value",
		"COUNT(DISTINCT l.song_id) AS count",
		"MAX(CASE WHEN s.cover_path != '' OR s.cover_url != '' THEN s.id END) AS cover_song_id",
	).
		From("song_tags t").
		Join("song_tag_links l ON t.id = l.tag_id").
		Join("songs s ON s.id = l.song_id")
	if f != nil && f.Keyword != "" {
		// t.name 是固定列（非用户输入），用 sq.Like 防 SQL 注入同列维度范式。
		sb = sb.Where(sq.Like{"t.name": "%" + f.Keyword + "%"})
	}
	sb = sb.GroupBy("t.id", "t.name")
	sb = applyFacetOrder(sb, f)
	if f != nil {
		sb = applyPagination(sb, f.Limit, f.Offset)
	}
	return sb
}

// buildArtistFacetSelect 构造 artist 维度的 facet 聚合：按 song_artists 关联表逐个
// 歌手聚合歌曲数 + 代表封面。多值歌手（对唱/合唱）的 "A & B" 在这里被拆成 A、B
// 两个独立取值，各自计数——而不是合并成一项。仅统计 role='artist' 的参与歌手。
func buildArtistFacetSelect(f *FacetFilter) sq.SelectBuilder {
	sb := sq.Select(
		"a.name AS value",
		"COUNT(DISTINCT sa.song_id) AS count",
		"MAX(CASE WHEN s.cover_path != '' OR s.cover_url != '' THEN s.id END) AS cover_song_id",
	).
		From("song_artists sa").
		Join("artists a ON a.id = sa.artist_id").
		Join("songs s ON s.id = sa.song_id").
		Where(sq.Eq{"sa.role": models.ArtistRoleArtist})
	if f != nil && f.Keyword != "" {
		sb = sb.Where(sq.Like{"a.name": "%" + f.Keyword + "%"})
	}
	sb = sb.GroupBy("a.id", "a.name")
	sb = applyFacetOrder(sb, f)
	if f != nil {
		sb = applyPagination(sb, f.Limit, f.Offset)
	}
	return sb
}

// ListFacet 按维度聚合曲库标签，返回该维度下的取值 + 计数 + 代表封面（支持搜索/排序/分页）。
// field 支持：genre / artist / album / language / style / year / decade / tag。
// tag 维度按用户自定义标签聚合（join song_tags ↔ song_tag_links ↔ songs），其余维度走 songs 单列。
// 未知 field 返回 ErrNotFound，交由 handler 转 400。
func (r *SongRepository) ListFacet(ctx context.Context, field string, f *FacetFilter) ([]Facet, error) {
	sb, err := buildFacetSelect(field, f)
	if err != nil {
		return nil, err
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
		// cover_song_id 取组内任意一首「有封面」（本地 cover_path 或远程 cover_url 非空）的歌曲。
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
	var inner sq.SelectBuilder
	if field == songFacetTagField {
		// tag：去重标签数（仅计至少挂了一首歌的标签）。内连接排除空标签。
		inner = sq.Select("1").
			From("song_tags t").
			Join("song_tag_links l ON t.id = l.tag_id")
		if keyword != "" {
			inner = inner.Where(sq.Like{"t.name": "%" + keyword + "%"})
		}
		inner = inner.GroupBy("t.id")
	} else if field == songFacetArtistField {
		// artist：去重参与歌手数（仅 role='artist'），与 buildArtistFacetSelect 一致。
		inner = sq.Select("1").
			From("song_artists sa").
			Join("artists a ON a.id = sa.artist_id").
			Where(sq.Eq{"sa.role": models.ArtistRoleArtist})
		if keyword != "" {
			inner = inner.Where(sq.Like{"a.name": "%" + keyword + "%"})
		}
		inner = inner.GroupBy("a.id")
	} else {
		col, ok := songFacetColumn[field]
		if !ok {
			return 0, ErrNotFound
		}
		inner = sq.Select("1").From("songs").Where(facetBaseCond(field, col))
		if keyword != "" {
			inner = inner.Where(sq.Expr("CAST("+col+" AS TEXT) LIKE ?", "%"+keyword+"%"))
		}
		inner = inner.GroupBy(col)
	}

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

// ListDistinctNames 返回某维度（title/artist）下曲库全部去重、非空取值，按取值升序。
// 不分页、不带计数与封面，供第三方客户端一次性拉取曲库歌名/歌手名做本地搜索匹配。
// 未知 field 返回 ErrNotFound，交由 handler 转 400。
func (r *SongRepository) ListDistinctNames(ctx context.Context, field string) ([]string, error) {
	// artist 维度走 song_artists 关联表：返回逐个参与歌手（多值已拆开），而非
	// songs.artist 整串去重。仅 role='artist'。
	if field == "artist" {
		query, args, err := sq.Select("DISTINCT a.name").
			From("song_artists sa").
			Join("artists a ON a.id = sa.artist_id").
			Where(sq.Eq{"sa.role": models.ArtistRoleArtist}).
			OrderBy("a.name COLLATE NOCASE ASC").
			ToSql()
		if err != nil {
			return nil, fmt.Errorf("build distinct artist sql: %w", err)
		}
		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("distinct artist: %w", err)
		}
		defer rows.Close()
		out := []string{}
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				return nil, fmt.Errorf("scan distinct artist: %w", err)
			}
			out = append(out, value)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate distinct artist: %w", err)
		}
		return out, nil
	}

	col, ok := songNameColumn[field]
	if !ok {
		return nil, ErrNotFound
	}

	sb := sq.Select("DISTINCT " + col).
		From("songs").
		Where(sq.NotEq{col: ""}).
		OrderBy(col + " ASC")

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build distinct %s sql: %w", field, err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("distinct %s: %w", field, err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("scan distinct %s: %w", field, err)
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate distinct %s: %w", field, err)
	}
	return out, nil
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

// ListSongsNeedingMetadata 返回本地有文件且元数据缺失的歌曲（本地歌曲或已缓存的网络歌曲，用于批量本地提取）。
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

// LibraryStats 曲库汇总统计信息
type LibraryStats struct {
	TotalSongs    int64   `json:"total_songs"`
	LocalSongs    int64   `json:"local_songs"`
	RemoteSongs   int64   `json:"remote_songs"`
	RadioSongs    int64   `json:"radio_songs"`
	TotalDuration float64 `json:"total_duration"`
	TotalFileSize int64   `json:"total_file_size"`
	ArtistCount   int64   `json:"artist_count"`
	AlbumCount    int64   `json:"album_count"`
	GenreCount    int64   `json:"genre_count"`
}

// GetLibraryStats 返回曲库汇总统计（一次查询完成）。
func (r *SongRepository) GetLibraryStats(ctx context.Context) (*LibraryStats, error) {
	query := `
		SELECT
			COUNT(*) AS total_songs,
			COALESCE(SUM(CASE WHEN type = 'local' THEN 1 ELSE 0 END), 0) AS local_songs,
			COALESCE(SUM(CASE WHEN type = 'remote' THEN 1 ELSE 0 END), 0) AS remote_songs,
			COALESCE(SUM(CASE WHEN type = 'radio' THEN 1 ELSE 0 END), 0) AS radio_songs,
			COALESCE(SUM(duration), 0) AS total_duration,
			COALESCE(SUM(file_size), 0) AS total_file_size
		FROM songs
	`
	stats := &LibraryStats{}
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalSongs,
		&stats.LocalSongs,
		&stats.RemoteSongs,
		&stats.RadioSongs,
		&stats.TotalDuration,
		&stats.TotalFileSize,
	)
	if err != nil {
		return nil, fmt.Errorf("get library stats: %w", err)
	}

	// 分别查询去重后的歌手、专辑、流派数量
	countQuery := `SELECT COUNT(DISTINCT a.id) FROM artists a JOIN song_artists sa ON sa.artist_id = a.id WHERE sa.role = 'artist'`
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&stats.ArtistCount); err != nil {
		return nil, fmt.Errorf("count artists: %w", err)
	}

	countQuery = `SELECT COUNT(DISTINCT album) FROM songs WHERE album != ''`
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&stats.AlbumCount); err != nil {
		return nil, fmt.Errorf("count albums: %w", err)
	}

	countQuery = `SELECT COUNT(DISTINCT genre) FROM songs WHERE genre != ''`
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&stats.GenreCount); err != nil {
		return nil, fmt.Errorf("count genres: %w", err)
	}

	return stats, nil
}

// FolderInfo 文件夹浏览条目。
type FolderInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	SongCount int    `json:"song_count"`
}

// ListFolders 按绝对路径前缀聚合下一级子文件夹，返回文件夹名与递归歌曲计数。
// absPrefix 必须以 "/" 结尾（如 "/volume1/music/" 或 "/volume1/music/评书/"）。
func (r *SongRepository) ListFolders(ctx context.Context, absPrefix, keyword string) ([]FolderInfo, error) {
	escaped := escapeLikeLiteral(absPrefix)
	query := `
SELECT
    SUBSTR(file_path, LENGTH(?) + 1,
           INSTR(SUBSTR(file_path, LENGTH(?) + 1), '/') - 1) AS folder_name,
    COUNT(*) AS song_count
FROM songs
WHERE type = 'local'
  AND file_path LIKE ? ESCAPE '\'
  AND INSTR(SUBSTR(file_path, LENGTH(?) + 1), '/') > 0
GROUP BY folder_name
HAVING folder_name != ''`

	args := []any{absPrefix, absPrefix, escaped + "%", absPrefix}

	if keyword != "" {
		query += ` AND folder_name LIKE ?`
		args = append(args, "%"+keyword+"%")
	}

	query += `
ORDER BY folder_name`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	defer rows.Close()

	var out []FolderInfo
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("scan folder: %w", err)
		}
		out = append(out, FolderInfo{Name: name, SongCount: count})
	}
	return out, rows.Err()
}

// ListDirectSongs 返回绝对路径前缀下直属的歌曲（不含子目录内的歌曲）。
func (r *SongRepository) ListDirectSongs(ctx context.Context, absPrefix, keyword string) ([]*models.Song, error) {
	escaped := escapeLikeLiteral(absPrefix)
	sb := songSelectBuilder().
		Where(sq.Eq{"type": "local"}).
		Where(sq.Expr(`file_path LIKE ? ESCAPE '\'`, escaped+"%")).
		Where(sq.Expr(`INSTR(SUBSTR(file_path, LENGTH(?) + 1), '/') = 0`, absPrefix)).
		OrderBy("file_path")

	if keyword != "" {
		kw := "%" + keyword + "%"
		sb = sb.Where(sq.Or{
			sq.Like{"title": kw},
			sq.Like{"artist": kw},
		})
	}

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build direct songs sql: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list direct songs: %w", err)
	}
	defer rows.Close()

	var songs []*models.Song
	for rows.Next() {
		song, err := scanSongRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan direct song: %w", err)
		}
		songs = append(songs, song)
	}
	return songs, rows.Err()
}
