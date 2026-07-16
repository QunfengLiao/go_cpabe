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
	"go-cpabe/backend/internal/repository"
)

// TestEncryptionAuditCoverage 验证创建、授权、进度、AES、DEK、上传和完成事件区分信任等级。
func TestEncryptionAuditCoverage(t *testing.T) {
	server := miniredis.RunT(t)
	serviceLayer, _, _, audit := newEncryptionServiceFixture(redis.NewClient(&redis.Options{Addr: server.Addr()}))
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "1234567890abcdef", encryptionCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := serviceLayer.ReportProgress(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, ProgressInput{Stage: domain.EncryptionValidating, TotalBytes: 6}); err != nil {
		t.Fatal(err)
	}
	object, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("a", 64), "GCPABE01", 6, strings.NewReader("cipher"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := serviceLayer.Complete(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, encryptionCompleteInput(object.PublicID)); err != nil {
		t.Fatal(err)
	}
	actions := map[string]string{}
	for _, event := range audit.events {
		actions[event.Action] = event.SourceTrust
	}
	for _, action := range []string{"encryption.task.create", "encryption.authorization.validated", "encryption.progress", "encryption.ciphertext.upload", "encryption.aes.complete", "encryption.dek.protect", "encryption.complete"} {
		if actions[action] == "" {
			t.Fatalf("missing audit action %s: %+v", action, audit.events)
		}
	}
	if actions["encryption.aes.complete"] != "CLIENT_REPORTED" || actions["encryption.dek.protect"] != "CLIENT_REPORTED" || actions["encryption.complete"] != "SERVER_OBSERVED" {
		t.Fatalf("incorrect audit trust: %+v", actions)
	}
}

// TestFailureRetryCancelDownloadAndCleanupAudits 验证失败、重试、取消、下载拒绝和清理失败均产生稳定审计。
func TestFailureRetryCancelDownloadAndCleanupAudits(t *testing.T) {
	server := miniredis.RunT(t)
	serviceLayer, repositoryLayer, storageLayer, audit := newEncryptionServiceFixture(redis.NewClient(&redis.Options{Addr: server.Addr()}))
	created, _, _ := serviceLayer.CreateTask(context.Background(), 3, 7, "1234567890abcdef", encryptionCreateInput())
	if err := serviceLayer.Fail(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, "UPLOAD_INTERRUPTED", true); err != nil {
		t.Fatal(err)
	}
	retried, err := serviceLayer.Retry(context.Background(), 3, 7, created.Task.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := serviceLayer.Cancel(context.Background(), 3, 7, retried.Task.PublicID); err != nil {
		t.Fatal(err)
	}
	fileService := NewEncryptedFileService(repositoryLayer, storageLayer, audit)
	_, _ = fileService.Download(context.Background(), 3, 99, repositoryLayer.aggregate.File.PublicID)
	repositoryLayer.orphans = []domain.OrphanStorageObject{{ID: 1, TenantID: 3, ObjectKey: "3/orphan.cipher", ReasonCode: "ROLLBACK", Status: "PENDING"}}
	storageLayer.deleteError = errors.New("delete failed")
	_, _ = NewOrphanCleanupService(repositoryLayer, storageLayer, audit, time.Hour).Run(context.Background(), 10)
	actions := map[string]bool{}
	for _, event := range audit.events {
		actions[event.Action] = true
	}
	for _, action := range []string{"encryption.fail", "encryption.retry", "encryption.cancel", "encrypted_file.download", "encryption.storage.cleanup"} {
		if !actions[action] {
			t.Fatalf("missing audit action %s: %+v", action, audit.events)
		}
	}
}

// TestEncryptionBenchmarkPersistenceInput 验证服务将新 Benchmark 契约映射为任务级指标，并保留接收者级封装耗时供仓储入库。
func TestEncryptionBenchmarkPersistenceInput(t *testing.T) {
	server := miniredis.RunT(t)
	serviceLayer, repositoryLayer, _, _ := newEncryptionServiceFixture(redis.NewClient(&redis.Options{Addr: server.Addr()}))
	create := encryptionCreateInput()
	create.Authorization = map[string]any{"type": "RSA_RECIPIENTS", "recipients": []any{
		map[string]any{"user_id": uint64(7), "public_key_id": "owner-key"},
		map[string]any{"user_id": uint64(9), "public_key_id": "623e4567-e89b-42d3-a456-426614174000"},
	}}
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "benchmark-contract", create)
	if err != nil {
		t.Fatal(err)
	}
	object, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("a", 64), "GCPABE01", 6, strings.NewReader("cipher"))
	if err != nil {
		t.Fatal(err)
	}
	complete := encryptionMultiRecipientCompleteInput(object.PublicID)
	complete.Benchmark.ValidationDurationMS = 2
	complete.Benchmark.FileEncryptionDurationMS = 3
	complete.Benchmark.KeyProtectionDurationMS = 9
	complete.Benchmark.UploadDurationMS = 4
	complete.Benchmark.MetadataCommitDurationMS = 5
	complete.Benchmark.TotalDurationMS = 23
	complete.Benchmark.PlaintextSizeBytes = 6
	complete.Benchmark.CiphertextSizeBytes = 6
	if _, _, err := serviceLayer.Complete(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, complete); err != nil {
		t.Fatal(err)
	}
	benchmark := repositoryLayer.lastCompletion.Benchmark
	if benchmark.ValidationDurationMS != 2 || benchmark.AESEncryptMS != 3 || benchmark.KeyProtectionDurationMS != 9 || benchmark.UploadMS != 4 || benchmark.MetadataCommitDurationMS != 5 || benchmark.TotalDurationMS != 23 || benchmark.RecipientCount != 2 || benchmark.ProtectedKeyTotalSizeBytes != 768 {
		t.Fatalf("benchmark mapping mismatch: %+v", benchmark)
	}
	for index, completion := range repositoryLayer.lastCompletion.ProtectedKeys {
		binding, ok := completion.AdapterBinding.(repository.RSAEncryptionAdapterBinding)
		if !ok || binding.Binding.ProtectDurationMS <= 0 {
			t.Fatalf("recipient %d duration missing: %+v", index, completion.AdapterBinding)
		}
	}
}

