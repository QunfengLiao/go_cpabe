# 数据模型：多接收者文件中心与本地解密闭环

## EncryptedFile

表示一份加密文件的业务元数据。文件内容只有一份密文对象，明文文件和本地路径永不入库。

| 字段 | 含义 | 规则 |
|------|------|------|
| `id` | 内部主键 | 不对外返回 |
| `public_id` | 文件外部 UUID | 全局唯一 |
| `tenant_id` | 文件所属租户 | 所有查询必须带租户范围 |
| `owner_user_id` | 文件拥有者 DO | 我的加密文件只按该字段过滤 |
| `original_filename` | 原始展示文件名 | 只用于展示和下载建议名，不用于本地路径 |
| `display_mime_type` | 展示 MIME | 不参与安全判断 |
| `plaintext_size` | 明文大小 | 创建任务时冻结 |
| `status` | 文件状态 | `DRAFT`、`AVAILABLE`、`FAILED`、`CANCELLED`、`CLEANUP_PENDING` |
| `current_task_id` | 当前任务 | 指向创建该文件的任务 |
| `completed_at` | 完成时间 | 仅 AVAILABLE 后存在 |

**关系**：一个 `EncryptedFile` 有一个可用 `CiphertextObject`，有多条 `ProtectedDEK`，有一个 owner。

## EncryptionTask

表示一次不可变加密意图。

| 字段 | 含义 | 规则 |
|------|------|------|
| `public_id` | 任务 UUID | 对外使用 |
| `tenant_id` | 租户 | 来自可信上下文 |
| `owner_user_id` | 发起用户 | 必须是当前用户 |
| `file_id` | 文件记录 | 一任务一文件 |
| `idempotency_key` | 幂等键 | 同一租户、owner、key 唯一 |
| `algorithm_code` | DEK 保护算法 | 首期 `RSA-OAEP-SHA256` |
| `algorithm_version` | 算法版本 | 首期 `1` |
| `authorization_type` | 授权类型 | 首期 `RSA_RECIPIENTS` |
| `authorization_snapshot` | 接收者快照 | 规范化数组，不含私钥和明文 DEK |
| `authorization_snapshot_sha256` | 快照摘要 | 绑定容器与 protected DEK |
| `status` | 任务状态 | 与执行状态同步 |

**接收者快照结构**：

```json
{
  "type": "RSA_RECIPIENTS",
  "recipients": [
    {
      "recipient_user_id": 6,
      "rsa_public_key_id": "uuid",
      "public_key_version": 2,
      "public_key_fingerprint_sha256": "hex",
      "owner": true
    }
  ]
}
```

## EncryptionTaskAttempt

表示任务的一次执行。

| 字段 | 含义 | 规则 |
|------|------|------|
| `public_id` | 执行 UUID | 对外使用 |
| `tenant_id` | 租户 | 冗余隔离 |
| `task_id` | 所属任务 | 与 attempt_no 唯一 |
| `attempt_no` | 执行序号 | 从 1 递增 |
| `status` | 阶段状态 | `PENDING`、`VALIDATING`、`ENCRYPTING_FILE`、`PROTECTING_KEY`、`UPLOADING`、`SAVING_METADATA`、终态 |
| `processed_bytes` | 处理字节 | AES/上传阶段真实更新 |
| `total_bytes` | 总字节 | 创建时冻结 |
| `protected_recipient_count` | 已保护接收者数量 | DEK 保护阶段真实更新 |
| `total_recipient_count` | 总接收者数量 | 创建时冻结 |
| `failure_code` | 稳定失败码 | 脱敏 |
| `retryable` | 是否可重试 | 终态后判断 |

## ProtectedDEK

表示一份被算法保护的 DEK。复用 `protected_keys` 表并扩展为多条记录。

| 字段 | 含义 | 规则 |
|------|------|------|
| `public_id` | protected DEK UUID | 对外可引用但默认不返回完整密文 |
| `tenant_id` | 租户 | 强制隔离 |
| `file_id` | 文件 | 同一文件可有多条 |
| `task_attempt_id` | 生成执行 | 同一执行可有多条 |
| `algorithm_code` | 保护算法 | 与任务一致 |
| `algorithm_version` | 算法版本 | 与任务一致 |
| `protected_key_format` | 格式 | 首期 `RSA-OAEP-SHA256-RAW` |
| `protected_key` | 受保护 DEK 字节 | 可随可见文件的密钥信封返回 base64；它不是明文 DEK，不能作为 RBAC 解密授权 |
| `context_sha256` | 容器上下文摘要 | 与 AES 元数据一致 |
| `protected_key_size` | 字节大小 | 用于性能指标 |

**唯一约束**：同一 `file_id`、同一接收者、同一公钥版本不得重复保存相同授权。该约束通过 RSA 绑定表的唯一键表达。

## RSAProtectedKeyBinding

表示 RSA 专属接收者绑定。

