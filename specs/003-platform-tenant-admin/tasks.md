# 任务列表：Platform Admin 平台管理员租户管理能力

**输入**：`specs/003-platform-tenant-admin/` 下的 `spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/platform-api.md`、`quickstart.md`

**前置条件**：已完成规格文档和实现计划；本文件只拆分可执行任务，不直接进入代码实现。

**组织方式**：按“可独立交付的功能模块”拆分为 10 个任务。每个任务保留 SpecKit 要求的复选框格式，并补充任务目标、涉及模块或文件、主要实现内容、验收标准和不做什么。

**范围边界**：本阶段只做 Platform Admin 平台租户管理，不实现 CP-ABE 加解密、文件上传下载、访问树可视化、租户内完整 RBAC 菜单权限、主密钥明文展示、用户私钥明文展示或绕过策略解密文件。

## Phase 1：平台角色与授权底座

**目标**：先把平台级角色、初始化数据和平台授权边界建立起来，作为后续平台接口和前端入口的共同基础。

- [X] T001 平台级角色模型与初始化数据 in backend/internal/domain/constants.go, backend/internal/domain/tenant.go, backend/internal/repository/tenant_repository.go, backend/internal/service/tenant_service.go, backend/internal/service/auth_service.go, backend/cmd/admin/main.go, backend/migrations/002_create_tenants_roles.sql

### 任务 1：平台级角色模型与初始化数据

**任务目标**：支持 `PLATFORM_ADMIN` 平台管理员角色，并初始化平台级和租户级基础角色。

**涉及模块或文件**：

- `backend/internal/domain/constants.go`
- `backend/internal/domain/tenant.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/repository/user_repository.go`
- `backend/internal/service/tenant_service.go`
- `backend/internal/service/auth_service.go`
- `backend/cmd/admin/main.go`
- `backend/migrations/002_create_tenants_roles.sql`
- `backend/internal/service/tenant_service_test.go`
- `backend/internal/service/tenant_test_helpers_test.go`

**主要实现内容**：

- 检查 `roles` 表和领域模型是否已支持 `scope`，复用 `platform` / `tenant` 作用域。
- 确认 `PLATFORM_ADMIN` 为平台级角色，`TENANT_ADMIN`、`DO`、`DU` 为租户级角色。
- 确认 `user_roles.tenant_id` 支持平台级角色为空、租户级角色必填的表达。
- 初始化 `PLATFORM_ADMIN`、`TENANT_ADMIN`、`DO`、`DU` 四个基础角色。
- 初始化或幂等确认 `scnu`、`sangfor`、`aia-hk` 三个演示租户，并保留历史 `default-tenant` 兼容。
- 扩展受控管理命令，支持创建测试平台管理员账号，或给已有账号分配 `PLATFORM_ADMIN`。
- 为平台级角色分配实现仓储层幂等保护，避免 MySQL 唯一索引无法约束 `tenant_id = NULL` 导致重复平台角色。
- 扩展登录响应，返回 `platform_roles` 给前端做菜单展示判断；后端安全边界仍以后端中间件查询为准。
- 对涉及平台角色和敏感登录响应的逻辑补充必要中文注释，说明为什么不能依赖旧 `users.role` 做平台授权。

**验收标准**：

- 系统存在 `PLATFORM_ADMIN` 平台级角色。
- 系统存在 `TENANT_ADMIN`、`DO`、`DU` 租户级角色。
- 可以通过受控命令给用户分配 `PLATFORM_ADMIN`。
- 重复执行初始化脚本或命令不会重复创建角色、租户或平台角色关系。
- 平台级角色 `tenant_id` 为空，租户级角色 `tenant_id` 不为空，边界清晰。
- 登录响应可提供 `platform_roles`，且不返回 `password_hash`、密码明文、主密钥明文或用户私钥明文。

**不做什么**：

- 不实现完整菜单权限。
- 不实现 CP-ABE 加密解密。
- 不用旧 `users.role = admin` 作为平台接口授权依据。

