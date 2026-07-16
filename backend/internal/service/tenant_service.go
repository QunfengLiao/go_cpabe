package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/pkg/validator"
	"go-cpabe/backend/internal/repository"
)

const defaultTenantMemberPassword = "lqf999.."

var tenantMemberUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)

// TenantService 负责普通租户上下文、成员管理和旧角色迁移逻辑。
type TenantService struct {
	tenants       repository.TenantRepository
	users         repository.UserRepository
	authorization *AuthorizationService
	audit         AuditRecorder
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

// CreateTenantMemberInput 表示租户管理员创建或复用普通成员账号的受控输入。
type CreateTenantMemberInput struct {
	Username    string
	DisplayName string
	Email       string
	Phone       string
	Roles       []domain.RoleCode
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
	return &TenantService{tenants: tenants, users: users, audit: NoopAuditRecorder{}}
}

// SetAuthorizationService 注入统一授权服务；保留 setter 是为了避免 AuthorizationService 反向依赖 TenantService 形成循环。
func (s *TenantService) SetAuthorizationService(authorization *AuthorizationService) {
	s.authorization = authorization
}

// SetAuditRecorder 注入成员管理审计记录器；生产装配必须使用持久化实现，测试可保留空实现。
func (s *TenantService) SetAuditRecorder(audit AuditRecorder) {
	if audit == nil {
		s.audit = NoopAuditRecorder{}
		return
	}
	s.audit = audit
}

// BootstrapDefaultTenant 幂等写入基础角色、演示租户，并为非平台管理员的历史用户补齐默认租户关系。
func (s *TenantService) BootstrapDefaultTenant(ctx context.Context) error {
	// Bootstrap 必须幂等：显式 seed 或兼容迁移可重复执行，不能破坏已有租户和授权。
	if err := s.EnsureBaseRoles(ctx); err != nil {
		return err
	}
	if err := s.tenants.EnsureTenants(ctx, append([]domain.Tenant{}, demoTenants...)); err != nil {
		return err
	}
	users, err := s.users.ListAll(ctx)
	if err != nil {
		return err
	}
	defaultTenant, err := s.tenants.FindTenantByCode(ctx, domain.DefaultTenantCode)
	if err != nil {
		return err
	}
	return s.ensureUsersInDefaultTenantBatch(ctx, defaultTenant.ID, users)
}

// EnsureBaseRoles 幂等写入平台管理员、租户管理员、DO、DU 等基础角色定义。
func (s *TenantService) EnsureBaseRoles(ctx context.Context) error {
	roles := []domain.Role{
		{TenantID: 0, Code: domain.RolePlatformAdmin, Name: "平台管理员", Scope: domain.RoleScopePlatform, ScopeType: domain.RoleScopeTypePlatform, RoleCategory: domain.RoleCategoryGovernance, IsBuiltin: true, Status: domain.RoleStatusActive, Description: "预留的平台级租户管理角色"},
		{TenantID: 0, Code: domain.RoleTenantAdmin, Name: "租户管理员", Scope: domain.RoleScopeTenant, ScopeType: domain.RoleScopeTypeTenant, RoleCategory: domain.RoleCategoryGovernance, IsBuiltin: true, Status: domain.RoleStatusActive, Description: "管理当前租户内用户和资源"},
		{TenantID: 0, Code: domain.RoleDO, Name: "数据拥有者", Scope: domain.RoleScopeTenant, ScopeType: domain.RoleScopeTypeTenant, RoleCategory: domain.RoleCategoryCapability, IsBuiltin: true, Status: domain.RoleStatusActive, Description: "当前租户内上传和管理自己文件"},
		{TenantID: 0, Code: domain.RoleDU, Name: "数据使用者", Scope: domain.RoleScopeTenant, ScopeType: domain.RoleScopeTypeTenant, RoleCategory: domain.RoleCategoryCapability, IsBuiltin: true, Status: domain.RoleStatusActive, Description: "当前租户内查看文件、下载密文并尝试解密"},
	}
	return s.tenants.EnsureRoles(ctx, roles)
}

// ensureUsersInDefaultTenantBatch 批量迁移历史用户到默认租户，避免 seed 对每个用户串行查询和写入。
func (s *TenantService) ensureUsersInDefaultTenantBatch(ctx context.Context, tenantID uint64, users []domain.User) error {
	platformAdmins, err := s.tenants.ListUserIDsByPlatformRole(ctx, domain.RolePlatformAdmin)
	if err != nil {
		return err
	}
	roleByCode := map[domain.RoleCode]domain.Role{}
	for _, code := range []domain.RoleCode{domain.RoleTenantAdmin, domain.RoleDO, domain.RoleDU} {
		role, err := s.tenants.FindRoleByCode(ctx, code)
		if err != nil {
			return err
		}
		roleByCode[code] = *role
	}

	members := make([]domain.TenantUser, 0, len(users))
	assignments := make([]domain.UserRoleAssignment, 0, len(users))
	for _, user := range users {
		if _, ok := platformAdmins[user.ID]; ok {
			continue
		}
		members = append(members, domain.TenantUser{TenantID: tenantID, UserID: user.ID, Status: domain.TenantUserStatusActive})
		role := roleByCode[domain.MapLegacyUserRole(user.Role)]
		assignments = append(assignments, domain.UserRoleAssignment{TenantID: &tenantID, UserID: user.ID, RoleID: role.ID})
	}
	if err := s.tenants.EnsureTenantUsers(ctx, members); err != nil {
		return err
	}
	return s.tenants.EnsureUserRoleAssignments(ctx, assignments)
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
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return domain.TenantContextDTO{}, err
	}
	platformRoles, err := s.PlatformRolesForUser(ctx, userID)
	if err != nil {
		return domain.TenantContextDTO{}, err
	}
	if ok := hasRole(platformRoles, domain.RolePlatformAdmin); ok {
		tenants, err := s.tenants.ListTenants(ctx)
		if err != nil {
			return domain.TenantContextDTO{}, err
		}
		items := make([]domain.TenantDTO, 0, len(tenants))
		for _, tenant := range tenants {
			if tenant.Status != domain.TenantStatusEnabled {
				continue
			}
			// 平台管理员进入租户页面时保持平台身份，不伪装为租户管理员或数据角色。
			items = append(items, toTenantDTO(tenant, []domain.RoleCode{domain.RolePlatformAdmin}))
		}
		var current *uint64
		var currentCode *string
		if len(items) == 1 {
			current = &items[0].TenantID
			currentCode = &items[0].TenantCode
		}
		return s.buildTenantContextDTO(ctx, userID, current, currentCode, items, platformRoles, user), nil
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
		items = append(items, toTenantDTO(tenant, roles))
	}
	var current *uint64
	var currentCode *string
	if len(items) == 1 {
		// 只有一个可用租户时自动选中，避免前端在最常见的单租户演示场景里多做一次切换。
		current = &items[0].TenantID
		currentCode = &items[0].TenantCode
	}
	return s.buildTenantContextDTO(ctx, userID, current, currentCode, items, platformRoles, user), nil
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
	// 登录入口中的 tenantCode 只是前端选择，必须在签发 token 前校验用户确实是该租户 active 成员。
	// 平台管理员进入租户业务上下文时也不能绕过 tenant_users 和租户角色规则。
	if _, _, err := s.ResolveTenantContext(ctx, userID, tenant.ID); err != nil {
		return domain.TenantContextDTO{}, err
	}
	context, err := s.TenantContextForUser(ctx, userID)
	if err != nil {
		return domain.TenantContextDTO{}, err
	}
	context.CurrentTenantID = &tenant.ID
	context.CurrentTenantCode = &tenant.Code
	for i := range context.Tenants {
		if context.Tenants[i].TenantID != tenant.ID {
			continue
		}
		currentTenant := context.Tenants[i]
		context.CurrentTenant = &currentTenant
		context.TenantRoles = currentTenant.Roles
		context.Permissions = s.permissionsForTenantRoles(ctx, userID, tenant.ID, currentTenant.Roles)
		break
	}
	return context, nil
}

