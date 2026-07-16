-- 客户端加密收口：服务端只保存客户端提交的密文算法元数据，不保存明文 DEK。
ALTER TABLE ciphertext_objects
  ADD COLUMN IF NOT EXISTS content_algorithm VARCHAR(64) NOT NULL DEFAULT 'AES-256-GCM' AFTER container_format,
  ADD COLUMN IF NOT EXISTS encryption_version VARCHAR(32) NOT NULL DEFAULT '1' AFTER content_algorithm,
  ADD COLUMN IF NOT EXISTS nonce_prefix_base64 VARCHAR(64) NOT NULL DEFAULT '' AFTER encryption_version,
  ADD COLUMN IF NOT EXISTS authentication_tag_length INT UNSIGNED NOT NULL DEFAULT 16 AFTER nonce_prefix_base64,
  ADD COLUMN IF NOT EXISTS aad_version VARCHAR(32) NOT NULL DEFAULT '1' AFTER authentication_tag_length;
