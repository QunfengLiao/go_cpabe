DROP PROCEDURE IF EXISTS migrate_018_async_tenant_import;
DELIMITER $$
CREATE PROCEDURE migrate_018_async_tenant_import()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tenant_import_batches' AND COLUMN_NAME = 'phase') THEN
    ALTER TABLE tenant_import_batches ADD COLUMN phase VARCHAR(32) NOT NULL DEFAULT 'WAITING' AFTER failure_reason;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tenant_import_batches' AND COLUMN_NAME = 'processed_count') THEN
    ALTER TABLE tenant_import_batches ADD COLUMN processed_count INT UNSIGNED NOT NULL DEFAULT 0 AFTER phase;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tenant_import_batches' AND COLUMN_NAME = 'lease_token') THEN
    ALTER TABLE tenant_import_batches ADD COLUMN lease_token CHAR(36) NULL AFTER processed_count;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tenant_import_batches' AND COLUMN_NAME = 'lease_expires_at') THEN
    ALTER TABLE tenant_import_batches ADD COLUMN lease_expires_at DATETIME(3) NULL AFTER lease_token;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tenant_import_batches' AND COLUMN_NAME = 'heartbeat_at') THEN
    ALTER TABLE tenant_import_batches ADD COLUMN heartbeat_at DATETIME(3) NULL AFTER lease_expires_at;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tenant_import_batches' AND COLUMN_NAME = 'attempt_count') THEN
    ALTER TABLE tenant_import_batches ADD COLUMN attempt_count INT UNSIGNED NOT NULL DEFAULT 0 AFTER heartbeat_at;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tenant_import_batches' AND INDEX_NAME = 'idx_tenant_import_batches_lease') THEN
    ALTER TABLE tenant_import_batches ADD INDEX idx_tenant_import_batches_lease (status, lease_expires_at, confirmed_at);
  END IF;
END$$
DELIMITER ;
CALL migrate_018_async_tenant_import();
DROP PROCEDURE IF EXISTS migrate_018_async_tenant_import;
