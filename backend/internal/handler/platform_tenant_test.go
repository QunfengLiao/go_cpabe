package handler

import (
	"net/http"
	"testing"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
)

// TestPlatformTenantEndpointsRequirePlatformAdmin 验证平台租户接口必须具备平台管理员角色。
func TestPlatformTenantEndpointsRequirePlatformAdmin(t *testing.T) {
	app := newTestApp()
	tenantAdminAccess := createAdminAndLogin(t, app)

	denied := performJSON(app.router, http.MethodGet, "/api/v1/platform/tenants", nil, tenantAdminAccess)
	if denied.Code != http.StatusForbidden || !bytesContains(denied.Body.String(), "PLATFORM_PERMISSION_DENIED") {
		t.Fatalf("denied status=%d body=%s", denied.Code, denied.Body.String())
	}

	platformAccess := createPlatformAdminAndLogin(t, app)
	list := performJSON(app.router, http.MethodGet, "/api/v1/platform/tenants", nil, platformAccess)
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
}

// TestPlatformTenantLifecycle 验证平台租户创建、详情和启停生命周期。
func TestPlatformTenantLifecycle(t *testing.T) {
	app := newTestApp()
	platformAccess := createPlatformAdminAndLogin(t, app)

	created := performJSON(app.router, http.MethodPost, "/api/v1/platform/tenants", map[string]any{
		"name": "实验室 A", "code": "lab-a", "description": "密码学实验室",
	}, platformAccess)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}

	duplicate := performJSON(app.router, http.MethodPost, "/api/v1/platform/tenants", map[string]any{
		"name": "重复实验室", "code": "lab-a",
	}, platformAccess)
	if duplicate.Code != http.StatusConflict || !bytesContains(duplicate.Body.String(), "TENANT_CODE_EXISTS") {
		t.Fatalf("duplicate status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}

	invalid := performJSON(app.router, http.MethodPost, "/api/v1/platform/tenants", map[string]any{
		"name": "非法编码", "code": "Lab_A",
	}, platformAccess)
	if invalid.Code != http.StatusBadRequest || !bytesContains(invalid.Body.String(), "TENANT_CODE_INVALID") {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}

	detail := performJSON(app.router, http.MethodGet, "/api/v1/platform/tenants/5", nil, platformAccess)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}

	disabled := performJSON(app.router, http.MethodPatch, "/api/v1/platform/tenants/5/disable", nil, platformAccess)
	if disabled.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disabled.Code, disabled.Body.String())
	}
}

// TestPlatformTenantUsersAndAdmins 验证平台后台成员添加和租户管理员授予/撤销。
func TestPlatformTenantUsersAndAdmins(t *testing.T) {
	app := newTestApp()
	platformAccess := createPlatformAdminAndLogin(t, app)
	userAccess, _ := registerAndLogin(t, app)
	_ = userAccess

	add := performJSON(app.router, http.MethodPost, "/api/v1/platform/tenants/1/users", map[string]any{"user_id": 2}, platformAccess)
	if add.Code != http.StatusOK {
		t.Fatalf("add user status=%d body=%s", add.Code, add.Body.String())
	}

	assign := performJSON(app.router, http.MethodPost, "/api/v1/platform/tenants/1/admins", map[string]any{"user_id": 2}, platformAccess)
	if assign.Code != http.StatusOK {
		t.Fatalf("assign status=%d body=%s", assign.Code, assign.Body.String())
	}

	users := performJSON(app.router, http.MethodGet, "/api/v1/platform/tenants/1/users", nil, platformAccess)
	if users.Code != http.StatusOK || !bytesContains(users.Body.String(), "TENANT_ADMIN") {
		t.Fatalf("users status=%d body=%s", users.Code, users.Body.String())
	}

	removeLast := performJSON(app.router, http.MethodDelete, "/api/v1/platform/tenants/1/admins/2", nil, platformAccess)
	if removeLast.Code != http.StatusConflict || !bytesContains(removeLast.Body.String(), "TENANT_LAST_ADMIN_FORBIDDEN") {
		t.Fatalf("remove last status=%d body=%s", removeLast.Code, removeLast.Body.String())
	}
}

