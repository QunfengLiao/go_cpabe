package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultAppEnv  = "development"
	defaultAppPort = 8080
	defaultRedisDB = 0
)

type Config struct {
	App   AppConfig
	MySQL MySQLConfig
	Redis RedisConfig
}

type AppConfig struct {
	Env  string
	Port int
}

type MySQLConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

func Load() (Config, error) {
	loadDotenv(".env", "../.env", "../../.env")

	appPort, err := intFromEnv("APP_PORT", defaultAppPort)
	if err != nil {
		return Config{}, err
	}

	mysqlPort, err := intFromEnv("MYSQL_PORT", 3306)
	if err != nil {
		return Config{}, err
	}

	redisDB, err := intFromEnv("REDIS_DB", defaultRedisDB)
	if err != nil {
		return Config{}, err
	}

	return Config{
		App: AppConfig{
			Env:  stringFromEnv("APP_ENV", defaultAppEnv),
			Port: appPort,
		},
		MySQL: MySQLConfig{
			Host:     os.Getenv("MYSQL_HOST"),
			Port:     mysqlPort,
			User:     os.Getenv("MYSQL_USER"),
			Password: os.Getenv("MYSQL_PASSWORD"),
			Database: os.Getenv("MYSQL_DATABASE"),
		},
		Redis: RedisConfig{
			Addr:     os.Getenv("REDIS_ADDR"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
		},
	}, nil
}

func loadDotenv(paths ...string) {
	for _, path := range paths {
		_ = godotenv.Load(path)
	}
}

func stringFromEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func intFromEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	return parsed, nil
}

func (cfg MySQLConfig) Ready() bool {
	return !isPlaceholder(cfg.Host) && !isPlaceholder(cfg.User) && cfg.Database != "" && cfg.Port > 0
}

func (cfg RedisConfig) Ready() bool {
	return !isPlaceholder(cfg.Addr) && cfg.DB >= 0
}

func isPlaceholder(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || strings.HasPrefix(trimmed, "your-")
}
