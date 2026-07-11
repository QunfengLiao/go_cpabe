package handler

import (
	"net/http"
	"testing"
)

// TestMyTenantsEndpoint 验证当前用户租户列表接口返回默认租户上下文。
func TestMyTenantsEndpoint(t *testing.T) {
	app := newTestApp()
	access, _ := registerAndLogin(t, app)

	w := performJSON(app.router, http.MethodGet, "/api/v1/me/tenants", nil, access)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w)
	if data["tenants"] == nil || data["current_tenant_id"] == nil {
		t.Fatalf("unexpected tenant data: %+v", data)
	}
	if data["platform_roles"] == nil {
		t.Fatalf("tenant context must include platform_roles: %+v", data)
	}
	if data["user"] == nil {
		t.Fatalf("tenant context must include user: %+v", data)
	}

	context := performJSON(app.router, http.MethodGet, "/api/v1/me/context", nil, access)
	if context.Code != http.StatusOK {
		t.Fatalf("context status=%d body=%s", context.Code, context.Body.String())
	}

	noLogin := performJSON(app.router, http.MethodGet, "/api/v1/me/tenants", nil, "")
	if noLogin.Code != http.StatusUnauthorized {
		t.Fatalf("no login status=%d body=%s", noLogin.Code, noLogin.Body.String())
	}
}
