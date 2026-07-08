package handler

import (
	"net/http"
	"testing"
)

// TestMeEndpoint 验证当前用户资料接口需要 access token 并隐藏敏感字段。
func TestMeEndpoint(t *testing.T) {
	app := newTestApp()
	access, refresh := registerAndLogin(t, app)
	w := performJSON(app.router, http.MethodGet, "/api/v1/users/me", nil, access)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if containsSensitive(w.Body.String()) {
		t.Fatalf("response leaked sensitive fields: %s", w.Body.String())
	}
	missing := performJSON(app.router, http.MethodGet, "/api/v1/users/me", nil, "")
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing status=%d", missing.Code)
	}
	wrongType := performJSON(app.router, http.MethodGet, "/api/v1/users/me", nil, refresh)
	if wrongType.Code != http.StatusUnauthorized {
		t.Fatalf("refresh token access status=%d body=%s", wrongType.Code, wrongType.Body.String())
	}
}
