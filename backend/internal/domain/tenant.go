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
	// LogoURL 是租户品牌 Logo 的静态资源 URL，可为空；为空时由 Service 按租户编码提供默认品牌。
	LogoURL string `gorm:"column:logo_url;type:varchar(512)" json:"logo_url,omitempty"`
	// LoginBackgroundURL 是登录页背景图 URL，可为空；前端会叠加遮罩保证表单可读性。
	LoginBackgroundURL string `gorm:"column:login_background_url;type:varchar(512)" json:"login_background_url,omitempty"`
	// WorkspaceBackgroundURL 是工作台背景图 URL，可为空；仅作为低对比度品牌氛围层。
	WorkspaceBackgroundURL string `gorm:"column:workspace_background_url;type:varchar(512)" json:"workspace_background_url,omitempty"`
	// PrimaryColor 是租户主色，必须由可信后台写入，前端只作为 CSS 变量使用。
	PrimaryColor string `gorm:"column:primary_color;type:varchar(32)" json:"primary_color,omitempty"`
	// SidebarColor 是侧边栏强调色，避免不同租户在导航区完全不可区分。
	SidebarColor string `gorm:"column:sidebar_color;type:varchar(32)" json:"sidebar_color,omitempty"`
	// BackgroundStart 是工作台渐变起始色，可为空。
	BackgroundStart string `gorm:"column:background_start;type:varchar(32)" json:"background_start,omitempty"`
	// BackgroundEnd 是工作台渐变结束色，可为空。
	BackgroundEnd string `gorm:"column:background_end;type:varchar(32)" json:"background_end,omitempty"`
	// BackgroundGlow 是背景柔光色，可为空；前端会降低透明度，避免影响内容可读性。
	BackgroundGlow string `gorm:"column:background_glow;type:varchar(32)" json:"background_glow,omitempty"`
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
	// TenantID 表示角色归属，0 表示系统内置角色，非 0 表示当前租户自定义角色。
	TenantID uint64 `gorm:"column:tenant_id;not null;default:0;uniqueIndex:uk_roles_tenant_code;index:idx_roles_tenant_status,priority:1" json:"tenant_id"`
	// Code 是业务判断使用的稳定角色编码，同一 tenant_id 内唯一，创建后不允许修改。
	Code RoleCode `gorm:"column:code;type:varchar(64);not null;uniqueIndex:uk_roles_tenant_code" json:"code"`
	// Name 是角色展示名称。
	Name string `gorm:"column:name;type:varchar(128);not null" json:"name"`
	// Scope 是旧版小写作用域字段，迁移期保留以兼容旧查询和前端返回。
	Scope RoleScope `gorm:"column:scope;type:varchar(32);not null;index" json:"scope"`
	// ScopeType 是新 RBAC 授权事实源使用的作用域，平台角色不得出现在租户成员分配接口。
	ScopeType RoleScopeType `gorm:"column:scope_type;type:varchar(32);not null;default:TENANT;index:idx_roles_scope_category_status,priority:1" json:"scope_type"`
	// RoleCategory 表示角色治理语义，自定义角色只能是 TENANT + BUSINESS。
	RoleCategory RoleCategory `gorm:"column:role_category;type:varchar(32);not null;default:BUSINESS;index:idx_roles_scope_category_status,priority:2" json:"role_category"`
	// IsBuiltin 标识系统内置角色，内置角色不可由租户修改 code、分类、作用域或权限。
	IsBuiltin bool `gorm:"column:is_builtin;not null;default:false" json:"is_builtin"`
	// Status 控制角色是否可产生权限和被新分配，禁用角色仍保留历史绑定。
	Status RoleStatus `gorm:"column:status;type:varchar(32);not null;default:ACTIVE;index:idx_roles_tenant_status,priority:2;index:idx_roles_scope_category_status,priority:3" json:"status"`
	// Description 保存角色说明，可为空，仅用于展示。
	Description string `gorm:"column:description;type:varchar(512)" json:"description"`
	// CreatedBy 记录创建人，系统 seed 或迁移数据可为空。
	CreatedBy *uint64 `gorm:"column:created_by" json:"created_by,omitempty"`
	// UpdatedBy 记录最后更新人，系统 seed 或迁移数据可为空。
	UpdatedBy *uint64 `gorm:"column:updated_by" json:"updated_by,omitempty"`
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
	// TenantID 为空是迁移前平台授权兼容值；新数据使用 0 表示平台作用域，非 0 表示租户作用域。
	TenantID *uint64 `gorm:"column:tenant_id;uniqueIndex:uk_user_roles_tenant_user_role;index" json:"tenant_id"`
	// UserID 指向被授权用户。
	UserID uint64 `gorm:"column:user_id;not null;uniqueIndex:uk_user_roles_tenant_user_role;index" json:"user_id"`
	// RoleID 指向 roles 表中的稳定角色定义。
	RoleID uint64 `gorm:"column:role_id;not null;uniqueIndex:uk_user_roles_tenant_user_role;index" json:"role_id"`
	// AssignmentSource 记录授权来源，迁移和 seed 会用它区分历史补偿与人工分配。
	AssignmentSource AssignmentSource `gorm:"column:assignment_source;type:varchar(32);not null;default:SYSTEM" json:"assignment_source"`
	// AssignedBy 记录分配人，系统迁移和 seed 可为空。
	AssignedBy *uint64 `gorm:"column:assigned_by" json:"assigned_by,omitempty"`
	// Status 表示授权生命周期，只有 ACTIVE 且未过期的记录可产生权限。
	Status UserRoleAssignmentStatus `gorm:"column:status;type:varchar(32);not null;default:ACTIVE;index:idx_user_roles_tenant_user_status,priority:3;index:idx_user_roles_tenant_role_status,priority:3" json:"status"`
	// ExpiresAt 表示授权过期时间，为空表示不过期；权限查询必须动态排除过期记录。
	ExpiresAt *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
	// RevokedAt 表示授权撤销时间，撤销时写入以保留审计线索。
	RevokedAt *time.Time `gorm:"column:revoked_at" json:"revoked_at,omitempty"`
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