| 字段 | 含义 | 规则 |
|------|------|------|
| `tenant_id` | 租户 | 与 protected DEK 一致 |
| `protected_key_id` | protected DEK | 一对一指向单条受保护 DEK |
| `file_id` | 文件 | 便于唯一约束和查询 |
| `recipient_user_id` | 接收者用户 | 必须属于当前租户 |
| `rsa_public_key_id` | 内部公钥版本 | 必须属于接收者和租户 |
| `public_key_version` | 公钥版本号 | 历史绑定保留 |
| `public_key_fingerprint_sha256` | 公钥指纹 | 加密时冻结 |
| `oaep_hash` | OAEP 哈希 | 首期 `SHA-256` |
| `oaep_label_sha256` | OAEP label 摘要 | 等于上下文摘要 |
| `protect_duration_ms` | 单接收者耗时 | 用于性能拆分 |

**唯一约束**：`tenant_id + file_id + recipient_user_id + public_key_version` 唯一。

## CiphertextObject

表示服务端保存的一份密文对象。

| 字段 | 含义 | 规则 |
|------|------|------|
| `public_id` | 上传 UUID | 完成协议使用 |
| `tenant_id` | 租户 | 强制隔离 |
| `file_id` | 文件 | 完成前可空，完成后一文件一对象 |
| `task_attempt_id` | 执行 | 一次执行最多一个对象 |
| `object_key` | 存储内部路径 | 永不对外返回 |
| `ciphertext_size` | 密文大小 | 服务端复核 |
| `ciphertext_sha256` | 密文摘要 | 下载响应头返回 |
| `status` | 对象状态 | `STAGING`、`AVAILABLE`、`DELETE_PENDING`、`DELETED` |

客户端加密收口字段：

| 字段 | 含义 | 规则 |
|------|------|------|
| `content_algorithm` | 文件内容算法 | 固定 `AES-256-GCM`，由 Electron 提交 |
| `encryption_version` | 密文容器版本 | 固定 `1` |
| `nonce_prefix_base64` | 客户端生成的 nonce 前缀 | 仅为 GCM 容器元数据，不是密钥 |
| `authentication_tag_length` | 每个分块认证标签长度 | 固定 16；标签随分块密文保存 |
| `aad_version` | AAD 版本 | 固定 `1` |

后端只校验并保存这些元数据，不生成 AES DEK、不保存原始 DEK、不接收明文文件。

## EncryptionBenchmark

表示任务级性能指标。

| 字段 | 含义 |
|------|------|
| `validation_duration_ms` | 创建任务和授权验证耗时 |
| `file_encryption_duration_ms` | 本地 AES-GCM 文件加密耗时 |
| `key_protection_duration_ms` | 所有接收者 DEK 保护总耗时 |
| `key_protection_avg_ms` | 平均单接收者 DEK 保护耗时 |
| `key_protection_min_ms` | 最小单接收者耗时 |
| `key_protection_max_ms` | 最大单接收者耗时 |
| `upload_duration_ms` | 密文上传耗时 |
| `metadata_commit_duration_ms` | 元数据提交耗时 |
| `total_duration_ms` | 用户确认执行到提交完成总耗时 |
| `recipient_count` | 接收者数量 |
| `plaintext_size_bytes` | 明文大小 |
| `ciphertext_size_bytes` | 密文大小 |
| `protected_key_total_size_bytes` | 受保护 DEK 总大小 |
| `algorithm_code` | DEK 保护算法 |
| `algorithm_version` | 算法版本 |
| `client_runtime` | 运行时 |
| `result` | 成功或失败 |

## FileCenterItem（密文仓库列表）

后端列表 DTO，不直接对应单表。

| 字段 | 含义 |
|------|------|
| `file` | 文件基本信息 |
| `owner` | owner 用户摘要 |
| `ciphertext_size` | 服务端保存的密文大小 | 用于列表展示和下载校验 |
| `recipient_summary` | 接收者姓名摘要和人数 |
| `performance_summary` | AES + DEK 保护耗时摘要 |

列表不返回 `can_decrypt`、`authorization_status`、`has_key_envelope` 或类似授权推断字段。所有有效租户成员都可以查看和下载可用密文；能否恢复明文只由客户端本地私钥和信封决定。

## KeyEnvelope

随可见文件返回的单个密钥信封。

| 字段 | 含义 | 安全规则 |
|------|------|----------|
| `key_id` | 受保护 DEK 的外部标识 | 用于审计和信封定位，不是私钥 |
| `protected_key_base64` | RSA-OAEP 密文 | 可公开传输，不包含明文 DEK |
| `rsa_public_key_id` | 目标公钥版本 UUID | 与本地私钥索引匹配 |
| `public_key_fingerprint_sha256` | 目标公钥指纹 | 与本地公钥指纹二次校验 |
| `context_sha256` | OAEP label/AES 上下文摘要 | 防止跨文件、跨版本替换 |
| `algorithm_code` / `algorithm_version` | 保护算法及版本 | 由 Worker 选择解封适配器 |

## DecryptionMaterial

随可见文件返回给当前用户的本地解密材料。

