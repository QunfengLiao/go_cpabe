# 实现计划：CP-ABE 系统多租户基础能力

**分支**：`feat/multi-tenant-base` | **日期**：2026-07-07 | **规格**：[spec.md](./spec.md)

**输入**：来自 `specs/002-multi-tenant-base/spec.md` 的功能规格。

## 摘要

本功能为现有 CP-ABE 文件加密与访问控制系统补充共享数据库、共享表、`tenant_id` 隔离的多租户基础能力。计划新增租户、租户成员、角色和租户内用户角色关系，改造登录响应返回租户列表，新增租户切换和租户管理接口，并在现有 Access Token 登录中间件之后增加租户上下文中间件，确保后续属性、文件、策略、加解密记录和审计日志都能按当前租户过滤。

本阶段只建立组织级隔离和角色关系基础，不实现完整 RBAC 菜单权限、不实现 CP-ABE 加解密、不实现文件上传下载主链路、不实现计费配额，也不采用每租户独立数据库。

## 现状分析

1. 当前注册、登录和登录态校验：
   - `backend/internal/service/auth_service.go` 负责注册、登录、刷新和退出。
   - 登录使用邮箱和密码校验，成功后调用 `issueTokenPair` 生成 Access Token 和 Refresh Token。
   - Access Token 是 JWT，当前 Claims 包含 `user_id`、`role`、`token_type`、`exp`、`iat`、`jti`。
   - Refresh Token 对应 Redis 服务端登录态，键前缀为 `auth:refresh:`，登录态包含 `user_id`、`role`、`session_id`、`refresh_token_hash`、`issued_at`、`expires_at`、`user_agent`、`client_ip`。
   - `backend/internal/middleware/auth.go` 的 `AuthRequired` 负责校验 `Authorization: Bearer <access_token>`，并向 Gin 上下文注入 `user_id` 和 `role`。

2. 当前 `users` 表字段结构：
   - Gorm 模型位于 `backend/internal/domain/user.go`。
   - SQL 迁移位于 `backend/migrations/001_create_users.sql`。
   - 字段包括 `id`、`email`、`password_hash`、`nickname`、`avatar_url`、`avatar_object_key`、`role`、`status`、`bio`、`birthday`、`created_at`、`updated_at`、`deleted_at`。
   - 唯一索引：`uk_users_email`。
   - 普通索引：`idx_users_deleted_at`、`idx_users_role`、`idx_users_status`。
   - 当前 `role` 是用户全局角色，取值为 `admin`、`data_owner`、`data_user`，不能继续作为租户内授权依据。

3. 当前数据库迁移方式：
   - 项目已有手写 SQL 迁移目录 `backend/migrations/`，目前只有 `001_create_users.sql`。
   - 服务启动入口 `backend/cmd/server/main.go` 同时调用 `db.AutoMigrate(&domain.User{})`。
   - 管理员初始化入口 `backend/cmd/admin/main.go` 也调用 `db.AutoMigrate(&domain.User{})`。
   - 本功能计划新增 SQL 迁移脚本作为结构说明和部署依据，同时在启动与初始化命令中纳入新增 Gorm 模型的 AutoMigrate，保持当前项目开发体验。

4. 当前分层：
   - 领域模型：`backend/internal/domain`。
   - 仓储：`backend/internal/repository`。
   - 服务：`backend/internal/service`。
   - 处理器：`backend/internal/handler`。
   - 中间件：`backend/internal/middleware`。
   - 公共响应、认证、存储等工具位于 `backend/internal/pkg`。
   - 本功能应新增 Tenant 相关 domain/repository/service/handler，并少量改造 AuthService 和 UserService。

5. 当前路由和中间件：
   - `backend/internal/handler/router.go` 创建 `/api/v1` 路由组。
   - 公开接口包括 `/auth/register`、`/auth/login`、`/auth/refresh`、`/auth/logout`。
   - 当前受保护用户接口挂载在 `/api/v1/users`，使用 `middleware.AuthRequired`。
   - 本功能应复用 `AuthRequired`，在其后追加租户上下文中间件，不重复实现登录态逻辑。

6. 当前登录成功响应：
   - `backend/internal/handler/auth_handler.go` 当前返回 `access_token`、`access_token_expires_in`、`refresh_token`、`refresh_token_expires_in`、`token_type`、`user`。
   - `user` 当前来自 `domain.UserDTO`，包含 `id`、`email`、`nickname`、`role`、`avatar_url`、`bio`、`birthday`、`created_at`，不包含密码哈希和头像内部存储标识。
   - 本功能需要在保持现有 Token 字段的基础上增加 `current_tenant_id` 和 `tenants`，并逐步弱化 `user.role` 的授权意义。

