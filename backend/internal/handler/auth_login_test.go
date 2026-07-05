package handler

import (
	"net/http"
	"testing"
)

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
	bad := performJSON(app.router, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "user@example.com", "password": "wrong",
	}, "")
	if bad.Code != http.StatusUnauthorized || !bytesContains(bad.Body.String(), "INVALID_CREDENTIALS") {
		t.Fatalf("bad status=%d body=%s", bad.Code, bad.Body.String())
	}
}
