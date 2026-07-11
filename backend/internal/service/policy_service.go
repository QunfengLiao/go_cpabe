package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/policytree"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// PolicyService 负责访问策略属性、模板和 DATA_OWNER 策略的业务规则与安全边界。
type PolicyService struct {
	policies repository.PolicyRepository
	tenants  repository.TenantRepository
	orgs     repository.OrgAttributeRepository
	authz    *AuthorizationService
}

// NewPolicyService 创建访问策略服务，依赖策略仓储和租户仓储完成数据读写与权限判断。
func NewPolicyService(policies repository.PolicyRepository, tenants repository.TenantRepository) *PolicyService {
	return &PolicyService{policies: policies, tenants: tenants}
}

// SetOrgAttributeRepository 注入租户级属性仓储，让 DATA_OWNER 策略保存使用真实租户属性校验。
func (s *PolicyService) SetOrgAttributeRepository(orgs repository.OrgAttributeRepository) {
	s.orgs = orgs
}

// SetAuthorizationService 注入统一授权服务，策略 Service 仍保留 owner 和租户归属等业务边界。
func (s *PolicyService) SetAuthorizationService(authz *AuthorizationService) {
	s.authz = authz
}

// PolicyAttributeInput 表示平台管理员创建或更新属性字典时允许提交的字段。
type PolicyAttributeInput struct {
	AttrCode    string
	AttrName    string
	AttrType    domain.PolicyAttributeType
	AttrValues  []string
	Description string
	Status      domain.PolicyStatus
}

// PolicyTemplateInput 表示平台管理员创建或更新策略模板时允许提交的字段。
type PolicyTemplateInput struct {
	Name           string
	Description    string
	PolicyTreeJSON json.RawMessage
	Status         domain.PolicyStatus
}

// AccessPolicyInput 表示 DATA_OWNER 创建或更新访问策略时允许提交的字段。
type AccessPolicyInput struct {
	Name           string
	Description    string
	PolicyExpr     string
	PolicyTreeJSON json.RawMessage
	Status         domain.PolicyStatus
}

var demoPolicyAttributes = []PolicyAttributeInput{
	{AttrCode: "role", AttrName: "租户角色", AttrType: domain.PolicyAttributeEnum, AttrValues: []string{"DATA_OWNER", "TENANT_ADMIN", "DATA_VISITOR"}, Description: "演示租户内业务角色，供 DATA_OWNER 构建访问策略", Status: domain.PolicyStatusEnabled},
	{AttrCode: "department", AttrName: "部门", AttrType: domain.PolicyAttributeEnum, AttrValues: []string{"研发部", "财务部", "安全部"}, Description: "演示组织部门属性，供访问树叶子节点引用", Status: domain.PolicyStatusEnabled},
	{AttrCode: "security_level", AttrName: "安全等级", AttrType: domain.PolicyAttributeNumber, Description: "演示数值型安全等级属性", Status: domain.PolicyStatusEnabled},
}

var demoPolicyTemplateTree = json.RawMessage(`{"type":"OR","children":[{"type":"AND","children":[{"type":"LEAF","attribute":"department","operator":"=","value":"研发部"},{"type":"LEAF","attribute":"role","operator":"=","value":"DATA_OWNER"}]},{"type":"LEAF","attribute":"role","operator":"=","value":"TENANT_ADMIN"}]}`)

// PolicyActor 表示从认证和租户中间件得到的调用方上下文。
type PolicyActor struct {
	UserID   uint64
	TenantID uint64
	Roles    []domain.RoleCode
}

// BootstrapDemoPolicyCatalog 幂等补齐演示环境可用的策略属性和模板。
//
// 访问树保存时以后端属性字典为权威边界；如果开发环境没有任何启用属性，前端示例树会因为
// 引用未开放属性而被拒绝。该方法只创建缺失的演示数据，不覆盖平台管理员已经维护的属性，
// 避免启动过程破坏人工配置。
func (s *PolicyService) BootstrapDemoPolicyCatalog(ctx context.Context) error {
	for _, input := range demoPolicyAttributes {
		if _, err := s.CreateAttribute(ctx, input); err != nil && !errors.Is(err, response.ErrPolicyAttributeCodeExists) {
			return err
		}
	}
	templates, err := s.ListTemplates(ctx, false)
	if err != nil {
		return err
	}
	for _, template := range templates {
		if template.Name == "数据拥有者或租户管理员可访问" {
			return nil
		}
	}
	_, err = s.CreateTemplate(ctx, PolicyTemplateInput{
		Name:           "数据拥有者或租户管理员可访问",
		Description:    "常用演示模板：研发部 DATA_OWNER 或 TENANT_ADMIN 可访问",
		PolicyTreeJSON: demoPolicyTemplateTree,
		Status:         domain.PolicyStatusEnabled,
	})
	return err
}

