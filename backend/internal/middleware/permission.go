package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/pkg/response"
)

// PermissionAuthorizer 定义权限中间件所需的最小授权服务接口，避免 middleware 反向依赖具体业务实现。
type PermissionAuthorizer interface {
	RequireTenantPermission(ctx context.Context, userID uint64, tenantID uint64, code string) error
	RequirePlatformPermission(ctx context.Context, userID uint64, code string) error
}

// TenantPermissionRequired 拦截当前租户接口，并要求登录用户拥有指定租户 permission code。
func TenantPermissionRequired(authorizer PermissionAuthorizer, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := currentUserID(c)
		if !ok {
			response.Fail(c, response.ErrAccessTokenInvalid)
			c.Abort()
			return
		}
		tenantID, ok := CurrentTenantID(c)
		if !ok || tenantID == 0 {
			response.Fail(c, response.ErrTenantIDMissing)
			c.Abort()
			return
		}
		if authorizer == nil {
			response.Fail(c, response.ErrInternal)
			c.Abort()
			return
		}
		// 权限中间件只做功能权限判断；资源归属、owner 和状态机仍由后续 Service 负责。
		if err := authorizer.RequireTenantPermission(c.Request.Context(), userID, tenantID, code); err != nil {
			response.Fail(c, err)
			c.Abort()
			return
		}
		c.Next()
	}
}

// PlatformPermissionRequired 拦截平台接口，并要求登录用户拥有指定平台 permission code。
func PlatformPermissionRequired(authorizer PermissionAuthorizer, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := currentUserID(c)
		if !ok {
			response.Fail(c, response.ErrAccessTokenInvalid)
			c.Abort()
			return
		}
		if authorizer == nil {
			response.Fail(c, response.ErrInternal)
			c.Abort()
			return
		}
		if err := authorizer.RequirePlatformPermission(c.Request.Context(), userID, code); err != nil {
			response.Fail(c, err)
			c.Abort()
			return
		}
		c.Next()
	}
}
