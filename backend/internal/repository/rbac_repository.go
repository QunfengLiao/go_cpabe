package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"go-cpabe/backend/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RBACRepository 定义租户级 RBAC 的角色、权限和成员授权事实源能力。
type RBACRepository interface {
	ListTenantPermissions(ctx context.Context) ([]domain.Permission, error)
	ListTenantRoles(ctx context.Context, tenantID uint64) ([]TenantRoleRecord, error)
	FindTenantRole(ctx context.Context, tenantID uint64, roleID uint64) (*domain.Role, error)
	CreateTenantCustomRole(ctx context.Context, role domain.Role, permissionCodes []string) (*domain.Role, error)
	UpdateTenantCustomRole(ctx context.Context, tenantID uint64, roleID uint64, name string, description string, actorID uint64) (*domain.Role, error)
	DisableTenantCustomRole(ctx context.Context, tenantID uint64, roleID uint64, actorID uint64) (int64, error)
	ListRolePermissions(ctx context.Context, tenantID uint64, roleID uint64) ([]domain.Permission, error)
	ReplaceRolePermissions(ctx context.Context, tenantID uint64, roleID uint64, permissionCodes []string, actorID uint64) error
	ListMemberRoles(ctx context.Context, tenantID uint64, userID uint64) ([]domain.Role, error)
	ReplaceMemberRoles(ctx context.Context, tenantID uint64, userID uint64, roleCodes []string, assignedBy uint64) error
	ListTenantPermissionCodesByUser(ctx context.Context, tenantID uint64, userID uint64) ([]string, error)
	ListPlatformPermissionCodesByUser(ctx context.Context, userID uint64) ([]string, error)
	HasTenantPermission(ctx context.Context, tenantID uint64, userID uint64, code string) (bool, error)
	HasPlatformPermission(ctx context.Context, userID uint64, code string) (bool, error)
	CountRoleActiveMembers(ctx context.Context, tenantID uint64, roleID uint64) (int64, error)
}

// TenantRoleRecord 是角色列表聚合结果，包含权限数和有效成员数。
type TenantRoleRecord struct {
	Role              domain.Role
	PermissionCount   int64
	ActiveMemberCount int64
}

// ListTenantPermissions 返回可绑定到租户自定义角色的有效租户权限。
func (r *GormTenantRepository) ListTenantPermissions(ctx context.Context) ([]domain.Permission, error) {
	var permissions []domain.Permission
	err := r.db.WithContext(ctx).
		Where("scope_type = ? AND status = ?", domain.PermissionScopeTenant, domain.PermissionStatusActive).
		Order("resource_type ASC, action ASC, id ASC").
		Find(&permissions).Error
	return permissions, err
}

// ListTenantRoles 返回系统内置租户角色和当前租户自定义角色，平台角色不会出现在结果中。
func (r *GormTenantRepository) ListTenantRoles(ctx context.Context, tenantID uint64) ([]TenantRoleRecord, error) {
	var roles []domain.Role
	if err := r.db.WithContext(ctx).
		Where("((tenant_id = 0 AND scope_type = ?) OR (tenant_id = ? AND scope_type = ?)) AND code <> ?", domain.RoleScopeTypeTenant, tenantID, domain.RoleScopeTypeTenant, domain.RolePlatformAdmin).
		Order("is_builtin DESC, tenant_id ASC, id ASC").
		Find(&roles).Error; err != nil {
		return nil, err
	}
	records := make([]TenantRoleRecord, 0, len(roles))
	for _, role := range roles {
		permissionCount, err := r.countRolePermissions(ctx, role.ID)
		if err != nil {
			return nil, err
		}
		memberCount, err := r.CountRoleActiveMembers(ctx, tenantID, role.ID)
		if err != nil {
			return nil, err
		}
		records = append(records, TenantRoleRecord{Role: role, PermissionCount: permissionCount, ActiveMemberCount: memberCount})
	}
	return records, nil
}

// FindTenantRole 在当前租户可见范围内查找角色，避免只凭全局 role_id 修改其他租户角色。
func (r *GormTenantRepository) FindTenantRole(ctx context.Context, tenantID uint64, roleID uint64) (*domain.Role, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).
		Where("id = ? AND ((tenant_id = 0 AND scope_type = ?) OR (tenant_id = ? AND scope_type = ?)) AND code <> ?", roleID, domain.RoleScopeTypeTenant, tenantID, domain.RoleScopeTypeTenant, domain.RolePlatformAdmin).
		First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoleNotFound
	}
	return &role, err
}