- [X] T002 [US4] Platform Admin 权限中间件 in backend/internal/middleware/platform.go, backend/internal/repository/tenant_repository.go, backend/internal/handler/router.go, backend/internal/pkg/response/errors.go, backend/internal/middleware/platform_test.go

### 任务 2：Platform Admin 权限中间件

**任务目标**：实现平台管理员接口访问控制，保证平台接口只接受平台级授权。

**涉及模块或文件**：

- `backend/internal/middleware/platform.go`
- `backend/internal/middleware/auth.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/handler/router.go`
- `backend/internal/pkg/response/errors.go`
- `backend/internal/middleware/platform_test.go`
- `backend/internal/handler/tenant_admin_test.go`

**主要实现内容**：

- 复用当前 `AuthRequired` 登录态校验，不重复解析或校验 token。
- 新增 `PlatformAdminRequired` 中间件，执行顺序为 `AuthRequired -> PlatformAdminRequired`。
- 从 Gin 上下文读取 `user_id`，查询 `user_roles` 联合 `roles`。
- 只认可 `tenant_id IS NULL` 且 `roles.code = PLATFORM_ADMIN` 的平台级角色。
- 平台管理接口不读取、不依赖 `currentTenantId`、`X-Tenant-Id` 或 `TenantRequired`。
- 非 `PLATFORM_ADMIN` 返回 403，并使用 `PLATFORM_PERMISSION_DENIED` 或等价明确错误码。
- 避免把旧全局 `admin`、`TENANT_ADMIN`、`DO`、`DU` 误判为平台管理员。
- 在权限代码中加入中文注释，解释平台级授权为什么必须与租户上下文分离。

**验收标准**：

- `PLATFORM_ADMIN` 可以访问平台管理接口。
- `TENANT_ADMIN` 不能访问平台管理接口。
- `DO` / `DU` 不能访问平台管理接口。
- 未登录用户不能访问平台管理接口。
- 携带旧 `X-Tenant-Id` 不会影响平台授权判断结果。

**不做什么**：

- 不重复实现登录认证。
- 不把租户级管理员当成平台管理员。
- 不把前端 `platform_roles` 字段当作安全边界。

## Phase 2：平台租户管理后端主链路

**目标**：交付平台租户生命周期、租户用户关系和租户管理员分配三条 P1 后端主链路。

- [X] T003 [US1] 平台租户管理接口 in backend/internal/service/platform_tenant_service.go, backend/internal/handler/platform_tenant_handler.go, backend/internal/handler/router.go, backend/internal/repository/tenant_repository.go, backend/internal/pkg/response/errors.go, backend/internal/handler/platform_tenant_test.go

### 任务 3：平台租户管理接口

**任务目标**：实现 Platform Admin 对租户的基础生命周期管理能力。

**涉及模块或文件**：

- `backend/internal/domain/tenant.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/service/platform_tenant_service.go`
- `backend/internal/handler/platform_tenant_handler.go`
- `backend/internal/handler/router.go`
- `backend/internal/pkg/response/errors.go`
- `backend/internal/service/tenant_service.go`
- `backend/internal/handler/platform_tenant_test.go`
- `backend/internal/service/platform_tenant_service_test.go`

**主要实现内容**：

- 新增或拆分 `PlatformTenantService`，承载平台租户列表、创建、详情、启用、禁用逻辑。
- 新增 `/api/v1/platform` 路由组，并挂载 `AuthRequired + PlatformAdminRequired`。
- 实现 `GET /api/v1/platform/tenants`。
- 实现 `POST /api/v1/platform/tenants`。
- 实现 `GET /api/v1/platform/tenants/:id`。
- 实现 `PATCH /api/v1/platform/tenants/:id/enable`。
- 实现 `PATCH /api/v1/platform/tenants/:id/disable`。
- 校验租户 `name` 不能为空、`code` 不能为空、`code` 全局唯一。
- 校验租户 `code` 只允许小写字母、数字和中划线，建议正则 `^[a-z0-9]+(?:-[a-z0-9]+)*$`。
- 重复 `code` 返回明确错误，例如 `TENANT_CODE_EXISTS`。
- 禁用租户只变更状态，不删除历史数据、成员关系或角色关系。
- 确认禁用租户后，现有 `TenantRequired`、`SwitchTenant`、用户租户上下文逻辑继续阻止普通用户进入该租户。

