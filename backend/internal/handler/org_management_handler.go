package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

// OrgManagementHandler 负责当前租户组织管理接口的请求绑定和响应封装。
type OrgManagementHandler struct {
	service *service.OrgManagementService
}

// NewOrgManagementHandler 创建当前租户组织管理 Handler。
func NewOrgManagementHandler(service *service.OrgManagementService) *OrgManagementHandler {
	return &OrgManagementHandler{service: service}
}

// createOrgUnitRequest 是创建部门接口的请求体，tenant_id 不允许由前端传入。
type createOrgUnitRequest struct {
	ParentID  *uint64 `json:"parentId"`
	Name      string  `json:"name"`
	SortOrder int     `json:"sortOrder"`
}

// updateOrgUnitRequest 是编辑部门接口的请求体，只包含可变展示字段和启停状态。
type updateOrgUnitRequest struct {
	Name      string                `json:"name"`
	SortOrder *int                  `json:"sortOrder"`
	Status    *domain.OrgUnitStatus `json:"status"`
}

// moveOrgUnitRequest 是移动部门接口的请求体，targetParentId 为空表示移动为根部门。
type moveOrgUnitRequest struct {
	TargetParentID *uint64 `json:"targetParentId"`
	SortOrder      *int    `json:"sortOrder"`
}

// addOrgManagementMemberRequest 是加入部门接口的请求体，不包含 systemRoles。
type addOrgManagementMemberRequest struct {
	UserID    uint64 `json:"userId"`
	OrgUnitID uint64 `json:"orgUnitId"`
	IsPrimary bool   `json:"isPrimary"`
}

// setOrgMemberPrimaryRequest 是设置主部门接口的请求体。
type setOrgMemberPrimaryRequest struct {
	Primary bool `json:"primary"`
}

// setOrgMemberPositionsRequest 是设置部门职务接口的请求体，不允许携带系统角色。
type setOrgMemberPositionsRequest struct {
	Positions []domain.OrgMemberRoleCode `json:"positions"`
}

// removeOrgMemberRequest 是移除部门关系接口的可选请求体。
type removeOrgMemberRequest struct {
	NewPrimaryMemberID *uint64 `json:"newPrimaryMemberId"`
}

// OrgTree 返回当前租户完整组织树，租户 ID 只来自后端租户上下文。
func (h *OrgManagementHandler) OrgTree(c *gin.Context) {
	actor, ok := orgManagementActor(c)
	if !ok {
		return
	}
	tree, err := h.service.ListOrgTree(c.Request.Context(), actor, c.Query("status") != "enabled")
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": tree, "total": len(tree)})
}

// CreateOrgUnit 处理创建根部门或子部门请求，并由 Service 同事务创建属性值。
func (h *OrgManagementHandler) CreateOrgUnit(c *gin.Context) {
	actor, ok := orgManagementActor(c)
	if !ok {
		return
	}
	var req createOrgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, err := h.service.CreateOrgUnit(c.Request.Context(), actor, service.CreateOrgUnitInput{ParentID: req.ParentID, Name: req.Name, SortOrder: req.SortOrder})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, result)
}

