package config

import "github.com/redis/go-redis/v9"

// OpenRedis 根据配置创建 Redis 客户端，连接可用性由健康检查或首次操作确认。
func OpenRedis(cfg Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
}