// ListAttributes 返回属性字典；onlyEnabled 为 true 时用于 DATA_OWNER 构建入口。
func (s *PolicyService) ListAttributes(ctx context.Context, onlyEnabled bool) ([]domain.PolicyAttribute, error) {
	return s.policies.ListAttributes(ctx, onlyEnabled)
}

// CreateAttribute 校验平台属性字段后写入属性字典。
func (s *PolicyService) CreateAttribute(ctx context.Context, input PolicyAttributeInput) (domain.PolicyAttribute, error) {
	attr, err := s.buildAttribute(input)
	if err != nil {
		return domain.PolicyAttribute{}, err
	}
	if err := s.policies.CreateAttribute(ctx, &attr); err != nil {
		if errors.Is(err, repository.ErrPolicyAttributeCodeExists) {
			return domain.PolicyAttribute{}, response.ErrPolicyAttributeCodeExists
		}
		return domain.PolicyAttribute{}, err
	}
	return attr, nil
}

// UpdateAttribute 校验并更新属性字典；禁用属性后既有策略会在后续校验中提示不可用。
func (s *PolicyService) UpdateAttribute(ctx context.Context, id uint64, input PolicyAttributeInput) (domain.PolicyAttribute, error) {
	attr, err := s.policies.FindAttributeByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrPolicyAttributeNotFound) {
			return domain.PolicyAttribute{}, response.ErrPolicyAttributeInvalid
		}
		return domain.PolicyAttribute{}, err
	}
	next, err := s.buildAttribute(input)
	if err != nil {
		return domain.PolicyAttribute{}, err
	}
	attr.AttrCode = next.AttrCode
	attr.AttrName = next.AttrName
	attr.AttrType = next.AttrType
	attr.AttrValues = next.AttrValues
	attr.Description = next.Description
	attr.Status = next.Status
	if err := s.policies.UpdateAttribute(ctx, attr); err != nil {
		if errors.Is(err, repository.ErrPolicyAttributeCodeExists) {
			return domain.PolicyAttribute{}, response.ErrPolicyAttributeCodeExists
		}
		return domain.PolicyAttribute{}, err
	}
	return *attr, nil
}

// DeleteAttribute 软删除属性字典，防止 DATA_OWNER 继续在新策略中使用该属性。
func (s *PolicyService) DeleteAttribute(ctx context.Context, id uint64) error {
	if err := s.policies.DeleteAttribute(ctx, id); err != nil {
		if errors.Is(err, repository.ErrPolicyAttributeNotFound) {
			return response.ErrPolicyAttributeInvalid
		}
		return err
	}
	return nil
}

// ListTemplates 返回策略模板列表；onlyEnabled 为 true 时用于 DATA_OWNER 构建入口。
func (s *PolicyService) ListTemplates(ctx context.Context, onlyEnabled bool) ([]domain.PolicyTemplate, error) {
	return s.policies.ListTemplates(ctx, onlyEnabled)
}

// TemplateDetail 返回策略模板详情。
func (s *PolicyService) TemplateDetail(ctx context.Context, id uint64) (domain.PolicyTemplate, error) {
	template, err := s.policies.FindTemplateByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrPolicyTemplateNotFound) {
			return domain.PolicyTemplate{}, response.ErrPolicyTemplateInvalid
		}
		return domain.PolicyTemplate{}, err
	}
	return *template, nil
}

// CreateTemplate 校验模板访问树并由后端生成标准表达式。
func (s *PolicyService) CreateTemplate(ctx context.Context, input PolicyTemplateInput) (domain.PolicyTemplate, error) {
	template, err := s.buildTemplate(ctx, input)
	if err != nil {
		return domain.PolicyTemplate{}, err
	}
	if err := s.policies.CreateTemplate(ctx, &template); err != nil {
		return domain.PolicyTemplate{}, err
	}
	return template, nil
}

