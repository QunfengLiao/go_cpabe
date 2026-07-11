package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

// OrgAttributeHandler 负责租户组织架构、部门角色和用户属性 HTTP 请求。
type OrgAttributeHandler struct {
	service           *service.OrgAttributeService
	managementService *service.OrgManagementService
}

// NewOrgAttributeHandler 创建组织属性 Handler。
func NewOrgAttributeHandler(service *service.OrgAttributeService) *OrgAttributeHandler {
	return &OrgAttributeHandler{service: service}
}

// NewOrgAttributeHandlerWithManagement 创建组织属性 Handler，并为旧组织写接口注入新组织管理 Service。
func NewOrgAttributeHandlerWithManagement(service *service.OrgAttributeService, managementService *service.OrgManagementService) *OrgAttributeHandler {
	return &OrgAttributeHandler{service: service, managementService: managementService}
}

// addOrgMemberRequest 是租户管理员把用户加入部门的请求体。
type addOrgMemberRequest struct {
	UserID uint64 `json:"userId"`
}

// setOrgMemberRolesRequest 是租户管理员设置部门内通用角色的请求体。
type setOrgMemberRolesRequest struct {
	RoleCodes []domain.OrgMemberRoleCode `json:"roleCodes"`
}

// OrgTree 返回当前租户组织树，供组织管理和访问策略构建器使用。
func (h *OrgAttributeHandler) OrgTree(c *gin.Context) {
	actor, ok := orgAttributeActor(c)
	if !ok {
		return
	}
	tree, err := h.service.ListOrgTree(c.Request.Context(), actor, c.Query("status") == "all")
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": tree, "total": len(tree)})
}

// PolicyAttributes 返回访问策略构建器使用的当前租户真实属性字典。
func (h *OrgAttributeHandler) PolicyAttributes(c *gin.Context) {
	actor, ok := orgAttributeActor(c)
	if !ok {
		return
	}
	attrs, err := h.service.ListPolicyAttributes(c.Request.Context(), actor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": attrs, "total": len(attrs)})
}

// ListOrgMembers 返回指定部门成员和部门内角色。
func (h *OrgAttributeHandler) ListOrgMembers(c *gin.Context) {
	actor, orgUnitID, ok := orgUnitActor(c)
	if !ok {
		return
	}
	members, err := h.service.ListOrgMembers(c.Request.Context(), actor, orgUnitID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": members, "total": len(members)})
}

// AddOrgMember 处理把本租户用户加入部门的请求。
//
// Deprecated: 旧 `/api/v1/tenants/:id/...` 写接口仅用于过渡期，前端组织管理页面应调用 `/api/v1/tenant/org-members`。
func (h *OrgAttributeHandler) AddOrgMember(c *gin.Context) {
	actor, orgUnitID, ok := orgUnitActor(c)
	if !ok {
		return
	}
	var req addOrgMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	if h.managementService != nil {
		member, err := h.managementService.AddOrgMember(c.Request.Context(), managementActorFromAttribute(actor), service.AddOrgManagementMemberInput{UserID: req.UserID, OrgUnitID: orgUnitID})
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.Created(c, gin.H{"member": member})
		return
	}
	member, err := h.service.AddOrgMember(c.Request.Context(), actor, orgUnitID, service.AddOrgMemberInput{UserID: req.UserID})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, gin.H{"member": member})
}

// RemoveOrgMember 处理从部门移除用户的请求。
//
// Deprecated: 旧 `/api/v1/tenants/:id/...` 写接口仅用于过渡期，前端组织管理页面应调用 `/api/v1/tenant/org-members/:id`。
func (h *OrgAttributeHandler) RemoveOrgMember(c *gin.Context) {
	actor, orgUnitID, ok := orgUnitActor(c)
	if !ok {
		return
	}
	userID, ok := uintParam(c, "userId")
	if !ok {
		return
	}
	if h.managementService != nil {
		member, err := h.service.FindOrgMemberForBridge(c.Request.Context(), actor, orgUnitID, userID)
		if err != nil {
			response.Fail(c, err)
			return
		}
		if _, err := h.managementService.RemoveOrgMember(c.Request.Context(), managementActorFromAttribute(actor), member.ID, service.RemoveOrgMemberInput{}); err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, gin.H{"removed": true, "orgUnitId": orgUnitID, "userId": userID})
		return
	}
	if err := h.service.RemoveOrgMember(c.Request.Context(), actor, orgUnitID, userID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"removed": true, "orgUnitId": orgUnitID, "userId": userID})
}

