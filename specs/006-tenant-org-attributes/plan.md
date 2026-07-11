# 实现计划：租户级组织架构与 CP-ABE 属性体系

**分支**：`006-tenant-org-attributes` | **日期**：2026-07-09 | **规格**：[spec.md](./spec.md)

**输入**：来自 `/specs/006-tenant-org-attributes/spec.md` 的功能规格，以及用户补充的技术栈、数据表、接口、种子数据、前端改造和权限边界要求。

## 摘要

本功能为现有多租户 CP-ABE 演示系统补齐“真实租户组织与属性来源”。MVP 优先保证访问策略构建器不再依赖静态假数据，而是从当前租户读取组织树、属性字典、属性值和可比较属性；同时提供租户管理员维护组织、部门成员、部门角色和同步用户 CP-ABE 属性的最小闭环。

实现策略是新增租户级组织与属性数据模型，保留现有 `users`、`tenants`、`tenant_users`、`roles`、`user_roles` 和平台级 `policy_attributes` 不破坏；新增 `tenant_attributes` 作为策略构建器的租户级权威属性来源，并通过 `user_attributes` 输出后续策略匹配和密钥发放所需的用户属性输入。本阶段不实现真实 CP-ABE 加解密、完整文件上传、复杂审批流或外部组织同步。

## 技术上下文

**语言/版本**：后端 Go；前端 TypeScript；具体版本沿用当前仓库 `backend/go.mod` 与 `desktop/package.json`。

**主要依赖**：Gin、Gorm、MySQL、Redis、React、Electron、Vite、Vitest；不新增密码学依赖。

**存储**：MySQL 保存组织、成员、部门角色、租户属性、属性值和用户属性；Redis 仅沿用现有登录态/缓存能力，本功能 MVP 不要求新增 Redis 缓存。

**测试**：后端使用 Go test 覆盖 Service、Repository 和 Handler；前端使用 TypeScript typecheck 与 Vitest 覆盖访问树节点模型、属性选择和旧策略兼容。

**目标平台**：Electron 桌面端 + 本地或局域网后端服务。

**项目类型**：桌面端 + Web API 后端的多租户演示系统。

**性能目标**：单租户组织树和属性字典加载在演示数据规模下应让用户感知为即时完成；用户属性同步应在单个用户粒度内完成，避免阻塞策略构建器入口。

**约束**：保持现有登录、多租户、租户成员、租户角色和访问策略构建器兼容；访问策略构建器优先使用真实租户属性，不能再静默回退到静态假数据；跨租户读写拒绝率必须达到 100%。

**规模/范围**：MVP 覆盖三个演示租户、组织树、部门成员、部门角色、租户属性字典、用户属性同步和构建器动态加载；不建设完整组织管理后台和复杂审计后台。

## 宪章检查

*门禁：Phase 0 研究前必须通过；Phase 1 设计后再次检查。*

- **混合加密边界**：本功能不触及 AES-GCM 文件内容加密、RSA-OAEP、CP-ABE 或 DEK 封装，只提供属性和策略输入。后续文件加密仍必须遵守“文件内容 -> AES-GCM，DEK -> RSA / CP-ABE”。
- **真实 CP-ABE 实现**：本功能不实现 CP-ABE 加解密，也不允许用属性同步或策略匹配结果冒充真实 CP-ABE。后续解密能力必须通过真实 Go CP-ABE 库接入。
- **模块边界**：组织成员与用户属性放在 User/租户管理边界；租户属性字典、策略构建器属性加载、访问树节点校验放在 Policy 边界；Crypto、File、Benchmark、Audit 不在本阶段修改。
- **算法对比口径**：本功能不新增算法 Benchmark，不产生 RSA 与 CP-ABE 性能结论。
- **可解释性**：用户属性必须保留来源类型、来源 ID、属性编码、值编码/路径和展示名，让界面能解释“用户为什么拥有这个属性”和“策略节点引用了哪个真实部门或角色”。
- **中文文档**：本 feature 的 SpecKit 文档使用简体中文；代码标识符、路径、API 字段和技术名按工程规范保留英文。
- **Go 注释策略**：后续实现会新增 Go 业务代码。所有新增函数/方法必须有中文前置注释；导出标识符 GoDoc 必须以标识符开头；实体字段必须解释业务含义、来源、可空性、敏感性、权限参与情况；Handler、Service、Repository、Middleware 必须解释租户边界、权限边界、事务和属性同步副作用。
- **关键注释和可读性检查**：后续 `tasks.md` 必须包含“关键注释和可读性检查”任务，覆盖函数/方法注释、GoDoc 前缀、实体字段注释、业务语义、安全边界和无意义注释清理。

