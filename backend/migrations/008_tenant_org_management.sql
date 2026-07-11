-- 007 租户组织架构管理增量迁移。
-- 本脚本保持幂等：可重复执行，不重复插入系统角色，不重复新增字段或索引。
-- 注意：若旧 ORG_MANAGER 在同一部门产生多个 active 负责人，脚本会主动失败，避免静默选择负责人。

DELIMITER $$

CREATE PROCEDURE migrate_008_tenant_org_management()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'tenant_org_members'
      AND COLUMN_NAME = 'is_primary'
  ) THEN
    ALTER TABLE tenant_org_members
      ADD COLUMN is_primary TINYINT(1) NOT NULL DEFAULT 0 AFTER user_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'tenant_org_members'
      AND INDEX_NAME = 'idx_tenant_org_members_primary'
  ) THEN
    ALTER TABLE tenant_org_members
      ADD KEY idx_tenant_org_members_primary (tenant_id, user_id, status, is_primary);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'tenant_org_member_roles'
      AND INDEX_NAME = 'idx_tenant_org_member_roles_unit_role'
  ) THEN
    ALTER TABLE tenant_org_member_roles
      ADD KEY idx_tenant_org_member_roles_unit_role (tenant_id, org_unit_id, role_code, status);
  END IF;
END$$

CREATE PROCEDURE assert_008_no_org_leader_conflict()
BEGIN
  IF EXISTS (
    SELECT 1
    FROM (
      SELECT tenant_id, org_unit_id, COUNT(*) AS leader_count
      FROM tenant_org_member_roles
      WHERE status = 'active'
        AND role_code IN ('ORG_MANAGER', 'ORG_LEADER')
      GROUP BY tenant_id, org_unit_id
      HAVING COUNT(*) > 1
    ) conflicts
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = '008_tenant_org_management: active ORG_MANAGER/ORG_LEADER conflict requires manual cleanup';
  END IF;
END$$

CREATE PROCEDURE normalize_008_primary_departments()
BEGIN
  UPDATE tenant_org_members
  SET is_primary = 0
  WHERE status <> 'active';

  UPDATE tenant_org_members m
  JOIN (
    SELECT tenant_id, user_id, COUNT(*) AS active_count
    FROM tenant_org_members
    WHERE status = 'active'
    GROUP BY tenant_id, user_id
  ) c ON c.tenant_id = m.tenant_id AND c.user_id = m.user_id
  SET m.is_primary = CASE WHEN c.active_count = 1 AND m.status = 'active' THEN 1 ELSE m.is_primary END;

  UPDATE tenant_org_members m
  JOIN (
    SELECT tenant_id, user_id, MIN(id) AS keep_id
    FROM tenant_org_members
    WHERE status = 'active' AND is_primary = 1
    GROUP BY tenant_id, user_id
    HAVING COUNT(*) > 1
  ) dup ON dup.tenant_id = m.tenant_id AND dup.user_id = m.user_id
  SET m.is_primary = CASE WHEN m.id = dup.keep_id THEN 1 ELSE 0 END
  WHERE m.status = 'active';

  UPDATE tenant_org_members m
  JOIN (
    SELECT active_members.tenant_id, active_members.user_id, MIN(active_members.id) AS keep_id
    FROM tenant_org_members active_members
    LEFT JOIN tenant_org_members primary_members
      ON primary_members.tenant_id = active_members.tenant_id
     AND primary_members.user_id = active_members.user_id
     AND primary_members.status = 'active'
     AND primary_members.is_primary = 1
    WHERE active_members.status = 'active'
    GROUP BY active_members.tenant_id, active_members.user_id
    HAVING COUNT(*) > 1 AND COUNT(primary_members.id) = 0
  ) missing ON missing.tenant_id = m.tenant_id AND missing.user_id = m.user_id
  SET m.is_primary = CASE WHEN m.id = missing.keep_id THEN 1 ELSE 0 END
  WHERE m.status = 'active';
END$$

DELIMITER ;

CALL migrate_008_tenant_org_management();
CALL assert_008_no_org_leader_conflict();

INSERT INTO user_roles (tenant_id, user_id, role_id, created_at, updated_at)
SELECT DISTINCT r.tenant_id, r.user_id, roles.id, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)
FROM tenant_org_member_roles r
JOIN roles ON roles.code = 'DO'
WHERE r.role_code = 'DATA_OWNER'
  AND r.status = 'active'
  AND NOT EXISTS (
    SELECT 1
    FROM user_roles ur
    WHERE ur.tenant_id = r.tenant_id
      AND ur.user_id = r.user_id
      AND ur.role_id = roles.id
  );

INSERT INTO user_roles (tenant_id, user_id, role_id, created_at, updated_at)
SELECT DISTINCT r.tenant_id, r.user_id, roles.id, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)
FROM tenant_org_member_roles r
JOIN roles ON roles.code = 'DU'
WHERE r.role_code = 'DATA_VISITOR'
  AND r.status = 'active'
  AND NOT EXISTS (
    SELECT 1
    FROM user_roles ur
    WHERE ur.tenant_id = r.tenant_id
      AND ur.user_id = r.user_id
      AND ur.role_id = roles.id
  );

UPDATE tenant_org_member_roles
SET role_code = 'ORG_LEADER', updated_at = CURRENT_TIMESTAMP(3)
WHERE role_code = 'ORG_MANAGER';

UPDATE tenant_org_member_roles
SET status = 'inactive', updated_at = CURRENT_TIMESTAMP(3)
WHERE role_code IN ('ORG_MEMBER', 'DATA_OWNER', 'DATA_VISITOR')
  AND status = 'active';

UPDATE tenant_attribute_values v
JOIN tenant_attributes a ON a.id = v.attribute_id AND a.tenant_id = v.tenant_id
SET v.value_label = CASE
    WHEN v.value_code = 'ORG_LEADER' THEN '部门负责人'
    WHEN v.value_code = 'DEPUTY_LEADER' THEN '部门副负责人'
    ELSE v.value_label
  END,
  v.status = 'enabled',
  v.updated_at = CURRENT_TIMESTAMP(3)
WHERE a.attr_code = 'org_role'
  AND v.value_code IN ('ORG_LEADER', 'DEPUTY_LEADER');

INSERT INTO tenant_attribute_values (tenant_id, attribute_id, value_code, value_label, sort_order, status, created_at, updated_at)
SELECT a.tenant_id, a.id, v.value_code, v.value_label, v.sort_order, 'enabled', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)
FROM tenant_attributes a
JOIN (
  SELECT 'ORG_LEADER' AS value_code, '部门负责人' AS value_label, 10 AS sort_order UNION ALL
  SELECT 'DEPUTY_LEADER', '部门副负责人', 20
) v
WHERE a.attr_code = 'org_role'
  AND NOT EXISTS (
    SELECT 1
    FROM tenant_attribute_values existing
    WHERE existing.tenant_id = a.tenant_id
      AND existing.attribute_id = a.id
      AND existing.value_code = v.value_code
  );

UPDATE tenant_attribute_values v
JOIN tenant_attributes a ON a.id = v.attribute_id AND a.tenant_id = v.tenant_id
SET v.status = 'disabled', v.updated_at = CURRENT_TIMESTAMP(3)
WHERE a.attr_code = 'org_role'
  AND v.value_code IN ('ORG_MANAGER', 'ORG_MEMBER', 'DATA_OWNER', 'DATA_VISITOR');

CALL normalize_008_primary_departments();

DROP PROCEDURE normalize_008_primary_departments;
DROP PROCEDURE assert_008_no_org_leader_conflict;
DROP PROCEDURE migrate_008_tenant_org_management;
