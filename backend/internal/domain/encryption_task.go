package domain

import "time"

// EncryptionTaskStatus 是任务和执行共享的受控状态。
type EncryptionTaskStatus string

const (
	// EncryptionPending 表示任务已创建但尚未开始校验。
	EncryptionPending EncryptionTaskStatus = "PENDING"
	// EncryptionValidating 表示正在校验文件和授权快照。
	EncryptionValidating EncryptionTaskStatus = "VALIDATING"
	// EncryptionEncryptingFile 表示本地 Crypto Worker 正在加密文件。
	EncryptionEncryptingFile EncryptionTaskStatus = "ENCRYPTING_FILE"
	// EncryptionProtectingKey 表示 CryptoEngine 正在保护 DEK。
	EncryptionProtectingKey EncryptionTaskStatus = "PROTECTING_KEY"
	// EncryptionUploading 表示 Electron 正在上传密文。
	EncryptionUploading EncryptionTaskStatus = "UPLOADING"
	// EncryptionSavingMetadata 表示服务端正在提交最终元数据。
	EncryptionSavingMetadata EncryptionTaskStatus = "SAVING_METADATA"
	// EncryptionCompleted 表示文件、密文和密钥元数据完整可用。
	EncryptionCompleted EncryptionTaskStatus = "COMPLETED"
	// EncryptionFailed 表示本次执行失败。
	EncryptionFailed EncryptionTaskStatus = "FAILED"
	// EncryptionCancelled 表示本次执行被安全取消。
	EncryptionCancelled EncryptionTaskStatus = "CANCELLED"
)

// EncryptionTask 保存一次不可变用户加密意图及当前执行聚合状态。
type EncryptionTask struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// PublicID 是 API 任务 UUID。
	PublicID string `gorm:"column:public_id;type:char(36);not null;uniqueIndex" json:"id"`
	// TenantID 来自可信中间件并参与幂等唯一键。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_encryption_task_idempotency,priority:1;index" json:"-"`
	// OwnerUserID 是发起 DO并参与幂等唯一键。
	OwnerUserID uint64 `gorm:"column:owner_user_id;not null;uniqueIndex:uk_encryption_task_idempotency,priority:2;index" json:"owner_user_id"`
	// FileID 指向本任务创建的草稿文件。
	FileID uint64 `gorm:"column:file_id;not null;uniqueIndex" json:"file_id"`
	// IdempotencyKey 仅区分一次用户意图，不用文件名推断重复。
	IdempotencyKey string `gorm:"column:idempotency_key;type:varchar(128);not null;uniqueIndex:uk_encryption_task_idempotency,priority:3" json:"-"`
	// AlgorithmCode 是 DEK 保护算法快照。
	AlgorithmCode string `gorm:"column:algorithm_code;type:varchar(64);not null" json:"algorithm_code"`
	// AlgorithmVersion 是算法参数版本快照。
	AlgorithmVersion string `gorm:"column:algorithm_version;type:varchar(32);not null" json:"algorithm_version"`
	// AuthorizationType 是算法无关授权配置类型。
	AuthorizationType string `gorm:"column:authorization_type;type:varchar(64);not null" json:"authorization_type"`
	// AuthorizationSnapshot 是已验证不可变 JSON，不含私钥。
	AuthorizationSnapshot []byte `gorm:"column:authorization_snapshot;type:json;not null" json:"-"`
	// AuthorizationSnapshotSHA256 用于绑定容器和受保护 DEK。
	AuthorizationSnapshotSHA256 string `gorm:"column:authorization_snapshot_sha256;type:char(64);not null" json:"authorization_snapshot_sha256"`
	// Status 是任务当前聚合状态。
	Status EncryptionTaskStatus `gorm:"column:status;type:varchar(32);not null;index" json:"status"`
	// CurrentAttemptNo 是当前执行序号。
	CurrentAttemptNo uint32 `gorm:"column:current_attempt_no;not null" json:"current_attempt_no"`
	// CancelRequestedAt 记录取消请求事实。
	CancelRequestedAt *time.Time `gorm:"column:cancel_requested_at" json:"cancel_requested_at"`
	// FailureCode 是稳定脱敏业务错误码。
	FailureCode string `gorm:"column:failure_code;type:varchar(64)" json:"failure_code"`
	// Retryable 表示是否允许创建新执行。
	Retryable bool `gorm:"column:retryable;not null" json:"retryable"`
	// LockVersion 防止未锁定路径误覆盖并发状态。
	LockVersion uint64 `gorm:"column:lock_version;not null" json:"-"`
	// CreatedAt 是任务创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是任务状态更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// CompletedAt 仅在完成时写入。
	CompletedAt *time.Time `gorm:"column:completed_at" json:"completed_at"`
}

// TableName 指定 EncryptionTask 对应 encryption_tasks 表。
func (EncryptionTask) TableName() string { return "encryption_tasks" }