// TestPlatformSearchUsersForTenantAccess 验证平台管理员能按账号、邮箱或手机号搜索已有用户用于租户接入。
func TestPlatformSearchUsersForTenantAccess(t *testing.T) {
	app := newTestApp()
	platformAccess := createPlatformAdminAndLogin(t, app)
	hash, err := auth.HashPassword("Passw0rd!")
	if err != nil {
		t.Fatalf("hash user password: %v", err)
	}
	user := &domain.User{
		Username:     "alice.scnu",
		Email:        "alice.scnu@example.com",
		Phone:        "13900000000",
		Nickname:     "Alice",
		PasswordHash: hash,
		Role:         domain.RoleDataUser,
		Status:       domain.StatusActive,
	}
	if err := app.repo.Create(nil, user); err != nil {
		t.Fatalf("create searchable user: %v", err)
	}

	search := performJSON(app.router, http.MethodGet, "/api/v1/platform/users/search?q=1390000", nil, platformAccess)
	if search.Code != http.StatusOK || !bytesContains(search.Body.String(), "alice.scnu") || !bytesContains(search.Body.String(), "13900000000") {
		t.Fatalf("search status=%d body=%s", search.Code, search.Body.String())
	}
}

// TestPlatformCreateTenantAdminAccount 验证平台管理员可以代建租户管理员账号，并且新账号必须首次改密。
func TestPlatformCreateTenantAdminAccount(t *testing.T) {
	app := newTestApp()
	platformAccess := createPlatformAdminAndLogin(t, app)

	created := performJSON(app.router, http.MethodPost, "/api/v1/platform/tenants/2/admins", map[string]any{
		"username":    "sangfor_admin",
		"displayName": "深信服租户管理员",
		"email":       "sangfor_admin@example.com",
		"phone":       "13800000000",
	}, platformAccess)
	if created.Code != http.StatusOK || !bytesContains(created.Body.String(), "TENANT_ADMIN") || !bytesContains(created.Body.String(), "created_user") {
		t.Fatalf("create tenant admin status=%d body=%s", created.Code, created.Body.String())
	}

	user, err := app.repo.FindByEmail(nil, "sangfor_admin@example.com")
	if err != nil {
		t.Fatalf("find created admin: %v", err)
	}
	if user.PasswordHash == "" || user.PasswordHash == "lqf999.." {
		t.Fatalf("password must be bcrypt hash, got %q", user.PasswordHash)
	}
	if !user.MustChangePassword {
		t.Fatalf("created tenant admin should require password change")
	}
	data := parseData(t, created)
	temporaryPassword, ok := data["temporary_password"].(string)
	if !ok || temporaryPassword != "lqf999.." {
		t.Fatalf("temporary password should use default initial password, got %+v", data["temporary_password"])
	}
	tenantID := uint64(2)
	if ok, err := app.tenantRepo.HasRole(nil, user.ID, &tenantID, domain.RoleTenantAdmin); err != nil || !ok {
		t.Fatalf("tenant admin role missing ok=%v err=%v", ok, err)
	}

	login := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "sangfor_admin@example.com", "password": "lqf999..",
	}, "")
	if login.Code != http.StatusOK || !bytesContains(login.Body.String(), "TENANT_ADMIN") || !bytesContains(login.Body.String(), "must_change_password") {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	loginData := parseData(t, login)
	if loginData["current_tenant_id"] == nil {
		t.Fatalf("missing tenant context: %+v", loginData)
	}
}

// createPlatformAdminAndLogin 创建平台管理员测试用户并返回登录 access token。
func createPlatformAdminAndLogin(t *testing.T, app testApp) string {
	t.Helper()
	hash, err := auth.HashPassword("Passw0rd!")
	if err != nil {
		t.Fatalf("hash platform password: %v", err)
	}
	user := &domain.User{
		Email:        "platform@example.com",
		Nickname:     "平台管理员",
		PasswordHash: hash,
		Role:         domain.RoleAdmin,
		Status:       domain.StatusActive,
	}
	if err := app.repo.Create(nil, user); err != nil {
		t.Fatalf("create platform admin: %v", err)
	}
	if err := app.tenantRepo.EnsureUserRole(nil, nil, user.ID, domain.RolePlatformAdmin); err != nil {
		t.Fatalf("platform role: %v", err)
	}
	w := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "platform@example.com", "password": "Passw0rd!"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("platform login status=%d body=%s", w.Code, w.Body.String())
	}
	if !bytesContains(w.Body.String(), "PLATFORM_ADMIN") {
		t.Fatalf("login response missing platform role: %s", w.Body.String())
	}
	data := parseData(t, w)
	return data["access_token"].(string)
}