7. 当前角色、菜单、权限代码：
   - 后端没有完整 RBAC、菜单或权限表。
   - 后端只有 `domain.UserRole` 字符串枚举和 JWT 中的 `role`。
   - 前端存在 `UserRole = "admin" | "data_owner" | "data_user"` 类型和角色展示文案，但没有后端菜单权限控制。
   - 本计划新增 `roles` 和 `user_roles` 数据结构，返回 `menus: []` 或预留菜单结构，但不实现复杂菜单权限。

## 技术上下文

**语言/版本**：Go 1.23；TypeScript 5.7；Node/Electron 桌面端。

**主要依赖**：Gin 1.10、Gorm 1.25、MySQL Driver、Redis Go Client、JWT v5、bcrypt；桌面端 React 19、Vite、Electron。

**存储**：MySQL 保存用户、租户、成员、角色关系；Redis 保存刷新登录态；本功能不新增文件存储。

**测试**：后端使用 Go `testing`、`httptest`、内存仓储和 `miniredis`；桌面端使用现有 TypeScript 类型检查与构建。

**目标平台**：本地开发和演示环境中的 Go 后端服务、Electron 桌面端。

**项目类型**：后端 Web API + 桌面端客户端。

**性能目标**：租户上下文校验不应明显增加登录和受保护接口延迟；MVP 验证中常规租户列表、切换租户和成员查询应在用户可感知的即时反馈范围内完成。

**约束**：
- 必须使用共享数据库、共享表、`tenant_id` 隔离。
- 必须复用现有双 Token 登录态，不重复实现认证。
- 必须保持中文规格、计划、接口说明和必要注释。
- 不得把 Tenant Admin、DO、DU 角色视为 CP-ABE 解密能力。

**规模/范围**：新增多租户基础数据模型、租户上下文、登录响应扩展、租户切换、租户管理基础接口和初始化数据；不实现完整 RBAC、文件加密主链路和 CP-ABE 算法。

## 宪章检查

*门禁：Phase 0 前必须通过，Phase 1 后再次检查。*

- **混合加密**：通过。本功能不实现文件内容加密或 DEK 封装；计划明确不触碰 AES-GCM、RSA-OAEP、CP-ABE 加密路径。
- **真实 CP-ABE**：通过。本功能不模拟 CP-ABE 加解密结果，不用角色替代属性私钥满足判断。
- **算法模块可插拔**：通过。本功能不改动 Crypto 模块；后续业务表只预留 `tenant_id` 约束。
- **RSA 是对比基线**：通过。本功能不生成 RSA 与 CP-ABE 性能结论。
- **策略表达与真实加密解耦**：通过。本功能只要求后续属性、策略表包含 `tenant_id`，不实现访问树或 LSSS。
- **系统可解释**：通过。计划覆盖租户禁用、非成员访问、角色随租户变化、跨租户拒绝等可解释错误。
- **语言与文档规范**：通过。所有计划产物使用简体中文，保留必要代码标识符和 API 路径。
- **AI 注释与可读性**：通过。实现阶段涉及认证、权限、Token、访问控制和租户隔离的代码必须添加解释安全边界的中文注释，并在实现后执行关键注释和可读性检查。
- **模块边界**：通过。租户能力规划为独立 Tenant 模块，认证模块只扩展登录响应和上下文协作，不把密码算法散落到 Handler 或 Service。

Phase 1 设计后复查：通过。数据模型、接口契约和快速验证指南均保持本阶段边界，未引入宪章禁止的算法模拟、计费配额或高级 CP-ABE 能力。

## 项目结构

### 文档结构

```text
specs/002-multi-tenant-base/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── tenant-api.md
└── checklists/
    └── requirements.md
```

### 源码结构

