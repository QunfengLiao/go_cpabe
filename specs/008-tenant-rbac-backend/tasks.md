# 任务：后端租户级 RBAC

**输入**：`specs/008-tenant-rbac-backend/` 下的 `spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/rbac-api.md`、`quickstart.md`

**前置条件**：已完成规格和实施计划；本阶段只实现后端 RBAC 能力，不实现前端页面，不扩展属性同步、策略版本、文件密钥、审计落库或 CP-ABE 解密流程。

**测试要求**：规格明确要求通过自动化测试或 API 集成测试验收，因此关键领域逻辑、迁移、跨租户安全、DO/DU 可叠加、最后一名 `TENANT_ADMIN` 并发保护和旧角色兼容必须配套测试。

**组织方式**：任务按可独立验证的交付阶段和用户故事组织。每条任务均包含前置依赖、涉及文件或目录、验收方式。

## 格式说明

- `[P]`：可并行执行，前提是依赖任务已完成且不修改同一核心文件。
- `[US1]` 等：对应规格中的用户故事。
- 所有路径均相对仓库根目录。

---

## 阶段 1：基线与共同准备

**目的**：锁定现有行为和回归边界，为后续迁移与重构建立可比较基线。

- [X] T001 现有角色与鉴权链路基线盘点和回归用例整理，依赖：无；文件：`specs/008-tenant-rbac-backend/tasks.md`、`backend/internal/service/tenant_service.go`、`backend/internal/repository/tenant_repository.go`、`backend/internal/service/policy_service.go`、`backend/internal/service/org_attribute_service.go`、`backend/internal/service/org_management_service.go`、`backend/internal/handler/router.go`、`backend/internal/**/*_test.go`；验收：形成测试清单，覆盖当前 `roles/user_roles` 结构、`PLATFORM_ADMIN/TENANT_ADMIN/DO/DU` 初始化、`DO/DU` 互斥链路、租户上下文来源、硬编码鉴权点和现有登录/租户切换/策略/组织基础回归路径。

---

## 阶段 2：基础能力（阻塞所有用户故事）

**目的**：完成共享领域模型、显式迁移、权限事实源、Repository、授权服务和中间件骨架。未完成本阶段前，不应开始接口故事实现。

- [X] T002 角色、权限领域常量与模型演进，依赖：T001；文件：`backend/internal/domain/constants.go`、`backend/internal/domain/tenant.go`；验收：`Role`、`UserRoleAssignment`、`Permission`、`RolePermission` 支持计划字段和状态，保留旧角色常量，新增作用域/分类/状态校验方法，`go test ./internal/domain/...` 或 `go test ./...` 可编译通过。
- [X] T003 数据库迁移、数据回填和幂等权限种子，依赖：T002；文件：`backend/migrations/010_tenant_rbac.sql`、`backend/internal/migrations/automigrate.go`、`backend/cmd/migrate/main.go`、`backend/cmd/seed/main.go`、`backend/internal/service/tenant_service.go`；验收：显式 SQL 完成字段新增、`tenant_id NULL -> 0` 回填、唯一约束调整、`permissions/role_permissions` 建表、内置角色分类回填、`user_roles.status` 回填、权限矩阵幂等 seed，重复执行迁移和 seed 不产生重复角色/权限/绑定。
- [X] T004 数据迁移前后验证脚本与旧角色兼容验证，依赖：T003；文件：`backend/internal/migrations/**/*_test.go`、`backend/migrations/010_tenant_rbac.sql`、`specs/008-tenant-rbac-backend/quickstart.md`；验收：自动化或可重复执行的数据库级验证覆盖迁移前重复/孤立数据检查、迁移后孤立 `user_roles` 检查、每个启用租户至少一名有效 `TENANT_ADMIN`、旧 `DATA_OWNER/DATA_VISITOR` 与现有 `DO/DU` 用户仍获得默认权限。
- [X] T005 角色与权限 Repository 能力实现，依赖：T003；文件：`backend/internal/repository/tenant_repository.go`、`backend/internal/repository/rbac_repository.go`、`backend/internal/domain/tenant.go`；验收：支持租户角色列表、角色详情、创建/更新/禁用自定义角色、角色权限查询和全量替换、成员角色查询和全量替换、权限并集查询、权限判断、有效管理员计数、角色有效成员计数，所有租户方法显式传入并使用可信 `tenantID`。
- [ ] T006 [P] Repository 领域测试覆盖权限并集和租户边界，依赖：T005；文件：`backend/internal/repository/*_test.go`、`backend/internal/service/tenant_test_helpers_test.go`；验收：测试覆盖系统内置租户角色复用、租户自定义角色隔离、跨租户角色不可读写、`REVOKED/EXPIRED/DISABLED` 不产生权限、重复权限去重、平台角色不进入租户权限并集。
- [X] T007 统一 AuthorizationService 实现，依赖：T005；文件：`backend/internal/service/authorization_service.go`、`backend/internal/service/tenant_service.go`；验收：提供平台权限、租户权限、`Has/Require` 权限判断、当前用户授权上下文、指定成员授权查询；仅依赖 Repository，不引入 Redis 权限缓存，不反向依赖 Tenant/Policy/Org Service。
- [ ] T008 [P] AuthorizationService 单元测试，依赖：T007；文件：`backend/internal/service/authorization_service_test.go`、`backend/internal/service/tenant_test_helpers_test.go`；验收：测试证明 `PLATFORM_ADMIN` 只有平台权限、不自动拥有租户权限，租户权限必须满足有效成员关系，多个角色权限合并去重，撤销/过期/禁用角色即时失效。
- [X] T009 Permission Middleware 与统一业务错误码，依赖：T007；文件：`backend/internal/middleware/permission.go`、`backend/internal/middleware/platform.go`、`backend/internal/pkg/response/errors.go`、`backend/internal/handler/router.go`；验收：新增租户/平台权限中间件和稳定错误码，认证中间件只负责认证，租户中间件只负责租户上下文，权限不足返回 403，跨租户资源按项目安全错误返回 404 或统一安全错误。
- [ ] T010 [P] Permission Middleware 与错误映射测试，依赖：T009；文件：`backend/internal/middleware/*_test.go`、`backend/internal/handler/*_test.go`；验收：未登录返回 401，已登录无权限返回 403，缺少租户上下文不会绕过权限判断，平台权限和租户权限作用域不可混用。

