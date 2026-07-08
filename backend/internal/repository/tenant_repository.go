package repository

import (
	"context"
	"errors"
	"sync"

	"go-cpabe/backend/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrTenantNotFound 表示租户不存在，Service 层会转换为对外业务错误。
	ErrTenantNotFound = errors.New("tenant not found")
	// ErrRoleNotFound 表示角色定义不存在，通常说明基础角色 seed 未完成。
	ErrRoleNotFound = errors.New("role not found")
	// ErrTenantMemberMissing 表示用户不是指定租户成员。
	ErrTenantMemberMissing = errors.New("tenant member missing")
)

// TenantRepository 定义租户、成员和角色授权的持久化能力。
type TenantRepository interface {
	FindTenantByID(ctx context.Context, tenantID uint64) (*domain.Tenant, error)
	FindTenantByCode(ctx context.Context, code string) (*domain.Tenant, error)
	CreateTenant(ctx context.Context, tenant *domain.Tenant) error
	UpdateTenantStatus(ctx context.Context, tenantID uint64, status domain.TenantStatus) (*domain.Tenant, error)
	ListTenants(ctx context.Context) ([]domain.Tenant, error)
	EnsureTenant(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error)

	EnsureTenantUser(ctx context.Context, tenantID uint64, userID uint64, status domain.TenantUserStatus) error
	RemoveTenantUser(ctx context.Context, tenantID uint64, userID uint64) error
	FindTenantUser(ctx context.Context, tenantID uint64, userID uint64) (*domain.TenantUser, error)
	ListTenantsByUser(ctx context.Context, userID uint64) ([]domain.Tenant, error)
	ListTenantUsers(ctx context.Context, tenantID uint64) ([]TenantMemberRecord, error)
	ListTenantUsageStats(ctx context.Context) ([]TenantUsageStats, error)
	GetTenantUsageStats(ctx context.Context, tenantID uint64) (TenantUsageStats, error)

	EnsureRole(ctx context.Context, role *domain.Role) (*domain.Role, error)
	FindRoleByCode(ctx context.Context, code domain.RoleCode) (*domain.Role, error)
	EnsureUserRole(ctx context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error
	RemoveUserRole(ctx context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error
	ReplaceTenantBusinessRole(ctx context.Context, tenantID uint64, userID uint64, roleCode domain.RoleCode) error
	ListRoleCodesByUserTenant(ctx context.Context, userID uint64, tenantID uint64) ([]domain.RoleCode, error)
	ListPlatformRoleCodes(ctx context.Context, userID uint64) ([]domain.RoleCode, error)
	HasRole(ctx context.Context, userID uint64, tenantID *uint64, roleCode domain.RoleCode) (bool, error)
	CountTenantAdmins(ctx context.Context, tenantID uint64) (int64, error)
}

// TenantMemberRecord 是租户成员列表查询的聚合结果，包含用户展示信息和租户内角色。
type TenantMemberRecord struct {
	UserID       uint64
	Email        string
	Nickname     string
	MemberStatus domain.TenantUserStatus
	Roles        []domain.RoleCode
}

// TenantUsageStats 是平台租户列表和 dashboard 所需的轻量统计结果。
type TenantUsageStats struct {
	TenantID         uint64
	UserCount        int64
	TenantAdminCount int64
}

// GormTenantRepository 使用 Gorm 实现租户和角色仓储。
type GormTenantRepository struct {
	db          *gorm.DB
	roleCacheMu sync.RWMutex
	roleIDs     map[domain.RoleCode]uint64
}

// NewGormTenantRepository 创建基于 Gorm 的租户仓储实例。
func NewGormTenantRepository(db *gorm.DB) *GormTenantRepository {
	return &GormTenantRepository{db: db, roleIDs: map[domain.RoleCode]uint64{}}
}

// FindTenantByID 按租户主键查找租户，找不到时返回 ErrTenantNotFound。
func (r *GormTenantRepository) FindTenantByID(ctx context.Context, tenantID uint64) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.WithContext(ctx).First(&tenant, tenantID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantNotFound
	}
	return &tenant, err
}

// FindTenantByCode 按稳定租户编码查找租户，找不到时返回 ErrTenantNotFound。
func (r *GormTenantRepository) FindTenantByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantNotFound
	}
	return &tenant, err
}