**验收标准**：

- `PLATFORM_ADMIN` 可以查询租户列表。
- `PLATFORM_ADMIN` 可以创建租户。
- 重复 `code` 创建失败，并返回明确错误。
- 非法 `code` 创建失败，并返回明确错误。
- `PLATFORM_ADMIN` 可以查询租户详情。
- `PLATFORM_ADMIN` 可以启用 / 禁用租户。
- 非 `PLATFORM_ADMIN` 不能调用这些接口。
- 禁用租户后普通用户、Tenant Admin、DO、DU 不能进入该租户。

**不做什么**：

- 不实现租户计费。
- 不实现租户资源配额。
- 不实现每租户独立数据库。
- 不把平台租户管理继续堆在旧 `/api/v1/tenants` 混合入口中。

- [X] T004 [US2] 租户用户关系管理接口 in backend/internal/service/platform_tenant_user_service.go, backend/internal/handler/platform_tenant_handler.go, backend/internal/repository/tenant_repository.go, backend/internal/repository/user_repository.go, backend/internal/handler/platform_tenant_user_test.go

### 任务 4：租户用户关系管理接口

**任务目标**：实现 Platform Admin 对用户和租户关系的管理。

**涉及模块或文件**：

- `backend/internal/domain/tenant.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/repository/user_repository.go`
- `backend/internal/service/platform_tenant_user_service.go`
- `backend/internal/service/tenant_service.go`
- `backend/internal/handler/platform_tenant_handler.go`
- `backend/internal/handler/router.go`
- `backend/internal/pkg/response/errors.go`
- `backend/internal/handler/platform_tenant_user_test.go`
- `backend/internal/service/platform_tenant_user_service_test.go`

**主要实现内容**：

- 新增 `PlatformTenantUserService`，避免把租户用户关系逻辑堆到 handler。
- 实现 `GET /api/v1/platform/tenants/:id/users`。
- 实现 `POST /api/v1/platform/tenants/:id/users`。
- 实现 `DELETE /api/v1/platform/tenants/:id/users/:userId`。
- 查询指定租户下用户列表，返回用户基础信息、成员状态和租户内角色。
- 将已存在用户加入指定租户。
- 将用户移出指定租户，移出后用户不能继续切换进入该租户。
- 校验用户是否存在。
- 校验租户是否存在。
- 默认拒绝向已禁用租户新增普通成员。
- 避免重复加入同一租户；重复加入应幂等返回现有关系或明确提示。
- 移除用户时检查是否会移除最后一个有效 `TENANT_ADMIN`，按计划默认保护。

**验收标准**：

- `PLATFORM_ADMIN` 可以查看租户用户列表。
- `PLATFORM_ADMIN` 可以将用户加入租户。
- 同一个用户不能重复加入同一租户。
- `PLATFORM_ADMIN` 可以将用户移出租户。
- 用户移出租户后不能再切换进入该租户。
- 移除最后一个有效 `TENANT_ADMIN` 时默认被拒绝。
- 非 `PLATFORM_ADMIN` 不能调用这些接口。

**不做什么**：

- 不实现邀请邮件。
- 不实现复杂组织架构。
- 不实现部门管理。
- 不实现租户内完整用户画像管理。

- [X] T005 [US3] 租户管理员分配接口 in backend/internal/service/platform_role_service.go, backend/internal/handler/platform_tenant_handler.go, backend/internal/repository/tenant_repository.go, backend/internal/pkg/response/errors.go, backend/internal/handler/platform_tenant_admin_test.go

