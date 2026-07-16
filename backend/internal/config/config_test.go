package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoadFindsDotEnvUpwards 验证配置加载会从子目录向上查找 .env 文件。
func TestLoadFindsDotEnvUpwards(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "backend", "cmd", "server")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	env := []byte("JWT_SECRET=test-secret\nMYSQL_HOST=127.0.0.1\nMYSQL_PORT=3306\nMYSQL_USER=root\nMYSQL_PASSWORD=pw\nMYSQL_DATABASE=go_cpabe\nAPP_PORT=18080\nAPP_ENV=test\n")
	if err := os.WriteFile(filepath.Join(root, ".env"), env, 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	for _, key := range []string{"JWT_SECRET", "MYSQL_DSN", "MYSQL_HOST", "MYSQL_PORT", "MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_DATABASE", "APP_PORT", "SERVER_ADDR", "APP_ENV"} {
		old, existed := os.LookupEnv(key)
		_ = os.Unsetenv(key)
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.JWTSecret != "test-secret" {
		t.Fatalf("unexpected jwt secret: %q", cfg.JWTSecret)
	}
	if cfg.MySQLDSN == "" {
		t.Fatal("expected mysql dsn composed from MYSQL_* variables")
	}
	if cfg.ServerAddr != ":18080" {
		t.Fatalf("unexpected server addr: %q", cfg.ServerAddr)
	}
	if cfg.AppEnv != "test" {
		t.Fatalf("unexpected app env: %q", cfg.AppEnv)
	}
}

// TestLoadRejectsInvalidRedisDB 验证 Redis DB 配置格式错误会阻止配置加载。
func TestLoadRejectsInvalidRedisDB(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("MYSQL_DSN", "root:pw@tcp(127.0.0.1:3306)/go_cpabe")
	t.Setenv("REDIS_DB", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid REDIS_DB error")
	}
}

// TestLoadParsesAuditDispatcherConfig 验证审计 Dispatcher 的批量、租约、退避和保留期配置均按显式环境变量解析，
// 以便运维调整吞吐时不会改变死信或租约安全边界。
func TestLoadParsesAuditDispatcherConfig(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("MYSQL_DSN", "root:pw@tcp(127.0.0.1:3306)/go_cpabe")
	t.Setenv("REDIS_DB", "0")
	t.Setenv("AUDIT_DISPATCH_BATCH_SIZE", "250")
	t.Setenv("AUDIT_DISPATCH_LEASE", "45s")
	t.Setenv("AUDIT_DISPATCH_MAX_RETRIES", "12")
	t.Setenv("AUDIT_DISPATCH_BASE_BACKOFF", "5s")
	t.Setenv("AUDIT_DISPATCH_MAX_BACKOFF", "15m")
	t.Setenv("AUDIT_DELIVERED_RETENTION", "336h")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AuditDispatchBatchSize != 250 {
		t.Fatalf("AuditDispatchBatchSize = %d, want 250", cfg.AuditDispatchBatchSize)
	}
	if cfg.AuditDispatchLease != 45*time.Second {
		t.Fatalf("AuditDispatchLease = %s, want 45s", cfg.AuditDispatchLease)
	}
	if cfg.AuditDispatchMaxRetries != 12 {
		t.Fatalf("AuditDispatchMaxRetries = %d, want 12", cfg.AuditDispatchMaxRetries)
	}
	if cfg.AuditDispatchBaseBackoff != 5*time.Second {
		t.Fatalf("AuditDispatchBaseBackoff = %s, want 5s", cfg.AuditDispatchBaseBackoff)
	}
	if cfg.AuditDispatchMaxBackoff != 15*time.Minute {
		t.Fatalf("AuditDispatchMaxBackoff = %s, want 15m", cfg.AuditDispatchMaxBackoff)
	}
	if cfg.AuditDeliveredRetention != 14*24*time.Hour {
		t.Fatalf("AuditDeliveredRetention = %s, want 336h", cfg.AuditDeliveredRetention)
	}
}

// TestLoadRejectsUnsafeAuditDispatcherConfig 验证不安全的领取范围、短租约、重试上限、退避关系和过短保留期会阻止启动；
// 每个用例都先设置其余合法值，确保错误确实来自当前被测安全边界。
func TestLoadRejectsUnsafeAuditDispatcherConfig(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		wantError string
	}{
		{name: "批量为零", key: "AUDIT_DISPATCH_BATCH_SIZE", value: "0", wantError: "AUDIT_DISPATCH_BATCH_SIZE"},
		{name: "批量超过上限", key: "AUDIT_DISPATCH_BATCH_SIZE", value: "1001", wantError: "AUDIT_DISPATCH_BATCH_SIZE"},
		{name: "租约过短", key: "AUDIT_DISPATCH_LEASE", value: "500ms", wantError: "AUDIT_DISPATCH_LEASE"},
		{name: "重试次数为零", key: "AUDIT_DISPATCH_MAX_RETRIES", value: "0", wantError: "AUDIT_DISPATCH_MAX_RETRIES"},
		{name: "基础退避过短", key: "AUDIT_DISPATCH_BASE_BACKOFF", value: "500ms", wantError: "AUDIT_DISPATCH_BASE_BACKOFF"},
		{name: "最大退避小于基础值", key: "AUDIT_DISPATCH_MAX_BACKOFF", value: "2s", wantError: "AUDIT_DISPATCH_MAX_BACKOFF"},
		{name: "已投递保留期过短", key: "AUDIT_DELIVERED_RETENTION", value: "30m", wantError: "AUDIT_DELIVERED_RETENTION"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "test-secret")
			t.Setenv("MYSQL_DSN", "root:pw@tcp(127.0.0.1:3306)/go_cpabe")
			t.Setenv("REDIS_DB", "0")
			t.Setenv("AUDIT_DISPATCH_BATCH_SIZE", "100")
			t.Setenv("AUDIT_DISPATCH_LEASE", "1m")
			t.Setenv("AUDIT_DISPATCH_MAX_RETRIES", "10")
			t.Setenv("AUDIT_DISPATCH_BASE_BACKOFF", "30s")
			t.Setenv("AUDIT_DISPATCH_MAX_BACKOFF", "1h")
			t.Setenv("AUDIT_DELIVERED_RETENTION", "168h")
			t.Setenv(test.key, test.value)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Load() error = %v, want error containing %q", err, test.wantError)
			}
		})
	}
}
