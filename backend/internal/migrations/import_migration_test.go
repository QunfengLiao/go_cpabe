package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTenantImportMigrationShape 验证批次表具备租户隔离、快照完整性、进度和租约恢复所需字段。
func TestTenantImportMigrationShape(t *testing.T) {
	var sqlBuilder strings.Builder
	for _, name := range []string{"017_tenant_import.sql", "018_async_tenant_import.sql"} {
		path := filepath.Join("..", "..", "migrations", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		sqlBuilder.Write(data)
	}
	sql := strings.ToLower(sqlBuilder.String())
	for _, token := range []string{"tenant_import_batches", "tenant_id", "created_by", "file_hash", "snapshot_hash", "rows_json", "tenant.import.manage", "phase", "processed_count", "lease_token", "lease_expires_at", "heartbeat_at", "attempt_count", "idx_tenant_import_batches_lease"} {
		if !strings.Contains(sql, token) {
			t.Fatalf("migration missing %s", token)
		}
	}
}

// TestZeroUserRelationRepairMigration 验证历史修复只按零用户主键清理受导入影响的关系表。
func TestZeroUserRelationRepairMigration(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "019_repair_zero_user_relations.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read zero user repair migration: %v", err)
	}
	sql := strings.ToLower(string(data))
	for _, table := range []string{"tenant_users", "user_roles", "tenant_org_members", "tenant_org_member_roles", "user_attributes"} {
		if !strings.Contains(sql, "delete from "+table+" where user_id = 0") {
			t.Fatalf("repair migration missing exact zero-user cleanup for %s", table)
		}
	}
	for _, unsafe := range []string{"username =", "email =", " like "} {
		if strings.Contains(sql, unsafe) {
			t.Fatalf("repair migration must not use broad identity condition %q", unsafe)
		}
	}
}
