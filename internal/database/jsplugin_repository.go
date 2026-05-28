package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"songloft/internal/database/sqlc"
	"songloft/internal/models"
)

// JSPluginRepository 是 JS 插件仓储，复用 sqlc 生成的查询。
type JSPluginRepository struct {
	queries *sqlc.Queries
}

// NewJSPluginRepository 用 *sql.DB 或 *sql.Tx 构造仓储。
func NewJSPluginRepository(db sqlc.DBTX) *JSPluginRepository {
	return &JSPluginRepository{queries: sqlc.New(db)}
}

// GetAll 获取所有 JS 插件。
func (r *JSPluginRepository) GetAll(ctx context.Context) ([]*models.JSPlugin, error) {
	rows, err := r.queries.ListJSPlugins(ctx)
	if err != nil {
		return nil, fmt.Errorf("list js_plugins: %w", err)
	}
	plugins := make([]*models.JSPlugin, 0, len(rows))
	for _, row := range rows {
		plugins = append(plugins, jsPluginRowToModel(row))
	}
	return plugins, nil
}

// GetByID 根据 ID 获取插件，找不到返回 ErrNotFound。
func (r *JSPluginRepository) GetByID(ctx context.Context, id int64) (*models.JSPlugin, error) {
	row, err := r.queries.GetJSPluginByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get js_plugin by id %d: %w", id, err)
	}
	return jsPluginRowToModel(row), nil
}

// GetByEntryPath 根据 entryPath 获取插件，找不到返回 ErrNotFound。
func (r *JSPluginRepository) GetByEntryPath(ctx context.Context, entryPath string) (*models.JSPlugin, error) {
	row, err := r.queries.GetJSPluginByEntryPath(ctx, entryPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get js_plugin by entry_path %q: %w", entryPath, err)
	}
	return jsPluginRowToModel(row), nil
}

// Create 创建插件，回填自增 ID。
func (r *JSPluginRepository) Create(ctx context.Context, plugin *models.JSPlugin) error {
	permissionsJSON, err := json.Marshal(plugin.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}
	id, err := r.queries.CreateJSPlugin(ctx, sqlc.CreateJSPluginParams{
		Name:           plugin.Name,
		Version:        plugin.Version,
		Description:    plugin.Description,
		Author:         plugin.Author,
		Homepage:       plugin.Homepage,
		License:        plugin.License,
		EntryPath:      plugin.EntryPath,
		Main:           plugin.Main,
		MinHostVersion: plugin.MinHostVersion,
		Permissions:    string(permissionsJSON),
		UpdateUrl:      plugin.UpdateURL,
		DownloadUrl:    plugin.DownloadURL,
		Status:         string(plugin.Status),
		ZipHash:        plugin.ZipHash,
		EntryHash:      plugin.EntryHash,
		FileModTime:    plugin.FileModTime,
		FilePath:       plugin.FilePath,
	})
	if err != nil {
		return fmt.Errorf("insert js_plugin: %w", err)
	}
	plugin.ID = id
	return nil
}

// Update 更新插件全部字段。
func (r *JSPluginRepository) Update(ctx context.Context, plugin *models.JSPlugin) error {
	permissionsJSON, err := json.Marshal(plugin.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}
	if err := r.queries.UpdateJSPlugin(ctx, sqlc.UpdateJSPluginParams{
		Name:           plugin.Name,
		Version:        plugin.Version,
		Description:    plugin.Description,
		Author:         plugin.Author,
		Homepage:       plugin.Homepage,
		License:        plugin.License,
		EntryPath:      plugin.EntryPath,
		Main:           plugin.Main,
		MinHostVersion: plugin.MinHostVersion,
		Permissions:    string(permissionsJSON),
		UpdateUrl:      plugin.UpdateURL,
		DownloadUrl:    plugin.DownloadURL,
		Status:         string(plugin.Status),
		ZipHash:        plugin.ZipHash,
		EntryHash:      plugin.EntryHash,
		FileModTime:    plugin.FileModTime,
		FilePath:       plugin.FilePath,
		ID:             plugin.ID,
	}); err != nil {
		return fmt.Errorf("update js_plugin %d: %w", plugin.ID, err)
	}
	return nil
}

// Delete 删除插件。
func (r *JSPluginRepository) Delete(ctx context.Context, id int64) error {
	if err := r.queries.DeleteJSPlugin(ctx, id); err != nil {
		return fmt.Errorf("delete js_plugin %d: %w", id, err)
	}
	return nil
}

// UpdateStatus 更新插件状态。
func (r *JSPluginRepository) UpdateStatus(ctx context.Context, id int64, status models.JSPluginStatus) error {
	if err := r.queries.UpdateJSPluginStatus(ctx, sqlc.UpdateJSPluginStatusParams{
		Status: string(status),
		ID:     id,
	}); err != nil {
		return fmt.Errorf("update js_plugin status %d: %w", id, err)
	}
	return nil
}

// UpdateHashes 更新插件的哈希和文件修改时间。
func (r *JSPluginRepository) UpdateHashes(ctx context.Context, id int64, zipHash, entryHash, fileModTime string) error {
	if err := r.queries.UpdateJSPluginHashes(ctx, sqlc.UpdateJSPluginHashesParams{
		ZipHash:     zipHash,
		EntryHash:   entryHash,
		FileModTime: fileModTime,
		ID:          id,
	}); err != nil {
		return fmt.Errorf("update js_plugin hashes %d: %w", id, err)
	}
	return nil
}

// jsPluginRowToModel 把 sqlc.JsPlugin 映射到 models.JSPlugin。
func jsPluginRowToModel(row sqlc.JsPlugin) *models.JSPlugin {
	p := &models.JSPlugin{
		ID:             row.ID,
		Name:           row.Name,
		Version:        row.Version,
		Description:    row.Description,
		Author:         row.Author,
		Homepage:       row.Homepage,
		License:        row.License,
		EntryPath:      row.EntryPath,
		Main:           row.Main,
		MinHostVersion: row.MinHostVersion,
		UpdateURL:      row.UpdateUrl,
		DownloadURL:    row.DownloadUrl,
		Status:         models.JSPluginStatus(row.Status),
		ZipHash:        row.ZipHash,
		EntryHash:      row.EntryHash,
		FileModTime:    row.FileModTime,
		FilePath:       row.FilePath,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if row.Permissions != "" {
		if err := json.Unmarshal([]byte(row.Permissions), &p.Permissions); err != nil {
			p.Permissions = []string{}
		}
	} else {
		p.Permissions = []string{}
	}
	return p
}
