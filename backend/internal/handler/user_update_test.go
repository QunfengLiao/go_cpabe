package handler

import (
	"net/http"
	"testing"
)

func TestUpdateMeEndpoint(t *testing.T) {
	app := newTestApp()
	access, _ := registerAndLogin(t, app)
	w := performJSON(app.router, http.MethodPut, "/api/v1/users/me", map[string]any{
		"nickname": "新昵称", "bio": "新的个人简介", "birthday": "1998-01-01", "role": "admin",
	}, access)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !bytesContains(w.Body.String(), "新昵称") || bytesContains(w.Body.String(), "admin") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	bad := performJSON(app.router, http.MethodPut, "/api/v1/users/me", map[string]any{
		"nickname": "", "bio": "", "birthday": "not-a-date",
	}, access)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad status=%d body=%s", bad.Code, bad.Body.String())
	}
}
