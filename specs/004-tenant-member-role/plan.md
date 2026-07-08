# 实现计划：租户成员角色分配

**分支**：`004-tenant-member-role` | **日期**：2026-07-08 | **规格**：[spec.md](./spec.md)

**输入**：来自 `specs/004-tenant-member-role/spec.md` 的功能规格。

## 摘要

本功能在已有多租户、平台管理员、租户成员管理和成员列表能力之上，补齐“租户管理员给本租户普通成员分配普通业务角色”的闭环。核心范围只包含数据拥有者和数据访问者两类普通业务角色分配；平台管理员指定租户管理员继续走平台后台兜底能力，不混入普通角色分配弹窗。

后端新增租户内角色分配接口，使用当前登录用户和路径中的 `tenantId/userId` 做权限校验，确保只有目标租户的 `TENANT_ADMIN` 可以操作本租户有效成员。保存角色时由 service 层协调 repository 事务，先删除目标用户在当前租户下旧的普通业务角色，再写入新的普通业务角色，保证原子性和幂等性。

前端在租户成员列表中补充角色列和角色分配弹窗。弹窗只展示数据拥有者和数据访问者，不展示 `PLATFORM_ADMIN` 或 `TENANT_ADMIN`。保存成功后刷新成员列表；失败时展示明确错误。

## 技术上下文

**语言/版本**：Go 1.23；TypeScript 5.7；React 19；Electron 33；Vite 6。

**主要依赖**：Gin、Gorm、MySQL Driver、Redis Go Client、JWT v5、bcrypt；前端 React Router、Electron、Vite。

**存储**：复用 MySQL 中的 `roles`、`user_roles`、`tenant_users`、`users`、`tenants`；不新增角色表。

**测试**：后端使用 Go `testing`、`httptest`、内存仓储和现有测试辅助；前端使用 TypeScript 类型检查和构建校验，必要时补充页面手动验收。

**目标平台**：本地开发和演示环境中的 Go Web API + Electron 桌面端。

**项目类型**：后端 Web API + 桌面客户端。

**性能目标**：单次角色分配在演示规模下应即时完成；成员列表刷新不引入 N+1 查询。

**约束**：

- 本功能不重做登录、租户列表、租户详情、创建租户、创建用户和成员列表基础查询。
- 普通角色分配接口只允许目标租户内 `TENANT_ADMIN` 调用；`PLATFORM_ADMIN` 不通过本接口写入普通业务角色。
- `DATA_OWNER` / `DATA_VISITOR` 是本功能对外业务命名，现有系统内部角色编码为 `DO` / `DU`，实现必须显式映射。
- 所有 Go 新增或修改函数、方法必须有中文注释，导出标识符符合 GoDoc。
- 保存角色必须使用数据库事务。

**规模/范围**：补齐后端角色分配接口、repository 事务方法、错误码、测试、前端成员列表角色列、角色分配弹窗和 API 调用。不包含完整 RBAC、文件管理、访问策略、密钥管理或加密算法模块。

## 宪章检查

*门禁：Phase 0 前必须通过，Phase 1 后再次检查。*

- **混合加密边界**：通过。本功能不涉及 AES-GCM、RSA-OAEP、CP-ABE 或 DEK 封装，不改变文件加密主链路。
- **真实 CP-ABE 实现**：通过。本功能不实现或模拟 CP-ABE，加密和解密仍由后续 Crypto 模块负责。
- **模块边界**：通过。角色分配属于租户管理和访问控制边界，落在 Handler、Service、Repository、Middleware 相关模块，不进入 Crypto、Policy、File、Benchmark。
- **算法对比口径**：通过。本功能不记录或展示算法耗时，不产生 RSA 与 CP-ABE 性能结论。
- **可解释性**：通过。规格和界面将明确平台管理员、租户管理员、数据拥有者、数据访问者的职责边界，避免把平台管理解释为文件解密权限。
- **中文文档**：通过。本 feature 的 SpecKit 文档使用简体中文。
- **Go 注释策略**：通过。新增 Handler、Service、Repository、测试辅助函数必须有前置中文注释；权限校验、平台管理员拒绝、事务写入、角色映射处必须用块级注释解释原因。
- **关键注释和可读性检查**：通过。`tasks.md` 将包含专门检查任务，覆盖函数/方法注释、GoDoc 前缀、业务语义、安全边界和无意义注释清理。

Phase 1 设计后复查：通过。数据模型、接口契约和快速验证指南均保持本阶段边界，没有引入加密算法、密钥明文或完整 RBAC 后台。

## 项目结构

### 文档结构

```text
specs/004-tenant-member-role/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── tenant-member-role-api.md
└── checklists/
    └── requirements.md
```

### 源码结构

```text
backend/
├── internal/
│   ├── domain/               # 复用 RoleCode、TenantMemberDTO，必要时补充普通业务角色 DTO
│   ├── repository/           # 扩展 TenantRepository 事务写入方法
│   ├── service/              # 扩展 TenantService 或新增 TenantRoleService
│   ├── handler/              # 扩展 TenantHandler 和路由
│   ├── middleware/           # 复用 AuthRequired，角色分配接口在 service 内校验租户管理员
│   └── pkg/response/         # 增加角色分配相关错误码
└── migrations/               # 如索引缺失，补充建议迁移或确认现有索引

desktop/
└── src/renderer/src/
    ├── api/                  # 扩展 tenant.ts 或新增 tenantRoles.ts
    ├── pages/                # 扩展租户成员列表页面或新增租户成员管理页
    ├── components/           # 新增角色分配弹窗组件
    └── types.ts              # 扩展普通业务角色类型与接口返回类型
```

