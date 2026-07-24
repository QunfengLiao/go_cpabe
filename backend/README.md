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
- `IMPORT_MAX_FILE_SIZE`：租户导入 Excel 文件大小上限，默认 10 MiB
- `IMPORT_MAX_ROWS`：单个导入文件最大数据行数，默认 10000
- `IMPORT_BATCH_TTL`：预校验批次有效期，默认 30 分钟
- `IMPORT_TEMP_DIR`：导入暂存目录配置，默认 `uploads/imports/.staging`
- `IMPORT_WORKER_POLL_INTERVAL`：后台导入队列轮询间隔，默认 5 秒；空队列探测不会输出 `record not found`
- `IMPORT_WORKER_LEASE`：导入 Worker 租约，默认 2 分钟，至少 10 秒
- `IMPORT_BULK_SIZE`：用户、成员、角色和组织关系的单次批量写入行数，默认 300，范围 1–1000

## 租户数据批量导入

租户管理员可在成员管理或组织管理页面下载用户/组织模板，上传 `.xlsx` 文件进行服务端预校验，
确认接口只读取不含 `rows_json` 的批次元数据，会立即把批次持久化排队并返回 HTTP 202，后台 Worker 再加载完整快照并由一个数据库事务批量写入。Worker 会将 MySQL 返回的 JSON 反序列化后重新规范化校验摘要，兼容数据库调整键顺序和空白，同时仍会拒绝字段值或结构被篡改的快照。导入批次、文件摘要、行级动作与错误会绑定当前租户和操作者，
重复确认成功批次具有幂等语义，错误报告下载会对可能被表格软件执行的公式前缀进行安全转义。

Worker 通过 MySQL 行锁、租约令牌和心跳支持多实例竞争与进程重启恢复；用户、租户成员、角色和组织关系按
`IMPORT_BULK_SIZE` 分批 UPSERT，避免万行导入退化为逐行查询链路。轮询接口不加载 `rows_json`。
Worker 领取任务时只排序并锁定批次主键，租约提交后才按主键加载完整 `rows_json`，避免万行快照进入 MySQL filesort；无需通过放大全局 `sort_buffer_size` 掩盖大字段查询问题。

用户模板包含“用户导入”“字段说明”“数据字典”三个工作表；首个工作表通过隐藏系统字段行保持
导入映射兼容，并提供中文表头、示例、冻结窗格、筛选和角色/成员状态下拉校验。

万级用户预校验会对合法新增用户的初始密码执行受限并发 bcrypt 摘要计算；明文仅在当前请求内存中
短暂存在，不进入批次快照、响应、日志或审计。超过 `IMPORT_MAX_ROWS` 时接口返回独立的行数超限错误。
预校验同时拒绝文件内重复邮箱、邮箱已属于其他用户名和软删除用户名；正式 UPSERT 后还会验证每一行
都解析到非零用户主键，任何缺失都会让整个批次回滚，禁止产生 `user_id=0` 关系。

租户成员列表使用 `page` 和 `page_size` 服务端分页，默认每页 50 条、最大 100 条；Repository 只为
当前页加载用户和角色，内部需要完整成员集时也会按 500 条窗口遍历，避免万级 `IN (...)` SQL。
迁移 `019_repair_zero_user_relations.sql` 会幂等清理历史导入产生的零用户主键关系。

接口前缀为 `/api/v1/tenant/import`，主要接口包括：

- `GET /templates/users`、`GET /templates/org-units`：下载模板
- `POST /users/validate`、`POST /org-units/validate`：上传并预校验
- `POST /users/confirm`、`POST /org-units/confirm`：确认批次并返回 HTTP 202
- `GET /batches/:batchId/status`：查询轻量进度
- `GET /batches`、`GET /batches/:batchId`、`GET /batches/:batchId/errors`：查询批次和错误报告

当前功能面向工程演示：用户可提供新用户初始密码，服务端只保存 bcrypt 摘要；未提供时生成随机密码，
明文不进入批次或日志。生产使用前仍需进行身份、权限、数据库和密码学安全审计。

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
