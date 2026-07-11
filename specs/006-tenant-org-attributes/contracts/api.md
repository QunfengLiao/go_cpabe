# 接口契约：租户级组织架构与 CP-ABE 属性体系

## 通用约定

- 所有接口前缀为 `/api/v1`。
- 除登录注册外，接口必须携带有效登录态。
- 路径中的 `:id` 表示租户 ID，必须与当前租户上下文一致。
- 响应沿用当前项目统一格式；本文只描述 `data` 内的关键结构。
- 平台管理员不维护具体部门；租户管理员维护本租户组织和属性；DATA_OWNER 只能读取本租户构建器属性；DATA_VISITOR 只能查看自己的有效属性。

## 查询当前租户组织树

`GET /api/v1/tenants/:id/org-units/tree`

**允许角色**：`TENANT_ADMIN`、`DATA_OWNER`、`DATA_VISITOR`

**查询参数**：

- `status`：可选，`enabled` 或 `all`，默认 `enabled`。

**响应**：

```json
{
  "items": [
    {
      "id": 1,
      "tenantId": 2,
      "code": "AI_BG",
      "name": "AI BG",
      "path": "/AI_BG",
      "level": 1,
      "sortOrder": 30,
      "status": "enabled",
      "children": [
        {
          "id": 2,
          "tenantId": 2,
          "code": "AI_PLATFORM",
          "name": "AI 平台部",
          "path": "/AI_BG/AI_PLATFORM",
          "level": 2,
          "sortOrder": 10,
          "status": "enabled",
          "children": []
        }
      ]
    }
  ],
  "total": 1
}
```

## 查询当前租户属性字典

`GET /api/v1/tenants/:id/access-policy/attributes`

**用途**：访问树构建器加载真实租户属性字典。该接口替代构建器中的静态假数据来源。

**允许角色**：`DATA_OWNER`、`TENANT_ADMIN`

**响应**：

```json
{
  "items": [
    {
      "id": 11,
      "tenantId": 2,
      "attrCode": "department",
      "attrName": "部门",
      "attrType": "tree",
      "valueSource": "org_tree",
      "operators": ["belongs_to", "="],
      "status": "enabled",
      "description": "当前租户组织树部门属性",
      "tree": [
        {
          "id": 1,
          "valueId": 101,
          "valueCode": "AI_BG",
          "label": "AI BG",
          "path": "/AI_BG",
          "children": []
        }
      ],
      "values": []
    },
    {
      "id": 12,
      "tenantId": 2,
      "attrCode": "org_role",
      "attrName": "部门角色",
      "attrType": "enum",
      "operators": ["=", "!="],
      "values": [
        { "valueId": 201, "valueCode": "ORG_MANAGER", "label": "部门主管" },
        { "valueId": 202, "valueCode": "ORG_MEMBER", "label": "部门成员" }
      ]
    },
    {
      "id": 13,
      "tenantId": 2,
      "attrCode": "security_level",
      "attrName": "安全等级",
      "attrType": "number",
      "operators": [">=", "<=", "="],
      "values": []
    }
  ],
  "total": 3
}
```

## 查询部门成员

`GET /api/v1/tenants/:id/org-units/:orgUnitId/members`

**允许角色**：`TENANT_ADMIN`

**响应**：

```json
{
  "items": [
    {
      "userId": 31,
      "email": "ai.manager@example.com",
      "nickname": "AI 部门主管",
      "orgUnitId": 1,
      "orgUnitName": "AI BG",
      "memberStatus": "active",
      "orgRoles": ["ORG_MANAGER", "DATA_OWNER"]
    }
  ],
  "total": 1
}
```

## 添加用户到部门

`POST /api/v1/tenants/:id/org-units/:orgUnitId/members`

**允许角色**：`TENANT_ADMIN`

**请求体**：

```json
{
  "userId": 31
}
```

**响应**：

```json
{
  "member": {
    "id": 88,
    "tenantId": 2,
    "orgUnitId": 1,
    "userId": 31,
    "status": "active"
  }
}
```

**错误边界**：

- 用户不属于当前租户：返回权限或业务错误。
- 部门不属于当前租户：返回租户边界错误。
- 重复添加：返回已有有效成员关系，保持幂等。

## 设置用户部门角色

`PUT /api/v1/tenants/:id/org-units/:orgUnitId/members/:userId/roles`

**允许角色**：`TENANT_ADMIN`

