# 接口契约：用户认证与资料基础模块

## 通用约定

### 基础路径

```text
/api/v1
```

### 统一成功响应

```json
{
  "code": "OK",
  "message": "success",
  "data": {},
  "request_id": "req_xxx"
}
```

### 统一错误响应

```json
{
  "code": "AUTH_INVALID_TOKEN",
  "message": "登录已过期，请重新登录",
  "data": null,
  "request_id": "req_xxx"
}
```

### 常用错误码

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| `BAD_REQUEST` | 400 | 参数错误 |
| `INVALID_EMAIL` | 400 | 邮箱格式错误 |
| `EMAIL_ALREADY_EXISTS` | 409 | 邮箱已存在 |
| `PASSWORD_CONFIRM_MISMATCH` | 400 | 两次密码不一致 |
| `INVALID_ROLE` | 400 | 角色非法 |
| `ADMIN_REGISTER_FORBIDDEN` | 403 | 禁止公开注册 `admin` |
| `INVALID_CREDENTIALS` | 401 | 邮箱或密码错误 |
| `USER_DISABLED` | 403 | 用户已被禁用 |
| `AUTH_ACCESS_TOKEN_MISSING` | 401 | Access Token 缺失 |
| `AUTH_ACCESS_TOKEN_INVALID` | 401 | Access Token 无效 |
| `AUTH_ACCESS_TOKEN_EXPIRED` | 401 | Access Token 已过期 |
| `AUTH_REFRESH_TOKEN_MISSING` | 401 | Refresh Token 缺失 |
| `AUTH_REFRESH_TOKEN_INVALID` | 401 | Refresh Token 无效 |
| `AUTH_REFRESH_TOKEN_EXPIRED` | 401 | Refresh Token 已过期 |
| `AUTH_REFRESH_SESSION_NOT_FOUND` | 401 | Redis 中不存在 Refresh Token 登录态 |
| `AUTH_REFRESH_TOKEN_MISMATCH` | 401 | Refresh Token Hash 不匹配 |
| `AVATAR_EMPTY` | 400 | 头像文件为空 |
| `AVATAR_UNSUPPORTED_TYPE` | 400 | 头像文件类型不支持 |
| `AVATAR_TOO_LARGE` | 400 | 头像文件超过大小限制 |
| `AVATAR_SAVE_FAILED` | 500 | 头像保存失败 |
| `REDIS_WRITE_FAILED` | 500 | 登录态写入失败 |
| `INTERNAL_ERROR` | 500 | 内部错误 |

内部错误细节不得直接返回给前端。登录失败必须统一使用 `INVALID_CREDENTIALS`，不能暴露邮箱是否存在。

## UserDTO

```json
{
  "id": 1,
  "email": "owner@example.com",
  "nickname": "数据拥有者",
  "role": "data_owner",
  "avatar_url": "/uploads/avatars/1/avatar.webp",
  "bio": "示例简介",
  "birthday": "1998-01-01",
  "status": "active",
  "created_at": "2026-07-05T20:00:00+08:00",
  "updated_at": "2026-07-05T20:10:00+08:00"
}
```

所有用户响应不得包含 `password_hash` 和 `avatar_object_key`。

## POST /api/v1/auth/register

### 用途

注册 `data_owner` 或 `data_user` 用户。

### 请求

```json
{
  "email": "owner@example.com",
  "password": "Passw0rd!Demo",
  "confirm_password": "Passw0rd!Demo",
  "nickname": "数据拥有者",
  "role": "data_owner"
}
```

### 校验

- `email` 必填，必须符合邮箱格式，必须唯一。
- `password` 必填。
- `confirm_password` 必填，必须与 `password` 一致。
- `nickname` 必填，长度 1 到 20 个字符。
- `role` 必填，只允许 `data_owner` 或 `data_user`。
- `role=admin` 必须失败。

### 成功响应

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "user": {
      "id": 1,
      "email": "owner@example.com",
      "nickname": "数据拥有者",
      "role": "data_owner",
      "avatar_url": "",
      "bio": "",
      "birthday": null,
      "created_at": "2026-07-05T20:00:00+08:00"
    }
  },
  "request_id": "req_xxx"
}
```

### 处理流程

1. Handler 绑定请求并做基础格式校验。
2. Service 校验角色限制、密码一致性和邮箱唯一性。
3. Service 生成密码哈希。
4. Repository 创建用户，默认状态为 `active`。
5. Service 映射 `UserDTO`。
6. Handler 返回统一响应。

## POST /api/v1/auth/login

### 用途

使用邮箱和密码登录，建立双 Token 登录态。

### 请求

```json
{
  "email": "owner@example.com",
  "password": "Passw0rd!Demo"
}
```

### 成功响应

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "access_token": "access.jwt.value",
    "access_token_expires_in": 900,
    "refresh_token": "refresh.random.value",
    "refresh_token_expires_in": 604800,
    "token_type": "Bearer",
    "user": {
      "id": 1,
      "email": "owner@example.com",
      "nickname": "数据拥有者",
      "role": "data_owner",
      "avatar_url": "",
      "bio": "",
      "birthday": null,
      "created_at": "2026-07-05T20:00:00+08:00"
    }
  },
  "request_id": "req_xxx"
}
```

