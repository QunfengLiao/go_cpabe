package migrations

import (
	"go-cpabe/backend/internal/domain"

	"gorm.io/gorm"
)

// AutoMigrate 执行开发环境 Gorm 表结构同步，只应由 cmd/migrate 或显式环境变量触发。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&domain.User{},
		&domain.Tenant{},
		&domain.TenantUser{},
		&domain.Role{},
		&domain.UserRoleAssignment{},
		&domain.Permission{},
		&domain.RolePermission{},
		&domain.PolicyAttribute{},
		&domain.PolicyTemplate{},
		&domain.AccessPolicy{},
		&domain.TenantOrgUnit{},
		&domain.TenantOrgMember{},
		&domain.TenantOrgMemberRole{},
		&domain.TenantAttribute{},
		&domain.TenantAttributeValue{},
		&domain.UserAttribute{},
	)
}
