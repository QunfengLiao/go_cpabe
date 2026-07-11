# 数据模型：租户 RBAC 前端授权与角色管理

本文描述前端状态模型和 DTO。后端数据库模型以 `specs/008-tenant-rbac-backend` 为准，本阶段不新增前端持久化数据库。

## AuthorizationState

表示当前前端用于菜单、路由和按钮判断的授权状态。

| 字段 | 类型 | 说明 |
|------|------|------|
| `authorizationStatus` | `idle | loading | ready | error` | 授权生命周期状态。 |
| `authorizationUserId` | `string` | 当前 permissions 所属用户；必须等于 `currentUserId` 才可用于判断。 |
| `authorizationTenantId` | `string` | 当前 permissions 所属租户；必须等于 `currentTenantId` 才可用于租户功能判断。 |
| `authorizationGeneration` | `number` | 与账号/租户切换 generation 对齐，防止旧请求回写。 |
| `authorizationError` | `string` | 授权加载失败时展示给用户的错误信息。 |
| `permissions` | `string[]` | 服务端返回的 permission code 集合。 |
| `tenantRoles` | `TenantRoleDTO[]` 或兼容 role code 摘要 | 用于展示当前角色和成员角色，不作为功能授权来源。 |

### 状态转换

- `idle -> loading`：登录恢复、账号切换、租户切换或手动刷新授权。
- `loading -> ready`：`/tenant/me/authorization` 成功且 userId、tenantId、generation 匹配。
- `loading -> error`：授权接口失败、网络失败、租户成员不可用。
- `ready -> loading`：切换账号、切换租户、修改自身角色或角色权限。
- `ready/error -> idle`：退出登录或清空会话。

## PermissionDTO

来自 `GET /tenant/permissions`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `number` | 权限主键。 |
| `code` | `string` | 稳定权限编码，例如 `tenant.role.read`。 |
| `name` | `string` | 中文展示名。 |
| `description` | `string` | 权限说明。 |
| `scopeType` | `PLATFORM | TENANT` | 权限作用域；前端权限选择器只显示 `TENANT`。 |
| `resourceType` | `string` | 所属资源模块。 |
| `action` | `string` | 动作类型。 |
| `status` | `ACTIVE | DISABLED` | 权限状态。 |

## TenantRoleDTO

来自角色列表、详情、成员角色和授权上下文。

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `number` | 角色主键。 |
| `tenantId` | `number` | `0` 表示系统内置租户角色，大于 0 表示租户自定义角色。 |
| `code` | `string` | 稳定角色编码，只用于展示、分组和提交角色集合。 |
| `name` | `string` | 角色名称；修改名称不得影响权限判断。 |
| `description` | `string` | 角色说明。 |
| `scopeType` | `TENANT | PLATFORM` | 租户角色管理只接受 `TENANT`。 |
| `roleCategory` / `category` | `GOVERNANCE | BUSINESS | CAPABILITY` | 展示分组。 |
| `builtin` / `isBuiltin` | `boolean` | 是否系统内置；内置角色只读。 |
| `status` | `ACTIVE | DISABLED` | 角色状态。 |
| `permissionCount` | `number` | 权限数量，用于列表展示。 |
| `activeMemberCount` | `number` | 有效成员数量，用于列表展示和禁用影响提示。 |

## RolePermissionDTO

来自 `GET/PUT /tenant/roles/:roleId/permissions`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `roleId` | `number` | 目标角色 ID。 |
| `permissionCodes` | `string[]` | 当前绑定权限 code。 |
| `permissions` | `PermissionDTO[]` | 权限详情，用于详情展示。 |

## MemberRoleDTO

来自 `GET/PUT /tenant/members/:userId/roles`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `tenantId` | `number` | 当前租户 ID。 |
| `userId` | `number` | 成员用户 ID。 |
| `roles` | `TenantRoleDTO[]` | 成员当前完整角色集合。 |
| `permissions` | `string[]` | 成员由角色并集获得的权限 code。 |

## AuthorizationContextDTO

来自 `GET /tenant/me/authorization`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `tenantId` | `number` | 当前可信租户 ID。 |
| `roles` | `TenantRoleDTO[]` | 当前用户在该租户内的有效角色。 |
| `permissions` | `string[]` | 当前用户有效权限 code 并集。 |

## MenuItemPermissionMeta

前端菜单过滤使用的本地模型。

| 字段 | 类型 | 说明 |
|------|------|------|
| `key` | `string` | 菜单 key 或 path。 |
| `path` | `string` | 点击跳转路径。 |
| `requiredPermission` | `string` | 需要单个权限。 |
| `requiredAnyPermissions` | `string[]` | 任一权限命中即可显示。 |
| `requiredAllPermissions` | `string[]` | 全部权限命中才显示。 |
| `scope` | `platform | tenant | public` | 菜单权限作用域。 |
| `children` | `MenuItemPermissionMeta[]` | 子菜单。 |

## 本地持久化规则

- `permissions` 可以继续存入 `go_cpabe_tenant_contexts`，但只作为最近一次服务端结果快照。
- 快照必须随 userId 和 tenantId 存储，不得使用全局 permissions key。
- access token、currentUserId、currentTenantId 或 generation 变化时，快照立即失效。
- route/menu/button 判断不得仅凭本地快照进入 ready。

## 校验规则

- permission code 必须是非空字符串。
- 租户权限选择器必须过滤 `scopeType !== TENANT` 的权限。
- 内置角色 `builtin=true` 时，编辑、禁用、权限保存入口必须不可用。
- 成员角色弹窗不得显示 `scopeType=PLATFORM` 或 `code=PLATFORM_ADMIN` 的角色。
- 禁用角色不可新选；如果成员已拥有禁用角色，必须以禁用状态展示并禁止再次添加。