// SwitchTenant 校验用户可进入目标租户后返回新的当前租户上下文。
func (s *TenantService) SwitchTenant(ctx context.Context, userID uint64, tenantID uint64) (domain.SwitchTenantDTO, error) {
	tenant, roles, err := s.ResolveTenantContext(ctx, userID, tenantID)
	if err != nil {
		return domain.SwitchTenantDTO{}, err
	}
	tenantDTO := toTenantDTO(*tenant, roles)
	return domain.SwitchTenantDTO{
		CurrentTenantID: tenant.ID,
		CurrentTenant:   tenantDTO,
		Tenant:          tenantDTO,
		TenantRoles:     roles,
		Permissions:     s.permissionsForTenantRoles(ctx, userID, tenant.ID, roles),
		Roles:           roles,
		Menus:           []any{},
	}, nil
}

// ResolveTenantContext 校验租户存在、已启用，并要求用户是当前租户有效成员。
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

// CreateTenantMember 创建或按邮箱复用普通成员账号，并在可信租户内追加 DO/DU 角色与脱敏审计。
func (s *TenantService) CreateTenantMember(ctx context.Context, actorID uint64, tenantID uint64, input CreateTenantMemberInput) (domain.TenantMemberCreateDTO, error) {
	if err := s.ensureTenantManager(ctx, actorID, tenantID); err != nil {
		return domain.TenantMemberCreateDTO{}, err
	}
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		return domain.TenantMemberCreateDTO{}, err
	}
	if tenant.Status != domain.TenantStatusEnabled {
		return domain.TenantMemberCreateDTO{}, response.ErrTenantDisabled
	}
	normalized, roles, err := normalizeAndValidateTenantMember(input)
	if err != nil {
		return domain.TenantMemberCreateDTO{}, err
	}
	user, err := s.users.FindByEmail(ctx, normalized.Email)
	created := false
	if errors.Is(err, repository.ErrUserNotFound) {
		passwordHash, hashErr := auth.HashPassword(defaultTenantMemberPassword)
		if hashErr != nil {
			return domain.TenantMemberCreateDTO{}, hashErr
		}
		user = &domain.User{Username: normalized.Username, Email: normalized.Email, PasswordHash: passwordHash, Nickname: normalized.DisplayName, Phone: normalized.Phone, Role: domain.RoleDataUser, Status: domain.StatusActive, MustChangePassword: true}
		if createErr := s.users.Create(ctx, user); createErr != nil {
			return domain.TenantMemberCreateDTO{}, createErr
		}
		created = true
	} else if err != nil {
		return domain.TenantMemberCreateDTO{}, err
	} else if user.Status != domain.StatusActive {
		return domain.TenantMemberCreateDTO{}, response.ErrUserDisabled
	}
	// 已有账号只复用身份，绝不把表单中的密码或资料写回 users，避免租户管理员越权修改全局账号。
	if err := s.tenants.EnsureTenantUser(ctx, tenantID, user.ID, domain.TenantUserStatusActive); err != nil {
		return domain.TenantMemberCreateDTO{}, err
	}
	for _, role := range roles {
		if err := s.tenants.EnsureUserRole(ctx, &tenantID, user.ID, role); err != nil {
			return domain.TenantMemberCreateDTO{}, err
		}
	}
	member, err := s.findTenantMemberDTO(ctx, tenantID, user.ID)
	if err != nil {
		return domain.TenantMemberCreateDTO{}, err
	}
	action := "tenant_member.account_reused"
	if created {
		action = "tenant_member.account_created"
	}
	roleCodes := make([]string, 0, len(roles))
	for _, role := range roles {
		roleCodes = append(roleCodes, string(role))
	}
	// 审计元数据只传递白名单标量，角色集合使用稳定逗号分隔编码，避免数组类型绕过敏感字段审计规则。
	if err := s.audit.Record(ctx, AuditEvent{TenantID: &tenantID, ActorUserID: actorID, Action: action, TargetType: "tenant_member", TargetID: user.ID, Result: "SUCCESS", SourceTrust: "SERVER_OBSERVED", Metadata: map[string]any{"roles": strings.Join(roleCodes, ","), "created_user": created}}); err != nil {
		return domain.TenantMemberCreateDTO{}, err
	}
	result := domain.TenantMemberCreateDTO{Member: member, CreatedUser: created}
	if created {
		result.TemporaryPassword = defaultTenantMemberPassword
	}
	return result, nil
}

