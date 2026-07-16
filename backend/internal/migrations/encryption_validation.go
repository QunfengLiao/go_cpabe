package migrations

import (
	"context"
	"errors"
	"fmt"

	"go-cpabe/backend/internal/domain"

	"gorm.io/gorm"
)

// ValidateEncryptionFrameworkMigration 验证加密框架迁移的关键后置条件，避免缺权限或伪算法进入运行态。
func ValidateEncryptionFrameworkMigration(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("database is required")
	}
	var count int64
	if err := db.WithContext(ctx).Table("encryption_algorithms").Where("status = ?", "ACTIVE").Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return errors.New("exactly one encryption algorithm must be active in phase 1")
	}
	if err := db.WithContext(ctx).Table("encryption_algorithms").Where("status = ? AND code <> ?", "ACTIVE", "RSA-OAEP-SHA256").Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return errors.New("placeholder CP-ABE algorithm must not be active")
	}
	if err := db.WithContext(ctx).Table("permissions").Where("code IN ? AND status = ?", []string{"crypto.key.self.manage", "crypto.key.manage"}, "ACTIVE").Count(&count).Error; err != nil {
		return err
	}
	if count != 2 {
		return errors.New("encryption key permissions are incomplete")
	}
	for _, column := range []string{"content_algorithm", "encryption_version", "nonce_prefix_base64", "authentication_tag_length", "aad_version"} {
		if !db.Migrator().HasColumn(&domain.CiphertextObject{}, column) {
			return fmt.Errorf("ciphertext metadata column %s is missing", column)
		}
	}
	return validateAuditOutboxSchema(db)
}

// validateAuditOutboxSchema 检查可靠审计所依赖的独立表、脱敏标记和调度索引；
// 该后置校验只读取数据库结构，缺失任一约束都阻止迁移命令报告成功，避免运行时静默丢失幂等或租约语义。
func validateAuditOutboxSchema(db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable(&domain.AuditOutboxEvent{}) {
		return errors.New("audit outbox table is missing")
	}
	for _, column := range []string{"event_public_id", "dedup_key", "metadata_redacted", "payload_version", "occurred_at", "lock_token"} {
		if !migrator.HasColumn(&domain.AuditOutboxEvent{}, column) {
			return fmt.Errorf("audit outbox column %s is missing", column)
		}
	}
	if !migrator.HasColumn(&domain.AuditLog{}, "metadata_redacted") {
		return errors.New("audit log metadata redaction marker is missing")
	}
	for _, index := range []string{
		"uk_audit_outbox_event_public_id",
		"uk_audit_outbox_dedup_key",
		"idx_audit_outbox_claim",
		"idx_audit_outbox_lease",
		"idx_audit_outbox_tenant_status",
	} {
		if !migrator.HasIndex(&domain.AuditOutboxEvent{}, index) {
			return fmt.Errorf("audit outbox index %s is missing", index)
		}
	}
	return nil
}
