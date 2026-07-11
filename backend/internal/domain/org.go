package domain

import (
	"time"

	"gorm.io/gorm"
)

// OrgUnitStatus 表示租户组织单元是否可用于新策略和成员绑定。
type OrgUnitStatus string

// OrgMemberStatus 表示用户在组织单元中的成员关系是否有效。
type OrgMemberStatus string

// OrgMemberRoleCode 表示部门作用域内的特殊组织职务编码。
type OrgMemberRoleCode string

// OrgRelationSource 表示组织、成员、角色或属性的来源，便于解释演示数据和人工维护数据。
type OrgRelationSource string

const (
	// OrgUnitStatusEnabled 表示组织单元可被选择并参与属性投影。
	OrgUnitStatusEnabled OrgUnitStatus = "enabled"
	// OrgUnitStatusDisabled 表示组织单元已停用，历史策略可回显但新策略不应选择。
	OrgUnitStatusDisabled OrgUnitStatus = "disabled"
)

const (
	// OrgMemberStatusActive 表示用户当前属于该组织单元。
	OrgMemberStatusActive OrgMemberStatus = "active"
	// OrgMemberStatusInactive 表示成员关系已失效，仅保留历史解释线索。
	OrgMemberStatusInactive OrgMemberStatus = "inactive"
)

const (
	// OrgRoleLeader 表示用户是某个部门范围内唯一负责人。
	OrgRoleLeader OrgMemberRoleCode = "ORG_LEADER"
	// OrgRoleDeputyLeader 表示用户是某个部门范围内副负责人。
	OrgRoleDeputyLeader OrgMemberRoleCode = "DEPUTY_LEADER"
	// OrgRoleManager 是旧部门主管编码，仅供迁移识别，不能再作为新写入值。
	OrgRoleManager OrgMemberRoleCode = "ORG_MANAGER"
	// OrgRoleMember 是旧普通成员编码，仅供迁移识别，普通成员身份应由 tenant_org_members 表达。
	OrgRoleMember OrgMemberRoleCode = "ORG_MEMBER"
	// OrgRoleDataOwner 是旧数据拥有者部门职务编码，仅供迁移到 user_roles.DO 时识别。
	OrgRoleDataOwner OrgMemberRoleCode = "DATA_OWNER"
	// OrgRoleDataVisitor 是旧数据访问者部门职务编码，仅供迁移到 user_roles.DU 时识别。
	OrgRoleDataVisitor OrgMemberRoleCode = "DATA_VISITOR"
)

const (
	// OrgRelationSourceManual 表示数据来自租户管理员人工维护。
	OrgRelationSourceManual OrgRelationSource = "manual"
	// OrgRelationSourceSeed 表示数据来自演示环境种子数据。
	OrgRelationSourceSeed OrgRelationSource = "seed"
)

// Valid 判断组织单元状态是否属于系统允许值。
func (s OrgUnitStatus) Valid() bool {
	return s == OrgUnitStatusEnabled || s == OrgUnitStatusDisabled
}

// Valid 判断组织成员状态是否属于系统允许值。
func (s OrgMemberStatus) Valid() bool {
	return s == OrgMemberStatusActive || s == OrgMemberStatusInactive
}

// Valid 判断部门职务编码是否属于 007 后允许写入的特殊组织职务集合。
func (r OrgMemberRoleCode) Valid() bool {
	return r == OrgRoleLeader || r == OrgRoleDeputyLeader
}

