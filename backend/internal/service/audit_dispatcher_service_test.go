package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/repository"
	"gorm.io/gorm"
)

// auditOutboxRepositoryStub 保存 Dispatcher 可观察状态，避免单元测试依赖真实 MySQL。
type auditOutboxRepositoryStub struct {
	claimed       []domain.AuditOutboxEvent
	deliverErrors map[uint64]error
	delivered     []uint64
	failed        []domain.AuditOutboxEvent
	failureCodes  []string
	nextRetries   []time.Time
	maxRetries    []uint32
	deleted       int64
}

// Enqueue 保存事件的最小测试实现；Dispatcher 测试不使用该路径。
func (s *auditOutboxRepositoryStub) Enqueue(_ context.Context, event domain.AuditOutboxEvent) (domain.AuditOutboxEvent, bool, error) {
	return event, false, nil
}

// EnqueueWithDB 保存事件的事务测试实现；Dispatcher 测试不使用该路径。
func (s *auditOutboxRepositoryStub) EnqueueWithDB(_ context.Context, _ *gorm.DB, event domain.AuditOutboxEvent) (domain.AuditOutboxEvent, bool, error) {
	return event, false, nil
}

// Claim 返回预置事件副本，模拟仓储已经分配独立租约。
func (s *auditOutboxRepositoryStub) Claim(context.Context, int, time.Time, time.Duration) ([]domain.AuditOutboxEvent, error) {
	return append([]domain.AuditOutboxEvent(nil), s.claimed...), nil
}

// Deliver 记录成功投递，或返回按事件主键注入的稳定故障。
func (s *auditOutboxRepositoryStub) Deliver(_ context.Context, event domain.AuditOutboxEvent, _ string, _ time.Time) error {
	if err := s.deliverErrors[event.ID]; err != nil {
		return err
	}
	s.delivered = append(s.delivered, event.ID)
	return nil
}

// MarkFailure 保存失败事件的下一状态输入；测试通过 RetryCount 判断重试或死信。
func (s *auditOutboxRepositoryStub) MarkFailure(_ context.Context, event domain.AuditOutboxEvent, _, errorCode string, nextRetry time.Time, maxRetries uint32) error {
	s.failed = append(s.failed, event)
	s.failureCodes = append(s.failureCodes, errorCode)
	s.nextRetries = append(s.nextRetries, nextRetry)
	s.maxRetries = append(s.maxRetries, maxRetries)
	return nil
}

// DeleteDeliveredBefore 返回预置清理数量，确保死信处理不会影响保留期清理。
func (s *auditOutboxRepositoryStub) DeleteDeliveredBefore(context.Context, time.Time, int) (int64, error) {
	return s.deleted, nil
}

