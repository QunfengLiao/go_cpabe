package auth

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrRefreshSessionNotFound = errors.New("refresh session not found")

type TokenStore interface {
	Save(ctx context.Context, tokenID string, session RefreshSession, ttl time.Duration) error
	Get(ctx context.Context, tokenID string) (*RefreshSession, error)
	Delete(ctx context.Context, tokenID string) error
	Rotate(ctx context.Context, oldTokenID, newTokenID string, session RefreshSession, ttl time.Duration) error
}

func RefreshKey(tokenID string) string {
	return "auth:refresh:" + tokenID
}

type MemoryTokenStore struct {
	mu       sync.Mutex
	sessions map[string]RefreshSession
}

func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{sessions: map[string]RefreshSession{}}
}

func (s *MemoryTokenStore) Save(_ context.Context, tokenID string, session RefreshSession, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[tokenID] = session
	return nil
}

func (s *MemoryTokenStore) Get(_ context.Context, tokenID string) (*RefreshSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[tokenID]
	if !ok {
		return nil, ErrRefreshSessionNotFound
	}
	return &session, nil
}

func (s *MemoryTokenStore) Delete(_ context.Context, tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tokenID)
	return nil
}

func (s *MemoryTokenStore) Rotate(ctx context.Context, oldTokenID, newTokenID string, session RefreshSession, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, oldTokenID)
	s.sessions[newTokenID] = session
	return nil
}
