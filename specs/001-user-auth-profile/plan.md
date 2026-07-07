# 实现计划：用户认证与资料基础模块

**分支**：`feat/auth-role-base` | **日期**：2026-07-05 | **规格**：[spec.md](./spec.md)
**输入**：来自 `specs/001-user-auth-profile/spec.md` 的功能规格，以及本次 `/speckit-plan` 补充的分层、数据库、Redis、双 Token、接口、头像上传、配置和错误处理要求

## 摘要

本阶段实现 User 模块的身份基础能力：公开注册 `data_owner` 与 `data_user`，禁止公开创建 `admin`；邮箱密码登录；短期 Access Token 与长期 Refresh Token；Redis 服务端刷新登录态；刷新 Token；退出当前会话；获取当前用户；编辑个人资料；上传头像。

技术方案采用清晰分层：Handler 只处理请求绑定、基础校验和响应；Service 负责业务编排；Repository 负责 `users` 表读写；Auth 组件负责 Token 签发、解析和类型校验；Redis Token Store 负责刷新登录态；Middleware 只接受 Access Token；Storage 组件负责头像保存并预留对象存储扩展。

桌面端补充本机账号记忆能力：历史账号展示缓存只保存 `userId`、邮箱、昵称、头像和上次登录时间；刷新凭证通过独立 Token 存储适配层管理，当前为开发阶段 localStorage MVP，后续可整体替换为 Electron safeStorage、keytar 或系统凭据存储。登录页在本机有历史账号时优先展示账号选择，并在刷新失败后自动回退到邮箱预填的密码登录。

桌面端补充“记住密码 / 邮箱联想回填”能力：登录成功后由用户显式选择是否记住密码；记住后通过主进程 `credentialStore` 调用 Electron `safeStorage` 加密保存到用户数据目录，渲染层只通过 preload 暴露的最小 IPC 能力访问邮箱列表和单个邮箱凭据。该能力不复用 `cachedAccounts` 或 Token 存储，避免把密码和展示缓存、刷新凭证混在一起。

## 技术上下文

**语言/版本**：Go、TypeScript
**主要依赖**：Gin、Gorm、MySQL、Redis、bcrypt、JWT、Electron；本功能不触发 CP-ABE 库集成
**存储**：MySQL 保存用户资料；Redis 保存 Refresh Token 登录态；本地文件系统保存头像文件
**测试**：后端使用 Go 单元测试和接口测试；桌面端联调时补充 Electron/TypeScript 测试
**目标项目**：Electron 桌面端 + Go 后端的 CP-ABE 加密文件共享系统
**性能目标**：认证接口在普通开发环境中应保持用户可感知快速响应；头像上传限制 2MB；本功能不产生算法 Benchmark
**约束**：密码不得明文保存；JWT secret 不能硬编码；Refresh Token 不在 Redis 明文保存；普通业务接口拒绝 Refresh Token；刷新接口拒绝 Access Token；用户响应不得泄露 `password_hash` 与 `avatar_object_key`
**桌面端存储约束**：本机历史账号缓存不得保存密码或刷新凭证；刷新凭证暂由独立 Token 存储模块封装，生产环境必须替换为系统级安全存储
**本机凭据约束**：记住密码必须通过独立 `credentialStore` 和系统级安全存储完成；不得把密码写入 localStorage、IndexedDB、日志、普通配置文件或账号展示缓存
**规模/范围**：完成用户认证与资料基础模块；不实现文件加密、访问树、密钥管理、完整 RBAC、管理员后台和多端会话管理页面

## 宪章检查

*关卡：必须在第 0 阶段研究前通过；第 1 阶段设计后必须再次检查。*

