package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/validator"
	"go-cpabe/backend/internal/repository"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "create":
		if err := createAdmin(os.Args[2:]); err != nil {
			log.Fatalf("创建管理员失败: %v", err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func createAdmin(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	email := fs.String("email", "", "管理员邮箱")
	password := fs.String("password", "", "管理员密码；更推荐使用 ADMIN_PASSWORD 环境变量")
	nickname := fs.String("nickname", "管理员", "管理员昵称")
	if err := fs.Parse(args); err != nil {
		return err
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(*email))
	adminPassword := *password
	if adminPassword == "" {
		adminPassword = os.Getenv("ADMIN_PASSWORD")
	}
	if normalizedEmail == "" || adminPassword == "" {
		return errors.New("必须提供 -email，并通过 ADMIN_PASSWORD 或 -password 提供密码")
	}
	if !validator.ValidEmail(normalizedEmail) {
		return errors.New("管理员邮箱格式不合法")
	}
	if !validator.ValidNickname(*nickname) {
		return errors.New("管理员昵称长度必须为 1 到 20 个字符")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("加载配置: %w", err)
	}
	db, err := config.OpenDatabase(cfg)
	if err != nil {
		return fmt.Errorf("连接数据库: %w", err)
	}
	if err := db.AutoMigrate(&domain.User{}); err != nil {
		return fmt.Errorf("同步 users 表结构: %w", err)
	}

	repo := repository.NewGormUserRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := repo.FindByEmail(ctx, normalizedEmail); err == nil {
		return errors.New("该邮箱已存在，请直接使用已有账号登录或更换邮箱")
	} else if !errors.Is(err, repository.ErrUserNotFound) {
		return fmt.Errorf("检查邮箱是否存在: %w", err)
	}

	passwordHash, err := auth.HashPassword(adminPassword)
	if err != nil {
		return fmt.Errorf("生成密码哈希: %w", err)
	}

	// 管理员不能通过公开注册接口创建，只能走本地受控初始化命令，避免任何人从前端自助提升为 admin。
	user := &domain.User{
		Email:        normalizedEmail,
		PasswordHash: passwordHash,
		Nickname:     strings.TrimSpace(*nickname),
		Role:         domain.RoleAdmin,
		Status:       domain.StatusActive,
	}
	if err := repo.Create(ctx, user); err != nil {
		return fmt.Errorf("写入管理员用户: %w", err)
	}

	log.Printf("管理员已创建: email=%s id=%d", user.Email, user.ID)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "用法:")
	fmt.Fprintln(os.Stderr, "  ADMIN_PASSWORD='Admin@123456' go run ./cmd/admin create -email admin@example.com -nickname 管理员")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/admin create -email admin@example.com -password Admin@123456 -nickname 管理员")
}
