# 接口契约：Platform Admin 平台租户管理

## 通用约定

- 基础路径：`/api/v1`
- 平台后台路径前缀：`/api/v1/platform`
- 响应信封沿用当前格式：

```json
{
  "code": "OK",
  "message": "success",
  "data": {},
  "request_id": ""
}
```

- 平台接口必须携带：

```text
Authorization: Bearer <access_token>
```

- 平台接口不要求 `X-Tenant-Id`，也不应依赖当前租户上下文。
- 平台接口只允许拥有平台级 `PLATFORM_ADMIN` 的用户访问。

## 登录响应扩展

`POST /api/v1/auth/login`

成功响应在现有字段基础上增加平台角色信息：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "access_token": "access-token",
    "access_token_expires_in": 900,
    "refresh_token": "refresh-token",
    "refresh_token_expires_in": 604800,
    "token_type": "Bearer",
    "user": {
      "id": 1,
      "email": "platform@example.com",
      "nickname": "平台管理员",
      "role": "admin",
      "avatar_url": "",
      "bio": "",
      "birthday": null,
      "created_at": "2026-07-07T00:00:00Z"
    },
    "platform_roles": ["PLATFORM_ADMIN"],
    "current_tenant_id": null,
    "current_tenant_code": null,
    "tenants": []
  },
  "request_id": ""
}
```

规则：

- `platform_roles` 用于前端菜单判断，不作为安全边界。
- 后端平台接口仍必须查询服务端角色关系。
- `user` 不得包含 `password_hash`。

## 查询平台控制台

`GET /api/v1/platform/dashboard`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_count": 4,
    "enabled_tenant_count": 3,
    "disabled_tenant_count": 1,
    "user_count": 12,
    "tenant_user_count": 20,
    "tenant_admin_count": 5,
    "audit_enabled": false
  },
  "request_id": ""
}
```

## 查询租户列表

`GET /api/v1/platform/tenants`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenants": [
      {
        "tenant_id": 1,
        "tenant_name": "四川师范大学",
        "tenant_code": "scnu",
        "status": "enabled",
        "description": "科研数据安全共享演示租户",
        "user_count": 8,
        "tenant_admin_count": 1
      }
    ]
  },
  "request_id": ""
}
```

## 创建租户

`POST /api/v1/platform/tenants`

请求体：

```json
{
  "name": "实验室 A",
  "code": "lab-a",
  "description": "密码学实验室"
}
```

成功响应：`201 Created`

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant": {
      "tenant_id": 10,
      "tenant_name": "实验室 A",
      "tenant_code": "lab-a",
      "status": "enabled",
      "description": "密码学实验室"
    }
  },
  "request_id": ""
}
```

失败情况：

- 名称为空：`BAD_REQUEST`。
- 编码为空：`BAD_REQUEST`。
- 编码格式非法：建议 `TENANT_CODE_INVALID` 或 `BAD_REQUEST`。
- 编码重复：`TENANT_CODE_EXISTS`。

## 查询租户详情

`GET /api/v1/platform/tenants/{tenantId}`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant": {
      "tenant_id": 1,
      "tenant_name": "深信服科技",
      "tenant_code": "sangfor",
      "status": "enabled",
      "description": "企业安全协作演示租户",
      "user_count": 6,
      "tenant_admin_count": 1,
      "created_at": "2026-07-07T00:00:00Z",
      "updated_at": "2026-07-07T00:00:00Z"
    }
  },
  "request_id": ""
}
```

## 启用租户

`PATCH /api/v1/platform/tenants/{tenantId}/enable`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_id": 1,
    "status": "enabled"
  },
  "request_id": ""
}
```

## 禁用租户

`PATCH /api/v1/platform/tenants/{tenantId}/disable`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_id": 1,
    "status": "disabled"
  },
  "request_id": ""
}
```

规则：

- 禁用不删除租户历史数据。
- 禁用后普通用户不能切换进入该租户。
- Platform Admin 仍可查看租户详情。

## 查询租户用户列表

`GET /api/v1/platform/tenants/{tenantId}/users`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "users": [
      {
        "user_id": 2,
        "email": "user@example.com",
        "nickname": "数据使用者",
        "member_status": "active",
        "roles": ["DU"]
      }
    ]
  },
  "request_id": ""
}
```

## 将用户加入租户

`POST /api/v1/platform/tenants/{tenantId}/users`

请求体：

```json
{
  "user_id": 2
}
```

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_id": 1,
    "user_id": 2,
    "member_status": "active",
    "roles": []
  },
  "request_id": ""
}
```

规则：

- 用户必须存在。
- 租户必须存在且默认要求启用。
- 重复加入应幂等返回现有关系。

## 将用户移出租户

`DELETE /api/v1/platform/tenants/{tenantId}/users/{userId}`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_id": 1,
    "user_id": 2,
    "removed": true
  },
  "request_id": ""
}
```

规则：

- 如果该用户是最后一个有效 Tenant Admin，默认拒绝移出。

## 分配 Tenant Admin

`POST /api/v1/platform/tenants/{tenantId}/admins`

请求体：

```json
{
  "user_id": 2
}
```

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_id": 1,
    "user_id": 2,
    "role": "TENANT_ADMIN",
    "assigned": true
  },
  "request_id": ""
}
```

规则：

- 用户必须属于该租户。
- 重复分配应幂等返回成功。
- 该角色只在当前租户生效。

## 移除 Tenant Admin

`DELETE /api/v1/platform/tenants/{tenantId}/admins/{userId}`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_id": 1,
    "user_id": 2,
    "role": "TENANT_ADMIN",
    "removed": true
  },
  "request_id": ""
}
```

规则：

- 如果该用户是最后一个有效 Tenant Admin，默认拒绝移除。

## 查询平台用户列表

`GET /api/v1/platform/users`

本接口为平台用户管理页面建议接口，可 P2 实现。

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "users": [
      {
        "user_id": 1,
        "email": "platform@example.com",
        "nickname": "平台管理员",
        "status": "active",
        "platform_roles": ["PLATFORM_ADMIN"],
        "tenant_count": 0
      }
    ]
  },
  "request_id": ""
}
```

## 错误码建议

| 错误码 | HTTP 状态 | 含义 |
|--------|-----------|------|
| `PLATFORM_PERMISSION_DENIED` | 403 | 当前用户不是平台管理员 |
| `TENANT_NOT_FOUND` | 404 | 租户不存在 |
| `TENANT_DISABLED` | 403 | 租户已禁用 |
| `TENANT_CODE_EXISTS` | 409 | 租户编码已存在 |
| `TENANT_CODE_INVALID` | 400 | 租户编码格式非法 |
| `TENANT_MEMBER_FORBIDDEN` | 403 | 用户不属于目标租户 |
| `TENANT_LAST_ADMIN_FORBIDDEN` | 409 | 不能移除最后一个租户管理员 |
| `INVALID_ROLE` | 400 | 角色非法 |
| `BAD_REQUEST` | 400 | 参数错误 |

## 安全响应边界

平台接口不得返回：

- `password_hash`
- 主密钥明文
- 用户私钥明文
- 密码明文
- 文件明文内容
- 可绕过 CP-ABE 策略的任何敏感秘密
