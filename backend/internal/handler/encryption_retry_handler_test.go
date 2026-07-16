package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestEncryptionCancelAndRetryContract 验证取消终态和重试新执行序号通过独立接口返回。
func TestEncryptionCancelAndRetryContract(t *testing.T) {
	handler := NewEncryptionHandler(&encryptionApplicationStub{}, 1024)
	router := newEncryptionContractRouter(func(router *gin.Engine) {
		router.POST("/tasks/:taskId/cancel", handler.Cancel)
		router.POST("/tasks/:taskId/retry", handler.Retry)
	})
	cancel := httptest.NewRecorder()
	router.ServeHTTP(cancel, httptest.NewRequest(http.MethodPost, "/tasks/task/cancel", nil))
	if cancel.Code != http.StatusOK || !strings.Contains(cancel.Body.String(), "CANCELLED") {
		t.Fatalf("cancel=%d %s", cancel.Code, cancel.Body.String())
	}
	retry := httptest.NewRecorder()
	router.ServeHTTP(retry, httptest.NewRequest(http.MethodPost, "/tasks/task/retry", nil))
	if retry.Code != http.StatusCreated || !strings.Contains(retry.Body.String(), `"attempt_no":2`) {
		t.Fatalf("retry=%d %s", retry.Code, retry.Body.String())
	}
}
