package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
)

// TestAuthRequiredAcceptsOnlyAccessToken 验证认证中间件只接受 access token 并拒绝缺失凭证。
func TestAuthRequiredAcceptsOnlyAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := auth.NewManager("secret", time.Minute)
	token, _, err := manager.GenerateAccessToken(1, domain.RoleDataUser)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	router := gin.New()
	router.GET("/protected", AuthRequired(manager), func(c *gin.Context) {
		if c.GetUint64(ContextUserID) != 1 {
			t.Fatalf("missing user id")
		}
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing status=%d", missing.Code)
	}
}
