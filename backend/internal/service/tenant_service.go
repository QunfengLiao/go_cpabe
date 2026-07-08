package service

import (
	"context"
	"errors"
	"strings"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// TenantService 负责普通租户上下文、成员管理和旧角色迁移逻辑。
type TenantService struct {
	tenants repository.TenantRepository
	users   repository.UserRepository
}

// CreateTenantInput 表示创建租户时允许提交的业务字段。
type CreateTenantInput struct {
	Name        string
	Code        string
	Status      domain.TenantStatus
	Description string
}

// AddTenantUserInput 表示向租户添加用户时的目标用户和待授予租户角色。
type AddTenantUserInput struct {
	UserID uint64
	Roles  []domain.RoleCode
}

// AssignTenantMemberRoleInput 表示租户管理员为成员保存普通业务角色的输入。
type AssignTenantMemberRoleInput struct {
	RoleCode string
}

// demoTenants 是开发演示环境默认写入的租户集合。
var demoTenants = []domain.Tenant{
	{Name: "四川师范大学", Code: "scnu", Status: domain.TenantStatusEnabled, Description: "科研数据安全共享演示租户"},
	{Name: "深信服科技", Code: "sangfor", Status: domain.TenantStatusEnabled, Description: "企业安全协作演示租户"},
	{Name: "香港友邦保险", Code: "aia-hk", Status: domain.TenantStatusEnabled, Description: "保险数据协作演示租户"},
	{Name: "默认租户", Code: domain.DefaultTenantCode, Status: domain.TenantStatusEnabled, Description: "用于承接单租户阶段的历史用户和演示数据"},
}

// NewTenantService 创建租户服务，依赖租户仓储和用户仓储完成权限校验。
func NewTenantService(tenants repository.TenantRepository, users repository.UserRepository) *TenantService {
	return &TenantService{tenants: tenants, users: users}
}

// BootstrapDefaultTenant 幂等写入基础角色、演示租户，并为非平台管理员的历史用户补齐默认租户关系。
func (s *TenantService) BootstrapDefaultTenant(ctx context.Context) error {
	// Bootstrap 必须幂等：开发环境会在每次启动时执行，不能因为重复 seed 破坏已有租户和授权。
	if err := s.EnsureBaseRoles(ctx); err != nil {
		return err
	}
	for i := range demoTenants {
		tenant := demoTenants[i]
		if _, err := s.tenants.EnsureTenant(ctx, &tenant); err != nil {
			return err
		}
	}
	users, err := s.users.ListAll(ctx)
	if err != nil {
		return err
	}
	// 历史用户没有 tenant_users/user_roles 记录时无法进入多租户上下文，
	// 因此启动时做一次兼容性补齐；平台管理员是全局运维身份，不能被这个迁移步骤自动变成租户成员。
	for _, user := range users {
		if err := s.EnsureUserInDefaultTenant(ctx, user.ID, user.Role); err != nil {
			return err
		}
	}
	return nil
}

// EnsureBaseRoles 幂等写入平台管理员、租户管理员、DO、DU 等基础角色定义。
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

// EnsureUserInDefaultTenant 将普通历史用户加入默认租户，并把旧单租户角色映射为租户内角色。
func (s *TenantService) EnsureUserInDefaultTenant(ctx context.Context, userID uint64, legacyRole domain.UserRole) error {
	if ok, err := s.isPlatformAdmin(ctx, userID); err != nil {
		return err
	} else if ok {
		// PLATFORM_ADMIN 只代表平台后台管理权限。跳过默认租户迁移，避免启动或登录补齐时把平台管理员写入 tenant_users，
		// 也避免把旧 users.role=admin 误映射为默认租户的 TENANT_ADMIN。
		return nil
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

// TenantContextForUser 返回用户可进入的租户列表，只有一个租户时自动设置当前租户。
func (s *TenantService) TenantContextForUser(ctx context.Context, userID uint64) (domain.TenantContextDTO, error) {
	if ok, err := s.isPlatformAdmin(ctx, userID); err != nil {
		return domain.TenantContextDTO{}, err
	} else if ok {
		tenants, err := s.tenants.ListTenants(ctx)
		if err != nil {
			return domain.TenantContextDTO{}, err
		}
		items := make([]domain.TenantDTO, 0, len(tenants))
		for _, tenant := range tenants {
			if tenant.Status != domain.TenantStatusEnabled {
				continue
			}
			items = append(items, domain.TenantDTO{
				TenantID:   tenant.ID,
				TenantName: tenant.Name,
				TenantCode: tenant.Code,
				Status:     tenant.Status,
				// 平台管理员进入租户页面时保持平台身份，不伪装为租户管理员或数据角色。
				Roles: []domain.RoleCode{domain.RolePlatformAdmin},
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
		// 只有一个可用租户时自动选中，避免前端在最常见的单租户演示场景里多做一次切换。
		current = &items[0].TenantID
		currentCode = &items[0].TenantCode
	}
	return domain.TenantContextDTO{CurrentTenantID: current, CurrentTenantCode: currentCode, Tenants: items}, nil
}

// PlatformRolesForUser 返回用户拥有的平台级角色，仅用于前端展示和菜单初始化。
func (s *TenantService) PlatformRolesForUser(ctx context.Context, userID uint64) ([]domain.RoleCode, error) {
	return s.tenants.ListPlatformRoleCodes(ctx, userID)
}

// TenantContextForUserByCode 校验登录时选择的租户编码，并返回带当前租户的上下文。
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
	// 登录入口中的 tenantCode 只是前端选择，必须在签发 token 前校验用户确实有权进入该租户。
	// PLATFORM_ADMIN 走平台身份例外路径；普通用户仍必须是 tenant_users 中的 active 成员。
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

// SwitchTenant 校验用户可进入目标租户后返回新的当前租户上下文。
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

// ResolveTenantContext 校验租户存在、已启用，并按平台管理员或普通成员路径返回当前身份。
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
	if ok, err := s.isPlatformAdmin(ctx, userID); err != nil {
		return nil, nil, err
	} else if ok {
		// PLATFORM_ADMIN 是平台级运维身份，可以进入任意 active 租户页面做管理，
		// 但它不是租户成员，不能在这里写 tenant_users，也不能授予 DO/DU 等文件解密业务角色。
		return tenant, []domain.RoleCode{domain.RolePlatformAdmin}, nil
	}
	// 租户上下文不能只信任 token 或请求头；每次切换/访问都重新校验成员关系和租户状态。
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

// CreateTenant 为有平台或旧管理员权限的用户创建租户。
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

// ListTenants 按调用者权限返回所有租户或调用者所属租户。
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

// TenantDetail 校验调用者管理权限后返回指定租户详情。
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

// SetTenantStatus 校验平台或旧管理员权限后更新租户启用状态。
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

// AddTenantUser 校验租户管理权限后添加成员并授予租户内角色。
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
			// 租户成员接口只能授予租户内角色，平台角色必须走受控的平台授权路径。
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

// RemoveTenantUser 校验租户管理权限后停用指定成员关系。
func (s *TenantService) RemoveTenantUser(ctx context.Context, actorID uint64, tenantID uint64, userID uint64) error {
	if err := s.ensureTenantManager(ctx, actorID, tenantID); err != nil {
		return err
	}
	return s.tenants.RemoveTenantUser(ctx, tenantID, userID)
}

// ListTenantUsers 校验租户管理权限后返回租户成员列表。
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

// AssignTenantMemberBusinessRole 校验租户管理员身份后，为本租户成员替换单一普通业务角色。
func (s *TenantService) AssignTenantMemberBusinessRole(ctx context.Context, actorID uint64, tenantID uint64, targetUserID uint64, input AssignTenantMemberRoleInput) (domain.TenantMemberDTO, error) {
	role, err := tenantBusinessRoleFromRequest(input.RoleCode)
	if err != nil {
		return domain.TenantMemberDTO{}, err
	}
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return domain.TenantMemberDTO{}, response.ErrTenantNotFound
		}
		return domain.TenantMemberDTO{}, err
	}
	if tenant.Status != domain.TenantStatusEnabled {
		return domain.TenantMemberDTO{}, response.ErrTenantDisabled
	}
	if ok, err := s.isPlatformAdmin(ctx, actorID); err != nil {
		return domain.TenantMemberDTO{}, err
	} else if ok {
		// PLATFORM_ADMIN 负责平台治理和兜底指定租户管理员，不参与租户内 DO/DU 日常分配。
		// 这里显式拒绝，避免平台身份绕过 tenant_users 成员关系并获得租户业务角色写入能力。
		return domain.TenantMemberDTO{}, response.ErrTenantRoleAssignPlatformForbidden
	}
	if ok, err := s.tenants.HasRole(ctx, actorID, &tenantID, domain.RoleTenantAdmin); err != nil {
		return domain.TenantMemberDTO{}, err
	} else if !ok {
		return domain.TenantMemberDTO{}, response.ErrTenantPermissionDenied
	}
	if actorID == targetUserID {
		// 只有确认操作者确实是本租户管理员后，才返回“不能修改自己的管理员角色”；
		// 普通成员直接调用接口仍应按无权限处理，避免暴露多余的角色语义。
		return domain.TenantMemberDTO{}, response.ErrTenantAdminSelfRoleForbidden
	}
	member, err := s.tenants.FindTenantUser(ctx, tenantID, targetUserID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantMemberMissing) {
			return domain.TenantMemberDTO{}, response.ErrTenantMemberForbidden
		}
		return domain.TenantMemberDTO{}, err
	}
	if member.Status != domain.TenantUserStatusActive {
		return domain.TenantMemberDTO{}, response.ErrTenantMemberDisabled
	}
	if err := s.tenants.ReplaceTenantBusinessRole(ctx, tenantID, targetUserID, role); err != nil {
		return domain.TenantMemberDTO{}, err
	}
	return s.findTenantMemberDTO(ctx, tenantID, targetUserID)
}

// ensureTenantManager 校验用户是否具备指定租户的管理权限。
func (s *TenantService) ensureTenantManager(ctx context.Context, userID uint64, tenantID uint64) error {
	if err := s.ensurePlatformOrLegacyAdmin(ctx, userID); err == nil {
		return nil
	}
	// 租户管理员权限必须绑定到具体 tenant_id，避免 A 租户管理员跨租户管理 B 租户。
	ok, err := s.tenants.HasRole(ctx, userID, &tenantID, domain.RoleTenantAdmin)
	if err != nil {
		return err
	}
	if !ok {
		return response.ErrTenantPermissionDenied
	}
	return nil
}

// ensurePlatformOrLegacyAdmin 校验平台管理员角色，并兼容旧 users.role 中的 admin。
func (s *TenantService) ensurePlatformOrLegacyAdmin(ctx context.Context, userID uint64) error {
	ok, err := s.isPlatformAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	// 兼容旧数据中的 admin 字段；新授权应优先写入 user_roles 的平台角色。
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return response.ErrTenantPermissionDenied
	}
	if user.Role == domain.RoleAdmin {
		return nil
	}
	return response.ErrTenantPermissionDenied
}

// isPlatformAdmin 只检查 user_roles 中 tenant_id IS NULL 的平台管理员授权，不把旧 admin 字段当作租户上下文绕过依据。
func (s *TenantService) isPlatformAdmin(ctx context.Context, userID uint64) (bool, error) {
	return s.tenants.HasRole(ctx, userID, nil, domain.RolePlatformAdmin)
}

// findTenantMemberDTO 从最新成员列表中定位目标成员，确保角色分配响应和列表回显使用同一聚合口径。
func (s *TenantService) findTenantMemberDTO(ctx context.Context, tenantID uint64, userID uint64) (domain.TenantMemberDTO, error) {
	members, err := s.tenants.ListTenantUsers(ctx, tenantID)
	if err != nil {
		return domain.TenantMemberDTO{}, err
	}
	for _, member := range members {
		if member.UserID == userID {
			return toTenantMemberDTO(member), nil
		}
	}
	return domain.TenantMemberDTO{}, response.ErrTenantMemberForbidden
}

// tenantBusinessRoleFromRequest 将前端业务角色名映射为系统内部稳定角色编码，并拒绝平台或管理员角色。
func tenantBusinessRoleFromRequest(raw string) (domain.RoleCode, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DATA_OWNER", string(domain.RoleDO):
		return domain.RoleDO, nil
	case "DATA_VISITOR", string(domain.RoleDU):
		return domain.RoleDU, nil
	default:
		return "", response.ErrInvalidRole
	}
}

// toTenantDTO 将租户实体转换为对外 DTO，并附带调用方传入的角色列表。
func toTenantDTO(tenant domain.Tenant, roles []domain.RoleCode) domain.TenantDTO {
	return domain.TenantDTO{TenantID: tenant.ID, TenantName: tenant.Name, TenantCode: tenant.Code, Status: tenant.Status, Roles: roles}
}

// toTenantDTOs 批量转换租户实体列表，用于列表接口响应。
func toTenantDTOs(tenants []domain.Tenant) []domain.TenantDTO {
	result := make([]domain.TenantDTO, 0, len(tenants))
	for _, tenant := range tenants {
		result = append(result, toTenantDTO(tenant, nil))
	}
	return result
}

// toTenantMemberDTO 将仓储层成员聚合记录转换为对外成员 DTO。
func toTenantMemberDTO(member repository.TenantMemberRecord) domain.TenantMemberDTO {
	return domain.TenantMemberDTO{
		UserID:       member.UserID,
		Email:        member.Email,
		Nickname:     member.Nickname,
		MemberStatus: member.MemberStatus,
		Roles:        member.Roles,
	}
}
