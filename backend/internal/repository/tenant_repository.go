package repository

import (
	"context"
	"errors"
	"sync"
	"time"

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
	// ErrRoleCodeExists 表示同一租户内角色编码已经存在。
	ErrRoleCodeExists = errors.New("role code exists")
	// ErrPermissionNotFound 表示权限编码不存在或不可用于当前作用域。
	ErrPermissionNotFound = errors.New("permission not found")
	// ErrBuiltinRoleImmutable 表示调用方试图修改系统内置角色。
	ErrBuiltinRoleImmutable = errors.New("builtin role immutable")
	// ErrRoleDisabled 表示目标角色已禁用，不能继续分配。
	ErrRoleDisabled = errors.New("role disabled")
	// ErrInvalidRoleScope 表示角色作用域不符合当前接口规则。
	ErrInvalidRoleScope = errors.New("invalid role scope")
	// ErrCannotAssignPlatformRole 表示租户成员接口拒绝分配平台角色。
	ErrCannotAssignPlatformRole = errors.New("cannot assign platform role")
	// ErrCannotRemoveLastTenantAdmin 表示操作会导致租户失去最后一个有效管理员。
	ErrCannotRemoveLastTenantAdmin = errors.New("cannot remove last tenant admin")
)

// TenantRepository 定义租户、成员和角色授权的持久化能力。
type TenantRepository interface {
	FindTenantByID(ctx context.Context, tenantID uint64) (*domain.Tenant, error)
	FindTenantByCode(ctx context.Context, code string) (*domain.Tenant, error)
	CreateTenant(ctx context.Context, tenant *domain.Tenant) error
	UpdateTenantStatus(ctx context.Context, tenantID uint64, status domain.TenantStatus) (*domain.Tenant, error)
	ListTenants(ctx context.Context) ([]domain.Tenant, error)
	EnsureTenant(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error)
	EnsureTenants(ctx context.Context, tenants []domain.Tenant) error

	EnsureTenantUser(ctx context.Context, tenantID uint64, userID uint64, status domain.TenantUserStatus) error
	EnsureTenantUsers(ctx context.Context, members []domain.TenantUser) error
	RemoveTenantUser(ctx context.Context, tenantID uint64, userID uint64) error
	FindTenantUser(ctx context.Context, tenantID uint64, userID uint64) (*domain.TenantUser, error)
	ListTenantsByUser(ctx context.Context, userID uint64) ([]domain.Tenant, error)
	ListTenantUsers(ctx context.Context, tenantID uint64) ([]TenantMemberRecord, error)
	ListTenantUsageStats(ctx context.Context) ([]TenantUsageStats, error)
	GetTenantUsageStats(ctx context.Context, tenantID uint64) (TenantUsageStats, error)

	EnsureRole(ctx context.Context, role *domain.Role) (*domain.Role, error)
	EnsureRoles(ctx context.Context, roles []domain.Role) error
	FindRoleByCode(ctx context.Context, code domain.RoleCode) (*domain.Role, error)
	EnsureUserRole(ctx context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error
	EnsureUserRoleAssignments(ctx context.Context, assignments []domain.UserRoleAssignment) error
	RemoveUserRole(ctx context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error
	ListRoleCodesByUserTenant(ctx context.Context, userID uint64, tenantID uint64) ([]domain.RoleCode, error)
	ListPlatformRoleCodes(ctx context.Context, userID uint64) ([]domain.RoleCode, error)
	ListUserIDsByPlatformRole(ctx context.Context, roleCode domain.RoleCode) (map[uint64]struct{}, error)
	HasRole(ctx context.Context, userID uint64, tenantID *uint64, roleCode domain.RoleCode) (bool, error)
	CountTenantAdmins(ctx context.Context, tenantID uint64) (int64, error)
}

// TenantMemberRecord 是租户成员列表查询的聚合结果，包含用户展示信息和租户内角色。
type TenantMemberRecord struct {
	UserID       uint64
	Username     string
	Email        string
	Nickname     string
	Phone        string
	MemberStatus domain.TenantUserStatus
	Roles        []domain.RoleCode
	JoinedAt     time.Time
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

// EnsureTenants 批量写入缺失租户；已存在租户保持原值，避免 seed 刷新 updated_at 或覆盖人工维护字段。
func (r *GormTenantRepository) EnsureTenants(ctx context.Context, tenants []domain.Tenant) error {
	if len(tenants) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "code"}}, DoNothing: true}).Create(&tenants).Error
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

