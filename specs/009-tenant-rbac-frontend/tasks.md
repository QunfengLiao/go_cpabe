# 任务清单：租户 RBAC 前端授权与角色管理

**输入**：`specs/009-tenant-rbac-frontend/spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/frontend-rbac-api.md`、`quickstart.md`

**前置条件**：后端 008 租户级 RBAC 接口已存在；本阶段不新增数据库迁移，不生成属性、策略版本、文件、密钥或审计任务；只允许必要的小范围后端契约适配。

**测试要求**：规格要求必须通过自动化测试或手工验收验证。前端优先使用 Vitest 覆盖授权状态、请求隔离、菜单/路由/按钮守卫和关键组件；Electron 刷新、账号切换和租户切换使用手工回归或后续端到端测试补充。

**组织方式**：任务按独立可验收的前端能力拆分，并按用户故事和交付依赖排序。每条任务包含前置依赖、修改范围、验收方式和对应规格场景。

## 阶段 1：准备与基线盘点

**目标**：在实现前固定当前固定角色判断、授权状态来源和高风险页面清单，避免遗漏旧逻辑。

- [X] T001 前端角色与权限硬编码基线盘点；前置依赖：无；修改范围：扫描 `desktop/src/renderer/src/main.tsx`、`desktop/src/renderer/src/components/AppLayout.tsx`、`desktop/src/renderer/src/auth/AuthContext.tsx`、`desktop/src/renderer/src/api/authStorage.ts`、`desktop/src/renderer/src/components/TenantMemberRoleDialog.tsx`、`desktop/src/renderer/src/pages/TenantMembersPage.tsx`、`desktop/src/renderer/src/pages/TenantOrgManagementPage.tsx`、`desktop/src/renderer/src/pages/MyAccessPoliciesPage.tsx`、`desktop/src/renderer/src/pages/TenantAccessPolicyViewPage.tsx` 并在实现记录中列出需替换的 `TENANT_ADMIN`、`DO`、`DU`、`PLATFORM_ADMIN` 功能授权判断；验收方式：`rg "TENANT_ADMIN|\\bDO\\b|\\bDU\\b|PLATFORM_ADMIN|roles\\.includes|RequireTenantRole" desktop/src/renderer/src` 输出已分类为展示用途、平台管理用途或待迁移授权用途；对应规格场景：SC-013、FR-004、FR-042。

---

## 阶段 2：基础能力（阻塞所有用户故事）

**目标**：先完成 RBAC API、授权上下文和账号/租户生命周期，后续菜单、路由、按钮和页面都只依赖统一权限事实源。

**关键检查点**：完成本阶段前，不开始页面级 RBAC 功能。

- [X] T002 RBAC API Client 和 DTO；前置依赖：T001；修改范围：新增或修改 `desktop/src/renderer/src/api/rbac.ts`、`desktop/src/renderer/src/types.ts`、`desktop/src/renderer/src/api/rbac.test.ts`，实现 `getCurrentAuthorization`、`listTenantPermissions`、`listTenantRoles`、`createTenantRole`、`getTenantRole`、`updateTenantRole`、`disableTenantRole`、`getTenantRolePermissions`、`replaceTenantRolePermissions`、`getTenantMemberRoles`、`replaceTenantMemberRoles` 及 DTO；验收方式：mock `request()` 验证所有方法路径、请求体、响应解析和错误 code 映射符合 `specs/009-tenant-rbac-frontend/contracts/frontend-rbac-api.md`；对应规格场景：US3、US4、FR-021 至 FR-039、SC-003 至 SC-008。
- [X] T003 Authorization Context 与权限查询工具；前置依赖：T002；修改范围：修改 `desktop/src/renderer/src/auth/AuthContext.tsx`、`desktop/src/renderer/src/api/authRuntime.ts`、`desktop/src/renderer/src/api/authStorage.ts`，新增 `desktop/src/renderer/src/auth/permissions.ts`，实现 `authorizationStatus`、`authorizationUserId`、`authorizationTenantId`、`authorizationGeneration`、`authorizationError`、`refreshAuthorization()`、`clearAuthorization()`、`hasPermission()`、`hasAnyPermission()`、`hasAllPermissions()`，并移除 `permissionsFromRoles()` 作为授权成功来源；验收方式：单元测试覆盖 ready、loading、error、userId/tenantId/generation 不匹配时权限判断均不放行，且 permissions 不作为长期可信缓存；对应规格场景：US2、FR-001 至 FR-004、FR-041、SC-010 至 SC-013。
- [X] T004 登录、退出、账号切换和租户切换的权限生命周期；前置依赖：T003；修改范围：修改 `desktop/src/renderer/src/auth/AuthContext.tsx`、`desktop/src/renderer/src/api/authStorage.ts`、`desktop/src/renderer/src/api/request.ts`、`desktop/src/renderer/src/pages/LoginPage.tsx`、`desktop/src/renderer/src/pages/SelectTenantPage.tsx`、`desktop/src/renderer/src/pages/AccountSwitchPage.tsx` 及相关测试，确保 `finishLogin`、`switchAccount`、`switchTenant`、`logout`、`clearAuth`、Electron 刷新恢复都会清空旧授权并重新加载 `/tenant/me/authorization`；验收方式：自动化测试验证账号 1 切换账号 2 时不短暂显示账号 1 菜单、不携带账号 1 的 `tenant_id`，租户切换后 cache key/generation 防止旧请求回写，网络失败不会保留为 ready；对应规格场景：US2、FR-005 至 FR-010、SC-010、SC-011、SC-012。

