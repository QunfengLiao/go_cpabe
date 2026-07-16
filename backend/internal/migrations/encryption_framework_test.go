package migrations

import (
	"context"
	"os"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestEncryptionFrameworkMigrationIsIdempotentAndSeeded 静态检查显式迁移可重复执行并包含首期算法和权限事实源。
func TestEncryptionFrameworkMigrationIsIdempotentAndSeeded(t *testing.T) {
	content, err := os.ReadFile("../../migrations/011_encrypted_file_framework.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(content)
	tables := []string{"encryption_algorithms", "tenant_encryption_algorithms", "rsa_public_keys", "encrypted_files", "encryption_tasks", "encryption_task_attempts", "ciphertext_objects", "protected_keys", "rsa_protected_key_bindings", "encryption_benchmarks", "orphan_storage_objects", "audit_logs"}
	for _, table := range tables {
		if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("missing idempotent table %s", table)
		}
	}
	for _, seed := range []string{"RSA-OAEP-SHA256", "crypto.key.self.manage", "crypto.key.manage", "ON DUPLICATE KEY UPDATE", "INSERT IGNORE INTO role_permissions"} {
		if !strings.Contains(sql, seed) {
			t.Fatalf("missing migration seed or idempotency clause %s", seed)
		}
	}
	if strings.Contains(strings.ToUpper(sql), "TKN20')") || strings.Contains(strings.ToUpper(sql), "CP-ABE', 'ACTIVE") {
		t.Fatal("phase 1 must not seed a fake active CP-ABE algorithm")
	}
}

// TestMultiRecipientMigrationRelaxesSingleProtectedKeyConstraints 静态检查 012 迁移解除单文件
// 单 protected DEK 限制，并把唯一性收敛到“文件 + 接收者 + 公钥版本”的安全边界。
func TestMultiRecipientMigrationRelaxesSingleProtectedKeyConstraints(t *testing.T) {
	content, err := os.ReadFile("../../migrations/012_multi_recipient_file_center.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(content)
	required := []string{
		"uk_protected_keys_file",
		"uk_protected_keys_attempt",
		"idx_protected_keys_file",
		"idx_protected_keys_attempt",
		"file_id BIGINT UNSIGNED",
		"protect_duration_ms BIGINT UNSIGNED",
		"uk_rsa_binding_file_recipient_key",
		"recipient_count BIGINT UNSIGNED",
		"protected_key_total_size_bytes BIGINT UNSIGNED",
	}
	for _, token := range required {
		if !strings.Contains(sql, token) {
			t.Fatalf("012 migration missing token %s", token)
		}
	}
	if !strings.Contains(sql, "DROP INDEX") || !strings.Contains(sql, "UNIQUE INDEX uk_rsa_binding_file_recipient_key") {
		t.Fatalf("012 migration must explicitly drop old uniqueness and add scoped uniqueness:\n%s", sql)
	}
}

// TestDataOwnerCanInvokeReceivedFileDecryptionMigration 验证数据拥有者也能打开“分享给我”入口；
// 真正能否解密仍由 protected DEK 接收者绑定决定，RBAC 只控制是否允许调用流程。
func TestDataOwnerCanInvokeReceivedFileDecryptionMigration(t *testing.T) {
	rbacContent, err := os.ReadFile("../../migrations/010_tenant_rbac.sql")
	if err != nil {
		t.Fatal(err)
	}
	repairContent, err := os.ReadFile("../../migrations/013_do_received_file_permission.sql")
	if err != nil {
		t.Fatal(err)
	}
	rbacSQL := string(rbacContent)
	repairSQL := string(repairContent)
	if !strings.Contains(rbacSQL, "r.code = 'DO'") || !strings.Contains(rbacSQL, "'file.decrypt.invoke'") {
		t.Fatal("010 RBAC seed must include DO file.decrypt.invoke")
	}
	for _, token := range []string{"INSERT IGNORE INTO role_permissions", "r.code = 'DO'", "p.code = 'file.decrypt.invoke'"} {
		if !strings.Contains(repairSQL, token) {
			t.Fatalf("013 migration missing token %s", token)
		}
	}
}

// TestAuditOutboxMigrationIsIdempotentAndIsolated 静态验证审计 Outbox 具备幂等建表、生产与投递去重、
// 并发领取和租约恢复索引；同时确保迁移没有把审计事件塞入会触发密文删除的孤儿对象表。
func TestAuditOutboxMigrationIsIdempotentAndIsolated(t *testing.T) {
	content, err := os.ReadFile("../../migrations/014_audit_outbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(content)
	required := []string{
		"CREATE TABLE IF NOT EXISTS audit_outbox",
		"UNIQUE KEY uk_audit_outbox_event_public_id (event_public_id)",
		"UNIQUE KEY uk_audit_outbox_dedup_key (dedup_key)",
		"KEY idx_audit_outbox_claim (status, next_retry_at, id)",
		"KEY idx_audit_outbox_lease (status, locked_at)",
		"KEY idx_audit_outbox_tenant_status (tenant_id, status, created_at)",
		"metadata_redacted TINYINT(1) NOT NULL DEFAULT 0",
		"CALL cpabe_add_column_if_missing('audit_logs', 'metadata_redacted'",
	}
	for _, token := range required {
		if !strings.Contains(sql, token) {
			t.Fatalf("014 migration missing token %s", token)
		}
	}
	if strings.Contains(sql, "INSERT INTO orphan_storage_objects") || strings.Contains(sql, "ALTER TABLE orphan_storage_objects") {
		t.Fatal("audit outbox migration must not reuse or mutate orphan_storage_objects")
	}
}

// TestAuditOutboxMigrationIsWired 静态验证 014 同时进入显式迁移、开发环境 AutoMigrate 和后置结构校验，
// 防止只创建领域类型却在真实部署路径遗漏表、脱敏标记或调度索引。
func TestAuditOutboxMigrationIsWired(t *testing.T) {
	tests := []struct {
		path   string
		tokens []string
	}{
		{path: "sql_migrations.go", tokens: []string{"014_audit_outbox.sql"}},
		{path: "automigrate.go", tokens: []string{"&domain.AuditOutboxEvent{}", "&domain.AuditLog{}"}},
		{path: "encryption_validation.go", tokens: []string{"validateAuditOutboxSchema", "metadata_redacted", "idx_audit_outbox_claim", "idx_audit_outbox_lease"}},
	}
	for _, test := range tests {
		content, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range test.tokens {
			if !strings.Contains(string(content), token) {
				t.Fatalf("%s missing audit outbox wiring token %s", test.path, token)
			}
		}
	}
}

// TestEncryptionFrameworkMigrationAgainstMySQL 在显式隔离测试库上重复执行迁移并运行全部后置校验。
func TestEncryptionFrameworkMigrationAgainstMySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("设置 TEST_MYSQL_DSN 指向可销毁的隔离 MySQL 测试库后运行集成迁移门禁")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_create_users.sql", "002_create_tenants_roles.sql", "003_add_tenant_query_indexes.sql", "004_create_policy_tables.sql", "005_create_tenant_org_attribute_tables.sql", "006_seed_tenant_org_attribute_demo_data.sql", "007_add_tenant_admin_account_fields.sql", "008_tenant_org_management.sql", "009_add_tenant_branding.sql"} {
		if err := runSQLMigrationFile(db, name); err != nil {
			t.Fatalf("bootstrap migration %s: %v", name, err)
		}
	}
	for run := 1; run <= 2; run++ {
		if err := RunExplicitMigrations(db); err != nil {
			t.Fatalf("migration run %d: %v", run, err)
		}
	}
	if err := ValidateRBACMigration(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEncryptionFrameworkMigration(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}
