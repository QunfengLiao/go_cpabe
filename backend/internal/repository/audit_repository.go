package repository

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"

	"go-cpabe/backend/internal/domain"
	"gorm.io/gorm"
)

var (
	// ErrAuditSensitiveMetadata 表示审计元数据包含密钥、Token 或本地路径等禁用字段。
	ErrAuditSensitiveMetadata = errors.New("audit metadata contains sensitive field")
)

const maxAuditMetadataBytes = 16 * 1024

var (
	auditCodePattern  = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)
	auditUUIDPattern  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	auditActionFields = map[string]map[string]struct{}{
		"encryption.task.create":              fieldSet("task_id", "attempt_id", "algorithm_code", "algorithm_version"),
		"encryption.authorization.validated":  fieldSet("task_id", "algorithm_code", "algorithm_version", "recipient_user_id", "public_key_id", "public_key_version", "fingerprint_sha256"),
		"encryption.progress":                 fieldSet("task_id", "attempt_id", "stage", "processed_bytes"),
		"encryption.ciphertext.upload":        fieldSet("task_id", "attempt_id", "ciphertext_size", "ciphertext_sha256"),
		"encryption.aes.complete":             fieldSet("task_id", "attempt_id", "stage", "plaintext_size", "ciphertext_size", "aes_encrypt_ms"),
		"encryption.dek.protect":              fieldSet("task_id", "attempt_id", "algorithm_code", "algorithm_version", "dek_protect_ms", "recipient_count", "protected_key_total_size"),
		"encryption.complete":                 fieldSet("file_id", "task_id", "attempt_id", "ciphertext_size", "algorithm_code"),
		"encryption.cancel":                   fieldSet("task_id", "attempt_id"),
		"encryption.retry":                    fieldSet("task_id", "attempt_id", "attempt_no"),
		"encryption.fail":                     fieldSet("task_id", "attempt_id", "retryable"),
		"encryption.storage.cleanup":          fieldSet("reason_code"),
		"encrypted_file.download":             fieldSet("file_id"),
		"received_file.download":              fieldSet("file_id"),
		"rsa_key.register":                    fieldSet("public_key_id", "public_key_version", "fingerprint_sha256"),
		"rsa_key.recipients.list":             fieldSet(),
		"rsa_key.status.update":               fieldSet("public_key_id", "status"),
		"tenant_member.account_created":       fieldSet("roles", "created_user"),
		"tenant_member.added":                 fieldSet("roles", "created_user"),
		"tenant_user.added":                   fieldSet("tenant_id"),
		"tenant_user.removed":                 fieldSet("tenant_id"),
		"tenant_admin.account_created":        fieldSet("tenant_id"),
		"tenant_admin.assigned":               fieldSet("tenant_id"),
		"tenant_admin.removed":                fieldSet("tenant_id"),
		"tenant.import.template.download":     fieldSet("import_type"),
		"tenant.import.validate":              fieldSet("batch_id", "file_hash", "total_count", "failure_count", "import_type"),
		"tenant.import.confirm":               fieldSet("batch_id", "file_hash", "success_count", "failure_count", "import_type"),
		"tenant.import.completed":             fieldSet("batch_id", "success_count", "import_type"),
		"tenant.import.error_report.download": fieldSet("batch_id", "import_type"),
	}
)

// AuditRepository 定义持久化审计写入与租户范围查询能力。
type AuditRepository interface {
	Create(ctx context.Context, log domain.AuditLog, metadata map[string]any) error
	ListByTenant(ctx context.Context, tenantID uint64, offset, limit int) ([]domain.AuditLog, error)
}

// AuditOutboxWriter 定义已规范化事件的可靠入队能力，数据库审计记录器优先使用该接口而不是直接双写正式日志。
type AuditOutboxWriter interface {
	EnqueueAuditEvent(ctx context.Context, event domain.AuditOutboxEvent) (domain.AuditOutboxEvent, bool, error)
}

// GormAuditRepository 强制审计元数据白名单，避免日志成为密钥或路径泄漏通道。
type GormAuditRepository struct{ db *gorm.DB }

// NewGormAuditRepository 创建数据库审计仓储。
func NewGormAuditRepository(db *gorm.DB) *GormAuditRepository {
	return &GormAuditRepository{db: db}
}

// EnqueueAuditEvent 把已过滤事件写入独立 outbox；正式 audit_logs 只由 Dispatcher 幂等生成。
func (r *GormAuditRepository) EnqueueAuditEvent(ctx context.Context, event domain.AuditOutboxEvent) (domain.AuditOutboxEvent, bool, error) {
	return NewGormAuditOutboxRepository(r.db).Enqueue(ctx, event)
}

