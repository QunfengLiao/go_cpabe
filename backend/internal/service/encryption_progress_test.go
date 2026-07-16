package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// TestEncryptionProgressValidation 验证合法字节进度、总量冻结、倒退/跳阶段冲突和跨租户不可见。
func TestEncryptionProgressValidation(t *testing.T) {
	server := miniredis.RunT(t)
	serviceLayer, repositoryLayer, _, _ := newEncryptionServiceFixture(redis.NewClient(&redis.Options{Addr: server.Addr()}))
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "1234567890abcdef", encryptionCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	updated, err := serviceLayer.ReportProgress(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, ProgressInput{Stage: domain.EncryptionValidating, ProcessedBytes: 0, TotalBytes: 6})
	if err != nil || updated.Task.Status != domain.EncryptionValidating {
		t.Fatalf("valid progress failed: %+v %v", updated.Task, err)
	}
	if _, err := serviceLayer.ReportProgress(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, ProgressInput{Stage: domain.EncryptionEncryptingFile, ProcessedBytes: 1, TotalBytes: 7}); err != response.ErrEncryptionStateConflict {
		t.Fatalf("changed total error=%v", err)
	}
	repositoryLayer.progressError = repository.ErrEncryptionStateConflict
	if _, err := serviceLayer.ReportProgress(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, ProgressInput{Stage: domain.EncryptionUploading, ProcessedBytes: 0, TotalBytes: 6}); err != response.ErrEncryptionStateConflict {
		t.Fatalf("state conflict error=%v", err)
	}
	if _, err := serviceLayer.ReportProgress(context.Background(), 4, 7, created.Task.PublicID, created.Attempt.PublicID, ProgressInput{Stage: domain.EncryptionValidating, TotalBytes: 6}); err != response.ErrEncryptionAttemptNotFound {
		t.Fatalf("cross tenant error=%v", err)
	}
}