// CreateTenant 写入新租户，调用方负责校验 code 格式和唯一性错误映射。
func (r *GormTenantRepository) CreateTenant(ctx context.Context, tenant *domain.Tenant) error {
	return r.db.WithContext(ctx).Create(tenant).Error
}

// UpdateTenantStatus 更新租户启用状态，并返回更新后的租户实体。
func (r *GormTenantRepository) UpdateTenantStatus(ctx context.Context, tenantID uint64, status domain.TenantStatus) (*domain.Tenant, error) {
	if err := r.db.WithContext(ctx).Model(&domain.Tenant{}).Where("id = ?", tenantID).Update("status", status).Error; err != nil {
		return nil, err
	}
	return r.FindTenantByID(ctx, tenantID)
}

// ListTenants 按主键升序返回所有未软删除租户，用于平台列表和统计。
func (r *GormTenantRepository) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	var tenants []domain.Tenant
	if err := r.db.WithContext(ctx).
		Select("id, name, code, status, description, created_at, updated_at").
		Order("id ASC").
		Find(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

// EnsureTenant 确保租户存在；已存在时不覆盖人工维护的租户字段。
func (r *GormTenantRepository) EnsureTenant(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error) {
	// seed 和启动补偿会重复调用该方法，先查后建能保持租户元数据稳定，避免覆盖人工维护的名称和状态。
	existing, err := r.FindTenantByCode(ctx, tenant.Code)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrTenantNotFound) {
		return nil, err
	}
	if err := r.CreateTenant(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

// EnsureTenantUser 幂等写入租户成员关系，重复加入时恢复状态和软删除标记。
func (r *GormTenantRepository) EnsureTenantUser(ctx context.Context, tenantID uint64, userID uint64, status domain.TenantUserStatus) error {
	member := domain.TenantUser{TenantID: tenantID, UserID: userID, Status: status}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}},
		// 重新加入租户时同步恢复状态和 deleted_at，避免软删除记录挡住幂等写入。
		DoUpdates: clause.AssignmentColumns([]string{"status", "updated_at", "deleted_at"}),
	}).Create(&member).Error
}

// RemoveTenantUser 将成员关系置为 disabled，保留历史记录供审计和后续恢复。
func (r *GormTenantRepository) RemoveTenantUser(ctx context.Context, tenantID uint64, userID uint64) error {
	// 成员移除采用禁用状态而不是物理删除，便于后续审计和恢复历史成员关系。
	return r.db.WithContext(ctx).Model(&domain.TenantUser{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("status", domain.TenantUserStatusDisabled).Error
}

// FindTenantUser 查找指定用户在指定租户中的成员关系。
func (r *GormTenantRepository) FindTenantUser(ctx context.Context, tenantID uint64, userID uint64) (*domain.TenantUser, error) {
	var member domain.TenantUser
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantMemberMissing
	}
	return &member, err
}

// ListTenantsByUser 返回用户可进入的启用租户，只包含 active 成员关系。
func (r *GormTenantRepository) ListTenantsByUser(ctx context.Context, userID uint64) ([]domain.Tenant, error) {
	var tenants []domain.Tenant
	err := r.db.WithContext(ctx).
		Table("tenants").
		Select("tenants.id, tenants.name, tenants.code, tenants.status").
		Joins("JOIN tenant_users ON tenant_users.tenant_id = tenants.id").
		Where("tenant_users.user_id = ? AND tenant_users.status = ? AND tenants.status = ?", userID, domain.TenantUserStatusActive, domain.TenantStatusEnabled).
		Order("tenants.id ASC").
		Find(&tenants).Error
	return tenants, err
}

// ListTenantUsers 返回指定租户成员及其展示资料和租户内角色。
func (r *GormTenantRepository) ListTenantUsers(ctx context.Context, tenantID uint64) ([]TenantMemberRecord, error) {
	var members []domain.TenantUser
	if err := r.db.WithContext(ctx).
		Select("id, tenant_id, user_id, status").
		Where("tenant_id = ?", tenantID).
		Order("id ASC").
		Find(&members).Error; err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return []TenantMemberRecord{}, nil
	}
	userIDs := make([]uint64, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
	}

	var users []domain.User
	if err := r.db.WithContext(ctx).
		Select("id, email, nickname").
		Where("id IN ?", userIDs).
		Find(&users).Error; err != nil {
		return nil, err
	}
	usersByID := make(map[uint64]domain.User, len(users))
	for _, user := range users {
		usersByID[user.ID] = user
	}

	rolesByUserID, err := r.listRoleCodesByTenantUsers(ctx, tenantID, userIDs)
	if err != nil {
		return nil, err
	}

	records := make([]TenantMemberRecord, 0, len(members))
	// 成员列表以前在循环中逐个查询 users 和 roles；这里按租户一次性批量取回，避免成员数增长时 SQL 线性膨胀。
	for _, member := range members {
		user, ok := usersByID[member.UserID]
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		records = append(records, TenantMemberRecord{
			UserID:       user.ID,
			Email:        user.Email,
			Nickname:     user.Nickname,
			MemberStatus: member.Status,
			Roles:        rolesByUserID[member.UserID],
		})
	}
	return records, nil
}

