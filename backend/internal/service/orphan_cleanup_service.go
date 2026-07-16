package service

import (
	"context"
	"time"

	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/repository"
)

// CleanupResult 汇总一次可重复清理运行的非敏感结果。
type CleanupResult struct {
	Claimed        int
	Cleaned        int
	Failed         int
	ExpiredStaging int
}

// OrphanCleanupService 删除完成事务失败留下的对象，并指数退避保存失败结果。
type OrphanCleanupService struct {
	repository repository.EncryptionRepository
	storage    storage.EncryptedFileStorage
	audit      AuditRecorder
	stagingTTL time.Duration
}

// NewOrphanCleanupService 创建补偿清理服务。
func NewOrphanCleanupService(repository repository.EncryptionRepository, encryptedStorage storage.EncryptedFileStorage, audit AuditRecorder, stagingTTL time.Duration) *OrphanCleanupService {
	return &OrphanCleanupService{repository: repository, storage: encryptedStorage, audit: audit, stagingTTL: stagingTTL}
}

// Run 领取有限批次孤儿对象并清理过期暂存文件；单个对象失败不会阻断其余对象。
func (s *OrphanCleanupService) Run(ctx context.Context, limit int) (CleanupResult, error) {
	if limit <= 0 {
		limit = 100
	}
	now := time.Now()
	items, err := s.repository.ClaimOrphans(ctx, limit, now)
	if err != nil {
		return CleanupResult{}, err
	}
	result := CleanupResult{Claimed: len(items)}
	for _, item := range items {
		if err := s.storage.DeleteCiphertext(ctx, item.ObjectKey); err != nil {
			item.Status, item.LastErrorCode = "FAILED", "STORAGE_DELETE_FAILED"
			item.RetryCount++
			delay := time.Minute * time.Duration(1<<minUint32(item.RetryCount, 10))
			next := now.Add(delay)
			item.NextRetryAt = &next
			result.Failed++
			_ = s.record(ctx, item.TenantID, "FAILURE", item.LastErrorCode, item.ReasonCode)
		} else {
			item.Status, item.LastErrorCode, item.NextRetryAt = "CLEANED", "", nil
			item.RetryCount++
			item.CleanedAt = &now
			result.Cleaned++
			_ = s.record(ctx, item.TenantID, "SUCCESS", "", item.ReasonCode)
		}
		if err := s.repository.SaveOrphanResult(ctx, item); err != nil {
			return result, err
		}
	}
	deleted, err := s.storage.DeleteExpiredStaging(ctx, now.Add(-s.stagingTTL), limit)
	result.ExpiredStaging = deleted
	return result, err
}

// record 写入对象清理结果，不记录内部对象键。
func (s *OrphanCleanupService) record(ctx context.Context, tenantID uint64, result, errorCode, reasonCode string) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Record(ctx, AuditEvent{TenantID: &tenantID, Action: "encryption.storage.cleanup", TargetType: "storage_object", Result: result, SourceTrust: "SERVER_OBSERVED", ErrorCode: errorCode, Metadata: map[string]any{"reason_code": reasonCode}})
}

// minUint32 为退避指数设置上限，避免位移和时长溢出。
func minUint32(value, maximum uint32) uint32 {
	if value < maximum {
		return value
	}
	return maximum
}
