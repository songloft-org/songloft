package jsplugin

import (
	"context"

	"songloft/internal/models"
)

// Repository 定义 JS 插件的数据库操作接口
type Repository interface {
	// GetAll 获取所有 JS 插件
	GetAll(ctx context.Context) ([]*models.JSPlugin, error)
	// GetByID 根据 ID 获取插件
	GetByID(ctx context.Context, id int64) (*models.JSPlugin, error)
	// GetByEntryPath 根据 entryPath 获取插件
	GetByEntryPath(ctx context.Context, entryPath string) (*models.JSPlugin, error)
	// Create 创建插件
	Create(ctx context.Context, plugin *models.JSPlugin) error
	// Update 更新插件
	Update(ctx context.Context, plugin *models.JSPlugin) error
	// Delete 删除插件
	Delete(ctx context.Context, id int64) error
	// UpdateStatus 更新插件状态
	UpdateStatus(ctx context.Context, id int64, status models.JSPluginStatus) error
	// UpdateHashes 更新插件的哈希和文件修改时间
	UpdateHashes(ctx context.Context, id int64, zipHash, entryHash, fileModTime string) error
}