// CreateTenantCustomRole 在一个事务中创建租户自定义业务角色并可选绑定权限。
func (r *GormTenantRepository) CreateTenantCustomRole(ctx context.Context, role domain.Role, permissionCodes []string) (*domain.Role, error) {
	role.TenantID = normalizeTenantID(role.TenantID)
	role.Scope = domain.RoleScopeTenant
	role.ScopeType = domain.RoleScopeTypeTenant
	role.RoleCategory = domain.RoleCategoryBusiness
	role.IsBuiltin = false
	if role.Status == "" {
		role.Status = domain.RoleStatusActive
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			if isDuplicateKey(err) {
				return ErrRoleCodeExists
			}
			return err
		}
		if len(permissionCodes) == 0 {
			return nil
		}
		return r.replaceRolePermissionsTx(ctx, tx, role.TenantID, role.ID, permissionCodes, valueOrZero(role.CreatedBy))
	})
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// UpdateTenantCustomRole 只允许修改当前租户自定义业务角色的展示信息。
func (r *GormTenantRepository) UpdateTenantCustomRole(ctx context.Context, tenantID uint64, roleID uint64, name string, description string, actorID uint64) (*domain.Role, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND tenant_id = ?", roleID, tenantID).First(&role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRoleNotFound
			}
			return err
		}
		if role.IsBuiltin || role.RoleCategory != domain.RoleCategoryBusiness || role.ScopeType != domain.RoleScopeTypeTenant {
			return ErrBuiltinRoleImmutable
		}
		updates := map[string]any{"name": name, "description": description, "updated_by": actorID, "updated_at": time.Now()}
		if err := tx.Model(&domain.Role{}).Where("id = ? AND tenant_id = ?", roleID, tenantID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&role, roleID).Error
	})
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// DisableTenantCustomRole 逻辑禁用当前租户自定义角色，并返回当前仍绑定该角色的有效成员数。
func (r *GormTenantRepository) DisableTenantCustomRole(ctx context.Context, tenantID uint64, roleID uint64, actorID uint64) (int64, error) {
	var affectedMembers int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		role, err := r.findTenantRoleForUpdateTx(ctx, tx, tenantID, roleID)
		if err != nil {
			return err
		}
		if role.IsBuiltin || role.TenantID == 0 {
			return ErrBuiltinRoleImmutable
		}
		if role.RoleCategory != domain.RoleCategoryBusiness || role.ScopeType != domain.RoleScopeTypeTenant {
			return ErrInvalidRoleScope
		}
		count, err := r.countRoleActiveMembersTx(ctx, tx, tenantID, roleID)
		if err != nil {
			return err
		}
		affectedMembers = count
		return tx.Model(&domain.Role{}).
			Where("id = ? AND tenant_id = ?", roleID, tenantID).
			Updates(map[string]any{"status": domain.RoleStatusDisabled, "updated_by": actorID, "updated_at": time.Now()}).Error
	})
	return affectedMembers, err
}

// ListRolePermissions 返回当前租户可见角色绑定的有效权限。
func (r *GormTenantRepository) ListRolePermissions(ctx context.Context, tenantID uint64, roleID uint64) ([]domain.Permission, error) {
	if _, err := r.FindTenantRole(ctx, tenantID, roleID); err != nil {
		return nil, err
	}
	var permissions []domain.Permission
	err := r.db.WithContext(ctx).
		Table("permissions").
		Select("permissions.*").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id = ? AND permissions.status = ? AND permissions.scope_type = ?", roleID, domain.PermissionStatusActive, domain.PermissionScopeTenant).
		Order("permissions.resource_type ASC, permissions.action ASC, permissions.id ASC").
		Find(&permissions).Error
	return permissions, err
}

