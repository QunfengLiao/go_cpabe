-- 010_tenant_rbac.sql
-- 租户级 RBAC 增量迁移。本脚本保持幂等，可重复执行。
-- 说明：MySQL DDL 可能隐式提交，因此破坏性唯一约束调整前先做冲突检查。

DELIMITER $$

DROP PROCEDURE IF EXISTS assert_010_rbac_preconditions$$
DROP PROCEDURE IF EXISTS migrate_010_rbac_schema$$
DROP PROCEDURE IF EXISTS migrate_010_rbac_backfill$$
DROP PROCEDURE IF EXISTS migrate_010_rbac_indexes$$
DROP PROCEDURE IF EXISTS seed_010_permissions$$
DROP PROCEDURE IF EXISTS seed_010_role_permissions$$
DROP PROCEDURE IF EXISTS assert_010_rbac_postconditions$$

CREATE PROCEDURE assert_010_rbac_preconditions()
BEGIN
  IF EXISTS (
    SELECT 1
    FROM (
      SELECT tenant_id, user_id, role_id, COUNT(*) AS cnt
      FROM user_roles
      GROUP BY tenant_id, user_id, role_id
      HAVING COUNT(*) > 1
    ) dup
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = '010_tenant_rbac: duplicated user_roles must be cleaned before migration';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (
      SELECT COALESCE(tenant_id, 0) AS normalized_tenant_id, user_id, role_id, COUNT(*) AS cnt
      FROM user_roles
      GROUP BY COALESCE(tenant_id, 0), user_id, role_id
      HAVING COUNT(*) > 1
    ) dup
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = '010_tenant_rbac: duplicated platform user_roles after tenant_id normalization';
  END IF;
END$$

CREATE PROCEDURE migrate_010_rbac_schema()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND COLUMN_NAME = 'tenant_id'
  ) THEN
    ALTER TABLE roles ADD COLUMN tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND COLUMN_NAME = 'scope_type'
  ) THEN
    ALTER TABLE roles ADD COLUMN scope_type VARCHAR(32) NOT NULL DEFAULT 'TENANT' AFTER scope;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND COLUMN_NAME = 'role_category'
  ) THEN
    ALTER TABLE roles ADD COLUMN role_category VARCHAR(32) NOT NULL DEFAULT 'BUSINESS' AFTER scope_type;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND COLUMN_NAME = 'is_builtin'
  ) THEN
    ALTER TABLE roles ADD COLUMN is_builtin TINYINT(1) NOT NULL DEFAULT 0 AFTER role_category;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND COLUMN_NAME = 'status'
  ) THEN
    ALTER TABLE roles ADD COLUMN status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' AFTER is_builtin;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND COLUMN_NAME = 'created_by'
  ) THEN
    ALTER TABLE roles ADD COLUMN created_by BIGINT UNSIGNED NULL AFTER description;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND COLUMN_NAME = 'updated_by'
  ) THEN
    ALTER TABLE roles ADD COLUMN updated_by BIGINT UNSIGNED NULL AFTER created_by;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_roles' AND COLUMN_NAME = 'assignment_source'
  ) THEN
    ALTER TABLE user_roles ADD COLUMN assignment_source VARCHAR(32) NOT NULL DEFAULT 'SYSTEM' AFTER role_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_roles' AND COLUMN_NAME = 'assigned_by'
  ) THEN
    ALTER TABLE user_roles ADD COLUMN assigned_by BIGINT UNSIGNED NULL AFTER assignment_source;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_roles' AND COLUMN_NAME = 'status'
  ) THEN
    ALTER TABLE user_roles ADD COLUMN status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' AFTER assigned_by;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_roles' AND COLUMN_NAME = 'expires_at'
  ) THEN
    ALTER TABLE user_roles ADD COLUMN expires_at DATETIME(3) NULL AFTER status;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_roles' AND COLUMN_NAME = 'revoked_at'
  ) THEN
    ALTER TABLE user_roles ADD COLUMN revoked_at DATETIME(3) NULL AFTER expires_at;
  END IF;

  CREATE TABLE IF NOT EXISTS permissions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code VARCHAR(128) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(512) NULL,
    scope_type VARCHAR(32) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_permissions_code (code),
    KEY idx_permissions_scope_status (scope_type, status)
  );

  CREATE TABLE IF NOT EXISTS role_permissions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    role_id BIGINT UNSIGNED NOT NULL,
    permission_id BIGINT UNSIGNED NOT NULL,
    granted_by BIGINT UNSIGNED NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_role_permissions_role_permission (role_id, permission_id),
    KEY idx_role_permissions_role_id (role_id),
    KEY idx_role_permissions_permission_id (permission_id)
  );
END$$