### 任务 5：租户管理员分配接口

**任务目标**：实现 Platform Admin 给指定租户分配 Tenant Admin 的能力。

**涉及模块或文件**：

- `backend/internal/domain/tenant.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/repository/user_repository.go`
- `backend/internal/service/platform_role_service.go`
- `backend/internal/service/platform_tenant_user_service.go`
- `backend/internal/handler/platform_tenant_handler.go`
- `backend/internal/handler/router.go`
- `backend/internal/pkg/response/errors.go`
- `backend/internal/handler/platform_tenant_admin_test.go`
- `backend/internal/service/platform_role_service_test.go`

**主要实现内容**：

- 新增或扩展 `PlatformRoleService`，集中封装平台级角色和租户级角色分配规则。
- 实现 `POST /api/v1/platform/tenants/:id/admins`。
- 实现 `DELETE /api/v1/platform/tenants/:id/admins/:userId`。
- 给指定用户分配当前租户的 `TENANT_ADMIN` 角色。
- 移除指定用户在当前租户下的 `TENANT_ADMIN` 角色。
- 校验用户是否存在。
- 校验租户是否存在。
- 校验用户已属于该租户且成员状态有效。
- 如果用户不属于租户，按计划返回错误并提示需先加入租户。
- 防止重复分配 `TENANT_ADMIN`；重复分配应幂等返回成功或明确说明已存在。
- 默认防止移除最后一个有效 Tenant Admin。
- 确认租户级角色只影响当前 `tenant_id`，不影响用户在其他租户的角色。

**验收标准**：

- `PLATFORM_ADMIN` 可以分配 Tenant Admin。
- `PLATFORM_ADMIN` 可以移除 Tenant Admin。
- 被分配的用户在对应租户下拥有 `TENANT_ADMIN`。
- 同一用户同一租户不会重复分配 `TENANT_ADMIN`。
- 用户不属于目标租户时不能被直接分配 `TENANT_ADMIN`。
- 移除最后一个有效 Tenant Admin 时默认被拒绝。
- Tenant Admin 不能给其他租户分配管理员。
- 非 `PLATFORM_ADMIN` 不能调用这些接口。

**不做什么**：

- 不实现租户内完整角色菜单管理。
- 不实现 DO / DU 业务权限页面。
- 不把 `TENANT_ADMIN` 当成平台管理员。

## Phase 3：平台首页统计

**目标**：补齐平台后台的首页统计能力，为 Platform Admin 提供平台治理概览。

- [X] T006 [P] [US6] 平台控制台统计接口 in backend/internal/service/platform_dashboard_service.go, backend/internal/handler/platform_dashboard_handler.go, backend/internal/repository/tenant_repository.go, backend/internal/handler/router.go, backend/internal/handler/platform_dashboard_test.go

### 任务 6：平台控制台统计接口，可选

**任务目标**：为 Platform Admin 首页提供平台级统计数据；如果实现成本影响主链路，则至少返回稳定预留结构。

**涉及模块或文件**：

- `backend/internal/service/platform_dashboard_service.go`
- `backend/internal/handler/platform_dashboard_handler.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/repository/user_repository.go`
- `backend/internal/handler/router.go`
- `backend/internal/handler/platform_dashboard_test.go`
- `specs/003-platform-tenant-admin/contracts/platform-api.md`
- `specs/003-platform-tenant-admin/quickstart.md`

**主要实现内容**：

- 实现 `GET /api/v1/platform/dashboard`。
- 统计租户总数。
- 统计启用租户数。
- 统计禁用租户数。
- 统计平台用户总数。
- 统计租户用户关系数量。
- 统计 Tenant Admin 数量。
- 返回 `audit_enabled` 字段，明确当前审计是否已落地。
- 如果暂不实现真实统计，返回稳定结构并标明未实现状态，前端可正常展示预留位置。

**验收标准**：

