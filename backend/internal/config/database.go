package config

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func OpenDatabase(cfg Config) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
}
