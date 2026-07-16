package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
)

// TestEncryptionProgressHandlerContract 验证合法阶段和真实字节进入服务且响应隐藏授权快照原文。
func TestEncryptionProgressHandlerContract(t *testing.T) {
	application := &encryptionApplicationStub{}
	handler := NewEncryptionHandler(application, 1024)
	router := newEncryptionContractRouter(func(router *gin.Engine) { router.POST("/tasks/:taskId/attempts/:attemptId/progress", handler.Progress) })
	request := httptest.NewRequest(http.MethodPost, "/tasks/task/attempts/attempt/progress", strings.NewReader(`{"stage":"ENCRYPTING_FILE","processed_bytes":4,"total_bytes":6}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || application.lastProgress.Stage != domain.EncryptionEncryptingFile || application.lastProgress.ProcessedBytes != 4 || strings.Contains(recorder.Body.String(), `"authorization_snapshot":`) {
		t.Fatalf("progress response=%d %s", recorder.Code, recorder.Body.String())
	}
}