```text
backend/
├── cmd/
│   ├── server/main.go        # 服务启动，纳入新增模型迁移和依赖组装
│   └── admin/main.go         # 默认租户、角色、管理员初始化扩展
├── migrations/
│   ├── 001_create_users.sql
│   └── 002_create_tenants_roles.sql
├── internal/
│   ├── domain/               # Tenant、TenantUser、Role、UserRole 等领域模型
│   ├── repository/           # TenantRepository、RoleRepository，必要时扩展 UserRepository
│   ├── service/              # TenantService、AuthService 登录响应扩展
│   ├── handler/              # TenantHandler、AuthHandler 响应扩展、路由注册
│   ├── middleware/           # TenantContext 中间件，复用 AuthRequired 后的 user_id
│   └── pkg/response/         # 租户相关错误码
└── scripts/
    └── verify_auth_flow.sh   # 后续可扩展多租户验证脚本

desktop/
└── src/renderer/src/
    ├── types.ts              # 登录响应和租户类型扩展
    ├── api/auth.ts           # 登录响应解析扩展
    └── api/request.ts        # 后续请求附带 X-Tenant-Id 的位置
```

**结构决策**：后端延续现有 `domain -> repository -> service -> handler -> router/middleware` 分层。租户上下文校验属于中间件职责，租户成员与角色查询属于 repository/service 职责，Handler 只做请求绑定和响应封装。桌面端本阶段只规划类型和请求上下文协作，不要求完成完整租户选择页面。

## Phase 0 研究结论

详见 [research.md](./research.md)。关键决策如下：

- 当前租户上下文 MVP 使用 `X-Tenant-Id` 请求头，由后端在 AuthRequired 之后校验成员关系。
- Access Token 暂不写入 `tenant_id`，避免频繁切换租户导致 Token 重签和刷新会话复杂化。
- 新增 `roles` 表，使用 `scope` 区分 `platform` 与 `tenant` 角色；`user_roles.tenant_id` 对租户角色必填，平台角色可为空。
- 保留 `users.role` 作为过渡兼容字段，但租户内授权必须使用 `user_roles`。
- 新注册用户 MVP 默认加入 `default-tenant` 并按注册角色映射为 `DO` 或 `DU`；后续邀请制可在租户管理阶段增强。

## Phase 1 设计产物

- 数据模型：[data-model.md](./data-model.md)
- 接口契约：[contracts/tenant-api.md](./contracts/tenant-api.md)
- 快速验证指南：[quickstart.md](./quickstart.md)

本项目当前没有 `.specify/scripts` 下的 agent context 更新脚本；已确认无需执行该步骤。

## 一、数据模型设计计划

### `tenants`

- 作用：表示组织、实验室或公司的数据隔离边界。
- 核心字段：`id`、`name`、`code`、`status`、`description`、`created_at`、`updated_at`、`deleted_at`。
- 唯一索引：`uk_tenants_code(code)`。
- 普通索引：`idx_tenants_status(status)`、`idx_tenants_deleted_at(deleted_at)`。
- 软删除：需要。租户作为组织边界，删除应可追溯，MVP 优先使用禁用，删除只作为维护能力。

### `tenant_users`

- 作用：表示用户加入了哪些租户，以及成员关系是否启用。
- 核心字段：`id`、`tenant_id`、`user_id`、`status`、`created_at`、`updated_at`、`deleted_at`。
- 唯一索引：`uk_tenant_users_tenant_user(tenant_id, user_id)`，避免重复成员关系。
- 普通索引：`idx_tenant_users_user_id(user_id)`、`idx_tenant_users_tenant_id(tenant_id)`、`idx_tenant_users_status(status)`、`idx_tenant_users_deleted_at(deleted_at)`。
- 软删除：需要。移出租户可选择软删除或状态禁用，便于后续审计。

### `roles`

- 作用：定义系统级与租户级角色。
- 核心字段：`id`、`code`、`name`、`scope`、`description`、`created_at`、`updated_at`、`deleted_at`。
- 角色编码：`PLATFORM_ADMIN`、`TENANT_ADMIN`、`DO`、`DU`。
- `scope`：`platform` 或 `tenant`。
- 唯一索引：`uk_roles_code(code)`。
- 普通索引：`idx_roles_scope(scope)`、`idx_roles_deleted_at(deleted_at)`。
- 软删除：需要，但基础角色不应被普通业务删除。

### `user_roles`

- 作用：表示用户在某个租户下拥有哪些角色，或用户是否拥有平台级角色。
- 核心字段：`id`、`tenant_id`、`user_id`、`role_id`、`created_at`、`updated_at`、`deleted_at`。
- 关系规则：租户级角色必须有 `tenant_id`；平台级角色 `tenant_id` 可以为空。
- 唯一索引：租户级角色使用 `uk_user_roles_tenant_user_role(tenant_id, user_id, role_id)`；平台级角色需要通过实现校验或独立索引策略避免重复。
- 普通索引：`idx_user_roles_user_id(user_id)`、`idx_user_roles_tenant_id(tenant_id)`、`idx_user_roles_role_id(role_id)`、`idx_user_roles_deleted_at(deleted_at)`。
- 软删除：需要。角色变更应保留可追踪空间。

