# 实施计划：租户 RBAC 前端授权与角色管理

**分支**：`009-tenant-rbac-frontend` | **日期**：2026-07-11 | **规格**：[spec.md](./spec.md)

**输入**：来自 `specs/009-tenant-rbac-frontend/spec.md` 的功能规格，以及本轮计划前对当前 React + Electron 前端、后端 RBAC 路由和测试配置的实际扫描结果。

## 概要

本阶段在现有 Electron + React 桌面端中补齐租户级 RBAC 前端闭环：把菜单、路由和按钮从固定角色 code 授权迁移到 permission code 授权；新增租户角色管理页面、权限选择器和成员多角色弹窗；保证登录、账号切换、租户切换、退出登录和 Electron 页面刷新时授权上下文不会串账号、串租户或短暂闪现旧权限。

技术路径以当前项目结构为基础：继续使用 `AuthContext` 承载账号和租户上下文，在其中扩展明确的授权状态和权限判断方法；新增当前租户 RBAC API client；新增 permission-based `RouteGuard`、`PermissionGuard`、菜单过滤器和 403/错误/加载页；前端 permissions 只作为短期恢复体验数据，不作为长期可信缓存，最终以服务端 `/tenant/me/authorization` 重新加载结果为准。

## 技术上下文

**语言/版本**：TypeScript，React 19，Electron 33.2，Vite 6。

**主要依赖**：`react-router-dom` 7、Ant Design 5.29、`@ant-design/icons`、`framer-motion`、项目现有 `fetch` 请求封装。

**存储**：Electron 渲染进程 `localStorage` 保存 access token、当前用户、账号列表和按账号隔离的租户上下文；Electron 主进程安全会话存储保存 refresh token；本功能不新增数据库表。

**测试**：`npm run typecheck`、`npm run test`、`npm run build:renderer`；现有前端测试为 Vitest，测试文件与源码同目录，例如 `authStorage.test.ts`、`request.test.ts` 和访问树相关测试。

**目标平台**：Electron 桌面应用，开发时可用 Vite Web 渲染入口验证。

**项目类型**：桌面端前端功能，依赖 Go 后端已有 RBAC API。

**性能目标**：账号/租户切换期间立即清空旧权限；授权接口成功后一个渲染周期内重算菜单、路由和按钮；权限判断使用内存集合，常规判断为常数级。

**约束**：不编写业务代码于计划阶段；不新增动态菜单数据库；不把前端权限当作最终安全边界；不长期信任本地持久化 permissions；不修改 CP-ABE、AES-GCM、RSA-OAEP 加密逻辑。

**规模/范围**：改造约 1 个授权上下文、3 个路由守卫/权限组件、1 个菜单布局、1 个租户角色管理页面、3-5 个角色管理子组件、1 个成员多角色弹窗、成员/组织/策略关键页面权限入口和前端测试。

## 前端现状盘点

### 路由入口

- `desktop/src/renderer/src/main.tsx` 是渲染进程路由入口，使用 `HashRouter`、`Routes`、`Route`。
- 认证页：`/login`、`/login/:tenantCode`、`/register` 由 `GuestOnly` 和 `GuestLayout` 包裹。
- 登录后应用页由 `RequireAuth` 和 `AppLayout` 包裹。
- 平台页通过 `RequirePlatformAdmin` 包裹：`/platform`、`/platform/tenants`、`/platform/policies` 等。
- 数据拥有者策略页通过 `RequireTenantRole roles={["DO"]}` 包裹：`/access-policies/builder`、`/access-policies`、编辑器等。
- 租户管理页通过 `RequireTenantRole roles={["TENANT_ADMIN"]}` 包裹：`/tenant/members`、`/tenant/access-policies`、`/tenant/org-management`。
- 当前没有 `/tenant/roles` 或角色管理页面路由，也没有通用 permission-based 路由守卫。

### 菜单配置

- `desktop/src/renderer/src/components/AppLayout.tsx` 内联构造 Ant Design `Menu` 项。
- 平台菜单通过 `auth.isPlatformAdmin` 判断。
- 策略菜单通过 `currentTenant?.roles?.includes("DO")` 判断。
- 租户管理菜单通过 `currentTenant?.roles?.includes("TENANT_ADMIN")` 判断。
- 菜单项没有 `requiredPermission`、`requiredAnyPermissions` 或统一过滤器；父菜单隐藏依赖手写分支。

### AuthContext

- `desktop/src/renderer/src/auth/AuthContext.tsx` 是当前账号、用户、租户、角色、权限和会话生命周期的中心。
- 公开字段包括 `currentUserId`、`accessToken`、`user`、`tenants`、`currentTenantId`、`currentTenant`、`tenantRoles`、`permissions`、`platformRoles`、`authReady`、`tenantContextReady`、`authStatus`、`isPlatformAdmin`、`cachedAccounts`。
- 已有 `finishLogin`、`refreshTenantContext`、`switchTenant`、`switchAccount`、`clearAuth`、`logout`。
- 已有 `authGeneration` 用于账号切换期间防止旧请求回写。
- 当前没有 `hasPermission`、`hasAnyPermission`、`hasAllPermissions`、`authorizationStatus`、`authorizationTenantId`、`authorizationUserId` 等明确授权语义字段。

