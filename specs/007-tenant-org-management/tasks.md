# 任务清单：租户组织架构管理

**输入**：`specs/007-tenant-org-management/` 下的 `spec.md`、`plan.md`、`data-model.md`、`contracts/api.md`、`quickstart.md`

**前置依赖**：`006-tenant-org-attributes` 已交付组织、属性和策略构建基础设施；本任务清单不得修改 `specs/006-tenant-org-attributes/`。

**组织方式**：按用户故事分阶段，每个故事可独立验证。测试任务明确纳入计划，因为本功能涉及租户隔离、数据迁移、事务和权限边界。

## 阶段 1：基础准备

**目标**：确认旧数据影响范围，建立新 feature 的迁移和类型基础。

- [X] T001 统计旧部门职务数据并记录结果 in `specs/007-tenant-org-management/quickstart.md`
- [X] T002 新增幂等数据库迁移脚本 in `backend/migrations/008_tenant_org_management.sql`
- [X] T003 [P] 更新部门职务枚举与成员主部门字段模型 in `backend/internal/domain/org.go`
- [X] T004 [P] 更新组织管理相关错误码 in `backend/internal/pkg/response/errors.go`
- [X] T005 调整启动迁移模型注册以包含 `is_primary` 字段 in `backend/cmd/server/main.go`

---

## 阶段 2：基础仓储与服务边界

**目标**：建立新旧接口可复用的组织管理 Service，确保后续用户故事不会分叉实现。

- [X] T006 扩展组织仓储接口支持部门 CRUD、子树锁定、成员聚合和职务查询 in `backend/internal/repository/org_attribute_repository.go`
- [X] T007 实现主部门和部门职务事务所需的 Gorm 仓储方法 in `backend/internal/repository/org_attribute_repository.go`
- [X] T008 新增组织管理 Service 骨架并复用当前租户 Actor in `backend/internal/service/org_management_service.go`
- [X] T009 新增当前租户组织管理 Handler 骨架 in `backend/internal/handler/org_management_handler.go`
- [X] T010 注册 `/api/v1/tenant/...` 路由并保留旧接口桥接入口 in `backend/internal/handler/router.go`

---

## 阶段 3：用户故事 1 - 租户管理员维护部门树（P1，MVP）

**目标**：租户管理员可以查询、创建、编辑、移动、停用和删除当前租户部门，并同步部门属性值。

**独立测试**：以当前租户 `TENANT_ADMIN` 调用 `/api/v1/tenant/org-units/tree`、`POST/PUT/PUT move/DELETE`，验证部门树和属性值同事务一致。

### 测试

- [ ] T011 [P] [US1] 编写部门树 Service 测试 in `backend/internal/service/org_management_service_test.go`
- [ ] T012 [P] [US1] 编写部门树 Handler 测试 in `backend/internal/handler/org_management_handler_test.go`

### 实现

- [X] T013 [US1] 实现当前租户部门树查询 in `backend/internal/service/org_management_service.go`
- [X] T014 [US1] 实现部门稳定编码生成和创建部门事务 in `backend/internal/service/org_management_service.go`
- [X] T015 [US1] 实现部门改名、排序、状态更新和属性值同步 in `backend/internal/service/org_management_service.go`
- [X] T016 [US1] 实现部门移动、循环校验、子树 path/level 更新和 value_path 同步 in `backend/internal/service/org_management_service.go`
- [X] T017 [US1] 实现部门删除保护和停用规则 in `backend/internal/service/org_management_service.go`
- [X] T018 [US1] 实现部门树接口请求绑定和响应封装 in `backend/internal/handler/org_management_handler.go`
- [X] T019 [US1] 确认旧 `/tenants/:id/org-units` 写接口复用新 Service in `backend/internal/handler/org_attribute_handler.go`

**检查点**：US1 可单独演示部门树管理和属性值同步，不依赖成员页面。

---

## 阶段 4：用户故事 2 - 租户管理员维护部门成员和主部门（P1）

**目标**：租户管理员可以查询组织成员、加入部门、设置主部门和移除部门关系，并满足主部门一致性规则。

**独立测试**：构造一个用户加入 0、1、2 个部门，验证自动主部门、主部门切换、删除主部门和删除最后部门关系。

### 测试

- [ ] T020 [P] [US2] 编写成员归属和主部门 Service 测试 in `backend/internal/service/org_management_service_test.go`
- [ ] T021 [P] [US2] 编写成员接口 Handler 测试 in `backend/internal/handler/org_management_handler_test.go`

### 实现

