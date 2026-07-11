# 功能规格：后端租户级 RBAC

**功能分支**：`008-tenant-rbac-backend`

**创建日期**：2026-07-10

**状态**：草稿

**输入**：用户描述：“本阶段只完成后端 RBAC 能力，包括数据库迁移、权限种子、自定义角色接口、成员多角色接口和统一授权服务。本阶段不实现前端页面。”

## 当前项目现状盘点

### 角色与成员授权表

- 当前不存在独立的 `tenant_roles` 授权事实表；代码中出现的 `tenantRoles` 是租户上下文 DTO 字段，不是数据库表。
- `roles` 当前由 `backend/migrations/002_create_tenants_roles.sql` 创建，字段为 `id`、`code`、`name`、`scope`、`description`、时间字段和软删除字段；`code` 通过 `uk_roles_code` 全局唯一，尚不支持租户自定义角色、角色分类、内置标记、状态和创建更新人。
- `user_roles` 当前由同一迁移创建，字段为 `id`、可空 `tenant_id`、`user_id`、`role_id`、时间字段和软删除字段；`tenant_id IS NULL` 表示平台级角色，非空表示租户内角色。当前撤销角色会物理删除记录以释放唯一约束，不具备 `ACTIVE/REVOKED/EXPIRED` 状态语义。
- `tenant_users` 是用户与租户的成员关系事实表，当前成员状态为 `active/disabled`，成员列表和租户上下文都会依赖它。
- `tenant_org_member_roles` 是组织职务表，不是 RBAC 角色表。该表在 008 组织迁移中已将旧 `DATA_OWNER/DATA_VISITOR` 职务迁移到 `user_roles` 的 `DO/DU`，并把组织职务收敛为 `ORG_LEADER/DEPUTY_LEADER`。

### 内置角色和初始化方式

- 角色常量定义在 `backend/internal/domain/constants.go`：`PLATFORM_ADMIN`、`TENANT_ADMIN`、`DO`、`DU`，同时仍保留旧单租户 `admin/data_owner/data_user`。
- `TenantService.EnsureBaseRoles` 幂等写入 `PLATFORM_ADMIN`、`TENANT_ADMIN`、`DO`、`DU`，当前只写入 `code/name/scope/description`。
- `cmd/seed` 调用 `TenantService.BootstrapDefaultTenant` 初始化基础角色、演示租户和历史用户默认租户关系。
- `EnsureUserInDefaultTenant` 和批量迁移会把旧单租户角色映射为默认租户内角色：`admin -> TENANT_ADMIN`、`data_owner -> DO`、其他默认 `DU`；已经拥有平台级 `PLATFORM_ADMIN` 的用户会跳过默认租户迁移。
- `PlatformRoleService.EnsurePlatformAdmin` 会给指定用户写入 `tenant_id IS NULL` 的 `PLATFORM_ADMIN`。
- `PlatformRoleService.AssignTenantAdmin` 和 `CreateTenantAdminAccount` 会先确认或创建租户成员，再授予该租户内 `TENANT_ADMIN`。

### 角色查询、分配和撤销调用链

- 平台授权中间件 `PlatformAdminRequired` 通过 `TenantRepository.HasRole(userID, nil, PLATFORM_ADMIN)` 判断平台后台访问。
- 租户上下文中间件 `TenantRequired` 从 `X-Tenant-Id` 读取租户选择，调用 `TenantService.ResolveTenantContext` 后写入 `gin.Context`：`tenant_id`、`tenant_code`、`tenant_roles`。
- 当前 `ResolveTenantContext` 会让平台管理员直接进入任意启用租户并返回 `PLATFORM_ADMIN` 作为租户上下文角色；这与本阶段目标“平台管理员进入租户业务上下文仍必须满足该租户成员与角色规则”冲突，需迁移。
- 租户成员添加接口当前会调用 `EnsureTenantUser` 并循环调用 `EnsureUserRole` 授予请求中的租户角色。
- 租户成员普通业务角色接口当前为 `PUT /api/v1/tenants/:id/members/:userId/role`，只接受单个 `roleCode`，通过 `AssignTenantMemberBusinessRole` 调用 `ReplaceTenantBusinessRole`。
- `ReplaceTenantBusinessRole` 当前在事务中删除旧 `DO/DU` 再写入新角色，明确实现了 `DO/DU` 互斥；本阶段必须废弃该行为。
- 租户管理员撤销当前通过 `RemoveUserRole` 物理删除 `user_roles` 记录；最后租户管理员保护通过 `CountTenantAdmins` 统计有效成员中的 `TENANT_ADMIN`。