**Phase 1 设计后复查**：通过。设计产物将真实加解密排除在本阶段之外，明确了 User/Policy 模块边界、租户隔离规则、访问策略可解释字段和 Go 注释策略；无宪章违例。

## 架构设计

### 后端分层

```text
backend/internal/domain/
├── org.go                 # OrgUnit、OrgMember、OrgMemberRole
└── tenant_attribute.go    # TenantAttribute、TenantAttributeValue、UserAttribute

backend/internal/repository/
└── org_attribute_repository.go
    # 组织树、成员、部门角色、属性字典、属性值、用户属性持久化

backend/internal/service/
└── org_attribute_service.go
    # 租户边界、角色边界、组织树路径、成员角色幂等、用户属性同步

backend/internal/handler/
└── org_attribute_handler.go
    # 租户组织与属性接口的请求绑定、响应封装

backend/internal/pkg/policytree/
└── 扩展属性元数据校验
    # 支持 tree/enum/number、valueId/path、>=、<=
```

### 前端分层

```text
desktop/src/renderer/src/api/
└── tenantOrg.ts           # 组织树、属性字典、成员、用户属性接口

desktop/src/renderer/src/components/access-policy/tree/
├── types.ts               # 扩展 PolicyTreeNode 和 PolicyAttribute
├── validate.ts            # 基于租户属性元数据校验节点
├── expression.ts          # 支持“属于”和数字比较展示
└── convert.ts             # 保留旧节点兼容

desktop/src/renderer/src/components/access-policy/
├── AttributeDictionaryPanel.tsx
├── NodeConfigPopover.tsx
└── AccessTreeConfigPanel.tsx
```

### MVP 范围切分

1. **第一步：后端数据基础与种子数据**  
   创建 6 张新表、领域模型、仓储、演示租户组织和属性种子数据，保证真实字典可以查询。

2. **第二步：构建器数据接口**  
   优先交付 `GET /tenants/:id/access-policy/attributes` 的租户级增强版，以及组织树接口，让前端可以加载真实属性。

3. **第三步：前端构建器改造**  
   移除静态假数据作为正常路径，支持 `department` 树形选择、枚举选择、数字比较、节点保存 `code/valueId/label/path`。

4. **第四步：部门成员与用户属性同步**  
   提供租户管理员添加部门成员、设置部门角色、同步用户属性和 DU 查看自己属性的最小接口。

5. **第五步：管理体验补齐**  
   在已有租户成员页或轻量组织管理入口展示部门成员与角色；完整拖拽式组织管理后台延后。

## 数据流

### 访问策略构建器加载真实属性

```text
DATA_OWNER 进入访问策略构建器
-> 前端读取当前 tenantId
-> GET /api/v1/tenants/:id/access-policy/attributes
-> 后端校验登录态、租户上下文、DO/TENANT_ADMIN 可读边界
-> 查询 tenant_attributes 和 tenant_attribute_values
-> department 属性附带当前租户 org tree
-> 前端渲染属性字典、树形部门选择器、枚举选择器、数字比较控件
-> 保存访问树节点时写入 code/valueId/valueCode/label/path/operator
```

### 租户管理员维护组织成员和部门角色

```text
TENANT_ADMIN 添加用户到部门
-> POST /api/v1/tenants/:id/org-units/:orgUnitId/members
-> 校验用户属于 tenant_users 且部门属于当前租户
-> 写入 tenant_org_members，重复添加保持幂等
-> 设置部门角色 PUT /api/v1/tenants/:id/org-units/:orgUnitId/members/:userId/roles
-> 校验 roleCode 属于 ORG_MANAGER/ORG_MEMBER/DATA_OWNER/DATA_VISITOR
-> 事务内替换或合并 tenant_org_member_roles
-> 触发或提示执行用户属性同步
```

### 用户 CP-ABE 属性同步

