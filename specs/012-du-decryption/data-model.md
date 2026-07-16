# 数据模型

不新增数据库表。复用 `encrypted_files`、`ciphertext_objects`、`protected_keys`、`rsa_protected_key_bindings` 和 `rsa_public_keys`。接收者查询链为：文件 → 受保护密钥 → RSA 绑定 → 历史公钥，且每一步都带可信 `tenant_id`。
