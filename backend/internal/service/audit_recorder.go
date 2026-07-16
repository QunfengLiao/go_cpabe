package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/identifier"
	"go-cpabe/backend/internal/repository"
)

// AuditEvent 表示一次需要记录的安全或管理操作。
type AuditEvent struct {
	// TenantID 来自可信租户上下文；平台事件可空。
	TenantID *uint64
	// ActorUserID 来自认证上下文，零值表示系统事件。
	ActorUserID uint64
	// Action 是决定 Metadata 白名单的稳定动作编码。
	Action string
	// TargetType 是不含内部表名或路径的业务目标类别。
	TargetType string
	// TargetID 是旧调用方的内部标识兼容字段，新业务应优先使用外部 UUID。
	TargetID uint64
	// TargetPublicID 是允许写入审计的外部目标标识。
	TargetPublicID string
	// Result 是 SUCCESS、FAILURE 或 DENIED。
	Result string
	// SourceTrust 区分服务端观察事实与客户端报告指标。
	SourceTrust string
	// ErrorCode 只能是稳定脱敏错误分类。
	ErrorCode string
	// RequestID 是不含 Token 的可选链路标识。
	RequestID string
	// Metadata 是待按 Action 白名单过滤的补充标量，不能包含秘密材料或路径。
	Metadata map[string]any
	// EventPublicID 是可选稳定事件 UUID，空值时由记录器生成。
	EventPublicID string
	// DedupKey 是可选业务幂等键，不得拼接秘密原文。
	DedupKey string
	// OccurredAt 是事件真实发生时间，零值时由记录器冻结当前时间。
	OccurredAt time.Time
}

// AuditRecorder 定义审计记录能力，后续可替换为数据库或日志实现。
type AuditRecorder interface {
	Record(ctx context.Context, event AuditEvent) error
}

// NoopAuditRecorder 是空审计实现，用于审计模块尚未落库时保持主链路可运行。
type NoopAuditRecorder struct{}

// Record 接收审计事件但不持久化，始终返回 nil。
func (NoopAuditRecorder) Record(context.Context, AuditEvent) error {
	return nil
}

// DatabaseAuditRecorder 将规范化安全事件优先写入 outbox；正式日志由 Dispatcher 幂等投递。
type DatabaseAuditRecorder struct {
	repository repository.AuditRepository
}

// NewDatabaseAuditRecorder 创建持久化审计记录器。
func NewDatabaseAuditRecorder(repository repository.AuditRepository) *DatabaseAuditRecorder {
	return &DatabaseAuditRecorder{repository: repository}
}

// Record 规范化事件并可靠入队；仓储不支持 outbox 时保留直写兼容路径，便于旧测试替身逐步迁移。
func (r *DatabaseAuditRecorder) Record(ctx context.Context, event AuditEvent) error {
	prepared, err := r.Prepare(event)
	if err != nil {
		return err
	}
	return r.persistPrepared(ctx, prepared)
}

// Prepare 生成入队后不可变的事件 UUID、发生时间和安全 Metadata；顶层字段非法会拒绝整个事件。
func (r *DatabaseAuditRecorder) Prepare(event AuditEvent) (domain.AuditOutboxEvent, error) {
	publicID := strings.TrimSpace(event.EventPublicID)
	var err error
	if publicID == "" {
		publicID, err = identifier.NewUUID()
		if err != nil {
			return domain.AuditOutboxEvent{}, err
		}
	}
	if strings.TrimSpace(event.Action) == "" || len(event.Action) > 128 || strings.TrimSpace(event.TargetType) == "" || len(event.TargetType) > 64 {
		return domain.AuditOutboxEvent{}, errors.New("invalid audit event identity")
	}
	targetPublicID := event.TargetPublicID
	if targetPublicID == "" && event.TargetID != 0 {
		targetPublicID = strconv.FormatUint(event.TargetID, 10)
	}
	result := event.Result
	if result == "" {
		result = "SUCCESS"
	}
	sourceTrust := event.SourceTrust
	if sourceTrust == "" {
		sourceTrust = "SERVER_OBSERVED"
	}
	var actorID *uint64
	if event.ActorUserID != 0 {
		actor := event.ActorUserID
		actorID = &actor
	}
	if result != "SUCCESS" && result != "FAILURE" && result != "DENIED" {
		return domain.AuditOutboxEvent{}, errors.New("invalid audit result")
	}
	if sourceTrust != "SERVER_OBSERVED" && sourceTrust != "CLIENT_REPORTED" {
		return domain.AuditOutboxEvent{}, errors.New("invalid audit source trust")
	}
	metadataJSON, metadataRedacted := repository.PrepareAuditMetadata(event.Action, event.Metadata)
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	var dedupKey *string
	if value := strings.TrimSpace(event.DedupKey); value != "" {
		if len(value) > 128 {
			return domain.AuditOutboxEvent{}, errors.New("invalid audit dedup key")
		}
		dedupKey = &value
	}
	return domain.AuditOutboxEvent{EventPublicID: publicID, DedupKey: dedupKey, TenantID: event.TenantID, ActorUserID: actorID, Action: event.Action, TargetType: event.TargetType, TargetPublicID: targetPublicID, Result: result, SourceTrust: sourceTrust, ErrorCode: event.ErrorCode, RequestID: event.RequestID, MetadataJSON: metadataJSON, MetadataRedacted: metadataRedacted, PayloadVersion: 1, OccurredAt: occurredAt, Status: domain.AuditOutboxStatusPending}, nil
}

