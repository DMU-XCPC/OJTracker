package sqlite

import (
	_ "embed"
	"errors"

	driver "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//go:embed schema.sql
var schema string

// Open 使用原有建表和迁移脚本，使新旧数据库使用相同的约束。
func Open(path string) (*gorm.DB, error) {
	db, err := gorm.Open(driver.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(1)
	if err := db.Exec("PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;" + schema).Error; err != nil {
		return nil, errors.Join(err, pool.Close())
	}
	return db, nil
}

// Close 释放 GORM 管理的底层连接池。
func Close(db *gorm.DB) error {
	pool, err := db.DB()
	if err != nil {
		return err
	}
	return pool.Close()
}
