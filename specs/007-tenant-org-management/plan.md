# 实施计划：租户组织架构管理

**分支**：`007-tenant-org-management` | **日期**：2026-07-10 | **规格**：[spec.md](./spec.md)

**输入**：来自 `specs/007-tenant-org-management/spec.md` 的功能规格，以及用户关于 006 依赖、部门职务枚举、系统角色分离、旧数据迁移、主部门、稳定编码、停用语义和接口迁移的修正要求。

## 摘要

本功能在不修改 `006-tenant-org-attributes` 的前提下，新增租户组织架构管理模块。`006` 提供组织、属性和策略构建基础设施；本功能补齐真正的租户管理员组织维护能力，包括部门树 CRUD、部门移动、部门停用/删除保护、成员归属、主部门、部门负责人/副负责人、旧部门角色迁移、当前租户接口和前端组织管理页面。

核心技术取舍是：新接口统一使用 `/api/v1/tenant/...`，`tenant_id` 只来自后端租户上下文；旧 `/api/v1/tenants/:id/...` 组织写接口进入迁移过渡期，但必须复用同一个 Service；部门职务只表达组织职务，不再承载 `DO/DU` 或普通成员身份；系统角色继续由已有成员角色接口写入 `user_roles`。

## 技术上下文

**语言/版本**：后端 Go；前端 TypeScript。版本沿用当前仓库 `backend/go.mod` 与 `desktop/package.json`。

**主要依赖**：Gin、Gorm、MySQL、Redis、React、Electron、Vite、Ant Design、Vitest。稳定编码使用 UUID 或 ULID，第一版优先使用 Go 标准库或现有依赖可接受的 UUID 方案，避免为了一个编码字段引入重依赖。

**存储**：MySQL 保存组织、成员、部门职务、租户属性值和系统角色；Redis 沿用登录态，不为本功能新增缓存。

**测试**：后端使用 `go test ./...`，重点覆盖 Service 和 Handler；前端使用 `npm run typecheck` 和 Vitest 覆盖 API 封装、页面状态和纯函数。

**目标平台**：Electron 桌面端 + Go HTTP 后端。

**项目类型**：桌面端 + Web API 后端的多租户演示系统。

**性能目标**：演示规模下部门树、成员列表和属性同步对用户表现为即时完成；移动部门时以单租户单棵子树为事务范围，避免扫描其他租户数据。

**约束**：所有写操作必须事务化；所有组织查询必须限制当前租户；不得信任前端传入 `tenant_id`；不得把系统角色写入部门职务表；不得修改 006 文档。

**规模/范围**：第一版覆盖当前租户组织管理闭环、旧部门职务迁移、前端管理页面和接口迁移计划；不实现外部组织同步、审批流、完整 RBAC 菜单权限配置或 CP-ABE 密钥撤销。

## 宪章检查

*门禁：Phase 0 研究前必须通过；Phase 1 设计后再次检查。*

- **混合加密边界**：本功能不涉及 AES-GCM、RSA-OAEP、CP-ABE 或 DEK 封装，只为后续策略匹配提供组织属性来源。
- **真实 CP-ABE 实现**：本功能不实现 CP-ABE 加解密，不得用组织属性同步结果冒充真实 CP-ABE 行为。
- **模块边界**：组织成员归属在 User/租户管理边界；属性值同步为 Policy 输入边界；系统角色保留在现有租户角色服务；Crypto、File、Benchmark、Audit 不修改。
- **算法对比口径**：本功能不产生 RSA 与 CP-ABE 性能对比结论。
- **可解释性**：部门稳定 `code/value_code` 用于策略权威引用，`path/value_path` 只用于展示解释；部门改名和移动后历史策略仍能基于稳定引用解释。
- **中文文档**：本 feature 所有 SpecKit 文档使用简体中文，路径、命令、API 和代码标识符保留英文。
- **Go 注释策略**：后续实现会新增或修改 Go 业务代码，必须为所有函数/方法添加中文前置注释；导出标识符 GoDoc 以标识符开头；实体字段、Handler、Service、Repository、Middleware 注释必须解释租户边界、事务边界、幂等恢复、职务/系统角色分离和安全边界。
- **关键注释和可读性检查**：`tasks.md` 必须包含专门任务，检查函数/方法注释、GoDoc 前缀、实体字段注释、业务语义、安全边界和无意义注释清理。

**Phase 1 设计后复查**：通过。设计明确排除了真实加解密和 Benchmark，保留 006 作为依赖，新增接口与迁移都以租户隔离和可解释性为核心。

## 当前代码现状