// UpdateTemplate 更新模板访问树和元数据，表达式始终由后端重新生成。
func (s *PolicyService) UpdateTemplate(ctx context.Context, id uint64, input PolicyTemplateInput) (domain.PolicyTemplate, error) {
	existing, err := s.policies.FindTemplateByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrPolicyTemplateNotFound) {
			return domain.PolicyTemplate{}, response.ErrPolicyTemplateInvalid
		}
		return domain.PolicyTemplate{}, err
	}
	next, err := s.buildTemplate(ctx, input)
	if err != nil {
		return domain.PolicyTemplate{}, err
	}
	existing.Name = next.Name
	existing.Description = next.Description
	existing.PolicyExpr = next.PolicyExpr
	existing.PolicyTreeJSON = next.PolicyTreeJSON
	existing.Status = next.Status
	if err := s.policies.UpdateTemplate(ctx, existing); err != nil {
		return domain.PolicyTemplate{}, err
	}
	return *existing, nil
}

// DeleteTemplate 软删除模板；已创建的访问策略不依赖模板继续存在。
func (s *PolicyService) DeleteTemplate(ctx context.Context, id uint64) error {
	if err := s.policies.DeleteTemplate(ctx, id); err != nil {
		if errors.Is(err, repository.ErrPolicyTemplateNotFound) {
			return response.ErrPolicyTemplateInvalid
		}
		return err
	}
	return nil
}

// ListAccessPolicies 使用 policy.read 授权读取当前租户策略列表，租户边界仍由仓储查询保证。
func (s *PolicyService) ListAccessPolicies(ctx context.Context, actor PolicyActor, status domain.PolicyStatus, keyword string) ([]domain.AccessPolicy, error) {
	if !s.hasTenantPermission(ctx, actor, "policy.read") {
		return nil, response.ErrAccessPolicyForbidden
	}
	input := repository.ListAccessPoliciesInput{TenantID: actor.TenantID, Status: status, Keyword: keyword}
	if s.authz == nil && hasRole(actor.Roles, domain.RoleDO) && !hasRole(actor.Roles, domain.RoleTenantAdmin) {
		input.OwnerID = actor.UserID
	}
	return s.policies.ListAccessPolicies(ctx, input)
}

// CreateAccessPolicy 保存当前用户创建的访问策略；tenant_id 和 owner_id 只来自后端上下文。
func (s *PolicyService) CreateAccessPolicy(ctx context.Context, actor PolicyActor, input AccessPolicyInput) (domain.AccessPolicy, error) {
	if !s.hasTenantPermission(ctx, actor, "policy.write") {
		return domain.AccessPolicy{}, response.ErrAccessPolicyForbidden
	}
	policy, err := s.buildAccessPolicy(ctx, actor.TenantID, input)
	if err != nil {
		return domain.AccessPolicy{}, err
	}
	policy.TenantID = actor.TenantID
	policy.OwnerID = actor.UserID
	if err := s.policies.CreateAccessPolicy(ctx, &policy); err != nil {
		return domain.AccessPolicy{}, err
	}
	return policy, nil
}

// AccessPolicyDetail 使用 policy.read 授权读取当前租户内策略详情。
func (s *PolicyService) AccessPolicyDetail(ctx context.Context, actor PolicyActor, policyID uint64) (domain.AccessPolicy, error) {
	if !s.hasTenantPermission(ctx, actor, "policy.read") {
		return domain.AccessPolicy{}, response.ErrAccessPolicyForbidden
	}
	return s.findTenantPolicy(ctx, actor.TenantID, policyID)
}

// UpdateAccessPolicy 使用 policy.write 授权更新当前租户策略，并保留策略租户归属校验。
func (s *PolicyService) UpdateAccessPolicy(ctx context.Context, actor PolicyActor, policyID uint64, input AccessPolicyInput) (domain.AccessPolicy, error) {
	if !s.hasTenantPermission(ctx, actor, "policy.write") {
		return domain.AccessPolicy{}, response.ErrAccessPolicyForbidden
	}
	existing, err := s.findTenantPolicy(ctx, actor.TenantID, policyID)
	if err != nil {
		return domain.AccessPolicy{}, err
	}
	next, err := s.buildAccessPolicy(ctx, actor.TenantID, input)
	if err != nil {
		return domain.AccessPolicy{}, err
	}
	existing.Name = next.Name
	existing.Description = next.Description
	existing.PolicyExpr = next.PolicyExpr
	existing.PolicyTreeJSON = next.PolicyTreeJSON
	existing.Status = next.Status
	if err := s.policies.UpdateAccessPolicy(ctx, &existing); err != nil {
		return domain.AccessPolicy{}, err
	}
	return existing, nil
}