### 当前硬编码授权点

- `tenant_service.go` 中 `ensureTenantManager` 先允许平台或旧 `admin`，再检查指定租户 `TENANT_ADMIN`；租户成员列表、成员添加、成员移除和租户详情依赖该逻辑。
- `tenant_service.go` 中 `permissionsForRoles` 按固定角色拼接前端轻量权限，例如 `TENANT_ADMIN -> tenant:manage/org:manage/policy:view`、`DO -> policy:write/file:upload`、`DU -> file:read`；这不是持久化权限事实源。
- `policy_service.go` 中策略读写按 `TENANT_ADMIN/DO` 分支判断：`DO` 可写自己的策略，`TENANT_ADMIN` 只读租户策略。
- `org_attribute_service.go` 中组织树、组织成员、策略属性和属性同步按 `TENANT_ADMIN/DO/DU` 分支判断。
- `org_management_service.go` 的当前租户组织管理接口通过 `requireTenantAdmin` 硬编码 `TENANT_ADMIN`。
- `platform_service.go` 中最后租户管理员保护仍需要角色语义判断，这是允许保留的领域规则，不应改成普通权限点。

### 当前路由结构

- `/api/v1/platform` 已存在并使用 `AuthRequired + PlatformAdminRequired`，覆盖平台 dashboard、租户、平台成员和平台公共策略属性/模板。
- `/api/v1/tenant` 已存在当前租户上下文式组织管理路由，使用 `AuthRequired + TenantRequired`，真实租户来自 `X-Tenant-Id` 后端校验结果。
- `/api/v1/tenants/:id` 仍承载旧式租户、成员、策略和组织属性接口，其中部分写接口已有 Deprecated 注释；策略接口仍使用路径租户 ID，并由 `TenantRequired` 校验路径 ID 与上下文 ID 一致。
- 本阶段新增 RBAC 接口应优先落在 `/api/v1/tenant` 当前租户上下文下；不得让前端通过请求体或 query 参数决定真实 `tenant_id`。

### 当前迁移和测试机制

- 项目同时存在显式 SQL 迁移文件和 Gorm `AutoMigrate`。`backend/README.md` 明确 HTTP 服务默认不执行 `AutoMigrate` 或 seed，首次部署或表结构变化时显式执行 `go run ./cmd/migrate`，基础数据通过 `go run ./cmd/seed` 写入。
- `cmd/migrate` 当前只调用 `internal/migrations.AutoMigrate`；显式 SQL 文件位于 `backend/migrations/`，其中 007、008 等脚本使用 `information_schema` 和存储过程表达幂等迁移。
- 本阶段不能只依赖 `AutoMigrate`，必须补充幂等显式 SQL 迁移，并保持 `AutoMigrate` 的结构定义与迁移结果一致。
- 当前测试主要是 Go 单元测试和 Handler 集成式测试，使用内存仓储、`httptest` 和 Gin Test Mode；没有发现真实 MySQL Repository 测试或迁移重复执行测试。本阶段由于涉及唯一约束、事务、状态回填和并发保护，需要补齐数据库级验证或等价的自动化集成测试。

## 用户场景与测试 *(必填)*

### 用户故事 1 - 租户管理员维护自定义角色（优先级：P1）

租户管理员需要在企业内部创建、修改、禁用业务角色，并为这些角色配置可选租户权限，使企业可以按部门、职能或岗位表达管理能力，而不是把所有功能绑定到固定系统角色上。

**为什么优先**：这是租户级 RBAC 的核心价值，也是从固定角色迁移到持久化权限点的基础。

**独立测试**：使用一个拥有 `tenant.role.manage` 权限的租户成员，在当前租户上下文中创建自定义角色、修改展示信息、替换权限集合并禁用角色，可完整验证角色生命周期。

