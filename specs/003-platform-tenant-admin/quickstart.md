# 快速验证指南：Platform Admin 平台管理员租户管理能力

## 前置条件

- 已配置 `backend/.env` 或环境变量，包含 MySQL、Redis、`JWT_SECRET`。
- MySQL 和 Redis 可用。
- 已完成本功能实现任务后再执行本指南。
- 后端默认监听地址以当前配置为准；以下示例使用 `http://127.0.0.1:18080/api/v1`。

## 启动后端测试

```bash
cd backend
go test ./...
go run ./cmd/server
```

期望：

- 后端测试通过。
- 服务正常启动。
- 初始化流程可幂等创建基础角色和演示租户。

## 初始化平台管理员

推荐实现后的命令形式：

```bash
cd backend
ADMIN_PASSWORD='Admin@123456' go run ./cmd/admin create-platform -email platform@example.com -nickname 平台管理员
```

期望：

- 存在 `PLATFORM_ADMIN`、`TENANT_ADMIN`、`DO`、`DU`。
- 存在 `scnu`、`sangfor`、`aia-hk` 和 `default-tenant`。
- `platform@example.com` 拥有平台级 `PLATFORM_ADMIN`。
- 重复执行命令不会创建重复角色、重复租户或重复平台角色关系。

## 登录平台管理员

```bash
curl -sS -X POST http://127.0.0.1:18080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "platform@example.com",
    "password": "Admin@123456"
  }'
```

期望：

- 响应包含 `access_token`。
- 响应包含 `platform_roles`，且包含 `PLATFORM_ADMIN`。
- 响应不包含 `password_hash`。

保存 token：

```bash
ACCESS_TOKEN='复制上一步返回的 access_token'
```

## 查询平台租户列表

```bash
curl -sS http://127.0.0.1:18080/api/v1/platform/tenants \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

期望：

- 返回 200。
- 返回租户列表。
- 包含 `scnu`、`sangfor`、`aia-hk`。

## 创建租户

```bash
curl -sS -X POST http://127.0.0.1:18080/api/v1/platform/tenants \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "实验室 A",
    "code": "lab-a",
    "description": "密码学实验室"
  }'
```

期望：

- 返回 201。
- 租户状态为 `enabled`。
- 再次使用 `lab-a` 创建返回重复编码错误。

## 校验租户编码格式

```bash
curl -sS -X POST http://127.0.0.1:18080/api/v1/platform/tenants \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "非法编码租户",
    "code": "Lab_A",
    "description": "应被拒绝"
  }'
```

期望：

- 返回 400。
- 错误信息说明租户编码格式非法或参数错误。

## 启用和禁用租户

```bash
curl -sS -X PATCH http://127.0.0.1:18080/api/v1/platform/tenants/1/disable \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

期望：

- 返回 200。
- 租户状态为 `disabled`。
- 该租户内普通用户不能切换进入该租户。

重新启用：

```bash
curl -sS -X PATCH http://127.0.0.1:18080/api/v1/platform/tenants/1/enable \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

## 管理租户用户关系

将用户加入租户：

```bash
curl -sS -X POST http://127.0.0.1:18080/api/v1/platform/tenants/1/users \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "user_id": 2
  }'
```

期望：

- 用户加入租户成功。
- 重复执行不产生重复成员关系。

查询租户用户：

```bash
curl -sS http://127.0.0.1:18080/api/v1/platform/tenants/1/users \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

期望：

- 返回成员列表、成员状态和角色列表。

移出用户：

```bash
curl -sS -X DELETE http://127.0.0.1:18080/api/v1/platform/tenants/1/users/2 \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

期望：

- 用户被移出或成员状态禁用。
- 用户不能再切换进入该租户。

## 分配 Tenant Admin

```bash
curl -sS -X POST http://127.0.0.1:18080/api/v1/platform/tenants/1/admins \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "user_id": 2
  }'
```

期望：

- 用户必须已经属于该租户。
- 分配成功后，用户在该租户角色包含 `TENANT_ADMIN`。
- 重复分配不产生重复角色关系。

移除 Tenant Admin：

```bash
curl -sS -X DELETE http://127.0.0.1:18080/api/v1/platform/tenants/1/admins/2 \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

期望：

- 用户在该租户下的 `TENANT_ADMIN` 被移除。
- 如果这是最后一个有效 Tenant Admin，应返回拒绝错误。

## 校验非平台管理员不能访问平台接口

使用普通用户登录并保存 `NORMAL_TOKEN` 后执行：

```bash
curl -sS http://127.0.0.1:18080/api/v1/platform/tenants \
  -H "Authorization: Bearer $NORMAL_TOKEN"
```

期望：

- 返回 403。
- 错误码为 `PLATFORM_PERMISSION_DENIED` 或等价权限不足错误。

## 校验禁用租户后不能进入

普通用户尝试切换到已禁用租户：

```bash
curl -sS -X POST http://127.0.0.1:18080/api/v1/me/switch-tenant \
  -H "Authorization: Bearer $NORMAL_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": 1
  }'
```

期望：

- 返回 403。
- 错误码为 `TENANT_DISABLED`。

## 前端验证

```bash
cd desktop
npm run typecheck
npm run build
npm run dev:electron:sangfor
```

期望：

- 类型检查通过。
- 构建通过。
- `dev:electron:sangfor` 仍进入 `#/login/sangfor`，不被旧登录态覆盖。
- Platform Admin 登录后进入平台后台或可见平台后台入口。
- 非 Platform Admin 登录后看不到平台管理菜单。
- 手动输入平台后台路径时，非 Platform Admin 被前端提示无权限，后端接口仍返回 403。

## 安全边界检查

实现完成后逐项检查：

- 平台管理响应不包含 `password_hash`。
- 平台管理响应不包含主密钥明文。
- 平台管理响应不包含用户私钥明文。
- 平台管理响应不包含文件明文。
- Platform Admin 不能绕过 CP-ABE 策略解密文件。
- 新增认证、权限、Token、租户隔离相关代码包含解释安全边界的中文注释。
