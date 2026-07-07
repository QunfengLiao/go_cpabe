# 后端说明：用户认证与资料基础模块

本后端模块实现用户注册、登录、双 Token 登录态、刷新 Token、退出登录、当前用户资料、资料编辑和头像上传。

## 配置

复制 `.env.example` 并按本地环境设置：

```bash
cp .env.example .env
```

关键配置包括：

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

## 数据库

开发环境启动时会执行 `AutoMigrate`。受控环境建议执行：

```bash
mysql < migrations/001_create_users.sql
```

## 运行

```bash
go test ./...
go run ./cmd/server
```

## 管理员初始化

管理员账号不能通过公开注册接口创建，必须走本地受控初始化命令写入 `users` 表。
推荐通过环境变量传入密码，避免明文密码进入命令历史。

PowerShell 示例：

```powershell
$env:ADMIN_PASSWORD = "Admin@123456"
go run ./cmd/admin create -email admin@example.com -nickname 管理员
Remove-Item Env:ADMIN_PASSWORD
```

也可以显式传入 `-password`，但不推荐在共享机器或生产环境使用：

```bash
go run ./cmd/admin create -email admin@example.com -password Admin@123456 -nickname 管理员
```

## 验证

服务启动后可运行：

```bash
BASE_URL=http://localhost:18080/api/v1 ./scripts/verify_auth_flow.sh
```

所有用户响应都必须避免返回 `password_hash` 和 `avatar_object_key`。Refresh Token 登录态保存在 Redis，Redis 中只保存 Refresh Token Hash。

## 代码注释要求

后续修改核心业务代码时，需要遵守项目宪章中的 AI 协作与代码注释规范。涉及认证、Token、
密码、文件上传和访问控制的逻辑，应使用必要中文注释说明设计原因和安全边界，避免无意义
逐行注释。
