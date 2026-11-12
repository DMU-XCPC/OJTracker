package sqlite

import (
	"database/sql"
	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

// Open 初始化数据库连接与内嵌表结构。
func Open(path string) (*sql.DB, error) {
	// 打开连接池，实际建库和建表在执行 SQL 时进行。
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	// SQLite 的外键开关和繁忙等待超时按连接生效。
	db.SetMaxOpenConns(1)
	// 应用连接参数及内嵌迁移，失败时释放连接池。
	if _, err = db.Exec("PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;" + schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
