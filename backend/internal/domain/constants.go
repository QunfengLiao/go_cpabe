package domain

// UserRole 表示旧单租户用户角色，当前主要用于登录态兼容和迁移到租户角色。
type UserRole string

// UserStatus 表示用户账号状态，禁用用户不能登录或刷新长期会话。
type UserStatus string

// TokenType 标识 JWT 或刷新凭证的用途，避免不同生命周期的 token 混用。
type TokenType string

// TenantStatus 表示租户是否可用，禁用租户不能继续新增成员或进入租户上下文。
type TenantStatus string

// TenantUserStatus 表示用户在某个租户中的成员关系状态。
type TenantUserStatus string

// RoleCode 表示平台和租户授权使用的稳定角色编码。
type RoleCode string

// RoleScope 表示角色作用域，区分平台级权限和租户内权限。
type RoleScope string

// RoleScopeType 表示新 RBAC 角色作用域，数据库授权事实源使用大写枚举值。
type RoleScopeType string

// RoleCategory 表示角色治理语义分类，用于区分治理、业务和能力角色。
type RoleCategory string

// RoleStatus 表示角色定义是否仍可产生权限或被新分配。
type RoleStatus string

// UserRoleAssignmentStatus 表示用户角色授权记录的生命周期状态。
type UserRoleAssignmentStatus string

// PermissionStatus 表示权限点是否仍可绑定和参与授权判断。
type PermissionStatus string

// PermissionScopeType 表示权限点所属平台或租户作用域。
type PermissionScopeType string

// AssignmentSource 表示用户角色授权记录的来源，便于区分系统迁移和人工分配。
type AssignmentSource string

const (
	// RoleAdmin 是旧单租户管理员角色，只允许本地受控命令创建。
	RoleAdmin UserRole = "admin"
	// RoleDataOwner 表示数据拥有者，迁移后对应租户内 DO 角色。
	RoleDataOwner UserRole = "data_owner"
	// RoleDataUser 表示数据使用者，迁移后对应租户内 DU 角色。
	RoleDataUser UserRole = "data_user"
)

const (
	// StatusActive 表示账号可正常登录和访问。
	StatusActive UserStatus = "active"
	// StatusDisabled 表示账号已禁用，长期会话刷新时也会被拒绝。
	StatusDisabled UserStatus = "disabled"
)

const (
	// TenantStatusEnabled 表示租户可用。
	TenantStatusEnabled TenantStatus = "enabled"
	// TenantStatusDisabled 表示租户被停用。
	TenantStatusDisabled TenantStatus = "disabled"
)

const (
	// TenantUserStatusActive 表示用户是租户的有效成员。
	TenantUserStatusActive TenantUserStatus = "active"
	// TenantUserStatusDisabled 表示成员关系已停用但保留历史记录。
	TenantUserStatusDisabled TenantUserStatus = "disabled"
)

const (
	// RolePlatformAdmin 表示平台级管理员，不绑定具体租户。
	RolePlatformAdmin RoleCode = "PLATFORM_ADMIN"
	// RoleTenantAdmin 表示租户管理员，只在指定租户内生效。
	RoleTenantAdmin RoleCode = "TENANT_ADMIN"
	// RoleDO 表示数据拥有者，通常负责上传和管理文件。
	RoleDO RoleCode = "DO"
	// RoleDU 表示数据使用者，通常负责查看、下载和尝试解密文件。
	RoleDU RoleCode = "DU"
)

const (
	// RoleScopePlatform 表示角色作用于整个平台后台。
	RoleScopePlatform RoleScope = "platform"
	// RoleScopeTenant 表示角色只作用于某个租户。
	RoleScopeTenant RoleScope = "tenant"
)

const (
	// RoleScopeTypePlatform 表示平台治理作用域，不得自动进入租户业务权限。
	RoleScopeTypePlatform RoleScopeType = "PLATFORM"
	// RoleScopeTypeTenant 表示租户业务作用域，必须绑定有效租户成员关系。
	RoleScopeTypeTenant RoleScopeType = "TENANT"
)

const (
	// RoleCategoryGovernance 表示治理角色，例如平台管理员和租户管理员。
	RoleCategoryGovernance RoleCategory = "GOVERNANCE"
	// RoleCategoryBusiness 表示租户自定义业务角色，只能由当前租户创建和维护。
	RoleCategoryBusiness RoleCategory = "BUSINESS"
	// RoleCategoryCapability 表示系统内置能力角色，例如 DO 和 DU。
	RoleCategoryCapability RoleCategory = "CAPABILITY"
)

const (
	// RoleStatusActive 表示角色可产生权限并可被分配。
	RoleStatusActive RoleStatus = "ACTIVE"
	// RoleStatusDisabled 表示角色被逻辑禁用，不再产生权限也不能新分配。
	RoleStatusDisabled RoleStatus = "DISABLED"
)

const (
	// UserRoleStatusActive 表示授权记录当前有效。
	UserRoleStatusActive UserRoleAssignmentStatus = "ACTIVE"
	// UserRoleStatusRevoked 表示授权已被撤销，但历史记录保留。
	UserRoleStatusRevoked UserRoleAssignmentStatus = "REVOKED"
	// UserRoleStatusExpired 表示授权已过期，查询时也会动态排除 expires_at 过期记录。
	UserRoleStatusExpired UserRoleAssignmentStatus = "EXPIRED"
)

