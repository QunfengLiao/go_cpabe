# 任务：访问策略管理与 DATA_OWNER 可视化访问树构建

**输入**：来自 `specs/005-access-policy-tree/` 的 `spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**前置条件**：已完成 `$speckit-specify` 与 `$speckit-plan`；当前分支为 `feat/access-policy-tree`

**测试要求**：规格要求访问树校验自动化测试，计划要求后端权限/handler 测试、前端类型检查、构建和访问树纯函数验证，因此本任务清单包含测试任务。

**组织方式**：任务按用户故事分组，确保每个用户故事可独立实现、独立验证和分阶段演示。

## 阶段 1：准备工作（共享基础）

**目标**：准备依赖、目录、测试入口和迁移文件骨架。

- [X] T001 在 `desktop/package.json` 添加 React Flow 依赖、前端纯函数测试依赖和对应脚本
- [X] T002 在 `desktop/package-lock.json` 同步前端依赖锁定结果
- [X] T003 [P] 创建访问策略前端组件目录 `desktop/src/renderer/src/components/access-policy/`
- [X] T004 [P] 创建访问树节点组件目录 `desktop/src/renderer/src/components/access-policy/nodes/`
- [X] T005 [P] 创建访问树前端纯函数目录 `desktop/src/renderer/src/components/access-policy/tree/`
- [X] T006 [P] 创建访问策略页面占位文件 `desktop/src/renderer/src/pages/AccessPolicyBuilderPage.tsx`
- [X] T007 [P] 创建访问策略 API 客户端占位文件 `desktop/src/renderer/src/api/policy.ts`
- [X] T008 [P] 创建后端访问策略迁移文件 `backend/migrations/004_create_policy_tables.sql`
- [X] T009 [P] 创建后端 Policy domain 文件 `backend/internal/domain/policy.go`
- [X] T010 [P] 创建后端访问树包目录和基础文件 `backend/internal/pkg/policytree/tree.go`

---

## 阶段 2：基础能力（阻塞所有用户故事）

**目标**：完成所有故事共用的数据结构、访问树校验、表达式生成、数据库模型和服务骨架。

**注意**：本阶段完成前，不应开始具体页面或接口集成。

- [X] T011 编写 `policy_attributes`、`policy_templates`、`access_policies` 建表、索引和软删除迁移于 `backend/migrations/004_create_policy_tables.sql`
- [X] T012 定义 PolicyStatus、PolicyAttributeType、PolicyOperator、PolicyAttribute、PolicyTemplate、AccessPolicy 和字段中文注释于 `backend/internal/domain/policy.go`
- [X] T013 [P] 定义访问树 Go 结构体、节点类型和值对象于 `backend/internal/pkg/policytree/tree.go`
- [X] T014 [P] 编写访问树校验失败用例测试于 `backend/internal/pkg/policytree/validator_test.go`
- [X] T015 [P] 编写访问树表达式生成测试于 `backend/internal/pkg/policytree/expression_test.go`
- [X] T016 实现访问树结构校验、单根/循环/孤立节点防护和属性引用校验于 `backend/internal/pkg/policytree/validator.go`
- [X] T017 实现访问树标准表达式生成和括号规则于 `backend/internal/pkg/policytree/expression.go`
- [X] T018 [P] 定义前端访问树、属性、模板、策略、校验错误类型于 `desktop/src/renderer/src/components/access-policy/tree/types.ts`
- [X] T019 [P] 编写前端 `flowToTree`、`treeToFlow` 转换测试于 `desktop/src/renderer/src/components/access-policy/tree/convert.test.ts`
- [X] T020 [P] 编写前端表达式生成和基础校验测试于 `desktop/src/renderer/src/components/access-policy/tree/expression.test.ts`
- [X] T021 实现前端 `flowToTree`、`treeToFlow` 双向转换于 `desktop/src/renderer/src/components/access-policy/tree/convert.ts`
- [X] T022 实现前端 `generatePolicyExpr` 表达式生成于 `desktop/src/renderer/src/components/access-policy/tree/expression.ts`
- [X] T023 实现前端 `validateTree` 基础校验和错误定位于 `desktop/src/renderer/src/components/access-policy/tree/validate.ts`
- [X] T024 定义访问策略 repository 接口、Gorm 实现和字段映射于 `backend/internal/repository/policy_repository.go`
- [X] T025 定义访问策略 service 输入输出 DTO、权限上下文和服务骨架于 `backend/internal/service/policy_service.go`
- [X] T026 在 `backend/internal/pkg/response/errors.go` 增加访问策略属性、模板、策略、访问树和权限相关错误码
- [X] T027 在 `backend/internal/handler/router.go` 预留 PolicyService 依赖和访问策略路由装配位置

**检查点**：访问树校验、表达式生成和核心数据结构已可单独测试。

---

## 阶段 3：用户故事 1 - DATA_OWNER 可视化构建访问策略（优先级：P1，MVP）

**目标**：DATA_OWNER 可以使用可视化编辑器基于属性和模板构建访问树，实时生成 JSON 和表达式，并完成保存与回显。

**独立测试**：使用 mock 或真实启用属性和模板，DATA_OWNER 能进入构建页面，创建包含 OR、AND、LEAF 的访问树，看到表达式和 JSON，触发校验，保存后重新打开完整回显。

### 测试任务

- [X] T028 [P] [US1] 编写 DATA_OWNER 创建访问策略 service 测试于 `backend/internal/service/policy_service_test.go`
- [X] T029 [P] [US1] 编写 DATA_OWNER 访问策略创建和详情 handler 测试于 `backend/internal/handler/policy_handler_test.go`
- [X] T030 [P] [US1] 编写访问树编辑器纯函数回显验证测试于 `desktop/src/renderer/src/components/access-policy/tree/roundtrip.test.ts`

### 实现任务

- [X] T031 [P] [US1] 实现访问策略 mock 属性、mock 模板和 mock 访问树数据于 `desktop/src/renderer/src/components/access-policy/tree/mockData.ts`
- [X] T032 [P] [US1] 实现 AND 自定义节点视觉与错误态于 `desktop/src/renderer/src/components/access-policy/nodes/AndNode.tsx`
- [X] T033 [P] [US1] 实现 OR 自定义节点视觉与错误态于 `desktop/src/renderer/src/components/access-policy/nodes/OrNode.tsx`
- [X] T034 [P] [US1] 实现属性叶子节点视觉、属性名、操作符、值和错误态于 `desktop/src/renderer/src/components/access-policy/nodes/AttributeNode.tsx`
- [X] T035 [P] [US1] 实现 React Flow 画布封装、拖拽、缩放、选中、高亮和 controls 于 `desktop/src/renderer/src/components/access-policy/AccessTreeCanvas.tsx`
- [X] T036 [P] [US1] 实现节点工具栏、保存、重置、自动布局和居中入口于 `desktop/src/renderer/src/components/access-policy/AccessTreeToolbar.tsx`
- [X] T037 [P] [US1] 实现属性字典面板和创建属性叶子节点入口于 `desktop/src/renderer/src/components/access-policy/AttributeDictionaryPanel.tsx`
- [X] T038 [P] [US1] 实现策略模板选择器和覆盖当前画布提示于 `desktop/src/renderer/src/components/access-policy/PolicyTemplateSelector.tsx`
- [X] T039 [P] [US1] 实现右侧节点配置面板和属性/操作符/值编辑于 `desktop/src/renderer/src/components/access-policy/AccessTreeConfigPanel.tsx`
- [X] T040 [P] [US1] 实现策略表达式预览和非法状态提示于 `desktop/src/renderer/src/components/access-policy/PolicyExpressionPreview.tsx`
- [X] T041 [P] [US1] 实现访问树 JSON 预览和格式化展示于 `desktop/src/renderer/src/components/access-policy/PolicyJsonPreview.tsx`
- [X] T042 [US1] 实现 AccessTreeEditor 状态 reducer、节点操作、转换、校验、dirty 和 saving 状态于 `desktop/src/renderer/src/components/access-policy/AccessTreeEditor.tsx`
- [X] T043 [US1] 实现 DATA_OWNER 构建页面布局、mock 数据接入和编辑器首屏体验于 `desktop/src/renderer/src/pages/AccessPolicyBuilderPage.tsx`
- [X] T044 [US1] 实现访问策略 API 客户端的可用属性、可用模板、创建、详情、更新方法于 `desktop/src/renderer/src/api/policy.ts`
- [X] T045 [US1] 实现后端创建访问策略、详情查询、访问树校验和表达式生成业务逻辑于 `backend/internal/service/policy_service.go`
- [X] T046 [US1] 实现 DATA_OWNER 创建和详情 handler 请求绑定、响应封装和错误边界于 `backend/internal/handler/policy_handler.go`
- [X] T047 [US1] 在 `backend/internal/handler/router.go` 注册 DATA_OWNER 可用属性、可用模板、创建策略和查询详情路由
- [X] T048 [US1] 在 `desktop/src/renderer/src/main.tsx` 注册访问策略构建和编辑路由
- [X] T049 [US1] 为访问树编辑器增加桌面端自适应布局和非普通表单视觉样式于 `desktop/src/renderer/src/styles.css`

**检查点**：US1 可作为 MVP 单独演示，DATA_OWNER 能完成访问树构建、校验、预览、保存和回显。

---

## 阶段 4：用户故事 2 - PLATFORM_ADMIN 管理策略基础能力（优先级：P2）

**目标**：PLATFORM_ADMIN 可以维护属性字典和策略模板，统一约束 DATA_OWNER 可使用的属性与模板。

**独立测试**：PLATFORM_ADMIN 能创建、查看、更新、禁用或删除属性和模板；DATA_OWNER 只能读取启用属性和模板。

### 测试任务

- [X] T050 [P] [US2] 编写平台属性字典 service 测试于 `backend/internal/service/policy_service_test.go`
- [X] T051 [P] [US2] 编写平台策略模板 service 测试于 `backend/internal/service/policy_service_test.go`
- [X] T052 [P] [US2] 编写 PLATFORM_ADMIN 属性和模板 handler 测试于 `backend/internal/handler/policy_handler_test.go`

### 实现任务

- [X] T053 [US2] 实现平台属性字典 CRUD repository 方法于 `backend/internal/repository/policy_repository.go`
- [X] T054 [US2] 实现平台策略模板 CRUD repository 方法于 `backend/internal/repository/policy_repository.go`
- [X] T055 [US2] 实现平台属性字典创建、更新、禁用、删除、编码唯一和 enum 值校验于 `backend/internal/service/policy_service.go`
- [X] T056 [US2] 实现平台策略模板创建、更新、禁用、删除、访问树校验和表达式生成于 `backend/internal/service/policy_service.go`
- [X] T057 [US2] 实现平台属性字典和策略模板 handler 方法于 `backend/internal/handler/policy_handler.go`
- [X] T058 [US2] 在 `backend/internal/handler/router.go` 注册 `/api/v1/platform/policy-attributes` 和 `/api/v1/platform/policy-templates` 路由
- [X] T059 [P] [US2] 扩展访问策略 API 客户端的平台属性和模板 CRUD 方法于 `desktop/src/renderer/src/api/policy.ts`
- [X] T060 [P] [US2] 创建平台访问策略管理页面并组织属性和模板管理区域于 `desktop/src/renderer/src/pages/PlatformPolicyManagementPage.tsx`
- [X] T061 [US2] 实现平台属性字典管理表单、列表、状态切换和错误提示于 `desktop/src/renderer/src/pages/PlatformPolicyManagementPage.tsx`
- [X] T062 [US2] 实现平台策略模板管理表单、访问树 JSON 输入/预览、列表和状态切换于 `desktop/src/renderer/src/pages/PlatformPolicyManagementPage.tsx`
- [X] T063 [US2] 在 `desktop/src/renderer/src/main.tsx` 注册 PLATFORM_ADMIN 访问策略管理路由

**检查点**：US2 可独立演示平台管理员维护属性字典和策略模板，不授予租户文件访问或解密权限。

---

## 阶段 5：用户故事 3 - 按角色与租户边界访问策略（优先级：P3）

**目标**：菜单展示、页面访问和接口操作均遵守 PLATFORM_ADMIN、DATA_OWNER、TENANT_ADMIN、DATA_VISITOR 的角色边界、租户边界和 owner 边界。

**独立测试**：分别用四类角色访问菜单和接口，验证 DATA_OWNER 只能管理自己的策略，TENANT_ADMIN 只读，DATA_VISITOR 禁止写，跨租户访问被拒绝。

### 测试任务

- [X] T064 [P] [US3] 编写 DATA_OWNER 只能更新和删除自己策略的 service 测试于 `backend/internal/service/policy_service_test.go`
- [X] T065 [P] [US3] 编写 TENANT_ADMIN 只读和 DATA_VISITOR 禁止写的 handler 测试于 `backend/internal/handler/policy_handler_test.go`
- [X] T066 [P] [US3] 编写跨租户访问和 PLATFORM_ADMIN 调用租户策略写接口被拒绝的 handler 测试于 `backend/internal/handler/policy_handler_test.go`

### 实现任务

- [X] T067 [US3] 实现访问策略列表、更新、删除 repository 方法并按 tenant_id、owner_id 限定范围于 `backend/internal/repository/policy_repository.go`
- [X] T068 [US3] 实现 DATA_OWNER 更新、删除、列表和 TENANT_ADMIN 只读策略查询业务规则于 `backend/internal/service/policy_service.go`
- [X] T069 [US3] 实现租户访问策略列表、更新、删除 handler 方法于 `backend/internal/handler/policy_handler.go`
- [X] T070 [US3] 在 `backend/internal/handler/router.go` 为租户访问策略接口接入 AuthRequired、TenantRequired 和 Policy handler
- [X] T071 [US3] 在 `backend/internal/repository/tenant_repository.go` 确认或补充策略服务所需租户角色查询能力
- [X] T072 [P] [US3] 创建租户角色路由守卫或页面内守卫组件于 `desktop/src/renderer/src/auth/RequireTenantRole.tsx`
- [X] T073 [US3] 在 `desktop/src/renderer/src/components/AppLayout.tsx` 按 PLATFORM_ADMIN、DO/DATA_OWNER、TENANT_ADMIN、DU/DATA_VISITOR 生成访问策略菜单
- [X] T074 [P] [US3] 创建 DATA_OWNER 我的访问策略列表、编辑、删除入口页面于 `desktop/src/renderer/src/pages/MyAccessPoliciesPage.tsx`
- [X] T075 [P] [US3] 创建 TENANT_ADMIN 租户策略只读查看页面于 `desktop/src/renderer/src/pages/TenantAccessPolicyViewPage.tsx`
- [X] T076 [US3] 在 `desktop/src/renderer/src/main.tsx` 注册我的访问策略和租户策略查看路由并接入路由守卫
- [X] T077 [US3] 在 `desktop/src/renderer/src/api/policy.ts` 接入列表、更新、删除和 TENANT_ADMIN 只读查询方法

**检查点**：US3 完成后，四类角色的菜单、路由和接口权限全部可独立验证。

---

## 阶段 6：收尾与横切关注点

**目标**：完成桌面体验增强、文档验证、宪章要求检查和最终质量门禁。

- [X] T078 [P] 实现访问树本地草稿保存、恢复和远端更新时间冲突提示于 `desktop/src/renderer/src/components/access-policy/tree/draft.ts`
- [X] T079 [P] 实现编辑器快捷键 Ctrl+S、Ctrl+Z、Ctrl+Y、Ctrl+0 的 Renderer 侧处理于 `desktop/src/renderer/src/components/access-policy/AccessTreeEditor.tsx`
- [X] T080 [P] 优化访问树编辑器空状态、错误提示、宽屏布局和节点 hover 操作样式于 `desktop/src/renderer/src/styles.css`
- [ ] T081 运行 gofmt 并修正格式问题于 `backend/internal/domain/policy.go`、`backend/internal/pkg/policytree/*.go`、`backend/internal/repository/policy_repository.go`、`backend/internal/service/policy_service.go`、`backend/internal/handler/policy_handler.go`
- [ ] T082 运行 `go test ./...` 并修复后端访问树、权限、handler 和既有回归测试问题于 `backend/`
- [X] T083 运行 `npm run typecheck` 并修复 TypeScript 类型问题于 `desktop/`
- [X] T084 运行 `npm run build` 并修复 Electron 主进程和 Renderer 构建问题于 `desktop/`
- [ ] T085 根据 `specs/005-access-policy-tree/quickstart.md` 执行 PLATFORM_ADMIN、DATA_OWNER、TENANT_ADMIN、DATA_VISITOR 手工验收并记录偏差于 `specs/005-access-policy-tree/quickstart.md`
- [X] T086 完成 Go 关键注释和可读性检查，确认新增或修改的函数/方法前置注释、GoDoc 前缀、实体字段、Handler、Service、Repository、Middleware 业务语义和安全边界注释于 `backend/internal/`
- [X] T087 确认本阶段未实现文件上传、AES 加密、CP-ABE 加密、密钥生成、私钥分发、文件下载和策略满足性判断，并在 `specs/005-access-policy-tree/quickstart.md` 保留非目标验证说明
- [X] T088 确认 Crypto、File、Benchmark、Audit 模块未被本功能错误改动，Policy 模块边界和后续加密接入边界记录于 `specs/005-access-policy-tree/plan.md`

---

## 依赖与执行顺序

### 阶段依赖

- **阶段 1 准备工作**：无依赖，可立即开始。
- **阶段 2 基础能力**：依赖阶段 1，阻塞所有用户故事。
- **阶段 3 US1**：依赖阶段 2，是 MVP 第一交付。
- **阶段 4 US2**：依赖阶段 2，可与 US1 部分并行，但真实模板接入需要复用访问树校验能力。
- **阶段 5 US3**：依赖阶段 2，菜单和权限联调建议在 US1、US2 有页面和接口后完成。
- **阶段 6 收尾**：依赖计划交付范围内的用户故事完成。

### 用户故事依赖

- **US1（P1）**：基础能力完成后即可独立实现和演示，建议作为 MVP。
- **US2（P2）**：基础能力完成后即可独立实现；与 US1 共享属性、模板和访问树校验。
- **US3（P3）**：基础能力完成后可先做后端权限测试；最终菜单与页面联调依赖 US1、US2 的路由和页面。

### 任务内依赖

- 测试任务应先于对应实现任务编写，并在实现前失败。
- 数据模型和迁移先于 repository。
- repository 先于 service。
- service 先于 handler。
- handler 和 API 客户端先于前后端联调页面。
- 前端 `types.ts`、`convert.ts`、`expression.ts`、`validate.ts` 先于 `AccessTreeEditor.tsx`。

## 并行执行示例

### US1 并行示例

```text
Task: "T032 实现 AND 自定义节点视觉与错误态于 desktop/src/renderer/src/components/access-policy/nodes/AndNode.tsx"
Task: "T033 实现 OR 自定义节点视觉与错误态于 desktop/src/renderer/src/components/access-policy/nodes/OrNode.tsx"
Task: "T034 实现属性叶子节点视觉、属性名、操作符、值和错误态于 desktop/src/renderer/src/components/access-policy/nodes/AttributeNode.tsx"
Task: "T037 实现属性字典面板和创建属性叶子节点入口于 desktop/src/renderer/src/components/access-policy/AttributeDictionaryPanel.tsx"
Task: "T040 实现策略表达式预览和非法状态提示于 desktop/src/renderer/src/components/access-policy/PolicyExpressionPreview.tsx"
```

### US2 并行示例

```text
Task: "T050 编写平台属性字典 service 测试于 backend/internal/service/policy_service_test.go"
Task: "T052 编写 PLATFORM_ADMIN 属性和模板 handler 测试于 backend/internal/handler/policy_handler_test.go"
Task: "T059 扩展访问策略 API 客户端的平台属性和模板 CRUD 方法于 desktop/src/renderer/src/api/policy.ts"
Task: "T060 创建平台访问策略管理页面并组织属性和模板管理区域于 desktop/src/renderer/src/pages/PlatformPolicyManagementPage.tsx"
```

### US3 并行示例

```text
Task: "T064 编写 DATA_OWNER 只能更新和删除自己策略的 service 测试于 backend/internal/service/policy_service_test.go"
Task: "T066 编写跨租户访问和 PLATFORM_ADMIN 调用租户策略写接口被拒绝的 handler 测试于 backend/internal/handler/policy_handler_test.go"
Task: "T072 创建租户角色路由守卫或页面内守卫组件于 desktop/src/renderer/src/auth/RequireTenantRole.tsx"
Task: "T074 创建 DATA_OWNER 我的访问策略列表、编辑、删除入口页面于 desktop/src/renderer/src/pages/MyAccessPoliciesPage.tsx"
Task: "T075 创建 TENANT_ADMIN 租户策略只读查看页面于 desktop/src/renderer/src/pages/TenantAccessPolicyViewPage.tsx"
```

## 实施策略

### MVP 优先

1. 完成阶段 1 准备工作。
2. 完成阶段 2 基础能力。
3. 完成阶段 3 US1。
4. 停下来独立验证 DATA_OWNER 可视化访问树构建、预览、校验、保存和回显。
5. 演示 MVP 后再推进平台管理和完整权限边界。

### 增量交付

1. US1 交付 DATA_OWNER 编辑器体验和策略保存闭环。
2. US2 交付 PLATFORM_ADMIN 属性字典和模板管理。
3. US3 交付角色菜单、租户隔离、owner 边界和只读/禁写规则。
4. 阶段 6 完成桌面体验增强、测试、构建和宪章检查。

### 并行团队策略

1. 共同完成阶段 1 和阶段 2。
2. 前端成员优先推进 US1 编辑器组件和视觉体验。
3. 后端成员并行推进 Policy repository、service、handler 和访问树校验。
4. 另一名成员可推进 US2 平台管理页面和 API 契约测试。
5. 最后集中完成 US3 权限联调和 quickstart 验证。

## 任务格式校验

- 所有任务均使用 `- [ ] T###` checklist 格式。
- 用户故事阶段任务均包含 `[US1]`、`[US2]` 或 `[US3]` 标签。
- 可并行任务使用 `[P]` 标记。
- 每个任务均包含明确文件路径。
- Go 注释和可读性检查已纳入 T086。
