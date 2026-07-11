package auth

import (
	"context"
	"errors"
	"strconv"
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
	ReplaceDeviceSession(ctx context.Context, userID uint64, deviceID string, newTokenID string, session RefreshSession, ttl time.Duration) error
}

// RefreshKey 将 tokenID 转换为 Redis key，统一认证会话命名空间。
func RefreshKey(tokenID string) string {
	return "auth:refresh:" + tokenID
}

// UserDeviceSessionKey 定位同一用户在同一客户端设备上的当前 Refresh Session。
func UserDeviceSessionKey(userID uint64, deviceID string) string {
	return "auth:user_session:" + strconv.FormatUint(userID, 10) + ":" + deviceID
}

// MemoryTokenStore 是测试使用的内存刷新会话存储。
type MemoryTokenStore struct {
	mu          sync.Mutex
	sessions    map[string]RefreshSession
	deviceIndex map[string]string
}

// Count 返回当前内存会话数量，供认证服务测试验证同设备重复登录不会累积。
func (s *MemoryTokenStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

// NewMemoryTokenStore 创建内存刷新会话存储，主要用于单元测试。
func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{sessions: map[string]RefreshSession{}, deviceIndex: map[string]string{}}
}

// Save 将刷新会话保存到内存映射中。
func (s *MemoryTokenStore) Save(_ context.Context, tokenID string, session RefreshSession, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[tokenID] = session
	if session.DeviceID != "" {
		s.deviceIndex[UserDeviceSessionKey(session.UserID, session.DeviceID)] = tokenID
	}
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
	if existing, ok := s.sessions[tokenID]; ok && existing.DeviceID != "" {
		delete(s.deviceIndex, UserDeviceSessionKey(existing.UserID, existing.DeviceID))
	}
	delete(s.sessions, tokenID)
	return nil
}

// Rotate 按 Redis 实现的语义删除旧会话并保存新会话。
func (s *MemoryTokenStore) Rotate(ctx context.Context, oldTokenID, newTokenID string, session RefreshSession, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 内存实现仅用于测试；仍保持与 Redis 相同的轮换语义，避免测试掩盖重放风险。
	if existing, ok := s.sessions[oldTokenID]; ok && existing.DeviceID != "" {
		delete(s.deviceIndex, UserDeviceSessionKey(existing.UserID, existing.DeviceID))
	}
	delete(s.sessions, oldTokenID)
	s.sessions[newTokenID] = session
	if session.DeviceID != "" {
		s.deviceIndex[UserDeviceSessionKey(session.UserID, session.DeviceID)] = newTokenID
	}
	return nil
}

// ReplaceDeviceSession 替换同一用户同一设备上的旧会话，避免重复登录无限累积刷新会话。
func (s *MemoryTokenStore) ReplaceDeviceSession(_ context.Context, userID uint64, deviceID string, newTokenID string, session RefreshSession, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	indexKey := UserDeviceSessionKey(userID, deviceID)
	if oldTokenID := s.deviceIndex[indexKey]; oldTokenID != "" {
		delete(s.sessions, oldTokenID)
	}
	s.sessions[newTokenID] = session
	s.deviceIndex[indexKey] = newTokenID
	return nil
}