- `PLATFORM_ADMIN` 可以查看平台统计。
- 非 `PLATFORM_ADMIN` 不能访问平台统计。
- 已实现统计时，数据与当前表数据基本准确。
- 未实现统计时，接口返回结构清晰，不影响平台首页渲染。

**不做什么**：

- 不做复杂数据分析。
- 不做图表大屏。
- 不做性能压测。
- 不把统计结果扩展为跨租户文件内容或密钥可见性。

## Phase 4：前端平台管理体验

**目标**：让 Platform Admin 登录后能进入平台后台，并完成租户生命周期、用户关系和租户管理员分配的可视化管理。

- [X] T007 [US4] 前端平台管理菜单与路由 in desktop/src/renderer/src/types.ts, desktop/src/renderer/src/api/authStorage.ts, desktop/src/renderer/src/auth/AuthContext.tsx, desktop/src/renderer/src/auth/RequirePlatformAdmin.tsx, desktop/src/renderer/src/components/AppLayout.tsx, desktop/src/renderer/src/main.tsx

### 任务 7：前端平台管理菜单与路由

**任务目标**：Platform Admin 登录后可以进入平台管理后台，非 Platform Admin 看不到平台管理入口。

**涉及模块或文件**：

- `desktop/src/renderer/src/types.ts`
- `desktop/src/renderer/src/api/authStorage.ts`
- `desktop/src/renderer/src/auth/AuthContext.tsx`
- `desktop/src/renderer/src/auth/RequirePlatformAdmin.tsx`
- `desktop/src/renderer/src/components/AppLayout.tsx`
- `desktop/src/renderer/src/main.tsx`
- `desktop/src/renderer/src/pages/StartupRedirect.tsx`
- `desktop/src/renderer/src/config/tenantLoginConfigs.ts`

**主要实现内容**：

- 扩展前端登录态类型和存储，支持 `platform_roles`、`isPlatformAdmin`、`tenants`、`currentTenantId`、`currentTenantCode`。
- `authStorage.ts` 可保存平台角色信息，但不新增密码、用户私钥、主密钥等敏感明文存储。
- `AuthContext` 暴露平台管理员判断能力。
- 新增平台路由守卫，例如 `RequirePlatformAdmin`。
- 新增平台控制台路由。
- 新增租户管理路由。
- 新增租户详情路由。
- 新增租户用户关系管理路由。
- 在 `AppLayout` 中按角色生成菜单，只有 `PLATFORM_ADMIN` 展示平台后台入口。
- 保持单一 `LoginPage + tenantLoginConfigs`，不复制租户登录页。
- 保持 `dev:electron:sangfor` 等开发启动逻辑进入指定租户登录页，不被平台菜单改动破坏。

**验收标准**：

- `PLATFORM_ADMIN` 可以看到平台管理菜单。
- 非 `PLATFORM_ADMIN` 看不到平台管理菜单。
- 手动访问平台路由时，非 `PLATFORM_ADMIN` 被前端拦截并提示无权限。
- 后端接口仍以 403 作为真正安全边界。
- 页面风格和当前项目保持一致。
- 既有多租户登录页和 Electron 指定租户启动逻辑不被破坏。

**不做什么**：

- 不实现租户内完整菜单权限。
- 不复制多套页面。
- 不把前端菜单隐藏当成安全边界。

- [X] T008 [US1] 前端租户管理页面 in desktop/src/renderer/src/api/platform.ts, desktop/src/renderer/src/pages/PlatformDashboardPage.tsx, desktop/src/renderer/src/pages/PlatformTenantListPage.tsx, desktop/src/renderer/src/pages/PlatformTenantCreatePage.tsx, desktop/src/renderer/src/pages/PlatformTenantDetailPage.tsx, desktop/src/renderer/src/pages/PlatformTenantUsersPage.tsx, desktop/src/renderer/src/styles.css

### 任务 8：前端租户管理页面

**任务目标**：实现 Platform Admin 对租户、租户用户和租户管理员的可视化管理。

**涉及模块或文件**：

