-- 011_encrypted_file_framework.sql
-- DO 文件加密上传基础闭环增量迁移；所有语句保持可重复执行。

CREATE TABLE IF NOT EXISTS encryption_algorithms (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL,
  category VARCHAR(32) NOT NULL,
  version VARCHAR(32) NOT NULL,
  authorization_type VARCHAR(64) NOT NULL,
  protected_key_format VARCHAR(64) NOT NULL,
  client_runtime VARCHAR(32) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_algorithm_code_version (code, version),
  KEY idx_encryption_algorithms_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS tenant_encryption_algorithms (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  algorithm_id BIGINT UNSIGNED NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  disabled_reason VARCHAR(255) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_algorithm (tenant_id, algorithm_id),
  KEY idx_tenant_algorithm_enabled (tenant_id, enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS rsa_public_keys (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id CHAR(36) NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  version INT UNSIGNED NOT NULL,
  fingerprint_sha256 CHAR(64) NOT NULL,
  public_key_pem TEXT NOT NULL,
  key_bits SMALLINT UNSIGNED NOT NULL,
  algorithm VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_by BIGINT UNSIGNED NOT NULL,
  disabled_by BIGINT UNSIGNED NULL,
  disabled_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_rsa_public_keys_public_id (public_id),
  UNIQUE KEY uk_rsa_key_version (tenant_id, user_id, version),
  UNIQUE KEY uk_rsa_key_fingerprint (tenant_id, fingerprint_sha256),
  KEY idx_rsa_public_keys_recipient (tenant_id, user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS encrypted_files (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id CHAR(36) NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  owner_user_id BIGINT UNSIGNED NOT NULL,
  original_filename VARCHAR(255) NOT NULL,
  display_mime_type VARCHAR(255) NULL,
  plaintext_size BIGINT UNSIGNED NOT NULL,
  status VARCHAR(32) NOT NULL,
  current_task_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  completed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_encrypted_files_public_id (public_id),
  KEY idx_encrypted_files_tenant_owner (tenant_id, owner_user_id, created_at),
  KEY idx_encrypted_files_status (tenant_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS encryption_tasks (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id CHAR(36) NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  owner_user_id BIGINT UNSIGNED NOT NULL,
  file_id BIGINT UNSIGNED NOT NULL,
  idempotency_key VARCHAR(128) NOT NULL,
  algorithm_code VARCHAR(64) NOT NULL,
  algorithm_version VARCHAR(32) NOT NULL,
  authorization_type VARCHAR(64) NOT NULL,
  authorization_snapshot JSON NOT NULL,
  authorization_snapshot_sha256 CHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  current_attempt_no INT UNSIGNED NOT NULL DEFAULT 1,
  cancel_requested_at DATETIME(3) NULL,
  failure_code VARCHAR(64) NULL,
  retryable TINYINT(1) NOT NULL DEFAULT 0,
  lock_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_encryption_tasks_public_id (public_id),
  UNIQUE KEY uk_encryption_task_idempotency (tenant_id, owner_user_id, idempotency_key),
  UNIQUE KEY uk_encryption_tasks_file_id (file_id),
  KEY idx_encryption_tasks_scope (tenant_id, owner_user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS encryption_task_attempts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id CHAR(36) NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  task_id BIGINT UNSIGNED NOT NULL,
  attempt_no INT UNSIGNED NOT NULL,
  status VARCHAR(32) NOT NULL,
  processed_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
  total_bytes BIGINT UNSIGNED NOT NULL,
  failure_code VARCHAR(64) NULL,
  failure_stage VARCHAR(64) NULL,
  retryable TINYINT(1) NOT NULL DEFAULT 0,
  started_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  finished_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_encryption_attempts_public_id (public_id),
  UNIQUE KEY uk_task_attempt_no (task_id, attempt_no),
  KEY idx_encryption_attempts_scope (tenant_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ciphertext_objects (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id CHAR(36) NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  file_id BIGINT UNSIGNED NULL,
  task_attempt_id BIGINT UNSIGNED NOT NULL,
  object_key VARCHAR(512) NOT NULL,
  storage_backend VARCHAR(32) NOT NULL,
  container_format VARCHAR(32) NOT NULL,
  ciphertext_size BIGINT UNSIGNED NOT NULL,
  ciphertext_sha256 CHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  available_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_ciphertext_objects_public_id (public_id),
  UNIQUE KEY uk_ciphertext_objects_file (file_id),
  UNIQUE KEY uk_ciphertext_objects_attempt (task_attempt_id),
  UNIQUE KEY uk_ciphertext_objects_key (object_key),
  KEY idx_ciphertext_objects_scope (tenant_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS protected_keys (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id CHAR(36) NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  file_id BIGINT UNSIGNED NOT NULL,
  task_attempt_id BIGINT UNSIGNED NOT NULL,
  algorithm_code VARCHAR(64) NOT NULL,
  algorithm_version VARCHAR(32) NOT NULL,
  protected_key_format VARCHAR(64) NOT NULL,
  protected_key BLOB NOT NULL,
  context_sha256 CHAR(64) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_protected_keys_public_id (public_id),
  UNIQUE KEY uk_protected_keys_file (file_id),
  UNIQUE KEY uk_protected_keys_attempt (task_attempt_id),
  KEY idx_protected_keys_tenant (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS rsa_protected_key_bindings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  protected_key_id BIGINT UNSIGNED NOT NULL,
  recipient_user_id BIGINT UNSIGNED NOT NULL,
  rsa_public_key_id BIGINT UNSIGNED NOT NULL,
  public_key_fingerprint_sha256 CHAR(64) NOT NULL,
  oaep_hash VARCHAR(32) NOT NULL,
  oaep_label_sha256 CHAR(64) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_rsa_binding_protected_key (protected_key_id),
  KEY idx_rsa_binding_scope (tenant_id, recipient_user_id),
  KEY idx_rsa_binding_public_key (rsa_public_key_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS encryption_benchmarks (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  task_attempt_id BIGINT UNSIGNED NOT NULL,
  plaintext_size BIGINT UNSIGNED NOT NULL,
  ciphertext_size BIGINT UNSIGNED NOT NULL,
  aes_encrypt_ms BIGINT UNSIGNED NOT NULL,
  dek_protect_ms BIGINT UNSIGNED NOT NULL,
  upload_ms BIGINT UNSIGNED NOT NULL,
  peak_working_set_bytes BIGINT UNSIGNED NULL,
  client_runtime VARCHAR(64) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_encryption_benchmarks_attempt (task_attempt_id),
  KEY idx_encryption_benchmarks_tenant (tenant_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS orphan_storage_objects (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  task_attempt_id BIGINT UNSIGNED NULL,
  object_key VARCHAR(512) NOT NULL,
  reason_code VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  retry_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) NULL,
  next_retry_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  cleaned_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_orphan_storage_object_key (object_key),
  KEY idx_orphan_storage_claim (status, next_retry_at),
  KEY idx_orphan_storage_tenant (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id CHAR(36) NOT NULL,
  tenant_id BIGINT UNSIGNED NULL,
  actor_user_id BIGINT UNSIGNED NULL,
  action VARCHAR(128) NOT NULL,
  target_type VARCHAR(64) NOT NULL,
  target_public_id VARCHAR(64) NULL,
  result VARCHAR(32) NOT NULL,
  source_trust VARCHAR(32) NOT NULL,
  error_code VARCHAR(64) NULL,
  request_id VARCHAR(128) NULL,
  metadata JSON NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_audit_logs_public_id (public_id),
  KEY idx_audit_logs_scope (tenant_id, created_at),
  KEY idx_audit_logs_actor (actor_user_id, created_at),
  KEY idx_audit_logs_action (action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO encryption_algorithms
  (code, display_name, category, version, authorization_type, protected_key_format, client_runtime, status, created_at, updated_at)
VALUES
  ('RSA-OAEP-SHA256', 'RSA + AES', 'PUBLIC_KEY', '1', 'RSA_RECIPIENT', 'RSA-OAEP-SHA256-RAW', 'LOCAL_GO_WORKER', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE
  display_name = VALUES(display_name), authorization_type = VALUES(authorization_type),
  protected_key_format = VALUES(protected_key_format), client_runtime = VALUES(client_runtime), status = VALUES(status), updated_at = CURRENT_TIMESTAMP(3);

INSERT INTO tenant_encryption_algorithms (tenant_id, algorithm_id, enabled, created_at, updated_at)
SELECT t.id, a.id, 1, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)
FROM tenants t
JOIN encryption_algorithms a ON a.code = 'RSA-OAEP-SHA256' AND a.version = '1'
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP(3);

INSERT INTO permissions (code, name, description, scope_type, resource_type, action, status, created_at, updated_at)
VALUES
  ('crypto.key.self.manage', '管理自己的 RSA 公钥', '当前租户成员登记和查看自己的 RSA 公钥版本', 'TENANT', 'crypto_key', 'self_manage', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
  ('crypto.key.manage', '管理租户 RSA 公钥', '租户管理员禁用或撤销成员 RSA 公钥', 'TENANT', 'crypto_key', 'manage', 'ACTIVE', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE name = VALUES(name), description = VALUES(description), status = 'ACTIVE', updated_at = CURRENT_TIMESTAMP(3);

INSERT IGNORE INTO role_permissions (role_id, permission_id, granted_by, created_at)
SELECT r.id, p.id, NULL, CURRENT_TIMESTAMP(3)
FROM roles r
JOIN permissions p ON p.code = 'crypto.key.self.manage'
WHERE r.tenant_id = 0 AND r.code IN ('DO', 'DU');

INSERT IGNORE INTO role_permissions (role_id, permission_id, granted_by, created_at)
SELECT r.id, p.id, NULL, CURRENT_TIMESTAMP(3)
FROM roles r
JOIN permissions p ON p.code = 'crypto.key.manage'
WHERE r.tenant_id = 0 AND r.code = 'TENANT_ADMIN';
