package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/model"
	"go-cpabe/backend/internal/router"

	"github.com/gin-gonic/gin"
)

func TestHealthDegradedWhenDependenciesUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := router.New(router.Dependencies{
		Config:     config.Config{App: config.AppConfig{Env: "test", Port: 8080}},
		MySQLError: fmt.Errorf("mysql connection failed: password=secret"),
		RedisError: fmt.Errorf("redis connection failed: token=secret"),
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body model.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}

	if body.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", body.Status)
	}
	if body.App.Status != "ok" || body.App.Env != "test" {
		t.Fatalf("unexpected app health: %+v", body.App)
	}
	if body.MySQL.Status != "error" || body.Redis.Status != "error" {
		t.Fatalf("unexpected dependency health: mysql=%+v redis=%+v", body.MySQL, body.Redis)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("response leaked sensitive detail: %s", rec.Body.String())
	}
}

func TestHealthAllowsDesktopRendererOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := router.New(router.Dependencies{
		Config: config.Config{App: config.AppConfig{Env: "test", Port: 8080}},
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "null")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS header for desktop renderer")
	}
}