```text
同步单个用户
-> POST /api/v1/tenants/:id/users/:userId/attributes/sync
-> 读取 tenant_users、user_roles、tenant_org_members、tenant_org_member_roles、tenant_attributes
-> 生成 department、org_role、tenant_role、security_level 等属性
-> 事务内将旧 source 范围属性置为 inactive 或替换为新集合
-> 返回用户有效属性列表和同步摘要
```

## 数据库迁移计划

新增迁移建议命名：`backend/migrations/005_create_tenant_org_attribute_tables.sql`。

### `tenant_org_units`

- `id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT`
- `tenant_id BIGINT UNSIGNED NOT NULL`
- `parent_id BIGINT UNSIGNED NULL`
- `code VARCHAR(64) NOT NULL`
- `name VARCHAR(128) NOT NULL`
- `path VARCHAR(512) NOT NULL`
- `level INT NOT NULL DEFAULT 1`
- `sort_order INT NOT NULL DEFAULT 0`
- `status VARCHAR(32) NOT NULL DEFAULT 'enabled'`
- `created_at DATETIME(3) NOT NULL`
- `updated_at DATETIME(3) NOT NULL`
- `deleted_at DATETIME(3) NULL`
- 唯一键：`uk_tenant_org_units_tenant_code (tenant_id, code)`、`uk_tenant_org_units_tenant_path (tenant_id, path)`
- 索引：`idx_tenant_org_units_parent (tenant_id, parent_id, sort_order)`、`idx_tenant_org_units_status (tenant_id, status)`、`idx_tenant_org_units_deleted_at (deleted_at)`

### `tenant_org_members`

- `id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT`
- `tenant_id BIGINT UNSIGNED NOT NULL`
- `org_unit_id BIGINT UNSIGNED NOT NULL`
- `user_id BIGINT UNSIGNED NOT NULL`
- `status VARCHAR(32) NOT NULL DEFAULT 'active'`
- `source VARCHAR(32) NOT NULL DEFAULT 'manual'`
- `created_at DATETIME(3) NOT NULL`
- `updated_at DATETIME(3) NOT NULL`
- `deleted_at DATETIME(3) NULL`
- 唯一键：`uk_tenant_org_members_scope (tenant_id, org_unit_id, user_id)`
- 索引：`idx_tenant_org_members_user (tenant_id, user_id)`、`idx_tenant_org_members_unit (tenant_id, org_unit_id, status)`

### `tenant_org_member_roles`

- `id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT`
- `tenant_id BIGINT UNSIGNED NOT NULL`
- `org_member_id BIGINT UNSIGNED NOT NULL`
- `org_unit_id BIGINT UNSIGNED NOT NULL`
- `user_id BIGINT UNSIGNED NOT NULL`
- `role_code VARCHAR(64) NOT NULL`
- `status VARCHAR(32) NOT NULL DEFAULT 'active'`
- `source VARCHAR(32) NOT NULL DEFAULT 'manual'`
- `created_at DATETIME(3) NOT NULL`
- `updated_at DATETIME(3) NOT NULL`
- `deleted_at DATETIME(3) NULL`
- 唯一键：`uk_tenant_org_member_roles_scope (tenant_id, org_unit_id, user_id, role_code)`
- 索引：`idx_tenant_org_member_roles_member (org_member_id)`、`idx_tenant_org_member_roles_user (tenant_id, user_id)`

### `tenant_attributes`

- `id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT`
- `tenant_id BIGINT UNSIGNED NOT NULL`
- `attr_code VARCHAR(64) NOT NULL`
- `attr_name VARCHAR(128) NOT NULL`
- `attr_type VARCHAR(32) NOT NULL`
- `value_source VARCHAR(32) NOT NULL DEFAULT 'manual'`
- `is_required TINYINT(1) NOT NULL DEFAULT 0`
- `is_policy_enabled TINYINT(1) NOT NULL DEFAULT 1`
- `description TEXT NULL`
- `status VARCHAR(32) NOT NULL DEFAULT 'enabled'`
- `created_at DATETIME(3) NOT NULL`
- `updated_at DATETIME(3) NOT NULL`
- `deleted_at DATETIME(3) NULL`
- 唯一键：`uk_tenant_attributes_tenant_code (tenant_id, attr_code)`
- 索引：`idx_tenant_attributes_policy (tenant_id, is_policy_enabled, status)`

