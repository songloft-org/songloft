//go:build !android || !amd64 || !cgo

package database

import (
	"database/sql"
	"database/sql/driver"

	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("songloft_tag_number", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			value, _ := args[0].(string)
			return tagNumber(value), nil
		})
	sqlite.MustRegisterDeterministicScalarFunction("songloft_text_sort_key", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return nullableTextSortKey(args[0]), nil
		})
}

func openSQLite(dataSourceName string) (*sql.DB, error) {
	dsn := sqliteDSN(dataSourceName, "_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)&_pragma=cache_size(10000)&_pragma=foreign_keys(1)")
	return sql.Open("sqlite", dsn)
}