// Permission 表示服务端统一授权服务可判断的稳定功能权限点。
type Permission struct {
	// ID 是权限主键，由数据库自增生成。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// Code 是全局唯一权限编码，不允许使用通配符作为真实权限。
	Code string `gorm:"column:code;type:varchar(128);not null;uniqueIndex:uk_permissions_code" json:"code"`
	// Name 是权限展示名称，主要用于后续前端分组展示。
	Name string `gorm:"column:name;type:varchar(128);not null" json:"name"`
	// Description 描述权限边界，可为空。
	Description string `gorm:"column:description;type:varchar(512)" json:"description"`
	// ScopeType 区分平台权限和租户权限，绑定角色时必须与角色作用域匹配。
	ScopeType PermissionScopeType `gorm:"column:scope_type;type:varchar(32);not null;index:idx_permissions_scope_status,priority:1" json:"scope_type"`
	// ResourceType 表示权限保护的资源类型，便于前端分组和排障。
	ResourceType string `gorm:"column:resource_type;type:varchar(64);not null;index" json:"resource_type"`
	// Action 表示权限动作，例如 read、manage、write 或 publish。
	Action string `gorm:"column:action;type:varchar(64);not null" json:"action"`
	// Status 控制权限是否可绑定和参与授权判断。
	Status PermissionStatus `gorm:"column:status;type:varchar(32);not null;default:ACTIVE;index:idx_permissions_scope_status,priority:2" json:"status"`
	// CreatedAt 是权限创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// UpdatedAt 是权限更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定 Permission 对应 permissions 表。
func (Permission) TableName() string {
	return "permissions"
}

// RolePermission 表示角色与权限点之间的授权绑定关系。
type RolePermission struct {
	// ID 是角色权限绑定主键，由数据库自增生成。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// RoleID 指向 roles 表，内置角色和自定义角色都通过该字段绑定权限。
	RoleID uint64 `gorm:"column:role_id;not null;uniqueIndex:uk_role_permissions_role_permission;index" json:"role_id"`
	// PermissionID 指向 permissions 表，必须是有效权限点。
	PermissionID uint64 `gorm:"column:permission_id;not null;uniqueIndex:uk_role_permissions_role_permission;index" json:"permission_id"`
	// GrantedBy 记录授权人，内置角色 seed 可为空。
	GrantedBy *uint64 `gorm:"column:granted_by" json:"granted_by,omitempty"`
	// CreatedAt 是绑定创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定 RolePermission 对应 role_permissions 表。
func (RolePermission) TableName() string {
	return "role_permissions"
}

// TenantDTO 是返回给前端的租户展示模型，隐藏数据库软删除等内部字段。
type TenantDTO struct {
	TenantID         uint64       `json:"tenant_id"`
	TenantName       string       `json:"tenant_name"`
	TenantCode       string       `json:"tenant_code"`
	Status           TenantStatus `json:"status,omitempty"`
	Description      string       `json:"description,omitempty"`
	Branding         TenantBrandingDTO `json:"branding,omitempty"`
	Roles            []RoleCode   `json:"roles,omitempty"`
	UserCount        int64        `json:"user_count,omitempty"`
	TenantAdminCount int64        `json:"tenant_admin_count,omitempty"`
	CreatedAt        *time.Time   `json:"created_at,omitempty"`
	UpdatedAt        *time.Time   `json:"updated_at,omitempty"`
}

// TenantBrandingDTO 是前端主题系统使用的租户品牌配置，不参与权限判断。
type TenantBrandingDTO struct {
	LogoURL                string `json:"logoUrl,omitempty"`
	LoginBackgroundURL     string `json:"loginBackgroundUrl,omitempty"`
	WorkspaceBackgroundURL string `json:"workspaceBackgroundUrl,omitempty"`
	PrimaryColor           string `json:"primaryColor,omitempty"`
	SidebarColor           string `json:"sidebarColor,omitempty"`
	BackgroundStart        string `json:"backgroundStart,omitempty"`
	BackgroundEnd          string `json:"backgroundEnd,omitempty"`
	BackgroundGlow         string `json:"backgroundGlow,omitempty"`
}

// TenantContextDTO 描述用户当前可访问租户列表和默认选中租户。
type TenantContextDTO struct {
	CurrentTenantID   *uint64     `json:"current_tenant_id"`
	CurrentTenantCode *string     `json:"current_tenant_code,omitempty"`
	CurrentTenant     *TenantDTO  `json:"currentTenant,omitempty"`
	TenantRoles       []RoleCode   `json:"tenantRoles,omitempty"`
	Permissions       []string     `json:"permissions,omitempty"`
	PlatformRoles     []RoleCode   `json:"platform_roles"`
	User              *UserDTO     `json:"user,omitempty"`
	Tenants           []TenantDTO  `json:"tenants"`
}

// SwitchTenantDTO 是切换租户后的响应模型，包含租户角色和后续菜单扩展点。
type SwitchTenantDTO struct {
	CurrentTenantID uint64     `json:"current_tenant_id"`
	CurrentTenant   TenantDTO  `json:"currentTenant"`
	Tenant          TenantDTO  `json:"tenant"`
	TenantRoles     []RoleCode `json:"tenantRoles"`
	Permissions     []string   `json:"permissions"`
	Roles           []RoleCode `json:"roles"`
	Menus           []any      `json:"menus"`
}

// TenantMemberDTO 是租户成员列表的展示模型，包含成员状态和租户内角色。
type TenantMemberDTO struct {
	UserID       uint64           `json:"user_id"`
	Username     string           `json:"username,omitempty"`
	Email        string           `json:"email"`
	Nickname     string           `json:"nickname"`
	Phone        string           `json:"phone,omitempty"`
	MemberStatus TenantUserStatus `json:"member_status"`
	Roles        []RoleCode       `json:"roles"`
	JoinedAt     *time.Time       `json:"joined_at,omitempty"`
}

// TenantAdminAssignmentDTO 描述租户管理员授权或撤销操作的结果。
type TenantAdminAssignmentDTO struct {
	TenantID    uint64   `json:"tenant_id"`
	UserID      uint64   `json:"user_id"`
	Role        RoleCode `json:"role"`
	Assigned    bool     `json:"assigned,omitempty"`
	Removed     bool     `json:"removed,omitempty"`
	CreatedUser bool     `json:"created_user,omitempty"`
	// TemporaryPassword 仅在平台代建新租户管理员账号时返回一次，避免固定默认密码扩散到长期凭据。
	TemporaryPassword string   `json:"temporary_password,omitempty"`
	User              *UserDTO `json:"user,omitempty"`
}

// PermissionDTO 是租户权限目录和角色权限响应使用的权限展示模型。
type PermissionDTO struct {
	ID           uint64              `json:"id"`
	Code         string              `json:"code"`
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	ScopeType    PermissionScopeType `json:"scopeType"`
	ResourceType string              `json:"resourceType"`
	Action       string              `json:"action"`
	Status       PermissionStatus    `json:"status"`
}

// TenantRoleDTO 是租户角色列表、详情和成员角色响应中的统一角色摘要。
type TenantRoleDTO struct {
	ID                uint64       `json:"id"`
	TenantID          uint64       `json:"tenantId"`
	Code              string       `json:"code"`
	Name              string       `json:"name"`
	Description       string       `json:"description,omitempty"`
	ScopeType         RoleScopeType `json:"scopeType"`
	RoleCategory      RoleCategory `json:"roleCategory"`
	Category          RoleCategory `json:"category,omitempty"`
	Builtin           bool         `json:"builtin"`
	IsBuiltin         bool         `json:"is_builtin,omitempty"`
	Status            RoleStatus   `json:"status"`
	PermissionCount   int64        `json:"permissionCount,omitempty"`
	ActiveMemberCount int64        `json:"activeMemberCount,omitempty"`
	// CreatedAt 记录角色创建时间，仅用于管理界面解释角色来源和排查数据初始化问题。
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 记录角色最近更新时间，用于前端展示角色配置是否近期发生变化。
	UpdatedAt time.Time `json:"updatedAt"`
}

// RolePermissionDTO 描述某个角色当前绑定的权限集合。
type RolePermissionDTO struct {
	RoleID          uint64          `json:"roleId"`
	PermissionCodes []string        `json:"permissionCodes"`
	Permissions     []PermissionDTO `json:"permissions,omitempty"`
}

// MemberRoleDTO 描述租户成员当前角色与权限并集，成员角色全量替换后直接返回该模型。
type MemberRoleDTO struct {
	TenantID    uint64          `json:"tenantId"`
	UserID      uint64          `json:"userId"`
	Roles       []TenantRoleDTO `json:"roles"`
	Permissions []string        `json:"permissions"`
}

// AuthorizationContextDTO 描述当前用户或指定成员在租户作用域下的真实授权上下文。
type AuthorizationContextDTO struct {
	TenantID    uint64          `json:"tenantId"`
	Roles       []TenantRoleDTO `json:"roles"`
	Permissions []string        `json:"permissions"`
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
