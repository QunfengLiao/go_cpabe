package main

import (
	"context"
	"log"
	"time"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/handler"
	"go-cpabe/backend/internal/migrations"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// main 装配后端服务依赖并启动 Gin HTTP 服务；迁移和 seed 默认由独立命令执行。
func main() {
	startedAt := time.Now()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := config.OpenDatabase(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if cfg.RunAutoMigrate {
		if err := migrations.AutoMigrate(db); err != nil {
			log.Fatalf("auto migrate: %v", err)
		}
	}

	redisClient := config.OpenRedis(cfg)
	userRepo := repository.NewGormUserRepository(db)
	tenantRepo := repository.NewGormTenantRepository(db)
	policyRepo := repository.NewGormPolicyRepository(db)
	orgAttributeRepo := repository.NewGormOrgAttributeRepository(db)
	authManager := auth.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	tokenStore := auth.NewRedisTokenStore(redisClient, cfg.RefreshTokenTTL)
	localStorage := storage.NewLocalStorage(cfg.AvatarUploadDir, cfg.AvatarURLPrefix)
	encryptedStorage := storage.NewLocalEncryptedFileStorage(cfg.EncryptedFileStorageDir, cfg.EncryptedFileTempDir)
	encryptionRepo := repository.NewGormEncryptionRepository(db)
	rsaKeyRepo := repository.NewGormRSAKeyRepository(db)
	auditRepo := repository.NewGormAuditRepository(db)

	tenantSvc := service.NewTenantService(tenantRepo, userRepo)
	authorizationSvc := service.NewAuthorizationService(tenantRepo)
	tenantRoleSvc := service.NewTenantRoleService(tenantRepo, authorizationSvc)
	tenantSvc.SetAuthorizationService(authorizationSvc)
	auditRecorder := service.NewDatabaseAuditRecorder(auditRepo)
	tenantSvc.SetAuditRecorder(auditRecorder)
	platformTenantSvc := service.NewPlatformTenantService(tenantRepo, userRepo, auditRecorder)
	platformTenantUserSvc := service.NewPlatformTenantUserService(tenantRepo, userRepo, auditRecorder)
	platformRoleSvc := service.NewPlatformRoleService(tenantRepo, userRepo, auditRecorder)
	platformDashboardSvc := service.NewPlatformDashboardService(tenantRepo, userRepo)
	policySvc := service.NewPolicyService(policyRepo, tenantRepo)
	orgAttributeSvc := service.NewOrgAttributeService(orgAttributeRepo, tenantRepo)
	orgManagementSvc := service.NewOrgManagementService(orgAttributeRepo, tenantRepo)
	policySvc.SetOrgAttributeRepository(orgAttributeRepo)
	policySvc.SetAuthorizationService(authorizationSvc)
	orgAttributeSvc.SetAuthorizationService(authorizationSvc)
	orgManagementSvc.SetAuthorizationService(authorizationSvc)
	if cfg.RunSeed {
		if err := tenantSvc.BootstrapDefaultTenant(context.Background()); err != nil {
			log.Fatalf("bootstrap tenants: %v", err)
		}
	}
	if cfg.RunSeed && cfg.SeedDemoData {
		if err := policySvc.BootstrapDemoPolicyCatalog(context.Background()); err != nil {
			log.Fatalf("bootstrap policy catalog: %v", err)
		}
		if err := orgAttributeSvc.BootstrapDemoOrgAttributes(context.Background()); err != nil {
			log.Fatalf("bootstrap tenant org attributes: %v", err)
		}
	}
	authSvc := service.NewAuthService(userRepo, authManager, tokenStore, cfg.RefreshTokenTTL, tenantSvc)
	userSvc := service.NewUserService(userRepo, localStorage)
	healthSvc := service.NewHealthService(cfg, db, nil, redisClient, nil)
	rsaKeySvc := service.NewRSAKeyService(rsaKeyRepo, auditRecorder)
	encryptionAdmission := service.NewEncryptionAdmission(redisClient, cfg.EncryptionMaxConcurrentPerTenant, 15*time.Minute)
	encryptionSvc := service.NewEncryptionService(encryptionRepo, rsaKeySvc, encryptedStorage, encryptionAdmission, auditRecorder, cfg.EncryptedFileMaxSize)
	encryptedFileSvc := service.NewEncryptedFileService(encryptionRepo, encryptedStorage, auditRecorder)

	router := handler.NewRouter(handler.Dependencies{
		AuthService:               authSvc,
		UserService:               userSvc,
		TenantService:             tenantSvc,
		PlatformTenantService:     platformTenantSvc,
		PlatformTenantUserService: platformTenantUserSvc,
		PlatformRoleService:       platformRoleSvc,
		PlatformDashboardService:  platformDashboardSvc,
		PolicyService:             policySvc,
		OrgAttributeService:       orgAttributeSvc,
		OrgManagementService:      orgManagementSvc,
		AuthorizationService:      authorizationSvc,
		TenantRoleService:         tenantRoleSvc,
		PlatformRoleResolver:      tenantRepo,
		AuthManager:               authManager,
		HealthService:             healthSvc,
		EncryptionService:         encryptionSvc,
		RSAKeyService:             rsaKeySvc,
		EncryptedFileService:      encryptedFileSvc,
		MaxAvatarSize:             cfg.AvatarMaxSize,
		MaxEncryptedFileSize:      cfg.EncryptedFileMaxSize,
	})
	router.Static(cfg.AvatarURLPrefix, cfg.AvatarUploadDir)

	log.Printf("server dependencies ready in %s (auto_migrate=%t seed=%t demo_seed=%t)", time.Since(startedAt).Round(time.Millisecond), cfg.RunAutoMigrate, cfg.RunSeed, cfg.SeedDemoData)
	if err := router.Run(cfg.ServerAddr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
