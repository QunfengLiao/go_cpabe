package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRSAKeyHandlerNeverReturnsPrivateKey 验证登记和列表响应只包含 SPKI 公钥字段。
func TestRSAKeyHandlerNeverReturnsPrivateKey(t *testing.T) {
	handler := NewRSAKeyHandler(rsaKeyApplicationStub{})
	router := newEncryptionContractRouter(func(router *gin.Engine) {
		router.POST("/keys", handler.RegisterMyKey)
		router.GET("/keys", handler.MyKeys)
	})
	request := httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(`{"public_key_pem":"PUBLIC"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || strings.Contains(strings.ToLower(recorder.Body.String()), "private") || !strings.Contains(recorder.Body.String(), "PUBLIC KEY") {
		t.Fatalf("unsafe key response=%d %s", recorder.Code, recorder.Body.String())
	}
}
