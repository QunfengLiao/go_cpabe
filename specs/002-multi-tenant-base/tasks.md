# Tasks：CP-ABE 系统多租户基础能力

**输入**：`specs/002-multi-tenant-base/spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/tenant-api.md`、`quickstart.md`

**目标**：按可独立交付的功能模块生成任务，不直接开始写代码。每个任务都必须保持中文说明，并遵守当前项目 `domain / repository / service / handler / middleware` 分层。

**路径约定**：

- 后端代码：`backend/internal/...`
- 后端入口：`backend/cmd/...`
- 数据库脚本：`backend/migrations/...`
- 验证脚本：`backend/scripts/...`
- 前端类型和请求封装：`desktop/src/renderer/src/...`
- 规范文档：`specs/002-multi-tenant-base/...`

## Phase 1：基础数据结构

### 任务 1：多租户数据模型与迁移脚本

- [X] T001 [US5] 实现多租户基础数据模型与迁移脚本，涉及 `backend/internal/domain/tenant.go`、`backend/migrations/002_create_tenants_roles.sql`、`backend/cmd/server/main.go`、`backend/cmd/admin/main.go`

**任务目标**：新增多租户基础数据表，为后续租户隔离、RBAC、CP-ABE 业务表提供基础。

**涉及模块或文件**：

- `backend/internal/domain/tenant.go`
- `backend/internal/domain/constants.go`
- `backend/migrations/002_create_tenants_roles.sql`
- `backend/cmd/server/main.go`
- `backend/cmd/admin/main.go`
- `specs/002-multi-tenant-base/data-model.md`

**主要实现内容**：

- 新增 `tenants` 租户表，字段至少包含 `id`、`name`、`code`、`status`、`description`、`created_at`、`updated_at`、`deleted_at`。
- 新增 `tenant_users` 用户租户关系表，字段至少包含 `id`、`tenant_id`、`user_id`、`status`、`created_at`、`updated_at`、`deleted_at`。
- 检查当前 `users` 表，不删除现有 `role` 字段，仅标记为过渡兼容字段，不作为租户内授权依据。
- 检查当前项目是否已有 `roles` / `user_roles`。当前项目尚未实现完整角色表，因此本任务需要在迁移脚本中预留或创建 `roles`、`user_roles`，并明确角色分配是后续 RBAC 可继续扩展的基础。
- 如果实现阶段决定暂不创建 `roles` / `user_roles`，必须在任务结果中明确说明：本阶段只落地 `tenants` 和 `tenant_users`，角色分配延后到 RBAC 阶段，且登录返回不承诺租户内角色。
- 设计必要索引：`tenants.code` 唯一索引、`tenant_users(tenant_id, user_id)` 唯一索引、`tenant_users.user_id` 普通索引、`tenant_users.tenant_id` 普通索引。
- 为后续业务表记录 `tenant_id` 设计规范，包括 `attributes`、`user_attributes`、`files`、`file_policies`、`tenant_keys`、`user_private_keys`、`encrypt_records`、`decrypt_records`、`audit_logs`。
- 将新增 Gorm 模型加入当前 `AutoMigrate` 列表，保持现有开发启动方式可用。

**验收标准**：

- 数据库迁移脚本可以正常执行。
- `tenants` 表创建成功。
- `tenant_users` 表创建成功。
- 同一个用户不能重复加入同一个租户。
- 租户 `code` 不能重复。
- 新增模型命名、表名和字段风格与当前 `domain.User` 保持一致。

**不做什么**：

- 不实现 CP-ABE 加密解密。
- 不实现完整 RBAC 菜单权限。
- 不实现文件上传下载。
- 不删除或破坏现有 `users.role` 字段。

## Phase 2：初始化与历史用户承接

### 任务 2：默认租户与初始化数据

- [X] T002 [US1] 实现默认租户与历史用户幂等初始化，涉及 `backend/cmd/admin/main.go`、`backend/internal/service/tenant_service.go`、`backend/internal/repository/tenant_repository.go`、`backend/migrations/002_create_tenants_roles.sql`

**任务目标**：初始化系统默认租户和默认管理员关系，保证已有登录注册功能可以平滑接入多租户。

