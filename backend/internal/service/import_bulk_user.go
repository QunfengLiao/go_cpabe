package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// importUsersBulkTx 以集合查询和分批 UPSERT 导入用户，事务边界仍覆盖成员、角色、组织和属性事实。
func (s *ImportService) importUsersBulkTx(ctx context.Context, tx *gorm.DB, tenantID, actorID uint64, rows []domain.ImportRowResult, batchSize int, progress func(string, int) error) error {
	if batchSize <= 0 {
		batchSize = 300
	}
	userByName, err := loadImportUsers(ctx, tx, rows)
	if err != nil {
		return err
	}
	users, err := buildImportUsers(rows, userByName)
	if err != nil {
		return err
	}
	if err := upsertImportUsers(ctx, tx, users, batchSize); err != nil {
		return err
	}
	if err := reportImportProgress(progress, "USERS", len(rows)/5); err != nil {
		return err
	}
	userByName, err = loadImportUsers(ctx, tx, rows)
	if err != nil {
		return err
	}
	if err := ensureImportUsersResolved(rows, userByName); err != nil {
		return err
	}
	if err := s.upsertImportTenantMembers(ctx, tx, tenantID, rows, userByName, batchSize); err != nil {
		return err
	}
	if err := reportImportProgress(progress, "MEMBERS", len(rows)*2/5); err != nil {
		return err
	}
	if err := s.replaceImportRolesBulkTx(ctx, tx, tenantID, actorID, rows, userByName, batchSize); err != nil {
		return err
	}
	if err := reportImportProgress(progress, "ROLES", len(rows)*3/5); err != nil {
		return err
	}
	if err := s.ensureImportOrgMembersBulkTx(ctx, tx, tenantID, rows, userByName, batchSize); err != nil {
		return err
	}
	if err := reportImportProgress(progress, "ORGANIZATIONS", len(rows)*4/5); err != nil {
		return err
	}
	// 属性通常是稀疏字段，沿用已校验的写入函数可以保持 CP-ABE 属性来源语义不变。
	for _, row := range rows {
		if strings.TrimSpace(row.Fields["attributes"]) == "" || strings.EqualFold(strings.TrimSpace(row.Fields["member_status"]), "DISABLED") {
			continue
		}
		if err := s.replaceImportAttributesTx(ctx, tx, tenantID, userByName[row.Key].ID, row.Fields["attributes"]); err != nil {
			return err
		}
	}
	return reportImportProgress(progress, "FINALIZING", len(rows))
}

// reportImportProgress 把阶段进度交给 Worker 持久化；测试或同步调用可传 nil。
func reportImportProgress(progress func(string, int) error, phase string, processed int) error {
	if progress == nil {
		return nil
	}
	return progress(phase, processed)
}

// loadImportUsers 一次性读取批次用户名，避免逐行查询造成数据库往返放大。
func loadImportUsers(ctx context.Context, tx *gorm.DB, rows []domain.ImportRowResult) (map[string]domain.User, error) {
	usernames := make([]string, 0, len(rows))
	for _, row := range rows {
		usernames = append(usernames, row.Key)
	}
	var users []domain.User
	if err := tx.WithContext(ctx).Where("username IN ?", usernames).Find(&users).Error; err != nil {
		return nil, err
	}
	result := make(map[string]domain.User, len(users))
	for _, user := range users {
		result[user.Username] = user
	}
	return result, nil
}

// buildImportUsers 构建 UPSERT 输入；新用户只接受预校验阶段生成的密码摘要，缺失时生成随机临时密码摘要。
func buildImportUsers(rows []domain.ImportRowResult, existing map[string]domain.User) ([]domain.User, error) {
	users := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		fields := row.Fields
		current, found := existing[row.Key]
		email := strings.TrimSpace(fields["email"])
		passwordHash := current.PasswordHash
		if !found {
			if email == "" {
				email = row.Key + "@import.invalid"
			}
			passwordHash = strings.TrimSpace(fields["initial_password_hash"])
			if passwordHash == "" {
				var err error
				passwordHash, err = auth.HashPassword(generateImportPassword())
				if err != nil {
					return nil, err
				}
			}
		} else if email == "" {
			email = current.Email
		}
		users = append(users, domain.User{Username: row.Key, Email: email, PasswordHash: passwordHash, Nickname: strings.TrimSpace(fields["display_name"]), Phone: strings.TrimSpace(fields["phone"]), Role: domain.RoleDataUser, Status: domain.StatusActive, MustChangePassword: current.MustChangePassword})
	}
	return users, nil
}