// DeleteAccessPolicy 使用 policy.write 授权删除当前租户策略，并通过租户范围查询防止跨租户越权。
func (s *PolicyService) DeleteAccessPolicy(ctx context.Context, actor PolicyActor, policyID uint64) error {
	if !s.hasTenantPermission(ctx, actor, "policy.write") {
		return response.ErrAccessPolicyForbidden
	}
	policy, err := s.findTenantPolicy(ctx, actor.TenantID, policyID)
	if err != nil {
		return err
	}
	if err := s.policies.DeleteAccessPolicyForOwner(ctx, actor.TenantID, policy.OwnerID, policyID); err != nil {
		if errors.Is(err, repository.ErrAccessPolicyNotFound) {
			return response.ErrAccessPolicyNotFound
		}
		return err
	}
	return nil
}

// buildAttribute 统一清洗属性字典输入，避免 handler 层散落规则。
func (s *PolicyService) buildAttribute(input PolicyAttributeInput) (domain.PolicyAttribute, error) {
	code := strings.TrimSpace(input.AttrCode)
	name := strings.TrimSpace(input.AttrName)
	attrType := input.AttrType
	status := normalizePolicyStatus(input.Status)
	if code == "" || name == "" || !attrType.Valid() || !status.Valid() {
		return domain.PolicyAttribute{}, response.ErrPolicyAttributeInvalid
	}
	values := normalizeStringSlice(input.AttrValues)
	if attrType == domain.PolicyAttributeEnum && len(values) == 0 {
		return domain.PolicyAttribute{}, response.ErrPolicyAttributeInvalid
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return domain.PolicyAttribute{}, response.ErrPolicyAttributeInvalid
	}
	return domain.PolicyAttribute{
		AttrCode:    code,
		AttrName:    name,
		AttrType:    attrType,
		AttrValues:  domain.JSONText(raw),
		Description: strings.TrimSpace(input.Description),
		Status:      status,
	}, nil
}

// buildTemplate 校验模板树并生成标准表达式。
func (s *PolicyService) buildTemplate(ctx context.Context, input PolicyTemplateInput) (domain.PolicyTemplate, error) {
	name := strings.TrimSpace(input.Name)
	status := normalizePolicyStatus(input.Status)
	if name == "" || !status.Valid() {
		return domain.PolicyTemplate{}, response.ErrPolicyTemplateInvalid
	}
	_, canonical, expr, err := s.validateTreeJSON(ctx, 0, input.PolicyTreeJSON)
	if err != nil {
		return domain.PolicyTemplate{}, err
	}
	return domain.PolicyTemplate{
		Name:           name,
		Description:    strings.TrimSpace(input.Description),
		PolicyExpr:     expr,
		PolicyTreeJSON: domain.JSONText(canonical),
		Status:         status,
	}, nil
}

// buildAccessPolicy 校验 DATA_OWNER 策略输入并生成标准访问树 JSON 和表达式。
func (s *PolicyService) buildAccessPolicy(ctx context.Context, tenantID uint64, input AccessPolicyInput) (domain.AccessPolicy, error) {
	name := strings.TrimSpace(input.Name)
	status := normalizePolicyStatus(input.Status)
	if name == "" || !status.Valid() {
		return domain.AccessPolicy{}, response.ErrBadRequest
	}
	_, canonical, expr, err := s.validateTreeJSON(ctx, tenantID, input.PolicyTreeJSON)
	if err != nil {
		return domain.AccessPolicy{}, err
	}
	return domain.AccessPolicy{
		Name:           name,
		Description:    strings.TrimSpace(input.Description),
		PolicyExpr:     expr,
		PolicyTreeJSON: domain.JSONText(canonical),
		Status:         status,
	}, nil
}

// validateTreeJSON 使用当前启用属性字典校验访问树，并返回标准 JSON 和表达式。
func (s *PolicyService) validateTreeJSON(ctx context.Context, tenantID uint64, raw json.RawMessage) (policytree.Node, []byte, string, error) {
	tree, err := policytree.Parse(raw)
	if err != nil {
		return policytree.Node{}, nil, "", response.ErrAccessPolicyTreeInvalid
	}
	attrs, err := s.enabledAttributeMetas(ctx, tenantID)
	if err != nil {
		return policytree.Node{}, nil, "", err
	}
	if err := policytree.Validate(tree, attrs); err != nil {
		return policytree.Node{}, nil, "", response.ErrAccessPolicyTreeInvalid
	}
	canonical, err := policytree.MarshalCanonical(tree)
	if err != nil {
		return policytree.Node{}, nil, "", err
	}
	return tree, canonical, policytree.GenerateExpression(tree), nil
}

