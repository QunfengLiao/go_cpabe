package auth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisTokenStore 使用 Redis 保存 Refresh Token 服务端会话。
type RedisTokenStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisTokenStore 创建基于 Redis 的 Refresh Token 会话存储。
func NewRedisTokenStore(client *redis.Client, ttl time.Duration) *RedisTokenStore {
	return &RedisTokenStore{client: client, ttl: ttl}
}

// Save 将刷新会话写入 Redis，ttl 控制服务端会话过期时间。
func (s *RedisTokenStore) Save(ctx context.Context, tokenID string, session RefreshSession, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.ttl
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	// Redis TTL 是 Refresh Session 的服务端失效边界，不能只依赖客户端过期时间。
	return s.client.Set(ctx, RefreshKey(tokenID), payload, ttl).Err()
}

// Get 按 tokenID 读取刷新会话，过期或不存在时返回 ErrRefreshSessionNotFound。
func (s *RedisTokenStore) Get(ctx context.Context, tokenID string) (*RefreshSession, error) {
	value, err := s.client.Get(ctx, RefreshKey(tokenID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrRefreshSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	var session RefreshSession
	if err := json.Unmarshal([]byte(value), &session); err != nil {
		return nil, err
	}
	if !session.ExpiresAt.IsZero() && time.Now().UTC().After(session.ExpiresAt) {
		// 双重检查过期时间：即使 Redis TTL 配置异常，也不允许过期会话继续刷新 token。
		_ = s.Delete(ctx, tokenID)
		return nil, ErrRefreshSessionNotFound
	}
	return &session, nil
}

// Delete 删除指定刷新会话，常用于退出登录或清理过期会话。
func (s *RedisTokenStore) Delete(ctx context.Context, tokenID string) error {
	return s.client.Del(ctx, RefreshKey(tokenID)).Err()
}

// Rotate 删除旧刷新会话并写入新会话，用于刷新 token 时降低重放风险。
func (s *RedisTokenStore) Rotate(ctx context.Context, oldTokenID, newTokenID string, session RefreshSession, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.ttl
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	// 轮换必须删除旧会话再写入新会话，减少 Refresh Token 被截获后的重放机会。
	pipe.Del(ctx, RefreshKey(oldTokenID))
	pipe.Set(ctx, RefreshKey(newTokenID), payload, ttl)
	_, err = pipe.Exec(ctx)
	return err
}
