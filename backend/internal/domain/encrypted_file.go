package domain

import "time"

// EncryptedFileStatus 是加密文件业务可用状态。
type EncryptedFileStatus string

const (
	// EncryptedFileDraft 表示文件记录已创建但密文尚不可用。
	EncryptedFileDraft EncryptedFileStatus = "DRAFT"
	// EncryptedFileAvailable 表示密文和元数据完整且允许鉴权下载。
	EncryptedFileAvailable EncryptedFileStatus = "AVAILABLE"
	// EncryptedFileFailed 表示任务失败且不得下载半成品。
	EncryptedFileFailed EncryptedFileStatus = "FAILED"
	// EncryptedFileCancelled 表示用户取消且不得产生可用密文。
	EncryptedFileCancelled EncryptedFileStatus = "CANCELLED"
)

// CiphertextObjectStatus 是服务端密文对象生命周期状态。
type CiphertextObjectStatus string

const (
	// CiphertextStaging 表示对象已上传但尚未通过完成事务。
	CiphertextStaging CiphertextObjectStatus = "STAGING"
	// CiphertextAvailable 表示对象已与完整元数据原子关联。
	CiphertextAvailable CiphertextObjectStatus = "AVAILABLE"
	// CiphertextDeletePending 表示对象等待补偿清理。
	CiphertextDeletePending CiphertextObjectStatus = "DELETE_PENDING"
	// CiphertextDeleted 表示对象已从存储层删除。
	CiphertextDeleted CiphertextObjectStatus = "DELETED"
)

// EncryptedFile 保存 DO 的文件业务记录；不包含明文、本地路径或密钥材料。
type EncryptedFile struct {
	// ID 是内部自增主键，不对外用于资源定位。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// PublicID 是 API 使用的不可枚举 UUID。
	PublicID string `gorm:"column:public_id;type:char(36);not null;uniqueIndex" json:"id"`
	// TenantID 来自可信租户上下文，参与所有权限查询。
	TenantID uint64 `gorm:"column:tenant_id;not null;index:idx_encrypted_files_tenant_owner,priority:1" json:"-"`
	// OwnerUserID 是创建文件的 DO，参与所有者访问控制。
	OwnerUserID uint64 `gorm:"column:owner_user_id;not null;index:idx_encrypted_files_tenant_owner,priority:2" json:"owner_user_id"`
	// OriginalFilename 仅用于展示，不能用于服务端对象路径。
	OriginalFilename string `gorm:"column:original_filename;type:varchar(255);not null" json:"original_filename"`
	// DisplayMIMEType 只用于展示，不参与安全判断。
	DisplayMIMEType string `gorm:"column:display_mime_type;type:varchar(255)" json:"display_mime_type"`
	// PlaintextSize 是客户端声明并在任务快照中冻结的明文大小。
	PlaintextSize int64 `gorm:"column:plaintext_size;not null" json:"plaintext_size"`
	// Status 控制文件是否可被查询和下载。
	Status EncryptedFileStatus `gorm:"column:status;type:varchar(32);not null;index" json:"status"`
	// CurrentTaskID 指向创建该文件记录的加密任务。
	CurrentTaskID uint64 `gorm:"column:current_task_id;not null;index" json:"-"`
	// CompletedAt 仅在文件完整可用时写入。
	CompletedAt *time.Time `gorm:"column:completed_at" json:"completed_at"`
	// CreatedAt 是文件记录创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是状态或展示元数据更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定 EncryptedFile 对应 encrypted_files 表。
func (EncryptedFile) TableName() string { return "encrypted_files" }

// CiphertextObject 保存密文存储事实；ObjectKey 永不进入外部响应。
type CiphertextObject struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// PublicID 是上传完成协议使用的 UUID。
	PublicID string `gorm:"column:public_id;type:char(36);not null;uniqueIndex" json:"upload_id"`
	// TenantID 强制对象租户隔离。
	TenantID uint64 `gorm:"column:tenant_id;not null;index" json:"-"`
	// FileID 在完成事务前可为空，完成后唯一关联文件。
	FileID *uint64 `gorm:"column:file_id;uniqueIndex" json:"-"`
	// TaskAttemptID 保证一次执行最多存在一个暂存对象。
	TaskAttemptID uint64 `gorm:"column:task_attempt_id;not null;uniqueIndex" json:"-"`
	// ObjectKey 是受控存储内部路径，属于敏感实现信息。
	ObjectKey string `gorm:"column:object_key;type:varchar(512);not null;uniqueIndex" json:"-"`
	// StorageBackend 是可替换存储实现编码。
	StorageBackend string `gorm:"column:storage_backend;type:varchar(32);not null" json:"-"`
	// ContainerFormat 是版本化密文容器编码。
	ContainerFormat string `gorm:"column:container_format;type:varchar(32);not null" json:"format"`
	// ContentAlgorithm 是客户端实际使用的文件内容加密算法；后端只保存元数据，不执行加解密。
	ContentAlgorithm string `gorm:"column:content_algorithm;type:varchar(64);not null" json:"content_algorithm"`
	// EncryptionVersion 是密文容器与客户端加密参数版本，用于兼容性校验。
	EncryptionVersion string `gorm:"column:encryption_version;type:varchar(32);not null" json:"encryption_version"`
	// NoncePrefixBase64 是客户端生成的 GCM nonce 前缀；分块 nonce 在客户端派生。
	NoncePrefixBase64 string `gorm:"column:nonce_prefix_base64;type:varchar(64);not null" json:"nonce_prefix_base64"`
	// AuthenticationTagLength 是每个 AES-GCM 分块认证标签的字节长度，标签字节嵌入密文容器。
	AuthenticationTagLength int `gorm:"column:authentication_tag_length;not null" json:"authentication_tag_length"`
	// AADVersion 是密文分块有效认证附加数据的版本，服务端不持有明文或 DEK。
	AADVersion string `gorm:"column:aad_version;type:varchar(32);not null" json:"aad_version"`
	// CiphertextSize 是服务端实际写入字节数。
	CiphertextSize int64 `gorm:"column:ciphertext_size;not null" json:"ciphertext_size"`
	// CiphertextSHA256 是服务端流式复核摘要。
	CiphertextSHA256 string `gorm:"column:ciphertext_sha256;type:char(64);not null" json:"ciphertext_sha256"`
	// Status 控制对象是否允许鉴权下载。
	Status CiphertextObjectStatus `gorm:"column:status;type:varchar(32);not null;index" json:"status"`
	// CreatedAt 是对象登记时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// AvailableAt 是完成事务提交时间。
	AvailableAt *time.Time `gorm:"column:available_at" json:"available_at"`
	// DeletedAt 是补偿删除完成时间。
	DeletedAt *time.Time `gorm:"column:deleted_at" json:"-"`
}

// TableName 指定 CiphertextObject 对应 ciphertext_objects 表。
func (CiphertextObject) TableName() string { return "ciphertext_objects" }