**检查点**：基础表、权限事实源、授权服务和权限中间件可编译、可测试；用户故事接口可以开始实现。

---

## 阶段 3：用户故事 1 - 租户管理员维护自定义角色（优先级：P1，MVP）

**目标**：租户管理员可创建、查看、修改、禁用自定义业务角色，并查询可绑定的租户权限目录。

**独立测试**：使用具备 `tenant.role.manage` 的成员创建 `SRE_ENGINEER`，绑定租户权限，修改名称描述，禁用角色；普通成员无权操作；不同租户可创建相同 code，同租户重复 code 返回冲突。

- [ ] T011 [P] [US1] 自定义角色 CRUD 和权限目录接口契约测试，依赖：T009；文件：`backend/internal/handler/rbac_handler_test.go`、`backend/internal/handler/test_helpers_test.go`；验收：先写失败测试覆盖 `GET /api/v1/tenant/permissions`、`GET/POST/PATCH/DELETE /api/v1/tenant/roles`、角色详情、普通成员 403、同租户 code 冲突、跨租户不可见、内置角色不可变。
- [X] T012 [US1] 租户角色服务实现自定义角色生命周期，依赖：T005、T007、T011；文件：`backend/internal/service/tenant_role_service.go`、`backend/internal/domain/tenant.go`；验收：创建角色强制 `TENANT + BUSINESS + builtin=false + ACTIVE`，忽略或拒绝客户端 `tenant_id/scope/category/builtin`，role code 创建后不可修改，禁用为逻辑禁用并返回有效成员数。
- [X] T013 [US1] RBAC Handler 和路由注册角色 CRUD 与权限目录，依赖：T009、T012；文件：`backend/internal/handler/rbac_handler.go`、`backend/internal/handler/router.go`；验收：接口全部挂载到 `/api/v1/tenant`，使用 `TenantRequired` 的可信 `tenant_id`，权限目录只返回 `TENANT + ACTIVE` 权限，不返回平台权限。
- [ ] T014 [US1] 自定义角色 CRUD Service/Handler 测试补齐，依赖：T012、T013；文件：`backend/internal/service/tenant_role_service_test.go`、`backend/internal/handler/rbac_handler_test.go`；验收：自动化测试覆盖租户管理员成功、普通成员失败、同租户重复 code 冲突、不同租户同 code 成功、系统内置角色 code/分类/作用域不可修改、租户 A 不可查询或修改租户 B 角色。