### TenantContext 或相似上下文

- 当前没有独立 `TenantContext`。
- 租户上下文由 `AuthContext`、`authStorage.ts`、`authRuntime.ts` 共同承载。
- `SelectTenantPage.tsx` 通过 `auth.refreshTenantContext()` 和 `auth.switchTenant()` 恢复或切换租户。
- `tenantStartup.ts` 处理开发租户入口和显式租户登录入口，必要时清空旧会话，避免启动时误入上一次租户。

### 本地存储封装

- `desktop/src/renderer/src/api/authStorage.ts` 封装当前用户、账号列表和租户上下文。
- 关键 localStorage key：`go_cpabe_user`、`go_cpabe_current_user_id`、`go_cpabe_current_tenant_id`、`go_cpabe_current_tenant_code`、`go_cpabe_last_tenant_code`、`go_cpabe_tenants`、`go_cpabe_platform_roles`、`go_cpabe_cached_accounts`、`go_cpabe_tenant_contexts`。
- `go_cpabe_tenant_contexts` 已按 userId 保存 `currentTenantId`、`tenants`、`tenantRoles`、`platformRoles`、`permissions`。
- `tokenStorage.ts` 保存 access token，清理旧 refresh token key；`authSessionStore.ts` 调用 Electron 主进程安全会话能力保存和刷新 refresh token。
- 现状风险：`permissions` 会写入本地租户上下文，且 `tenantContextFromAPI` 在后端未返回 permissions 时会用 `permissionsFromRoles` 从固定角色生成伪权限。

### 请求拦截器

- `desktop/src/renderer/src/api/request.ts` 封装 `request<T>()`。
- 非公开 API 会添加 `Authorization`；租户 API 会等待 `waitForAuthReady()` 后添加 `X-Tenant-Id`。
- `isTenantApi()` 覆盖 `/tenant/...` 和部分 `/tenants/:id/...` 租户资源。
- GET 请求有 800ms 短缓存和 in-flight 去重，cache key 包含 `currentUserId` 与 `currentTenantId`。
- 写请求会清空短缓存；401 会尝试 refresh token。
- 当前没有把 authorization generation 纳入 cache key，且授权接口失败后需要更明确地清空或标记授权错误。

### 账号切换逻辑

- `AuthContext.switchAccount()` 会先验证本地安全会话，再将 `authGeneration` 加一，清空 access token、user、platformRoles、tenantRoles、permissions、currentTenant，并设置 `switching-account`。
- 刷新账号 token 后调用 `getCurrentUser()` 和 `listMyTenants()`，期间多次检查 generation 防止旧请求回写。
- 完成后按当前账号租户上下文导航到 `/profile` 或 `/select-tenant`。
- `LoginPage.tsx` 和 `AccountSwitchPage.tsx` 都调用 `auth.switchAccount()`。
- 待改造点：切换完成后应调用 `/tenant/me/authorization` 作为当前租户权限事实源，而不是只依赖 `/me/context` 的 permissions 字段。

### 租户切换逻辑

- `AuthContext.switchTenant()` 调用 `switchTenantRequest()` 即 `POST /me/switch-tenant`。
- 切换前设置 `switching-tenant`、清理请求缓存、清空主题品牌；切换后写入当前租户、角色和 permissions。
- `SelectTenantPage.tsx` 调用 `auth.switchTenant()` 并跳转 `/profile`。
- 待改造点：租户切换开始时必须显式清空授权上下文，切换成功后再调用 `/tenant/me/authorization`，并用新授权重算菜单和路由。

### 当前 permissions 数据来源

- 登录响应 `LoginData.permissions`。
- `/me/context` 响应 `TenantContextData.permissions`。
- `/me/switch-tenant` 响应 `SwitchTenantData.permissions`。
- `authStorage.tenantContextFromAPI()` 在 API 未返回 permissions 时调用 `permissionsFromRoles(currentTenant?.roles)`，生成 `tenant:admin`、`data:own`、`data:visit`、`platform:admin` 等旧伪权限。
- 计划后事实源改为 `/tenant/me/authorization`；登录、租户上下文和切换租户响应中的 permissions 只能作为过渡快照，不能作为最终授权就绪依据。

### 所有固定角色判断

