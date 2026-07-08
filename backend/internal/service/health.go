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

// HealthService 负责聚合应用、MySQL 和 Redis 的健康状态。
type HealthService struct {
	appEnv   string
	mysql    *gorm.DB
	mysqlErr error
	redis    *redis.Client
	redisErr error
	now      func() time.Time
}

// NewHealthService 创建健康检查服务，允许注入依赖初始化错误以便启动后展示降级状态。
func NewHealthService(cfg config.Config, mysql *gorm.DB, mysqlErr error, redis *redis.Client, redisErr error) *HealthService {
	return &HealthService{
		appEnv:   cfg.AppEnv,
		mysql:    mysql,
		mysqlErr: mysqlErr,
		redis:    redis,
		redisErr: redisErr,
		now:      time.Now,
	}
}

// Check 执行一次健康检查并返回聚合状态，任一依赖异常时整体状态为 degraded。
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

// dependencyError 将依赖异常转换为统一响应模型，并在返回前做敏感信息脱敏。
func dependencyError(err error) model.DependencyHealth {
	message := "dependency unavailable"
	if err != nil {
		message = sanitizeError(err.Error())
	}

	return model.DependencyHealth{Status: "error", Message: message}
}

// sanitizeError 对可能包含密码、token 或 secret 的依赖错误信息进行脱敏。
func sanitizeError(message string) string {
	sensitiveMarkers := []string{"password", "passwd", "pwd=", "token", "secret", "MYSQL_PASSWORD", "REDIS_PASSWORD"}
	lower := strings.ToLower(message)
	for _, marker := range sensitiveMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			// 健康检查会直接暴露给运维或前端，连接串/密钥类错误必须脱敏后再返回。
			return "dependency connection failed: sensitive details hidden"
		}
	}
	return message
}
