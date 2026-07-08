package domain

import (
	"time"

	"gorm.io/gorm"
)

// DefaultTenantCode 是承接旧单租户用户和演示数据的默认租户编码。
const DefaultTenantCode = "default-tenant"

// Tenant 表示一个数据共享组织或演示租户，对应 tenants 表。
type Tenant struct {
	// ID 是租户主键，由数据库自增生成。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// Name 是租户展示名称，来源于平台管理员创建或启动 seed。
	Name string `gorm:"column:name;type:varchar(128);not null" json:"name"`
	// Code 是租户稳定编码，必须唯一，常用于登录选择和前端缓存。
	Code string `gorm:"column:code;type:varchar(64);not null;uniqueIndex:uk_tenants_code" json:"code"`
	// Status 控制租户是否允许进入上下文或新增成员。
	Status TenantStatus `gorm:"column:status;type:varchar(32);not null;default:enabled;index" json:"status"`
	// Description 保存租户说明，可为空，仅用于展示。
	Description string `gorm:"column:description;type:varchar(512)" json:"description"`
	// CreatedAt 是数据库写入的创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是数据库维护的更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// DeletedAt 是 Gorm 软删除字段，接口响应不暴露。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// TableName 指定 Tenant 对应 tenants 表，避免 Gorm 默认命名变化影响迁移脚本。
func (Tenant) TableName() string {
	return "tenants"
}

// TenantUser 表示用户与租户的成员关系，对应 tenant_users 表。
type TenantUser struct {
	// ID 是成员关系主键，由数据库自增生成。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// TenantID 指向租户，和 UserID 组成唯一成员关系。
	TenantID uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uk_tenant_users_tenant_user;index" json:"tenant_id"`
	// UserID 指向用户，和 TenantID 组成唯一成员关系。
	UserID uint64 `gorm:"column:user_id;not null;uniqueIndex:uk_tenant_users_tenant_user;index" json:"user_id"`
	// Status 表示成员是否仍可进入租户；移除成员时会置为 disabled。
	Status TenantUserStatus `gorm:"column:status;type:varchar(32);not null;default:active;index" json:"status"`
	// CreatedAt 是成员关系创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是成员关系更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// DeletedAt 保留软删除能力，但业务移除优先使用 Status。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// TableName 指定 TenantUser 对应 tenant_users 表。
func (TenantUser) TableName() string {
	return "tenant_users"
}

// Role 表示平台或租户内可授予的权限角色，对应 roles 表。
type Role struct {
	// ID 是角色主键，由数据库自增生成。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// Code 是业务判断使用的稳定角色编码，必须唯一。
	Code RoleCode `gorm:"column:code;type:varchar(64);not null;uniqueIndex:uk_roles_code" json:"code"`
	// Name 是角色展示名称。
	Name string `gorm:"column:name;type:varchar(128);not null" json:"name"`
	// Scope 区分平台角色和租户角色，权限校验会依赖该边界。
	Scope RoleScope `gorm:"column:scope;type:varchar(32);not null;index" json:"scope"`
	// Description 保存角色说明，可为空，仅用于展示。
	Description string `gorm:"column:description;type:varchar(512)" json:"description"`
	// CreatedAt 是角色创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是角色更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// DeletedAt 是 Gorm 软删除字段，接口响应不暴露。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// TableName 指定 Role 对应 roles 表。
func (Role) TableName() string {
	return "roles"
}

// UserRoleAssignment 表示用户获得某个角色的授权记录，对应 user_roles 表。
type UserRoleAssignment struct {
	// ID 是授权记录主键，由数据库自增生成。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// TenantID 为空表示平台级角色，非空表示该角色只在指定租户内生效。
	TenantID *uint64 `gorm:"column:tenant_id;uniqueIndex:uk_user_roles_tenant_user_role;index" json:"tenant_id"`
	// UserID 指向被授权用户。
	UserID uint64 `gorm:"column:user_id;not null;uniqueIndex:uk_user_roles_tenant_user_role;index" json:"user_id"`
	// RoleID 指向 roles 表中的稳定角色定义。
	RoleID uint64 `gorm:"column:role_id;not null;uniqueIndex:uk_user_roles_tenant_user_role;index" json:"role_id"`
	// CreatedAt 是授权创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是授权更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	// DeletedAt 是 Gorm 软删除字段；撤销授权时会物理删除以释放唯一索引。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// TableName 指定 UserRoleAssignment 对应 user_roles 表。
func (UserRoleAssignment) TableName() string {
	return "user_roles"
}

// TenantDTO 是返回给前端的租户展示模型，隐藏数据库软删除等内部字段。
type TenantDTO struct {
	TenantID         uint64       `json:"tenant_id"`
	TenantName       string       `json:"tenant_name"`
	TenantCode       string       `json:"tenant_code"`
	Status           TenantStatus `json:"status,omitempty"`
	Description      string       `json:"description,omitempty"`
	Roles            []RoleCode   `json:"roles,omitempty"`
	UserCount        int64        `json:"user_count,omitempty"`
	TenantAdminCount int64        `json:"tenant_admin_count,omitempty"`
	CreatedAt        *time.Time   `json:"created_at,omitempty"`
	UpdatedAt        *time.Time   `json:"updated_at,omitempty"`
}

// TenantContextDTO 描述用户当前可访问租户列表和默认选中租户。
type TenantContextDTO struct {
	CurrentTenantID   *uint64     `json:"current_tenant_id"`
	CurrentTenantCode *string     `json:"current_tenant_code,omitempty"`
	Tenants           []TenantDTO `json:"tenants"`
}

// SwitchTenantDTO 是切换租户后的响应模型，包含租户角色和后续菜单扩展点。
type SwitchTenantDTO struct {
	CurrentTenantID uint64     `json:"current_tenant_id"`
	Tenant          TenantDTO  `json:"tenant"`
	Roles           []RoleCode `json:"roles"`
	Menus           []any      `json:"menus"`
}

// TenantMemberDTO 是租户成员列表的展示模型，包含成员状态和租户内角色。
type TenantMemberDTO struct {
	UserID       uint64           `json:"user_id"`
	Email        string           `json:"email"`
	Nickname     string           `json:"nickname"`
	MemberStatus TenantUserStatus `json:"member_status"`
	Roles        []RoleCode       `json:"roles"`
}

// TenantAdminAssignmentDTO 描述租户管理员授权或撤销操作的结果。
type TenantAdminAssignmentDTO struct {
	TenantID uint64   `json:"tenant_id"`
	UserID   uint64   `json:"user_id"`
	Role     RoleCode `json:"role"`
	Assigned bool     `json:"assigned,omitempty"`
	Removed  bool     `json:"removed,omitempty"`
}

// PlatformDashboardDTO 是平台首页统计数据，当前聚合租户、用户和管理员数量。
type PlatformDashboardDTO struct {
	TenantCount         int64 `json:"tenant_count"`
	EnabledTenantCount  int64 `json:"enabled_tenant_count"`
	DisabledTenantCount int64 `json:"disabled_tenant_count"`
	UserCount           int64 `json:"user_count"`
	TenantUserCount     int64 `json:"tenant_user_count"`
	TenantAdminCount    int64 `json:"tenant_admin_count"`
	AuditEnabled        bool  `json:"audit_enabled"`
}