- `main.tsx`：`RequireTenantRole roles={["DO"]}`、`RequireTenantRole roles={["TENANT_ADMIN"]}`。
- `RequireTenantRole.tsx`：按 `currentTenant?.roles?.some(...)` 放行。
- `RequirePlatformAdmin.tsx`：按 `auth.isPlatformAdmin` 放行平台页，并在文案中提示 `PLATFORM_ADMIN`。
- `AuthContext.tsx`：`isPlatformAdmin` 和主题品牌判断使用 `platformRoles.includes("PLATFORM_ADMIN")`。
- `AppLayout.tsx`：菜单通过 `PLATFORM_ADMIN`、`TENANT_ADMIN`、`DO` 判断。
- `authStorage.ts`：`normalizeTenantRoles()` 固定白名单；`permissionsFromRoles()` 从 `TENANT_ADMIN/DO/DU/PLATFORM_ADMIN` 生成旧伪权限。
- `LoginPage.tsx`：登录后按 `PLATFORM_ADMIN` 跳转 `/platform`。
- `TenantMembersPage.tsx`：修改角色入口按 `TENANT_ADMIN`，禁用目标管理员按 `member.roles.includes("TENANT_ADMIN")`，按钮文案按 `DO/DU`。
- `TenantMemberRoleDialog.tsx`：单选 `DO/DU`，角色展示按 `TENANT_ADMIN/DO/DU`。
- `TenantOrgManagementPage.tsx` 与 `OrgMemberDrawer.tsx`：组织成员编辑会调用旧单角色接口，抽屉中的系统角色按 `DO/DU` 单选。
- `PlatformTenantUsersPage.tsx`：平台兜底管理页面按 `TENANT_ADMIN` 展示和统计租户管理员；这是平台管理领域展示/操作语义，可保留但不得作为租户功能授权来源。
- 访问树展示相关 `display.ts`、`mockData.ts`、测试中出现 `TENANT_ADMIN/DO/DU` 作为策略属性展示或示例，不属于前端功能授权来源，可按策略可视化需求保留或在后续统一属性字典时调整。

### TenantMemberRoleDialog

- 文件：`desktop/src/renderer/src/components/TenantMemberRoleDialog.tsx`。
- 当前 props 为 `member`、`saving`、`onClose`、`onSave(role: TenantBusinessRole)`。
- 当前内部固定 `businessRoleOptions`，只允许 `DATA_OWNER -> DO` 和 `DATA_VISITOR -> DU`。
- 当前点击选项即保存，没有多选、差异预览、禁用角色展示、完整 `roleCodes` 提交或角色分组。

### 成员管理页面

- 文件：`desktop/src/renderer/src/pages/TenantMembersPage.tsx`。
- 当前调用 `listTenantMembers(tenantId)` 和旧 `assignTenantMemberRole(tenantId, userId, roleCode)`。
- 角色列以 `roleListLabel()` 显示固定角色。
- 修改入口由当前租户 `TENANT_ADMIN` 决定；目标 `TENANT_ADMIN` 成员按钮禁用。
- 计划后应改用 `tenant.member.read/manage`，成员角色读取/更新改为 `/tenant/members/:userId/roles`，角色候选来自 `/tenant/roles`。

### 策略页面

- `MyAccessPoliciesPage.tsx`：新建、编辑、删除按钮始终显示，当前依赖外层 `DO` 路由保护。
- `AccessPolicyBuilderPage.tsx` 和 `AccessPolicyEditorPage.tsx`：按当前租户加载属性、策略和编辑器，错误提示较粗；写入口应受 `policy.write` 控制，发布入口后续受 `policy.publish`。
- `TenantAccessPolicyViewPage.tsx`：租户策略查看页只读，当前依赖外层 `TENANT_ADMIN` 路由保护；应改为 `policy.read`。
- `api/policy.ts`：仍使用 `/tenants/:id/access-policies` 路径式接口，后端已通过 `TenantRequired` 和 permission 中间件校验；前端按钮仍需按 permission 优化体验。

### 组织页面

- `TenantOrgManagementPage.tsx`：当前页面整体通过 `TENANT_ADMIN` 路由保护，页面内新建部门、编辑、移动、删除、成员编辑等操作不做 permission 分层。
- `OrgUnitTreePanel.tsx`：部门树操作菜单始终展示新建子部门、编辑、移动、删除。
- `OrgMemberTable.tsx`：成员编辑按钮始终展示。
- `OrgMemberDrawer.tsx`：系统角色仍是 `DATA_OWNER/DATA_VISITOR` 单选，并调用旧成员角色接口；计划后成员租户角色统一由新的多角色弹窗处理，组织抽屉只保留组织归属与部门职务。
- `api/tenantOrg.ts`：当前租户组织接口走 `/tenant/org-*`，后端实际已按 `tenant.org.read/manage` 保护。

### 前端测试配置

- `desktop/package.json`：`typecheck` 使用 `tsc --noEmit -p tsconfig.json`，`test` 使用 `vitest run`，`build:renderer` 使用 `vite build`。
- `desktop/vite.config.ts`：Vite root 为 `src/renderer`，React 插件，未配置单独 test 段，Vitest 使用默认环境；现有测试通过内存 `localStorage/sessionStorage` 和 fetch mock。
- 现有测试位置：`api/authStorage.test.ts`、`api/request.test.ts`、访问树 `*.test.ts`。
- 本功能计划新增授权上下文、请求隔离、菜单过滤、路由守卫、权限选择器和成员多角色弹窗相关测试。

## 授权状态设计

### 授权上下文放置

- 继续放在 `AuthContext`，因为账号、access token、租户上下文、请求运行时和主题品牌已经由它统一协调。
- 不新建独立 `TenantContext`，避免账号切换和租户切换出现两个状态源；可在 `auth` 目录新增小型工具文件承载类型、权限判断和路由元数据。
- `authRuntime.ts` 同步扩展授权字段，供 `request.ts` 在 React 渲染外等待授权就绪并读取可信 `X-Tenant-Id`。

### 状态字段