// enabledAttributeMetas 将平台属性字典转换为访问树校验所需的只读元数据。
func (s *PolicyService) enabledAttributeMetas(ctx context.Context, tenantID uint64) (map[string]policytree.AttributeMeta, error) {
	if tenantID != 0 && s.orgs != nil {
		attrs, err := s.orgs.ListTenantAttributes(ctx, tenantID, true)
		if err != nil {
			return nil, err
		}
		ids := make([]uint64, 0, len(attrs))
		for _, attr := range attrs {
			ids = append(ids, attr.ID)
		}
		values, err := s.orgs.ListTenantAttributeValues(ctx, tenantID, ids, true)
		if err != nil {
			return nil, err
		}
		valuesByAttr := map[uint64][]string{}
		valueMetasByAttr := map[uint64]map[string]policytree.AttributeValueMeta{}
		for _, value := range values {
			valuesByAttr[value.AttributeID] = append(valuesByAttr[value.AttributeID], value.ValueCode)
			if valueMetasByAttr[value.AttributeID] == nil {
				valueMetasByAttr[value.AttributeID] = map[string]policytree.AttributeValueMeta{}
			}
			valueMetasByAttr[value.AttributeID][value.ValueCode] = policytree.AttributeValueMeta{ID: value.ID, Code: value.ValueCode, Path: value.ValuePath}
		}
		metas := make(map[string]policytree.AttributeMeta, len(attrs))
		for _, attr := range attrs {
			metas[attr.AttrCode] = policytree.AttributeMeta{Code: attr.AttrCode, Type: attr.AttrType, Values: valuesByAttr[attr.ID], ValuesByCode: valueMetasByAttr[attr.ID], Status: attr.Status}
		}
		return metas, nil
	}
	attrs, err := s.policies.ListAttributes(ctx, true)
	if err != nil {
		return nil, err
	}
	metas := make(map[string]policytree.AttributeMeta, len(attrs))
	for _, attr := range attrs {
		values := []string{}
		if len(attr.AttrValues) > 0 {
			_ = json.Unmarshal(attr.AttrValues, &values)
		}
		metas[attr.AttrCode] = policytree.AttributeMeta{Code: attr.AttrCode, Type: attr.AttrType, Values: values, Status: attr.Status}
	}
	return metas, nil
}

// findTenantPolicy 将仓储未找到错误转换为对外统一错误。
func (s *PolicyService) findTenantPolicy(ctx context.Context, tenantID, policyID uint64) (domain.AccessPolicy, error) {
	policy, err := s.policies.FindAccessPolicy(ctx, tenantID, policyID)
	if err != nil {
		if errors.Is(err, repository.ErrAccessPolicyNotFound) {
			return domain.AccessPolicy{}, response.ErrAccessPolicyNotFound
		}
		return domain.AccessPolicy{}, err
	}
	return *policy, nil
}

// findOwnerPolicy 通过 owner 范围查询策略，避免 DATA_OWNER 看到或修改他人策略。
func (s *PolicyService) findOwnerPolicy(ctx context.Context, tenantID, ownerID, policyID uint64) (domain.AccessPolicy, error) {
	policy, err := s.policies.FindAccessPolicyForOwner(ctx, tenantID, ownerID, policyID)
	if err != nil {
		if errors.Is(err, repository.ErrAccessPolicyNotFound) {
			return domain.AccessPolicy{}, response.ErrAccessPolicyNotFound
		}
		return domain.AccessPolicy{}, err
	}
	return *policy, nil
}

// hasTenantPermission 使用数据库 RBAC 判断功能权限；测试或迁移未注入授权服务时回退到旧角色语义。
func (s *PolicyService) hasTenantPermission(ctx context.Context, actor PolicyActor, code string) bool {
	if s != nil && s.authz != nil {
		ok, err := s.authz.HasTenantPermission(ctx, actor.UserID, actor.TenantID, code)
		return err == nil && ok
	}
	switch code {
	case "policy.read":
		return hasRole(actor.Roles, domain.RoleDO) || hasRole(actor.Roles, domain.RoleTenantAdmin)
	case "policy.write":
		return hasRole(actor.Roles, domain.RoleDO)
	default:
		return false
	}
}

// hasRole 判断角色列表是否包含指定角色。
func hasRole(roles []domain.RoleCode, expected domain.RoleCode) bool {
	for _, role := range roles {
		if role == expected {
			return true
		}
	}
	return false
}

// normalizePolicyStatus 为空时使用 enabled，保持创建流程符合演示系统默认开启习惯。
func normalizePolicyStatus(status domain.PolicyStatus) domain.PolicyStatus {
	if status == "" {
		return domain.PolicyStatusEnabled
	}
	return status
}

// normalizeStringSlice 清理字符串数组中的空值，避免 enum 可选值出现不可见空白。
func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
