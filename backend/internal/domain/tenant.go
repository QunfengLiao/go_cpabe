package domain

import (
	"time"

	"gorm.io/gorm"
)

const DefaultTenantCode = "default-tenant"

type Tenant struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string         `gorm:"column:name;type:varchar(128);not null" json:"name"`
	Code        string         `gorm:"column:code;type:varchar(64);not null;uniqueIndex:uk_tenants_code" json:"code"`
	Status      TenantStatus   `gorm:"column:status;type:varchar(32);not null;default:enabled;index" json:"status"`
	Description string         `gorm:"column:description;type:varchar(512)" json:"description"`
	CreatedAt   time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (Tenant) TableName() string {
	return "tenants"
}

type TenantUser struct {
	ID        uint64           `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  uint64           `gorm:"column:tenant_id;not null;uniqueIndex:uk_tenant_users_tenant_user;index" json:"tenant_id"`
	UserID    uint64           `gorm:"column:user_id;not null;uniqueIndex:uk_tenant_users_tenant_user;index" json:"user_id"`
	Status    TenantUserStatus `gorm:"column:status;type:varchar(32);not null;default:active;index" json:"status"`
	CreatedAt time.Time        `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time        `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt   `gorm:"column:deleted_at;index" json:"-"`
}

func (TenantUser) TableName() string {
	return "tenant_users"
}

type Role struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        RoleCode       `gorm:"column:code;type:varchar(64);not null;uniqueIndex:uk_roles_code" json:"code"`
	Name        string         `gorm:"column:name;type:varchar(128);not null" json:"name"`
	Scope       RoleScope      `gorm:"column:scope;type:varchar(32);not null;index" json:"scope"`
	Description string         `gorm:"column:description;type:varchar(512)" json:"description"`
	CreatedAt   time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (Role) TableName() string {
	return "roles"
}

type UserRoleAssignment struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  *uint64        `gorm:"column:tenant_id;uniqueIndex:uk_user_roles_tenant_user_role;index" json:"tenant_id"`
	UserID    uint64         `gorm:"column:user_id;not null;uniqueIndex:uk_user_roles_tenant_user_role;index" json:"user_id"`
	RoleID    uint64         `gorm:"column:role_id;not null;uniqueIndex:uk_user_roles_tenant_user_role;index" json:"role_id"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (UserRoleAssignment) TableName() string {
	return "user_roles"
}

type TenantDTO struct {
	TenantID   uint64       `json:"tenant_id"`
	TenantName string       `json:"tenant_name"`
	TenantCode string       `json:"tenant_code"`
	Status     TenantStatus `json:"status,omitempty"`
	Roles      []RoleCode   `json:"roles,omitempty"`
}

type TenantContextDTO struct {
	CurrentTenantID   *uint64     `json:"current_tenant_id"`
	CurrentTenantCode *string     `json:"current_tenant_code,omitempty"`
	Tenants           []TenantDTO `json:"tenants"`
}

type SwitchTenantDTO struct {
	CurrentTenantID uint64      `json:"current_tenant_id"`
	Tenant          TenantDTO   `json:"tenant"`
	Roles           []RoleCode  `json:"roles"`
	Menus           []any       `json:"menus"`
}

type TenantMemberDTO struct {
	UserID       uint64             `json:"user_id"`
	Email        string             `json:"email"`
	Nickname     string             `json:"nickname"`
	MemberStatus TenantUserStatus   `json:"member_status"`
	Roles        []RoleCode         `json:"roles"`
}
