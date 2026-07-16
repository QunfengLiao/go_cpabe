package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestEncryptionHandlerCreateAndUploadContract 验证任务 UUID 响应和受限密文上传契约。
func TestEncryptionHandlerCreateAndUploadContract(t *testing.T) {
	application := &encryptionApplicationStub{}
	handler := NewEncryptionHandler(application, 1024)
	router := newEncryptionContractRouter(func(router *gin.Engine) {
		router.POST("/tasks", handler.CreateTask)
		router.PUT("/tasks/:taskId/attempts/:attemptId/ciphertext", handler.UploadCiphertext)
	})
	request := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"file":{"name":"demo.txt","size":6},"algorithm":{"code":"RSA-OAEP-SHA256","version":"1"},"authorization":{"type":"RSA_RECIPIENTS","recipients":[{"user_id":7,"public_key_id":"owner-key"},{"user_id":9,"public_key_id":"623e4567-e89b-42d3-a456-426614174000"}]}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "1234567890abcdef")
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusCreated || !strings.Contains(responseRecorder.Body.String(), `"file_id":"523e4567`) || strings.Contains(responseRecorder.Body.String(), `"file_id":92`) {
		t.Fatalf("create response=%d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if application.lastCreate.Authorization["type"] != "RSA_RECIPIENTS" {
		t.Fatalf("multi-recipient authorization was not bound: %+v", application.lastCreate.Authorization)
	}
	if recipients, ok := application.lastCreate.Authorization["recipients"].([]any); !ok || len(recipients) != 2 {
		t.Fatalf("recipient array was not bound: %+v", application.lastCreate.Authorization)
	}
	upload := httptest.NewRequest(http.MethodPut, "/tasks/task/attempts/attempt/ciphertext", bytes.NewReader([]byte("cipher")))
	upload.ContentLength = 6
	upload.Header.Set("X-Ciphertext-SHA256", strings.Repeat("a", 64))
	upload.Header.Set("X-Ciphertext-Format", "GCPABE01")
	uploadRecorder := httptest.NewRecorder()
	router.ServeHTTP(uploadRecorder, upload)
	if uploadRecorder.Code != http.StatusCreated || string(application.lastUpload) != "cipher" {
		t.Fatalf("upload response=%d %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
}
