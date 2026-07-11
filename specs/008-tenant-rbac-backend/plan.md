# 实施计划：后端租户级 RBAC

**分支**：`008-tenant-rbac-backend` | **日期**：2026-07-10 | **规格**：[spec.md](./spec.md)

**输入**：来自 `specs/008-tenant-rbac-backend/spec.md` 的功能规格，以及本轮计划前对当前仓库的实际扫描结果。

## 摘要

本阶段只实现后端租户级 RBAC：演进现有 `roles` 和 `user_roles`，新增 `permissions` 与 `role_permissions`，以持久化权限点作为授权事实源；提供租户自定义业务角色、角色权限、成员多角色、当前用户授权上下文和统一权限中间件；迁移策略、组织、成员角色管理的关键鉴权逻辑；不实现前端页面，不修改 CP-ABE 解密流程，不扩展审计落库。

技术路线是沿用当前 Go + Gin + Gorm + MySQL 分层：领域模型放在 `backend/internal/domain`，仓储能力收敛在现有 `backend/internal/repository`，业务服务放在 `backend/internal/service`，中间件放在 `backend/internal/middleware`，接口放在 `backend/internal/handler` 和 `backend/internal/handler/router.go`；数据库变更必须通过幂等显式 SQL 迁移和 Gorm 模型同步共同完成，不能只依赖 `AutoMigrate`。

## 技术上下文

**语言/版本**：Go 1.23；前端现有 Electron + TypeScript 仅标注 API 类型影响，本阶段不改页面。

**主要依赖**：Gin、Gorm、MySQL driver、Redis 客户端、JWT 认证组件；测试使用 Go testing、httptest、Gin Test Mode、内存仓储和 miniredis。

**存储**：MySQL 为 RBAC 事实源；Redis 仅保留现有认证会话用途，本阶段不新增权限缓存。

**测试**：`go test ./...`；现有测试目录在 `backend/internal/**`，新增 Repository/迁移验证需要可连接 MySQL 的集成测试或等价数据库级测试说明。

**目标平台**：后端 HTTP 服务和显式迁移/seed 命令；桌面端只消费 API。

**项目类型**：桌面应用 + 后端 Web API；本阶段实际交付后端 API。

**性能目标**：权限判断以数据库索引查询为主，单个用户租户权限查询应使用一次角色权限聚合查询完成；不引入复杂缓存，避免撤销后失效问题。

**约束**：所有租户写查询必须带可信 `tenant_id`；平台权限和租户权限隔离；内置角色只读；成员多角色替换和最后管理员保护必须在事务内完成；文档与注释遵守中文规范。

**规模范围**：覆盖平台内置角色、租户内置角色、租户自定义业务角色、第一批 19 个权限点、11 个新增/调整 API、策略与组织关键鉴权迁移。

## 宪章检查

*GATE：计划前检查通过；Phase 1 设计后复核通过。*

- **混合加密边界**：本功能不触及 AES-GCM、RSA-OAEP、CP-ABE 或 DEK 封装实现；`file.decrypt.invoke` 只是调用解密接口的 RBAC 权限，不代表最终解密成功。
- **真实 CP-ABE 实现**：本功能不新增或替换 CP-ABE 库，不得使用 RBAC 或访问树模拟 CP-ABE 加解密。
- **模块边界**：RBAC 放在 domain/repository/service/middleware/handler；Policy 和 Org 只迁移功能授权入口；Crypto、File、Benchmark、Audit 不被本阶段扩展。空 `NoopAuditRecorder` 保持不变。
- **算法对比口径**：本功能不涉及 Benchmark 指标，不能混入 RSA/CP-ABE 性能结论。
- **可解释性**：当前用户授权接口返回真实角色和权限来源；策略解密解释仍由属性、策略快照、密钥材料和密码计算结果负责。
- **中文文档**：本 plan、research、data-model、contracts、quickstart 全部使用简体中文。
- **Go 注释策略**：实现阶段新增/修改的 Go 业务代码必须为所有函数/方法添加前置中文注释；导出标识符符合 GoDoc；实体字段、Handler、Service、Repository、Middleware 注释必须覆盖业务语义、事务边界、权限前置条件和安全边界。
- **关键注释和可读性检查**：后续 `tasks.md` 必须包含“关键注释和可读性检查”，并在交付说明明确函数/方法注释、GoDoc 前缀、核心业务语义和安全边界检查结果。

## 项目结构

### 本功能文档

```text
specs/008-tenant-rbac-backend/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── rbac-api.md
└── checklists/
    └── requirements.md
```

### 代码结构

```text
backend/
├── cmd/
│   ├── migrate/
│   └── seed/
├── migrations/
├── internal/
│   ├── domain/
│   ├── repository/
│   ├── service/
│   ├── middleware/
│   ├── handler/
│   ├── migrations/
│   └── pkg/response/
└── README.md

desktop/
└── src/renderer/src/
    ├── api/
    └── types.ts
```

**结构决策**：不新建顶层模块。RBAC 与当前租户、成员、策略、组织授权强相关，按现有后端分层扩展最贴合项目风格。若新增测试辅助目录，只建议在 `backend/internal/repository` 或 `backend/internal/migrations` 旁增加 `_test.go`，不引入独立 `tests/` 顶层目录。

