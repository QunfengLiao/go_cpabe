package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

// PlatformHandler 负责平台后台租户、成员、角色和 dashboard HTTP 请求。
type PlatformHandler struct {
	tenants   *service.PlatformTenantService
	users     *service.PlatformTenantUserService
	roles     *service.PlatformRoleService
	dashboard *service.PlatformDashboardService
}

// NewPlatformHandler 创建平台后台 Handler。
func NewPlatformHandler(
	tenants *service.PlatformTenantService,
	users *service.PlatformTenantUserService,
	roles *service.PlatformRoleService,
	dashboard *service.PlatformDashboardService,
) *PlatformHandler {
	return &PlatformHandler{tenants: tenants, users: users, roles: roles, dashboard: dashboard}
}

// platformAddUserRequest 是平台后台添加租户成员请求体。
type platformAddUserRequest struct {
	UserID uint64 `json:"user_id"`
}

// platformAssignAdminRequest 是平台后台授予租户管理员请求体。
type platformAssignAdminRequest struct {
	UserID uint64 `json:"user_id"`
}

// Dashboard 返回平台后台首页统计数据。
func (h *PlatformHandler) Dashboard(c *gin.Context) {
	summary, err := h.dashboard.Summary(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, summary)
}

// ListTenants 返回平台后台租户列表。
func (h *PlatformHandler) ListTenants(c *gin.Context) {
	tenants, err := h.tenants.ListTenants(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenants": tenants})
}

// CreateTenant 处理平台后台创建租户请求，并记录操作者 ID。
func (h *PlatformHandler) CreateTenant(c *gin.Context) {
	actorID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return
	}
	var req createTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	tenant, err := h.tenants.CreateTenant(c.Request.Context(), actorID, service.CreateTenantInput{
		Name:        req.Name,
		Code:        req.Code,
		Status:      req.Status,
		Description: req.Description,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, gin.H{"tenant": tenant})
}

// TenantDetail 返回平台后台租户详情。
func (h *PlatformHandler) TenantDetail(c *gin.Context) {
	tenantID, ok := platformTenantID(c)
	if !ok {
		return
	}
	tenant, err := h.tenants.TenantDetail(c.Request.Context(), tenantID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenant": tenant})
}

// EnableTenant 将指定租户设置为启用。
func (h *PlatformHandler) EnableTenant(c *gin.Context) {
	h.setTenantStatus(c, domain.TenantStatusEnabled)
}

// DisableTenant 将指定租户设置为禁用。
func (h *PlatformHandler) DisableTenant(c *gin.Context) {
	h.setTenantStatus(c, domain.TenantStatusDisabled)
}

// ListTenantUsers 返回平台后台指定租户成员列表。
func (h *PlatformHandler) ListTenantUsers(c *gin.Context) {
	tenantID, ok := platformTenantID(c)
	if !ok {
		return
	}
	users, err := h.users.ListTenantUsers(c.Request.Context(), tenantID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"users": users})
}

// AddTenantUser 处理平台后台向租户添加成员请求。
func (h *PlatformHandler) AddTenantUser(c *gin.Context) {
	actorID, tenantID, ok := platformActorAndTenant(c)
	if !ok {
		return
	}
	var req platformAddUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	member, err := h.users.AddTenantUser(c.Request.Context(), actorID, tenantID, req.UserID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, member)
}

// RemoveTenantUser 处理平台后台移除租户成员请求。
func (h *PlatformHandler) RemoveTenantUser(c *gin.Context) {
	actorID, tenantID, ok := platformActorAndTenant(c)
	if !ok {
		return
	}
	userID, ok := platformUserID(c)
	if !ok {
		return
	}
	if err := h.users.RemoveTenantUser(c.Request.Context(), actorID, tenantID, userID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenant_id": tenantID, "user_id": userID, "removed": true})
}

// AssignTenantAdmin 处理平台后台授予租户管理员角色请求。
func (h *PlatformHandler) AssignTenantAdmin(c *gin.Context) {
	actorID, tenantID, ok := platformActorAndTenant(c)
	if !ok {
		return
	}
	var req platformAssignAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, err := h.roles.AssignTenantAdmin(c.Request.Context(), actorID, tenantID, req.UserID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// RemoveTenantAdmin 处理平台后台撤销租户管理员角色请求。
func (h *PlatformHandler) RemoveTenantAdmin(c *gin.Context) {
	actorID, tenantID, ok := platformActorAndTenant(c)
	if !ok {
		return
	}
	userID, ok := platformUserID(c)
	if !ok {
		return
	}
	result, err := h.roles.RemoveTenantAdmin(c.Request.Context(), actorID, tenantID, userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// setTenantStatus 复用平台后台启用/禁用租户逻辑。
func (h *PlatformHandler) setTenantStatus(c *gin.Context, status domain.TenantStatus) {
	actorID, tenantID, ok := platformActorAndTenant(c)
	if !ok {
		return
	}
	tenant, err := h.tenants.SetTenantStatus(c.Request.Context(), actorID, tenantID, status)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenant_id": tenant.TenantID, "status": tenant.Status})
}

// platformActorAndTenant 解析当前平台操作者 ID 和路径中的租户 ID。
func platformActorAndTenant(c *gin.Context) (uint64, uint64, bool) {
	actorID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return 0, 0, false
	}
	tenantID, ok := platformTenantID(c)
	if !ok {
		return 0, 0, false
	}
	return actorID, tenantID, true
}

// platformTenantID 从路径参数中解析租户 ID，失败时写入请求错误响应。
func platformTenantID(c *gin.Context) (uint64, bool) {
	parsed, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || parsed == 0 {
		response.Fail(c, response.ErrBadRequest)
		return 0, false
	}
	return parsed, true
}

// platformUserID 从路径参数中解析目标用户 ID，失败时写入请求错误响应。
func platformUserID(c *gin.Context) (uint64, bool) {
	parsed, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil || parsed == 0 {
		response.Fail(c, response.ErrBadRequest)
		return 0, false
	}
	return parsed, true
}