**请求体**：

```json
{
  "roleCodes": ["ORG_MANAGER", "DATA_OWNER"]
}
```

**响应**：

```json
{
  "userId": 31,
  "orgUnitId": 1,
  "roleCodes": ["ORG_MANAGER", "DATA_OWNER"],
  "synced": false
}
```

**规则**：

- `roleCodes` 只能包含 `ORG_MANAGER`、`ORG_MEMBER`、`DATA_OWNER`、`DATA_VISITOR`。
- 不允许 `AI_BG_MANAGER` 等部门专属角色。
- 保存后可以同步用户属性；若未立即同步，响应必须提示需要同步。

## 同步用户 CP-ABE 属性

`POST /api/v1/tenants/:id/users/:userId/attributes/sync`

**允许角色**：`TENANT_ADMIN`

**响应**：

```json
{
  "userId": 31,
  "tenantId": 2,
  "syncedAt": "2026-07-09T15:30:00+08:00",
  "items": [
    {
      "attrCode": "department",
      "attrName": "部门",
      "valueId": 101,
      "valueCode": "AI_BG",
      "valueLabel": "AI BG",
      "valuePath": "/AI_BG",
      "sourceType": "org_member",
      "status": "active"
    },
    {
      "attrCode": "org_role",
      "attrName": "部门角色",
      "valueCode": "ORG_MANAGER",
      "valueLabel": "部门主管",
      "sourceType": "org_member_role",
      "status": "active"
    },
    {
      "attrCode": "security_level",
      "attrName": "安全等级",
      "numberValue": 3,
      "sourceType": "manual_seed",
      "status": "active"
    }
  ]
}
```

## 查看自己的用户属性

`GET /api/v1/tenants/:id/users/me/attributes`

**允许角色**：`DATA_VISITOR`、`DATA_OWNER`、`TENANT_ADMIN`

**响应**：同“同步用户 CP-ABE 属性”的 `items` 结构，但只返回当前登录用户的有效属性。

## 访问策略保存节点结构

新策略叶子节点建议提交：

```json
{
  "type": "LEAF",
  "attribute": "department",
  "operator": "belongs_to",
  "value": "AI_BG",
  "valueId": 101,
  "valueCode": "AI_BG",
  "label": "AI BG",
  "path": "/AI_BG"
}
```

数字属性示例：

```json
{
  "type": "LEAF",
  "attribute": "security_level",
  "operator": ">=",
  "value": 3,
  "label": "安全等级 3"
}
```

后端保存访问策略时必须重新校验：

- 属性属于当前租户。
- 属性允许用于策略构建。
- 操作符适配属性类型。
- `valueId/valueCode/path` 属于当前租户属性值。
- 不信任前端传入的 `label` 作为匹配依据。

## 平台管理员创建租户管理员账号

`POST /api/v1/platform/tenants/:tenantId/admins`

**允许角色**：`PLATFORM_ADMIN`

**兼容请求体 1：授予已有租户成员租户管理员角色**

```json
{
  "user_id": 1001
}
```

**请求体 2：创建或复用租户管理员账号**

```json
{
  "username": "sangfor_admin",
  "displayName": "深信服租户管理员",
  "email": "sangfor_admin@example.com",
  "phone": "13800000000",
  "password": "lqf999.."
}
```

`password` 可以为空，后端默认使用 `lqf999..`，并将新建账号标记为 `must_change_password = true`。如果邮箱对应用户已存在，后端不重复创建 `users`，只确保 `tenant_users` 和当前租户下的 `TENANT_ADMIN` 授权存在。

**响应**

```json
{
  "tenant_id": 2,
  "user_id": 1001,
  "role": "TENANT_ADMIN",
  "assigned": true,
  "created_user": true,
  "user": {
    "id": 1001,
    "username": "sangfor_admin",
    "email": "sangfor_admin@example.com",
    "nickname": "深信服租户管理员",
    "phone": "13800000000",
    "role": "data_user",
    "must_change_password": true,
    "status": "active"
  }
}
```

**安全边界**

- 非 `PLATFORM_ADMIN` 调用必须被拒绝。
- 新建账号密码必须使用 bcrypt 哈希保存，禁止明文存储。
- 新建租户管理员只获得指定租户下的 `TENANT_ADMIN`，不得获得 `PLATFORM_ADMIN`，也不得把旧 `users.role` 写成 `admin`。
