package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/pkg/validator"
	"go-cpabe/backend/internal/repository"
)

// tenantCodePattern 约束租户编码只包含小写字母、数字和短横线，便于 URL 和缓存使用。
var tenantCodePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// tenantAdminUsernamePattern 约束后台账号名只使用稳定 ASCII 字符，避免后续接入用户名登录时出现歧义。
var tenantAdminUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)

// defaultTenantAdminPassword 是平台代建租户管理员时的固定初始密码，仅用于创建时哈希入库并强制首次改密。
const defaultTenantAdminPassword = "lqf999.."

// CreateTenantAdminAccountInput 表示平台后台创建或复用租户管理员账号所需的输入。
type CreateTenantAdminAccountInput struct {
	UserID      uint64
	Username    string
	DisplayName string
	Email       string
	Phone       string
	Password    string
}

// PlatformTenantService 负责平台后台的租户列表、创建、详情和状态管理。
type PlatformTenantService struct {
	tenants repository.TenantRepository
	users   repository.UserRepository
	audit   AuditRecorder
}

// PlatformTenantUserService 负责平台后台管理租户成员关系。
type PlatformTenantUserService struct {
	tenants repository.TenantRepository
	users   repository.UserRepository
	audit   AuditRecorder
}

// PlatformRoleService 负责平台后台授予或撤销平台/租户管理员角色。
type PlatformRoleService struct {
	tenants repository.TenantRepository
	users   repository.UserRepository
	audit   AuditRecorder
}

// PlatformDashboardService 负责平台后台首页统计数据聚合。
type PlatformDashboardService struct {
	tenants repository.TenantRepository
	users   repository.UserRepository
}

// NewPlatformTenantService 创建平台租户服务，并在审计记录器为空时使用空实现。
func NewPlatformTenantService(tenants repository.TenantRepository, users repository.UserRepository, audit AuditRecorder) *PlatformTenantService {
	return &PlatformTenantService{tenants: tenants, users: users, audit: normalizeAuditRecorder(audit)}
}

// NewPlatformTenantUserService 创建平台租户成员服务。
func NewPlatformTenantUserService(tenants repository.TenantRepository, users repository.UserRepository, audit AuditRecorder) *PlatformTenantUserService {
	return &PlatformTenantUserService{tenants: tenants, users: users, audit: normalizeAuditRecorder(audit)}
}

// NewPlatformRoleService 创建平台角色服务。
func NewPlatformRoleService(tenants repository.TenantRepository, users repository.UserRepository, audit AuditRecorder) *PlatformRoleService {
	return &PlatformRoleService{tenants: tenants, users: users, audit: normalizeAuditRecorder(audit)}
}

// NewPlatformDashboardService 创建平台首页统计服务。
func NewPlatformDashboardService(tenants repository.TenantRepository, users repository.UserRepository) *PlatformDashboardService {
	return &PlatformDashboardService{tenants: tenants, users: users}
}

// normalizeAuditRecorder 返回可安全调用的审计记录器，避免审计模块未实现时阻塞主链路。
func normalizeAuditRecorder(audit AuditRecorder) AuditRecorder {
	if audit == nil {
		// 审计模块尚未落库时使用空实现，让平台管理主链路先闭环，同时保留后续替换点。
		return NoopAuditRecorder{}
	}
	return audit
}

// ListTenants 返回平台租户列表，并为每个租户补充成员数量和管理员数量。
func (s *PlatformTenantService) ListTenants(ctx context.Context) ([]domain.TenantDTO, error) {
	tenants, err := s.tenants.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	stats, err := s.tenants.ListTenantUsageStats(ctx)
	if err != nil {
		return nil, err
	}
	statsByTenantID := tenantStatsByID(stats)
	result := make([]domain.TenantDTO, 0, len(tenants))
	for _, tenant := range tenants {
		dto := platformTenantDTO(tenant)
		// 列表页只展示成员数量和管理员数量，不需要加载每个成员的用户资料和角色明细。
		if stat, ok := statsByTenantID[tenant.ID]; ok {
			dto.UserCount = stat.UserCount
			dto.TenantAdminCount = stat.TenantAdminCount
		}
		result = append(result, dto)
	}
	return result, nil
}

