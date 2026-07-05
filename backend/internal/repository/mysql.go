package repository

import (
	"fmt"
	"time"

	"go-cpabe/backend/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func OpenMySQL(cfg config.Config) (*gorm.DB, error) {
	if cfg.MySQLDSN == "" {
		return nil, fmt.Errorf("mysql config missing: MYSQL_DSN or MYSQL_* variables are required")
	}
	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("mysql connection failed: authentication or network error")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("mysql connection failed: invalid database handle")
	}
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)
	return db, nil
}