**验收场景**：

1. **假设** 当前用户在租户 A 拥有角色管理权限，**当** 创建角色 `SRE_ENGINEER` 并提交权限 `tenant.org.read`、`policy.read`、`file.upload`，**那么** 系统创建 `TENANT + BUSINESS + 非内置` 角色，并返回真实权限绑定结果。
2. **假设** 租户 A 已存在自定义角色 `SRE_ENGINEER`，**当** 租户 A 再次创建相同 code，**那么** 请求失败并返回角色 code 冲突。
3. **假设** 租户 A 已存在自定义角色 `SRE_ENGINEER`，**当** 租户 B 创建相同 code，**那么** 请求成功。
4. **假设** 当前用户试图在创建请求中指定平台作用域、治理分类、能力分类或 `tenant_id`，**当** 请求提交，**那么** 系统忽略或拒绝这些客户端字段，只允许创建 `TENANT + BUSINESS` 自定义角色。
5. **假设** 目标角色是系统内置角色，**当** 租户管理员尝试修改其 code、作用域、分类或权限，**那么** 系统拒绝并返回内置角色不可变。

---

### 用户故事 2 - 租户管理员给成员分配多个角色（优先级：P1）

租户管理员需要一次性替换某个成员在当前租户内的完整角色集合，使同一成员可以同时拥有治理角色、能力角色和自定义业务角色。

**为什么优先**：成员多角色是权限并集计算的前提，也直接修复当前 `DO/DU` 互斥问题。

**独立测试**：给同一成员同时分配 `DO`、`DU` 和自定义业务角色，再查询成员角色与权限集合，确认所有有效权限正确合并。

**验收场景**：

1. **假设** 成员属于当前租户且状态有效，**当** 管理员提交完整角色集合 `[DO, DU, 自定义角色]`，**那么** 系统同时保留 `DO` 和 `DU`，不会互相删除。
2. **假设** 成员当前拥有 `DO`，**当** 管理员为其追加或替换为包含 `DU` 的完整集合，**那么** 原 `DO` 仅在新集合不包含时才撤销。
3. **假设** 请求包含 `PLATFORM_ADMIN` 或平台作用域角色，**当** 管理员提交成员角色更新，**那么** 系统拒绝分配平台角色。
4. **假设** 请求包含已禁用角色，**当** 管理员提交成员角色更新，**那么** 系统拒绝该角色且不产生部分更新。
5. **假设** 操作会移除当前租户最后一名有效 `TENANT_ADMIN`，**当** 管理员提交角色替换或禁用相关角色，**那么** 系统拒绝并保持原角色集合。

---

### 用户故事 3 - 当前用户获得真实授权上下文（优先级：P1）

当前登录用户进入某个租户后，需要从后端获得真实角色集合和权限集合，用于后端授权、后续前端展示和自动化测试验证。

**为什么优先**：权限结果必须来自数据库权限事实源，否则无法证明 RBAC 已替代固定角色拼接。

**独立测试**：为用户配置多个有效角色、撤销角色、过期角色和禁用角色，调用当前用户授权上下文接口，确认返回权限只来自有效角色权限并集。

**验收场景**：

1. **假设** 用户拥有多个有效角色且角色权限有重叠，**当** 查询当前租户授权上下文，**那么** 返回所有有效角色和去重后的权限 code 集合。
2. **假设** 用户存在 `REVOKED`、`EXPIRED` 或绑定到 `DISABLED` 角色的授权记录，**当** 查询权限集合，**那么** 这些记录不产生权限。
3. **假设** 用户是平台管理员但不是目标租户有效成员，**当** 进入租户授权上下文，**那么** 不返回租户业务权限。
4. **假设** 用户同时拥有 `DO` 和 `DU`，**当** 查询权限集合，**那么** 同时包含数据拥有者和数据使用者能力权限。

---

### 用户故事 4 - 后端统一权限中间件阻止未授权请求（优先级：P1）

后端需要通过统一授权服务和权限中间件判断功能权限，避免 Service、Handler 或 Middleware 中继续新增固定角色 code 授权分支。