---

## 阶段 3：用户故事 1 - 用户按真实权限进入功能（优先级：P1，MVP）

**目标**：菜单、路由和按钮全部按 permission code 授权；有权限能进，无权限看不到也进不去。

**独立测试**：准备有 `tenant.role.read` 和无 `tenant.role.read` 的用户，验证角色管理菜单显示、手动输入 `#/tenant/roles` 的路由保护、只读用户按钮不可操作。

- [X] T005 [US1] 菜单权限过滤；前置依赖：T003、T004；修改范围：修改 `desktop/src/renderer/src/components/AppLayout.tsx`、`desktop/src/renderer/src/auth/permissions.ts`，为静态菜单增加 `requiredPermission`、`requiredAnyPermissions`、`scope` 元数据和递归过滤器，新增角色管理菜单并隐藏无可见子项的父菜单；验收方式：组件或工具测试验证有 `tenant.role.read` 可见角色管理菜单、无权限不可见、切换账号/租户期间不显示旧菜单、平台权限不作为租户菜单依据；对应规格场景：US1、US2、FR-011 至 FR-014、SC-001、SC-010、SC-011。
- [X] T006 [US1] 路由权限守卫与 403 页面；前置依赖：T003、T004；修改范围：新增 `desktop/src/renderer/src/auth/RequirePermission.tsx`、`desktop/src/renderer/src/pages/ForbiddenPage.tsx`，修改 `desktop/src/renderer/src/main.tsx`，新增 `/tenant/roles` 路由并用 permission guard 替换租户业务路由中的 `RequireTenantRole`；验收方式：手动输入 `#/tenant/roles` 时，无 `tenant.role.read` 用户无法进入并看到 403 或安全页，有权限用户可进入，授权 loading/error 不放行且不形成重定向循环；对应规格场景：US1、FR-015 至 FR-017、SC-002、SC-009。
- [X] T007 [US1] 按钮和操作 PermissionGuard；前置依赖：T003、T005、T006；修改范围：新增或修改 `desktop/src/renderer/src/auth/PermissionGuard.tsx`、`desktop/src/renderer/src/pages/TenantMembersPage.tsx`、`desktop/src/renderer/src/pages/TenantOrgManagementPage.tsx`、`desktop/src/renderer/src/components/tenant-org/OrgUnitTreePanel.tsx`、`desktop/src/renderer/src/components/tenant-org/OrgMemberTable.tsx`、`desktop/src/renderer/src/pages/MyAccessPoliciesPage.tsx`、`desktop/src/renderer/src/pages/AccessPolicyBuilderPage.tsx`、`desktop/src/renderer/src/pages/AccessPolicyEditorPage.tsx`、`desktop/src/renderer/src/pages/TenantAccessPolicyViewPage.tsx`，用 `tenant.member.manage`、`tenant.org.read/manage`、`policy.read/write/publish` 控制关键入口；验收方式：只读用户能查看但不能写，普通成员不能修改其他成员角色，后端 403 时前端提示权限不足；对应规格场景：US1、US5、FR-018 至 FR-020、FR-040、SC-003、SC-008、SC-014。

**检查点**：MVP 完成后，应能独立验证菜单、路由和按钮不再依赖固定租户角色 code 授权。

---

## 阶段 4：用户故事 3 - 租户管理员维护租户角色（优先级：P1）

**目标**：提供可验收的角色管理页面，支持查看、筛选、创建、编辑自定义角色、权限配置和内置角色只读。

**独立测试**：拥有 `tenant.role.manage` 的用户创建并编辑自定义角色；只有 `tenant.role.read` 的用户只能查看；切换租户后角色列表隔离。

