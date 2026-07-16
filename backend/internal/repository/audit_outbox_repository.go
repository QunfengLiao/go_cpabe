package repository

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/identifier"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrAuditOutboxLeaseLost 表示事件租约已经过期或被其他 Dispatcher 接管，旧 Worker 不得更新状态。
	ErrAuditOutboxLeaseLost = errors.New("audit outbox lease lost")
	// ErrAuditOutboxConflict 表示唯一键指向的既有事件与本次业务事实不一致，调用方必须回滚而不能吞掉审计。
	ErrAuditOutboxConflict = errors.New("audit outbox idempotency conflict")
)

// AuditOutboxRepository 定义审计事件入队、租约领取、幂等投递和失败退避能力。
type AuditOutboxRepository interface {
	// Enqueue 使用独立短事务写入无业务事务可复用的事件。
	Enqueue(ctx context.Context, event domain.AuditOutboxEvent) (domain.AuditOutboxEvent, bool, error)
	// EnqueueWithDB 使用调用方事务原子写入关键业务事件。
	EnqueueWithDB(ctx context.Context, db *gorm.DB, event domain.AuditOutboxEvent) (domain.AuditOutboxEvent, bool, error)
	// Claim 领取到期或租约过期事件，并为每条事件分配新令牌。
	Claim(ctx context.Context, limit int, now time.Time, lease time.Duration) ([]domain.AuditOutboxEvent, error)
	// Deliver 只按数据库中的不可变事件正文幂等创建正式日志。
	Deliver(ctx context.Context, event domain.AuditOutboxEvent, lockToken string, deliveredAt time.Time) error
	// MarkFailure 以租约 CAS 记录稳定错误码，并推进重试或死信状态。
	MarkFailure(ctx context.Context, event domain.AuditOutboxEvent, lockToken, errorCode string, nextRetryAt time.Time, maxRetries uint32) error
	// DeleteDeliveredBefore 分批清理超过保留期的已投递事件。
	DeleteDeliveredBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error)
}

// GormAuditOutboxRepository 使用 MySQL 行锁和唯一键实现多实例安全的审计投递队列。
type GormAuditOutboxRepository struct{ db *gorm.DB }

// NewGormAuditOutboxRepository 创建独立审计 Outbox 仓储；该仓储永不访问文件孤儿对象表。
func NewGormAuditOutboxRepository(db *gorm.DB) *GormAuditOutboxRepository {
	return &GormAuditOutboxRepository{db: db}
}

// Enqueue 在独立短事务中幂等写入已脱敏事件，适用于下载、拒绝和进度等没有业务写事务的事实。
func (r *GormAuditOutboxRepository) Enqueue(ctx context.Context, event domain.AuditOutboxEvent) (domain.AuditOutboxEvent, bool, error) {
	return r.EnqueueWithDB(ctx, r.db, event)
}

// EnqueueWithDB 使用调用方传入的事务句柄入队；业务事实回滚时 outbox 必须随同回滚。
func (r *GormAuditOutboxRepository) EnqueueWithDB(ctx context.Context, db *gorm.DB, event domain.AuditOutboxEvent) (domain.AuditOutboxEvent, bool, error) {
	if db == nil {
		return domain.AuditOutboxEvent{}, false, errors.New("audit outbox database is required")
	}
	if err := validatePreparedAuditOutboxEvent(event); err != nil {
		return domain.AuditOutboxEvent{}, false, err
	}
	result := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
	if result.Error != nil {
		return domain.AuditOutboxEvent{}, false, result.Error
	}
	if result.RowsAffected == 1 {
		return event, false, nil
	}
	existing, err := loadMatchingAuditOutboxEvent(ctx, db, event)
	if err != nil {
		return domain.AuditOutboxEvent{}, false, err
	}
	return existing, true, nil
}

// loadMatchingAuditOutboxEvent 分别核对事件 UUID 和业务去重键，禁止两个键命中不同记录或相同键复用到不同事件正文。
func loadMatchingAuditOutboxEvent(ctx context.Context, db *gorm.DB, event domain.AuditOutboxEvent) (domain.AuditOutboxEvent, error) {
	var byPublicID domain.AuditOutboxEvent
	publicIDFound, err := findAuditOutboxEvent(db.WithContext(ctx).Where("event_public_id = ?", event.EventPublicID), &byPublicID)
	if err != nil {
		return domain.AuditOutboxEvent{}, err
	}
	var byDedup domain.AuditOutboxEvent
	dedupFound := false
	if event.DedupKey != nil {
		dedupFound, err = findAuditOutboxEvent(db.WithContext(ctx).Where("dedup_key = ?", *event.DedupKey), &byDedup)
		if err != nil {
			return domain.AuditOutboxEvent{}, err
		}
	}
	if publicIDFound && dedupFound && byPublicID.ID != byDedup.ID {
		return domain.AuditOutboxEvent{}, ErrAuditOutboxConflict
	}
	existing := byPublicID
	if !publicIDFound {
		if !dedupFound {
			return domain.AuditOutboxEvent{}, ErrAuditOutboxConflict
		}
		existing = byDedup
	}
	if !sameAuditOutboxPayload(existing, event) || publicIDFound && !sameOptionalString(existing.DedupKey, event.DedupKey) {
		return domain.AuditOutboxEvent{}, ErrAuditOutboxConflict
	}
	return existing, nil
}

