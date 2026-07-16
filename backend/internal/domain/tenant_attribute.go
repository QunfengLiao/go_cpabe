package domain

import (
	"time"

	"gorm.io/gorm"
)

// TenantAttributeValueSource 表示租户属性值从哪里生成或维护。
type TenantAttributeValueSource string

// UserAttributeStatus 表示用户属性当前是否参与策略匹配输入。
type UserAttributeStatus string

// UserAttributeSourceType 表示用户属性投影来源，帮助解释用户为什么拥有该属性。
type UserAttributeSourceType string

const (
	// TenantAttributeValueSourceManual 表示属性值由租户管理员或种子数据维护。
	TenantAttributeValueSourceManual TenantAttributeValueSource = "manual"
	// TenantAttributeValueSourceOrgTree 表示属性值来自租户组织树。
	TenantAttributeValueSourceOrgTree TenantAttributeValueSource = "org_tree"
	// TenantAttributeValueSourceDerived 表示属性值来自系统已有角色或关系派生。
	TenantAttributeValueSourceDerived TenantAttributeValueSource = "derived"
)

const (
	// UserAttributeStatusActive 表示用户属性当前有效，可作为策略匹配输入。
	UserAttributeStatusActive UserAttributeStatus = "active"
	// UserAttributeStatusInactive 表示用户属性已失效，仅保留历史解释上下文。
	UserAttributeStatusInactive UserAttributeStatus = "inactive"
)

const (
	// UserAttributeSourceOrgMember 表示属性来自组织成员关系。
	UserAttributeSourceOrgMember UserAttributeSourceType = "org_member"
	// UserAttributeSourceOrgMemberRole 表示属性来自部门角色绑定。
	UserAttributeSourceOrgMemberRole UserAttributeSourceType = "org_member_role"
	// UserAttributeSourceTenantRole 表示属性来自租户级 user_roles 授权。
	UserAttributeSourceTenantRole UserAttributeSourceType = "tenant_role"
	// UserAttributeSourceManualSeed 表示属性来自演示 seed 或人工维护的演示值。
	UserAttributeSourceManualSeed UserAttributeSourceType = "manual_seed"
)

