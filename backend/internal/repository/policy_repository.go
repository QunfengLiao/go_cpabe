package repository

import (
	"context"
	"errors"
	"strings"

	"go-cpabe/backend/internal/domain"

	"gorm.io/gorm"
)

var (
	// ErrPolicyAttributeNotFound 表示属性字典记录不存在或已被软删除。
	ErrPolicyAttributeNotFound = errors.New("policy attribute not found")
	// ErrPolicyAttributeCodeExists 表示属性编码已被占用。
	ErrPolicyAttributeCodeExists = errors.New("policy attribute code exists")
	// ErrPolicyTemplateNotFound 表示策略模板不存在或已被软删除。
	ErrPolicyTemplateNotFound = errors.New("policy template not found")
	// ErrAccessPolicyNotFound 表示访问策略不存在、跨租户或不属于当前 DATA_OWNER。
	ErrAccessPolicyNotFound = errors.New("access policy not found")
)

// PolicyRepository 定义访问策略模块的持久化能力，Service 层通过它隔离 Gorm 查询细节。
type PolicyRepository interface {
	ListAttributes(ctx context.Context, onlyEnabled bool) ([]domain.PolicyAttribute, error)
	FindAttributeByID(ctx context.Context, id uint64) (*domain.PolicyAttribute, error)
	FindAttributeByCode(ctx context.Context, code string) (*domain.PolicyAttribute, error)
	CreateAttribute(ctx context.Context, attr *domain.PolicyAttribute) error
	UpdateAttribute(ctx context.Context, attr *domain.PolicyAttribute) error
	DeleteAttribute(ctx context.Context, id uint64) error

	ListTemplates(ctx context.Context, onlyEnabled bool) ([]domain.PolicyTemplate, error)
	FindTemplateByID(ctx context.Context, id uint64) (*domain.PolicyTemplate, error)
	CreateTemplate(ctx context.Context, template *domain.PolicyTemplate) error
	UpdateTemplate(ctx context.Context, template *domain.PolicyTemplate) error
	DeleteTemplate(ctx context.Context, id uint64) error

	ListAccessPolicies(ctx context.Context, input ListAccessPoliciesInput) ([]domain.AccessPolicy, error)
	FindAccessPolicy(ctx context.Context, tenantID, policyID uint64) (*domain.AccessPolicy, error)
	FindAccessPolicyForOwner(ctx context.Context, tenantID, ownerID, policyID uint64) (*domain.AccessPolicy, error)
	CreateAccessPolicy(ctx context.Context, policy *domain.AccessPolicy) error
	UpdateAccessPolicy(ctx context.Context, policy *domain.AccessPolicy) error
	DeleteAccessPolicyForOwner(ctx context.Context, tenantID, ownerID, policyID uint64) error
}

// ListAccessPoliciesInput 表示访问策略列表查询范围，OwnerID 为 0 时返回租户全部策略。
type ListAccessPoliciesInput struct {
	TenantID uint64
	OwnerID  uint64
	Status   domain.PolicyStatus
	Keyword  string
}

// GormPolicyRepository 使用 Gorm 实现 PolicyRepository，负责策略相关三张表的读写。
type GormPolicyRepository struct {
	db *gorm.DB
}

// NewGormPolicyRepository 创建基于 Gorm 的访问策略仓储。
func NewGormPolicyRepository(db *gorm.DB) *GormPolicyRepository {
	return &GormPolicyRepository{db: db}
}

// ListAttributes 返回属性字典列表，onlyEnabled 为 true 时只返回开放给 DATA_OWNER 的属性。
func (r *GormPolicyRepository) ListAttributes(ctx context.Context, onlyEnabled bool) ([]domain.PolicyAttribute, error) {
	var attrs []domain.PolicyAttribute
	query := r.db.WithContext(ctx).Order("id ASC")
	if onlyEnabled {
		query = query.Where("status = ?", domain.PolicyStatusEnabled)
	}
	return attrs, query.Find(&attrs).Error
}

// FindAttributeByID 按主键查找属性字典，找不到时返回 ErrPolicyAttributeNotFound。
func (r *GormPolicyRepository) FindAttributeByID(ctx context.Context, id uint64) (*domain.PolicyAttribute, error) {
	var attr domain.PolicyAttribute
	err := r.db.WithContext(ctx).First(&attr, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPolicyAttributeNotFound
	}
	return &attr, err
}

// FindAttributeByCode 按稳定属性编码查找属性字典，访问树校验依赖该查询。
func (r *GormPolicyRepository) FindAttributeByCode(ctx context.Context, code string) (*domain.PolicyAttribute, error) {
	var attr domain.PolicyAttribute
	err := r.db.WithContext(ctx).Where("attr_code = ?", code).First(&attr).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPolicyAttributeNotFound
	}
	return &attr, err
}

// CreateAttribute 写入属性字典，唯一性冲突会转换为仓储语义错误。
func (r *GormPolicyRepository) CreateAttribute(ctx context.Context, attr *domain.PolicyAttribute) error {
	err := r.db.WithContext(ctx).Create(attr).Error
	if isDuplicateKey(err) {
		return ErrPolicyAttributeCodeExists
	}
	return err
}