// EncryptionTaskAttempt 保存初次执行或重试执行，历史终态不得覆盖。
type EncryptionTaskAttempt struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// PublicID 是 API 执行 UUID。
	PublicID string `gorm:"column:public_id;type:char(36);not null;uniqueIndex" json:"id"`
	// TenantID 是冗余租户隔离键。
	TenantID uint64 `gorm:"column:tenant_id;not null;index" json:"-"`
	// TaskID 指向加密任务并与 AttemptNo 唯一。
	TaskID uint64 `gorm:"column:task_id;not null;uniqueIndex:uk_task_attempt_no,priority:1;index" json:"-"`
	// AttemptNo 从 1 递增，区分重试历史。
	AttemptNo uint32 `gorm:"column:attempt_no;not null;uniqueIndex:uk_task_attempt_no,priority:2" json:"attempt_no"`
	// Status 是本执行当前状态。
	Status EncryptionTaskStatus `gorm:"column:status;type:varchar(32);not null;index" json:"status"`
	// ProcessedBytes 是客户端报告的真实处理字节数。
	ProcessedBytes int64 `gorm:"column:processed_bytes;not null" json:"processed_bytes"`
	// TotalBytes 是任务冻结的明文大小。
	TotalBytes int64 `gorm:"column:total_bytes;not null" json:"total_bytes"`
	// FailureCode 是稳定脱敏失败码。
	FailureCode string `gorm:"column:failure_code;type:varchar(64)" json:"failure_code"`
	// FailureStage 是失败时所在阶段。
	FailureStage string `gorm:"column:failure_stage;type:varchar(64)" json:"failure_stage"`
	// Retryable 表示该错误是否允许创建下一执行。
	Retryable bool `gorm:"column:retryable;not null" json:"retryable"`
	// StartedAt 是执行开始时间。
	StartedAt time.Time `gorm:"column:started_at" json:"started_at"`
	// UpdatedAt 是进度更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// FinishedAt 是终态时间。
	FinishedAt *time.Time `gorm:"column:finished_at" json:"finished_at"`
}

// TableName 指定 EncryptionTaskAttempt 对应 encryption_task_attempts 表。
func (EncryptionTaskAttempt) TableName() string { return "encryption_task_attempts" }

// EncryptionBenchmark 分离记录 AES、DEK 保护和上传指标，禁止混合算法结论。
type EncryptionBenchmark struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// TenantID 是指标所属租户，仅用于租户内演示。
	TenantID uint64 `gorm:"column:tenant_id;not null;index" json:"-"`
	// TaskAttemptID 保证一次执行只有一组指标。
	TaskAttemptID uint64 `gorm:"column:task_attempt_id;not null;uniqueIndex" json:"-"`
	// ValidationDurationMS 记录任务提交前后端校验耗时，不混入算法耗时。
	ValidationDurationMS int64 `gorm:"column:validation_duration_ms;not null" json:"validation_duration_ms"`
	// PlaintextSize 是输入文件大小。
	PlaintextSize int64 `gorm:"column:plaintext_size;not null" json:"plaintext_size"`
	// CiphertextSize 是完整容器大小。
	CiphertextSize int64 `gorm:"column:ciphertext_size;not null" json:"ciphertext_size"`
	// ProtectedKeyTotalSizeBytes 是所有接收者受保护 DEK 的总大小。
	ProtectedKeyTotalSizeBytes int64 `gorm:"column:protected_key_total_size_bytes;not null" json:"protected_key_total_size_bytes"`
	// AESEncryptMS 仅记录文件内容加密耗时。
	AESEncryptMS int64 `gorm:"column:aes_encrypt_ms;not null" json:"aes_encrypt_ms"`
	// DEKProtectMS 仅记录 DEK 保护耗时。
	DEKProtectMS int64 `gorm:"column:dek_protect_ms;not null" json:"dek_protect_ms"`
	// KeyProtectionDurationMS 与 DEKProtectMS 保持兼容，用于新详情按标准字段展示。
	KeyProtectionDurationMS int64 `gorm:"column:key_protection_duration_ms;not null" json:"key_protection_duration_ms"`
	// UploadMS 仅记录网络上传耗时。
	UploadMS int64 `gorm:"column:upload_ms;not null" json:"upload_ms"`
	// MetadataCommitDurationMS 记录服务端完成元数据事务耗时。
	MetadataCommitDurationMS int64 `gorm:"column:metadata_commit_duration_ms;not null" json:"metadata_commit_duration_ms"`
	// TotalDurationMS 是客户端观察到的端到端总耗时。
	TotalDurationMS int64 `gorm:"column:total_duration_ms;not null" json:"total_duration_ms"`
	// RecipientCount 是本次 DEK 封装的授权接收者数量。
	RecipientCount int64 `gorm:"column:recipient_count;not null" json:"recipient_count"`
	// PeakWorkingSetBytes 是客户端报告的近似峰值，不参与授权。
	PeakWorkingSetBytes int64 `gorm:"column:peak_working_set_bytes" json:"peak_working_set_bytes"`
	// ClientRuntime 标记 LOCAL_GO_WORKER 运行时。
	ClientRuntime string `gorm:"column:client_runtime;type:varchar(64);not null" json:"client_runtime"`
	// AlgorithmCode 是指标对应的 DEK 保护算法编码。
	AlgorithmCode string `gorm:"column:algorithm_code;type:varchar(64);not null" json:"algorithm_code"`
	// AlgorithmVersion 是指标对应的算法参数版本。
	AlgorithmVersion string `gorm:"column:algorithm_version;type:varchar(32);not null" json:"algorithm_version"`
	// Result 是本次执行的客户端结果标记，失败指标不得包含密钥材料。
	Result string `gorm:"column:result;type:varchar(32);not null" json:"result"`
	// CreatedAt 是指标保存时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定 EncryptionBenchmark 对应 encryption_benchmarks 表。
func (EncryptionBenchmark) TableName() string { return "encryption_benchmarks" }
