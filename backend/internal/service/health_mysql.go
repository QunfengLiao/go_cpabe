package service

import (
	"context"
	"fmt"
	"time"

	"go-cpabe/backend/internal/model"

	"gorm.io/gorm"
)

func CheckMySQL(ctx context.Context, db *gorm.DB, initErr error) model.DependencyHealth {
	if initErr != nil {
		return dependencyError(initErr)
	}
	if db == nil {
		return dependencyError(fmt.Errorf("mysql connection failed: database handle is not initialized"))
	}

	sqlDB, err := db.DB()
	if err != nil {
		return dependencyError(fmt.Errorf("mysql connection failed: invalid database handle"))
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		return dependencyError(fmt.Errorf("mysql connection failed: authentication or network error"))
	}

	return model.DependencyHealth{Status: "ok", Message: "connected"}
}