### `tenant_attribute_values`

- `id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT`
- `tenant_id BIGINT UNSIGNED NOT NULL`
- `attribute_id BIGINT UNSIGNED NOT NULL`
- `value_code VARCHAR(128) NOT NULL`
- `value_label VARCHAR(128) NOT NULL`
- `value_path VARCHAR(512) NULL`
- `org_unit_id BIGINT UNSIGNED NULL`
- `sort_order INT NOT NULL DEFAULT 0`
- `status VARCHAR(32) NOT NULL DEFAULT 'enabled'`
- `created_at DATETIME(3) NOT NULL`
- `updated_at DATETIME(3) NOT NULL`
- `deleted_at DATETIME(3) NULL`
- 唯一键：`uk_tenant_attribute_values_code (tenant_id, attribute_id, value_code)`
- 索引：`idx_tenant_attribute_values_attr (tenant_id, attribute_id, status)`、`idx_tenant_attribute_values_org_unit (tenant_id, org_unit_id)`

### `user_attributes`

- `id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT`
- `tenant_id BIGINT UNSIGNED NOT NULL`
- `user_id BIGINT UNSIGNED NOT NULL`
- `attribute_id BIGINT UNSIGNED NOT NULL`
- `attr_code VARCHAR(64) NOT NULL`
- `value_id BIGINT UNSIGNED NULL`
- `value_code VARCHAR(128) NULL`
- `value_label VARCHAR(128) NULL`
- `value_path VARCHAR(512) NULL`
- `number_value DECIMAL(10,2) NULL`
- `source_type VARCHAR(64) NOT NULL`
- `source_id BIGINT UNSIGNED NULL`
- `status VARCHAR(32) NOT NULL DEFAULT 'active'`
- `synced_at DATETIME(3) NOT NULL`
- `created_at DATETIME(3) NOT NULL`
- `updated_at DATETIME(3) NOT NULL`
- `deleted_at DATETIME(3) NULL`
- 唯一键：`uk_user_attributes_effective (tenant_id, user_id, attr_code, value_code, value_path, source_type, source_id)`
- 索引：`idx_user_attributes_user (tenant_id, user_id, status)`、`idx_user_attributes_attr (tenant_id, attr_code, status)`

### 兼容策略

- 保留 `policy_attributes` 作为平台级策略管理历史能力，不在迁移中删除或改名。
- `GET /tenants/:id/access-policy/attributes` 从租户级 `tenant_attributes` 返回真实数据；若租户级数据未初始化，可返回空列表和明确提示，不再静默使用 `policy_attributes` 假数据。
- 已保存旧访问树 `{ attribute, operator, value }` 继续回显；新节点保存扩展结构，并在后端保存前规范化。

## 后端接口列表

详细契约见 [contracts/api.md](./contracts/api.md)。

### 构建器与属性查询

- `GET /api/v1/tenants/:id/org-units/tree`：查询当前租户组织树。
- `GET /api/v1/tenants/:id/access-policy/attributes`：访问树构建器加载真实租户属性字典。
- `GET /api/v1/tenants/:id/users/me/attributes`：DU/DO 查看自己的有效 CP-ABE 属性。

### 组织成员与角色

- `GET /api/v1/tenants/:id/org-units/:orgUnitId/members`：查询部门成员。
- `POST /api/v1/tenants/:id/org-units/:orgUnitId/members`：添加用户到部门。
- `DELETE /api/v1/tenants/:id/org-units/:orgUnitId/members/:userId`：从部门移除用户。
- `PUT /api/v1/tenants/:id/org-units/:orgUnitId/members/:userId/roles`：设置用户部门角色。

### 用户属性同步

- `POST /api/v1/tenants/:id/users/:userId/attributes/sync`：租户管理员同步指定用户 CP-ABE 属性。
- `POST /api/v1/tenants/:id/users/attributes/sync`：可选，批量同步本租户用户属性；MVP 可延后。

### 组织和属性管理最小接口

