package handler

import (
	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/service"
)

type Dependencies struct {
	AuthService   *service.AuthService
	UserService   *service.UserService
	AuthManager   *auth.Manager
	MaxAvatarSize int64
}

func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.CORS())

	authHandler := NewAuthHandler(deps.AuthService)
	userHandler := NewUserHandler(deps.UserService, deps.MaxAvatarSize)

	api := router.Group("/api/v1")
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/refresh", authHandler.Refresh)
	api.POST("/auth/logout", authHandler.Logout)

	protected := api.Group("/users", middleware.AuthRequired(deps.AuthManager))
	protected.GET("/me", userHandler.Me)
	protected.PUT("/me", userHandler.UpdateMe)
	protected.POST("/me/avatar", userHandler.UploadAvatar)

	return router
}