**检查点**：US1 可独立演示角色目录、权限目录、创建、修改、禁用，并通过接口测试。

---

## 阶段 4：用户故事 2 - 租户管理员给成员分配多个角色（优先级：P1）

**目标**：管理员可全量替换成员角色集合，允许 `DO` 与 `DU` 叠加，拒绝平台角色和禁用角色，并保护最后一名租户管理员。

**独立测试**：给同一成员同时分配 `DO`、`DU`、自定义业务角色，查询成员角色和权限并集；任一分配顺序都不删除另一个能力角色；尝试移除最后一名 `TENANT_ADMIN` 被拒绝。

- [ ] T015 [P] [US2] 成员多角色接口和 DO/DU 可叠加测试，依赖：T005、T009；文件：`backend/internal/handler/rbac_handler_test.go`、`backend/internal/service/tenant_role_service_test.go`、`backend/internal/repository/*_test.go`；验收：先写失败测试覆盖 `GET/PUT /api/v1/tenant/members/:userId/roles`、`DO+DU` 同时存在、先分配 `DO` 后分配 `DU` 不删除 `DO`、先分配 `DU` 后分配 `DO` 不删除 `DU`、返回权限并集。
- [X] T016 [US2] 成员多角色全量替换 Repository 与 Service 实现，依赖：T005、T015；文件：`backend/internal/repository/rbac_repository.go`、`backend/internal/repository/tenant_repository.go`、`backend/internal/service/tenant_role_service.go`；验收：事务内校验成员属于当前租户且状态有效，校验角色为系统内置租户角色或当前租户自定义角色，拒绝 `PLATFORM_ADMIN` 和禁用角色，撤销缺失角色为 `REVOKED`，激活或创建新增角色，不物理删除绑定。
- [X] T017 [US2] 移除或废弃旧 DO/DU 互斥逻辑，依赖：T016；文件：`backend/internal/service/tenant_service.go`、`backend/internal/repository/tenant_repository.go`、`backend/internal/service/tenant_test_helpers_test.go`、`backend/internal/handler/test_helpers_test.go`；验收：`ReplaceTenantBusinessRole` 不再删除另一种能力角色，旧单角色接口若保留则作为兼容包装且不会制造 `DO/DU` 互斥，相关内存测试仓储同步新行为。
- [ ] T018 [US2] 最后一名 TENANT_ADMIN 保护实现，依赖：T016；文件：`backend/internal/repository/rbac_repository.go`、`backend/internal/repository/tenant_repository.go`、`backend/internal/service/tenant_role_service.go`、`backend/internal/service/platform_service.go`；验收：影响 `TENANT_ADMIN` 的成员角色替换、成员移除、相关角色禁用均在事务中锁定当前租户行并重新计数，不能使启用租户有效管理员数量降为 0。
- [ ] T019 [P] [US2] 最后一名 TENANT_ADMIN 并发保护测试，依赖：T018；文件：`backend/internal/repository/*_test.go`、`backend/internal/service/tenant_role_service_test.go`；验收：并发替换或撤销两个管理员角色时，至少一个操作失败并返回 `CANNOT_REMOVE_LAST_TENANT_ADMIN` 或冲突错误，最终数据库仍至少有一名有效 `TENANT_ADMIN`。
- [X] T020 [US2] 成员角色 Handler 和路由接入，依赖：T013、T016、T018；文件：`backend/internal/handler/rbac_handler.go`、`backend/internal/handler/router.go`；验收：`GET /api/v1/tenant/members/:userId/roles` 需要 `tenant.member.read`，`PUT /api/v1/tenant/members/:userId/roles` 需要 `tenant.member.manage`，不信任请求体或 query 中的 `tenant_id`，成功响应包含最新角色和权限集合。

**检查点**：US2 可独立演示成员多角色分配、`DO/DU` 叠加、平台角色拒绝、最后管理员保护。

---

## 阶段 5：用户故事 3 - 当前用户获得真实授权上下文（优先级：P1）

**目标**：当前用户进入租户后，可从后端获得来自 `permissions` 与 `role_permissions` 的真实角色集合和权限集合。

**独立测试**：为用户配置有效、多重、撤销、过期和禁用角色，调用当前用户授权接口，只返回有效角色与去重权限；平台管理员不因平台身份获得租户业务权限。

