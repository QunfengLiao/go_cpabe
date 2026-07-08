package handler

import (
	"net/http"
	"testing"
)

// TestLogoutEndpointInvalidatesRefreshToken 验证退出登录接口会删除刷新会话。
func TestLogoutEndpointInvalidatesRefreshToken(t *testing.T) {
	app := newTestApp()
	_, refresh := registerAndLogin(t, app)
	w := performJSON(app.router, http.MethodPost, "/api/v1/auth/logout", map[string]any{"refresh_token": refresh}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	again := performJSON(app.router, http.MethodPost, "/api/v1/auth/refresh", map[string]any{"refresh_token": refresh}, "")
	if again.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout status=%d body=%s", again.Code, again.Body.String())
	}
}
