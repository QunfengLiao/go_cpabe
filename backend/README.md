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

HTTP 服务默认不再执行 `AutoMigrate` 或初始化数据，避免启动时反复触发 `information_schema`
检查和 seed 写入。首次部署或表结构变化时显式执行：

```bash
go run ./cmd/migrate
```

基础租户、基础角色和历史用户兼容数据通过独立 seed 命令写入：

```bash
go run ./cmd/seed
```

需要演示策略、组织和属性数据时再追加 `-demo`：

```bash
go run ./cmd/seed -demo
```

兼容开发环境的一键启动仍可通过环境变量显式开启，但默认值必须保持关闭：

- `RUN_AUTO_MIGRATE=false`
- `RUN_SEED=false`
- `RUN_DEMO_SEED=false`

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

后续修改 Go 业务代码时，需要遵守项目宪章中的 AI 协作与代码注释规范。每个函数和方法前
都必须有中文注释；导出的函数、方法、类型、接口、常量和变量必须符合 GoDoc 规范，注释
必须以标识符名称开头。

新增或修改 Service、Repository、Handler/Middleware 中的业务规则，以及 Crypto、Policy、
Benchmark、Audit 相关实现后，必须执行“关键注释和可读性检查”。检查重点是确认实体字段、
Handler、Service、Repository、Middleware 的注释解释了业务语义、副作用、风险点、安全边界
和关键取舍，而不是重复代码表面行为。
