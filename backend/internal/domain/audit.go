package domain

import "time"

// AuditOutboxStatus 表示内部审计事件从待领取到投递完成或死信的生命周期状态。
type AuditOutboxStatus string

const (
	// AuditOutboxStatusPending 表示事件已随业务事实提交，等待 Dispatcher 首次领取。
	AuditOutboxStatusPending AuditOutboxStatus = "PENDING"
	// AuditOutboxStatusProcessing 表示事件已被持有有效租约的 Dispatcher 领取。
	AuditOutboxStatusProcessing AuditOutboxStatus = "PROCESSING"
	// AuditOutboxStatusRetry 表示上次投递发生可恢复错误，将在退避时间到期后重试。
	AuditOutboxStatusRetry AuditOutboxStatus = "RETRY"
	// AuditOutboxStatusDelivered 表示正式审计日志已幂等落库，事件可以按保留策略清理。
	AuditOutboxStatusDelivered AuditOutboxStatus = "DELIVERED"
	// AuditOutboxStatusDeadLetter 表示事件超过重试上限，需要人工诊断后受控重放。
	AuditOutboxStatusDeadLetter AuditOutboxStatus = "DEAD_LETTER"
)

// AuditLog 是安全与业务关键事件的持久化记录。
type AuditLog struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// PublicID 是审计查询 UUID。
	PublicID string `gorm:"column:public_id;type:char(36);not null;uniqueIndex" json:"id"`
	// TenantID 是租户事件范围；平台事件可空。
	TenantID *uint64 `gorm:"column:tenant_id;index" json:"tenant_id"`
	// ActorUserID 是可信认证操作者；无法识别时可空。
	ActorUserID *uint64 `gorm:"column:actor_user_id;index" json:"actor_user_id"`
	// Action 是稳定动作编码。
	Action string `gorm:"column:action;type:varchar(128);not null;index" json:"action"`
	// TargetType 是目标业务类型。
	TargetType string `gorm:"column:target_type;type:varchar(64);not null" json:"target_type"`
	// TargetPublicID 是不敏感外部目标标识。
	TargetPublicID string `gorm:"column:target_public_id;type:varchar(64)" json:"target_public_id"`
	// Result 是 SUCCESS、FAILURE 或 DENIED。
	Result string `gorm:"column:result;type:varchar(32);not null" json:"result"`
	// SourceTrust 区分服务端事实与本地 Crypto Worker 报告。
	SourceTrust string `gorm:"column:source_trust;type:varchar(32);not null" json:"source_trust"`
	// ErrorCode 是稳定脱敏错误码。
	ErrorCode string `gorm:"column:error_code;type:varchar(64)" json:"error_code"`
	// RequestID 关联 HTTP 请求或本地执行。
	RequestID string `gorm:"column:request_id;type:varchar(128);index" json:"request_id"`
	// MetadataJSON 只允许白名单非敏感 JSON，禁止密钥与本地路径。
	MetadataJSON []byte `gorm:"column:metadata;type:json;not null" json:"metadata"`
	// MetadataRedacted 表示原补充信息未通过安全校验并已清空，帮助读取方区别于天然没有补充信息的事件。
	MetadataRedacted bool `gorm:"column:metadata_redacted;not null;default:false" json:"metadata_redacted"`
	// CreatedAt 是事件发生时间。
	CreatedAt time.Time `gorm:"column:created_at;index" json:"created_at"`
}

// TableName 指定 AuditLog 对应 audit_logs 表。
func (AuditLog) TableName() string { return "audit_logs" }

