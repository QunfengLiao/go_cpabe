package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/repository"
)

// importBatchMemoryRepository 只实现异步状态测试涉及的方法，其余接口由嵌入占位且不会被调用。
type importBatchMemoryRepository struct {
	repository.ImportRepository
	batch        domain.TenantImportBatch
	enqueueCount int
	expireCount  int
	failCount    int
	saveCount    int
}

// FindImportBatch 返回内存批次副本，模拟租户和操作者已经通过仓储隔离条件。
func (r *importBatchMemoryRepository) FindImportBatch(_ context.Context, _, _ uint64, _ string) (*domain.TenantImportBatch, error) {
	copy := r.batch
	return &copy, nil
}

// FindImportBatchStatus 返回不含逐行快照的批次元数据，模拟确认接口的轻量查询。
func (r *importBatchMemoryRepository) FindImportBatchStatus(_ context.Context, _, _ uint64, _ string) (*domain.TenantImportBatch, error) {
	copy := r.batch
	copy.RowsJSON = nil
	copy.SnapshotHash = ""
	return &copy, nil
}

// SaveImportBatch 保存测试中的过期状态转换，不产生数据库副作用。
func (r *importBatchMemoryRepository) SaveImportBatch(_ context.Context, batch *domain.TenantImportBatch) error {
	r.batch = *batch
	r.saveCount++
	return nil
}

// ExpireImportBatch 模拟只更新状态的过期操作，确保轻量元数据不会覆盖测试批次快照。
func (r *importBatchMemoryRepository) ExpireImportBatch(_ context.Context, _, _ uint64, _ string, _ time.Time) error {
	r.batch.Status = domain.ImportBatchExpired
	r.expireCount++
	return nil
}

// EnqueueImportBatch 模拟条件更新，确保同一已校验批次只能首次进入队列。
func (r *importBatchMemoryRepository) EnqueueImportBatch(_ context.Context, _, _ uint64, _ string, confirmedAt time.Time) error {
	if r.batch.Status != domain.ImportBatchValidated {
		return repository.ErrImportBatchState
	}
	r.batch.Status = domain.ImportBatchQueued
	r.batch.Phase = "WAITING"
	r.batch.ConfirmedAt = &confirmedAt
	r.enqueueCount++
	return nil
}

// FailImportBatch 记录 Worker 的脱敏失败终态，验证快照错误不会进入业务事务。
func (r *importBatchMemoryRepository) FailImportBatch(_ context.Context, _, _, reason string, failureCount int, now time.Time) error {
	r.batch.Status = domain.ImportBatchFailed
	r.batch.FailureReason = reason
	r.batch.FailureCount = failureCount
	r.batch.CompletedAt = &now
	r.failCount++
	return nil
}

// TestImportConfirmIsIdempotentQueue 验证重复确认只执行一次持久化入队，后续直接返回同一批次状态。
func TestImportConfirmIsIdempotentQueue(t *testing.T) {
	rows := []domain.ImportRowResult{{RowNumber: 4, Key: "demo.user", Action: domain.ImportRowCreate, Status: domain.ImportRowValid, Fields: map[string]string{"username": "demo.user"}}}
	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	validatedAt := time.Now()
	repo := &importBatchMemoryRepository{batch: domain.TenantImportBatch{BatchID: "batch-1", TenantID: 7, CreatedBy: 9, ImportType: domain.ImportTypeUsers, Status: domain.ImportBatchValidated, ValidatedAt: &validatedAt, RowsJSON: encoded, SnapshotHash: sha256Hex(encoded)}}
	service := NewImportService(nil, repo, nil, nil, 1024, 10000, time.Hour, "")

	first, err := service.Confirm(context.Background(), 7, 9, domain.ImportTypeUsers, "batch-1")
	if err != nil || first.Status != domain.ImportBatchQueued {
		t.Fatalf("首次确认结果=%+v，错误=%v", first, err)
	}
	second, err := service.Confirm(context.Background(), 7, 9, domain.ImportTypeUsers, "batch-1")
	if err != nil || second.Status != domain.ImportBatchQueued || repo.enqueueCount != 1 {
		t.Fatalf("重复确认结果=%+v，入队次数=%d，错误=%v", second, repo.enqueueCount, err)
	}
}