// upsertImportUsers 分批写入用户基础资料，密码摘要不会因更新既有用户而被覆盖。
func upsertImportUsers(ctx context.Context, tx *gorm.DB, users []domain.User, batchSize int) error {
	return tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "username"}}, DoUpdates: clause.AssignmentColumns([]string{"email", "nickname", "phone", "updated_at"})}).CreateInBatches(&users, batchSize).Error
}

// upsertImportTenantMembers 批量恢复或更新租户成员状态，确保所有后续关系都绑定当前租户。
func (s *ImportService) upsertImportTenantMembers(ctx context.Context, tx *gorm.DB, tenantID uint64, rows []domain.ImportRowResult, users map[string]domain.User, batchSize int) error {
	members := make([]domain.TenantUser, 0, len(rows))
	for _, row := range rows {
		status := domain.TenantUserStatusActive
		if strings.EqualFold(strings.TrimSpace(row.Fields["member_status"]), "DISABLED") {
			status = domain.TenantUserStatusDisabled
		}
		members = append(members, domain.TenantUser{TenantID: tenantID, UserID: users[row.Key].ID, Status: status})
	}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"status", "updated_at", "deleted_at"})}).CreateInBatches(&members, batchSize).Error
}

// replaceImportRolesBulkTx 先保护最后一个租户管理员，再批量撤销并恢复本次 active 成员的目标角色。
func (s *ImportService) replaceImportRolesBulkTx(ctx context.Context, tx *gorm.DB, tenantID, actorID uint64, rows []domain.ImportRowResult, users map[string]domain.User, batchSize int) error {
	codeSet := map[string]bool{}
	activeIDs := make([]uint64, 0, len(rows))
	desiredByUser := make(map[uint64][]string, len(rows))
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.Fields["member_status"]), "DISABLED") {
			continue
		}
		userID := users[row.Key].ID
		codes := splitComma(row.Fields["role_codes"])
		activeIDs = append(activeIDs, userID)
		desiredByUser[userID] = codes
		for _, code := range codes {
			codeSet[code] = true
		}
	}
	if len(activeIDs) == 0 {
		return nil
	}
	codes := make([]string, 0, len(codeSet))
	for code := range codeSet {
		codes = append(codes, code)
	}
	var roles []domain.Role
	if err := tx.WithContext(ctx).Where("code IN ? AND scope_type = ? AND status = ? AND code <> ? AND (tenant_id = 0 OR tenant_id = ?)", codes, domain.RoleScopeTypeTenant, domain.RoleStatusActive, domain.RolePlatformAdmin, tenantID).Find(&roles).Error; err != nil {
		return err
	}
	roleByCode := make(map[string]domain.Role, len(roles))
	for _, role := range roles {
		roleByCode[string(role.Code)] = role
	}
	if len(roleByCode) != len(codeSet) {
		return fmt.Errorf("批次包含不存在或不可分配的租户角色")
	}
	if err := protectLastImportTenantAdmin(ctx, tx, tenantID, desiredByUser, roleByCode); err != nil {
		return err
	}
	now := time.Now()
	if err := tx.WithContext(ctx).Model(&domain.UserRoleAssignment{}).Where("tenant_id = ? AND user_id IN ? AND status = ?", tenantID, activeIDs, domain.UserRoleStatusActive).Updates(map[string]any{"status": domain.UserRoleStatusRevoked, "revoked_at": now, "updated_at": now}).Error; err != nil {
		return err
	}
	assignments := make([]domain.UserRoleAssignment, 0, len(activeIDs)*2)
	for userID, desiredCodes := range desiredByUser {
		for _, code := range desiredCodes {
			tenant := tenantID
			actor := actorID
			assignments = append(assignments, domain.UserRoleAssignment{TenantID: &tenant, UserID: userID, RoleID: roleByCode[code].ID, AssignmentSource: domain.AssignmentSourceManual, AssignedBy: &actor, Status: domain.UserRoleStatusActive})
		}
	}
	if len(assignments) == 0 {
		return nil
	}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "role_id"}}, DoUpdates: clause.Assignments(map[string]any{"assignment_source": domain.AssignmentSourceManual, "assigned_by": actorID, "status": domain.UserRoleStatusActive, "revoked_at": nil, "updated_at": now, "deleted_at": nil})}).CreateInBatches(&assignments, batchSize).Error
}

