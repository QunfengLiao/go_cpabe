package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

type PlatformHandler struct {
	tenants   *service.PlatformTenantService
	users     *service.PlatformTenantUserService
	roles     *service.PlatformRoleService
	dashboard *service.PlatformDashboardService
}

func NewPlatformHandler(
	tenants *service.PlatformTenantService,
	users *service.PlatformTenantUserService,
	roles *service.PlatformRoleService,
	dashboard *service.PlatformDashboardService,
) *PlatformHandler {
	return &PlatformHandler{tenants: tenants, users: users, roles: roles, dashboard: dashboard}
}

type platformAddUserRequest struct {
	UserID uint64 `json:"user_id"`
}

type platformAssignAdminRequest struct {
	UserID uint64 `json:"user_id"`
}

func (h *PlatformHandler) Dashboard(c *gin.Context) {
	summary, err := h.dashboard.Summary(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, summary)
}

func (h *PlatformHandler) ListTenants(c *gin.Context) {
	tenants, err := h.tenants.ListTenants(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenants": tenants})
}

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

func (h *PlatformHandler) EnableTenant(c *gin.Context) {
	h.setTenantStatus(c, domain.TenantStatusEnabled)
}

func (h *PlatformHandler) DisableTenant(c *gin.Context) {
	h.setTenantStatus(c, domain.TenantStatusDisabled)
}

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

func platformTenantID(c *gin.Context) (uint64, bool) {
	parsed, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || parsed == 0 {
		response.Fail(c, response.ErrBadRequest)
		return 0, false
	}
	return parsed, true
}

func platformUserID(c *gin.Context) (uint64, bool) {
	parsed, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil || parsed == 0 {
		response.Fail(c, response.ErrBadRequest)
		return 0, false
	}
	return parsed, true
}
