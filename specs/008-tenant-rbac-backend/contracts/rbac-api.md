# API 契约：后端租户级 RBAC

所有 `/api/v1/tenant/...` 接口都必须经过：

1. `AuthRequired` 解析登录用户。
2. `TenantRequired` 从 `X-Tenant-Id` 解析并验证可信租户上下文。
3. `PermissionRequired` 调用 AuthorizationService 判断权限。

请求体、query 和 path 中的任何 `tenant_id` 都不能作为实际租户边界。

## 通用响应

成功响应沿用项目 Envelope：

```json
{
  "code": "OK",
  "message": "success",
  "data": {},
  "request_id": ""
}
```

错误响应沿用 `response.AppError`。

## GET /api/v1/tenant/permissions

**权限**：`tenant.role.read` 或 `tenant.role.manage`

**响应 data**：

```json
{
  "items": [
    {
      "id": 1,
      "code": "policy.read",
      "name": "查看策略",
      "description": "查看当前租户访问策略",
      "scopeType": "TENANT",
      "resourceType": "policy",
      "action": "read",
      "status": "ACTIVE"
    }
  ]
}
```

**错误码**：`PERMISSION_DENIED`

**事务边界**：只读。

## GET /api/v1/tenant/roles

**权限**：`tenant.role.read`

**响应 data**：

```json
{
  "items": [
    {
      "id": 2,
      "tenantId": 0,
      "code": "DO",
      "name": "数据拥有者",
      "description": "当前租户内上传和管理文件",
      "scopeType": "TENANT",
      "roleCategory": "CAPABILITY",
      "builtin": true,
      "status": "ACTIVE",
      "permissionCount": 7,
      "activeMemberCount": 3
    }
  ]
}
```

**错误码**：`PERMISSION_DENIED`

**事务边界**：只读。

## POST /api/v1/tenant/roles

**权限**：`tenant.role.manage`

**请求**：

```json
{
  "code": "SRE_ENGINEER",
  "name": "SRE 工程师",
  "description": "负责稳定性相关操作",
  "permissionCodes": ["tenant.org.read", "policy.read"]
}
```

**响应 data**：`TenantRoleDetail`

**规则**：

- 自动使用当前租户 ID。
- 自动写入 `TENANT + BUSINESS + builtin=false + ACTIVE`。
- 客户端不能指定 `tenantId`、`scopeType`、`roleCategory`、`builtin`。
- 角色创建和可选权限绑定必须在一个事务内完成。

**错误码**：`ROLE_CODE_EXISTS`、`INVALID_PERMISSION_SCOPE`、`PERMISSION_DENIED`

## GET /api/v1/tenant/roles/:roleId

**权限**：`tenant.role.read`

**规则**：角色必须是系统内置租户角色或当前租户自定义角色。

**响应 data**：`TenantRoleDetail`

**错误码**：`ROLE_NOT_FOUND`、`CROSS_TENANT_ACCESS_DENIED`、`PERMISSION_DENIED`

## PATCH /api/v1/tenant/roles/:roleId

**权限**：`tenant.role.manage`

**请求**：

```json
{
  "name": "高级 SRE 工程师",
  "description": "负责稳定性和应急响应"
}
```

**规则**：

- 只允许修改当前租户自定义角色。
- 不允许修改 code、scopeType、roleCategory、builtin。

**错误码**：`ROLE_NOT_FOUND`、`BUILTIN_ROLE_IMMUTABLE`、`PERMISSION_DENIED`

## DELETE /api/v1/tenant/roles/:roleId

**权限**：`tenant.role.manage`

**响应 data**：

```json
{
  "roleId": 15,
  "status": "DISABLED",
  "affectedMemberCount": 4
}
```

**规则**：

- 逻辑禁用，不物理删除。
- 系统内置角色不可禁用。
- 禁用后不再产生权限，不能新分配。

**错误码**：`ROLE_NOT_FOUND`、`BUILTIN_ROLE_IMMUTABLE`、`CANNOT_REMOVE_LAST_TENANT_ADMIN`、`PERMISSION_DENIED`

**事务边界**：禁用和最后管理员保护同事务。

## GET /api/v1/tenant/roles/:roleId/permissions

**权限**：`tenant.role.read`

**响应 data**：

```json
{
  "roleId": 15,
  "permissionCodes": ["tenant.org.read", "policy.read"],
  "permissions": []
}
```

**错误码**：`ROLE_NOT_FOUND`、`PERMISSION_DENIED`

## PUT /api/v1/tenant/roles/:roleId/permissions

**权限**：`tenant.role.manage`

**请求**：

```json
{
  "permissionCodes": ["tenant.member.read", "policy.read", "file.upload"]
}
```

**规则**：

- 全量替换。
- 只能修改当前租户自定义 `BUSINESS` 角色。
- 不允许绑定平台权限。
- 空数组表示清空权限。

**错误码**：`ROLE_NOT_FOUND`、`BUILTIN_ROLE_IMMUTABLE`、`INVALID_PERMISSION_SCOPE`、`PERMISSION_DENIED`

**事务边界**：删除旧绑定和写入新绑定同事务。

## GET /api/v1/tenant/members/:userId/roles

**权限**：`tenant.member.read`

**响应 data**：

```json
{
  "tenantId": 1,
  "userId": 8,
  "roles": [],
  "permissions": []
}
```

**错误码**：`MEMBER_NOT_FOUND_IN_TENANT`、`PERMISSION_DENIED`

## PUT /api/v1/tenant/members/:userId/roles

**权限**：`tenant.member.manage`

**请求**：

```json
{
  "roleCodes": ["DO", "DU", "SRE_ENGINEER"]
}
```

**规则**：

- 完整集合替换。
- 允许 `DO + DU`。
- 拒绝 `PLATFORM_ADMIN`。
- 只能分配系统内置租户角色或当前租户自定义角色。
- 禁用角色不可分配。
- 不能移除最后一名有效 `TENANT_ADMIN`。

**响应 data**：成员最新角色和权限集合。

**错误码**：`MEMBER_NOT_FOUND_IN_TENANT`、`CANNOT_ASSIGN_PLATFORM_ROLE`、`ROLE_DISABLED`、`CANNOT_REMOVE_LAST_TENANT_ADMIN`、`PERMISSION_DENIED`

**事务边界**：成员校验、角色校验、撤销旧角色、激活/创建新角色、最后管理员保护同事务。

## GET /api/v1/tenant/me/authorization

**权限**：有效登录和有效租户成员。

**响应 data**：

```json
{
  "tenantId": 1,
  "roles": [
    {
      "id": 2,
      "code": "DO",
      "name": "数据拥有者",
      "category": "CAPABILITY",
      "builtin": true
    }
  ],
  "permissions": ["tenant.dashboard.read", "policy.read", "file.upload"]
}
```

**错误码**：`TENANT_MEMBER_FORBIDDEN`、`TENANT_MEMBER_DISABLED`

**事务边界**：只读。