## 1. 当前实现盘点

### 角色相关模型和表

| 名称 | 当前位置 | 表名 | 实际字段 | 当前职责 | 本阶段处理 |
|------|----------|------|----------|----------|------------|
| `domain.Role` | `backend/internal/domain/tenant.go` | `roles` | `id`、`code`、`name`、`scope`、`description`、`created_at`、`updated_at`、`deleted_at` | 平台/租户系统角色定义，`code` 当前全局唯一 | 演进为系统内置角色 + 租户自定义角色的唯一角色定义来源 |
| `domain.UserRoleAssignment` | `backend/internal/domain/tenant.go` | `user_roles` | `id`、`tenant_id`、`user_id`、`role_id`、`created_at`、`updated_at`、`deleted_at` | 用户角色授权；`tenant_id IS NULL` 表示平台级授权 | 演进为状态化授权记录，撤销不再物理删除 |
| `tenant_roles` | 未发现模型或 SQL 表 | 不存在 | 不存在 | 仅有 DTO 字段 `tenantRoles`，不是表 | 不新建重复角色表 |
| `domain.TenantUser` | `backend/internal/domain/tenant.go` | `tenant_users` | `id`、`tenant_id`、`user_id`、`status`、时间字段、`deleted_at` | 租户成员关系事实表 | 继续作为租户授权前置条件 |
| `domain.TenantOrgMemberRole` | `backend/internal/domain/org.go` | `tenant_org_member_roles` | 组织成员、部门和 `role_code` 等 | 组织职务，不是 RBAC 角色 | 不作为功能权限来源 |

### 当前 AutoMigrate 或迁移入口

- `backend/internal/migrations/automigrate.go`：`AutoMigrate` 当前同步 `User`、`Tenant`、`TenantUser`、`Role`、`UserRoleAssignment`、策略、组织和属性模型。
- `backend/cmd/migrate/main.go`：当前只调用 `migrations.AutoMigrate(db)`。
- `backend/migrations/*.sql`：存在显式 SQL 迁移，`008_tenant_org_management.sql` 已使用 `information_schema`、存储过程、幂等回填。
- `backend/README.md`：明确 HTTP 服务默认不执行 AutoMigrate 或 seed。

### 内置角色种子位置

- `PLATFORM_ADMIN`、`TENANT_ADMIN`、`DO`、`DU` 常量在 `backend/internal/domain/constants.go`。
- `TenantService.EnsureBaseRoles` 写入 4 个基础角色。
- `TenantService.BootstrapDefaultTenant` 调用 `EnsureBaseRoles`，并迁移历史用户默认租户关系。
- `cmd/seed/main.go` 调用 `BootstrapDefaultTenant`；`cmd/server/main.go` 在 `RUN_SEED=true` 时也调用。
- `PlatformRoleService.EnsurePlatformAdmin` 给用户写入 `tenant_id IS NULL` 的 `PLATFORM_ADMIN`。
- `PlatformRoleService.AssignTenantAdmin` 与 `CreateTenantAdminAccount` 写入租户内 `TENANT_ADMIN`。

### 当前角色分配接口及调用链

- `PUT /api/v1/tenants/:id/members/:userId/role`
- Handler：`TenantHandler.AssignTenantMemberRole`
- Service：`TenantService.AssignTenantMemberBusinessRole`
- Repository：`GormTenantRepository.ReplaceTenantBusinessRole`
- 角色输入：`DATA_OWNER` 或 `DO` 映射为 `DO`；`DATA_VISITOR` 或 `DU` 映射为 `DU`；拒绝 `TENANT_ADMIN`。
- 平台管理员会被 `AssignTenantMemberBusinessRole` 显式拒绝。

### 当前 DO/DU 互斥实现

- 具体方法：`backend/internal/repository/tenant_repository.go` 的 `ReplaceTenantBusinessRole`。
- 行为：查询 `DO/DU` 角色 ID，在事务中 `Unscoped().Delete` 删除目标用户当前租户内所有 `DO/DU` 授权，再插入新角色。
- 测试桩也复制该行为：`backend/internal/service/tenant_test_helpers_test.go` 和 `backend/internal/handler/test_helpers_test.go` 的 `ReplaceTenantBusinessRole`。

### 当前登录用户与 tenant_id 的可信来源

- 登录用户：`AuthRequired` 从 `Authorization: Bearer` access token 解析 claims，写入 `gin.Context` 的 `ContextUserID = "user_id"`；旧单租户角色写入 `ContextRole = "role"`。
- 租户选择输入：`TenantRequired` 从 `X-Tenant-Id` 读取用户选择。
- 可信租户上下文：`TenantRequired` 调用 `TenantService.ResolveTenantContext` 验证租户存在、启用、成员关系或当前平台管理员绕行逻辑；成功后写入 `ContextTenantID = "tenant_id"`、`ContextTenantCode`、`ContextTenantRoles`。
- 本阶段需要调整：平台管理员不能再仅凭平台身份进入租户业务上下文。

