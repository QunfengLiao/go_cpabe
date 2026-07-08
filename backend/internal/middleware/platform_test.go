package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
)

// platformRoleResolverStub 是平台管理员中间件测试使用的角色查询桩。
type platformRoleResolverStub struct {
	allowed bool
	err     error
}

// HasRole 模拟平台角色查询结果，供平台管理员中间件测试控制授权路径。
func (s platformRoleResolverStub) HasRole(context.Context, uint64, *uint64, domain.RoleCode) (bool, error) {
	return s.allowed, s.err
}

// TestPlatformAdminRequiredAllowsPlatformRole 验证平台角色存在时平台后台请求可继续执行。
func TestPlatformAdminRequiredAllowsPlatformRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/platform", func(c *gin.Context) {
		c.Set(ContextUserID, uint64(1))
	}, PlatformAdminRequired(platformRoleResolverStub{allowed: true}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/platform", nil))

	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

// TestPlatformAdminRequiredRejectsTenantRole 验证没有平台角色时平台后台请求会被拒绝。
func TestPlatformAdminRequiredRejectsTenantRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/platform", func(c *gin.Context) {
		c.Set(ContextUserID, uint64(1))
	}, PlatformAdminRequired(platformRoleResolverStub{allowed: false}), func(c *gin.Context) {
		t.Fatal("handler should not run")
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/platform", nil))

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), response.ErrPlatformPermissionDenied.Code) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