// EnsureTenantUsers 批量写入缺失租户成员；seed 不更新已有记录，避免每次启动刷新 updated_at。
func (r *GormTenantRepository) EnsureTenantUsers(ctx context.Context, members []domain.TenantUser) error {
	if len(members) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}},
		DoNothing: true,
	}).CreateInBatches(members, 200).Error
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
		Select("id, tenant_id, user_id, status, created_at").
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
		Select("id, username, email, nickname, phone").
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
			Username:     user.Username,
			Email:        user.Email,
			Nickname:     user.Nickname,
			Phone:        user.Phone,
			MemberStatus: member.Status,
			Roles:        rolesByUserID[member.UserID],
			JoinedAt:     member.CreatedAt,
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
		Joins("LEFT JOIN user_roles ON user_roles.tenant_id = tenant_users.tenant_id AND user_roles.user_id = tenant_users.user_id AND user_roles.role_id = ? AND user_roles.status = ?", tenantAdminRoleID, domain.UserRoleStatusActive).
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
		Joins("LEFT JOIN user_roles ON user_roles.tenant_id = tenant_users.tenant_id AND user_roles.user_id = tenant_users.user_id AND user_roles.role_id = ? AND user_roles.status = ?", tenantAdminRoleID, domain.UserRoleStatusActive).
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

// EnsureRoles 批量写入缺失基础角色；已存在角色不更新，避免 seed 造成 updated_at 抖动。
func (r *GormTenantRepository) EnsureRoles(ctx context.Context, roles []domain.Role) error {
	if len(roles) == 0 {
		return nil
	}
	// MySQL ON DUPLICATE DO NOTHING 命中已有记录时不会把真实 ID 回填到结构体；
	// 因此这里不写入 roleID 缓存，避免把 0 当成有效角色 ID 影响后续授权。
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "code"}}, DoNothing: true}).Create(&roles).Error
}

// FindRoleByCode 按系统内置角色编码查找角色定义，找不到时返回 ErrRoleNotFound。
func (r *GormTenantRepository) FindRoleByCode(ctx context.Context, code domain.RoleCode) (*domain.Role, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND code = ?", 0, code).First(&role).Error
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
		// 新 RBAC 使用 tenant_id=0 表示平台授权；查询仍兼容 NULL，避免迁移前后平台管理员失权。
		hasRole, err := r.HasRole(ctx, userID, nil, roleCode)
		if err != nil {
			return err
		}
		if hasRole {
			return nil
		}
		platformTenantID := uint64(0)
		tenantID = &platformTenantID
	}
	assignment := domain.UserRoleAssignment{TenantID: tenantID, UserID: userID, RoleID: roleID, AssignmentSource: domain.AssignmentSourceSystem, Status: domain.UserRoleStatusActive}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "role_id"}},
		DoUpdates: clause.Assignments(map[string]any{"status": domain.UserRoleStatusActive, "revoked_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)")}),
	}).Create(&assignment).Error
}

// EnsureUserRoleAssignments 批量写入缺失角色授权；已有授权不更新，专供 seed 降低 SQL 往返次数。
func (r *GormTenantRepository) EnsureUserRoleAssignments(ctx context.Context, assignments []domain.UserRoleAssignment) error {
	if len(assignments) == 0 {
		return nil
	}
	normalized := make([]domain.UserRoleAssignment, 0, len(assignments))
	for i := range assignments {
		assignment := assignments[i]
		if assignment.TenantID == nil {
			platformTenantID := uint64(0)
			assignment.TenantID = &platformTenantID
		}
		if assignment.AssignmentSource == "" {
			assignment.AssignmentSource = domain.AssignmentSourceSystem
		}
		if assignment.Status == "" {
			assignment.Status = domain.UserRoleStatusActive
		}
		normalized = append(normalized, assignment)
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "role_id"}},
		DoNothing: true,
	}).CreateInBatches(normalized, 200).Error
}