### 当前角色权限轻量映射

- 实现：`backend/internal/service/tenant_service.go` 的 `permissionsForRoles`。
- 当前映射：`PLATFORM_ADMIN -> platform:manage/tenant:switch`；`TENANT_ADMIN -> tenant:manage/org:manage/policy:view`；`DO -> policy:write/file:upload`；`DU -> file:read`。
- 当前登录、租户切换和上下文返回会调用它；本阶段必须改为真实 `permissions` + `role_permissions` 查询。

### 需要迁移的硬编码鉴权位置

- `backend/internal/middleware/platform.go`：`PlatformAdminRequired` 当前直接查 `PLATFORM_ADMIN`，平台路由可保留兼容包装但应迁移为 `platform.tenant.*` 等权限。
- `backend/internal/service/tenant_service.go`：`ensureTenantManager`、`ensurePlatformOrLegacyAdmin`、`AssignTenantMemberBusinessRole`、`permissionsForRoles`、`ResolveTenantContext` 平台管理员绕行逻辑。
- `backend/internal/service/policy_service.go`：`hasRole(actor.Roles, DO/TENANT_ADMIN)`、`canReadTenantPolicies`、策略创建/更新/删除的 `DO` 判断。
- `backend/internal/service/org_attribute_service.go`：`hasTenantRole(TENANT_ADMIN/DO/DU)`、`canReadOrgTree`、`canReadPolicyAttributes`、`canReadOwnAttributes`、`SyncUserAttributes`。
- `backend/internal/service/org_management_service.go`：`requireTenantAdmin`。
- `backend/internal/service/platform_service.go`：`hasRoleCode(member.Roles, TENANT_ADMIN)` 与最后管理员保护属于角色语义，可保留，但查询应适配新状态模型。
- `backend/internal/handler/policy_handler.go`、`org_attribute_handler.go`、`org_management_handler.go`：Actor 仍携带 `Roles`，需要补充或替换为权限上下文。
- 测试桩：`memoryTenantRepo`、`testTenantRepo` 中角色授权行为需要同步。

### 当前测试工具和目录

- 单元和 Handler 测试在 `backend/internal/**/**_test.go`。
- Handler 集成式测试使用 `newTestApp`、内存仓储、`httptest`、Gin Test Mode。
- Service 测试使用 `memoryUserRepo`、`memoryTenantRepo`、`memoryPolicyRepo`。
- Redis token store 测试使用 `miniredis`。
- 未发现 testcontainers、SQLite、sqlmock 或真实 MySQL Repository 测试。

### 可能受影响的前端 API 类型

本阶段不修改前端页面，但后端返回字段会影响：

- `desktop/src/renderer/src/types.ts`：`TenantRole` 当前只允许 `"PLATFORM_ADMIN" | "TENANT_ADMIN" | "DO" | "DU"`，无法表达自定义角色；`TenantMember.roles`、`TenantSummary.roles`、`TenantContextData.permissions`、`SwitchTenantData.permissions` 需要后续兼容。
- `desktop/src/renderer/src/api/tenant.ts`：仍调用旧 `assignTenantMemberRole(tenantId, userId, roleCode)` 单角色接口。
- `desktop/src/renderer/src/api/authStorage.ts`：`normalizeTenantRoles` 过滤掉非内置角色；`permissionsFromRoles` 仍按角色推导权限，后续需要在前端阶段移除。
- `RequireTenantRole`、`AppLayout`、`TenantMembersPage`、`TenantOrgManagementPage` 仍按固定角色判断页面/按钮，本阶段不改页面，但后端必须以权限为准。

## 2. 领域与数据库设计

### 最终模型关系

```text
roles 1 ── n user_roles n ── 1 users
roles 1 ── n role_permissions n ── 1 permissions
tenant_users 作为租户内 user_roles 生效的成员前置条件
```

### `roles`

- 字段：`id`、`tenant_id`、`code`、`name`、`description`、`scope_type`、`role_category`、`is_builtin`、`status`、`created_by`、`updated_by`、`created_at`、`updated_at`、保留 `deleted_at` 兼容 Gorm 但业务禁用优先用 `status`。
- 约束：`UNIQUE(tenant_id, code)`；`INDEX(tenant_id, status)`；建议 `INDEX(scope_type, role_category, status)`。
- 系统内置角色：`tenant_id=0` 且 `is_builtin=true`。其中 `PLATFORM_ADMIN` 为 `PLATFORM/GOVERNANCE`，`TENANT_ADMIN/DO/DU` 为 `TENANT/...`。
- 自定义角色：`tenant_id=当前租户`、`scope_type=TENANT`、`role_category=BUSINESS`、`is_builtin=false`。
- 平台角色隔离：`scope_type=PLATFORM` 的角色只允许平台授权链路使用，不出现在租户角色列表和成员角色分配接口中。
- role code 是否允许修改：创建后不允许修改。原因是 code 会进入审计、排障、前端缓存和接口语义，允许修改会破坏历史解释。

### `user_roles`