- [ ] T021 [P] [US3] 当前用户授权上下文接口测试，依赖：T007、T009；文件：`backend/internal/handler/rbac_handler_test.go`、`backend/internal/service/authorization_service_test.go`；验收：先写失败测试覆盖 `GET /api/v1/tenant/me/authorization` 返回真实角色与权限、`REVOKED/EXPIRED/DISABLED` 不生效、`DO+DU` 权限同时出现、`PLATFORM_ADMIN` 不自动拥有租户权限。
- [X] T022 [US3] 当前租户上下文权限来源迁移，依赖：T007、T021；文件：`backend/internal/service/tenant_service.go`、`backend/internal/middleware/tenant.go`；验收：`ResolveTenantContext` 不再让平台管理员仅凭平台身份进入任意租户业务上下文，租户权限集合由 AuthorizationService 查询，不再由 `permissionsForRoles` 按角色 code 拼接。
- [X] T023 [US3] 当前用户授权上下文 Handler 实现，依赖：T013、T022；文件：`backend/internal/handler/rbac_handler.go`、`backend/internal/handler/router.go`；验收：`GET /api/v1/tenant/me/authorization` 使用 `gin.Context` 中可信 `user_id` 和 `tenant_id`，响应包含 `tenantId`、有效角色摘要、去重权限 code，权限来自数据库真实查询。
- [ ] T024 [US3] 授权上下文回归测试和旧权限映射收口，依赖：T022、T023；文件：`backend/internal/service/tenant_service_test.go`、`backend/internal/handler/auth_handler_test.go`、`backend/internal/handler/tenant_handler_test.go`；验收：登录、租户切换和当前租户上下文仍可用，返回权限不再调用硬编码角色映射；旧字段如仍保留，值也来自新权限查询。

**检查点**：US3 可独立验证“权限结果不再由角色 code 硬编码拼接”。

---

## 阶段 6：用户故事 4 - 统一权限中间件阻止未授权请求（优先级：P1）

**目标**：租户角色管理、成员角色管理、策略读取/写入、组织读取/管理使用 permission code 授权，Service 继续保留资源归属和业务规则检查。

**独立测试**：无目标权限的成员调用受保护接口返回 403；授予自定义角色目标权限后通过权限中间件；资源跨租户仍被 Service 拦截。

- [ ] T025 [P] [US4] 策略和组织关键鉴权迁移测试，依赖：T009、T023；文件：`backend/internal/service/policy_service_test.go`、`backend/internal/service/org_attribute_service_test.go`、`backend/internal/service/org_management_service_test.go`、`backend/internal/handler/*_test.go`；验收：先写失败测试覆盖无 `policy.write` 不能写策略、有自定义角色 `policy.write` 可写当前租户策略、无 `tenant.org.manage` 不能改组织、跨租户资源不可见。
- [X] T026 [US4] 策略模块授权迁移，依赖：T007、T025；文件：`backend/internal/service/policy_service.go`、`backend/internal/handler/policy_handler.go`、`backend/internal/handler/router.go`；验收：策略读取使用 `policy.read`，策略创建/更新/删除/发布使用 `policy.write` 或 `policy.publish`，保留 owner、状态、租户归属等业务规则，新增逻辑不使用固定角色 code 做功能授权。
- [X] T027 [US4] 组织模块授权迁移，依赖：T007、T025；文件：`backend/internal/service/org_attribute_service.go`、`backend/internal/service/org_management_service.go`、`backend/internal/handler/org_attribute_handler.go`、`backend/internal/handler/org_management_handler.go`、`backend/internal/handler/router.go`；验收：组织读取使用 `tenant.org.read`，组织管理写操作使用 `tenant.org.manage`，组织职务 `ORG_LEADER/DEPUTY_LEADER` 只保留组织语义，不作为 RBAC 功能权限来源。
- [ ] T028 [US4] 旧 HasRole 和 ensureTenantManager 兼容包装收口，依赖：T026、T027；文件：`backend/internal/service/tenant_service.go`、`backend/internal/repository/tenant_repository.go`、`backend/internal/middleware/platform.go`、`backend/internal/service/platform_service.go`；验收：旧 `HasRole/ensureTenantManager` 仅用于最后管理员、内置角色分类、旧接口兼容等角色语义；功能授权包装内部调用 `RequireTenantPermission` 或 `RequirePlatformPermission`；代码搜索无新增固定角色 code 功能授权分支。
- [ ] T029 [P] [US4] 跨租户安全测试，依赖：T026、T027、T028；文件：`backend/internal/handler/rbac_handler_test.go`、`backend/internal/handler/policy_handler_test.go`、`backend/internal/handler/org_management_handler_test.go`；验收：租户 A 用户无法查询、修改、分配租户 B 的自定义角色、成员角色、策略或组织资源；所有相关 Repository 写查询均带可信 `tenant_id` 条件。

