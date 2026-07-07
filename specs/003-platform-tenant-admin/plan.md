# 实现计划：Platform Admin 平台管理员租户管理能力

**分支**：`feat/platform-tenant-admin` | **日期**：2026-07-07 | **规格**：[spec.md](./spec.md)

**输入**：来自 `specs/003-platform-tenant-admin/spec.md` 的功能规格。

## 摘要

本功能在已有用户注册登录和多租户基础能力之上，补齐真正的平台级租户治理能力：平台管理员 `PLATFORM_ADMIN` 可以管理租户生命周期、租户用户关系、租户管理员分配，并可查看平台控制台统计；租户管理员 `TENANT_ADMIN`、数据拥有者 `DO`、数据使用者 `DU` 继续只在当前租户内生效。

实现方向是延续现有 Go + Gin + Gorm 后端分层和 Electron + React 前端结构，但把平台后台接口从当前混合的 `/api/v1/tenants` 管理入口中拆到 `/api/v1/platform/...`，由新的平台权限中间件严格校验 `user_roles.tenant_id IS NULL + PLATFORM_ADMIN`。现有 `/api/v1/me/tenants` 和 `/api/v1/me/switch-tenant` 继续服务普通用户的租户上下文选择。

本阶段不实现 CP-ABE 加密解密、文件上传下载、访问树可视化、租户内完整 RBAC 菜单、租户内属性授权、主密钥明文展示、用户私钥明文展示，也不允许 Platform Admin 绕过策略解密文件。

## 现状分析

### 1. 用户登录注册逻辑

- `backend/internal/service/auth_service.go` 负责注册、登录、刷新和退出。
- 注册仍使用旧全局 `users.role`，公开注册只允许 `data_owner` 和 `data_user`，禁止公开注册 `admin`。
- 登录通过邮箱、密码和可选 `tenantCode` 完成校验，成功后返回 Access Token、Refresh Token、`user`、`current_tenant_id`、`current_tenant_code` 和 `tenants`。
- Access Token 仍只表达登录身份和旧全局 `UserRole`，不包含平台角色或当前租户角色。
- 登录响应中的 `tenants[].roles` 已能返回租户级角色，但前端目前没有稳定暴露平台级角色判断。
- 计划：登录响应增加 `platform_roles` 或等价字段，至少包含 `PLATFORM_ADMIN`，用于前端判断是否展示平台后台入口；后端授权仍以服务端中间件查询为准，不能信任前端字段。

### 2. 多租户基础能力

- `backend/internal/domain/tenant.go` 已定义 `Tenant`、`TenantUser`、`Role`、`UserRoleAssignment` 和 DTO。
- `backend/internal/service/tenant_service.go` 已有租户上下文、租户切换、创建租户、查询租户、启用/禁用、成员增删和成员列表能力。
- `backend/internal/middleware/tenant.go` 已有 `TenantRequired`，基于 `X-Tenant-Id` 校验租户存在、租户启用、成员关系有效，并注入 `tenant_id`、`tenant_code`、`tenant_roles`。
- 当前 `TenantService.ensurePlatformOrLegacyAdmin` 允许 `PLATFORM_ADMIN` 或旧 `users.role = admin` 访问部分管理能力；这不符合本功能“平台接口只允许 `PLATFORM_ADMIN`”的目标。
- 计划：平台管理走独立 `PlatformTenantService` / 平台中间件；旧 `users.role = admin` 不再作为平台接口授权依据，只可作为迁移初始化辅助。

### 3. `tenants` 和 `tenant_users` 表结构

- `backend/migrations/002_create_tenants_roles.sql` 已存在 `tenants` 和 `tenant_users`。
- `tenants` 包含 `id`、`name`、`code`、`status`、`description`、`created_at`、`updated_at`、`deleted_at`。
- `tenants.code` 已有唯一索引 `uk_tenants_code`，`status` 和 `deleted_at` 已有普通索引。
- `tenant_users` 包含 `tenant_id`、`user_id`、`status`、时间字段和软删除字段。
- `tenant_users` 已有唯一索引 `uk_tenant_users_tenant_user(tenant_id, user_id)`，可防止重复加入同一租户。
- 计划：结构基本可复用；需要补充 code 格式校验、禁用租户新增成员保护、移除最后一个 Tenant Admin 保护。