**为什么优先**：这是服务端真实 RBAC 的安全边界，直接影响策略、组织和成员管理接口。

**独立测试**：使用没有目标权限的成员调用受保护接口应被拒绝；授予自定义角色目标权限后同一用户可通过授权，但仍需满足资源归属等业务规则。

**验收场景**：

1. **假设** 用户没有 `tenant.role.manage`，**当** 调用创建角色接口，**那么** 请求返回权限不足。
2. **假设** 用户没有 `policy.write`，**当** 调用策略创建或修改接口，**那么** 请求返回权限不足。
3. **假设** 用户通过自定义角色获得 `policy.write`，**当** 修改当前租户内允许修改的策略，**那么** 权限中间件放行，Service 继续校验资源归属和策略状态。
4. **假设** 用户没有 `tenant.org.manage`，**当** 调用组织写接口，**那么** 请求返回权限不足。
5. **假设** 用户拥有功能权限但资源不属于当前租户，**当** 调用资源详情或修改接口，**那么** 系统返回租户范围内不可见的错误，避免泄露其他租户数据。

---

### 用户故事 5 - 数据迁移保持旧数据可用（优先级：P2）

已有租户、成员和角色授权需要在迁移后继续可用，旧角色常量和旧字段在本阶段保持兼容，但新的授权结果必须由权限表和角色权限绑定计算。

**为什么优先**：RBAC 表结构变化较大，必须保证现有登录、租户切换、组织和策略基础功能不回归。

**独立测试**：在包含旧角色、旧 `user_roles`、组织职务迁移结果和历史用户的数据库上重复执行迁移与 seed，确认不会重复创建数据且现有管理员不丢权限。

**验收场景**：

1. **假设** 迁移前存在 `TENANT_ADMIN`、`DO`、`DU` 授权，**当** 迁移完成，**那么** 这些用户仍获得对应默认权限。
2. **假设** 迁移前存在 `DATA_OWNER/DATA_VISITOR` 兼容值，**当** 迁移完成，**那么** 能映射或保留兼容并产生 `DO/DU` 默认能力。
3. **假设** 重复执行迁移和 seed，**当** 检查角色、权限和角色权限关系，**那么** 不出现重复记录。
4. **假设** 迁移过程遇到不可自动处理的数据冲突，**当** 迁移失败，**那么** 失败信息可定位原因，并且事务或可恢复设计保证不会留下半迁移授权状态。

### 边界情况

- 平台管理员只有平台权限，不自动拥有任何租户业务权限。
- 平台权限不能绑定到租户自定义角色。
- 禁用自定义角色后，已绑定成员不再通过该角色获得权限，也不能被新分配给成员。
- 空权限数组表示清空自定义角色权限，但不删除角色。
- 空角色数组通常表示撤销成员所有可撤销角色；如果目标成员是最后一名 `TENANT_ADMIN`，必须拒绝。
- 角色 ID 或权限 code 属于其他租户、平台作用域或禁用状态时，必须拒绝或按租户不可见处理。
- 并发移除或替换租户管理员角色时，不能让租户最终没有有效管理员。
- `file.decrypt.invoke` 只代表允许调用解密流程，不代表 CP-ABE 策略一定满足。
- 自定义角色名称和 code 不得自动同步为 CP-ABE 属性。
- 本阶段不因 `audit_recorder.go` 为空实现而扩展审计落库域。

## 需求 *(必填)*

### 功能需求

