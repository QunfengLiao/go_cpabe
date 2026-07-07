# 数据模型：CP-ABE 系统多租户基础能力

## 现有用户 User

当前 `users` 表已经存在，作为账号主体。

| 字段 | 类型建议 | 必填 | 说明 |
|------|----------|------|------|
| `id` | `BIGINT UNSIGNED` | 是 | 用户主键 |
| `email` | `VARCHAR(255)` | 是 | 登录邮箱，全局唯一 |
| `password_hash` | `VARCHAR(255)` | 是 | 密码哈希，不对外返回 |
| `nickname` | `VARCHAR(64)` | 是 | 昵称 |
| `avatar_url` | `VARCHAR(512)` | 否 | 头像访问地址 |
| `avatar_object_key` | `VARCHAR(512)` | 否 | 头像内部存储标识，不对外返回 |
| `role` | `VARCHAR(32)` | 是 | 旧全局角色，MVP 仅兼容使用 |
| `status` | `VARCHAR(32)` | 是 | `active` 或 `disabled` |
| `bio` | `VARCHAR(200)` | 否 | 个人简介 |
| `birthday` | `DATE` | 否 | 生日 |
| `created_at` | `DATETIME(3)` | 是 | 创建时间 |
| `updated_at` | `DATETIME(3)` | 是 | 更新时间 |
| `deleted_at` | `DATETIME(3)` | 否 | 软删除时间 |

索引：

- `uk_users_email(email)`。
- `idx_users_role(role)`，过渡期保留。
- `idx_users_status(status)`。
- `idx_users_deleted_at(deleted_at)`。

规则：

- `users.role` 不再作为租户内授权依据。
- 公开注册保留 `data_owner` 和 `data_user`，用于映射默认租户角色。

## 租户 Tenant

租户是组织级数据隔离边界。

| 字段 | 类型建议 | 必填 | 说明 |
|------|----------|------|------|
| `id` | `BIGINT UNSIGNED` | 是 | 租户主键 |
| `name` | `VARCHAR(128)` | 是 | 租户名称 |
| `code` | `VARCHAR(64)` | 是 | 租户编码，全局唯一 |
| `status` | `VARCHAR(32)` | 是 | `enabled` 或 `disabled` |
| `description` | `VARCHAR(512)` | 否 | 租户描述 |
| `created_at` | `DATETIME(3)` | 是 | 创建时间 |
| `updated_at` | `DATETIME(3)` | 是 | 更新时间 |
| `deleted_at` | `DATETIME(3)` | 否 | 软删除时间 |

索引：

- `uk_tenants_code(code)`。
- `idx_tenants_status(status)`。
- `idx_tenants_deleted_at(deleted_at)`。

校验规则：

- `name` 不能为空。
- `code` 不能为空，只允许稳定的短编码，建议使用小写字母、数字和中划线。
- 禁用租户不能被切换进入，也不能访问租户内业务数据。

## 租户用户 TenantUser

表示用户是否属于某个租户。

| 字段 | 类型建议 | 必填 | 说明 |
|------|----------|------|------|
| `id` | `BIGINT UNSIGNED` | 是 | 关系主键 |
| `tenant_id` | `BIGINT UNSIGNED` | 是 | 租户 ID |
| `user_id` | `BIGINT UNSIGNED` | 是 | 用户 ID |
| `status` | `VARCHAR(32)` | 是 | `active` 或 `disabled` |
| `created_at` | `DATETIME(3)` | 是 | 创建时间 |
| `updated_at` | `DATETIME(3)` | 是 | 更新时间 |
| `deleted_at` | `DATETIME(3)` | 否 | 软删除时间 |

索引：

- `uk_tenant_users_tenant_user(tenant_id, user_id)`。
- `idx_tenant_users_user_id(user_id)`。
- `idx_tenant_users_tenant_id(tenant_id)`。
- `idx_tenant_users_status(status)`。
- `idx_tenant_users_deleted_at(deleted_at)`。

关系：

- 多个用户可以加入同一租户。
- 同一用户可以加入多个租户。
- 同一用户在同一租户只能有一条有效成员关系。

## 角色 Role

表示平台级或租户级角色定义。

