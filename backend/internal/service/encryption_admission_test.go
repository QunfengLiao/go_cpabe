package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go-cpabe/backend/internal/pkg/response"
)

// TestEncryptionAdmissionConcurrentLeaseLimit 验证单租户前三个租约成功且第四个返回稳定可重试拒绝。
func TestEncryptionAdmissionConcurrentLeaseLimit(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	admission := NewEncryptionAdmission(client, 3, time.Minute)
	for index := 0; index < 3; index++ {
		if err := admission.Acquire(context.Background(), 1, uint64(index+1), string(rune('a'+index))); err != nil {
			t.Fatal(err)
		}
	}
	if err := admission.Acquire(context.Background(), 1, 9, "fourth"); err != response.ErrEncryptionConcurrencyLimited {
		t.Fatalf("unexpected fourth lease error: %v", err)
	}
	admission.Release(context.Background(), 1, "a")
	if err := admission.Acquire(context.Background(), 1, 9, "after-release"); err != nil {
		t.Fatal(err)
	}
}

// TestEncryptionAdmissionFailsClosedWithoutRedis 验证准入事实源不可用时不会创建无限制任务。
func TestEncryptionAdmissionFailsClosedWithoutRedis(t *testing.T) {
	if err := (*EncryptionAdmission)(nil).Acquire(context.Background(), 1, 1, "lease"); err != response.ErrEncryptionAdmissionUnavailable {
		t.Fatalf("unexpected error: %v", err)
	}
}
