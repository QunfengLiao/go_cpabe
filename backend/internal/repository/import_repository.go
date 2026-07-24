package repository

import (
	"context"
	"errors"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/identifier"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var (
	// ErrImportBatchNotFound 表示批次不属于当前租户或不存在，调用方不应暴露更细的枚举信息。
	ErrImportBatchNotFound = errors.New("import batch not found")
	// ErrImportBatchState 表示批次状态不允许当前生命周期操作。
	ErrImportBatchState = errors.New("invalid import batch state")
)

// ImportRepository 定义导入批次的租户隔离持久化能力。
type ImportRepository interface {
	CreateImportBatch(ctx context.Context, batch *domain.TenantImportBatch) error
	FindImportBatch(ctx context.Context, tenantID, actorID uint64, batchID string) (*domain.TenantImportBatch, error)
	FindImportBatchStatus(ctx context.Context, tenantID, actorID uint64, batchID string) (*domain.TenantImportBatch, error)
	ListImportBatches(ctx context.Context, tenantID, actorID uint64) ([]domain.TenantImportBatch, error)
	SaveImportBatch(ctx context.Context, batch *domain.TenantImportBatch) error
	ExpireImportBatch(ctx context.Context, tenantID, actorID uint64, batchID string, expiredAt time.Time) error
	EnqueueImportBatch(ctx context.Context, tenantID, actorID uint64, batchID string, confirmedAt time.Time) error
	ClaimNextImportBatch(ctx context.Context, now time.Time, lease time.Duration) (*domain.TenantImportBatch, error)
	UpdateImportProgress(ctx context.Context, batchID, leaseToken, phase string, processed int, now time.Time, lease time.Duration) error
	CompleteImportBatch(ctx context.Context, batchID, leaseToken string, successCount, skippedCount int, now time.Time) error
	FailImportBatch(ctx context.Context, batchID, leaseToken, reason string, failureCount int, now time.Time) error
	Transaction(ctx context.Context, fn func(*gorm.DB) error) error
}

// GormImportRepository 使用 Gorm 保存批次快照，任何查询都显式带租户和操作者条件。
type GormImportRepository struct {
	db *gorm.DB
}

// importBatchClaimCandidate 是领取事务的窄行投影；禁止加入 rows_json 等大字段，否则 MySQL filesort 可能耗尽排序内存。
type importBatchClaimCandidate struct {
	ID uint64
}

// NewGormImportRepository 创建导入批次仓储。
func NewGormImportRepository(db *gorm.DB) *GormImportRepository {
	return &GormImportRepository{db: db}
}

// CreateImportBatch 保存预校验批次，rows_json 是后端生成的可信快照。
func (r *GormImportRepository) CreateImportBatch(ctx context.Context, batch *domain.TenantImportBatch) error {
	return r.db.WithContext(ctx).Create(batch).Error
}

// FindImportBatch 仅按当前租户和当前操作者读取批次，阻止 batch_id 横向枚举。
func (r *GormImportRepository) FindImportBatch(ctx context.Context, tenantID, actorID uint64, batchID string) (*domain.TenantImportBatch, error) {
	var batch domain.TenantImportBatch
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND created_by = ? AND batch_id = ?", tenantID, actorID, batchID).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrImportBatchNotFound
	}
	return &batch, err
}

// FindImportBatchStatus 只读取状态轮询和确认所需字段，避免 1 万行批次请求加载 rows_json。
func (r *GormImportRepository) FindImportBatchStatus(ctx context.Context, tenantID, actorID uint64, batchID string) (*domain.TenantImportBatch, error) {
	var batch domain.TenantImportBatch
	err := r.db.WithContext(ctx).Select("batch_id,tenant_id,created_by,import_type,file_hash,total_count,valid_count,processed_count,success_count,failure_count,skipped_count,status,phase,attempt_count,failure_reason,validated_at,confirmed_at,completed_at").Where("tenant_id = ? AND created_by = ? AND batch_id = ?", tenantID, actorID, batchID).Take(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrImportBatchNotFound
	}
	return &batch, err
}

// ListImportBatches 返回当前租户且由当前操作者创建的批次，避免管理接口泄露其他用户快照。
func (r *GormImportRepository) ListImportBatches(ctx context.Context, tenantID, actorID uint64) ([]domain.TenantImportBatch, error) {
	var batches []domain.TenantImportBatch
	err := r.db.WithContext(ctx).Select("id,batch_id,tenant_id,import_type,file_name,total_count,valid_count,success_count,failure_count,skipped_count,status,created_by,validated_at,confirmed_at,completed_at,failure_reason,created_at,updated_at").Where("tenant_id = ? AND created_by = ?", tenantID, actorID).Order("created_at DESC").Find(&batches).Error
	return batches, err
}

// SaveImportBatch 更新批次生命周期和统计，调用方必须已经完成状态转换校验。
func (r *GormImportRepository) SaveImportBatch(ctx context.Context, batch *domain.TenantImportBatch) error {
	return r.db.WithContext(ctx).Save(batch).Error
}

// ExpireImportBatch 以条件更新标记预校验批次过期，避免把轻量查询中未加载的快照字段覆盖为空值。
func (r *GormImportRepository) ExpireImportBatch(ctx context.Context, tenantID, actorID uint64, batchID string, expiredAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&domain.TenantImportBatch{}).Where("tenant_id = ? AND created_by = ? AND batch_id = ? AND status = ?", tenantID, actorID, batchID, domain.ImportBatchValidated).Updates(map[string]any{"status": domain.ImportBatchExpired, "updated_at": expiredAt})
	return result.Error
}