// TenantAttribute 表示某个租户可用于 CP-ABE 策略构建和用户属性投影的属性定义。
type TenantAttribute struct {
	// ID 是属性定义主键，访问树节点可通过 valueId 间接关联属性值而不是直接依赖它。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// TenantID 绑定属性所属租户，防止构建器展示其他租户属性。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_tenant_attributes_tenant_code;index" json:"tenantId"`
	// AttrCode 是策略和用户属性匹配使用的稳定属性编码。
	AttrCode string `gorm:"column:attr_code;type:varchar(64);not null;uniqueIndex:uk_tenant_attributes_tenant_code" json:"attrCode"`
	// AttrName 是属性展示名称，用于前端构建器和解释信息。
	AttrName string `gorm:"column:attr_name;type:varchar(128);not null" json:"attrName"`
	// AttrType 决定前端控件、后端校验和用户属性值字段的使用方式。
	AttrType PolicyAttributeType `gorm:"column:attr_type;type:varchar(32);not null" json:"attrType"`
	// ValueSource 说明属性值来源，例如 department 来自组织树。
	ValueSource TenantAttributeValueSource `gorm:"column:value_source;type:varchar(32);not null;default:manual" json:"valueSource"`
	// IsRequired 表示该属性是否属于用户属性同步的基础属性。
	IsRequired bool `gorm:"column:is_required;not null;default:false" json:"isRequired"`
	// IsPolicyEnabled 控制该属性是否开放给 DATA_OWNER 构建新策略。
	IsPolicyEnabled bool `gorm:"column:is_policy_enabled;not null;default:true;index" json:"isPolicyEnabled"`
	// Description 解释属性业务来源和适用场景，帮助 DATA_OWNER 选择正确属性。
	Description string `gorm:"column:description" json:"description"`
	// Status 控制属性是否启用；禁用后历史策略仍可解释但新策略不能选择。
	Status PolicyStatus `gorm:"column:status;type:varchar(32);not null;default:enabled;index" json:"status"`
	// CreatedAt 记录属性创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	// UpdatedAt 记录属性最近更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
	// DeletedAt 是软删除标记，保留历史策略解释上下文。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// TenantAttributeValue 表示租户属性的可选值，department 类型值会关联组织单元。
type TenantAttributeValue struct {
	// ID 是属性值主键，访问树节点保存它作为稳定 valueId。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// TenantID 绑定属性值所属租户，防止跨租户复用 valueId。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_tenant_attribute_values_code;index" json:"tenantId"`
	// AttributeID 指向同租户属性定义。
	AttributeID uint64 `gorm:"column:attribute_id;not null;uniqueIndex:uk_tenant_attribute_values_code;index" json:"attributeId"`
	// ValueCode 是策略匹配使用的稳定值编码。
	ValueCode string `gorm:"column:value_code;type:varchar(128);not null;uniqueIndex:uk_tenant_attribute_values_code" json:"valueCode"`
	// ValueLabel 是前端展示名称，不作为安全判断唯一依据。
	ValueLabel string `gorm:"column:value_label;type:varchar(128);not null" json:"label"`
	// ValuePath 保存树形路径或解释路径，用于表达 department 属于某节点及其子树。
	ValuePath string `gorm:"column:value_path;type:varchar(512)" json:"path,omitempty"`
	// OrgUnitID 在 department 属性值中指向组织单元，其他属性值为空。
	OrgUnitID *uint64 `gorm:"column:org_unit_id;index" json:"orgUnitId,omitempty"`
	// SortOrder 控制枚举和树节点展示顺序。
	SortOrder int `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
	// Status 控制属性值是否可用于新策略。
	Status PolicyStatus `gorm:"column:status;type:varchar(32);not null;default:enabled;index" json:"status"`
	// CreatedAt 记录属性值创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	// UpdatedAt 记录属性值最近更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
	// DeletedAt 是软删除标记，历史策略仍可能需要解释该值。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// UserAttribute 表示用户在某个租户下实际参与策略匹配的 CP-ABE 属性输入。
type UserAttribute struct {
	// ID 是用户属性主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// TenantID 绑定属性所属租户，用户在不同租户的属性必须隔离。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_user_attributes_effective;index" json:"tenantId"`
	// UserID 指向拥有该属性的用户，查看接口只能返回当前用户自己的属性。
	UserID uint64 `gorm:"column:user_id;not null;uniqueIndex:uk_user_attributes_effective;index" json:"userId"`
	// AttributeID 指向租户属性定义，保证属性编码来自当前租户字典。
	AttributeID uint64 `gorm:"column:attribute_id;not null" json:"attributeId"`
	// AttrCode 冗余保存属性编码，便于策略匹配和解释输出。
	AttrCode string `gorm:"column:attr_code;type:varchar(64);not null;uniqueIndex:uk_user_attributes_effective;index" json:"attrCode"`
	// ValueID 指向树形或枚举属性值；数字属性可为空。
	ValueID *uint64 `gorm:"column:value_id" json:"valueId,omitempty"`
	// ValueCode 保存树形或枚举稳定值编码；数字属性可为空。
	ValueCode string `gorm:"column:value_code;type:varchar(128);uniqueIndex:uk_user_attributes_effective" json:"valueCode,omitempty"`
	// ValueLabel 保存展示名称，帮助解释属性来源但不作为安全判断依据。
	ValueLabel string `gorm:"column:value_label;type:varchar(128)" json:"valueLabel,omitempty"`
	// ValuePath 保存树形路径，帮助解释 department 属于语义；长度控制在 255 以内以避免 MySQL utf8mb4 复合唯一索引超过 3072 bytes。
	ValuePath string `gorm:"column:value_path;type:varchar(255);uniqueIndex:uk_user_attributes_effective" json:"valuePath,omitempty"`
	// NumberValue 保存数字属性值，例如 security_level。
	NumberValue *float64 `gorm:"column:number_value;type:decimal(10,2)" json:"numberValue,omitempty"`
	// SourceType 说明属性来自组织成员、部门角色、租户角色或演示维护值。
	SourceType UserAttributeSourceType `gorm:"column:source_type;type:varchar(64);not null;uniqueIndex:uk_user_attributes_effective" json:"sourceType"`
	// SourceID 指向来源记录；演示手工属性可为空。
	SourceID *uint64 `gorm:"column:source_id;uniqueIndex:uk_user_attributes_effective" json:"sourceId,omitempty"`
	// Status 表示该属性当前是否有效参与策略匹配。
	Status UserAttributeStatus `gorm:"column:status;type:varchar(32);not null;default:active;index" json:"status"`
	// SyncedAt 记录最近一次同步时间，用于解释属性新鲜度。
	SyncedAt time.Time `gorm:"column:synced_at" json:"syncedAt"`
	// CreatedAt 记录属性首次创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	// UpdatedAt 记录属性最近更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
	// DeletedAt 是软删除标记，当前业务优先使用 Status 表示失效。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// TenantAttributeDTO 是访问策略构建器读取的租户属性元数据。
type TenantAttributeDTO struct {
	ID          uint64                     `json:"id"`
	TenantID    uint64                     `json:"tenantId"`
	AttrCode    string                     `json:"attrCode"`
	AttrName    string                     `json:"attrName"`
	AttrType    PolicyAttributeType        `json:"attrType"`
	ValueSource TenantAttributeValueSource `json:"valueSource"`
	Operators   []string                   `json:"operators"`
	Status      PolicyStatus               `json:"status"`
	Description string                     `json:"description,omitempty"`
	Values      []TenantAttributeValueDTO  `json:"values"`
	Tree        []TenantAttributeValueDTO  `json:"tree"`
}

// TenantAttributeValueDTO 是属性字典中 enum/tree 值的前端展示模型。
type TenantAttributeValueDTO struct {
	ID        uint64                    `json:"id,omitempty"`
	ValueID   uint64                    `json:"valueId"`
	ValueCode string                    `json:"valueCode"`
	Label     string                    `json:"label"`
	Path      string                    `json:"path,omitempty"`
	Children  []TenantAttributeValueDTO `json:"children,omitempty"`
}

// UserAttributeDTO 是对前端解释用户 CP-ABE 属性的安全响应模型。
type UserAttributeDTO struct {
	ID          uint64                  `json:"id"`
	TenantID    uint64                  `json:"tenantId"`
	UserID      uint64                  `json:"userId"`
	AttrCode    string                  `json:"attrCode"`
	AttrName    string                  `json:"attrName"`
	ValueID     *uint64                 `json:"valueId,omitempty"`
	ValueCode   string                  `json:"valueCode,omitempty"`
	ValueLabel  string                  `json:"valueLabel,omitempty"`
	ValuePath   string                  `json:"valuePath,omitempty"`
	NumberValue *float64                `json:"numberValue,omitempty"`
	SourceType  UserAttributeSourceType `json:"sourceType"`
	Status      UserAttributeStatus     `json:"status"`
	SyncedAt    time.Time               `json:"syncedAt"`
}

// TableName 指定 TenantAttribute 对应租户属性定义表。
func (TenantAttribute) TableName() string { return "tenant_attributes" }

// TableName 指定 TenantAttributeValue 对应租户属性值表。
func (TenantAttributeValue) TableName() string { return "tenant_attribute_values" }

// TableName 指定 UserAttribute 对应用户 CP-ABE 属性表。
func (UserAttribute) TableName() string { return "user_attributes" }