- `desktop/src/renderer/src/api/platform.ts`
- `desktop/src/renderer/src/types.ts`
- `desktop/src/renderer/src/pages/PlatformDashboardPage.tsx`
- `desktop/src/renderer/src/pages/PlatformTenantListPage.tsx`
- `desktop/src/renderer/src/pages/PlatformTenantCreatePage.tsx`
- `desktop/src/renderer/src/pages/PlatformTenantDetailPage.tsx`
- `desktop/src/renderer/src/pages/PlatformTenantUsersPage.tsx`
- `desktop/src/renderer/src/components/AppLayout.tsx`
- `desktop/src/renderer/src/styles.css`

**主要实现内容**：

- 新增平台 API 客户端，封装 `/api/v1/platform` 下的租户、用户关系、管理员分配和控制台接口。
- 实现平台控制台页面，展示统计数据或预留结构。
- 实现租户列表页面，展示 `name`、`code`、`status`、`description`、`created_at`、用户数和 Tenant Admin 数。
- 实现创建租户表单，包含名称、编码、描述校验。
- 实现租户详情页面，展示基础信息和状态。
- 实现启用 / 禁用租户操作。
- 实现租户用户列表页面。
- 实现添加用户到租户。
- 实现分配 / 移除 Tenant Admin。
- 操作失败时显示友好提示，例如重复编码、无权限、用户不存在、不能移除最后一个 Tenant Admin。
- 使用现有布局和视觉风格，保持管理后台清爽、专业、信息密度合理。

**验收标准**：

- 可以在前端查看租户列表。
- 可以在前端创建租户。
- 可以在前端启用 / 禁用租户。
- 可以在前端查看租户用户。
- 可以在前端添加或移出租户用户。
- 可以在前端分配 / 移除 Tenant Admin。
- 操作失败时有友好提示。
- 非 Platform Admin 无法通过前端页面完成平台操作。

**不做什么**：

- 不做复杂 UI 动画。
- 不做文件管理页面。
- 不做 CP-ABE 业务页面。
- 不把页面做成营销式 landing page。

## Phase 4.5：平台操作审计预留

**目标**：为平台级操作预留审计能力，保持 Audit 模块边界清晰。

- [X] T009 [P] [US6] 平台操作审计预留 in backend/internal/service/audit_recorder.go, backend/internal/service/platform_tenant_service.go, backend/internal/service/platform_tenant_user_service.go, backend/internal/service/platform_role_service.go, specs/003-platform-tenant-admin/quickstart.md

### 任务 9：平台操作审计预留

**任务目标**：为平台级操作预留审计能力，并明确本阶段是否落表。

**涉及模块或文件**：

- `backend/internal/service/audit_recorder.go`
- `backend/internal/service/platform_tenant_service.go`
- `backend/internal/service/platform_tenant_user_service.go`
- `backend/internal/service/platform_role_service.go`
- `backend/cmd/server/main.go`
- `specs/003-platform-tenant-admin/data-model.md`
- `specs/003-platform-tenant-admin/quickstart.md`

**主要实现内容**：

- 检查当前项目是否已有审计日志模块或 `audit_logs` 表。
- 如果已有审计模块，接入平台关键操作事件。
- 如果没有审计模块，定义轻量 `AuditRecorder` 接口或 no-op 调用点，避免本阶段临时落一个不可复用的审计表。
- 预留创建租户、启用租户、禁用租户、用户加入租户、用户移出租户、分配 Tenant Admin、移除 Tenant Admin 等事件。
- 在文档中说明后续审计表设计和事件字段建议。
- 确保审计预留不影响平台租户管理主链路验收。

**验收标准**：

- 如果已有审计模块，平台关键操作能记录日志。
- 如果没有审计模块，文档中有清晰预留设计。
- Service 层有清晰可替换的审计调用位置或后续接入点。
- 审计预留不返回主密钥明文、用户私钥明文、密码明文或文件明文。

**不做什么**：