- 字段：`id`、`tenant_id`、`user_id`、`role_id`、`assignment_source`、`assigned_by`、`status`、`expires_at`、`revoked_at`、`created_at`、`updated_at`，保留 `deleted_at` 但撤销不再使用物理删除。
- 约束：`UNIQUE(tenant_id, user_id, role_id)`；`INDEX(tenant_id, user_id, status)`；`INDEX(tenant_id, role_id, status)`。
- 平台授权：`tenant_id=0` 作为事实值，替代当前 `NULL`，避免 MySQL 唯一索引对 `NULL` 不相等导致重复平台授权。迁移中需把 `NULL` 回填为 `0`。
- 租户授权：`tenant_id>0`，必须对应有效 `tenant_users`。
- 状态：`ACTIVE`、`REVOKED`、`EXPIRED`；过期可通过 `expires_at <= now` 动态判断，也可由后台任务后续回填 `EXPIRED`，本阶段查询必须兼容动态过期。

### `permissions`

- 字段：`id`、`code`、`name`、`description`、`scope_type`、`resource_type`、`action`、`status`、`created_at`、`updated_at`。
- 约束：`UNIQUE(code)`；`INDEX(scope_type, status)`。
- 不保存通配符权限。

### `role_permissions`

- 字段：`id`、`role_id`、`permission_id`、`granted_by`、`created_at`。
- 约束：`UNIQUE(role_id, permission_id)`；`INDEX(role_id)`；`INDEX(permission_id)`。
- 内置角色权限由 seed 管理；自定义角色权限由租户管理员管理。

### 查询用户权限的 SQL/Gorm 逻辑

租户作用域权限并集查询：

```sql
SELECT DISTINCT p.code
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
JOIN role_permissions rp ON rp.role_id = r.id
JOIN permissions p ON p.id = rp.permission_id
JOIN tenant_users tu ON tu.tenant_id = ur.tenant_id AND tu.user_id = ur.user_id
WHERE ur.tenant_id = ?
  AND ur.user_id = ?
  AND ur.status = 'ACTIVE'
  AND (ur.expires_at IS NULL OR ur.expires_at > CURRENT_TIMESTAMP(3))
  AND r.status = 'ACTIVE'
  AND r.scope_type = 'TENANT'
  AND (r.tenant_id = 0 OR r.tenant_id = ?)
  AND p.status = 'ACTIVE'
  AND p.scope_type = 'TENANT'
  AND tu.status = 'active';
```

平台作用域权限并集查询：

```sql
SELECT DISTINCT p.code
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
JOIN role_permissions rp ON rp.role_id = r.id
JOIN permissions p ON p.id = rp.permission_id
WHERE ur.tenant_id = 0
  AND ur.user_id = ?
  AND ur.status = 'ACTIVE'
  AND (ur.expires_at IS NULL OR ur.expires_at > CURRENT_TIMESTAMP(3))
  AND r.status = 'ACTIVE'
  AND r.scope_type = 'PLATFORM'
  AND r.tenant_id = 0
  AND p.status = 'ACTIVE'
  AND p.scope_type = 'PLATFORM';
```

### 有效角色和有效权限判断

- 有效角色：`user_roles.status=ACTIVE`、`roles.status=ACTIVE`、`expires_at IS NULL OR expires_at > now`。
- 租户有效角色还必须满足 `tenant_users.status=active`。
- 有效权限：`permissions.status=ACTIVE` 且通过有效角色绑定。
- 禁用角色后：不能被新分配；已绑定成员不产生权限；历史绑定保留；禁用自定义角色需返回当前有效绑定成员数。

### 最后一名 `TENANT_ADMIN` 并发保护

- 成员角色全量替换、禁用角色、撤销租户管理员、移除成员都必须在事务中执行。
- 事务内对当前租户成员/授权集合加锁，建议使用 `SELECT ... FOR UPDATE` 锁住当前租户 `tenant_users` 中所有 active `TENANT_ADMIN` 成员相关行，或锁住一个租户级哨兵行（如 `tenants` 当前租户行）后再统计。
- 推荐实现：事务开始后 `SELECT id FROM tenants WHERE id=? FOR UPDATE` 作为租户级串行化锁，再统计有效 `TENANT_ADMIN` 数量并执行变更；变更后再次统计，若为 0 则回滚并返回冲突。
- 这样不依赖单个用户行锁，能覆盖并发两人互相移除管理员的情况。

## 3. 数据迁移计划

### 使用的迁移机制

- 新增 `backend/migrations/010_tenant_rbac.sql`，使用当前显式 SQL 风格：`information_schema` 判断字段/索引是否存在，必要时使用存储过程保证幂等。
- 调整 `backend/cmd/migrate/main.go`：先执行显式 SQL 迁移文件，后执行 `AutoMigrate` 或在显式迁移成功后仅让 `AutoMigrate` 校验补齐非破坏字段。不能只调用 `AutoMigrate`。
- 同步更新 `backend/internal/migrations/automigrate.go` 的模型列表，加入 `Permission`、`RolePermission`。

### Schema 变更顺序