### 4. `roles` 和 `user_roles`

- `roles` 表已存在，包含 `code`、`name`、`scope`、`description`。
- `user_roles` 表已存在，`tenant_id` 允许为空，适合表达平台级角色。
- 当前唯一索引为 `uk_user_roles_tenant_user_role(tenant_id, user_id, role_id)`。
- 重要风险：MySQL 唯一索引中 `NULL` 不按相等处理，多个 `(NULL, same_user, same_role)` 可能重复。平台级角色去重不能只依赖该唯一索引。
- 计划：实现层必须先查后写平台级角色；可选新增生成列或独立唯一索引策略来约束平台级角色重复。为保持本阶段改动克制，优先在仓储层提供 `EnsurePlatformRole` 幂等方法，并在计划/任务中注明数据库层加强项。

### 5. 权限中间件

- 现有 `AuthRequired` 已完成 Access Token 校验并注入 `user_id`。
- 现有 `TenantRequired` 已完成租户上下文校验。
- 当前没有专门的 `PlatformAdminRequired`。
- 计划：新增平台权限中间件，必须在 `AuthRequired` 后执行，读取 `user_id`，查询 `user_roles` + `roles` 判断是否存在 `tenant_id IS NULL` 且 `roles.code = PLATFORM_ADMIN`。该中间件不读取 `X-Tenant-Id`，不依赖当前租户。

### 6. 路由分组

- 当前 `backend/internal/handler/router.go` 下存在 `/api/v1` 分组。
- 当前租户管理接口挂在 `/api/v1/tenants`，且只使用 `AuthRequired`。
- 当前用户租户接口挂在 `/api/v1/me/tenants` 和 `/api/v1/me/switch-tenant`。
- 计划：新增 `/api/v1/platform` 路由组，挂载 `AuthRequired + PlatformAdminRequired`。平台租户接口使用 `/api/v1/platform/tenants...`，平台统计使用 `/api/v1/platform/dashboard`。保留 `/api/v1/me/...` 作为普通当前用户上下文接口。

### 7. 审计日志

- 当前代码中没有 `audit_logs` 或平台审计表实现。
- 规格允许平台审计本阶段预留。
- 计划：本阶段不强制落表实现平台审计，先在接口和 service 设计中预留 `AuditRecorder` 接口或 TODO 任务；后续 Audit 模块统一实现时再记录创建租户、启用/禁用、加入/移出用户、分配/移除 Tenant Admin。

### 8. 前端多租户登录页和租户状态

- `desktop/src/renderer/src/config/tenantLoginConfigs.ts` 已配置 `scnu`、`sangfor`、`aia-hk` 三个租户登录入口。
- `tenantStartup.ts` 已支持 `VITE_DEFAULT_TENANT_CODE`，启动时能清理旧登录态并跳转到指定租户登录页。
- `authStorage.ts` 已保存 `current_tenant_id`、`current_tenant_code`、`last_tenant_code`、`tenants`。
- `request.ts` 会自动携带 `Authorization` 和当前 `X-Tenant-Id`。
- 计划：扩展登录态保存 `platform_roles`，让 `AuthContext` 暴露 `isPlatformAdmin`、`tenants`、`currentTenantId`、`currentTenantCode`。

### 9. 前端菜单和页面布局

- `AppLayout.tsx` 已有侧边栏、账号切换和预留模块入口。
- 当前没有平台后台菜单、租户列表页、租户详情页、用户租户关系页。
- 当前路由只有登录、注册、租户选择、个人资料。
- 计划：在单一 `AppLayout` 内按角色生成菜单，不复制多套布局。Platform Admin 登录后默认进入 `/platform`，显示平台控制台和平台管理菜单；非 Platform Admin 不显示平台管理入口。

## 技术上下文

**语言/版本**：Go 1.23；TypeScript 5.7；React 19；Electron 33；Vite 6。

**主要依赖**：Gin、Gorm、MySQL Driver、Redis Go Client、JWT v5、bcrypt；前端 React Router、Electron、Vite。

**存储**：MySQL 保存用户、租户、成员、角色关系；Redis 保存 Refresh Token 登录态；本功能不新增文件存储和密钥存储。