**涉及模块或文件**：

- `backend/cmd/admin/main.go`
- `backend/cmd/server/main.go`
- `backend/internal/service/tenant_service.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/domain/tenant.go`
- `backend/migrations/002_create_tenants_roles.sql`
- `backend/internal/service/auth_service.go`

**主要实现内容**：

- 初始化默认租户：`name = 默认租户`、`code = default-tenant`、`status = enabled`。
- 将 `users` 表中所有已有用户加入 `default-tenant`，写入 `tenant_users`。
- 初始化脚本必须具备幂等性：重复执行不会重复创建默认租户，也不会重复插入 `tenant_users` 关系。
- 如果本阶段创建了 `roles` / `user_roles`，则按历史用户 `users.role` 映射默认租户角色：原 `admin` 分配 `TENANT_ADMIN`，原 `data_owner` 分配 `DO`，原 `data_user` 分配 `DU`。
- 如果本阶段没有创建角色表，则只要求已有用户加入 `default-tenant`，角色分配明确放到后续 RBAC 阶段。
- 新注册用户第一版默认加入 `default-tenant`，避免破坏现有注册登录流程。
- 预留 `PLATFORM_ADMIN`、`TENANT_ADMIN`、`DO`、`DU` 角色初始化方案；公开注册不得创建 `TENANT_ADMIN` 或 `PLATFORM_ADMIN`。

**验收标准**：

- 系统启动、迁移或管理员初始化后存在默认租户。
- 默认管理员属于默认租户。
- 已有用户可以正常登录。
- 登录后可以查到所属默认租户。
- 重复执行初始化 3 次不会产生重复默认租户、重复成员关系或重复基础角色。

**不做什么**：

- 不实现复杂租户邀请流程。
- 不实现租户套餐、计费、配额。
- 不要求一次性实现完整 Platform Admin 后台。

## Phase 3：租户数据访问基础

### 任务 3：租户领域模型与数据访问层

- [X] T003 [US1] 实现租户领域模型与数据访问层，涉及 `backend/internal/domain/tenant.go`、`backend/internal/repository/tenant_repository.go`、`backend/internal/repository/user_repository.go`

**任务目标**：实现租户相关的 model / repository / dao，为 service 层提供基础查询能力。

**涉及模块或文件**：

- `backend/internal/domain/tenant.go`
- `backend/internal/domain/user_dto.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/repository/user_repository.go`
- `backend/internal/repository/mysql.go`
- `backend/internal/service/tenant_service.go`
- `backend/internal/service/auth_register_test.go`

**主要实现内容**：

- 新增 `Tenant` 模型。
- 新增 `TenantUser` 模型。
- 如本阶段创建角色表，则新增 `Role` 和 `UserRoleAssignment` 模型。
- 新增按 `tenantId` 查询租户的方法。
- 新增按 `userId` 查询用户所属租户列表的方法。
- 新增校验 `userId` 是否属于 `tenantId` 的方法。
- 新增校验租户是否启用的方法。
- 新增加入租户、移出租户、查询租户用户列表的仓储方法。
- 代码风格保持和当前 `GormUserRepository` 一致，repository 层只做数据访问和简单条件过滤。

**验收标准**：

- 可以根据 `tenantId` 查询租户。
- 可以根据 `userId` 查询用户所属租户列表。
- 可以判断 `userId` 是否属于 `tenantId`。
- 可以判断租户是否启用。
- repository 单元测试或服务测试能覆盖启用租户、禁用租户、非成员关系和重复成员关系。

**不做什么**：

- 不在 repository 层写复杂业务判断。
- 不绕过当前项目已有数据库封装。
- 不在 repository 层处理 Token、菜单或 CP-ABE 权限。

## Phase 4：登录与当前用户租户能力

### 任务 4：登录返回租户信息改造

- [X] T004 [US1] 改造登录返回租户信息，涉及 `backend/internal/service/auth_service.go`、`backend/internal/handler/auth_handler.go`、`backend/internal/domain/user_dto.go`、`desktop/src/renderer/src/types.ts`、`desktop/src/renderer/src/api/auth.ts`

**任务目标**：改造登录成功后的返回结构，让前端知道当前用户属于哪些租户。

