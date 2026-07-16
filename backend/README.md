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
- `ENCRYPTED_FILE_STORAGE_DIR`：正式密文对象目录，不得注册静态路由
- `ENCRYPTED_FILE_TEMP_DIR`：上传暂存目录
- `ENCRYPTED_FILE_MAX_SIZE`：明文大小上限，默认 1 GiB
- `ENCRYPTION_MAX_CONCURRENT_PER_TENANT`：租户并发租约上限，默认 3
- `ENCRYPTION_STAGING_TTL`：暂存与异常对象巡检时限
- `AUDIT_DISPATCH_BATCH_SIZE`：单次审计投递上限，范围 1–1000
- `AUDIT_DISPATCH_LEASE`：处理中事件的租约时长，至少 1 秒
- `AUDIT_DISPATCH_MAX_RETRIES`：进入死信前允许的最大失败次数，范围 1–100
- `AUDIT_DISPATCH_BASE_BACKOFF`：首次重试退避基数，至少 1 秒
- `AUDIT_DISPATCH_MAX_BACKOFF`：指数退避上限，不得小于基础退避
- `AUDIT_DELIVERED_RETENTION`：已投递 Outbox 记录保留期，至少 1 小时

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

## 审计 Dispatcher

安全关键业务事件先写入独立的 `audit_outbox`，再由 Dispatcher 幂等投递到 `audit_logs`。
该表与 `orphan_storage_objects` 完全分离：文件孤儿清理只删除无业务引用的密文，不能领取、
更新或删除审计事件。部署或升级后必须先执行 `go run ./cmd/migrate`，确保 014 迁移及
`audit_logs.metadata_redacted` 已就绪。

Dispatcher 是单批次命令：每次领取一批到期事件，完成投递、重试或死信状态更新后退出。
可使用操作系统计划任务、容器定时任务或其他受控调度器重复执行：

```bash
go run ./cmd/audit-dispatcher
go run ./cmd/audit-dispatcher -limit 100
```

可以并行启动多个实例；数据库租约和 `SKIP LOCKED` 用于避免同一事件被同时处理，稳定事件
UUID 保证重复投递最终只产生一条正式审计日志。`RETRY` 事件按指数退避再次领取，超过
`AUDIT_DISPATCH_MAX_RETRIES` 后进入 `DEAD_LETTER`，死信必须人工诊断并受控重放，不能自动删除。
`AUDIT_DELIVERED_RETENTION` 只控制已投递 Outbox 记录的清理期限，不删除正式审计日志。

同库 Outbox 只能保证 MySQL 可用时业务事实与待投递事件原子提交；如果 MySQL 整体不可用，
系统只能输出不含 Metadata 和原始数据库错误的结构化告警，不能声称审计零丢失。

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
BASE_URL=http://localhost:8080/api/v1 ./scripts/verify_auth_flow.sh
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

## 加密文件框架

首期使用 RSA-OAEP-SHA-256 保护随机数据密钥 DEK，文件内容始终由本地 Go Crypto Worker
使用 AES-256-GCM 认证分块加密。远程服务只接收 `GCPABE01` 密文容器、受保护 DEK 和脱敏
元数据，不接收文件明文、明文 DEK、RSA 私钥或客户端本地路径。可见文件的密文下载和密钥信封读取复用文件可见性校验，不使用 `file.decrypt.invoke` 判断最终能否解密；服务端不执行 RSA 解封装、AES 解密或明文返回。RSA 是首个适配器，不是
通用文件加密、上传、任务、审计或补偿框架的依赖。

部署前必须执行显式迁移。补偿清理命令可重复运行：

```bash
go run ./cmd/migrate
go run ./cmd/cleanup -limit 100
```

Benchmark 必须分别解释 AES 文件加密耗时、DEK 保护耗时、上传耗时和密文大小，不能把
AES 处理大文件的耗时归因于 RSA 或未来 CP-ABE 算法。本项目只用于工程学习和演示；用于
生产环境前必须接受专业密码学、密钥管理与系统安全审计。
