package migrations

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// ValidateRBACMigration 执行 RBAC 迁移后的数据库一致性检查。
//
// 该函数不修改数据，适合迁移命令、集成测试或手工排障复用。检查项聚焦授权事实源是否仍可解释：
// 角色编码是否按租户唯一、用户角色是否悬空、启用租户是否至少有一名有效管理员，以及内置角色权限种子是否完整。
func ValidateRBACMigration(ctx context.Context, db *gorm.DB) error {
	checks := []struct {
		name  string
		query string
	}{
		{
			name: "重复角色编码",
			query: `
SELECT COUNT(*) FROM (
  SELECT tenant_id, code
  FROM roles
  GROUP BY tenant_id, code
  HAVING COUNT(*) > 1
) duplicated_roles`,
		},
		{
			name: "孤立用户角色授权",
			query: `
SELECT COUNT(*)
FROM user_roles ur
LEFT JOIN roles r ON r.id = ur.role_id
WHERE r.id IS NULL`,
		},
		{
			name: "启用租户缺少有效租户管理员",
			query: `
SELECT COUNT(*) FROM (
  SELECT t.id
  FROM tenants t
  LEFT JOIN user_roles ur ON ur.tenant_id = t.id AND ur.status = 'ACTIVE'
  LEFT JOIN roles r ON r.id = ur.role_id AND r.code = 'TENANT_ADMIN' AND r.status = 'ACTIVE'
  LEFT JOIN tenant_users tu ON tu.tenant_id = t.id AND tu.user_id = ur.user_id AND tu.status = 'active'
  WHERE t.status = 'enabled'
  GROUP BY t.id
  HAVING COUNT(DISTINCT tu.user_id) = 0
) tenants_without_admin`,
		},
		{
			name: "内置角色权限种子缺失",
			query: `
SELECT COUNT(*) FROM (
  SELECT 'PLATFORM_ADMIN' AS role_code, 'platform.tenant.read' AS permission_code UNION ALL
  SELECT 'PLATFORM_ADMIN', 'platform.tenant.manage' UNION ALL
  SELECT 'PLATFORM_ADMIN', 'platform.template.read' UNION ALL
  SELECT 'PLATFORM_ADMIN', 'platform.template.manage' UNION ALL
  SELECT 'TENANT_ADMIN', 'tenant.dashboard.read' UNION ALL
  SELECT 'TENANT_ADMIN', 'tenant.role.read' UNION ALL
  SELECT 'TENANT_ADMIN', 'tenant.role.manage' UNION ALL
  SELECT 'TENANT_ADMIN', 'tenant.member.read' UNION ALL
  SELECT 'TENANT_ADMIN', 'tenant.member.manage' UNION ALL
  SELECT 'TENANT_ADMIN', 'tenant.org.read' UNION ALL
  SELECT 'TENANT_ADMIN', 'tenant.org.manage' UNION ALL
  SELECT 'TENANT_ADMIN', 'policy.read' UNION ALL
  SELECT 'TENANT_ADMIN', 'audit.read' UNION ALL
  SELECT 'DO', 'tenant.dashboard.read' UNION ALL
  SELECT 'DO', 'policy.read' UNION ALL
  SELECT 'DO', 'policy.write' UNION ALL
  SELECT 'DO', 'policy.publish' UNION ALL
  SELECT 'DO', 'file.read' UNION ALL
  SELECT 'DO', 'file.upload' UNION ALL
  SELECT 'DO', 'file.manage' UNION ALL
  SELECT 'DU', 'tenant.dashboard.read' UNION ALL
  SELECT 'DU', 'file.read' UNION ALL
  SELECT 'DU', 'file.decrypt.invoke'
) expected
LEFT JOIN roles r ON r.tenant_id = 0 AND r.code = expected.role_code
LEFT JOIN permissions p ON p.code = expected.permission_code
LEFT JOIN role_permissions rp ON rp.role_id = r.id AND rp.permission_id = p.id
WHERE rp.id IS NULL`,
		},
	}
	for _, check := range checks {
		var count int64
		if err := db.WithContext(ctx).Raw(check.query).Scan(&count).Error; err != nil {
			return fmt.Errorf("RBAC 迁移验证失败（%s 查询错误）: %w", check.name, err)
		}
		if count > 0 {
			return fmt.Errorf("RBAC 迁移验证失败（%s）: %d", check.name, count)
		}
	}
	return nil
}