1. 给 `roles` 添加 `tenant_id`、`scope_type`、`role_category`、`is_builtin`、`status`、`created_by`、`updated_by`。
2. 回填 `roles.scope_type`：优先由旧 `scope` 映射，`platform -> PLATFORM`、`tenant -> TENANT`。
3. 回填 `role_category`、`is_builtin`、`status`：四个内置角色为内置，`PLATFORM_ADMIN/TENANT_ADMIN` 为 `GOVERNANCE`，`DO/DU` 为 `CAPABILITY`，状态 `ACTIVE`。
4. 给 `user_roles` 添加 `assignment_source`、`assigned_by`、`status`、`expires_at`、`revoked_at`。
5. 回填 `user_roles.tenant_id`：`NULL` 改为 `0`，表示平台授权。
6. 回填 `user_roles.status='ACTIVE'`。
7. 新建 `permissions`。
8. 新建 `role_permissions`。
9. 创建新索引和唯一约束。
10. 删除或替换旧唯一约束：`roles.uk_roles_code` 改为 `uk_roles_tenant_code`；`user_roles.uk_user_roles_tenant_user_role` 确认适用于 `tenant_id=0` 后保留或重建。

### 老数据回填顺序

1. 确保四个内置角色存在且 `tenant_id=0`。
2. 兼容旧 `scope` 字段，不立即删除；新代码读 `scope_type`，旧兼容代码可在过渡期由 `scope` 映射。
3. `DATA_OWNER`、`DATA_VISITOR` 仍作为兼容输入和值，不删除常量；迁移不强行改策略属性值。
4. 现有 `DO/DU/TENANT_ADMIN/PLATFORM_ADMIN` 授权转为 `ACTIVE`。
5. 检查租户内 `user_roles` 是否存在无对应 `tenant_users` 的记录；孤立记录不删除，先标记迁移报告并在权限查询中不生效。
6. 对没有任何有效 `TENANT_ADMIN` 的启用租户，迁移必须失败并报告租户 ID，要求人工修复；不得自动提升任意成员。

### 唯一约束修改顺序

- 添加 `tenant_id` 并回填完成后，先检查是否存在 `(tenant_id, code)` 重复；若存在，迁移失败。
- 创建新唯一索引 `uk_roles_tenant_code`。
- 确认新索引存在后再删除旧 `uk_roles_code`。
- `user_roles` 先把 `NULL tenant_id` 回填为 `0`，清理重复平台授权，再重建或确认唯一索引。

### DO/DU 互斥移除

- 删除新接口对 `ReplaceTenantBusinessRole` 的调用。
- Repository 新增成员角色全量替换方法，不再删除 `DO/DU` 互斥集合。
- 旧 `ReplaceTenantBusinessRole` 可暂时保留为 Deprecated 兼容包装，但实现应调用新的全量替换或明确只供旧接口过渡；过渡期间也不得再删除另一个能力角色。
- 同步修改内存测试仓储和 Handler 测试仓储。

### 权限和角色权限种子

- 权限 seed 使用 `INSERT ... ON DUPLICATE KEY UPDATE` 或 `WHERE NOT EXISTS`，按 `code` 幂等。
- 角色权限 seed 使用 `role.code + permission.code` 查询 ID 后 `INSERT IGNORE` 或 `ON DUPLICATE DO NOTHING`。
- 内置角色权限矩阵固定：
  - `PLATFORM_ADMIN`：`platform.tenant.read`、`platform.tenant.manage`、`platform.template.read`、`platform.template.manage`
  - `TENANT_ADMIN`：`tenant.dashboard.read`、`tenant.role.read`、`tenant.role.manage`、`tenant.member.read`、`tenant.member.manage`、`tenant.org.read`、`tenant.org.manage`、`policy.read`、`audit.read`
  - `DO`：`tenant.dashboard.read`、`policy.read`、`policy.write`、`policy.publish`、`file.read`、`file.upload`、`file.manage`
  - `DU`：`tenant.dashboard.read`、`file.read`、`file.decrypt.invoke`

### 事务和失败恢复

- `010_tenant_rbac.sql` 中字段添加和索引 DDL 在 MySQL 下可能隐式提交，因此要把“可失败检查”前置：重复数据、孤立高风险关系、无管理员租户先检查再执行破坏性约束变更。
- 数据回填和 seed 可以放在显式事务内；若 MySQL DDL 隐式提交，脚本必须保证幂等，失败后重复执行可继续。
- 开发环境已有数据处理：迁移前输出检查 SQL；失败时按租户/用户/角色 ID 报告冲突，人工修复后重复执行。
- 验证孤立关系：迁移后执行 `LEFT JOIN roles`、`LEFT JOIN users`、`LEFT JOIN tenants/tenant_users` 检查 `user_roles` 是否无法生效。
- 验证管理员：每个 enabled 租户至少有一名 active 成员绑定 active `TENANT_ADMIN`。

## 4. 后端分层计划

### domain

- 修改 `backend/internal/domain/constants.go`：新增 `RoleScopeType`、`RoleCategory`、`RoleStatus`、`UserRoleAssignmentStatus`、`PermissionStatus`、`PermissionScopeType`、`AssignmentSource` 等常量和校验方法；保留旧常量。
- 修改 `backend/internal/domain/tenant.go`：演进 `Role`、`UserRoleAssignment`；新增 `Permission`、`RolePermission`；新增角色/权限 DTO。