// TestImportConfirmExpiresWithTargetedUpdate 验证轻量确认发现过期时只更新状态，不会用未加载字段覆盖完整快照。
func TestImportConfirmExpiresWithTargetedUpdate(t *testing.T) {
	validatedAt := time.Now().Add(-2 * time.Hour)
	originalRows := []byte(`[{"row_number":4}]`)
	repo := &importBatchMemoryRepository{batch: domain.TenantImportBatch{BatchID: "batch-expired", TenantID: 7, CreatedBy: 9, ImportType: domain.ImportTypeUsers, Status: domain.ImportBatchValidated, ValidatedAt: &validatedAt, RowsJSON: originalRows}}
	service := NewImportService(nil, repo, nil, nil, 1024, 10000, time.Hour, "")

	_, err := service.Confirm(context.Background(), 7, 9, domain.ImportTypeUsers, "batch-expired")
	if err != ErrImportBatchExpired {
		t.Fatalf("过期批次应返回 ErrImportBatchExpired，实际=%v", err)
	}
	if repo.expireCount != 1 || repo.saveCount != 0 || string(repo.batch.RowsJSON) != string(originalRows) {
		t.Fatalf("过期更新不应覆盖快照: expire=%d save=%d batch=%+v", repo.expireCount, repo.saveCount, repo.batch)
	}
}

// TestImportWorkerRejectsTamperedSnapshot 验证 Worker 在事务前拒绝被篡改快照并保存稳定失败原因。
func TestImportWorkerRejectsTamperedSnapshot(t *testing.T) {
	repo := &importBatchMemoryRepository{}
	service := NewImportService(nil, repo, nil, nil, 1024, 10000, time.Hour, "")
	worker := NewImportWorker(service, ImportWorkerConfig{Lease: time.Minute})
	batch := &domain.TenantImportBatch{BatchID: "batch-2", TenantID: 7, CreatedBy: 9, ImportType: domain.ImportTypeUsers, Status: domain.ImportBatchImporting, LeaseToken: "lease-1", RowsJSON: []byte(`[]`), SnapshotHash: "tampered", TotalCount: 10000}

	if err := worker.execute(context.Background(), batch); err != nil {
		t.Fatalf("保存失败终态不应再返回错误: %v", err)
	}
	if repo.failCount != 1 || repo.batch.Status != domain.ImportBatchFailed || repo.batch.FailureReason != "批次快照校验失败" {
		t.Fatalf("失败终态不正确: %+v", repo.batch)
	}
}

// TestDecodeAndVerifyImportRowsAcceptsNormalizedJSON 验证数据库改变 JSON 键顺序后，语义未变的可信快照不会被误判为篡改。
func TestDecodeAndVerifyImportRowsAcceptsNormalizedJSON(t *testing.T) {
	rows := []domain.ImportRowResult{{RowNumber: 4, Key: "demo.user", Action: domain.ImportRowCreate, Status: domain.ImportRowValid, Fields: map[string]string{"username": "demo.user", "role_code": "DU"}}}
	canonical, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var normalizedValue any
	if err := json.Unmarshal(canonical, &normalizedValue); err != nil {
		t.Fatal(err)
	}
	normalized, err := json.Marshal(normalizedValue)
	if err != nil {
		t.Fatal(err)
	}
	if string(normalized) == string(canonical) {
		t.Fatal("测试前提不成立：规范化 JSON 应改变结构体字段顺序")
	}

	decoded, err := decodeAndVerifyImportRows(normalized, sha256Hex(canonical))
	if err != nil {
		t.Fatalf("语义相同的规范化 JSON 不应被拒绝: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Fields["role_code"] != "DU" {
		t.Fatalf("规范化快照解码结果不正确: %+v", decoded)
	}
}
