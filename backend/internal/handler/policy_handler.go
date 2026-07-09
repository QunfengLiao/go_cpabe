package handler

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

// PolicyHandler 负责访问策略属性、模板和 DATA_OWNER 策略 HTTP 请求的参数绑定与响应封装。
type PolicyHandler struct {
	service *service.PolicyService
}

// NewPolicyHandler 创建访问策略 Handler。
func NewPolicyHandler(service *service.PolicyService) *PolicyHandler {
	return &PolicyHandler{service: service}
}

// policyAttributeRequest 是平台属性字典创建和更新请求体。
type policyAttributeRequest struct {
	AttrCode    string                     `json:"attrCode"`
	AttrName    string                     `json:"attrName"`
	AttrType    domain.PolicyAttributeType `json:"attrType"`
	AttrValues  []string                   `json:"attrValues"`
	Description string                     `json:"description"`
	Status      domain.PolicyStatus        `json:"status"`
}

// policyTemplateRequest 是平台策略模板创建和更新请求体。
type policyTemplateRequest struct {
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	PolicyTreeJSON json.RawMessage     `json:"policyTreeJson"`
	Status         domain.PolicyStatus `json:"status"`
}

// accessPolicyRequest 是 DATA_OWNER 创建和更新访问策略请求体。
type accessPolicyRequest struct {
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	PolicyExpr     string              `json:"policyExpr"`
	PolicyTreeJSON json.RawMessage     `json:"policyTreeJson"`
	Status         domain.PolicyStatus `json:"status"`
}

// ListAttributes 返回平台属性字典列表。
func (h *PolicyHandler) ListAttributes(c *gin.Context) {
	attrs, err := h.service.ListAttributes(c.Request.Context(), false)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": attrs, "total": len(attrs)})
}

// CreateAttribute 处理平台管理员创建属性字典请求。
func (h *PolicyHandler) CreateAttribute(c *gin.Context) {
	var req policyAttributeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	attr, err := h.service.CreateAttribute(c.Request.Context(), toAttributeInput(req))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, gin.H{"attribute": attr})
}

// UpdateAttribute 处理平台管理员更新属性字典请求。
func (h *PolicyHandler) UpdateAttribute(c *gin.Context) {
	id, ok := uintParam(c, "attributeId")
	if !ok {
		return
	}
	var req policyAttributeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	attr, err := h.service.UpdateAttribute(c.Request.Context(), id, toAttributeInput(req))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"attribute": attr})
}

// DeleteAttribute 处理平台管理员删除属性字典请求，底层使用软删除。
func (h *PolicyHandler) DeleteAttribute(c *gin.Context) {
	id, ok := uintParam(c, "attributeId")
	if !ok {
		return
	}
	if err := h.service.DeleteAttribute(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true, "attribute_id": id})
}

// ListTemplates 返回平台策略模板列表。
func (h *PolicyHandler) ListTemplates(c *gin.Context) {
	templates, err := h.service.ListTemplates(c.Request.Context(), false)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": templates, "total": len(templates)})
}

// CreateTemplate 处理平台管理员创建策略模板请求。
func (h *PolicyHandler) CreateTemplate(c *gin.Context) {
	var req policyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	template, err := h.service.CreateTemplate(c.Request.Context(), toTemplateInput(req))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, gin.H{"template": template})
}

// TemplateDetail 返回平台策略模板详情。
func (h *PolicyHandler) TemplateDetail(c *gin.Context) {
	id, ok := uintParam(c, "templateId")
	if !ok {
		return
	}
	template, err := h.service.TemplateDetail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"template": template})
}

// UpdateTemplate 处理平台管理员更新策略模板请求。
func (h *PolicyHandler) UpdateTemplate(c *gin.Context) {
	id, ok := uintParam(c, "templateId")
	if !ok {
		return
	}
	var req policyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	template, err := h.service.UpdateTemplate(c.Request.Context(), id, toTemplateInput(req))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"template": template})
}

// DeleteTemplate 处理平台管理员删除策略模板请求，已创建策略不受影响。
func (h *PolicyHandler) DeleteTemplate(c *gin.Context) {
	id, ok := uintParam(c, "templateId")
	if !ok {
		return
	}
	if err := h.service.DeleteTemplate(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true, "template_id": id})
}

// AvailableAttributes 返回 DATA_OWNER 可用于构建访问树的启用属性。
func (h *PolicyHandler) AvailableAttributes(c *gin.Context) {
	if _, ok := tenantActor(c); !ok {
		return
	}
	attrs, err := h.service.ListAttributes(c.Request.Context(), true)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": attrs, "total": len(attrs)})
}

