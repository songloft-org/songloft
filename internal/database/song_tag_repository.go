package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"

	"songloft/internal/database/sqlc"
	"songloft/internal/models"
)

// SongTagRepository 自定义标签仓储。
type SongTagRepository struct {
	db      sqlc.DBTX
	queries *sqlc.Queries
}

func NewSongTagRepository(db sqlc.DBTX) *SongTagRepository {
	return &SongTagRepository{db: db, queries: sqlc.New(db)}
}

func (r *SongTagRepository) Create(ctx context.Context, name, color string) (int64, error) {
	return r.queries.CreateSongTag(ctx, sqlc.CreateSongTagParams{Name: name, Color: color})
}

func (r *SongTagRepository) GetByID(ctx context.Context, id int64) (*models.SongTag, error) {
	row, err := r.queries.GetSongTagByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	tag := sqlcSongTagToModel(row)
	songs, err := r.CountSongs(ctx, id)
	if err != nil {
		return nil, err
	}
	playlists, err := r.queries.CountVisiblePlaylistsByTag(ctx, models.PlaylistTagLabel(id))
	if err != nil {
		return nil, err
	}
	tag.SongCount, tag.PlaylistCount = int(songs), int(playlists)
	return tag, nil
}

func (r *SongTagRepository) GetByName(ctx context.Context, name string) (*models.SongTag, error) {
	row, err := r.queries.GetSongTagByName(ctx, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return sqlcSongTagToModel(row), nil
}

func (r *SongTagRepository) Update(ctx context.Context, id int64, name, color string) error {
	return r.queries.UpdateSongTag(ctx, sqlc.UpdateSongTagParams{ID: id, Name: name, Color: color})
}

func (r *SongTagRepository) Delete(ctx context.Context, id int64) error {
	return r.queries.DeleteSongTag(ctx, id)
}

func (r *SongTagRepository) List(ctx context.Context, keyword, orderBy, order string, limit, offset int) ([]models.SongTag, error) {
	sb := sq.Select(
		"t.id", "t.name", "t.color", "t.created_at",
		"COUNT(l.song_id) AS song_count",
		"(SELECT COUNT(*) FROM playlists p WHERE EXISTS (SELECT 1 FROM json_each(p.labels) j WHERE j.value = 'tag:' || t.id) AND NOT EXISTS (SELECT 1 FROM json_each(p.labels) WHERE value = 'hidden')) AS playlist_count",
		"COALESCE((SELECT s.cover_url FROM songs s JOIN song_tag_links l2 ON s.id = l2.song_id WHERE l2.tag_id = t.id ORDER BY l2.created_at DESC LIMIT 1), '') AS cover_url",
	).From("song_tags t").
		LeftJoin("song_tag_links l ON t.id = l.tag_id").
		GroupBy("t.id")

	if keyword != "" {
		sb = sb.Where(sq.Like{"t.name": "%" + keyword + "%"})
	}

	sb = applySongTagOrder(sb, orderBy, order)

	if limit > 0 {
		sb = sb.Limit(uint64(limit))
	}
	if offset > 0 {
		sb = sb.Offset(uint64(offset))
	}

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("song_tag_repository: build list query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []models.SongTag
	for rows.Next() {
		var t models.SongTag
		var createdAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &createdAt, &t.SongCount, &t.PlaylistCount, &t.CoverURL); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			t.CreatedAt = createdAt.Time
		}
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []models.SongTag{}
	}
	return tags, rows.Err()
}

func (r *SongTagRepository) Count(ctx context.Context, keyword string) (int64, error) {
	sb := sq.Select("COUNT(*)").From("song_tags")
	if keyword != "" {
		sb = sb.Where(sq.Like{"name": "%" + keyword + "%"})
	}
	query, args, err := sb.ToSql()
	if err != nil {
		return 0, err
	}
	var count int64
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *SongTagRepository) GetBySongID(ctx context.Context, songID int64) ([]models.SongTag, error) {
	rows, err := r.queries.GetTagsBySongID(ctx, songID)
	if err != nil {
		return nil, err
	}
	tags := make([]models.SongTag, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, *sqlcSongTagToModel(row))
	}
	return tags, nil
}

