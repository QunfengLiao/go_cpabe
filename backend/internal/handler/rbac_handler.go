package handler

import (
	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

// RBACHandler 负责当前租户角色、权限和成员多角色授权相关 HTTP 接口。
type RBACHandler struct {
	service *service.TenantRoleService
}

// NewRBACHandler 创建 RBAC Handler。
func NewRBACHandler(service *service.TenantRoleService) *RBACHandler {
	return &RBACHandler{service: service}
}

// createTenantRoleRequest 是创建租户自定义业务角色的请求体。
type createTenantRoleRequest struct {
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	PermissionCodes []string `json:"permissionCodes"`
}

// updateTenantRoleRequest 是修改租户自定义业务角色展示字段的请求体。
type updateTenantRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// replaceRolePermissionsRequest 是全量替换角色权限集合的请求体。
type replaceRolePermissionsRequest struct {
	PermissionCodes []string `json:"permissionCodes"`
}

// replaceMemberRolesRequest 是全量替换成员角色集合的请求体。
type replaceMemberRolesRequest struct {
	RoleCodes []string `json:"roleCodes"`
}

// Permissions 返回租户自定义角色可选择的有效租户权限目录。
func (h *RBACHandler) Permissions(c *gin.Context) {
	items, err := h.service.ListPermissions(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

// Roles 返回当前租户可见的系统内置租户角色和自定义角色。
func (h *RBACHandler) Roles(c *gin.Context) {
	tenantID, ok := middleware.CurrentTenantID(c)
	if !ok {
		response.Fail(c, response.ErrTenantIDMissing)
		return
	}
	items, err := h.service.ListRoles(c.Request.Context(), tenantID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

// CreateRole 创建当前租户自定义业务角色，客户端不能指定租户、作用域或分类。
func (h *RBACHandler) CreateRole(c *gin.Context) {
	tenantID, actorID, ok := currentTenantActor(c)
	if !ok {
		return
	}
	var req createTenantRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	role, err := h.service.CreateRole(c.Request.Context(), tenantID, actorID, service.CreateTenantRoleInput{
		Code:            req.Code,
		Name:            req.Name,
		Description:     req.Description,
		PermissionCodes: req.PermissionCodes,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, role)
}

// RoleDetail 返回当前租户可见角色详情。
func (h *RBACHandler) RoleDetail(c *gin.Context) {
	tenantID, ok := middleware.CurrentTenantID(c)
	if !ok {
		response.Fail(c, response.ErrTenantIDMissing)
		return
	}
	roleID, ok := uintParam(c, "roleId")
	if !ok {
		return
	}
	role, err := h.service.GetRole(c.Request.Context(), tenantID, roleID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, role)
}

// UpdateRole 修改当前租户自定义业务角色的名称和描述。
func (h *RBACHandler) UpdateRole(c *gin.Context) {
	tenantID, actorID, ok := currentTenantActor(c)
	if !ok {
		return
	}
	roleID, ok := uintParam(c, "roleId")
	if !ok {
		return
	}
	var req updateTenantRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	role, err := h.service.UpdateRole(c.Request.Context(), tenantID, roleID, actorID, service.UpdateTenantRoleInput{Name: req.Name, Description: req.Description})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, role)
}

// DisableRole 逻辑禁用当前租户自定义业务角色，不物理删除历史绑定。
func (h *RBACHandler) DisableRole(c *gin.Context) {
	tenantID, actorID, ok := currentTenantActor(c)
	if !ok {
		return
	}
	roleID, ok := uintParam(c, "roleId")
	if !ok {
		return
	}
	id, affected, err := h.service.DisableRole(c.Request.Context(), tenantID, roleID, actorID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"roleId": id, "status": "DISABLED", "affectedMemberCount": affected})
}

// RolePermissions 查询当前租户可见角色绑定的权限集合。
func (h *RBACHandler) RolePermissions(c *gin.Context) {
	tenantID, ok := middleware.CurrentTenantID(c)
	if !ok {
		response.Fail(c, response.ErrTenantIDMissing)
		return
	}
	roleID, ok := uintParam(c, "roleId")
	if !ok {
		return
	}
	result, err := h.service.ListRolePermissions(c.Request.Context(), tenantID, roleID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// ReplaceRolePermissions 全量替换当前租户自定义业务角色权限。
func (h *RBACHandler) ReplaceRolePermissions(c *gin.Context) {
	tenantID, actorID, ok := currentTenantActor(c)
	if !ok {
		return
	}
	roleID, ok := uintParam(c, "roleId")
	if !ok {
		return
	}
	var req replaceRolePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, err := h.service.ReplaceRolePermissions(c.Request.Context(), tenantID, roleID, actorID, service.ReplaceRolePermissionsInput{PermissionCodes: req.PermissionCodes})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// MemberRoles 查询指定成员在当前租户内的角色和权限并集。
func (h *RBACHandler) MemberRoles(c *gin.Context) {
	tenantID, ok := middleware.CurrentTenantID(c)
	if !ok {
		response.Fail(c, response.ErrTenantIDMissing)
		return
	}
	userID, ok := uintParam(c, "userId")
	if !ok {
		return
	}
	result, err := h.service.GetMemberRoles(c.Request.Context(), tenantID, userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// ReplaceMemberRoles 全量替换指定成员在当前租户内的角色集合。
func (h *RBACHandler) ReplaceMemberRoles(c *gin.Context) {
	tenantID, actorID, ok := currentTenantActor(c)
	if !ok {
		return
	}
	userID, ok := uintParam(c, "userId")
	if !ok {
		return
	}
	var req replaceMemberRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, err := h.service.ReplaceMemberRoles(c.Request.Context(), tenantID, userID, actorID, service.ReplaceMemberRolesInput{RoleCodes: req.RoleCodes})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// CurrentAuthorization 返回当前登录用户在当前租户内的真实授权上下文。
func (h *RBACHandler) CurrentAuthorization(c *gin.Context) {
	tenantID, actorID, ok := currentTenantActor(c)
	if !ok {
		return
	}
	result, err := h.service.CurrentAuthorization(c.Request.Context(), tenantID, actorID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// currentTenantActor 从 gin.Context 读取可信租户 ID 和登录用户 ID。
func currentTenantActor(c *gin.Context) (uint64, uint64, bool) {
	tenantID, ok := middleware.CurrentTenantID(c)
	if !ok {
		response.Fail(c, response.ErrTenantIDMissing)
		return 0, 0, false
	}
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return 0, 0, false
	}
	return tenantID, userID, true
}
