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
	TenantService *service.TenantService
	AuthManager   *auth.Manager
	HealthService *service.HealthService
	MaxAvatarSize int64
}

func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.CORS())

	authHandler := NewAuthHandler(deps.AuthService)
	userHandler := NewUserHandler(deps.UserService, deps.MaxAvatarSize)
	tenantHandler := NewTenantHandler(deps.TenantService)

	if deps.HealthService != nil {
		healthHandler := NewHealthHandler(deps.HealthService)
		router.GET("/health", healthHandler.Get)
	}

	api := router.Group("/api/v1")
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/refresh", authHandler.Refresh)
	api.POST("/auth/logout", authHandler.Logout)

	me := api.Group("/me", middleware.AuthRequired(deps.AuthManager))
	me.GET("/tenants", tenantHandler.MyTenants)
	me.POST("/switch-tenant", tenantHandler.SwitchTenant)

	tenants := api.Group("/tenants", middleware.AuthRequired(deps.AuthManager))
	tenants.POST("", tenantHandler.CreateTenant)
	tenants.GET("", tenantHandler.ListTenants)
	tenants.GET("/:id", tenantHandler.TenantDetail)
	tenants.PATCH("/:id/enable", tenantHandler.EnableTenant)
	tenants.PATCH("/:id/disable", tenantHandler.DisableTenant)
	tenants.POST("/:id/users", tenantHandler.AddTenantUser)
	tenants.DELETE("/:id/users/:userId", tenantHandler.RemoveTenantUser)
	tenants.GET("/:id/users", tenantHandler.ListTenantUsers)

	protected := api.Group("/users", middleware.AuthRequired(deps.AuthManager))
	protected.GET("/me", userHandler.Me)
	protected.PUT("/me", userHandler.UpdateMe)
	protected.POST("/me/avatar", userHandler.UploadAvatar)

	return router
}
