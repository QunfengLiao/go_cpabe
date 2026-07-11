package service

import (
	"context"
	"errors"
	"strings"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// TenantRoleService 负责租户自定义角色、角色权限和成员多角色的业务编排。
type TenantRoleService struct {
	rbac          repository.RBACRepository
	authorization *AuthorizationService
}

// CreateTenantRoleInput 表示创建租户自定义角色时允许客户端提交的字段。
type CreateTenantRoleInput struct {
	Code            string
	Name            string
	Description     string
	PermissionCodes []string
}

// UpdateTenantRoleInput 表示修改租户自定义角色展示信息时允许提交的字段。
type UpdateTenantRoleInput struct {
	Name        string
	Description string
}

// ReplaceRolePermissionsInput 表示全量替换自定义角色权限的请求。
type ReplaceRolePermissionsInput struct {
	PermissionCodes []string
}

// ReplaceMemberRolesInput 表示全量替换租户成员角色集合的请求。
type ReplaceMemberRolesInput struct {
	RoleIDs []uint64
}

// NewTenantRoleService 创建租户 RBAC 角色服务。
func NewTenantRoleService(rbac repository.RBACRepository, authorization *AuthorizationService) *TenantRoleService {
	return &TenantRoleService{rbac: rbac, authorization: authorization}
}

// ListPermissions 返回当前租户自定义角色可绑定的有效租户权限目录。
func (s *TenantRoleService) ListPermissions(ctx context.Context) ([]domain.PermissionDTO, error) {
	if s == nil || s.rbac == nil {
		return nil, response.ErrInternal
	}
	permissions, err := s.rbac.ListTenantPermissions(ctx)
	if err != nil {
		return nil, err
	}
	return toPermissionDTOs(permissions), nil
}

// ListRoles 返回系统内置租户角色和当前租户自定义角色，附带权限数和有效成员数。
func (s *TenantRoleService) ListRoles(ctx context.Context, tenantID uint64) ([]domain.TenantRoleDTO, error) {
	records, err := s.rbac.ListTenantRoles(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	items := make([]domain.TenantRoleDTO, 0, len(records))
	for _, record := range records {
		items = append(items, toTenantRoleDTO(record.Role, record.PermissionCount, record.ActiveMemberCount))
	}
	return items, nil
}

// CreateRole 创建当前租户自定义业务角色，并可在同一事务中绑定权限。
func (s *TenantRoleService) CreateRole(ctx context.Context, tenantID uint64, actorID uint64, input CreateTenantRoleInput) (domain.TenantRoleDTO, error) {
	code := normalizeRoleCode(input.Code)
	name := strings.TrimSpace(input.Name)
	if tenantID == 0 || code == "" || name == "" {
		return domain.TenantRoleDTO{}, response.ErrBadRequest
	}
	createdBy := actorID
	role := domain.Role{
		TenantID:      tenantID,
		Code:          domain.RoleCode(code),
		Name:          name,
		Description:   strings.TrimSpace(input.Description),
		Scope:         domain.RoleScopeTenant,
		ScopeType:     domain.RoleScopeTypeTenant,
		RoleCategory:  domain.RoleCategoryBusiness,
		IsBuiltin:     false,
		Status:        domain.RoleStatusActive,
		CreatedBy:     &createdBy,
		UpdatedBy:     &createdBy,
	}
	created, err := s.rbac.CreateTenantCustomRole(ctx, role, input.PermissionCodes)
	if err != nil {
		return domain.TenantRoleDTO{}, mapRBACError(err)
	}
	return toTenantRoleDTO(*created, int64(len(input.PermissionCodes)), 0), nil
}

// GetRole 查询当前租户可见角色详情。
func (s *TenantRoleService) GetRole(ctx context.Context, tenantID uint64, roleID uint64) (domain.TenantRoleDTO, error) {
	role, err := s.rbac.FindTenantRole(ctx, tenantID, roleID)
	if err != nil {
		return domain.TenantRoleDTO{}, mapRBACError(err)
	}
	permissions, err := s.rbac.ListRolePermissions(ctx, tenantID, roleID)
	if err != nil {
		return domain.TenantRoleDTO{}, mapRBACError(err)
	}
	memberCount, err := s.rbac.CountRoleActiveMembers(ctx, tenantID, roleID)
	if err != nil {
		return domain.TenantRoleDTO{}, err
	}
	return toTenantRoleDTO(*role, int64(len(permissions)), memberCount), nil
}

// UpdateRole 修改当前租户自定义业务角色的展示字段，role code、作用域和分类保持不可变。
func (s *TenantRoleService) UpdateRole(ctx context.Context, tenantID uint64, roleID uint64, actorID uint64, input UpdateTenantRoleInput) (domain.TenantRoleDTO, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.TenantRoleDTO{}, response.ErrBadRequest
	}
	role, err := s.rbac.UpdateTenantCustomRole(ctx, tenantID, roleID, name, strings.TrimSpace(input.Description), actorID)
	if err != nil {
		return domain.TenantRoleDTO{}, mapRBACError(err)
	}
	return toTenantRoleDTO(*role, 0, 0), nil
}

// DisableRole 逻辑禁用当前租户自定义角色，并返回受影响成员数。
func (s *TenantRoleService) DisableRole(ctx context.Context, tenantID uint64, roleID uint64, actorID uint64) (uint64, int64, error) {
	count, err := s.rbac.DisableTenantCustomRole(ctx, tenantID, roleID, actorID)
	if err != nil {
		return 0, 0, mapRBACError(err)
	}
	return roleID, count, nil
}

// ListRolePermissions 查询当前租户可见角色的权限绑定。
func (s *TenantRoleService) ListRolePermissions(ctx context.Context, tenantID uint64, roleID uint64) (domain.RolePermissionDTO, error) {
	permissions, err := s.rbac.ListRolePermissions(ctx, tenantID, roleID)
	if err != nil {
		return domain.RolePermissionDTO{}, mapRBACError(err)
	}
	return domain.RolePermissionDTO{RoleID: roleID, PermissionCodes: permissionCodes(permissions), Permissions: toPermissionDTOs(permissions)}, nil
}

// ReplaceRolePermissions 全量替换当前租户自定义业务角色权限，空数组表示清空。
func (s *TenantRoleService) ReplaceRolePermissions(ctx context.Context, tenantID uint64, roleID uint64, actorID uint64, input ReplaceRolePermissionsInput) (domain.RolePermissionDTO, error) {
	if err := s.rbac.ReplaceRolePermissions(ctx, tenantID, roleID, input.PermissionCodes, actorID); err != nil {
		return domain.RolePermissionDTO{}, mapRBACError(err)
	}
	return s.ListRolePermissions(ctx, tenantID, roleID)
}

// GetMemberRoles 查询指定成员在当前租户内的角色和权限并集。
func (s *TenantRoleService) GetMemberRoles(ctx context.Context, tenantID uint64, userID uint64) (domain.MemberRoleDTO, error) {
	roles, err := s.rbac.ListMemberRoles(ctx, tenantID, userID)
	if err != nil {
		return domain.MemberRoleDTO{}, mapRBACError(err)
	}
	permissions, err := s.rbac.ListTenantPermissionCodesByUser(ctx, tenantID, userID)
	if err != nil {
		return domain.MemberRoleDTO{}, err
	}
	return domain.MemberRoleDTO{TenantID: tenantID, UserID: userID, Roles: toTenantRoleDTOs(roles), Permissions: permissions}, nil
}

// ReplaceMemberRoles 全量替换指定成员角色集合，Repository 负责事务、角色范围和最后管理员保护。
func (s *TenantRoleService) ReplaceMemberRoles(ctx context.Context, tenantID uint64, userID uint64, actorID uint64, input ReplaceMemberRolesInput) (domain.MemberRoleDTO, error) {
	if err := s.rbac.ReplaceMemberRoles(ctx, tenantID, userID, input.RoleIDs, actorID); err != nil {
		return domain.MemberRoleDTO{}, mapRBACError(err)
	}
	return s.GetMemberRoles(ctx, tenantID, userID)
}

// CurrentAuthorization 返回当前登录用户在当前租户内的真实授权上下文。
func (s *TenantRoleService) CurrentAuthorization(ctx context.Context, tenantID uint64, userID uint64) (domain.AuthorizationContextDTO, error) {
	if s.authorization == nil {
		return domain.AuthorizationContextDTO{}, response.ErrInternal
	}
	return s.authorization.CurrentTenantAuthorization(ctx, userID, tenantID)
}

// normalizeRoleCode 统一自定义角色 code 格式，避免大小写差异绕过同租户唯一约束。
func normalizeRoleCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// toPermissionDTOs 将权限实体转换为对外 DTO。
func toPermissionDTOs(permissions []domain.Permission) []domain.PermissionDTO {
	result := make([]domain.PermissionDTO, 0, len(permissions))
	for _, permission := range permissions {
		result = append(result, domain.PermissionDTO{
			ID:           permission.ID,
			Code:         permission.Code,
			Name:         permission.Name,
			Description:  permission.Description,
			ScopeType:    permission.ScopeType,
			ResourceType: permission.ResourceType,
			Action:       permission.Action,
			Status:       permission.Status,
		})
	}
	return result
}

// permissionCodes 提取权限 code 集合，用于角色权限接口保持简洁响应。
func permissionCodes(permissions []domain.Permission) []string {
	result := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		result = append(result, permission.Code)
	}
	return result
}

// mapRBACError 将仓储层错误转换为稳定 API 错误码。
func mapRBACError(err error) error {
	switch {
	case errors.Is(err, repository.ErrRoleNotFound):
		return response.ErrRoleNotFound
	case errors.Is(err, repository.ErrRoleCodeExists):
		return response.ErrRoleCodeExists
	case errors.Is(err, repository.ErrPermissionNotFound):
		return response.ErrInvalidPermissionScope
	case errors.Is(err, repository.ErrBuiltinRoleImmutable):
		return response.ErrBuiltinRoleImmutable
	case errors.Is(err, repository.ErrRoleDisabled):
		return response.ErrRoleDisabled
	case errors.Is(err, repository.ErrInvalidRoleScope):
		return response.ErrInvalidRoleScope
	case errors.Is(err, repository.ErrTenantMemberMissing):
		return response.ErrMemberNotFoundInTenant
	case errors.Is(err, repository.ErrCannotAssignPlatformRole):
		return response.ErrCannotAssignPlatformRole
	case errors.Is(err, repository.ErrCannotRemoveLastTenantAdmin):
		return response.ErrCannotRemoveLastTenantAdmin
	default:
		return err
	}
}