- 不实现复杂审计检索。
- 不实现审计报表。
- 不为了本阶段强行引入完整 Audit 模块和新审计表。


## Phase 5：接口测试与整体验收

**目标**：验证 Platform Admin 租户管理闭环可用，并确认不影响已有多租户登录、用户注册登录和后续 CP-ABE 边界。

- [X] T010 接口测试与验收 in backend/internal/middleware/platform_test.go, backend/internal/handler/platform_tenant_test.go, backend/internal/handler/platform_tenant_user_test.go, backend/internal/handler/platform_tenant_admin_test.go, backend/internal/service/platform_role_service_test.go, specs/003-platform-tenant-admin/quickstart.md, desktop/package.json

### 任务 10：接口测试与验收

**任务目标**：验证 Platform Admin 租户管理闭环可用，并完成回归检查。

**涉及模块或文件**：

- `backend/internal/middleware/platform_test.go`
- `backend/internal/handler/platform_tenant_test.go`
- `backend/internal/handler/platform_tenant_user_test.go`
- `backend/internal/handler/platform_tenant_admin_test.go`
- `backend/internal/service/platform_role_service_test.go`
- `backend/internal/service/platform_tenant_service_test.go`
- `backend/internal/service/platform_tenant_user_service_test.go`
- `specs/003-platform-tenant-admin/quickstart.md`
- `desktop/package.json`
- `desktop/src/renderer/src/pages/LoginPage.tsx`
- `desktop/src/renderer/src/tenantStartup.ts`

**主要实现内容**：

- 测试 `PLATFORM_ADMIN` 登录并可进入平台后台。
- 测试非 `PLATFORM_ADMIN` 访问平台接口失败。
- 测试查询租户列表。
- 测试创建租户。
- 测试重复 `code` 创建失败。
- 测试非法 `code` 创建失败。
- 测试启用 / 禁用租户。
- 测试禁用租户后普通用户不能进入。
- 测试用户加入租户。
- 测试重复加入租户不会产生重复有效关系。
- 测试用户移出租户。
- 测试分配 Tenant Admin。
- 测试移除 Tenant Admin。
- 测试不能移除最后一个有效 Tenant Admin。
- 测试前端平台菜单展示。
- 测试非平台管理员前端不可见平台菜单。
- 运行 `go test ./...`。
- 运行 `npm run typecheck` 和 `npm run build`。
- 验证 `npm run dev:electron:sangfor` 仍进入 `#/login/sangfor`，不会被旧 token、旧租户上下文或平台后台路由覆盖。
- 完成关键注释和可读性检查，确认认证、权限、Token、租户隔离相关注释解释安全边界，且没有无意义注释。

**验收标准**：

- 后端核心接口可用。
- 前端平台管理页面可用。
- 权限边界正确。
- 不影响已有多租户登录功能。
- 不影响已有用户注册登录功能。
- 不破坏 CP-ABE 后续扩展边界。
- 平台管理响应不泄露 `password_hash`、主密钥明文、用户私钥明文、密码明文或文件明文。
- 新增文档、注释和说明保持简体中文。

**不做什么**：

- 不做性能压测。
- 不做完整安全审计。
- 不做 CP-ABE 加密解密测试。
- 不把 Platform Admin 当作跨租户文件解密入口。

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1 平台角色与授权底座**：必须最先完成，阻塞所有平台接口和平台前端入口。
- **Phase 2 平台租户管理后端主链路**：依赖 T001 和 T002，优先完成 P1 后端能力。
- **Phase 3 平台首页统计**：T006 依赖 T001、T002 和基础仓储查询，建议在 T003 后补充真实统计。
- **Phase 4 前端平台管理体验**：T007 依赖 T001、T002 和登录响应扩展；T008 依赖 T003、T004、T005、T007。
- **Phase 4.5 平台操作审计预留**：T009 可在 T003-T005 的 service 边界明确后并行预留。
- **Phase 5 接口测试与验收**：依赖所有目标任务完成。

### 任务依赖

