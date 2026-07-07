CREATE TABLE IF NOT EXISTS tenants (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(128) NOT NULL,
  code VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'enabled',
  description VARCHAR(512) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenants_code (code),
  KEY idx_tenants_status (status),
  KEY idx_tenants_deleted_at (deleted_at)
);

CREATE TABLE IF NOT EXISTS tenant_users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_users_tenant_user (tenant_id, user_id),
  KEY idx_tenant_users_user_id (user_id),
  KEY idx_tenant_users_tenant_id (tenant_id),
  KEY idx_tenant_users_status (status),
  KEY idx_tenant_users_deleted_at (deleted_at)
);

CREATE TABLE IF NOT EXISTS roles (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  scope VARCHAR(32) NOT NULL,
  description VARCHAR(512) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_roles_code (code),
  KEY idx_roles_scope (scope),
  KEY idx_roles_deleted_at (deleted_at)
);

CREATE TABLE IF NOT EXISTS user_roles (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  role_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_user_roles_tenant_user_role (tenant_id, user_id, role_id),
  KEY idx_user_roles_user_id (user_id),
  KEY idx_user_roles_tenant_id (tenant_id),
  KEY idx_user_roles_role_id (role_id),
  KEY idx_user_roles_deleted_at (deleted_at)
);