CREATE PROCEDURE migrate_010_rbac_backfill()
BEGIN
  INSERT INTO roles (tenant_id, code, name, scope, scope_type, role_category, is_builtin, status, description, created_at, updated_at)
  SELECT 0, seed.code, seed.name, seed.scope, seed.scope_type, seed.role_category, 1, 'ACTIVE', seed.description, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)
  FROM (
    SELECT 'PLATFORM_ADMIN' AS code, '平台管理员' AS name, 'platform' AS scope, 'PLATFORM' AS scope_type, 'GOVERNANCE' AS role_category, '平台治理角色' AS description UNION ALL
    SELECT 'TENANT_ADMIN', '租户管理员', 'tenant', 'TENANT', 'GOVERNANCE', '租户治理角色' UNION ALL
    SELECT 'DO', '数据拥有者', 'tenant', 'TENANT', 'CAPABILITY', '数据拥有者能力角色' UNION ALL
    SELECT 'DU', '数据使用者', 'tenant', 'TENANT', 'CAPABILITY', '数据使用者能力角色'
  ) seed
  WHERE NOT EXISTS (
    SELECT 1 FROM roles existing WHERE existing.tenant_id = 0 AND existing.code = seed.code
  );

  UPDATE roles
  SET tenant_id = 0
  WHERE tenant_id IS NULL;

  UPDATE roles
  SET scope_type = CASE WHEN scope = 'platform' THEN 'PLATFORM' ELSE 'TENANT' END
  WHERE scope_type IS NULL OR scope_type = '' OR scope_type NOT IN ('PLATFORM', 'TENANT');

  UPDATE roles
  SET role_category = CASE
      WHEN code IN ('PLATFORM_ADMIN', 'TENANT_ADMIN') THEN 'GOVERNANCE'
      WHEN code IN ('DO', 'DU') THEN 'CAPABILITY'
      ELSE role_category
    END,
    is_builtin = CASE WHEN code IN ('PLATFORM_ADMIN', 'TENANT_ADMIN', 'DO', 'DU') AND tenant_id = 0 THEN 1 ELSE is_builtin END,
    status = CASE WHEN status IS NULL OR status = '' THEN 'ACTIVE' ELSE status END,
    updated_at = CURRENT_TIMESTAMP(3)
  WHERE tenant_id = 0;

  UPDATE user_roles
  SET tenant_id = 0
  WHERE tenant_id IS NULL;

  UPDATE user_roles
  SET status = 'ACTIVE'
  WHERE status IS NULL OR status = '';

  UPDATE user_roles
  SET assignment_source = 'MIGRATION'
  WHERE assignment_source IS NULL OR assignment_source = '';
END$$

CREATE PROCEDURE migrate_010_rbac_indexes()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND INDEX_NAME = 'uk_roles_tenant_code'
  ) THEN
    ALTER TABLE roles ADD UNIQUE KEY uk_roles_tenant_code (tenant_id, code);
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND INDEX_NAME = 'uk_roles_code'
  ) THEN
    ALTER TABLE roles DROP INDEX uk_roles_code;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND INDEX_NAME = 'idx_roles_tenant_status'
  ) THEN
    ALTER TABLE roles ADD KEY idx_roles_tenant_status (tenant_id, status);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND INDEX_NAME = 'idx_roles_scope_category_status'
  ) THEN
    ALTER TABLE roles ADD KEY idx_roles_scope_category_status (scope_type, role_category, status);
  END IF;

  ALTER TABLE user_roles MODIFY tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_roles' AND INDEX_NAME = 'idx_user_roles_tenant_user_status'
  ) THEN
    ALTER TABLE user_roles ADD KEY idx_user_roles_tenant_user_status (tenant_id, user_id, status);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_roles' AND INDEX_NAME = 'idx_user_roles_tenant_role_status'
  ) THEN
    ALTER TABLE user_roles ADD KEY idx_user_roles_tenant_role_status (tenant_id, role_id, status);
  END IF;
END$$