- `POST /api/v1/tenants/:id/org-units`：创建组织单元。
- `PUT /api/v1/tenants/:id/org-units/:orgUnitId`：更新组织单元基础信息、父级和排序。
- `PATCH /api/v1/tenants/:id/org-units/:orgUnitId/disable`：停用组织单元。
- `GET /api/v1/tenants/:id/tenant-attributes`：租户管理员查询属性字典。
- `PUT /api/v1/tenants/:id/tenant-attributes/:attributeId`：租户管理员启停或调整租户属性；MVP 可先仅支持种子数据初始化后的读取。

## 前端改造点

- 新增 `desktop/src/renderer/src/api/tenantOrg.ts`，封装组织树、租户属性字典、部门成员、用户属性同步接口。
- 扩展 `PolicyAttribute` 类型：支持 `attrType = tree | enum | number | string`、`operators`、`values`、`tree`、`valueSource`。
- 扩展 `PolicyOperator`：从 `=`、`!=` 扩展为 `=`、`!=`、`>=`、`<=`；数字属性仅展示数字比较，枚举/树属性默认展示 `=`/`!=` 或“属于”语义。
- 扩展 `PolicyTreeNode` 叶子节点：保存 `attribute`、`operator`、`value`、`valueId`、`valueCode`、`label`、`path`，同时兼容旧策略只含 `value` 的结构。
- `AttributeDictionaryPanel` 改为展示后端动态字典，空列表时显示“当前租户未初始化属性字典”，不再默认渲染静态 mock。
- `NodeConfigPopover` 和 `AccessTreeConfigPanel` 增加：
  - `department` 树形选择器；
  - `org_role`、`tenant_role`、`data_category` 枚举选择；
  - `security_level` 数字输入和比较操作符；
  - 保存稳定值与展示值。
- `AccessPolicyBuilderPage` 和独立编辑器监听 `auth.currentTenantId` 变化，切换租户后清空旧属性、重新加载字典和组织树，并提示当前草稿可能属于原租户。
- `validate.ts` 使用租户属性元数据校验属性编码、值编码、值路径和操作符，不再只校验静态 enum 字符串。
- 小画布继续只做预览；复杂节点编辑仍进入独立编辑器。

## 种子数据设计

### 租户定位

通过 `tenants.code` 或 `tenants.name` 定位三个演示租户：

- 深信服科技：建议 code `sangfor`
- 四川师范大学：建议 code `scnu`
- 香港友邦保险：建议 code `aia-hk`

若当前库中 code 不一致，种子逻辑以名称兜底查找，不创建重复租户。

### 组织树

按规格完整预置三棵树。组织单元 `code` 使用稳定英文/拼音/缩写，`name` 使用中文展示名，`path` 使用稳定 code 路径，例如 `/AI_BG/AI_PLATFORM`。

### 属性字典

每个租户预置：

- `department`：`tree` 类型，值来源 `org_tree`，值由 `tenant_org_units` 同步生成。
- `org_role`：`enum` 类型，值为 `ORG_MANAGER`、`ORG_MEMBER`、`DATA_OWNER`、`DATA_VISITOR`。
- `tenant_role`：`enum` 类型，值为 `TENANT_ADMIN`、`DATA_OWNER`、`DATA_VISITOR`，并兼容现有 `DO`/`DU` 映射。
- `security_level`：`number` 类型，允许 `>=`、`<=`、`=`。
- `data_category`：`enum` 类型，预置若干演示分类，例如 `PUBLIC`、`INTERNAL`、`CONFIDENTIAL`。

### 演示用户、成员和角色

- 不强制创建大量新用户；优先复用现有 `users` 和 `tenant_users`。
- 若演示用户不足，可为每个租户补齐 3 到 5 个演示用户，并分配到关键部门。
- 每个租户至少预置：
  - 1 名 `TENANT_ADMIN`；
  - 1 名部门 `ORG_MANAGER`；
  - 1 名部门 `DATA_OWNER`；
  - 1 名部门 `DATA_VISITOR`；
  - 1 名安全等级满足示例策略的 DU。

### `user_attributes`

种子完成后执行一次同步：

- 根据部门成员生成 `department` 属性，包含 `value_id`、`value_code`、`value_label`、`value_path`。
- 根据部门角色生成 `org_role` 属性，`source_type = org_member_role`。
- 根据 `user_roles` 生成 `tenant_role` 属性，`source_type = tenant_role`。
- 根据演示配置生成 `security_level` 数字属性，`source_type = manual_seed`。

