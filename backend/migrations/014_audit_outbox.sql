-- 014_audit_outbox.sql
-- 独立审计 Outbox：可靠保存已经脱敏的待投递事件，不复用会触发密文删除的孤儿对象表。

CREATE TABLE IF NOT EXISTS audit_outbox (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_public_id CHAR(36) NOT NULL,
  dedup_key VARCHAR(128) NULL,
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
  metadata_redacted TINYINT(1) NOT NULL DEFAULT 0,
  payload_version SMALLINT UNSIGNED NOT NULL DEFAULT 1,
  occurred_at DATETIME(3) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
  retry_count INT UNSIGNED NOT NULL DEFAULT 0,
  next_retry_at DATETIME(3) NULL,
  locked_at DATETIME(3) NULL,
  lock_token CHAR(36) NULL,
  last_error_code VARCHAR(64) NULL,
  delivered_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_audit_outbox_event_public_id (event_public_id),
  UNIQUE KEY uk_audit_outbox_dedup_key (dedup_key),
  KEY idx_audit_outbox_claim (status, next_retry_at, id),
  KEY idx_audit_outbox_lease (status, locked_at),
  KEY idx_audit_outbox_tenant_status (tenant_id, status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 兼容已经执行过 011 的开发库；通过 information_schema 守卫保证重复执行不会再次添加列。
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

CALL cpabe_add_column_if_missing('audit_logs', 'metadata_redacted', 'metadata_redacted TINYINT(1) NOT NULL DEFAULT 0 AFTER metadata');

DROP PROCEDURE IF EXISTS cpabe_add_column_if_missing;