- [X] T008 [US3] 租户角色管理列表与筛选；前置依赖：T002、T005、T006；修改范围：新增 `desktop/src/renderer/src/pages/TenantRolesPage.tsx`、`desktop/src/renderer/src/components/tenant-rbac/RoleList.tsx`、`desktop/src/renderer/src/components/tenant-rbac/RoleDetailPanel.tsx`，按治理角色、业务角色、能力角色分组，支持分类、状态、关键字筛选和详情查看；验收方式：列表显示角色名称、code、分类、内置/自定义、状态、权限数量、有效成员数量、描述、创建时间和操作入口，不同租户切换后角色列表不混入；对应规格场景：US3、FR-021 至 FR-023、FR-026、SC-004。
- [X] T009 [US3] 创建和编辑自定义角色；前置依赖：T002、T008；修改范围：新增或修改 `desktop/src/renderer/src/components/tenant-rbac/RoleEditorDrawer.tsx`、`desktop/src/renderer/src/pages/TenantRolesPage.tsx`、`desktop/src/renderer/src/api/rbac.ts`，建立创建/编辑表单、保存骨架和错误处理，创建时提交 `code/name/description/permissionCodes`，编辑时只允许修改名称和描述，角色 code 创建后不可修改；验收方式：有 `tenant.role.manage` 可创建角色，只有 `tenant.role.read` 只能查看，创建成功后自定义角色出现在业务角色分组，`ROLE_CODE_EXISTS` 显示明确 code 冲突提示，修改角色名称不影响 permission 判断；对应规格场景：US3、FR-024、FR-025、FR-028、FR-039、FR-043、SC-003、SC-004、SC-012、SC-013。
- [X] T010 [US3] 权限目录与权限选择器；前置依赖：T002、T003、T009；修改范围：新增 `desktop/src/renderer/src/components/tenant-rbac/PermissionSelector.tsx` 并接入 `desktop/src/renderer/src/components/tenant-rbac/RoleEditorDrawer.tsx`、`desktop/src/renderer/src/components/tenant-rbac/RoleDetailPanel.tsx`，从 `/tenant/permissions` 加载权限目录，按资源模块分组，支持搜索、分组全选、取消、已选数量和未保存提示；验收方式：权限选择器只显示 `scopeType=TENANT` 且 active 的权限，不显示平台权限，保存角色权限后刷新角色详情和权限数量；对应规格场景：US3、FR-029 至 FR-031、SC-006。
- [X] T011 [US3] 角色禁用和只读内置角色；前置依赖：T008、T009、T010；修改范围：修改 `desktop/src/renderer/src/pages/TenantRolesPage.tsx`、`desktop/src/renderer/src/components/tenant-rbac/RoleDetailPanel.tsx`、`desktop/src/renderer/src/components/tenant-rbac/RoleEditorDrawer.tsx`，基于 `builtin/isBuiltin`、`status` 和 `tenant.role.manage` 控制编辑、权限保存和禁用入口；验收方式：内置角色 100% 显示只读和系统内置标识，不能编辑权限或禁用，自定义角色可禁用，后端返回 `BUILTIN_ROLE_IMMUTABLE` 或角色禁用错误时显示明确提示；对应规格场景：US3、US5、FR-027、FR-036、FR-039、SC-005、SC-012。

---

## 阶段 5：用户故事 4 - 租户管理员为成员分配多个角色（优先级：P1）

**目标**：成员角色弹窗从 DO/DU 单选改为完整多角色集合替换，并处理自身角色变化后的权限刷新。

**独立测试**：打开成员角色弹窗，同时选择 `DO`、`DU` 和一个自定义业务角色并保存，确认 `PLATFORM_ADMIN` 不出现，最后管理员保护和自身失权跳转生效。

