package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"songloft/internal/database/sqlc"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// SQLiteDB SQLite 数据库实现。
// 持有 *sql.DB 给 squirrel 用，*sqlc.Queries 给固定 SQL 用。
type SQLiteDB struct {
	db      *sql.DB
	queries *sqlc.Queries
}

// Open 打开 SQLite，执行 goose Up，返回 *SQLiteDB。
// dataSourceName 是文件路径或 ":memory:"。
func Open(dataSourceName string) (*SQLiteDB, error) {
	// 通过 DSN 参数开启 WAL 模式、busy_timeout 等优化配置。
	// 注意 modernc.org/sqlite 只识别 _pragma=... 形式。
	//   journal_mode(WAL)    : WAL 模式允许读写并发，读不被写阻塞
	//   busy_timeout(10000)  : 遇到锁时最多等待 10s，避免并发写直接 SQLITE_BUSY
	//   synchronous(NORMAL)  : WAL 模式下 NORMAL 已足够安全
	//   cache_size(10000)    : 页缓存 ~40MB
	//   foreign_keys(1)      : 启用外键
	dsn := dataSourceName + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)&_pragma=cache_size(10000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteDB{
		db:      db,
		queries: sqlc.New(db),
	}, nil
}

// runMigrations 使用 goose 执行 embed 的 SQL 迁移。
func runMigrations(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// Close 关闭数据库连接
func (s *SQLiteDB) Close() error {
	return s.db.Close()
}

// DB 返回底层 *sql.DB。仅供 testutil 与少数需要 raw 连接的场景使用。
func (s *SQLiteDB) DB() *sql.DB {
	return s.db
}

// RunInTx 在事务中执行 fn，fn 返回非 nil 时回滚。
// fn 拿到的 UnitOfWork 里每个 Repository 都绑定在同一个 *sql.Tx 上。
func (s *SQLiteDB) RunInTx(ctx context.Context, fn func(context.Context, *UnitOfWork) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	uow := &UnitOfWork{
		Songs:         NewSongRepository(tx),
		Playlists:     NewPlaylistRepository(tx),
		PlaylistSongs: NewPlaylistSongRepository(tx),
	}
	if err := fn(ctx, uow); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// JSPluginRepository 返回 JS 插件仓储（旧接口兼容）
func (s *SQLiteDB) JSPluginRepository() *JSPluginRepository {
	return NewJSPluginRepository(s.db)
}

// TokenRepository 返回认证令牌仓储
func (s *SQLiteDB) TokenRepository() *TokenRepository {
	return NewTokenRepository(s.db)
}

// ConfigRepository 返回配置项仓储
func (s *SQLiteDB) ConfigRepository() *ConfigRepository {
	return NewConfigRepository(s.db)
}

// SongRepository 返回歌曲仓储
func (s *SQLiteDB) SongRepository() *SongRepository {
	return NewSongRepository(s.db)
}

// PlaylistRepository 返回歌单仓储
func (s *SQLiteDB) PlaylistRepository() *PlaylistRepository {
	return NewPlaylistRepository(s.db)
}

// PlaylistSongRepository 返回歌单-歌曲关联仓储
func (s *SQLiteDB) PlaylistSongRepository() *PlaylistSongRepository {
	return NewPlaylistSongRepository(s.db)
}
