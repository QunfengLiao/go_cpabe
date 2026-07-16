package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestCORSAllowsTenantHeader 验证租户接口预检允许 X-Tenant-Id。
//
// 前端所有当前租户接口都会携带该头；如果 CORS 预检不放行，浏览器会在到达鉴权逻辑前拦截请求，
// 导致授权上下文加载失败，租户菜单被权限过滤清空。
func TestCORSAllowsTenantHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS())
	router.GET("/api/v1/tenant/me/authorization", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/tenant/me/authorization", nil)
	request.Header.Set("Origin", "http://127.0.0.1:5173")
	request.Header.Set("Access-Control-Request-Method", "GET")
	request.Header.Set("Access-Control-Request-Headers", "authorization,x-tenant-id")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
	allowHeaders := response.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(strings.ToLower(allowHeaders), "x-tenant-id") {
		t.Fatalf("missing X-Tenant-Id in allow headers: %q", allowHeaders)
	}
}

// TestCORSAllowsEncryptionHeaders 验证 RSA 登记幂等头可通过预检，并允许前端读取密文摘要。
func TestCORSAllowsEncryptionHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS())
	router.POST("/api/v1/tenant/me/rsa-public-keys", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/tenant/me/rsa-public-keys", nil)
	request.Header.Set("Origin", "http://127.0.0.1:5173")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "authorization,content-type,x-tenant-id,idempotency-key")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
	allowHeaders := strings.ToLower(response.Header().Get("Access-Control-Allow-Headers"))
	if !strings.Contains(allowHeaders, "idempotency-key") {
		t.Fatalf("missing Idempotency-Key in allow headers: %q", allowHeaders)
	}
	exposedHeaders := strings.ToLower(response.Header().Get("Access-Control-Expose-Headers"))
	if !strings.Contains(exposedHeaders, "x-ciphertext-sha256") {
		t.Fatalf("missing X-Ciphertext-SHA256 in expose headers: %q", exposedHeaders)
	}
}