// ReplaceRolePermissions 全量替换当前租户自定义业务角色权限，系统内置角色权限只允许 seed 管理。
func (r *GormTenantRepository) ReplaceRolePermissions(ctx context.Context, tenantID uint64, roleID uint64, permissionCodes []string, actorID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		role, err := r.findTenantRoleForUpdateTx(ctx, tx, tenantID, roleID)
		if err != nil {
			return err
		}
		if role.IsBuiltin || role.TenantID == 0 {
			return ErrBuiltinRoleImmutable
		}
		if role.RoleCategory != domain.RoleCategoryBusiness || role.ScopeType != domain.RoleScopeTypeTenant {
			return ErrInvalidRoleScope
		}
		return r.replaceRolePermissionsTx(ctx, tx, tenantID, roleID, permissionCodes, actorID)
	})
}

// ListMemberRoles 返回成员在当前租户内所有有效角色，查询前会校验成员关系有效。
func (r *GormTenantRepository) ListMemberRoles(ctx context.Context, tenantID uint64, userID uint64) ([]domain.Role, error) {
	if err := r.ensureActiveTenantMember(ctx, r.db.WithContext(ctx), tenantID, userID); err != nil {
		return nil, err
	}
	var roles []domain.Role
	err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.*").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.tenant_id = ? AND user_roles.user_id = ?", tenantID, userID).
		Where("user_roles.status = ? AND (user_roles.expires_at IS NULL OR user_roles.expires_at > CURRENT_TIMESTAMP(3))", domain.UserRoleStatusActive).
		Where("roles.status = ? AND roles.scope_type = ? AND roles.code <> ? AND (roles.tenant_id = 0 OR roles.tenant_id = ?)", domain.RoleStatusActive, domain.RoleScopeTypeTenant, domain.RolePlatformAdmin, tenantID).
		Order("roles.id ASC").
		Find(&roles).Error
	return roles, err
}

// ReplaceMemberRoles 在事务中全量替换成员角色，撤销旧角色时保留历史记录并允许 DO 和 DU 同时存在。
func (r *GormTenantRepository) ReplaceMemberRoles(ctx context.Context, tenantID uint64, userID uint64, roleCodes []string, assignedBy uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockTenantTx(ctx, tx, tenantID); err != nil {
			return err
		}
		if err := r.ensureActiveTenantMember(ctx, tx, tenantID, userID); err != nil {
			return err
		}
		roles, err := r.listAssignableRolesForUpdateTx(ctx, tx, tenantID, roleCodes)
		if err != nil {
			return err
		}
		for _, role := range roles {
			if role.ScopeType != domain.RoleScopeTypeTenant || role.Code == domain.RolePlatformAdmin {
				return ErrCannotAssignPlatformRole
			}
			if role.Status != domain.RoleStatusActive {
				return ErrRoleDisabled
			}
		}
		roleIDs := roleIDsOf(roles)
		if err := r.revokeMissingRolesTx(ctx, tx, tenantID, userID, roleIDs); err != nil {
			return err
		}
		for _, roleID := range roleIDs {
			assignment := domain.UserRoleAssignment{
				TenantID:         &tenantID,
				UserID:           userID,
				RoleID:           roleID,
				AssignmentSource: domain.AssignmentSourceManual,
				AssignedBy:       &assignedBy,
				Status:           domain.UserRoleStatusActive,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "role_id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"assignment_source": domain.AssignmentSourceManual,
					"assigned_by":       assignedBy,
					"status":            domain.UserRoleStatusActive,
					"revoked_at":        nil,
					"updated_at":        gorm.Expr("CURRENT_TIMESTAMP(3)"),
				}),
			}).Create(&assignment).Error; err != nil {
				return err
			}
		}
		count, err := r.countTenantAdminsTx(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		if count == 0 {
			return ErrCannotRemoveLastTenantAdmin
		}
		return nil
	})
}