- `authorizationStatus`: `idle | loading | ready | error`。
- `authorizationTenantId`: 当前 permissions 属于哪个租户。
- `authorizationUserId`: 当前 permissions 属于哪个用户。
- `authorizationGeneration`: 与 `authGeneration` 对齐，防止旧请求回写。
- `authorizationError`: 授权加载失败时的明确错误消息。
- `permissions`: 当前服务端确认的 permission code 集合。
- `tenantRoles`: 当前服务端返回的角色展示信息或兼容 role code 摘要；只用于展示和提交成员角色，不用于功能授权。
- 派生方法：`hasPermission(code)`、`hasAnyPermission(codes)`、`hasAllPermissions(codes)`。

### 加载时机

- 登录成功并确定当前租户后加载 `/tenant/me/authorization`。
- `refreshTenantContext()` 完成 `/me/context` 并确定当前租户后加载授权上下文。
- `switchTenant()` 完成 `/me/switch-tenant` 并写入可信租户后加载授权上下文。
- `switchAccount()` 完成新账号 token、用户资料和租户上下文恢复后加载授权上下文。
- 成员角色或角色权限更新影响当前用户时，保存成功后立即调用 `refreshAuthorization()`。
- Electron 页面刷新时，初始快照只可用于显示恢复中状态，必须重新请求服务端授权后才能进入 `ready`。

### 清空时机

- 登录开始切换账号或添加账号前清空旧授权。
- 账号切换开始时清空旧用户、旧租户、旧角色、旧 permissions、请求短缓存和授权状态。
- 租户切换开始时清空旧租户 permissions，设置 `authorizationStatus=loading`。
- access token refresh 失败、401 过期、退出登录和显式清理会话时清空授权上下文。
- 当前用户被移除租户、租户禁用、授权接口返回 403/成员禁用时清空 permissions 并进入 error 或安全页。

### 切换账号流程

1. 增加 generation，立即清空旧 access token、user、tenantRoles、permissions、currentTenant 和 authorization。
2. 设置 `authStatus=switching-account`、`authorizationStatus=loading`，清空请求 cache。
3. 刷新目标账号 token，恢复目标账号用户信息。
4. 拉取目标账号租户上下文，选择有效当前租户。
5. 若有当前租户，调用 `/tenant/me/authorization`；无租户则进入 `no-tenant` 或 `select-tenant`。
6. 只有 generation 仍匹配时写入状态并渲染受保护菜单。
7. 根据新权限选择落点：优先保守跳到 `/profile`，后续可由安全落点工具跳到首个有权限页面。

### 切换租户流程

1. 设置 `authStatus=switching-tenant`，清空旧 permissions 和授权所属租户。
2. 清空请求 cache 和当前租户主题状态。
3. 调用 `/me/switch-tenant` 更新可信租户。
4. 写入新租户基础上下文，但保持 `authorizationStatus=loading`。
5. 调用 `/tenant/me/authorization` 获取真实角色和权限。
6. 写入新授权并重算菜单、路由和按钮。
7. 若当前路径无权限，跳转到安全落点。

### 页面刷新恢复流程

- 初始 `getAuthSnapshot()` 可以恢复当前用户、access token、租户列表和当前租户用于 skeleton 和请求上下文。
- 只要存在 access token 和 currentUserId，初始 `authReady` 和 `tenantContextReady` 保持非最终态，启动 `refreshTenantContext()`。
- `refreshTenantContext()` 成功后必须接续 `refreshAuthorization()`；失败则 `authorizationStatus=error`。
- 本地保存的 permissions 只允许作为“上次快照”用于调试或非常短暂 skeleton，不允许作为 route/menu ready 的依据。

### 避免旧账号和旧租户权限闪现

- 菜单过滤器和 RouteGuard 必须要求 `authorizationStatus=ready` 且 `authorizationUserId=currentUserId` 且 `authorizationTenantId=currentTenantId`。
- `AppLayout` 在 `authorizationStatus=loading` 时只显示公共安全入口，例如当前用户、租户选择、加载骨架，不显示租户业务菜单。
- `request.ts` 的 `waitForAuthReady()` 应扩展为等待授权状态稳定；租户 API 在授权 loading/error 时不得提前携带旧 `X-Tenant-Id`。
- 所有授权刷新 promise 都绑定 generation，旧 promise 完成后直接丢弃。

### 避免重复请求

- `refreshAuthorization()` 内部维护 `authorizationPromise`，按 `currentUserId + currentTenantId + generation` 去重。
- `request.ts` 继续保留 GET in-flight 去重；授权接口本身使用独立 key，账号、token、租户或 generation 变化时立即清空。
- 页面组件不直接请求 `/tenant/me/authorization`，只调用 `auth.refreshAuthorization()`。

### 授权接口失败处理

- 401：触发现有 token refresh 或会话过期流程。
- 403：清空 permissions，展示权限不足或租户成员不可用提示，跳转 403/安全页。
- 网络失败：`authorizationStatus=error`，保留账号和租户基础信息但不放行受保护菜单、路由或按钮，提供重试。
- 非 JSON 或未知错误：显示后端服务不可用或响应格式错误，不把旧授权标记为成功。

### 是否允许本地持久化 permissions