**测试**：后端使用 Go `testing`、`httptest`、内存仓储和现有测试辅助；前端使用 TypeScript typecheck 和构建校验，必要时补充页面级手动验收。

**目标平台**：本地开发和演示环境中的 Go Web API + Electron 桌面端。

**项目类型**：后端 Web API + 桌面客户端。

**性能目标**：平台后台常规列表和详情操作在演示规模下应即时响应；平台权限校验不应明显增加接口延迟。

**约束**：

- 平台接口只允许 `PLATFORM_ADMIN`，不能继续依赖旧 `users.role = admin`。
- 平台接口不依赖 `currentTenantId` 或 `X-Tenant-Id`。
- 租户内业务接口继续依赖 `TenantRequired` 和当前租户上下文。
- 文档、注释和规格说明使用简体中文。
- 不触碰 CP-ABE、文件、密钥明文、策略解密等非目标能力。

**规模/范围**：补齐平台租户管理后端接口、角色初始化、平台权限中间件、前端平台后台菜单和租户管理页面；平台审计只预留，平台控制台统计可做轻量实现。

## 宪章检查

*门禁：Phase 0 前必须通过，Phase 1 后再次检查。*

- **混合加密**：通过。本功能不实现文件内容加密或 DEK 封装，不改变 AES-GCM、RSA-OAEP、CP-ABE 边界。
- **真实 CP-ABE**：通过。本功能不实现或模拟 CP-ABE，加密解密能力保持不触碰。
- **算法模块可插拔**：通过。平台租户管理不进入 Crypto 模块，不在 Handler 或 Service 中散落密码算法逻辑。
- **RSA 是对比基线**：通过。本功能不记录算法耗时，不输出 RSA 与 CP-ABE 性能结论。
- **策略表达与真实加解密**：通过。本功能只区分平台/租户角色，不用角色替代 CP-ABE 策略满足判断。
- **系统可解释**：通过。计划明确 Platform Admin 只能管理平台租户，不能解密所有租户文件。
- **语言与文档规范**：通过。所有计划产物使用简体中文，保留必要代码标识符和 API 路径。
- **AI 注释与可读性**：通过。后续实现阶段涉及认证、权限、Token、租户隔离时必须添加解释安全边界的中文注释，并完成关键注释和可读性检查。
- **模块边界**：通过。平台治理逻辑放在 platform/tenant 相关 service、中间件、handler，审计只预留，不污染 Crypto/File/Policy。

Phase 1 设计后复查：通过。数据模型、接口契约和快速验证指南均保持本阶段边界，没有引入宪章禁止的算法模拟、密钥明文暴露或高级 CP-ABE 扩展。

## 项目结构

### 文档结构

```text
specs/003-platform-tenant-admin/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── platform-api.md
└── checklists/
    └── requirements.md
```

### 源码结构

```text
backend/
├── cmd/
│   ├── server/main.go        # 组装平台 service、中间件和 handler
│   └── admin/main.go         # 扩展平台管理员分配或初始化能力
├── migrations/
│   ├── 001_create_users.sql
│   └── 002_create_tenants_roles.sql
├── internal/
│   ├── domain/               # 复用 Tenant、TenantUser、Role、UserRoleAssignment，扩展 DTO
│   ├── repository/           # 扩展 TenantRepository/UserRepository 平台查询和角色写入方法
│   ├── service/              # 新增 PlatformTenantService、PlatformRoleService 或拆分相关服务
│   ├── handler/              # 新增 PlatformHandler 或 PlatformTenantHandler
│   ├── middleware/           # 新增 PlatformAdminRequired
│   └── pkg/response/         # 新增平台权限和校验错误码

desktop/
└── src/renderer/src/
    ├── api/                  # 新增 platformApi.ts，复用 request.ts
    ├── auth/                 # 扩展 AuthContext 暴露平台角色和租户状态
    ├── components/           # 扩展 AppLayout 菜单驱动
    ├── pages/                # 新增平台控制台、租户列表、租户详情、租户用户关系页面
    └── types.ts              # 扩展平台角色、租户详情、平台统计、成员 DTO 类型
```

**结构决策**：沿用当前 `domain -> repository -> service -> handler -> router/middleware` 后端分层；平台后台接口独立路由组隔离授权语义。前端沿用单一 Electron/React 应用和单一登录页，按角色驱动菜单与页面，不复制多套租户页面。

