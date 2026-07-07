package domain

type UserRole string
type UserStatus string
type TokenType string
type TenantStatus string
type TenantUserStatus string
type RoleCode string
type RoleScope string

const (
	RoleAdmin     UserRole = "admin"
	RoleDataOwner UserRole = "data_owner"
	RoleDataUser  UserRole = "data_user"
)

const (
	StatusActive   UserStatus = "active"
	StatusDisabled UserStatus = "disabled"
)

const (
	TenantStatusEnabled  TenantStatus = "enabled"
	TenantStatusDisabled TenantStatus = "disabled"
)

const (
	TenantUserStatusActive   TenantUserStatus = "active"
	TenantUserStatusDisabled TenantUserStatus = "disabled"
)

const (
	RolePlatformAdmin RoleCode = "PLATFORM_ADMIN"
	RoleTenantAdmin   RoleCode = "TENANT_ADMIN"
	RoleDO            RoleCode = "DO"
	RoleDU            RoleCode = "DU"
)

const (
	RoleScopePlatform RoleScope = "platform"
	RoleScopeTenant   RoleScope = "tenant"
)

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

func (r UserRole) Valid() bool {
	return r == RoleAdmin || r == RoleDataOwner || r == RoleDataUser
}

func (r UserRole) PublicRegistrable() bool {
	return r == RoleDataOwner || r == RoleDataUser
}

func (s UserStatus) Valid() bool {
	return s == StatusActive || s == StatusDisabled
}

func (s TenantStatus) Valid() bool {
	return s == TenantStatusEnabled || s == TenantStatusDisabled
}

func (s TenantUserStatus) Valid() bool {
	return s == TenantUserStatusActive || s == TenantUserStatusDisabled
}

func (r RoleCode) Valid() bool {
	return r == RolePlatformAdmin || r == RoleTenantAdmin || r == RoleDO || r == RoleDU
}

func (r RoleCode) TenantScoped() bool {
	return r == RoleTenantAdmin || r == RoleDO || r == RoleDU
}

func (s RoleScope) Valid() bool {
	return s == RoleScopePlatform || s == RoleScopeTenant
}

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