// CreateTenant 校验租户编码并创建新租户，成功后记录平台审计事件。
func (s *PlatformTenantService) CreateTenant(ctx context.Context, actorID uint64, input CreateTenantInput) (domain.TenantDTO, error) {
	name := strings.TrimSpace(input.Name)
	code := strings.ToLower(strings.TrimSpace(input.Code))
	if name == "" || code == "" {
		return domain.TenantDTO{}, response.ErrBadRequest
	}
	if !tenantCodePattern.MatchString(code) {
		// 租户 code 会出现在 URL、筛选条件和前端缓存中，只允许小写字母数字与短横线降低歧义。
		return domain.TenantDTO{}, response.ErrTenantCodeInvalid
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
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: "tenant.created", TargetType: "tenant", TargetID: tenant.ID}); err != nil {
		return domain.TenantDTO{}, err
	}
	return platformTenantDTO(*tenant), nil
}

// TenantDetail 返回租户详情，并聚合成员数量和租户管理员数量。
func (s *PlatformTenantService) TenantDetail(ctx context.Context, tenantID uint64) (domain.TenantDTO, error) {
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return domain.TenantDTO{}, response.ErrTenantNotFound
		}
		return domain.TenantDTO{}, err
	}
	dto := platformTenantDTO(*tenant)
	stat, err := s.tenants.GetTenantUsageStats(ctx, tenantID)
	if err != nil {
		return domain.TenantDTO{}, err
	}
	dto.UserCount = stat.UserCount
	dto.TenantAdminCount = stat.TenantAdminCount
	return dto, nil
}

// SetTenantStatus 更新租户启用状态，并记录启用或禁用审计事件。
func (s *PlatformTenantService) SetTenantStatus(ctx context.Context, actorID uint64, tenantID uint64, status domain.TenantStatus) (domain.TenantDTO, error) {
	if !status.Valid() {
		return domain.TenantDTO{}, response.ErrBadRequest
	}
	tenant, err := s.tenants.UpdateTenantStatus(ctx, tenantID, status)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return domain.TenantDTO{}, response.ErrTenantNotFound
		}
		return domain.TenantDTO{}, err
	}
	action := "tenant.enabled"
	if status == domain.TenantStatusDisabled {
		action = "tenant.disabled"
	}
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: action, TargetType: "tenant", TargetID: tenantID}); err != nil {
		return domain.TenantDTO{}, err
	}
	return platformTenantDTO(*tenant), nil
}

// ListTenantUsers 返回指定租户的成员列表，租户不存在时返回业务错误。
func (s *PlatformTenantUserService) ListTenantUsers(ctx context.Context, tenantID uint64) ([]domain.TenantMemberDTO, error) {
	if _, err := s.findTenant(ctx, tenantID); err != nil {
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

// SearchUsers 为平台成员接入页面搜索已有账号，只返回非敏感展示字段，供人工选择后再加入租户。
func (s *PlatformTenantUserService) SearchUsers(ctx context.Context, query string) ([]domain.UserDTO, error) {
	users, err := s.users.SearchUsers(ctx, strings.TrimSpace(query), 20)
	if err != nil {
		return nil, err
	}
	result := make([]domain.UserDTO, 0, len(users))
	for _, user := range users {
		result = append(result, domain.ToUserDTO(user, true))
	}
	return result, nil
}

// AddTenantUser 将已存在用户加入启用租户，并记录成员新增审计事件。
func (s *PlatformTenantUserService) AddTenantUser(ctx context.Context, actorID uint64, tenantID uint64, userID uint64) (domain.TenantMemberDTO, error) {
	tenant, err := s.findTenant(ctx, tenantID)
	if err != nil {
		return domain.TenantMemberDTO{}, err
	}
	if tenant.Status != domain.TenantStatusEnabled {
		// 禁用租户不允许新增成员，避免被关闭的组织继续扩大访问范围。
		return domain.TenantMemberDTO{}, response.ErrTenantDisabled
	}
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return domain.TenantMemberDTO{}, response.ErrBadRequest
	}
	if err := s.tenants.EnsureTenantUser(ctx, tenantID, userID, domain.TenantUserStatusActive); err != nil {
		return domain.TenantMemberDTO{}, err
	}
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: "tenant_user.added", TargetType: "tenant_user", TargetID: userID, Metadata: map[string]any{"tenant_id": tenantID}}); err != nil {
		return domain.TenantMemberDTO{}, err
	}
	return s.findTenantMember(ctx, tenantID, userID)
}