- 推荐不把 permissions 作为长期可信缓存。
- 为刷新体验可以继续把最近一次 permissions 写入 `go_cpabe_tenant_contexts`，但必须带 `userId`、`tenantId`、更新时间和来源标识。
- 账号、access token、租户、generation 任一变化时，本地 permissions 立即失效。
- 路由、菜单和按钮只在服务端重新加载授权成功后进入 `ready`；本地 permissions 不得使受保护功能直接放行。
- 删除 `permissionsFromRoles()` 作为授权 fallback；如果后端没有返回 permissions，应视为待加载或空权限，而不是从固定角色推导。

## 权限组件设计

### hasPermission / hasAnyPermission / hasAllPermissions

- `hasPermission(code)`：只有 `authorizationStatus=ready` 且授权所属用户、租户匹配当前上下文时，才检查 `permissionsSet.has(code)`。
- `hasAnyPermission(codes)`：空数组返回 false；任一权限命中返回 true。
- `hasAllPermissions(codes)`：空数组返回 true，仅用于无权限要求的公共项；所有权限命中返回 true。
- 所有方法都不读取角色 code，不根据角色名称做判断。

### PermissionGuard

- 建议新增 `desktop/src/renderer/src/auth/PermissionGuard.tsx`。
- 支持 `permission`、`anyPermissions`、`allPermissions`、`mode="hide|disable|fallback"`。
- 用于按钮、操作入口和局部面板；默认隐藏无权限内容。
- 对危险操作保留后端 403 提示处理，因为前端隐藏只是体验优化。

### RouteGuard

- 建议替换或旁路 `RequireTenantRole`，新增 `RequirePermission` 或通用 `RouteGuard`。
- 支持单权限、任一权限、全部权限和平台权限作用域。
- 处理未登录、未选租户、授权加载中、授权失败、无权限和有权限。
- 无权限时渲染 403 页面或跳转到安全落点；需要记录来源路径，避免无限重定向。

### 菜单过滤器

- 建议新增 `desktop/src/renderer/src/auth/menuPermissions.ts` 或 `desktop/src/renderer/src/components/app-menu.tsx`。
- 菜单项元数据包含 `requiredPermission`、`requiredAnyPermissions`、`scope`。
- 过滤器递归过滤 children，父菜单没有可见子菜单时自动隐藏。
- 菜单过滤只依赖 permission code 和授权 ready 状态，不依赖折叠/展开 UI 状态。

### 403 页面、loading 页面、error 页面

- 新增 `desktop/src/renderer/src/pages/ForbiddenPage.tsx`：展示无权限原因、当前租户、返回安全页按钮。
- 可复用现有 `.route-loading` 样式或新增轻量 `AuthorizationLoadingPage`，用于授权恢复、账号切换和租户切换。
- 新增 `AuthorizationErrorPage` 或在 `ForbiddenPage` 支持 error 模式，展示授权加载失败、重试和退出登录入口。

## 页面和组件计划

### 需要新增的文件

- `desktop/src/renderer/src/api/rbac.ts`：当前租户 RBAC API client。
- `desktop/src/renderer/src/auth/PermissionGuard.tsx`：局部权限控制组件。
- `desktop/src/renderer/src/auth/RequirePermission.tsx`：permission-based 路由守卫。
- `desktop/src/renderer/src/auth/permissions.ts`：权限判断工具、菜单过滤和安全落点帮助函数。
- `desktop/src/renderer/src/pages/ForbiddenPage.tsx`：403 和无权限页面。
- `desktop/src/renderer/src/pages/TenantRolesPage.tsx`：租户角色管理页面。
- `desktop/src/renderer/src/components/tenant-rbac/TenantRoleList.tsx`：角色列表、筛选和分组。
- `desktop/src/renderer/src/components/tenant-rbac/TenantRoleDetailDrawer.tsx`：角色详情抽屉。
- `desktop/src/renderer/src/components/tenant-rbac/TenantRoleEditorDrawer.tsx`：创建/编辑角色抽屉。
- `desktop/src/renderer/src/components/tenant-rbac/PermissionSelector.tsx`：权限选择器。
- `desktop/src/renderer/src/components/tenant-rbac/RoleStatusTag.tsx`：角色分类、内置、自定义和状态标签。
- `desktop/src/renderer/src/auth/permissions.test.ts`：权限判断和菜单过滤测试。
- `desktop/src/renderer/src/api/rbac.test.ts` 或扩展 `request.test.ts`：RBAC client 与请求头/错误处理测试。

### 需要修改的文件

