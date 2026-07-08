package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config 汇总后端运行所需的数据库、Redis、JWT、HTTP 和头像上传配置。
type Config struct {
	AppEnv          string
	MySQLDSN        string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	ServerAddr      string
	AvatarUploadDir string
	AvatarURLPrefix string
	AvatarMaxSize   int64
}

// Load 从环境变量和可选 .env 文件中加载运行配置，并校验必要的密钥和数据库连接信息。
func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:          getenv("APP_ENV", "development"),
		MySQLDSN:        mysqlDSNFromEnv(),
		RedisAddr:       getenv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:   getenv("REDIS_PASSWORD", ""),
		ServerAddr:      serverAddrFromEnv(),
		AvatarUploadDir: getenv("AVATAR_UPLOAD_DIR", "uploads/avatars"),
		AvatarURLPrefix: getenv("AVATAR_URL_PREFIX", "/uploads/avatars"),
	}
	cfg.JWTSecret = getenv("JWT_SECRET", "")
	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}
	if cfg.MySQLDSN == "" {
		return Config{}, errors.New("MYSQL_DSN is required")
	}

	var err error
	cfg.RedisDB, err = getenvInt("REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	cfg.AccessTokenTTL, err = getenvDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	cfg.RefreshTokenTTL, err = getenvDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cfg.AvatarMaxSize, err = getenvInt64("AVATAR_MAX_SIZE", 2*1024*1024)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// loadDotEnv 从当前工作目录向上查找 .env 文件并加载，找不到时允许继续使用系统环境变量。
func loadDotEnv() error {
	envPath, err := findFileUpwards(".env")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	// Go 运行时默认不会自动读取 .env。IDE 从 backend/cmd/server 启动时工作目录
	// 可能不在项目根目录，因此这里显式向上查找并加载最近的 .env 文件。
	if err := godotenv.Load(envPath); err != nil {
		return fmt.Errorf("load .env from %s: %w", envPath, err)
	}
	return nil
}

// findFileUpwards 从当前目录逐级向父目录查找指定文件，返回第一个命中的路径。
func findFileUpwards(filename string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// getenv 读取环境变量；当变量为空时返回调用方提供的默认值。
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// serverAddrFromEnv 解析服务监听地址，优先使用 SERVER_ADDR，其次兼容 APP_PORT。
func serverAddrFromEnv() string {
	if v := getenv("SERVER_ADDR", ""); v != "" {
		return v
	}
	if port := getenv("APP_PORT", ""); port != "" {
		return ":" + port
	}
	return ":8080"
}

// mysqlDSNFromEnv 解析 MySQL DSN，支持完整 MYSQL_DSN 或分散的主机、用户、密码配置。
func mysqlDSNFromEnv() string {
	if dsn := getenv("MYSQL_DSN", ""); dsn != "" {
		return dsn
	}
	host := getenv("MYSQL_HOST", "")
	user := getenv("MYSQL_USER", "")
	password := getenv("MYSQL_PASSWORD", "")
	database := getenv("MYSQL_DATABASE", "")
	if host == "" || user == "" || database == "" {
		return ""
	}
	port := getenv("MYSQL_PORT", "3306")
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, password, host, port, database)
}

// getenvInt 读取整数环境变量，未设置时返回默认值，格式错误时返回解析错误。
func getenvInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

// getenvInt64 读取 int64 环境变量，适合头像大小等容量类配置。
func getenvInt64(key string, fallback int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

// getenvDuration 读取 Go duration 格式的环境变量，未设置时返回默认时长。
func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}