### 处理流程

1. Handler 绑定请求。
2. Service 按邮箱查询用户。
3. Service 校验用户存在、状态为 `active`、密码匹配。
4. Auth 组件生成 Access Token、Refresh Token、`jti`、`token_id` 和 `session_id`。
5. Redis Token Store 保存 Refresh Session，TTL 等于 Refresh Token 有效期。
6. Handler 返回 Token 和用户基础信息。

## POST /api/v1/auth/refresh

### 用途

使用 Refresh Token 刷新 Access Token。默认采用 Refresh Token 轮换。

### 请求

```json
{
  "refresh_token": "refresh.random.value"
}
```

如后续改为 Cookie 传递，前端不需要在请求体中传 `refresh_token`，但服务端仍必须校验刷新凭证来源和类型。

### 成功响应

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "access_token": "new.access.jwt.value",
    "access_token_expires_in": 900,
    "refresh_token": "new.refresh.random.value",
    "refresh_token_expires_in": 604800,
    "token_type": "Bearer"
  },
  "request_id": "req_xxx"
}
```

### 处理流程

1. Handler 获取 Refresh Token。
2. Auth 组件解析或提取 `token_id`。
3. Redis Token Store 查询 `auth:refresh:{token_id}`。
4. Token Store 比对 Refresh Token Hash。
5. Service 校验用户仍存在且状态为 `active`。
6. Auth 组件生成新的 Access Token。
7. 若启用轮换，Token Store 删除旧 Key，写入新 Refresh Session。
8. Handler 返回新的 Token 信息。

## POST /api/v1/auth/logout

### 用途

退出当前会话，使当前 Refresh Token 失效。

### 请求

```json
{
  "refresh_token": "refresh.random.value"
}
```

### 成功响应

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "logged_out": true
  },
  "request_id": "req_xxx"
}
```

### 处理流程

1. Handler 获取 Refresh Token。
2. Auth 组件提取 `token_id`。
3. Token Store 查询并校验 Hash。
4. Token Store 删除当前 Refresh Session。
5. 如果使用 Cookie，Handler 同时清除 `refresh_token` Cookie。
6. Access Token 本阶段不加入黑名单，等待自然过期。

## GET /api/v1/users/me

### 用途

获取当前登录用户资料。

### 请求头

```text
Authorization: Bearer <access_token>
```

### 成功响应

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "user": {
      "id": 1,
      "email": "owner@example.com",
      "nickname": "数据拥有者",
      "role": "data_owner",
      "avatar_url": "",
      "bio": "",
      "birthday": null,
      "status": "active",
      "created_at": "2026-07-05T20:00:00+08:00",
      "updated_at": "2026-07-05T20:00:00+08:00"
    }
  },
  "request_id": "req_xxx"
}
```

### 处理流程

1. Middleware 校验 Access Token，要求 `token_type=access`。
2. Middleware 注入 `user_id` 和 `role`。
3. Handler 从上下文读取当前用户 ID。
4. Service 查询用户并映射 `UserDTO`。
5. Handler 返回当前用户资料。

## PUT /api/v1/users/me

### 用途

编辑当前用户个人资料。

### 请求头

```text
Authorization: Bearer <access_token>
```

### 请求

```json
{
  "nickname": "新的昵称",
  "bio": "新的个人简介",
  "birthday": "1998-01-01"
}
```

### 校验

- 只能修改 `nickname`、`bio`、`birthday`。
- `nickname` 长度 1 到 20 个字符。
- `bio` 不超过 200 个字符。
- `birthday` 为空或格式为 `YYYY-MM-DD`。
- 请求中的 `email`、`password`、`role`、`status` 必须被忽略或拒绝。

### 成功响应

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "user": {
      "id": 1,
      "email": "owner@example.com",
      "nickname": "新的昵称",
      "role": "data_owner",
      "avatar_url": "",
      "bio": "新的个人简介",
      "birthday": "1998-01-01",
      "status": "active",
      "created_at": "2026-07-05T20:00:00+08:00",
      "updated_at": "2026-07-05T20:10:00+08:00"
    }
  },
  "request_id": "req_xxx"
}
```

## POST /api/v1/users/me/avatar

### 用途

上传当前用户头像。

### 请求头

```text
Authorization: Bearer <access_token>
Content-Type: multipart/form-data
```

### 表单字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `avatar` | 文件 | `jpg`、`jpeg`、`png` 或 `webp`，不超过 2MB |

### 成功响应

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "avatar_url": "/uploads/avatars/1/20260705201000_abcd.webp"
  },
  "request_id": "req_xxx"
}
```

### 处理流程

1. Middleware 校验 Access Token。
2. Handler 读取 `avatar` 文件并做基础大小、空文件和格式校验。
3. Service 生成头像对象 Key。
4. Storage 保存文件并返回 `avatar_url` 与 `avatar_object_key`。
5. Repository 更新当前用户头像字段。
6. 本阶段只覆盖数据库记录；后续可根据旧 `avatar_object_key` 删除旧头像文件。
