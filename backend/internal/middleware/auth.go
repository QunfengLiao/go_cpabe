package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/response"
)

const (
	// ContextUserID 是认证中间件写入 gin.Context 的当前用户 ID 键。
	ContextUserID = "user_id"
	// ContextRole 是认证中间件写入 gin.Context 的旧单租户角色键。
	ContextRole = "role"
)

// AuthRequired 拦截需要登录的请求，从 Authorization Bearer 头解析 access token 并写入用户上下文。
func AuthRequired(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			response.Fail(c, response.ErrAccessTokenMissing)
			c.Abort()
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			response.Fail(c, response.ErrAccessTokenInvalid)
			c.Abort()
			return
		}
		// 中间件只解析短期 access token；用户状态和细粒度权限由后续 service/middleware 再次查库确认。
		claims, err := manager.ParseAccessToken(parts[1])
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrTokenExpired):
				response.Fail(c, response.ErrAccessTokenExpired)
			default:
				response.Fail(c, response.ErrAccessTokenInvalid)
			}
			c.Abort()
			return
		}
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextRole, claims.Role)
		c.Next()
	}
}
