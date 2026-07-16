# 研究记录：租户 RBAC 前端授权与角色管理

## 决策 1：继续使用 AuthContext 承载授权上下文

**决策**：不新增独立 `TenantContext`，在现有 `AuthContext` 中扩展 `authorizationStatus`、`authorizationUserId`、`authorizationTenantId`、`authorizationGeneration`、`authorizationError`、`refreshAuthorization()` 和权限判断方法。

**理由**：当前账号切换、租户切换、请求运行时、主题品牌和本地上下文都已经由 `AuthContext` 协调。新增独立上下文会增加两个状态源，反而更容易在账号切换和租户切换时出现旧权限回写。

**替代方案**：新建 `AuthorizationProvider`。该方案可以隔离职责，但必须与 `AuthContext` 做复杂同步；在当前代码规模下收益不足。

## 决策 2：permissions 不作为长期可信缓存

**决策**：允许本地保存最近一次 permissions 作为恢复快照，但菜单、路由和按钮只有在 `/tenant/me/authorization` 服务端重新加载成功后才进入授权就绪状态。

**理由**：本地 permissions 可能来自旧账号、旧租户、旧 token 或旧角色，直接放行会造成短暂闪现和错误请求头。服务端授权上下文才是事实源。

**替代方案**：完全不保存 permissions。安全性最好，但刷新体验较硬；当前可通过 skeleton 和快速重新加载平衡体验。

## 决策 3：功能授权统一使用 permission code

**决策**：菜单、路由和按钮统一调用 `hasPermission`、`hasAnyPermission`、`hasAllPermissions`，不得使用 `TENANT_ADMIN`、`DO`、`DU`、`PLATFORM_ADMIN` 作为租户功能授权来源。

**理由**：角色名称和角色 code 可以变成展示或分组数据，但功能访问必须来自后端 RBAC 权限并集，才能支持租户自定义角色和角色名称修改不影响授权。

**替代方案**：继续按内置角色授权，自定义角色只作展示。该方案无法满足租户自定义角色管理和权限选择器目标。

## 决策 4：使用当前租户 RBAC API，不复用旧单角色接口

**决策**：新增前端 `api/rbac.ts` 调用 `/tenant/permissions`、`/tenant/roles`、`/tenant/members/:userId/roles`、`/tenant/me/authorization` 等当前租户上下文接口；旧 `/tenants/:id/members/:userId/role` 只保留兼容。

**理由**：当前租户接口由后端从 `X-Tenant-Id` 解析可信租户，不需要前端在 URL 中传租户 ID 作为事实边界；成员多角色也需要完整 roleCodes 替换接口。

**替代方案**：扩展旧路径式接口。该方案会继续携带 tenantId 参数，不符合后端 008 契约推荐方向。

## 决策 5：权限组件采用轻量组件和工具函数

**决策**：新增 `PermissionGuard`、`RequirePermission` 和菜单过滤工具，不引入新的状态管理库或路由框架。

**理由**：现有应用已经使用 React Context、React Router 和 Ant Design；轻量组件能贴合现有结构，降低重构面。

**替代方案**：引入 RBAC 专用状态库或路由元框架。功能量不足以抵消依赖和迁移成本。

## 决策 6：测试优先覆盖状态隔离和授权边界

**决策**：Vitest 优先覆盖 authStorage、request、权限判断、菜单过滤、API client、关键组件交互；Electron 手工回归覆盖刷新、账号切换和租户切换。

**理由**：最大风险在旧授权闪现、租户头污染和请求竞态，这些可以用单元测试稳定捕获；完整 Electron 流程需要实际窗口和后端环境，适合作为手工验收或后续端到端测试。

**替代方案**：计划阶段引入 Playwright 端到端测试。当前项目尚未配置 Playwright，会扩大本阶段范围。
