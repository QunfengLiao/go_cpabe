package repository

import (
	"fmt"

	"go-cpabe/backend/internal/config"

	"github.com/redis/go-redis/v9"
)

func OpenRedis(cfg config.RedisConfig) (*redis.Client, error) {
	if !cfg.Ready() {
		return nil, fmt.Errorf("redis config missing: addr and db are required")
	}

	return redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}), nil
}
