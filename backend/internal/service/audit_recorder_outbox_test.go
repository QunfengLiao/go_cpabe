package service

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"go-cpabe/backend/internal/domain"
)

// auditRepositoryOutboxStub 同时实现正式审计兼容接口和 outbox writer，验证生产装配优先可靠入队。
type auditRepositoryOutboxStub struct {
	events      []domain.AuditOutboxEvent
	enqueueErr  error
	directCalls int
}

// Create 记录兼容直写次数；支持 outbox 时该方法不应被生产记录器调用。
func (s *auditRepositoryOutboxStub) Create(context.Context, domain.AuditLog, map[string]any) error {
	s.directCalls++
	return nil
}

// ListByTenant 返回空审计列表；本测试只验证写入边界。
func (s *auditRepositoryOutboxStub) ListByTenant(context.Context, uint64, int, int) ([]domain.AuditLog, error) {
	return nil, nil
}

// EnqueueAuditEvent 保存安全快照或返回注入错误，模拟 MySQL outbox 可用与不可用场景。
func (s *auditRepositoryOutboxStub) EnqueueAuditEvent(_ context.Context, event domain.AuditOutboxEvent) (domain.AuditOutboxEvent, bool, error) {
	if s.enqueueErr != nil {
		return domain.AuditOutboxEvent{}, false, s.enqueueErr
	}
	s.events = append(s.events, event)
	return event, false, nil
}

// TestDatabaseAuditRecorderEnqueuesSanitizedEvent 验证 outbox 保存稳定事件时间和最小脱敏事实，不直写正式日志。
func TestDatabaseAuditRecorderEnqueuesSanitizedEvent(t *testing.T) {
	repositoryLayer := &auditRepositoryOutboxStub{}
	recorder := NewDatabaseAuditRecorder(repositoryLayer)
	tenantID := uint64(3)
	err := recorder.Record(context.Background(), AuditEvent{TenantID: &tenantID, ActorUserID: 7, Action: "encryption.progress", TargetType: "encryption", TargetPublicID: "123e4567-e89b-42d3-a456-426614174000", Metadata: map[string]any{"access_token": "must-not-persist"}})
	if err != nil {
		t.Fatal(err)
	}
	if repositoryLayer.directCalls != 0 || len(repositoryLayer.events) != 1 {
		t.Fatalf("outbox must be preferred: direct=%d events=%d", repositoryLayer.directCalls, len(repositoryLayer.events))
	}
	event := repositoryLayer.events[0]
	if event.EventPublicID == "" || event.OccurredAt.IsZero() || !event.MetadataRedacted || string(event.MetadataJSON) != "{}" || strings.Contains(string(event.MetadataJSON), "must-not-persist") {
		t.Fatalf("unsafe outbox event: %+v metadata=%s", event, event.MetadataJSON)
	}
}

// TestDatabaseAuditRecorderBestEffortLogOmitsSecrets 验证数据库整体不可用时仅输出稳定分类，不泄露 Metadata 或原始数据库错误。
func TestDatabaseAuditRecorderBestEffortLogOmitsSecrets(t *testing.T) {
	repositoryLayer := &auditRepositoryOutboxStub{enqueueErr: errors.New("dial mysql user=secret host=internal")}
	recorder := NewDatabaseAuditRecorder(repositoryLayer)
	var output bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})
	recorder.RecordBestEffort(context.Background(), AuditEvent{Action: "encryption.progress", TargetType: "encryption", Metadata: map[string]any{"access_token": "token-secret"}})
	text := output.String()
	if !strings.Contains(text, "AUDIT_PERSIST_FAILED") || strings.Contains(text, "token-secret") || strings.Contains(text, "mysql") || strings.Contains(text, "internal") || strings.Contains(text, "access_token") {
		t.Fatalf("unsafe fallback log: %s", text)
	}
}