## Phase 0 研究结论

详见 [research.md](./research.md)。关键决策如下：

- 平台后台接口使用 `/api/v1/platform` 路由组，与 `/api/v1/me` 和租户内业务分离。
- 平台授权新增 `PlatformAdminRequired`，只认 `PLATFORM_ADMIN` 平台级角色，不再将旧 `users.role = admin` 当作授权依据。
- `roles.scope` 和 `user_roles.tenant_id` 结构基本可复用；平台级角色 `tenant_id = NULL`，租户级角色 `tenant_id` 必填。
- `user_roles` 平台级重复分配先由仓储层幂等保护，后续可用数据库生成列或唯一索引加强。
- 初始化数据必须包括四个基础角色、三个演示租户，以及受控创建或分配平台管理员的命令。
- 平台审计本阶段预留设计，不强制建表。

## Phase 1 设计产物

- 数据模型：[data-model.md](./data-model.md)
- 接口契约：[contracts/platform-api.md](./contracts/platform-api.md)
- 快速验证指南：[quickstart.md](./quickstart.md)

## 一、角色模型改造计划

### 表结构判断

- `roles.scope` 已存在，不需要新增字段。
- `PLATFORM_ADMIN` 作为平台级角色，`scope = platform`。
- `TENANT_ADMIN`、`DO`、`DU` 作为租户级角色，`scope = tenant`。
- `user_roles.tenant_id` 已允许为空，适合表达平台级角色。

### 角色关系规则

- 平台级角色：`user_roles.tenant_id` 必须为空。
- 租户级角色：`user_roles.tenant_id` 必须不为空。
- 分配租户级角色前必须确认用户属于目标租户。
- 分配 `TENANT_ADMIN` 时不能影响用户在其他租户的角色。
- 平台级 `PLATFORM_ADMIN` 不自动拥有任意租户内 `TENANT_ADMIN`、`DO`、`DU`，也不拥有文件解密能力。

### 唯一性和索引

- 租户用户关系已由 `uk_tenant_users_tenant_user(tenant_id, user_id)` 防重复。
- 租户级角色可继续使用 `uk_user_roles_tenant_user_role(tenant_id, user_id, role_id)` 防重复。
- 平台级角色因为 `tenant_id = NULL`，MySQL 唯一索引不能完全防重复；计划新增仓储层幂等方法：
  - `EnsurePlatformRole(ctx, userID, roleCode)`
  - `RemovePlatformRole(ctx, userID, roleCode)` 可选
  - 写入前先查询 `tenant_id IS NULL` 的同角色关系。
- 后续数据库加强可选方案：
  - 增加生成列 `tenant_scope_key = COALESCE(tenant_id, 0)` 并建立唯一索引。
  - 或拆分平台角色关系表。当前阶段不优先采用，避免扩大迁移范围。

## 二、初始化数据计划

初始化应集中在 `TenantService.EnsureBaseRoles` 或新的 `BootstrapPlatformData` 中，并保证重复执行幂等。

### 基础角色

- `PLATFORM_ADMIN`：平台管理员，`scope = platform`。
- `TENANT_ADMIN`：租户管理员，`scope = tenant`。
- `DO`：数据拥有者，`scope = tenant`。
- `DU`：数据使用者，`scope = tenant`。

### 演示租户

- 四川师范大学：`code = scnu`，`status = enabled`。
- 深信服科技：`code = sangfor`，`status = enabled`。
- 香港友邦保险：`code = aia-hk`，`status = enabled`。
- 保留已有 `default-tenant` 作为历史兼容和默认演示租户，但平台管理不应默认跳回该租户。

### 平台管理员账号

推荐扩展 `backend/cmd/admin/main.go`：

- `go run ./cmd/admin create-platform -email platform@example.com -password ... -nickname 平台管理员`
- 若用户不存在，则创建普通用户记录并分配 `PLATFORM_ADMIN`。
- 若用户已存在，则只补充分配 `PLATFORM_ADMIN`，不覆盖密码和用户资料。
- 公开注册仍禁止创建平台管理员。

幂等要求：

