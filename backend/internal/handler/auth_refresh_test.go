package handler

import (
	"net/http"
	"testing"
)

// TestRefreshEndpointRotatesToken 验证刷新接口会轮换 refresh token 并使旧 token 失效。
func TestRefreshEndpointRotatesToken(t *testing.T) {
	app := newTestApp()
	_, refresh := registerAndLogin(t, app)
	w := performJSON(app.router, http.MethodPost, "/api/v1/auth/refresh", map[string]any{"refresh_token": refresh}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	data := parseData(t, w)
	newRefresh := data["refresh_token"].(string)
	if newRefresh == "" || newRefresh == refresh {
		t.Fatalf("expected rotated refresh token")
	}
	old := performJSON(app.router, http.MethodPost, "/api/v1/auth/refresh", map[string]any{"refresh_token": refresh}, "")
	if old.Code != http.StatusUnauthorized {
		t.Fatalf("old refresh status=%d body=%s", old.Code, old.Body.String())
	}
}
