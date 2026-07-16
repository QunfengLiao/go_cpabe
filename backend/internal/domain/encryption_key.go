package domain

import "time"

// EncryptionAlgorithm 描述可供租户选择的真实 DEK 保护算法版本。
type EncryptionAlgorithm struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// Code 是稳定算法编码。
	Code string `gorm:"column:code;type:varchar(64);not null;uniqueIndex:uk_algorithm_code_version,priority:1" json:"code"`
	// DisplayName 是中文展示名称。
	DisplayName string `gorm:"column:display_name;type:varchar(128);not null" json:"display_name"`
	// Category 区分公钥和 CP-ABE 等算法类别。
	Category string `gorm:"column:category;type:varchar(32);not null" json:"category"`
	// Version 是算法参数协议版本。
	Version string `gorm:"column:version;type:varchar(32);not null;uniqueIndex:uk_algorithm_code_version,priority:2" json:"version"`
	// AuthorizationType 驱动前端动态授权表单。
	AuthorizationType string `gorm:"column:authorization_type;type:varchar(64);not null" json:"authorization_type"`
	// ProtectedKeyFormat 描述引擎输出格式。
	ProtectedKeyFormat string `gorm:"column:protected_key_format;type:varchar(64);not null" json:"protected_key_format"`
	// ClientRuntime 首期固定为 LOCAL_GO_WORKER。
	ClientRuntime string `gorm:"column:client_runtime;type:varchar(32);not null" json:"client_runtime"`
	// Status 控制算法是否可供新租户配置。
	Status string `gorm:"column:status;type:varchar(32);not null;index" json:"status"`
	// CreatedAt 是算法登记时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是算法配置更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定 EncryptionAlgorithm 对应 encryption_algorithms 表。
func (EncryptionAlgorithm) TableName() string { return "encryption_algorithms" }

// TenantEncryptionAlgorithm 表示某算法版本是否允许在特定租户使用。
type TenantEncryptionAlgorithm struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// TenantID 是可信租户标识。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_tenant_algorithm,priority:1" json:"-"`
	// AlgorithmID 指向真实算法目录。
	AlgorithmID uint64 `gorm:"column:algorithm_id;not null;uniqueIndex:uk_tenant_algorithm,priority:2" json:"-"`
	// Enabled 控制当前租户新任务是否可使用该算法。
	Enabled bool `gorm:"column:enabled;not null" json:"enabled"`
	// DisabledReason 是可向用户展示的非敏感原因。
	DisabledReason string `gorm:"column:disabled_reason;type:varchar(255)" json:"unavailable_reason"`
	// CreatedAt 是租户配置创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是租户配置更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定 TenantEncryptionAlgorithm 对应 tenant_encryption_algorithms 表。
func (TenantEncryptionAlgorithm) TableName() string { return "tenant_encryption_algorithms" }

// RSAPublicKey 保存租户成员的 RSA 公钥历史，绝不保存私钥。
type RSAPublicKey struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// PublicID 是 API 公钥 UUID。
	PublicID string `gorm:"column:public_id;type:char(36);not null;uniqueIndex" json:"id"`
	// TenantID 是公钥所属租户，参与所有查询。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_rsa_key_version,priority:1;uniqueIndex:uk_rsa_key_fingerprint,priority:1;index" json:"-"`
	// UserID 是公钥所属成员。
	UserID uint64 `gorm:"column:user_id;not null;uniqueIndex:uk_rsa_key_version,priority:2;index" json:"user_id"`
	// Version 是同一租户成员内递增版本。
	Version uint32 `gorm:"column:version;not null;uniqueIndex:uk_rsa_key_version,priority:3" json:"version"`
	// FingerprintSHA256 是规范 SPKI DER 的服务端重算摘要。
	FingerprintSHA256 string `gorm:"column:fingerprint_sha256;type:char(64);not null;uniqueIndex:uk_rsa_key_fingerprint,priority:2" json:"fingerprint_sha256"`
	// PublicKeyPEM 是 SPKI 公钥，仅授权接口返回。
	PublicKeyPEM string `gorm:"column:public_key_pem;type:text;not null" json:"public_key_pem"`
	// KeyBits 首期必须为 3072。
	KeyBits uint16 `gorm:"column:key_bits;not null" json:"key_bits"`
	// Algorithm 固定记录 RSA-OAEP-SHA256。
	Algorithm string `gorm:"column:algorithm;type:varchar(64);not null" json:"algorithm"`
	// Status 控制是否允许创建新任务，历史引用仍保留。
	Status string `gorm:"column:status;type:varchar(32);not null;index" json:"status"`
	// CreatedBy 是登记操作者。
	CreatedBy uint64 `gorm:"column:created_by;not null" json:"-"`
	// DisabledBy 是禁用或撤销操作者，可空。
	DisabledBy *uint64 `gorm:"column:disabled_by" json:"-"`
	// DisabledAt 是禁用时间，可空。
	DisabledAt *time.Time `gorm:"column:disabled_at" json:"disabled_at"`
	// CreatedAt 是公钥登记时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是状态更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定 RSAPublicKey 对应 rsa_public_keys 表。