- 角色按 `roles.code` 幂等创建。
- 租户按 `tenants.code` 幂等创建。
- 平台管理员角色按 `user_id + role_id + tenant_id IS NULL` 幂等写入。
- 多次执行初始化不能产生重复租户、重复角色、重复平台角色。

## 三、平台权限中间件计划

新增 `middleware.PlatformAdminRequired`。

执行顺序：

```text
AuthRequired(authManager) -> PlatformAdminRequired(platformRoleResolver)
```

职责：

- 从 Gin 上下文读取 `user_id`。
- 查询 `user_roles` 联合 `roles`，校验 `tenant_id IS NULL` 且 `roles.code = PLATFORM_ADMIN`。
- 不读取 `X-Tenant-Id`。
- 不调用 `TenantRequired`。
- 不接受旧 `users.role = admin` 作为平台授权。
- 非平台管理员返回 `403`，建议错误码 `PLATFORM_PERMISSION_DENIED` 或复用并明确化 `TENANT_PERMISSION_DENIED`。推荐新增错误码，避免语义混乱。

设计原因：

- 登录认证和平台授权分离，避免重复解析 token。
- 平台级权限和租户级权限分离，避免 Tenant Admin 借当前租户上下文访问平台管理接口。
- 授权以服务端数据库角色关系为准，前端菜单隐藏只做体验优化，不做安全边界。

## 四、后端接口计划

沿用当前 `/api/v1` 风格，新增平台后台分组：

```text
/api/v1/platform
```

接口规划：

- `GET /api/v1/platform/tenants`：查询租户列表。
- `POST /api/v1/platform/tenants`：创建租户。
- `GET /api/v1/platform/tenants/:id`：查询租户详情。
- `PATCH /api/v1/platform/tenants/:id/enable`：启用租户。
- `PATCH /api/v1/platform/tenants/:id/disable`：禁用租户。
- `GET /api/v1/platform/tenants/:id/users`：查询租户下用户列表。
- `POST /api/v1/platform/tenants/:id/users`：将用户加入租户。
- `DELETE /api/v1/platform/tenants/:id/users/:userId`：将用户移出租户。
- `POST /api/v1/platform/tenants/:id/admins`：给租户分配 Tenant Admin。
- `DELETE /api/v1/platform/tenants/:id/admins/:userId`：移除租户 Tenant Admin。
- `GET /api/v1/platform/dashboard`：平台控制台统计，可实现轻量统计，也可先返回预留结构。
- `GET /api/v1/platform/users`：平台用户列表，前端平台用户管理需要；可作为 P2。
- `GET /api/v1/platform/users/:userId/tenants`：用户租户关系，前端平台用户管理需要；可作为 P2。

保留接口：

- `GET /api/v1/me/tenants`：当前用户所属启用租户列表。
- `POST /api/v1/me/switch-tenant`：当前用户切换租户。

迁移建议：

- 当前 `/api/v1/tenants` 已混合“平台”和“当前用户视角”。新功能应把平台能力迁移到 `/api/v1/platform/tenants`。
- `/api/v1/tenants` 可以暂时保留兼容，但不应继续新增平台管理能力；后续可标记为废弃或限制为租户内管理视角。

## 五、Service 设计计划

不要把所有逻辑堆到 handler。建议拆分如下：

- `PlatformTenantService`
  - 查询所有租户、创建租户、租户详情、启用/禁用。
  - 负责租户 code 规范化和格式校验。
  - 负责禁用租户不删除历史数据的状态变更。

- `PlatformTenantUserService`
  - 查询租户用户列表。
  - 将用户加入租户、移出租户。
  - 校验用户存在、租户存在、租户状态、重复关系。
  - 检查是否会移除最后一个 Tenant Admin。

- `PlatformRoleService`
  - 分配和移除平台级 `PLATFORM_ADMIN`，供初始化命令或后续管理入口使用。
  - 分配和移除租户级 `TENANT_ADMIN`。
  - 封装 `tenant_id IS NULL` 和租户级 `tenant_id 必填` 的规则。

- `PlatformDashboardService`
  - 汇总租户总数、启用租户数、禁用租户数、平台用户总数、租户用户关系数量、Tenant Admin 数量。
  - 可第一阶段实现，或先返回结构化空值并在任务中标为 P3。