- `006-tenant-org-attributes` 已新增 `tenant_org_units`、`tenant_org_members`、`tenant_org_member_roles`、`tenant_attributes`、`tenant_attribute_values`、`user_attributes` 及基础后端/前端接入，本功能不修改该目录。
- 后端 `backend/internal/domain/org.go` 当前部门职务枚举为 `ORG_MANAGER`、`ORG_MEMBER`、`DATA_OWNER`、`DATA_VISITOR`，需要在新实现中收敛为 `ORG_LEADER`、`DEPUTY_LEADER`。
- `tenant_org_members` 当前缺少 `is_primary`，需要增量迁移。
- 当前组织接口位于 `/api/v1/tenants/:id/...`，需要新增 `/api/v1/tenant/...` 当前租户接口，并制定旧接口废弃计划。
- 前端已有 `desktop/src/renderer/src/api/tenantOrg.ts` 基础 API 封装，但暂无完整组织管理页面。
- 只读数据库统计已执行：当前开发库 `tenant_org_member_roles` 没有返回旧 `role_code` 聚合行；上线前仍必须按任务再次统计目标环境。

## 数据迁移方案

新增迁移建议命名为 `backend/migrations/008_tenant_org_management.sql`。迁移必须幂等，并在执行前输出或记录旧数据统计。

### 迁移前统计 SQL

```sql
SELECT role_code, status, COUNT(*) AS total
FROM tenant_org_member_roles
GROUP BY role_code, status
ORDER BY role_code, status;

SELECT role_code, tenant_id, COUNT(DISTINCT user_id) AS user_count
FROM tenant_org_member_roles
WHERE role_code IN ('DATA_OWNER', 'DATA_VISITOR')
  AND status = 'active'
GROUP BY role_code, tenant_id
ORDER BY tenant_id, role_code;
```

### 表结构增量

`tenant_org_members` 新增：

- `is_primary TINYINT(1) NOT NULL DEFAULT 0`
- 索引 `idx_tenant_org_members_primary (tenant_id, user_id, status, is_primary)`
- 可选索引 `idx_tenant_org_members_org_primary (tenant_id, org_unit_id, status, is_primary)`

`tenant_org_member_roles` 建议新增：

- 索引 `idx_tenant_org_member_roles_unit_role (tenant_id, org_unit_id, role_code, status)`，用于检查每部门最多一个负责人。

MySQL 无条件唯一索引不能直接表达“每部门最多一个 active `ORG_LEADER`”。第一版使用事务内 `SELECT ... FOR UPDATE` 加服务校验；如后续采用生成列，可增加 `active_leader_key` 唯一约束。

### 旧 `role_code` 数据迁移

1. `ORG_MANAGER -> ORG_LEADER`
   - active/inactive 记录均可幂等更新为 `ORG_LEADER`。
   - 如果同一部门因此出现多个 active `ORG_LEADER`，迁移前必须列出冲突；自动迁移策略只保留最早一条 active 负责人，其余转为 `DEPUTY_LEADER` 或 inactive 需要人工确认。第一版建议迁移脚本检测冲突并失败，交由人工处理。

2. `ORG_MEMBER`
   - 不迁移为 `MEMBER`。
   - 将旧 `ORG_MEMBER` 记录标记为 inactive 或物理删除。推荐标记 inactive，保留历史来源。
   - 普通成员身份由 `tenant_org_members` active 记录自然表达。

3. `DATA_OWNER -> DO`
   - 先查 `roles.code = 'DO'` 的角色 ID。
   - 对每条 active `DATA_OWNER` 部门职务，确保 `user_roles(tenant_id, user_id, role_id)` 中存在 `DO`。
   - 再将旧 `DATA_OWNER` 部门职务标记为 inactive。
   - 不能将它迁移为普通成员；普通成员关系已由 `tenant_org_members` 表达。

4. `DATA_VISITOR -> DU`
   - 先查 `roles.code = 'DU'` 的角色 ID。
   - 对每条 active `DATA_VISITOR` 部门职务，确保 `user_roles(tenant_id, user_id, role_id)` 中存在 `DU`。
   - 再将旧 `DATA_VISITOR` 部门职务标记为 inactive。

5. 幂等性
   - 插入 `user_roles` 使用 `NOT EXISTS` 或唯一键冲突忽略。
   - 停用旧职务使用 `WHERE status = 'active' AND role_code IN (...)`。
   - 重复执行不得重复授权、不得重复插入部门职务。

### 主部门数据补齐

- 对每个 `tenant_id + user_id`：
  - active 部门数为 0：所有 `is_primary = 0`。
  - active 部门数为 1：该记录 `is_primary = 1`。
  - active 部门数大于 1 且已有一个 primary：保留该 primary，清除其他 primary。
  - active 部门数大于 1 且没有 primary：选择最早创建的一条 active 记录作为 primary。
  - active 部门数大于 1 且多个 primary：保留最早更新或最早创建的一条，其余清除。

## 最终 API 定义

详细契约见 [contracts/api.md](./contracts/api.md)。

### 当前租户组织架构

- `GET /api/v1/tenant/org-units/tree`
- `POST /api/v1/tenant/org-units`
- `PUT /api/v1/tenant/org-units/:id`
- `PUT /api/v1/tenant/org-units/:id/move`
- `DELETE /api/v1/tenant/org-units/:id`

