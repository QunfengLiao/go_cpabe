package handler

import (
	"net/http"
	"testing"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
)

// TestTenantMemberRoleEndpointRemoved 验证旧单角色成员角色接口已经从当前租户授权链路中移除。
func TestTenantMemberRoleEndpointRemoved(t *testing.T) {
	app := newTestApp()
	adminAccess := createAdminAndLogin(t, app)

	removed := performJSON(app.router, http.MethodPut, "/api/v1/tenants/1/members/2/role", map[string]any{"roleCode": "DATA_OWNER"}, adminAccess)
	if removed.Code != http.StatusNotFound {
		t.Fatalf("old member role endpoint should be removed, status=%d body=%s", removed.Code, removed.Body.String())
	}
}

// TestTenantMemberRolesEndpointReplacesFullRoleCodeSet 验证当前租户成员角色接口按 role code 集合全量替换。
func TestTenantMemberRoleEndpoint(t *testing.T) {
	app := newTestApp()
	adminAccess := createAdminAndLogin(t, app)
	memberID := createTenantMemberForRoleTest(t, app, 1, "member-role@example.com")

	owner := performJSONWithTenant(app.router, http.MethodPut, "/api/v1/tenant/members/2/roles", map[string]any{"roleCodes": []string{"DO"}}, adminAccess, 1)
	if owner.Code != http.StatusOK || !bytesContains(owner.Body.String(), "DO") {
		t.Fatalf("assign owner status=%d body=%s", owner.Code, owner.Body.String())
	}
	visitor := performJSONWithTenant(app.router, http.MethodPut, "/api/v1/tenant/members/2/roles", map[string]any{"roleCodes": []string{"DO", "DU"}}, adminAccess, 1)
	if visitor.Code != http.StatusOK || !bytesContains(visitor.Body.String(), "DU") || !bytesContains(visitor.Body.String(), "DO") {
		t.Fatalf("assign visitor should preserve submitted owner capability, member=%d status=%d body=%s", memberID, visitor.Code, visitor.Body.String())
	}
	replaced := performJSONWithTenant(app.router, http.MethodPut, "/api/v1/tenant/members/2/roles", map[string]any{"roleCodes": []string{"DU"}}, adminAccess, 1)
	if replaced.Code != http.StatusOK || bytesContains(replaced.Body.String(), "DO") || !bytesContains(replaced.Body.String(), "DU") {
		t.Fatalf("full replacement should remove DO and keep DU, status=%d body=%s", replaced.Code, replaced.Body.String())
	}
}

// TestTenantMemberRoleEndpointRejectsForbiddenActors 验证平台角色和不存在角色不能通过当前租户成员角色接口分配。
func TestTenantMemberRoleEndpointRejectsForbiddenActors(t *testing.T) {
	app := newTestApp()
	adminAccess := createAdminAndLogin(t, app)
	memberID := createTenantMemberForRoleTest(t, app, 1, "member-denied@example.com")

	invalid := performJSONWithTenant(app.router, http.MethodPut, "/api/v1/tenant/members/2/roles", map[string]any{"roleCodes": []string{"MISSING_ROLE"}}, adminAccess, 1)
	if invalid.Code != http.StatusNotFound || !bytesContains(invalid.Body.String(), "ROLE_NOT_FOUND") {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	platform := performJSONWithTenant(app.router, http.MethodPut, "/api/v1/tenant/members/2/roles", map[string]any{"roleCodes": []string{"PLATFORM_ADMIN"}}, adminAccess, 1)
	if platform.Code != http.StatusBadRequest || !bytesContains(platform.Body.String(), "CANNOT_ASSIGN_PLATFORM_ROLE") {
		t.Fatalf("platform status=%d body=%s member=%d", platform.Code, platform.Body.String(), memberID)
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
