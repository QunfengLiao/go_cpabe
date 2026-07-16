package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/repository"
)

// testPolicyRepo 是 handler 集成测试使用的线程安全内存访问策略仓储。
type testPolicyRepo struct {
	mu           sync.Mutex
	nextAttr     uint64
	nextTemplate uint64
	nextPolicy   uint64
	attrs        map[uint64]*domain.PolicyAttribute
	attrCodes    map[string]uint64
	templates    map[uint64]*domain.PolicyTemplate
	policies     map[uint64]*domain.AccessPolicy
}

// newTestPolicyRepo 创建 handler 测试用访问策略仓储。
func newTestPolicyRepo() *testPolicyRepo {
	return &testPolicyRepo{
		nextAttr:     1,
		nextTemplate: 1,
		nextPolicy:   1,
		attrs:        map[uint64]*domain.PolicyAttribute{},
		attrCodes:    map[string]uint64{},
		templates:    map[uint64]*domain.PolicyTemplate{},
		policies:     map[uint64]*domain.AccessPolicy{},
	}
}

// ListAttributes 返回属性字典列表，支持 DATA_OWNER 可用属性过滤。
func (r *testPolicyRepo) ListAttributes(_ context.Context, onlyEnabled bool) ([]domain.PolicyAttribute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	attrs := make([]domain.PolicyAttribute, 0, len(r.attrs))
	for _, attr := range r.attrs {
		if onlyEnabled && attr.Status != domain.PolicyStatusEnabled {
			continue
		}
		attrs = append(attrs, *attr)
	}
	sort.Slice(attrs, func(i, j int) bool { return attrs[i].ID < attrs[j].ID })
	return attrs, nil
}

// FindAttributeByID 按 ID 查找属性并返回副本，避免测试篡改内部状态。
func (r *testPolicyRepo) FindAttributeByID(_ context.Context, id uint64) (*domain.PolicyAttribute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	attr, ok := r.attrs[id]
	if !ok {
		return nil, repository.ErrPolicyAttributeNotFound
	}
	copy := *attr
	return &copy, nil
}

// FindAttributeByCode 按属性编码查找属性，供访问树校验流程使用。
func (r *testPolicyRepo) FindAttributeByCode(_ context.Context, code string) (*domain.PolicyAttribute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.attrCodes[code]
	if !ok {
		return nil, repository.ErrPolicyAttributeNotFound
	}
	copy := *r.attrs[id]
	return &copy, nil
}

// CreateAttribute 写入属性并模拟唯一编码约束。
func (r *testPolicyRepo) CreateAttribute(_ context.Context, attr *domain.PolicyAttribute) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.attrCodes[attr.AttrCode]; exists {
		return repository.ErrPolicyAttributeCodeExists
	}
	attr.ID = r.nextAttr
	r.nextAttr++
	now := time.Now().UTC()
	attr.CreatedAt = now
	attr.UpdatedAt = now
	copy := *attr
	r.attrs[attr.ID] = &copy
	r.attrCodes[attr.AttrCode] = attr.ID
	return nil
}

// UpdateAttribute 更新属性并维护 attr_code 索引。
func (r *testPolicyRepo) UpdateAttribute(_ context.Context, attr *domain.PolicyAttribute) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.attrs[attr.ID]
	if !ok {
		return repository.ErrPolicyAttributeNotFound
	}
	if id, exists := r.attrCodes[attr.AttrCode]; exists && id != attr.ID {
		return repository.ErrPolicyAttributeCodeExists
	}
	delete(r.attrCodes, existing.AttrCode)
	attr.UpdatedAt = time.Now().UTC()
	copy := *attr
	r.attrs[attr.ID] = &copy
	r.attrCodes[attr.AttrCode] = attr.ID
	return nil
}

// DeleteAttribute 删除属性记录，测试中用物理删除模拟软删除不可见结果。
func (r *testPolicyRepo) DeleteAttribute(_ context.Context, id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	attr, ok := r.attrs[id]
	if !ok {
		return repository.ErrPolicyAttributeNotFound
	}
	delete(r.attrCodes, attr.AttrCode)
	delete(r.attrs, id)
	return nil
}

// ListTemplates 返回策略模板列表，并按启用状态过滤 DATA_OWNER 可用模板。
func (r *testPolicyRepo) ListTemplates(_ context.Context, onlyEnabled bool) ([]domain.PolicyTemplate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	templates := make([]domain.PolicyTemplate, 0, len(r.templates))
	for _, template := range r.templates {
		if onlyEnabled && template.Status != domain.PolicyStatusEnabled {
			continue
		}
		templates = append(templates, *template)
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].ID < templates[j].ID })
	return templates, nil
}