// RemoveTenantUser 停用租户成员关系，目标为租户管理员时会保护最后管理员。
func (s *PlatformTenantUserService) RemoveTenantUser(ctx context.Context, actorID uint64, tenantID uint64, userID uint64) error {
	if _, err := s.findTenant(ctx, tenantID); err != nil {
		return err
	}
	member, err := s.findTenantMember(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if hasRoleCode(member.Roles, domain.RoleTenantAdmin) {
		// 删除租户管理员前先确认不是最后一位管理员，避免租户进入无人可治理状态。
		if err := ensureNotLastTenantAdmin(ctx, s.tenants, tenantID); err != nil {
			return err
		}
	}
	if err := s.tenants.RemoveTenantUser(ctx, tenantID, userID); err != nil {
		return err
	}
	return s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: "tenant_user.removed", TargetType: "tenant_user", TargetID: userID, Metadata: map[string]any{"tenant_id": tenantID}})
}

// findTenant 查找租户并把仓储层不存在错误转换为对外业务错误。
func (s *PlatformTenantUserService) findTenant(ctx context.Context, tenantID uint64) (*domain.Tenant, error) {
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return nil, response.ErrTenantNotFound
		}
		return nil, err
	}
	return tenant, nil
}

// findTenantMember 从租户成员列表中定位目标用户，用于返回最新成员展示数据。
func (s *PlatformTenantUserService) findTenantMember(ctx context.Context, tenantID uint64, userID uint64) (domain.TenantMemberDTO, error) {
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

// EnsurePlatformAdmin 为指定用户授予平台管理员角色，通常由本地受控命令调用。
func (s *PlatformRoleService) EnsurePlatformAdmin(ctx context.Context, userID uint64) error {
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return response.ErrBadRequest
	}
	// 平台管理员是 tenant_id 为 NULL 的全局角色，不能复用某个租户内的管理员授权。
	return s.tenants.EnsureUserRole(ctx, nil, userID, domain.RolePlatformAdmin)
}

// CreateTenantAdminAccount 为指定租户创建或复用租户管理员账号，并保证成员关系和 TENANT_ADMIN 授权同时存在。
func (s *PlatformRoleService) CreateTenantAdminAccount(ctx context.Context, actorID uint64, tenantID uint64, input CreateTenantAdminAccountInput) (domain.TenantAdminAssignmentDTO, error) {
	if input.UserID != 0 {
		return s.AssignTenantAdmin(ctx, actorID, tenantID, input.UserID)
	}
	if _, err := s.findEnabledTenant(ctx, tenantID); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	normalized := normalizeTenantAdminAccountInput(input)
	if err := validateTenantAdminAccountInput(normalized); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if normalized.Password == "" {
		normalized.Password = defaultTenantAdminPassword
	}
	user, created, err := s.findOrCreateTenantAdminUser(ctx, normalized)
	if err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	// 平台代建账号必须先进入 tenant_users，再写入租户内角色；这样权限判断始终有明确租户身份作为边界。
	if err := s.tenants.EnsureTenantUser(ctx, tenantID, user.ID, domain.TenantUserStatusActive); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if err := s.tenants.EnsureUserRole(ctx, &tenantID, user.ID, domain.RoleTenantAdmin); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	action := "tenant_admin.assigned"
	if created {
		action = "tenant_admin.account_created"
	}
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: action, TargetType: "tenant_admin", TargetID: user.ID, Metadata: map[string]any{"tenant_id": tenantID}}); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	userDTO := domain.ToUserDTO(*user, true)
	result := domain.TenantAdminAssignmentDTO{TenantID: tenantID, UserID: user.ID, Role: domain.RoleTenantAdmin, Assigned: true, CreatedUser: created, User: &userDTO}
	if created && normalized.Password == defaultTenantAdminPassword {
		result.TemporaryPassword = defaultTenantAdminPassword
	}
	return result, nil
}

