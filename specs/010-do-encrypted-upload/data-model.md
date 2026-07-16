# 数据模型：DO 文件加密上传基础闭环与通用混合加密框架

## 设计原则

- 所有租户业务表必须包含 `tenant_id`，所有资源查询必须同时限定可信租户。
- 通用文件、任务和受保护密钥表不得出现固定 `rsa_*` 字段；RSA 专有关系使用独立表。
- 明文文件、明文 DEK、RSA 私钥和私钥口令不得进入任何服务端表。
- 任务、授权、算法和公钥版本在创建后形成不可变快照；重试只新增执行记录。
- 外部 API 使用不可枚举的 UUID，内部可保留 `BIGINT UNSIGNED` 主键；本文统一以 `id` 表示内部主键、`public_id` 表示外部 UUID。
- 时间字段使用毫秒精度 UTC 时间；状态字段使用受控枚举字符串。

## 实体关系概览

```text
Tenant
├── TenantEncryptionAlgorithm ── EncryptionAlgorithm
├── RSAPublicKey ── User
├── EncryptionTask ── EncryptedFile
│   └── EncryptionTaskAttempt
│       ├── CiphertextObject
│       ├── ProtectedKey
│       │   └── RSAProtectedKeyBinding ── RSAPublicKey
│       └── EncryptionBenchmark
├── OrphanStorageObject
└── AuditLog
```

## 1. 加密算法 `encryption_algorithms`

表示系统可识别的 DEK 保护算法能力，不保存密钥材料。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `code` | VARCHAR(64) | 唯一、非空 | 稳定算法编码，如 `RSA-OAEP-SHA256` |
| `display_name` | VARCHAR(128) | 非空 | 中文展示名称 |
| `category` | VARCHAR(32) | 非空 | `PUBLIC_KEY`、`CP_ABE` 等类别 |
| `version` | VARCHAR(32) | 非空 | 协议和参数版本 |
| `authorization_type` | VARCHAR(64) | 非空 | 授权配置类型，如 `RSA_RECIPIENT` |
| `protected_key_format` | VARCHAR(64) | 非空 | 受保护密钥格式编码 |
| `client_runtime` | VARCHAR(32) | 非空 | 首期为 `LOCAL_GO_WORKER`，由 Electron 受控启动 |
| `status` | VARCHAR(32) | 非空、索引 | `ACTIVE`、`DISABLED` |
| `created_at`、`updated_at` | DATETIME(3) | 非空 | 维护时间 |

**唯一约束**：`(code, version)`。

**首期种子**：只启用 `RSA-OAEP-SHA256` 版本 `1`。BSW07、Waters11、TKN20 不创建伪实现记录；如需要展示未来能力，只能以静态产品说明展示，不得标记为可执行。

## 2. 租户算法配置 `tenant_encryption_algorithms`

表示某算法版本是否允许在特定租户使用。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `tenant_id` | BIGINT UNSIGNED | 非空、索引 | 可信租户 |
| `algorithm_id` | BIGINT UNSIGNED | 非空、索引 | 算法版本 |
| `enabled` | TINYINT(1) | 非空 | 是否可用于新任务 |
| `disabled_reason` | VARCHAR(255) | 可空 | 面向用户的非敏感原因 |
| `created_at`、`updated_at` | DATETIME(3) | 非空 | 维护时间 |

**唯一约束**：`(tenant_id, algorithm_id)`。

**默认规则**：没有租户覆盖记录时，首期 RSA 是否启用由种子明确创建，不通过代码隐式放行。

## 3. RSA 公钥版本 `rsa_public_keys`