- `desktop/src/renderer/src/types.ts`：新增 `PermissionDTO`、`TenantRoleDTO`、`AuthorizationContextDTO`、`MemberRoleDTO`、角色分类和状态类型；扩展现有角色类型时保留兼容。
- `desktop/src/renderer/src/auth/AuthContext.tsx`：扩展授权状态、刷新/清空方法和权限判断方法。
- `desktop/src/renderer/src/api/authRuntime.ts`：同步授权状态、授权所属用户/租户和 generation。
- `desktop/src/renderer/src/api/authStorage.ts`：移除角色生成伪权限 fallback；保存 permissions 时增加失效语义；账号、token、租户变化时清空旧授权。
- `desktop/src/renderer/src/api/request.ts`：租户请求等待授权稳定；cache key 纳入 generation 或授权版本；授权失败不携带旧租户请求头。
- `desktop/src/renderer/src/api/tenant.ts`：保留旧接口兼容，新增或迁移成员角色读取/更新调用到 `rbac.ts`。
- `desktop/src/renderer/src/main.tsx`：新增 `/tenant/roles` 路由；用 permission guard 替换 `RequireTenantRole`。
- `desktop/src/renderer/src/components/AppLayout.tsx`：将菜单配置改为 permission 元数据和过滤器；新增角色管理菜单。
- `desktop/src/renderer/src/components/TenantMemberRoleDialog.tsx`：改为多选、分组、差异预览、完整 roleCodes 提交。
- `desktop/src/renderer/src/pages/TenantMembersPage.tsx`：按 `tenant.member.read/manage` 控制页面和操作；切换到成员多角色 API。
- `desktop/src/renderer/src/pages/TenantOrgManagementPage.tsx`：按 `tenant.org.read/manage` 控制页面和按钮。
- `desktop/src/renderer/src/components/tenant-org/OrgUnitTreePanel.tsx`：操作菜单根据 `canManageOrg` 展示或禁用。
- `desktop/src/renderer/src/components/tenant-org/OrgMemberTable.tsx`：编辑按钮根据 `canManageOrg` 控制。
- `desktop/src/renderer/src/components/tenant-org/OrgMemberDrawer.tsx`：移除或停用系统角色单选，避免与成员多角色弹窗冲突。
- `desktop/src/renderer/src/pages/MyAccessPoliciesPage.tsx`：新建、编辑、删除按 `policy.write` 控制。
- `desktop/src/renderer/src/pages/AccessPolicyBuilderPage.tsx`、`AccessPolicyEditorPage.tsx`：编辑入口和保存入口按 `policy.write` 控制。
- `desktop/src/renderer/src/pages/TenantAccessPolicyViewPage.tsx`：路由改为 `policy.read`。
- `desktop/src/renderer/src/pages/LoginPage.tsx`：登录后平台跳转仍可用平台角色展示，但租户菜单授权不得依赖该判断；如后端后续提供平台 permissions，再迁移到平台 permission。
- `desktop/src/renderer/src/api/authStorage.test.ts`、`request.test.ts`：扩展账号/租户切换和 permissions 失效测试。

## 后端接口契约核对

后端实际代码已存在 `backend/internal/handler/rbac_handler.go`、`TenantRoleService`、`AuthorizationService` 和 `router.go` 中的 `/api/v1/tenant/...` RBAC 路由。前端计划按这些接口接入。

| 能力 | 008 契约 | 实际路由/处理器 | 前端结论 |
|------|----------|----------------|----------|
| 权限目录 | `GET /api/v1/tenant/permissions` | 已注册 `currentTenant.GET("/permissions", ... rbacHandler.Permissions)` | 满足，前端新增 `listTenantPermissions()`。 |
| 角色列表 | `GET /api/v1/tenant/roles` | 已注册 `rbacHandler.Roles` | 满足，用于角色管理页和成员弹窗候选角色。 |
| 创建角色 | `POST /api/v1/tenant/roles` | 已注册 `rbacHandler.CreateRole` | 满足，前端提交 `code/name/description/permissionCodes`。 |
| 编辑角色 | `PATCH /api/v1/tenant/roles/:roleId` | 已注册 `rbacHandler.UpdateRole` | 满足，只编辑名称和描述。 |
| 禁用角色 | `DELETE /api/v1/tenant/roles/:roleId` | 已注册 `rbacHandler.DisableRole` | 满足，前端需显示受影响成员数和内置不可变错误。 |
| 角色权限读取 | `GET /api/v1/tenant/roles/:roleId/permissions` | 已注册 `rbacHandler.RolePermissions` | 满足，用于详情和权限选择器初始值。 |
| 角色权限更新 | `PUT /api/v1/tenant/roles/:roleId/permissions` | 已注册 `rbacHandler.ReplaceRolePermissions` | 满足，全量替换。 |
| 成员角色读取 | `GET /api/v1/tenant/members/:userId/roles` | 已注册 `rbacHandler.MemberRoles` | 满足，用于打开成员角色弹窗时加载完整角色集合。 |
| 成员角色更新 | `PUT /api/v1/tenant/members/:userId/roles` | 已注册 `rbacHandler.ReplaceMemberRoles` | 满足，替代旧单角色接口。 |
| 当前用户授权上下文 | `GET /api/v1/tenant/me/authorization` | 已注册 `rbacHandler.CurrentAuthorization` | 满足，作为前端授权事实源。 |

### 最小适配方案

- 如果前端需要角色详情同时返回权限明细，而 `GET /tenant/roles/:roleId` 当前只返回统计值，则前端通过 `GET /tenant/roles/:roleId/permissions` 组合获取，不要求后端改动。
- 如果 `POST /tenant/roles` 响应直接返回角色 DTO 而不是 `{ role: ... }` envelope data 包装，前端 API client 按实际 data 解析，不要求后端改动。
- 如果后端错误码和前端文案未完全一一对应，前端优先展示后端 `message`，只对常见 code 做增强提示。
- 旧 `PUT /tenants/:id/members/:userId/role` 保留兼容，但新成员多角色弹窗不再使用。

