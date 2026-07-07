# 接口契约：多租户基础能力

## 通用约定

- 基础路径：`/api/v1`
- 响应信封沿用现有格式：

```json
{
  "code": "OK",
  "message": "success",
  "data": {},
  "request_id": ""
}
```

- 受保护接口使用 `Authorization: Bearer <access_token>`。
- 租户内业务接口额外使用 `X-Tenant-Id: <tenant_id>`。
- 本契约使用 snake_case JSON 字段以匹配当前后端响应风格。

## 登录响应扩展

`POST /api/v1/auth/login`

成功响应在现有字段基础上增加 `current_tenant_id` 和 `tenants`：

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
      "email": "user@example.com",
      "nickname": "测试用户",
      "role": "data_user",
      "avatar_url": "",
      "bio": "",
      "birthday": null,
      "created_at": "2026-07-07T00:00:00Z"
    },
    "current_tenant_id": 1001,
    "tenants": [
      {
        "tenant_id": 1001,
        "tenant_name": "默认租户",
        "tenant_code": "default-tenant",
        "roles": ["DU"]
      }
    ]
  },
  "request_id": ""
}
```

规则：

- 单启用租户用户返回该租户 ID。
- 多启用租户用户可以返回 `current_tenant_id: null`。
- 无启用租户用户返回空租户列表，前端不得进入租户内业务。

## 查询当前用户租户列表

`GET /api/v1/me/tenants`

请求头：

```text
Authorization: Bearer <access_token>
```

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "current_tenant_id": 1001,
    "tenants": [
      {
        "tenant_id": 1001,
        "tenant_name": "默认租户",
        "tenant_code": "default-tenant",
        "status": "enabled",
        "roles": ["DU"]
      }
    ]
  },
  "request_id": ""
}
```

## 切换租户

`POST /api/v1/me/switch-tenant`

请求头：

```text
Authorization: Bearer <access_token>
```

请求体：

```json
{
  "tenant_id": 1001
}
```

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "current_tenant_id": 1001,
    "tenant": {
      "tenant_id": 1001,
      "tenant_name": "默认租户",
      "tenant_code": "default-tenant",
      "status": "enabled"
    },
    "roles": ["TENANT_ADMIN"],
    "menus": []
  },
  "request_id": ""
}
```

失败情况：

- 目标租户不存在：`TENANT_NOT_FOUND`。
- 目标租户禁用：`TENANT_DISABLED`。
- 当前用户不是目标租户成员：`TENANT_MEMBER_FORBIDDEN`。
- 成员关系禁用：`TENANT_MEMBER_DISABLED`。

## 创建租户

`POST /api/v1/tenants`

请求头：

```text
Authorization: Bearer <access_token>
```

请求体：

```json
{
  "name": "实验室 A",
  "code": "lab-a",
  "status": "enabled",
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
      "tenant_id": 1002,
      "tenant_name": "实验室 A",
      "tenant_code": "lab-a",
      "status": "enabled",
      "description": "密码学实验室"
    }
  },
  "request_id": ""
}
```

## 查询租户列表

`GET /api/v1/tenants`

规则：

- Platform Admin 返回全部租户。
- 普通用户返回自己所属租户。

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenants": [
      {
        "tenant_id": 1001,
        "tenant_name": "默认租户",
        "tenant_code": "default-tenant",
        "status": "enabled"
      }
    ]
  },
  "request_id": ""
}
```

## 查询租户详情

`GET /api/v1/tenants/{tenantId}`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant": {
      "tenant_id": 1001,
      "tenant_name": "默认租户",
      "tenant_code": "default-tenant",
      "status": "enabled",
      "description": "默认演示租户"
    }
  },
  "request_id": ""
}
```

## 启用租户

`PATCH /api/v1/tenants/{tenantId}/enable`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_id": 1001,
    "status": "enabled"
  },
  "request_id": ""
}
```

## 禁用租户

`PATCH /api/v1/tenants/{tenantId}/disable`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_id": 1001,
    "status": "disabled"
  },
  "request_id": ""
}
```

## 将用户加入租户

`POST /api/v1/tenants/{tenantId}/users`

请求体：

```json
{
  "user_id": 2,
  "roles": ["DO"]
}
```

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_id": 1001,
    "user_id": 2,
    "status": "active",
    "roles": ["DO"]
  },
  "request_id": ""
}
```

## 将用户移出租户

`DELETE /api/v1/tenants/{tenantId}/users/{userId}`

成功响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "tenant_id": 1001,
    "user_id": 2,
    "removed": true
  },
  "request_id": ""
}
```

## 查询租户用户列表

`GET /api/v1/tenants/{tenantId}/users`

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
        "nickname": "测试用户",
        "member_status": "active",
        "roles": ["DO"]
      }
    ]
  },
  "request_id": ""
}
```

## 租户上下文错误码

| 错误码 | HTTP 状态 | 含义 |
|--------|-----------|------|
| `TENANT_ID_MISSING` | 400 | 租户上下文缺失 |
| `TENANT_ID_INVALID` | 400 | 租户标识格式非法 |
| `TENANT_NOT_FOUND` | 404 | 租户不存在 |
| `TENANT_DISABLED` | 403 | 租户已禁用 |
| `TENANT_MEMBER_FORBIDDEN` | 403 | 当前用户不属于该租户 |
| `TENANT_MEMBER_DISABLED` | 403 | 当前用户在该租户的成员关系已禁用 |
| `TENANT_PERMISSION_DENIED` | 403 | 当前租户角色无权执行该操作 |
| `TENANT_CODE_EXISTS` | 409 | 租户编码已存在 |