- [X] T012 [US4] TenantMemberRoleDialog 多角色改造；前置依赖：T002、T007、T008；修改范围：修改 `desktop/src/renderer/src/components/TenantMemberRoleDialog.tsx`、`desktop/src/renderer/src/pages/TenantMembersPage.tsx`、`desktop/src/renderer/src/api/tenant.ts`、`desktop/src/renderer/src/api/rbac.ts`，弹窗打开时加载候选租户角色和成员当前完整角色，按分组多选，保存时提交完整 `roleCodes`，并保留关闭不保存时的临时状态隔离；验收方式：`DO` 和 `DU` 可以同时选择并保存，`PLATFORM_ADMIN` 不出现在租户角色选项，禁用角色不可新选，普通成员不能修改其他成员角色，保存后成员列表和角色数量刷新；对应规格场景：US4、FR-032 至 FR-038、SC-007、SC-008。
- [X] T013 [US4] 修改自身角色后的权限刷新和安全跳转；前置依赖：T003、T006、T012；修改范围：修改 `desktop/src/renderer/src/auth/AuthContext.tsx`、`desktop/src/renderer/src/pages/TenantMembersPage.tsx`、`desktop/src/renderer/src/pages/TenantRolesPage.tsx`、`desktop/src/renderer/src/auth/RequirePermission.tsx`，成员角色或角色权限保存影响当前用户时调用 `refreshAuthorization()`，并在失去当前页面权限后计算安全落点；验收方式：修改自己的角色后当前权限立即刷新，菜单和按钮同步变化，失去当前页面权限后跳转到第一个有权限页面或 403，不产生重定向循环；对应规格场景：US4、US2、FR-017、FR-031、FR-038、SC-009。

---

## 阶段 6：用户故事 5 - 错误提示和既有页面不回归（优先级：P2）

**目标**：补齐错误、并发、加载状态和回归测试，确保旧授权不会闪现，组织和策略页面不被 RBAC 改造破坏。

**独立测试**：模拟 403、角色 code 冲突、网络失败、最后管理员保护、账号切换、租户切换、Electron 刷新，验证错误提示和授权状态。

- [X] T014 [US5] 错误、并发和加载状态；前置依赖：T002、T003、T004、T011、T012；修改范围：修改 `desktop/src/renderer/src/api/rbac.ts`、`desktop/src/renderer/src/api/request.ts`、`desktop/src/renderer/src/auth/AuthContext.tsx`、`desktop/src/renderer/src/pages/ForbiddenPage.tsx`、`desktop/src/renderer/src/pages/TenantRolesPage.tsx`、`desktop/src/renderer/src/components/TenantMemberRoleDialog.tsx`，统一处理 `PERMISSION_DENIED`、`ROLE_CODE_EXISTS`、`BUILTIN_ROLE_IMMUTABLE`、`ROLE_DISABLED`、`INVALID_PERMISSION_SCOPE`、`CANNOT_ASSIGN_PLATFORM_ROLE`、`CANNOT_REMOVE_LAST_TENANT_ADMIN`、网络失败和旧请求回写；验收方式：403、code 冲突、最后管理员保护和网络失败均显示明确提示，授权接口失败不保留为 ready，旧账号/旧租户请求不会覆盖新状态；对应规格场景：US5、US2、FR-039 至 FR-041、SC-012。
- [X] T015 [US5] 前端单元与组件测试；前置依赖：T005、T006、T007、T010、T012、T014；修改范围：新增或修改 `desktop/src/renderer/src/auth/permissions.test.ts`、`desktop/src/renderer/src/api/rbac.test.ts`、`desktop/src/renderer/src/api/authStorage.test.ts`、`desktop/src/renderer/src/api/request.test.ts`、`desktop/src/renderer/src/components/tenant-rbac/PermissionSelector.test.tsx`、`desktop/src/renderer/src/components/TenantMemberRoleDialog.test.tsx`、`desktop/src/renderer/src/components/AppLayout.test.tsx`；验收方式：`cd desktop; npm run test` 通过，测试覆盖无权限直接访问 URL 的守卫逻辑、菜单过滤、按钮守卫、权限选择器不显示平台权限、DO/DU 同时选择、账号/租户切换权限隔离和网络失败状态；对应规格场景：US1、US2、US3、US4、US5、SC-001 至 SC-013。
- [X] T016 [US5] 账号切换、租户切换和 Electron 回归测试；前置依赖：T014、T015；修改范围：执行并必要时补充 `specs/009-tenant-rbac-frontend/quickstart.md`，验证 `desktop/package.json` 中 `typecheck`、`test`、`build:renderer`，并手工覆盖 Electron 的登录、退出、账号切换、租户切换、刷新恢复、组织管理和策略页面；验收方式：`cd desktop; npm run typecheck; npm run test; npm run build:renderer` 通过，手工验收记录确认账号 2 不携带账号 1 的 `tenant_id`，切换租户后权限和菜单立即重算，退出后角色和权限完全清空，组织和策略页面不回归；对应规格场景：US2、US5、SC-010、SC-011、SC-014。

---

## 阶段 7：收尾与一致性检查

**目标**：确认固定角色授权已清理，文档和注释治理要求满足宪章。