// ListTenantUsageStats 批量统计各租户成员数和活跃管理员数，供列表页和 dashboard 避免拉取成员明细。
func (r *GormTenantRepository) ListTenantUsageStats(ctx context.Context) ([]TenantUsageStats, error) {
	tenantAdminRoleID, err := r.roleIDByCode(ctx, domain.RoleTenantAdmin)
	if errors.Is(err, ErrRoleNotFound) {
		tenantAdminRoleID = 0
	} else if err != nil {
		return nil, err
	}

	var rows []struct {
		TenantID         uint64
		UserCount        int64
		TenantAdminCount int64
	}
	// dashboard 只需要每个租户的成员数和活跃管理员数，使用一次条件聚合避免两次 GROUP BY 和 roles join。
	if err := r.db.WithContext(ctx).Table("tenant_users").
		Select(
			`tenant_users.tenant_id AS tenant_id,
			COUNT(*) AS user_count,
			COUNT(DISTINCT CASE WHEN tenant_users.status = ? AND user_roles.role_id = ? THEN tenant_users.user_id END) AS tenant_admin_count`,
			domain.TenantUserStatusActive,
			tenantAdminRoleID,
		).
		Joins("LEFT JOIN user_roles ON user_roles.tenant_id = tenant_users.tenant_id AND user_roles.user_id = tenant_users.user_id AND user_roles.role_id = ?", tenantAdminRoleID).
		Where("tenant_users.deleted_at IS NULL").
		Group("tenant_users.tenant_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	stats := make([]TenantUsageStats, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, TenantUsageStats{TenantID: row.TenantID, UserCount: row.UserCount, TenantAdminCount: row.TenantAdminCount})
	}
	return stats, nil
}

// GetTenantUsageStats 只统计单个租户的成员数和活跃管理员数，避免详情页复用全租户聚合。
func (r *GormTenantRepository) GetTenantUsageStats(ctx context.Context, tenantID uint64) (TenantUsageStats, error) {
	tenantAdminRoleID, err := r.roleIDByCode(ctx, domain.RoleTenantAdmin)
	if errors.Is(err, ErrRoleNotFound) {
		tenantAdminRoleID = 0
	} else if err != nil {
		return TenantUsageStats{}, err
	}

	var row struct {
		TenantID         uint64
		UserCount        int64
		TenantAdminCount int64
	}
	// 详情页只看一个租户，必须把 tenant_id 下推到 WHERE，避免扫描其他租户成员。
	err = r.db.WithContext(ctx).Table("tenant_users").
		Select(
			`tenant_users.tenant_id AS tenant_id,
			COUNT(*) AS user_count,
			COUNT(DISTINCT CASE WHEN tenant_users.status = ? AND user_roles.role_id = ? THEN tenant_users.user_id END) AS tenant_admin_count`,
			domain.TenantUserStatusActive,
			tenantAdminRoleID,
		).
		Joins("LEFT JOIN user_roles ON user_roles.tenant_id = tenant_users.tenant_id AND user_roles.user_id = tenant_users.user_id AND user_roles.role_id = ?", tenantAdminRoleID).
		Where("tenant_users.tenant_id = ? AND tenant_users.deleted_at IS NULL", tenantID).
		Group("tenant_users.tenant_id").
		Scan(&row).Error
	if err != nil {
		return TenantUsageStats{}, err
	}
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	return TenantUsageStats{TenantID: row.TenantID, UserCount: row.UserCount, TenantAdminCount: row.TenantAdminCount}, nil
}