// TenantOrgUnit 表示某个租户下的组织架构节点，是 department 属性和部门角色作用域的来源。
type TenantOrgUnit struct {
	// ID 是组织单元主键，前端可把它保存为访问策略节点的稳定 valueId。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// TenantID 绑定组织单元所属租户，是所有组织查询和写入的安全边界。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_tenant_org_units_tenant_code;uniqueIndex:uk_tenant_org_units_tenant_path;index" json:"tenantId"`
	// ParentID 指向同租户父组织单元；为空表示根级节点，移动节点时必须防止循环。
	ParentID *uint64 `gorm:"column:parent_id;index" json:"parentId,omitempty"`
	// Code 是租户内稳定编码，策略和种子数据使用它而不是中文名称做长期引用。
	Code string `gorm:"column:code;type:varchar(64);not null;uniqueIndex:uk_tenant_org_units_tenant_code" json:"code"`
	// Name 是展示名称，可随业务调整，不应作为权限判断唯一依据。
	Name string `gorm:"column:name;type:varchar(128);not null" json:"name"`
	// Path 保存稳定编码路径，用于解释“属于某部门及其下级”的策略语义。
	Path string `gorm:"column:path;type:varchar(512);not null;uniqueIndex:uk_tenant_org_units_tenant_path" json:"path"`
	// Level 是树深度，帮助前端渲染和后端校验移动结果。
	Level int `gorm:"column:level;not null;default:1" json:"level"`
	// SortOrder 是同级排序值，只影响展示顺序，不参与权限判断。
	SortOrder int `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
	// Status 控制组织单元是否可被新策略或新成员关系使用。
	Status OrgUnitStatus `gorm:"column:status;type:varchar(32);not null;default:enabled;index" json:"status"`
	// CreatedAt 记录组织节点创建时间，用于管理页排序和演示排查。
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	// UpdatedAt 记录最近更新时间，用于前端缓存失效和人工排查。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
	// DeletedAt 是软删除标记，历史策略仍可能需要解释该节点。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// TenantOrgMember 表示用户属于某个租户组织单元的成员关系。
type TenantOrgMember struct {
	// ID 是成员关系主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// TenantID 绑定成员关系所属租户，防止跨租户把用户加入部门。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_tenant_org_members_scope;index" json:"tenantId"`
	// OrgUnitID 指向成员所属组织单元，必须与 TenantID 属于同一租户。
	OrgUnitID uint64 `gorm:"column:org_unit_id;not null;uniqueIndex:uk_tenant_org_members_scope;index" json:"orgUnitId"`
	// UserID 指向租户有效成员，只有已加入 tenant_users 的用户才能写入。
	UserID uint64 `gorm:"column:user_id;not null;uniqueIndex:uk_tenant_org_members_scope;index" json:"userId"`
	// IsPrimary 标记用户在当前租户的主部门；同一租户同一用户多个 active 关系中必须且只能有一个为 true。
	IsPrimary bool `gorm:"column:is_primary;not null;default:false;index:idx_tenant_org_members_primary" json:"isPrimary"`
	// Status 表示成员关系是否仍可投影为用户属性。
	Status OrgMemberStatus `gorm:"column:status;type:varchar(32);not null;default:active;index" json:"status"`
	// Source 记录成员关系来自人工维护还是演示 seed，便于解释和避免 seed 覆盖人工数据。
	Source OrgRelationSource `gorm:"column:source;type:varchar(32);not null;default:manual" json:"source"`
	// CreatedAt 记录成员关系创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	// UpdatedAt 记录成员关系最近更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
	// DeletedAt 是软删除标记，当前业务优先使用 Status 失效关系。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// TenantOrgMemberRole 表示用户在某个部门内拥有的特殊组织职务绑定。
type TenantOrgMemberRole struct {
	// ID 是部门角色绑定主键。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// TenantID 绑定角色所属租户，是部门角色投影到用户属性时的安全边界。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_tenant_org_member_roles_scope;index" json:"tenantId"`
	// OrgMemberID 指向成员关系，确保角色只能绑定给已经属于该部门的用户。
	OrgMemberID uint64 `gorm:"column:org_member_id;not null;index" json:"orgMemberId"`
	// OrgUnitID 冗余保存部门作用域，便于按部门查询和构建属性来源。
	OrgUnitID uint64 `gorm:"column:org_unit_id;not null;uniqueIndex:uk_tenant_org_member_roles_scope;index" json:"orgUnitId"`
	// UserID 冗余保存用户标识，便于同步单个用户属性时减少额外 join。
	UserID uint64 `gorm:"column:user_id;not null;uniqueIndex:uk_tenant_org_member_roles_scope;index" json:"userId"`
	// RoleCode 是受控部门职务，只允许负责人和副负责人，系统权限角色必须写入 user_roles。
	RoleCode OrgMemberRoleCode `gorm:"column:role_code;type:varchar(64);not null;uniqueIndex:uk_tenant_org_member_roles_scope" json:"roleCode"`
	// Status 表示该部门角色是否仍可投影为用户属性。
	Status OrgMemberStatus `gorm:"column:status;type:varchar(32);not null;default:active;index" json:"status"`
	// Source 记录角色绑定来自人工维护还是演示 seed。
	Source OrgRelationSource `gorm:"column:source;type:varchar(32);not null;default:manual" json:"source"`
	// CreatedAt 记录角色绑定创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	// UpdatedAt 记录角色绑定最近更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
	// DeletedAt 是软删除标记，当前业务优先使用 Status 失效关系。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// OrgUnitTreeDTO 是前端组织树和 department 属性选择器共用的树节点。
type OrgUnitTreeDTO struct {
	ID             uint64                    `json:"id"`
	TenantID       uint64                    `json:"tenantId"`
	ParentID       *uint64                   `json:"parentId,omitempty"`
	Code           string                    `json:"code"`
	Name           string                    `json:"name"`
	Path           string                    `json:"path"`
	Level          int                       `json:"level"`
	SortOrder      int                       `json:"sortOrder"`
	Status         OrgUnitStatus             `json:"status"`
	AttributeValue *OrgUnitAttributeValueDTO `json:"attributeValue,omitempty"`
	// MemberCount 是当前部门 active 成员数，由组织树接口批量聚合，避免前端逐部门请求成员列表。
	MemberCount int64 `json:"memberCount"`
	// Leader 展示当前部门负责人摘要，只来自 active ORG_LEADER 职务，不代表系统权限角色。
	Leader *OrgUnitLeaderDTO `json:"leader,omitempty"`
	// DeputyLeaderCount 是当前部门 active 副负责人数量，用于详情摘要展示。
	DeputyLeaderCount int64 `json:"deputyLeaderCount"`
	Children       []OrgUnitTreeDTO          `json:"children"`
}

// OrgUnitLeaderDTO 是组织树节点上负责人展示所需的最小用户信息。
type OrgUnitLeaderDTO struct {
	UserID   uint64 `json:"userId"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
}

// OrgUnitAttributeValueDTO 是组织树节点关联的 department 属性值展示模型。
type OrgUnitAttributeValueDTO struct {
	ValueID    uint64              `json:"valueId"`
	ValueCode  string              `json:"valueCode"`
	ValueLabel string              `json:"valueLabel"`
	ValuePath  string              `json:"valuePath"`
	Status     PolicyStatus        `json:"status"`
}

// OrgMemberDTO 是组织成员列表返回给租户管理员的展示模型。
type OrgMemberDTO struct {
	ID           uint64              `json:"id"`
	UserID       uint64              `json:"userId"`
	Username     string              `json:"username,omitempty"`
	Email        string              `json:"email"`
	Nickname     string              `json:"nickname"`
	MemberStatus OrgMemberStatus     `json:"memberStatus"`
	OrgUnit      OrgMemberUnitDTO    `json:"orgUnit"`
	IsPrimary    bool                `json:"isPrimary"`
	Positions    []OrgMemberRoleCode `json:"positions"`
	SystemRoles  []RoleCode          `json:"systemRoles"`
}

// OrgMemberUnitDTO 是组织成员列表中部门基础信息的内嵌展示模型。
type OrgMemberUnitDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// TableName 指定 TenantOrgUnit 对应租户组织单元表。
func (TenantOrgUnit) TableName() string { return "tenant_org_units" }

// TableName 指定 TenantOrgMember 对应租户组织成员表。
func (TenantOrgMember) TableName() string { return "tenant_org_members" }

// TableName 指定 TenantOrgMemberRole 对应租户组织成员角色表。
func (TenantOrgMemberRole) TableName() string { return "tenant_org_member_roles" }
