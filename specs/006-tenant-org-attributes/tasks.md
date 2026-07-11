# 任务清单：租户级组织架构与 CP-ABE 属性体系

**输入**：`/specs/006-tenant-org-attributes/` 下的 `spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**组织方式**：按可独立交付的功能模块拆分。每个任务都包含目标、修改范围、后端改动、前端改动、数据库改动、验收标准和注意事项。

**本次明确不做**：真实 CP-ABE 加解密、完整文件加密上传、复杂审批流、企业微信或外部组织同步、复杂权限审计后台。

## 阶段 1：基础设施与数据库模型

**目标**：先把组织架构和属性体系的持久化基础打稳，后续接口和前端都依赖这一阶段。

- [X] T001 数据库迁移与组织属性表结构 in `backend/migrations/005_create_tenant_org_attribute_tables.sql`
  - **任务目标**：新增 `tenant_org_units`、`tenant_org_members`、`tenant_org_member_roles`、`tenant_attributes`、`tenant_attribute_values`、`user_attributes` 六张表。
  - **修改范围**：仅新增迁移文件，不修改既有 `users`、`tenants`、`tenant_users`、`roles`、`user_roles`、`policy_attributes` 表。
  - **后端改动**：无业务代码改动。
  - **前端改动**：无。
  - **数据库改动**：按 `data-model.md` 建表，包含唯一键、租户边界索引、软删除字段和状态字段。
  - **验收标准**：迁移可在空库和已有演示库执行；六张表创建成功；重复执行不破坏已有表；索引名称和字段类型与计划一致。
  - **注意事项**：不要删除或改名 `policy_attributes`，本功能新增租户级属性表以保持兼容。

- [X] T002 [P] 后端领域模型定义 in `backend/internal/domain/org.go` and `backend/internal/domain/tenant_attribute.go`
  - **任务目标**：定义 `OrgUnit`、`OrgMember`、`OrgMemberRole`、`TenantAttribute`、`TenantAttributeValue`、`UserAttribute` 及状态/类型枚举。
  - **修改范围**：新增 domain 文件，必要时在 `backend/internal/domain/constants.go` 补充部门角色常量。
  - **后端改动**：补齐 Gorm 标签、JSON 字段、`TableName()`、枚举 `Valid()` 方法和响应 DTO。
  - **前端改动**：无。
  - **数据库改动**：无迁移外改动。
  - **验收标准**：模型字段与迁移表一致；导出类型和方法 GoDoc 以标识符开头；关键字段注释说明租户边界、权限参与、是否可空和是否用于策略匹配。
  - **注意事项**：字段注释不能只复述字段名，要说明为什么该字段对访问控制或可解释性重要。

- [X] T003 [P] 组织属性仓储接口与 Gorm 实现 in `backend/internal/repository/org_attribute_repository.go`
  - **任务目标**：提供组织树、成员、部门角色、属性字典、属性值、用户属性的持久化能力。
  - **修改范围**：新增 repository 文件，必要时复用现有 `isDuplicateKey` 错误转换。
  - **后端改动**：实现按租户查询组织树、查找组织单元、幂等添加成员、设置角色、查询租户属性、写入用户属性事务等仓储方法。
  - **前端改动**：无。
  - **数据库改动**：无。
  - **验收标准**：所有查询默认带 `tenant_id` 范围；唯一键冲突转换为明确仓储错误；未找到记录返回语义错误；仓储单元测试覆盖跨租户查询、重复成员、重复角色。
  - **注意事项**：Repository 注释必须说明查询范围、唯一性假设、未找到记录语义和数据库副作用。

- [X] T004 基础服务骨架与路由依赖装配 in `backend/internal/service/org_attribute_service.go`, `backend/internal/handler/org_attribute_handler.go`, `backend/internal/handler/router.go`, `backend/cmd`
  - **任务目标**：接入组织属性服务、Handler 和路由依赖，为后续模块任务提供统一入口。
  - **修改范围**：新增 service/handler 文件，修改路由依赖结构和应用启动装配代码。
  - **后端改动**：定义 `OrgAttributeService`、`OrgAttributeHandler`、Actor 上下文解析、统一错误边界；注册基础路由组但可先返回未实现或空结构。
  - **前端改动**：无。
  - **数据库改动**：无。
  - **验收标准**：项目可编译；现有认证、租户、访问策略接口不受影响；新增路由均受登录态和租户上下文中间件保护。
  - **注意事项**：平台管理员不应通过这些租户路由维护具体部门。

---

## 阶段 2：演示种子数据与租户组织树

**目标**：让三个演示租户先拥有真实组织树和基础属性值，为访问策略构建器提供真实数据来源。

- [X] T005 [US5] 租户组织架构种子数据 in `backend/scripts` and `backend/internal/service/org_attribute_service.go`
  - **任务目标**：为深信服科技、四川师范大学、香港友邦保险幂等预置规格要求的组织树。
  - **修改范围**：新增或扩展后端 seed 脚本/启动初始化逻辑，写入 `tenant_org_units`。
  - **后端改动**：通过租户 `code` 或中文名称定位租户；按稳定 `code/path/level/sort_order` 创建组织单元；重复执行只补缺不覆盖人工调整。
  - **前端改动**：无。
  - **数据库改动**：写入三家租户组织树数据。
  - **验收标准**：深信服科技包含安全 BG、云 BG、AI BG 等完整树；四川师范大学包含计算机科学学院及下级；香港友邦保险包含数字化与科技部及下级；重复运行不产生重复节点。
  - **注意事项**：`path` 使用稳定 code 路径，中文名称只用于展示，不作为策略匹配唯一依据。

- [X] T006 [US5] 租户属性字典与属性值种子数据 in `backend/scripts` and `backend/internal/service/org_attribute_service.go`
  - **任务目标**：为每个演示租户预置 `department`、`org_role`、`tenant_role`、`security_level`、`data_category` 属性及可选值。
  - **修改范围**：扩展种子逻辑，写入 `tenant_attributes` 和 `tenant_attribute_values`。
  - **后端改动**：从组织树同步 `department` 树形属性值；预置通用部门角色、租户角色、数据分类和安全等级属性元数据。
  - **前端改动**：无。
  - **数据库改动**：写入租户属性定义和属性值。
  - **验收标准**：每个租户至少有 5 类属性；`department` 值关联对应 `org_unit_id`；`security_level` 为 number 类型且有比较操作符元数据；重复运行不产生重复属性值。
  - **注意事项**：`tenant_role` 需要兼容现有 `TENANT_ADMIN`、`DO`/`DU` 或 `DATA_OWNER`/`DATA_VISITOR` 映射，具体映射在服务层集中处理。

- [X] T007 [P] [US1] 当前租户组织树查询接口 in `backend/internal/service/org_attribute_service.go`, `backend/internal/handler/org_attribute_handler.go`, `backend/internal/handler/router.go`
  - **任务目标**：交付 `GET /api/v1/tenants/:id/org-units/tree`，让用户能按租户查看真实组织树。
  - **修改范围**：Service、Handler、Router、Repository 测试。
  - **后端改动**：实现组织树查询、`status=enabled/all`、树形组装、租户上下文校验。
  - **前端改动**：无。
  - **数据库改动**：无。
  - **验收标准**：三个租户返回各自组织树；跨租户路径 ID 与上下文不一致时拒绝；禁用节点默认不返回。
  - **注意事项**：该接口允许 `TENANT_ADMIN`、`DATA_OWNER`、`DATA_VISITOR` 读取，但不允许未加入租户用户读取。

---

## 阶段 3：租户组织成员与部门角色绑定

**目标**：租户管理员可以把用户加入部门，并在部门作用域内绑定通用部门角色，避免角色爆炸。

- [X] T008 [US2] 部门成员管理服务与接口 in `backend/internal/service/org_attribute_service.go`, `backend/internal/handler/org_attribute_handler.go`, `backend/internal/handler/router.go`
  - **任务目标**：交付查询部门成员、添加用户到部门、从部门移除用户的最小闭环。
  - **修改范围**：Service、Handler、Router、Repository、错误定义。
  - **后端改动**：实现 `GET/POST/DELETE /tenants/:id/org-units/:orgUnitId/members` 相关能力；校验用户是当前租户有效成员；移除成员后标记关系失效。
  - **前端改动**：本任务不要求 UI，可用接口验证。
  - **数据库改动**：写入或更新 `tenant_org_members`。
  - **验收标准**：租户管理员可添加本租户用户到本租户部门；重复添加幂等；目标用户或部门跨租户时拒绝；移除后成员不再出现在 active 列表。
  - **注意事项**：移除成员会影响用户属性，需在响应中提示需要同步或触发后续同步任务。

- [X] T009 [US2] 部门角色绑定服务与接口 in `backend/internal/service/org_attribute_service.go`, `backend/internal/handler/org_attribute_handler.go`, `backend/internal/handler/router.go`
  - **任务目标**：交付 `PUT /tenants/:id/org-units/:orgUnitId/members/:userId/roles`，支持通用角色 + 部门作用域。
  - **修改范围**：Service、Handler、Router、Repository、测试。
  - **后端改动**：校验 `roleCodes` 只允许 `ORG_MANAGER`、`ORG_MEMBER`、`DATA_OWNER`、`DATA_VISITOR`；事务内替换或合并部门角色；返回当前有效角色。
  - **前端改动**：无。
  - **数据库改动**：写入或更新 `tenant_org_member_roles`。
  - **验收标准**：同一 `ORG_MANAGER` 可绑定不同部门；`AI_BG_MANAGER`、`CLOUD_BG_MANAGER` 等专属角色被拒绝；重复保存幂等；跨租户操作拒绝。
  - **注意事项**：不要把部门角色写进 `roles` 表；`roles/user_roles` 仍用于租户级角色。

- [ ] T010 [P] [US2] 部门成员与角色后端测试 in `backend/internal/service/org_attribute_service_test.go` and `backend/internal/handler/org_attribute_handler_test.go`
  - **任务目标**：覆盖部门成员和部门角色的核心合法/非法场景。
  - **修改范围**：新增或扩展后端测试。
  - **后端改动**：构造测试租户、用户、组织单元、成员和角色绑定样例。
  - **前端改动**：无。
  - **数据库改动**：仅测试数据库数据。
  - **验收标准**：测试覆盖添加成员、重复添加、跨租户拒绝、未知角色拒绝、部门专属角色拒绝、重复角色幂等。
  - **注意事项**：测试辅助函数也必须有中文前置注释。

---

## 阶段 4：CP-ABE 用户属性投影

**目标**：把组织成员、部门角色、租户角色和安全等级同步为可解释的 `user_attributes`，供 DU 后续参与策略匹配。

- [X] T011 [US3] 用户属性投影服务 in `backend/internal/service/org_attribute_service.go`
  - **任务目标**：实现单用户 CP-ABE 属性同步逻辑。
  - **修改范围**：Service、Repository、领域 DTO。
  - **后端改动**：读取 `tenant_org_members`、`tenant_org_member_roles`、`tenant_attributes`、`tenant_attribute_values`、`user_roles`；生成 `department`、`org_role`、`tenant_role`、`security_level` 属性；事务内失效旧属性并写入新属性。
  - **前端改动**：无。
  - **数据库改动**：写入 `user_attributes`。
  - **验收标准**：同步后用户拥有正确部门、部门角色、租户角色、安全等级属性；移除部门后再次同步，旧部门属性失效；同步失败时不留下部分 active 结果。
  - **注意事项**：本任务只生成属性输入，不做真实 CP-ABE 私钥发放或解密。

- [X] T012 [US3] 用户属性同步与查看接口 in `backend/internal/handler/org_attribute_handler.go` and `backend/internal/handler/router.go`
  - **任务目标**：交付 `POST /tenants/:id/users/:userId/attributes/sync` 和 `GET /tenants/:id/users/me/attributes`。
  - **修改范围**：Handler、Router、响应 DTO、错误处理。
  - **后端改动**：租户管理员可同步本租户用户；DO/DU/TENANT_ADMIN 可查看自己的有效属性；禁止读取他人属性。
  - **前端改动**：可先不做 UI。
  - **数据库改动**：同步接口写入 `user_attributes`，查看接口只读。
  - **验收标准**：租户管理员同步成功返回属性列表；DU 查看自己的属性成功；DU 查看他人属性无入口且接口拒绝；跨租户同步拒绝。
  - **注意事项**：响应必须保留 `sourceType`、`valueCode`、`valueLabel`、`valuePath` 或 `numberValue`，便于解释属性来源。

- [ ] T013 [P] [US3] 用户属性投影后端测试 in `backend/internal/service/org_attribute_service_test.go` and `backend/internal/handler/org_attribute_handler_test.go`
  - **任务目标**：验证用户属性投影的准确性、幂等性和租户隔离。
  - **修改范围**：后端测试。
  - **后端改动**：构造组织成员、部门角色、租户角色和安全等级演示数据。
  - **前端改动**：无。
  - **数据库改动**：仅测试数据。
  - **验收标准**：同步重复执行不产生重复 active 属性；跨租户属性不会被投影；移除成员后旧属性 inactive；同步失败不产生部分 active 属性。
  - **注意事项**：测试要明确本阶段不校验真实 CP-ABE 解密。

---

## 阶段 5：租户属性字典查询接口

**目标**：让访问策略构建器从后端加载当前租户真实属性字典、部门树、枚举值和数字操作符。

- [X] T014 [US4] 租户属性字典查询服务 in `backend/internal/service/org_attribute_service.go` and `backend/internal/pkg/policytree/validator.go`
  - **任务目标**：实现构建器所需的租户属性元数据聚合。
  - **修改范围**：Service、Repository、PolicyTree 元数据校验。
  - **后端改动**：查询 `tenant_attributes` 和 `tenant_attribute_values`；为 `department` 组装 tree；为 enum 组装 values；为 number 返回 `>=`、`<=`、`=`；扩展策略树校验支持 `tree/enum/number` 和 `valueId/valueCode/path`。
  - **前端改动**：无。
  - **数据库改动**：无。
  - **验收标准**：每个演示租户返回不同部门树；禁用属性和值不出现在默认结果；后端保存策略时拒绝跨租户属性值和不适配操作符。
  - **注意事项**：不能继续以平台级 `policy_attributes` 作为构建器权威来源。

- [X] T015 [US4] 构建器属性接口兼容改造 in `backend/internal/handler/policy_handler.go`, `backend/internal/service/policy_service.go`, `backend/internal/handler/router.go`
  - **任务目标**：让现有 `GET /tenants/:id/access-policy/attributes` 返回租户级真实属性字典。
  - **修改范围**：现有 PolicyHandler/PolicyService 与新 OrgAttributeService 的协作边界。
  - **后端改动**：将 DATA_OWNER 构建器属性查询从平台属性切换为租户属性；保存访问策略时使用租户级属性元数据校验；保留平台属性管理接口不变。
  - **前端改动**：无。
  - **数据库改动**：无。
  - **验收标准**：现有访问策略列表、模板、创建、编辑接口继续可用；构建器属性接口返回 `department/org_role/tenant_role/security_level/data_category`；空租户属性时返回空态而不是假数据。
  - **注意事项**：旧策略 JSON 必须继续可回显；无法补齐稳定值时保存应给出明确错误。

- [ ] T016 [P] [US4] 租户属性字典接口测试 in `backend/internal/handler/policy_handler_test.go` and `backend/internal/service/policy_service_test.go`
  - **任务目标**：验证构建器属性接口和策略保存校验使用真实租户属性。
  - **修改范围**：后端测试。
  - **后端改动**：新增租户级属性测试数据，覆盖 tree/enum/number 三类属性。
  - **前端改动**：无。
  - **数据库改动**：仅测试数据。
  - **验收标准**：深信服与四川师范大学返回不同 department；`security_level >= 3` 可通过校验；跨租户 `valueId` 保存被拒绝；平台属性不再污染租户构建器结果。
  - **注意事项**：测试名称和注释需要说明这是策略输入校验，不是真实 CP-ABE 加密。

---

## 阶段 6：访问树构建器接入真实属性字典

**目标**：前端访问策略构建器不再依赖静态假数据，优先展示当前租户后端真实属性字典。

- [X] T017 [US4] 前端组织属性 API 封装 in `desktop/src/renderer/src/api/tenantOrg.ts` and `desktop/src/renderer/src/api/policy.ts`
  - **任务目标**：封装组织树、属性字典、成员和用户属性相关请求。
  - **修改范围**：新增 `tenantOrg.ts`，调整 `policy.ts` 的属性查询类型。
  - **后端改动**：无。
  - **前端改动**：新增 `listOrgTree`、`listTenantPolicyAttributes`、`listOrgMembers`、`syncUserAttributes`、`listMyUserAttributes` 等函数。
  - **数据库改动**：无。
  - **验收标准**：TypeScript 类型通过；请求路径与 `contracts/api.md` 一致；调用方能拿到 tree/enum/number 元数据。
  - **注意事项**：保留现有 `listAvailableAttributes` 调用兼容，内部可转向新响应结构。

- [X] T018 [US4] 访问树类型模型扩展 in `desktop/src/renderer/src/components/access-policy/tree/types.ts`
  - **任务目标**：扩展前端属性和叶子节点结构，支持真实值标识和数字比较。
  - **修改范围**：访问树 TypeScript 类型、辅助函数。
  - **后端改动**：无。
  - **前端改动**：扩展 `PolicyAttributeType` 为 `tree | enum | number | string`；扩展 `PolicyOperator`；扩展 `PolicyTreeNode` 叶子节点字段 `valueId/valueCode/label/path`；保留旧结构兼容 helper。
  - **数据库改动**：无。
  - **验收标准**：已有访问树测试仍可编译；旧 `{attribute, operator, value}` 节点可转换和回显；新节点能表达 department 和 security_level。
  - **注意事项**：不要一次重写所有访问树逻辑，先保持兼容再逐步替换。

- [X] T019 [US4] 构建器属性加载去 mock 化 in `desktop/src/renderer/src/pages/AccessPolicyBuilderPage.tsx` and `desktop/src/renderer/src/pages/AccessPolicyEditorPage.tsx`
  - **任务目标**：构建器入口从后端加载真实属性字典，移除正常路径中的静态假数据回退。
  - **修改范围**：访问策略构建器页面和独立编辑器页面。
  - **后端改动**：无。
  - **前端改动**：加载当前租户属性；空结果显示空态；失败显示错误提示；不再静默使用 `mockAttributes` 作为真实数据。
  - **数据库改动**：无。
  - **验收标准**：深信服租户显示 AI BG 等部门；四川师范大学显示学院部门；接口失败时用户看到错误而不是假数据。
  - **注意事项**：`mockTree` 可以仅保留为开发测试夹具，不得作为生产加载 fallback。

- [X] T020 [P] [US4] 前端属性加载测试 in `desktop/src/renderer/src/components/access-policy/tree/policyModel.test.ts` and `desktop/src/renderer/src/components/access-policy/tree/roundtrip.test.ts`
  - **任务目标**：验证真实属性元数据能驱动访问树模型。
  - **修改范围**：前端 Vitest 测试。
  - **后端改动**：无。
  - **前端改动**：补充 tree/enum/number 属性样例和旧策略兼容断言。
  - **数据库改动**：无。
  - **验收标准**：`npm run test` 通过；旧节点 roundtrip 不丢失；新节点 roundtrip 保留 `valueId/valueCode/label/path`。
  - **注意事项**：测试不要依赖真实后端，使用接口契约形状的 fixture。

---

## 阶段 7：部门树选择器与策略节点保存结构改造

**目标**：DATA_OWNER 能在节点配置中选择真实部门、角色和安全等级，并保存稳定编码/标识/路径。

- [X] T021 [US4] 属性节点配置控件改造 in `desktop/src/renderer/src/components/access-policy/NodeConfigPopover.tsx` and `desktop/src/renderer/src/components/access-policy/AccessTreeConfigPanel.tsx`
  - **任务目标**：根据属性类型渲染树形、枚举和数字输入控件。
  - **修改范围**：节点配置弹窗和侧边配置面板。
  - **后端改动**：无。
  - **前端改动**：`department` 使用树形选择器；`org_role/tenant_role/data_category` 使用枚举列表；`security_level` 使用数字输入和比较操作符。
  - **数据库改动**：无。
  - **验收标准**：用户能选择 AI BG、计算机科学学院、理赔服务部等真实部门；数字属性可选择 `>=`、`<=`、`=`；不适配操作符不会展示。
  - **注意事项**：不要把说明文字堆在界面上，控件状态和错误提示要简洁。

- [X] T022 [US4] 策略节点转换、表达式和校验改造 in `desktop/src/renderer/src/components/access-policy/tree/convert.ts`, `desktop/src/renderer/src/components/access-policy/tree/expression.ts`, `desktop/src/renderer/src/components/access-policy/tree/validate.ts`
  - **任务目标**：让访问树保存和预览支持 `code/valueId/label/path` 与数字比较。
  - **修改范围**：访问树转换、表达式生成、前端校验逻辑。
  - **后端改动**：无。
  - **前端改动**：flow 节点与树 JSON 互转时保留稳定值字段；表达式展示“department 属于 AI_BG”“security_level >= 3”；校验属性值是否属于当前租户字典。
  - **数据库改动**：无。
  - **验收标准**：保存 JSON 包含稳定字段；表达式与示例策略一致；跨租户旧值在前端校验中提示不可用；旧策略仍可预览。
  - **注意事项**：最终安全校验以后端为准，前端校验只做体验优化。

- [X] T023 [P] [US4] 访问树节点结构前端测试 in `desktop/src/renderer/src/components/access-policy/tree/convert.test.ts`, `desktop/src/renderer/src/components/access-policy/tree/expression.test.ts`, `desktop/src/renderer/src/components/access-policy/tree/validate.test.ts`
  - **任务目标**：覆盖新节点结构的转换、表达式和校验。
  - **修改范围**：前端访问树测试。
  - **后端改动**：无。
  - **前端改动**：新增 department tree、org_role enum、security_level number 测试。
  - **数据库改动**：无。
  - **验收标准**：`department 属于 AI_BG`、`org_role = ORG_MANAGER`、`security_level >= 3` 的测试全部通过；旧节点测试仍通过。
  - **注意事项**：如当前没有 `validate.test.ts`，可新增该文件。

---

## 阶段 8：租户切换刷新与前端集成体验

**目标**：不同租户切换后，组织树、属性字典和策略节点选择状态自动刷新，不残留上一租户数据。

- [X] T024 [US4] 租户切换后的属性刷新 in `desktop/src/renderer/src/pages/AccessPolicyBuilderPage.tsx`, `desktop/src/renderer/src/pages/AccessPolicyEditorPage.tsx`, `desktop/src/renderer/src/auth/AuthContext.tsx`
  - **任务目标**：监听当前租户变化并刷新属性字典、组织树和策略列表。
  - **修改范围**：访问策略构建器、独立编辑器、必要的认证上下文辅助状态。
  - **后端改动**：无。
  - **前端改动**：租户 ID 变化时清空旧属性缓存、取消或忽略旧请求结果、重新加载当前租户属性；当前草稿引用旧租户属性时提示重新选择。
  - **数据库改动**：无。
  - **验收标准**：从深信服切到四川师范大学后不再显示 AI BG；从四川师范大学切到香港友邦保险后显示理赔服务部/合规法务部；旧请求晚返回不会覆盖新租户状态。
  - **注意事项**：避免全局缓存按属性编码覆盖，不同租户的 `department` 编码相同但值不同。

- [X] T025 [P] [US4] 属性字典面板空态与错误态 in `desktop/src/renderer/src/components/access-policy/AttributeDictionaryPanel.tsx`
  - **任务目标**：让构建器在租户未初始化或接口失败时给出明确反馈。
  - **修改范围**：属性字典面板。
  - **后端改动**：无。
  - **前端改动**：展示加载态、空态、错误态；保留属性类型标签和稳定编码展示。
  - **数据库改动**：无。
  - **验收标准**：空属性字典时显示“当前租户未初始化属性字典”；接口失败时显示错误提示；正常状态展示后端返回的属性。
  - **注意事项**：不要在错误态偷偷注入 mock 属性。

---

## 阶段 9：验收用例与示例策略

**目标**：把三家租户的示例策略跑通，并形成可重复验证的后端与前端用例。

- [X] T026 [US5] 三家租户示例策略验收数据 in `backend/scripts` and `backend/internal/service/org_attribute_service.go`
  - **任务目标**：为示例策略准备必要用户、部门成员、部门角色、安全等级和用户属性。
  - **修改范围**：种子数据、同步逻辑调用、演示说明。
  - **后端改动**：每个租户至少准备 TENANT_ADMIN、部门 ORG_MANAGER、DATA_OWNER、DATA_VISITOR 和一个满足安全等级条件的用户；执行一次属性同步。
  - **前端改动**：无。
  - **数据库改动**：写入演示用户成员、部门角色和 `user_attributes`。
  - **验收标准**：深信服可构建 `(department 属于 AI_BG AND org_role = ORG_MANAGER) OR tenant_role = TENANT_ADMIN`；四川师范大学可构建计算机科学学院示例；香港友邦保险可构建理赔服务部安全等级示例。
  - **注意事项**：若复用已有用户，不要覆盖用户密码或破坏现有登录演示账号。

- [ ] T027 [P] [US5] 快速验证文档补齐 in `specs/006-tenant-org-attributes/quickstart.md` and `README.md`
  - **任务目标**：把实现后的迁移、种子数据、构建器验证和权限验证步骤补齐到文档。
  - **修改范围**：本功能 quickstart，必要时更新项目 README 的功能索引。
  - **后端改动**：无。
  - **前端改动**：无。
  - **数据库改动**：无。
  - **验收标准**：按文档步骤可以验证组织树隔离、真实属性字典、三条示例策略、部门角色和用户属性同步。
  - **注意事项**：文档必须使用简体中文，命令和路径可保留英文。

- [ ] T028 [US5] 端到端验收回归 in `backend` and `desktop`
  - **任务目标**：执行后端、前端和手工验收，确认本功能闭环可演示。
  - **修改范围**：不限定文件，修复验收发现的小问题。
  - **后端改动**：运行 `go test ./...` 并修复失败。
  - **前端改动**：运行 `npm run typecheck`、`npm run test`，并修复失败。
  - **数据库改动**：在演示库执行迁移和种子数据。
  - **验收标准**：quickstart 六个验证场景通过；所有越权访问被拒绝；构建器不再显示静态假数据。
  - **注意事项**：本任务不允许顺手加入真实 CP-ABE 加解密或文件上传流程。

---

## 阶段 10：收尾与横切检查

**目标**：补齐项目宪章要求的注释、可读性、安全边界和兼容性检查。

- [X] T029 关键注释和可读性检查 in `backend/internal/domain`, `backend/internal/repository`, `backend/internal/service`, `backend/internal/handler`, `backend/internal/pkg/policytree`
  - **任务目标**：确认所有新增或修改 Go 业务代码满足项目中文注释规范。
  - **修改范围**：所有本功能涉及的 Go 文件。
  - **后端改动**：补齐函数/方法前置注释、导出标识符 GoDoc、实体字段业务含义、Handler/Service/Repository 权限边界和副作用说明，删除无意义逐行注释。
  - **前端改动**：无。
  - **数据库改动**：无。
  - **验收标准**：每个函数和方法都有中文前置注释；导出标识符注释以标识符开头；安全、权限、租户隔离、属性同步事务均有边界说明。
  - **注意事项**：注释解释“为什么这么设计”，不要写“获取用户”“调用函数”这类噪音。

- [X] T030 [P] 安全边界与兼容性复查 in `backend/internal/handler`, `backend/internal/service`, `desktop/src/renderer/src/components/access-policy`
  - **任务目标**：检查租户隔离、角色边界、旧访问树兼容和平台属性兼容。
  - **修改范围**：后端 Handler/Service，前端访问树组件。
  - **后端改动**：确认所有组织、成员、角色、属性、用户属性接口都绑定当前租户上下文；确认平台管理员不维护具体部门。
  - **前端改动**：确认租户切换清空旧属性；确认旧策略能回显；确认 mock 不作为正常数据来源。
  - **数据库改动**：无。
  - **验收标准**：跨租户读写拒绝率 100%；旧访问策略可查看；新策略保存稳定字段；平台 `policy_attributes` 管理接口仍可用。
  - **注意事项**：如发现与宪章或规格冲突，先修正文档或实现，不要扩大本次范围。

## 依赖与执行顺序

### 阶段依赖

- **阶段 1**：无依赖，必须最先完成。
- **阶段 2**：依赖阶段 1 的迁移和模型。
- **阶段 3**：依赖阶段 1，建议在阶段 2 种子数据完成后验证。
- **阶段 4**：依赖阶段 2 和阶段 3。
- **阶段 5**：依赖阶段 1 和阶段 2，可与阶段 3/4 部分并行，但策略保存校验需要属性字典完成。
- **阶段 6**：依赖阶段 5 的接口契约稳定。
- **阶段 7**：依赖阶段 6 的前端类型模型。
- **阶段 8**：依赖阶段 6 和阶段 7。
- **阶段 9**：依赖阶段 2、4、5、7、8。
- **阶段 10**：依赖计划内所有目标任务基本完成。

### 建议 MVP

最小可演示范围建议完成：

1. T001-T007：数据库、模型、仓储、种子数据、组织树查询。
2. T014-T019：租户属性字典接口和构建器真实数据加载。
3. T021-T024：节点配置、保存结构和租户切换刷新。

完成后即可演示“访问策略构建器使用真实租户组织和属性字典”，再继续做部门成员、角色绑定和用户属性同步。

### 可并行机会

- T002 与 T003 可在迁移设计稳定后并行。
- T005 与 T006 可在 T001/T002 后并行设计，但写入顺序应先组织树后 department 属性值。
- T010、T013、T016、T020、T023 可与对应实现任务后半段并行补测试。
- 前端 T017-T020 可在后端接口契约稳定后并行推进。
- T027 文档可与 T026 验收数据并行。

## 并行执行示例

```text
并行组 A：
- T002 后端领域模型定义
- T003 组织属性仓储接口与 Gorm 实现

