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

type platformRoleResolverStub struct {
	allowed bool
	err     error
}

func (s platformRoleResolverStub) HasRole(context.Context, uint64, *uint64, domain.RoleCode) (bool, error) {
	return s.allowed, s.err
}

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