**涉及模块或文件**：

- `backend/internal/service/auth_service.go`
- `backend/internal/handler/auth_handler.go`
- `backend/internal/domain/user_dto.go`
- `backend/internal/service/tenant_service.go`
- `backend/internal/handler/auth_login_test.go`
- `backend/internal/service/auth_login_test.go`
- `desktop/src/renderer/src/types.ts`
- `desktop/src/renderer/src/api/auth.ts`

**主要实现内容**：

- 登录成功后查询当前用户所属租户列表。
- 如果用户只属于一个启用租户，返回 `current_tenant_id`。
- 如果用户属于多个启用租户，返回 `tenants` 列表，`current_tenant_id` 可以为空。
- 返回结构包含 `user.id`、`user.email`、`user.nickname`、`current_tenant_id`、`tenants`。
- 如本阶段已落地 `roles` / `user_roles`，租户项返回用户在该租户下的 `roles`。
- 如 RBAC 尚未实现，仅返回租户信息和空角色数组，不能伪造菜单权限。
- 保留现有 Token 字段，确保原有刷新和退出登录流程不受影响。

**验收标准**：

- 用户登录成功后可以看到所属租户列表。
- 单租户用户可以自动得到 `current_tenant_id`。
- 多租户用户可以看到多个租户。
- 原有登录注册功能不被破坏。
- 登录响应仍不包含 `password_hash`、`avatar_object_key` 等敏感字段。

**不做什么**：

- 不在登录接口里实现完整菜单树。
- 不在登录接口里实现 CP-ABE 权限判断。
- 不把 `users.role` 当作租户内授权依据。

### 任务 5：当前用户租户列表接口

- [X] T005 [US1] 实现当前用户租户列表接口，涉及 `backend/internal/handler/tenant_handler.go`、`backend/internal/service/tenant_service.go`、`backend/internal/handler/router.go`

**任务目标**：提供接口查询当前登录用户所属租户列表。

**涉及模块或文件**：

- `backend/internal/handler/tenant_handler.go`
- `backend/internal/service/tenant_service.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/handler/router.go`
- `backend/internal/pkg/response/errors.go`
- `backend/internal/handler/tenant_me_test.go`
- `specs/002-multi-tenant-base/contracts/tenant-api.md`

**主要实现内容**：

- 按当前项目 REST 风格实现 `GET /api/v1/me/tenants`。
- 从现有登录态中获取当前 `userId`。
- 查询该用户所属租户。
- 只返回启用状态租户，或在响应中明确区分启用 / 禁用状态。
- 返回租户 `id`、名称、编码、状态、用户在该租户下的角色信息，可选。
- 未登录请求复用现有 `AuthRequired` 失败逻辑。

**验收标准**：

- 已登录用户可以查询自己的租户列表。
- 未登录用户不能访问。
- 用户只能看到自己加入的租户。
- 不会返回其他用户的租户关系。
- 响应结构与 `contracts/tenant-api.md` 一致。

**不做什么**：

- 不允许普通用户查询全平台所有租户。
- 不返回敏感字段。
- 不在该接口中执行租户切换或菜单计算。

## Phase 5：租户切换与上下文

### 任务 6：租户切换接口

- [X] T006 [US2] 实现租户切换接口，涉及 `backend/internal/handler/tenant_handler.go`、`backend/internal/service/tenant_service.go`、`backend/internal/handler/router.go`、`desktop/src/renderer/src/api/request.ts`

**任务目标**：实现当前用户切换租户能力，为后续 RBAC 菜单权限和业务数据隔离提供 `currentTenantId`。

**涉及模块或文件**：

- `backend/internal/handler/tenant_handler.go`
- `backend/internal/service/tenant_service.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/handler/router.go`
- `backend/internal/pkg/response/errors.go`
- `backend/internal/handler/tenant_switch_test.go`
- `desktop/src/renderer/src/api/request.ts`
- `specs/002-multi-tenant-base/contracts/tenant-api.md`

**主要实现内容**：