- **FR-001**：系统必须将平台角色和租户角色的作用域分离；平台管理员只能获得平台治理权限，进入租户业务上下文时必须满足目标租户的成员与角色规则。
- **FR-002**：系统必须支持角色的 `scope_type` 和 `role_category` 两个维度，并把内置角色归类为：`PLATFORM_ADMIN=PLATFORM+GOVERNANCE`、`TENANT_ADMIN=TENANT+GOVERNANCE`、`DO=TENANT+CAPABILITY`、`DU=TENANT+CAPABILITY`。
- **FR-003**：系统必须支持租户自定义业务角色，且自定义角色只能是 `TENANT+BUSINESS`、`is_builtin=false`、属于当前租户。
- **FR-004**：系统必须演进现有 `roles` 表作为唯一角色定义来源，不得新建与现有模型职责重复的角色表。
- **FR-005**：系统必须演进现有 `user_roles` 表支持授权来源、状态、过期时间、撤销时间和重新激活语义；撤销角色不得物理删除。
- **FR-006**：系统必须新增权限目录，并保证权限 code 全局唯一、作用域明确、状态可控。
- **FR-007**：系统必须新增角色权限绑定，并通过角色权限绑定计算用户有效权限并集；权限结果必须去重。
- **FR-008**：系统必须提供幂等权限种子，初始化平台权限、租户权限、内置角色和内置角色权限矩阵。
- **FR-009**：`PLATFORM_ADMIN` 默认只绑定平台权限，不得自动绑定租户成员、组织、策略、文件或审计业务权限。
- **FR-010**：`TENANT_ADMIN` 默认绑定租户治理权限，但不得因为治理身份自动获得 `DO/DU` 文件能力。
- **FR-011**：`DO` 默认绑定数据拥有者能力，覆盖策略写入、策略发布、文件读取、上传和管理等权限。
- **FR-012**：`DU` 默认绑定数据使用者能力，覆盖工作台读取、文件读取和解密流程调用权限。
- **FR-013**：系统必须允许同一成员在同一租户拥有多个角色，包括 `DO+DU`、自定义业务角色与能力角色叠加、`TENANT_ADMIN+其他角色`。
- **FR-014**：系统必须删除或废弃当前分配 `DO` 时删除 `DU`、分配 `DU` 时删除 `DO` 的业务行为。
- **FR-015**：系统必须提供当前租户可选权限目录查询，只返回租户权限，不返回平台权限。
- **FR-016**：系统必须提供当前租户角色列表，包含系统内置租户角色和当前租户自定义角色，并返回权限数量和有效成员数量。
- **FR-017**：系统必须提供创建、详情、修改、禁用租户自定义角色的能力。
- **FR-018**：系统必须禁止租户修改系统内置角色的 code、作用域、分类和权限；系统内置角色本阶段只读。
- **FR-019**：系统必须提供查询和全量替换自定义角色权限的能力；只能绑定有效租户权限，空数组表示清空权限。
- **FR-020**：系统必须提供查询和全量替换成员角色的能力；更新完成后返回成员最新角色和权限集合。
- **FR-021**：成员角色替换必须校验成员属于当前租户且状态有效，角色属于系统内置租户角色或当前租户自定义角色，且角色状态有效。
- **FR-022**：成员角色替换必须拒绝 `PLATFORM_ADMIN` 和任何平台作用域角色。
- **FR-023**：成员角色替换和禁用相关角色时必须保护最后一名有效 `TENANT_ADMIN`，并在并发操作下仍成立。
- **FR-024**：系统必须新增统一授权服务，支持查询平台作用域权限、查询指定租户权限、判断指定 permission code、返回当前登录用户权限集合和对无权限返回统一业务错误。
- **FR-025**：系统必须新增权限中间件；认证中间件只负责认证，租户中间件只负责解析和验证租户上下文，权限中间件只调用统一授权服务判断 permission code。
- **FR-026**：Service 仍必须负责资源归属、操作者是否为资源创建者、策略状态、最后管理员等业务规则，不得把对象归属检查完全交给权限中间件或前端。
- **FR-027**：系统必须新增或扩展当前租户授权上下文接口，返回当前租户 ID、真实角色集合和真实权限 code 集合。
- **FR-028**：当前用户权限集合不得继续由固定角色 code 硬编码拼接。
- **FR-029**：迁移期间可以保留旧角色判断作为兼容包装，但新业务功能不得新增以固定角色 code 代替 permission code 的授权逻辑。
- **FR-030**：本阶段必须至少把租户角色管理、租户成员角色管理、策略读取、策略写入、组织读取、组织管理迁移到 permission code 授权。
- **FR-031**：系统必须定义稳定业务错误，覆盖权限不足、角色不存在、角色 code 冲突、内置角色不可变、角色禁用、作用域非法、成员不属于租户、不能分配平台角色、不能移除最后管理员和跨租户访问拒绝。
- **FR-032**：所有租户 Repository 方法必须显式包含可信 `tenant_id` 条件，不能只通过全局 `role_id` 或 `user_id` 修改租户数据。
- **FR-033**：数据迁移必须回填角色作用域、角色分类、内置标记、状态、用户角色状态、权限、角色权限，并保留现有用户角色关系。
- **FR-034**：数据迁移必须具备事务或可恢复设计，提供失败处理和回滚说明，并能重复执行。
- **FR-035**：本阶段不得修改 CP-ABE 解密算法，不得把 RBAC 角色或自定义角色名称自动同步为 CP-ABE 属性。
- **FR-036**：本阶段不得实现前端页面、动态菜单、角色继承、DENY、条件权限、数据范围表达式、审计落库或文件密钥封装。
- **FR-037**：涉及新增或修改 Go 业务代码的计划必须说明注释策略，任务必须包含“关键注释和可读性检查”。

