package auth

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrRefreshSessionNotFound 表示 refresh token 对应的服务端会话不存在或已过期。
var ErrRefreshSessionNotFound = errors.New("refresh session not found")

// TokenStore 定义 Refresh Token 服务端会话的保存、读取、删除和轮换能力。
type TokenStore interface {
	Save(ctx context.Context, tokenID string, session RefreshSession, ttl time.Duration) error
	Get(ctx context.Context, tokenID string) (*RefreshSession, error)
	Delete(ctx context.Context, tokenID string) error
	Rotate(ctx context.Context, oldTokenID, newTokenID string, session RefreshSession, ttl time.Duration) error
}

// RefreshKey 将 tokenID 转换为 Redis key，统一认证会话命名空间。
func RefreshKey(tokenID string) string {
	return "auth:refresh:" + tokenID
}

// MemoryTokenStore 是测试使用的内存刷新会话存储。
type MemoryTokenStore struct {
	mu       sync.Mutex
	sessions map[string]RefreshSession
}

// NewMemoryTokenStore 创建内存刷新会话存储，主要用于单元测试。
func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{sessions: map[string]RefreshSession{}}
}

// Save 将刷新会话保存到内存映射中。
func (s *MemoryTokenStore) Save(_ context.Context, tokenID string, session RefreshSession, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[tokenID] = session
	return nil
}

// Get 从内存映射中读取刷新会话，不存在时返回 ErrRefreshSessionNotFound。
func (s *MemoryTokenStore) Get(_ context.Context, tokenID string) (*RefreshSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[tokenID]
	if !ok {
		return nil, ErrRefreshSessionNotFound
	}
	return &session, nil
}

// Delete 从内存映射中删除刷新会话。
func (s *MemoryTokenStore) Delete(_ context.Context, tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tokenID)
	return nil
}

// Rotate 按 Redis 实现的语义删除旧会话并保存新会话。
func (s *MemoryTokenStore) Rotate(ctx context.Context, oldTokenID, newTokenID string, session RefreshSession, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 内存实现仅用于测试；仍保持与 Redis 相同的轮换语义，避免测试掩盖重放风险。
	delete(s.sessions, oldTokenID)
	s.sessions[newTokenID] = session
	return nil
}
