# 快速验证指南：CP-ABE 系统多租户基础能力

## 前置条件

- 已配置 `backend/.env` 或环境变量，包括 MySQL、Redis、`JWT_SECRET`。
- MySQL 和 Redis 可用。
- 当前功能已完成实现任务后再执行本指南。

## 启动后端

```bash
cd backend
go test ./...
go run ./cmd/server
```

期望：

- 测试通过。
- 服务正常启动。
- 新增租户、角色、成员关系模型完成迁移。

## 初始化默认租户和管理员

示例命令以最终实现为准，计划建议扩展现有管理员初始化命令：

```bash
cd backend
ADMIN_PASSWORD='Admin@123456' go run ./cmd/admin create -email admin@example.com -nickname 管理员
```

期望：

- 存在 `default-tenant`。
- 存在 `TENANT_ADMIN`、`DO`、`DU`、`PLATFORM_ADMIN` 角色。
- 默认管理员属于默认租户，并拥有 `TENANT_ADMIN` 角色。
- 重复执行不会产生重复租户、角色或关系。

## 注册普通用户

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "owner@example.com",
    "password": "Passw0rd!",
    "confirm_password": "Passw0rd!",
    "nickname": "数据拥有者",
    "role": "data_owner"
  }'
```

期望：

- 注册成功。
- 用户自动加入默认租户。
- 用户在默认租户下拥有 `DO` 角色。

## 登录并查看租户列表

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "owner@example.com",
    "password": "Passw0rd!"
  }'
```

期望：

- 响应包含 `access_token`、`refresh_token`。
- 响应包含 `current_tenant_id`。
- 响应包含 `tenants`，其中默认租户角色包含 `DO`。
- 响应不包含 `password_hash` 或 `avatar_object_key`。

## 查询当前用户租户列表

```bash
curl -sS http://127.0.0.1:8080/api/v1/me/tenants \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

期望：

- 返回当前用户所属启用租户。
- 租户项包含角色列表。

## 切换到所属租户

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/me/switch-tenant \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": 1001
  }'
```

期望：

- 切换成功。
- 响应返回 `current_tenant_id`、当前租户信息、角色列表和 `menus` 预留字段。

## 拒绝切换到非成员租户

准备两个租户，只把用户加入租户 A，然后尝试切换到租户 B：

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/me/switch-tenant \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": 2002
  }'
```

期望：

- 返回 403。
- 错误码为 `TENANT_MEMBER_FORBIDDEN` 或等效错误。

## 拒绝切换到禁用租户

管理员禁用租户后，普通用户再次切换：

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/me/switch-tenant \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": 1001
  }'
```

期望：

- 返回 403。
- 错误码为 `TENANT_DISABLED`。

## 验证租户上下文中间件

对后续任一租户内业务测试接口或示例接口发起请求：

```bash
curl -sS http://127.0.0.1:8080/api/v1/example-tenant-resource \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "X-Tenant-Id: 1001"
```

期望：

- 后端能从请求上下文读取当前 `tenant_id`。
- 缺少 `X-Tenant-Id` 时返回租户上下文缺失。
- 传入非成员租户 ID 时返回拒绝访问。

## 验证数据隔离

后续属性、文件、策略、加解密记录和审计日志接口完成后，必须执行以下验证：

1. 在租户 A 和租户 B 创建同名属性或文件。
2. 使用租户 A 用户携带 `X-Tenant-Id: A` 查询，只能看到租户 A 数据。
3. 使用租户 A 用户尝试访问租户 B 数据 ID，必须失败。
4. 检查详情、更新、删除均使用 `id + tenant_id` 限定。

## 文档和注释检查

实现完成后检查：

- 新增文档为简体中文。
- 涉及认证、权限、Token、租户隔离和访问控制的关键代码包含解释安全边界的中文注释。
- 没有用 Tenant Admin、DO、DU 角色暗示可解密所有文件。
