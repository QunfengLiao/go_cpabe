package handler

import (
	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/service"
)

// Dependencies 汇总 HTTP 路由装配所需的 service、中间件依赖和上传限制。
type Dependencies struct {
	AuthService               *service.AuthService
	UserService               *service.UserService
	TenantService             *service.TenantService
	PlatformTenantService     *service.PlatformTenantService
	PlatformTenantUserService *service.PlatformTenantUserService
	PlatformRoleService       *service.PlatformRoleService
	PlatformDashboardService  *service.PlatformDashboardService
	PolicyService             *service.PolicyService
	PlatformRoleResolver      middleware.PlatformRoleResolver
	AuthManager               *auth.Manager
	HealthService             *service.HealthService
	MaxAvatarSize             int64
}

// NewRouter 创建 Gin 路由，注册认证、用户、租户、平台后台和健康检查接口。
func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.CORS())

	authHandler := NewAuthHandler(deps.AuthService)
	userHandler := NewUserHandler(deps.UserService, deps.MaxAvatarSize)
	tenantHandler := NewTenantHandler(deps.TenantService)
	platformHandler := NewPlatformHandler(
		deps.PlatformTenantService,
		deps.PlatformTenantUserService,
		deps.PlatformRoleService,
		deps.PlatformDashboardService,
	)
	policyHandler := NewPolicyHandler(deps.PolicyService)

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
	tenants.PUT("/:id/members/:userId/role", tenantHandler.AssignTenantMemberRole)

	tenantPolicies := api.Group("/tenants/:id", middleware.AuthRequired(deps.AuthManager), middleware.TenantRequired(deps.TenantService))
	tenantPolicies.GET("/access-policy/attributes", policyHandler.AvailableAttributes)
	tenantPolicies.GET("/access-policy/templates", policyHandler.AvailableTemplates)
	tenantPolicies.GET("/access-policies", policyHandler.ListAccessPolicies)
	tenantPolicies.POST("/access-policies", policyHandler.CreateAccessPolicy)
	tenantPolicies.GET("/access-policies/:policyId", policyHandler.AccessPolicyDetail)
	tenantPolicies.PUT("/access-policies/:policyId", policyHandler.UpdateAccessPolicy)
	tenantPolicies.DELETE("/access-policies/:policyId", policyHandler.DeleteAccessPolicy)

	platform := api.Group("/platform", middleware.AuthRequired(deps.AuthManager), middleware.PlatformAdminRequired(deps.PlatformRoleResolver))
	platform.GET("/dashboard", platformHandler.Dashboard)
	platform.GET("/tenants", platformHandler.ListTenants)
	platform.POST("/tenants", platformHandler.CreateTenant)
	platform.GET("/tenants/:id", platformHandler.TenantDetail)
	platform.PATCH("/tenants/:id/enable", platformHandler.EnableTenant)
	platform.PATCH("/tenants/:id/disable", platformHandler.DisableTenant)
	platform.GET("/tenants/:id/users", platformHandler.ListTenantUsers)
	platform.POST("/tenants/:id/users", platformHandler.AddTenantUser)
	platform.DELETE("/tenants/:id/users/:userId", platformHandler.RemoveTenantUser)
	platform.POST("/tenants/:id/admins", platformHandler.AssignTenantAdmin)
	platform.DELETE("/tenants/:id/admins/:userId", platformHandler.RemoveTenantAdmin)
	platform.GET("/policy-attributes", policyHandler.ListAttributes)
	platform.POST("/policy-attributes", policyHandler.CreateAttribute)
	platform.PUT("/policy-attributes/:attributeId", policyHandler.UpdateAttribute)
	platform.DELETE("/policy-attributes/:attributeId", policyHandler.DeleteAttribute)
	platform.GET("/policy-templates", policyHandler.ListTemplates)
	platform.POST("/policy-templates", policyHandler.CreateTemplate)
	platform.GET("/policy-templates/:templateId", policyHandler.TemplateDetail)
	platform.PUT("/policy-templates/:templateId", policyHandler.UpdateTemplate)
	platform.DELETE("/policy-templates/:templateId", policyHandler.DeleteTemplate)

	protected := api.Group("/users", middleware.AuthRequired(deps.AuthManager))
	protected.GET("/me", userHandler.Me)
	protected.PUT("/me", userHandler.UpdateMe)
	protected.POST("/me/avatar", userHandler.UploadAvatar)

	return router
}
