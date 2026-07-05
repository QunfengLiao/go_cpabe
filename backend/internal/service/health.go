package service

import (
	"context"
	"strings"
	"time"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/model"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthService struct {
	appEnv   string
	mysql    *gorm.DB
	mysqlErr error
	redis    *redis.Client
	redisErr error
	now      func() time.Time
}

func NewHealthService(cfg config.Config, mysql *gorm.DB, mysqlErr error, redis *redis.Client, redisErr error) *HealthService {
	return &HealthService{
		appEnv:   cfg.App.Env,
		mysql:    mysql,
		mysqlErr: mysqlErr,
		redis:    redis,
		redisErr: redisErr,
		now:      time.Now,
	}
}

func (s *HealthService) Check(ctx context.Context) model.HealthResponse {
	mysqlHealth := CheckMySQL(ctx, s.mysql, s.mysqlErr)
	redisHealth := CheckRedis(ctx, s.redis, s.redisErr)

	status := "ok"
	if mysqlHealth.Status != "ok" || redisHealth.Status != "ok" {
		status = "degraded"
	}

	return model.HealthResponse{
		Status:    status,
		CheckedAt: s.now().Format(time.RFC3339),
		App: model.AppHealth{
			Status: "ok",
			Env:    s.appEnv,
		},
		MySQL: mysqlHealth,
		Redis: redisHealth,
	}
}

func dependencyError(err error) model.DependencyHealth {
	message := "dependency unavailable"
	if err != nil {
		message = sanitizeError(err.Error())
	}

	return model.DependencyHealth{Status: "error", Message: message}
}

func sanitizeError(message string) string {
	sensitiveMarkers := []string{"password", "passwd", "pwd=", "token", "secret", "MYSQL_PASSWORD", "REDIS_PASSWORD"}
	lower := strings.ToLower(message)
	for _, marker := range sensitiveMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return "dependency connection failed: sensitive details hidden"
		}
	}
	return message
}
