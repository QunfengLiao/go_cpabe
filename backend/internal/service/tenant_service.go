package service

import (
	"context"
	"errors"
	"strings"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

type TenantService struct {
	tenants repository.TenantRepository
	users   repository.UserRepository
}

type CreateTenantInput struct {
	Name        string
	Code        string
	Status      domain.TenantStatus
	Description string
}

type AddTenantUserInput struct {
	UserID uint64
	Roles  []domain.RoleCode
}

func NewTenantService(tenants repository.TenantRepository, users repository.UserRepository) *TenantService {
	return &TenantService{tenants: tenants, users: users}
}

func (s *TenantService) BootstrapDefaultTenant(ctx context.Context) error {
	if err := s.EnsureBaseRoles(ctx); err != nil {
		return err
	}
	tenant, err := s.tenants.EnsureTenant(ctx, &domain.Tenant{
		Name:        "默认租户",
		Code:        domain.DefaultTenantCode,
		Status:      domain.TenantStatusEnabled,
		Description: "用于承接单租户阶段的历史用户和演示数据",
	})
	if err != nil {
		return err
	}
	users, err := s.users.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, user := range users {
		if err := s.EnsureUserInDefaultTenant(ctx, user.ID, user.Role); err != nil {
			return err
		}
	}
	_ = tenant
	return nil
}

func (s *TenantService) EnsureBaseRoles(ctx context.Context) error {
	roles := []domain.Role{
		{Code: domain.RolePlatformAdmin, Name: "平台管理员", Scope: domain.RoleScopePlatform, Description: "预留的平台级租户管理角色"},
		{Code: domain.RoleTenantAdmin, Name: "租户管理员", Scope: domain.RoleScopeTenant, Description: "管理当前租户内用户和资源"},
		{Code: domain.RoleDO, Name: "数据拥有者", Scope: domain.RoleScopeTenant, Description: "当前租户内上传和管理自己文件"},
		{Code: domain.RoleDU, Name: "数据使用者", Scope: domain.RoleScopeTenant, Description: "当前租户内查看文件、下载密文并尝试解密"},
	}
	for i := range roles {
		if _, err := s.tenants.EnsureRole(ctx, &roles[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *TenantService) EnsureUserInDefaultTenant(ctx context.Context, userID uint64, legacyRole domain.UserRole) error {
	tenant, err := s.tenants.EnsureTenant(ctx, &domain.Tenant{
		Name:        "默认租户",
		Code:        domain.DefaultTenantCode,
		Status:      domain.TenantStatusEnabled,
		Description: "用于承接单租户阶段的历史用户和演示数据",
	})
	if err != nil {
		return err
	}
	if err := s.tenants.EnsureTenantUser(ctx, tenant.ID, userID, domain.TenantUserStatusActive); err != nil {
		return err
	}
	if err := s.EnsureBaseRoles(ctx); err != nil {
		return err
	}
	role := domain.MapLegacyUserRole(legacyRole)
	// 旧单租户角色只用于迁移映射。租户内授权之后必须读取 user_roles，避免全局角色污染其他租户。
	return s.tenants.EnsureUserRole(ctx, &tenant.ID, userID, role)
}

func (s *TenantService) TenantContextForUser(ctx context.Context, userID uint64) (domain.TenantContextDTO, error) {
	tenants, err := s.tenants.ListTenantsByUser(ctx, userID)
	if err != nil {
		return domain.TenantContextDTO{}, err
	}
	items := make([]domain.TenantDTO, 0, len(tenants))
	for _, tenant := range tenants {
		roles, err := s.tenants.ListRoleCodesByUserTenant(ctx, userID, tenant.ID)
		if err != nil {
			return domain.TenantContextDTO{}, err
		}
		items = append(items, domain.TenantDTO{
			TenantID:   tenant.ID,
			TenantName: tenant.Name,
			TenantCode: tenant.Code,
			Status:     tenant.Status,
			Roles:      roles,
		})
	}
	var current *uint64
	var currentCode *string
	if len(items) == 1 {
		current = &items[0].TenantID
		currentCode = &items[0].TenantCode
	}
	return domain.TenantContextDTO{CurrentTenantID: current, CurrentTenantCode: currentCode, Tenants: items}, nil
}

func (s *TenantService) TenantContextForUserByCode(ctx context.Context, userID uint64, tenantCode string) (domain.TenantContextDTO, error) {
	code := strings.ToLower(strings.TrimSpace(tenantCode))
	if code == "" {
		return s.TenantContextForUser(ctx, userID)
	}
	tenant, err := s.tenants.FindTenantByCode(ctx, code)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return domain.TenantContextDTO{}, response.ErrTenantNotFound
		}
		return domain.TenantContextDTO{}, err
	}
	// 登录入口中的 tenantCode 只是前端选择，必须在签发 token 前校验用户确实属于该租户。
	if _, _, err := s.ResolveTenantContext(ctx, userID, tenant.ID); err != nil {
		return domain.TenantContextDTO{}, err
	}
	context, err := s.TenantContextForUser(ctx, userID)
	if err != nil {
		return domain.TenantContextDTO{}, err
	}
	context.CurrentTenantID = &tenant.ID
	context.CurrentTenantCode = &tenant.Code
	return context, nil
}

func (s *TenantService) SwitchTenant(ctx context.Context, userID uint64, tenantID uint64) (domain.SwitchTenantDTO, error) {
	tenant, roles, err := s.ResolveTenantContext(ctx, userID, tenantID)
	if err != nil {
		return domain.SwitchTenantDTO{}, err
	}
	return domain.SwitchTenantDTO{
		CurrentTenantID: tenant.ID,
		Tenant: domain.TenantDTO{
			TenantID:   tenant.ID,
			TenantName: tenant.Name,
			TenantCode: tenant.Code,
			Status:     tenant.Status,
			Roles:      roles,
		},
		Roles: roles,
		Menus: []any{},
	}, nil
}

func (s *TenantService) ResolveTenantContext(ctx context.Context, userID uint64, tenantID uint64) (*domain.Tenant, []domain.RoleCode, error) {
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return nil, nil, response.ErrTenantNotFound
		}
		return nil, nil, err
	}
	if tenant.Status != domain.TenantStatusEnabled {
		return nil, nil, response.ErrTenantDisabled
	}
	member, err := s.tenants.FindTenantUser(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantMemberMissing) {
			return nil, nil, response.ErrTenantMemberForbidden
		}
		return nil, nil, err
	}
	if member.Status != domain.TenantUserStatusActive {
		return nil, nil, response.ErrTenantMemberDisabled
	}
	roles, err := s.tenants.ListRoleCodesByUserTenant(ctx, userID, tenantID)
	if err != nil {
		return nil, nil, err
	}
	return tenant, roles, nil
}

