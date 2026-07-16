package repository

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"go-cpabe/backend/internal/domain"
)

// TestValidProgressTransition 验证状态图只允许保持或前进一个阶段，不能跳阶段或从终态推进。
func TestValidProgressTransition(t *testing.T) {
	if !validProgressTransition(domain.EncryptionPending, domain.EncryptionValidating) {
		t.Fatal("pending should enter validating")
	}
	if !validProgressTransition(domain.EncryptionEncryptingFile, domain.EncryptionEncryptingFile) {
		t.Fatal("same-stage byte progress should be allowed")
	}
	if validProgressTransition(domain.EncryptionPending, domain.EncryptionUploading) {
		t.Fatal("stage jump must fail")
	}
	if validProgressTransition(domain.EncryptionCompleted, domain.EncryptionSavingMetadata) {
		t.Fatal("terminal state must not advance")
	}
}

// TestEncryptionRepositoryContainsIdempotencyAndRowLocks 验证任务仓储保留租户幂等键和 UPDATE 行锁。
func TestEncryptionRepositoryContainsIdempotencyAndRowLocks(t *testing.T) {
	content, err := os.ReadFile("encryption_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(content)
	for _, required := range []string{"tenant_id = ? AND owner_user_id = ? AND idempotency_key = ?", `clause.Locking{Strength: "UPDATE"}`, "tenant_id = ? AND owner_user_id = ? AND public_id = ?"} {
		if !strings.Contains(source, required) {
			t.Fatalf("missing repository safety boundary %s", required)
		}
	}
}

// TestEncryptionRepositoryPersistsMultipleProtectedKeys 验证完成事务为每个接收者各写一条 protected key 和 RSA binding，且正确传递文件、执行与接收者耗时。
func TestEncryptionRepositoryPersistsMultipleProtectedKeys(t *testing.T) {
	writer := &completionRowWriterStub{}
	items := []ProtectedKeyCompletion{
		{ProtectedKey: domain.ProtectedKey{PublicID: "pk-owner"}, AdapterBinding: RSAEncryptionAdapterBinding{Binding: domain.RSAProtectedKeyBinding{RecipientUserID: 7, RSAPublicKeyID: 31, ProtectDurationMS: 4}}},
		{ProtectedKey: domain.ProtectedKey{PublicID: "pk-user"}, AdapterBinding: RSAEncryptionAdapterBinding{Binding: domain.RSAProtectedKeyBinding{RecipientUserID: 9, RSAPublicKeyID: 32, ProtectDurationMS: 6}}},
	}
	persisted, err := persistProtectedKeyRows(writer, 3, 11, 13, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(writer.protectedKeys) != 2 || len(writer.bindings) != 2 || len(persisted) != 2 {
		t.Fatalf("unexpected writes protected=%+v bindings=%+v", writer.protectedKeys, writer.bindings)
	}
	for index := range writer.bindings {
		if writer.protectedKeys[index].TenantID != 3 || writer.protectedKeys[index].FileID != 11 || writer.protectedKeys[index].TaskAttemptID != 13 {
			t.Fatalf("protected key scope mismatch: %+v", writer.protectedKeys[index])
		}
		if writer.bindings[index].FileID != 11 || writer.bindings[index].ProtectedKeyID != writer.protectedKeys[index].ID {
			t.Fatalf("binding relation mismatch: %+v", writer.bindings[index])
		}
	}
}

// TestBenchmarkSummaryFromPersistedRow 验证详情查询层完整映射任务级 Benchmark，并按接收者数量计算平均封装耗时。
func TestBenchmarkSummaryFromPersistedRow(t *testing.T) {
	summary := benchmarkSummaryFromRow(fileCenterBenchmarkRow{PlaintextSize: 6, CiphertextSize: 18, ProtectedKeyTotalSizeBytes: 768, AESEncryptMS: 3, KeyProtectionDurationMS: 10, UploadMS: 4, MetadataCommitDurationMS: 5, TotalDurationMS: 24, RecipientCount: 2})
	if summary.AESEncryptMS != 3 || summary.DEKProtectMS != 10 || summary.AverageRecipientProtectMS != 5 || summary.UploadMS != 4 || summary.MetadataCommitMS != 5 || summary.TotalMS != 24 || summary.RecipientCount != 2 || summary.ProtectedKeyTotalSizeBytes != 768 {
		t.Fatalf("benchmark detail mismatch: %+v", summary)
	}
}

// completionRowWriterStub 模拟事务内实体写入并回填主键，用来验证真实完成编排而不是扫描实现源码。
type completionRowWriterStub struct {
	protectedKeys []domain.ProtectedKey
	bindings      []domain.RSAProtectedKeyBinding
}

// Create 保存支持的完成实体；未知类型立即失败，防止测试静默漏掉新增持久化分支。
func (w *completionRowWriterStub) Create(value any) error {
	switch row := value.(type) {
	case *domain.ProtectedKey:
		row.ID = uint64(100 + len(w.protectedKeys))
		w.protectedKeys = append(w.protectedKeys, *row)
	case *domain.RSAProtectedKeyBinding:
		w.bindings = append(w.bindings, *row)
	default:
		return fmt.Errorf("unexpected completion row %T", value)
	}
	return nil
}

// TestTerminalStatusClassification 验证完成、失败和取消是唯一终态。
func TestTerminalStatusClassification(t *testing.T) {
	for _, status := range []domain.EncryptionTaskStatus{domain.EncryptionCompleted, domain.EncryptionFailed, domain.EncryptionCancelled} {
		if !isTerminalStatus(status) {
			t.Fatalf("%s should be terminal", status)
		}
	}
	if isTerminalStatus(domain.EncryptionUploading) {
		t.Fatal("uploading must not be terminal")
	}
}
