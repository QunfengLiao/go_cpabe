package repository

import (
	"fmt"
	"time"

	"go-cpabe/backend/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func OpenMySQL(cfg config.MySQLConfig) (*gorm.DB, error) {
	if !cfg.Ready() {
		return nil, fmt.Errorf("mysql config missing: host, user, database and port are required")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=3s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
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