// RemoveUserRole 撤销用户角色授权，tenantID 为 nil 时兼容撤销 NULL 和 0 两种平台级授权。
func (r *GormTenantRepository) RemoveUserRole(ctx context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error {
	role, err := r.FindRoleByCode(ctx, roleCode)
	if err != nil {
		return err
	}
	// 角色撤销使用 Unscoped 物理删除，确保唯一索引不会被软删除记录占住导致后续无法重新授权。
	query := r.db.WithContext(ctx).Unscoped().
		Where("user_id = ? AND role_id = ?", userID, role.ID)
	if tenantID == nil {
		query = query.Where("tenant_id IS NULL OR tenant_id = 0")
	} else {
		query = query.Where("tenant_id = ?", *tenantID)
	}
	return query.Delete(&domain.UserRoleAssignment{}).Error
}

// ListRoleCodesByUserTenant 返回用户在指定租户内拥有的角色编码列表。
func (r *GormTenantRepository) ListRoleCodesByUserTenant(ctx context.Context, userID uint64, tenantID uint64) ([]domain.RoleCode, error) {
	var roles []domain.RoleCode
	err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ? AND user_roles.tenant_id = ?", userID, tenantID).
		Where("user_roles.status = ? AND (user_roles.expires_at IS NULL OR user_roles.expires_at > CURRENT_TIMESTAMP(3))", domain.UserRoleStatusActive).
		Where("roles.status = ? AND roles.scope_type = ?", domain.RoleStatusActive, domain.RoleScopeTypeTenant).
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
		Where("user_roles.user_id = ? AND (user_roles.tenant_id IS NULL OR user_roles.tenant_id = 0) AND user_roles.status = ? AND roles.status = ?", userID, domain.UserRoleStatusActive, domain.RoleStatusActive).
		Order("roles.id ASC").
		Pluck("roles.code", &roles).Error
	if err != nil {
		return nil, err
	}
	return roles, nil
}

// ListUserIDsByPlatformRole 批量返回拥有平台角色的用户 ID，供 seed 跳过平台管理员默认租户迁移。
func (r *GormTenantRepository) ListUserIDsByPlatformRole(ctx context.Context, roleCode domain.RoleCode) (map[uint64]struct{}, error) {
	roleID, err := r.roleIDByCode(ctx, roleCode)
	if errors.Is(err, ErrRoleNotFound) {
		return map[uint64]struct{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	var userIDs []uint64
	if err := r.db.WithContext(ctx).Table("user_roles").
		Where("(tenant_id IS NULL OR tenant_id = 0) AND role_id = ?", roleID).
		Pluck("user_id", &userIDs).Error; err != nil {
		return nil, err
	}
	result := make(map[uint64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		result[userID] = struct{}{}
	}
	return result, nil
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
		// nil tenantID 表示平台级角色，迁移期兼容旧 NULL 和新 0，不把租户角色提升为平台权限。
		query = query.Where("user_roles.tenant_id IS NULL OR user_roles.tenant_id = 0")
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
		Where("user_roles.tenant_id = ? AND user_roles.role_id = ? AND user_roles.status = ? AND tenant_users.status = ?", tenantID, roleID, domain.UserRoleStatusActive, domain.TenantUserStatusActive).
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
		Where("user_roles.status = ? AND roles.status = ?", domain.UserRoleStatusActive, domain.RoleStatusActive).
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
	err := r.db.WithContext(ctx).Model(&domain.Role{}).Select("id, code").Where("tenant_id = ? AND code = ?", 0, code).First(&role).Error
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