### repository

- 扩展 `backend/internal/repository/tenant_repository.go` 或新增 `rbac_repository.go`。若新增文件仍属于 `repository` 包，符合现有风格。
- 新增能力：角色列表、按租户查角色、创建/更新/禁用自定义角色、角色权限查询和替换、成员角色查询和替换、权限并集查询、权限判断、管理员数量、角色成员数量。
- 所有租户方法显式传入可信 `tenantID`。

### service

- 新增 `backend/internal/service/authorization_service.go`：统一授权服务。
- 新增或拆分 `TenantRoleService`：自定义角色 CRUD、角色权限维护、成员多角色。
- 修改 `TenantService`：上下文权限来源切换为 AuthorizationService；旧方法作为兼容包装。
- 修改 `PolicyService`、`OrgAttributeService`、`OrgManagementService`：关键入口迁移到 permission code。

### middleware

- 新增 `backend/internal/middleware/permission.go`：`PermissionRequired(permissionCode string)`、平台/租户作用域变体。
- 修改 `platform.go`：平台权限可通过 AuthorizationService 判断；保留 `PlatformAdminRequired` 兼容时内部调用平台权限。

### handler

- 新增 `backend/internal/handler/rbac_handler.go` 或 `tenant_role_handler.go`，注册当前租户 RBAC API。
- 修改 `tenant_handler.go`：当前用户授权上下文接口可放这里或新 Handler。
- 修改 `policy_handler.go`、组织 Handler Actor 组装，减少直接依赖角色列表做功能授权。

### router

- 修改 `backend/internal/handler/router.go`：在 `/api/v1/tenant` 当前租户组注册权限目录、角色、成员角色和授权上下文接口；为策略和组织关键写接口挂权限中间件。

### migration/seed

- 新增 `backend/migrations/010_tenant_rbac.sql`。
- 修改 `backend/internal/migrations/automigrate.go`。
- 修改 `backend/cmd/migrate/main.go`，让显式 SQL 迁移进入标准入口。
- 修改 `backend/cmd/seed/main.go` 或服务 seed，新增权限和角色权限幂等初始化。

### DTO

- 在 domain 或 handler 内新增请求/响应 DTO：`PermissionDTO`、`TenantRoleDTO`、`RolePermissionDTO`、`MemberRoleDTO`、`AuthorizationContextDTO`。

### error code

- 扩展 `backend/internal/pkg/response/errors.go`：`PERMISSION_DENIED`、`ROLE_NOT_FOUND`、`ROLE_CODE_EXISTS`、`BUILTIN_ROLE_IMMUTABLE`、`ROLE_DISABLED`、`INVALID_ROLE_SCOPE`、`INVALID_PERMISSION_SCOPE`、`MEMBER_NOT_FOUND_IN_TENANT`、`CANNOT_ASSIGN_PLATFORM_ROLE`、`CANNOT_REMOVE_LAST_TENANT_ADMIN`、`CROSS_TENANT_ACCESS_DENIED` 等。

### tests

- 扩展内存仓储测试桩。
- Service 单元测试：授权服务、角色管理、成员多角色、兼容包装。
- Handler/API 测试：11 个接口与权限中间件。
- Repository/迁移集成测试：建议使用真实 MySQL 测试库；若 CI 暂无 MySQL，至少提供可手动运行的迁移验证脚本和 SQL 检查。

## 5. AuthorizationService 设计

### 接口签名

```go
type AuthorizationService struct {
    rbac repository.RBACRepository
}

func (s *AuthorizationService) PlatformPermissions(ctx context.Context, userID uint64) ([]string, error)
func (s *AuthorizationService) TenantPermissions(ctx context.Context, userID uint64, tenantID uint64) ([]string, error)
func (s *AuthorizationService) HasPlatformPermission(ctx context.Context, userID uint64, code string) (bool, error)
func (s *AuthorizationService) HasTenantPermission(ctx context.Context, userID uint64, tenantID uint64, code string) (bool, error)
func (s *AuthorizationService) RequirePlatformPermission(ctx context.Context, userID uint64, code string) error
func (s *AuthorizationService) RequireTenantPermission(ctx context.Context, userID uint64, tenantID uint64, code string) error
func (s *AuthorizationService) CurrentTenantAuthorization(ctx context.Context, userID uint64, tenantID uint64) (domain.AuthorizationContextDTO, error)
func (s *AuthorizationService) UserTenantAuthorization(ctx context.Context, tenantID uint64, targetUserID uint64) (domain.AuthorizationContextDTO, error)
```

### 当前用户授权与指定用户查询

- 当前用户授权：只从 `gin.Context` 中的可信 `ContextUserID` 和 `ContextTenantID` 组装；用于中间件和 `/tenant/me/authorization`。
- 指定用户查询：用于管理员查看成员角色/权限；必须先由调用方通过 `tenant.member.read` 或 `tenant.member.manage`，再按 `tenantID + targetUserID` 查询。