- 混合加密：本功能不涉及文件内容加密或 DEK 封装，不会新增任何文件加密实现。
- 真实 CP-ABE：本功能不调用 CP-ABE，不会用用户角色或登录态模拟 CP-ABE 结果。
- 可插拔算法：本功能不引入 `CryptoEngine`；认证模块与后续 Crypto 模块保持解耦。
- 公平基准：本功能不输出 RSA 与 CP-ABE 对比结论，不混入算法 Benchmark。
- 策略解耦：本功能不解析访问树或 LSSS，角色仅作为用户身份基础，不替代后续属性策略。
- 可解释性：登录失败、凭证过期、凭证混用、资料校验失败、头像校验失败均使用统一错误结构表达原因。
- 模块边界：User 模块内部按 Handler、Service、Repository、Auth、Token Store、Middleware、Storage 分层，避免 Handler 写数据库或复杂业务。
- 范围纪律：本阶段只交付用户认证与资料基础能力，不提前实现高级 CP-ABE、密钥管理、完整 RBAC 或对象存储。
- 语言规范：本计划以及 `research.md`、`data-model.md`、`quickstart.md`、`contracts/` 文档均使用简体中文。
- 本机账号记忆：该增强只复用现有刷新接口和前端状态管理，不新增后端业务逻辑；refreshToken 不混入展示缓存，避免账号列表泄露敏感凭证。
- 本机凭据回填：该增强只影响桌面端本机体验，不改变服务端认证流程；密码只在用户明确同意后通过 Electron `safeStorage` 加密保存，无法安全存储时不得降级为明文保存。

**初始关卡结果**：通过，无宪章偏离。

## 项目结构

```text
backend/
├── cmd/server/                 # 后端启动入口
├── internal/config/            # 配置读取与校验
├── internal/domain/            # 领域实体、角色常量、状态常量、Token 常量
├── internal/repository/        # users 表 Repository / DAO
├── internal/service/           # 用户认证、资料、头像上传业务编排
├── internal/handler/           # Gin Handler，请求绑定、基础校验、响应
├── internal/middleware/        # Access Token 认证中间件
├── internal/pkg/auth/          # JWT、Refresh Token、Hash、session_id/token_id
├── internal/pkg/storage/       # Storage 接口与 LocalStorage 实现
├── internal/pkg/response/      # 统一响应和错误码
├── internal/pkg/validator/     # 可复用参数校验辅助
└── migrations/                 # 手写 SQL migration，必要时与 AutoMigrate 配合

desktop/                        # Electron + TypeScript 桌面端
specs/001-user-auth-profile/    # 当前功能规格、计划和设计产物
```

### 目录职责说明

- `internal/domain`：放 `User` 领域模型、`UserRole`、`UserStatus`、`TokenType` 等常量和值校验方法。
- `internal/repository`：封装用户查询与更新，包括按邮箱查询、按 ID 查询、创建用户、更新资料、更新头像。
- `internal/service`：编排注册、登录、刷新、退出、当前用户、资料编辑、头像上传流程。
- `internal/handler`：定义请求结构、绑定参数、调用 Service、返回统一响应，不直接访问数据库或 Redis。
- `internal/middleware`：解析 `Authorization: Bearer <access_token>`，校验 `token_type=access`，注入 `user_id` 和 `role`。
- `internal/pkg/auth`：负责 JWT Claims、Access Token 生成/解析、Refresh Token 随机值生成、Token Hash、`jti`、`token_id`、`session_id`。
- `internal/pkg/storage`：定义 `Storage` 接口，本阶段实现 `LocalStorage`，后续扩展 `MinIOStorage`、`COSStorage`、`OSSStorage`。
- `internal/pkg/response`：统一成功响应、错误响应、错误码与可展示消息。
- `internal/config`：读取 MySQL、Redis、JWT、Token 过期时间、头像目录、头像 URL 前缀和最大头像大小。

## 第 0 阶段：研究

已生成：[research.md](./research.md)

研究结论：

- Access Token 使用 JWT，Refresh Token 使用高熵随机字符串。
- Redis Key 使用 `auth:refresh:{token_id}`，Value 使用 JSON 保存会话数据和 `refresh_token_hash`。
- Refresh Token 采用轮换机制作为本阶段目标；如果实现时遇到时间风险，可保留 Token Store 接口并先落非轮换版本，但任务应优先安排轮换。
- 用户表采用 Gorm Model，同时提供手写 migration SQL；开发环境可使用 AutoMigrate，受控环境以 SQL migration 为准。
- 头像保存采用 Storage 接口 + `LocalStorage`，目录为 `uploads/avatars`，文件名使用用户 ID、时间戳和随机串避免冲突。

