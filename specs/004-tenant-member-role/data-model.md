# 数据模型：租户成员角色分配

## 租户成员 TenantMember

**含义**：用户与租户之间的有效成员关系，是普通业务角色分配的前置条件。

**关键字段**：

- `tenant_id`：成员所属租户。角色分配时必须等于路径中的目标租户。
- `user_id`：目标成员用户。
- `status`：成员状态。只有 `active` 成员可以被分配普通业务角色。

**关系**：

- 一个租户有多个成员。
- 一个成员可以在不同租户拥有不同角色，但本功能只操作路径指定租户。

**校验规则**：

- 目标成员必须存在于 `tenant_users`。
- 目标成员状态必须为 `active`。
- 操作者必须是同一租户的 `TENANT_ADMIN`。

## 角色 Role

**含义**：系统内置的平台级或租户级角色定义。

**本功能相关角色**：

- `TENANT_ADMIN`：租户管理员，本接口只用于校验操作者，不允许作为普通分配目标。
- `DO`：内部稳定编码，对外业务名称为数据拥有者或 `DATA_OWNER`。
- `DU`：内部稳定编码，对外业务名称为数据访问者或 `DATA_VISITOR`。
- `PLATFORM_ADMIN`：平台管理员，不属于本功能分配范围。

**校验规则**：

- 普通角色分配只允许 `DO`、`DU`。
- `PLATFORM_ADMIN` 的 `tenant_id` 必须为空，不得由本接口写入。
- `TENANT_ADMIN` 只能由平台兜底能力指定，不得由本接口写入。

## 用户角色关系 UserRoleAssignment

**含义**：记录用户在平台级或某个租户下获得的角色。

**关键字段**：

- `tenant_id`：普通业务角色必须绑定当前租户，不能为 `NULL`。
- `user_id`：被分配角色的成员用户。
- `role_id`：指向 `roles` 中的 `DO` 或 `DU`。

**关系**：

- 一个用户在一个租户下，本期只允许一个普通业务角色。
- 同一用户可以在不同租户下拥有不同普通业务角色。

**状态转换**：

```text
无普通业务角色 -> DATA_OWNER
无普通业务角色 -> DATA_VISITOR
DATA_OWNER -> DATA_VISITOR
DATA_VISITOR -> DATA_OWNER
DATA_OWNER -> DATA_OWNER（幂等）
DATA_VISITOR -> DATA_VISITOR（幂等）
```

**事务规则**：

- 删除旧普通业务角色和写入新普通业务角色必须在同一个事务中完成。
- 失败时回滚到操作前状态。

## 角色分配请求 TenantMemberRoleAssignment

**含义**：租户管理员对某个租户成员执行的一次普通业务角色保存动作。

**字段**：

- `tenantId`：来自路径，表示被操作的租户。
- `userId`：来自路径，表示被操作的成员。
- `actorUserId`：来自登录态，表示当前操作者。
- `roleCode`：来自请求体，允许 `DATA_OWNER` 或 `DATA_VISITOR`。

**校验规则**：

- `actorUserId` 必须是 `tenantId` 下的 `TENANT_ADMIN`。
- `actorUserId` 不能通过 `PLATFORM_ADMIN` 身份绕过本校验。
- `userId` 必须是 `tenantId` 下的 active 成员。
- `actorUserId == userId` 且操作者拥有 `TENANT_ADMIN` 时，不允许通过本接口修改自己的租户管理员角色。
- `roleCode` 必须映射到 `DO` 或 `DU`。

## 成员列表回显 TenantMemberDTO

**含义**：前端成员列表展示的数据结构。

**关键字段**：

- `user_id`：成员用户 ID。
- `email`：成员邮箱。
- `nickname`：成员昵称。
- `member_status`：成员状态。
- `roles`：成员在当前租户下的角色列表。

**回显规则**：

- 如果 `roles` 包含 `TENANT_ADMIN`，展示租户管理员。
- 如果 `roles` 包含 `DO`，展示数据拥有者。
- 如果 `roles` 包含 `DU`，展示数据访问者。
- 如果无普通业务角色，展示未分配或空状态。
