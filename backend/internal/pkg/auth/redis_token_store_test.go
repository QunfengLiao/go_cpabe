package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestRedisTokenStoreRotate 验证 Redis 刷新会话轮换会删除旧会话并保存新会话。
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

// TestRedisTokenStoreReplaceDeviceSession 验证同一用户同一设备重新登录会替换旧 Refresh Session，并保持索引 TTL。
func TestRedisTokenStoreReplaceDeviceSession(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	store := NewRedisTokenStore(client, time.Hour)
	ctx := context.Background()

	first := RefreshSession{UserID: 7, DeviceID: "electron-device", SessionID: "s1", RefreshTokenHash: "old", ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.ReplaceDeviceSession(ctx, 7, "electron-device", "old-token", first, time.Hour); err != nil {
		t.Fatalf("replace first: %v", err)
	}
	second := RefreshSession{UserID: 7, DeviceID: "electron-device", SessionID: "s2", RefreshTokenHash: "new", ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.ReplaceDeviceSession(ctx, 7, "electron-device", "new-token", second, time.Hour); err != nil {
		t.Fatalf("replace second: %v", err)
	}
	if _, err := store.Get(ctx, "old-token"); err != ErrRefreshSessionNotFound {
		t.Fatalf("expected old session removed, got %v", err)
	}
	if got := server.Get(UserDeviceSessionKey(7, "electron-device")); got != "new-token" {
		t.Fatalf("unexpected device index: %q", got)
	}
	if ttl := server.TTL(UserDeviceSessionKey(7, "electron-device")); ttl <= 0 {
		t.Fatalf("expected index ttl, got %v", ttl)
	}
}
