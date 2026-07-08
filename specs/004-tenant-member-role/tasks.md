# 任务列表：租户成员角色分配

**输入**：`specs/004-tenant-member-role/` 下的 `spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/tenant-member-role-api.md`、`quickstart.md`

**前置条件**：已完成多租户基础能力、平台管理员能力、租户成员列表能力；本任务列表只拆分“租户管理员给本租户成员分配普通业务角色”的增量实现。

**组织方式**：按可独立交付的功能模块拆分，并保留 SpecKit 要求的复选框格式。每个用户故事阶段都能独立验证一个业务增量。

## Phase 1：准备与边界确认

**目标**：确认当前角色编码、接口位置、错误码和数据库索引现状，避免实现阶段扩大范围。

- [X] T001 梳理现有租户成员、角色、平台管理员相关实现并记录影响点 in backend/internal/service/tenant_service.go, backend/internal/service/platform_service.go, backend/internal/repository/tenant_repository.go, desktop/src/renderer/src/pages/PlatformTenantUsersPage.tsx
- [X] T002 [P] 确认角色编码映射和错误码命名方案 in backend/internal/domain/constants.go, backend/internal/pkg/response/errors.go, desktop/src/renderer/src/types.ts
- [X] T003 [P] 检查并记录角色分配所需索引是否已存在 in backend/migrations/002_create_tenants_roles.sql, backend/migrations/003_add_tenant_query_indexes.sql

---

## Phase 2：后端事务与权限基础

**目标**：建立普通业务角色替换的事务能力和严格租户管理员权限边界，阻塞所有接口实现。

- [X] T004 [P] 增加租户普通业务角色映射和校验辅助逻辑 in backend/internal/service/tenant_service.go
- [X] T005 [P] 增加角色分配相关业务错误 in backend/internal/pkg/response/errors.go
- [X] T006 扩展 TenantRepository 事务替换普通业务角色能力 in backend/internal/repository/tenant_repository.go
- [X] T007 [P] 同步内存测试仓储的事务替换行为 in backend/internal/service/tenant_test_helpers_test.go, backend/internal/handler/test_helpers_test.go

**检查点**：Repository 已能在一次原子操作中替换 `DO/DU`，并且不会处理 `PLATFORM_ADMIN` 或 `TENANT_ADMIN`。

---

## Phase 3：用户故事 1 - 分配数据拥有者（优先级：P1） MVP

**目标**：租户管理员可以把本租户普通成员设置为数据拥有者。

**独立测试**：使用租户 A 的 `TENANT_ADMIN` 给租户 A active 成员保存 `DATA_OWNER`，成员列表回显 `DO`，同租户同用户只保留一个普通业务角色。

### 测试

- [X] T008 [P] [US1] 编写服务层数据拥有者分配测试 in backend/internal/service/tenant_service_test.go
- [X] T009 [P] [US1] 编写接口层数据拥有者分配测试 in backend/internal/handler/tenant_role_test.go

### 实现

- [X] T010 [US1] 在 TenantService 中实现 AssignTenantMemberBusinessRole 业务流程 in backend/internal/service/tenant_service.go
- [X] T011 [US1] 在 TenantHandler 中实现角色分配请求解析和响应 in backend/internal/handler/tenant_handler.go
- [X] T012 [US1] 注册 PUT /api/v1/tenants/:tenantId/members/:userId/role 路由 in backend/internal/handler/router.go
- [X] T013 [US1] 确保成员列表保存后可返回最新角色 in backend/internal/service/tenant_service.go, backend/internal/repository/tenant_repository.go

**检查点**：数据拥有者分配主链路可独立演示。

---

## Phase 4：用户故事 2 - 分配数据访问者与幂等替换（优先级：P1）

**目标**：租户管理员可以把普通成员设置为数据访问者，并支持 `DO` 与 `DU` 之间相互替换。

**独立测试**：先给成员分配 `DATA_OWNER`，再保存 `DATA_VISITOR`，确认旧 `DO` 被替换为 `DU`；重复保存 `DATA_VISITOR` 不产生重复记录。

### 测试

- [X] T014 [P] [US2] 编写数据访问者分配和角色替换测试 in backend/internal/service/tenant_service_test.go
- [X] T015 [P] [US2] 编写重复保存幂等测试 in backend/internal/repository/tenant_repository_test.go 或 backend/internal/service/tenant_service_test.go

### 实现

- [X] T016 [US2] 完善 DATA_VISITOR 到 DU 的映射和响应回显 in backend/internal/service/tenant_service.go
- [X] T017 [US2] 确保事务失败会回滚旧角色状态 in backend/internal/repository/tenant_repository.go

**检查点**：普通业务角色单一性、幂等性和事务安全可独立验证。

---

## Phase 5：用户故事 3 - 权限和越权保护（优先级：P1）

**目标**：平台管理员、普通成员、其他租户管理员均不能通过普通角色分配接口越权写入普通业务角色。

**独立测试**：分别使用平台管理员、普通成员、其他租户管理员、目标租户管理员本人调用接口，确认只有合法场景成功，非法场景返回明确错误。

### 测试

- [X] T018 [P] [US3] 编写平台管理员被普通角色分配接口拒绝的测试 in backend/internal/service/tenant_service_test.go, backend/internal/handler/tenant_role_test.go
- [X] T019 [P] [US3] 编写普通成员和跨租户管理员越权测试 in backend/internal/handler/tenant_role_test.go
- [X] T020 [P] [US3] 编写禁止分配 TENANT_ADMIN 和禁止修改自己 TENANT_ADMIN 的测试 in backend/internal/service/tenant_service_test.go