### 可信 user_id 和 tenant_id

- `userID` 来自 `AuthRequired` 已验证 token。
- `tenantID` 来自 `TenantRequired` 校验后写入的 `ContextTenantID`。
- Handler 不接受请求体或 query 中的 `tenant_id` 作为实际租户边界。

### 平台作用域

- 平台权限只查询 `user_roles.tenant_id=0`、`roles.scope_type=PLATFORM`、`permissions.scope_type=PLATFORM`。
- `PLATFORM_ADMIN` 不参与租户权限并集。

### 租户作用域

- 租户权限必须校验 `tenant_users.status=active`。
- 系统内置租户角色通过 `roles.tenant_id=0 AND roles.scope_type=TENANT` 被所有租户复用。
- 自定义角色通过 `roles.tenant_id=当前租户` 隔离。

### 缓存

- 本阶段不新增 Redis 或内存权限缓存。原因是角色撤销、禁用、权限替换必须即时生效，现有项目没有成熟权限缓存失效机制。

### 避免循环依赖

- AuthorizationService 只依赖 Repository，不依赖 TenantService、PolicyService 或 OrgService。
- Permission Middleware 依赖 AuthorizationService。
- TenantService 可调用 AuthorizationService 计算上下文权限，但 AuthorizationService 不反向调用 TenantService。

### 旧方法兼容迁移

- `TenantRepository.HasRole` 可保留给最后管理员保护、内置角色语义判断和旧接口兼容；新功能不再用它做功能授权。
- `ensureTenantManager` 改为兼容包装，内部调用 `RequireTenantPermission("tenant.member.manage")` 或对应权限；平台旧 admin 兼容仅限旧平台治理接口，不能绕过租户业务授权。
- `permissionsForRoles` 废弃；登录和切换租户返回权限改为 AuthorizationService 查询。前端过渡期若仍接收旧字段，字段名可保留但值来自数据库。

### 必须保留角色语义的判断

- 是否移除最后一个 `TENANT_ADMIN`。
- 目标角色是否为系统内置、平台角色、租户内置能力角色或租户自定义业务角色。
- 自定义角色是否允许修改权限。
- `DO/DU` 是否为能力角色，可叠加但不用于功能授权硬编码。
- 组织职务 `ORG_LEADER/DEPUTY_LEADER` 仍是组织属性语义，不是 RBAC 功能权限。

## 6. API 契约

完整契约见 [contracts/rbac-api.md](./contracts/rbac-api.md)。概要如下：

| Method | Path | 请求 DTO | 响应 DTO | 权限 | 事务边界 | tenant_id 校验 |
|--------|------|----------|----------|------|----------|----------------|
| GET | `/api/v1/tenant/permissions` | 无 | `PermissionCatalogResponse` | `tenant.role.read` 或 `tenant.role.manage` | 只读 | `TenantRequired` |
| GET | `/api/v1/tenant/roles` | query 可选 `status` | `TenantRoleListResponse` | `tenant.role.read` | 只读 | `TenantRequired` |
| POST | `/api/v1/tenant/roles` | `CreateTenantRoleRequest` | `TenantRoleDetailResponse` | `tenant.role.manage` | 创建角色与可选权限同事务 | `TenantRequired` |
| GET | `/api/v1/tenant/roles/:roleId` | path `roleId` | `TenantRoleDetailResponse` | `tenant.role.read` | 只读 | 按系统租户角色或当前租户角色校验 |
| PATCH | `/api/v1/tenant/roles/:roleId` | `UpdateTenantRoleRequest` | `TenantRoleDetailResponse` | `tenant.role.manage` | 单角色更新事务 | 同上 |
| DELETE | `/api/v1/tenant/roles/:roleId` | 无 | `DisableRoleResponse` | `tenant.role.manage` | 禁用和最后管理员检查同事务 | 同上 |
| GET | `/api/v1/tenant/roles/:roleId/permissions` | path `roleId` | `RolePermissionResponse` | `tenant.role.read` | 只读 | 同上 |
| PUT | `/api/v1/tenant/roles/:roleId/permissions` | `ReplaceRolePermissionsRequest` | `RolePermissionResponse` | `tenant.role.manage` | 全量替换同事务 | 只允许当前租户自定义角色 |
| GET | `/api/v1/tenant/members/:userId/roles` | path `userId` | `MemberRoleResponse` | `tenant.member.read` | 只读 | 目标成员必须属于当前租户 |
| PUT | `/api/v1/tenant/members/:userId/roles` | `ReplaceMemberRolesRequest` | `MemberRoleResponse` | `tenant.member.manage` | 全量替换、最后管理员检查同事务 | 目标成员和角色都按当前租户校验 |
| GET | `/api/v1/tenant/me/authorization` | 无 | `AuthorizationContextResponse` | 已登录且有效租户成员 | 只读 | `TenantRequired` |