- `TenantService`
  - 保留当前用户租户上下文、登录时租户列表、租户切换、`TenantRequired` 所需解析能力。
  - 不再承担平台后台全部治理逻辑。

- `AuthService`
  - 仅扩展登录响应中的平台角色信息。
  - 不在认证服务里实现平台租户管理。

## 六、数据校验计划

创建租户：

- `name` 不能为空。
- `code` 不能为空。
- `code` 全局唯一。
- `code` 只允许小写字母、数字、中划线，建议正则 `^[a-z0-9]+(?:-[a-z0-9]+)*$`。
- `status` 为空时默认 `enabled`，非 `enabled/disabled` 拒绝。
- 重复 code 返回 `TENANT_CODE_EXISTS`。

租户状态：

- 启用/禁用前校验租户存在。
- 禁用不删除历史数据。
- 禁用后 `TenantContextForUser`、`SwitchTenant`、`TenantRequired` 必须继续拒绝普通用户进入。

租户用户关系：

- 用户必须存在。
- 租户必须存在。
- 禁用租户默认拒绝新增成员。
- 同一用户不能重复加入同一租户；重复加入应幂等返回现有关系或明确提示。
- 移出用户后成员状态应变为 disabled 或软删除，但不能继续进入租户。
- 移出用户前检查是否会移除最后一个有效 `TENANT_ADMIN`。

Tenant Admin 分配：

- 被分配用户必须存在。
- 租户必须存在。
- 用户必须属于目标租户且成员状态 active。
- 同一用户在同一租户下不能重复分配 `TENANT_ADMIN`。
- 移除 `TENANT_ADMIN` 前检查是否会移除最后一个有效租户管理员。

平台权限：

- 平台接口必须经过 `PlatformAdminRequired`。
- 非 `PLATFORM_ADMIN` 访问返回 403。
- 平台接口不能因携带旧 `X-Tenant-Id` 或旧登录态而改变授权结果。

## 七、前端实现计划

### 登录后角色识别

- 扩展 `LoginData`：增加 `platform_roles?: TenantRole[]` 或 `roles?: TenantRole[]` 中的平台角色字段。
- 扩展 `AuthStateSnapshot` 和 `AuthContext`：
  - `tenants`
  - `currentTenantId`
  - `currentTenantCode`
  - `platformRoles`
  - `isPlatformAdmin`
- `authStorage.ts` 持久化平台角色，但敏感 token 仍按现有逻辑存储，不新增密码或密钥明文保存。

### 路由计划

- `/platform`：平台控制台。
- `/platform/tenants`：租户列表。
- `/platform/tenants/new`：创建租户。
- `/platform/tenants/:tenantId`：租户详情。
- `/platform/tenants/:tenantId/users`：租户用户和 Tenant Admin 分配。
- `/platform/users`：平台用户列表，可 P2。
- `/platform/audit`：平台审计预留页，可 P3。

### 菜单计划

Platform Admin 可见：

- 平台首页
  - 平台控制台
- 租户管理
  - 租户列表
  - 创建租户
  - 租户详情
  - 启用 / 禁用租户
  - 租户管理员分配
- 平台用户管理
  - 用户列表
  - 用户租户关系
- 平台审计
  - 平台操作日志，可预留

非 Platform Admin：

- 不显示平台管理菜单。
- 保留当前个人资料、租户选择、后续租户内模块入口。
- 即使手动输入 `/platform` 路由，也应由前端拦截显示无权限；真正安全边界仍由后端 403 保证。

### 页面设计原则

- 不复制多套登录页，继续使用 `LoginPage + tenantLoginConfigs`。
- 不复制多套平台页面，租户列表和详情使用配置和数据驱动。
- 使用现有 `AppLayout` 统一布局，增加菜单配置数组。
- 风格保持现代、清爽、专业，偏管理后台密集信息布局，不做营销式大幅 hero。
- 平台管理操作按钮清晰：创建、启用、禁用、加入用户、移出用户、分配管理员、移除管理员。

## 八、平台控制台计划

建议本阶段实现轻量统计接口，因为统计项都能从现有表得出：

- 租户总数：`tenants`。
- 启用租户数：`tenants.status = enabled`。
- 禁用租户数：`tenants.status = disabled`。
- 平台用户总数：`users`。
- 租户用户关系数量：`tenant_users`。
- Tenant Admin 数量：`user_roles` 关联 `roles.code = TENANT_ADMIN`。

