package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"go-cpabe/backend/internal/domain"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TestValidatePreparedAuditOutboxEventRejectsBypass 验证内部调用不能绕过规范化器把任意 Metadata 或非法状态写入 outbox。
func TestValidatePreparedAuditOutboxEventRejectsBypass(t *testing.T) {
	event := auditOutboxFixture("123e4567-e89b-42d3-a456-426614174000", "dedup:validate:1")
	if err := validatePreparedAuditOutboxEvent(event); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	event.MetadataJSON = []byte(`{"access_token":"secret"}`)
	if err := validatePreparedAuditOutboxEvent(event); !errors.Is(err, ErrAuditSensitiveMetadata) {
		t.Fatalf("unsafe metadata error=%v", err)
	}
	event.MetadataJSON, event.MetadataRedacted = []byte(`{}`), true
	if err := validatePreparedAuditOutboxEvent(event); err != nil {
		t.Fatalf("redacted minimal event rejected: %v", err)
	}
}

// TestSameAuditOutboxPayloadDistinguishesSecurityFacts 验证业务去重只接受相同安全正文，并保留平台空租户与租户零值指针的差异。
func TestSameAuditOutboxPayloadDistinguishesSecurityFacts(t *testing.T) {
	left := auditOutboxFixture("123e4567-e89b-42d3-a456-426614174000", "dedup:payload:1")
	right := left
	right.EventPublicID = "223e4567-e89b-42d3-a456-426614174000"
	right.OccurredAt = right.OccurredAt.Add(time.Second)
	if !sameAuditOutboxPayload(left, right) {
		t.Fatal("same business payload should allow a regenerated event UUID")
	}
	right.Action = "encryption.complete"
	if sameAuditOutboxPayload(left, right) {
		t.Fatal("different action must not be swallowed as an idempotent duplicate")
	}
	right = left
	zero := uint64(0)
	right.TenantID = &zero
	if sameAuditOutboxPayload(left, right) {
		t.Fatal("platform nil tenant and tenant zero pointer must remain distinct")
	}
}