// findAuditOutboxEvent 将未命中与数据库故障分开返回，避免唯一冲突后的回查把真实错误误判为幂等。
func findAuditOutboxEvent(query *gorm.DB, target *domain.AuditOutboxEvent) (bool, error) {
	err := query.First(target).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

// sameAuditOutboxPayload 比较业务事实的安全正文；事件 UUID 和发生时间允许在业务去重重试时不同，其他字段必须完全一致。
func sameAuditOutboxPayload(left, right domain.AuditOutboxEvent) bool {
	return sameOptionalUint64(left.TenantID, right.TenantID) && sameOptionalUint64(left.ActorUserID, right.ActorUserID) &&
		left.Action == right.Action && left.TargetType == right.TargetType && left.TargetPublicID == right.TargetPublicID &&
		left.Result == right.Result && left.SourceTrust == right.SourceTrust && left.ErrorCode == right.ErrorCode && left.RequestID == right.RequestID &&
		left.MetadataRedacted == right.MetadataRedacted && left.PayloadVersion == right.PayloadVersion && reflect.DeepEqual(left.MetadataJSON, right.MetadataJSON)
}

// sameOptionalUint64 比较可空租户或操作者主键，nil 与零值指针具有不同安全语义。
func sameOptionalUint64(left, right *uint64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

// sameOptionalString 比较可空业务去重键，禁止同一事件 UUID 在重试时更换幂等身份。
func sameOptionalString(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

// Claim 领取到期或租约过期事件并逐条写入随机锁令牌；SKIP LOCKED 允许多个 Dispatcher 并行处理不同事件。
func (r *GormAuditOutboxRepository) Claim(ctx context.Context, limit int, now time.Time, lease time.Duration) ([]domain.AuditOutboxEvent, error) {
	if limit <= 0 {
		return nil, nil
	}
	if lease <= 0 {
		return nil, errors.New("audit outbox lease must be positive")
	}
	var claimed []domain.AuditOutboxEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		leaseExpiredAt := now.Add(-lease)
		var candidates []domain.AuditOutboxEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("((status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND (locked_at IS NULL OR locked_at <= ?)))", []domain.AuditOutboxStatus{domain.AuditOutboxStatusPending, domain.AuditOutboxStatusRetry}, now, domain.AuditOutboxStatusProcessing, leaseExpiredAt).
			Order("id ASC").Limit(limit).Find(&candidates).Error; err != nil {
			return err
		}
		claimed = make([]domain.AuditOutboxEvent, 0, len(candidates))
		for _, event := range candidates {
			token, err := newAuditLockToken()
			if err != nil {
				return err
			}
			updates := map[string]any{"status": domain.AuditOutboxStatusProcessing, "locked_at": now, "lock_token": token, "updated_at": now}
			if err := tx.Model(&domain.AuditOutboxEvent{}).Where("id = ?", event.ID).Updates(updates).Error; err != nil {
				return err
			}
			event.Status, event.LockedAt, event.LockToken = domain.AuditOutboxStatusProcessing, &now, &token
			claimed = append(claimed, event)
		}
		return nil
	})
	return claimed, err
}

// Deliver 在一个事务中幂等创建正式日志并以租约 CAS 标记投递完成，覆盖“日志已写、状态未改”的崩溃窗口。
func (r *GormAuditOutboxRepository) Deliver(ctx context.Context, event domain.AuditOutboxEvent, lockToken string, deliveredAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var persisted domain.AuditOutboxEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ? AND lock_token = ?", event.ID, domain.AuditOutboxStatusProcessing, lockToken).First(&persisted).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAuditOutboxLeaseLost
			}
			return err
		}
		if err := validatePreparedAuditOutboxEvent(persisted); err != nil {
			return err
		}
		log := domain.AuditLog{PublicID: persisted.EventPublicID, TenantID: persisted.TenantID, ActorUserID: persisted.ActorUserID, Action: persisted.Action, TargetType: persisted.TargetType, TargetPublicID: persisted.TargetPublicID, Result: persisted.Result, SourceTrust: persisted.SourceTrust, ErrorCode: persisted.ErrorCode, RequestID: persisted.RequestID, MetadataJSON: append([]byte(nil), persisted.MetadataJSON...), MetadataRedacted: persisted.MetadataRedacted, CreatedAt: persisted.OccurredAt}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "public_id"}}, DoNothing: true}).Create(&log).Error; err != nil {
			return err
		}
		result := tx.Model(&domain.AuditOutboxEvent{}).Where("id = ? AND status = ? AND lock_token = ?", persisted.ID, domain.AuditOutboxStatusProcessing, lockToken).Updates(map[string]any{"status": domain.AuditOutboxStatusDelivered, "delivered_at": deliveredAt, "locked_at": nil, "lock_token": nil, "last_error_code": "", "updated_at": deliveredAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAuditOutboxLeaseLost
		}
		return nil
	})
}