保存租户用户的 RSA 公钥历史版本。私钥永不进入该表。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `public_id` | CHAR(36) | 唯一、非空 | API 使用的不可枚举标识 |
| `tenant_id` | BIGINT UNSIGNED | 非空、复合索引 | 公钥所属租户 |
| `user_id` | BIGINT UNSIGNED | 非空、复合索引 | 公钥所属用户 |
| `version` | INT UNSIGNED | 非空 | 用户在该租户内递增版本 |
| `fingerprint_sha256` | CHAR(64) | 非空 | 规范化 SPKI DER 的 SHA-256 十六进制指纹 |
| `public_key_pem` | TEXT | 非空、敏感度低 | SPKI PEM 公钥；仅在授权接口返回 |
| `key_bits` | SMALLINT UNSIGNED | 非空 | 首期必须为 3072 |
| `algorithm` | VARCHAR(64) | 非空 | `RSA-OAEP-SHA256` |
| `status` | VARCHAR(32) | 非空、索引 | `ACTIVE`、`DISABLED`、`REVOKED` |
| `created_by` | BIGINT UNSIGNED | 非空 | 登记操作者，首期通常为本人 |
| `disabled_by` | BIGINT UNSIGNED | 可空 | 禁用操作者 |
| `disabled_at` | DATETIME(3) | 可空 | 禁用时间 |
| `created_at`、`updated_at` | DATETIME(3) | 非空 | 维护时间 |

**唯一约束**：`(tenant_id, user_id, version)`、`(tenant_id, fingerprint_sha256)`。

**验证规则**：用户是当前租户有效成员；PEM 可解析为 RSA SPKI 公钥；位数为 3072；指纹由服务端重新计算；客户端提交的版本号不可信，由事务锁定成员范围后分配。

## 4. 加密文件 `encrypted_files`

表示 DO 的业务文件记录，不包含明文或本地路径。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `public_id` | CHAR(36) | 唯一、非空 | 外部文件标识 |
| `tenant_id` | BIGINT UNSIGNED | 非空、复合索引 | 所属租户 |
| `owner_user_id` | BIGINT UNSIGNED | 非空、复合索引 | 创建该记录的 DO |
| `original_filename` | VARCHAR(255) | 非空 | 仅用于展示，已标准化且不参与对象路径 |
| `display_mime_type` | VARCHAR(255) | 可空 | 展示信息，不作为安全判断 |
| `plaintext_size` | BIGINT UNSIGNED | 非空 | 加密前大小，首期 `1..1 GiB` |
| `status` | VARCHAR(32) | 非空、索引 | `DRAFT`、`AVAILABLE`、`FAILED`、`CANCELLED` |
| `current_task_id` | BIGINT UNSIGNED | 非空 | 创建该文件的任务 |
| `completed_at` | DATETIME(3) | 可空 | 变为可用的时间 |
| `created_at`、`updated_at` | DATETIME(3) | 非空 | 维护时间 |

**访问规则**：本期只有 `owner_user_id` 在同租户上下文中可列表、查看和下载；管理员权限不自动获得文件读取权。

## 5. 密文对象 `ciphertext_objects`

描述服务端受控存储中的密文容器。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `public_id` | CHAR(36) | 唯一、非空 | API 标识 |
| `tenant_id` | BIGINT UNSIGNED | 非空、复合索引 | 所属租户 |
| `file_id` | BIGINT UNSIGNED | 可空、唯一 | 完成后关联文件；暂存阶段可空 |
| `task_attempt_id` | BIGINT UNSIGNED | 非空、唯一 | 产生该对象的执行 |
| `object_key` | VARCHAR(512) | 唯一、非空、响应隐藏 | 内部不可预测对象键 |
| `storage_backend` | VARCHAR(32) | 非空 | 首期 `LOCAL` |
| `container_format` | VARCHAR(32) | 非空 | `GCPABE01` |
| `ciphertext_size` | BIGINT UNSIGNED | 非空 | 完整容器大小 |
| `ciphertext_sha256` | CHAR(64) | 非空 | 服务端复核后的容器摘要 |
| `status` | VARCHAR(32) | 非空、索引 | `STAGING`、`AVAILABLE`、`DELETE_PENDING`、`DELETED` |
| `created_at`、`available_at`、`deleted_at` | DATETIME(3) | 后两者可空 | 生命周期时间 |

**下载条件**：文件和对象均为可用；租户、所有者、文件、对象关联一致；对象在存储层存在且大小符合记录。

## 6. 加密任务 `encryption_tasks`

