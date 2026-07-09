package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PolicyStatus 表示策略相关资源是否对后续构建流程开放。
type PolicyStatus string

// PolicyAttributeType 表示属性叶子节点值的校验方式。
type PolicyAttributeType string

const (
	// PolicyStatusEnabled 表示属性、模板或策略当前可用。
	PolicyStatusEnabled PolicyStatus = "enabled"
	// PolicyStatusDisabled 表示资源被停用但保留历史记录。
	PolicyStatusDisabled PolicyStatus = "disabled"
)

const (
	// PolicyAttributeString 表示属性值由数据拥有者输入文本。
	PolicyAttributeString PolicyAttributeType = "string"
	// PolicyAttributeEnum 表示属性值必须来自平台管理员维护的可选值。
	PolicyAttributeEnum PolicyAttributeType = "enum"
	// PolicyAttributeNumber 表示属性值必须是数字。
	PolicyAttributeNumber PolicyAttributeType = "number"
)

// Valid 判断策略状态是否属于系统允许的枚举值。
func (s PolicyStatus) Valid() bool {
	return s == PolicyStatusEnabled || s == PolicyStatusDisabled
}

// Valid 判断属性类型是否属于访问树叶子节点可校验的类型。
func (t PolicyAttributeType) Valid() bool {
	return t == PolicyAttributeString || t == PolicyAttributeEnum || t == PolicyAttributeNumber
}

// PolicyAttribute 是平台管理员维护的属性字典，决定 DATA_OWNER 构建访问树时能引用哪些属性。
type PolicyAttribute struct {
	// ID 是属性字典主键，外部响应可展示但不作为访问树引用来源。
	ID uint64 `gorm:"primaryKey" json:"id"`
	// AttrCode 是访问树叶子节点引用的稳定编码，必须唯一且不可被用户随意伪造。
	AttrCode string `gorm:"column:attr_code;size:64;uniqueIndex" json:"attr_code"`
	// AttrName 是属性展示名称，用于前端编辑器和错误提示。
	AttrName string `gorm:"column:attr_name;size:128" json:"attr_name"`
	// AttrType 决定前端输入控件和后端保存前校验规则。
	AttrType PolicyAttributeType `gorm:"column:attr_type;size:32" json:"attr_type"`
	// AttrValues 保存 enum 可选值；string/number 类型可为空，不能存放敏感信息。
	AttrValues JSONText `gorm:"column:attr_values;type:json" json:"attr_values"`
	// Description 解释属性来源和业务含义，帮助 DATA_OWNER 选择正确属性。
	Description string `gorm:"column:description" json:"description"`
	// Status 控制属性是否开放给 DATA_OWNER 新建或保存访问树。
	Status PolicyStatus `gorm:"column:status;size:32" json:"status"`
	// CreatedAt 记录属性创建时间，用于管理页排序和审计展示。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 记录属性最近更新时间，用于前端缓存和冲突判断。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// DeletedAt 是软删除标记，避免历史策略引用丢失审计上下文。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// PolicyTemplate 是平台管理员维护的可复用访问树起点，不代表具体租户文件授权。
type PolicyTemplate struct {
	// ID 是模板主键，DATA_OWNER 选择模板时使用该值读取详情。
	ID uint64 `gorm:"primaryKey" json:"id"`
	// Name 是模板名称，用于构建入口展示。
	Name string `gorm:"column:name;size:128" json:"name"`
	// Description 解释模板适用场景，避免 DATA_OWNER 误选过宽策略。
	Description string `gorm:"column:description" json:"description"`
	// PolicyExpr 是后端根据访问树生成的标准表达式，只用于展示和后续策略转换准备。
	PolicyExpr string `gorm:"column:policy_expr" json:"policy_expr"`
	// PolicyTreeJSON 是模板访问树结构，保存前必须经过后端校验。
	PolicyTreeJSON JSONText `gorm:"column:policy_tree_json;type:json" json:"policy_tree_json"`
	// Status 控制模板是否可作为 DATA_OWNER 新策略起点。
	Status PolicyStatus `gorm:"column:status;size:32" json:"status"`
	// CreatedAt 记录模板创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 记录模板最近更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// DeletedAt 是模板软删除标记，已基于模板创建的访问策略不依赖该记录继续存在。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// AccessPolicy 是 DATA_OWNER 在租户内创建的具体访问策略，后续文件加密共享会引用它。
type AccessPolicy struct {
	// ID 是访问策略主键。
	ID uint64 `gorm:"primaryKey" json:"id"`
	// TenantID 绑定策略所属租户，是防止跨租户访问的安全边界字段。
	TenantID uint64 `gorm:"column:tenant_id;index" json:"tenant_id"`
	// OwnerID 绑定创建者用户，是防止 DATA_OWNER 修改他人策略的安全边界字段。
	OwnerID uint64 `gorm:"column:owner_id;index" json:"owner_id"`
	// Name 是策略展示名称，由 DATA_OWNER 维护。
	Name string `gorm:"column:name;size:128" json:"name"`
	// Description 描述策略业务用途，不参与权限判断。
	Description string `gorm:"column:description" json:"description"`
	// PolicyExpr 是后端根据访问树生成的标准表达式，客户端提交值不作为权威来源。
	PolicyExpr string `gorm:"column:policy_expr" json:"policy_expr"`
	// PolicyTreeJSON 是后端校验通过后的访问树结构，后续加密模块应以它为策略来源。
	PolicyTreeJSON JSONText `gorm:"column:policy_tree_json;type:json" json:"policy_tree_json"`
	// Status 表示策略是否可作为后续文件上传策略候选。
	Status PolicyStatus `gorm:"column:status;size:32" json:"status"`
	// CreatedAt 记录策略创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 记录策略更新时间，用于前端草稿冲突检测。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// DeletedAt 是策略软删除标记，保留历史用于后续审计扩展。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// JSONText 是用于 Gorm JSON 列的轻量封装，避免在业务层散落 []byte 与字符串转换。
type JSONText []byte

// MarshalJSON 将空 JSONText 输出为 null，避免前端收到非法空 JSON。
func (j JSONText) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON 保存调用方提交的 JSON 原文，Service 层再执行结构校验。
func (j *JSONText) UnmarshalJSON(data []byte) error {
	if !json.Valid(data) {
		return json.Unmarshal(data, new(any))
	}
	*j = append((*j)[:0], data...)
	return nil
}

// Value 将 JSONText 写入数据库 JSON/TEXT 列，空值写为 null 保持 SQL 层语义明确。
func (j JSONText) Value() (driver.Value, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("invalid json text")
	}
	return []byte(j), nil
}

// Scan 从数据库 JSON/TEXT 列恢复 JSONText，兼容 MySQL driver 返回的 []byte 或 string。
func (j *JSONText) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append((*j)[:0], v...)
	case string:
		*j = append((*j)[:0], []byte(v)...)
	default:
		return fmt.Errorf("unsupported JSONText scan type %T", value)
	}
	return nil
}

// TableName 指定属性字典表名，保持与迁移文件一致。
func (PolicyAttribute) TableName() string { return "policy_attributes" }

// TableName 指定策略模板表名，保持与迁移文件一致。
func (PolicyTemplate) TableName() string { return "policy_templates" }

// TableName 指定访问策略表名，保持与迁移文件一致。
func (AccessPolicy) TableName() string { return "access_policies" }
