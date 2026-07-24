-- 客户端加密收口：服务端只保存客户端提交的密文算法元数据，不保存明文 DEK。
-- 使用 information_schema 判断以兼容不支持 ADD COLUMN IF NOT EXISTS 的 MySQL 版本。
DROP PROCEDURE IF EXISTS migrate_015_client_ciphertext_metadata;
DELIMITER $$
CREATE PROCEDURE migrate_015_client_ciphertext_metadata()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ciphertext_objects' AND COLUMN_NAME = 'content_algorithm') THEN
    ALTER TABLE ciphertext_objects ADD COLUMN content_algorithm VARCHAR(64) NOT NULL DEFAULT 'AES-256-GCM' AFTER container_format;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ciphertext_objects' AND COLUMN_NAME = 'encryption_version') THEN
    ALTER TABLE ciphertext_objects ADD COLUMN encryption_version VARCHAR(32) NOT NULL DEFAULT '1' AFTER content_algorithm;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ciphertext_objects' AND COLUMN_NAME = 'nonce_prefix_base64') THEN
    ALTER TABLE ciphertext_objects ADD COLUMN nonce_prefix_base64 VARCHAR(64) NOT NULL DEFAULT '' AFTER encryption_version;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ciphertext_objects' AND COLUMN_NAME = 'authentication_tag_length') THEN
    ALTER TABLE ciphertext_objects ADD COLUMN authentication_tag_length INT UNSIGNED NOT NULL DEFAULT 16 AFTER nonce_prefix_base64;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ciphertext_objects' AND COLUMN_NAME = 'aad_version') THEN
    ALTER TABLE ciphertext_objects ADD COLUMN aad_version VARCHAR(32) NOT NULL DEFAULT '1' AFTER authentication_tag_length;
  END IF;
END$$
DELIMITER ;
CALL migrate_015_client_ciphertext_metadata();
DROP PROCEDURE IF EXISTS migrate_015_client_ciphertext_metadata;