// EnsureRole 确保基础角色定义存在；已存在时返回现有角色。
func (r *GormTenantRepository) EnsureRole(ctx context.Context, role *domain.Role) (*domain.Role, error) {
	existing, err := r.FindRoleByCode(ctx, role.Code)
	if err == nil {
		r.rememberRoleID(existing.Code, existing.ID)
		return existing, nil
	}
	if !errors.Is(err, ErrRoleNotFound) {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Create(role).Error; err != nil {
		return nil, err
	}
	r.rememberRoleID(role.Code, role.ID)
	return role, nil
}

// FindRoleByCode 按角色编码查找角色定义，找不到时返回 ErrRoleNotFound。
func (r *GormTenantRepository) FindRoleByCode(ctx context.Context, code domain.RoleCode) (*domain.Role, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoleNotFound
	}
	if err == nil {
		r.rememberRoleID(role.Code, role.ID)
	}
	return &role, err
}

// EnsureUserRole 幂等授予用户角色；tenantID 为 nil 时表示平台级角色。
func (r *GormTenantRepository) EnsureUserRole(ctx context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error {
	roleID, err := r.roleIDByCode(ctx, roleCode)
	if err != nil {
		return err
	}
	if tenantID == nil {
		// MySQL 唯一索引不会把 NULL 当作相等值；平台级角色必须先查后写，避免重复授权记录。
		hasRole, err := r.HasRole(ctx, userID, nil, roleCode)
		if err != nil {
			return err
		}
		if hasRole {
			return nil
		}
	}
	assignment := domain.UserRoleAssignment{TenantID: tenantID, UserID: userID, RoleID: roleID}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "role_id"}},
		DoNothing: true,
	}).Create(&assignment).Error
}

// RemoveUserRole 撤销用户角色授权，tenantID 为 nil 时只撤销平台级角色。
func (r *GormTenantRepository) RemoveUserRole(ctx context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error {
	role, err := r.FindRoleByCode(ctx, roleCode)
	if err != nil {
		return err
	}
	// 角色撤销使用 Unscoped 物理删除，确保唯一索引不会被软删除记录占住导致后续无法重新授权。
	query := r.db.WithContext(ctx).Unscoped().
		Where("user_id = ? AND role_id = ?", userID, role.ID)
	if tenantID == nil {
		query = query.Where("tenant_id IS NULL")
	} else {
		query = query.Where("tenant_id = ?", *tenantID)
	}
	return query.Delete(&domain.UserRoleAssignment{}).Error
}

// ReplaceTenantBusinessRole 在事务中替换用户的租户内普通业务角色，保证 DO/DU 互斥且失败时回滚旧角色。
func (r *GormTenantRepository) ReplaceTenantBusinessRole(ctx context.Context, tenantID uint64, userID uint64, roleCode domain.RoleCode) error {
	roleID, err := r.roleIDByCode(ctx, roleCode)
	if err != nil {
		return err
	}
	businessRoleIDs := make([]uint64, 0, 2)
	for _, code := range []domain.RoleCode{domain.RoleDO, domain.RoleDU} {
		id, err := r.roleIDByCode(ctx, code)
		if errors.Is(err, ErrRoleNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		businessRoleIDs = append(businessRoleIDs, id)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(businessRoleIDs) > 0 {
			// 普通业务角色本期必须单选；先删除旧 DO/DU 再写入新角色，整个过程放在事务中避免半完成状态。
			if err := tx.Unscoped().
				Where("tenant_id = ? AND user_id = ? AND role_id IN ?", tenantID, userID, businessRoleIDs).
				Delete(&domain.UserRoleAssignment{}).Error; err != nil {
				return err
			}
		}
		assignment := domain.UserRoleAssignment{TenantID: &tenantID, UserID: userID, RoleID: roleID}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "role_id"}},
			DoNothing: true,
		}).Create(&assignment).Error
	})
}

// ListRoleCodesByUserTenant 返回用户在指定租户内拥有的角色编码列表。
func (r *GormTenantRepository) ListRoleCodesByUserTenant(ctx context.Context, userID uint64, tenantID uint64) ([]domain.RoleCode, error) {
	var roles []domain.RoleCode
	err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ? AND user_roles.tenant_id = ?", userID, tenantID).
		Order("roles.id ASC").
		Pluck("roles.code", &roles).Error
	if err != nil {
		return nil, err
	}
	return roles, nil
}

