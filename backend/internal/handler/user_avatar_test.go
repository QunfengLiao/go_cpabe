package handler

import (
	"net/http"
	"testing"
)

func TestUploadAvatarEndpoint(t *testing.T) {
	app := newTestApp()
	access, _ := registerAndLogin(t, app)
	w := performMultipart(app.router, "/api/v1/users/me/avatar", "avatar", "avatar.webp", "image/webp", []byte("image"), access)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !bytesContains(w.Body.String(), "avatar_url") {
		t.Fatalf("missing avatar_url: %s", w.Body.String())
	}
	badType := performMultipart(app.router, "/api/v1/users/me/avatar", "avatar", "avatar.gif", "image/gif", []byte("image"), access)
	if badType.Code != http.StatusBadRequest {
		t.Fatalf("bad type status=%d body=%s", badType.Code, badType.Body.String())
	}
	noLogin := performMultipart(app.router, "/api/v1/users/me/avatar", "avatar", "avatar.webp", "image/webp", []byte("image"), "")
	if noLogin.Code != http.StatusUnauthorized {
		t.Fatalf("no login status=%d body=%s", noLogin.Code, noLogin.Body.String())
	}
}