// ListTenantPermissionCodesByUser 查询用户在指定租户内所有有效角色权限的去重并集。
func (r *GormTenantRepository) ListTenantPermissionCodesByUser(ctx context.Context, tenantID uint64, userID uint64) ([]string, error) {
	var permissions []string
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Distinct("permissions.code").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Joins("JOIN role_permissions ON role_permissions.role_id = roles.id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Joins("JOIN tenant_users ON tenant_users.tenant_id = user_roles.tenant_id AND tenant_users.user_id = user_roles.user_id").
		Where("user_roles.tenant_id = ? AND user_roles.user_id = ?", tenantID, userID).
		Where("user_roles.status = ? AND (user_roles.expires_at IS NULL OR user_roles.expires_at > CURRENT_TIMESTAMP(3))", domain.UserRoleStatusActive).
		Where("roles.status = ? AND roles.scope_type = ? AND roles.code <> ? AND (roles.tenant_id = 0 OR roles.tenant_id = ?)", domain.RoleStatusActive, domain.RoleScopeTypeTenant, domain.RolePlatformAdmin, tenantID).
		Where("permissions.status = ? AND permissions.scope_type = ?", domain.PermissionStatusActive, domain.PermissionScopeTenant).
		Where("tenant_users.status = ?", domain.TenantUserStatusActive).
		Order("permissions.code ASC").
		Pluck("permissions.code", &permissions).Error
	return permissions, err
}

// ListPlatformPermissionCodesByUser 查询用户平台作用域下所有有效权限，平台权限不参与租户业务授权。
func (r *GormTenantRepository) ListPlatformPermissionCodesByUser(ctx context.Context, userID uint64) ([]string, error) {
	var permissions []string
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Distinct("permissions.code").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Joins("JOIN role_permissions ON role_permissions.role_id = roles.id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("(user_roles.tenant_id IS NULL OR user_roles.tenant_id = 0) AND user_roles.user_id = ?", userID).
		Where("user_roles.status = ? AND (user_roles.expires_at IS NULL OR user_roles.expires_at > CURRENT_TIMESTAMP(3))", domain.UserRoleStatusActive).
		Where("roles.status = ? AND roles.scope_type = ? AND roles.tenant_id = 0", domain.RoleStatusActive, domain.RoleScopeTypePlatform).
		Where("permissions.status = ? AND permissions.scope_type = ?", domain.PermissionStatusActive, domain.PermissionScopePlatform).
		Order("permissions.code ASC").
		Pluck("permissions.code", &permissions).Error
	return permissions, err
}

// HasTenantPermission 判断用户是否拥有指定租户权限，使用数据库事实源而不是固定角色编码。
func (r *GormTenantRepository) HasTenantPermission(ctx context.Context, tenantID uint64, userID uint64, code string) (bool, error) {
	var exists int
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Select("1").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Joins("JOIN role_permissions ON role_permissions.role_id = roles.id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Joins("JOIN tenant_users ON tenant_users.tenant_id = user_roles.tenant_id AND tenant_users.user_id = user_roles.user_id").
		Where("user_roles.tenant_id = ? AND user_roles.user_id = ?", tenantID, userID).
		Where("user_roles.status = ? AND (user_roles.expires_at IS NULL OR user_roles.expires_at > CURRENT_TIMESTAMP(3))", domain.UserRoleStatusActive).
		Where("roles.status = ? AND roles.scope_type = ? AND roles.code <> ? AND (roles.tenant_id = 0 OR roles.tenant_id = ?)", domain.RoleStatusActive, domain.RoleScopeTypeTenant, domain.RolePlatformAdmin, tenantID).
		Where("permissions.code = ? AND permissions.status = ? AND permissions.scope_type = ?", code, domain.PermissionStatusActive, domain.PermissionScopeTenant).
		Where("tenant_users.status = ?", domain.TenantUserStatusActive).
		Limit(1).
		Scan(&exists).Error
	return exists == 1, err
}

// HasPlatformPermission 判断用户是否拥有指定平台权限。
func (r *GormTenantRepository) HasPlatformPermission(ctx context.Context, userID uint64, code string) (bool, error) {
	var exists int
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Select("1").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Joins("JOIN role_permissions ON role_permissions.role_id = roles.id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("(user_roles.tenant_id IS NULL OR user_roles.tenant_id = 0) AND user_roles.user_id = ?", userID).
		Where("user_roles.status = ? AND (user_roles.expires_at IS NULL OR user_roles.expires_at > CURRENT_TIMESTAMP(3))", domain.UserRoleStatusActive).
		Where("roles.status = ? AND roles.scope_type = ? AND roles.tenant_id = 0", domain.RoleStatusActive, domain.RoleScopeTypePlatform).
		Where("permissions.code = ? AND permissions.status = ? AND permissions.scope_type = ?", code, domain.PermissionStatusActive, domain.PermissionScopePlatform).
		Limit(1).
		Scan(&exists).Error
	return exists == 1, err
}

// CountRoleActiveMembers 统计当前租户内有效绑定某角色的 active 成员数。
func (r *GormTenantRepository) CountRoleActiveMembers(ctx context.Context, tenantID uint64, roleID uint64) (int64, error) {
	return r.countRoleActiveMembersTx(ctx, r.db.WithContext(ctx), tenantID, roleID)
}

// countRolePermissions 统计角色绑定的有效权限数量，用于角色列表展示。
func (r *GormTenantRepository) countRolePermissions(ctx context.Context, roleID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("role_permissions").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ? AND permissions.status = ? AND permissions.scope_type = ?", roleID, domain.PermissionStatusActive, domain.PermissionScopeTenant).
		Count(&count).Error
	return count, err
}

// replaceRolePermissionsTx 在事务中校验租户权限并全量替换角色权限绑定。
func (r *GormTenantRepository) replaceRolePermissionsTx(ctx context.Context, tx *gorm.DB, tenantID uint64, roleID uint64, permissionCodes []string, actorID uint64) error {
	codes := uniqueStrings(permissionCodes)
	if len(codes) == 0 {
		return tx.WithContext(ctx).Where("role_id = ?", roleID).Delete(&domain.RolePermission{}).Error
	}
	var permissions []domain.Permission
	if err := tx.WithContext(ctx).
		Where("code IN ? AND scope_type = ? AND status = ?", codes, domain.PermissionScopeTenant, domain.PermissionStatusActive).
		Find(&permissions).Error; err != nil {
		return err
	}
	if len(permissions) != len(codes) {
		return ErrPermissionNotFound
	}
	if err := tx.WithContext(ctx).Where("role_id = ?", roleID).Delete(&domain.RolePermission{}).Error; err != nil {
		return err
	}
	bindings := make([]domain.RolePermission, 0, len(permissions))
	for _, permission := range permissions {
		bindings = append(bindings, domain.RolePermission{RoleID: roleID, PermissionID: permission.ID, GrantedBy: &actorID})
	}
	return tx.WithContext(ctx).Create(&bindings).Error
}

// findTenantRoleForUpdateTx 在事务中锁定当前租户可修改的角色，防止并发状态变化。
func (r *GormTenantRepository) findTenantRoleForUpdateTx(ctx context.Context, tx *gorm.DB, tenantID uint64, roleID uint64) (*domain.Role, error) {
	var role domain.Role
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND ((tenant_id = 0 AND scope_type = ?) OR (tenant_id = ? AND scope_type = ?)) AND code <> ?", roleID, domain.RoleScopeTypeTenant, tenantID, domain.RoleScopeTypeTenant, domain.RolePlatformAdmin).
		First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoleNotFound
	}
	return &role, err
}

// ensureActiveTenantMember 校验成员关系有效，所有租户成员角色查询和写入都必须先过这一关。
func (r *GormTenantRepository) ensureActiveTenantMember(ctx context.Context, db *gorm.DB, tenantID uint64, userID uint64) error {
	var exists int
	if err := db.WithContext(ctx).Table("tenant_users").
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.TenantUserStatusActive).
		Select("1").Limit(1).Scan(&exists).Error; err != nil {
		return err
	}
	if exists != 1 {
		return ErrTenantMemberMissing
	}
	return nil
}

// listAssignableRolesForUpdateTx 校验待分配角色 code 都属于系统内置租户角色或当前租户自定义角色。
func (r *GormTenantRepository) listAssignableRolesForUpdateTx(ctx context.Context, tx *gorm.DB, tenantID uint64, roleCodes []string) ([]domain.Role, error) {
	uniqueCodes := uniqueRoleCodes(roleCodes)
	if len(uniqueCodes) == 0 {
		return []domain.Role{}, nil
	}
	for _, code := range uniqueCodes {
		if domain.RoleCode(code) == domain.RolePlatformAdmin {
			return nil, ErrCannotAssignPlatformRole
		}
	}
	var roles []domain.Role
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("code IN ? AND scope_type = ? AND code <> ? AND (tenant_id = 0 OR tenant_id = ?)", uniqueCodes, domain.RoleScopeTypeTenant, domain.RolePlatformAdmin, tenantID).
		Order("tenant_id ASC, id ASC").
		Find(&roles).Error; err != nil {
		return nil, err
	}
	if len(roles) != len(uniqueCodes) {
		return nil, ErrRoleNotFound
	}
	return roles, nil
}

// revokeMissingRolesTx 将不在新集合中的旧授权置为 REVOKED，保留历史而不物理删除。
func (r *GormTenantRepository) revokeMissingRolesTx(ctx context.Context, tx *gorm.DB, tenantID uint64, userID uint64, keepRoleIDs []uint64) error {
	query := tx.WithContext(ctx).Model(&domain.UserRoleAssignment{}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.UserRoleStatusActive)
	if uniqueIDs := uniqueUint64s(keepRoleIDs); len(uniqueIDs) > 0 {
		query = query.Where("role_id NOT IN ?", uniqueIDs)
	}
	return query.Updates(map[string]any{"status": domain.UserRoleStatusRevoked, "revoked_at": time.Now(), "updated_at": time.Now()}).Error
}

// lockTenantTx 锁定租户行，把同一租户内的管理员角色变更串行化。
func (r *GormTenantRepository) lockTenantTx(ctx context.Context, tx *gorm.DB, tenantID uint64) error {
	var tenant domain.Tenant
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&tenant, tenantID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTenantNotFound
	}
	return err
}

// countTenantAdminsTx 在事务内统计有效租户管理员，用于最后一名管理员保护。
func (r *GormTenantRepository) countTenantAdminsTx(ctx context.Context, tx *gorm.DB, tenantID uint64) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Joins("JOIN tenant_users ON tenant_users.tenant_id = user_roles.tenant_id AND tenant_users.user_id = user_roles.user_id").
		Where("user_roles.tenant_id = ? AND user_roles.status = ?", tenantID, domain.UserRoleStatusActive).
		Where("roles.tenant_id = 0 AND roles.code = ? AND roles.status = ?", domain.RoleTenantAdmin, domain.RoleStatusActive).
		Where("tenant_users.status = ?", domain.TenantUserStatusActive).
		Distinct("user_roles.user_id").
		Count(&count).Error
	return count, err
}