## 实施顺序

1. API 类型和客户端。
   - 工作：在 `types.ts` 和 `api/rbac.ts` 定义 RBAC DTO 与调用方法。
   - 独立验收：mock `request()` 后，所有 RBAC client 方法使用 `/tenant/...` 路径、请求体字段和返回解析符合契约。

2. Authorization Context。
   - 工作：扩展 `AuthContext` 和 `authRuntime`，新增授权状态、所属用户/租户、刷新和清空方法。
   - 独立验收：登录后可调用 `/tenant/me/authorization` 写入 permissions；授权 loading/error 时 `hasPermission()` 返回 false。

3. 账号/租户切换权限清理。
   - 工作：在 `finishLogin`、`switchAccount`、`switchTenant`、`logout`、`clearAuth`、启动恢复中清空或刷新授权。
   - 独立验收：账号 1 切换账号 2 过程中菜单不显示账号 1 权限，租户请求不携带账号 1 的 `tenant_id`。

4. hasPermission 和 Guard。
   - 工作：实现 `hasPermission`、`hasAnyPermission`、`hasAllPermissions`、`PermissionGuard`、`RequirePermission`。
   - 独立验收：单元测试覆盖 ready、loading、error、用户不匹配、租户不匹配、任一权限和全部权限。

5. 菜单权限化。
   - 工作：重构 `AppLayout` 菜单配置，按 permission 递归过滤，增加角色管理菜单。
   - 独立验收：有 `tenant.role.read` 可见角色管理菜单；无该权限不可见；父菜单无子项时隐藏。

6. 路由权限化。
   - 工作：在 `main.tsx` 用 permission guard 替代租户固定角色守卫，新增 `/tenant/roles` 和 403 页面。
   - 独立验收：无权限用户手动输入 `/tenant/roles` 无法进入；有权限用户可进入；无无限重定向。

7. 关键按钮权限化。
   - 工作：改造成员、组织、策略页面关键按钮和危险操作入口。
   - 独立验收：只读权限用户能看列表但不能写；后端 403 仍有明确提示。

8. 角色管理页面。
   - 工作：新增 `TenantRolesPage`、角色列表、筛选、分组、状态和详情入口。
   - 独立验收：内置角色、自定义角色按治理/业务/能力分组展示；不同租户角色不混入。

9. 角色创建编辑。
   - 工作：新增角色创建/编辑抽屉，创建时提交 code/name/description/permissionCodes，编辑时只改 name/description。
   - 独立验收：有 `tenant.role.manage` 可创建；只有 read 权限只能查看；code 冲突显示明确错误。

10. 权限选择器。
    - 工作：按后端权限目录分组展示，支持搜索、全选、取消、已选数量和未保存提示。
    - 独立验收：平台权限不显示；保存自定义角色权限后详情刷新；内置角色无编辑入口。

11. 成员多角色弹窗。
    - 工作：改造 `TenantMemberRoleDialog` 为多选，加载候选角色和成员当前角色，提交完整 `roleCodes`。
    - 独立验收：`DO` 和 `DU` 可同时选择保存；`PLATFORM_ADMIN` 不出现；禁用角色不可新选。

12. 自身角色变化后的权限刷新。
    - 工作：成员角色或角色权限保存成功后，若影响当前用户，则调用 `refreshAuthorization()` 并检查当前路径。
    - 独立验收：修改自己角色后菜单立即变化；失去当前页面权限后跳到安全页。

13. 错误和并发处理。
    - 工作：统一处理 403、409、最后管理员保护、角色禁用、权限变更、网络失败。
    - 独立验收：错误提示不再笼统为“操作失败”；网络失败不保留授权成功状态。

14. 单元测试。
    - 工作：覆盖 `authStorage` permissions 失效、`request` 租户头隔离、权限判断、菜单过滤、API client。
    - 独立验收：`npm run test` 通过，新增测试可证明账号和租户切换不串权限。

15. 组件和流程测试。
    - 工作：为角色页面、权限选择器、成员弹窗补充组件级或集成式测试；若测试环境不足，形成手工验收脚本。
    - 独立验收：覆盖规格验收清单中角色管理和成员多角色核心路径。

16. Electron 回归测试。
    - 工作：运行 `typecheck`、`build:renderer`，手工验证 Electron 刷新、登录、账号切换、租户切换、组织和策略页面。
    - 独立验收：原有租户切换、组织管理和策略页面不回归。

## 风险分析