func (RSAPublicKey) TableName() string { return "rsa_public_keys" }

// ProtectedKey 保存算法无关受保护 DEK；ProtectedKeyBytes 默认不返回外部响应。
type ProtectedKey struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// PublicID 是外部 UUID。
	PublicID string `gorm:"column:public_id;type:char(36);not null;uniqueIndex" json:"id"`
	// TenantID 强制密钥记录租户隔离。
	TenantID uint64 `gorm:"column:tenant_id;not null;index" json:"-"`
	// FileID 关联完成文件；多接收者场景下同一文件会有多条受保护 DEK。
	FileID uint64 `gorm:"column:file_id;not null;index" json:"-"`
	// TaskAttemptID 关联生成执行；一次执行可为多个接收者产生多条受保护 DEK。
	TaskAttemptID uint64 `gorm:"column:task_attempt_id;not null;index" json:"-"`
	// AlgorithmCode 是引擎编码快照。
	AlgorithmCode string `gorm:"column:algorithm_code;type:varchar(64);not null" json:"algorithm_code"`
	// AlgorithmVersion 是引擎版本快照。
	AlgorithmVersion string `gorm:"column:algorithm_version;type:varchar(32);not null" json:"algorithm_version"`
	// ProtectedKeyFormat 是受保护密钥格式。
	ProtectedKeyFormat string `gorm:"column:protected_key_format;type:varchar(64);not null" json:"protected_key_format"`
	// ProtectedKeyBytes 是受保护 DEK，仍属于敏感材料且默认隐藏。
	ProtectedKeyBytes []byte `gorm:"column:protected_key;type:blob;not null" json:"-"`
	// ContextSHA256 绑定文件、任务和授权快照。
	ContextSHA256 string `gorm:"column:context_sha256;type:char(64);not null" json:"context_sha256"`
	// CreatedAt 是密钥记录创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定 ProtectedKey 对应 protected_keys 表。
func (ProtectedKey) TableName() string { return "protected_keys" }

// RSAProtectedKeyBinding 保存 RSA 专属关系，避免污染通用 ProtectedKey。
type RSAProtectedKeyBinding struct {
	// ID 是内部主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	// TenantID 是绑定所属租户，参与接收者唯一性判断。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_rsa_binding_file_recipient_key,priority:1;index" json:"-"`
	// FileID 冗余记录文件维度，避免查询唯一接收者时必须回表通用 protected_keys。
	FileID uint64 `gorm:"column:file_id;not null;uniqueIndex:uk_rsa_binding_file_recipient_key,priority:2;index:idx_rsa_binding_file,priority:2" json:"-"`
	// ProtectedKeyID 一对一指向通用受保护密钥。
	ProtectedKeyID uint64 `gorm:"column:protected_key_id;not null;uniqueIndex" json:"-"`
	// RecipientUserID 是加密时接收用户。
	RecipientUserID uint64 `gorm:"column:recipient_user_id;not null;uniqueIndex:uk_rsa_binding_file_recipient_key,priority:3;index" json:"recipient_user_id"`
	// RSAPublicKeyID 指向加密时具体公钥版本。
	RSAPublicKeyID uint64 `gorm:"column:rsa_public_key_id;not null;uniqueIndex:uk_rsa_binding_file_recipient_key,priority:4;index" json:"-"`
	// PublicKeyFingerprintSHA256 固化公钥指纹。
	PublicKeyFingerprintSHA256 string `gorm:"column:public_key_fingerprint_sha256;type:char(64);not null" json:"public_key_fingerprint_sha256"`
	// OAEPHash 固定记录 SHA-256。
	OAEPHash string `gorm:"column:oaep_hash;type:varchar(32);not null" json:"oaep_hash"`
	// OAEPLabelSHA256 记录上下文 label 摘要。
	OAEPLabelSHA256 string `gorm:"column:oaep_label_sha256;type:char(64);not null" json:"oaep_label_sha256"`
	// ProtectDurationMS 记录该接收者 RSA-OAEP 封装耗时，用于算法对比且不参与鉴权。
	ProtectDurationMS int64 `gorm:"column:protect_duration_ms;not null" json:"protect_duration_ms"`
	// CreatedAt 是绑定创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定 RSAProtectedKeyBinding 对应 rsa_protected_key_bindings 表。
func (RSAProtectedKeyBinding) TableName() string { return "rsa_protected_key_bindings" }