表示一次用户意图及其不可变授权快照。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `public_id` | CHAR(36) | 唯一、非空 | 外部任务标识 |
| `tenant_id` | BIGINT UNSIGNED | 非空、复合索引 | 可信租户 |
| `owner_user_id` | BIGINT UNSIGNED | 非空、复合索引 | 发起 DO |
| `file_id` | BIGINT UNSIGNED | 非空、唯一 | 草稿文件记录 |
| `idempotency_key` | VARCHAR(128) | 非空 | 一次用户意图的幂等键 |
| `algorithm_code` | VARCHAR(64) | 非空 | DEK 保护算法快照 |
| `algorithm_version` | VARCHAR(32) | 非空 | 算法版本快照 |
| `authorization_type` | VARCHAR(64) | 非空 | 通用授权配置类型 |
| `authorization_snapshot` | JSON | 非空、响应脱敏 | 适配器验证后的不可变快照；允许包含公钥标识和指纹，不含私钥 |
| `authorization_snapshot_sha256` | CHAR(64) | 非空 | 规范化快照摘要 |
| `status` | VARCHAR(32) | 非空、索引 | 任务状态 |
| `current_attempt_no` | INT UNSIGNED | 非空 | 当前执行序号 |
| `cancel_requested_at` | DATETIME(3) | 可空 | 取消请求时间 |
| `failure_code` | VARCHAR(64) | 可空 | 稳定业务错误码 |
| `retryable` | TINYINT(1) | 非空 | 是否可创建新执行 |
| `lock_version` | BIGINT UNSIGNED | 非空 | 乐观版本，配合行锁防误写 |
| `created_at`、`updated_at`、`completed_at` | DATETIME(3) | 后者可空 | 生命周期时间 |

**唯一约束**：`(tenant_id, owner_user_id, idempotency_key)`。

**不可变字段**：租户、所有者、文件、算法、授权类型、授权快照及摘要。

## 7. 加密任务执行 `encryption_task_attempts`

表示初次执行或某次重试。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `public_id` | CHAR(36) | 唯一、非空 | 外部执行标识 |
| `tenant_id` | BIGINT UNSIGNED | 非空、复合索引 | 冗余租户隔离键 |
| `task_id` | BIGINT UNSIGNED | 非空、复合唯一 | 所属任务 |
| `attempt_no` | INT UNSIGNED | 非空、复合唯一 | 从 1 递增 |
| `status` | VARCHAR(32) | 非空、索引 | 执行状态 |
| `processed_bytes` | BIGINT UNSIGNED | 非空 | 已处理字节数，不能超过总量 |
| `total_bytes` | BIGINT UNSIGNED | 非空 | 原始文件总量 |
| `failure_code` | VARCHAR(64) | 可空 | 稳定错误码 |
| `failure_stage` | VARCHAR(64) | 可空 | 失败阶段 |
| `retryable` | TINYINT(1) | 非空 | 失败是否允许重试 |
| `started_at`、`updated_at`、`finished_at` | DATETIME(3) | 完成时间可空 | 执行时间 |

**唯一约束**：`(task_id, attempt_no)`。

### 状态转换

```text
PENDING
  -> VALIDATING
  -> ENCRYPTING_FILE
  -> PROTECTING_KEY
  -> UPLOADING
  -> SAVING_METADATA
  -> COMPLETED

PENDING/VALIDATING/ENCRYPTING_FILE/PROTECTING_KEY/UPLOADING
  -> CANCELLED（仅服务端确认允许时）

任一非终态 -> FAILED
```

- `COMPLETED`、`FAILED`、`CANCELLED` 不得转回运行态。
- 重试新增下一 `attempt_no`，旧执行保持终态。
- 任务的聚合状态跟随当前执行，但历史执行不得被覆盖。
- 重复进度只允许保持或前进，`processed_bytes` 不得倒退。

## 8. 通用受保护密钥 `protected_keys`