// AvailableTemplates 返回 DATA_OWNER 可作为构建起点的启用策略模板。
func (h *PolicyHandler) AvailableTemplates(c *gin.Context) {
	if _, ok := tenantActor(c); !ok {
		return
	}
	templates, err := h.service.ListTemplates(c.Request.Context(), true)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": templates, "total": len(templates)})
}

// ListAccessPolicies 按当前租户角色返回访问策略列表。
func (h *PolicyHandler) ListAccessPolicies(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		return
	}
	status := domain.PolicyStatus(c.Query("status"))
	policies, err := h.service.ListAccessPolicies(c.Request.Context(), actor, status, c.Query("keyword"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"items": policies, "total": len(policies)})
}

// CreateAccessPolicy 处理 DATA_OWNER 创建访问策略请求。
func (h *PolicyHandler) CreateAccessPolicy(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		return
	}
	var req accessPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	policy, err := h.service.CreateAccessPolicy(c.Request.Context(), actor, toAccessPolicyInput(req))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, gin.H{"policy": policy})
}

// AccessPolicyDetail 返回访问策略详情，DATA_OWNER 和 TENANT_ADMIN 读取范围不同。
func (h *PolicyHandler) AccessPolicyDetail(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		return
	}
	policyID, ok := uintParam(c, "policyId")
	if !ok {
		return
	}
	policy, err := h.service.AccessPolicyDetail(c.Request.Context(), actor, policyID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"policy": policy})
}

// UpdateAccessPolicy 处理 DATA_OWNER 更新自己访问策略的请求。
func (h *PolicyHandler) UpdateAccessPolicy(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		return
	}
	policyID, ok := uintParam(c, "policyId")
	if !ok {
		return
	}
	var req accessPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	policy, err := h.service.UpdateAccessPolicy(c.Request.Context(), actor, policyID, toAccessPolicyInput(req))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"policy": policy})
}

// DeleteAccessPolicy 处理 DATA_OWNER 删除自己访问策略的请求。
func (h *PolicyHandler) DeleteAccessPolicy(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		return
	}
	policyID, ok := uintParam(c, "policyId")
	if !ok {
		return
	}
	if err := h.service.DeleteAccessPolicy(c.Request.Context(), actor, policyID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true, "policy_id": policyID})
}

// tenantActor 从认证和租户中间件上下文构造 PolicyActor，并校验路径租户与上下文一致。
func tenantActor(c *gin.Context) (service.PolicyActor, bool) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return service.PolicyActor{}, false
	}
	pathTenantID, ok := uintParam(c, "id")
	if !ok {
		return service.PolicyActor{}, false
	}
	contextTenantID, ok := middleware.CurrentTenantID(c)
	if !ok || contextTenantID != pathTenantID {
		response.Fail(c, response.ErrTenantPermissionDenied)
		return service.PolicyActor{}, false
	}
	value, _ := c.Get(middleware.ContextTenantRoles)
	roles, _ := value.([]domain.RoleCode)
	return service.PolicyActor{UserID: userID, TenantID: pathTenantID, Roles: roles}, true
}

// uintParam 解析无符号整数路径参数，失败时直接写入统一错误响应。
func uintParam(c *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, response.ErrBadRequest)
		return 0, false
	}
	return id, true
}

// toAttributeInput 将 HTTP 请求体转换为 service 输入，避免 handler 承担业务校验。
func toAttributeInput(req policyAttributeRequest) service.PolicyAttributeInput {
	return service.PolicyAttributeInput{AttrCode: req.AttrCode, AttrName: req.AttrName, AttrType: req.AttrType, AttrValues: req.AttrValues, Description: req.Description, Status: req.Status}
}

// toTemplateInput 将模板请求体转换为 service 输入。
func toTemplateInput(req policyTemplateRequest) service.PolicyTemplateInput {
	return service.PolicyTemplateInput{Name: req.Name, Description: req.Description, PolicyTreeJSON: req.PolicyTreeJSON, Status: req.Status}
}

// toAccessPolicyInput 将访问策略请求体转换为 service 输入。
func toAccessPolicyInput(req accessPolicyRequest) service.AccessPolicyInput {
	return service.AccessPolicyInput{Name: req.Name, Description: req.Description, PolicyExpr: req.PolicyExpr, PolicyTreeJSON: req.PolicyTreeJSON, Status: req.Status}
}
