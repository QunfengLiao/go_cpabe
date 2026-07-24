package service

import (
	"testing"

	"go-cpabe/backend/internal/domain"
	"gorm.io/gorm"
)

// TestValidateImportUserIdentityConflictsRejectsEmailOwner 验证邮箱归属于其他用户名时预校验失败，避免 MySQL 唯一键命中错误账号。
func TestValidateImportUserIdentityConflictsRejectsEmailOwner(t *testing.T) {
	results := []domain.ImportRowResult{
		{RowNumber: 5, Key: "new.user", Status: domain.ImportRowValid, Fields: map[string]string{"email": "owned@example.com"}},
	}
	persisted := []domain.User{{ID: 7, Username: "existing.user", Email: "owned@example.com"}}

	validateImportUserIdentityConflicts(results, persisted)

	if results[0].Status != domain.ImportRowInvalid || !importRowHasErrorCode(results[0], "EMAIL_OWNED_BY_OTHER_USER") {
		t.Fatalf("邮箱跨用户名冲突必须被拒绝: %+v", results[0])
	}
}

// TestValidateImportUserIdentityConflictsRejectsDuplicateEmail 验证同一文件的重复邮箱会同时标记相关行，管理员无需猜测首个占用行。
func TestValidateImportUserIdentityConflictsRejectsDuplicateEmail(t *testing.T) {
	results := []domain.ImportRowResult{
		{RowNumber: 5, Key: "first.user", Status: domain.ImportRowValid, Fields: map[string]string{"email": "same@example.com"}},
		{RowNumber: 6, Key: "second.user", Status: domain.ImportRowValid, Fields: map[string]string{"email": "same@example.com"}},
	}

	validateImportUserIdentityConflicts(results, nil)

	for _, result := range results {
		if result.Status != domain.ImportRowInvalid || !importRowHasErrorCode(result, "DUPLICATE_EMAIL") {
			t.Fatalf("重复邮箱相关行都必须失败: %+v", results)
		}
	}
}

// TestValidateImportUserIdentityConflictsRejectsDeletedUsername 验证软删除账号不会被导入流程隐式恢复。
func TestValidateImportUserIdentityConflictsRejectsDeletedUsername(t *testing.T) {
	results := []domain.ImportRowResult{
		{RowNumber: 5, Key: "deleted.user", Status: domain.ImportRowValid, Fields: map[string]string{"email": "deleted@example.com"}},
	}
	persisted := []domain.User{{ID: 8, Username: "deleted.user", Email: "deleted@example.com", DeletedAt: gorm.DeletedAt{Valid: true}}}

	validateImportUserIdentityConflicts(results, persisted)

	if results[0].Status != domain.ImportRowInvalid || !importRowHasErrorCode(results[0], "USERNAME_DELETED") {
		t.Fatalf("软删除用户名必须显式拒绝: %+v", results[0])
	}
}

// TestEnsureImportUsersResolvedRejectsMissingID 验证 UPSERT 后任一用户名缺失或主键为零都会阻断后续关系写入。
func TestEnsureImportUsersResolvedRejectsMissingID(t *testing.T) {
	rows := []domain.ImportRowResult{
		{RowNumber: 5, Key: "resolved.user"},
		{RowNumber: 6, Key: "missing.user"},
	}
	users := map[string]domain.User{"resolved.user": {ID: 10, Username: "resolved.user"}}

	if err := ensureImportUsersResolved(rows, users); err == nil {
		t.Fatal("缺失用户主键时必须返回错误并触发事务回滚")
	}
	users["missing.user"] = domain.User{ID: 0, Username: "missing.user"}
	if err := ensureImportUsersResolved(rows, users); err == nil {
		t.Fatal("零用户主键必须被拒绝")
	}
}

// importRowHasErrorCode 判断测试行是否包含指定稳定错误码。
func importRowHasErrorCode(row domain.ImportRowResult, code string) bool {
	for _, item := range row.Errors {
		if item.Code == code {
			return true
		}
	}
	return false
}