// AssignTenantAdmin 为已加入租户的用户授予租户管理员角色，并记录审计事件。
func (s *PlatformRoleService) AssignTenantAdmin(ctx context.Context, actorID uint64, tenantID uint64, userID uint64) (domain.TenantAdminAssignmentDTO, error) {
	if err := s.ensureTenantMember(ctx, tenantID, userID); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	// 只有已加入租户的用户才能成为租户管理员，避免授权记录指向无成员关系的孤儿用户。
	if err := s.tenants.EnsureUserRole(ctx, &tenantID, userID, domain.RoleTenantAdmin); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: "tenant_admin.assigned", TargetType: "tenant_admin", TargetID: userID, Metadata: map[string]any{"tenant_id": tenantID}}); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	return domain.TenantAdminAssignmentDTO{TenantID: tenantID, UserID: userID, Role: domain.RoleTenantAdmin, Assigned: true}, nil
}

// RemoveTenantAdmin 撤销用户的租户管理员角色，撤销前会保护最后管理员。
func (s *PlatformRoleService) RemoveTenantAdmin(ctx context.Context, actorID uint64, tenantID uint64, userID uint64) (domain.TenantAdminAssignmentDTO, error) {
	if err := s.ensureTenantMember(ctx, tenantID, userID); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if err := ensureNotLastTenantAdmin(ctx, s.tenants, tenantID); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if err := s.tenants.RemoveUserRole(ctx, &tenantID, userID, domain.RoleTenantAdmin); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	if err := s.audit.Record(ctx, AuditEvent{ActorUserID: actorID, Action: "tenant_admin.removed", TargetType: "tenant_admin", TargetID: userID, Metadata: map[string]any{"tenant_id": tenantID}}); err != nil {
		return domain.TenantAdminAssignmentDTO{}, err
	}
	return domain.TenantAdminAssignmentDTO{TenantID: tenantID, UserID: userID, Role: domain.RoleTenantAdmin, Removed: true}, nil
}

// findEnabledTenant 校验租户存在且启用，供平台后台创建成员或授权前统一收口租户状态边界。
func (s *PlatformRoleService) findEnabledTenant(ctx context.Context, tenantID uint64) (*domain.Tenant, error) {
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return nil, response.ErrTenantNotFound
		}
		return nil, err
	}
	if tenant.Status != domain.TenantStatusEnabled {
		return nil, response.ErrTenantDisabled
	}
	return tenant, nil
}

// normalizeTenantAdminAccountInput 统一清理平台后台表单输入，避免大小写或空格导致邮箱重复判断失效。
func normalizeTenantAdminAccountInput(input CreateTenantAdminAccountInput) CreateTenantAdminAccountInput {
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Phone = strings.TrimSpace(input.Phone)
	input.Password = strings.TrimSpace(input.Password)
	return input
}

// validateTenantAdminAccountInput 校验平台代建账号的外部输入边界；密码可为空，由服务端使用固定初始密码。
func validateTenantAdminAccountInput(input CreateTenantAdminAccountInput) error {
	if !tenantAdminUsernamePattern.MatchString(input.Username) {
		return response.ErrBadRequest
	}
	if !validator.ValidNickname(input.DisplayName) {
		return response.ErrBadRequest
	}
	if !validator.ValidEmail(input.Email) {
		return response.ErrInvalidEmail
	}
	if len([]rune(input.Phone)) > 32 {
		return response.ErrBadRequest
	}
	return nil
}