- **旧 permissions 短暂闪现**：本地上下文已有 permissions，刷新时可能先渲染旧菜单。缓解：菜单/路由必须要求 `authorizationStatus=ready` 且 user/tenant 匹配。
- **全局 localStorage tenant_id 污染**：旧全局 key 仍可能存在。缓解：继续迁移到 `go_cpabe_tenant_contexts`，并在清理会话时删除全局旧 key。
- **切换账号后请求竞态**：账号 1 的请求可能晚于账号 2 返回。缓解：所有授权和租户上下文写入检查 generation。
- **切换租户后旧请求回写状态**：租户 A 的授权请求可能覆盖租户 B。缓解：授权 promise key 包含 userId、tenantId、generation。
- **菜单隐藏但路由未保护**：用户手动输入 URL 仍可进。缓解：所有受保护路由使用 permission guard。
- **组件继续直接判断角色**：遗漏页面可能仍用 `roles.includes()` 授权。缓解：tasks 阶段加入固定角色授权扫描任务，展示标签与授权判断分开处理。
- **修改自身角色后页面失效**：保存成功后当前页面可能无权限。缓解：影响当前用户时刷新授权并调用安全落点计算。
- **当前页面失权后的重定向循环**：安全落点也可能无权限。缓解：安全落点优先公共 `/profile`，无任何租户权限时显示 403，不反复跳转。
- **内置角色被误编辑**：UI 可能只禁用部分按钮。缓解：角色 DTO 的 `builtin/isBuiltin` 同时控制编辑、权限保存和禁用入口；后端错误仍兜底。
- **权限选择器绑定平台权限**：权限目录或前端过滤错误会提交平台权限。缓解：前端只使用 `/tenant/permissions`，并二次过滤 `scopeType=TENANT`；后端 `INVALID_PERMISSION_SCOPE` 兜底。
- **API DTO 与后端不一致**：后端部分响应是直接 DTO，部分是 `{items}`。缓解：API client 按实际 `RBACHandler` 响应解析，并用契约测试固定。
- **Electron 刷新后的状态恢复**：刷新时本地快照可能旧。缓解：刷新后进入恢复中，服务端授权成功前不放行租户业务功能。

## 宪章检查

*GATE：计划前检查通过；设计后复查通过。*

- **混合加密边界**：本功能不涉及 AES-GCM、RSA-OAEP、CP-ABE 或 DEK 封装。权限仅控制前端功能入口和请求体验，不改变文件加密链路。
- **真实 CP-ABE 实现**：本功能不新增或替换 CP-ABE 实现，不使用模拟加密逻辑。
- **模块边界**：前端授权体验、租户角色管理、成员多角色、组织和策略按钮权限分别在前端 auth、api、components、pages 中实现；后端仍是最终安全边界；Crypto、File、Benchmark、Audit 不在本阶段改造。
- **算法对比口径**：本功能不涉及算法性能对比，不改变 AES 文件加密耗时与 DEK 封装/解封装统计口径。
- **可解释性**：角色页展示内置/自定义、分类、权限数量、成员数量和只读状态；403/错误页解释当前权限不足或授权加载失败；权限判断以 permission code 为准。
- **中文文档**：本 feature 的 `spec.md`、`plan.md`、`research.md`、`data-model.md`、`quickstart.md` 和 `contracts/` 均使用简体中文。
- **Go 注释策略**：本计划不主动修改 Go 业务代码。若实现阶段发现必须做最小后端适配，所有新增或修改 Go 函数/方法必须有中文前置注释，导出标识符符合 GoDoc，Handler、Service、Repository、Middleware 说明业务语义、副作用、权限前置条件和安全边界。
- **关键注释和可读性检查**：即使本阶段主要为前端，后续 `tasks.md` 仍需包含“关键注释和可读性检查”任务；若没有 Go 代码改动，则检查项记录为“不涉及 Go 业务代码”，同时检查前端授权逻辑命名和注释是否清晰。

## 项目结构

### 文档（本功能）

```text
specs/009-tenant-rbac-frontend/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── frontend-rbac-api.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### 源码（仓库根目录）

```text
desktop/
├── package.json
├── vite.config.ts
└── src/
    ├── main/
    │   ├── main.ts
    │   └── authSessionStore.ts
    ├── preload/
    │   └── preload.ts
    └── renderer/
        └── src/
            ├── api/
            │   ├── request.ts
            │   ├── authStorage.ts
            │   ├── authRuntime.ts
            │   ├── tenant.ts
            │   ├── tenantOrg.ts
            │   ├── policy.ts
            │   └── rbac.ts
            ├── auth/
            │   ├── AuthContext.tsx
            │   ├── RequireAuth.tsx
            │   ├── RequirePermission.tsx
            │   ├── PermissionGuard.tsx
            │   └── permissions.ts
            ├── components/
            │   ├── AppLayout.tsx
            │   ├── TenantMemberRoleDialog.tsx
            │   ├── tenant-org/
            │   └── tenant-rbac/
            ├── pages/
            │   ├── ForbiddenPage.tsx
            │   ├── TenantRolesPage.tsx
            │   ├── TenantMembersPage.tsx
            │   ├── TenantOrgManagementPage.tsx
            │   ├── MyAccessPoliciesPage.tsx
            │   └── TenantAccessPolicyViewPage.tsx
            └── types.ts
```

**结构决策**：只在现有 Electron 桌面端结构下新增 `api/rbac.ts`、`auth` 权限工具、`tenant-rbac` 组件目录和 `TenantRolesPage`；不新增新的前端应用、不引入新的状态管理库。

## 复杂度跟踪

无宪章违规项；本阶段复杂度来自现有多账号、多租户状态，需要通过统一授权上下文降低风险。