CREATE PROCEDURE seed_010_permissions()
BEGIN
  INSERT INTO permissions (code, name, description, scope_type, resource_type, action, status, created_at, updated_at)
  VALUES
    ('platform.tenant.read', '查看租户', '查看平台租户信息', 'PLATFORM', 'platform_tenant', 'read', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('platform.tenant.manage', '管理租户', '创建、启停和维护平台租户', 'PLATFORM', 'platform_tenant', 'manage', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('platform.template.read', '查看平台模板', '查看平台公共模板', 'PLATFORM', 'platform_template', 'read', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('platform.template.manage', '管理平台模板', '维护平台公共模板', 'PLATFORM', 'platform_template', 'manage', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('tenant.dashboard.read', '查看租户概览', '查看当前租户概览', 'TENANT', 'tenant_dashboard', 'read', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('tenant.role.read', '查看角色', '查看当前租户角色和权限', 'TENANT', 'tenant_role', 'read', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('tenant.role.manage', '管理角色', '创建、修改、禁用自定义角色并配置权限', 'TENANT', 'tenant_role', 'manage', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('tenant.member.read', '查看成员', '查看当前租户成员和角色', 'TENANT', 'tenant_member', 'read', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('tenant.member.manage', '管理成员', '维护当前租户成员角色', 'TENANT', 'tenant_member', 'manage', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('tenant.org.read', '查看组织', '查看当前租户组织结构', 'TENANT', 'tenant_org', 'read', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('tenant.org.manage', '管理组织', '维护当前租户组织结构', 'TENANT', 'tenant_org', 'manage', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('policy.read', '查看策略', '查看当前租户访问策略', 'TENANT', 'policy', 'read', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('policy.write', '编辑策略', '创建和修改当前租户访问策略', 'TENANT', 'policy', 'write', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('policy.publish', '发布策略', '发布当前租户访问策略', 'TENANT', 'policy', 'publish', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('file.read', '查看文件', '查看当前租户文件', 'TENANT', 'file', 'read', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('file.upload', '上传文件', '上传当前租户文件', 'TENANT', 'file', 'upload', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('file.manage', '管理文件', '管理当前租户文件', 'TENANT', 'file', 'manage', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('file.decrypt.invoke', '调用解密', '调用文件解密流程，不代表一定满足 CP-ABE 策略', 'TENANT', 'file', 'decrypt.invoke', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
    ('audit.read', '查看审计', '查看当前租户审计信息', 'TENANT', 'audit', 'read', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3))
  ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    description = VALUES(description),
    scope_type = VALUES(scope_type),
    resource_type = VALUES(resource_type),
    action = VALUES(action),
    status = VALUES(status),
    updated_at = CURRENT_TIMESTAMP(3);
END$$

CREATE PROCEDURE seed_010_role_permissions()
BEGIN
  INSERT IGNORE INTO role_permissions (role_id, permission_id, granted_by, created_at)
  SELECT r.id, p.id, NULL, CURRENT_TIMESTAMP(3)
  FROM roles r
  JOIN permissions p ON p.code IN ('platform.tenant.read', 'platform.tenant.manage', 'platform.template.read', 'platform.template.manage')
  WHERE r.tenant_id = 0 AND r.code = 'PLATFORM_ADMIN';

  INSERT IGNORE INTO role_permissions (role_id, permission_id, granted_by, created_at)
  SELECT r.id, p.id, NULL, CURRENT_TIMESTAMP(3)
  FROM roles r
  JOIN permissions p ON p.code IN ('tenant.dashboard.read', 'tenant.role.read', 'tenant.role.manage', 'tenant.member.read', 'tenant.member.manage', 'tenant.org.read', 'tenant.org.manage', 'policy.read', 'audit.read')
  WHERE r.tenant_id = 0 AND r.code = 'TENANT_ADMIN';

  INSERT IGNORE INTO role_permissions (role_id, permission_id, granted_by, created_at)
  SELECT r.id, p.id, NULL, CURRENT_TIMESTAMP(3)
  FROM roles r
  JOIN permissions p ON p.code IN ('tenant.dashboard.read', 'policy.read', 'policy.write', 'policy.publish', 'file.read', 'file.upload', 'file.manage')
  WHERE r.tenant_id = 0 AND r.code = 'DO';

  INSERT IGNORE INTO role_permissions (role_id, permission_id, granted_by, created_at)
  SELECT r.id, p.id, NULL, CURRENT_TIMESTAMP(3)
  FROM roles r
  JOIN permissions p ON p.code IN ('tenant.dashboard.read', 'file.read', 'file.decrypt.invoke')
  WHERE r.tenant_id = 0 AND r.code = 'DU';
END$$

CREATE PROCEDURE assert_010_rbac_postconditions()
BEGIN
  IF EXISTS (
    SELECT 1
    FROM (
      SELECT tenant_id, code, COUNT(*) AS cnt
      FROM roles
      GROUP BY tenant_id, code
      HAVING COUNT(*) > 1
    ) dup
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = '010_tenant_rbac: duplicated roles after migration';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM user_roles ur
    LEFT JOIN roles r ON r.id = ur.role_id
    WHERE r.id IS NULL
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = '010_tenant_rbac: orphan user_roles after migration';
  END IF;
END$$

DELIMITER ;

CALL assert_010_rbac_preconditions();
CALL migrate_010_rbac_schema();
CALL migrate_010_rbac_backfill();
CALL migrate_010_rbac_indexes();
CALL seed_010_permissions();
CALL seed_010_role_permissions();
CALL assert_010_rbac_postconditions();

DROP PROCEDURE assert_010_rbac_postconditions;
DROP PROCEDURE seed_010_role_permissions;
DROP PROCEDURE seed_010_permissions;
DROP PROCEDURE migrate_010_rbac_indexes;
DROP PROCEDURE migrate_010_rbac_backfill;
DROP PROCEDURE migrate_010_rbac_schema;
DROP PROCEDURE assert_010_rbac_preconditions;