// EnqueueImportBatch 原子地把已校验批次写入持久化队列，重复点击不会生成第二个任务。
func (r *GormImportRepository) EnqueueImportBatch(ctx context.Context, tenantID, actorID uint64, batchID string, confirmedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&domain.TenantImportBatch{}).Where("tenant_id = ? AND created_by = ? AND batch_id = ? AND status = ?", tenantID, actorID, batchID, domain.ImportBatchValidated).Updates(map[string]any{"status": domain.ImportBatchQueued, "phase": "WAITING", "processed_count": 0, "confirmed_at": confirmedAt, "updated_at": confirmedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrImportBatchState
	}
	return nil
}

// ClaimNextImportBatch 使用行锁和租约领取一个待执行批次；过期租约允许其他实例安全接管。
func (r *GormImportRepository) ClaimNextImportBatch(ctx context.Context, now time.Time, lease time.Duration) (*domain.TenantImportBatch, error) {
	var claimedID uint64
	var claimedToken string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidate importBatchClaimCandidate
		// 空队列是 Worker 的正常状态；Find 不产生 ErrRecordNotFound，并仅对这条高频探测关闭 SQL 日志。
		// 这里只排序并锁定主键，避免让万行 rows_json 进入 MySQL filesort；完整快照在租约提交后按主键读取。
		selector := tx.Session(&gorm.Session{Logger: tx.Logger.LogMode(logger.Silent)})
		selectResult := importBatchClaimCandidateQuery(selector, now).Find(&candidate)
		if selectResult.Error != nil {
			return selectResult.Error
		}
		if selectResult.RowsAffected == 0 {
			return nil
		}
		token, err := identifier.NewUUID()
		if err != nil {
			return err
		}
		expires := now.Add(lease)
		result := tx.Model(&domain.TenantImportBatch{}).Where("id = ? AND (status = ? OR (status = ? AND lease_expires_at < ?))", candidate.ID, domain.ImportBatchQueued, domain.ImportBatchImporting, now).Updates(map[string]any{"status": domain.ImportBatchImporting, "phase": "PREPARING", "lease_token": token, "lease_expires_at": expires, "heartbeat_at": now, "attempt_count": gorm.Expr("attempt_count + 1"), "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrImportBatchState
		}
		claimedID = candidate.ID
		claimedToken = token
		return nil
	})
	if err != nil || claimedID == 0 {
		return nil, err
	}

	// 租约提交后再按主键加载完整可信快照，缩短行锁事务并让 JSON 大字段完全绕开排序路径。
	var claimed domain.TenantImportBatch
	err = r.db.WithContext(ctx).Where("id = ? AND status = ? AND lease_token = ?", claimedID, domain.ImportBatchImporting, claimedToken).Take(&claimed).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrImportBatchState
	}
	return &claimed, err
}

// importBatchClaimCandidateQuery 构造只选择主键的待领取查询，调用方必须在事务中追加 Find 才能持有行锁。
func importBatchClaimCandidateQuery(db *gorm.DB, now time.Time) *gorm.DB {
	return db.Model(&domain.TenantImportBatch{}).Select("id").Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status = ? OR (status = ? AND lease_expires_at < ?)", domain.ImportBatchQueued, domain.ImportBatchImporting, now).Order("confirmed_at ASC, id ASC").Limit(1)
}

// UpdateImportProgress 仅允许当前租约持有者刷新进度和心跳，避免失效 Worker 覆盖新执行者状态。
func (r *GormImportRepository) UpdateImportProgress(ctx context.Context, batchID, leaseToken, phase string, processed int, now time.Time, lease time.Duration) error {
	result := r.db.WithContext(ctx).Model(&domain.TenantImportBatch{}).Where("batch_id = ? AND status = ? AND lease_token = ?", batchID, domain.ImportBatchImporting, leaseToken).Updates(map[string]any{"phase": phase, "processed_count": processed, "heartbeat_at": now, "lease_expires_at": now.Add(lease), "updated_at": now})
	return importLeaseResult(result)
}

// CompleteImportBatch 以租约令牌提交终态，确保过期 Worker 无法把新执行结果覆盖掉。
func (r *GormImportRepository) CompleteImportBatch(ctx context.Context, batchID, leaseToken string, successCount, skippedCount int, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&domain.TenantImportBatch{}).Where("batch_id = ? AND status = ? AND lease_token = ?", batchID, domain.ImportBatchImporting, leaseToken).Updates(map[string]any{"status": domain.ImportBatchSucceeded, "phase": "COMPLETED", "processed_count": successCount + skippedCount, "success_count": successCount, "skipped_count": skippedCount, "failure_count": 0, "completed_at": now, "lease_token": "", "lease_expires_at": nil, "heartbeat_at": now, "updated_at": now})
	return importLeaseResult(result)
}

// FailImportBatch 保存脱敏失败原因并释放租约；业务事务已经回滚，不会留下半批数据。
func (r *GormImportRepository) FailImportBatch(ctx context.Context, batchID, leaseToken, reason string, failureCount int, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&domain.TenantImportBatch{}).Where("batch_id = ? AND status = ? AND lease_token = ?", batchID, domain.ImportBatchImporting, leaseToken).Updates(map[string]any{"status": domain.ImportBatchFailed, "phase": "FAILED", "failure_count": failureCount, "failure_reason": reason, "completed_at": now, "lease_token": "", "lease_expires_at": nil, "heartbeat_at": now, "updated_at": now})
	return importLeaseResult(result)
}

// importLeaseResult 把条件更新未命中统一转换成状态冲突，调用方据此停止失效 Worker。
func importLeaseResult(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrImportBatchState
	}
	return nil
}

// Transaction 在同一数据库事务中执行正式导入，禁止调用方在行循环内再次开启事务。
func (r *GormImportRepository) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}