并行组 B：
- T010 部门成员与角色后端测试
- T013 用户属性投影后端测试
- T016 租户属性字典接口测试

并行组 C：
- T020 前端属性加载测试
- T023 访问树节点结构前端测试
- T025 属性字典面板空态与错误态
```

## 实施策略

### 后端先行

先完成数据库迁移、领域模型、仓储和种子数据，确保真实租户组织和属性字典存在。然后优先交付构建器属性接口，让前端可以尽早接入真实数据。

### 前端增量接入

前端先接入真实属性字典并移除静态假数据回退，再做树形选择器、枚举选择和数字比较控件。旧访问树 JSON 必须保持可回显。

### 验收闭环

每完成一个模块就按 quickstart 对应场景验证，不等所有功能完成后才发现租户隔离或数据结构问题。

## 备注

- 每个任务提交前运行相关测试；涉及后端公共能力时优先运行 `go test ./...`。
- 本阶段不实现真实 CP-ABE 加解密，不生成私钥，不做文件上传。
- 新增 Go 业务代码必须满足中文注释规范。
- 访问树和用户属性是后续加密能力的输入，不得声称已经具备生产级安全能力。

## 补充任务：平台管理员创建租户管理员账号

- [X] T031 平台管理员创建或复用租户管理员账号 in `backend/internal/service/platform_service.go`, `backend/internal/handler/platform_handler.go`, `backend/migrations/007_add_tenant_admin_account_fields.sql`, `desktop/src/renderer/src/pages/PlatformTenantUsersPage.tsx`
  - **任务目标**：允许 `PLATFORM_ADMIN` 在平台后台为指定租户创建租户管理员账号，或复用已存在用户并绑定当前租户的 `TENANT_ADMIN`。
  - **修改范围**：扩展现有 `POST /api/v1/platform/tenants/:id/admins`，保持 `{user_id}` 授权旧请求兼容，同时支持用户名、姓名、邮箱、手机号和可选密码的新建账号请求体。
  - **后端改动**：新增 `username`、`phone`、`must_change_password` 用户字段；服务层按邮箱复用用户，新建用户时使用 bcrypt 保存密码哈希，旧 `users.role` 使用低权限兼容角色，租户权限仅写入 `tenant_users` 和 `user_roles`。
  - **前端改动**：平台租户用户页面新增创建租户管理员表单；登录后当 `must_change_password = true` 时展示首次改密提示。
  - **数据库改动**：新增 `007_add_tenant_admin_account_fields.sql`，为 `users` 补充账号名、手机号和首次改密标记，并建立必要索引。
  - **验收标准**：平台管理员可为深信服科技、四川师范大学、香港友邦保险分别创建租户管理员；空密码使用默认密码 `lqf999..`；新账号可登录并进入对应租户；用户拥有该租户 `TENANT_ADMIN`；非平台管理员被拒绝；数据库密码不是明文；登录响应可识别 `must_change_password = true`。
  - **注意事项**：本任务不创建 `PLATFORM_ADMIN`，也不把租户管理员写成旧 `users.role=admin`，避免租户管理员意外获得平台或旧全局管理权限。
