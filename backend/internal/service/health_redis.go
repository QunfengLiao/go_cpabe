package service

import (
	"context"
	"fmt"
	"time"

	"go-cpabe/backend/internal/model"

	"github.com/redis/go-redis/v9"
)

// CheckRedis 检查 Redis 连接是否可 ping 通，并把初始化错误转换为依赖健康状态。
func CheckRedis(ctx context.Context, client *redis.Client, initErr error) model.DependencyHealth {
	if initErr != nil {
		return dependencyError(initErr)
	}
	if client == nil {
		return dependencyError(fmt.Errorf("redis connection failed: client is not initialized"))
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		return dependencyError(fmt.Errorf("redis connection failed: authentication or network error"))
	}

	return model.DependencyHealth{Status: "ok", Message: "connected"}
}
