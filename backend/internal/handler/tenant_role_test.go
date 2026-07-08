package handler

import (
	"net/http"
	"testing"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
)

// TestTenantMemberRoleEndpoint 验证租户管理员可通过接口分配并替换普通业务角色。
func TestTenantMemberRoleEndpoint(t *testing.T) {
	app := newTestApp()
	adminAccess := createAdminAndLogin(t, app)
	memberID := createTenantMemberForRoleTest(t, app, 1, "member-role@example.com")

	owner := performJSON(app.router, http.MethodPut, "/api/v1/tenants/1/members/2/role", map[string]any{"roleCode": "DATA_OWNER"}, adminAccess)
	if owner.Code != http.StatusOK || !bytesContains(owner.Body.String(), "DO") {
		t.Fatalf("assign owner status=%d body=%s", owner.Code, owner.Body.String())
	}
	visitor := performJSON(app.router, http.MethodPut, "/api/v1/tenants/1/members/2/role", map[string]any{"roleCode": "DATA_VISITOR"}, adminAccess)
	if visitor.Code != http.StatusOK || !bytesContains(visitor.Body.String(), "DU") || bytesContains(visitor.Body.String(), "DO") {
		t.Fatalf("assign visitor should replace owner, member=%d status=%d body=%s", memberID, visitor.Code, visitor.Body.String())
	}
}

// TestTenantMemberRoleEndpointRejectsForbiddenActors 验证平台管理员、普通成员和非法角色不能调用普通角色分配接口。
func TestTenantMemberRoleEndpointRejectsForbiddenActors(t *testing.T) {
	app := newTestApp()
	adminAccess := createAdminAndLogin(t, app)
	memberID := createTenantMemberForRoleTest(t, app, 1, "member-denied@example.com")
	memberAccess := loginExistingUser(t, app, "member-denied@example.com")
	platformAccess := createPlatformAdminAndLogin(t, app)

	platform := performJSON(app.router, http.MethodPut, "/api/v1/tenants/1/members/2/role", map[string]any{"roleCode": "DATA_OWNER"}, platformAccess)
	if platform.Code != http.StatusForbidden || !bytesContains(platform.Body.String(), "TENANT_ROLE_ASSIGN_PLATFORM_FORBIDDEN") {
		t.Fatalf("platform status=%d body=%s", platform.Code, platform.Body.String())
	}
	ordinary := performJSON(app.router, http.MethodPut, "/api/v1/tenants/1/members/1/role", map[string]any{"roleCode": "DATA_OWNER"}, memberAccess)
	if ordinary.Code != http.StatusForbidden || !bytesContains(ordinary.Body.String(), "TENANT_PERMISSION_DENIED") {
		t.Fatalf("ordinary status=%d body=%s", ordinary.Code, ordinary.Body.String())
	}
	invalid := performJSON(app.router, http.MethodPut, "/api/v1/tenants/1/members/2/role", map[string]any{"roleCode": "TENANT_ADMIN"}, adminAccess)
	if invalid.Code != http.StatusBadRequest || !bytesContains(invalid.Body.String(), "INVALID_ROLE") {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	self := performJSON(app.router, http.MethodPut, "/api/v1/tenants/1/members/1/role", map[string]any{"roleCode": "DATA_VISITOR"}, adminAccess)
	if self.Code != http.StatusForbidden || !bytesContains(self.Body.String(), "TENANT_ADMIN_SELF_ROLE_FORBIDDEN") {
		t.Fatalf("self status=%d body=%s member=%d", self.Code, self.Body.String(), memberID)
	}
}

// createTenantMemberForRoleTest 创建租户成员测试用户并加入指定租户，避免依赖公开注册默认租户迁移。
func createTenantMemberForRoleTest(t *testing.T, app testApp, tenantID uint64, email string) uint64 {
	t.Helper()
	hash, err := auth.HashPassword("Passw0rd!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &domain.User{Email: email, PasswordHash: hash, Nickname: "成员", Role: domain.RoleDataUser, Status: domain.StatusActive}
	if err := app.repo.Create(nil, user); err != nil {
		t.Fatalf("create member: %v", err)
	}
	if err := app.tenantRepo.EnsureTenantUser(nil, tenantID, user.ID, domain.TenantUserStatusActive); err != nil {
		t.Fatalf("tenant member: %v", err)
	}
	return user.ID
}

// loginExistingUser 使用测试用户的固定密码登录并返回 access token。
func loginExistingUser(t *testing.T, app testApp, email string) string {
	t.Helper()
	w := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": "Passw0rd!"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login %s status=%d body=%s", email, w.Code, w.Body.String())
	}
	data := parseData(t, w)
	return data["access_token"].(string)
}
