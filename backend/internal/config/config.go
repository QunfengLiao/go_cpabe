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

// Config 汇总后端运行所需的数据库、Redis、JWT、HTTP、头像和加密文件配置。
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
	// EncryptedFileStorageDir 是服务端密文正式对象根目录，不得注册为静态资源目录。
	EncryptedFileStorageDir string
	// EncryptedFileTempDir 是上传暂存对象目录，失败和过期对象由清理流程回收。
	EncryptedFileTempDir string
	// EncryptedFileMaxSize 是单个明文文件允许的最大字节数，服务端据此限制密文请求体。
	EncryptedFileMaxSize int64
	// EncryptionMaxConcurrentPerTenant 是单租户允许的并发加密执行数。
	EncryptionMaxConcurrentPerTenant int
	// EncryptionStagingTTL 是暂存密文和异常租约的最长存活时间。
	EncryptionStagingTTL time.Duration
	// AuditDispatchBatchSize 限制单次审计 Dispatcher 领取数量，避免长事务占用过多连接。
	AuditDispatchBatchSize int
	// AuditDispatchLease 是单批事件的处理租约，进程崩溃后允许其他实例安全接管。
	AuditDispatchLease time.Duration
	// AuditDispatchMaxRetries 是事件进入死信前允许的最大失败次数。
	AuditDispatchMaxRetries int
	// AuditDispatchBaseBackoff 是首次可重试失败的退避基数。
	AuditDispatchBaseBackoff time.Duration
	// AuditDispatchMaxBackoff 限制指数退避上限，避免故障事件长期不可见。
	AuditDispatchMaxBackoff time.Duration
	// AuditDeliveredRetention 是已投递 outbox 记录的保留时间，死信不受该配置自动删除。
	AuditDeliveredRetention time.Duration
	RunAutoMigrate          bool
	RunSeed                 bool
	SeedDemoData            bool
}

// Load 从环境变量和可选 .env 文件中加载运行配置，并校验必要的密钥和数据库连接信息。
func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:                  getenv("APP_ENV", "development"),
		MySQLDSN:                mysqlDSNFromEnv(),
		RedisAddr:               getenv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:           getenv("REDIS_PASSWORD", ""),
		ServerAddr:              serverAddrFromEnv(),
		AvatarUploadDir:         getenv("AVATAR_UPLOAD_DIR", "uploads/avatars"),
		AvatarURLPrefix:         getenv("AVATAR_URL_PREFIX", "/uploads/avatars"),
		EncryptedFileStorageDir: getenv("ENCRYPTED_FILE_STORAGE_DIR", "uploads/ciphertexts"),
		EncryptedFileTempDir:    getenv("ENCRYPTED_FILE_TEMP_DIR", "uploads/ciphertexts/.staging"),
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
	if cfg.AccessTokenTTL < time.Minute {
		return Config{}, fmt.Errorf("ACCESS_TOKEN_TTL must be at least 1m, got %s", cfg.AccessTokenTTL)
	}
	cfg.RefreshTokenTTL, err = getenvDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cfg.AvatarMaxSize, err = getenvInt64("AVATAR_MAX_SIZE", 2*1024*1024)
	if err != nil {
		return Config{}, err
	}
	cfg.EncryptedFileMaxSize, err = getenvInt64("ENCRYPTED_FILE_MAX_SIZE", 1024*1024*1024)
	if err != nil {
		return Config{}, err
	}
	if cfg.EncryptedFileMaxSize <= 0 {
		return Config{}, errors.New("ENCRYPTED_FILE_MAX_SIZE must be positive")
	}
	cfg.EncryptionMaxConcurrentPerTenant, err = getenvInt("ENCRYPTION_MAX_CONCURRENT_PER_TENANT", 3)
	if err != nil {
		return Config{}, err
	}
	if cfg.EncryptionMaxConcurrentPerTenant <= 0 {
		return Config{}, errors.New("ENCRYPTION_MAX_CONCURRENT_PER_TENANT must be positive")
	}
	cfg.EncryptionStagingTTL, err = getenvDuration("ENCRYPTION_STAGING_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	if cfg.EncryptionStagingTTL < time.Minute {
		return Config{}, errors.New("ENCRYPTION_STAGING_TTL must be at least 1m")
	}
	cfg.AuditDispatchBatchSize, err = getenvInt("AUDIT_DISPATCH_BATCH_SIZE", 100)
	if err != nil || cfg.AuditDispatchBatchSize <= 0 || cfg.AuditDispatchBatchSize > 1000 {
		return Config{}, errors.New("AUDIT_DISPATCH_BATCH_SIZE must be between 1 and 1000")
	}
	cfg.AuditDispatchLease, err = getenvDuration("AUDIT_DISPATCH_LEASE", time.Minute)
	if err != nil || cfg.AuditDispatchLease < time.Second {
		return Config{}, errors.New("AUDIT_DISPATCH_LEASE must be at least 1s")
	}
	cfg.AuditDispatchMaxRetries, err = getenvInt("AUDIT_DISPATCH_MAX_RETRIES", 10)
	if err != nil || cfg.AuditDispatchMaxRetries <= 0 || cfg.AuditDispatchMaxRetries > 100 {
		return Config{}, errors.New("AUDIT_DISPATCH_MAX_RETRIES must be between 1 and 100")
	}
	cfg.AuditDispatchBaseBackoff, err = getenvDuration("AUDIT_DISPATCH_BASE_BACKOFF", 30*time.Second)
	if err != nil || cfg.AuditDispatchBaseBackoff < time.Second {
		return Config{}, errors.New("AUDIT_DISPATCH_BASE_BACKOFF must be at least 1s")
	}
	cfg.AuditDispatchMaxBackoff, err = getenvDuration("AUDIT_DISPATCH_MAX_BACKOFF", time.Hour)
	if err != nil || cfg.AuditDispatchMaxBackoff < cfg.AuditDispatchBaseBackoff {
		return Config{}, errors.New("AUDIT_DISPATCH_MAX_BACKOFF must be at least AUDIT_DISPATCH_BASE_BACKOFF")
	}
	cfg.AuditDeliveredRetention, err = getenvDuration("AUDIT_DELIVERED_RETENTION", 7*24*time.Hour)
	if err != nil || cfg.AuditDeliveredRetention < time.Hour {
		return Config{}, errors.New("AUDIT_DELIVERED_RETENTION must be at least 1h")
	}
	cfg.RunAutoMigrate, err = getenvBool("RUN_AUTO_MIGRATE", false)
	if err != nil {
		return Config{}, err
	}
	cfg.RunSeed, err = getenvBool("RUN_SEED", false)
	if err != nil {
		return Config{}, err
	}
	cfg.SeedDemoData, err = getenvBool("RUN_DEMO_SEED", false)
	if err != nil {
		return Config{}, err
	}
	if legacyDemoSeed := os.Getenv("SEED_DEMO_DATA"); legacyDemoSeed != "" {
		cfg.SeedDemoData, err = strconv.ParseBool(legacyDemoSeed)
		if err != nil {
			return Config{}, err
		}
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

// getenvBool 读取布尔环境变量，未设置时返回默认值。
func getenvBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseBool(value)
}

// getenvDuration 读取 Go duration 格式的环境变量，未设置时返回默认时长。
func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}
