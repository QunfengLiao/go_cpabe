package domain

type UserRole string
type UserStatus string
type TokenType string

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
