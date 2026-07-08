package handler

import (
	"context"
	"net/http"
	"testing"

	"go-cpabe/backend/internal/domain"
)

// TestLoginEndpoint 验证登录接口返回 token、用户信息和租户上下文。
func TestLoginEndpoint(t *testing.T) {
	app := newTestApp()
	performJSON(app.router, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": "user@example.com", "password": "Passw0rd!", "confirm_password": "Passw0rd!", "nickname": "用户", "role": "data_user",
	}, "")
	w := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "user@example.com", "password": "Passw0rd!",
	}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w)
	if data["access_token"] == "" || data["refresh_token"] == "" || data["token_type"] != "Bearer" {
		t.Fatalf("unexpected login data: %+v", data)
	}
	if data["current_tenant_id"] == nil || data["tenants"] == nil {
		t.Fatalf("missing tenant context: %+v", data)
	}
	if data["current_tenant_code"] == nil {
		t.Fatalf("missing tenant code: %+v", data)
	}
	if _, err := app.tenantRepo.EnsureTenant(context.Background(), &domain.Tenant{Name: "深信服科技", Code: "sangfor", Status: domain.TenantStatusEnabled}); err != nil {
		t.Fatalf("ensure sangfor tenant: %v", err)
	}
	forbidden := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "user@example.com", "password": "Passw0rd!", "tenantCode": "sangfor",
	}, "")
	if forbidden.Code != http.StatusForbidden || !bytesContains(forbidden.Body.String(), "TENANT_MEMBER_FORBIDDEN") {
		t.Fatalf("forbidden status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
	defaultTenant := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "user@example.com", "password": "Passw0rd!", "tenantCode": domain.DefaultTenantCode,
	}, "")
	if defaultTenant.Code != http.StatusOK {
		t.Fatalf("default tenant status=%d body=%s", defaultTenant.Code, defaultTenant.Body.String())
	}
	bad := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "user@example.com", "password": "wrong",
	}, "")
	if bad.Code != http.StatusUnauthorized || !bytesContains(bad.Body.String(), "INVALID_CREDENTIALS") {
		t.Fatalf("bad status=%d body=%s", bad.Code, bad.Body.String())
	}
}