- [X] T017 清理固定角色授权判断和更新文档；前置依赖：T015、T016；修改范围：复扫 `desktop/src/renderer/src/**`、更新 `specs/009-tenant-rbac-frontend/quickstart.md` 中实际验收记录或偏差说明，必要时最小修改 `backend/internal/handler/rbac_handler.go` 或相关 DTO 契约适配文件；验收方式：`rg "RequireTenantRole|roles\\.includes|TENANT_ADMIN|\\bDO\\b|\\bDU\\b|PLATFORM_ADMIN" desktop/src/renderer/src` 中剩余项均为展示、平台管理或测试示例而非租户功能授权来源；若触及 Go 业务代码，完成“关键注释和可读性检查”，确认每个新增/修改 Go 函数和方法有中文前置注释、导出标识符符合 GoDoc、安全和权限边界说明清楚；对应规格场景：US5、FR-042、FR-043、SC-013、SC-014。

---

## 依赖与执行顺序

### 阶段依赖

- 阶段 1 无依赖，可立即开始。
- 阶段 2 依赖 T001，是所有用户故事的阻塞基础。
- 阶段 3 依赖 T003 和 T004，是 MVP 验收入口。
- 阶段 4 依赖 T002、T005、T006，并复用统一权限状态。
- 阶段 5 依赖 T002、T007、T008，并复用角色列表和成员权限入口。
- 阶段 6 依赖主要功能完成后进行错误、并发和回归验证。
- 阶段 7 依赖自动化和手工验收完成后收尾。

### 用户故事依赖

- **US1 用户按真实权限进入功能**：依赖基础阶段完成；可作为 MVP 独立交付。
- **US2 授权上下文在账号和租户生命周期中安全刷新**：由 T003、T004 实现核心能力，由 T016 做端到端回归验证；它是所有故事的基础。
- **US3 租户管理员维护租户角色**：依赖 US1 的菜单和路由入口，以及 RBAC API client。
- **US4 租户管理员为成员分配多个角色**：依赖 RBAC API client、成员按钮权限和角色候选列表。
- **US5 错误提示和既有页面不回归**：依赖主要功能完成后统一验证。

### 任务级依赖摘要

```text
T001
  -> T002 -> T003 -> T004
  -> T005 -> T006 -> T007
  -> T008 -> T009 -> T010 -> T011
  -> T012 -> T013
  -> T014 -> T015 -> T016 -> T017
```

---

## 可并行机会

- T002 的 API client 与 T003 的权限工具设计可先并行阅读，但代码落地需要 T002 的 DTO 稳定后再接入。
- T005 菜单过滤和 T006 路由守卫在 T003、T004 完成后可并行实现，最终共同接入 `AppLayout` 和 `main.tsx`。
- T008 角色列表完成后，T009 创建/编辑骨架和 T010 权限选择器按顺序组合；如拆成多人实现，T010 的纯选择器内部交互可在 T009 表单骨架稳定后独立推进。
- T012 成员多角色弹窗可在 T008 提供候选角色列表后推进，T013 依赖保存后的自身授权刷新。
- T015 中 API、权限工具、菜单过滤和组件测试可按文件并行补齐。

---

## 实施策略

### MVP 优先

1. 完成 T001 至 T004，建立可信授权上下文和账号/租户生命周期。
2. 完成 T005 至 T007，先交付“菜单可见、路由可进、按钮可用”三层 permission code 授权。
3. 停下验证 US1：有 `tenant.role.read` 的用户可见并进入角色管理入口，无权限用户菜单不可见且手动 URL 不能进入。

### 增量交付

1. 完成 T008 至 T011，交付角色管理闭环。
2. 完成 T012 至 T013，交付成员多角色和自身角色变化后的授权刷新。
3. 完成 T014 至 T016，补齐错误、并发、自动化测试和 Electron 回归。
4. 完成 T017，清理固定角色授权残留和文档偏差。

### 注意事项

- 角色 code 可以用于展示、分组和提交成员角色集合，但不得作为菜单、路由或按钮授权来源。
- permissions 可以作为本地恢复快照，但不能作为长期可信缓存；服务端 `/tenant/me/authorization` 成功且 userId、tenantId、generation 匹配后才可进入授权 ready。
- 本阶段不新增数据库迁移，不触碰 CP-ABE、AES-GCM、RSA-OAEP、文件、密钥、审计或策略版本实现。
- 若实现中发现后端接口 envelope 或错误 code 与契约不一致，优先在前端 API client 做最小兼容；只有无法兼容时才做小范围后端契约适配。
- 如果没有修改 Go 业务代码，T017 的关键注释检查记录为“不涉及 Go 业务代码”；如果修改了 Go 业务代码，必须满足宪章中的中文前置注释、GoDoc 前缀和安全边界说明要求。
