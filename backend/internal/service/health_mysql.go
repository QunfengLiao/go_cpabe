package service

import (
	"context"
	"fmt"
	"time"

	"go-cpabe/backend/internal/model"

	"gorm.io/gorm"
)

// CheckMySQL 检查 MySQL 连接是否可 ping 通，并把初始化错误转换为依赖健康状态。
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

	startedAt := time.Now()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return dependencyError(fmt.Errorf("mysql connection failed: authentication or network error"))
	}
	elapsed := time.Since(startedAt)
	stats := sqlDB.Stats()

	return model.DependencyHealth{
		Status:  "ok",
		Message: "connected",
		Metrics: map[string]any{
			"pingMs":          elapsed.Milliseconds(),
			"openConnections": stats.OpenConnections,
			"inUse":           stats.InUse,
			"idle":            stats.Idle,
			"waitCount":       stats.WaitCount,
			"waitDurationMs":  stats.WaitDuration.Milliseconds(),
			"maxOpen":         stats.MaxOpenConnections,
		},
	}
}