// FindTemplateByID 按 ID 查找模板。
func (r *testPolicyRepo) FindTemplateByID(_ context.Context, id uint64) (*domain.PolicyTemplate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	template, ok := r.templates[id]
	if !ok {
		return nil, repository.ErrPolicyTemplateNotFound
	}
	copy := *template
	return &copy, nil
}

// CreateTemplate 写入策略模板并模拟数据库生成主键。
func (r *testPolicyRepo) CreateTemplate(_ context.Context, template *domain.PolicyTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	template.ID = r.nextTemplate
	r.nextTemplate++
	now := time.Now().UTC()
	template.CreatedAt = now
	template.UpdatedAt = now
	copy := *template
	r.templates[template.ID] = &copy
	return nil
}

// UpdateTemplate 更新模板记录。
func (r *testPolicyRepo) UpdateTemplate(_ context.Context, template *domain.PolicyTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.templates[template.ID]; !ok {
		return repository.ErrPolicyTemplateNotFound
	}
	template.UpdatedAt = time.Now().UTC()
	copy := *template
	r.templates[template.ID] = &copy
	return nil
}

// DeleteTemplate 删除模板记录，已创建策略在测试中不依赖模板继续存在。
func (r *testPolicyRepo) DeleteTemplate(_ context.Context, id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.templates[id]; !ok {
		return repository.ErrPolicyTemplateNotFound
	}
	delete(r.templates, id)
	return nil
}

// ListAccessPolicies 按租户、owner、状态和关键字返回策略列表。
func (r *testPolicyRepo) ListAccessPolicies(_ context.Context, input repository.ListAccessPoliciesInput) ([]domain.AccessPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	policies := make([]domain.AccessPolicy, 0, len(r.policies))
	for _, policy := range r.policies {
		if policy.TenantID != input.TenantID {
			continue
		}
		if input.OwnerID != 0 && policy.OwnerID != input.OwnerID {
			continue
		}
		if input.Status.Valid() && policy.Status != input.Status {
			continue
		}
		if keyword := strings.TrimSpace(input.Keyword); keyword != "" && !strings.Contains(policy.Name, keyword) {
			continue
		}
		policies = append(policies, *policy)
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i].ID > policies[j].ID })
	return policies, nil
}

// FindAccessPolicy 按租户和策略 ID 查找策略，供只读详情接口使用。
func (r *testPolicyRepo) FindAccessPolicy(_ context.Context, tenantID, policyID uint64) (*domain.AccessPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	policy, ok := r.policies[policyID]
	if !ok || policy.TenantID != tenantID {
		return nil, repository.ErrAccessPolicyNotFound
	}
	copy := *policy
	return &copy, nil
}

// FindAccessPolicyForOwner 按租户、owner 和策略 ID 查找策略，防止 DATA_OWNER 越权。
func (r *testPolicyRepo) FindAccessPolicyForOwner(_ context.Context, tenantID, ownerID, policyID uint64) (*domain.AccessPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	policy, ok := r.policies[policyID]
	if !ok || policy.TenantID != tenantID || policy.OwnerID != ownerID {
		return nil, repository.ErrAccessPolicyNotFound
	}
	copy := *policy
	return &copy, nil
}

// CreateAccessPolicy 写入租户访问策略并分配 ID。
func (r *testPolicyRepo) CreateAccessPolicy(_ context.Context, policy *domain.AccessPolicy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	policy.ID = r.nextPolicy
	r.nextPolicy++
	now := time.Now().UTC()
	policy.CreatedAt = now
	policy.UpdatedAt = now
	copy := *policy
	r.policies[policy.ID] = &copy
	return nil
}

// UpdateAccessPolicy 更新访问策略，owner 校验由 service 保证。
func (r *testPolicyRepo) UpdateAccessPolicy(_ context.Context, policy *domain.AccessPolicy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.policies[policy.ID]; !ok {
		return repository.ErrAccessPolicyNotFound
	}
	policy.UpdatedAt = time.Now().UTC()
	copy := *policy
	r.policies[policy.ID] = &copy
	return nil
}

// DeleteAccessPolicyForOwner 删除指定 owner 范围内的策略。
func (r *testPolicyRepo) DeleteAccessPolicyForOwner(_ context.Context, tenantID, ownerID, policyID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	policy, ok := r.policies[policyID]
	if !ok || policy.TenantID != tenantID || policy.OwnerID != ownerID {
		return repository.ErrAccessPolicyNotFound
	}
	delete(r.policies, policyID)
	return nil
}