// findOrCreateTenantAdminUser 按邮箱复用已有用户；仅新建账号时写入初始密码哈希和首次改密标记。
func (s *PlatformRoleService) findOrCreateTenantAdminUser(ctx context.Context, input CreateTenantAdminAccountInput) (*domain.User, bool, error) {
	user, err := s.users.FindByEmail(ctx, input.Email)
	if err == nil {
		return user, false, nil
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		return nil, false, err
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, false, err
	}
	user = &domain.User{
		Username:           input.Username,
		Email:              input.Email,
		PasswordHash:       passwordHash,
		Nickname:           input.DisplayName,
		Phone:              input.Phone,
		Role:               domain.RoleDataUser,
		Status:             domain.StatusActive,
		MustChangePassword: true,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, false, err
	}
	return user, true, nil
}

// ensureTenantMember 校验用户存在、租户启用且用户是 active 成员。
func (s *PlatformRoleService) ensureTenantMember(ctx context.Context, tenantID uint64, userID uint64) error {
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return response.ErrBadRequest
	}
	tenant, err := s.tenants.FindTenantByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantNotFound) {
			return response.ErrTenantNotFound
		}
		return err
	}
	if tenant.Status != domain.TenantStatusEnabled {
		return response.ErrTenantDisabled
	}
	// 角色授权必须建立在有效成员关系之上，否则权限判断会出现“有角色但无租户身份”的裂缝。
	member, err := s.tenants.FindTenantUser(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantMemberMissing) {
			return response.ErrTenantMemberForbidden
		}
		return err
	}
	if member.Status != domain.TenantUserStatusActive {
		return response.ErrTenantMemberDisabled
	}
	return nil
}

// Summary 汇总平台首页统计数据，使用聚合查询避免 dashboard 随租户和成员数量线性变慢。
func (s *PlatformDashboardService) Summary(ctx context.Context) (domain.PlatformDashboardDTO, error) {
	tenants, err := s.tenants.ListTenants(ctx)
	if err != nil {
		return domain.PlatformDashboardDTO{}, err
	}
	userCount, err := s.users.CountUsers(ctx)
	if err != nil {
		return domain.PlatformDashboardDTO{}, err
	}
	stats, err := s.tenants.ListTenantUsageStats(ctx)
	if err != nil {
		return domain.PlatformDashboardDTO{}, err
	}
	summary := domain.PlatformDashboardDTO{TenantCount: int64(len(tenants)), UserCount: userCount, AuditEnabled: false}
	for _, tenant := range tenants {
		switch tenant.Status {
		case domain.TenantStatusEnabled:
			summary.EnabledTenantCount++
		case domain.TenantStatusDisabled:
			summary.DisabledTenantCount++
		}
	}
	for _, stat := range stats {
		summary.TenantUserCount += stat.UserCount
		summary.TenantAdminCount += stat.TenantAdminCount
	}
	return summary, nil
}

// ensureNotLastTenantAdmin 确保撤销或移除管理员后租户仍至少保留一名管理员。
func ensureNotLastTenantAdmin(ctx context.Context, tenants repository.TenantRepository, tenantID uint64) error {
	count, err := tenants.CountTenantAdmins(ctx, tenantID)
	if err != nil {
		return err
	}
	if count <= 1 {
		// 至少保留一名活跃租户管理员，保证租户后续还能自助管理成员和权限。
		return response.ErrTenantLastAdminForbidden
	}
	return nil
}

// hasRoleCode 判断角色列表中是否包含指定角色编码。
func hasRoleCode(roles []domain.RoleCode, role domain.RoleCode) bool {
	for _, item := range roles {
		if item == role {
			return true
		}
	}
	return false
}

// tenantStatsByID 将聚合统计转成租户 ID 索引，避免服务层为了填充 DTO 再次查询数据库。
func tenantStatsByID(stats []repository.TenantUsageStats) map[uint64]repository.TenantUsageStats {
	result := make(map[uint64]repository.TenantUsageStats, len(stats))
	for _, stat := range stats {
		result[stat.TenantID] = stat
	}
	return result
}

// platformTenantDTO 将租户实体转换为平台后台租户 DTO。
func platformTenantDTO(tenant domain.Tenant) domain.TenantDTO {
	dto := toTenantDTO(tenant, nil)
	createdAt := tenant.CreatedAt
	updatedAt := tenant.UpdatedAt
	dto.CreatedAt = &createdAt
	dto.UpdatedAt = &updatedAt
	return dto
}