// UpdateAttribute 更新属性字典，调用方必须先完成业务校验和状态判断。
func (r *GormPolicyRepository) UpdateAttribute(ctx context.Context, attr *domain.PolicyAttribute) error {
	err := r.db.WithContext(ctx).Save(attr).Error
	if isDuplicateKey(err) {
		return ErrPolicyAttributeCodeExists
	}
	return err
}

// DeleteAttribute 软删除属性字典，历史策略仍可保留 JSON 中的属性编码用于审计。
func (r *GormPolicyRepository) DeleteAttribute(ctx context.Context, id uint64) error {
	result := r.db.WithContext(ctx).Delete(&domain.PolicyAttribute{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPolicyAttributeNotFound
	}
	return nil
}

// ListTemplates 返回策略模板列表，onlyEnabled 为 true 时只返回可被 DATA_OWNER 使用的模板。
func (r *GormPolicyRepository) ListTemplates(ctx context.Context, onlyEnabled bool) ([]domain.PolicyTemplate, error) {
	var templates []domain.PolicyTemplate
	query := r.db.WithContext(ctx).Order("id ASC")
	if onlyEnabled {
		query = query.Where("status = ?", domain.PolicyStatusEnabled)
	}
	return templates, query.Find(&templates).Error
}

// FindTemplateByID 按主键查找策略模板。
func (r *GormPolicyRepository) FindTemplateByID(ctx context.Context, id uint64) (*domain.PolicyTemplate, error) {
	var template domain.PolicyTemplate
	err := r.db.WithContext(ctx).First(&template, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPolicyTemplateNotFound
	}
	return &template, err
}

// CreateTemplate 写入策略模板，模板访问树和表达式由 Service 层预先生成。
func (r *GormPolicyRepository) CreateTemplate(ctx context.Context, template *domain.PolicyTemplate) error {
	return r.db.WithContext(ctx).Create(template).Error
}

// UpdateTemplate 更新策略模板。
func (r *GormPolicyRepository) UpdateTemplate(ctx context.Context, template *domain.PolicyTemplate) error {
	return r.db.WithContext(ctx).Save(template).Error
}

// DeleteTemplate 软删除策略模板，不影响已经创建的访问策略。
func (r *GormPolicyRepository) DeleteTemplate(ctx context.Context, id uint64) error {
	result := r.db.WithContext(ctx).Delete(&domain.PolicyTemplate{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPolicyTemplateNotFound
	}
	return nil
}

// ListAccessPolicies 按租户和可选 owner 范围返回访问策略列表。
func (r *GormPolicyRepository) ListAccessPolicies(ctx context.Context, input ListAccessPoliciesInput) ([]domain.AccessPolicy, error) {
	var policies []domain.AccessPolicy
	query := r.db.WithContext(ctx).Where("tenant_id = ?", input.TenantID).Order("id DESC")
	if input.OwnerID != 0 {
		query = query.Where("owner_id = ?", input.OwnerID)
	}
	if input.Status.Valid() {
		query = query.Where("status = ?", input.Status)
	}
	if keyword := strings.TrimSpace(input.Keyword); keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	return policies, query.Find(&policies).Error
}

// FindAccessPolicy 按租户和策略 ID 查找策略，TENANT_ADMIN 只读详情使用该范围。
func (r *GormPolicyRepository) FindAccessPolicy(ctx context.Context, tenantID, policyID uint64) (*domain.AccessPolicy, error) {
	var policy domain.AccessPolicy
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, policyID).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAccessPolicyNotFound
	}
	return &policy, err
}

// FindAccessPolicyForOwner 按租户、owner 和策略 ID 查找策略，DATA_OWNER 写操作必须使用该范围。
func (r *GormPolicyRepository) FindAccessPolicyForOwner(ctx context.Context, tenantID, ownerID, policyID uint64) (*domain.AccessPolicy, error) {
	var policy domain.AccessPolicy
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND id = ?", tenantID, ownerID, policyID).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAccessPolicyNotFound
	}
	return &policy, err
}

// CreateAccessPolicy 写入 DATA_OWNER 的访问策略，tenant_id 和 owner_id 必须来自后端上下文。
func (r *GormPolicyRepository) CreateAccessPolicy(ctx context.Context, policy *domain.AccessPolicy) error {
	return r.db.WithContext(ctx).Create(policy).Error
}

// UpdateAccessPolicy 更新访问策略，调用方必须先校验 owner 边界。
func (r *GormPolicyRepository) UpdateAccessPolicy(ctx context.Context, policy *domain.AccessPolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

// DeleteAccessPolicyForOwner 软删除 DATA_OWNER 自己的策略，避免跨 owner 删除。
func (r *GormPolicyRepository) DeleteAccessPolicyForOwner(ctx context.Context, tenantID, ownerID, policyID uint64) error {
	result := r.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND id = ?", tenantID, ownerID, policyID).Delete(&domain.AccessPolicy{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAccessPolicyNotFound
	}
	return nil
}

// isDuplicateKey 粗粒度识别 MySQL 唯一键冲突，避免 repository 直接依赖具体 driver 错误类型。
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}