| 字段 | 类型建议 | 必填 | 说明 |
|------|----------|------|------|
| `id` | `BIGINT UNSIGNED` | 是 | 角色主键 |
| `code` | `VARCHAR(64)` | 是 | 角色编码 |
| `name` | `VARCHAR(128)` | 是 | 角色名称 |
| `scope` | `VARCHAR(32)` | 是 | `platform` 或 `tenant` |
| `description` | `VARCHAR(512)` | 否 | 角色说明 |
| `created_at` | `DATETIME(3)` | 是 | 创建时间 |
| `updated_at` | `DATETIME(3)` | 是 | 更新时间 |
| `deleted_at` | `DATETIME(3)` | 否 | 软删除时间 |

基础数据：

| code | name | scope | 说明 |
|------|------|-------|------|
| `PLATFORM_ADMIN` | 平台管理员 | `platform` | 可管理平台租户，MVP 预留 |
| `TENANT_ADMIN` | 租户管理员 | `tenant` | 管理当前租户内资源 |
| `DO` | 数据拥有者 | `tenant` | 当前租户内上传和管理自己的文件 |
| `DU` | 数据使用者 | `tenant` | 当前租户内查看文件、下载密文、尝试解密 |

索引：

- `uk_roles_code(code)`。
- `idx_roles_scope(scope)`。
- `idx_roles_deleted_at(deleted_at)`。

## 用户角色 UserRoleAssignment

表示用户在某个租户下的角色，或平台级角色。

| 字段 | 类型建议 | 必填 | 说明 |
|------|----------|------|------|
| `id` | `BIGINT UNSIGNED` | 是 | 关系主键 |
| `tenant_id` | `BIGINT UNSIGNED` | 否 | 租户 ID；租户角色必填，平台角色可为空 |
| `user_id` | `BIGINT UNSIGNED` | 是 | 用户 ID |
| `role_id` | `BIGINT UNSIGNED` | 是 | 角色 ID |
| `created_at` | `DATETIME(3)` | 是 | 创建时间 |
| `updated_at` | `DATETIME(3)` | 是 | 更新时间 |
| `deleted_at` | `DATETIME(3)` | 否 | 软删除时间 |

索引：

- `uk_user_roles_tenant_user_role(tenant_id, user_id, role_id)`。
- `idx_user_roles_user_id(user_id)`。
- `idx_user_roles_tenant_id(tenant_id)`。
- `idx_user_roles_role_id(role_id)`。
- `idx_user_roles_deleted_at(deleted_at)`。

规则：

- `TENANT_ADMIN`、`DO`、`DU` 必须绑定 `tenant_id`。
- `PLATFORM_ADMIN` 可不绑定 `tenant_id`。
- 同一用户可以在不同租户拥有不同角色。
- 给用户分配租户角色时必须先确保用户是该租户成员。

## 当前租户上下文 CurrentTenantContext

该实体不一定落表，表示一次请求中的可信上下文。

| 字段 | 来源 | 说明 |
|------|------|------|
| `user_id` | Access Token | 当前登录用户 |
| `tenant_id` | `X-Tenant-Id` 经后端校验 | 当前租户 |
| `tenant_code` | 租户查询 | 当前租户编码 |
| `roles` | `user_roles` 查询 | 用户在当前租户下的角色 |
| `is_platform_admin` | `user_roles` 查询 | 是否拥有平台管理员角色 |

规则：

- 普通租户内业务只能使用上下文中的 `tenant_id`。
- 请求体中的 `tenant_id` 不能作为可信来源。
- Platform Admin 跨租户管理必须显式走管理接口，而不是绕过上下文校验。

## 后续租户内业务表约束

后续创建以下业务表时必须包含 `tenant_id`：

| 表 | `tenant_id` 规则 | 推荐索引 |
|----|------------------|----------|
| `attributes` | 属性定义属于租户 | `idx_attributes_tenant_id`、`uk_attributes_tenant_code(tenant_id, code)` |
| `user_attributes` | 用户属性分配属于租户 | `idx_user_attributes_tenant_user(tenant_id, user_id)` |
| `files` | 文件元数据属于租户 | `idx_files_tenant_id`、`idx_files_tenant_owner(tenant_id, owner_id)` |
| `file_policies` | 文件策略属于租户 | `idx_file_policies_tenant_file(tenant_id, file_id)` |
| `encrypt_records` | 加密记录属于租户 | `idx_encrypt_records_tenant_file(tenant_id, file_id)` |
| `decrypt_records` | 解密记录属于租户 | `idx_decrypt_records_tenant_user(tenant_id, user_id)` |
| `audit_logs` | 审计日志属于租户 | `idx_audit_logs_tenant_created(tenant_id, created_at)` |

所有详情、更新和删除操作必须使用 `id + tenant_id` 组合条件。