### `users`

- 调整策略：MVP 不强制删除现有 `role` 字段，避免破坏当前注册、登录、前端资料展示和测试。
- 过渡要求：`users.role` 仅用于兼容旧注册身份和初始化流程，不作为租户内授权依据。
- 后续可在 RBAC 功能中评估迁移或废弃该字段。

### 后续业务表的 `tenant_id`

以下表如果在本功能后续创建，必须从第一版开始包含 `tenant_id`：

- `attributes`：租户内可分配属性定义。
- `user_attributes`：租户内用户属性分配。
- `files`：租户内文件元数据。
- `file_policies`：租户内文件访问策略。
- `encrypt_records`：租户内加密和 DEK 封装记录。
- `decrypt_records`：租户内解密尝试记录。
- `audit_logs`：租户内审计日志。

这些表的查询、详情、更新、删除、统计都必须带 `tenant_id` 条件，并优先建立 `tenant_id` 普通索引；涉及用户或文件的组合查询应建立 `(tenant_id, user_id)`、`(tenant_id, file_id)` 等组合索引。

## 二、租户上下文设计计划

当前系统已经有 Access Token 登录态和 Redis Refresh Session。最适合本阶段的方案是：

- 登录后返回用户所属启用租户列表。
- 单租户用户自动返回 `current_tenant_id`。
- 多租户用户返回 `current_tenant_id = null` 或不设置，由前端引导选择租户。
- 租户内业务请求通过请求头 `X-Tenant-Id` 携带当前租户。
- 后端新增租户上下文中间件，在 `AuthRequired` 之后读取 `user_id` 和 `X-Tenant-Id`，校验租户启用、成员关系启用，再把 `tenant_id`、租户内角色写入 Gin 上下文。

不选择 Token 中保存 `currentTenantId` 作为 MVP 默认方案，原因是当前 Access Token 只表达身份和全局角色；多租户切换频繁时每次切换都重签 Token 会增加客户端存储、刷新 Token 轮换和旧 Token 失效策略复杂度。

不选择 Redis Session 保存当前租户作为 MVP 默认方案，原因是当前 Redis 只保存 Refresh Session，不保存每次 Access Token 请求的服务端会话；将当前租户放入请求头更符合无状态 API 的现有形态。

后续如果需要降低前端传头成本，可以在切换租户成功后签发包含 `tenant_id` 的短期 Access Token，作为第二阶段优化。

## 三、登录流程改造计划

登录服务需要从 `TenantRepository` 或 `TenantService` 查询当前用户所属的启用租户和角色。登录成功响应建议调整为：

```json
{
  "access_token": "...",
  "access_token_expires_in": 900,
  "refresh_token": "...",
  "refresh_token_expires_in": 604800,
  "token_type": "Bearer",
  "user": {
    "id": 1,
    "email": "user@example.com",
    "nickname": "测试用户",
    "role": "data_user"
  },
  "current_tenant_id": 1001,
  "tenants": [
    {
      "tenant_id": 1001,
      "tenant_name": "默认租户",
      "tenant_code": "default-tenant",
      "roles": ["TENANT_ADMIN"]
    }
  ]
}
```

计划说明：

- 保留现有 `user` 字段，降低前端改造成本。
- 新增 `current_tenant_id` 和 `tenants`。
- `user.role` 标记为兼容字段，后续租户内授权必须使用 `tenants[].roles` 或当前租户上下文角色。
- 如用户无启用租户，登录仍可成功返回 Token，但 `tenants` 为空、`current_tenant_id` 为空，前端不得进入租户内业务。

## 四、租户切换设计计划

接口：`POST /api/v1/me/switch-tenant`

请求体：

```json
{
  "tenant_id": 1001
}
```

处理流程：

1. 复用 `AuthRequired` 校验 Access Token，获得 `user_id`。
2. `TenantService.SwitchTenant` 查询目标租户是否存在且启用。
3. 校验当前用户在 `tenant_users` 中存在启用成员关系。
4. 查询当前用户在该租户下的 `user_roles`。
5. 返回当前租户、角色集合和菜单预留字段。

