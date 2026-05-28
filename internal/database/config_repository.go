package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"mimusic/internal/database/sqlc"
	"mimusic/internal/models"
)

// ConfigRepository 配置项仓储。
// 固定 SQL（Get/Set/Delete）走 sqlc.Queries，动态过滤的 List/Count 走 squirrel。
type ConfigRepository struct {
	db      sqlc.DBTX
	queries *sqlc.Queries
}

// NewConfigRepository 用 *sql.DB 或 *sql.Tx 构造仓储。
func NewConfigRepository(db sqlc.DBTX) *ConfigRepository {
	return &ConfigRepository{db: db, queries: sqlc.New(db)}
}

// Get 按 key 取配置，找不到返回 ErrNotFound。
func (r *ConfigRepository) Get(ctx context.Context, key string) (*models.Config, error) {
	row, err := r.queries.GetConfig(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get config: %w", err)
	}
	return &models.Config{
		ID:        row.ID,
		Key:       row.Key,
		Value:     row.Value,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// Set 写入或更新配置（UPSERT on key）。
func (r *ConfigRepository) Set(ctx context.Context, config *models.Config) error {
	if err := r.queries.SetConfig(ctx, sqlc.SetConfigParams{
		Key:   config.Key,
		Value: config.Value,
	}); err != nil {
		return fmt.Errorf("set config: %w", err)
	}
	return nil
}

// Delete 按 key 删除配置，找不到返回 ErrNotFound。
func (r *ConfigRepository) Delete(ctx context.Context, key string) error {
	rows, err := r.queries.DeleteConfig(ctx, key)
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// List 列出配置，支持 key LIKE 关键词 + 白名单排序 + 分页。
func (r *ConfigRepository) List(ctx context.Context, filter *ConfigFilter) ([]*models.Config, error) {
	if filter == nil {
		filter = &ConfigFilter{}
	}
	sb := sq.Select("id", "key", "value", "updated_at").From("configs")
	if filter.Keyword != "" {
		sb = sb.Where(sq.Like{"key": "%" + filter.Keyword + "%"})
	}
	sb = applyOrder(sb, filter.OrderBy, filter.Order, "key ASC", configOrderWhitelist, "")
	sb = applyPagination(sb, filter.Limit, filter.Offset)

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list configs sql: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list configs: %w", err)
	}
	defer rows.Close()

	configs := []*models.Config{}
	for rows.Next() {
		c := &models.Config{}
		if err := rows.Scan(&c.ID, &c.Key, &c.Value, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate configs: %w", err)
	}
	return configs, nil
}

// Count 统计配置数量（同 List 的过滤条件）。
func (r *ConfigRepository) Count(ctx context.Context, filter *ConfigFilter) (int64, error) {
	if filter == nil {
		filter = &ConfigFilter{}
	}
	sb := sq.Select("COUNT(*)").From("configs")
	if filter.Keyword != "" {
		sb = sb.Where(sq.Like{"key": "%" + filter.Keyword + "%"})
	}
	query, args, err := sb.ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count configs sql: %w", err)
	}
	var n int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count configs: %w", err)
	}
	return n, nil
}
