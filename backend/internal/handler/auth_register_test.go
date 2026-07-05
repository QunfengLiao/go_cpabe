package handler

import (
	"net/http"
	"testing"
)

func TestRegisterEndpoint(t *testing.T) {
	app := newTestApp()
	ok := performJSON(app.router, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": "owner@example.com", "password": "Passw0rd!", "confirm_password": "Passw0rd!", "nickname": "拥有者", "role": "data_owner",
	}, "")
	if ok.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", ok.Code, ok.Body.String())
	}
	if ok.Body.String() == "" || containsSensitive(ok.Body.String()) {
		t.Fatalf("response leaked sensitive fields: %s", ok.Body.String())
	}
	admin := performJSON(app.router, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": "admin@example.com", "password": "Passw0rd!", "confirm_password": "Passw0rd!", "nickname": "管理员", "role": "admin",
	}, "")
	if admin.Code != http.StatusForbidden {
		t.Fatalf("admin status=%d body=%s", admin.Code, admin.Body.String())
	}
	dup := performJSON(app.router, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email": "owner@example.com", "password": "Passw0rd!", "confirm_password": "Passw0rd!", "nickname": "重复", "role": "data_user",
	}, "")
	if dup.Code != http.StatusConflict {
		t.Fatalf("dup status=%d body=%s", dup.Code, dup.Body.String())
	}
}

func containsSensitive(body string) bool {
	return bytesContains(body, "password_hash") || bytesContains(body, "avatar_object_key")
}

func bytesContains(body, sub string) bool {
	for i := 0; i+len(sub) <= len(body); i++ {
		if body[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