## 风险点

- **旧平台属性与新租户属性并存**：如果不明确数据来源，构建器可能混用平台假数据和租户真实数据。计划要求构建器入口以租户级属性为准，平台属性仅保留管理兼容。
- **访问树 JSON 兼容**：旧节点只保存 `value`，新节点保存 `valueId/path/label`。需要在前后端转换层兼容旧结构，避免已保存策略无法回显。
- **部门移动后的路径变化**：策略保存 path 后，如果组织树移动，历史策略解释可能变化。MVP 使用稳定 `org_unit_id/valueId` 作为权威，`path` 用于可解释回显；路径变化后可重新生成展示路径。
- **角色编码冲突**：部门内 `DATA_OWNER`/`DATA_VISITOR` 与租户级 DO/DU 可能让用户混淆。计划要求文档和界面区分“部门角色”和“租户角色”，后端字段使用 `org_role` 与 `tenant_role` 分开。
- **属性同步一致性**：成员和角色变更后若未同步，DU 属性可能过期。MVP 在写操作后同步单个用户或返回“需同步”提示，后续再引入异步任务。
- **权限边界遗漏**：组织、属性、用户属性均是访问控制输入，所有查询和写入必须绑定 `tenant_id` 与当前租户上下文。
- **范围膨胀**：完整组织管理后台、外部组织同步、审批流和复杂审计都很诱人，但本阶段只保证构建器真实数据闭环。

## 验收标准

- 三个演示租户的组织树完整初始化，并且跨租户查询或写入组织数据全部被拒绝。
- `GET /api/v1/tenants/:id/access-policy/attributes` 返回当前租户真实属性字典，`department` 带组织树，`enum` 带选项，`number` 带比较操作符。
- 访问策略构建器切换租户后自动刷新属性字典，不显示上一个租户的部门或角色。
- `department` 节点保存 `code/valueId/label/path`，`org_role` 和 `tenant_role` 保存稳定枚举值，`security_level` 保存数字值和比较操作符。
- 租户管理员可以把用户加入部门、设置通用部门角色，并同步该用户 CP-ABE 属性。
- DU 可以查看自己的有效用户属性；本阶段不要求完成真实解密。
- 后端 Service、Repository、Handler 的租户边界和角色边界测试通过。
- 前端类型检查和访问树相关单元测试通过。
- 新增 Go 业务代码完成中文注释和可读性检查。

## 项目结构

### 文档（本功能）

```text
specs/006-tenant-org-attributes/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── api.md
└── tasks.md              # 由 /speckit-tasks 后续生成
```

### 源码（仓库根目录）

```text
backend/
├── migrations/
│   └── 005_create_tenant_org_attribute_tables.sql
├── internal/
│   ├── domain/
│   ├── repository/
│   ├── service/
│   ├── handler/
│   ├── router/
│   └── pkg/policytree/
└── scripts/

desktop/
└── src/renderer/src/
    ├── api/
    ├── pages/
    └── components/access-policy/
```

**结构决策**：沿用当前后端 `domain/repository/service/handler/router` 分层和前端 `api/pages/components/access-policy` 结构，不新增独立子应用。组织与属性能力作为现有租户和 Policy 能力的增量模块接入。

## 复杂度跟踪

无宪章违例，不需要复杂度豁免。

## 补充设计：平台管理员创建租户管理员账号

该能力作为平台治理入口接入现有平台后台，不改变组织架构、属性字典和访问策略构建器的核心边界。后端复用 `POST /api/v1/platform/tenants/:id/admins`，兼容原有 `{user_id}` 授权模式，并新增账号创建请求体。服务层按邮箱复用用户，新建用户时使用 bcrypt 写入密码哈希，设置 `must_change_password = true`，并只通过 `tenant_users` 与当前租户下的 `user_roles(TENANT_ADMIN)` 表达租户管理员身份。

安全取舍：新建租户管理员的旧 `users.role` 使用低权限兼容角色，不写成 `admin`，避免被旧平台管理判断误识别为平台或全局管理员。该补充不涉及真实 CP-ABE 加解密、文件上传或密钥发放，只为租户后续自主管理组织和 CP-ABE 用户属性提供账号初始化能力。