// TestPolicyHandlerDataOwnerCreateAndDetail 验证 DATA_OWNER 可创建访问策略并读取自己的详情。
func TestPolicyHandlerDataOwnerCreateAndDetail(t *testing.T) {
	app := newTestApp()
	seedPolicyRoleAttribute(t, app)
	_, ownerAccess := createPolicyMemberAndLogin(t, app, 1, "policy-owner@example.com", domain.RoleDO)

	created := performJSONWithTenant(app.router, http.MethodPost, "/api/v1/tenants/1/access-policies", map[string]any{
		"name":           "研发资料访问策略",
		"policyExpr":     "client expr should be ignored",
		"policyTreeJson": validPolicyTreeObject(),
	}, ownerAccess, 1)
	if created.Code != http.StatusCreated || !bytesContains(created.Body.String(), "policy_expr") {
		t.Fatalf("create policy status=%d body=%s", created.Code, created.Body.String())
	}
	policyID := responsePolicyID(t, created)

	detail := performJSONWithTenant(app.router, http.MethodGet, "/api/v1/tenants/1/access-policies/"+strconv.FormatUint(policyID, 10), nil, ownerAccess, 1)
	if detail.Code != http.StatusOK || !bytesContains(detail.Body.String(), "研发资料访问策略") {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}
}

// TestPolicyHandlerPlatformAttributeAndTemplateLifecycle 验证平台管理员可管理属性字典和策略模板。
func TestPolicyHandlerPlatformAttributeAndTemplateLifecycle(t *testing.T) {
	app := newTestApp()
	platformAccess := createPlatformAdminAndLogin(t, app)

	attr := performJSON(app.router, http.MethodPost, "/api/v1/platform/policy-attributes", map[string]any{
		"attrCode": "role", "attrName": "角色", "attrType": "enum", "attrValues": []string{"DATA_OWNER", "TENANT_ADMIN"},
	}, platformAccess)
	if attr.Code != http.StatusCreated || !bytesContains(attr.Body.String(), "role") {
		t.Fatalf("create attr status=%d body=%s", attr.Code, attr.Body.String())
	}

	template := performJSON(app.router, http.MethodPost, "/api/v1/platform/policy-templates", map[string]any{
		"name": "基础访问模板", "policyTreeJson": validPolicyTreeObject(),
	}, platformAccess)
	if template.Code != http.StatusCreated || !bytesContains(template.Body.String(), "policy_expr") {
		t.Fatalf("create template status=%d body=%s", template.Code, template.Body.String())
	}
	list := performJSON(app.router, http.MethodGet, "/api/v1/platform/policy-templates", nil, platformAccess)
	if list.Code != http.StatusOK || !bytesContains(list.Body.String(), "基础访问模板") {
		t.Fatalf("list template status=%d body=%s", list.Code, list.Body.String())
	}
}

// TestPolicyHandlerTenantReadOnlyAndVisitorWriteDenied 验证 TENANT_ADMIN 只读和 DATA_VISITOR 禁止写入。
func TestPolicyHandlerTenantReadOnlyAndVisitorWriteDenied(t *testing.T) {
	app := newTestApp()
	seedPolicyRoleAttribute(t, app)
	_, ownerAccess := createPolicyMemberAndLogin(t, app, 1, "policy-owner-read@example.com", domain.RoleDO)
	_, visitorAccess := createPolicyMemberAndLogin(t, app, 1, "policy-visitor@example.com", domain.RoleDU)
	adminAccess := createAdminAndLogin(t, app)

	created := performJSONWithTenant(app.router, http.MethodPost, "/api/v1/tenants/1/access-policies", map[string]any{"name": "只读验证策略", "policyTreeJson": validPolicyTreeObject()}, ownerAccess, 1)
	if created.Code != http.StatusCreated {
		t.Fatalf("owner create status=%d body=%s", created.Code, created.Body.String())
	}
	policyID := responsePolicyID(t, created)

	list := performJSONWithTenant(app.router, http.MethodGet, "/api/v1/tenants/1/access-policies", nil, adminAccess, 1)
	if list.Code != http.StatusOK || !bytesContains(list.Body.String(), "只读验证策略") {
		t.Fatalf("tenant admin list status=%d body=%s", list.Code, list.Body.String())
	}
	update := performJSONWithTenant(app.router, http.MethodPut, "/api/v1/tenants/1/access-policies/"+strconv.FormatUint(policyID, 10), map[string]any{"name": "管理员写入", "policyTreeJson": validPolicyTreeObject()}, adminAccess, 1)
	if update.Code != http.StatusForbidden || !bytesContains(update.Body.String(), "ACCESS_POLICY_FORBIDDEN") {
		t.Fatalf("tenant admin write status=%d body=%s", update.Code, update.Body.String())
	}
	visitorCreate := performJSONWithTenant(app.router, http.MethodPost, "/api/v1/tenants/1/access-policies", map[string]any{"name": "访客写入", "policyTreeJson": validPolicyTreeObject()}, visitorAccess, 1)
	if visitorCreate.Code != http.StatusForbidden || !bytesContains(visitorCreate.Body.String(), "ACCESS_POLICY_FORBIDDEN") {
		t.Fatalf("visitor write status=%d body=%s", visitorCreate.Code, visitorCreate.Body.String())
	}
}

