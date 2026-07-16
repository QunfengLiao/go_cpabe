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
	OrgAttributeService       *service.OrgAttributeService
	OrgManagementService      *service.OrgManagementService
	AuthorizationService      middleware.PermissionAuthorizer
	TenantRoleService         *service.TenantRoleService
	PlatformRoleResolver      middleware.PlatformRoleResolver
	AuthManager               *auth.Manager
	HealthService             *service.HealthService
	EncryptionService         *service.EncryptionService
	RSAKeyService             *service.RSAKeyService
	EncryptedFileService      *service.EncryptedFileService
	MaxAvatarSize             int64
	MaxEncryptedFileSize      int64
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
	orgAttributeHandler := NewOrgAttributeHandlerWithManagement(deps.OrgAttributeService, deps.OrgManagementService)
	orgManagementHandler := NewOrgManagementHandler(deps.OrgManagementService)
	rbacHandler := NewRBACHandler(deps.TenantRoleService)

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
	me.GET("/context", tenantHandler.MyTenants)
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

	tenantPolicies := api.Group("/tenants/:id", middleware.AuthRequired(deps.AuthManager), middleware.TenantRequired(deps.TenantService))
	if deps.OrgAttributeService != nil {
		tenantPolicies.GET("/access-policy/attributes", middleware.TenantPermissionRequired(deps.AuthorizationService, "policy.read"), orgAttributeHandler.PolicyAttributes)
	} else {
		tenantPolicies.GET("/access-policy/attributes", middleware.TenantPermissionRequired(deps.AuthorizationService, "policy.read"), policyHandler.AvailableAttributes)
	}
	tenantPolicies.GET("/access-policy/templates", middleware.TenantPermissionRequired(deps.AuthorizationService, "policy.read"), policyHandler.AvailableTemplates)
	tenantPolicies.GET("/access-policies", middleware.TenantPermissionRequired(deps.AuthorizationService, "policy.read"), policyHandler.ListAccessPolicies)
	tenantPolicies.POST("/access-policies", middleware.TenantPermissionRequired(deps.AuthorizationService, "policy.write"), policyHandler.CreateAccessPolicy)
	tenantPolicies.GET("/access-policies/:policyId", middleware.TenantPermissionRequired(deps.AuthorizationService, "policy.read"), policyHandler.AccessPolicyDetail)
	tenantPolicies.PUT("/access-policies/:policyId", middleware.TenantPermissionRequired(deps.AuthorizationService, "policy.write"), policyHandler.UpdateAccessPolicy)
	tenantPolicies.DELETE("/access-policies/:policyId", middleware.TenantPermissionRequired(deps.AuthorizationService, "policy.write"), policyHandler.DeleteAccessPolicy)
	tenantPolicies.GET("/org-units/tree", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.read"), orgAttributeHandler.OrgTree)
	tenantPolicies.GET("/org-units/:orgUnitId/members", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.read"), orgAttributeHandler.ListOrgMembers)
	tenantPolicies.POST("/org-units/:orgUnitId/members", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgAttributeHandler.AddOrgMember)
	tenantPolicies.DELETE("/org-units/:orgUnitId/members/:userId", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgAttributeHandler.RemoveOrgMember)
	tenantPolicies.PUT("/org-units/:orgUnitId/members/:userId/roles", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgAttributeHandler.SetOrgMemberRoles)
	tenantPolicies.POST("/users/:userId/attributes/sync", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgAttributeHandler.SyncUserAttributes)
	tenantPolicies.GET("/users/me/attributes", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.read"), orgAttributeHandler.MyUserAttributes)

	currentTenant := api.Group("/tenant", middleware.AuthRequired(deps.AuthManager), middleware.TenantRequired(deps.TenantService))
	// 文件中心是密文仓库读取边界，只校验登录态和当前租户成员关系，不复用任何 file.* 权限中间件。
	// 这样即使上传、删除等管理权限发生变化，租户成员仍能下载原始密文并在本地决定是否解密。
	fileCenterTenant := api.Group("/tenant", middleware.AuthRequired(deps.AuthManager), middleware.TenantRequired(deps.TenantService))
	currentTenant.POST("/members", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.member.manage"), tenantHandler.CreateCurrentTenantMember)
	if deps.TenantRoleService != nil {
		currentTenant.GET("/permissions", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.role.read"), rbacHandler.Permissions)
		currentTenant.GET("/roles", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.role.read"), rbacHandler.Roles)
		currentTenant.POST("/roles", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.role.manage"), rbacHandler.CreateRole)
		currentTenant.GET("/roles/:roleId", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.role.read"), rbacHandler.RoleDetail)
		currentTenant.PATCH("/roles/:roleId", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.role.manage"), rbacHandler.UpdateRole)
		currentTenant.DELETE("/roles/:roleId", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.role.manage"), rbacHandler.DisableRole)
		currentTenant.GET("/roles/:roleId/permissions", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.role.read"), rbacHandler.RolePermissions)
		currentTenant.PUT("/roles/:roleId/permissions", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.role.manage"), rbacHandler.ReplaceRolePermissions)
		currentTenant.GET("/members/:userId/roles", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.member.read"), rbacHandler.MemberRoles)
		currentTenant.PUT("/members/:userId/roles", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.member.manage"), rbacHandler.ReplaceMemberRoles)
		currentTenant.GET("/me/authorization", rbacHandler.CurrentAuthorization)
	}
	if deps.OrgManagementService != nil {
		currentTenant.GET("/org-units/tree", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.read"), orgManagementHandler.OrgTree)
		currentTenant.POST("/org-units", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgManagementHandler.CreateOrgUnit)
		currentTenant.PUT("/org-units/:id", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgManagementHandler.UpdateOrgUnit)
		currentTenant.PUT("/org-units/:id/move", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgManagementHandler.MoveOrgUnit)
		currentTenant.DELETE("/org-units/:id", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgManagementHandler.DeleteOrgUnit)
		currentTenant.GET("/org-members", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.read"), orgManagementHandler.ListOrgMembers)
		currentTenant.POST("/org-members", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgManagementHandler.AddOrgMember)
		currentTenant.PUT("/org-members/:id/primary", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgManagementHandler.SetOrgMemberPrimary)
		currentTenant.PUT("/org-members/:id/positions", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgManagementHandler.SetOrgMemberPositions)
		currentTenant.DELETE("/org-members/:id", middleware.TenantPermissionRequired(deps.AuthorizationService, "tenant.org.manage"), orgManagementHandler.RemoveOrgMember)
	}
	if deps.EncryptionService != nil && deps.RSAKeyService != nil && deps.EncryptedFileService != nil {
		encryptionHandler := NewEncryptionHandler(deps.EncryptionService, deps.MaxEncryptedFileSize)
		rsaKeyHandler := NewRSAKeyHandler(deps.RSAKeyService)
		encryptedFileHandler := NewEncryptedFileHandler(deps.EncryptedFileService)
		currentTenant.GET("/encryption-algorithms", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.upload"), encryptionHandler.Algorithms)
		currentTenant.GET("/rsa-recipients", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.upload"), rsaKeyHandler.Recipients)
		currentTenant.GET("/rsa-recipients/:userId/public-keys", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.upload"), rsaKeyHandler.RecipientKeys)
		currentTenant.GET("/me/rsa-public-keys", middleware.TenantPermissionRequired(deps.AuthorizationService, "crypto.key.self.manage"), rsaKeyHandler.MyKeys)
		currentTenant.POST("/me/rsa-public-keys", middleware.TenantPermissionRequired(deps.AuthorizationService, "crypto.key.self.manage"), rsaKeyHandler.RegisterMyKey)
		currentTenant.PATCH("/rsa-public-keys/:keyId/status", middleware.TenantPermissionRequired(deps.AuthorizationService, "crypto.key.manage"), rsaKeyHandler.UpdateStatus)
		currentTenant.POST("/encryption-tasks", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.upload"), encryptionHandler.CreateTask)
		currentTenant.GET("/encryption-tasks/:taskId", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.upload"), encryptionHandler.Task)
		currentTenant.POST("/encryption-tasks/:taskId/attempts/:attemptId/progress", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.upload"), encryptionHandler.Progress)
		currentTenant.PUT("/encryption-tasks/:taskId/attempts/:attemptId/ciphertext", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.upload"), encryptionHandler.UploadCiphertext)
		currentTenant.POST("/encryption-tasks/:taskId/attempts/:attemptId/complete", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.upload"), encryptionHandler.Complete)
		currentTenant.POST("/encryption-tasks/:taskId/attempts/:attemptId/fail", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.upload"), encryptionHandler.Fail)
		currentTenant.POST("/encryption-tasks/:taskId/cancel", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.manage"), encryptionHandler.Cancel)
		currentTenant.POST("/encryption-tasks/:taskId/retry", middleware.TenantPermissionRequired(deps.AuthorizationService, "file.manage"), encryptionHandler.Retry)
		// 文件仓库只依赖 AuthRequired + TenantRequired：有效租户成员可以读取和下载密文，密钥恢复完全由客户端本地私钥决定。
		fileCenterTenant.GET("/files", encryptedFileHandler.ListFileCenter)
		fileCenterTenant.GET("/files/:fileId", encryptedFileHandler.FileCenterDetail)
		fileCenterTenant.GET("/files/:fileId/ciphertext", encryptedFileHandler.DownloadFileCenter)
		fileCenterTenant.GET("/files/:fileId/decryption-material", encryptedFileHandler.OwnDecryptionMaterial)
		currentTenant.GET("/encrypted-files", encryptedFileHandler.List)
		currentTenant.GET("/encrypted-files/:fileId", encryptedFileHandler.Detail)
		currentTenant.GET("/encrypted-files/:fileId/ciphertext", encryptedFileHandler.Download)
		currentTenant.GET("/received-files", encryptedFileHandler.ListReceived)
		currentTenant.GET("/received-files/:fileId/decryption-material", encryptedFileHandler.ReceivedMaterial)
		currentTenant.GET("/received-files/:fileId/ciphertext", encryptedFileHandler.DownloadReceived)
	}

	platform := api.Group("/platform", middleware.AuthRequired(deps.AuthManager), middleware.PlatformAdminRequired(deps.PlatformRoleResolver))
	platform.GET("/dashboard", platformHandler.Dashboard)
	platform.GET("/users/search", platformHandler.SearchUsers)
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
