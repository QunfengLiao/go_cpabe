# 数据模型：Platform Admin 平台管理员租户管理能力

## 现有用户 User

`users` 表已经存在，作为账号主体。该表保留旧 `role` 字段用于兼容注册、登录资料展示和历史初始化，但不再作为平台或租户内授权的可信依据。

| 字段 | 类型建议 | 必填 | 说明 |
|------|----------|------|------|
| `id` | `BIGINT UNSIGNED` | 是 | 用户主键 |
| `email` | `VARCHAR(255)` | 是 | 登录邮箱，全局唯一 |
| `password_hash` | `VARCHAR(255)` | 是 | 密码哈希，不对外返回 |
| `nickname` | `VARCHAR(64)` | 是 | 昵称 |
| `role` | `VARCHAR(32)` | 是 | 旧全局角色，兼容字段 |
| `status` | `VARCHAR(32)` | 是 | `active` 或 `disabled` |

规则：

- 公开注册只能创建 `data_owner` 或 `data_user`。
- 公开注册不能创建 `admin`、`TENANT_ADMIN` 或 `PLATFORM_ADMIN`。
- 平台授权必须查询 `user_roles`，不能依赖 `users.role`。

## 租户 Tenant

租户是平台下的组织边界。

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
- `code` 不能为空。
- `code` 全局唯一。
- `code` 建议正则：`^[a-z0-9]+(?:-[a-z0-9]+)*$`。
- 禁用租户不删除历史数据。

初始演示租户：

| 名称 | code | 状态 |
|------|------|------|
| 四川师范大学 | `scnu` | `enabled` |
| 深信服科技 | `sangfor` | `enabled` |
| 香港友邦保险 | `aia-hk` | `enabled` |
| 默认租户 | `default-tenant` | `enabled` |

## 租户用户关系 TenantUser

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

规则：

- 同一用户在同一租户只能有一条有效关系。
- 用户移出租户后，成员状态变为 `disabled` 或软删除，但不得继续进入该租户上下文。
- 禁用租户默认不允许新增普通成员。

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

基础角色：

| code | name | scope | 说明 |
|------|------|-------|------|
| `PLATFORM_ADMIN` | 平台管理员 | `platform` | 管理平台租户、租户用户关系和租户管理员分配 |
| `TENANT_ADMIN` | 租户管理员 | `tenant` | 管理当前租户内部用户、角色、属性、审计等资源 |
| `DO` | 数据拥有者 | `tenant` | 当前租户内上传文件、创建策略和管理自有文件的业务身份 |
| `DU` | 数据使用者 | `tenant` | 当前租户内查看属性、下载密文和尝试解密的业务身份 |

索引：

- `uk_roles_code(code)`。
- `idx_roles_scope(scope)`。

## 用户角色关系 UserRoleAssignment

表示用户在平台或租户下拥有的角色。

| 字段 | 类型建议 | 必填 | 说明 |
|------|----------|------|------|
| `id` | `BIGINT UNSIGNED` | 是 | 关系主键 |
| `tenant_id` | `BIGINT UNSIGNED` | 否 | 租户 ID；平台角色为空，租户角色必填 |
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

规则：

- `PLATFORM_ADMIN` 必须使用 `tenant_id = NULL`。
- `TENANT_ADMIN`、`DO`、`DU` 必须使用具体 `tenant_id`。
- 分配租户级角色前必须确认用户属于该租户。
- 同一用户在同一租户下不能重复拥有同一租户级角色。
- MySQL 对唯一索引中的 `NULL` 不按相等处理，因此平台级角色需要仓储层幂等保护；后续可用生成列或独立索引加强。

## 平台控制台摘要 PlatformDashboardSummary

不一定落表，可由查询聚合生成。

| 字段 | 类型 | 说明 |
|------|------|------|
| `tenant_count` | number | 租户总数 |
| `enabled_tenant_count` | number | 启用租户数 |
| `disabled_tenant_count` | number | 禁用租户数 |
| `user_count` | number | 平台用户总数 |
| `tenant_user_count` | number | 租户用户关系数量 |
| `tenant_admin_count` | number | Tenant Admin 数量 |
| `audit_enabled` | boolean | 平台审计是否已实现 |

## 平台操作日志 PlatformAuditLog

本阶段预留，不强制落表。后续 Audit 模块可统一设计。

候选字段：

| 字段 | 说明 |
|------|------|
| `id` | 日志主键 |
| `actor_user_id` | 操作人 |
| `action` | 操作类型 |
| `target_type` | 目标类型，例如 tenant、tenant_user、tenant_admin |
| `target_id` | 目标 ID |
| `metadata` | 操作摘要 |
| `created_at` | 操作时间 |

候选事件：

- `tenant.created`
- `tenant.enabled`
- `tenant.disabled`
- `tenant_user.added`
- `tenant_user.removed`
- `tenant_admin.assigned`
- `tenant_admin.removed`

## 状态转换

### Tenant.status

```text
enabled -> disabled
disabled -> enabled
```

规则：

- 禁用后普通租户用户不能进入工作台。
- 禁用后 Platform Admin 仍可查看租户信息。
- 禁用不删除历史数据。

### TenantUser.status

```text
active -> disabled
disabled -> active
```

规则：

- `active` 才能进入租户上下文。
- `disabled` 不能进入租户上下文。
- 重新加入租户可把 disabled 关系恢复为 active。

## 敏感信息边界

平台管理相关 DTO 不得返回：

- `password_hash`
- 主密钥明文
- 用户私钥明文
- 密码明文
- 文件明文内容
- 任何可绕过 CP-ABE 访问策略的敏感秘密
