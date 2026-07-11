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
	"go-cpabe/backend/internal/service"
)

// main 提供本地受控管理员初始化命令，避免通过公开注册接口创建高权限账号。
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
	case "create-platform":
		if err := createPlatformAdmin(os.Args[2:]); err != nil {
			log.Fatalf("创建平台管理员失败: %v", err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

// createAdmin 创建旧单租户管理员账号，并写入默认租户兼容授权。
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
	repo := repository.NewGormUserRepository(db)
	tenantRepo := repository.NewGormTenantRepository(db)
	tenantSvc := service.NewTenantService(tenantRepo, repo)
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
	if err := tenantSvc.EnsureUserInDefaultTenant(ctx, user.ID, user.Role); err != nil {
		return fmt.Errorf("绑定默认租户管理员: %w", err)
	}

	log.Printf("管理员已创建: email=%s id=%d", user.Email, user.ID)
	return nil
}

// createPlatformAdmin 创建或提升平台管理员账号，副作用是写入 users 和 user_roles。
func createPlatformAdmin(args []string) error {
	fs := flag.NewFlagSet("create-platform", flag.ContinueOnError)
	email := fs.String("email", "", "平台管理员邮箱")
	password := fs.String("password", "", "平台管理员密码；用户不存在时必填，更推荐使用 ADMIN_PASSWORD 环境变量")
	nickname := fs.String("nickname", "平台管理员", "平台管理员昵称")
	if err := fs.Parse(args); err != nil {
		return err
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(*email))
	adminPassword := *password
	if adminPassword == "" {
		adminPassword = os.Getenv("ADMIN_PASSWORD")
	}
	if normalizedEmail == "" {
		return errors.New("必须提供 -email")
	}
	if !validator.ValidEmail(normalizedEmail) {
		return errors.New("平台管理员邮箱格式不合法")
	}
	if !validator.ValidNickname(*nickname) {
		return errors.New("平台管理员昵称长度必须为 1 到 20 个字符")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("加载配置: %w", err)
	}
	db, err := config.OpenDatabase(cfg)
	if err != nil {
		return fmt.Errorf("连接数据库: %w", err)
	}
	userRepo := repository.NewGormUserRepository(db)
	tenantRepo := repository.NewGormTenantRepository(db)
	tenantSvc := service.NewTenantService(tenantRepo, userRepo)
	platformRoleSvc := service.NewPlatformRoleService(tenantRepo, userRepo, service.NoopAuditRecorder{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tenantSvc.EnsureBaseRoles(ctx); err != nil {
		return fmt.Errorf("初始化基础角色: %w", err)
	}

	user, err := userRepo.FindByEmail(ctx, normalizedEmail)
	if err != nil {
		if !errors.Is(err, repository.ErrUserNotFound) {
			return fmt.Errorf("检查邮箱是否存在: %w", err)
		}
		if adminPassword == "" {
			return errors.New("用户不存在时必须通过 ADMIN_PASSWORD 或 -password 提供密码")
		}
		passwordHash, err := auth.HashPassword(adminPassword)
		if err != nil {
			return fmt.Errorf("生成密码哈希: %w", err)
		}
		// 平台管理员通过本地受控命令创建；公开注册仍不允许创建任何管理员身份。
		user = &domain.User{
			Email:        normalizedEmail,
			PasswordHash: passwordHash,
			Nickname:     strings.TrimSpace(*nickname),
			Role:         domain.RoleAdmin,
			Status:       domain.StatusActive,
		}
		if err := userRepo.Create(ctx, user); err != nil {
			return fmt.Errorf("写入平台管理员用户: %w", err)
		}
	}

	if err := platformRoleSvc.EnsurePlatformAdmin(ctx, user.ID); err != nil {
		return fmt.Errorf("分配平台管理员角色: %w", err)
	}
	log.Printf("平台管理员已就绪: email=%s id=%d", user.Email, user.ID)
	return nil
}

// usage 输出本地管理员命令的用法说明，供命令参数错误时提示操作者。
func usage() {
	fmt.Fprintln(os.Stderr, "用法:")
	fmt.Fprintln(os.Stderr, "  ADMIN_PASSWORD='Admin@123456' go run ./cmd/admin create -email admin@example.com -nickname 管理员")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/admin create -email admin@example.com -password Admin@123456 -nickname 管理员")
	fmt.Fprintln(os.Stderr, "  ADMIN_PASSWORD='Admin@123456' go run ./cmd/admin create-platform -email platform@example.com -nickname 平台管理员")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/admin create-platform -email platform@example.com -password Admin@123456 -nickname 平台管理员")
}