func (s *TenantService) CreateTenant(ctx context.Context, actorID uint64, input CreateTenantInput) (domain.TenantDTO, error) {
	if err := s.ensurePlatformOrLegacyAdmin(ctx, actorID); err != nil {
		return domain.TenantDTO{}, err
	}
	name := strings.TrimSpace(input.Name)
	code := strings.ToLower(strings.TrimSpace(input.Code))
	if name == "" || code == "" {
		return domain.TenantDTO{}, response.ErrBadRequest
	}
	status := input.Status
	if status == "" {
		status = domain.TenantStatusEnabled
	}
	if !status.Valid() {
		return domain.TenantDTO{}, response.ErrBadRequest
	}
	if _, err := s.tenants.FindTenantByCode(ctx, code); err == nil {
		return domain.TenantDTO{}, response.ErrTenantCodeExists
	} else if !errors.Is(err, repository.ErrTenantNotFound) {
		return domain.TenantDTO{}, err
	}
	tenant := &domain.Tenant{Name: name, Code: code, Status: status, Description: strings.TrimSpace(input.Description)}
	if err := s.tenants.CreateTenant(ctx, tenant); err != nil {
		return domain.TenantDTO{}, err
	}
	return toTenantDTO(*tenant, nil), nil
}

func (s *TenantService) ListTenants(ctx context.Context, actorID uint64) ([]domain.TenantDTO, error) {
	if err := s.ensurePlatformOrLegacyAdmin(ctx, actorID); err == nil {
		tenants, err := s.tenants.ListTenants(ctx)
		if err != nil {
			return nil, err
		}
		return toTenantDTOs(tenants), nil
	}
	context, err := s.TenantContextForUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return context.Tenants, nil
}

func (s *TenantService) TenantDetail(ctx context.Context, actorID uint64, tenantID uint64) (domain.TenantDTO, error) {
	if err := s.ensureTenantManager(ctx, actorID, tenantID); err != nil {
		return domain.TenantDTO{}, err
	}
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return domain.TenantDTO{}, response.ErrTenantNotFound
		}
		return domain.TenantDTO{}, err
	}
	return toTenantDTO(*tenant, nil), nil
}

