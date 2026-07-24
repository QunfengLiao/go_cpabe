package repository

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestImportLeaseResultRequiresSingleOwner 验证租约终态更新必须恰好命中一行，旧 Worker 的令牌不能静默成功。
func TestImportLeaseResultRequiresSingleOwner(t *testing.T) {
	if err := importLeaseResult(&gorm.DB{RowsAffected: 1}); err != nil {
		t.Fatalf("有效租约更新被拒绝: %v", err)
	}
	if err := importLeaseResult(&gorm.DB{RowsAffected: 0}); !errors.Is(err, ErrImportBatchState) {
		t.Fatalf("失效租约应返回状态冲突，实际=%v", err)
	}
	databaseError := errors.New("database unavailable")
	if err := importLeaseResult(&gorm.DB{Error: databaseError}); !errors.Is(err, databaseError) {
		t.Fatalf("数据库错误应原样返回，实际=%v", err)
	}
}

// TestImportBatchClaimCandidateQuerySelectsOnlyID 验证领取排序只携带主键，防止 rows_json 大字段进入 MySQL filesort。
func TestImportBatchClaimCandidateQuerySelectsOnlyID(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: "user:password@tcp(127.0.0.1:3306)/test?parseTime=true", SkipInitializeWithVersion: true}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		var candidate importBatchClaimCandidate
		return importBatchClaimCandidateQuery(tx, time.Date(2026, 7, 23, 14, 29, 33, 0, time.UTC)).Find(&candidate)
	})
	lowerSQL := strings.ToLower(sql)
	if !strings.Contains(lowerSQL, "select `id` from `tenant_import_batches`") {
		t.Fatalf("领取查询必须只选择主键，SQL=%s", sql)
	}
	if strings.Contains(lowerSQL, "rows_json") || strings.Contains(lowerSQL, "select *") {
		t.Fatalf("领取排序不得加载完整快照，SQL=%s", sql)
	}
}
