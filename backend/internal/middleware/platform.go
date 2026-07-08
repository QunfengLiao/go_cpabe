package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
)

// PlatformRoleResolver 定义平台后台中间件校验平台角色所需的查询能力。
type PlatformRoleResolver interface {
	HasRole(ctx context.Context, userID uint64, tenantID *uint64, roleCode domain.RoleCode) (bool, error)
}

// PlatformAdminRequired 拦截平台后台接口，只允许拥有 tenant_id IS NULL 平台管理员角色的用户访问。
func PlatformAdminRequired(roles PlatformRoleResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := currentUserID(c)
		if !ok {
			response.Fail(c, response.ErrAccessTokenInvalid)
			c.Abort()
			return
		}
		if roles == nil {
			response.Fail(c, response.ErrInternal)
			c.Abort()
			return
		}
		// 平台后台不能依赖 X-Tenant-Id；只查询 tenant_id IS NULL 的平台角色，避免租户管理员越权治理平台。
		allowed, err := roles.HasRole(c.Request.Context(), userID, nil, domain.RolePlatformAdmin)
		if err != nil {
			response.Fail(c, err)
			c.Abort()
			return
		}
		if !allowed {
			response.Fail(c, response.ErrPlatformPermissionDenied)
			c.Abort()
			return
		}
		c.Next()
	}
}
