package service

import "context"

// AuditEvent 表示一次需要记录的安全或管理操作。
type AuditEvent struct {
	ActorUserID uint64
	Action      string
	TargetType  string
	TargetID    uint64
	Metadata    map[string]any
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
