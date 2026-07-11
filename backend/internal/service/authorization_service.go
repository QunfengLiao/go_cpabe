package service

import (
	"context"
	"errors"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// AuthorizationService 负责统一 RBAC 授权决策，权限事实源只来自数据库。
type AuthorizationService struct {
	rbac repository.RBACRepository
}

// NewAuthorizationService 创建统一授权服务；服务只依赖 RBAC 仓储以避免和业务 Service 形成循环依赖。
func NewAuthorizationService(rbac repository.RBACRepository) *AuthorizationService {
	return &AuthorizationService{rbac: rbac}
}

// PlatformPermissions 查询用户平台作用域权限，平台权限不会自动转换为租户业务权限。
func (s *AuthorizationService) PlatformPermissions(ctx context.Context, userID uint64) ([]string, error) {
	if s == nil || s.rbac == nil {
		return nil, response.ErrInternal
	}
	return s.rbac.ListPlatformPermissionCodesByUser(ctx, userID)
}

// TenantPermissions 查询用户在指定租户内由所有有效角色产生的权限并集。
func (s *AuthorizationService) TenantPermissions(ctx context.Context, userID uint64, tenantID uint64) ([]string, error) {
	if s == nil || s.rbac == nil {
		return nil, response.ErrInternal
	}
	return s.rbac.ListTenantPermissionCodesByUser(ctx, tenantID, userID)
}

// HasPlatformPermission 判断用户是否拥有指定平台权限。
func (s *AuthorizationService) HasPlatformPermission(ctx context.Context, userID uint64, code string) (bool, error) {
	if s == nil || s.rbac == nil {
		return false, response.ErrInternal
	}
	return s.rbac.HasPlatformPermission(ctx, userID, code)
}

// HasTenantPermission 判断用户是否拥有指定租户权限。
func (s *AuthorizationService) HasTenantPermission(ctx context.Context, userID uint64, tenantID uint64, code string) (bool, error) {
	if s == nil || s.rbac == nil {
		return false, response.ErrInternal
	}
	return s.rbac.HasTenantPermission(ctx, tenantID, userID, code)
}

// RequirePlatformPermission 要求用户拥有指定平台权限，无权限时返回统一业务错误。
func (s *AuthorizationService) RequirePlatformPermission(ctx context.Context, userID uint64, code string) error {
	ok, err := s.HasPlatformPermission(ctx, userID, code)
	if err != nil {
		return err
	}
	if !ok {
		return response.ErrPermissionDenied
	}
	return nil
}

// RequireTenantPermission 要求用户拥有指定租户权限，无权限时返回统一业务错误。
func (s *AuthorizationService) RequireTenantPermission(ctx context.Context, userID uint64, tenantID uint64, code string) error {
	ok, err := s.HasTenantPermission(ctx, userID, tenantID, code)
	if err != nil {
		return err
	}
	if !ok {
		return response.ErrPermissionDenied
	}
	return nil
}

// CurrentTenantAuthorization 返回当前登录用户在可信租户上下文中的真实角色和权限。
func (s *AuthorizationService) CurrentTenantAuthorization(ctx context.Context, userID uint64, tenantID uint64) (domain.AuthorizationContextDTO, error) {
	return s.UserTenantAuthorization(ctx, tenantID, userID)
}

// UserTenantAuthorization 返回指定成员在当前租户内的角色和权限，调用方必须先完成管理权限校验。
func (s *AuthorizationService) UserTenantAuthorization(ctx context.Context, tenantID uint64, targetUserID uint64) (domain.AuthorizationContextDTO, error) {
	if s == nil || s.rbac == nil {
		return domain.AuthorizationContextDTO{}, response.ErrInternal
	}
	roles, err := s.rbac.ListMemberRoles(ctx, tenantID, targetUserID)
	if err != nil {
		if errors.Is(err, repository.ErrTenantMemberMissing) {
			return domain.AuthorizationContextDTO{}, response.ErrTenantMemberForbidden
		}
		return domain.AuthorizationContextDTO{}, err
	}
	permissions, err := s.rbac.ListTenantPermissionCodesByUser(ctx, tenantID, targetUserID)
	if err != nil {
		return domain.AuthorizationContextDTO{}, err
	}
	return domain.AuthorizationContextDTO{TenantID: tenantID, Roles: toTenantRoleDTOs(roles), Permissions: permissions}, nil
}

// toTenantRoleDTOs 将角色实体转换为授权上下文对外摘要，避免暴露 Gorm 软删除等内部字段。
func toTenantRoleDTOs(roles []domain.Role) []domain.TenantRoleDTO {
	result := make([]domain.TenantRoleDTO, 0, len(roles))
	for _, role := range roles {
		result = append(result, toTenantRoleDTO(role, 0, 0))
	}
	return result
}

// toTenantRoleDTO 转换角色实体并补充可选统计值，供角色列表、成员角色和授权上下文复用。
func toTenantRoleDTO(role domain.Role, permissionCount int64, activeMemberCount int64) domain.TenantRoleDTO {
	category := role.RoleCategory
	return domain.TenantRoleDTO{
		ID:                role.ID,
		TenantID:          role.TenantID,
		Code:              string(role.Code),
		Name:              role.Name,
		Description:       role.Description,
		ScopeType:         role.ScopeType,
		RoleCategory:      category,
		Category:          category,
		Builtin:           role.IsBuiltin,
		IsBuiltin:         role.IsBuiltin,
		Status:            role.Status,
		PermissionCount:   permissionCount,
		ActiveMemberCount: activeMemberCount,
		CreatedAt:         role.CreatedAt,
		UpdatedAt:         role.UpdatedAt,
	}
}