// countRoleActiveMembersTx 在给定数据库句柄上统计某角色的有效成员数。
func (r *GormTenantRepository) countRoleActiveMembersTx(ctx context.Context, db *gorm.DB, tenantID uint64, roleID uint64) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Table("user_roles").
		Joins("JOIN tenant_users ON tenant_users.tenant_id = user_roles.tenant_id AND tenant_users.user_id = user_roles.user_id").
		Where("user_roles.tenant_id = ? AND user_roles.role_id = ? AND user_roles.status = ?", tenantID, roleID, domain.UserRoleStatusActive).
		Where("(user_roles.expires_at IS NULL OR user_roles.expires_at > CURRENT_TIMESTAMP(3))").
		Where("tenant_users.status = ?", domain.TenantUserStatusActive).
		Distinct("user_roles.user_id").
		Count(&count).Error
	return count, err
}

// normalizeTenantID 防止调用方遗漏租户 ID 时创建 tenant_id=0 的自定义业务角色。
func normalizeTenantID(tenantID uint64) uint64 {
	return tenantID
}

// valueOrZero 解引用可空用户 ID，系统 seed 场景使用 0 表示无人工操作者。
func valueOrZero(value *uint64) uint64 {
	if value == nil {
		return 0
	}
	return *value
}

// uniqueStrings 对权限 code 去重，避免重复请求产生重复绑定。
func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// uniqueRoleCodes 规范化角色 code 输入，避免大小写或空白差异绕过角色存在性校验。
func uniqueRoleCodes(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := string(domain.RoleCode(normalizeRoleCodeString(value)))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

// normalizeRoleCodeString 对角色 code 使用统一大写规则；权限 code 不调用该函数。
func normalizeRoleCodeString(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// uniqueUint64s 对角色 ID 去重，保证全量替换语义稳定。
func uniqueUint64s(values []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(values))
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// roleIDsOf 提取已校验角色的 ID，并稳定去重供成员角色全量替换使用。
func roleIDsOf(roles []domain.Role) []uint64 {
	ids := make([]uint64, 0, len(roles))
	for _, role := range roles {
		ids = append(ids, role.ID)
	}
	return uniqueUint64s(ids)
}
