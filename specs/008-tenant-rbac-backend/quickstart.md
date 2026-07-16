# 快速验证：后端租户级 RBAC

## 前置条件

- 已配置后端 MySQL 与 Redis 环境变量。
- 当前分支已完成本功能实现和迁移。
- 本阶段不需要启动或修改前端页面。

## 初始化

```bash
cd backend
go run ./cmd/migrate
go run ./cmd/seed
```

期望结果：

- `roles` 包含四个内置角色，且分类和作用域正确。
- `permissions` 包含平台和租户第一批权限。
- `role_permissions` 包含内置角色权限矩阵。
- 重复执行上述命令不产生重复数据。

## 自动化测试

```bash
cd backend
go test ./...
```

期望覆盖：

- 平台管理员没有租户业务权限。
- 租户管理员可创建、修改、禁用自定义角色。
- 普通成员不能管理角色。
- 同一成员可同时拥有 `DO` 和 `DU`。
- 撤销、过期、禁用角色不产生权限。
- 权限中间件阻止未授权策略和组织写操作。
- 迁移后现有租户管理员仍可管理租户。

## API 验证流程

以下流程可使用任意 HTTP 客户端执行。所有 `/api/v1/tenant/...` 请求都需要携带：

```text
Authorization: Bearer <access_token>
X-Tenant-Id: <tenant_id>
Content-Type: application/json
```

### 1. 查询当前用户授权上下文

```text
GET /api/v1/tenant/me/authorization
```

期望：返回当前租户真实角色和权限集合，权限来自数据库绑定。

### 2. 创建自定义角色

```text
POST /api/v1/tenant/roles
```

请求：

```json
{
  "code": "SRE_ENGINEER",
  "name": "SRE 工程师",
  "description": "负责稳定性相关操作",
  "permissionCodes": ["tenant.org.read", "policy.read"]
}
```

期望：角色为 `TENANT + BUSINESS + builtin=false`。

### 3. 替换角色权限

```text
PUT /api/v1/tenant/roles/{roleId}/permissions
```

请求：

```json
{
  "permissionCodes": ["tenant.org.read", "policy.read", "file.upload"]
}
```

期望：权限全量替换，平台权限会被拒绝。

### 4. 给成员分配多个角色

```text
PUT /api/v1/tenant/members/{userId}/roles
```

请求：

```json
{
  "roleCodes": ["DO", "DU", "SRE_ENGINEER"]
}
```

期望：成员可同时拥有 `DO`、`DU` 和自定义角色；返回权限并集。

### 5. 验证权限中间件

- 没有 `policy.write` 的用户调用策略写接口应返回 403。
- 获得含 `policy.write` 的自定义角色后，同一用户可通过权限中间件，但仍受策略 owner 和租户边界校验。

## 数据库验证 SQL

检查重复角色：

```sql
SELECT tenant_id, code, COUNT(*)
FROM roles
GROUP BY tenant_id, code
HAVING COUNT(*) > 1;
```

检查孤立用户角色：

```sql
SELECT ur.*
FROM user_roles ur
LEFT JOIN roles r ON r.id = ur.role_id
WHERE r.id IS NULL;
```

检查启用租户是否至少一名管理员：

```sql
SELECT t.id, t.code
FROM tenants t
LEFT JOIN user_roles ur ON ur.tenant_id = t.id AND ur.status = 'ACTIVE'
LEFT JOIN roles r ON r.id = ur.role_id AND r.code = 'TENANT_ADMIN' AND r.status = 'ACTIVE'
LEFT JOIN tenant_users tu ON tu.tenant_id = t.id AND tu.user_id = ur.user_id AND tu.status = 'active'
WHERE t.status = 'enabled'
GROUP BY t.id, t.code
HAVING COUNT(DISTINCT tu.user_id) = 0;
```

以上查询均应返回空结果。