### 数据和迁移需求

- **DR-001**：`roles` 必须至少支持 `id`、`tenant_id`、`code`、`name`、`description`、`scope_type`、`role_category`、`is_builtin`、`status`、`created_by`、`updated_by`、`created_at`、`updated_at`。
- **DR-002**：`roles` 必须满足 `UNIQUE(tenant_id, code)` 和 `INDEX(tenant_id, status)`；`tenant_id=0` 表示系统内置角色，`tenant_id>0` 表示租户自定义角色。
- **DR-003**：租户角色列表必须包含 `tenant_id=0 AND scope_type=TENANT` 的系统内置租户角色，以及 `tenant_id=当前租户` 的自定义角色；不得暴露或允许分配 `PLATFORM_ADMIN`。
- **DR-004**：`user_roles` 必须至少支持 `id`、`tenant_id`、`user_id`、`role_id`、`assignment_source`、`assigned_by`、`status`、`expires_at`、`revoked_at`、`created_at`、`updated_at`。
- **DR-005**：`user_roles` 必须满足 `UNIQUE(tenant_id, user_id, role_id)`、`INDEX(tenant_id, user_id, status)`、`INDEX(tenant_id, role_id, status)`。
- **DR-006**：有效角色条件至少为 `user_roles.status=ACTIVE`、`roles.status=ACTIVE`、`expires_at` 为空或晚于当前时间。
- **DR-007**：再次分配同一角色时必须重新激活已有记录、清理 `revoked_at`、更新分配人和来源，不重复创建绑定记录。
- **DR-008**：`permissions` 必须至少支持 `id`、`code`、`name`、`description`、`scope_type`、`resource_type`、`action`、`status`、`created_at`、`updated_at`，且 `code` 全局唯一。
- **DR-009**：第一批权限必须包含平台权限 `platform.tenant.read`、`platform.tenant.manage`、`platform.template.read`、`platform.template.manage`。
- **DR-010**：第一批权限必须包含租户权限 `tenant.dashboard.read`、`tenant.role.read`、`tenant.role.manage`、`tenant.member.read`、`tenant.member.manage`、`tenant.org.read`、`tenant.org.manage`、`policy.read`、`policy.write`、`policy.publish`、`file.read`、`file.upload`、`file.manage`、`file.decrypt.invoke`、`audit.read`。
- **DR-011**：数据库中不得用通配符权限作为真实权限，例如不得只保存 `tenant.*`。
- **DR-012**：`role_permissions` 必须至少支持 `id`、`role_id`、`permission_id`、`granted_by`、`created_at`，并满足 `UNIQUE(role_id, permission_id)`。

### 后端接口需求

