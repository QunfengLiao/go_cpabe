package migrations

import (
	"os"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestAuditOutboxMigrationAgainstMySQL 在隔离 MySQL 中重复执行 014，并核对可靠领取与幂等所依赖的真实列和索引。
func TestAuditOutboxMigrationAgainstMySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("设置 TEST_MYSQL_DSN 指向可销毁的隔离 MySQL 测试库后运行迁移集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// 014 只扩展既有 audit_logs；隔离库若未跑历史迁移，先创建满足前置条件的最小表。
	if err := db.Exec("CREATE TABLE IF NOT EXISTS audit_logs (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, metadata JSON NOT NULL, PRIMARY KEY (id)) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	if err := runSQLMigrationFile(db, "014_audit_outbox.sql"); err != nil {
		t.Fatal(err)
	}
	if err := runSQLMigrationFile(db, "014_audit_outbox.sql"); err != nil {
		t.Fatalf("014 migration must be idempotent: %v", err)
	}

	for _, column := range []string{"event_public_id", "dedup_key", "tenant_id", "metadata_redacted", "status", "retry_count", "next_retry_at", "locked_at", "lock_token", "delivered_at"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'audit_outbox' AND column_name = ?", column).Scan(&count).Error; err != nil || count != 1 {
			t.Fatalf("missing audit_outbox column %s: count=%d err=%v", column, count, err)
		}
	}
	for _, index := range []string{"uk_audit_outbox_event_public_id", "uk_audit_outbox_dedup_key", "idx_audit_outbox_claim", "idx_audit_outbox_lease", "idx_audit_outbox_tenant_status"} {
		var count int64
		if err := db.Raw("SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'audit_outbox' AND index_name = ?", index).Scan(&count).Error; err != nil || count != 1 {
			t.Fatalf("missing audit_outbox index %s: count=%d err=%v", index, count, err)
		}
	}
}