// protectLastImportTenantAdmin 防止批量角色替换移除租户唯一有效管理员。
func protectLastImportTenantAdmin(ctx context.Context, tx *gorm.DB, tenantID uint64, desired map[uint64][]string, roles map[string]domain.Role) error {
	adminRole, exists := roles[string(domain.RoleTenantAdmin)]
	if !exists {
		var role domain.Role
		if err := tx.WithContext(ctx).Where("code = ? AND scope_type = ? AND status = ? AND (tenant_id = 0 OR tenant_id = ?)", domain.RoleTenantAdmin, domain.RoleScopeTypeTenant, domain.RoleStatusActive, tenantID).First(&role).Error; err != nil {
			return err
		}
		adminRole = role
	}
	var adminIDs []uint64
	if err := tx.WithContext(ctx).Model(&domain.UserRoleAssignment{}).Where("tenant_id = ? AND role_id = ? AND status = ?", tenantID, adminRole.ID, domain.UserRoleStatusActive).Pluck("user_id", &adminIDs).Error; err != nil {
		return err
	}
	if len(adminIDs) != 1 {
		return nil
	}
	codes, affected := desired[adminIDs[0]]
	if !affected {
		return nil
	}
	for _, code := range codes {
		if code == string(domain.RoleTenantAdmin) {
			return nil
		}
	}
	return repository.ErrCannotRemoveLastTenantAdmin
}

// ensureImportOrgMembersBulkTx 批量建立组织关系，并只在用户尚无 active 组织时设置主组织。
func (s *ImportService) ensureImportOrgMembersBulkTx(ctx context.Context, tx *gorm.DB, tenantID uint64, rows []domain.ImportRowResult, users map[string]domain.User, batchSize int) error {
	codeSet := map[string]bool{}
	for _, row := range rows {
		if code := strings.TrimSpace(row.Fields["org_code"]); code != "" && !strings.EqualFold(strings.TrimSpace(row.Fields["member_status"]), "DISABLED") {
			codeSet[code] = true
		}
	}
	codes := make([]string, 0, len(codeSet))
	for code := range codeSet {
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return nil
	}
	var orgs []domain.TenantOrgUnit
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND code IN ? AND status = ?", tenantID, codes, domain.OrgUnitStatusEnabled).Find(&orgs).Error; err != nil {
		return err
	}
	orgByCode := make(map[string]domain.TenantOrgUnit, len(orgs))
	for _, org := range orgs {
		orgByCode[org.Code] = org
	}
	if len(orgByCode) != len(codeSet) {
		return fmt.Errorf("批次包含不存在或未启用的组织编码")
	}
	userIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		userIDs = append(userIDs, users[row.Key].ID)
	}
	var existingPrimary []uint64
	if err := tx.WithContext(ctx).Model(&domain.TenantOrgMember{}).Where("tenant_id = ? AND user_id IN ? AND status = ?", tenantID, userIDs, domain.OrgMemberStatusActive).Distinct().Pluck("user_id", &existingPrimary).Error; err != nil {
		return err
	}
	hasOrg := make(map[uint64]bool, len(existingPrimary))
	for _, userID := range existingPrimary {
		hasOrg[userID] = true
	}
	members := make([]domain.TenantOrgMember, 0, len(rows))
	for _, row := range rows {
		code := strings.TrimSpace(row.Fields["org_code"])
		if code == "" || strings.EqualFold(strings.TrimSpace(row.Fields["member_status"]), "DISABLED") {
			continue
		}
		userID := users[row.Key].ID
		members = append(members, domain.TenantOrgMember{TenantID: tenantID, OrgUnitID: orgByCode[code].ID, UserID: userID, IsPrimary: !hasOrg[userID], Status: domain.OrgMemberStatusActive, Source: domain.OrgRelationSourceManual})
		hasOrg[userID] = true
	}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "org_unit_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"status", "source", "updated_at", "deleted_at"})}).CreateInBatches(&members, batchSize).Error
}