// Create 校验并序列化非敏感元数据后写入不可变审计事件。
func (r *GormAuditRepository) Create(ctx context.Context, log domain.AuditLog, metadata map[string]any) error {
	encoded, redacted := PrepareAuditMetadata(log.Action, metadata)
	log.MetadataRedacted = log.MetadataRedacted || redacted
	log.MetadataJSON = encoded
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Create(&log).Error
}

// PrepareAuditMetadata 返回只含动作白名单标量的 JSON；非法输入被清空并显式标记，不向调用方返回原始值。
func PrepareAuditMetadata(action string, metadata map[string]any) ([]byte, bool) {
	safe, redacted := sanitizeAuditMetadata(action, metadata)
	encoded, err := json.Marshal(safe)
	if err != nil || len(encoded) > maxAuditMetadataBytes {
		return []byte("{}"), true
	}
	return encoded, redacted
}

// ListByTenant 只返回指定可信租户事件，并使用稳定倒序分页。
func (r *GormAuditRepository) ListByTenant(ctx context.Context, tenantID uint64, offset, limit int) ([]domain.AuditLog, error) {
	var logs []domain.AuditLog
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, err
}

// sanitizeAuditMetadata 按动作校验非敏感标量；任一字段非法时清空整体元数据，避免部分保留造成上下文误读或秘密泄漏。
func sanitizeAuditMetadata(action string, metadata map[string]any) (map[string]any, bool) {
	allowed, knownAction := auditActionFields[action]
	if len(metadata) == 0 {
		return map[string]any{}, false
	}
	if !knownAction {
		return map[string]any{}, true
	}
	result := make(map[string]any, len(metadata))
	for key, value := range metadata {
		if _, ok := allowed[key]; !ok {
			return map[string]any{}, true
		}
		if !validAuditMetadataValue(key, value) {
			return map[string]any{}, true
		}
		result[key] = value
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > maxAuditMetadataBytes {
		return map[string]any{}, true
	}
	return result, false
}

// fieldSet 构造动作级字段集合；集中声明可以让审计调用新增字段时显式经过安全评审。
func fieldSet(keys ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		result[key] = struct{}{}
	}
	return result
}

// validAuditMetadataValue 校验字段的格式、范围和长度；输入只允许可稳定 JSON 编码的标量。
func validAuditMetadataValue(key string, value any) bool {
	switch key {
	case "retryable", "created_user":
		_, ok := value.(bool)
		return ok
	case "ciphertext_sha256", "fingerprint_sha256":
		text, ok := value.(string)
		if !ok || len(text) != 64 || strings.ToLower(text) != text {
			return false
		}
		decoded, err := hex.DecodeString(text)
		return err == nil && len(decoded) == 32
	case "file_id", "task_id", "attempt_id", "public_key_id":
		text, ok := value.(string)
		return ok && auditUUIDPattern.MatchString(text)
	case "algorithm_code", "algorithm_version", "stage", "reason_code", "status":
		text, ok := value.(string)
		return ok && len(text) >= 1 && len(text) <= 64 && auditCodePattern.MatchString(text)
	case "roles":
		text, ok := value.(string)
		if !ok || len(text) == 0 || len(text) > 64 {
			return false
		}
		for _, role := range strings.Split(text, ",") {
			if role != "DO" && role != "DU" && role != "TENANT_ADMIN" {
				return false
			}
		}
		return true
	case "tenant_id", "attempt_no", "ciphertext_size", "processed_bytes", "aes_encrypt_ms", "dek_protect_ms", "upload_ms", "plaintext_size", "recipient_count", "protected_key_total_size", "recipient_user_id", "public_key_version":
		number, ok := auditNonNegativeInteger(value)
		return ok && number <= math.MaxInt64
	default:
		return false
	}
}

// auditNonNegativeInteger 归一化 JSON 和 Go 常见整数类型；拒绝负数、NaN、无穷和带小数的数值。
func auditNonNegativeInteger(value any) (uint64, bool) {
	switch number := value.(type) {
	case int:
		return uint64(number), number >= 0
	case int32:
		return uint64(number), number >= 0
	case int64:
		return uint64(number), number >= 0
	case uint:
		return uint64(number), true
	case uint32:
		return uint64(number), true
	case uint64:
		return number, true
	case float64:
		return uint64(number), !math.IsNaN(number) && !math.IsInf(number, 0) && number >= 0 && number == math.Trunc(number) && number <= math.MaxInt64
	default:
		return 0, false
	}
}