- 按当前项目 REST 风格实现 `POST /api/v1/me/switch-tenant`。
- 从登录态获取 `userId`。
- 校验 `tenantId` 是否存在。
- 校验租户是否启用。
- 校验当前用户是否属于该租户。
- 遵循 plan 阶段选择：MVP 不把 `currentTenantId` 写入 Token 或 Redis Session；切换成功表示后端已验证可进入该租户，后续请求由前端携带 `X-Tenant-Id`。
- 返回当前租户信息。
- 如 RBAC 已实现或本阶段已建角色关系，返回当前租户下 `roles`；`menus` 返回空数组或预留结构。

**验收标准**：

- 用户可以切换到自己所属的租户。
- 用户不能切换到自己不属于的租户。
- 用户不能切换到禁用租户。
- 切换成功后后端可以识别并返回 `current_tenant_id`。
- 错误码能区分租户不存在、租户禁用、非成员访问。

**不做什么**：

- 不实现前端租户选择页面。
- 不实现菜单树完整逻辑，除非 RBAC 已经完成。
- 不重写双 Token 登录态。

### 任务 7：租户上下文中间件

- [X] T007 [US5] 实现租户上下文中间件，涉及 `backend/internal/middleware/tenant.go`、`backend/internal/middleware/auth.go`、`backend/internal/handler/router.go`、`backend/internal/service/tenant_service.go`

**任务目标**：新增或扩展中间件，让后续接口可以从请求上下文中安全获取 `currentTenantId`。

**涉及模块或文件**：

- `backend/internal/middleware/tenant.go`
- `backend/internal/middleware/auth.go`
- `backend/internal/service/tenant_service.go`
- `backend/internal/handler/router.go`
- `backend/internal/pkg/response/errors.go`
- `backend/internal/middleware/tenant_test.go`

**主要实现内容**：

- 复用当前已有登录中间件 `AuthRequired`。
- 从上下文获取当前登录用户 `userId`。
- 从请求头 `X-Tenant-Id` 获取 `currentTenantId`。
- 校验用户是否属于当前租户。
- 校验租户是否启用。
- 将 `tenantId` 写入请求上下文。
- 统一定义 Context Key，例如 `ContextTenantID`、`ContextTenantRoles`，避免字符串散落。
- 后续 handler / service 可以通过上下文获取 `tenantId`。
- 涉及安全边界的代码必须用中文注释解释：请求头只是租户选择输入，真正可信的是后端成员关系校验后的上下文。

**验收标准**：

- 已登录且属于当前租户的用户可以通过校验。
- 未登录用户不能通过校验。
- 不属于该租户的用户不能通过校验。
- 禁用租户不能通过校验。
- handler 中可以拿到 `tenantId`。
- 用户篡改 `X-Tenant-Id` 不能访问其他租户。

**不做什么**：

- 不重复实现登录认证。
- 不把 `tenantId` 直接信任为用户传参。
- 不允许用户通过篡改 `tenantId` 访问其他租户。

## Phase 6：租户管理能力

### 任务 8：租户管理接口

- [X] T008 [US3] 实现基础租户管理接口，涉及 `backend/internal/handler/tenant_handler.go`、`backend/internal/service/tenant_service.go`、`backend/internal/repository/tenant_repository.go`、`backend/internal/handler/router.go`

**任务目标**：提供基础租户管理能力，为平台管理员或租户管理员后续操作提供接口。

**涉及模块或文件**：

- `backend/internal/handler/tenant_handler.go`
- `backend/internal/service/tenant_service.go`
- `backend/internal/repository/tenant_repository.go`
- `backend/internal/pkg/response/errors.go`
- `backend/internal/handler/router.go`
- `backend/internal/handler/tenant_admin_test.go`
- `specs/002-multi-tenant-base/contracts/tenant-api.md`

**主要实现内容**：

- 按当前项目 `/api/v1` REST 风格实现接口路径。
- `POST /api/v1/tenants`：创建租户。
- `GET /api/v1/tenants`：查询租户列表。
- `GET /api/v1/tenants/:id`：查询租户详情。
- `PATCH /api/v1/tenants/:id/enable`：启用租户。
- `PATCH /api/v1/tenants/:id/disable`：禁用租户。
- `POST /api/v1/tenants/:id/users`：将用户加入租户。
- `DELETE /api/v1/tenants/:id/users/:userId`：将用户移出租户。
- `GET /api/v1/tenants/:id/users`：查询租户用户列表。
- 创建租户时校验 `code` 唯一。
- 禁用租户后用户不能切换进入该租户。
- 加入租户时避免重复加入。
- 移出租户时可选保护最后一个租户管理员，若不实现必须在接口行为中说明。
- 预留 Platform Admin 权限校验；当前无完整 RBAC 时，可用默认管理员或租户管理员关系做 MVP 约束。

