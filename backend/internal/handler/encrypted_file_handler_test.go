package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestEncryptedFileHandlerListDetailAndDownload 验证分页与流式下载响应不暴露对象键。
func TestEncryptedFileHandlerListDetailAndDownload(t *testing.T) {
	handler := NewEncryptedFileHandler(encryptedFileApplicationStub{})
	router := newEncryptionContractRouter(func(router *gin.Engine) {
		router.GET("/tenant/files", handler.ListFileCenter)
		router.GET("/tenant/files/:fileId", handler.FileCenterDetail)
		router.GET("/tenant/files/:fileId/ciphertext", handler.DownloadFileCenter)
		router.GET("/tenant/files/:fileId/decryption-material", handler.OwnDecryptionMaterial)
		router.GET("/files", handler.List)
		router.GET("/files/:fileId", handler.Detail)
		router.GET("/files/:fileId/ciphertext", handler.Download)
		router.GET("/received-files", handler.ListReceived)
		router.GET("/received-files/:fileId/decryption-material", handler.ReceivedMaterial)
		router.GET("/received-files/:fileId/ciphertext", handler.DownloadReceived)
	})
	for _, path := range []string{"/tenant/files?scope=tenant_cloud", "/tenant/files/file"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "object_key") || strings.Contains(recorder.Body.String(), "protected_key_bytes") {
			t.Fatalf("unsafe file center response %s: %d %s", path, recorder.Code, recorder.Body.String())
		}
	}
	centerDownload := httptest.NewRecorder()
	router.ServeHTTP(centerDownload, httptest.NewRequest(http.MethodGet, "/tenant/files/file/ciphertext", nil))
	if centerDownload.Code != http.StatusOK || centerDownload.Body.String() != "cipher" || centerDownload.Header().Get("X-Ciphertext-SHA256") == "" {
		t.Fatalf("file center download=%d headers=%v body=%s", centerDownload.Code, centerDownload.Header(), centerDownload.Body.String())
	}
	centerMaterial := httptest.NewRecorder()
	router.ServeHTTP(centerMaterial, httptest.NewRequest(http.MethodGet, "/tenant/files/file/decryption-material", nil))
	if centerMaterial.Code != http.StatusOK || !strings.Contains(centerMaterial.Body.String(), "protected_key_base64") || strings.Contains(centerMaterial.Body.String(), "private_key") {
		t.Fatalf("file center material=%d body=%s", centerMaterial.Code, centerMaterial.Body.String())
	}
	for _, path := range []string{"/files", "/files/file"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "object_key") || strings.Contains(recorder.Body.String(), "protected_key_bytes") {
			t.Fatalf("unsafe response %s: %d %s", path, recorder.Code, recorder.Body.String())
		}
	}
	download := httptest.NewRecorder()
	router.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/files/file/ciphertext", nil))
	if download.Code != http.StatusOK || download.Body.String() != "cipher" || download.Header().Get("X-Ciphertext-SHA256") == "" || !strings.Contains(download.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("download response=%d headers=%v body=%s", download.Code, download.Header(), download.Body.String())
	}
	material := httptest.NewRecorder()
	router.ServeHTTP(material, httptest.NewRequest(http.MethodGet, "/received-files/file/decryption-material", nil))
	if material.Code != http.StatusOK || strings.Contains(material.Body.String(), "private_key") || !strings.Contains(material.Body.String(), "protected_key_base64") {
		t.Fatalf("unsafe material response=%d body=%s", material.Code, material.Body.String())
	}
	received := httptest.NewRecorder()
	router.ServeHTTP(received, httptest.NewRequest(http.MethodGet, "/received-files/file/ciphertext", nil))
	if received.Code != http.StatusOK || received.Body.String() != "cipher" {
		t.Fatalf("received download=%d body=%s", received.Code, received.Body.String())
	}
}
