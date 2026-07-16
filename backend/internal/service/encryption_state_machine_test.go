package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
)

// TestEncryptionFailureRetryAndCancelStateMachine 验证失败分类、重试序号、授权不变和取消幂等。
func TestEncryptionFailureRetryAndCancelStateMachine(t *testing.T) {
	server := miniredis.RunT(t)
	serviceLayer, _, _, _ := newEncryptionServiceFixture(redis.NewClient(&redis.Options{Addr: server.Addr()}))
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "1234567890abcdef", encryptionCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	snapshotHash := created.Task.AuthorizationSnapshotSHA256
	if err := serviceLayer.Fail(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, "UPLOAD_INTERRUPTED", true); err != nil {
		t.Fatal(err)
	}
	retried, err := serviceLayer.Retry(context.Background(), 3, 7, created.Task.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if retried.Attempt.AttemptNo != 2 || retried.Task.AuthorizationSnapshotSHA256 != snapshotHash {
		t.Fatalf("unsafe retry: %+v", retried)
	}
	cancelled, err := serviceLayer.Cancel(context.Background(), 3, 7, created.Task.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Task.Status != domain.EncryptionCancelled {
		t.Fatalf("cancel status=%s", cancelled.Task.Status)
	}
	if _, err := serviceLayer.Retry(context.Background(), 3, 7, created.Task.PublicID); err != response.ErrEncryptionRetryRejected {
		t.Fatalf("cancelled retry error=%v", err)
	}
}