// ListPlatformRoleCodes 返回用户拥有的平台级角色编码列表。
func (r *GormTenantRepository) ListPlatformRoleCodes(ctx context.Context, userID uint64) ([]domain.RoleCode, error) {
	var roles []domain.RoleCode
	err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ? AND user_roles.tenant_id IS NULL", userID).
		Order("roles.id ASC").
		Pluck("roles.code", &roles).Error
	if err != nil {
		return nil, err
	}
	return roles, nil
}

// HasRole 判断用户是否拥有指定角色；tenantID 为 nil 时只检查平台级授权。
func (r *GormTenantRepository) HasRole(ctx context.Context, userID uint64, tenantID *uint64, roleCode domain.RoleCode) (bool, error) {
	roleID, err := r.roleIDByCode(ctx, roleCode)
	if errors.Is(err, ErrRoleNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	query := r.db.WithContext(ctx).Table("user_roles").
		Where("user_roles.user_id = ? AND user_roles.role_id = ?", userID, roleID)
	if tenantID == nil {
		// nil tenantID 表示平台级角色，只允许匹配 tenant_id IS NULL，不能把租户角色提升为平台权限。
		query = query.Where("user_roles.tenant_id IS NULL")
	} else {
		query = query.Where("user_roles.tenant_id = ?", *tenantID)
	}
	var exists int
	// 权限判断只关心是否存在匹配授权，SELECT 1 LIMIT 1 比 COUNT JOIN 更容易利用索引并减少扫描行数。
	if err := query.Select("1").Limit(1).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists == 1, nil
}

// CountTenantAdmins 统计指定租户仍为 active 成员的租户管理员数量。
func (r *GormTenantRepository) CountTenantAdmins(ctx context.Context, tenantID uint64) (int64, error) {
	roleID, err := r.roleIDByCode(ctx, domain.RoleTenantAdmin)
	if errors.Is(err, ErrRoleNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var count int64
	// 只统计仍为活跃成员的租户管理员，避免已移除成员继续阻止“最后管理员”保护逻辑。
	err = r.db.WithContext(ctx).Table("user_roles").
		Joins("JOIN tenant_users ON tenant_users.tenant_id = user_roles.tenant_id AND tenant_users.user_id = user_roles.user_id").
		Where("user_roles.tenant_id = ? AND user_roles.role_id = ? AND tenant_users.status = ?", tenantID, roleID, domain.TenantUserStatusActive).
		Distinct("user_roles.user_id").
		Count(&count).Error
	return count, err
}

// listRoleCodesByTenantUsers 批量返回一组用户在同一租户内的角色编码，供成员列表消除角色查询 N+1。
func (r *GormTenantRepository) listRoleCodesByTenantUsers(ctx context.Context, tenantID uint64, userIDs []uint64) (map[uint64][]domain.RoleCode, error) {
	var rows []struct {
		UserID uint64
		Code   domain.RoleCode
	}
	if err := r.db.WithContext(ctx).
		Table("user_roles").
		Select("user_roles.user_id AS user_id, roles.code AS code").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.tenant_id = ? AND user_roles.user_id IN ?", tenantID, userIDs).
		Order("user_roles.user_id ASC, roles.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	rolesByUserID := make(map[uint64][]domain.RoleCode, len(userIDs))
	for _, userID := range userIDs {
		rolesByUserID[userID] = []domain.RoleCode{}
	}
	for _, row := range rows {
		rolesByUserID[row.UserID] = append(rolesByUserID[row.UserID], row.Code)
	}
	return rolesByUserID, nil
}

// roleIDByCode 返回稳定角色编码对应的主键，并缓存结果以避免高频权限判断反复 join roles 表。
func (r *GormTenantRepository) roleIDByCode(ctx context.Context, code domain.RoleCode) (uint64, error) {
	r.roleCacheMu.RLock()
	id, ok := r.roleIDs[code]
	r.roleCacheMu.RUnlock()
	if ok {
		return id, nil
	}
	var role domain.Role
	err := r.db.WithContext(ctx).Model(&domain.Role{}).Select("id, code").Where("code = ?", code).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrRoleNotFound
	}
	if err != nil {
		return 0, err
	}
	r.rememberRoleID(code, role.ID)
	return role.ID, nil
}

// rememberRoleID 记录角色编码到主键的映射，角色 seed 稳定后可安全复用该缓存降低权限查询成本。
func (r *GormTenantRepository) rememberRoleID(code domain.RoleCode, id uint64) {
	r.roleCacheMu.Lock()
	defer r.roleCacheMu.Unlock()
	r.roleIDs[code] = id
}