**验收标准**：

- 可以创建租户。
- 可以查询租户列表和详情。
- 可以启用 / 禁用租户。
- 可以将用户加入租户。
- 可以将用户移出租户。
- 可以查询租户下用户列表。
- 非授权用户不能管理其他租户。

**不做什么**：

- 不实现复杂组织架构。
- 不实现租户邀请邮件。
- 不实现套餐和计费。
- 不实现完整按钮级 RBAC 权限。

## Phase 7：数据隔离规范

### 任务 9：租户级数据隔离规范落地

- [X] T009 [US5] 落地租户级数据隔离规范文档与代码约束入口，涉及 `specs/002-multi-tenant-base/data-model.md`、`specs/002-multi-tenant-base/quickstart.md`、`backend/internal/middleware/tenant.go`

**任务目标**：为后续 RBAC 和 CP-ABE 业务表建立统一的数据隔离规范，避免跨租户访问。

**涉及模块或文件**：

- `specs/002-multi-tenant-base/data-model.md`
- `specs/002-multi-tenant-base/quickstart.md`
- `specs/002-multi-tenant-base/contracts/tenant-api.md`
- `backend/internal/middleware/tenant.go`
- `backend/internal/repository/tenant_repository.go`
- 后续待建模块：`backend/internal/domain/attribute.go`、`backend/internal/domain/file.go`、`backend/internal/domain/policy.go`、`backend/internal/domain/audit.go`

**主要实现内容**：

- 明确所有租户内业务表必须包含 `tenant_id`。
- 明确所有租户内查询必须带 `tenant_id` 条件。
- 明确 `tenant_id` 必须来自后端上下文，不直接信任前端传参。
- 为后续表预留规范：`attributes`、`user_attributes`、`files`、`file_policies`、`tenant_keys`、`user_private_keys`、`encrypt_records`、`decrypt_records`、`audit_logs`。
- 在文档中说明多租户、RBAC、CP-ABE 三层权限边界。
- 在中间件或仓储方法注释中说明安全边界：租户上下文只解决组织隔离，不代表菜单权限或解密权限。

**验收标准**：

- 项目中有清晰的多租户开发规范文档。
- 后续业务开发知道 `tenant_id` 应该怎么使用。
- 文档说明了如何避免跨租户数据访问。
- 文档明确 Tenant Admin、DO、DU 不天然拥有 CP-ABE 解密权限。

**不做什么**：

- 不实际实现所有 CP-ABE 业务表。
- 不实现文件加密解密业务。
- 不把规范文档替代真实代码校验。

## Phase 8：接口测试与验收

### 任务 10：接口测试与验收

- [ ] T010 [US1] [US2] [US3] [US5] 完成多租户接口测试与验收说明，涉及 `backend/internal/handler/*tenant*_test.go`、`backend/internal/middleware/tenant_test.go`、`backend/scripts/verify_auth_flow.sh`、`specs/002-multi-tenant-base/quickstart.md`

**任务目标**：验证多租户基础能力可用，并且不破坏已有登录注册功能。

**涉及模块或文件**：

- `backend/internal/handler/tenant_me_test.go`
- `backend/internal/handler/tenant_switch_test.go`
- `backend/internal/handler/tenant_admin_test.go`
- `backend/internal/middleware/tenant_test.go`
- `backend/internal/service/tenant_service_test.go`
- `backend/internal/handler/auth_login_test.go`
- `backend/internal/handler/auth_register_test.go`
- `backend/scripts/verify_auth_flow.sh`
- `specs/002-multi-tenant-base/quickstart.md`

**主要实现内容**：

