package main

import (
	"log"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/handler"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := config.OpenDatabase(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&domain.User{}); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	redisClient := config.OpenRedis(cfg)
	userRepo := repository.NewGormUserRepository(db)
	authManager := auth.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	tokenStore := auth.NewRedisTokenStore(redisClient, cfg.RefreshTokenTTL)
	localStorage := storage.NewLocalStorage(cfg.AvatarUploadDir, cfg.AvatarURLPrefix)

	authSvc := service.NewAuthService(userRepo, authManager, tokenStore, cfg.RefreshTokenTTL)
	userSvc := service.NewUserService(userRepo, localStorage)
	healthSvc := service.NewHealthService(cfg, db, nil, redisClient, nil)

	router := handler.NewRouter(handler.Dependencies{
		AuthService:   authSvc,
		UserService:   userSvc,
		AuthManager:   authManager,
		HealthService: healthSvc,
		MaxAvatarSize: cfg.AvatarMaxSize,
	})
	router.Static(cfg.AvatarURLPrefix, cfg.AvatarUploadDir)

	if err := router.Run(cfg.ServerAddr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
