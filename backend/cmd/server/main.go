package main

import (
	"context"
	"log"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/handler"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// main 装配后端服务依赖、执行开发环境迁移与基础数据补齐，并启动 Gin HTTP 服务。
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := config.OpenDatabase(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.Tenant{}, &domain.TenantUser{}, &domain.Role{}, &domain.UserRoleAssignment{}, &domain.PolicyAttribute{}, &domain.PolicyTemplate{}, &domain.AccessPolicy{}); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	redisClient := config.OpenRedis(cfg)
	userRepo := repository.NewGormUserRepository(db)
	tenantRepo := repository.NewGormTenantRepository(db)
	policyRepo := repository.NewGormPolicyRepository(db)
	authManager := auth.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	tokenStore := auth.NewRedisTokenStore(redisClient, cfg.RefreshTokenTTL)
	localStorage := storage.NewLocalStorage(cfg.AvatarUploadDir, cfg.AvatarURLPrefix)

	tenantSvc := service.NewTenantService(tenantRepo, userRepo)
	// 启动阶段补齐基础租户和角色，主要服务于演示环境与旧单租户数据迁移；
	// 受控环境后续应迁移到显式 seed/migration，避免启动路径承担大量写入。
	if err := tenantSvc.BootstrapDefaultTenant(context.Background()); err != nil {
		log.Fatalf("bootstrap tenants: %v", err)
	}
	auditRecorder := service.NoopAuditRecorder{}
	platformTenantSvc := service.NewPlatformTenantService(tenantRepo, userRepo, auditRecorder)
	platformTenantUserSvc := service.NewPlatformTenantUserService(tenantRepo, userRepo, auditRecorder)
	platformRoleSvc := service.NewPlatformRoleService(tenantRepo, userRepo, auditRecorder)
	platformDashboardSvc := service.NewPlatformDashboardService(tenantRepo, userRepo)
	policySvc := service.NewPolicyService(policyRepo, tenantRepo)
	if err := policySvc.BootstrapDemoPolicyCatalog(context.Background()); err != nil {
		log.Fatalf("bootstrap policy catalog: %v", err)
	}
	authSvc := service.NewAuthService(userRepo, authManager, tokenStore, cfg.RefreshTokenTTL, tenantSvc)
	userSvc := service.NewUserService(userRepo, localStorage)
	healthSvc := service.NewHealthService(cfg, db, nil, redisClient, nil)

	router := handler.NewRouter(handler.Dependencies{
		AuthService:               authSvc,
		UserService:               userSvc,
		TenantService:             tenantSvc,
		PlatformTenantService:     platformTenantSvc,
		PlatformTenantUserService: platformTenantUserSvc,
		PlatformRoleService:       platformRoleSvc,
		PlatformDashboardService:  platformDashboardSvc,
		PolicyService:             policySvc,
		PlatformRoleResolver:      tenantRepo,
		AuthManager:               authManager,
		HealthService:             healthSvc,
		MaxAvatarSize:             cfg.AvatarMaxSize,
	})
	router.Static(cfg.AvatarURLPrefix, cfg.AvatarUploadDir)

	if err := router.Run(cfg.ServerAddr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
