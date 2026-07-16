package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestEncryptionHandlerRejectsOversizedAndUntrustedContext 验证超大请求在进入服务前被拒绝且缺少可信租户时失败。
func TestEncryptionHandlerRejectsOversizedAndUntrustedContext(t *testing.T) {
	handler := NewEncryptionHandler(&encryptionApplicationStub{}, 16)
	router := newEncryptionContractRouter(func(router *gin.Engine) { router.PUT("/upload", handler.UploadCiphertext) })
	request := httptest.NewRequest(http.MethodPut, "/upload", bytes.NewReader(bytes.Repeat([]byte("x"), 20000)))
	request.ContentLength = 20000
	request.Header.Set("X-Ciphertext-Format", "GCPABE01")
	request.Header.Set("X-Ciphertext-SHA256", strings.Repeat("a", 64))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized response=%d %s", recorder.Code, recorder.Body.String())
	}
	untrusted := gin.New()
	untrusted.POST("/tasks", handler.CreateTask)
	missing := httptest.NewRecorder()
	untrusted.ServeHTTP(missing, httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{}`)))
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing tenant response=%d %s", missing.Code, missing.Body.String())
	}
}
