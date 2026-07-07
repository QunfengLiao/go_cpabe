package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

type TenantHandler struct {
	service *service.TenantService
}

func NewTenantHandler(service *service.TenantService) *TenantHandler {
	return &TenantHandler{service: service}
}

type switchTenantRequest struct {
	TenantID      uint64 `json:"tenant_id"`
	TenantIDCamel uint64 `json:"tenantId"`
}

type createTenantRequest struct {
	Name        string              `json:"name"`
	Code        string              `json:"code"`
	Status      domain.TenantStatus `json:"status"`
	Description string              `json:"description"`
}

type addTenantUserRequest struct {
	UserID uint64            `json:"user_id"`
	Roles  []domain.RoleCode `json:"roles"`
}

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

func (h *TenantHandler) EnableTenant(c *gin.Context) {
	h.setTenantStatus(c, domain.TenantStatusEnabled)
}

func (h *TenantHandler) DisableTenant(c *gin.Context) {
	h.setTenantStatus(c, domain.TenantStatusDisabled)
}

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
