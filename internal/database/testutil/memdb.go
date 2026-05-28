package testutil

import (
	"testing"

	"mimusic/internal/database"
)

// OpenMemoryDB 启动一个 :memory: SQLite，并在测试结束时自动关闭。
// goose Up 由 database.Open 内部触发，迁移完成后直接可用。
func OpenMemoryDB(t *testing.T) *database.SQLiteDB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