- **API-001**：`GET /api/v1/tenant/permissions` 必须返回当前租户自定义角色可选择的租户权限，需要 `tenant.role.read` 或 `tenant.role.manage`。
- **API-002**：`GET /api/v1/tenant/roles` 必须返回系统内置租户角色和当前租户自定义角色，需要 `tenant.role.read`。
- **API-003**：`POST /api/v1/tenant/roles` 必须创建当前租户自定义业务角色，需要 `tenant.role.manage`。
- **API-004**：`GET /api/v1/tenant/roles/:roleId` 必须校验角色属于系统内置租户角色或当前租户自定义角色，需要 `tenant.role.read`。
- **API-005**：`PATCH /api/v1/tenant/roles/:roleId` 必须只允许修改自定义角色的 `name/description`；角色 code 创建后不允许修改，需要 `tenant.role.manage`。
- **API-006**：`DELETE /api/v1/tenant/roles/:roleId` 或符合现有风格的状态接口必须逻辑禁用自定义角色，返回受影响成员数量，需要 `tenant.role.manage`。
- **API-007**：`GET /api/v1/tenant/roles/:roleId/permissions` 必须返回角色权限，需要 `tenant.role.read`。
- **API-008**：`PUT /api/v1/tenant/roles/:roleId/permissions` 必须全量替换自定义业务角色权限，需要 `tenant.role.manage`。
- **API-009**：`GET /api/v1/tenant/members/:userId/roles` 必须返回成员当前租户角色，需要 `tenant.member.read`。
- **API-010**：`PUT /api/v1/tenant/members/:userId/roles` 必须全量替换成员角色集合，需要 `tenant.member.manage`。
- **API-011**：`GET /api/v1/tenant/me/authorization` 必须返回当前用户在当前租户的真实角色和权限集合。

### 关键实体

- **角色**：表示平台或租户内可授予的身份集合。关键属性包括租户归属、稳定 code、展示名称、作用域、分类、内置标记和状态。
- **用户角色授权**：表示用户在平台或某租户内获得某个角色。关键属性包括租户、用户、角色、来源、分配人、状态、过期时间和撤销时间。
- **权限**：表示可被统一授权服务判断的稳定功能点。关键属性包括 code、作用域、资源类型、动作和状态。
- **角色权限绑定**：表示某个角色拥有某个权限。内置角色权限由种子和系统版本管理；租户自定义业务角色权限由租户管理员维护。
- **租户成员**：表示用户是否是某租户有效成员。租户业务授权必须以有效成员关系为前提。
- **授权上下文**：表示当前登录用户在平台或租户作用域下的有效角色和有效权限集合。

### 宪章对齐（必填）

- **混合加密与真实 CP-ABE**：本功能不修改文件内容 AES-GCM、DEK 封装、RSA 或 CP-ABE 算法。`file.decrypt.invoke` 只控制是否允许调用解密流程，不改变最终 CP-ABE 策略满足判断。
- **模块边界**：RBAC 能力应收敛在认证授权、Tenant、Policy、Org、Repository 和 Middleware 边界内；密码算法逻辑不得散落到 RBAC、Handler 或 Service 中。
- **可解释性**：授权上下文接口必须清楚返回角色和权限来源；策略解密成功与失败仍由用户属性、策略快照、密钥材料和密码计算结果解释，不由 RBAC 角色直接解释。
- **Go 注释要求**：本功能会新增或修改 Go 业务代码。计划和任务必须要求每个函数/方法有前置中文注释，导出标识符符合 GoDoc，实体字段、Handler、Service、Repository、Middleware 必须解释业务语义、副作用、权限前置条件、事务边界和安全边界；实现后必须完成“关键注释和可读性检查”。

## 成功标准 *(必填)*

### 可衡量结果

- **SC-001**：通过自动化测试证明平台管理员只有平台权限，且不因平台身份自动获得任一租户业务权限。
- **SC-002**：通过自动化测试证明租户管理员可在 1 个租户内完成自定义角色创建、修改、禁用和权限替换完整流程。
- **SC-003**：通过自动化测试证明不同租户可创建相同角色 code，同一租户重复 code 返回冲突。
- **SC-004**：通过自动化测试证明同一成员可同时拥有 `DO` 和 `DU`，并且任一分配顺序都不会删除另一个角色。
- **SC-005**：通过自动化测试证明用户权限由所有有效角色权限并集计算，重复权限会去重，撤销、过期和禁用角色不产生权限。
- **SC-006**：通过自动化测试证明无 `tenant.role.manage`、`tenant.member.manage`、`policy.write`、`tenant.org.manage` 的用户无法调用对应写接口。
- **SC-007**：通过自动化测试证明拥有 `policy.write` 的自定义角色用户可以修改当前租户内允许修改的策略。
- **SC-008**：通过自动化测试证明所有租户角色、成员角色和角色权限写操作均使用当前可信租户上下文，不能跨租户查询或修改。
- **SC-009**：通过并发或数据库级测试证明成员角色替换不会导致租户没有有效 `TENANT_ADMIN`。
- **SC-010**：通过迁移测试证明迁移和 seed 重复执行不会生成重复角色、权限或角色权限关系。
- **SC-011**：通过迁移测试证明已有租户管理员、原有 `DO/DU` 用户在迁移后仍拥有对应默认权限。
- **SC-012**：通过回归测试证明原有登录、租户切换、组织读取、组织管理和策略基础功能不回归。

