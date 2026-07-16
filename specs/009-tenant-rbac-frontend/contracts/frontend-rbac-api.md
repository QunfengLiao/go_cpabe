# 前端 RBAC API 契约

本文记录桌面端前端将调用的当前租户 RBAC API。所有接口都通过现有 `request()` 自动携带 `Authorization`，并在租户上下文和授权状态就绪后携带 `X-Tenant-Id`。

## 通用约定

- 基础路径：`/api/v1`。
- 前端 client 文件：`desktop/src/renderer/src/api/rbac.ts`。
- 成功响应由项目 Envelope 包装，前端 `request<T>()` 返回 `data`。
- 错误响应转为 `ApiError`，前端优先展示 `message`，并对常见 `code` 做明确中文提示。

## GET /tenant/me/authorization

**用途**：加载当前用户在当前租户的真实授权上下文。

**前端方法**：`getCurrentAuthorization(): Promise<AuthorizationContextDTO>`。

**响应 data**：

```json
{
  "tenantId": 1,
  "roles": [
    {
      "id": 2,
      "tenantId": 0,
      "code": "DO",
      "name": "数据拥有者",
      "scopeType": "TENANT",
      "roleCategory": "CAPABILITY",
      "category": "CAPABILITY",
      "builtin": true,
      "isBuiltin": true,
      "status": "ACTIVE"
    }
  ],
  "permissions": ["tenant.dashboard.read", "policy.read"]
}
```

## GET /tenant/permissions

**用途**：加载租户自定义角色可绑定的租户权限目录。

**前端方法**：`listTenantPermissions(): Promise<PermissionDTO[]>`。

**响应 data**：

```json
{
  "items": [
    {
      "id": 1,
      "code": "tenant.role.read",
      "name": "查看角色",
      "description": "查看当前租户角色和权限",
      "scopeType": "TENANT",
      "resourceType": "tenant_role",
      "action": "read",
      "status": "ACTIVE"
    }
  ]
}
```

## GET /tenant/roles

**用途**：加载当前租户可见角色，包括系统内置租户角色和当前租户自定义角色。

**前端方法**：`listTenantRoles(): Promise<TenantRoleDTO[]>`。

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
      "category": "CAPABILITY",
      "builtin": true,
      "isBuiltin": true,
      "status": "ACTIVE",
      "permissionCount": 7,
      "activeMemberCount": 3
    }
  ]
}
```

## POST /tenant/roles

**用途**：创建租户自定义业务角色，可同时绑定权限。

**前端方法**：`createTenantRole(input): Promise<TenantRoleDTO>`。

**请求**：

```json
{
  "code": "SRE_ENGINEER",
  "name": "SRE 工程师",
  "description": "负责稳定性相关操作",
  "permissionCodes": ["tenant.org.read", "policy.read"]
}
```

**错误处理**：

- `ROLE_CODE_EXISTS`：显示“角色编码已存在，请更换 code”。
- `INVALID_PERMISSION_SCOPE`：显示“权限作用域非法，不能绑定平台权限”。
- `PERMISSION_DENIED`：显示“权限不足，无法创建角色”。

## GET /tenant/roles/:roleId

**用途**：读取角色详情。

**前端方法**：`getTenantRole(roleId): Promise<TenantRoleDTO>`。

**说明**：如果页面需要权限明细，必须同时调用 `getTenantRolePermissions(roleId)`。

## PATCH /tenant/roles/:roleId

**用途**：修改自定义角色名称和描述。

**前端方法**：`updateTenantRole(roleId, input): Promise<TenantRoleDTO>`。

**请求**：

```json
{
  "name": "高级 SRE 工程师",
  "description": "负责稳定性和应急响应"
}
```

## DELETE /tenant/roles/:roleId

**用途**：禁用自定义角色。

**前端方法**：`disableTenantRole(roleId): Promise<{ roleId: number; status: "DISABLED"; affectedMemberCount: number }>`。

**错误处理**：

- `BUILTIN_ROLE_IMMUTABLE`：显示“系统内置角色不可禁用”。
- `CANNOT_REMOVE_LAST_TENANT_ADMIN`：显示“不能移除最后一名租户管理员”。

## GET /tenant/roles/:roleId/permissions

**用途**：读取角色权限集合。

**前端方法**：`getTenantRolePermissions(roleId): Promise<RolePermissionDTO>`。

## PUT /tenant/roles/:roleId/permissions

**用途**：全量替换自定义角色权限。

**前端方法**：`replaceTenantRolePermissions(roleId, permissionCodes): Promise<RolePermissionDTO>`。

**请求**：

```json
{
  "permissionCodes": ["tenant.member.read", "policy.read"]
}
```

## GET /tenant/members/:userId/roles

**用途**：读取成员当前租户角色和权限并集。

**前端方法**：`getTenantMemberRoles(userId): Promise<MemberRoleDTO>`。

## PUT /tenant/members/:userId/roles

**用途**：全量替换成员角色集合。

**前端方法**：`replaceTenantMemberRoles(userId, roleCodes): Promise<MemberRoleDTO>`。

**请求**：

```json
{
  "roleCodes": ["DO", "DU", "SRE_ENGINEER"]
}
```

**错误处理**：

- `CANNOT_ASSIGN_PLATFORM_ROLE`：显示“不能分配平台角色”。
- `ROLE_DISABLED`：显示“角色已禁用，不能分配”。
- `CANNOT_REMOVE_LAST_TENANT_ADMIN`：显示“不能移除最后一名租户管理员”。
- `MEMBER_NOT_FOUND_IN_TENANT`：显示“成员不属于当前租户或已不可用”。

## 前端错误映射要求

| 后端 code | 前端提示 |
|-----------|----------|
| `PERMISSION_DENIED` | 权限不足，当前账号无法执行该操作 |
| `ROLE_CODE_EXISTS` | 角色编码已存在 |
| `BUILTIN_ROLE_IMMUTABLE` | 系统内置角色由系统管理，不允许修改 |
| `ROLE_DISABLED` | 角色已禁用 |
| `INVALID_PERMISSION_SCOPE` | 权限作用域非法 |
| `CANNOT_ASSIGN_PLATFORM_ROLE` | 不能在租户内分配平台角色 |
| `CANNOT_REMOVE_LAST_TENANT_ADMIN` | 不能移除最后一名租户管理员 |
| `MEMBER_NOT_FOUND_IN_TENANT` | 成员不属于当前租户或已不可用 |

未知错误使用后端 `message`，没有 `message` 时再显示“请求失败，请稍后重试”。
