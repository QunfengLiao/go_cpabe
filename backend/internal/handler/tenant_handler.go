package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

// TenantHandler 负责普通租户上下文、租户管理和成员管理 HTTP 请求。
type TenantHandler struct {
	service *service.TenantService
}

// NewTenantHandler 创建租户 Handler。
func NewTenantHandler(service *service.TenantService) *TenantHandler {
	return &TenantHandler{service: service}
}

// switchTenantRequest 是切换租户请求体，兼容 snake_case 和 camelCase。
type switchTenantRequest struct {
	TenantID      uint64 `json:"tenant_id"`
	TenantIDCamel uint64 `json:"tenantId"`
}

// createTenantRequest 是创建租户时使用的请求体。
type createTenantRequest struct {
	Name        string              `json:"name"`
	Code        string              `json:"code"`
	Status      domain.TenantStatus `json:"status"`
	Description string              `json:"description"`
}

// addTenantUserRequest 是向租户添加成员时的请求体。
type addTenantUserRequest struct {
	UserID uint64            `json:"user_id"`
	Roles  []domain.RoleCode `json:"roles"`
}

// MyTenants 返回当前用户可访问的租户上下文。
func (h *TenantHandler) MyTenants(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return
	}
	context, err := h.service.TenantContextForUser(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, context)
}

// SwitchTenant 处理租户切换请求，并返回切换后的租户角色上下文。
func (h *TenantHandler) SwitchTenant(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return
	}
	var req switchTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	tenantID := req.TenantID
	if tenantID == 0 {
		tenantID = req.TenantIDCamel
	}
	if tenantID == 0 {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, err := h.service.SwitchTenant(c.Request.Context(), userID, tenantID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// CreateTenant 处理普通租户创建请求，权限由 Service 层校验。
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return
	}
	var req createTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	tenant, err := h.service.CreateTenant(c.Request.Context(), userID, service.CreateTenantInput{
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

// ListTenants 返回调用者可见的租户列表。
func (h *TenantHandler) ListTenants(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return
	}
	tenants, err := h.service.ListTenants(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenants": tenants})
}

// TenantDetail 返回指定租户详情，租户 ID 来自路径参数。
func (h *TenantHandler) TenantDetail(c *gin.Context) {
	userID, tenantID, ok := tenantPathIDs(c, false)
	if !ok {
		return
	}
	tenant, err := h.service.TenantDetail(c.Request.Context(), userID, tenantID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenant": tenant})
}

// EnableTenant 将指定租户状态设置为启用。
func (h *TenantHandler) EnableTenant(c *gin.Context) {
	h.setTenantStatus(c, domain.TenantStatusEnabled)
}

// DisableTenant 将指定租户状态设置为禁用。
func (h *TenantHandler) DisableTenant(c *gin.Context) {
	h.setTenantStatus(c, domain.TenantStatusDisabled)
}

// AddTenantUser 处理向租户添加成员和授予租户内角色的请求。
func (h *TenantHandler) AddTenantUser(c *gin.Context) {
	userID, tenantID, ok := tenantPathIDs(c, false)
	if !ok {
		return
	}
	var req addTenantUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	member, err := h.service.AddTenantUser(c.Request.Context(), userID, tenantID, service.AddTenantUserInput{UserID: req.UserID, Roles: req.Roles})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, member)
}

// RemoveTenantUser 处理从租户移除成员的请求。
func (h *TenantHandler) RemoveTenantUser(c *gin.Context) {
	userID, tenantID, ok := tenantPathIDs(c, false)
	if !ok {
		return
	}
	targetUserID, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil || targetUserID == 0 {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	if err := h.service.RemoveTenantUser(c.Request.Context(), userID, tenantID, targetUserID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenant_id": tenantID, "user_id": targetUserID, "removed": true})
}

// ListTenantUsers 返回指定租户成员列表。
func (h *TenantHandler) ListTenantUsers(c *gin.Context) {
	userID, tenantID, ok := tenantPathIDs(c, false)
	if !ok {
		return
	}
	users, err := h.service.ListTenantUsers(c.Request.Context(), userID, tenantID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"users": users})
}

// setTenantStatus 复用启用/禁用租户逻辑，并统一响应租户状态。
func (h *TenantHandler) setTenantStatus(c *gin.Context, status domain.TenantStatus) {
	userID, tenantID, ok := tenantPathIDs(c, false)
	if !ok {
		return
	}
	tenant, err := h.service.SetTenantStatus(c.Request.Context(), userID, tenantID, status)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenant_id": tenant.TenantID, "status": tenant.Status})
}

// tenantPathIDs 从路径参数中解析租户 ID 和当前用户 ID，失败时直接写入错误响应。
func tenantPathIDs(c *gin.Context, _ bool) (userID uint64, tenantID uint64, ok bool) {
	id, hasUser := currentUserID(c)
	if !hasUser {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return 0, 0, false
	}
	parsed, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || parsed == 0 {
		response.Fail(c, response.ErrBadRequest)
		return 0, 0, false
	}
	return id, parsed, true
}