保存适配器输出的 DEK 密文，不包含算法专属外键。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `public_id` | CHAR(36) | 唯一、非空 | 外部标识 |
| `tenant_id` | BIGINT UNSIGNED | 非空、复合索引 | 所属租户 |
| `file_id` | BIGINT UNSIGNED | 非空、唯一 | 所属完成文件 |
| `task_attempt_id` | BIGINT UNSIGNED | 非空、唯一 | 生成执行 |
| `algorithm_code` | VARCHAR(64) | 非空 | 算法快照 |
| `algorithm_version` | VARCHAR(32) | 非空 | 算法版本 |
| `protected_key_format` | VARCHAR(64) | 非空 | 适配器输出格式 |
| `protected_key` | BLOB | 非空、敏感、响应默认隐藏 | 受保护 DEK，不是明文 |
| `context_sha256` | CHAR(64) | 非空 | 与 AES AAD/OAEP label 一致的上下文摘要 |
| `created_at` | DATETIME(3) | 非空 | 创建时间 |

**规则**：受保护密钥可在未来解密协议中按权限返回，但本期文件详情默认只返回格式、算法和摘要前缀，不返回完整值。

## 9. RSA 受保护密钥绑定 `rsa_protected_key_bindings`

把通用受保护密钥关联到 RSA 专属授权事实。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `tenant_id` | BIGINT UNSIGNED | 非空、复合索引 | 所属租户 |
| `protected_key_id` | BIGINT UNSIGNED | 唯一、非空 | 通用受保护密钥 |
| `recipient_user_id` | BIGINT UNSIGNED | 非空、索引 | 加密时接收用户 |
| `rsa_public_key_id` | BIGINT UNSIGNED | 非空、索引 | 加密时具体公钥版本 |
| `public_key_fingerprint_sha256` | CHAR(64) | 非空 | 防止历史引用解释变化 |
| `oaep_hash` | VARCHAR(32) | 非空 | `SHA-256` |
| `oaep_label_sha256` | CHAR(64) | 非空 | 实际 label 摘要 |
| `created_at` | DATETIME(3) | 非空 | 创建时间 |

**说明**：下一期优先实现的真实 Go `TKN20Engine` 使用自己的绑定表，不改动 `protected_keys` 和文件主流程；只有研究否决 TKN20 后才可替换为真实 Go BSW07 或其他方案。

## 10. 加密性能记录 `encryption_benchmarks`

分别记录各阶段指标，避免错误比较口径。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `tenant_id` | BIGINT UNSIGNED | 非空、复合索引 | 所属租户 |
| `task_attempt_id` | BIGINT UNSIGNED | 唯一、非空 | 对应执行 |
| `plaintext_size`、`ciphertext_size` | BIGINT UNSIGNED | 非空 | 输入和输出大小 |
| `aes_encrypt_ms` | BIGINT UNSIGNED | 非空 | 仅文件内容 AES 耗时 |
| `dek_protect_ms` | BIGINT UNSIGNED | 非空 | 仅 DEK 保护耗时 |
| `upload_ms` | BIGINT UNSIGNED | 非空 | 仅密文上传耗时 |
| `peak_working_set_bytes` | BIGINT UNSIGNED | 可空 | 客户端报告的近似内存峰值 |
| `client_runtime` | VARCHAR(64) | 非空 | Electron/Node 运行时版本摘要 |
| `created_at` | DATETIME(3) | 非空 | 记录时间 |

**安全规则**：客户端指标仅供演示与比较，不作为授权事实；服务端校验非负和合理上限。

## 11. 孤儿存储对象 `orphan_storage_objects`

记录数据库事务失败或清理失败后的密文对象。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `tenant_id` | BIGINT UNSIGNED | 非空、索引 | 所属租户 |
| `task_attempt_id` | BIGINT UNSIGNED | 可空、索引 | 来源执行 |
| `object_key` | VARCHAR(512) | 唯一、非空、响应隐藏 | 待清理内部对象键 |
| `reason_code` | VARCHAR(64) | 非空 | 产生原因 |
| `status` | VARCHAR(32) | 非空、索引 | `PENDING`、`CLEANING`、`CLEANED`、`FAILED` |
| `retry_count` | INT UNSIGNED | 非空 | 清理次数 |
| `last_error_code` | VARCHAR(64) | 可空 | 脱敏错误码 |
| `next_retry_at` | DATETIME(3) | 可空、索引 | 下次清理时间 |
| `created_at`、`updated_at`、`cleaned_at` | DATETIME(3) | 最后者可空 | 生命周期时间 |

