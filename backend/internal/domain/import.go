package domain

import "time"

// ImportType 标识批次正在处理的租户数据类型。
type ImportType string

const (
	// ImportTypeUsers 表示租户用户导入。
	ImportTypeUsers ImportType = "users"
	// ImportTypeOrgUnits 表示组织架构导入。
	ImportTypeOrgUnits ImportType = "org_units"
)

// ImportBatchStatus 表示导入批次的生命周期状态。
type ImportBatchStatus string

const (
	// ImportBatchUploaded 表示文件已上传并等待解析。
	ImportBatchUploaded ImportBatchStatus = "UPLOADED"
	// ImportBatchValidating 表示服务端正在解析和校验。
	ImportBatchValidating ImportBatchStatus = "VALIDATING"
	// ImportBatchValidated 表示预校验完成。
	ImportBatchValidated ImportBatchStatus = "VALIDATED"
	// ImportBatchQueued 表示用户已确认，批次已持久化排队等待后台执行。
	ImportBatchQueued ImportBatchStatus = "QUEUED"
	// ImportBatchImporting 表示正式事务正在写入。
	ImportBatchImporting ImportBatchStatus = "IMPORTING"
	// ImportBatchSucceeded 表示批次原子导入成功。
	ImportBatchSucceeded ImportBatchStatus = "SUCCEEDED"
	// ImportBatchFailed 表示批次导入失败并已回滚。
	ImportBatchFailed ImportBatchStatus = "FAILED"
	// ImportBatchExpired 表示批次超过确认有效期。
	ImportBatchExpired ImportBatchStatus = "EXPIRED"
)

// ImportRowAction 表示一行数据在确认阶段的预期动作。
type ImportRowAction string

const (
	// ImportRowCreate 表示等待新增。
	ImportRowCreate ImportRowAction = "CREATE"
	// ImportRowUpdate 表示等待更新。
	ImportRowUpdate ImportRowAction = "UPDATE"
	// ImportRowSkip 表示确认时跳过。
	ImportRowSkip ImportRowAction = "SKIP"
)

// ImportRowStatus 表示一行数据的预校验状态。
type ImportRowStatus string

const (
	// ImportRowValid 表示行通过预校验。
	ImportRowValid ImportRowStatus = "VALID"
	// ImportRowInvalid 表示行校验失败。
	ImportRowInvalid ImportRowStatus = "INVALID"
)

// ImportError 描述可定位到 Excel 行和字段的脱敏错误。
type ImportError struct {
	RowNumber int    `json:"row_number"`
	Field     string `json:"field"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// ImportRowResult 保存预校验后的安全快照；密码等敏感字段不得写入 Fields。
type ImportRowResult struct {
	RowNumber int               `json:"row_number"`
	Key       string            `json:"key"`
	Action    ImportRowAction   `json:"action"`
	Status    ImportRowStatus   `json:"status"`
	Fields    map[string]string `json:"fields"`
	Errors    []ImportError     `json:"errors,omitempty"`
}

// ImportSummary 是预览和结果页面共用的数量统计。
type ImportSummary struct {
	Total   int `json:"total"`
	Valid   int `json:"valid"`
	Created int `json:"created"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// TenantImportBatch 是租户导入批次实体；RowsJSON 是服务端可信快照，不直接对外暴露。
type TenantImportBatch struct {
	// ID 是数据库内部主键，不作为跨接口批次标识。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// BatchID 是暴露给当前创建人的随机批次标识，不能替代租户鉴权条件。
	BatchID string `gorm:"column:batch_id;type:char(36);not null;uniqueIndex" json:"batch_id"`
	// TenantID 是导入数据的租户安全边界，任何业务写入都必须使用该值。
	TenantID uint64 `gorm:"column:tenant_id;not null;index" json:"-"`
	// ImportType 区分用户和组织架构导入，Worker 不接受批次外部覆盖。
	ImportType ImportType `gorm:"column:import_type;type:varchar(32);not null" json:"import_type"`
	// FileName 是清理路径后的展示名，不用于读取本地文件。
	FileName string `gorm:"column:file_name;type:varchar(255);not null" json:"file_name"`
	// FileHash 是原始上传内容摘要，用于审计关联而不保留原文件。
	FileHash string `gorm:"column:file_hash;type:char(64);not null" json:"-"`
	// SnapshotHash 保护服务端可信逐行快照，后台执行前必须再次校验。
	SnapshotHash string `gorm:"column:snapshot_hash;type:char(64);not null" json:"-"`
	// RowsJSON 保存已校验字段和密码摘要，禁止直接响应或写入日志。
	RowsJSON []byte `gorm:"column:rows_json;type:json;not null" json:"-"`
	// TotalCount 是批次总行数，用作进度分母。
	TotalCount int `gorm:"column:total_count;not null" json:"total_count"`
	// ValidCount 是预校验通过行数。
	ValidCount int `gorm:"column:valid_count;not null" json:"valid_count"`
	// SuccessCount 是终态成功写入行数。
	SuccessCount int `gorm:"column:success_count;not null" json:"success_count"`
	// FailureCount 是校验失败或事务回滚影响行数。
	FailureCount int `gorm:"column:failure_count;not null" json:"failure_count"`
	// SkippedCount 是按预览动作跳过的行数。
	SkippedCount int `gorm:"column:skipped_count;not null" json:"skipped_count"`
	// Status 是批次生命周期事实源，所有转换都需要条件更新。
	Status ImportBatchStatus `gorm:"column:status;type:varchar(32);not null;index" json:"status"`
	// CreatedBy 是批次创建人，查询接口用它防止同租户内横向枚举。
	CreatedBy uint64 `gorm:"column:created_by;not null;index" json:"created_by"`
	// ValidatedAt 是确认有效期的起点。
	ValidatedAt *time.Time `gorm:"column:validated_at" json:"validated_at,omitempty"`
	// ConfirmedAt 是批次首次成功入队时间，重复确认不会覆盖。
	ConfirmedAt *time.Time `gorm:"column:confirmed_at" json:"confirmed_at,omitempty"`
	// CompletedAt 是成功或失败终态落库时间。
	CompletedAt *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	// FailureReason 是可返回前端的脱敏批次原因，不得保存 SQL 或敏感字段。
	FailureReason string `gorm:"column:failure_reason;type:varchar(512)" json:"failure_reason,omitempty"`
	// Phase 是 Worker 当前阶段，用于解释进度而不是驱动权限判断。
	Phase string `gorm:"column:phase;type:varchar(32);not null;default:WAITING" json:"phase,omitempty"`
	// ProcessedCount 是近似已处理行数，终态必须等于成功与跳过数量之和。
	ProcessedCount int `gorm:"column:processed_count;not null;default:0" json:"processed_count"`
	// LeaseToken 是当前 Worker 的随机所有权令牌，只用于服务端 CAS 更新。
	LeaseToken string `gorm:"column:lease_token;type:char(36)" json:"-"`
	// LeaseExpiresAt 控制崩溃任务何时可被其他实例接管。
	LeaseExpiresAt *time.Time `gorm:"column:lease_expires_at;index" json:"-"`
	// HeartbeatAt 记录执行器最近一次存活信号，便于排障。
	HeartbeatAt *time.Time `gorm:"column:heartbeat_at" json:"-"`
	// AttemptCount 记录租约领取次数，用于识别恢复或反复失败。
	AttemptCount int `gorm:"column:attempt_count;not null;default:0" json:"attempt_count"`
	// CreatedAt 是批次创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是生命周期或进度最近更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定 TenantImportBatch 对应的批次表。
func (TenantImportBatch) TableName() string { return "tenant_import_batches" }
