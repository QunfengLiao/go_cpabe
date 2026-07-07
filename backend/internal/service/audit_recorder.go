package service

import "context"

type AuditEvent struct {
	ActorUserID uint64
	Action      string
	TargetType  string
	TargetID    uint64
	Metadata    map[string]any
}

type AuditRecorder interface {
	Record(ctx context.Context, event AuditEvent) error
}

type NoopAuditRecorder struct{}

func (NoopAuditRecorder) Record(context.Context, AuditEvent) error {
	return nil
}
