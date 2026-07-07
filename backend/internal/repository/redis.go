package repository

import (
	"go-cpabe/backend/internal/config"

	"github.com/redis/go-redis/v9"
)

func OpenRedis(cfg config.Config) (*redis.Client, error) {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}), nil
}
