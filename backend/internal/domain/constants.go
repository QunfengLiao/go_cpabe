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
