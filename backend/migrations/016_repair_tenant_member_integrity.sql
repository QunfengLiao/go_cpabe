-- 修复租户级角色授权与 tenant_users 成员关系不一致的问题。
--
-- 文件中心只要求 AuthRequired + TenantRequired。历史数据库可能已经存在
-- TENANT_ADMIN/DO/DU 的租户级 user_roles 记录，但没有对应 tenant_users 记录，
-- 这会让 TenantRequired 在进入文件中心前返回 403。角色授权本身已经说明用户
-- 被加入了该租户，因此这里只补齐“缺失”的 active 成员关系，不覆盖已有的
-- disabled 成员，避免绕过管理员明确执行的成员停用操作。
INSERT INTO tenant_users (tenant_id, user_id, status, created_at, updated_at)
SELECT DISTINCT
  ur.tenant_id,
  ur.user_id,
  'active',
  CURRENT_TIMESTAMP(3),
  CURRENT_TIMESTAMP(3)
FROM user_roles ur
JOIN roles r
  ON r.id = ur.role_id
  AND r.tenant_id = 0
  AND r.scope_type = 'TENANT'
  AND r.status = 'ACTIVE'
JOIN tenants t
  ON t.id = ur.tenant_id
  AND t.status = 'enabled'
JOIN users u
  ON u.id = ur.user_id
  AND u.status = 'active'
LEFT JOIN tenant_users existing
  ON existing.tenant_id = ur.tenant_id
  AND existing.user_id = ur.user_id
WHERE ur.tenant_id IS NOT NULL
  AND ur.tenant_id <> 0
  AND ur.status = 'ACTIVE'
  AND existing.id IS NULL;