- T001 是所有任务的基础。
- T002 依赖 T001，并阻塞 T003、T004、T005、T006、T007、T008。
- T003 依赖 T001、T002。
- T004 依赖 T003，因为租户存在性和平台路由已经稳定。
- T005 依赖 T004，因为分配 `TENANT_ADMIN` 前用户必须属于目标租户。
- T006 依赖 T001、T002，建议在 T003 后补充真实统计。
- T007 依赖 T001、T002 中的登录响应和平台授权语义。
- T008 依赖 T003、T004、T005、T007。
- T009 可在 T003、T004、T005 的 service 调用点明确后进行。
- T010 依赖 T001-T009。

### 用户故事映射

- **US1 平台管理员管理租户生命周期**：T003、T008、T010。
- **US2 平台管理员维护租户用户关系**：T004、T008、T010。
- **US3 平台管理员分配租户管理员**：T005、T008、T010。
- **US4 平台管理接口只接受平台级授权**：T002、T007、T010。
- **US5 禁用租户后的访问边界**：T003、T004、T010。
- **US6 平台控制台和平台审计预留**：T006、T009、T008、T010。

## 并行机会

- T006 可以在 T003 的租户查询接口稳定后，与 T004/T005 的业务开发并行。
- T009 可以在 service 边界明确后，与前端 T007/T008 并行。
- T007 的路由守卫和菜单可在后端接口实现期间先完成，前提是 T001 的登录响应字段已经确定。
- T008 的页面骨架可与 T003-T005 后端接口并行推进，但接口联调必须等后端契约稳定。

## 独立测试标准

- **US1**：使用 `PLATFORM_ADMIN` 查询、创建、查看、启用、禁用租户；使用非平台管理员访问同接口应失败。
- **US2**：使用 `PLATFORM_ADMIN` 将用户加入租户、重复加入、移出租户；移出后用户不能切换进入该租户。
- **US3**：使用 `PLATFORM_ADMIN` 分配和移除 `TENANT_ADMIN`；目标用户不属于租户时分配失败；租户管理员不能访问平台接口。
- **US4**：分别使用 Platform Admin、Tenant Admin、DO、DU、未登录用户访问平台接口，只有 Platform Admin 成功。
- **US5**：禁用租户后，Tenant Admin、DO、DU 不能进入该租户；Platform Admin 仍可查看租户详情。
- **US6**：平台后台存在控制台入口；统计实现时数字与数据一致；审计未实现时有清晰预留说明。

## 实现策略

### MVP 优先

1. 完成 T001：平台级角色模型与初始化数据。
2. 完成 T002：Platform Admin 权限中间件。
3. 完成 T003：平台租户管理接口。
4. 停下来按 `quickstart.md` 验证租户列表、创建、详情、启用、禁用和越权拒绝。

### P1 完整主链路

1. 完成 MVP。
2. 完成 T004：租户用户关系管理接口。
3. 完成 T005：租户管理员分配接口。
4. 完成 T007：前端平台管理菜单与路由。
5. 完成 T008：前端租户管理页面。
6. 运行 T010 中的接口、前端和回归验收。

### 后续增强

1. 完成 T006：平台控制台统计接口。
2. 完成 T009：平台操作审计预留。
3. 在后续 Audit 模块落地时，把 no-op 审计预留替换为真实审计记录。

## 注意事项

- `[P]` 表示任务可在依赖满足后与其他任务并行推进，但仍需避免修改同一文件造成冲突。
- 每个任务都应保持中文文档和必要中文注释。
- 涉及认证、权限、Token、租户隔离的实现必须解释安全边界。
- 后端平台接口必须使用 `/api/v1/platform/...`，与当前 `/api/v1` 风格一致。
- 平台接口不依赖 `currentTenantId` 或 `X-Tenant-Id`。
- Platform Admin 只能管理平台租户和关系，不能查看主密钥明文、用户私钥明文、密码明文或文件明文，也不能绕过 CP-ABE 策略解密文件。