// TestPolicyHandlerRejectsCrossTenantAndPlatformTenantWrite 验证跨租户访问和平台管理员写租户策略被拒绝。
func TestPolicyHandlerRejectsCrossTenantAndPlatformTenantWrite(t *testing.T) {
	app := newTestApp()
	seedPolicyRoleAttribute(t, app)
	_, ownerAccess := createPolicyMemberAndLogin(t, app, 1, "policy-owner-boundary@example.com", domain.RoleDO)
	platformAccess := createPlatformAdminAndLogin(t, app)

	created := performJSONWithTenant(app.router, http.MethodPost, "/api/v1/tenants/1/access-policies", map[string]any{"name": "边界策略", "policyTreeJson": validPolicyTreeObject()}, ownerAccess, 1)
	if created.Code != http.StatusCreated {
		t.Fatalf("owner create status=%d body=%s", created.Code, created.Body.String())
	}
	policyID := responsePolicyID(t, created)

	mismatch := performJSONWithTenant(app.router, http.MethodGet, "/api/v1/tenants/2/access-policies/"+strconv.FormatUint(policyID, 10), nil, ownerAccess, 1)
	if mismatch.Code != http.StatusForbidden || !bytesContains(mismatch.Body.String(), "TENANT_PERMISSION_DENIED") {
		t.Fatalf("path/header tenant mismatch status=%d body=%s", mismatch.Code, mismatch.Body.String())
	}
	platformWrite := performJSONWithTenant(app.router, http.MethodPost, "/api/v1/tenants/1/access-policies", map[string]any{"name": "平台写入", "policyTreeJson": validPolicyTreeObject()}, platformAccess, 1)
	if platformWrite.Code != http.StatusForbidden || !bytesContains(platformWrite.Body.String(), "TENANT_MEMBER_FORBIDDEN") {
		t.Fatalf("platform tenant write status=%d body=%s", platformWrite.Code, platformWrite.Body.String())
	}
}

// createPolicyMemberAndLogin 创建租户成员、授予指定业务角色并返回用户 ID 和 access token。
func createPolicyMemberAndLogin(t *testing.T, app testApp, tenantID uint64, email string, role domain.RoleCode) (uint64, string) {
	t.Helper()
	hash, err := auth.HashPassword("Passw0rd!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &domain.User{Email: email, PasswordHash: hash, Nickname: "策略成员", Role: domain.RoleDataUser, Status: domain.StatusActive}
	if err := app.repo.Create(nil, user); err != nil {
		t.Fatalf("create policy member: %v", err)
	}
	if err := app.tenantRepo.EnsureTenantUser(nil, tenantID, user.ID, domain.TenantUserStatusActive); err != nil {
		t.Fatalf("tenant member: %v", err)
	}
	if err := app.tenantRepo.EnsureUserRole(nil, &tenantID, user.ID, role); err != nil {
		t.Fatalf("tenant role %s: %v", role, err)
	}
	return user.ID, loginExistingUser(t, app, email)
}

// seedPolicyRoleAttribute 直接写入启用角色属性，避免 DATA_OWNER 测试重复走平台管理接口。
func seedPolicyRoleAttribute(t *testing.T, app testApp) {
	t.Helper()
	err := app.policyRepo.CreateAttribute(context.Background(), &domain.PolicyAttribute{
		AttrCode:   "role",
		AttrName:   "角色",
		AttrType:   domain.PolicyAttributeEnum,
		AttrValues: domain.JSONText([]byte(`["DATA_OWNER","TENANT_ADMIN"]`)),
		Status:     domain.PolicyStatusEnabled,
	})
	if err != nil && !errors.Is(err, repository.ErrPolicyAttributeCodeExists) {
		t.Fatalf("seed role attr: %v", err)
	}
}

// validPolicyTreeObject 返回可被 JSON 请求编码的合法访问树对象。
func validPolicyTreeObject() map[string]any {
	return map[string]any{
		"type": "OR",
		"children": []map[string]any{
			{"type": "LEAF", "attribute": "role", "operator": "=", "value": "DATA_OWNER"},
			{"type": "LEAF", "attribute": "role", "operator": "=", "value": "TENANT_ADMIN"},
		},
	}
}

// responsePolicyID 从创建策略响应中提取 policy.id，供后续详情和更新测试复用。
func responsePolicyID(t *testing.T, recorder *httptest.ResponseRecorder) uint64 {
	t.Helper()
	data := parseData(t, recorder)
	policy, _ := data["policy"].(map[string]any)
	id, _ := policy["id"].(float64)
	if id == 0 {
		t.Fatalf("missing policy id: %s", recorder.Body.String())
	}
	return uint64(id)
}