### 实现

- [X] T021 [US3] 收紧新角色分配接口权限为目标租户 TENANT_ADMIN，禁止 PLATFORM_ADMIN 兜底放行 in backend/internal/service/tenant_service.go
- [X] T022 [US3] 增加非法角色、非成员、禁用成员、禁用租户的错误处理 in backend/internal/service/tenant_service.go, backend/internal/pkg/response/errors.go
- [X] T023 [US3] 在权限校验关键分支补充中文块级注释说明平台管理员和租户管理员职责边界 in backend/internal/service/tenant_service.go

**检查点**：所有越权场景拒绝率达到规格要求。

---

## Phase 6：用户故事 4 - 前端角色列和分配弹窗（优先级：P2）

**目标**：租户成员列表可展示角色，并让租户管理员通过弹窗分配数据拥有者或数据访问者。

**独立测试**：租户管理员打开成员列表，看到角色列和分配按钮；弹窗只显示数据拥有者、数据访问者；保存成功后关闭并刷新列表。

### 实现

- [X] T024 [P] [US4] 扩展前端类型和角色展示文案 in desktop/src/renderer/src/types.ts
- [X] T025 [P] [US4] 新增租户成员角色分配 API 调用 in desktop/src/renderer/src/api/tenant.ts
- [X] T026 [US4] 在成员列表页面增加角色列和分配角色按钮 in desktop/src/renderer/src/pages/PlatformTenantUsersPage.tsx 或对应租户成员页
- [X] T027 [US4] 实现角色分配弹窗并限制选项为数据拥有者和数据访问者 in desktop/src/renderer/src/components/TenantMemberRoleDialog.tsx
- [X] T028 [US4] 保存成功后关闭弹窗并刷新成员列表，失败时展示明确错误 in desktop/src/renderer/src/pages/PlatformTenantUsersPage.tsx
- [X] T029 [US4] 对 PLATFORM_ADMIN 隐藏或禁用普通业务角色分配入口并展示说明 in desktop/src/renderer/src/pages/PlatformTenantUsersPage.tsx

**检查点**：前端角色分配闭环可演示，不影响现有布局和主题色。

---

## Phase 7：验证、文档和可读性收尾

**目标**：完成测试、类型检查、构建、注释和文档校验。

- [X] T030 [P] 更新接口说明或 README 中的租户成员角色分配说明 in backend/README.md 或 specs/004-tenant-member-role/contracts/tenant-member-role-api.md
- [X] T031 Go 关键注释和可读性检查：确认新增或修改的 Go 业务代码中每个函数/方法都有前置中文注释，导出标识符符合 GoDoc 前缀规范，Handler、Service、Repository、Middleware 注释解释权限、事务、副作用和安全边界，并移除无意义逐行注释
- [X] T032 运行 gofmt 和 go test ./... in backend/
- [X] T033 运行 npm run typecheck 和 npm run build in desktop/
- [ ] T034 按 quickstart.md 完成端到端验收并记录结果 in specs/004-tenant-member-role/quickstart.md

---

## 依赖与执行顺序

### 阶段依赖

- **Phase 1**：无依赖，可立即开始。
- **Phase 2**：依赖 Phase 1 的边界确认，阻塞所有后端故事。
- **US1 / Phase 3**：依赖 Phase 2，是 MVP。
- **US2 / Phase 4**：依赖 Phase 3 的主链路，可在后端事务能力稳定后补充。
- **US3 / Phase 5**：依赖 Phase 2，可与 US1/US2 部分并行，但最终必须在前端联调前完成。
- **US4 / Phase 6**：依赖后端接口契约稳定。
- **Phase 7**：依赖计划内实现完成。

### 用户故事依赖

- **US1**：MVP，完成后可独立分配数据拥有者。
- **US2**：在 US1 基础上补齐数据访问者、替换和幂等。
- **US3**：与 US1/US2 同等重要，保证权限安全边界。
- **US4**：在后端接口稳定后补齐前端交互闭环。

### 并行机会

- T002、T003 可与 T001 并行。
- T004、T005、T007 可并行，T006 需要统一 repository 接口后落地。
- US1 的服务层测试和接口层测试可并行。
- US3 的平台管理员、普通成员、跨租户越权测试可并行。
- 前端类型/API 与弹窗组件可并行开发。

## 实施策略

### MVP 优先

1. 完成 Phase 1 和 Phase 2。
2. 完成 US1，先让租户管理员可以分配数据拥有者。
3. 立即验证接口、成员列表回显和数据库唯一性。

### 增量交付

1. US1：数据拥有者分配。
2. US2：数据访问者、替换、幂等和事务回滚。
3. US3：权限拒绝和错误提示完整化。
4. US4：前端角色列、弹窗和刷新闭环。

### 注意事项

- 不要把平台管理员兜底指定 `TENANT_ADMIN` 的接口和普通业务角色分配接口混在一起。
- 不要改动登录、多租户选择、租户创建、用户创建等既有能力。
- 不要把角色分配解释为 CP-ABE 解密授权；后续文件解密仍必须走属性和策略判断。
- AI 新增或修改 Go 代码时必须同步补充高质量中文注释。
