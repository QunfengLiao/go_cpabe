package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisTokenStoreRotate(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	store := NewRedisTokenStore(client, time.Hour)
	ctx := context.Background()

	oldSession := RefreshSession{UserID: 1, SessionID: "s1", RefreshTokenHash: "old", ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.Save(ctx, "old", oldSession, time.Hour); err != nil {
		t.Fatalf("save: %v", err)
	}
	newSession := RefreshSession{UserID: 1, SessionID: "s2", RefreshTokenHash: "new", ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.Rotate(ctx, "old", "new", newSession, time.Hour); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := store.Get(ctx, "old"); err != ErrRefreshSessionNotFound {
		t.Fatalf("expected old missing, got %v", err)
	}
	got, err := store.Get(ctx, "new")
	if err != nil {
		t.Fatalf("get new: %v", err)
	}
	if got.RefreshTokenHash != "new" {
		t.Fatalf("unexpected session: %+v", got)
	}
	if ttl := server.TTL(RefreshKey("new")); ttl <= 0 {
		t.Fatalf("expected ttl, got %v", ttl)
	}
}