- 测试创建租户。
- 测试用户加入租户。
- 测试历史用户迁移到 `default-tenant` 的幂等性。
- 测试用户登录返回租户列表。
- 测试用户查询自己的租户列表。
- 测试切换租户成功。
- 测试切换到不属于自己的租户失败。
- 测试切换到禁用租户失败。
- 测试未登录访问租户接口失败。
- 测试已有登录注册流程仍然可用。
- 测试篡改 `X-Tenant-Id` 不能越权访问。
- 更新手动验证说明，覆盖 `quickstart.md` 中的核心路径。

**验收标准**：

- 所有核心接口手动测试通过。
- 原有登录注册功能正常。
- 多租户上下文可以正常获取。
- 不能通过篡改 `tenantId` 访问其他租户。
- 关键逻辑有必要的测试或验证说明。
- `go test ./...` 通过。

**不做什么**：

- 不做性能压测。
- 不做复杂安全审计。
- 不做前端页面开发。

## 依赖关系与执行顺序

### 阶段依赖

- Phase 1 `T001` 是所有后续任务的数据结构基础。
- Phase 2 `T002` 依赖 `T001`，用于承接历史用户和默认租户。
- Phase 3 `T003` 依赖 `T001`，为后续服务和接口提供仓储能力。
- Phase 4 `T004`、`T005` 依赖 `T002` 和 `T003`。
- Phase 5 `T006`、`T007` 依赖 `T003`，其中 `T007` 可在 `T006` 后或并行推进，但验收依赖租户校验能力稳定。
- Phase 6 `T008` 依赖 `T003`、`T006`、`T007`。
- Phase 7 `T009` 可在 `T007` 后完善，也可与 `T008` 并行补充文档。
- Phase 8 `T010` 依赖目标验收范围内的任务完成。

### 用户故事映射

- **US1：用户登录后获得租户列表**：`T002`、`T003`、`T004`、`T005`、`T010`。
- **US2：用户切换当前租户**：`T006`、`T007`、`T010`。
- **US3：管理员维护租户与成员关系**：`T008`、`T010`。
- **US4：同一用户在不同租户拥有不同角色**：`T001`、`T002`、`T003`、`T004`、`T006`、`T008`。
- **US5：租户内业务数据隔离**：`T001`、`T007`、`T009`、`T010`。

### MVP 建议范围

MVP 优先完成：

1. `T001` 多租户数据模型与迁移脚本。
2. `T002` 默认租户与初始化数据。
3. `T003` 租户领域模型与数据访问层。
4. `T004` 登录返回租户信息改造。
5. `T005` 当前用户租户列表接口。
6. `T006` 租户切换接口。
7. `T007` 租户上下文中间件。

完成上述任务后，系统已经具备“用户登录可获得租户列表、可切换租户、后端可校验当前租户”的基础闭环。`T008` 租户管理接口、`T009` 数据隔离规范和 `T010` 完整验收用于把能力补齐到可演示、可继续开发的状态。

## 可并行机会

- `T001` 中 SQL 迁移脚本与 Gorm 模型可由不同开发者并行设计，但合并前必须统一字段和索引。
- `T004` 登录响应改造和 `T005` 当前用户租户列表接口在 `T003` 完成后可以并行。
- `T006` 切换接口和 `T007` 租户上下文中间件可在共享校验服务稳定后并行。
- `T008` 租户管理接口和 `T009` 数据隔离规范可并行。
- `T010` 的测试用例可随着各任务完成逐步补充，不必等全部接口完成后一次性编写。

## 实施策略

1. 先完成数据结构和默认租户，确保现有单租户用户不会因为引入多租户而无法登录。
2. 再改造登录响应和当前用户租户列表，让前端获得租户上下文来源。
3. 然后实现切换租户和租户上下文中间件，建立后续业务查询的可信 `tenant_id`。
4. 最后补齐租户管理接口、数据隔离规范和测试验收。

## 任务格式检查

- 所有主任务均使用 `- [ ] Txxx ... 文件路径` 格式。
- 任务编号从 `T001` 到 `T010` 连续。
- 每个任务包含任务编号、任务名称、任务目标、涉及模块或文件、主要实现内容、验收标准和不做什么。
- 所有任务描述使用中文，保留必要代码标识符、路径、接口和 JSON 字段名。