// normalizeAndValidateTenantMember 规范化账号字段并限制角色为 DO/DU，所有校验在任何数据库写入前完成。
func normalizeAndValidateTenantMember(input CreateTenantMemberInput) (CreateTenantMemberInput, []domain.RoleCode, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Phone = strings.TrimSpace(input.Phone)
	if !tenantMemberUsernamePattern.MatchString(input.Username) || !validator.ValidNickname(input.DisplayName) || !validator.ValidEmail(input.Email) || len([]rune(input.Phone)) > 32 {
		return input, nil, response.ErrBadRequest
	}
	seen := make(map[domain.RoleCode]struct{})
	roles := make([]domain.RoleCode, 0, len(input.Roles))
	for _, role := range input.Roles {
		if role != domain.RoleDO && role != domain.RoleDU {
			return input, nil, response.ErrInvalidRole
		}
		if _, exists := seen[role]; exists {
			continue
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	if len(roles) == 0 {
		return input, nil, response.ErrInvalidRole
	}
	return input, roles, nil
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

// ensureTenantManager 是迁移期兼容函数，只允许真实租户管理员角色管理旧租户资源接口。
func (s *TenantService) ensureTenantManager(ctx context.Context, userID uint64, tenantID uint64) error {
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

// ensurePlatformOrLegacyAdmin 校验平台管理员角色；函数名保留历史兼容，但不再读取 users.role=admin 放行。
func (s *TenantService) ensurePlatformOrLegacyAdmin(ctx context.Context, userID uint64) error {
	ok, err := s.isPlatformAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if ok {
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

// toTenantDTO 将租户实体转换为对外 DTO，并附带调用方传入的角色列表。
func toTenantDTO(tenant domain.Tenant, roles []domain.RoleCode) domain.TenantDTO {
	return domain.TenantDTO{
		TenantID:    tenant.ID,
		TenantName:  tenant.Name,
		TenantCode:  tenant.Code,
		Status:      tenant.Status,
		Description: tenant.Description,
		Branding:    tenantBrandingFor(tenant),
		Roles:       roles,
	}
}

// toTenantDTOs 批量转换租户实体列表，用于列表接口响应。
func toTenantDTOs(tenants []domain.Tenant) []domain.TenantDTO {
	result := make([]domain.TenantDTO, 0, len(tenants))
	for _, tenant := range tenants {
		result = append(result, toTenantDTO(tenant, nil))
	}
	return result
}

// buildTenantContextDTO 补齐当前租户详情、租户角色和权限，优先使用数据库权限事实源。
func (s *TenantService) buildTenantContextDTO(ctx context.Context, userID uint64, current *uint64, currentCode *string, tenants []domain.TenantDTO, platformRoles []domain.RoleCode, user *domain.User) domain.TenantContextDTO {
	var userDTO *domain.UserDTO
	if user != nil {
		dto := domain.ToUserDTO(*user, true)
		userDTO = &dto
	}
	context := domain.TenantContextDTO{CurrentTenantID: current, CurrentTenantCode: currentCode, Tenants: tenants, PlatformRoles: platformRoles, User: userDTO}
	if current == nil {
		return context
	}
	for i := range tenants {
		if tenants[i].TenantID != *current {
			continue
		}
		currentTenant := tenants[i]
		context.CurrentTenant = &currentTenant
		context.TenantRoles = currentTenant.Roles
		context.Permissions = s.permissionsForTenantRoles(ctx, userID, *current, currentTenant.Roles)
		break
	}
	return context
}

// permissionsForTenantRoles 返回当前租户权限集合；数据库授权失败时返回空集合，避免旧角色映射重新成为事实源。
func (s *TenantService) permissionsForTenantRoles(ctx context.Context, userID uint64, tenantID uint64, _ []domain.RoleCode) []string {
	if s != nil && s.authorization != nil {
		permissions, err := s.authorization.TenantPermissions(ctx, userID, tenantID)
		if err == nil {
			return permissions
		}
	}
	return []string{}
}

// tenantBrandingFor 合并数据库覆盖值和内置演示租户品牌，避免前端按中文租户名推导资源路径。
func tenantBrandingFor(tenant domain.Tenant) domain.TenantBrandingDTO {
	branding := defaultTenantBranding(strings.ToLower(strings.TrimSpace(tenant.Code)))
	overlayTenantBranding(&branding, domain.TenantBrandingDTO{
		LogoURL:                tenant.LogoURL,
		LoginBackgroundURL:     tenant.LoginBackgroundURL,
		WorkspaceBackgroundURL: tenant.WorkspaceBackgroundURL,
		PrimaryColor:           tenant.PrimaryColor,
		SidebarColor:           tenant.SidebarColor,
		BackgroundStart:        tenant.BackgroundStart,
		BackgroundEnd:          tenant.BackgroundEnd,
		BackgroundGlow:         tenant.BackgroundGlow,
	})
	return branding
}

// defaultTenantBranding 返回平台和演示租户的默认视觉配置；数据库未配置时仍能体现当前租户。
func defaultTenantBranding(code string) domain.TenantBrandingDTO {
	switch code {
	case "scnu":
		return domain.TenantBrandingDTO{
			LogoURL:                "/tenant-branding/scnu/logo.png",
			LoginBackgroundURL:     "/tenant-branding/scnu/logo.png",
			WorkspaceBackgroundURL: "/tenant-branding/scnu/logo.png",
			PrimaryColor:           "#1c5d99",
			SidebarColor:           "#1d4f91",
			BackgroundStart:        "#f7fbff",
			BackgroundEnd:          "#fffaf0",
			BackgroundGlow:         "#7db7e8",
		}
	case "sangfor":
		return domain.TenantBrandingDTO{
			LogoURL:                "/tenant-branding/sangfor/logo.png",
			LoginBackgroundURL:     "/tenant-branding/sangfor/logo.png",
			WorkspaceBackgroundURL: "/tenant-branding/sangfor/logo.png",
			PrimaryColor:           "#183b73",
			SidebarColor:           "#102a55",
			BackgroundStart:        "#f3f6fa",
			BackgroundEnd:          "#e8eef6",
			BackgroundGlow:         "#4f8edb",
		}
	case "aia", "aia-hk":
		return domain.TenantBrandingDTO{
			LogoURL:                "/tenant-branding/aia/logo.png",
			LoginBackgroundURL:     "/tenant-branding/aia/logo.png",
			WorkspaceBackgroundURL: "/tenant-branding/aia/logo.png",
			PrimaryColor:           "#d71920",
			SidebarColor:           "#b5121b",
			BackgroundStart:        "#fffafa",
			BackgroundEnd:          "#f7f8fb",
			BackgroundGlow:         "#f05a61",
		}
	default:
		return domain.TenantBrandingDTO{
			PrimaryColor:    "#1c5d99",
			SidebarColor:    "#174f86",
			BackgroundStart: "#eef3f8",
			BackgroundEnd:   "#f8fbff",
			BackgroundGlow:  "#7db7e8",
		}
	}
}

// overlayTenantBranding 将数据库中的非空品牌字段覆盖默认值，支持后续平台后台做细粒度配置。
func overlayTenantBranding(target *domain.TenantBrandingDTO, override domain.TenantBrandingDTO) {
	if override.LogoURL != "" {
		target.LogoURL = override.LogoURL
	}
	if override.LoginBackgroundURL != "" {
		target.LoginBackgroundURL = override.LoginBackgroundURL
	}
	if override.WorkspaceBackgroundURL != "" {
		target.WorkspaceBackgroundURL = override.WorkspaceBackgroundURL
	}
	if override.PrimaryColor != "" {
		target.PrimaryColor = override.PrimaryColor
	}
	if override.SidebarColor != "" {
		target.SidebarColor = override.SidebarColor
	}
	if override.BackgroundStart != "" {
		target.BackgroundStart = override.BackgroundStart
	}
	if override.BackgroundEnd != "" {
		target.BackgroundEnd = override.BackgroundEnd
	}
	if override.BackgroundGlow != "" {
		target.BackgroundGlow = override.BackgroundGlow
	}
}

// toTenantMemberDTO 将仓储层成员聚合记录转换为对外成员 DTO。
func toTenantMemberDTO(member repository.TenantMemberRecord) domain.TenantMemberDTO {
	joinedAt := member.JoinedAt
	var joinedAtPtr *time.Time
	if !joinedAt.IsZero() {
		joinedAtPtr = &joinedAt
	}
	return domain.TenantMemberDTO{
		UserID:       member.UserID,
		Username:     member.Username,
		Email:        member.Email,
		Nickname:     member.Nickname,
		Phone:        member.Phone,
		MemberStatus: member.MemberStatus,
		Roles:        member.Roles,
		JoinedAt:     joinedAtPtr,
	}
}
