package handler

import (
	"net/http"
	"testing"

	"go-cpabe/backend/internal/domain"
)

// TestSwitchTenantEndpoint 验证租户切换接口的成功、越权和禁用租户场景。
func TestSwitchTenantEndpoint(t *testing.T) {
	app := newTestApp()
	access, _ := registerAndLogin(t, app)
	defaultTenant, err := app.tenantRepo.FindTenantByCode(nil, domain.DefaultTenantCode)
	if err != nil {
		t.Fatalf("default tenant: %v", err)
	}

	ok := performJSON(app.router, http.MethodPost, "/api/v1/me/switch-tenant", map[string]any{"tenantId": defaultTenant.ID}, access)
	if ok.Code != http.StatusOK {
		t.Fatalf("switch status=%d body=%s", ok.Code, ok.Body.String())
	}
	data := parseData(t, ok)
	if data["current_tenant_id"] == nil {
		t.Fatalf("missing current tenant: %+v", data)
	}

	other := createTenantForTest(t, app, "其他租户", "other")
	forbidden := performJSON(app.router, http.MethodPost, "/api/v1/me/switch-tenant", map[string]any{"tenant_id": other.ID}, access)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}

	app.tenantRepo.tenants[defaultTenant.ID].Status = domain.TenantStatusDisabled
	disabled := performJSON(app.router, http.MethodPost, "/api/v1/me/switch-tenant", map[string]any{"tenant_id": defaultTenant.ID}, access)
	if disabled.Code != http.StatusForbidden || !bytesContains(disabled.Body.String(), "TENANT_DISABLED") {
		t.Fatalf("disabled status=%d body=%s", disabled.Code, disabled.Body.String())
	}
}

// createTenantForTest 在测试仓储中创建租户，供租户切换测试准备数据。
func createTenantForTest(t *testing.T, app testApp, name, code string) *domain.Tenant {
	t.Helper()
	tenant := &domain.Tenant{Name: name, Code: code, Status: domain.TenantStatusEnabled}
	if err := app.tenantRepo.CreateTenant(nil, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return tenant
}