// MarkFailure 使用锁令牌更新退避或死信状态；只保存稳定错误码，原始数据库错误不得入库。
func (r *GormAuditOutboxRepository) MarkFailure(ctx context.Context, event domain.AuditOutboxEvent, lockToken, errorCode string, nextRetryAt time.Time, maxRetries uint32) error {
	if maxRetries == 0 || len(errorCode) == 0 || len(errorCode) > 64 || !auditCodePattern.MatchString(errorCode) || nextRetryAt.IsZero() {
		return errors.New("invalid audit outbox failure transition")
	}
	retryCount := event.RetryCount + 1
	status := domain.AuditOutboxStatusRetry
	var next any = nextRetryAt
	if retryCount >= maxRetries {
		status = domain.AuditOutboxStatusDeadLetter
		next = nil
	}
	result := r.db.WithContext(ctx).Model(&domain.AuditOutboxEvent{}).Where("id = ? AND status = ? AND lock_token = ?", event.ID, domain.AuditOutboxStatusProcessing, lockToken).Updates(map[string]any{"status": status, "retry_count": retryCount, "next_retry_at": next, "locked_at": nil, "lock_token": nil, "last_error_code": errorCode, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrAuditOutboxLeaseLost
	}
	return nil
}

// DeleteDeliveredBefore 分批删除超过保留期的已投递事件；死信和待处理事件永远不在该清理范围内。
func (r *GormAuditOutboxRepository) DeleteDeliveredBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	var ids []uint64
	if err := r.db.WithContext(ctx).Model(&domain.AuditOutboxEvent{}).Where("status = ? AND delivered_at < ?", domain.AuditOutboxStatusDelivered, cutoff).Order("id ASC").Limit(limit).Pluck("id", &ids).Error; err != nil || len(ids) == 0 {
		return 0, err
	}
	result := r.db.WithContext(ctx).Where("id IN ? AND status = ?", ids, domain.AuditOutboxStatusDelivered).Delete(&domain.AuditOutboxEvent{})
	return result.RowsAffected, result.Error
}

// newAuditLockToken 生成不可预测 UUID 作为一次处理租约身份，避免旧 Worker 误提交新租约结果。
func newAuditLockToken() (string, error) {
	return identifier.NewUUID()
}

// validatePreparedAuditOutboxEvent 对入队和投递执行同一纵深校验，防止内部调用绕过规范化器写入秘密或畸形状态。
func validatePreparedAuditOutboxEvent(event domain.AuditOutboxEvent) error {
	if !auditUUIDPattern.MatchString(event.EventPublicID) || strings.TrimSpace(event.Action) == "" || len(event.Action) > 128 || strings.TrimSpace(event.TargetType) == "" || len(event.TargetType) > 64 || event.PayloadVersion != 1 || event.OccurredAt.IsZero() {
		return errors.New("invalid prepared audit outbox event")
	}
	if event.Result != "SUCCESS" && event.Result != "FAILURE" && event.Result != "DENIED" {
		return errors.New("invalid prepared audit result")
	}
	if event.SourceTrust != "SERVER_OBSERVED" && event.SourceTrust != "CLIENT_REPORTED" {
		return errors.New("invalid prepared audit source trust")
	}
	if event.Status != domain.AuditOutboxStatusPending && event.Status != domain.AuditOutboxStatusProcessing && event.Status != domain.AuditOutboxStatusRetry {
		return errors.New("invalid prepared audit status")
	}
	if event.DedupKey != nil && (strings.TrimSpace(*event.DedupKey) == "" || len(*event.DedupKey) > 128) {
		return errors.New("invalid prepared audit dedup key")
	}
	var metadata map[string]any
	if len(event.MetadataJSON) == 0 || len(event.MetadataJSON) > maxAuditMetadataBytes || json.Unmarshal(event.MetadataJSON, &metadata) != nil {
		return ErrAuditSensitiveMetadata
	}
	if event.MetadataRedacted {
		if len(metadata) != 0 {
			return ErrAuditSensitiveMetadata
		}
		return nil
	}
	encoded, redacted := PrepareAuditMetadata(event.Action, metadata)
	if redacted {
		return ErrAuditSensitiveMetadata
	}
	var normalized map[string]any
	if json.Unmarshal(encoded, &normalized) != nil || !reflect.DeepEqual(metadata, normalized) {
		return ErrAuditSensitiveMetadata
	}
	return nil
}
