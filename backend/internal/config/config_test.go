package config

import (
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("APP_PORT", "")
	t.Setenv("MYSQL_PORT", "")
	t.Setenv("REDIS_DB", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.App.Env != defaultAppEnv {
		t.Fatalf("App.Env = %q, want %q", cfg.App.Env, defaultAppEnv)
	}
	if cfg.App.Port != defaultAppPort {
		t.Fatalf("App.Port = %d, want %d", cfg.App.Port, defaultAppPort)
	}
	if cfg.MySQL.Port != 3306 {
		t.Fatalf("MySQL.Port = %d, want 3306", cfg.MySQL.Port)
	}
	if cfg.Redis.DB != defaultRedisDB {
		t.Fatalf("Redis.DB = %d, want %d", cfg.Redis.DB, defaultRedisDB)
	}
}

func TestLoadRejectsInvalidAppPort(t *testing.T) {
	t.Setenv("APP_PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid APP_PORT error")
	}
}

func TestLoadRejectsInvalidRedisDB(t *testing.T) {
	t.Setenv("REDIS_DB", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid REDIS_DB error")
	}
}