主要错误码：`PERMISSION_DENIED`(403)、`ROLE_NOT_FOUND`(404)、`ROLE_CODE_EXISTS`(409)、`BUILTIN_ROLE_IMMUTABLE`(400/409)、`ROLE_DISABLED`(400)、`INVALID_ROLE_SCOPE`(400)、`INVALID_PERMISSION_SCOPE`(400)、`MEMBER_NOT_FOUND_IN_TENANT`(404)、`CANNOT_ASSIGN_PLATFORM_ROLE`(400/403)、`CANNOT_REMOVE_LAST_TENANT_ADMIN`(409)、`CROSS_TENANT_ACCESS_DENIED`(404/403)。

## 7. 实施顺序

1. **领域常量和模型**  
   完成条件：`Role`、`UserRoleAssignment`、`Permission`、`RolePermission` 及状态/分类常量可编译，旧常量保留。

2. **显式数据库迁移**  
   完成条件：`010_tenant_rbac.sql` 可重复执行；迁移前检查、字段添加、回填、索引调整、孤立关系检查都有明确脚本。

3. **权限和角色权限种子**  
   完成条件：重复执行 seed 不产生重复权限或绑定；四个内置角色权限矩阵符合规格。

4. **Repository**  
   完成条件：所有 RBAC 查询/写入方法具备 `tenantID` 边界；成员角色全量替换事务内保护最后管理员；权限并集查询去重。

5. **AuthorizationService**  
   完成条件：平台/租户权限查询、权限判断、当前用户授权上下文、指定用户授权查询全部有单元测试。

6. **Permission Middleware**  
   完成条件：无权限请求返回 403；认证和租户上下文职责不被混入权限中间件。

7. **自定义角色 CRUD**  
   完成条件：创建、详情、列表、修改、禁用通过 Handler 测试；内置角色不可变；跨租户不可见。

8. **角色权限接口**  
   完成条件：自定义角色可全量替换租户权限，拒绝平台权限和禁用权限，空数组清空。

9. **成员多角色接口**  
   完成条件：同一成员可同时有 `DO/DU`；拒绝平台角色；禁用角色不可分配；不能移除最后管理员。

10. **当前用户授权上下文接口**  
    完成条件：返回真实角色和权限；`REVOKED/EXPIRED/DISABLED` 不生效；不再调用 `permissionsForRoles`。

11. **策略和组织模块关键鉴权迁移**  
    完成条件：策略读写、组织读写使用 permission code；资源归属和 owner 边界仍在 Service 保留。

12. **兼容层**  
    完成条件：旧接口可过渡但不再制造 `DO/DU` 互斥；旧 `HasRole` 只用于角色语义和兼容包装。

13. **单元测试**  
    完成条件：Service、Middleware、DTO 校验、权限并集、错误映射覆盖核心场景。

14. **Repository 集成测试**  
    完成条件：真实 MySQL 或等价环境验证唯一约束、事务、并发最后管理员保护、迁移重复执行。

15. **Handler/API 集成测试**  
    完成条件：11 个 API 的成功、无权限、跨租户、禁用/撤销/过期角色场景覆盖。

16. **回归测试**  
    完成条件：`go test ./...` 通过；登录、租户切换、组织基础功能、策略基础功能不回归。

## 8. 风险分析

| 风险 | 影响 | 缓解 |
|------|------|------|
| 数据迁移破坏现有角色绑定 | 已有用户失去租户管理或文件能力 | 迁移前检查、回填后对照、重复执行测试、迁移后管理员和 DO/DU 权限验证 |
| AutoMigrate 无法安全修改唯一约束 | 约束残留或重复数据导致线上失败 | 显式 SQL 控制字段、索引和回填顺序，AutoMigrate 只做模型同步 |
| DO/DU 旧互斥逻辑残留 | 成员无法同时拥有两种能力 | 废弃 `ReplaceTenantBusinessRole`，同步修改测试桩，增加分配顺序测试 |
| 平台管理员意外获得租户权限 | 平台治理越权到租户业务数据 | `ResolveTenantContext` 移除平台绕行；租户权限查询必须要求 `tenant_users` |
| 自定义角色跨租户越权 | 租户 A 修改或分配租户 B 角色 | 所有角色查询条件限定 `(tenant_id=0 TENANT 内置) OR tenant_id=当前租户` |
| 最后一名管理员并发删除 | 租户无人治理 | 事务内锁租户行并前后统计有效管理员 |
| 权限查询性能 | 高频接口多次 join | 使用索引和一次 DISTINCT 查询；暂不缓存，后续按监控决定 |
| 角色撤销后权限未及时失效 | 已撤销用户仍可操作 | 不做缓存；每次权限中间件查数据库事实源 |
| 旧权限映射与新权限结果并存 | 前后端看到不一致权限 | 保留字段名但后端值来自新权限查询；标记 `permissionsForRoles` 废弃 |
| 测试环境与生产数据库差异 | 内存测试无法发现 MySQL 约束问题 | 增加 MySQL 迁移/Repository 集成验证；内存仓储只覆盖业务分支 |

## 复杂度跟踪

无宪章违规项。引入显式 SQL 迁移、Repository 扩展和权限服务是本功能的必要复杂度，因为当前 `AutoMigrate` 和硬编码角色无法安全表达状态化、多角色、租户自定义角色和权限并集。