// persistPrepared 将已过滤事件交给 outbox；兼容直写路径仍复用同一安全快照，避免两套过滤规则漂移。
func (r *DatabaseAuditRecorder) persistPrepared(ctx context.Context, event domain.AuditOutboxEvent) error {
	if writer, ok := r.repository.(repository.AuditOutboxWriter); ok {
		_, _, err := writer.EnqueueAuditEvent(ctx, event)
		return err
	}
	var metadata map[string]any
	if err := json.Unmarshal(event.MetadataJSON, &metadata); err != nil {
		return err
	}
	return r.repository.Create(ctx, domain.AuditLog{PublicID: event.EventPublicID, TenantID: event.TenantID, ActorUserID: event.ActorUserID, Action: event.Action, TargetType: event.TargetType, TargetPublicID: event.TargetPublicID, Result: event.Result, SourceTrust: event.SourceTrust, ErrorCode: event.ErrorCode, RequestID: event.RequestID, MetadataRedacted: event.MetadataRedacted, CreatedAt: event.OccurredAt}, metadata)
}

// RecordBestEffort 在不影响主链路的前提下写审计；失败时输出不含业务元数据的结构化日志，供日志系统告警和离线恢复。
func (r *DatabaseAuditRecorder) RecordBestEffort(ctx context.Context, event AuditEvent) {
	if r == nil || r.repository == nil {
		return
	}
	prepared, err := r.Prepare(event)
	if err == nil {
		err = r.persistPrepared(ctx, prepared)
	}
	if err != nil {
		r.logFailure(prepared.EventPublicID, event.Action, classifyAuditRecordError(err))
	}
}

// logFailure 只记录事件 UUID、动作和稳定错误分类；禁止输出 Metadata、请求字段、内部 ID 或原始数据库错误。
func (r *DatabaseAuditRecorder) logFailure(eventPublicID, action, errorCode string) {
	payload := map[string]any{
		"component":       "audit",
		"event_public_id": eventPublicID,
		"action":          action,
		"record_error":    errorCode,
	}
	if encoded, marshalErr := json.Marshal(payload); marshalErr == nil {
		log.Printf("audit_write_failed %s", encoded)
	} else {
		log.Printf("audit_write_failed record_error=AUDIT_LOG_ENCODING_FAILED")
	}
}

// classifyAuditRecordError 把内部错误收敛为稳定分类，避免普通日志泄露 SQL、主机名或秘密材料。
func classifyAuditRecordError(err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "AUDIT_CONTEXT_CANCELLED"
	}
	if errors.Is(err, repository.ErrAuditSensitiveMetadata) {
		return "AUDIT_METADATA_REJECTED"
	}
	return "AUDIT_PERSIST_FAILED"
}

// recordAuditBestEffort 统一所有审计实现的降级语义；数据库实现保留结构化失败日志，测试或替代实现也不得阻断加密主链路。
func recordAuditBestEffort(ctx context.Context, recorder AuditRecorder, event AuditEvent) {
	if recorder == nil {
		return
	}
	if bestEffort, ok := recorder.(interface {
		RecordBestEffort(context.Context, AuditEvent)
	}); ok {
		bestEffort.RecordBestEffort(ctx, event)
		return
	}
	_ = recorder.Record(ctx, event)
}
