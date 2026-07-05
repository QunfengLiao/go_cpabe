package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/response"
)

const (
	ContextUserID = "user_id"
	ContextRole   = "role"
)

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