const (
	// PermissionStatusActive 表示权限点可用于角色绑定和授权判断。
	PermissionStatusActive PermissionStatus = "ACTIVE"
	// PermissionStatusDisabled 表示权限点被禁用，不参与授权判断。
	PermissionStatusDisabled PermissionStatus = "DISABLED"
)

const (
	// PermissionScopePlatform 表示平台治理权限。
	PermissionScopePlatform PermissionScopeType = "PLATFORM"
	// PermissionScopeTenant 表示租户业务权限。
	PermissionScopeTenant PermissionScopeType = "TENANT"
)

const (
	// AssignmentSourceSystem 表示系统初始化或内部受控流程产生的授权。
	AssignmentSourceSystem AssignmentSource = "SYSTEM"
	// AssignmentSourceManual 表示管理员通过接口手动分配的授权。
	AssignmentSourceManual AssignmentSource = "MANUAL"
	// AssignmentSourceMigration 表示历史数据迁移产生的授权。
	AssignmentSourceMigration AssignmentSource = "MIGRATION"
)

const (
	// TokenTypeAccess 表示短期访问 token。
	TokenTypeAccess TokenType = "access"
	// TokenTypeRefresh 表示长期刷新 token。
	TokenTypeRefresh TokenType = "refresh"
)

// Valid 判断旧单租户用户角色是否属于系统允许的枚举值。
func (r UserRole) Valid() bool {
	return r == RoleAdmin || r == RoleDataOwner || r == RoleDataUser
}

// PublicRegistrable 判断角色是否允许通过公开注册接口自助选择。
func (r UserRole) PublicRegistrable() bool {
	return r == RoleDataOwner || r == RoleDataUser
}

// Valid 判断用户状态是否属于系统允许的枚举值。
func (s UserStatus) Valid() bool {
	return s == StatusActive || s == StatusDisabled
}

// Valid 判断租户状态是否属于系统允许的枚举值。
func (s TenantStatus) Valid() bool {
	return s == TenantStatusEnabled || s == TenantStatusDisabled
}

// Valid 判断租户成员状态是否属于系统允许的枚举值。
func (s TenantUserStatus) Valid() bool {
	return s == TenantUserStatusActive || s == TenantUserStatusDisabled
}

// Valid 判断角色编码是否属于当前系统支持的平台或租户角色。
func (r RoleCode) Valid() bool {
	return r == RolePlatformAdmin || r == RoleTenantAdmin || r == RoleDO || r == RoleDU
}

// TenantScoped 判断角色是否必须绑定具体租户，平台授权会拒绝这些租户内角色。
func (r RoleCode) TenantScoped() bool {
	return r == RoleTenantAdmin || r == RoleDO || r == RoleDU
}

// Valid 判断角色作用域是否属于平台级或租户级。
func (s RoleScope) Valid() bool {
	return s == RoleScopePlatform || s == RoleScopeTenant
}

// Valid 判断新 RBAC 角色作用域是否属于平台或租户枚举。
func (s RoleScopeType) Valid() bool {
	return s == RoleScopeTypePlatform || s == RoleScopeTypeTenant
}

// Valid 判断角色分类是否属于本阶段支持的治理、业务或能力枚举。
func (c RoleCategory) Valid() bool {
	return c == RoleCategoryGovernance || c == RoleCategoryBusiness || c == RoleCategoryCapability
}

// Valid 判断角色状态是否属于本阶段支持的有效或禁用枚举。
func (s RoleStatus) Valid() bool {
	return s == RoleStatusActive || s == RoleStatusDisabled
}

// Valid 判断用户角色授权状态是否属于有效、撤销或过期枚举。
func (s UserRoleAssignmentStatus) Valid() bool {
	return s == UserRoleStatusActive || s == UserRoleStatusRevoked || s == UserRoleStatusExpired
}

// Valid 判断权限状态是否属于有效或禁用枚举。
func (s PermissionStatus) Valid() bool {
	return s == PermissionStatusActive || s == PermissionStatusDisabled
}

// Valid 判断权限作用域是否属于平台或租户枚举。
func (s PermissionScopeType) Valid() bool {
	return s == PermissionScopePlatform || s == PermissionScopeTenant
}

// Valid 判断授权来源是否属于系统、人工或迁移来源。
func (s AssignmentSource) Valid() bool {
	return s == AssignmentSourceSystem || s == AssignmentSourceManual || s == AssignmentSourceMigration
}

// RoleScopeTypeFromLegacy 将旧小写 scope 转换为新 RBAC 大写作用域，供迁移期兼容读取。
func RoleScopeTypeFromLegacy(scope RoleScope) RoleScopeType {
	if scope == RoleScopePlatform {
		return RoleScopeTypePlatform
	}
	return RoleScopeTypeTenant
}

// LegacyRoleScopeFromType 将新 RBAC 作用域转换为旧 scope 字段，避免过渡期旧代码读到空值。
func LegacyRoleScopeFromType(scope RoleScopeType) RoleScope {
	if scope == RoleScopeTypePlatform {
		return RoleScopePlatform
	}
	return RoleScopeTenant
}

// MapLegacyUserRole 将旧单租户角色映射为默认租户内角色，用于迁移历史用户授权。
func MapLegacyUserRole(role UserRole) RoleCode {
	switch role {
	case RoleAdmin:
		return RoleTenantAdmin
	case RoleDataOwner:
		return RoleDO
	default:
		return RoleDU
	}
}
