package database

import (
	"context"
)

// DB 数据库接口，按职责分为仓储入口 + UnitOfWork 事务入口。
type DB interface {
	Close() error

	// RunInTx 在事务中执行 fn，fn 返回非 nil 时回滚。
	// fn 拿到的 UnitOfWork 里每个 Repository 都绑定在同一个 *sql.Tx 上。
	RunInTx(ctx context.Context, fn func(context.Context, *UnitOfWork) error) error

	JSPluginRepository() *JSPluginRepository
	TokenRepository() *TokenRepository
	ConfigRepository() *ConfigRepository
	SongRepository() *SongRepository
	PlaylistRepository() *PlaylistRepository
	PlaylistSongRepository() *PlaylistSongRepository
}

// NewSQLiteDB 兼容旧调用方的构造函数，内部委托给 Open。
// 调用方应逐步迁移到 Open() 直接拿 *SQLiteDB。
func NewSQLiteDB(dataSourceName string) (DB, error) {
	return Open(dataSourceName)
}
