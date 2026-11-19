//go:build android && amd64 && cgo

package database

import (
	"database/sql"

	"github.com/mattn/go-sqlite3"
)

const androidSQLiteDriver = "songloft-android-sqlite"

func init() {
	// The x86_64 gomobile host runs under Android's application sandbox. SQLite's
	// native driver uses Android libc instead of the raw Linux syscalls used
	// by modernc (for example lstat on x86_64).
	sql.Register(androidSQLiteDriver, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			if err := conn.RegisterFunc("songloft_tag_number", func(value any) any {
				text, _ := value.(string)
				return tagNumber(text)
			}, true); err != nil {
				return err
			}
			return conn.RegisterFunc("songloft_text_sort_key", nullableTextSortKey, true)
		},
	})
}

func openSQLite(dataSourceName string) (*sql.DB, error) {
	dsn := sqliteDSN(dataSourceName, "_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL&_cache_size=10000&_foreign_keys=on")
	return sql.Open(androidSQLiteDriver, dsn)
}