// TestAuditOutboxRepositoryAgainstMySQL 验证真实唯一键、事务回滚、并发领取、租约恢复和重复投递收敛。
func TestAuditOutboxRepositoryAgainstMySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("设置 TEST_MYSQL_DSN 指向可销毁的隔离 MySQL 测试库后运行 outbox 集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.AuditLog{}, &domain.AuditOutboxEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM audit_outbox").Error; err != nil {
		t.Fatal(err)
	}
	repo := NewGormAuditOutboxRepository(db)
	ctx := context.Background()
	event := auditOutboxFixture("123e4567-e89b-42d3-a456-426614174000", "dedup:task:create:1")
	created, duplicate, err := repo.Enqueue(ctx, event)
	if err != nil || duplicate || created.EventPublicID != event.EventPublicID {
		t.Fatalf("first enqueue: created=%+v duplicate=%t err=%v", created, duplicate, err)
	}
	if _, duplicate, err := repo.Enqueue(ctx, event); err != nil || !duplicate {
		t.Fatalf("duplicate enqueue must converge: duplicate=%t err=%v", duplicate, err)
	}
	conflict := event
	conflict.EventPublicID = "223e4567-e89b-42d3-a456-426614174000"
	conflict.Action = "encryption.complete"
	if _, _, err := repo.Enqueue(ctx, conflict); !errors.Is(err, ErrAuditOutboxConflict) {
		t.Fatalf("reused dedup key with different payload must conflict: %v", err)
	}

	rolledBack := auditOutboxFixture("223e4567-e89b-42d3-a456-426614174000", "dedup:rollback:1")
	tx := db.Begin()
	if _, _, err := repo.EnqueueWithDB(ctx, tx, rolledBack); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	var rollbackCount int64
	if err := db.Model(&domain.AuditOutboxEvent{}).Where("event_public_id = ?", rolledBack.EventPublicID).Count(&rollbackCount).Error; err != nil || rollbackCount != 0 {
		t.Fatalf("rolled back outbox must not persist: count=%d err=%v", rollbackCount, err)
	}

	for index, publicID := range []string{"323e4567-e89b-42d3-a456-426614174000", "423e4567-e89b-42d3-a456-426614174000", "523e4567-e89b-42d3-a456-426614174000", "623e4567-e89b-42d3-a456-426614174000"} {
		fixture := auditOutboxFixture(publicID, "dedup:concurrent:"+string(rune('a'+index)))
		if _, _, err := repo.Enqueue(ctx, fixture); err != nil {
			t.Fatal(err)
		}
	}
	claimed := make(chan []domain.AuditOutboxEvent, 2)
	errorsCh := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			items, err := repo.Claim(ctx, 2, time.Now(), time.Minute)
			claimed <- items
			errorsCh <- err
		}()
	}
	wait.Wait()
	close(claimed)
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[uint64]bool{}
	var oneClaimed domain.AuditOutboxEvent
	for items := range claimed {
		for _, item := range items {
			if seen[item.ID] {
				t.Fatalf("event %d claimed by multiple workers", item.ID)
			}
			seen[item.ID] = true
			oneClaimed = item
		}
	}
	if len(seen) == 0 || oneClaimed.LockToken == nil {
		t.Fatalf("expected claimed events, seen=%+v", seen)
	}
	originalAction := oneClaimed.Action
	oneClaimed.Action = "tampered.in.memory"
	if err := repo.Deliver(ctx, oneClaimed, *oneClaimed.LockToken, time.Now()); err != nil {
		t.Fatal(err)
	}
	var persistedLog domain.AuditLog
	if err := db.Where("public_id = ?", oneClaimed.EventPublicID).First(&persistedLog).Error; err != nil || persistedLog.Action != originalAction {
		t.Fatalf("delivery must use persisted payload: action=%s err=%v", persistedLog.Action, err)
	}

	// 隔离租约恢复场景，避免前一段并发领取留下的候选事件影响顺序。
	if err := db.Model(&domain.AuditOutboxEvent{}).Where("status <> ?", domain.AuditOutboxStatusDelivered).Updates(map[string]any{"status": domain.AuditOutboxStatusDelivered, "delivered_at": time.Now(), "locked_at": nil, "lock_token": nil}).Error; err != nil {
		t.Fatal(err)
	}
	leaseEvent := auditOutboxFixture("723e4567-e89b-42d3-a456-426614174000", "dedup:lease:1")
	if _, _, err := repo.Enqueue(ctx, leaseEvent); err != nil {
		t.Fatal(err)
	}
	firstLease, err := repo.Claim(ctx, 1, time.Now(), time.Second)
	if err != nil || len(firstLease) != 1 || firstLease[0].LockToken == nil {
		t.Fatalf("first lease: events=%+v err=%v", firstLease, err)
	}
	oldToken := *firstLease[0].LockToken
	if err := db.Model(&domain.AuditOutboxEvent{}).Where("id = ?", firstLease[0].ID).Update("locked_at", time.Now().Add(-2*time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	secondLease, err := repo.Claim(ctx, 1, time.Now(), time.Second)
	if err != nil || len(secondLease) != 1 || secondLease[0].LockToken == nil || *secondLease[0].LockToken == oldToken {
		t.Fatalf("expired lease must receive a new token: events=%+v err=%v", secondLease, err)
	}
	if err := repo.Deliver(ctx, firstLease[0], oldToken, time.Now()); !errors.Is(err, ErrAuditOutboxLeaseLost) {
		t.Fatalf("old worker must lose CAS: %v", err)
	}
	// 预先存在的正式日志模拟崩溃窗口；重放应依赖 public_id 唯一键收敛，且仍能完成 outbox 状态。
	preexisting := domain.AuditLog{PublicID: secondLease[0].EventPublicID, Action: secondLease[0].Action, TargetType: secondLease[0].TargetType, TargetPublicID: secondLease[0].TargetPublicID, Result: secondLease[0].Result, SourceTrust: secondLease[0].SourceTrust, MetadataJSON: secondLease[0].MetadataJSON, CreatedAt: secondLease[0].OccurredAt}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&preexisting).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.Deliver(ctx, secondLease[0], *secondLease[0].LockToken, time.Now()); err != nil {
		t.Fatal(err)
	}
	var replayLogs int64
	if err := db.Model(&domain.AuditLog{}).Where("public_id = ?", secondLease[0].EventPublicID).Count(&replayLogs).Error; err != nil || replayLogs != 1 {
		t.Fatalf("replay must converge to one formal log: count=%d err=%v", replayLogs, err)
	}
}

// auditOutboxFixture 构造不含秘密的固定事件，所有时间和标识都可安全进入隔离测试库。
func auditOutboxFixture(publicID, dedup string) domain.AuditOutboxEvent {
	now := time.Now()
	return domain.AuditOutboxEvent{EventPublicID: publicID, DedupKey: &dedup, Action: "encryption.task.create", TargetType: "encryption", TargetPublicID: publicID, Result: "SUCCESS", SourceTrust: "SERVER_OBSERVED", MetadataJSON: []byte("{}"), PayloadVersion: 1, OccurredAt: now, Status: domain.AuditOutboxStatusPending}
}