## 12. 审计日志 `audit_logs`

替代本功能路径中的空审计实现。

| 字段 | 类型 | 约束 | 业务含义 |
|------|------|------|----------|
| `id` | BIGINT UNSIGNED | 主键 | 内部标识 |
| `public_id` | CHAR(36) | 唯一、非空 | 审计查询标识 |
| `tenant_id` | BIGINT UNSIGNED | 可空、索引 | 平台事件可空，租户事件必须非空 |
| `actor_user_id` | BIGINT UNSIGNED | 可空、索引 | 无法识别操作者时可空 |
| `action` | VARCHAR(128) | 非空、索引 | 稳定动作编码 |
| `target_type` | VARCHAR(64) | 非空 | 目标类型 |
| `target_public_id` | VARCHAR(64) | 可空 | 对外目标标识 |
| `result` | VARCHAR(32) | 非空 | `SUCCESS`、`FAILURE`、`DENIED` |
| `source_trust` | VARCHAR(32) | 非空 | `SERVER_OBSERVED` 或 `CLIENT_REPORTED`，区分服务端事实与本地 Crypto Worker 报告 |
| `error_code` | VARCHAR(64) | 可空 | 稳定业务错误码 |
| `request_id` | VARCHAR(128) | 可空、索引 | 请求关联标识 |
| `metadata` | JSON | 非空 | 只允许白名单非敏感字段 |
| `created_at` | DATETIME(3) | 非空、索引 | 事件时间 |

**禁止字段**：文件本地路径、明文、DEK、私钥、私钥口令、完整受保护密钥、生产随机数和底层堆栈。

**审计调用规则**：任务创建/校验、授权快照、公钥选择、上传、完成、失败、取消、重试、下载和清理补偿必须在对应 Service 显式调用审计。AES、DEK 保护和客户端 Benchmark 只能记为 `CLIENT_REPORTED`；上传哈希、数据库事务、鉴权和存储结果记为 `SERVER_OBSERVED`。

## 事务边界

### 创建任务

1. 在事务中校验可信租户、DO 权限、算法启用、接收成员和 RSA 公钥状态。
2. 按租户和用户查找幂等键；已存在则返回原任务。
3. 创建 `encrypted_files(DRAFT)`、`encryption_tasks(PENDING)` 和首次 `encryption_task_attempts(PENDING)`。
4. 保存规范化授权快照和摘要后提交。

### 上传密文

1. 校验任务、执行、所有者和允许状态。
2. 流式写入 `STAGING` 对象并计算摘要；不在数据库长事务中传输 1 GiB 内容。
3. 短事务保存或更新 `ciphertext_objects(STAGING)`；重复上传必须使用新执行或安全替换尚未提交的同执行暂存对象。

### 完成任务

1. 锁定任务、执行、文件和暂存对象。
2. 若已完成，返回原结果；若状态或版本冲突，拒绝。
3. 校验容器、哈希、算法、授权摘要、公钥绑定和进度。
4. 写入 `protected_keys`、适配绑定、Benchmark 和审计；更新文件与对象为 `AVAILABLE`，任务与执行为 `COMPLETED`。
5. 提交失败时删除暂存对象或记录孤儿对象。

### 取消与重试

- 取消锁定当前执行，仅允许从可中断状态转为 `CANCELLED`，同时删除暂存对象。
- 重试要求当前执行是可重试失败；锁定任务并创建递增执行，不修改授权快照。
- 若用户更换接收者、公钥或算法，必须创建新任务而不是重试。

## 数据保留与清理

- 完成文件和审计保留期限由后续治理功能确定，本期不提供 DO 删除。
- `STAGING` 对象超过配置期限且没有活动执行时转为孤儿对象。
- Electron `.part` 文件在成功、失败、取消和应用启动巡检时清理；文件名不得包含原始文件名。
- 历史 RSA 公钥禁用后仍保留，禁止物理删除有绑定引用的版本。