// TestEncryptionCriticalAuditsUseBusinessTransaction 验证数据库审计不会提前独立入队，创建与完成事件都交给业务仓储的同一事务提交。
func TestEncryptionCriticalAuditsUseBusinessTransaction(t *testing.T) {
	server := miniredis.RunT(t)
	serviceLayer, repositoryLayer, _, _ := newEncryptionServiceFixture(redis.NewClient(&redis.Options{Addr: server.Addr()}))
	auditRepository := &auditRepositoryOutboxStub{}
	serviceLayer.audit = NewDatabaseAuditRecorder(auditRepository)

	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "atomic-audit-create", encryptionCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(repositoryLayer.createAuditEvents) != 2 || len(auditRepository.events) != 0 {
		t.Fatalf("create audits must be transaction inputs: inputs=%d independent=%d", len(repositoryLayer.createAuditEvents), len(auditRepository.events))
	}
	for _, event := range repositoryLayer.createAuditEvents {
		if event.DedupKey == nil || *event.DedupKey == "" || event.Status != domain.AuditOutboxStatusPending {
			t.Fatalf("invalid create outbox event: %+v", event)
		}
	}

	object, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("a", 64), "GCPABE01", 6, strings.NewReader("cipher"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := serviceLayer.Complete(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, encryptionCompleteInput(object.PublicID)); err != nil {
		t.Fatal(err)
	}
	if len(repositoryLayer.lastCompletion.AuditEvents) != 3 {
		t.Fatalf("complete audits must be transaction inputs: %d", len(repositoryLayer.lastCompletion.AuditEvents))
	}
	// 上传事件属于允许降级的短事务，因此仍会独立入队；关键完成事件不应再次写入。
	if len(auditRepository.events) != 1 || auditRepository.events[0].Action != "encryption.ciphertext.upload" {
		t.Fatalf("unexpected independent audit events: %+v", auditRepository.events)
	}
}

var _ repository.EncryptionRepository = (*encryptionRepositoryStub)(nil)
