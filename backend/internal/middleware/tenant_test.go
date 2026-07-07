package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
)

type tenantValidatorStub struct {
	tenant *domain.Tenant
	roles  []domain.RoleCode
	err    error
}

func (s tenantValidatorStub) ResolveTenantContext(_ context.Context, _ uint64, _ uint64) (*domain.Tenant, []domain.RoleCode, error) {
	return s.tenant, s.roles, s.err
}

func TestTenantRequiredRejectsMissingTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/tenant", func(c *gin.Context) {
		c.Set(ContextUserID, uint64(1))
		TenantRequired(nil)(c)
	}, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/tenant", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestTenantRequiredStoresTenantContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	resolver := tenantValidatorStub{tenant: &domain.Tenant{ID: 7, Code: "lab-a"}, roles: []domain.RoleCode{domain.RoleDO}}
	router.GET("/tenant", func(c *gin.Context) {
		c.Set(ContextUserID, uint64(1))
	}, TenantRequired(resolver), func(c *gin.Context) {
		if id, ok := CurrentTenantID(c); !ok || id != 7 {
			t.Fatalf("unexpected tenant id: %d %v", id, ok)
		}
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/tenant", nil)
	req.Header.Set("X-Tenant-Id", "7")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestTenantRequiredRejectsForbiddenTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	resolver := tenantValidatorStub{err: response.ErrTenantMemberForbidden}
	router.GET("/tenant", func(c *gin.Context) {
		c.Set(ContextUserID, uint64(1))
	}, TenantRequired(resolver), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/tenant", nil)
	req.Header.Set("X-Tenant-Id", "8")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
