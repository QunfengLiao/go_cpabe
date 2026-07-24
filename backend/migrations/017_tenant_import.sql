CREATE TABLE IF NOT EXISTS tenant_import_batches (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  batch_id CHAR(36) NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  import_type VARCHAR(32) NOT NULL,
  file_name VARCHAR(255) NOT NULL,
  file_hash CHAR(64) NOT NULL,
  snapshot_hash CHAR(64) NOT NULL,
  rows_json JSON NOT NULL,
  total_count INT UNSIGNED NOT NULL DEFAULT 0,
  valid_count INT UNSIGNED NOT NULL DEFAULT 0,
  success_count INT UNSIGNED NOT NULL DEFAULT 0,
  failure_count INT UNSIGNED NOT NULL DEFAULT 0,
  skipped_count INT UNSIGNED NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL,
  created_by BIGINT UNSIGNED NOT NULL,
  validated_at DATETIME(3) NULL,
  confirmed_at DATETIME(3) NULL,
  completed_at DATETIME(3) NULL,
  failure_reason VARCHAR(512) NULL,
  phase VARCHAR(32) NOT NULL DEFAULT 'WAITING',
  processed_count INT UNSIGNED NOT NULL DEFAULT 0,
  lease_token CHAR(36) NULL,
  lease_expires_at DATETIME(3) NULL,
  heartbeat_at DATETIME(3) NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_import_batches_batch_id (batch_id),
  KEY idx_tenant_import_batches_tenant_created (tenant_id, created_at),
  KEY idx_tenant_import_batches_tenant_creator (tenant_id, created_by),
  KEY idx_tenant_import_batches_lease (status, lease_expires_at, confirmed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO permissions (code, name, description, scope_type, resource_type, action, status, created_at, updated_at)
VALUES ('tenant.import.manage', '管理批量导入', '下载模板、校验和确认当前租户批量导入', 'TENANT', 'tenant_import', 'manage', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE name = VALUES(name), description = VALUES(description), status = VALUES(status), updated_at = CURRENT_TIMESTAMP(3);

INSERT IGNORE INTO role_permissions (role_id, permission_id, granted_by, created_at)
SELECT r.id, p.id, NULL, CURRENT_TIMESTAMP(3)
FROM roles r
JOIN permissions p ON p.code = 'tenant.import.manage'
WHERE r.tenant_id = 0 AND r.code = 'TENANT_ADMIN';
