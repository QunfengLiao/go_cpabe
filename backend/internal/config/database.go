package config

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// OpenDatabase 使用配置中的 MySQL DSN 建立 Gorm 连接，调用方负责迁移和健康检查。
func OpenDatabase(cfg Config) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
}