func (r *SongTagRepository) SetSongTags(ctx context.Context, songID int64, tagIDs []int64) error {
	return r.runInTx(ctx, func(db sqlc.DBTX, q *sqlc.Queries) error {
		if err := q.UnlinkAllBySong(ctx, songID); err != nil {
			return err
		}
		for _, tagID := range tagIDs {
			if err := q.LinkSongTag(ctx, sqlc.LinkSongTagParams{SongID: songID, TagID: tagID}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SongTagRepository) LinkSongTag(ctx context.Context, songID, tagID int64) error {
	return r.queries.LinkSongTag(ctx, sqlc.LinkSongTagParams{SongID: songID, TagID: tagID})
}

func (r *SongTagRepository) UnlinkSongTag(ctx context.Context, songID, tagID int64) error {
	return r.queries.UnlinkSongTag(ctx, sqlc.UnlinkSongTagParams{SongID: songID, TagID: tagID})
}

func (r *SongTagRepository) ListSongIDs(ctx context.Context, tagID int64, limit, offset int) ([]int64, error) {
	sb := sq.Select("song_id").From("song_tag_links").Where(sq.Eq{"tag_id": tagID}).
		OrderBy("created_at DESC")
	if limit > 0 {
		sb = sb.Limit(uint64(limit))
	}
	if offset > 0 {
		sb = sb.Offset(uint64(offset))
	}
	query, args, err := sb.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if ids == nil {
		ids = []int64{}
	}
	return ids, rows.Err()
}

func (r *SongTagRepository) CountSongs(ctx context.Context, tagID int64) (int64, error) {
	return r.queries.CountSongsByTag(ctx, tagID)
}

func (r *SongTagRepository) runInTx(ctx context.Context, fn func(sqlc.DBTX, *sqlc.Queries) error) error {
	if sqlDB, ok := r.db.(*sql.DB); ok {
		tx, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := fn(tx, r.queries.WithTx(tx)); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	return fn(r.db, r.queries)
}

func sqlcSongTagToModel(row sqlc.SongTag) *models.SongTag {
	t := &models.SongTag{
		ID:    row.ID,
		Name:  row.Name,
		Color: row.Color,
	}
	if row.CreatedAt.Valid {
		t.CreatedAt = row.CreatedAt.Time
	} else {
		t.CreatedAt = time.Time{}
	}
	return t
}

var songTagOrderWhitelist = map[string]struct{}{
	"id": {}, "name": {}, "created_at": {}, "song_count": {}, "playlist_count": {},
}

func (r *SongTagRepository) GetByPlaylistID(ctx context.Context, id int64) ([]models.SongTag, error) {
	if _, err := NewPlaylistRepository(r.db).GetByID(ctx, id); err != nil {
		return nil, err
	}
	rows, err := r.queries.GetTagsByPlaylistID(ctx, id)
	if err != nil {
		return nil, err
	}
	tags := make([]models.SongTag, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, *sqlcSongTagToModel(row))
	}
	return tags, nil
}

// SetPlaylistTags runs on the caller's UnitOfWork transaction. System labels survive.
func (r *SongTagRepository) SetPlaylistTags(ctx context.Context, id int64, tagIDs []int64) error {
	p, err := NewPlaylistRepository(r.db).GetByID(ctx, id)
	if err != nil {
		return err
	}
	labels := make([]string, 0, len(p.Labels)+len(tagIDs))
	for _, label := range p.Labels {
		if !strings.HasPrefix(label, "tag:") {
			labels = append(labels, label)
		}
	}
	seen := make(map[int64]bool)
	for _, tagID := range tagIDs {
		if tagID <= 0 {
			return fmt.Errorf("invalid tag ID: %d", tagID)
		}
		if seen[tagID] {
			continue
		}
		if _, err := r.queries.GetSongTagByID(ctx, tagID); err != nil {
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		labels = append(labels, models.PlaylistTagLabel(tagID))
		seen[tagID] = true
	}
	encoded, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	_, err = r.queries.UpdatePlaylistLabels(ctx, sqlc.UpdatePlaylistLabelsParams{ID: id, Labels: string(encoded)})
	return err
}

func (r *SongTagRepository) ClearPlaylistReferences(ctx context.Context, id int64) error {
	return r.queries.ClearPlaylistTagReferences(ctx, models.PlaylistTagLabel(id))
}

func applySongTagOrder(sb sq.SelectBuilder, orderBy, order string) sq.SelectBuilder {
	if orderBy == "" {
		orderBy = "song_count"
	}
	if _, ok := songTagOrderWhitelist[orderBy]; !ok {
		orderBy = "song_count"
	}
	if order == "" {
		order = "desc"
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}
	return sb.OrderBy(orderBy + " " + order)
}