如果实现成本影响主链路，则后端先返回固定结构和 `implemented: false`，前端保留控制台卡片位置并显示暂无统计。

## 九、审计日志计划

当前项目没有审计日志表，本阶段不强制落地完整平台审计。

预留事件：

- 创建租户。
- 启用租户。
- 禁用租户。
- 加入租户用户。
- 移出租户用户。
- 分配 Tenant Admin。
- 移除 Tenant Admin。

设计建议：

- 先定义 `AuditRecorder` 接口或 service 层调用点，默认 no-op。
- 后续 Audit 模块实现时再落表，避免本阶段临时建一个不可复用的审计结构。
- 计划和任务中必须明确平台审计是预留，不影响主链路验收。

## 十、开发边界

本阶段只做 Platform Admin 平台租户管理。

不实现：

- CP-ABE 加密算法。
- CP-ABE 解密算法。
- 文件上传。
- 文件下载。
- 访问树可视化。
- 租户内完整 RBAC 菜单权限。
- 租户内属性授权。
- 主密钥明文展示。
- 用户私钥明文展示。
- Platform Admin 绕过策略解密文件。

## 十一、推荐实现顺序

1. 检查和调整角色模型，确认 `roles.scope`、`user_roles.tenant_id` 和常量语义。
2. 补充初始化数据：`PLATFORM_ADMIN`、`TENANT_ADMIN`、`DO`、`DU`、`scnu`、`sangfor`、`aia-hk`。
3. 扩展管理命令，支持给测试账号创建或分配 `PLATFORM_ADMIN`。
4. 实现 `PlatformAdminRequired` 权限中间件。
5. 新增 `/api/v1/platform` 路由组。
6. 实现平台租户管理后端接口：列表、创建、详情、启用、禁用。
7. 实现租户用户关系管理接口：列表、加入、移出。
8. 实现租户管理员分配接口：分配和移除 `TENANT_ADMIN`。
9. 实现平台控制台统计接口，可选但推荐。
10. 扩展登录响应和前端 AuthContext，支持 `isPlatformAdmin`。
11. 实现前端平台管理菜单和 `/platform` 路由守卫。
12. 实现平台控制台、租户列表、创建租户、租户详情页面。
13. 实现租户用户和租户管理员分配页面。
14. 补充后端 service、handler、中间件测试。
15. 运行后端 `go test ./...`、前端 `npm run typecheck` 和 `npm run build`。
16. 完成关键注释和可读性检查，确认认证、权限、Token、租户隔离相关注释解释安全边界。

## 十二、验收标准

- `PLATFORM_ADMIN` 可以登录并进入平台后台。
- `PLATFORM_ADMIN` 可以看到平台管理菜单。
- 非 `PLATFORM_ADMIN` 看不到平台管理菜单。
- 非 `PLATFORM_ADMIN` 不能访问平台管理接口。
- `PLATFORM_ADMIN` 可以查询租户列表。
- `PLATFORM_ADMIN` 可以创建租户。
- 创建租户时 `code` 不能为空、格式合法且不能重复。
- `PLATFORM_ADMIN` 可以启用 / 禁用租户。
- 禁用租户后普通用户、Tenant Admin、DO、DU 不能进入该租户。
- `PLATFORM_ADMIN` 可以查看租户下用户列表。
- `PLATFORM_ADMIN` 可以管理租户用户关系。
- `PLATFORM_ADMIN` 可以分配和移除 Tenant Admin。
- Tenant Admin 不能管理其他租户，不能访问平台租户管理接口。
- Platform Admin 不能解密所有租户文件。
- 平台管理响应不返回主密钥明文、用户私钥明文、密码明文或文件明文。
- 新增文档、注释和规格说明使用简体中文。
- 代码风格与当前 `domain/repository/service/handler/middleware` 分层保持一致。

## 复杂度跟踪

无宪章违规项。

本功能新增平台路由组、平台权限中间件和平台 service，是为了把平台级权限与租户级权限分离；这是多租户治理必需复杂度。平台审计暂不落表，平台控制台统计轻量实现或预留，避免提前扩大范围。
