package service

import (
	"context"
	"fmt"
	"strings"

	"go-cpabe/backend/internal/domain"
	"gorm.io/gorm"
)

const importIdentityLookupChunkSize = 500

// loadImportIdentityUsers 分块读取用户名或邮箱相关的全部账号（含软删除），用于在预校验阶段模拟 MySQL 所有唯一键冲突。
func loadImportIdentityUsers(ctx context.Context, db *gorm.DB, rows []domain.ImportRowResult) ([]domain.User, error) {
	usersByID := make(map[uint64]domain.User)
	for start := 0; start < len(rows); start += importIdentityLookupChunkSize {
		end := start + importIdentityLookupChunkSize
		if end > len(rows) {
			end = len(rows)
		}
		usernames := make([]string, 0, end-start)
		emails := make([]string, 0, end-start)
		for _, row := range rows[start:end] {
			if username := strings.TrimSpace(row.Key); username != "" {
				usernames = append(usernames, username)
			}
			if email := effectiveImportEmail(row); email != "" {
				emails = append(emails, email)
			}
		}
		if len(usernames) == 0 && len(emails) == 0 {
			continue
		}
		var chunk []domain.User
		if err := db.WithContext(ctx).Unscoped().
			Select("id, username, email, deleted_at").
			Where("username IN ? OR email IN ?", usernames, emails).
			Find(&chunk).Error; err != nil {
			return nil, err
		}
		for _, user := range chunk {
			usersByID[user.ID] = user
		}
	}
	users := make([]domain.User, 0, len(usersByID))
	for _, user := range usersByID {
		users = append(users, user)
	}
	return users, nil
}

// validateImportUserIdentityConflicts 把文件内和数据库内的唯一身份冲突写回行错误，防止邮箱冲突被 MySQL 误当作用户名 UPSERT。
func validateImportUserIdentityConflicts(results []domain.ImportRowResult, persisted []domain.User) {
	byUsername := make(map[string]domain.User, len(persisted))
	byEmail := make(map[string]domain.User, len(persisted))
	for _, user := range persisted {
		byUsername[normalizeImportIdentity(user.Username)] = user
		byEmail[normalizeImportIdentity(user.Email)] = user
	}

	firstEmailRow := make(map[string]int, len(results))
	for index := range results {
		usernameKey := normalizeImportIdentity(results[index].Key)
		emailKey := normalizeImportIdentity(effectiveImportEmail(results[index]))
		if emailKey != "" {
			if firstIndex, exists := firstEmailRow[emailKey]; exists {
				addImportErrorIfMissing(&results[firstIndex], "email", "DUPLICATE_EMAIL", fmt.Sprintf("与第 %d 行使用相同邮箱", results[index].RowNumber))
				addImportErrorIfMissing(&results[index], "email", "DUPLICATE_EMAIL", fmt.Sprintf("与第 %d 行使用相同邮箱", results[firstIndex].RowNumber))
			} else {
				firstEmailRow[emailKey] = index
			}
		}
		if existing, exists := byUsername[usernameKey]; exists && existing.DeletedAt.Valid {
			addImportErrorIfMissing(&results[index], "username", "USERNAME_DELETED", "用户名对应账号已删除，不能通过导入隐式恢复")
		}
		if owner, exists := byEmail[emailKey]; exists && normalizeImportIdentity(owner.Username) != usernameKey {
			addImportErrorIfMissing(&results[index], "email", "EMAIL_OWNED_BY_OTHER_USER", "邮箱已被其他用户名使用")
		}
	}
}

// ensureImportUsersResolved 在任何成员、角色、组织或属性写入前验证用户名到主键的完整映射，错误会使外层事务整体回滚。
func ensureImportUsersResolved(rows []domain.ImportRowResult, users map[string]domain.User) error {
	for _, row := range rows {
		user, exists := users[row.Key]
		if !exists || user.ID == 0 {
			return fmt.Errorf("导入用户身份解析失败，行号=%d", row.RowNumber)
		}
	}
	return nil
}

// effectiveImportEmail 返回写入 users 表时实际使用的邮箱，空邮箱按新增用户规则生成稳定占位值。
func effectiveImportEmail(row domain.ImportRowResult) string {
	if email := strings.TrimSpace(row.Fields["email"]); email != "" {
		return email
	}
	if username := strings.TrimSpace(row.Key); username != "" {
		return username + "@import.invalid"
	}
	return ""
}

// normalizeImportIdentity 按 MySQL 常用不区分大小写排序规则归一化用户名和邮箱比较键。
func normalizeImportIdentity(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// addImportErrorIfMissing 添加稳定行错误并避免同一检查重复写入相同错误码。
func addImportErrorIfMissing(result *domain.ImportRowResult, field, code, message string) {
	for _, item := range result.Errors {
		if item.Code == code {
			return
		}
	}
	addImportError(result, field, code, message)
}