**检查点**：US4 可独立证明权限中间件和 AuthorizationService 已接管关键链路功能授权。

---

## 阶段 7：用户故事 5 - 数据迁移保持旧数据可用（优先级：P2）

**目标**：迁移后现有租户、成员和角色授权继续可用，旧常量保留兼容，重复执行迁移和 seed 不产生重复数据。

**独立测试**：在包含旧 `roles/user_roles`、旧单租户角色、历史 `DO/DU` 和兼容 `DATA_OWNER/DATA_VISITOR` 数据的库上重复执行迁移与 seed，验证权限和管理员不丢失。

- [ ] T030 [P] [US5] 迁移幂等与旧数据兼容集成测试，依赖：T003、T004；文件：`backend/internal/migrations/*_test.go`、`backend/migrations/010_tenant_rbac.sql`；验收：重复执行迁移和 seed 后，`roles(tenant_id, code)`、`permissions(code)`、`role_permissions(role_id, permission_id)` 无重复，已有 `TENANT_ADMIN/DO/DU` 用户获得默认权限，旧兼容值不导致迁移失败。
- [ ] T031 [US5] 开发环境迁移失败处理和回滚说明补齐，依赖：T003、T030；文件：`specs/008-tenant-rbac-backend/quickstart.md`、`backend/README.md`、`backend/migrations/010_tenant_rbac.sql`；验收：文档说明迁移前检查、失败定位、可恢复重跑、手工修复重复或孤立角色关系、迁移后验证 SQL；不声称 DDL 可完全事务回滚。
- [ ] T032 [US5] 迁移后基础功能回归测试，依赖：T024、T026、T027、T030；文件：`backend/internal/handler/auth_handler_test.go`、`backend/internal/handler/tenant_handler_test.go`、`backend/internal/handler/policy_handler_test.go`、`backend/internal/handler/org_attribute_handler_test.go`；验收：原有登录、租户切换、组织读取/管理、策略基础读写功能不回归，迁移后的现有租户管理员仍可管理租户。

**检查点**：US5 可独立证明迁移安全、可重跑、旧数据兼容。

---

## 最终阶段：收口、质量与文档

**目的**：完成全局测试、中文文档和注释可读性检查，确保没有越界实现。

- [ ] T033 全量 Repository 与 Service 测试收口，依赖：T014、T019、T024、T025、T030；文件：`backend/internal/repository/*_test.go`、`backend/internal/service/*_test.go`；验收：覆盖权限并集、状态失效、角色不可变、禁用行为、成员多角色、最后管理员保护、旧角色兼容、平台/租户权限隔离。
- [ ] T034 Handler、跨租户与迁移集成测试收口，依赖：T020、T023、T029、T032；文件：`backend/internal/handler/*_test.go`、`backend/internal/migrations/*_test.go`；验收：11 个 RBAC API 的成功、无权限、非法作用域、跨租户、重复 code、禁用角色、平台角色拒绝场景通过自动化测试。
- [ ] T035 关键注释和可读性检查，依赖：T012、T016、T022、T026、T027、T028；文件：`backend/internal/domain/*.go`、`backend/internal/repository/*.go`、`backend/internal/service/*.go`、`backend/internal/middleware/*.go`、`backend/internal/handler/*.go`；验收：新增或修改的 Go 函数/方法都有前置中文注释，导出标识符符合 GoDoc 前缀，实体字段、Handler、Service、Repository、Middleware 注释说明业务语义、副作用、事务边界、鉴权前提和安全边界，移除无意义逐行注释。
- [ ] T036 回归验证和文档更新，依赖：T033、T034、T035；文件：`specs/008-tenant-rbac-backend/quickstart.md`、`specs/008-tenant-rbac-backend/contracts/rbac-api.md`、`backend/README.md`；验收：`cd backend && go test ./...` 通过或记录不可运行原因；quickstart 中迁移、seed、API 验证步骤可执行；文档明确本阶段不实现前端页面、动态菜单、属性同步、策略版本、文件密钥和审计落库。

