package handler

import (
	"net/http"
	"testing"
)

// TestTenantAdminCreatesCurrentTenantMember 验证租户管理员可在可信当前租户创建 DU，并使用一次性初始密码登录。
func TestTenantAdminCreatesCurrentTenantMember(t *testing.T) {
	app := newTestApp()
	adminAccess := createAdminAndLogin(t, app)
	created := performJSONWithTenant(app.router, http.MethodPost, "/api/v1/tenant/members", map[string]any{"username": "new.du", "display_name": "新数据用户", "email": "new.du@example.com", "phone": "13800000000", "roles": []string{"DU"}}, adminAccess, 1)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	data := parseData(t, created)
	if data["temporary_password"] != "lqf999.." || data["created_user"] != true {
		t.Fatalf("unexpected response: %+v", data)
	}
	login := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "new.du@example.com", "password": "lqf999.."}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
}

// TestTenantMemberCreateRejectsInvalidRole 验证普通成员创建入口不能授予租户管理员角色。
func TestTenantMemberCreateRejectsInvalidRole(t *testing.T) {
	app := newTestApp()
	adminAccess := createAdminAndLogin(t, app)
	response := performJSONWithTenant(app.router, http.MethodPost, "/api/v1/tenant/members", map[string]any{"username": "bad.role", "display_name": "非法角色", "email": "bad.role@example.com", "roles": []string{"TENANT_ADMIN"}}, adminAccess, 1)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
