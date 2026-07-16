package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go-cpabe/backend/internal/domain"
)

// TestEncryptionCompleteFailureRegistersOrphan 验证对象提交后数据库事务失败会登记不可下载孤儿对象。
func TestEncryptionCompleteFailureRegistersOrphan(t *testing.T) {
	server := miniredis.RunT(t)
	serviceLayer, repositoryLayer, _, _ := newEncryptionServiceFixture(redis.NewClient(&redis.Options{Addr: server.Addr()}))
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "1234567890abcdef", encryptionCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	object, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("a", 64), "GCPABE01", 6, strings.NewReader("cipher"))
	if err != nil {
		t.Fatal(err)
	}
	repositoryLayer.completeError = errors.New("injected transaction rollback")
	if _, _, err := serviceLayer.Complete(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, encryptionCompleteInput(object.PublicID)); err == nil {
		t.Fatal("complete should fail")
	}
	if len(repositoryLayer.orphans) != 1 || !strings.HasSuffix(repositoryLayer.orphans[0].ObjectKey, ".cipher") {
		t.Fatalf("orphan not registered: %+v", repositoryLayer.orphans)
	}
}

// TestOrphanCleanupBackoff 验证删除失败保存脱敏错误并安排指数退避。
func TestOrphanCleanupBackoff(t *testing.T) {
	repositoryLayer := &encryptionRepositoryStub{orphans: []domain.OrphanStorageObject{{ID: 1, TenantID: 3, ObjectKey: "3/object.cipher", ReasonCode: "ROLLBACK", Status: "PENDING"}}}
	storageLayer := &encryptedStorageStub{deleteError: errors.New("disk unavailable")}
	audit := &auditRecorderStub{}
	result, err := NewOrphanCleanupService(repositoryLayer, storageLayer, audit, time.Hour).Run(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed != 1 || repositoryLayer.orphans[0].Status != "FAILED" || repositoryLayer.orphans[0].LastErrorCode != "STORAGE_DELETE_FAILED" || repositoryLayer.orphans[0].NextRetryAt == nil {
		t.Fatalf("unexpected cleanup result: %+v orphan=%+v", result, repositoryLayer.orphans[0])
	}
}