| 字段 | 含义 | 安全规则 |
|------|------|----------|
| `file_id` | 文件 UUID | 绑定容器 |
| `original_filename` | 建议输出名 | 主进程仍需清理非法字符 |
| `plaintext_size` | 明文大小 | 非敏感 |
| `content_encryption` | AES 元数据 | 不含 DEK |
| `key_envelopes` | 文件全部密钥信封 | 不包含私钥、明文 DEK或明文；客户端自行匹配 |
| `rsa_public_key_id` | 首个兼容字段 | 新客户端以 `key_envelopes` 为准 |
| `public_key_fingerprint_sha256` | 首个兼容字段 | 不作为服务端授权判断 |

## AuditOutboxEvent

表示已经完成安全过滤、等待投递到 `audit_logs` 的内部审计事件。它不对应文件对象，不得被文件孤儿清理命令领取，也不通过业务 API 返回。

| 字段 | 含义 | 规则 |
|------|------|------|
| `id` | 内部主键 | 不对外返回 |
| `event_public_id` | 稳定事件 UUID | 全局唯一；投递后复用为 `audit_logs.public_id`，重试不得重新生成 |
| `dedup_key` | 生产者业务去重摘要 | 可空、唯一；由版本、租户、动作、公开目标和业务转换标识生成，不得只使用请求 ID |
| `tenant_id` | 可信租户范围 | 平台事件可空；禁止从 Metadata 覆盖 |
| `actor_user_id` | 可信操作者 | 系统事件可空 |
| `action` | 稳定动作编码 | 最大 128 字符，入队后不可变 |
| `target_type` | 目标类型 | 最大 64 字符 |
| `target_public_id` | 非敏感目标标识 | 不保存对象内部路径 |
| `result` | 业务结果 | `SUCCESS`、`FAILURE` 或 `DENIED` |
| `source_trust` | 事实来源 | `SERVER_OBSERVED` 或 `CLIENT_REPORTED` |
| `error_code` | 稳定脱敏错误码 | 不保存原始 `error` 或 SQL 文本 |
| `request_id` | 请求关联标识 | 可空，不得保存 Token |
| `metadata` | 已过滤 JSON | 只允许当前 action 声明的非敏感标量，限制序列化大小；禁止原始任意 Map |
| `metadata_redacted` | 元数据是否被清空 | 原 Metadata 校验失败时为 true，只保留顶层最小事件 |
| `payload_version` | 事件结构版本 | 首期为 `1`，便于兼容重放 |
| `occurred_at` | 事件实际发生时间 | 投递时写入正式日志，不能改成重放时间 |
| `status` | 投递状态 | `PENDING`、`PROCESSING`、`RETRY`、`DELIVERED`、`DEAD_LETTER` |
| `retry_count` | 已失败次数 | 每次投递失败递增 |
| `next_retry_at` | 下次可领取时间 | `PENDING` 可空，`RETRY` 必须存在 |
| `locked_at` | 当前租约开始时间 | 非 `PROCESSING` 状态清空 |
| `lock_token` | 当前租约随机令牌 | 完成或失败更新必须匹配，禁止旧 Worker 覆盖新租约 |
| `last_error_code` | 最近脱敏投递错误 | 只保存稳定枚举，不保存堆栈和秘密 |
| `delivered_at` | 正式日志确认时间 | 仅 `DELIVERED` 存在 |
| `created_at` | 入队时间 | 数据库生成 |
| `updated_at` | 最近状态变化时间 | 数据库或仓储更新 |

**索引和约束**：

- `UNIQUE(event_public_id)`：并发入队和重复重放幂等。
- `UNIQUE(dedup_key)`：生产者重复发射同一业务事实时收敛；空值允许重复。
- `(status, next_retry_at, id)`：按状态和到期时间批量领取。
- `(status, locked_at)`：恢复租约过期的 `PROCESSING` 事件。
- `(tenant_id, status, created_at)`：租户范围运维诊断，不对普通租户 API 暴露。
- 不设置到业务对象或用户的外键；即使业务对象后续删除，审计证据仍需保留。

**状态转换**：

```text
PENDING -> PROCESSING -> DELIVERED
PROCESSING -> RETRY -> PROCESSING
PROCESSING -> DEAD_LETTER
PROCESSING（租约过期）-> 被新 lock_token 重新领取
DEAD_LETTER -> RETRY（仅人工修复后的受控重放）
```

Dispatcher 在同一事务内幂等写入 `audit_logs` 并把 outbox 标记为 `DELIVERED`。如果正式日志已存在相同 `event_public_id`，应视为前次投递已成功并完成状态收敛。

正式 `AuditLog` 同步增加 `metadata_redacted` 布尔字段；Dispatcher 原样传递该标记，让审计读取方区分“事件没有补充信息”和“补充信息因安全校验被清空”。

## 状态转换

```text
DRAFT/PENDING
  -> VALIDATING
  -> ENCRYPTING_FILE
  -> PROTECTING_KEY
  -> UPLOADING
  -> SAVING_METADATA
  -> AVAILABLE/COMPLETED

任意可中断阶段 -> FAILED 或 CANCELLED
提交元数据完成后 -> 不再允许取消
```
