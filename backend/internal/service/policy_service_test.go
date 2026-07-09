package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// memoryPolicyRepo 是 PolicyService 单元测试使用的线程安全内存仓储。
type memoryPolicyRepo struct {
	mu           sync.Mutex
	nextAttr     uint64
	nextTemplate uint64
	nextPolicy   uint64
	attrs        map[uint64]*domain.PolicyAttribute
	attrCodes    map[string]uint64
	templates    map[uint64]*domain.PolicyTemplate
	policies     map[uint64]*domain.AccessPolicy
}

// newMemoryPolicyRepo 创建访问策略服务测试用内存仓储，避免测试依赖真实 MySQL。
func newMemoryPolicyRepo() *memoryPolicyRepo {
	return &memoryPolicyRepo{
		nextAttr:     1,
		nextTemplate: 1,
		nextPolicy:   1,
		attrs:        map[uint64]*domain.PolicyAttribute{},
		attrCodes:    map[string]uint64{},
		templates:    map[uint64]*domain.PolicyTemplate{},
		policies:     map[uint64]*domain.AccessPolicy{},
	}
}

// ListAttributes 返回内存属性字典，onlyEnabled 用于验证 DATA_OWNER 构建入口过滤。
func (r *memoryPolicyRepo) ListAttributes(_ context.Context, onlyEnabled bool) ([]domain.PolicyAttribute, error) {
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

// FindAttributeByID 按 ID 返回属性副本，模拟仓储未找到语义。
func (r *memoryPolicyRepo) FindAttributeByID(_ context.Context, id uint64) (*domain.PolicyAttribute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	attr, ok := r.attrs[id]
	if !ok {
		return nil, repository.ErrPolicyAttributeNotFound
	}
	copy := *attr
	return &copy, nil
}

// FindAttributeByCode 按属性编码查找属性，供访问树校验测试使用。
func (r *memoryPolicyRepo) FindAttributeByCode(_ context.Context, code string) (*domain.PolicyAttribute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.attrCodes[code]
	if !ok {
		return nil, repository.ErrPolicyAttributeNotFound
	}
	copy := *r.attrs[id]
	return &copy, nil
}

// CreateAttribute 写入属性并模拟 attr_code 唯一约束。
func (r *memoryPolicyRepo) CreateAttribute(_ context.Context, attr *domain.PolicyAttribute) error {
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

// UpdateAttribute 更新属性并继续维护属性编码唯一性。
func (r *memoryPolicyRepo) UpdateAttribute(_ context.Context, attr *domain.PolicyAttribute) error {
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

// DeleteAttribute 从内存仓储移除属性，表达软删除后的不可见状态。
func (r *memoryPolicyRepo) DeleteAttribute(_ context.Context, id uint64) error {
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

// ListTemplates 返回内存模板列表，onlyEnabled 用于 DATA_OWNER 可用模板过滤。
func (r *memoryPolicyRepo) ListTemplates(_ context.Context, onlyEnabled bool) ([]domain.PolicyTemplate, error) {
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

// FindTemplateByID 返回模板副本，避免测试直接修改仓储内部状态。
func (r *memoryPolicyRepo) FindTemplateByID(_ context.Context, id uint64) (*domain.PolicyTemplate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	template, ok := r.templates[id]
	if !ok {
		return nil, repository.ErrPolicyTemplateNotFound
	}
	copy := *template
	return &copy, nil
}

// CreateTemplate 写入策略模板并生成测试主键。
func (r *memoryPolicyRepo) CreateTemplate(_ context.Context, template *domain.PolicyTemplate) error {
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

// UpdateTemplate 更新策略模板，调用方负责访问树校验。
func (r *memoryPolicyRepo) UpdateTemplate(_ context.Context, template *domain.PolicyTemplate) error {
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

// DeleteTemplate 删除内存模板，模拟软删除后不再出现在查询中。
func (r *memoryPolicyRepo) DeleteTemplate(_ context.Context, id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.templates[id]; !ok {
		return repository.ErrPolicyTemplateNotFound
	}
	delete(r.templates, id)
	return nil
}

// ListAccessPolicies 按租户、owner、状态和关键字过滤访问策略。
func (r *memoryPolicyRepo) ListAccessPolicies(_ context.Context, input repository.ListAccessPoliciesInput) ([]domain.AccessPolicy, error) {
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

// FindAccessPolicy 按租户范围查找策略，供 TENANT_ADMIN 只读详情使用。
func (r *memoryPolicyRepo) FindAccessPolicy(_ context.Context, tenantID, policyID uint64) (*domain.AccessPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	policy, ok := r.policies[policyID]
	if !ok || policy.TenantID != tenantID {
		return nil, repository.ErrAccessPolicyNotFound
	}
	copy := *policy
	return &copy, nil
}

// FindAccessPolicyForOwner 按租户和 owner 范围查找策略，模拟 DATA_OWNER 安全边界。
func (r *memoryPolicyRepo) FindAccessPolicyForOwner(_ context.Context, tenantID, ownerID, policyID uint64) (*domain.AccessPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	policy, ok := r.policies[policyID]
	if !ok || policy.TenantID != tenantID || policy.OwnerID != ownerID {
		return nil, repository.ErrAccessPolicyNotFound
	}
	copy := *policy
	return &copy, nil
}

// CreateAccessPolicy 写入 DATA_OWNER 策略并分配测试主键。
func (r *memoryPolicyRepo) CreateAccessPolicy(_ context.Context, policy *domain.AccessPolicy) error {
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

// UpdateAccessPolicy 更新访问策略，调用方必须已经完成 owner 校验。
func (r *memoryPolicyRepo) UpdateAccessPolicy(_ context.Context, policy *domain.AccessPolicy) error {
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

// DeleteAccessPolicyForOwner 只删除指定 owner 范围内的访问策略。
func (r *memoryPolicyRepo) DeleteAccessPolicyForOwner(_ context.Context, tenantID, ownerID, policyID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	policy, ok := r.policies[policyID]
	if !ok || policy.TenantID != tenantID || policy.OwnerID != ownerID {
		return repository.ErrAccessPolicyNotFound
	}
	delete(r.policies, policyID)
	return nil
}

// TestPolicyServiceCreatesAccessPolicyForDataOwner 验证 DATA_OWNER 创建策略时服务端重算表达式并绑定后端上下文。
func TestPolicyServiceCreatesAccessPolicyForDataOwner(t *testing.T) {
	ctx := context.Background()
	svc := newPolicyServiceForTest(t, ctx)
	actor := PolicyActor{UserID: 7, TenantID: 3, Roles: []domain.RoleCode{domain.RoleDO}}

	policy, err := svc.CreateAccessPolicy(ctx, actor, AccessPolicyInput{
		Name:           "研发访问策略",
		PolicyExpr:     "client tampered expression",
		PolicyTreeJSON: validPolicyTreeJSON(),
	})
	if err != nil {
		t.Fatalf("create access policy: %v", err)
	}
	if policy.ID == 0 || policy.TenantID != actor.TenantID || policy.OwnerID != actor.UserID {
		t.Fatalf("policy should bind backend actor context: %+v", policy)
	}
	if policy.PolicyExpr == "client tampered expression" || policy.PolicyExpr == "" {
		t.Fatalf("service must regenerate policy expr, got %q", policy.PolicyExpr)
	}
	if policy.Status != domain.PolicyStatusEnabled {
		t.Fatalf("empty status should default enabled, got %s", policy.Status)
	}

	_, err = svc.CreateAccessPolicy(ctx, PolicyActor{UserID: 8, TenantID: 3, Roles: []domain.RoleCode{domain.RoleDU}}, AccessPolicyInput{Name: "访客策略", PolicyTreeJSON: validPolicyTreeJSON()})
	if !errors.Is(err, response.ErrAccessPolicyForbidden) {
		t.Fatalf("DATA_VISITOR should not create policy, got %v", err)
	}
}

// TestPolicyServiceManagesPlatformAttributes 验证平台属性字典校验、唯一性和启用过滤。
func TestPolicyServiceManagesPlatformAttributes(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryPolicyRepo()
	svc := NewPolicyService(repo, nil)

	attr, err := svc.CreateAttribute(ctx, PolicyAttributeInput{AttrCode: "role", AttrName: "角色", AttrType: domain.PolicyAttributeEnum, AttrValues: []string{"DATA_OWNER", "DATA_OWNER", "TENANT_ADMIN"}})
	if err != nil {
		t.Fatalf("create attr: %v", err)
	}
	if attr.Status != domain.PolicyStatusEnabled || string(attr.AttrValues) != `["DATA_OWNER","TENANT_ADMIN"]` {
		t.Fatalf("attribute should normalize status and enum values: %+v", attr)
	}
	if _, err := svc.CreateAttribute(ctx, PolicyAttributeInput{AttrCode: "role", AttrName: "重复", AttrType: domain.PolicyAttributeString}); !errors.Is(err, response.ErrPolicyAttributeCodeExists) {
		t.Fatalf("expected duplicate attr code, got %v", err)
	}
	if _, err := svc.CreateAttribute(ctx, PolicyAttributeInput{AttrCode: "level", AttrName: "级别", AttrType: domain.PolicyAttributeEnum}); !errors.Is(err, response.ErrPolicyAttributeInvalid) {
		t.Fatalf("enum without values should be invalid, got %v", err)
	}
	if _, err := svc.CreateAttribute(ctx, PolicyAttributeInput{AttrCode: "department", AttrName: "部门", AttrType: domain.PolicyAttributeString, Status: domain.PolicyStatusDisabled}); err != nil {
		t.Fatalf("create disabled attr: %v", err)
	}
	enabled, err := svc.ListAttributes(ctx, true)
	if err != nil {
		t.Fatalf("list enabled attrs: %v", err)
	}
	if len(enabled) != 1 || enabled[0].AttrCode != "role" {
		t.Fatalf("expected only enabled role attr, got %+v", enabled)
	}
}

// TestPolicyServiceManagesTemplates 验证模板保存复用访问树校验并按启用状态过滤。
func TestPolicyServiceManagesTemplates(t *testing.T) {
	ctx := context.Background()
	svc := newPolicyServiceForTest(t, ctx)

	template, err := svc.CreateTemplate(ctx, PolicyTemplateInput{Name: "基础模板", PolicyTreeJSON: validPolicyTreeJSON()})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if template.ID == 0 || template.PolicyExpr == "" || len(template.PolicyTreeJSON) == 0 {
		t.Fatalf("template should persist canonical tree and expr: %+v", template)
	}
	if _, err := svc.CreateTemplate(ctx, PolicyTemplateInput{Name: "坏模板", PolicyTreeJSON: []byte(`{"type":"AND","children":[{"type":"LEAF","attribute":"role","operator":"=","value":"DATA_OWNER"}]}`)}); !errors.Is(err, response.ErrAccessPolicyTreeInvalid) {
		t.Fatalf("invalid template tree should fail, got %v", err)
	}
	if _, err := svc.CreateTemplate(ctx, PolicyTemplateInput{Name: "停用模板", PolicyTreeJSON: validPolicyTreeJSON(), Status: domain.PolicyStatusDisabled}); err != nil {
		t.Fatalf("create disabled template: %v", err)
	}
	enabled, err := svc.ListTemplates(ctx, true)
	if err != nil {
		t.Fatalf("list enabled templates: %v", err)
	}
	if len(enabled) != 1 || enabled[0].Name != "基础模板" {
		t.Fatalf("expected only enabled template, got %+v", enabled)
	}
}

// TestPolicyServiceRestrictsOwnerUpdatesAndDeletes 验证 DATA_OWNER 只能更新和删除自己创建的策略。
func TestPolicyServiceRestrictsOwnerUpdatesAndDeletes(t *testing.T) {
	ctx := context.Background()
	svc := newPolicyServiceForTest(t, ctx)
	owner := PolicyActor{UserID: 7, TenantID: 3, Roles: []domain.RoleCode{domain.RoleDO}}
	otherOwner := PolicyActor{UserID: 8, TenantID: 3, Roles: []domain.RoleCode{domain.RoleDO}}

	policy, err := svc.CreateAccessPolicy(ctx, owner, AccessPolicyInput{Name: "原策略", PolicyTreeJSON: validPolicyTreeJSON()})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	updated, err := svc.UpdateAccessPolicy(ctx, owner, policy.ID, AccessPolicyInput{Name: "更新策略", PolicyTreeJSON: validPolicyTreeJSON(), Status: domain.PolicyStatusDisabled})
	if err != nil {
		t.Fatalf("owner update: %v", err)
	}
	if updated.Name != "更新策略" || updated.Status != domain.PolicyStatusDisabled {
		t.Fatalf("unexpected updated policy: %+v", updated)
	}
	if _, err := svc.UpdateAccessPolicy(ctx, otherOwner, policy.ID, AccessPolicyInput{Name: "越权", PolicyTreeJSON: validPolicyTreeJSON()}); !errors.Is(err, response.ErrAccessPolicyNotFound) {
		t.Fatalf("other DATA_OWNER should not update policy, got %v", err)
	}
	if _, err := svc.UpdateAccessPolicy(ctx, PolicyActor{UserID: 9, TenantID: 3, Roles: []domain.RoleCode{domain.RoleTenantAdmin}}, policy.ID, AccessPolicyInput{Name: "管理员写", PolicyTreeJSON: validPolicyTreeJSON()}); !errors.Is(err, response.ErrAccessPolicyForbidden) {
		t.Fatalf("TENANT_ADMIN should not update policy, got %v", err)
	}
	if err := svc.DeleteAccessPolicy(ctx, otherOwner, policy.ID); !errors.Is(err, response.ErrAccessPolicyNotFound) {
		t.Fatalf("other DATA_OWNER should not delete policy, got %v", err)
	}
	if err := svc.DeleteAccessPolicy(ctx, owner, policy.ID); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if _, err := svc.AccessPolicyDetail(ctx, owner, policy.ID); !errors.Is(err, response.ErrAccessPolicyNotFound) {
		t.Fatalf("deleted policy should be hidden, got %v", err)
	}
}

// newPolicyServiceForTest 创建带有 role 属性的策略服务，供模板和访问策略测试复用。
func newPolicyServiceForTest(t *testing.T, ctx context.Context) *PolicyService {
	t.Helper()
	repo := newMemoryPolicyRepo()
	svc := NewPolicyService(repo, nil)
	if _, err := svc.CreateAttribute(ctx, PolicyAttributeInput{AttrCode: "role", AttrName: "角色", AttrType: domain.PolicyAttributeEnum, AttrValues: []string{"DATA_OWNER", "TENANT_ADMIN"}}); err != nil {
		t.Fatalf("seed role attr: %v", err)
	}
	return svc
}

// validPolicyTreeJSON 返回包含两个角色叶子的合法 OR 访问树。
func validPolicyTreeJSON() []byte {
	return []byte(`{"type":"OR","children":[{"type":"LEAF","attribute":"role","operator":"=","value":"DATA_OWNER"},{"type":"LEAF","attribute":"role","operator":"=","value":"TENANT_ADMIN"}]}`)
}