- [X] T022 [US2] 实现当前租户组织成员聚合查询 in `backend/internal/service/org_management_service.go`
- [X] T023 [US2] 实现加入部门并恢复旧成员关系 in `backend/internal/service/org_management_service.go`
- [X] T024 [US2] 实现唯一主部门事务规则 in `backend/internal/service/org_management_service.go`
- [X] T025 [US2] 实现移除部门关系和删除主部门后的新主部门选择 in `backend/internal/service/org_management_service.go`
- [X] T026 [US2] 实现成员管理接口请求绑定和响应封装 in `backend/internal/handler/org_management_handler.go`
- [X] T027 [US2] 确认成员接口不接收也不修改 `systemRoles` in `backend/internal/handler/org_management_handler.go`

**检查点**：US2 可单独验证部门归属和主部门，不依赖部门职务。

---

## 阶段 5：用户故事 3 - 租户管理员维护部门职务（P1）

**目标**：部门职务只保存 `ORG_LEADER` 和 `DEPUTY_LEADER`，且系统角色继续走现有角色接口。

**独立测试**：同一部门设置负责人和多个副负责人，非法职务和系统角色写入全部被拒绝。

### 测试

- [ ] T028 [P] [US3] 编写部门职务白名单和唯一负责人 Service 测试 in `backend/internal/service/org_management_service_test.go`
- [ ] T029 [P] [US3] 编写部门职务接口 Handler 测试 in `backend/internal/handler/org_management_handler_test.go`

### 实现

- [X] T030 [US3] 实现部门职务白名单校验 in `backend/internal/service/org_management_service.go`
- [X] T031 [US3] 实现每部门最多一个 `ORG_LEADER` 的事务校验 in `backend/internal/service/org_management_service.go`
- [X] T032 [US3] 实现多个 `DEPUTY_LEADER` 保存和职务清空 in `backend/internal/service/org_management_service.go`
- [X] T033 [US3] 实现 `PUT /api/v1/tenant/org-members/:id/positions` in `backend/internal/handler/org_management_handler.go`
- [X] T034 [US3] 更新用户属性同步逻辑以使用 `ORG_LEADER/DEPUTY_LEADER` in `backend/internal/service/org_attribute_service.go`
- [X] T035 [US3] 更新租户属性字典中 `org_role` 预置值 in `backend/internal/service/org_attribute_service.go`

**检查点**：US3 完成后不会再产生新的旧部门角色编码。

---

## 阶段 6：用户故事 4 - 迁移旧部门角色与废弃旧接口（P1）

**目标**：旧 `role_code` 数据被安全、幂等迁移，新旧组织写接口共用 Service，前端迁移后可废弃旧写接口。

**独立测试**：准备旧职务数据，执行迁移多次，验证 `DO/DU` 补齐、旧职务停用、`ORG_MANAGER` 映射和重复执行幂等。

### 测试

- [X] T036 [P] [US4] 编写迁移 SQL 验证脚本或测试说明 in `specs/007-tenant-org-management/quickstart.md`
- [ ] T037 [P] [US4] 编写旧接口桥接 Handler 测试 in `backend/internal/handler/org_attribute_handler_test.go`

### 实现

- [X] T038 [US4] 在迁移脚本中实现 `ORG_MANAGER -> ORG_LEADER` 幂等迁移 in `backend/migrations/008_tenant_org_management.sql`
- [X] T039 [US4] 在迁移脚本中停用或删除旧 `ORG_MEMBER` in `backend/migrations/008_tenant_org_management.sql`
- [X] T040 [US4] 在迁移脚本中将旧 `DATA_OWNER` 补齐为 `DO` 后停用 in `backend/migrations/008_tenant_org_management.sql`
- [X] T041 [US4] 在迁移脚本中将旧 `DATA_VISITOR` 补齐为 `DU` 后停用 in `backend/migrations/008_tenant_org_management.sql`
- [X] T042 [US4] 在迁移脚本中补齐 `tenant_org_members.is_primary` 数据一致性 in `backend/migrations/008_tenant_org_management.sql`
- [X] T043 [US4] 为旧 `/api/v1/tenants/:id/...` 组织写接口添加废弃注释和 Service 桥接 in `backend/internal/handler/org_attribute_handler.go`
- [X] T044 [US4] 在文档中记录旧接口废弃计划 in `specs/007-tenant-org-management/contracts/api.md`

**检查点**：US4 完成后，旧数据不再污染新部门职务模型。

---

## 阶段 7：用户故事 5 - 前端组织管理页面（P2）

**目标**：租户管理员可通过桌面端完成组织架构和成员管理。

**独立测试**：在桌面端以 `TENANT_ADMIN` 打开组织管理页面，完成部门和成员操作，确认接口调用拆分正确。

### 测试