**结构决策**：后端沿用现有 `domain -> repository -> service -> handler -> router` 分层。普通业务角色分配属于租户内管理能力，优先放在租户 service 侧，并明确不复用平台后台的 `PlatformRoleService.AssignTenantAdmin`。前端优先扩展已有成员列表页面和 API 层，不创建独立权限后台。

## Phase 0 研究结论

详见 [research.md](./research.md)。关键决策如下：

- 普通角色分配使用租户内接口 `PUT /api/v1/tenants/:tenantId/members/:userId/role`。
- 该接口必须只允许目标租户 `TENANT_ADMIN` 调用，不能因为用户拥有 `PLATFORM_ADMIN` 而放行。
- 对外请求值采用 `DATA_OWNER` / `DATA_VISITOR`，服务层映射到现有内部角色 `DO` / `DU`。
- 保存角色使用 repository 事务方法：删除目标用户在当前租户下旧的普通业务角色，再幂等写入新角色。
- 成员列表继续复用现有成员列表能力，但必须确保返回当前租户角色用于回显。
- 数据库优先复用现有表，索引重点关注 `user_roles(user_id, tenant_id, role_id)`、`user_roles(tenant_id, user_id)`、`roles(code)`、`tenant_users(tenant_id, user_id)`。

## Phase 1 设计产物

- 数据模型：[data-model.md](./data-model.md)
- 接口契约：[contracts/tenant-member-role-api.md](./contracts/tenant-member-role-api.md)
- 快速验证指南：[quickstart.md](./quickstart.md)

## 实现策略

### 后端接口策略

- 在普通租户路由下新增 `PUT /api/v1/tenants/:tenantId/members/:userId/role`。
- 请求体使用 `roleCode` 字段，允许值为 `DATA_OWNER` 或 `DATA_VISITOR`。
- Handler 只负责解析路径参数、请求体和当前用户 ID；权限、角色映射和事务写入放在 Service。
- Service 校验顺序：
  1. 当前用户必须是目标租户 `TENANT_ADMIN`。
  2. 当前用户不能通过 `PLATFORM_ADMIN` 身份走本接口。
  3. 目标租户必须存在且启用。
  4. 目标用户必须是该租户 active 成员。
  5. 当前用户不能修改自己的 `TENANT_ADMIN` 角色。
  6. 请求角色必须映射到 `DO` 或 `DU`。
- Repository 提供事务方法，例如 `ReplaceTenantBusinessRole(ctx, tenantID, userID, roleCode)`。

### 事务和幂等策略

- 普通业务角色集合固定为 `DO`、`DU`。
- 保存时在一个事务中删除 `tenant_id = 当前租户`、`user_id = 目标用户`、`role_id IN (DO, DU)` 的旧记录。
- 随后写入新的 `tenant_id/user_id/role_id`。
- 重复保存同一角色时，删除后重新写入或先判断后写入都可以，但最终必须只保留一条有效记录。
- 如果写入失败，事务回滚，旧角色保持不变。

### 前端交互策略

- 在租户成员列表中新增或确认“角色”列。
- 每行根据当前用户身份显示“分配角色”或“修改角色”按钮。
- 当前用户是目标租户 `TENANT_ADMIN` 时显示可用按钮；`PLATFORM_ADMIN` 不显示普通业务角色分配入口，或展示禁用态和说明。
- 弹窗展示昵称、邮箱、当前角色，只展示数据拥有者和数据访问者选项。
- 保存成功后关闭弹窗并重新拉取成员列表。
- 保存失败时使用统一错误解析展示明确消息。

## 数据库与索引策略

- 复用 `roles`、`user_roles`、`tenant_users`。
- 不新增角色表，不新增完整权限菜单表。
- 如索引缺失，补充迁移或至少在实施说明中建议：
  - `user_roles(user_id, tenant_id, role_id)`
  - `user_roles(tenant_id, user_id)`
  - `roles(code)`
  - `tenant_users(tenant_id, user_id)`
- 如果 MySQL 因 `NULL` 唯一索引无法约束平台角色重复，本功能不处理平台角色写入；普通业务角色绑定非空 `tenant_id`，可继续依赖唯一约束和事务幂等保护。

## 风险和缓解

- **角色编码混淆**：现有系统内部使用 `DO/DU`，需求中使用 `DATA_OWNER/DATA_VISITOR`。缓解方式是在 Service 层集中映射，并在接口契约中明确。
- **平台管理员误获得普通业务角色**：角色分配接口显式拒绝非目标租户 `TENANT_ADMIN` 的调用，且不因平台角色放行。
- **半完成写入**：使用数据库事务包裹删除旧角色和写入新角色。
- **成员列表回显不准确**：保存成功后前端强制刷新成员列表，后端成员列表继续返回 `roles`。

## 复杂度跟踪

无宪章违规项。本功能复用既有多租户模型和表结构，不引入新的权限后台或加密模块。
