package middleware

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
)

const (
	// ContextTenantID 是租户中间件写入 gin.Context 的当前租户 ID 键。
	ContextTenantID = "tenant_id"
	// ContextTenantRoles 是租户中间件写入 gin.Context 的当前租户内角色列表键。
	ContextTenantRoles = "tenant_roles"
	// ContextTenantCode 是租户中间件写入 gin.Context 的当前租户编码键。
	ContextTenantCode = "tenant_code"
)

// TenantResolver 定义租户中间件校验租户上下文所需的服务能力。
type TenantResolver interface {
	ResolveTenantContext(ctx context.Context, userID uint64, tenantID uint64) (*domain.Tenant, []domain.RoleCode, error)
}

// TenantRequired 从 X-Tenant-Id 读取租户选择，校验成员关系后写入当前租户上下文。
func TenantRequired(tenants TenantResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := currentUserID(c)
		if !ok {
			response.Fail(c, response.ErrAccessTokenInvalid)
			c.Abort()
			return
		}
		rawTenantID := strings.TrimSpace(c.GetHeader("X-Tenant-Id"))
		if rawTenantID == "" {
			response.Fail(c, response.ErrTenantIDMissing)
			c.Abort()
			return
		}
		tenantID, err := strconv.ParseUint(rawTenantID, 10, 64)
		if err != nil || tenantID == 0 {
			response.Fail(c, response.ErrTenantIDInvalid)
			c.Abort()
			return
		}
		if tenants == nil {
			response.Fail(c, response.ErrInternal)
			c.Abort()
			return
		}
		// X-Tenant-Id 只是用户选择的租户输入，真正可信的边界来自后端对租户状态和成员关系的校验。
		tenant, roles, err := tenants.ResolveTenantContext(c.Request.Context(), userID, tenantID)
		if err != nil {
			response.Fail(c, err)
			c.Abort()
			return
		}
		c.Set(ContextTenantID, tenant.ID)
		c.Set(ContextTenantCode, tenant.Code)
		c.Set(ContextTenantRoles, roles)
		c.Next()
	}
}

// currentUserID 从 gin.Context 读取认证中间件写入的用户 ID。
func currentUserID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(ContextUserID)
	if !ok {
		return 0, false
	}
	id, ok := value.(uint64)
	return id, ok
}

// CurrentTenantID 从 gin.Context 读取当前租户 ID，供后续 handler 或 service 装配使用。
func CurrentTenantID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(ContextTenantID)
	if !ok {
		return 0, false
	}
	id, ok := value.(uint64)
	return id, ok
}
