# 数据模型：后端租户级 RBAC

## Role（角色）

**表名**：`roles`

**含义**：平台或租户内可授予的角色定义。系统内置角色和租户自定义角色共享同一表。

**字段**：

| 字段 | 类型语义 | 规则 |
|------|----------|------|
| `id` | 自增主键 | 内部引用 |
| `tenant_id` | 角色归属 | `0` 表示系统内置角色；`>0` 表示租户自定义角色 |
| `code` | 稳定编码 | 同一 `tenant_id` 内唯一；创建后不可修改 |
| `name` | 展示名称 | 自定义角色可修改 |
| `description` | 描述 | 可为空 |
| `scope_type` | `PLATFORM` 或 `TENANT` | 平台角色不得出现在租户成员角色接口 |
| `role_category` | `GOVERNANCE`、`BUSINESS`、`CAPABILITY` | 自定义角色只能是 `BUSINESS` |
| `is_builtin` | 是否内置 | 内置角色不可删除、不可改分类和权限 |
| `status` | `ACTIVE`、`DISABLED` | 禁用后不产生权限、不可新分配 |
| `created_by` | 创建人 | 系统 seed 可为空或 0 |
| `updated_by` | 更新人 | 系统 seed 可为空或 0 |
| `created_at`、`updated_at` | 时间 | 数据库维护 |

**约束**：

- `UNIQUE(tenant_id, code)`
- `INDEX(tenant_id, status)`
- `INDEX(scope_type, role_category, status)`

**状态转换**：

- 自定义角色：`ACTIVE -> DISABLED`
- 内置角色：本阶段不允许由租户修改状态。

## UserRoleAssignment（用户角色授权）

**表名**：`user_roles`

**含义**：用户在平台或某租户内拥有某角色的授权记录。

**字段**：

| 字段 | 类型语义 | 规则 |
|------|----------|------|
| `id` | 自增主键 | 内部引用 |
| `tenant_id` | 授权作用域 | `0` 平台；`>0` 租户 |
| `user_id` | 用户 ID | 必须存在 |
| `role_id` | 角色 ID | 必须存在 |
| `assignment_source` | 来源 | `SYSTEM`、`MANUAL`、`MIGRATION` 等 |
| `assigned_by` | 分配人 | 系统迁移可为空或 0 |
| `status` | `ACTIVE`、`REVOKED`、`EXPIRED` | 只有 ACTIVE 可生效 |
| `expires_at` | 过期时间 | 为空表示不过期 |
| `revoked_at` | 撤销时间 | 撤销时写入 |
| `created_at`、`updated_at` | 时间 | 数据库维护 |

**约束**：

- `UNIQUE(tenant_id, user_id, role_id)`
- `INDEX(tenant_id, user_id, status)`
- `INDEX(tenant_id, role_id, status)`

**有效条件**：

- `status=ACTIVE`
- `expires_at IS NULL OR expires_at > now`
- 关联角色 `status=ACTIVE`
- 租户授权还要求 `tenant_users.status=active`

**状态转换**：

- 分配：不存在则创建 `ACTIVE`；已撤销则恢复为 `ACTIVE` 并清理 `revoked_at`。
- 撤销：`ACTIVE -> REVOKED`，写入 `revoked_at`。
- 过期：查询时动态排除；可后续异步标记 `EXPIRED`。

## Permission（权限）

**表名**：`permissions`

**含义**：后端授权服务判断的稳定功能权限点。

**字段**：

| 字段 | 类型语义 | 规则 |
|------|----------|------|
| `id` | 自增主键 | 内部引用 |
| `code` | 权限编码 | 全局唯一，不使用通配符 |
| `name` | 展示名称 | 中文 |
| `description` | 描述 | 可为空 |
| `scope_type` | `PLATFORM` 或 `TENANT` | 与角色作用域匹配 |
| `resource_type` | 资源类型 | 如 `tenant_role`、`policy`、`file` |
| `action` | 动作 | 如 `read`、`manage`、`write` |
| `status` | `ACTIVE`、`DISABLED` | 禁用后不可绑定、不生效 |
| `created_at`、`updated_at` | 时间 | 数据库维护 |

**约束**：

- `UNIQUE(code)`
- `INDEX(scope_type, status)`

## RolePermission（角色权限绑定）

**表名**：`role_permissions`

**含义**：角色拥有某个权限的授权关系。

**字段**：

| 字段 | 类型语义 | 规则 |
|------|----------|------|
| `id` | 自增主键 | 内部引用 |
| `role_id` | 角色 ID | 必须存在 |
| `permission_id` | 权限 ID | 必须存在 |
| `granted_by` | 授权人 | 内置 seed 可为空或 0 |
| `created_at` | 创建时间 | 数据库维护 |

**约束**：

- `UNIQUE(role_id, permission_id)`
- `INDEX(role_id)`
- `INDEX(permission_id)`

## AuthorizationContext（授权上下文）

**含义**：当前用户或指定成员在某作用域下的有效角色和权限集合。

**字段**：

| 字段 | 含义 |
|------|------|
| `tenantId` | 当前租户 ID；平台上下文为空或 0 |
| `roles` | 有效角色摘要，含 `id/code/name/category/builtin` |
| `permissions` | 去重后的有效权限 code |

## 关系和隔离规则

- 系统内置租户角色：`roles.tenant_id=0 AND scope_type=TENANT`，所有租户可分配。
- 租户自定义角色：`roles.tenant_id=当前租户 AND scope_type=TENANT AND role_category=BUSINESS`。
- 平台角色：`roles.tenant_id=0 AND scope_type=PLATFORM`，只能在平台作用域授权。
- 租户成员角色分配不得出现平台角色。
- 自定义角色权限不得绑定平台权限。
