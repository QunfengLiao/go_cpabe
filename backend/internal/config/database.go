package config

import (
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// OpenDatabase 使用配置中的 MySQL DSN 建立单例 Gorm 连接并设置连接池边界。
func OpenDatabase(cfg Config) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("mysql connection failed: authentication or network error")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("mysql connection failed: invalid database handle")
	}
	ConfigureSQLPool(sqlDB)
	return db, nil
}

// ConfigureSQLPool 统一设置 MySQL 连接池，避免查询高峰频繁创建新连接和握手。
func ConfigureSQLPool(sqlDB *sql.DB) {
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(30)
}
