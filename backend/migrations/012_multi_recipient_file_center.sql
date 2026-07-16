-- 012_multi_recipient_file_center.sql
-- 多接收者文件中心增量迁移；解除单文件单 protected DEK 限制并补齐性能指标字段。

DROP PROCEDURE IF EXISTS cpabe_drop_index_if_exists;
DELIMITER $$
CREATE PROCEDURE cpabe_drop_index_if_exists(IN p_table VARCHAR(128), IN p_index VARCHAR(128))
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = p_table AND index_name = p_index
  ) THEN
    SET @sql = CONCAT('ALTER TABLE `', p_table, '` DROP INDEX `', p_index, '`');
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS cpabe_add_column_if_missing;
DELIMITER $$
CREATE PROCEDURE cpabe_add_column_if_missing(IN p_table VARCHAR(128), IN p_column VARCHAR(128), IN p_definition TEXT)
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = p_table AND column_name = p_column
  ) THEN
    SET @sql = CONCAT('ALTER TABLE `', p_table, '` ADD COLUMN ', p_definition);
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS cpabe_add_index_if_missing;
DELIMITER $$
CREATE PROCEDURE cpabe_add_index_if_missing(IN p_table VARCHAR(128), IN p_index VARCHAR(128), IN p_definition TEXT)
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = p_table AND index_name = p_index
  ) THEN
    SET @sql = CONCAT('ALTER TABLE `', p_table, '` ADD ', p_definition);
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DELIMITER ;

CALL cpabe_drop_index_if_exists('protected_keys', 'uk_protected_keys_file');
CALL cpabe_drop_index_if_exists('protected_keys', 'uk_protected_keys_attempt');
CALL cpabe_add_index_if_missing('protected_keys', 'idx_protected_keys_file', 'INDEX idx_protected_keys_file (file_id)');
CALL cpabe_add_index_if_missing('protected_keys', 'idx_protected_keys_attempt', 'INDEX idx_protected_keys_attempt (task_attempt_id)');

CALL cpabe_add_column_if_missing('rsa_protected_key_bindings', 'file_id', 'file_id BIGINT UNSIGNED NULL AFTER tenant_id');
CALL cpabe_add_column_if_missing('rsa_protected_key_bindings', 'protect_duration_ms', 'protect_duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER oaep_label_sha256');

UPDATE rsa_protected_key_bindings b
JOIN protected_keys pk ON pk.id = b.protected_key_id
SET b.file_id = pk.file_id
WHERE b.file_id IS NULL;

ALTER TABLE rsa_protected_key_bindings MODIFY COLUMN file_id BIGINT UNSIGNED NOT NULL;
CALL cpabe_add_index_if_missing('rsa_protected_key_bindings', 'idx_rsa_binding_file', 'INDEX idx_rsa_binding_file (tenant_id, file_id)');
CALL cpabe_add_index_if_missing('rsa_protected_key_bindings', 'uk_rsa_binding_file_recipient_key', 'UNIQUE INDEX uk_rsa_binding_file_recipient_key (tenant_id, file_id, recipient_user_id, rsa_public_key_id)');

CALL cpabe_add_column_if_missing('encryption_benchmarks', 'validation_duration_ms', 'validation_duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER task_attempt_id');
CALL cpabe_add_column_if_missing('encryption_benchmarks', 'key_protection_duration_ms', 'key_protection_duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER aes_encrypt_ms');
CALL cpabe_add_column_if_missing('encryption_benchmarks', 'metadata_commit_duration_ms', 'metadata_commit_duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER upload_ms');
CALL cpabe_add_column_if_missing('encryption_benchmarks', 'total_duration_ms', 'total_duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER metadata_commit_duration_ms');
CALL cpabe_add_column_if_missing('encryption_benchmarks', 'recipient_count', 'recipient_count BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER total_duration_ms');
CALL cpabe_add_column_if_missing('encryption_benchmarks', 'protected_key_total_size_bytes', 'protected_key_total_size_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER ciphertext_size');
CALL cpabe_add_column_if_missing('encryption_benchmarks', 'algorithm_code', 'algorithm_code VARCHAR(64) NOT NULL DEFAULT ''RSA-OAEP-SHA256'' AFTER client_runtime');
CALL cpabe_add_column_if_missing('encryption_benchmarks', 'algorithm_version', 'algorithm_version VARCHAR(32) NOT NULL DEFAULT ''1'' AFTER algorithm_code');
CALL cpabe_add_column_if_missing('encryption_benchmarks', 'result', 'result VARCHAR(32) NOT NULL DEFAULT ''SUCCESS'' AFTER algorithm_version');

CALL cpabe_add_column_if_missing('encryption_task_attempts', 'protected_key_processed', 'protected_key_processed BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER total_bytes');
CALL cpabe_add_column_if_missing('encryption_task_attempts', 'protected_key_total', 'protected_key_total BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER protected_key_processed');

DROP PROCEDURE IF EXISTS cpabe_drop_index_if_exists;
DROP PROCEDURE IF EXISTS cpabe_add_column_if_missing;
DROP PROCEDURE IF EXISTS cpabe_add_index_if_missing;