- [X] T045 [P] [US5] 编写组织管理 API 封装测试或类型校验 in `desktop/src/renderer/src/api/tenantOrg.ts`
- [ ] T046 [P] [US5] 编写成员抽屉保存拆分逻辑测试 in `desktop/src/renderer/src/components/tenant-org/OrgMemberDrawer.test.tsx`

### 实现

- [X] T047 [US5] 扩展当前租户组织管理 API 封装 in `desktop/src/renderer/src/api/tenantOrg.ts`
- [X] T048 [US5] 新增组织管理页面路由 in `desktop/src/renderer/src/main.tsx`
- [X] T049 [US5] 在租户管理菜单新增“组织管理” in `desktop/src/renderer/src/components/AppLayout.tsx`
- [X] T050 [US5] 实现组织管理页签容器 in `desktop/src/renderer/src/pages/TenantOrgManagementPage.tsx`
- [X] T051 [US5] 实现部门树和部门详情组件 in `desktop/src/renderer/src/components/tenant-org/OrgUnitTreePanel.tsx`
- [X] T052 [US5] 实现部门创建、编辑、移动抽屉 in `desktop/src/renderer/src/components/tenant-org/OrgUnitDrawer.tsx`
- [X] T053 [US5] 实现成员列表搜索和筛选 in `desktop/src/renderer/src/components/tenant-org/OrgMemberTable.tsx`
- [X] T054 [US5] 实现成员编辑抽屉并拆分部门职务与系统角色保存调用 in `desktop/src/renderer/src/components/tenant-org/OrgMemberDrawer.tsx`
- [X] T055 [US5] 使用设计系统确认组件替代任何危险操作原生确认 in `desktop/src/renderer/src/pages/TenantOrgManagementPage.tsx`

**检查点**：US5 完成后，页面具备可演示的组织管理体验。

---

## 阶段 8：收尾与横切检查

**目标**：完成文档、测试、注释和安全边界复查。

- [X] T056 [P] 更新 README 或后端说明中的组织管理入口和迁移说明 in `README.md`
- [X] T057 [P] 补充 quickstart 验证结果记录 in `specs/007-tenant-org-management/quickstart.md`
- [ ] T058 运行后端测试并修复本功能相关失败 in `backend`
- [X] T059 运行前端类型检查和测试并修复本功能相关失败 in `desktop`
- [X] T060 Go 关键注释和可读性检查：确认新增或修改的 Go 业务代码中每个函数/方法都有前置注释，导出标识符符合 GoDoc 前缀规范，实体字段、Handler、Service、Repository、Middleware 注释解释业务语义、副作用、事务边界、租户隔离和安全边界，并移除无意义逐行注释 in `backend/internal`
- [X] T061 安全边界复查：确认组织、成员、职务、属性同步查询和写入全部限制当前租户，部门职务接口不写系统角色，最后租户管理员保护仍由现有系统角色服务负责 in `backend/internal`

## 依赖与执行顺序

### 阶段依赖

- **阶段 1**：无依赖，必须先完成。
- **阶段 2**：依赖阶段 1 的模型和迁移设计。
- **US1**：依赖阶段 2，可作为 MVP。
- **US2**：依赖阶段 2 和 US1 的部门基础。
- **US3**：依赖 US2 的成员关系。
- **US4**：依赖阶段 1，可与 US1-US3 部分并行，但最终必须在正式切换前完成。
- **US5**：依赖后端接口契约稳定。
- **阶段 8**：依赖目标用户故事完成。

### 建议 MVP

1. 完成阶段 1 和阶段 2。
2. 完成 US1 部门树维护。
3. 完成 US4 中旧角色迁移的统计和脚本。
4. 停下来验证部门树和迁移安全性。

### 可并行机会

- T003 与 T004 可并行。
- T011 与 T012 可并行。
- T020 与 T021 可并行。
- T028 与 T029 可并行。
- T038-T041 在迁移设计确认后可并行评审，但脚本落地需串联验证。
- 前端 T051-T054 可在页面容器完成后并行开发。

## 实施策略

### 后端先行

先实现迁移、模型、仓储和 Service，确保所有核心规则由后端强制执行，前端只做体验优化。

### 接口切换

新增 `/api/v1/tenant/...` 当前租户接口作为正式入口；旧 `/api/v1/tenants/:id/...` 只做过渡桥接，不再发展新能力。

### 前端增量接入

前端先实现组织树和成员列表，再实现抽屉编辑。即使同一抽屉展示部门职务和系统角色，也必须拆分 API 调用。

## 备注

- 不修改 `specs/006-tenant-org-attributes/`。
- 不实现完整 RBAC、自定义职务系统、审批流或 CP-ABE 密钥撤销。
- 每个实现任务提交前优先运行对应测试。
- 涉及 Go 业务代码时必须满足中文注释规范。


