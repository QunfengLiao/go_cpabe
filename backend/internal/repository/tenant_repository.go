package repository

import (
	"context"
	"errors"

	"go-cpabe/backend/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrTenantNotFound      = errors.New("tenant not found")
	ErrRoleNotFound        = errors.New("role not found")
	ErrTenantMemberMissing = errors.New("tenant member missing")
)

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

	EnsureRole(ctx context.Context, role *domain.Role) (*domain.Role, error)
	FindRoleByCode(ctx context.Context, code domain.RoleCode) (*domain.Role, error)
	EnsureUserRole(ctx context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error
	ListRoleCodesByUserTenant(ctx context.Context, userID uint64, tenantID uint64) ([]domain.RoleCode, error)
	HasRole(ctx context.Context, userID uint64, tenantID *uint64, roleCode domain.RoleCode) (bool, error)
}

type TenantMemberRecord struct {
	UserID       uint64
	Email        string
	Nickname     string
	MemberStatus domain.TenantUserStatus
	Roles        []domain.RoleCode
}

type GormTenantRepository struct {
	db *gorm.DB
}

func NewGormTenantRepository(db *gorm.DB) *GormTenantRepository {
	return &GormTenantRepository{db: db}
}

func (r *GormTenantRepository) FindTenantByID(ctx context.Context, tenantID uint64) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.WithContext(ctx).First(&tenant, tenantID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantNotFound
	}
	return &tenant, err
}

func (r *GormTenantRepository) FindTenantByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantNotFound
	}
	return &tenant, err
}

func (r *GormTenantRepository) CreateTenant(ctx context.Context, tenant *domain.Tenant) error {
	return r.db.WithContext(ctx).Create(tenant).Error
}

func (r *GormTenantRepository) UpdateTenantStatus(ctx context.Context, tenantID uint64, status domain.TenantStatus) (*domain.Tenant, error) {
	if err := r.db.WithContext(ctx).Model(&domain.Tenant{}).Where("id = ?", tenantID).Update("status", status).Error; err != nil {
		return nil, err
	}
	return r.FindTenantByID(ctx, tenantID)
}

func (r *GormTenantRepository) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	var tenants []domain.Tenant
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

func (r *GormTenantRepository) EnsureTenant(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error) {
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

func (r *GormTenantRepository) EnsureTenantUser(ctx context.Context, tenantID uint64, userID uint64, status domain.TenantUserStatus) error {
	member := domain.TenantUser{TenantID: tenantID, UserID: userID, Status: status}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "updated_at", "deleted_at"}),
	}).Create(&member).Error
}

func (r *GormTenantRepository) RemoveTenantUser(ctx context.Context, tenantID uint64, userID uint64) error {
	return r.db.WithContext(ctx).Model(&domain.TenantUser{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("status", domain.TenantUserStatusDisabled).Error
}

func (r *GormTenantRepository) FindTenantUser(ctx context.Context, tenantID uint64, userID uint64) (*domain.TenantUser, error) {
	var member domain.TenantUser
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantMemberMissing
	}
	return &member, err
}

func (r *GormTenantRepository) ListTenantsByUser(ctx context.Context, userID uint64) ([]domain.Tenant, error) {
	var tenants []domain.Tenant
	err := r.db.WithContext(ctx).
		Table("tenants").
		Select("tenants.*").
		Joins("JOIN tenant_users ON tenant_users.tenant_id = tenants.id").
		Where("tenant_users.user_id = ? AND tenant_users.status = ? AND tenants.status = ?", userID, domain.TenantUserStatusActive, domain.TenantStatusEnabled).
		Order("tenants.id ASC").
		Find(&tenants).Error
	return tenants, err
}

func (r *GormTenantRepository) ListTenantUsers(ctx context.Context, tenantID uint64) ([]TenantMemberRecord, error) {
	var members []domain.TenantUser
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&members).Error; err != nil {
		return nil, err
	}
	records := make([]TenantMemberRecord, 0, len(members))
	for _, member := range members {
		var user domain.User
		if err := r.db.WithContext(ctx).First(&user, member.UserID).Error; err != nil {
			return nil, err
		}
		roles, err := r.ListRoleCodesByUserTenant(ctx, member.UserID, tenantID)
		if err != nil {
			return nil, err
		}
		records = append(records, TenantMemberRecord{UserID: user.ID, Email: user.Email, Nickname: user.Nickname, MemberStatus: member.Status, Roles: roles})
	}
	return records, nil
}

func (r *GormTenantRepository) EnsureRole(ctx context.Context, role *domain.Role) (*domain.Role, error) {
	existing, err := r.FindRoleByCode(ctx, role.Code)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrRoleNotFound) {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Create(role).Error; err != nil {
		return nil, err
	}
	return role, nil
}

func (r *GormTenantRepository) FindRoleByCode(ctx context.Context, code domain.RoleCode) (*domain.Role, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoleNotFound
	}
	return &role, err
}

func (r *GormTenantRepository) EnsureUserRole(ctx context.Context, tenantID *uint64, userID uint64, roleCode domain.RoleCode) error {
	role, err := r.FindRoleByCode(ctx, roleCode)
	if err != nil {
		return err
	}
	assignment := domain.UserRoleAssignment{TenantID: tenantID, UserID: userID, RoleID: role.ID}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "role_id"}},
		DoNothing: true,
	}).Create(&assignment).Error
}

func (r *GormTenantRepository) ListRoleCodesByUserTenant(ctx context.Context, userID uint64, tenantID uint64) ([]domain.RoleCode, error) {
	var roles []domain.Role
	err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.*").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ? AND user_roles.tenant_id = ?", userID, tenantID).
		Order("roles.id ASC").
		Find(&roles).Error
	if err != nil {
		return nil, err
	}
	codes := make([]domain.RoleCode, 0, len(roles))
	for _, role := range roles {
		codes = append(codes, role.Code)
	}
	return codes, nil
}

func (r *GormTenantRepository) HasRole(ctx context.Context, userID uint64, tenantID *uint64, roleCode domain.RoleCode) (bool, error) {
	query := r.db.WithContext(ctx).Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ? AND roles.code = ?", userID, roleCode)
	if tenantID == nil {
		query = query.Where("user_roles.tenant_id IS NULL")
	} else {
		query = query.Where("user_roles.tenant_id = ?", *tenantID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
