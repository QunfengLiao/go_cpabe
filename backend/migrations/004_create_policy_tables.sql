CREATE TABLE IF NOT EXISTS policy_attributes (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  attr_code VARCHAR(64) NOT NULL,
  attr_name VARCHAR(128) NOT NULL,
  attr_type VARCHAR(32) NOT NULL,
  attr_values JSON NULL,
  description TEXT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'enabled',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_policy_attributes_attr_code (attr_code),
  KEY idx_policy_attributes_status (status),
  KEY idx_policy_attributes_deleted_at (deleted_at)
);

CREATE TABLE IF NOT EXISTS policy_templates (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(128) NOT NULL,
  description TEXT NULL,
  policy_expr TEXT NOT NULL,
  policy_tree_json JSON NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'enabled',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_policy_templates_name (name),
  KEY idx_policy_templates_status (status),
  KEY idx_policy_templates_deleted_at (deleted_at)
);

CREATE TABLE IF NOT EXISTS access_policies (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  owner_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(128) NOT NULL,
  description TEXT NULL,
  policy_expr TEXT NOT NULL,
  policy_tree_json JSON NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'enabled',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_access_policies_tenant_owner (tenant_id, owner_id),
  KEY idx_access_policies_tenant_status (tenant_id, status),
  KEY idx_access_policies_deleted_at (deleted_at),
  KEY idx_access_policies_tenant_owner_name (tenant_id, owner_id, name)
);