## 假设

- 本阶段以当前后端为唯一交付范围，不新增前端页面或前端弹窗。
- 当前认证方式和 `X-Tenant-Id` 租户选择机制继续沿用，但租户上下文校验和权限判断必须由后端完成。
- 角色 code 创建后不可修改，避免权限缓存、审计和历史绑定难以解释。
- 内置角色权限本阶段由种子管理，租户管理员只能读取，不能修改。
- 权限缓存本阶段不引入 Redis；若后续需要缓存，必须先定义正确失效机制。
- `users.role` 等旧兼容字段本阶段保留，不做最终清理。
- `DATA_OWNER`、`DATA_VISITOR` 等旧兼容常量和值本阶段保留用于迁移和属性兼容，不直接删除。
- 文件域、审计域、文件密钥封装和 CP-ABE 解密改造不属于本阶段。

## 明确排除范围

- 前端角色管理页面、前端多角色弹窗。
- `ui_menus`、`permission_menus`、`api_resources`、`permission_apis`、动态菜单数据库。
- 角色继承、DENY 权限、权限优先级、条件权限、数据范围表达式。
- 组织和角色变化后的自动属性同步。
- `access_policy_versions`、文件元数据与文件版本、文件密钥封装。
- `tenant_key_sets`、`user_key_materials`、`audit_logs` 落库、`decrypt_events`。
- 清理 `users.role` 等旧兼容字段。
- 删除 `DATA_OWNER`、`DATA_VISITOR` 等旧兼容常量。
- CP-ABE 加解密流程改造。

## 验收清单

- 平台管理员只有平台权限，不自动拥有租户业务权限。
- 租户管理员可以创建租户自定义角色。
- 普通成员不能创建、修改或禁用角色。
- 自定义角色只能创建为 `TENANT + BUSINESS`。
- 不同租户可以创建相同 role code。
- 同一租户不能创建重复 role code。
- 租户 A 无法查询或修改租户 B 的自定义角色。
- 租户不能修改系统内置角色的分类和作用域。
- 租户不能修改系统内置角色权限。
- 自定义角色可以绑定多个租户权限。
- 自定义角色不能绑定平台权限。
- 多个角色的权限能够正确合并并去重。
- `REVOKED` 角色不再产生权限。
- `EXPIRED` 角色不再产生权限。
- `DISABLED` 角色不再产生权限。
- 同一成员可以同时拥有 `DO` 和 `DU`。
- 分配 `DO` 时不会删除 `DU`。
- 分配 `DU` 时不会删除 `DO`。
- `PLATFORM_ADMIN` 不能通过租户成员接口分配。
- 不能移除当前租户最后一名 `TENANT_ADMIN`。
- 并发操作也不能导致租户没有有效管理员。
- 无 `policy.write` 权限的用户不能修改策略。
- 有 `policy.write` 的自定义角色用户能够修改当前租户策略。
- 无 `tenant.org.manage` 权限的用户不能修改组织。
- 当前用户授权接口返回真实角色和权限。
- 权限结果不再由角色 code 硬编码拼接。
- 数据迁移后，已有租户管理员仍然可以管理租户。
- 数据迁移重复执行不会生成重复数据。
- 所有租户查询均带 `tenant_id` 条件。
- 原有登录、租户切换、组织和策略基础功能不回归。