func (s *TenantService) SetTenantStatus(ctx context.Context, actorID uint64, tenantID uint64, status domain.TenantStatus) (domain.TenantDTO, error) {
	if err := s.ensurePlatformOrLegacyAdmin(ctx, actorID); err != nil {
		return domain.TenantDTO{}, err
	}
	tenant, err := s.tenants.UpdateTenantStatus(ctx, tenantID, status)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return domain.TenantDTO{}, response.ErrTenantNotFound
		}
		return domain.TenantDTO{}, err
	}
	return toTenantDTO(*tenant, nil), nil
}

func (s *TenantService) AddTenantUser(ctx context.Context, actorID uint64, tenantID uint64, input AddTenantUserInput) (domain.TenantMemberDTO, error) {
	if err := s.ensureTenantManager(ctx, actorID, tenantID); err != nil {
		return domain.TenantMemberDTO{}, err
	}
	if _, err := s.users.FindByID(ctx, input.UserID); err != nil {
		return domain.TenantMemberDTO{}, response.ErrBadRequest
	}
	if err := s.tenants.EnsureTenantUser(ctx, tenantID, input.UserID, domain.TenantUserStatusActive); err != nil {
		return domain.TenantMemberDTO{}, err
	}
	for _, role := range input.Roles {
		if !role.Valid() || !role.TenantScoped() {
			return domain.TenantMemberDTO{}, response.ErrInvalidRole
		}
		if err := s.tenants.EnsureUserRole(ctx, &tenantID, input.UserID, role); err != nil {
			return domain.TenantMemberDTO{}, err
		}
	}
	members, err := s.tenants.ListTenantUsers(ctx, tenantID)
	if err != nil {
		return domain.TenantMemberDTO{}, err
	}
	for _, member := range members {
		if member.UserID == input.UserID {
			return toTenantMemberDTO(member), nil
		}
	}
	return domain.TenantMemberDTO{}, response.ErrBadRequest
}

func (s *TenantService) RemoveTenantUser(ctx context.Context, actorID uint64, tenantID uint64, userID uint64) error {
	if err := s.ensureTenantManager(ctx, actorID, tenantID); err != nil {
		return err
	}
	return s.tenants.RemoveTenantUser(ctx, tenantID, userID)
}

func (s *TenantService) ListTenantUsers(ctx context.Context, actorID uint64, tenantID uint64) ([]domain.TenantMemberDTO, error) {
	if err := s.ensureTenantManager(ctx, actorID, tenantID); err != nil {
		return nil, err
	}
	members, err := s.tenants.ListTenantUsers(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.TenantMemberDTO, 0, len(members))
	for _, member := range members {
		result = append(result, toTenantMemberDTO(member))
	}
	return result, nil
}

func (s *TenantService) ensureTenantManager(ctx context.Context, userID uint64, tenantID uint64) error {
	if err := s.ensurePlatformOrLegacyAdmin(ctx, userID); err == nil {
		return nil
	}
	ok, err := s.tenants.HasRole(ctx, userID, &tenantID, domain.RoleTenantAdmin)
	if err != nil {
		return err
	}
	if !ok {
		return response.ErrTenantPermissionDenied
	}
	return nil
}

func (s *TenantService) ensurePlatformOrLegacyAdmin(ctx context.Context, userID uint64) error {
	ok, err := s.tenants.HasRole(ctx, userID, nil, domain.RolePlatformAdmin)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return response.ErrTenantPermissionDenied
	}
	if user.Role == domain.RoleAdmin {
		return nil
	}
	return response.ErrTenantPermissionDenied
}

func toTenantDTO(tenant domain.Tenant, roles []domain.RoleCode) domain.TenantDTO {
	return domain.TenantDTO{TenantID: tenant.ID, TenantName: tenant.Name, TenantCode: tenant.Code, Status: tenant.Status, Roles: roles}
}

func toTenantDTOs(tenants []domain.Tenant) []domain.TenantDTO {
	result := make([]domain.TenantDTO, 0, len(tenants))
	for _, tenant := range tenants {
		result = append(result, toTenantDTO(tenant, nil))
	}
	return result
}

func toTenantMemberDTO(member repository.TenantMemberRecord) domain.TenantMemberDTO {
	return domain.TenantMemberDTO{
		UserID:       member.UserID,
		Email:        member.Email,
		Nickname:     member.Nickname,
		MemberStatus: member.MemberStatus,
		Roles:        member.Roles,
	}
}