MVP 中切换成功不写 Redis Session、不重签 Access Token；前端保存当前租户并在后续租户内请求中携带 `X-Tenant-Id`。这样切换接口本质上是一次“可进入目标租户的验证与上下文返回”。

成功响应示例：

```json
{
  "current_tenant_id": 1001,
  "tenant": {
    "tenant_id": 1001,
    "tenant_name": "默认租户",
    "tenant_code": "default-tenant",
    "status": "enabled"
  },
  "roles": ["TENANT_ADMIN"],
  "menus": []
}
```

## 五、租户管理接口计划

沿用当前 `/api/v1` REST 风格，建议接口如下：

- `POST /api/v1/tenants`：创建租户。
- `GET /api/v1/tenants`：查询租户列表。Platform Admin 可看全部；普通用户可看自己所属租户。
- `GET /api/v1/tenants/:tenantId`：查询租户详情。
- `PATCH /api/v1/tenants/:tenantId/enable`：启用租户。
- `PATCH /api/v1/tenants/:tenantId/disable`：禁用租户。
- `POST /api/v1/tenants/:tenantId/users`：将用户加入租户，可携带角色编码。
- `DELETE /api/v1/tenants/:tenantId/users/:userId`：将用户移出租户或禁用成员关系。
- `GET /api/v1/tenants/:tenantId/users`：查询租户下用户列表。
- `GET /api/v1/me/tenants`：查询当前用户所属租户列表。
- `POST /api/v1/me/switch-tenant`：切换当前租户。

权限计划：

- `POST /api/v1/tenants`、启用和禁用租户原则上只允许 Platform Admin。
- MVP 如果 Platform Admin 只预留，则本地初始化管理员可承担演示管理能力，但计划中需标注这是过渡方案。
- 租户成员管理允许 Platform Admin 或当前租户的 Tenant Admin。

## 六、权限边界计划

- 多租户：负责组织级数据隔离，判断数据属于哪个租户、用户是否属于当前租户。
- RBAC：负责当前租户内菜单可见性和接口权限，判断用户能否调用某类管理或业务接口。
- CP-ABE：负责密文文件解密权限，判断用户属性私钥是否满足文件访问策略。

角色边界：

- Platform Admin：系统级角色，可管理所有租户；MVP 可只预留。
- Tenant Admin：只能管理当前租户内用户、角色、属性、文件元数据和审计日志，不能跨租户管理。
- DO：只能管理当前租户内自己上传的文件、策略和加密记录。
- DU：只能在当前租户内查看可访问文件、下载密文并尝试解密。
- Admin 或 Tenant Admin 不天然拥有解密所有文件的能力；是否能解密必须由 CP-ABE 属性私钥和访问策略决定。

## 七、中间件设计计划

需要新增租户上下文中间件，建议命名为 `TenantRequired` 或 `TenantContextRequired`。

职责：

1. 从请求头读取 `X-Tenant-Id`。
2. 校验请求已经通过 `AuthRequired`，能从 Gin 上下文读取 `user_id`。
3. 校验 `tenant_id` 格式合法。
4. 调用租户服务或仓储校验租户启用、用户成员关系启用。
5. 查询用户在当前租户下的角色集合。
6. 写入 Gin 上下文：`tenant_id`、`tenant_roles`、`tenant_code` 可选。
7. 后续 Handler/Service 只能从上下文获取当前租户，不信任请求体中的 `tenant_id`。

中间件组织方式：

```text
api.Group("/...", AuthRequired(authManager), TenantRequired(tenantService))
```

不重复解析 Token，不重复校验密码或刷新登录态。

## 八、数据隔离计划

所有租户内查询必须以当前租户上下文为唯一可信来源：

- 查询属性时必须带 `tenant_id`。
- 查询用户属性时必须带 `tenant_id`。
- 查询文件时必须带 `tenant_id`。
- 查询策略时必须带 `tenant_id`。
- 查询加密记录时必须带 `tenant_id`。
- 查询解密记录时必须带 `tenant_id`。
- 查询审计日志时必须带 `tenant_id`。
- 给用户分配角色时必须带 `tenant_id`。

防篡改策略：