// TestAuditDispatcherContinuesAfterSingleFailure 验证单条投递失败进入重试且不阻断同批成功事件。
func TestAuditDispatcherContinuesAfterSingleFailure(t *testing.T) {
	tokenA, tokenB := "lease-a", "lease-b"
	repo := &auditOutboxRepositoryStub{claimed: []domain.AuditOutboxEvent{{ID: 1, EventPublicID: "event-a", LockToken: &tokenA}, {ID: 2, EventPublicID: "event-b", LockToken: &tokenB}}, deliverErrors: map[uint64]error{1: errors.New("database unavailable")}, deleted: 3}
	dispatcher, err := NewAuditDispatcherService(repo, auditDispatcherTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	dispatcher.now = func() time.Time { return time.Unix(100, 0) }
	result, err := dispatcher.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 2 || result.Delivered != 1 || result.Retried != 1 || result.Dead != 0 || result.Deleted != 3 || len(repo.failed) != 1 {
		t.Fatalf("unexpected result=%+v delivered=%+v failed=%+v", result, repo.delivered, repo.failed)
	}
	if repo.failureCodes[0] != "AUDIT_DELIVERY_FAILED" || repo.maxRetries[0] != 10 || !repo.nextRetries[0].After(time.Unix(100, 0)) {
		t.Fatalf("failure transition must use stable classification and future retry: codes=%v next=%v max=%v", repo.failureCodes, repo.nextRetries, repo.maxRetries)
	}
}

// TestAuditDispatcherMarksDeadAndHonorsLeaseLoss 验证达到上限计入死信，租约丢失不会被错误地标记失败。
func TestAuditDispatcherMarksDeadAndHonorsLeaseLoss(t *testing.T) {
	tokenA, tokenB := "lease-a", "lease-b"
	repo := &auditOutboxRepositoryStub{claimed: []domain.AuditOutboxEvent{{ID: 1, EventPublicID: "event-a", LockToken: &tokenA, RetryCount: 9}, {ID: 2, EventPublicID: "event-b", LockToken: &tokenB}}, deliverErrors: map[uint64]error{1: errors.New("write failed"), 2: repository.ErrAuditOutboxLeaseLost}}
	dispatcher, err := NewAuditDispatcherService(repo, auditDispatcherTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	result, err := dispatcher.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Dead != 1 || result.LeaseLost != 1 || len(repo.failed) != 1 || repo.failed[0].ID != 1 {
		t.Fatalf("unexpected result=%+v failed=%+v", result, repo.failed)
	}
}

// TestAuditDispatcherRejectsInvalidConfig 验证零租约或零重试等危险配置不能启动 Dispatcher。
func TestAuditDispatcherRejectsInvalidConfig(t *testing.T) {
	if _, err := NewAuditDispatcherService(&auditOutboxRepositoryStub{}, AuditDispatcherConfig{}); err == nil {
		t.Fatal("invalid configuration must fail")
	}
}

// TestAuditDispatcherHonorsCancelledContext 验证取消后不再投递已领取事件，租约留待过期后由其他实例接管。
func TestAuditDispatcherHonorsCancelledContext(t *testing.T) {
	token := "lease-a"
	repo := &auditOutboxRepositoryStub{claimed: []domain.AuditOutboxEvent{{ID: 1, EventPublicID: "event-a", LockToken: &token}}}
	dispatcher, err := NewAuditDispatcherService(repo, auditDispatcherTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := dispatcher.RunOnce(ctx)
	if !errors.Is(err, context.Canceled) || result.Claimed != 1 || len(repo.delivered) != 0 || len(repo.failed) != 0 {
		t.Fatalf("cancelled batch must stop safely: result=%+v err=%v", result, err)
	}
}

// TestAuditDispatcherRetryDelayIsBounded 验证指数退避随重试次数增长，并始终受配置上限约束。
func TestAuditDispatcherRetryDelayIsBounded(t *testing.T) {
	dispatcher, err := NewAuditDispatcherService(&auditOutboxRepositoryStub{}, auditDispatcherTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	first := dispatcher.retryDelay(domain.AuditOutboxEvent{EventPublicID: "event-a"})
	later := dispatcher.retryDelay(domain.AuditOutboxEvent{EventPublicID: "event-a", RetryCount: 3})
	capped := dispatcher.retryDelay(domain.AuditOutboxEvent{EventPublicID: "event-a", RetryCount: 30})
	if first < time.Second || later <= first || capped != time.Hour {
		t.Fatalf("unexpected retry delays: first=%s later=%s capped=%s", first, later, capped)
	}
}

// auditDispatcherTestConfig 返回无外部依赖的固定测试配置，覆盖重试、死信和保留期清理。
func auditDispatcherTestConfig() AuditDispatcherConfig {
	return AuditDispatcherConfig{BatchSize: 10, Lease: time.Minute, MaxRetries: 10, BaseBackoff: time.Second, MaxBackoff: time.Hour, DeliveredRetention: 24 * time.Hour}
}

var _ repository.AuditOutboxRepository = (*auditOutboxRepositoryStub)(nil)
