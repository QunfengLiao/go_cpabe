package handler

import (
	"net/http"
	"testing"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
)

// TestTenantAdminEndpoints 验证租户管理员对成员管理接口的权限和行为。
func TestTenantAdminEndpoints(t *testing.T) {
	app := newTestApp()
	adminAccess := createAdminAndLogin(t, app)

	list := performJSON(app.router, http.MethodGet, "/api/v1/tenants", nil, adminAccess)
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	access, _ := registerAndLogin(t, app)
	add := performJSON(app.router, http.MethodPost, "/api/v1/tenants/1/users", map[string]any{"user_id": 1, "roles": []string{"DU"}}, adminAccess)
	if add.Code != http.StatusOK {
		t.Fatalf("add status=%d body=%s", add.Code, add.Body.String())
	}

	users := performJSON(app.router, http.MethodGet, "/api/v1/tenants/1/users", nil, adminAccess)
	if users.Code != http.StatusOK {
		t.Fatalf("users status=%d body=%s", users.Code, users.Body.String())
	}

	platformOnly := performJSON(app.router, http.MethodPatch, "/api/v1/tenants/1/disable", nil, adminAccess)
	if platformOnly.Code != http.StatusForbidden {
		t.Fatalf("tenant lifecycle should require platform role, status=%d body=%s", platformOnly.Code, platformOnly.Body.String())
	}
	denied := performJSON(app.router, http.MethodPost, "/api/v1/me/switch-tenant", map[string]any{"tenant_id": 1}, access)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("switch disabled status=%d body=%s", denied.Code, denied.Body.String())
	}
}

// createAdminAndLogin 创建旧管理员测试用户并返回登录 access token。
func createAdminAndLogin(t *testing.T, app testApp) string {
	t.Helper()
	hash, err := auth.HashPassword("Passw0rd!")
	if err != nil {
		t.Fatalf("hash admin password: %v", err)
	}
	user := &domain.User{
		Email:        "admin@example.com",
		Nickname:     "管理员",
		PasswordHash: hash,
		Role:         domain.RoleAdmin,
		Status:       domain.StatusActive,
	}
	if err := app.repo.Create(nil, user); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := app.tenantRepo.EnsureTenantUser(nil, 1, user.ID, "active"); err != nil {
		t.Fatalf("admin member: %v", err)
	}
	tenantID := uint64(1)
	if err := app.tenantRepo.EnsureUserRole(nil, &tenantID, user.ID, "TENANT_ADMIN"); err != nil {
		t.Fatalf("admin role: %v", err)
	}
	w := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "admin@example.com", "password": "Passw0rd!"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin login status=%d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w)
	return data["access_token"].(string)
}
