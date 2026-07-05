# 快速验证指南：用户认证与资料基础模块

## 前置条件

- MySQL 可用，并已创建项目数据库。
- Redis 可用。
- 后端配置已提供以下项：
  - `MYSQL_DSN`
  - `REDIS_ADDR`
  - `REDIS_PASSWORD`
  - `REDIS_DB`
  - `JWT_SECRET`
  - `ACCESS_TOKEN_TTL`
  - `REFRESH_TOKEN_TTL`
  - `AVATAR_UPLOAD_DIR`
  - `AVATAR_URL_PREFIX`
  - `AVATAR_MAX_SIZE`
- 已执行 `users` 表 migration，详见 [data-model.md](./data-model.md)。

## 启动服务

实际命令由后续实现阶段确定。建议后端提供以下等价能力：

```bash
cd backend
go test ./...
go run ./cmd/server
```

服务启动后，接口基础路径为：

```text
http://localhost:8080/api/v1
```

## 验证场景

### 1. 注册 data_owner 成功

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "owner@example.com",
    "password": "Passw0rd!Demo",
    "confirm_password": "Passw0rd!Demo",
    "nickname": "数据拥有者",
    "role": "data_owner"
  }'
```

期望结果：

- 返回 `code=OK`。
- 返回用户基础信息。
- 不包含 `password_hash` 和 `avatar_object_key`。

### 2. 注册 admin 被拒绝

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "admin@example.com",
    "password": "Passw0rd!Demo",
    "confirm_password": "Passw0rd!Demo",
    "nickname": "管理员",
    "role": "admin"
  }'
```

期望结果：

- 返回 `ADMIN_REGISTER_FORBIDDEN` 或等价错误码。
- 数据库中不会创建该用户。

### 3. 登录成功并写入 Redis 登录态

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "owner@example.com",
    "password": "Passw0rd!Demo"
  }'
```

期望结果：

- 返回 `access_token` 和 `refresh_token`。
- 返回 `token_type=Bearer`。
- Redis 中存在对应 `auth:refresh:{token_id}` 登录态，且 TTL 与 Refresh Token 有效期一致。

### 4. 错误密码不暴露具体原因

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "owner@example.com",
    "password": "wrong-password"
  }'
```

期望结果：

- 返回 `INVALID_CREDENTIALS`。
- 提示为统一的“邮箱或密码错误”含义。

### 5. 获取当前用户资料

```bash
curl http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

期望结果：

- 有效 Access Token 返回当前用户资料。
- 不携带 Token、携带过期 Access Token、携带 Refresh Token 均被拒绝。
- 响应不包含 `password_hash` 和 `avatar_object_key`。

### 6. 刷新 Token

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d "{
    \"refresh_token\": \"${REFRESH_TOKEN}\"
  }"
```

期望结果：

- 返回新的 Access Token。
- 若启用轮换，同时返回新的 Refresh Token。
- 旧 Refresh Token 不能再次刷新。

### 7. 编辑个人资料

```bash
curl -X PUT http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "nickname": "新的昵称",
    "bio": "新的个人简介",
    "birthday": "1998-01-01"
  }'
```

期望结果：

- 返回更新后的用户资料。
- 邮箱、密码、角色、状态不会被修改。
- 昵称为空、昵称超过 20 个字符、简介超过 200 个字符、生日格式错误时返回参数错误。

### 8. 上传头像

```bash
curl -X POST http://localhost:8080/api/v1/users/me/avatar \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -F "avatar=@./avatar.webp"
```

期望结果：

- 合法 `jpg`、`jpeg`、`png`、`webp` 且不超过 2MB 的文件上传成功。
- 返回新的 `avatar_url`。
- 再次获取当前用户资料时能看到新的 `avatar_url`。
- 空文件、不支持格式、超过 2MB 的文件会失败。

### 9. 退出登录

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H 'Content-Type: application/json' \
  -d "{
    \"refresh_token\": \"${REFRESH_TOKEN}\"
  }"
```

期望结果：

- Redis 中当前 Refresh Token 登录态被删除。
- 旧 Refresh Token 不能再刷新 Access Token。
- Access Token 本阶段等待自然过期。

## 完成判定

- `go test ./...` 通过。2026-07-05 已在 `backend/` 下验证通过。
- 上述 9 个验证场景均符合预期。
- 所有用户响应均不包含 `password_hash` 和 `avatar_object_key`。
- Refresh Token 在 Redis 中不以明文保存。
- Handler、Service、Repository、Auth、Token Store、Middleware、Storage 职责没有交叉污染。