// AuditOutboxEvent 是完成白名单过滤后等待投递的内部审计事件。
//
// 该实体不关联文件孤儿清理流程，也不能直接序列化到业务 API；其中 MetadataJSON 仍属于受控内部数据，
// 生产者必须先完成按 action 的字段校验、大小限制和敏感信息清理。
type AuditOutboxEvent struct {
	// ID 是数据库生成的内部主键，不参与外部响应或业务权限判断。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// EventPublicID 是生产者生成的稳定 UUID；投递时复用为 audit_logs.public_id，且不得在重试时更换。
	EventPublicID string `gorm:"column:event_public_id;type:char(36);not null;uniqueIndex:uk_audit_outbox_event_public_id" json:"-"`
	// DedupKey 是可空的业务事实摘要；它只用于生产者幂等，不得包含 Token、路径或秘密原文。
	DedupKey *string `gorm:"column:dedup_key;type:varchar(128);uniqueIndex:uk_audit_outbox_dedup_key" json:"-"`
	// TenantID 来自可信租户上下文；平台事件可空，Metadata 不得覆盖该权限边界字段。
	TenantID *uint64 `gorm:"column:tenant_id;index:idx_audit_outbox_tenant_status,priority:1" json:"-"`
	// ActorUserID 来自已认证登录态；系统事件可空，不从客户端补充信息反推操作者。
	ActorUserID *uint64 `gorm:"column:actor_user_id" json:"-"`
	// Action 是入队后不可变的稳定动作编码，用于选择 Metadata 白名单。
	Action string `gorm:"column:action;type:varchar(128);not null" json:"-"`
	// TargetType 是非敏感目标类别，不承载数据库表名或内部存储位置。
	TargetType string `gorm:"column:target_type;type:varchar(64);not null" json:"-"`
	// TargetPublicID 是可对外关联的目标标识，不允许保存对象键或本地路径。
	TargetPublicID string `gorm:"column:target_public_id;type:varchar(64)" json:"-"`
	// Result 是 SUCCESS、FAILURE 或 DENIED，由可信业务分支确定。
	Result string `gorm:"column:result;type:varchar(32);not null" json:"-"`
	// SourceTrust 区分服务端观察事实与客户端报告指标，防止把客户端耗时误当成服务端证明。
	SourceTrust string `gorm:"column:source_trust;type:varchar(32);not null" json:"-"`
	// ErrorCode 只保存稳定脱敏分类，不得写入原始数据库错误、堆栈或秘密值。
	ErrorCode string `gorm:"column:error_code;type:varchar(64)" json:"-"`
	// RequestID 是可空的链路关联标识，不得保存 Authorization 或其他 Token。
	RequestID string `gorm:"column:request_id;type:varchar(128)" json:"-"`
	// MetadataJSON 只保存当前 Action 允许的非敏感标量 JSON；校验失败时必须改为空对象。
	MetadataJSON []byte `gorm:"column:metadata;type:json;not null" json:"-"`
	// MetadataRedacted 表示原 Metadata 因安全校验失败已被清空，不记录失败字段和值。
	MetadataRedacted bool `gorm:"column:metadata_redacted;not null;default:false" json:"-"`
	// PayloadVersion 是内部事件结构版本，首期固定为 1，用于后续兼容重放。
	PayloadVersion uint16 `gorm:"column:payload_version;not null;default:1" json:"-"`
	// OccurredAt 是业务事件真实发生时间，正式投递时必须原样保留而不是使用重放时间。
	OccurredAt time.Time `gorm:"column:occurred_at;not null" json:"-"`
	// Status 是投递状态；状态变化只能由受约束的仓储状态机执行。
	Status AuditOutboxStatus `gorm:"column:status;type:varchar(32);not null;default:PENDING;index:idx_audit_outbox_claim,priority:1;index:idx_audit_outbox_lease,priority:1;index:idx_audit_outbox_tenant_status,priority:2" json:"-"`
	// RetryCount 是已经确认失败的投递次数，Dispatcher 用它计算退避和死信边界。
	RetryCount uint32 `gorm:"column:retry_count;not null;default:0" json:"-"`
	// NextRetryAt 是下次允许领取的时间；首次待处理可空，RETRY 状态必须有值。
	NextRetryAt *time.Time `gorm:"column:next_retry_at;index:idx_audit_outbox_claim,priority:2" json:"-"`
	// LockedAt 是当前处理租约开始时间；非 PROCESSING 状态应清空，过期后允许新 Worker 接管。
	LockedAt *time.Time `gorm:"column:locked_at;index:idx_audit_outbox_lease,priority:2" json:"-"`
	// LockToken 是可空的随机租约令牌；完成和失败更新必须匹配它，避免旧 Worker 覆盖新租约。
	LockToken *string `gorm:"column:lock_token;type:char(36)" json:"-"`
	// LastErrorCode 是最近一次投递的稳定脱敏错误分类，不保存原始错误或堆栈。
	LastErrorCode string `gorm:"column:last_error_code;type:varchar(64)" json:"-"`
	// DeliveredAt 是正式日志确认写入的时间，仅 DELIVERED 状态存在。
	DeliveredAt *time.Time `gorm:"column:delivered_at" json:"-"`
	// CreatedAt 是数据库记录的入队时间，只用于内部调度和运维诊断。
	CreatedAt time.Time `gorm:"column:created_at;index:idx_audit_outbox_tenant_status,priority:3" json:"-"`
	// UpdatedAt 是最近状态变化时间，不代表事件真实发生时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"-"`
}

// TableName 指定 AuditOutboxEvent 对应独立 audit_outbox 表，避免被文件孤儿清理器误领取。
func (AuditOutboxEvent) TableName() string { return "audit_outbox" }

// OrphanStorageObject 记录需要后台补偿清理的服务端对象。
type OrphanStorageObject struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// TenantID 是对象所属租户。
	TenantID uint64 `gorm:"column:tenant_id;not null;index" json:"-"`
	// TaskAttemptID 关联来源执行，可空。
	TaskAttemptID *uint64 `gorm:"column:task_attempt_id;index" json:"-"`
	// ObjectKey 是存储内部对象键，禁止外部响应。
	ObjectKey string `gorm:"column:object_key;type:varchar(512);not null;uniqueIndex" json:"-"`
	// ReasonCode 是产生孤儿对象的稳定原因。
	ReasonCode string `gorm:"column:reason_code;type:varchar(64);not null" json:"reason_code"`
	// Status 是 PENDING、CLEANING、CLEANED 或 FAILED。
	Status string `gorm:"column:status;type:varchar(32);not null;index" json:"status"`
	// RetryCount 是清理重试次数。
	RetryCount uint32 `gorm:"column:retry_count;not null" json:"retry_count"`
	// LastErrorCode 是最近脱敏清理错误。
	LastErrorCode string `gorm:"column:last_error_code;type:varchar(64)" json:"last_error_code"`
	// NextRetryAt 是下次可领取时间。
	NextRetryAt *time.Time `gorm:"column:next_retry_at;index" json:"next_retry_at"`
	// CreatedAt 是孤儿登记时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是清理状态更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// CleanedAt 是删除成功时间。
	CleanedAt *time.Time `gorm:"column:cleaned_at" json:"cleaned_at"`
}

// TableName 指定 OrphanStorageObject 对应 orphan_storage_objects 表。
func (OrphanStorageObject) TableName() string { return "orphan_storage_objects" }