---

## 依赖与执行顺序

### 阶段依赖

- 阶段 1 无前置依赖。
- 阶段 2 依赖阶段 1，阻塞全部用户故事。
- US1、US2、US3、US4 均依赖阶段 2，可在基础能力完成后按团队容量并行，但推荐按 US1 -> US2 -> US3 -> US4 顺序集成。
- US5 依赖数据库迁移与基础回归能力，可与 US1-US4 的部分测试并行，但最终验收依赖关键接口完成。
- 最终阶段依赖所选用户故事全部完成。

### 用户故事依赖

- **US1 自定义角色管理**：依赖基础 Repository、AuthorizationService、Permission Middleware；是 MVP。
- **US2 成员多角色分配**：依赖基础 Repository 和 US1 中的自定义角色服务形态，但可用内置角色先行验证 `DO/DU` 叠加。
- **US3 当前用户授权上下文**：依赖 AuthorizationService，可与 US1/US2 部分并行。
- **US4 统一权限中间件阻止未授权请求**：依赖 Permission Middleware 和当前授权上下文，策略/组织迁移需在基础权限查询稳定后执行。
- **US5 数据迁移保持旧数据可用**：依赖显式迁移和 seed，最终需结合 US3/US4 证明迁移后权限链路可用。

### 并行机会

- T006、T008、T010 可在对应实现完成后分别并行补测试。
- T011 可与 T012 的服务设计并行准备，但合并前测试应先失败。
- T015、T019 可分别围绕成员多角色和最后管理员保护并行编写测试。
- T021 可与 US1/US2 后续 Handler 实现并行。
- T025、T029 可并行覆盖策略/组织和跨租户安全场景。
- T030 可在迁移完成后与业务接口实现并行验证幂等性。

---

## 并行执行示例

### 用户故事 1

```text
任务：T011 自定义角色 CRUD 和权限目录接口契约测试
任务：T012 租户角色服务实现自定义角色生命周期
```

### 用户故事 2

```text
任务：T015 成员多角色接口和 DO/DU 可叠加测试
任务：T019 最后一名 TENANT_ADMIN 并发保护测试
```

### 用户故事 4

```text
任务：T025 策略和组织关键鉴权迁移测试
任务：T029 跨租户安全测试
```

---

## 实施策略

### MVP 优先

1. 完成 T001 到 T010，建立模型、迁移、权限事实源、Repository、AuthorizationService 和 Permission Middleware。
2. 完成 US1 的 T011 到 T014，交付租户自定义角色生命周期和权限目录。
3. 暂停并独立验证：租户管理员能创建、修改、禁用自定义角色；普通成员不能管理角色；同租户重复 code 冲突；不同租户同 code 成功。

### 增量交付

1. MVP 后完成 US2，修复 `DO/DU` 互斥并交付成员多角色。
2. 完成 US3，让当前租户上下文返回真实角色和权限。
3. 完成 US4，将策略和组织关键链路迁移到 permission code。
4. 完成 US5 和最终阶段，验证迁移可重跑、旧数据兼容和全量回归。

### 范围控制

- 不创建 `tenant_roles` 重复角色表。
- 不实现前端页面、动态菜单、权限菜单、API 资源表或菜单数据库。
- 不实现角色继承、DENY、条件权限、数据范围表达式。
- 不实现组织和角色变化后的自动属性同步。
- 不实现策略版本、文件元数据、文件密钥封装、租户密钥、用户密钥材料、审计落库或解密事件。
- 不清理 `users.role` 等旧兼容字段，不删除 `DATA_OWNER/DATA_VISITOR` 等旧兼容常量。
- 不修改 CP-ABE 加解密流程。

---

## 独立验收摘要

- **US1**：通过 API 测试完成自定义角色创建、修改、禁用、权限目录查询和跨租户隔离。
- **US2**：通过 API 和 Repository/Service 测试完成成员多角色、`DO/DU` 叠加、平台角色拒绝、最后管理员保护。
- **US3**：通过授权上下文接口测试证明角色和权限来自数据库并集，撤销/过期/禁用不生效。
- **US4**：通过中间件和策略/组织测试证明无权限请求被阻止，有权限仍受资源归属规则约束。
- **US5**：通过迁移集成测试证明旧角色数据兼容、重复迁移幂等、现有管理员和 `DO/DU` 用户不丢权限。