## 第 1 阶段：设计

已生成：

- [data-model.md](./data-model.md)
- [contracts/auth-users-api.md](./contracts/auth-users-api.md)
- [quickstart.md](./quickstart.md)

### 数据库设计

- `users` 表包含 `id`、`email`、`password_hash`、`nickname`、`avatar_url`、`avatar_object_key`、`role`、`status`、`bio`、`birthday`、`created_at`、`updated_at`、`deleted_at`。
- `email` 建唯一索引。
- `password_hash`、`nickname`、`role`、`status` 非空。
- `role` 使用字符串枚举：`admin`、`data_owner`、`data_user`。
- `status` 使用字符串枚举：`active`、`disabled`，默认 `active`。
- `bio`、`birthday`、`avatar_url`、`avatar_object_key` 可为空。
- 对外统一使用 `UserDTO`，只包含用户可见字段，避免泄露 `password_hash` 和 `avatar_object_key`。

### Redis Refresh Token 登录态

- Key：`auth:refresh:{token_id}`。
- Value：JSON，包含 `user_id`、`role`、`session_id`、`refresh_token_hash`、`issued_at`、`expires_at`、可选 `user_agent`、可选 `client_ip`。
- TTL：与 Refresh Token 有效期一致。
- 刷新：默认轮换，校验旧 Token 后删除旧 Key，生成新 Refresh Token 和新 Key。
- 退出：根据 Refresh Token 定位 `token_id`，校验 Hash 后删除当前 Key。

### 双 Token 设计

- Access Token：JWT，默认 15 分钟，Claims 包含 `user_id`、`role`、`token_type=access`、`exp`、`iat`、`jti`。
- Refresh Token：随机字符串，默认 7 天，返回前端；Redis 只保存 Hash。
- 配置项：`JWT_SECRET`、`ACCESS_TOKEN_TTL`、`REFRESH_TOKEN_TTL` 必须来自配置或环境变量，不能硬编码。

### 接口设计

接口契约详见 [contracts/auth-users-api.md](./contracts/auth-users-api.md)，覆盖：

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/users/me`
- `PUT /api/v1/users/me`
- `POST /api/v1/users/me/avatar`

### 错误处理

- 统一响应结构，包含 `code`、`message`、`data`、`request_id`。
- 登录失败统一返回“邮箱或密码错误”。
- 内部错误只记录日志，不把底层数据库、Redis、文件系统或 JWT 错误细节直接返回前端。
- 错误码覆盖参数错误、邮箱错误、重复邮箱、密码不一致、角色非法、禁止注册 admin、用户禁用、Token 缺失/无效/过期、Refresh Token Redis 缺失/Hash 不匹配、头像文件错误和保存失败。

### 宪章复查

- 设计产物未引入文件加密、DEK 封装、CP-ABE 模拟或 Benchmark 结论。
- User 模块边界清晰，Handler、Service、Repository、Auth、Token Store、Middleware、Storage 职责分离。
- 所有 SpecKit 产物使用简体中文。

**设计后关卡结果**：通过，无宪章偏离。

### Agent 上下文更新

仓库未提供 `.specify/scripts/bash/update-agent-context.sh` 或等价脚本，因此本阶段未执行自动上下文更新。当前 `AGENTS.md` 已包含项目宪章、技术栈、模块边界、密码学约束和中文文档规范，后续任务可直接依据本计划继续。

## 第 2 阶段：任务规划

由后续 `/speckit-tasks` 工作流生成。建议任务按以下顺序拆分：

1. 配置、统一响应和错误码基础设施。
2. 用户领域模型、角色/状态常量、Gorm Model 与 migration。
3. Repository 和 DTO 映射。
4. Auth 组件与 Access Token / Refresh Token 能力。
5. Redis Token Store 与刷新轮换。
6. Service 注册、登录、刷新、退出、当前用户、资料编辑。
7. Middleware 认证上下文。
8. Storage 接口与本地头像存储。
9. Handler 与路由注册。
10. 单元测试、接口测试和 quickstart 验证。

## 复杂度跟踪

无宪章偏离，无需复杂度豁免。
