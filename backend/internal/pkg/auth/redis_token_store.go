package auth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisTokenStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisTokenStore(client *redis.Client, ttl time.Duration) *RedisTokenStore {
	return &RedisTokenStore{client: client, ttl: ttl}
}

func (s *RedisTokenStore) Save(ctx context.Context, tokenID string, session RefreshSession, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.ttl
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, RefreshKey(tokenID), payload, ttl).Err()
}

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
		_ = s.Delete(ctx, tokenID)
		return nil, ErrRefreshSessionNotFound
	}
	return &session, nil
}

func (s *RedisTokenStore) Delete(ctx context.Context, tokenID string) error {
	return s.client.Del(ctx, RefreshKey(tokenID)).Err()
}

func (s *RedisTokenStore) Rotate(ctx context.Context, oldTokenID, newTokenID string, session RefreshSession, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.ttl
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, RefreshKey(oldTokenID))
	pipe.Set(ctx, RefreshKey(newTokenID), payload, ttl)
	_, err = pipe.Exec(ctx)
	return err
}
