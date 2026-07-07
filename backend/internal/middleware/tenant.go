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
	ContextTenantID    = "tenant_id"
	ContextTenantRoles = "tenant_roles"
	ContextTenantCode  = "tenant_code"
)

type TenantResolver interface {
	ResolveTenantContext(ctx context.Context, userID uint64, tenantID uint64) (*domain.Tenant, []domain.RoleCode, error)
}

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

func currentUserID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(ContextUserID)
	if !ok {
		return 0, false
	}
	id, ok := value.(uint64)
	return id, ok
}

func CurrentTenantID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(ContextTenantID)
	if !ok {
		return 0, false
	}
	id, ok := value.(uint64)
	return id, ok
}