### 当前租户成员管理

- `GET /api/v1/tenant/org-members`
- `POST /api/v1/tenant/org-members`
- `PUT /api/v1/tenant/org-members/:id/primary`
- `PUT /api/v1/tenant/org-members/:id/positions`
- `DELETE /api/v1/tenant/org-members/:id`

### 系统角色

- 继续复用已有成员角色接口，不在上述部门成员接口中修改 `systemRoles`。
- 最后一个 `TENANT_ADMIN` 保护继续由现有系统角色服务负责。

### 旧接口迁移与废弃

- 旧 `/api/v1/tenants/:id/org-units/...` 和 `/api/v1/tenants/:id/org-units/:orgUnitId/members...` 进入过渡期。
- 过渡期内旧接口必须调用新 Service，且校验路径 `:id` 与后端租户上下文一致。
- 前端组织管理页面只调用 `/api/v1/tenant/...`。
- 前端迁移完成后废弃旧组织写接口；读接口可视策略构建器兼容需求保留一段时间。
- 不允许长期维护两套业务实现。

## 数据库约束与事务规则

### 稳定编码

- `tenant_org_units.code` 创建时使用 ULID 或 UUID，创建后不可修改。
- `tenant_attribute_values.value_code` 基于不可变部门 `code` 生成，创建后不可修改。
- 部门改名只同步 `name/value_label`。
- 部门移动只同步 `path/level/value_path`。
- 策略权威引用为属性值 ID 或 `value_code`，`value_path` 只作解释。

### 部门树事务

- 创建部门：事务内创建部门、计算路径层级、创建属性值。
- 编辑部门：事务内更新部门基础字段和属性展示字段。
- 移动部门：事务内锁定当前节点、目标父节点和子树，校验循环，更新当前节点及后代路径层级和属性解释路径。
- 删除部门：事务内检查子部门和 active 成员；有依赖则拒绝。
- 停用部门：事务内检查不存在 enabled 子部门，更新部门和属性值状态；不得解释为密钥撤销。

### 成员事务

- 加入部门：事务内校验租户成员、部门启用状态，恢复旧记录或创建新记录；若这是用户唯一 active 部门，自动设为主部门。
- 设置主部门：事务内锁定该用户当前租户 active 部门关系，清除旧主部门并设置新主部门。
- 删除成员关系：事务内停用成员关系和该关系下职务；如果删除主部门且仍有其他部门，必须同事务选择新主部门或按请求指定。
- 多部门约束：active 部门数大于 1 时必须且只能有一个 primary。

### 职务事务

- 设置部门职务前必须确认 `tenant_org_members` active。
- `tenant_org_member_roles` 只允许 `ORG_LEADER`、`DEPUTY_LEADER`。
- 设置 `ORG_LEADER` 时必须锁定该部门 active leader 记录，确保最多一个。
- `DEPUTY_LEADER` 可多人。
- 移除成员关系时，该成员关系下职务同事务 inactive。

## 前端设计

- `desktop/src/renderer/src/pages/TenantOrgManagementPage.tsx` 新增组织管理页面。
- `AppLayout.tsx` 在“租户管理”菜单下新增“组织管理”。
- 页面包含“组织架构”和“成员管理”两个页签。
- 组织架构页签采用左侧部门树、右侧详情布局。
- 创建、编辑、移动部门使用抽屉。
- 删除和停用使用 Ant Design Modal，不使用原生 `confirm`。
- 成员管理支持搜索、部门筛选、状态筛选。
- 成员编辑抽屉可展示部门、主部门、部门职务和系统角色；保存时分别调用组织成员/职务接口和现有系统角色接口。

## 测试策略

- 后端 Service 测试覆盖租户隔离、部门创建/编辑/移动/停用/删除、属性同步、主部门规则、职务白名单、每部门唯一负责人、旧职务迁移幂等。
- 后端 Handler 测试覆盖 `/api/v1/tenant/...` 当前租户接口、非 `TENANT_ADMIN` 拒绝、路径租户旧接口兼容校验。
- 前端测试覆盖 API 路径、组织管理页面状态、抽屉保存拆分调用、无原生 `confirm`。
- 验收前运行 `go test ./...`、`npm run typecheck` 和相关 Vitest。

## 项目结构

### 文档（本功能）

```text
specs/007-tenant-org-management/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── api.md
└── tasks.md
```

### 源码（仓库根目录）

```text
backend/
├── migrations/
│   └── 008_tenant_org_management.sql
└── internal/
    ├── domain/
    ├── repository/
    ├── service/
    ├── handler/
    └── pkg/response/

desktop/
└── src/renderer/src/
    ├── api/
    ├── pages/
    └── components/tenant-org/
```

**结构决策**：沿用当前后端 `domain/repository/service/handler` 分层和前端 `api/pages/components` 结构，不新增独立子应用。

## 复杂度跟踪

无宪章违例，不需要复杂度豁免。
