package config

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestLoadRejectsInvalidRedisDB(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("MYSQL_DSN", "root:pw@tcp(127.0.0.1:3306)/go_cpabe")
	t.Setenv("REDIS_DB", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid REDIS_DB error")
	}
}
