package repository

import (
	"go-cpabe/backend/internal/config"

	"github.com/redis/go-redis/v9"
)

// OpenRedis 使用仓储层配置创建 Redis 客户端，返回 error 是为了和其他依赖初始化保持一致。
func OpenRedis(cfg config.Config) (*redis.Client, error) {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}), nil
}