// UpdateOrgUnit 处理部门名称、排序和状态更新，停用语义由 Service 统一校验。
func (h *OrgManagementHandler) UpdateOrgUnit(c *gin.Context) {
	actor, id, ok := orgManagementUnitActor(c)
	if !ok {
		return
	}
	var req updateOrgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, err := h.service.UpdateOrgUnit(c.Request.Context(), actor, id, service.UpdateOrgUnitInput{Name: req.Name, SortOrder: req.SortOrder, Status: req.Status})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// MoveOrgUnit 处理部门移动请求，循环校验和子树属性同步由 Service 完成。
func (h *OrgManagementHandler) MoveOrgUnit(c *gin.Context) {
	actor, id, ok := orgManagementUnitActor(c)
	if !ok {
		return
	}
	var req moveOrgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, err := h.service.MoveOrgUnit(c.Request.Context(), actor, id, service.MoveOrgUnitInput{TargetParentID: req.TargetParentID, SortOrder: req.SortOrder})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// DeleteOrgUnit 处理部门删除请求，有子部门或有效成员时返回业务冲突。
func (h *OrgManagementHandler) DeleteOrgUnit(c *gin.Context) {
	actor, id, ok := orgManagementUnitActor(c)
	if !ok {
		return
	}
	if err := h.service.DeleteOrgUnit(c.Request.Context(), actor, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true, "id": id})
}

// ListOrgMembers 返回当前租户组织成员分页列表，systemRoles 只读不写。
func (h *OrgManagementHandler) ListOrgMembers(c *gin.Context) {
	actor, ok := orgManagementActor(c)
	if !ok {
		return
	}
	orgUnitID, _ := strconv.ParseUint(c.Query("orgUnitId"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	result, err := h.service.ListOrgMembers(c.Request.Context(), actor, service.ListOrgMembersInput{
		Keyword:   c.Query("keyword"),
		OrgUnitID: orgUnitID,
		Status:    c.DefaultQuery("status", "active"),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// AddOrgMember 处理把租户有效成员加入部门的请求，本接口不会修改 systemRoles。
func (h *OrgManagementHandler) AddOrgMember(c *gin.Context) {
	actor, ok := orgManagementActor(c)
	if !ok {
		return
	}
	var req addOrgManagementMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	member, err := h.service.AddOrgMember(c.Request.Context(), actor, service.AddOrgManagementMemberInput{UserID: req.UserID, OrgUnitID: req.OrgUnitID, IsPrimary: req.IsPrimary})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, gin.H{"member": member})
}

// SetOrgMemberPrimary 处理主部门切换请求，确保同租户同用户只保留一个主部门。
func (h *OrgManagementHandler) SetOrgMemberPrimary(c *gin.Context) {
	actor, memberID, ok := orgManagementMemberActor(c)
	if !ok {
		return
	}
	var req setOrgMemberPrimaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	member, err := h.service.SetOrgMemberPrimary(c.Request.Context(), actor, memberID, service.SetOrgMemberPrimaryInput{Primary: req.Primary})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"memberId": member.ID, "userId": member.UserID, "primary": member.IsPrimary})
}

// SetOrgMemberPositions 处理部门职务替换请求，显式拒绝系统角色写入部门职务表。
func (h *OrgManagementHandler) SetOrgMemberPositions(c *gin.Context) {
	actor, memberID, ok := orgManagementMemberActor(c)
	if !ok {
		return
	}
	var req setOrgMemberPositionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	positions, err := h.service.SetOrgMemberPositions(c.Request.Context(), actor, memberID, service.SetOrgMemberPositionsInput{Positions: req.Positions})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"memberId": memberID, "positions": positions})
}

// RemoveOrgMember 处理移除部门关系请求，删除主部门时由 Service 同事务选择新主部门。
func (h *OrgManagementHandler) RemoveOrgMember(c *gin.Context) {
	actor, memberID, ok := orgManagementMemberActor(c)
	if !ok {
		return
	}
	var req removeOrgMemberRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, response.ErrBadRequest)
			return
		}
	}
	newPrimary, err := h.service.RemoveOrgMember(c.Request.Context(), actor, memberID, service.RemoveOrgMemberInput{NewPrimaryMemberID: req.NewPrimaryMemberID})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"removed": true, "memberId": memberID, "newPrimaryMemberId": newPrimary})
}

// orgManagementActor 从认证和租户中间件上下文构造当前租户组织管理调用者。
func orgManagementActor(c *gin.Context) (service.OrgManagementActor, bool) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return service.OrgManagementActor{}, false
	}
	tenantID, ok := middleware.CurrentTenantID(c)
	if !ok || tenantID == 0 {
		response.Fail(c, response.ErrTenantIDMissing)
		return service.OrgManagementActor{}, false
	}
	value, _ := c.Get(middleware.ContextTenantRoles)
	roles, _ := value.([]domain.RoleCode)
	return service.OrgManagementActor{UserID: userID, TenantID: tenantID, Roles: roles}, true
}

// orgManagementUnitActor 解析当前租户组织管理 actor 和路径中的部门 ID。
func orgManagementUnitActor(c *gin.Context) (service.OrgManagementActor, uint64, bool) {
	actor, ok := orgManagementActor(c)
	if !ok {
		return service.OrgManagementActor{}, 0, false
	}
	id, ok := uintParam(c, "id")
	if !ok {
		return service.OrgManagementActor{}, 0, false
	}
	return actor, id, true
}

// orgManagementMemberActor 解析当前租户组织管理 actor 和路径中的成员关系 ID。
func orgManagementMemberActor(c *gin.Context) (service.OrgManagementActor, uint64, bool) {
	actor, ok := orgManagementActor(c)
	if !ok {
		return service.OrgManagementActor{}, 0, false
	}
	id, ok := uintParam(c, "id")
	if !ok {
		return service.OrgManagementActor{}, 0, false
	}
	return actor, id, true
}