- Handler 不接受或不信任普通用户提交的 `tenant_id` 作为数据归属来源。
- 对必须出现在路径中的 `tenantId`，必须与上下文 `tenant_id` 做一致性校验，除非调用者是 Platform Admin。
- Repository 方法命名和签名应显式包含 `tenantID`，例如 `ListFilesByTenant(ctx, tenantID, ...)`。
- 所有详情、更新、删除使用 `WHERE id = ? AND tenant_id = ?`。
- 测试必须覆盖“知道其他租户数据 ID 也无法读取”的场景。

## 九、初始化数据计划

初始化数据包括：

- 默认租户：`code = default-tenant`，`name = 默认租户`，`status = enabled`。
- 基础角色：`TENANT_ADMIN`、`DO`、`DU`。
- 预留角色：`PLATFORM_ADMIN`。
- 默认租户管理员：通过本地受控命令创建或扩展现有 `cmd/admin create` 命令。

新注册用户处理：

- MVP 建议将公开注册的 `data_owner` 自动加入 `default-tenant` 并分配 `DO`。
- MVP 建议将公开注册的 `data_user` 自动加入 `default-tenant` 并分配 `DU`。
- 公开注册不得创建 `TENANT_ADMIN` 或 `PLATFORM_ADMIN`。
- 后续企业化租户管理可改为“先创建租户，再邀请用户加入”，但这不阻塞当前演示闭环。

初始化幂等性：

- 基于 `tenants.code`、`roles.code`、`tenant_users(tenant_id,user_id)`、`user_roles(tenant_id,user_id,role_id)` 做存在性检查。
- 重复执行初始化 3 次不应产生重复数据。

## 十、开发边界

本阶段不实现：

- CP-ABE 加密算法。
- CP-ABE 解密算法。
- 文件上传完整业务。
- 文件下载完整业务。
- 复杂套餐计费。
- 每个租户一个数据库。
- 复杂按钮级权限。
- 主密钥明文展示。
- 用户私钥明文展示。
- 完整 RBAC 菜单权限系统。
- 用户撤销、多授权机构、策略隐藏和区块链审计。

## 十一、推荐实现顺序

1. 新增 `Tenant`、`TenantUser`、`Role`、`UserRoleAssignment` 领域模型和枚举常量。
2. 新增 `002_create_tenants_roles.sql` 数据库迁移脚本。
3. 新增租户、角色仓储接口和 Gorm 实现。
4. 扩展启动入口和管理员初始化入口的 AutoMigrate 模型列表。
5. 实现默认租户、基础角色、默认管理员关系的幂等初始化能力。
6. 改造注册流程：公开注册用户加入默认租户，并将旧角色映射为租户角色。
7. 改造登录流程：返回租户列表和 `current_tenant_id`。
8. 实现 `GET /api/v1/me/tenants`。
9. 实现 `POST /api/v1/me/switch-tenant`。
10. 新增租户上下文中间件，在受保护租户业务路由中注入 `tenant_id` 和当前租户角色。
11. 实现租户管理接口：创建、列表、详情、启用、禁用、加入用户、移除用户、成员列表。
12. 更新响应错误码和测试辅助内存仓储。
13. 编写服务层、处理器和中间件测试，覆盖登录租户列表、切换租户、非成员拒绝、禁用租户拒绝和角色隔离。
14. 更新桌面端类型和请求封装，让后续页面可保存并发送 `X-Tenant-Id`。
15. 执行关键注释和可读性检查，确认安全边界注释足够且不制造阅读噪音。

## 十二、验收标准

- 系统可以创建租户。
- 用户可以加入租户。
- 一个用户可以属于多个租户。
- 同一个用户在不同租户下可以拥有不同角色。
- 用户登录后可以返回所属启用租户列表。
- 单启用租户用户登录后可以返回 `current_tenant_id`。
- 用户可以切换当前租户。
- 用户不能切换到自己不属于的租户。
- 禁用租户不能被切换进入。
- 后端可以从请求上下文中获取 `currentTenantId`。
- 租户上下文中间件复用现有登录中间件，不重复实现认证。
- 后续业务查询可以基于 `currentTenantId` 做数据隔离。
- 用户不能通过篡改 `tenant_id` 或业务数据 ID 访问其他租户数据。
- 新增代码符合当前 `domain/repository/service/handler/middleware` 分层和响应封装风格。
- 所有文档、注释和计划说明使用中文，代码标识符和 API 路径保留工程规范命名。

## 复杂度跟踪

无宪章违规项。本功能新增表和中间件是多租户基础隔离的必要复杂度，且沿用现有项目分层，没有引入额外服务、独立数据库或复杂权限引擎。