// SetOrgMemberRoles 处理设置用户部门内通用角色的请求。
//
// Deprecated: 旧 `/api/v1/tenants/:id/...` 写接口仅用于过渡期，前端组织管理页面应调用 `/api/v1/tenant/org-members/:id/positions`。
func (h *OrgAttributeHandler) SetOrgMemberRoles(c *gin.Context) {
	actor, orgUnitID, ok := orgUnitActor(c)
	if !ok {
		return
	}
	userID, ok := uintParam(c, "userId")
	if !ok {
		return
	}
	var req setOrgMemberRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	if h.managementService != nil {
		member, err := h.service.FindOrgMemberForBridge(c.Request.Context(), actor, orgUnitID, userID)
		if err != nil {
			response.Fail(c, err)
			return
		}
		roles, err := h.managementService.SetOrgMemberPositions(c.Request.Context(), managementActorFromAttribute(actor), member.ID, service.SetOrgMemberPositionsInput{Positions: req.RoleCodes})
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, gin.H{"userId": userID, "orgUnitId": orgUnitID, "roleCodes": roles, "synced": false})
		return
	}
	roles, err := h.service.SetOrgMemberRoles(c.Request.Context(), actor, orgUnitID, userID, service.SetOrgMemberRolesInput{RoleCodes: req.RoleCodes})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"userId": userID, "orgUnitId": orgUnitID, "roleCodes": roles, "synced": false})
}

// managementActorFromAttribute 将旧组织属性 Actor 转换为新组织管理 Actor，保持同一租户上下文。
func managementActorFromAttribute(actor service.OrgAttributeActor) service.OrgManagementActor {
	return service.OrgManagementActor{UserID: actor.UserID, TenantID: actor.TenantID, Roles: actor.Roles}
}

// SyncUserAttributes 处理租户管理员同步指定用户 CP-ABE 属性的请求。
func (h *OrgAttributeHandler) SyncUserAttributes(c *gin.Context) {
	actor, ok := orgAttributeActor(c)
	if !ok {
		return
	}
	userID, ok := uintParam(c, "userId")
	if !ok {
		return
	}
	attrs, err := h.service.SyncUserAttributes(c.Request.Context(), actor, userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenantId": actor.TenantID, "userId": userID, "items": attrs})
}

// MyUserAttributes 返回当前用户在当前租户下的有效 CP-ABE 属性。
func (h *OrgAttributeHandler) MyUserAttributes(c *gin.Context) {
	actor, ok := orgAttributeActor(c)
	if !ok {
		return
	}
	attrs, err := h.service.ListMyUserAttributes(c.Request.Context(), actor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"tenantId": actor.TenantID, "userId": actor.UserID, "items": attrs})
}

// orgUnitActor 同时解析租户上下文和部门 ID，供部门成员相关接口复用。
func orgUnitActor(c *gin.Context) (service.OrgAttributeActor, uint64, bool) {
	actor, ok := orgAttributeActor(c)
	if !ok {
		return service.OrgAttributeActor{}, 0, false
	}
	raw := c.Param("orgUnitId")
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, response.ErrBadRequest)
		return service.OrgAttributeActor{}, 0, false
	}
	return actor, id, true
}

// orgAttributeActor 从认证和租户中间件上下文构造组织属性调用者。
func orgAttributeActor(c *gin.Context) (service.OrgAttributeActor, bool) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return service.OrgAttributeActor{}, false
	}
	pathTenantID, ok := uintParam(c, "id")
	if !ok {
		return service.OrgAttributeActor{}, false
	}
	contextTenantID, ok := middleware.CurrentTenantID(c)
	if !ok || contextTenantID != pathTenantID {
		response.Fail(c, response.ErrTenantPermissionDenied)
		return service.OrgAttributeActor{}, false
	}
	value, _ := c.Get(middleware.ContextTenantRoles)
	roles, _ := value.([]domain.RoleCode)
	return service.OrgAttributeActor{UserID: userID, TenantID: pathTenantID, Roles: roles}, true
}
